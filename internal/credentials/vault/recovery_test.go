package vault

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResetLocalVaultCryptoErasesFilesAndAllowsFreshVault(t *testing.T) {
	root := t.TempDir()
	privateDirectory := filepath.Join(root, "private")
	stateDirectory := filepath.Join(root, "state")
	for _, directory := range []string{privateDirectory, stateDirectory} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	keyPath := filepath.Join(privateDirectory, "vault.key")
	pendingPath := filepath.Join(privateDirectory, "vault.key.rotation-pending")
	databasePath := filepath.Join(stateDirectory, "credential-vault.db")
	material, err := (LocalKeyFile{Path: keyPath}).LoadOrCreate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	oldKeyID := material.KeyID()
	store, err := OpenStore(StoreConfig{
		DatabasePath: databasePath,
		KeyMaterial:  material,
	})
	if err != nil {
		t.Fatal(err)
	}
	identity := CredentialIdentity{
		ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
		CredentialReference: "credential-ref-reset-test", CredentialRevision: 1,
	}
	if err := store.PutCredential(
		context.Background(), identity, []byte("reset-secret-not-for-disk"),
	); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	if err := ResetLocalVault(
		context.Background(), databasePath, keyPath, pendingPath,
	); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{keyPath, pendingPath, databasePath} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("reset path %s remains: %v", path, err)
		}
	}

	material, err = (LocalKeyFile{Path: keyPath}).LoadOrCreate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if material.KeyID() == oldKeyID {
		t.Fatal("recovery reset reused the old Vault master key")
	}
	fresh, err := OpenStore(StoreConfig{
		DatabasePath: databasePath,
		KeyMaterial:  material,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	if available, err := fresh.HasCredential(context.Background(), identity); err != nil || available {
		t.Fatalf("old credential available after reset = %v, %v", available, err)
	}
}

func TestResetLocalVaultRejectsUnsafeFileBeforeCryptoErase(t *testing.T) {
	root := t.TempDir()
	privateDirectory := filepath.Join(root, "private")
	stateDirectory := filepath.Join(root, "state")
	for _, directory := range []string{privateDirectory, stateDirectory} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	keyPath := filepath.Join(privateDirectory, "vault.key")
	pendingPath := filepath.Join(privateDirectory, "vault.key.rotation-pending")
	databasePath := filepath.Join(stateDirectory, "credential-vault.db")
	material, err := (LocalKeyFile{Path: keyPath}).LoadOrCreate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	material.Close()
	if err := os.Symlink(keyPath, pendingPath); err != nil {
		t.Fatal(err)
	}
	if err := ResetLocalVault(
		context.Background(), databasePath, keyPath, pendingPath,
	); !errors.Is(err, ErrVaultUnavailable) {
		t.Fatalf("unsafe reset error = %v", err)
	}
	if _, err := os.Lstat(keyPath); err != nil {
		t.Fatalf("key removed before unsafe path rejection: %v", err)
	}
}
