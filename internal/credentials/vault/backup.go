package vault

import (
	"bytes"
	"crypto/hkdf"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"slices"

	"golang.org/x/crypto/argon2"
)

const (
	backupSchemaVersion = uint16(1)
	backupCipherVersion = uint16(1)
	backupKDFVersion    = uint16(1)
	backupArgonTime     = uint32(3)
	backupArgonMemory   = uint32(64 * 1024)
	backupArgonThreads  = uint8(4)
	backupSaltBytes     = 16
	maximumBackupBytes  = 4 * 1024 * 1024
	maximumBackupRows   = 256
	minimumPassphrase   = 12
	maximumPassphrase   = 1024

	exportDEKDomain    = ExportWrapDomain + "/dek"
	exportBundleDomain = ExportWrapDomain + "/bundle"
)

type BackupArtifact struct {
	Data            []byte
	Digest          string
	CredentialCount int
}

type encryptedBackupEnvelope struct {
	SchemaVersion uint16 `json:"schema_version"`
	CipherVersion uint16 `json:"cipher_version"`
	KDFVersion    uint16 `json:"kdf_version"`
	ArgonTime     uint32 `json:"argon_time"`
	ArgonMemory   uint32 `json:"argon_memory_kib"`
	ArgonThreads  uint8  `json:"argon_threads"`
	Salt          []byte `json:"salt"`
	Nonce         []byte `json:"nonce"`
	Ciphertext    []byte `json:"ciphertext"`
}

type encryptedBackupPayload struct {
	SchemaVersion uint16                      `json:"schema_version"`
	Credentials   []encryptedBackupCredential `json:"credentials"`
}

type encryptedBackupCredential struct {
	SchemaVersion uint16             `json:"schema_version"`
	CipherVersion uint16             `json:"cipher_version"`
	Identity      CredentialIdentity `json:"identity"`
	WrappedDEK    []byte             `json:"wrapped_dek"`
	WrapNonce     []byte             `json:"wrap_nonce"`
	Ciphertext    []byte             `json:"ciphertext"`
	DataNonce     []byte             `json:"data_nonce"`
	AADDigest     []byte             `json:"aad_digest"`
}

func buildEncryptedBackup(
	passphrase,
	vmk []byte,
	records []EncryptedCredential,
	random io.Reader,
) (BackupArtifact, error) {
	if !validBackupPassphrase(passphrase) || len(vmk) != vaultKeyBytes ||
		len(records) > maximumBackupRows || random == nil {
		return BackupArtifact{}, ErrInvalidVaultInput
	}
	salt := make([]byte, backupSaltBytes)
	if _, err := io.ReadFull(random, salt); err != nil {
		return BackupArtifact{}, ErrVaultUnavailable
	}
	root := argon2.IDKey(
		passphrase, salt, backupArgonTime, backupArgonMemory,
		backupArgonThreads, vaultKeyBytes,
	)
	defer clearBytes(root)
	dekKey, bundleKey, err := deriveExportKeys(root)
	if err != nil {
		return BackupArtifact{}, err
	}
	defer clearBytes(dekKey)
	defer clearBytes(bundleKey)
	payload := encryptedBackupPayload{
		SchemaVersion: backupSchemaVersion,
		Credentials:   make([]encryptedBackupCredential, 0, len(records)),
	}
	seen := make(map[string]struct{}, len(records))
	nonces := make(map[string]struct{}, len(records)+1)
	for _, record := range records {
		if !validEncryptedCredential(record) {
			return BackupArtifact{}, ErrInvalidVaultInput
		}
		identityKey := record.Identity.ProviderAccountID + "\x00" +
			record.Identity.CredentialReference
		if _, duplicate := seen[identityKey]; duplicate {
			return BackupArtifact{}, ErrInvalidVaultInput
		}
		seen[identityKey] = struct{}{}
		entry, err := exportCredentialDEK(vmk, dekKey, record, random, nonces)
		if err != nil {
			return BackupArtifact{}, err
		}
		payload.Credentials = append(payload.Credentials, entry)
	}
	slices.SortFunc(payload.Credentials, func(left, right encryptedBackupCredential) int {
		if value := bytes.Compare(
			[]byte(left.Identity.ProviderAccountID),
			[]byte(right.Identity.ProviderAccountID),
		); value != 0 {
			return value
		}
		return bytes.Compare(
			[]byte(left.Identity.CredentialReference),
			[]byte(right.Identity.CredentialReference),
		)
	})
	plaintext, err := json.Marshal(payload)
	if err != nil || len(plaintext) > maximumBackupBytes {
		clearBytes(plaintext)
		return BackupArtifact{}, ErrVaultUnavailable
	}
	defer clearBytes(plaintext)
	nonce, err := uniqueBackupNonce(random, nonces)
	if err != nil {
		return BackupArtifact{}, err
	}
	aead, err := newVaultAEAD(bundleKey)
	if err != nil {
		return BackupArtifact{}, err
	}
	envelope := encryptedBackupEnvelope{
		SchemaVersion: backupSchemaVersion,
		CipherVersion: backupCipherVersion,
		KDFVersion:    backupKDFVersion,
		ArgonTime:     backupArgonTime, ArgonMemory: backupArgonMemory,
		ArgonThreads: backupArgonThreads, Salt: salt, Nonce: nonce,
	}
	header := canonicalBackupHeader(envelope)
	envelope.Ciphertext = aead.Seal(nil, nonce, plaintext, header)
	data, err := json.Marshal(envelope)
	if err != nil || len(data) > maximumBackupBytes {
		clearBytes(data)
		return BackupArtifact{}, ErrVaultUnavailable
	}
	digest := sha256.Sum256(data)
	return BackupArtifact{
		Data: data, Digest: hex.EncodeToString(digest[:]),
		CredentialCount: len(records),
	}, nil
}

func rewrapEncryptedBackup(
	passphrase,
	data,
	targetVMK []byte,
	targetKeyVersion uint32,
	random io.Reader,
) ([]EncryptedCredential, error) {
	if !validBackupPassphrase(passphrase) || len(data) == 0 ||
		len(data) > maximumBackupBytes || len(targetVMK) != vaultKeyBytes ||
		targetKeyVersion == 0 || random == nil {
		return nil, ErrInvalidVaultInput
	}
	var envelope encryptedBackupEnvelope
	if err := decodeStrictJSON(data, &envelope); err != nil ||
		!validBackupEnvelope(envelope) {
		return nil, ErrVaultAuthentication
	}
	root := argon2.IDKey(
		passphrase, envelope.Salt, envelope.ArgonTime, envelope.ArgonMemory,
		envelope.ArgonThreads, vaultKeyBytes,
	)
	defer clearBytes(root)
	dekKey, bundleKey, err := deriveExportKeys(root)
	if err != nil {
		return nil, err
	}
	defer clearBytes(dekKey)
	defer clearBytes(bundleKey)
	aead, err := newVaultAEAD(bundleKey)
	if err != nil {
		return nil, err
	}
	plaintext, err := aead.Open(
		nil, envelope.Nonce, envelope.Ciphertext,
		canonicalBackupHeader(envelope),
	)
	if err != nil || len(plaintext) > maximumBackupBytes {
		clearBytes(plaintext)
		return nil, ErrVaultAuthentication
	}
	defer clearBytes(plaintext)
	var payload encryptedBackupPayload
	if err := decodeStrictJSON(plaintext, &payload); err != nil ||
		payload.SchemaVersion != backupSchemaVersion ||
		len(payload.Credentials) > maximumBackupRows {
		return nil, ErrVaultAuthentication
	}
	credentialKey, err := DeriveDomainKey(targetVMK, CredentialWrapDomain)
	if err != nil {
		return nil, err
	}
	defer clearBytes(credentialKey)
	output := make([]EncryptedCredential, 0, len(payload.Credentials))
	seen := make(map[string]struct{}, len(payload.Credentials))
	for _, entry := range payload.Credentials {
		record, err := importCredentialDEK(
			dekKey, credentialKey, targetKeyVersion, entry, random,
		)
		if err != nil {
			return nil, err
		}
		key := record.Identity.ProviderAccountID + "\x00" +
			record.Identity.CredentialReference
		if _, duplicate := seen[key]; duplicate {
			return nil, ErrVaultAuthentication
		}
		seen[key] = struct{}{}
		output = append(output, record)
	}
	return output, nil
}

func exportCredentialDEK(
	vmk,
	exportKey []byte,
	record EncryptedCredential,
	random io.Reader,
	nonces map[string]struct{},
) (encryptedBackupCredential, error) {
	aad, err := canonicalCredentialAAD(record)
	if err != nil {
		return encryptedBackupCredential{}, err
	}
	digest := sha256.Sum256(aad)
	if subtle.ConstantTimeCompare(record.AADDigest, digest[:]) != 1 {
		return encryptedBackupCredential{}, ErrVaultAuthentication
	}
	credentialKey, err := DeriveDomainKey(vmk, CredentialWrapDomain)
	if err != nil {
		return encryptedBackupCredential{}, err
	}
	defer clearBytes(credentialKey)
	credentialAEAD, err := newVaultAEAD(credentialKey)
	if err != nil {
		return encryptedBackupCredential{}, err
	}
	dek, err := credentialAEAD.Open(nil, record.WrapNonce, record.WrappedDEK, aad)
	if err != nil || len(dek) != vaultKeyBytes {
		clearBytes(dek)
		return encryptedBackupCredential{}, ErrVaultAuthentication
	}
	defer clearBytes(dek)
	entry := encryptedBackupCredential{
		SchemaVersion: record.SchemaVersion, CipherVersion: record.CipherVersion,
		Identity: record.Identity, Ciphertext: append([]byte(nil), record.Ciphertext...),
		DataNonce: append([]byte(nil), record.DataNonce...),
		AADDigest: append([]byte(nil), record.AADDigest...),
	}
	entry.WrapNonce, err = uniqueBackupNonce(random, nonces)
	if err != nil {
		return encryptedBackupCredential{}, err
	}
	exportAEAD, err := newVaultAEAD(exportKey)
	if err != nil {
		return encryptedBackupCredential{}, err
	}
	entry.WrappedDEK = exportAEAD.Seal(
		nil, entry.WrapNonce, dek, canonicalBackupCredentialAAD(entry),
	)
	return entry, nil
}

func importCredentialDEK(
	exportKey,
	credentialKey []byte,
	targetKeyVersion uint32,
	entry encryptedBackupCredential,
	random io.Reader,
) (EncryptedCredential, error) {
	if !validBackupCredential(entry) {
		return EncryptedCredential{}, ErrVaultAuthentication
	}
	exportAEAD, err := newVaultAEAD(exportKey)
	if err != nil {
		return EncryptedCredential{}, err
	}
	dek, err := exportAEAD.Open(
		nil, entry.WrapNonce, entry.WrappedDEK,
		canonicalBackupCredentialAAD(entry),
	)
	if err != nil || len(dek) != vaultKeyBytes {
		clearBytes(dek)
		return EncryptedCredential{}, ErrVaultAuthentication
	}
	defer clearBytes(dek)
	record := EncryptedCredential{
		SchemaVersion: entry.SchemaVersion, CipherVersion: entry.CipherVersion,
		KeyVersion: targetKeyVersion, Identity: entry.Identity,
		WrapNonce:  make([]byte, vaultNonceBytes),
		Ciphertext: append([]byte(nil), entry.Ciphertext...),
		DataNonce:  append([]byte(nil), entry.DataNonce...),
		AADDigest:  append([]byte(nil), entry.AADDigest...),
	}
	if _, err := io.ReadFull(random, record.WrapNonce); err != nil ||
		bytes.Equal(record.WrapNonce, record.DataNonce) {
		return EncryptedCredential{}, ErrVaultUnavailable
	}
	credentialAEAD, err := newVaultAEAD(credentialKey)
	if err != nil {
		return EncryptedCredential{}, err
	}
	aad, err := canonicalCredentialAAD(record)
	if err != nil {
		return EncryptedCredential{}, err
	}
	record.WrappedDEK = credentialAEAD.Seal(nil, record.WrapNonce, dek, aad)
	dataAEAD, err := newVaultAEAD(dek)
	if err != nil {
		return EncryptedCredential{}, err
	}
	plaintext, err := dataAEAD.Open(nil, record.DataNonce, record.Ciphertext, aad)
	clearBytes(plaintext)
	if err != nil {
		return EncryptedCredential{}, ErrVaultAuthentication
	}
	return record, nil
}

func deriveExportKeys(root []byte) ([]byte, []byte, error) {
	if len(root) != vaultKeyBytes {
		return nil, nil, ErrInvalidVaultInput
	}
	dekKey, err := hkdf.Key(sha256.New, root, nil, exportDEKDomain, vaultKeyBytes)
	if err != nil {
		return nil, nil, ErrVaultUnavailable
	}
	bundleKey, err := hkdf.Key(sha256.New, root, nil, exportBundleDomain, vaultKeyBytes)
	if err != nil {
		clearBytes(dekKey)
		return nil, nil, ErrVaultUnavailable
	}
	return dekKey, bundleKey, nil
}

func canonicalBackupHeader(envelope encryptedBackupEnvelope) []byte {
	var body bytes.Buffer
	body.WriteString("LOOM-VAULT-BACKUP-HEADER")
	_ = binary.Write(&body, binary.BigEndian, envelope.SchemaVersion)
	_ = binary.Write(&body, binary.BigEndian, envelope.CipherVersion)
	_ = binary.Write(&body, binary.BigEndian, envelope.KDFVersion)
	_ = binary.Write(&body, binary.BigEndian, envelope.ArgonTime)
	_ = binary.Write(&body, binary.BigEndian, envelope.ArgonMemory)
	body.WriteByte(envelope.ArgonThreads)
	body.Write(envelope.Salt)
	return body.Bytes()
}

func canonicalBackupCredentialAAD(entry encryptedBackupCredential) []byte {
	record := EncryptedCredential{
		SchemaVersion: entry.SchemaVersion, CipherVersion: entry.CipherVersion,
		KeyVersion: 1, Identity: entry.Identity,
	}
	aad, _ := canonicalCredentialAAD(record)
	ciphertextDigest := sha256.Sum256(entry.Ciphertext)
	var body bytes.Buffer
	body.WriteString("LOOM-VAULT-BACKUP-CREDENTIAL")
	body.Write(aad)
	body.Write(entry.DataNonce)
	body.Write(entry.AADDigest)
	body.Write(ciphertextDigest[:])
	return body.Bytes()
}

func validBackupEnvelope(envelope encryptedBackupEnvelope) bool {
	return envelope.SchemaVersion == backupSchemaVersion &&
		envelope.CipherVersion == backupCipherVersion &&
		envelope.KDFVersion == backupKDFVersion &&
		envelope.ArgonTime == backupArgonTime &&
		envelope.ArgonMemory == backupArgonMemory &&
		envelope.ArgonThreads == backupArgonThreads &&
		len(envelope.Salt) == backupSaltBytes &&
		len(envelope.Nonce) == vaultNonceBytes &&
		len(envelope.Ciphertext) > 16
}

func validBackupCredential(entry encryptedBackupCredential) bool {
	return entry.SchemaVersion == vaultSchemaVersion &&
		entry.CipherVersion == vaultCipherVersion &&
		validCredentialIdentity(entry.Identity) &&
		len(entry.WrappedDEK) == vaultKeyBytes+16 &&
		len(entry.WrapNonce) == vaultNonceBytes &&
		len(entry.DataNonce) == vaultNonceBytes &&
		len(entry.AADDigest) == vaultAADDigestBytes &&
		len(entry.Ciphertext) > 16 &&
		!bytes.Equal(entry.WrapNonce, entry.DataNonce)
}

func validBackupPassphrase(passphrase []byte) bool {
	return len(passphrase) >= minimumPassphrase && len(passphrase) <= maximumPassphrase
}

func ValidEncryptedBackupRequest(passphrase []byte, destination string) bool {
	return validBackupPassphrase(passphrase) && validBackupDestination(destination)
}

func uniqueBackupNonce(random io.Reader, seen map[string]struct{}) ([]byte, error) {
	for range 4 {
		nonce := make([]byte, vaultNonceBytes)
		if _, err := io.ReadFull(random, nonce); err != nil {
			return nil, ErrVaultUnavailable
		}
		key := string(nonce)
		if _, duplicate := seen[key]; !duplicate {
			seen[key] = struct{}{}
			return nonce, nil
		}
	}
	return nil, ErrVaultUnavailable
}

func decodeStrictJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ErrVaultAuthentication
	}
	return nil
}
