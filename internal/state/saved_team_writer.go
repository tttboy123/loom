package state

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"loom-pi-rebuild/internal/agents"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/mode"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/teams"
)

var (
	ErrInvalidSavedTeamCommitInput   = errors.New("invalid saved team commit input")
	ErrInvalidSavedTeamCommitSource  = errors.New("invalid saved team commit source")
	ErrInvalidSavedTeamCommitResult  = errors.New("invalid saved team commit result")
	ErrSavedTeamCommitResultMismatch = errors.New("saved team commit result mismatch")
	ErrSavedTeamCommitDigestMismatch = errors.New("saved team commit digest mismatch")
)

const (
	teamInstanceCreatedEventType  = "TeamInstanceCreated"
	agentInstanceCreatedEventType = "AgentInstanceCreated"
	savedTeamEventSchemaVersion   = 1
	savedTeamCommitEventCount     = 2
)

type EventBatchAppender interface {
	AppendBatch(context.Context, []journal.Event) ([]journal.Event, error)
}

type SavedTeamCommitInput struct {
	TeamEventID             string
	TeamIdempotencyKey      string
	MainAgentEventID        string
	MainAgentIdempotencyKey string
	EmittedAt               time.Time
}

type SavedTeamCommitCandidate struct {
	committed             bool
	sourceRecordSetDigest string
	teamEvent             journal.Event
	mainAgentEvent        journal.Event
	eventCount            int
	commitDigest          string
}

func CommitSavedTeamInstanceRecordSet(
	ctx context.Context,
	appender EventBatchAppender,
	records teams.SavedTeamInstanceRecordSetCandidate,
	plan teams.SavedTeamInstantiationPlanCandidate,
	intent mode.Intent,
	scope agents.ScopeIdentity,
	catalog teams.TeamResolutionCatalogInput,
	binding teams.SavedTeamRuntimeBindingCandidate,
	discovery loomruntime.RuntimeDiscoverySnapshot,
	selections []teams.SavedTeamRuntimeSelection,
	identity teams.SavedTeamInstanceIdentityInput,
	commit SavedTeamCommitInput,
) (SavedTeamCommitCandidate, error) {
	if ctx == nil || isNilEventBatchAppender(appender) {
		return SavedTeamCommitCandidate{}, ErrInvalidSavedTeamCommitInput
	}
	if err := ctx.Err(); err != nil {
		return SavedTeamCommitCandidate{}, err
	}
	normalizedCommit, err := validateSavedTeamCommitInput(commit)
	if err != nil {
		return SavedTeamCommitCandidate{}, err
	}

	validated, err := teams.ValidateSavedTeamInstanceRecordSet(
		records,
		plan,
		intent,
		scope,
		catalog,
		binding,
		discovery,
		selections,
		identity,
	)
	if err != nil {
		return SavedTeamCommitCandidate{}, err
	}
	if !validated.Valid ||
		validated.RecordSetDigest == "" ||
		validated.RecordSetDigest != records.RecordSetDigest() ||
		validated.SourcePlanDigest != records.SourcePlanDigest() ||
		validated.TeamInstanceCount != 1 ||
		validated.AgentInstanceCount != 1 ||
		validated.ActiveSubAgentCount != 0 ||
		validated.WorkItemCount != 0 {
		return SavedTeamCommitCandidate{}, ErrInvalidSavedTeamCommitSource
	}

	requested, err := buildSavedTeamCommitEvents(records, binding, normalizedCommit)
	if err != nil {
		return SavedTeamCommitCandidate{}, err
	}
	if err := ctx.Err(); err != nil {
		return SavedTeamCommitCandidate{}, err
	}

	committed, err := appender.AppendBatch(ctx, cloneSavedTeamCommitEvents(requested))
	if err != nil {
		return SavedTeamCommitCandidate{}, err
	}
	if len(committed) != savedTeamCommitEventCount {
		return SavedTeamCommitCandidate{}, ErrSavedTeamCommitResultMismatch
	}
	for index := range requested {
		if !sameSavedTeamCommitEvent(requested[index], committed[index]) {
			return SavedTeamCommitCandidate{}, ErrSavedTeamCommitResultMismatch
		}
	}

	candidate := SavedTeamCommitCandidate{
		committed:             true,
		sourceRecordSetDigest: records.RecordSetDigest(),
		teamEvent:             cloneSavedTeamCommitEvent(committed[0]),
		mainAgentEvent:        cloneSavedTeamCommitEvent(committed[1]),
		eventCount:            savedTeamCommitEventCount,
	}
	candidate.commitDigest, err = digestSavedTeamCommitCandidate(candidate)
	if err != nil {
		return SavedTeamCommitCandidate{}, err
	}
	if err := validateSavedTeamCommitCandidate(candidate); err != nil {
		return SavedTeamCommitCandidate{}, err
	}
	return candidate, nil
}

func validateSavedTeamCommitInput(input SavedTeamCommitInput) (SavedTeamCommitInput, error) {
	if input.TeamEventID == "" ||
		input.TeamIdempotencyKey == "" ||
		input.MainAgentEventID == "" ||
		input.MainAgentIdempotencyKey == "" ||
		input.TeamEventID == input.MainAgentEventID ||
		input.TeamIdempotencyKey == input.MainAgentIdempotencyKey ||
		input.EmittedAt.IsZero() {
		return SavedTeamCommitInput{}, ErrInvalidSavedTeamCommitInput
	}
	input.EmittedAt = input.EmittedAt.UTC()
	return input, nil
}

func buildSavedTeamCommitEvents(
	records teams.SavedTeamInstanceRecordSetCandidate,
	binding teams.SavedTeamRuntimeBindingCandidate,
	commit SavedTeamCommitInput,
) ([]journal.Event, error) {
	team := records.Team()
	main := records.MainAgent()
	dormant := records.DormantSubAgents()

	teamPayload, err := json.Marshal(savedTeamCreatedPayload{
		Team:                  canonicalSavedTeamInstanceRecord(team),
		DormantSubAgents:      canonicalSavedTeamDormantRecords(dormant),
		SourcePlanDigest:      records.SourcePlanDigest(),
		SourceRecordSetDigest: records.RecordSetDigest(),
		TeamInstanceCount:     records.TeamInstanceCount(),
		AgentInstanceCount:    records.AgentInstanceCount(),
		ActiveSubAgentCount:   records.ActiveSubAgentCount(),
		WorkItemCount:         records.WorkItemCount(),
	})
	if err != nil {
		return nil, fmt.Errorf("%w: Team payload: %w", ErrInvalidSavedTeamCommitSource, err)
	}
	mainPayload, err := json.Marshal(savedTeamMainAgentCreatedPayload{
		MainAgent:              canonicalSavedTeamMainAgentRecord(main),
		RuntimeBinding:         canonicalSavedTeamRuntimeBinding(main.Binding),
		SourcePlanDigest:       records.SourcePlanDigest(),
		SourceRecordSetDigest:  records.RecordSetDigest(),
		TeamCreatedAt:          team.CreatedAt,
		BindingDigest:          binding.BindingDigest(),
		RuntimeDiscoveryDigest: binding.MainRuntimeDiscoveryDigest(),
	})
	if err != nil {
		return nil, fmt.Errorf("%w: Main Agent payload: %w", ErrInvalidSavedTeamCommitSource, err)
	}

	return []journal.Event{
		{
			ID:             commit.TeamEventID,
			StreamID:       "team_instance:" + team.ID,
			Seq:            1,
			IdempotencyKey: commit.TeamIdempotencyKey,
			Type:           teamInstanceCreatedEventType,
			SchemaVersion:  savedTeamEventSchemaVersion,
			EmittedAt:      commit.EmittedAt,
			CorrelationID:  team.WorkRequestID,
			CausationID:    "",
			PayloadJSON:    append([]byte(nil), teamPayload...),
		},
		{
			ID:             commit.MainAgentEventID,
			StreamID:       "agent_instance:" + main.ID,
			Seq:            1,
			IdempotencyKey: commit.MainAgentIdempotencyKey,
			Type:           agentInstanceCreatedEventType,
			SchemaVersion:  savedTeamEventSchemaVersion,
			EmittedAt:      commit.EmittedAt,
			CorrelationID:  team.WorkRequestID,
			CausationID:    commit.TeamEventID,
			PayloadJSON:    append([]byte(nil), mainPayload...),
		},
	}, nil
}

type savedTeamCreatedPayload struct {
	Team                  savedTeamInstanceRecordPayload    `json:"team"`
	DormantSubAgents      []savedTeamDormantSubAgentPayload `json:"dormant_sub_agents"`
	SourcePlanDigest      string                            `json:"source_plan_digest"`
	SourceRecordSetDigest string                            `json:"source_record_set_digest"`
	TeamInstanceCount     int                               `json:"team_instance_count"`
	AgentInstanceCount    int                               `json:"agent_instance_count"`
	ActiveSubAgentCount   int                               `json:"active_sub_agent_count"`
	WorkItemCount         int                               `json:"work_item_count"`
}

type savedTeamInstanceRecordPayload struct {
	ID                    string                            `json:"id"`
	WorkRequestID         string                            `json:"work_request_id"`
	SourceKind            teams.SavedTeamInstanceSourceKind `json:"source_kind"`
	TeamDefinitionID      string                            `json:"team_definition_id"`
	TeamDefinitionVersion int                               `json:"team_definition_version"`
	TeamDefinitionScope   teams.TeamDefinitionScope         `json:"team_definition_scope"`
	ScopeIdentity         savedTeamScopeIdentityPayload     `json:"scope_identity"`
	TeamDefinitionDigest  string                            `json:"team_definition_digest"`
	SourcePlanDigest      string                            `json:"source_plan_digest"`
	State                 teams.SavedTeamInstanceState      `json:"state"`
	CreatedAt             int64                             `json:"created_at"`
}

type savedTeamDormantSubAgentPayload struct {
	Dormant           bool   `json:"dormant"`
	AgentDefinitionID string `json:"agent_definition_id"`
	RuntimeProfileID  string `json:"runtime_profile_id"`
	RuntimeInstanceID string `json:"runtime_instance_id"`
}

type savedTeamMainAgentCreatedPayload struct {
	MainAgent              savedTeamMainAgentRecordPayload `json:"main_agent"`
	RuntimeBinding         savedTeamRuntimeBindingPayload  `json:"runtime_binding"`
	SourcePlanDigest       string                          `json:"source_plan_digest"`
	SourceRecordSetDigest  string                          `json:"source_record_set_digest"`
	TeamCreatedAt          int64                           `json:"team_created_at"`
	BindingDigest          string                          `json:"binding_digest"`
	RuntimeDiscoveryDigest string                          `json:"runtime_discovery_digest"`
}

type savedTeamMainAgentRecordPayload struct {
	ID                     string                            `json:"id"`
	TeamInstanceID         string                            `json:"team_instance_id"`
	AgentDefinitionID      string                            `json:"agent_definition_id"`
	AgentDefinitionVersion int                               `json:"agent_definition_version"`
	AgentDefinitionScope   agents.Scope                      `json:"agent_definition_scope"`
	ScopeIdentity          savedTeamScopeIdentityPayload     `json:"scope_identity"`
	RuntimeProfileID       string                            `json:"runtime_profile_id"`
	RuntimeInstanceID      string                            `json:"runtime_instance_id"`
	IsMain                 bool                              `json:"is_main"`
	State                  teams.SavedTeamAgentInstanceState `json:"state"`
}

type savedTeamRuntimeBindingPayload struct {
	Accepted   bool   `json:"accepted"`
	ProfileID  string `json:"profile_id"`
	InstanceID string `json:"instance_id"`
}

type savedTeamScopeIdentityPayload struct {
	ProjectID    string `json:"project_id"`
	GenerationID string `json:"generation_id"`
}

func canonicalSavedTeamInstanceRecord(
	input teams.SavedTeamInstanceRecord,
) savedTeamInstanceRecordPayload {
	return savedTeamInstanceRecordPayload{
		ID:                    input.ID,
		WorkRequestID:         input.WorkRequestID,
		SourceKind:            input.SourceKind,
		TeamDefinitionID:      input.TeamDefinitionID,
		TeamDefinitionVersion: input.TeamDefinitionVersion,
		TeamDefinitionScope:   input.TeamDefinitionScope,
		ScopeIdentity:         canonicalSavedTeamScopeIdentity(input.ScopeIdentity),
		TeamDefinitionDigest:  input.TeamDefinitionDigest,
		SourcePlanDigest:      input.SourcePlanDigest,
		State:                 input.State,
		CreatedAt:             input.CreatedAt,
	}
}

func canonicalSavedTeamDormantRecords(
	input []teams.SavedTeamDormantSubAgentRecord,
) []savedTeamDormantSubAgentPayload {
	result := make([]savedTeamDormantSubAgentPayload, len(input))
	for index, current := range input {
		result[index] = savedTeamDormantSubAgentPayload{
			Dormant:           current.Dormant,
			AgentDefinitionID: current.AgentDefinitionID,
			RuntimeProfileID:  current.RuntimeProfileID,
			RuntimeInstanceID: current.RuntimeInstanceID,
		}
	}
	return result
}

func canonicalSavedTeamMainAgentRecord(
	input teams.SavedTeamMainAgentInstanceRecord,
) savedTeamMainAgentRecordPayload {
	return savedTeamMainAgentRecordPayload{
		ID:                     input.ID,
		TeamInstanceID:         input.TeamInstanceID,
		AgentDefinitionID:      input.AgentDefinitionID,
		AgentDefinitionVersion: input.AgentDefinitionVersion,
		AgentDefinitionScope:   input.AgentDefinitionScope,
		ScopeIdentity:          canonicalSavedTeamScopeIdentity(input.ScopeIdentity),
		RuntimeProfileID:       input.RuntimeProfileID,
		RuntimeInstanceID:      input.RuntimeInstanceID,
		IsMain:                 input.IsMain,
		State:                  input.State,
	}
}

func canonicalSavedTeamRuntimeBinding(
	input loomruntime.BindingCandidate,
) savedTeamRuntimeBindingPayload {
	return savedTeamRuntimeBindingPayload{
		Accepted:   input.Accepted,
		ProfileID:  input.ProfileID,
		InstanceID: input.InstanceID,
	}
}

func canonicalSavedTeamScopeIdentity(
	input agents.ScopeIdentity,
) savedTeamScopeIdentityPayload {
	return savedTeamScopeIdentityPayload{
		ProjectID:    input.ProjectID,
		GenerationID: input.GenerationID,
	}
}

func validateSavedTeamCommitCandidate(input SavedTeamCommitCandidate) error {
	if !input.committed ||
		input.sourceRecordSetDigest == "" ||
		input.eventCount != savedTeamCommitEventCount ||
		input.commitDigest == "" ||
		input.teamEvent.Type != teamInstanceCreatedEventType ||
		input.mainAgentEvent.Type != agentInstanceCreatedEventType {
		return ErrInvalidSavedTeamCommitResult
	}
	digest, err := digestSavedTeamCommitCandidate(input)
	if err != nil {
		return err
	}
	if digest != input.commitDigest {
		return ErrSavedTeamCommitDigestMismatch
	}
	return nil
}

func digestSavedTeamCommitCandidate(input SavedTeamCommitCandidate) (string, error) {
	encoded, err := json.Marshal(savedTeamCommitDigestPayload{
		Committed:             input.committed,
		SourceRecordSetDigest: input.sourceRecordSetDigest,
		TeamEvent:             canonicalSavedTeamCommitDigestEvent(input.teamEvent),
		MainAgentEvent:        canonicalSavedTeamCommitDigestEvent(input.mainAgentEvent),
		EventCount:            input.eventCount,
	})
	if err != nil {
		return "", fmt.Errorf("%w: digest encoding: %w", ErrInvalidSavedTeamCommitResult, err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

type savedTeamCommitDigestPayload struct {
	Committed             bool                       `json:"committed"`
	SourceRecordSetDigest string                     `json:"source_record_set_digest"`
	TeamEvent             savedTeamCommitDigestEvent `json:"team_event"`
	MainAgentEvent        savedTeamCommitDigestEvent `json:"main_agent_event"`
	EventCount            int                        `json:"event_count"`
}

type savedTeamCommitDigestEvent struct {
	ID             string `json:"id"`
	StreamID       string `json:"stream_id"`
	Seq            int64  `json:"seq"`
	IdempotencyKey string `json:"idempotency_key"`
	Type           string `json:"type"`
	SchemaVersion  int    `json:"schema_version"`
	EmittedAt      string `json:"emitted_at"`
	CorrelationID  string `json:"correlation_id"`
	CausationID    string `json:"causation_id"`
	PayloadJSON    []byte `json:"payload_json"`
}

func canonicalSavedTeamCommitDigestEvent(input journal.Event) savedTeamCommitDigestEvent {
	return savedTeamCommitDigestEvent{
		ID:             input.ID,
		StreamID:       input.StreamID,
		Seq:            input.Seq,
		IdempotencyKey: input.IdempotencyKey,
		Type:           input.Type,
		SchemaVersion:  input.SchemaVersion,
		EmittedAt:      input.EmittedAt.UTC().Format(time.RFC3339Nano),
		CorrelationID:  input.CorrelationID,
		CausationID:    input.CausationID,
		PayloadJSON:    append([]byte(nil), input.PayloadJSON...),
	}
}

func sameSavedTeamCommitEvent(left, right journal.Event) bool {
	return left.ID == right.ID &&
		left.StreamID == right.StreamID &&
		left.Seq == right.Seq &&
		left.IdempotencyKey == right.IdempotencyKey &&
		left.Type == right.Type &&
		left.SchemaVersion == right.SchemaVersion &&
		left.EmittedAt.Equal(right.EmittedAt) &&
		left.EmittedAt.Location() == time.UTC &&
		right.EmittedAt.Location() == time.UTC &&
		left.CorrelationID == right.CorrelationID &&
		left.CausationID == right.CausationID &&
		bytes.Equal(left.PayloadJSON, right.PayloadJSON)
}

func isNilEventBatchAppender(appender EventBatchAppender) bool {
	if appender == nil {
		return true
	}
	value := reflect.ValueOf(appender)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func cloneSavedTeamCommitEvents(input []journal.Event) []journal.Event {
	result := make([]journal.Event, len(input))
	for index, event := range input {
		result[index] = cloneSavedTeamCommitEvent(event)
	}
	return result
}

func cloneSavedTeamCommitEvent(input journal.Event) journal.Event {
	input.PayloadJSON = append([]byte(nil), input.PayloadJSON...)
	return input
}

func (c SavedTeamCommitCandidate) Committed() bool {
	return c.committed
}

func (c SavedTeamCommitCandidate) SourceRecordSetDigest() string {
	return c.sourceRecordSetDigest
}

func (c SavedTeamCommitCandidate) TeamEvent() journal.Event {
	return cloneSavedTeamCommitEvent(c.teamEvent)
}

func (c SavedTeamCommitCandidate) MainAgentEvent() journal.Event {
	return cloneSavedTeamCommitEvent(c.mainAgentEvent)
}

func (c SavedTeamCommitCandidate) EventCount() int {
	return c.eventCount
}

func (c SavedTeamCommitCandidate) CommitDigest() string {
	return c.commitDigest
}
