package projection

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"loom-pi-rebuild/internal/assets"
	"loom-pi-rebuild/internal/journal"
)

type TeamExecution struct {
	TeamInstanceID          string
	PlanDigest              string
	Status                  string
	Nodes                   []TeamExecutionNode
	AssetLineageAvailable   bool
	LegacySemanticUnbound   bool
	LegacyAcceptanceUnbound bool
}

type TeamExecutionNode struct {
	LogicalNodeID               string
	Title                       string
	AgentInstanceID             string
	RuntimeInstanceID           string
	Role                        string
	DependsOn                   []string
	MaxAttempts                 int
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
	AssetLineageAvailable       bool
	AssetRevisionBindings       []assets.ExactAssetRevisionBinding
	AssetRevisionSetDigest      string
}

type TeamExecutionAttempt struct {
	AttemptNumber                 int
	WorkItemID                    string
	RunID                         string
	ClaimID                       string
	ClaimGeneration               int64
	RuntimeInstanceID             string
	AgentInstanceID               string
	Status                        string
	EvidenceID                    string
	EvidenceDigest                string
	WorkflowPath                  string
	OutputContractVersion         int
	OutputContractDigest          string
	OutputClassification          string
	OutputClassificationDigest    string
	OutputSummaryDigest           string
	AssetLineageAvailable         bool
	AssetRevisionBindings         []assets.ExactAssetRevisionBinding
	AssetRevisionSetDigest        string
	MaterializationManifestDigest string
	MaterializationRootDigest     string
}

type TeamTimelineAnchor struct {
	TeamInstanceID string
	Kind           string
	Confirmed      bool
	Executable     bool
	ReadOnly       bool
}

type GlobalReadView struct {
	version                      string
	heads                        map[string]journal.StreamHead
	workItems                    map[string]WorkItem
	runs                         map[string]Run
	agentGrants                  map[string]AgentGrant
	latestGrantByRun             map[string]AgentGrant
	evidence                     map[string]Evidence
	ruleSets                     map[string]ProjectedRuleSet
	approvalRequests             map[string]ProjectedApprovalRequest
	teams                        map[string]TeamInstance
	agentInstances               map[string]AgentInstance
	runtimeInstances             map[string]RuntimeInstance
	teamDefinitions              map[string]TeamDefinitionRecord
	providerCredentials          map[string]ProviderCredentialRecord
	teamExecutions               map[string]TeamExecution
	sideTaskHandoffs             map[string]SideTaskHandoff
	activeRunCount               map[string]int
	evolutionAssetDefinitions    map[string]assets.SkillDefinition
	evolutionAssetRevisions      map[string]assets.SkillRevision
	evolutionAssetCandidates     map[string]assets.EvolutionCandidate
	evolutionAssetEvaluations    map[string]assets.EvaluationRecord
	evolutionAssetBindings       map[string]assets.EvolutionAssetBindingRecord
	runtimeSkillMaterializations map[string]assets.RuntimeSkillMaterializationRecord
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
	record.AssetRevisionBindings = append(
		[]ProjectedAssetRevisionBinding(nil),
		record.AssetRevisionBindings...,
	)
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

func (view GlobalReadView) TeamDefinition(
	id string,
) (TeamDefinitionRecord, bool) {
	record, ok := view.teamDefinitions[id]
	return cloneTeamDefinitionRecord(record), ok
}

func (view GlobalReadView) ProviderCredential(
	providerID string,
) (ProviderCredentialRecord, bool) {
	record, ok := view.providerCredentials[providerID]
	return record, ok
}

func (view GlobalReadView) TeamDefinitions(
	afterID string,
	limit int,
) ([]TeamDefinitionRecord, bool) {
	ids, ok := globalReadPageIDs(view.teamDefinitions, afterID, limit)
	if !ok {
		return []TeamDefinitionRecord{}, false
	}
	records := make([]TeamDefinitionRecord, len(ids))
	for index, id := range ids {
		records[index] = cloneTeamDefinitionRecord(view.teamDefinitions[id])
	}
	return records, globalReadPageHasMore(
		view.teamDefinitions,
		ids,
		afterID,
	)
}

func (view GlobalReadView) TeamExecution(id string) (TeamExecution, bool) {
	record, ok := view.teamExecutions[id]
	return cloneGlobalTeamExecution(record), ok
}

func (view GlobalReadView) SideTaskHandoff(id string) (SideTaskHandoff, bool) {
	record, ok := view.sideTaskHandoffs[id]
	return cloneGlobalSideTaskHandoff(record), ok
}

func (view GlobalReadView) SideTaskHandoffs(
	parentMissionID string,
	limit int,
) ([]SideTaskHandoff, bool) {
	if limit <= 0 {
		return []SideTaskHandoff{}, len(view.sideTaskHandoffs) > 0
	}
	records := make([]SideTaskHandoff, 0, len(view.sideTaskHandoffs))
	for _, record := range view.sideTaskHandoffs {
		if parentMissionID == "" || record.ParentMissionID == parentMissionID {
			records = append(records, cloneGlobalSideTaskHandoff(record))
		}
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].SideTaskID < records[j].SideTaskID
	})
	hasMore := len(records) > limit
	if hasMore {
		records = records[:limit]
	}
	return records, hasMore
}

func (view GlobalReadView) Teams(
	afterID string,
	limit int,
) ([]TeamInstance, bool) {
	ids, ok := globalReadPageIDs(view.teams, afterID, limit)
	if !ok {
		return []TeamInstance{}, false
	}
	records := make([]TeamInstance, len(ids))
	for index, id := range ids {
		records[index] = cloneProjectedTeamInstance(view.teams[id])
	}
	return records, globalReadPageHasMore(view.teams, ids, afterID)
}

func (view GlobalReadView) Runs(afterID string, limit int) ([]Run, bool) {
	ids, ok := globalReadPageIDs(view.runs, afterID, limit)
	if !ok {
		return []Run{}, false
	}
	records := make([]Run, len(ids))
	for index, id := range ids {
		records[index] = view.runs[id]
	}
	return records, globalReadPageHasMore(view.runs, ids, afterID)
}

func (view GlobalReadView) EvidenceRecords(
	afterID string,
	limit int,
) ([]Evidence, bool) {
	ids, ok := globalReadPageIDs(view.evidence, afterID, limit)
	if !ok {
		return []Evidence{}, false
	}
	records := make([]Evidence, len(ids))
	for index, id := range ids {
		records[index] = view.evidence[id]
	}
	return records, globalReadPageHasMore(view.evidence, ids, afterID)
}

func (view GlobalReadView) RuntimeInstances(
	afterID string,
	limit int,
) ([]RuntimeInstance, bool) {
	ids, ok := globalReadPageIDs(view.runtimeInstances, afterID, limit)
	if !ok {
		return []RuntimeInstance{}, false
	}
	records := make([]RuntimeInstance, len(ids))
	for index, id := range ids {
		records[index] = cloneProjectedRuntimeInstance(view.runtimeInstances[id])
	}
	return records, globalReadPageHasMore(view.runtimeInstances, ids, afterID)
}

func (view GlobalReadView) TeamExecutions(
	afterID string,
	limit int,
) ([]TeamExecution, bool) {
	ids, ok := globalReadPageIDs(view.teamExecutions, afterID, limit)
	if !ok {
		return []TeamExecution{}, false
	}
	records := make([]TeamExecution, len(ids))
	for index, id := range ids {
		records[index] = cloneGlobalTeamExecution(view.teamExecutions[id])
	}
	return records, globalReadPageHasMore(view.teamExecutions, ids, afterID)
}

func (view GlobalReadView) TeamTimelineAnchor(
	teamInstanceID string,
) (TeamTimelineAnchor, bool) {
	if !validGlobalReadPageID(teamInstanceID) {
		return TeamTimelineAnchor{}, false
	}
	if team, ok := view.Team(teamInstanceID); ok &&
		team.ID == teamInstanceID {
		return TeamTimelineAnchor{
			TeamInstanceID: teamInstanceID,
			Kind:           "saved_team",
			Confirmed:      true,
			Executable:     true,
		}, true
	}
	execution, ok := view.TeamExecution(teamInstanceID)
	if !ok ||
		execution.TeamInstanceID != teamInstanceID ||
		(execution.Status != "succeeded" && execution.Status != "failed") ||
		len(execution.Nodes) == 0 {
		return TeamTimelineAnchor{}, false
	}
	hasAttempt := false
	for _, node := range execution.Nodes {
		if len(node.Attempts) > 0 {
			hasAttempt = true
			break
		}
	}
	head, ok := view.Head("team-execution/" + teamInstanceID)
	if !hasAttempt || !ok || head.Sequence <= 0 || head.EventID == "" {
		return TeamTimelineAnchor{}, false
	}
	return TeamTimelineAnchor{
		TeamInstanceID: teamInstanceID,
		Kind:           "historical_execution_only",
		ReadOnly:       true,
	}, true
}

func (view GlobalReadView) ActiveRunCount(runtimeInstanceID string) int {
	return view.activeRunCount[runtimeInstanceID]
}

func (view GlobalReadView) EvolutionAssetDefinition(id string) (assets.SkillDefinition, bool) {
	record, ok := view.evolutionAssetDefinitions[id]
	return record, ok
}

func (view GlobalReadView) EvolutionAssetDefinitions(afterID string, limit int) ([]assets.SkillDefinition, bool) {
	ids, ok := globalReadPageIDs(view.evolutionAssetDefinitions, afterID, limit)
	if !ok {
		return []assets.SkillDefinition{}, false
	}
	records := make([]assets.SkillDefinition, len(ids))
	for index, id := range ids {
		records[index] = view.evolutionAssetDefinitions[id]
	}
	return records, globalReadPageHasMore(view.evolutionAssetDefinitions, ids, afterID)
}

func (view GlobalReadView) EvolutionAssetRevision(id string) (assets.SkillRevision, bool) {
	record, ok := view.evolutionAssetRevisions[id]
	record.Dependencies = append([]string(nil), record.Dependencies...)
	record.CompatibleRuntimeCapabilities = append([]string(nil), record.CompatibleRuntimeCapabilities...)
	return record, ok
}
func (view GlobalReadView) EvolutionAssetRevisions(afterID string, limit int) ([]assets.SkillRevision, bool) {
	ids, ok := globalReadPageIDs(view.evolutionAssetRevisions, afterID, limit)
	if !ok {
		return []assets.SkillRevision{}, false
	}
	records := make([]assets.SkillRevision, 0, len(ids))
	for _, id := range ids {
		record, _ := view.EvolutionAssetRevision(id)
		records = append(records, record)
	}
	return records, globalReadPageHasMore(view.evolutionAssetRevisions, ids, afterID)
}
func (view GlobalReadView) EvolutionAssetCandidate(id string) (assets.EvolutionCandidate, bool) {
	record, ok := view.evolutionAssetCandidates[id]
	record.SourceEvidenceIDs = append([]string(nil), record.SourceEvidenceIDs...)
	record.SourceEvidenceDigests = append([]string(nil), record.SourceEvidenceDigests...)
	record.RequiredEvaluationIDs = append([]string(nil), record.RequiredEvaluationIDs...)
	return record, ok
}
func (view GlobalReadView) EvolutionAssetCandidates(afterID string, limit int) ([]assets.EvolutionCandidate, bool) {
	ids, ok := globalReadPageIDs(view.evolutionAssetCandidates, afterID, limit)
	if !ok {
		return []assets.EvolutionCandidate{}, false
	}
	records := make([]assets.EvolutionCandidate, 0, len(ids))
	for _, id := range ids {
		record, _ := view.EvolutionAssetCandidate(id)
		records = append(records, record)
	}
	return records, globalReadPageHasMore(view.evolutionAssetCandidates, ids, afterID)
}
func (view GlobalReadView) EvolutionAssetEvaluation(id string) (assets.EvaluationRecord, bool) {
	record, ok := view.evolutionAssetEvaluations[id]
	return record, ok
}
func (view GlobalReadView) EvolutionAssetEvaluations(afterID string, limit int) ([]assets.EvaluationRecord, bool) {
	ids, ok := globalReadPageIDs(view.evolutionAssetEvaluations, afterID, limit)
	if !ok {
		return []assets.EvaluationRecord{}, false
	}
	records := make([]assets.EvaluationRecord, len(ids))
	for index, id := range ids {
		records[index] = view.evolutionAssetEvaluations[id]
	}
	return records, globalReadPageHasMore(view.evolutionAssetEvaluations, ids, afterID)
}
func (view GlobalReadView) EvolutionAssetBinding(id string) (assets.EvolutionAssetBindingRecord, bool) {
	record, ok := view.evolutionAssetBindings[id]
	record.Bindings = append([]assets.ExactAssetRevisionBinding(nil), record.Bindings...)
	return record, ok
}
func (view GlobalReadView) EvolutionAssetBindings(afterID string, limit int) ([]assets.EvolutionAssetBindingRecord, bool) {
	ids, ok := globalReadPageIDs(view.evolutionAssetBindings, afterID, limit)
	if !ok {
		return []assets.EvolutionAssetBindingRecord{}, false
	}
	records := make([]assets.EvolutionAssetBindingRecord, 0, len(ids))
	for _, id := range ids {
		record, _ := view.EvolutionAssetBinding(id)
		records = append(records, record)
	}
	return records, globalReadPageHasMore(view.evolutionAssetBindings, ids, afterID)
}
func (view GlobalReadView) RuntimeSkillMaterialization(id string) (assets.RuntimeSkillMaterializationRecord, bool) {
	record, ok := view.runtimeSkillMaterializations[id]
	record.AssetRevisionBindings = append(
		[]assets.ExactAssetRevisionBinding(nil), record.AssetRevisionBindings...,
	)
	return record, ok
}
func (view GlobalReadView) RuntimeSkillMaterializations(afterID string, limit int) ([]assets.RuntimeSkillMaterializationRecord, bool) {
	ids, ok := globalReadPageIDs(view.runtimeSkillMaterializations, afterID, limit)
	if !ok {
		return []assets.RuntimeSkillMaterializationRecord{}, false
	}
	records := make([]assets.RuntimeSkillMaterializationRecord, len(ids))
	for index, id := range ids {
		records[index], _ = view.RuntimeSkillMaterialization(id)
	}
	return records, globalReadPageHasMore(view.runtimeSkillMaterializations, ids, afterID)
}

func globalReadPageIDs[T any](
	records map[string]T,
	afterID string,
	limit int,
) ([]string, bool) {
	if limit < 1 || limit > 64 ||
		afterID != "" && !validGlobalReadPageID(afterID) {
		return []string{}, false
	}
	ids := make([]string, 0, len(records))
	for id := range records {
		if id > afterID {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if len(ids) > limit {
		ids = ids[:limit]
	}
	return ids, true
}

func globalReadPageHasMore[T any](
	records map[string]T,
	pageIDs []string,
	afterID string,
) bool {
	lastID := afterID
	if len(pageIDs) > 0 {
		lastID = pageIDs[len(pageIDs)-1]
	}
	for id := range records {
		if id > lastID {
			return true
		}
	}
	return false
}

func validGlobalReadPageID(value string) bool {
	if value == "" || len(value) > 512 ||
		!utf8.ValidString(value) ||
		strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func buildGlobalReadView(
	snapshot Snapshot,
	events []journal.Event,
	teamExecutions map[string]TeamExecution,
) (GlobalReadView, error) {
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
	sideTaskHandoffs, err := projectSideTaskHandoffs(events)
	if err != nil {
		return GlobalReadView{}, err
	}
	view := GlobalReadView{
		version:                      hex.EncodeToString(sum[:]),
		heads:                        heads,
		workItems:                    cloned.WorkItems,
		runs:                         cloned.Runs,
		agentGrants:                  cloned.AgentGrants,
		latestGrantByRun:             latestGrantByRun,
		evidence:                     cloned.Evidence,
		ruleSets:                     cloned.RuleSets,
		approvalRequests:             cloned.ApprovalRequests,
		teams:                        cloned.Teams,
		agentInstances:               cloned.AgentInstances,
		runtimeInstances:             cloned.RuntimeInstances,
		teamDefinitions:              cloned.TeamDefinitions,
		providerCredentials:          cloned.ProviderCredentials,
		teamExecutions:               make(map[string]TeamExecution, len(teamExecutions)),
		sideTaskHandoffs:             sideTaskHandoffs,
		activeRunCount:               active,
		evolutionAssetDefinitions:    map[string]assets.SkillDefinition{},
		evolutionAssetRevisions:      map[string]assets.SkillRevision{},
		evolutionAssetCandidates:     map[string]assets.EvolutionCandidate{},
		evolutionAssetEvaluations:    map[string]assets.EvaluationRecord{},
		evolutionAssetBindings:       map[string]assets.EvolutionAssetBindingRecord{},
		runtimeSkillMaterializations: map[string]assets.RuntimeSkillMaterializationRecord{},
	}
	if cloned.EvolutionAssets != nil {
		assetSnapshot := cloneEvolutionAssetSnapshot(*cloned.EvolutionAssets)
		view.evolutionAssetDefinitions = assetSnapshot.Definitions
		view.evolutionAssetRevisions = assetSnapshot.Revisions
		view.evolutionAssetCandidates = assetSnapshot.Candidates
		view.evolutionAssetEvaluations = assetSnapshot.Evaluations
		view.evolutionAssetBindings = assetSnapshot.Bindings
		view.runtimeSkillMaterializations = assetSnapshot.Materializations
	}
	for id, record := range teamExecutions {
		clonedTeam := cloneGlobalTeamExecution(record)
		view.teamExecutions[id] = clonedTeam
		for _, node := range clonedTeam.Nodes {
			for _, attempt := range node.Attempts {
				run, exists := view.runs[attempt.RunID]
				if !exists || !attempt.AssetLineageAvailable {
					continue
				}
				run.AssetLineageAvailable = true
				run.AssetRevisionBindings = projectedRunAssetBindings(
					attempt.AssetRevisionBindings,
				)
				run.AssetRevisionSetDigest = attempt.AssetRevisionSetDigest
				run.MaterializationManifestDigest =
					attempt.MaterializationManifestDigest
				run.MaterializationRootDigest = attempt.MaterializationRootDigest
				view.runs[attempt.RunID] = run
			}
		}
	}
	return view, nil
}

func cloneGlobalTeamExecution(record TeamExecution) TeamExecution {
	record.Nodes = append([]TeamExecutionNode(nil), record.Nodes...)
	for index := range record.Nodes {
		record.Nodes[index].DependsOn = append(
			[]string(nil),
			record.Nodes[index].DependsOn...,
		)
		record.Nodes[index].PriorClassifications = append(
			[]string(nil),
			record.Nodes[index].PriorClassifications...,
		)
		record.Nodes[index].Attempts = append(
			[]TeamExecutionAttempt(nil),
			record.Nodes[index].Attempts...,
		)
		record.Nodes[index].AssetRevisionBindings = append(
			[]assets.ExactAssetRevisionBinding(nil),
			record.Nodes[index].AssetRevisionBindings...,
		)
		for attemptIndex := range record.Nodes[index].Attempts {
			record.Nodes[index].Attempts[attemptIndex].AssetRevisionBindings = append(
				[]assets.ExactAssetRevisionBinding(nil),
				record.Nodes[index].Attempts[attemptIndex].AssetRevisionBindings...,
			)
		}
	}
	return record
}
