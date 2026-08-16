package vault

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"io"
	"strings"

	"loom-pi-rebuild/internal/credentials"
)

const (
	CredentialWrapDomain   = "loom/credential-wrap/v1"
	ConversationWrapDomain = "loom/conversation-wrap/v1"
	ExportWrapDomain       = "loom/export-wrap/v1"
	MaximumSecretBytes     = 8_192

	vaultSchemaVersion  = uint16(1)
	vaultCipherVersion  = uint16(1)
	vaultKeyBytes       = 32
	vaultNonceBytes     = 12
	vaultAADDigestBytes = 32
)

var (
	ErrInvalidVaultInput   = errors.New("invalid credential vault input")
	ErrVaultAuthentication = errors.New("credential vault authentication failed")
	ErrVaultUnavailable    = errors.New("credential vault unavailable")
)

type CredentialIdentity struct {
	CredentialReference string `json:"credential_reference"`
	ProviderID          string `json:"provider_id"`
	ProviderAccountID   string `json:"provider_account_id"`
	CredentialRevision  int64  `json:"credential_revision"`
}

type EncryptedCredential struct {
	SchemaVersion uint16
	CipherVersion uint16
	KeyVersion    uint32
	Identity      CredentialIdentity
	WrappedDEK    []byte
	WrapNonce     []byte
	Ciphertext    []byte
	DataNonce     []byte
	AADDigest     []byte
}

type EnvelopeCipher struct {
	random io.Reader
}

func NewEnvelopeCipher(randomSource io.Reader) (*EnvelopeCipher, error) {
	if randomSource == nil {
		randomSource = rand.Reader
	}
	return &EnvelopeCipher{random: randomSource}, nil
}

func DeriveDomainKey(vmk []byte, domain string) ([]byte, error) {
	if len(vmk) != vaultKeyBytes || !validVaultDomain(domain) {
		return nil, ErrInvalidVaultInput
	}
	key, err := hkdf.Key(sha256.New, vmk, nil, domain, vaultKeyBytes)
	if err != nil {
		return nil, ErrVaultUnavailable
	}
	return key, nil
}

func (envelope *EnvelopeCipher) Encrypt(
	vmk []byte,
	keyVersion uint32,
	identity CredentialIdentity,
	secret []byte,
) (EncryptedCredential, error) {
	if envelope == nil || envelope.random == nil || keyVersion == 0 ||
		!validCredentialIdentity(identity) || len(secret) == 0 ||
		len(secret) > MaximumSecretBytes || len(vmk) != vaultKeyBytes {
		return EncryptedCredential{}, ErrInvalidVaultInput
	}
	record := EncryptedCredential{
		SchemaVersion: vaultSchemaVersion,
		CipherVersion: vaultCipherVersion,
		KeyVersion:    keyVersion,
		Identity:      identity,
		WrapNonce:     make([]byte, vaultNonceBytes),
		DataNonce:     make([]byte, vaultNonceBytes),
	}
	dek := make([]byte, vaultKeyBytes)
	defer clearBytes(dek)
	if _, err := io.ReadFull(envelope.random, dek); err != nil {
		return EncryptedCredential{}, ErrVaultUnavailable
	}
	if _, err := io.ReadFull(envelope.random, record.WrapNonce); err != nil {
		return EncryptedCredential{}, ErrVaultUnavailable
	}
	if _, err := io.ReadFull(envelope.random, record.DataNonce); err != nil {
		return EncryptedCredential{}, ErrVaultUnavailable
	}
	if bytes.Equal(record.WrapNonce, record.DataNonce) {
		return EncryptedCredential{}, ErrVaultUnavailable
	}
	aad, err := canonicalCredentialAAD(record)
	if err != nil {
		return EncryptedCredential{}, err
	}
	digest := sha256.Sum256(aad)
	record.AADDigest = append([]byte(nil), digest[:]...)

	kek, err := DeriveDomainKey(vmk, CredentialWrapDomain)
	if err != nil {
		return EncryptedCredential{}, err
	}
	defer clearBytes(kek)
	wrapAEAD, err := newVaultAEAD(kek)
	if err != nil {
		return EncryptedCredential{}, err
	}
	dataAEAD, err := newVaultAEAD(dek)
	if err != nil {
		return EncryptedCredential{}, err
	}
	record.WrappedDEK = wrapAEAD.Seal(nil, record.WrapNonce, dek, aad)
	record.Ciphertext = dataAEAD.Seal(nil, record.DataNonce, secret, aad)
	return record, nil
}

func (envelope *EnvelopeCipher) Decrypt(
	vmk []byte,
	record EncryptedCredential,
) ([]byte, error) {
	if envelope == nil || len(vmk) != vaultKeyBytes {
		return nil, credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultDecrypt,
			ErrInvalidVaultInput,
		)
	}
	if !validCredentialIdentity(record.Identity) {
		return nil, credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultAADValidation,
			ErrVaultAuthentication,
		)
	}
	if !validEncryptedCredential(record) {
		return nil, credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultDecrypt,
			ErrInvalidVaultInput,
		)
	}
	aad, err := canonicalCredentialAAD(record)
	if err != nil {
		return nil, credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultAADValidation,
			ErrVaultAuthentication,
		)
	}
	digest := sha256.Sum256(aad)
	if subtle.ConstantTimeCompare(record.AADDigest, digest[:]) != 1 {
		return nil, credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultAADValidation,
			ErrVaultAuthentication,
		)
	}
	kek, err := DeriveDomainKey(vmk, CredentialWrapDomain)
	if err != nil {
		return nil, credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultDecrypt,
			err,
		)
	}
	defer clearBytes(kek)
	wrapAEAD, err := newVaultAEAD(kek)
	if err != nil {
		return nil, credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultDecrypt,
			err,
		)
	}
	dek, err := wrapAEAD.Open(nil, record.WrapNonce, record.WrappedDEK, aad)
	if err != nil || len(dek) != vaultKeyBytes {
		clearBytes(dek)
		return nil, credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultDecrypt,
			ErrVaultAuthentication,
		)
	}
	defer clearBytes(dek)
	dataAEAD, err := newVaultAEAD(dek)
	if err != nil {
		return nil, credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultDecrypt,
			err,
		)
	}
	plaintext, err := dataAEAD.Open(nil, record.DataNonce, record.Ciphertext, aad)
	if err != nil || len(plaintext) == 0 || len(plaintext) > MaximumSecretBytes {
		clearBytes(plaintext)
		return nil, credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultDecrypt,
			ErrVaultAuthentication,
		)
	}
	return plaintext, nil
}

func (envelope *EnvelopeCipher) RotateWrappingKey(
	oldVMK,
	newVMK []byte,
	newKeyVersion uint32,
	record EncryptedCredential,
) (EncryptedCredential, error) {
	if envelope == nil || envelope.random == nil ||
		len(oldVMK) != vaultKeyBytes || len(newVMK) != vaultKeyBytes ||
		subtle.ConstantTimeCompare(oldVMK, newVMK) == 1 ||
		!validEncryptedCredential(record) ||
		newKeyVersion <= record.KeyVersion {
		return EncryptedCredential{}, ErrInvalidVaultInput
	}
	oldAAD, err := canonicalCredentialAAD(record)
	if err != nil {
		return EncryptedCredential{}, err
	}
	oldDigest := sha256.Sum256(oldAAD)
	if subtle.ConstantTimeCompare(record.AADDigest, oldDigest[:]) != 1 {
		return EncryptedCredential{}, ErrVaultAuthentication
	}
	oldKEK, err := DeriveDomainKey(oldVMK, CredentialWrapDomain)
	if err != nil {
		return EncryptedCredential{}, err
	}
	defer clearBytes(oldKEK)
	oldWrapAEAD, err := newVaultAEAD(oldKEK)
	if err != nil {
		return EncryptedCredential{}, err
	}
	dek, err := oldWrapAEAD.Open(
		nil, record.WrapNonce, record.WrappedDEK, oldAAD,
	)
	if err != nil || len(dek) != vaultKeyBytes {
		clearBytes(dek)
		return EncryptedCredential{}, ErrVaultAuthentication
	}
	defer clearBytes(dek)

	rotated := EncryptedCredential{
		SchemaVersion: record.SchemaVersion,
		CipherVersion: record.CipherVersion,
		KeyVersion:    newKeyVersion,
		Identity:      record.Identity,
		WrapNonce:     make([]byte, vaultNonceBytes),
		DataNonce:     append([]byte(nil), record.DataNonce...),
		Ciphertext:    append([]byte(nil), record.Ciphertext...),
		AADDigest:     append([]byte(nil), record.AADDigest...),
	}
	if _, err := io.ReadFull(envelope.random, rotated.WrapNonce); err != nil {
		return EncryptedCredential{}, ErrVaultUnavailable
	}
	if bytes.Equal(rotated.WrapNonce, rotated.DataNonce) {
		return EncryptedCredential{}, ErrVaultUnavailable
	}
	newAAD, err := canonicalCredentialAAD(rotated)
	if err != nil {
		return EncryptedCredential{}, err
	}
	if !bytes.Equal(newAAD, oldAAD) {
		return EncryptedCredential{}, ErrVaultAuthentication
	}
	newKEK, err := DeriveDomainKey(newVMK, CredentialWrapDomain)
	if err != nil {
		return EncryptedCredential{}, err
	}
	defer clearBytes(newKEK)
	newWrapAEAD, err := newVaultAEAD(newKEK)
	if err != nil {
		return EncryptedCredential{}, err
	}
	rotated.WrappedDEK = newWrapAEAD.Seal(
		nil, rotated.WrapNonce, dek, newAAD,
	)
	return rotated, nil
}

func validEncryptedCredential(record EncryptedCredential) bool {
	return record.SchemaVersion == vaultSchemaVersion &&
		record.CipherVersion == vaultCipherVersion && record.KeyVersion > 0 &&
		validCredentialIdentity(record.Identity) &&
		len(record.WrapNonce) == vaultNonceBytes &&
		len(record.DataNonce) == vaultNonceBytes &&
		len(record.AADDigest) == vaultAADDigestBytes &&
		len(record.WrappedDEK) == vaultKeyBytes+16 &&
		len(record.Ciphertext) > 16 &&
		!bytes.Equal(record.WrapNonce, record.DataNonce)
}

func canonicalCredentialAAD(record EncryptedCredential) ([]byte, error) {
	if record.SchemaVersion != vaultSchemaVersion ||
		record.CipherVersion != vaultCipherVersion || record.KeyVersion == 0 ||
		!validCredentialIdentity(record.Identity) {
		return nil, ErrInvalidVaultInput
	}
	var body bytes.Buffer
	body.WriteString("LOOM-CREDENTIAL-AAD")
	_ = binary.Write(&body, binary.BigEndian, record.SchemaVersion)
	writeCanonicalString(&body, record.Identity.CredentialReference)
	writeCanonicalString(&body, record.Identity.ProviderID)
	writeCanonicalString(&body, record.Identity.ProviderAccountID)
	_ = binary.Write(&body, binary.BigEndian, record.Identity.CredentialRevision)
	_ = binary.Write(&body, binary.BigEndian, record.CipherVersion)
	return body.Bytes(), nil
}

func writeCanonicalString(body *bytes.Buffer, value string) {
	_ = binary.Write(body, binary.BigEndian, uint32(len(value)))
	body.WriteString(value)
}

func newVaultAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrVaultUnavailable
	}
	aead, err := cipher.NewGCM(block)
	if err != nil || aead.NonceSize() != vaultNonceBytes {
		return nil, ErrVaultUnavailable
	}
	return aead, nil
}

func validVaultDomain(domain string) bool {
	switch domain {
	case CredentialWrapDomain, ConversationWrapDomain, ExportWrapDomain:
		return true
	default:
		return false
	}
}

func validCredentialIdentity(identity CredentialIdentity) bool {
	if !credentials.ValidProviderIdentifier(identity.ProviderID) ||
		!credentials.ValidProviderAccountIdentifier(
			identity.ProviderID,
			identity.ProviderAccountID,
		) || identity.CredentialRevision <= 0 ||
		!strings.HasPrefix(identity.CredentialReference, "credential-ref-") ||
		len(identity.CredentialReference) > 128 ||
		len(identity.CredentialReference) <= len("credential-ref-") {
		return false
	}
	for _, character := range identity.CredentialReference[len("credential-ref-"):] {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func cloneEncryptedCredential(record EncryptedCredential) EncryptedCredential {
	clone := record
	clone.WrappedDEK = append([]byte(nil), record.WrappedDEK...)
	clone.WrapNonce = append([]byte(nil), record.WrapNonce...)
	clone.Ciphertext = append([]byte(nil), record.Ciphertext...)
	clone.DataNonce = append([]byte(nil), record.DataNonce...)
	clone.AADDigest = append([]byte(nil), record.AADDigest...)
	return clone
}

func clearBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
