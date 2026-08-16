package execution

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/permissions"
)

type ToolRecoveryAction string

const (
	ToolRecoveryAbortAttempt         ToolRecoveryAction = "abort_attempt"
	ToolRecoveryAcceptObservedEffect ToolRecoveryAction = "accept_observed_effect"
	ToolRecoveryRetryInNewAttempt    ToolRecoveryAction = "retry_in_new_attempt"
)

type ToolRecoveryCandidateStatus string

const (
	ToolRecoveryCandidateAvailable ToolRecoveryCandidateStatus = "available"
	ToolRecoveryCandidateResolved  ToolRecoveryCandidateStatus = "resolved"
)

type ToolRecoveryCandidate struct {
	SchemaVersion      int
	Status             ToolRecoveryCandidateStatus
	CandidateDigest    string
	DecisionID         string
	Decision           ToolRecoveryAction
	ExecutionID        string
	JobID              string
	CallDigest         string
	Tool               permissions.ToolKind
	Generation         int64
	OperationID        string
	IncidentID         string
	RecoveryCode       string
	RecoveryRequiredAt string
	AvailableActions   []ToolRecoveryAction
}

type ToolRecoveryPreview struct {
	SchemaVersion int
	Candidates    []ToolRecoveryCandidate
}

type ToolRecoveryDecisionInput struct {
	SchemaVersion   int
	DecisionID      string
	CorrelationID   string
	PrincipalID     string
	Action          ToolRecoveryAction
	CandidateDigest string
}

type ToolRecoveryObservation struct {
	SchemaVersion      int
	CandidateDigest    string
	EvidenceID         string
	ObservationDigest  string
	OutputDigest       string
	ChangedFilesDigest string
}

type ToolRecoveryReplacement struct {
	SchemaVersion          int
	CandidateDigest        string
	AttemptID              string
	RunID                  string
	ExecutionBindingDigest string
	ContextCapsuleDigest   string
}

type ToolRecoveryDecision struct {
	SchemaVersion          int
	DecisionID             string
	EventID                string
	StreamID               string
	Action                 ToolRecoveryAction
	CandidateDigest        string
	ExecutionID            string
	EvidenceID             string
	ObservationDigest      string
	OutputDigest           string
	ChangedFilesDigest     string
	ReplacementAttemptID   string
	ReplacementRunID       string
	ExecutionBindingDigest string
	ContextCapsuleDigest   string
}

type ToolRecoveryObservationResolver interface {
	ResolveToolRecoveryObservation(context.Context, ToolRecoveryCandidate) (ToolRecoveryObservation, error)
}

type ToolRecoveryReplacementResolver interface {
	ResolveToolRecoveryReplacement(context.Context, ToolRecoveryCandidate) (ToolRecoveryReplacement, error)
}

type ToolRecoveryAuthority struct {
	store        *journal.Store
	observations ToolRecoveryObservationResolver
	replacements ToolRecoveryReplacementResolver
	now          func() time.Time
}

type toolRecoveryResolvedPayloadV2 struct {
	ExecutionID            string             `json:"execution_id"`
	DecisionID             string             `json:"decision_id"`
	PrincipalID            string             `json:"principal_id"`
	Action                 ToolRecoveryAction `json:"action"`
	CandidateDigest        string             `json:"candidate_digest"`
	ResolvedAt             string             `json:"resolved_at"`
	EvidenceID             string             `json:"evidence_id,omitempty"`
	ObservationDigest      string             `json:"observation_digest,omitempty"`
	OutputDigest           string             `json:"output_digest,omitempty"`
	ChangedFilesDigest     string             `json:"changed_files_digest,omitempty"`
	ReplacementAttemptID   string             `json:"replacement_attempt_id,omitempty"`
	ReplacementRunID       string             `json:"replacement_run_id,omitempty"`
	ExecutionBindingDigest string             `json:"execution_binding_digest,omitempty"`
	ContextCapsuleDigest   string             `json:"context_capsule_digest,omitempty"`
}

func NewToolRecoveryAuthority(
	store *journal.Store,
	observations ToolRecoveryObservationResolver,
	replacements ToolRecoveryReplacementResolver,
	now func() time.Time,
) (*ToolRecoveryAuthority, error) {
	if store == nil || now == nil {
		return nil, ErrInvalidToolRecovery
	}
	if isNilToolRecoveryCapability(observations) {
		observations = nil
	}
	if isNilToolRecoveryCapability(replacements) {
		replacements = nil
	}
	return &ToolRecoveryAuthority{
		store: store, observations: observations, replacements: replacements, now: now,
	}, nil
}

func isNilToolRecoveryCapability(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

// Preview is content-free and read-only. Resolver presence advertises a
// possible governed action; no observation or replacement authority is minted.
func (authority *ToolRecoveryAuthority) Preview(ctx context.Context) (ToolRecoveryPreview, error) {
	if authority == nil || authority.store == nil || ctx == nil || ctx.Err() != nil {
		return ToolRecoveryPreview{}, ErrInvalidToolRecovery
	}
	events, err := authority.store.ReadAll(ctx)
	if err != nil {
		return ToolRecoveryPreview{}, err
	}
	snapshot, err := ReplaySnapshot(events)
	if err != nil {
		return ToolRecoveryPreview{}, err
	}
	preview := ToolRecoveryPreview{SchemaVersion: 1}
	for _, record := range snapshot.Records {
		if record.Status != "recovery_required" &&
			record.Status != "recovery_aborted" &&
			record.Status != "recovery_effect_accepted" &&
			record.Status != "recovery_retry_authorized" {
			continue
		}
		candidate := toolRecoveryCandidate(record, authority)
		preview.Candidates = append(preview.Candidates, candidate)
	}
	sort.Slice(preview.Candidates, func(left, right int) bool {
		return preview.Candidates[left].CandidateDigest < preview.Candidates[right].CandidateDigest
	})
	return preview, nil
}

// Resolve records one exact human-governed resolution. It has no executor,
// Tool Gateway, Runtime, Provider, credential, or payload-content dependency.
func (authority *ToolRecoveryAuthority) Resolve(
	ctx context.Context,
	input ToolRecoveryDecisionInput,
) (ToolRecoveryDecision, error) {
	if authority == nil || authority.store == nil || authority.now == nil ||
		ctx == nil || ctx.Err() != nil || !validToolRecoveryDecisionInput(input) {
		return ToolRecoveryDecision{}, ErrInvalidToolRecovery
	}
	preview, err := authority.Preview(ctx)
	if err != nil {
		return ToolRecoveryDecision{}, err
	}
	candidate, found := toolRecoveryCandidateByDigest(preview, input.CandidateDigest)
	if !found {
		return ToolRecoveryDecision{}, ErrToolRecoveryConflict
	}
	streamID := executionStreamID(candidate.JobID, candidate.ExecutionID)
	if candidate.Status == ToolRecoveryCandidateResolved {
		return authority.replayExact(ctx, streamID, input)
	}

	payload := toolRecoveryResolvedPayloadV2{
		ExecutionID: candidate.ExecutionID, DecisionID: input.DecisionID,
		PrincipalID: input.PrincipalID, Action: input.Action,
		CandidateDigest: input.CandidateDigest,
	}
	switch input.Action {
	case ToolRecoveryAbortAttempt:
	case ToolRecoveryAcceptObservedEffect:
		if authority.observations == nil {
			return ToolRecoveryDecision{}, ErrToolRecoveryUnavailable
		}
		observation, resolveErr := authority.observations.ResolveToolRecoveryObservation(ctx, candidate)
		if resolveErr != nil || !validToolRecoveryObservation(observation, candidate) {
			return ToolRecoveryDecision{}, errors.Join(ErrToolRecoveryEvidence, resolveErr)
		}
		payload.EvidenceID = observation.EvidenceID
		payload.ObservationDigest = observation.ObservationDigest
		payload.OutputDigest = observation.OutputDigest
		payload.ChangedFilesDigest = observation.ChangedFilesDigest
	case ToolRecoveryRetryInNewAttempt:
		if authority.replacements == nil {
			return ToolRecoveryDecision{}, ErrToolRecoveryUnavailable
		}
		replacement, resolveErr := authority.replacements.ResolveToolRecoveryReplacement(ctx, candidate)
		if resolveErr != nil || !validToolRecoveryReplacement(replacement, candidate) {
			return ToolRecoveryDecision{}, errors.Join(ErrToolRecoveryEvidence, resolveErr)
		}
		payload.ReplacementAttemptID = replacement.AttemptID
		payload.ReplacementRunID = replacement.RunID
		payload.ExecutionBindingDigest = replacement.ExecutionBindingDigest
		payload.ContextCapsuleDigest = replacement.ContextCapsuleDigest
	default:
		return ToolRecoveryDecision{}, ErrInvalidToolRecovery
	}

	stream, err := authority.store.ReadStreamSet(ctx, []string{streamID})
	if err != nil {
		return ToolRecoveryDecision{}, err
	}
	currentSnapshot, err := ReplaySnapshot(stream.Events())
	if err != nil {
		return ToolRecoveryDecision{}, err
	}
	current, ok := currentSnapshot.Record(candidate.ExecutionID)
	if !ok || current.Status != "recovery_required" ||
		toolRecoveryCandidateDigest(current) != input.CandidateDigest {
		return authority.replayExact(ctx, streamID, input)
	}
	emittedAt := authority.now().UTC()
	if emittedAt.IsZero() {
		return ToolRecoveryDecision{}, ErrInvalidToolRecovery
	}
	payload.ResolvedAt = emittedAt.Format(time.RFC3339Nano)
	body, err := json.Marshal(payload)
	if err != nil {
		return ToolRecoveryDecision{}, err
	}
	head, _ := stream.Head(streamID)
	eventID := deterministicEventID(
		"tool-recovery", EventToolRecoveryResolved, streamID,
		input.DecisionID+"/"+input.CandidateDigest+"/"+string(input.Action),
	)
	event := journal.Event{
		ID: eventID, StreamID: streamID, Seq: head.Sequence + 1,
		IdempotencyKey: "tool-recovery/" + input.DecisionID,
		Type:           EventToolRecoveryResolved, SchemaVersion: 2,
		EmittedAt: emittedAt, CorrelationID: input.CorrelationID,
		CausationID: head.EventID, PayloadJSON: body,
	}
	if _, err := authority.store.AppendBatchIfStreamHeads(
		ctx,
		[]journal.StreamHeadExpectation{{StreamID: streamID, Sequence: head.Sequence}},
		[]journal.Event{event},
	); err != nil {
		if errors.Is(err, journal.ErrStreamHeadConflict) ||
			errors.Is(err, journal.ErrIdempotencyConflict) ||
			errors.Is(err, journal.ErrSequenceConflict) {
			return authority.replayExact(ctx, streamID, input)
		}
		return ToolRecoveryDecision{}, err
	}
	return toolRecoveryDecisionFromPayload(event, payload), nil
}

func (authority *ToolRecoveryAuthority) replayExact(
	ctx context.Context,
	streamID string,
	input ToolRecoveryDecisionInput,
) (ToolRecoveryDecision, error) {
	events, err := authority.store.ReadStream(ctx, streamID)
	if err != nil {
		return ToolRecoveryDecision{}, err
	}
	for _, event := range events {
		if event.Type != EventToolRecoveryResolved {
			continue
		}
		var payload toolRecoveryResolvedPayloadV2
		if decodeExecutionPayload(event, &payload) != nil || !validToolRecoveryResolvedPayload(payload) {
			return ToolRecoveryDecision{}, ErrInvalidExecutionEvent
		}
		if payload.DecisionID == input.DecisionID && payload.Action == input.Action &&
			payload.CandidateDigest == input.CandidateDigest {
			return toolRecoveryDecisionFromPayload(event, payload), nil
		}
		return ToolRecoveryDecision{}, ErrToolRecoveryConflict
	}
	return ToolRecoveryDecision{}, ErrToolRecoveryConflict
}

func toolRecoveryCandidate(record ExecutionRecord, authority *ToolRecoveryAuthority) ToolRecoveryCandidate {
	status := ToolRecoveryCandidateAvailable
	actions := []ToolRecoveryAction{ToolRecoveryAbortAttempt}
	if authority.observations != nil {
		actions = append(actions, ToolRecoveryAcceptObservedEffect)
	}
	if authority.replacements != nil {
		actions = append(actions, ToolRecoveryRetryInNewAttempt)
	}
	if record.Status != "recovery_required" {
		status = ToolRecoveryCandidateResolved
		actions = nil
	}
	return ToolRecoveryCandidate{
		SchemaVersion: 1, Status: status,
		CandidateDigest: toolRecoveryCandidateDigest(record),
		DecisionID:      record.RecoveryDecisionID,
		Decision:        ToolRecoveryAction(record.RecoveryDecision),
		ExecutionID:     record.ExecutionID, JobID: record.JobID,
		CallDigest: record.CallDigest, Tool: record.Tool, Generation: record.Generation,
		OperationID: record.OperationID, IncidentID: record.JourneyID,
		RecoveryCode:       record.RecoveryCode,
		RecoveryRequiredAt: record.RecoveryRequiredAt,
		AvailableActions:   actions,
	}
}

func toolRecoveryCandidateDigest(record ExecutionRecord) string {
	values := []string{
		"loom/tool-recovery-candidate/v1", record.ExecutionID, record.JobID,
		record.CallDigest, string(record.Tool), fmt.Sprint(record.Generation),
		record.OperationID, record.JourneyID, record.AllowedAt,
		record.RecoveryRequiredAt, record.RecoveryCode, record.RecoveryAction,
		record.recoveryEventID,
	}
	sum := sha256.Sum256([]byte(strings.Join(values, "\n")))
	return hex.EncodeToString(sum[:])
}

func toolRecoveryCandidateByDigest(
	preview ToolRecoveryPreview,
	digest string,
) (ToolRecoveryCandidate, bool) {
	for _, candidate := range preview.Candidates {
		if candidate.CandidateDigest == digest {
			return candidate, true
		}
	}
	return ToolRecoveryCandidate{}, false
}

func toolRecoveryDecisionFromPayload(
	event journal.Event,
	payload toolRecoveryResolvedPayloadV2,
) ToolRecoveryDecision {
	return ToolRecoveryDecision{
		SchemaVersion: 1, DecisionID: payload.DecisionID,
		EventID: event.ID, StreamID: event.StreamID, Action: payload.Action,
		CandidateDigest: payload.CandidateDigest, ExecutionID: payload.ExecutionID,
		EvidenceID: payload.EvidenceID, ObservationDigest: payload.ObservationDigest,
		OutputDigest: payload.OutputDigest, ChangedFilesDigest: payload.ChangedFilesDigest,
		ReplacementAttemptID:   payload.ReplacementAttemptID,
		ReplacementRunID:       payload.ReplacementRunID,
		ExecutionBindingDigest: payload.ExecutionBindingDigest,
		ContextCapsuleDigest:   payload.ContextCapsuleDigest,
	}
}

func validToolRecoveryDecisionInput(input ToolRecoveryDecisionInput) bool {
	return input.SchemaVersion == 1 && validToolRecoveryIdentifier(input.DecisionID) &&
		validToolRecoveryIdentifier(input.CorrelationID) &&
		validToolRecoveryIdentifier(input.PrincipalID) && validHexDigest(input.CandidateDigest) &&
		(input.Action == ToolRecoveryAbortAttempt ||
			input.Action == ToolRecoveryAcceptObservedEffect ||
			input.Action == ToolRecoveryRetryInNewAttempt)
}

func validToolRecoveryObservation(
	observation ToolRecoveryObservation,
	candidate ToolRecoveryCandidate,
) bool {
	return observation.SchemaVersion == 1 &&
		observation.CandidateDigest == candidate.CandidateDigest &&
		validToolRecoveryIdentifier(observation.EvidenceID) &&
		validHexDigest(observation.ObservationDigest) &&
		validSHA256Digest(observation.OutputDigest) &&
		validSHA256Digest(observation.ChangedFilesDigest)
}

func validToolRecoveryReplacement(
	replacement ToolRecoveryReplacement,
	candidate ToolRecoveryCandidate,
) bool {
	return replacement.SchemaVersion == 1 &&
		replacement.CandidateDigest == candidate.CandidateDigest &&
		validToolRecoveryIdentifier(replacement.AttemptID) &&
		validToolRecoveryIdentifier(replacement.RunID) &&
		validHexDigest(replacement.ExecutionBindingDigest) &&
		validHexDigest(replacement.ContextCapsuleDigest)
}

func validToolRecoveryResolvedPayload(payload toolRecoveryResolvedPayloadV2) bool {
	if !validToolRecoveryIdentifier(payload.ExecutionID) ||
		!validToolRecoveryIdentifier(payload.DecisionID) ||
		!validToolRecoveryIdentifier(payload.PrincipalID) ||
		!validHexDigest(payload.CandidateDigest) || payload.ResolvedAt == "" {
		return false
	}
	switch payload.Action {
	case ToolRecoveryAbortAttempt:
		return payload.EvidenceID == "" && payload.ObservationDigest == "" &&
			payload.OutputDigest == "" && payload.ChangedFilesDigest == "" &&
			payload.ReplacementAttemptID == "" && payload.ReplacementRunID == "" &&
			payload.ExecutionBindingDigest == "" && payload.ContextCapsuleDigest == ""
	case ToolRecoveryAcceptObservedEffect:
		return validToolRecoveryIdentifier(payload.EvidenceID) &&
			validHexDigest(payload.ObservationDigest) &&
			validSHA256Digest(payload.OutputDigest) &&
			validSHA256Digest(payload.ChangedFilesDigest) &&
			payload.ReplacementAttemptID == "" && payload.ReplacementRunID == "" &&
			payload.ExecutionBindingDigest == "" && payload.ContextCapsuleDigest == ""
	case ToolRecoveryRetryInNewAttempt:
		return payload.EvidenceID == "" && payload.ObservationDigest == "" &&
			payload.OutputDigest == "" && payload.ChangedFilesDigest == "" &&
			validToolRecoveryIdentifier(payload.ReplacementAttemptID) &&
			validToolRecoveryIdentifier(payload.ReplacementRunID) &&
			validHexDigest(payload.ExecutionBindingDigest) &&
			validHexDigest(payload.ContextCapsuleDigest)
	default:
		return false
	}
}

func validToolRecoveryIdentifier(value string) bool {
	if value == "" || len(value) > 160 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			strings.ContainsRune("-._:/", character) {
			continue
		}
		return false
	}
	return true
}

func validHexDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validSHA256Digest(value string) bool {
	return strings.HasPrefix(value, "sha256:") && validHexDigest(strings.TrimPrefix(value, "sha256:"))
}
