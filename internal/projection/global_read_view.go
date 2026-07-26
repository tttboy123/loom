package projection

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
	"time"

	"loom-pi-rebuild/internal/journal"
)

type TeamExecution struct {
	TeamInstanceID          string
	PlanDigest              string
	Status                  string
	Nodes                   []TeamExecutionNode
	LegacySemanticUnbound   bool
	LegacyAcceptanceUnbound bool
}

type TeamExecutionNode struct {
	LogicalNodeID               string
	Status                      string
	DependencySatisfied         bool
	CurrentAttempt              int
	RetryAt                     time.Time
	Attempts                    []TeamExecutionAttempt
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
	AcceptanceDecisionKind      string
	AcceptanceDecisionDigest    string
	AcceptanceDecisionTime      time.Time
	RecoveryTrigger             string
	RecoveryAction              string
	RecoveryDecisionDigest      string
	RecoveryDecisionTime        time.Time
	CreditsBefore               int
	CreditsAfter                int
	FallbackConsumed            bool
	PriorClassifications        []string
}

type TeamExecutionAttempt struct {
	AttemptNumber              int
	WorkItemID                 string
	RunID                      string
	ClaimID                    string
	ClaimGeneration            int64
	RuntimeInstanceID          string
	AgentInstanceID            string
	Status                     string
	EvidenceID                 string
	EvidenceDigest             string
	WorkflowPath               string
	OutputContractVersion      int
	OutputContractDigest       string
	OutputClassification       string
	OutputClassificationDigest string
	OutputSummaryDigest        string
}

type GlobalReadView struct {
	version          string
	heads            map[string]journal.StreamHead
	workItems        map[string]WorkItem
	runs             map[string]Run
	agentGrants      map[string]AgentGrant
	latestGrantByRun map[string]AgentGrant
	evidence         map[string]Evidence
	ruleSets         map[string]ProjectedRuleSet
	approvalRequests map[string]ProjectedApprovalRequest
	teams            map[string]TeamInstance
	agentInstances   map[string]AgentInstance
	runtimeInstances map[string]RuntimeInstance
	teamExecutions   map[string]TeamExecution
	activeRunCount   map[string]int
}

func (view GlobalReadView) Version() string { return view.version }

func (view GlobalReadView) Head(streamID string) (journal.StreamHead, bool) {
	head, ok := view.heads[streamID]
	return head, ok
}

func (view GlobalReadView) WorkItem(id string) (WorkItem, bool) {
	record, ok := view.workItems[id]
	return record, ok
}

func (view GlobalReadView) WorkItemsForTeam(teamID string) []WorkItem {
	records := make([]WorkItem, 0)
	if teamID == "" {
		return records
	}
	for _, record := range view.workItems {
		if record.TeamInstanceID == teamID {
			records = append(records, record)
		}
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].ID < records[j].ID
	})
	return records
}

func (view GlobalReadView) Run(id string) (Run, bool) {
	record, ok := view.runs[id]
	return record, ok
}

func (view GlobalReadView) AgentGrant(id string) (AgentGrant, bool) {
	record, ok := view.agentGrants[id]
	record.AllowedOperations = append([]string(nil), record.AllowedOperations...)
	return record, ok
}

func (view GlobalReadView) LatestAgentGrantForRun(runID string) (AgentGrant, bool) {
	record, ok := view.latestGrantByRun[runID]
	record.AllowedOperations = append([]string(nil), record.AllowedOperations...)
	return record, ok
}

func (view GlobalReadView) AgentGrantsForRun(runID string) []AgentGrant {
	records := make([]AgentGrant, 0)
	if runID == "" {
		return records
	}
	for _, record := range view.agentGrants {
		if record.RunID != runID {
			continue
		}
		record.AllowedOperations = append(
			[]string(nil),
			record.AllowedOperations...,
		)
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		if !records[i].IssuedAt.Equal(records[j].IssuedAt) {
			return records[i].IssuedAt.Before(records[j].IssuedAt)
		}
		return records[i].ID < records[j].ID
	})
	return records
}

func (view GlobalReadView) Evidence(id string) (Evidence, bool) {
	record, ok := view.evidence[id]
	return record, ok
}

func (view GlobalReadView) RuleSet(streamID string) (ProjectedRuleSet, bool) {
	record, ok := view.ruleSets[streamID]
	return cloneProjectedRuleSet(record), ok
}

func (view GlobalReadView) ApprovalRequest(
	id string,
) (ProjectedApprovalRequest, bool) {
	record, ok := view.approvalRequests[id]
	return cloneProjectedApprovalRequest(record), ok
}

func (view GlobalReadView) ApprovalRequestsForTeam(
	teamID string,
) []ProjectedApprovalRequest {
	records := make([]ProjectedApprovalRequest, 0)
	if teamID == "" {
		return records
	}
	for _, record := range view.approvalRequests {
		if record.TeamInstanceID == teamID {
			records = append(records, cloneProjectedApprovalRequest(record))
		}
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].ID < records[j].ID
	})
	return records
}

func (view GlobalReadView) Team(id string) (TeamInstance, bool) {
	record, ok := view.teams[id]
	return cloneProjectedTeamInstance(record), ok
}

func (view GlobalReadView) AgentInstance(id string) (AgentInstance, bool) {
	record, ok := view.agentInstances[id]
	return record, ok
}

func (view GlobalReadView) RuntimeInstance(id string) (RuntimeInstance, bool) {
	record, ok := view.runtimeInstances[id]
	return cloneProjectedRuntimeInstance(record), ok
}

func (view GlobalReadView) TeamExecution(id string) (TeamExecution, bool) {
	record, ok := view.teamExecutions[id]
	return cloneGlobalTeamExecution(record), ok
}

func (view GlobalReadView) ActiveRunCount(runtimeInstanceID string) int {
	return view.activeRunCount[runtimeInstanceID]
}

func buildGlobalReadView(
	snapshot Snapshot,
	events []journal.Event,
	teamExecutions map[string]TeamExecution,
) GlobalReadView {
	heads := make(map[string]journal.StreamHead)
	for _, event := range events {
		head, exists := heads[event.StreamID]
		if !exists || event.Seq > head.Sequence ||
			event.Seq == head.Sequence && event.ID < head.EventID {
			heads[event.StreamID] = journal.StreamHead{
				StreamID: event.StreamID,
				Sequence: event.Seq,
				EventID:  event.ID,
			}
		}
	}
	streamIDs := make([]string, 0, len(heads))
	for streamID := range heads {
		streamIDs = append(streamIDs, streamID)
	}
	sort.Strings(streamIDs)
	var canonical strings.Builder
	for _, streamID := range streamIDs {
		head := heads[streamID]
		canonical.WriteString(streamID)
		canonical.WriteByte(0)
		canonical.WriteString(strconv.FormatInt(head.Sequence, 10))
		canonical.WriteByte(0)
		canonical.WriteString(head.EventID)
		canonical.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(canonical.String()))
	cloned := snapshot.clone()
	active := make(map[string]int)
	for _, run := range cloned.Runs {
		if run.RuntimeInstanceID != "" &&
			(run.Phase == "claimed" || run.Phase == "running") {
			active[run.RuntimeInstanceID]++
		}
	}
	latestGrantByRun := make(map[string]AgentGrant)
	latestGrantSequence := make(map[string]int64)
	for _, event := range events {
		if event.Type != "AgentGrantIssued" ||
			!strings.HasPrefix(event.StreamID, "agent-grant/") {
			continue
		}
		var payload grantProjectionIssuePayload
		if decodeExactProjectionPayload(event, &payload) != nil ||
			payload.GrantID == nil ||
			payload.RunID == nil {
			continue
		}
		grant, exists := cloned.AgentGrants[*payload.GrantID]
		if !exists || grant.RunID != *payload.RunID {
			continue
		}
		if sequence, exists := latestGrantSequence[*payload.RunID]; exists &&
			sequence >= event.Seq {
			continue
		}
		grant.AllowedOperations = append(
			[]string(nil),
			grant.AllowedOperations...,
		)
		latestGrantByRun[*payload.RunID] = grant
		latestGrantSequence[*payload.RunID] = event.Seq
	}
	view := GlobalReadView{
		version:          hex.EncodeToString(sum[:]),
		heads:            heads,
		workItems:        cloned.WorkItems,
		runs:             cloned.Runs,
		agentGrants:      cloned.AgentGrants,
		latestGrantByRun: latestGrantByRun,
		evidence:         cloned.Evidence,
		ruleSets:         cloned.RuleSets,
		approvalRequests: cloned.ApprovalRequests,
		teams:            cloned.Teams,
		agentInstances:   cloned.AgentInstances,
		runtimeInstances: cloned.RuntimeInstances,
		teamExecutions:   make(map[string]TeamExecution, len(teamExecutions)),
		activeRunCount:   active,
	}
	for id, record := range teamExecutions {
		view.teamExecutions[id] = cloneGlobalTeamExecution(record)
	}
	return view
}

func cloneGlobalTeamExecution(record TeamExecution) TeamExecution {
	record.Nodes = append([]TeamExecutionNode(nil), record.Nodes...)
	for index := range record.Nodes {
		record.Nodes[index].PriorClassifications = append(
			[]string(nil),
			record.Nodes[index].PriorClassifications...,
		)
		record.Nodes[index].Attempts = append(
			[]TeamExecutionAttempt(nil),
			record.Nodes[index].Attempts...,
		)
	}
	return record
}
