package projection

import (
	"sort"
	"strings"
	"time"

	"loom-pi-rebuild/internal/journal"
)

type SideTaskHandoff struct {
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
	DecisionDeadline                    time.Time
	DecisionID                          string
	Decision                            string
	ContextPacketDigest                 string
	ContinuationExecutionTeamInstanceID string
	ContinuationPlanDigest              string
	ParentLogicalNodeID                 string
	ParentAttemptNumber                 int
	CancellationEffectDigest            string
	EffectDigest                        string
	EffectStatus                        string
	LastEventID                         string
	StreamSequence                      int64
}

func projectSideTaskHandoffs(events []journal.Event) (map[string]SideTaskHandoff, error) {
	byStream := make(map[string][]journal.Event)
	parentEvents := make([]journal.Event, 0)
	for _, event := range events {
		if strings.HasPrefix(event.StreamID, "side-task/") {
			byStream[event.StreamID] = append(byStream[event.StreamID], event)
		}
		if strings.HasPrefix(event.StreamID, "parent-handoff/") {
			parentEvents = append(parentEvents, event)
		}
	}
	projected := make(map[string]SideTaskHandoff, len(byStream))
	for streamID, streamEvents := range byStream {
		sort.Slice(streamEvents, func(i, j int) bool {
			return streamEvents[i].Seq < streamEvents[j].Seq
		})
		sideTaskID := strings.TrimPrefix(streamID, "side-task/")
		record, err := projectSideTaskStream(sideTaskID, streamEvents)
		if err != nil {
			return nil, err
		}
		projected[sideTaskID] = record
	}
	sort.Slice(parentEvents, func(i, j int) bool {
		if parentEvents[i].StreamID != parentEvents[j].StreamID {
			return parentEvents[i].StreamID < parentEvents[j].StreamID
		}
		return parentEvents[i].Seq < parentEvents[j].Seq
	})
	parentSequences := make(map[string]int64)
	for _, event := range parentEvents {
		parentTaskID := strings.TrimPrefix(event.StreamID, "parent-handoff/")
		if parentTaskID == "" || event.SchemaVersion != 1 ||
			event.Seq != parentSequences[event.StreamID]+1 {
			return nil, ErrInvalidProjectionEvent
		}
		if err := applyParentHandoffProjection(projected, event); err != nil {
			return nil, err
		}
		parentSequences[event.StreamID] = event.Seq
	}
	return projected, nil
}

func projectSideTaskStream(sideTaskID string, events []journal.Event) (SideTaskHandoff, error) {
	var record SideTaskHandoff
	for index, event := range events {
		if event.StreamID != "side-task/"+sideTaskID || event.Seq != int64(index+1) ||
			event.SchemaVersion != 1 {
			return SideTaskHandoff{}, ErrInvalidProjectionEvent
		}
		switch event.Type {
		case "SideTaskAdmitted":
			var payload struct {
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
			if record.SideTaskID != "" ||
				decodeExactProjectionPayload(event, &payload) != nil ||
				payload.SideTaskID != sideTaskID ||
				payload.AdmissionKind != "explicit_confirmation" ||
				!validSHA256Digest(payload.ProposalDigest) ||
				!validSHA256Digest(payload.InputArtifactDigest) ||
				!validSHA256Digest(payload.ParentExecutionDigest) ||
				!validSHA256Digest(payload.ExpectedViewVersion) ||
				!validProjectedSideTaskPurpose(payload.Purpose) ||
				!validProjectedSideTaskMode(payload.Mode) ||
				payload.ParentClaimGeneration < 0 || payload.PermissionScopes == nil ||
				payload.PolicyStreamID != "" || payload.PolicyVersion != 0 ||
				payload.PolicyDigest != "" {
				return SideTaskHandoff{}, ErrInvalidProjectionEvent
			}
			record = SideTaskHandoff{
				SideTaskID:                  payload.SideTaskID,
				ParentMissionID:             payload.ParentMissionID,
				ParentTeamInstanceID:        payload.ParentTeamInstanceID,
				ParentTaskID:                payload.ParentTaskID,
				ParentRunID:                 payload.ParentRunID,
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
			var payload struct {
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
			if record.SideTaskID == "" || record.HandoffVersion != 0 ||
				decodeExactProjectionPayload(event, &payload) != nil ||
				payload.SideTaskID != sideTaskID || payload.Mode != record.Mode ||
				!validProjectedSideTaskHandoffStatus(payload.Mode, payload.Status) ||
				payload.SourceGeneration < 0 || payload.SourceWorkItemID == "" ||
				payload.SourceRunID == "" || payload.SourceEvidenceID == "" ||
				!validSHA256Digest(payload.SourceEvidenceDigest) ||
				(payload.VerifierEvidenceID == "") != (payload.VerifierEvidenceDigest == "") ||
				payload.VerifierEvidenceDigest != "" &&
					!validSHA256Digest(payload.VerifierEvidenceDigest) ||
				payload.HandoffVersion <= 0 ||
				!validSHA256Digest(payload.HandoffDigest) ||
				!validSHA256Digest(payload.SummaryArtifactDigest) ||
				!validProjectedSideTaskUsage(payload.UsageObserved,
					payload.UsageMicrounits, payload.UsageCurrency) ||
				!validProjectedSideTaskTimeout(record.Mode,
					payload.DecisionTimeoutSeconds, payload.DecisionDeadline) {
				return SideTaskHandoff{}, ErrInvalidProjectionEvent
			}
			deadline, err := parseProjectedOptionalUTC(payload.DecisionDeadline)
			if err != nil {
				return SideTaskHandoff{}, err
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
			record.DecisionDeadline = deadline
		case "SideTaskDecisionCommitted":
			var payload struct {
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
			if record.HandoffVersion == 0 || record.DecisionID != "" ||
				decodeExactProjectionPayload(event, &payload) != nil ||
				payload.SideTaskID != sideTaskID ||
				!validProjectedSideTaskDecision(payload.Decision) ||
				!validProjectedSideTaskDecisionStatus(payload.Decision,
					payload.ResultingStatus) ||
				payload.HandoffDigest != record.HandoffDigest ||
				payload.ParentTaskID != record.ParentTaskID ||
				payload.ParentRunID != record.ParentRunID ||
				payload.ParentClaimGeneration != record.ParentClaimGeneration ||
				payload.SideTaskGeneration != record.SourceGeneration ||
				payload.HandoffVersion != record.HandoffVersion ||
				!validSHA256Digest(payload.ExpectedViewVersion) ||
				!validSHA256Digest(payload.EffectDigest) ||
				(payload.Decision == "absorb") != validSHA256Digest(payload.ContextPacketDigest) ||
				payload.Decision != "absorb" && payload.ContextPacketDigest != "" {
				return SideTaskHandoff{}, ErrInvalidProjectionEvent
			}
			record.DecisionID = payload.DecisionID
			record.Decision = payload.Decision
			record.ContextPacketDigest = payload.ContextPacketDigest
			record.EffectDigest = payload.EffectDigest
			record.Status = payload.ResultingStatus
			if payload.Decision == "absorb" || payload.Decision == "continue" ||
				payload.Decision == "cancel_parent" {
				record.EffectStatus = "pending"
			}
		case "SideTaskDecisionTimedOut":
			var payload struct {
				SideTaskID           string `json:"side_task_id"`
				HandoffVersion       int    `json:"handoff_version"`
				HandoffDigest        string `json:"handoff_digest"`
				SourceHandoffEventID string `json:"source_handoff_event_id"`
				DecisionDeadline     string `json:"decision_deadline"`
				TimeoutObservedAt    string `json:"timeout_observed_at"`
				Outcome              string `json:"outcome"`
			}
			if record.HandoffVersion == 0 || record.DecisionID != "" ||
				decodeExactProjectionPayload(event, &payload) != nil {
				return SideTaskHandoff{}, ErrInvalidProjectionEvent
			}
			deadline, deadlineErr := parseProjectedOptionalUTC(payload.DecisionDeadline)
			observedAt, observedErr := parseProjectedOptionalUTC(payload.TimeoutObservedAt)
			if payload.SideTaskID != sideTaskID ||
				payload.HandoffVersion != record.HandoffVersion ||
				payload.HandoffDigest != record.HandoffDigest ||
				payload.SourceHandoffEventID != record.LastEventID ||
				deadlineErr != nil || observedErr != nil || deadline.IsZero() || observedAt.IsZero() ||
				!deadline.Equal(record.DecisionDeadline) || observedAt.Before(deadline) ||
				(payload.Outcome != "paused" && payload.Outcome != "human_required") {
				return SideTaskHandoff{}, ErrInvalidProjectionEvent
			}
			record.Status = payload.Outcome
		default:
			return SideTaskHandoff{}, ErrInvalidProjectionEvent
		}
		record.LastEventID = event.ID
		record.StreamSequence = event.Seq
	}
	return cloneGlobalSideTaskHandoff(record), nil
}

func applyParentHandoffProjection(records map[string]SideTaskHandoff, event journal.Event) error {
	if event.Type == "ContextPacketCommitted" {
		var packet struct {
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
		}
		if decodeExactProjectionPayload(event, &packet) != nil ||
			packet.ContextPacketID == "" || packet.ContextPacketVersion != 1 ||
			!validSHA256Digest(packet.ContextPacketDigest) ||
			packet.SideTaskID == "" || packet.HandoffVersion <= 0 ||
			!validSHA256Digest(packet.HandoffDigest) ||
			!validSHA256Digest(packet.SummaryArtifactDigest) ||
			packet.ParentTaskID == "" || packet.ParentRunID == "" ||
			packet.ParentClaimGeneration < 0 || packet.SideTaskGeneration < 0 ||
			packet.ContinuationExecutionTeamInstanceID == "" {
			return ErrInvalidProjectionEvent
		}
		record, ok := records[packet.SideTaskID]
		if !ok || event.StreamID != "parent-handoff/"+record.ParentTaskID ||
			record.Decision != "absorb" || record.ContextPacketDigest != packet.ContextPacketDigest ||
			record.HandoffVersion != packet.HandoffVersion ||
			record.HandoffDigest != packet.HandoffDigest ||
			record.SummaryArtifactDigest != packet.SummaryArtifactDigest ||
			record.ParentTaskID != packet.ParentTaskID || record.ParentRunID != packet.ParentRunID ||
			record.ParentClaimGeneration != packet.ParentClaimGeneration ||
			record.SourceGeneration != packet.SideTaskGeneration {
			return ErrInvalidProjectionEvent
		}
		record.ContextPacketDigest = packet.ContextPacketDigest
		record.ContinuationExecutionTeamInstanceID = packet.ContinuationExecutionTeamInstanceID
		records[packet.SideTaskID] = record
		return nil
	}
	if event.Type == "ParentHandoffEffectCompleted" {
		var completed struct {
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
		if decodeExactProjectionPayload(event, &completed) != nil ||
			(completed.EffectKind != "continuation" && completed.EffectKind != "cancellation") ||
			(completed.TerminalStatus != "succeeded" && completed.TerminalStatus != "failed" &&
				completed.TerminalStatus != "cancelled") ||
			completed.ExecutionTeamInstanceID == "" || completed.TerminalEvidenceID == "" ||
			!validSHA256Digest(completed.ExecutionPlanDigest) ||
			!validSHA256Digest(completed.EffectDigest) ||
			!validSHA256Digest(completed.TerminalEvidenceDigest) {
			return ErrInvalidProjectionEvent
		}
		record, ok := records[completed.SideTaskID]
		if !ok || event.StreamID != "parent-handoff/"+record.ParentTaskID ||
			record.EffectStatus != "pending" ||
			record.DecisionID != completed.DecisionID ||
			record.EffectDigest != completed.EffectDigest ||
			completed.EffectKind == "continuation" &&
				(record.Decision != "absorb" && record.Decision != "continue" ||
					completed.ExecutionTeamInstanceID != record.ContinuationExecutionTeamInstanceID ||
					completed.ExecutionPlanDigest != record.ContinuationPlanDigest) ||
			completed.EffectKind == "cancellation" &&
				(record.Decision != "cancel_parent" ||
					completed.ExecutionTeamInstanceID != record.ParentTeamInstanceID) {
			return ErrInvalidProjectionEvent
		}
		record.EffectStatus = "completed"
		records[completed.SideTaskID] = record
		return nil
	}
	switch event.Type {
	case "ParentContinuationAuthorized":
		var binding struct {
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
		if decodeExactProjectionPayload(event, &binding) != nil ||
			(binding.Action != "absorb" && binding.Action != "continue") ||
			binding.HandoffVersion <= 0 || binding.ContinuationExecutionTeamInstanceID == "" ||
			!validSHA256Digest(binding.ContinuationPlanDigest) ||
			!validSHA256Digest(binding.EffectDigest) ||
			binding.Action == "absorb" && !validSHA256Digest(binding.ContextPacketDigest) ||
			binding.Action == "continue" && binding.ContextPacketDigest != "" {
			return ErrInvalidProjectionEvent
		}
		record, ok := records[binding.SideTaskID]
		if !ok || event.StreamID != "parent-handoff/"+record.ParentTaskID ||
			record.DecisionID != binding.DecisionID ||
			record.EffectDigest != binding.EffectDigest || record.HandoffDigest != binding.HandoffDigest ||
			record.HandoffVersion != binding.HandoffVersion ||
			record.ParentMissionID != binding.ParentMissionID ||
			record.ParentTeamInstanceID != binding.ParentTeamInstanceID ||
			record.ParentTaskID != binding.ParentTaskID || record.ParentRunID != binding.ParentRunID ||
			record.ParentClaimGeneration != binding.ParentClaimGeneration ||
			record.SourceGeneration != binding.SideTaskGeneration ||
			record.ContextPacketDigest != binding.ContextPacketDigest ||
			record.ContinuationExecutionTeamInstanceID != "" &&
				record.ContinuationExecutionTeamInstanceID != binding.ContinuationExecutionTeamInstanceID {
			return ErrInvalidProjectionEvent
		}
		record.ContinuationExecutionTeamInstanceID = binding.ContinuationExecutionTeamInstanceID
		record.ContinuationPlanDigest = binding.ContinuationPlanDigest
		records[binding.SideTaskID] = record
		return nil
	case "ParentFollowupProposed":
		var payload struct {
			DecisionID             string `json:"decision_id"`
			SideTaskID             string `json:"side_task_id"`
			HandoffVersion         int    `json:"handoff_version"`
			HandoffDigest          string `json:"handoff_digest"`
			ParentTaskID           string `json:"parent_task_id"`
			ParentRunID            string `json:"parent_run_id"`
			ParentClaimGeneration  int64  `json:"parent_claim_generation"`
			FollowupProposalDigest string `json:"followup_proposal_digest"`
			EffectDigest           string `json:"effect_digest"`
		}
		if decodeExactProjectionPayload(event, &payload) != nil ||
			payload.HandoffVersion <= 0 || !validSHA256Digest(payload.HandoffDigest) ||
			payload.ParentTaskID == "" || payload.ParentRunID == "" ||
			payload.ParentClaimGeneration < 0 ||
			!validSHA256Digest(payload.FollowupProposalDigest) ||
			!validSHA256Digest(payload.EffectDigest) {
			return ErrInvalidProjectionEvent
		}
		return bindProjectedParentIntent(records, event, payload.SideTaskID,
			payload.DecisionID, "request_followup", payload.HandoffVersion,
			payload.HandoffDigest, payload.ParentTaskID, payload.ParentRunID,
			payload.ParentClaimGeneration, payload.EffectDigest)
	case "ParentScopePivotProposed":
		var payload struct {
			DecisionID            string `json:"decision_id"`
			SideTaskID            string `json:"side_task_id"`
			HandoffVersion        int    `json:"handoff_version"`
			HandoffDigest         string `json:"handoff_digest"`
			ParentTaskID          string `json:"parent_task_id"`
			ParentRunID           string `json:"parent_run_id"`
			ParentClaimGeneration int64  `json:"parent_claim_generation"`
			ScopeDeltaDigest      string `json:"scope_delta_digest"`
			EffectDigest          string `json:"effect_digest"`
		}
		if decodeExactProjectionPayload(event, &payload) != nil ||
			payload.HandoffVersion <= 0 || !validSHA256Digest(payload.HandoffDigest) ||
			payload.ParentTaskID == "" || payload.ParentRunID == "" ||
			payload.ParentClaimGeneration < 0 ||
			!validSHA256Digest(payload.ScopeDeltaDigest) ||
			!validSHA256Digest(payload.EffectDigest) {
			return ErrInvalidProjectionEvent
		}
		return bindProjectedParentIntent(records, event, payload.SideTaskID,
			payload.DecisionID, "pivot", payload.HandoffVersion,
			payload.HandoffDigest, payload.ParentTaskID, payload.ParentRunID,
			payload.ParentClaimGeneration, payload.EffectDigest)
	case "ParentCancellationRequested":
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
		if decodeExactProjectionPayload(event, &payload) != nil ||
			payload.HandoffVersion <= 0 || !validSHA256Digest(payload.HandoffDigest) ||
			!validSHA256Digest(payload.ParentExecutionDigest) ||
			!validSHA256Digest(payload.CancellationEffectDigest) ||
			!validSHA256Digest(payload.EffectDigest) {
			return ErrInvalidProjectionEvent
		}
		record, ok := records[payload.SideTaskID]
		if !ok || event.StreamID != "parent-handoff/"+record.ParentTaskID ||
			record.Decision != "cancel_parent" || record.DecisionID != payload.DecisionID ||
			record.HandoffVersion != payload.HandoffVersion ||
			record.HandoffDigest != payload.HandoffDigest || record.EffectDigest != payload.EffectDigest ||
			record.ParentMissionID != payload.ParentMissionID || record.ParentTeamInstanceID != payload.ParentTeamInstanceID ||
			record.ParentTaskID != payload.ParentTaskID || record.ParentRunID != payload.ParentRunID ||
			record.ParentClaimGeneration != payload.ParentClaimGeneration ||
			record.ParentExecutionDigest != payload.ParentExecutionDigest ||
			payload.ParentLogicalNodeID == "" || payload.ParentAttemptNumber <= 0 {
			return ErrInvalidProjectionEvent
		}
		record.ParentLogicalNodeID = payload.ParentLogicalNodeID
		record.ParentAttemptNumber = payload.ParentAttemptNumber
		record.ParentExecutionDigest = payload.ParentExecutionDigest
		record.CancellationEffectDigest = payload.CancellationEffectDigest
		records[payload.SideTaskID] = record
		return nil
	default:
		return ErrInvalidProjectionEvent
	}
}

func bindProjectedParentIntent(
	records map[string]SideTaskHandoff,
	event journal.Event,
	sideTaskID, decisionID, decision string,
	handoffVersion int,
	handoffDigest, parentTaskID, parentRunID string,
	parentClaimGeneration int64,
	effectDigest string,
) error {
	record, ok := records[sideTaskID]
	if !ok || event.StreamID != "parent-handoff/"+record.ParentTaskID ||
		record.Decision != decision || record.DecisionID != decisionID ||
		record.HandoffVersion != handoffVersion || record.HandoffDigest != handoffDigest ||
		record.ParentTaskID != parentTaskID || record.ParentRunID != parentRunID ||
		record.ParentClaimGeneration != parentClaimGeneration ||
		record.EffectDigest != effectDigest {
		return ErrInvalidProjectionEvent
	}
	records[sideTaskID] = record
	return nil
}

func parseProjectedOptionalUTC(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || parsed.IsZero() || parsed.Location() != time.UTC {
		return time.Time{}, ErrInvalidProjectionEvent
	}
	return parsed, nil
}

func validProjectedSideTaskPurpose(value string) bool {
	switch value {
	case "research", "comparison", "diagnosis", "verification", "read_only_review":
		return true
	default:
		return false
	}
}

func validProjectedSideTaskMode(value string) bool {
	return value == "report_only" || value == "decision_required" || value == "merge_candidate"
}

func validProjectedSideTaskHandoffStatus(mode, status string) bool {
	return mode == "report_only" && status == "report_delivered" ||
		mode == "decision_required" && status == "decision_required" ||
		mode == "merge_candidate" && status == "merge_candidate_ready"
}

func validProjectedSideTaskDecision(value string) bool {
	switch value {
	case "absorb", "continue", "request_followup", "pivot", "discard", "archive", "cancel_parent":
		return true
	default:
		return false
	}
}

func validProjectedSideTaskDecisionStatus(decision, status string) bool {
	if decision == "archive" {
		return status == "archived"
	}
	return status == "decided"
}

func validProjectedSideTaskUsage(observed bool, microunits int64, currency string) bool {
	if !observed {
		return microunits == 0 && currency == ""
	}
	if microunits < 0 || len(currency) != 3 {
		return false
	}
	for _, character := range currency {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}

func validProjectedSideTaskTimeout(mode string, seconds int64, deadline string) bool {
	if mode == "report_only" {
		return seconds == 0 && deadline == ""
	}
	if seconds < 60 || seconds > 2592000 || deadline == "" {
		return false
	}
	_, err := parseProjectedOptionalUTC(deadline)
	return err == nil
}

func cloneGlobalSideTaskHandoff(record SideTaskHandoff) SideTaskHandoff {
	record.PermissionScopes = append([]string{}, record.PermissionScopes...)
	return record
}
