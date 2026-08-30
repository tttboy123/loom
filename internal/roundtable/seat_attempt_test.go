package roundtable

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSeatAttemptsFreezeIndependentBindingsAndReplayTerminalIsolation(t *testing.T) {
	authority, _, journalStore := newRoundtableFixture(t)
	ctx := context.Background()
	correlation := "88888888-8888-4888-8888-888888888888"
	bindings := setupMissionRoundForAttempts(t, authority, correlation)

	first, err := authority.StartSeatAttempt(ctx, StartSeatAttemptCommand{
		SessionID: "session-attempts", RoundID: "round-1", SeatID: "seat-planner",
		AttemptID: "attempt-planner-1", AttemptNumber: 1, MembershipRevision: 1,
		ExecutionTeamID: "roundtable-team-1", WorkItemID: "work-planner-1",
		RunID: "run-planner-1", SegmentID: "segment-planner-1", ClaimGeneration: 1,
		RuntimeInstanceID:      bindings["seat-planner"].ExecutionBinding.RuntimeInstanceID,
		AgentInstanceID:        "roundtable-agent-planner",
		SeatBindingDigest:      bindings["seat-planner"].BindingDigest,
		ExecutionBindingDigest: bindings["seat-planner"].ExecutionBinding.BindingDigest,
		ContextCapsuleDigest:   strings.Repeat("c", 64), EmittedAt: utcTime(5),
		CorrelationID: correlation,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := authority.StartSeatAttempt(ctx, StartSeatAttemptCommand{
		SessionID: "session-attempts", RoundID: "round-1", SeatID: "seat-reviewer",
		AttemptID: "attempt-reviewer-1", AttemptNumber: 1, MembershipRevision: 1,
		ExecutionTeamID: "roundtable-team-1", WorkItemID: "work-reviewer-1",
		RunID: "run-reviewer-1", SegmentID: "segment-reviewer-1", ClaimGeneration: 1,
		RuntimeInstanceID:      bindings["seat-reviewer"].ExecutionBinding.RuntimeInstanceID,
		AgentInstanceID:        "roundtable-agent-reviewer",
		SeatBindingDigest:      bindings["seat-reviewer"].BindingDigest,
		ExecutionBindingDigest: bindings["seat-reviewer"].ExecutionBinding.BindingDigest,
		ContextCapsuleDigest:   strings.Repeat("d", 64), EmittedAt: utcTime(6),
		CorrelationID: correlation,
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Attempts["attempt-planner-1"].SeatBindingDigest ==
		second.Attempts["attempt-reviewer-1"].SeatBindingDigest ||
		second.Attempts["attempt-reviewer-1"].ContextCapsuleDigest ==
			first.Attempts["attempt-planner-1"].ContextCapsuleDigest ||
		first.Attempts["attempt-planner-1"].SegmentID != "segment-planner-1" {
		t.Fatalf("independent Attempt authority collapsed: %#v", second.Attempts)
	}

	view, err := authority.SucceedSeatAttempt(ctx, SucceedSeatAttemptCommand{
		SessionID: "session-attempts", AttemptID: "attempt-planner-1",
		PayloadReference: "roundtable-payload-planner-1",
		OutputDigest:     strings.Repeat("e", 64), EmittedAt: utcTime(7),
		CorrelationID: correlation,
	})
	if err != nil {
		t.Fatal(err)
	}
	view, err = authority.FailSeatAttempt(ctx, FailSeatAttemptCommand{
		SessionID: "session-attempts", AttemptID: "attempt-reviewer-1",
		IncidentID: "incident-reviewer-1", FailureCode: "provider_rejected",
		FailureStage: "agent_attempt_dispatch", Retryable: false,
		EmittedAt: utcTime(8), CorrelationID: correlation,
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.Attempts["attempt-planner-1"].Status != SeatAttemptSucceeded ||
		view.Attempts["attempt-reviewer-1"].Status != SeatAttemptFailed ||
		view.Attempts["attempt-planner-1"].PayloadReference == "" ||
		view.Attempts["attempt-reviewer-1"].IncidentID != "incident-reviewer-1" {
		t.Fatalf("terminal isolation = %#v", view.Attempts)
	}
	summary := BuildAlignmentSummary(view, utcTime(9))
	if summary.Context == nil || summary.Candidate == nil ||
		summary.Candidate.AttemptID != "attempt-planner-1" ||
		summary.Candidate.OutputDigest != strings.Repeat("e", 64) ||
		summary.Candidate.PayloadReference != "roundtable-payload-planner-1" {
		t.Fatalf("Mission-linked candidate summary = %#v", summary)
	}
	replayed, err := authority.ReadView(ctx, "session-attempts")
	if err != nil || replayed.Digest != view.Digest ||
		replayed.Attempts["attempt-planner-1"] != view.Attempts["attempt-planner-1"] ||
		replayed.Attempts["attempt-reviewer-1"] != view.Attempts["attempt-reviewer-1"] {
		t.Fatalf("Attempt replay = %#v err=%v", replayed.Attempts, err)
	}
	events, err := journalStore.ReadStream(ctx, sessionStream("session-attempts"))
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		body := string(event.PayloadJSON)
		if strings.Contains(body, "model output") || strings.Contains(body, "output_text") ||
			strings.Contains(body, "prompt") || strings.Contains(body, "provider_response") {
			t.Fatalf("Attempt Journal leaked content in %s: %s", event.Type, body)
		}
	}
}

func TestConcludeUsesLatestRoundAttemptInsteadOfOlderHigherAttemptNumber(t *testing.T) {
	authority, _, _ := newRoundtableFixture(t)
	ctx := context.Background()
	correlation := "89898989-8989-4989-8989-898989898989"
	bindings := setupMissionRoundForAttempts(t, authority, correlation)

	plannerFirst := seatAttemptStartCommand(
		bindings["seat-planner"], "seat-planner", "attempt-planner-r1-1", 1,
		"segment-planner-r1-1", 5,
	)
	if _, err := authority.StartSeatAttempt(ctx, plannerFirst); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.FailSeatAttempt(ctx, FailSeatAttemptCommand{
		SessionID: "session-attempts", AttemptID: "attempt-planner-r1-1",
		IncidentID: "incident-planner-r1-1", FailureCode: "provider_rejected",
		FailureStage: "agent_attempt_dispatch", Retryable: true,
		EmittedAt: utcTime(6), CorrelationID: correlation,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.RecordSeatRetry(ctx, RecordSeatRetryCommand{
		SessionID: "session-attempts", RoundID: "round-1",
		InterventionID: "retry-planner-r1", ModeratorSeat: "seat-moderator",
		SeatID: "seat-planner", AttemptID: "attempt-planner-r1-1",
		RequestedAttemptNumber: 2, EmittedAt: utcTime(7),
		CorrelationID: correlation,
	}); err != nil {
		t.Fatal(err)
	}
	plannerRetry := seatAttemptStartCommand(
		bindings["seat-planner"], "seat-planner", "attempt-planner-r1-2", 2,
		"segment-planner-r1-2", 8,
	)
	if _, err := authority.StartSeatAttempt(ctx, plannerRetry); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.FailSeatAttempt(ctx, FailSeatAttemptCommand{
		SessionID: "session-attempts", AttemptID: "attempt-planner-r1-2",
		IncidentID: "incident-planner-r1-2", FailureCode: "provider_rejected",
		FailureStage: "agent_attempt_dispatch", Retryable: false,
		EmittedAt: utcTime(9), CorrelationID: correlation,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.OpenRound(ctx, OpenRoundCommand{
		SessionID: "session-attempts", RoundID: "round-2",
		ModeratorSeat: "seat-moderator", EmittedAt: utcTime(10),
		CorrelationID: correlation,
	}); err != nil {
		t.Fatal(err)
	}

	plannerSecondRound := seatAttemptStartCommand(
		bindings["seat-planner"], "seat-planner", "attempt-planner-r2-1", 1,
		"segment-planner-r2-1", 11,
	)
	plannerSecondRound.RoundID = "round-2"
	reviewerSecondRound := seatAttemptStartCommand(
		bindings["seat-reviewer"], "seat-reviewer", "attempt-reviewer-r2-1", 1,
		"segment-reviewer-r2-1", 12,
	)
	reviewerSecondRound.RoundID = "round-2"
	for _, command := range []StartSeatAttemptCommand{
		plannerSecondRound, reviewerSecondRound,
	} {
		if _, err := authority.StartSeatAttempt(ctx, command); err != nil {
			t.Fatal(err)
		}
	}
	for index, attemptID := range []string{
		"attempt-planner-r2-1", "attempt-reviewer-r2-1",
	} {
		if _, err := authority.SucceedSeatAttempt(ctx, SucceedSeatAttemptCommand{
			SessionID: "session-attempts", AttemptID: attemptID,
			PayloadReference: "payload-" + attemptID,
			OutputDigest:     strings.Repeat(string(rune('e'+index)), 64),
			EmittedAt:        utcTime(13 + index), CorrelationID: correlation,
		}); err != nil {
			t.Fatal(err)
		}
	}

	view, err := authority.ConcludeSession(ctx, ConcludeSessionCommand{
		SessionID: "session-attempts", ModeratorSeat: "seat-moderator",
		EmittedAt: utcTime(15), CorrelationID: correlation,
	})
	if err != nil {
		t.Fatalf("latest round should be independently concludable: %v", err)
	}
	if !view.Session.Concluded {
		t.Fatal("latest successful round did not conclude")
	}
}

func TestReconcileInterruptedAttemptsCancelsRunningAttemptOnce(t *testing.T) {
	authority, evidenceStore, journalStore := newRoundtableFixture(t)
	ctx := context.Background()
	correlation := "78787878-7878-4878-8878-787878787878"
	bindings := setupMissionRoundForAttempts(t, authority, correlation)
	if _, err := authority.StartSeatAttempt(ctx, StartSeatAttemptCommand{
		SessionID: "session-attempts", RoundID: "round-1", SeatID: "seat-planner",
		AttemptID: "attempt-planner-interrupted", AttemptNumber: 1, MembershipRevision: 1,
		ExecutionTeamID: "roundtable-team-restart", WorkItemID: "work-planner-restart",
		RunID: "run-planner-restart", SegmentID: "segment-planner-restart", ClaimGeneration: 1,
		RuntimeInstanceID:      bindings["seat-planner"].ExecutionBinding.RuntimeInstanceID,
		AgentInstanceID:        "roundtable-agent-planner-restart",
		SeatBindingDigest:      bindings["seat-planner"].BindingDigest,
		ExecutionBindingDigest: bindings["seat-planner"].ExecutionBinding.BindingDigest,
		ContextCapsuleDigest:   strings.Repeat("a", 64), EmittedAt: utcTime(5),
		CorrelationID: correlation,
	}); err != nil {
		t.Fatal(err)
	}
	restarted, err := NewAuthority(
		journalStore, evidenceStore, func() time.Time { return utcTime(12) }, deterministicReader{},
	)
	if err != nil {
		t.Fatal(err)
	}
	reconciled, err := restarted.ReconcileInterruptedAttempts(ctx, correlation)
	if err != nil || reconciled != 1 {
		t.Fatalf("reconciled = %d, err = %v", reconciled, err)
	}
	view, err := restarted.ReadView(ctx, "session-attempts")
	if err != nil {
		t.Fatal(err)
	}
	attempt := view.Attempts["attempt-planner-interrupted"]
	if attempt.Status != SeatAttemptCancelled || !attempt.CompletedAt.Equal(utcTime(12)) {
		t.Fatalf("reconciled Attempt = %#v", attempt)
	}
	reconciled, err = restarted.ReconcileInterruptedAttempts(ctx, correlation)
	if err != nil || reconciled != 0 {
		t.Fatalf("idempotent reconcile = %d, err = %v", reconciled, err)
	}
}

func TestSeatAttemptRejectsBindingSubstitutionAndDuplicateActiveAttempt(t *testing.T) {
	authority, _, journalStore := newRoundtableFixture(t)
	correlation := "99999999-9999-4999-8999-999999999999"
	bindings := setupMissionRoundForAttempts(t, authority, correlation)
	ctx := context.Background()
	start := StartSeatAttemptCommand{
		SessionID: "session-attempts", RoundID: "round-1", SeatID: "seat-planner",
		AttemptID: "attempt-planner-1", AttemptNumber: 1, MembershipRevision: 1,
		ExecutionTeamID: "roundtable-team-1", WorkItemID: "work-planner-1",
		RunID: "run-planner-1", SegmentID: "segment-planner-1", ClaimGeneration: 1,
		RuntimeInstanceID:      bindings["seat-planner"].ExecutionBinding.RuntimeInstanceID,
		AgentInstanceID:        "roundtable-agent-planner",
		SeatBindingDigest:      bindings["seat-planner"].BindingDigest,
		ExecutionBindingDigest: bindings["seat-planner"].ExecutionBinding.BindingDigest,
		ContextCapsuleDigest:   strings.Repeat("c", 64), EmittedAt: utcTime(5),
		CorrelationID: correlation,
	}
	if _, err := authority.StartSeatAttempt(ctx, start); err != nil {
		t.Fatal(err)
	}
	before, _ := journalStore.ReadStream(ctx, sessionStream(start.SessionID))
	substituted := start
	substituted.AttemptID = "attempt-substituted"
	substituted.SeatBindingDigest = bindings["seat-reviewer"].BindingDigest
	if _, err := authority.StartSeatAttempt(ctx, substituted); !errors.Is(
		err, ErrInvalidRoundtableSeatAttempt,
	) {
		t.Fatalf("binding substitution error = %v", err)
	}
	duplicate := start
	duplicate.AttemptID = "attempt-duplicate"
	if _, err := authority.StartSeatAttempt(ctx, duplicate); !errors.Is(
		err, ErrRoundtableSeatAttemptConflict,
	) {
		t.Fatalf("duplicate active error = %v", err)
	}
	after, _ := journalStore.ReadStream(ctx, sessionStream(start.SessionID))
	if len(after) != len(before) {
		t.Fatalf("rejected Attempt changed Journal: before=%d after=%d", len(before), len(after))
	}

	wrongNumber := start
	wrongNumber.AttemptID = "attempt-wrong-number"
	wrongNumber.AttemptNumber = 2
	if _, err := authority.StartSeatAttempt(ctx, wrongNumber); !errors.Is(
		err, ErrInvalidRoundtableSeatAttempt,
	) {
		t.Fatalf("compiler Attempt number mismatch error = %v", err)
	}
	missingSegment := start
	missingSegment.AttemptID = "attempt-missing-segment"
	missingSegment.SegmentID = ""
	if _, err := authority.StartSeatAttempt(ctx, missingSegment); !errors.Is(
		err, ErrInvalidRoundtableSeatAttempt,
	) {
		t.Fatalf("missing Segment identity error = %v", err)
	}
}

func setupMissionRoundForAttempts(
	t *testing.T,
	authority *Authority,
	correlation string,
) map[string]FrozenSeatBinding {
	t.Helper()
	ctx := context.Background()
	sessionContext := roundtableMissionContext()
	if _, err := authority.CreateSession(ctx, CreateSessionCommand{
		SessionID: "session-attempts", ModeratorSeat: "seat-moderator",
		Title: "Attempt lifecycle", Context: &sessionContext,
		EmittedAt: utcTime(1), CorrelationID: correlation,
	}); err != nil {
		t.Fatal(err)
	}
	inputs := []struct {
		seatID, agentID, role, profile, account, model string
		revision                                       int64
	}{
		{"seat-planner", "agent.planner", "main", "profile.planner", "deepseek.planning", "deepseek-reasoner", 3},
		{"seat-reviewer", "agent.reviewer", "subagent", "profile.reviewer", "deepseek.review", "deepseek-chat", 5},
	}
	bindings := make(map[string]FrozenSeatBinding, len(inputs))
	for index, input := range inputs {
		execution := roundtableExecutionBinding(
			t, input.profile, input.account, input.model, input.revision,
		)
		binding, err := FreezeSeatBinding(
			"session-attempts", input.seatID, sessionContext, input.agentID,
			input.role, input.profile, execution, 1,
		)
		if err != nil {
			t.Fatal(err)
		}
		bindings[input.seatID] = binding
		if _, err := authority.AddSeat(ctx, AddSeatCommand{
			SessionID: "session-attempts", SeatID: input.seatID,
			DisplayName: input.agentID, Binding: &binding,
			EmittedAt: utcTime(index + 2), CorrelationID: correlation,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := authority.OpenRound(ctx, OpenRoundCommand{
		SessionID: "session-attempts", RoundID: "round-1",
		ModeratorSeat: "seat-moderator", EmittedAt: utcTime(4),
		CorrelationID: correlation,
	}); err != nil {
		t.Fatal(err)
	}
	return bindings
}
