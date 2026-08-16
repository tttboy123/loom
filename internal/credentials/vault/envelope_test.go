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

func TestEnvelopeCipherRoundTripAndBindingSubstitutionFailsClosed(t *testing.T) {
	vmk := bytes.Repeat([]byte{0x41}, 32)
	randomInput := append(bytes.Repeat([]byte{0x17}, 32), bytes.Repeat([]byte{0x18}, 12)...)
	randomInput = append(randomInput, bytes.Repeat([]byte{0x19}, 12)...)
	cipher, err := NewEnvelopeCipher(bytes.NewReader(randomInput))
	if err != nil {
		t.Fatal(err)
	}
	identity := CredentialIdentity{
		CredentialReference: "credential-ref-deepseek-primary",
		ProviderID:          "deepseek", ProviderAccountID: "deepseek.primary",
		CredentialRevision: 3,
	}
	secret := []byte("test-only-deepseek-secret")
	record, err := cipher.Encrypt(vmk, 1, identity, secret)
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := cipher.Decrypt(vmk, record)
	if err != nil || !bytes.Equal(plaintext, secret) {
		t.Fatalf("decrypt = %q, %v", plaintext, err)
	}
	clearBytes(plaintext)

	for name, test := range map[string]struct {
		mutate func(*EncryptedCredential)
		stage  string
	}{
		"provider": {mutate: func(candidate *EncryptedCredential) {
			candidate.Identity.ProviderID = "openai"
		}, stage: credentials.CredentialStageVaultAADValidation},
		"account": {mutate: func(candidate *EncryptedCredential) {
			candidate.Identity.ProviderAccountID = "deepseek.backup"
		}, stage: credentials.CredentialStageVaultAADValidation},
		"revision": {mutate: func(candidate *EncryptedCredential) {
			candidate.Identity.CredentialRevision++
		}, stage: credentials.CredentialStageVaultAADValidation},
		"ciphertext": {mutate: func(candidate *EncryptedCredential) {
			candidate.Ciphertext[0] ^= 1
		}, stage: credentials.CredentialStageVaultDecrypt},
		"wrapped_dek": {mutate: func(candidate *EncryptedCredential) {
			candidate.WrappedDEK[0] ^= 1
		}, stage: credentials.CredentialStageVaultDecrypt},
		"unknown_version": {mutate: func(candidate *EncryptedCredential) {
			candidate.CipherVersion++
		}, stage: credentials.CredentialStageVaultDecrypt},
	} {
		t.Run(name, func(t *testing.T) {
			candidate := cloneEncryptedCredential(record)
			test.mutate(&candidate)
			plaintext, err := cipher.Decrypt(vmk, candidate)
			if err == nil {
				clearBytes(plaintext)
				t.Fatal("modified envelope decrypted")
			}
			if stage := credentials.CredentialFailureStage(err); stage != test.stage {
				t.Fatalf("failure stage = %q, want %q: %v", stage, test.stage, err)
			}
		})
	}
}

func TestEnvelopeCipherUsesUniqueNoncesAndRejectsSecretBounds(t *testing.T) {
	cipher, err := NewEnvelopeCipher(nil)
	if err != nil {
		t.Fatal(err)
	}
	identity := CredentialIdentity{
		CredentialReference: "credential-ref-minimax-primary",
		ProviderID:          "minimax", ProviderAccountID: "minimax.primary",
		CredentialRevision: 2,
	}
	vmk := bytes.Repeat([]byte{0x52}, 32)
	first, err := cipher.Encrypt(vmk, 1, identity, []byte("secret-one"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := cipher.Encrypt(vmk, 1, identity, []byte("secret-one"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first.WrapNonce, second.WrapNonce) ||
		bytes.Equal(first.DataNonce, second.DataNonce) ||
		bytes.Equal(first.WrapNonce, first.DataNonce) {
		t.Fatal("AEAD nonce reuse")
	}
	for _, secret := range [][]byte{nil, bytes.Repeat([]byte{'x'}, MaximumSecretBytes+1)} {
		if _, err := cipher.Encrypt(vmk, 1, identity, secret); !errors.Is(err, ErrInvalidVaultInput) {
			t.Fatalf("secret length %d error = %v", len(secret), err)
		}
	}
}

func TestHKDFDomainsAreSeparated(t *testing.T) {
	vmk := bytes.Repeat([]byte{0x63}, 32)
	credential, err := DeriveDomainKey(vmk, CredentialWrapDomain)
	if err != nil {
		t.Fatal(err)
	}
	conversation, err := DeriveDomainKey(vmk, ConversationWrapDomain)
	if err != nil {
		t.Fatal(err)
	}
	export, err := DeriveDomainKey(vmk, ExportWrapDomain)
	if err != nil {
		t.Fatal(err)
	}
	defer clearBytes(credential)
	defer clearBytes(conversation)
	defer clearBytes(export)
	if bytes.Equal(credential, conversation) || bytes.Equal(credential, export) ||
		bytes.Equal(conversation, export) {
		t.Fatal("HKDF domains produced the same key")
	}
}

func TestEnvelopeCipherRotatesWrappingKeyAndPreservesCredentialBinding(
	t *testing.T,
) {
	cipher, err := NewEnvelopeCipher(nil)
	if err != nil {
		t.Fatal(err)
	}
	oldVMK := bytes.Repeat([]byte{0x71}, vaultKeyBytes)
	newVMK := bytes.Repeat([]byte{0x72}, vaultKeyBytes)
	identity := CredentialIdentity{
		CredentialReference: "credential-ref-rotation-primary",
		ProviderID:          "deepseek",
		ProviderAccountID:   "deepseek.primary",
		CredentialRevision:  9,
	}
	secret := []byte("rotation-secret")
	record, err := cipher.Encrypt(oldVMK, 1, identity, secret)
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := cipher.RotateWrappingKey(oldVMK, newVMK, 2, record)
	if err != nil {
		t.Fatal(err)
	}
	if rotated.KeyVersion != 2 || rotated.Identity != identity ||
		bytes.Equal(rotated.WrapNonce, record.WrapNonce) ||
		bytes.Equal(rotated.WrappedDEK, record.WrappedDEK) ||
		!bytes.Equal(rotated.DataNonce, record.DataNonce) ||
		!bytes.Equal(rotated.Ciphertext, record.Ciphertext) ||
		!bytes.Equal(rotated.AADDigest, record.AADDigest) {
		t.Fatalf("rotated envelope = %#v", rotated)
	}
	plaintext, err := cipher.Decrypt(newVMK, rotated)
	if err != nil || !bytes.Equal(plaintext, secret) {
		clearBytes(plaintext)
		t.Fatalf("rotated decrypt = %q, %v", plaintext, err)
	}
	clearBytes(plaintext)
	if plaintext, err := cipher.Decrypt(oldVMK, rotated); err == nil {
		clearBytes(plaintext)
		t.Fatal("old VMK decrypted rotated envelope")
	}
	if _, err := cipher.RotateWrappingKey(
		oldVMK, oldVMK, 2, record,
	); !errors.Is(err, ErrInvalidVaultInput) {
		t.Fatalf("same VMK rotation error = %v", err)
	}
	if _, err := cipher.RotateWrappingKey(
		oldVMK, newVMK, record.KeyVersion, record,
	); !errors.Is(err, ErrInvalidVaultInput) {
		t.Fatalf("non-advancing rotation error = %v", err)
	}
}

func TestLocalKeyFileCreatesPrivateKeyAndRejectsUnsafeFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "vault.key")
	provider := LocalKeyFile{Path: path}
	first, err := provider.LoadOrCreate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("key mode = %v, %v", info, err)
	}
	second, err := provider.LoadOrCreate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if first.KeyVersion() != second.KeyVersion() || first.KeyID() != second.KeyID() {
		t.Fatal("reopened key identity changed")
	}

	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if material, err := provider.LoadOrCreate(context.Background()); err == nil {
		material.Close()
		t.Fatal("world-readable key accepted")
	}
}

func TestLocalKeyFileCreatesExplicitNextVersion(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "vault.next.key")
	material, err := (LocalKeyFile{Path: path}).Create(
		context.Background(), 2,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer material.Close()
	if material.KeyVersion() != 2 {
		t.Fatalf("key version = %d", material.KeyVersion())
	}
	if duplicate, err := (LocalKeyFile{Path: path}).Create(
		context.Background(), 3,
	); err == nil {
		duplicate.Close()
		t.Fatal("explicit key creation replaced an existing key")
	}
}

func TestLocalKeyFileRejectsSymlinkAndHardlink(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "source.key")
	material, err := (LocalKeyFile{Path: target}).LoadOrCreate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	material.Close()
	symlink := filepath.Join(root, "symlink.key")
	if err := os.Symlink(target, symlink); err != nil {
		t.Fatal(err)
	}
	if material, err := (LocalKeyFile{Path: symlink}).LoadOrCreate(context.Background()); err == nil {
		material.Close()
		t.Fatal("symlink key accepted")
	}
	hardlink := filepath.Join(root, "hardlink.key")
	if err := os.Link(target, hardlink); err != nil {
		t.Fatal(err)
	}
	if material, err := (LocalKeyFile{Path: hardlink}).LoadOrCreate(context.Background()); err == nil {
		material.Close()
		t.Fatal("hardlinked key accepted")
	}
}

func TestLocalKeyFileRejectsUnsafeParentAndMalformedExistingKey(t *testing.T) {
	t.Run("unsafe_parent", func(t *testing.T) {
		root := t.TempDir()
		if err := os.Chmod(root, 0o755); err != nil {
			t.Fatal(err)
		}
		if material, err := (LocalKeyFile{
			Path: filepath.Join(root, "vault.key"),
		}).LoadOrCreate(context.Background()); err == nil {
			material.Close()
			t.Fatal("unsafe parent accepted")
		}
	})

	t.Run("malformed_existing_key", func(t *testing.T) {
		root := t.TempDir()
		if err := os.Chmod(root, 0o700); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, "vault.key")
		if err := os.WriteFile(path, []byte("LOOMVLT1-partial"), 0o600); err != nil {
			t.Fatal(err)
		}
		if material, err := (LocalKeyFile{
			Path: path,
		}).LoadOrCreate(context.Background()); err == nil {
			material.Close()
			t.Fatal("malformed key accepted")
		}
	})
}
