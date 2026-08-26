package vault

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestCredentialLeaseClosesAndZeroizesPlaintext(t *testing.T) {
	identity := CredentialIdentity{
		CredentialReference: "credential-ref-deepseek-primary",
		ProviderID:          "deepseek", ProviderAccountID: "deepseek.primary",
		CredentialRevision: 3,
	}
	manager, err := NewCredentialLeaseManager(&leaseFixtureReader{
		secrets: map[CredentialIdentity][]byte{identity: []byte("lease-secret")},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	lease, err := manager.Acquire(context.Background(), identity, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.WithSecret(func(_ context.Context, secret []byte) error {
		if !bytes.Equal(secret, []byte("lease-secret")) {
			t.Fatalf("secret = %q", secret)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	buffer := lease.secret
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	for _, value := range buffer {
		if value != 0 {
			t.Fatal("closed lease retained plaintext")
		}
	}
	if err := lease.WithSecret(func(context.Context, []byte) error { return nil }); !errors.Is(err, ErrCredentialLeaseClosed) {
		t.Fatalf("closed lease error = %v", err)
	}
}

func TestCredentialLeaseExpiryAndAccountScopedRevoke(t *testing.T) {
	primary := CredentialIdentity{
		CredentialReference: "credential-ref-deepseek-primary",
		ProviderID:          "deepseek", ProviderAccountID: "deepseek.primary",
		CredentialRevision: 3,
	}
	backup := CredentialIdentity{
		CredentialReference: "credential-ref-deepseek-backup",
		ProviderID:          "deepseek", ProviderAccountID: "deepseek.backup",
		CredentialRevision: 8,
	}
	manager, err := NewCredentialLeaseManager(&leaseFixtureReader{secrets: map[CredentialIdentity][]byte{
		primary: []byte("primary-secret"), backup: []byte("backup-secret"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	expiring, err := manager.Acquire(context.Background(), primary, 20*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(40 * time.Millisecond)
	if err := expiring.WithSecret(func(context.Context, []byte) error { return nil }); !errors.Is(err, ErrCredentialLeaseExpired) {
		t.Fatalf("expired lease error = %v", err)
	}
	primaryLease, err := manager.Acquire(context.Background(), primary, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	backupLease, err := manager.Acquire(context.Background(), backup, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if count := manager.Revoke(primary); count != 1 {
		t.Fatalf("revoked leases = %d", count)
	}
	if err := primaryLease.WithSecret(func(context.Context, []byte) error { return nil }); !errors.Is(err, ErrCredentialLeaseRevoked) {
		t.Fatalf("revoked lease error = %v", err)
	}
	if err := backupLease.WithSecret(func(_ context.Context, secret []byte) error {
		if !bytes.Equal(secret, []byte("backup-secret")) {
			t.Fatalf("backup secret = %q", secret)
		}
		return nil
	}); err != nil {
		t.Fatalf("backup lease was affected: %v", err)
	}
	backupLease.Close()
}

func TestCredentialLeaseImmediateExpiryPublishesInitializedTimer(t *testing.T) {
	identity := CredentialIdentity{
		CredentialReference: "credential-ref-deepseek-primary",
		ProviderID:          "deepseek", ProviderAccountID: "deepseek.primary",
		CredentialRevision: 3,
	}
	manager, err := NewCredentialLeaseManager(&leaseFixtureReader{
		secrets: map[CredentialIdentity][]byte{identity: []byte("lease-secret")},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	for index := 0; index < 256; index++ {
		lease, err := manager.Acquire(context.Background(), identity, time.Nanosecond)
		if err != nil {
			t.Fatal(err)
		}
		_ = lease.Close()
	}
}

func TestCredentialLeaseSupportsBoundedHarnessExecutionWindow(t *testing.T) {
	identity := CredentialIdentity{
		CredentialReference: "credential-ref-minimax-primary",
		ProviderID:          "minimax", ProviderAccountID: "minimax.primary",
		CredentialRevision: 23,
	}
	manager, err := NewCredentialLeaseManager(&leaseFixtureReader{
		secrets: map[CredentialIdentity][]byte{identity: []byte("lease-secret")},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	lease, err := manager.Acquire(context.Background(), identity, 10*time.Minute)
	if err != nil {
		t.Fatalf("10 minute Agent lease = %v", err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if lease, err := manager.Acquire(
		context.Background(), identity, 15*time.Minute+time.Nanosecond,
	); !errors.Is(err, ErrInvalidVaultInput) || lease != nil {
		t.Fatalf("overlong lease = %#v, %v", lease, err)
	}
}

func TestCredentialLeaseRevokeRejectsInflightAndFutureAcquire(t *testing.T) {
	identity := CredentialIdentity{
		CredentialReference: "credential-ref-deepseek-primary",
		ProviderID:          "deepseek", ProviderAccountID: "deepseek.primary",
		CredentialRevision: 3,
	}
	reader := &blockingLeaseReader{
		started: make(chan struct{}), release: make(chan struct{}),
		secret: []byte("lease-secret"),
	}
	manager, err := NewCredentialLeaseManager(reader)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	result := make(chan error, 1)
	go func() {
		lease, acquireErr := manager.Acquire(context.Background(), identity, time.Second)
		if lease != nil {
			_ = lease.Close()
		}
		result <- acquireErr
	}()
	<-reader.started
	if count := manager.Revoke(identity); count != 0 {
		t.Fatalf("revoked published leases = %d", count)
	}
	close(reader.release)
	if err := <-result; !errors.Is(err, ErrCredentialLeaseRevoked) {
		t.Fatalf("inflight acquire error = %v", err)
	}
	for _, value := range reader.returned {
		if value != 0 {
			t.Fatal("rejected inflight lease retained plaintext")
		}
	}
	if lease, err := manager.Acquire(
		context.Background(), identity, time.Second,
	); !errors.Is(err, ErrCredentialLeaseRevoked) || lease != nil {
		t.Fatalf("future acquire = %#v, %v", lease, err)
	}
}

func TestCredentialLeaseRevokeCancelsActiveUseBeforeZeroize(t *testing.T) {
	identity := CredentialIdentity{
		CredentialReference: "credential-ref-deepseek-primary",
		ProviderID:          "deepseek", ProviderAccountID: "deepseek.primary",
		CredentialRevision: 3,
	}
	manager, err := NewCredentialLeaseManager(&leaseFixtureReader{
		secrets: map[CredentialIdentity][]byte{identity: []byte("lease-secret")},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	lease, err := manager.Acquire(context.Background(), identity, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	useResult := make(chan error, 1)
	go func() {
		useResult <- lease.WithSecret(func(ctx context.Context, _ []byte) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		})
	}()
	<-started
	if count := manager.Revoke(identity); count != 1 {
		t.Fatalf("revoked leases = %d", count)
	}
	if err := <-useResult; !errors.Is(err, context.Canceled) {
		t.Fatalf("active use result = %v", err)
	}
	if err := lease.WithSecret(
		func(context.Context, []byte) error { return nil },
	); !errors.Is(err, ErrCredentialLeaseRevoked) {
		t.Fatalf("revoked lease error = %v", err)
	}
}

func TestCredentialLeaseCancelledInflightAcquireIsNotPublished(t *testing.T) {
	identity := CredentialIdentity{
		CredentialReference: "credential-ref-deepseek-primary",
		ProviderID:          "deepseek", ProviderAccountID: "deepseek.primary",
		CredentialRevision: 3,
	}
	reader := &blockingLeaseReader{
		started: make(chan struct{}), release: make(chan struct{}),
		secret: []byte("lease-secret"),
	}
	manager, err := NewCredentialLeaseManager(reader)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		lease, acquireErr := manager.Acquire(ctx, identity, time.Second)
		if lease != nil {
			_ = lease.Close()
		}
		result <- acquireErr
	}()
	<-reader.started
	cancel()
	close(reader.release)
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled acquire error = %v", err)
	}
	for _, value := range reader.returned {
		if value != 0 {
			t.Fatal("cancelled inflight lease retained plaintext")
		}
	}
}

func TestCredentialLeaseRotationBarrierRevokesActiveLeaseAndAllowsFreshAcquire(t *testing.T) {
	identity := CredentialIdentity{
		CredentialReference: "credential-ref-deepseek-primary",
		ProviderID:          "deepseek", ProviderAccountID: "deepseek.primary",
		CredentialRevision: 3,
	}
	manager, err := NewCredentialLeaseManager(&leaseFixtureReader{
		secrets: map[CredentialIdentity][]byte{identity: []byte("lease-secret")},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	lease, err := manager.Acquire(context.Background(), identity, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.WithRotationBarrier(
		context.Background(),
		func() error {
			return lease.WithSecret(func(context.Context, []byte) error { return nil })
		},
	); !errors.Is(err, ErrCredentialLeaseRevoked) {
		t.Fatalf("rotation callback = %v", err)
	}
	fresh, err := manager.Acquire(context.Background(), identity, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	fresh.Close()
}

func TestCredentialLeaseRotationBarrierRejectsInflightOldGeneration(t *testing.T) {
	identity := CredentialIdentity{
		CredentialReference: "credential-ref-deepseek-primary",
		ProviderID:          "deepseek", ProviderAccountID: "deepseek.primary",
		CredentialRevision: 3,
	}
	reader := &blockingLeaseReader{
		started: make(chan struct{}), release: make(chan struct{}),
		secret: []byte("lease-secret"),
	}
	manager, err := NewCredentialLeaseManager(reader)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	acquireResult := make(chan error, 1)
	go func() {
		lease, acquireErr := manager.Acquire(
			context.Background(), identity, time.Second,
		)
		if lease != nil {
			lease.Close()
		}
		acquireResult <- acquireErr
	}()
	<-reader.started
	if err := manager.WithRotationBarrier(
		context.Background(), func() error { return nil },
	); err != nil {
		t.Fatal(err)
	}
	close(reader.release)
	if err := <-acquireResult; !errors.Is(err, ErrCredentialLeaseRevoked) {
		t.Fatalf("inflight acquire error = %v", err)
	}
	for _, value := range reader.returned {
		if value != 0 {
			t.Fatal("rotation-rejected lease retained plaintext")
		}
	}
}

type leaseFixtureReader struct {
	mu      sync.Mutex
	secrets map[CredentialIdentity][]byte
}

type blockingLeaseReader struct {
	started  chan struct{}
	release  chan struct{}
	secret   []byte
	returned []byte
}

func (reader *blockingLeaseReader) ReadCredential(
	_ context.Context,
	_ CredentialIdentity,
) ([]byte, error) {
	close(reader.started)
	<-reader.release
	reader.returned = append([]byte(nil), reader.secret...)
	return reader.returned, nil
}

func (reader *leaseFixtureReader) ReadCredential(
	_ context.Context,
	identity CredentialIdentity,
) ([]byte, error) {
	reader.mu.Lock()
	defer reader.mu.Unlock()
	secret, ok := reader.secrets[identity]
	if !ok {
		return nil, ErrVaultUnavailable
	}
	return append([]byte(nil), secret...), nil
}
