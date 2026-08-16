package work

import (
	"strconv"
	"strings"
	"time"

	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/journal"
)

type providerAccountCapacityBinding struct {
	capacityBinding
	providerID             string
	providerAccountID      string
	policyRevision         int64
	policyDigest           string
	executionBindingDigest string
	assignedBudgetUnits    int64
}

type providerAccountDispatchStart struct {
	reservedAt time.Time
	binding    providerAccountCapacityBinding
}

type providerAccountCapacity struct {
	active map[string]providerAccountCapacityBinding
	starts []providerAccountDispatchStart
}

type providerAccountCapacityPayload struct {
	WorkItemID             string `json:"work_item_id"`
	RunID                  string `json:"run_id"`
	ClaimID                string `json:"claim_id"`
	ClaimGeneration        int64  `json:"claim_generation"`
	RuntimeInstanceID      string `json:"runtime_instance_id"`
	AgentInstanceID        string `json:"agent_instance_id"`
	ProviderID             string `json:"provider_id"`
	ProviderAccountID      string `json:"provider_account_id"`
	PolicyRevision         int64  `json:"policy_revision"`
	PolicyDigest           string `json:"policy_digest"`
	ExecutionBindingDigest string `json:"execution_binding_digest"`
	AssignedBudgetUnits    int64  `json:"assigned_budget_units"`
}

func (record RunRecord) ProviderAccountPolicy() (int64, string, bool) {
	return record.providerAccountPolicyRevision,
		record.providerAccountPolicyDigest,
		record.providerAccountPolicyRevision > 0 &&
			validSHA256Hex(record.providerAccountPolicyDigest)
}

func (record RunRecord) ProviderAccountDisclosurePolicy() (
	int,
	string,
	string,
	string,
	bool,
) {
	governed := record.providerAccountPolicyRevision > 0 &&
		validSHA256Hex(record.providerAccountPolicyDigest) &&
		validProviderAccountPolicyVersionDisclosure(
			record.providerAccountPolicyVersion,
			record.providerAccountTrustDomain,
			record.providerAccountRetentionMode,
			record.providerAccountDataRegion,
		)
	return record.providerAccountPolicyVersion,
		record.providerAccountTrustDomain,
		record.providerAccountRetentionMode,
		record.providerAccountDataRegion,
		governed
}

func freezeRunProviderAccountPolicy(
	run *RunRecord,
	policy ProviderAccountPolicy,
) {
	run.providerAccountPolicyRevision = policy.Revision()
	run.providerAccountPolicyDigest = policy.Digest()
	run.providerAccountPolicyVersion = policy.Version()
	run.providerAccountTrustDomain = policy.TrustDomain()
	run.providerAccountRetentionMode = policy.RetentionMode()
	run.providerAccountDataRegion = policy.DataRegion()
}

func providerAccountCapacityStream(providerAccountID string) string {
	return "provider-account-capacity/" + providerAccountID
}

func providerAccountCapacityBindingFor(
	run RunRecord,
	claimID string,
	claimGeneration int64,
	runtimeInstanceID string,
	policy ProviderAccountPolicy,
) (providerAccountCapacityBinding, error) {
	binding := run.executionBinding
	if !policy.Valid() || binding.BindingDigest == "" ||
		binding.ProviderID != policy.ProviderID() ||
		binding.ProviderAccountID != policy.ProviderAccountID() ||
		binding.RuntimeInstanceID != runtimeInstanceID {
		return providerAccountCapacityBinding{}, ErrRunAuthorityConflict
	}
	budget := int64(0)
	if binding.Budget != nil {
		budget = *binding.Budget
	}
	if budget < 0 {
		return providerAccountCapacityBinding{}, ErrRunAuthorityConflict
	}
	return providerAccountCapacityBinding{
		capacityBinding: capacityBinding{
			workItemID: run.workItemID, runID: run.id,
			claimID: claimID, claimGeneration: claimGeneration,
			runtimeInstanceID: runtimeInstanceID,
			agentInstanceID:   run.agentInstanceID,
		},
		providerID: policy.ProviderID(), providerAccountID: policy.ProviderAccountID(),
		policyRevision: policy.Revision(), policyDigest: policy.Digest(),
		executionBindingDigest: binding.BindingDigest,
		assignedBudgetUnits:    budget,
	}, nil
}

func providerAccountActiveBindingForRun(
	state authorityState,
	run RunRecord,
) (providerAccountCapacityBinding, bool, error) {
	if run.providerAccountPolicyRevision == 0 {
		return providerAccountCapacityBinding{}, false, nil
	}
	policy, found := state.providerAccountPolicyRevision(
		run.executionBinding.ProviderID,
		run.executionBinding.ProviderAccountID,
		run.providerAccountPolicyRevision,
		run.providerAccountPolicyDigest,
	)
	if !found {
		return providerAccountCapacityBinding{}, false, ErrRunAuthorityConflict
	}
	binding, err := providerAccountCapacityBindingFor(
		run, run.claimID, run.claimGeneration, run.runtimeInstanceID, policy,
	)
	if err != nil || binding.assignedBudgetUnits != run.providerAccountBudgetUnits {
		return providerAccountCapacityBinding{}, false, ErrRunAuthorityConflict
	}
	active := state.providerAccountCapacity[binding.providerAccountID].active
	if active[capacityKey(binding.runID, binding.claimGeneration)] != binding {
		return providerAccountCapacityBinding{}, false, ErrRunAuthorityConflict
	}
	return binding, true, nil
}

func (binding providerAccountCapacityBinding) payload() providerAccountCapacityPayload {
	return providerAccountCapacityPayload{
		WorkItemID: binding.workItemID, RunID: binding.runID,
		ClaimID: binding.claimID, ClaimGeneration: binding.claimGeneration,
		RuntimeInstanceID: binding.runtimeInstanceID,
		AgentInstanceID:   binding.agentInstanceID,
		ProviderID:        binding.providerID, ProviderAccountID: binding.providerAccountID,
		PolicyRevision: binding.policyRevision, PolicyDigest: binding.policyDigest,
		ExecutionBindingDigest: binding.executionBindingDigest,
		AssignedBudgetUnits:    binding.assignedBudgetUnits,
	}
}

func (payload providerAccountCapacityPayload) binding() providerAccountCapacityBinding {
	return providerAccountCapacityBinding{
		capacityBinding: capacityBinding{
			workItemID: payload.WorkItemID, runID: payload.RunID,
			claimID: payload.ClaimID, claimGeneration: payload.ClaimGeneration,
			runtimeInstanceID: payload.RuntimeInstanceID,
			agentInstanceID:   payload.AgentInstanceID,
		},
		providerID: payload.ProviderID, providerAccountID: payload.ProviderAccountID,
		policyRevision: payload.PolicyRevision, policyDigest: payload.PolicyDigest,
		executionBindingDigest: payload.ExecutionBindingDigest,
		assignedBudgetUnits:    payload.AssignedBudgetUnits,
	}
}

func validProviderAccountCapacityEventIdentity(
	event journal.Event,
	binding providerAccountCapacityBinding,
) bool {
	if !validCanonicalUUID(event.CorrelationID) || event.CausationID == "" {
		return false
	}
	expectedID := deterministicEventID(
		event.Type,
		binding.runID,
		binding.claimID,
		strconv.FormatInt(binding.claimGeneration, 10),
		event.CausationID,
	)
	return event.ID == expectedID && event.IdempotencyKey == expectedID
}

func replayProviderAccountPolicyStreams(
	state *authorityState,
	byStream map[string][]journal.Event,
) error {
	for streamID, events := range byStream {
		if !strings.HasPrefix(streamID, "provider-account-policy/") {
			continue
		}
		accountID := strings.TrimPrefix(streamID, "provider-account-policy/")
		var previousEventID string
		for _, event := range events {
			policy, commandID, err := DecodeProviderAccountPolicyConfiguredEvent(
				event, previousEventID,
			)
			if err != nil || policy.ProviderAccountID() != accountID {
				return ErrRunAuthorityConflict
			}
			history := state.providerAccountPolicies[accountID]
			if len(history) > 0 &&
				!policy.ConfiguredAt().After(history[len(history)-1].policy.ConfiguredAt()) {
				return ErrRunAuthorityConflict
			}
			state.providerAccountPolicies[accountID] = append(
				history,
				providerAccountPolicyState{
					policy: policy, commandID: commandID, eventID: event.ID,
				},
			)
			previousEventID = event.ID
		}
	}
	return nil
}

func (state authorityState) providerAccountPolicyAt(
	providerID string,
	providerAccountID string,
	at time.Time,
) (ProviderAccountPolicy, bool) {
	history := state.providerAccountPolicies[providerAccountID]
	for index := len(history) - 1; index >= 0; index-- {
		policy := history[index].policy
		if policy.ProviderID() == providerID && !policy.ConfiguredAt().After(at) {
			return policy, true
		}
	}
	return ProviderAccountPolicy{}, false
}

func (state authorityState) providerAccountPolicyRevision(
	providerID string,
	providerAccountID string,
	revision int64,
	digest string,
) (ProviderAccountPolicy, bool) {
	for _, candidate := range state.providerAccountPolicies[providerAccountID] {
		policy := candidate.policy
		if policy.ProviderID() == providerID && policy.Revision() == revision &&
			policy.Digest() == digest {
			return policy, true
		}
	}
	return ProviderAccountPolicy{}, false
}

func validateProviderAccountCapacityAdmission(
	capacity providerAccountCapacity,
	policy ProviderAccountPolicy,
	binding providerAccountCapacityBinding,
	now time.Time,
	replacing *providerAccountCapacityBinding,
) error {
	activeCount := len(capacity.active)
	assignedBudget := int64(0)
	for _, active := range capacity.active {
		assignedBudget += active.assignedBudgetUnits
	}
	if replacing != nil {
		key := capacityKey(replacing.runID, replacing.claimGeneration)
		active, ok := capacity.active[key]
		if !ok || active != *replacing {
			return ErrRunAuthorityConflict
		}
		activeCount--
		assignedBudget -= active.assignedBudgetUnits
	}
	if activeCount >= policy.MaximumConcurrentAttempts() {
		return ErrProviderAccountConcurrencyExhausted
	}
	windowStart := now.Add(-policy.DispatchWindow())
	dispatchStarts := 0
	for _, start := range capacity.starts {
		if !start.reservedAt.Before(windowStart) && !start.reservedAt.After(now) {
			dispatchStarts++
		}
	}
	if dispatchStarts >= policy.MaximumDispatchStarts() {
		return ErrProviderAccountDispatchRateExhausted
	}
	if binding.assignedBudgetUnits > policy.MaximumAssignedBudgetUnits()-assignedBudget {
		return ErrProviderAccountBudgetExhausted
	}
	return nil
}

func replayProviderAccountCapacityStream(
	state *authorityState,
	streamID string,
	events []journal.Event,
	claims map[string]*claimReplay,
	terminals map[string]*terminalReplay,
	allowOrphanCapacity bool,
) error {
	accountID := strings.TrimPrefix(streamID, "provider-account-capacity/")
	capacity := state.providerAccountCapacity[accountID]
	if capacity.active == nil {
		capacity.active = make(map[string]providerAccountCapacityBinding)
	}
	for _, event := range events {
		if event.Type != "ProviderAccountCapacityReserved" &&
			event.Type != "ProviderAccountCapacityReleased" {
			return ErrRunAuthorityConflict
		}
		var payload providerAccountCapacityPayload
		if decodeExactPayload(event.PayloadJSON, &payload) != nil ||
			payload.ProviderAccountID != accountID ||
			!credentials.ValidProviderAccountIdentifier(
				payload.ProviderID, payload.ProviderAccountID,
			) || payload.PolicyRevision <= 0 ||
			!validSHA256Hex(payload.PolicyDigest) ||
			!validSHA256Hex(payload.ExecutionBindingDigest) ||
			payload.AssignedBudgetUnits < 0 {
			return ErrRunAuthorityConflict
		}
		binding := payload.binding()
		if !validProviderAccountCapacityEventIdentity(event, binding) {
			return ErrRunAuthorityConflict
		}
		policy, found := state.providerAccountPolicyRevision(
			binding.providerID, binding.providerAccountID,
			binding.policyRevision, binding.policyDigest,
		)
		if !found {
			return ErrRunAuthorityConflict
		}
		key := capacityKey(binding.runID, binding.claimGeneration)
		switch event.Type {
		case "ProviderAccountCapacityReserved":
			current, currentFound := state.providerAccountPolicyAt(
				binding.providerID, binding.providerAccountID, event.EmittedAt,
			)
			if !currentFound || current.Digest() != policy.Digest() ||
				capacity.active[key].runID != "" ||
				validateProviderAccountCapacityAdmission(
					capacity, policy, binding, event.EmittedAt, nil,
				) != nil {
				return ErrRunAuthorityConflict
			}
			claim, ok := claims[event.CausationID]
			if ok && (!claim.accountManaged || claim.accountBinding != binding) {
				return ErrRunAuthorityConflict
			}
			if !ok && !allowOrphanCapacity {
				return ErrRunAuthorityConflict
			}
			capacity.active[key] = binding
			capacity.starts = append(capacity.starts, providerAccountDispatchStart{
				reservedAt: event.EmittedAt, binding: binding,
			})
			if ok {
				claim.accountReserved = true
				run := state.runs[binding.runID]
				freezeRunProviderAccountPolicy(&run, policy)
				run.providerAccountBudgetUnits = binding.assignedBudgetUnits
				state.runs[run.id] = run
			}
		case "ProviderAccountCapacityReleased":
			active, exists := capacity.active[key]
			if !exists || active != binding {
				return ErrRunAuthorityConflict
			}
			delete(capacity.active, key)
			if terminal, ok := terminals[event.CausationID]; ok {
				if !terminal.accountManaged || terminal.accountBinding != binding {
					return ErrRunAuthorityConflict
				}
				terminal.accountReleased = true
			} else if claim, ok := claims[event.CausationID]; ok &&
				claim.needsOldAccountRelease {
				if claim.previousAccountBinding != binding {
					return ErrRunAuthorityConflict
				}
				claim.accountReleased = true
			} else if !allowOrphanCapacity {
				return ErrRunAuthorityConflict
			}
		}
	}
	state.providerAccountCapacity[accountID] = capacity
	return nil
}
