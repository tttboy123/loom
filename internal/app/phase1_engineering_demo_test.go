package app_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"loom-pi-rebuild/internal/agents"
	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/authorization"
	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/mode"
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

const (
	demoCorrelation  = "11111111-1111-4111-8111-111111111111"
	demoContract     = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	demoContinuation = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
)

var demoNow = time.Date(2026, 7, 26, 16, 0, 0, 0, time.UTC)

type demoClock struct {
	mu  sync.Mutex
	now time.Time
}

func (clock *demoClock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return clock.now
}

func (clock *demoClock) Set(now time.Time) {
	clock.mu.Lock()
	clock.now = now
	clock.mu.Unlock()
}

type demoBarrier struct {
	active    atomic.Int32
	maxActive atomic.Int32
	subCount  atomic.Int32
	both      chan struct{}
	subBDone  chan struct{}
	bothOnce  sync.Once
	doneOnce  sync.Once
}

func newDemoBarrier() *demoBarrier {
	return &demoBarrier{
		both:     make(chan struct{}),
		subBDone: make(chan struct{}),
	}
}

func (barrier *demoBarrier) enterSub(
	ctx context.Context,
	logicalNodeID string,
) error {
	active := barrier.active.Add(1)
	for {
		maximum := barrier.maxActive.Load()
		if active <= maximum ||
			barrier.maxActive.CompareAndSwap(maximum, active) {
			break
		}
	}
	if barrier.subCount.Add(1) == 2 {
		barrier.bothOnce.Do(func() { close(barrier.both) })
	}
	select {
	case <-barrier.both:
	case <-ctx.Done():
		barrier.active.Add(-1)
		return ctx.Err()
	}
	if logicalNodeID == "sub-a" {
		select {
		case <-barrier.subBDone:
		case <-ctx.Done():
			barrier.active.Add(-1)
			return ctx.Err()
		}
	}
	return nil
}

func (barrier *demoBarrier) leaveSub(logicalNodeID string) {
	if logicalNodeID == "sub-b" {
		barrier.doneOnce.Do(func() { close(barrier.subBDone) })
	}
	barrier.active.Add(-1)
}

type demoAdapter struct {
	barrier         *demoBarrier
	logicalNodeID   string
	runtimeID       string
	terminalStatus  string
	terminalReason  string
	calls           *atomic.Int32
	effectCalls     *atomic.Int32
	effectOnSuccess bool
	staleGeneration bool
}

func (adapter *demoAdapter) AdapterType() string { return "pi" }
func (adapter *demoAdapter) RuntimeInstanceID() string {
	return adapter.runtimeID
}

func (adapter *demoAdapter) Execute(
	ctx context.Context,
	request supervisor.AdapterRequest,
) (supervisor.AdapterResult, error) {
	if adapter.calls != nil {
		adapter.calls.Add(1)
	}
	isSub := strings.HasPrefix(adapter.logicalNodeID, "sub-")
	if isSub {
		if err := adapter.barrier.enterSub(ctx, adapter.logicalNodeID); err != nil {
			return supervisor.AdapterResult{}, err
		}
		defer adapter.barrier.leaveSub(adapter.logicalNodeID)
	}
	status := adapter.terminalStatus
	if status == "" {
		status = "succeeded"
	}
	reason := adapter.terminalReason
	if status == "succeeded" {
		reason = ""
		if adapter.effectOnSuccess && adapter.effectCalls != nil {
			adapter.effectCalls.Add(1)
		}
	} else if reason == "" {
		reason = "controlled_failure"
	}
	frames := []bridgev1.Frame{
		demoInboundFrame(request, 2, bridgev1.MessageAck, map[string]string{
			"message_id": request.Dispatch.MessageID(),
		}, adapter.staleGeneration),
		demoInboundFrame(request, 3, bridgev1.MessageEvent, map[string]string{
			"delta": "authorized-" + adapter.logicalNodeID,
		}, adapter.staleGeneration),
		demoInboundFrame(request, 4, bridgev1.MessageResult, map[string]string{
			"status": status,
			"reason": reason,
		}, adapter.staleGeneration),
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
	})
}

type demoOutputObserver struct {
	store     *journal.Store
	mu        sync.Mutex
	eventKeys []string
}

func (observer *demoOutputObserver) ObserveNodeOutput(
	ctx context.Context,
	output app.NodeOutput,
) error {
	if !output.Tentative() || !output.AuthorizedFrame().Tentative() {
		return errors.New("non-tentative source output")
	}
	frame := output.AuthorizedFrame().Frame()
	if frame.Type() != bridgev1.MessageEvent {
		return nil
	}
	events, err := observer.store.ReadStream(
		ctx,
		"work-item/"+output.AuthorizedFrame().Binding().WorkItemID,
	)
	if err != nil {
		return err
	}
	for _, event := range events {
		if event.Type == "WorkItemDone" {
			return errors.New("tentative output arrived after authoritative Done")
		}
	}
	observer.mu.Lock()
	observer.eventKeys = append(observer.eventKeys, fmt.Sprintf(
		"%s/%d/%s",
		output.LogicalNodeID(),
		output.AttemptNumber(),
		frame.Type(),
	))
	observer.mu.Unlock()
	return nil
}

type demoEnvironment struct {
	db           *sql.DB
	dbPath       string
	store        *journal.Store
	clock        *demoClock
	work         *work.Authority
	grants       *authorization.Authority
	projection   *projection.Projection
	evidence     *evidence.Store
	evidenceRoot string
	coordinator  *app.TeamCoordinator
}

type demoScenarioResult struct {
	trace          []string
	packageID      string
	evidenceTypes  []string
	observerEvents []string
	maxActive      int32
	sourceCalls    int32
	verifierCalls  int32
	effectCalls    int32
}

func TestPhase1EngineeringDemoWorkPackageParity(t *testing.T) {
	if got := mode.Route(mode.Intent{
		Trigger: mode.TriggerPlainInput,
		Text:    "please do ordinary conversation",
	}); got.Mode != mode.ModeConversation {
		t.Fatalf("plain input mode = %q", got.Mode)
	}
	if got := mode.Route(mode.Intent{
		Trigger:  mode.TriggerSelectTeam,
		TargetID: "saved-team.phase1-engineering",
	}); got.Mode != mode.ModeAgent {
		t.Fatalf("explicit Team mode = %q", got.Mode)
	}

	coding, err := work.CodingWorkPackage()
	if err != nil {
		t.Fatal(err)
	}
	knowledge, err := work.KnowledgeWorkPackage()
	if err != nil {
		t.Fatal(err)
	}
	codingResult := runDemoScenario(t, coding)
	knowledgeResult := runDemoScenario(t, knowledge)
	if !reflect.DeepEqual(codingResult.trace, knowledgeResult.trace) {
		t.Fatalf(
			"normalized authority trace differs:\ncoding=%v\nknowledge=%v",
			codingResult.trace,
			knowledgeResult.trace,
		)
	}
	if codingResult.packageID == knowledgeResult.packageID ||
		reflect.DeepEqual(codingResult.evidenceTypes, knowledgeResult.evidenceTypes) {
		t.Fatal("domain-specific package fields were not distinct")
	}
	for _, result := range []demoScenarioResult{codingResult, knowledgeResult} {
		if result.maxActive != 2 ||
			result.sourceCalls != 4 ||
			result.verifierCalls != 1 ||
			result.effectCalls != 1 {
			t.Fatalf("execution bounds = %#v", result)
		}
		if len(result.observerEvents) != 4 {
			t.Fatalf("source tentative events = %v", result.observerEvents)
		}
		for _, key := range result.observerEvents {
			if strings.Contains(key, "verifier") {
				t.Fatalf("verifier output leaked to source observer: %s", key)
			}
		}
	}
}

func TestPhase1EngineeringDemoApprovalRestartReconnectAndRecovery(t *testing.T) {
	coding, err := work.CodingWorkPackage()
	if err != nil {
		t.Fatal(err)
	}
	testDemoApprovalRestart(t, coding)

	environment := newDemoEnvironment(t, "timeline-recovery")
	plan := demoExecutionPlan(t, coding, "team-timeline-recovery")
	request, barrier, sourceCalls, verifierCalls, effectCalls, observer := demoRequest(
		t,
		environment,
		coding,
		plan,
	)
	first, err := environment.coordinator.Run(context.Background(), request)
	if !errors.Is(err, app.ErrTeamExecutionIncomplete) ||
		first.Team().Status() != "awaiting_recovery" &&
			first.Team().Status() != "running" {
		t.Fatalf("first recovery result = %#v, %v", first, err)
	}
	environment.clock.Set(demoNow.Add(time.Minute))
	request.AuthoritativeTime = demoNow.Add(time.Minute)
	environment.coordinator = runCompetingDemoRecovery(
		t,
		environment,
		request,
		barrier,
		sourceCalls,
		verifierCalls,
		effectCalls,
	)
	before, err := environment.store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	environment.projection = projection.New(environment.db)
	environment.coordinator = mustDemoCoordinator(
		t,
		environment.work,
		environment.grants,
		environment.projection,
		environment.evidence,
	)
	replayed, err := environment.coordinator.Run(context.Background(), request)
	if err != nil || replayed.Team().Status() != "succeeded" {
		t.Fatalf("idempotent restart = %#v, %v", replayed, err)
	}
	after, err := environment.store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) ||
		sourceCalls.Load() != 4 ||
		verifierCalls.Load() != 1 ||
		effectCalls.Load() != 1 ||
		barrier.maxActive.Load() != 2 {
		t.Fatalf(
			"restart duplicated work: Events=%d->%d source=%d verifier=%d effect=%d active=%d",
			len(before),
			len(after),
			sourceCalls.Load(),
			verifierCalls.Load(),
			effectCalls.Load(),
			barrier.maxActive.Load(),
		)
	}
	assertDemoExactOnce(t, after, effectCalls.Load())
	assertDemoTimelineReconnect(t, environment, plan.TeamInstanceID())
	assertDemoStaleCursorRejected(t, environment, plan.TeamInstanceID())
	observer.mu.Lock()
	if len(observer.eventKeys) != 4 {
		t.Fatalf("observer events = %v", observer.eventKeys)
	}
	observer.mu.Unlock()
	assertPrivateTreeModes(t, filepath.Dir(environment.dbPath), environment.evidenceRoot)
}

func TestPhase1EngineeringDemoFailClosedAndPrivateModes(t *testing.T) {
	coding, err := work.CodingWorkPackage()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := work.LoadWorkPackage(
		work.WorkPackageInput{
			ID:                             coding.ID(),
			Version:                        coding.Version(),
			DomainKind:                     coding.DomainKind(),
			RecommendedAgentDefinitionIDs:  coding.RecommendedAgentDefinitionIDs(),
			ToolCategories:                 coding.ToolCategories(),
			DefaultVerifierKey:             coding.DefaultVerifierKey(),
			EvidenceTypes:                  coding.EvidenceTypes(),
			DefaultCustomerRuleTemplateIDs: coding.DefaultCustomerRuleTemplateIDs(),
		},
		strings.Repeat("0", 64),
	); !errors.Is(err, work.ErrWorkPackageDigestMismatch) {
		t.Fatalf("stale WorkPackage error = %v", err)
	}

	environment := newDemoEnvironment(t, "fail-closed")
	plan := demoExecutionPlan(t, coding, "team-fail-closed")
	request, _, _, _, _, _ := demoRequest(t, environment, coding, plan)
	request.Nodes[0].Executor = newDemoSupervisor(
		t,
		environment.work,
		environment.grants,
		&demoAdapter{
			barrier:         newDemoBarrier(),
			logicalNodeID:   request.Nodes[0].LogicalNodeID,
			runtimeID:       request.Nodes[0].Instance.ID,
			staleGeneration: true,
		},
	)
	if _, err := environment.coordinator.Run(
		context.Background(),
		request,
	); err == nil {
		t.Fatal("stale generation Frame was accepted")
	}
	staleWorkItemID := request.Nodes[0].Dispatch.WorkItemID()
	staleRunID := request.Nodes[0].Dispatch.RunID()
	events, err := environment.store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if event.StreamID == "work-item/"+staleWorkItemID &&
			event.Type == "WorkItemDone" ||
			event.Type == "EvidenceSubmitted" &&
				bytes.Contains(event.PayloadJSON, []byte(staleRunID)) {
			t.Fatalf("stale Frame reached authority: %s", event.Type)
		}
	}

	readModel := projection.New(environment.db)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	oldVersion := readModel.GlobalReadView().Version()
	if _, err := environment.store.Append(context.Background(), journal.Event{
		ID:             "corrupt-projection-event",
		StreamID:       "mode/corrupt-demo",
		Seq:            1,
		IdempotencyKey: "corrupt-projection-event",
		Type:           "ModeSelected",
		SchemaVersion:  1,
		EmittedAt:      demoNow.Add(2 * time.Minute),
		CorrelationID:  demoCorrelation,
		PayloadJSON:    []byte(`{}`),
	}); err != nil {
		t.Fatal(err)
	}
	if err := readModel.Rebuild(context.Background()); !errors.Is(
		err,
		projection.ErrInvalidProjectionEvent,
	) {
		t.Fatalf("corrupt Projection error = %v", err)
	}
	if readModel.GlobalReadView().Version() != oldVersion {
		t.Fatal("failed Projection rebuild replaced the last good view")
	}

	stream, err := api.NewTeamExecutionStream(api.TeamExecutionStreamConfig{
		TeamInstanceID: plan.TeamInstanceID(),
		Journal:        environment.store,
		Projection:     readModel,
		Now:            environment.clock.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stream.ReadPage(
		context.Background(),
		"not-a-canonical-cursor",
		1,
	); !errors.Is(err, api.ErrInvalidTimelineCursor) ||
		!errors.Is(err, api.ErrStreamGap) {
		t.Fatalf("malformed cursor error = %v", err)
	}
	assertPrivateTreeModes(t, filepath.Dir(environment.dbPath), environment.evidenceRoot)
}

func TestPhase1LiveGateManifestIsBoundedAndNonExecuting(t *testing.T) {
	manifestPath := filepath.Join(
		"..",
		"..",
		".loom-evidence",
		"phase1-slice5",
		"S5-W2",
		"live-demo-manifest.json",
	)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("live-gate manifest missing: %v", err)
	}
	if len(data) > 8192 || len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatalf("manifest bytes = %d or missing trailing newline", len(data))
	}
	if bytes.Contains(data, []byte("/Users/")) ||
		bytes.Contains(data, []byte("credential")) ||
		bytes.Contains(data, []byte("raw_grant")) ||
		bytes.Contains(data, []byte("hidden_reasoning")) {
		t.Fatalf("manifest contains forbidden private content: %s", data)
	}
	type selectedPackage struct {
		ID      string `json:"id"`
		Version int    `json:"version"`
		Digest  string `json:"digest"`
	}
	type runtimeCapability struct {
		AdapterType           string `json:"adapter_type"`
		BridgeProtocol        string `json:"bridge_protocol"`
		Streaming             bool   `json:"streaming"`
		SessionResumeRequired bool   `json:"session_resume_required"`
		RuntimeInstanceID     string `json:"runtime_instance_id"`
	}
	type boundedTask struct {
		Summary                    string `json:"summary"`
		MaxNodes                   int    `json:"max_nodes"`
		MaxAttemptsPerNode         int    `json:"max_attempts_per_node"`
		NetworkAllowed             bool   `json:"network_allowed"`
		ExternalSideEffectsAllowed bool   `json:"external_side_effects_allowed"`
	}
	var manifest struct {
		SchemaVersion             int               `json:"schema_version"`
		Status                    string            `json:"status"`
		SelectedSavedTeamID       string            `json:"selected_saved_team_id"`
		SelectedWorkPackage       selectedPackage   `json:"selected_work_package"`
		RequiredRuntimeCapability runtimeCapability `json:"required_runtime_capability"`
		BoundedTask               boundedTask       `json:"bounded_task"`
		ApprovalExpectations      []string          `json:"approval_expectations"`
		EvidenceLocation          string            `json:"evidence_location"`
		StopCancelProcedure       []string          `json:"stop_cancel_procedure"`
		RollbackRecoveryChecks    []string          `json:"rollback_recovery_checks"`
		UnresolvedHumanInputs     []string          `json:"unresolved_human_inputs"`
		ExecutionAuthorized       bool              `json:"execution_authorized"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		t.Fatalf("trailing JSON = %v", err)
	}
	if manifest.SchemaVersion != 1 ||
		manifest.Status != "non_executing" ||
		manifest.SelectedSavedTeamID != "saved-team.phase1-engineering" ||
		manifest.SelectedWorkPackage != (selectedPackage{
			ID:      "work-package.coding",
			Version: 1,
			Digest:  "4eea514fca13aa241cd004277e31c9c1fe296d34646ceafd618809b6b22c8a4f",
		}) ||
		manifest.RequiredRuntimeCapability != (runtimeCapability{
			AdapterType:           "pi",
			BridgeProtocol:        "loom.bridge.v1",
			Streaming:             true,
			SessionResumeRequired: false,
			RuntimeInstanceID:     "HUMAN_SELECTION_REQUIRED",
		}) ||
		manifest.BoundedTask != (boundedTask{
			Summary:                    "bounded_local_coding_fixture",
			MaxNodes:                   3,
			MaxAttemptsPerNode:         3,
			NetworkAllowed:             false,
			ExternalSideEffectsAllowed: false,
		}) ||
		manifest.EvidenceLocation != "PRIVATE_USER_SELECTED_ROOT" ||
		manifest.ExecutionAuthorized {
		t.Fatalf("unsafe live-gate manifest = %#v", manifest)
	}
	assertExactIdentifiers(t, manifest.ApprovalExpectations, []string{
		"user_confirms_saved_team_and_work_package",
		"customer_rule_may_require_start_run_approval",
		"approval_decision_is_journal_authoritative",
	})
	assertExactIdentifiers(t, manifest.StopCancelProcedure, []string{
		"cancel_client_delivery_without_cancelling_run",
		"request_authoritative_run_cancel",
		"verify_terminal_and_grant_revocation",
		"do_not_kill_or_restart_daemon",
	})
	assertExactIdentifiers(t, manifest.RollbackRecoveryChecks, []string{
		"reconnect_from_last_cursor",
		"rebuild_projection_from_journal",
		"verify_evidence_digest_and_private_modes",
		"verify_no_duplicate_run_grant_evidence_done",
		"stop_human_required_on_indeterminate_state",
	})
	assertExactIdentifiers(t, manifest.UnresolvedHumanInputs, []string{
		"installed_runtime_instance_id",
		"private_source_root",
		"approval_decision",
		"live_execution_authorization",
		"final_review_signoff",
	})
	checklist, err := os.ReadFile(filepath.Join(filepath.Dir(manifestPath), "live-demo-checklist.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(checklist, []byte("VERDICT: PASS\n")) ||
		bytes.Contains(checklist, []byte("live task passed")) {
		t.Fatalf("invalid live checklist:\n%s", checklist)
	}
}

func runDemoScenario(
	t *testing.T,
	pkg work.WorkPackage,
) demoScenarioResult {
	t.Helper()
	assertSavedTeamPrerequisite(t, pkg)
	environment := newDemoEnvironment(t, strings.ReplaceAll(pkg.ID(), ".", "-"))
	plan := demoExecutionPlan(t, pkg, "team-phase1-demo")
	request, barrier, sourceCalls, verifierCalls, effectCalls, observer := demoRequest(
		t,
		environment,
		pkg,
		plan,
	)
	first, err := environment.coordinator.Run(context.Background(), request)
	if !errors.Is(err, app.ErrTeamExecutionIncomplete) ||
		first.Team().Status() != "awaiting_recovery" &&
			first.Team().Status() != "running" {
		t.Fatalf("initial scenario result = %#v, %v", first, err)
	}
	environment.clock.Set(demoNow.Add(time.Minute))
	request.AuthoritativeTime = demoNow.Add(time.Minute)
	environment.projection = projection.New(environment.db)
	environment.coordinator = mustDemoCoordinator(
		t,
		environment.work,
		environment.grants,
		environment.projection,
		environment.evidence,
	)
	final, err := environment.coordinator.Run(context.Background(), request)
	if err != nil || final.Team().Status() != "succeeded" {
		t.Fatalf("recovered scenario result = %#v, %v", final, err)
	}
	events, err := environment.store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if bytes.Contains(event.PayloadJSON, []byte("authorized-")) ||
			bytes.Contains(event.PayloadJSON, []byte("opaque-local-authorization")) {
			t.Fatalf("private/tentative content entered Journal: %s", event.PayloadJSON)
		}
	}
	observer.mu.Lock()
	observed := append([]string(nil), observer.eventKeys...)
	observer.mu.Unlock()
	sort.Strings(observed)
	return demoScenarioResult{
		trace:          normalizedDemoTrace(events, plan.TeamInstanceID()),
		packageID:      pkg.ID(),
		evidenceTypes:  pkg.EvidenceTypes(),
		observerEvents: observed,
		maxActive:      barrier.maxActive.Load(),
		sourceCalls:    sourceCalls.Load(),
		verifierCalls:  verifierCalls.Load(),
		effectCalls:    effectCalls.Load(),
	}
}

type demoCoordinatorOutcome struct {
	coordinator *app.TeamCoordinator
	result      app.TeamExecutionResult
	err         error
}

func runCompetingDemoRecovery(
	t *testing.T,
	environment *demoEnvironment,
	request app.TeamExecutionRequest,
	barrier *demoBarrier,
	sourceCalls *atomic.Int32,
	verifierCalls *atomic.Int32,
	effectCalls *atomic.Int32,
) *app.TeamCoordinator {
	t.Helper()
	outcomes := make([]demoCoordinatorOutcome, 2)
	start := make(chan struct{})
	var callers sync.WaitGroup
	for index := range outcomes {
		readModel := projection.New(environment.db)
		workAuthority, grantAuthority := newCompetingDemoAuthorities(
			t,
			environment,
			byte(index+1),
		)
		coordinator := mustDemoCoordinator(
			t,
			workAuthority,
			grantAuthority,
			readModel,
			environment.evidence,
		)
		callerRequest := cloneDemoRequestForAuthorities(
			t,
			request,
			workAuthority,
			grantAuthority,
			barrier,
			sourceCalls,
			verifierCalls,
			effectCalls,
		)
		outcomes[index].coordinator = coordinator
		callers.Add(1)
		go func(index int) {
			defer callers.Done()
			<-start
			outcomes[index].result, outcomes[index].err =
				coordinator.Run(context.Background(), callerRequest)
		}(index)
	}
	close(start)
	callers.Wait()

	winners := 0
	var winner *app.TeamCoordinator
	for index, outcome := range outcomes {
		if outcome.err != nil {
			if !errors.Is(outcome.err, work.ErrTeamExecutionConflict) &&
				!errors.Is(outcome.err, work.ErrStaleGlobalReadView) &&
				!errors.Is(
					outcome.err,
					authorization.ErrGrantIDCollision,
				) &&
				!errors.Is(
					outcome.err,
					authorization.ErrGrantAuthorityConflict,
				) {
				t.Fatalf(
					"competing coordinator %d error = %v",
					index,
					outcome.err,
				)
			}
			continue
		}
		if outcome.result.Team().Status() != "succeeded" {
			t.Fatalf(
				"competing coordinator %d result = %#v",
				index,
				outcome.result,
			)
		}
		switch len(outcome.result.ExecutedNodeIDs()) {
		case 0:
			// A caller that observes the winner's terminal fact is an
			// idempotent loser, not a second dispatcher.
		case 1:
			if outcome.result.ExecutedNodeIDs()[0] != "main" {
				t.Fatalf(
					"competing coordinator %d executed %v",
					index,
					outcome.result.ExecutedNodeIDs(),
				)
			}
			winners++
			winner = outcome.coordinator
		default:
			t.Fatalf(
				"competing coordinator %d executed %v",
				index,
				outcome.result.ExecutedNodeIDs(),
			)
		}
	}
	if winners != 1 || winner == nil {
		t.Fatalf(
			"concurrent authoritative winners = %d, errors=[%v, %v] results=[%v, %v]",
			winners,
			outcomes[0].err,
			outcomes[1].err,
			outcomes[0].result.ExecutedNodeIDs(),
			outcomes[1].result.ExecutedNodeIDs(),
		)
	}
	return winner
}

func cloneDemoRequestForAuthorities(
	t *testing.T,
	request app.TeamExecutionRequest,
	workAuthority *work.Authority,
	grantAuthority *authorization.Authority,
	barrier *demoBarrier,
	sourceCalls *atomic.Int32,
	verifierCalls *atomic.Int32,
	effectCalls *atomic.Int32,
) app.TeamExecutionRequest {
	t.Helper()
	cloned := request
	cloned.Nodes = append([]app.TeamNodeExecution(nil), request.Nodes...)
	for index := range cloned.Nodes {
		node := &cloned.Nodes[index]
		status := "succeeded"
		if node.LogicalNodeID == "main" && node.AttemptNumber == 1 {
			status = "failed"
		}
		node.Executor = newDemoSupervisor(
			t,
			workAuthority,
			grantAuthority,
			&demoAdapter{
				barrier:        barrier,
				logicalNodeID:  node.LogicalNodeID,
				runtimeID:      node.Instance.ID,
				terminalStatus: status,
				calls:          sourceCalls,
				effectCalls:    effectCalls,
				effectOnSuccess: node.LogicalNodeID == "main" &&
					node.AttemptNumber == 2,
			},
		)
	}
	cloned.Semantics = append(
		[]app.TeamNodeSemantics(nil),
		request.Semantics...,
	)
	for index := range cloned.Semantics {
		semantic := &cloned.Semantics[index]
		if semantic.VerifierExecution == nil {
			continue
		}
		execution := *semantic.VerifierExecution
		execution.Executor = newDemoSupervisor(
			t,
			workAuthority,
			grantAuthority,
			&demoAdapter{
				barrier:       newDemoBarrier(),
				logicalNodeID: "verifier-main",
				runtimeID:     execution.Instance.ID,
				calls:         verifierCalls,
			},
		)
		semantic.VerifierExecution = &execution
	}
	return cloned
}

func newCompetingDemoAuthorities(
	t *testing.T,
	environment *demoEnvironment,
	seed byte,
) (*work.Authority, *authorization.Authority) {
	t.Helper()
	workRandom := make([]byte, 4096)
	for index := range workRandom {
		workRandom[index] = 0x70 + seed + byte(index/16)
	}
	workAuthority, err := work.NewAuthority(
		environment.store,
		environment.clock.Now,
		bytes.NewReader(workRandom),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := workAuthority.InitializeRunIdentityIndex(
		context.Background(),
	); err != nil {
		t.Fatal(err)
	}
	grantRandom := make([]byte, 48*32)
	for index := range grantRandom {
		grantRandom[index] = 0x50 + seed + byte(index/48)
	}
	grantAuthority, err := authorization.NewAuthority(
		environment.store,
		workAuthority,
		environment.clock.Now,
		bytes.NewReader(grantRandom),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := grantAuthority.InitializeGrantIdentityIndex(
		context.Background(),
	); err != nil {
		t.Fatal(err)
	}
	return workAuthority, grantAuthority
}

func newDemoEnvironment(t *testing.T, name string) *demoEnvironment {
	t.Helper()
	db, dbPath := openDemoDB(t, name)
	store := journal.NewStore(db)
	for _, runtimeID := range []string{"runtime-a", "runtime-b", "runtime-verifier"} {
		seedDemoRuntime(t, store, runtimeID, 1, demoNow)
	}
	clock := &demoClock{now: demoNow}
	workAuthority, err := work.NewAuthority(
		store,
		clock.Now,
		bytes.NewReader(bytes.Repeat([]byte{0x31}, 4096)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := workAuthority.InitializeRunIdentityIndex(context.Background()); err != nil {
		t.Fatal(err)
	}
	grantRandom := make([]byte, 48*32)
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
	if err := grantAuthority.InitializeGrantIdentityIndex(context.Background()); err != nil {
		t.Fatal(err)
	}
	readModel := projection.New(db)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	evidenceParent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(evidenceParent, 0o700); err != nil {
		t.Fatal(err)
	}
	evidenceRoot := filepath.Join(evidenceParent, "evidence")
	artifactStore, err := evidence.NewStore(evidenceRoot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = artifactStore.Close() })
	return &demoEnvironment{
		db:           db,
		dbPath:       dbPath,
		store:        store,
		clock:        clock,
		work:         workAuthority,
		grants:       grantAuthority,
		projection:   readModel,
		evidence:     artifactStore,
		evidenceRoot: evidenceRoot,
		coordinator: mustDemoCoordinator(
			t,
			workAuthority,
			grantAuthority,
			readModel,
			artifactStore,
		),
	}
}

func demoExecutionPlan(
	t *testing.T,
	pkg work.WorkPackage,
	teamInstanceID string,
) teams.ExecutionPlan {
	t.Helper()
	agents := pkg.RecommendedAgentDefinitionIDs()
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: teamInstanceID,
		Nodes: []teams.ExecutionNodeInput{
			{
				LogicalNodeID: "main", Title: "Integrate",
				AgentInstanceID: agents[0], RuntimeInstanceID: "runtime-a",
				Role:      teams.ExecutionRoleMain,
				DependsOn: []string{"sub-a", "sub-b"}, MaxAttempts: 2,
			},
			{
				LogicalNodeID: "sub-a", Title: "Build A",
				AgentInstanceID: agents[1], RuntimeInstanceID: "runtime-a",
				Role: teams.ExecutionRoleSubAgent, MaxAttempts: 1,
			},
			{
				LogicalNodeID: "sub-b", Title: "Build B",
				AgentInstanceID: agents[2], RuntimeInstanceID: "runtime-b",
				Role: teams.ExecutionRoleSubAgent, MaxAttempts: 1,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func demoRequest(
	t *testing.T,
	environment *demoEnvironment,
	pkg work.WorkPackage,
	plan teams.ExecutionPlan,
) (
	app.TeamExecutionRequest,
	*demoBarrier,
	*atomic.Int32,
	*atomic.Int32,
	*atomic.Int32,
	*demoOutputObserver,
) {
	t.Helper()
	seedDemoTeamInstance(t, environment.store, pkg, plan.TeamInstanceID())
	barrier := newDemoBarrier()
	sourceCalls := &atomic.Int32{}
	verifierCalls := &atomic.Int32{}
	effectCalls := &atomic.Int32{}
	executions := make([]app.TeamNodeExecution, 0, 4)
	for _, node := range plan.Nodes() {
		status := "succeeded"
		if node.LogicalNodeID() == "main" {
			status = "failed"
		}
		executions = append(executions, demoNodeExecution(
			t,
			environment,
			plan,
			node.LogicalNodeID(),
			1,
			node.AgentInstanceID(),
			node.RuntimeInstanceID(),
			status,
			sourceCalls,
			effectCalls,
			barrier,
		))
	}
	main := plan.Nodes()[0]
	for _, node := range plan.Nodes() {
		if node.LogicalNodeID() == "main" {
			main = node
		}
	}
	second := demoNodeExecution(
		t,
		environment,
		plan,
		"main",
		2,
		main.AgentInstanceID(),
		main.RuntimeInstanceID(),
		"succeeded",
		sourceCalls,
		effectCalls,
		barrier,
	)
	second.WorkflowPath = "cached-source"
	executions = append(executions, second)
	semantics := demoSemantics(t, plan)
	verifierProfile := demoProfile(t, "profile-verifier")
	verifierInstance := demoInstance(t, "runtime-verifier")
	for index := range semantics {
		if semantics[index].LogicalNodeID != "main" {
			continue
		}
		acceptance, err := verification.NewAcceptanceContract(
			1,
			[]string{"accepted " + pkg.EvidenceTypes()[0]},
			verification.AcceptanceRiskHigh,
		)
		if err != nil {
			t.Fatal(err)
		}
		semantics[index].AcceptanceContract = acceptance
		semantics[index].VerifierAgentInstanceID = pkg.DefaultVerifierKey()
		semantics[index].VerifierRuntimeInstanceID = "runtime-verifier"
		semantics[index].VerifierWorkflowPath = "independent-verification"
		semantics[index].VerifierExecution = &app.TeamVerifierExecution{
			SourcePath: t.TempDir(),
			Profile:    verifierProfile,
			Instance:   verifierInstance,
			Executor: newDemoSupervisor(
				t,
				environment.work,
				environment.grants,
				&demoAdapter{
					barrier:       newDemoBarrier(),
					logicalNodeID: "verifier-main",
					runtimeID:     "runtime-verifier",
					calls:         verifierCalls,
				},
			),
		}
	}
	observer := &demoOutputObserver{store: environment.store}
	return app.TeamExecutionRequest{
			Plan:                 plan,
			Nodes:                executions,
			Semantics:            semantics,
			AuthoritativeTime:    demoNow,
			PrepareLeaseDuration: time.Minute,
			GrantLifetime:        time.Minute,
			CorrelationID:        demoCorrelation,
			OutputObserver:       observer,
		},
		barrier,
		sourceCalls,
		verifierCalls,
		effectCalls,
		observer
}

func demoNodeExecution(
	t *testing.T,
	environment *demoEnvironment,
	plan teams.ExecutionPlan,
	logicalNodeID string,
	attemptNumber int,
	agentInstanceID string,
	runtimeInstanceID string,
	status string,
	calls *atomic.Int32,
	effectCalls *atomic.Int32,
	barrier *demoBarrier,
) app.TeamNodeExecution {
	t.Helper()
	return app.TeamNodeExecution{
		LogicalNodeID: logicalNodeID,
		AttemptNumber: attemptNumber,
		WorkflowPath:  "primary",
		SourcePath:    t.TempDir(),
		Profile:       demoProfile(t, "profile-"+logicalNodeID+fmt.Sprint(attemptNumber)),
		Instance:      demoInstance(t, runtimeInstanceID),
		Dispatch: demoDispatchFrame(
			t,
			plan,
			logicalNodeID,
			attemptNumber,
			agentInstanceID,
			runtimeInstanceID,
			demoNow,
		),
		Executor: newDemoSupervisor(
			t,
			environment.work,
			environment.grants,
			&demoAdapter{
				barrier:        barrier,
				logicalNodeID:  logicalNodeID,
				runtimeID:      runtimeInstanceID,
				terminalStatus: status,
				calls:          calls,
				effectCalls:    effectCalls,
				effectOnSuccess: logicalNodeID == "main" &&
					attemptNumber == 2,
			},
		),
	}
}

func demoSemantics(
	t *testing.T,
	plan teams.ExecutionPlan,
) []app.TeamNodeSemantics {
	t.Helper()
	result := make([]app.TeamNodeSemantics, 0, len(plan.Nodes()))
	for _, node := range plan.Nodes() {
		output, err := verification.NewOutputContract(
			1,
			verification.EmptyOutputInvalid,
		)
		if err != nil {
			t.Fatal(err)
		}
		fallback := ""
		retryDelay := time.Duration(0)
		if node.LogicalNodeID() == "main" {
			fallback = "cached-source"
			retryDelay = time.Minute
		}
		policy, err := rules.NewRecoveryPolicy(rules.RecoveryPolicyInput{
			Version:             1,
			RetryDelay:          retryDelay,
			AttemptCredits:      node.MaxAttempts() - 1,
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
		result = append(result, app.TeamNodeSemantics{
			LogicalNodeID:       node.LogicalNodeID(),
			OutputContract:      output,
			RecoveryPolicy:      policy,
			AcceptanceContract:  acceptance,
			PrimaryWorkflowPath: "primary",
		})
	}
	return result
}

func testDemoApprovalRestart(t *testing.T, pkg work.WorkPackage) {
	t.Helper()
	db, _ := openDemoDB(t, "approval")
	store := journal.NewStore(db)
	clock := &demoClock{now: demoNow}
	workAuthority, err := work.NewAuthority(
		store,
		clock.Now,
		bytes.NewReader(bytes.Repeat([]byte{0x55}, 1024)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := workAuthority.InitializeRunIdentityIndex(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := workAuthority.CreateAndAssign(
		context.Background(),
		work.WorkItemAssignmentInput{
			WorkItemID:      "work-approval-demo",
			Title:           "Approval-gated demo",
			RunID:           "run-approval-demo",
			AgentInstanceID: "agent-approval-demo",
			CorrelationID:   demoCorrelation,
		},
	); err != nil {
		t.Fatal(err)
	}
	authorizer := &demoCustomerAuthorizer{now: demoNow}
	authority, err := rules.NewAuthority(store, authorizer, clock.Now)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := rules.NewScope("work_item", "work-approval-demo")
	if err != nil {
		t.Fatal(err)
	}
	condition, err := rules.NewCondition("start_run", "high")
	if err != nil {
		t.Fatal(err)
	}
	effect, err := rules.NewEffect(
		"require_approval",
		"",
		[]string{"local-owner"},
		5*time.Minute,
		"reject",
	)
	if err != nil {
		t.Fatal(err)
	}
	rule, err := rules.NewRule("approve-start", condition, effect)
	if err != nil {
		t.Fatal(err)
	}
	ruleSet, err := rules.NewRuleSet(scope, 1, []rules.Rule{rule})
	if err != nil {
		t.Fatal(err)
	}
	activation, err := rules.NewRuleSetActivationRequest(
		ruleSet,
		[]byte("opaque-local-authorization"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authority.ActivateRuleSet(
		context.Background(),
		activation,
		demoCorrelation,
	); err != nil {
		t.Fatal(err)
	}
	action, err := rules.NewActionContext(rules.ActionContextInput{
		ProjectID:       "project.phase1",
		TeamInstanceID:  "team-phase1-demo",
		WorkPackageID:   pkg.ID(),
		WorkItemID:      "work-approval-demo",
		RunID:           "run-approval-demo",
		AgentInstanceID: "agent-approval-demo",
		LogicalNodeID:   "main",
		AttemptNumber:   1,
		Action:          "start_run",
		Risk:            "high",
		ClaimGeneration: 0,
		ContractDigest:  demoContract,
	})
	if err != nil || action.WorkPackageID() != pkg.ID() {
		t.Fatalf("ActionContext = %#v, %v", action, err)
	}
	decision, err := rules.Evaluate([]rules.RuleSet{ruleSet}, action)
	if err != nil || decision.Kind() != "require_approval" {
		t.Fatalf("decision = %#v, %v", decision, err)
	}
	pending, err := authority.RequestApproval(
		context.Background(),
		rules.ApprovalRequestInput{
			Context:            action,
			ContinuationDigest: demoContinuation,
			Decision:           decision,
			RequestedAt:        demoNow,
			CorrelationID:      demoCorrelation,
		},
	)
	if err != nil || pending.Status() != "pending" {
		t.Fatalf("pending approval = %#v, %v", pending, err)
	}
	restarted, err := rules.NewAuthority(store, authorizer, clock.Now)
	if err != nil {
		t.Fatal(err)
	}
	command, err := rules.NewApprovalDecisionRequest(
		pending.ID(),
		pending.Digest(),
		"approved",
		[]byte("opaque-local-authorization"),
	)
	if err != nil {
		t.Fatal(err)
	}
	approved, err := restarted.DecideApproval(
		context.Background(),
		command,
		demoCorrelation,
	)
	if err != nil ||
		approved.Status() != "approved" ||
		approved.ResumeCandidate().ContinuationDigest() != demoContinuation {
		t.Fatalf("approved = %#v, %v", approved, err)
	}
	reopenedProjection := projection.New(db)
	if err := reopenedProjection.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	projected, ok := reopenedProjection.GlobalReadView().WorkItem("work-approval-demo")
	if !ok || projected.Status != "assigned" {
		t.Fatalf("approval restart projection = %#v, %v", projected, ok)
	}
}

type demoCustomerAuthorizer struct {
	now time.Time
}

func (authorizer *demoCustomerAuthorizer) AuthorizeRuleSet(
	ctx context.Context,
	request rules.RuleSetActivationRequest,
) (rules.AuthorizedRuleSetActivation, error) {
	if err := ctx.Err(); err != nil {
		return rules.AuthorizedRuleSetActivation{}, err
	}
	return rules.NewAuthorizedRuleSetActivation(
		request,
		"local-owner",
		demoDigest("activation", request.RuleSet().Digest()),
		authorizer.now,
		authorizer.now.Add(time.Minute),
	)
}

func (authorizer *demoCustomerAuthorizer) AuthorizeApprovalDecision(
	ctx context.Context,
	request rules.ApprovalDecisionRequest,
) (rules.AuthorizedApprovalDecision, error) {
	if err := ctx.Err(); err != nil {
		return rules.AuthorizedApprovalDecision{}, err
	}
	return rules.NewAuthorizedApprovalDecision(
		request,
		"local-owner",
		demoDigest("decision", request.ApprovalRequestDigest()),
		authorizer.now,
		authorizer.now.Add(time.Minute),
	)
}

func assertSavedTeamPrerequisite(t *testing.T, pkg work.WorkPackage) {
	t.Helper()
	agentIDs := pkg.RecommendedAgentDefinitionIDs()
	scope := agents.ScopeIdentity{ProjectID: "project.phase1"}
	definitions := make([]agents.AgentDefinition, len(agentIDs))
	profiles := make([]loomruntime.RuntimeProfile, len(agentIDs))
	roles := make([]teams.TeamDefinitionRole, len(agentIDs))
	for index, agentID := range agentIDs {
		definition, err := agents.NewAgentDefinition(agents.AgentDefinition{
			ID:            agentID,
			Version:       1,
			Scope:         agents.ScopeProject,
			ScopeIdentity: scope,
			Name:          agentID,
			RoleSpec:      "bounded phase1 role",
			Status:        agents.DefinitionActive,
		})
		if err != nil {
			t.Fatal(err)
		}
		definitions[index] = definition
		profile := demoCatalogProfile("profile." + strings.ReplaceAll(agentID, "agent.", ""))
		profiles[index] = profile
		role := teams.TeamDefinitionRoleSubAgent
		if index == 0 {
			role = teams.TeamDefinitionRoleMain
		}
		roles[index] = teams.TeamDefinitionRole{
			Kind:              role,
			AgentDefinitionID: agentID,
			RuntimeProfileID:  profile.ID,
			Responsibility:    "bounded phase1 responsibility",
		}
	}
	definition, err := teams.BuildTeamDefinition(
		teams.TeamDefinitionInput{
			ID:            "saved-team.phase1-engineering",
			Version:       1,
			Scope:         teams.TeamDefinitionScopeProject,
			ScopeIdentity: scope,
			Name:          "Phase 1 Engineering",
			Status:        teams.TeamDefinitionActive,
			Roles:         roles,
		},
		definitions,
		profiles,
	)
	if err != nil {
		t.Fatal(err)
	}
	discovery, err := loomruntime.DiscoverRuntime(
		context.Background(),
		[]loomruntime.RuntimeProbe{demoRuntimeProbe{}},
	)
	if err != nil {
		t.Fatal(err)
	}
	selections := make([]teams.SavedTeamRuntimeSelection, len(agentIDs))
	for index, agentID := range agentIDs {
		selections[index] = teams.SavedTeamRuntimeSelection{
			AgentDefinitionID: agentID,
			RuntimeInstanceID: "runtime.shared",
		}
	}
	binding, err := teams.BuildSavedTeamRuntimeBinding(
		[]teams.TeamDefinition{definition},
		definition.ID(),
		scope,
		definitions,
		profiles,
		discovery,
		selections,
	)
	if err != nil {
		t.Fatal(err)
	}
	catalog := teams.TeamResolutionCatalogInput{
		AgentDefinitions:       definitions,
		RuntimeProfiles:        profiles,
		TeamDefinitions:        []teams.TeamDefinition{definition},
		MainAgentDefinitionIDs: []string{agentIDs[0]},
		DefaultMainAgentID:     agentIDs[0],
		ProjectDefaultTeamID:   definition.ID(),
	}
	intent := mode.Intent{
		Trigger:  mode.TriggerSelectTeam,
		TargetID: definition.ID(),
		Text:     "bounded private task",
	}
	plan, err := teams.BuildSavedTeamInstantiationPlan(
		intent,
		scope,
		catalog,
		binding,
		discovery,
		selections,
	)
	if err != nil ||
		!plan.Ready() ||
		!plan.CreateTeamInstance() ||
		!plan.CreateMainAgentInstance() ||
		plan.CreateSubAgentInstances() ||
		len(plan.DormantSubAgents()) != 2 {
		t.Fatalf("saved-Team prerequisite = %#v, %v", plan, err)
	}
}

type demoRuntimeProbe struct{}

func (demoRuntimeProbe) ID() string { return "probe.phase1-demo" }
func (demoRuntimeProbe) ObserveRuntime(
	context.Context,
) ([]loomruntime.RuntimeObservation, error) {
	return []loomruntime.RuntimeObservation{{
		Instance: loomruntime.RuntimeInstance{
			ID:                   "runtime.shared",
			DeviceID:             "device.phase1",
			AdapterType:          "pi",
			DisplayName:          "Phase 1 fixture",
			ExecutableVersion:    "1.0.0",
			Status:               loomruntime.RuntimeOnline,
			ObservedCapabilities: []string{"text"},
			Capacity:             3,
		},
		ModelIDs: []string{"model.test"},
	}}, nil
}

func demoCatalogProfile(id string) loomruntime.RuntimeProfile {
	budget := int64(100)
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID:                   id,
		AdapterType:          "pi",
		ProviderID:           "provider.local",
		ModelID:              "model.test",
		AuthMode:             loomruntime.AuthBrokered,
		RequiredCapabilities: []string{"text"},
		Timeout:              time.Minute,
		Budget:               &budget,
	})
	if err != nil {
		panic(err)
	}
	return profile
}

func demoProfile(t *testing.T, id string) loomruntime.RuntimeProfile {
	t.Helper()
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID:          id,
		AdapterType: "pi",
		AuthMode:    loomruntime.AuthBrokered,
		Timeout:     5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	return profile
}

func demoInstance(t *testing.T, id string) loomruntime.RuntimeInstance {
	t.Helper()
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID:                id,
		DeviceID:          "device.phase1",
		AdapterType:       "pi",
		DisplayName:       id,
		ExecutableVersion: "1.0.0",
		Status:            loomruntime.RuntimeOnline,
		Capacity:          1,
	})
	if err != nil {
		t.Fatal(err)
	}
	return instance
}

func newDemoSupervisor(
	t *testing.T,
	workAuthority *work.Authority,
	grantAuthority *authorization.Authority,
	adapter supervisor.RuntimeAdapter,
) app.ManagedNodeExecutor {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	controller, err := supervisor.New(
		supervisor.Config{
			WorkspaceRoot:  root,
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

func demoDispatchFrame(
	t *testing.T,
	plan teams.ExecutionPlan,
	logicalNodeID string,
	attemptNumber int,
	agentInstanceID string,
	runtimeInstanceID string,
	now time.Time,
) bridgev1.Frame {
	t.Helper()
	frame, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID:             demoUUID("dispatch", logicalNodeID, fmt.Sprint(attemptNumber)),
		CorrelationID:         demoCorrelation,
		WorkItemID:            demoAttemptIdentity("work", plan, logicalNodeID, attemptNumber),
		RunID:                 demoAttemptIdentity("run", plan, logicalNodeID, attemptNumber),
		ClaimGeneration:       1,
		RuntimeInstanceID:     runtimeInstanceID,
		SenderAgentInstanceID: agentInstanceID,
		Sequence:              1,
		Type:                  bridgev1.MessageDispatch,
		EmittedAt:             now,
		Payload:               []byte(`{"task":"bounded-phase1-demo"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	return frame
}

func demoInboundFrame(
	request supervisor.AdapterRequest,
	sequence int64,
	messageType bridgev1.MessageType,
	payload map[string]string,
	staleGeneration bool,
) bridgev1.Frame {
	encoded, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	generation := request.Binding.ClaimGeneration
	if staleGeneration {
		generation++
	}
	frame, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID: demoUUID(
			"inbound",
			request.Binding.RunID,
			fmt.Sprint(sequence),
			string(messageType),
		),
		CorrelationID:         request.Dispatch.CorrelationID(),
		WorkItemID:            request.Binding.WorkItemID,
		RunID:                 request.Binding.RunID,
		ClaimGeneration:       generation,
		RuntimeInstanceID:     request.Binding.RuntimeInstanceID,
		SenderAgentInstanceID: request.Binding.SenderAgentInstanceID,
		Sequence:              sequence,
		Type:                  messageType,
		EmittedAt:             request.Dispatch.EmittedAt(),
		Payload:               encoded,
	})
	if err != nil {
		panic(err)
	}
	return frame
}

func demoAttemptIdentity(
	label string,
	plan teams.ExecutionPlan,
	logicalNodeID string,
	attemptNumber int,
) string {
	digest := sha256.Sum256([]byte(strings.Join([]string{
		label,
		plan.TeamInstanceID(),
		plan.Digest(),
		logicalNodeID,
		fmt.Sprint(attemptNumber),
	}, "\x00")))
	return "team-" + label + "-" + hex.EncodeToString(digest[:16])
}

func demoUUID(fields ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(fields, "\x00")))
	value := append([]byte(nil), sum[:16]...)
	value[6] = value[6]&0x0f | 0x40
	value[8] = value[8]&0x3f | 0x80
	encoded := hex.EncodeToString(value)
	return encoded[:8] + "-" + encoded[8:12] + "-" +
		encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]
}

func demoDigest(fields ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(fields, "\x00")))
	return hex.EncodeToString(sum[:])
}

func mustDemoCoordinator(
	t *testing.T,
	workAuthority *work.Authority,
	grantAuthority *authorization.Authority,
	readModel *projection.Projection,
	artifactStore *evidence.Store,
) *app.TeamCoordinator {
	t.Helper()
	coordinator, err := app.NewTeamCoordinator(
		workAuthority,
		grantAuthority,
		readModel,
		artifactStore,
	)
	if err != nil {
		t.Fatal(err)
	}
	return coordinator
}

func openDemoDB(t *testing.T, name string) (*sql.DB, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, name+".db")
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "journal_mode(WAL)")
	db, err := sql.Open("sqlite", fmt.Sprintf(
		"file:%s?%s",
		path,
		values.Encode(),
	))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := journal.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	return db, path
}

func seedDemoRuntime(
	t *testing.T,
	store *journal.Store,
	runtimeID string,
	capacity int,
	now time.Time,
) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"discovery_digest": strings.Repeat("a", 64),
		"source_probe_id":  "probe.phase1",
		"instance": map[string]any{
			"id": runtimeID, "device_id": "device.phase1",
			"adapter_type": "pi", "display_name": runtimeID,
			"executable_version": "1.0.0", "status": "online",
			"observed_capabilities": []string{"streaming"}, "capacity": capacity,
		},
		"model_ids": []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(context.Background(), journal.Event{
		ID:             "runtime-" + runtimeID,
		StreamID:       "runtime_instance:" + runtimeID,
		Seq:            1,
		IdempotencyKey: "runtime-" + runtimeID,
		Type:           "RuntimeInstanceDiscovered",
		SchemaVersion:  1,
		EmittedAt:      now.Add(-time.Minute),
		CorrelationID:  demoCorrelation,
		PayloadJSON:    payload,
	}); err != nil {
		t.Fatal(err)
	}
}

func seedDemoTeamInstance(
	t *testing.T,
	store *journal.Store,
	pkg work.WorkPackage,
	teamInstanceID string,
) {
	t.Helper()
	agents := pkg.RecommendedAgentDefinitionIDs()
	dormant := []map[string]any{
		{
			"dormant":             true,
			"agent_definition_id": agents[1],
			"runtime_profile_id":  "profile.sub-a",
			"runtime_instance_id": "runtime-a",
		},
		{
			"dormant":             true,
			"agent_definition_id": agents[2],
			"runtime_profile_id":  "profile.sub-b",
			"runtime_instance_id": "runtime-b",
		},
	}
	sort.Slice(dormant, func(first, second int) bool {
		return dormant[first]["agent_definition_id"].(string) <
			dormant[second]["agent_definition_id"].(string)
	})
	payload, err := json.Marshal(map[string]any{
		"team": map[string]any{
			"id":                      teamInstanceID,
			"work_request_id":         "request.phase1-demo",
			"source_kind":             "saved_team",
			"team_definition_id":      "saved-team.phase1-engineering",
			"team_definition_version": 1,
			"team_definition_scope":   "project",
			"scope_identity": map[string]any{
				"project_id":    "project.phase1",
				"generation_id": "",
			},
			"team_definition_digest": pkg.Digest(),
			"source_plan_digest":     pkg.Digest(),
			"state":                  "created",
			"created_at":             demoNow.Unix(),
		},
		"dormant_sub_agents":       dormant,
		"source_plan_digest":       pkg.Digest(),
		"source_record_set_digest": pkg.Digest(),
		"team_instance_count":      1,
		"agent_instance_count":     1,
		"active_sub_agent_count":   0,
		"work_item_count":          0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(context.Background(), journal.Event{
		ID:             "team-instance-" + pkg.Digest()[:16],
		StreamID:       "team_instance:" + teamInstanceID,
		Seq:            1,
		IdempotencyKey: "team-instance-" + pkg.Digest()[:16],
		Type:           "TeamInstanceCreated",
		SchemaVersion:  1,
		EmittedAt:      demoNow.Add(-30 * time.Second),
		CorrelationID:  "request.phase1-demo",
		PayloadJSON:    payload,
	}); err != nil {
		t.Fatal(err)
	}
	agentPayload, err := json.Marshal(map[string]any{
		"main_agent": map[string]any{
			"id":                       agents[0],
			"team_instance_id":         teamInstanceID,
			"agent_definition_id":      agents[0],
			"agent_definition_version": 1,
			"agent_definition_scope":   "project",
			"scope_identity": map[string]any{
				"project_id":    "project.phase1",
				"generation_id": "",
			},
			"runtime_profile_id":  "profile.main",
			"runtime_instance_id": "runtime-a",
			"is_main":             true,
			"state":               "created",
		},
		"runtime_binding": map[string]any{
			"accepted":    true,
			"profile_id":  "profile.main",
			"instance_id": "runtime-a",
		},
		"source_plan_digest":       pkg.Digest(),
		"source_record_set_digest": pkg.Digest(),
		"team_created_at":          demoNow.Unix(),
		"binding_digest":           pkg.Digest(),
		"runtime_discovery_digest": pkg.Digest(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(context.Background(), journal.Event{
		ID:             "main-agent-" + pkg.Digest()[:16],
		StreamID:       "agent_instance:" + agents[0],
		Seq:            1,
		IdempotencyKey: "main-agent-" + pkg.Digest()[:16],
		Type:           "AgentInstanceCreated",
		SchemaVersion:  1,
		EmittedAt:      demoNow.Add(-30 * time.Second),
		CorrelationID:  "request.phase1-demo",
		CausationID:    "team-instance-" + pkg.Digest()[:16],
		PayloadJSON:    agentPayload,
	}); err != nil {
		t.Fatal(err)
	}
}

func normalizedDemoTrace(
	events []journal.Event,
	teamInstanceID string,
) []string {
	counts := make(map[string]int)
	var teamTrace []string
	for _, event := range events {
		counts[event.Type]++
		if event.StreamID == "team-execution/"+teamInstanceID {
			teamTrace = append(teamTrace, fmt.Sprintf(
				"team:%03d:%s",
				event.Seq,
				event.Type,
			))
		}
	}
	types := make([]string, 0, len(counts))
	for eventType := range counts {
		types = append(types, eventType)
	}
	sort.Strings(types)
	result := append([]string(nil), teamTrace...)
	for _, eventType := range types {
		result = append(result, fmt.Sprintf(
			"count:%s=%d",
			eventType,
			counts[eventType],
		))
	}
	return result
}

func assertDemoExactOnce(
	t *testing.T,
	events []journal.Event,
	effectCalls int32,
) {
	t.Helper()
	counts := make(map[string]int)
	uniqueRuns := make(map[string]struct{})
	uniqueEvidence := make(map[string]struct{})
	grantStreams := make(map[string]map[string]int)
	for _, event := range events {
		counts[event.Type]++
		switch event.Type {
		case "RunCreated", "RunTerminalCommitted":
			uniqueRuns[event.StreamID] = struct{}{}
		case "EvidenceSubmitted":
			uniqueEvidence[event.StreamID] = struct{}{}
		case "AgentGrantIssued",
			"AgentGrantAuthorized",
			"AgentGrantRevoked":
			if grantStreams[event.StreamID] == nil {
				grantStreams[event.StreamID] = make(map[string]int)
			}
			grantStreams[event.StreamID][event.Type]++
		}
	}
	if counts["TeamExecutionTerminal"] != 1 ||
		counts["WorkItemDone"] != 3 ||
		len(uniqueRuns) != 5 ||
		len(uniqueEvidence) != 5 ||
		counts["AgentGrantIdentityReserved"] != 5 ||
		len(grantStreams) != 5 ||
		effectCalls != 1 {
		t.Fatalf(
			"exact-once counts=%v runs=%d evidence=%d grants=%v effects=%d",
			counts,
			len(uniqueRuns),
			len(uniqueEvidence),
			grantStreams,
			effectCalls,
		)
	}
	for streamID, lifecycle := range grantStreams {
		if lifecycle["AgentGrantIssued"] != 1 ||
			lifecycle["AgentGrantAuthorized"] != 3 ||
			lifecycle["AgentGrantRevoked"] != 1 {
			t.Fatalf(
				"Grant lifecycle %s = %v",
				streamID,
				lifecycle,
			)
		}
	}
}

func assertDemoTimelineReconnect(
	t *testing.T,
	environment *demoEnvironment,
	teamInstanceID string,
) {
	t.Helper()
	firstProjection := projection.New(environment.db)
	stream, err := api.NewTeamExecutionStream(api.TeamExecutionStreamConfig{
		TeamInstanceID: teamInstanceID,
		Journal:        environment.store,
		Projection:     firstProjection,
		Now:            environment.clock.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	page, err := stream.ReadPage(context.Background(), "", 2)
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]struct{})
	recordIDs := func(records []api.DeliveryRecord) {
		for _, record := range records {
			data, marshalErr := json.Marshal(record)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			var wire struct {
				DeliveryID string `json:"delivery_id"`
			}
			if err := json.Unmarshal(data, &wire); err != nil {
				t.Fatal(err)
			}
			if _, duplicate := seen[wire.DeliveryID]; duplicate {
				t.Fatalf("duplicate timeline delivery %s", wire.DeliveryID)
			}
			seen[wire.DeliveryID] = struct{}{}
		}
	}
	recordIDs(page.Records())
	cursor := page.NextCursor()
	for page.HasMore() {
		restartedProjection := projection.New(environment.db)
		restarted, streamErr := api.NewTeamExecutionStream(api.TeamExecutionStreamConfig{
			TeamInstanceID: teamInstanceID,
			Journal:        environment.store,
			Projection:     restartedProjection,
			Now:            environment.clock.Now,
		})
		if streamErr != nil {
			t.Fatal(streamErr)
		}
		page, err = restarted.ReadPage(context.Background(), cursor, 2)
		if err != nil {
			t.Fatal(err)
		}
		recordIDs(page.Records())
		cursor = page.NextCursor()
	}
	if len(seen) == 0 || cursor == "" {
		t.Fatalf("reconnect delivered %d records cursor=%q", len(seen), cursor)
	}
}

type frozenDemoViewSource struct {
	view projection.GlobalReadView
}

func (source *frozenDemoViewSource) Rebuild(context.Context) error {
	return nil
}

func (source *frozenDemoViewSource) GlobalReadView() projection.GlobalReadView {
	return source.view
}

func assertDemoStaleCursorRejected(
	t *testing.T,
	environment *demoEnvironment,
	teamInstanceID string,
) {
	t.Helper()
	readModel := projection.New(environment.db)
	stream, err := api.NewTeamExecutionStream(api.TeamExecutionStreamConfig{
		TeamInstanceID: teamInstanceID,
		Journal:        environment.store,
		Projection:     readModel,
		Now:            environment.clock.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	page, err := stream.ReadPage(context.Background(), "", journal.MaxReadPageEvents)
	if err != nil || page.NextCursor() == "" || page.HasMore() {
		t.Fatalf("canonical cursor page = %#v, %v", page, err)
	}
	frozen := &frozenDemoViewSource{view: readModel.GlobalReadView()}
	if _, err := environment.db.ExecContext(
		context.Background(),
		`DROP TRIGGER events_no_update`,
	); err != nil {
		t.Fatal(err)
	}
	streamID := "team-execution/" + teamInstanceID
	if result, err := environment.db.ExecContext(
		context.Background(),
		`UPDATE events
		 SET id = ?
		 WHERE stream_id = ?
		   AND seq = (SELECT MAX(seq) FROM events WHERE stream_id = ?)`,
		demoUUID("stale-head", teamInstanceID),
		streamID,
		streamID,
	); err != nil {
		t.Fatal(err)
	} else if changed, changeErr := result.RowsAffected(); changeErr != nil ||
		changed != 1 {
		t.Fatalf("stale-head mutation changed %d rows, %v", changed, changeErr)
	}
	staleStream, err := api.NewTeamExecutionStream(api.TeamExecutionStreamConfig{
		TeamInstanceID: teamInstanceID,
		Journal:        environment.store,
		Projection:     frozen,
		Now:            environment.clock.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := staleStream.ReadPage(
		context.Background(),
		page.NextCursor(),
		1,
	); !errors.Is(err, api.ErrTimelineCursorConflict) ||
		!errors.Is(err, api.ErrStreamGap) {
		t.Fatalf("canonical stale cursor error = %v", err)
	}
}

func assertPrivateTreeModes(t *testing.T, roots ...string) {
	t.Helper()
	for _, root := range roots {
		if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return os.Chmod(path, 0o700)
			}
			if info.Mode().IsRegular() {
				return os.Chmod(path, 0o600)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			mode := info.Mode().Perm()
			if info.IsDir() && mode != 0o700 {
				return fmt.Errorf("directory %s mode %04o", filepath.Base(path), mode)
			}
			if info.Mode().IsRegular() && mode != 0o600 {
				return fmt.Errorf("file %s mode %04o", filepath.Base(path), mode)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func assertExactIdentifiers(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("identifiers = %v, want %v", got, want)
	}
	seen := make(map[string]struct{}, len(got))
	for _, value := range got {
		if len(value) == 0 || len(value) > 64 ||
			value[0] < 'a' || value[0] > 'z' {
			t.Fatalf("invalid identifier %q", value)
		}
		for _, character := range []byte(value) {
			if (character >= 'a' && character <= 'z') ||
				(character >= '0' && character <= '9') ||
				character == '_' {
				continue
			}
			t.Fatalf("invalid identifier %q", value)
		}
		if _, duplicate := seen[value]; duplicate {
			t.Fatalf("duplicate identifier %q", value)
		}
		seen[value] = struct{}{}
	}
}
