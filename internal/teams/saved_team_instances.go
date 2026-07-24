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
	ErrInvalidSavedTeamInstanceIdentity         = errors.New("invalid saved team instance identity")
	ErrInvalidSavedTeamInstanceRecordSet        = errors.New("invalid saved team instance record set")
	ErrSavedTeamInstanceMainDefinitionMismatch  = errors.New("saved team instance main definition mismatch")
	ErrInvalidSavedTeamInstanceDormantState     = errors.New("invalid saved team instance dormant state")
	ErrSavedTeamInstanceRecordSourceMismatch    = errors.New("saved team instance record source mismatch")
	ErrSavedTeamInstanceRecordSetDigestMismatch = errors.New("saved team instance record set digest mismatch")
)

type SavedTeamInstanceSourceKind string

const SavedTeamInstanceSourceSavedTeam SavedTeamInstanceSourceKind = "saved_team"

type SavedTeamInstanceState string

const SavedTeamInstanceStateCreated SavedTeamInstanceState = "created"

type SavedTeamAgentInstanceState string

const SavedTeamAgentInstanceStateCreated SavedTeamAgentInstanceState = "created"

type SavedTeamInstanceIdentityInput struct {
	WorkRequestID       string
	TeamInstanceID      string
	MainAgentInstanceID string
	CreatedAt           int64
}

type SavedTeamInstanceRecord struct {
	ID                    string
	WorkRequestID         string
	SourceKind            SavedTeamInstanceSourceKind
	TeamDefinitionID      string
	TeamDefinitionVersion int
	TeamDefinitionScope   TeamDefinitionScope
	ScopeIdentity         agents.ScopeIdentity
	TeamDefinitionDigest  string
	SourcePlanDigest      string
	State                 SavedTeamInstanceState
	CreatedAt             int64
}

type SavedTeamMainAgentInstanceRecord struct {
	ID                     string
	TeamInstanceID         string
	AgentDefinitionID      string
	AgentDefinitionVersion int
	AgentDefinitionScope   agents.Scope
	ScopeIdentity          agents.ScopeIdentity
	RuntimeProfileID       string
	RuntimeInstanceID      string
	IsMain                 bool
	State                  SavedTeamAgentInstanceState
	Binding                loomruntime.BindingCandidate
}

type SavedTeamDormantSubAgentRecord struct {
	Dormant           bool
	AgentDefinitionID string
	RuntimeProfileID  string
	RuntimeInstanceID string
}

type SavedTeamInstanceRecordSetCandidate struct {
	ready               bool
	team                SavedTeamInstanceRecord
	mainAgent           SavedTeamMainAgentInstanceRecord
	dormantSubAgents    []SavedTeamDormantSubAgentRecord
	teamInstanceCount   int
	agentInstanceCount  int
	activeSubAgentCount int
	workItemCount       int
	sourcePlanDigest    string
	recordSetDigest     string
}

type SavedTeamInstanceRecordSetValidationCandidate struct {
	Valid                bool
	TeamInstanceID       string
	MainAgentInstanceID  string
	TeamDefinitionID     string
	TeamDefinitionDigest string
	SourcePlanDigest     string
	RecordSetDigest      string
	TeamInstanceCount    int
	AgentInstanceCount   int
	ActiveSubAgentCount  int
	WorkItemCount        int
}

func BuildSavedTeamInstanceRecordSet(
	plan SavedTeamInstantiationPlanCandidate,
	intent mode.Intent,
	context agents.ScopeIdentity,
	catalog TeamResolutionCatalogInput,
	binding SavedTeamRuntimeBindingCandidate,
	discovery loomruntime.RuntimeDiscoverySnapshot,
	selections []SavedTeamRuntimeSelection,
	identity SavedTeamInstanceIdentityInput,
) (SavedTeamInstanceRecordSetCandidate, error) {
	validated, err := ValidateSavedTeamInstantiationPlan(
		plan, intent, context, catalog, binding, discovery, selections,
	)
	if err != nil {
		return SavedTeamInstanceRecordSetCandidate{}, err
	}
	if !validated.Valid {
		return SavedTeamInstanceRecordSetCandidate{}, ErrInvalidSavedTeamInstanceRecordSet
	}
	if identity.WorkRequestID == "" || identity.TeamInstanceID == "" ||
		identity.MainAgentInstanceID == "" || identity.CreatedAt <= 0 {
		return SavedTeamInstanceRecordSetCandidate{}, ErrInvalidSavedTeamInstanceIdentity
	}

	mainSeed := plan.MainSeed()
	mainBinding := binding.MainBinding()
	definition, err := agents.ResolveDefinition(
		catalog.AgentDefinitions,
		agents.ResolutionContext{
			DefinitionID: mainSeed.AgentDefinitionID,
			ProjectID:    plan.ScopeIdentity().ProjectID,
			GenerationID: plan.ScopeIdentity().GenerationID,
		},
	)
	if err != nil {
		return SavedTeamInstanceRecordSetCandidate{}, err
	}
	if definition.ID != mainSeed.AgentDefinitionID ||
		definition.ID != mainBinding.AgentDefinitionID ||
		mainSeed.RuntimeProfileID != mainBinding.RuntimeProfileID ||
		mainSeed.RuntimeInstanceID != mainBinding.RuntimeInstanceID ||
		mainSeed.Binding != mainBinding.Binding {
		return SavedTeamInstanceRecordSetCandidate{}, ErrSavedTeamInstanceMainDefinitionMismatch
	}

	dormantSeeds := plan.DormantSubAgents()
	dormant := make([]SavedTeamDormantSubAgentRecord, len(dormantSeeds))
	for index, seed := range dormantSeeds {
		if !seed.Dormant {
			return SavedTeamInstanceRecordSetCandidate{}, ErrInvalidSavedTeamInstanceDormantState
		}
		dormant[index] = SavedTeamDormantSubAgentRecord{
			Dormant:           true,
			AgentDefinitionID: seed.AgentDefinitionID,
			RuntimeProfileID:  seed.RuntimeProfileID,
			RuntimeInstanceID: seed.RuntimeInstanceID,
		}
	}
	sortSavedTeamDormantRecords(dormant)

	candidate := SavedTeamInstanceRecordSetCandidate{
		ready: true,
		team: SavedTeamInstanceRecord{
			ID:                    identity.TeamInstanceID,
			WorkRequestID:         identity.WorkRequestID,
			SourceKind:            SavedTeamInstanceSourceSavedTeam,
			TeamDefinitionID:      plan.TeamDefinitionID(),
			TeamDefinitionVersion: plan.TeamDefinitionVersion(),
			TeamDefinitionScope:   plan.TeamDefinitionScope(),
			ScopeIdentity:         plan.ScopeIdentity(),
			TeamDefinitionDigest:  plan.TeamDefinitionDigest(),
			SourcePlanDigest:      plan.PlanDigest(),
			State:                 SavedTeamInstanceStateCreated,
			CreatedAt:             identity.CreatedAt,
		},
		mainAgent: SavedTeamMainAgentInstanceRecord{
			ID:                     identity.MainAgentInstanceID,
			TeamInstanceID:         identity.TeamInstanceID,
			AgentDefinitionID:      definition.ID,
			AgentDefinitionVersion: definition.Version,
			AgentDefinitionScope:   definition.Scope,
			ScopeIdentity:          definition.ScopeIdentity,
			RuntimeProfileID:       mainSeed.RuntimeProfileID,
			RuntimeInstanceID:      mainSeed.RuntimeInstanceID,
			IsMain:                 true,
			State:                  SavedTeamAgentInstanceStateCreated,
			Binding:                mainSeed.Binding,
		},
		dormantSubAgents:    append([]SavedTeamDormantSubAgentRecord(nil), dormant...),
		teamInstanceCount:   1,
		agentInstanceCount:  1,
		activeSubAgentCount: 0,
		workItemCount:       0,
		sourcePlanDigest:    plan.PlanDigest(),
	}
	candidate.recordSetDigest, err = digestSavedTeamInstanceRecordSet(candidate)
	if err != nil {
		return SavedTeamInstanceRecordSetCandidate{}, err
	}
	return candidate, nil
}

func ValidateSavedTeamInstanceRecordSet(
	current SavedTeamInstanceRecordSetCandidate,
	plan SavedTeamInstantiationPlanCandidate,
	intent mode.Intent,
	context agents.ScopeIdentity,
	catalog TeamResolutionCatalogInput,
	binding SavedTeamRuntimeBindingCandidate,
	discovery loomruntime.RuntimeDiscoverySnapshot,
	selections []SavedTeamRuntimeSelection,
	identity SavedTeamInstanceIdentityInput,
) (SavedTeamInstanceRecordSetValidationCandidate, error) {
	if !validSavedTeamInstanceRecordSetShape(current) {
		return SavedTeamInstanceRecordSetValidationCandidate{}, ErrInvalidSavedTeamInstanceRecordSet
	}
	if !validSavedTeamDormantRecords(current.dormantSubAgents) {
		return SavedTeamInstanceRecordSetValidationCandidate{}, ErrInvalidSavedTeamInstanceDormantState
	}
	digest, err := digestSavedTeamInstanceRecordSet(current)
	if err != nil {
		return SavedTeamInstanceRecordSetValidationCandidate{}, err
	}
	if digest != current.recordSetDigest {
		return SavedTeamInstanceRecordSetValidationCandidate{}, ErrSavedTeamInstanceRecordSetDigestMismatch
	}
	expected, err := BuildSavedTeamInstanceRecordSet(
		plan, intent, context, catalog, binding, discovery, selections, identity,
	)
	if err != nil {
		return SavedTeamInstanceRecordSetValidationCandidate{}, err
	}
	if !reflect.DeepEqual(current, expected) {
		return SavedTeamInstanceRecordSetValidationCandidate{}, ErrSavedTeamInstanceRecordSourceMismatch
	}
	return SavedTeamInstanceRecordSetValidationCandidate{
		Valid:                true,
		TeamInstanceID:       current.team.ID,
		MainAgentInstanceID:  current.mainAgent.ID,
		TeamDefinitionID:     current.team.TeamDefinitionID,
		TeamDefinitionDigest: current.team.TeamDefinitionDigest,
		SourcePlanDigest:     current.sourcePlanDigest,
		RecordSetDigest:      current.recordSetDigest,
		TeamInstanceCount:    current.teamInstanceCount,
		AgentInstanceCount:   current.agentInstanceCount,
		ActiveSubAgentCount:  current.activeSubAgentCount,
		WorkItemCount:        current.workItemCount,
	}, nil
}

func validSavedTeamInstanceRecordSetShape(input SavedTeamInstanceRecordSetCandidate) bool {
	return input.ready &&
		input.team.ID != "" &&
		input.team.WorkRequestID != "" &&
		input.team.SourceKind == SavedTeamInstanceSourceSavedTeam &&
		input.team.TeamDefinitionID != "" &&
		input.team.TeamDefinitionVersion > 0 &&
		input.team.TeamDefinitionDigest != "" &&
		input.team.SourcePlanDigest != "" &&
		input.team.State == SavedTeamInstanceStateCreated &&
		input.team.CreatedAt > 0 &&
		input.mainAgent.ID != "" &&
		input.mainAgent.TeamInstanceID == input.team.ID &&
		input.mainAgent.AgentDefinitionID != "" &&
		input.mainAgent.AgentDefinitionVersion > 0 &&
		input.mainAgent.RuntimeProfileID != "" &&
		input.mainAgent.RuntimeInstanceID != "" &&
		input.mainAgent.IsMain &&
		input.mainAgent.State == SavedTeamAgentInstanceStateCreated &&
		input.mainAgent.Binding.Accepted &&
		input.mainAgent.Binding.ProfileID == input.mainAgent.RuntimeProfileID &&
		input.mainAgent.Binding.InstanceID == input.mainAgent.RuntimeInstanceID &&
		len(input.dormantSubAgents) <= 2 &&
		input.teamInstanceCount == 1 &&
		input.agentInstanceCount == 1 &&
		input.activeSubAgentCount == 0 &&
		input.workItemCount == 0 &&
		input.sourcePlanDigest != "" &&
		input.sourcePlanDigest == input.team.SourcePlanDigest &&
		input.recordSetDigest != ""
}

func validSavedTeamDormantRecords(input []SavedTeamDormantSubAgentRecord) bool {
	for index, record := range input {
		if !record.Dormant || record.AgentDefinitionID == "" ||
			record.RuntimeProfileID == "" || record.RuntimeInstanceID == "" {
			return false
		}
		if index > 0 && !lessSavedTeamDormantRecord(input[index-1], record) {
			return false
		}
	}
	return true
}

func sortSavedTeamDormantRecords(input []SavedTeamDormantSubAgentRecord) {
	sort.Slice(input, func(i, j int) bool {
		return lessSavedTeamDormantRecord(input[i], input[j])
	})
}

func lessSavedTeamDormantRecord(
	left SavedTeamDormantSubAgentRecord,
	right SavedTeamDormantSubAgentRecord,
) bool {
	if left.AgentDefinitionID != right.AgentDefinitionID {
		return left.AgentDefinitionID < right.AgentDefinitionID
	}
	if left.RuntimeProfileID != right.RuntimeProfileID {
		return left.RuntimeProfileID < right.RuntimeProfileID
	}
	return left.RuntimeInstanceID < right.RuntimeInstanceID
}

func digestSavedTeamInstanceRecordSet(
	input SavedTeamInstanceRecordSetCandidate,
) (string, error) {
	canonical := struct {
		Ready               bool
		Team                SavedTeamInstanceRecord
		MainAgent           SavedTeamMainAgentInstanceRecord
		DormantSubAgents    []SavedTeamDormantSubAgentRecord
		TeamInstanceCount   int
		AgentInstanceCount  int
		ActiveSubAgentCount int
		WorkItemCount       int
		SourcePlanDigest    string
	}{
		Ready:               input.ready,
		Team:                input.team,
		MainAgent:           input.mainAgent,
		DormantSubAgents:    append([]SavedTeamDormantSubAgentRecord(nil), input.dormantSubAgents...),
		TeamInstanceCount:   input.teamInstanceCount,
		AgentInstanceCount:  input.agentInstanceCount,
		ActiveSubAgentCount: input.activeSubAgentCount,
		WorkItemCount:       input.workItemCount,
		SourcePlanDigest:    input.sourcePlanDigest,
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func cloneSavedTeamInstanceRecordSet(
	input SavedTeamInstanceRecordSetCandidate,
) SavedTeamInstanceRecordSetCandidate {
	input.dormantSubAgents = append(
		[]SavedTeamDormantSubAgentRecord(nil), input.dormantSubAgents...,
	)
	return input
}

func (c SavedTeamInstanceRecordSetCandidate) Ready() bool {
	return c.ready
}

func (c SavedTeamInstanceRecordSetCandidate) Team() SavedTeamInstanceRecord {
	return c.team
}

func (c SavedTeamInstanceRecordSetCandidate) MainAgent() SavedTeamMainAgentInstanceRecord {
	return c.mainAgent
}

func (c SavedTeamInstanceRecordSetCandidate) DormantSubAgents() []SavedTeamDormantSubAgentRecord {
	return append([]SavedTeamDormantSubAgentRecord(nil), c.dormantSubAgents...)
}

func (c SavedTeamInstanceRecordSetCandidate) TeamInstanceCount() int {
	return c.teamInstanceCount
}

func (c SavedTeamInstanceRecordSetCandidate) AgentInstanceCount() int {
	return c.agentInstanceCount
}

func (c SavedTeamInstanceRecordSetCandidate) ActiveSubAgentCount() int {
	return c.activeSubAgentCount
}

func (c SavedTeamInstanceRecordSetCandidate) WorkItemCount() int {
	return c.workItemCount
}

func (c SavedTeamInstanceRecordSetCandidate) SourcePlanDigest() string {
	return c.sourcePlanDigest
}

func (c SavedTeamInstanceRecordSetCandidate) RecordSetDigest() string {
	return c.recordSetDigest
}
