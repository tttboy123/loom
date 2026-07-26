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
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"loom-pi-rebuild/internal/authorization"
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
	now time.Time
}

func (clock *teamCanaryClock) Now() time.Time { return clock.now }

type teamCanaryBarrier struct {
	active    atomic.Int32
	maxActive atomic.Int32
	subCount  atomic.Int32
	release   chan struct{}
	once      sync.Once
}

type teamCanaryAdapter struct {
	barrier        *teamCanaryBarrier
	subagent       bool
	terminalStatus string
	terminalReason string
	runtimeID      string
	calls          *atomic.Int32
	omitOutput     bool
}

func (adapter *teamCanaryAdapter) AdapterType() string { return "pi" }
func (adapter *teamCanaryAdapter) RuntimeInstanceID() string {
	return adapter.runtimeID
}

func (adapter *teamCanaryAdapter) Execute(
	ctx context.Context,
	request supervisor.AdapterRequest,
) (supervisor.AdapterResult, error) {
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
	if adapter.subagent {
		if adapter.barrier.subCount.Add(1) == 2 {
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
		frames = append(frames, teamCanaryInboundFrame(
			request,
			3,
			bridgev1.MessageEvent,
			mustTeamCanaryJSON(map[string]string{
				"delta": "authorized-" + request.Binding.RunID,
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
	})
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
			barrier:        &teamCanaryBarrier{release: make(chan struct{})},
			runtimeID:      "runtime-verifier",
			calls:          &verifierCalls,
			terminalStatus: "failed",
			terminalReason: string(
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
	for _, event := range events {
		counts[event.Type]++
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
	if _, _, err := fixture.work.CreateAndAssign(
		context.Background(),
		work.WorkItemAssignmentInput{
			WorkItemID:      verifierWorkItemID,
			Title:           "Independent verification",
			RunID:           verifierRunID,
			AgentInstanceID: semantics.VerifierAgentInstanceID,
			CorrelationID:   fixture.request.CorrelationID,
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
	fixture.clock.now = now.Add(2 * time.Minute)
	fixture.request.AuthoritativeTime = fixture.clock.now
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

func TestTeamDAGExecutionControlledCanary(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	db := openTeamCanaryDB(t)
	store := journal.NewStore(db)
	seedTeamCanaryRuntime(t, store, "runtime-a", 1, now)
	seedTeamCanaryRuntime(t, store, "runtime-b", 1, now)
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
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-canary",
		Nodes: []teams.ExecutionNodeInput{
			{
				LogicalNodeID: "main", Title: "Integrate",
				AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-a",
				Role:      teams.ExecutionRoleMain,
				DependsOn: []string{"sub-a", "sub-b"}, MaxAttempts: 2,
			},
			{
				LogicalNodeID: "sub-a", Title: "Build A",
				AgentInstanceID: "agent-a", RuntimeInstanceID: "runtime-a",
				Role: teams.ExecutionRoleSubAgent, MaxAttempts: 1,
			},
			{
				LogicalNodeID: "sub-b", Title: "Build B",
				AgentInstanceID: "agent-b", RuntimeInstanceID: "runtime-b",
				Role: teams.ExecutionRoleSubAgent, MaxAttempts: 1,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	semantics := testTeamNodeSemantics(t, plan, time.Minute, "cached-source")
	barrier := &teamCanaryBarrier{release: make(chan struct{})}
	requestNodes := make([]TeamNodeExecution, 0, 3)
	for _, node := range plan.Nodes() {
		workItemID := appTeamAttemptIdentity("work", plan, node.LogicalNodeID(), 1)
		runID := appTeamAttemptIdentity("run", plan, node.LogicalNodeID(), 1)
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
			Payload:               []byte(`{"task":"controlled-canary"}`),
		})
		if err != nil {
			t.Fatal(err)
		}
		profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
			ID:          "profile-" + node.LogicalNodeID(),
			AdapterType: "pi",
			AuthMode:    loomruntime.AuthBrokered,
			Timeout:     5 * time.Second,
		})
		if err != nil {
			t.Fatal(err)
		}
		instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
			ID: node.RuntimeInstanceID(), DeviceID: "device-1",
			AdapterType: "pi", DisplayName: node.RuntimeInstanceID(),
			ExecutableVersion: "1.0.0", Status: loomruntime.RuntimeOnline,
			Capacity: 1,
		})
		if err != nil {
			t.Fatal(err)
		}
		requestNodes = append(requestNodes, TeamNodeExecution{
			LogicalNodeID: node.LogicalNodeID(), AttemptNumber: 1,
			WorkflowPath: "primary",
			SourcePath:   t.TempDir(), Profile: profile, Instance: instance,
			Dispatch: dispatch,
			Executor: newTeamCanarySupervisor(
				t,
				workAuthority,
				grantAuthority,
				&teamCanaryAdapter{
					barrier:   barrier,
					subagent:  node.Role() == teams.ExecutionRoleSubAgent,
					runtimeID: node.RuntimeInstanceID(),
					terminalStatus: func() string {
						if node.Role() == teams.ExecutionRoleMain {
							return "failed"
						}
						return "succeeded"
					}(),
				},
			),
		})
	}
	mainAttemptTwo := teamCanaryNodeExecution(
		t,
		plan,
		"main",
		2,
		"agent-main",
		"runtime-a",
		now,
		newTeamCanarySupervisor(
			t,
			workAuthority,
			grantAuthority,
			&teamCanaryAdapter{
				barrier:        barrier,
				runtimeID:      "runtime-a",
				terminalStatus: "succeeded",
			},
		),
	)
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
		len(firstResult.ExecutedNodeIDs()) != 3 {
		t.Fatalf("first Run() = %#v, %v", firstResult, err)
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
	clock.now = retryAt
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
	if maximum := barrier.maxActive.Load(); maximum != 2 {
		t.Fatalf("maximum concurrent executions = %d, want 2", maximum)
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
	events, err := store.ReadAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	mainAttemptOneRunID := appTeamAttemptIdentity("run", plan, "main", 1)
	controlledFailureObserved := false
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
			controlledFailureObserved =
				terminal.Status == "failed" &&
					terminal.Reason == "controlled_failure"
		}
	}
	if !controlledFailureObserved {
		t.Fatal("main attempt 1 did not preserve controlled failure terminal")
	}
}

func TestTeamCoordinatorRecoversDurableAttemptWindows(t *testing.T) {
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
		fixture.clock.now = fixture.clock.now.Add(2 * time.Minute)
		fixture.request.AuthoritativeTime = fixture.clock.now
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
			fixture.clock.now = fixture.clock.now.Add(2 * time.Minute)
			fixture.request.AuthoritativeTime = fixture.clock.now
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
		retryAt := fixture.clock.now.Add(time.Minute)
		fixture.clock.now = retryAt
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
		fixture.clock.now = fixture.clock.now.Add(2 * time.Minute)
		fixture.request.AuthoritativeTime = fixture.clock.now
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

func newTeamRecoveryFixture(t testing.TB) *teamRecoveryFixture {
	return newTeamRecoveryFixtureWithMaxAttempts(t, 1)
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
	tasks, err := fixture.coordinator.prepareTeamTasks(
		ctx,
		fixture.request,
		dispatched,
		fixture.request.Nodes,
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
		LogicalNodeID: "main",
		AttemptNumber: 1,
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
		Payload:               []byte(`{"task":"controlled-canary-recovery"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID:          "profile-" + logicalNodeID + fmt.Sprint(attemptNumber),
		AdapterType: "pi",
		AuthMode:    loomruntime.AuthBrokered,
		Timeout:     5 * time.Second,
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
	return TeamNodeExecution{
		LogicalNodeID: logicalNodeID,
		AttemptNumber: attemptNumber,
		WorkflowPath:  "primary",
		SourcePath:    t.TempDir(),
		Profile:       profile,
		Instance:      instance,
		Dispatch:      dispatch,
		Executor:      executor,
	}
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
