package work

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/teams"
	"loom-pi-rebuild/internal/verification"
)

const (
	teamRecoveryTriggerOutput               = "output"
	teamRecoveryTriggerVerificationRejected = "verification_rejected"
)

var (
	ErrInvalidTeamExecution         = errors.New("invalid Team execution")
	ErrTeamExecutionConflict        = errors.New("Team execution conflict")
	ErrStaleGlobalReadView          = errors.New("stale global read view")
	ErrTeamExecutionAlreadyTerminal = errors.New("Team execution already terminal")
	ErrInvalidTeamAttempt           = errors.New("invalid Team node attempt")
	ErrTeamAttemptLimit             = errors.New("Team node attempt limit reached")
	ErrInvalidTeamRecovery          = errors.New("invalid Team node recovery")
	ErrTeamAttemptRecoveryRequired  = errors.New("Team node attempt recovery requires human action")
	ErrLegacyAcceptanceUnbound      = errors.New("legacy Team acceptance binding is unavailable")
)

type TeamAttemptSelection struct {
	LogicalNodeID string
	AttemptNumber int
}

type TeamDispatchInput struct {
	Plan                 teams.ExecutionPlan
	ReadyAttempts        []TeamAttemptSelection
	SemanticBindings     []TeamNodeSemanticBinding
	ViewVersion          string
	ExpectedHeads        []journal.StreamHead
	AuthoritativeTime    time.Time
	PrepareLeaseDuration time.Duration
	CorrelationID        string
}

type TeamNodeSemanticBinding struct {
	LogicalNodeID               string
	OutputContractVersion       int
	OutputContractDigest        string
	RecoveryPolicyVersion       int
	RecoveryPolicyDigest        string
	AttemptCredits              int
	PrimaryWorkflowPath         string
	WorkflowFallbackKey         string
	RecoveryApprovalRequired    bool
	AcceptanceContractVersion   int
	AcceptanceContractDigest    string
	AcceptanceRisk              string
	IndependentVerifierRequired bool
	VerifierAgentInstanceID     string
	VerifierRuntimeInstanceID   string
	VerifierWorkflowPath        string
}

type TeamAttemptEvidenceInput struct {
	TeamInstanceID    string
	PlanDigest        string
	LogicalNodeID     string
	AttemptNumber     int
	WorkItemID        string
	RunID             string
	ClaimID           string
	ClaimGeneration   int64
	RuntimeInstanceID string
	AgentInstanceID   string
	Receipt           evidence.AttemptReceipt
	Classification    verification.Classification
	CorrelationID     string
}

type TeamAttemptRebindInput struct {
	TeamInstanceID          string
	PlanDigest              string
	LogicalNodeID           string
	AttemptNumber           int
	WorkItemID              string
	RunID                   string
	PreviousClaimID         string
	PreviousClaimGeneration int64
	ClaimID                 string
	ClaimGeneration         int64
	RuntimeInstanceID       string
	AgentInstanceID         string
	CorrelationID           string
}

type TeamRecoveryInput struct {
	TeamInstanceID      string
	PlanDigest          string
	LogicalNodeID       string
	AttemptNumber       int
	MaxAttempts         int
	AgentInstanceID     string
	RuntimeInstanceID   string
	EvidenceID          string
	EvidenceDigest      string
	OutputSummaryDigest string
	Classification      verification.Classification
	RecoveryPolicy      TeamRecoveryPolicy
	Decision            TeamRecoveryDecision
	CorrelationID       string
}

type TeamRecoveryPolicy interface {
	Valid() bool
	Version() int
	Digest() string
	AttemptCredits() int
	RetryDelay() time.Duration
	RecoveryApprovalRequired() bool
	Decide(TeamRecoveryDecisionRequest) (TeamRecoveryDecision, error)
}

type TeamRecoveryDecisionRequest struct {
	TeamInstanceID           string
	PlanDigest               string
	LogicalNodeID            string
	AttemptNumber            int
	MaxAttempts              int
	AgentInstanceID          string
	RuntimeInstanceID        string
	EvidenceID               string
	EvidenceDigest           string
	OutputSummaryDigest      string
	Classification           verification.Classification
	PriorClassifications     []verification.OutputClassification
	RemainingCredits         int
	FallbackConsumed         bool
	DecisionTime             time.Time
	Trigger                  string
	AcceptanceDecisionDigest string
}

type TeamRecoveryDecision interface {
	Valid() bool
	TeamInstanceID() string
	PlanDigest() string
	LogicalNodeID() string
	AttemptNumber() int
	MaxAttempts() int
	AgentInstanceID() string
	RuntimeInstanceID() string
	EvidenceID() string
	EvidenceDigest() string
	OutputSummaryDigest() string
	ClassificationValue() string
	ClassificationDigest() string
	PriorClassificationValues() []string
	RemainingCredits() int
	FallbackConsumed() bool
	DecisionTime() time.Time
	ActionValue() string
	NextAttemptNumber() int
	NextAgentInstanceID() string
	NextRuntimeInstanceID() string
	WorkflowFallbackKey() string
	CreditsBefore() int
	CreditsAfter() int
	RetryAt() time.Time
	PolicyVersion() int
	PolicyDigest() string
	RecoveryApprovalRequired() bool
	TriggerValue() string
	AcceptanceDecisionDigest() string
	Digest() string
}

type TeamAttemptRecord struct {
	attemptNumber              int
	workItemID                 string
	runID                      string
	claimID                    string
	claimGeneration            int64
	runtimeInstanceID          string
	agentInstanceID            string
	status                     string
	workflowPath               string
	evidenceID                 string
	evidenceDigest             string
	outputContractVersion      int
	outputContractDigest       string
	outputClassification       verification.OutputClassification
	outputClassificationDigest string
	outputSummaryDigest        string
}

type TeamNodeRecord struct {
	logicalNodeID            string
	status                   string
	dependencySatisfied      bool
	currentAttempt           int
	retryAt                  time.Time
	maxAttempts              int
	attempts                 []TeamAttemptRecord
	semanticBinding          TeamNodeSemanticBinding
	acceptanceDecisionDigest string
	acceptanceDecisionTime   time.Time
	recoveryTrigger          string
	recoveryAction           string
	recoveryPolicyVersion    int
	recoveryPolicyDigest     string
	recoveryDecisionDigest   string
	recoveryDecisionTime     time.Time
	creditsBefore            int
	creditsAfter             int
	fallbackConsumed         bool
	priorClassifications     []verification.OutputClassification
}

type TeamExecutionRecord struct {
	teamInstanceID          string
	planDigest              string
	status                  string
	nodes                   []TeamNodeRecord
	streamSequence          int64
	lastEventID             string
	legacySemanticUnbound   bool
	legacyAcceptanceUnbound bool
}

type TeamDispatchedNode struct {
	logicalNode teams.ExecutionNode
	attempt     TeamAttemptRecord
	workItem    WorkItemRecord
	run         RunRecord
}

type TeamDispatchResult struct {
	teamInstanceID string
	planDigest     string
	viewVersion    string
	nodes          []TeamDispatchedNode
}

func (result TeamDispatchResult) TeamInstanceID() string { return result.teamInstanceID }
func (result TeamDispatchResult) PlanDigest() string     { return result.planDigest }
func (result TeamDispatchResult) ViewVersion() string    { return result.viewVersion }
func (result TeamDispatchResult) Nodes() []TeamDispatchedNode {
	return append([]TeamDispatchedNode(nil), result.nodes...)
}
func (node TeamDispatchedNode) LogicalNode() teams.ExecutionNode { return node.logicalNode }
func (node TeamDispatchedNode) Attempt() TeamAttemptRecord       { return node.attempt }
func (node TeamDispatchedNode) WorkItem() WorkItemRecord         { return node.workItem.public() }
func (node TeamDispatchedNode) Run() RunRecord                   { return node.run.public() }

func (record TeamExecutionRecord) TeamInstanceID() string { return record.teamInstanceID }
func (record TeamExecutionRecord) PlanDigest() string     { return record.planDigest }
func (record TeamExecutionRecord) Status() string         { return record.status }
func (record TeamExecutionRecord) LegacyAcceptanceUnbound() bool {
	return record.legacyAcceptanceUnbound
}
func (record TeamExecutionRecord) Nodes() []TeamNodeRecord {
	return cloneTeamNodeRecords(record.nodes)
}
func (record TeamNodeRecord) LogicalNodeID() string     { return record.logicalNodeID }
func (record TeamNodeRecord) Status() string            { return record.status }
func (record TeamNodeRecord) DependencySatisfied() bool { return record.dependencySatisfied }
func (record TeamNodeRecord) CurrentAttempt() int       { return record.currentAttempt }
func (record TeamNodeRecord) RetryAt() time.Time        { return record.retryAt }
func (record TeamNodeRecord) AcceptanceDecisionDigest() string {
	return record.acceptanceDecisionDigest
}
func (record TeamNodeRecord) AcceptanceDecisionTime() time.Time {
	return record.acceptanceDecisionTime
}
func (record TeamNodeRecord) RecoveryTrigger() string      { return record.recoveryTrigger }
func (record TeamNodeRecord) RecoveryPolicyVersion() int   { return record.recoveryPolicyVersion }
func (record TeamNodeRecord) RecoveryPolicyDigest() string { return record.recoveryPolicyDigest }
func (record TeamNodeRecord) Attempts() []TeamAttemptRecord {
	return append([]TeamAttemptRecord(nil), record.attempts...)
}
func (record TeamAttemptRecord) AttemptNumber() int        { return record.attemptNumber }
func (record TeamAttemptRecord) WorkItemID() string        { return record.workItemID }
func (record TeamAttemptRecord) RunID() string             { return record.runID }
func (record TeamAttemptRecord) ClaimID() string           { return record.claimID }
func (record TeamAttemptRecord) ClaimGeneration() int64    { return record.claimGeneration }
func (record TeamAttemptRecord) RuntimeInstanceID() string { return record.runtimeInstanceID }
func (record TeamAttemptRecord) AgentInstanceID() string   { return record.agentInstanceID }
func (record TeamAttemptRecord) Status() string            { return record.status }
func (record TeamAttemptRecord) EvidenceID() string        { return record.evidenceID }
func (record TeamAttemptRecord) EvidenceDigest() string    { return record.evidenceDigest }
func (record TeamAttemptRecord) WorkflowPath() string      { return record.workflowPath }
func (record TeamAttemptRecord) OutputClassification() verification.OutputClassification {
	return record.outputClassification
}
func (record TeamAttemptRecord) OutputClassificationDigest() string {
	return record.outputClassificationDigest
}
func (record TeamAttemptRecord) OutputSummaryDigest() string {
	return record.outputSummaryDigest
}

func (authority *Authority) DispatchTeamReadySet(
	ctx context.Context,
	input TeamDispatchInput,
) (TeamDispatchResult, error) {
	if authority == nil || !authority.runIdentityReady() {
		return TeamDispatchResult{}, ErrRunIdentityIndexRequired
	}
	nodes, selections, err := validateTeamDispatchInput(ctx, input)
	if err != nil {
		return TeamDispatchResult{}, err
	}
	candidateEvents, err := authority.store.ReadStream(
		ctx,
		teamExecutionStream(input.Plan.TeamInstanceID()),
	)
	if err != nil {
		return TeamDispatchResult{}, err
	}
	candidateTeam, err := replayTeamExecution(
		input.Plan.TeamInstanceID(),
		candidateEvents,
	)
	if err != nil {
		return TeamDispatchResult{}, err
	}
	streamIDs := teamDispatchStreams(
		input.Plan,
		nodes,
		selections,
		candidateTeam,
	)
	snapshot, err := authority.store.ReadStreamSet(ctx, streamIDs)
	if err != nil {
		return TeamDispatchResult{}, err
	}
	if !exactTeamHeads(input.ExpectedHeads, snapshot.Heads()) {
		return TeamDispatchResult{}, ErrStaleGlobalReadView
	}
	state, err := replayTeamDispatchState(ctx, snapshot.Events())
	if err != nil {
		return TeamDispatchResult{}, err
	}
	if !state.runIdentityInitialized {
		return TeamDispatchResult{}, ErrRunIdentityIndexRequired
	}
	teamStreamID := teamExecutionStream(input.Plan.TeamInstanceID())
	team, err := replayTeamExecution(
		input.Plan.TeamInstanceID(),
		filterTeamEvents(snapshot.Events(), teamStreamID),
	)
	if err != nil {
		return TeamDispatchResult{}, err
	}
	if team.status != "" {
		if team.legacySemanticUnbound {
			return TeamDispatchResult{}, ErrTeamAttemptRecoveryRequired
		}
		if team.legacyAcceptanceUnbound {
			return TeamDispatchResult{}, ErrLegacyAcceptanceUnbound
		}
		if !exactTeamSemanticBindings(team, input.SemanticBindings) {
			return TeamDispatchResult{}, ErrTeamExecutionConflict
		}
	}
	if team.status != "" && isTerminalTeamStatus(team.status) {
		return TeamDispatchResult{}, ErrTeamExecutionAlreadyTerminal
	}
	now := input.AuthoritativeTime
	teamSequence := snapshotHead(snapshot, teamStreamID).Sequence
	identitySequence := snapshotHead(snapshot, runIdentityStreamID).Sequence
	capacitySequences := make(map[string]int64)
	runtimeActive := make(map[string]int)
	for _, selection := range selections {
		node := nodeByLogicalID(nodes, selection.LogicalNodeID)
		runtimeInstanceID := node.RuntimeInstanceID()
		if nodeRecord := teamNodeByID(&team, selection.LogicalNodeID); nodeRecord != nil {
			if attempt := teamAttemptByNumber(
				nodeRecord,
				selection.AttemptNumber,
			); attempt != nil {
				runtimeInstanceID = attempt.runtimeInstanceID
			}
		}
		if _, exists := capacitySequences[runtimeInstanceID]; exists {
			continue
		}
		runtime := state.runtimes[runtimeInstanceID]
		capacitySequences[runtimeInstanceID] =
			state.heads[runtimeCapacityStream(runtimeInstanceID)]
		runtimeActive[runtimeInstanceID] = len(runtime.active)
	}
	events := make([]journal.Event, 0, 24)
	if team.status == "" {
		teamSequence++
		plannedID := deterministicEventID(
			"TeamExecutionPlanned",
			input.Plan.TeamInstanceID(),
			input.Plan.Digest(),
			canonicalTeamSemanticDigest(input.SemanticBindings),
		)
		events = append(events, newEvent(
			plannedID,
			teamStreamID,
			teamSequence,
			"TeamExecutionPlanned",
			now,
			input.CorrelationID,
			"",
			teamPlanPayload(
				input.Plan,
				input.ViewVersion,
				input.SemanticBindings,
			),
		))
		team = plannedTeamRecord(
			input.Plan,
			input.SemanticBindings,
			teamSequence,
			plannedID,
		)
		for _, selection := range selections {
			node := nodeByLogicalID(nodes, selection.LogicalNodeID)
			if selection.AttemptNumber != 1 {
				return TeamDispatchResult{}, ErrInvalidTeamAttempt
			}
			teamSequence++
			binding := teamSemanticBindingByID(
				input.SemanticBindings,
				node.LogicalNodeID(),
			)
			scheduled := scheduledTeamAttempt(
				input.Plan,
				node,
				1,
				binding.PrimaryWorkflowPath,
			)
			scheduledID := deterministicEventID(
				"TeamNodeAttemptScheduled",
				input.Plan.TeamInstanceID(),
				input.Plan.Digest(),
				node.LogicalNodeID(),
				"1",
			)
			events = append(events, newEvent(
				scheduledID,
				teamStreamID,
				teamSequence,
				"TeamNodeAttemptScheduled",
				now,
				input.CorrelationID,
				plannedID,
				teamAttemptScheduledPayload(node.LogicalNodeID(), scheduled, time.Time{}),
			))
			applyScheduledAttempt(&team, node.LogicalNodeID(), scheduled, time.Time{})
		}
	} else if team.planDigest != input.Plan.Digest() {
		return TeamDispatchResult{}, ErrTeamExecutionConflict
	}
	if team.status != "" {
		for _, selection := range selections {
			node := nodeByLogicalID(nodes, selection.LogicalNodeID)
			nodeRecord := teamNodeByID(&team, selection.LogicalNodeID)
			if nodeRecord == nil {
				return TeamDispatchResult{}, ErrInvalidTeamAttempt
			}
			if nodeRecord.currentAttempt == 0 &&
				nodeRecord.status == "pending" &&
				selection.AttemptNumber == 1 {
				teamSequence++
				scheduled := scheduledTeamAttempt(
					input.Plan,
					node,
					1,
					nodeRecord.semanticBinding.PrimaryWorkflowPath,
				)
				scheduledID := deterministicEventID(
					"TeamNodeAttemptScheduled",
					input.Plan.TeamInstanceID(),
					input.Plan.Digest(),
					node.LogicalNodeID(),
					"1",
				)
				events = append(events, newEvent(
					scheduledID,
					teamStreamID,
					teamSequence,
					"TeamNodeAttemptScheduled",
					now,
					input.CorrelationID,
					team.lastEventID,
					teamAttemptScheduledPayload(
						node.LogicalNodeID(), scheduled, time.Time{},
					),
				))
				applyScheduledAttempt(
					&team, node.LogicalNodeID(), scheduled, time.Time{},
				)
			}
		}
	}

	dispatched := make([]TeamDispatchedNode, 0, len(selections))
	dispatchPayloads := make([]teamDispatchAttemptPayload, 0, len(selections))
	for _, selection := range selections {
		node := nodeByLogicalID(nodes, selection.LogicalNodeID)
		nodeRecord := teamNodeByID(&team, selection.LogicalNodeID)
		if nodeRecord == nil ||
			nodeRecord.currentAttempt != selection.AttemptNumber {
			return TeamDispatchResult{}, ErrInvalidTeamAttempt
		}
		attemptRecord := teamAttemptByNumber(nodeRecord, selection.AttemptNumber)
		if attemptRecord == nil || attemptRecord.status != "scheduled" {
			return TeamDispatchResult{}, ErrInvalidTeamAttempt
		}
		for _, dependencyID := range node.DependsOn() {
			dependency := teamNodeByID(&team, dependencyID)
			if dependency == nil || !dependency.dependencySatisfied {
				return TeamDispatchResult{}, ErrInvalidTeamAttempt
			}
		}
		if !nodeRecord.retryAt.IsZero() && now.Before(nodeRecord.retryAt) {
			return TeamDispatchResult{}, ErrInvalidTeamAttempt
		}
		runtime := state.runtimes[attemptRecord.runtimeInstanceID]
		if runtime.id == "" || runtime.status != "online" ||
			runtime.capacity <= runtimeActive[attemptRecord.runtimeInstanceID] {
			return TeamDispatchResult{}, ErrRuntimeCapacityExhausted
		}
		if !runtime.currentStatusHead(state.heads) {
			return TeamDispatchResult{}, ErrTeamExecutionConflict
		}
		if state.workItems[attemptRecord.workItemID].id != "" ||
			state.runs[attemptRecord.runID].id != "" ||
			state.runIdentities[attemptRecord.runID].runID != "" {
			return TeamDispatchResult{}, ErrTeamExecutionConflict
		}
		claimID, claimErr := authority.newClaimID()
		if claimErr != nil {
			return TeamDispatchResult{}, claimErr
		}
		expiresAt := now.Add(input.PrepareLeaseDuration)
		createdID := deterministicEventID(
			"WorkItemCreated", attemptRecord.workItemID,
			node.Title(), input.CorrelationID,
		)
		assignedID := deterministicEventID(
			"WorkItemAssigned", attemptRecord.workItemID,
			attemptRecord.runID, attemptRecord.agentInstanceID,
			input.CorrelationID,
		)
		events = append(events,
			newEvent(
				createdID, workItemStream(attemptRecord.workItemID), 1,
				"WorkItemCreated", now, input.CorrelationID, "",
				struct {
					WorkItemID string `json:"work_item_id"`
					Title      string `json:"title"`
					Status     string `json:"status"`
				}{attemptRecord.workItemID, node.Title(), "ready"},
			),
			newEvent(
				assignedID, workItemStream(attemptRecord.workItemID), 2,
				"WorkItemAssigned", now, input.CorrelationID, createdID,
				struct {
					WorkItemID      string `json:"work_item_id"`
					RunID           string `json:"run_id"`
					AgentInstanceID string `json:"agent_instance_id"`
					Status          string `json:"status"`
				}{
					attemptRecord.workItemID, attemptRecord.runID,
					attemptRecord.agentInstanceID, "assigned",
				},
			),
		)
		identitySequence++
		identityID := deterministicEventID(
			"WorkRunIdentityReserved", attemptRecord.runID,
			attemptRecord.workItemID, assignedID,
		)
		events = append(events, newEvent(
			identityID, runIdentityStreamID, identitySequence,
			"WorkRunIdentityReserved", now, input.CorrelationID, assignedID,
			runIdentityPayload(runIdentityReservation{
				runID:              attemptRecord.runID,
				workItemID:         attemptRecord.workItemID,
				agentInstanceID:    attemptRecord.agentInstanceID,
				assignmentStreamID: workItemStream(attemptRecord.workItemID),
				assignmentSequence: 2,
				assignmentEventID:  assignedID,
			}),
		))
		claimEventID := deterministicEventID(
			"RunClaimed", attemptRecord.runID, claimID,
			"1", input.CorrelationID,
		)
		statusReference := runtime.statusHead.payload()
		events = append(events, newEvent(
			claimEventID, runStream(attemptRecord.runID), 1,
			"RunClaimed", now, input.CorrelationID, assignedID,
			struct {
				WorkItemID            string `json:"work_item_id"`
				RunID                 string `json:"run_id"`
				ClaimID               string `json:"claim_id"`
				ClaimGeneration       int64  `json:"claim_generation"`
				RuntimeInstanceID     string `json:"runtime_instance_id"`
				AgentInstanceID       string `json:"agent_instance_id"`
				PrepareLeaseExpiresAt string `json:"prepare_lease_expires_at"`
				runtimeStatusReferencePayload
			}{
				attemptRecord.workItemID, attemptRecord.runID, claimID, 1,
				attemptRecord.runtimeInstanceID, attemptRecord.agentInstanceID,
				expiresAt.Format(time.RFC3339Nano), statusReference,
			},
		))
		capacitySequences[attemptRecord.runtimeInstanceID]++
		reserveID := deterministicEventID(
			"RuntimeCapacityReserved", attemptRecord.runID, claimID,
			"1", claimEventID,
		)
		events = append(events, newEvent(
			reserveID,
			runtimeCapacityStream(attemptRecord.runtimeInstanceID),
			capacitySequences[attemptRecord.runtimeInstanceID],
			"RuntimeCapacityReserved",
			now,
			input.CorrelationID,
			claimEventID,
			capacityEventPayload{
				WorkItemID:                    attemptRecord.workItemID,
				RunID:                         attemptRecord.runID,
				ClaimID:                       claimID,
				ClaimGeneration:               1,
				RuntimeInstanceID:             attemptRecord.runtimeInstanceID,
				AgentInstanceID:               attemptRecord.agentInstanceID,
				runtimeStatusReferencePayload: statusReference,
			},
		))
		runtimeActive[attemptRecord.runtimeInstanceID]++
		attemptRecord.claimID = claimID
		attemptRecord.claimGeneration = 1
		attemptRecord.status = "dispatched"
		nodeRecord.status = "running"
		nodeRecord.retryAt = time.Time{}
		workItem := WorkItemRecord{
			id: attemptRecord.workItemID, title: node.Title(), status: "assigned",
			runID:           attemptRecord.runID,
			agentInstanceID: attemptRecord.agentInstanceID,
			lastEventID:     assignedID, streamSequence: 2,
		}
		run := RunRecord{
			id: attemptRecord.runID, workItemID: attemptRecord.workItemID,
			phase: "claimed", claimID: claimID, claimGeneration: 1,
			runtimeInstanceID:     attemptRecord.runtimeInstanceID,
			agentInstanceID:       attemptRecord.agentInstanceID,
			prepareLeaseExpiresAt: expiresAt,
			lastEventID:           claimEventID, streamSequence: 1,
		}
		dispatched = append(dispatched, TeamDispatchedNode{
			logicalNode: node, attempt: *attemptRecord,
			workItem: workItem, run: run,
		})
		dispatchPayloads = append(dispatchPayloads, teamDispatchAttemptPayload{
			LogicalNodeID:     node.LogicalNodeID(),
			AttemptNumber:     selection.AttemptNumber,
			WorkItemID:        attemptRecord.workItemID,
			RunID:             attemptRecord.runID,
			ClaimID:           claimID,
			ClaimGeneration:   1,
			RuntimeInstanceID: attemptRecord.runtimeInstanceID,
			AgentInstanceID:   attemptRecord.agentInstanceID,
		})
	}
	sort.Slice(dispatchPayloads, func(i, j int) bool {
		return dispatchPayloads[i].LogicalNodeID < dispatchPayloads[j].LogicalNodeID
	})
	teamSequence++
	dispatchID := deterministicEventID(
		"TeamReadySetDispatched",
		input.Plan.TeamInstanceID(),
		input.Plan.Digest(),
		input.ViewVersion,
		fmt.Sprint(teamSequence),
	)
	events = append(events, newEvent(
		dispatchID, teamStreamID, teamSequence,
		"TeamReadySetDispatched", now, input.CorrelationID,
		team.lastEventID,
		struct {
			TeamInstanceID string                       `json:"team_instance_id"`
			PlanDigest     string                       `json:"plan_digest"`
			ViewVersion    string                       `json:"view_version"`
			Attempts       []teamDispatchAttemptPayload `json:"attempts"`
		}{
			input.Plan.TeamInstanceID(), input.Plan.Digest(),
			input.ViewVersion, dispatchPayloads,
		},
	))
	expectations := make([]journal.StreamHeadExpectation, 0, len(snapshot.Heads()))
	for _, head := range snapshot.Heads() {
		expectations = append(expectations, journal.StreamHeadExpectation{
			StreamID: head.StreamID, Sequence: head.Sequence,
		})
	}
	if _, err := authority.store.AppendBatchIfStreamHeads(
		ctx,
		expectations,
		events,
	); err != nil {
		if errors.Is(err, journal.ErrStreamHeadConflict) ||
			errors.Is(err, journal.ErrIdempotencyConflict) ||
			errors.Is(err, journal.ErrSequenceConflict) ||
			errors.Is(err, journal.ErrPartialEventBatchConflict) {
			return TeamDispatchResult{}, ErrTeamExecutionConflict
		}
		return TeamDispatchResult{}, mapJournalWriteError(err)
	}
	sort.Slice(dispatched, func(i, j int) bool {
		return dispatched[i].logicalNode.LogicalNodeID() <
			dispatched[j].logicalNode.LogicalNodeID()
	})
	return TeamDispatchResult{
		teamInstanceID: input.Plan.TeamInstanceID(),
		planDigest:     input.Plan.Digest(),
		viewVersion:    input.ViewVersion,
		nodes:          dispatched,
	}, nil
}

func (authority *Authority) TeamExecution(
	ctx context.Context,
	teamInstanceID string,
) (TeamExecutionRecord, error) {
	if authority == nil || ctx == nil || !validOpaqueID(teamInstanceID) {
		return TeamExecutionRecord{}, ErrInvalidTeamExecution
	}
	events, err := authority.store.ReadStream(
		ctx,
		teamExecutionStream(teamInstanceID),
	)
	if err != nil {
		return TeamExecutionRecord{}, err
	}
	record, err := replayTeamExecution(teamInstanceID, events)
	if err != nil {
		return TeamExecutionRecord{}, err
	}
	if record.status == "" {
		return TeamExecutionRecord{}, ErrInvalidTeamExecution
	}
	return cloneTeamExecutionRecord(record), nil
}

func (authority *Authority) RebindTeamAttempt(
	ctx context.Context,
	input TeamAttemptRebindInput,
) (TeamExecutionRecord, error) {
	if err := validateTeamRebindInput(ctx, input); err != nil {
		return TeamExecutionRecord{}, err
	}
	streamIDs := []string{
		teamExecutionStream(input.TeamInstanceID),
		workItemStream(input.WorkItemID),
		runStream(input.RunID),
		runtimeStatusStream(input.RuntimeInstanceID),
		runtimeCapacityStream(input.RuntimeInstanceID),
	}
	snapshot, err := authority.store.ReadStreamSet(ctx, streamIDs)
	if err != nil {
		return TeamExecutionRecord{}, err
	}
	team, err := replayTeamExecution(
		input.TeamInstanceID,
		filterTeamEvents(
			snapshot.Events(),
			teamExecutionStream(input.TeamInstanceID),
		),
	)
	if err != nil ||
		team.status == "" ||
		team.planDigest != input.PlanDigest ||
		isTerminalTeamStatus(team.status) {
		return TeamExecutionRecord{}, ErrTeamExecutionConflict
	}
	node := teamNodeByID(&team, input.LogicalNodeID)
	attempt := teamAttemptByNumber(node, input.AttemptNumber)
	if node == nil ||
		attempt == nil ||
		node.currentAttempt != input.AttemptNumber ||
		node.status != "running" ||
		attempt.status != "dispatched" ||
		attempt.workItemID != input.WorkItemID ||
		attempt.runID != input.RunID ||
		attempt.runtimeInstanceID != input.RuntimeInstanceID ||
		attempt.agentInstanceID != input.AgentInstanceID {
		return TeamExecutionRecord{}, ErrTeamExecutionConflict
	}
	if attempt.claimID == input.ClaimID &&
		attempt.claimGeneration == input.ClaimGeneration {
		if !exactCommittedTeamRebind(snapshot.Events(), input) {
			return TeamExecutionRecord{}, ErrTeamExecutionConflict
		}
		return cloneTeamExecutionRecord(team), nil
	}
	if attempt.claimID != input.PreviousClaimID ||
		attempt.claimGeneration != input.PreviousClaimGeneration {
		return TeamExecutionRecord{}, ErrTeamExecutionConflict
	}
	runState, err := replayAuthorityEventsSelective(ctx, snapshot.Events())
	if err != nil {
		return TeamExecutionRecord{}, ErrTeamExecutionConflict
	}
	run, exists := runState.runs[input.RunID]
	if !exists ||
		run.workItemID != input.WorkItemID ||
		run.phase != "claimed" ||
		run.claimID != input.ClaimID ||
		run.claimGeneration != input.ClaimGeneration ||
		run.runtimeInstanceID != input.RuntimeInstanceID ||
		run.agentInstanceID != input.AgentInstanceID {
		return TeamExecutionRecord{}, ErrTeamExecutionConflict
	}
	previousExpiresAt, currentReference, err := teamRebindRunFacts(
		snapshot.Events(),
		input,
	)
	if err != nil {
		return TeamExecutionRecord{}, err
	}
	now, err := authority.operationTime()
	if err != nil {
		return TeamExecutionRecord{}, err
	}
	if now.Before(previousExpiresAt) {
		return TeamExecutionRecord{}, ErrTeamExecutionConflict
	}
	teamHead := snapshotHead(
		snapshot,
		teamExecutionStream(input.TeamInstanceID),
	)
	eventID := deterministicEventID(
		"TeamNodeAttemptRebound",
		input.TeamInstanceID,
		input.PlanDigest,
		input.LogicalNodeID,
		fmt.Sprint(input.AttemptNumber),
		input.WorkItemID,
		input.RunID,
		input.PreviousClaimID,
		fmt.Sprint(input.PreviousClaimGeneration),
		input.ClaimID,
		fmt.Sprint(input.ClaimGeneration),
		input.RuntimeInstanceID,
		input.AgentInstanceID,
		currentReference.eventID,
	)
	event := newEvent(
		eventID,
		teamExecutionStream(input.TeamInstanceID),
		teamHead.Sequence+1,
		"TeamNodeAttemptRebound",
		now,
		input.CorrelationID,
		team.lastEventID,
		teamAttemptReboundPayload{
			TeamInstanceID:          input.TeamInstanceID,
			PlanDigest:              input.PlanDigest,
			LogicalNodeID:           input.LogicalNodeID,
			AttemptNumber:           input.AttemptNumber,
			WorkItemID:              input.WorkItemID,
			RunID:                   input.RunID,
			PreviousClaimID:         input.PreviousClaimID,
			PreviousClaimGeneration: input.PreviousClaimGeneration,
			ClaimID:                 input.ClaimID,
			ClaimGeneration:         input.ClaimGeneration,
			RuntimeInstanceID:       input.RuntimeInstanceID,
			AgentInstanceID:         input.AgentInstanceID,
			RunStream:               runStream(input.RunID),
			RunSequence:             currentReference.sequence,
			RunEventID:              currentReference.eventID,
		},
	)
	expectations := make([]journal.StreamHeadExpectation, 0, len(snapshot.Heads()))
	for _, head := range snapshot.Heads() {
		expectations = append(expectations, journal.StreamHeadExpectation{
			StreamID: head.StreamID,
			Sequence: head.Sequence,
		})
	}
	if _, err := authority.store.AppendBatchIfStreamHeads(
		ctx,
		expectations,
		[]journal.Event{event},
	); err != nil {
		return TeamExecutionRecord{}, ErrTeamExecutionConflict
	}
	return authority.TeamExecution(ctx, input.TeamInstanceID)
}

func exactCommittedTeamRebind(
	events []journal.Event,
	input TeamAttemptRebindInput,
) bool {
	for _, event := range events {
		if event.StreamID != teamExecutionStream(input.TeamInstanceID) ||
			event.Type != "TeamNodeAttemptRebound" {
			continue
		}
		var payload teamAttemptReboundPayload
		if decodeExactPayload(event.PayloadJSON, &payload) != nil ||
			payload.ClaimID != input.ClaimID ||
			payload.ClaimGeneration != input.ClaimGeneration {
			continue
		}
		return event.CorrelationID == input.CorrelationID &&
			payload.TeamInstanceID == input.TeamInstanceID &&
			payload.PlanDigest == input.PlanDigest &&
			payload.LogicalNodeID == input.LogicalNodeID &&
			payload.AttemptNumber == input.AttemptNumber &&
			payload.WorkItemID == input.WorkItemID &&
			payload.RunID == input.RunID &&
			payload.PreviousClaimID == input.PreviousClaimID &&
			payload.PreviousClaimGeneration ==
				input.PreviousClaimGeneration &&
			payload.RuntimeInstanceID == input.RuntimeInstanceID &&
			payload.AgentInstanceID == input.AgentInstanceID
	}
	return false
}

func (authority *Authority) CommitTeamAttemptEvidence(
	ctx context.Context,
	input TeamAttemptEvidenceInput,
) (TeamExecutionRecord, error) {
	if err := validateTeamEvidenceInput(ctx, input); err != nil {
		return TeamExecutionRecord{}, err
	}
	receipt := input.Receipt
	summary := receipt.OutputSummary()
	classification := input.Classification
	streamIDs := []string{
		teamExecutionStream(input.TeamInstanceID),
		"evidence/" + receipt.EvidenceID(),
		workItemStream(input.WorkItemID),
		runStream(input.RunID),
		runtimeStatusStream(input.RuntimeInstanceID),
		runtimeCapacityStream(input.RuntimeInstanceID),
	}
	snapshot, err := authority.store.ReadStreamSet(ctx, streamIDs)
	if err != nil {
		return TeamExecutionRecord{}, err
	}
	team, err := replayTeamExecution(
		input.TeamInstanceID,
		filterTeamEvents(snapshot.Events(), teamExecutionStream(input.TeamInstanceID)),
	)
	if err != nil || team.status == "" || team.planDigest != input.PlanDigest {
		return TeamExecutionRecord{}, ErrInvalidTeamExecution
	}
	if team.legacySemanticUnbound {
		return TeamExecutionRecord{}, ErrTeamAttemptRecoveryRequired
	}
	node := teamNodeByID(&team, input.LogicalNodeID)
	attempt := teamAttemptByNumber(node, input.AttemptNumber)
	if node == nil || attempt == nil ||
		attempt.workItemID != input.WorkItemID ||
		attempt.runID != input.RunID ||
		attempt.claimID != input.ClaimID ||
		attempt.claimGeneration != input.ClaimGeneration ||
		attempt.runtimeInstanceID != input.RuntimeInstanceID ||
		attempt.agentInstanceID != input.AgentInstanceID {
		return TeamExecutionRecord{}, ErrInvalidTeamAttempt
	}
	if attempt.evidenceID != "" {
		if attempt.evidenceID == receipt.EvidenceID() &&
			attempt.evidenceDigest == receipt.Digest() &&
			attempt.outputClassificationDigest == classification.Digest() &&
			attempt.outputSummaryDigest == summary.Digest() {
			return cloneTeamExecutionRecord(team), nil
		}
		return TeamExecutionRecord{}, ErrTeamExecutionConflict
	}
	if receipt.EvidenceID() != summary.EvidenceID() ||
		receipt.Digest() != summary.EvidenceDigest() ||
		classification.EvidenceID() != receipt.EvidenceID() ||
		classification.EvidenceDigest() != receipt.Digest() ||
		classification.SummaryDigest() != summary.Digest() ||
		classification.OutputContractVersion() !=
			node.semanticBinding.OutputContractVersion ||
		classification.OutputContractDigest() !=
			node.semanticBinding.OutputContractDigest {
		return TeamExecutionRecord{}, ErrTeamExecutionConflict
	}
	exactObservation, err := verification.NewOutputObservation(
		receipt.EvidenceID(),
		receipt.Digest(),
		summary.Digest(),
		summary.AuthorizedFrameCount(),
		summary.OutputFrameCount(),
		summary.OutputPayloadBytes(),
		summary.ResultObserved(),
		summary.TerminalStatus(),
	)
	if err != nil ||
		classification.ObservationDigest() != exactObservation.Digest() {
		return TeamExecutionRecord{}, ErrTeamExecutionConflict
	}
	runState, replayErr := replayAuthorityEventsSelective(
		ctx,
		snapshot.Events(),
	)
	if replayErr != nil {
		return TeamExecutionRecord{}, ErrInvalidTeamAttempt
	}
	run, exists := runState.runs[input.RunID]
	if !exists ||
		run.workItemID != input.WorkItemID ||
		run.claimID != input.ClaimID ||
		run.claimGeneration != input.ClaimGeneration ||
		run.runtimeInstanceID != input.RuntimeInstanceID ||
		run.agentInstanceID != input.AgentInstanceID ||
		run.phase != "terminal" ||
		run.terminalStatus != "succeeded" &&
			run.terminalStatus != "failed" &&
			run.terminalStatus != "cancelled" {
		return TeamExecutionRecord{}, ErrInvalidTeamAttempt
	}
	terminalStatus := run.terminalStatus
	if summary.TerminalStatus() != terminalStatus ||
		classification.TerminalStatus() != terminalStatus {
		return TeamExecutionRecord{}, ErrTeamExecutionConflict
	}
	now, err := authority.operationTime()
	if err != nil {
		return TeamExecutionRecord{}, err
	}
	evidenceHead := snapshotHead(
		snapshot,
		"evidence/"+receipt.EvidenceID(),
	)
	if evidenceHead.Sequence != 0 {
		return TeamExecutionRecord{}, ErrTeamExecutionConflict
	}
	evidenceEventID := deterministicEventID(
		"EvidenceSubmitted", receipt.EvidenceID(), receipt.Digest(),
		input.RunID, fmt.Sprint(input.ClaimGeneration),
	)
	evidenceEvent := newEvent(
		evidenceEventID, "evidence/"+receipt.EvidenceID(), 1,
		"EvidenceSubmitted", now, input.CorrelationID, "",
		struct {
			EvidenceID string `json:"evidence_id"`
			WorkItemID string `json:"work_item_id"`
			Digest     string `json:"digest"`
		}{receipt.EvidenceID(), input.WorkItemID, receipt.Digest()},
	)
	teamHead := snapshotHead(snapshot, teamExecutionStream(input.TeamInstanceID))
	terminalEventID := deterministicEventID(
		"TeamNodeAttemptTerminal", input.TeamInstanceID,
		input.LogicalNodeID, fmt.Sprint(input.AttemptNumber),
		receipt.EvidenceID(), receipt.Digest(),
		classification.Digest(), summary.Digest(),
	)
	terminalEvent := newEvent(
		terminalEventID,
		teamExecutionStream(input.TeamInstanceID),
		teamHead.Sequence+1,
		"TeamNodeAttemptTerminal",
		now,
		input.CorrelationID,
		team.lastEventID,
		teamAttemptTerminalPayload{
			LogicalNodeID:              input.LogicalNodeID,
			AttemptNumber:              input.AttemptNumber,
			WorkItemID:                 input.WorkItemID,
			RunID:                      input.RunID,
			ClaimID:                    input.ClaimID,
			ClaimGeneration:            input.ClaimGeneration,
			RuntimeInstanceID:          input.RuntimeInstanceID,
			AgentInstanceID:            input.AgentInstanceID,
			Status:                     terminalStatus,
			EvidenceID:                 receipt.EvidenceID(),
			EvidenceDigest:             receipt.Digest(),
			OutputContractVersion:      classification.OutputContractVersion(),
			OutputContractDigest:       classification.OutputContractDigest(),
			OutputClassification:       string(classification.Kind()),
			OutputClassificationDigest: classification.Digest(),
			OutputSummaryDigest:        summary.Digest(),
		},
	)
	events := []journal.Event{evidenceEvent, terminalEvent}
	applyAttemptTerminal(&team, input.LogicalNodeID, input.AttemptNumber,
		terminalStatus, receipt.EvidenceID(), receipt.Digest(),
		classification.OutputContractVersion(),
		classification.OutputContractDigest(),
		classification.Kind(),
		classification.Digest(),
		summary.Digest(),
	)
	if teamIsTerminal(team) {
		status, reason := aggregateTeamTerminal(team)
		teamTerminalID := deterministicEventID(
			"TeamExecutionTerminal", input.TeamInstanceID,
			input.PlanDigest, status, reason,
		)
		events = append(events, newEvent(
			teamTerminalID,
			teamExecutionStream(input.TeamInstanceID),
			teamHead.Sequence+2,
			"TeamExecutionTerminal",
			now,
			input.CorrelationID,
			terminalEventID,
			struct {
				TeamInstanceID string `json:"team_instance_id"`
				PlanDigest     string `json:"plan_digest"`
				Status         string `json:"status"`
				Reason         string `json:"reason"`
			}{input.TeamInstanceID, input.PlanDigest, status, reason},
		))
	}
	expectations := make([]journal.StreamHeadExpectation, 0, len(snapshot.Heads()))
	for _, head := range snapshot.Heads() {
		expectations = append(expectations, journal.StreamHeadExpectation{
			StreamID: head.StreamID, Sequence: head.Sequence,
		})
	}
	if _, err := authority.store.AppendBatchIfStreamHeads(ctx, expectations, events); err != nil {
		return TeamExecutionRecord{}, ErrTeamExecutionConflict
	}
	return authority.TeamExecution(ctx, input.TeamInstanceID)
}

func (authority *Authority) ScheduleTeamNodeRecovery(
	ctx context.Context,
	input TeamRecoveryInput,
) (TeamExecutionRecord, error) {
	if err := validateTeamRecoveryInput(ctx, input); err != nil {
		return TeamExecutionRecord{}, err
	}
	streamID := teamExecutionStream(input.TeamInstanceID)
	events, err := authority.store.ReadStream(ctx, streamID)
	if err != nil {
		return TeamExecutionRecord{}, err
	}
	team, err := replayTeamExecution(input.TeamInstanceID, events)
	if err != nil {
		return TeamExecutionRecord{}, ErrTeamExecutionConflict
	}
	if team.status == "" || team.planDigest != input.PlanDigest {
		return TeamExecutionRecord{}, ErrInvalidTeamRecovery
	}
	if team.legacySemanticUnbound {
		return TeamExecutionRecord{}, ErrTeamAttemptRecoveryRequired
	}
	node := teamNodeByID(&team, input.LogicalNodeID)
	attempt := teamAttemptByNumber(node, input.AttemptNumber)
	if node == nil || attempt == nil ||
		input.MaxAttempts != node.maxAttempts ||
		input.AgentInstanceID != attempt.agentInstanceID ||
		input.RuntimeInstanceID != attempt.runtimeInstanceID ||
		input.EvidenceID != attempt.evidenceID ||
		input.EvidenceDigest != attempt.evidenceDigest ||
		input.OutputSummaryDigest != attempt.outputSummaryDigest ||
		input.Classification.Kind() != attempt.outputClassification ||
		input.Classification.Digest() != attempt.outputClassificationDigest ||
		input.RecoveryPolicy.Version() !=
			node.semanticBinding.RecoveryPolicyVersion ||
		input.RecoveryPolicy.Digest() !=
			node.semanticBinding.RecoveryPolicyDigest ||
		input.RecoveryPolicy.AttemptCredits() !=
			node.semanticBinding.AttemptCredits ||
		input.RecoveryPolicy.RecoveryApprovalRequired() !=
			node.semanticBinding.RecoveryApprovalRequired {
		return TeamExecutionRecord{}, ErrInvalidTeamRecovery
	}
	matched, existingConflict, matchErr := matchExistingTeamRecovery(
		events,
		input,
	)
	if matchErr != nil {
		return TeamExecutionRecord{}, matchErr
	}
	if matched {
		return cloneTeamExecutionRecord(team), nil
	}
	if existingConflict {
		return TeamExecutionRecord{}, ErrTeamExecutionConflict
	}
	if isTerminalTeamStatus(team.status) {
		return TeamExecutionRecord{}, ErrTeamExecutionAlreadyTerminal
	}
	if node == nil || attempt == nil ||
		node.currentAttempt != input.AttemptNumber ||
		node.status != "awaiting_recovery" {
		return TeamExecutionRecord{}, ErrInvalidTeamRecovery
	}
	expectedTrigger := teamRecoveryTriggerOutput
	expectedAcceptanceDecisionDigest := ""
	if node.recoveryTrigger != "" {
		expectedTrigger = node.recoveryTrigger
		expectedAcceptanceDecisionDigest = node.acceptanceDecisionDigest
	}
	expectedPrior := teamPriorClassifications(node, input.AttemptNumber)
	expectedCredits := node.semanticBinding.AttemptCredits -
		(input.AttemptNumber - 1)
	if expectedCredits < 0 {
		expectedCredits = 0
	}
	fallbackConsumed := teamFallbackConsumed(node)
	now, err := authority.operationTime()
	if err != nil {
		return TeamExecutionRecord{}, err
	}
	decision, err := input.RecoveryPolicy.Decide(
		TeamRecoveryDecisionRequest{
			TeamInstanceID:           input.TeamInstanceID,
			PlanDigest:               input.PlanDigest,
			LogicalNodeID:            input.LogicalNodeID,
			AttemptNumber:            input.AttemptNumber,
			MaxAttempts:              input.MaxAttempts,
			AgentInstanceID:          input.AgentInstanceID,
			RuntimeInstanceID:        input.RuntimeInstanceID,
			EvidenceID:               input.EvidenceID,
			EvidenceDigest:           input.EvidenceDigest,
			OutputSummaryDigest:      input.OutputSummaryDigest,
			Classification:           input.Classification,
			PriorClassifications:     expectedPrior,
			RemainingCredits:         expectedCredits,
			FallbackConsumed:         fallbackConsumed,
			DecisionTime:             now,
			Trigger:                  expectedTrigger,
			AcceptanceDecisionDigest: expectedAcceptanceDecisionDigest,
		},
	)
	if err != nil || !validExactTeamRecoveryDecision(decision) ||
		!decision.Valid() || !decision.DecisionTime().Equal(now) ||
		!exactTeamRecoveryDecision(
			decision,
			input,
			expectedPrior,
			expectedCredits,
			fallbackConsumed,
			expectedTrigger,
			expectedAcceptanceDecisionDigest,
		) ||
		expectedTrigger == teamRecoveryTriggerVerificationRejected &&
			decision.ActionValue() == "fallback" {
		return TeamExecutionRecord{}, ErrInvalidTeamRecovery
	}
	if decision.ActionValue() == "none" {
		return TeamExecutionRecord{}, ErrTeamAttemptRecoveryRequired
	}
	toAppend, err := buildTeamRecoveryTransaction(team, input, decision)
	if err != nil {
		return TeamExecutionRecord{}, err
	}
	if _, err := authority.store.AppendBatchIfStreamHeads(
		ctx,
		[]journal.StreamHeadExpectation{{
			StreamID: streamID, Sequence: team.streamSequence,
		}},
		toAppend,
	); err != nil {
		return TeamExecutionRecord{}, ErrTeamExecutionConflict
	}
	return authority.TeamExecution(ctx, decision.TeamInstanceID())
}

func buildTeamRecoveryTransaction(
	team TeamExecutionRecord,
	input TeamRecoveryInput,
	decision TeamRecoveryDecision,
) ([]journal.Event, error) {
	team = cloneTeamExecutionRecord(team)
	node := teamNodeByID(&team, input.LogicalNodeID)
	attempt := teamAttemptByNumber(node, input.AttemptNumber)
	if node == nil || attempt == nil ||
		node.currentAttempt != input.AttemptNumber ||
		node.status != "awaiting_recovery" {
		return nil, ErrInvalidTeamRecovery
	}
	expectedPrior := teamPriorClassifications(node, input.AttemptNumber)
	expectedCredits := node.semanticBinding.AttemptCredits -
		(input.AttemptNumber - 1)
	if expectedCredits < 0 {
		expectedCredits = 0
	}
	fallbackConsumed := teamFallbackConsumed(node)
	expectedTrigger := node.recoveryTrigger
	if expectedTrigger == "" {
		expectedTrigger = teamRecoveryTriggerOutput
	}
	if !validExactTeamRecoveryDecision(decision) ||
		!decision.Valid() ||
		!exactTeamRecoveryDecision(
			decision,
			input,
			expectedPrior,
			expectedCredits,
			fallbackConsumed,
			expectedTrigger,
			node.acceptanceDecisionDigest,
		) ||
		expectedTrigger == teamRecoveryTriggerVerificationRejected &&
			decision.ActionValue() == "fallback" {
		return nil, ErrInvalidTeamRecovery
	}
	streamID := teamExecutionStream(input.TeamInstanceID)
	sequence := team.streamSequence + 1
	recoveryID := deterministicEventID(
		"TeamNodeRecoveryRecorded",
		decision.TeamInstanceID(),
		decision.LogicalNodeID(),
		fmt.Sprint(decision.AttemptNumber()),
		decision.Digest(),
	)
	prior := make([]string, len(expectedPrior))
	for index, classification := range expectedPrior {
		prior[index] = string(classification)
	}
	dependencySatisfied := decision.ActionValue() == "degraded"
	recoveryEvent := newEvent(
		recoveryID,
		streamID,
		sequence,
		"TeamNodeRecoveryRecorded",
		decision.DecisionTime(),
		input.CorrelationID,
		team.lastEventID,
		teamRecoveryPayload{
			LogicalNodeID:          decision.LogicalNodeID(),
			AttemptNumber:          decision.AttemptNumber(),
			Action:                 decision.ActionValue(),
			DecisionTime:           decision.DecisionTime().Format(time.RFC3339Nano),
			RetryAt:                formatOptionalUTC(decision.RetryAt()),
			NextAttemptNumber:      decision.NextAttemptNumber(),
			NextAgentInstanceID:    decision.NextAgentInstanceID(),
			NextRuntimeInstanceID:  decision.NextRuntimeInstanceID(),
			WorkflowFallbackKey:    decision.WorkflowFallbackKey(),
			RecoveryPolicyVersion:  decision.PolicyVersion(),
			RecoveryPolicyDigest:   decision.PolicyDigest(),
			RecoveryDecisionDigest: decision.Digest(),
			ClassificationDigest:   decision.ClassificationDigest(),
			PriorClassifications:   prior,
			CreditsBefore:          decision.CreditsBefore(),
			CreditsAfter:           decision.CreditsAfter(),
			FallbackConsumed: fallbackConsumed ||
				decision.ActionValue() == "fallback",
			RecoveryApprovalRequired: decision.RecoveryApprovalRequired(),
			DependencySatisfied:      dependencySatisfied,
			RecoveryTrigger:          decision.TriggerValue(),
			AcceptanceDecisionDigest: decision.AcceptanceDecisionDigest(),
		},
	)
	transaction := []journal.Event{recoveryEvent}
	switch decision.ActionValue() {
	case "retry", "fallback":
		if decision.AttemptNumber() >= node.maxAttempts ||
			decision.NextAttemptNumber() != decision.AttemptNumber()+1 ||
			decision.CreditsAfter() != expectedCredits-1 ||
			decision.NextAgentInstanceID() != attempt.agentInstanceID ||
			decision.NextRuntimeInstanceID() != attempt.runtimeInstanceID ||
			decision.RetryAt().IsZero() {
			return nil, ErrTeamAttemptLimit
		}
		workflowPath := attempt.workflowPath
		if decision.ActionValue() == "fallback" {
			if fallbackConsumed ||
				decision.WorkflowFallbackKey() == "" ||
				decision.WorkflowFallbackKey() !=
					node.semanticBinding.WorkflowFallbackKey {
				return nil, ErrInvalidTeamRecovery
			}
			workflowPath = decision.WorkflowFallbackKey()
		} else if decision.WorkflowFallbackKey() != "" {
			return nil, ErrInvalidTeamRecovery
		}
		nextAttempt := decision.NextAttemptNumber()
		sequence++
		scheduled := TeamAttemptRecord{
			attemptNumber: nextAttempt,
			workItemID: teamAttemptIdentityFromValues(
				"work",
				decision.TeamInstanceID(),
				decision.PlanDigest(),
				decision.LogicalNodeID(),
				nextAttempt,
			),
			runID: teamAttemptIdentityFromValues(
				"run",
				decision.TeamInstanceID(),
				decision.PlanDigest(),
				decision.LogicalNodeID(),
				nextAttempt,
			),
			runtimeInstanceID: decision.NextRuntimeInstanceID(),
			agentInstanceID:   decision.NextAgentInstanceID(),
			workflowPath:      workflowPath,
			status:            "scheduled",
		}
		scheduledID := deterministicEventID(
			"TeamNodeAttemptScheduled",
			decision.TeamInstanceID(),
			decision.PlanDigest(),
			decision.LogicalNodeID(),
			fmt.Sprint(nextAttempt),
			workflowPath,
		)
		transaction = append(transaction, newEvent(
			scheduledID,
			streamID,
			sequence,
			"TeamNodeAttemptScheduled",
			decision.DecisionTime(),
			input.CorrelationID,
			recoveryID,
			teamAttemptScheduledPayload(
				decision.LogicalNodeID(),
				scheduled,
				decision.RetryAt(),
			),
		))
	case "degraded", "blocked", "human_required":
		if !decision.RetryAt().IsZero() ||
			decision.NextAttemptNumber() != 0 ||
			decision.NextAgentInstanceID() != "" ||
			decision.NextRuntimeInstanceID() != "" ||
			decision.WorkflowFallbackKey() != "" ||
			decision.CreditsAfter() != expectedCredits {
			return nil, ErrInvalidTeamRecovery
		}
		node.status = decision.ActionValue()
		node.dependencySatisfied = dependencySatisfied
		if teamIsTerminal(team) {
			status, reason := aggregateTeamTerminal(team)
			sequence++
			terminalID := deterministicEventID(
				"TeamExecutionTerminal",
				decision.TeamInstanceID(),
				decision.PlanDigest(),
				status,
				reason,
			)
			transaction = append(transaction, newEvent(
				terminalID,
				streamID,
				sequence,
				"TeamExecutionTerminal",
				decision.DecisionTime(),
				input.CorrelationID,
				recoveryID,
				struct {
					TeamInstanceID string `json:"team_instance_id"`
					PlanDigest     string `json:"plan_digest"`
					Status         string `json:"status"`
					Reason         string `json:"reason"`
				}{
					decision.TeamInstanceID(),
					decision.PlanDigest(),
					status,
					reason,
				},
			))
		}
	default:
		return nil, ErrInvalidTeamRecovery
	}
	return transaction, nil
}

type teamPlanNodePayload struct {
	LogicalNodeID     string              `json:"logical_node_id"`
	Title             string              `json:"title"`
	AgentInstanceID   string              `json:"agent_instance_id"`
	RuntimeInstanceID string              `json:"runtime_instance_id"`
	Role              teams.ExecutionRole `json:"role"`
	DependsOn         []string            `json:"depends_on"`
	MaxAttempts       int                 `json:"max_attempts"`
}

type teamSemanticBindingPayload struct {
	LogicalNodeID               string  `json:"logical_node_id"`
	OutputContractVersion       int     `json:"output_contract_version"`
	OutputContractDigest        string  `json:"output_contract_digest"`
	RecoveryPolicyVersion       int     `json:"recovery_policy_version"`
	RecoveryPolicyDigest        string  `json:"recovery_policy_digest"`
	AttemptCredits              int     `json:"attempt_credits"`
	PrimaryWorkflowPath         string  `json:"primary_workflow_path"`
	WorkflowFallbackKey         string  `json:"workflow_fallback_key"`
	RecoveryApprovalRequired    bool    `json:"recovery_approval_required"`
	AcceptanceContractVersion   *int    `json:"acceptance_contract_version,omitempty"`
	AcceptanceContractDigest    *string `json:"acceptance_contract_digest,omitempty"`
	AcceptanceRisk              *string `json:"risk,omitempty"`
	IndependentVerifierRequired *bool   `json:"independent_verifier_required,omitempty"`
	VerifierAgentInstanceID     *string `json:"verifier_agent_instance_id,omitempty"`
	VerifierRuntimeInstanceID   *string `json:"verifier_runtime_instance_id,omitempty"`
	VerifierWorkflowPath        *string `json:"verifier_workflow_path,omitempty"`
}

type teamDispatchAttemptPayload struct {
	LogicalNodeID     string `json:"logical_node_id"`
	AttemptNumber     int    `json:"attempt_number"`
	WorkItemID        string `json:"work_item_id"`
	RunID             string `json:"run_id"`
	ClaimID           string `json:"claim_id"`
	ClaimGeneration   int64  `json:"claim_generation"`
	RuntimeInstanceID string `json:"runtime_instance_id"`
	AgentInstanceID   string `json:"agent_instance_id"`
}

type teamAttemptTerminalPayload struct {
	LogicalNodeID              string `json:"logical_node_id"`
	AttemptNumber              int    `json:"attempt_number"`
	WorkItemID                 string `json:"work_item_id"`
	RunID                      string `json:"run_id"`
	ClaimID                    string `json:"claim_id"`
	ClaimGeneration            int64  `json:"claim_generation"`
	RuntimeInstanceID          string `json:"runtime_instance_id"`
	AgentInstanceID            string `json:"agent_instance_id"`
	Status                     string `json:"status"`
	EvidenceID                 string `json:"evidence_id"`
	EvidenceDigest             string `json:"evidence_digest"`
	OutputContractVersion      int    `json:"output_contract_version"`
	OutputContractDigest       string `json:"output_contract_digest"`
	OutputClassification       string `json:"output_classification"`
	OutputClassificationDigest string `json:"output_classification_digest"`
	OutputSummaryDigest        string `json:"output_summary_digest"`
}

type teamAttemptReboundPayload struct {
	TeamInstanceID          string `json:"team_instance_id"`
	PlanDigest              string `json:"plan_digest"`
	LogicalNodeID           string `json:"logical_node_id"`
	AttemptNumber           int    `json:"attempt_number"`
	WorkItemID              string `json:"work_item_id"`
	RunID                   string `json:"run_id"`
	PreviousClaimID         string `json:"previous_claim_id"`
	PreviousClaimGeneration int64  `json:"previous_claim_generation"`
	ClaimID                 string `json:"claim_id"`
	ClaimGeneration         int64  `json:"claim_generation"`
	RuntimeInstanceID       string `json:"runtime_instance_id"`
	AgentInstanceID         string `json:"agent_instance_id"`
	RunStream               string `json:"run_stream"`
	RunSequence             int64  `json:"run_sequence"`
	RunEventID              string `json:"run_event_id"`
}

type teamRecoveryPayload struct {
	LogicalNodeID            string   `json:"logical_node_id"`
	AttemptNumber            int      `json:"attempt_number"`
	Action                   string   `json:"action"`
	DecisionTime             string   `json:"decision_time"`
	RetryAt                  string   `json:"retry_at"`
	NextAttemptNumber        int      `json:"next_attempt_number"`
	NextAgentInstanceID      string   `json:"next_agent_instance_id"`
	NextRuntimeInstanceID    string   `json:"next_runtime_instance_id"`
	WorkflowFallbackKey      string   `json:"workflow_fallback_key"`
	RecoveryPolicyVersion    int      `json:"recovery_policy_version"`
	RecoveryPolicyDigest     string   `json:"recovery_policy_digest"`
	RecoveryDecisionDigest   string   `json:"recovery_decision_digest"`
	ClassificationDigest     string   `json:"classification_digest"`
	PriorClassifications     []string `json:"prior_classifications"`
	CreditsBefore            int      `json:"credits_before"`
	CreditsAfter             int      `json:"credits_after"`
	FallbackConsumed         bool     `json:"fallback_consumed"`
	RecoveryApprovalRequired bool     `json:"recovery_approval_required"`
	DependencySatisfied      bool     `json:"dependency_satisfied"`
	RecoveryTrigger          string   `json:"recovery_trigger"`
	AcceptanceDecisionDigest string   `json:"acceptance_decision_digest"`
}

func validateTeamDispatchInput(
	ctx context.Context,
	input TeamDispatchInput,
) ([]teams.ExecutionNode, []TeamAttemptSelection, error) {
	if ctx == nil {
		return nil, nil, ErrInvalidTeamExecution
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	nodes := input.Plan.Nodes()
	if !validOpaqueID(input.Plan.TeamInstanceID()) ||
		!validSHA256Hex(input.Plan.Digest()) ||
		!validSHA256Hex(input.ViewVersion) ||
		len(nodes) == 0 || len(nodes) > 3 ||
		len(input.ReadyAttempts) == 0 ||
		len(input.ReadyAttempts) > len(nodes) ||
		input.AuthoritativeTime.IsZero() ||
		input.AuthoritativeTime.Location() != time.UTC ||
		input.PrepareLeaseDuration <= 0 ||
		input.PrepareLeaseDuration > maxPrepareLease ||
		!validCanonicalUUID(input.CorrelationID) {
		return nil, nil, ErrInvalidTeamExecution
	}
	rebuiltInputs := make([]teams.ExecutionNodeInput, len(nodes))
	for index, node := range nodes {
		rebuiltInputs[index] = teams.ExecutionNodeInput{
			LogicalNodeID: node.LogicalNodeID(), Title: node.Title(),
			AgentInstanceID:   node.AgentInstanceID(),
			RuntimeInstanceID: node.RuntimeInstanceID(),
			Role:              node.Role(), DependsOn: node.DependsOn(),
			MaxAttempts: node.MaxAttempts(),
		}
	}
	rebuilt, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: input.Plan.TeamInstanceID(),
		Nodes:          rebuiltInputs,
	})
	if err != nil || rebuilt.Digest() != input.Plan.Digest() {
		return nil, nil, ErrInvalidTeamExecution
	}
	if len(input.SemanticBindings) != len(nodes) {
		return nil, nil, ErrInvalidTeamExecution
	}
	for index, binding := range input.SemanticBindings {
		node := nodeByLogicalID(nodes, binding.LogicalNodeID)
		if node.LogicalNodeID() == "" ||
			index > 0 &&
				input.SemanticBindings[index-1].LogicalNodeID >=
					binding.LogicalNodeID ||
			binding.OutputContractVersion < 1 ||
			!validSHA256Hex(binding.OutputContractDigest) ||
			binding.RecoveryPolicyVersion < 1 ||
			!validSHA256Hex(binding.RecoveryPolicyDigest) ||
			binding.AttemptCredits < 0 ||
			binding.AttemptCredits > 2 ||
			binding.AttemptCredits > node.MaxAttempts()-1 ||
			!validOpaqueID(binding.PrimaryWorkflowPath) ||
			binding.WorkflowFallbackKey != "" &&
				(!validOpaqueID(binding.WorkflowFallbackKey) ||
					binding.WorkflowFallbackKey ==
						binding.PrimaryWorkflowPath) ||
			!validTeamAcceptanceBinding(binding, node.AgentInstanceID()) {
			return nil, nil, ErrInvalidTeamExecution
		}
	}
	selections := append([]TeamAttemptSelection(nil), input.ReadyAttempts...)
	sort.Slice(selections, func(i, j int) bool {
		return selections[i].LogicalNodeID < selections[j].LogicalNodeID
	})
	for index, selection := range selections {
		node := nodeByLogicalID(nodes, selection.LogicalNodeID)
		if node.LogicalNodeID() == "" ||
			selection.AttemptNumber < 1 ||
			selection.AttemptNumber > node.MaxAttempts() ||
			index > 0 &&
				selection.LogicalNodeID == selections[index-1].LogicalNodeID {
			return nil, nil, ErrInvalidTeamAttempt
		}
	}
	return nodes, selections, nil
}

func validateTeamEvidenceInput(
	ctx context.Context,
	input TeamAttemptEvidenceInput,
) error {
	if ctx == nil {
		return ErrInvalidTeamExecution
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validOpaqueID(input.TeamInstanceID) ||
		!validSHA256Hex(input.PlanDigest) ||
		!validOpaqueID(input.LogicalNodeID) ||
		input.AttemptNumber < 1 || input.AttemptNumber > 3 ||
		!validOpaqueID(input.WorkItemID) ||
		!validOpaqueID(input.RunID) ||
		!validCanonicalUUID(input.ClaimID) ||
		input.ClaimGeneration <= 0 ||
		!validOpaqueID(input.RuntimeInstanceID) ||
		!validOpaqueID(input.AgentInstanceID) ||
		input.Receipt.EvidenceID() == "" ||
		input.Receipt.Digest() == "" ||
		input.Receipt.OutputSummary().Digest() == "" ||
		!input.Classification.Valid() ||
		!validCanonicalUUID(input.CorrelationID) {
		return ErrInvalidTeamExecution
	}
	return nil
}

func validateTeamRebindInput(
	ctx context.Context,
	input TeamAttemptRebindInput,
) error {
	if ctx == nil {
		return ErrInvalidTeamAttempt
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validOpaqueID(input.TeamInstanceID) ||
		!validSHA256Hex(input.PlanDigest) ||
		!validOpaqueID(input.LogicalNodeID) ||
		input.AttemptNumber < 1 ||
		input.AttemptNumber > 3 ||
		!validOpaqueID(input.WorkItemID) ||
		!validOpaqueID(input.RunID) ||
		!validCanonicalUUID(input.PreviousClaimID) ||
		input.PreviousClaimGeneration <= 0 ||
		!validCanonicalUUID(input.ClaimID) ||
		input.ClaimID == input.PreviousClaimID ||
		input.ClaimGeneration != input.PreviousClaimGeneration+1 ||
		!validOpaqueID(input.RuntimeInstanceID) ||
		!validOpaqueID(input.AgentInstanceID) ||
		!validCanonicalUUID(input.CorrelationID) {
		return ErrInvalidTeamAttempt
	}
	return nil
}

func validateTeamRecoveryInput(ctx context.Context, input TeamRecoveryInput) error {
	if ctx == nil {
		return ErrInvalidTeamRecovery
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validOpaqueID(input.TeamInstanceID) ||
		!validSHA256Hex(input.PlanDigest) ||
		!validOpaqueID(input.LogicalNodeID) ||
		input.AttemptNumber < 1 || input.AttemptNumber > 3 ||
		input.MaxAttempts < input.AttemptNumber || input.MaxAttempts > 3 ||
		!validOpaqueID(input.AgentInstanceID) ||
		!validOpaqueID(input.RuntimeInstanceID) ||
		!validOpaqueID(input.EvidenceID) ||
		!validSHA256Hex(input.EvidenceDigest) ||
		!validSHA256Hex(input.OutputSummaryDigest) ||
		!input.Classification.Valid() ||
		nilInterface(input.RecoveryPolicy) ||
		!input.RecoveryPolicy.Valid() ||
		!validCanonicalUUID(input.CorrelationID) {
		return ErrInvalidTeamRecovery
	}
	return nil
}

func teamExecutionStream(teamInstanceID string) string {
	return "team-execution/" + teamInstanceID
}

func teamAttemptIdentity(
	label string,
	plan teams.ExecutionPlan,
	logicalNodeID string,
	attemptNumber int,
) string {
	return teamAttemptIdentityFromValues(
		label, plan.TeamInstanceID(), plan.Digest(),
		logicalNodeID, attemptNumber,
	)
}

func teamAttemptIdentityFromValues(
	label string,
	teamInstanceID string,
	planDigest string,
	logicalNodeID string,
	attemptNumber int,
) string {
	digest := sha256.Sum256([]byte(strings.Join([]string{
		label, teamInstanceID, planDigest, logicalNodeID,
		fmt.Sprint(attemptNumber),
	}, "\x00")))
	prefix := "team-" + label + "-"
	return prefix + hex.EncodeToString(digest[:16])
}

func scheduledTeamAttempt(
	plan teams.ExecutionPlan,
	node teams.ExecutionNode,
	attemptNumber int,
	workflowPath string,
) TeamAttemptRecord {
	return TeamAttemptRecord{
		attemptNumber:     attemptNumber,
		workItemID:        teamAttemptIdentity("work", plan, node.LogicalNodeID(), attemptNumber),
		runID:             teamAttemptIdentity("run", plan, node.LogicalNodeID(), attemptNumber),
		runtimeInstanceID: node.RuntimeInstanceID(),
		agentInstanceID:   node.AgentInstanceID(),
		workflowPath:      workflowPath,
		status:            "scheduled",
	}
}

func teamPlanPayload(
	plan teams.ExecutionPlan,
	viewVersion string,
	bindings []TeamNodeSemanticBinding,
) struct {
	TeamInstanceID   string                       `json:"team_instance_id"`
	PlanDigest       string                       `json:"plan_digest"`
	ViewVersion      string                       `json:"view_version"`
	Nodes            []teamPlanNodePayload        `json:"nodes"`
	SemanticBindings []teamSemanticBindingPayload `json:"semantic_bindings"`
} {
	nodes := plan.Nodes()
	payloadNodes := make([]teamPlanNodePayload, len(nodes))
	for index, node := range nodes {
		payloadNodes[index] = teamPlanNodePayload{
			LogicalNodeID: node.LogicalNodeID(), Title: node.Title(),
			AgentInstanceID:   node.AgentInstanceID(),
			RuntimeInstanceID: node.RuntimeInstanceID(),
			Role:              node.Role(), DependsOn: node.DependsOn(),
			MaxAttempts: node.MaxAttempts(),
		}
	}
	payloadBindings := make([]teamSemanticBindingPayload, len(bindings))
	for index, binding := range bindings {
		payloadBindings[index] = teamSemanticBindingPayloadFrom(binding)
	}
	return struct {
		TeamInstanceID   string                       `json:"team_instance_id"`
		PlanDigest       string                       `json:"plan_digest"`
		ViewVersion      string                       `json:"view_version"`
		Nodes            []teamPlanNodePayload        `json:"nodes"`
		SemanticBindings []teamSemanticBindingPayload `json:"semantic_bindings"`
	}{
		plan.TeamInstanceID(), plan.Digest(), viewVersion,
		payloadNodes, payloadBindings,
	}
}

func teamSemanticBindingPayloadFrom(
	binding TeamNodeSemanticBinding,
) teamSemanticBindingPayload {
	version := binding.AcceptanceContractVersion
	digest := binding.AcceptanceContractDigest
	risk := binding.AcceptanceRisk
	required := binding.IndependentVerifierRequired
	verifierAgent := binding.VerifierAgentInstanceID
	verifierRuntime := binding.VerifierRuntimeInstanceID
	verifierWorkflow := binding.VerifierWorkflowPath
	return teamSemanticBindingPayload{
		LogicalNodeID:               binding.LogicalNodeID,
		OutputContractVersion:       binding.OutputContractVersion,
		OutputContractDigest:        binding.OutputContractDigest,
		RecoveryPolicyVersion:       binding.RecoveryPolicyVersion,
		RecoveryPolicyDigest:        binding.RecoveryPolicyDigest,
		AttemptCredits:              binding.AttemptCredits,
		PrimaryWorkflowPath:         binding.PrimaryWorkflowPath,
		WorkflowFallbackKey:         binding.WorkflowFallbackKey,
		RecoveryApprovalRequired:    binding.RecoveryApprovalRequired,
		AcceptanceContractVersion:   &version,
		AcceptanceContractDigest:    &digest,
		AcceptanceRisk:              &risk,
		IndependentVerifierRequired: &required,
		VerifierAgentInstanceID:     &verifierAgent,
		VerifierRuntimeInstanceID:   &verifierRuntime,
		VerifierWorkflowPath:        &verifierWorkflow,
	}
}

func teamAttemptScheduledPayload(
	logicalNodeID string,
	attempt TeamAttemptRecord,
	retryAt time.Time,
) struct {
	LogicalNodeID     string `json:"logical_node_id"`
	AttemptNumber     int    `json:"attempt_number"`
	WorkItemID        string `json:"work_item_id"`
	RunID             string `json:"run_id"`
	RuntimeInstanceID string `json:"runtime_instance_id"`
	AgentInstanceID   string `json:"agent_instance_id"`
	WorkflowPath      string `json:"workflow_path"`
	RetryAt           string `json:"retry_at"`
} {
	return struct {
		LogicalNodeID     string `json:"logical_node_id"`
		AttemptNumber     int    `json:"attempt_number"`
		WorkItemID        string `json:"work_item_id"`
		RunID             string `json:"run_id"`
		RuntimeInstanceID string `json:"runtime_instance_id"`
		AgentInstanceID   string `json:"agent_instance_id"`
		WorkflowPath      string `json:"workflow_path"`
		RetryAt           string `json:"retry_at"`
	}{
		logicalNodeID, attempt.attemptNumber, attempt.workItemID,
		attempt.runID, attempt.runtimeInstanceID, attempt.agentInstanceID,
		attempt.workflowPath,
		formatOptionalUTC(retryAt),
	}
}

func formatOptionalUTC(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}

func teamDispatchStreams(
	plan teams.ExecutionPlan,
	nodes []teams.ExecutionNode,
	selections []TeamAttemptSelection,
	team TeamExecutionRecord,
) []string {
	streams := map[string]struct{}{
		teamExecutionStream(plan.TeamInstanceID()): {},
		runIdentityStreamID:                        {},
	}
	for _, selection := range selections {
		node := nodeByLogicalID(nodes, selection.LogicalNodeID)
		runtimeInstanceID := node.RuntimeInstanceID()
		if teamNode := teamNodeByID(&team, selection.LogicalNodeID); teamNode != nil {
			if attempt := teamAttemptByNumber(
				teamNode,
				selection.AttemptNumber,
			); attempt != nil {
				runtimeInstanceID = attempt.runtimeInstanceID
			}
		}
		workItemID := teamAttemptIdentity(
			"work", plan, selection.LogicalNodeID, selection.AttemptNumber,
		)
		runID := teamAttemptIdentity(
			"run", plan, selection.LogicalNodeID, selection.AttemptNumber,
		)
		streams[workItemStream(workItemID)] = struct{}{}
		streams[runStream(runID)] = struct{}{}
		streams[runtimeStatusStream(runtimeInstanceID)] = struct{}{}
		streams[runtimeCapacityStream(runtimeInstanceID)] = struct{}{}
	}
	output := make([]string, 0, len(streams))
	for streamID := range streams {
		output = append(output, streamID)
	}
	sort.Strings(output)
	return output
}

func exactTeamHeads(left []journal.StreamHead, right []journal.StreamHead) bool {
	if len(left) != len(right) {
		return false
	}
	leftCopy := append([]journal.StreamHead(nil), left...)
	rightCopy := append([]journal.StreamHead(nil), right...)
	sort.Slice(leftCopy, func(i, j int) bool { return leftCopy[i].StreamID < leftCopy[j].StreamID })
	sort.Slice(rightCopy, func(i, j int) bool { return rightCopy[i].StreamID < rightCopy[j].StreamID })
	for index := range leftCopy {
		if leftCopy[index] != rightCopy[index] {
			return false
		}
	}
	return true
}

func snapshotHead(snapshot journal.StreamSetSnapshot, streamID string) journal.StreamHead {
	head, _ := snapshot.Head(streamID)
	return head
}

func nodeByLogicalID(
	nodes []teams.ExecutionNode,
	logicalNodeID string,
) teams.ExecutionNode {
	for _, node := range nodes {
		if node.LogicalNodeID() == logicalNodeID {
			return node
		}
	}
	return teams.ExecutionNode{}
}

func plannedTeamRecord(
	plan teams.ExecutionPlan,
	bindings []TeamNodeSemanticBinding,
	sequence int64,
	eventID string,
) TeamExecutionRecord {
	nodes := plan.Nodes()
	records := make([]TeamNodeRecord, len(nodes))
	for index, node := range nodes {
		binding := teamSemanticBindingByID(bindings, node.LogicalNodeID())
		records[index] = TeamNodeRecord{
			logicalNodeID: node.LogicalNodeID(),
			status:        "pending", maxAttempts: node.MaxAttempts(),
			attempts:        []TeamAttemptRecord{},
			semanticBinding: binding,
		}
	}
	return TeamExecutionRecord{
		teamInstanceID: plan.TeamInstanceID(), planDigest: plan.Digest(),
		status: "pending", nodes: records,
		streamSequence: sequence, lastEventID: eventID,
	}
}

func teamSemanticBindingByID(
	bindings []TeamNodeSemanticBinding,
	logicalNodeID string,
) TeamNodeSemanticBinding {
	for _, binding := range bindings {
		if binding.LogicalNodeID == logicalNodeID {
			return binding
		}
	}
	return TeamNodeSemanticBinding{}
}

func exactTeamSemanticBindings(
	team TeamExecutionRecord,
	bindings []TeamNodeSemanticBinding,
) bool {
	if team.legacySemanticUnbound || team.legacyAcceptanceUnbound ||
		len(team.nodes) != len(bindings) {
		return false
	}
	for _, node := range team.nodes {
		if node.semanticBinding !=
			teamSemanticBindingByID(bindings, node.logicalNodeID) {
			return false
		}
	}
	return true
}

func canonicalTeamSemanticDigest(
	bindings []TeamNodeSemanticBinding,
) string {
	hash := sha256.New()
	fields := []string{"loom.team-semantic-bindings.v1"}
	for _, binding := range bindings {
		fields = append(fields,
			binding.LogicalNodeID,
			fmt.Sprint(binding.OutputContractVersion),
			binding.OutputContractDigest,
			fmt.Sprint(binding.RecoveryPolicyVersion),
			binding.RecoveryPolicyDigest,
			fmt.Sprint(binding.AttemptCredits),
			binding.PrimaryWorkflowPath,
			binding.WorkflowFallbackKey,
			fmt.Sprint(binding.RecoveryApprovalRequired),
			fmt.Sprint(binding.AcceptanceContractVersion),
			binding.AcceptanceContractDigest,
			binding.AcceptanceRisk,
			fmt.Sprint(binding.IndependentVerifierRequired),
			binding.VerifierAgentInstanceID,
			binding.VerifierRuntimeInstanceID,
			binding.VerifierWorkflowPath,
		)
	}
	for _, field := range fields {
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(field)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write([]byte(field))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func completeTeamAcceptancePayload(
	payload teamSemanticBindingPayload,
) (bool, bool) {
	present := 0
	for _, value := range []any{
		payload.AcceptanceContractVersion,
		payload.AcceptanceContractDigest,
		payload.AcceptanceRisk,
		payload.IndependentVerifierRequired,
		payload.VerifierAgentInstanceID,
		payload.VerifierRuntimeInstanceID,
		payload.VerifierWorkflowPath,
	} {
		if !nilInterface(value) {
			present++
		}
	}
	return present == 7, present == 0 || present == 7
}

func validTeamAcceptanceBinding(
	binding TeamNodeSemanticBinding,
	sourceAgentInstanceID string,
) bool {
	if binding.AcceptanceContractVersion < 1 ||
		!validSHA256Hex(binding.AcceptanceContractDigest) ||
		!validAcceptanceRiskString(binding.AcceptanceRisk) {
		return false
	}
	required := binding.AcceptanceRisk == "medium" ||
		binding.AcceptanceRisk == "high"
	if binding.IndependentVerifierRequired != required {
		return false
	}
	if !required {
		return binding.VerifierAgentInstanceID == "" &&
			binding.VerifierRuntimeInstanceID == "" &&
			binding.VerifierWorkflowPath == ""
	}
	return validOpaqueID(binding.VerifierAgentInstanceID) &&
		binding.VerifierAgentInstanceID != sourceAgentInstanceID &&
		validOpaqueID(binding.VerifierRuntimeInstanceID) &&
		validOpaqueID(binding.VerifierWorkflowPath)
}

func validAcceptanceRiskString(value string) bool {
	return value == "low" || value == "medium" || value == "high"
}

func applyScheduledAttempt(
	team *TeamExecutionRecord,
	logicalNodeID string,
	attempt TeamAttemptRecord,
	retryAt time.Time,
) {
	node := teamNodeByID(team, logicalNodeID)
	if node == nil {
		return
	}
	node.currentAttempt = attempt.attemptNumber
	node.retryAt = retryAt
	if attempt.attemptNumber == 1 {
		node.status = "pending"
	} else if node.recoveryAction == "fallback" {
		node.status = "fallback_scheduled"
	} else {
		node.status = "retry_scheduled"
	}
	node.attempts = append(node.attempts, attempt)
}

func teamNodeByID(
	team *TeamExecutionRecord,
	logicalNodeID string,
) *TeamNodeRecord {
	if team == nil {
		return nil
	}
	for index := range team.nodes {
		if team.nodes[index].logicalNodeID == logicalNodeID {
			return &team.nodes[index]
		}
	}
	return nil
}

func teamAttemptByNumber(
	node *TeamNodeRecord,
	attemptNumber int,
) *TeamAttemptRecord {
	if node == nil {
		return nil
	}
	for index := range node.attempts {
		if node.attempts[index].attemptNumber == attemptNumber {
			return &node.attempts[index]
		}
	}
	return nil
}

func filterTeamEvents(events []journal.Event, streamID string) []journal.Event {
	filtered := make([]journal.Event, 0)
	for _, event := range events {
		if event.StreamID == streamID {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func replayTeamDispatchState(
	ctx context.Context,
	events []journal.Event,
) (authorityState, error) {
	state := authorityState{
		workItems:     make(map[string]WorkItemRecord),
		runs:          make(map[string]RunRecord),
		runtimes:      make(map[string]authorityRuntime),
		heads:         make(map[string]int64),
		runIdentities: make(map[string]runIdentityReservation),
	}
	ordered := append([]journal.Event(nil), events...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].StreamID != ordered[j].StreamID {
			return ordered[i].StreamID < ordered[j].StreamID
		}
		return ordered[i].Seq < ordered[j].Seq
	})
	byStream := make(map[string][]journal.Event)
	for _, event := range ordered {
		if err := ctx.Err(); err != nil {
			return authorityState{}, err
		}
		if event.Seq != state.heads[event.StreamID]+1 {
			return authorityState{}, ErrTeamExecutionConflict
		}
		state.heads[event.StreamID] = event.Seq
		byStream[event.StreamID] = append(byStream[event.StreamID], event)
	}
	if identityEvents := byStream[runIdentityStreamID]; len(identityEvents) > 0 {
		if err := replayRunIdentityStream(&state, identityEvents); err != nil {
			return authorityState{}, err
		}
	}
	for streamID, streamEvents := range byStream {
		if strings.HasPrefix(streamID, "runtime_instance:") {
			if err := replayRuntimeIdentityStream(
				&state,
				streamID,
				streamEvents,
			); err != nil {
				return authorityState{}, err
			}
		}
	}
	for streamID, streamEvents := range byStream {
		if !strings.HasPrefix(streamID, "runtime_capacity:") {
			continue
		}
		runtimeID := strings.TrimPrefix(streamID, "runtime_capacity:")
		runtime := state.runtimes[runtimeID]
		if runtime.id == "" {
			return authorityState{}, ErrTeamExecutionConflict
		}
		for _, event := range streamEvents {
			var payload capacityEventPayload
			if decodeExactPayload(event.PayloadJSON, &payload) != nil ||
				payload.RuntimeInstanceID != runtimeID ||
				payload.WorkItemID == "" ||
				payload.RunID == "" ||
				!validCanonicalUUID(payload.ClaimID) ||
				payload.ClaimGeneration <= 0 ||
				payload.AgentInstanceID == "" {
				return authorityState{}, ErrTeamExecutionConflict
			}
			statusFact, exists := runtime.statusFacts[payload.RuntimeStatusSequence]
			if !exists ||
				payload.RuntimeStatusStreamID != statusFact.reference.streamID ||
				payload.RuntimeStatusEventID != statusFact.reference.eventID {
				return authorityState{}, ErrTeamExecutionConflict
			}
			binding := capacityBinding{
				workItemID:        payload.WorkItemID,
				runID:             payload.RunID,
				claimID:           payload.ClaimID,
				claimGeneration:   payload.ClaimGeneration,
				runtimeInstanceID: payload.RuntimeInstanceID,
				agentInstanceID:   payload.AgentInstanceID,
			}
			key := capacityKey(payload.RunID, payload.ClaimGeneration)
			switch event.Type {
			case "RuntimeCapacityReserved":
				if existing, duplicate := runtime.active[key]; duplicate &&
					existing != binding {
					return authorityState{}, ErrTeamExecutionConflict
				}
				runtime.active[key] = binding
			case "RuntimeCapacityReleased":
				if existing, exists := runtime.active[key]; !exists ||
					existing != binding {
					return authorityState{}, ErrTeamExecutionConflict
				}
				delete(runtime.active, key)
			default:
				return authorityState{}, ErrTeamExecutionConflict
			}
		}
		state.runtimes[runtimeID] = runtime
	}
	return state, nil
}

func replayTeamExecution(
	teamInstanceID string,
	events []journal.Event,
) (TeamExecutionRecord, error) {
	var team TeamExecutionRecord
	var sequence int64
	for _, event := range events {
		if event.StreamID != teamExecutionStream(teamInstanceID) ||
			event.Seq != sequence+1 ||
			event.SchemaVersion != 1 {
			return TeamExecutionRecord{}, ErrTeamExecutionConflict
		}
		sequence = event.Seq
		switch event.Type {
		case "TeamExecutionPlanned":
			var payload struct {
				TeamInstanceID   string                        `json:"team_instance_id"`
				PlanDigest       string                        `json:"plan_digest"`
				ViewVersion      string                        `json:"view_version"`
				Nodes            []teamPlanNodePayload         `json:"nodes"`
				SemanticBindings *[]teamSemanticBindingPayload `json:"semantic_bindings"`
			}
			if team.status != "" ||
				decodeExactPayload(event.PayloadJSON, &payload) != nil ||
				payload.TeamInstanceID != teamInstanceID ||
				!validSHA256Hex(payload.PlanDigest) ||
				!validSHA256Hex(payload.ViewVersion) ||
				len(payload.Nodes) == 0 || len(payload.Nodes) > 3 {
				return TeamExecutionRecord{}, ErrTeamExecutionConflict
			}
			team = TeamExecutionRecord{
				teamInstanceID:        teamInstanceID,
				planDigest:            payload.PlanDigest,
				status:                "pending",
				nodes:                 make([]TeamNodeRecord, len(payload.Nodes)),
				legacySemanticUnbound: payload.SemanticBindings == nil,
			}
			bindings := make(map[string]TeamNodeSemanticBinding)
			if payload.SemanticBindings != nil {
				if len(*payload.SemanticBindings) != len(payload.Nodes) {
					return TeamExecutionRecord{}, ErrTeamExecutionConflict
				}
				for index, binding := range *payload.SemanticBindings {
					if index > 0 &&
						(*payload.SemanticBindings)[index-1].LogicalNodeID >=
							binding.LogicalNodeID {
						return TeamExecutionRecord{}, ErrTeamExecutionConflict
					}
					acceptancePresent, complete := completeTeamAcceptancePayload(binding)
					if !complete ||
						index > 0 &&
							team.legacyAcceptanceUnbound == acceptancePresent {
						return TeamExecutionRecord{}, ErrTeamExecutionConflict
					}
					if index == 0 {
						team.legacyAcceptanceUnbound = !acceptancePresent
					}
					public := TeamNodeSemanticBinding{
						LogicalNodeID:            binding.LogicalNodeID,
						OutputContractVersion:    binding.OutputContractVersion,
						OutputContractDigest:     binding.OutputContractDigest,
						RecoveryPolicyVersion:    binding.RecoveryPolicyVersion,
						RecoveryPolicyDigest:     binding.RecoveryPolicyDigest,
						AttemptCredits:           binding.AttemptCredits,
						PrimaryWorkflowPath:      binding.PrimaryWorkflowPath,
						WorkflowFallbackKey:      binding.WorkflowFallbackKey,
						RecoveryApprovalRequired: binding.RecoveryApprovalRequired,
					}
					if acceptancePresent {
						public.AcceptanceContractVersion = *binding.AcceptanceContractVersion
						public.AcceptanceContractDigest = *binding.AcceptanceContractDigest
						public.AcceptanceRisk = *binding.AcceptanceRisk
						public.IndependentVerifierRequired = *binding.IndependentVerifierRequired
						public.VerifierAgentInstanceID = *binding.VerifierAgentInstanceID
						public.VerifierRuntimeInstanceID = *binding.VerifierRuntimeInstanceID
						public.VerifierWorkflowPath = *binding.VerifierWorkflowPath
					}
					if public.OutputContractVersion < 1 ||
						!validSHA256Hex(public.OutputContractDigest) ||
						public.RecoveryPolicyVersion < 1 ||
						!validSHA256Hex(public.RecoveryPolicyDigest) ||
						public.AttemptCredits < 0 ||
						public.AttemptCredits > 2 ||
						!validOpaqueID(public.PrimaryWorkflowPath) ||
						public.WorkflowFallbackKey != "" &&
							(!validOpaqueID(public.WorkflowFallbackKey) ||
								public.WorkflowFallbackKey ==
									public.PrimaryWorkflowPath) {
						return TeamExecutionRecord{}, ErrTeamExecutionConflict
					}
					bindings[public.LogicalNodeID] = public
				}
			}
			for index, node := range payload.Nodes {
				binding := bindings[node.LogicalNodeID]
				if payload.SemanticBindings != nil &&
					(binding.LogicalNodeID == "" ||
						binding.AttemptCredits > node.MaxAttempts-1 ||
						!team.legacyAcceptanceUnbound &&
							!validTeamAcceptanceBinding(
								binding,
								node.AgentInstanceID,
							)) {
					return TeamExecutionRecord{}, ErrTeamExecutionConflict
				}
				team.nodes[index] = TeamNodeRecord{
					logicalNodeID:   node.LogicalNodeID,
					status:          "pending",
					maxAttempts:     node.MaxAttempts,
					attempts:        []TeamAttemptRecord{},
					semanticBinding: binding,
				}
			}
			sort.Slice(team.nodes, func(i, j int) bool {
				return team.nodes[i].logicalNodeID < team.nodes[j].logicalNodeID
			})
		case "TeamNodeAttemptScheduled":
			var payload struct {
				LogicalNodeID     string  `json:"logical_node_id"`
				AttemptNumber     int     `json:"attempt_number"`
				WorkItemID        string  `json:"work_item_id"`
				RunID             string  `json:"run_id"`
				RuntimeInstanceID string  `json:"runtime_instance_id"`
				AgentInstanceID   string  `json:"agent_instance_id"`
				WorkflowPath      *string `json:"workflow_path"`
				RetryAt           string  `json:"retry_at"`
			}
			if decodeExactPayload(event.PayloadJSON, &payload) != nil {
				return TeamExecutionRecord{}, ErrTeamExecutionConflict
			}
			retryAt, err := parseOptionalUTC(payload.RetryAt)
			if err != nil {
				return TeamExecutionRecord{}, ErrTeamExecutionConflict
			}
			attempt := TeamAttemptRecord{
				attemptNumber: payload.AttemptNumber,
				workItemID:    payload.WorkItemID, runID: payload.RunID,
				runtimeInstanceID: payload.RuntimeInstanceID,
				agentInstanceID:   payload.AgentInstanceID,
				workflowPath:      "",
				status:            "scheduled",
			}
			node := teamNodeByID(&team, payload.LogicalNodeID)
			if node == nil {
				return TeamExecutionRecord{}, ErrTeamExecutionConflict
			}
			if teamAttemptByNumber(node, payload.AttemptNumber) != nil {
				return TeamExecutionRecord{}, ErrTeamExecutionConflict
			}
			if team.legacySemanticUnbound {
				if payload.WorkflowPath != nil {
					return TeamExecutionRecord{}, ErrTeamExecutionConflict
				}
			} else {
				if payload.WorkflowPath == nil ||
					!validOpaqueID(*payload.WorkflowPath) {
					return TeamExecutionRecord{}, ErrTeamExecutionConflict
				}
				attempt.workflowPath = *payload.WorkflowPath
				if payload.AttemptNumber == 1 {
					if attempt.workflowPath !=
						node.semanticBinding.PrimaryWorkflowPath ||
						!retryAt.IsZero() {
						return TeamExecutionRecord{}, ErrTeamExecutionConflict
					}
				} else {
					expectedWorkflow := ""
					switch node.recoveryAction {
					case "retry":
						previous := teamAttemptByNumber(
							node,
							payload.AttemptNumber-1,
						)
						if previous != nil {
							expectedWorkflow = previous.workflowPath
						}
					case "fallback":
						expectedWorkflow =
							node.semanticBinding.WorkflowFallbackKey
					}
					if expectedWorkflow == "" ||
						attempt.workflowPath != expectedWorkflow ||
						!retryAt.Equal(node.retryAt) {
						return TeamExecutionRecord{}, ErrTeamExecutionConflict
					}
				}
			}
			applyScheduledAttempt(&team, payload.LogicalNodeID, attempt, retryAt)
		case "TeamReadySetDispatched":
			var payload struct {
				TeamInstanceID string                       `json:"team_instance_id"`
				PlanDigest     string                       `json:"plan_digest"`
				ViewVersion    string                       `json:"view_version"`
				Attempts       []teamDispatchAttemptPayload `json:"attempts"`
			}
			if decodeExactPayload(event.PayloadJSON, &payload) != nil ||
				payload.TeamInstanceID != team.teamInstanceID ||
				payload.PlanDigest != team.planDigest {
				return TeamExecutionRecord{}, ErrTeamExecutionConflict
			}
			for _, dispatched := range payload.Attempts {
				node := teamNodeByID(&team, dispatched.LogicalNodeID)
				attempt := teamAttemptByNumber(node, dispatched.AttemptNumber)
				if attempt == nil ||
					attempt.workItemID != dispatched.WorkItemID ||
					attempt.runID != dispatched.RunID ||
					attempt.runtimeInstanceID != dispatched.RuntimeInstanceID ||
					attempt.agentInstanceID != dispatched.AgentInstanceID {
					return TeamExecutionRecord{}, ErrTeamExecutionConflict
				}
				attempt.claimID = dispatched.ClaimID
				attempt.claimGeneration = dispatched.ClaimGeneration
				attempt.status = "dispatched"
				node.status = "running"
				node.retryAt = time.Time{}
			}
			team.status = "running"
		case "TeamNodeAttemptRebound":
			var payload teamAttemptReboundPayload
			if decodeExactPayload(event.PayloadJSON, &payload) != nil ||
				payload.TeamInstanceID != team.teamInstanceID ||
				payload.PlanDigest != team.planDigest ||
				payload.RunStream != runStream(payload.RunID) ||
				payload.RunSequence <= 0 ||
				!validOpaqueID(payload.RunEventID) ||
				!validCanonicalUUID(payload.PreviousClaimID) ||
				!validCanonicalUUID(payload.ClaimID) ||
				payload.ClaimID == payload.PreviousClaimID ||
				payload.ClaimGeneration != payload.PreviousClaimGeneration+1 {
				return TeamExecutionRecord{}, ErrTeamExecutionConflict
			}
			node := teamNodeByID(&team, payload.LogicalNodeID)
			attempt := teamAttemptByNumber(node, payload.AttemptNumber)
			if node == nil ||
				attempt == nil ||
				node.status != "running" ||
				node.currentAttempt != payload.AttemptNumber ||
				attempt.status != "dispatched" ||
				attempt.workItemID != payload.WorkItemID ||
				attempt.runID != payload.RunID ||
				attempt.claimID != payload.PreviousClaimID ||
				attempt.claimGeneration != payload.PreviousClaimGeneration ||
				attempt.runtimeInstanceID != payload.RuntimeInstanceID ||
				attempt.agentInstanceID != payload.AgentInstanceID {
				return TeamExecutionRecord{}, ErrTeamExecutionConflict
			}
			attempt.claimID = payload.ClaimID
			attempt.claimGeneration = payload.ClaimGeneration
		case "TeamNodeAttemptTerminal":
			var payload teamAttemptTerminalPayload
			if decodeExactPayload(event.PayloadJSON, &payload) != nil {
				return TeamExecutionRecord{}, ErrTeamExecutionConflict
			}
			node := teamNodeByID(&team, payload.LogicalNodeID)
			attempt := teamAttemptByNumber(node, payload.AttemptNumber)
			if node == nil ||
				attempt == nil ||
				node.currentAttempt != payload.AttemptNumber ||
				attempt.status != "dispatched" ||
				attempt.workItemID != payload.WorkItemID ||
				attempt.runID != payload.RunID ||
				attempt.claimID != payload.ClaimID ||
				attempt.claimGeneration != payload.ClaimGeneration ||
				attempt.runtimeInstanceID != payload.RuntimeInstanceID ||
				attempt.agentInstanceID != payload.AgentInstanceID ||
				payload.Status != "succeeded" &&
					payload.Status != "failed" &&
					payload.Status != "cancelled" ||
				!validOpaqueID(payload.EvidenceID) ||
				!validSHA256Hex(payload.EvidenceDigest) {
				return TeamExecutionRecord{}, ErrTeamExecutionConflict
			}
			if !team.legacySemanticUnbound {
				if payload.OutputContractVersion !=
					node.semanticBinding.OutputContractVersion ||
					payload.OutputContractDigest !=
						node.semanticBinding.OutputContractDigest ||
					!validOutputClassificationString(
						payload.OutputClassification,
					) ||
					!validSHA256Hex(
						payload.OutputClassificationDigest,
					) ||
					!validSHA256Hex(payload.OutputSummaryDigest) {
					return TeamExecutionRecord{}, ErrTeamExecutionConflict
				}
			}
			applyAttemptTerminal(
				&team, payload.LogicalNodeID, payload.AttemptNumber,
				payload.Status, payload.EvidenceID, payload.EvidenceDigest,
				payload.OutputContractVersion,
				payload.OutputContractDigest,
				verification.OutputClassification(
					payload.OutputClassification,
				),
				payload.OutputClassificationDigest,
				payload.OutputSummaryDigest,
			)
		case "TeamNodeAcceptanceCommitted":
			var payload teamNodeAcceptancePayload
			if decodeExactPayload(event.PayloadJSON, &payload) != nil ||
				payload.TeamInstanceID != team.teamInstanceID ||
				payload.PlanDigest != team.planDigest ||
				event.ID != teamAcceptancePayloadEventID(payload) ||
				event.CausationID != payload.WorkOutcomeEventID ||
				!validOpaqueID(payload.WorkOutcomeEventID) ||
				!validSHA256Hex(payload.AcceptanceDecisionDigest) {
				return TeamExecutionRecord{}, ErrTeamExecutionConflict
			}
			node := teamNodeByID(&team, payload.LogicalNodeID)
			attempt := teamAttemptByNumber(node, payload.AttemptNumber)
			if team.legacyAcceptanceUnbound ||
				node == nil ||
				attempt == nil ||
				node.currentAttempt != payload.AttemptNumber ||
				node.status != "ready_for_review" ||
				attempt.workItemID != payload.WorkItemID ||
				payload.AcceptanceContractVersion !=
					node.semanticBinding.AcceptanceContractVersion ||
				payload.AcceptanceContractDigest !=
					node.semanticBinding.AcceptanceContractDigest {
				return TeamExecutionRecord{}, ErrTeamExecutionConflict
			}
			decidedAt, err := parseOptionalUTC(payload.DecidedAt)
			if err != nil || decidedAt.IsZero() {
				return TeamExecutionRecord{}, ErrTeamExecutionConflict
			}
			switch payload.AcceptanceDecisionKind {
			case "accepted":
				if payload.NodeStatus != "succeeded" ||
					!payload.DependencySatisfied ||
					payload.RecoveryTrigger != "" ||
					payload.RecoveryPolicyVersion != 0 ||
					payload.RecoveryPolicyDigest != "" ||
					payload.CreditsBefore != 0 {
					return TeamExecutionRecord{}, ErrTeamExecutionConflict
				}
			case "rejected":
				expectedCredits := node.semanticBinding.AttemptCredits -
					(payload.AttemptNumber - 1)
				if expectedCredits < 0 {
					expectedCredits = 0
				}
				if payload.NodeStatus != "awaiting_recovery" ||
					payload.DependencySatisfied ||
					payload.RecoveryTrigger !=
						teamRecoveryTriggerVerificationRejected ||
					payload.RecoveryPolicyVersion !=
						node.semanticBinding.RecoveryPolicyVersion ||
					payload.RecoveryPolicyDigest !=
						node.semanticBinding.RecoveryPolicyDigest ||
					payload.CreditsBefore != expectedCredits {
					return TeamExecutionRecord{}, ErrTeamExecutionConflict
				}
			default:
				return TeamExecutionRecord{}, ErrTeamExecutionConflict
			}
			node.acceptanceDecisionDigest =
				payload.AcceptanceDecisionDigest
			node.acceptanceDecisionTime = decidedAt
			node.status = payload.NodeStatus
			node.dependencySatisfied = payload.DependencySatisfied
			node.recoveryTrigger = payload.RecoveryTrigger
			node.recoveryPolicyVersion = payload.RecoveryPolicyVersion
			node.recoveryPolicyDigest = payload.RecoveryPolicyDigest
			node.creditsBefore = payload.CreditsBefore
			if payload.NodeStatus == "awaiting_recovery" {
				team.status = "awaiting_recovery"
			}
		case "TeamNodeRecoveryRecorded":
			var payload teamRecoveryPayload
			if decodeExactPayload(event.PayloadJSON, &payload) != nil {
				return TeamExecutionRecord{}, ErrTeamExecutionConflict
			}
			node := teamNodeByID(&team, payload.LogicalNodeID)
			if node == nil || node.currentAttempt != payload.AttemptNumber {
				return TeamExecutionRecord{}, ErrTeamExecutionConflict
			}
			attempt := teamAttemptByNumber(node, payload.AttemptNumber)
			expectedPrior := teamPriorClassifications(
				node,
				payload.AttemptNumber,
			)
			expectedCredits := node.semanticBinding.AttemptCredits -
				(payload.AttemptNumber - 1)
			if expectedCredits < 0 {
				expectedCredits = 0
			}
			retryAt, retryErr := parseOptionalUTC(payload.RetryAt)
			fallbackBefore := teamFallbackConsumed(node)
			expectedTrigger := node.recoveryTrigger
			if expectedTrigger == "" {
				expectedTrigger = teamRecoveryTriggerOutput
			}
			payloadTrigger := payload.RecoveryTrigger
			if payloadTrigger == "" &&
				payload.AcceptanceDecisionDigest == "" {
				payloadTrigger = teamRecoveryTriggerOutput
			}
			if team.legacySemanticUnbound ||
				attempt == nil ||
				!validSHA256Hex(payload.RecoveryDecisionDigest) ||
				payload.RecoveryPolicyVersion !=
					node.semanticBinding.RecoveryPolicyVersion ||
				payload.RecoveryPolicyDigest !=
					node.semanticBinding.RecoveryPolicyDigest ||
				payload.CreditsBefore < 0 ||
				payload.CreditsAfter < 0 ||
				payload.CreditsBefore > node.semanticBinding.AttemptCredits ||
				payload.CreditsBefore != expectedCredits ||
				len(payload.PriorClassifications) > 2 ||
				len(payload.PriorClassifications) !=
					payload.AttemptNumber-1 ||
				payload.ClassificationDigest !=
					attempt.outputClassificationDigest ||
				payloadTrigger != expectedTrigger ||
				payload.AcceptanceDecisionDigest !=
					node.acceptanceDecisionDigest ||
				retryErr != nil {
				return TeamExecutionRecord{}, ErrTeamExecutionConflict
			}
			for index, classification := range expectedPrior {
				if payload.PriorClassifications[index] !=
					string(classification) {
					return TeamExecutionRecord{}, ErrTeamExecutionConflict
				}
			}
			switch payload.Action {
			case "retry":
				if payload.NextAttemptNumber != payload.AttemptNumber+1 ||
					payload.NextAgentInstanceID !=
						attempt.agentInstanceID ||
					payload.NextRuntimeInstanceID !=
						attempt.runtimeInstanceID ||
					payload.WorkflowFallbackKey != "" ||
					payload.CreditsAfter != payload.CreditsBefore-1 ||
					retryAt.IsZero() ||
					payload.DependencySatisfied ||
					payload.FallbackConsumed != fallbackBefore {
					return TeamExecutionRecord{}, ErrTeamExecutionConflict
				}
			case "fallback":
				if payload.NextAttemptNumber != payload.AttemptNumber+1 ||
					payload.NextAgentInstanceID !=
						attempt.agentInstanceID ||
					payload.NextRuntimeInstanceID !=
						attempt.runtimeInstanceID ||
					payload.WorkflowFallbackKey == "" ||
					payload.WorkflowFallbackKey !=
						node.semanticBinding.WorkflowFallbackKey ||
					payload.CreditsAfter != payload.CreditsBefore-1 ||
					retryAt.IsZero() ||
					payload.DependencySatisfied ||
					fallbackBefore ||
					!payload.FallbackConsumed {
					return TeamExecutionRecord{}, ErrTeamExecutionConflict
				}
			case "degraded":
				if payload.NextAttemptNumber != 0 ||
					payload.NextAgentInstanceID != "" ||
					payload.NextRuntimeInstanceID != "" ||
					payload.WorkflowFallbackKey != "" ||
					payload.CreditsAfter != payload.CreditsBefore ||
					!retryAt.IsZero() ||
					!payload.DependencySatisfied ||
					payload.FallbackConsumed != fallbackBefore {
					return TeamExecutionRecord{}, ErrTeamExecutionConflict
				}
			case "blocked", "human_required":
				if payload.NextAttemptNumber != 0 ||
					payload.NextAgentInstanceID != "" ||
					payload.NextRuntimeInstanceID != "" ||
					payload.WorkflowFallbackKey != "" ||
					payload.CreditsAfter != payload.CreditsBefore ||
					!retryAt.IsZero() ||
					payload.DependencySatisfied ||
					payload.FallbackConsumed != fallbackBefore {
					return TeamExecutionRecord{}, ErrTeamExecutionConflict
				}
			default:
				return TeamExecutionRecord{}, ErrTeamExecutionConflict
			}
			node.dependencySatisfied = payload.DependencySatisfied
			node.recoveryTrigger = payloadTrigger
			node.recoveryAction = payload.Action
			node.recoveryDecisionDigest = payload.RecoveryDecisionDigest
			node.creditsBefore = payload.CreditsBefore
			node.creditsAfter = payload.CreditsAfter
			node.fallbackConsumed = payload.FallbackConsumed
			node.priorClassifications = make(
				[]verification.OutputClassification,
				len(payload.PriorClassifications),
			)
			for index, value := range payload.PriorClassifications {
				if !validOutputClassificationString(value) {
					return TeamExecutionRecord{}, ErrTeamExecutionConflict
				}
				node.priorClassifications[index] =
					verification.OutputClassification(value)
			}
			decisionTime, err := parseOptionalUTC(payload.DecisionTime)
			if err != nil || decisionTime.IsZero() {
				return TeamExecutionRecord{}, ErrTeamExecutionConflict
			}
			node.recoveryDecisionTime = decisionTime
			node.retryAt = retryAt
			switch payload.Action {
			case "retry":
				node.status = "retry_scheduled"
			case "fallback":
				node.status = "fallback_scheduled"
			case "degraded":
				node.status = "degraded"
			case "blocked":
				node.status = "blocked"
			case "human_required":
				node.status = "human_required"
			default:
				return TeamExecutionRecord{}, ErrTeamExecutionConflict
			}
		case "TeamExecutionTerminal":
			var payload struct {
				TeamInstanceID string `json:"team_instance_id"`
				PlanDigest     string `json:"plan_digest"`
				Status         string `json:"status"`
				Reason         string `json:"reason"`
			}
			expectedStatus, expectedReason :=
				aggregateTeamTerminal(team)
			if decodeExactPayload(event.PayloadJSON, &payload) != nil ||
				payload.TeamInstanceID != team.teamInstanceID ||
				payload.PlanDigest != team.planDigest ||
				isTerminalTeamStatus(team.status) ||
				!isTerminalTeamStatus(payload.Status) ||
				!teamIsTerminal(team) ||
				payload.Status != expectedStatus ||
				payload.Reason != expectedReason ||
				event.CausationID != team.lastEventID {
				return TeamExecutionRecord{}, ErrTeamExecutionConflict
			}
			team.status = payload.Status
		default:
			return TeamExecutionRecord{}, ErrTeamExecutionConflict
		}
		team.streamSequence = event.Seq
		team.lastEventID = event.ID
	}
	return team, nil
}

func applyAttemptTerminal(
	team *TeamExecutionRecord,
	logicalNodeID string,
	attemptNumber int,
	status string,
	evidenceID string,
	evidenceDigest string,
	outputContractVersion int,
	outputContractDigest string,
	outputClassification verification.OutputClassification,
	outputClassificationDigest string,
	outputSummaryDigest string,
) {
	node := teamNodeByID(team, logicalNodeID)
	attempt := teamAttemptByNumber(node, attemptNumber)
	if attempt == nil {
		return
	}
	attempt.status = status
	attempt.evidenceID = evidenceID
	attempt.evidenceDigest = evidenceDigest
	attempt.outputContractVersion = outputContractVersion
	attempt.outputContractDigest = outputContractDigest
	attempt.outputClassification = outputClassification
	attempt.outputClassificationDigest = outputClassificationDigest
	attempt.outputSummaryDigest = outputSummaryDigest
	switch outputClassification {
	case verification.OutputValidNonEmpty, verification.OutputValidEmpty:
		node.status = "ready_for_review"
		node.dependencySatisfied = false
		node.recoveryTrigger = ""
	default:
		node.status = "awaiting_recovery"
		node.dependencySatisfied = false
		node.recoveryTrigger = teamRecoveryTriggerOutput
	}
	if node.status == "awaiting_recovery" {
		team.status = "awaiting_recovery"
	}
}

func validOutputClassificationString(value string) bool {
	switch verification.OutputClassification(value) {
	case verification.OutputValidNonEmpty,
		verification.OutputValidEmpty,
		verification.OutputTransientEmpty,
		verification.OutputInvalid:
		return true
	default:
		return false
	}
}

type teamRunEventReference struct {
	sequence int64
	eventID  string
}

func teamRebindRunFacts(
	events []journal.Event,
	input TeamAttemptRebindInput,
) (time.Time, teamRunEventReference, error) {
	var previousExpiresAt time.Time
	var current teamRunEventReference
	var runHead int64
	for _, event := range events {
		if event.StreamID != runStream(input.RunID) {
			continue
		}
		if event.Seq > runHead {
			runHead = event.Seq
		}
		if event.Type != "RunClaimed" {
			continue
		}
		var payload runClaimedPayload
		if decodeExactPayload(event.PayloadJSON, &payload) != nil ||
			payload.ClaimID == nil ||
			payload.ClaimGeneration == nil ||
			payload.PrepareLeaseExpiresAt == nil {
			return time.Time{}, teamRunEventReference{}, ErrTeamExecutionConflict
		}
		switch {
		case *payload.ClaimID == input.PreviousClaimID &&
			*payload.ClaimGeneration == input.PreviousClaimGeneration:
			expiresAt, err := parseUTC(*payload.PrepareLeaseExpiresAt)
			if err != nil {
				return time.Time{}, teamRunEventReference{}, ErrTeamExecutionConflict
			}
			previousExpiresAt = expiresAt
		case *payload.ClaimID == input.ClaimID &&
			*payload.ClaimGeneration == input.ClaimGeneration:
			current = teamRunEventReference{
				sequence: event.Seq,
				eventID:  event.ID,
			}
		}
	}
	if previousExpiresAt.IsZero() ||
		current.sequence <= 0 ||
		current.sequence != runHead ||
		!validOpaqueID(current.eventID) {
		return time.Time{}, teamRunEventReference{}, ErrTeamExecutionConflict
	}
	return previousExpiresAt, current, nil
}

func teamIsTerminal(team TeamExecutionRecord) bool {
	for _, node := range team.nodes {
		switch node.status {
		case "succeeded", "failed", "cancelled", "degraded", "blocked", "human_required":
		default:
			return false
		}
	}
	return len(team.nodes) > 0
}

func aggregateTeamTerminal(team TeamExecutionRecord) (string, string) {
	for _, status := range []string{
		"human_required",
		"blocked",
		"failed",
		"cancelled",
		"degraded",
	} {
		for _, node := range team.nodes {
			if node.status == status {
				return status, "node_" + node.logicalNodeID + "_" + status
			}
		}
	}
	for _, node := range team.nodes {
		if node.status != "succeeded" {
			return "failed", "node_" + node.logicalNodeID + "_" + node.status
		}
	}
	return "succeeded", ""
}

func matchExistingTeamRecovery(
	events []journal.Event,
	input TeamRecoveryInput,
) (bool, bool, error) {
	matchIndex := -1
	var payload teamRecoveryPayload
	for index, event := range events {
		if event.Type != "TeamNodeRecoveryRecorded" {
			continue
		}
		var candidate teamRecoveryPayload
		if err := decodeExactPayload(event.PayloadJSON, &candidate); err != nil {
			return false, false, ErrTeamExecutionConflict
		}
		if candidate.LogicalNodeID != input.LogicalNodeID ||
			candidate.AttemptNumber != input.AttemptNumber {
			continue
		}
		if matchIndex >= 0 {
			return false, true, ErrTeamExecutionConflict
		}
		matchIndex = index
		payload = candidate
	}
	if matchIndex < 0 {
		return false, false, nil
	}
	decisionTime, err := time.Parse(time.RFC3339Nano, payload.DecisionTime)
	if err != nil || decisionTime.Location() != time.UTC {
		return false, true, ErrTeamExecutionConflict
	}
	priorTeam, err := replayTeamExecution(
		input.TeamInstanceID,
		events[:matchIndex],
	)
	if err != nil {
		return false, true, ErrTeamExecutionConflict
	}
	node := teamNodeByID(&priorTeam, input.LogicalNodeID)
	attempt := teamAttemptByNumber(node, input.AttemptNumber)
	if node == nil || attempt == nil ||
		node.currentAttempt != input.AttemptNumber ||
		node.status != "awaiting_recovery" {
		return false, true, ErrTeamExecutionConflict
	}
	prior := teamPriorClassifications(node, input.AttemptNumber)
	credits := node.semanticBinding.AttemptCredits -
		(input.AttemptNumber - 1)
	if credits < 0 {
		credits = 0
	}
	fallbackConsumed := teamFallbackConsumed(node)
	trigger := node.recoveryTrigger
	if trigger == "" {
		trigger = teamRecoveryTriggerOutput
	}
	decision, decisionErr := input.RecoveryPolicy.Decide(
		TeamRecoveryDecisionRequest{
			TeamInstanceID:           input.TeamInstanceID,
			PlanDigest:               input.PlanDigest,
			LogicalNodeID:            input.LogicalNodeID,
			AttemptNumber:            input.AttemptNumber,
			MaxAttempts:              input.MaxAttempts,
			AgentInstanceID:          input.AgentInstanceID,
			RuntimeInstanceID:        input.RuntimeInstanceID,
			EvidenceID:               input.EvidenceID,
			EvidenceDigest:           input.EvidenceDigest,
			OutputSummaryDigest:      input.OutputSummaryDigest,
			Classification:           input.Classification,
			PriorClassifications:     prior,
			RemainingCredits:         credits,
			FallbackConsumed:         fallbackConsumed,
			DecisionTime:             decisionTime,
			Trigger:                  trigger,
			AcceptanceDecisionDigest: node.acceptanceDecisionDigest,
		},
	)
	if decisionErr != nil ||
		!validExactTeamRecoveryDecision(decision) ||
		!decision.Valid() ||
		!decision.DecisionTime().Equal(decisionTime) ||
		!exactTeamRecoveryDecision(
			decision,
			input,
			prior,
			credits,
			fallbackConsumed,
			trigger,
			node.acceptanceDecisionDigest,
		) {
		return false, true, nil
	}
	expected, err := buildTeamRecoveryTransaction(
		priorTeam,
		input,
		decision,
	)
	if err != nil || len(events)-matchIndex < len(expected) {
		return false, true, ErrTeamExecutionConflict
	}
	for offset := range expected {
		if !exactJournalEvent(
			events[matchIndex+offset],
			expected[offset],
		) {
			return false, true, ErrTeamExecutionConflict
		}
	}
	return true, true, nil
}

func exactJournalEvent(left, right journal.Event) bool {
	return left.ID == right.ID &&
		left.StreamID == right.StreamID &&
		left.Seq == right.Seq &&
		left.IdempotencyKey == right.IdempotencyKey &&
		left.Type == right.Type &&
		left.SchemaVersion == right.SchemaVersion &&
		left.EmittedAt.Equal(right.EmittedAt) &&
		left.CorrelationID == right.CorrelationID &&
		left.CausationID == right.CausationID &&
		string(left.PayloadJSON) == string(right.PayloadJSON)
}

func validExactTeamRecoveryDecision(decision TeamRecoveryDecision) bool {
	decisionType := reflect.TypeOf(decision)
	return decisionType != nil &&
		decisionType.Kind() == reflect.Struct &&
		decisionType.PkgPath() == "loom-pi-rebuild/internal/rules" &&
		decisionType.Name() == "RecoveryDecision"
}

func exactTeamRecoveryDecision(
	decision TeamRecoveryDecision,
	input TeamRecoveryInput,
	prior []verification.OutputClassification,
	credits int,
	fallbackConsumed bool,
	trigger string,
	acceptanceDecisionDigest string,
) bool {
	return decision.TeamInstanceID() == input.TeamInstanceID &&
		decision.PlanDigest() == input.PlanDigest &&
		decision.LogicalNodeID() == input.LogicalNodeID &&
		decision.AttemptNumber() == input.AttemptNumber &&
		decision.MaxAttempts() == input.MaxAttempts &&
		decision.AgentInstanceID() == input.AgentInstanceID &&
		decision.RuntimeInstanceID() == input.RuntimeInstanceID &&
		decision.EvidenceID() == input.EvidenceID &&
		decision.EvidenceDigest() == input.EvidenceDigest &&
		decision.OutputSummaryDigest() == input.OutputSummaryDigest &&
		decision.ClassificationValue() == string(input.Classification.Kind()) &&
		decision.ClassificationDigest() == input.Classification.Digest() &&
		exactOutputClassificationStrings(
			prior,
			decision.PriorClassificationValues(),
		) &&
		decision.RemainingCredits() == credits &&
		decision.CreditsBefore() == credits &&
		decision.FallbackConsumed() == fallbackConsumed &&
		decision.PolicyVersion() == input.RecoveryPolicy.Version() &&
		decision.PolicyDigest() == input.RecoveryPolicy.Digest() &&
		decision.RecoveryApprovalRequired() ==
			input.RecoveryPolicy.RecoveryApprovalRequired() &&
		decision.TriggerValue() == trigger &&
		decision.AcceptanceDecisionDigest() == acceptanceDecisionDigest
}

func teamPriorClassifications(
	node *TeamNodeRecord,
	attemptNumber int,
) []verification.OutputClassification {
	if node == nil || attemptNumber <= 1 {
		return []verification.OutputClassification{}
	}
	result := make([]verification.OutputClassification, 0, attemptNumber-1)
	for _, attempt := range node.attempts {
		if attempt.attemptNumber >= attemptNumber {
			continue
		}
		result = append(result, attempt.outputClassification)
	}
	return result
}

func exactOutputClassificationStrings(
	left []verification.OutputClassification,
	right []string,
) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if string(left[index]) != right[index] {
			return false
		}
	}
	return true
}

func teamFallbackConsumed(node *TeamNodeRecord) bool {
	if node == nil || node.semanticBinding.WorkflowFallbackKey == "" {
		return false
	}
	for _, attempt := range node.attempts {
		if attempt.workflowPath == node.semanticBinding.WorkflowFallbackKey {
			return true
		}
	}
	return false
}

func isTerminalTeamStatus(status string) bool {
	switch status {
	case "succeeded", "failed", "degraded", "blocked", "human_required", "cancelled":
		return true
	default:
		return false
	}
}

func parseOptionalUTC(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || parsed.Location() != time.UTC {
		return time.Time{}, ErrInvalidTeamExecution
	}
	return parsed, nil
}

func cloneTeamNodeRecords(records []TeamNodeRecord) []TeamNodeRecord {
	cloned := make([]TeamNodeRecord, len(records))
	for index, record := range records {
		cloned[index] = record
		cloned[index].attempts = append([]TeamAttemptRecord(nil), record.attempts...)
		cloned[index].priorClassifications = append(
			[]verification.OutputClassification(nil),
			record.priorClassifications...,
		)
	}
	return cloned
}

func cloneTeamExecutionRecord(record TeamExecutionRecord) TeamExecutionRecord {
	record.nodes = cloneTeamNodeRecords(record.nodes)
	return record
}
