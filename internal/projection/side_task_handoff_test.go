package projection

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/work"
)

func TestSideTaskProjectionIsImmutableAndFailurePreservesPriorView(t *testing.T) {
	db := openProjectionTestDB(t)
	store := journal.NewStore(db)
	now := time.Date(2026, 8, 3, 3, 0, 0, 0, time.UTC)
	authority, err := work.NewAuthority(
		store,
		func() time.Time { return now },
		bytes.NewReader(bytes.Repeat([]byte{0x31}, 1024)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := authority.InitializeRunIdentityIndex(context.Background()); err != nil {
		t.Fatal(err)
	}
	admission := work.SideTaskAdmissionInput{
		SideTaskID: "side-projection-1", ParentMissionID: "mission-parent-1",
		ParentTeamInstanceID: "team-parent-1", ParentTaskID: "task-parent-1",
		ParentRunID: "run-parent-1", ParentClaimGeneration: 1,
		ParentExecutionDigest:       strings.Repeat("9", 64),
		SideExecutionTeamInstanceID: "team-side-projection-1",
		Purpose:                     "verification", Mode: "decision_required", Title: "Verify bounded result",
		ProposalDigest: strings.Repeat("a", 64), InputArtifactDigest: strings.Repeat("b", 64),
		ExpectedViewVersion: strings.Repeat("c", 64),
		PermissionScopes:    []string{"read:project"}, Confirmed: true,
		CorrelationID: "33333333-3333-4333-8333-333333333333",
	}
	if _, err := authority.AdmitSideTask(context.Background(), admission); err != nil {
		t.Fatal(err)
	}
	handoffPayload, err := json.Marshal(map[string]any{
		"side_task_id":                    admission.SideTaskID,
		"side_execution_team_instance_id": admission.SideExecutionTeamInstanceID,
		"source_work_item_id":             "work-side-1", "source_run_id": "run-side-1",
		"source_generation": int64(1), "source_evidence_id": "evidence-source-1",
		"source_evidence_digest":   strings.Repeat("d", 64),
		"verifier_evidence_id":     "evidence-verifier-1",
		"verifier_evidence_digest": strings.Repeat("e", 64),
		"handoff_version":          1, "handoff_digest": strings.Repeat("f", 64),
		"summary_artifact_digest": strings.Repeat("1", 64),
		"mode":                    "decision_required", "status": "decision_required",
		"usage_observed": false, "usage_microunits": int64(0), "usage_currency": "",
		"decision_timeout_seconds": int64(3600),
		"decision_deadline":        now.Add(time.Hour).Format(time.RFC3339Nano),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(context.Background(), journal.Event{
		ID: "event-side-handoff", StreamID: "side-task/" + admission.SideTaskID,
		Seq: 2, IdempotencyKey: "event-side-handoff", Type: "SideTaskHandoffCommitted",
		SchemaVersion: 1, EmittedAt: now, CorrelationID: admission.CorrelationID,
		PayloadJSON: handoffPayload,
	}); err != nil {
		t.Fatal(err)
	}

	projection := New(db)
	if err := projection.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	accepted := projection.GlobalReadView()
	record, ok := accepted.SideTaskHandoff(admission.SideTaskID)
	if !ok || record.Status != "decision_required" ||
		record.SummaryArtifactDigest != strings.Repeat("1", 64) {
		t.Fatalf("record = %#v, ok=%v", record, ok)
	}
	record.PermissionScopes[0] = "mutated"
	again, _ := accepted.SideTaskHandoff(admission.SideTaskID)
	if again.PermissionScopes[0] != "read:project" {
		t.Fatalf("view leaked mutable collection: %#v", again.PermissionScopes)
	}
	decisionPayload, err := json.Marshal(map[string]any{
		"decision_id": "decision-followup-1", "side_task_id": admission.SideTaskID,
		"parent_task_id": admission.ParentTaskID, "parent_run_id": admission.ParentRunID,
		"parent_claim_generation": admission.ParentClaimGeneration,
		"side_task_generation":    int64(1), "handoff_version": 1,
		"handoff_digest":        strings.Repeat("f", 64),
		"expected_view_version": strings.Repeat("c", 64),
		"decision":              "request_followup", "context_packet_digest": "",
		"effect_digest": strings.Repeat("2", 64), "resulting_status": "decided",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(context.Background(), journal.Event{
		ID: "event-side-followup", StreamID: "side-task/" + admission.SideTaskID,
		Seq: 3, IdempotencyKey: "event-side-followup", Type: "SideTaskDecisionCommitted",
		SchemaVersion: 1, EmittedAt: now, CorrelationID: admission.CorrelationID,
		PayloadJSON: decisionPayload,
	}); err != nil {
		t.Fatal(err)
	}
	if err := projection.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	accepted = projection.GlobalReadView()
	followup, ok := accepted.SideTaskHandoff(admission.SideTaskID)
	if !ok || followup.Decision != "request_followup" || followup.EffectStatus != "none" {
		t.Fatalf("followup = %#v", followup)
	}

	badPayload, err := json.Marshal(map[string]any{
		"decision_id": "decision-followup-1", "side_task_id": admission.SideTaskID,
		"handoff_version": 1, "handoff_digest": strings.Repeat("f", 64),
		"parent_task_id": admission.ParentTaskID, "parent_run_id": "run-substituted",
		"parent_claim_generation":  admission.ParentClaimGeneration,
		"followup_proposal_digest": strings.Repeat("3", 64),
		"effect_digest":            strings.Repeat("2", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(context.Background(), journal.Event{
		ID: "event-parent-bad", StreamID: "parent-handoff/" + admission.ParentTaskID,
		Seq: 1, IdempotencyKey: "event-parent-bad", Type: "ParentFollowupProposed",
		SchemaVersion: 1, EmittedAt: now.Add(time.Second),
		CorrelationID: admission.CorrelationID, PayloadJSON: badPayload,
	}); err != nil {
		t.Fatal(err)
	}
	if err := projection.Rebuild(context.Background()); err == nil {
		t.Fatal("malformed parent-handoff tuple rebuild error = nil")
	}
	preserved := projection.GlobalReadView()
	if preserved.Version() != accepted.Version() {
		t.Fatalf("failed rebuild replaced view: got %s want %s", preserved.Version(), accepted.Version())
	}
}

func TestSideTaskProjectionRejectsParentStreamSequenceAndSchemaWithoutPublishing(t *testing.T) {
	db := openProjectionTestDB(t)
	store := journal.NewStore(db)
	projection := New(db)
	if err := projection.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	accepted := projection.GlobalReadView()
	if _, err := store.Append(context.Background(), journal.Event{
		ID: "event-parent-gap", StreamID: "parent-handoff/task-gap", Seq: 2,
		IdempotencyKey: "event-parent-gap", Type: "ParentFollowupProposed",
		SchemaVersion: 1, EmittedAt: time.Date(2026, 8, 3, 3, 0, 0, 0, time.UTC),
		CorrelationID: "33333333-3333-4333-8333-333333333333", PayloadJSON: []byte(`{}`),
	}); err != nil {
		t.Fatal(err)
	}
	if err := projection.Rebuild(context.Background()); err == nil {
		t.Fatal("parent stream gap/schema rebuild error = nil")
	}
	if preserved := projection.GlobalReadView(); preserved.Version() != accepted.Version() {
		t.Fatalf("parent stream gap published view: got %s want %s", preserved.Version(), accepted.Version())
	}
	if _, err := projectSideTaskHandoffs([]journal.Event{{
		ID: "event-parent-schema", StreamID: "parent-handoff/task-schema", Seq: 1,
		Type: "ParentFollowupProposed", SchemaVersion: 2, PayloadJSON: []byte(`{}`),
	}}); err == nil {
		t.Fatal("parent stream schema error = nil")
	}
}

func TestParentHandoffProjectionRejectsCrossEventContinuationIdentityAndSecondCompletion(t *testing.T) {
	record := SideTaskHandoff{
		SideTaskID: "side-cross-1", ParentMissionID: "mission-parent-1",
		ParentTeamInstanceID: "team-parent-1", ParentTaskID: "work-parent-1",
		ParentRunID: "run-parent-1", ParentClaimGeneration: 1,
		SourceGeneration: 2, HandoffVersion: 1, HandoffDigest: strings.Repeat("a", 64),
		SummaryArtifactDigest: strings.Repeat("b", 64), DecisionID: "decision-1",
		Decision: "absorb", ContextPacketDigest: strings.Repeat("c", 64),
		ContinuationExecutionTeamInstanceID: "team-continuation-a",
		EffectDigest:                        strings.Repeat("d", 64), EffectStatus: "pending",
	}
	binding, err := json.Marshal(map[string]any{
		"decision_id": record.DecisionID, "side_task_id": record.SideTaskID,
		"handoff_version": record.HandoffVersion, "handoff_digest": record.HandoffDigest,
		"parent_mission_id": record.ParentMissionID, "parent_team_instance_id": record.ParentTeamInstanceID,
		"parent_task_id": record.ParentTaskID, "parent_run_id": record.ParentRunID,
		"parent_claim_generation": record.ParentClaimGeneration,
		"side_task_generation":    record.SourceGeneration, "action": "absorb",
		"context_packet_digest":                   record.ContextPacketDigest,
		"continuation_execution_team_instance_id": "team-continuation-b",
		"continuation_plan_digest":                strings.Repeat("e", 64), "effect_digest": record.EffectDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	records := map[string]SideTaskHandoff{record.SideTaskID: record}
	if err = applyParentHandoffProjection(records, journal.Event{
		StreamID: "parent-handoff/" + record.ParentTaskID,
		Type:     "ParentContinuationAuthorized", SchemaVersion: 1, PayloadJSON: binding,
	}); err == nil {
		t.Fatal("cross-event continuation Team identity drift accepted")
	}
	record.EffectStatus = "completed"
	record.ContinuationPlanDigest = strings.Repeat("e", 64)
	records[record.SideTaskID] = record
	completed, err := json.Marshal(map[string]any{
		"decision_id": record.DecisionID, "side_task_id": record.SideTaskID,
		"effect_kind": "continuation", "effect_digest": record.EffectDigest,
		"execution_team_instance_id": record.ContinuationExecutionTeamInstanceID,
		"execution_plan_digest":      record.ContinuationPlanDigest, "terminal_status": "succeeded",
		"terminal_evidence_id":     "evidence-terminal-1",
		"terminal_evidence_digest": strings.Repeat("f", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = applyParentHandoffProjection(records, journal.Event{
		StreamID: "parent-handoff/" + record.ParentTaskID,
		Type:     "ParentHandoffEffectCompleted", SchemaVersion: 1, PayloadJSON: completed,
	}); err == nil {
		t.Fatal("second completion after completed effect accepted")
	}
}

func TestSideTaskProjectionRejectsIllegalTimeoutBoundWithoutPublishing(t *testing.T) {
	db := openProjectionTestDB(t)
	store := journal.NewStore(db)
	now := time.Date(2026, 8, 3, 3, 0, 0, 0, time.UTC)
	authority, err := work.NewAuthority(store, func() time.Time { return now },
		bytes.NewReader(bytes.Repeat([]byte{0x41}, 1024)))
	if err != nil {
		t.Fatal(err)
	}
	if err := authority.InitializeRunIdentityIndex(context.Background()); err != nil {
		t.Fatal(err)
	}
	admission := work.SideTaskAdmissionInput{
		SideTaskID: "side-timeout-bound-1", ParentMissionID: "mission-parent-1",
		ParentTeamInstanceID: "team-parent-1", ParentTaskID: "task-parent-1",
		ParentRunID: "run-parent-1", ParentClaimGeneration: 1,
		ParentExecutionDigest: strings.Repeat("9", 64), SideExecutionTeamInstanceID: "team-side-1",
		Purpose: "verification", Mode: "decision_required", Title: "Verify bounded result",
		ProposalDigest: strings.Repeat("a", 64), InputArtifactDigest: strings.Repeat("b", 64),
		ExpectedViewVersion: strings.Repeat("c", 64), PermissionScopes: []string{}, Confirmed: true,
		CorrelationID: "33333333-3333-4333-8333-333333333333",
	}
	if _, err := authority.AdmitSideTask(context.Background(), admission); err != nil {
		t.Fatal(err)
	}
	projection := New(db)
	if err := projection.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	accepted := projection.GlobalReadView()
	payload, err := json.Marshal(map[string]any{
		"side_task_id": admission.SideTaskID, "side_execution_team_instance_id": admission.SideExecutionTeamInstanceID,
		"source_work_item_id": "work-side-1", "source_run_id": "run-side-1", "source_generation": int64(1),
		"source_evidence_id": "evidence-source-1", "source_evidence_digest": strings.Repeat("d", 64),
		"verifier_evidence_id": "", "verifier_evidence_digest": "", "handoff_version": 1,
		"handoff_digest": strings.Repeat("f", 64), "summary_artifact_digest": strings.Repeat("1", 64),
		"mode": "decision_required", "status": "decision_required", "usage_observed": false,
		"usage_microunits": int64(0), "usage_currency": "", "decision_timeout_seconds": int64(2592001),
		"decision_deadline": now.Add(2592001 * time.Second).Format(time.RFC3339Nano),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(context.Background(), journal.Event{
		ID: "event-illegal-timeout", StreamID: "side-task/" + admission.SideTaskID,
		Seq: 2, IdempotencyKey: "event-illegal-timeout", Type: "SideTaskHandoffCommitted",
		SchemaVersion: 1, EmittedAt: now, CorrelationID: admission.CorrelationID, PayloadJSON: payload,
	}); err != nil {
		t.Fatal(err)
	}
	if err := projection.Rebuild(context.Background()); err == nil {
		t.Fatal("illegal timeout rebuild error = nil")
	}
	if preserved := projection.GlobalReadView(); preserved.Version() != accepted.Version() {
		t.Fatalf("illegal timeout published view: got %s want %s", preserved.Version(), accepted.Version())
	}
}
