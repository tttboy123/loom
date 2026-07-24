package teams

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"loom-pi-rebuild/internal/agents"
	"loom-pi-rebuild/internal/runtime"
)

var (
	ErrInvalidTeamDraftCatalog       = errors.New("invalid team draft catalog")
	ErrTeamDraftCatalogLimitExceeded = errors.New("team draft catalog limit exceeded")
	ErrInvalidTeamDraftReference     = errors.New("invalid team draft reference")
	ErrDuplicateTeamDraftReference   = errors.New("duplicate team draft reference")
	ErrInventedTeamDraftReference    = errors.New("invented team draft reference")
	ErrRuntimeModelPairMismatch      = errors.New("runtime model pair mismatch")
	ErrBudgetCeilingExceeded         = errors.New("budget ceiling exceeded")
	ErrConcurrencyCeilingExceeded    = errors.New("concurrency ceiling exceeded")
)

type TeamDraftCatalogMaxCounts struct {
	Agents      int
	Runtimes    int
	Models      int
	Skills      int
	Members     int
	Permissions int
}

type TeamDraftCatalogInput struct {
	AgentDefinitions   []agents.AgentDefinition
	ResolutionContext  agents.ResolutionContext
	RuntimeDiscovery   runtime.RuntimeDiscoverySnapshot
	SkillIDs           []string
	MemberIDs          []string
	PermissionIDs      []string
	BudgetCeiling      int
	ConcurrencyCeiling int
	MaxCounts          TeamDraftCatalogMaxCounts
}

type AgentCatalogEntry struct {
	ID            string
	Version       int
	Scope         agents.Scope
	ScopeIdentity agents.ScopeIdentity
	Name          string
	RoleSpec      string
	Status        agents.DefinitionStatus
}

type RuntimeCatalogEntry struct {
	ID                   string
	DeviceID             string
	AdapterType          string
	DisplayName          string
	ExecutableVersion    string
	Status               runtime.RuntimeStatus
	ObservedCapabilities []string
	Capacity             int
	ModelIDs             []string
	SourceProbeID        string
}

type TeamDraftCatalogSnapshot struct {
	digest                 string
	runtimeDiscoveryDigest string
	agents                 []AgentCatalogEntry
	runtimes               []RuntimeCatalogEntry
	skills                 []string
	members                []string
	permissions            []string
	budgetCeiling          int
	concurrencyCeiling     int
	maxCounts              TeamDraftCatalogMaxCounts
}

type RuntimeModelReference struct {
	RuntimeInstanceID string
	ModelID           string
}

type TeamDraftReferences struct {
	MainAgentDefinitionID string
	SubAgentDefinitionIDs []string
	RuntimeInstanceIDs    []string
	RuntimeModels         []RuntimeModelReference
	SkillIDs              []string
	MemberIDs             []string
	PermissionIDs         []string
	RequestedBudget       int
	RequestedConcurrency  int
}

type TeamDraftCatalogValidationCandidate struct {
	Accepted              bool
	CatalogDigest         string
	MainAgentDefinitionID string
}

func BuildTeamDraftCatalog(input TeamDraftCatalogInput) (TeamDraftCatalogSnapshot, error) {
	if err := validateCatalogCeilings(input); err != nil {
		return TeamDraftCatalogSnapshot{}, err
	}
	if input.RuntimeDiscovery.Digest() == "" {
		return TeamDraftCatalogSnapshot{}, fmt.Errorf("%w: zero runtime discovery snapshot", ErrInvalidTeamDraftCatalog)
	}

	skills, err := normalizeUniqueCatalogIDs(input.SkillIDs)
	if err != nil {
		return TeamDraftCatalogSnapshot{}, err
	}
	members, err := normalizeUniqueCatalogIDs(input.MemberIDs)
	if err != nil {
		return TeamDraftCatalogSnapshot{}, err
	}
	permissions, err := normalizeUniqueCatalogIDs(input.PermissionIDs)
	if err != nil {
		return TeamDraftCatalogSnapshot{}, err
	}

	agentEntries, err := buildAgentCatalog(input.AgentDefinitions, input.ResolutionContext)
	if err != nil {
		return TeamDraftCatalogSnapshot{}, err
	}
	runtimeEntries := buildRuntimeCatalog(input.RuntimeDiscovery.Observations())

	modelCount := 0
	for _, entry := range runtimeEntries {
		modelCount += len(entry.ModelIDs)
	}
	if len(agentEntries) > input.MaxCounts.Agents ||
		len(runtimeEntries) > input.MaxCounts.Runtimes ||
		modelCount > input.MaxCounts.Models ||
		len(skills) > input.MaxCounts.Skills ||
		len(members) > input.MaxCounts.Members ||
		len(permissions) > input.MaxCounts.Permissions {
		return TeamDraftCatalogSnapshot{}, ErrTeamDraftCatalogLimitExceeded
	}

	snapshot := TeamDraftCatalogSnapshot{
		runtimeDiscoveryDigest: input.RuntimeDiscovery.Digest(),
		agents:                 copyAgentCatalogEntries(agentEntries),
		runtimes:               copyRuntimeCatalogEntries(runtimeEntries),
		skills:                 append([]string(nil), skills...),
		members:                append([]string(nil), members...),
		permissions:            append([]string(nil), permissions...),
		budgetCeiling:          input.BudgetCeiling,
		concurrencyCeiling:     input.ConcurrencyCeiling,
		maxCounts:              input.MaxCounts,
	}
	digest, err := digestTeamDraftCatalog(snapshot)
	if err != nil {
		return TeamDraftCatalogSnapshot{}, err
	}
	snapshot.digest = digest
	return snapshot, nil
}

func (s TeamDraftCatalogSnapshot) Digest() string {
	return s.digest
}

func (s TeamDraftCatalogSnapshot) RuntimeDiscoveryDigest() string {
	return s.runtimeDiscoveryDigest
}

func (s TeamDraftCatalogSnapshot) Agents() []AgentCatalogEntry {
	return copyAgentCatalogEntries(s.agents)
}

func (s TeamDraftCatalogSnapshot) Runtimes() []RuntimeCatalogEntry {
	return copyRuntimeCatalogEntries(s.runtimes)
}

func (s TeamDraftCatalogSnapshot) Skills() []string {
	return append([]string(nil), s.skills...)
}

func (s TeamDraftCatalogSnapshot) Members() []string {
	return append([]string(nil), s.members...)
}

func (s TeamDraftCatalogSnapshot) Permissions() []string {
	return append([]string(nil), s.permissions...)
}

func (s TeamDraftCatalogSnapshot) BudgetCeiling() int {
	return s.budgetCeiling
}

func (s TeamDraftCatalogSnapshot) ConcurrencyCeiling() int {
	return s.concurrencyCeiling
}

func (s TeamDraftCatalogSnapshot) MaxCounts() TeamDraftCatalogMaxCounts {
	return s.maxCounts
}

func ValidateTeamDraftCatalog(snapshot TeamDraftCatalogSnapshot, refs TeamDraftReferences) (TeamDraftCatalogValidationCandidate, error) {
	if snapshot.digest == "" {
		return TeamDraftCatalogValidationCandidate{}, ErrInvalidTeamDraftCatalog
	}

	agentsByID := make(map[string]struct{}, len(snapshot.agents))
	for _, entry := range snapshot.agents {
		agentsByID[entry.ID] = struct{}{}
	}
	if refs.MainAgentDefinitionID == "" {
		return TeamDraftCatalogValidationCandidate{}, ErrInvalidTeamDraftReference
	}
	if _, ok := agentsByID[refs.MainAgentDefinitionID]; !ok {
		return TeamDraftCatalogValidationCandidate{}, ErrInventedTeamDraftReference
	}
	if err := validateAgentReferences(refs, agentsByID); err != nil {
		return TeamDraftCatalogValidationCandidate{}, err
	}

	runtimesByID := make(map[string]RuntimeCatalogEntry, len(snapshot.runtimes))
	for _, entry := range snapshot.runtimes {
		runtimesByID[entry.ID] = entry
	}
	referencedRuntimes, err := validateRuntimeReferences(refs.RuntimeInstanceIDs, runtimesByID)
	if err != nil {
		return TeamDraftCatalogValidationCandidate{}, err
	}
	if err := validateRuntimeModelReferences(refs.RuntimeModels, referencedRuntimes, runtimesByID); err != nil {
		return TeamDraftCatalogValidationCandidate{}, err
	}
	if err := validateCatalogReferenceSet(refs.SkillIDs, stringSet(snapshot.skills)); err != nil {
		return TeamDraftCatalogValidationCandidate{}, err
	}
	if err := validateCatalogReferenceSet(refs.MemberIDs, stringSet(snapshot.members)); err != nil {
		return TeamDraftCatalogValidationCandidate{}, err
	}
	if err := validateCatalogReferenceSet(refs.PermissionIDs, stringSet(snapshot.permissions)); err != nil {
		return TeamDraftCatalogValidationCandidate{}, err
	}
	if refs.RequestedBudget < 0 {
		return TeamDraftCatalogValidationCandidate{}, ErrInvalidTeamDraftReference
	}
	if refs.RequestedBudget > snapshot.budgetCeiling {
		return TeamDraftCatalogValidationCandidate{}, ErrBudgetCeilingExceeded
	}
	if refs.RequestedConcurrency <= 0 {
		return TeamDraftCatalogValidationCandidate{}, ErrInvalidTeamDraftReference
	}
	if refs.RequestedConcurrency > snapshot.concurrencyCeiling {
		return TeamDraftCatalogValidationCandidate{}, ErrConcurrencyCeilingExceeded
	}

	return TeamDraftCatalogValidationCandidate{
		Accepted:              true,
		CatalogDigest:         snapshot.digest,
		MainAgentDefinitionID: refs.MainAgentDefinitionID,
	}, nil
}

type agentDefinitionKey struct {
	id           string
	version      int
	scope        agents.Scope
	projectID    string
	generationID string
}

func buildAgentCatalog(inputs []agents.AgentDefinition, context agents.ResolutionContext) ([]AgentCatalogEntry, error) {
	validated := make([]agents.AgentDefinition, 0, len(inputs))
	seenIdentity := make(map[agentDefinitionKey]struct{}, len(inputs))
	stableIDs := make(map[string]struct{}, len(inputs))

	for _, input := range inputs {
		definition, err := agents.NewAgentDefinition(input)
		if err != nil {
			return nil, err
		}
		key := agentDefinitionKey{
			id:           definition.ID,
			version:      definition.Version,
			scope:        definition.Scope,
			projectID:    definition.ScopeIdentity.ProjectID,
			generationID: definition.ScopeIdentity.GenerationID,
		}
		if _, ok := seenIdentity[key]; ok {
			return nil, fmt.Errorf("%w: %s", agents.ErrDuplicateAgentDefinition, definition.ID)
		}
		seenIdentity[key] = struct{}{}
		stableIDs[definition.ID] = struct{}{}
		validated = append(validated, definition)
	}

	ids := make([]string, 0, len(stableIDs))
	for id := range stableIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	entries := make([]AgentCatalogEntry, 0, len(ids))
	for _, id := range ids {
		resolutionContext := context
		resolutionContext.DefinitionID = id
		selected, err := agents.ResolveDefinition(validated, resolutionContext)
		if errors.Is(err, agents.ErrAgentDefinitionNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		entries = append(entries, AgentCatalogEntry{
			ID:            selected.ID,
			Version:       selected.Version,
			Scope:         selected.Scope,
			ScopeIdentity: selected.ScopeIdentity,
			Name:          selected.Name,
			RoleSpec:      selected.RoleSpec,
			Status:        selected.Status,
		})
	}
	return entries, nil
}

func buildRuntimeCatalog(observations []runtime.RuntimeObservation) []RuntimeCatalogEntry {
	entries := make([]RuntimeCatalogEntry, 0, len(observations))
	for _, observation := range observations {
		if observation.Instance.Status != runtime.RuntimeOnline {
			continue
		}
		entries = append(entries, RuntimeCatalogEntry{
			ID:                   observation.Instance.ID,
			DeviceID:             observation.Instance.DeviceID,
			AdapterType:          observation.Instance.AdapterType,
			DisplayName:          observation.Instance.DisplayName,
			ExecutableVersion:    observation.Instance.ExecutableVersion,
			Status:               observation.Instance.Status,
			ObservedCapabilities: append([]string(nil), observation.Instance.ObservedCapabilities...),
			Capacity:             observation.Instance.Capacity,
			ModelIDs:             append([]string(nil), observation.ModelIDs...),
			SourceProbeID:        observation.SourceProbeID,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ID < entries[j].ID
	})
	return entries
}

func validateCatalogCeilings(input TeamDraftCatalogInput) error {
	if input.BudgetCeiling < 0 {
		return fmt.Errorf("%w: negative budget ceiling", ErrInvalidTeamDraftCatalog)
	}
	if input.ConcurrencyCeiling <= 0 {
		return fmt.Errorf("%w: nonpositive concurrency ceiling", ErrInvalidTeamDraftCatalog)
	}
	if input.MaxCounts.Agents <= 0 ||
		input.MaxCounts.Runtimes <= 0 ||
		input.MaxCounts.Models <= 0 ||
		input.MaxCounts.Skills <= 0 ||
		input.MaxCounts.Members <= 0 ||
		input.MaxCounts.Permissions <= 0 {
		return fmt.Errorf("%w: nonpositive max count", ErrInvalidTeamDraftCatalog)
	}
	return nil
}

func normalizeUniqueCatalogIDs(input []string) ([]string, error) {
	ids := append([]string(nil), input...)
	sort.Strings(ids)
	for i, id := range ids {
		if id == "" {
			return nil, fmt.Errorf("%w: empty id", ErrInvalidTeamDraftCatalog)
		}
		if i > 0 && id == ids[i-1] {
			return nil, fmt.Errorf("%w: duplicate id %q", ErrInvalidTeamDraftCatalog, id)
		}
	}
	return ids, nil
}

func validateAgentReferences(refs TeamDraftReferences, agentsByID map[string]struct{}) error {
	seen := map[string]struct{}{refs.MainAgentDefinitionID: {}}
	for _, id := range refs.SubAgentDefinitionIDs {
		if id == "" {
			return ErrInvalidTeamDraftReference
		}
		if _, ok := seen[id]; ok {
			return ErrDuplicateTeamDraftReference
		}
		seen[id] = struct{}{}
		if _, ok := agentsByID[id]; !ok {
			return ErrInventedTeamDraftReference
		}
	}
	return nil
}

func validateRuntimeReferences(ids []string, runtimesByID map[string]RuntimeCatalogEntry) (map[string]struct{}, error) {
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			return nil, ErrInvalidTeamDraftReference
		}
		if _, ok := seen[id]; ok {
			return nil, ErrDuplicateTeamDraftReference
		}
		if _, ok := runtimesByID[id]; !ok {
			return nil, ErrInventedTeamDraftReference
		}
		seen[id] = struct{}{}
	}
	return seen, nil
}

func validateRuntimeModelReferences(refs []RuntimeModelReference, referencedRuntimes map[string]struct{}, runtimesByID map[string]RuntimeCatalogEntry) error {
	seen := make(map[RuntimeModelReference]struct{}, len(refs))
	for _, ref := range refs {
		if ref.RuntimeInstanceID == "" || ref.ModelID == "" {
			return ErrInvalidTeamDraftReference
		}
		if _, ok := seen[ref]; ok {
			return ErrDuplicateTeamDraftReference
		}
		seen[ref] = struct{}{}

		runtimeEntry, runtimeExists := runtimesByID[ref.RuntimeInstanceID]
		if !runtimeExists {
			return ErrInventedTeamDraftReference
		}
		if _, ok := referencedRuntimes[ref.RuntimeInstanceID]; !ok {
			return ErrInventedTeamDraftReference
		}
		if stringSliceContains(runtimeEntry.ModelIDs, ref.ModelID) {
			continue
		}
		for _, candidate := range runtimesByID {
			if candidate.ID != ref.RuntimeInstanceID && stringSliceContains(candidate.ModelIDs, ref.ModelID) {
				return ErrRuntimeModelPairMismatch
			}
		}
		return ErrInventedTeamDraftReference
	}
	return nil
}

func validateCatalogReferenceSet(refs []string, allowed map[string]struct{}) error {
	seen := make(map[string]struct{}, len(refs))
	for _, id := range refs {
		if id == "" {
			return ErrInvalidTeamDraftReference
		}
		if _, ok := seen[id]; ok {
			return ErrDuplicateTeamDraftReference
		}
		seen[id] = struct{}{}
		if _, ok := allowed[id]; !ok {
			return ErrInventedTeamDraftReference
		}
	}
	return nil
}

func stringSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

func stringSliceContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func digestTeamDraftCatalog(snapshot TeamDraftCatalogSnapshot) (string, error) {
	canonical := teamDraftCatalogCanonical{
		RuntimeDiscoveryDigest: snapshot.runtimeDiscoveryDigest,
		Agents:                 copyAgentCatalogEntries(snapshot.agents),
		Runtimes:               copyRuntimeCatalogEntries(snapshot.runtimes),
		Skills:                 append([]string(nil), snapshot.skills...),
		Members:                append([]string(nil), snapshot.members...),
		Permissions:            append([]string(nil), snapshot.permissions...),
		BudgetCeiling:          snapshot.budgetCeiling,
		ConcurrencyCeiling:     snapshot.concurrencyCeiling,
		MaxCounts:              snapshot.maxCounts,
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("%w: digest encoding: %w", ErrInvalidTeamDraftCatalog, err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

type teamDraftCatalogCanonical struct {
	RuntimeDiscoveryDigest string                    `json:"runtime_discovery_digest"`
	Agents                 []AgentCatalogEntry       `json:"agents"`
	Runtimes               []RuntimeCatalogEntry     `json:"runtimes"`
	Skills                 []string                  `json:"skills"`
	Members                []string                  `json:"members"`
	Permissions            []string                  `json:"permissions"`
	BudgetCeiling          int                       `json:"budget_ceiling"`
	ConcurrencyCeiling     int                       `json:"concurrency_ceiling"`
	MaxCounts              TeamDraftCatalogMaxCounts `json:"max_counts"`
}

func copyAgentCatalogEntries(entries []AgentCatalogEntry) []AgentCatalogEntry {
	if len(entries) == 0 {
		return []AgentCatalogEntry{}
	}
	copied := make([]AgentCatalogEntry, len(entries))
	copy(copied, entries)
	return copied
}

func copyRuntimeCatalogEntries(entries []RuntimeCatalogEntry) []RuntimeCatalogEntry {
	if len(entries) == 0 {
		return []RuntimeCatalogEntry{}
	}
	copied := make([]RuntimeCatalogEntry, len(entries))
	for i, entry := range entries {
		copied[i] = entry
		copied[i].ObservedCapabilities = append([]string(nil), entry.ObservedCapabilities...)
		copied[i].ModelIDs = append([]string(nil), entry.ModelIDs...)
	}
	return copied
}
