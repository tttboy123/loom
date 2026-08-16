package execution

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/permissions"
)

func TestToolRecoveryAuthorityPreviewAndAbortAreContentFree(t *testing.T) {
	ctx, store, adapter, proposal, executionID := toolRecoveryAuthorityFixture(t, "preview-abort")
	authority, err := NewToolRecoveryAuthority(
		store, nil, nil,
		func() time.Time { return time.Date(2026, 8, 14, 20, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}

	preview, err := authority.Preview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if preview.SchemaVersion != 1 || len(preview.Candidates) != 1 {
		t.Fatalf("preview = %#v", preview)
	}
	candidate := preview.Candidates[0]
	if candidate.SchemaVersion != 1 || candidate.Status != ToolRecoveryCandidateAvailable ||
		candidate.ExecutionID != executionID || candidate.JobID != proposal.JobID ||
		candidate.CallDigest != callDigest(proposal.Call) || candidate.Tool != permissions.ToolBash ||
		candidate.OperationID != proposal.OperationID || candidate.IncidentID != proposal.JourneyID ||
		candidate.CandidateDigest == "" ||
		len(candidate.AvailableActions) != 1 ||
		candidate.AvailableActions[0] != ToolRecoveryAbortAttempt {
		t.Fatalf("candidate = %#v", candidate)
	}

	input := ToolRecoveryDecisionInput{
		SchemaVersion:   1,
		DecisionID:      "10101010-1010-4010-8010-101010101010",
		CorrelationID:   "20202020-2020-4020-8020-202020202020",
		PrincipalID:     "local-user",
		Action:          ToolRecoveryAbortAttempt,
		CandidateDigest: candidate.CandidateDigest,
	}
	decision, err := authority.Resolve(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if decision.SchemaVersion != 1 || decision.DecisionID != input.DecisionID ||
		decision.Action != input.Action || decision.CandidateDigest != input.CandidateDigest ||
		decision.ExecutionID != executionID || decision.EventID == "" || decision.StreamID == "" {
		t.Fatalf("decision = %#v", decision)
	}
	replayed, err := authority.Resolve(ctx, input)
	if err != nil || replayed != decision {
		t.Fatalf("idempotent replay = %#v, %v", replayed, err)
	}

	events, err := store.ReadStream(ctx, decision.StreamID)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		proposal.Call.Command, "Authorization", "api_key", "provider_response", "tool_result_body",
	} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("recovery decision leaked %q: %s", forbidden, encoded)
		}
	}

	snapshot, err := ReplaySnapshot(events)
	if err != nil {
		t.Fatal(err)
	}
	record, found := snapshot.Record(executionID)
	if !found || record.Status != "recovery_aborted" ||
		record.RecoveryDecisionID != input.DecisionID ||
		record.RecoveryDecision != string(ToolRecoveryAbortAttempt) ||
		record.RecoveryResolvedAt == "" {
		t.Fatalf("resolved record = %#v found=%t", record, found)
	}
	if adapter.executor.(*recordingExecutor).runCount() != 0 {
		t.Fatal("recovery decision re-executed the original ToolCall")
	}
	result, err := adapter.Execute(ctx, proposal)
	if err != nil || result.RecoveryRequired || result.Note != "recovery aborted" {
		t.Fatalf("resolved execution replay = %#v, %v", result, err)
	}
	if adapter.executor.(*recordingExecutor).runCount() != 0 {
		t.Fatal("resolved execution replay ran the original ToolCall")
	}
}

func TestToolRecoveryAuthorityRequiresTrustedObservationAndReplacement(t *testing.T) {
	ctx, store, _, _, _ := toolRecoveryAuthorityFixture(t, "trusted-resolution")
	base, err := NewToolRecoveryAuthority(store, nil, nil, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := base.Preview(ctx)
	if err != nil || len(preview.Candidates) != 1 {
		t.Fatalf("preview = %#v, %v", preview, err)
	}
	candidate := preview.Candidates[0]

	for _, action := range []ToolRecoveryAction{
		ToolRecoveryAcceptObservedEffect,
		ToolRecoveryRetryInNewAttempt,
	} {
		_, err := base.Resolve(ctx, ToolRecoveryDecisionInput{
			SchemaVersion: 1, DecisionID: "30303030-3030-4030-8030-303030303030",
			CorrelationID: "40404040-4040-4040-8040-404040404040",
			PrincipalID:   "local-user", Action: action,
			CandidateDigest: candidate.CandidateDigest,
		})
		if !errors.Is(err, ErrToolRecoveryUnavailable) {
			t.Fatalf("action %s error = %v", action, err)
		}
	}

	observation := ToolRecoveryObservation{
		SchemaVersion: 1, CandidateDigest: candidate.CandidateDigest,
		EvidenceID:         "evidence-observed-effect",
		ObservationDigest:  strings.Repeat("a", 64),
		OutputDigest:       "sha256:" + strings.Repeat("b", 64),
		ChangedFilesDigest: "sha256:" + strings.Repeat("c", 64),
	}
	replacement := ToolRecoveryReplacement{
		SchemaVersion: 1, CandidateDigest: candidate.CandidateDigest,
		AttemptID: "attempt-replacement", RunID: "run-replacement",
		ExecutionBindingDigest: strings.Repeat("d", 64),
		ContextCapsuleDigest:   strings.Repeat("e", 64),
	}
	authority, err := NewToolRecoveryAuthority(
		store,
		toolRecoveryObservationResolverFixture{observation: observation},
		toolRecoveryReplacementResolverFixture{replacement: replacement},
		time.Now,
	)
	if err != nil {
		t.Fatal(err)
	}
	preview, err = authority.Preview(ctx)
	if err != nil || len(preview.Candidates) != 1 ||
		len(preview.Candidates[0].AvailableActions) != 3 {
		t.Fatalf("capability preview = %#v, %v", preview, err)
	}

	accepted, err := authority.Resolve(ctx, ToolRecoveryDecisionInput{
		SchemaVersion: 1, DecisionID: "50505050-5050-4050-8050-505050505050",
		CorrelationID: "60606060-6060-4060-8060-606060606060",
		PrincipalID:   "local-user", Action: ToolRecoveryAcceptObservedEffect,
		CandidateDigest: candidate.CandidateDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if accepted.EvidenceID != observation.EvidenceID ||
		accepted.ObservationDigest != observation.ObservationDigest ||
		accepted.OutputDigest != observation.OutputDigest ||
		accepted.ChangedFilesDigest != observation.ChangedFilesDigest {
		t.Fatalf("accepted = %#v", accepted)
	}
}

func TestToolRecoveryAuthorityDoesNotAdvertiseTypedNilResolvers(t *testing.T) {
	ctx, store, _, _, _ := toolRecoveryAuthorityFixture(t, "typed-nil")
	var observations *toolRecoveryObservationResolverFixture
	var replacements *toolRecoveryReplacementResolverFixture
	authority, err := NewToolRecoveryAuthority(
		store, observations, replacements, time.Now,
	)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := authority.Preview(ctx)
	if err != nil || len(preview.Candidates) != 1 {
		t.Fatalf("preview = %#v, %v", preview, err)
	}
	if actions := preview.Candidates[0].AvailableActions; len(actions) != 1 || actions[0] != ToolRecoveryAbortAttempt {
		t.Fatalf("available actions = %#v, want abort only", actions)
	}
}

func TestToolRecoveryAuthorityFreezesReplacementAttemptAndRejectsResolverDrift(t *testing.T) {
	ctx, store, _, _, _ := toolRecoveryAuthorityFixture(t, "replacement")
	base, err := NewToolRecoveryAuthority(store, nil, nil, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := base.Preview(ctx)
	if err != nil || len(preview.Candidates) != 1 {
		t.Fatalf("preview = %#v, %v", preview, err)
	}
	candidate := preview.Candidates[0]
	replacement := ToolRecoveryReplacement{
		SchemaVersion: 1, CandidateDigest: candidate.CandidateDigest,
		AttemptID: "attempt-replacement", RunID: "run-replacement",
		ExecutionBindingDigest: strings.Repeat("d", 64),
		ContextCapsuleDigest:   strings.Repeat("e", 64),
	}
	authority, err := NewToolRecoveryAuthority(
		store, nil,
		toolRecoveryReplacementResolverFixture{replacement: replacement},
		time.Now,
	)
	if err != nil {
		t.Fatal(err)
	}
	input := ToolRecoveryDecisionInput{
		SchemaVersion: 1, DecisionID: "11112222-3333-4444-8555-666677778888",
		CorrelationID: "99990000-aaaa-4bbb-8ccc-ddddeeeeffff",
		PrincipalID:   "local-user", Action: ToolRecoveryRetryInNewAttempt,
		CandidateDigest: candidate.CandidateDigest,
	}
	decision, err := authority.Resolve(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if decision.ReplacementAttemptID != replacement.AttemptID ||
		decision.ReplacementRunID != replacement.RunID ||
		decision.ExecutionBindingDigest != replacement.ExecutionBindingDigest ||
		decision.ContextCapsuleDigest != replacement.ContextCapsuleDigest {
		t.Fatalf("replacement decision = %#v", decision)
	}

	driftCtx, driftStore, _, _, _ := toolRecoveryAuthorityFixture(t, "replacement-drift")
	driftBase, _ := NewToolRecoveryAuthority(driftStore, nil, nil, time.Now)
	driftPreview, _ := driftBase.Preview(driftCtx)
	driftCandidate := driftPreview.Candidates[0]
	drifted := replacement
	drifted.CandidateDigest = strings.Repeat("f", 64)
	driftedAuthority, _ := NewToolRecoveryAuthority(
		driftStore, nil,
		toolRecoveryReplacementResolverFixture{replacement: drifted},
		time.Now,
	)
	input.CandidateDigest = driftCandidate.CandidateDigest
	if _, err := driftedAuthority.Resolve(driftCtx, input); !errors.Is(err, ErrToolRecoveryEvidence) {
		t.Fatalf("replacement drift error = %v", err)
	}
}

func TestToolRecoveryResolvedReplayRejectsUnknownFieldsAndCandidateSubstitution(t *testing.T) {
	ctx, store, _, _, _ := toolRecoveryAuthorityFixture(t, "strict-replay")
	authority, _ := NewToolRecoveryAuthority(store, nil, nil, time.Now)
	preview, _ := authority.Preview(ctx)
	decision, err := authority.Resolve(ctx, ToolRecoveryDecisionInput{
		SchemaVersion: 1, DecisionID: "12121212-1212-4212-8212-121212121212",
		CorrelationID: "13131313-1313-4313-8313-131313131313",
		PrincipalID:   "local-user", Action: ToolRecoveryAbortAttempt,
		CandidateDigest: preview.Candidates[0].CandidateDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	events, err := store.ReadStream(ctx, decision.StreamID)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*journal.Event){
		func(event *journal.Event) {
			var payload map[string]any
			if err := json.Unmarshal(event.PayloadJSON, &payload); err != nil {
				t.Fatal(err)
			}
			payload["secret"] = "must-not-be-accepted"
			event.PayloadJSON, _ = json.Marshal(payload)
		},
		func(event *journal.Event) {
			var payload map[string]any
			if err := json.Unmarshal(event.PayloadJSON, &payload); err != nil {
				t.Fatal(err)
			}
			payload["candidate_digest"] = strings.Repeat("9", 64)
			event.PayloadJSON, _ = json.Marshal(payload)
		},
	} {
		mutated := append([]journal.Event(nil), events...)
		mutate(&mutated[len(mutated)-1])
		if _, err := ReplaySnapshot(mutated); !errors.Is(err, ErrInvalidExecutionEvent) {
			t.Fatalf("mutated replay error = %v", err)
		}
	}
}

func TestToolRecoveryAuthorityCASAllowsOneDecisionAndNeverRunsExecutor(t *testing.T) {
	ctx, store, adapter, _, _ := toolRecoveryAuthorityFixture(t, "cas")
	authority, err := NewToolRecoveryAuthority(store, nil, nil, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := authority.Preview(ctx)
	if err != nil || len(preview.Candidates) != 1 {
		t.Fatalf("preview = %#v, %v", preview, err)
	}
	base := ToolRecoveryDecisionInput{
		SchemaVersion: 1,
		CorrelationID: "70707070-7070-4070-8070-707070707070",
		PrincipalID:   "local-user", Action: ToolRecoveryAbortAttempt,
		CandidateDigest: preview.Candidates[0].CandidateDigest,
	}
	inputs := []ToolRecoveryDecisionInput{base, base}
	inputs[0].DecisionID = "80808080-8080-4080-8080-808080808080"
	inputs[1].DecisionID = "90909090-9090-4090-8090-909090909090"

	start := make(chan struct{})
	errs := make([]error, len(inputs))
	var wait sync.WaitGroup
	for index := range inputs {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			_, errs[index] = authority.Resolve(ctx, inputs[index])
		}(index)
	}
	close(start)
	wait.Wait()
	succeeded, conflicted := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrToolRecoveryConflict):
			conflicted++
		default:
			t.Fatalf("unexpected decision error = %v", err)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf("succeeded=%d conflicted=%d errors=%#v", succeeded, conflicted, errs)
	}
	if adapter.executor.(*recordingExecutor).runCount() != 0 {
		t.Fatal("concurrent recovery resolution ran the original ToolCall")
	}
}

func toolRecoveryAuthorityFixture(
	t *testing.T,
	suffix string,
) (context.Context, *journal.Store, *Adapter, Proposal, string) {
	t.Helper()
	ctx := context.Background()
	store := openExecStore(t)
	projection, err := mustProfileProjection(t, store, "tool-recovery-"+suffix, []string{"src/**"})
	if err != nil {
		t.Fatal(err)
	}
	now := func() time.Time { return time.Date(2026, 8, 14, 19, 30, 0, 0, time.UTC) }
	adapter := mustAdapter(t, store, projection, &recordingExecutor{}, now)
	proposal := Proposal{
		JobID: execTestJobA, OperationID: "tool-recovery-" + suffix,
		JourneyID: "incident-tool-recovery-" + suffix,
		Call:      permissions.ProposedCall{Tool: permissions.ToolBash, Command: "private-recovery-command"},
	}
	call := callDigest(proposal.Call)
	executionID := executionID(proposal.JobID, call, proposal.OperationID)
	streamID := executionStreamID(proposal.JobID, executionID)
	events, err := store.ReadAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	heads := streamHeads(events)
	proposed, err := adapter.buildEvent(
		"proposed", streamID, proposal.JobID, executionID, call,
		proposal, 1, now(), proposal.JourneyID,
	)
	if err != nil {
		t.Fatal(err)
	}
	allowed, err := adapter.buildAllowedEvent(streamID, executionID, now())
	if err != nil {
		t.Fatal(err)
	}
	recovery, err := adapter.buildRecoveryRequiredEvent(
		streamID, executionID, proposal.JourneyID, now(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.appendCAS(
		ctx, events, heads, streamID, []journal.Event{proposed, allowed, recovery},
	); err != nil {
		t.Fatal(err)
	}
	return ctx, store, adapter, proposal, executionID
}

type toolRecoveryObservationResolverFixture struct {
	observation ToolRecoveryObservation
	err         error
}

func (fixture toolRecoveryObservationResolverFixture) ResolveToolRecoveryObservation(
	context.Context,
	ToolRecoveryCandidate,
) (ToolRecoveryObservation, error) {
	return fixture.observation, fixture.err
}

type toolRecoveryReplacementResolverFixture struct {
	replacement ToolRecoveryReplacement
	err         error
}

func (fixture toolRecoveryReplacementResolverFixture) ResolveToolRecoveryReplacement(
	context.Context,
	ToolRecoveryCandidate,
) (ToolRecoveryReplacement, error) {
	return fixture.replacement, fixture.err
}
