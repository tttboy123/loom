package main

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/credentials"
	credentialvault "loom-pi-rebuild/internal/credentials/vault"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/projection"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/runtime/piadapter"
	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/work"
)

func TestCOMP2DConversationAttemptScopeFreezesSnapshotAndBinding(t *testing.T) {
	recorder := &compositionTestRecorder{}
	slot := &productCapabilityScopeSlot{}
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}),
		"incident-comp2d-conversation-scope", recorder,
		productCompatibilityConstruction{scopeSlot: slot},
	)
	if err != nil {
		t.Fatal(err)
	}
	bindingDigest := strings.Repeat("a", 64)
	lease, err := slot.OpenConversationAttempt(
		context.Background(),
		api.LocalProductConversationScopeRequest{
			ConversationID:         "conversation-1",
			TeamID:                 "conversation:conversation-1",
			AgentID:                "conversation-agent:loom",
			AttemptID:              "attempt-1",
			TurnID:                 "attempt-1:turn-1",
			TurnGeneration:         1,
			ExecutionBindingDigest: bindingDigest,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if lease.CompositionSnapshotDigest() != facade.Snapshot().Digest ||
		lease.ExecutionBindingDigest() != bindingDigest {
		t.Fatalf("snapshot=%q binding=%q", lease.CompositionSnapshotDigest(), lease.ExecutionBindingDigest())
	}
	if err := lease.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := facade.Close(); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []composition.ScopeKind{
		composition.ScopeConversation, composition.ScopeTeam, composition.ScopeAgent,
		composition.ScopeAttempt, composition.ScopeTurn,
	} {
		if !compositionRecorderHasScopeStage(recorder, kind, composition.StageScopeOpen) ||
			!compositionRecorderHasScopeStage(recorder, kind, composition.StageScopeClose) {
			t.Fatalf("missing lifecycle for %s: %#v", kind, recorder.snapshot())
		}
	}
	for _, record := range recorder.snapshot() {
		if record.ScopeKind != "" && strings.Contains(record.ScopeID, "conversation-1") {
			t.Fatalf("scope diagnostic exposed source identity: %#v", record)
		}
	}
}

func TestCOMP2DConversationAttemptScopeCancelsActiveExecutionOnProductClose(t *testing.T) {
	slot := &productCapabilityScopeSlot{}
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}),
		"incident-comp2d-conversation-cancel", &compositionTestRecorder{},
		productCompatibilityConstruction{scopeSlot: slot},
	)
	if err != nil {
		t.Fatal(err)
	}
	lease, err := slot.OpenConversationAttempt(
		context.Background(), api.LocalProductConversationScopeRequest{
			ConversationID:         "conversation-cancel",
			TeamID:                 "conversation:conversation-cancel",
			AgentID:                "conversation-agent:loom",
			AttemptID:              "attempt-cancel",
			TurnID:                 "attempt-cancel:turn-1",
			TurnGeneration:         1,
			ExecutionBindingDigest: strings.Repeat("c", 64),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	executionContext := lease.ExecutionContext()
	if executionContext == nil || executionContext.Err() != nil {
		t.Fatalf("execution context=%v err=%v", executionContext, executionContext.Err())
	}
	if err := facade.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-executionContext.Done():
		if !errors.Is(context.Cause(executionContext), composition.ErrScopeClosed) {
			t.Fatalf("cause=%v", context.Cause(executionContext))
		}
	case <-time.After(time.Second):
		t.Fatal("Product close did not cancel Conversation Attempt execution")
	}
}

func TestCOMP2DConversationAttemptOwnsExternalSessionHandleLease(t *testing.T) {
	root := t.TempDir()
	privateDirectory := filepath.Join(root, "private")
	stateDirectory := filepath.Join(root, "state")
	for _, directory := range []string{privateDirectory, stateDirectory} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	material, err := (credentialvault.LocalKeyFile{
		Path: filepath.Join(privateDirectory, "vault.key"),
	}).LoadOrCreate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	store, err := credentialvault.OpenStore(credentialvault.StoreConfig{
		DatabasePath: filepath.Join(stateDirectory, "credential-vault.db"),
		KeyMaterial:  material,
	})
	if err != nil {
		t.Fatal(err)
	}
	runtime := &productCredentialVaultRuntime{store: store}
	defer runtime.Close()

	binding := credentialvault.ExternalSessionHandleBinding{
		ConversationID: "conversation-session", SegmentID: "segment-2",
		ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
		ModelID: "deepseek-chat", AuthMode: "brokered",
		CredentialReference: "credential-ref-deepseek-primary",
		CredentialRevision:  9,
	}
	handle := credentialvault.ExternalSessionHandle{
		Binding: binding, Kind: credentialvault.ExternalSessionHandleConversationID,
		Revision: 4, Value: []byte("attempt-owned-provider-session"),
	}
	if err := runtime.PutExternalSessionHandle(context.Background(), handle); err != nil {
		t.Fatal(err)
	}

	slot := &productCapabilityScopeSlot{}
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}),
		"incident-comp2d-conversation-session", &compositionTestRecorder{},
		productCompatibilityConstruction{scopeSlot: slot},
	)
	if err != nil {
		t.Fatal(err)
	}
	lease, err := slot.OpenConversationAttempt(
		context.Background(), api.LocalProductConversationScopeRequest{
			ConversationID:         "conversation-session",
			TeamID:                 "conversation:conversation-session",
			AgentID:                "conversation-agent:loom",
			AttemptID:              "attempt-session",
			TurnID:                 "attempt-session:turn-1",
			TurnGeneration:         1,
			ExecutionBindingDigest: strings.Repeat("e", 64),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	finished := make(chan error, 1)
	var plaintext []byte
	go func() {
		finished <- runtime.UseExternalSessionHandle(
			lease.ExecutionContext(), binding,
			credentialvault.ExternalSessionHandleConversationID,
			func(ctx context.Context, value []byte, revision int64) error {
				if revision != 4 || string(value) != "attempt-owned-provider-session" {
					return errors.New("unexpected Provider session handle")
				}
				plaintext = value
				close(started)
				<-ctx.Done()
				return context.Cause(ctx)
			},
		)
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("ExternalSessionHandle lease did not start")
	}
	if err := facade.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		if !errors.Is(err, composition.ErrScopeClosed) {
			t.Fatalf("session lease err=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Product close did not terminate ExternalSessionHandle lease")
	}
	for _, value := range plaintext {
		if value != 0 {
			t.Fatal("ExternalSessionHandle plaintext was not zeroized")
		}
	}
}

func TestCOMP2DTeamScopeIsolatesAgentAttemptsWithFrozenBindings(t *testing.T) {
	recorder := &compositionTestRecorder{}
	slot := &productCapabilityScopeSlot{}
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}),
		"incident-comp2d-team-scope", recorder,
		productCompatibilityConstruction{scopeSlot: slot},
	)
	if err != nil {
		t.Fatal(err)
	}
	team, err := slot.OpenTeamExecution(
		context.Background(), "team-mixed", "execution-mixed",
	)
	if err != nil {
		t.Fatal(err)
	}
	for index, agentID := range []string{"agent-claude", "agent-deepseek"} {
		digest := strings.Repeat(string(rune('d'+index)), 64)
		attempt, openErr := team.OpenAgentAttempt(
			context.Background(), productAgentAttemptScopeRequest{
				AgentID: agentID, AttemptID: "run-" + agentID,
				TurnID: "run-" + agentID + ":turn-1", TurnGeneration: 1,
				ExecutionBindingDigest: digest,
			},
		)
		if openErr != nil {
			t.Fatal(openErr)
		}
		if attempt.CompositionSnapshotDigest() != facade.Snapshot().Digest ||
			attempt.ExecutionBindingDigest() != digest {
			t.Fatalf("agent=%s snapshot=%q binding=%q", agentID,
				attempt.CompositionSnapshotDigest(), attempt.ExecutionBindingDigest())
		}
		if err := attempt.Close(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if err := team.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := compositionRecorderScopeStageCount(
		recorder, composition.ScopeAgent, composition.StageScopeOpen,
	); got != 2 {
		t.Fatalf("agent opens=%d", got)
	}
	if got := compositionRecorderScopeStageCount(
		recorder, composition.ScopeAttempt, composition.StageScopeOpen,
	); got != 2 {
		t.Fatalf("attempt opens=%d", got)
	}
	if err := facade.Close(); err != nil {
		t.Fatal(err)
	}
	for _, record := range recorder.snapshot() {
		if record.ScopeKind != "" &&
			(strings.Contains(record.ScopeID, "team-mixed") ||
				strings.Contains(record.ScopeID, "agent-claude") ||
				strings.Contains(record.ScopeID, "agent-deepseek")) {
			t.Fatalf("scope diagnostic exposed source identity: %#v", record)
		}
	}
}

type productScopedExecutorFixture struct {
	scopeOpened *bool
	executed    bool
	err         error
	context     context.Context
	started     chan<- struct{}
	waitForDone bool
}

func (executor *productScopedExecutorFixture) Execute(
	ctx context.Context,
	_ supervisor.ExecuteInput,
) (supervisor.Outcome, error) {
	if executor.scopeOpened != nil && !*executor.scopeOpened {
		return supervisor.Outcome{}, errors.New("executor ran outside Agent Attempt scope")
	}
	executor.executed = true
	executor.context = ctx
	if executor.started != nil {
		executor.started <- struct{}{}
	}
	if executor.waitForDone {
		<-ctx.Done()
		return supervisor.Outcome{}, context.Cause(ctx)
	}
	return supervisor.Outcome{}, executor.err
}

func (*productScopedExecutorFixture) Close(context.Context) error { return nil }

type productTeamScopeFixture struct {
	opened  bool
	request productAgentAttemptScopeRequest
	err     error
}

func (scope *productTeamScopeFixture) OpenAgentAttempt(
	_ context.Context,
	request productAgentAttemptScopeRequest,
) (productAgentAttemptScopeLease, error) {
	scope.request = request
	if scope.err != nil {
		return nil, scope.err
	}
	scope.opened = true
	return &productAttemptScopeLeaseFixture{scope: scope}, nil
}

func (*productTeamScopeFixture) Close(context.Context) error { return nil }

type productAttemptScopeLeaseFixture struct{ scope *productTeamScopeFixture }

func (*productAttemptScopeLeaseFixture) CompositionSnapshotDigest() string {
	return strings.Repeat("a", 64)
}
func (lease *productAttemptScopeLeaseFixture) ExecutionBindingDigest() string {
	return lease.scope.request.ExecutionBindingDigest
}
func (*productAttemptScopeLeaseFixture) Close(context.Context) error { return nil }
func (*productAttemptScopeLeaseFixture) ExecutionContext() context.Context {
	return context.Background()
}
func (*productAttemptScopeLeaseFixture) AdvanceTurn(context.Context, int64) error {
	return nil
}

func TestCOMP2DScopedMissionExecutorOpensFrozenScopeBeforeDelegate(t *testing.T) {
	profile, instance := productScopeRuntimeFixture(t)
	scope := &productTeamScopeFixture{}
	delegate := &productScopedExecutorFixture{scopeOpened: &scope.opened}
	executor := &productMissionScopedExecutor{delegate: delegate, team: scope}
	_, err := executor.Execute(context.Background(), supervisor.ExecuteInput{
		Profile: profile, Instance: instance,
		Generation: work.RunGenerationInput{
			RunID: "run-agent", AgentInstanceID: "agent-1", ClaimGeneration: 1,
		},
	})
	if err != nil || !delegate.executed || !scope.opened {
		t.Fatalf("err=%v executed=%t opened=%t", err, delegate.executed, scope.opened)
	}
	if _, found := productAttemptTurnScopeFromContext(delegate.context); !found {
		t.Fatal("delegate context missing Attempt Turn controller")
	}
	binding, err := loomruntime.FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatal(err)
	}
	if scope.request.AgentID != "agent-1" || scope.request.AttemptID != "run-agent" ||
		scope.request.ExecutionBindingDigest != binding.BindingDigest {
		t.Fatalf("scope request=%#v binding=%#v", scope.request, binding)
	}
}

func TestCOMP2DAttemptOwnsExecutionCancellation(t *testing.T) {
	recorder := &compositionTestRecorder{}
	slot := &productCapabilityScopeSlot{}
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}),
		"incident-comp2d-attempt-cancel", recorder,
		productCompatibilityConstruction{scopeSlot: slot},
	)
	if err != nil {
		t.Fatal(err)
	}
	team, err := slot.OpenTeamExecution(
		context.Background(), "team-cancel", "execution-cancel",
	)
	if err != nil {
		t.Fatal(err)
	}
	profile, instance := productScopeRuntimeFixture(t)
	started := make(chan struct{}, 1)
	delegate := &productScopedExecutorFixture{started: started, waitForDone: true}
	executor := &productMissionScopedExecutor{delegate: delegate, team: team}
	executed := make(chan error, 1)
	go func() {
		_, executeErr := executor.Execute(context.Background(), supervisor.ExecuteInput{
			Profile: profile, Instance: instance,
			Generation: work.RunGenerationInput{
				RunID: "run-cancel", AgentInstanceID: "agent-cancel", ClaimGeneration: 1,
			},
		})
		executed <- executeErr
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("delegate did not start")
	}
	if delegate.context == nil || delegate.context.Err() != nil {
		t.Fatalf("execution context before close=%v", delegate.context)
	}
	if err := team.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case executeErr := <-executed:
		if !errors.Is(executeErr, composition.ErrScopeClosed) {
			t.Fatalf("execute error=%v", executeErr)
		}
	case <-time.After(time.Second):
		t.Fatal("scope close did not cancel delegate")
	}
	if !errors.Is(context.Cause(delegate.context), composition.ErrScopeClosed) {
		t.Fatalf("context cause=%v", context.Cause(delegate.context))
	}
	if err := facade.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCOMP2DAttemptCancelsExecutionContextAfterReturn(t *testing.T) {
	slot := &productCapabilityScopeSlot{}
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}),
		"incident-comp2d-attempt-return", &compositionTestRecorder{},
		productCompatibilityConstruction{scopeSlot: slot},
	)
	if err != nil {
		t.Fatal(err)
	}
	team, err := slot.OpenTeamExecution(
		context.Background(), "team-return", "execution-return",
	)
	if err != nil {
		t.Fatal(err)
	}
	profile, instance := productScopeRuntimeFixture(t)
	delegate := &productScopedExecutorFixture{}
	executor := &productMissionScopedExecutor{delegate: delegate, team: team}
	if _, err := executor.Execute(context.Background(), supervisor.ExecuteInput{
		Profile: profile, Instance: instance,
		Generation: work.RunGenerationInput{
			RunID: "run-return", AgentInstanceID: "agent-return", ClaimGeneration: 1,
		},
	}); err != nil {
		t.Fatal(err)
	}
	if delegate.context == nil ||
		!errors.Is(context.Cause(delegate.context), composition.ErrScopeClosed) {
		t.Fatalf("context cause=%v", context.Cause(delegate.context))
	}
	if err := team.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := facade.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCOMP2DAttemptAdvancesSequentialToolTurns(t *testing.T) {
	recorder := &compositionTestRecorder{}
	slot := &productCapabilityScopeSlot{}
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}),
		"incident-comp2d-turn-advance", recorder,
		productCompatibilityConstruction{scopeSlot: slot},
	)
	if err != nil {
		t.Fatal(err)
	}
	team, err := slot.OpenTeamExecution(
		context.Background(), "team-turn", "execution-turn",
	)
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := team.OpenAgentAttempt(
		context.Background(), productAgentAttemptScopeRequest{
			AgentID: "agent-turn", AttemptID: "run-turn", TurnID: "run-turn:turn-1",
			TurnGeneration: 1, ExecutionBindingDigest: strings.Repeat("a", 64),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := attempt.AdvanceTurn(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if err := attempt.AdvanceTurn(context.Background(), 1); err == nil {
		t.Fatal("duplicate tool sequence advanced Turn")
	}
	if err := attempt.AdvanceTurn(context.Background(), 2); err != nil {
		t.Fatal(err)
	}
	if got := compositionRecorderScopeStageCount(
		recorder, composition.ScopeTurn, composition.StageScopeOpen,
	); got != 3 {
		t.Fatalf("turn opens=%d", got)
	}
	if got := compositionRecorderScopeStageCount(
		recorder, composition.ScopeTurn, composition.StageScopeClose,
	); got != 2 {
		t.Fatalf("turn closes before Attempt close=%d", got)
	}
	if err := attempt.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := team.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := facade.Close(); err != nil {
		t.Fatal(err)
	}
}

type productAttemptTurnControllerFixture struct{ sequences []int64 }

func (fixture *productAttemptTurnControllerFixture) AdvanceTurn(
	_ context.Context,
	sequence int64,
) error {
	fixture.sequences = append(fixture.sequences, sequence)
	return nil
}

func TestCOMP2DToolResultAdvancesOnlyResolvedTurn(t *testing.T) {
	controller := &productAttemptTurnControllerFixture{}
	ctx, err := bindProductAttemptTurnScope(context.Background(), controller)
	if err != nil {
		t.Fatal(err)
	}
	if err := advanceProductAttemptTurnAfterToolResult(
		ctx, 1, piadapter.ToolCallResult{Verdict: permissions.VerdictAsk},
	); err != nil {
		t.Fatal(err)
	}
	if len(controller.sequences) != 0 {
		t.Fatalf("ask advanced turns=%v", controller.sequences)
	}
	if err := advanceProductAttemptTurnAfterToolResult(
		ctx, 1, piadapter.ToolCallResult{
			Verdict: permissions.VerdictAllow, ExecutionID: "execution-1",
		},
	); err != nil {
		t.Fatal(err)
	}
	if len(controller.sequences) != 1 || controller.sequences[0] != 1 {
		t.Fatalf("resolved sequences=%v", controller.sequences)
	}
}

type productScopeVaultReader struct {
	mu       sync.Mutex
	secrets  map[credentialvault.CredentialIdentity][]byte
	returned map[credentialvault.CredentialIdentity][]byte
}

func (reader *productScopeVaultReader) ReadCredential(
	_ context.Context,
	identity credentialvault.CredentialIdentity,
) ([]byte, error) {
	reader.mu.Lock()
	defer reader.mu.Unlock()
	secret := append([]byte(nil), reader.secrets[identity]...)
	reader.returned[identity] = secret
	return secret, nil
}

func (reader *productScopeVaultReader) zeroized(
	identity credentialvault.CredentialIdentity,
) bool {
	reader.mu.Lock()
	defer reader.mu.Unlock()
	value := reader.returned[identity]
	if len(value) == 0 {
		return false
	}
	for _, current := range value {
		if current != 0 {
			return false
		}
	}
	return true
}

func TestCOMP2DCredentialRevokeIsolatesAgentAndScopeClosesPeerLease(t *testing.T) {
	slot := &productCapabilityScopeSlot{}
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}),
		"incident-comp2d-credential-isolation", &compositionTestRecorder{},
		productCompatibilityConstruction{scopeSlot: slot},
	)
	if err != nil {
		t.Fatal(err)
	}
	team, err := slot.OpenTeamExecution(
		context.Background(), "team-credentials", "execution-credentials",
	)
	if err != nil {
		t.Fatal(err)
	}
	bindings := make([]loomruntime.FrozenExecutionBinding, 0, 2)
	attempts := make([]productAgentAttemptScopeLease, 0, 2)
	for index, value := range []struct {
		provider  string
		account   string
		model     string
		reference string
	}{
		{"deepseek", "deepseek.primary", "deepseek-chat", "credential-ref-deepseek-scope"},
		{"minimax", "minimax.primary", "MiniMax-M2.1", "credential-ref-minimax-scope"},
	} {
		profile, profileErr := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
			ID: "scope-profile-" + value.provider, AdapterType: "loom-native",
			ProviderID: value.provider, ProviderAccountID: value.account,
			ModelID: value.model, AuthMode: loomruntime.AuthBrokered,
			EndpointFingerprint: strings.Repeat(string(rune('1'+index)), 64),
			CredentialReference: value.reference, CredentialRevision: int64(index + 3),
			Timeout: time.Minute,
		})
		if profileErr != nil {
			t.Fatal(profileErr)
		}
		instance, instanceErr := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
			ID: "runtime.scope." + value.provider, DeviceID: "device.scope",
			AdapterType: "loom-native", DisplayName: "Scope Runtime " + value.provider,
			Status: loomruntime.RuntimeOnline, Capacity: 2,
		})
		if instanceErr != nil {
			t.Fatal(instanceErr)
		}
		binding, bindingErr := loomruntime.FreezeExecutionBinding(profile, instance)
		if bindingErr != nil {
			t.Fatal(bindingErr)
		}
		attempt, openErr := team.OpenAgentAttempt(
			context.Background(), productAgentAttemptScopeRequest{
				AgentID: "agent-" + value.provider, AttemptID: "run-" + value.provider,
				TurnID: "run-" + value.provider + ":turn-1", TurnGeneration: 1,
				ExecutionBindingDigest: binding.BindingDigest,
			},
		)
		if openErr != nil {
			t.Fatal(openErr)
		}
		bindings = append(bindings, binding)
		attempts = append(attempts, attempt)
	}
	identities := make([]credentialvault.CredentialIdentity, len(bindings))
	reader := &productScopeVaultReader{
		secrets:  make(map[credentialvault.CredentialIdentity][]byte),
		returned: make(map[credentialvault.CredentialIdentity][]byte),
	}
	for index, binding := range bindings {
		identities[index] = credentialvault.CredentialIdentity{
			ProviderID: binding.ProviderID, ProviderAccountID: binding.ProviderAccountID,
			CredentialReference: binding.CredentialReference,
			CredentialRevision:  binding.CredentialRevision,
		}
		reader.secrets[identities[index]] = []byte("secret-" + binding.ProviderID)
	}
	manager, err := credentialvault.NewCredentialLeaseManager(reader)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	leasing, err := newProductVaultCredentialLeaseAccess(manager)
	if err != nil {
		t.Fatal(err)
	}
	started := []chan struct{}{make(chan struct{}), make(chan struct{})}
	results := []chan error{make(chan error, 1), make(chan error, 1)}
	causes := make([]error, 2)
	for index, binding := range bindings {
		access, accessErr := newProductAgentCredentialAccess(
			&productAgentCredentialSourceFixture{record: projection.ProviderCredentialRecord{
				ProviderID: binding.ProviderID, ProviderAccountID: binding.ProviderAccountID,
				CredentialReference: binding.CredentialReference,
				Revision:            binding.CredentialRevision,
				Status:              string(credentials.CredentialVerified),
			}},
			leasing,
		)
		if accessErr != nil {
			t.Fatal(accessErr)
		}
		go func(index int, access *productAgentCredentialAccess) {
			results[index] <- access.UseCredential(
				attempts[index].ExecutionContext(), bindings[index],
				func(leaseContext context.Context, _ []byte) error {
					close(started[index])
					<-leaseContext.Done()
					causes[index] = context.Cause(leaseContext)
					return leaseContext.Err()
				},
			)
		}(index, access)
	}
	for _, signal := range started {
		select {
		case <-signal:
		case <-time.After(time.Second):
			t.Fatal("credential lease did not start")
		}
	}
	if revoked := manager.Revoke(identities[0]); revoked != 1 {
		t.Fatalf("revoked leases=%d", revoked)
	}
	if err := <-results[0]; !errors.Is(err, context.Canceled) {
		t.Fatalf("revoked Agent result=%v", err)
	}
	select {
	case err := <-results[1]:
		t.Fatalf("peer Agent was contaminated: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	if !reader.zeroized(identities[0]) || reader.zeroized(identities[1]) {
		t.Fatalf("zeroization after revoke: first=%t peer=%t",
			reader.zeroized(identities[0]), reader.zeroized(identities[1]))
	}
	if err := team.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := <-results[1]; !errors.Is(err, context.Canceled) {
		t.Fatalf("peer scope-close result=%v", err)
	}
	if !errors.Is(causes[0], context.Canceled) ||
		!errors.Is(causes[1], composition.ErrScopeClosed) ||
		!reader.zeroized(identities[1]) {
		t.Fatalf("causes=%v zeroized=%t", causes, reader.zeroized(identities[1]))
	}
	if err := facade.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCOMP2DScopedMissionExecutorFailsClosedBeforeDelegate(t *testing.T) {
	profile, instance := productScopeRuntimeFixture(t)
	want := errors.New("scope unavailable")
	scope := &productTeamScopeFixture{err: want}
	delegate := &productScopedExecutorFixture{}
	executor := &productMissionScopedExecutor{delegate: delegate, team: scope}
	_, err := executor.Execute(context.Background(), supervisor.ExecuteInput{
		Profile: profile, Instance: instance,
		Generation: work.RunGenerationInput{
			RunID: "run-agent", AgentInstanceID: "agent-1", ClaimGeneration: 1,
		},
	})
	if !errors.Is(err, want) || delegate.executed {
		t.Fatalf("err=%v executed=%t", err, delegate.executed)
	}
}

func productScopeRuntimeFixture(
	t *testing.T,
) (loomruntime.RuntimeProfile, loomruntime.RuntimeInstance) {
	t.Helper()
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID: "scope-profile", AdapterType: "loom-native", ProviderID: "deepseek",
		ProviderAccountID: "deepseek.primary", ModelID: "deepseek-chat",
		AuthMode: loomruntime.AuthBrokered, EndpointFingerprint: strings.Repeat("1", 64),
		CredentialReference: "credential-ref-scope", CredentialRevision: 3,
		Timeout: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID: "runtime.scope", DeviceID: "device.scope", AdapterType: "loom-native",
		DisplayName: "Scope Runtime", Status: loomruntime.RuntimeOnline, Capacity: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	return profile, instance
}

func TestCOMP2DConversationScopeIsReusedAcrossAttemptsAndRevoked(t *testing.T) {
	recorder := &compositionTestRecorder{}
	slot := &productCapabilityScopeSlot{}
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}),
		"incident-comp2d-conversation-reuse", recorder,
		productCompatibilityConstruction{scopeSlot: slot},
	)
	if err != nil {
		t.Fatal(err)
	}
	for index, attemptID := range []string{"attempt-1", "attempt-2"} {
		lease, openErr := slot.OpenConversationAttempt(
			context.Background(), api.LocalProductConversationScopeRequest{
				ConversationID:         "conversation-reuse",
				TeamID:                 "conversation:conversation-reuse",
				AgentID:                "conversation-agent:loom",
				AttemptID:              attemptID,
				TurnID:                 attemptID + ":turn-1",
				TurnGeneration:         1,
				ExecutionBindingDigest: strings.Repeat(string(rune('b'+index)), 64),
			},
		)
		if openErr != nil {
			t.Fatal(openErr)
		}
		if err := lease.Close(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if got := compositionRecorderScopeStageCount(
		recorder, composition.ScopeConversation, composition.StageScopeOpen,
	); got != 1 {
		t.Fatalf("conversation opens=%d", got)
	}
	if got := compositionRecorderScopeStageCount(
		recorder, composition.ScopeAttempt, composition.StageScopeOpen,
	); got != 2 {
		t.Fatalf("attempt opens=%d", got)
	}
	if err := facade.Close(); err != nil {
		t.Fatal(err)
	}
	if slot.Ready() {
		t.Fatal("closed Product scope retained scope admission")
	}
	if _, err := slot.OpenConversationAttempt(
		context.Background(), api.LocalProductConversationScopeRequest{},
	); err == nil {
		t.Fatal("closed Product scope admitted an Attempt")
	}
}

func TestCOMP2DProductionWiresConversationScopeSlotBeforeAdmission(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "product_daemon.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	foundConfig := false
	foundTeamConfig := false
	foundComposition := false
	foundScopedExecutor := false
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "newProductDaemonRunnerWithPreparedDecisions" {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			switch current := node.(type) {
			case *ast.AssignStmt:
				for index, left := range current.Lhs {
					selector, ok := left.(*ast.SelectorExpr)
					if ok && selector.Sel.Name == "ConversationScopes" && index < len(current.Rhs) &&
						identifierNamed(current.Rhs[index], "scopeRouteSlot") {
						foundConfig = true
					}
					if ok && selector.Sel.Name == "TeamScopes" && index < len(current.Rhs) &&
						identifierNamed(current.Rhs[index], "scopeRouteSlot") {
						foundTeamConfig = true
					}
				}
			case *ast.KeyValueExpr:
				key, ok := current.Key.(*ast.Ident)
				if ok && key.Name == "scopeSlot" && identifierNamed(current.Value, "scopeRouteSlot") {
					foundComposition = true
				}
			}
			return true
		})
	}
	parsed, err = parser.ParseFile(token.NewFileSet(), "product_daemon.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "Run" || function.Recv == nil {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			literal, ok := node.(*ast.CompositeLit)
			if !ok {
				return true
			}
			identifier, ok := literal.Type.(*ast.Ident)
			if ok && identifier.Name == "productMissionScopedExecutor" {
				foundScopedExecutor = true
			}
			return true
		})
	}
	if !foundConfig || !foundTeamConfig || !foundComposition || !foundScopedExecutor {
		t.Fatalf(
			"conversation=%t team=%t composition=%t executor=%t",
			foundConfig, foundTeamConfig, foundComposition, foundScopedExecutor,
		)
	}
}

func compositionRecorderHasScopeStage(
	recorder *compositionTestRecorder,
	kind composition.ScopeKind,
	stage composition.DiagnosticStage,
) bool {
	for _, record := range recorder.snapshot() {
		if record.ScopeKind == kind && record.Stage == stage {
			return true
		}
	}
	return false
}

func compositionRecorderScopeStageCount(
	recorder *compositionTestRecorder,
	kind composition.ScopeKind,
	stage composition.DiagnosticStage,
) int {
	count := 0
	for _, record := range recorder.snapshot() {
		if record.ScopeKind == kind && record.Stage == stage {
			count++
		}
	}
	return count
}
