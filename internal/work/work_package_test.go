package work

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

const (
	codingWorkPackageDigest    = "4eea514fca13aa241cd004277e31c9c1fe296d34646ceafd618809b6b22c8a4f"
	knowledgeWorkPackageDigest = "5a834baad4a0e4c557d96883f242860f4d136de208dd6721b85d8c2b8c5a0393"
)

func TestWorkPackageBuiltInsHaveExactCanonicalIdentity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		build  func() (WorkPackage, error)
		id     string
		domain WorkPackageDomain
		digest string
		json   string
	}{
		{
			name:   "coding",
			build:  CodingWorkPackage,
			id:     "work-package.coding",
			domain: WorkPackageCoding,
			digest: codingWorkPackageDigest,
			json:   `{"schema_version":1,"id":"work-package.coding","version":1,"digest":"4eea514fca13aa241cd004277e31c9c1fe296d34646ceafd618809b6b22c8a4f","domain_kind":"coding","recommended_agent_definition_ids":["agent.coding.main","agent.coding.reviewer","agent.coding.test"],"tool_categories":["artifact.read","repository.read","workspace.edit","workspace.test"],"default_verifier_key":"verifier.code-review.v1","evidence_types":["code_patch","test_report","verification_report"],"default_customer_rule_template_ids":["rule.template.coding-review","rule.template.protected-write"]}`,
		},
		{
			name:   "knowledge",
			build:  KnowledgeWorkPackage,
			id:     "work-package.knowledge",
			domain: WorkPackageKnowledge,
			digest: knowledgeWorkPackageDigest,
			json:   `{"schema_version":1,"id":"work-package.knowledge","version":1,"digest":"5a834baad4a0e4c557d96883f242860f4d136de208dd6721b85d8c2b8c5a0393","domain_kind":"knowledge_work","recommended_agent_definition_ids":["agent.knowledge.main","agent.knowledge.research","agent.knowledge.verifier"],"tool_categories":["artifact.read","document.read","research.local","workspace.edit"],"default_verifier_key":"verifier.source-review.v1","evidence_types":["analysis_report","source_index","verification_report"],"default_customer_rule_template_ids":["rule.template.human-publication","rule.template.source-attribution"]}`,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			value, err := test.build()
			if err != nil {
				t.Fatal(err)
			}
			if value.SchemaVersion() != WorkPackageSchemaVersion ||
				value.ID() != test.id ||
				value.Version() != 1 ||
				value.DomainKind() != test.domain ||
				value.Digest() != test.digest {
				t.Fatalf("unexpected identity: %#v", value)
			}
			encoded, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if string(encoded) != test.json {
				t.Fatalf("JSON mismatch:\n got: %s\nwant: %s", encoded, test.json)
			}
			if len(encoded) > MaxWorkPackageEncodedBytes {
				t.Fatalf("encoded bytes = %d", len(encoded))
			}
		})
	}
}

func TestWorkPackageInputOrderDoesNotChangeIdentity(t *testing.T) {
	base := codingInput()
	expected, err := NewWorkPackage(base)
	if err != nil {
		t.Fatal(err)
	}
	random := rand.New(rand.NewSource(20260726))
	for iteration := 0; iteration < 100; iteration++ {
		input := base
		input.RecommendedAgentDefinitionIDs = shuffled(random, base.RecommendedAgentDefinitionIDs)
		input.ToolCategories = shuffled(random, base.ToolCategories)
		input.EvidenceTypes = shuffled(random, base.EvidenceTypes)
		input.DefaultCustomerRuleTemplateIDs = shuffled(
			random,
			base.DefaultCustomerRuleTemplateIDs,
		)
		value, err := NewWorkPackage(input)
		if err != nil {
			t.Fatalf("iteration %d: %v", iteration, err)
		}
		if value.Digest() != expected.Digest() {
			t.Fatalf("iteration %d digest = %s", iteration, value.Digest())
		}
	}
}

func TestWorkPackageCopiesInputAndAccessors(t *testing.T) {
	input := codingInput()
	value, err := NewWorkPackage(input)
	if err != nil {
		t.Fatal(err)
	}
	input.RecommendedAgentDefinitionIDs[0] = "agent.mutated"
	input.ToolCategories[0] = "credential.read"
	input.EvidenceTypes[0] = "raw_prompt"
	input.DefaultCustomerRuleTemplateIDs[0] = "rule.mutated"
	if value.Digest() != codingWorkPackageDigest {
		t.Fatalf("digest changed to %s", value.Digest())
	}

	mutations := [][]string{
		value.RecommendedAgentDefinitionIDs(),
		value.ToolCategories(),
		value.EvidenceTypes(),
		value.DefaultCustomerRuleTemplateIDs(),
	}
	for _, values := range mutations {
		values[0] = "mutated"
	}
	if value.RecommendedAgentDefinitionIDs()[0] != "agent.coding.main" ||
		value.ToolCategories()[0] != "artifact.read" ||
		value.EvidenceTypes()[0] != "code_patch" ||
		value.DefaultCustomerRuleTemplateIDs()[0] != "rule.template.coding-review" {
		t.Fatal("an accessor exposed mutable storage")
	}

	first, err := CodingWorkPackage()
	if err != nil {
		t.Fatal(err)
	}
	second, err := CodingWorkPackage()
	if err != nil {
		t.Fatal(err)
	}
	firstAgents := first.RecommendedAgentDefinitionIDs()
	firstAgents[0] = "agent.mutated"
	if reflect.DeepEqual(firstAgents, second.RecommendedAgentDefinitionIDs()) {
		t.Fatal("built-in calls share caller-mutable storage")
	}
}

func TestWorkPackageValidationAndTypedErrors(t *testing.T) {
	base := codingInput()
	tests := []struct {
		name   string
		mutate func(*WorkPackageInput)
	}{
		{"empty id", func(input *WorkPackageInput) { input.ID = "" }},
		{"leading digit", func(input *WorkPackageInput) { input.ID = "1bad" }},
		{"uppercase", func(input *WorkPackageInput) { input.ID = "Bad" }},
		{"space", func(input *WorkPackageInput) { input.ID = "bad id" }},
		{"unicode", func(input *WorkPackageInput) { input.ID = "badé" }},
		{"control", func(input *WorkPackageInput) { input.ID = "bad\nid" }},
		{"oversized id", func(input *WorkPackageInput) { input.ID = "a" + strings.Repeat("b", 128) }},
		{"zero version", func(input *WorkPackageInput) { input.Version = 0 }},
		{"oversized version", func(input *WorkPackageInput) { input.Version = 1_000_001 }},
		{"unknown domain", func(input *WorkPackageInput) { input.DomainKind = "other" }},
		{"nil agents", func(input *WorkPackageInput) { input.RecommendedAgentDefinitionIDs = nil }},
		{"too many agents", func(input *WorkPackageInput) {
			input.RecommendedAgentDefinitionIDs = []string{"agent.a", "agent.b", "agent.c", "agent.d"}
		}},
		{"duplicate agents", func(input *WorkPackageInput) {
			input.RecommendedAgentDefinitionIDs = []string{"agent.a", "agent.a"}
		}},
		{"empty tools", func(input *WorkPackageInput) { input.ToolCategories = nil }},
		{"duplicate tools", func(input *WorkPackageInput) {
			input.ToolCategories = []string{"artifact.read", "artifact.read"}
		}},
		{"oversized tool", func(input *WorkPackageInput) {
			input.ToolCategories = []string{"a" + strings.Repeat("b", 64)}
		}},
		{"empty verifier", func(input *WorkPackageInput) { input.DefaultVerifierKey = "" }},
		{"empty evidence", func(input *WorkPackageInput) { input.EvidenceTypes = nil }},
		{"duplicate evidence", func(input *WorkPackageInput) {
			input.EvidenceTypes = []string{"test_report", "test_report"}
		}},
		{"empty rules", func(input *WorkPackageInput) {
			input.DefaultCustomerRuleTemplateIDs = nil
		}},
		{"duplicate rules", func(input *WorkPackageInput) {
			input.DefaultCustomerRuleTemplateIDs = []string{"rule.a", "rule.a"}
		}},
		{"encoded package exceeds bound", func(input *WorkPackageInput) {
			input.RecommendedAgentDefinitionIDs = boundedIdentifiers("agent", 3, 128)
			input.ToolCategories = boundedIdentifiers("tool", 16, 64)
			input.EvidenceTypes = boundedIdentifiers("evidence", 16, 64)
			input.DefaultCustomerRuleTemplateIDs = boundedIdentifiers("rule", 16, 128)
		}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			input := cloneInput(base)
			test.mutate(&input)
			value, err := NewWorkPackage(input)
			if !errors.Is(err, ErrInvalidWorkPackage) {
				t.Fatalf("error = %v", err)
			}
			if !reflect.DeepEqual(value, WorkPackage{}) {
				t.Fatalf("partial value returned: %#v", value)
			}
		})
	}

	valid, err := NewWorkPackage(base)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadWorkPackage(base, valid.Digest())
	if err != nil || loaded.Digest() != valid.Digest() {
		t.Fatalf("valid load = %#v, %v", loaded, err)
	}
	loaded, err = LoadWorkPackage(base, strings.Repeat("0", 64))
	if !errors.Is(err, ErrWorkPackageDigestMismatch) {
		t.Fatalf("mismatch error = %v", err)
	}
	if !reflect.DeepEqual(loaded, WorkPackage{}) {
		t.Fatalf("partial mismatch value returned: %#v", loaded)
	}
	invalid := cloneInput(base)
	invalid.ID = ""
	_, err = LoadWorkPackage(invalid, valid.Digest())
	if !errors.Is(err, ErrInvalidWorkPackage) ||
		errors.Is(err, ErrWorkPackageDigestMismatch) {
		t.Fatalf("invalid load error = %v", err)
	}
}

func TestWorkPackageZeroValueMarshalFailsClosed(t *testing.T) {
	var value WorkPackage
	_, err := value.MarshalJSON()
	if !errors.Is(err, ErrInvalidWorkPackage) {
		t.Fatalf("MarshalJSON error = %v", err)
	}
	_, err = json.Marshal(value)
	if !errors.Is(err, ErrInvalidWorkPackage) {
		t.Fatalf("json.Marshal error = %v", err)
	}
}

func codingInput() WorkPackageInput {
	return WorkPackageInput{
		ID:         "work-package.coding",
		Version:    1,
		DomainKind: WorkPackageCoding,
		RecommendedAgentDefinitionIDs: []string{
			"agent.coding.test",
			"agent.coding.main",
			"agent.coding.reviewer",
		},
		ToolCategories: []string{
			"workspace.test",
			"artifact.read",
			"workspace.edit",
			"repository.read",
		},
		DefaultVerifierKey: "verifier.code-review.v1",
		EvidenceTypes: []string{
			"verification_report",
			"code_patch",
			"test_report",
		},
		DefaultCustomerRuleTemplateIDs: []string{
			"rule.template.protected-write",
			"rule.template.coding-review",
		},
	}
}

func cloneInput(input WorkPackageInput) WorkPackageInput {
	input.RecommendedAgentDefinitionIDs = append(
		[]string(nil),
		input.RecommendedAgentDefinitionIDs...,
	)
	input.ToolCategories = append([]string(nil), input.ToolCategories...)
	input.EvidenceTypes = append([]string(nil), input.EvidenceTypes...)
	input.DefaultCustomerRuleTemplateIDs = append(
		[]string(nil),
		input.DefaultCustomerRuleTemplateIDs...,
	)
	return input
}

func shuffled(random *rand.Rand, values []string) []string {
	result := append([]string(nil), values...)
	random.Shuffle(len(result), func(first, second int) {
		result[first], result[second] = result[second], result[first]
	})
	return result
}

func boundedIdentifiers(prefix string, count, bytes int) []string {
	result := make([]string, count)
	for index := range result {
		suffix := fmt.Sprintf("%02d", index)
		result[index] = prefix + strings.Repeat(
			"x",
			bytes-len(prefix)-len(suffix),
		) + suffix
	}
	return result
}
