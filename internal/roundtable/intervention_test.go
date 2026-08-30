package roundtable

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

const interventionCorrelation = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"

func TestGovernedInterventionsProjectDigestAndReplayWithoutContent(t *testing.T) {
	authority, _, journalStore := newRoundtableFixture(t)
	ctx := context.Background()
	bindings := setupMissionRoundForAttempts(t, authority, interventionCorrelation)
	plannerStart := seatAttemptStartCommand(
		bindings["seat-planner"], "seat-planner", "attempt-planner-1", 1,
		"segment-planner-1", 5,
	)
	if _, err := authority.StartSeatAttempt(ctx, plannerStart); err != nil {
		t.Fatal(err)
	}

	if _, err := authority.PauseRound(ctx, PauseRoundCommand{
		SessionID: "session-attempts", RoundID: "round-1",
		InterventionID: "pause-1", ModeratorSeat: "seat-planner",
		EmittedAt: utcTime(6), CorrelationID: interventionCorrelation,
	}); !errors.Is(err, ErrRoundtableNotModerator) {
		t.Fatalf("non-moderator pause error = %v", err)
	}
	guidance := "Keep the guidance out of the RoundTable Journal."
	contentDigest := digestBytes(guidance)
	view, err := authority.RecordSeatSteer(ctx, RecordSeatSteerCommand{
		SessionID: "session-attempts", RoundID: "round-1",
		InterventionID: "steer-1", ModeratorSeat: "seat-moderator",
		SeatID: "seat-planner", AttemptID: "attempt-planner-1",
		InputID: "input-planner-1", ContentDigest: contentDigest,
		EmittedAt: utcTime(7), CorrelationID: interventionCorrelation,
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt := view.Interventions["steer-1"]
	if receipt.InputID != "input-planner-1" || receipt.ContentDigest != contentDigest ||
		receipt.AttemptID != "attempt-planner-1" || receipt.Digest == "" {
		t.Fatalf("steer receipt = %#v", receipt)
	}

	view, err = authority.CancelSeatAttempt(ctx, CancelSeatAttemptCommand{
		SessionID: "session-attempts", RoundID: "round-1",
		SeatID: "seat-planner", AttemptID: "attempt-planner-1",
		EmittedAt: utcTime(8), CorrelationID: interventionCorrelation,
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.Attempts["attempt-planner-1"].Status != SeatAttemptCancelled ||
		view.Attempts["attempt-planner-1"].CompletedAt != utcTime(8) ||
		findIntervention(view, InterventionCancelSeatAttempt, "attempt-planner-1").Kind != InterventionCancelSeatAttempt {
		t.Fatalf("cancel projection = %#v %#v", view.Attempts, view.Interventions)
	}
	view, err = authority.RecordSeatSteer(ctx, RecordSeatSteerCommand{
		SessionID: "session-attempts", RoundID: "round-1",
		InterventionID: "steer-admitted-before-terminal", ModeratorSeat: "seat-moderator",
		SeatID: "seat-planner", AttemptID: "attempt-planner-1",
		InputID: "input-before-terminal", ContentDigest: strings.Repeat("d", 64),
		EmittedAt: utcTime(7).Add(500 * time.Millisecond), CorrelationID: interventionCorrelation,
	})
	if err != nil || view.Interventions["steer-admitted-before-terminal"].InputID != "input-before-terminal" {
		t.Fatalf("post-terminal steer receipt = %#v err=%v", view.Interventions, err)
	}
	if _, err := authority.RecordSeatSteer(ctx, RecordSeatSteerCommand{
		SessionID: "session-attempts", RoundID: "round-1",
		InterventionID: "steer-admitted-after-terminal", ModeratorSeat: "seat-moderator",
		SeatID: "seat-planner", AttemptID: "attempt-planner-1",
		InputID: "input-after-terminal", ContentDigest: strings.Repeat("e", 64),
		EmittedAt: utcTime(8).Add(time.Nanosecond), CorrelationID: interventionCorrelation,
	}); !errors.Is(err, ErrInvalidRoundtableIntervention) {
		t.Fatalf("post-completion steer error = %v", err)
	}
	if repeated, err := authority.CancelSeatAttempt(ctx, CancelSeatAttemptCommand{
		SessionID: "session-attempts", RoundID: "round-1",
		SeatID: "seat-planner", AttemptID: "attempt-planner-1",
		EmittedAt: utcTime(9), CorrelationID: interventionCorrelation,
	}); err != nil || repeated.Digest != view.Digest {
		t.Fatalf("Attempt-idempotent cancel = %#v err=%v", repeated, err)
	}

	view, err = authority.RecordSeatRetry(ctx, RecordSeatRetryCommand{
		SessionID: "session-attempts", RoundID: "round-1",
		InterventionID: "retry-1", ModeratorSeat: "seat-moderator",
		SeatID: "seat-planner", AttemptID: "attempt-planner-1",
		RequestedAttemptNumber: 2, EmittedAt: utcTime(9),
		CorrelationID: interventionCorrelation,
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.Interventions["retry-1"].RequestedAttemptNumber != 2 {
		t.Fatalf("retry audit metadata = %#v", view.Interventions["retry-1"])
	}
	retryStart := seatAttemptStartCommand(
		bindings["seat-planner"], "seat-planner", "attempt-planner-2", 2,
		"segment-planner-2", 10,
	)
	view, err = authority.StartSeatAttempt(ctx, retryStart)
	if err != nil {
		t.Fatal(err)
	}
	if view.Attempts["attempt-planner-2"].AttemptNumber != 2 ||
		view.Attempts["attempt-planner-2"].SegmentID != "segment-planner-2" {
		t.Fatalf("retry Attempt identity = %#v", view.Attempts["attempt-planner-2"])
	}
	view, err = authority.FailSeatAttempt(ctx, FailSeatAttemptCommand{
		SessionID: "session-attempts", AttemptID: "attempt-planner-2",
		IncidentID: "incident-planner-2", FailureCode: "provider_rejected",
		FailureStage: "agent_attempt_dispatch", Retryable: false,
		EmittedAt: utcTime(11), CorrelationID: interventionCorrelation,
	})
	if err != nil {
		t.Fatal(err)
	}

	oldAttempts := map[string]SeatAttempt{
		"attempt-planner-1": view.Attempts["attempt-planner-1"],
		"attempt-planner-2": view.Attempts["attempt-planner-2"],
	}
	replacementExecution := roundtableExecutionBinding(
		t, "profile.replacement", "deepseek.replacement", "deepseek-chat", 7,
	)
	replacement, err := FreezeSeatBinding(
		"session-attempts", "seat-planner", roundtableMissionContext(),
		"agent.replacement", "subagent", "profile.replacement",
		replacementExecution, 2,
	)
	if err != nil {
		t.Fatal(err)
	}
	view, err = authority.ReplaceSeat(ctx, ReplaceSeatCommand{
		SessionID: "session-attempts", RoundID: "round-1",
		InterventionID: "replace-1", ModeratorSeat: "seat-moderator",
		SeatID: "seat-planner", DisplayName: "agent.replacement",
		Binding: replacement, EmittedAt: utcTime(12),
		CorrelationID: interventionCorrelation,
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.Seats["seat-planner"].Binding.MembershipRevision != 2 ||
		view.Seats["seat-planner"].Binding.BindingDigest != replacement.BindingDigest ||
		view.Attempts["attempt-planner-1"] != oldAttempts["attempt-planner-1"] ||
		view.Attempts["attempt-planner-2"] != oldAttempts["attempt-planner-2"] {
		t.Fatalf("replacement mutated history: seat=%#v attempts=%#v", view.Seats["seat-planner"], view.Attempts)
	}
	replacementReceipt := view.Interventions["replace-1"]
	if replacementReceipt.PreviousMembershipRevision != 1 ||
		replacementReceipt.MembershipRevision != 2 ||
		replacementReceipt.PreviousBindingDigest != bindings["seat-planner"].BindingDigest ||
		replacementReceipt.SeatBindingDigest != replacement.BindingDigest {
		t.Fatalf("replacement receipt = %#v", replacementReceipt)
	}

	if _, err := authority.SkipSeat(ctx, SkipSeatCommand{
		SessionID: "session-attempts", RoundID: "round-1",
		InterventionID: "skip-stale", ModeratorSeat: "seat-moderator",
		SeatID: "seat-reviewer", EmittedAt: utcTime(13),
		CorrelationID:              interventionCorrelation,
		ExpectedMembershipRevision: bindings["seat-reviewer"].MembershipRevision + 1,
		ExpectedSeatBindingDigest:  bindings["seat-reviewer"].BindingDigest,
	}); !errors.Is(err, ErrRoundtableInterventionConflict) {
		t.Fatalf("stale frozen seat binding skip error = %v", err)
	}
	view, err = authority.SkipSeat(ctx, SkipSeatCommand{
		SessionID: "session-attempts", RoundID: "round-1",
		InterventionID: "skip-1", ModeratorSeat: "seat-moderator",
		SeatID: "seat-reviewer", EmittedAt: utcTime(13),
		CorrelationID:              interventionCorrelation,
		ExpectedMembershipRevision: bindings["seat-reviewer"].MembershipRevision,
		ExpectedSeatBindingDigest:  bindings["seat-reviewer"].BindingDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.Interventions["skip-1"].Kind != InterventionSkipSeat {
		t.Fatalf("skip receipt = %#v", view.Interventions["skip-1"])
	}
	skippedStart := seatAttemptStartCommand(
		bindings["seat-reviewer"], "seat-reviewer", "attempt-reviewer-1", 1,
		"segment-reviewer-1", 14,
	)
	if _, err := authority.StartSeatAttempt(ctx, skippedStart); !errors.Is(
		err, ErrInvalidRoundtableSeatAttempt,
	) {
		t.Fatalf("skipped seat admitted Attempt: %v", err)
	}
	view, err = authority.PauseRound(ctx, PauseRoundCommand{
		SessionID: "session-attempts", RoundID: "round-1",
		InterventionID: "pause-1", ModeratorSeat: "seat-moderator",
		EmittedAt: utcTime(14), CorrelationID: interventionCorrelation,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !view.Rounds[0].PauseRequested ||
		view.Interventions["pause-1"].Kind != InterventionPauseRound {
		t.Fatalf("pause projection = %#v %#v", view.Rounds[0], view.Interventions)
	}
	pausedStart := seatAttemptStartCommand(
		replacement, "seat-planner", "attempt-planner-3", 3,
		"segment-planner-3", 15,
	)
	if _, err := authority.StartSeatAttempt(ctx, pausedStart); !errors.Is(
		err, ErrInvalidRoundtableSeatAttempt,
	) {
		t.Fatalf("paused round admitted Attempt: %v", err)
	}
	beforeIdempotent, _ := journalStore.ReadStream(ctx, sessionStream("session-attempts"))
	idempotentCommands := []func() error{
		func() error {
			_, err := authority.PauseRound(ctx, PauseRoundCommand{
				SessionID: "session-attempts", RoundID: "round-1",
				InterventionID: "pause-1", ModeratorSeat: "seat-moderator",
				EmittedAt: utcTime(14), CorrelationID: interventionCorrelation,
			})
			return err
		},
		func() error {
			_, err := authority.RecordSeatSteer(ctx, RecordSeatSteerCommand{
				SessionID: "session-attempts", RoundID: "round-1",
				InterventionID: "steer-1", ModeratorSeat: "seat-moderator",
				SeatID: "seat-planner", AttemptID: "attempt-planner-1",
				InputID: "input-planner-1", ContentDigest: contentDigest,
				EmittedAt: utcTime(7), CorrelationID: interventionCorrelation,
			})
			return err
		},
		func() error {
			_, err := authority.RecordSeatRetry(ctx, RecordSeatRetryCommand{
				SessionID: "session-attempts", RoundID: "round-1",
				InterventionID: "retry-1", ModeratorSeat: "seat-moderator",
				SeatID: "seat-planner", AttemptID: "attempt-planner-1",
				RequestedAttemptNumber: 2, EmittedAt: utcTime(9),
				CorrelationID: interventionCorrelation,
			})
			return err
		},
		func() error {
			_, err := authority.ReplaceSeat(ctx, ReplaceSeatCommand{
				SessionID: "session-attempts", RoundID: "round-1",
				InterventionID: "replace-1", ModeratorSeat: "seat-moderator",
				SeatID: "seat-planner", DisplayName: "agent.replacement",
				Binding: replacement, EmittedAt: utcTime(12),
				CorrelationID: interventionCorrelation,
			})
			return err
		},
		func() error {
			_, err := authority.SkipSeat(ctx, SkipSeatCommand{
				SessionID: "session-attempts", RoundID: "round-1",
				InterventionID: "skip-1", ModeratorSeat: "seat-moderator",
				SeatID: "seat-reviewer", EmittedAt: utcTime(13),
				CorrelationID: interventionCorrelation,
			})
			return err
		},
	}
	for _, reapply := range idempotentCommands {
		if err := reapply(); err != nil {
			t.Fatalf("idempotent intervention: %v", err)
		}
	}
	afterIdempotent, _ := journalStore.ReadStream(ctx, sessionStream("session-attempts"))
	if len(afterIdempotent) != len(beforeIdempotent) {
		t.Fatalf("idempotent interventions appended facts: before=%d after=%d", len(beforeIdempotent), len(afterIdempotent))
	}

	replayed, err := authority.ReadView(ctx, "session-attempts")
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Digest != view.Digest ||
		len(replayed.Interventions) != len(view.Interventions) ||
		replayed.Interventions["replace-1"] != view.Interventions["replace-1"] {
		t.Fatalf("intervention replay mismatch: got=%#v want=%#v", replayed, view)
	}
	events, err := journalStore.ReadStream(ctx, sessionStream("session-attempts"))
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		payload := string(event.PayloadJSON)
		if strings.Contains(payload, guidance) || strings.Contains(payload, `"guidance"`) ||
			strings.Contains(payload, "provider output") || strings.Contains(payload, "secret") {
			t.Fatalf("Journal content leak in %s: %s", event.Type, payload)
		}
	}
}

func TestInterventionsRejectStaleAuthorityAndReplayTampering(t *testing.T) {
	authority, _, journalStore := newRoundtableFixture(t)
	ctx := context.Background()
	bindings := setupMissionRoundForAttempts(t, authority, interventionCorrelation)
	start := seatAttemptStartCommand(
		bindings["seat-planner"], "seat-planner", "attempt-planner-1", 1,
		"segment-planner-1", 5,
	)
	if _, err := authority.StartSeatAttempt(ctx, start); err != nil {
		t.Fatal(err)
	}
	before, _ := journalStore.ReadStream(ctx, sessionStream("session-attempts"))

	badSteer := RecordSeatSteerCommand{
		SessionID: "session-attempts", RoundID: "round-1",
		InterventionID: "steer-bad", ModeratorSeat: "seat-moderator",
		SeatID: "seat-reviewer", AttemptID: "attempt-planner-1",
		InputID: "input-bad", ContentDigest: strings.Repeat("b", 64),
		EmittedAt: utcTime(6), CorrelationID: interventionCorrelation,
	}
	if _, err := authority.RecordSeatSteer(ctx, badSteer); !errors.Is(err, ErrInvalidRoundtableIntervention) {
		t.Fatalf("cross-seat Steer error = %v", err)
	}
	if _, err := authority.RecordSeatRetry(ctx, RecordSeatRetryCommand{
		SessionID: "session-attempts", RoundID: "round-1",
		InterventionID: "retry-running", ModeratorSeat: "seat-moderator",
		SeatID: "seat-planner", AttemptID: "attempt-planner-1",
		RequestedAttemptNumber: 2, EmittedAt: utcTime(6),
		CorrelationID: interventionCorrelation,
	}); !errors.Is(err, ErrRoundtableInterventionConflict) {
		t.Fatalf("running retry error = %v", err)
	}
	replacement := bindings["seat-planner"]
	replacement.MembershipRevision = 2
	if _, err := authority.ReplaceSeat(ctx, ReplaceSeatCommand{
		SessionID: "session-attempts", RoundID: "round-1",
		InterventionID: "replace-invalid", ModeratorSeat: "seat-moderator",
		SeatID: "seat-planner", DisplayName: "replacement", Binding: replacement,
		EmittedAt: utcTime(6), CorrelationID: interventionCorrelation,
	}); !errors.Is(err, ErrInvalidRoundtableSeatBinding) {
		t.Fatalf("unfrozen replacement error = %v", err)
	}
	after, _ := journalStore.ReadStream(ctx, sessionStream("session-attempts"))
	if len(after) != len(before) {
		t.Fatalf("rejected interventions changed Journal: before=%d after=%d", len(before), len(after))
	}

	if _, err := authority.RecordSeatSteer(ctx, RecordSeatSteerCommand{
		SessionID: "session-attempts", RoundID: "round-1",
		InterventionID: "steer-good", ModeratorSeat: "seat-moderator",
		SeatID: "seat-planner", AttemptID: "attempt-planner-1",
		InputID: "input-good", ContentDigest: strings.Repeat("c", 64),
		EmittedAt: utcTime(7), CorrelationID: interventionCorrelation,
	}); err != nil {
		t.Fatal(err)
	}
	events, err := journalStore.ReadStream(ctx, sessionStream("session-attempts"))
	if err != nil {
		t.Fatal(err)
	}
	for index := range events {
		if events[index].Type != FactRoundSteered {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal(events[index].PayloadJSON, &raw); err != nil {
			t.Fatal(err)
		}
		raw["guidance"] = "must be rejected"
		mutated, err := json.Marshal(raw)
		if err != nil {
			t.Fatal(err)
		}
		events[index].PayloadJSON = mutated
		break
	}
	if _, err := replaySession("session-attempts", events); !errors.Is(err, ErrRoundtableConflict) {
		t.Fatalf("unknown guidance replay error = %v", err)
	}
}

func TestSeatRetryRejectsFrozenBindingDriftAtomically(t *testing.T) {
	authority, _, journalStore := newRoundtableFixture(t)
	ctx := context.Background()
	bindings := setupMissionRoundForAttempts(t, authority, interventionCorrelation)
	start := seatAttemptStartCommand(
		bindings["seat-planner"], "seat-planner", "attempt-planner-stale", 1,
		"segment-planner-stale", 5,
	)
	if _, err := authority.StartSeatAttempt(ctx, start); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.FailSeatAttempt(ctx, FailSeatAttemptCommand{
		SessionID: "session-attempts", AttemptID: start.AttemptID,
		IncidentID: "incident-planner-stale", FailureCode: "provider_rejected",
		FailureStage: "agent_attempt_dispatch", Retryable: true,
		EmittedAt: utcTime(6), CorrelationID: interventionCorrelation,
	}); err != nil {
		t.Fatal(err)
	}
	replacementExecution := roundtableExecutionBinding(
		t, "profile.retry.replacement", "deepseek.retry.replacement", "deepseek-chat", 9,
	)
	replacement, err := FreezeSeatBinding(
		"session-attempts", "seat-planner", roundtableMissionContext(),
		"agent.retry.replacement", "subagent", "profile.retry.replacement",
		replacementExecution, 2,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authority.ReplaceSeat(ctx, ReplaceSeatCommand{
		SessionID: "session-attempts", RoundID: "round-1",
		InterventionID: "replace-before-retry", ModeratorSeat: "seat-moderator",
		SeatID: "seat-planner", DisplayName: "Retry replacement", Binding: replacement,
		EmittedAt: utcTime(7), CorrelationID: interventionCorrelation,
	}); err != nil {
		t.Fatal(err)
	}
	before, err := journalStore.ReadStream(ctx, sessionStream("session-attempts"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authority.RecordSeatRetry(ctx, RecordSeatRetryCommand{
		SessionID: "session-attempts", RoundID: "round-1",
		InterventionID: "retry-stale-binding", ModeratorSeat: "seat-moderator",
		SeatID: "seat-planner", AttemptID: start.AttemptID,
		RequestedAttemptNumber:     2,
		ExpectedMembershipRevision: bindings["seat-planner"].MembershipRevision,
		ExpectedSeatBindingDigest:  bindings["seat-planner"].BindingDigest,
		EmittedAt:                  utcTime(8), CorrelationID: interventionCorrelation,
	}); !errors.Is(err, ErrRoundtableInterventionConflict) {
		t.Fatalf("stale retry binding error = %v", err)
	}
	after, err := journalStore.ReadStream(ctx, sessionStream("session-attempts"))
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("stale retry appended fact: before=%d after=%d", len(before), len(after))
	}
}

func TestInterventionsRejectHistoricalRoundAtomically(t *testing.T) {
	authority, _, journalStore := newRoundtableFixture(t)
	ctx := context.Background()
	bindings := setupMissionRoundForAttempts(t, authority, interventionCorrelation)
	if _, err := authority.OpenRound(ctx, OpenRoundCommand{
		SessionID: "session-attempts", RoundID: "round-2",
		ModeratorSeat: "seat-moderator", Prompt: "Current round",
		EmittedAt: utcTime(5), CorrelationID: interventionCorrelation,
	}); err != nil {
		t.Fatal(err)
	}
	before, err := journalStore.ReadStream(ctx, sessionStream("session-attempts"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authority.PauseRound(ctx, PauseRoundCommand{
		SessionID: "session-attempts", RoundID: "round-1",
		InterventionID: "pause-historical", ModeratorSeat: "seat-moderator",
		EmittedAt: utcTime(6), CorrelationID: interventionCorrelation,
	}); !errors.Is(err, ErrInvalidRoundtableIntervention) {
		t.Fatalf("historical pause error = %v", err)
	}
	if _, err := authority.SkipSeat(ctx, SkipSeatCommand{
		SessionID: "session-attempts", RoundID: "round-1",
		InterventionID: "skip-historical", ModeratorSeat: "seat-moderator",
		SeatID:                     "seat-planner",
		ExpectedMembershipRevision: bindings["seat-planner"].MembershipRevision,
		ExpectedSeatBindingDigest:  bindings["seat-planner"].BindingDigest,
		EmittedAt:                  utcTime(7), CorrelationID: interventionCorrelation,
	}); !errors.Is(err, ErrInvalidRoundtableIntervention) {
		t.Fatalf("historical skip error = %v", err)
	}
	after, err := journalStore.ReadStream(ctx, sessionStream("session-attempts"))
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("historical intervention appended fact: before=%d after=%d", len(before), len(after))
	}
}

func TestRecordRoundDispatchFailureIsDurableContentNegativeAndClosesRound(t *testing.T) {
	authority, _, journalStore := newRoundtableFixture(t)
	ctx := context.Background()
	bindings := setupMissionRoundForAttempts(t, authority, interventionCorrelation)
	command := RecordRoundDispatchFailureCommand{
		SessionID: "session-attempts", RoundID: "round-1",
		InterventionID: "dispatch-failure-1", ModeratorSeat: "seat-moderator",
		IncidentID: "incident-dispatch-1", FailureCode: "execution_setup_failed",
		FailureStage: "round_dispatch_setup", Retryable: true,
		EmittedAt: utcTime(5), CorrelationID: interventionCorrelation,
	}
	unauthorized := command
	unauthorized.InterventionID = "dispatch-failure-unauthorized"
	unauthorized.ModeratorSeat = "seat-planner"
	if _, err := authority.RecordRoundDispatchFailure(ctx, unauthorized); !errors.Is(
		err, ErrRoundtableNotModerator,
	) {
		t.Fatalf("non-moderator dispatch failure error = %v", err)
	}
	unsafe := command
	unsafe.InterventionID = "dispatch-failure-unsafe"
	unsafe.FailureCode = "provider returned private body"
	if _, err := authority.RecordRoundDispatchFailure(ctx, unsafe); !errors.Is(
		err, ErrInvalidRoundtableIntervention,
	) {
		t.Fatalf("unsafe dispatch metadata error = %v", err)
	}
	view, err := authority.RecordRoundDispatchFailure(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	receipt := view.Interventions[command.InterventionID]
	if receipt.Kind != InterventionRoundDispatchFailure ||
		receipt.IncidentID != command.IncidentID || receipt.FailureCode != command.FailureCode ||
		receipt.FailureStage != command.FailureStage || !receipt.Retryable {
		t.Fatalf("dispatch failure receipt = %#v", receipt)
	}
	eventsBefore, err := journalStore.ReadStream(ctx, sessionStream(command.SessionID))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authority.RecordRoundDispatchFailure(ctx, command); err != nil {
		t.Fatalf("idempotent dispatch failure: %v", err)
	}
	eventsAfter, _ := journalStore.ReadStream(ctx, sessionStream(command.SessionID))
	if len(eventsAfter) != len(eventsBefore) {
		t.Fatalf("idempotent failure appended facts: before=%d after=%d", len(eventsBefore), len(eventsAfter))
	}
	start := seatAttemptStartCommand(
		bindings["seat-planner"], "seat-planner", "attempt-planner-1", 1,
		"segment-planner-1", 6,
	)
	if _, err := authority.StartSeatAttempt(ctx, start); !errors.Is(err, ErrInvalidRoundtableSeatAttempt) {
		t.Fatalf("failed round admitted Attempt: %v", err)
	}
	replayed, err := authority.ReadView(ctx, command.SessionID)
	if err != nil || replayed.Digest != view.Digest ||
		replayed.Interventions[command.InterventionID] != receipt {
		t.Fatalf("dispatch failure replay = %#v err=%v", replayed, err)
	}
	last := eventsAfter[len(eventsAfter)-1]
	if last.Type != FactRoundDispatchFailed || strings.Contains(string(last.PayloadJSON), "prompt") ||
		strings.Contains(string(last.PayloadJSON), "body") {
		t.Fatalf("dispatch failure fact = %s %s", last.Type, last.PayloadJSON)
	}
}

func seatAttemptStartCommand(
	binding FrozenSeatBinding,
	seatID string,
	attemptID string,
	attemptNumber int,
	segmentID string,
	second int,
) StartSeatAttemptCommand {
	return StartSeatAttemptCommand{
		SessionID: "session-attempts", RoundID: "round-1", SeatID: seatID,
		AttemptID: attemptID, AttemptNumber: attemptNumber,
		ExecutionTeamID: "roundtable-team-1", WorkItemID: "work-" + attemptID,
		RunID: "run-" + attemptID, SegmentID: segmentID, ClaimGeneration: 1,
		RuntimeInstanceID:      binding.ExecutionBinding.RuntimeInstanceID,
		AgentInstanceID:        "roundtable-agent-" + seatID,
		MembershipRevision:     binding.MembershipRevision,
		SeatBindingDigest:      binding.BindingDigest,
		ExecutionBindingDigest: binding.ExecutionBinding.BindingDigest,
		ContextCapsuleDigest:   strings.Repeat("d", 64), EmittedAt: utcTime(second),
		CorrelationID: interventionCorrelation,
	}
}

func findIntervention(view View, kind, attemptID string) Intervention {
	for _, intervention := range view.Interventions {
		if intervention.Kind == kind && intervention.AttemptID == attemptID {
			return intervention
		}
	}
	return Intervention{}
}
