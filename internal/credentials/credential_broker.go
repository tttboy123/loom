package credentials

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	ErrInvalidCredentialCommand     = errors.New("invalid credential command")
	ErrCredentialNotFound           = errors.New("credential not found")
	ErrCredentialStoreUnavailable   = errors.New("credential store unavailable")
	ErrCredentialStoreDenied        = errors.New("credential store denied")
	ErrCredentialMetadataConflict   = errors.New("credential metadata conflict")
	ErrCredentialRollbackFailed     = errors.New("credential rollback failed")
	ErrCredentialRejected           = errors.New("credential rejected")
	ErrCredentialHelperProtocol     = errors.New("credential helper protocol")
	ErrCredentialHelperUnauthorized = errors.New("credential helper unauthorized")
)

const (
	CredentialStageHelperValidation    = "helper_validation"
	CredentialStageHelperStart         = "helper_start"
	CredentialStageHelperAuthorization = "helper_authorization"
	CredentialStageHelperRequest       = "helper_request"
	CredentialStageHelperTimeout       = "helper_timeout"
	CredentialStageHelperResponse      = "helper_response"
	CredentialStageHelperExit          = "helper_exit"
	CredentialStageKeychainAccess      = "keychain_access"
	CredentialStageMetadataCommit      = "metadata_commit"
	CredentialStageVaultKeyLoad        = "vault_key_load"
	CredentialStageVaultOpen           = "vault_open"
	CredentialStageVaultEncrypt        = "vault_encrypt"
	CredentialStageVaultCommit         = "vault_commit"
	CredentialStageVaultDecrypt        = "vault_decrypt"
	CredentialStageVaultAADValidation  = "vault_aad_validation"
	CredentialStageVaultRotation       = "vault_rotation"
	CredentialStageVaultRecovery       = "vault_recovery"
	CredentialStageVaultExport         = "vault_export"
	CredentialStageLeaseIssue          = "credential_lease_issue"
	CredentialStageLeaseExpire         = "credential_lease_expire"
	CredentialStageLeaseRevoke         = "credential_lease_revoke"
	CredentialStageMigrationRead       = "migration_read"
	CredentialStageMigrationCommit     = "migration_commit"
	CredentialStageMigrationCleanup    = "migration_cleanup"
)

type credentialFailureStageError struct {
	stage string
	err   error
}

func (failure *credentialFailureStageError) Error() string {
	if failure == nil || failure.err == nil {
		return ErrCredentialStoreUnavailable.Error()
	}
	return failure.err.Error()
}

func (failure *credentialFailureStageError) Unwrap() error {
	if failure == nil {
		return nil
	}
	return failure.err
}

func (failure *credentialFailureStageError) CredentialFailureStage() string {
	if failure == nil {
		return ""
	}
	return failure.stage
}

func validCredentialFailureStage(stage string) bool {
	switch stage {
	case CredentialStageHelperValidation,
		CredentialStageHelperStart,
		CredentialStageHelperAuthorization,
		CredentialStageHelperRequest,
		CredentialStageHelperTimeout,
		CredentialStageHelperResponse,
		CredentialStageHelperExit,
		CredentialStageKeychainAccess,
		CredentialStageMetadataCommit,
		CredentialStageVaultKeyLoad,
		CredentialStageVaultOpen,
		CredentialStageVaultEncrypt,
		CredentialStageVaultCommit,
		CredentialStageVaultDecrypt,
		CredentialStageVaultAADValidation,
		CredentialStageVaultRotation,
		CredentialStageVaultRecovery,
		CredentialStageVaultExport,
		CredentialStageLeaseIssue,
		CredentialStageLeaseExpire,
		CredentialStageLeaseRevoke,
		CredentialStageMigrationRead,
		CredentialStageMigrationCommit,
		CredentialStageMigrationCleanup:
		return true
	default:
		return false
	}
}

func withCredentialFailureStage(stage string, err error) error {
	if err == nil || !validCredentialFailureStage(stage) {
		return err
	}
	return &credentialFailureStageError{stage: stage, err: err}
}

// WithCredentialFailureStage attaches only a closed, non-secret stage.
func WithCredentialFailureStage(stage string, err error) error {
	return withCredentialFailureStage(stage, err)
}

// CredentialFailureStage returns only a bounded non-secret stage identifier.
func CredentialFailureStage(err error) string {
	var failure interface{ CredentialFailureStage() string }
	if !errors.As(err, &failure) ||
		!validCredentialFailureStage(failure.CredentialFailureStage()) {
		return ""
	}
	return failure.CredentialFailureStage()
}

// CredentialFailureRetryable reports whether repeating an operation can
// reasonably recover without first repairing or unlocking durable Vault state.
func CredentialFailureRetryable(stage string) bool {
	switch stage {
	case CredentialStageLeaseIssue,
		CredentialStageLeaseExpire,
		CredentialStageLeaseRevoke:
		return true
	default:
		return false
	}
}

// ProductKeychainHelperExitCode maps internal helper failures to fixed,
// non-secret process evidence. No system error text crosses this boundary.
func ProductKeychainHelperExitCode(err error) int {
	switch {
	case errors.Is(err, ErrCredentialHelperProtocol):
		return 5
	case errors.Is(err, ErrCredentialStoreUnavailable):
		return 6
	default:
		return 4
	}
}

type CredentialStatus string

const (
	CredentialConfigured        CredentialStatus = "configured"
	CredentialVerified          CredentialStatus = "verified"
	CredentialRejected          CredentialStatus = "rejected"
	CredentialRevoked           CredentialStatus = "revoked"
	CredentialMigrationRequired CredentialStatus = "migration_required"
	CredentialRecoveryRequired  CredentialStatus = "recovery_required"
)

type VerificationStatus string

const (
	VerificationValid       VerificationStatus = "valid"
	VerificationRejected    VerificationStatus = "rejected"
	VerificationUnavailable VerificationStatus = "unavailable"
)

type VerificationReason string

const (
	VerificationReasonNone              VerificationReason = ""
	VerificationReasonProviderRejected  VerificationReason = "provider_rejected"
	VerificationReasonUnavailable       VerificationReason = "unavailable"
	VerificationReasonTimeout           VerificationReason = "timeout"
	VerificationReasonVaultEntryMissing VerificationReason = "vault_entry_missing"
	VerificationReasonVaultUnavailable  VerificationReason = "vault_unavailable"
)

type VerificationResult struct {
	Status      VerificationStatus
	Reason      VerificationReason
	SafeMessage string
}

type CredentialCommand struct {
	CommandID           string
	ProviderID          string
	ProviderAccountID   string
	CredentialReference string
	ExpectedRevision    int64
	OccurredAt          time.Time
	Secret              []byte
}

type MetadataCommand struct {
	CommandID           string
	ProviderID          string
	ProviderAccountID   string
	CredentialReference string
	ExpectedRevision    int64
	OccurredAt          time.Time
	Status              CredentialStatus
	Reason              VerificationReason
}

type MetadataResult struct {
	ProviderID          string
	ProviderAccountID   string
	CredentialReference string
	Revision            int64
	Status              CredentialStatus
	Reason              VerificationReason
}

type SecretStore interface {
	Put(context.Context, string, []byte) error
	Read(context.Context, string) ([]byte, error)
	Delete(context.Context, string) error
}

var (
	newProductKeychainStore = func(string, string) (SecretStore, error) {
		return nil, ErrCredentialStoreUnavailable
	}
	runProductKeychainHelper = func() error {
		return ErrCredentialStoreUnavailable
	}
)

// NewProductKeychainStore returns the production process-owned Keychain
// boundary. Platform implementations must fail closed when the exact
// executable or private product socket cannot be authenticated.
func NewProductKeychainStore(
	executablePath,
	socketPath string,
) (SecretStore, error) {
	return newProductKeychainStore(executablePath, socketPath)
}

// RunProductKeychainHelper runs the internal one-operation helper mode. It is
// intentionally not a user-facing command and authenticates its parent before
// reading an operation.
func RunProductKeychainHelper() error {
	return runProductKeychainHelper()
}

type CredentialVerifier interface {
	Verify(context.Context, string, []byte) (VerificationResult, error)
}

type MetadataCommitter interface {
	CommitCredentialMetadata(
		context.Context,
		MetadataCommand,
	) (MetadataResult, error)
}

type CredentialBrokerConfig struct {
	Store     SecretStore
	Verifier  CredentialVerifier
	Committer MetadataCommitter
}

type CredentialBroker struct {
	mu        sync.Mutex
	store     SecretStore
	verifier  CredentialVerifier
	committer MetadataCommitter
}

func NewCredentialBroker(
	config CredentialBrokerConfig,
) (*CredentialBroker, error) {
	if interfaceNil(config.Store) ||
		interfaceNil(config.Verifier) ||
		interfaceNil(config.Committer) {
		return nil, ErrInvalidCredentialCommand
	}
	return &CredentialBroker{
		store:     config.Store,
		verifier:  config.Verifier,
		committer: config.Committer,
	}, nil
}

func (broker *CredentialBroker) Configure(
	ctx context.Context,
	command CredentialCommand,
) (MetadataResult, error) {
	defer clearBytes(command.Secret)
	if broker == nil || ctx == nil {
		return MetadataResult{}, ErrInvalidCredentialCommand
	}
	if err := validateCredentialCommand(command, true); err != nil {
		return MetadataResult{}, err
	}
	broker.mu.Lock()
	defer broker.mu.Unlock()
	existing, readErr := broker.store.Read(
		ctx,
		command.CredentialReference,
	)
	if readErr == nil {
		clearBytes(existing)
		return MetadataResult{}, ErrCredentialMetadataConflict
	}
	if !errors.Is(readErr, ErrCredentialNotFound) {
		return MetadataResult{}, closedStoreError(readErr)
	}
	secret := append([]byte(nil), command.Secret...)
	defer clearBytes(secret)
	if err := broker.store.Put(
		ctx,
		command.CredentialReference,
		secret,
	); err != nil {
		return MetadataResult{}, closedStoreError(err)
	}
	result, err := broker.committer.CommitCredentialMetadata(
		ctx,
		metadataFor(command, CredentialConfigured, VerificationReasonNone),
	)
	if err == nil {
		return result, nil
	}
	err = withCredentialFailureStage(CredentialStageMetadataCommit, err)
	if rollbackErr := broker.store.Delete(
		context.WithoutCancel(ctx),
		command.CredentialReference,
	); rollbackErr != nil {
		return MetadataResult{}, errors.Join(
			err,
			ErrCredentialRollbackFailed,
		)
	}
	return MetadataResult{}, err
}

func (broker *CredentialBroker) Verify(
	ctx context.Context,
	command CredentialCommand,
) (MetadataResult, error) {
	if broker == nil || ctx == nil {
		return MetadataResult{}, ErrInvalidCredentialCommand
	}
	if err := validateCredentialCommand(command, false); err != nil {
		return MetadataResult{}, err
	}
	broker.mu.Lock()
	defer broker.mu.Unlock()
	secret, err := broker.store.Read(ctx, command.CredentialReference)
	if err != nil {
		return broker.commitVerificationMetadata(
			ctx,
			command,
			CredentialRejected,
			VerificationReasonUnavailable,
		)
	}
	defer clearBytes(secret)
	verification, verifyErr := broker.verifier.Verify(
		ctx,
		command.ProviderID,
		secret,
	)
	if verifyErr != nil {
		return MetadataResult{}, closedVerificationError(verifyErr)
	}
	status := CredentialVerified
	if verification.Status != VerificationValid {
		status = CredentialRejected
	}
	if !validVerification(verification) {
		return MetadataResult{}, ErrCredentialRejected
	}
	return broker.commitVerificationMetadata(
		ctx, command, status, verification.Reason,
	)
}

func (broker *CredentialBroker) commitVerificationMetadata(
	ctx context.Context,
	command CredentialCommand,
	status CredentialStatus,
	reason VerificationReason,
) (MetadataResult, error) {
	commitCtx, cancelCommit := context.WithTimeout(
		context.WithoutCancel(ctx),
		time.Second,
	)
	defer cancelCommit()
	result, err := broker.committer.CommitCredentialMetadata(
		commitCtx,
		metadataFor(command, status, reason),
	)
	return result, withCredentialFailureStage(CredentialStageMetadataCommit, err)
}

func (broker *CredentialBroker) Replace(
	ctx context.Context,
	command CredentialCommand,
) (MetadataResult, error) {
	defer clearBytes(command.Secret)
	if broker == nil || ctx == nil {
		return MetadataResult{}, ErrInvalidCredentialCommand
	}
	if err := validateCredentialCommand(command, true); err != nil ||
		command.ExpectedRevision <= 0 {
		return MetadataResult{}, ErrInvalidCredentialCommand
	}
	broker.mu.Lock()
	defer broker.mu.Unlock()
	previous, err := broker.store.Read(ctx, command.CredentialReference)
	if err != nil {
		return MetadataResult{}, closedStoreError(err)
	}
	defer clearBytes(previous)
	replacement := append([]byte(nil), command.Secret...)
	defer clearBytes(replacement)
	if err := broker.store.Put(
		ctx,
		command.CredentialReference,
		replacement,
	); err != nil {
		return MetadataResult{}, closedStoreError(err)
	}
	result, commitErr := broker.committer.CommitCredentialMetadata(
		ctx,
		metadataFor(command, CredentialConfigured, VerificationReasonNone),
	)
	if commitErr == nil {
		return result, nil
	}
	commitErr = withCredentialFailureStage(
		CredentialStageMetadataCommit,
		commitErr,
	)
	if rollbackErr := broker.store.Put(
		context.WithoutCancel(ctx),
		command.CredentialReference,
		previous,
	); rollbackErr != nil {
		return MetadataResult{}, errors.Join(
			commitErr,
			ErrCredentialRollbackFailed,
		)
	}
	return MetadataResult{}, commitErr
}

func (broker *CredentialBroker) Revoke(
	ctx context.Context,
	command CredentialCommand,
) (MetadataResult, error) {
	if broker == nil || ctx == nil {
		return MetadataResult{}, ErrInvalidCredentialCommand
	}
	if err := validateCredentialCommand(command, false); err != nil ||
		command.ExpectedRevision <= 0 {
		return MetadataResult{}, ErrInvalidCredentialCommand
	}
	broker.mu.Lock()
	defer broker.mu.Unlock()
	previous, err := broker.store.Read(ctx, command.CredentialReference)
	if err != nil {
		return MetadataResult{}, closedStoreError(err)
	}
	defer clearBytes(previous)
	if err := broker.store.Delete(
		ctx,
		command.CredentialReference,
	); err != nil {
		return MetadataResult{}, closedStoreError(err)
	}
	result, commitErr := broker.committer.CommitCredentialMetadata(
		ctx,
		metadataFor(command, CredentialRevoked, VerificationReasonNone),
	)
	if commitErr == nil {
		return result, nil
	}
	commitErr = withCredentialFailureStage(
		CredentialStageMetadataCommit,
		commitErr,
	)
	if rollbackErr := broker.store.Put(
		context.WithoutCancel(ctx),
		command.CredentialReference,
		previous,
	); rollbackErr != nil {
		return MetadataResult{}, errors.Join(
			commitErr,
			ErrCredentialRollbackFailed,
		)
	}
	return MetadataResult{}, commitErr
}

func metadataFor(
	command CredentialCommand,
	status CredentialStatus,
	reason VerificationReason,
) MetadataCommand {
	commandID := command.CommandID
	if commandID == "" {
		commandID = fmt.Sprintf(
			"credential-%s-%d-%s",
			command.ProviderID,
			command.ExpectedRevision+1,
			status,
		)
	}
	return MetadataCommand{
		CommandID:           commandID,
		ProviderID:          command.ProviderID,
		ProviderAccountID:   command.ProviderAccountID,
		CredentialReference: command.CredentialReference,
		ExpectedRevision:    command.ExpectedRevision,
		OccurredAt:          command.OccurredAt,
		Status:              status,
		Reason:              reason,
	}
}

func validateCredentialCommand(
	command CredentialCommand,
	requireSecret bool,
) error {
	if !ValidProviderIdentifier(command.ProviderID) ||
		command.ProviderAccountID != "" &&
			!ValidProviderAccountIdentifier(command.ProviderID, command.ProviderAccountID) ||
		!validCredentialReference(command.CredentialReference) ||
		command.ExpectedRevision < 0 ||
		command.OccurredAt.IsZero() ||
		command.OccurredAt.Location() != time.UTC ||
		requireSecret && (len(command.Secret) < 1 || len(command.Secret) > 8192) ||
		!requireSecret && len(command.Secret) != 0 {
		return ErrInvalidCredentialCommand
	}
	return nil
}

// ValidateCredentialCommand applies the shared Broker boundary validation.
func ValidateCredentialCommand(command CredentialCommand, requireSecret bool) error {
	return validateCredentialCommand(command, requireSecret)
}

// ValidProviderIdentifier validates the durable identifier shape shared by
// credential metadata writers and projections. Provider availability remains
// owned by the Provider registry at the product boundary.
func ValidProviderIdentifier(value string) bool {
	if value == "" || len(value) > 64 || value[0] == '-' ||
		value[len(value)-1] == '-' {
		return false
	}
	previousHyphen := false
	for _, character := range value {
		switch {
		case character >= 'a' && character <= 'z',
			character >= '0' && character <= '9':
			previousHyphen = false
		case character == '-' && !previousHyphen:
			previousHyphen = true
		default:
			return false
		}
	}
	return true
}

// ValidProviderAccountIdentifier validates a durable, non-secret account ID
// scoped to one Provider. Account IDs never carry endpoint or credential data.
func ValidProviderAccountIdentifier(providerID, value string) bool {
	if !ValidProviderIdentifier(providerID) || len(value) > 128 ||
		!strings.HasPrefix(value, providerID+".") {
		return false
	}
	suffix := value[len(providerID)+1:]
	if suffix == "" || suffix[0] == '.' || suffix[len(suffix)-1] == '.' ||
		suffix[0] == '-' || suffix[len(suffix)-1] == '-' {
		return false
	}
	previousSeparator := false
	for _, character := range suffix {
		switch {
		case character >= 'a' && character <= 'z',
			character >= '0' && character <= '9':
			previousSeparator = false
		case (character == '.' || character == '-') && !previousSeparator:
			previousSeparator = true
		default:
			return false
		}
	}
	return true
}

func validCredentialReference(value string) bool {
	const prefix = "credential-ref-"
	if !strings.HasPrefix(value, prefix) ||
		len(value) <= len(prefix) ||
		len(value) > 128 {
		return false
	}
	for _, character := range value[len(prefix):] {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '-' {
			continue
		}
		return false
	}
	return true
}

func validCredentialIdentifier(value string, maximum int) bool {
	if value == "" || len(value) > maximum ||
		!utf8.ValidString(value) ||
		strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validVerification(result VerificationResult) bool {
	switch result.Status {
	case VerificationValid:
		return result.Reason == VerificationReasonNone
	case VerificationRejected:
		return result.Reason == VerificationReasonProviderRejected
	case VerificationUnavailable:
		return result.Reason == VerificationReasonUnavailable ||
			result.Reason == VerificationReasonTimeout
	default:
		return false
	}
}

// ValidateVerificationResult rejects unbounded or inconsistent Provider status.
func ValidateVerificationResult(result VerificationResult) bool {
	return validVerification(result)
}

func closedStoreError(err error) error {
	stage := CredentialFailureStage(err)
	var closed error
	switch {
	case errors.Is(err, ErrCredentialNotFound):
		closed = ErrCredentialNotFound
	case errors.Is(err, ErrCredentialStoreDenied):
		closed = ErrCredentialStoreDenied
	case errors.Is(err, ErrCredentialStoreUnavailable):
		closed = ErrCredentialStoreUnavailable
	default:
		closed = ErrCredentialStoreUnavailable
	}
	return withCredentialFailureStage(stage, closed)
}

func closedVerificationError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	return ErrCredentialRejected
}

// ClosedVerificationError maps Provider errors to the bounded public set.
func ClosedVerificationError(err error) error {
	return closedVerificationError(err)
}

func clearBytes(input []byte) {
	for index := range input {
		input[index] = 0
	}
}

func interfaceNil(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
