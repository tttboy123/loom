package vault

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

const (
	localKeyMagic      = "LOOMVLT1"
	localKeySchema     = uint16(1)
	localKeyVersion    = uint32(1)
	localKeyIDBytes    = 16
	localKeyFileLength = len(localKeyMagic) + 2 + 4 + localKeyIDBytes + vaultKeyBytes
)

type LocalKeyFile struct {
	Path   string
	Random io.Reader
}

type KeyMaterial struct {
	mu             sync.Mutex
	key            []byte
	keyID          [localKeyIDBytes]byte
	keyVersion     uint32
	sourcePath     string
	sourceIdentity privateFileIdentity
	closed         bool
}

func (provider LocalKeyFile) LoadOrCreate(
	ctx context.Context,
) (*KeyMaterial, error) {
	parent, err := provider.validate(ctx)
	if err != nil {
		return nil, err
	}
	material, err := loadLocalKeyMaterial(provider.Path)
	if err == nil {
		return material, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := provider.publish(parent, localKeyVersion); err != nil {
		if errors.Is(err, os.ErrExist) {
			return loadLocalKeyMaterial(provider.Path)
		}
		return nil, err
	}
	return loadLocalKeyMaterial(provider.Path)
}

// Create publishes a new key file at an explicit version without replacing an
// existing path. It is used to stage rotation material before database rewrap.
func (provider LocalKeyFile) Create(
	ctx context.Context,
	keyVersion uint32,
) (*KeyMaterial, error) {
	parent, err := provider.validate(ctx)
	if err != nil {
		return nil, err
	}
	if keyVersion == 0 {
		return nil, ErrInvalidVaultInput
	}
	if err := provider.publish(parent, keyVersion); err != nil {
		return nil, err
	}
	return loadLocalKeyMaterial(provider.Path)
}

func (provider LocalKeyFile) validate(ctx context.Context) (string, error) {
	if ctx == nil || ctx.Err() != nil || !filepath.IsAbs(provider.Path) ||
		filepath.Clean(provider.Path) != provider.Path {
		return "", ErrInvalidVaultInput
	}
	parent := filepath.Dir(provider.Path)
	if err := validatePrivateDirectory(parent); err != nil {
		return "", err
	}
	return parent, nil
}

func (provider LocalKeyFile) publish(parent string, keyVersion uint32) error {
	randomSource := provider.Random
	if randomSource == nil {
		randomSource = rand.Reader
	}
	key := make([]byte, vaultKeyBytes)
	if _, err := io.ReadFull(randomSource, key); err != nil {
		clearBytes(key)
		return ErrVaultUnavailable
	}
	data := encodeLocalKeyFile(key, keyVersion)
	clearBytes(key)
	defer clearBytes(data)
	return publishPrivateKeyFile(parent, provider.Path, data)
}

func (material *KeyMaterial) KeyVersion() uint32 {
	if material == nil {
		return 0
	}
	material.mu.Lock()
	defer material.mu.Unlock()
	if material.closed {
		return 0
	}
	return material.keyVersion
}

func (material *KeyMaterial) KeyID() [localKeyIDBytes]byte {
	if material == nil {
		return [localKeyIDBytes]byte{}
	}
	material.mu.Lock()
	defer material.mu.Unlock()
	if material.closed {
		return [localKeyIDBytes]byte{}
	}
	return material.keyID
}

func (material *KeyMaterial) keyCopy() ([]byte, error) {
	if material == nil {
		return nil, ErrVaultUnavailable
	}
	material.mu.Lock()
	defer material.mu.Unlock()
	identity, err := capturePrivateFileIdentity(material.sourcePath)
	if material.closed || len(material.key) != vaultKeyBytes || err != nil ||
		identity != material.sourceIdentity {
		return nil, ErrVaultUnavailable
	}
	return append([]byte(nil), material.key...), nil
}

func (material *KeyMaterial) Close() error {
	if material == nil {
		return nil
	}
	material.mu.Lock()
	defer material.mu.Unlock()
	clearBytes(material.key)
	material.key = nil
	material.closed = true
	return nil
}

func encodeLocalKeyFile(key []byte, keyVersion uint32) []byte {
	body := make([]byte, 0, localKeyFileLength)
	body = append(body, localKeyMagic...)
	version := make([]byte, 6)
	binary.BigEndian.PutUint16(version[:2], localKeySchema)
	binary.BigEndian.PutUint32(version[2:], keyVersion)
	body = append(body, version...)
	digest := sha256.Sum256(key)
	body = append(body, digest[:localKeyIDBytes]...)
	body = append(body, key...)
	return body
}

func decodeLocalKeyMaterial(data []byte) (*KeyMaterial, error) {
	defer clearBytes(data)
	if len(data) != localKeyFileLength ||
		string(data[:len(localKeyMagic)]) != localKeyMagic {
		return nil, ErrVaultUnavailable
	}
	offset := len(localKeyMagic)
	keyVersion := binary.BigEndian.Uint32(data[offset+2 : offset+6])
	if binary.BigEndian.Uint16(data[offset:offset+2]) != localKeySchema ||
		keyVersion == 0 {
		return nil, ErrVaultUnavailable
	}
	offset += 6
	var keyID [localKeyIDBytes]byte
	copy(keyID[:], data[offset:offset+localKeyIDBytes])
	offset += localKeyIDBytes
	key := append([]byte(nil), data[offset:]...)
	digest := sha256.Sum256(key)
	if keyID != [localKeyIDBytes]byte(digest[:localKeyIDBytes]) {
		clearBytes(key)
		return nil, ErrVaultUnavailable
	}
	return &KeyMaterial{
		key: key, keyID: keyID, keyVersion: keyVersion,
	}, nil
}

func loadLocalKeyMaterial(path string) (*KeyMaterial, error) {
	data, identity, err := readPrivateKeyFile(path)
	if err != nil {
		return nil, err
	}
	material, err := decodeLocalKeyMaterial(data)
	if err != nil {
		return nil, err
	}
	material.sourcePath = path
	material.sourceIdentity = identity
	return material, nil
}

func validatePrivateDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() ||
		info.Mode().Perm() != 0o700 || !ownedByCurrentUser(info) {
		return ErrVaultUnavailable
	}
	return nil
}

func readPrivateKeyFile(path string) ([]byte, privateFileIdentity, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, privateFileIdentity{}, err
	}
	if err := validatePrivateRegularFile(before); err != nil {
		return nil, privateFileIdentity{}, err
	}
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, privateFileIdentity{}, ErrVaultUnavailable
	}
	defer file.Close()
	after, err := file.Stat()
	if err != nil || !sameFileIdentity(before, after) {
		return nil, privateFileIdentity{}, ErrVaultUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(localKeyFileLength+1)))
	if err != nil || len(data) != localKeyFileLength {
		clearBytes(data)
		return nil, privateFileIdentity{}, ErrVaultUnavailable
	}
	stat, ok := after.Sys().(*syscall.Stat_t)
	if !ok {
		clearBytes(data)
		return nil, privateFileIdentity{}, ErrVaultUnavailable
	}
	return data, privateFileIdentity{
		device: uint64(stat.Dev),
		inode:  uint64(stat.Ino),
	}, nil
}

func publishPrivateKeyFile(parent, path string, data []byte) error {
	temp, err := os.CreateTemp(parent, ".loom-vault-key-*")
	if err != nil {
		return ErrVaultUnavailable
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return ErrVaultUnavailable
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return ErrVaultUnavailable
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return ErrVaultUnavailable
	}
	if err := temp.Close(); err != nil {
		return ErrVaultUnavailable
	}
	if err := os.Link(tempPath, path); err != nil {
		return err
	}
	if err := os.Remove(tempPath); err != nil {
		return ErrVaultUnavailable
	}
	directory, err := os.Open(parent)
	if err != nil {
		return ErrVaultUnavailable
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return ErrVaultUnavailable
	}
	return nil
}

func validatePrivateRegularFile(info os.FileInfo) error {
	if info == nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() ||
		info.Mode().Perm() != 0o600 || !ownedByCurrentUser(info) {
		return ErrVaultUnavailable
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Nlink != 1 {
		return ErrVaultUnavailable
	}
	return nil
}

func ownedByCurrentUser(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Geteuid())
}

func sameFileIdentity(left, right os.FileInfo) bool {
	leftStat, leftOK := left.Sys().(*syscall.Stat_t)
	rightStat, rightOK := right.Sys().(*syscall.Stat_t)
	return leftOK && rightOK && leftStat.Dev == rightStat.Dev &&
		leftStat.Ino == rightStat.Ino && rightStat.Nlink == 1 &&
		right.Mode().Perm() == 0o600 && right.Mode().IsRegular() &&
		rightStat.Uid == uint32(os.Geteuid())
}
