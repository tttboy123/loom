package projection

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"loom-pi-rebuild/internal/journal"
)

var (
	ErrConflictingEvent        = errors.New("conflicting projection event")
	ErrSequenceGap             = errors.New("projection event sequence gap")
	ErrUnsupportedEventVersion = errors.New("unsupported projection event schema version")
	ErrInvalidProjectionEvent  = errors.New("invalid projection event")
)

type source interface {
	Events(context.Context) ([]journal.Event, error)
}

type journalSource struct {
	db *sql.DB
}

func newJournalSource(db *sql.DB) journalSource {
	return journalSource{db: db}
}

func (s journalSource) Events(ctx context.Context) ([]journal.Event, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, stream_id, seq, idempotency_key, event_type, schema_version,
		       emitted_at, correlation_id, causation_id, payload_json
		FROM events
		ORDER BY stream_id ASC, seq ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []journal.Event
	for rows.Next() {
		event, err := scanJournalEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

type Snapshot struct {
	Modes                        map[string]string
	WorkItems                    map[string]WorkItem
	Runs                         map[string]Run
	AgentGrants                  map[string]AgentGrant
	Evidence                     map[string]Evidence
	RuleSets                     map[string]ProjectedRuleSet
	ApprovalRequests             map[string]ProjectedApprovalRequest
	Teams                        map[string]TeamInstance
	AgentInstances               map[string]AgentInstance
	RuntimeInstances             map[string]RuntimeInstance
	MissionFallbackDecisions     map[string]MissionFallbackDecisionRecord
	ProviderAccountPolicies      map[string]ProviderAccountPolicyRecord
	ProviderModelRateCards       map[string]ProviderModelRateCardRecord
	RemoteToolBackendEnrollments map[string]RemoteToolBackendEnrollmentRecord
	EvolutionAssets              *EvolutionAssetSnapshot
	localProductSetupSnapshotFields
}

type WorkItem struct {
	ID                        string
	Title                     string
	Status                    string
	RunID                     string
	AgentInstanceID           string
	VerificationStatus        string
	TeamInstanceID            string
	PlanDigest                string
	LogicalNodeID             string
	AttemptNumber             int
	SourceEvidenceID          string
	SourceEvidenceDigest      string
	OutputSummaryDigest       string
	AcceptanceContractVersion int
	AcceptanceContractDigest  string
	AcceptanceRisk            string
	DeterministicResultDigest string
	VerifierRequired          bool
	VerifierWorkItemID        string
	VerifierRunID             string
	VerifierAgentInstanceID   string
	VerifierRuntimeInstanceID string
	VerifierEvidenceID        string
	VerifierEvidenceDigest    string
	VerifierCandidateDigest   string
	AcceptanceDecisionKind    string
	AcceptanceDecisionDigest  string
	AcceptanceDecisionReason  string
	AcceptanceDecisionTime    time.Time
	RecoveryTrigger           string
	RecoveryPolicyVersion     int
	RecoveryPolicyDigest      string
}

type Run struct {
	ID                                 string
	WorkItemID                         string
	Phase                              string
	ClaimID                            string
	ClaimGeneration                    int64
	RuntimeInstanceID                  string
	AgentInstanceID                    string
	ExecutionBindingAvailable          bool
	ExecutionBinding                   projectedFrozenExecutionBinding
	PrepareLeaseExpiresAt              time.Time
	TerminalStatus                     string
	TerminalReason                     string
	AccountingAvailable                bool
	Accounting                         RunAccounting
	ProviderAccountPolicyAvailable     bool
	ProviderAccountPolicyVersion       int
	ProviderAccountPolicyRevision      int64
	ProviderAccountPolicyDigest        string
	ProviderAccountTrustDomain         string
	ProviderAccountRetentionMode       string
	ProviderAccountDataRegion          string
	ProviderAccountAssignedBudgetUnits int64
	ProviderModelRateCardAvailable     bool
	ProviderModelRateCard              ProjectedProviderModelRateCard
	AssetLineageAvailable              bool
	AssetRevisionBindings              []ProjectedAssetRevisionBinding
	AssetRevisionSetDigest             string
	MaterializationManifestDigest      string
	MaterializationRootDigest          string
}

type RunAccounting struct {
	UsageObserved    bool
	InputTokens      int64
	OutputTokens     int64
	CacheReadTokens  int64
	CacheWriteTokens int64
	TotalTokens      int64
	CostObserved     bool
	CostMicrounits   int64
	CostCurrency     string
	CostSource       string
}

// ProjectedAssetRevisionBinding is the Projection-owned immutable wire copy
// of exact execution asset lineage. Keeping this local avoids making the core
// replay package depend on the evolution authority package.
type ProjectedAssetRevisionBinding struct {
	AssetKind    string
	DefinitionID string
	RevisionID   string
	SHA256Digest string
	SourceScope  string
}

type AgentGrant struct {
	ID                string
	WorkItemID        string
	RunID             string
	ClaimID           string
	ClaimGeneration   int64
	RuntimeInstanceID string
	AgentInstanceID   string
	AllowedOperations []string
	IssuedAt          time.Time
	ExpiresAt         time.Time
	RevokedAt         time.Time
	RevocationReason  string
}

type Evidence struct {
	ID         string
	WorkItemID string
	Digest     string
}

type ScopeIdentity struct {
	ProjectID    string `json:"project_id"`
	GenerationID string `json:"generation_id"`
}

type DormantSubAgent struct {
	Dormant           bool   `json:"dormant"`
	AgentDefinitionID string `json:"agent_definition_id"`
	RuntimeProfileID  string `json:"runtime_profile_id"`
	RuntimeInstanceID string `json:"runtime_instance_id"`
}

type TeamInstance struct {
	ID                    string
	WorkRequestID         string
	SourceKind            string
	TeamDefinitionID      string
	TeamDefinitionVersion int
	TeamDefinitionScope   string
	ScopeIdentity         ScopeIdentity
	TeamDefinitionDigest  string
	SourcePlanDigest      string
	SourceRecordSetDigest string
	State                 string
	CreatedAt             int64
	DormantSubAgents      []DormantSubAgent
	TeamInstanceCount     int
	AgentInstanceCount    int
	ActiveSubAgentCount   int
	WorkItemCount         int
	CreationEventID       string
}

type RuntimeBinding struct {
	Accepted   bool   `json:"accepted"`
	ProfileID  string `json:"profile_id"`
	InstanceID string `json:"instance_id"`
}

type AgentInstance struct {
	ID                     string
	TeamInstanceID         string
	WorkRequestID          string
	AgentDefinitionID      string
	AgentDefinitionVersion int
	AgentDefinitionScope   string
	ScopeIdentity          ScopeIdentity
	RuntimeProfileID       string
	RuntimeInstanceID      string
	IsMain                 bool
	State                  string
	RuntimeBinding         RuntimeBinding
	SourcePlanDigest       string
	SourceRecordSetDigest  string
	TeamCreatedAt          int64
	BindingDigest          string
	RuntimeDiscoveryDigest string
	CreationEventID        string
	TeamCreationEventID    string
}

type Projection struct {
	mu          sync.RWMutex
	source      source
	snapshot    Snapshot
	view        GlobalReadView
	rebuildGate chan struct{}
}

func New(db *sql.DB) *Projection {
	snapshot := emptySnapshot()
	view, err := buildGlobalReadView(snapshot, nil, nil)
	if err != nil {
		panic(err)
	}
	return &Projection{
		source:      newJournalSource(db),
		snapshot:    snapshot,
		view:        view,
		rebuildGate: make(chan struct{}, 1),
	}
}

func (p *Projection) Rebuild(ctx context.Context) error {
	select {
	case p.rebuildGate <- struct{}{}:
		defer func() { <-p.rebuildGate }()
	case <-ctx.Done():
		return ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	events, err := p.source.Events(ctx)
	if err != nil {
		return err
	}
	candidate, err := replay(ctx, events)
	if err != nil {
		return fmt.Errorf("projection replay: %w", err)
	}
	teamExecutions, err := projectTeamExecutions(events)
	if err != nil {
		return fmt.Errorf("team execution projection: %w", err)
	}
	if _, err := projectSideTaskHandoffs(events); err != nil {
		return fmt.Errorf("side-task projection: %w", err)
	}
	candidateView, err := buildGlobalReadView(candidate, events, teamExecutions)
	if err != nil {
		return fmt.Errorf("global read view: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.snapshot = candidate.clone()
	p.view = candidateView
	return nil
}

func (p *Projection) Snapshot() Snapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.snapshot.clone()
}

func (p *Projection) GlobalReadView() GlobalReadView {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.view
}

func replay(ctx context.Context, events []journal.Event) (Snapshot, error) {
	ordered := append([]journal.Event(nil), events...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].StreamID != ordered[j].StreamID {
			return ordered[i].StreamID < ordered[j].StreamID
		}
		if ordered[i].Seq != ordered[j].Seq {
			return ordered[i].Seq < ordered[j].Seq
		}
		return ordered[i].ID < ordered[j].ID
	})

	candidate := emptySnapshot()
	seenByID := make(map[string]journal.Event)
	seenByStreamSeq := make(map[streamSeq]journal.Event)
	nextSeq := make(map[string]int64)
	var runAuthorityEvents []journal.Event
	var grantAuthorityEvents []journal.Event
	var evidenceEvents []journal.Event
	var approvalEvents []journal.Event
	var evolutionAssetEvents []journal.Event

	for _, event := range ordered {
		if err := ctx.Err(); err != nil {
			return Snapshot{}, err
		}
		if !supportedProjectionEnvelopeVersion(event) {
			return Snapshot{}, fmt.Errorf("%w: %d", ErrUnsupportedEventVersion, event.SchemaVersion)
		}
		if existing, ok := seenByID[event.ID]; ok {
			if !sameImmutableEvent(existing, event) {
				return Snapshot{}, fmt.Errorf("%w: id %s", ErrConflictingEvent, event.ID)
			}
		}
		key := streamSeq{streamID: event.StreamID, seq: event.Seq}
		if existing, ok := seenByStreamSeq[key]; ok {
			if !sameImmutableEvent(existing, event) {
				return Snapshot{}, fmt.Errorf("%w: stream %s seq %d", ErrConflictingEvent, event.StreamID, event.Seq)
			}
		}
		if existing, ok := seenByID[event.ID]; ok && sameImmutableEvent(existing, event) {
			continue
		}

		wantSeq := nextSeq[event.StreamID] + 1
		if event.Seq != wantSeq {
			return Snapshot{}, fmt.Errorf("%w: stream %s seq %d want %d", ErrSequenceGap, event.StreamID, event.Seq, wantSeq)
		}
		seenByID[event.ID] = event
		seenByStreamSeq[key] = event
		nextSeq[event.StreamID] = event.Seq

		if isApprovalProjectionEvent(event) {
			approvalEvents = append(approvalEvents, event)
			continue
		}
		if isEvolutionAssetProjectionEvent(event) {
			evolutionAssetEvents = append(evolutionAssetEvents, event)
			continue
		}
		if isRunAuthorityProjectionEvent(event) {
			runAuthorityEvents = append(runAuthorityEvents, event)
			continue
		}
		if event.Type == "ProviderModelRateCardConfigured" {
			if err := applyProviderModelRateCardProjection(&candidate, event); err != nil {
				return Snapshot{}, err
			}
			continue
		}
		if isGrantAuthorityProjectionEvent(event) {
			grantAuthorityEvents = append(grantAuthorityEvents, event)
			continue
		}
		if isRunAuthorityRuntimeReferenceEvent(event) {
			runAuthorityEvents = append(runAuthorityEvents, event)
		}
		if event.Type == "EvidenceSubmitted" {
			evidenceEvents = append(evidenceEvents, event)
			continue
		}
		if err := candidate.apply(event); err != nil {
			return Snapshot{}, err
		}
	}
	normalizedRunEvents := runAuthorityEventsWithoutApprovalFacts(
		runAuthorityEvents,
	)
	if err := applyRunAuthorityProjection(
		ctx,
		&candidate,
		normalizedRunEvents,
		ordered,
	); err != nil {
		return Snapshot{}, fmt.Errorf("run authority: %w", err)
	}
	if err := applyGrantAuthorityProjection(
		ctx,
		&candidate,
		normalizedRunEvents,
		grantAuthorityEvents,
	); err != nil {
		return Snapshot{}, fmt.Errorf("grant authority: %w", err)
	}
	if err := applyApprovalProjection(
		ctx,
		&candidate,
		approvalEvents,
		ordered,
	); err != nil {
		return Snapshot{}, err
	}
	for _, event := range evidenceEvents {
		if err := candidate.apply(event); err != nil {
			return Snapshot{}, err
		}
	}
	if err := candidate.validateSavedTeamLinks(); err != nil {
		return Snapshot{}, err
	}
	if len(evolutionAssetEvents) > 0 {
		projected, err := replayEvolutionAssetEvents(evolutionAssetEvents)
		if err != nil {
			return Snapshot{}, fmt.Errorf("%w: evolution assets", ErrInvalidProjectionEvent)
		}
		candidate.EvolutionAssets = &projected
	}
	return candidate, nil
}

// The global read projection validates Journal identity and sequence for every
// stream, but dedicated authorities own their payload decoding. Schema v2 is
// admitted only for exact content-free execution and permission-decision facts;
// an unknown v2 event remains fail-closed.
func supportedProjectionEnvelopeVersion(event journal.Event) bool {
	if event.SchemaVersion == 1 {
		return true
	}
	if event.SchemaVersion != 2 {
		return false
	}
	if strings.HasPrefix(event.StreamID, "permission-decision/") {
		return event.Type == "PermissionDecisionRecorded"
	}
	if !strings.HasPrefix(event.StreamID, "execution/") {
		return false
	}
	switch event.Type {
	case "ToolExecutionProposed", "ToolExecutionDenied", "ToolExecutionFailed",
		"ToolExecutionRecoveryRequired", "ToolExecutionRecoveryResolved":
		return true
	default:
		return false
	}
}

func isEvolutionAssetProjectionEvent(event journal.Event) bool {
	return strings.HasPrefix(event.Type, "EvolutionAsset") ||
		event.Type == "EvolutionTemplateInstantiated" ||
		event.Type == "EvolutionRunPromotionProposed" ||
		strings.HasPrefix(event.Type, "RuntimeSkillMaterialization")
}

func (s *Snapshot) apply(event journal.Event) error {
	if isMissionFallbackDecisionProjectionEvent(event) {
		return applyMissionFallbackDecisionProjection(s, event)
	}
	if event.Type == "ProviderAccountPolicyConfigured" {
		return applyProviderAccountPolicyProjection(s, event)
	}
	if event.Type == "RemoteToolBackendEnrollmentConfigured" ||
		event.Type == "RemoteToolBackendEnrollmentRevoked" {
		return applyRemoteToolBackendEnrollmentProjection(s, event)
	}
	if isLocalProductSetupProjectionEvent(event) {
		ensureLocalProductSetupSnapshot(s)
		return applyLocalProductSetupProjection(*s, event)
	}
	switch event.Type {
	case "ModeSelected":
		var payload struct {
			Mode string `json:"mode"`
		}
		if err := decodeRelevantPayload(event, &payload); err != nil {
			return err
		}
		if payload.Mode != "conversation" && payload.Mode != "agent" {
			return fmt.Errorf("%w: invalid mode", ErrInvalidProjectionEvent)
		}
		if existing, ok := s.Modes[event.StreamID]; ok && existing != payload.Mode {
			return fmt.Errorf("%w: conflicting mode", ErrInvalidProjectionEvent)
		}
		s.Modes[event.StreamID] = payload.Mode
	case "WorkItemCreated":
		var payload struct {
			WorkItemID string `json:"work_item_id"`
			Title      string `json:"title"`
			Status     string `json:"status"`
		}
		if err := decodeRelevantPayload(event, &payload); err != nil {
			return err
		}
		if payload.WorkItemID == "" || payload.Title == "" || payload.Status == "" {
			return fmt.Errorf("%w: missing work item create field", ErrInvalidProjectionEvent)
		}
		workItem := WorkItem{ID: payload.WorkItemID, Title: payload.Title, Status: payload.Status}
		if existing, ok := s.WorkItems[payload.WorkItemID]; ok {
			if existing == workItem {
				return nil
			}
			return fmt.Errorf("%w: conflicting work item create", ErrInvalidProjectionEvent)
		}
		s.WorkItems[payload.WorkItemID] = workItem
	case "WorkItemTerminal":
		var payload struct {
			WorkItemID string `json:"work_item_id"`
			Status     string `json:"status"`
		}
		if err := decodeRelevantPayload(event, &payload); err != nil {
			return err
		}
		if payload.WorkItemID == "" || payload.Status == "" {
			return fmt.Errorf("%w: missing work item terminal field", ErrInvalidProjectionEvent)
		}
		workItem, ok := s.WorkItems[payload.WorkItemID]
		if !ok {
			return fmt.Errorf("%w: terminal before create", ErrInvalidProjectionEvent)
		}
		workItem.Status = payload.Status
		s.WorkItems[payload.WorkItemID] = workItem
	case "EvidenceSubmitted":
		var payload struct {
			EvidenceID string `json:"evidence_id"`
			WorkItemID string `json:"work_item_id"`
			Digest     string `json:"digest"`
		}
		if err := decodeRelevantPayload(event, &payload); err != nil {
			return err
		}
		if payload.EvidenceID == "" || payload.WorkItemID == "" || payload.Digest == "" {
			return fmt.Errorf("%w: missing evidence field", ErrInvalidProjectionEvent)
		}
		if !validSHA256Digest(payload.Digest) {
			return fmt.Errorf("%w: invalid evidence digest", ErrInvalidProjectionEvent)
		}
		if _, ok := s.WorkItems[payload.WorkItemID]; !ok {
			return fmt.Errorf("%w: evidence references unknown work item", ErrInvalidProjectionEvent)
		}
		evidence := Evidence{ID: payload.EvidenceID, WorkItemID: payload.WorkItemID, Digest: payload.Digest}
		if existing, ok := s.Evidence[payload.EvidenceID]; ok {
			if existing == evidence {
				return nil
			}
			return fmt.Errorf("%w: conflicting evidence", ErrInvalidProjectionEvent)
		}
		s.Evidence[payload.EvidenceID] = evidence
	case "TeamInstanceCreated":
		team, err := projectSavedTeamInstance(event)
		if err != nil {
			return err
		}
		if existing, ok := s.Teams[team.ID]; ok {
			if equalProjectedTeamInstance(existing, team) {
				return nil
			}
			return fmt.Errorf("%w: conflicting TeamInstance create", ErrInvalidProjectionEvent)
		}
		s.Teams[team.ID] = cloneProjectedTeamInstance(team)
	case "AgentInstanceCreated":
		agent, err := projectSavedTeamMainAgentInstance(event)
		if err != nil {
			return err
		}
		if existing, ok := s.AgentInstances[agent.ID]; ok {
			if existing == agent {
				return nil
			}
			return fmt.Errorf("%w: conflicting AgentInstance create", ErrInvalidProjectionEvent)
		}
		s.AgentInstances[agent.ID] = agent
	case "RuntimeInstanceDiscovered":
		instance, err := projectRuntimeDiscoveryEvent(event)
		if err != nil {
			return err
		}
		if existing, ok := s.RuntimeInstances[instance.ID]; ok &&
			(existing.DeviceID != instance.DeviceID ||
				existing.AdapterType != instance.AdapterType) {
			return fmt.Errorf("%w: conflicting RuntimeInstance identity", ErrInvalidProjectionEvent)
		}
		s.RuntimeInstances[instance.ID] = cloneProjectedRuntimeInstance(instance)
	case "RuntimeInstanceStatusChanged":
		existing, ok := s.RuntimeInstances[runtimeInstanceIDFromStream(event.StreamID)]
		if !ok {
			return fmt.Errorf("%w: status before RuntimeInstance discovery", ErrInvalidProjectionEvent)
		}
		instance, err := projectRuntimeStatusEvent(event, existing)
		if err != nil {
			return err
		}
		s.RuntimeInstances[instance.ID] = cloneProjectedRuntimeInstance(instance)
	default:
		return nil
	}
	return nil
}

type savedTeamInstanceCreatedPayload struct {
	Team                  savedTeamInstanceFactRecord `json:"team"`
	DormantSubAgents      *[]DormantSubAgent          `json:"dormant_sub_agents"`
	SourcePlanDigest      string                      `json:"source_plan_digest"`
	SourceRecordSetDigest string                      `json:"source_record_set_digest"`
	TeamInstanceCount     *int                        `json:"team_instance_count"`
	AgentInstanceCount    *int                        `json:"agent_instance_count"`
	ActiveSubAgentCount   *int                        `json:"active_sub_agent_count"`
	WorkItemCount         *int                        `json:"work_item_count"`
}

type savedTeamInstanceFactRecord struct {
	ID                    string                     `json:"id"`
	WorkRequestID         string                     `json:"work_request_id"`
	SourceKind            string                     `json:"source_kind"`
	TeamDefinitionID      string                     `json:"team_definition_id"`
	TeamDefinitionVersion int                        `json:"team_definition_version"`
	TeamDefinitionScope   string                     `json:"team_definition_scope"`
	ScopeIdentity         savedTeamScopeIdentityFact `json:"scope_identity"`
	TeamDefinitionDigest  string                     `json:"team_definition_digest"`
	SourcePlanDigest      string                     `json:"source_plan_digest"`
	State                 string                     `json:"state"`
	CreatedAt             int64                      `json:"created_at"`
}

type savedTeamMainAgentCreatedPayload struct {
	MainAgent              savedTeamMainAgentFactRecord `json:"main_agent"`
	RuntimeBinding         RuntimeBinding               `json:"runtime_binding"`
	SourcePlanDigest       string                       `json:"source_plan_digest"`
	SourceRecordSetDigest  string                       `json:"source_record_set_digest"`
	TeamCreatedAt          int64                        `json:"team_created_at"`
	BindingDigest          string                       `json:"binding_digest"`
	RuntimeDiscoveryDigest string                       `json:"runtime_discovery_digest"`
}

type savedTeamMainAgentFactRecord struct {
	ID                     string                     `json:"id"`
	TeamInstanceID         string                     `json:"team_instance_id"`
	AgentDefinitionID      string                     `json:"agent_definition_id"`
	AgentDefinitionVersion int                        `json:"agent_definition_version"`
	AgentDefinitionScope   string                     `json:"agent_definition_scope"`
	ScopeIdentity          savedTeamScopeIdentityFact `json:"scope_identity"`
	RuntimeProfileID       string                     `json:"runtime_profile_id"`
	RuntimeInstanceID      string                     `json:"runtime_instance_id"`
	IsMain                 bool                       `json:"is_main"`
	State                  string                     `json:"state"`
}

type savedTeamScopeIdentityFact struct {
	ProjectID    *string `json:"project_id"`
	GenerationID *string `json:"generation_id"`
}

func projectSavedTeamInstance(event journal.Event) (TeamInstance, error) {
	var payload savedTeamInstanceCreatedPayload
	if err := decodeExactProjectionPayload(event, &payload); err != nil {
		return TeamInstance{}, err
	}
	team := payload.Team
	scopeIdentity, scopePresent := projectedScopeIdentity(team.ScopeIdentity)
	invalidReason := ""
	switch {
	case !validSavedTeamProjectionEnvelope(event):
		invalidReason = "envelope"
	case event.Seq != 1 || event.CausationID != "":
		invalidReason = "stream position"
	case team.ID == "" || event.StreamID != "team_instance:"+team.ID:
		invalidReason = "team identity"
	case team.WorkRequestID == "" || event.CorrelationID != team.WorkRequestID:
		invalidReason = "work request correlation"
	case team.SourceKind != "saved_team" || team.TeamDefinitionID == "" ||
		team.TeamDefinitionVersion <= 0:
		invalidReason = "definition identity"
	case !scopePresent || !validProjectedScope(team.TeamDefinitionScope, scopeIdentity):
		invalidReason = "scope"
	case !validSHA256Digest(team.TeamDefinitionDigest):
		invalidReason = "definition digest"
	case !validSHA256Digest(team.SourcePlanDigest) ||
		!validSHA256Digest(payload.SourcePlanDigest) ||
		team.SourcePlanDigest != payload.SourcePlanDigest:
		invalidReason = "plan digest"
	case !validSHA256Digest(payload.SourceRecordSetDigest):
		invalidReason = "record set digest"
	case team.State != "created" || team.CreatedAt <= 0:
		invalidReason = "lifecycle"
	case payload.DormantSubAgents == nil ||
		payload.TeamInstanceCount == nil || payload.AgentInstanceCount == nil ||
		payload.ActiveSubAgentCount == nil || payload.WorkItemCount == nil:
		invalidReason = "required counters"
	case *payload.TeamInstanceCount != 1 || *payload.AgentInstanceCount != 1 ||
		*payload.ActiveSubAgentCount != 0 || *payload.WorkItemCount != 0:
		invalidReason = "counter values"
	case !validProjectedDormantSubAgents(*payload.DormantSubAgents):
		invalidReason = "dormant subagents"
	}
	if invalidReason != "" {
		return TeamInstance{}, fmt.Errorf(
			"%w: invalid saved TeamInstance %s",
			ErrInvalidProjectionEvent,
			invalidReason,
		)
	}
	return TeamInstance{
		ID:                    team.ID,
		WorkRequestID:         team.WorkRequestID,
		SourceKind:            team.SourceKind,
		TeamDefinitionID:      team.TeamDefinitionID,
		TeamDefinitionVersion: team.TeamDefinitionVersion,
		TeamDefinitionScope:   team.TeamDefinitionScope,
		ScopeIdentity:         scopeIdentity,
		TeamDefinitionDigest:  team.TeamDefinitionDigest,
		SourcePlanDigest:      team.SourcePlanDigest,
		SourceRecordSetDigest: payload.SourceRecordSetDigest,
		State:                 team.State,
		CreatedAt:             team.CreatedAt,
		DormantSubAgents:      cloneProjectedDormantSubAgents(*payload.DormantSubAgents),
		TeamInstanceCount:     *payload.TeamInstanceCount,
		AgentInstanceCount:    *payload.AgentInstanceCount,
		ActiveSubAgentCount:   *payload.ActiveSubAgentCount,
		WorkItemCount:         *payload.WorkItemCount,
		CreationEventID:       event.ID,
	}, nil
}

func projectSavedTeamMainAgentInstance(event journal.Event) (AgentInstance, error) {
	var payload savedTeamMainAgentCreatedPayload
	if err := decodeExactProjectionPayload(event, &payload); err != nil {
		return AgentInstance{}, err
	}
	main := payload.MainAgent
	scopeIdentity, scopePresent := projectedScopeIdentity(main.ScopeIdentity)
	if !validSavedTeamProjectionEnvelope(event) ||
		event.Seq != 1 ||
		event.CorrelationID == "" ||
		event.CausationID == "" ||
		main.ID == "" ||
		event.StreamID != "agent_instance:"+main.ID ||
		main.TeamInstanceID == "" ||
		main.AgentDefinitionID == "" ||
		main.AgentDefinitionVersion <= 0 ||
		!scopePresent ||
		!validProjectedScope(main.AgentDefinitionScope, scopeIdentity) ||
		main.RuntimeProfileID == "" ||
		main.RuntimeInstanceID == "" ||
		!main.IsMain ||
		main.State != "created" ||
		!payload.RuntimeBinding.Accepted ||
		payload.RuntimeBinding.ProfileID != main.RuntimeProfileID ||
		payload.RuntimeBinding.InstanceID != main.RuntimeInstanceID ||
		!validSHA256Digest(payload.SourcePlanDigest) ||
		!validSHA256Digest(payload.SourceRecordSetDigest) ||
		payload.TeamCreatedAt <= 0 ||
		!validSHA256Digest(payload.BindingDigest) ||
		!validSHA256Digest(payload.RuntimeDiscoveryDigest) {
		return AgentInstance{}, ErrInvalidProjectionEvent
	}
	return AgentInstance{
		ID:                     main.ID,
		TeamInstanceID:         main.TeamInstanceID,
		WorkRequestID:          event.CorrelationID,
		AgentDefinitionID:      main.AgentDefinitionID,
		AgentDefinitionVersion: main.AgentDefinitionVersion,
		AgentDefinitionScope:   main.AgentDefinitionScope,
		ScopeIdentity:          scopeIdentity,
		RuntimeProfileID:       main.RuntimeProfileID,
		RuntimeInstanceID:      main.RuntimeInstanceID,
		IsMain:                 main.IsMain,
		State:                  main.State,
		RuntimeBinding:         payload.RuntimeBinding,
		SourcePlanDigest:       payload.SourcePlanDigest,
		SourceRecordSetDigest:  payload.SourceRecordSetDigest,
		TeamCreatedAt:          payload.TeamCreatedAt,
		BindingDigest:          payload.BindingDigest,
		RuntimeDiscoveryDigest: payload.RuntimeDiscoveryDigest,
		CreationEventID:        event.ID,
		TeamCreationEventID:    event.CausationID,
	}, nil
}

func (s Snapshot) validateSavedTeamLinks() error {
	mainByTeam := make(map[string]int, len(s.Teams))
	for _, agent := range s.AgentInstances {
		team, ok := s.Teams[agent.TeamInstanceID]
		if !ok ||
			agent.WorkRequestID != team.WorkRequestID ||
			agent.SourcePlanDigest != team.SourcePlanDigest ||
			agent.SourceRecordSetDigest != team.SourceRecordSetDigest ||
			agent.TeamCreatedAt != team.CreatedAt ||
			agent.TeamCreationEventID != team.CreationEventID {
			return ErrInvalidProjectionEvent
		}
		mainByTeam[team.ID]++
		if mainByTeam[team.ID] > 1 {
			return ErrInvalidProjectionEvent
		}
	}
	for teamID := range s.Teams {
		if mainByTeam[teamID] != 1 {
			return ErrInvalidProjectionEvent
		}
	}
	return nil
}

func validSavedTeamProjectionEnvelope(event journal.Event) bool {
	return event.ID != "" &&
		event.IdempotencyKey != "" &&
		!event.EmittedAt.IsZero()
}

func projectedScopeIdentity(
	input savedTeamScopeIdentityFact,
) (ScopeIdentity, bool) {
	if input.ProjectID == nil || input.GenerationID == nil {
		return ScopeIdentity{}, false
	}
	return ScopeIdentity{
		ProjectID:    *input.ProjectID,
		GenerationID: *input.GenerationID,
	}, true
}

func validProjectedScope(scope string, identity ScopeIdentity) bool {
	switch scope {
	case "project":
		return identity.ProjectID != "" && identity.GenerationID == ""
	case "reusable":
		return identity.ProjectID == "" && identity.GenerationID == ""
	default:
		return false
	}
}

func validProjectedDormantSubAgents(input []DormantSubAgent) bool {
	if len(input) > maxProjectedTeamAgentCount-1 {
		return false
	}
	for index, current := range input {
		if !current.Dormant ||
			current.AgentDefinitionID == "" ||
			current.RuntimeProfileID == "" ||
			current.RuntimeInstanceID == "" {
			return false
		}
		if index > 0 {
			if input[index-1].AgentDefinitionID == current.AgentDefinitionID ||
				!lessProjectedDormantSubAgent(input[index-1], current) {
				return false
			}
		}
	}
	return true
}

func lessProjectedDormantSubAgent(left, right DormantSubAgent) bool {
	if left.AgentDefinitionID != right.AgentDefinitionID {
		return left.AgentDefinitionID < right.AgentDefinitionID
	}
	if left.RuntimeProfileID != right.RuntimeProfileID {
		return left.RuntimeProfileID < right.RuntimeProfileID
	}
	return left.RuntimeInstanceID < right.RuntimeInstanceID
}

func decodeExactProjectionPayload(event journal.Event, target any) error {
	if !json.Valid(event.PayloadJSON) ||
		hasDuplicateProjectionJSONKeys(event.PayloadJSON) {
		return fmt.Errorf("%w: malformed payload", ErrInvalidProjectionEvent)
	}
	decoder := json.NewDecoder(bytes.NewReader(event.PayloadJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidProjectionEvent, err)
	}
	return nil
}

func hasDuplicateProjectionJSONKeys(payload []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	return scanDuplicateProjectionJSONValue(decoder)
}

func scanDuplicateProjectionJSONValue(decoder *json.Decoder) bool {
	token, err := decoder.Token()
	if err != nil {
		return true
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return false
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return true
			}
			key, ok := keyToken.(string)
			if !ok {
				return true
			}
			if _, duplicate := seen[key]; duplicate {
				return true
			}
			seen[key] = struct{}{}
			if scanDuplicateProjectionJSONValue(decoder) {
				return true
			}
		}
		_, err = decoder.Token()
		return err != nil
	case '[':
		for decoder.More() {
			if scanDuplicateProjectionJSONValue(decoder) {
				return true
			}
		}
		_, err = decoder.Token()
		return err != nil
	default:
		return true
	}
}

func validSHA256Digest(digest string) bool {
	if len(digest) != 64 {
		return false
	}
	for _, ch := range digest {
		switch {
		case ch >= '0' && ch <= '9':
		case ch >= 'a' && ch <= 'f':
		default:
			return false
		}
	}
	return true
}

func decodeRelevantPayload(event journal.Event, target any) error {
	if !json.Valid(event.PayloadJSON) {
		return fmt.Errorf("%w: malformed payload", ErrInvalidProjectionEvent)
	}
	if err := json.Unmarshal(event.PayloadJSON, target); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidProjectionEvent, err)
	}
	return nil
}

func emptySnapshot() Snapshot {
	return Snapshot{
		Modes:            make(map[string]string),
		WorkItems:        make(map[string]WorkItem),
		Runs:             make(map[string]Run),
		AgentGrants:      make(map[string]AgentGrant),
		Evidence:         make(map[string]Evidence),
		RuleSets:         make(map[string]ProjectedRuleSet),
		ApprovalRequests: make(map[string]ProjectedApprovalRequest),
		Teams:            make(map[string]TeamInstance),
		AgentInstances:   make(map[string]AgentInstance),
		RuntimeInstances: make(map[string]RuntimeInstance),
	}
}

func (s Snapshot) clone() Snapshot {
	out := emptySnapshot()
	for id, mode := range s.Modes {
		out.Modes[id] = mode
	}
	for id, workItem := range s.WorkItems {
		out.WorkItems[id] = workItem
	}
	for id, run := range s.Runs {
		run.AssetRevisionBindings = append(
			[]ProjectedAssetRevisionBinding(nil),
			run.AssetRevisionBindings...,
		)
		run.ExecutionBinding = cloneProjectedExecutionBinding(run.ExecutionBinding)
		out.Runs[id] = run
	}
	for id, grant := range s.AgentGrants {
		grant.AllowedOperations = append([]string(nil), grant.AllowedOperations...)
		out.AgentGrants[id] = grant
	}
	for id, evidence := range s.Evidence {
		out.Evidence[id] = evidence
	}
	for id, ruleSet := range s.RuleSets {
		out.RuleSets[id] = cloneProjectedRuleSet(ruleSet)
	}
	for id, approval := range s.ApprovalRequests {
		out.ApprovalRequests[id] = cloneProjectedApprovalRequest(approval)
	}
	for id, team := range s.Teams {
		out.Teams[id] = cloneProjectedTeamInstance(team)
	}
	for id, agent := range s.AgentInstances {
		out.AgentInstances[id] = agent
	}
	if s.MissionFallbackDecisions != nil {
		out.MissionFallbackDecisions = make(
			map[string]MissionFallbackDecisionRecord,
			len(s.MissionFallbackDecisions),
		)
		for digest, decision := range s.MissionFallbackDecisions {
			out.MissionFallbackDecisions[digest] = decision
		}
	}
	if s.ProviderAccountPolicies != nil {
		out.ProviderAccountPolicies = make(
			map[string]ProviderAccountPolicyRecord,
			len(s.ProviderAccountPolicies),
		)
		for accountID, policy := range s.ProviderAccountPolicies {
			out.ProviderAccountPolicies[accountID] = policy
		}
	}
	if s.ProviderModelRateCards != nil {
		out.ProviderModelRateCards = make(
			map[string]ProviderModelRateCardRecord,
			len(s.ProviderModelRateCards),
		)
		for identity, rateCard := range s.ProviderModelRateCards {
			out.ProviderModelRateCards[identity] = rateCard
		}
	}
	if s.RemoteToolBackendEnrollments != nil {
		out.RemoteToolBackendEnrollments = make(
			map[string]RemoteToolBackendEnrollmentRecord,
			len(s.RemoteToolBackendEnrollments),
		)
		for identity, enrollment := range s.RemoteToolBackendEnrollments {
			out.RemoteToolBackendEnrollments[identity] = enrollment
		}
	}
	for id, instance := range s.RuntimeInstances {
		out.RuntimeInstances[id] = cloneProjectedRuntimeInstance(instance)
	}
	cloneLocalProductSetupSnapshot(s, &out)
	if s.EvolutionAssets != nil {
		cloned := cloneEvolutionAssetSnapshot(*s.EvolutionAssets)
		out.EvolutionAssets = &cloned
	}
	return out
}

func cloneProjectedTeamInstance(input TeamInstance) TeamInstance {
	input.DormantSubAgents = cloneProjectedDormantSubAgents(input.DormantSubAgents)
	return input
}

func cloneProjectedDormantSubAgents(input []DormantSubAgent) []DormantSubAgent {
	if len(input) == 0 {
		return []DormantSubAgent{}
	}
	return append([]DormantSubAgent(nil), input...)
}

func equalProjectedTeamInstance(left, right TeamInstance) bool {
	if left.ID != right.ID ||
		left.WorkRequestID != right.WorkRequestID ||
		left.SourceKind != right.SourceKind ||
		left.TeamDefinitionID != right.TeamDefinitionID ||
		left.TeamDefinitionVersion != right.TeamDefinitionVersion ||
		left.TeamDefinitionScope != right.TeamDefinitionScope ||
		left.ScopeIdentity != right.ScopeIdentity ||
		left.TeamDefinitionDigest != right.TeamDefinitionDigest ||
		left.SourcePlanDigest != right.SourcePlanDigest ||
		left.SourceRecordSetDigest != right.SourceRecordSetDigest ||
		left.State != right.State ||
		left.CreatedAt != right.CreatedAt ||
		left.TeamInstanceCount != right.TeamInstanceCount ||
		left.AgentInstanceCount != right.AgentInstanceCount ||
		left.ActiveSubAgentCount != right.ActiveSubAgentCount ||
		left.WorkItemCount != right.WorkItemCount ||
		left.CreationEventID != right.CreationEventID ||
		len(left.DormantSubAgents) != len(right.DormantSubAgents) {
		return false
	}
	for index := range left.DormantSubAgents {
		if left.DormantSubAgents[index] != right.DormantSubAgents[index] {
			return false
		}
	}
	return true
}

type streamSeq struct {
	streamID string
	seq      int64
}

type eventScanner interface {
	Scan(dest ...any) error
}

func scanJournalEvent(scanner eventScanner) (journal.Event, error) {
	var event journal.Event
	var emittedAt int64
	var correlationID sql.NullString
	var causationID sql.NullString
	var payload string
	if err := scanner.Scan(
		&event.ID,
		&event.StreamID,
		&event.Seq,
		&event.IdempotencyKey,
		&event.Type,
		&event.SchemaVersion,
		&emittedAt,
		&correlationID,
		&causationID,
		&payload,
	); err != nil {
		return journal.Event{}, err
	}
	event.EmittedAt = time.Unix(0, emittedAt).UTC()
	if correlationID.Valid {
		event.CorrelationID = correlationID.String
	}
	if causationID.Valid {
		event.CausationID = causationID.String
	}
	event.PayloadJSON = []byte(payload)
	return event, nil
}

func sameImmutableEvent(left, right journal.Event) bool {
	return left.ID == right.ID &&
		left.StreamID == right.StreamID &&
		left.Seq == right.Seq &&
		left.IdempotencyKey == right.IdempotencyKey &&
		left.Type == right.Type &&
		left.SchemaVersion == right.SchemaVersion &&
		left.EmittedAt.Equal(right.EmittedAt) &&
		left.CorrelationID == right.CorrelationID &&
		left.CausationID == right.CausationID &&
		bytes.Equal(left.PayloadJSON, right.PayloadJSON)
}
