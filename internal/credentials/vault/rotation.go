package vault

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
)

// RecoverLocalKeyRotation resolves the only two valid crash states for a
// staged LocalKeyFile rotation. Database metadata is authoritative: a pending
// key is promoted after commit or removed when commit never happened.
func RecoverLocalKeyRotation(
	ctx context.Context,
	databasePath,
	canonicalKeyPath,
	pendingKeyPath string,
) error {
	if ctx == nil || ctx.Err() != nil || !validVaultDatabasePath(databasePath) ||
		!validRotationKeyPaths(canonicalKeyPath, pendingKeyPath) {
		return ErrInvalidVaultInput
	}
	if _, err := os.Lstat(pendingKeyPath); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return ErrVaultUnavailable
	}
	canonical, err := loadLocalKeyMaterial(canonicalKeyPath)
	if err != nil {
		return ErrVaultUnavailable
	}
	defer canonical.Close()
	pending, err := loadLocalKeyMaterial(pendingKeyPath)
	if err != nil {
		return ErrVaultUnavailable
	}
	defer pending.Close()
	keyVersion, keyID, err := readVaultKeyMetadata(ctx, databasePath)
	if err != nil {
		return err
	}
	switch {
	case keyVersion == pending.KeyVersion() && keyID == pending.KeyID():
		return promotePrivateKeyFile(
			canonicalKeyPath, canonical.sourceIdentity,
			pendingKeyPath, pending.sourceIdentity,
		)
	case keyVersion == canonical.KeyVersion() && keyID == canonical.KeyID():
		return removePrivateKeyFile(pendingKeyPath, pending.sourceIdentity)
	default:
		return ErrVaultUnavailable
	}
}

func readVaultKeyMetadata(
	ctx context.Context,
	databasePath string,
) (uint32, [localKeyIDBytes]byte, error) {
	identity, err := capturePrivateFileIdentity(databasePath)
	if err != nil {
		return 0, [localKeyIDBytes]byte{}, ErrVaultUnavailable
	}
	database, err := sql.Open("sqlite", "file:"+databasePath+"?mode=ro")
	if err != nil {
		return 0, [localKeyIDBytes]byte{}, ErrVaultUnavailable
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	defer database.Close()
	if _, err := database.ExecContext(ctx, `PRAGMA query_only=ON`); err != nil {
		return 0, [localKeyIDBytes]byte{}, ErrVaultUnavailable
	}
	var schema int
	var keyVersion uint32
	var storedKeyID []byte
	if err := database.QueryRowContext(
		ctx,
		`SELECT schema_version, key_version, key_id
		   FROM vault_metadata WHERE singleton = 1`,
	).Scan(&schema, &keyVersion, &storedKeyID); err != nil ||
		schema != vaultDatabaseSchema || keyVersion == 0 ||
		len(storedKeyID) != localKeyIDBytes {
		return 0, [localKeyIDBytes]byte{}, ErrVaultUnavailable
	}
	currentIdentity, err := capturePrivateFileIdentity(databasePath)
	if err != nil || currentIdentity != identity {
		return 0, [localKeyIDBytes]byte{}, ErrVaultUnavailable
	}
	var keyID [localKeyIDBytes]byte
	copy(keyID[:], storedKeyID)
	return keyVersion, keyID, nil
}

func validRotationKeyPaths(canonical, pending string) bool {
	return filepath.IsAbs(canonical) && filepath.Clean(canonical) == canonical &&
		filepath.IsAbs(pending) && filepath.Clean(pending) == pending &&
		canonical != pending && filepath.Dir(canonical) == filepath.Dir(pending)
}

func promotePrivateKeyFile(
	canonicalPath string,
	canonicalIdentity privateFileIdentity,
	pendingPath string,
	pendingIdentity privateFileIdentity,
) error {
	if !validRotationKeyPaths(canonicalPath, pendingPath) ||
		!matchesPrivateFileIdentity(canonicalPath, canonicalIdentity) ||
		!matchesPrivateFileIdentity(pendingPath, pendingIdentity) {
		return ErrVaultUnavailable
	}
	if err := os.Rename(pendingPath, canonicalPath); err != nil {
		return ErrVaultUnavailable
	}
	return syncPrivateDirectory(filepath.Dir(canonicalPath))
}

func removePrivateKeyFile(path string, identity privateFileIdentity) error {
	if !matchesPrivateFileIdentity(path, identity) {
		return ErrVaultUnavailable
	}
	if err := os.Remove(path); err != nil {
		return ErrVaultUnavailable
	}
	return syncPrivateDirectory(filepath.Dir(path))
}

func matchesPrivateFileIdentity(path string, expected privateFileIdentity) bool {
	identity, err := capturePrivateFileIdentity(path)
	return err == nil && identity == expected
}

func syncPrivateDirectory(path string) error {
	if err := validatePrivateDirectory(path); err != nil {
		return err
	}
	directory, err := os.Open(path)
	if err != nil {
		return ErrVaultUnavailable
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return ErrVaultUnavailable
	}
	return nil
}
