package work

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

var (
	ErrInvalidWorkPackage        = errors.New("invalid WorkPackage")
	ErrWorkPackageDigestMismatch = errors.New("WorkPackage digest mismatch")
)

type WorkPackageDomain string

const (
	WorkPackageCoding              WorkPackageDomain = "coding"
	WorkPackageKnowledge           WorkPackageDomain = "knowledge_work"
	WorkPackageSchemaVersion                         = 1
	MaxWorkPackageEncodedBytes                       = 4096
	MaxWorkPackageAgentDefinitions                   = 3
	MaxWorkPackageToolCategories                     = 16
	MaxWorkPackageEvidenceTypes                      = 16
	MaxWorkPackageRuleTemplates                      = 16
)

type WorkPackageInput struct {
	ID                             string
	Version                        int
	DomainKind                     WorkPackageDomain
	RecommendedAgentDefinitionIDs  []string
	ToolCategories                 []string
	DefaultVerifierKey             string
	EvidenceTypes                  []string
	DefaultCustomerRuleTemplateIDs []string
}

type WorkPackage struct {
	id                             string
	version                        int
	digest                         string
	domainKind                     WorkPackageDomain
	recommendedAgentDefinitionIDs  []string
	toolCategories                 []string
	defaultVerifierKey             string
	evidenceTypes                  []string
	defaultCustomerRuleTemplateIDs []string
}

type canonicalWorkPackage struct {
	SchemaVersion                  int               `json:"schema_version"`
	ID                             string            `json:"id"`
	Version                        int               `json:"version"`
	DomainKind                     WorkPackageDomain `json:"domain_kind"`
	RecommendedAgentDefinitionIDs  []string          `json:"recommended_agent_definition_ids"`
	ToolCategories                 []string          `json:"tool_categories"`
	DefaultVerifierKey             string            `json:"default_verifier_key"`
	EvidenceTypes                  []string          `json:"evidence_types"`
	DefaultCustomerRuleTemplateIDs []string          `json:"default_customer_rule_template_ids"`
}

type encodedWorkPackage struct {
	SchemaVersion                  int               `json:"schema_version"`
	ID                             string            `json:"id"`
	Version                        int               `json:"version"`
	Digest                         string            `json:"digest"`
	DomainKind                     WorkPackageDomain `json:"domain_kind"`
	RecommendedAgentDefinitionIDs  []string          `json:"recommended_agent_definition_ids"`
	ToolCategories                 []string          `json:"tool_categories"`
	DefaultVerifierKey             string            `json:"default_verifier_key"`
	EvidenceTypes                  []string          `json:"evidence_types"`
	DefaultCustomerRuleTemplateIDs []string          `json:"default_customer_rule_template_ids"`
}

func NewWorkPackage(input WorkPackageInput) (WorkPackage, error) {
	agents, err := normalizedIdentifiers(
		input.RecommendedAgentDefinitionIDs,
		128,
		MaxWorkPackageAgentDefinitions,
	)
	if err != nil {
		return WorkPackage{}, err
	}
	tools, err := normalizedIdentifiers(
		input.ToolCategories,
		64,
		MaxWorkPackageToolCategories,
	)
	if err != nil {
		return WorkPackage{}, err
	}
	evidenceTypes, err := normalizedIdentifiers(
		input.EvidenceTypes,
		64,
		MaxWorkPackageEvidenceTypes,
	)
	if err != nil {
		return WorkPackage{}, err
	}
	rules, err := normalizedIdentifiers(
		input.DefaultCustomerRuleTemplateIDs,
		128,
		MaxWorkPackageRuleTemplates,
	)
	if err != nil {
		return WorkPackage{}, err
	}
	if !validWorkPackageIdentifier(input.ID, 128) ||
		!validWorkPackageIdentifier(input.DefaultVerifierKey, 128) ||
		input.Version < 1 ||
		input.Version > 1_000_000 ||
		(input.DomainKind != WorkPackageCoding &&
			input.DomainKind != WorkPackageKnowledge) {
		return WorkPackage{}, ErrInvalidWorkPackage
	}

	value := WorkPackage{
		id:                             input.ID,
		version:                        input.Version,
		domainKind:                     input.DomainKind,
		recommendedAgentDefinitionIDs:  agents,
		toolCategories:                 tools,
		defaultVerifierKey:             input.DefaultVerifierKey,
		evidenceTypes:                  evidenceTypes,
		defaultCustomerRuleTemplateIDs: rules,
	}
	canonical, err := json.Marshal(value.canonical())
	if err != nil {
		return WorkPackage{}, fmt.Errorf("%w: canonical encoding: %v", ErrInvalidWorkPackage, err)
	}
	sum := sha256.Sum256(canonical)
	value.digest = hex.EncodeToString(sum[:])
	encoded, err := value.marshalValidated()
	if err != nil {
		return WorkPackage{}, err
	}
	if len(encoded) > MaxWorkPackageEncodedBytes {
		return WorkPackage{}, ErrInvalidWorkPackage
	}
	return value, nil
}

func LoadWorkPackage(input WorkPackageInput, digest string) (WorkPackage, error) {
	value, err := NewWorkPackage(input)
	if err != nil {
		return WorkPackage{}, err
	}
	if digest != value.digest {
		return WorkPackage{}, fmt.Errorf(
			"%w: got %q",
			ErrWorkPackageDigestMismatch,
			digest,
		)
	}
	return value, nil
}

func CodingWorkPackage() (WorkPackage, error) {
	return NewWorkPackage(WorkPackageInput{
		ID:         "work-package.coding",
		Version:    1,
		DomainKind: WorkPackageCoding,
		RecommendedAgentDefinitionIDs: []string{
			"agent.coding.main",
			"agent.coding.reviewer",
			"agent.coding.test",
		},
		ToolCategories: []string{
			"artifact.read",
			"repository.read",
			"workspace.edit",
			"workspace.test",
		},
		DefaultVerifierKey: "verifier.code-review.v1",
		EvidenceTypes: []string{
			"code_patch",
			"test_report",
			"verification_report",
		},
		DefaultCustomerRuleTemplateIDs: []string{
			"rule.template.coding-review",
			"rule.template.protected-write",
		},
	})
}

func KnowledgeWorkPackage() (WorkPackage, error) {
	return NewWorkPackage(WorkPackageInput{
		ID:         "work-package.knowledge",
		Version:    1,
		DomainKind: WorkPackageKnowledge,
		RecommendedAgentDefinitionIDs: []string{
			"agent.knowledge.main",
			"agent.knowledge.research",
			"agent.knowledge.verifier",
		},
		ToolCategories: []string{
			"artifact.read",
			"document.read",
			"research.local",
			"workspace.edit",
		},
		DefaultVerifierKey: "verifier.source-review.v1",
		EvidenceTypes: []string{
			"analysis_report",
			"source_index",
			"verification_report",
		},
		DefaultCustomerRuleTemplateIDs: []string{
			"rule.template.human-publication",
			"rule.template.source-attribution",
		},
	})
}

func (value WorkPackage) SchemaVersion() int {
	return WorkPackageSchemaVersion
}

func (value WorkPackage) ID() string {
	return value.id
}

func (value WorkPackage) Version() int {
	return value.version
}

func (value WorkPackage) Digest() string {
	return value.digest
}

func (value WorkPackage) DomainKind() WorkPackageDomain {
	return value.domainKind
}

func (value WorkPackage) RecommendedAgentDefinitionIDs() []string {
	return cloneStrings(value.recommendedAgentDefinitionIDs)
}

func (value WorkPackage) ToolCategories() []string {
	return cloneStrings(value.toolCategories)
}

func (value WorkPackage) DefaultVerifierKey() string {
	return value.defaultVerifierKey
}

func (value WorkPackage) EvidenceTypes() []string {
	return cloneStrings(value.evidenceTypes)
}

func (value WorkPackage) DefaultCustomerRuleTemplateIDs() []string {
	return cloneStrings(value.defaultCustomerRuleTemplateIDs)
}

func (value WorkPackage) MarshalJSON() ([]byte, error) {
	if !value.valid() {
		return nil, ErrInvalidWorkPackage
	}
	return value.marshalValidated()
}

func (value WorkPackage) canonical() canonicalWorkPackage {
	return canonicalWorkPackage{
		SchemaVersion:                  WorkPackageSchemaVersion,
		ID:                             value.id,
		Version:                        value.version,
		DomainKind:                     value.domainKind,
		RecommendedAgentDefinitionIDs:  cloneStrings(value.recommendedAgentDefinitionIDs),
		ToolCategories:                 cloneStrings(value.toolCategories),
		DefaultVerifierKey:             value.defaultVerifierKey,
		EvidenceTypes:                  cloneStrings(value.evidenceTypes),
		DefaultCustomerRuleTemplateIDs: cloneStrings(value.defaultCustomerRuleTemplateIDs),
	}
}

func (value WorkPackage) marshalValidated() ([]byte, error) {
	return json.Marshal(encodedWorkPackage{
		SchemaVersion:                  WorkPackageSchemaVersion,
		ID:                             value.id,
		Version:                        value.version,
		Digest:                         value.digest,
		DomainKind:                     value.domainKind,
		RecommendedAgentDefinitionIDs:  cloneStrings(value.recommendedAgentDefinitionIDs),
		ToolCategories:                 cloneStrings(value.toolCategories),
		DefaultVerifierKey:             value.defaultVerifierKey,
		EvidenceTypes:                  cloneStrings(value.evidenceTypes),
		DefaultCustomerRuleTemplateIDs: cloneStrings(value.defaultCustomerRuleTemplateIDs),
	})
}

func (value WorkPackage) valid() bool {
	if value.digest == "" {
		return false
	}
	rebuilt, err := NewWorkPackage(WorkPackageInput{
		ID:                             value.id,
		Version:                        value.version,
		DomainKind:                     value.domainKind,
		RecommendedAgentDefinitionIDs:  value.recommendedAgentDefinitionIDs,
		ToolCategories:                 value.toolCategories,
		DefaultVerifierKey:             value.defaultVerifierKey,
		EvidenceTypes:                  value.evidenceTypes,
		DefaultCustomerRuleTemplateIDs: value.defaultCustomerRuleTemplateIDs,
	})
	return err == nil && rebuilt.digest == value.digest
}

func normalizedIdentifiers(
	values []string,
	maxBytes int,
	maxValues int,
) ([]string, error) {
	if len(values) == 0 || len(values) > maxValues {
		return nil, ErrInvalidWorkPackage
	}
	normalized := cloneStrings(values)
	sort.Strings(normalized)
	for index, value := range normalized {
		if !validWorkPackageIdentifier(value, maxBytes) ||
			(index > 0 && normalized[index-1] == value) {
			return nil, ErrInvalidWorkPackage
		}
	}
	return normalized, nil
}

func validWorkPackageIdentifier(value string, maxBytes int) bool {
	if len(value) == 0 || len(value) > maxBytes ||
		value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for _, character := range []byte(value) {
		if (character >= 'a' && character <= 'z') ||
			(character >= '0' && character <= '9') ||
			character == '.' ||
			character == '_' ||
			character == '-' {
			continue
		}
		return false
	}
	return true
}

func cloneStrings(values []string) []string {
	return append([]string(nil), values...)
}
