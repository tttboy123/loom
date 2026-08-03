package work

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"loom-pi-rebuild/internal/journal"
)

var (
	ErrInvalidSideTaskHandoff  = errors.New("invalid side-task handoff")
	ErrSideTaskConflict        = errors.New("side-task handoff conflict")
	ErrSideTaskConfirmation    = errors.New("side-task confirmation required")
	ErrSideTaskCapabilityGap   = errors.New("side-task policy capability gap")
	ErrSideTaskNotFound        = errors.New("side-task not found")
	ErrSideTaskNotReady        = errors.New("side-task handoff not ready")
	ErrSideTaskDecisionExpired = errors.New("side-task decision expired")
)

const (
	SideTaskSchemaVersion = 1
	maxSideTaskScopes     = 32
	maxDecisionTimeout    = 30 * 24 * time.Hour
)

type SideTaskAdmissionInput struct {
	SideTaskID                  string
	ParentMissionID             string
	ParentTeamInstanceID        string
	ParentTaskID                string
	ParentRunID                 string
	ParentClaimGeneration       int64
	ParentExecutionDigest       string
	SideExecutionTeamInstanceID string
	Purpose                     string
	Mode                        string
	Title                       string
	ProposalDigest              string
	InputArtifactDigest         string
	ExpectedViewVersion         string
	PermissionScopes            []string
	Confirmed                   bool
	PolicyStreamID              string
	PolicyVersion               int
	PolicyDigest                string
	BudgetMicrounits            int64
	BudgetCurrency              string
	CorrelationID               string
}

type SideTaskHandoffCommitInput struct {
	SideTaskID                  string
	SideExecutionTeamInstanceID string
	SourceWorkItemID            string
	SourceRunID                 string
	SourceGeneration            int64
	SourceEvidenceID            string
	SourceEvidenceDigest        string
	VerifierEvidenceID          string
	VerifierEvidenceDigest      string
	HandoffVersion              int
	HandoffDigest               string
	SummaryArtifactDigest       string
	UsageObserved               bool
	UsageMicrounits             int64
	UsageCurrency               string
	DecisionTimeout             time.Duration
	ExpectedViewVersion         string
	CorrelationID               string
}

type SideTaskDecisionInput struct {
	DecisionID                          string
	SideTaskID                          string
	ParentMissionID                     string
	ParentTeamInstanceID                string
	ParentTaskID                        string
	ParentRunID                         string
	ParentLogicalNodeID                 string
	ParentAttemptNumber                 int
	ParentClaimGeneration               int64
	ParentExecutionDigest               string
	SideTaskGeneration                  int64
	HandoffVersion                      int
	HandoffDigest                       string
	Decision                            string
	EffectDigest                        string
	ContextPacketID                     string
	ContextPacketVersion                int
	ContextPacketDigest                 string
	ContinuationExecutionTeamInstanceID string
	ContinuationPlanDigest              string
	FollowupProposalDigest              string
	ScopeDeltaDigest                    string
	CancellationEffectDigest            string
	ExpectedViewVersion                 string
	CorrelationID                       string
}

type SideTaskDecisionExpiryInput struct {
	SideTaskID          string
	HandoffVersion      int
	HandoffDigest       string
	Outcome             string
	ExpectedViewVersion string
	CorrelationID       string
}

type ParentHandoffEffectCompletionInput struct {
	DecisionID              string
	SideTaskID              string
	EffectKind              string
	EffectDigest            string
	ExecutionTeamInstanceID string
	ExecutionPlanDigest     string
	TerminalStatus          string
	TerminalEvidenceID      string
	TerminalEvidenceDigest  string
	ExpectedViewVersion     string
	CorrelationID           string
}

type SideTaskHandoffRecord struct {
	SideTaskID                          string
	ParentMissionID                     string
	ParentTeamInstanceID                string
	ParentTaskID                        string
	ParentRunID                         string
	ParentClaimGeneration               int64
	ParentExecutionDigest               string
	SideExecutionTeamInstanceID         string
	Purpose                             string
	Mode                                string
	Title                               string
	ProposalDigest                      string
	InputArtifactDigest                 string
	PermissionScopes                    []string
	Status                              string
	SourceWorkItemID                    string
	SourceRunID                         string
	SourceGeneration                    int64
	SourceEvidenceID                    string
	SourceEvidenceDigest                string
	VerifierEvidenceID                  string
	VerifierEvidenceDigest              string
	HandoffVersion                      int
	HandoffDigest                       string
	SummaryArtifactDigest               string
	UsageObserved                       bool
	UsageMicrounits                     int64
	UsageCurrency                       string
	DecisionTimeoutSeconds              int64
	DecisionDeadline                    time.Time
	DecisionID                          string
	Decision                            string
	ContextPacketDigest                 string
	ContinuationExecutionTeamInstanceID string
	EffectDigest                        string
	EffectStatus                        string
	LastEventID                         string
	StreamSequence                      int64
}

type sideTaskAdmittedPayload struct {
	SideTaskID                  string   `json:"side_task_id"`
	ParentMissionID             string   `json:"parent_mission_id"`
	ParentTeamInstanceID        string   `json:"parent_team_instance_id"`
	ParentTaskID                string   `json:"parent_task_id"`
	ParentRunID                 string   `json:"parent_run_id"`
	ParentClaimGeneration       int64    `json:"parent_claim_generation"`
	ParentExecutionDigest       string   `json:"parent_execution_digest"`
	SideExecutionTeamInstanceID string   `json:"side_execution_team_instance_id"`
	Purpose                     string   `json:"purpose"`
	Mode                        string   `json:"mode"`
	Title                       string   `json:"title"`
	AdmissionKind               string   `json:"admission_kind"`
	ProposalDigest              string   `json:"proposal_digest"`
	InputArtifactDigest         string   `json:"input_artifact_digest"`
	ExpectedViewVersion         string   `json:"expected_view_version"`
	PermissionScopes            []string `json:"permission_scopes"`
	PolicyStreamID              string   `json:"policy_stream_id"`
	PolicyVersion               int      `json:"policy_version"`
	PolicyDigest                string   `json:"policy_digest"`
	BudgetMicrounits            int64    `json:"budget_microunits"`
	BudgetCurrency              string   `json:"budget_currency"`
}

type sideTaskHandoffCommittedPayload struct {
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
}

type sideTaskDecisionCommittedPayload struct {
	DecisionID            string `json:"decision_id"`
	SideTaskID            string `json:"side_task_id"`
	ParentTaskID          string `json:"parent_task_id"`
	ParentRunID           string `json:"parent_run_id"`
	ParentClaimGeneration int64  `json:"parent_claim_generation"`
	SideTaskGeneration    int64  `json:"side_task_generation"`
	HandoffVersion        int    `json:"handoff_version"`
	HandoffDigest         string `json:"handoff_digest"`
	ExpectedViewVersion   string `json:"expected_view_version"`
	Decision              string `json:"decision"`
	ContextPacketDigest   string `json:"context_packet_digest"`
	EffectDigest          string `json:"effect_digest"`
	ResultingStatus       string `json:"resulting_status"`
}

type sideTaskDecisionTimedOutPayload struct {
	SideTaskID           string `json:"side_task_id"`
	HandoffVersion       int    `json:"handoff_version"`
	HandoffDigest        string `json:"handoff_digest"`
	SourceHandoffEventID string `json:"source_handoff_event_id"`
	DecisionDeadline     string `json:"decision_deadline"`
	TimeoutObservedAt    string `json:"timeout_observed_at"`
	Outcome              string `json:"outcome"`
}

type parentHandoffEffectCompletedPayload struct {
	DecisionID              string `json:"decision_id"`
	SideTaskID              string `json:"side_task_id"`
	EffectKind              string `json:"effect_kind"`
	EffectDigest            string `json:"effect_digest"`
	ExecutionTeamInstanceID string `json:"execution_team_instance_id"`
	ExecutionPlanDigest     string `json:"execution_plan_digest"`
	TerminalStatus          string `json:"terminal_status"`
	TerminalEvidenceID      string `json:"terminal_evidence_id"`
	TerminalEvidenceDigest  string `json:"terminal_evidence_digest"`
}

func (authority *Authority) AdmitSideTask(
	ctx context.Context,
	input SideTaskAdmissionInput,
) (SideTaskHandoffRecord, error) {
	if authority == nil || ctx == nil || !validSideTaskAdmission(input) {
		return SideTaskHandoffRecord{}, ErrInvalidSideTaskHandoff
	}
	if input.PolicyStreamID != "" || input.PolicyVersion != 0 ||
		input.PolicyDigest != "" {
		return SideTaskHandoffRecord{}, ErrSideTaskCapabilityGap
	}
	if !input.Confirmed {
		return SideTaskHandoffRecord{}, ErrSideTaskConfirmation
	}
	streamID := sideTaskStream(input.SideTaskID)
	snapshot, err := authority.store.ReadStreamSet(ctx, []string{streamID})
	if err != nil {
		return SideTaskHandoffRecord{}, err
	}
	sideEvents := filterSideTaskEvents(snapshot.Events(), streamID)
	record, err := replaySideTaskStream(input.SideTaskID, sideEvents)
	if err != nil {
		return SideTaskHandoffRecord{}, err
	}
	if record.SideTaskID != "" {
		if exactSideTaskAdmission(record, input) {
			return record, nil
		}
		return SideTaskHandoffRecord{}, ErrSideTaskConflict
	}
	now, err := authority.operationTime()
	if err != nil {
		return SideTaskHandoffRecord{}, err
	}
	payload := sideTaskAdmittedPayload{
		SideTaskID:                  input.SideTaskID,
		ParentMissionID:             input.ParentMissionID,
		ParentTeamInstanceID:        input.ParentTeamInstanceID,
		ParentTaskID:                input.ParentTaskID,
		ParentRunID:                 input.ParentRunID,
		ParentClaimGeneration:       input.ParentClaimGeneration,
		ParentExecutionDigest:       input.ParentExecutionDigest,
		SideExecutionTeamInstanceID: input.SideExecutionTeamInstanceID,
		Purpose:                     input.Purpose, Mode: input.Mode, Title: input.Title,
		AdmissionKind:       "explicit_confirmation",
		ProposalDigest:      input.ProposalDigest,
		InputArtifactDigest: input.InputArtifactDigest,
		ExpectedViewVersion: input.ExpectedViewVersion,
		PermissionScopes:    append([]string{}, input.PermissionScopes...),
		PolicyStreamID:      "", PolicyVersion: 0, PolicyDigest: "",
		BudgetMicrounits: input.BudgetMicrounits,
		BudgetCurrency:   input.BudgetCurrency,
	}
	eventID := deterministicEventID("side-task-admit", input.SideTaskID, input.ProposalDigest)
	event := newEvent(eventID, streamID, 1, "SideTaskAdmitted", now,
		input.CorrelationID, "", payload)
	if _, err := authority.store.AppendBatchIfStreamHeads(ctx,
		[]journal.StreamHeadExpectation{{StreamID: streamID, Sequence: 0}},
		[]journal.Event{event}); err != nil {
		return SideTaskHandoffRecord{}, mapSideTaskWriteError(err)
	}
	return replaySideTaskStream(input.SideTaskID, []journal.Event{event})
}

func (authority *Authority) CommitSideTaskHandoff(
	ctx context.Context,
	input SideTaskHandoffCommitInput,
) (SideTaskHandoffRecord, error) {
	if authority == nil || ctx == nil || !validSideTaskHandoffCommit(input) {
		return SideTaskHandoffRecord{}, ErrInvalidSideTaskHandoff
	}
	streamID := sideTaskStream(input.SideTaskID)
	lineageSnapshot, err := authority.store.ReadStreamSet(ctx, []string{
		streamID,
		teamExecutionStream(input.SideExecutionTeamInstanceID),
	})
	if err != nil {
		return SideTaskHandoffRecord{}, err
	}
	lineageTeam, err := replayTeamExecution(input.SideExecutionTeamInstanceID,
		filterTeamEvents(lineageSnapshot.Events(),
			teamExecutionStream(input.SideExecutionTeamInstanceID)))
	if err != nil {
		return SideTaskHandoffRecord{}, ErrSideTaskConflict
	}
	lineageAttempt := findSideTaskAttempt(lineageTeam, input.SourceWorkItemID,
		input.SourceRunID, input.SourceGeneration)
	if lineageAttempt == nil {
		return SideTaskHandoffRecord{}, ErrSideTaskConflict
	}
	streamIDs := []string{
		streamID,
		teamExecutionStream(input.SideExecutionTeamInstanceID),
		workItemStream(input.SourceWorkItemID),
		runStream(input.SourceRunID),
		runtimeStatusStream(lineageAttempt.runtimeInstanceID),
		runtimeCapacityStream(lineageAttempt.runtimeInstanceID),
		"evidence/" + input.SourceEvidenceID,
	}
	if input.VerifierEvidenceID != "" {
		streamIDs = append(streamIDs, "evidence/"+input.VerifierEvidenceID)
	}
	snapshot, err := authority.store.ReadStreamSet(ctx, streamIDs)
	if err != nil {
		return SideTaskHandoffRecord{}, err
	}
	sideEvents := filterSideTaskEvents(snapshot.Events(), streamID)
	record, err := replaySideTaskStream(input.SideTaskID, sideEvents)
	if err != nil {
		return SideTaskHandoffRecord{}, err
	}
	if record.SideTaskID == "" {
		return SideTaskHandoffRecord{}, ErrSideTaskNotFound
	}
	if record.HandoffVersion != 0 {
		if exactSideTaskHandoff(record, input) {
			return record, nil
		}
		return SideTaskHandoffRecord{}, ErrSideTaskConflict
	}
	if record.SideExecutionTeamInstanceID != input.SideExecutionTeamInstanceID ||
		record.Mode == "merge_candidate" && input.VerifierEvidenceID == "" ||
		!validSideTaskTerminalLineage(ctx, snapshot, record, input) {
		return SideTaskHandoffRecord{}, ErrSideTaskConflict
	}
	now, err := authority.operationTime()
	if err != nil {
		return SideTaskHandoffRecord{}, err
	}
	status := "report_delivered"
	if record.Mode == "decision_required" {
		status = "decision_required"
	} else if record.Mode == "merge_candidate" {
		status = "merge_candidate_ready"
	}
	deadline := time.Time{}
	seconds := int64(0)
	if record.Mode != "report_only" {
		seconds = int64(input.DecisionTimeout / time.Second)
		deadline = now.Add(input.DecisionTimeout)
	}
	payload := sideTaskHandoffCommittedPayload{
		SideTaskID:                  input.SideTaskID,
		SideExecutionTeamInstanceID: input.SideExecutionTeamInstanceID,
		SourceWorkItemID:            input.SourceWorkItemID, SourceRunID: input.SourceRunID,
		SourceGeneration:       input.SourceGeneration,
		SourceEvidenceID:       input.SourceEvidenceID,
		SourceEvidenceDigest:   input.SourceEvidenceDigest,
		VerifierEvidenceID:     input.VerifierEvidenceID,
		VerifierEvidenceDigest: input.VerifierEvidenceDigest,
		HandoffVersion:         input.HandoffVersion, HandoffDigest: input.HandoffDigest,
		SummaryArtifactDigest: input.SummaryArtifactDigest,
		Mode:                  record.Mode, Status: status,
		UsageObserved: input.UsageObserved, UsageMicrounits: input.UsageMicrounits,
		UsageCurrency:          input.UsageCurrency,
		DecisionTimeoutSeconds: seconds, DecisionDeadline: formatOptionalUTC(deadline),
	}
	eventID := deterministicEventID("side-task-handoff", input.SideTaskID,
		fmt.Sprint(input.HandoffVersion), input.HandoffDigest)
	event := newEvent(eventID, streamID, record.StreamSequence+1,
		"SideTaskHandoffCommitted", now, input.CorrelationID, record.LastEventID, payload)
	if _, err := authority.store.AppendBatchIfStreamHeads(ctx,
		sideTaskSnapshotExpectations(snapshot),
		[]journal.Event{event}); err != nil {
		return SideTaskHandoffRecord{}, mapSideTaskWriteError(err)
	}
	return replaySideTaskStream(input.SideTaskID,
		append(sideEvents, event))
}

func (authority *Authority) DecideSideTaskHandoff(
	ctx context.Context,
	input SideTaskDecisionInput,
) (SideTaskHandoffRecord, error) {
	if authority == nil || ctx == nil || !validSideTaskDecision(input) {
		return SideTaskHandoffRecord{}, ErrInvalidSideTaskHandoff
	}
	sideStream := sideTaskStream(input.SideTaskID)
	parentStream := parentHandoffStream(input.ParentTaskID)
	snapshot, err := authority.store.ReadStreamSet(ctx, []string{
		sideStream,
		parentStream,
		teamExecutionStream(input.ParentTeamInstanceID),
		workItemStream(input.ParentTaskID),
	})
	if err != nil {
		return SideTaskHandoffRecord{}, err
	}
	record, err := replaySideTaskStream(input.SideTaskID,
		filterSideTaskEvents(snapshot.Events(), sideStream))
	if err != nil {
		return SideTaskHandoffRecord{}, err
	}
	if record.SideTaskID == "" || record.HandoffVersion == 0 {
		return SideTaskHandoffRecord{}, ErrSideTaskNotReady
	}
	if record.DecisionID != "" {
		if record.DecisionID == input.DecisionID && record.Decision == input.Decision &&
			record.EffectDigest == input.EffectDigest {
			return record, nil
		}
		return SideTaskHandoffRecord{}, ErrSideTaskConflict
	}
	if record.ParentMissionID != input.ParentMissionID ||
		record.ParentTeamInstanceID != input.ParentTeamInstanceID ||
		record.ParentTaskID != input.ParentTaskID ||
		record.ParentRunID != input.ParentRunID ||
		record.ParentClaimGeneration != input.ParentClaimGeneration ||
		record.ParentExecutionDigest != input.ParentExecutionDigest ||
		record.SourceGeneration != input.SideTaskGeneration ||
		record.HandoffVersion != input.HandoffVersion ||
		record.HandoffDigest != input.HandoffDigest {
		return SideTaskHandoffRecord{}, ErrSideTaskConflict
	}
	if !validParentDecisionLineage(ctx, snapshot, input) {
		return SideTaskHandoffRecord{}, ErrSideTaskConflict
	}
	now, err := authority.operationTime()
	if err != nil {
		return SideTaskHandoffRecord{}, err
	}
	if !record.DecisionDeadline.IsZero() && now.After(record.DecisionDeadline) {
		return SideTaskHandoffRecord{}, ErrSideTaskDecisionExpired
	}
	resultingStatus := "decided"
	if input.Decision == "archive" {
		resultingStatus = "archived"
	}
	sidePayload := sideTaskDecisionCommittedPayload{
		DecisionID: input.DecisionID, SideTaskID: input.SideTaskID,
		ParentTaskID: input.ParentTaskID, ParentRunID: input.ParentRunID,
		ParentClaimGeneration: input.ParentClaimGeneration,
		SideTaskGeneration:    input.SideTaskGeneration,
		HandoffVersion:        input.HandoffVersion, HandoffDigest: input.HandoffDigest,
		ExpectedViewVersion: input.ExpectedViewVersion, Decision: input.Decision,
		ContextPacketDigest: input.ContextPacketDigest, EffectDigest: input.EffectDigest,
		ResultingStatus: resultingStatus,
	}
	sideEventID := deterministicEventID("side-task-decision", input.SideTaskID,
		input.DecisionID, input.HandoffDigest)
	sideEvent := newEvent(sideEventID, sideStream, record.StreamSequence+1,
		"SideTaskDecisionCommitted", now, input.CorrelationID, record.LastEventID,
		sidePayload)
	events := []journal.Event{sideEvent}
	expectations := sideTaskSnapshotExpectations(snapshot)
	if parentEvents := buildParentHandoffEvents(input, record, now, sideEventID,
		snapshot); len(parentEvents) > 0 {
		events = append(events, parentEvents...)
	}
	if _, err := authority.store.AppendBatchIfStreamHeads(ctx, expectations, events); err != nil {
		return SideTaskHandoffRecord{}, mapSideTaskWriteError(err)
	}
	decided, err := replaySideTaskStream(input.SideTaskID,
		append(filterSideTaskEvents(snapshot.Events(), sideStream), sideEvent))
	if err == nil {
		decided.ContinuationExecutionTeamInstanceID =
			input.ContinuationExecutionTeamInstanceID
	}
	return decided, err
}

func (authority *Authority) ExpireSideTaskDecision(
	ctx context.Context,
	input SideTaskDecisionExpiryInput,
) (SideTaskHandoffRecord, error) {
	if authority == nil || ctx == nil || !validSideTaskExpiry(input) {
		return SideTaskHandoffRecord{}, ErrInvalidSideTaskHandoff
	}
	streamID := sideTaskStream(input.SideTaskID)
	snapshot, err := authority.store.ReadStreamSet(ctx, []string{streamID})
	if err != nil {
		return SideTaskHandoffRecord{}, err
	}
	record, err := replaySideTaskStream(input.SideTaskID, snapshot.Events())
	if err != nil {
		return SideTaskHandoffRecord{}, err
	}
	if record.SideTaskID == "" || record.HandoffVersion != input.HandoffVersion ||
		record.HandoffDigest != input.HandoffDigest || record.DecisionID != "" {
		return SideTaskHandoffRecord{}, ErrSideTaskConflict
	}
	now, err := authority.operationTime()
	if err != nil {
		return SideTaskHandoffRecord{}, err
	}
	if record.DecisionDeadline.IsZero() || now.Before(record.DecisionDeadline) {
		return SideTaskHandoffRecord{}, ErrSideTaskNotReady
	}
	payload := sideTaskDecisionTimedOutPayload{
		SideTaskID: input.SideTaskID, HandoffVersion: input.HandoffVersion,
		HandoffDigest: input.HandoffDigest, SourceHandoffEventID: record.LastEventID,
		DecisionDeadline:  record.DecisionDeadline.Format(time.RFC3339Nano),
		TimeoutObservedAt: now.Format(time.RFC3339Nano), Outcome: input.Outcome,
	}
	eventID := deterministicEventID("side-task-timeout", input.SideTaskID,
		input.HandoffDigest, input.Outcome)
	event := newEvent(eventID, streamID, record.StreamSequence+1,
		"SideTaskDecisionTimedOut", now, input.CorrelationID, record.LastEventID, payload)
	if _, err := authority.store.AppendBatchIfStreamHeads(ctx,
		[]journal.StreamHeadExpectation{{StreamID: streamID, Sequence: record.StreamSequence}},
		[]journal.Event{event}); err != nil {
		return SideTaskHandoffRecord{}, mapSideTaskWriteError(err)
	}
	return replaySideTaskStream(input.SideTaskID, append(snapshot.Events(), event))
}

func (authority *Authority) CompleteParentHandoffEffect(
	ctx context.Context,
	input ParentHandoffEffectCompletionInput,
) (SideTaskHandoffRecord, error) {
	if authority == nil || ctx == nil || !validParentEffectCompletion(input) {
		return SideTaskHandoffRecord{}, ErrInvalidSideTaskHandoff
	}
	sideStream := sideTaskStream(input.SideTaskID)
	sideSnapshot, err := authority.store.ReadStreamSet(ctx, []string{sideStream})
	if err != nil {
		return SideTaskHandoffRecord{}, err
	}
	record, err := replaySideTaskStream(input.SideTaskID, sideSnapshot.Events())
	if err != nil {
		return SideTaskHandoffRecord{}, err
	}
	if record.DecisionID != input.DecisionID || record.EffectDigest != input.EffectDigest ||
		record.EffectStatus == "none" {
		return SideTaskHandoffRecord{}, ErrSideTaskConflict
	}
	parentStream := parentHandoffStream(record.ParentTaskID)
	parentSnapshot, err := authority.store.ReadStreamSet(ctx, []string{
		sideStream,
		parentStream,
		teamExecutionStream(input.ExecutionTeamInstanceID),
		"evidence/" + input.TerminalEvidenceID,
	})
	if err != nil {
		return SideTaskHandoffRecord{}, err
	}
	record, err = replaySideTaskStream(input.SideTaskID,
		filterSideTaskEvents(parentSnapshot.Events(), sideStream))
	if err != nil || record.DecisionID != input.DecisionID ||
		record.EffectDigest != input.EffectDigest || record.EffectStatus == "none" {
		return SideTaskHandoffRecord{}, ErrSideTaskConflict
	}
	authorization, authorized := authorizedParentEffect(
		parentSnapshot.Events(), record, input,
	)
	if !authorized || !validParentEffectTerminal(parentSnapshot, input, authorization) {
		return SideTaskHandoffRecord{}, ErrSideTaskConflict
	}
	for _, event := range parentSnapshot.Events() {
		if event.Type != "ParentHandoffEffectCompleted" {
			continue
		}
		var payload parentHandoffEffectCompletedPayload
		if decodeExactPayload(event.PayloadJSON, &payload) != nil {
			return SideTaskHandoffRecord{}, ErrSideTaskConflict
		}
		if payload.DecisionID == input.DecisionID {
			if exactParentEffectCompletion(payload, input) {
				record.EffectStatus = "completed"
				return record, nil
			}
			return SideTaskHandoffRecord{}, ErrSideTaskConflict
		}
	}
	now, err := authority.operationTime()
	if err != nil {
		return SideTaskHandoffRecord{}, err
	}
	payload := parentHandoffEffectCompletedPayload{
		DecisionID: input.DecisionID, SideTaskID: input.SideTaskID,
		EffectKind: input.EffectKind, EffectDigest: input.EffectDigest,
		ExecutionTeamInstanceID: input.ExecutionTeamInstanceID,
		ExecutionPlanDigest:     input.ExecutionPlanDigest,
		TerminalStatus:          input.TerminalStatus,
		TerminalEvidenceID:      input.TerminalEvidenceID,
		TerminalEvidenceDigest:  input.TerminalEvidenceDigest,
	}
	head, _ := parentSnapshot.Head(parentStream)
	eventID := deterministicEventID("parent-handoff-effect-complete",
		input.DecisionID, input.EffectDigest)
	event := newEvent(eventID, parentStream, head.Sequence+1,
		"ParentHandoffEffectCompleted", now, input.CorrelationID, head.EventID, payload)
	if _, err := authority.store.AppendBatchIfStreamHeads(ctx,
		sideTaskSnapshotExpectations(parentSnapshot),
		[]journal.Event{event}); err != nil {
		return SideTaskHandoffRecord{}, mapSideTaskWriteError(err)
	}
	record.EffectStatus = "completed"
	return record, nil
}

type parentEffectAuthorization struct {
	logicalNodeID string
	attemptNumber int
}

func authorizedParentEffect(
	events []journal.Event,
	record SideTaskHandoffRecord,
	input ParentHandoffEffectCompletionInput,
) (parentEffectAuthorization, bool) {
	for _, event := range events {
		if event.StreamID != parentHandoffStream(record.ParentTaskID) {
			continue
		}
		switch event.Type {
		case "ParentContinuationAuthorized":
			if input.EffectKind != "continuation" ||
				(record.Decision != "absorb" && record.Decision != "continue") {
				continue
			}
			var payload struct {
				DecisionID                          string `json:"decision_id"`
				SideTaskID                          string `json:"side_task_id"`
				HandoffVersion                      int    `json:"handoff_version"`
				HandoffDigest                       string `json:"handoff_digest"`
				ParentMissionID                     string `json:"parent_mission_id"`
				ParentTeamInstanceID                string `json:"parent_team_instance_id"`
				ParentTaskID                        string `json:"parent_task_id"`
				ParentRunID                         string `json:"parent_run_id"`
				ParentClaimGeneration               int64  `json:"parent_claim_generation"`
				SideTaskGeneration                  int64  `json:"side_task_generation"`
				Action                              string `json:"action"`
				ContextPacketDigest                 string `json:"context_packet_digest"`
				ContinuationExecutionTeamInstanceID string `json:"continuation_execution_team_instance_id"`
				ContinuationPlanDigest              string `json:"continuation_plan_digest"`
				EffectDigest                        string `json:"effect_digest"`
			}
			if decodeExactPayload(event.PayloadJSON, &payload) == nil &&
				payload.DecisionID == input.DecisionID &&
				payload.SideTaskID == input.SideTaskID &&
				payload.HandoffVersion == record.HandoffVersion &&
				payload.HandoffDigest == record.HandoffDigest &&
				payload.ParentMissionID == record.ParentMissionID &&
				payload.ParentTeamInstanceID == record.ParentTeamInstanceID &&
				payload.ParentTaskID == record.ParentTaskID &&
				payload.ParentRunID == record.ParentRunID &&
				payload.ParentClaimGeneration == record.ParentClaimGeneration &&
				payload.SideTaskGeneration == record.SourceGeneration &&
				payload.Action == record.Decision &&
				payload.ContextPacketDigest == record.ContextPacketDigest &&
				payload.ContinuationExecutionTeamInstanceID == input.ExecutionTeamInstanceID &&
				payload.ContinuationPlanDigest == input.ExecutionPlanDigest &&
				payload.EffectDigest == input.EffectDigest {
				return parentEffectAuthorization{}, true
			}
		case "ParentCancellationRequested":
			if input.EffectKind != "cancellation" || record.Decision != "cancel_parent" {
				continue
			}
			var payload struct {
				DecisionID               string `json:"decision_id"`
				SideTaskID               string `json:"side_task_id"`
				HandoffVersion           int    `json:"handoff_version"`
				HandoffDigest            string `json:"handoff_digest"`
				ParentMissionID          string `json:"parent_mission_id"`
				ParentTeamInstanceID     string `json:"parent_team_instance_id"`
				ParentTaskID             string `json:"parent_task_id"`
				ParentRunID              string `json:"parent_run_id"`
				ParentLogicalNodeID      string `json:"parent_logical_node_id"`
				ParentAttemptNumber      int    `json:"parent_attempt_number"`
				ParentClaimGeneration    int64  `json:"parent_claim_generation"`
				ParentExecutionDigest    string `json:"parent_execution_digest"`
				CancellationEffectDigest string `json:"cancellation_effect_digest"`
				EffectDigest             string `json:"effect_digest"`
			}
			if decodeExactPayload(event.PayloadJSON, &payload) == nil &&
				payload.DecisionID == input.DecisionID && payload.SideTaskID == input.SideTaskID &&
				payload.HandoffVersion == record.HandoffVersion &&
				payload.HandoffDigest == record.HandoffDigest &&
				payload.ParentMissionID == record.ParentMissionID &&
				payload.ParentTeamInstanceID == input.ExecutionTeamInstanceID &&
				payload.ParentTaskID == record.ParentTaskID &&
				payload.ParentRunID == record.ParentRunID &&
				payload.ParentLogicalNodeID != "" && payload.ParentAttemptNumber > 0 &&
				payload.ParentClaimGeneration == record.ParentClaimGeneration &&
				payload.ParentExecutionDigest == record.ParentExecutionDigest &&
				validSHA256(payload.CancellationEffectDigest) &&
				payload.EffectDigest == input.EffectDigest {
				return parentEffectAuthorization{
					logicalNodeID: payload.ParentLogicalNodeID,
					attemptNumber: payload.ParentAttemptNumber,
				}, true
			}
		}
	}
	return parentEffectAuthorization{}, false
}

func (authority *Authority) SideTaskHandoff(
	ctx context.Context,
	sideTaskID string,
) (SideTaskHandoffRecord, error) {
	if authority == nil || ctx == nil || !validOpaqueID(sideTaskID) {
		return SideTaskHandoffRecord{}, ErrInvalidSideTaskHandoff
	}
	streamID := sideTaskStream(sideTaskID)
	snapshot, err := authority.store.ReadStreamSet(ctx, []string{streamID})
	if err != nil {
		return SideTaskHandoffRecord{}, err
	}
	record, err := replaySideTaskStream(sideTaskID, snapshot.Events())
	if err != nil {
		return SideTaskHandoffRecord{}, err
	}
	if record.SideTaskID == "" {
		return SideTaskHandoffRecord{}, ErrSideTaskNotFound
	}
	return cloneSideTaskHandoff(record), nil
}

func replaySideTaskStream(sideTaskID string, events []journal.Event) (SideTaskHandoffRecord, error) {
	var record SideTaskHandoffRecord
	for index, event := range events {
		if event.StreamID != sideTaskStream(sideTaskID) || event.Seq != int64(index+1) ||
			event.SchemaVersion != SideTaskSchemaVersion {
			return SideTaskHandoffRecord{}, ErrSideTaskConflict
		}
		switch event.Type {
		case "SideTaskAdmitted":
			if record.SideTaskID != "" {
				return SideTaskHandoffRecord{}, ErrSideTaskConflict
			}
			var payload sideTaskAdmittedPayload
			if decodeExactPayload(event.PayloadJSON, &payload) != nil ||
				payload.SideTaskID != sideTaskID ||
				payload.AdmissionKind != "explicit_confirmation" {
				return SideTaskHandoffRecord{}, ErrSideTaskConflict
			}
			record = SideTaskHandoffRecord{
				SideTaskID:           payload.SideTaskID,
				ParentMissionID:      payload.ParentMissionID,
				ParentTeamInstanceID: payload.ParentTeamInstanceID,
				ParentTaskID:         payload.ParentTaskID, ParentRunID: payload.ParentRunID,
				ParentClaimGeneration:       payload.ParentClaimGeneration,
				ParentExecutionDigest:       payload.ParentExecutionDigest,
				SideExecutionTeamInstanceID: payload.SideExecutionTeamInstanceID,
				Purpose:                     payload.Purpose, Mode: payload.Mode, Title: payload.Title,
				ProposalDigest:      payload.ProposalDigest,
				InputArtifactDigest: payload.InputArtifactDigest,
				PermissionScopes:    append([]string{}, payload.PermissionScopes...),
				Status:              "admitted", EffectStatus: "none",
			}
		case "SideTaskHandoffCommitted":
			if record.SideTaskID == "" || record.HandoffVersion != 0 {
				return SideTaskHandoffRecord{}, ErrSideTaskConflict
			}
			var payload sideTaskHandoffCommittedPayload
			if decodeExactPayload(event.PayloadJSON, &payload) != nil ||
				payload.SideTaskID != sideTaskID || payload.Mode != record.Mode ||
				payload.SideExecutionTeamInstanceID != record.SideExecutionTeamInstanceID {
				return SideTaskHandoffRecord{}, ErrSideTaskConflict
			}
			deadline, err := parseOptionalUTC(payload.DecisionDeadline)
			if err != nil {
				return SideTaskHandoffRecord{}, ErrSideTaskConflict
			}
			record.SourceWorkItemID = payload.SourceWorkItemID
			record.SourceRunID = payload.SourceRunID
			record.SourceGeneration = payload.SourceGeneration
			record.SourceEvidenceID = payload.SourceEvidenceID
			record.SourceEvidenceDigest = payload.SourceEvidenceDigest
			record.VerifierEvidenceID = payload.VerifierEvidenceID
			record.VerifierEvidenceDigest = payload.VerifierEvidenceDigest
			record.HandoffVersion = payload.HandoffVersion
			record.HandoffDigest = payload.HandoffDigest
			record.SummaryArtifactDigest = payload.SummaryArtifactDigest
			record.Status = payload.Status
			record.UsageObserved = payload.UsageObserved
			record.UsageMicrounits = payload.UsageMicrounits
			record.UsageCurrency = payload.UsageCurrency
			record.DecisionTimeoutSeconds = payload.DecisionTimeoutSeconds
			record.DecisionDeadline = deadline
		case "SideTaskDecisionCommitted":
			if record.HandoffVersion == 0 || record.DecisionID != "" {
				return SideTaskHandoffRecord{}, ErrSideTaskConflict
			}
			var payload sideTaskDecisionCommittedPayload
			if decodeExactPayload(event.PayloadJSON, &payload) != nil ||
				payload.SideTaskID != sideTaskID ||
				payload.HandoffVersion != record.HandoffVersion ||
				payload.HandoffDigest != record.HandoffDigest {
				return SideTaskHandoffRecord{}, ErrSideTaskConflict
			}
			record.DecisionID = payload.DecisionID
			record.Decision = payload.Decision
			record.ContextPacketDigest = payload.ContextPacketDigest
			record.EffectDigest = payload.EffectDigest
			record.Status = payload.ResultingStatus
			if sideTaskDecisionHasParentEffect(payload.Decision) {
				record.EffectStatus = "pending"
			}
		case "SideTaskDecisionTimedOut":
			if record.HandoffVersion == 0 || record.DecisionID != "" {
				return SideTaskHandoffRecord{}, ErrSideTaskConflict
			}
			var payload sideTaskDecisionTimedOutPayload
			if decodeExactPayload(event.PayloadJSON, &payload) != nil ||
				payload.SideTaskID != sideTaskID ||
				payload.HandoffDigest != record.HandoffDigest ||
				(payload.Outcome != "paused" && payload.Outcome != "human_required") {
				return SideTaskHandoffRecord{}, ErrSideTaskConflict
			}
			record.Status = payload.Outcome
		default:
			return SideTaskHandoffRecord{}, ErrSideTaskConflict
		}
		record.LastEventID = event.ID
		record.StreamSequence = event.Seq
	}
	return cloneSideTaskHandoff(record), nil
}

func buildParentHandoffEvents(input SideTaskDecisionInput,
	record SideTaskHandoffRecord, now time.Time,
	causationID string, snapshot journal.StreamSetSnapshot) []journal.Event {
	streamID := parentHandoffStream(input.ParentTaskID)
	head, _ := snapshot.Head(streamID)
	var eventType string
	var payload any
	switch input.Decision {
	case "absorb":
		packetPayload := struct {
			ContextPacketID                     string `json:"context_packet_id"`
			ContextPacketVersion                int    `json:"context_packet_version"`
			ContextPacketDigest                 string `json:"context_packet_digest"`
			SideTaskID                          string `json:"side_task_id"`
			HandoffVersion                      int    `json:"handoff_version"`
			HandoffDigest                       string `json:"handoff_digest"`
			SummaryArtifactDigest               string `json:"summary_artifact_digest"`
			ParentTaskID                        string `json:"parent_task_id"`
			ParentRunID                         string `json:"parent_run_id"`
			ParentClaimGeneration               int64  `json:"parent_claim_generation"`
			SideTaskGeneration                  int64  `json:"side_task_generation"`
			ContinuationExecutionTeamInstanceID string `json:"continuation_execution_team_instance_id"`
		}{input.ContextPacketID, input.ContextPacketVersion, input.ContextPacketDigest,
			input.SideTaskID, input.HandoffVersion, input.HandoffDigest,
			record.SummaryArtifactDigest, input.ParentTaskID, input.ParentRunID, input.ParentClaimGeneration,
			input.SideTaskGeneration, input.ContinuationExecutionTeamInstanceID}
		packetID := deterministicEventID("parent-handoff", input.DecisionID,
			"ContextPacketCommitted", input.ContextPacketDigest)
		packetEvent := newEvent(packetID, streamID, head.Sequence+1,
			"ContextPacketCommitted", now, input.CorrelationID, causationID,
			packetPayload)
		continuationID := deterministicEventID("parent-handoff", input.DecisionID,
			"ParentContinuationAuthorized", input.EffectDigest)
		continuationEvent := newEvent(continuationID, streamID, head.Sequence+2,
			"ParentContinuationAuthorized", now, input.CorrelationID, packetID,
			parentContinuationPayload(input, "absorb"))
		return []journal.Event{packetEvent, continuationEvent}
	case "continue":
		eventType = "ParentContinuationAuthorized"
		payload = parentContinuationPayload(input, "continue")
	case "request_followup":
		eventType = "ParentFollowupProposed"
		payload = struct {
			DecisionID             string `json:"decision_id"`
			SideTaskID             string `json:"side_task_id"`
			HandoffVersion         int    `json:"handoff_version"`
			HandoffDigest          string `json:"handoff_digest"`
			ParentTaskID           string `json:"parent_task_id"`
			ParentRunID            string `json:"parent_run_id"`
			ParentClaimGeneration  int64  `json:"parent_claim_generation"`
			FollowupProposalDigest string `json:"followup_proposal_digest"`
			EffectDigest           string `json:"effect_digest"`
		}{input.DecisionID, input.SideTaskID, input.HandoffVersion,
			input.HandoffDigest, input.ParentTaskID, input.ParentRunID,
			input.ParentClaimGeneration, input.FollowupProposalDigest, input.EffectDigest}
	case "pivot":
		eventType = "ParentScopePivotProposed"
		payload = struct {
			DecisionID            string `json:"decision_id"`
			SideTaskID            string `json:"side_task_id"`
			HandoffVersion        int    `json:"handoff_version"`
			HandoffDigest         string `json:"handoff_digest"`
			ParentTaskID          string `json:"parent_task_id"`
			ParentRunID           string `json:"parent_run_id"`
			ParentClaimGeneration int64  `json:"parent_claim_generation"`
			ScopeDeltaDigest      string `json:"scope_delta_digest"`
			EffectDigest          string `json:"effect_digest"`
		}{input.DecisionID, input.SideTaskID, input.HandoffVersion,
			input.HandoffDigest, input.ParentTaskID, input.ParentRunID,
			input.ParentClaimGeneration, input.ScopeDeltaDigest, input.EffectDigest}
	case "cancel_parent":
		eventType = "ParentCancellationRequested"
		payload = struct {
			DecisionID               string `json:"decision_id"`
			SideTaskID               string `json:"side_task_id"`
			HandoffVersion           int    `json:"handoff_version"`
			HandoffDigest            string `json:"handoff_digest"`
			ParentMissionID          string `json:"parent_mission_id"`
			ParentTeamInstanceID     string `json:"parent_team_instance_id"`
			ParentTaskID             string `json:"parent_task_id"`
			ParentRunID              string `json:"parent_run_id"`
			ParentLogicalNodeID      string `json:"parent_logical_node_id"`
			ParentAttemptNumber      int    `json:"parent_attempt_number"`
			ParentClaimGeneration    int64  `json:"parent_claim_generation"`
			ParentExecutionDigest    string `json:"parent_execution_digest"`
			CancellationEffectDigest string `json:"cancellation_effect_digest"`
			EffectDigest             string `json:"effect_digest"`
		}{input.DecisionID, input.SideTaskID, input.HandoffVersion,
			input.HandoffDigest, input.ParentMissionID, input.ParentTeamInstanceID,
			input.ParentTaskID, input.ParentRunID, input.ParentLogicalNodeID,
			input.ParentAttemptNumber, input.ParentClaimGeneration,
			input.ParentExecutionDigest, input.CancellationEffectDigest, input.EffectDigest}
	default:
		return nil
	}
	eventID := deterministicEventID("parent-handoff", input.DecisionID,
		eventType, input.EffectDigest)
	return []journal.Event{newEvent(eventID, streamID, head.Sequence+1,
		eventType, now, input.CorrelationID, causationID, payload)}
}

func parentContinuationPayload(input SideTaskDecisionInput, action string) any {
	return struct {
		DecisionID                          string `json:"decision_id"`
		SideTaskID                          string `json:"side_task_id"`
		HandoffVersion                      int    `json:"handoff_version"`
		HandoffDigest                       string `json:"handoff_digest"`
		ParentMissionID                     string `json:"parent_mission_id"`
		ParentTeamInstanceID                string `json:"parent_team_instance_id"`
		ParentTaskID                        string `json:"parent_task_id"`
		ParentRunID                         string `json:"parent_run_id"`
		ParentClaimGeneration               int64  `json:"parent_claim_generation"`
		SideTaskGeneration                  int64  `json:"side_task_generation"`
		Action                              string `json:"action"`
		ContextPacketDigest                 string `json:"context_packet_digest"`
		ContinuationExecutionTeamInstanceID string `json:"continuation_execution_team_instance_id"`
		ContinuationPlanDigest              string `json:"continuation_plan_digest"`
		EffectDigest                        string `json:"effect_digest"`
	}{input.DecisionID, input.SideTaskID, input.HandoffVersion,
		input.HandoffDigest, input.ParentMissionID, input.ParentTeamInstanceID,
		input.ParentTaskID, input.ParentRunID, input.ParentClaimGeneration,
		input.SideTaskGeneration, action, input.ContextPacketDigest,
		input.ContinuationExecutionTeamInstanceID, input.ContinuationPlanDigest,
		input.EffectDigest}
}

func validSideTaskAdmission(input SideTaskAdmissionInput) bool {
	return validOpaqueID(input.SideTaskID) && validOpaqueID(input.ParentMissionID) &&
		validOpaqueID(input.ParentTeamInstanceID) && validOpaqueID(input.ParentTaskID) &&
		validOpaqueID(input.ParentRunID) && input.ParentClaimGeneration >= 0 &&
		validSHA256(input.ParentExecutionDigest) &&
		validOpaqueID(input.SideExecutionTeamInstanceID) &&
		validSideTaskPurpose(input.Purpose) && validSideTaskMode(input.Mode) &&
		validBoundedSideTaskText(input.Title, 128) && validSHA256(input.ProposalDigest) &&
		validSHA256(input.InputArtifactDigest) && validSHA256(input.ExpectedViewVersion) &&
		validSortedScopes(input.PermissionScopes) && input.BudgetMicrounits >= 0 &&
		(input.BudgetMicrounits == 0 && input.BudgetCurrency == "" ||
			input.BudgetMicrounits > 0 && validCurrency(input.BudgetCurrency)) &&
		validOpaqueID(input.CorrelationID)
}

func validSideTaskHandoffCommit(input SideTaskHandoffCommitInput) bool {
	if !validOpaqueID(input.SideTaskID) ||
		!validOpaqueID(input.SideExecutionTeamInstanceID) ||
		!validOpaqueID(input.SourceWorkItemID) || !validOpaqueID(input.SourceRunID) ||
		input.SourceGeneration < 0 || !validOpaqueID(input.SourceEvidenceID) ||
		!validSHA256(input.SourceEvidenceDigest) || input.HandoffVersion <= 0 ||
		!validSHA256(input.HandoffDigest) || !validSHA256(input.SummaryArtifactDigest) ||
		!validSHA256(input.ExpectedViewVersion) || !validOpaqueID(input.CorrelationID) {
		return false
	}
	if (input.VerifierEvidenceID == "") != (input.VerifierEvidenceDigest == "") ||
		input.VerifierEvidenceID != "" && (!validOpaqueID(input.VerifierEvidenceID) ||
			!validSHA256(input.VerifierEvidenceDigest)) {
		return false
	}
	if input.UsageObserved {
		if input.UsageMicrounits < 0 || !validCurrency(input.UsageCurrency) {
			return false
		}
	} else if input.UsageMicrounits != 0 || input.UsageCurrency != "" {
		return false
	}
	return input.DecisionTimeout == 0 ||
		input.DecisionTimeout >= time.Minute && input.DecisionTimeout <= maxDecisionTimeout
}

func validSideTaskDecision(input SideTaskDecisionInput) bool {
	if !validOpaqueID(input.DecisionID) || !validOpaqueID(input.SideTaskID) ||
		!validOpaqueID(input.ParentMissionID) || !validOpaqueID(input.ParentTeamInstanceID) ||
		!validOpaqueID(input.ParentTaskID) || !validOpaqueID(input.ParentRunID) ||
		input.ParentClaimGeneration < 0 || !validSHA256(input.ParentExecutionDigest) ||
		input.SideTaskGeneration < 0 ||
		input.HandoffVersion <= 0 || !validSHA256(input.HandoffDigest) ||
		!validSideTaskDecisionValue(input.Decision) ||
		input.EffectDigest != sideTaskParentEffectDigest(input) ||
		!validSHA256(input.ExpectedViewVersion) || !validOpaqueID(input.CorrelationID) {
		return false
	}
	switch input.Decision {
	case "absorb":
		return validOpaqueID(input.ContextPacketID) && input.ContextPacketVersion == 1 &&
			validSHA256(input.ContextPacketDigest) &&
			validOpaqueID(input.ContinuationExecutionTeamInstanceID) &&
			validSHA256(input.ContinuationPlanDigest)
	case "continue":
		return input.ContextPacketID == "" && input.ContextPacketDigest == "" &&
			validOpaqueID(input.ContinuationExecutionTeamInstanceID) &&
			validSHA256(input.ContinuationPlanDigest)
	case "request_followup":
		return validSHA256(input.FollowupProposalDigest)
	case "pivot":
		return validSHA256(input.ScopeDeltaDigest)
	case "cancel_parent":
		return validOpaqueID(input.ParentLogicalNodeID) && input.ParentAttemptNumber > 0 &&
			validSHA256(input.CancellationEffectDigest)
	default:
		return input.ContextPacketID == "" && input.ContextPacketDigest == "" &&
			input.ContinuationExecutionTeamInstanceID == ""
	}
}

func sideTaskParentEffectDigest(input SideTaskDecisionInput) string {
	digest := sha256.Sum256([]byte(strings.Join([]string{
		"parent-effect-v1", input.SideTaskID, input.ParentTeamInstanceID,
		input.ParentTaskID, input.ParentRunID,
		fmt.Sprint(input.ParentClaimGeneration), input.ParentExecutionDigest,
		input.HandoffDigest, input.Decision,
	}, "\x00")))
	return hex.EncodeToString(digest[:])
}

func validSideTaskExpiry(input SideTaskDecisionExpiryInput) bool {
	return validOpaqueID(input.SideTaskID) && input.HandoffVersion > 0 &&
		validSHA256(input.HandoffDigest) &&
		(input.Outcome == "paused" || input.Outcome == "human_required") &&
		validSHA256(input.ExpectedViewVersion) && validOpaqueID(input.CorrelationID)
}

func validParentEffectCompletion(input ParentHandoffEffectCompletionInput) bool {
	return validOpaqueID(input.DecisionID) && validOpaqueID(input.SideTaskID) &&
		(input.EffectKind == "continuation" || input.EffectKind == "cancellation") &&
		validSHA256(input.EffectDigest) && validOpaqueID(input.ExecutionTeamInstanceID) &&
		validSHA256(input.ExecutionPlanDigest) &&
		(input.TerminalStatus == "succeeded" || input.TerminalStatus == "cancelled" ||
			input.TerminalStatus == "failed") && validOpaqueID(input.TerminalEvidenceID) &&
		validSHA256(input.TerminalEvidenceDigest) && validSHA256(input.ExpectedViewVersion) &&
		validOpaqueID(input.CorrelationID)
}

func validSideTaskPurpose(value string) bool {
	switch value {
	case "research", "comparison", "diagnosis", "verification", "read_only_review":
		return true
	default:
		return false
	}
}

func validSideTaskMode(value string) bool {
	return value == "report_only" || value == "decision_required" || value == "merge_candidate"
}

func validSideTaskDecisionValue(value string) bool {
	switch value {
	case "absorb", "continue", "request_followup", "pivot", "discard", "archive", "cancel_parent":
		return true
	default:
		return false
	}
}

func sideTaskDecisionHasParentEffect(value string) bool {
	return value == "absorb" || value == "continue" || value == "cancel_parent"
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !(character >= '0' && character <= '9' || character >= 'a' && character <= 'f') {
			return false
		}
	}
	return true
}

func validBoundedSideTaskText(value string, maximum int) bool {
	return value != "" && len(value) <= maximum && validOpaqueID(value)
}

func validSortedScopes(values []string) bool {
	if values == nil || len(values) > maxSideTaskScopes {
		return false
	}
	for index, value := range values {
		if !validOpaqueID(value) || index > 0 && values[index-1] >= value {
			return false
		}
	}
	return true
}

func validCurrency(value string) bool {
	if len(value) != 3 {
		return false
	}
	for _, character := range value {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}

func exactSideTaskAdmission(record SideTaskHandoffRecord, input SideTaskAdmissionInput) bool {
	return record.ParentMissionID == input.ParentMissionID &&
		record.ParentTeamInstanceID == input.ParentTeamInstanceID &&
		record.ParentTaskID == input.ParentTaskID && record.ParentRunID == input.ParentRunID &&
		record.ParentClaimGeneration == input.ParentClaimGeneration &&
		record.ParentExecutionDigest == input.ParentExecutionDigest &&
		record.SideExecutionTeamInstanceID == input.SideExecutionTeamInstanceID &&
		record.Purpose == input.Purpose && record.Mode == input.Mode &&
		record.Title == input.Title && record.ProposalDigest == input.ProposalDigest &&
		record.InputArtifactDigest == input.InputArtifactDigest &&
		equalStringSlices(record.PermissionScopes, input.PermissionScopes)
}

func exactSideTaskHandoff(record SideTaskHandoffRecord, input SideTaskHandoffCommitInput) bool {
	timeoutSeconds := int64(input.DecisionTimeout / time.Second)
	return record.SideExecutionTeamInstanceID == input.SideExecutionTeamInstanceID &&
		record.SourceWorkItemID == input.SourceWorkItemID && record.SourceRunID == input.SourceRunID &&
		record.SourceGeneration == input.SourceGeneration &&
		record.SourceEvidenceID == input.SourceEvidenceID &&
		record.SourceEvidenceDigest == input.SourceEvidenceDigest &&
		record.VerifierEvidenceID == input.VerifierEvidenceID &&
		record.VerifierEvidenceDigest == input.VerifierEvidenceDigest &&
		record.HandoffVersion == input.HandoffVersion &&
		record.HandoffDigest == input.HandoffDigest &&
		record.SummaryArtifactDigest == input.SummaryArtifactDigest &&
		record.UsageObserved == input.UsageObserved &&
		record.UsageMicrounits == input.UsageMicrounits &&
		record.UsageCurrency == input.UsageCurrency &&
		record.DecisionTimeoutSeconds == timeoutSeconds
}

func sideTaskSnapshotExpectations(snapshot journal.StreamSetSnapshot) []journal.StreamHeadExpectation {
	heads := snapshot.Heads()
	expectations := make([]journal.StreamHeadExpectation, len(heads))
	for index, head := range heads {
		expectations[index] = journal.StreamHeadExpectation{
			StreamID: head.StreamID,
			Sequence: head.Sequence,
		}
	}
	return expectations
}

func findSideTaskAttempt(
	team TeamExecutionRecord,
	workItemID string,
	runID string,
	generation int64,
) *TeamAttemptRecord {
	for nodeIndex := range team.nodes {
		for attemptIndex := range team.nodes[nodeIndex].attempts {
			attempt := &team.nodes[nodeIndex].attempts[attemptIndex]
			if attempt.workItemID == workItemID && attempt.runID == runID &&
				attempt.claimGeneration == generation {
				return attempt
			}
		}
	}
	return nil
}

func validSideTaskTerminalLineage(
	ctx context.Context,
	snapshot journal.StreamSetSnapshot,
	record SideTaskHandoffRecord,
	input SideTaskHandoffCommitInput,
) bool {
	team, err := replayTeamExecution(input.SideExecutionTeamInstanceID,
		filterTeamEvents(snapshot.Events(),
			teamExecutionStream(input.SideExecutionTeamInstanceID)))
	if err != nil || team.status != "succeeded" {
		return false
	}
	attempt := findSideTaskAttempt(team, input.SourceWorkItemID,
		input.SourceRunID, input.SourceGeneration)
	if attempt == nil || attempt.status != "succeeded" ||
		attempt.evidenceID != input.SourceEvidenceID ||
		attempt.evidenceDigest != input.SourceEvidenceDigest {
		return false
	}
	state, err := replayAuthorityEventsSelective(ctx, snapshot.Events())
	if err != nil {
		return false
	}
	workItem, ok := state.workItems[input.SourceWorkItemID]
	acceptedStatus := workItem.status == "done" ||
		workItem.status == "ready_for_review" && workItem.verificationEventID != "" &&
			workItem.acceptanceDecisionDigest != ""
	if !ok || !acceptedStatus || workItem.runID != input.SourceRunID ||
		workItem.verificationClaimGeneration != input.SourceGeneration ||
		workItem.sourceEvidenceID != input.SourceEvidenceID ||
		workItem.sourceEvidenceDigest != input.SourceEvidenceDigest {
		return false
	}
	if record.Mode == "merge_candidate" {
		return workItem.verifierRequired &&
			workItem.verifierEvidenceID == input.VerifierEvidenceID &&
			workItem.verifierEvidenceDigest == input.VerifierEvidenceDigest
	}
	return workItem.verifierEvidenceID == input.VerifierEvidenceID &&
		workItem.verifierEvidenceDigest == input.VerifierEvidenceDigest
}

func validParentDecisionLineage(
	_ context.Context,
	snapshot journal.StreamSetSnapshot,
	input SideTaskDecisionInput,
) bool {
	team, err := replayTeamExecution(input.ParentTeamInstanceID,
		filterTeamEvents(snapshot.Events(), teamExecutionStream(input.ParentTeamInstanceID)))
	if err != nil || !isTerminalTeamStatus(team.status) {
		return false
	}
	node := teamNodeByID(&team, input.ParentLogicalNodeID)
	attempt := teamAttemptByNumber(node, input.ParentAttemptNumber)
	return node != nil && attempt != nil &&
		attempt.workItemID == input.ParentTaskID &&
		attempt.runID == input.ParentRunID &&
		attempt.claimGeneration == input.ParentClaimGeneration &&
		attempt.status != ""
}

func validParentEffectTerminal(
	snapshot journal.StreamSetSnapshot,
	input ParentHandoffEffectCompletionInput,
	authorization parentEffectAuthorization,
) bool {
	team, err := replayTeamExecution(input.ExecutionTeamInstanceID,
		filterTeamEvents(snapshot.Events(), teamExecutionStream(input.ExecutionTeamInstanceID)))
	if err != nil || team.planDigest != input.ExecutionPlanDigest ||
		team.status != input.TerminalStatus || !isTerminalTeamStatus(team.status) {
		return false
	}
	var node *TeamNodeRecord
	if input.EffectKind == "continuation" {
		if len(team.nodes) != 1 {
			return false
		}
		node = &team.nodes[0]
	} else if input.EffectKind == "cancellation" {
		node = teamNodeByID(&team, authorization.logicalNodeID)
	} else {
		return false
	}
	if node == nil || node.currentAttempt <= 0 {
		return false
	}
	attemptNumber := node.currentAttempt
	if input.EffectKind == "cancellation" &&
		(attemptNumber != authorization.attemptNumber) {
		return false
	}
	attempt := teamAttemptByNumber(node, attemptNumber)
	return attempt != nil && attempt.status == input.TerminalStatus &&
		attempt.evidenceID == input.TerminalEvidenceID &&
		attempt.evidenceDigest == input.TerminalEvidenceDigest &&
		validSideTaskEvidenceFact(snapshot.Events(), input.TerminalEvidenceID,
			input.TerminalEvidenceDigest, attempt.workItemID)
}

func validSideTaskEvidenceFact(
	events []journal.Event,
	evidenceID string,
	digest string,
	workItemID string,
) bool {
	streamID := "evidence/" + evidenceID
	for _, event := range events {
		if event.StreamID != streamID {
			continue
		}
		var payload struct {
			EvidenceID string `json:"evidence_id"`
			WorkItemID string `json:"work_item_id"`
			Digest     string `json:"digest"`
		}
		return event.Seq == 1 && event.SchemaVersion == 1 &&
			event.Type == "EvidenceSubmitted" &&
			decodeExactPayload(event.PayloadJSON, &payload) == nil &&
			payload.EvidenceID == evidenceID && payload.WorkItemID == workItemID &&
			payload.Digest == digest
	}
	return false
}

func exactParentEffectCompletion(
	payload parentHandoffEffectCompletedPayload,
	input ParentHandoffEffectCompletionInput,
) bool {
	return payload.DecisionID == input.DecisionID &&
		payload.SideTaskID == input.SideTaskID &&
		payload.EffectKind == input.EffectKind &&
		payload.EffectDigest == input.EffectDigest &&
		payload.ExecutionTeamInstanceID == input.ExecutionTeamInstanceID &&
		payload.ExecutionPlanDigest == input.ExecutionPlanDigest &&
		payload.TerminalStatus == input.TerminalStatus &&
		payload.TerminalEvidenceID == input.TerminalEvidenceID &&
		payload.TerminalEvidenceDigest == input.TerminalEvidenceDigest
}

func equalStringSlices(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func sideTaskStream(id string) string      { return "side-task/" + id }
func parentHandoffStream(id string) string { return "parent-handoff/" + id }

func filterSideTaskEvents(events []journal.Event, streamID string) []journal.Event {
	filtered := make([]journal.Event, 0)
	for _, event := range events {
		if event.StreamID == streamID {
			filtered = append(filtered, event)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Seq < filtered[j].Seq })
	return filtered
}

func mapSideTaskWriteError(err error) error {
	if errors.Is(err, journal.ErrStreamHeadConflict) ||
		errors.Is(err, journal.ErrSequenceConflict) ||
		errors.Is(err, journal.ErrIdempotencyConflict) ||
		errors.Is(err, journal.ErrPartialEventBatchConflict) {
		return fmt.Errorf("%w: %v", ErrSideTaskConflict, err)
	}
	return err
}

func cloneSideTaskHandoff(record SideTaskHandoffRecord) SideTaskHandoffRecord {
	record.PermissionScopes = append([]string{}, record.PermissionScopes...)
	return record
}
