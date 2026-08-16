package teams

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"loom-pi-rebuild/internal/runtime"
)

var (
	ErrInvalidTeamDraftContent              = errors.New("invalid team draft content")
	ErrTeamDraftContentLimitExceeded        = errors.New("team draft content limit exceeded")
	ErrInvalidTeamDraftRole                 = errors.New("invalid team draft role")
	ErrDuplicateTeamDraftRole               = errors.New("duplicate team draft role")
	ErrMissingTeamDraftRole                 = errors.New("missing team draft role")
	ErrExtraTeamDraftRole                   = errors.New("extra team draft role")
	ErrTeamDraftRuntimeProfileModelMismatch = errors.New("team draft runtime profile model mismatch")
	ErrTeamDraftRuntimeCoverageMismatch     = errors.New("team draft runtime coverage mismatch")
	ErrInvalidTeamDraftTask                 = errors.New("invalid team draft task")
	ErrDuplicateTeamDraftTask               = errors.New("duplicate team draft task")
	ErrUnknownTeamDraftTaskDependency       = errors.New("unknown team draft task dependency")
	ErrTeamDraftTaskSelfDependency          = errors.New("team draft task self dependency")
	ErrCyclicTeamDraftTaskGraph             = errors.New("cyclic team draft task graph")
	ErrMainAgentDeliveryAssignment          = errors.New("main agent delivery assignment")
	ErrUnassignedTeamDraftSubAgent          = errors.New("unassigned team draft subagent")
	ErrInvalidTeamDraftApprovalMarker       = errors.New("invalid team draft approval marker")
	ErrDuplicateTeamDraftApprovalMarker     = errors.New("duplicate team draft approval marker")
	ErrInvalidTeamDraftCapabilityGap        = errors.New("invalid team draft capability gap")
	ErrDuplicateTeamDraftCapabilityGap      = errors.New("duplicate team draft capability gap")
	ErrTeamDraftContentDigestMismatch       = errors.New("team draft content digest mismatch")
)

type TeamDraftContentLimits struct {
	MaxTasks                     int
	MaxDependenciesPerTask       int
	MaxAcceptanceCriteriaPerTask int
	MaxApprovalMarkers           int
	MaxCapabilityGaps            int
}

type TeamDraftRoleSelection struct {
	AgentDefinitionID string
	RuntimeProfile    runtime.RuntimeProfile
	RuntimeInstanceID string
	SkillIDs          []string
	MemberIDs         []string
	PermissionIDs     []string
}

type TeamDraftTaskCandidate struct {
	ID                     string
	OwnerAgentDefinitionID string
	DependencyTaskIDs      []string
	AcceptanceCriteria     []string
}

type TeamDraftApprovalMarker struct {
	ID     string
	Reason string
}

type TeamDraftCapabilityGap struct {
	Capability string
	Reason     string
}

type TeamDraftContentInput struct {
	References          TeamDraftReferences
	Roles               []TeamDraftRoleSelection
	Tasks               []TeamDraftTaskCandidate
	CustomerRuleSummary string
	ApprovalMarkers     []TeamDraftApprovalMarker
	CapabilityGaps      []TeamDraftCapabilityGap
	Limits              TeamDraftContentLimits
}

type TeamDraftContentSnapshot struct {
	digest              string
	catalogDigest       string
	references          TeamDraftReferences
	roles               []TeamDraftRoleSelection
	tasks               []TeamDraftTaskCandidate
	customerRuleSummary string
	approvalMarkers     []TeamDraftApprovalMarker
	capabilityGaps      []TeamDraftCapabilityGap
	limits              TeamDraftContentLimits
}

type TeamDraftContentValidationCandidate struct {
	Valid                 bool
	AcceptanceReady       bool
	ContentDigest         string
	CatalogDigest         string
	MainAgentDefinitionID string
	RoleCount             int
	TaskCount             int
}

func BuildTeamDraftContent(
	catalog TeamDraftCatalogSnapshot,
	input TeamDraftContentInput,
) (TeamDraftContentSnapshot, error) {
	if catalog.Digest() == "" {
		return TeamDraftContentSnapshot{}, ErrInvalidTeamDraftCatalog
	}
	if err := validateTeamDraftContentLimits(input.Limits); err != nil {
		return TeamDraftContentSnapshot{}, err
	}
	references, err := validateAndNormalizeDraftReferences(catalog, input.References)
	if err != nil {
		return TeamDraftContentSnapshot{}, err
	}
	if len(references.SubAgentDefinitionIDs) == 0 {
		return TeamDraftContentSnapshot{}, ErrMissingTeamDraftRole
	}
	if len(references.SubAgentDefinitionIDs) > MaxTeamAgentCount-1 {
		return TeamDraftContentSnapshot{}, ErrTeamDraftContentLimitExceeded
	}

	roles, err := validateAndNormalizeTeamDraftRoles(catalog, references, input.Roles)
	if err != nil {
		return TeamDraftContentSnapshot{}, err
	}
	tasks, err := validateAndNormalizeTeamDraftTasks(references, input.Tasks, input.Limits)
	if err != nil {
		return TeamDraftContentSnapshot{}, err
	}
	if input.CustomerRuleSummary == "" {
		return TeamDraftContentSnapshot{}, ErrInvalidTeamDraftContent
	}
	markers, err := validateAndNormalizeApprovalMarkers(input.ApprovalMarkers, input.Limits)
	if err != nil {
		return TeamDraftContentSnapshot{}, err
	}
	gaps, err := validateAndNormalizeCapabilityGaps(input.CapabilityGaps, input.Limits)
	if err != nil {
		return TeamDraftContentSnapshot{}, err
	}

	snapshot := TeamDraftContentSnapshot{
		catalogDigest:       catalog.Digest(),
		references:          copyTeamDraftReferences(references),
		roles:               copyTeamDraftRoles(roles),
		tasks:               copyTeamDraftTasks(tasks),
		customerRuleSummary: input.CustomerRuleSummary,
		approvalMarkers:     append([]TeamDraftApprovalMarker(nil), markers...),
		capabilityGaps:      append([]TeamDraftCapabilityGap(nil), gaps...),
		limits:              input.Limits,
	}
	digest, err := digestTeamDraftContent(snapshot)
	if err != nil {
		return TeamDraftContentSnapshot{}, err
	}
	snapshot.digest = digest
	return snapshot, nil
}

func ValidateTeamDraftContent(
	snapshot TeamDraftContentSnapshot,
	catalog TeamDraftCatalogSnapshot,
) (TeamDraftContentValidationCandidate, error) {
	if snapshot.digest == "" || snapshot.catalogDigest == "" {
		return TeamDraftContentValidationCandidate{}, ErrInvalidTeamDraftContent
	}
	if catalog.Digest() == "" {
		return TeamDraftContentValidationCandidate{}, ErrInvalidTeamDraftCatalog
	}
	if catalog.Digest() != snapshot.catalogDigest {
		return TeamDraftContentValidationCandidate{}, ErrTeamDraftCatalogMismatch
	}
	digest, err := digestTeamDraftContent(snapshot)
	if err != nil {
		return TeamDraftContentValidationCandidate{}, err
	}
	if digest != snapshot.digest {
		return TeamDraftContentValidationCandidate{}, ErrTeamDraftContentDigestMismatch
	}

	rebuilt, err := BuildTeamDraftContent(catalog, TeamDraftContentInput{
		References:          snapshot.References(),
		Roles:               snapshot.Roles(),
		Tasks:               snapshot.Tasks(),
		CustomerRuleSummary: snapshot.CustomerRuleSummary(),
		ApprovalMarkers:     snapshot.ApprovalMarkers(),
		CapabilityGaps:      snapshot.CapabilityGaps(),
		Limits:              snapshot.Limits(),
	})
	if err != nil {
		return TeamDraftContentValidationCandidate{}, err
	}
	if rebuilt.digest != snapshot.digest {
		return TeamDraftContentValidationCandidate{}, ErrTeamDraftContentDigestMismatch
	}
	return TeamDraftContentValidationCandidate{
		Valid:                 true,
		AcceptanceReady:       len(snapshot.capabilityGaps) == 0,
		ContentDigest:         snapshot.digest,
		CatalogDigest:         snapshot.catalogDigest,
		MainAgentDefinitionID: snapshot.references.MainAgentDefinitionID,
		RoleCount:             len(snapshot.roles),
		TaskCount:             len(snapshot.tasks),
	}, nil
}

func (s TeamDraftContentSnapshot) Digest() string {
	return s.digest
}

func (s TeamDraftContentSnapshot) CatalogDigest() string {
	return s.catalogDigest
}

func (s TeamDraftContentSnapshot) References() TeamDraftReferences {
	return copyTeamDraftReferences(s.references)
}

func (s TeamDraftContentSnapshot) Roles() []TeamDraftRoleSelection {
	return copyTeamDraftRoles(s.roles)
}

func (s TeamDraftContentSnapshot) Tasks() []TeamDraftTaskCandidate {
	return copyTeamDraftTasks(s.tasks)
}

func (s TeamDraftContentSnapshot) CustomerRuleSummary() string {
	return s.customerRuleSummary
}

func (s TeamDraftContentSnapshot) ApprovalMarkers() []TeamDraftApprovalMarker {
	return append([]TeamDraftApprovalMarker(nil), s.approvalMarkers...)
}

func (s TeamDraftContentSnapshot) CapabilityGaps() []TeamDraftCapabilityGap {
	return append([]TeamDraftCapabilityGap(nil), s.capabilityGaps...)
}

func (s TeamDraftContentSnapshot) Limits() TeamDraftContentLimits {
	return s.limits
}

func validateTeamDraftContentLimits(limits TeamDraftContentLimits) error {
	if limits.MaxTasks <= 0 ||
		limits.MaxDependenciesPerTask <= 0 ||
		limits.MaxAcceptanceCriteriaPerTask <= 0 ||
		limits.MaxApprovalMarkers <= 0 ||
		limits.MaxCapabilityGaps <= 0 {
		return ErrInvalidTeamDraftContent
	}
	return nil
}

func validateAndNormalizeTeamDraftRoles(
	catalog TeamDraftCatalogSnapshot,
	references TeamDraftReferences,
	input []TeamDraftRoleSelection,
) ([]TeamDraftRoleSelection, error) {
	expected := make(map[string]struct{}, len(references.SubAgentDefinitionIDs)+1)
	expected[references.MainAgentDefinitionID] = struct{}{}
	for _, id := range references.SubAgentDefinitionIDs {
		expected[id] = struct{}{}
	}
	seen := make(map[string]struct{}, len(input))
	runtimes := make(map[string]RuntimeCatalogEntry, len(catalog.runtimes))
	for _, entry := range catalog.runtimes {
		runtimes[entry.ID] = entry
	}
	referencedRuntimeIDs := stringSet(references.RuntimeInstanceIDs)
	referencedPairs := runtimeModelReferenceSet(references.RuntimeModels)
	usedRuntimeIDs := make(map[string]struct{})
	usedPairs := make(map[string]struct{})
	roles := make([]TeamDraftRoleSelection, 0, len(input))

	for _, role := range input {
		if role.AgentDefinitionID == "" {
			return nil, ErrInvalidTeamDraftRole
		}
		if _, ok := seen[role.AgentDefinitionID]; ok {
			return nil, ErrDuplicateTeamDraftRole
		}
		seen[role.AgentDefinitionID] = struct{}{}
		if _, ok := expected[role.AgentDefinitionID]; !ok {
			return nil, ErrExtraTeamDraftRole
		}
		if _, ok := referencedRuntimeIDs[role.RuntimeInstanceID]; !ok {
			return nil, ErrTeamDraftRuntimeCoverageMismatch
		}
		entry, ok := runtimes[role.RuntimeInstanceID]
		if !ok {
			return nil, ErrTeamDraftRuntimeCoverageMismatch
		}
		pairKey := runtimeModelKey(role.RuntimeInstanceID, role.RuntimeProfile.ModelID)
		if _, ok := referencedPairs[pairKey]; !ok {
			return nil, ErrTeamDraftRuntimeProfileModelMismatch
		}
		instance := runtime.RuntimeInstance{
			ID:                   entry.ID,
			DeviceID:             entry.DeviceID,
			AdapterType:          entry.AdapterType,
			DisplayName:          entry.DisplayName,
			ExecutableVersion:    entry.ExecutableVersion,
			Status:               entry.Status,
			ObservedCapabilities: append([]string(nil), entry.ObservedCapabilities...),
			Capacity:             entry.Capacity,
		}
		if _, err := runtime.ValidateBinding(role.RuntimeProfile, instance); err != nil {
			return nil, fmt.Errorf("validate team draft role %q: %w", role.AgentDefinitionID, err)
		}
		profile, err := runtime.NewRuntimeProfile(role.RuntimeProfile)
		if err != nil {
			return nil, fmt.Errorf("copy team draft role %q: %w", role.AgentDefinitionID, err)
		}
		skills, err := normalizeRoleReferenceSubset(role.SkillIDs, references.SkillIDs)
		if err != nil {
			return nil, err
		}
		members, err := normalizeRoleReferenceSubset(role.MemberIDs, references.MemberIDs)
		if err != nil {
			return nil, err
		}
		permissions, err := normalizeRoleReferenceSubset(role.PermissionIDs, references.PermissionIDs)
		if err != nil {
			return nil, err
		}
		roles = append(roles, TeamDraftRoleSelection{
			AgentDefinitionID: role.AgentDefinitionID,
			RuntimeProfile:    profile,
			RuntimeInstanceID: role.RuntimeInstanceID,
			SkillIDs:          skills,
			MemberIDs:         members,
			PermissionIDs:     permissions,
		})
		usedRuntimeIDs[role.RuntimeInstanceID] = struct{}{}
		usedPairs[pairKey] = struct{}{}
	}
	for id := range expected {
		if _, ok := seen[id]; !ok {
			return nil, ErrMissingTeamDraftRole
		}
	}
	if !equalStringSets(usedRuntimeIDs, referencedRuntimeIDs) ||
		!equalStringSets(usedPairs, referencedPairs) {
		return nil, ErrTeamDraftRuntimeCoverageMismatch
	}
	sort.Slice(roles, func(i, j int) bool {
		return roles[i].AgentDefinitionID < roles[j].AgentDefinitionID
	})
	return roles, nil
}

func validateAndNormalizeTeamDraftTasks(
	references TeamDraftReferences,
	input []TeamDraftTaskCandidate,
	limits TeamDraftContentLimits,
) ([]TeamDraftTaskCandidate, error) {
	if len(input) == 0 {
		return nil, ErrInvalidTeamDraftTask
	}
	if len(input) > limits.MaxTasks {
		return nil, ErrTeamDraftContentLimitExceeded
	}
	subAgents := stringSet(references.SubAgentDefinitionIDs)
	owners := make(map[string]struct{}, len(subAgents))
	taskIDs := make(map[string]struct{}, len(input))
	tasks := make([]TeamDraftTaskCandidate, 0, len(input))

	for _, task := range input {
		if task.ID == "" || task.OwnerAgentDefinitionID == "" ||
			len(task.AcceptanceCriteria) == 0 {
			return nil, ErrInvalidTeamDraftTask
		}
		if _, ok := taskIDs[task.ID]; ok {
			return nil, ErrDuplicateTeamDraftTask
		}
		taskIDs[task.ID] = struct{}{}
		if task.OwnerAgentDefinitionID == references.MainAgentDefinitionID {
			return nil, ErrMainAgentDeliveryAssignment
		}
		if _, ok := subAgents[task.OwnerAgentDefinitionID]; !ok {
			return nil, ErrInvalidTeamDraftTask
		}
		owners[task.OwnerAgentDefinitionID] = struct{}{}
		if len(task.DependencyTaskIDs) > limits.MaxDependenciesPerTask ||
			len(task.AcceptanceCriteria) > limits.MaxAcceptanceCriteriaPerTask {
			return nil, ErrTeamDraftContentLimitExceeded
		}
		dependencies, err := normalizeUniqueTaskValues(task.DependencyTaskIDs)
		if err != nil {
			return nil, err
		}
		for _, dependency := range dependencies {
			if dependency == task.ID {
				return nil, ErrTeamDraftTaskSelfDependency
			}
		}
		criteria, err := normalizeUniqueTaskValues(task.AcceptanceCriteria)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, TeamDraftTaskCandidate{
			ID:                     task.ID,
			OwnerAgentDefinitionID: task.OwnerAgentDefinitionID,
			DependencyTaskIDs:      dependencies,
			AcceptanceCriteria:     criteria,
		})
	}
	for subAgent := range subAgents {
		if _, ok := owners[subAgent]; !ok {
			return nil, ErrUnassignedTeamDraftSubAgent
		}
	}
	for _, task := range tasks {
		for _, dependency := range task.DependencyTaskIDs {
			if _, ok := taskIDs[dependency]; !ok {
				return nil, ErrUnknownTeamDraftTaskDependency
			}
		}
	}
	if teamDraftTasksContainCycle(tasks) {
		return nil, ErrCyclicTeamDraftTaskGraph
	}
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].ID < tasks[j].ID
	})
	return tasks, nil
}

func validateAndNormalizeApprovalMarkers(
	input []TeamDraftApprovalMarker,
	limits TeamDraftContentLimits,
) ([]TeamDraftApprovalMarker, error) {
	if len(input) > limits.MaxApprovalMarkers {
		return nil, ErrTeamDraftContentLimitExceeded
	}
	seen := make(map[string]struct{}, len(input))
	markers := append([]TeamDraftApprovalMarker(nil), input...)
	for _, marker := range markers {
		if marker.ID == "" || marker.Reason == "" {
			return nil, ErrInvalidTeamDraftApprovalMarker
		}
		if _, ok := seen[marker.ID]; ok {
			return nil, ErrDuplicateTeamDraftApprovalMarker
		}
		seen[marker.ID] = struct{}{}
	}
	sort.Slice(markers, func(i, j int) bool {
		return markers[i].ID < markers[j].ID
	})
	return markers, nil
}

func validateAndNormalizeCapabilityGaps(
	input []TeamDraftCapabilityGap,
	limits TeamDraftContentLimits,
) ([]TeamDraftCapabilityGap, error) {
	if len(input) > limits.MaxCapabilityGaps {
		return nil, ErrTeamDraftContentLimitExceeded
	}
	seen := make(map[string]struct{}, len(input))
	gaps := append([]TeamDraftCapabilityGap(nil), input...)
	for _, gap := range gaps {
		if gap.Capability == "" || gap.Reason == "" {
			return nil, ErrInvalidTeamDraftCapabilityGap
		}
		key := gap.Capability + "\x00" + gap.Reason
		if _, ok := seen[key]; ok {
			return nil, ErrDuplicateTeamDraftCapabilityGap
		}
		seen[key] = struct{}{}
	}
	sort.Slice(gaps, func(i, j int) bool {
		if gaps[i].Capability == gaps[j].Capability {
			return gaps[i].Reason < gaps[j].Reason
		}
		return gaps[i].Capability < gaps[j].Capability
	})
	return gaps, nil
}

func normalizeRoleReferenceSubset(input, allowed []string) ([]string, error) {
	allowedSet := stringSet(allowed)
	seen := make(map[string]struct{}, len(input))
	normalized := append([]string(nil), input...)
	for _, id := range normalized {
		if id == "" {
			return nil, ErrInvalidTeamDraftRole
		}
		if _, ok := seen[id]; ok {
			return nil, ErrInvalidTeamDraftRole
		}
		if _, ok := allowedSet[id]; !ok {
			return nil, ErrInvalidTeamDraftRole
		}
		seen[id] = struct{}{}
	}
	sort.Strings(normalized)
	return normalized, nil
}

func normalizeUniqueTaskValues(input []string) ([]string, error) {
	seen := make(map[string]struct{}, len(input))
	normalized := append([]string(nil), input...)
	for _, value := range normalized {
		if value == "" {
			return nil, ErrInvalidTeamDraftTask
		}
		if _, ok := seen[value]; ok {
			return nil, ErrInvalidTeamDraftTask
		}
		seen[value] = struct{}{}
	}
	sort.Strings(normalized)
	return normalized, nil
}

func teamDraftTasksContainCycle(tasks []TeamDraftTaskCandidate) bool {
	dependencies := make(map[string][]string, len(tasks))
	for _, task := range tasks {
		dependencies[task.ID] = task.DependencyTaskIDs
	}
	const (
		unvisited = iota
		visiting
		visited
	)
	state := make(map[string]int, len(tasks))
	var visit func(string) bool
	visit = func(id string) bool {
		switch state[id] {
		case visiting:
			return true
		case visited:
			return false
		}
		state[id] = visiting
		for _, dependency := range dependencies[id] {
			if visit(dependency) {
				return true
			}
		}
		state[id] = visited
		return false
	}
	for id := range dependencies {
		if visit(id) {
			return true
		}
	}
	return false
}

func digestTeamDraftContent(snapshot TeamDraftContentSnapshot) (string, error) {
	canonical := struct {
		CatalogDigest       string
		References          TeamDraftReferences
		Roles               []TeamDraftRoleSelection
		Tasks               []TeamDraftTaskCandidate
		CustomerRuleSummary string
		ApprovalMarkers     []TeamDraftApprovalMarker
		CapabilityGaps      []TeamDraftCapabilityGap
		Limits              TeamDraftContentLimits
	}{
		CatalogDigest:       snapshot.catalogDigest,
		References:          snapshot.References(),
		Roles:               snapshot.Roles(),
		Tasks:               snapshot.Tasks(),
		CustomerRuleSummary: snapshot.customerRuleSummary,
		ApprovalMarkers:     snapshot.ApprovalMarkers(),
		CapabilityGaps:      snapshot.CapabilityGaps(),
		Limits:              snapshot.limits,
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("%w: digest content: %v", ErrInvalidTeamDraftContent, err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func runtimeModelReferenceSet(input []RuntimeModelReference) map[string]struct{} {
	set := make(map[string]struct{}, len(input))
	for _, reference := range input {
		set[runtimeModelKey(reference.RuntimeInstanceID, reference.ModelID)] = struct{}{}
	}
	return set
}

func runtimeModelKey(runtimeInstanceID, modelID string) string {
	return runtimeInstanceID + "\x00" + modelID
}

func equalStringSets(left, right map[string]struct{}) bool {
	if len(left) != len(right) {
		return false
	}
	for value := range left {
		if _, ok := right[value]; !ok {
			return false
		}
	}
	return true
}

func copyTeamDraftRoles(input []TeamDraftRoleSelection) []TeamDraftRoleSelection {
	output := make([]TeamDraftRoleSelection, len(input))
	for i, role := range input {
		output[i] = TeamDraftRoleSelection{
			AgentDefinitionID: role.AgentDefinitionID,
			RuntimeProfile:    copyTeamDraftRuntimeProfile(role.RuntimeProfile),
			RuntimeInstanceID: role.RuntimeInstanceID,
			SkillIDs:          append([]string(nil), role.SkillIDs...),
			MemberIDs:         append([]string(nil), role.MemberIDs...),
			PermissionIDs:     append([]string(nil), role.PermissionIDs...),
		}
	}
	return output
}

func copyTeamDraftRuntimeProfile(input runtime.RuntimeProfile) runtime.RuntimeProfile {
	input.RequiredCapabilities = append([]string(nil), input.RequiredCapabilities...)
	if input.Budget != nil {
		budget := *input.Budget
		input.Budget = &budget
	}
	return input
}

func copyTeamDraftTasks(input []TeamDraftTaskCandidate) []TeamDraftTaskCandidate {
	output := make([]TeamDraftTaskCandidate, len(input))
	for i, task := range input {
		output[i] = TeamDraftTaskCandidate{
			ID:                     task.ID,
			OwnerAgentDefinitionID: task.OwnerAgentDefinitionID,
			DependencyTaskIDs:      append([]string(nil), task.DependencyTaskIDs...),
			AcceptanceCriteria:     append([]string(nil), task.AcceptanceCriteria...),
		}
	}
	return output
}

func cloneTeamDraftContentSnapshot(input TeamDraftContentSnapshot) TeamDraftContentSnapshot {
	input.references = copyTeamDraftReferences(input.references)
	input.roles = copyTeamDraftRoles(input.roles)
	input.tasks = copyTeamDraftTasks(input.tasks)
	input.approvalMarkers = append([]TeamDraftApprovalMarker(nil), input.approvalMarkers...)
	input.capabilityGaps = append([]TeamDraftCapabilityGap(nil), input.capabilityGaps...)
	return input
}
