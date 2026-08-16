package work

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"loom-pi-rebuild/internal/journal"
	loomruntime "loom-pi-rebuild/internal/runtime"
)

var (
	ErrInvalidAgentAttemptRecovery    = errors.New("invalid Agent Attempt recovery decision")
	ErrAgentAttemptRecoveryConflict   = errors.New("Agent Attempt recovery decision conflict")
	ErrAgentAttemptRecoveryUnsafe     = errors.New("Agent Attempt recovery is not safe to authorize")
	ErrAgentAttemptRecoveryCapability = errors.New("Agent Attempt recovery capability is invalid")
	ErrAgentAttemptRecoveryConsumed   = errors.New("Agent Attempt recovery decision already consumed")
)

type AgentAttemptRecoveryAction string

const AgentAttemptRecoveryResumePreModel AgentAttemptRecoveryAction = "resume_pre_model"

type AgentAttemptRecoveryCandidateStatus string

const (
	AgentAttemptRecoveryCandidateAvailable  AgentAttemptRecoveryCandidateStatus = "available"
	AgentAttemptRecoveryCandidateAuthorized AgentAttemptRecoveryCandidateStatus = "authorized"
	AgentAttemptRecoveryCandidateConsumed   AgentAttemptRecoveryCandidateStatus = "consumed"
)

type AgentAttemptRecoveryCandidate struct {
	SchemaVersion      int
	Status             AgentAttemptRecoveryCandidateStatus
	Action             AgentAttemptRecoveryAction
	DecisionID         string
	CandidateDigest    string
	CapabilityDigest   string
	AttemptID          string
	TeamInstanceID     string
	SegmentID          string
	WorkItemID         string
	RunID              string
	ClaimGeneration    int64
	RuntimeInstanceID  string
	AgentInstanceID    string
	HarnessAdapter     string
	ProviderID         string
	ProviderAccountID  string
	ModelID            string
	CredentialRevision int64
}

type AgentAttemptRecoveryPreview struct {
	SchemaVersion int
	Candidates    []AgentAttemptRecoveryCandidate
}

type AgentAttemptRestartResumeMode string

const AgentAttemptRestartResumeFromCheckpoint AgentAttemptRestartResumeMode = "restart_safe_checkpoint"

// AgentAttemptRestartCapability is a content-free, Runtime-minted conformance
// statement. It is not a process handle or dispatch authority.
type AgentAttemptRestartCapability struct {
	SchemaVersion          int
	AttemptID              string
	RuntimeInstanceID      string
	ExecutionBindingDigest string
	ContextCapsuleDigest   string
	ResumeMode             AgentAttemptRestartResumeMode
	SessionBindingDigest   string
	CapabilityDigest       string
}

type AgentAttemptRecoveryDecisionInput struct {
	SchemaVersion    int
	DecisionID       string
	CorrelationID    string
	PrincipalID      string
	Action           AgentAttemptRecoveryAction
	CandidateDigest  string
	CapabilityDigest string
}

type AgentAttemptRecoveryDecision struct {
	SchemaVersion    int
	DecisionID       string
	EventID          string
	StreamID         string
	Action           AgentAttemptRecoveryAction
	CandidateDigest  string
	CapabilityDigest string
}

type AgentAttemptRecoveryConsumeInput struct {
	SchemaVersion    int
	CorrelationID    string
	DecisionID       string
	CandidateDigest  string
	CapabilityDigest string
}

type AgentAttemptRecoveryDispatchGrant struct {
	outcome         AgentAttemptRestartOutcome
	capability      AgentAttemptRestartCapability
	incidentID      string
	decisionID      string
	leaseID         string
	consumedEventID string
}

type AgentAttemptRecoveryDispatchLease struct {
	mu       sync.Mutex
	consumed bool
	grant    AgentAttemptRecoveryDispatchGrant
}

type AgentAttemptRestartCapabilityResolver interface {
	ResolveAgentAttemptRestartCapability(
		context.Context,
		AgentAttemptRestartOutcome,
	) (AgentAttemptRestartCapability, error)
}

type AgentAttemptRecoveryAuthority struct {
	coordinator  *AgentInboxCoordinator
	store        *journal.Store
	capabilities AgentAttemptRestartCapabilityResolver
	now          func() time.Time
	randomMu     sync.Mutex
	random       io.Reader
}

type agentAttemptRecoveryEventPayload struct {
	SchemaVersion          int                           `json:"schema_version"`
	DecisionID             string                        `json:"decision_id"`
	PrincipalID            string                        `json:"principal_id"`
	Action                 AgentAttemptRecoveryAction    `json:"action"`
	CandidateDigest        string                        `json:"candidate_digest"`
	CapabilityDigest       string                        `json:"capability_digest"`
	AttemptID              string                        `json:"attempt_id"`
	RunID                  string                        `json:"run_id"`
	ClaimGeneration        int64                         `json:"claim_generation"`
	RuntimeInstanceID      string                        `json:"runtime_instance_id"`
	AgentInstanceID        string                        `json:"agent_instance_id"`
	ExecutionBindingDigest string                        `json:"execution_binding_digest"`
	ContextCapsuleDigest   string                        `json:"context_capsule_digest"`
	SegmentID              string                        `json:"segment_id"`
	CheckpointDigest       string                        `json:"checkpoint_digest"`
	TurnID                 string                        `json:"turn_id"`
	TurnSequence           int                           `json:"turn_sequence"`
	StepID                 string                        `json:"step_id,omitempty"`
	StepSequence           int                           `json:"step_sequence,omitempty"`
	RuntimeSessionDigest   string                        `json:"runtime_session_digest"`
	RuntimeResumeMode      AgentAttemptRestartResumeMode `json:"runtime_resume_mode"`
}

type agentAttemptRecoveryConsumedPayload struct {
	SchemaVersion     int    `json:"schema_version"`
	LeaseID           string `json:"lease_id"`
	DecisionID        string `json:"decision_id"`
	CandidateDigest   string `json:"candidate_digest"`
	CapabilityDigest  string `json:"capability_digest"`
	AttemptID         string `json:"attempt_id"`
	RunID             string `json:"run_id"`
	ClaimGeneration   int64  `json:"claim_generation"`
	RuntimeInstanceID string `json:"runtime_instance_id"`
	AgentInstanceID   string `json:"agent_instance_id"`
}

func NewAgentAttemptRecoveryAuthority(
	coordinator *AgentInboxCoordinator,
	capabilities AgentAttemptRestartCapabilityResolver,
	now func() time.Time,
	random io.Reader,
) (*AgentAttemptRecoveryAuthority, error) {
	if coordinator == nil || coordinator.authority == nil ||
		coordinator.authority.loops == nil || coordinator.authority.loops.runs == nil ||
		coordinator.authority.loops.runs.store == nil || nilInterface(capabilities) ||
		now == nil || nilInterface(random) {
		return nil, ErrInvalidAgentAttemptRecovery
	}
	return &AgentAttemptRecoveryAuthority{
		coordinator:  coordinator,
		store:        coordinator.authority.loops.runs.store,
		capabilities: capabilities,
		now:          now,
		random:       random,
	}, nil
}

// Preview returns content-free, currently resumable candidates and their
// durable decision state. It writes no recovery authorization fact and mints no
// dispatch lease.
func (authority *AgentAttemptRecoveryAuthority) Preview(
	ctx context.Context,
) (AgentAttemptRecoveryPreview, error) {
	if authority == nil || authority.coordinator == nil || authority.store == nil ||
		nilInterface(authority.capabilities) || ctx == nil || ctx.Err() != nil {
		return AgentAttemptRecoveryPreview{}, ErrInvalidAgentAttemptRecovery
	}
	report, err := authority.coordinator.RecoverAfterRestart(ctx)
	if err != nil {
		return AgentAttemptRecoveryPreview{}, err
	}
	preview := AgentAttemptRecoveryPreview{
		SchemaVersion: 1,
		Candidates:    make([]AgentAttemptRecoveryCandidate, 0, len(report.Outcomes)),
	}
	for _, outcome := range report.Outcomes {
		if outcome.Disposition != AgentAttemptRestartPreModelResume {
			continue
		}
		capability, capabilityErr := authority.capabilities.ResolveAgentAttemptRestartCapability(
			ctx, outcome,
		)
		if capabilityErr != nil || !validAgentAttemptRestartCapability(capability) ||
			!agentAttemptRestartCapabilityMatches(capability, outcome) {
			return AgentAttemptRecoveryPreview{}, ErrAgentAttemptRecoveryCapability
		}
		events, readErr := authority.store.ReadStream(
			ctx, agentAttemptRecoveryStream(outcome.Binding),
		)
		if readErr != nil {
			return AgentAttemptRecoveryPreview{}, readErr
		}
		status, decisionID, stateErr := agentAttemptRecoveryCandidateState(
			events, outcome, capability,
		)
		if stateErr != nil {
			return AgentAttemptRecoveryPreview{}, stateErr
		}
		binding := outcome.Binding.PayloadAuthority
		preview.Candidates = append(preview.Candidates, AgentAttemptRecoveryCandidate{
			SchemaVersion: 1, Status: status,
			Action: AgentAttemptRecoveryResumePreModel, DecisionID: decisionID,
			CandidateDigest:  AgentAttemptRestartCandidateDigest(outcome),
			CapabilityDigest: capability.CapabilityDigest,
			AttemptID:        outcome.Binding.AttemptID, TeamInstanceID: outcome.Binding.TeamInstanceID,
			SegmentID: outcome.SegmentID, WorkItemID: binding.WorkItemID,
			RunID: binding.RunID, ClaimGeneration: binding.ClaimGeneration,
			RuntimeInstanceID:  binding.RuntimeInstanceID,
			AgentInstanceID:    binding.AgentInstanceID,
			HarnessAdapter:     outcome.ExecutionBinding.HarnessAdapter,
			ProviderID:         outcome.ExecutionBinding.ProviderID,
			ProviderAccountID:  outcome.ExecutionBinding.ProviderAccountID,
			ModelID:            outcome.ExecutionBinding.ModelID,
			CredentialRevision: outcome.ExecutionBinding.CredentialRevision,
		})
	}
	sort.Slice(preview.Candidates, func(left, right int) bool {
		return preview.Candidates[left].CandidateDigest <
			preview.Candidates[right].CandidateDigest
	})
	return preview, nil
}

// Authorize records an explicit user decision. It does not register an active
// Attempt, claim a native session, or dispatch a Runtime/Provider request.
func (authority *AgentAttemptRecoveryAuthority) Authorize(
	ctx context.Context,
	input AgentAttemptRecoveryDecisionInput,
) (AgentAttemptRecoveryDecision, error) {
	if authority == nil || authority.coordinator == nil || authority.store == nil ||
		nilInterface(authority.capabilities) || authority.now == nil || ctx == nil || ctx.Err() != nil ||
		!validAgentAttemptRecoveryDecisionInput(input) {
		return AgentAttemptRecoveryDecision{}, ErrInvalidAgentAttemptRecovery
	}

	report, err := authority.coordinator.RecoverAfterRestart(ctx)
	if err != nil {
		return AgentAttemptRecoveryDecision{}, err
	}
	outcome, found := agentAttemptRestartOutcomeByDigest(report, input.CandidateDigest)
	if !found {
		return AgentAttemptRecoveryDecision{}, ErrAgentAttemptRecoveryConflict
	}
	if outcome.Disposition != AgentAttemptRestartPreModelResume {
		return AgentAttemptRecoveryDecision{}, ErrAgentAttemptRecoveryUnsafe
	}
	capability, err := authority.capabilities.ResolveAgentAttemptRestartCapability(ctx, outcome)
	if err != nil || !validAgentAttemptRestartCapability(capability) ||
		capability.CapabilityDigest != input.CapabilityDigest {
		return AgentAttemptRecoveryDecision{}, ErrAgentAttemptRecoveryCapability
	}
	if !validAgentAttemptRestartPreModelOutcome(outcome) ||
		!agentAttemptRestartCapabilityMatches(capability, outcome) {
		return AgentAttemptRecoveryDecision{}, ErrAgentAttemptRecoveryCapability
	}

	recoveryStream := agentAttemptRecoveryStream(outcome.Binding)
	streams := []string{
		recoveryStream,
		attemptLoopStream(outcome.Binding),
		runStream(outcome.Binding.PayloadAuthority.RunID),
	}
	snapshot, err := authority.store.ReadStreamSet(ctx, streams)
	if err != nil {
		return AgentAttemptRecoveryDecision{}, err
	}
	if recoveryHead, _ := snapshot.Head(recoveryStream); recoveryHead.Sequence != 0 {
		return replayExactAgentAttemptRecoveryDecision(
			snapshot.Events(), recoveryStream, input, capability, outcome,
		)
	}
	confirmed, err := authority.coordinator.RecoverAfterRestart(ctx)
	if err != nil {
		return AgentAttemptRecoveryDecision{}, err
	}
	confirmedOutcome, confirmedFound := agentAttemptRestartOutcomeByDigest(
		confirmed, input.CandidateDigest,
	)
	if !confirmedFound || AgentAttemptRestartCandidateDigest(confirmedOutcome) !=
		AgentAttemptRestartCandidateDigest(outcome) {
		return AgentAttemptRecoveryDecision{}, ErrAgentAttemptRecoveryConflict
	}
	confirmedCapability, err := authority.capabilities.ResolveAgentAttemptRestartCapability(
		ctx, confirmedOutcome,
	)
	if err != nil || !validAgentAttemptRestartCapability(confirmedCapability) ||
		confirmedCapability != capability {
		return AgentAttemptRecoveryDecision{}, ErrAgentAttemptRecoveryCapability
	}

	emittedAt := authority.now().UTC()
	if emittedAt.IsZero() {
		return AgentAttemptRecoveryDecision{}, ErrInvalidAgentAttemptRecovery
	}
	payload := agentAttemptRecoveryPayload(input, capability, outcome)
	eventID := deterministicEventID(
		"agent-attempt-recovery-authorized", input.DecisionID,
		input.CandidateDigest, input.CapabilityDigest,
	)
	attemptHead, found := snapshot.Head(attemptLoopStream(outcome.Binding))
	if !found || attemptHead.EventID == "" {
		return AgentAttemptRecoveryDecision{}, ErrAgentAttemptRecoveryConflict
	}
	event := newEvent(
		eventID, recoveryStream, 1, "AgentAttemptRecoveryAuthorized", emittedAt,
		input.CorrelationID, attemptHead.EventID, payload,
	)
	expectations := make([]journal.StreamHeadExpectation, 0, len(streams))
	for _, streamID := range streams {
		head, _ := snapshot.Head(streamID)
		expectations = append(expectations, journal.StreamHeadExpectation{
			StreamID: streamID, Sequence: head.Sequence,
		})
	}
	if _, err := authority.store.AppendBatchIfStreamHeads(
		ctx, expectations, []journal.Event{event},
	); err != nil {
		if errors.Is(err, journal.ErrStreamHeadConflict) ||
			errors.Is(err, journal.ErrSequenceConflict) ||
			errors.Is(err, journal.ErrIdempotencyConflict) {
			return AgentAttemptRecoveryDecision{}, ErrAgentAttemptRecoveryConflict
		}
		return AgentAttemptRecoveryDecision{}, err
	}
	return agentAttemptRecoveryDecision(event, payload), nil
}

// Consume closes the durable decision exactly once and returns an in-process
// lease. It still does not register an active Attempt or dispatch a Runtime.
func (authority *AgentAttemptRecoveryAuthority) Consume(
	ctx context.Context,
	input AgentAttemptRecoveryConsumeInput,
) (*AgentAttemptRecoveryDispatchLease, error) {
	if authority == nil || authority.coordinator == nil || authority.store == nil ||
		nilInterface(authority.capabilities) || authority.now == nil ||
		nilInterface(authority.random) || ctx == nil || ctx.Err() != nil ||
		!validAgentAttemptRecoveryConsumeInput(input) {
		return nil, ErrInvalidAgentAttemptRecovery
	}
	report, err := authority.coordinator.RecoverAfterRestart(ctx)
	if err != nil {
		return nil, err
	}
	outcome, found := agentAttemptRestartOutcomeByDigest(report, input.CandidateDigest)
	if !found {
		return nil, ErrAgentAttemptRecoveryConflict
	}
	if outcome.Disposition != AgentAttemptRestartPreModelResume {
		return nil, ErrAgentAttemptRecoveryUnsafe
	}
	capability, err := authority.capabilities.ResolveAgentAttemptRestartCapability(ctx, outcome)
	if err != nil || !validAgentAttemptRestartCapability(capability) ||
		capability.CapabilityDigest != input.CapabilityDigest ||
		!agentAttemptRestartCapabilityMatches(capability, outcome) {
		return nil, ErrAgentAttemptRecoveryCapability
	}

	recoveryStream := agentAttemptRecoveryStream(outcome.Binding)
	loopStream := attemptLoopStream(outcome.Binding)
	runAuthorityStream := runStream(outcome.Binding.PayloadAuthority.RunID)
	streams := []string{recoveryStream, loopStream, runAuthorityStream}
	snapshot, err := authority.store.ReadStreamSet(ctx, streams)
	if err != nil {
		return nil, err
	}
	authorizedEvent, authorized, err := exactAgentAttemptRecoveryAuthorization(
		snapshot.Events(), recoveryStream,
	)
	if err != nil {
		return nil, err
	}
	if authorized.DecisionID != input.DecisionID ||
		authorized.CandidateDigest != input.CandidateDigest ||
		authorized.CapabilityDigest != input.CapabilityDigest {
		return nil, ErrAgentAttemptRecoveryConflict
	}

	confirmed, err := authority.coordinator.RecoverAfterRestart(ctx)
	if err != nil {
		return nil, err
	}
	confirmedOutcome, confirmedFound := agentAttemptRestartOutcomeByDigest(
		confirmed, input.CandidateDigest,
	)
	if !confirmedFound || AgentAttemptRestartCandidateDigest(confirmedOutcome) !=
		AgentAttemptRestartCandidateDigest(outcome) {
		return nil, ErrAgentAttemptRecoveryConflict
	}
	confirmedCapability, err := authority.capabilities.ResolveAgentAttemptRestartCapability(
		ctx, confirmedOutcome,
	)
	if err != nil || !validAgentAttemptRestartCapability(confirmedCapability) ||
		confirmedCapability != capability {
		return nil, ErrAgentAttemptRecoveryCapability
	}
	leaseID, err := authority.newRecoveryLeaseID()
	if err != nil {
		return nil, err
	}
	emittedAt := authority.now().UTC()
	if emittedAt.IsZero() {
		return nil, ErrInvalidAgentAttemptRecovery
	}
	payload := agentAttemptRecoveryConsumedPayload{
		SchemaVersion: 1, LeaseID: leaseID, DecisionID: input.DecisionID,
		CandidateDigest: input.CandidateDigest, CapabilityDigest: input.CapabilityDigest,
		AttemptID:         outcome.Binding.AttemptID,
		RunID:             outcome.Binding.PayloadAuthority.RunID,
		ClaimGeneration:   outcome.Binding.PayloadAuthority.ClaimGeneration,
		RuntimeInstanceID: outcome.Binding.PayloadAuthority.RuntimeInstanceID,
		AgentInstanceID:   outcome.Binding.PayloadAuthority.AgentInstanceID,
	}
	eventID := deterministicEventID(
		"agent-attempt-recovery-consumed", leaseID, input.DecisionID,
		input.CandidateDigest, input.CapabilityDigest,
	)
	event := newEvent(
		eventID, recoveryStream, 2, "AgentAttemptRecoveryConsumed", emittedAt,
		input.CorrelationID, authorizedEvent.ID, payload,
	)
	expectations := make([]journal.StreamHeadExpectation, 0, len(streams))
	for _, streamID := range streams {
		head, _ := snapshot.Head(streamID)
		expectations = append(expectations, journal.StreamHeadExpectation{
			StreamID: streamID, Sequence: head.Sequence,
		})
	}
	if _, err := authority.store.AppendBatchIfStreamHeads(
		ctx, expectations, []journal.Event{event},
	); err != nil {
		if errors.Is(err, journal.ErrStreamHeadConflict) ||
			errors.Is(err, journal.ErrSequenceConflict) ||
			errors.Is(err, journal.ErrIdempotencyConflict) {
			return nil, ErrAgentAttemptRecoveryConflict
		}
		return nil, err
	}
	return &AgentAttemptRecoveryDispatchLease{grant: AgentAttemptRecoveryDispatchGrant{
		outcome: cloneAgentAttemptRestartOutcome(outcome), capability: capability,
		incidentID: input.CorrelationID, decisionID: input.DecisionID,
		leaseID: leaseID, consumedEventID: eventID,
	}}, nil
}

func (lease *AgentAttemptRecoveryDispatchLease) LeaseID() string {
	if lease == nil {
		return ""
	}
	return lease.grant.leaseID
}

func (lease *AgentAttemptRecoveryDispatchLease) DecisionID() string {
	if lease == nil {
		return ""
	}
	return lease.grant.decisionID
}

func (lease *AgentAttemptRecoveryDispatchLease) CandidateDigest() string {
	if lease == nil {
		return ""
	}
	return AgentAttemptRestartCandidateDigest(lease.grant.outcome)
}

func (lease *AgentAttemptRecoveryDispatchLease) CapabilityDigest() string {
	if lease == nil {
		return ""
	}
	return lease.grant.capability.CapabilityDigest
}

func (lease *AgentAttemptRecoveryDispatchLease) AttemptID() string {
	if lease == nil {
		return ""
	}
	return lease.grant.outcome.Binding.AttemptID
}

func (lease *AgentAttemptRecoveryDispatchLease) RuntimeInstanceID() string {
	if lease == nil {
		return ""
	}
	return lease.grant.outcome.Binding.PayloadAuthority.RuntimeInstanceID
}

func (lease *AgentAttemptRecoveryDispatchLease) CheckpointDigest() string {
	if lease == nil {
		return ""
	}
	return lease.grant.outcome.CheckpointDigest
}

// Take exposes the frozen grant once to trusted product composition.
func (lease *AgentAttemptRecoveryDispatchLease) Take() (AgentAttemptRecoveryDispatchGrant, error) {
	if lease == nil {
		return AgentAttemptRecoveryDispatchGrant{}, ErrInvalidAgentAttemptRecovery
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if lease.consumed {
		return AgentAttemptRecoveryDispatchGrant{}, ErrAgentAttemptRecoveryConsumed
	}
	lease.consumed = true
	grant := lease.grant
	grant.outcome = cloneAgentAttemptRestartOutcome(grant.outcome)
	return grant, nil
}

func (grant AgentAttemptRecoveryDispatchGrant) Outcome() AgentAttemptRestartOutcome {
	return cloneAgentAttemptRestartOutcome(grant.outcome)
}

func (grant AgentAttemptRecoveryDispatchGrant) Capability() AgentAttemptRestartCapability {
	return grant.capability
}

func (grant AgentAttemptRecoveryDispatchGrant) LeaseID() string    { return grant.leaseID }
func (grant AgentAttemptRecoveryDispatchGrant) DecisionID() string { return grant.decisionID }
func (grant AgentAttemptRecoveryDispatchGrant) IncidentID() string { return grant.incidentID }
func (grant AgentAttemptRecoveryDispatchGrant) ConsumedEventID() string {
	return grant.consumedEventID
}

// ValidateAgentAttemptRecoveryDispatchGrant is the product-composition boundary
// for a consumed grant. It returns defensive copies and no dispatch authority.
func ValidateAgentAttemptRecoveryDispatchGrant(
	grant AgentAttemptRecoveryDispatchGrant,
) (AgentAttemptRestartOutcome, AgentAttemptRestartCapability, error) {
	outcome := grant.Outcome()
	capability := grant.Capability()
	if !validAgentAttemptRestartPreModelOutcome(outcome) ||
		!validAgentAttemptRestartCapability(capability) ||
		!agentAttemptRestartCapabilityMatches(capability, outcome) ||
		!validCanonicalUUID(grant.incidentID) || !validCanonicalUUID(grant.decisionID) ||
		!validOpaqueID(grant.leaseID) || !validOpaqueID(grant.consumedEventID) {
		return AgentAttemptRestartOutcome{}, AgentAttemptRestartCapability{},
			ErrInvalidAgentAttemptRecovery
	}
	return outcome, capability, nil
}

// ValidateAgentAttemptRestartOutcome is the content-free product-composition
// boundary used before a Runtime mints a restart capability.
func ValidateAgentAttemptRestartOutcome(
	outcome AgentAttemptRestartOutcome,
) (AgentAttemptRestartOutcome, error) {
	if !validAgentAttemptRestartPreModelOutcome(outcome) {
		return AgentAttemptRestartOutcome{}, ErrInvalidAgentAttemptRecovery
	}
	return cloneAgentAttemptRestartOutcome(outcome), nil
}

func AgentAttemptRestartCandidateDigest(outcome AgentAttemptRestartOutcome) string {
	inputIDs := append([]string(nil), outcome.InputIDs...)
	sort.Strings(inputIDs)
	canonical := struct {
		SchemaVersion          int                            `json:"schema_version"`
		AttemptID              string                         `json:"attempt_id"`
		TeamInstanceID         string                         `json:"team_instance_id"`
		RunID                  string                         `json:"run_id"`
		ClaimID                string                         `json:"claim_id"`
		ClaimGeneration        int64                          `json:"claim_generation"`
		RuntimeInstanceID      string                         `json:"runtime_instance_id"`
		AgentInstanceID        string                         `json:"agent_instance_id"`
		ExecutionBindingDigest string                         `json:"execution_binding_digest"`
		ContextCapsuleDigest   string                         `json:"context_capsule_digest"`
		SegmentID              string                         `json:"segment_id"`
		Disposition            AgentAttemptRestartDisposition `json:"disposition"`
		TurnID                 string                         `json:"turn_id"`
		TurnSequence           int                            `json:"turn_sequence"`
		StepID                 string                         `json:"step_id"`
		StepSequence           int                            `json:"step_sequence"`
		CheckpointDigest       string                         `json:"checkpoint_digest"`
		InputIDs               []string                       `json:"input_ids"`
		ModelRequestID         string                         `json:"model_request_id"`
	}{
		SchemaVersion: outcome.Binding.SchemaVersion,
		AttemptID:     outcome.Binding.AttemptID, TeamInstanceID: outcome.Binding.TeamInstanceID,
		RunID:                  outcome.Binding.PayloadAuthority.RunID,
		ClaimID:                outcome.Binding.PayloadAuthority.ClaimID,
		ClaimGeneration:        outcome.Binding.PayloadAuthority.ClaimGeneration,
		RuntimeInstanceID:      outcome.Binding.PayloadAuthority.RuntimeInstanceID,
		AgentInstanceID:        outcome.Binding.PayloadAuthority.AgentInstanceID,
		ExecutionBindingDigest: outcome.Binding.PayloadAuthority.ExecutionBindingDigest,
		ContextCapsuleDigest:   outcome.Binding.PayloadAuthority.CapsuleDigest,
		SegmentID:              outcome.SegmentID,
		Disposition:            outcome.Disposition, TurnID: outcome.TurnID,
		TurnSequence: outcome.TurnSequence, StepID: outcome.StepID,
		StepSequence: outcome.StepSequence, CheckpointDigest: outcome.CheckpointDigest,
		InputIDs: inputIDs, ModelRequestID: outcome.ModelRequestID,
	}
	return canonicalDomainSHA256("loom/agent-attempt-restart-candidate/v2", canonical)
}

func AgentAttemptRestartCapabilityDigest(capability AgentAttemptRestartCapability) string {
	canonical := struct {
		SchemaVersion          int                           `json:"schema_version"`
		AttemptID              string                        `json:"attempt_id"`
		RuntimeInstanceID      string                        `json:"runtime_instance_id"`
		ExecutionBindingDigest string                        `json:"execution_binding_digest"`
		ContextCapsuleDigest   string                        `json:"context_capsule_digest"`
		ResumeMode             AgentAttemptRestartResumeMode `json:"resume_mode"`
		SessionBindingDigest   string                        `json:"session_binding_digest"`
	}{
		SchemaVersion: capability.SchemaVersion, AttemptID: capability.AttemptID,
		RuntimeInstanceID:      capability.RuntimeInstanceID,
		ExecutionBindingDigest: capability.ExecutionBindingDigest,
		ContextCapsuleDigest:   capability.ContextCapsuleDigest,
		ResumeMode:             capability.ResumeMode,
		SessionBindingDigest:   capability.SessionBindingDigest,
	}
	return canonicalDomainSHA256("loom/agent-attempt-restart-capability/v1", canonical)
}

func validAgentAttemptRecoveryDecisionInput(input AgentAttemptRecoveryDecisionInput) bool {
	return input.SchemaVersion == 1 && validCanonicalUUID(input.DecisionID) &&
		validCanonicalUUID(input.CorrelationID) && validOpaqueID(input.PrincipalID) &&
		input.Action == AgentAttemptRecoveryResumePreModel &&
		validSHA256Hex(input.CandidateDigest) && validSHA256Hex(input.CapabilityDigest)
}

func validAgentAttemptRecoveryConsumeInput(input AgentAttemptRecoveryConsumeInput) bool {
	return input.SchemaVersion == 1 && validCanonicalUUID(input.CorrelationID) &&
		validCanonicalUUID(input.DecisionID) && validSHA256Hex(input.CandidateDigest) &&
		validSHA256Hex(input.CapabilityDigest)
}

func validAgentAttemptRestartCapability(capability AgentAttemptRestartCapability) bool {
	return capability.SchemaVersion == 1 && validOpaqueID(capability.AttemptID) &&
		validOpaqueID(capability.RuntimeInstanceID) &&
		validSHA256Hex(capability.ExecutionBindingDigest) &&
		validSHA256Hex(capability.ContextCapsuleDigest) &&
		capability.ResumeMode == AgentAttemptRestartResumeFromCheckpoint &&
		validSHA256Hex(capability.SessionBindingDigest) &&
		validSHA256Hex(capability.CapabilityDigest) &&
		capability.CapabilityDigest == AgentAttemptRestartCapabilityDigest(capability)
}

func validAgentAttemptRestartPreModelOutcome(outcome AgentAttemptRestartOutcome) bool {
	validated, err := loomruntime.ValidateFrozenExecutionBinding(outcome.ExecutionBinding)
	if err != nil || validated.BindingDigest != outcome.Binding.PayloadAuthority.ExecutionBindingDigest {
		return false
	}
	if outcome.Disposition != AgentAttemptRestartPreModelResume ||
		!validAttemptLoopBinding(outcome.Binding) || !validAttemptLoopBudget(outcome.Budget) ||
		!validOpaqueID(outcome.SegmentID) ||
		!validOpaqueID(outcome.TurnID) || outcome.TurnSequence <= 0 ||
		!validSHA256Hex(outcome.CheckpointDigest) || len(outcome.InputIDs) == 0 ||
		outcome.ModelRequestID != "" || outcome.ErrorCode != "" || outcome.Retryable {
		return false
	}
	if (outcome.StepID == "") != (outcome.StepSequence == 0) ||
		outcome.StepID != "" && !validOpaqueID(outcome.StepID) {
		return false
	}
	seen := make(map[string]struct{}, len(outcome.InputIDs))
	for _, inputID := range outcome.InputIDs {
		if !validOpaqueID(inputID) {
			return false
		}
		if _, exists := seen[inputID]; exists {
			return false
		}
		seen[inputID] = struct{}{}
	}
	return true
}

func agentAttemptRestartCapabilityMatches(
	capability AgentAttemptRestartCapability,
	outcome AgentAttemptRestartOutcome,
) bool {
	authority := outcome.Binding.PayloadAuthority
	return capability.AttemptID == outcome.Binding.AttemptID &&
		capability.RuntimeInstanceID == authority.RuntimeInstanceID &&
		capability.ExecutionBindingDigest == authority.ExecutionBindingDigest &&
		capability.ContextCapsuleDigest == authority.CapsuleDigest
}

func agentAttemptRestartOutcomeByDigest(
	report AgentAttemptRestartReport,
	digest string,
) (AgentAttemptRestartOutcome, bool) {
	for _, outcome := range report.Outcomes {
		if AgentAttemptRestartCandidateDigest(outcome) == digest {
			return outcome, true
		}
	}
	return AgentAttemptRestartOutcome{}, false
}

func agentAttemptRecoveryPayload(
	input AgentAttemptRecoveryDecisionInput,
	capability AgentAttemptRestartCapability,
	outcome AgentAttemptRestartOutcome,
) agentAttemptRecoveryEventPayload {
	authority := outcome.Binding.PayloadAuthority
	return agentAttemptRecoveryEventPayload{
		SchemaVersion: 1, DecisionID: input.DecisionID, PrincipalID: input.PrincipalID,
		Action: input.Action, CandidateDigest: input.CandidateDigest,
		CapabilityDigest: input.CapabilityDigest, AttemptID: outcome.Binding.AttemptID,
		RunID: authority.RunID, ClaimGeneration: authority.ClaimGeneration,
		RuntimeInstanceID:      authority.RuntimeInstanceID,
		AgentInstanceID:        authority.AgentInstanceID,
		ExecutionBindingDigest: authority.ExecutionBindingDigest,
		ContextCapsuleDigest:   authority.CapsuleDigest,
		SegmentID:              outcome.SegmentID,
		CheckpointDigest:       outcome.CheckpointDigest,
		TurnID:                 outcome.TurnID, TurnSequence: outcome.TurnSequence,
		StepID: outcome.StepID, StepSequence: outcome.StepSequence,
		RuntimeSessionDigest: capability.SessionBindingDigest,
		RuntimeResumeMode:    capability.ResumeMode,
	}
}

func agentAttemptRecoveryStream(binding AttemptLoopBinding) string {
	digest := sha256.Sum256([]byte(strings.Join([]string{
		"loom/agent-attempt-recovery/v1", binding.AttemptID,
		binding.PayloadAuthority.RunID,
		binding.PayloadAuthority.ExecutionBindingDigest,
		binding.PayloadAuthority.CapsuleDigest,
	}, "\x00")))
	return "agent-attempt-recovery/" + hex.EncodeToString(digest[:16])
}

func replayExactAgentAttemptRecoveryDecision(
	events []journal.Event,
	streamID string,
	input AgentAttemptRecoveryDecisionInput,
	capability AgentAttemptRestartCapability,
	outcome AgentAttemptRestartOutcome,
) (AgentAttemptRecoveryDecision, error) {
	var selected []journal.Event
	for _, event := range events {
		if event.StreamID == streamID {
			selected = append(selected, event)
		}
	}
	if len(selected) != 1 || selected[0].Seq != 1 ||
		selected[0].Type != "AgentAttemptRecoveryAuthorized" ||
		selected[0].SchemaVersion != 1 {
		return AgentAttemptRecoveryDecision{}, ErrAgentAttemptRecoveryConflict
	}
	var payload agentAttemptRecoveryEventPayload
	if decodeExactPayload(selected[0].PayloadJSON, &payload) != nil ||
		payload != agentAttemptRecoveryPayload(input, capability, outcome) ||
		selected[0].CorrelationID != input.CorrelationID {
		return AgentAttemptRecoveryDecision{}, ErrAgentAttemptRecoveryConflict
	}
	return agentAttemptRecoveryDecision(selected[0], payload), nil
}

func exactAgentAttemptRecoveryAuthorization(
	events []journal.Event,
	streamID string,
) (journal.Event, agentAttemptRecoveryEventPayload, error) {
	var selected []journal.Event
	for _, event := range events {
		if event.StreamID == streamID {
			selected = append(selected, event)
		}
	}
	if len(selected) > 1 {
		if len(selected) == 2 && selected[1].Seq == 2 &&
			selected[1].Type == "AgentAttemptRecoveryConsumed" {
			return journal.Event{}, agentAttemptRecoveryEventPayload{}, ErrAgentAttemptRecoveryConsumed
		}
		return journal.Event{}, agentAttemptRecoveryEventPayload{}, ErrAgentAttemptRecoveryConflict
	}
	if len(selected) != 1 || selected[0].Seq != 1 ||
		selected[0].Type != "AgentAttemptRecoveryAuthorized" ||
		selected[0].SchemaVersion != 1 {
		return journal.Event{}, agentAttemptRecoveryEventPayload{}, ErrAgentAttemptRecoveryConflict
	}
	var payload agentAttemptRecoveryEventPayload
	if decodeExactPayload(selected[0].PayloadJSON, &payload) != nil ||
		payload.SchemaVersion != 1 || !validCanonicalUUID(payload.DecisionID) ||
		payload.Action != AgentAttemptRecoveryResumePreModel ||
		!validSHA256Hex(payload.CandidateDigest) ||
		!validSHA256Hex(payload.CapabilityDigest) {
		return journal.Event{}, agentAttemptRecoveryEventPayload{}, ErrAgentAttemptRecoveryConflict
	}
	return selected[0], payload, nil
}

func agentAttemptRecoveryCandidateState(
	events []journal.Event,
	outcome AgentAttemptRestartOutcome,
	capability AgentAttemptRestartCapability,
) (AgentAttemptRecoveryCandidateStatus, string, error) {
	streamID := agentAttemptRecoveryStream(outcome.Binding)
	selected := make([]journal.Event, 0, 2)
	for _, event := range events {
		if event.StreamID == streamID {
			selected = append(selected, event)
		}
	}
	if len(selected) == 0 {
		return AgentAttemptRecoveryCandidateAvailable, "", nil
	}
	if len(selected) > 2 || selected[0].Seq != 1 ||
		selected[0].Type != "AgentAttemptRecoveryAuthorized" ||
		selected[0].SchemaVersion != 1 {
		return "", "", ErrAgentAttemptRecoveryConflict
	}
	var authorized agentAttemptRecoveryEventPayload
	if decodeExactPayload(selected[0].PayloadJSON, &authorized) != nil ||
		!agentAttemptRecoveryPayloadMatchesCandidate(authorized, outcome, capability) {
		return "", "", ErrAgentAttemptRecoveryConflict
	}
	if len(selected) == 1 {
		return AgentAttemptRecoveryCandidateAuthorized, authorized.DecisionID, nil
	}
	consumedEvent := selected[1]
	var consumed agentAttemptRecoveryConsumedPayload
	if consumedEvent.Seq != 2 || consumedEvent.Type != "AgentAttemptRecoveryConsumed" ||
		consumedEvent.SchemaVersion != 1 || consumedEvent.CausationID != selected[0].ID ||
		decodeExactPayload(consumedEvent.PayloadJSON, &consumed) != nil ||
		consumed.SchemaVersion != 1 || !validOpaqueID(consumed.LeaseID) ||
		consumed.DecisionID != authorized.DecisionID ||
		consumed.CandidateDigest != authorized.CandidateDigest ||
		consumed.CapabilityDigest != authorized.CapabilityDigest ||
		consumed.AttemptID != authorized.AttemptID ||
		consumed.RunID != authorized.RunID ||
		consumed.ClaimGeneration != authorized.ClaimGeneration ||
		consumed.RuntimeInstanceID != authorized.RuntimeInstanceID ||
		consumed.AgentInstanceID != authorized.AgentInstanceID {
		return "", "", ErrAgentAttemptRecoveryConflict
	}
	return AgentAttemptRecoveryCandidateConsumed, authorized.DecisionID, nil
}

func agentAttemptRecoveryPayloadMatchesCandidate(
	payload agentAttemptRecoveryEventPayload,
	outcome AgentAttemptRestartOutcome,
	capability AgentAttemptRestartCapability,
) bool {
	authority := outcome.Binding.PayloadAuthority
	return payload.SchemaVersion == 1 && validCanonicalUUID(payload.DecisionID) &&
		validOpaqueID(payload.PrincipalID) &&
		payload.Action == AgentAttemptRecoveryResumePreModel &&
		payload.CandidateDigest == AgentAttemptRestartCandidateDigest(outcome) &&
		payload.CapabilityDigest == capability.CapabilityDigest &&
		payload.AttemptID == outcome.Binding.AttemptID &&
		payload.RunID == authority.RunID &&
		payload.ClaimGeneration == authority.ClaimGeneration &&
		payload.RuntimeInstanceID == authority.RuntimeInstanceID &&
		payload.AgentInstanceID == authority.AgentInstanceID &&
		payload.ExecutionBindingDigest == authority.ExecutionBindingDigest &&
		payload.ContextCapsuleDigest == authority.CapsuleDigest &&
		payload.SegmentID == outcome.SegmentID &&
		payload.CheckpointDigest == outcome.CheckpointDigest &&
		payload.TurnID == outcome.TurnID && payload.TurnSequence == outcome.TurnSequence &&
		payload.StepID == outcome.StepID && payload.StepSequence == outcome.StepSequence &&
		payload.RuntimeSessionDigest == capability.SessionBindingDigest &&
		payload.RuntimeResumeMode == capability.ResumeMode
}

func agentAttemptRecoveryDecision(
	event journal.Event,
	payload agentAttemptRecoveryEventPayload,
) AgentAttemptRecoveryDecision {
	return AgentAttemptRecoveryDecision{
		SchemaVersion: payload.SchemaVersion, DecisionID: payload.DecisionID,
		EventID: event.ID, StreamID: event.StreamID, Action: payload.Action,
		CandidateDigest:  payload.CandidateDigest,
		CapabilityDigest: payload.CapabilityDigest,
	}
}

func canonicalDomainSHA256(domain string, value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	digest := sha256.Sum256(append(append([]byte(domain), 0), encoded...))
	return hex.EncodeToString(digest[:])
}

func (authority *AgentAttemptRecoveryAuthority) newRecoveryLeaseID() (string, error) {
	authority.randomMu.Lock()
	defer authority.randomMu.Unlock()
	value := make([]byte, 16)
	if _, err := io.ReadFull(authority.random, value); err != nil {
		return "", err
	}
	return "recovery-lease-" + hex.EncodeToString(value), nil
}

func cloneAgentAttemptRestartOutcome(outcome AgentAttemptRestartOutcome) AgentAttemptRestartOutcome {
	clone := outcome
	clone.InputIDs = append([]string(nil), outcome.InputIDs...)
	validated, err := loomruntime.ValidateFrozenExecutionBinding(outcome.ExecutionBinding)
	if err == nil {
		clone.ExecutionBinding = validated
	}
	return clone
}
