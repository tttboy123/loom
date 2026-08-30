package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/roundtable"
	loomruntime "loom-pi-rebuild/internal/runtime"
)

func TestProductRoundtableMissionContextFreezesProjectedObjective(t *testing.T) {
	execution := projection.TeamExecution{
		TeamInstanceID: "team-roundtable-context",
		PlanDigest:     strings.Repeat("8", 64),
		Status:         "blocked",
		Nodes: []projection.TeamExecutionNode{
			{LogicalNodeID: "main", Role: "main", Title: "Improve Mission visibility without losing conversation continuity."},
			{LogicalNodeID: "review", Role: "subagent", Title: "Review the result."},
		},
	}
	context, err := productRoundtableMissionContext(
		"mission/team-roundtable-context", execution,
	)
	if err != nil {
		t.Fatal(err)
	}
	want := app.RoundtableMissionContext{
		MissionID:      "mission/team-roundtable-context",
		TeamInstanceID: "team-roundtable-context",
		Objective:      "Improve Mission visibility without losing conversation continuity.",
		Status:         "blocked", PlanDigest: strings.Repeat("8", 64),
	}
	if context != want {
		t.Fatalf("Mission context = %#v, want %#v", context, want)
	}

	execution.Nodes = append(execution.Nodes, projection.TeamExecutionNode{
		LogicalNodeID: "main-duplicate", Role: "main", Title: "Substituted objective",
	})
	if _, err := productRoundtableMissionContext(
		"mission/team-roundtable-context", execution,
	); err == nil {
		t.Fatal("duplicate authoritative Mission objective was accepted")
	}
}

type productRoundtableExecutionMixedRunner struct{}

type productRoundtableMissionContextSourceFixture struct{}

func (productRoundtableMissionContextSourceFixture) ResolveRoundtableMissionContext(
	_ context.Context,
	missionID string,
) (app.RoundtableMissionContext, error) {
	teamID := strings.TrimPrefix(missionID, "mission/")
	if teamID == "" || missionID != "mission/"+teamID {
		return app.RoundtableMissionContext{}, roundtable.ErrInvalidRoundtableSession
	}
	return app.RoundtableMissionContext{
		MissionID: missionID, TeamInstanceID: teamID,
		Objective: "Produce one bounded RoundTable recommendation.",
		Status:    "running", PlanDigest: strings.Repeat("7", 64),
	}, nil
}

func (productRoundtableExecutionMixedRunner) RunRound(
	_ context.Context,
	request app.TeamExecutionRequest,
) (map[string]productRoundtableSeatOutcome, error) {
	observer, ok := request.OutputObserver.(*productRoundtableOutputObserver)
	if !ok {
		return nil, errors.New("missing RoundTable output observer")
	}
	observer.mu.Lock()
	observer.outputs = map[string][]byte{
		"seat-planner": []byte("Planner produced a governed visible contribution."),
	}
	observer.mu.Unlock()
	return map[string]productRoundtableSeatOutcome{
		"seat-planner":  {status: "succeeded"},
		"seat-reviewer": {status: "failed", reason: "provider_rejected"},
	}, errors.New("controlled partial Team execution")
}

type productRoundtableAllSuccessRunner struct{}

func (productRoundtableAllSuccessRunner) RunRound(
	_ context.Context,
	request app.TeamExecutionRequest,
) (map[string]productRoundtableSeatOutcome, error) {
	observer, ok := request.OutputObserver.(*productRoundtableOutputObserver)
	if !ok {
		return nil, errors.New("missing RoundTable output observer")
	}
	outputs := make(map[string][]byte)
	outcomes := make(map[string]productRoundtableSeatOutcome)
	for _, node := range request.Plan.Nodes() {
		outputs[node.LogicalNodeID()] = []byte(
			"Accepted contribution from " + node.LogicalNodeID() + ".",
		)
		outcomes[node.LogicalNodeID()] = productRoundtableSeatOutcome{status: "succeeded"}
	}
	observer.mu.Lock()
	observer.outputs = outputs
	observer.mu.Unlock()
	return outcomes, nil
}

type productRoundtableBlockingRunner struct {
	started chan struct{}
	once    sync.Once
}

type productRoundtableUnexpectedRunner struct {
	mu     sync.Mutex
	called bool
}

type productRoundtableTerminalizingAgentInput struct {
	authority *roundtable.Authority
	now       time.Time
}

type productRoundtableDelayedAgentInput struct {
	mu       sync.Mutex
	failures int
	calls    int
	content  string
}

type productRoundtableInputCapabilityResolverFixture struct {
	attempt productActiveAttempt
	err     error
}

func (fixture productRoundtableInputCapabilityResolverFixture) Resolve(
	productActiveAttemptQuery,
) (productActiveAttempt, error) {
	return fixture.attempt, fixture.err
}

func (input *productRoundtableDelayedAgentInput) AdmitAgentInput(
	_ context.Context,
	request productAgentInputRequest,
) (productAgentInputReceipt, error) {
	if !validProductAgentInputRequest(request) {
		return productAgentInputReceipt{}, errProductInvalidAgentInput
	}
	input.mu.Lock()
	defer input.mu.Unlock()
	input.calls++
	if input.calls <= input.failures {
		return productAgentInputReceipt{}, errProductActiveAttemptNotFound
	}
	input.content = string(request.Content)
	return productAgentInputReceipt{
		SchemaVersion: productAgentInputSchemaVersion,
		IncidentID:    request.IncidentID, InputID: "input-steer-delayed-ready",
		Mode: request.Mode, OrderKey: 1,
	}, nil
}

func TestProductRoundtableProjectsExactRunningAttemptInputCapability(t *testing.T) {
	const attemptID = "attempt-capability"
	view := roundtable.View{
		Session: roundtable.Session{
			ID:      "session-capability",
			Context: &roundtable.SessionContext{ConversationID: "conversation-capability"},
		},
		Attempts: map[string]roundtable.SeatAttempt{
			attemptID: {
				AttemptID: attemptID, Status: roundtable.SeatAttemptRunning,
				SegmentID: "segment-capability", AgentInstanceID: "agent-capability",
				WorkItemID: "work-capability", RunID: "run-capability", ClaimGeneration: 1,
			},
		},
		Deliveries: map[string]roundtable.SeatDelivery{
			attemptID: {AttemptID: attemptID, SeatID: "seat-capability", Status: roundtable.SeatAttemptRunning},
		},
	}
	for _, test := range []struct {
		name     string
		resolver productRoundtableAgentInputCapabilityResolver
		want     string
	}{
		{
			name: "available",
			resolver: productRoundtableInputCapabilityResolverFixture{
				attempt: productActiveAttempt{AcceptsAgentInputs: true},
			},
			want: productRoundtableAgentInputAvailable,
		},
		{
			name: "unavailable",
			resolver: productRoundtableInputCapabilityResolverFixture{
				attempt: productActiveAttempt{AcceptsAgentInputs: false},
			},
			want: productRoundtableAgentInputUnavailable,
		},
		{
			name: "pending",
			resolver: productRoundtableInputCapabilityResolverFixture{
				err: errProductActiveAttemptNotFound,
			},
			want: productRoundtableAgentInputPending,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			execution := &productRoundtableExecution{activeAttempts: test.resolver}
			projected := execution.Project(context.Background(), view)
			if got := projected.Deliveries[attemptID].AgentInputCapability; got != test.want {
				t.Fatalf("capability = %q, want %q", got, test.want)
			}
		})
	}
}

func TestProductRoundtableSteerRejectsGuidanceAboveRoundtableBoundary(t *testing.T) {
	fixture := newProductRoundtableRouteFixture(t)
	if err := fixture.controller.SetAgentInput(&productRoundtableDelayedAgentInput{}); err != nil {
		t.Fatal(err)
	}
	guidance := make([]byte, roundtable.MaxRoundPromptBytes+1)
	for index := range guidance {
		guidance[index] = 'x'
	}
	_, err := fixture.controller.SteerSeat(
		context.Background(),
		productRoundtableSteerSeatCommand{Guidance: guidance},
	)
	if !errors.Is(err, roundtable.ErrInvalidRoundtableIntervention) {
		t.Fatalf("oversized guidance error = %v", err)
	}
	for index, value := range guidance {
		if value != 0 {
			t.Fatalf("guidance byte %d was not cleared", index)
		}
	}
}

func (input productRoundtableTerminalizingAgentInput) AdmitAgentInput(
	ctx context.Context,
	request productAgentInputRequest,
) (productAgentInputReceipt, error) {
	view, err := input.authority.ReadView(ctx, "session-steer-terminal-race")
	if err != nil {
		return productAgentInputReceipt{}, err
	}
	var current roundtable.SeatAttempt
	for _, attempt := range view.Attempts {
		if attempt.SegmentID == request.SegmentID &&
			attempt.AgentInstanceID == request.AgentInstanceID {
			current = attempt
			break
		}
	}
	if current.AttemptID == "" {
		return productAgentInputReceipt{}, errors.New("active RoundTable Attempt unavailable")
	}
	if _, err := input.authority.CancelSeatAttempt(ctx, roundtable.CancelSeatAttemptCommand{
		SessionID: "session-steer-terminal-race", RoundID: current.RoundID,
		SeatID: current.SeatID, AttemptID: current.AttemptID,
		EmittedAt: input.now, CorrelationID: roundtableRouteCorrelation,
	}); err != nil {
		return productAgentInputReceipt{}, err
	}
	return productAgentInputReceipt{
		SchemaVersion: productAgentInputSchemaVersion,
		IncidentID:    request.IncidentID, InputID: "input-steer-terminal-race",
		Mode: request.Mode, OrderKey: 1,
	}, nil
}

func (runner *productRoundtableUnexpectedRunner) RunRound(
	context.Context,
	app.TeamExecutionRequest,
) (map[string]productRoundtableSeatOutcome, error) {
	runner.mu.Lock()
	runner.called = true
	runner.mu.Unlock()
	return nil, errors.New("runner must not start")
}

func (runner *productRoundtableBlockingRunner) RunRound(
	ctx context.Context,
	_ app.TeamExecutionRequest,
) (map[string]productRoundtableSeatOutcome, error) {
	runner.once.Do(func() { close(runner.started) })
	<-ctx.Done()
	return nil, ctx.Err()
}

type productRoundtableExecutionStoreFixture struct {
	mu       sync.Mutex
	payloads map[string]attemptpayload.Payload
	capsules map[string]recordedRoundtableContextCapsule
}

type recordedRoundtableContextCapsule struct {
	authority contextcapsule.AuthorityRecord
	capsule   contextcapsule.RoleContextCapsule
	payload   []byte
}

func (store *productRoundtableExecutionStoreFixture) PutRoleContextCapsule(
	_ context.Context,
	capsule contextcapsule.RoleContextCapsule,
	payload []byte,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.capsules == nil {
		store.capsules = make(map[string]recordedRoundtableContextCapsule)
	}
	authority := capsule.AuthorityRecord()
	store.capsules[authority.CapsuleDigest] = recordedRoundtableContextCapsule{
		authority: authority, capsule: capsule, payload: append([]byte(nil), payload...),
	}
	return nil
}

func (store *productRoundtableExecutionStoreFixture) ReadRoleContextCapsule(
	_ context.Context,
	authority contextcapsule.AuthorityRecord,
) (contextcapsule.RoleContextCapsule, []byte, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	record, found := store.capsules[authority.CapsuleDigest]
	if !found || record.authority != authority {
		return contextcapsule.RoleContextCapsule{}, nil, contextcapsule.ErrInvalidCapsule
	}
	return record.capsule, append([]byte(nil), record.payload...), nil
}

func (store *productRoundtableExecutionStoreFixture) ListRoleContextCapsuleAuthorities(
	_ context.Context,
	conversationID string,
) ([]contextcapsule.AuthorityRecord, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	authorities := make([]contextcapsule.AuthorityRecord, 0, len(store.capsules))
	for _, record := range store.capsules {
		if record.authority.ConversationID == conversationID {
			authorities = append(authorities, record.authority)
		}
	}
	sort.Slice(authorities, func(i, j int) bool {
		return authorities[i].CapsuleDigest < authorities[j].CapsuleDigest
	})
	return authorities, nil
}

func (store *productRoundtableExecutionStoreFixture) PutAttemptPayload(
	_ context.Context,
	payload attemptpayload.Payload,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.payloads == nil {
		store.payloads = make(map[string]attemptpayload.Payload)
	}
	payload.Content = append([]byte(nil), payload.Content...)
	store.payloads[payload.Binding.PayloadID] = payload
	return nil
}

func (store *productRoundtableExecutionStoreFixture) ReadAttemptPayload(
	_ context.Context,
	binding attemptpayload.Binding,
) (attemptpayload.Payload, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	payload, found := store.payloads[binding.PayloadID]
	if !found || payload.Binding != binding {
		return attemptpayload.Payload{}, attemptpayload.ErrPayloadNotFound
	}
	payload.Content = append([]byte(nil), payload.Content...)
	return payload, nil
}

func (store *productRoundtableExecutionStoreFixture) ListPendingAttemptPayloads(
	context.Context,
	attemptpayload.Scope,
) ([]attemptpayload.Payload, error) {
	return []attemptpayload.Payload{}, nil
}

func (store *productRoundtableExecutionStoreFixture) MarkAttemptPayloadDelivered(
	_ context.Context,
	binding attemptpayload.Binding,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	payload, found := store.payloads[binding.PayloadID]
	if !found {
		return attemptpayload.ErrPayloadNotFound
	}
	payload.Status = attemptpayload.StatusDelivered
	store.payloads[binding.PayloadID] = payload
	return nil
}

type productRoundtableFixture struct {
	authority   *roundtable.Authority
	controller  *productRoundtableController
	evidence    *evidence.Store
	journal     *journal.Store
	handler     func(context.Context, localipc.Request) localipc.Response
	concludedAt time.Time
}

func newProductRoundtableRouteFixture(t *testing.T) *productRoundtableFixture {
	return newProductRoundtableRouteFixtureWithResolver(t, nil)
}

func newProductRoundtableRouteFixtureWithResolver(
	t *testing.T,
	resolver productRoundtableBindingResolver,
) *productRoundtableFixture {
	t.Helper()
	realTemp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(realTemp, "roundtable-route")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	evidenceStore, err := evidence.NewStore(filepath.Join(root, "evidence"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = evidenceStore.Close() })
	database, err := sql.Open("sqlite", filepath.Join(root, "roundtable.db"))
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = database.Close() })
	if _, err := database.Exec("PRAGMA busy_timeout=5000"); err != nil {
		t.Fatal(err)
	}
	if err := journal.Migrate(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	journalStore := journal.NewStore(database)
	now := func() time.Time {
		return time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	}
	authority, err := roundtable.NewAuthority(
		journalStore, evidenceStore, now, productRoundtableReader{},
	)
	if err != nil {
		t.Fatal(err)
	}
	controller, err := newProductRoundtableController(
		authority, now, resolver,
	)
	if err != nil {
		t.Fatal(err)
	}
	handler := newProductRouteHandler(productRouteServices{roundtable: controller})
	return &productRoundtableFixture{
		authority: authority, controller: controller, evidence: evidenceStore,
		journal: journalStore, handler: handler,
	}
}

type productRoundtableBindingResolverFixture struct {
	context roundtable.SessionContext
}

func (fixture productRoundtableBindingResolverFixture) ResolveSessionContext(
	_ context.Context,
	request roundtable.SessionLinkRequest,
) (roundtable.SessionContext, error) {
	if request.ConversationID != fixture.context.ConversationID ||
		request.MissionID != fixture.context.MissionID ||
		request.TeamInstanceID != fixture.context.TeamID {
		return roundtable.SessionContext{}, roundtable.ErrInvalidRoundtableSession
	}
	return fixture.context, nil
}

func (fixture productRoundtableBindingResolverFixture) ResolveSeatBinding(
	_ context.Context,
	sessionID string,
	seatID string,
	sessionContext roundtable.SessionContext,
	request roundtable.SeatBindingRequest,
	membershipRevision int,
) (roundtable.FrozenSeatBinding, error) {
	profile := loomruntime.RuntimeProfile{
		ID: request.RuntimeProfileID, AdapterType: "loom-native",
		ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
		ModelID: "deepseek-chat", AuthMode: loomruntime.AuthBrokered,
		EndpointFingerprint: strings.Repeat("a", 64),
		CredentialReference: "credential-ref-deepseek-primary",
		CredentialRevision:  3, RequiredCapabilities: []string{"text"},
		Timeout: time.Minute,
	}
	runtimeInstance := loomruntime.RuntimeInstance{
		ID: "runtime-deepseek", DeviceID: "device-local",
		AdapterType: "loom-native", DisplayName: "Loom Native",
		Status: loomruntime.RuntimeOnline, ObservedCapabilities: []string{"text"},
		Capacity: 2,
	}
	execution, err := loomruntime.FreezeExecutionBinding(profile, runtimeInstance)
	if err != nil {
		return roundtable.FrozenSeatBinding{}, err
	}
	return roundtable.FreezeSeatBinding(
		sessionID, seatID, sessionContext, request.AgentDefinitionID,
		request.TeamRoleKind, request.RuntimeProfileID, execution,
		membershipRevision,
	)
}

type productRoundtableReader struct{}

func (productRoundtableReader) Read([]byte) (int, error) { return 0, io.EOF }

func (fixture *productRoundtableFixture) call(
	t *testing.T,
	method string,
	params any,
) localipc.Response {
	t.Helper()
	encoded, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	return fixture.handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "roundtable-route-request", Method: method,
		Params: encoded,
	})
}

func (fixture *productRoundtableFixture) decodeView(
	t *testing.T,
	response localipc.Response,
) roundtable.View {
	t.Helper()
	if !response.OK || response.Error != nil {
		t.Fatalf("response = %#v", response)
	}
	var view roundtable.View
	if err := json.Unmarshal(response.Result, &view); err != nil {
		t.Fatalf("decode view: %v", err)
	}
	return view
}

const roundtableRouteCorrelation = "11111111-1111-4111-8111-111111111111"

func TestProductRoundtableOpenRoundIsolatesFailedSeatAndPersistsSuccessfulOutput(
	t *testing.T,
) {
	sessionContext := roundtable.SessionContext{
		ConversationID: "conversation-roundtable-execution",
		MissionID:      "mission/team-roundtable-execution",
		TeamID:         "team-roundtable-execution", TeamVersion: 1,
		WorkspaceID: "project-roundtable-execution",
	}
	fixture := newProductRoundtableRouteFixtureWithResolver(
		t, productRoundtableBindingResolverFixture{context: sessionContext},
	)
	store := &productRoundtableExecutionStoreFixture{}
	execution, err := newProductRoundtableExecution(
		fixture.authority, productRoundtableExecutionMixedRunner{}, store, store,
		nil, productRoundtableMissionContextSourceFixture{}, t.TempDir(), func() time.Time {
			return time.Date(2026, 8, 16, 12, 1, 0, 0, time.UTC)
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.controller.SetExecution(execution); err != nil {
		t.Fatal(err)
	}
	fixture.decodeView(t, fixture.call(t, "roundtable_session_create", productRoundtableSessionCreateParams{
		SchemaVersion: 1, SessionID: "session-execution", ModeratorSeat: "seat-moderator",
		Title: "Real seat execution", CorrelationID: roundtableRouteCorrelation,
		Link: &roundtable.SessionLinkRequest{
			ConversationID: sessionContext.ConversationID,
			MissionID:      sessionContext.MissionID, TeamInstanceID: sessionContext.TeamID,
		},
	}))
	for _, seat := range []struct {
		id, role, profile string
	}{
		{"seat-planner", "main", "profile.planner"},
		{"seat-reviewer", "subagent", "profile.reviewer"},
	} {
		fixture.decodeView(t, fixture.call(t, "roundtable_add_seat", productRoundtableAddSeatParams{
			SchemaVersion: 1, SessionID: "session-execution", SeatID: seat.id,
			DisplayName: seat.id, CorrelationID: roundtableRouteCorrelation,
			Selection: &roundtable.SeatBindingRequest{
				AgentDefinitionID: "agent." + seat.id, TeamRoleKind: seat.role,
				RuntimeProfileID: seat.profile,
			},
		}))
	}
	const prompt = "Private user deliberation prompt that must not enter the Journal."
	opened := fixture.decodeView(t, fixture.call(t, "roundtable_open_round", productRoundtableOpenRoundParams{
		SchemaVersion: 1, SessionID: "session-execution", RoundID: "round-1",
		ModeratorSeat: "seat-moderator", Prompt: prompt,
		CorrelationID: roundtableRouteCorrelation,
	}))
	if len(opened.Attempts) != 2 {
		t.Fatalf("started Attempts = %#v", opened.Attempts)
	}
	deadline := time.Now().Add(2 * time.Second)
	var projected roundtable.View
	for time.Now().Before(deadline) {
		projected, err = fixture.controller.ReadView(context.Background(), "session-execution")
		if err != nil {
			t.Fatal(err)
		}
		terminal := 0
		for _, attempt := range projected.Attempts {
			if attempt.Status == roundtable.SeatAttemptFailed ||
				attempt.Status == roundtable.SeatAttemptSucceeded {
				terminal++
			}
		}
		if terminal == 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	var planner, reviewer roundtable.SeatAttempt
	for _, attempt := range projected.Attempts {
		switch attempt.SeatID {
		case "seat-planner":
			planner = attempt
		case "seat-reviewer":
			reviewer = attempt
		}
	}
	if planner.Status != roundtable.SeatAttemptSucceeded || planner.PayloadReference == "" ||
		projected.Deliveries[planner.AttemptID].Body == "" {
		t.Fatalf("successful seat = %#v delivery=%#v", planner, projected.Deliveries[planner.AttemptID])
	}
	if reviewer.Status != roundtable.SeatAttemptFailed ||
		reviewer.FailureStage != "agent_attempt_dispatch" || reviewer.IncidentID == "" {
		t.Fatalf("failed seat = %#v", reviewer)
	}
	reviewerBinding := projected.Seats["seat-reviewer"].Binding
	if reviewerBinding == nil {
		t.Fatal("reviewer seat binding is unavailable before retry")
	}
	const retryGuidance = "Retry this review using the frozen seat route."
	staleRetry := fixture.call(t, "roundtable_retry_seat", productRoundtableRetrySeatParams{
		SchemaVersion: 1, SessionID: "session-execution", RoundID: "round-1",
		InterventionID: "intervention-retry-reviewer-stale", ModeratorSeat: "seat-moderator",
		SeatID: "seat-reviewer", AttemptID: reviewer.AttemptID,
		ExpectedMembershipRevision: reviewerBinding.MembershipRevision + 1,
		ExpectedSeatBindingDigest:  reviewerBinding.BindingDigest,
		Guidance:                   retryGuidance,
		CorrelationID:              roundtableRouteCorrelation,
	})
	if staleRetry.OK || staleRetry.Error == nil || staleRetry.Error.Code != "intervention_conflict" {
		t.Fatalf("stale frozen retry = %#v error=%#v", staleRetry, staleRetry.Error)
	}
	retried := fixture.decodeView(t, fixture.call(t, "roundtable_retry_seat", productRoundtableRetrySeatParams{
		SchemaVersion: 1, SessionID: "session-execution", RoundID: "round-1",
		InterventionID: "intervention-retry-reviewer-2", ModeratorSeat: "seat-moderator",
		SeatID: "seat-reviewer", AttemptID: reviewer.AttemptID,
		ExpectedMembershipRevision: reviewerBinding.MembershipRevision,
		ExpectedSeatBindingDigest:  reviewerBinding.BindingDigest,
		Guidance:                   retryGuidance,
		CorrelationID:              roundtableRouteCorrelation,
	}))
	var retry roundtable.SeatAttempt
	for _, candidate := range retried.Attempts {
		if candidate.SeatID == "seat-reviewer" && candidate.AttemptNumber == 2 {
			retry = candidate
		}
	}
	if retry.AttemptID == "" || retry.AttemptID == reviewer.AttemptID ||
		retry.SegmentID == reviewer.SegmentID ||
		retry.ExecutionBindingDigest != reviewer.ExecutionBindingDigest {
		t.Fatalf("fresh retry = %#v, previous = %#v", retry, reviewer)
	}
	store.mu.Lock()
	retryCapsuleRecord, found := store.capsules[retry.ContextCapsuleDigest]
	store.mu.Unlock()
	if !found || retryCapsuleRecord.authority.TeamID != retry.ExecutionTeamID {
		t.Fatalf("retry capsule = %#v, found=%t", retryCapsuleRecord.authority, found)
	}
	retryItems := make(map[string]contextcapsule.DisclosedItem)
	for _, item := range retryCapsuleRecord.capsule.Disclosed() {
		retryItems[item.ItemID] = item
	}
	if item := retryItems["roundtable-prompt"]; string(item.Content) != prompt ||
		item.Kind != contextcapsule.KindConversationGoal ||
		item.Trust != contextcapsule.TrustAuthoritative {
		t.Fatalf("retry original prompt = %#v", item)
	}
	if item := retryItems["roundtable-retry-guidance"]; string(item.Content) != retryGuidance ||
		item.Kind != contextcapsule.KindConfirmedConstraint ||
		item.Trust != contextcapsule.TrustAuthoritative ||
		item.AllowedRoleID != "seat-reviewer" {
		t.Fatalf("retry guidance = %#v", item)
	}
	priorCount := 0
	for _, item := range retryItems {
		if item.Kind != contextcapsule.KindPriorModelOutput {
			continue
		}
		priorCount++
		if string(item.Content) != "Planner produced a governed visible contribution." ||
			item.Trust != contextcapsule.TrustUntrusted ||
			item.SourceType != contextcapsule.SourceModelOutput ||
			item.SourceRef != "roundtable-attempt:"+planner.AttemptID+":"+
				planner.OutputDigest {
			t.Fatalf("retry prior contribution = %#v", item)
		}
	}
	if priorCount != 1 {
		t.Fatalf("retry prior contribution count = %d, want 1", priorCount)
	}
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		retried, err = fixture.controller.ReadView(context.Background(), "session-execution")
		if err != nil {
			t.Fatal(err)
		}
		if retried.Attempts[retry.AttemptID].Status == roundtable.SeatAttemptFailed {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	beforeSkip := fixture.call(t, "roundtable_conclude", productRoundtableConcludeParams{
		SchemaVersion: 1, SessionID: "session-execution", ModeratorSeat: "seat-moderator",
		CorrelationID: roundtableRouteCorrelation,
	})
	if beforeSkip.OK || beforeSkip.Error == nil || beforeSkip.Error.Code != "intervention_required" {
		t.Fatalf("conclusion without peer resolution = %#v error=%#v", beforeSkip, beforeSkip.Error)
	}
	reviewerBinding = retried.Seats["seat-reviewer"].Binding
	if reviewerBinding == nil {
		t.Fatal("reviewer seat binding is unavailable before skip")
	}
	staleSkip := fixture.call(t, "roundtable_skip_seat", productRoundtableSkipSeatParams{
		SchemaVersion: 1, SessionID: "session-execution", RoundID: "round-1",
		InterventionID: "intervention-skip-reviewer-stale", ModeratorSeat: "seat-moderator",
		SeatID:                     "seat-reviewer",
		ExpectedMembershipRevision: reviewerBinding.MembershipRevision + 1,
		ExpectedSeatBindingDigest:  reviewerBinding.BindingDigest,
		CorrelationID:              roundtableRouteCorrelation,
	})
	if staleSkip.OK || staleSkip.Error == nil || staleSkip.Error.Code != "intervention_conflict" {
		t.Fatalf("stale frozen skip = %#v error=%#v", staleSkip, staleSkip.Error)
	}
	fixture.decodeView(t, fixture.call(t, "roundtable_skip_seat", productRoundtableSkipSeatParams{
		SchemaVersion: 1, SessionID: "session-execution", RoundID: "round-1",
		InterventionID: "intervention-skip-reviewer", ModeratorSeat: "seat-moderator",
		SeatID:                     "seat-reviewer",
		ExpectedMembershipRevision: reviewerBinding.MembershipRevision,
		ExpectedSeatBindingDigest:  reviewerBinding.BindingDigest,
		CorrelationID:              roundtableRouteCorrelation,
	}))
	concluded := fixture.decodeView(t, fixture.call(t, "roundtable_conclude", productRoundtableConcludeParams{
		SchemaVersion: 1, SessionID: "session-execution", ModeratorSeat: "seat-moderator",
		CorrelationID: roundtableRouteCorrelation,
	}))
	if !concluded.Session.Concluded {
		t.Fatalf("accepted candidate did not conclude Mission-linked session: %#v", concluded.Session)
	}
	if concluded.Deliveries[planner.AttemptID].Body == "" {
		t.Fatalf("conclusion response dropped accepted Agent output: %#v", concluded.Deliveries)
	}
	store.mu.Lock()
	persisted := store.payloads[planner.PayloadReference]
	store.mu.Unlock()
	if persisted.Binding.Scope.ConversationID != sessionContext.ConversationID {
		t.Fatalf(
			"payload ConversationID = %q, want linked Conversation %q",
			persisted.Binding.Scope.ConversationID, sessionContext.ConversationID,
		)
	}
	events, err := fixture.journal.ReadStream(
		context.Background(), "roundtable/session/session-execution",
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if strings.Contains(string(event.PayloadJSON), prompt) ||
			strings.Contains(string(event.PayloadJSON), retryGuidance) {
			t.Fatalf("RoundTable Journal leaked dispatch content in %s", event.Type)
		}
	}
}

func TestProductRoundtableSecondRoundReceivesPriorAgentResultsAsUntrustedContext(
	t *testing.T,
) {
	sessionContext := roundtable.SessionContext{
		ConversationID: "conversation-roundtable-synthesis",
		MissionID:      "mission/team-roundtable-synthesis",
		TeamID:         "team-roundtable-synthesis", TeamVersion: 1,
		WorkspaceID: "project-roundtable-synthesis",
	}
	fixture := newProductRoundtableRouteFixtureWithResolver(
		t, productRoundtableBindingResolverFixture{context: sessionContext},
	)
	store := &productRoundtableExecutionStoreFixture{}
	execution, err := newProductRoundtableExecution(
		fixture.authority, productRoundtableAllSuccessRunner{}, store, store,
		nil, productRoundtableMissionContextSourceFixture{}, t.TempDir(), func() time.Time {
			return time.Date(2026, 8, 27, 2, 0, 0, 0, time.UTC)
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = execution.Close() })
	if err := fixture.controller.SetExecution(execution); err != nil {
		t.Fatal(err)
	}
	const sessionID = "session-roundtable-synthesis"
	fixture.decodeView(t, fixture.call(t, "roundtable_session_create", productRoundtableSessionCreateParams{
		SchemaVersion: 1, SessionID: sessionID, ModeratorSeat: "seat-moderator",
		Title: "Synthesize Agent results", CorrelationID: roundtableRouteCorrelation,
		Link: &roundtable.SessionLinkRequest{
			ConversationID: sessionContext.ConversationID,
			MissionID:      sessionContext.MissionID, TeamInstanceID: sessionContext.TeamID,
		},
	}))
	for _, seat := range []struct{ id, role string }{
		{"seat-lead", "main"}, {"seat-participant", "subagent"},
	} {
		fixture.decodeView(t, fixture.call(t, "roundtable_add_seat", productRoundtableAddSeatParams{
			SchemaVersion: 1, SessionID: sessionID, SeatID: seat.id,
			DisplayName: seat.id, CorrelationID: roundtableRouteCorrelation,
			Selection: &roundtable.SeatBindingRequest{
				AgentDefinitionID: "agent." + seat.id, TeamRoleKind: seat.role,
				RuntimeProfileID: "profile." + seat.id,
			},
		}))
	}
	const roundOnePrompt = "Offer independent UX recommendations."
	fixture.decodeView(t, fixture.call(t, "roundtable_open_round", productRoundtableOpenRoundParams{
		SchemaVersion: 1, SessionID: sessionID, RoundID: "round-1",
		ModeratorSeat: "seat-moderator", Prompt: roundOnePrompt,
		CorrelationID: roundtableRouteCorrelation,
	}))
	roundOne := waitForRoundtableTerminalAttempts(t, fixture.controller, sessionID, "round-1", 2)
	const synthesisPrompt = "Synthesize the prior Agent contributions into one recommendation."
	roundTwo := fixture.decodeView(t, fixture.call(t, "roundtable_open_round", productRoundtableOpenRoundParams{
		SchemaVersion: 1, SessionID: sessionID, RoundID: "round-2",
		ModeratorSeat: "seat-moderator", Prompt: synthesisPrompt,
		CorrelationID: "22222222-2222-4222-8222-222222222222",
	}))
	priorOutputs := make(map[string]string)
	for _, attempt := range roundOne.Attempts {
		if attempt.RoundID == "round-1" && attempt.Status == roundtable.SeatAttemptSucceeded {
			priorOutputs[attempt.AttemptID] = roundOne.Deliveries[attempt.AttemptID].Body
		}
	}
	if len(priorOutputs) != 2 {
		t.Fatalf("round one prior outputs = %#v", priorOutputs)
	}
	roundTwoAttempts := 0
	for _, attempt := range roundTwo.Attempts {
		if attempt.RoundID != "round-2" {
			continue
		}
		roundTwoAttempts++
		store.mu.Lock()
		record, found := store.capsules[attempt.ContextCapsuleDigest]
		store.mu.Unlock()
		if !found {
			t.Fatalf("round two capsule %q unavailable", attempt.ContextCapsuleDigest)
		}
		seen := make(map[string]string)
		for _, item := range record.capsule.Disclosed() {
			if item.Kind != contextcapsule.KindPriorModelOutput {
				continue
			}
			if item.Trust != contextcapsule.TrustUntrusted ||
				item.SourceType != contextcapsule.SourceModelOutput {
				t.Fatalf("round two prior item = %#v", item)
			}
			seen[item.SourceRef] = string(item.Content)
		}
		if len(seen) != 2 {
			t.Fatalf("round two prior items = %#v", seen)
		}
		for priorAttemptID, output := range priorOutputs {
			foundOutput := false
			for sourceRef, content := range seen {
				if strings.HasPrefix(sourceRef, "roundtable-attempt:"+priorAttemptID+":") &&
					content == output {
					foundOutput = true
				}
			}
			if !foundOutput {
				t.Fatalf("round two capsule omitted prior Attempt %q", priorAttemptID)
			}
		}
	}
	if roundTwoAttempts != 2 {
		t.Fatalf("round two Attempt count = %d", roundTwoAttempts)
	}
	events, err := fixture.journal.ReadStream(
		context.Background(), "roundtable/session/"+sessionID,
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		body := string(event.PayloadJSON)
		if strings.Contains(body, roundOnePrompt) || strings.Contains(body, synthesisPrompt) ||
			strings.Contains(body, "Accepted contribution from") {
			t.Fatalf("RoundTable Journal leaked synthesis content in %s", event.Type)
		}
	}
}

func waitForRoundtableTerminalAttempts(
	t *testing.T,
	controller *productRoundtableController,
	sessionID string,
	roundID string,
	want int,
) roundtable.View {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		view, err := controller.ReadView(context.Background(), sessionID)
		if err != nil {
			t.Fatal(err)
		}
		terminal := 0
		for _, attempt := range view.Attempts {
			if attempt.RoundID == roundID &&
				(attempt.Status == roundtable.SeatAttemptSucceeded ||
					attempt.Status == roundtable.SeatAttemptFailed ||
					attempt.Status == roundtable.SeatAttemptCancelled) {
				terminal++
			}
		}
		if terminal == want {
			return view
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("RoundTable %s did not reach %d terminal Attempts", roundID, want)
	return roundtable.View{}
}

func TestProductRoundtableSteerRetainsAuditWhenAttemptTerminatesAfterInboxAdmission(
	t *testing.T,
) {
	sessionContext := roundtable.SessionContext{
		ConversationID: "conversation-roundtable-steer-race",
		MissionID:      "mission/team-roundtable-steer-race",
		TeamID:         "team-roundtable-steer-race", TeamVersion: 1,
		WorkspaceID: "project-roundtable-steer-race",
	}
	fixture := newProductRoundtableRouteFixtureWithResolver(
		t, productRoundtableBindingResolverFixture{context: sessionContext},
	)
	const sessionID = "session-steer-terminal-race"
	fixture.decodeView(t, fixture.call(t, "roundtable_session_create", productRoundtableSessionCreateParams{
		SchemaVersion: 1, SessionID: sessionID, ModeratorSeat: "seat-moderator",
		Title: "Steer terminal race", CorrelationID: roundtableRouteCorrelation,
		Link: &roundtable.SessionLinkRequest{
			ConversationID: sessionContext.ConversationID, MissionID: sessionContext.MissionID,
			TeamInstanceID: sessionContext.TeamID,
		},
	}))
	for _, seat := range []struct{ id, role string }{{"seat-main", "main"}, {"seat-peer", "subagent"}} {
		fixture.decodeView(t, fixture.call(t, "roundtable_add_seat", productRoundtableAddSeatParams{
			SchemaVersion: 1, SessionID: sessionID, SeatID: seat.id,
			DisplayName: seat.id, CorrelationID: roundtableRouteCorrelation,
			Selection: &roundtable.SeatBindingRequest{
				AgentDefinitionID: "agent." + seat.id, TeamRoleKind: seat.role,
				RuntimeProfileID: "profile." + seat.id,
			},
		}))
	}
	opened, err := fixture.authority.OpenRound(context.Background(), roundtable.OpenRoundCommand{
		SessionID: sessionID, RoundID: "round-1", ModeratorSeat: "seat-moderator",
		EmittedAt:     time.Date(2026, 8, 16, 12, 0, 30, 0, time.UTC),
		CorrelationID: roundtableRouteCorrelation,
	})
	if err != nil {
		t.Fatal(err)
	}
	binding := opened.Seats["seat-main"].Binding
	if binding == nil {
		t.Fatal("main seat binding missing")
	}
	started, err := fixture.authority.StartSeatAttempt(context.Background(), roundtable.StartSeatAttemptCommand{
		SessionID: sessionID, RoundID: "round-1", SeatID: "seat-main",
		AttemptID: "attempt-steer-terminal-race", AttemptNumber: 1,
		ExecutionTeamID: "execution-team-steer-race", WorkItemID: "work-steer-race",
		RunID: "run-steer-race", SegmentID: "segment-steer-race", ClaimGeneration: 1,
		RuntimeInstanceID:  binding.ExecutionBinding.RuntimeInstanceID,
		AgentInstanceID:    "agent-instance-steer-race",
		MembershipRevision: binding.MembershipRevision, SeatBindingDigest: binding.BindingDigest,
		ExecutionBindingDigest: binding.ExecutionBinding.BindingDigest,
		ContextCapsuleDigest:   strings.Repeat("c", 64),
		EmittedAt:              time.Date(2026, 8, 16, 12, 0, 31, 0, time.UTC),
		CorrelationID:          roundtableRouteCorrelation,
	})
	if err != nil {
		t.Fatal(err)
	}
	attempt := started.Attempts["attempt-steer-terminal-race"]
	requestedAt := time.Date(2026, 8, 16, 12, 1, 0, 0, time.UTC)
	fixture.controller.now = func() time.Time { return requestedAt }
	if err := fixture.controller.SetAgentInput(productRoundtableTerminalizingAgentInput{
		authority: fixture.authority, now: requestedAt.Add(time.Second),
	}); err != nil {
		t.Fatal(err)
	}
	guidance := []byte("Keep the accepted Steer auditable across terminal projection races.")
	view, err := fixture.controller.SteerSeat(context.Background(), productRoundtableSteerSeatCommand{
		SessionID: sessionID, RoundID: "round-1", InterventionID: "steer-terminal-race",
		ModeratorSeat: "seat-moderator", SeatID: "seat-main", AttemptID: attempt.AttemptID,
		Guidance: guidance, CorrelationID: roundtableRouteCorrelation,
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt := view.Interventions["steer-terminal-race"]
	if view.Attempts[attempt.AttemptID].Status != roundtable.SeatAttemptCancelled ||
		receipt.Kind != roundtable.InterventionSteer ||
		receipt.InputID != "input-steer-terminal-race" || receipt.RequestedAt != requestedAt {
		t.Fatalf("terminal-race steer = attempt %#v receipt %#v", view.Attempts[attempt.AttemptID], receipt)
	}
}

func TestProductRoundtableSteerWaitsForExactRunningAttemptInputReadiness(t *testing.T) {
	sessionContext := roundtable.SessionContext{
		ConversationID: "conversation-roundtable-steer-ready",
		MissionID:      "mission/team-roundtable-steer-ready",
		TeamID:         "team-roundtable-steer-ready", TeamVersion: 1,
		WorkspaceID: "project-roundtable-steer-ready",
	}
	fixture := newProductRoundtableRouteFixtureWithResolver(
		t, productRoundtableBindingResolverFixture{context: sessionContext},
	)
	const sessionID = "session-steer-delayed-ready"
	fixture.decodeView(t, fixture.call(t, "roundtable_session_create", productRoundtableSessionCreateParams{
		SchemaVersion: 1, SessionID: sessionID, ModeratorSeat: "seat-moderator",
		Title: "Steer delayed input readiness", CorrelationID: roundtableRouteCorrelation,
		Link: &roundtable.SessionLinkRequest{
			ConversationID: sessionContext.ConversationID, MissionID: sessionContext.MissionID,
			TeamInstanceID: sessionContext.TeamID,
		},
	}))
	for _, seat := range []struct{ id, role string }{{"seat-main", "main"}, {"seat-peer", "subagent"}} {
		fixture.decodeView(t, fixture.call(t, "roundtable_add_seat", productRoundtableAddSeatParams{
			SchemaVersion: 1, SessionID: sessionID, SeatID: seat.id,
			DisplayName: seat.id, CorrelationID: roundtableRouteCorrelation,
			Selection: &roundtable.SeatBindingRequest{
				AgentDefinitionID: "agent." + seat.id, TeamRoleKind: seat.role,
				RuntimeProfileID: "profile." + seat.id,
			},
		}))
	}
	opened, err := fixture.authority.OpenRound(context.Background(), roundtable.OpenRoundCommand{
		SessionID: sessionID, RoundID: "round-1", ModeratorSeat: "seat-moderator",
		EmittedAt:     time.Date(2026, 8, 16, 12, 2, 30, 0, time.UTC),
		CorrelationID: roundtableRouteCorrelation,
	})
	if err != nil {
		t.Fatal(err)
	}
	binding := opened.Seats["seat-main"].Binding
	if binding == nil {
		t.Fatal("main seat binding missing")
	}
	started, err := fixture.authority.StartSeatAttempt(context.Background(), roundtable.StartSeatAttemptCommand{
		SessionID: sessionID, RoundID: "round-1", SeatID: "seat-main",
		AttemptID: "attempt-steer-delayed-ready", AttemptNumber: 1,
		ExecutionTeamID: "execution-team-steer-ready", WorkItemID: "work-steer-ready",
		RunID: "run-steer-ready", SegmentID: "segment-steer-ready", ClaimGeneration: 1,
		RuntimeInstanceID:  binding.ExecutionBinding.RuntimeInstanceID,
		AgentInstanceID:    "agent-instance-steer-ready",
		MembershipRevision: binding.MembershipRevision, SeatBindingDigest: binding.BindingDigest,
		ExecutionBindingDigest: binding.ExecutionBinding.BindingDigest,
		ContextCapsuleDigest:   strings.Repeat("d", 64),
		EmittedAt:              time.Date(2026, 8, 16, 12, 2, 31, 0, time.UTC),
		CorrelationID:          roundtableRouteCorrelation,
	})
	if err != nil {
		t.Fatal(err)
	}
	attempt := started.Attempts["attempt-steer-delayed-ready"]
	fixture.controller.now = func() time.Time {
		return time.Date(2026, 8, 16, 12, 2, 32, 0, time.UTC)
	}
	input := &productRoundtableDelayedAgentInput{failures: 2}
	if err := fixture.controller.SetAgentInput(input); err != nil {
		t.Fatal(err)
	}
	fixture.controller.inputReadyDelay = time.Millisecond
	fixture.controller.inputReadyWait = 100 * time.Millisecond
	guidanceText := "Keep the exact running Attempt and frozen route while input becomes ready."
	guidance := []byte(guidanceText)
	view, err := fixture.controller.SteerSeat(context.Background(), productRoundtableSteerSeatCommand{
		SessionID: sessionID, RoundID: "round-1", InterventionID: "steer-delayed-ready",
		ModeratorSeat: "seat-moderator", SeatID: "seat-main", AttemptID: attempt.AttemptID,
		Guidance: guidance, CorrelationID: roundtableRouteCorrelation,
	})
	if err != nil {
		t.Fatal(err)
	}
	input.mu.Lock()
	calls, content := input.calls, input.content
	input.mu.Unlock()
	receipt := view.Interventions["steer-delayed-ready"]
	if calls != 3 || content != guidanceText || receipt.InputID != "input-steer-delayed-ready" ||
		receipt.AttemptID != attempt.AttemptID {
		t.Fatalf("delayed steer calls=%d content=%q receipt=%#v", calls, content, receipt)
	}
	replayGuidance := []byte(guidanceText)
	replayed, err := fixture.controller.SteerSeat(
		context.Background(), productRoundtableSteerSeatCommand{
			SessionID: sessionID, RoundID: "round-1",
			InterventionID: "steer-delayed-ready", ModeratorSeat: "seat-moderator",
			SeatID: "seat-main", AttemptID: attempt.AttemptID,
			Guidance: replayGuidance, CorrelationID: roundtableRouteCorrelation,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	input.mu.Lock()
	replayCalls := input.calls
	input.mu.Unlock()
	if replayCalls != calls ||
		replayed.Interventions["steer-delayed-ready"] != receipt {
		t.Fatalf(
			"steer replay repeated delivery: calls=%d want=%d receipt=%#v",
			replayCalls, calls, replayed.Interventions["steer-delayed-ready"],
		)
	}
	for index, value := range replayGuidance {
		if value != 0 {
			t.Fatalf("replay guidance byte %d was not cleared", index)
		}
	}
	for index, value := range guidance {
		if value != 0 {
			t.Fatalf("guidance byte %d was not cleared", index)
		}
	}
}

func TestProductRoundtablePauseCancelsOnlyRunningSeatAttempts(t *testing.T) {
	sessionContext := roundtable.SessionContext{
		ConversationID: "conversation-roundtable-pause",
		MissionID:      "mission/team-roundtable-pause", TeamID: "team-roundtable-pause",
		TeamVersion: 1, WorkspaceID: "project-roundtable-pause",
	}
	fixture := newProductRoundtableRouteFixtureWithResolver(
		t, productRoundtableBindingResolverFixture{context: sessionContext},
	)
	runner := &productRoundtableBlockingRunner{started: make(chan struct{})}
	store := &productRoundtableExecutionStoreFixture{}
	execution, err := newProductRoundtableExecution(
		fixture.authority, runner, store, store, nil,
		productRoundtableMissionContextSourceFixture{}, t.TempDir(),
		func() time.Time { return time.Date(2026, 8, 16, 12, 2, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = execution.Close() })
	if err := fixture.controller.SetExecution(execution); err != nil {
		t.Fatal(err)
	}
	fixture.decodeView(t, fixture.call(t, "roundtable_session_create", productRoundtableSessionCreateParams{
		SchemaVersion: 1, SessionID: "session-pause", ModeratorSeat: "seat-moderator",
		Title: "Pause governed execution", CorrelationID: roundtableRouteCorrelation,
		Link: &roundtable.SessionLinkRequest{
			ConversationID: sessionContext.ConversationID, MissionID: sessionContext.MissionID,
			TeamInstanceID: sessionContext.TeamID,
		},
	}))
	for _, seat := range []struct{ id, role string }{{"seat-main", "main"}, {"seat-peer", "subagent"}} {
		fixture.decodeView(t, fixture.call(t, "roundtable_add_seat", productRoundtableAddSeatParams{
			SchemaVersion: 1, SessionID: "session-pause", SeatID: seat.id,
			DisplayName: seat.id, CorrelationID: roundtableRouteCorrelation,
			Selection: &roundtable.SeatBindingRequest{
				AgentDefinitionID: "agent." + seat.id, TeamRoleKind: seat.role,
				RuntimeProfileID: "profile." + seat.id,
			},
		}))
	}
	fixture.decodeView(t, fixture.call(t, "roundtable_open_round", productRoundtableOpenRoundParams{
		SchemaVersion: 1, SessionID: "session-pause", RoundID: "round-1",
		ModeratorSeat: "seat-moderator", Prompt: "Pause this governed discussion.",
		CorrelationID: roundtableRouteCorrelation,
	}))
	select {
	case <-runner.started:
	case <-time.After(time.Second):
		t.Fatal("RoundTable runner did not start")
	}
	pauseResponse := fixture.call(t, "roundtable_pause_round", productRoundtablePauseRoundParams{
		SchemaVersion: 1, SessionID: "session-pause", RoundID: "round-1",
		InterventionID: "intervention-pause-1", ModeratorSeat: "seat-moderator",
		CorrelationID: roundtableRouteCorrelation,
	})
	if !pauseResponse.OK {
		t.Fatalf("pause response error = %#v", pauseResponse.Error)
	}
	paused := fixture.decodeView(t, pauseResponse)
	if !paused.Rounds[0].PauseRequested {
		t.Fatalf("pause projection = %#v", paused.Rounds[0])
	}
	deadline := time.Now().Add(2 * time.Second)
	allCancelled := false
	for time.Now().Before(deadline) {
		paused, err = fixture.controller.ReadView(context.Background(), "session-pause")
		if err != nil {
			t.Fatal(err)
		}
		cancelled := 0
		for _, attempt := range paused.Attempts {
			if attempt.Status == roundtable.SeatAttemptCancelled {
				cancelled++
			}
		}
		if cancelled == 2 {
			allCancelled = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !allCancelled {
		t.Fatalf("running Attempts were not cancelled: %#v", paused.Attempts)
	}
	var cancelledMain roundtable.SeatAttempt
	for _, attempt := range paused.Attempts {
		if attempt.SeatID == "seat-main" {
			cancelledMain = attempt
			break
		}
	}
	if cancelledMain.AttemptID == "" {
		t.Fatal("cancelled main Attempt unavailable")
	}
	if _, err := fixture.authority.RecordSeatRetry(context.Background(), roundtable.RecordSeatRetryCommand{
		SessionID: "session-pause", RoundID: "round-1",
		InterventionID: "intervention-retry-precommitted", ModeratorSeat: "seat-moderator",
		SeatID: "seat-main", AttemptID: cancelledMain.AttemptID,
		RequestedAttemptNumber:     2,
		ExpectedMembershipRevision: cancelledMain.MembershipRevision,
		ExpectedSeatBindingDigest:  cancelledMain.SeatBindingDigest,
		EmittedAt:                  fixture.controller.stamp().Add(time.Nanosecond),
		CorrelationID:              roundtableRouteCorrelation,
	}); err != nil {
		t.Fatal(err)
	}
	retryResponse := fixture.call(t, "roundtable_retry_seat", productRoundtableRetrySeatParams{
		SchemaVersion: 1, SessionID: "session-pause", RoundID: "round-1",
		InterventionID: "intervention-retry-precommitted", ModeratorSeat: "seat-moderator",
		SeatID: "seat-main", AttemptID: cancelledMain.AttemptID,
		ExpectedMembershipRevision: cancelledMain.MembershipRevision,
		ExpectedSeatBindingDigest:  cancelledMain.SeatBindingDigest,
		Guidance:                   "Resume only this seat after the explicit pause.",
		CorrelationID:              roundtableRouteCorrelation,
	})
	if !retryResponse.OK {
		t.Fatalf("paused retry response error = %#v", retryResponse.Error)
	}
	retried := fixture.decodeView(t, retryResponse)
	runningMain := 0
	cancelledPeer := 0
	retryFacts := 0
	for _, attempt := range retried.Attempts {
		if attempt.SeatID == "seat-main" && attempt.AttemptNumber == 2 &&
			attempt.Status == roundtable.SeatAttemptRunning {
			runningMain++
		}
		if attempt.SeatID == "seat-peer" && attempt.Status == roundtable.SeatAttemptCancelled {
			cancelledPeer++
		}
	}
	for _, intervention := range retried.Interventions {
		if intervention.Kind == roundtable.InterventionRetrySeat &&
			intervention.SeatID == "seat-main" && intervention.RequestedAttemptNumber == 2 {
			retryFacts++
		}
	}
	if !retried.Rounds[0].PauseRequested || runningMain != 1 || cancelledPeer != 1 || retryFacts != 1 {
		t.Fatalf("paused single-seat retry = %#v %#v", retried.Attempts, retried.Interventions)
	}
}

func TestProductRoundtableStartRoundCancelsAttemptsStartedBeforeLaterSeatConflict(t *testing.T) {
	sessionContext := roundtable.SessionContext{
		ConversationID: "conversation-roundtable-partial-start",
		MissionID:      "mission/team-roundtable-partial-start",
		TeamID:         "team-roundtable-partial-start", TeamVersion: 1,
		WorkspaceID: "project-roundtable-partial-start",
	}
	fixture := newProductRoundtableRouteFixtureWithResolver(
		t, productRoundtableBindingResolverFixture{context: sessionContext},
	)
	const sessionID = "session-partial-start"
	fixture.decodeView(t, fixture.call(t, "roundtable_session_create", productRoundtableSessionCreateParams{
		SchemaVersion: 1, SessionID: sessionID, ModeratorSeat: "seat-moderator",
		Title: "Partial start cleanup", CorrelationID: roundtableRouteCorrelation,
		Link: &roundtable.SessionLinkRequest{
			ConversationID: sessionContext.ConversationID, MissionID: sessionContext.MissionID,
			TeamInstanceID: sessionContext.TeamID,
		},
	}))
	for _, seat := range []struct{ id, role string }{{"seat-main", "main"}, {"seat-peer", "subagent"}} {
		fixture.decodeView(t, fixture.call(t, "roundtable_add_seat", productRoundtableAddSeatParams{
			SchemaVersion: 1, SessionID: sessionID, SeatID: seat.id,
			DisplayName: seat.id, CorrelationID: roundtableRouteCorrelation,
			Selection: &roundtable.SeatBindingRequest{
				AgentDefinitionID: "agent." + seat.id, TeamRoleKind: seat.role,
				RuntimeProfileID: "profile." + seat.id,
			},
		}))
	}
	opened, err := fixture.authority.OpenRound(context.Background(), roundtable.OpenRoundCommand{
		SessionID: sessionID, RoundID: "round-1", ModeratorSeat: "seat-moderator",
		EmittedAt:     time.Date(2026, 8, 16, 12, 0, 30, 0, time.UTC),
		CorrelationID: roundtableRouteCorrelation,
	})
	if err != nil {
		t.Fatal(err)
	}
	peer := opened.Seats["seat-peer"]
	if peer.Binding == nil {
		t.Fatal("peer binding missing")
	}
	_, err = fixture.authority.StartSeatAttempt(context.Background(), roundtable.StartSeatAttemptCommand{
		SessionID: sessionID, RoundID: "round-1", SeatID: "seat-peer",
		AttemptID: "attempt-peer-preexisting", AttemptNumber: 1,
		ExecutionTeamID: "execution-team-preexisting", WorkItemID: "work-preexisting",
		RunID: "run-preexisting", SegmentID: "segment-preexisting", ClaimGeneration: 1,
		RuntimeInstanceID:      peer.Binding.ExecutionBinding.RuntimeInstanceID,
		AgentInstanceID:        "agent-instance-preexisting",
		MembershipRevision:     peer.Binding.MembershipRevision,
		SeatBindingDigest:      peer.Binding.BindingDigest,
		ExecutionBindingDigest: peer.Binding.ExecutionBinding.BindingDigest,
		ContextCapsuleDigest:   strings.Repeat("c", 64),
		EmittedAt:              time.Date(2026, 8, 16, 12, 0, 31, 0, time.UTC),
		CorrelationID:          roundtableRouteCorrelation,
	})
	if err != nil {
		t.Fatal(err)
	}
	runner := &productRoundtableUnexpectedRunner{}
	store := &productRoundtableExecutionStoreFixture{}
	execution, err := newProductRoundtableExecution(
		fixture.authority, runner, store, store, nil,
		productRoundtableMissionContextSourceFixture{}, t.TempDir(),
		func() time.Time { return time.Date(2026, 8, 16, 12, 1, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = execution.Close() })
	if _, err = execution.StartRound(
		context.Background(), opened, "Trigger a controlled later-seat conflict.",
		roundtableRouteCorrelation,
	); err == nil {
		t.Fatal("partial start unexpectedly succeeded")
	}
	projected, err := fixture.authority.ReadView(context.Background(), sessionID)
	if err != nil {
		t.Fatal(err)
	}
	running, cancelled := 0, 0
	for _, attempt := range projected.Attempts {
		switch attempt.Status {
		case roundtable.SeatAttemptRunning:
			running++
		case roundtable.SeatAttemptCancelled:
			cancelled++
		}
	}
	if running != 1 || cancelled != 1 ||
		projected.Attempts["attempt-peer-preexisting"].Status != roundtable.SeatAttemptRunning {
		t.Fatalf("partial-start attempts = %#v", projected.Attempts)
	}
	runner.mu.Lock()
	called := runner.called
	runner.mu.Unlock()
	if called {
		t.Fatal("provider runner started after partial attempt admission")
	}
}

func TestProductRoundtableReplaceSeatRouteRetryIsIdempotent(t *testing.T) {
	sessionContext := roundtable.SessionContext{
		ConversationID: "conversation-roundtable-replace",
		MissionID:      "mission/team-roundtable-replace", TeamID: "team-roundtable-replace",
		TeamVersion: 1, WorkspaceID: "project-roundtable-replace",
	}
	fixture := newProductRoundtableRouteFixtureWithResolver(
		t, productRoundtableBindingResolverFixture{context: sessionContext},
	)
	const sessionID = "session-replace-idempotent"
	fixture.decodeView(t, fixture.call(t, "roundtable_session_create", productRoundtableSessionCreateParams{
		SchemaVersion: 1, SessionID: sessionID, ModeratorSeat: "seat-moderator",
		Title: "Replace retry", CorrelationID: roundtableRouteCorrelation,
		Link: &roundtable.SessionLinkRequest{
			ConversationID: sessionContext.ConversationID, MissionID: sessionContext.MissionID,
			TeamInstanceID: sessionContext.TeamID,
		},
	}))
	for _, seat := range []struct{ id, role string }{{"seat-main", "main"}, {"seat-peer", "subagent"}} {
		fixture.decodeView(t, fixture.call(t, "roundtable_add_seat", productRoundtableAddSeatParams{
			SchemaVersion: 1, SessionID: sessionID, SeatID: seat.id,
			DisplayName: seat.id, CorrelationID: roundtableRouteCorrelation,
			Selection: &roundtable.SeatBindingRequest{
				AgentDefinitionID: "agent." + seat.id, TeamRoleKind: seat.role,
				RuntimeProfileID: "profile." + seat.id,
			},
		}))
	}
	if _, err := fixture.authority.OpenRound(context.Background(), roundtable.OpenRoundCommand{
		SessionID: sessionID, RoundID: "round-1", ModeratorSeat: "seat-moderator",
		EmittedAt:     time.Date(2026, 8, 16, 12, 0, 30, 0, time.UTC),
		CorrelationID: roundtableRouteCorrelation,
	}); err != nil {
		t.Fatal(err)
	}
	request := productRoundtableReplaceSeatParams{
		SchemaVersion: 1, SessionID: sessionID, RoundID: "round-1",
		InterventionID: "intervention-replace-idempotent", ModeratorSeat: "seat-moderator",
		SeatID: "seat-peer", DisplayName: "Replacement peer",
		Selection: roundtable.SeatBindingRequest{
			AgentDefinitionID: "agent.replacement", TeamRoleKind: "subagent",
			RuntimeProfileID: "profile.replacement",
		},
		CorrelationID: roundtableRouteCorrelation,
	}
	first := fixture.decodeView(t, fixture.call(t, "roundtable_replace_seat", request))
	second := fixture.decodeView(t, fixture.call(t, "roundtable_replace_seat", request))
	if second.Digest != first.Digest ||
		second.Seats["seat-peer"].Binding.MembershipRevision != 2 {
		t.Fatalf("replace retry = %#v, first digest = %q", second.Seats["seat-peer"], first.Digest)
	}
	events, err := fixture.journal.ReadStream(context.Background(), "roundtable/session/"+sessionID)
	if err != nil {
		t.Fatal(err)
	}
	replacements := 0
	for _, event := range events {
		if event.Type == roundtable.FactSeatReplaced {
			replacements++
		}
	}
	if replacements != 1 {
		t.Fatalf("replacement fact count = %d", replacements)
	}
}

func TestProductRoundtableRouteResolvesMissionAndSeatAuthorityServerSide(t *testing.T) {
	sessionContext := roundtable.SessionContext{
		ConversationID: "conversation-roundtable", MissionID: "mission/team-roundtable",
		TeamID: "team-roundtable", TeamVersion: 4, WorkspaceID: "project-roundtable",
	}
	fixture := newProductRoundtableRouteFixtureWithResolver(
		t, productRoundtableBindingResolverFixture{context: sessionContext},
	)
	view := fixture.decodeView(t, fixture.call(t, "roundtable_session_create", productRoundtableSessionCreateParams{
		SchemaVersion: 1, SessionID: "session-linked", ModeratorSeat: "seat-moderator",
		Title: "Mission deliberation", CorrelationID: roundtableRouteCorrelation,
		Link: &roundtable.SessionLinkRequest{
			ConversationID: sessionContext.ConversationID,
			MissionID:      sessionContext.MissionID,
			TeamInstanceID: sessionContext.TeamID,
		},
	}))
	if view.Session.Context == nil || *view.Session.Context != sessionContext {
		t.Fatalf("resolved context = %#v, want %#v", view.Session.Context, sessionContext)
	}
	view = fixture.decodeView(t, fixture.call(t, "roundtable_add_seat", productRoundtableAddSeatParams{
		SchemaVersion: 1, SessionID: "session-linked", SeatID: "seat-planner",
		DisplayName: "Planner", CorrelationID: roundtableRouteCorrelation,
		Selection: &roundtable.SeatBindingRequest{
			AgentDefinitionID: "agent.planner", TeamRoleKind: "main",
			RuntimeProfileID: "profile.planner",
		},
	}))
	seat := view.Seats["seat-planner"]
	if seat.Binding == nil || seat.Binding.AgentDefinitionID != "agent.planner" ||
		seat.Binding.ExecutionBinding.ProviderAccountID != "deepseek.primary" ||
		seat.Binding.ExecutionBinding.CredentialRevision != 3 ||
		seat.Binding.ExecutionBinding.ModelID != "deepseek-chat" {
		t.Fatalf("server-resolved seat = %#v", seat)
	}
	restartedAuthority, err := roundtable.NewAuthority(
		fixture.journal, fixture.evidence,
		func() time.Time { return time.Date(2026, 8, 16, 12, 5, 0, 0, time.UTC) },
		productRoundtableReader{},
	)
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := newProductRoundtableController(
		restartedAuthority,
		func() time.Time { return time.Date(2026, 8, 16, 12, 5, 0, 0, time.UTC) },
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := restarted.ReadView(context.Background(), "session-linked")
	if err != nil || replayed.Digest != view.Digest ||
		replayed.Session.Context == nil ||
		replayed.Seats["seat-planner"].Binding == nil ||
		replayed.Seats["seat-planner"].Binding.BindingDigest != seat.Binding.BindingDigest {
		t.Fatalf("restart replay = %#v err=%v", replayed, err)
	}

	// Full bindings are never client-authored. Exact decoding rejects an
	// invented Provider or credential payload before Authority is reached.
	invented := fixture.call(t, "roundtable_add_seat", map[string]any{
		"schema_version": 1, "session_id": "session-linked",
		"seat_id": "seat-invented", "display_name": "Invented",
		"correlation_id": roundtableRouteCorrelation,
		"binding":        map[string]any{"provider_account_id": "attacker.account"},
	})
	if invented.OK || invented.Error == nil || invented.Error.Code != "invalid_request" {
		t.Fatalf("client-authored binding response = %#v", invented)
	}
}

func TestProductRoundtableRouteFullLifecycle(t *testing.T) {
	fixture := newProductRoundtableRouteFixture(t)
	params := productRoundtableSessionCreateParams{
		SchemaVersion: 1, SessionID: "session-route", ModeratorSeat: "seat-moderator",
		Title: "Governed diagnosis handoff", CorrelationID: roundtableRouteCorrelation,
	}
	view := fixture.decodeView(t, fixture.call(t, "roundtable_session_create", params))
	if view.Session.ModeratorSeat != "seat-moderator" || len(view.Seats) != 1 {
		t.Fatalf("session view = %#v", view)
	}
	view = fixture.decodeView(t, fixture.call(t, "roundtable_add_seat", productRoundtableAddSeatParams{
		SchemaVersion: 1, SessionID: "session-route", SeatID: "seat-writer",
		DisplayName: "Writer Seat", CorrelationID: roundtableRouteCorrelation,
	}))
	view = fixture.decodeView(t, fixture.call(t, "roundtable_add_seat", productRoundtableAddSeatParams{
		SchemaVersion: 1, SessionID: "session-route", SeatID: "seat-target",
		DisplayName: "Target Seat", CorrelationID: roundtableRouteCorrelation,
	}))
	if len(view.Seats) != 3 {
		t.Fatalf("seats = %#v", view.Seats)
	}
	fixture.decodeView(t, fixture.call(t, "roundtable_open_round", productRoundtableOpenRoundParams{
		SchemaVersion: 1, SessionID: "session-route", RoundID: "round-1",
		ModeratorSeat: "seat-moderator", CorrelationID: roundtableRouteCorrelation,
	}))
	view = fixture.decodeView(t, fixture.call(t, "roundtable_propose_message", productRoundtableProposeMessageParams{
		SchemaVersion: 1, SessionID: "session-route", RoundID: "round-1",
		MessageID: "msg-route", WriterSeat: "seat-writer", TargetSeat: "seat-target",
		Body: "Diagnosis: the route is over-constrained.", ArtifactRefs: []string{},
		CorrelationID: roundtableRouteCorrelation,
	}))
	if view.Messages["msg-route"].Status != roundtable.MessagePending {
		t.Fatalf("proposed status = %q", view.Messages["msg-route"].Status)
	}
	// Non-moderator relay is rejected with a typed code.
	rejected := fixture.call(t, "roundtable_relay_message", productRoundtableRelayMessageParams{
		SchemaVersion: 1, SessionID: "session-route", MessageID: "msg-route",
		ModeratorSeat: "seat-writer", CorrelationID: roundtableRouteCorrelation,
	})
	if rejected.OK || rejected.Error == nil || rejected.Error.Code != "not_moderator" {
		t.Fatalf("non-moderator relay code=%q msg=%q stage=%q recoverable=%t response=%#v",
			rejected.Error.Code, rejected.Error.Message, rejected.Error.Stage,
			rejected.Error.Recoverable, rejected)
	}
	view = fixture.decodeView(t, fixture.call(t, "roundtable_relay_message", productRoundtableRelayMessageParams{
		SchemaVersion: 1, SessionID: "session-route", MessageID: "msg-route",
		ModeratorSeat: "seat-moderator", CorrelationID: roundtableRouteCorrelation,
	}))
	if view.Messages["msg-route"].Status != roundtable.MessageRelayed {
		t.Fatalf("relayed status = %q", view.Messages["msg-route"].Status)
	}
	wrongAck := fixture.call(t, "roundtable_ack_message", productRoundtableAckMessageParams{
		SchemaVersion: 1, SessionID: "session-route", MessageID: "msg-route",
		SeatID: "seat-writer", CorrelationID: roundtableRouteCorrelation,
	})
	if wrongAck.OK || wrongAck.Error == nil || wrongAck.Error.Code != "not_found" {
		t.Fatalf("wrong-seat ack = %#v", wrongAck)
	}
	view = fixture.decodeView(t, fixture.call(t, "roundtable_ack_message", productRoundtableAckMessageParams{
		SchemaVersion: 1, SessionID: "session-route", MessageID: "msg-route",
		SeatID: "seat-target", CorrelationID: roundtableRouteCorrelation,
	}))
	if view.Messages["msg-route"].Status != roundtable.MessageAcknowledged {
		t.Fatalf("ack status = %q", view.Messages["msg-route"].Status)
	}
	view = fixture.decodeView(t, fixture.call(t, "roundtable_insert_message", productRoundtableInsertMessageParams{
		SchemaVersion: 1, SessionID: "session-route", MessageID: "msg-route",
		ModeratorSeat: "seat-moderator", CorrelationID: roundtableRouteCorrelation,
	}))
	if view.Messages["msg-route"].Status != roundtable.MessageInserted {
		t.Fatalf("insert status = %q", view.Messages["msg-route"].Status)
	}
	snapshot := fixture.decodeView(t, fixture.call(t, "roundtable_snapshot", productRoundtableSnapshotParams{
		SchemaVersion: 1, SessionID: "session-route",
	}))
	if snapshot.Digest != view.Digest ||
		snapshot.Messages["msg-route"].Status != roundtable.MessageInserted {
		t.Fatalf("snapshot mismatch digest=%q vs %q", snapshot.Digest, view.Digest)
	}
	concluded := fixture.decodeView(t, fixture.call(t, "roundtable_conclude", productRoundtableConcludeParams{
		SchemaVersion: 1, SessionID: "session-route",
		ModeratorSeat: "seat-moderator", CorrelationID: roundtableRouteCorrelation,
	}))
	if !concluded.Session.Concluded {
		t.Fatal("session not concluded")
	}
	exportResponse := fixture.call(t, "roundtable_export", productRoundtableExportParams{
		SchemaVersion: 1, SessionID: "session-route", CorrelationID: roundtableRouteCorrelation,
	})
	if !exportResponse.OK {
		t.Fatalf("export response = %#v", exportResponse.Error)
	}
	var exported roundtable.ExportDocumentResult
	if err := json.Unmarshal(exportResponse.Result, &exported); err != nil ||
		len(exported.Document) == 0 || exported.Export.Digest == "" {
		t.Fatalf("exported document = %#v err=%v", exported, err)
	}
	importResponse := fixture.call(t, "roundtable_import", productRoundtableImportParams{
		SchemaVersion: 1, Document: exported.Document, CorrelationID: roundtableRouteCorrelation,
	})
	if !importResponse.OK {
		t.Fatalf("import response = %#v", importResponse.Error)
	}
	var imported roundtable.ImportResult
	if err := json.Unmarshal(importResponse.Result, &imported); err != nil ||
		imported.ContractDigest != exported.Export.Digest || imported.View.Session.ID != "session-route" {
		t.Fatalf("imported document = %#v err=%v", imported, err)
	}
	// Post-conclude writes are rejected.
	after := fixture.call(t, "roundtable_propose_message", productRoundtableProposeMessageParams{
		SchemaVersion: 1, SessionID: "session-route", RoundID: "round-1",
		MessageID: "msg-after", WriterSeat: "seat-writer", TargetSeat: "seat-target",
		Body: "too late", ArtifactRefs: []string{},
		CorrelationID: roundtableRouteCorrelation,
	})
	if after.OK || after.Error == nil || after.Error.Code != "concluded" {
		t.Fatalf("post-conclude write = %#v", after)
	}
	// Strict decode: unknown field and bad schema version are invalid_request.
	invalid := fixture.call(t, "roundtable_session_create", map[string]any{
		"schema_version": 1, "session_id": "session-bad",
		"moderator_seat": "seat-moderator", "title": "bad",
		"correlation_id": roundtableRouteCorrelation, "extra": true,
	})
	if invalid.OK || invalid.Error == nil || invalid.Error.Code != "invalid_request" ||
		invalid.Error.Stage != "input_admission" {
		t.Fatalf("unknown-field response = %#v", invalid)
	}
	badVersion := fixture.call(t, "roundtable_snapshot", productRoundtableSnapshotParams{
		SchemaVersion: 2, SessionID: "session-route",
	})
	if badVersion.OK || badVersion.Error == nil || badVersion.Error.Code != "invalid_request" {
		t.Fatalf("bad version response = %#v", badVersion)
	}
	missing := fixture.call(t, "roundtable_snapshot", productRoundtableSnapshotParams{
		SchemaVersion: 1, SessionID: "session-missing",
	})
	if missing.OK || missing.Error == nil || missing.Error.Code != "not_found" ||
		missing.Error.Stage != "daemon_admission" {
		t.Fatalf("missing session response = %#v", missing)
	}
}

func TestProductRoundtableControllerNormalizesProductionClockToUTC(t *testing.T) {
	fixture := newProductRoundtableRouteFixture(t)
	fixture.controller.now = func() time.Time {
		return time.Date(
			2026, 8, 26, 17, 52, 0, 0,
			time.FixedZone("Asia/Singapore", 8*60*60),
		)
	}
	view := fixture.decodeView(t, fixture.call(
		t, "roundtable_session_create", productRoundtableSessionCreateParams{
			SchemaVersion: 1, SessionID: "session-local-clock",
			ModeratorSeat: "seat-moderator", Title: "Local clock admission",
			CorrelationID: roundtableRouteCorrelation,
		},
	))
	if view.Session.CreatedAt.Location() != time.UTC {
		t.Fatalf("created_at location = %v, want UTC", view.Session.CreatedAt.Location())
	}
}

func TestProductRoundtableRouteClassifiesUnresolvableMissionLink(t *testing.T) {
	fixture := newProductRoundtableRouteFixture(t)
	response := fixture.call(t, "roundtable_session_create", productRoundtableSessionCreateParams{
		SchemaVersion: 1, SessionID: "session-unresolvable", ModeratorSeat: "seat-moderator",
		Title: "Mission deliberation", CorrelationID: roundtableRouteCorrelation,
		Link: &roundtable.SessionLinkRequest{
			ConversationID: "conversation-1", MissionID: "mission/team-1",
			TeamInstanceID: "team-1",
		},
	})
	if response.OK || response.Error == nil || response.Error.Code != "invalid_request" ||
		response.Error.Stage != "daemon_admission" {
		t.Fatalf("unresolvable Mission link response = %#v", response)
	}
}

func TestProductRoundtableRouteRejoinsRetiredSeatBeforeRound(t *testing.T) {
	fixture := newProductRoundtableRouteFixture(t)
	const sessionID = "session-route-rejoin"
	const seatID = "agent-reviewer"

	fixture.decodeView(t, fixture.call(t, "roundtable_session_create", productRoundtableSessionCreateParams{
		SchemaVersion: 1, SessionID: sessionID, ModeratorSeat: "seat-moderator",
		Title: "Rejoin Agent", CorrelationID: roundtableRouteCorrelation,
	}))
	add := productRoundtableAddSeatParams{
		SchemaVersion: 1, SessionID: sessionID, SeatID: seatID,
		DisplayName: "Reviewer", CorrelationID: roundtableRouteCorrelation,
	}
	fixture.decodeView(t, fixture.call(t, "roundtable_add_seat", add))
	retired := fixture.decodeView(t, fixture.call(t, "roundtable_retire_seat", productRoundtableRetireSeatParams{
		SchemaVersion: 1, SessionID: sessionID, SeatID: seatID,
		ModeratorSeat: "seat-moderator", CorrelationID: roundtableRouteCorrelation,
	}))
	if retired.Seats[seatID].Available {
		t.Fatalf("retired seat remained active: %#v", retired.Seats[seatID])
	}
	rejoined := fixture.decodeView(t, fixture.call(t, "roundtable_add_seat", add))
	if !rejoined.Seats[seatID].Available || len(rejoined.Rounds) != 0 {
		t.Fatalf("rejoined view = %#v", rejoined)
	}
	// Add is shared by the workbench button and drop destination. A retry must
	// return the same authoritative view without appending another fact.
	retried := fixture.decodeView(t, fixture.call(t, "roundtable_add_seat", add))
	if retried.Digest != rejoined.Digest {
		t.Fatalf("retry digest = %q, want %q", retried.Digest, rejoined.Digest)
	}
	events, err := fixture.journal.ReadStream(context.Background(), "roundtable/session/"+sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 4 || events[3].Type != roundtable.FactSeatRejoined {
		t.Fatalf("route facts = %#v", events)
	}
}

func TestProductRoundtableRouteNilServiceIsStateUnavailable(t *testing.T) {
	handler := newProductRouteHandler(productRouteServices{})
	params, err := json.Marshal(productRoundtableSnapshotParams{
		SchemaVersion: 1, SessionID: "session-any",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "roundtable-nil-service", Method: "roundtable_snapshot",
		Params: params,
	})
	if response.OK || response.Error == nil || response.Error.Code != "state_unavailable" {
		t.Fatalf("nil service response = %#v", response)
	}
}

func TestProductRoundtableSteerPreservesAgentInputFailureStage(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		code        string
		recoverable bool
	}{
		{name: "invalid", err: errProductInvalidAgentInput, code: "invalid_request"},
		{name: "not ready", err: errProductActiveAttemptNotFound, code: "stale_generation"},
		{name: "unsupported", err: errProductAgentInputUnsupported, code: "capability_gap"},
		{name: "conflict", err: errProductAgentInputConflict, code: "conflict", recoverable: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := productRoundtableServiceError(test.err)
			if response.OK || response.Error == nil || response.Error.Code != test.code ||
				response.Error.Stage != productAgentInputStage ||
				response.Error.Recoverable != test.recoverable {
				t.Fatalf("Agent input response = %#v", response)
			}
		})
	}
}

func TestProductRoundtableRouteDegradedRuntimeIsStateUnavailable(t *testing.T) {
	runtimeSlot := &productAgentRuntimeRouteSlot{}
	cause := newProductAgentRuntimeBuildError(
		"mission_resume", errors.New("private startup detail"),
	)
	if err := runtimeSlot.BindUnavailable(
		productDegradedAgentRuntimeRoutes(productSavedTeamMaterializerFixture{}),
		cause,
	); err != nil {
		t.Fatal(err)
	}
	handler := newProductRouteHandler(productRouteServices{roundtable: runtimeSlot})
	params, err := json.Marshal(productRoundtableSnapshotParams{
		SchemaVersion: 1, SessionID: "session-any",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "roundtable-degraded-runtime",
		Method: "roundtable_snapshot", Params: params,
	})
	if response.OK || response.Error == nil ||
		response.Error.Code != "state_unavailable" ||
		response.Error.Stage != "daemon_admission" ||
		strings.Contains(response.Error.Message, "private startup detail") {
		t.Fatalf("degraded runtime response = %#v error=%#v", response, response.Error)
	}
}

func TestProductRoundtableRouteInitializingRuntimeIsRecoverable(t *testing.T) {
	runtimeSlot := &productAgentRuntimeRouteSlot{}
	if err := runtimeSlot.BeginInitialization(); err != nil {
		t.Fatal(err)
	}
	handler := newProductRouteHandler(productRouteServices{roundtable: runtimeSlot})
	params, err := json.Marshal(productRoundtableSnapshotParams{
		SchemaVersion: 1, SessionID: "session-any",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "roundtable-initializing-runtime",
		Method: "roundtable_snapshot", Params: params,
	})
	if response.OK || response.Error == nil ||
		response.Error.Code != "state_unavailable" ||
		response.Error.Stage != productAgentRuntimeInitializationStage ||
		!response.Error.Recoverable {
		t.Fatalf("initializing runtime response = %#v error=%#v", response, response.Error)
	}
}

func TestProductRoundtableRouteTraversesAuthenticatedLocalIPC(t *testing.T) {
	fixture := newProductRoundtableRouteFixture(t)
	root, err := os.MkdirTemp("/private/tmp", "loom-roundtable-route-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	server, err := localipc.NewServer(localipc.ServerConfig{
		SocketPath: filepath.Join(root, "loomd.sock"), EffectiveUID: os.Geteuid(),
		BuildID: "roundtable-route-ipc-fixture",
		Handler: localipc.HandlerFunc(fixture.handler),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	select {
	case <-server.Ready():
	case err := <-done:
		t.Fatalf("server failed before ready: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not become ready")
	}
	defer func() {
		cancel()
		if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("server close: %v", err)
		}
	}()
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: filepath.Join(root, "loomd.sock"), Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	var created roundtable.View
	if err := client.Call(
		context.Background(), "roundtable_session_create",
		productRoundtableSessionCreateParams{
			SchemaVersion: 1, SessionID: "session-ipc", ModeratorSeat: "seat-moderator",
			Title: "IPC handoff", CorrelationID: roundtableRouteCorrelation,
		},
		&created,
	); err != nil {
		t.Fatal(err)
	}
	if created.Session.ModeratorSeat != "seat-moderator" || len(created.Seats) != 1 {
		t.Fatalf("created view = %#v", created)
	}
	var concluded roundtable.View
	if err := client.Call(
		context.Background(), "roundtable_conclude",
		productRoundtableConcludeParams{
			SchemaVersion: 1, SessionID: "session-ipc",
			ModeratorSeat: "seat-moderator", CorrelationID: roundtableRouteCorrelation,
		},
		&concluded,
	); err != nil {
		t.Fatal(err)
	}
	if !concluded.Session.Concluded || len(concluded.Seats) != 1 {
		t.Fatalf("concluded view = %#v", concluded)
	}
}

func mustProductRoundtableJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
