package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"loom-pi-rebuild/internal/assets"
	"loom-pi-rebuild/internal/authorization"
	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/credentials"
	credentialvault "loom-pi-rebuild/internal/credentials/vault"
	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/rules"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/teams"
	"loom-pi-rebuild/internal/verification"
	"loom-pi-rebuild/internal/work"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"

	_ "modernc.org/sqlite"
)

type teamCanaryClock struct {
	mu   sync.Mutex
	now  time.Time
	step time.Duration
}

func TestVerifierPrivacyInstructionDistinguishesMissionMarkersFromCredentials(t *testing.T) {
	instruction := verifierPrivacyInstruction(
		"Return the exact non-secret acceptance marker LOOM-MIXED-TEAM-OK.",
	)
	if strings.Contains(instruction, "secrets, tokens, or credentials") ||
		!strings.Contains(instruction, "API keys") ||
		!strings.Contains(instruction, "non-secret acceptance markers") {
		t.Fatalf("ambiguous verifier privacy instruction: %q", instruction)
	}
}

type p3aCleanupRecordingMaterializer struct {
	values []TeamAssetMaterialization
}

type recordingTeamWorkspacePublisher struct {
	publications []TeamWorkspacePublication
	err          error
}

func (publisher *recordingTeamWorkspacePublisher) PublishAcceptedWorkspace(
	_ context.Context,
	publication TeamWorkspacePublication,
) error {
	publisher.publications = append(publisher.publications, publication)
	return publisher.err
}

type restartAssetMaterializer struct {
	authority  *assets.Authority
	sourcePath string
	requests   []TeamAssetMaterializationRequest
}

func (materializer *restartAssetMaterializer) PrepareTeamAttemptMaterialization(
	ctx context.Context,
	request TeamAssetMaterializationRequest,
) (TeamAssetMaterialization, error) {
	materializer.requests = append(materializer.requests, request)
	manifestDigest := strings.Repeat("c", 64)
	rootDigest := strings.Repeat("d", 64)
	prepared, err := materializer.authority.PrepareMaterialization(ctx, assets.Command{
		OperationID:               fmt.Sprintf("materialize:%s:%d:%d", request.RunID, request.AttemptNumber, request.Generation),
		JourneyID:                 request.JourneyID,
		ExpectedViewVersion:       strings.Repeat("a", 64),
		TeamExecutionID:           request.TeamExecutionID,
		LogicalNodeID:             request.LogicalNodeID,
		RunID:                     request.RunID,
		AttemptNumber:             request.AttemptNumber,
		Generation:                request.Generation,
		RuntimeInstanceID:         request.Instance.ID,
		RuntimeIdentityDigest:     strings.Repeat("b", 64),
		Capability:                "loom.skill-materialization.pi.v1",
		Bindings:                  request.Bindings,
		AssetRevisionSetDigest:    request.RevisionSetDigest,
		ManifestArtifactDigest:    manifestDigest,
		MaterializationRootDigest: rootDigest,
	})
	if err != nil {
		return TeamAssetMaterialization{}, err
	}
	return TeamAssetMaterialization{
		SourcePath: materializer.sourcePath,
		RunID:      request.RunID, AttemptNumber: request.AttemptNumber,
		Generation: request.Generation, JourneyID: request.JourneyID,
		ManifestDigest: manifestDigest, RootDigest: rootDigest,
		Authoritative: prepared.AlreadyCommitted(),
		AttemptLineage: work.TeamAttemptMaterialization{
			LogicalNodeID: request.LogicalNodeID, AttemptNumber: request.AttemptNumber,
			AssetRevisionBindings:         append([]assets.ExactAssetRevisionBinding(nil), request.Bindings...),
			AssetRevisionSetDigest:        request.RevisionSetDigest,
			MaterializationManifestDigest: manifestDigest,
			MaterializationRootDigest:     rootDigest,
			Prepared:                      prepared,
		},
	}, nil
}

func (*restartAssetMaterializer) CleanupTeamAttemptMaterialization(
	context.Context,
	TeamAssetMaterialization,
) error {
	return nil
}

func (*p3aCleanupRecordingMaterializer) PrepareTeamAttemptMaterialization(
	context.Context,
	TeamAssetMaterializationRequest,
) (TeamAssetMaterialization, error) {
	return TeamAssetMaterialization{}, ErrInvalidTeamCoordinator
}

func (materializer *p3aCleanupRecordingMaterializer) CleanupTeamAttemptMaterialization(
	_ context.Context,
	value TeamAssetMaterialization,
) error {
	materializer.values = append(materializer.values, value)
	return nil
}

func TestP3ATerminalMaterializationCleanupIsExplicitlyAuthoritative(t *testing.T) {
	recorder := &p3aCleanupRecordingMaterializer{}
	coordinator := &TeamCoordinator{assetMaterializer: recorder}
	value := TeamAssetMaterialization{
		RunID: "run-1", AttemptNumber: 1, Generation: 1,
		JourneyID:      "123e4567-e89b-42d3-a456-426614174000",
		ManifestDigest: strings.Repeat("a", 64), RootDigest: strings.Repeat("b", 64),
		Authoritative: true,
	}
	if err := coordinator.cleanupCommittedTeamAssetMaterializations(
		context.Background(), []TeamAssetMaterialization{value},
	); err != nil {
		t.Fatal(err)
	}
	if len(recorder.values) != 1 || !recorder.values[0].Authoritative ||
		recorder.values[0].RunID != value.RunID {
		t.Fatalf("cleanup values = %#v", recorder.values)
	}
}

func TestP3ADispatchFailureCleanupPreservesAlreadyAuthoritativeRoot(t *testing.T) {
	recorder := &p3aCleanupRecordingMaterializer{}
	coordinator := &TeamCoordinator{assetMaterializer: recorder}
	coordinator.cleanupTeamAssetMaterializations(
		context.Background(),
		[]TeamAssetMaterialization{
			{RunID: "run-authoritative", Authoritative: true},
			{RunID: "run-uncommitted", Authoritative: false},
		},
	)
	if len(recorder.values) != 1 || recorder.values[0].RunID != "run-uncommitted" {
		t.Fatalf("dispatch cleanup touched authoritative root: %#v", recorder.values)
	}
}

func (clock *teamCanaryClock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	current := clock.now
	clock.now = clock.now.Add(clock.step)
	return current
}

func (clock *teamCanaryClock) Set(now time.Time) {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	clock.now = now
}

func (clock *teamCanaryClock) Current() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return clock.now
}

func (clock *teamCanaryClock) SetStep(step time.Duration) {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	clock.step = step
}

type teamCanaryBarrier struct {
	active    atomic.Int32
	maxActive atomic.Int32
	subCount  atomic.Int32
	release   chan struct{}
	once      sync.Once
}

type teamCanaryAdapter struct {
	barrier         *teamCanaryBarrier
	subagent        bool
	adapterType     string
	terminalStatus  string
	terminalReason  string
	runtimeID       string
	calls           *atomic.Int32
	omitOutput      bool
	outputDelta     string
	accounting      *work.RunAccounting
	requestObserver func(supervisor.AdapterRequest) error
	contextObserver func(context.Context) error
	credentialUse   func(
		context.Context,
		loomruntime.FrozenExecutionBinding,
		func(context.Context, []byte) error,
	) error
}

type teamCanaryCredentialReader struct{}

func (teamCanaryCredentialReader) ReadCredential(
	_ context.Context,
	_ credentialvault.CredentialIdentity,
) ([]byte, error) {
	return []byte("bounded-canary-secret"), nil
}

func (adapter *teamCanaryAdapter) AdapterType() string {
	if adapter.adapterType == "" {
		return "pi"
	}
	return adapter.adapterType
}
func (adapter *teamCanaryAdapter) RuntimeInstanceID() string {
	return adapter.runtimeID
}

func (adapter *teamCanaryAdapter) Execute(
	ctx context.Context,
	request supervisor.AdapterRequest,
) (supervisor.AdapterResult, error) {
	if adapter.contextObserver != nil {
		if err := adapter.contextObserver(ctx); err != nil {
			return supervisor.AdapterResult{}, err
		}
	}
	if adapter.requestObserver != nil {
		if err := adapter.requestObserver(request); err != nil {
			return supervisor.AdapterResult{}, err
		}
	}
	if adapter.calls != nil {
		adapter.calls.Add(1)
	}
	active := adapter.barrier.active.Add(1)
	for {
		maximum := adapter.barrier.maxActive.Load()
		if active <= maximum ||
			adapter.barrier.maxActive.CompareAndSwap(maximum, active) {
			break
		}
	}
	defer adapter.barrier.active.Add(-1)
	if adapter.credentialUse != nil {
		err := adapter.credentialUse(
			ctx,
			request.ExecutionBinding,
			func(context.Context, []byte) error { return nil },
		)
		if err != nil {
			return teamCanaryTerminalResult(
				ctx, request, "failed", "credential_unavailable", nil,
			)
		}
	}
	if adapter.subagent {
		if adapter.barrier.subCount.Add(1) == 3 {
			adapter.barrier.once.Do(func() { close(adapter.barrier.release) })
		}
		select {
		case <-adapter.barrier.release:
		case <-ctx.Done():
			return supervisor.AdapterResult{}, ctx.Err()
		}
	}
	terminalStatus := adapter.terminalStatus
	if terminalStatus == "" {
		terminalStatus = "succeeded"
	}
	reason := ""
	if terminalStatus != "succeeded" {
		reason = adapter.terminalReason
		if reason == "" {
			reason = "controlled_failure"
		}
	}
	frames := []bridgev1.Frame{
		teamCanaryInboundFrame(
			request,
			2,
			bridgev1.MessageAck,
			mustTeamCanaryJSON(map[string]string{
				"message_id": request.Dispatch.MessageID(),
			}),
		),
	}
	if !adapter.omitOutput {
		outputDelta := adapter.outputDelta
		if outputDelta == "" {
			outputDelta = "authorized-" + request.Binding.RunID
		}
		frames = append(frames, teamCanaryInboundFrame(
			request,
			3,
			bridgev1.MessageEvent,
			mustTeamCanaryJSON(map[string]string{
				"delta": outputDelta,
			}),
		))
	}
	frames = append(frames, teamCanaryInboundFrame(
		request,
		4,
		bridgev1.MessageResult,
		mustTeamCanaryJSON(map[string]string{
			"status": terminalStatus,
			"reason": reason,
		}),
	))
	for _, frame := range frames {
		if err := request.FrameSink.AcceptFrame(ctx, frame); err != nil {
			return supervisor.AdapterResult{}, err
		}
	}
	return supervisor.NewAdapterResult(supervisor.AdapterResultInput{
		InboundFrames:        frames,
		ExitCode:             0,
		DispatchAcknowledged: true,
		ResultAcknowledged:   true,
		Accounting:           adapter.accounting,
	})
}

func teamCanaryTerminalResult(
	ctx context.Context,
	request supervisor.AdapterRequest,
	status string,
	reason string,
	accounting *work.RunAccounting,
) (supervisor.AdapterResult, error) {
	frames := []bridgev1.Frame{
		teamCanaryInboundFrame(
			request,
			2,
			bridgev1.MessageAck,
			mustTeamCanaryJSON(map[string]string{
				"message_id": request.Dispatch.MessageID(),
			}),
		),
		teamCanaryInboundFrame(
			request,
			3,
			bridgev1.MessageResult,
			mustTeamCanaryJSON(map[string]string{
				"status": status,
				"reason": reason,
			}),
		),
	}
	for _, frame := range frames {
		if err := request.FrameSink.AcceptFrame(ctx, frame); err != nil {
			return supervisor.AdapterResult{}, err
		}
	}
	return supervisor.NewAdapterResult(supervisor.AdapterResultInput{
		InboundFrames:        frames,
		ExitCode:             0,
		DispatchAcknowledged: true,
		ResultAcknowledged:   true,
		Accounting:           accounting,
	})
}

type fourProviderTeamCanaryFixture struct {
	coordinator *TeamCoordinator
	request     TeamExecutionRequest
	adapters    map[string]*teamCanaryAdapter
	store       *journal.Store
	readModel   *projection.Projection
}

type teamAggregationCapsuleCapture struct {
	mu       sync.Mutex
	capsules map[string]contextcapsule.RoleContextCapsule
}

type staleOnceTeamCapsuleStore struct {
	store   *journal.Store
	now     time.Time
	once    sync.Once
	mu      sync.Mutex
	digests []string
}

func (store *staleOnceTeamCapsuleStore) PutRoleContextCapsule(
	ctx context.Context,
	capsule contextcapsule.RoleContextCapsule,
	_ []byte,
) error {
	store.mu.Lock()
	store.digests = append(store.digests, capsule.AuthorityRecord().CapsuleDigest)
	store.mu.Unlock()
	var appendErr error
	store.once.Do(func() {
		payload, err := json.Marshal(map[string]any{
			"discovery_digest": strings.Repeat("9", 64),
			"source_probe_id":  "probe-stale-context-retry",
			"instance": map[string]any{
				"id": "runtime-recovery", "device_id": "device-1",
				"adapter_type": "pi", "display_name": "stale retry noise",
				"executable_version": "1.0.0", "status": "online",
				"observed_capabilities": []string{"models"}, "capacity": 1,
			},
			"model_ids": []string{},
		})
		if err != nil {
			appendErr = err
			return
		}
		_, appendErr = store.store.Append(ctx, journal.Event{
			ID:       "runtime-stale-context-retry",
			StreamID: "runtime_instance:runtime-recovery", Seq: 2,
			IdempotencyKey: "runtime-stale-context-retry",
			Type:           "RuntimeInstanceDiscovered", SchemaVersion: 1,
			EmittedAt: store.now, CorrelationID: "33333333-3333-4333-8333-333333333333",
			PayloadJSON: payload,
		})
	})
	return appendErr
}

func (store *staleOnceTeamCapsuleStore) capturedDigests() []string {
	store.mu.Lock()
	defer store.mu.Unlock()
	return append([]string(nil), store.digests...)
}

type teamAggregationCapacityCounter struct {
	mu         sync.Mutex
	identifier string
	version    string
	inputs     [][]byte
}

func (counter *teamAggregationCapacityCounter) ID() string {
	return counter.identifier
}

func (counter *teamAggregationCapacityCounter) Version() string {
	return counter.version
}

func (counter *teamAggregationCapacityCounter) CountTokens(content []byte) (int, error) {
	counter.mu.Lock()
	defer counter.mu.Unlock()
	counter.inputs = append(counter.inputs, append([]byte(nil), content...))
	return missionContextTokenCount(content), nil
}

func (counter *teamAggregationCapacityCounter) inputCount() int {
	counter.mu.Lock()
	defer counter.mu.Unlock()
	return len(counter.inputs)
}

func testTeamAggregationCapacityAuthority() contextcapsule.CapacityAuthority {
	return contextcapsule.CapacityAuthority{
		SchemaVersion:             contextcapsule.CapacitySchemaVersion,
		Status:                    contextcapsule.CapacityExact,
		ContextWindowTokens:       16_384,
		ReservedOutputTokens:      1_024,
		AdapterToolOverheadTokens: 512,
		TokenCounterID:            "counter:team-aggregation-test",
		TokenCounterVersion:       "v1",
	}
}

type teamGovernedTestReportSourceCapture struct {
	mu      sync.Mutex
	reports map[string][]verification.GovernedTestReport
	queries []work.AttemptReportQuery
}

func (capture *teamGovernedTestReportSourceCapture) GovernedTestReportsForAttempt(
	_ context.Context,
	query work.AttemptReportQuery,
) ([]verification.GovernedTestReport, error) {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	capture.queries = append(capture.queries, query)
	return append(
		[]verification.GovernedTestReport(nil),
		capture.reports[query.Authority.AgentInstanceID]...,
	), nil
}

func (capture *teamAggregationCapsuleCapture) PutRoleContextCapsule(
	_ context.Context,
	capsule contextcapsule.RoleContextCapsule,
	_ []byte,
) error {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	if capture.capsules == nil {
		capture.capsules = make(map[string]contextcapsule.RoleContextCapsule)
	}
	capture.capsules[capsule.Target().RoleID] = capsule
	return nil
}

func TestParallelRouteSiblingsDispatchIndependentAttemptsThenAggregation(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 13, 8, 0, 0, 0, time.UTC)
	db := openTeamCanaryDB(t)
	store := journal.NewStore(db)
	for _, runtimeID := range []string{"runtime-route-a", "runtime-route-b", "runtime-aggregate"} {
		seedTeamCanaryRuntime(t, store, runtimeID, 1, now)
	}
	clock := &teamCanaryClock{now: now}
	workRandom := make([]byte, 2048)
	for index := range workRandom {
		workRandom[index] = 0x71 + byte(index/16)
	}
	workAuthority, err := work.NewAuthority(
		store, clock.Now, bytes.NewReader(workRandom),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := workAuthority.InitializeRunIdentityIndex(ctx); err != nil {
		t.Fatal(err)
	}
	grantRandom := make([]byte, 48*16)
	for index := range grantRandom {
		grantRandom[index] = 0x72 + byte(index/48)
	}
	grantAuthority, err := authorization.NewAuthority(
		store, workAuthority, clock.Now,
		bytes.NewReader(grantRandom),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := grantAuthority.InitializeGrantIdentityIndex(ctx); err != nil {
		t.Fatal(err)
	}
	readModel := projection.New(db)
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	evidenceRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := evidence.NewStore(filepath.Join(evidenceRoot, "evidence"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = artifacts.Close() })
	coordinator, err := NewTeamCoordinator(
		workAuthority, grantAuthority, readModel, artifacts,
	)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-parallel-aggregation",
		Nodes: []teams.ExecutionNodeInput{
			{
				LogicalNodeID: "route-a", Title: "Route A",
				AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-route-a",
				Role: teams.ExecutionRoleMain, Kind: teams.ExecutionNodeRouteSibling,
				RouteGroupID: "route-group-main", MaxAttempts: 1,
			},
			{
				LogicalNodeID: "route-b", Title: "Route B",
				AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-route-b",
				Role: teams.ExecutionRoleMain, Kind: teams.ExecutionNodeRouteSibling,
				RouteGroupID: "route-group-main", MaxAttempts: 1,
			},
			{
				LogicalNodeID: "main", Title: "Aggregate routes",
				AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-aggregate",
				Role: teams.ExecutionRoleMain, Kind: teams.ExecutionNodeAggregation,
				RouteGroupID: "route-group-main", DependsOn: []string{"route-a", "route-b"},
				MaxAttempts: 1,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var aggregationPayload []byte
	capacityAuthority := testTeamAggregationCapacityAuthority()
	capacityCounter := &teamAggregationCapacityCounter{
		identifier: capacityAuthority.TokenCounterID,
		version:    capacityAuthority.TokenCounterVersion,
	}
	nodes := make([]TeamNodeExecution, 0, 3)
	for _, node := range plan.Nodes() {
		accountingTokens := map[string]int64{"route-a": 11, "route-b": 22, "main": 33}[node.LogicalNodeID()]
		adapter := &teamCanaryAdapter{
			barrier:   &teamCanaryBarrier{release: make(chan struct{})},
			runtimeID: node.RuntimeInstanceID(),
			accounting: &work.RunAccounting{
				UsageObserved: true, InputTokens: accountingTokens, OutputTokens: 1,
				TotalTokens: accountingTokens + 1, CostObserved: true,
				CostMicrounits: accountingTokens * 100, CostCurrency: "USD",
				CostSource: work.CostSourceProviderReported,
			},
		}
		if node.Kind() == teams.ExecutionNodeAggregation {
			adapter.requestObserver = func(request supervisor.AdapterRequest) error {
				aggregationPayload = request.Dispatch.Payload()
				return nil
			}
		}
		execution := teamCanaryNodeExecution(
			t, plan, node.LogicalNodeID(), 1, node.AgentInstanceID(),
			node.RuntimeInstanceID(), now,
			newTeamCanarySupervisor(t, workAuthority, grantAuthority, adapter),
		)
		if node.Kind() == teams.ExecutionNodeAggregation {
			execution.Aggregation = &TeamAggregationExecution{
				MaxSourceArtifactBytes: maxTeamAggregationSourceBytes,
				CapacityAuthority:      capacityAuthority,
				TokenCounter:           capacityCounter,
			}
			goalContent := []byte("Execute the controlled canary.")
			base, capsuleErr := contextcapsule.BuildRoleContextCapsuleWithCapacity(
				contextcapsule.Target{
					ConversationID: "team-conversation:" + plan.TeamInstanceID(),
					TeamID:         plan.TeamInstanceID(), AgentID: node.AgentInstanceID(),
					RoleID: node.LogicalNodeID(), ProviderID: execution.Profile.ProviderID,
					ProviderAccountID: execution.Profile.ProviderAccountID,
					ModelID:           execution.Profile.ModelID, AuthMode: string(execution.Profile.AuthMode),
					ContextAdapterID:   "context:pi:v1",
					DisclosurePolicyID: "policy.test", DisclosurePolicyVersion: 1,
					TokenBudget: 2048,
				},
				[]contextcapsule.ItemInput{
					{
						ItemID: "goal-1", Kind: contextcapsule.KindConversationGoal,
						Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeTeamShared,
						Priority:   contextcapsule.PrioritySystem,
						TokenCount: missionContextTokenCount(goalContent), Required: true,
						Content: goalContent, SourceType: contextcapsule.SourceAuthority,
						SourceRef: "team-plan:" + plan.Digest(),
					},
					{
						ItemID: "fixed-policy-omission", Kind: contextcapsule.KindPriorModelOutput,
						Trust: contextcapsule.TrustUntrusted, Scope: contextcapsule.ScopeRoleRestricted,
						Priority: contextcapsule.PriorityHistory, TokenCount: 4,
						Content: []byte("API_KEY=fixed-private"), SourceType: contextcapsule.SourceModelOutput,
						SourceRef: "attempt-output:" + strings.Repeat("9", 64), AllowedRoleID: "main",
						PolicyFiltered: true,
					},
					{
						ItemID: "fixed-scope-omission", Kind: contextcapsule.KindObservedExecutionState,
						Trust: contextcapsule.TrustObserved, Scope: contextcapsule.ScopeAgentPrivate,
						Priority: contextcapsule.PriorityWorkspace, TokenCount: 3,
						Content: []byte("other agent state"), SourceType: contextcapsule.SourceObservation,
						SourceRef: "acceptance:" + strings.Repeat("8", 64), AllowedAgentID: "agent-other",
					},
				},
				capacityAuthority,
				capacityCounter,
			)
			if capsuleErr != nil {
				t.Fatal(capsuleErr)
			}
			execution.ContextCapsule = base
			execution.ContextCapacityAuthority = capacityAuthority
			execution.ContextTokenCounter = capacityCounter
			payload, renderErr := contextcapsule.RenderDispatchPayload(base)
			if renderErr != nil {
				t.Fatal(renderErr)
			}
			execution.Dispatch = replaceTeamDispatchPayload(t, execution.Dispatch, payload)
		}
		nodes = append(nodes, execution)
	}
	capture := &teamAggregationCapsuleCapture{}
	request := TeamExecutionRequest{
		Plan: plan, Nodes: nodes, Semantics: testTeamNodeSemantics(t, plan, 0, ""),
		AuthoritativeTime: now, PrepareLeaseDuration: time.Minute,
		GrantLifetime:   time.Minute,
		CorrelationID:   "11111111-1111-4111-8111-111111111111",
		ContextCapsules: capture,
	}
	if _, err := validateTeamExecutionRequest(ctx, request); err != nil {
		t.Fatalf("validate parallel request: %v", err)
	}
	result, err := coordinator.Run(ctx, request)
	if err != nil {
		events, _ := store.ReadAll(ctx)
		types := make([]string, len(events))
		for index := range events {
			types[index] = events[index].Type
		}
		t.Fatalf("Run error=%v aggregation_payload=%d events=%v", err, len(aggregationPayload), types)
	}
	if result.Team().Status() != "succeeded" ||
		!reflect.DeepEqual(result.ExecutedNodeIDs(), []string{"main", "route-a", "route-b"}) {
		t.Fatalf("parallel result status=%q nodes=%v", result.Team().Status(), result.ExecutedNodeIDs())
	}
	if !bytes.Contains(aggregationPayload, []byte("authorized-team-run-")) ||
		!bytes.Contains(aggregationPayload, []byte("untrusted_model_output")) ||
		bytes.Contains(aggregationPayload, []byte("private_evidence")) {
		t.Fatalf("aggregation dispatch payload = %s", aggregationPayload)
	}
	capture.mu.Lock()
	aggregationCapsule := capture.capsules["main"]
	capture.mu.Unlock()
	if !aggregationCapsule.Valid() {
		t.Fatal("aggregation Capsule was not persisted")
	}
	capacityProjection, capacityAvailable := aggregationCapsule.CapacityProjection()
	admittedItemsBySource := make(map[contextcapsule.SourceType]int)
	for _, contribution := range capacityProjection.Contributions {
		admittedItemsBySource[contribution.SourceType] += contribution.AdmittedItemCount
	}
	if !capacityAvailable ||
		capacityProjection.AdmittedContributionTokens != aggregationCapsule.TokenCount() ||
		capacityProjection.TokenCounterID != capacityAuthority.TokenCounterID ||
		capacityProjection.TokenCounterVersion != capacityAuthority.TokenCounterVersion ||
		capacityCounter.inputCount() != 8 ||
		admittedItemsBySource[contextcapsule.SourceAuthority] != 3 ||
		admittedItemsBySource[contextcapsule.SourceObservation] != 2 ||
		admittedItemsBySource[contextcapsule.SourceModelOutput] != 2 {
		t.Fatalf(
			"aggregation capacity projection=%#v available=%t token_count=%d counter_inputs=%d admitted_items=%v",
			capacityProjection, capacityAvailable, aggregationCapsule.TokenCount(),
			capacityCounter.inputCount(), admittedItemsBySource,
		)
	}
	omissions := make(map[string]contextcapsule.OmissionReason)
	for _, omission := range aggregationCapsule.Omitted() {
		omissions[omission.ItemID] = omission.Reason
	}
	if omissions["fixed-policy-omission"] != contextcapsule.OmissionPolicyFiltered ||
		omissions["fixed-scope-omission"] != contextcapsule.OmissionAccessDenied {
		t.Fatalf("aggregation fixed omissions = %#v", aggregationCapsule.Omitted())
	}
	authorityCount, observationCount, outputCount := 0, 0, 0
	for _, item := range aggregationCapsule.Disclosed() {
		switch item.Kind {
		case contextcapsule.KindAggregationSource:
			authorityCount++
			if item.Trust != contextcapsule.TrustAuthoritative {
				t.Fatalf("aggregation authority trust = %q", item.Trust)
			}
		case contextcapsule.KindObservedExecutionState:
			observationCount++
			if item.Trust != contextcapsule.TrustObserved ||
				item.SourceType != contextcapsule.SourceObservation {
				t.Fatalf("aggregation observation = %#v", item)
			}
		case contextcapsule.KindPriorModelOutput:
			outputCount++
			if item.Trust != contextcapsule.TrustUntrusted ||
				item.SourceType != contextcapsule.SourceModelOutput {
				t.Fatalf("aggregation output = %#v", item)
			}
		}
	}
	if authorityCount != 2 || observationCount != 2 || outputCount != 2 {
		t.Fatalf(
			"aggregation items authority=%d observation=%d output=%d",
			authorityCount, observationCount, outputCount,
		)
	}
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	view := readModel.GlobalReadView()
	wantAccounting := map[string]int64{"route-a": 12, "route-b": 23, "main": 34}
	seenAccounts := make(map[string]struct{})
	for _, node := range result.Team().Nodes() {
		attempts := node.Attempts()
		if len(attempts) != 1 {
			t.Fatalf("%s Attempts = %#v", node.LogicalNodeID(), attempts)
		}
		run, found := view.Run(attempts[0].RunID())
		if !found || !run.AccountingAvailable || run.Accounting.TotalTokens != wantAccounting[node.LogicalNodeID()] ||
			run.ExecutionBinding.ProviderAccountID == "" {
			t.Fatalf("%s account-local accounting = %#v", node.LogicalNodeID(), run)
		}
		seenAccounts[run.ExecutionBinding.ProviderAccountID] = struct{}{}
	}
	if len(seenAccounts) != 3 {
		t.Fatalf("parallel route accounting merged Provider Accounts: %v", seenAccounts)
	}
	events, err := store.ReadAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if bytes.Contains(event.PayloadJSON, []byte("authorized-team-run-")) {
			t.Fatalf("model output leaked into Journal event %s", event.Type)
		}
	}
}

func TestAggregationCapsuleRejectsSourceAndPlanSubstitution(t *testing.T) {
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-aggregation-substitution",
		Nodes: []teams.ExecutionNodeInput{
			{
				LogicalNodeID: "route-a", Title: "Route A",
				AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-a",
				Role: teams.ExecutionRoleMain, Kind: teams.ExecutionNodeRouteSibling,
				RouteGroupID: "route-group-main", MaxAttempts: 1,
			},
			{
				LogicalNodeID: "route-b", Title: "Route B",
				AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-b",
				Role: teams.ExecutionRoleMain, Kind: teams.ExecutionNodeRouteSibling,
				RouteGroupID: "route-group-main", MaxAttempts: 1,
			},
			{
				LogicalNodeID: "main", Title: "Synthesis",
				AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-aggregate",
				Role: teams.ExecutionRoleMain, Kind: teams.ExecutionNodeAggregation,
				RouteGroupID: "route-group-main", DependsOn: []string{"route-a", "route-b"},
				MaxAttempts: 1,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	aggregator, found := missionContextPlanNode(plan, "main")
	if !found {
		t.Fatal("aggregation node not found")
	}
	capacityAuthority := testTeamAggregationCapacityAuthority()
	capacityCounter := &teamAggregationCapacityCounter{
		identifier: capacityAuthority.TokenCounterID,
		version:    capacityAuthority.TokenCounterVersion,
	}
	goalContent := []byte("Synthesize routes.")
	base, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(
		contextcapsule.Target{
			ConversationID: "team-conversation:" + plan.TeamInstanceID(),
			TeamID:         plan.TeamInstanceID(), AgentID: "agent-main", RoleID: "main",
			ProviderID: "openai", ProviderAccountID: "openai.aggregate",
			ModelID: "gpt-test", AuthMode: string(loomruntime.AuthBrokered),
			ContextAdapterID: "context:pi:v1", DisclosurePolicyID: "policy.test",
			DisclosurePolicyVersion: 1, TokenBudget: 256,
		},
		[]contextcapsule.ItemInput{{
			ItemID: "goal-1", Kind: contextcapsule.KindConversationGoal,
			Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeTeamShared,
			Priority:   contextcapsule.PrioritySystem,
			TokenCount: missionContextTokenCount(goalContent), Required: true,
			Content: goalContent, SourceType: contextcapsule.SourceAuthority,
			SourceRef: "team-plan:" + plan.Digest(),
		}},
		capacityAuthority,
		capacityCounter,
	)
	if err != nil {
		t.Fatal(err)
	}
	sources := []TeamAggregationSource{
		{
			LogicalNodeID: "route-a", AttemptNumber: 1, WorkItemID: "work-a", RunID: "run-a",
			ClaimID: "11111111-1111-4111-8111-111111111111", ClaimGeneration: 1,
			RuntimeInstanceID: "runtime-a", AgentInstanceID: "agent-main", EvidenceID: "evidence-a",
			EvidenceDigest: strings.Repeat("a", 64), OutputSummaryDigest: strings.Repeat("b", 64),
			TerminalStatus: "succeeded", OutputContractVersion: 1,
			OutputContractDigest:       strings.Repeat("1", 64),
			OutputClassification:       "valid_nonempty",
			OutputClassificationDigest: strings.Repeat("2", 64),
			AcceptanceDecisionKind:     "accepted", AcceptanceDecisionDigest: strings.Repeat("3", 64),
			AcceptanceDecisionTime: time.Date(2026, 8, 15, 1, 0, 0, 0, time.UTC),
			Content:                []byte(`{"schema_version":1,"events":[{"text":"route a"}]}`),
		},
		{
			LogicalNodeID: "route-b", AttemptNumber: 1, WorkItemID: "work-b", RunID: "run-b",
			ClaimID: "22222222-2222-4222-8222-222222222222", ClaimGeneration: 1,
			RuntimeInstanceID: "runtime-b", AgentInstanceID: "agent-main", EvidenceID: "evidence-b",
			EvidenceDigest: strings.Repeat("c", 64), OutputSummaryDigest: strings.Repeat("d", 64),
			TerminalStatus: "succeeded", OutputContractVersion: 1,
			OutputContractDigest:       strings.Repeat("4", 64),
			OutputClassification:       "valid_nonempty",
			OutputClassificationDigest: strings.Repeat("5", 64),
			AcceptanceDecisionKind:     "accepted", AcceptanceDecisionDigest: strings.Repeat("6", 64),
			AcceptanceDecisionTime: time.Date(2026, 8, 15, 1, 0, 0, 0, time.UTC),
			Content:                []byte(`{"schema_version":1,"events":[{"text":"route b"}]}`),
		},
	}
	capsule, err := buildTeamAggregationContextCapsule(
		plan, aggregator, base, sources, capacityAuthority, capacityCounter,
	)
	if err != nil || !capsule.Valid() {
		t.Fatalf("valid aggregation Capsule error=%v", err)
	}
	for _, item := range capsule.Disclosed() {
		if item.Kind == contextcapsule.KindPriorModelOutput &&
			(item.Trust != contextcapsule.TrustUntrusted || item.SourceType != contextcapsule.SourceModelOutput) {
			t.Fatalf("model output trust was elevated: %#v", item)
		}
	}
	filteredSources := append([]TeamAggregationSource(nil), sources...)
	filteredSources[0].Content = []byte("API_KEY=untrusted-output")
	filtered, err := buildTeamAggregationContextCapsule(
		plan, aggregator, base, filteredSources, capacityAuthority, capacityCounter,
	)
	if err != nil || !filtered.Valid() {
		t.Fatalf("policy-filtered model output Capsule error=%v", err)
	}
	for _, item := range filtered.Omitted() {
		if item.Kind == contextcapsule.KindPriorModelOutput && item.Reason != contextcapsule.OmissionPolicyFiltered &&
			item.Reason != contextcapsule.OmissionBudgetExceeded {
			t.Fatalf("model output omission reason = %s", item.Reason)
		}
	}

	mutations := []struct {
		name   string
		mutate func([]TeamAggregationSource) []TeamAggregationSource
	}{
		{"agent", func(values []TeamAggregationSource) []TeamAggregationSource {
			values[0].AgentInstanceID = "agent-other"
			return values
		}},
		{"summary digest", func(values []TeamAggregationSource) []TeamAggregationSource {
			values[0].OutputSummaryDigest = strings.Repeat("e", 63)
			return values
		}},
		{"source node", func(values []TeamAggregationSource) []TeamAggregationSource {
			values[0].LogicalNodeID = "route-b"
			return values
		}},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			candidate := append([]TeamAggregationSource(nil), sources...)
			candidate = mutation.mutate(candidate)
			if _, err := buildTeamAggregationContextCapsule(
				plan, aggregator, base, candidate, capacityAuthority, capacityCounter,
			); !errors.Is(err, ErrInvalidTeamCoordinator) {
				t.Fatalf("substitution error = %v", err)
			}
		})
	}

	otherPlan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: plan.TeamInstanceID(),
		Nodes: []teams.ExecutionNodeInput{
			{LogicalNodeID: "route-a", Title: "Route A", AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-a", Role: teams.ExecutionRoleMain, Kind: teams.ExecutionNodeRouteSibling, RouteGroupID: "other-group", MaxAttempts: 1},
			{LogicalNodeID: "route-b", Title: "Route B", AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-b", Role: teams.ExecutionRoleMain, Kind: teams.ExecutionNodeRouteSibling, RouteGroupID: "other-group", MaxAttempts: 1},
			{LogicalNodeID: "main", Title: "Synthesis", AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-aggregate", Role: teams.ExecutionRoleMain, Kind: teams.ExecutionNodeAggregation, RouteGroupID: "other-group", DependsOn: []string{"route-a", "route-b"}, MaxAttempts: 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	foreignAggregator, _ := missionContextPlanNode(otherPlan, "main")
	if _, err := buildTeamAggregationContextCapsule(
		plan, foreignAggregator, base, sources, capacityAuthority, capacityCounter,
	); !errors.Is(err, ErrInvalidTeamCoordinator) {
		t.Fatalf("foreign aggregation node error = %v", err)
	}

	driftedAuthority := capacityAuthority
	driftedAuthority.ReservedOutputTokens++
	if _, err := buildTeamAggregationContextCapsule(
		plan, aggregator, base, sources,
		contextcapsule.CapacityAuthority{}, nil,
	); !errors.Is(err, contextcapsule.ErrInvalidCapacityAuthority) {
		t.Fatalf("missing aggregation capacity authority error = %v", err)
	}
	if _, err := buildTeamAggregationContextCapsule(
		plan, aggregator, base, sources, driftedAuthority, capacityCounter,
	); !errors.Is(err, contextcapsule.ErrInvalidCapacityAuthority) {
		t.Fatalf("aggregation authority drift error = %v", err)
	}
	driftedCounter := &teamAggregationCapacityCounter{
		identifier: capacityAuthority.TokenCounterID,
		version:    "v2",
	}
	if _, err := buildTeamAggregationContextCapsule(
		plan, aggregator, base, sources, capacityAuthority, driftedCounter,
	); !errors.Is(err, contextcapsule.ErrInvalidCapacityAuthority) {
		t.Fatalf("aggregation counter drift error = %v", err)
	}

	overflowGoal := []byte("Goal")
	overflowGoalTokens := missionContextTokenCount(overflowGoal)
	overflowCounter := &teamAggregationCapacityCounter{
		identifier: capacityAuthority.TokenCounterID,
		version:    capacityAuthority.TokenCounterVersion,
	}
	baseWithFixedOmission, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(
		contextcapsule.Target{
			ConversationID: "team-conversation:" + plan.TeamInstanceID(), TeamID: plan.TeamInstanceID(),
			AgentID: "agent-main", RoleID: "main", ProviderID: "openai",
			ProviderAccountID: "openai.aggregate", ModelID: "gpt-test",
			AuthMode: string(loomruntime.AuthBrokered), ContextAdapterID: "context:pi:v1",
			DisclosurePolicyID: "policy.test", DisclosurePolicyVersion: 1, TokenBudget: 256,
		},
		[]contextcapsule.ItemInput{
			{ItemID: "goal-1", Kind: contextcapsule.KindConversationGoal, Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeTeamShared, Priority: contextcapsule.PrioritySystem, TokenCount: overflowGoalTokens, Required: true, Content: overflowGoal, SourceType: contextcapsule.SourceAuthority, SourceRef: "team-plan:" + plan.Digest()},
			{ItemID: "fixed-policy-omission", Kind: contextcapsule.KindPriorModelOutput, Trust: contextcapsule.TrustUntrusted, Scope: contextcapsule.ScopeRoleRestricted, Priority: contextcapsule.PriorityHistory, TokenCount: contextcapsule.MaxCapacityTokens - overflowGoalTokens, Content: []byte("fixed private output"), SourceType: contextcapsule.SourceModelOutput, SourceRef: "attempt-output:" + strings.Repeat("f", 64), AllowedRoleID: "main", PolicyFiltered: true},
		},
		capacityAuthority,
		overflowCounter,
	)
	if err != nil || len(baseWithFixedOmission.Omitted()) != 1 {
		t.Fatalf("fixed-omission base error=%v omitted=%d", err, len(baseWithFixedOmission.Omitted()))
	}
	countBeforeOverflow := overflowCounter.inputCount()
	if _, err := buildTeamAggregationContextCapsule(
		plan, aggregator, baseWithFixedOmission, sources, capacityAuthority, overflowCounter,
	); !errors.Is(err, contextcapsule.ErrInvalidCapacityAuthority) {
		t.Fatalf("aggregation cumulative overflow error = %v", err)
	}
	if overflowCounter.inputCount() != countBeforeOverflow {
		t.Fatalf("counter observed extension before overflow admission")
	}
}

func TestDependencyCapsuleRejectsPeerSourceAndKeepsOutputRoleRestricted(t *testing.T) {
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-dependency-capsule",
		Nodes: []teams.ExecutionNodeInput{
			{
				LogicalNodeID: "main", Title: "Integrate",
				AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-main",
				Role: teams.ExecutionRoleMain, DependsOn: []string{"review"}, MaxAttempts: 2,
			},
			{
				LogicalNodeID: "review", Title: "Review",
				AgentInstanceID: "agent-review", RuntimeInstanceID: "runtime-review",
				Role: teams.ExecutionRoleSubAgent, MaxAttempts: 1,
			},
			{
				LogicalNodeID: "peer", Title: "Peer",
				AgentInstanceID: "agent-peer", RuntimeInstanceID: "runtime-peer",
				Role: teams.ExecutionRoleSubAgent, MaxAttempts: 1,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	main, found := missionContextPlanNode(plan, "main")
	if !found {
		t.Fatal("main node not found")
	}
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID: "profile-main", AdapterType: "codex", ProviderID: "openai",
		ProviderAccountID: "openai.main", ModelID: "gpt-test",
		AuthMode: loomruntime.AuthBrokered, EndpointFingerprint: strings.Repeat("a", 64),
		CredentialReference: "credential-ref-main", CredentialRevision: 1,
		Timeout: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	base := testTeamRoleCapsule(t, plan, "main", "agent-main", profile)
	testCommand, err := verification.RecognizeGovernedTestCommand("go test ./...")
	if err != nil {
		t.Fatal(err)
	}
	testReport, err := verification.NewGovernedTestReport(
		testCommand,
		verification.GovernedTestReportInput{
			CallID: "call-review-test", CallSequence: 1,
			ArgumentsDigest: strings.Repeat("1", 64),
			ExecutionID:     "execution-review-test", ExitCode: 0,
			OutputDigest: "sha256:" + strings.Repeat("2", 64), DurationMS: 18,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	source := TeamAggregationSource{
		LogicalNodeID: "review", AttemptNumber: 1,
		WorkItemID: "work-review", RunID: "run-review",
		ClaimID: "11111111-1111-4111-8111-111111111111", ClaimGeneration: 1,
		RuntimeInstanceID: "runtime-review", AgentInstanceID: "agent-review",
		EvidenceID: "evidence-review", EvidenceDigest: strings.Repeat("b", 64),
		OutputSummaryDigest: strings.Repeat("c", 64),
		TerminalStatus:      "succeeded", OutputContractVersion: 1,
		OutputContractDigest:       strings.Repeat("d", 64),
		OutputClassification:       "valid_nonempty",
		OutputClassificationDigest: strings.Repeat("e", 64),
		AcceptanceDecisionKind:     "accepted", AcceptanceDecisionDigest: strings.Repeat("f", 64),
		AcceptanceDecisionTime: time.Date(2026, 8, 15, 2, 0, 0, 0, time.UTC),
		GovernedTestReports:    []verification.GovernedTestReport{testReport},
		Content:                []byte(`{"schema_version":1,"events":[{"delta":"reviewed"}]}`),
	}
	capsule, err := buildTeamDependencyContextCapsule(
		plan, main, base, []TeamAggregationSource{source},
		missionContextCapacityAuthority(), missionContextCounter,
	)
	if err != nil || !capsule.Valid() {
		t.Fatalf("dependency Capsule = %#v, err=%v", capsule.AuthorityRecord(), err)
	}
	authorityCount, observationCount, outputCount := 0, 0, 0
	for _, item := range capsule.Disclosed() {
		switch item.Kind {
		case contextcapsule.KindDependencySource:
			authorityCount++
			if item.Trust != contextcapsule.TrustAuthoritative || item.AllowedRoleID != "main" {
				t.Fatalf("dependency authority = %#v", item)
			}
		case contextcapsule.KindObservedExecutionState:
			observationCount++
			if item.Trust != contextcapsule.TrustObserved ||
				item.SourceType != contextcapsule.SourceObservation ||
				item.AllowedRoleID != "main" ||
				!bytes.Contains(item.Content, []byte(`"test_report_count":1`)) ||
				!bytes.Contains(item.Content, []byte(`"runner":"go_test"`)) ||
				!bytes.Contains(item.Content, []byte(testReport.Digest())) {
				t.Fatalf("dependency observation = %#v", item)
			}
		case contextcapsule.KindPriorModelOutput:
			outputCount++
			if item.Trust != contextcapsule.TrustUntrusted ||
				item.SourceType != contextcapsule.SourceModelOutput ||
				item.AllowedRoleID != "main" {
				t.Fatalf("dependency output = %#v", item)
			}
		}
	}
	if authorityCount != 1 || observationCount != 1 || outputCount != 1 {
		t.Fatalf(
			"dependency items authority=%d observation=%d output=%d",
			authorityCount, observationCount, outputCount,
		)
	}
	for name, mutate := range map[string]func(TeamAggregationSource) TeamAggregationSource{
		"peer source": func(value TeamAggregationSource) TeamAggregationSource {
			value.LogicalNodeID = "peer"
			value.AgentInstanceID = "agent-peer"
			value.RuntimeInstanceID = "runtime-peer"
			return value
		},
		"agent substitution": func(value TeamAggregationSource) TeamAggregationSource {
			value.AgentInstanceID = "agent-peer"
			return value
		},
		"evidence substitution": func(value TeamAggregationSource) TeamAggregationSource {
			value.EvidenceDigest = strings.Repeat("d", 63)
			return value
		},
		"classification substitution": func(value TeamAggregationSource) TeamAggregationSource {
			value.OutputClassification = "invalid"
			return value
		},
		"acceptance substitution": func(value TeamAggregationSource) TeamAggregationSource {
			value.AcceptanceDecisionDigest = strings.Repeat("f", 63)
			return value
		},
		"duplicate test report sequence": func(value TeamAggregationSource) TeamAggregationSource {
			value.GovernedTestReports = []verification.GovernedTestReport{testReport, testReport}
			return value
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := buildTeamDependencyContextCapsule(
				plan, main, base, []TeamAggregationSource{mutate(source)},
				missionContextCapacityAuthority(), missionContextCounter,
			); !errors.Is(err, ErrInvalidTeamCoordinator) {
				t.Fatalf("substitution error = %v", err)
			}
		})
	}
}

func newFourProviderTeamCanaryFixture(
	t testing.TB,
	teamInstanceID string,
) fourProviderTeamCanaryFixture {
	return newFourProviderTeamCanaryFixtureWithMainAttempts(t, teamInstanceID, 1)
}

func newFourProviderTeamCanaryFixtureWithMainAttempts(
	t testing.TB,
	teamInstanceID string,
	mainAttempts int,
) fourProviderTeamCanaryFixture {
	t.Helper()
	if mainAttempts < 1 || mainAttempts > 2 {
		t.Fatal("invalid main Attempt count")
	}
	ctx := context.Background()
	now := time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC)
	db := openTeamCanaryDB(t)
	store := journal.NewStore(db)
	runtimeIDs := map[string]string{
		"main": "runtime-codex-revoke", "sub-a": "runtime-claude-revoke",
		"sub-b": "runtime-kimi-revoke", "sub-c": "runtime-minimax-revoke",
	}
	for _, runtimeID := range runtimeIDs {
		seedTeamCanaryRuntime(t, store, runtimeID, 1, now)
	}
	clock := &teamCanaryClock{now: now}
	workAuthority, err := work.NewAuthority(
		store, clock.Now, bytes.NewReader(bytes.Repeat([]byte{0x81}, 2048)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := workAuthority.InitializeRunIdentityIndex(ctx); err != nil {
		t.Fatal(err)
	}
	grantRandom := make([]byte, 48*16)
	for index := range grantRandom {
		grantRandom[index] = 0x21 + byte(index/48)
	}
	grantAuthority, err := authorization.NewAuthority(
		store, workAuthority, clock.Now, bytes.NewReader(grantRandom),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := grantAuthority.InitializeGrantIdentityIndex(ctx); err != nil {
		t.Fatal(err)
	}
	readModel := projection.New(db)
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	evidenceParent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	artifactStore, err := evidence.NewStore(filepath.Join(evidenceParent, "evidence"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = artifactStore.Close() })
	coordinator, err := NewTeamCoordinator(
		workAuthority, grantAuthority, readModel, artifactStore,
	)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: teamInstanceID,
		Nodes: []teams.ExecutionNodeInput{
			{LogicalNodeID: "main", Title: "Codex", AgentInstanceID: "agent-codex-revoke", RuntimeInstanceID: runtimeIDs["main"], Role: teams.ExecutionRoleMain, MaxAttempts: mainAttempts},
			{LogicalNodeID: "sub-a", Title: "Claude", AgentInstanceID: "agent-claude-revoke", RuntimeInstanceID: runtimeIDs["sub-a"], Role: teams.ExecutionRoleSubAgent, MaxAttempts: 1},
			{LogicalNodeID: "sub-b", Title: "Kimi", AgentInstanceID: "agent-kimi-revoke", RuntimeInstanceID: runtimeIDs["sub-b"], Role: teams.ExecutionRoleSubAgent, MaxAttempts: 1},
			{LogicalNodeID: "sub-c", Title: "MiniMax", AgentInstanceID: "agent-minimax-revoke", RuntimeInstanceID: runtimeIDs["sub-c"], Role: teams.ExecutionRoleSubAgent, MaxAttempts: 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	providers := map[string]string{
		"main": "openai", "sub-a": "anthropic", "sub-b": "kimi", "sub-c": "minimax",
	}
	models := map[string]string{
		"main": "gpt-5.5-codex", "sub-a": "claude-sonnet-5",
		"sub-b": "kimi-k2.6", "sub-c": "MiniMax-M3",
	}
	harnesses := map[string]string{
		"main": "codex", "sub-a": "claude-code", "sub-b": "loom-native", "sub-c": "loom-native",
	}
	revisions := map[string]int64{"main": 3, "sub-a": 7, "sub-b": 11, "sub-c": 13}
	adapters := make(map[string]*teamCanaryAdapter, len(runtimeIDs))
	nodes := make([]TeamNodeExecution, 0, len(runtimeIDs))
	for _, node := range plan.Nodes() {
		logicalNodeID := node.LogicalNodeID()
		budget := int64(100) + revisions[logicalNodeID]
		profile, profileErr := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
			ID:          "profile-revoke-" + logicalNodeID,
			AdapterType: harnesses[logicalNodeID], ProviderID: providers[logicalNodeID],
			ProviderAccountID: providers[logicalNodeID] + "." + logicalNodeID,
			ModelID:           models[logicalNodeID], AuthMode: loomruntime.AuthBrokered,
			EndpointFingerprint:  strings.Repeat("f", 64),
			CredentialReference:  "credential-ref-revoke-" + logicalNodeID,
			CredentialRevision:   revisions[logicalNodeID],
			RequiredCapabilities: []string{"text"}, Timeout: 5 * time.Second,
			Budget: &budget,
		})
		if profileErr != nil {
			t.Fatal(profileErr)
		}
		instance, instanceErr := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
			ID: runtimeIDs[logicalNodeID], DeviceID: "device-1",
			AdapterType: harnesses[logicalNodeID], DisplayName: logicalNodeID,
			ExecutableVersion: "1.0.0", Status: loomruntime.RuntimeOnline,
			ObservedCapabilities: []string{"text"}, Capacity: 1,
		})
		if instanceErr != nil {
			t.Fatal(instanceErr)
		}
		adapter := &teamCanaryAdapter{
			barrier:     &teamCanaryBarrier{release: make(chan struct{})},
			adapterType: harnesses[logicalNodeID], runtimeID: runtimeIDs[logicalNodeID],
		}
		adapters[logicalNodeID] = adapter
		execution := teamCanaryNodeExecution(
			t, plan, logicalNodeID, 1, node.AgentInstanceID(), runtimeIDs[logicalNodeID],
			now, newTeamCanarySupervisor(t, workAuthority, grantAuthority, adapter),
		)
		execution.Profile = profile
		execution.Instance = instance
		execution.ContextCapsule = testTeamRoleCapsule(
			t, plan, logicalNodeID, node.AgentInstanceID(), profile,
		)
		payload, renderErr := contextcapsule.RenderDispatchPayload(execution.ContextCapsule)
		if renderErr != nil {
			t.Fatal(renderErr)
		}
		execution.Dispatch = replaceTeamDispatchPayload(t, execution.Dispatch, payload)
		nodes = append(nodes, execution)
		if logicalNodeID == "main" && mainAttempts == 2 {
			fallback := teamCanaryNodeExecution(
				t, plan, logicalNodeID, 2, node.AgentInstanceID(), runtimeIDs[logicalNodeID],
				now, execution.Executor,
			)
			fallback.Profile = profile
			fallback.Instance = instance
			fallback.ContextCapsule = testTeamRoleCapsule(
				t, plan, logicalNodeID, node.AgentInstanceID(), profile,
			)
			fallbackPayload, fallbackErr := contextcapsule.RenderDispatchPayload(
				fallback.ContextCapsule,
			)
			if fallbackErr != nil {
				t.Fatal(fallbackErr)
			}
			fallback.Dispatch = replaceTeamDispatchPayload(
				t, fallback.Dispatch, fallbackPayload,
			)
			nodes = append(nodes, fallback)
		}
	}
	return fourProviderTeamCanaryFixture{
		coordinator: coordinator,
		request: TeamExecutionRequest{
			Plan: plan, Nodes: nodes, Semantics: testTeamNodeSemantics(t, plan, 0, ""),
			AuthoritativeTime: now, PrepareLeaseDuration: time.Minute,
			GrantLifetime: time.Minute,
			CorrelationID: "11111111-1111-4111-8111-111111111111",
		},
		adapters:  adapters,
		store:     store,
		readModel: readModel,
	}
}

func TestFourProviderTeamRevokedCredentialIsolatesOneAgent(t *testing.T) {
	fixture := newFourProviderTeamCanaryFixture(t, "team-revoked-canary")
	revokedAccount := "minimax.sub-c"
	leaseManager, err := credentialvault.NewCredentialLeaseManager(
		teamCanaryCredentialReader{},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = leaseManager.Close() })
	revokedIdentity := credentialvault.CredentialIdentity{
		ProviderID: "minimax", ProviderAccountID: revokedAccount,
		CredentialReference: "credential-ref-revoke-sub-c", CredentialRevision: 13,
	}
	leaseManager.Revoke(revokedIdentity)
	for index := range fixture.request.Nodes {
		execution := &fixture.request.Nodes[index]
		adapter := fixture.adapters[execution.LogicalNodeID]
		adapter.terminalStatus = "succeeded"
		adapter.terminalReason = ""
		adapter.accounting = nil
		adapter.credentialUse = func(
			ctx context.Context,
			binding loomruntime.FrozenExecutionBinding,
			use func(context.Context, []byte) error,
		) error {
			identity := credentialvault.CredentialIdentity{
				ProviderID: binding.ProviderID, ProviderAccountID: binding.ProviderAccountID,
				CredentialReference: binding.CredentialReference,
				CredentialRevision:  binding.CredentialRevision,
			}
			lease, acquireErr := leaseManager.Acquire(ctx, identity, time.Minute)
			if acquireErr != nil {
				stage := credentials.CredentialStageLeaseIssue
				if errors.Is(acquireErr, credentialvault.ErrCredentialLeaseRevoked) {
					stage = credentials.CredentialStageLeaseRevoke
				}
				return credentials.WithCredentialFailureStage(stage, acquireErr)
			}
			defer lease.Close()
			return lease.WithSecret(use)
		}
	}

	result, err := fixture.coordinator.Run(context.Background(), fixture.request)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Team().Status() != "blocked" || len(result.ExecutedNodeIDs()) != 4 {
		t.Fatalf("Team result = %q, nodes=%v", result.Team().Status(), result.ExecutedNodeIDs())
	}
	for _, node := range result.Team().Nodes() {
		attempts := node.Attempts()
		if len(attempts) != 1 {
			t.Fatalf("%s Attempts = %#v", node.LogicalNodeID(), attempts)
		}
		binding := attempts[0].ExecutionBinding()
		if node.LogicalNodeID() == "sub-c" {
			if node.Status() != "blocked" ||
				attempts[0].TerminalReason() != "credential_unavailable" ||
				binding.ProviderAccountID != revokedAccount {
				t.Fatalf("revoked Agent = %#v", node)
			}
			continue
		}
		if node.Status() != "succeeded" || attempts[0].TerminalReason() != "" {
			t.Fatalf("healthy Agent %s = %#v", node.LogicalNodeID(), node)
		}
	}
}

func TestFourProviderTeamCorruptVaultRecordIsolatesOneAgent(t *testing.T) {
	fixture := newFourProviderTeamCanaryFixture(t, "team-corrupt-vault-canary")
	root := t.TempDir()
	privateDirectory := filepath.Join(root, "private")
	stateDirectory := filepath.Join(root, "state")
	for _, directory := range []string{privateDirectory, stateDirectory} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	keyPath := filepath.Join(privateDirectory, "vault.key")
	databasePath := filepath.Join(stateDirectory, "credential-vault.db")
	material, err := (credentialvault.LocalKeyFile{Path: keyPath}).LoadOrCreate(
		context.Background(),
	)
	if err != nil {
		t.Fatal(err)
	}
	store, err := credentialvault.OpenStore(credentialvault.StoreConfig{
		DatabasePath: databasePath, KeyMaterial: material,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if store != nil {
			_ = store.Close()
		}
	})
	identities := make(map[string]credentialvault.CredentialIdentity)
	secretMarkers := make([][]byte, 0, len(fixture.request.Nodes))
	defer func() {
		for _, marker := range secretMarkers {
			clearTeamCanaryBytes(marker)
		}
	}()
	for _, execution := range fixture.request.Nodes {
		binding, freezeErr := loomruntime.FreezeExecutionBinding(
			execution.Profile, execution.Instance,
		)
		if freezeErr != nil {
			t.Fatal(freezeErr)
		}
		identity := credentialvault.CredentialIdentity{
			ProviderID: binding.ProviderID, ProviderAccountID: binding.ProviderAccountID,
			CredentialReference: binding.CredentialReference,
			CredentialRevision:  binding.CredentialRevision,
		}
		identities[execution.LogicalNodeID] = identity
		secret := []byte("vault-record-isolation-" + execution.LogicalNodeID)
		secretMarkers = append(secretMarkers, append([]byte(nil), secret...))
		if putErr := store.PutCredential(context.Background(), identity, secret); putErr != nil {
			clearTeamCanaryBytes(secret)
			t.Fatal(putErr)
		}
		clearTeamCanaryBytes(secret)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	databaseBytes, err := os.ReadFile(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range secretMarkers {
		if bytes.Contains(databaseBytes, marker) {
			clearTeamCanaryBytes(databaseBytes)
			t.Fatal("credential secret remained plaintext in encrypted Vault database")
		}
	}
	clearTeamCanaryBytes(databaseBytes)

	corruptDatabase, err := sql.Open("sqlite", "file:"+databasePath)
	if err != nil {
		t.Fatal(err)
	}
	corruptIdentity := identities["sub-c"]
	result, err := corruptDatabase.Exec(
		`UPDATE encrypted_credentials
		    SET ciphertext = zeroblob(length(ciphertext))
		  WHERE credential_reference = ? AND provider_id = ?
		    AND provider_account_id = ? AND credential_revision = ?`,
		corruptIdentity.CredentialReference, corruptIdentity.ProviderID,
		corruptIdentity.ProviderAccountID, corruptIdentity.CredentialRevision,
	)
	if err != nil {
		_ = corruptDatabase.Close()
		t.Fatal(err)
	}
	if count, rowsErr := result.RowsAffected(); rowsErr != nil || count != 1 {
		_ = corruptDatabase.Close()
		t.Fatalf("corrupted rows = %d, %v", count, rowsErr)
	}
	if err := corruptDatabase.Close(); err != nil {
		t.Fatal(err)
	}

	material, err = (credentialvault.LocalKeyFile{Path: keyPath}).LoadOrCreate(
		context.Background(),
	)
	if err != nil {
		t.Fatal(err)
	}
	store, err = credentialvault.OpenStore(credentialvault.StoreConfig{
		DatabasePath: databasePath, KeyMaterial: material,
	})
	if err != nil {
		t.Fatal(err)
	}
	leaseManager, err := credentialvault.NewCredentialLeaseManager(store)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = leaseManager.Close() })

	stages := make(map[string]string)
	var stageMu sync.Mutex
	for index := range fixture.request.Nodes {
		execution := &fixture.request.Nodes[index]
		logicalNodeID := execution.LogicalNodeID
		adapter := fixture.adapters[logicalNodeID]
		adapter.terminalStatus = "succeeded"
		adapter.terminalReason = ""
		adapter.accounting = nil
		adapter.credentialUse = func(
			ctx context.Context,
			binding loomruntime.FrozenExecutionBinding,
			use func(context.Context, []byte) error,
		) error {
			identity := credentialvault.CredentialIdentity{
				ProviderID: binding.ProviderID, ProviderAccountID: binding.ProviderAccountID,
				CredentialReference: binding.CredentialReference,
				CredentialRevision:  binding.CredentialRevision,
			}
			lease, acquireErr := leaseManager.Acquire(ctx, identity, time.Minute)
			if acquireErr != nil {
				stageMu.Lock()
				stages[logicalNodeID] = credentials.CredentialFailureStage(acquireErr)
				stageMu.Unlock()
				return acquireErr
			}
			defer lease.Close()
			return lease.WithSecret(use)
		}
	}

	teamResult, err := fixture.coordinator.Run(context.Background(), fixture.request)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if teamResult.Team().Status() != "blocked" ||
		len(teamResult.ExecutedNodeIDs()) != 4 {
		t.Fatalf(
			"Team result = %q, nodes=%v",
			teamResult.Team().Status(), teamResult.ExecutedNodeIDs(),
		)
	}
	for _, node := range teamResult.Team().Nodes() {
		attempts := node.Attempts()
		if len(attempts) != 1 {
			t.Fatalf("%s Attempts = %#v", node.LogicalNodeID(), attempts)
		}
		if node.LogicalNodeID() == "sub-c" {
			if node.Status() != "blocked" ||
				attempts[0].TerminalReason() != "credential_unavailable" {
				t.Fatalf("corrupt Vault Agent = %#v", node)
			}
			continue
		}
		if node.Status() != "succeeded" || attempts[0].TerminalReason() != "" {
			t.Fatalf("healthy Agent %s = %#v", node.LogicalNodeID(), node)
		}
	}
	stageMu.Lock()
	defer stageMu.Unlock()
	if len(stages) != 1 ||
		stages["sub-c"] != credentials.CredentialStageVaultDecrypt {
		t.Fatalf("credential failure stages = %#v", stages)
	}
	events, err := fixture.store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		for _, marker := range secretMarkers {
			if bytes.Contains(event.PayloadJSON, marker) {
				t.Fatalf("credential secret reached Journal event %s", event.Type)
			}
		}
	}
}

func TestFourProviderTeamApprovedAccountFallbackUsesIndependentVaultRecord(t *testing.T) {
	fixture := newFourProviderTeamCanaryFixtureWithMainAttempts(
		t, "team-vault-fallback-canary", 2,
	)
	primary := &fixture.request.Nodes[0]
	fallback := &fixture.request.Nodes[1]
	if primary.LogicalNodeID != "main" || primary.AttemptNumber != 1 ||
		fallback.LogicalNodeID != "main" || fallback.AttemptNumber != 2 {
		t.Fatalf("unexpected main Attempt order = %#v", fixture.request.Nodes[:2])
	}
	fallback.WorkflowPath = "approved-openai-account-fallback"
	fallback.Profile.ID = "profile-openai-main-backup-r5"
	fallback.Profile.ProviderAccountID = "openai.main-backup"
	fallback.Profile.ModelID = "gpt-5.5-codex"
	fallback.Profile.CredentialReference = "credential-ref-openai-main-backup"
	fallback.Profile.CredentialRevision = 5
	fallback.ContextCapsule = testTeamRoleCapsule(
		t, fixture.request.Plan, "main", "agent-codex-revoke", fallback.Profile,
	)
	fallbackPayload, err := contextcapsule.RenderDispatchPayload(fallback.ContextCapsule)
	if err != nil {
		t.Fatal(err)
	}
	fallback.Dispatch = replaceTeamDispatchPayload(t, fallback.Dispatch, fallbackPayload)

	source := teamCanaryExecutionBinding(t, *primary)
	target := teamCanaryExecutionBinding(t, *fallback)
	policy, err := rules.NewRecoveryPolicy(rules.RecoveryPolicyInput{
		Version:             2,
		AttemptCredits:      1,
		ExhaustionAction:    rules.ExhaustionBlocked,
		RetryInvalid:        true,
		WorkflowFallbackKey: "approved-openai-account-fallback",
	})
	if err != nil {
		t.Fatal(err)
	}
	approval, err := work.NewTeamFallbackApproval(work.TeamFallbackApprovalInput{
		Version:             1,
		ApprovalID:          "fallback-approval-four-provider-openai-v1",
		ActorRef:            "user:local-owner",
		ApprovedAt:          fixture.request.AuthoritativeTime,
		SourceBindingDigest: source.BindingDigest,
		TargetBindingDigest: target.BindingDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	for index := range fixture.request.Semantics {
		if fixture.request.Semantics[index].LogicalNodeID == "main" {
			fixture.request.Semantics[index].RecoveryPolicy = policy
			fixture.request.Semantics[index].FallbackApproval = approval
		}
	}

	root := t.TempDir()
	privateDirectory := filepath.Join(root, "private")
	stateDirectory := filepath.Join(root, "state")
	for _, directory := range []string{privateDirectory, stateDirectory} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	keyPath := filepath.Join(privateDirectory, "vault.key")
	databasePath := filepath.Join(stateDirectory, "credential-vault.db")
	material, err := (credentialvault.LocalKeyFile{Path: keyPath}).LoadOrCreate(
		context.Background(),
	)
	if err != nil {
		t.Fatal(err)
	}
	vaultStore, err := credentialvault.OpenStore(credentialvault.StoreConfig{
		DatabasePath: databasePath, KeyMaterial: material,
	})
	if err != nil {
		t.Fatal(err)
	}
	identities := make(map[string]credentialvault.CredentialIdentity)
	secretMarkers := make([][]byte, 0, len(fixture.request.Nodes))
	defer func() {
		for _, marker := range secretMarkers {
			clearTeamCanaryBytes(marker)
		}
	}()
	for _, execution := range fixture.request.Nodes {
		binding := teamCanaryExecutionBinding(t, execution)
		identity := teamCanaryCredentialIdentity(binding)
		key := appExecutionKey(execution.LogicalNodeID, execution.AttemptNumber)
		identities[key] = identity
		secret := []byte("vault-mixed-fallback-" + key)
		secretMarkers = append(secretMarkers, append([]byte(nil), secret...))
		if err := vaultStore.PutCredential(
			context.Background(), identity, secret,
		); err != nil {
			clearTeamCanaryBytes(secret)
			t.Fatal(err)
		}
		clearTeamCanaryBytes(secret)
	}
	if err := vaultStore.Close(); err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("sqlite", "file:"+databasePath)
	if err != nil {
		t.Fatal(err)
	}
	primaryIdentity := identities[appExecutionKey("main", 1)]
	corrupted, err := database.Exec(
		`UPDATE encrypted_credentials
		    SET ciphertext = zeroblob(length(ciphertext))
		  WHERE credential_reference = ? AND provider_id = ?
		    AND provider_account_id = ? AND credential_revision = ?`,
		primaryIdentity.CredentialReference, primaryIdentity.ProviderID,
		primaryIdentity.ProviderAccountID, primaryIdentity.CredentialRevision,
	)
	if err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if rows, rowsErr := corrupted.RowsAffected(); rowsErr != nil || rows != 1 {
		_ = database.Close()
		t.Fatalf("corrupted rows = %d, %v", rows, rowsErr)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	material, err = (credentialvault.LocalKeyFile{Path: keyPath}).LoadOrCreate(
		context.Background(),
	)
	if err != nil {
		t.Fatal(err)
	}
	vaultStore, err = credentialvault.OpenStore(credentialvault.StoreConfig{
		DatabasePath: databasePath, KeyMaterial: material,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer vaultStore.Close()
	leasing, err := credentialvault.NewCredentialLeaseManager(vaultStore)
	if err != nil {
		t.Fatal(err)
	}
	defer leasing.Close()

	var observedMu sync.Mutex
	observed := make(map[credentialvault.CredentialIdentity]int)
	stages := make(map[credentialvault.CredentialIdentity]string)
	wantAccounting := map[string]projection.RunAccounting{
		"main": {
			UsageObserved: true, InputTokens: 50, OutputTokens: 5,
			TotalTokens: 55, CostObserved: true,
			CostMicrounits: 5_000, CostCurrency: "USD",
			CostSource: work.CostSourceProviderReported,
		},
		"sub-a": {
			UsageObserved: true, InputTokens: 70, OutputTokens: 7,
			TotalTokens: 77, CostObserved: true,
			CostMicrounits: 7_000, CostCurrency: "USD",
			CostSource: work.CostSourceProviderReported,
		},
		"sub-b": {
			UsageObserved: true, InputTokens: 110, OutputTokens: 11,
			TotalTokens: 121, CostObserved: true,
			CostMicrounits: 11_000, CostCurrency: "USD",
			CostSource: work.CostSourceProviderReported,
		},
		"sub-c": {
			UsageObserved: true, InputTokens: 130, OutputTokens: 13,
			TotalTokens: 143, CostObserved: true,
			CostMicrounits: 13_000, CostCurrency: "USD",
			CostSource: work.CostSourceProviderReported,
		},
	}
	for logicalNodeID, adapter := range fixture.adapters {
		adapter.terminalStatus = "succeeded"
		adapter.terminalReason = ""
		accounting := wantAccounting[logicalNodeID]
		adapter.accounting = &work.RunAccounting{
			UsageObserved: accounting.UsageObserved,
			InputTokens:   accounting.InputTokens, OutputTokens: accounting.OutputTokens,
			TotalTokens: accounting.TotalTokens, CostObserved: accounting.CostObserved,
			CostMicrounits: accounting.CostMicrounits,
			CostCurrency:   accounting.CostCurrency, CostSource: accounting.CostSource,
		}
		adapter.credentialUse = func(
			ctx context.Context,
			binding loomruntime.FrozenExecutionBinding,
			use func(context.Context, []byte) error,
		) error {
			identity := teamCanaryCredentialIdentity(binding)
			observedMu.Lock()
			observed[identity]++
			observedMu.Unlock()
			lease, acquireErr := leasing.Acquire(ctx, identity, time.Minute)
			if acquireErr != nil {
				observedMu.Lock()
				stages[identity] = credentials.CredentialFailureStage(acquireErr)
				observedMu.Unlock()
				return acquireErr
			}
			defer lease.Close()
			return lease.WithSecret(use)
		}
	}

	teamResult, err := fixture.coordinator.Run(context.Background(), fixture.request)
	if err != nil || teamResult.Team().Status() != "succeeded" ||
		len(teamResult.ExecutedNodeIDs()) != 5 {
		t.Fatalf("mixed fallback Team = %#v, nodes=%v, %v",
			teamResult.Team(), teamResult.ExecutedNodeIDs(), err)
	}
	for _, node := range teamResult.Team().Nodes() {
		attempts := node.Attempts()
		if node.LogicalNodeID() == "main" {
			if node.Status() != "succeeded" || len(attempts) != 2 ||
				attempts[0].TerminalReason() != "credential_unavailable" ||
				attempts[0].ExecutionBinding().BindingDigest != source.BindingDigest ||
				attempts[1].TerminalReason() != "" ||
				attempts[1].ExecutionBinding().BindingDigest != target.BindingDigest ||
				attempts[1].ExecutionBinding().ProviderAccountID != "openai.main-backup" ||
				attempts[1].ExecutionBinding().CredentialRevision != 5 {
				t.Fatalf("main fallback Agent = %#v", node)
			}
			continue
		}
		if node.Status() != "succeeded" || len(attempts) != 1 ||
			attempts[0].TerminalReason() != "" {
			t.Fatalf("healthy peer Agent %s = %#v", node.LogicalNodeID(), node)
		}
	}
	if err := fixture.readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	projected, ok := fixture.readModel.GlobalReadView().TeamExecution(
		fixture.request.Plan.TeamInstanceID(),
	)
	if !ok || projected.Status != "succeeded" {
		t.Fatalf("projected mixed fallback Team = %#v, %t", projected, ok)
	}
	for _, node := range projected.Nodes {
		if node.LogicalNodeID == "main" {
			if len(node.Attempts) != 2 ||
				node.Attempts[0].ExecutionBinding.ProviderAccountID != "openai.main" ||
				node.Attempts[0].AccountingAvailable ||
				node.Attempts[1].ExecutionBinding.ProviderAccountID !=
					"openai.main-backup" ||
				!node.Attempts[1].AccountingAvailable ||
				node.Attempts[1].Accounting != wantAccounting["main"] {
				t.Fatalf("projected main fallback accounting = %#v", node.Attempts)
			}
			continue
		}
		if len(node.Attempts) != 1 ||
			!node.Attempts[0].AccountingAvailable ||
			node.Attempts[0].Accounting != wantAccounting[node.LogicalNodeID] {
			t.Fatalf("projected peer accounting %s = %#v", node.LogicalNodeID, node.Attempts)
		}
	}
	observedMu.Lock()
	defer observedMu.Unlock()
	if len(observed) != len(identities) || len(stages) != 1 ||
		stages[primaryIdentity] != credentials.CredentialStageVaultDecrypt {
		t.Fatalf("Vault observations=%#v stages=%#v", observed, stages)
	}
	for _, identity := range identities {
		if observed[identity] != 1 {
			t.Fatalf("Vault identity %v observed %d times", identity, observed[identity])
		}
	}
	events, err := fixture.store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		for _, marker := range secretMarkers {
			if bytes.Contains(event.PayloadJSON, marker) {
				t.Fatalf("credential secret reached Journal event %s", event.Type)
			}
		}
	}
}

func TestFourProviderTeamContinuesAfterVaultRotation(t *testing.T) {
	fixture := newFourProviderTeamCanaryFixture(t, "team-vault-rotation-canary")
	root := t.TempDir()
	privateDirectory := filepath.Join(root, "private")
	stateDirectory := filepath.Join(root, "state")
	for _, directory := range []string{privateDirectory, stateDirectory} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	keyPath := filepath.Join(privateDirectory, "vault.key")
	pendingKeyPath := filepath.Join(privateDirectory, "vault.key.rotation-pending")
	databasePath := filepath.Join(stateDirectory, "credential-vault.db")
	material, err := (credentialvault.LocalKeyFile{Path: keyPath}).LoadOrCreate(
		context.Background(),
	)
	if err != nil {
		t.Fatal(err)
	}
	store, err := credentialvault.OpenStore(credentialvault.StoreConfig{
		DatabasePath: databasePath, KeyMaterial: material,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	identities := make(map[string]credentialvault.CredentialIdentity)
	wantBindings := make(map[string]loomruntime.FrozenExecutionBinding)
	for _, execution := range fixture.request.Nodes {
		binding, freezeErr := loomruntime.FreezeExecutionBinding(
			execution.Profile, execution.Instance,
		)
		if freezeErr != nil {
			t.Fatal(freezeErr)
		}
		identity := credentialvault.CredentialIdentity{
			ProviderID: binding.ProviderID, ProviderAccountID: binding.ProviderAccountID,
			CredentialReference: binding.CredentialReference,
			CredentialRevision:  binding.CredentialRevision,
		}
		identities[execution.LogicalNodeID] = identity
		wantBindings[execution.LogicalNodeID] = binding
		secret := []byte("vault-rotation-isolation-" + execution.LogicalNodeID)
		if putErr := store.PutCredential(context.Background(), identity, secret); putErr != nil {
			clearTeamCanaryBytes(secret)
			t.Fatal(putErr)
		}
		clearTeamCanaryBytes(secret)
	}
	leaseManager, err := credentialvault.NewCredentialLeaseManager(store)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = leaseManager.Close() })
	oldLease, err := leaseManager.Acquire(
		context.Background(), identities["main"], time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	newMaterial, err := (credentialvault.LocalKeyFile{Path: pendingKeyPath}).Create(
		context.Background(), 2,
	)
	if err != nil {
		t.Fatal(err)
	}
	transferred := false
	err = leaseManager.WithRotationBarrier(context.Background(), func() error {
		if rotateErr := store.RotateWrappingKey(
			context.Background(), newMaterial,
		); rotateErr != nil {
			return rotateErr
		}
		transferred = true
		if recoverErr := credentialvault.RecoverLocalKeyRotation(
			context.Background(), databasePath, keyPath, pendingKeyPath,
		); recoverErr != nil {
			return recoverErr
		}
		canonical, loadErr := (credentialvault.LocalKeyFile{Path: keyPath}).LoadOrCreate(
			context.Background(),
		)
		if loadErr != nil {
			return loadErr
		}
		if adoptErr := store.AdoptRotatedKeyMaterial(
			context.Background(), canonical,
		); adoptErr != nil {
			_ = canonical.Close()
			return adoptErr
		}
		return nil
	})
	if err != nil {
		if !transferred {
			_ = newMaterial.Close()
		}
		t.Fatal(err)
	}
	if err := oldLease.WithSecret(
		func(context.Context, []byte) error { return nil },
	); !errors.Is(err, credentialvault.ErrCredentialLeaseRevoked) {
		t.Fatalf("pre-rotation lease error = %v", err)
	}
	version, _, err := store.KeyIdentity(context.Background())
	if err != nil || version != 2 {
		t.Fatalf("rotated Vault key version = %d, %v", version, err)
	}
	if _, err := os.Lstat(pendingKeyPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pending rotation key remains: %v", err)
	}
	if err := leaseManager.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	material, err = (credentialvault.LocalKeyFile{Path: keyPath}).LoadOrCreate(
		context.Background(),
	)
	if err != nil || material.KeyVersion() != 2 {
		t.Fatalf("restarted Vault key material = %v, %v", material, err)
	}
	store, err = credentialvault.OpenStore(credentialvault.StoreConfig{
		DatabasePath: databasePath, KeyMaterial: material,
	})
	if err != nil {
		t.Fatal(err)
	}
	leaseManager, err = credentialvault.NewCredentialLeaseManager(store)
	if err != nil {
		t.Fatal(err)
	}

	for index := range fixture.request.Nodes {
		execution := &fixture.request.Nodes[index]
		adapter := fixture.adapters[execution.LogicalNodeID]
		adapter.terminalStatus = "succeeded"
		adapter.terminalReason = ""
		adapter.accounting = nil
		adapter.credentialUse = func(
			ctx context.Context,
			binding loomruntime.FrozenExecutionBinding,
			use func(context.Context, []byte) error,
		) error {
			identity := credentialvault.CredentialIdentity{
				ProviderID: binding.ProviderID, ProviderAccountID: binding.ProviderAccountID,
				CredentialReference: binding.CredentialReference,
				CredentialRevision:  binding.CredentialRevision,
			}
			lease, acquireErr := leaseManager.Acquire(ctx, identity, time.Minute)
			if acquireErr != nil {
				return acquireErr
			}
			defer lease.Close()
			return lease.WithSecret(use)
		}
	}
	teamResult, err := fixture.coordinator.Run(context.Background(), fixture.request)
	if err != nil || teamResult.Team().Status() != "succeeded" ||
		len(teamResult.ExecutedNodeIDs()) != 4 {
		t.Fatalf(
			"post-rotation Team = %#v, nodes=%v, %v",
			teamResult.Team(), teamResult.ExecutedNodeIDs(), err,
		)
	}
	for _, node := range teamResult.Team().Nodes() {
		attempts := node.Attempts()
		if node.Status() != "succeeded" || len(attempts) != 1 ||
			!reflect.DeepEqual(
				attempts[0].ExecutionBinding(),
				wantBindings[node.LogicalNodeID()],
			) {
			t.Fatalf("post-rotation Agent %s = %#v", node.LogicalNodeID(), node)
		}
	}
	events, err := fixture.store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if bytes.Contains(event.PayloadJSON, []byte("vault-rotation-isolation-")) {
			t.Fatalf("rotation credential secret reached Journal event %s", event.Type)
		}
	}
}

func clearTeamCanaryBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}

func TestTeamCoordinatorWaveLimitScalesWithTopologyAndRetries(t *testing.T) {
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-wave-budget",
		Nodes: []teams.ExecutionNodeInput{
			{LogicalNodeID: "main", Title: "finish", AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-main", Role: teams.ExecutionRoleMain, DependsOn: []string{"worker-a", "worker-b", "worker-c"}, MaxAttempts: 2},
			{LogicalNodeID: "worker-a", Title: "a", AgentInstanceID: "agent-a", RuntimeInstanceID: "runtime-a", Role: teams.ExecutionRoleSubAgent, MaxAttempts: 2},
			{LogicalNodeID: "worker-b", Title: "b", AgentInstanceID: "agent-b", RuntimeInstanceID: "runtime-b", Role: teams.ExecutionRoleSubAgent, MaxAttempts: 2},
			{LogicalNodeID: "worker-c", Title: "c", AgentInstanceID: "agent-c", RuntimeInstanceID: "runtime-c", Role: teams.ExecutionRoleSubAgent, MaxAttempts: 2},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	limit := teamCoordinatorWaveLimit(plan)
	if limit <= 9 || limit > 512 {
		t.Fatalf("four-node retry wave limit = %d, want bounded topology-aware budget", limit)
	}
}

func TestTeamCoordinatorRestartAuthorityEndsAfterTerminalReopen(t *testing.T) {
	generationID := "22222222-2222-4222-8222-222222222222"
	request := TeamExecutionRequest{
		RestartTerminal: true,
		CorrelationID:   generationID,
	}
	if salt := appTeamAttemptIdentitySalt(request); len(salt) != 1 || salt[0] != generationID {
		t.Fatalf("initial restart identity salt = %#v", salt)
	}
	if !restartTeamExecutionFromTerminal(
		request,
		projection.TeamExecution{Status: "blocked"},
	) {
		t.Fatal("explicit new Attempt did not authorize terminal reopen")
	}
	for _, status := range []string{"running", "awaiting_recovery"} {
		active := projection.TeamExecution{
			Status: status,
			Nodes: []projection.TeamExecutionNode{{
				Status: status, CurrentAttempt: 1,
				Attempts: []projection.TeamExecutionAttempt{{
					AttemptNumber: 1, Status: "dispatched",
				}},
			}},
		}
		if restartTeamExecutionFromTerminal(request, active) {
			t.Fatalf("restart authority leaked into active %s attempt", status)
		}
		active.Nodes[0].Attempts[0].Status = "failed"
		if !restartTeamExecutionFromTerminal(request, active) {
			t.Fatalf("durable failed %s attempt did not authorize reopen", status)
		}
	}
	request.RestartTerminal = false
	request.ExecutionGenerationID = generationID
	if salt := appTeamAttemptIdentitySalt(request); len(salt) != 1 || salt[0] != generationID {
		t.Fatalf("continuation identity salt = %#v", salt)
	}
	if restartTeamExecutionFromTerminal(
		request,
		projection.TeamExecution{Status: "blocked"},
	) {
		t.Fatal("ordinary continuation reopened a terminal Team")
	}
}

func TestTerminalTeamSuccessPublishesRememberedMainCandidate(t *testing.T) {
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-publish-terminal",
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID: "main", Title: "publish", AgentInstanceID: "agent-main",
			RuntimeInstanceID: "runtime-main", Role: teams.ExecutionRoleMain,
			MaxAttempts: 1,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	publisher := &recordingTeamWorkspacePublisher{}
	request := TeamExecutionRequest{
		Plan: plan, WorkspacePublisher: publisher,
	}
	candidates := map[string]teamWorkspaceCandidate{
		"main": {
			execution: TeamNodeExecution{
				LogicalNodeID: "main", SourcePath: "/workspace",
				SourceSnapshotDigest: strings.Repeat("1", 64),
			},
			attemptNumber: 1, workspaceDigest: strings.Repeat("2", 64),
			changes: []TeamWorkspaceChange{{
				Path: "index.html", Kind: supervisor.WorkspaceChangeAdded,
				Mode: 0o600, Digest: strings.Repeat("3", 64), Content: []byte("snake"),
			}},
		},
	}
	if err := publishTerminalTeamWorkspace(
		context.Background(), request, "succeeded", candidates,
	); err != nil {
		t.Fatal(err)
	}
	if len(publisher.publications) != 1 ||
		publisher.publications[0].LogicalNodeID != "main" ||
		len(publisher.publications[0].Changes) != 1 {
		t.Fatalf("terminal publications = %#v", publisher.publications)
	}
	if err := publishTerminalTeamWorkspace(
		context.Background(), request, "failed", candidates,
	); err != nil || len(publisher.publications) != 1 {
		t.Fatalf("failed Team publication = %#v, err=%v", publisher.publications, err)
	}
}

func TestTerminalTeamSuccessClassifiesMissingMainCandidate(t *testing.T) {
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-publish-missing-candidate",
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID: "main", Title: "publish", AgentInstanceID: "agent-main",
			RuntimeInstanceID: "runtime-main", Role: teams.ExecutionRoleMain,
			MaxAttempts: 1,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	err = publishTerminalTeamWorkspace(
		context.Background(),
		TeamExecutionRequest{Plan: plan, WorkspacePublisher: &recordingTeamWorkspacePublisher{}},
		"succeeded",
		map[string]teamWorkspaceCandidate{},
	)
	if !errors.Is(err, ErrTeamWorkspacePublish) {
		t.Fatalf("missing candidate error = %v", err)
	}
	if stage, ok := TeamWorkspacePublishStage(err); !ok || stage != "main_candidate_missing" {
		t.Fatalf("missing candidate stage = %q, %t", stage, ok)
	}
}

func TestTeamCoordinatorUsesDistinctIndependentVerifierLineage(t *testing.T) {
	fixture := newTeamRecoveryFixture(t)
	now := fixture.clock.Now()
	seedTeamCanaryRuntime(
		t,
		fixture.store,
		"runtime-verifier",
		1,
		now,
	)
	var verifierCalls atomic.Int32
	verifierExecutor := newTeamCanarySupervisor(
		t,
		fixture.work,
		fixture.grants,
		&teamCanaryAdapter{
			barrier:   &teamCanaryBarrier{release: make(chan struct{})},
			runtimeID: "runtime-verifier",
			calls:     &verifierCalls,
			outputDelta: string(
				verification.VerifierReasonCriteriaSatisfied,
			),
			contextObserver: func(ctx context.Context) error {
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) <= 20*time.Second {
					return errors.New("verifier inherited terminal commit deadline")
				}
				return nil
			},
		},
	)
	template := teamCanaryNodeExecution(
		t,
		fixture.plan,
		"main",
		1,
		"agent-verifier",
		"runtime-verifier",
		now,
		verifierExecutor,
	)
	template.Profile.Timeout = 30 * time.Second
	contract, err := verification.NewAcceptanceContract(
		1,
		[]string{"controlled output is accepted"},
		verification.AcceptanceRiskHigh,
	)
	if err != nil {
		t.Fatal(err)
	}
	fixture.request.Semantics[0].AcceptanceContract = contract
	fixture.request.Semantics[0].VerifierAgentInstanceID =
		"agent-verifier"
	fixture.request.Semantics[0].VerifierRuntimeInstanceID =
		"runtime-verifier"
	fixture.request.Semantics[0].VerifierWorkflowPath =
		"independent-verification"
	fixture.request.Semantics[0].VerifierExecution =
		&TeamVerifierExecution{
			SourcePath: template.SourcePath,
			Profile:    template.Profile,
			Instance:   template.Instance,
			Executor:   verifierExecutor,
		}
	fixture.clock.SetStep(time.Millisecond)
	result, err := fixture.coordinator.Run(
		context.Background(),
		fixture.request,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Team().Status() != "succeeded" ||
		fixture.calls.Load() != 1 ||
		verifierCalls.Load() != 1 {
		t.Fatalf(
			"result=%#v source_calls=%d verifier_calls=%d",
			result,
			fixture.calls.Load(),
			verifierCalls.Load(),
		)
	}
	if err := fixture.readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	view := fixture.readModel.GlobalReadView()
	var source projection.WorkItem
	var found bool
	for _, node := range result.Team().Nodes() {
		attempts := node.Attempts()
		if len(attempts) != 1 {
			t.Fatalf("source attempts = %d", len(attempts))
		}
		source, found = view.WorkItem(attempts[0].WorkItemID())
	}
	if !found ||
		source.Status != "done" ||
		!source.VerifierRequired ||
		source.VerifierWorkItemID == "" ||
		source.VerifierRunID == "" ||
		source.VerifierEvidenceID == "" ||
		source.VerifierWorkItemID == source.ID ||
		source.VerifierRunID == source.RunID ||
		source.VerifierAgentInstanceID == source.AgentInstanceID {
		t.Fatalf("source acceptance projection = %#v", source)
	}
	verifierRun, found := view.Run(source.VerifierRunID)
	wantVerifierBinding, err := loomruntime.FreezeExecutionBinding(
		template.Profile,
		template.Instance,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !found || !verifierRun.ExecutionBindingAvailable ||
		verifierRun.ExecutionBinding.BindingDigest !=
			wantVerifierBinding.BindingDigest ||
		verifierRun.ExecutionBinding.RuntimeInstanceID != "runtime-verifier" {
		t.Fatalf("verifier Run binding = %#v, found=%v", verifierRun, found)
	}
	before, err := fixture.store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := fixture.coordinator.Run(
		context.Background(),
		fixture.request,
	)
	if err != nil || restarted.Team().Status() != "succeeded" {
		t.Fatalf("restart = %#v, %v", restarted, err)
	}
	after, err := fixture.store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	foundContractRisk := false
	for _, event := range after {
		if event.Type != "TeamExecutionPlanned" {
			continue
		}
		if strings.Contains(
			string(event.PayloadJSON),
			`"acceptance_risk"`,
		) || !strings.Contains(
			string(event.PayloadJSON),
			`"risk":"high"`,
		) {
			t.Fatalf(
				"TeamExecutionPlanned acceptance risk schema = %s",
				event.PayloadJSON,
			)
		}
		foundContractRisk = true
	}
	if len(after) != len(before) ||
		fixture.calls.Load() != 1 ||
		verifierCalls.Load() != 1 {
		t.Fatalf(
			"restart Events=%d source_calls=%d verifier_calls=%d",
			len(after)-len(before),
			fixture.calls.Load(),
			verifierCalls.Load(),
		)
	}
	if !foundContractRisk {
		t.Fatal("missing TeamExecutionPlanned acceptance risk")
	}
}

func TestVerifierRoleScopeInstructionSeparatesNodeContribution(t *testing.T) {
	subagent, err := verifierRoleScopeInstruction(
		teams.ExecutionRoleSubAgent,
		"Produce a short implementation plan without editing files",
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"Current node role: subagent",
		"Produce a short implementation plan without editing files",
		"Do not require work explicitly assigned to another role",
	} {
		if !strings.Contains(subagent, fragment) {
			t.Fatalf("subagent verifier scope %q missing %q", subagent, fragment)
		}
	}

	if _, err := verifierRoleScopeInstruction(
		teams.ExecutionRole("unknown"), "assignment",
	); !errors.Is(err, ErrInvalidTeamCoordinator) {
		t.Fatalf("invalid verifier role error = %v", err)
	}
}

func TestTeamCoordinatorRoutesVerifierRejectionThroughBoundedRecovery(t *testing.T) {
	fixture := newTeamRecoveryFixtureWithMaxAttempts(t, 2)
	now := fixture.clock.Now()
	seedTeamCanaryRuntime(
		t,
		fixture.store,
		"runtime-verifier",
		1,
		now,
	)
	var verifierCalls atomic.Int32
	verifierExecutor := newTeamCanarySupervisor(
		t,
		fixture.work,
		fixture.grants,
		&teamCanaryAdapter{
			barrier:   &teamCanaryBarrier{release: make(chan struct{})},
			runtimeID: "runtime-verifier",
			calls:     &verifierCalls,
			outputDelta: string(
				verification.VerifierReasonCriteriaNotSatisfied,
			),
		},
	)
	template := teamCanaryNodeExecution(
		t,
		fixture.plan,
		"main",
		1,
		"agent-verifier",
		"runtime-verifier",
		now,
		verifierExecutor,
	)
	contract, err := verification.NewAcceptanceContract(
		1,
		[]string{"controlled output is accepted"},
		verification.AcceptanceRiskHigh,
	)
	if err != nil {
		t.Fatal(err)
	}
	fixture.request.Semantics[0].AcceptanceContract = contract
	fixture.request.Semantics[0].VerifierAgentInstanceID =
		"agent-verifier"
	fixture.request.Semantics[0].VerifierRuntimeInstanceID =
		"runtime-verifier"
	fixture.request.Semantics[0].VerifierWorkflowPath =
		"independent-verification"
	fixture.request.Semantics[0].VerifierExecution =
		&TeamVerifierExecution{
			SourcePath: template.SourcePath,
			Profile:    template.Profile,
			Instance:   template.Instance,
			Executor:   verifierExecutor,
		}
	fixture.clock.SetStep(time.Millisecond)
	result, err := fixture.coordinator.Run(
		context.Background(),
		fixture.request,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Team().Status() != "blocked" ||
		fixture.calls.Load() != 2 ||
		verifierCalls.Load() != 2 {
		t.Fatalf(
			"result=%#v source_calls=%d verifier_calls=%d",
			result,
			fixture.calls.Load(),
			verifierCalls.Load(),
		)
	}
	events, err := fixture.store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	counts := make(map[string]int)
	acceptanceTimes := make([]time.Time, 0, 2)
	recoveryTimes := make([]time.Time, 0, 2)
	for _, event := range events {
		counts[event.Type]++
		if event.Type == "TeamNodeAcceptanceCommitted" {
			acceptanceTimes = append(acceptanceTimes, event.EmittedAt)
		}
		if event.Type == "TeamNodeRecoveryRecorded" {
			recoveryTimes = append(recoveryTimes, event.EmittedAt)
		}
		if event.Type == "TeamNodeRecoveryRecorded" &&
			(strings.Contains(
				string(event.PayloadJSON),
				`"action":"fallback"`,
			) ||
				!strings.Contains(
					string(event.PayloadJSON),
					`"recovery_trigger":"verification_rejected"`,
				)) {
			t.Fatalf("invalid verification recovery = %s", event.PayloadJSON)
		}
	}
	if counts["WorkItemRejected"] != 2 ||
		counts["WorkItemDone"] != 0 ||
		counts["TeamNodeRecoveryRecorded"] != 2 ||
		counts["TeamExecutionTerminal"] != 1 {
		t.Fatalf("rejection Event counts = %v", counts)
	}
	if len(acceptanceTimes) != 2 || len(recoveryTimes) != 2 {
		t.Fatalf(
			"advancing recovery/acceptance times = %v/%v",
			acceptanceTimes,
			recoveryTimes,
		)
	}
	for index := range acceptanceTimes {
		if !recoveryTimes[index].After(acceptanceTimes[index]) {
			t.Fatalf(
				"recovery %d did not follow acceptance: %s/%s",
				index,
				acceptanceTimes[index],
				recoveryTimes[index],
			)
		}
	}
}

func TestTeamCoordinatorTurnsVerifierRuntimeFailureIntoGovernedRejection(t *testing.T) {
	fixture := newTeamRecoveryFixture(t)
	now := fixture.clock.Now()
	seedTeamCanaryRuntime(
		t,
		fixture.store,
		"runtime-verifier",
		1,
		now,
	)
	var verifierCalls atomic.Int32
	verifierExecutor := newTeamCanarySupervisor(
		t,
		fixture.work,
		fixture.grants,
		&teamCanaryAdapter{
			barrier:        &teamCanaryBarrier{release: make(chan struct{})},
			runtimeID:      "runtime-verifier",
			calls:          &verifierCalls,
			terminalStatus: "failed",
			terminalReason: "runtime_process_failed",
		},
	)
	template := teamCanaryNodeExecution(
		t,
		fixture.plan,
		"main",
		1,
		"agent-verifier",
		"runtime-verifier",
		now,
		verifierExecutor,
	)
	contract, err := verification.NewAcceptanceContract(
		1,
		[]string{"controlled output is accepted"},
		verification.AcceptanceRiskHigh,
	)
	if err != nil {
		t.Fatal(err)
	}
	semantics := &fixture.request.Semantics[0]
	semantics.AcceptanceContract = contract
	semantics.VerifierAgentInstanceID = "agent-verifier"
	semantics.VerifierRuntimeInstanceID = "runtime-verifier"
	semantics.VerifierWorkflowPath = "independent-verification"
	semantics.VerifierExecution = &TeamVerifierExecution{
		SourcePath: template.SourcePath,
		Profile:    template.Profile,
		Instance:   template.Instance,
		Executor:   verifierExecutor,
	}
	fixture.clock.SetStep(time.Millisecond)
	result, err := fixture.coordinator.Run(
		context.Background(),
		fixture.request,
	)
	if err != nil || result.Team().Status() != "blocked" ||
		fixture.calls.Load() != 1 || verifierCalls.Load() != 1 {
		t.Fatalf(
			"runtime failure result=%#v source_calls=%d verifier_calls=%d err=%v",
			result,
			fixture.calls.Load(),
			verifierCalls.Load(),
			err,
		)
	}
	events, err := fixture.store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	counts := make(map[string]int)
	foundGovernedReason := false
	foundTerminalCause := false
	for _, event := range events {
		counts[event.Type]++
		if event.Type == "WorkItemVerificationCommitted" &&
			strings.Contains(
				string(event.PayloadJSON),
				`"verifier_reason_code":"insufficient_evidence"`,
			) {
			foundGovernedReason = true
		}
		if event.Type == "RunTerminalCommitted" &&
			strings.Contains(
				string(event.PayloadJSON),
				`"reason":"runtime_process_failed"`,
			) {
			foundTerminalCause = true
		}
	}
	if counts["EvidenceSubmitted"] != 2 ||
		counts["WorkItemRejected"] != 1 ||
		counts["TeamNodeRecoveryRecorded"] != 1 ||
		!foundGovernedReason || !foundTerminalCause {
		t.Fatalf(
			"runtime failure governance counts=%v reason=%v cause=%v",
			counts,
			foundGovernedReason,
			foundTerminalCause,
		)
	}
}

func TestIndependentVerifierTerminalReceiptRestartsWithoutReexecution(t *testing.T) {
	fixture := newTeamRecoveryFixture(t)
	now := fixture.clock.Now()
	seedTeamCanaryRuntime(
		t,
		fixture.store,
		"runtime-verifier",
		1,
		now,
	)
	var verifierCalls atomic.Int32
	verifierExecutor := newTeamCanarySupervisor(
		t,
		fixture.work,
		fixture.grants,
		&teamCanaryAdapter{
			barrier:   &teamCanaryBarrier{release: make(chan struct{})},
			runtimeID: "runtime-verifier",
			calls:     &verifierCalls,
			outputDelta: string(
				verification.VerifierReasonCriteriaSatisfied,
			),
		},
	)
	template := teamCanaryNodeExecution(
		t,
		fixture.plan,
		"main",
		1,
		"agent-verifier",
		"runtime-verifier",
		now,
		verifierExecutor,
	)
	acceptance, err := verification.NewAcceptanceContract(
		1,
		[]string{"controlled output is accepted"},
		verification.AcceptanceRiskHigh,
	)
	if err != nil {
		t.Fatal(err)
	}
	semantics := &fixture.request.Semantics[0]
	semantics.AcceptanceContract = acceptance
	semantics.VerifierAgentInstanceID = "agent-verifier"
	semantics.VerifierRuntimeInstanceID = "runtime-verifier"
	semantics.VerifierWorkflowPath = "independent-verification"
	semantics.VerifierExecution = &TeamVerifierExecution{
		SourcePath: template.SourcePath,
		Profile:    template.Profile,
		Instance:   template.Instance,
		Executor:   verifierExecutor,
	}
	tasks := fixture.dispatchAndPrepare(t)
	outcomes := executeTeamTasks(context.Background(), tasks)
	if len(outcomes) != 1 ||
		outcomes[0].outcome.Run().TerminalStatus() != "succeeded" {
		t.Fatalf("source outcome = %#v", outcomes)
	}
	sourceTerminal := outcomes[0].outcome.Run()
	sourceReceipt, err := fixture.artifacts.FinalizeAttemptCapture(
		context.Background(),
		tasks[0].evidenceID,
		evidence.AttemptTerminal{
			Status: sourceTerminal.TerminalStatus(),
			Reason: sourceTerminal.TerminalReason(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	summary := sourceReceipt.OutputSummary()
	observation, err := verification.NewOutputObservation(
		sourceReceipt.EvidenceID(),
		sourceReceipt.Digest(),
		summary.Digest(),
		summary.AuthorizedFrameCount(),
		summary.OutputFrameCount(),
		summary.OutputPayloadBytes(),
		summary.ResultObserved(),
		summary.TerminalStatus(),
	)
	if err != nil {
		t.Fatal(err)
	}
	classification, err := verification.Classify(
		semantics.OutputContract,
		observation,
	)
	if err != nil {
		t.Fatal(err)
	}
	generation := tasks[0].generation
	if _, err := fixture.work.CommitTeamAttemptEvidence(
		context.Background(),
		work.TeamAttemptEvidenceInput{
			TeamInstanceID:    fixture.plan.TeamInstanceID(),
			PlanDigest:        fixture.plan.Digest(),
			LogicalNodeID:     "main",
			AttemptNumber:     1,
			WorkItemID:        generation.WorkItemID,
			RunID:             generation.RunID,
			ClaimID:           generation.ClaimID,
			ClaimGeneration:   generation.ClaimGeneration,
			RuntimeInstanceID: generation.RuntimeInstanceID,
			AgentInstanceID:   generation.AgentInstanceID,
			Receipt:           sourceReceipt,
			Classification:    classification,
			CorrelationID:     fixture.request.CorrelationID,
		},
	); err != nil {
		t.Fatal(err)
	}
	deterministic, err := verification.VerifyDeterministic(
		semantics.AcceptanceContract,
		verification.DeterministicVerificationInput{
			TeamInstanceID:             fixture.plan.TeamInstanceID(),
			PlanDigest:                 fixture.plan.Digest(),
			LogicalNodeID:              "main",
			AttemptNumber:              1,
			WorkItemID:                 generation.WorkItemID,
			RunID:                      generation.RunID,
			ClaimID:                    generation.ClaimID,
			ClaimGeneration:            generation.ClaimGeneration,
			SourceEvidenceID:           sourceReceipt.EvidenceID(),
			SourceEvidenceDigest:       sourceReceipt.Digest(),
			OutputSummaryDigest:        summary.Digest(),
			OutputContractVersion:      semantics.OutputContract.Version(),
			OutputContractDigest:       semantics.OutputContract.Digest(),
			OutputClassification:       classification.Kind(),
			OutputClassificationDigest: classification.Digest(),
			AcceptanceContractDigest:   semantics.AcceptanceContract.Digest(),
			TerminalStatus:             summary.TerminalStatus(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	sourceBinding := deterministic.Input()
	verifierWorkItemID := appVerifierIdentity(
		"verifier-work",
		sourceBinding.TeamInstanceID,
		sourceBinding.PlanDigest,
		sourceBinding.LogicalNodeID,
		fmt.Sprint(sourceBinding.AttemptNumber),
		sourceBinding.WorkItemID,
		sourceBinding.RunID,
		sourceBinding.SourceEvidenceDigest,
		semantics.AcceptanceContract.Digest(),
	)
	verifierRunID := appVerifierIdentity(
		"verifier-run",
		verifierWorkItemID,
		semantics.VerifierAgentInstanceID,
		semantics.VerifierRuntimeInstanceID,
		semantics.VerifierWorkflowPath,
	)
	verifierBinding, _, _, err := appFreezeVerifierExecution(*semantics)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := fixture.work.CreateAndAssign(
		context.Background(),
		work.WorkItemAssignmentInput{
			WorkItemID:       verifierWorkItemID,
			Title:            "Independent verification",
			RunID:            verifierRunID,
			AgentInstanceID:  semantics.VerifierAgentInstanceID,
			ExecutionBinding: verifierBinding,
			CorrelationID:    fixture.request.CorrelationID,
		},
	); err != nil {
		t.Fatal(err)
	}
	_, staleClaim, err := fixture.work.Claim(
		context.Background(),
		work.RunClaimInput{
			WorkItemID:           verifierWorkItemID,
			RunID:                verifierRunID,
			RuntimeInstanceID:    semantics.VerifierRuntimeInstanceID,
			AgentInstanceID:      semantics.VerifierAgentInstanceID,
			PrepareLeaseDuration: time.Minute,
			CorrelationID:        fixture.request.CorrelationID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	staleGrant, err := fixture.coordinator.issueTeamGrant(
		context.Background(),
		fixture.request,
		work.RunGenerationInput{
			WorkItemID:        verifierWorkItemID,
			RunID:             verifierRunID,
			ClaimID:           staleClaim.ClaimID(),
			ClaimGeneration:   staleClaim.ClaimGeneration(),
			RuntimeInstanceID: staleClaim.RuntimeInstanceID(),
			AgentInstanceID:   staleClaim.AgentInstanceID(),
			CorrelationID:     fixture.request.CorrelationID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	fixture.clock.Set(now.Add(2 * time.Minute))
	fixture.request.AuthoritativeTime = fixture.clock.Current()
	firstCandidate, firstReceipt, err :=
		fixture.coordinator.runIndependentVerifier(
			context.Background(),
			fixture.request,
			*semantics,
			deterministic,
			sourceReceipt,
		)
	if err != nil {
		t.Fatal(err)
	}
	if firstCandidate.Binding().ClaimGeneration != 2 ||
		firstCandidate.Binding().GrantID == staleGrant.Record().ID() {
		t.Fatalf(
			"recovered verifier generation/grant = %d/%s",
			firstCandidate.Binding().ClaimGeneration,
			firstCandidate.Binding().GrantID,
		)
	}
	for name, mutate := range map[string]func(
		verification.VerifierTerminalInput,
	) verification.VerifierTerminalInput{
		"unauthorized grant": func(
			binding verification.VerifierTerminalInput,
		) verification.VerifierTerminalInput {
			binding.GrantID = "forged-grant"
			return binding
		},
		"stale generation": func(
			binding verification.VerifierTerminalInput,
		) verification.VerifierTerminalInput {
			binding.ClaimGeneration++
			return binding
		},
	} {
		t.Run(name, func(t *testing.T) {
			forged, candidateErr :=
				verification.VerifierCandidateFromTerminal(
					mutate(firstCandidate.Binding()),
				)
			if candidateErr != nil {
				t.Fatal(candidateErr)
			}
			decision, decisionErr := verification.DecideAcceptance(
				verification.AcceptanceDecisionInput{
					Contract:            semantics.AcceptanceContract,
					DeterministicResult: deterministic,
					VerifierCandidate:   forged,
					DecisionTime:        fixture.request.AuthoritativeTime,
				},
			)
			if decisionErr != nil {
				t.Fatal(decisionErr)
			}
			_, acceptErr :=
				fixture.work.CommitTeamNodeAcceptance(
					context.Background(),
					work.TeamNodeAcceptanceInput{
						TeamInstanceID:      fixture.plan.TeamInstanceID(),
						PlanDigest:          fixture.plan.Digest(),
						LogicalNodeID:       "main",
						AttemptNumber:       1,
						SourceReceipt:       sourceReceipt,
						AcceptanceContract:  semantics.AcceptanceContract,
						DeterministicResult: deterministic,
						VerifierCandidate:   forged,
						VerifierReceipt:     firstReceipt,
						Decision:            decision,
						RecoveryPolicy:      semantics.RecoveryPolicy,
						MaxAttempts:         1,
						CreditsBefore:       0,
						CorrelationID:       fixture.request.CorrelationID,
					},
				)
			if !errors.Is(acceptErr, work.ErrVerifierLineageMismatch) {
				t.Fatalf("forged acceptance error = %v", acceptErr)
			}
		})
	}
	before, err := fixture.store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	secondCandidate, secondReceipt, err :=
		fixture.coordinator.runIndependentVerifier(
			context.Background(),
			fixture.request,
			*semantics,
			deterministic,
			sourceReceipt,
		)
	if err != nil {
		t.Fatal(err)
	}
	after, err := fixture.store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if firstCandidate.Digest() != secondCandidate.Digest() ||
		firstReceipt.Digest() != secondReceipt.Digest() ||
		verifierCalls.Load() != 1 ||
		len(after) != len(before) {
		t.Fatalf(
			"restart candidate=%s/%s receipt=%s/%s calls=%d Events=%d",
			firstCandidate.Digest(),
			secondCandidate.Digest(),
			firstReceipt.Digest(),
			secondReceipt.Digest(),
			verifierCalls.Load(),
			len(after)-len(before),
		)
	}
	oldView := fixture.readModel.GlobalReadView()
	workHead, ok := oldView.Head("work-item/" + generation.WorkItemID)
	if !ok {
		t.Fatal("missing accepted WorkItem head")
	}
	if _, err := fixture.store.Append(
		context.Background(),
		journal.Event{
			ID:             "malformed-acceptance",
			StreamID:       "work-item/" + generation.WorkItemID,
			Seq:            workHead.Sequence + 1,
			IdempotencyKey: "malformed-acceptance",
			Type:           "WorkItemDone",
			SchemaVersion:  1,
			EmittedAt:      fixture.clock.Now().Add(time.Second),
			CorrelationID:  fixture.request.CorrelationID,
			CausationID:    workHead.EventID,
			PayloadJSON:    []byte(`{}`),
		},
	); err != nil {
		t.Fatal(err)
	}
	if err := fixture.readModel.Rebuild(context.Background()); !errors.Is(err, projection.ErrInvalidProjectionEvent) {
		t.Fatalf("malformed rebuild error = %v", err)
	}
	preserved := fixture.readModel.GlobalReadView()
	preservedSource, ok := preserved.WorkItem(generation.WorkItemID)
	if !ok ||
		preserved.Version() != oldView.Version() ||
		preservedSource.Status != "ready_for_review" {
		t.Fatalf(
			"preserved view=%s/%s source=%#v",
			oldView.Version(),
			preserved.Version(),
			preservedSource,
		)
	}
}

type teamCanaryOutputObserver struct {
	mu              sync.Mutex
	barrier         *teamCanaryBarrier
	counts          map[string]int
	overlapObserved bool
}

func (observer *teamCanaryOutputObserver) ObserveNodeOutput(
	_ context.Context,
	output NodeOutput,
) error {
	if !output.Tentative() || !output.AuthorizedFrame().Tentative() {
		return errors.New("non-tentative node output")
	}
	observer.mu.Lock()
	defer observer.mu.Unlock()
	key := fmt.Sprintf(
		"%s/%d",
		output.LogicalNodeID(),
		output.AttemptNumber(),
	)
	observer.counts[key]++
	if strings.HasPrefix(output.LogicalNodeID(), "sub-") &&
		observer.barrier.active.Load() >= 2 {
		observer.overlapObserved = true
	}
	return nil
}

func TestFourProviderTeamDAGExecutionControlledCanary(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	db := openTeamCanaryDB(t)
	store := journal.NewStore(db)
	for _, runtimeID := range []string{
		"runtime-codex",
		"runtime-claude",
		"runtime-kimi",
		"runtime-minimax",
	} {
		seedTeamCanaryRuntime(t, store, runtimeID, 1, now)
	}
	clock := &teamCanaryClock{now: now}
	workAuthority, err := work.NewAuthority(
		store,
		clock.Now,
		bytes.NewReader(bytes.Repeat([]byte{0x31}, 1024)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := workAuthority.InitializeRunIdentityIndex(ctx); err != nil {
		t.Fatal(err)
	}
	grantRandom := make([]byte, 48*8)
	for index := range grantRandom {
		grantRandom[index] = 0x41 + byte(index/48)
	}
	grantAuthority, err := authorization.NewAuthority(
		store,
		workAuthority,
		clock.Now,
		bytes.NewReader(grantRandom),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := grantAuthority.InitializeGrantIdentityIndex(ctx); err != nil {
		t.Fatal(err)
	}
	readModel := projection.New(db)
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	evidenceParent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	artifactStore, err := evidence.NewStore(filepath.Join(evidenceParent, "evidence"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = artifactStore.Close() })
	coordinator, err := NewTeamCoordinator(
		workAuthority,
		grantAuthority,
		readModel,
		artifactStore,
	)
	if err != nil {
		t.Fatal(err)
	}
	testCommand, err := verification.RecognizeGovernedTestCommand("go test ./...")
	if err != nil {
		t.Fatal(err)
	}
	testReport, err := verification.NewGovernedTestReport(
		testCommand,
		verification.GovernedTestReportInput{
			CallID: "call-sub-a-tests", CallSequence: 1,
			ArgumentsDigest: strings.Repeat("7", 64),
			ExecutionID:     "execution-sub-a-tests", ExitCode: 0,
			OutputDigest: "sha256:" + strings.Repeat("8", 64), DurationMS: 42,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	testReports := &teamGovernedTestReportSourceCapture{
		reports: map[string][]verification.GovernedTestReport{
			"agent-a": {testReport},
		},
	}
	if err := coordinator.SetGovernedTestReportSource(testReports); err != nil {
		t.Fatal(err)
	}
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-canary",
		Nodes: []teams.ExecutionNodeInput{
			{
				LogicalNodeID: "main", Title: "Integrate",
				AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-codex",
				Role:      teams.ExecutionRoleMain,
				DependsOn: []string{"sub-a", "sub-b", "sub-c"}, MaxAttempts: 2,
			},
			{
				LogicalNodeID: "sub-a", Title: "Build A",
				AgentInstanceID: "agent-a", RuntimeInstanceID: "runtime-claude",
				Role: teams.ExecutionRoleSubAgent, MaxAttempts: 1,
			},
			{
				LogicalNodeID: "sub-b", Title: "Build B",
				AgentInstanceID: "agent-b", RuntimeInstanceID: "runtime-kimi",
				Role: teams.ExecutionRoleSubAgent, MaxAttempts: 1,
			},
			{
				LogicalNodeID: "sub-c", Title: "Build C",
				AgentInstanceID: "agent-c", RuntimeInstanceID: "runtime-minimax",
				Role: teams.ExecutionRoleSubAgent, MaxAttempts: 1,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	semantics := testTeamNodeSemantics(t, plan, time.Minute, "cached-source")
	barrier := &teamCanaryBarrier{release: make(chan struct{})}
	var mainDispatchMu sync.Mutex
	var mainDispatches [][]byte
	requestNodes := make([]TeamNodeExecution, 0, 5)
	for _, node := range plan.Nodes() {
		workItemID := appTeamAttemptIdentity("work", plan, node.LogicalNodeID(), 1)
		runID := appTeamAttemptIdentity("run", plan, node.LogicalNodeID(), 1)
		providerID := map[string]string{
			"main": "openai", "sub-a": "anthropic", "sub-b": "kimi",
			"sub-c": "minimax",
		}[node.LogicalNodeID()]
		modelID := map[string]string{
			"main": "gpt-5.5-codex", "sub-a": "claude-sonnet-5",
			"sub-b": "kimi-k2.6", "sub-c": "MiniMax-M3",
		}[node.LogicalNodeID()]
		adapterType := map[string]string{
			"main": "codex", "sub-a": "claude-code",
			"sub-b": "loom-native", "sub-c": "loom-native",
		}[node.LogicalNodeID()]
		credentialRevision := map[string]int64{
			"main": 3, "sub-a": 7, "sub-b": 11, "sub-c": 13,
		}[node.LogicalNodeID()]
		inputTokens := credentialRevision * 10
		outputTokens := credentialRevision
		accounting := &work.RunAccounting{
			UsageObserved:  true,
			InputTokens:    inputTokens,
			OutputTokens:   outputTokens,
			TotalTokens:    inputTokens + outputTokens,
			CostObserved:   true,
			CostMicrounits: credentialRevision * 1_000,
			CostCurrency:   "USD",
			CostSource:     work.CostSourceProviderReported,
		}
		budget := int64(100) + credentialRevision
		profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
			ID:                   "profile-" + node.LogicalNodeID(),
			AdapterType:          adapterType,
			ProviderID:           providerID,
			ProviderAccountID:    providerID + "." + node.LogicalNodeID(),
			ModelID:              modelID,
			AuthMode:             loomruntime.AuthBrokered,
			EndpointFingerprint:  strings.Repeat("d", 64),
			CredentialReference:  "credential-ref-" + node.LogicalNodeID(),
			CredentialRevision:   credentialRevision,
			RequiredCapabilities: []string{"text"},
			Timeout:              5 * time.Second,
			Budget:               &budget,
		})
		if err != nil {
			t.Fatal(err)
		}
		instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
			ID: node.RuntimeInstanceID(), DeviceID: "device-1",
			AdapterType: adapterType, DisplayName: node.RuntimeInstanceID(),
			ExecutableVersion: "1.0.0", Status: loomruntime.RuntimeOnline,
			Capacity: 1, ObservedCapabilities: []string{"text"},
		})
		if err != nil {
			t.Fatal(err)
		}
		capsule := testTeamRoleCapsule(
			t, plan, node.LogicalNodeID(), node.AgentInstanceID(), profile,
		)
		payload, err := contextcapsule.RenderDispatchPayload(capsule)
		if err != nil {
			t.Fatal(err)
		}
		dispatch, err := bridgev1.NewFrame(bridgev1.FrameInput{
			MessageID:             teamCanaryMessageID(node.LogicalNodeID()),
			CorrelationID:         "11111111-1111-4111-8111-111111111111",
			WorkItemID:            workItemID,
			RunID:                 runID,
			ClaimGeneration:       1,
			RuntimeInstanceID:     node.RuntimeInstanceID(),
			SenderAgentInstanceID: node.AgentInstanceID(),
			Sequence:              1,
			Type:                  bridgev1.MessageDispatch,
			EmittedAt:             now,
			Payload:               payload,
		})
		if err != nil {
			t.Fatal(err)
		}
		adapter := &teamCanaryAdapter{
			barrier:     barrier,
			subagent:    node.Role() == teams.ExecutionRoleSubAgent,
			adapterType: adapterType,
			runtimeID:   node.RuntimeInstanceID(),
			accounting:  accounting,
			terminalReason: func() string {
				if node.Role() == teams.ExecutionRoleMain {
					return "provider_rejected"
				}
				return ""
			}(),
			terminalStatus: func() string {
				if node.Role() == teams.ExecutionRoleMain {
					return "failed"
				}
				return "succeeded"
			}(),
		}
		if node.Role() == teams.ExecutionRoleMain {
			adapter.requestObserver = func(request supervisor.AdapterRequest) error {
				mainDispatchMu.Lock()
				defer mainDispatchMu.Unlock()
				mainDispatches = append(mainDispatches, request.Dispatch.Payload())
				return nil
			}
		}
		requestNodes = append(requestNodes, TeamNodeExecution{
			LogicalNodeID: node.LogicalNodeID(), AttemptNumber: 1,
			WorkflowPath: "primary",
			SourcePath:   t.TempDir(), Profile: profile, Instance: instance,
			Dispatch:                 dispatch,
			ContextCapsule:           capsule,
			ContextCapacityAuthority: missionContextCapacityAuthority(),
			ContextTokenCounter:      missionContextCounter,
			Executor:                 newTeamCanarySupervisor(t, workAuthority, grantAuthority, adapter),
		})
	}
	mainAttemptTwo := teamCanaryNodeExecution(
		t,
		plan,
		"main",
		2,
		"agent-main",
		"runtime-codex",
		now,
		newTeamCanarySupervisor(
			t,
			workAuthority,
			grantAuthority,
			&teamCanaryAdapter{
				barrier:        barrier,
				adapterType:    "codex",
				runtimeID:      "runtime-codex",
				terminalStatus: "succeeded",
				accounting: &work.RunAccounting{
					UsageObserved:  true,
					InputTokens:    40,
					OutputTokens:   4,
					TotalTokens:    44,
					CostObserved:   true,
					CostMicrounits: 4_000,
					CostCurrency:   "USD",
					CostSource:     work.CostSourceProviderReported,
				},
			},
		),
	)
	for _, execution := range requestNodes {
		if execution.LogicalNodeID == "main" && execution.AttemptNumber == 1 {
			mainAttemptTwo.Profile = execution.Profile
			mainAttemptTwo.Instance = execution.Instance
			mainAttemptTwo.ContextCapsule = execution.ContextCapsule
			payload, renderErr := contextcapsule.RenderDispatchPayload(
				mainAttemptTwo.ContextCapsule,
			)
			if renderErr != nil {
				t.Fatal(renderErr)
			}
			mainAttemptTwo.Dispatch = replaceTeamDispatchPayload(
				t, mainAttemptTwo.Dispatch, payload,
			)
			break
		}
	}
	mainAttemptTwo.WorkflowPath = "cached-source"
	requestNodes = append(requestNodes, mainAttemptTwo)
	outputObserver := &teamCanaryOutputObserver{
		barrier: barrier,
		counts:  make(map[string]int),
	}
	request := TeamExecutionRequest{
		Plan:                 plan,
		Nodes:                requestNodes,
		Semantics:            semantics,
		AuthoritativeTime:    now,
		PrepareLeaseDuration: time.Minute,
		GrantLifetime:        time.Minute,
		CorrelationID:        "11111111-1111-4111-8111-111111111111",
		OutputObserver:       outputObserver,
	}
	firstResult, err := coordinator.Run(ctx, request)
	if !errors.Is(err, ErrTeamExecutionIncomplete) ||
		firstResult.Team().Status() != "awaiting_recovery" &&
			firstResult.Team().Status() != "running" ||
		len(firstResult.ExecutedNodeIDs()) != 4 {
		t.Fatalf("first Run() = %#v, %v", firstResult, err)
	}
	mainDispatchMu.Lock()
	if len(mainDispatches) != 1 {
		mainDispatchMu.Unlock()
		t.Fatalf("main dispatches = %d", len(mainDispatches))
	}
	mainDispatch := append([]byte(nil), mainDispatches[0]...)
	mainDispatchMu.Unlock()
	if !bytes.Contains(mainDispatch, []byte("dependency_source_authority")) ||
		!bytes.Contains(mainDispatch, []byte("observed_execution_state")) ||
		!bytes.Contains(mainDispatch, []byte("acceptance_decision_kind")) ||
		!bytes.Contains(mainDispatch, []byte("accepted")) ||
		!bytes.Contains(mainDispatch, []byte("untrusted_model_output")) ||
		!bytes.Contains(mainDispatch, []byte(`\\\"runner\\\":\\\"go_test\\\"`)) ||
		!bytes.Contains(mainDispatch, []byte(testReport.Digest())) ||
		bytes.Contains(mainDispatch, []byte("go test ./...")) {
		t.Fatalf("main Role Capsule lacks dependency provenance: %s", mainDispatch)
	}
	testReports.mu.Lock()
	reportQueries := append([]work.AttemptReportQuery(nil), testReports.queries...)
	testReports.mu.Unlock()
	if len(reportQueries) != 3 {
		t.Fatalf("governed test report queries = %#v", reportQueries)
	}
	for _, query := range reportQueries {
		if query.TeamInstanceID != plan.TeamInstanceID() ||
			query.Authority.ConversationID != "team-conversation:"+plan.TeamInstanceID() ||
			query.Authority.WorkItemID == "" || query.Authority.RunID == "" ||
			query.Authority.ClaimID == "" || query.Authority.ClaimGeneration != 1 ||
			query.Authority.RuntimeInstanceID == "" ||
			query.Authority.ExecutionBindingDigest == "" ||
			query.Authority.CapsuleDigest == "" ||
			query.Authority.AgentInstanceID == "" || query.Authority.IncidentID == "" {
			t.Fatalf("governed test report query = %#v", query)
		}
	}
	for _, dependency := range []string{"sub-a", "sub-b", "sub-c"} {
		runID := appTeamAttemptIdentity("run", plan, dependency, 1)
		if !bytes.Contains(mainDispatch, []byte("authorized-"+runID)) {
			t.Fatalf("main Role Capsule lacks %s output: %s", dependency, mainDispatch)
		}
	}
	for _, forbidden := range [][]byte{
		[]byte("private_evidence"), []byte("credential-ref-"),
		[]byte("anthropic.sub-a"), []byte("kimi.sub-b"), []byte("minimax.sub-c"),
	} {
		if bytes.Contains(mainDispatch, forbidden) {
			t.Fatalf("main Role Capsule leaked %q: %s", forbidden, mainDispatch)
		}
	}
	wantAccounts := map[string]string{
		"main": "openai.main", "sub-a": "anthropic.sub-a",
		"sub-b": "kimi.sub-b", "sub-c": "minimax.sub-c",
	}
	wantRevisions := map[string]int64{
		"main": 3, "sub-a": 7, "sub-b": 11, "sub-c": 13,
	}
	wantHarnesses := map[string]string{
		"main": "codex", "sub-a": "claude-code",
		"sub-b": "loom-native", "sub-c": "loom-native",
	}
	wantProviders := map[string]string{
		"main": "openai", "sub-a": "anthropic",
		"sub-b": "kimi", "sub-c": "minimax",
	}
	wantModels := map[string]string{
		"main": "gpt-5.5-codex", "sub-a": "claude-sonnet-5",
		"sub-b": "kimi-k2.6", "sub-c": "MiniMax-M3",
	}
	seenDigests := make(map[string]struct{})
	for _, node := range firstResult.Team().Nodes() {
		attempts := node.Attempts()
		if len(attempts) == 0 {
			t.Fatalf("%s has no attempts", node.LogicalNodeID())
		}
		binding := attempts[0].ExecutionBinding()
		if binding.ProviderAccountID != wantAccounts[node.LogicalNodeID()] ||
			binding.CredentialRevision != wantRevisions[node.LogicalNodeID()] ||
			binding.HarnessAdapter != wantHarnesses[node.LogicalNodeID()] ||
			binding.ProviderID != wantProviders[node.LogicalNodeID()] ||
			binding.ModelID != wantModels[node.LogicalNodeID()] ||
			binding.CredentialReference != "credential-ref-"+node.LogicalNodeID() ||
			binding.Timeout != 5*time.Second ||
			binding.Budget == nil ||
			*binding.Budget != 100+wantRevisions[node.LogicalNodeID()] ||
			len(binding.Capabilities) != 1 || binding.Capabilities[0] != "text" ||
			binding.BindingDigest == "" {
			t.Fatalf("%s frozen binding = %#v", node.LogicalNodeID(), binding)
		}
		if node.LogicalNodeID() == "main" {
			if attempts[0].TerminalReason() != "provider_rejected" {
				t.Fatalf(
					"%s terminal reason = %q",
					node.LogicalNodeID(),
					attempts[0].TerminalReason(),
				)
			}
		} else if node.Status() != "succeeded" ||
			attempts[0].TerminalReason() != "" {
			t.Fatalf(
				"unaffected %s status = %q, terminal reason = %q",
				node.LogicalNodeID(),
				node.Status(),
				attempts[0].TerminalReason(),
			)
		}
		seenDigests[binding.BindingDigest] = struct{}{}
	}
	if len(seenDigests) != 4 {
		t.Fatalf("mixed Agent binding digests = %v", seenDigests)
	}
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	projectedFirstWave, ok := readModel.GlobalReadView().TeamExecution("team-canary")
	if !ok {
		t.Fatal("missing projected first-wave Team")
	}
	wantAccounting := map[string]projection.RunAccounting{
		"main": {
			UsageObserved: true, InputTokens: 30, OutputTokens: 3,
			TotalTokens: 33, CostObserved: true,
			CostMicrounits: 3_000, CostCurrency: "USD",
			CostSource: work.CostSourceProviderReported,
		},
		"sub-a": {
			UsageObserved: true, InputTokens: 70, OutputTokens: 7,
			TotalTokens: 77, CostObserved: true,
			CostMicrounits: 7_000, CostCurrency: "USD",
			CostSource: work.CostSourceProviderReported,
		},
		"sub-b": {
			UsageObserved: true, InputTokens: 110, OutputTokens: 11,
			TotalTokens: 121, CostObserved: true,
			CostMicrounits: 11_000, CostCurrency: "USD",
			CostSource: work.CostSourceProviderReported,
		},
		"sub-c": {
			UsageObserved: true, InputTokens: 130, OutputTokens: 13,
			TotalTokens: 143, CostObserved: true,
			CostMicrounits: 13_000, CostCurrency: "USD",
			CostSource: work.CostSourceProviderReported,
		},
	}
	for _, node := range projectedFirstWave.Nodes {
		if len(node.Attempts) < 1 ||
			!node.Attempts[0].ExecutionBindingAvailable ||
			node.Attempts[0].ExecutionBinding.ProviderAccountID !=
				wantAccounts[node.LogicalNodeID] ||
			!node.Attempts[0].AccountingAvailable ||
			node.Attempts[0].Accounting != wantAccounting[node.LogicalNodeID] {
			t.Fatalf("%s projected first-wave accounting = %#v", node.LogicalNodeID, node.Attempts)
		}
		if node.LogicalNodeID == "main" {
			if len(node.Attempts) != 2 || node.Attempts[1].Status != "scheduled" ||
				node.Attempts[1].ExecutionBindingAvailable ||
				node.Attempts[1].AccountingAvailable {
				t.Fatalf("main scheduled fallback inherited prior facts = %#v", node.Attempts)
			}
		} else if len(node.Attempts) != 1 {
			t.Fatalf("%s unexpected peer Attempts = %#v", node.LogicalNodeID, node.Attempts)
		}
	}
	changedRequest := request
	changedRequest.Semantics = append(
		[]TeamNodeSemantics(nil),
		request.Semantics...,
	)
	for index := range changedRequest.Semantics {
		if changedRequest.Semantics[index].LogicalNodeID != "main" {
			continue
		}
		changedPolicy, policyErr := rules.NewRecoveryPolicy(
			rules.RecoveryPolicyInput{
				Version:             2,
				RetryDelay:          time.Minute,
				AttemptCredits:      1,
				ExhaustionAction:    rules.ExhaustionBlocked,
				WorkflowFallbackKey: "cached-source",
			},
		)
		if policyErr != nil {
			t.Fatal(policyErr)
		}
		changedRequest.Semantics[index].RecoveryPolicy = changedPolicy
	}
	beforeSwap, err := store.ReadStream(
		ctx,
		"team-execution/"+plan.TeamInstanceID(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := coordinator.Run(
		ctx,
		changedRequest,
	); !errors.Is(err, work.ErrTeamExecutionConflict) {
		t.Fatalf("policy-swap restart error = %v", err)
	}
	afterSwap, err := store.ReadStream(
		ctx,
		"team-execution/"+plan.TeamInstanceID(),
	)
	if err != nil || len(afterSwap) != len(beforeSwap) {
		t.Fatalf(
			"policy-swap Team Events = %d -> %d, %v",
			len(beforeSwap),
			len(afterSwap),
			err,
		)
	}
	retryAt := now.Add(time.Minute)
	request.AuthoritativeTime = retryAt
	clock.Set(retryAt)
	result, err := coordinator.Run(ctx, request)
	if err != nil {
		t.Fatalf("recovery Run() error = %v", err)
	}
	if result.Team().Status() != "succeeded" ||
		len(result.ExecutedNodeIDs()) != 1 ||
		result.ExecutedNodeIDs()[0] != "main" {
		t.Fatalf("result = team %q nodes %v",
			result.Team().Status(), result.ExecutedNodeIDs())
	}
	if maximum := barrier.maxActive.Load(); maximum != 3 {
		t.Fatalf("maximum concurrent executions = %d, want 3", maximum)
	}
	outputObserver.mu.Lock()
	counts := make(map[string]int, len(outputObserver.counts))
	for key, count := range outputObserver.counts {
		counts[key] = count
	}
	overlapObserved := outputObserver.overlapObserved
	outputObserver.mu.Unlock()
	if !overlapObserved ||
		counts["sub-a/1"] != 3 ||
		counts["sub-b/1"] != 3 ||
		counts["sub-c/1"] != 3 ||
		counts["main/1"] != 3 ||
		counts["main/2"] != 3 {
		t.Fatalf(
			"authorized output overlap=%v counts=%v",
			overlapObserved,
			counts,
		)
	}
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	projected, ok := readModel.GlobalReadView().TeamExecution("team-canary")
	if !ok || projected.Status != "succeeded" {
		t.Fatalf("projected Team = %#v, %v", projected, ok)
	}
	for _, node := range projected.Nodes {
		if node.LogicalNodeID == "main" {
			if len(node.Attempts) != 2 ||
				!node.Attempts[0].AccountingAvailable ||
				node.Attempts[0].Accounting != wantAccounting["main"] ||
				!node.Attempts[1].ExecutionBindingAvailable ||
				node.Attempts[1].ExecutionBinding.ProviderAccountID !=
					wantAccounts["main"] ||
				!node.Attempts[1].AccountingAvailable ||
				node.Attempts[1].Accounting != (projection.RunAccounting{
					UsageObserved: true, InputTokens: 40, OutputTokens: 4,
					TotalTokens: 44, CostObserved: true,
					CostMicrounits: 4_000, CostCurrency: "USD",
					CostSource: work.CostSourceProviderReported,
				}) {
				t.Fatalf("main recovered accounting = %#v", node.Attempts)
			}
			continue
		}
		if len(node.Attempts) != 1 ||
			!node.Attempts[0].ExecutionBindingAvailable ||
			node.Attempts[0].ExecutionBinding.ProviderAccountID !=
				wantAccounts[node.LogicalNodeID] ||
			!node.Attempts[0].AccountingAvailable ||
			node.Attempts[0].Accounting != wantAccounting[node.LogicalNodeID] {
			t.Fatalf("%s accounting changed during peer recovery = %#v", node.LogicalNodeID, node.Attempts)
		}
	}
	events, err := store.ReadAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	mainAttemptOneRunID := appTeamAttemptIdentity("run", plan, "main", 1)
	accountFailureObserved := false
	for _, event := range events {
		if bytes.Contains(event.PayloadJSON, []byte("authorized-")) {
			t.Fatalf(
				"tentative output persisted in Journal Event %s/%s",
				event.StreamID,
				event.Type,
			)
		}
		if event.StreamID == "run/"+mainAttemptOneRunID &&
			event.Type == "RunTerminalCommitted" {
			var terminal struct {
				Status string `json:"status"`
				Reason string `json:"reason"`
			}
			if err := json.Unmarshal(event.PayloadJSON, &terminal); err != nil {
				t.Fatal(err)
			}
			accountFailureObserved =
				terminal.Status == "failed" &&
					terminal.Reason == "provider_rejected"
		}
	}
	if !accountFailureObserved {
		t.Fatal("main attempt 1 did not preserve Provider Account failure terminal")
	}
}

func TestTeamCoordinatorStartsHealthySiblingWithInitialProviderBlock(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 12, 2, 0, 0, 0, time.UTC)
	db := openTeamCanaryDB(t)
	store := journal.NewStore(db)
	for _, runtimeID := range []string{"runtime-healthy"} {
		seedTeamCanaryRuntime(t, store, runtimeID, 1, now)
	}
	clock := &teamCanaryClock{now: now}
	workAuthority, err := work.NewAuthority(store, clock.Now, bytes.NewReader(bytes.Repeat([]byte{0x71}, 1024)))
	if err != nil {
		t.Fatal(err)
	}
	if err := workAuthority.InitializeRunIdentityIndex(ctx); err != nil {
		t.Fatal(err)
	}
	grantAuthority, err := authorization.NewAuthority(
		store, workAuthority, clock.Now, bytes.NewReader(bytes.Repeat([]byte{0x72}, 48*8)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := grantAuthority.InitializeGrantIdentityIndex(ctx); err != nil {
		t.Fatal(err)
	}
	readModel := projection.New(db)
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	evidenceParent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := evidence.NewStore(filepath.Join(evidenceParent, "evidence"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = artifacts.Close() })
	coordinator, err := NewTeamCoordinator(workAuthority, grantAuthority, readModel, artifacts)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-initial-isolation",
		Nodes: []teams.ExecutionNodeInput{
			{LogicalNodeID: "main", Title: "Integrate", AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-main", Role: teams.ExecutionRoleMain, DependsOn: []string{"blocked", "healthy"}, MaxAttempts: 1},
			{LogicalNodeID: "blocked", Title: "Blocked", AgentInstanceID: "agent-blocked", RuntimeInstanceID: "runtime-blocked", Role: teams.ExecutionRoleSubAgent, MaxAttempts: 1},
			{LogicalNodeID: "healthy", Title: "Healthy", AgentInstanceID: "agent-healthy", RuntimeInstanceID: "runtime-healthy", Role: teams.ExecutionRoleSubAgent, MaxAttempts: 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var mainCalls, blockedCalls, healthyCalls atomic.Int32
	calls := map[string]*atomic.Int32{"main": &mainCalls, "blocked": &blockedCalls, "healthy": &healthyCalls}
	nodes := make([]TeamNodeExecution, 0, 3)
	for _, node := range plan.Nodes() {
		executor := newTeamCanarySupervisor(t, workAuthority, grantAuthority, &teamCanaryAdapter{
			barrier: &teamCanaryBarrier{release: make(chan struct{})}, runtimeID: node.RuntimeInstanceID(), calls: calls[node.LogicalNodeID()],
		})
		nodes = append(nodes, teamCanaryNodeExecution(t, plan, node.LogicalNodeID(), 1, node.AgentInstanceID(), node.RuntimeInstanceID(), now, executor))
	}
	blocks, err := teams.PropagateInitialExecutionBlocks(plan, []teams.InitialExecutionBlock{{
		LogicalNodeID: "blocked", Code: "credential_unavailable",
		Stage: "credential_lease_issue", Reason: "Credential is not verified.", Retryable: true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := coordinator.Run(ctx, TeamExecutionRequest{
		Plan: plan, Nodes: nodes, Semantics: testTeamNodeSemantics(t, plan, 0, ""),
		InitialBlocks: blocks, AuthoritativeTime: now, PrepareLeaseDuration: time.Minute,
		GrantLifetime: time.Minute, CorrelationID: "11111111-1111-4111-8111-111111111111",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Team().Status() != "blocked" || mainCalls.Load() != 0 ||
		blockedCalls.Load() != 0 || healthyCalls.Load() != 1 ||
		len(result.ExecutedNodeIDs()) != 1 || result.ExecutedNodeIDs()[0] != "healthy" {
		t.Fatalf("result=%q executed=%v calls=%d/%d/%d", result.Team().Status(), result.ExecutedNodeIDs(), mainCalls.Load(), blockedCalls.Load(), healthyCalls.Load())
	}
	status := make(map[string]string)
	for _, node := range result.Team().Nodes() {
		status[node.LogicalNodeID()] = node.Status()
		if node.LogicalNodeID() == "blocked" &&
			(node.InitialBlockCode() != "credential_unavailable" || node.InitialBlockStage() != "credential_lease_issue") {
			t.Fatalf("blocked authority = %#v", node)
		}
	}
	if status["blocked"] != "blocked" || status["main"] != "blocked" || status["healthy"] != "succeeded" {
		t.Fatalf("node status = %v", status)
	}

	allBlockedPlan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-all-initially-blocked",
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID: "main", Title: "Blocked Main",
			AgentInstanceID: "agent-all-blocked", RuntimeInstanceID: "runtime-not-observed",
			Role: teams.ExecutionRoleMain, MaxAttempts: 1,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var allBlockedCalls atomic.Int32
	allBlockedExecutor := newTeamCanarySupervisor(t, workAuthority, grantAuthority, &teamCanaryAdapter{
		barrier:   &teamCanaryBarrier{release: make(chan struct{})},
		runtimeID: "runtime-not-observed", calls: &allBlockedCalls,
	})
	allBlockedNode := teamCanaryNodeExecution(
		t, allBlockedPlan, "main", 1, "agent-all-blocked", "runtime-not-observed", now,
		allBlockedExecutor,
	)
	allBlocked, err := teams.PropagateInitialExecutionBlocks(allBlockedPlan, []teams.InitialExecutionBlock{{
		LogicalNodeID: "main", Code: "runtime_unavailable",
		Stage: "agent_attempt_dispatch", Reason: "Runtime is not online.", Retryable: true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	allBlockedResult, err := coordinator.Run(ctx, TeamExecutionRequest{
		Plan: allBlockedPlan, Nodes: []TeamNodeExecution{allBlockedNode},
		Semantics: testTeamNodeSemantics(t, allBlockedPlan, 0, ""), InitialBlocks: allBlocked,
		AuthoritativeTime: now, PrepareLeaseDuration: time.Minute,
		GrantLifetime: time.Minute, CorrelationID: "33333333-3333-4333-8333-333333333333",
	})
	if err != nil || allBlockedResult.Team().Status() != "blocked" ||
		allBlockedCalls.Load() != 0 || len(allBlockedResult.ExecutedNodeIDs()) != 0 {
		t.Fatalf("all-blocked result=%#v calls=%d err=%v", allBlockedResult, allBlockedCalls.Load(), err)
	}
	allBlockedNodes := allBlockedResult.Team().Nodes()
	if len(allBlockedNodes) != 1 || len(allBlockedNodes[0].Attempts()) != 0 ||
		allBlockedNodes[0].InitialBlockCode() != "runtime_unavailable" {
		t.Fatalf("all-blocked authority = %#v", allBlockedNodes)
	}
}

func TestTeamCoordinatorRecoversDurableAttemptWindows(t *testing.T) {
	t.Run("startup terminal before capture records governed failure", func(t *testing.T) {
		fixture := newTeamRecoveryFixtureWithMaxAttempts(t, 2)
		dispatched := fixture.dispatch(t)
		nodes := dispatched.Nodes()
		if len(nodes) != 1 {
			t.Fatalf("dispatched nodes = %d, want 1", len(nodes))
		}
		run := nodes[0].Run()
		if _, _, err := fixture.work.CommitTerminal(
			context.Background(),
			work.RunTerminalInput{
				RunGenerationInput: work.RunGenerationInput{
					WorkItemID: run.WorkItemID(), RunID: run.ID(),
					ClaimID: run.ClaimID(), ClaimGeneration: run.ClaimGeneration(),
					RuntimeInstanceID: run.RuntimeInstanceID(), AgentInstanceID: run.AgentInstanceID(),
					CorrelationID: "22222222-2222-4222-8222-222222222222",
				},
				Status: "failed", Reason: "agent_attempt_recovery_required",
			},
		); err != nil {
			t.Fatal(err)
		}
		result, err := fixture.coordinator.Run(context.Background(), fixture.request)
		if err != nil || result.Team().Status() != "blocked" || fixture.calls.Load() != 0 {
			t.Fatalf("recovery Run() = %#v, calls=%d, err=%v", result, fixture.calls.Load(), err)
		}
		attempts := result.Team().Nodes()[0].Attempts()
		if len(attempts) != 1 || attempts[0].Status() != "failed" ||
			attempts[0].TerminalReason() != "agent_attempt_recovery_required" ||
			attempts[0].EvidenceID() == "" || attempts[0].EvidenceDigest() == "" {
			t.Fatalf("recovered attempts = %#v", attempts)
		}
	})

	t.Run("terminal Run capture finalizes without duplicate execution", func(t *testing.T) {
		fixture := newTeamRecoveryFixture(t)
		tasks := fixture.dispatchAndPrepare(t)
		outcomes := executeTeamTasks(context.Background(), tasks)
		if len(outcomes) != 1 || outcomes[0].err != nil ||
			outcomes[0].outcome.Run().TerminalStatus() != "succeeded" {
			t.Fatalf("initial outcome = %#v", outcomes)
		}
		result, err := fixture.coordinator.Run(
			context.Background(),
			fixture.request,
		)
		if err != nil || result.Team().Status() != "succeeded" {
			t.Fatalf("recovery Run() = %#v, %v", result, err)
		}
		if got := fixture.calls.Load(); got != 1 {
			t.Fatalf("adapter calls = %d, want 1", got)
		}
		receipt, found, err := fixture.artifacts.AttemptReceipt(
			context.Background(),
			appTeamAttemptIdentity("evidence", fixture.plan, "main", 1),
		)
		if err != nil || !found || receipt.Digest() == "" {
			t.Fatalf("AttemptReceipt() = %#v, %v, %v", receipt, found, err)
		}
		reopened, err := fixture.coordinator.Run(
			context.Background(),
			fixture.request,
		)
		if err != nil || reopened.Team().Status() != "succeeded" ||
			fixture.calls.Load() != 1 {
			t.Fatalf("reopened Run() = %#v, %v", reopened, err)
		}
		assertTeamRecoveryExactOnce(t, fixture)
	})

	t.Run("finalized receipt commits missing metadata without execution", func(t *testing.T) {
		fixture := newTeamRecoveryFixture(t)
		tasks := fixture.dispatchAndPrepare(t)
		outcomes := executeTeamTasks(context.Background(), tasks)
		if len(outcomes) != 1 || outcomes[0].err != nil {
			t.Fatalf("initial outcome = %#v", outcomes)
		}
		receipt, err := fixture.artifacts.FinalizeAttemptCapture(
			context.Background(),
			tasks[0].evidenceID,
			evidence.AttemptTerminal{Status: "succeeded"},
		)
		if err != nil || receipt.Digest() == "" {
			t.Fatalf("FinalizeAttemptCapture() = %#v, %v", receipt, err)
		}
		result, err := fixture.coordinator.Run(
			context.Background(),
			fixture.request,
		)
		if err != nil || result.Team().Status() != "succeeded" ||
			fixture.calls.Load() != 1 {
			t.Fatalf("receipt recovery Run() = %#v, %v", result, err)
		}
		assertTeamRecoveryExactOnce(t, fixture)
	})

	t.Run("expired claim rebinds generation before one execution", func(t *testing.T) {
		fixture := newTeamRecoveryFixture(t)
		tasks := fixture.dispatchAndPrepare(t)
		oldGrantID := tasks[0].grant.Record().ID()
		fixture.clock.Set(fixture.clock.Current().Add(2 * time.Minute))
		fixture.request.AuthoritativeTime = fixture.clock.Current()
		result, err := fixture.coordinator.Run(
			context.Background(),
			fixture.request,
		)
		if err != nil || result.Team().Status() != "succeeded" {
			t.Fatalf("recovery Run() = %#v, %v", result, err)
		}
		if got := fixture.calls.Load(); got != 1 {
			t.Fatalf("adapter calls = %d, want 1", got)
		}
		attempt := result.Team().Nodes()[0].Attempts()[0]
		if attempt.ClaimGeneration() != 2 ||
			attempt.ClaimID() == tasks[0].generation.ClaimID {
			t.Fatalf("rebound attempt = %#v", attempt)
		}
		capture, found, err := fixture.artifacts.AttemptCapture(
			context.Background(),
			appTeamAttemptIdentity("evidence", fixture.plan, "main", 1),
		)
		if err != nil || !found ||
			capture.Binding().ClaimGeneration != 2 ||
			capture.Binding().ClaimID != attempt.ClaimID() {
			t.Fatalf("AttemptCapture() = %#v, %v, %v", capture, found, err)
		}
		if err := fixture.readModel.Rebuild(context.Background()); err != nil {
			t.Fatal(err)
		}
		oldGrant, ok := fixture.readModel.GlobalReadView().AgentGrant(oldGrantID)
		if !ok || oldGrant.RevocationReason != string(authorization.RevocationOperator) {
			t.Fatalf("old Grant = %#v, %v", oldGrant, ok)
		}
	})

	t.Run("running phase requires human recovery without re-execution", func(t *testing.T) {
		fixture := newTeamRecoveryFixture(t)
		tasks := fixture.dispatchAndPrepare(t)
		if _, _, err := fixture.work.Start(
			context.Background(),
			tasks[0].generation,
		); err != nil {
			t.Fatal(err)
		}
		_, err := fixture.coordinator.Run(
			context.Background(),
			fixture.request,
		)
		if !errors.Is(err, work.ErrTeamAttemptRecoveryRequired) {
			t.Fatalf("Run() error = %v", err)
		}
		if got := fixture.calls.Load(); got != 0 {
			t.Fatalf("adapter calls = %d, want 0", got)
		}
	})

	t.Run("missing capture rejects later same-generation Run activity", func(t *testing.T) {
		fixture := newTeamRecoveryFixture(t)
		dispatched := fixture.dispatch(t)
		run := dispatched.Nodes()[0].Run()
		if _, err := fixture.work.ExtendPrepareLease(
			context.Background(),
			work.RunGenerationInput{
				WorkItemID:        run.WorkItemID(),
				RunID:             run.ID(),
				ClaimID:           run.ClaimID(),
				ClaimGeneration:   run.ClaimGeneration(),
				RuntimeInstanceID: run.RuntimeInstanceID(),
				AgentInstanceID:   run.AgentInstanceID(),
				CorrelationID:     fixture.request.CorrelationID,
			},
			2*time.Minute,
		); err != nil {
			t.Fatal(err)
		}
		_, err := fixture.coordinator.Run(
			context.Background(),
			fixture.request,
		)
		if !errors.Is(err, work.ErrTeamAttemptRecoveryRequired) {
			t.Fatalf("Run() error = %v", err)
		}
		evidenceID := appTeamAttemptIdentity(
			"evidence",
			fixture.plan,
			"main",
			1,
		)
		if _, found, err := fixture.artifacts.AttemptCapture(
			context.Background(),
			evidenceID,
		); err != nil || found {
			t.Fatalf("AttemptCapture() found=%v error=%v", found, err)
		}
	})

	t.Run("divergent capture conflicts before lease expiry", func(t *testing.T) {
		fixture := newTeamRecoveryFixture(t)
		dispatched := fixture.dispatch(t)
		attempt := dispatched.Nodes()[0].Attempt()
		evidenceID := appTeamAttemptIdentity(
			"evidence",
			fixture.plan,
			"main",
			1,
		)
		if err := fixture.artifacts.BeginAttemptCapture(
			context.Background(),
			evidence.AttemptCaptureInput{
				EvidenceID:        evidenceID,
				TeamInstanceID:    fixture.plan.TeamInstanceID(),
				PlanDigest:        fixture.plan.Digest(),
				LogicalNodeID:     "main",
				AttemptNumber:     1,
				WorkItemID:        attempt.WorkItemID(),
				RunID:             attempt.RunID(),
				ClaimID:           attempt.ClaimID(),
				ClaimGeneration:   attempt.ClaimGeneration(),
				RuntimeInstanceID: attempt.RuntimeInstanceID(),
				AgentInstanceID:   "agent-divergent",
			},
		); err != nil {
			t.Fatal(err)
		}
		_, err := fixture.coordinator.Run(
			context.Background(),
			fixture.request,
		)
		if !errors.Is(err, work.ErrTeamAttemptRecoveryRequired) {
			t.Fatalf("Run() error = %v", err)
		}
		if got := fixture.calls.Load(); got != 0 {
			t.Fatalf("adapter calls = %d, want 0", got)
		}
	})

	t.Run("immediate post-dispatch missing capture repairs only capture", func(t *testing.T) {
		fixture := newTeamRecoveryFixture(t)
		dispatched := fixture.dispatch(t)
		attempt := dispatched.Nodes()[0].Attempt()
		_, err := fixture.coordinator.Run(
			context.Background(),
			fixture.request,
		)
		if !errors.Is(err, ErrTeamExecutionIncomplete) {
			t.Fatalf("Run() error = %v", err)
		}
		capture, found, err := fixture.artifacts.AttemptCapture(
			context.Background(),
			appTeamAttemptIdentity("evidence", fixture.plan, "main", 1),
		)
		if err != nil || !found ||
			capture.Binding().ClaimID != attempt.ClaimID() ||
			capture.Binding().ClaimGeneration != 1 ||
			fixture.calls.Load() != 0 {
			t.Fatalf("capture repair = %#v, %v, %v", capture, found, err)
		}
	})

	for _, testCase := range []struct {
		name          string
		rebindCapture bool
	}{
		{name: "after Run reclaim before capture rebind"},
		{name: "after capture rebind before Team rebound", rebindCapture: true},
	} {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newTeamRecoveryFixture(t)
			tasks := fixture.dispatchAndPrepare(t)
			fixture.clock.Set(fixture.clock.Current().Add(2 * time.Minute))
			fixture.request.AuthoritativeTime = fixture.clock.Current()
			if _, err := fixture.grants.Revoke(
				context.Background(),
				authorization.RevokeInput{
					GrantID:       tasks[0].grant.Record().ID(),
					Reason:        authorization.RevocationOperator,
					CorrelationID: fixture.request.CorrelationID,
				},
			); err != nil {
				t.Fatal(err)
			}
			_, reclaimed, err := fixture.work.Claim(
				context.Background(),
				work.RunClaimInput{
					WorkItemID:           tasks[0].generation.WorkItemID,
					RunID:                tasks[0].generation.RunID,
					RuntimeInstanceID:    tasks[0].generation.RuntimeInstanceID,
					AgentInstanceID:      tasks[0].generation.AgentInstanceID,
					PrepareLeaseDuration: fixture.request.PrepareLeaseDuration,
					CorrelationID:        fixture.request.CorrelationID,
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			if testCase.rebindCapture {
				binding, found, err := fixture.artifacts.AttemptCapture(
					context.Background(),
					tasks[0].evidenceID,
				)
				if err != nil || !found {
					t.Fatalf("AttemptCapture() = %#v, %v, %v", binding, found, err)
				}
				next := binding.Binding()
				next.ClaimID = reclaimed.ClaimID()
				next.ClaimGeneration = reclaimed.ClaimGeneration()
				if err := fixture.artifacts.RebindAttemptCapture(
					context.Background(),
					next,
				); err != nil {
					t.Fatal(err)
				}
			}
			result, err := fixture.coordinator.Run(
				context.Background(),
				fixture.request,
			)
			if err != nil || result.Team().Status() != "succeeded" ||
				fixture.calls.Load() != 1 {
				t.Fatalf("split recovery Run() = %#v, %v", result, err)
			}
			attempt := result.Team().Nodes()[0].Attempts()[0]
			if attempt.ClaimID() != reclaimed.ClaimID() ||
				attempt.ClaimGeneration() != 2 {
				t.Fatalf("split rebound attempt = %#v", attempt)
			}
		})
	}
}

func TestTeamCoordinatorAttemptsEveryTerminalCommitAfterEarlierFailure(t *testing.T) {
	fixture := newTeamRecoveryFixture(t)
	tasks := fixture.dispatchAndPrepare(t)
	outcomes := executeTeamTasks(context.Background(), tasks)
	if len(outcomes) != 1 || outcomes[0].err != nil ||
		outcomes[0].outcome.Run().TerminalStatus() != "succeeded" {
		t.Fatalf("initial outcome = %#v", outcomes)
	}
	broken := outcomes[0]
	broken.task.evidenceID = "missing-attempt-capture"
	err := fixture.coordinator.commitTeamTaskOutcomes(
		context.Background(),
		fixture.request,
		[]teamTaskOutcome{broken, outcomes[0]},
	)
	if !errors.Is(err, evidence.ErrAttemptCaptureIncomplete) {
		t.Fatalf("aggregated terminal commit error = %v", err)
	}
	receipt, found, receiptErr := fixture.artifacts.AttemptReceipt(
		context.Background(),
		tasks[0].evidenceID,
	)
	if receiptErr != nil || !found || receipt.Digest() == "" {
		t.Fatalf("later attempt receipt = %#v, %v, %v", receipt, found, receiptErr)
	}
	events, readErr := fixture.store.ReadAll(context.Background())
	if readErr != nil {
		t.Fatal(readErr)
	}
	counts := make(map[string]int)
	for _, event := range events {
		counts[event.Type]++
	}
	if counts["EvidenceSubmitted"] != 1 ||
		counts["TeamNodeAttemptTerminal"] != 1 ||
		counts["TeamExecutionTerminal"] != 1 {
		t.Fatalf("later terminal lineage counts = %v", counts)
	}
}

func TestTeamCoordinatorClassifiesAllowedAndTransientEmptyOutput(t *testing.T) {
	t.Run("contract-valid empty succeeds", func(t *testing.T) {
		fixture := newTeamRecoveryFixture(t)
		fixture.request.Nodes[0].Executor = newTeamCanarySupervisor(
			t,
			fixture.work,
			fixture.grants,
			&teamCanaryAdapter{
				barrier:    &teamCanaryBarrier{release: make(chan struct{})},
				runtimeID:  "runtime-recovery",
				calls:      &fixture.calls,
				omitOutput: true,
			},
		)
		contract, err := verification.NewOutputContract(
			1,
			verification.EmptyOutputValid,
		)
		if err != nil {
			t.Fatal(err)
		}
		fixture.request.Semantics[0].OutputContract = contract
		result, err := fixture.coordinator.Run(
			context.Background(),
			fixture.request,
		)
		if err != nil || result.Team().Status() != "succeeded" ||
			fixture.calls.Load() != 1 {
			t.Fatalf("allowed-empty Run() = %#v, %v", result, err)
		}
		attempt := result.Team().Nodes()[0].Attempts()[0]
		if attempt.OutputClassification() != verification.OutputValidEmpty {
			t.Fatalf("allowed-empty classification = %#v", attempt)
		}
	})

	t.Run("transient empty retries only when due", func(t *testing.T) {
		fixture := newTeamRecoveryFixtureWithMaxAttempts(t, 2)
		for index := range fixture.request.Nodes {
			fixture.request.Nodes[index].Executor = newTeamCanarySupervisor(
				t,
				fixture.work,
				fixture.grants,
				&teamCanaryAdapter{
					barrier:    &teamCanaryBarrier{release: make(chan struct{})},
					runtimeID:  "runtime-recovery",
					calls:      &fixture.calls,
					omitOutput: index == 0,
				},
			)
		}
		contract, err := verification.NewOutputContract(
			1,
			verification.EmptyOutputTransient,
		)
		if err != nil {
			t.Fatal(err)
		}
		policy, err := rules.NewRecoveryPolicy(rules.RecoveryPolicyInput{
			Version:          1,
			RetryDelay:       time.Minute,
			AttemptCredits:   1,
			ExhaustionAction: rules.ExhaustionBlocked,
		})
		if err != nil {
			t.Fatal(err)
		}
		fixture.request.Semantics[0].OutputContract = contract
		fixture.request.Semantics[0].RecoveryPolicy = policy
		first, err := fixture.coordinator.Run(
			context.Background(),
			fixture.request,
		)
		if !errors.Is(err, ErrTeamExecutionIncomplete) ||
			fixture.calls.Load() != 1 ||
			len(first.ExecutedNodeIDs()) != 1 {
			t.Fatalf("transient first Run() = %#v, %v", first, err)
		}
		retryAt := fixture.clock.Current().Add(time.Minute)
		fixture.clock.Set(retryAt)
		fixture.request.AuthoritativeTime = retryAt
		second, err := fixture.coordinator.Run(
			context.Background(),
			fixture.request,
		)
		if err != nil || second.Team().Status() != "succeeded" ||
			fixture.calls.Load() != 2 {
			t.Fatalf("transient retry Run() = %#v, %v", second, err)
		}
		attempts := second.Team().Nodes()[0].Attempts()
		if len(attempts) != 2 ||
			attempts[0].OutputClassification() !=
				verification.OutputTransientEmpty ||
			attempts[1].OutputClassification() !=
				verification.OutputValidNonEmpty {
			t.Fatalf("transient attempts = %#v", attempts)
		}
	})
}

func TestTeamCoordinatorRecoveryApprovalFailsClosedToHumanRequired(t *testing.T) {
	fixture := newTeamRecoveryFixture(t)
	fixture.request.Nodes[0].Executor = newTeamCanarySupervisor(
		t,
		fixture.work,
		fixture.grants,
		&teamCanaryAdapter{
			barrier:        &teamCanaryBarrier{release: make(chan struct{})},
			runtimeID:      "runtime-recovery",
			calls:          &fixture.calls,
			terminalStatus: "failed",
		},
	)
	policy, err := rules.NewRecoveryPolicy(rules.RecoveryPolicyInput{
		Version:                  1,
		AttemptCredits:           0,
		ExhaustionAction:         rules.ExhaustionBlocked,
		RecoveryApprovalRequired: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	fixture.request.Semantics[0].RecoveryPolicy = policy
	result, err := fixture.coordinator.Run(
		context.Background(),
		fixture.request,
	)
	if err != nil || result.Team().Status() != "human_required" ||
		result.Team().Nodes()[0].Status() != "human_required" ||
		fixture.calls.Load() != 1 {
		t.Fatalf("approval-required Run() = %#v, %v", result, err)
	}
}

type teamRecoveryFixture struct {
	clock       *teamCanaryClock
	db          *sql.DB
	store       *journal.Store
	work        *work.Authority
	grants      *authorization.Authority
	readModel   *projection.Projection
	artifacts   *evidence.Store
	coordinator *TeamCoordinator
	plan        teams.ExecutionPlan
	request     TeamExecutionRequest
	calls       atomic.Int32
}

type teamProjectionFreshnessObserver struct {
	readModel          *projection.Projection
	teamInstanceID     string
	expectedGeneration int64
	seen               atomic.Int32
}

type teamRejectingOutputObserver struct {
	seen atomic.Int32
}

func (observer *teamRejectingOutputObserver) ObserveNodeOutput(
	_ context.Context,
	output NodeOutput,
) error {
	if output.AuthorizedFrame().Frame().Type() == bridgev1.MessageEvent {
		observer.seen.Add(1)
		return errors.New("tentative Mission timeline unavailable")
	}
	return nil
}

func (observer *teamProjectionFreshnessObserver) ObserveNodeOutput(
	_ context.Context,
	output NodeOutput,
) error {
	execution, ok := observer.readModel.GlobalReadView().TeamExecution(
		observer.teamInstanceID,
	)
	if !ok {
		return errors.New("fresh Team execution missing")
	}
	binding := output.AuthorizedFrame().Binding()
	for _, node := range execution.Nodes {
		if node.LogicalNodeID != output.LogicalNodeID() {
			continue
		}
		for _, attempt := range node.Attempts {
			if attempt.AttemptNumber == output.AttemptNumber() &&
				attempt.WorkItemID == binding.WorkItemID &&
				attempt.RunID == binding.RunID &&
				attempt.ClaimGeneration == observer.expectedGeneration &&
				attempt.ClaimGeneration == binding.ClaimGeneration &&
				attempt.RuntimeInstanceID == binding.RuntimeInstanceID &&
				attempt.AgentInstanceID == binding.SenderAgentInstanceID {
				if output.AuthorizedFrame().Frame().Type() ==
					bridgev1.MessageEvent {
					observer.seen.Add(1)
				}
				return nil
			}
		}
	}
	return errors.New("fresh exact attempt generation missing")
}

func TestTeamCoordinatorRefreshesProjectionBeforeAuthorizedObservation(t *testing.T) {
	t.Run("first dispatch", func(t *testing.T) {
		fixture := newTeamRecoveryFixture(t)
		observer := &teamProjectionFreshnessObserver{
			readModel:          fixture.readModel,
			teamInstanceID:     fixture.plan.TeamInstanceID(),
			expectedGeneration: 1,
		}
		fixture.request.OutputObserver = observer
		result, err := fixture.coordinator.Run(
			context.Background(),
			fixture.request,
		)
		if err != nil || result.Team().Status() != "succeeded" {
			t.Fatalf("Run() = %#v, %v", result, err)
		}
		if got := observer.seen.Load(); got != 1 {
			t.Fatalf("observed authorized event Frames = %d, want 1", got)
		}
	})

	t.Run("generation rebound", func(t *testing.T) {
		fixture := newTeamRecoveryFixture(t)
		fixture.dispatchAndPrepare(t)
		observer := &teamProjectionFreshnessObserver{
			readModel:          fixture.readModel,
			teamInstanceID:     fixture.plan.TeamInstanceID(),
			expectedGeneration: 2,
		}
		fixture.request.OutputObserver = observer
		fixture.clock.Set(fixture.clock.Current().Add(2 * time.Minute))
		fixture.request.AuthoritativeTime = fixture.clock.Current()
		result, err := fixture.coordinator.Run(
			context.Background(),
			fixture.request,
		)
		if err != nil || result.Team().Status() != "succeeded" {
			t.Fatalf("rebound Run() = %#v, %v", result, err)
		}
		if got := observer.seen.Load(); got != 1 {
			t.Fatalf("rebound observed event Frames = %d, want 1", got)
		}
	})

	t.Run("refresh failure stops execution", func(t *testing.T) {
		fixture := newTeamRecoveryFixture(t)
		if _, err := fixture.db.ExecContext(context.Background(), `
			CREATE TRIGGER corrupt_projection_after_team_dispatch
			AFTER INSERT ON events
			WHEN NEW.event_type = 'TeamReadySetDispatched'
			BEGIN
				INSERT INTO events (
					id, stream_id, seq, idempotency_key, event_type,
					schema_version, emitted_at, correlation_id,
					causation_id, payload_json
				)
				VALUES (
					'event-corrupt-refresh', 'mode/corrupt-refresh', 1,
					'corrupt-refresh', 'ModeSelected', 1,
					NEW.emitted_at, NEW.correlation_id, NEW.id, '{}'
				);
			END
		`); err != nil {
			t.Fatal(err)
		}
		_, err := fixture.coordinator.Run(
			context.Background(),
			fixture.request,
		)
		if !errors.Is(err, projection.ErrInvalidProjectionEvent) {
			t.Fatalf("Run() error = %v", err)
		}
		if got := fixture.calls.Load(); got != 0 {
			t.Fatalf("adapter calls = %d, want 0", got)
		}
	})
}

func TestTeamCoordinatorDoesNotLetTentativeOutputObserverRejectResult(t *testing.T) {
	fixture := newTeamRecoveryFixture(t)
	observer := &teamRejectingOutputObserver{}
	fixture.request.OutputObserver = observer

	result, err := fixture.coordinator.Run(context.Background(), fixture.request)
	if err != nil || result.Team().Status() != "succeeded" ||
		fixture.calls.Load() != 1 {
		t.Fatalf("Run() = %#v, %v", result, err)
	}
	if observer.seen.Load() != 1 {
		t.Fatalf("tentative output observations = %d, want 1", observer.seen.Load())
	}
	assertTeamRecoveryExactOnce(t, fixture)
}

func TestTeamCoordinatorRebuildsDependencyCapsuleAfterStaleDispatchView(t *testing.T) {
	fixture := newTeamRecoveryFixture(t)
	now := fixture.clock.Current()
	seedTeamCanaryRuntime(t, fixture.store, "runtime-dependency-sub", 1, now)
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: fixture.plan.TeamInstanceID(),
		Nodes: []teams.ExecutionNodeInput{
			{
				LogicalNodeID: "main", Title: "Integrate accepted dependency",
				AgentInstanceID: "agent-recovery", RuntimeInstanceID: "runtime-recovery",
				Role: teams.ExecutionRoleMain, DependsOn: []string{"sub"}, MaxAttempts: 1,
			},
			{
				LogicalNodeID: "sub", Title: "Inspect source",
				AgentInstanceID:   "agent-dependency-sub",
				RuntimeInstanceID: "runtime-dependency-sub",
				Role:              teams.ExecutionRoleSubAgent, MaxAttempts: 1,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	mainCalls := &atomic.Int32{}
	subCalls := &atomic.Int32{}
	mainExecution := teamCanaryNodeExecution(
		t, plan, "main", 1, "agent-recovery", "runtime-recovery", now,
		newTeamCanarySupervisor(t, fixture.work, fixture.grants, &teamCanaryAdapter{
			barrier:   &teamCanaryBarrier{release: make(chan struct{})},
			runtimeID: "runtime-recovery", calls: mainCalls,
		}),
	)
	subExecution := teamCanaryNodeExecution(
		t, plan, "sub", 1, "agent-dependency-sub", "runtime-dependency-sub", now,
		newTeamCanarySupervisor(t, fixture.work, fixture.grants, &teamCanaryAdapter{
			barrier:   &teamCanaryBarrier{release: make(chan struct{})},
			runtimeID: "runtime-dependency-sub", calls: subCalls,
		}),
	)
	contextStore := &staleOnceTeamCapsuleStore{store: fixture.store, now: now}
	request := TeamExecutionRequest{
		Plan: plan, Nodes: []TeamNodeExecution{mainExecution, subExecution},
		Semantics:         testTeamNodeSemantics(t, plan, 0, ""),
		AuthoritativeTime: now, PrepareLeaseDuration: time.Minute,
		GrantLifetime:   time.Minute,
		CorrelationID:   "11111111-1111-4111-8111-111111111111",
		ContextCapsules: contextStore,
	}

	result, err := fixture.coordinator.Run(context.Background(), request)
	if err != nil || result.Team().Status() != "succeeded" {
		t.Fatalf("Run() = %#v, %v", result, err)
	}
	if mainCalls.Load() != 1 || subCalls.Load() != 1 {
		t.Fatalf("adapter calls main=%d sub=%d", mainCalls.Load(), subCalls.Load())
	}
	digests := contextStore.capturedDigests()
	if len(digests) != 2 || digests[0] == "" || digests[0] != digests[1] {
		t.Fatalf("dependency Capsule retry digests = %#v", digests)
	}
}

func newTeamRecoveryFixture(t testing.TB) *teamRecoveryFixture {
	return newTeamRecoveryFixtureWithMaxAttempts(t, 1)
}

func TestTeamCoordinatorRestartTerminalMaterializesAuthoritativeSaltedRun(t *testing.T) {
	fixture := newTeamRecoveryFixture(t)
	assetAuthority, err := assets.NewAuthority(assets.AuthorityConfig{
		Store: fixture.store,
		Now:   fixture.clock.Now,
		ViewVersion: func() string {
			return strings.Repeat("a", 64)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	binding := seedRestartMaterializationAsset(t, assetAuthority)
	setDigest, err := assets.CanonicalAssetRevisionSetDigest(
		[]assets.ExactAssetRevisionBinding{binding},
	)
	if err != nil {
		t.Fatal(err)
	}
	assetPlan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: fixture.plan.TeamInstanceID(),
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID:          fixture.plan.Nodes()[0].LogicalNodeID(),
			Title:                  fixture.plan.Nodes()[0].Title(),
			AgentInstanceID:        fixture.plan.Nodes()[0].AgentInstanceID(),
			RuntimeInstanceID:      fixture.plan.Nodes()[0].RuntimeInstanceID(),
			Role:                   teams.ExecutionRoleMain,
			MaxAttempts:            1,
			AssetRevisionBindings:  []assets.ExactAssetRevisionBinding{binding},
			AssetRevisionSetDigest: setDigest,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	initialTime := fixture.clock.Current()
	initialExecution := teamCanaryNodeExecution(
		t,
		assetPlan,
		"main",
		1,
		"agent-recovery",
		"runtime-recovery",
		initialTime,
		fixture.request.Nodes[0].Executor,
	)
	initialExecution = teamExecutionWithWorkspaceSnapshot(t, assetPlan, initialExecution)
	materializer := &restartAssetMaterializer{
		authority:  assetAuthority,
		sourcePath: initialExecution.SourcePath,
	}
	if err := fixture.coordinator.SetAssetMaterializer(materializer); err != nil {
		t.Fatal(err)
	}
	initialRequest := fixture.request
	initialRequest.Plan = assetPlan
	initialRequest.Nodes = []TeamNodeExecution{initialExecution}
	initialRequest.Semantics = testTeamNodeSemantics(t, assetPlan, 0, "")
	first, err := fixture.coordinator.Run(context.Background(), initialRequest)
	if err != nil || first.Team().Status() != "succeeded" {
		t.Fatalf("initial terminal Run() = %#v, %v", first, err)
	}
	priorRunID := first.Team().Nodes()[0].Attempts()[0].RunID()

	now := fixture.clock.Current().Add(time.Minute)
	fixture.clock.Set(now)
	restartExecution := teamCanaryNodeExecution(
		t,
		assetPlan,
		"main",
		1,
		"agent-recovery",
		"runtime-recovery",
		now,
		fixture.request.Nodes[0].Executor,
	)
	restartCorrelationID := "22222222-2222-4222-8222-222222222222"
	restartExecution = teamExecutionWithRestartIdentity(
		t, assetPlan, restartCorrelationID, restartExecution,
	)
	restartExecution = teamExecutionWithWorkspaceSnapshot(t, assetPlan, restartExecution)
	restartRequest := fixture.request
	restartRequest.Plan = assetPlan
	restartRequest.Nodes = []TeamNodeExecution{restartExecution}
	restartRequest.Semantics = testTeamNodeSemantics(t, assetPlan, 0, "")
	restartRequest.AuthoritativeTime = now
	restartRequest.CorrelationID = restartCorrelationID
	restartRequest.RestartTerminal = true
	executions, err := validateTeamExecutionRequest(context.Background(), restartRequest)
	if err != nil {
		t.Fatalf("restart request validation: %v", err)
	}
	if err := fixture.readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	view := fixture.readModel.GlobalReadView()
	node := assetPlan.Nodes()[0]
	execution := executions[appExecutionKey("main", 1)]
	selection := work.TeamAttemptSelection{
		LogicalNodeID: "main", AttemptNumber: 1,
		ExecutionBinding: execution.executionBinding,
		ContextCapsule:   execution.ContextCapsule,
	}
	_, materializations, cleanup, err := fixture.coordinator.prepareTeamAssetMaterializations(
		context.Background(), restartRequest, view,
		[]teams.ExecutionNode{node}, []work.TeamAttemptSelection{selection},
		[]TeamNodeExecution{execution},
	)
	if err != nil {
		t.Fatalf("restart materialization: %v", err)
	}
	dispatched, err := fixture.work.DispatchTeamReadySet(
		context.Background(),
		work.TeamDispatchInput{
			Plan: assetPlan, ReadyAttempts: []work.TeamAttemptSelection{selection},
			RouteSummaries:   appInitialDispatchRoutes(restartRequest, view),
			SemanticBindings: appSemanticBindings(restartRequest),
			Materializations: materializations,
			ViewVersion:      view.Version(),
			ExpectedHeads: appDispatchHeadsWithSources(
				view, assetPlan, []work.TeamAttemptSelection{selection},
				[]teams.ExecutionNode{node}, nil, materializations,
				appTeamAttemptIdentitySalt(restartRequest)...,
			),
			AuthoritativeTime: now, PrepareLeaseDuration: time.Minute,
			CorrelationID: restartCorrelationID, RestartTerminal: true,
		},
	)
	if err != nil {
		fixture.coordinator.cleanupTeamAssetMaterializations(context.Background(), cleanup)
		t.Fatalf("restart terminal dispatch: %v", err)
	}
	if len(materializer.requests) != 2 {
		t.Fatalf("materialization requests = %d, want 2", len(materializer.requests))
	}
	authoritativeRunID := dispatched.Nodes()[0].Attempt().RunID()
	if got := materializer.requests[1].RunID; got != authoritativeRunID {
		t.Fatalf("materialization RunID = %q, authoritative RunID = %q", got, authoritativeRunID)
	}
	if authoritativeRunID == priorRunID {
		t.Fatalf("restarted RunID reused prior lineage %q", priorRunID)
	}
}

func teamExecutionWithRestartIdentity(
	t testing.TB,
	plan teams.ExecutionPlan,
	correlationID string,
	execution TeamNodeExecution,
) TeamNodeExecution {
	t.Helper()
	dispatch, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID: execution.Dispatch.MessageID(), CorrelationID: correlationID,
		WorkItemID: appTeamAttemptIdentity(
			"work", plan, execution.LogicalNodeID, execution.AttemptNumber, correlationID,
		),
		RunID: appTeamAttemptIdentity(
			"run", plan, execution.LogicalNodeID, execution.AttemptNumber, correlationID,
		),
		ClaimGeneration:       execution.Dispatch.ClaimGeneration(),
		RuntimeInstanceID:     execution.Dispatch.RuntimeInstanceID(),
		SenderAgentInstanceID: execution.Dispatch.SenderAgentInstanceID(),
		Sequence:              execution.Dispatch.Sequence(), Type: execution.Dispatch.Type(),
		EmittedAt: execution.Dispatch.EmittedAt(), Payload: execution.Dispatch.Payload(),
	})
	if err != nil {
		t.Fatal(err)
	}
	execution.Dispatch = dispatch
	return execution
}

func teamExecutionWithWorkspaceSnapshot(
	t testing.TB,
	plan teams.ExecutionPlan,
	execution TeamNodeExecution,
) TeamNodeExecution {
	t.Helper()
	snapshot, err := supervisor.ObserveSourceSnapshot(execution.SourcePath)
	if err != nil {
		t.Fatal(err)
	}
	content, err := json.Marshal(struct {
		SchemaVersion  int    `json:"schema_version"`
		SnapshotKind   string `json:"snapshot_kind"`
		TreeDigest     string `json:"tree_digest"`
		EntryCount     int    `json:"entry_count"`
		FileCount      int    `json:"file_count"`
		DirectoryCount int    `json:"directory_count"`
		TotalBytes     int64  `json:"total_bytes"`
	}{
		SchemaVersion: 1, SnapshotKind: "managed_source_baseline",
		TreeDigest: snapshot.TreeDigest(), EntryCount: snapshot.EntryCount(),
		FileCount: snapshot.FileCount(), DirectoryCount: snapshot.DirectoryCount(),
		TotalBytes: snapshot.TotalBytes(),
	})
	if err != nil {
		t.Fatal(err)
	}
	capsule, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(
		contextcapsule.Target{
			ConversationID: "team-conversation:" + plan.TeamInstanceID(),
			TeamID:         plan.TeamInstanceID(), AgentID: appPlanNode(plan, execution.LogicalNodeID).AgentInstanceID(),
			RoleID: execution.LogicalNodeID, ProviderID: execution.Profile.ProviderID,
			ProviderAccountID: execution.Profile.ProviderAccountID,
			ModelID:           execution.Profile.ModelID, AuthMode: string(execution.Profile.AuthMode),
			ContextAdapterID:   "context:" + execution.Profile.AdapterType + ":v1",
			DisclosurePolicyID: "policy.test", DisclosurePolicyVersion: 1,
			TokenBudget: 2048,
		},
		[]contextcapsule.ItemInput{
			{
				ItemID: "goal-1", Kind: contextcapsule.KindConversationGoal,
				Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeTeamShared,
				Priority: contextcapsule.PrioritySystem, Required: true,
				Content:    []byte("Execute the controlled canary."),
				SourceType: contextcapsule.SourceAuthority, SourceRef: "team-plan:" + plan.Digest(),
			},
			{
				ItemID: "workspace-snapshot", Kind: contextcapsule.KindWorkspaceSnapshot,
				Trust: contextcapsule.TrustObserved, Scope: contextcapsule.ScopeTeamShared,
				Priority: contextcapsule.PrioritySystem, Required: true,
				Content: content, SourceType: contextcapsule.SourceObservation,
				SourceRef: "managed-source:" + snapshot.TreeDigest(),
			},
		},
		missionContextCapacityAuthority(),
		missionContextCounter,
	)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := contextcapsule.RenderDispatchPayload(capsule)
	if err != nil {
		t.Fatal(err)
	}
	execution.ContextCapsule = capsule
	execution.ContextCapacityAuthority = missionContextCapacityAuthority()
	execution.ContextTokenCounter = missionContextCounter
	execution.Dispatch = replaceTeamDispatchPayload(t, execution.Dispatch, payload)
	execution.SourceSnapshotDigest = snapshot.TreeDigest()
	return execution
}

func seedRestartMaterializationAsset(
	t testing.TB,
	authority *assets.Authority,
) assets.ExactAssetRevisionBinding {
	t.Helper()
	digest := strings.Repeat("a", 64)
	command := assets.Command{
		OperationID:            "create-restart-asset",
		JourneyID:              "33333333-3333-4333-8333-333333333333",
		ExpectedViewVersion:    digest,
		DecisionSource:         "user_explicit",
		AssetKind:              assets.AssetKindSkill,
		DefinitionID:           "skill-restart",
		RevisionID:             "revision-restart",
		CandidateID:            "candidate-restart",
		Name:                   "Restart materialization",
		Description:            "Regression fixture",
		Scope:                  "project",
		ArtifactDigest:         digest,
		ContentDigest:          digest,
		SourceScope:            assets.SourceScopeLocal,
		SourceReferenceDigest:  digest,
		ProvenanceDigest:       digest,
		Risk:                   assets.RiskLow,
		CompatibleCapabilities: []string{"loom.skill-materialization.pi.v1"},
	}
	if _, err := authority.CreateSkill(context.Background(), command); err != nil {
		t.Fatal(err)
	}
	command.OperationID = "activate-restart-asset"
	if _, err := authority.ActivateCandidate(context.Background(), command); err != nil {
		t.Fatal(err)
	}
	return assets.ExactAssetRevisionBinding{
		AssetKind:    assets.AssetKindSkill,
		DefinitionID: command.DefinitionID,
		RevisionID:   command.RevisionID,
		SHA256Digest: digest,
		SourceScope:  assets.SourceScopeLocal,
	}
}

func TestPhase2DTeamExecutionRejectsSilentRetryBindingChange(t *testing.T) {
	fixture := newTeamRecoveryFixtureWithMaxAttempts(t, 2)
	fixture.request.Nodes[1].Profile.ProviderID = "anthropic"
	fixture.request.Nodes[1].Profile.ProviderAccountID = "anthropic.fallback"
	fixture.request.Nodes[1].Profile.ModelID = "claude-sonnet"
	fixture.request.Nodes[1].Profile.CredentialReference =
		"credential-ref-anthropic-fallback"
	fixture.request.Nodes[1].Profile.CredentialRevision = 2

	_, err := validateTeamExecutionRequest(context.Background(), fixture.request)
	if !errors.Is(err, ErrInvalidTeamCoordinator) {
		t.Fatalf("silent retry binding change error = %v", err)
	}
}

func TestPhase2DTeamExecutionRejectsContextDispatchDrift(t *testing.T) {
	fixture := newTeamRecoveryFixture(t)
	execution := &fixture.request.Nodes[0]
	var wire map[string]any
	if err := json.Unmarshal(execution.Dispatch.Payload(), &wire); err != nil {
		t.Fatal(err)
	}
	wire["prompt"] = "raw objective that was not admitted by the Role Context Capsule"
	tampered, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	execution.Dispatch = replaceTeamDispatchPayload(t, execution.Dispatch, tampered)

	_, err = validateTeamExecutionRequest(context.Background(), fixture.request)
	if !errors.Is(err, ErrInvalidTeamCoordinator) {
		t.Fatalf("context dispatch drift error = %v", err)
	}
}

func TestPhase2DTeamExecutionRejectsUnapprovedFallbackBindingChange(t *testing.T) {
	fixture := newTeamRecoveryFixtureWithMaxAttempts(t, 2)
	fixture.request.Nodes[1].WorkflowPath = "unapproved-provider-fallback"
	fixture.request.Nodes[1].Profile.ProviderID = "anthropic"
	fixture.request.Nodes[1].Profile.ProviderAccountID = "anthropic.unapproved"
	fixture.request.Nodes[1].Profile.ModelID = "claude-sonnet"
	fixture.request.Nodes[1].Profile.CredentialReference =
		"credential-ref-anthropic-unapproved"
	fixture.request.Nodes[1].Profile.CredentialRevision = 2
	policy, err := rules.NewRecoveryPolicy(rules.RecoveryPolicyInput{
		Version:             2,
		AttemptCredits:      1,
		ExhaustionAction:    rules.ExhaustionBlocked,
		RetryInvalid:        true,
		WorkflowFallbackKey: "unapproved-provider-fallback",
	})
	if err != nil {
		t.Fatal(err)
	}
	fixture.request.Semantics[0].RecoveryPolicy = policy

	_, err = validateTeamExecutionRequest(context.Background(), fixture.request)
	if !errors.Is(err, ErrInvalidTeamCoordinator) {
		t.Fatalf("unapproved fallback binding change error = %v", err)
	}
}

func TestPhase2DTeamExecutionProfileFromFrozenBindingPreservesEnrollment(t *testing.T) {
	// appRuntimeProfileFromFrozenBinding must round-trip the remote-tool
	// Enrollment fields. validateTeamExecutionRequest replaces each node's
	// Profile with this conversion; dropping the fields made the supervisor
	// recompute a legacy binding digest while the Route Segment / journal
	// binding carried the v3 digest, so every enrollment-bound Mission start
	// failed with ErrInvalidManagedExecution (surfaced as state_unavailable).
	binding, err := loomruntime.FreezeExecutionBinding(
		loomruntime.RuntimeProfile{
			ID: "profile.web", AdapterType: "loom-native",
			ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
			ModelID: "deepseek-chat", AuthMode: loomruntime.AuthBrokered,
			EndpointFingerprint: strings.Repeat("b", 64),
			CredentialReference: "credential-ref-deepseek-primary",
			CredentialRevision:  2,
			RequiredCapabilities: []string{
				loomruntime.CapabilityContextRetrieval,
			},
			Timeout:                    45 * time.Second,
			RemoteToolEnrollmentID:     "enroll-web-live-001",
			RemoteToolEnrollmentDigest: strings.Repeat("c", 64),
		},
		loomruntime.RuntimeInstance{
			ID: "runtime.loom-native.local", DeviceID: "device.local",
			AdapterType: "loom-native", DisplayName: "Loom Native",
			Status: loomruntime.RuntimeOnline,
			ObservedCapabilities: []string{
				loomruntime.CapabilityContextRetrieval,
			},
			Capacity: 2,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	profile := appRuntimeProfileFromFrozenBinding(binding)
	if profile.RemoteToolEnrollmentID != binding.RemoteToolEnrollmentID ||
		profile.RemoteToolEnrollmentDigest != binding.RemoteToolEnrollmentDigest {
		t.Fatalf("profile dropped enrollment: %#v", profile)
	}
	// The profile must re-freeze to the exact same binding digest so the
	// Route Segment, journal binding and supervisor recomputation all agree.
	refrozen, err := loomruntime.FreezeExecutionBinding(
		profile,
		loomruntime.RuntimeInstance{
			ID: binding.RuntimeInstanceID, DeviceID: "device.local",
			AdapterType: binding.HarnessAdapter, DisplayName: "Loom Native",
			Status: loomruntime.RuntimeOnline,
			ObservedCapabilities: append(
				[]string(nil), binding.Capabilities...,
			),
			Capacity: 2,
		},
	)
	if err != nil || refrozen.BindingDigest != binding.BindingDigest {
		t.Fatalf("refrozen digest = %s err=%v want %s",
			refrozen.BindingDigest, err, binding.BindingDigest)
	}
}

func TestPhase2DTeamExecutionUsesOnlyExactApprovedFallbackBinding(t *testing.T) {
	fixture := newTeamRecoveryFixtureWithMaxAttempts(t, 2)
	fixture.request.Nodes[0].Executor = newTeamCanarySupervisor(
		t,
		fixture.work,
		fixture.grants,
		&teamCanaryAdapter{
			barrier:        &teamCanaryBarrier{release: make(chan struct{})},
			runtimeID:      "runtime-recovery",
			calls:          &fixture.calls,
			terminalStatus: "failed",
			terminalReason: "provider_rejected",
		},
	)
	fixture.request.Nodes[1].WorkflowPath = "approved-provider-fallback"
	fixture.request.Nodes[1].Profile.ID = "profile-approved-fallback"
	fixture.request.Nodes[1].Profile.ProviderID = "anthropic"
	fixture.request.Nodes[1].Profile.ProviderAccountID = "anthropic.team-backup"
	fixture.request.Nodes[1].Profile.ModelID = "claude-sonnet"
	fixture.request.Nodes[1].Profile.CredentialReference =
		"credential-ref-anthropic-team-backup"
	fixture.request.Nodes[1].Profile.CredentialRevision = 4
	fixture.request.Nodes[1].ContextCapsule = testTeamRoleCapsule(
		t, fixture.plan, "main", "agent-recovery",
		fixture.request.Nodes[1].Profile,
	)
	fallbackPayload, err := contextcapsule.RenderDispatchPayload(
		fixture.request.Nodes[1].ContextCapsule,
	)
	if err != nil {
		t.Fatal(err)
	}
	fixture.request.Nodes[1].Dispatch = replaceTeamDispatchPayload(
		t, fixture.request.Nodes[1].Dispatch, fallbackPayload,
	)

	policy, err := rules.NewRecoveryPolicy(rules.RecoveryPolicyInput{
		Version:             2,
		AttemptCredits:      1,
		ExhaustionAction:    rules.ExhaustionBlocked,
		RetryInvalid:        true,
		WorkflowFallbackKey: "approved-provider-fallback",
	})
	if err != nil {
		t.Fatal(err)
	}
	source := teamCanaryExecutionBinding(t, fixture.request.Nodes[0])
	target := teamCanaryExecutionBinding(t, fixture.request.Nodes[1])
	approval, err := work.NewTeamFallbackApproval(work.TeamFallbackApprovalInput{
		Version:             1,
		ApprovalID:          "fallback-approval-team-recovery-main-v1",
		ActorRef:            "user:local-owner",
		ApprovedAt:          fixture.request.AuthoritativeTime,
		SourceBindingDigest: source.BindingDigest,
		TargetBindingDigest: target.BindingDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	fixture.request.Semantics[0].RecoveryPolicy = policy
	fixture.request.Semantics[0].FallbackApproval = approval

	result, err := fixture.coordinator.Run(context.Background(), fixture.request)
	if err != nil || result.Team().Status() != "succeeded" ||
		fixture.calls.Load() != 2 {
		t.Fatalf("approved fallback result = %#v, calls=%d, %v", result, fixture.calls.Load(), err)
	}
	attempts := result.Team().Nodes()[0].Attempts()
	if len(attempts) != 2 ||
		attempts[0].ExecutionBinding().BindingDigest != source.BindingDigest ||
		attempts[1].ExecutionBinding().BindingDigest != target.BindingDigest ||
		attempts[1].ExecutionBinding().ProviderAccountID !=
			"anthropic.team-backup" ||
		attempts[0].ContextCapsuleDigest() == attempts[1].ContextCapsuleDigest() ||
		attempts[1].ContextCapsuleAuthority().ProviderAccountID !=
			"anthropic.team-backup" {
		t.Fatalf("approved fallback attempts = %#v", attempts)
	}
	events, err := fixture.store.ReadStream(
		context.Background(),
		"team-execution/"+fixture.plan.TeamInstanceID(),
	)
	if err != nil {
		t.Fatal(err)
	}
	var audited bool
	for _, event := range events {
		if event.Type == "TeamNodeRecoveryRecorded" &&
			strings.Contains(string(event.PayloadJSON),
				`"fallback_approval_digest":"`+approval.Digest()+`"`) &&
			strings.Contains(string(event.PayloadJSON),
				`"fallback_target_binding_digest":"`+target.BindingDigest+`"`) {
			audited = true
		}
	}
	if !audited {
		t.Fatal("approved fallback was not linked from the recovery fact")
	}
}

func TestPhase2DApprovedFallbackUsesExactIndependentVaultCredential(t *testing.T) {
	fixture := newTeamRecoveryFixtureWithMaxAttempts(t, 2)
	fixture.request.Nodes[1].WorkflowPath = "approved-vault-fallback"
	fixture.request.Nodes[1].Profile.ID = "profile-approved-vault-fallback"
	fixture.request.Nodes[1].Profile.ProviderID = "anthropic"
	fixture.request.Nodes[1].Profile.ProviderAccountID = "anthropic.team-backup"
	fixture.request.Nodes[1].Profile.ModelID = "claude-sonnet-5"
	fixture.request.Nodes[1].Profile.CredentialReference =
		"credential-ref-anthropic-team-backup"
	fixture.request.Nodes[1].Profile.CredentialRevision = 4
	fixture.request.Nodes[1].ContextCapsule = testTeamRoleCapsule(
		t, fixture.plan, "main", "agent-recovery",
		fixture.request.Nodes[1].Profile,
	)
	fallbackPayload, err := contextcapsule.RenderDispatchPayload(
		fixture.request.Nodes[1].ContextCapsule,
	)
	if err != nil {
		t.Fatal(err)
	}
	fixture.request.Nodes[1].Dispatch = replaceTeamDispatchPayload(
		t, fixture.request.Nodes[1].Dispatch, fallbackPayload,
	)

	source := teamCanaryExecutionBinding(t, fixture.request.Nodes[0])
	target := teamCanaryExecutionBinding(t, fixture.request.Nodes[1])
	policy, err := rules.NewRecoveryPolicy(rules.RecoveryPolicyInput{
		Version:             2,
		AttemptCredits:      1,
		ExhaustionAction:    rules.ExhaustionBlocked,
		RetryInvalid:        true,
		WorkflowFallbackKey: "approved-vault-fallback",
	})
	if err != nil {
		t.Fatal(err)
	}
	approval, err := work.NewTeamFallbackApproval(work.TeamFallbackApprovalInput{
		Version:             1,
		ApprovalID:          "fallback-approval-vault-identity-v1",
		ActorRef:            "user:local-owner",
		ApprovedAt:          fixture.request.AuthoritativeTime,
		SourceBindingDigest: source.BindingDigest,
		TargetBindingDigest: target.BindingDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	fixture.request.Semantics[0].RecoveryPolicy = policy
	fixture.request.Semantics[0].FallbackApproval = approval

	root := t.TempDir()
	privateDirectory := filepath.Join(root, "private")
	stateDirectory := filepath.Join(root, "state")
	for _, directory := range []string{privateDirectory, stateDirectory} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	keyPath := filepath.Join(privateDirectory, "vault.key")
	databasePath := filepath.Join(stateDirectory, "credential-vault.db")
	material, err := (credentialvault.LocalKeyFile{Path: keyPath}).LoadOrCreate(
		context.Background(),
	)
	if err != nil {
		t.Fatal(err)
	}
	vaultStore, err := credentialvault.OpenStore(credentialvault.StoreConfig{
		DatabasePath: databasePath, KeyMaterial: material,
	})
	if err != nil {
		t.Fatal(err)
	}
	identities := []credentialvault.CredentialIdentity{
		teamCanaryCredentialIdentity(source),
		teamCanaryCredentialIdentity(target),
	}
	secretMarkers := [][]byte{
		[]byte("vault-primary-secret-must-not-escape"),
		[]byte("vault-fallback-secret-must-not-escape"),
	}
	defer func() {
		for _, marker := range secretMarkers {
			clearTeamCanaryBytes(marker)
		}
	}()
	for index, identity := range identities {
		secret := append([]byte(nil), secretMarkers[index]...)
		if err := vaultStore.PutCredential(
			context.Background(), identity, secret,
		); err != nil {
			clearTeamCanaryBytes(secret)
			t.Fatal(err)
		}
		clearTeamCanaryBytes(secret)
	}
	if err := vaultStore.Close(); err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("sqlite", "file:"+databasePath)
	if err != nil {
		t.Fatal(err)
	}
	corrupted, err := database.Exec(
		`UPDATE encrypted_credentials
		    SET ciphertext = zeroblob(length(ciphertext))
		  WHERE credential_reference = ? AND provider_id = ?
		    AND provider_account_id = ? AND credential_revision = ?`,
		identities[0].CredentialReference, identities[0].ProviderID,
		identities[0].ProviderAccountID, identities[0].CredentialRevision,
	)
	if err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if rows, rowsErr := corrupted.RowsAffected(); rowsErr != nil || rows != 1 {
		_ = database.Close()
		t.Fatalf("corrupted rows = %d, %v", rows, rowsErr)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	material, err = (credentialvault.LocalKeyFile{Path: keyPath}).LoadOrCreate(
		context.Background(),
	)
	if err != nil {
		t.Fatal(err)
	}
	vaultStore, err = credentialvault.OpenStore(credentialvault.StoreConfig{
		DatabasePath: databasePath, KeyMaterial: material,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer vaultStore.Close()
	leasing, err := credentialvault.NewCredentialLeaseManager(vaultStore)
	if err != nil {
		t.Fatal(err)
	}
	defer leasing.Close()

	var observedMu sync.Mutex
	observed := make([]credentialvault.CredentialIdentity, 0, 2)
	stages := make([]string, 0, 1)
	credentialUse := func(
		ctx context.Context,
		binding loomruntime.FrozenExecutionBinding,
		_ func(context.Context, []byte) error,
	) error {
		identity := teamCanaryCredentialIdentity(binding)
		observedMu.Lock()
		observed = append(observed, identity)
		observedMu.Unlock()
		lease, acquireErr := leasing.Acquire(ctx, identity, time.Minute)
		if acquireErr != nil {
			observedMu.Lock()
			stages = append(stages, credentials.CredentialFailureStage(acquireErr))
			observedMu.Unlock()
			return acquireErr
		}
		defer lease.Close()
		return lease.WithSecret(func(_ context.Context, secret []byte) error {
			if identity != identities[1] || !bytes.Equal(secret, secretMarkers[1]) {
				return errors.New("fallback credential identity drift")
			}
			return nil
		})
	}
	adapter := &teamCanaryAdapter{
		barrier:       &teamCanaryBarrier{release: make(chan struct{})},
		runtimeID:     "runtime-recovery",
		calls:         &fixture.calls,
		credentialUse: credentialUse,
	}
	executor := newTeamCanarySupervisor(t, fixture.work, fixture.grants, adapter)
	for index := range fixture.request.Nodes {
		fixture.request.Nodes[index].Executor = executor
	}

	teamResult, err := fixture.coordinator.Run(context.Background(), fixture.request)
	if err != nil || teamResult.Team().Status() != "succeeded" ||
		fixture.calls.Load() != 2 {
		t.Fatalf("Vault fallback result = %#v, calls=%d, %v", teamResult, fixture.calls.Load(), err)
	}
	attempts := teamResult.Team().Nodes()[0].Attempts()
	if len(attempts) != 2 ||
		attempts[0].ExecutionBinding().BindingDigest != source.BindingDigest ||
		attempts[0].TerminalReason() != "credential_unavailable" ||
		attempts[1].ExecutionBinding().BindingDigest != target.BindingDigest ||
		attempts[1].ExecutionBinding().ProviderAccountID !=
			"anthropic.team-backup" ||
		attempts[1].ExecutionBinding().CredentialRevision != 4 ||
		attempts[1].TerminalReason() != "" {
		t.Fatalf("Vault fallback Attempts = %#v", attempts)
	}
	observedMu.Lock()
	defer observedMu.Unlock()
	if !reflect.DeepEqual(observed, identities) ||
		!reflect.DeepEqual(stages, []string{credentials.CredentialStageVaultDecrypt}) {
		t.Fatalf("Vault identities=%#v stages=%#v", observed, stages)
	}
	events, err := fixture.store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		for _, marker := range secretMarkers {
			if bytes.Contains(event.PayloadJSON, marker) {
				t.Fatalf("credential secret reached Journal event %s", event.Type)
			}
		}
	}
}

func teamCanaryCredentialIdentity(
	binding loomruntime.FrozenExecutionBinding,
) credentialvault.CredentialIdentity {
	return credentialvault.CredentialIdentity{
		ProviderID:          binding.ProviderID,
		ProviderAccountID:   binding.ProviderAccountID,
		CredentialReference: binding.CredentialReference,
		CredentialRevision:  binding.CredentialRevision,
	}
}

func TestPhase2DTeamExecutionRejectsForgedFallbackApprovalTarget(t *testing.T) {
	fixture := newTeamRecoveryFixtureWithMaxAttempts(t, 2)
	fixture.request.Nodes[1].WorkflowPath = "approved-provider-fallback"
	fixture.request.Nodes[1].Profile.ProviderAccountID = "openai.backup"
	fixture.request.Nodes[1].Profile.CredentialReference = "credential-ref-openai-backup"
	fixture.request.Nodes[1].Profile.CredentialRevision = 2
	policy, err := rules.NewRecoveryPolicy(rules.RecoveryPolicyInput{
		Version:             2,
		AttemptCredits:      1,
		ExhaustionAction:    rules.ExhaustionBlocked,
		RetryInvalid:        true,
		WorkflowFallbackKey: "approved-provider-fallback",
	})
	if err != nil {
		t.Fatal(err)
	}
	source := teamCanaryExecutionBinding(t, fixture.request.Nodes[0])
	approval, err := work.NewTeamFallbackApproval(work.TeamFallbackApprovalInput{
		Version:             1,
		ApprovalID:          "fallback-approval-forged-target-v1",
		ActorRef:            "user:local-owner",
		ApprovedAt:          fixture.request.AuthoritativeTime,
		SourceBindingDigest: source.BindingDigest,
		TargetBindingDigest: strings.Repeat("f", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	fixture.request.Semantics[0].RecoveryPolicy = policy
	fixture.request.Semantics[0].FallbackApproval = approval

	_, err = validateTeamExecutionRequest(context.Background(), fixture.request)
	if !errors.Is(err, ErrInvalidTeamCoordinator) {
		t.Fatalf("forged fallback target error = %v", err)
	}
}

func TestPhase2DTeamExecutionRejectsFutureDatedFallbackApproval(t *testing.T) {
	fixture := newTeamRecoveryFixtureWithMaxAttempts(t, 2)
	fixture.request.Nodes[1].WorkflowPath = "future-provider-fallback"
	fixture.request.Nodes[1].Profile.ProviderAccountID = "openai.future-backup"
	fixture.request.Nodes[1].Profile.CredentialReference =
		"credential-ref-openai-future-backup"
	fixture.request.Nodes[1].Profile.CredentialRevision = 2
	policy, err := rules.NewRecoveryPolicy(rules.RecoveryPolicyInput{
		Version:             2,
		AttemptCredits:      1,
		ExhaustionAction:    rules.ExhaustionBlocked,
		RetryInvalid:        true,
		WorkflowFallbackKey: "future-provider-fallback",
	})
	if err != nil {
		t.Fatal(err)
	}
	source := teamCanaryExecutionBinding(t, fixture.request.Nodes[0])
	target := teamCanaryExecutionBinding(t, fixture.request.Nodes[1])
	approval, err := work.NewTeamFallbackApproval(work.TeamFallbackApprovalInput{
		Version:             1,
		ApprovalID:          "fallback-approval-future-v1",
		ActorRef:            "user:local-owner",
		ApprovedAt:          fixture.request.AuthoritativeTime.Add(time.Minute),
		SourceBindingDigest: source.BindingDigest,
		TargetBindingDigest: target.BindingDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	fixture.request.Semantics[0].RecoveryPolicy = policy
	fixture.request.Semantics[0].FallbackApproval = approval

	_, err = validateTeamExecutionRequest(context.Background(), fixture.request)
	if !errors.Is(err, ErrInvalidTeamCoordinator) {
		t.Fatalf("future fallback approval error = %v", err)
	}
}

func newTeamRecoveryFixtureWithMaxAttempts(
	t testing.TB,
	maxAttempts int,
) *teamRecoveryFixture {
	t.Helper()
	ctx := context.Background()
	now := time.Date(2026, 7, 26, 15, 0, 0, 0, time.UTC)
	db := openTeamCanaryDB(t)
	store := journal.NewStore(db)
	seedTeamCanaryRuntime(t, store, "runtime-recovery", 1, now)
	clock := &teamCanaryClock{now: now}
	workRandom := make([]byte, 1024)
	for index := range workRandom {
		workRandom[index] = 0x51 + byte(index/16)
	}
	workAuthority, err := work.NewAuthority(
		store,
		clock.Now,
		bytes.NewReader(workRandom),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := workAuthority.InitializeRunIdentityIndex(ctx); err != nil {
		t.Fatal(err)
	}
	grantRandom := make([]byte, 48*8)
	for index := range grantRandom {
		grantRandom[index] = 0x61 + byte(index/48)
	}
	grantAuthority, err := authorization.NewAuthority(
		store,
		workAuthority,
		clock.Now,
		bytes.NewReader(grantRandom),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := grantAuthority.InitializeGrantIdentityIndex(ctx); err != nil {
		t.Fatal(err)
	}
	readModel := projection.New(db)
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	evidenceParent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := evidence.NewStore(filepath.Join(evidenceParent, "evidence"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = artifacts.Close() })
	coordinator, err := NewTeamCoordinator(
		workAuthority,
		grantAuthority,
		readModel,
		artifacts,
	)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-recovery",
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID:     "main",
			Title:             "Recover",
			AgentInstanceID:   "agent-recovery",
			RuntimeInstanceID: "runtime-recovery",
			Role:              teams.ExecutionRoleMain,
			MaxAttempts:       maxAttempts,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	fixture := &teamRecoveryFixture{
		clock:       clock,
		db:          db,
		store:       store,
		work:        workAuthority,
		grants:      grantAuthority,
		readModel:   readModel,
		artifacts:   artifacts,
		coordinator: coordinator,
		plan:        plan,
	}
	executor := newTeamCanarySupervisor(
		t,
		workAuthority,
		grantAuthority,
		&teamCanaryAdapter{
			barrier:   &teamCanaryBarrier{release: make(chan struct{})},
			runtimeID: "runtime-recovery",
			calls:     &fixture.calls,
		},
	)
	executions := make([]TeamNodeExecution, 0, maxAttempts)
	for attemptNumber := 1; attemptNumber <= maxAttempts; attemptNumber++ {
		executions = append(executions, teamCanaryNodeExecution(
			t,
			plan,
			"main",
			attemptNumber,
			"agent-recovery",
			"runtime-recovery",
			now,
			executor,
		))
	}
	fixture.request = TeamExecutionRequest{
		Plan:                 plan,
		Nodes:                executions,
		Semantics:            testTeamNodeSemantics(t, plan, 0, ""),
		AuthoritativeTime:    now,
		PrepareLeaseDuration: time.Minute,
		GrantLifetime:        time.Minute,
		CorrelationID:        "11111111-1111-4111-8111-111111111111",
	}
	return fixture
}

func assertTeamRecoveryExactOnce(
	t testing.TB,
	fixture *teamRecoveryFixture,
) {
	t.Helper()
	events, err := fixture.store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	counts := make(map[string]int)
	for _, event := range events {
		switch event.Type {
		case "EvidenceSubmitted",
			"TeamNodeAttemptTerminal",
			"TeamExecutionTerminal":
			counts[event.Type]++
		}
	}
	if counts["EvidenceSubmitted"] != 1 ||
		counts["TeamNodeAttemptTerminal"] != 1 ||
		counts["TeamExecutionTerminal"] != 1 {
		t.Fatalf("terminal Event counts = %v", counts)
	}
}

func (fixture *teamRecoveryFixture) dispatchAndPrepare(
	t testing.TB,
) []teamExecutionTask {
	t.Helper()
	ctx := context.Background()
	dispatched := fixture.dispatch(t)
	normalized, err := validateTeamExecutionRequest(ctx, fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	executions := make([]TeamNodeExecution, 0, len(fixture.request.Nodes))
	for _, node := range dispatched.Nodes() {
		execution, ok := normalized[appExecutionKey(
			node.LogicalNode().LogicalNodeID(),
			node.Attempt().AttemptNumber(),
		)]
		if !ok {
			t.Fatalf("normalized execution missing for %s", node.LogicalNode().LogicalNodeID())
		}
		executions = append(executions, execution)
	}
	tasks, err := fixture.coordinator.prepareTeamTasks(
		ctx,
		fixture.request,
		dispatched,
		executions,
	)
	if err != nil {
		t.Fatal(err)
	}
	return tasks
}

func (fixture *teamRecoveryFixture) dispatch(
	t testing.TB,
) work.TeamDispatchResult {
	t.Helper()
	ctx := context.Background()
	if err := fixture.readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	view := fixture.readModel.GlobalReadView()
	node := fixture.plan.Nodes()[0]
	selections := []work.TeamAttemptSelection{{
		LogicalNodeID: "main", AttemptNumber: 1,
		ExecutionBinding: teamCanaryExecutionBinding(
			t, fixture.request.Nodes[0],
		),
		ContextCapsule: fixture.request.Nodes[0].ContextCapsule,
	}}
	dispatched, err := fixture.work.DispatchTeamReadySet(
		ctx,
		work.TeamDispatchInput{
			Plan:          fixture.plan,
			ReadyAttempts: selections,
			SemanticBindings: appSemanticBindings(
				fixture.request,
			),
			ViewVersion: view.Version(),
			ExpectedHeads: appDispatchHeads(
				view,
				fixture.plan,
				selections,
				[]teams.ExecutionNode{node},
				nil,
			),
			AuthoritativeTime:    fixture.request.AuthoritativeTime,
			PrepareLeaseDuration: fixture.request.PrepareLeaseDuration,
			CorrelationID:        fixture.request.CorrelationID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return dispatched
}

func teamCanaryExecutionBinding(
	t testing.TB,
	execution TeamNodeExecution,
) loomruntime.FrozenExecutionBinding {
	t.Helper()
	binding, err := loomruntime.FreezeExecutionBinding(
		execution.Profile,
		execution.Instance,
	)
	if err != nil {
		t.Fatal(err)
	}
	return binding
}

func teamCanaryNodeExecution(
	t testing.TB,
	plan teams.ExecutionPlan,
	logicalNodeID string,
	attemptNumber int,
	agentInstanceID string,
	runtimeInstanceID string,
	now time.Time,
	executor ManagedNodeExecutor,
) TeamNodeExecution {
	t.Helper()
	workItemID := appTeamAttemptIdentity(
		"work",
		plan,
		logicalNodeID,
		attemptNumber,
	)
	runID := appTeamAttemptIdentity(
		"run",
		plan,
		logicalNodeID,
		attemptNumber,
	)
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID:                  "profile-" + logicalNodeID,
		AdapterType:         "pi",
		ProviderID:          "openai",
		ProviderAccountID:   "openai." + logicalNodeID,
		ModelID:             "gpt-test",
		AuthMode:            loomruntime.AuthBrokered,
		EndpointFingerprint: strings.Repeat("e", 64),
		CredentialReference: "credential-ref-" + logicalNodeID,
		CredentialRevision:  1,
		Timeout:             5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID: runtimeInstanceID, DeviceID: "device-1",
		AdapterType: "pi", DisplayName: runtimeInstanceID,
		ExecutableVersion: "1.0.0", Status: loomruntime.RuntimeOnline,
		Capacity: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	capsule := testTeamRoleCapsule(
		t, plan, logicalNodeID, agentInstanceID, profile,
	)
	payload, err := contextcapsule.RenderDispatchPayload(capsule)
	if err != nil {
		t.Fatal(err)
	}
	dispatch, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID:             teamCanaryMessageID(logicalNodeID + fmt.Sprint(attemptNumber)),
		CorrelationID:         "11111111-1111-4111-8111-111111111111",
		WorkItemID:            workItemID,
		RunID:                 runID,
		ClaimGeneration:       1,
		RuntimeInstanceID:     runtimeInstanceID,
		SenderAgentInstanceID: agentInstanceID,
		Sequence:              1,
		Type:                  bridgev1.MessageDispatch,
		EmittedAt:             now,
		Payload:               payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	return TeamNodeExecution{
		LogicalNodeID:            logicalNodeID,
		AttemptNumber:            attemptNumber,
		WorkflowPath:             "primary",
		SourcePath:               t.TempDir(),
		Profile:                  profile,
		Instance:                 instance,
		Dispatch:                 dispatch,
		Executor:                 executor,
		ContextCapsule:           capsule,
		ContextCapacityAuthority: missionContextCapacityAuthority(),
		ContextTokenCounter:      missionContextCounter,
	}
}

func testTeamRoleCapsule(
	t testing.TB,
	plan teams.ExecutionPlan,
	logicalNodeID string,
	agentInstanceID string,
	profile loomruntime.RuntimeProfile,
) contextcapsule.RoleContextCapsule {
	t.Helper()
	capsule, err := contextcapsule.BuildRoleContextCapsuleWithCapacity(
		contextcapsule.Target{
			ConversationID: "team-conversation:" + plan.TeamInstanceID(),
			TeamID:         plan.TeamInstanceID(), AgentID: agentInstanceID,
			RoleID: logicalNodeID, ProviderID: profile.ProviderID,
			ProviderAccountID: profile.ProviderAccountID, ModelID: profile.ModelID,
			AuthMode:           string(profile.AuthMode),
			ContextAdapterID:   "context:" + profile.AdapterType + ":v1",
			DisclosurePolicyID: "policy.test", DisclosurePolicyVersion: 1,
			TokenBudget: 2048,
		},
		[]contextcapsule.ItemInput{{
			ItemID: "goal-1", Kind: contextcapsule.KindConversationGoal,
			Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeTeamShared,
			Priority: contextcapsule.PrioritySystem, Required: true,
			Content:    []byte("Execute the controlled canary."),
			SourceType: contextcapsule.SourceAuthority,
			SourceRef:  "team-plan:" + plan.Digest(),
		}},
		missionContextCapacityAuthority(),
		missionContextCounter,
	)
	if err != nil {
		t.Fatal(err)
	}
	return capsule
}

func replaceTeamDispatchPayload(
	t testing.TB,
	frame bridgev1.Frame,
	payload []byte,
) bridgev1.Frame {
	t.Helper()
	replacement, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID: frame.MessageID(), CorrelationID: frame.CorrelationID(),
		WorkItemID: frame.WorkItemID(), RunID: frame.RunID(),
		ClaimGeneration:       frame.ClaimGeneration(),
		RuntimeInstanceID:     frame.RuntimeInstanceID(),
		SenderAgentInstanceID: frame.SenderAgentInstanceID(),
		Sequence:              frame.Sequence(), Type: frame.Type(), EmittedAt: frame.EmittedAt(),
		Payload: payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	return replacement
}

func testTeamNodeSemantics(
	t testing.TB,
	plan teams.ExecutionPlan,
	retryDelay time.Duration,
	mainFallback string,
) []TeamNodeSemantics {
	t.Helper()
	nodes := plan.Nodes()
	result := make([]TeamNodeSemantics, 0, len(nodes))
	for _, node := range nodes {
		contract, err := verification.NewOutputContract(
			1,
			verification.EmptyOutputInvalid,
		)
		if err != nil {
			t.Fatal(err)
		}
		credits := node.MaxAttempts() - 1
		fallback := ""
		if node.Role() == teams.ExecutionRoleMain {
			fallback = mainFallback
		}
		policy, err := rules.NewRecoveryPolicy(rules.RecoveryPolicyInput{
			Version:             1,
			RetryDelay:          retryDelay,
			AttemptCredits:      credits,
			ExhaustionAction:    rules.ExhaustionBlocked,
			WorkflowFallbackKey: fallback,
		})
		if err != nil {
			t.Fatal(err)
		}
		acceptance, err := verification.NewAcceptanceContract(
			1,
			[]string{"controlled output is accepted"},
			verification.AcceptanceRiskLow,
		)
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, TeamNodeSemantics{
			LogicalNodeID:       node.LogicalNodeID(),
			OutputContract:      contract,
			RecoveryPolicy:      policy,
			AcceptanceContract:  acceptance,
			PrimaryWorkflowPath: "primary",
		})
	}
	return result
}

func newTeamCanarySupervisor(
	t testing.TB,
	workAuthority *work.Authority,
	grantAuthority *authorization.Authority,
	adapter supervisor.RuntimeAdapter,
) ManagedNodeExecutor {
	t.Helper()
	workspaceRoot := t.TempDir()
	if err := os.Chmod(workspaceRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	controller, err := supervisor.New(
		supervisor.Config{
			WorkspaceRoot:  workspaceRoot,
			CleanupTimeout: time.Second,
		},
		workAuthority,
		grantAuthority,
		adapter,
	)
	if err != nil {
		t.Fatal(err)
	}
	return controller
}

func teamCanaryInboundFrame(
	request supervisor.AdapterRequest,
	sequence int64,
	messageType bridgev1.MessageType,
	payload []byte,
) bridgev1.Frame {
	material := sha256.Sum256([]byte(fmt.Sprintf(
		"%s/%d/%s",
		request.Binding.RunID,
		sequence,
		messageType,
	)))
	material[6] = material[6]&0x0f | 0x40
	material[8] = material[8]&0x3f | 0x80
	messageID := fmt.Sprintf(
		"%x-%x-%x-%x-%x",
		material[0:4],
		material[4:6],
		material[6:8],
		material[8:10],
		material[10:16],
	)
	frame, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID:             messageID,
		CorrelationID:         request.Dispatch.CorrelationID(),
		WorkItemID:            request.Binding.WorkItemID,
		RunID:                 request.Binding.RunID,
		ClaimGeneration:       request.Binding.ClaimGeneration,
		RuntimeInstanceID:     request.Binding.RuntimeInstanceID,
		SenderAgentInstanceID: request.Binding.SenderAgentInstanceID,
		Sequence:              sequence,
		Type:                  messageType,
		EmittedAt:             request.Dispatch.EmittedAt(),
		Payload:               payload,
	})
	if err != nil {
		panic(err)
	}
	return frame
}

func mustTeamCanaryJSON(value any) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return encoded
}

func openTeamCanaryDB(t testing.TB) *sql.DB {
	t.Helper()
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "journal_mode(WAL)")
	db, err := sql.Open("sqlite", fmt.Sprintf(
		"file:%s?%s",
		filepath.Join(t.TempDir(), "team-canary.db"),
		values.Encode(),
	))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := journal.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

func seedTeamCanaryRuntime(
	t testing.TB,
	store *journal.Store,
	runtimeID string,
	capacity int,
	now time.Time,
) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"discovery_digest": strings.Repeat("a", 64),
		"source_probe_id":  "probe-1",
		"instance": map[string]any{
			"id": runtimeID, "device_id": "device-1",
			"adapter_type": "pi", "display_name": runtimeID,
			"executable_version": "1.0.0", "status": "online",
			"observed_capabilities": []string{"models"}, "capacity": capacity,
		},
		"model_ids": []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(context.Background(), journal.Event{
		ID:       "runtime-" + runtimeID,
		StreamID: "runtime_instance:" + runtimeID,
		Seq:      1, IdempotencyKey: "runtime-" + runtimeID,
		Type: "RuntimeInstanceDiscovered", SchemaVersion: 1,
		EmittedAt:     now.Add(-time.Minute),
		CorrelationID: "22222222-2222-4222-8222-222222222222",
		PayloadJSON:   payload,
	}); err != nil {
		t.Fatal(err)
	}
}

func teamCanaryMessageID(logicalNodeID string) string {
	switch logicalNodeID {
	case "main":
		return "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	case "sub-a":
		return "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	default:
		return "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	}
}

// TestPhase2DPerAgentFailureIsolationMatrix proves every runtime failure class
// stays Agent-local: the affected sub-agent's Attempt fails with the exact
// closed terminal reason and the node is recovery-blocked, while healthy peers
// (other sub-agents and the main Agent) succeed and no peer inherits the
// failure. This is the source-level matrix for the installed-live G4 gate.
func TestPhase2DPerAgentFailureIsolationMatrix(t *testing.T) {
	cells := []struct {
		name       string
		failedNode string
		reason     string
	}{
		{name: "provider auth isolates one account", failedNode: "sub-a", reason: "provider_auth"},
		{name: "rate limit isolates one agent", failedNode: "sub-a", reason: "provider_rate_limit"},
		{name: "timeout isolates one agent", failedNode: "sub-b", reason: "timeout"},
		{name: "insufficient balance isolates one agent", failedNode: "sub-c", reason: "provider_insufficient_balance"},
		{name: "credential unavailable isolates one agent", failedNode: "sub-a", reason: "credential_unavailable"},
	}
	for _, cell := range cells {
		t.Run(cell.name, func(t *testing.T) {
			fixture := newFourProviderTeamCanaryFixture(
				t, "team-isolation-matrix-"+cell.failedNode,
			)
			expectedAccounts := make(map[string]string, len(fixture.request.Nodes))
			for index := range fixture.request.Nodes {
				execution := &fixture.request.Nodes[index]
				expectedAccounts[execution.LogicalNodeID] = execution.Profile.ProviderAccountID
				adapter := fixture.adapters[execution.LogicalNodeID]
				adapter.terminalStatus = "succeeded"
				adapter.terminalReason = ""
				if execution.LogicalNodeID == cell.failedNode {
					adapter.terminalStatus = "failed"
					adapter.terminalReason = cell.reason
				}
			}
			result, err := fixture.coordinator.Run(
				context.Background(), fixture.request,
			)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if result.Team().Status() != "blocked" ||
				len(result.ExecutedNodeIDs()) != 4 {
				t.Fatalf("Team = %q nodes=%v", result.Team().Status(), result.ExecutedNodeIDs())
			}
			for _, node := range result.Team().Nodes() {
				attempts := node.Attempts()
				if len(attempts) != 1 {
					t.Fatalf("%s Attempts = %#v", node.LogicalNodeID(), attempts)
				}
				if account := attempts[0].ExecutionBinding().ProviderAccountID; account != expectedAccounts[node.LogicalNodeID()] {
					t.Fatalf("Agent %s account = %q, want %q",
						node.LogicalNodeID(), account, expectedAccounts[node.LogicalNodeID()])
				}
				if node.LogicalNodeID() == cell.failedNode {
					if node.Status() != "blocked" ||
						attempts[0].Status() != "failed" ||
						attempts[0].TerminalReason() != cell.reason {
						t.Fatalf("affected Agent %s node=%s attempt=%s/%s want blocked/failed/%s",
							node.LogicalNodeID(), node.Status(),
							attempts[0].Status(), attempts[0].TerminalReason(),
							cell.reason)
					}
					continue
				}
				if node.Status() != "succeeded" ||
					attempts[0].Status() != "succeeded" ||
					attempts[0].TerminalReason() != "" {
					t.Fatalf("healthy Agent %s node=%s attempt=%s/%s",
						node.LogicalNodeID(), node.Status(),
						attempts[0].Status(), attempts[0].TerminalReason())
				}
			}
		})
	}
}
