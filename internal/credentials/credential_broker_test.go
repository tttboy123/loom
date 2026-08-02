package credentials

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type brokerTestStore struct {
	mu        sync.Mutex
	values    map[string][]byte
	putErr    error
	readErr   error
	deleteErr error
	puts      int
	deletes   int
}

func (store *brokerTestStore) Put(
	_ context.Context,
	reference string,
	secret []byte,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.putErr != nil {
		return store.putErr
	}
	if store.values == nil {
		store.values = make(map[string][]byte)
	}
	store.values[reference] = append([]byte(nil), secret...)
	store.puts++
	return nil
}

func (store *brokerTestStore) Read(
	_ context.Context,
	reference string,
) ([]byte, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.readErr != nil {
		return nil, store.readErr
	}
	value, ok := store.values[reference]
	if !ok {
		return nil, ErrCredentialNotFound
	}
	return append([]byte(nil), value...), nil
}

func (store *brokerTestStore) Delete(
	_ context.Context,
	reference string,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.deleteErr != nil {
		return store.deleteErr
	}
	if _, ok := store.values[reference]; !ok {
		return ErrCredentialNotFound
	}
	delete(store.values, reference)
	store.deletes++
	return nil
}

type brokerTestVerifier struct {
	status VerificationStatus
	reason VerificationReason
	err    error
	seen   []byte
}

func (verifier *brokerTestVerifier) Verify(
	_ context.Context,
	providerID string,
	secret []byte,
) (VerificationResult, error) {
	if providerID != "minimax" {
		return VerificationResult{}, errors.New("wrong provider")
	}
	verifier.seen = append([]byte(nil), secret...)
	return VerificationResult{
		Status: verifier.status,
		Reason: verifier.reason,
	}, verifier.err
}

type brokerTestCommitter struct {
	mu       sync.Mutex
	commands []MetadataCommand
	failAt   int
}

func (committer *brokerTestCommitter) CommitCredentialMetadata(
	_ context.Context,
	command MetadataCommand,
) (MetadataResult, error) {
	committer.mu.Lock()
	defer committer.mu.Unlock()
	committer.commands = append(committer.commands, command)
	if committer.failAt > 0 && len(committer.commands) == committer.failAt {
		return MetadataResult{}, ErrCredentialMetadataConflict
	}
	return MetadataResult{
		ProviderID:          command.ProviderID,
		CredentialReference: command.CredentialReference,
		Revision:            command.ExpectedRevision + 1,
		Status:              command.Status,
		Reason:              command.Reason,
	}, nil
}

type brokerCancelingVerifier struct {
	cancel context.CancelFunc
	calls  int
}

func (verifier *brokerCancelingVerifier) Verify(
	_ context.Context,
	providerID string,
	secret []byte,
) (VerificationResult, error) {
	if providerID != "minimax" || len(secret) == 0 {
		return VerificationResult{}, errors.New("invalid fixture input")
	}
	verifier.calls++
	verifier.cancel()
	return VerificationResult{
		Status: VerificationValid,
		Reason: VerificationReasonNone,
	}, nil
}

type brokerCommitContextRecorder struct {
	calls       int
	contextErr  error
	hasDeadline bool
	remaining   time.Duration
}

func (recorder *brokerCommitContextRecorder) CommitCredentialMetadata(
	ctx context.Context,
	command MetadataCommand,
) (MetadataResult, error) {
	recorder.calls++
	recorder.contextErr = ctx.Err()
	deadline, ok := ctx.Deadline()
	recorder.hasDeadline = ok
	if ok {
		recorder.remaining = time.Until(deadline)
	}
	if recorder.contextErr != nil {
		return MetadataResult{}, recorder.contextErr
	}
	return MetadataResult{
		ProviderID:          command.ProviderID,
		CredentialReference: command.CredentialReference,
		Revision:            command.ExpectedRevision + 1,
		Status:              command.Status,
		Reason:              command.Reason,
	}, nil
}

func TestCredentialBrokerCommitsObservedVerificationAfterCallerCancellation(
	t *testing.T,
) {
	ctx, cancel := context.WithCancel(context.Background())
	store := &brokerTestStore{values: map[string][]byte{
		"credential-ref-1": {0x10, 0x20, 0x30, 0x40},
	}}
	verifier := &brokerCancelingVerifier{cancel: cancel}
	committer := &brokerCommitContextRecorder{}
	broker, err := NewCredentialBroker(CredentialBrokerConfig{
		Store:     store,
		Verifier:  verifier,
		Committer: committer,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := broker.Verify(ctx, CredentialCommand{
		CommandID:           "verify-after-observation",
		ProviderID:          "minimax",
		CredentialReference: "credential-ref-1",
		ExpectedRevision:    1,
		OccurredAt:          time.Unix(500, 0).UTC(),
	})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if verifier.calls != 1 ||
		committer.calls != 1 ||
		committer.contextErr != nil ||
		!committer.hasDeadline ||
		committer.remaining <= 0 ||
		committer.remaining > time.Second ||
		result.Revision != 2 ||
		result.Status != CredentialVerified {
		t.Fatalf(
			"result=%#v verifier_calls=%d commit=%#v",
			result,
			verifier.calls,
			committer,
		)
	}
}

func TestCredentialBrokerStoreFailureCommitsUnavailableTerminalWithoutProvider(
	t *testing.T,
) {
	for name, readErr := range map[string]error{
		"not_found":   ErrCredentialNotFound,
		"denied":      ErrCredentialStoreDenied,
		"unavailable": ErrCredentialStoreUnavailable,
		"unknown":     errors.New("private store failure"),
	} {
		t.Run(name, func(t *testing.T) {
			store := &brokerTestStore{readErr: readErr}
			verifier := &brokerTestVerifier{
				status: VerificationValid,
				reason: VerificationReasonNone,
			}
			committer := &brokerTestCommitter{}
			broker, err := NewCredentialBroker(CredentialBrokerConfig{
				Store: store, Verifier: verifier, Committer: committer,
			})
			if err != nil {
				t.Fatal(err)
			}
			occurredAt := time.Unix(501, 0).UTC()
			result, err := broker.Verify(context.Background(), CredentialCommand{
				CommandID: "verify-store-" + name, ProviderID: "minimax",
				CredentialReference: "credential-ref-1", ExpectedRevision: 1,
				OccurredAt: occurredAt,
			})
			if err != nil {
				t.Fatalf("Verify() error = %v", err)
			}
			if len(verifier.seen) != 0 || len(committer.commands) != 1 ||
				result.Revision != 2 || result.Status != CredentialRejected ||
				result.Reason != VerificationReasonUnavailable {
				t.Fatalf(
					"result=%#v verifier=%v commands=%#v",
					result, verifier.seen, committer.commands,
				)
			}
			command := committer.commands[0]
			if command.CommandID != "verify-store-"+name ||
				command.ProviderID != "minimax" ||
				command.CredentialReference != "credential-ref-1" ||
				command.ExpectedRevision != 1 ||
				!command.OccurredAt.Equal(occurredAt) ||
				command.Status != CredentialRejected ||
				command.Reason != VerificationReasonUnavailable {
				t.Fatalf("terminal command = %#v", command)
			}
		})
	}
}

func TestCredentialBrokerStoreFailureTerminalCommitIsBoundedAndRequired(
	t *testing.T,
) {
	t.Run("bounded", func(t *testing.T) {
		recorder := &brokerCommitContextRecorder{}
		broker, err := NewCredentialBroker(CredentialBrokerConfig{
			Store:    &brokerTestStore{readErr: ErrCredentialStoreUnavailable},
			Verifier: &brokerTestVerifier{}, Committer: recorder,
		})
		if err != nil {
			t.Fatal(err)
		}
		result, err := broker.Verify(context.Background(), CredentialCommand{
			CommandID: "verify-store-bounded", ProviderID: "minimax",
			CredentialReference: "credential-ref-1", ExpectedRevision: 4,
			OccurredAt: time.Unix(502, 0).UTC(),
		})
		if err != nil || recorder.calls != 1 || recorder.contextErr != nil ||
			!recorder.hasDeadline || recorder.remaining <= 0 ||
			recorder.remaining > time.Second || result.Revision != 5 ||
			result.Status != CredentialRejected ||
			result.Reason != VerificationReasonUnavailable {
			t.Fatalf("result=%#v err=%v recorder=%#v", result, err, recorder)
		}
	})

	t.Run("commit_failure", func(t *testing.T) {
		committer := &brokerTestCommitter{failAt: 1}
		verifier := &brokerTestVerifier{}
		broker, err := NewCredentialBroker(CredentialBrokerConfig{
			Store:    &brokerTestStore{readErr: ErrCredentialStoreDenied},
			Verifier: verifier, Committer: committer,
		})
		if err != nil {
			t.Fatal(err)
		}
		result, err := broker.Verify(context.Background(), CredentialCommand{
			CommandID: "verify-store-commit-failure", ProviderID: "minimax",
			CredentialReference: "credential-ref-1", ExpectedRevision: 6,
			OccurredAt: time.Unix(503, 0).UTC(),
		})
		if !errors.Is(err, ErrCredentialMetadataConflict) ||
			result != (MetadataResult{}) || len(verifier.seen) != 0 ||
			len(committer.commands) != 1 {
			t.Fatalf(
				"result=%#v err=%v verifier=%v commands=%#v",
				result, err, verifier.seen, committer.commands,
			)
		}
	})
}

func TestCredentialBrokerConfigureVerifyReplaceAndRevoke(t *testing.T) {
	store := &brokerTestStore{}
	verifier := &brokerTestVerifier{
		status: VerificationValid,
		reason: VerificationReasonNone,
	}
	committer := &brokerTestCommitter{}
	broker, err := NewCredentialBroker(CredentialBrokerConfig{
		Store:     store,
		Verifier:  verifier,
		Committer: committer,
	})
	if err != nil {
		t.Fatalf("NewCredentialBroker() error = %v", err)
	}

	configureSecret := []byte{0x10, 0x21, 0x32, 0x43, 0x54}
	configured, err := broker.Configure(context.Background(), CredentialCommand{
		ProviderID:          "minimax",
		CredentialReference: "credential-ref-1",
		ExpectedRevision:    0,
		OccurredAt:          time.Unix(100, 0).UTC(),
		Secret:              configureSecret,
	})
	if err != nil {
		t.Fatalf("Configure() error = %v", err)
	}
	if configured.Revision != 1 ||
		configured.Status != CredentialConfigured ||
		!allCredentialBytesZero(configureSecret) {
		t.Fatalf("Configure() result = %#v, input = %v", configured, configureSecret)
	}

	verified, err := broker.Verify(context.Background(), CredentialCommand{
		ProviderID:          "minimax",
		CredentialReference: "credential-ref-1",
		ExpectedRevision:    1,
		OccurredAt:          time.Unix(101, 0).UTC(),
	})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if verified.Status != CredentialVerified ||
		verified.Reason != VerificationReasonNone ||
		!bytes.Equal(verifier.seen, []byte{0x10, 0x21, 0x32, 0x43, 0x54}) {
		t.Fatalf("Verify() result = %#v, seen = %v", verified, verifier.seen)
	}

	replacement := []byte{0x65, 0x76, 0x07, 0x18, 0x29}
	replaced, err := broker.Replace(context.Background(), CredentialCommand{
		ProviderID:          "minimax",
		CredentialReference: "credential-ref-1",
		ExpectedRevision:    2,
		OccurredAt:          time.Unix(102, 0).UTC(),
		Secret:              replacement,
	})
	if err != nil {
		t.Fatalf("Replace() error = %v", err)
	}
	if replaced.Status != CredentialConfigured ||
		replaced.Revision != 3 ||
		!allCredentialBytesZero(replacement) {
		t.Fatalf("Replace() result = %#v, input = %v", replaced, replacement)
	}

	revoked, err := broker.Revoke(context.Background(), CredentialCommand{
		ProviderID:          "minimax",
		CredentialReference: "credential-ref-1",
		ExpectedRevision:    3,
		OccurredAt:          time.Unix(103, 0).UTC(),
	})
	if err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}
	if revoked.Status != CredentialRevoked || revoked.Revision != 4 {
		t.Fatalf("Revoke() result = %#v", revoked)
	}
	if _, err := store.Read(context.Background(), "credential-ref-1"); !errors.Is(
		err,
		ErrCredentialNotFound,
	) {
		t.Fatalf("credential remains after revoke: %v", err)
	}
}

func TestCredentialBrokerRollsBackStoreOnMetadataFailure(t *testing.T) {
	oldSecret := []byte{0x01, 0x02, 0x03, 0x04}
	store := &brokerTestStore{
		values: map[string][]byte{
			"credential-ref-1": append([]byte(nil), oldSecret...),
		},
	}
	committer := &brokerTestCommitter{failAt: 1}
	broker, err := NewCredentialBroker(CredentialBrokerConfig{
		Store: store,
		Verifier: &brokerTestVerifier{
			status: VerificationValid,
			reason: VerificationReasonNone,
		},
		Committer: committer,
	})
	if err != nil {
		t.Fatalf("NewCredentialBroker() error = %v", err)
	}

	replacement := []byte{0x09, 0x08, 0x07, 0x06}
	if _, err := broker.Replace(context.Background(), CredentialCommand{
		ProviderID:          "minimax",
		CredentialReference: "credential-ref-1",
		ExpectedRevision:    1,
		OccurredAt:          time.Unix(200, 0).UTC(),
		Secret:              replacement,
	}); !errors.Is(err, ErrCredentialMetadataConflict) {
		t.Fatalf("Replace() error = %v", err)
	}
	restored, err := store.Read(context.Background(), "credential-ref-1")
	if err != nil || !bytes.Equal(restored, oldSecret) {
		t.Fatalf("rollback restored %v, error = %v", restored, err)
	}
	if !allCredentialBytesZero(replacement) {
		t.Fatalf("replacement not cleared: %v", replacement)
	}

	committer.failAt = 2
	if _, err := broker.Revoke(context.Background(), CredentialCommand{
		ProviderID:          "minimax",
		CredentialReference: "credential-ref-1",
		ExpectedRevision:    1,
		OccurredAt:          time.Unix(201, 0).UTC(),
	}); !errors.Is(err, ErrCredentialMetadataConflict) {
		t.Fatalf("Revoke() error = %v", err)
	}
	restored, err = store.Read(context.Background(), "credential-ref-1")
	if err != nil || !bytes.Equal(restored, oldSecret) {
		t.Fatalf("revoke rollback restored %v, error = %v", restored, err)
	}
}

func TestCredentialBrokerReturnsClosedErrorsWithoutSecretDisclosure(t *testing.T) {
	store := &brokerTestStore{putErr: ErrCredentialStoreDenied}
	broker, err := NewCredentialBroker(CredentialBrokerConfig{
		Store: store,
		Verifier: &brokerTestVerifier{
			status: VerificationRejected,
			reason: VerificationReasonProviderRejected,
		},
		Committer: &brokerTestCommitter{},
	})
	if err != nil {
		t.Fatalf("NewCredentialBroker() error = %v", err)
	}
	secret := []byte{0xaa, 0xbb, 0xcc, 0xdd}
	_, err = broker.Configure(context.Background(), CredentialCommand{
		ProviderID:          "minimax",
		CredentialReference: "credential-ref-1",
		OccurredAt:          time.Unix(300, 0).UTC(),
		Secret:              secret,
	})
	if !errors.Is(err, ErrCredentialStoreDenied) ||
		bytes.Contains([]byte(err.Error()), []byte{0xaa, 0xbb}) ||
		!allCredentialBytesZero(secret) {
		t.Fatalf("unsafe Configure() error = %v, input = %v", err, secret)
	}
}

func TestCredentialBrokerConcurrentConfigureKeepsSingleWinnerSecret(
	t *testing.T,
) {
	store := &brokerTestStore{}
	broker, err := NewCredentialBroker(CredentialBrokerConfig{
		Store: store,
		Verifier: &brokerTestVerifier{
			status: VerificationValid,
			reason: VerificationReasonNone,
		},
		Committer: &brokerTestCommitter{},
	})
	if err != nil {
		t.Fatal(err)
	}
	secrets := [][]byte{
		{0x31, 0x42, 0x53, 0x64},
		{0x75, 0x26, 0x37, 0x48},
	}
	expected := [][]byte{
		append([]byte(nil), secrets[0]...),
		append([]byte(nil), secrets[1]...),
	}
	errorsByCall := make(chan error, 2)
	var wait sync.WaitGroup
	for index := range secrets {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			_, callErr := broker.Configure(
				context.Background(),
				CredentialCommand{
					CommandID:           "configure-concurrent",
					ProviderID:          "minimax",
					CredentialReference: "credential-ref-concurrent",
					ExpectedRevision:    0,
					OccurredAt:          time.Unix(400, 0).UTC(),
					Secret:              secrets[index],
				},
			)
			errorsByCall <- callErr
		}(index)
	}
	wait.Wait()
	close(errorsByCall)
	successes := 0
	conflicts := 0
	for callErr := range errorsByCall {
		switch {
		case callErr == nil:
			successes++
		case errors.Is(callErr, ErrCredentialMetadataConflict):
			conflicts++
		default:
			t.Fatalf("Configure() error = %v", callErr)
		}
	}
	stored, err := store.Read(
		context.Background(),
		"credential-ref-concurrent",
	)
	if err != nil ||
		successes != 1 ||
		conflicts != 1 ||
		(!bytes.Equal(stored, expected[0]) &&
			!bytes.Equal(stored, expected[1])) ||
		store.deletes != 0 {
		t.Fatalf(
			"successes=%d conflicts=%d stored=%v deletes=%d err=%v",
			successes,
			conflicts,
			stored,
			store.deletes,
			err,
		)
	}
}

func allCredentialBytesZero(input []byte) bool {
	for _, value := range input {
		if value != 0 {
			return false
		}
	}
	return true
}
