package vault

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/credentials"
)

func TestCredentialVaultCoordinatorLifecycleAdvancesExactRevision(t *testing.T) {
	store, _ := newVaultStoreFixture(t)
	defer store.Close()
	leases, err := NewCredentialLeaseManager(store)
	if err != nil {
		t.Fatal(err)
	}
	defer leases.Close()
	committer := newVaultCoordinatorCommitter()
	verifier := &vaultCoordinatorVerifier{}
	coordinator, err := NewCredentialVaultCoordinator(CredentialVaultCoordinatorConfig{
		Store: store, Leases: leases, Verifier: verifier, Committer: committer,
	})
	if err != nil {
		t.Fatal(err)
	}
	base := credentials.CredentialCommand{
		ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
		CredentialReference: "credential-ref-deepseek-primary",
		OccurredAt:          time.Unix(1_000, 0).UTC(),
	}
	configuredSecret := []byte("configured-secret")
	configure := base
	configure.CommandID = "configure-deepseek-primary-r1"
	configure.Secret = configuredSecret
	configured, err := coordinator.Configure(context.Background(), configure)
	if err != nil || configured.Revision != 1 ||
		configured.Status != credentials.CredentialConfigured {
		t.Fatalf("configured = %#v, %v", configured, err)
	}
	assertZeroBytes(t, configuredSecret)
	assertVaultCredential(t, store, coordinatorIdentity(configure, 1), []byte("configured-secret"))

	verify := base
	verify.CommandID = "verify-deepseek-primary-r2"
	verify.ExpectedRevision = 1
	verify.OccurredAt = time.Unix(1_001, 0).UTC()
	verified, err := coordinator.Verify(context.Background(), verify)
	if err != nil || verified.Revision != 2 ||
		verified.Status != credentials.CredentialVerified {
		t.Fatalf("verified = %#v, %v", verified, err)
	}
	if !bytes.Equal(verifier.seen, []byte("configured-secret")) {
		t.Fatalf("verifier saw %q", verifier.seen)
	}
	if secret, err := store.ReadCredential(
		context.Background(), coordinatorIdentity(verify, 1),
	); !errors.Is(err, credentials.ErrCredentialNotFound) {
		clearBytes(secret)
		t.Fatalf("old verified revision readable: %v", err)
	}
	assertVaultCredential(t, store, coordinatorIdentity(verify, 2), []byte("configured-secret"))

	replacementSecret := []byte("replacement-secret")
	replace := base
	replace.CommandID = "replace-deepseek-primary-r3"
	replace.ExpectedRevision = 2
	replace.OccurredAt = time.Unix(1_002, 0).UTC()
	replace.Secret = replacementSecret
	replaced, err := coordinator.Replace(context.Background(), replace)
	if err != nil || replaced.Revision != 3 ||
		replaced.Status != credentials.CredentialConfigured {
		t.Fatalf("replaced = %#v, %v", replaced, err)
	}
	assertZeroBytes(t, replacementSecret)
	assertVaultCredential(t, store, coordinatorIdentity(replace, 3), []byte("replacement-secret"))

	revoke := base
	revoke.CommandID = "revoke-deepseek-primary-r4"
	revoke.ExpectedRevision = 3
	revoke.OccurredAt = time.Unix(1_003, 0).UTC()
	revoked, err := coordinator.Revoke(context.Background(), revoke)
	if err != nil || revoked.Revision != 4 ||
		revoked.Status != credentials.CredentialRevoked {
		t.Fatalf("revoked = %#v, %v", revoked, err)
	}
	if secret, err := store.ReadCredential(
		context.Background(), coordinatorIdentity(revoke, 3),
	); !errors.Is(err, credentials.ErrCredentialNotFound) {
		clearBytes(secret)
		t.Fatalf("revoked revision readable: %v", err)
	}
	pending, err := store.PendingCredentialMutations(context.Background())
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending = %#v, %v", pending, err)
	}
}

func TestCredentialVaultCoordinatorMetadataFailureRollsBackPending(t *testing.T) {
	store, _ := newVaultStoreFixture(t)
	defer store.Close()
	leases, err := NewCredentialLeaseManager(store)
	if err != nil {
		t.Fatal(err)
	}
	defer leases.Close()
	committer := newVaultCoordinatorCommitter()
	committer.fail = true
	coordinator, err := NewCredentialVaultCoordinator(CredentialVaultCoordinatorConfig{
		Store: store, Leases: leases, Verifier: &vaultCoordinatorVerifier{},
		Committer: committer,
	})
	if err != nil {
		t.Fatal(err)
	}
	secret := []byte("must-rollback")
	command := credentials.CredentialCommand{
		CommandID: "configure-deepseek-rollback-r1", ProviderID: "deepseek",
		ProviderAccountID:   "deepseek.primary",
		CredentialReference: "credential-ref-deepseek-primary",
		OccurredAt:          time.Unix(2_000, 0).UTC(), Secret: secret,
	}
	if _, err := coordinator.Configure(
		context.Background(), command,
	); !errors.Is(err, credentials.ErrCredentialMetadataConflict) ||
		credentials.CredentialFailureStage(err) != credentials.CredentialStageMetadataCommit {
		t.Fatalf("configure error = %v", err)
	}
	assertZeroBytes(t, secret)
	pending, err := store.PendingCredentialMutations(context.Background())
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending = %#v, %v", pending, err)
	}
	if secret, err := store.ReadCredential(
		context.Background(), coordinatorIdentity(command, 1),
	); !errors.Is(err, credentials.ErrCredentialNotFound) {
		clearBytes(secret)
		t.Fatalf("rolled-back configure readable: %v", err)
	}
}

func TestCredentialVaultCoordinatorReconcilesCommittedMetadataAfterFinalizeFailure(
	t *testing.T,
) {
	store, _ := newVaultStoreFixture(t)
	defer store.Close()
	leasing, err := NewCredentialLeaseManager(store)
	if err != nil {
		t.Fatal(err)
	}
	defer leasing.Close()
	failingStore := &vaultCoordinatorFailingCommitStore{VaultStore: store, fail: true}
	committer := newVaultCoordinatorCommitter()
	coordinator, err := NewCredentialVaultCoordinator(CredentialVaultCoordinatorConfig{
		Store: failingStore, Leases: leasing, Verifier: &vaultCoordinatorVerifier{},
		Committer: committer,
	})
	if err != nil {
		t.Fatal(err)
	}
	command := credentials.CredentialCommand{
		CommandID: "configure-deepseek-recovery-r1", ProviderID: "deepseek",
		ProviderAccountID:   "deepseek.primary",
		CredentialReference: "credential-ref-deepseek-primary",
		OccurredAt:          time.Unix(3_000, 0).UTC(),
		Secret:              []byte("recovery-secret"),
	}
	if _, err := coordinator.Configure(
		context.Background(), command,
	); !errors.Is(err, credentials.ErrCredentialStoreUnavailable) ||
		credentials.CredentialFailureStage(err) != credentials.CredentialStageVaultCommit {
		t.Fatalf("configure error = %v", err)
	}
	pending, err := store.PendingCredentialMutations(context.Background())
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending = %#v, %v", pending, err)
	}
	metadata, found := committer.current(command)
	if !found || metadata.Revision != 1 {
		t.Fatalf("metadata = %#v, found=%v", metadata, found)
	}
	if err := coordinator.ReconcileCredentialMutation(
		context.Background(), pending[0], metadata, true,
	); err != nil {
		t.Fatal(err)
	}
	assertVaultCredential(
		t, store, coordinatorIdentity(command, 1), []byte("recovery-secret"),
	)
	pending, err = store.PendingCredentialMutations(context.Background())
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending after recovery = %#v, %v", pending, err)
	}
}

func TestCredentialVaultCoordinatorExplicitReplaceImportsMissingVaultCredential(
	t *testing.T,
) {
	store, _ := newVaultStoreFixture(t)
	defer store.Close()
	leasing, err := NewCredentialLeaseManager(store)
	if err != nil {
		t.Fatal(err)
	}
	defer leasing.Close()
	committer := newVaultCoordinatorCommitter()
	command := credentials.CredentialCommand{
		CommandID: "replace-deepseek-migration-r4", ProviderID: "deepseek",
		ProviderAccountID:   "deepseek.primary",
		CredentialReference: "credential-ref-deepseek-primary",
		ExpectedRevision:    3, OccurredAt: time.Unix(4_000, 0).UTC(),
		Secret: []byte("reentered-secret"),
	}
	committer.seed(command, credentials.MetadataResult{
		ProviderID: command.ProviderID, ProviderAccountID: command.ProviderAccountID,
		CredentialReference: command.CredentialReference,
		Revision:            3, Status: credentials.CredentialVerified,
	})
	coordinator, err := NewCredentialVaultCoordinator(CredentialVaultCoordinatorConfig{
		Store: store, Leases: leasing, Verifier: &vaultCoordinatorVerifier{},
		Committer: committer,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := coordinator.Replace(context.Background(), command)
	if err != nil || result.Revision != 4 ||
		result.Status != credentials.CredentialConfigured {
		t.Fatalf("replace import = %#v, %v", result, err)
	}
	assertVaultCredential(
		t, store, coordinatorIdentity(command, 4), []byte("reentered-secret"),
	)
}

type vaultCoordinatorVerifier struct {
	seen []byte
}

func (verifier *vaultCoordinatorVerifier) Verify(
	_ context.Context,
	providerID string,
	secret []byte,
) (credentials.VerificationResult, error) {
	if providerID != "deepseek" {
		return credentials.VerificationResult{}, credentials.ErrCredentialRejected
	}
	verifier.seen = append([]byte(nil), secret...)
	return credentials.VerificationResult{
		Status: credentials.VerificationValid,
		Reason: credentials.VerificationReasonNone,
	}, nil
}

type vaultCoordinatorCommitter struct {
	mu        sync.Mutex
	revisions map[string]int64
	results   map[string]credentials.MetadataResult
	fail      bool
}

func newVaultCoordinatorCommitter() *vaultCoordinatorCommitter {
	return &vaultCoordinatorCommitter{
		revisions: make(map[string]int64),
		results:   make(map[string]credentials.MetadataResult),
	}
}

func (committer *vaultCoordinatorCommitter) CommitCredentialMetadata(
	_ context.Context,
	command credentials.MetadataCommand,
) (credentials.MetadataResult, error) {
	committer.mu.Lock()
	defer committer.mu.Unlock()
	if committer.fail {
		return credentials.MetadataResult{}, credentials.ErrCredentialMetadataConflict
	}
	key := command.ProviderID + "\x00" + command.ProviderAccountID + "\x00" +
		command.CredentialReference
	if committer.revisions[key] != command.ExpectedRevision {
		return credentials.MetadataResult{}, credentials.ErrCredentialMetadataConflict
	}
	committer.revisions[key]++
	result := credentials.MetadataResult{
		ProviderID: command.ProviderID, ProviderAccountID: command.ProviderAccountID,
		CredentialReference: command.CredentialReference,
		Revision:            committer.revisions[key], Status: command.Status, Reason: command.Reason,
	}
	committer.results[key] = result
	return result, nil
}

func (committer *vaultCoordinatorCommitter) current(
	command credentials.CredentialCommand,
) (credentials.MetadataResult, bool) {
	committer.mu.Lock()
	defer committer.mu.Unlock()
	key := command.ProviderID + "\x00" + command.ProviderAccountID + "\x00" +
		command.CredentialReference
	result, ok := committer.results[key]
	return result, ok
}

func (committer *vaultCoordinatorCommitter) seed(
	command credentials.CredentialCommand,
	result credentials.MetadataResult,
) {
	committer.mu.Lock()
	defer committer.mu.Unlock()
	key := command.ProviderID + "\x00" + command.ProviderAccountID + "\x00" +
		command.CredentialReference
	committer.revisions[key] = result.Revision
	committer.results[key] = result
}

type vaultCoordinatorFailingCommitStore struct {
	*VaultStore
	fail bool
}

func (store *vaultCoordinatorFailingCommitStore) CommitCredentialMutation(
	ctx context.Context,
	receipt CredentialMutationReceipt,
) error {
	if store.fail {
		store.fail = false
		return credentials.ErrCredentialStoreUnavailable
	}
	return store.VaultStore.CommitCredentialMutation(ctx, receipt)
}

func assertZeroBytes(t *testing.T, value []byte) {
	t.Helper()
	for _, character := range value {
		if character != 0 {
			t.Fatal("secret bytes were not cleared")
		}
	}
}
