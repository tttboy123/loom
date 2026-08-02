package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

const localProductSchemaVersion = 2

const (
	maxMissionExecutionObservers = 64
	maxMissionTentativeRecords   = 64
)

var (
	ErrInvalidLocalProductRequest   = errors.New("invalid local product request")
	ErrLocalProductStateUnavailable = errors.New(
		"local product state unavailable",
	)
)

type LocalProductReadConfig struct {
	Journal       *journal.Store
	Projection    ViewSource
	Now           func() time.Time
	Decisions     MissionDecisionCommandSource
	RuntimeHealth RuntimeObservationHealthSource
}

type LocalProductReadService struct {
	journal       *journal.Store
	projection    ViewSource
	now           func() time.Time
	decisions     MissionDecisionCommandSource
	runtimeHealth RuntimeObservationHealthSource

	mu                    sync.Mutex
	lastView              *projection.GlobalReadView
	lastPreparedDecisions []app.MissionDecisionCommand

	executionMu        sync.Mutex
	executionObservers map[string]*localProductMissionObserver
	tentativeRecords   map[string][]LocalProductTimelineRecord
	tentativeGaps      map[string]*LocalProductStreamGap
}

type localProductMissionObserver struct {
	service        *LocalProductReadService
	teamInstanceID string

	mu           sync.Mutex
	stream       *TeamExecutionStream
	subscription *Subscription
	closed       bool
}

type MissionDecisionCommandSource interface {
	ListMissionDecisionCommands(
		context.Context,
		app.MissionDecisionCommandQuery,
	) ([]app.MissionDecisionCommand, error)
}

type RuntimeObservationHealthSource interface {
	RuntimeObservationHealth() (reason string, partial bool)
}

func NewLocalProductReadService(
	config LocalProductReadConfig,
) (*LocalProductReadService, error) {
	if config.Journal == nil || config.Projection == nil || config.Now == nil {
		return nil, ErrInvalidLocalProductRequest
	}
	return &LocalProductReadService{
		journal:            config.Journal,
		projection:         config.Projection,
		now:                config.Now,
		decisions:          config.Decisions,
		runtimeHealth:      config.RuntimeHealth,
		executionObservers: make(map[string]*localProductMissionObserver),
		tentativeRecords:   make(map[string][]LocalProductTimelineRecord),
		tentativeGaps:      make(map[string]*LocalProductStreamGap),
	}, nil
}

type LocalProductSnapshotRequest struct {
	AfterTeamID     string `json:"after_team_id"`
	AfterRuntimeID  string `json:"after_runtime_id"`
	AfterRunID      string `json:"after_run_id"`
	AfterEvidenceID string `json:"after_evidence_id"`
	Limit           int    `json:"limit"`
}

type LocalProductRuntimeSummary struct {
	RuntimeInstanceID    string   `json:"runtime_instance_id"`
	DisplayName          string   `json:"display_name"`
	AdapterType          string   `json:"adapter_type"`
	ExecutableVersion    string   `json:"executable_version"`
	Status               string   `json:"status"`
	Capacity             int      `json:"capacity"`
	ModelIDs             []string `json:"model_ids"`
	ObservedCapabilities []string `json:"observed_capabilities"`
}

type LocalProductTeamSummary struct {
	TeamInstanceID string `json:"team_instance_id"`
	DisplayName    string `json:"display_name"`
	SourceKind     string `json:"source_kind"`
	State          string `json:"state"`
	Confirmed      bool   `json:"confirmed"`
	Executable     bool   `json:"executable"`
	ReadOnly       bool   `json:"read_only"`
}

type LocalProductRunSummary struct {
	RunID             string `json:"run_id"`
	WorkItemID        string `json:"work_item_id"`
	Phase             string `json:"phase"`
	TerminalStatus    string `json:"terminal_status"`
	TerminalReason    string `json:"terminal_reason"`
	RuntimeInstanceID string `json:"runtime_instance_id"`
	AgentInstanceID   string `json:"agent_instance_id"`
	ClaimGeneration   int64  `json:"claim_generation"`
}

type LocalProductEvidenceSummary struct {
	EvidenceID string `json:"evidence_id"`
	WorkItemID string `json:"work_item_id"`
	Digest     string `json:"digest"`
}

type LocalProductPageCursor struct {
	NextCursor string `json:"next_cursor"`
	HasMore    bool   `json:"has_more"`
}

type LocalProductHealth struct {
	Daemon     string `json:"daemon"`
	Journal    string `json:"journal"`
	Projection string `json:"projection"`
}

type LocalProductSnapshot struct {
	SchemaVersion int                `json:"schema_version"`
	ViewVersion   string             `json:"view_version"`
	Partial       bool               `json:"partial"`
	Stale         bool               `json:"stale"`
	Reason        string             `json:"reason"`
	Health        LocalProductHealth `json:"health"`

	Runtimes          []LocalProductRuntimeSummary  `json:"runtimes"`
	Teams             []LocalProductTeamSummary     `json:"teams"`
	Missions          []LocalProductMissionSummary  `json:"missions"`
	Runs              []LocalProductRunSummary      `json:"runs"`
	Evidence          []LocalProductEvidenceSummary `json:"evidence"`
	Attention         []AttentionItem               `json:"attention"`
	PreparedDecisions []app.MissionDecisionCommand  `json:"prepared_decisions"`

	RuntimePage  LocalProductPageCursor `json:"runtime_page"`
	TeamPage     LocalProductPageCursor `json:"team_page"`
	MissionPage  LocalProductPageCursor `json:"mission_page"`
	RunPage      LocalProductPageCursor `json:"run_page"`
	EvidencePage LocalProductPageCursor `json:"evidence_page"`
}

func (service *LocalProductReadService) ReadLocalProductSnapshot(
	ctx context.Context,
	request LocalProductSnapshotRequest,
) (LocalProductSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return LocalProductSnapshot{}, err
	}
	if !validLocalProductSnapshotRequest(request) {
		return LocalProductSnapshot{}, ErrInvalidLocalProductRequest
	}

	service.mu.Lock()
	defer service.mu.Unlock()

	if err := service.projection.Rebuild(ctx); err != nil {
		if service.lastView == nil {
			return LocalProductSnapshot{}, ErrLocalProductStateUnavailable
		}
		stale := buildLocalProductSnapshot(*service.lastView, request)
		if stale, err = service.applyRuntimeObservationHealth(stale); err != nil {
			return LocalProductSnapshot{}, err
		}
		prepared, err := service.preparedMissionDecisionCommands(
			ctx,
			app.MissionDecisionCommandQuery{
				ViewVersion: service.lastView.Version(),
				Mode:        app.MissionDecisionCommandPreserveStale,
			},
		)
		if err != nil || !equalMissionDecisionCommands(
			prepared,
			service.lastPreparedDecisions,
		) {
			return LocalProductSnapshot{}, ErrLocalProductStateUnavailable
		}
		stale.PreparedDecisions = cloneLocalProductSlice(
			service.lastPreparedDecisions,
		)
		stale.Stale = true
		stale.Reason = "projection_refresh_failed"
		stale.Health.Projection = "stale"
		return cloneLocalProductSnapshot(stale), nil
	}
	view := service.projection.GlobalReadView()
	snapshot := buildLocalProductSnapshot(view, request)
	prepared, err := service.preparedMissionDecisionCommands(
		ctx,
		app.MissionDecisionCommandQuery{
			ViewVersion: view.Version(),
			Mode:        app.MissionDecisionCommandRefreshCurrent,
		},
	)
	if err != nil {
		return LocalProductSnapshot{}, ErrLocalProductStateUnavailable
	}
	snapshot.PreparedDecisions = prepared
	snapshot, err = service.applyRuntimeObservationHealth(snapshot)
	if err != nil {
		return LocalProductSnapshot{}, err
	}
	service.lastView = &view
	service.lastPreparedDecisions = cloneLocalProductSlice(prepared)
	return cloneLocalProductSnapshot(snapshot), nil
}

func (service *LocalProductReadService) applyRuntimeObservationHealth(
	snapshot LocalProductSnapshot,
) (LocalProductSnapshot, error) {
	if service.runtimeHealth == nil {
		return snapshot, nil
	}
	reason, partial := service.runtimeHealth.RuntimeObservationHealth()
	if !partial {
		if reason != "" {
			return LocalProductSnapshot{}, ErrLocalProductStateUnavailable
		}
		return snapshot, nil
	}
	if reason != "observer_version_timeout" &&
		reason != "observer_models_timeout" {
		return LocalProductSnapshot{}, ErrLocalProductStateUnavailable
	}
	snapshot.Partial = true
	snapshot.Reason = reason
	return snapshot, nil
}

func (service *LocalProductReadService) preparedMissionDecisionCommands(
	ctx context.Context,
	query app.MissionDecisionCommandQuery,
) ([]app.MissionDecisionCommand, error) {
	if service.decisions == nil {
		return []app.MissionDecisionCommand{}, nil
	}
	commands, err := service.decisions.ListMissionDecisionCommands(ctx, query)
	if err != nil {
		return nil, err
	}
	if commands == nil {
		return []app.MissionDecisionCommand{}, nil
	}
	return cloneLocalProductSlice(commands), nil
}

func equalMissionDecisionCommands(
	left, right []app.MissionDecisionCommand,
) bool {
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

func buildLocalProductSnapshot(
	view projection.GlobalReadView,
	request LocalProductSnapshotRequest,
) LocalProductSnapshot {
	runtimes, runtimeMore := view.RuntimeInstances(
		request.AfterRuntimeID,
		request.Limit,
	)
	teams, savedTeamMore := view.Teams(request.AfterTeamID, request.Limit)
	runs, runMore := view.Runs(request.AfterRunID, request.Limit)
	evidence, evidenceMore := view.EvidenceRecords(
		request.AfterEvidenceID,
		request.Limit,
	)
	executions, executionTeamMore := view.TeamExecutions(
		request.AfterTeamID,
		request.Limit,
	)

	teamSummaries, teamPage := buildLocalProductTeamPage(
		view,
		teams,
		executions,
		request.Limit,
		savedTeamMore || executionTeamMore,
	)
	missionSummaries, missionPage := buildLocalProductMissionPage(
		view,
		executions,
		request.Limit,
		executionTeamMore,
	)

	runtimeSummaries := make([]LocalProductRuntimeSummary, len(runtimes))
	for index, runtime := range runtimes {
		runtimeSummaries[index] = LocalProductRuntimeSummary{
			RuntimeInstanceID: runtime.ID,
			DisplayName:       runtime.DisplayName,
			AdapterType:       runtime.AdapterType,
			ExecutableVersion: runtime.ExecutableVersion,
			Status:            runtime.Status,
			Capacity:          runtime.Capacity,
			ModelIDs: cloneLocalProductSlice(
				runtime.ModelIDs,
			),
			ObservedCapabilities: cloneLocalProductSlice(
				runtime.ObservedCapabilities,
			),
		}
	}
	runSummaries := make([]LocalProductRunSummary, len(runs))
	for index, run := range runs {
		runSummaries[index] = LocalProductRunSummary{
			RunID:             run.ID,
			WorkItemID:        run.WorkItemID,
			Phase:             run.Phase,
			TerminalStatus:    run.TerminalStatus,
			TerminalReason:    run.TerminalReason,
			RuntimeInstanceID: run.RuntimeInstanceID,
			AgentInstanceID:   run.AgentInstanceID,
			ClaimGeneration:   run.ClaimGeneration,
		}
	}
	evidenceSummaries := make([]LocalProductEvidenceSummary, len(evidence))
	for index, record := range evidence {
		evidenceSummaries[index] = LocalProductEvidenceSummary{
			EvidenceID: record.ID,
			WorkItemID: record.WorkItemID,
			Digest:     record.Digest,
		}
	}
	attention := make([]AttentionItem, 0)
	seenAttention := make(map[string]struct{})
	attentionTeamIDs := make(map[string]struct{}, len(teams)+len(executions))
	for _, team := range teams {
		attentionTeamIDs[team.ID] = struct{}{}
	}
	for _, execution := range executions {
		attentionTeamIDs[execution.TeamInstanceID] = struct{}{}
	}
	orderedAttentionTeamIDs := make([]string, 0, len(attentionTeamIDs))
	for teamID := range attentionTeamIDs {
		orderedAttentionTeamIDs = append(orderedAttentionTeamIDs, teamID)
	}
	sort.Strings(orderedAttentionTeamIDs)
	for _, teamID := range orderedAttentionTeamIDs {
		_, teamAttention := deriveBoardAndAttention(
			view,
			teamID,
		)
		for _, item := range teamAttention {
			if _, duplicate := seenAttention[item.AttentionID]; duplicate {
				continue
			}
			seenAttention[item.AttentionID] = struct{}{}
			attention = append(attention, item)
		}
	}
	sortAttention(attention)
	if len(attention) > request.Limit {
		attention = attention[:request.Limit]
	}

	return LocalProductSnapshot{
		SchemaVersion: localProductSchemaVersion,
		ViewVersion:   view.Version(),
		Health: LocalProductHealth{
			Daemon:     "serving_request",
			Journal:    "available",
			Projection: "current",
		},
		Partial: runtimeMore || teamPage.HasMore || missionPage.HasMore || runMore ||
			evidenceMore || len(seenAttention) > len(attention),
		Runtimes:  runtimeSummaries,
		Teams:     teamSummaries,
		Missions:  missionSummaries,
		Runs:      runSummaries,
		Evidence:  evidenceSummaries,
		Attention: attention,
		RuntimePage: localProductPageCursor(
			runtimeSummaries,
			runtimeMore,
			func(record LocalProductRuntimeSummary) string {
				return record.RuntimeInstanceID
			},
		),
		TeamPage:    teamPage,
		MissionPage: missionPage,
		RunPage: localProductPageCursor(
			runSummaries,
			runMore,
			func(record LocalProductRunSummary) string { return record.RunID },
		),
		EvidencePage: localProductPageCursor(
			evidenceSummaries,
			evidenceMore,
			func(record LocalProductEvidenceSummary) string {
				return record.EvidenceID
			},
		),
	}
}

func buildLocalProductTeamPage(
	view projection.GlobalReadView,
	teams []projection.TeamInstance,
	executions []projection.TeamExecution,
	limit int,
	sourceHasMore bool,
) ([]LocalProductTeamSummary, LocalProductPageCursor) {
	teamByID := make(map[string]projection.TeamInstance, len(teams))
	executionByID := make(
		map[string]projection.TeamExecution,
		len(executions),
	)
	candidateIDs := make(map[string]struct{}, len(teams)+len(executions))
	for _, team := range teams {
		teamByID[team.ID] = team
		candidateIDs[team.ID] = struct{}{}
	}
	for _, execution := range executions {
		executionByID[execution.TeamInstanceID] = execution
		candidateIDs[execution.TeamInstanceID] = struct{}{}
	}
	orderedIDs := make([]string, 0, len(candidateIDs))
	for id := range candidateIDs {
		orderedIDs = append(orderedIDs, id)
	}
	sort.Strings(orderedIDs)
	page := LocalProductPageCursor{
		HasMore: sourceHasMore || len(orderedIDs) > limit,
	}
	if len(orderedIDs) > limit {
		orderedIDs = orderedIDs[:limit]
	}
	if len(orderedIDs) > 0 {
		page.NextCursor = orderedIDs[len(orderedIDs)-1]
	}
	summaries := make(
		[]LocalProductTeamSummary,
		0,
		len(orderedIDs),
	)
	for _, id := range orderedIDs {
		if team, ok := teamByID[id]; ok {
			anchor, _ := view.TeamTimelineAnchor(team.ID)
			displayName := localProductSavedTeamDisplayName(view, team)
			summaries = append(summaries, LocalProductTeamSummary{
				TeamInstanceID: team.ID,
				DisplayName:    displayName,
				SourceKind:     team.SourceKind,
				State:          team.State,
				Confirmed:      anchor.Confirmed,
				Executable:     anchor.Executable,
				ReadOnly:       anchor.ReadOnly,
			})
			continue
		}
		execution := executionByID[id]
		anchor, ok := view.TeamTimelineAnchor(execution.TeamInstanceID)
		if !ok || anchor.Kind != "historical_execution_only" {
			continue
		}
		summaries = append(summaries, LocalProductTeamSummary{
			TeamInstanceID: execution.TeamInstanceID,
			DisplayName:    execution.TeamInstanceID,
			SourceKind:     anchor.Kind,
			State:          execution.Status,
			ReadOnly:       true,
		})
	}
	return summaries, page
}

func localProductSavedTeamDisplayName(
	view projection.GlobalReadView,
	team projection.TeamInstance,
) string {
	const fallback = "Saved team"
	definition, ok := view.TeamDefinition(team.TeamDefinitionID)
	if !ok || definition.ID != team.TeamDefinitionID ||
		definition.Version != team.TeamDefinitionVersion ||
		definition.Scope != team.TeamDefinitionScope ||
		definition.DefinitionDigest != team.TeamDefinitionDigest ||
		definition.Name == "" {
		return fallback
	}
	switch definition.Scope {
	case "project":
		if definition.ScopeIdentity.ProjectID == "" ||
			definition.ScopeIdentity.ProjectID != team.ScopeIdentity.ProjectID ||
			definition.ScopeIdentity.GenerationID == "" ||
			team.ScopeIdentity.GenerationID != "" {
			return fallback
		}
	case "reusable":
		if definition.ScopeIdentity != (projection.ScopeIdentity{}) ||
			team.ScopeIdentity != (projection.ScopeIdentity{}) {
			return fallback
		}
	default:
		return fallback
	}
	return definition.Name
}

func localProductPageCursor[T any](
	records []T,
	hasMore bool,
	id func(T) string,
) LocalProductPageCursor {
	cursor := LocalProductPageCursor{HasMore: hasMore}
	if len(records) > 0 {
		cursor.NextCursor = id(records[len(records)-1])
	}
	return cursor
}

type LocalProductTimelineRequest struct {
	TeamInstanceID string `json:"team_instance_id"`
	Cursor         string `json:"cursor"`
	Limit          int    `json:"limit"`
}

type LocalProductCostObservation struct {
	Observed         bool   `json:"observed"`
	AmountMicrounits *int64 `json:"amount_microunits"`
	Currency         string `json:"currency"`
}

type LocalProductTimelinePayload struct {
	Status         string                      `json:"status"`
	ReasonCode     string                      `json:"reason_code"`
	Action         string                      `json:"action"`
	WarningCode    string                      `json:"warning_code"`
	RetryAt        string                      `json:"retry_at"`
	TextDelta      string                      `json:"text_delta"`
	EvidenceDigest string                      `json:"evidence_digest"`
	Cost           LocalProductCostObservation `json:"cost"`
}

type LocalProductTimelineRecord struct {
	SchemaVersion  int                         `json:"schema_version"`
	DeliveryID     string                      `json:"delivery_id"`
	Kind           string                      `json:"kind"`
	Authority      string                      `json:"authority"`
	TeamInstanceID string                      `json:"team_instance_id"`
	LogicalNodeID  string                      `json:"logical_node_id"`
	AttemptNumber  int                         `json:"attempt_number"`
	SourceStreamID string                      `json:"source_stream_id"`
	SourceSequence int64                       `json:"source_sequence"`
	SourceEventID  string                      `json:"source_event_id"`
	OccurredAt     string                      `json:"occurred_at"`
	Cursor         string                      `json:"cursor"`
	Payload        LocalProductTimelinePayload `json:"payload"`
}

type LocalProductStreamGap struct {
	SchemaVersion        int    `json:"schema_version"`
	DeliveryID           string `json:"delivery_id"`
	Kind                 string `json:"kind"`
	TeamInstanceID       string `json:"team_instance_id"`
	Reason               string `json:"reason"`
	PreviousCursorDigest string `json:"previous_cursor_digest"`
	CurrentViewVersion   string `json:"current_view_version"`
	ArtifactAvailable    bool   `json:"artifact_available"`
	ArtifactDigest       string `json:"artifact_digest"`
	Recoverable          bool   `json:"recoverable"`
	OccurredAt           string `json:"occurred_at"`
}

type LocalProductTeamBoard struct {
	SchemaVersion  int                         `json:"schema_version"`
	TeamInstanceID string                      `json:"team_instance_id"`
	PlanDigest     string                      `json:"plan_digest"`
	Status         string                      `json:"status"`
	ViewVersion    string                      `json:"view_version"`
	Nodes          []NodeBoardRow              `json:"nodes"`
	Cost           LocalProductCostObservation `json:"cost"`
}

type LocalProductTimelinePage struct {
	SchemaVersion  int                          `json:"schema_version"`
	TeamInstanceID string                       `json:"team_instance_id"`
	ViewVersion    string                       `json:"view_version"`
	NextCursor     string                       `json:"next_cursor"`
	HasMore        bool                         `json:"has_more"`
	Gap            *LocalProductStreamGap       `json:"gap"`
	Records        []LocalProductTimelineRecord `json:"records"`
	Board          LocalProductTeamBoard        `json:"board"`
	Attention      []AttentionItem              `json:"attention"`
}

func (service *LocalProductReadService) MissionExecutionObserver(
	ctx context.Context,
	teamInstanceID string,
) (app.NodeOutputObserver, error) {
	if service == nil || ctx == nil || !validTimelineID(teamInstanceID) {
		return nil, ErrInvalidLocalProductRequest
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	service.executionMu.Lock()
	defer service.executionMu.Unlock()
	if existing := service.executionObservers[teamInstanceID]; existing != nil {
		return existing, nil
	}
	if len(service.executionObservers) >= maxMissionExecutionObservers {
		return nil, ErrTooManySubscribers
	}
	observer := &localProductMissionObserver{
		service: service, teamInstanceID: teamInstanceID,
	}
	service.executionObservers[teamInstanceID] = observer
	return observer, nil
}

func (observer *localProductMissionObserver) ObserveNodeOutput(
	ctx context.Context,
	output app.NodeOutput,
) error {
	if observer == nil || observer.service == nil || ctx == nil {
		return ErrInvalidNodeOutput
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	observer.mu.Lock()
	defer observer.mu.Unlock()
	if observer.closed {
		return context.Canceled
	}
	frame := output.AuthorizedFrame().Frame()
	if frame.Type() != bridgev1.MessageEvent {
		if observer.stream == nil {
			return nil
		}
		return observer.stream.ObserveNodeOutput(ctx, output)
	}
	if observer.stream == nil {
		stream, err := NewTeamExecutionStream(TeamExecutionStreamConfig{
			TeamInstanceID: observer.teamInstanceID,
			Journal:        observer.service.journal,
			Projection:     observer.service.projection,
			Now:            observer.service.now,
		})
		if err != nil {
			return err
		}
		subscription, err := stream.Subscribe(ctx, "")
		if err != nil {
			return err
		}
		observer.stream = stream
		observer.subscription = subscription
	}
	if err := observer.stream.ObserveNodeOutput(ctx, output); err != nil {
		return err
	}
	item, err := observer.subscription.Next(ctx)
	if err != nil {
		return err
	}
	if delivery, ok := item.Delivery(); ok {
		encoded, err := json.Marshal(delivery)
		if err != nil {
			return ErrInvalidDeliveryRecord
		}
		var record LocalProductTimelineRecord
		if err := json.Unmarshal(encoded, &record); err != nil {
			return ErrInvalidDeliveryRecord
		}
		return observer.service.cacheMissionExecutionDelivery(record)
	}
	if gap, ok := item.Gap(); ok {
		encoded, err := json.Marshal(gap)
		if err != nil {
			return ErrInvalidDeliveryRecord
		}
		var record LocalProductStreamGap
		if err := json.Unmarshal(encoded, &record); err != nil {
			return ErrInvalidDeliveryRecord
		}
		observer.service.cacheMissionExecutionGap(record)
		return nil
	}
	return ErrInvalidDeliveryRecord
}

func (observer *localProductMissionObserver) Close() error {
	if observer == nil {
		return nil
	}
	observer.mu.Lock()
	if observer.closed {
		observer.mu.Unlock()
		return nil
	}
	observer.closed = true
	subscription := observer.subscription
	observer.subscription = nil
	observer.stream = nil
	observer.mu.Unlock()
	if subscription != nil {
		return subscription.Close()
	}
	return nil
}

func (service *LocalProductReadService) CloseMissionExecutionObservers() error {
	if service == nil {
		return nil
	}
	service.executionMu.Lock()
	observers := make([]*localProductMissionObserver, 0, len(service.executionObservers))
	for _, observer := range service.executionObservers {
		observers = append(observers, observer)
	}
	service.executionObservers = make(map[string]*localProductMissionObserver)
	service.executionMu.Unlock()
	var result error
	for _, observer := range observers {
		result = errors.Join(result, observer.Close())
	}
	return result
}

func (service *LocalProductReadService) cacheMissionExecutionDelivery(
	record LocalProductTimelineRecord,
) error {
	if service == nil || record.SchemaVersion != timelineSchemaVersion ||
		!validDigest(record.DeliveryID) ||
		record.Kind != "node_output_delta" ||
		record.Authority != "tentative" ||
		!validTimelineID(record.TeamInstanceID) ||
		!validTimelineID(record.LogicalNodeID) ||
		record.AttemptNumber <= 0 || record.SourceSequence <= 0 ||
		!validTimelineID(record.SourceEventID) ||
		!validTentativeDelta(record.Payload.TextDelta) {
		return ErrInvalidDeliveryRecord
	}
	occurredAt, err := time.Parse(time.RFC3339Nano, record.OccurredAt)
	if err != nil || occurredAt.Location() != time.UTC {
		return ErrInvalidDeliveryRecord
	}
	record.Cursor = ""
	service.executionMu.Lock()
	defer service.executionMu.Unlock()
	records := service.tentativeRecords[record.TeamInstanceID]
	for _, existing := range records {
		if existing.DeliveryID == record.DeliveryID {
			return nil
		}
	}
	if len(records) >= maxMissionTentativeRecords {
		service.tentativeGaps[record.TeamInstanceID] =
			service.newMissionExecutionGap(record.TeamInstanceID, occurredAt)
		return nil
	}
	service.tentativeRecords[record.TeamInstanceID] = append(
		append([]LocalProductTimelineRecord(nil), records...),
		record,
	)
	return nil
}

func (service *LocalProductReadService) newMissionExecutionGap(
	teamInstanceID string,
	occurredAt time.Time,
) *LocalProductStreamGap {
	viewVersion := service.projection.GlobalReadView().Version()
	digest := sha256.Sum256([]byte(
		"loom.local-product.tentative-gap.v1\x00" +
			teamInstanceID + "\x00" + viewVersion,
	))
	return &LocalProductStreamGap{
		SchemaVersion:      timelineSchemaVersion,
		DeliveryID:         hex.EncodeToString(digest[:]),
		Kind:               "stream_gap",
		TeamInstanceID:     teamInstanceID,
		Reason:             "tentative_overflow",
		CurrentViewVersion: viewVersion,
		Recoverable:        true,
		OccurredAt:         occurredAt.UTC().Format(time.RFC3339Nano),
	}
}

func (service *LocalProductReadService) cacheMissionExecutionGap(
	gap LocalProductStreamGap,
) {
	if service == nil || !validTimelineID(gap.TeamInstanceID) {
		return
	}
	service.executionMu.Lock()
	copy := gap
	service.tentativeGaps[gap.TeamInstanceID] = &copy
	service.executionMu.Unlock()
}

func (service *LocalProductReadService) mergeMissionExecutionTimeline(
	result *LocalProductTimelinePage,
	limit int,
) {
	if service == nil || result == nil || limit < 1 {
		return
	}
	if terminalLocalProductTeamStatus(result.Board.Status) {
		service.clearMissionExecutionState(result.TeamInstanceID)
		return
	}
	service.executionMu.Lock()
	records := append(
		[]LocalProductTimelineRecord(nil),
		service.tentativeRecords[result.TeamInstanceID]...,
	)
	var gap *LocalProductStreamGap
	if stored := service.tentativeGaps[result.TeamInstanceID]; stored != nil {
		copy := *stored
		gap = &copy
	}
	service.executionMu.Unlock()
	seen := make(map[string]struct{}, len(result.Records))
	for _, record := range result.Records {
		seen[record.DeliveryID] = struct{}{}
	}
	for _, record := range records {
		if len(result.Records) >= limit {
			result.HasMore = true
			break
		}
		if _, exists := seen[record.DeliveryID]; exists {
			continue
		}
		record.Cursor = result.NextCursor
		result.Records = append(result.Records, record)
		seen[record.DeliveryID] = struct{}{}
	}
	if result.Gap == nil && gap != nil {
		result.Gap = gap
	}
}

func terminalLocalProductTeamStatus(status string) bool {
	switch status {
	case "succeeded", "failed", "cancelled", "blocked", "human_required":
		return true
	default:
		return false
	}
}

func (service *LocalProductReadService) clearMissionExecutionState(
	teamInstanceID string,
) {
	service.executionMu.Lock()
	observer := service.executionObservers[teamInstanceID]
	delete(service.executionObservers, teamInstanceID)
	delete(service.tentativeRecords, teamInstanceID)
	delete(service.tentativeGaps, teamInstanceID)
	service.executionMu.Unlock()
	if observer != nil {
		_ = observer.Close()
	}
}

func (service *LocalProductReadService) ReadLocalProductTimeline(
	ctx context.Context,
	request LocalProductTimelineRequest,
) (LocalProductTimelinePage, error) {
	if !validTimelineID(request.TeamInstanceID) ||
		request.Limit < 1 ||
		request.Limit > journal.MaxReadPageEvents {
		return LocalProductTimelinePage{}, ErrInvalidLocalProductRequest
	}
	stream, err := NewTeamExecutionStream(TeamExecutionStreamConfig{
		TeamInstanceID: request.TeamInstanceID,
		Journal:        service.journal,
		Projection:     service.projection,
		Now:            service.now,
	})
	if err != nil {
		return LocalProductTimelinePage{}, ErrInvalidLocalProductRequest
	}
	page, readErr := stream.ReadPage(ctx, request.Cursor, request.Limit)
	if readErr != nil {
		var gapErr *TimelineGapError
		if !errors.As(readErr, &gapErr) {
			return LocalProductTimelinePage{}, readErr
		}
		page = gapErr.Page()
	}
	encoded, err := json.Marshal(page)
	if err != nil {
		return LocalProductTimelinePage{}, ErrLocalProductStateUnavailable
	}
	var result LocalProductTimelinePage
	if err := json.Unmarshal(encoded, &result); err != nil {
		return LocalProductTimelinePage{}, ErrLocalProductStateUnavailable
	}
	if result.Records == nil {
		result.Records = []LocalProductTimelineRecord{}
	}
	if result.Attention == nil {
		result.Attention = []AttentionItem{}
	}
	if result.Board.Nodes == nil {
		result.Board.Nodes = []NodeBoardRow{}
	}
	enrichLocalProductEvidenceDigests(
		&result,
		service.projection.GlobalReadView(),
	)
	service.mergeMissionExecutionTimeline(&result, request.Limit)
	if readErr != nil {
		return result, readErr
	}
	return result, nil
}

func enrichLocalProductEvidenceDigests(
	page *LocalProductTimelinePage,
	view projection.GlobalReadView,
) {
	if page == nil {
		return
	}
	for index := range page.Records {
		record := &page.Records[index]
		if record.Kind != "evidence_available" ||
			record.Authority != "journal" ||
			record.Payload.EvidenceDigest != "" ||
			!strings.HasPrefix(record.SourceStreamID, "evidence/") {
			continue
		}
		evidenceID := strings.TrimPrefix(record.SourceStreamID, "evidence/")
		evidence, ok := view.Evidence(evidenceID)
		if !ok || evidence.ID != evidenceID || !validDigest(evidence.Digest) {
			continue
		}
		record.Payload.EvidenceDigest = evidence.Digest
	}
}

func validLocalProductSnapshotRequest(
	request LocalProductSnapshotRequest,
) bool {
	if request.Limit < 1 || request.Limit > 64 {
		return false
	}
	for _, id := range []string{
		request.AfterTeamID,
		request.AfterRuntimeID,
		request.AfterRunID,
		request.AfterEvidenceID,
	} {
		if id != "" && !validTimelineID(id) {
			return false
		}
	}
	return true
}

func cloneLocalProductSnapshot(
	snapshot LocalProductSnapshot,
) LocalProductSnapshot {
	snapshot.Runtimes = cloneLocalProductSlice(snapshot.Runtimes)
	for index := range snapshot.Runtimes {
		snapshot.Runtimes[index].ModelIDs =
			cloneLocalProductSlice(
				snapshot.Runtimes[index].ModelIDs,
			)
		snapshot.Runtimes[index].ObservedCapabilities =
			cloneLocalProductSlice(
				snapshot.Runtimes[index].ObservedCapabilities,
			)
	}
	snapshot.Teams = cloneLocalProductSlice(snapshot.Teams)
	snapshot.Missions = cloneLocalProductSlice(snapshot.Missions)
	for index := range snapshot.Missions {
		snapshot.Missions[index].TeamPulse = cloneLocalProductSlice(
			snapshot.Missions[index].TeamPulse,
		)
		snapshot.Missions[index].Topology = cloneLocalProductSlice(
			snapshot.Missions[index].Topology,
		)
		for nodeIndex := range snapshot.Missions[index].Topology {
			snapshot.Missions[index].Topology[nodeIndex].DependsOn =
				cloneLocalProductSlice(
					snapshot.Missions[index].Topology[nodeIndex].DependsOn,
				)
		}
	}
	snapshot.Runs = cloneLocalProductSlice(snapshot.Runs)
	snapshot.Evidence = cloneLocalProductSlice(snapshot.Evidence)
	snapshot.Attention = cloneLocalProductSlice(snapshot.Attention)
	snapshot.PreparedDecisions = cloneLocalProductSlice(
		snapshot.PreparedDecisions,
	)
	return snapshot
}

func cloneLocalProductSlice[T any](records []T) []T {
	copied := make([]T, len(records))
	copy(copied, records)
	return copied
}
