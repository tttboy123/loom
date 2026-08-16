package vault

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func PublishEncryptedBackup(
	ctx context.Context,
	path string,
	data []byte,
) error {
	if ctx == nil || ctx.Err() != nil || len(data) == 0 ||
		len(data) > maximumBackupBytes || !validBackupDestination(path) {
		return ErrInvalidVaultInput
	}
	parent := filepath.Dir(path)
	info, err := os.Lstat(parent)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() ||
		!ownedByCurrentUser(info) || info.Mode().Perm()&0o022 != 0 {
		return ErrVaultUnavailable
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		return ErrVaultUnavailable
	}
	file, err := os.OpenFile(
		path,
		os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW,
		0o600,
	)
	if err != nil {
		return ErrVaultUnavailable
	}
	remove := true
	defer func() {
		file.Close()
		if remove {
			_ = os.Remove(path)
		}
	}()
	if _, err := file.Write(data); err != nil || file.Sync() != nil ||
		file.Close() != nil {
		return ErrVaultUnavailable
	}
	written, err := os.Lstat(path)
	if err != nil || validatePrivateRegularFile(written) != nil ||
		written.Size() != int64(len(data)) {
		return ErrVaultUnavailable
	}
	directory, err := os.Open(parent)
	if err != nil {
		return ErrVaultUnavailable
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil || closeErr != nil {
		return ErrVaultUnavailable
	}
	remove = false
	return nil
}

func validBackupDestination(path string) bool {
	return filepath.IsAbs(path) && filepath.Clean(path) == path &&
		len(path) <= 1024 && !strings.ContainsAny(path, "\x00\n\r?#") &&
		filepath.Ext(path) == ".loomvault" &&
		filepath.Base(path) != ".loomvault"
}
