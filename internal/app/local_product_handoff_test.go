package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/work"
)

func TestParentContinuationGateBlocksOnlyExactPendingGeneration(t *testing.T) {
	db := openTeamCanaryDB(t)
	store := journal.NewStore(db)
	now := time.Date(2026, 8, 3, 8, 0, 0, 0, time.UTC)
	authority, err := work.NewAuthority(store, func() time.Time { return now }, bytes.NewReader(bytes.Repeat([]byte{1}, 1024)))
	if err != nil {
		t.Fatal(err)
	}
	if err = authority.InitializeRunIdentityIndex(context.Background()); err != nil {
		t.Fatal(err)
	}
	admission := work.SideTaskAdmissionInput{SideTaskID: "side-gate-1", ParentMissionID: "mission/team-parent", ParentTeamInstanceID: "team-parent", ParentTaskID: "work-parent", ParentRunID: "run-parent", ParentClaimGeneration: 2, ParentExecutionDigest: strings.Repeat("9", 64), SideExecutionTeamInstanceID: "team-side", Purpose: "research", Mode: "decision_required", Title: "Research", ProposalDigest: strings.Repeat("a", 64), InputArtifactDigest: strings.Repeat("b", 64), ExpectedViewVersion: strings.Repeat("c", 64), PermissionScopes: []string{}, Confirmed: true, CorrelationID: "11111111-1111-4111-8111-111111111111"}
	if _, err = authority.AdmitSideTask(context.Background(), admission); err != nil {
		t.Fatal(err)
	}
	handoffPayload, err := json.Marshal(struct {
		SideTaskID                  string `json:"side_task_id"`
		SideExecutionTeamInstanceID string `json:"side_execution_team_instance_id"`
		SourceWorkItemID            string `json:"source_work_item_id"`
		SourceRunID                 string `json:"source_run_id"`
		SourceGeneration            int64  `json:"source_generation"`
		SourceEvidenceID            string `json:"source_evidence_id"`
		SourceEvidenceDigest        string `json:"source_evidence_digest"`
		VerifierEvidenceID          string `json:"verifier_evidence_id"`
		VerifierEvidenceDigest      string `json:"verifier_evidence_digest"`
		HandoffVersion              int    `json:"handoff_version"`
		HandoffDigest               string `json:"handoff_digest"`
		SummaryArtifactDigest       string `json:"summary_artifact_digest"`
		Mode                        string `json:"mode"`
		Status                      string `json:"status"`
		UsageObserved               bool   `json:"usage_observed"`
		UsageMicrounits             int64  `json:"usage_microunits"`
		UsageCurrency               string `json:"usage_currency"`
		DecisionTimeoutSeconds      int64  `json:"decision_timeout_seconds"`
		DecisionDeadline            string `json:"decision_deadline"`
	}{admission.SideTaskID, admission.SideExecutionTeamInstanceID,
		"work-side", "run-side", 1, "evidence-source", strings.Repeat("d", 64),
		"", "", 1, strings.Repeat("e", 64), strings.Repeat("f", 64),
		"decision_required", "decision_required", false, 0, "", 60,
		now.Add(time.Minute).Format(time.RFC3339Nano)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Append(context.Background(), journal.Event{
		ID: "evt-side-gate-handoff", StreamID: "side-task/" + admission.SideTaskID,
		Seq: 2, IdempotencyKey: "evt-side-gate-handoff", Type: "SideTaskHandoffCommitted",
		SchemaVersion: 1, EmittedAt: now, CorrelationID: admission.CorrelationID,
		PayloadJSON: handoffPayload,
	}); err != nil {
		t.Fatal(err)
	}
	admittedChild := admission
	admittedChild.SideTaskID = "side-gate-admitted-2"
	admittedChild.SideExecutionTeamInstanceID = "team-side-admitted-2"
	admittedChild.ProposalDigest = strings.Repeat("1", 64)
	admittedChild.InputArtifactDigest = strings.Repeat("2", 64)
	admittedChild.CorrelationID = "22222222-2222-4222-8222-222222222222"
	if _, err = authority.AdmitSideTask(context.Background(), admittedChild); err != nil {
		t.Fatal(err)
	}
	readModel := projection.New(db)
	if err = readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	gate, err := NewProjectionParentContinuationGate(readModel)
	if err != nil {
		t.Fatal(err)
	}
	if err = gate.AllowParentContinuation(context.Background(), "team-parent", 2); !errors.Is(err, ErrSideTaskProductConflict) {
		t.Fatalf("pending gate err=%v", err)
	}
	if err = gate.AllowParentContinuation(context.Background(), "team-parent", 3); err != nil {
		t.Fatalf("other generation err=%v", err)
	}
	if err = gate.AllowParentContinuation(context.Background(), admittedChild.SideExecutionTeamInstanceID, 0); !errors.Is(err, ErrSideTaskProductConflict) {
		t.Fatalf("admitted child must be reserved for Side-task recovery, err=%v", err)
	}
	decisionPayload, err := json.Marshal(map[string]any{
		"decision_id": "decision-continue-gate", "side_task_id": admission.SideTaskID,
		"parent_task_id": admission.ParentTaskID, "parent_run_id": admission.ParentRunID,
		"parent_claim_generation": admission.ParentClaimGeneration,
		"side_task_generation":    int64(1), "handoff_version": 1,
		"handoff_digest": strings.Repeat("e", 64), "expected_view_version": strings.Repeat("c", 64),
		"decision": "continue", "context_packet_digest": "",
		"effect_digest": strings.Repeat("3", 64), "resulting_status": "decided",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Append(context.Background(), journal.Event{
		ID: "evt-side-gate-decision", StreamID: "side-task/" + admission.SideTaskID,
		Seq: 3, IdempotencyKey: "evt-side-gate-decision", Type: "SideTaskDecisionCommitted",
		SchemaVersion: 1, EmittedAt: now, CorrelationID: admission.CorrelationID,
		PayloadJSON: decisionPayload,
	}); err != nil {
		t.Fatal(err)
	}
	continuationTeamID := "team-continuation-reserved"
	continuationPayload, err := json.Marshal(map[string]any{
		"decision_id": "decision-continue-gate", "side_task_id": admission.SideTaskID,
		"handoff_version": 1, "handoff_digest": strings.Repeat("e", 64),
		"parent_mission_id":       admission.ParentMissionID,
		"parent_team_instance_id": admission.ParentTeamInstanceID,
		"parent_task_id":          admission.ParentTaskID, "parent_run_id": admission.ParentRunID,
		"parent_claim_generation": admission.ParentClaimGeneration,
		"side_task_generation":    int64(1), "action": "continue", "context_packet_digest": "",
		"continuation_execution_team_instance_id": continuationTeamID,
		"continuation_plan_digest":                strings.Repeat("4", 64),
		"effect_digest":                           strings.Repeat("3", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Append(context.Background(), journal.Event{
		ID: "evt-parent-gate-continuation", StreamID: "parent-handoff/" + admission.ParentTaskID,
		Seq: 1, IdempotencyKey: "evt-parent-gate-continuation", Type: "ParentContinuationAuthorized",
		SchemaVersion: 1, EmittedAt: now, CorrelationID: admission.CorrelationID,
		PayloadJSON: continuationPayload,
	}); err != nil {
		t.Fatal(err)
	}
	if err = readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = gate.AllowParentContinuation(context.Background(), continuationTeamID, 0); !errors.Is(err, ErrSideTaskProductConflict) {
		t.Fatalf("pending continuation child must be reserved for Side-task recovery, err=%v", err)
	}
	if err = gate.AllowParentContinuation(context.Background(), "team-other", 0); err != nil {
		t.Fatalf("other team err=%v", err)
	}
}

func TestRecoveredSideTaskInputRequiresExactVersionTwoTimeoutAndBinding(t *testing.T) {
	record := projection.SideTaskHandoff{
		SideTaskID: "side-restart-1", ParentMissionID: "mission/team-parent",
		ParentTaskID: "work-parent", ParentRunID: "run-parent",
		ParentClaimGeneration: 3, Purpose: "research", Mode: "decision_required",
		ParentExecutionDigest: strings.Repeat("b", 64),
		Title:                 "Research exact restart", ProposalDigest: strings.Repeat("a", 64),
		PermissionScopes: []string{"read:repository"},
	}
	input := SideTaskInputArtifact{
		SchemaVersion: 2, SideTaskID: record.SideTaskID,
		ParentMissionID: record.ParentMissionID, ParentTaskID: record.ParentTaskID,
		ParentRunID: record.ParentRunID, ParentClaimGeneration: record.ParentClaimGeneration,
		ParentExecutionDigest: record.ParentExecutionDigest,
		Purpose:               record.Purpose, Mode: record.Mode, Title: record.Title,
		AuthorizedRequest: "Read only the authorized repository scope.",
		PermissionScopes:  append([]string{}, record.PermissionScopes...),
		ProposalDigest:    record.ProposalDigest, DecisionTimeoutSeconds: 900,
		CreatedAt: "2026-08-03T08:00:00Z",
	}
	if !validRecoveredSideTaskInput(record, input) {
		t.Fatal("exact Artifact v2 did not validate")
	}
	v1 := input
	v1.SchemaVersion = 1
	if validRecoveredSideTaskInput(record, v1) {
		t.Fatal("Artifact v1 must fail recovery closed")
	}
	unknown := input
	unknown.SchemaVersion = 3
	if validRecoveredSideTaskInput(record, unknown) {
		t.Fatal("unknown Artifact version must fail recovery closed")
	}
	changedTimeout := input
	changedTimeout.DecisionTimeoutSeconds = 59
	if validRecoveredSideTaskInput(record, changedTimeout) {
		t.Fatal("out-of-contract timeout must fail recovery closed")
	}
	changedBinding := input
	changedBinding.ParentRunID = "run-drifted"
	if validRecoveredSideTaskInput(record, changedBinding) {
		t.Fatal("binding drift must fail recovery closed")
	}
	report := input
	report.Mode = "report_only"
	record.Mode = "report_only"
	report.DecisionTimeoutSeconds = 0
	if !validRecoveredSideTaskInput(record, report) {
		t.Fatal("report-only zero timeout did not validate")
	}
	report.DecisionTimeoutSeconds = 60
	if validRecoveredSideTaskInput(record, report) {
		t.Fatal("report-only nonzero timeout must fail recovery closed")
	}
}

func TestSideTaskInputArtifactVersionTwoCanonicalBytesBindTimeout(t *testing.T) {
	input := SideTaskInputArtifact{
		SchemaVersion: 2, SideTaskID: "side-1", ParentMissionID: "mission/team-1",
		ParentTaskID: "work-1", ParentRunID: "run-1", ParentClaimGeneration: 1,
		ParentExecutionDigest: strings.Repeat("b", 64),
		Purpose:               "research", Mode: "decision_required", Title: "Research",
		AuthorizedRequest: "Find one bounded fact.", PermissionScopes: []string{},
		ProposalDigest: strings.Repeat("a", 64), DecisionTimeoutSeconds: 600,
		CreatedAt: "2026-08-03T08:00:00Z",
	}
	encoded, digest, err := canonicalSideTaskArtifact(input, maxSideTaskInputArtifactBytes)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"schema_version":2`) ||
		!strings.Contains(string(encoded), `"decision_timeout_seconds":600`) {
		t.Fatalf("canonical v2 bytes = %s", encoded)
	}
	changed := input
	changed.DecisionTimeoutSeconds = 601
	_, changedDigest, err := canonicalSideTaskArtifact(changed, maxSideTaskInputArtifactBytes)
	if err != nil {
		t.Fatal(err)
	}
	if changedDigest == digest {
		t.Fatal("timeout change did not change Artifact digest")
	}
	changedExecution := input
	changedExecution.ParentExecutionDigest = strings.Repeat("c", 64)
	_, changedExecutionDigest, err := canonicalSideTaskArtifact(
		changedExecution, maxSideTaskInputArtifactBytes,
	)
	if err != nil {
		t.Fatal(err)
	}
	if changedExecutionDigest == digest {
		t.Fatal("parent execution binding change did not change Artifact digest")
	}
	var decoded SideTaskInputArtifact
	if err := decodeCanonicalSideTaskArtifact(encoded, &decoded); err != nil || !reflect.DeepEqual(decoded, input) {
		t.Fatalf("decode=%#v err=%v", decoded, err)
	}
}

func TestSideTaskArtifactsRejectNullBoundsEnumsAndNonCanonicalCollections(t *testing.T) {
	input := SideTaskInputArtifact{
		SchemaVersion: 2, SideTaskID: "side-1", ParentMissionID: "mission/team-1",
		ParentTaskID: "work-1", ParentRunID: "run-1", ParentClaimGeneration: 1,
		ParentExecutionDigest: strings.Repeat("b", 64), Purpose: "research",
		Mode: "decision_required", Title: "Research", AuthorizedRequest: "Find one fact.",
		PermissionScopes: []string{}, ProposalDigest: strings.Repeat("a", 64),
		DecisionTimeoutSeconds: 600, CreatedAt: "2026-08-03T08:00:00Z",
	}
	assertInvalid := func(name string, value any, maximum int) {
		t.Helper()
		if _, _, err := canonicalSideTaskArtifact(value, maximum); !errors.Is(err, ErrInvalidSideTaskProduct) {
			t.Fatalf("%s canonical error = %v", name, err)
		}
	}
	nullScopes := input
	nullScopes.PermissionScopes = nil
	assertInvalid("null permission scopes", nullScopes, maxSideTaskInputArtifactBytes)
	oversized := input
	oversized.AuthorizedRequest = strings.Repeat("x", 4097)
	assertInvalid("oversized request", oversized, maxSideTaskInputArtifactBytes)
	unknownPurpose := input
	unknownPurpose.Purpose = "browse_everything"
	assertInvalid("unknown purpose", unknownPurpose, maxSideTaskInputArtifactBytes)
	unsortedScopes := input
	unsortedScopes.PermissionScopes = []string{"read:z", "read:a"}
	assertInvalid("unsorted scopes", unsortedScopes, maxSideTaskInputArtifactBytes)

	validJSON, _, err := canonicalSideTaskArtifact(input, maxSideTaskInputArtifactBytes)
	if err != nil {
		t.Fatal(err)
	}
	nullJSON := bytes.Replace(validJSON, []byte(`"permission_scopes":[]`), []byte(`"permission_scopes":null`), 1)
	var decoded SideTaskInputArtifact
	if err = decodeCanonicalSideTaskArtifact(nullJSON, &decoded); !errors.Is(err, ErrInvalidSideTaskProduct) {
		t.Fatalf("null decode error = %v", err)
	}
	duplicateJSON := bytes.Replace(validJSON, []byte(`{"schema_version":2`), []byte(`{"schema_version":2,"schema_version":2`), 1)
	if err = decodeCanonicalSideTaskArtifact(duplicateJSON, &decoded); !errors.Is(err, ErrInvalidSideTaskProduct) {
		t.Fatalf("duplicate decode error = %v", err)
	}

	summary := SideTaskSummaryArtifact{
		SchemaVersion: 1, SideTaskID: "side-1", ParentTaskID: "work-1",
		Purpose: "research", Status: "decision_required", SourceGeneration: 1,
		SummaryVersion: 1, WhatHappened: "One bounded finding.",
		AuthorizedFindings: []string{"One bounded finding."},
		EvidenceReferences: []SideTaskEvidenceReference{{EvidenceID: "evidence-1", Digest: strings.Repeat("c", 64), Kind: "source"}},
		ArtifactReferences: []SideTaskArtifactReference{{Digest: strings.Repeat("d", 64), Kind: "input"}},
		Risk:               "medium", Uncertainties: []string{}, ScopeDelta: []string{},
		DecisionOptions:         sideTaskDecisionOptions("decision_required"),
		RecommendationAuthority: "proposal_only", CreatedAt: "2026-08-03T08:00:00Z",
	}
	if _, _, err = canonicalSideTaskArtifact(summary, maxSideTaskSummaryArtifactBytes); err != nil {
		t.Fatalf("valid summary error = %v", err)
	}
	nullFindings := summary
	nullFindings.AuthorizedFindings = nil
	assertInvalid("null summary findings", nullFindings, maxSideTaskSummaryArtifactBytes)
	badRisk := summary
	badRisk.Risk = "critical"
	assertInvalid("unknown risk", badRisk, maxSideTaskSummaryArtifactBytes)
	badReference := summary
	badReference.EvidenceReferences[0].Digest = "not-a-digest"
	assertInvalid("bad evidence reference", badReference, maxSideTaskSummaryArtifactBytes)

	packet := SideTaskContextPacket{
		SchemaVersion: 1, ContextPacketID: "packet-1", SideTaskID: "side-1",
		ParentTaskID: "work-1", ParentRunID: "run-1", ParentClaimGeneration: 1,
		SideTaskGeneration: 1, HandoffVersion: 1, HandoffDigest: strings.Repeat("e", 64),
		SummaryArtifactDigest: strings.Repeat("f", 64), AuthorizedFindings: []string{"One bounded finding."},
		Risk: "low", Uncertainties: []string{}, ScopeDelta: []string{}, CreatedAt: "2026-08-03T08:00:00Z",
	}
	if _, _, err = canonicalSideTaskArtifact(packet, maxSideTaskContextPacketBytes); err != nil {
		t.Fatalf("valid packet error = %v", err)
	}
	packet.ScopeDelta = nil
	assertInvalid("null packet scope delta", packet, maxSideTaskContextPacketBytes)
}

func TestSideTaskSummaryMustBindExactAuthoritativeReferences(t *testing.T) {
	record := projection.SideTaskHandoff{
		SideTaskID: "side-bind-1", ParentTaskID: "work-parent-1", Purpose: "verification",
		Mode: "decision_required", SourceGeneration: 2,
		SourceEvidenceID: "evidence-source-1", SourceEvidenceDigest: strings.Repeat("a", 64),
		VerifierEvidenceID: "evidence-verifier-1", VerifierEvidenceDigest: strings.Repeat("b", 64),
		InputArtifactDigest: strings.Repeat("c", 64),
	}
	summary := SideTaskSummaryArtifact{
		SideTaskID: record.SideTaskID, ParentTaskID: record.ParentTaskID,
		Purpose: record.Purpose, Status: "decision_required", SourceGeneration: 2,
		EvidenceReferences: []SideTaskEvidenceReference{
			{EvidenceID: record.SourceEvidenceID, Digest: record.SourceEvidenceDigest, Kind: "source"},
			{EvidenceID: record.VerifierEvidenceID, Digest: record.VerifierEvidenceDigest, Kind: "verifier"},
		},
		ArtifactReferences: []SideTaskArtifactReference{{Digest: record.InputArtifactDigest, Kind: "input"}},
	}
	if !validBoundSideTaskSummary(record, summary) {
		t.Fatal("exact authoritative summary binding rejected")
	}
	drift := summary
	drift.Purpose = "research"
	if validBoundSideTaskSummary(record, drift) {
		t.Fatal("purpose drift accepted")
	}
	drift = summary
	drift.EvidenceReferences = append([]SideTaskEvidenceReference{}, summary.EvidenceReferences...)
	drift.EvidenceReferences[0].Digest = strings.Repeat("d", 64)
	if validBoundSideTaskSummary(record, drift) {
		t.Fatal("Evidence reference drift accepted")
	}
	drift = summary
	drift.ArtifactReferences = []SideTaskArtifactReference{{Digest: strings.Repeat("e", 64), Kind: "input"}}
	if validBoundSideTaskSummary(record, drift) {
		t.Fatal("input Artifact reference drift accepted")
	}
}

func TestSideTaskProposalDigestBindsParentExecutionDigest(t *testing.T) {
	request := SideTaskProposalRequest{
		SchemaVersion: 1, Operation: "propose", ParentMissionID: "mission/team-parent",
		ParentTeamInstanceID: "team-parent", ParentTaskID: "work-parent",
		ParentRunID: "run-parent", ParentClaimGeneration: 3,
		ParentExecutionDigest: strings.Repeat("a", 64), Purpose: "verification",
		Mode: "report_only", Title: "Verify bounded output",
		AuthorizedRequest: "Return one bounded authorized verification result.",
		PermissionScopes:  []string{}, ExpectedViewVersion: strings.Repeat("b", 64),
		CorrelationID: "11111111-1111-4111-8111-111111111111",
	}
	first, err := sideTaskProposalDigest(request)
	if err != nil {
		t.Fatal(err)
	}
	request.ParentExecutionDigest = strings.Repeat("c", 64)
	second, err := sideTaskProposalDigest(request)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("parent execution binding change did not change proposal digest")
	}
}
