package vault

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"loom-pi-rebuild/internal/credentials"
)

func TestEncryptedBackupRewrapsDEKsWithoutExposingCredentialPlaintext(
	t *testing.T,
) {
	oldVMK := bytes.Repeat([]byte{0x91}, vaultKeyBytes)
	newVMK := bytes.Repeat([]byte{0x92}, vaultKeyBytes)
	cipher, err := NewEnvelopeCipher(nil)
	if err != nil {
		t.Fatal(err)
	}
	secrets := [][]byte{
		[]byte("deepseek-export-secret-not-plaintext"),
		[]byte("openai-export-secret-not-plaintext"),
	}
	records := make([]EncryptedCredential, 0, len(secrets))
	for index, identity := range []CredentialIdentity{
		{
			ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
			CredentialReference: "credential-ref-backup-deepseek",
			CredentialRevision:  3,
		},
		{
			ProviderID: "openai", ProviderAccountID: "openai.primary",
			CredentialReference: "credential-ref-backup-openai",
			CredentialRevision:  7,
		},
	} {
		record, err := cipher.Encrypt(oldVMK, 4, identity, secrets[index])
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	passphrase := []byte("correct horse battery staple")
	artifact, err := buildEncryptedBackup(
		passphrase, oldVMK, records, cipher.random,
	)
	if err != nil {
		t.Fatal(err)
	}
	if artifact.CredentialCount != 2 || len(artifact.Digest) != 64 {
		t.Fatalf("backup artifact = %#v", artifact)
	}
	for _, secret := range secrets {
		if bytes.Contains(artifact.Data, secret) {
			t.Fatal("backup contains plaintext credential")
		}
	}
	for _, metadata := range [][]byte{[]byte("deepseek"), []byte("openai.primary")} {
		if bytes.Contains(artifact.Data, metadata) {
			t.Fatal("backup exposes Provider Account metadata")
		}
	}
	rewrapped, err := rewrapEncryptedBackup(
		passphrase, artifact.Data, newVMK, 1, cipher.random,
	)
	if err != nil || len(rewrapped) != len(records) {
		t.Fatalf("rewrapped records = %d, %v", len(rewrapped), err)
	}
	for index, record := range rewrapped {
		plaintext, err := cipher.Decrypt(newVMK, record)
		if err != nil || !bytes.Equal(plaintext, secrets[index]) {
			clearBytes(plaintext)
			t.Fatalf("restored credential %d = %q, %v", index, plaintext, err)
		}
		clearBytes(plaintext)
		if plaintext, err := cipher.Decrypt(oldVMK, record); err == nil {
			clearBytes(plaintext)
			t.Fatal("old VMK decrypted restored credential")
		}
	}
}

func TestEncryptedBackupRejectsWrongPassphraseTamperAndUnknownShape(t *testing.T) {
	vmk := bytes.Repeat([]byte{0x93}, vaultKeyBytes)
	cipher, err := NewEnvelopeCipher(nil)
	if err != nil {
		t.Fatal(err)
	}
	record, err := cipher.Encrypt(
		vmk, 1,
		CredentialIdentity{
			ProviderID: "minimax", ProviderAccountID: "minimax.primary",
			CredentialReference: "credential-ref-backup-minimax",
			CredentialRevision:  2,
		},
		[]byte("minimax-backup-secret"),
	)
	if err != nil {
		t.Fatal(err)
	}
	passphrase := []byte("backup passphrase one")
	artifact, err := buildEncryptedBackup(
		passphrase, vmk, []EncryptedCredential{record}, cipher.random,
	)
	if err != nil {
		t.Fatal(err)
	}
	for name, candidate := range map[string][]byte{
		"wrong_passphrase": artifact.Data,
		"tamper":           append([]byte(nil), artifact.Data...),
		"unknown_field": append(
			bytes.TrimSuffix(append([]byte(nil), artifact.Data...), []byte("}")),
			[]byte(`,"unknown":true}`)...,
		),
	} {
		t.Run(name, func(t *testing.T) {
			password := passphrase
			if name == "wrong_passphrase" {
				password = []byte("backup passphrase two")
			}
			if name == "tamper" {
				candidate[len(candidate)/2] ^= 1
			}
			if output, err := rewrapEncryptedBackup(
				password, candidate, vmk, 2, cipher.random,
			); err == nil {
				for index := range output {
					clearBytes(output[index].WrappedDEK)
				}
				t.Fatal("unsafe backup accepted")
			}
		})
	}
}

func TestVaultStoreEncryptedBackupClearsPassphraseAndRejectsPendingMutation(
	t *testing.T,
) {
	store, identities := newVaultStoreFixture(t)
	defer store.Close()
	secret := []byte("store-backup-secret")
	if err := store.PutCredential(context.Background(), identities[0], secret); err != nil {
		t.Fatal(err)
	}
	passphrase := []byte("store backup passphrase")
	artifact, err := store.ExportEncryptedBackup(context.Background(), passphrase)
	if err != nil || artifact.CredentialCount != 1 {
		t.Fatalf("store backup = %#v, %v", artifact, err)
	}
	if !bytes.Equal(passphrase, make([]byte, len(passphrase))) {
		t.Fatal("store backup did not clear caller passphrase")
	}
	receipt, err := store.PrepareCredentialMutation(
		context.Background(), CredentialMutation{
			MutationID: "credential-mutation-backup-pending", Kind: MutationReplace,
			Current: identities[0], Candidate: CredentialIdentity{
				ProviderID:          identities[0].ProviderID,
				ProviderAccountID:   identities[0].ProviderAccountID,
				CredentialReference: identities[0].CredentialReference,
				CredentialRevision:  identities[0].CredentialRevision + 1,
			},
			Secret: []byte("replacement-secret"),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer store.RollbackCredentialMutation(context.Background(), receipt)
	if _, err := store.ExportEncryptedBackup(
		context.Background(), []byte("another backup passphrase"),
	); !errors.Is(err, credentials.ErrCredentialMetadataConflict) {
		t.Fatalf("backup with pending mutation error = %v", err)
	}
}
