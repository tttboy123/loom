package contextcapsule

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"
)

const (
	schemaVersion        = 1
	maxCapsuleItems      = 512
	maxItemContentBytes  = 1 << 20
	maxTotalContentBytes = 8 << 20
	maxTokenBudget       = 1_000_000
)

var (
	ErrInvalidCapsule            = errors.New("invalid Role Context Capsule")
	ErrRequiredContextOmitted    = errors.New("required Role Context Capsule item omitted")
	ErrContextItemNotRetrievable = errors.New("Role Context Capsule item is not retrievable")
	ErrContextRetrievalDenied    = errors.New("Role Context Capsule retrieval denied")
)

type TrustClass string

const (
	TrustAuthoritative TrustClass = "authoritative"
	TrustObserved      TrustClass = "observed"
	TrustUntrusted     TrustClass = "untrusted"
)

type Scope string

const (
	ScopeConversationShared  Scope = "conversation_shared"
	ScopeTeamShared          Scope = "team_shared"
	ScopeAgentPrivate        Scope = "agent_private"
	ScopeRoleRestricted      Scope = "role_restricted"
	ScopeArtifactScoped      Scope = "artifact_scoped"
	ScopeSecretReferenceOnly Scope = "secret_reference_only"
)

type Priority int

const (
	PrioritySystem Priority = iota
	PriorityConfirmed
	PriorityWorkspace
	PriorityHistory
	PriorityRetrievable
)

type ItemKind string

const (
	KindConversationGoal       ItemKind = "conversation_goal"
	KindCurrentTaskState       ItemKind = "current_task_state"
	KindSystemPolicy           ItemKind = "system_policy"
	KindAcceptedDecision       ItemKind = "accepted_decision"
	KindArtifactReference      ItemKind = "artifact_reference"
	KindConfirmedConstraint    ItemKind = "confirmed_user_constraint"
	KindWorkspaceSnapshot      ItemKind = "workspace_snapshot"
	KindGovernanceState        ItemKind = "team_governance_state"
	KindRecentUserTurn         ItemKind = "recent_user_turn"
	KindUnresolvedQuestion     ItemKind = "unresolved_question"
	KindPriorModelOutput       ItemKind = "untrusted_model_output"
	KindAggregationSource      ItemKind = "aggregation_source_authority"
	KindDependencySource       ItemKind = "dependency_source_authority"
	KindObservedExecutionState ItemKind = "observed_execution_state"
	KindCredentialReference    ItemKind = "credential_reference"
)

type SourceType string

const (
	SourceAuthority           SourceType = "authority"
	SourceObservation         SourceType = "observation"
	SourceModelOutput         SourceType = "model_output"
	SourceCredentialReference SourceType = "credential_reference"
)

type OmissionReason string

const (
	OmissionAccessDenied   OmissionReason = "access_denied"
	OmissionBudgetExceeded OmissionReason = "budget_exceeded"
	OmissionPolicyFiltered OmissionReason = "policy_filtered"
)

type Target struct {
	ConversationID          string
	TeamID                  string
	AgentID                 string
	RoleID                  string
	ProviderID              string
	ProviderAccountID       string
	ModelID                 string
	AuthMode                string
	ContextAdapterID        string
	DisclosurePolicyID      string
	DisclosurePolicyVersion int
	ArtifactRefs            []string
	TokenBudget             int
}

type ItemInput struct {
	ItemID         string
	Kind           ItemKind
	Trust          TrustClass
	Scope          Scope
	Priority       Priority
	TokenCount     int
	Required       bool
	Content        []byte
	ReferenceID    string
	SourceType     SourceType
	SourceRef      string
	AllowedAgentID string
	AllowedRoleID  string
	ArtifactRef    string
	PolicyFiltered bool
}

type DisclosedItem struct {
	ItemID         string
	Kind           ItemKind
	Trust          TrustClass
	Scope          Scope
	Priority       Priority
	TokenCount     int
	Required       bool
	Content        []byte
	ContentDigest  string
	ReferenceID    string
	SourceType     SourceType
	SourceRef      string
	AllowedAgentID string
	AllowedRoleID  string
	ArtifactRef    string
}

type OmittedItem struct {
	ItemID         string
	Kind           ItemKind
	Trust          TrustClass
	Scope          Scope
	Priority       Priority
	TokenCount     int
	Required       bool
	ContentDigest  string
	SourceType     SourceType
	SourceRef      string
	AllowedAgentID string
	AllowedRoleID  string
	ArtifactRef    string
	Reason         OmissionReason
}

type RetrievalRequest struct {
	Authority        AuthorityRecord
	ItemID           string
	ContentDigest    string
	RequesterAgentID string
	RequesterRoleID  string
	ArtifactRef      string
}

type RetrievedItem struct {
	ItemID         string
	Kind           ItemKind
	Trust          TrustClass
	Scope          Scope
	Priority       Priority
	TokenCount     int
	Content        []byte
	ContentDigest  string
	SourceType     SourceType
	SourceRef      string
	AllowedAgentID string
	AllowedRoleID  string
	ArtifactRef    string
}

func (item *RetrievedItem) Close() {
	if item == nil {
		return
	}
	for index := range item.Content {
		item.Content[index] = 0
	}
	item.Content = nil
}

type retrievableItem struct {
	ItemID         string     `json:"item_id"`
	Kind           ItemKind   `json:"kind"`
	Trust          TrustClass `json:"trust"`
	Scope          Scope      `json:"scope"`
	Priority       Priority   `json:"priority"`
	TokenCount     int        `json:"token_count"`
	Content        []byte     `json:"content"`
	ContentDigest  string     `json:"content_digest"`
	ReferenceID    string     `json:"reference_id,omitempty"`
	SourceType     SourceType `json:"source_type"`
	SourceRef      string     `json:"source_ref"`
	AllowedAgentID string     `json:"allowed_agent_id,omitempty"`
	AllowedRoleID  string     `json:"allowed_role_id,omitempty"`
	ArtifactRef    string     `json:"artifact_ref,omitempty"`
}

type AuthorityRecord struct {
	SchemaVersion                   int            `json:"schema_version"`
	CapsuleDigest                   string         `json:"capsule_digest"`
	DisclosureReceiptDigest         string         `json:"disclosure_receipt_digest"`
	ConversationID                  string         `json:"conversation_id"`
	TeamID                          string         `json:"team_id"`
	AgentID                         string         `json:"agent_id"`
	RoleID                          string         `json:"role_id"`
	ProviderID                      string         `json:"provider_id"`
	ProviderAccountID               string         `json:"provider_account_id"`
	ModelID                         string         `json:"model_id"`
	AuthMode                        string         `json:"auth_mode"`
	ContextAdapterID                string         `json:"context_adapter_id"`
	DisclosurePolicyID              string         `json:"disclosure_policy_id"`
	DisclosurePolicyVersion         int            `json:"disclosure_policy_version"`
	TokenBudget                     int            `json:"token_budget"`
	TokenCount                      int            `json:"token_count"`
	DisclosedCount                  int            `json:"disclosed_count"`
	OmittedCount                    int            `json:"omitted_count"`
	CapacityProjectionDigest        string         `json:"capacity_projection_digest,omitempty"`
	CapacitySchemaVersion           int            `json:"capacity_schema_version,omitempty"`
	CapacityStatus                  CapacityStatus `json:"capacity_status,omitempty"`
	ContextWindowTokens             int            `json:"context_window_tokens,omitempty"`
	ReservedOutputTokens            int            `json:"reserved_output_tokens,omitempty"`
	AdapterToolOverheadTokens       int            `json:"adapter_tool_overhead_tokens,omitempty"`
	PolicyInputBudgetTokens         int            `json:"policy_input_budget_tokens,omitempty"`
	AdmittedInputBudgetTokens       int            `json:"admitted_input_budget_tokens,omitempty"`
	TokenCounterID                  string         `json:"token_counter_id,omitempty"`
	TokenCounterVersion             string         `json:"token_counter_version,omitempty"`
	AdmittedContributionTokens      int            `json:"admitted_contribution_tokens,omitempty"`
	BudgetOmittedContributionTokens int            `json:"budget_omitted_contribution_tokens,omitempty"`
}

type RoleContextCapsule struct {
	target                  Target
	digest                  string
	disclosureReceiptDigest string
	tokenCount              int
	disclosed               []DisclosedItem
	omitted                 []OmittedItem
	retrievable             map[string]retrievableItem
	capacity                *CapacityProjection
}

func BuildRoleContextCapsule(target Target, inputs []ItemInput) (RoleContextCapsule, error) {
	return buildRoleContextCapsule(target, inputs, target.TokenBudget)
}

func buildRoleContextCapsule(
	target Target,
	inputs []ItemInput,
	admittedBudget int,
) (RoleContextCapsule, error) {
	target, items, artifacts, err := prepareRoleContextCapsuleInputs(target, inputs, admittedBudget)
	if err != nil {
		return RoleContextCapsule{}, err
	}
	return packRoleContextCapsule(target, items, artifacts, admittedBudget)
}

func prepareRoleContextCapsuleInputs(
	target Target,
	inputs []ItemInput,
	admittedBudget int,
) (Target, []ItemInput, map[string]struct{}, error) {
	if !validIdentifier(target.ConversationID) || !validIdentifier(target.TeamID) ||
		!validIdentifier(target.AgentID) || !validIdentifier(target.RoleID) ||
		!validRouteTarget(target.ProviderID, target.ProviderAccountID, target.ModelID, target.AuthMode) ||
		!validIdentifier(target.ContextAdapterID) ||
		!validIdentifier(target.DisclosurePolicyID) ||
		target.DisclosurePolicyVersion < 1 ||
		target.TokenBudget < 1 || target.TokenBudget > maxTokenBudget ||
		admittedBudget < 1 || admittedBudget > target.TokenBudget ||
		len(inputs) == 0 || len(inputs) > maxCapsuleItems {
		return Target{}, nil, nil, ErrInvalidCapsule
	}
	target.ArtifactRefs = append([]string(nil), target.ArtifactRefs...)
	sort.Strings(target.ArtifactRefs)
	artifacts := make(map[string]struct{}, len(target.ArtifactRefs))
	for _, reference := range target.ArtifactRefs {
		if !validIdentifier(reference) {
			return Target{}, nil, nil, ErrInvalidCapsule
		}
		if _, duplicate := artifacts[reference]; duplicate {
			return Target{}, nil, nil, ErrInvalidCapsule
		}
		artifacts[reference] = struct{}{}
	}

	items := append([]ItemInput(nil), inputs...)
	seen := make(map[string]struct{}, len(items))
	totalContentBytes := 0
	for _, item := range items {
		if !validItem(item) {
			return Target{}, nil, nil, ErrInvalidCapsule
		}
		if _, duplicate := seen[item.ItemID]; duplicate {
			return Target{}, nil, nil, ErrInvalidCapsule
		}
		seen[item.ItemID] = struct{}{}
		if len(item.Content) > maxTotalContentBytes-totalContentBytes {
			return Target{}, nil, nil, ErrInvalidCapsule
		}
		totalContentBytes += len(item.Content)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Priority != items[j].Priority {
			return items[i].Priority < items[j].Priority
		}
		return items[i].ItemID < items[j].ItemID
	})
	return target, items, artifacts, nil
}

func packRoleContextCapsule(
	target Target,
	items []ItemInput,
	artifacts map[string]struct{},
	admittedBudget int,
) (RoleContextCapsule, error) {
	capsule := RoleContextCapsule{
		target:      target,
		disclosed:   make([]DisclosedItem, 0, len(items)),
		omitted:     make([]OmittedItem, 0, len(items)),
		retrievable: make(map[string]retrievableItem),
	}
	for _, item := range items {
		contentDigest := digestBytes(item.Content)
		if item.PolicyFiltered {
			if item.Required {
				return RoleContextCapsule{}, ErrRequiredContextOmitted
			}
			capsule.omitted = append(capsule.omitted, omittedItem(
				item, contentDigest, OmissionPolicyFiltered,
			))
			continue
		}
		if !scopeAllows(target, artifacts, item) {
			if item.Required {
				return RoleContextCapsule{}, ErrRequiredContextOmitted
			}
			capsule.omitted = append(capsule.omitted, omittedItem(
				item, contentDigest, OmissionAccessDenied,
			))
			continue
		}
		if capsule.tokenCount > admittedBudget-item.TokenCount {
			if item.Required {
				return RoleContextCapsule{}, ErrRequiredContextOmitted
			}
			capsule.omitted = append(capsule.omitted, omittedItem(
				item, contentDigest, OmissionBudgetExceeded,
			))
			capsule.retrievable[item.ItemID] = retrievableContextItem(
				item, contentDigest,
			)
			continue
		}
		capsule.disclosed = append(capsule.disclosed, DisclosedItem{
			ItemID: item.ItemID, Kind: item.Kind, Trust: item.Trust,
			Scope: item.Scope, Priority: item.Priority, TokenCount: item.TokenCount,
			Required: item.Required,
			Content:  append([]byte(nil), item.Content...), ContentDigest: contentDigest,
			ReferenceID: item.ReferenceID,
			SourceType:  item.SourceType, SourceRef: item.SourceRef,
			AllowedAgentID: item.AllowedAgentID, AllowedRoleID: item.AllowedRoleID,
			ArtifactRef: item.ArtifactRef,
		})
		capsule.tokenCount += item.TokenCount
	}
	if len(capsule.disclosed) == 0 {
		return RoleContextCapsule{}, ErrInvalidCapsule
	}

	if err := capsule.seal(); err != nil {
		return RoleContextCapsule{}, ErrInvalidCapsule
	}
	return capsule, nil
}

// ExtendRoleContextCapsule deterministically repacks an admitted Capsule with
// new target-local inputs. Existing items cannot be replaced, and prior
// policy/access omissions remain content-free and non-retrievable.
func ExtendRoleContextCapsule(
	base RoleContextCapsule,
	additions []ItemInput,
) (RoleContextCapsule, error) {
	if !base.Valid() || base.capacity != nil || len(additions) == 0 ||
		len(base.disclosed)+len(base.omitted)+len(additions) > maxCapsuleItems {
		return RoleContextCapsule{}, ErrInvalidCapsule
	}
	items, fixedOmissions, err := extensionInputs(base, additions)
	if err != nil {
		return RoleContextCapsule{}, err
	}
	rebuilt, err := BuildRoleContextCapsule(base.Target(), items)
	if err != nil {
		return RoleContextCapsule{}, err
	}
	return finalizeExtendedCapsule(rebuilt, fixedOmissions)
}

func extensionInputs(
	base RoleContextCapsule,
	additions []ItemInput,
) ([]ItemInput, []OmittedItem, error) {
	items := make([]ItemInput, 0, len(base.disclosed)+len(base.retrievable)+len(additions))
	seen := make(map[string]struct{}, len(base.disclosed)+len(base.omitted)+len(additions))
	for _, item := range base.disclosed {
		seen[item.ItemID] = struct{}{}
		items = append(items, itemInputFromDisclosed(item))
	}
	fixedOmissions := make([]OmittedItem, 0, len(base.omitted))
	for _, omission := range base.omitted {
		if _, duplicate := seen[omission.ItemID]; duplicate {
			return nil, nil, ErrInvalidCapsule
		}
		seen[omission.ItemID] = struct{}{}
		if omission.Reason != OmissionBudgetExceeded {
			fixedOmissions = append(fixedOmissions, omission)
			continue
		}
		retrievable, found := base.retrievable[omission.ItemID]
		if !found || !retrievableMatchesOmission(retrievable, omission) {
			return nil, nil, ErrInvalidCapsule
		}
		items = append(items, itemInputFromRetrievable(retrievable))
	}
	for _, addition := range additions {
		if _, duplicate := seen[addition.ItemID]; duplicate {
			return nil, nil, ErrInvalidCapsule
		}
		seen[addition.ItemID] = struct{}{}
		items = append(items, addition)
	}
	return items, fixedOmissions, nil
}

func finalizeExtendedCapsule(
	rebuilt RoleContextCapsule,
	fixedOmissions []OmittedItem,
) (RoleContextCapsule, error) {
	rebuilt.omitted = append(rebuilt.omitted, fixedOmissions...)
	sort.Slice(rebuilt.omitted, func(i, j int) bool {
		if rebuilt.omitted[i].Priority != rebuilt.omitted[j].Priority {
			return rebuilt.omitted[i].Priority < rebuilt.omitted[j].Priority
		}
		return rebuilt.omitted[i].ItemID < rebuilt.omitted[j].ItemID
	})
	if err := rebuilt.seal(); err != nil {
		return RoleContextCapsule{}, ErrInvalidCapsule
	}
	if !rebuilt.Valid() {
		return RoleContextCapsule{}, ErrInvalidCapsule
	}
	return rebuilt, nil
}

func itemInputFromDisclosed(item DisclosedItem) ItemInput {
	return ItemInput{
		ItemID: item.ItemID, Kind: item.Kind, Trust: item.Trust,
		Scope: item.Scope, Priority: item.Priority, TokenCount: item.TokenCount,
		Required: item.Required, Content: append([]byte(nil), item.Content...),
		ReferenceID: item.ReferenceID, SourceType: item.SourceType,
		SourceRef: item.SourceRef, AllowedAgentID: item.AllowedAgentID,
		AllowedRoleID: item.AllowedRoleID, ArtifactRef: item.ArtifactRef,
	}
}

func itemInputFromRetrievable(item retrievableItem) ItemInput {
	return ItemInput{
		ItemID: item.ItemID, Kind: item.Kind, Trust: item.Trust,
		Scope: item.Scope, Priority: item.Priority, TokenCount: item.TokenCount,
		Content: append([]byte(nil), item.Content...), ReferenceID: item.ReferenceID,
		SourceType: item.SourceType,
		SourceRef:  item.SourceRef, AllowedAgentID: item.AllowedAgentID,
		AllowedRoleID: item.AllowedRoleID, ArtifactRef: item.ArtifactRef,
	}
}

func retrievableContextItem(item ItemInput, contentDigest string) retrievableItem {
	return retrievableItem{
		ItemID: item.ItemID, Kind: item.Kind, Trust: item.Trust,
		Scope: item.Scope, Priority: item.Priority, TokenCount: item.TokenCount,
		Content: append([]byte(nil), item.Content...), ContentDigest: contentDigest,
		ReferenceID: item.ReferenceID, SourceType: item.SourceType, SourceRef: item.SourceRef,
		AllowedAgentID: item.AllowedAgentID, AllowedRoleID: item.AllowedRoleID,
		ArtifactRef: item.ArtifactRef,
	}
}

func omittedItem(
	item ItemInput,
	contentDigest string,
	reason OmissionReason,
) OmittedItem {
	return OmittedItem{
		ItemID: item.ItemID, Kind: item.Kind, Trust: item.Trust,
		Scope: item.Scope, Priority: item.Priority, TokenCount: item.TokenCount,
		Required: item.Required, ContentDigest: contentDigest,
		SourceType: item.SourceType, SourceRef: item.SourceRef,
		AllowedAgentID: item.AllowedAgentID, AllowedRoleID: item.AllowedRoleID,
		ArtifactRef: item.ArtifactRef, Reason: reason,
	}
}

func (capsule RoleContextCapsule) Digest() string { return capsule.digest }

func (capsule RoleContextCapsule) TokenCount() int { return capsule.tokenCount }

func (capsule RoleContextCapsule) DisclosureReceiptDigest() string {
	return capsule.disclosureReceiptDigest
}

func (capsule RoleContextCapsule) Target() Target {
	target := capsule.target
	target.ArtifactRefs = append([]string(nil), target.ArtifactRefs...)
	return target
}

func (capsule RoleContextCapsule) CapacityProjection() (CapacityProjection, bool) {
	if !capsule.Valid() || capsule.capacity == nil {
		return CapacityProjection{}, false
	}
	return *cloneCapacityProjection(capsule.capacity), true
}

func (capsule RoleContextCapsule) Valid() bool {
	if capsule.digest == "" || capsule.disclosureReceiptDigest == "" {
		return false
	}
	if capsule.capacity != nil && !validCapacityProjectionForCapsule(*capsule.capacity, capsule) {
		return false
	}
	body, err := capsule.canonicalBody()
	if err != nil || digestBytes(body) != capsule.digest {
		return false
	}
	receiptBody, err := canonicalReceiptBody(capsule.authorityRecord())
	return err == nil && digestBytes(receiptBody) == capsule.disclosureReceiptDigest
}

func (capsule RoleContextCapsule) AuthorityRecord() AuthorityRecord {
	if !capsule.Valid() {
		return AuthorityRecord{}
	}
	record := capsule.authorityRecord()
	record.DisclosureReceiptDigest = capsule.disclosureReceiptDigest
	return record
}

func (capsule RoleContextCapsule) Disclosed() []DisclosedItem {
	result := append([]DisclosedItem(nil), capsule.disclosed...)
	for index := range result {
		result[index].Content = append([]byte(nil), result[index].Content...)
	}
	return result
}

func (capsule RoleContextCapsule) Omitted() []OmittedItem {
	return append([]OmittedItem(nil), capsule.omitted...)
}

func (capsule RoleContextCapsule) IsRetrievable(itemID string) bool {
	if !capsule.Valid() || !validIdentifier(itemID) {
		return false
	}
	omission, found := capsule.omittedItem(itemID)
	if !found || omission.Reason != OmissionBudgetExceeded ||
		omission.Scope == ScopeSecretReferenceOnly ||
		omission.Kind == KindCredentialReference ||
		omission.SourceType == SourceCredentialReference {
		return false
	}
	item, found := capsule.retrievable[itemID]
	return found && retrievableMatchesOmission(item, omission) &&
		digestBytes(item.Content) == item.ContentDigest
}

func (capsule RoleContextCapsule) Retrieve(request RetrievalRequest) (RetrievedItem, error) {
	if !capsule.Valid() || request.Authority != capsule.AuthorityRecord() ||
		!validIdentifier(request.ItemID) || !validDigest(request.ContentDigest) ||
		request.RequesterAgentID != capsule.target.AgentID ||
		request.RequesterRoleID != capsule.target.RoleID {
		return RetrievedItem{}, ErrContextRetrievalDenied
	}
	omission, found := capsule.omittedItem(request.ItemID)
	if !found || omission.Reason != OmissionBudgetExceeded {
		return RetrievedItem{}, ErrContextItemNotRetrievable
	}
	item, found := capsule.retrievable[request.ItemID]
	if !found {
		return RetrievedItem{}, ErrContextItemNotRetrievable
	}
	if request.ContentDigest != omission.ContentDigest ||
		item.ContentDigest != omission.ContentDigest ||
		digestBytes(item.Content) != item.ContentDigest ||
		!retrievalScopeAllows(capsule.target, request, item) {
		return RetrievedItem{}, ErrContextRetrievalDenied
	}
	return RetrievedItem{
		ItemID: item.ItemID, Kind: item.Kind, Trust: item.Trust,
		Scope: item.Scope, Priority: item.Priority, TokenCount: item.TokenCount,
		Content: append([]byte(nil), item.Content...), ContentDigest: item.ContentDigest,
		SourceType: item.SourceType, SourceRef: item.SourceRef,
		AllowedAgentID: item.AllowedAgentID, AllowedRoleID: item.AllowedRoleID,
		ArtifactRef: item.ArtifactRef,
	}, nil
}

func (capsule RoleContextCapsule) omittedItem(itemID string) (OmittedItem, bool) {
	for _, item := range capsule.omitted {
		if item.ItemID == itemID {
			return item, true
		}
	}
	return OmittedItem{}, false
}

func retrievalScopeAllows(
	target Target,
	request RetrievalRequest,
	item retrievableItem,
) bool {
	switch item.Scope {
	case ScopeConversationShared, ScopeTeamShared:
		return request.ArtifactRef == ""
	case ScopeAgentPrivate:
		return request.ArtifactRef == "" && item.AllowedAgentID == target.AgentID
	case ScopeRoleRestricted:
		return request.ArtifactRef == "" && item.AllowedRoleID == target.RoleID
	case ScopeArtifactScoped:
		if request.ArtifactRef == "" || request.ArtifactRef != item.ArtifactRef {
			return false
		}
		for _, reference := range target.ArtifactRefs {
			if reference == request.ArtifactRef {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func MarshalCanonicalRetrievableContext(capsule RoleContextCapsule) ([]byte, error) {
	if !capsule.Valid() {
		return nil, ErrInvalidCapsule
	}
	items := make([]retrievableItem, 0, len(capsule.retrievable))
	for _, item := range capsule.retrievable {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ItemID < items[j].ItemID })
	body, err := json.Marshal(struct {
		SchemaVersion int               `json:"schema_version"`
		Items         []retrievableItem `json:"items"`
	}{SchemaVersion: 1, Items: items})
	if err != nil || len(body) == 0 || len(body) > maxTotalContentBytes {
		return nil, ErrInvalidCapsule
	}
	return body, nil
}

func RestoreRetrievableContext(
	capsule RoleContextCapsule,
	canonicalBody []byte,
) (RoleContextCapsule, error) {
	if !capsule.Valid() || len(canonicalBody) == 0 ||
		len(canonicalBody) > maxTotalContentBytes {
		return RoleContextCapsule{}, ErrInvalidCapsule
	}
	decoder := json.NewDecoder(bytes.NewReader(canonicalBody))
	decoder.DisallowUnknownFields()
	var wire struct {
		SchemaVersion int               `json:"schema_version"`
		Items         []retrievableItem `json:"items"`
	}
	if decoder.Decode(&wire) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		wire.SchemaVersion != 1 || len(wire.Items) > len(capsule.omitted) {
		return RoleContextCapsule{}, ErrInvalidCapsule
	}
	canonical, err := json.Marshal(wire)
	if err != nil || !bytes.Equal(canonical, canonicalBody) {
		return RoleContextCapsule{}, ErrInvalidCapsule
	}
	retrievable := make(map[string]retrievableItem, len(wire.Items))
	previousID := ""
	for _, item := range wire.Items {
		omission, found := capsule.omittedItem(item.ItemID)
		if !found || omission.Reason != OmissionBudgetExceeded ||
			previousID != "" && item.ItemID <= previousID ||
			!retrievableMatchesOmission(item, omission) {
			return RoleContextCapsule{}, ErrInvalidCapsule
		}
		retrievable[item.ItemID] = retrievableContextItem(ItemInput{
			ItemID: item.ItemID, Kind: item.Kind, Trust: item.Trust,
			Scope: item.Scope, Priority: item.Priority, TokenCount: item.TokenCount,
			Content: item.Content, ReferenceID: item.ReferenceID,
			SourceType: item.SourceType, SourceRef: item.SourceRef,
			AllowedAgentID: item.AllowedAgentID, AllowedRoleID: item.AllowedRoleID,
			ArtifactRef: item.ArtifactRef,
		}, item.ContentDigest)
		previousID = item.ItemID
	}
	for _, omission := range capsule.omitted {
		if omission.Reason == OmissionBudgetExceeded {
			if _, found := retrievable[omission.ItemID]; !found {
				return RoleContextCapsule{}, ErrInvalidCapsule
			}
		}
	}
	capsule.retrievable = retrievable
	return capsule, nil
}

func retrievableMatchesOmission(item retrievableItem, omission OmittedItem) bool {
	return validItem(ItemInput{
		ItemID: item.ItemID, Kind: item.Kind, Trust: item.Trust,
		Scope: item.Scope, Priority: item.Priority, TokenCount: item.TokenCount,
		Content: item.Content, ReferenceID: item.ReferenceID,
		SourceType: item.SourceType, SourceRef: item.SourceRef,
		AllowedAgentID: item.AllowedAgentID, AllowedRoleID: item.AllowedRoleID,
		ArtifactRef: item.ArtifactRef,
	}) && item.ItemID == omission.ItemID && item.Kind == omission.Kind &&
		item.Trust == omission.Trust && item.Scope == omission.Scope &&
		item.Priority == omission.Priority && item.TokenCount == omission.TokenCount &&
		item.ContentDigest == omission.ContentDigest && digestBytes(item.Content) == item.ContentDigest &&
		(item.SourceType != SourceCredentialReference || validIdentifier(item.ReferenceID)) &&
		item.SourceType == omission.SourceType && item.SourceRef == omission.SourceRef &&
		item.AllowedAgentID == omission.AllowedAgentID &&
		item.AllowedRoleID == omission.AllowedRoleID && item.ArtifactRef == omission.ArtifactRef
}

func MarshalCanonicalRoleContextCapsule(
	capsule RoleContextCapsule,
) ([]byte, error) {
	if !capsule.Valid() {
		return nil, ErrInvalidCapsule
	}
	body, err := capsule.canonicalBody()
	if err != nil || len(body) == 0 || len(body) > maxTotalContentBytes {
		return nil, ErrInvalidCapsule
	}
	return body, nil
}

func RestoreRoleContextCapsule(
	authority AuthorityRecord,
	canonicalBody []byte,
) (RoleContextCapsule, error) {
	validatedAuthority, err := ValidateAuthorityRecord(authority)
	if err != nil || len(canonicalBody) == 0 ||
		len(canonicalBody) > maxTotalContentBytes {
		return RoleContextCapsule{}, ErrInvalidCapsule
	}
	decoder := json.NewDecoder(bytes.NewReader(canonicalBody))
	decoder.DisallowUnknownFields()
	var wire struct {
		SchemaVersion int                 `json:"schema_version"`
		Target        Target              `json:"target"`
		Disclosed     []DisclosedItem     `json:"disclosed"`
		Omitted       []OmittedItem       `json:"omitted"`
		Capacity      *CapacityProjection `json:"capacity,omitempty"`
	}
	if decoder.Decode(&wire) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		wire.SchemaVersion != schemaVersion || len(wire.Disclosed) == 0 ||
		len(wire.Disclosed)+len(wire.Omitted) > maxCapsuleItems {
		return RoleContextCapsule{}, ErrInvalidCapsule
	}
	canonical, err := json.Marshal(wire)
	if err != nil || !bytes.Equal(canonical, canonicalBody) {
		return RoleContextCapsule{}, ErrInvalidCapsule
	}
	seen := make(map[string]struct{}, len(wire.Disclosed)+len(wire.Omitted))
	tokenCount := 0
	previousPriority := PrioritySystem
	previousID := ""
	for _, item := range wire.Disclosed {
		if !validRestoredDisclosedItem(item) ||
			!restoredItemOrder(previousPriority, previousID, item.Priority, item.ItemID) {
			return RoleContextCapsule{}, ErrInvalidCapsule
		}
		if _, duplicate := seen[item.ItemID]; duplicate {
			return RoleContextCapsule{}, ErrInvalidCapsule
		}
		seen[item.ItemID] = struct{}{}
		tokenCount += item.TokenCount
		previousPriority, previousID = item.Priority, item.ItemID
	}
	previousPriority, previousID = PrioritySystem, ""
	for _, item := range wire.Omitted {
		if !validRestoredOmittedItem(item) ||
			!restoredItemOrder(previousPriority, previousID, item.Priority, item.ItemID) {
			return RoleContextCapsule{}, ErrInvalidCapsule
		}
		if _, duplicate := seen[item.ItemID]; duplicate {
			return RoleContextCapsule{}, ErrInvalidCapsule
		}
		seen[item.ItemID] = struct{}{}
		previousPriority, previousID = item.Priority, item.ItemID
	}
	if tokenCount != validatedAuthority.TokenCount {
		return RoleContextCapsule{}, ErrInvalidCapsule
	}
	restored := RoleContextCapsule{
		target: wire.Target, disclosed: wire.Disclosed, omitted: wire.Omitted,
		tokenCount:              validatedAuthority.TokenCount,
		digest:                  validatedAuthority.CapsuleDigest,
		disclosureReceiptDigest: validatedAuthority.DisclosureReceiptDigest,
		capacity:                cloneCapacityProjection(wire.Capacity),
	}
	if capacityProjectionDigest(restored.capacity) != validatedAuthority.CapacityProjectionDigest {
		return RoleContextCapsule{}, ErrInvalidCapsule
	}
	if !restored.Valid() || restored.AuthorityRecord() != validatedAuthority {
		return RoleContextCapsule{}, ErrInvalidCapsule
	}
	return restored, nil
}

func validRestoredDisclosedItem(item DisclosedItem) bool {
	return item.ContentDigest == digestBytes(item.Content) && validItem(ItemInput{
		ItemID: item.ItemID, Kind: item.Kind, Trust: item.Trust,
		Scope: item.Scope, Priority: item.Priority, TokenCount: item.TokenCount,
		Required: item.Required, Content: item.Content, ReferenceID: item.ReferenceID,
		SourceType: item.SourceType, SourceRef: item.SourceRef,
		AllowedAgentID: item.AllowedAgentID, AllowedRoleID: item.AllowedRoleID,
		ArtifactRef: item.ArtifactRef,
	})
}

func validRestoredOmittedItem(item OmittedItem) bool {
	if !validIdentifier(item.ItemID) || !validKind(item.Kind) ||
		!validTrust(item.Trust) || !validScope(item.Scope) ||
		item.Priority < PrioritySystem || item.Priority > PriorityRetrievable ||
		item.TokenCount <= 0 || item.Required || !validDigest(item.ContentDigest) ||
		!validIdentifier(item.SourceRef) ||
		item.Scope == ScopeAgentPrivate && !validIdentifier(item.AllowedAgentID) ||
		item.Scope == ScopeRoleRestricted && !validIdentifier(item.AllowedRoleID) ||
		item.Scope == ScopeArtifactScoped && !validIdentifier(item.ArtifactRef) ||
		(item.Reason != OmissionAccessDenied &&
			item.Reason != OmissionBudgetExceeded &&
			item.Reason != OmissionPolicyFiltered) {
		return false
	}
	if item.Scope == ScopeSecretReferenceOnly || item.Kind == KindCredentialReference ||
		item.SourceType == SourceCredentialReference {
		return item.Scope == ScopeSecretReferenceOnly && item.Kind == KindCredentialReference &&
			item.SourceType == SourceCredentialReference && item.Trust == TrustAuthoritative &&
			item.ContentDigest == digestBytes(nil) && item.AllowedAgentID == "" &&
			item.AllowedRoleID == "" && item.ArtifactRef == "" &&
			(item.Reason == OmissionBudgetExceeded || item.Reason == OmissionPolicyFiltered)
	}
	if item.SourceType == SourceModelOutput || item.Kind == KindPriorModelOutput {
		if item.SourceType != SourceModelOutput || item.Kind != KindPriorModelOutput ||
			item.Trust != TrustUntrusted {
			return false
		}
	}
	switch item.SourceType {
	case SourceAuthority:
		return item.Trust == TrustAuthoritative
	case SourceObservation:
		return item.Trust == TrustObserved
	case SourceModelOutput:
		return item.Trust == TrustUntrusted && item.Kind == KindPriorModelOutput
	default:
		return false
	}
}

func restoredItemOrder(
	previousPriority Priority,
	previousID string,
	priority Priority,
	itemID string,
) bool {
	return previousID == "" || priority > previousPriority ||
		priority == previousPriority && itemID > previousID
}

func (capsule RoleContextCapsule) canonicalBody() ([]byte, error) {
	canonical := struct {
		SchemaVersion int                 `json:"schema_version"`
		Target        Target              `json:"target"`
		Disclosed     []DisclosedItem     `json:"disclosed"`
		Omitted       []OmittedItem       `json:"omitted"`
		Capacity      *CapacityProjection `json:"capacity,omitempty"`
	}{schemaVersion, capsule.target, capsule.disclosed, capsule.omitted, capsule.capacity}
	return json.Marshal(canonical)
}

func (capsule *RoleContextCapsule) seal() error {
	body, err := capsule.canonicalBody()
	if err != nil {
		return err
	}
	capsule.digest = digestBytes(body)
	receiptBody, err := canonicalReceiptBody(capsule.authorityRecord())
	if err != nil {
		return err
	}
	capsule.disclosureReceiptDigest = digestBytes(receiptBody)
	return nil
}

func (capsule RoleContextCapsule) authorityRecord() AuthorityRecord {
	record := AuthorityRecord{
		SchemaVersion: schemaVersion, CapsuleDigest: capsule.digest,
		ConversationID: capsule.target.ConversationID, TeamID: capsule.target.TeamID,
		AgentID: capsule.target.AgentID, RoleID: capsule.target.RoleID,
		ProviderID:        capsule.target.ProviderID,
		ProviderAccountID: capsule.target.ProviderAccountID,
		ModelID:           capsule.target.ModelID, AuthMode: capsule.target.AuthMode,
		ContextAdapterID:        capsule.target.ContextAdapterID,
		DisclosurePolicyID:      capsule.target.DisclosurePolicyID,
		DisclosurePolicyVersion: capsule.target.DisclosurePolicyVersion,
		TokenBudget:             capsule.target.TokenBudget, TokenCount: capsule.tokenCount,
		DisclosedCount: len(capsule.disclosed), OmittedCount: len(capsule.omitted),
	}
	if capsule.capacity != nil {
		record.CapacityProjectionDigest = capacityProjectionDigest(capsule.capacity)
		record.CapacitySchemaVersion = capsule.capacity.SchemaVersion
		record.CapacityStatus = capsule.capacity.Status
		record.ContextWindowTokens = capsule.capacity.ContextWindowTokens
		record.ReservedOutputTokens = capsule.capacity.ReservedOutputTokens
		record.AdapterToolOverheadTokens = capsule.capacity.AdapterToolOverheadTokens
		record.PolicyInputBudgetTokens = capsule.capacity.PolicyInputBudgetTokens
		record.AdmittedInputBudgetTokens = capsule.capacity.AdmittedInputBudgetTokens
		record.TokenCounterID = capsule.capacity.TokenCounterID
		record.TokenCounterVersion = capsule.capacity.TokenCounterVersion
		record.AdmittedContributionTokens = capsule.capacity.AdmittedContributionTokens
		record.BudgetOmittedContributionTokens = capsule.capacity.BudgetOmittedContributionTokens
	}
	return record
}

func ValidateAuthorityRecord(record AuthorityRecord) (AuthorityRecord, error) {
	if record.SchemaVersion != schemaVersion || !validDigest(record.CapsuleDigest) ||
		!validDigest(record.DisclosureReceiptDigest) ||
		!validIdentifier(record.ConversationID) || !validIdentifier(record.TeamID) ||
		!validIdentifier(record.AgentID) || !validIdentifier(record.RoleID) ||
		!validRouteTarget(record.ProviderID, record.ProviderAccountID, record.ModelID, record.AuthMode) ||
		!validIdentifier(record.ContextAdapterID) ||
		!validIdentifier(record.DisclosurePolicyID) || record.DisclosurePolicyVersion < 1 ||
		record.TokenBudget < 1 || record.TokenBudget > maxTokenBudget ||
		record.TokenCount < 1 ||
		record.TokenCount > record.TokenBudget || record.DisclosedCount < 0 ||
		record.OmittedCount < 0 || record.DisclosedCount < 1 ||
		record.DisclosedCount+record.OmittedCount > maxCapsuleItems ||
		!validAuthorityCapacity(record) {
		return AuthorityRecord{}, ErrInvalidCapsule
	}
	body, err := canonicalReceiptBody(record)
	if err != nil || digestBytes(body) != record.DisclosureReceiptDigest {
		return AuthorityRecord{}, ErrInvalidCapsule
	}
	return record, nil
}

func validAuthorityCapacity(record AuthorityRecord) bool {
	if record.CapacitySchemaVersion == 0 {
		return record.CapacityProjectionDigest == "" && record.CapacityStatus == "" &&
			record.ContextWindowTokens == 0 && record.ReservedOutputTokens == 0 &&
			record.AdapterToolOverheadTokens == 0 && record.PolicyInputBudgetTokens == 0 &&
			record.AdmittedInputBudgetTokens == 0 && record.TokenCounterID == "" &&
			record.TokenCounterVersion == "" && record.AdmittedContributionTokens == 0 &&
			record.BudgetOmittedContributionTokens == 0
	}
	if !validDigest(record.CapacityProjectionDigest) ||
		record.PolicyInputBudgetTokens != record.TokenBudget ||
		record.AdmittedContributionTokens != record.TokenCount ||
		record.BudgetOmittedContributionTokens < 0 ||
		record.BudgetOmittedContributionTokens > MaxCapacityTokens {
		return false
	}
	authority := CapacityAuthority{
		SchemaVersion: record.CapacitySchemaVersion, Status: record.CapacityStatus,
		ContextWindowTokens:       record.ContextWindowTokens,
		ReservedOutputTokens:      record.ReservedOutputTokens,
		AdapterToolOverheadTokens: record.AdapterToolOverheadTokens,
		TokenCounterID:            record.TokenCounterID, TokenCounterVersion: record.TokenCounterVersion,
	}
	resolved, err := resolveAdmittedInputBudget(record.PolicyInputBudgetTokens, authority)
	return err == nil && resolved == record.AdmittedInputBudgetTokens &&
		record.TokenCount <= record.AdmittedInputBudgetTokens
}

func canonicalReceiptBody(record AuthorityRecord) ([]byte, error) {
	record.DisclosureReceiptDigest = ""
	return json.Marshal(record)
}

func validItem(item ItemInput) bool {
	if !validIdentifier(item.ItemID) || !validKind(item.Kind) ||
		!validTrust(item.Trust) || !validScope(item.Scope) ||
		item.Priority < PrioritySystem || item.Priority > PriorityRetrievable ||
		item.TokenCount <= 0 || item.SourceType == "" || !validIdentifier(item.SourceRef) ||
		item.PolicyFiltered && item.Required ||
		item.Scope == ScopeAgentPrivate && !validIdentifier(item.AllowedAgentID) ||
		item.Scope == ScopeRoleRestricted && !validIdentifier(item.AllowedRoleID) ||
		item.Scope == ScopeArtifactScoped && !validIdentifier(item.ArtifactRef) {
		return false
	}
	switch item.SourceType {
	case SourceAuthority:
		if item.Trust != TrustAuthoritative {
			return false
		}
	case SourceObservation:
		if item.Trust != TrustObserved {
			return false
		}
	case SourceModelOutput:
		if item.Trust != TrustUntrusted {
			return false
		}
	case SourceCredentialReference:
		if item.Trust != TrustAuthoritative {
			return false
		}
	default:
		return false
	}
	if item.SourceType == SourceModelOutput || item.Kind == KindPriorModelOutput {
		if item.SourceType != SourceModelOutput || item.Kind != KindPriorModelOutput ||
			item.Trust != TrustUntrusted {
			return false
		}
	}
	if item.Scope == ScopeSecretReferenceOnly || item.Kind == KindCredentialReference ||
		item.SourceType == SourceCredentialReference {
		return item.Scope == ScopeSecretReferenceOnly && item.Kind == KindCredentialReference &&
			item.SourceType == SourceCredentialReference && validIdentifier(item.ReferenceID) &&
			len(item.Content) == 0
	}
	return len(item.Content) > 0 && len(item.Content) <= maxItemContentBytes &&
		item.ReferenceID == ""
}

func validKind(value ItemKind) bool {
	switch value {
	case KindConversationGoal, KindCurrentTaskState, KindSystemPolicy,
		KindAcceptedDecision, KindArtifactReference, KindConfirmedConstraint,
		KindWorkspaceSnapshot, KindGovernanceState, KindRecentUserTurn,
		KindUnresolvedQuestion, KindPriorModelOutput, KindAggregationSource,
		KindDependencySource, KindObservedExecutionState,
		KindCredentialReference:
		return true
	default:
		return false
	}
}

func scopeAllows(target Target, artifacts map[string]struct{}, item ItemInput) bool {
	switch item.Scope {
	case ScopeAgentPrivate:
		return item.AllowedAgentID == target.AgentID
	case ScopeRoleRestricted:
		return item.AllowedRoleID == target.RoleID
	case ScopeArtifactScoped:
		_, ok := artifacts[item.ArtifactRef]
		return ok
	default:
		return true
	}
}

func validTrust(value TrustClass) bool {
	switch value {
	case TrustAuthoritative, TrustObserved, TrustUntrusted:
		return true
	default:
		return false
	}
}

func validScope(value Scope) bool {
	switch value {
	case ScopeConversationShared, ScopeTeamShared, ScopeAgentPrivate,
		ScopeRoleRestricted, ScopeArtifactScoped, ScopeSecretReferenceOnly:
		return true
	default:
		return false
	}
}

func validIdentifier(value string) bool {
	return value != "" && value == strings.TrimSpace(value) &&
		len(value) <= 512 && !strings.ContainsAny(value, "\x00\r\n")
}

func validRouteTarget(providerID string, providerAccountID string, modelID string, authMode string) bool {
	if !validIdentifier(authMode) {
		return false
	}
	if providerID == "" && providerAccountID == "" && modelID == "" {
		return authMode == "native_auth"
	}
	if !validIdentifier(providerID) || !validIdentifier(modelID) {
		return false
	}
	if providerAccountID == "" {
		return authMode == "native_auth"
	}
	return validIdentifier(providerAccountID)
}

func digestBytes(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func validDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
