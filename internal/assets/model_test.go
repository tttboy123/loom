package assets_test

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"loom-pi-rebuild/internal/assets"
)

func TestP3AAssetWireModelsUseExactSnakeCaseAndOmitUnknownMeasurements(t *testing.T) {
	candidate, err := json.Marshal(assets.EvolutionCandidate{
		CandidateID: "candidate-1", AssetKind: assets.AssetKindSkill,
		DefinitionID: "skill-1", RevisionID: "revision-1",
		SourceScope:       assets.SourceScopeLocal,
		SourceEvidenceIDs: []string{}, SourceEvidenceDigests: []string{},
		RequiredEvaluationIDs: []string{}, Risk: assets.RiskLow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(candidate, []byte(`"candidate_id":"candidate-1"`)) ||
		bytes.Contains(candidate, []byte("CandidateID")) {
		t.Fatalf("candidate wire = %s", candidate)
	}
	evaluation, err := json.Marshal(assets.EvaluationRecord{
		EvaluationID: "evaluation-1", CandidateID: "candidate-1",
		FixtureKind: "synthetic", FixtureDigest: digestA,
		BaselineRevisionID: "revision-0", BaselineDigest: digestB,
		CandidateRevisionID: "revision-1", CandidateDigest: digestA,
		QualityResult: "pass", CaseCount: 1,
		UsageObserved: false, CostObserved: false,
		CompatibilityResult: "pass", ApplicableScope: "project",
		RegressionResult: "pass", SecurityResult: "pass",
		EvidenceID: "evidence-1", EvidenceDigest: digestA,
	})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(evaluation, []byte("usage_value")) ||
		bytes.Contains(evaluation, []byte("cost_value")) {
		t.Fatalf("unknown measurement encoded as value: %s", evaluation)
	}
}

func TestP3AAssetDomainDeclaresReviewedAuthoritySurface(t *testing.T) {
	required := map[string][]string{
		"model.go": {
			"SkillDefinition", "SkillRevision", "EvolutionCandidate",
			"EvaluationRecord", "ExactAssetRevisionBinding",
			"EvolutionAssetBindingRecord",
		},
		"authority.go": {
			"Authority", "BindingSubjectResolver", "Command", "Result",
		},
		"canonical.go": {
			"CanonicalIntentDigest", "CanonicalAssetRevisionSetDigest",
			"CanonicalSubjectIdentityDigest",
		},
		"replay.go": {"Replay"},
	}
	for file, names := range required {
		parsed, err := parser.ParseFile(
			token.NewFileSet(), filepath.Join(".", file), nil, parser.SkipObjectResolution,
		)
		if err != nil {
			t.Fatalf("parse required asset source %s: %v", file, err)
		}
		declared := make(map[string]bool)
		for _, declaration := range parsed.Decls {
			switch value := declaration.(type) {
			case *ast.GenDecl:
				for _, specification := range value.Specs {
					if named, ok := specification.(*ast.TypeSpec); ok {
						declared[named.Name.Name] = true
					}
				}
			case *ast.FuncDecl:
				declared[value.Name.Name] = true
			}
		}
		for _, name := range names {
			if !declared[name] {
				t.Fatalf("%s declaration %s is missing", file, name)
			}
		}
	}
}
