package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"loom-pi-rebuild/internal/assets"
	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/work"
)

var ErrInvalidLocalProductAsset = errors.New("invalid local product asset request")

type EvolutionAssetSnapshotRequest struct {
	JourneyID  string           `json:"-"`
	Cursor     string           `json:"cursor"`
	Limit      int              `json:"limit"`
	AssetKind  assets.AssetKind `json:"asset_kind,omitempty"`
	Lifecycle  assets.Lifecycle `json:"lifecycle,omitempty"`
	SearchText string           `json:"search_text,omitempty"`
}
type EvolutionAssetSnapshot struct {
	ViewVersion      string                                     `json:"view_version"`
	NextCursor       string                                     `json:"next_cursor"`
	Definitions      []assets.SkillDefinition                   `json:"definitions"`
	Revisions        []assets.SkillRevision                     `json:"revisions"`
	Candidates       []assets.EvolutionCandidate                `json:"candidates"`
	Evaluations      []assets.EvaluationRecord                  `json:"evaluations"`
	Bindings         []assets.EvolutionAssetBindingRecord       `json:"bindings"`
	Materializations []assets.RuntimeSkillMaterializationRecord `json:"materializations"`
	BindingSubjects  []EvolutionAssetBindingSubject             `json:"binding_subjects"`
	PromotionSources []EvolutionAssetPromotionSource            `json:"promotion_sources"`
}
type EvolutionAssetBindingSubject struct {
	assets.SubjectIdentity
	SubjectIdentityDigest string `json:"subject_identity_digest"`
}
type EvolutionAssetPromotionSource struct {
	RunID           string   `json:"run_id"`
	RunGeneration   int64    `json:"run_generation"`
	RunDigest       string   `json:"run_digest"`
	EvidenceIDs     []string `json:"evidence_ids"`
	EvidenceDigests []string `json:"evidence_digests"`
}
type EvolutionAssetDiffRequest struct {
	JourneyID       string `json:"-"`
	DefinitionID    string `json:"definition_id"`
	LeftRevisionID  string `json:"left_revision_id"`
	LeftDigest      string `json:"left_digest"`
	RightRevisionID string `json:"right_revision_id"`
	RightDigest     string `json:"right_digest"`
}
type EvolutionAssetDiff struct {
	DefinitionID    string                     `json:"definition_id"`
	LeftRevisionID  string                     `json:"left_revision_id"`
	LeftDigest      string                     `json:"left_digest"`
	RightRevisionID string                     `json:"right_revision_id"`
	RightDigest     string                     `json:"right_digest"`
	Changes         []EvolutionAssetDiffChange `json:"changes"`
}
type EvolutionAssetDiffChange struct {
	Kind         string `json:"kind"`
	RelativePath string `json:"relative_path"`
	LeftDigest   string `json:"left_digest"`
	RightDigest  string `json:"right_digest"`
}
type EvolutionAssetCommandRequest struct {
	JourneyID           string               `json:"-"`
	OperationID         string               `json:"operation_id"`
	Action              string               `json:"action"`
	ExpectedViewVersion string               `json:"expected_view_version"`
	ExpectedStreamHeads []journal.StreamHead `json:"expected_stream_heads"`
	Input               json.RawMessage      `json:"input"`
}
type EvolutionAssetCommandResult struct {
	OperationID                   string   `json:"operation_id"`
	Action                        string   `json:"action"`
	ViewVersion                   string   `json:"view_version"`
	EventIDs                      []string `json:"event_ids"`
	DefinitionID                  string   `json:"definition_id"`
	RevisionID                    string   `json:"revision_id"`
	CandidateID                   string   `json:"candidate_id"`
	EvaluationID                  string   `json:"evaluation_id"`
	SubjectKind                   string   `json:"subject_kind"`
	SubjectID                     string   `json:"subject_id"`
	AssetRevisionSetDigest        string   `json:"asset_revision_set_digest"`
	MaterializationManifestDigest string   `json:"materialization_manifest_digest"`
	Status                        string   `json:"status"`
}

type EvolutionAssetProjectionRefresh func(context.Context, string) error

type LocalProductAssetService struct {
	projection *projection.Projection
	authority  *assets.Authority
	artifacts  *evidence.Store
	refresh    EvolutionAssetProjectionRefresh
}

func NewLocalProductAssetService(
	readModel *projection.Projection,
	authority *assets.Authority,
	artifacts *evidence.Store,
	refreshes ...EvolutionAssetProjectionRefresh,
) (*LocalProductAssetService, error) {
	if readModel == nil || authority == nil || artifacts == nil ||
		len(refreshes) > 1 {
		return nil, ErrInvalidLocalProductAsset
	}
	refresh := EvolutionAssetProjectionRefresh(func(
		ctx context.Context,
		_ string,
	) error {
		return readModel.Rebuild(ctx)
	})
	if len(refreshes) == 1 && refreshes[0] != nil {
		refresh = refreshes[0]
	}
	return &LocalProductAssetService{
		projection: readModel,
		authority:  authority,
		artifacts:  artifacts,
		refresh:    refresh,
	}, nil
}

func (service *LocalProductAssetService) ReadEvolutionAssetSnapshot(ctx context.Context, request EvolutionAssetSnapshotRequest) (EvolutionAssetSnapshot, error) {
	if service == nil || service.projection == nil || ctx == nil || request.Limit < 1 || request.Limit > 64 || !validAssetJourney(request.JourneyID) {
		return EvolutionAssetSnapshot{}, ErrInvalidLocalProductAsset
	}
	if err := ctx.Err(); err != nil {
		return EvolutionAssetSnapshot{}, err
	}
	view := service.projection.GlobalReadView()
	definitions, _ := view.EvolutionAssetDefinitions("", 64)
	filtered := make([]assets.SkillDefinition, 0, request.Limit+1)
	for _, definition := range definitions {
		if definition.DefinitionID <= request.Cursor || request.Lifecycle != "" && definition.Lifecycle != request.Lifecycle || request.SearchText != "" && !strings.Contains(strings.ToLower(definition.Name+" "+definition.Description), strings.ToLower(request.SearchText)) {
			continue
		}
		if request.AssetKind != "" {
			matched := false
			for _, revisionID := range []string{definition.LatestRevisionID, definition.ActiveRevisionID} {
				_, matched = view.EvolutionAssetRevision(string(request.AssetKind) + "/" + definition.DefinitionID + "/" + revisionID)
				if matched {
					break
				}
			}
			if !matched {
				continue
			}
		}
		filtered = append(filtered, definition)
		if len(filtered) > request.Limit {
			break
		}
	}
	hasMore := len(filtered) > request.Limit
	if hasMore {
		filtered = filtered[:request.Limit]
	}
	next := ""
	if hasMore && len(filtered) > 0 {
		next = filtered[len(filtered)-1].DefinitionID
	}
	revisions, _ := view.EvolutionAssetRevisions("", 64)
	allowedDefinitions := map[string]bool{}
	for _, definition := range filtered {
		allowedDefinitions[definition.DefinitionID] = true
	}
	visibleRevisions := make([]assets.SkillRevision, 0, len(revisions))
	for _, revision := range revisions {
		if allowedDefinitions[revision.DefinitionID] && (request.AssetKind == "" || revision.AssetKind == request.AssetKind) {
			visibleRevisions = append(visibleRevisions, revision)
		}
	}
	candidates, _ := view.EvolutionAssetCandidates("", 64)
	evaluations, _ := view.EvolutionAssetEvaluations("", 64)
	bindings, _ := view.EvolutionAssetBindings("", 64)
	materializations, _ := view.RuntimeSkillMaterializations("", 64)
	bindingSubjects, err := evolutionAssetBindingSubjects(view)
	if err != nil {
		return EvolutionAssetSnapshot{}, err
	}
	promotionSources, err := service.evolutionAssetPromotionSources(ctx, view)
	if err != nil {
		return EvolutionAssetSnapshot{}, err
	}
	return EvolutionAssetSnapshot{ViewVersion: view.Version(), NextCursor: next, Definitions: nonNilDefinitions(filtered), Revisions: nonNilRevisions(visibleRevisions), Candidates: nonNilCandidates(candidates), Evaluations: nonNilEvaluations(evaluations), Bindings: nonNilBindings(bindings), Materializations: nonNilMaterializations(materializations), BindingSubjects: bindingSubjects, PromotionSources: promotionSources}, nil
}

func (service *LocalProductAssetService) evolutionAssetPromotionSources(
	ctx context.Context,
	view projection.GlobalReadView,
) ([]EvolutionAssetPromotionSource, error) {
	runs, _ := view.Runs("", 64)
	output := make([]EvolutionAssetPromotionSource, 0)
	for _, run := range runs {
		if run.Phase != "terminal" || run.TerminalStatus != "succeeded" || run.ClaimGeneration < 1 {
			continue
		}
		item, ok := view.WorkItem(run.WorkItemID)
		if !ok || item.RunID != run.ID || item.Status != "done" ||
			item.AcceptanceDecisionKind != "accepted" ||
			item.SourceEvidenceID == "" || item.SourceEvidenceDigest == "" {
			continue
		}
		evidenceIDs := []string{item.SourceEvidenceID}
		evidenceDigests := []string{item.SourceEvidenceDigest}
		if item.VerifierRequired {
			if item.VerifierEvidenceID == "" || item.VerifierEvidenceDigest == "" {
				continue
			}
			evidenceIDs = append(evidenceIDs, item.VerifierEvidenceID)
			evidenceDigests = append(evidenceDigests, item.VerifierEvidenceDigest)
		}
		validEvidence := true
		for index, evidenceID := range evidenceIDs {
			projected, found := view.Evidence(evidenceID)
			if !found || projected.Digest != evidenceDigests[index] {
				validEvidence = false
				break
			}
			if _, err := service.artifacts.ReadArtifact(ctx, evidenceDigests[index], 16<<20); err != nil {
				validEvidence = false
				break
			}
		}
		if !validEvidence {
			continue
		}
		runBytes, err := canonicalAssetJSON(struct {
			SchemaVersion    int      `json:"schema_version"`
			RunID            string   `json:"run_id"`
			RunGeneration    int64    `json:"run_generation"`
			WorkItemID       string   `json:"work_item_id"`
			TerminalStatus   string   `json:"terminal_status"`
			AcceptanceDigest string   `json:"acceptance_decision_digest"`
			EvidenceIDs      []string `json:"evidence_ids"`
			EvidenceDigests  []string `json:"evidence_digests"`
		}{1, run.ID, run.ClaimGeneration, run.WorkItemID, run.TerminalStatus,
			item.AcceptanceDecisionDigest, evidenceIDs, evidenceDigests})
		if err != nil {
			return nil, err
		}
		output = append(output, EvolutionAssetPromotionSource{
			RunID: run.ID, RunGeneration: run.ClaimGeneration,
			RunDigest: sha256Hex(runBytes), EvidenceIDs: evidenceIDs,
			EvidenceDigests: evidenceDigests,
		})
	}
	sort.Slice(output, func(i, j int) bool { return output[i].RunID < output[j].RunID })
	return output, nil
}

func evolutionAssetBindingSubjects(view projection.GlobalReadView) ([]EvolutionAssetBindingSubject, error) {
	values := make([]assets.SubjectIdentity, 0)
	definitions, _ := view.TeamDefinitions("", 64)
	for _, definition := range definitions {
		values = append(values, assets.SubjectIdentity{
			SubjectKind: "team_definition", SubjectID: definition.ID,
			SubjectVersion: int64(definition.Version), SubjectDigest: definition.DefinitionDigest,
			Scope: definition.Scope, ProjectID: definition.ScopeIdentity.ProjectID,
			GenerationID: definition.ScopeIdentity.GenerationID,
		})
	}
	for _, build := range []func() (work.WorkPackage, error){
		work.CodingWorkPackage,
		work.KnowledgeWorkPackage,
	} {
		value, err := build()
		if err != nil {
			return nil, err
		}
		values = append(values, assets.SubjectIdentity{
			SubjectKind: "work_package", SubjectID: value.ID(),
			SubjectVersion: int64(value.Version()), SubjectDigest: value.Digest(),
			Scope: "builtin",
		})
	}
	output := make([]EvolutionAssetBindingSubject, len(values))
	for index, value := range values {
		digest, err := assets.CanonicalSubjectIdentityDigest(value)
		if err != nil {
			return nil, err
		}
		output[index] = EvolutionAssetBindingSubject{
			SubjectIdentity: value, SubjectIdentityDigest: digest,
		}
	}
	sort.Slice(output, func(i, j int) bool {
		if output[i].SubjectKind != output[j].SubjectKind {
			return output[i].SubjectKind < output[j].SubjectKind
		}
		return output[i].SubjectID < output[j].SubjectID
	})
	return output, nil
}

func (service *LocalProductAssetService) DiffEvolutionAssetRevisions(ctx context.Context, request EvolutionAssetDiffRequest) (EvolutionAssetDiff, error) {
	if service == nil || service.projection == nil || ctx == nil || !validAssetJourney(request.JourneyID) || request.DefinitionID == "" {
		return EvolutionAssetDiff{}, ErrInvalidLocalProductAsset
	}
	view := service.projection.GlobalReadView()
	var left, right assets.SkillRevision
	foundLeft, foundRight := false, false
	for _, kind := range []assets.AssetKind{assets.AssetKindSkill, assets.AssetKindAgentTemplate, assets.AssetKindTeamTemplate, assets.AssetKindWorkPackageTemplate, assets.AssetKindRecoveryStrategyTemplate} {
		if value, ok := view.EvolutionAssetRevision(string(kind) + "/" + request.DefinitionID + "/" + request.LeftRevisionID); ok {
			left = value
			foundLeft = true
		}
		if value, ok := view.EvolutionAssetRevision(string(kind) + "/" + request.DefinitionID + "/" + request.RightRevisionID); ok {
			right = value
			foundRight = true
		}
	}
	if !foundLeft || !foundRight || left.ArtifactDigest != request.LeftDigest || right.ArtifactDigest != request.RightDigest {
		return EvolutionAssetDiff{}, assets.ErrDigestMismatch
	}
	changes := []EvolutionAssetDiffChange{}
	if left.ArtifactDigest != right.ArtifactDigest {
		changes = append(changes, EvolutionAssetDiffChange{Kind: "changed", RelativePath: "artifact", LeftDigest: left.ArtifactDigest, RightDigest: right.ArtifactDigest})
	}
	if left.ContentDigest != right.ContentDigest {
		changes = append(changes, EvolutionAssetDiffChange{Kind: "changed", RelativePath: "content", LeftDigest: left.ContentDigest, RightDigest: right.ContentDigest})
	}
	if len(changes) == 0 {
		changes = append(changes, EvolutionAssetDiffChange{Kind: "unchanged", RelativePath: "content", LeftDigest: left.ContentDigest, RightDigest: right.ContentDigest})
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].RelativePath < changes[j].RelativePath })
	return EvolutionAssetDiff{DefinitionID: request.DefinitionID, LeftRevisionID: request.LeftRevisionID, LeftDigest: request.LeftDigest, RightRevisionID: request.RightRevisionID, RightDigest: request.RightDigest, Changes: changes}, nil
}

func (service *LocalProductAssetService) CommitEvolutionAssetCommand(ctx context.Context, request EvolutionAssetCommandRequest) (EvolutionAssetCommandResult, error) {
	if service == nil || service.authority == nil || ctx == nil || !validAssetJourney(request.JourneyID) || request.OperationID == "" || request.Action == "" || request.ExpectedViewVersion == "" || len(request.Input) == 0 {
		return EvolutionAssetCommandResult{}, ErrInvalidLocalProductAsset
	}
	command, err := service.decodeAssetCommandInput(ctx, request.Action, request.OperationID, request.Input)
	if err != nil {
		return EvolutionAssetCommandResult{}, err
	}
	command.Action = request.Action
	command.OperationID = request.OperationID
	command.JourneyID = request.JourneyID
	command.ExpectedViewVersion = request.ExpectedViewVersion
	command.ExpectedStreamHeads = append([]journal.StreamHead(nil), request.ExpectedStreamHeads...)
	var result assets.Result
	switch command.Action {
	case "create_skill":
		result, err = service.authority.CreateSkill(ctx, command)
	case "import_skill":
		result, err = service.authority.ImportSkill(ctx, command)
	case "create_template":
		result, err = service.authority.CreateTemplate(ctx, command)
	case "instantiate_template":
		result, err = service.authority.InstantiateTemplate(ctx, command)
	case "promote_run":
		result, err = service.authority.PromoteRun(ctx, command)
	case "record_evaluation":
		result, err = service.authority.RecordEvaluation(ctx, command)
	case "set_binding":
		result, err = service.authority.SetBinding(ctx, command)
	case "activate":
		result, err = service.authority.ActivateCandidate(ctx, command)
	case "reject":
		result, err = service.authority.RejectCandidate(ctx, command)
	case "retain":
		result, err = service.authority.RetainCandidate(ctx, command)
	case "archive":
		result, err = service.authority.ArchiveRevision(ctx, command)
	case "restore":
		result, err = service.authority.RestoreRevision(ctx, command)
	case "rollback":
		result, err = service.authority.RollbackActivation(ctx, command)
	default:
		err = ErrInvalidLocalProductAsset
	}
	if err != nil {
		return EvolutionAssetCommandResult{}, err
	}
	if err := service.refresh(ctx, command.Action); err != nil {
		return EvolutionAssetCommandResult{}, err
	}
	eventIDs := append([]string(nil), result.EventIDs...)
	if eventIDs == nil {
		eventIDs = []string{}
	}
	status := command.Action
	if result.Definition.Lifecycle != "" {
		status = string(result.Definition.Lifecycle)
	}
	if result.Candidate.Decision != "" {
		status = result.Candidate.Decision
	}
	return EvolutionAssetCommandResult{
		OperationID: request.OperationID, Action: request.Action,
		ViewVersion: service.projection.GlobalReadView().Version(), EventIDs: eventIDs,
		DefinitionID: command.DefinitionID, RevisionID: command.RevisionID,
		CandidateID: command.CandidateID, EvaluationID: command.EvaluationID,
		SubjectKind: command.Subject.SubjectKind, SubjectID: command.Subject.SubjectID,
		AssetRevisionSetDigest:        result.Binding.AssetRevisionSetDigest,
		MaterializationManifestDigest: result.MaterializationManifestDigest,
		Status:                        status,
	}, nil
}

func decodeStrictAssetInput(data []byte, output any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(output); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return ErrInvalidLocalProductAsset
	}
	return nil
}

type evolutionEvaluationFixture struct {
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
}

// CanonicalEvolutionEvaluationFixture returns the frozen content-addressed
// evaluation fixture for a synthetic evaluation over the given case IDs
// (sorted, duplicate-free). The canonical expected outcomes are the standard
// passing synthetic result set; the immutable fixture is the authority for
// the recorded evaluation results (P3A-W1-IMPLEMENTATION-REPAIR.md §4).
func CanonicalEvolutionEvaluationFixture(
	kind string,
	caseIDs []string,
) ([]byte, error) {
	if kind != "historical" && kind != "synthetic" {
		return nil, ErrInvalidLocalProductAsset
	}
	cases := nonNilStrings(caseIDs)
	sort.Strings(cases)
	for index, caseID := range cases {
		if caseID == "" || index > 0 && cases[index-1] == caseID {
			return nil, ErrInvalidLocalProductAsset
		}
	}
	return canonicalAssetJSON(struct {
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
		SchemaVersion: 1, FixtureKind: kind, CaseIDs: cases,
		Expected: struct {
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
			QualityResult: "pass", FailureCount: 0,
			UsageObserved: true, UsageMicrounits: 250,
			CostObserved: true, CostMicrounits: 1250, CostCurrency: "USD",
			CompatibilityResult: "compatible", ApplicableScope: "bounded_fixture",
			RegressionResult: "equivalent", SecurityResult: "pass",
		},
	})
}

func decodeEvaluationFixture(
	data []byte,
	kind string,
	requestedCaseIDs []string,
) (*evolutionEvaluationFixture, error) {
	var fixture evolutionEvaluationFixture
	if err := decodeStrictAssetInput(data, &fixture); err != nil {
		return nil, err
	}
	if fixture.SchemaVersion != 1 || fixture.FixtureKind != kind {
		return nil, ErrInvalidLocalProductAsset
	}
	cases := nonNilStrings(fixture.CaseIDs)
	sort.Strings(cases)
	if !reflect.DeepEqual(cases, requestedCaseIDs) {
		return nil, assets.ErrDigestMismatch
	}
	switch fixture.Expected.QualityResult {
	case "pass", "partial", "fail":
	default:
		return nil, ErrInvalidLocalProductAsset
	}
	switch fixture.Expected.CompatibilityResult {
	case "compatible", "incompatible", "partial":
	default:
		return nil, ErrInvalidLocalProductAsset
	}
	switch fixture.Expected.RegressionResult {
	case "improved", "equivalent", "regressed", "unknown":
	default:
		return nil, ErrInvalidLocalProductAsset
	}
	switch fixture.Expected.SecurityResult {
	case "pass", "fail", "partial":
	default:
		return nil, ErrInvalidLocalProductAsset
	}
	if fixture.Expected.FailureCount < 0 ||
		fixture.Expected.FailureCount > len(requestedCaseIDs) ||
		len(fixture.Expected.ApplicableScope) == 0 ||
		len(fixture.Expected.ApplicableScope) > 4096 {
		return nil, ErrInvalidLocalProductAsset
	}
	if !fixture.Expected.UsageObserved && fixture.Expected.UsageMicrounits != 0 ||
		fixture.Expected.UsageObserved && fixture.Expected.UsageMicrounits < 0 {
		return nil, ErrInvalidLocalProductAsset
	}
	if fixture.Expected.CostObserved &&
		(fixture.Expected.CostMicrounits < 0 ||
			len(fixture.Expected.CostCurrency) != 3 ||
			!allUpperASCII(fixture.Expected.CostCurrency)) ||
		!fixture.Expected.CostObserved &&
			(fixture.Expected.CostMicrounits != 0 || fixture.Expected.CostCurrency != "") {
		return nil, ErrInvalidLocalProductAsset
	}
	return &fixture, nil
}

func allUpperASCII(value string) bool {
	for _, character := range value {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}

type assetParameterValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type assetCommandInput struct {
	CandidateID                   string                             `json:"candidate_id"`
	AssetKind                     assets.AssetKind                   `json:"asset_kind"`
	DefinitionID                  string                             `json:"definition_id"`
	RevisionID                    string                             `json:"revision_id"`
	Name                          string                             `json:"name"`
	Description                   string                             `json:"description"`
	SubjectScope                  string                             `json:"subject_scope"`
	SourcePath                    string                             `json:"source_path"`
	SuppliedArtifactDigest        string                             `json:"supplied_artifact_digest"`
	SuppliedContentDigest         string                             `json:"supplied_content_digest"`
	ExternalSourceDigest          string                             `json:"external_source_digest"`
	ProvenanceDigest              string                             `json:"provenance_digest"`
	Dependencies                  []string                           `json:"dependencies"`
	CompatibleRuntimeCapabilities []string                           `json:"compatible_runtime_capabilities"`
	Risk                          assets.Risk                        `json:"risk"`
	TemplateOutput                assets.TemplateOutput              `json:"template_output"`
	ParameterSchemaDigest         string                             `json:"parameter_schema_digest"`
	PermissionCeilingDigest       string                             `json:"permission_ceiling_digest"`
	ScopeCeilingDigest            string                             `json:"scope_ceiling_digest"`
	RevisionDigest                string                             `json:"revision_digest"`
	ParameterValues               []assetParameterValue              `json:"parameter_values"`
	ParameterDigest               string                             `json:"parameter_digest"`
	SourceRunID                   string                             `json:"source_run_id"`
	SourceRunGeneration           int64                              `json:"source_run_generation"`
	SourceRunDigest               string                             `json:"source_run_digest"`
	SourceEvidenceIDs             []string                           `json:"source_evidence_ids"`
	SourceEvidenceDigests         []string                           `json:"source_evidence_digests"`
	RedactedSummary               string                             `json:"redacted_summary"`
	RedactedSummaryDigest         string                             `json:"redacted_summary_digest"`
	ScopeDifference               string                             `json:"scope_difference"`
	ExpectedBenefit               string                             `json:"expected_benefit"`
	EvaluationID                  string                             `json:"evaluation_id"`
	FixtureKind                   string                             `json:"fixture_kind"`
	FixtureDigest                 string                             `json:"fixture_digest"`
	BaselineRevisionID            string                             `json:"baseline_revision_id"`
	BaselineDigest                string                             `json:"baseline_digest"`
	CandidateRevisionID           string                             `json:"candidate_revision_id"`
	CandidateDigest               string                             `json:"candidate_digest"`
	RequestedCaseIDs              []string                           `json:"requested_case_ids"`
	SubjectKind                   string                             `json:"subject_kind"`
	SubjectID                     string                             `json:"subject_id"`
	SubjectVersion                int64                              `json:"subject_version"`
	SubjectDigest                 string                             `json:"subject_digest"`
	SubjectProjectID              string                             `json:"subject_project_id"`
	SubjectGenerationID           string                             `json:"subject_generation_id"`
	SubjectIdentityDigest         string                             `json:"subject_identity_digest"`
	AssetRevisionBindings         []assets.ExactAssetRevisionBinding `json:"asset_revision_bindings"`
	AssetRevisionSetDigest        string                             `json:"asset_revision_set_digest"`
	ExpectedPreviousRevisionID    string                             `json:"expected_previous_revision_id"`
	EvaluationIDs                 []string                           `json:"evaluation_ids"`
	ReasonCode                    string                             `json:"reason_code"`
	FromRevisionID                string                             `json:"from_revision_id"`
	FromDigest                    string                             `json:"from_digest"`
	ToRevisionID                  string                             `json:"to_revision_id"`
	ToDigest                      string                             `json:"to_digest"`
}

var assetActionInputKeys = map[string][]string{
	"create_skill":         {"definition_id", "revision_id", "name", "description", "subject_scope", "source_path", "supplied_artifact_digest", "supplied_content_digest", "dependencies", "compatible_runtime_capabilities", "risk"},
	"import_skill":         {"candidate_id", "definition_id", "revision_id", "name", "description", "subject_scope", "source_path", "supplied_artifact_digest", "supplied_content_digest", "external_source_digest", "provenance_digest", "dependencies", "compatible_runtime_capabilities", "risk"},
	"create_template":      {"asset_kind", "definition_id", "revision_id", "name", "description", "subject_scope", "source_path", "supplied_artifact_digest", "supplied_content_digest", "template_output", "parameter_schema_digest", "permission_ceiling_digest", "scope_ceiling_digest", "compatible_runtime_capabilities", "risk"},
	"instantiate_template": {"asset_kind", "definition_id", "revision_id", "revision_digest", "parameter_values", "parameter_digest"},
	"promote_run":          {"candidate_id", "asset_kind", "definition_id", "revision_id", "source_run_id", "source_run_generation", "source_run_digest", "source_evidence_ids", "source_evidence_digests", "redacted_summary", "redacted_summary_digest", "scope_difference", "expected_benefit", "risk"},
	"record_evaluation":    {"evaluation_id", "candidate_id", "fixture_kind", "fixture_digest", "baseline_revision_id", "baseline_digest", "candidate_revision_id", "candidate_digest", "requested_case_ids"},
	"set_binding":          {"subject_kind", "subject_id", "subject_version", "subject_digest", "subject_scope", "subject_project_id", "subject_generation_id", "subject_identity_digest", "asset_revision_bindings", "asset_revision_set_digest"},
	"activate":             {"candidate_id", "asset_kind", "definition_id", "revision_id", "revision_digest", "expected_previous_revision_id", "evaluation_ids"},
	"reject":               {"candidate_id", "asset_kind", "definition_id", "revision_id", "revision_digest", "reason_code"},
	"retain":               {"candidate_id", "asset_kind", "definition_id", "revision_id", "revision_digest", "reason_code"},
	"archive":              {"asset_kind", "definition_id", "revision_id", "revision_digest", "reason_code"},
	"restore":              {"asset_kind", "definition_id", "revision_id", "revision_digest", "reason_code"},
	"rollback":             {"asset_kind", "definition_id", "from_revision_id", "from_digest", "to_revision_id", "to_digest", "evaluation_ids", "reason_code"},
}

func (service *LocalProductAssetService) decodeAssetCommandInput(ctx context.Context, action, operationID string, data []byte) (assets.Command, error) {
	want, ok := assetActionInputKeys[action]
	if !ok || !exactJSONKeys(data, want) {
		return assets.Command{}, ErrInvalidLocalProductAsset
	}
	var input assetCommandInput
	if err := decodeStrictAssetInput(data, &input); err != nil {
		return assets.Command{}, err
	}
	command := assets.Command{
		AssetKind: input.AssetKind, DefinitionID: input.DefinitionID, RevisionID: input.RevisionID,
		CandidateID: input.CandidateID, EvaluationID: input.EvaluationID, Name: input.Name,
		Description: input.Description, Scope: input.SubjectScope, ArtifactDigest: input.RevisionDigest,
		ContentDigest: input.SuppliedContentDigest, SourceReferenceDigest: input.ExternalSourceDigest,
		ProvenanceDigest: input.ProvenanceDigest, Dependencies: nonNilStrings(input.Dependencies),
		CompatibleCapabilities: nonNilStrings(input.CompatibleRuntimeCapabilities), Risk: input.Risk,
		TemplateOutput: input.TemplateOutput, ParameterSchemaDigest: input.ParameterSchemaDigest,
		PermissionCeilingDigest: input.PermissionCeilingDigest, ScopeCeilingDigest: input.ScopeCeilingDigest,
		SourceRunID: input.SourceRunID, SourceRunGeneration: input.SourceRunGeneration,
		SourceRunDigest: input.SourceRunDigest, SourceEvidenceIDs: nonNilStrings(input.SourceEvidenceIDs),
		SourceEvidenceDigests: nonNilStrings(input.SourceEvidenceDigests), RedactedSummary: input.RedactedSummary,
		ScopeDifference: input.ScopeDifference, ExpectedBenefit: input.ExpectedBenefit,
		ReasonCode: input.ReasonCode, ExpectedPreviousRevisionID: input.ExpectedPreviousRevisionID,
		TargetRevisionID: input.ToRevisionID, TargetRevisionDigest: input.ToDigest,
		Bindings:         append([]assets.ExactAssetRevisionBinding(nil), input.AssetRevisionBindings...),
		ParametersDigest: input.ParameterDigest,
		ParameterValues: func() []assets.ParameterValue {
			values := make([]assets.ParameterValue, len(input.ParameterValues))
			for index, value := range input.ParameterValues {
				values[index] = assets.ParameterValue{Name: value.Name, Value: value.Value}
			}
			return values
		}(),
		RequestedCaseIDs: nonNilStrings(input.RequestedCaseIDs),
	}
	switch action {
	case "create_skill", "import_skill", "create_template":
		artifactBytes, artifactDigest, contentDigest, err := CanonicalEvolutionAssetArtifact(input.AssetKind, input.DefinitionID, input.RevisionID, input.SourcePath)
		if action == "create_skill" || action == "import_skill" {
			artifactBytes, artifactDigest, contentDigest, err = CanonicalEvolutionAssetArtifact(assets.AssetKindSkill, input.DefinitionID, input.RevisionID, input.SourcePath)
		} else {
			artifactBytes, artifactDigest, contentDigest, err = CanonicalEvolutionTemplateArtifact(
				input.AssetKind, input.DefinitionID, input.RevisionID, input.SourcePath,
				input.TemplateOutput, input.ParameterSchemaDigest,
				input.PermissionCeilingDigest, input.ScopeCeilingDigest,
			)
		}
		if err != nil || artifactDigest != input.SuppliedArtifactDigest || contentDigest != input.SuppliedContentDigest {
			return assets.Command{}, assets.ErrDigestMismatch
		}
		if _, err := service.artifacts.Publish(ctx, bytes.NewReader(artifactBytes), artifactDigest); err != nil {
			return assets.Command{}, err
		}
		command.ArtifactDigest, command.ContentDigest = artifactDigest, contentDigest
		if action == "create_skill" {
			command.AssetKind, command.SourceScope = assets.AssetKindSkill, assets.SourceScopeLocal
			command.CandidateID = deterministicAssetID("candidate", operationID, input.DefinitionID, input.RevisionID)
			command.SourceReferenceDigest, command.ProvenanceDigest = artifactDigest, artifactDigest
		} else if action == "import_skill" {
			command.AssetKind, command.SourceScope = assets.AssetKindSkill, assets.SourceScopeImported
		} else {
			command.SourceScope = assets.SourceScopeLocal
			command.CandidateID = deterministicAssetID("candidate", operationID, input.DefinitionID, input.RevisionID)
			command.SourceReferenceDigest, command.ProvenanceDigest = artifactDigest, artifactDigest
		}
	case "instantiate_template":
		if !validAssetParameterValues(input.ParameterValues) {
			return assets.Command{}, ErrInvalidLocalProductAsset
		}
		parameterJSON, err := json.Marshal(input.ParameterValues)
		if err != nil || sha256Hex(parameterJSON) != input.ParameterDigest {
			return assets.Command{}, assets.ErrDigestMismatch
		}
		command.ArtifactDigest = input.RevisionDigest
		command.CandidateID = deterministicAssetID("template-output", operationID, input.DefinitionID, input.RevisionID)
		if revision, found := service.projection.GlobalReadView().EvolutionAssetRevision(string(input.AssetKind) + "/" + input.DefinitionID + "/" + input.RevisionID); found {
			command.TemplateOutput = revision.TemplateOutput
		}
	case "promote_run":
		command.SourceScope = assets.SourceScopePromoted
		if sha256Hex([]byte(input.RedactedSummary)) != input.RedactedSummaryDigest {
			return assets.Command{}, assets.ErrDigestMismatch
		}
	case "record_evaluation":
		if len(input.RequestedCaseIDs) == 0 {
			return assets.Command{}, ErrInvalidLocalProductAsset
		}
		requestedCaseIDs := nonNilStrings(input.RequestedCaseIDs)
		sort.Strings(requestedCaseIDs)
		for index, caseID := range requestedCaseIDs {
			if caseID == "" || index > 0 && requestedCaseIDs[index-1] == caseID {
				return assets.Command{}, ErrInvalidLocalProductAsset
			}
		}
		fixtureBytes, fixtureErr := service.artifacts.ReadArtifact(ctx, input.FixtureDigest, 1<<20)
		if fixtureErr != nil || len(fixtureBytes) == 0 {
			canonical, canonicalErr := CanonicalEvolutionEvaluationFixture(
				input.FixtureKind, requestedCaseIDs,
			)
			if canonicalErr != nil || sha256Hex(canonical) != input.FixtureDigest {
				return assets.Command{}, assets.ErrDigestMismatch
			}
			if _, publishErr := service.artifacts.Publish(
				ctx, bytes.NewReader(canonical), input.FixtureDigest,
			); publishErr != nil {
				return assets.Command{}, publishErr
			}
			fixtureBytes = canonical
		}
		fixture, fixtureErr := decodeEvaluationFixture(fixtureBytes, input.FixtureKind, requestedCaseIDs)
		if fixtureErr != nil {
			return assets.Command{}, fixtureErr
		}
		candidate, found := service.projection.GlobalReadView().EvolutionAssetCandidate(input.CandidateID)
		if !found || candidate.RevisionID != input.CandidateRevisionID {
			return assets.Command{}, assets.ErrNotFound
		}
		command.AssetKind = candidate.AssetKind
		command.DefinitionID = candidate.DefinitionID
		command.RevisionID = input.CandidateRevisionID
		command.ArtifactDigest = input.CandidateDigest
		candidateArtifact, readErr := service.artifacts.ReadArtifact(ctx, input.CandidateDigest, 16<<20)
		if readErr != nil || len(candidateArtifact) == 0 {
			return assets.Command{}, assets.ErrDigestMismatch
		}
		baselineFound := false
		for _, kind := range []assets.AssetKind{assets.AssetKindSkill, assets.AssetKindAgentTemplate, assets.AssetKindTeamTemplate, assets.AssetKindWorkPackageTemplate, assets.AssetKindRecoveryStrategyTemplate} {
			baseline, ok := service.projection.GlobalReadView().EvolutionAssetRevision(
				string(kind) + "/" + candidate.DefinitionID + "/" + input.BaselineRevisionID,
			)
			if ok && baseline.ArtifactDigest == input.BaselineDigest {
				baselineFound = true
				break
			}
		}
		if !baselineFound {
			return assets.Command{}, assets.ErrDigestMismatch
		}
		baselineArtifact, readErr := service.artifacts.ReadArtifact(ctx, input.BaselineDigest, 16<<20)
		if readErr != nil || len(baselineArtifact) == 0 {
			return assets.Command{}, assets.ErrDigestMismatch
		}
		evidenceID := deterministicAssetID("evaluation-evidence", operationID)
		evidenceBytes, marshalErr := canonicalAssetJSON(struct {
			SchemaVersion       int      `json:"schema_version"`
			EvaluationID        string   `json:"evaluation_id"`
			CandidateID         string   `json:"candidate_id"`
			FixtureKind         string   `json:"fixture_kind"`
			FixtureDigest       string   `json:"fixture_digest"`
			BaselineRevisionID  string   `json:"baseline_revision_id"`
			BaselineDigest      string   `json:"baseline_digest"`
			CandidateRevisionID string   `json:"candidate_revision_id"`
			CandidateDigest     string   `json:"candidate_digest"`
			RequestedCaseIDs    []string `json:"requested_case_ids"`
			QualityResult       string   `json:"quality_result"`
			FailureCount        int      `json:"failure_count"`
			CaseCount           int      `json:"case_count"`
			CompatibilityResult string   `json:"compatibility_result"`
			ApplicableScope     string   `json:"applicable_scope"`
			RegressionResult    string   `json:"regression_result"`
			SecurityResult      string   `json:"security_result"`
			UsageObserved       bool     `json:"usage_observed"`
			UsageMicrounits     int64    `json:"usage_microunits"`
			CostObserved        bool     `json:"cost_observed"`
			CostMicrounits      int64    `json:"cost_microunits"`
			CostCurrency        string   `json:"cost_currency"`
		}{
			1, input.EvaluationID, input.CandidateID, input.FixtureKind,
			input.FixtureDigest, input.BaselineRevisionID, input.BaselineDigest,
			input.CandidateRevisionID, input.CandidateDigest, requestedCaseIDs,
			fixture.Expected.QualityResult, fixture.Expected.FailureCount,
			len(requestedCaseIDs), fixture.Expected.CompatibilityResult,
			fixture.Expected.ApplicableScope, fixture.Expected.RegressionResult,
			fixture.Expected.SecurityResult, fixture.Expected.UsageObserved,
			fixture.Expected.UsageMicrounits, fixture.Expected.CostObserved,
			fixture.Expected.CostMicrounits, fixture.Expected.CostCurrency,
		})
		if marshalErr != nil {
			return assets.Command{}, marshalErr
		}
		evidenceDigest := sha256Hex(evidenceBytes)
		if _, publishErr := service.artifacts.Publish(ctx, bytes.NewReader(evidenceBytes), evidenceDigest); publishErr != nil {
			return assets.Command{}, publishErr
		}
		var usageValue, costValue *int64
		if fixture.Expected.UsageObserved {
			value := fixture.Expected.UsageMicrounits
			usageValue = &value
		}
		if fixture.Expected.CostObserved {
			value := fixture.Expected.CostMicrounits
			costValue = &value
		}
		command.RequestedCaseIDs = requestedCaseIDs
		command.Evaluation = &assets.EvaluationRecord{
			EvaluationID: input.EvaluationID, CandidateID: input.CandidateID,
			FixtureKind: input.FixtureKind, FixtureDigest: input.FixtureDigest,
			BaselineRevisionID: input.BaselineRevisionID, BaselineDigest: input.BaselineDigest,
			CandidateRevisionID: input.CandidateRevisionID, CandidateDigest: input.CandidateDigest,
			QualityResult: fixture.Expected.QualityResult, FailureCount: fixture.Expected.FailureCount,
			CaseCount: len(requestedCaseIDs), UsageObserved: fixture.Expected.UsageObserved,
			UsageValue: usageValue, CostObserved: fixture.Expected.CostObserved,
			CostValue: costValue, CompatibilityResult: fixture.Expected.CompatibilityResult,
			ApplicableScope:  fixture.Expected.ApplicableScope,
			RegressionResult: fixture.Expected.RegressionResult,
			SecurityResult:   fixture.Expected.SecurityResult,
			EvidenceID:       evidenceID, EvidenceDigest: evidenceDigest,
		}
	case "set_binding":
		command.Subject = assets.SubjectIdentity{SubjectKind: input.SubjectKind, SubjectID: input.SubjectID, SubjectVersion: input.SubjectVersion, SubjectDigest: input.SubjectDigest, Scope: input.SubjectScope, ProjectID: input.SubjectProjectID, GenerationID: input.SubjectGenerationID}
		command.AssetRevisionSetDigest = input.AssetRevisionSetDigest
		identityDigest, err := assets.CanonicalSubjectIdentityDigest(command.Subject)
		if err != nil || identityDigest != input.SubjectIdentityDigest {
			return assets.Command{}, assets.ErrDigestMismatch
		}
		computed, err := assets.CanonicalAssetRevisionSetDigest(command.Bindings)
		if err != nil || computed != input.AssetRevisionSetDigest {
			return assets.Command{}, assets.ErrDigestMismatch
		}
	case "activate", "reject", "retain", "archive", "restore":
		command.ArtifactDigest = input.RevisionDigest
		command.RequiredEvaluationIDs = nonNilStrings(input.EvaluationIDs)
		if action == "activate" || action == "reject" || action == "retain" {
			command.DecisionSource = "user_explicit"
		}
	case "rollback":
		command.RevisionID, command.ArtifactDigest = input.FromRevisionID, input.FromDigest
		command.RequiredEvaluationIDs = nonNilStrings(input.EvaluationIDs)
		command.DecisionSource = "user_explicit"
	}
	return command, nil
}

func exactJSONKeys(data []byte, want []string) bool {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil || len(object) != len(want) {
		return false
	}
	for _, key := range want {
		if _, ok := object[key]; !ok {
			return false
		}
	}
	return true
}

func validAssetParameterValues(values []assetParameterValue) bool {
	if len(values) > 32 {
		return false
	}
	for index, value := range values {
		if value.Name == "" || len(value.Name) > 64 || len(value.Value) > 1024 ||
			strings.ContainsAny(value.Name, "/\\\x00") || strings.ContainsRune(value.Value, '\x00') ||
			index > 0 && values[index-1].Name >= value.Name {
			return false
		}
	}
	return true
}

type evolutionAssetArtifactEntry struct {
	RelativePath  string `json:"relative_path"`
	FileMode      int    `json:"file_mode"`
	FileSize      int    `json:"file_size"`
	FileSHA256    string `json:"file_sha256"`
	ContentBase64 string `json:"content_base64"`
}

func CanonicalEvolutionAssetArtifact(kind assets.AssetKind, definitionID, revisionID, path string) ([]byte, string, string, error) {
	data, relativePath, err := readExactAssetSource(path)
	if err != nil || kind == "" || definitionID == "" || revisionID == "" {
		return nil, "", "", ErrInvalidLocalProductAsset
	}
	return CanonicalEvolutionAssetArtifactBytes(kind, definitionID, revisionID, relativePath, data)
}

func CanonicalEvolutionAssetArtifactBytes(kind assets.AssetKind, definitionID, revisionID, relativePath string, data []byte) ([]byte, string, string, error) {
	if kind == "" || definitionID == "" || revisionID == "" ||
		relativePath == "" || filepath.Base(relativePath) != relativePath ||
		len(data) < 1 || len(data) > 1<<20 {
		return nil, "", "", ErrInvalidLocalProductAsset
	}
	entry := evolutionAssetArtifactEntry{RelativePath: relativePath, FileMode: 384, FileSize: len(data), FileSHA256: sha256Hex(data), ContentBase64: base64.StdEncoding.EncodeToString(data)}
	return canonicalEvolutionAssetEnvelope(kind, definitionID, revisionID, []evolutionAssetArtifactEntry{entry})
}

func CanonicalEvolutionTemplateArtifact(
	kind assets.AssetKind,
	definitionID, revisionID, path string,
	templateOutput assets.TemplateOutput,
	parameterSchemaDigest, permissionCeilingDigest, scopeCeilingDigest string,
) ([]byte, string, string, error) {
	data, relativePath, err := readExactAssetSource(path)
	if err != nil || kind == assets.AssetKindSkill || kind == "" ||
		definitionID == "" || revisionID == "" || relativePath == "template-contract.json" ||
		templateOutput == "" || !validLowerSHA256(parameterSchemaDigest) ||
		!validLowerSHA256(permissionCeilingDigest) || !validLowerSHA256(scopeCeilingDigest) {
		return nil, "", "", ErrInvalidLocalProductAsset
	}
	sourceDigest := sha256Hex(data)
	contract, err := canonicalAssetJSON(struct {
		SchemaVersion           int                   `json:"schema_version"`
		AssetKind               assets.AssetKind      `json:"asset_kind"`
		TemplateOutput          assets.TemplateOutput `json:"template_output"`
		ParameterSchemaDigest   string                `json:"parameter_schema_digest"`
		PermissionCeilingDigest string                `json:"permission_ceiling_digest"`
		ScopeCeilingDigest      string                `json:"scope_ceiling_digest"`
		SourceFileSHA256        string                `json:"source_file_sha256"`
	}{1, kind, templateOutput, parameterSchemaDigest, permissionCeilingDigest,
		scopeCeilingDigest, sourceDigest})
	if err != nil {
		return nil, "", "", err
	}
	entries := []evolutionAssetArtifactEntry{
		{
			RelativePath: "template-contract.json", FileMode: 384,
			FileSize: len(contract), FileSHA256: sha256Hex(contract),
			ContentBase64: base64.StdEncoding.EncodeToString(contract),
		},
		{
			RelativePath: relativePath, FileMode: 384, FileSize: len(data),
			FileSHA256: sourceDigest, ContentBase64: base64.StdEncoding.EncodeToString(data),
		},
	}
	return canonicalEvolutionAssetEnvelope(kind, definitionID, revisionID, entries)
}

func canonicalEvolutionAssetEnvelope(
	kind assets.AssetKind,
	definitionID, revisionID string,
	entries []evolutionAssetArtifactEntry,
) ([]byte, string, string, error) {
	entriesBytes, err := canonicalAssetJSON(entries)
	if err != nil {
		return nil, "", "", err
	}
	contentDigest := sha256Hex(entriesBytes)
	artifactBytes, err := canonicalAssetJSON(struct {
		SchemaVersion int                           `json:"schema_version"`
		AssetKind     assets.AssetKind              `json:"asset_kind"`
		DefinitionID  string                        `json:"definition_id"`
		RevisionID    string                        `json:"revision_id"`
		Entries       []evolutionAssetArtifactEntry `json:"entries"`
		ContentDigest string                        `json:"content_digest"`
	}{1, kind, definitionID, revisionID, entries, contentDigest})
	if err != nil || len(artifactBytes) > 1572864 {
		return nil, "", "", ErrInvalidLocalProductAsset
	}
	return artifactBytes, sha256Hex(artifactBytes), contentDigest, nil
}

func validLowerSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}

func canonicalAssetJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte("\n")), nil
}

func readExactAssetSource(path string) ([]byte, string, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, "", ErrInvalidLocalProductAsset
	}
	for current := path; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return nil, "", ErrInvalidLocalProductAsset
		}
		if current == filepath.Dir(current) {
			break
		}
	}
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Size() < 1 || before.Size() > 1<<20 {
		return nil, "", ErrInvalidLocalProductAsset
	}
	if stat := reflect.ValueOf(before.Sys()); stat.IsValid() {
		if stat.Kind() == reflect.Pointer {
			stat = stat.Elem()
		}
		if stat.IsValid() && stat.Kind() == reflect.Struct {
			links := stat.FieldByName("Nlink")
			if links.IsValid() && links.CanUint() && links.Uint() != 1 {
				return nil, "", ErrInvalidLocalProductAsset
			}
		}
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, "", ErrInvalidLocalProductAsset
	}
	defer file.Close()
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) {
		return nil, "", ErrInvalidLocalProductAsset
	}
	data, err := io.ReadAll(io.LimitReader(file, 1<<20+1))
	if err != nil || len(data) == 0 || len(data) > 1<<20 {
		return nil, "", ErrInvalidLocalProductAsset
	}
	relativePath := filepath.Base(path)
	if !validEvolutionAssetRelativePath(relativePath) {
		return nil, "", ErrInvalidLocalProductAsset
	}
	return data, relativePath, nil
}

func validEvolutionAssetRelativePath(value string) bool {
	if value == "" || value == "." || value == ".." || strings.ContainsAny(value, "/\\\x00") {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f || character == 0x202e || character >= 0x202a && character <= 0x202d || character == 0x1b {
			return false
		}
	}
	return true
}

func deterministicAssetID(prefix string, values ...string) string {
	return prefix + "-" + sha256Hex([]byte(strings.Join(values, "\n")))[:32]
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func nonNilStrings(values []string) []string {
	result := make([]string, len(values))
	copy(result, values)
	return result
}

func nonNilDefinitions(values []assets.SkillDefinition) []assets.SkillDefinition {
	if values == nil {
		return []assets.SkillDefinition{}
	}
	return values
}
func nonNilRevisions(values []assets.SkillRevision) []assets.SkillRevision {
	result := make([]assets.SkillRevision, len(values))
	for index, value := range values {
		value.Dependencies = nonNilStrings(value.Dependencies)
		value.CompatibleRuntimeCapabilities = nonNilStrings(value.CompatibleRuntimeCapabilities)
		result[index] = value
	}
	return result
}
func nonNilCandidates(values []assets.EvolutionCandidate) []assets.EvolutionCandidate {
	result := make([]assets.EvolutionCandidate, len(values))
	for index, value := range values {
		value.SourceEvidenceIDs = nonNilStrings(value.SourceEvidenceIDs)
		value.SourceEvidenceDigests = nonNilStrings(value.SourceEvidenceDigests)
		value.RequiredEvaluationIDs = nonNilStrings(value.RequiredEvaluationIDs)
		result[index] = value
	}
	return result
}
func nonNilEvaluations(values []assets.EvaluationRecord) []assets.EvaluationRecord {
	if values == nil {
		return []assets.EvaluationRecord{}
	}
	return values
}
func nonNilBindings(values []assets.EvolutionAssetBindingRecord) []assets.EvolutionAssetBindingRecord {
	result := make([]assets.EvolutionAssetBindingRecord, len(values))
	for index, value := range values {
		value.Bindings = append([]assets.ExactAssetRevisionBinding{}, value.Bindings...)
		result[index] = value
	}
	return result
}
func nonNilMaterializations(values []assets.RuntimeSkillMaterializationRecord) []assets.RuntimeSkillMaterializationRecord {
	result := make([]assets.RuntimeSkillMaterializationRecord, len(values))
	for index, value := range values {
		value.AssetRevisionBindings = append([]assets.ExactAssetRevisionBinding{}, value.AssetRevisionBindings...)
		result[index] = value
	}
	return result
}

func nonNilBindingSubjects(values []EvolutionAssetBindingSubject) []EvolutionAssetBindingSubject {
	if values == nil {
		return []EvolutionAssetBindingSubject{}
	}
	return values
}

func appendUniqueRevision(values []assets.SkillRevision, value assets.SkillRevision) []assets.SkillRevision {
	for _, existing := range values {
		if existing.AssetKind == value.AssetKind && existing.DefinitionID == value.DefinitionID && existing.RevisionID == value.RevisionID {
			return values
		}
	}
	return append(values, value)
}
func stringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
func validAssetJourney(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' || value[14] != '4' || !strings.ContainsRune("89ab", rune(value[19])) {
		return false
	}
	for index, ch := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		if ch < '0' || ch > '9' && ch < 'a' || ch > 'f' {
			return false
		}
	}
	return true
}
