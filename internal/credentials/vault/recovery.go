package vault

import (
	"context"
	"errors"
	"os"
	"path/filepath"
)

// ResetLocalVault crypto-erases an unrecoverable LocalKeyFile Vault. Callers
// must provide the canonical product paths and obtain explicit user approval.
func ResetLocalVault(
	ctx context.Context,
	databasePath,
	keyPath,
	pendingKeyPath string,
) error {
	if ctx == nil || ctx.Err() != nil ||
		!validRecoveryPaths(databasePath, keyPath, pendingKeyPath) {
		return ErrInvalidVaultInput
	}
	paths := []string{
		keyPath,
		pendingKeyPath,
		databasePath,
		databasePath + "-journal",
		databasePath + "-wal",
		databasePath + "-shm",
	}
	for _, path := range paths {
		if err := validateRecoverablePrivateFile(path); err != nil {
			return err
		}
	}
	for _, path := range paths {
		if err := removeRecoverablePrivateFile(path); err != nil {
			return err
		}
	}
	for _, parent := range []string{
		filepath.Dir(keyPath), filepath.Dir(databasePath),
	} {
		if err := syncPrivateDirectory(parent); err != nil {
			return err
		}
	}
	return nil
}

func validRecoveryPaths(databasePath, keyPath, pendingKeyPath string) bool {
	if !validVaultDatabasePath(databasePath) ||
		!filepath.IsAbs(keyPath) || filepath.Clean(keyPath) != keyPath ||
		!filepath.IsAbs(pendingKeyPath) || filepath.Clean(pendingKeyPath) != pendingKeyPath ||
		filepath.Base(databasePath) != "credential-vault.db" ||
		filepath.Base(keyPath) != "vault.key" ||
		filepath.Base(pendingKeyPath) != "vault.key.rotation-pending" ||
		filepath.Dir(keyPath) != filepath.Dir(pendingKeyPath) {
		return false
	}
	return validatePrivateDirectory(filepath.Dir(databasePath)) == nil &&
		validatePrivateDirectory(filepath.Dir(keyPath)) == nil
}

func validateRecoverablePrivateFile(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || validatePrivateRegularFile(info) != nil {
		return ErrVaultUnavailable
	}
	return nil
}

func removeRecoverablePrivateFile(path string) error {
	before, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || validatePrivateRegularFile(before) != nil {
		return ErrVaultUnavailable
	}
	after, err := os.Lstat(path)
	if err != nil || !sameFileIdentity(before, after) {
		return ErrVaultUnavailable
	}
	if err := os.Remove(path); err != nil {
		return ErrVaultUnavailable
	}
	return nil
}
