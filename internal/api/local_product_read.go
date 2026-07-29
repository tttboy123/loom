package api

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
)

const localProductSchemaVersion = 1

var (
	ErrInvalidLocalProductRequest   = errors.New("invalid local product request")
	ErrLocalProductStateUnavailable = errors.New(
		"local product state unavailable",
	)
)

type LocalProductReadConfig struct {
	Journal    *journal.Store
	Projection ViewSource
	Now        func() time.Time
}

type LocalProductReadService struct {
	journal    *journal.Store
	projection ViewSource
	now        func() time.Time

	mu       sync.Mutex
	lastView *projection.GlobalReadView
}

func NewLocalProductReadService(
	config LocalProductReadConfig,
) (*LocalProductReadService, error) {
	if config.Journal == nil || config.Projection == nil || config.Now == nil {
		return nil, ErrInvalidLocalProductRequest
	}
	return &LocalProductReadService{
		journal:    config.Journal,
		projection: config.Projection,
		now:        config.Now,
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

type LocalProductSnapshot struct {
	SchemaVersion int    `json:"schema_version"`
	ViewVersion   string `json:"view_version"`
	Partial       bool   `json:"partial"`
	Stale         bool   `json:"stale"`
	Reason        string `json:"reason"`

	Runtimes  []LocalProductRuntimeSummary  `json:"runtimes"`
	Teams     []LocalProductTeamSummary     `json:"teams"`
	Runs      []LocalProductRunSummary      `json:"runs"`
	Evidence  []LocalProductEvidenceSummary `json:"evidence"`
	Attention []AttentionItem               `json:"attention"`

	RuntimePage  LocalProductPageCursor `json:"runtime_page"`
	TeamPage     LocalProductPageCursor `json:"team_page"`
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
		stale.Stale = true
		stale.Reason = "projection_refresh_failed"
		return cloneLocalProductSnapshot(stale), nil
	}
	view := service.projection.GlobalReadView()
	snapshot := buildLocalProductSnapshot(view, request)
	service.lastView = &view
	return cloneLocalProductSnapshot(snapshot), nil
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
		Partial: runtimeMore || teamPage.HasMore || runMore ||
			evidenceMore || len(seenAttention) > len(attention),
		Runtimes:  runtimeSummaries,
		Teams:     teamSummaries,
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
		TeamPage: teamPage,
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
			displayName := team.TeamDefinitionID
			if displayName == "" {
				displayName = team.ID
			}
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
	if readErr != nil {
		return result, readErr
	}
	return result, nil
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
	snapshot.Runs = cloneLocalProductSlice(snapshot.Runs)
	snapshot.Evidence = cloneLocalProductSlice(snapshot.Evidence)
	snapshot.Attention = cloneLocalProductSlice(snapshot.Attention)
	return snapshot
}

func cloneLocalProductSlice[T any](records []T) []T {
	copied := make([]T, len(records))
	copy(copied, records)
	return copied
}
