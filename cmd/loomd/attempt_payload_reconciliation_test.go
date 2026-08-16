package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/runtime/nativeadapter"
	"loom-pi-rebuild/internal/work"
)

type productAttemptPayloadDiagnosticFixture struct {
	records []nativeadapter.AgentAttemptDiagnostic
}

func (fixture *productAttemptPayloadDiagnosticFixture) RecordAgentAttemptDiagnostic(
	_ context.Context,
	diagnostic nativeadapter.AgentAttemptDiagnostic,
) error {
	fixture.records = append(fixture.records, diagnostic)
	return nil
}

func TestProductAttemptPayloadReconciliationEmitsSafeAttemptDiagnostics(t *testing.T) {
	fixture := &productAttemptPayloadDiagnosticFixture{}
	authority := attemptpayload.Authority{
		Scope: attemptpayload.Scope{
			ConversationID: "mission:team-reconcile", WorkItemID: "work-reconcile",
			RunID: "run-reconcile", ClaimGeneration: 3,
			RuntimeInstanceID:      "runtime-reconcile",
			ExecutionBindingDigest: strings.Repeat("a", 64),
			CapsuleDigest:          strings.Repeat("b", 64),
		},
		ClaimID:         "11111111-1111-4111-8111-111111111111",
		AgentInstanceID: "agent-reconcile",
		IncidentID:      "22222222-2222-4222-8222-222222222222",
	}
	binding := attemptpayload.Binding{
		PayloadID: "payload-reconcile", Scope: authority.Scope,
		CallID: "context-read-reconcile", Sequence: 1,
		ContentType: "application/json", ContentDigest: strings.Repeat("c", 64),
	}
	executionBinding := work.FrozenExecutionBinding{
		ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
		ModelID: "deepseek-chat", BindingDigest: binding.ExecutionBindingDigest,
	}
	report := work.AttemptPayloadReconciliationReport{Outcomes: []work.AttemptPayloadReconciliation{
		{
			Authority: authority, Binding: binding, ExecutionBinding: executionBinding,
			RunPhase: "terminal", Result: work.AttemptPayloadReconcileRepaired,
		},
		{
			Authority: authority, Binding: binding, ExecutionBinding: executionBinding,
			RunPhase: "terminal", Result: work.AttemptPayloadReconcileBlocked,
			ErrorCode: "attempt_payload_unavailable",
		},
		{
			Authority: authority, Binding: binding, ExecutionBinding: executionBinding,
			RunPhase: "terminal", Result: work.AttemptPayloadReconcileAlreadyDelivered,
		},
		{
			Authority: authority, Binding: binding, ExecutionBinding: executionBinding,
			RunPhase: "terminal", Result: work.AttemptPayloadReconcileMissing,
		},
	}}
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	if err := recordProductAttemptPayloadReconciliation(
		context.Background(), fixture, func() time.Time { return now }, report,
	); err != nil {
		t.Fatal(err)
	}
	if len(fixture.records) != 2 {
		t.Fatalf("diagnostics = %#v", fixture.records)
	}
	repaired, blocked := fixture.records[0], fixture.records[1]
	for _, diagnostic := range fixture.records {
		if diagnostic.IncidentID != authority.IncidentID ||
			diagnostic.ProviderID != "deepseek" ||
			diagnostic.ProviderAccountID != "deepseek.primary" ||
			diagnostic.ModelID != "deepseek-chat" ||
			diagnostic.WorkItemID != binding.WorkItemID ||
			diagnostic.RunID != binding.RunID || diagnostic.ClaimGeneration != 3 ||
			diagnostic.RuntimeInstanceID != binding.RuntimeInstanceID ||
			diagnostic.AgentInstanceID != authority.AgentInstanceID ||
			diagnostic.ExecutionBindingDigest != binding.ExecutionBindingDigest ||
			diagnostic.ContextCapsuleDigest != binding.CapsuleDigest ||
			diagnostic.Stage != "context_delivery_reconcile" {
			t.Fatalf("diagnostic identity = %#v", diagnostic)
		}
	}
	if repaired.Result != "succeeded" || repaired.ErrorCode != "" || repaired.Retryable ||
		blocked.Result != "failed" || blocked.ErrorCode != "attempt_payload_unavailable" ||
		!blocked.Retryable {
		t.Fatalf("diagnostic results = %#v", fixture.records)
	}
}

func TestProductAgentAttemptRestartReconciliationEmitsGovernedDiagnostics(t *testing.T) {
	fixture := &productAttemptPayloadDiagnosticFixture{}
	authority := attemptpayload.Authority{
		Scope: attemptpayload.Scope{
			ConversationID: "mission:team-restart", WorkItemID: "work-restart",
			RunID: "run-restart", ClaimGeneration: 4,
			RuntimeInstanceID:      "runtime-restart",
			ExecutionBindingDigest: strings.Repeat("d", 64),
			CapsuleDigest:          strings.Repeat("e", 64),
		},
		ClaimID:         "33333333-3333-4333-8333-333333333333",
		AgentInstanceID: "agent-restart",
		IncidentID:      "44444444-4444-4444-8444-444444444444",
	}
	binding := work.AttemptLoopBinding{
		SchemaVersion: 1, AttemptID: authority.RunID, TeamInstanceID: "team-restart",
		PayloadAuthority: authority,
	}
	executionBinding := work.FrozenExecutionBinding{
		ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
		ModelID: "deepseek-chat", BindingDigest: authority.ExecutionBindingDigest,
	}
	report := work.AgentAttemptRestartReport{Outcomes: []work.AgentAttemptRestartOutcome{
		{
			Binding: binding, ExecutionBinding: executionBinding,
			Disposition: work.AgentAttemptRestartPreModelResume,
			TurnID:      "turn-2", TurnSequence: 2,
			CheckpointDigest: strings.Repeat("f", 64), InputIDs: []string{"input-1"},
		},
		{
			Binding: binding, ExecutionBinding: executionBinding,
			Disposition: work.AgentAttemptRestartProviderOutcomeUncertain,
			TurnID:      "turn-1", TurnSequence: 1, StepID: "step-1", StepSequence: 1,
			ModelRequestID: "request-1",
		},
		{
			Binding: binding, ExecutionBinding: executionBinding,
			Disposition: work.AgentAttemptRestartRecoveryBlocked,
			ErrorCode:   "agent_input_recovery_unavailable",
		},
	}}
	now := time.Date(2026, 8, 14, 9, 0, 0, 0, time.UTC)
	if err := recordProductAgentAttemptRestartReconciliation(
		context.Background(), fixture, func() time.Time { return now }, report,
	); err != nil {
		t.Fatal(err)
	}
	if len(fixture.records) != 3 {
		t.Fatalf("restart diagnostics = %#v", fixture.records)
	}
	for _, diagnostic := range fixture.records {
		if diagnostic.IncidentID != authority.IncidentID ||
			diagnostic.ProviderID != executionBinding.ProviderID ||
			diagnostic.ProviderAccountID != executionBinding.ProviderAccountID ||
			diagnostic.ModelID != executionBinding.ModelID ||
			diagnostic.WorkItemID != authority.WorkItemID || diagnostic.RunID != authority.RunID ||
			diagnostic.ClaimGeneration != authority.ClaimGeneration ||
			diagnostic.RuntimeInstanceID != authority.RuntimeInstanceID ||
			diagnostic.AgentInstanceID != authority.AgentInstanceID ||
			diagnostic.ExecutionBindingDigest != authority.ExecutionBindingDigest ||
			diagnostic.ContextCapsuleDigest != authority.CapsuleDigest ||
			diagnostic.Stage != "agent_attempt_reconcile" || diagnostic.Result != "failed" {
			t.Fatalf("restart diagnostic identity = %#v", diagnostic)
		}
	}
	if fixture.records[0].ErrorCode != "agent_input_resume_required" ||
		fixture.records[0].Retryable ||
		fixture.records[1].ErrorCode != "provider_outcome_uncertain" ||
		fixture.records[1].Retryable ||
		fixture.records[2].ErrorCode != "agent_input_recovery_unavailable" ||
		fixture.records[2].Retryable {
		t.Fatalf("restart diagnostic governance = %#v", fixture.records)
	}
}
