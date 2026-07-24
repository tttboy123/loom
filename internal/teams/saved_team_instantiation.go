package teams

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"sort"

	"loom-pi-rebuild/internal/agents"
	"loom-pi-rebuild/internal/mode"
	loomruntime "loom-pi-rebuild/internal/runtime"
)

var (
	ErrInvalidSavedTeamInstantiationPlan        = errors.New("invalid saved team instantiation plan")
	ErrSavedTeamInstantiationRequiresLoadTeam   = errors.New("saved team instantiation requires load team")
	ErrSavedTeamInstantiationSourceMismatch     = errors.New("saved team instantiation source mismatch")
	ErrInvalidSavedTeamDormantState             = errors.New("invalid saved team dormant state")
	ErrSavedTeamInstantiationPlanDigestMismatch = errors.New("saved team instantiation plan digest mismatch")
	ErrSavedTeamInstantiationPlanSourceMismatch = errors.New("saved team instantiation plan source mismatch")
)

type SavedTeamMainInstanceSeed struct {
	AgentDefinitionID string
	RuntimeProfileID  string
	RuntimeInstanceID string
	Binding           loomruntime.BindingCandidate
}

type SavedTeamDormantSubAgentSeed struct {
	Dormant           bool
	AgentDefinitionID string
	RuntimeProfileID  string
	RuntimeInstanceID string
}

type SavedTeamInstantiationPlanCandidate struct {
	ready                   bool
	trigger                 mode.Trigger
	targetID                string
	resolutionDigest        string
	teamDefinitionID        string
	teamDefinitionVersion   int
	teamDefinitionScope     TeamDefinitionScope
	scopeIdentity           agents.ScopeIdentity
	teamDefinitionDigest    string
	runtimeDiscoveryDigest  string
	bindingDigest           string
	mainSeed                SavedTeamMainInstanceSeed
	dormantSubAgents        []SavedTeamDormantSubAgentSeed
	createTeamInstance      bool
	createMainAgentInstance bool
	createSubAgentInstances bool
	requiresDraft           bool
	workItemCount           int
	planDigest              string
}

type SavedTeamInstantiationPlanValidationCandidate struct {
	Valid                 bool
	TeamDefinitionID      string
	TeamDefinitionDigest  string
	ResolutionDigest      string
	BindingDigest         string
	PlanDigest            string
	MainAgentDefinitionID string
	DormantSubAgentCount  int
	WorkItemCount         int
}

func BuildSavedTeamInstantiationPlan(
	intent mode.Intent,
	context agents.ScopeIdentity,
	catalog TeamResolutionCatalogInput,
	binding SavedTeamRuntimeBindingCandidate,
	discovery loomruntime.RuntimeDiscoverySnapshot,
	selections []SavedTeamRuntimeSelection,
) (SavedTeamInstantiationPlanCandidate, error) {
	resolution, err := ResolveAgentModeTeam(intent, context, catalog)
	if err != nil {
		return SavedTeamInstantiationPlanCandidate{}, err
	}
	if !resolution.Resolved || resolution.Kind != TeamResolutionLoadTeam ||
		resolution.RequiresDraft || !resolution.Team.Found {
		return SavedTeamInstantiationPlanCandidate{}, ErrSavedTeamInstantiationRequiresLoadTeam
	}

	if !savedTeamResolutionMatchesBinding(resolution.Team, binding) {
		return SavedTeamInstantiationPlanCandidate{}, ErrSavedTeamInstantiationSourceMismatch
	}
	if _, err := ValidateSavedTeamRuntimeBinding(
		binding,
		catalog.TeamDefinitions,
		binding.TeamDefinitionID(),
		resolution.Team.ScopeIdentity,
		catalog.AgentDefinitions,
		catalog.RuntimeProfiles,
		discovery,
		selections,
	); err != nil {
		return SavedTeamInstantiationPlanCandidate{}, err
	}

	mainBinding := binding.MainBinding()
	subBindings := binding.SubAgentBindings()
	if binding.RoleCount() != 1+len(subBindings) || len(subBindings) > 2 ||
		mainBinding.Kind != TeamDefinitionRoleMain {
		return SavedTeamInstantiationPlanCandidate{}, ErrInvalidSavedTeamInstantiationPlan
	}
	mainSeed := SavedTeamMainInstanceSeed{
		AgentDefinitionID: mainBinding.AgentDefinitionID,
		RuntimeProfileID:  mainBinding.RuntimeProfileID,
		RuntimeInstanceID: mainBinding.RuntimeInstanceID,
		Binding:           mainBinding.Binding,
	}
	dormant := make([]SavedTeamDormantSubAgentSeed, len(subBindings))
	for index, subBinding := range subBindings {
		if subBinding.Kind != TeamDefinitionRoleSubAgent {
			return SavedTeamInstantiationPlanCandidate{}, ErrInvalidSavedTeamDormantState
		}
		dormant[index] = SavedTeamDormantSubAgentSeed{
			Dormant:           true,
			AgentDefinitionID: subBinding.AgentDefinitionID,
			RuntimeProfileID:  subBinding.RuntimeProfileID,
			RuntimeInstanceID: subBinding.RuntimeInstanceID,
		}
	}
	sortSavedTeamDormantSubAgents(dormant)

	candidate := SavedTeamInstantiationPlanCandidate{
		ready:                   true,
		trigger:                 resolution.Trigger,
		targetID:                resolution.TargetID,
		resolutionDigest:        resolution.Digest,
		teamDefinitionID:        resolution.Team.ID,
		teamDefinitionVersion:   resolution.Team.Version,
		teamDefinitionScope:     resolution.Team.Scope,
		scopeIdentity:           resolution.Team.ScopeIdentity,
		teamDefinitionDigest:    resolution.Team.DefinitionDigest,
		runtimeDiscoveryDigest:  binding.RuntimeDiscoveryDigest(),
		bindingDigest:           binding.BindingDigest(),
		mainSeed:                mainSeed,
		dormantSubAgents:        append([]SavedTeamDormantSubAgentSeed(nil), dormant...),
		createTeamInstance:      true,
		createMainAgentInstance: true,
		createSubAgentInstances: false,
		requiresDraft:           false,
		workItemCount:           0,
	}
	candidate.planDigest, err = digestSavedTeamInstantiationPlan(candidate)
	if err != nil {
		return SavedTeamInstantiationPlanCandidate{}, err
	}
	return candidate, nil
}

func ValidateSavedTeamInstantiationPlan(
	current SavedTeamInstantiationPlanCandidate,
	intent mode.Intent,
	context agents.ScopeIdentity,
	catalog TeamResolutionCatalogInput,
	binding SavedTeamRuntimeBindingCandidate,
	discovery loomruntime.RuntimeDiscoverySnapshot,
	selections []SavedTeamRuntimeSelection,
) (SavedTeamInstantiationPlanValidationCandidate, error) {
	if !validSavedTeamInstantiationPlanShape(current) {
		return SavedTeamInstantiationPlanValidationCandidate{}, ErrInvalidSavedTeamInstantiationPlan
	}
	if !validSavedTeamDormantSubAgents(current.dormantSubAgents) {
		return SavedTeamInstantiationPlanValidationCandidate{}, ErrInvalidSavedTeamDormantState
	}
	digest, err := digestSavedTeamInstantiationPlan(current)
	if err != nil {
		return SavedTeamInstantiationPlanValidationCandidate{}, err
	}
	if digest != current.planDigest {
		return SavedTeamInstantiationPlanValidationCandidate{}, ErrSavedTeamInstantiationPlanDigestMismatch
	}
	expected, err := BuildSavedTeamInstantiationPlan(
		intent, context, catalog, binding, discovery, selections,
	)
	if err != nil {
		return SavedTeamInstantiationPlanValidationCandidate{}, err
	}
	if !reflect.DeepEqual(current, expected) {
		return SavedTeamInstantiationPlanValidationCandidate{}, ErrSavedTeamInstantiationPlanSourceMismatch
	}
	return SavedTeamInstantiationPlanValidationCandidate{
		Valid:                 true,
		TeamDefinitionID:      current.teamDefinitionID,
		TeamDefinitionDigest:  current.teamDefinitionDigest,
		ResolutionDigest:      current.resolutionDigest,
		BindingDigest:         current.bindingDigest,
		PlanDigest:            current.planDigest,
		MainAgentDefinitionID: current.mainSeed.AgentDefinitionID,
		DormantSubAgentCount:  len(current.dormantSubAgents),
		WorkItemCount:         current.workItemCount,
	}, nil
}

func savedTeamResolutionMatchesBinding(
	load TeamDefinitionLoadCandidate,
	binding SavedTeamRuntimeBindingCandidate,
) bool {
	return load.Found &&
		load.ID == binding.TeamDefinitionID() &&
		load.Version == binding.TeamDefinitionVersion() &&
		load.Scope == binding.TeamDefinitionScope() &&
		load.ScopeIdentity == binding.ScopeIdentity() &&
		load.DefinitionDigest == binding.TeamDefinitionDigest()
}

func validSavedTeamInstantiationPlanShape(input SavedTeamInstantiationPlanCandidate) bool {
	return input.ready &&
		input.trigger != "" &&
		input.resolutionDigest != "" &&
		input.teamDefinitionID != "" &&
		input.teamDefinitionVersion > 0 &&
		input.teamDefinitionDigest != "" &&
		input.runtimeDiscoveryDigest != "" &&
		input.bindingDigest != "" &&
		input.mainSeed.AgentDefinitionID != "" &&
		input.mainSeed.RuntimeProfileID != "" &&
		input.mainSeed.RuntimeInstanceID != "" &&
		input.mainSeed.Binding.Accepted &&
		input.mainSeed.Binding.ProfileID == input.mainSeed.RuntimeProfileID &&
		input.mainSeed.Binding.InstanceID == input.mainSeed.RuntimeInstanceID &&
		len(input.dormantSubAgents) <= 2 &&
		input.createTeamInstance &&
		input.createMainAgentInstance &&
		!input.createSubAgentInstances &&
		!input.requiresDraft &&
		input.workItemCount == 0 &&
		input.planDigest != ""
}

func validSavedTeamDormantSubAgents(input []SavedTeamDormantSubAgentSeed) bool {
	for index, seed := range input {
		if !seed.Dormant || seed.AgentDefinitionID == "" ||
			seed.RuntimeProfileID == "" || seed.RuntimeInstanceID == "" {
			return false
		}
		if index > 0 && !lessSavedTeamDormantSubAgent(input[index-1], seed) {
			return false
		}
	}
	return true
}

func sortSavedTeamDormantSubAgents(input []SavedTeamDormantSubAgentSeed) {
	sort.Slice(input, func(i, j int) bool {
		return lessSavedTeamDormantSubAgent(input[i], input[j])
	})
}

func lessSavedTeamDormantSubAgent(left, right SavedTeamDormantSubAgentSeed) bool {
	if left.AgentDefinitionID != right.AgentDefinitionID {
		return left.AgentDefinitionID < right.AgentDefinitionID
	}
	if left.RuntimeProfileID != right.RuntimeProfileID {
		return left.RuntimeProfileID < right.RuntimeProfileID
	}
	return left.RuntimeInstanceID < right.RuntimeInstanceID
}

func digestSavedTeamInstantiationPlan(input SavedTeamInstantiationPlanCandidate) (string, error) {
	canonical := struct {
		Ready                   bool
		Trigger                 mode.Trigger
		TargetID                string
		ResolutionDigest        string
		TeamDefinitionID        string
		TeamDefinitionVersion   int
		TeamDefinitionScope     TeamDefinitionScope
		ScopeIdentity           agents.ScopeIdentity
		TeamDefinitionDigest    string
		RuntimeDiscoveryDigest  string
		BindingDigest           string
		MainSeed                SavedTeamMainInstanceSeed
		DormantSubAgents        []SavedTeamDormantSubAgentSeed
		CreateTeamInstance      bool
		CreateMainAgentInstance bool
		CreateSubAgentInstances bool
		RequiresDraft           bool
		WorkItemCount           int
	}{
		Ready:                   input.ready,
		Trigger:                 input.trigger,
		TargetID:                input.targetID,
		ResolutionDigest:        input.resolutionDigest,
		TeamDefinitionID:        input.teamDefinitionID,
		TeamDefinitionVersion:   input.teamDefinitionVersion,
		TeamDefinitionScope:     input.teamDefinitionScope,
		ScopeIdentity:           input.scopeIdentity,
		TeamDefinitionDigest:    input.teamDefinitionDigest,
		RuntimeDiscoveryDigest:  input.runtimeDiscoveryDigest,
		BindingDigest:           input.bindingDigest,
		MainSeed:                input.mainSeed,
		DormantSubAgents:        append([]SavedTeamDormantSubAgentSeed(nil), input.dormantSubAgents...),
		CreateTeamInstance:      input.createTeamInstance,
		CreateMainAgentInstance: input.createMainAgentInstance,
		CreateSubAgentInstances: input.createSubAgentInstances,
		RequiresDraft:           input.requiresDraft,
		WorkItemCount:           input.workItemCount,
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func cloneSavedTeamInstantiationPlan(
	input SavedTeamInstantiationPlanCandidate,
) SavedTeamInstantiationPlanCandidate {
	input.dormantSubAgents = append(
		[]SavedTeamDormantSubAgentSeed(nil), input.dormantSubAgents...,
	)
	return input
}

func (c SavedTeamInstantiationPlanCandidate) Ready() bool {
	return c.ready
}

func (c SavedTeamInstantiationPlanCandidate) Trigger() mode.Trigger {
	return c.trigger
}

func (c SavedTeamInstantiationPlanCandidate) TargetID() string {
	return c.targetID
}

func (c SavedTeamInstantiationPlanCandidate) ResolutionDigest() string {
	return c.resolutionDigest
}

func (c SavedTeamInstantiationPlanCandidate) TeamDefinitionID() string {
	return c.teamDefinitionID
}

func (c SavedTeamInstantiationPlanCandidate) TeamDefinitionVersion() int {
	return c.teamDefinitionVersion
}

func (c SavedTeamInstantiationPlanCandidate) TeamDefinitionScope() TeamDefinitionScope {
	return c.teamDefinitionScope
}

func (c SavedTeamInstantiationPlanCandidate) ScopeIdentity() agents.ScopeIdentity {
	return c.scopeIdentity
}

func (c SavedTeamInstantiationPlanCandidate) TeamDefinitionDigest() string {
	return c.teamDefinitionDigest
}

func (c SavedTeamInstantiationPlanCandidate) RuntimeDiscoveryDigest() string {
	return c.runtimeDiscoveryDigest
}

func (c SavedTeamInstantiationPlanCandidate) BindingDigest() string {
	return c.bindingDigest
}

func (c SavedTeamInstantiationPlanCandidate) MainSeed() SavedTeamMainInstanceSeed {
	return c.mainSeed
}

func (c SavedTeamInstantiationPlanCandidate) DormantSubAgents() []SavedTeamDormantSubAgentSeed {
	return append([]SavedTeamDormantSubAgentSeed(nil), c.dormantSubAgents...)
}

func (c SavedTeamInstantiationPlanCandidate) CreateTeamInstance() bool {
	return c.createTeamInstance
}

func (c SavedTeamInstantiationPlanCandidate) CreateMainAgentInstance() bool {
	return c.createMainAgentInstance
}

func (c SavedTeamInstantiationPlanCandidate) CreateSubAgentInstances() bool {
	return c.createSubAgentInstances
}

func (c SavedTeamInstantiationPlanCandidate) RequiresDraft() bool {
	return c.requiresDraft
}

func (c SavedTeamInstantiationPlanCandidate) WorkItemCount() int {
	return c.workItemCount
}

func (c SavedTeamInstantiationPlanCandidate) PlanDigest() string {
	return c.planDigest
}
