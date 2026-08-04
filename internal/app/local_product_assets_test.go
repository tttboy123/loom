package app_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/assets"
	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"

	_ "modernc.org/sqlite"
)

func TestP3AApplicationServiceDeclaresProductionAssetJourney(t *testing.T) {
	parsed, err := parser.ParseFile(
		token.NewFileSet(), "local_product_assets.go", nil, parser.SkipObjectResolution,
	)
	if err != nil {
		t.Fatalf("parse required asset application service: %v", err)
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
	for _, name := range []string{
		"LocalProductAssetService",
		"NewLocalProductAssetService",
		"ReadEvolutionAssetSnapshot",
		"DiffEvolutionAssetRevisions",
		"CommitEvolutionAssetCommand",
	} {
		if !declared[name] {
			t.Fatalf("asset application declaration %s is missing", name)
		}
	}
}

func TestP3AApplicationServiceEnforcesExactActionInputAndEmptyArrayWire(t *testing.T) {
	service, store, artifacts, sourcePath, artifactDigest, contentDigest := newP3AAssetService(t)
	journeyID := "123e4567-e89b-42d3-a456-426614174000"
	before, err := service.ReadEvolutionAssetSnapshot(context.Background(), app.EvolutionAssetSnapshotRequest{JourneyID: journeyID, Limit: 64})
	if err != nil {
		t.Fatal(err)
	}
	wire, err := json.Marshal(before)
	if err != nil {
		t.Fatal(err)
	}
	subjects, err := json.Marshal(before.BindingSubjects)
	if err != nil {
		t.Fatal(err)
	}
	promotionSources, err := json.Marshal(before.PromotionSources)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"view_version":"` + before.ViewVersion + `","next_cursor":"","definitions":[],"revisions":[],"candidates":[],"evaluations":[],"bindings":[],"materializations":[],"binding_subjects":` + string(subjects) + `,"promotion_sources":` + string(promotionSources) + `}`
	if string(wire) != want {
		t.Fatalf("empty snapshot wire = %s, want %s", wire, want)
	}
	input, err := json.Marshal(struct {
		DefinitionID                  string   `json:"definition_id"`
		RevisionID                    string   `json:"revision_id"`
		Name                          string   `json:"name"`
		Description                   string   `json:"description"`
		SubjectScope                  string   `json:"subject_scope"`
		SourcePath                    string   `json:"source_path"`
		SuppliedArtifactDigest        string   `json:"supplied_artifact_digest"`
		SuppliedContentDigest         string   `json:"supplied_content_digest"`
		Dependencies                  []string `json:"dependencies"`
		CompatibleRuntimeCapabilities []string `json:"compatible_runtime_capabilities"`
		Risk                          string   `json:"risk"`
	}{"skill-app", "revision-1", "App Skill", "fixture", "project", sourcePath, artifactDigest, contentDigest, []string{}, []string{}, "low"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.CommitEvolutionAssetCommand(context.Background(), app.EvolutionAssetCommandRequest{JourneyID: journeyID, OperationID: "create-app-skill", Action: "create_skill", ExpectedViewVersion: before.ViewVersion, ExpectedStreamHeads: []journal.StreamHead{}, Input: input})
	if err != nil {
		t.Fatal(err)
	}
	if result.OperationID != "create-app-skill" || result.Action != "create_skill" || len(result.EventIDs) != 3 || result.DefinitionID != "skill-app" || result.CandidateID == "" {
		t.Fatalf("result = %#v", result)
	}
	afterCreate, err := service.ReadEvolutionAssetSnapshot(context.Background(), app.EvolutionAssetSnapshotRequest{JourneyID: journeyID, Limit: 64})
	if err != nil {
		t.Fatal(err)
	}
	afterCreateWire, err := json.Marshal(afterCreate)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(afterCreateWire, []byte(":null")) {
		t.Fatalf("authoritative asset snapshot contains null collection: %s", afterCreateWire)
	}
	if len(afterCreate.Revisions) != 1 || len(afterCreate.Revisions[0].Dependencies) != 0 ||
		afterCreate.Revisions[0].Dependencies == nil ||
		afterCreate.Revisions[0].CompatibleRuntimeCapabilities == nil ||
		len(afterCreate.Candidates) != 1 || afterCreate.Candidates[0].SourceEvidenceIDs == nil ||
		afterCreate.Candidates[0].SourceEvidenceDigests == nil ||
		afterCreate.Candidates[0].RequiredEvaluationIDs == nil {
		t.Fatalf("nested collection normalization = %#v %#v", afterCreate.Revisions, afterCreate.Candidates)
	}
	published, err := artifacts.ReadArtifact(context.Background(), artifactDigest, 1572864)
	if err != nil || sha256HexForTest(published) != artifactDigest {
		t.Fatalf("published canonical Artifact digest=%s error=%v", sha256HexForTest(published), err)
	}
	if artifactDigest != "a75161eb8a32f0517511033e4fc8c7948748dad21524498b7af91da29c3eba6e" ||
		contentDigest != "df76251cc14686fab4dccc638295cfaa8b1a32622752e8b2c337e2138c3c95d5" {
		t.Fatalf("canonical Artifact digests = %s %s", artifactDigest, contentDigest)
	}
	bad := append([]byte(nil), input[:len(input)-1]...)
	bad = append(bad, []byte(`,"unknown":true}`)...)
	if _, err := service.CommitEvolutionAssetCommand(context.Background(), app.EvolutionAssetCommandRequest{JourneyID: journeyID, OperationID: "bad-app-skill", Action: "create_skill", ExpectedViewVersion: result.ViewVersion, ExpectedStreamHeads: []journal.StreamHead{}, Input: bad}); !errors.Is(err, app.ErrInvalidLocalProductAsset) {
		t.Fatalf("unknown input error = %v", err)
	}
	events, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("events after rejected unknown field = %d", len(events))
	}
	caseIDs := []string{"artifact_digest", "runtime_compatibility", "security_boundary"}
	fixture, err := json.Marshal(struct {
		SchemaVersion int      `json:"schema_version"`
		FixtureKind   string   `json:"fixture_kind"`
		CaseIDs       []string `json:"case_ids"`
		Expected      struct {
			QualityResult       string `json:"quality_result"`
			FailureCount        int    `json:"failure_count"`
			UsageObserved       bool   `json:"usage_observed"`
			UsageMicrounits     int64  `json:"usage_microunits"`
			CostObserved        bool   `json:"cost_observed"`
			CostMicrounits      int64  `json:"cost_microunits"`
			CostCurrency        string `json:"cost_currency"`
			CompatibilityResult string `json:"compatibility_result"`
			ApplicableScope     string `json:"applicable_scope"`
			RegressionResult    string `json:"regression_result"`
			SecurityResult      string `json:"security_result"`
		} `json:"expected"`
	}{
		1, "synthetic", caseIDs, struct {
			QualityResult       string `json:"quality_result"`
			FailureCount        int    `json:"failure_count"`
			UsageObserved       bool   `json:"usage_observed"`
			UsageMicrounits     int64  `json:"usage_microunits"`
			CostObserved        bool   `json:"cost_observed"`
			CostMicrounits      int64  `json:"cost_microunits"`
			CostCurrency        string `json:"cost_currency"`
			CompatibilityResult string `json:"compatibility_result"`
			ApplicableScope     string `json:"applicable_scope"`
			RegressionResult    string `json:"regression_result"`
			SecurityResult      string `json:"security_result"`
		}{
			"pass", 0, true, 250, true, 1250, "USD",
			"compatible", "bounded_fixture", "equivalent", "pass",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	fixtureDigest := sha256HexForTest(fixture)
	if _, err := artifacts.Publish(context.Background(), bytes.NewReader(fixture), fixtureDigest); err != nil {
		t.Fatalf("publish evaluation fixture: %v", err)
	}
	evaluationInput, err := json.Marshal(struct {
		EvaluationID       string   `json:"evaluation_id"`
		CandidateID        string   `json:"candidate_id"`
		FixtureKind        string   `json:"fixture_kind"`
		FixtureDigest      string   `json:"fixture_digest"`
		BaselineRevisionID string   `json:"baseline_revision_id"`
		BaselineDigest     string   `json:"baseline_digest"`
		CandidateRevision  string   `json:"candidate_revision_id"`
		CandidateDigest    string   `json:"candidate_digest"`
		RequestedCaseIDs   []string `json:"requested_case_ids"`
	}{"evaluation-1", result.CandidateID, "synthetic", fixtureDigest, "revision-1", artifactDigest, "revision-1", artifactDigest, caseIDs})
	if err != nil {
		t.Fatal(err)
	}
	evaluated, err := service.CommitEvolutionAssetCommand(context.Background(), app.EvolutionAssetCommandRequest{
		JourneyID: journeyID, OperationID: "evaluate-app-skill", Action: "record_evaluation",
		ExpectedViewVersion: result.ViewVersion, ExpectedStreamHeads: []journal.StreamHead{}, Input: evaluationInput,
	})
	if err != nil || evaluated.EvaluationID != "evaluation-1" {
		t.Fatalf("record evaluation = %#v, %v", evaluated, err)
	}
	var drift map[string]any
	if err := json.Unmarshal(evaluationInput, &drift); err != nil {
		t.Fatal(err)
	}
	drift["fixture_digest"] = strings.Repeat("f", 64)
	driftInput, _ := json.Marshal(drift)
	if _, err := service.CommitEvolutionAssetCommand(context.Background(), app.EvolutionAssetCommandRequest{
		JourneyID: journeyID, OperationID: "evaluate-app-skill-drift", Action: "record_evaluation",
		ExpectedViewVersion: evaluated.ViewVersion, ExpectedStreamHeads: []journal.StreamHead{}, Input: driftInput,
	}); !errors.Is(err, assets.ErrDigestMismatch) {
		t.Fatalf("fixture digest drift error = %v", err)
	}
	activateInput, err := json.Marshal(struct {
		CandidateID                string           `json:"candidate_id"`
		AssetKind                  assets.AssetKind `json:"asset_kind"`
		DefinitionID               string           `json:"definition_id"`
		RevisionID                 string           `json:"revision_id"`
		RevisionDigest             string           `json:"revision_digest"`
		ExpectedPreviousRevisionID string           `json:"expected_previous_revision_id"`
		EvaluationIDs              []string         `json:"evaluation_ids"`
	}{result.CandidateID, assets.AssetKindSkill, "skill-app", "revision-1", artifactDigest, "", []string{"evaluation-1"}})
	if err != nil {
		t.Fatal(err)
	}
	activated, err := service.CommitEvolutionAssetCommand(context.Background(), app.EvolutionAssetCommandRequest{
		JourneyID: journeyID, OperationID: "activate-app-skill", Action: "activate",
		ExpectedViewVersion: evaluated.ViewVersion, ExpectedStreamHeads: []journal.StreamHead{}, Input: activateInput,
	})
	if err != nil {
		t.Fatal(err)
	}
	activeSnapshot, err := service.ReadEvolutionAssetSnapshot(context.Background(), app.EvolutionAssetSnapshotRequest{JourneyID: journeyID, Limit: 64})
	if err != nil || len(activeSnapshot.Revisions) != 1 || activeSnapshot.Revisions[0].Lifecycle != assets.LifecycleActive {
		t.Fatalf("active snapshot = %#v, %v", activeSnapshot.Revisions, err)
	}
	subject := activeSnapshot.BindingSubjects[0]
	binding := assets.ExactAssetRevisionBinding{AssetKind: assets.AssetKindSkill, DefinitionID: "skill-app", RevisionID: "revision-1", SHA256Digest: artifactDigest, SourceScope: assets.SourceScopeLocal}
	setDigest, err := assets.CanonicalAssetRevisionSetDigest([]assets.ExactAssetRevisionBinding{binding})
	if err != nil {
		t.Fatal(err)
	}
	bindingInput, err := json.Marshal(struct {
		SubjectKind            string                             `json:"subject_kind"`
		SubjectID              string                             `json:"subject_id"`
		SubjectVersion         int64                              `json:"subject_version"`
		SubjectDigest          string                             `json:"subject_digest"`
		SubjectScope           string                             `json:"subject_scope"`
		SubjectProjectID       string                             `json:"subject_project_id"`
		SubjectGenerationID    string                             `json:"subject_generation_id"`
		SubjectIdentityDigest  string                             `json:"subject_identity_digest"`
		AssetRevisionBindings  []assets.ExactAssetRevisionBinding `json:"asset_revision_bindings"`
		AssetRevisionSetDigest string                             `json:"asset_revision_set_digest"`
	}{subject.SubjectKind, subject.SubjectID, subject.SubjectVersion, subject.SubjectDigest, subject.Scope, subject.ProjectID, subject.GenerationID, subject.SubjectIdentityDigest, []assets.ExactAssetRevisionBinding{binding}, setDigest})
	if err != nil {
		t.Fatal(err)
	}
	bound, err := service.CommitEvolutionAssetCommand(context.Background(), app.EvolutionAssetCommandRequest{
		JourneyID: journeyID, OperationID: "bind-app-skill", Action: "set_binding",
		ExpectedViewVersion: activated.ViewVersion, ExpectedStreamHeads: []journal.StreamHead{}, Input: bindingInput,
	})
	if err != nil || bound.AssetRevisionSetDigest != setDigest {
		t.Fatalf("activate then bind = %#v, %v", bound, err)
	}
}

func TestP3AEvaluationFixtureResultsAreRecordedTruthfully(t *testing.T) {
	service, _, artifacts, sourcePath, artifactDigest, contentDigest := newP3AAssetService(t)
	ctx := context.Background()
	journeyID := "123e4567-e89b-42d3-a456-426614174000"
	before, err := service.ReadEvolutionAssetSnapshot(ctx, app.EvolutionAssetSnapshotRequest{JourneyID: journeyID, Limit: 64})
	if err != nil {
		t.Fatal(err)
	}
	createInput, err := json.Marshal(struct {
		DefinitionID                  string   `json:"definition_id"`
		RevisionID                    string   `json:"revision_id"`
		Name                          string   `json:"name"`
		Description                   string   `json:"description"`
		SubjectScope                  string   `json:"subject_scope"`
		SourcePath                    string   `json:"source_path"`
		SuppliedArtifactDigest        string   `json:"supplied_artifact_digest"`
		SuppliedContentDigest         string   `json:"supplied_content_digest"`
		Dependencies                  []string `json:"dependencies"`
		CompatibleRuntimeCapabilities []string `json:"compatible_runtime_capabilities"`
		Risk                          string   `json:"risk"`
	}{"skill-app", "revision-1", "Evaluated Skill", "fixture", "project", sourcePath, artifactDigest, contentDigest, []string{}, []string{}, "low"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.CommitEvolutionAssetCommand(ctx, app.EvolutionAssetCommandRequest{
		JourneyID: journeyID, OperationID: "create-eval-skill", Action: "create_skill",
		ExpectedViewVersion: before.ViewVersion, ExpectedStreamHeads: []journal.StreamHead{}, Input: createInput,
	})
	if err != nil {
		t.Fatal(err)
	}
	caseIDs := []string{"case.one", "case.two", "case.three"}
	fixture, err := json.Marshal(struct {
		SchemaVersion int      `json:"schema_version"`
		FixtureKind   string   `json:"fixture_kind"`
		CaseIDs       []string `json:"case_ids"`
		Expected      struct {
			QualityResult       string `json:"quality_result"`
			FailureCount        int    `json:"failure_count"`
			UsageObserved       bool   `json:"usage_observed"`
			UsageMicrounits     int64  `json:"usage_microunits"`
			CostObserved        bool   `json:"cost_observed"`
			CostMicrounits      int64  `json:"cost_microunits"`
			CostCurrency        string `json:"cost_currency"`
			CompatibilityResult string `json:"compatibility_result"`
			ApplicableScope     string `json:"applicable_scope"`
			RegressionResult    string `json:"regression_result"`
			SecurityResult      string `json:"security_result"`
		} `json:"expected"`
	}{
		1, "synthetic", caseIDs, struct {
			QualityResult       string `json:"quality_result"`
			FailureCount        int    `json:"failure_count"`
			UsageObserved       bool   `json:"usage_observed"`
			UsageMicrounits     int64  `json:"usage_microunits"`
			CostObserved        bool   `json:"cost_observed"`
			CostMicrounits      int64  `json:"cost_microunits"`
			CostCurrency        string `json:"cost_currency"`
			CompatibilityResult string `json:"compatibility_result"`
			ApplicableScope     string `json:"applicable_scope"`
			RegressionResult    string `json:"regression_result"`
			SecurityResult      string `json:"security_result"`
		}{
			"fail", 2, true, 512, true, 300, "USD",
			"incompatible", "bounded_fixture", "regressed", "partial",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	fixtureDigest := sha256HexForTest(fixture)
	if _, err := artifacts.Publish(ctx, bytes.NewReader(fixture), fixtureDigest); err != nil {
		t.Fatalf("publish failing fixture: %v", err)
	}
	evaluationInput, err := json.Marshal(struct {
		EvaluationID       string   `json:"evaluation_id"`
		CandidateID        string   `json:"candidate_id"`
		FixtureKind        string   `json:"fixture_kind"`
		FixtureDigest      string   `json:"fixture_digest"`
		BaselineRevisionID string   `json:"baseline_revision_id"`
		BaselineDigest     string   `json:"baseline_digest"`
		CandidateRevision  string   `json:"candidate_revision_id"`
		CandidateDigest    string   `json:"candidate_digest"`
		RequestedCaseIDs   []string `json:"requested_case_ids"`
	}{"evaluation-fail", result.CandidateID, "synthetic", fixtureDigest, "revision-1", artifactDigest, "revision-1", artifactDigest, caseIDs})
	if err != nil {
		t.Fatal(err)
	}
	evaluated, err := service.CommitEvolutionAssetCommand(ctx, app.EvolutionAssetCommandRequest{
		JourneyID: journeyID, OperationID: "evaluate-fail", Action: "record_evaluation",
		ExpectedViewVersion: result.ViewVersion, ExpectedStreamHeads: []journal.StreamHead{}, Input: evaluationInput,
	})
	if err != nil {
		t.Fatalf("record failing evaluation = %v", err)
	}
	after, err := service.ReadEvolutionAssetSnapshot(ctx, app.EvolutionAssetSnapshotRequest{JourneyID: journeyID, Limit: 64})
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Evaluations) != 1 {
		t.Fatalf("evaluations = %d", len(after.Evaluations))
	}
	record := after.Evaluations[0]
	if record.QualityResult != "fail" || record.FailureCount != 2 ||
		record.CompatibilityResult != "incompatible" ||
		record.RegressionResult != "regressed" || record.SecurityResult != "partial" {
		t.Fatalf("evaluation recorded = %#v, want fixture-declared failing results", record)
	}
	if !record.UsageObserved || record.UsageValue == nil || *record.UsageValue != 512 ||
		!record.CostObserved || record.CostValue == nil || *record.CostValue != 300 {
		t.Fatalf("evaluation usage/cost = %#v", record)
	}
	if evaluated.EvaluationID != "evaluation-fail" {
		t.Fatalf("evaluation id = %q", evaluated.EvaluationID)
	}
}

func TestP3ACanonicalEvaluationFixtureIsPublishedWhenMissing(t *testing.T) {
	service, _, _, sourcePath, artifactDigest, contentDigest := newP3AAssetService(t)
	ctx := context.Background()
	journeyID := "123e4567-e89b-42d3-a456-426614174000"
	before, err := service.ReadEvolutionAssetSnapshot(ctx, app.EvolutionAssetSnapshotRequest{JourneyID: journeyID, Limit: 64})
	if err != nil {
		t.Fatal(err)
	}
	createInput, err := json.Marshal(struct {
		DefinitionID                  string   `json:"definition_id"`
		RevisionID                    string   `json:"revision_id"`
		Name                          string   `json:"name"`
		Description                   string   `json:"description"`
		SubjectScope                  string   `json:"subject_scope"`
		SourcePath                    string   `json:"source_path"`
		SuppliedArtifactDigest        string   `json:"supplied_artifact_digest"`
		SuppliedContentDigest         string   `json:"supplied_content_digest"`
		Dependencies                  []string `json:"dependencies"`
		CompatibleRuntimeCapabilities []string `json:"compatible_runtime_capabilities"`
		Risk                          string   `json:"risk"`
	}{"skill-app", "revision-1", "Canonical Eval Skill", "fixture", "project", sourcePath, artifactDigest, contentDigest, []string{}, []string{}, "low"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.CommitEvolutionAssetCommand(ctx, app.EvolutionAssetCommandRequest{
		JourneyID: journeyID, OperationID: "create-canonical-eval", Action: "create_skill",
		ExpectedViewVersion: before.ViewVersion, ExpectedStreamHeads: []journal.StreamHead{}, Input: createInput,
	})
	if err != nil {
		t.Fatal(err)
	}
	caseIDs := []string{"artifact_digest", "runtime_compatibility", "security_boundary"}
	fixture, err := app.CanonicalEvolutionEvaluationFixture("synthetic", caseIDs)
	if err != nil {
		t.Fatal(err)
	}
	fixtureDigest := sha256HexForTest(fixture)
	evaluationInput, err := json.Marshal(struct {
		EvaluationID       string   `json:"evaluation_id"`
		CandidateID        string   `json:"candidate_id"`
		FixtureKind        string   `json:"fixture_kind"`
		FixtureDigest      string   `json:"fixture_digest"`
		BaselineRevisionID string   `json:"baseline_revision_id"`
		BaselineDigest     string   `json:"baseline_digest"`
		CandidateRevision  string   `json:"candidate_revision_id"`
		CandidateDigest    string   `json:"candidate_digest"`
		RequestedCaseIDs   []string `json:"requested_case_ids"`
	}{"evaluation-canonical", result.CandidateID, "synthetic", fixtureDigest, "revision-1", artifactDigest, "revision-1", artifactDigest, caseIDs})
	if err != nil {
		t.Fatal(err)
	}
	// No fixture artifact is published beforehand; the server must publish the
	// canonical fixture matching the supplied digest and record it.
	evaluated, err := service.CommitEvolutionAssetCommand(ctx, app.EvolutionAssetCommandRequest{
		JourneyID: journeyID, OperationID: "evaluate-canonical", Action: "record_evaluation",
		ExpectedViewVersion: result.ViewVersion, ExpectedStreamHeads: []journal.StreamHead{}, Input: evaluationInput,
	})
	if err != nil {
		t.Fatalf("record canonical evaluation = %v", err)
	}
	after, err := service.ReadEvolutionAssetSnapshot(ctx, app.EvolutionAssetSnapshotRequest{JourneyID: journeyID, Limit: 64})
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Evaluations) != 1 || after.Evaluations[0].QualityResult != "pass" ||
		after.Evaluations[0].FixtureDigest != fixtureDigest {
		t.Fatalf("canonical evaluation = %#v", after.Evaluations)
	}
	if evaluated.EvaluationID != "evaluation-canonical" {
		t.Fatalf("evaluation id = %q", evaluated.EvaluationID)
	}
}

func TestP3AApplicationServiceKeepsPriorViewWhenPostCommitRefreshFails(
	t *testing.T,
) {
	refreshFailure := errors.New("controlled projection refresh failure")
	service, store, readModel, _, sourcePath, artifactDigest, contentDigest :=
		newP3AAssetServiceWithRefresh(t, func(
			context.Context,
			string,
		) error {
			return refreshFailure
		})
	journeyID := "123e4567-e89b-42d3-a456-426614174000"
	before, err := service.ReadEvolutionAssetSnapshot(
		context.Background(),
		app.EvolutionAssetSnapshotRequest{JourneyID: journeyID, Limit: 64},
	)
	if err != nil {
		t.Fatal(err)
	}
	input, err := json.Marshal(struct {
		DefinitionID                  string   `json:"definition_id"`
		RevisionID                    string   `json:"revision_id"`
		Name                          string   `json:"name"`
		Description                   string   `json:"description"`
		SubjectScope                  string   `json:"subject_scope"`
		SourcePath                    string   `json:"source_path"`
		SuppliedArtifactDigest        string   `json:"supplied_artifact_digest"`
		SuppliedContentDigest         string   `json:"supplied_content_digest"`
		Dependencies                  []string `json:"dependencies"`
		CompatibleRuntimeCapabilities []string `json:"compatible_runtime_capabilities"`
		Risk                          string   `json:"risk"`
	}{"skill-app", "revision-1", "Refresh Skill", "fixture", "project",
		sourcePath, artifactDigest, contentDigest, []string{}, []string{}, "low"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.CommitEvolutionAssetCommand(
		context.Background(),
		app.EvolutionAssetCommandRequest{
			JourneyID: journeyID, OperationID: "create-refresh-skill",
			Action: "create_skill", ExpectedViewVersion: before.ViewVersion,
			ExpectedStreamHeads: []journal.StreamHead{}, Input: input,
		},
	)
	if !errors.Is(err, refreshFailure) {
		t.Fatalf("post-commit refresh error = %v", err)
	}
	events, err := store.ReadAll(context.Background())
	if err != nil || len(events) != 3 {
		t.Fatalf("committed authority events = %d, %v", len(events), err)
	}
	stale, err := service.ReadEvolutionAssetSnapshot(
		context.Background(),
		app.EvolutionAssetSnapshotRequest{JourneyID: journeyID, Limit: 64},
	)
	if err != nil || stale.ViewVersion != before.ViewVersion || len(stale.Definitions) != 0 {
		t.Fatalf("prior immutable view changed = %#v, %v", stale, err)
	}
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	recovered, err := service.ReadEvolutionAssetSnapshot(
		context.Background(),
		app.EvolutionAssetSnapshotRequest{JourneyID: journeyID, Limit: 64},
	)
	if err != nil || recovered.ViewVersion == before.ViewVersion ||
		len(recovered.Definitions) != 1 {
		t.Fatalf("Journal rebuild recovery = %#v, %v", recovered, err)
	}
}

func TestP3ACanonicalTemplateArtifactStartsWithExactContractAndBindsSource(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "template.md")
	if err := os.WriteFile(sourcePath, []byte("# Candidate template\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sourcePath, err := filepath.EvalSymlinks(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	parameterDigest := strings.Repeat("a", 64)
	permissionDigest := strings.Repeat("b", 64)
	scopeDigest := strings.Repeat("c", 64)
	artifact, artifactDigest, contentDigest, err := app.CanonicalEvolutionTemplateArtifact(
		assets.AssetKindTeamTemplate, "team-template-1", "revision-1", sourcePath,
		assets.TemplateOutputTeamDraft, parameterDigest, permissionDigest, scopeDigest,
	)
	if err != nil || artifactDigest != sha256HexForTest(artifact) || contentDigest == "" {
		t.Fatalf("template artifact digests = %s %s err=%v", artifactDigest, contentDigest, err)
	}
	var envelope struct {
		Entries []struct {
			RelativePath  string `json:"relative_path"`
			FileSHA256    string `json:"file_sha256"`
			ContentBase64 string `json:"content_base64"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(artifact, &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Entries) != 2 || envelope.Entries[0].RelativePath != "template-contract.json" ||
		envelope.Entries[1].RelativePath != "template.md" {
		t.Fatalf("template entries = %#v", envelope.Entries)
	}
	contractBytes, err := base64.StdEncoding.Strict().DecodeString(envelope.Entries[0].ContentBase64)
	if err != nil {
		t.Fatal(err)
	}
	var contract map[string]any
	if err := json.Unmarshal(contractBytes, &contract); err != nil {
		t.Fatal(err)
	}
	if SetOfKeysForP3ATest(contract) != "asset_kind,parameter_schema_digest,permission_ceiling_digest,schema_version,scope_ceiling_digest,source_file_sha256,template_output" ||
		contract["source_file_sha256"] != envelope.Entries[1].FileSHA256 {
		t.Fatalf("template contract = %s", contractBytes)
	}
}

func TestP3AApplicationServiceCreatesTemplateCandidateFromExactContractArtifact(t *testing.T) {
	service, _, artifacts, sourcePath, _, _ := newP3AAssetService(t)
	journeyID := "123e4567-e89b-42d3-a456-426614174000"
	before, err := service.ReadEvolutionAssetSnapshot(context.Background(), app.EvolutionAssetSnapshotRequest{
		JourneyID: journeyID, Limit: 64,
	})
	if err != nil {
		t.Fatal(err)
	}
	parameterDigest := strings.Repeat("a", 64)
	permissionDigest := strings.Repeat("b", 64)
	scopeDigest := strings.Repeat("c", 64)
	_, artifactDigest, contentDigest, err := app.CanonicalEvolutionTemplateArtifact(
		assets.AssetKindTeamTemplate, "team-template-app", "revision-1", sourcePath,
		assets.TemplateOutputTeamDraft, parameterDigest, permissionDigest, scopeDigest,
	)
	if err != nil {
		t.Fatal(err)
	}
	input, err := json.Marshal(struct {
		AssetKind                     assets.AssetKind      `json:"asset_kind"`
		DefinitionID                  string                `json:"definition_id"`
		RevisionID                    string                `json:"revision_id"`
		Name                          string                `json:"name"`
		Description                   string                `json:"description"`
		SubjectScope                  string                `json:"subject_scope"`
		SourcePath                    string                `json:"source_path"`
		SuppliedArtifactDigest        string                `json:"supplied_artifact_digest"`
		SuppliedContentDigest         string                `json:"supplied_content_digest"`
		TemplateOutput                assets.TemplateOutput `json:"template_output"`
		ParameterSchemaDigest         string                `json:"parameter_schema_digest"`
		PermissionCeilingDigest       string                `json:"permission_ceiling_digest"`
		ScopeCeilingDigest            string                `json:"scope_ceiling_digest"`
		CompatibleRuntimeCapabilities []string              `json:"compatible_runtime_capabilities"`
		Risk                          string                `json:"risk"`
	}{assets.AssetKindTeamTemplate, "team-template-app", "revision-1", "Team Template", "Candidate-only", "project", sourcePath, artifactDigest, contentDigest, assets.TemplateOutputTeamDraft, parameterDigest, permissionDigest, scopeDigest, []string{}, "medium"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.CommitEvolutionAssetCommand(context.Background(), app.EvolutionAssetCommandRequest{
		JourneyID: journeyID, OperationID: "create-team-template-app", Action: "create_template",
		ExpectedViewVersion: before.ViewVersion, ExpectedStreamHeads: []journal.StreamHead{}, Input: input,
	})
	if err != nil || result.DefinitionID != "team-template-app" || result.CandidateID == "" {
		t.Fatalf("template result = %#v, %v", result, err)
	}
	published, err := artifacts.ReadArtifact(context.Background(), artifactDigest, 1572864)
	if err != nil || sha256HexForTest(published) != artifactDigest {
		t.Fatalf("published template digest = %s, %v", sha256HexForTest(published), err)
	}
}

func SetOfKeysForP3ATest(value map[string]any) string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

func newP3AAssetService(t testing.TB) (*app.LocalProductAssetService, *journal.Store, *evidence.Store, string, string, string) {
	service, store, _, artifacts, sourcePath, artifactDigest, contentDigest :=
		newP3AAssetServiceWithRefresh(t, nil)
	return service, store, artifacts, sourcePath, artifactDigest, contentDigest
}

func newP3AAssetServiceWithRefresh(
	t testing.TB,
	refresh app.EvolutionAssetProjectionRefresh,
) (*app.LocalProductAssetService, *journal.Store, *projection.Projection, *evidence.Store, string, string, string) {
	t.Helper()
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("sqlite", filepath.Join(root, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := journal.Migrate(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	readModel := projection.New(database)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	store := journal.NewStore(database)
	authority, err := assets.NewAuthority(assets.AuthorityConfig{
		Store: store, Now: func() time.Time { return time.Date(2026, 8, 3, 18, 0, 0, 0, time.UTC) },
		ViewVersion:       func() string { return readModel.GlobalReadView().Version() },
		Subjects:          appTestSubjectResolver{},
		TemplateArtifacts: appTestTemplateArtifactResolver{},
	})
	if err != nil {
		t.Fatal(err)
	}
	artifactStore, err := evidence.NewStore(filepath.Join(root, "evidence"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = artifactStore.Close() })
	service, err := app.NewLocalProductAssetService(
		readModel,
		authority,
		artifactStore,
		refresh,
	)
	if err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(root, "SKILL.md")
	content := []byte("# App Skill\n")
	if err := os.WriteFile(sourcePath, content, 0o600); err != nil {
		t.Fatal(err)
	}
	sourcePath, err = filepath.EvalSymlinks(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	_, artifactDigest, contentDigest, err := app.CanonicalEvolutionAssetArtifact(assets.AssetKindSkill, "skill-app", "revision-1", sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	return service, store, readModel, artifactStore, sourcePath, artifactDigest, contentDigest
}

type appTestTemplateArtifactResolver struct{}

type appTestSubjectResolver struct{}

func (appTestSubjectResolver) ResolveBindingSubject(
	_ context.Context,
	subject assets.SubjectIdentity,
) (assets.SubjectIdentity, error) {
	return subject, nil
}

func (appTestTemplateArtifactResolver) ResolveTemplateArtifact(
	_ context.Context,
	request assets.TemplateArtifactRequest,
) (assets.TemplateArtifactContract, error) {
	outputs := map[assets.AssetKind]assets.TemplateOutput{
		assets.AssetKindAgentTemplate:            assets.TemplateOutputAgentCandidate,
		assets.AssetKindTeamTemplate:             assets.TemplateOutputTeamDraft,
		assets.AssetKindWorkPackageTemplate:      assets.TemplateOutputWorkPackageCandidate,
		assets.AssetKindRecoveryStrategyTemplate: assets.TemplateOutputRecoveryStrategyCandidate,
	}
	output, ok := outputs[request.AssetKind]
	if !ok {
		return assets.TemplateArtifactContract{}, assets.ErrDenied
	}
	return assets.TemplateArtifactContract{
		AssetKind: request.AssetKind, DefinitionID: request.DefinitionID,
		RevisionID: request.RevisionID, TemplateOutput: output,
		ParameterSchemaDigest:   strings.Repeat("a", 64),
		PermissionCeilingDigest: strings.Repeat("b", 64),
		ScopeCeilingDigest:      strings.Repeat("c", 64),
	}, nil
}

func sha256HexForTest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
