package vault

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"loom-pi-rebuild/internal/credentials"
)

var _ credentials.SecretStore = (*VaultStore)(nil)

func TestVaultStoreRoundTripRestartAndNoPlaintextAtRest(t *testing.T) {
	root := t.TempDir()
	privateDir := filepath.Join(root, "private")
	stateDir := filepath.Join(root, "state")
	for _, directory := range []string{privateDir, stateDir} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	keyPath := filepath.Join(privateDir, "vault.key")
	databasePath := filepath.Join(stateDir, "credential-vault.db")
	identity := CredentialIdentity{
		CredentialReference: "credential-ref-deepseek-primary",
		ProviderID:          "deepseek", ProviderAccountID: "deepseek.primary",
		CredentialRevision: 4,
	}
	resolver := func(_ context.Context, reference string) (CredentialIdentity, error) {
		if reference != identity.CredentialReference {
			return CredentialIdentity{}, credentials.ErrCredentialNotFound
		}
		return identity, nil
	}
	material, err := (LocalKeyFile{Path: keyPath}).LoadOrCreate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(StoreConfig{
		DatabasePath: databasePath, KeyMaterial: material,
		IdentityResolver: resolver,
	})
	if err != nil {
		t.Fatal(err)
	}
	secret := []byte("test-only-secret-must-not-appear-in-db")
	if err := store.Put(context.Background(), identity.CredentialReference, secret); err != nil {
		t.Fatal(err)
	}
	plaintext, err := store.Read(context.Background(), identity.CredentialReference)
	if err != nil || !bytes.Equal(plaintext, secret) {
		t.Fatalf("read = %q, %v", plaintext, err)
	}
	clearBytes(plaintext)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	database, err := os.ReadFile(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(database, secret) {
		t.Fatal("Vault database contains plaintext secret")
	}
	info, err := os.Stat(databasePath)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("database mode = %v, %v", info, err)
	}

	reopenedMaterial, err := (LocalKeyFile{Path: keyPath}).LoadOrCreate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(StoreConfig{
		DatabasePath: databasePath, KeyMaterial: reopenedMaterial,
		IdentityResolver: resolver,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	plaintext, err = reopened.ReadCredential(context.Background(), identity)
	if err != nil || !bytes.Equal(plaintext, secret) {
		t.Fatalf("restart read = %q, %v", plaintext, err)
	}
	clearBytes(plaintext)
}

func TestVaultStoreRejectsAccountRevisionSubstitutionAndIsolatesRecords(t *testing.T) {
	store, identities := newVaultStoreFixture(t)
	defer store.Close()
	firstSecret := []byte("first-account-secret")
	secondSecret := []byte("second-account-secret")
	if err := store.PutCredential(context.Background(), identities[0], firstSecret); err != nil {
		t.Fatal(err)
	}
	if err := store.PutCredential(
		context.Background(), identities[0], []byte("same-revision-replacement"),
	); !errors.Is(err, credentials.ErrCredentialMetadataConflict) {
		t.Fatalf("same-revision replacement error = %v", err)
	}
	if err := store.PutCredential(context.Background(), identities[1], secondSecret); err != nil {
		t.Fatal(err)
	}
	for name, identity := range map[string]CredentialIdentity{
		"account": {
			CredentialReference: identities[0].CredentialReference,
			ProviderID:          "deepseek", ProviderAccountID: "deepseek.backup",
			CredentialRevision: identities[0].CredentialRevision,
		},
		"revision": {
			CredentialReference: identities[0].CredentialReference,
			ProviderID:          "deepseek", ProviderAccountID: "deepseek.primary",
			CredentialRevision: identities[0].CredentialRevision + 1,
		},
	} {
		t.Run(name, func(t *testing.T) {
			if plaintext, err := store.ReadCredential(context.Background(), identity); err == nil {
				clearBytes(plaintext)
				t.Fatal("binding substitution read succeeded")
			}
		})
	}
	if err := store.DeleteCredential(context.Background(), identities[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadCredential(context.Background(), identities[0]); !errors.Is(err, credentials.ErrCredentialNotFound) {
		t.Fatalf("deleted read error = %v", err)
	}
	plaintext, err := store.ReadCredential(context.Background(), identities[1])
	if err != nil || !bytes.Equal(plaintext, secondSecret) {
		t.Fatalf("isolated record read = %q, %v", plaintext, err)
	}
	clearBytes(plaintext)
}

func TestVaultStoreHasCredentialUsesExactEncryptedIdentity(t *testing.T) {
	store, identities := newVaultStoreFixture(t)
	defer store.Close()
	identity := identities[0]
	available, err := store.HasCredential(context.Background(), identity)
	if err != nil || available {
		t.Fatalf("empty Vault availability = %v, %v", available, err)
	}
	if err := store.PutCredential(
		context.Background(), identity, []byte("availability-secret"),
	); err != nil {
		t.Fatal(err)
	}
	available, err = store.HasCredential(context.Background(), identity)
	if err != nil || !available {
		t.Fatalf("stored Vault availability = %v, %v", available, err)
	}
	substituted := identity
	substituted.ProviderAccountID = "deepseek.backup"
	available, err = store.HasCredential(context.Background(), substituted)
	if err != nil || available {
		t.Fatalf("substituted account availability = %v, %v", available, err)
	}
	substituted = identity
	substituted.CredentialRevision++
	available, err = store.HasCredential(context.Background(), substituted)
	if err != nil || available {
		t.Fatalf("substituted revision availability = %v, %v", available, err)
	}
}

func TestVaultStoreRejectsRecordKeyVersionDrift(t *testing.T) {
	store, identities := newVaultStoreFixture(t)
	defer store.Close()
	identity := identities[0]
	if err := store.PutCredential(
		context.Background(), identity, []byte("key-version-secret"),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := store.database.Exec(
		`UPDATE encrypted_credentials SET key_version = key_version + 1
		  WHERE credential_reference = ?`,
		identity.CredentialReference,
	); err != nil {
		t.Fatal(err)
	}
	plaintext, err := store.ReadCredential(context.Background(), identity)
	clearBytes(plaintext)
	if !errors.Is(err, credentials.ErrCredentialStoreUnavailable) {
		t.Fatalf("key version drift error = %v", err)
	}
}

func TestVaultStorePreservesAADValidationFailureStage(t *testing.T) {
	store, identities := newVaultStoreFixture(t)
	defer store.Close()
	identity := identities[0]
	if err := store.PutCredential(
		context.Background(), identity, []byte("aad-stage-secret"),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := store.database.Exec(
		`UPDATE encrypted_credentials SET aad_digest = zeroblob(32)
		  WHERE credential_reference = ?`,
		identity.CredentialReference,
	); err != nil {
		t.Fatal(err)
	}
	plaintext, err := store.ReadCredential(context.Background(), identity)
	clearBytes(plaintext)
	if !errors.Is(err, credentials.ErrCredentialStoreUnavailable) ||
		credentials.CredentialFailureStage(err) !=
			credentials.CredentialStageVaultAADValidation {
		t.Fatalf("AAD validation error = %v", err)
	}
}

func TestVaultStoreRejectsChangedLocalKeyAgainstExistingDatabase(t *testing.T) {
	store, identities := newVaultStoreFixture(t)
	root := filepath.Dir(filepath.Dir(store.databasePath))
	keyPath := filepath.Join(root, "private", "vault.key")
	databasePath := store.databasePath
	if err := store.PutCredential(context.Background(), identities[0], []byte("secret")); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	replacementPath := filepath.Join(root, "private", "replacement.key")
	replacement, err := (LocalKeyFile{Path: replacementPath}).LoadOrCreate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	replacement.Close()
	if err := os.Rename(replacementPath, keyPath); err != nil {
		t.Fatal(err)
	}
	material, err := (LocalKeyFile{Path: keyPath}).LoadOrCreate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if reopened, err := OpenStore(StoreConfig{
		DatabasePath: databasePath, KeyMaterial: material,
	}); err == nil {
		reopened.Close()
		t.Fatal("changed Vault key accepted")
	}
}

func TestVaultStoreRejectsUnsafeDatabaseFiles(t *testing.T) {
	for name, prepare := range map[string]func(string, string) error{
		"world_readable": func(_ string, databasePath string) error {
			return os.WriteFile(databasePath, nil, 0o644)
		},
		"symlink": func(root, databasePath string) error {
			target := filepath.Join(root, "database-target")
			if err := os.WriteFile(target, nil, 0o600); err != nil {
				return err
			}
			return os.Symlink(target, databasePath)
		},
		"hardlink": func(root, databasePath string) error {
			target := filepath.Join(root, "database-target")
			if err := os.WriteFile(target, nil, 0o600); err != nil {
				return err
			}
			return os.Link(target, databasePath)
		},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			privateDir := filepath.Join(root, "private")
			stateDir := filepath.Join(root, "state")
			for _, directory := range []string{privateDir, stateDir} {
				if err := os.Mkdir(directory, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			databasePath := filepath.Join(stateDir, "credential-vault.db")
			if err := prepare(root, databasePath); err != nil {
				t.Fatal(err)
			}
			material, err := (LocalKeyFile{
				Path: filepath.Join(privateDir, "vault.key"),
			}).LoadOrCreate(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if store, err := OpenStore(StoreConfig{
				DatabasePath: databasePath, KeyMaterial: material,
			}); err == nil {
				store.Close()
				t.Fatal("unsafe database accepted")
			}
		})
	}
}

func TestVaultStoreRejectsDatabasePathReplacementAfterOpen(t *testing.T) {
	store, identities := newVaultStoreFixture(t)
	defer store.Close()
	if err := store.PutCredential(
		context.Background(), identities[0], []byte("secret"),
	); err != nil {
		t.Fatal(err)
	}
	moved := store.databasePath + ".moved"
	if err := os.Rename(store.databasePath, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.databasePath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if secret, err := store.ReadCredential(
		context.Background(), identities[0],
	); !errors.Is(err, credentials.ErrCredentialStoreUnavailable) {
		clearBytes(secret)
		t.Fatalf("replaced database read error = %v", err)
	}
}

func TestVaultStoreRejectsKeyFilePathReplacementAfterOpen(t *testing.T) {
	store, identities := newVaultStoreFixture(t)
	defer store.Close()
	identity := identities[0]
	if err := store.PutCredential(
		context.Background(), identity, []byte("live-key-identity-secret"),
	); err != nil {
		t.Fatal(err)
	}
	root := filepath.Dir(filepath.Dir(store.databasePath))
	keyPath := filepath.Join(root, "private", "vault.key")
	moved := keyPath + ".moved"
	if err := os.Rename(keyPath, moved); err != nil {
		t.Fatal(err)
	}
	replacement, err := (LocalKeyFile{Path: keyPath}).LoadOrCreate(
		context.Background(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := replacement.Close(); err != nil {
		t.Fatal(err)
	}
	if secret, err := store.ReadCredential(
		context.Background(), identity,
	); !errors.Is(err, credentials.ErrCredentialStoreUnavailable) {
		clearBytes(secret)
		t.Fatalf("replaced key read error = %v", err)
	}
	if available, err := store.HasCredential(
		context.Background(), identity,
	); available || !errors.Is(err, credentials.ErrCredentialStoreUnavailable) {
		t.Fatalf("replaced key availability = %v, %v", available, err)
	}
}

func TestVaultStoreRotatesWrappingKeyAcrossAccountsAndRestart(t *testing.T) {
	store, identities := newVaultStoreFixture(t)
	root := filepath.Dir(filepath.Dir(store.databasePath))
	keyPath := filepath.Join(root, "private", "vault.key")
	pendingKeyPath := filepath.Join(root, "private", "vault.key.rotation-pending")
	secrets := [][]byte{[]byte("rotation-primary"), []byte("rotation-backup")}
	for index, identity := range identities {
		if err := store.PutCredential(
			context.Background(), identity, secrets[index],
		); err != nil {
			t.Fatal(err)
		}
	}
	newMaterial, err := (LocalKeyFile{Path: pendingKeyPath}).Create(
		context.Background(), 2,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RotateWrappingKey(context.Background(), newMaterial); err != nil {
		newMaterial.Close()
		t.Fatal(err)
	}
	for index, identity := range identities {
		assertVaultCredential(t, store, identity, secrets[index])
	}
	if err := os.Rename(pendingKeyPath, keyPath); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopenedMaterial, err := (LocalKeyFile{Path: keyPath}).LoadOrCreate(
		context.Background(),
	)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(StoreConfig{
		DatabasePath: store.databasePath,
		KeyMaterial:  reopenedMaterial,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	for index, identity := range identities {
		assertVaultCredential(t, reopened, identity, secrets[index])
	}
}

func TestVaultStoreRotationRollsBackBeforeCommitAndRetainsNewKeyOwnership(t *testing.T) {
	store, identities := newVaultStoreFixture(t)
	defer store.Close()
	for index, identity := range identities {
		if err := store.PutCredential(
			context.Background(), identity, []byte{byte('a' + index)},
		); err != nil {
			t.Fatal(err)
		}
	}
	root := filepath.Dir(filepath.Dir(store.databasePath))
	pendingPath := filepath.Join(root, "private", "vault.key.rotation-pending")
	newMaterial, err := (LocalKeyFile{Path: pendingPath}).Create(
		context.Background(), 2,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer newMaterial.Close()
	store.fault = func(stage string) error {
		if stage == vaultFaultRotationBeforeCommit {
			return errors.New("controlled rotation failure")
		}
		return nil
	}
	if err := store.RotateWrappingKey(
		context.Background(), newMaterial,
	); !errors.Is(err, credentials.ErrCredentialStoreUnavailable) {
		t.Fatalf("rotation error = %v", err)
	}
	store.fault = nil
	if _, err := newMaterial.keyCopy(); err != nil {
		t.Fatalf("failed rotation consumed new material: %v", err)
	}
	for index, identity := range identities {
		assertVaultCredential(t, store, identity, []byte{byte('a' + index)})
	}
	var keyVersion uint32
	if err := store.database.QueryRow(
		`SELECT key_version FROM vault_metadata WHERE singleton = 1`,
	).Scan(&keyVersion); err != nil || keyVersion != 1 {
		t.Fatalf("metadata key version = %d, %v", keyVersion, err)
	}
}

func TestVaultStoreRotationRejectsPendingCredentialMutation(t *testing.T) {
	store, identities := newVaultStoreFixture(t)
	defer store.Close()
	receipt, err := store.PrepareCredentialMutation(
		context.Background(), CredentialMutation{
			MutationID: "credential-mutation-rotation-pending-configure",
			Kind:       MutationConfigure,
			Candidate:  identities[0],
			Secret:     []byte("pending-secret"),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer store.RollbackCredentialMutation(context.Background(), receipt)
	root := filepath.Dir(filepath.Dir(store.databasePath))
	newMaterial, err := (LocalKeyFile{
		Path: filepath.Join(root, "private", "vault.key.rotation-pending"),
	}).Create(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	defer newMaterial.Close()
	if err := store.RotateWrappingKey(
		context.Background(), newMaterial,
	); !errors.Is(err, credentials.ErrCredentialMetadataConflict) {
		t.Fatalf("rotation error = %v", err)
	}
}

func TestRecoverLocalKeyRotationPromotesCommittedPendingKey(t *testing.T) {
	store, identities := newVaultStoreFixture(t)
	root := filepath.Dir(filepath.Dir(store.databasePath))
	keyPath := filepath.Join(root, "private", "vault.key")
	pendingPath := filepath.Join(root, "private", "vault.key.rotation-pending")
	secret := []byte("rotation-recovery-secret")
	if err := store.PutCredential(
		context.Background(), identities[0], secret,
	); err != nil {
		t.Fatal(err)
	}
	newMaterial, err := (LocalKeyFile{Path: pendingPath}).Create(
		context.Background(), 2,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RotateWrappingKey(context.Background(), newMaterial); err != nil {
		newMaterial.Close()
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := RecoverLocalKeyRotation(
		context.Background(), store.databasePath, keyPath, pendingPath,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(pendingPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pending key remains after recovery: %v", err)
	}
	material, err := (LocalKeyFile{Path: keyPath}).LoadOrCreate(
		context.Background(),
	)
	if err != nil || material.KeyVersion() != 2 {
		t.Fatalf("recovered key = %v, %v", material, err)
	}
	reopened, err := OpenStore(StoreConfig{
		DatabasePath: store.databasePath, KeyMaterial: material,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	assertVaultCredential(t, reopened, identities[0], secret)
}

func TestRecoverLocalKeyRotationRemovesUncommittedPendingKey(t *testing.T) {
	store, _ := newVaultStoreFixture(t)
	root := filepath.Dir(filepath.Dir(store.databasePath))
	keyPath := filepath.Join(root, "private", "vault.key")
	pendingPath := filepath.Join(root, "private", "vault.key.rotation-pending")
	pending, err := (LocalKeyFile{Path: pendingPath}).Create(
		context.Background(), 2,
	)
	if err != nil {
		t.Fatal(err)
	}
	pending.Close()
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := RecoverLocalKeyRotation(
		context.Background(), store.databasePath, keyPath, pendingPath,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(pendingPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("uncommitted pending key remains: %v", err)
	}
	material, err := (LocalKeyFile{Path: keyPath}).LoadOrCreate(
		context.Background(),
	)
	if err != nil || material.KeyVersion() != 1 {
		t.Fatalf("canonical key = %v, %v", material, err)
	}
	material.Close()
}

func TestVaultStoreMutationPrepareCommitAndRollback(t *testing.T) {
	store, identities := newVaultStoreFixture(t)
	defer store.Close()
	current := identities[0]
	configuredSecret := []byte("configured-secret")
	configured := CredentialMutation{
		MutationID: "credential-mutation-configure-deepseek-primary-r1",
		Kind:       MutationConfigure,
		Candidate:  current,
		Secret:     configuredSecret,
	}
	configureReceipt, err := store.PrepareCredentialMutation(
		context.Background(), configured,
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range configuredSecret {
		if value != 0 {
			t.Fatal("prepared configure secret was not cleared")
		}
	}
	if secret, err := store.ReadCredential(
		context.Background(), current,
	); !errors.Is(err, credentials.ErrCredentialNotFound) {
		clearBytes(secret)
		t.Fatalf("pending configure became readable: %v", err)
	}
	if err := store.CommitCredentialMutation(
		context.Background(), configureReceipt,
	); err != nil {
		t.Fatal(err)
	}
	assertVaultCredential(t, store, current, []byte("configured-secret"))

	replacement := current
	replacement.CredentialRevision++
	replacementSecret := []byte("replacement-secret")
	replaceReceipt, err := store.PrepareCredentialMutation(
		context.Background(), CredentialMutation{
			MutationID: "credential-mutation-replace-deepseek-primary-r2",
			Kind:       MutationReplace,
			Current:    current,
			Candidate:  replacement,
			Secret:     replacementSecret,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range replacementSecret {
		if value != 0 {
			t.Fatal("prepared replacement secret was not cleared")
		}
	}
	assertVaultCredential(t, store, current, []byte("configured-secret"))
	if err := store.RollbackCredentialMutation(
		context.Background(), replaceReceipt,
	); err != nil {
		t.Fatal(err)
	}
	assertVaultCredential(t, store, current, []byte("configured-secret"))
	if secret, err := store.ReadCredential(
		context.Background(), replacement,
	); !errors.Is(err, credentials.ErrCredentialNotFound) {
		clearBytes(secret)
		t.Fatalf("rolled-back replacement became readable: %v", err)
	}
}

func TestVaultStoreRebindAndRevokeAdvanceExactRevision(t *testing.T) {
	store, identities := newVaultStoreFixture(t)
	defer store.Close()
	current := identities[0]
	if err := store.PutCredential(
		context.Background(), current, []byte("verified-secret"),
	); err != nil {
		t.Fatal(err)
	}
	rebound := current
	rebound.CredentialRevision++
	rebindReceipt, err := store.PrepareCredentialMutation(
		context.Background(), CredentialMutation{
			MutationID: "credential-mutation-verify-deepseek-primary-r2",
			Kind:       MutationRebind,
			Current:    current,
			Candidate:  rebound,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	assertVaultCredential(t, store, current, []byte("verified-secret"))
	if err := store.CommitCredentialMutation(
		context.Background(), rebindReceipt,
	); err != nil {
		t.Fatal(err)
	}
	if secret, err := store.ReadCredential(
		context.Background(), current,
	); !errors.Is(err, credentials.ErrCredentialNotFound) {
		clearBytes(secret)
		t.Fatalf("old rebound revision remained readable: %v", err)
	}
	assertVaultCredential(t, store, rebound, []byte("verified-secret"))

	revoked := rebound
	revoked.CredentialRevision++
	revokeReceipt, err := store.PrepareCredentialMutation(
		context.Background(), CredentialMutation{
			MutationID: "credential-mutation-revoke-deepseek-primary-r3",
			Kind:       MutationRevoke,
			Current:    rebound,
			Candidate:  revoked,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	assertVaultCredential(t, store, rebound, []byte("verified-secret"))
	if err := store.CommitCredentialMutation(
		context.Background(), revokeReceipt,
	); err != nil {
		t.Fatal(err)
	}
	if secret, err := store.ReadCredential(
		context.Background(), rebound,
	); !errors.Is(err, credentials.ErrCredentialNotFound) {
		clearBytes(secret)
		t.Fatalf("revoked credential remained readable: %v", err)
	}
}

func TestVaultStorePendingMutationSurvivesRestart(t *testing.T) {
	root := t.TempDir()
	privateDir := filepath.Join(root, "private")
	stateDir := filepath.Join(root, "state")
	for _, directory := range []string{privateDir, stateDir} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	keyPath := filepath.Join(privateDir, "vault.key")
	databasePath := filepath.Join(stateDir, "credential-vault.db")
	identity := CredentialIdentity{
		CredentialReference: "credential-ref-deepseek-primary",
		ProviderID:          "deepseek", ProviderAccountID: "deepseek.primary",
		CredentialRevision: 1,
	}
	material, err := (LocalKeyFile{Path: keyPath}).LoadOrCreate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(StoreConfig{DatabasePath: databasePath, KeyMaterial: material})
	if err != nil {
		t.Fatal(err)
	}
	pendingSecret := []byte("restart-secret-not-plaintext")
	_, err = store.PrepareCredentialMutation(
		context.Background(), CredentialMutation{
			MutationID: "credential-mutation-configure-restart-r1",
			Kind:       MutationConfigure, Candidate: identity,
			Secret: pendingSecret,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	databaseBytes, err := os.ReadFile(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(databaseBytes, []byte("restart-secret-not-plaintext")) {
		t.Fatal("pending mutation persisted plaintext")
	}
	material, err = (LocalKeyFile{Path: keyPath}).LoadOrCreate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	store, err = OpenStore(StoreConfig{DatabasePath: databasePath, KeyMaterial: material})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	pending, err := store.PendingCredentialMutations(context.Background())
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending receipts = %#v, %v", pending, err)
	}
	if err := store.CommitCredentialMutation(context.Background(), pending[0]); err != nil {
		t.Fatal(err)
	}
	assertVaultCredential(t, store, identity, []byte("restart-secret-not-plaintext"))
}

func TestVaultStoreMutationFaultsPreserveAtomicState(t *testing.T) {
	store, identities := newVaultStoreFixture(t)
	defer store.Close()
	current := identities[0]
	store.fault = func(stage string) error {
		if stage == vaultFaultPrepareBeforeCommit {
			return errors.New("injected prepare crash")
		}
		return nil
	}
	secret := []byte("prepare-fault-secret")
	if _, err := store.PrepareCredentialMutation(
		context.Background(), CredentialMutation{
			MutationID: "credential-mutation-configure-prepare-fault-r1",
			Kind:       MutationConfigure, Candidate: current, Secret: secret,
		},
	); !errors.Is(err, credentials.ErrCredentialStoreUnavailable) {
		t.Fatalf("prepare fault error = %v", err)
	}
	assertZeroBytes(t, secret)
	pending, err := store.PendingCredentialMutations(context.Background())
	if err != nil || len(pending) != 0 {
		t.Fatalf("prepare fault pending = %#v, %v", pending, err)
	}

	store.fault = nil
	receipt, err := store.PrepareCredentialMutation(
		context.Background(), CredentialMutation{
			MutationID: "credential-mutation-configure-finalize-fault-r1",
			Kind:       MutationConfigure, Candidate: current,
			Secret: []byte("finalize-fault-secret"),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	store.fault = func(stage string) error {
		if stage == vaultFaultFinalizeBeforeCommit {
			return errors.New("injected finalize crash")
		}
		return nil
	}
	if err := store.CommitCredentialMutation(
		context.Background(), receipt,
	); !errors.Is(err, credentials.ErrCredentialStoreUnavailable) {
		t.Fatalf("finalize fault error = %v", err)
	}
	if secret, err := store.ReadCredential(
		context.Background(), current,
	); !errors.Is(err, credentials.ErrCredentialNotFound) {
		clearBytes(secret)
		t.Fatalf("faulted candidate became active: %v", err)
	}
	pending, err = store.PendingCredentialMutations(context.Background())
	if err != nil || len(pending) != 1 || pending[0] != receipt {
		t.Fatalf("finalize fault pending = %#v, %v", pending, err)
	}
	store.fault = nil
	if err := store.CommitCredentialMutation(context.Background(), receipt); err != nil {
		t.Fatal(err)
	}
	assertVaultCredential(t, store, current, []byte("finalize-fault-secret"))

	replacement := current
	replacement.CredentialRevision++
	replaceReceipt, err := store.PrepareCredentialMutation(
		context.Background(), CredentialMutation{
			MutationID: "credential-mutation-replace-finalize-fault-r2",
			Kind:       MutationReplace, Current: current, Candidate: replacement,
			Secret: []byte("replacement-fault-secret"),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	store.fault = func(stage string) error {
		if stage == vaultFaultFinalizeBeforeCommit {
			return errors.New("injected replacement crash")
		}
		return nil
	}
	if err := store.CommitCredentialMutation(
		context.Background(), replaceReceipt,
	); !errors.Is(err, credentials.ErrCredentialStoreUnavailable) {
		t.Fatalf("replacement fault error = %v", err)
	}
	assertVaultCredential(t, store, current, []byte("finalize-fault-secret"))
	if secret, err := store.ReadCredential(
		context.Background(), replacement,
	); !errors.Is(err, credentials.ErrCredentialNotFound) {
		clearBytes(secret)
		t.Fatalf("faulted replacement became active: %v", err)
	}
	store.fault = nil
	if err := store.RollbackCredentialMutation(
		context.Background(), replaceReceipt,
	); err != nil {
		t.Fatal(err)
	}
}

func assertVaultCredential(
	t *testing.T,
	store *VaultStore,
	identity CredentialIdentity,
	want []byte,
) {
	t.Helper()
	secret, err := store.ReadCredential(context.Background(), identity)
	if err != nil || !bytes.Equal(secret, want) {
		clearBytes(secret)
		t.Fatalf("credential read = %q, %v", secret, err)
	}
	clearBytes(secret)
}

func newVaultStoreFixture(t *testing.T) (*VaultStore, []CredentialIdentity) {
	t.Helper()
	root := t.TempDir()
	privateDir := filepath.Join(root, "private")
	stateDir := filepath.Join(root, "state")
	for _, directory := range []string{privateDir, stateDir} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	material, err := (LocalKeyFile{
		Path: filepath.Join(privateDir, "vault.key"),
	}).LoadOrCreate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(StoreConfig{
		DatabasePath: filepath.Join(stateDir, "credential-vault.db"),
		KeyMaterial:  material,
	})
	if err != nil {
		t.Fatal(err)
	}
	return store, []CredentialIdentity{
		{
			CredentialReference: "credential-ref-deepseek-primary",
			ProviderID:          "deepseek", ProviderAccountID: "deepseek.primary",
			CredentialRevision: 1,
		},
		{
			CredentialReference: "credential-ref-deepseek-backup",
			ProviderID:          "deepseek", ProviderAccountID: "deepseek.backup",
			CredentialRevision: 7,
		},
	}
}
