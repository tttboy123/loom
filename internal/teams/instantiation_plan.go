package teams

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"sort"

	"loom-pi-rebuild/internal/runtime"
)

var (
	ErrInvalidTeamInstantiationPlan           = errors.New("invalid team instantiation plan")
	ErrTeamInstantiationRequiresAcceptedDraft = errors.New("team instantiation requires accepted draft")
	ErrTeamInstantiationRoleTaskMismatch      = errors.New("team instantiation role task mismatch")
	ErrTeamInstantiationPlanDigestMismatch    = errors.New("team instantiation plan digest mismatch")
	ErrTeamInstantiationPlanSourceMismatch    = errors.New("team instantiation plan source mismatch")
)

type TeamInstantiationSourceKind string

const TeamInstantiationSourceAcceptedDraft TeamInstantiationSourceKind = "accepted_draft"

type TeamInstantiationRoleKind string

const (
	TeamInstantiationRoleMain     TeamInstantiationRoleKind = "main"
	TeamInstantiationRoleSubAgent TeamInstantiationRoleKind = "subagent"
)

type TeamInstantiationRoleSeed struct {
	Kind              TeamInstantiationRoleKind
	AgentDefinitionID string
	RuntimeProfile    runtime.RuntimeProfile
	RuntimeInstanceID string
	SkillIDs          []string
	MemberIDs         []string
	PermissionIDs     []string
}

type TeamInstantiationWorkItemSeed struct {
	ID                     string
	OwnerAgentDefinitionID string
	DependencyTaskIDs      []string
	AcceptanceCriteria     []string
}

type TeamInstantiationPlanCandidate struct {
	ready                bool
	sourceKind           TeamInstantiationSourceKind
	draftID              string
	sourceRevision       int
	terminalRevision     int
	catalogDigest        string
	contentDigest        string
	bindingDigest        string
	decisionDigest       string
	mainRole             TeamInstantiationRoleSeed
	subAgentRoles        []TeamInstantiationRoleSeed
	workItems            []TeamInstantiationWorkItemSeed
	customerRuleSummary  string
	approvalMarkers      []TeamDraftApprovalMarker
	requestedBudget      int
	requestedConcurrency int
	budgetCeiling        int
	concurrencyCeiling   int
	planDigest           string
}

type TeamInstantiationPlanValidationCandidate struct {
	Valid                 bool
	DraftID               string
	DecisionDigest        string
	PlanDigest            string
	MainAgentDefinitionID string
	RoleCount             int
	TaskCount             int
}

func BuildAcceptedDraftInstantiationPlan(
	decided DecidedTeamDraft,
	catalog TeamDraftCatalogSnapshot,
) (TeamInstantiationPlanCandidate, error) {
	decision, err := ValidateDecidedTeamDraft(decided, catalog)
	if err != nil {
		return TeamInstantiationPlanCandidate{}, err
	}
	if decision.Kind != TeamDraftDecisionAccepted {
		return TeamInstantiationPlanCandidate{}, ErrTeamInstantiationRequiresAcceptedDraft
	}

	content := decided.Content()
	contentCandidate, err := ValidateTeamDraftContent(content, catalog)
	if err != nil {
		return TeamInstantiationPlanCandidate{}, err
	}
	if !contentCandidate.AcceptanceReady {
		return TeamInstantiationPlanCandidate{}, ErrInvalidTeamInstantiationPlan
	}
	references := content.References()
	if references.RequestedBudget < 0 ||
		references.RequestedBudget > catalog.BudgetCeiling() ||
		references.RequestedConcurrency <= 0 ||
		references.RequestedConcurrency > catalog.ConcurrencyCeiling() {
		return TeamInstantiationPlanCandidate{}, ErrInvalidTeamInstantiationPlan
	}

	main, subAgents, err := buildInstantiationRoleSeeds(content.Roles(), references)
	if err != nil {
		return TeamInstantiationPlanCandidate{}, err
	}
	workItems, err := buildInstantiationWorkItemSeeds(content.Tasks(), main, subAgents)
	if err != nil {
		return TeamInstantiationPlanCandidate{}, err
	}

	plan := TeamInstantiationPlanCandidate{
		ready:                true,
		sourceKind:           TeamInstantiationSourceAcceptedDraft,
		draftID:              decision.DraftID,
		sourceRevision:       decision.SourceRevision,
		terminalRevision:     decision.Revision,
		catalogDigest:        decision.CatalogDigest,
		contentDigest:        decision.ContentDigest,
		bindingDigest:        decision.BindingDigest,
		decisionDigest:       decision.DecisionDigest,
		mainRole:             cloneTeamInstantiationRoleSeed(main),
		subAgentRoles:        cloneTeamInstantiationRoleSeeds(subAgents),
		workItems:            cloneTeamInstantiationWorkItemSeeds(workItems),
		customerRuleSummary:  content.CustomerRuleSummary(),
		approvalMarkers:      append([]TeamDraftApprovalMarker(nil), content.ApprovalMarkers()...),
		requestedBudget:      references.RequestedBudget,
		requestedConcurrency: references.RequestedConcurrency,
		budgetCeiling:        catalog.BudgetCeiling(),
		concurrencyCeiling:   catalog.ConcurrencyCeiling(),
	}
	sort.Slice(plan.approvalMarkers, func(i, j int) bool {
		return plan.approvalMarkers[i].ID < plan.approvalMarkers[j].ID
	})
	plan.planDigest, err = digestTeamInstantiationPlan(plan)
	if err != nil {
		return TeamInstantiationPlanCandidate{}, err
	}
	return plan, nil
}

func ValidateAcceptedDraftInstantiationPlan(
	current TeamInstantiationPlanCandidate,
	decided DecidedTeamDraft,
	catalog TeamDraftCatalogSnapshot,
) (TeamInstantiationPlanValidationCandidate, error) {
	if err := validateTeamInstantiationPlanShape(current); err != nil {
		return TeamInstantiationPlanValidationCandidate{}, err
	}
	digest, err := digestTeamInstantiationPlan(current)
	if err != nil {
		return TeamInstantiationPlanValidationCandidate{}, err
	}
	if digest != current.planDigest {
		return TeamInstantiationPlanValidationCandidate{}, ErrTeamInstantiationPlanDigestMismatch
	}
	expected, err := BuildAcceptedDraftInstantiationPlan(decided, catalog)
	if err != nil {
		return TeamInstantiationPlanValidationCandidate{}, err
	}
	if !reflect.DeepEqual(current, expected) {
		return TeamInstantiationPlanValidationCandidate{}, ErrTeamInstantiationPlanSourceMismatch
	}
	return TeamInstantiationPlanValidationCandidate{
		Valid:                 true,
		DraftID:               current.draftID,
		DecisionDigest:        current.decisionDigest,
		PlanDigest:            current.planDigest,
		MainAgentDefinitionID: current.mainRole.AgentDefinitionID,
		RoleCount:             1 + len(current.subAgentRoles),
		TaskCount:             len(current.workItems),
	}, nil
}

func buildInstantiationRoleSeeds(
	roles []TeamDraftRoleSelection,
	references TeamDraftReferences,
) (TeamInstantiationRoleSeed, []TeamInstantiationRoleSeed, error) {
	if references.MainAgentDefinitionID == "" ||
		len(references.SubAgentDefinitionIDs) == 0 ||
		len(references.SubAgentDefinitionIDs) > MaxTeamAgentCount-1 {
		return TeamInstantiationRoleSeed{}, nil, ErrTeamInstantiationRoleTaskMismatch
	}
	roleByID := make(map[string]TeamDraftRoleSelection, len(roles))
	for _, role := range roles {
		if role.AgentDefinitionID == "" {
			return TeamInstantiationRoleSeed{}, nil, ErrTeamInstantiationRoleTaskMismatch
		}
		if _, exists := roleByID[role.AgentDefinitionID]; exists {
			return TeamInstantiationRoleSeed{}, nil, ErrTeamInstantiationRoleTaskMismatch
		}
		roleByID[role.AgentDefinitionID] = role
	}
	mainSelection, ok := roleByID[references.MainAgentDefinitionID]
	if !ok {
		return TeamInstantiationRoleSeed{}, nil, ErrTeamInstantiationRoleTaskMismatch
	}
	main := roleSeedFromSelection(TeamInstantiationRoleMain, mainSelection)

	subAgentIDs := append([]string(nil), references.SubAgentDefinitionIDs...)
	sort.Strings(subAgentIDs)
	subAgents := make([]TeamInstantiationRoleSeed, 0, len(subAgentIDs))
	for _, id := range subAgentIDs {
		selection, found := roleByID[id]
		if !found || id == references.MainAgentDefinitionID {
			return TeamInstantiationRoleSeed{}, nil, ErrTeamInstantiationRoleTaskMismatch
		}
		subAgents = append(subAgents, roleSeedFromSelection(TeamInstantiationRoleSubAgent, selection))
	}
	if len(roleByID) != 1+len(subAgents) {
		return TeamInstantiationRoleSeed{}, nil, ErrTeamInstantiationRoleTaskMismatch
	}
	return main, subAgents, nil
}

func buildInstantiationWorkItemSeeds(
	tasks []TeamDraftTaskCandidate,
	main TeamInstantiationRoleSeed,
	subAgents []TeamInstantiationRoleSeed,
) ([]TeamInstantiationWorkItemSeed, error) {
	if len(tasks) == 0 {
		return nil, ErrTeamInstantiationRoleTaskMismatch
	}
	subAgentIDs := make(map[string]int, len(subAgents))
	for _, role := range subAgents {
		subAgentIDs[role.AgentDefinitionID] = 0
	}
	seeds := make([]TeamInstantiationWorkItemSeed, 0, len(tasks))
	for _, task := range tasks {
		if task.ID == "" || task.OwnerAgentDefinitionID == main.AgentDefinitionID {
			return nil, ErrTeamInstantiationRoleTaskMismatch
		}
		if _, ok := subAgentIDs[task.OwnerAgentDefinitionID]; !ok {
			return nil, ErrTeamInstantiationRoleTaskMismatch
		}
		subAgentIDs[task.OwnerAgentDefinitionID]++
		dependencies := append([]string(nil), task.DependencyTaskIDs...)
		criteria := append([]string(nil), task.AcceptanceCriteria...)
		sort.Strings(dependencies)
		sort.Strings(criteria)
		seeds = append(seeds, TeamInstantiationWorkItemSeed{
			ID:                     task.ID,
			OwnerAgentDefinitionID: task.OwnerAgentDefinitionID,
			DependencyTaskIDs:      dependencies,
			AcceptanceCriteria:     criteria,
		})
	}
	for _, count := range subAgentIDs {
		if count == 0 {
			return nil, ErrTeamInstantiationRoleTaskMismatch
		}
	}
	sort.Slice(seeds, func(i, j int) bool {
		return seeds[i].ID < seeds[j].ID
	})
	return seeds, nil
}

func roleSeedFromSelection(
	kind TeamInstantiationRoleKind,
	selection TeamDraftRoleSelection,
) TeamInstantiationRoleSeed {
	skills := append([]string(nil), selection.SkillIDs...)
	members := append([]string(nil), selection.MemberIDs...)
	permissions := append([]string(nil), selection.PermissionIDs...)
	sort.Strings(skills)
	sort.Strings(members)
	sort.Strings(permissions)
	return TeamInstantiationRoleSeed{
		Kind:              kind,
		AgentDefinitionID: selection.AgentDefinitionID,
		RuntimeProfile:    copyTeamDraftRuntimeProfile(selection.RuntimeProfile),
		RuntimeInstanceID: selection.RuntimeInstanceID,
		SkillIDs:          skills,
		MemberIDs:         members,
		PermissionIDs:     permissions,
	}
}

func validateTeamInstantiationPlanShape(input TeamInstantiationPlanCandidate) error {
	if !input.ready ||
		input.sourceKind != TeamInstantiationSourceAcceptedDraft ||
		input.draftID == "" ||
		input.sourceRevision <= 0 ||
		input.terminalRevision != input.sourceRevision+1 ||
		input.catalogDigest == "" ||
		input.contentDigest == "" ||
		input.bindingDigest == "" ||
		input.decisionDigest == "" ||
		input.planDigest == "" ||
		input.mainRole.Kind != TeamInstantiationRoleMain ||
		input.mainRole.AgentDefinitionID == "" ||
		len(input.subAgentRoles) == 0 ||
		len(input.subAgentRoles) > MaxTeamAgentCount-1 ||
		len(input.workItems) == 0 ||
		input.customerRuleSummary == "" ||
		input.requestedBudget < 0 ||
		input.requestedBudget > input.budgetCeiling ||
		input.requestedConcurrency <= 0 ||
		input.requestedConcurrency > input.concurrencyCeiling {
		return ErrInvalidTeamInstantiationPlan
	}
	return nil
}

func digestTeamInstantiationPlan(input TeamInstantiationPlanCandidate) (string, error) {
	canonical := struct {
		Ready                bool
		SourceKind           TeamInstantiationSourceKind
		DraftID              string
		SourceRevision       int
		TerminalRevision     int
		CatalogDigest        string
		ContentDigest        string
		BindingDigest        string
		DecisionDigest       string
		MainRole             TeamInstantiationRoleSeed
		SubAgentRoles        []TeamInstantiationRoleSeed
		WorkItems            []TeamInstantiationWorkItemSeed
		CustomerRuleSummary  string
		ApprovalMarkers      []TeamDraftApprovalMarker
		RequestedBudget      int
		RequestedConcurrency int
		BudgetCeiling        int
		ConcurrencyCeiling   int
	}{
		Ready:                input.ready,
		SourceKind:           input.sourceKind,
		DraftID:              input.draftID,
		SourceRevision:       input.sourceRevision,
		TerminalRevision:     input.terminalRevision,
		CatalogDigest:        input.catalogDigest,
		ContentDigest:        input.contentDigest,
		BindingDigest:        input.bindingDigest,
		DecisionDigest:       input.decisionDigest,
		MainRole:             cloneTeamInstantiationRoleSeed(input.mainRole),
		SubAgentRoles:        cloneTeamInstantiationRoleSeeds(input.subAgentRoles),
		WorkItems:            cloneTeamInstantiationWorkItemSeeds(input.workItems),
		CustomerRuleSummary:  input.customerRuleSummary,
		ApprovalMarkers:      append([]TeamDraftApprovalMarker(nil), input.approvalMarkers...),
		RequestedBudget:      input.requestedBudget,
		RequestedConcurrency: input.requestedConcurrency,
		BudgetCeiling:        input.budgetCeiling,
		ConcurrencyCeiling:   input.concurrencyCeiling,
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func cloneTeamInstantiationPlan(input TeamInstantiationPlanCandidate) TeamInstantiationPlanCandidate {
	input.mainRole = cloneTeamInstantiationRoleSeed(input.mainRole)
	input.subAgentRoles = cloneTeamInstantiationRoleSeeds(input.subAgentRoles)
	input.workItems = cloneTeamInstantiationWorkItemSeeds(input.workItems)
	input.approvalMarkers = append([]TeamDraftApprovalMarker(nil), input.approvalMarkers...)
	return input
}

func cloneTeamInstantiationRoleSeeds(input []TeamInstantiationRoleSeed) []TeamInstantiationRoleSeed {
	copied := make([]TeamInstantiationRoleSeed, len(input))
	for index, role := range input {
		copied[index] = cloneTeamInstantiationRoleSeed(role)
	}
	return copied
}

func cloneTeamInstantiationRoleSeed(input TeamInstantiationRoleSeed) TeamInstantiationRoleSeed {
	input.RuntimeProfile = copyTeamDraftRuntimeProfile(input.RuntimeProfile)
	input.SkillIDs = append([]string(nil), input.SkillIDs...)
	input.MemberIDs = append([]string(nil), input.MemberIDs...)
	input.PermissionIDs = append([]string(nil), input.PermissionIDs...)
	return input
}

func cloneTeamInstantiationWorkItemSeeds(input []TeamInstantiationWorkItemSeed) []TeamInstantiationWorkItemSeed {
	copied := make([]TeamInstantiationWorkItemSeed, len(input))
	for index, item := range input {
		item.DependencyTaskIDs = append([]string(nil), item.DependencyTaskIDs...)
		item.AcceptanceCriteria = append([]string(nil), item.AcceptanceCriteria...)
		copied[index] = item
	}
	return copied
}

func (p TeamInstantiationPlanCandidate) Ready() bool {
	return p.ready
}

func (p TeamInstantiationPlanCandidate) SourceKind() TeamInstantiationSourceKind {
	return p.sourceKind
}

func (p TeamInstantiationPlanCandidate) DraftID() string {
	return p.draftID
}

func (p TeamInstantiationPlanCandidate) SourceRevision() int {
	return p.sourceRevision
}

func (p TeamInstantiationPlanCandidate) TerminalRevision() int {
	return p.terminalRevision
}

func (p TeamInstantiationPlanCandidate) CatalogDigest() string {
	return p.catalogDigest
}

func (p TeamInstantiationPlanCandidate) ContentDigest() string {
	return p.contentDigest
}

func (p TeamInstantiationPlanCandidate) BindingDigest() string {
	return p.bindingDigest
}

func (p TeamInstantiationPlanCandidate) DecisionDigest() string {
	return p.decisionDigest
}

func (p TeamInstantiationPlanCandidate) MainRole() TeamInstantiationRoleSeed {
	return cloneTeamInstantiationRoleSeed(p.mainRole)
}

func (p TeamInstantiationPlanCandidate) SubAgentRoles() []TeamInstantiationRoleSeed {
	return cloneTeamInstantiationRoleSeeds(p.subAgentRoles)
}

func (p TeamInstantiationPlanCandidate) WorkItems() []TeamInstantiationWorkItemSeed {
	return cloneTeamInstantiationWorkItemSeeds(p.workItems)
}

func (p TeamInstantiationPlanCandidate) CustomerRuleSummary() string {
	return p.customerRuleSummary
}

func (p TeamInstantiationPlanCandidate) ApprovalMarkers() []TeamDraftApprovalMarker {
	return append([]TeamDraftApprovalMarker(nil), p.approvalMarkers...)
}

func (p TeamInstantiationPlanCandidate) CapabilityGaps() []TeamDraftCapabilityGap {
	return nil
}

func (p TeamInstantiationPlanCandidate) RequestedBudget() int {
	return p.requestedBudget
}

func (p TeamInstantiationPlanCandidate) RequestedConcurrency() int {
	return p.requestedConcurrency
}

func (p TeamInstantiationPlanCandidate) BudgetCeiling() int {
	return p.budgetCeiling
}

func (p TeamInstantiationPlanCandidate) ConcurrencyCeiling() int {
	return p.concurrencyCeiling
}

func (p TeamInstantiationPlanCandidate) PlanDigest() string {
	return p.planDigest
}
