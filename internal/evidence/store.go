//go:build !windows

package evidence

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/sys/unix"
)

const (
	privateDirMode  os.FileMode = 0o700
	privateFileMode os.FileMode = 0o600
)

var (
	ErrRootSymlink         = errors.New("evidence root is a symlink")
	ErrRootChanged         = errors.New("evidence root changed")
	ErrInvalidRoot         = errors.New("invalid evidence root")
	ErrStateSymlink        = errors.New("evidence state path is a symlink")
	ErrInvalidDigest       = errors.New("invalid evidence digest")
	ErrDigestMismatch      = errors.New("evidence digest mismatch")
	ErrCorruptArtifact     = errors.New("corrupt evidence artifact")
	ErrShardChanged        = errors.New("evidence shard changed")
	ErrStagingCleanup      = errors.New("evidence staging cleanup failed")
	ErrStoreClosed         = errors.New("evidence store is closed")
	ErrUnsupportedPlatform = errors.New("evidence atomic no-replace publish is unsupported on this platform")
)

type Artifact struct {
	Digest string
}

type Store struct {
	root       string
	sha256Dir  string
	stagingDir string

	rootFD      int
	artifactsFD int
	sha256FD    int
	stagingFD   int
	rootID      fileIdentity

	ops       storeOps
	lifecycle sync.RWMutex
	mu        sync.Mutex
	locks     map[string]*sync.Mutex
	closed    bool
}

type fileIdentity struct {
	dev uint64
	ino uint64
}

type storeOps struct {
	afterLease      func()
	beforeStaging   func()
	beforeRename    func(oldpath, newpath string)
	afterRename     func(oldpath, newpath string)
	unlinkStaging   func(dirFD int, name string) error
	chmodStaging    func(fd int) error
	renameNoReplace func(oldpath, newpath string, fromDirFD int, fromName string, toDirFD int, toName string) error
}

func NewStore(root string) (*Store, error) {
	return newStoreWithOptions(root, storeOps{})
}

func newStoreWithOptions(root string, ops storeOps) (*Store, error) {
	if root == "" {
		return nil, fmt.Errorf("%w: empty root", ErrInvalidRoot)
	}
	if ops.renameNoReplace == nil {
		ops.renameNoReplace = renameNoReplace
	}
	if ops.unlinkStaging == nil {
		ops.unlinkStaging = func(dirFD int, name string) error {
			return unix.Unlinkat(dirFD, name, 0)
		}
	}
	if ops.chmodStaging == nil {
		ops.chmodStaging = func(fd int) error {
			return unix.Fchmod(fd, uint32(privateFileMode))
		}
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	rootFD, rootID, err := openRootDescriptor(absRoot, true)
	if err != nil {
		return nil, err
	}
	store := &Store{
		root:        absRoot,
		sha256Dir:   filepath.Join(absRoot, "artifacts", "sha256"),
		stagingDir:  filepath.Join(absRoot, "artifacts", "staging"),
		rootFD:      rootFD,
		rootID:      rootID,
		ops:         ops,
		locks:       make(map[string]*sync.Mutex),
		artifactsFD: -1,
		sha256FD:    -1,
		stagingFD:   -1,
	}
	if err := store.openManagedDirs(); err != nil {
		_ = store.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) openManagedDirs() error {
	artifactsFD, err := ensureDirAt(s.rootFD, "artifacts")
	if err != nil {
		return err
	}
	s.artifactsFD = artifactsFD
	sha256FD, err := ensureDirAt(artifactsFD, "sha256")
	if err != nil {
		return err
	}
	s.sha256FD = sha256FD
	stagingFD, err := ensureDirAt(artifactsFD, "staging")
	if err != nil {
		return err
	}
	s.stagingFD = stagingFD
	return nil
}

func (s *Store) Close() error {
	s.lifecycle.Lock()
	defer s.lifecycle.Unlock()

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	fds := []int{s.stagingFD, s.sha256FD, s.artifactsFD, s.rootFD}
	s.stagingFD, s.sha256FD, s.artifactsFD, s.rootFD = -1, -1, -1, -1
	s.mu.Unlock()

	var closeErr error
	for _, fd := range fds {
		if fd >= 0 {
			if err := unix.Close(fd); err != nil && closeErr == nil {
				closeErr = err
			}
		}
	}
	return closeErr
}

func (s *Store) Publish(ctx context.Context, source io.Reader, expectedDigest string) (Artifact, error) {
	s.lifecycle.RLock()
	defer s.lifecycle.RUnlock()
	if s.ops.afterLease != nil {
		s.ops.afterLease()
	}

	if err := ctx.Err(); err != nil {
		return Artifact{}, err
	}
	if err := validateDigest(expectedDigest); err != nil {
		return Artifact{}, err
	}
	unlock, err := s.lockDigest(expectedDigest)
	if err != nil {
		return Artifact{}, err
	}
	defer unlock()

	if s.ops.beforeStaging != nil {
		s.ops.beforeStaging()
	}
	if err := s.revalidateRoot(); err != nil {
		return Artifact{}, err
	}

	stagingDirFD := s.stagingFD
	stagingName, stagingPath, staging, err := s.createStagingFile(stagingDirFD)
	if err != nil {
		return Artifact{}, err
	}
	cleanup := true
	cleanupStaging := func(baseErr error) error {
		if !cleanup {
			return baseErr
		}
		if cleanupErr := s.ops.unlinkStaging(stagingDirFD, stagingName); cleanupErr != nil {
			return errors.Join(baseErr, fmt.Errorf("%w: %w", ErrStagingCleanup, cleanupErr))
		}
		return baseErr
	}

	actualDigest, err := copyAndHash(ctx, staging, source)
	if closeErr := staging.Close(); err == nil && closeErr != nil {
		err = closeErr
	}
	if err != nil {
		return Artifact{}, cleanupStaging(err)
	}
	if actualDigest != expectedDigest {
		return Artifact{}, cleanupStaging(fmt.Errorf("%w: got %s want %s", ErrDigestMismatch, actualDigest, expectedDigest))
	}
	if err := ctx.Err(); err != nil {
		return Artifact{}, cleanupStaging(err)
	}

	shardName := expectedDigest[:2]
	shardFD, err := ensureDirAt(s.sha256FD, shardName)
	if err != nil {
		return Artifact{}, cleanupStaging(err)
	}
	defer unix.Close(shardFD)
	shardID, err := identityForFD(shardFD)
	if err != nil {
		return Artifact{}, cleanupStaging(err)
	}
	targetPath := artifactPathForRoot(s.root, expectedDigest)
	if found, err := verifyExistingArtifactAt(ctx, shardFD, expectedDigest); err != nil {
		return Artifact{}, cleanupStaging(err)
	} else if found {
		return Artifact{Digest: expectedDigest}, cleanupStaging(nil)
	}
	if err := ctx.Err(); err != nil {
		return Artifact{}, cleanupStaging(err)
	}

	if s.ops.beforeRename != nil {
		s.ops.beforeRename(stagingPath, targetPath)
	}
	if err := s.revalidateRoot(); err != nil {
		return Artifact{}, cleanupStaging(err)
	}
	if err := revalidateShard(s.sha256FD, shardName, shardID); err != nil {
		return Artifact{}, cleanupStaging(err)
	}
	if err := s.ops.renameNoReplace(stagingPath, targetPath, stagingDirFD, stagingName, shardFD, expectedDigest); err != nil {
		if errors.Is(err, os.ErrExist) || errors.Is(err, unix.EEXIST) {
			if found, verifyErr := verifyExistingArtifactAt(ctx, shardFD, expectedDigest); verifyErr != nil {
				return Artifact{}, cleanupStaging(verifyErr)
			} else if found {
				return Artifact{Digest: expectedDigest}, cleanupStaging(nil)
			}
		}
		return Artifact{}, cleanupStaging(err)
	}
	cleanup = false
	if s.ops.afterRename != nil {
		s.ops.afterRename(stagingPath, targetPath)
	}

	return Artifact{Digest: expectedDigest}, nil
}

func (s *Store) revalidateRoot() error {
	fd, id, err := openRootDescriptor(s.root, false)
	if err == nil {
		_ = unix.Close(fd)
		if id == s.rootID {
			return nil
		}
	}
	return ErrRootChanged
}

func revalidateShard(sha256FD int, shardName string, want fileIdentity) error {
	fd, err := unix.Openat(sha256FD, shardName, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		if errors.Is(err, unix.ENOENT) || errors.Is(err, unix.ELOOP) || errors.Is(err, unix.ENOTDIR) {
			return ErrShardChanged
		}
		return err
	}
	defer unix.Close(fd)

	got, err := identityForFD(fd)
	if err != nil {
		return err
	}
	if got != want {
		return ErrShardChanged
	}
	return nil
}

func openRootDescriptor(path string, create bool) (int, fileIdentity, error) {
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		return -1, fileIdentity{}, fmt.Errorf("evidence root must be absolute after normalization: %s", path)
	}
	if clean == string(filepath.Separator) {
		return -1, fileIdentity{}, fmt.Errorf("evidence root cannot be filesystem root")
	}
	components := strings.Split(strings.TrimPrefix(clean, string(filepath.Separator)), string(filepath.Separator))
	parentFD, err := unix.Open(string(filepath.Separator), unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, fileIdentity{}, err
	}
	currentFD := parentFD
	for i, component := range components {
		last := i == len(components)-1
		nextFD, err := unix.Openat(currentFD, component, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if errors.Is(err, unix.ENOENT) && create && last {
			if mkdirErr := unix.Mkdirat(currentFD, component, uint32(privateDirMode)); mkdirErr != nil && !errors.Is(mkdirErr, unix.EEXIST) {
				_ = unix.Close(currentFD)
				return -1, fileIdentity{}, mkdirErr
			}
			nextFD, err = unix.Openat(currentFD, component, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		}
		if errors.Is(err, unix.ELOOP) || errors.Is(err, unix.ENOTDIR) {
			_ = unix.Close(currentFD)
			return -1, fileIdentity{}, ErrRootSymlink
		}
		if err != nil {
			_ = unix.Close(currentFD)
			return -1, fileIdentity{}, err
		}
		_ = unix.Close(currentFD)
		currentFD = nextFD
		if last {
			if err := unix.Fchmod(currentFD, uint32(privateDirMode)); err != nil {
				_ = unix.Close(currentFD)
				return -1, fileIdentity{}, err
			}
			id, err := identityForFD(currentFD)
			if err != nil {
				_ = unix.Close(currentFD)
				return -1, fileIdentity{}, err
			}
			return currentFD, id, nil
		}
	}
	return -1, fileIdentity{}, fmt.Errorf("empty evidence root")
}

func identityForFD(fd int) (fileIdentity, error) {
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return fileIdentity{}, err
	}
	return fileIdentity{dev: uint64(stat.Dev), ino: uint64(stat.Ino)}, nil
}

func ensureDirAt(parentFD int, name string) (int, error) {
	fd, err := openDirAt(parentFD, name)
	if err == nil {
		return fd, nil
	}
	if errors.Is(err, unix.ENOENT) {
		if err := unix.Mkdirat(parentFD, name, uint32(privateDirMode)); err != nil && !errors.Is(err, unix.EEXIST) {
			return -1, err
		}
		return openDirAt(parentFD, name)
	}
	return -1, err
}

func openDirAt(parentFD int, name string) (int, error) {
	fd, err := unix.Openat(parentFD, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if errors.Is(err, unix.ELOOP) || errors.Is(err, unix.ENOTDIR) {
		return -1, fmt.Errorf("%w: %s", ErrStateSymlink, name)
	}
	if err != nil {
		return -1, err
	}
	if err := unix.Fchmod(fd, uint32(privateDirMode)); err != nil {
		_ = unix.Close(fd)
		return -1, err
	}
	return fd, nil
}

func (s *Store) createStagingFile(stagingDirFD int) (string, string, *os.File, error) {
	for attempt := 0; attempt < 16; attempt++ {
		name, err := randomStagingName()
		if err != nil {
			return "", "", nil, err
		}
		fd, err := unix.Openat(stagingDirFD, name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, uint32(privateFileMode))
		if errors.Is(err, unix.EEXIST) {
			continue
		}
		if err != nil {
			return "", "", nil, err
		}
		if err := s.ops.chmodStaging(fd); err != nil {
			_ = unix.Close(fd)
			if cleanupErr := s.ops.unlinkStaging(stagingDirFD, name); cleanupErr != nil {
				return "", "", nil, errors.Join(err, fmt.Errorf("%w: %w", ErrStagingCleanup, cleanupErr))
			}
			return "", "", nil, err
		}
		path := filepath.Join(s.stagingDir, name)
		return name, path, os.NewFile(uintptr(fd), path), nil
	}
	return "", "", nil, fmt.Errorf("create evidence staging file: exhausted unique names")
}

func randomStagingName() (string, error) {
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", err
	}
	return "artifact-" + hex.EncodeToString(token[:]), nil
}

func (s *Store) lockDigest(digest string) (func(), error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, ErrStoreClosed
	}
	lock := s.locks[digest]
	if lock == nil {
		lock = &sync.Mutex{}
		s.locks[digest] = lock
	}
	s.mu.Unlock()

	lock.Lock()
	return lock.Unlock, nil
}

func validateDigest(digest string) error {
	if len(digest) != sha256.Size*2 {
		return fmt.Errorf("%w: length %d", ErrInvalidDigest, len(digest))
	}
	if strings.ToLower(digest) != digest {
		return fmt.Errorf("%w: not lowercase", ErrInvalidDigest)
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidDigest, err)
	}
	return nil
}

func artifactPathForRoot(root, digest string) string {
	if validateDigest(digest) != nil {
		return ""
	}
	return filepath.Join(root, "artifacts", "sha256", digest[:2], digest)
}

func copyAndHash(ctx context.Context, dst *os.File, src io.Reader) (string, error) {
	hash := sha256.New()
	buf := make([]byte, 32*1024)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, readErr := src.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			if _, err := dst.Write(chunk); err != nil {
				return "", err
			}
			if _, err := hash.Write(chunk); err != nil {
				return "", err
			}
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return "", readErr
		}
	}
	if err := dst.Sync(); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func verifyExistingArtifactAt(ctx context.Context, shardFD int, expectedDigest string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	fd, err := unix.Openat(shardFD, expectedDigest, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if errors.Is(err, unix.ENOENT) {
		return false, nil
	}
	if errors.Is(err, unix.ELOOP) {
		return true, fmt.Errorf("%w: symlink target", ErrCorruptArtifact)
	}
	if err != nil {
		var stat unix.Stat_t
		if statErr := unix.Fstatat(shardFD, expectedDigest, &stat, unix.AT_SYMLINK_NOFOLLOW); statErr == nil {
			if stat.Mode&unix.S_IFMT != unix.S_IFREG {
				return true, fmt.Errorf("%w: non-regular target", ErrCorruptArtifact)
			}
		} else if errors.Is(statErr, unix.ENOENT) {
			return false, nil
		}
		return false, err
	}
	file := os.NewFile(uintptr(fd), expectedDigest)
	defer file.Close()

	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return false, err
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		return true, fmt.Errorf("%w: non-regular target", ErrCorruptArtifact)
	}
	actualDigest, err := hashReader(ctx, file)
	if err != nil {
		return false, err
	}
	if actualDigest != expectedDigest {
		return true, fmt.Errorf("%w: %s", ErrCorruptArtifact, expectedDigest)
	}
	if err := unix.Fchmod(fd, uint32(privateFileMode)); err != nil {
		return false, err
	}
	return true, nil
}

func hashReader(ctx context.Context, src io.Reader) (string, error) {
	hash := sha256.New()
	buf := make([]byte, 32*1024)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, readErr := src.Read(buf)
		if n > 0 {
			if _, err := hash.Write(buf[:n]); err != nil {
				return "", err
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return "", readErr
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
