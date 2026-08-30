package main

import (
	"errors"
	"sync"

	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/work"
)

var (
	errProductInvalidActiveAttempt  = errors.New("invalid active Agent Attempt")
	errProductActiveAttemptConflict = errors.New("active Agent Attempt conflict")
	errProductActiveAttemptNotFound = errors.New("active Agent Attempt not found")
)

// productActiveAttemptQuery is the complete non-secret identity a future
// product ingress must present. It deliberately excludes internal bindings.
type productActiveAttemptQuery struct {
	ConversationID  string
	SegmentID       string
	AgentInstanceID string
	WorkItemID      string
	RunID           string
	ClaimGeneration int64
}

type productActiveAttemptInputQuery struct {
	SegmentID       string
	AgentInstanceID string
	WorkItemID      string
	RunID           string
	ClaimGeneration int64
}

type productActiveAttempt struct {
	Identity           productActiveAttemptQuery
	AttemptLoopBinding work.AttemptLoopBinding
	Budget             work.AttemptLoopBudget
	ExecutionBinding   loomruntime.FrozenExecutionBinding
	CapsuleDigest      string
	IncidentID         string
	AcceptsAgentInputs bool
}

// productActiveAttemptRegistry is a revocable runtime projection, not an
// execution authority. Entries can only be created from an already validated
// AdapterRequest and its exact Attempt Loop binding, or from a consumed and
// validated recovery dispatch grant after Runtime reattachment succeeds.
type productActiveAttemptRegistry struct {
	mu        sync.RWMutex
	nextToken uint64
	attempts  map[productActiveAttemptQuery]productActiveAttemptEntry
}

type productActiveAttemptEntry struct {
	token   uint64
	attempt productActiveAttempt
}

type productActiveAttemptRegistration struct {
	registry *productActiveAttemptRegistry
	identity productActiveAttemptQuery
	token    uint64
	once     sync.Once
}

func newProductActiveAttemptRegistry() *productActiveAttemptRegistry {
	return &productActiveAttemptRegistry{
		attempts: make(map[productActiveAttemptQuery]productActiveAttemptEntry),
	}
}

func (registry *productActiveAttemptRegistry) Register(
	request supervisor.AdapterRequest,
	binding work.AttemptLoopBinding,
	budget work.AttemptLoopBudget,
) (*productActiveAttemptRegistration, productActiveAttempt, error) {
	if registry == nil {
		return nil, productActiveAttempt{}, errProductInvalidActiveAttempt
	}
	expectedBinding, expectedBudget, err := productAttemptLoopBinding(request)
	if request.AgentInputs != nil {
		expectedBudget.MaxTurns = productAttemptLoopMaxInputTurns
		expectedBudget.MaxStepsPerTurn = productAttemptLoopMaxInputSteps
	}
	if err != nil || binding != expectedBinding || budget != expectedBudget {
		return nil, productActiveAttempt{}, errProductInvalidActiveAttempt
	}
	executionBinding, err := loomruntime.ValidateFrozenExecutionBinding(
		request.ExecutionBinding,
	)
	if err != nil {
		return nil, productActiveAttempt{}, errProductInvalidActiveAttempt
	}
	identity := productActiveAttemptQuery{
		ConversationID:  request.ContextCapsule.ConversationID,
		SegmentID:       request.RouteSegment.SegmentID,
		AgentInstanceID: request.Binding.SenderAgentInstanceID,
		WorkItemID:      request.Binding.WorkItemID,
		RunID:           request.Binding.RunID,
		ClaimGeneration: request.Binding.ClaimGeneration,
	}
	if !validProductActiveAttemptQuery(identity) || request.IncidentID == "" {
		return nil, productActiveAttempt{}, errProductInvalidActiveAttempt
	}
	attempt := productActiveAttempt{
		Identity: identity, AttemptLoopBinding: binding, Budget: budget,
		ExecutionBinding:   executionBinding,
		CapsuleDigest:      request.ContextCapsule.CapsuleDigest,
		IncidentID:         request.IncidentID,
		AcceptsAgentInputs: request.AgentInputs != nil,
	}
	return registry.register(attempt)
}

func (registry *productActiveAttemptRegistry) RegisterRecovered(
	grant work.AgentAttemptRecoveryDispatchGrant,
) (*productActiveAttemptRegistration, productActiveAttempt, error) {
	if registry == nil {
		return nil, productActiveAttempt{}, errProductInvalidActiveAttempt
	}
	outcome, _, err := work.ValidateAgentAttemptRecoveryDispatchGrant(grant)
	if err != nil {
		return nil, productActiveAttempt{}, errProductInvalidActiveAttempt
	}
	authority := outcome.Binding.PayloadAuthority
	executionBinding, err := loomruntime.ValidateFrozenExecutionBinding(outcome.ExecutionBinding)
	if err != nil || executionBinding.BindingDigest != authority.ExecutionBindingDigest {
		return nil, productActiveAttempt{}, errProductInvalidActiveAttempt
	}
	attempt := productActiveAttempt{
		Identity: productActiveAttemptQuery{
			ConversationID: authority.ConversationID, SegmentID: outcome.SegmentID,
			AgentInstanceID: authority.AgentInstanceID, WorkItemID: authority.WorkItemID,
			RunID: authority.RunID, ClaimGeneration: authority.ClaimGeneration,
		},
		AttemptLoopBinding: outcome.Binding,
		Budget:             outcome.Budget,
		ExecutionBinding:   executionBinding,
		CapsuleDigest:      authority.CapsuleDigest,
		IncidentID:         grant.IncidentID(),
		AcceptsAgentInputs: outcome.Budget.MaxTurns > 1 || outcome.Budget.MaxStepsPerTurn > 1,
	}
	if !validProductActiveAttemptQuery(attempt.Identity) || attempt.IncidentID == "" {
		return nil, productActiveAttempt{}, errProductInvalidActiveAttempt
	}
	return registry.register(attempt)
}

func (registry *productActiveAttemptRegistry) register(
	attempt productActiveAttempt,
) (*productActiveAttemptRegistration, productActiveAttempt, error) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, exists := registry.attempts[attempt.Identity]; exists {
		return nil, productActiveAttempt{}, errProductActiveAttemptConflict
	}
	registry.nextToken++
	entry := productActiveAttemptEntry{token: registry.nextToken, attempt: attempt}
	registry.attempts[attempt.Identity] = entry
	registration := &productActiveAttemptRegistration{
		registry: registry, identity: attempt.Identity, token: entry.token,
	}
	return registration, cloneProductActiveAttempt(attempt), nil
}

func (registry *productActiveAttemptRegistry) Resolve(
	query productActiveAttemptQuery,
) (productActiveAttempt, error) {
	if registry == nil || !validProductActiveAttemptQuery(query) {
		return productActiveAttempt{}, errProductInvalidActiveAttempt
	}
	registry.mu.RLock()
	entry, found := registry.attempts[query]
	registry.mu.RUnlock()
	if !found {
		return productActiveAttempt{}, errProductActiveAttemptNotFound
	}
	return cloneProductActiveAttempt(entry.attempt), nil
}

func (registry *productActiveAttemptRegistry) ResolveAgentInput(
	query productActiveAttemptInputQuery,
) (productActiveAttempt, error) {
	if registry == nil || query.SegmentID == "" || query.AgentInstanceID == "" ||
		query.WorkItemID == "" || query.RunID == "" || query.ClaimGeneration <= 0 {
		return productActiveAttempt{}, errProductInvalidActiveAttempt
	}
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	var resolved productActiveAttempt
	found := false
	for identity, entry := range registry.attempts {
		if identity.SegmentID != query.SegmentID ||
			identity.AgentInstanceID != query.AgentInstanceID ||
			identity.WorkItemID != query.WorkItemID || identity.RunID != query.RunID ||
			identity.ClaimGeneration != query.ClaimGeneration {
			continue
		}
		if found {
			return productActiveAttempt{}, errProductActiveAttemptConflict
		}
		resolved, found = entry.attempt, true
	}
	if !found {
		return productActiveAttempt{}, errProductActiveAttemptNotFound
	}
	return cloneProductActiveAttempt(resolved), nil
}

func (registration *productActiveAttemptRegistration) Close() {
	if registration == nil {
		return
	}
	registration.once.Do(func() {
		registry := registration.registry
		if registry == nil {
			return
		}
		registry.mu.Lock()
		defer registry.mu.Unlock()
		entry, found := registry.attempts[registration.identity]
		if found && entry.token == registration.token {
			delete(registry.attempts, registration.identity)
		}
	})
}

func validProductActiveAttemptQuery(query productActiveAttemptQuery) bool {
	return query.ConversationID != "" && query.SegmentID != "" &&
		query.AgentInstanceID != "" && query.WorkItemID != "" &&
		query.RunID != "" && query.ClaimGeneration > 0
}

func cloneProductActiveAttempt(input productActiveAttempt) productActiveAttempt {
	clone := input
	validated, err := loomruntime.ValidateFrozenExecutionBinding(input.ExecutionBinding)
	if err == nil {
		clone.ExecutionBinding = validated
	}
	return clone
}
