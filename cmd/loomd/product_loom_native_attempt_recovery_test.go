package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"loom-pi-rebuild/internal/agentcheckpoint"
	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/journal"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/runtime/nativeadapter"
	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/work"
)

func TestProductLoomNativeAttemptRecoveryResumesExactEncryptedStateAndTerminatesLoop(t *testing.T) {
	ctx := context.Background()
	profile := productAttemptLoopProfile("account.primary")
	capsule := productAttemptLoopCapsuleValue(t, profile)
	contextPayload, err := contextcapsule.RenderDispatchPayload(capsule)
	if err != nil {
		t.Fatal(err)
	}
	capsuleStore := &productRecoveryCapsuleStore{
		capsule: capsule, payload: contextPayload,
	}
	checkpointStore := &productAgentCheckpointStoreFixture{}
	var credentialCalls atomic.Int64
	provider := &productRecoveryContinuationHTTPClient{}
	adapter, err := nativeadapter.NewDeepSeekAgentAdapter(nativeadapter.DeepSeekAgentAdapterConfig{
		RuntimeInstanceID: "runtime-1",
		CredentialAccess:  &productRecoveryContinuationCredentialAccess{calls: &credentialCalls},
		Diagnostics:       productRecoveryDiagnostics{}, Client: provider,
		Now:              func() time.Time { return time.Date(2026, 8, 14, 20, 30, 0, 0, time.UTC) },
		MaxResponseBytes: 64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	var recovery *productLoomNativeAttemptRecovery
	var recoveredInbox *work.AgentInboxCoordinator
	var recoveredLoops *work.AttemptLoopAuthority
	var recoveredJournal *journal.Store
	var recoveredOutcome work.AgentAttemptRestartOutcome
	ctx, lease, query, _ := productAgentAttemptRecoveryLeaseFixtureWithResolver(
		t, "continuation",
		func(outcome work.AgentAttemptRestartOutcome) work.AgentAttemptRestartCapabilityResolver {
			candidate, constructErr := newProductLoomNativeAttemptContinuation(
				[]supervisor.RuntimeAdapter{adapter}, capsuleStore, checkpointStore,
				recoveredInbox, recoveredLoops,
			)
			if constructErr != nil {
				t.Fatal(constructErr)
			}
			recovery = candidate
			return candidate
		},
		func(
			inbox *work.AgentInboxCoordinator,
			loops *work.AttemptLoopAuthority,
			journalStore *journal.Store,
			outcome work.AgentAttemptRestartOutcome,
		) {
			recoveredInbox, recoveredLoops, recoveredJournal, recoveredOutcome =
				inbox, loops, journalStore, outcome
			authority := outcome.Binding.PayloadAuthority
			checkpoint := agentcheckpoint.Payload{
				Binding: agentcheckpoint.Binding{
					CheckpointID:   "checkpoint-recovery-continuation",
					ConversationID: authority.ConversationID,
					SegmentID:      outcome.SegmentID, AttemptID: outcome.Binding.AttemptID,
					AgentInstanceID: authority.AgentInstanceID,
					WorkItemID:      authority.WorkItemID, RunID: authority.RunID,
					ClaimGeneration:        authority.ClaimGeneration,
					RuntimeInstanceID:      authority.RuntimeInstanceID,
					ExecutionBindingDigest: authority.ExecutionBindingDigest,
					CapsuleDigest:          authority.CapsuleDigest,
					TurnID:                 "turn-1", TurnSequence: 1,
					StepID: "step-1", StepSequence: 1,
					ContentType:   agentcheckpoint.ContentTypeTextUTF8,
					ContentDigest: outcome.CheckpointDigest,
				},
				Content: []byte(productRecoveryCheckpointContent),
			}
			if putErr := checkpointStore.PutAgentCheckpoint(ctx, checkpoint); putErr != nil {
				t.Fatal(putErr)
			}
		},
	)
	if recovery == nil || recoveredInbox == nil || recoveredLoops == nil {
		t.Fatal("continuation recovery fixture was not captured")
	}
	registry := newProductActiveAttemptRegistry()
	runtime, err := newProductAgentAttemptRecoveryRuntime(registry, recovery)
	if err != nil {
		t.Fatal(err)
	}
	sink := &productNativeAgentInputFrameSink{}
	result, err := runtime.Resume(ctx, lease, sink)
	if err != nil {
		t.Fatal(err)
	}
	if credentialCalls.Load() != 1 || provider.calls != 1 || len(provider.bodies) != 1 ||
		!bytes.Contains(provider.bodies[0], []byte(productRecoveryCheckpointContent)) ||
		!bytes.Contains(provider.bodies[0], []byte("private recovered input continuation")) ||
		!bytes.Contains(provider.bodies[0], []byte(capsule.Digest())) ||
		!result.ResultAcknowledged() || len(sink.frames) != 3 {
		t.Fatalf("resumed continuation = credentials=%d provider=%d bodies=%q result=%#v frames=%#v",
			credentialCalls.Load(), provider.calls, provider.bodies, result, sink.frames)
	}
	snapshot, err := recoveredLoops.Snapshot(ctx, recoveredOutcome.Binding)
	if err != nil || snapshot.Status != work.AttemptLoopRunning ||
		len(snapshot.Turns) != 2 || snapshot.Turns[1].Status != work.AttemptTurnSucceeded ||
		len(snapshot.Turns[1].Steps) != 1 ||
		snapshot.Turns[1].Steps[0].Outcome != work.AttemptStepFinal {
		t.Fatalf("resumed Attempt loop = %#v, %v", snapshot, err)
	}
	if len(checkpointStore.payloads) != 0 {
		t.Fatalf("terminal checkpoint remained = %#v", checkpointStore.payloads)
	}
	if _, err := registry.Resolve(query); !errors.Is(err, errProductActiveAttemptNotFound) {
		t.Fatalf("terminal recovery remained active = %v", err)
	}
	events, err := recoveredJournal.ReadAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range [][]byte{
		[]byte(productRecoveryCheckpointContent),
		[]byte("private recovered input continuation"),
		[]byte("Recovered final answer"),
		[]byte("test-only-recovery-secret"),
	} {
		if bytes.Contains(encoded, forbidden) {
			t.Fatalf("recovery plaintext entered Journal: %q", forbidden)
		}
	}
}

func TestProductRecoveryCheckpointCleanupOutlivesCancelledRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	store := &productRecoveryCleanupCheckpointStore{
		productAgentCheckpointStoreFixture: &productAgentCheckpointStoreFixture{},
	}
	binding := agentcheckpoint.Binding{CheckpointID: "checkpoint-cancelled-cleanup"}
	if err := deleteProductRecoveryCheckpoint(ctx, store, binding); err != nil {
		t.Fatal(err)
	}
	if store.sawCancelledContext {
		t.Fatal("checkpoint cleanup inherited cancelled request context")
	}
}

type productRecoveryCleanupCheckpointStore struct {
	*productAgentCheckpointStoreFixture
	sawCancelledContext bool
}

func (store *productRecoveryCleanupCheckpointStore) DeleteAgentCheckpoint(
	ctx context.Context,
	_ agentcheckpoint.Binding,
) error {
	store.sawCancelledContext = ctx.Err() != nil
	return ctx.Err()
}

type productRecoveryCapsuleStore struct {
	capsule contextcapsule.RoleContextCapsule
	payload []byte
}

func (store *productRecoveryCapsuleStore) PutRoleContextCapsule(
	context.Context,
	contextcapsule.RoleContextCapsule,
	[]byte,
) error {
	return errors.New("unexpected Capsule write")
}

func (store *productRecoveryCapsuleStore) ReadRoleContextCapsule(
	_ context.Context,
	authority contextcapsule.AuthorityRecord,
) (contextcapsule.RoleContextCapsule, []byte, error) {
	if store.capsule.AuthorityRecord() != authority {
		return contextcapsule.RoleContextCapsule{}, nil, errors.New("Capsule not found")
	}
	return store.capsule, bytes.Clone(store.payload), nil
}

func (store *productRecoveryCapsuleStore) ListRoleContextCapsuleAuthorities(
	context.Context,
	string,
) ([]contextcapsule.AuthorityRecord, error) {
	return []contextcapsule.AuthorityRecord{store.capsule.AuthorityRecord()}, nil
}

type productRecoveryContinuationCredentialAccess struct{ calls *atomic.Int64 }

func (access *productRecoveryContinuationCredentialAccess) UseCredential(
	ctx context.Context,
	_ loomruntime.FrozenExecutionBinding,
	use func(context.Context, []byte) error,
) error {
	access.calls.Add(1)
	secret := []byte("test-only-recovery-secret")
	defer clearProductRecoveryBytes(secret)
	return use(ctx, secret)
}

type productRecoveryContinuationHTTPClient struct {
	calls  int
	bodies [][]byte
}

func (client *productRecoveryContinuationHTTPClient) Do(
	request *http.Request,
) (*http.Response, error) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}
	client.calls++
	client.bodies = append(client.bodies, body)
	request.Header.Del("Authorization")
	return &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"Recovered final answer"}}],` +
				`"usage":{"prompt_tokens":18,"completion_tokens":4,"total_tokens":22}}`,
		)),
	}, nil
}

func TestProductLoomNativeAttemptRecoveryMintsAndReattachesExactCheckpoint(t *testing.T) {
	var executionCalls atomic.Int64
	adapter := productLoomNativeRecoveryAdapterFixture(t, &executionCalls)
	capsule := productAttemptLoopCapsuleValue(t, productAttemptLoopProfile("account.primary")).AuthorityRecord()
	source := &productRecoveryCapsuleAuthoritySource{authorities: []contextcapsule.AuthorityRecord{capsule}}
	recovery, err := newProductLoomNativeAttemptRecovery([]supervisor.RuntimeAdapter{adapter}, source)
	if err != nil {
		t.Fatal(err)
	}

	ctx, lease, query, capability := productAgentAttemptRecoveryLeaseFixtureWithResolver(
		t, "loom-native", func(work.AgentAttemptRestartOutcome) work.AgentAttemptRestartCapabilityResolver {
			return recovery
		}, nil,
	)
	if capability.SessionBindingDigest == "" || capability.CapabilityDigest == "" {
		t.Fatalf("capability = %#v", capability)
	}
	_, probeLease, _, _ := productAgentAttemptRecoveryLeaseFixture(t, "digest")
	probeGrant, err := probeLease.Take()
	if err != nil {
		t.Fatal(err)
	}
	probeOutcome := probeGrant.Outcome()
	baseline, err := recovery.ResolveAgentAttemptRestartCapability(ctx, probeOutcome)
	if err != nil {
		t.Fatal(err)
	}
	changedSegment := probeOutcome
	changedSegment.SegmentID = "segment-changed"
	segmentCapability, err := recovery.ResolveAgentAttemptRestartCapability(ctx, changedSegment)
	if err != nil {
		t.Fatal(err)
	}
	changedCheckpoint := probeOutcome
	changedCheckpoint.CheckpointDigest = "9999999999999999999999999999999999999999999999999999999999999999"
	checkpointCapability, err := recovery.ResolveAgentAttemptRestartCapability(ctx, changedCheckpoint)
	if err != nil {
		t.Fatal(err)
	}
	if baseline.SessionBindingDigest == segmentCapability.SessionBindingDigest ||
		baseline.SessionBindingDigest == checkpointCapability.SessionBindingDigest {
		t.Fatal("restart session digest did not freeze Segment and checkpoint")
	}
	registry := newProductActiveAttemptRegistry()
	attachmentRuntime, err := newProductAgentAttemptRecoveryRuntime(registry, recovery)
	if err != nil {
		t.Fatal(err)
	}
	attachment, active, err := attachmentRuntime.Attach(ctx, lease)
	if err != nil {
		t.Fatal(err)
	}
	if active.Identity != query || active.ExecutionBinding.BindingDigest != capability.ExecutionBindingDigest {
		t.Fatalf("active Attempt = %#v", active)
	}
	if executionCalls.Load() != 0 {
		t.Fatalf("recovery called Runtime Execute %d times", executionCalls.Load())
	}
	if err := attachment.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestProductLoomNativeAttemptRecoveryRejectsUnsupportedAndDriftedState(t *testing.T) {
	var executionCalls atomic.Int64
	adapter := productLoomNativeRecoveryAdapterFixture(t, &executionCalls)
	capsule := productAttemptLoopCapsuleValue(t, productAttemptLoopProfile("account.primary")).AuthorityRecord()
	source := &productRecoveryCapsuleAuthoritySource{authorities: []contextcapsule.AuthorityRecord{capsule}}
	recovery, err := newProductLoomNativeAttemptRecovery([]supervisor.RuntimeAdapter{adapter}, source)
	if err != nil {
		t.Fatal(err)
	}
	_, lease, _, _ := productAgentAttemptRecoveryLeaseFixtureWithResolver(
		t, "drift", func(work.AgentAttemptRestartOutcome) work.AgentAttemptRestartCapabilityResolver {
			return recovery
		}, nil,
	)
	source.authorities = nil
	registry := newProductActiveAttemptRegistry()
	attachmentRuntime, err := newProductAgentAttemptRecoveryRuntime(registry, recovery)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := attachmentRuntime.Attach(context.Background(), lease); !errors.Is(err, errProductAttemptRecoveryReattach) {
		t.Fatalf("capsule drift error = %v", err)
	}
	if executionCalls.Load() != 0 {
		t.Fatalf("failed recovery called Runtime Execute %d times", executionCalls.Load())
	}

	validSource := &productRecoveryCapsuleAuthoritySource{authorities: []contextcapsule.AuthorityRecord{
		{CapsuleDigest: "unrelated-corrupt-record"}, capsule,
	}}
	isolated, err := newProductLoomNativeAttemptRecovery([]supervisor.RuntimeAdapter{adapter}, validSource)
	if err != nil {
		t.Fatal(err)
	}
	_, probeLease, _, _ := productAgentAttemptRecoveryLeaseFixture(t, "isolated")
	probeGrant, err := probeLease.Take()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := isolated.ResolveAgentAttemptRestartCapability(context.Background(), probeGrant.Outcome()); err != nil {
		t.Fatalf("unrelated corrupt Capsule blocked exact Agent: %v", err)
	}
	duplicateSource := &productRecoveryCapsuleAuthoritySource{authorities: []contextcapsule.AuthorityRecord{capsule, capsule}}
	duplicate, err := newProductLoomNativeAttemptRecovery([]supervisor.RuntimeAdapter{adapter}, duplicateSource)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := duplicate.ResolveAgentAttemptRestartCapability(context.Background(), probeGrant.Outcome()); !errors.Is(err, errProductAttemptRecoveryReattach) {
		t.Fatalf("duplicate exact Capsule error = %v", err)
	}

	unsupported, err := newProductLoomNativeAttemptRecovery(
		[]supervisor.RuntimeAdapter{&productUnsupportedRecoveryAdapter{}},
		&productRecoveryCapsuleAuthoritySource{authorities: []contextcapsule.AuthorityRecord{capsule}},
	)
	if err != nil {
		t.Fatal(err)
	}
	grantLeaseCtx, grantLease, _, _ := productAgentAttemptRecoveryLeaseFixture(t, "unsupported")
	grant, err := grantLease.Take()
	if err != nil {
		t.Fatal(err)
	}
	outcome := grant.Outcome()
	if _, err := unsupported.ResolveAgentAttemptRestartCapability(grantLeaseCtx, outcome); !errors.Is(err, errProductAttemptRecoveryReattach) {
		t.Fatalf("unsupported Runtime error = %v", err)
	}
	if _, err := newProductLoomNativeAttemptRecovery(
		[]supervisor.RuntimeAdapter{adapter, adapter}, source,
	); !errors.Is(err, errProductInvalidAttemptRecoveryAttachment) {
		t.Fatalf("duplicate Runtime constructor error = %v", err)
	}
}

func productLoomNativeRecoveryAdapterFixture(
	t *testing.T,
	executionCalls *atomic.Int64,
) supervisor.RuntimeAdapter {
	t.Helper()
	delegate, err := nativeadapter.NewDeepSeekAgentAdapter(nativeadapter.DeepSeekAgentAdapterConfig{
		RuntimeInstanceID: "runtime-1",
		CredentialAccess:  &productRecoveryCredentialAccess{calls: executionCalls},
		Diagnostics:       productRecoveryDiagnostics{},
		Client:            productRecoveryHTTPClient{},
		Now:               func() time.Time { return time.Date(2026, 8, 14, 20, 0, 0, 0, time.UTC) },
		MaxResponseBytes:  64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	return delegate
}

type productRecoveryCapsuleAuthoritySource struct {
	authorities []contextcapsule.AuthorityRecord
}

func (source *productRecoveryCapsuleAuthoritySource) ListRoleContextCapsuleAuthorities(
	_ context.Context,
	_ string,
) ([]contextcapsule.AuthorityRecord, error) {
	return append([]contextcapsule.AuthorityRecord(nil), source.authorities...), nil
}

type productRecoveryCredentialAccess struct{ calls *atomic.Int64 }

func (access *productRecoveryCredentialAccess) UseCredential(
	context.Context,
	loomruntime.FrozenExecutionBinding,
	func(context.Context, []byte) error,
) error {
	access.calls.Add(1)
	return errors.New("credential access must not occur during recovery attachment")
}

type productRecoveryDiagnostics struct{}

func (productRecoveryDiagnostics) RecordAgentAttemptDiagnostic(
	context.Context,
	nativeadapter.AgentAttemptDiagnostic,
) error {
	return nil
}

type productRecoveryHTTPClient struct{}

func (productRecoveryHTTPClient) Do(*http.Request) (*http.Response, error) {
	return nil, errors.New("HTTP must not occur during recovery attachment")
}

type productUnsupportedRecoveryAdapter struct{}

func (*productUnsupportedRecoveryAdapter) AdapterType() string       { return "codex" }
func (*productUnsupportedRecoveryAdapter) RuntimeInstanceID() string { return "runtime-1" }
func (*productUnsupportedRecoveryAdapter) Execute(
	context.Context,
	supervisor.AdapterRequest,
) (supervisor.AdapterResult, error) {
	return supervisor.AdapterResult{}, errors.New("must not execute")
}
