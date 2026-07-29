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
	ErrInvalidCredentialCommand   = errors.New("invalid credential command")
	ErrCredentialNotFound         = errors.New("credential not found")
	ErrCredentialStoreUnavailable = errors.New("credential store unavailable")
	ErrCredentialStoreDenied      = errors.New("credential store denied")
	ErrCredentialMetadataConflict = errors.New("credential metadata conflict")
	ErrCredentialRollbackFailed   = errors.New("credential rollback failed")
	ErrCredentialRejected         = errors.New("credential rejected")
)

type CredentialStatus string

const (
	CredentialConfigured CredentialStatus = "configured"
	CredentialVerified   CredentialStatus = "verified"
	CredentialRejected   CredentialStatus = "rejected"
	CredentialRevoked    CredentialStatus = "revoked"
)

type VerificationStatus string

const (
	VerificationValid       VerificationStatus = "valid"
	VerificationRejected    VerificationStatus = "rejected"
	VerificationUnavailable VerificationStatus = "unavailable"
)

type VerificationReason string

const (
	VerificationReasonNone             VerificationReason = ""
	VerificationReasonProviderRejected VerificationReason = "provider_rejected"
	VerificationReasonUnavailable      VerificationReason = "unavailable"
	VerificationReasonTimeout          VerificationReason = "timeout"
)

type VerificationResult struct {
	Status      VerificationStatus
	Reason      VerificationReason
	SafeMessage string
}

type CredentialCommand struct {
	CommandID           string
	ProviderID          string
	CredentialReference string
	ExpectedRevision    int64
	OccurredAt          time.Time
	Secret              []byte
}

type MetadataCommand struct {
	CommandID           string
	ProviderID          string
	CredentialReference string
	ExpectedRevision    int64
	OccurredAt          time.Time
	Status              CredentialStatus
	Reason              VerificationReason
}

type MetadataResult struct {
	ProviderID          string
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
		return MetadataResult{}, closedStoreError(err)
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
	return broker.committer.CommitCredentialMetadata(
		ctx,
		metadataFor(command, status, verification.Reason),
	)
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
	if command.ProviderID != "minimax" ||
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

func closedStoreError(err error) error {
	switch {
	case errors.Is(err, ErrCredentialNotFound):
		return ErrCredentialNotFound
	case errors.Is(err, ErrCredentialStoreDenied):
		return ErrCredentialStoreDenied
	case errors.Is(err, ErrCredentialStoreUnavailable):
		return ErrCredentialStoreUnavailable
	default:
		return ErrCredentialStoreUnavailable
	}
}

func closedVerificationError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	return ErrCredentialRejected
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
