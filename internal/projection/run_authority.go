package projection

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"loom-pi-rebuild/internal/assets"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/work"
)

type runProjectionStatusReference struct {
	streamID string
	sequence int64
	eventID  string
}

func projectionDeterministicEventID(parts ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "evt-" + hex.EncodeToString(digest[:16])
}

func projectionAcceptanceOutcomeEventID(
	eventType string,
	payload runProjectionAcceptanceOutcomePayload,
) string {
	if eventType == "WorkItemDone" {
		return projectionDeterministicEventID(
			eventType,
			payload.WorkItemID,
			payload.VerificationEventID,
			payload.AcceptanceDecisionDigest,
			payload.SourceEvidenceDigest,
			payload.VerifierEvidenceDigest,
		)
	}
	return projectionDeterministicEventID(
		eventType,
		payload.WorkItemID,
		payload.RunID,
		fmt.Sprint(payload.ClaimGeneration),
		payload.Status,
		payload.VerificationEventID,
		payload.AcceptanceDecisionDigest,
		payload.RecoveryTrigger,
		fmt.Sprint(payload.RecoveryPolicyVersion),
		payload.RecoveryPolicyDigest,
		fmt.Sprint(payload.AttemptNumber),
		fmt.Sprint(payload.MaxAttempts),
		fmt.Sprint(payload.CreditsBefore),
	)
}

type runProjectionStatusFact struct {
	runtimeID string
	status    string
	capacity  int
}

type runProjectionStatusReferenceFields struct {
	RuntimeStatusStreamID *string `json:"runtime_status_stream_id"`
	RuntimeStatusSequence *int64  `json:"runtime_status_sequence"`
	RuntimeStatusEventID  *string `json:"runtime_status_event_id"`
}

type runProjectionBinding struct {
	workItemID        string
	runID             string
	claimID           string
	claimGeneration   int64
	runtimeInstanceID string
	agentInstanceID   string
}

type runProjectionProviderAccountBinding struct {
	runProjectionBinding
	providerID             string
	providerAccountID      string
	policyRevision         int64
	policyDigest           string
	executionBindingDigest string
	assignedBudgetUnits    int64
}

type runProjectionProviderAccountStart struct {
	reservedAt time.Time
	binding    runProjectionProviderAccountBinding
}

type runProjectionClaim struct {
	binding                runProjectionBinding
	previousBinding        runProjectionBinding
	statusReference        runProjectionStatusReference
	needsOldRelease        bool
	reserved               bool
	oldReleased            bool
	accountManaged         bool
	accountReserved        bool
	accountReleased        bool
	needsOldAccountRelease bool
	accountBinding         runProjectionProviderAccountBinding
	previousAccountBinding runProjectionProviderAccountBinding
}

type runProjectionTerminal struct {
	binding         runProjectionBinding
	statusReference runProjectionStatusReference
	status          string
	released        bool
	outcome         bool
	accountManaged  bool
	accountReleased bool
	accountBinding  runProjectionProviderAccountBinding
}

type runProjectionWork struct {
	record      WorkItem
	lastEventID string
	sequence    int64
}

type runProjectionRun struct {
	record                Run
	lastEventID           string
	sequence              int64
	prepareLeaseExpiresAt time.Time
}

type runProjectionClaimedPayload struct {
	ClaimContractVersion          *int                                `json:"claim_contract_version"`
	WorkItemID                    *string                             `json:"work_item_id"`
	RunID                         *string                             `json:"run_id"`
	ClaimID                       *string                             `json:"claim_id"`
	ClaimGeneration               *int64                              `json:"claim_generation"`
	RuntimeInstanceID             *string                             `json:"runtime_instance_id"`
	AgentInstanceID               *string                             `json:"agent_instance_id"`
	PrepareLeaseExpiresAt         *string                             `json:"prepare_lease_expires_at"`
	AssetRevisionBindings         *[]assets.ExactAssetRevisionBinding `json:"asset_revision_bindings"`
	AssetRevisionSetDigest        *string                             `json:"asset_revision_set_digest"`
	MaterializationManifestDigest *string                             `json:"materialization_manifest_digest"`
	MaterializationRootDigest     *string                             `json:"materialization_root_digest"`
	RateCardStatus                *string                             `json:"rate_card_status"`
	RateCard                      json.RawMessage                     `json:"rate_card"`
	runProjectionStatusReferenceFields
}

type runProjectionGenerationPayload struct {
	WorkItemID        *string `json:"work_item_id"`
	RunID             *string `json:"run_id"`
	ClaimID           *string `json:"claim_id"`
	ClaimGeneration   *int64  `json:"claim_generation"`
	RuntimeInstanceID *string `json:"runtime_instance_id"`
	AgentInstanceID   *string `json:"agent_instance_id"`
	runProjectionStatusReferenceFields
}

type runProjectionLeasePayload struct {
	WorkItemID             *string `json:"work_item_id"`
	RunID                  *string `json:"run_id"`
	ClaimID                *string `json:"claim_id"`
	ClaimGeneration        *int64  `json:"claim_generation"`
	RuntimeInstanceID      *string `json:"runtime_instance_id"`
	AgentInstanceID        *string `json:"agent_instance_id"`
	PreviousLeaseExpiresAt *string `json:"previous_lease_expires_at"`
	PrepareLeaseExpiresAt  *string `json:"prepare_lease_expires_at"`
}

type runProjectionTerminalPayload struct {
	WorkItemID        *string                         `json:"work_item_id"`
	RunID             *string                         `json:"run_id"`
	ClaimID           *string                         `json:"claim_id"`
	ClaimGeneration   *int64                          `json:"claim_generation"`
	RuntimeInstanceID *string                         `json:"runtime_instance_id"`
	AgentInstanceID   *string                         `json:"agent_instance_id"`
	Status            *string                         `json:"status"`
	Reason            *string                         `json:"reason"`
	Accounting        *runProjectionAccountingPayload `json:"accounting"`
	runProjectionStatusReferenceFields
}

type runProjectionAccountingPayload struct {
	UsageObserved    bool            `json:"usage_observed"`
	InputTokens      int64           `json:"input_tokens"`
	OutputTokens     int64           `json:"output_tokens"`
	CacheReadTokens  int64           `json:"cache_read_tokens"`
	CacheWriteTokens int64           `json:"cache_write_tokens"`
	TotalTokens      int64           `json:"total_tokens"`
	CostObserved     bool            `json:"cost_observed"`
	CostMicrounits   int64           `json:"cost_microunits"`
	CostCurrency     string          `json:"cost_currency"`
	CostSource       json.RawMessage `json:"cost_source"`
}

type runProjectionCapacityPayload struct {
	WorkItemID            *string `json:"work_item_id"`
	RunID                 *string `json:"run_id"`
	ClaimID               *string `json:"claim_id"`
	ClaimGeneration       *int64  `json:"claim_generation"`
	RuntimeInstanceID     *string `json:"runtime_instance_id"`
	AgentInstanceID       *string `json:"agent_instance_id"`
	RuntimeStatusStreamID *string `json:"runtime_status_stream_id"`
	RuntimeStatusSequence *int64  `json:"runtime_status_sequence"`
	RuntimeStatusEventID  *string `json:"runtime_status_event_id"`
}

type runProjectionProviderAccountCapacityPayload struct {
	WorkItemID             *string `json:"work_item_id"`
	RunID                  *string `json:"run_id"`
	ClaimID                *string `json:"claim_id"`
	ClaimGeneration        *int64  `json:"claim_generation"`
	RuntimeInstanceID      *string `json:"runtime_instance_id"`
	AgentInstanceID        *string `json:"agent_instance_id"`
	ProviderID             *string `json:"provider_id"`
	ProviderAccountID      *string `json:"provider_account_id"`
	PolicyRevision         *int64  `json:"policy_revision"`
	PolicyDigest           *string `json:"policy_digest"`
	ExecutionBindingDigest *string `json:"execution_binding_digest"`
	AssignedBudgetUnits    *int64  `json:"assigned_budget_units"`
}

type runProjectionVerificationPayload struct {
	TeamInstanceID              string `json:"team_instance_id"`
	PlanDigest                  string `json:"plan_digest"`
	LogicalNodeID               string `json:"logical_node_id"`
	AttemptNumber               int    `json:"attempt_number"`
	WorkItemID                  string `json:"work_item_id"`
	RunID                       string `json:"run_id"`
	ClaimID                     string `json:"claim_id"`
	ClaimGeneration             int64  `json:"claim_generation"`
	SourceEvidenceID            string `json:"source_evidence_id"`
	SourceEvidenceDigest        string `json:"source_evidence_digest"`
	OutputSummaryDigest         string `json:"output_summary_digest"`
	OutputContractVersion       int    `json:"output_contract_version"`
	OutputContractDigest        string `json:"output_contract_digest"`
	OutputClassification        string `json:"output_classification"`
	OutputClassificationDigest  string `json:"output_classification_digest"`
	AcceptanceContractVersion   int    `json:"acceptance_contract_version"`
	AcceptanceContractDigest    string `json:"acceptance_contract_digest"`
	Risk                        string `json:"risk"`
	DeterministicResultDigest   string `json:"deterministic_result_digest"`
	VerifierRequired            bool   `json:"verifier_required"`
	VerifierWorkItemID          string `json:"verifier_work_item_id"`
	VerifierRunID               string `json:"verifier_run_id"`
	VerifierClaimID             string `json:"verifier_claim_id"`
	VerifierClaimGeneration     int64  `json:"verifier_claim_generation"`
	VerifierRuntimeInstanceID   string `json:"verifier_runtime_instance_id"`
	VerifierAgentInstanceID     string `json:"verifier_agent_instance_id"`
	VerifierGrantID             string `json:"verifier_grant_id"`
	VerifierEvidenceID          string `json:"verifier_evidence_id"`
	VerifierEvidenceDigest      string `json:"verifier_evidence_digest"`
	VerifierOutputSummaryDigest string `json:"verifier_output_summary_digest"`
	VerifierCandidateKind       string `json:"verifier_candidate_kind"`
	VerifierReasonCode          string `json:"verifier_reason_code"`
	VerifierCandidateDigest     string `json:"verifier_candidate_digest"`
	AcceptanceDecisionKind      string `json:"acceptance_decision_kind"`
	AcceptanceDecisionDigest    string `json:"acceptance_decision_digest"`
	DecidedAt                   string `json:"decided_at"`
}

type runProjectionAcceptanceOutcomePayload struct {
	WorkItemID               string `json:"work_item_id"`
	RunID                    string `json:"run_id"`
	ClaimGeneration          int64  `json:"claim_generation"`
	Status                   string `json:"status"`
	VerificationEventID      string `json:"verification_event_id"`
	AcceptanceDecisionDigest string `json:"acceptance_decision_digest"`
	SourceEvidenceID         string `json:"source_evidence_id,omitempty"`
	SourceEvidenceDigest     string `json:"source_evidence_digest,omitempty"`
	VerifierEvidenceID       string `json:"verifier_evidence_id,omitempty"`
	VerifierEvidenceDigest   string `json:"verifier_evidence_digest,omitempty"`
	RecoveryTrigger          string `json:"recovery_trigger,omitempty"`
	RecoveryPolicyVersion    int    `json:"recovery_policy_version,omitempty"`
	RecoveryPolicyDigest     string `json:"recovery_policy_digest,omitempty"`
	AttemptNumber            int    `json:"attempt_number,omitempty"`
	MaxAttempts              int    `json:"max_attempts,omitempty"`
	CreditsBefore            int    `json:"credits_before,omitempty"`
}

func isRunAuthorityProjectionEvent(event journal.Event) bool {
	switch {
	case strings.HasPrefix(event.StreamID, "work-item/"):
		return true
	case strings.HasPrefix(event.StreamID, "run/"):
		return true
	case strings.HasPrefix(event.StreamID, "runtime_capacity:"):
		return true
	case strings.HasPrefix(event.StreamID, "provider-account-capacity/"):
		return true
	default:
		return false
	}
}

func isRunAuthorityRuntimeReferenceEvent(event journal.Event) bool {
	return strings.HasPrefix(event.StreamID, "runtime_instance:") &&
		(event.Type == "RuntimeInstanceDiscovered" ||
			event.Type == "RuntimeInstanceStatusChanged")
}

func applyRunAuthorityProjection(
	ctx context.Context,
	snapshot *Snapshot,
	events []journal.Event,
	authorityEvents []journal.Event,
) error {
	if snapshot == nil {
		return ErrInvalidProjectionEvent
	}
	statusFacts, err := indexRunProjectionStatusFacts(ctx, events)
	if err != nil {
		return fmt.Errorf("status facts: %w", err)
	}
	workItems, runsByAssignment, outcomes, err := indexRunProjectionWorkItems(ctx, events)
	if err != nil {
		return fmt.Errorf("work items: %w", err)
	}
	claims := make(map[string]*runProjectionClaim)
	terminals := make(map[string]*runProjectionTerminal)
	accountPolicies, err := indexRunProjectionProviderAccountPolicies(
		ctx, authorityEvents,
	)
	if err != nil {
		return fmt.Errorf("Provider Account policies: %w", err)
	}
	rateCards, err := indexRunProjectionProviderModelRateCards(ctx, authorityEvents)
	if err != nil {
		return fmt.Errorf("Provider model Rate Cards: %w", err)
	}
	runs, err := replayRunProjectionRuns(
		ctx, events, runsByAssignment, workItems, statusFacts, accountPolicies,
		rateCards,
		claims, terminals,
	)
	if err != nil {
		return fmt.Errorf("runs: %w", err)
	}
	if err := replayRunProjectionCapacity(
		ctx, events, snapshot, statusFacts, claims, terminals,
	); err != nil {
		return fmt.Errorf("capacity: %w", err)
	}
	if err := replayRunProjectionProviderAccountCapacity(
		ctx, events, accountPolicies, runs, claims, terminals,
	); err != nil {
		return fmt.Errorf("Provider Account capacity: %w", err)
	}
	for eventID, claim := range claims {
		if !claim.reserved || claim.needsOldRelease && !claim.oldReleased ||
			claim.accountManaged && (!claim.accountReserved ||
				claim.needsOldAccountRelease && !claim.accountReleased) {
			return fmt.Errorf("%w: incomplete claim %s", ErrInvalidProjectionEvent, eventID)
		}
	}
	for eventID, terminal := range terminals {
		outcome, ok := outcomes[eventID]
		if !ok || !terminal.released ||
			terminal.accountManaged && !terminal.accountReleased {
			return fmt.Errorf("%w: incomplete terminal %s", ErrInvalidProjectionEvent, eventID)
		}
		workItem := workItems[terminal.binding.workItemID]
		if err := validateRunProjectionOutcome(outcome, terminal, &workItem); err != nil {
			return err
		}
		workItems[workItem.record.ID] = workItem
		terminal.outcome = true
	}
	for _, outcome := range outcomes {
		if _, ok := terminals[outcome.CausationID]; !ok {
			return fmt.Errorf("%w: orphan WorkItem outcome", ErrInvalidProjectionEvent)
		}
	}
	for runID, run := range runs {
		snapshot.Runs[runID] = run.record
	}
	for workItemID, workItem := range workItems {
		snapshot.WorkItems[workItemID] = workItem.record
	}
	return nil
}

func indexRunProjectionStatusFacts(
	ctx context.Context,
	events []journal.Event,
) (map[runProjectionStatusReference]runProjectionStatusFact, error) {
	byStream := make(map[string][]journal.Event)
	for _, event := range events {
		if isRunAuthorityRuntimeReferenceEvent(event) {
			byStream[event.StreamID] = append(byStream[event.StreamID], event)
		}
	}
	facts := make(map[runProjectionStatusReference]runProjectionStatusFact)
	for streamID, streamEvents := range byStream {
		sort.SliceStable(streamEvents, func(i, j int) bool {
			return streamEvents[i].Seq < streamEvents[j].Seq
		})
		var current RuntimeInstance
		for _, event := range streamEvents {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			switch event.Type {
			case "RuntimeInstanceDiscovered":
				next, err := projectRuntimeDiscoveryEvent(event)
				if err != nil {
					return nil, err
				}
				if current.ID != "" &&
					(current.ID != next.ID ||
						current.DeviceID != next.DeviceID ||
						current.AdapterType != next.AdapterType) {
					return nil, ErrInvalidProjectionEvent
				}
				current = next
			case "RuntimeInstanceStatusChanged":
				next, err := projectRuntimeStatusEvent(event, current)
				if err != nil {
					return nil, err
				}
				current = next
			default:
				return nil, ErrInvalidProjectionEvent
			}
			reference := runProjectionStatusReference{
				streamID: streamID,
				sequence: event.Seq,
				eventID:  event.ID,
			}
			if streamID != "runtime_instance:"+current.ID {
				return nil, ErrInvalidProjectionEvent
			}
			facts[reference] = runProjectionStatusFact{
				runtimeID: current.ID,
				status:    current.Status,
				capacity:  current.Capacity,
			}
		}
	}
	return facts, nil
}

func indexRunProjectionWorkItems(
	ctx context.Context,
	events []journal.Event,
) (
	map[string]runProjectionWork,
	map[string]runProjectionRun,
	map[string]journal.Event,
	error,
) {
	byStream := make(map[string][]journal.Event)
	for _, event := range events {
		if strings.HasPrefix(event.StreamID, "work-item/") {
			byStream[event.StreamID] = append(byStream[event.StreamID], event)
		}
	}
	workItems := make(map[string]runProjectionWork)
	runs := make(map[string]runProjectionRun)
	outcomes := make(map[string]journal.Event)
	for streamID, streamEvents := range byStream {
		sort.SliceStable(streamEvents, func(i, j int) bool {
			return streamEvents[i].Seq < streamEvents[j].Seq
		})
		workItemID := strings.TrimPrefix(streamID, "work-item/")
		var workItem runProjectionWork
		for _, event := range streamEvents {
			if err := ctx.Err(); err != nil {
				return nil, nil, nil, err
			}
			if !validRunProjectionEnvelope(event) {
				return nil, nil, nil, ErrInvalidProjectionEvent
			}
			switch event.Type {
			case "WorkItemCreated":
				var payload struct {
					WorkItemID *string `json:"work_item_id"`
					Title      *string `json:"title"`
					Status     *string `json:"status"`
				}
				if err := decodeRunProjectionPayload(event, &payload); err != nil ||
					payload.WorkItemID == nil || payload.Title == nil ||
					payload.Status == nil || *payload.WorkItemID != workItemID ||
					*payload.Title == "" || *payload.Status != "ready" ||
					event.Seq != 1 || event.CausationID != "" ||
					workItem.record.ID != "" {
					return nil, nil, nil, ErrInvalidProjectionEvent
				}
				workItem = runProjectionWork{
					record: WorkItem{
						ID: workItemID, Title: *payload.Title, Status: "ready",
					},
					lastEventID: event.ID,
					sequence:    event.Seq,
				}
			case "WorkItemAssigned":
				var payload struct {
					WorkItemID       *string                           `json:"work_item_id"`
					RunID            *string                           `json:"run_id"`
					AgentInstanceID  *string                           `json:"agent_instance_id"`
					Status           *string                           `json:"status"`
					ExecutionBinding *projectedExecutionBindingPayload `json:"execution_binding,omitempty"`
				}
				if err := decodeRunProjectionPayload(event, &payload); err != nil ||
					workItem.record.ID == "" || payload.WorkItemID == nil ||
					payload.RunID == nil || payload.AgentInstanceID == nil ||
					payload.Status == nil || *payload.WorkItemID != workItemID ||
					*payload.RunID == "" || *payload.AgentInstanceID == "" ||
					*payload.Status != "assigned" ||
					event.Seq != workItem.sequence+1 ||
					event.CausationID != workItem.lastEventID {
					return nil, nil, nil, ErrInvalidProjectionEvent
				}
				if _, duplicate := runs[*payload.RunID]; duplicate {
					return nil, nil, nil, ErrInvalidProjectionEvent
				}
				var bindingAvailable bool
				var binding projectedFrozenExecutionBinding
				if payload.ExecutionBinding != nil {
					var bindingErr error
					binding, bindingErr = projectedExecutionBinding(
						payload.ExecutionBinding,
					)
					if bindingErr != nil {
						return nil, nil, nil, ErrInvalidProjectionEvent
					}
					bindingAvailable = true
				}
				workItem.record.Status = "assigned"
				workItem.record.RunID = *payload.RunID
				workItem.record.AgentInstanceID = *payload.AgentInstanceID
				workItem.lastEventID = event.ID
				workItem.sequence = event.Seq
				runs[*payload.RunID] = runProjectionRun{
					record: Run{
						ID: *payload.RunID, WorkItemID: workItemID,
						Phase:                     "unclaimed",
						AgentInstanceID:           *payload.AgentInstanceID,
						ExecutionBindingAvailable: bindingAvailable,
						ExecutionBinding:          binding,
					},
					lastEventID: event.ID,
				}
			case "WorkItemReadyForReview", "WorkItemTerminal":
				if workItem.record.RunID == "" ||
					event.Seq != workItem.sequence+1 ||
					event.CausationID == "" {
					return nil, nil, nil, ErrInvalidProjectionEvent
				}
				if _, duplicate := outcomes[event.CausationID]; duplicate {
					return nil, nil, nil, ErrInvalidProjectionEvent
				}
				outcomes[event.CausationID] = event
				workItem.sequence = event.Seq
				workItem.lastEventID = event.ID
			case "WorkItemVerificationCommitted":
				var payload runProjectionVerificationPayload
				decidedAt, timeErr := parseRunProjectionTime(
					func() string {
						if decodeRunProjectionPayload(
							event,
							&payload,
						) != nil {
							return ""
						}
						return payload.DecidedAt
					}(),
				)
				if timeErr != nil ||
					event.Seq != workItem.sequence+1 ||
					event.CausationID != workItem.lastEventID ||
					payload.WorkItemID != workItemID ||
					payload.RunID != workItem.record.RunID ||
					payload.TeamInstanceID == "" ||
					!validSHA256Digest(payload.PlanDigest) ||
					payload.LogicalNodeID == "" ||
					payload.AttemptNumber < 1 ||
					payload.ClaimID == "" ||
					payload.ClaimGeneration < 1 ||
					payload.SourceEvidenceID == "" ||
					!validSHA256Digest(payload.SourceEvidenceDigest) ||
					!validSHA256Digest(payload.OutputSummaryDigest) ||
					payload.OutputContractVersion < 1 ||
					!validSHA256Digest(payload.OutputContractDigest) ||
					!validProjectedOutputClassification(
						payload.OutputClassification,
					) ||
					!validSHA256Digest(
						payload.OutputClassificationDigest,
					) ||
					payload.AcceptanceContractVersion < 1 ||
					!validSHA256Digest(
						payload.AcceptanceContractDigest,
					) ||
					(payload.Risk != "low" &&
						payload.Risk != "medium" &&
						payload.Risk != "high") ||
					!validSHA256Digest(
						payload.DeterministicResultDigest,
					) ||
					(payload.AcceptanceDecisionKind != "accepted" &&
						payload.AcceptanceDecisionKind != "rejected") ||
					!validSHA256Digest(
						payload.AcceptanceDecisionDigest,
					) ||
					event.ID != projectionDeterministicEventID(
						"WorkItemVerificationCommitted",
						payload.TeamInstanceID,
						payload.PlanDigest,
						payload.LogicalNodeID,
						fmt.Sprint(payload.AttemptNumber),
						payload.WorkItemID,
						payload.RunID,
						fmt.Sprint(payload.ClaimGeneration),
						payload.SourceEvidenceID,
						payload.SourceEvidenceDigest,
						payload.AcceptanceContractDigest,
						payload.DeterministicResultDigest,
						payload.VerifierCandidateDigest,
						payload.AcceptanceDecisionDigest,
					) {
					return nil, nil, nil, ErrInvalidProjectionEvent
				}
				if payload.VerifierRequired {
					if payload.Risk == "low" ||
						payload.VerifierWorkItemID == "" ||
						payload.VerifierRunID == "" ||
						payload.VerifierClaimID == "" ||
						payload.VerifierClaimGeneration < 1 ||
						payload.VerifierRuntimeInstanceID == "" ||
						payload.VerifierAgentInstanceID == "" ||
						payload.VerifierGrantID == "" ||
						payload.VerifierEvidenceID == "" ||
						!validSHA256Digest(
							payload.VerifierEvidenceDigest,
						) ||
						!validSHA256Digest(
							payload.VerifierOutputSummaryDigest,
						) ||
						(payload.VerifierCandidateKind != "accepted" &&
							payload.VerifierCandidateKind !=
								"rejected") ||
						payload.VerifierReasonCode == "" ||
						!validSHA256Digest(
							payload.VerifierCandidateDigest,
						) {
						return nil, nil, nil, ErrInvalidProjectionEvent
					}
				} else if payload.Risk != "low" ||
					payload.VerifierWorkItemID != "" ||
					payload.VerifierRunID != "" ||
					payload.VerifierClaimID != "" ||
					payload.VerifierClaimGeneration != 0 ||
					payload.VerifierRuntimeInstanceID != "" ||
					payload.VerifierAgentInstanceID != "" ||
					payload.VerifierGrantID != "" ||
					payload.VerifierEvidenceID != "" ||
					payload.VerifierEvidenceDigest != "" ||
					payload.VerifierOutputSummaryDigest != "" ||
					payload.VerifierCandidateKind != "" ||
					payload.VerifierReasonCode != "" ||
					payload.VerifierCandidateDigest != "" {
					return nil, nil, nil, ErrInvalidProjectionEvent
				}
				workItem.record.VerificationStatus = "committed"
				workItem.record.TeamInstanceID =
					payload.TeamInstanceID
				workItem.record.PlanDigest = payload.PlanDigest
				workItem.record.LogicalNodeID = payload.LogicalNodeID
				workItem.record.AttemptNumber = payload.AttemptNumber
				workItem.record.SourceEvidenceID =
					payload.SourceEvidenceID
				workItem.record.SourceEvidenceDigest =
					payload.SourceEvidenceDigest
				workItem.record.OutputSummaryDigest =
					payload.OutputSummaryDigest
				workItem.record.AcceptanceContractVersion =
					payload.AcceptanceContractVersion
				workItem.record.AcceptanceContractDigest =
					payload.AcceptanceContractDigest
				workItem.record.AcceptanceRisk = payload.Risk
				workItem.record.DeterministicResultDigest =
					payload.DeterministicResultDigest
				workItem.record.VerifierRequired =
					payload.VerifierRequired
				workItem.record.VerifierWorkItemID =
					payload.VerifierWorkItemID
				workItem.record.VerifierRunID = payload.VerifierRunID
				workItem.record.VerifierAgentInstanceID =
					payload.VerifierAgentInstanceID
				workItem.record.VerifierRuntimeInstanceID =
					payload.VerifierRuntimeInstanceID
				workItem.record.VerifierEvidenceID =
					payload.VerifierEvidenceID
				workItem.record.VerifierEvidenceDigest =
					payload.VerifierEvidenceDigest
				workItem.record.VerifierCandidateDigest =
					payload.VerifierCandidateDigest
				workItem.record.AcceptanceDecisionKind =
					payload.AcceptanceDecisionKind
				workItem.record.AcceptanceDecisionDigest =
					payload.AcceptanceDecisionDigest
				workItem.record.AcceptanceDecisionReason =
					payload.VerifierReasonCode
				workItem.record.AcceptanceDecisionTime = decidedAt
				workItem.lastEventID = event.ID
				workItem.sequence = event.Seq
			case "WorkItemDone", "WorkItemRejected":
				var payload runProjectionAcceptanceOutcomePayload
				if decodeRunProjectionPayload(event, &payload) != nil ||
					workItem.record.VerificationStatus != "committed" ||
					event.Seq != workItem.sequence+1 ||
					event.CausationID != workItem.lastEventID ||
					payload.WorkItemID != workItemID ||
					payload.RunID != workItem.record.RunID ||
					payload.ClaimGeneration < 1 ||
					payload.VerificationEventID !=
						workItem.lastEventID ||
					payload.AcceptanceDecisionDigest !=
						workItem.record.AcceptanceDecisionDigest ||
					event.ID != projectionAcceptanceOutcomeEventID(
						event.Type,
						payload,
					) {
					return nil, nil, nil, ErrInvalidProjectionEvent
				}
				switch event.Type {
				case "WorkItemDone":
					if payload.Status != "done" ||
						workItem.record.AcceptanceDecisionKind !=
							"accepted" ||
						payload.SourceEvidenceID !=
							workItem.record.SourceEvidenceID ||
						payload.SourceEvidenceDigest !=
							workItem.record.SourceEvidenceDigest ||
						payload.VerifierEvidenceID !=
							workItem.record.VerifierEvidenceID ||
						payload.VerifierEvidenceDigest !=
							workItem.record.VerifierEvidenceDigest ||
						payload.RecoveryTrigger != "" ||
						payload.RecoveryPolicyVersion != 0 ||
						payload.RecoveryPolicyDigest != "" {
						return nil, nil, nil, ErrInvalidProjectionEvent
					}
				case "WorkItemRejected":
					if payload.Status != "rejected" ||
						workItem.record.AcceptanceDecisionKind !=
							"rejected" ||
						payload.SourceEvidenceID != "" ||
						payload.SourceEvidenceDigest != "" ||
						payload.VerifierEvidenceID != "" ||
						payload.VerifierEvidenceDigest != "" ||
						payload.RecoveryTrigger !=
							"verification_rejected" ||
						payload.RecoveryPolicyVersion < 1 ||
						!validSHA256Digest(
							payload.RecoveryPolicyDigest,
						) ||
						payload.AttemptNumber !=
							workItem.record.AttemptNumber ||
						payload.MaxAttempts <
							payload.AttemptNumber ||
						payload.CreditsBefore < 0 {
						return nil, nil, nil, ErrInvalidProjectionEvent
					}
					workItem.record.RecoveryTrigger =
						payload.RecoveryTrigger
					workItem.record.RecoveryPolicyVersion =
						payload.RecoveryPolicyVersion
					workItem.record.RecoveryPolicyDigest =
						payload.RecoveryPolicyDigest
				}
				workItem.record.Status = payload.Status
				workItem.lastEventID = event.ID
				workItem.sequence = event.Seq
			default:
				return nil, nil, nil, ErrInvalidProjectionEvent
			}
		}
		if workItem.record.RunID == "" {
			return nil, nil, nil, ErrInvalidProjectionEvent
		}
		workItems[workItemID] = workItem
	}
	return workItems, runs, outcomes, nil
}

func indexRunProjectionProviderAccountPolicies(
	ctx context.Context,
	events []journal.Event,
) (map[string][]work.ProviderAccountPolicy, error) {
	byStream := make(map[string][]journal.Event)
	seen := make(map[string]struct{})
	for _, event := range events {
		if !strings.HasPrefix(event.StreamID, "provider-account-policy/") {
			continue
		}
		if _, duplicate := seen[event.ID]; duplicate {
			continue
		}
		seen[event.ID] = struct{}{}
		byStream[event.StreamID] = append(byStream[event.StreamID], event)
	}
	history := make(map[string][]work.ProviderAccountPolicy)
	for streamID, streamEvents := range byStream {
		sort.SliceStable(streamEvents, func(i, j int) bool {
			return streamEvents[i].Seq < streamEvents[j].Seq
		})
		accountID := strings.TrimPrefix(streamID, "provider-account-policy/")
		previousEventID := ""
		for index, event := range streamEvents {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			policy, _, err := work.DecodeProviderAccountPolicyConfiguredEvent(
				event, previousEventID,
			)
			if err != nil || event.Seq != int64(index+1) ||
				policy.ProviderAccountID() != accountID ||
				(len(history[accountID]) > 0 && policy.ConfiguredAt().Before(
					history[accountID][len(history[accountID])-1].ConfiguredAt(),
				)) {
				return nil, ErrInvalidProjectionEvent
			}
			history[accountID] = append(history[accountID], policy)
			previousEventID = event.ID
		}
	}
	return history, nil
}

func indexRunProjectionProviderModelRateCards(
	ctx context.Context,
	events []journal.Event,
) (map[string][]work.ProviderModelRateCard, error) {
	byStream := make(map[string][]journal.Event)
	seen := make(map[string]struct{})
	for _, event := range events {
		if event.Type != "ProviderModelRateCardConfigured" {
			continue
		}
		if _, duplicate := seen[event.ID]; duplicate {
			continue
		}
		seen[event.ID] = struct{}{}
		byStream[event.StreamID] = append(byStream[event.StreamID], event)
	}
	history := make(map[string][]work.ProviderModelRateCard)
	for streamID, streamEvents := range byStream {
		sort.SliceStable(streamEvents, func(i, j int) bool {
			return streamEvents[i].Seq < streamEvents[j].Seq
		})
		previousEventID := ""
		for index, event := range streamEvents {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			rateCard, _, err := work.DecodeProviderModelRateCardConfiguredEvent(
				event, previousEventID,
			)
			if err != nil || event.Seq != int64(index+1) ||
				(len(history[streamID]) > 0 && !rateCard.ConfiguredAt().After(
					history[streamID][len(history[streamID])-1].ConfiguredAt(),
				)) {
				return nil, ErrInvalidProjectionEvent
			}
			history[streamID] = append(history[streamID], rateCard)
			previousEventID = event.ID
		}
	}
	return history, nil
}

func runProjectionProviderModelRateCardAt(
	history map[string][]work.ProviderModelRateCard,
	providerID, accountID, modelID string,
	at time.Time,
) (work.ProviderModelRateCard, bool) {
	streamID, err := work.ProviderModelRateCardStreamID(providerID, accountID, modelID)
	if err != nil {
		return work.ProviderModelRateCard{}, false
	}
	cards := history[streamID]
	for index := len(cards) - 1; index >= 0; index-- {
		if !cards[index].ConfiguredAt().After(at) {
			return cards[index], true
		}
	}
	return work.ProviderModelRateCard{}, false
}

func runProjectionProviderAccountPolicyAt(
	history map[string][]work.ProviderAccountPolicy,
	providerID string,
	providerAccountID string,
	at time.Time,
) (work.ProviderAccountPolicy, bool) {
	policies := history[providerAccountID]
	for index := len(policies) - 1; index >= 0; index-- {
		policy := policies[index]
		if policy.ProviderID() == providerID && !policy.ConfiguredAt().After(at) {
			return policy, true
		}
	}
	return work.ProviderAccountPolicy{}, false
}

func runProjectionProviderAccountPolicyRevision(
	history map[string][]work.ProviderAccountPolicy,
	providerID string,
	providerAccountID string,
	revision int64,
	digest string,
) (work.ProviderAccountPolicy, bool) {
	for _, policy := range history[providerAccountID] {
		if policy.ProviderID() == providerID && policy.Revision() == revision &&
			policy.Digest() == digest {
			return policy, true
		}
	}
	return work.ProviderAccountPolicy{}, false
}

func freezeProjectedRunProviderAccountPolicy(
	run *Run,
	policy work.ProviderAccountPolicy,
) {
	run.ProviderAccountPolicyVersion = policy.Version()
	run.ProviderAccountPolicyRevision = policy.Revision()
	run.ProviderAccountPolicyDigest = policy.Digest()
	run.ProviderAccountTrustDomain = policy.TrustDomain()
	run.ProviderAccountRetentionMode = policy.RetentionMode()
	run.ProviderAccountDataRegion = policy.DataRegion()
}

func projectedRunProviderAccountPolicyMatches(
	run Run,
	policy work.ProviderAccountPolicy,
) bool {
	return run.ProviderAccountPolicyVersion == policy.Version() &&
		run.ProviderAccountPolicyRevision == policy.Revision() &&
		run.ProviderAccountPolicyDigest == policy.Digest() &&
		run.ProviderAccountTrustDomain == policy.TrustDomain() &&
		run.ProviderAccountRetentionMode == policy.RetentionMode() &&
		run.ProviderAccountDataRegion == policy.DataRegion()
}

func runProjectionProviderAccountBindingForRun(
	run Run,
	claimID string,
	claimGeneration int64,
	runtimeInstanceID string,
	policy work.ProviderAccountPolicy,
) (runProjectionProviderAccountBinding, error) {
	if !run.ExecutionBindingAvailable || !policy.Valid() ||
		run.ExecutionBinding.BindingDigest == "" ||
		run.ExecutionBinding.ProviderID != policy.ProviderID() ||
		run.ExecutionBinding.ProviderAccountID != policy.ProviderAccountID() ||
		run.ExecutionBinding.RuntimeInstanceID != runtimeInstanceID {
		return runProjectionProviderAccountBinding{}, ErrInvalidProjectionEvent
	}
	budget := int64(0)
	if run.ExecutionBinding.Budget != nil {
		budget = *run.ExecutionBinding.Budget
	}
	if budget < 0 {
		return runProjectionProviderAccountBinding{}, ErrInvalidProjectionEvent
	}
	return runProjectionProviderAccountBinding{
		runProjectionBinding: runProjectionBinding{
			workItemID: run.WorkItemID, runID: run.ID, claimID: claimID,
			claimGeneration:   claimGeneration,
			runtimeInstanceID: runtimeInstanceID,
			agentInstanceID:   run.AgentInstanceID,
		},
		providerID: policy.ProviderID(), providerAccountID: policy.ProviderAccountID(),
		policyRevision: policy.Revision(), policyDigest: policy.Digest(),
		executionBindingDigest: run.ExecutionBinding.BindingDigest,
		assignedBudgetUnits:    budget,
	}, nil
}

func replayRunProjectionRuns(
	ctx context.Context,
	events []journal.Event,
	assigned map[string]runProjectionRun,
	workItems map[string]runProjectionWork,
	statusFacts map[runProjectionStatusReference]runProjectionStatusFact,
	accountPolicies map[string][]work.ProviderAccountPolicy,
	rateCards map[string][]work.ProviderModelRateCard,
	claims map[string]*runProjectionClaim,
	terminals map[string]*runProjectionTerminal,
) (map[string]runProjectionRun, error) {
	byStream := make(map[string][]journal.Event)
	for _, event := range events {
		if strings.HasPrefix(event.StreamID, "run/") {
			byStream[event.StreamID] = append(byStream[event.StreamID], event)
		}
	}
	for streamID, streamEvents := range byStream {
		sort.SliceStable(streamEvents, func(i, j int) bool {
			return streamEvents[i].Seq < streamEvents[j].Seq
		})
		runID := strings.TrimPrefix(streamID, "run/")
		run, ok := assigned[runID]
		if !ok {
			return nil, fmt.Errorf("%w: unassigned run", ErrInvalidProjectionEvent)
		}
		for _, event := range streamEvents {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if !validRunProjectionEnvelope(event) ||
				event.CausationID != run.lastEventID ||
				event.Seq != run.sequence+1 {
				return nil, fmt.Errorf("%w: run envelope %s", ErrInvalidProjectionEvent, event.Type)
			}
			switch event.Type {
			case "RunClaimed":
				var payload runProjectionClaimedPayload
				if err := decodeRunProjectionPayload(event, &payload); err != nil {
					return nil, fmt.Errorf("%w: RunClaimed decode: %v", ErrInvalidProjectionEvent, err)
				}
				if !validRunProjectionClaimPayload(payload, run.record) {
					return nil, fmt.Errorf(
						"%w: RunClaimed fields bindings=%t set=%t manifest=%t root=%t work=%t run=%t claim=%t generation=%t runtime=%t agent=%t lease=%t reference=%t",
						ErrInvalidProjectionEvent,
						payload.AssetRevisionBindings != nil,
						payload.AssetRevisionSetDigest != nil,
						payload.MaterializationManifestDigest != nil,
						payload.MaterializationRootDigest != nil,
						payload.WorkItemID != nil && *payload.WorkItemID == run.record.WorkItemID,
						payload.RunID != nil && *payload.RunID == run.record.ID,
						payload.ClaimID != nil && validRunProjectionCanonicalUUID(*payload.ClaimID),
						payload.ClaimGeneration != nil && *payload.ClaimGeneration == run.record.ClaimGeneration+1,
						payload.RuntimeInstanceID != nil && *payload.RuntimeInstanceID != "",
						payload.AgentInstanceID != nil && *payload.AgentInstanceID == run.record.AgentInstanceID,
						payload.PrepareLeaseExpiresAt != nil,
						payload.reference() != (runProjectionStatusReference{}),
					)
				}
				expiresAt, err := parseRunProjectionTime(*payload.PrepareLeaseExpiresAt)
				if err != nil || run.record.Phase == "running" ||
					run.record.Phase == "terminal" ||
					run.record.Phase == "claimed" &&
						event.EmittedAt.Before(run.prepareLeaseExpiresAt) ||
					run.record.ExecutionBindingAvailable &&
						run.record.ExecutionBinding.RuntimeInstanceID !=
							*payload.RuntimeInstanceID {
					return nil, ErrInvalidProjectionEvent
				}
				reference := payload.runProjectionStatusReferenceFields.reference()
				if !validRunProjectionStatusReference(
					statusFacts, reference, *payload.RuntimeInstanceID, true,
				) {
					return nil, ErrInvalidProjectionEvent
				}
				accountPolicy, accountManaged := runProjectionProviderAccountPolicyAt(
					accountPolicies,
					run.record.ExecutionBinding.ProviderID,
					run.record.ExecutionBinding.ProviderAccountID,
					event.EmittedAt,
				)
				rateCard, rateCardAvailable, rateCardErr := projectedClaimRateCard(payload)
				authoritativeRateCard, authoritativeRateCardAvailable :=
					runProjectionProviderModelRateCardAt(
						rateCards,
						run.record.ExecutionBinding.ProviderID,
						run.record.ExecutionBinding.ProviderAccountID,
						run.record.ExecutionBinding.ModelID,
						event.EmittedAt,
					)
				if rateCardErr != nil || payload.ClaimContractVersion != nil &&
					authoritativeRateCardAvailable != rateCardAvailable ||
					rateCardAvailable && authoritativeRateCard != rateCard ||
					rateCardAvailable &&
						(!run.record.ExecutionBindingAvailable ||
							rateCard.ProviderID() != run.record.ExecutionBinding.ProviderID ||
							rateCard.ProviderAccountID() != run.record.ExecutionBinding.ProviderAccountID ||
							rateCard.ModelID() != run.record.ExecutionBinding.ModelID ||
							rateCard.ConfiguredAt().After(event.EmittedAt)) {
					return nil, ErrInvalidProjectionEvent
				}
				var accountBinding runProjectionProviderAccountBinding
				if accountManaged {
					var bindingErr error
					accountBinding, bindingErr = runProjectionProviderAccountBindingForRun(
						run.record, *payload.ClaimID, *payload.ClaimGeneration,
						*payload.RuntimeInstanceID, accountPolicy,
					)
					if bindingErr != nil {
						return nil, bindingErr
					}
				}
				run.record.ProviderModelRateCardAvailable = rateCardAvailable
				run.record.ProviderModelRateCard = ProjectedProviderModelRateCard{}
				if rateCardAvailable {
					run.record.ProviderModelRateCard = projectedProviderModelRateCard(rateCard)
				}
				var previousAccountBinding runProjectionProviderAccountBinding
				needsOldAccountRelease := run.record.ProviderAccountPolicyAvailable
				if needsOldAccountRelease {
					previousPolicy, found := runProjectionProviderAccountPolicyRevision(
						accountPolicies,
						run.record.ExecutionBinding.ProviderID,
						run.record.ExecutionBinding.ProviderAccountID,
						run.record.ProviderAccountPolicyRevision,
						run.record.ProviderAccountPolicyDigest,
					)
					if !found {
						return nil, ErrInvalidProjectionEvent
					}
					previousAccountBinding, err =
						runProjectionProviderAccountBindingForRun(
							run.record, run.record.ClaimID,
							run.record.ClaimGeneration,
							run.record.RuntimeInstanceID, previousPolicy,
						)
					if err != nil || previousAccountBinding.assignedBudgetUnits !=
						run.record.ProviderAccountAssignedBudgetUnits {
						return nil, ErrInvalidProjectionEvent
					}
				}
				previousBinding := bindingFromRun(run.record)
				previousGeneration := run.record.ClaimGeneration
				run.record.Phase = "claimed"
				run.record.ClaimID = *payload.ClaimID
				run.record.ClaimGeneration = *payload.ClaimGeneration
				run.record.RuntimeInstanceID = *payload.RuntimeInstanceID
				run.record.AgentInstanceID = *payload.AgentInstanceID
				run.record.PrepareLeaseExpiresAt = expiresAt
				if accountManaged {
					run.record.ProviderAccountPolicyAvailable = true
					freezeProjectedRunProviderAccountPolicy(
						&run.record, accountPolicy,
					)
					run.record.ProviderAccountAssignedBudgetUnits =
						accountBinding.assignedBudgetUnits
				}
				lineagePresent := payload.AssetRevisionBindings != nil ||
					payload.AssetRevisionSetDigest != nil ||
					payload.MaterializationManifestDigest != nil ||
					payload.MaterializationRootDigest != nil
				if lineagePresent {
					run.record.AssetLineageAvailable = len(*payload.AssetRevisionBindings) > 0
					run.record.AssetRevisionBindings = projectedRunAssetBindings(
						*payload.AssetRevisionBindings,
					)
					run.record.AssetRevisionSetDigest = *payload.AssetRevisionSetDigest
					run.record.MaterializationManifestDigest = *payload.MaterializationManifestDigest
					run.record.MaterializationRootDigest = *payload.MaterializationRootDigest
				} else {
					run.record.AssetLineageAvailable = false
					run.record.AssetRevisionBindings = []ProjectedAssetRevisionBinding{}
					run.record.AssetRevisionSetDigest = ""
					run.record.MaterializationManifestDigest = ""
					run.record.MaterializationRootDigest = ""
				}
				run.prepareLeaseExpiresAt = expiresAt
				run.lastEventID = event.ID
				run.sequence = event.Seq
				claims[event.ID] = &runProjectionClaim{
					binding:                bindingFromRun(run.record),
					previousBinding:        previousBinding,
					statusReference:        reference,
					needsOldRelease:        previousGeneration > 0,
					accountManaged:         accountManaged,
					accountBinding:         accountBinding,
					needsOldAccountRelease: needsOldAccountRelease,
					previousAccountBinding: previousAccountBinding,
				}
			case "RunPrepareLeaseExtended":
				var payload runProjectionLeasePayload
				if err := decodeRunProjectionPayload(event, &payload); err != nil ||
					!validRunProjectionGeneration(
						payload.WorkItemID, payload.RunID, payload.ClaimID,
						payload.ClaimGeneration, payload.RuntimeInstanceID,
						payload.AgentInstanceID, run.record,
					) ||
					payload.PreviousLeaseExpiresAt == nil ||
					payload.PrepareLeaseExpiresAt == nil ||
					run.record.Phase != "claimed" {
					return nil, ErrInvalidProjectionEvent
				}
				previous, err := parseRunProjectionTime(*payload.PreviousLeaseExpiresAt)
				if err != nil || !previous.Equal(run.prepareLeaseExpiresAt) {
					return nil, ErrInvalidProjectionEvent
				}
				next, err := parseRunProjectionTime(*payload.PrepareLeaseExpiresAt)
				if err != nil || !next.After(previous) ||
					!event.EmittedAt.Before(previous) {
					return nil, ErrInvalidProjectionEvent
				}
				run.record.PrepareLeaseExpiresAt = next
				run.prepareLeaseExpiresAt = next
				run.lastEventID = event.ID
				run.sequence = event.Seq
			case "RunStarted":
				var payload runProjectionGenerationPayload
				if err := decodeRunProjectionPayload(event, &payload); err != nil ||
					!validRunProjectionGenerationPayload(payload, run.record) ||
					run.record.Phase != "claimed" ||
					!event.EmittedAt.Before(run.prepareLeaseExpiresAt) {
					return nil, ErrInvalidProjectionEvent
				}
				reference := payload.runProjectionStatusReferenceFields.reference()
				if !validRunProjectionStatusReference(
					statusFacts, reference, run.record.RuntimeInstanceID, true,
				) {
					return nil, ErrInvalidProjectionEvent
				}
				run.record.Phase = "running"
				run.lastEventID = event.ID
				run.sequence = event.Seq
				workItem := workItems[run.record.WorkItemID]
				if workItem.record.Status != "done" &&
					workItem.record.Status != "rejected" {
					workItem.record.Status = "running"
				}
				workItems[workItem.record.ID] = workItem
			case "RunTerminalCommitted":
				var payload runProjectionTerminalPayload
				if err := decodeRunProjectionPayload(event, &payload); err != nil ||
					!validRunProjectionTerminalPayload(payload, run.record) ||
					(run.record.Phase != "claimed" &&
						run.record.Phase != "running") {
					return nil, ErrInvalidProjectionEvent
				}
				reference := payload.runProjectionStatusReferenceFields.reference()
				if !validRunProjectionStatusReference(
					statusFacts, reference, run.record.RuntimeInstanceID, false,
				) {
					return nil, ErrInvalidProjectionEvent
				}
				var accountBinding runProjectionProviderAccountBinding
				accountManaged := run.record.ProviderAccountPolicyAvailable
				if accountManaged {
					policy, found := runProjectionProviderAccountPolicyRevision(
						accountPolicies,
						run.record.ExecutionBinding.ProviderID,
						run.record.ExecutionBinding.ProviderAccountID,
						run.record.ProviderAccountPolicyRevision,
						run.record.ProviderAccountPolicyDigest,
					)
					if !found {
						return nil, ErrInvalidProjectionEvent
					}
					var bindingErr error
					accountBinding, bindingErr = runProjectionProviderAccountBindingForRun(
						run.record, run.record.ClaimID,
						run.record.ClaimGeneration,
						run.record.RuntimeInstanceID, policy,
					)
					if bindingErr != nil || accountBinding.assignedBudgetUnits !=
						run.record.ProviderAccountAssignedBudgetUnits {
						return nil, ErrInvalidProjectionEvent
					}
				}
				run.record.Phase = "terminal"
				run.record.TerminalStatus = *payload.Status
				run.record.TerminalReason = *payload.Reason
				run.record.AccountingAvailable = payload.Accounting != nil
				if payload.Accounting != nil {
					run.record.Accounting = RunAccounting{
						UsageObserved:    payload.Accounting.UsageObserved,
						InputTokens:      payload.Accounting.InputTokens,
						OutputTokens:     payload.Accounting.OutputTokens,
						CacheReadTokens:  payload.Accounting.CacheReadTokens,
						CacheWriteTokens: payload.Accounting.CacheWriteTokens,
						TotalTokens:      payload.Accounting.TotalTokens,
						CostObserved:     payload.Accounting.CostObserved,
						CostMicrounits:   payload.Accounting.CostMicrounits,
						CostCurrency:     payload.Accounting.CostCurrency,
						CostSource:       payload.Accounting.projectedCostSource(),
					}
				}
				run.lastEventID = event.ID
				run.sequence = event.Seq
				terminals[event.ID] = &runProjectionTerminal{
					binding:         bindingFromRun(run.record),
					statusReference: reference,
					status:          run.record.TerminalStatus,
					accountManaged:  accountManaged,
					accountBinding:  accountBinding,
				}
			default:
				return nil, ErrInvalidProjectionEvent
			}
		}
		assigned[runID] = run
	}
	return assigned, nil
}

func projectedRunAssetBindings(
	bindings []assets.ExactAssetRevisionBinding,
) []ProjectedAssetRevisionBinding {
	result := make([]ProjectedAssetRevisionBinding, len(bindings))
	for index, binding := range bindings {
		result[index] = ProjectedAssetRevisionBinding{
			AssetKind: string(binding.AssetKind), DefinitionID: binding.DefinitionID,
			RevisionID: binding.RevisionID, SHA256Digest: binding.SHA256Digest,
			SourceScope: string(binding.SourceScope),
		}
	}
	return result
}

func replayRunProjectionCapacity(
	ctx context.Context,
	events []journal.Event,
	snapshot *Snapshot,
	statusFacts map[runProjectionStatusReference]runProjectionStatusFact,
	claims map[string]*runProjectionClaim,
	terminals map[string]*runProjectionTerminal,
) error {
	byStream := make(map[string][]journal.Event)
	for _, event := range events {
		if strings.HasPrefix(event.StreamID, "runtime_capacity:") {
			byStream[event.StreamID] = append(byStream[event.StreamID], event)
		}
	}
	for streamID, streamEvents := range byStream {
		sort.SliceStable(streamEvents, func(i, j int) bool {
			return streamEvents[i].Seq < streamEvents[j].Seq
		})
		runtimeID := strings.TrimPrefix(streamID, "runtime_capacity:")
		runtime, ok := snapshot.RuntimeInstances[runtimeID]
		if !ok || runtime.Capacity <= 0 {
			return ErrInvalidProjectionEvent
		}
		active := make(map[string]runProjectionBinding)
		for _, event := range streamEvents {
			if err := ctx.Err(); err != nil {
				return err
			}
			if !validRunProjectionEnvelope(event) {
				return ErrInvalidProjectionEvent
			}
			var payload runProjectionCapacityPayload
			if err := decodeRunProjectionPayload(event, &payload); err != nil ||
				!validRunProjectionCapacityPayload(payload, runtimeID) {
				return ErrInvalidProjectionEvent
			}
			binding := payload.binding()
			reference := payload.statusReference()
			key := runProjectionCapacityKey(binding)
			switch event.Type {
			case "RuntimeCapacityReserved":
				claim, ok := claims[event.CausationID]
				if !ok || claim.binding != binding ||
					claim.statusReference != reference ||
					!validRunProjectionStatusReference(
						statusFacts, reference, runtimeID, true,
					) {
					return ErrInvalidProjectionEvent
				}
				if _, exists := active[key]; exists {
					return ErrInvalidProjectionEvent
				}
				active[key] = binding
				claim.reserved = true
				statusFact := statusFacts[reference]
				if statusFact.capacity <= 0 ||
					len(active) > statusFact.capacity {
					return ErrInvalidProjectionEvent
				}
			case "RuntimeCapacityReleased":
				current, exists := active[key]
				if !exists || current != binding {
					return ErrInvalidProjectionEvent
				}
				delete(active, key)
				if terminal, ok := terminals[event.CausationID]; ok {
					if terminal.binding != binding ||
						terminal.statusReference != reference ||
						!validRunProjectionStatusReference(
							statusFacts, reference, runtimeID, false,
						) {
						return ErrInvalidProjectionEvent
					}
					terminal.released = true
				} else if claim, ok := claims[event.CausationID]; ok {
					if !claim.needsOldRelease ||
						claim.previousBinding != binding ||
						claim.statusReference != reference ||
						!validRunProjectionStatusReference(
							statusFacts, reference, runtimeID, true,
						) {
						return ErrInvalidProjectionEvent
					}
					claim.oldReleased = true
				} else {
					return ErrInvalidProjectionEvent
				}
			default:
				return ErrInvalidProjectionEvent
			}
		}
	}
	return nil
}

func replayRunProjectionProviderAccountCapacity(
	ctx context.Context,
	events []journal.Event,
	accountPolicies map[string][]work.ProviderAccountPolicy,
	runs map[string]runProjectionRun,
	claims map[string]*runProjectionClaim,
	terminals map[string]*runProjectionTerminal,
) error {
	byStream := make(map[string][]journal.Event)
	for _, event := range events {
		if strings.HasPrefix(event.StreamID, "provider-account-capacity/") {
			byStream[event.StreamID] = append(byStream[event.StreamID], event)
		}
	}
	for streamID, streamEvents := range byStream {
		sort.SliceStable(streamEvents, func(i, j int) bool {
			return streamEvents[i].Seq < streamEvents[j].Seq
		})
		accountID := strings.TrimPrefix(
			streamID, "provider-account-capacity/",
		)
		active := make(map[string]runProjectionProviderAccountBinding)
		starts := make([]runProjectionProviderAccountStart, 0)
		for _, event := range streamEvents {
			if err := ctx.Err(); err != nil {
				return err
			}
			if !validRunProjectionEnvelope(event) {
				return ErrInvalidProjectionEvent
			}
			var payload runProjectionProviderAccountCapacityPayload
			if decodeRunProjectionPayload(event, &payload) != nil {
				return ErrInvalidProjectionEvent
			}
			binding, valid := payload.binding()
			if !valid || binding.providerAccountID != accountID {
				return ErrInvalidProjectionEvent
			}
			policy, found := runProjectionProviderAccountPolicyRevision(
				accountPolicies, binding.providerID, binding.providerAccountID,
				binding.policyRevision, binding.policyDigest,
			)
			if !found {
				return ErrInvalidProjectionEvent
			}
			key := runProjectionCapacityKey(binding.runProjectionBinding)
			switch event.Type {
			case "ProviderAccountCapacityReserved":
				current, currentFound := runProjectionProviderAccountPolicyAt(
					accountPolicies, binding.providerID,
					binding.providerAccountID, event.EmittedAt,
				)
				claim, claimFound := claims[event.CausationID]
				if !currentFound || current.Digest() != policy.Digest() ||
					!claimFound || !claim.accountManaged ||
					claim.accountBinding != binding || active[key].runID != "" ||
					!validRunProjectionProviderAccountAdmission(
						active, starts, policy, binding, event.EmittedAt,
					) {
					return ErrInvalidProjectionEvent
				}
				run, ok := runs[binding.runID]
				if !ok || !run.record.ProviderAccountPolicyAvailable ||
					run.record.ProviderAccountPolicyRevision != binding.policyRevision ||
					run.record.ProviderAccountPolicyDigest != binding.policyDigest ||
					!projectedRunProviderAccountPolicyMatches(
						run.record, policy,
					) ||
					run.record.ProviderAccountAssignedBudgetUnits != binding.assignedBudgetUnits {
					return ErrInvalidProjectionEvent
				}
				active[key] = binding
				starts = append(starts, runProjectionProviderAccountStart{
					reservedAt: event.EmittedAt, binding: binding,
				})
				claim.accountReserved = true
			case "ProviderAccountCapacityReleased":
				current, exists := active[key]
				if !exists || current != binding {
					return ErrInvalidProjectionEvent
				}
				delete(active, key)
				if terminal, ok := terminals[event.CausationID]; ok {
					if !terminal.accountManaged || terminal.accountBinding != binding {
						return ErrInvalidProjectionEvent
					}
					terminal.accountReleased = true
				} else if claim, ok := claims[event.CausationID]; ok {
					if !claim.needsOldAccountRelease ||
						claim.previousAccountBinding != binding {
						return ErrInvalidProjectionEvent
					}
					claim.accountReleased = true
				} else {
					return ErrInvalidProjectionEvent
				}
			default:
				return ErrInvalidProjectionEvent
			}
		}
	}
	return nil
}

func (payload runProjectionProviderAccountCapacityPayload) binding() (
	runProjectionProviderAccountBinding,
	bool,
) {
	if payload.WorkItemID == nil || payload.RunID == nil ||
		payload.ClaimID == nil || payload.ClaimGeneration == nil ||
		payload.RuntimeInstanceID == nil || payload.AgentInstanceID == nil ||
		payload.ProviderID == nil || payload.ProviderAccountID == nil ||
		payload.PolicyRevision == nil || payload.PolicyDigest == nil ||
		payload.ExecutionBindingDigest == nil ||
		payload.AssignedBudgetUnits == nil ||
		*payload.WorkItemID == "" || *payload.RunID == "" ||
		!validRunProjectionCanonicalUUID(*payload.ClaimID) ||
		*payload.ClaimGeneration <= 0 || *payload.RuntimeInstanceID == "" ||
		*payload.AgentInstanceID == "" || *payload.ProviderID == "" ||
		*payload.ProviderAccountID == "" || *payload.PolicyRevision <= 0 ||
		!validSHA256Digest(*payload.PolicyDigest) ||
		!validSHA256Digest(*payload.ExecutionBindingDigest) ||
		*payload.AssignedBudgetUnits < 0 {
		return runProjectionProviderAccountBinding{}, false
	}
	return runProjectionProviderAccountBinding{
		runProjectionBinding: runProjectionBinding{
			workItemID: *payload.WorkItemID, runID: *payload.RunID,
			claimID:           *payload.ClaimID,
			claimGeneration:   *payload.ClaimGeneration,
			runtimeInstanceID: *payload.RuntimeInstanceID,
			agentInstanceID:   *payload.AgentInstanceID,
		},
		providerID:             *payload.ProviderID,
		providerAccountID:      *payload.ProviderAccountID,
		policyRevision:         *payload.PolicyRevision,
		policyDigest:           *payload.PolicyDigest,
		executionBindingDigest: *payload.ExecutionBindingDigest,
		assignedBudgetUnits:    *payload.AssignedBudgetUnits,
	}, true
}

func validRunProjectionProviderAccountAdmission(
	active map[string]runProjectionProviderAccountBinding,
	starts []runProjectionProviderAccountStart,
	policy work.ProviderAccountPolicy,
	binding runProjectionProviderAccountBinding,
	now time.Time,
) bool {
	if len(active) >= policy.MaximumConcurrentAttempts() {
		return false
	}
	assignedBudget := int64(0)
	for _, reservation := range active {
		assignedBudget += reservation.assignedBudgetUnits
	}
	if binding.assignedBudgetUnits >
		policy.MaximumAssignedBudgetUnits()-assignedBudget {
		return false
	}
	windowStart := now.Add(-policy.DispatchWindow())
	dispatchStarts := 0
	for _, start := range starts {
		if !start.reservedAt.Before(windowStart) &&
			!start.reservedAt.After(now) {
			dispatchStarts++
		}
	}
	return dispatchStarts < policy.MaximumDispatchStarts()
}

func validateRunProjectionOutcome(
	event journal.Event,
	terminal *runProjectionTerminal,
	workItem *runProjectionWork,
) error {
	var payload struct {
		WorkItemID      *string `json:"work_item_id"`
		RunID           *string `json:"run_id"`
		ClaimGeneration *int64  `json:"claim_generation"`
		Status          *string `json:"status"`
	}
	if err := decodeRunProjectionPayload(event, &payload); err != nil ||
		payload.WorkItemID == nil || payload.RunID == nil ||
		payload.ClaimGeneration == nil || payload.Status == nil ||
		*payload.WorkItemID != terminal.binding.workItemID ||
		*payload.RunID != terminal.binding.runID ||
		*payload.ClaimGeneration != terminal.binding.claimGeneration ||
		workItem.record.ID != terminal.binding.workItemID {
		return ErrInvalidProjectionEvent
	}
	if terminal.status == "succeeded" {
		if event.Type != "WorkItemReadyForReview" ||
			*payload.Status != "ready_for_review" {
			return ErrInvalidProjectionEvent
		}
		if workItem.record.Status != "done" &&
			workItem.record.Status != "rejected" {
			workItem.record.Status = "ready_for_review"
		}
	} else {
		if event.Type != "WorkItemTerminal" || *payload.Status != terminal.status {
			return ErrInvalidProjectionEvent
		}
		workItem.record.Status = terminal.status
	}
	return nil
}

func decodeRunProjectionPayload(event journal.Event, target any) error {
	if err := rejectDuplicateRuntimeStatusPayloadFields(event.PayloadJSON); err != nil {
		return err
	}
	return decodeExactProjectionPayload(event, target)
}

func validRunProjectionEnvelope(event journal.Event) bool {
	return event.ID != "" &&
		event.StreamID != "" &&
		event.Seq > 0 &&
		event.IdempotencyKey != "" &&
		event.CorrelationID != "" &&
		!event.EmittedAt.IsZero() &&
		event.EmittedAt.Location() == time.UTC
}

func (fields runProjectionStatusReferenceFields) reference() runProjectionStatusReference {
	if fields.RuntimeStatusStreamID == nil ||
		fields.RuntimeStatusSequence == nil ||
		fields.RuntimeStatusEventID == nil {
		return runProjectionStatusReference{}
	}
	return runProjectionStatusReference{
		streamID: *fields.RuntimeStatusStreamID,
		sequence: *fields.RuntimeStatusSequence,
		eventID:  *fields.RuntimeStatusEventID,
	}
}

func validRunProjectionStatusReference(
	facts map[runProjectionStatusReference]runProjectionStatusFact,
	reference runProjectionStatusReference,
	runtimeID string,
	requireOnline bool,
) bool {
	if reference.streamID != "runtime_instance:"+runtimeID ||
		reference.sequence <= 0 ||
		reference.eventID == "" {
		return false
	}
	fact, ok := facts[reference]
	if !ok || fact.runtimeID != runtimeID {
		return false
	}
	return !requireOnline || fact.status == "online"
}

func validRunProjectionClaimPayload(
	payload runProjectionClaimedPayload,
	run Run,
) bool {
	if payload.WorkItemID == nil || payload.RunID == nil ||
		payload.ClaimID == nil || payload.ClaimGeneration == nil ||
		payload.RuntimeInstanceID == nil || payload.AgentInstanceID == nil ||
		payload.PrepareLeaseExpiresAt == nil {
		return false
	}
	if payload.ClaimContractVersion == nil &&
		(payload.RateCardStatus != nil || len(payload.RateCard) > 0) ||
		payload.ClaimContractVersion != nil &&
			(*payload.ClaimContractVersion != 2 || payload.RateCardStatus == nil) {
		return false
	}
	lineagePresent := payload.AssetRevisionBindings != nil ||
		payload.AssetRevisionSetDigest != nil ||
		payload.MaterializationManifestDigest != nil ||
		payload.MaterializationRootDigest != nil
	if lineagePresent {
		if payload.AssetRevisionBindings == nil ||
			payload.AssetRevisionSetDigest == nil ||
			payload.MaterializationManifestDigest == nil ||
			payload.MaterializationRootDigest == nil {
			return false
		}
		if len(*payload.AssetRevisionBindings) == 0 {
			if *payload.AssetRevisionSetDigest != "" ||
				*payload.MaterializationManifestDigest != "" ||
				*payload.MaterializationRootDigest != "" {
				return false
			}
		} else {
			digest, err := assets.CanonicalAssetRevisionSetDigest(*payload.AssetRevisionBindings)
			if err != nil || digest != *payload.AssetRevisionSetDigest ||
				!validSHA256Digest(*payload.MaterializationManifestDigest) ||
				!validSHA256Digest(*payload.MaterializationRootDigest) {
				return false
			}
		}
	}
	return *payload.WorkItemID == run.WorkItemID &&
		*payload.RunID == run.ID &&
		validRunProjectionCanonicalUUID(*payload.ClaimID) &&
		*payload.RuntimeInstanceID != "" &&
		*payload.AgentInstanceID == run.AgentInstanceID &&
		*payload.ClaimGeneration == run.ClaimGeneration+1 &&
		payload.reference() != (runProjectionStatusReference{})
}

func projectedClaimRateCard(
	payload runProjectionClaimedPayload,
) (work.ProviderModelRateCard, bool, error) {
	if payload.ClaimContractVersion == nil {
		return work.ProviderModelRateCard{}, false, nil
	}
	switch *payload.RateCardStatus {
	case "not_configured":
		if len(payload.RateCard) != 0 {
			return work.ProviderModelRateCard{}, false, ErrInvalidProjectionEvent
		}
		return work.ProviderModelRateCard{}, false, nil
	case "configured":
		rateCard, err := work.DecodeFrozenProviderModelRateCard(payload.RateCard)
		if err != nil {
			return work.ProviderModelRateCard{}, false, ErrInvalidProjectionEvent
		}
		return rateCard, true, nil
	default:
		return work.ProviderModelRateCard{}, false, ErrInvalidProjectionEvent
	}
}

func validRunProjectionGenerationPayload(
	payload runProjectionGenerationPayload,
	run Run,
) bool {
	return validRunProjectionGeneration(
		payload.WorkItemID, payload.RunID, payload.ClaimID,
		payload.ClaimGeneration, payload.RuntimeInstanceID,
		payload.AgentInstanceID, run,
	) && payload.reference() != (runProjectionStatusReference{})
}

func validRunProjectionTerminalPayload(
	payload runProjectionTerminalPayload,
	run Run,
) bool {
	if !validRunProjectionGeneration(
		payload.WorkItemID, payload.RunID, payload.ClaimID,
		payload.ClaimGeneration, payload.RuntimeInstanceID,
		payload.AgentInstanceID, run,
	) || payload.Status == nil || payload.Reason == nil ||
		payload.reference() == (runProjectionStatusReference{}) {
		return false
	}
	if payload.Accounting != nil && !validProjectedRunAccounting(
		payload.Accounting.UsageObserved,
		payload.Accounting.InputTokens,
		payload.Accounting.OutputTokens,
		payload.Accounting.CacheReadTokens,
		payload.Accounting.CacheWriteTokens,
		payload.Accounting.TotalTokens,
		payload.Accounting.CostObserved,
		payload.Accounting.CostMicrounits,
		payload.Accounting.CostCurrency,
		payload.Accounting.CostSource,
	) {
		return false
	}
	switch *payload.Status {
	case "succeeded":
		return *payload.Reason == ""
	case "failed", "cancelled":
		return *payload.Reason != ""
	default:
		return false
	}
}

func validProjectedRunAccounting(
	usageObserved bool,
	inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens, totalTokens int64,
	costObserved bool,
	costMicrounits int64,
	costCurrency string,
	costSource json.RawMessage,
) bool {
	source, present, valid := projectedCostSourceValue(costSource)
	if !valid {
		return false
	}
	if usageObserved {
		if inputTokens < 0 || outputTokens < 0 || cacheReadTokens < 0 ||
			cacheWriteTokens < 0 || inputTokens > int64(1<<63-1)-outputTokens ||
			totalTokens != inputTokens+outputTokens {
			return false
		}
	} else if inputTokens != 0 || outputTokens != 0 || cacheReadTokens != 0 ||
		cacheWriteTokens != 0 || totalTokens != 0 {
		return false
	}
	if costObserved {
		if costMicrounits < 0 || len(costCurrency) != 3 {
			return false
		}
		for _, character := range costCurrency {
			if character < 'A' || character > 'Z' {
				return false
			}
		}
		if !present {
			return true
		}
		switch source {
		case work.CostSourceProviderReported, work.CostSourceHarnessReported,
			work.CostSourceRateCardEstimate:
			return true
		default:
			return false
		}
	}
	return costMicrounits == 0 && costCurrency == "" &&
		(!present || source == "")
}

func (accounting runProjectionAccountingPayload) projectedCostSource() string {
	source, present, _ := projectedCostSourceValue(accounting.CostSource)
	if accounting.CostObserved && !present {
		return work.CostSourceLegacyUnspecified
	}
	return source
}

func projectedCostSourceValue(raw json.RawMessage) (string, bool, bool) {
	if len(raw) == 0 {
		return "", false, true
	}
	var source string
	if json.Unmarshal(raw, &source) != nil {
		return "", true, false
	}
	return source, true, true
}

func validRunProjectionGeneration(
	workItemID *string,
	runID *string,
	claimID *string,
	claimGeneration *int64,
	runtimeInstanceID *string,
	agentInstanceID *string,
	run Run,
) bool {
	return workItemID != nil && runID != nil && claimID != nil &&
		claimGeneration != nil && runtimeInstanceID != nil &&
		agentInstanceID != nil &&
		*workItemID == run.WorkItemID &&
		*runID == run.ID &&
		*claimID == run.ClaimID &&
		*claimGeneration == run.ClaimGeneration &&
		*runtimeInstanceID == run.RuntimeInstanceID &&
		*agentInstanceID == run.AgentInstanceID
}

func validRunProjectionCapacityPayload(
	payload runProjectionCapacityPayload,
	runtimeID string,
) bool {
	return payload.WorkItemID != nil && payload.RunID != nil &&
		payload.ClaimID != nil && payload.ClaimGeneration != nil &&
		payload.RuntimeInstanceID != nil && payload.AgentInstanceID != nil &&
		payload.RuntimeStatusStreamID != nil &&
		payload.RuntimeStatusSequence != nil &&
		payload.RuntimeStatusEventID != nil &&
		*payload.WorkItemID != "" && *payload.RunID != "" &&
		*payload.ClaimID != "" && *payload.ClaimGeneration > 0 &&
		*payload.RuntimeInstanceID == runtimeID &&
		*payload.AgentInstanceID != "" &&
		payload.statusReference() != (runProjectionStatusReference{})
}

func (payload runProjectionCapacityPayload) binding() runProjectionBinding {
	return runProjectionBinding{
		workItemID:        *payload.WorkItemID,
		runID:             *payload.RunID,
		claimID:           *payload.ClaimID,
		claimGeneration:   *payload.ClaimGeneration,
		runtimeInstanceID: *payload.RuntimeInstanceID,
		agentInstanceID:   *payload.AgentInstanceID,
	}
}

func (payload runProjectionCapacityPayload) statusReference() runProjectionStatusReference {
	return runProjectionStatusReference{
		streamID: *payload.RuntimeStatusStreamID,
		sequence: *payload.RuntimeStatusSequence,
		eventID:  *payload.RuntimeStatusEventID,
	}
}

func bindingFromRun(run Run) runProjectionBinding {
	return runProjectionBinding{
		workItemID:        run.WorkItemID,
		runID:             run.ID,
		claimID:           run.ClaimID,
		claimGeneration:   run.ClaimGeneration,
		runtimeInstanceID: run.RuntimeInstanceID,
		agentInstanceID:   run.AgentInstanceID,
	}
}

func runProjectionCapacityKey(binding runProjectionBinding) string {
	return fmt.Sprintf("%s/%d", binding.runID, binding.claimGeneration)
}

func parseRunProjectionTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || parsed.IsZero() || parsed.Location() != time.UTC {
		return time.Time{}, ErrInvalidProjectionEvent
	}
	return parsed, nil
}

func validRunProjectionCanonicalUUID(value string) bool {
	if len(value) != 36 ||
		value[8] != '-' || value[13] != '-' ||
		value[18] != '-' || value[23] != '-' ||
		value[14] != '4' ||
		(value[19] != '8' && value[19] != '9' &&
			value[19] != 'a' && value[19] != 'b') {
		return false
	}
	for index, character := range value {
		switch index {
		case 8, 13, 18, 23:
			continue
		}
		if !(character >= '0' && character <= '9') &&
			!(character >= 'a' && character <= 'f') {
			return false
		}
	}
	return true
}
