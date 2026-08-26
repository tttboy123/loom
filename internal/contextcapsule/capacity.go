package contextcapsule

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
)

const (
	CapacitySchemaVersion = 1
	MaxCapacityTokens     = 10_000_000
)

var (
	ErrInvalidCapacityAuthority = errors.New("invalid Context Capsule capacity authority")
	ErrTokenCountMismatch       = errors.New("Context Capsule token count mismatch")
)

type CapacityStatus string

const (
	CapacityExact       CapacityStatus = "exact"
	CapacityEstimated   CapacityStatus = "estimated"
	CapacityUnavailable CapacityStatus = "unavailable"
)

// TokenCounter is invoked only for already bounded, dispatchable ItemInput
// content. Opaque credential references never enter the counter; their positive,
// schema-bounded TokenCount is treated as structural capacity metadata.
type TokenCounter interface {
	ID() string
	Version() string
	CountTokens(content []byte) (int, error)
}

// CapacityAuthority is the non-secret input authority for capacity resolution.
// An unavailable authority intentionally carries no context-window value.
type CapacityAuthority struct {
	SchemaVersion             int            `json:"schema_version"`
	Status                    CapacityStatus `json:"status"`
	ContextWindowTokens       int            `json:"context_window_tokens,omitempty"`
	ReservedOutputTokens      int            `json:"reserved_output_tokens"`
	AdapterToolOverheadTokens int            `json:"adapter_tool_overhead_tokens"`
	TokenCounterID            string         `json:"token_counter_id"`
	TokenCounterVersion       string         `json:"token_counter_version"`
}

// CapacityContribution contains aggregate counts only. It deliberately excludes
// item IDs, source references, content, route identities, and account identities.
type CapacityContribution struct {
	Priority                Priority   `json:"priority"`
	SourceType              SourceType `json:"source_type"`
	AdmittedItemCount       int        `json:"admitted_item_count"`
	AdmittedTokenCount      int        `json:"admitted_token_count"`
	BudgetOmittedItemCount  int        `json:"budget_omitted_item_count"`
	BudgetOmittedTokenCount int        `json:"budget_omitted_token_count"`
}

type CapacityProjection struct {
	SchemaVersion                   int                    `json:"schema_version"`
	Status                          CapacityStatus         `json:"status"`
	ContextWindowTokens             int                    `json:"context_window_tokens,omitempty"`
	ReservedOutputTokens            int                    `json:"reserved_output_tokens"`
	AdapterToolOverheadTokens       int                    `json:"adapter_tool_overhead_tokens"`
	PolicyInputBudgetTokens         int                    `json:"policy_input_budget_tokens"`
	AdmittedInputBudgetTokens       int                    `json:"admitted_input_budget_tokens"`
	TokenCounterID                  string                 `json:"token_counter_id"`
	TokenCounterVersion             string                 `json:"token_counter_version"`
	AdmittedContributionTokens      int                    `json:"admitted_contribution_tokens"`
	BudgetOmittedContributionTokens int                    `json:"budget_omitted_contribution_tokens"`
	Contributions                   []CapacityContribution `json:"contributions"`
}

func BuildRoleContextCapsuleWithCapacity(
	target Target,
	inputs []ItemInput,
	authority CapacityAuthority,
	counter TokenCounter,
) (RoleContextCapsule, error) {
	admittedBudget, err := resolveAdmittedInputBudget(target.TokenBudget, authority)
	if err != nil || !tokenCounterMatchesAuthority(counter, authority) {
		return RoleContextCapsule{}, ErrInvalidCapacityAuthority
	}
	preparedTarget, preparedInputs, artifacts, err := prepareRoleContextCapsuleCapacityInputs(
		target, inputs, admittedBudget,
	)
	if err != nil {
		return RoleContextCapsule{}, err
	}
	if err := validatePreCountCapacityTokens(preparedInputs); err != nil {
		return RoleContextCapsule{}, err
	}
	admitted := capacityInputAdmission(preparedTarget, preparedInputs, artifacts)
	preparedInputs, err = resolveCapacityTokenCounts(preparedInputs, admitted, counter)
	if err != nil {
		return RoleContextCapsule{}, err
	}
	if err := validateDeclaredCapacityTokenCounts(preparedInputs); err != nil {
		return RoleContextCapsule{}, err
	}
	if !tokenCounterMatchesAuthority(counter, authority) {
		return RoleContextCapsule{}, ErrInvalidCapacityAuthority
	}
	capsule, err := packRoleContextCapsule(
		preparedTarget, preparedInputs, artifacts, admittedBudget,
	)
	if err != nil {
		return RoleContextCapsule{}, err
	}
	projection := newCapacityProjection(authority, target.TokenBudget, admittedBudget, capsule)
	if !validCapacityProjectionForCapsule(projection, capsule) {
		return RoleContextCapsule{}, ErrInvalidCapacityAuthority
	}
	capsule.capacity = cloneCapacityProjection(&projection)
	if err := capsule.seal(); err != nil || !capsule.Valid() {
		return RoleContextCapsule{}, ErrInvalidCapsule
	}
	return capsule, nil
}

func validatePreCountCapacityTokens(inputs []ItemInput) error {
	totalTokens := 0
	for _, item := range inputs {
		if item.TokenCount < 0 ||
			item.TokenCount > 0 && !canAddCapacityTokens(totalTokens, item.TokenCount, true) {
			return ErrInvalidCapacityAuthority
		}
		totalTokens += item.TokenCount
	}
	return nil
}

func prepareRoleContextCapsuleCapacityInputs(
	target Target,
	inputs []ItemInput,
	admittedBudget int,
) (Target, []ItemInput, map[string]struct{}, error) {
	prepared := append([]ItemInput(nil), inputs...)
	autoCount := make(map[string]struct{})
	for index := range prepared {
		if prepared[index].TokenCount != 0 {
			continue
		}
		if len(prepared[index].Content) == 0 || opaqueCredentialReference(prepared[index]) {
			return Target{}, nil, nil, ErrInvalidCapsule
		}
		autoCount[prepared[index].ItemID] = struct{}{}
		prepared[index].TokenCount = 1
	}
	preparedTarget, preparedInputs, artifacts, err := prepareRoleContextCapsuleInputs(
		target, prepared, admittedBudget,
	)
	if err != nil {
		return Target{}, nil, nil, err
	}
	for index := range preparedInputs {
		if _, ok := autoCount[preparedInputs[index].ItemID]; ok {
			preparedInputs[index].TokenCount = 0
		}
	}
	return preparedTarget, preparedInputs, artifacts, nil
}

// ExtendRoleContextCapsuleWithCapacity deterministically reconstructs and
// repacks a capacity-bound Capsule. The supplied authority and TokenCounter
// identity must exactly match the authority frozen by the base Capsule.
func ExtendRoleContextCapsuleWithCapacity(
	base RoleContextCapsule,
	additions []ItemInput,
	authority CapacityAuthority,
	counter TokenCounter,
) (RoleContextCapsule, error) {
	if !base.Valid() || base.capacity == nil || len(additions) == 0 ||
		len(base.disclosed) > maxCapsuleItems ||
		len(base.omitted) > maxCapsuleItems-len(base.disclosed) {
		return RoleContextCapsule{}, ErrInvalidCapsule
	}
	baseItemCount := len(base.disclosed) + len(base.omitted)
	if len(additions) > maxCapsuleItems-baseItemCount {
		return RoleContextCapsule{}, ErrInvalidCapsule
	}
	frozenAuthority := capacityAuthorityFromProjection(*base.capacity)
	if authority != frozenAuthority || !tokenCounterMatchesAuthority(counter, frozenAuthority) {
		return RoleContextCapsule{}, ErrInvalidCapacityAuthority
	}
	items, fixedOmissions, err := extensionInputs(base, additions)
	if err != nil {
		return RoleContextCapsule{}, err
	}
	if err := validateExtensionDeclaredCapacityTokenCounts(items, fixedOmissions); err != nil {
		return RoleContextCapsule{}, err
	}
	rebuilt, err := BuildRoleContextCapsuleWithCapacity(base.Target(), items, authority, counter)
	if err != nil {
		return RoleContextCapsule{}, err
	}
	return finalizeExtendedCapsule(rebuilt, fixedOmissions)
}

func capacityInputAdmission(
	target Target,
	inputs []ItemInput,
	artifacts map[string]struct{},
) []bool {
	admitted := make([]bool, len(inputs))
	for index, item := range inputs {
		admitted[index] = !item.PolicyFiltered && scopeAllows(target, artifacts, item)
	}
	return admitted
}

func validateDeclaredCapacityTokenCounts(inputs []ItemInput) error {
	totalTokens := 0
	for _, item := range inputs {
		if !canAddCapacityTokens(totalTokens, item.TokenCount, true) {
			return ErrInvalidCapacityAuthority
		}
		totalTokens += item.TokenCount
	}
	return nil
}

func validateExtensionDeclaredCapacityTokenCounts(
	inputs []ItemInput,
	fixedOmissions []OmittedItem,
) error {
	totalTokens := 0
	for _, item := range inputs {
		if !canAddCapacityTokens(totalTokens, item.TokenCount, true) {
			return ErrInvalidCapacityAuthority
		}
		totalTokens += item.TokenCount
	}
	for _, omission := range fixedOmissions {
		if !canAddCapacityTokens(totalTokens, omission.TokenCount, true) {
			return ErrInvalidCapacityAuthority
		}
		totalTokens += omission.TokenCount
	}
	return nil
}

func resolveCapacityTokenCounts(
	inputs []ItemInput,
	admitted []bool,
	counter TokenCounter,
) ([]ItemInput, error) {
	if len(admitted) != len(inputs) {
		return nil, ErrInvalidCapacityAuthority
	}
	resolved := append([]ItemInput(nil), inputs...)
	totalTokens := 0
	for index, item := range resolved {
		count := item.TokenCount
		// Policy-filtered and scope-denied bodies are not dispatchable. Their
		// declared structural cost remains in omission metadata, but their bytes
		// must never cross a tokenizer or adapter boundary.
		if admitted[index] && !opaqueCredentialReference(item) {
			var countErr error
			count, countErr = counter.CountTokens(append([]byte(nil), item.Content...))
			if countErr != nil {
				return nil, fmt.Errorf("%w: token counter failed: %v", ErrInvalidCapacityAuthority, countErr)
			}
		}
		if !canAddCapacityTokens(totalTokens, count, true) {
			return nil, ErrInvalidCapacityAuthority
		}
		totalTokens += count
		if item.TokenCount != 0 && count != item.TokenCount {
			return nil, ErrTokenCountMismatch
		}
		resolved[index].TokenCount = count
	}
	return resolved, nil
}

func opaqueCredentialReference(item ItemInput) bool {
	return item.Scope == ScopeSecretReferenceOnly &&
		item.Kind == KindCredentialReference &&
		item.SourceType == SourceCredentialReference &&
		len(item.Content) == 0
}

func capacityAuthorityFromProjection(projection CapacityProjection) CapacityAuthority {
	return CapacityAuthority{
		SchemaVersion:             projection.SchemaVersion,
		Status:                    projection.Status,
		ContextWindowTokens:       projection.ContextWindowTokens,
		ReservedOutputTokens:      projection.ReservedOutputTokens,
		AdapterToolOverheadTokens: projection.AdapterToolOverheadTokens,
		TokenCounterID:            projection.TokenCounterID,
		TokenCounterVersion:       projection.TokenCounterVersion,
	}
}

func ResolveAdmittedInputBudget(policyBudget int, authority CapacityAuthority) (int, error) {
	return resolveAdmittedInputBudget(policyBudget, authority)
}

func resolveAdmittedInputBudget(policyBudget int, authority CapacityAuthority) (int, error) {
	if policyBudget < 1 || policyBudget > maxTokenBudget || !validCapacityAuthority(authority) {
		return 0, ErrInvalidCapacityAuthority
	}
	if authority.Status == CapacityUnavailable {
		return policyBudget, nil
	}
	available := authority.ContextWindowTokens - authority.ReservedOutputTokens
	if available <= authority.AdapterToolOverheadTokens {
		return 0, ErrInvalidCapacityAuthority
	}
	available -= authority.AdapterToolOverheadTokens
	if available < policyBudget {
		return available, nil
	}
	return policyBudget, nil
}

func validCapacityAuthority(authority CapacityAuthority) bool {
	if authority.SchemaVersion != CapacitySchemaVersion ||
		authority.ReservedOutputTokens < 0 || authority.ReservedOutputTokens > MaxCapacityTokens ||
		authority.AdapterToolOverheadTokens < 0 || authority.AdapterToolOverheadTokens > MaxCapacityTokens ||
		authority.ReservedOutputTokens > MaxCapacityTokens-authority.AdapterToolOverheadTokens ||
		!validIdentifier(authority.TokenCounterID) || !validIdentifier(authority.TokenCounterVersion) {
		return false
	}
	switch authority.Status {
	case CapacityExact, CapacityEstimated:
		return authority.ContextWindowTokens > 0 && authority.ContextWindowTokens <= MaxCapacityTokens &&
			authority.ReservedOutputTokens < authority.ContextWindowTokens &&
			authority.AdapterToolOverheadTokens < authority.ContextWindowTokens-authority.ReservedOutputTokens
	case CapacityUnavailable:
		return authority.ContextWindowTokens == 0
	default:
		return false
	}
}

func nilTokenCounter(counter TokenCounter) bool {
	if counter == nil {
		return true
	}
	value := reflect.ValueOf(counter)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func tokenCounterMatchesAuthority(counter TokenCounter, authority CapacityAuthority) bool {
	return !nilTokenCounter(counter) && counter.ID() == authority.TokenCounterID &&
		counter.Version() == authority.TokenCounterVersion
}

func canAddCapacityTokens(total int, count int, requirePositive bool) bool {
	if total < 0 || total > MaxCapacityTokens || count < 0 || count > MaxCapacityTokens ||
		requirePositive && count == 0 {
		return false
	}
	return total <= MaxCapacityTokens-count
}

type capacityContributionKey struct {
	priority   Priority
	sourceType SourceType
}

func newCapacityProjection(
	authority CapacityAuthority,
	policyBudget int,
	admittedBudget int,
	capsule RoleContextCapsule,
) CapacityProjection {
	projection := CapacityProjection{
		SchemaVersion: CapacitySchemaVersion, Status: authority.Status,
		ContextWindowTokens:       authority.ContextWindowTokens,
		ReservedOutputTokens:      authority.ReservedOutputTokens,
		AdapterToolOverheadTokens: authority.AdapterToolOverheadTokens,
		PolicyInputBudgetTokens:   policyBudget, AdmittedInputBudgetTokens: admittedBudget,
		TokenCounterID: authority.TokenCounterID, TokenCounterVersion: authority.TokenCounterVersion,
		Contributions: capacityContributions(capsule),
	}
	for _, contribution := range projection.Contributions {
		projection.AdmittedContributionTokens += contribution.AdmittedTokenCount
		projection.BudgetOmittedContributionTokens += contribution.BudgetOmittedTokenCount
	}
	return projection
}

func capacityContributions(capsule RoleContextCapsule) []CapacityContribution {
	byKey := make(map[capacityContributionKey]CapacityContribution)
	for _, item := range capsule.disclosed {
		key := capacityContributionKey{priority: item.Priority, sourceType: item.SourceType}
		contribution := byKey[key]
		contribution.Priority, contribution.SourceType = key.priority, key.sourceType
		contribution.AdmittedItemCount++
		contribution.AdmittedTokenCount += item.TokenCount
		byKey[key] = contribution
	}
	for _, item := range capsule.omitted {
		if item.Reason != OmissionBudgetExceeded {
			continue
		}
		key := capacityContributionKey{priority: item.Priority, sourceType: item.SourceType}
		contribution := byKey[key]
		contribution.Priority, contribution.SourceType = key.priority, key.sourceType
		contribution.BudgetOmittedItemCount++
		contribution.BudgetOmittedTokenCount += item.TokenCount
		byKey[key] = contribution
	}
	contributions := make([]CapacityContribution, 0, len(byKey))
	for _, contribution := range byKey {
		contributions = append(contributions, contribution)
	}
	sort.Slice(contributions, func(i, j int) bool {
		if contributions[i].Priority != contributions[j].Priority {
			return contributions[i].Priority < contributions[j].Priority
		}
		return contributions[i].SourceType < contributions[j].SourceType
	})
	return contributions
}

func validCapacityProjectionForCapsule(projection CapacityProjection, capsule RoleContextCapsule) bool {
	if !validCapacityProjection(projection) || projection.PolicyInputBudgetTokens != capsule.target.TokenBudget ||
		projection.AdmittedContributionTokens != capsule.tokenCount ||
		!validCapsuleCapacityTokenTotal(capsule) ||
		!reflect.DeepEqual(projection.Contributions, capacityContributions(capsule)) {
		return false
	}
	return true
}

func validCapacityProjection(projection CapacityProjection) bool {
	authority := CapacityAuthority{
		SchemaVersion: projection.SchemaVersion, Status: projection.Status,
		ContextWindowTokens:       projection.ContextWindowTokens,
		ReservedOutputTokens:      projection.ReservedOutputTokens,
		AdapterToolOverheadTokens: projection.AdapterToolOverheadTokens,
		TokenCounterID:            projection.TokenCounterID, TokenCounterVersion: projection.TokenCounterVersion,
	}
	resolved, err := resolveAdmittedInputBudget(projection.PolicyInputBudgetTokens, authority)
	if err != nil || resolved != projection.AdmittedInputBudgetTokens || len(projection.Contributions) == 0 ||
		len(projection.Contributions) > maxCapsuleItems || projection.AdmittedContributionTokens < 1 ||
		projection.AdmittedContributionTokens > projection.AdmittedInputBudgetTokens ||
		projection.BudgetOmittedContributionTokens < 0 ||
		projection.BudgetOmittedContributionTokens > MaxCapacityTokens ||
		!canAddCapacityTokens(
			projection.AdmittedContributionTokens,
			projection.BudgetOmittedContributionTokens,
			false,
		) {
		return false
	}
	admittedItems, omittedItems := 0, 0
	admittedTokens, omittedTokens := 0, 0
	previous := capacityContributionKey{priority: PrioritySystem}
	for index, contribution := range projection.Contributions {
		key := capacityContributionKey{priority: contribution.Priority, sourceType: contribution.SourceType}
		if contribution.Priority < PrioritySystem || contribution.Priority > PriorityRetrievable ||
			!validContributionSourceType(contribution.SourceType) ||
			contribution.AdmittedItemCount < 0 || contribution.BudgetOmittedItemCount < 0 ||
			contribution.AdmittedItemCount > maxCapsuleItems || contribution.BudgetOmittedItemCount > maxCapsuleItems ||
			contribution.AdmittedItemCount > maxCapsuleItems-contribution.BudgetOmittedItemCount ||
			contribution.AdmittedItemCount+contribution.BudgetOmittedItemCount < 1 ||
			contribution.AdmittedTokenCount < 0 || contribution.BudgetOmittedTokenCount < 0 ||
			contribution.AdmittedTokenCount < contribution.AdmittedItemCount ||
			contribution.BudgetOmittedTokenCount < contribution.BudgetOmittedItemCount ||
			contribution.AdmittedItemCount == 0 && contribution.AdmittedTokenCount != 0 ||
			contribution.BudgetOmittedItemCount == 0 && contribution.BudgetOmittedTokenCount != 0 ||
			index > 0 && (key.priority < previous.priority ||
				key.priority == previous.priority && key.sourceType <= previous.sourceType) ||
			!canAddCapacityTokens(admittedTokens, contribution.AdmittedTokenCount, false) ||
			!canAddCapacityTokens(omittedTokens, contribution.BudgetOmittedTokenCount, false) {
			return false
		}
		admittedItems += contribution.AdmittedItemCount
		omittedItems += contribution.BudgetOmittedItemCount
		if admittedItems > maxCapsuleItems-omittedItems {
			return false
		}
		admittedTokens += contribution.AdmittedTokenCount
		omittedTokens += contribution.BudgetOmittedTokenCount
		previous = key
	}
	return admittedTokens == projection.AdmittedContributionTokens &&
		omittedTokens == projection.BudgetOmittedContributionTokens
}

func validCapsuleCapacityTokenTotal(capsule RoleContextCapsule) bool {
	totalTokens := 0
	for _, item := range capsule.disclosed {
		if !canAddCapacityTokens(totalTokens, item.TokenCount, true) {
			return false
		}
		totalTokens += item.TokenCount
	}
	for _, item := range capsule.omitted {
		if !canAddCapacityTokens(totalTokens, item.TokenCount, true) {
			return false
		}
		totalTokens += item.TokenCount
	}
	return true
}

func validContributionSourceType(sourceType SourceType) bool {
	switch sourceType {
	case SourceAuthority, SourceObservation, SourceModelOutput, SourceCredentialReference:
		return true
	default:
		return false
	}
}

func cloneCapacityProjection(projection *CapacityProjection) *CapacityProjection {
	if projection == nil {
		return nil
	}
	cloned := *projection
	cloned.Contributions = append([]CapacityContribution(nil), projection.Contributions...)
	return &cloned
}

func MarshalCanonicalCapacityProjection(projection CapacityProjection) ([]byte, error) {
	if !validCapacityProjection(projection) {
		return nil, ErrInvalidCapacityAuthority
	}
	body, err := json.Marshal(projection)
	if err != nil || len(body) == 0 || len(body) > maxItemContentBytes {
		return nil, ErrInvalidCapacityAuthority
	}
	return body, nil
}

func capacityProjectionDigest(projection *CapacityProjection) string {
	if projection == nil {
		return ""
	}
	body, err := MarshalCanonicalCapacityProjection(*projection)
	if err != nil {
		return ""
	}
	return digestBytes(body)
}
