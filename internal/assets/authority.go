package assets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"loom-pi-rebuild/internal/journal"
)

var (
	ErrInvalidInput    = errors.New("invalid evolution asset input")
	ErrDenied          = errors.New("evolution asset command denied")
	ErrConflict        = errors.New("evolution asset conflict")
	ErrStaleView       = errors.New("stale evolution asset view")
	ErrStaleGeneration = errors.New("stale evolution asset generation")
	ErrDigestMismatch  = errors.New("evolution asset digest mismatch")
	ErrIncompatible    = errors.New("evolution asset incompatible")
	ErrNotFound        = errors.New("evolution asset not found")
)

type BindingSubjectResolver interface {
	ResolveBindingSubject(context.Context, SubjectIdentity) (SubjectIdentity, error)
}
type PromotionResolver interface {
	ResolvePromotion(context.Context, PromotionRequest) (PromotionSource, error)
}
type PromotionRequest struct {
	RunID           string
	RunGeneration   int64
	EvidenceIDs     []string
	AssetKind       AssetKind
	DefinitionID    string
	RevisionID      string
	RedactedSummary string
}
type PromotionSource struct {
	RunID                         string
	RunGeneration                 int64
	RunDigest                     string
	Terminal                      bool
	Accepted                      bool
	EvidenceAccepted              bool
	EvidenceIDs                   []string
	EvidenceDigests               []string
	ArtifactDigest                string
	ContentDigest                 string
	ProvenanceDigest              string
	Dependencies                  []string
	CompatibleRuntimeCapabilities []string
	Name                          string
	Description                   string
	Scope                         string
}
type TemplateOutputSink interface {
	CreateTemplateOutput(context.Context, TemplateInstantiation) error
}
type TemplateArtifactResolver interface {
	ResolveTemplateArtifact(context.Context, TemplateArtifactRequest) (TemplateArtifactContract, error)
}
type TemplateArtifactRequest struct {
	AssetKind      AssetKind
	DefinitionID   string
	RevisionID     string
	ArtifactDigest string
}
type TemplateArtifactContract struct {
	AssetKind               AssetKind
	DefinitionID            string
	RevisionID              string
	TemplateOutput          TemplateOutput
	ParameterSchemaDigest   string
	PermissionCeilingDigest string
	ScopeCeilingDigest      string
}

type Authority struct {
	store             *journal.Store
	now               func() time.Time
	viewVersion       func() string
	promotion         PromotionResolver
	subjects          BindingSubjectResolver
	templateOutputs   TemplateOutputSink
	templateArtifacts TemplateArtifactResolver
}
type AuthorityConfig struct {
	Store             *journal.Store
	Now               func() time.Time
	ViewVersion       func() string
	Promotion         PromotionResolver
	Subjects          BindingSubjectResolver
	TemplateOutputs   TemplateOutputSink
	TemplateArtifacts TemplateArtifactResolver
}

type Command struct {
	Action                     string                      `json:"action"`
	OperationID                string                      `json:"operation_id"`
	JourneyID                  string                      `json:"journey_id"`
	ExpectedViewVersion        string                      `json:"expected_view_version"`
	ExpectedStreamHeads        []journal.StreamHead        `json:"expected_stream_heads"`
	DecisionSource             string                      `json:"decision_source,omitempty"`
	AssetKind                  AssetKind                   `json:"asset_kind,omitempty"`
	DefinitionID               string                      `json:"definition_id,omitempty"`
	RevisionID                 string                      `json:"revision_id,omitempty"`
	CandidateID                string                      `json:"candidate_id,omitempty"`
	EvaluationID               string                      `json:"evaluation_id,omitempty"`
	Name                       string                      `json:"name,omitempty"`
	Description                string                      `json:"description,omitempty"`
	Scope                      string                      `json:"scope,omitempty"`
	ArtifactDigest             string                      `json:"artifact_digest,omitempty"`
	ContentDigest              string                      `json:"content_digest,omitempty"`
	SourceScope                SourceScope                 `json:"source_scope,omitempty"`
	SourceReferenceDigest      string                      `json:"source_reference_digest,omitempty"`
	ProvenanceDigest           string                      `json:"provenance_digest,omitempty"`
	Dependencies               []string                    `json:"dependencies,omitempty"`
	CompatibleCapabilities     []string                    `json:"compatible_runtime_capabilities,omitempty"`
	Risk                       Risk                        `json:"risk,omitempty"`
	TemplateOutput             TemplateOutput              `json:"template_output,omitempty"`
	ParameterSchemaDigest      string                      `json:"parameter_schema_digest,omitempty"`
	PermissionCeilingDigest    string                      `json:"permission_ceiling_digest,omitempty"`
	ScopeCeilingDigest         string                      `json:"scope_ceiling_digest,omitempty"`
	SourceRunID                string                      `json:"source_run_id,omitempty"`
	SourceRunGeneration        int64                       `json:"source_run_generation,omitempty"`
	SourceRunDigest            string                      `json:"source_run_digest,omitempty"`
	SourceEvidenceIDs          []string                    `json:"source_evidence_ids,omitempty"`
	SourceEvidenceDigests      []string                    `json:"source_evidence_digests,omitempty"`
	RedactedSummary            string                      `json:"redacted_summary,omitempty"`
	ScopeDifference            string                      `json:"scope_difference,omitempty"`
	ExpectedBenefit            string                      `json:"expected_benefit,omitempty"`
	RequiredEvaluationIDs      []string                    `json:"required_evaluation_ids,omitempty"`
	ReasonCode                 string                      `json:"reason_code,omitempty"`
	ExpectedPreviousRevisionID string                      `json:"expected_previous_revision_id,omitempty"`
	TargetRevisionID           string                      `json:"target_revision_id,omitempty"`
	TargetRevisionDigest       string                      `json:"target_revision_digest,omitempty"`
	Subject                    SubjectIdentity             `json:"subject,omitempty"`
	Bindings                   []ExactAssetRevisionBinding `json:"asset_revision_bindings,omitempty"`
	AssetRevisionSetDigest     string                      `json:"asset_revision_set_digest,omitempty"`
	ParametersDigest           string                      `json:"parameter_digest,omitempty"`
	ParameterValues            []ParameterValue            `json:"parameter_values,omitempty"`
	RequestedCaseIDs           []string                    `json:"requested_case_ids,omitempty"`
	Evaluation                 *EvaluationRecord           `json:"evaluation,omitempty"`
	TeamExecutionID            string                      `json:"team_execution_id,omitempty"`
	LogicalNodeID              string                      `json:"logical_node_id,omitempty"`
	RunID                      string                      `json:"run_id,omitempty"`
	AttemptNumber              int                         `json:"attempt_number,omitempty"`
	Generation                 int64                       `json:"generation,omitempty"`
	RuntimeInstanceID          string                      `json:"runtime_instance_id,omitempty"`
	RuntimeIdentityDigest      string                      `json:"runtime_identity_digest,omitempty"`
	Capability                 string                      `json:"capability,omitempty"`
	ManifestArtifactDigest     string                      `json:"manifest_artifact_digest,omitempty"`
	MaterializationRootDigest  string                      `json:"materialization_root_digest,omitempty"`
	CleanupResult              string                      `json:"cleanup_result,omitempty"`
}
type Result struct {
	EventIDs                      []string                    `json:"event_ids"`
	MaterializationManifestDigest string                      `json:"materialization_manifest_digest,omitempty"`
	Definition                    SkillDefinition             `json:"definition,omitempty"`
	Revision                      SkillRevision               `json:"revision,omitempty"`
	Candidate                     EvolutionCandidate          `json:"candidate,omitempty"`
	Evaluation                    EvaluationRecord            `json:"evaluation,omitempty"`
	Binding                       EvolutionAssetBindingRecord `json:"binding,omitempty"`
	TemplateInstantiation         TemplateInstantiation       `json:"template_instantiation,omitempty"`
	ViewVersion                   string                      `json:"view_version"`
	Heads                         []journal.StreamHead        `json:"heads"`
}

// PreparedMaterialization is an authority-produced, immutable Journal fact
// that another accepted writer may include in a larger multi-stream CAS. Its
// fields are intentionally private so callers cannot forge a publish fact.
type PreparedMaterialization struct {
	event            journal.Event
	heads            []journal.StreamHead
	command          Command
	alreadyCommitted bool
}

func (prepared PreparedMaterialization) Event() journal.Event {
	event := prepared.event
	event.PayloadJSON = append([]byte(nil), event.PayloadJSON...)
	return event
}

func (prepared PreparedMaterialization) Heads() []journal.StreamHead {
	return append([]journal.StreamHead(nil), prepared.heads...)
}

func (prepared PreparedMaterialization) AlreadyCommitted() bool {
	return prepared.alreadyCommitted
}

func (prepared PreparedMaterialization) Matches(
	logicalNodeID string,
	attemptNumber int,
	bindings []ExactAssetRevisionBinding,
	setDigest string,
	manifestDigest string,
	rootDigest string,
) bool {
	return prepared.event.Type == "RuntimeSkillMaterializationPublished" &&
		prepared.command.LogicalNodeID == logicalNodeID &&
		prepared.command.AttemptNumber == attemptNumber &&
		reflect.DeepEqual(prepared.command.Bindings, bindings) &&
		prepared.command.AssetRevisionSetDigest == setDigest &&
		prepared.command.ManifestArtifactDigest == manifestDigest &&
		prepared.command.MaterializationRootDigest == rootDigest
}

func NewAuthority(config AuthorityConfig) (*Authority, error) {
	if config.Store == nil || config.Now == nil {
		return nil, ErrInvalidInput
	}
	if config.ViewVersion == nil {
		config.ViewVersion = func() string { return "" }
	}
	return &Authority{store: config.Store, now: config.Now, viewVersion: config.ViewVersion, promotion: config.Promotion, subjects: config.Subjects, templateOutputs: config.TemplateOutputs, templateArtifacts: config.TemplateArtifacts}, nil
}

func (authority *Authority) CreateSkill(ctx context.Context, command Command) (Result, error) {
	command.Action = "create_skill"
	return authority.create(ctx, command, false)
}
func (authority *Authority) ImportSkill(ctx context.Context, command Command) (Result, error) {
	command.Action = "import_skill"
	command.SourceScope = SourceScopeImported
	return authority.create(ctx, command, true)
}
func (authority *Authority) CreateTemplate(ctx context.Context, command Command) (Result, error) {
	command.Action = "create_template"
	if command.AssetKind == AssetKindSkill || command.TemplateOutput != templateOutputForKind(command.AssetKind) ||
		!validDigest(command.ParameterSchemaDigest) || !validDigest(command.PermissionCeilingDigest) || !validDigest(command.ScopeCeilingDigest) {
		return Result{}, ErrInvalidInput
	}
	if err := authority.validateTemplateArtifact(ctx, command); err != nil {
		return Result{}, err
	}
	return authority.create(ctx, command, false)
}

func (authority *Authority) create(ctx context.Context, command Command, imported bool) (Result, error) {
	if err := authority.validateCommon(ctx, command); err != nil {
		return Result{}, err
	}
	if !validAssetKind(command.AssetKind) || !validID(command.DefinitionID) || !validID(command.RevisionID) || !validID(command.CandidateID) || !validDigest(command.ArtifactDigest) || !validDigest(command.ContentDigest) || !validDigest(command.SourceReferenceDigest) || !validDigest(command.ProvenanceDigest) || !validSourceScope(command.SourceScope) || !validRisk(command.Risk) || command.Name == "" || command.Scope == "" {
		return Result{}, ErrInvalidInput
	}
	if !validName(command.Name) || !validBoundedText(command.Description) ||
		!validBoundedText(command.RedactedSummary) ||
		len(command.Dependencies) > 32 ||
		len(command.CompatibleCapabilities) > 32 {
		return Result{}, ErrInvalidInput
	}
	definitionStream := definitionStream(command.AssetKind, command.DefinitionID)
	revisionStream := revisionStream(command.AssetKind, command.DefinitionID, command.RevisionID)
	candidateStream := candidateStream(command.CandidateID)
	streams := []string{definitionStream, revisionStream, candidateStream}
	snapshot, state, err := authority.readState(ctx, streams, command.ExpectedStreamHeads)
	if err != nil {
		return Result{}, err
	}
	intent, err := CanonicalIntentDigest(command)
	if err != nil {
		return Result{}, err
	}
	_, revisionExists := state.snapshot.Revisions[assetRevisionKey(command.AssetKind, command.DefinitionID, command.RevisionID)]
	_, candidateExists := state.snapshot.Candidates[command.CandidateID]
	if revisionExists || candidateExists {
		committed, exact := exactOperationEvents(snapshot.Events(), command.OperationID, intent)
		if exact {
			return authority.resultFrom(state, committed, command, streams), nil
		}
		return Result{}, ErrConflict
	}
	now := authority.now().UTC()
	events := []journal.Event{}
	planIndex := 0
	if _, exists := state.snapshot.Definitions[assetDefinitionKey(command.AssetKind, command.DefinitionID)]; !exists {
		payload := definitionCreatedPayload{SchemaVersion: 1, OperationID: command.OperationID, AssetKind: command.AssetKind, DefinitionID: command.DefinitionID, Name: command.Name, Description: command.Description, Scope: command.Scope}
		event, buildErr := newAssetEvent("EvolutionAssetDefinitionCreated", definitionStream, snapshotHead(snapshot, definitionStream).Sequence+1, command, intent, planIndex, now, payload)
		if buildErr != nil {
			return Result{}, buildErr
		}
		events = append(events, event)
		planIndex++
	}
	revisionPayload := revisionCreatedPayload{SchemaVersion: 1, OperationID: command.OperationID, AssetKind: command.AssetKind, DefinitionID: command.DefinitionID, RevisionID: command.RevisionID, ArtifactDigest: command.ArtifactDigest, ContentDigest: command.ContentDigest, SourceScope: command.SourceScope, SourceReferenceDigest: command.SourceReferenceDigest, ProvenanceDigest: command.ProvenanceDigest, Dependencies: nonNilStrings(command.Dependencies), CompatibleRuntimeCapabilities: nonNilStrings(command.CompatibleCapabilities), Risk: command.Risk, Lifecycle: LifecycleCandidate}
	event, err := newAssetEvent("EvolutionAssetRevisionCreated", revisionStream, snapshotHead(snapshot, revisionStream).Sequence+1, command, intent, planIndex, now, revisionPayload)
	if err != nil {
		return Result{}, err
	}
	events = append(events, event)
	planIndex++
	if imported {
		payload := struct {
			SchemaVersion        int       `json:"schema_version"`
			OperationID          string    `json:"operation_id"`
			CandidateID          string    `json:"candidate_id"`
			AssetKind            AssetKind `json:"asset_kind"`
			DefinitionID         string    `json:"definition_id"`
			RevisionID           string    `json:"revision_id"`
			ExternalSourceDigest string    `json:"external_source_digest"`
			ProvenanceDigest     string    `json:"provenance_digest"`
			Risk                 Risk      `json:"risk"`
		}{1, command.OperationID, command.CandidateID, command.AssetKind, command.DefinitionID, command.RevisionID, command.SourceReferenceDigest, command.ProvenanceDigest, command.Risk}
		event, err := newAssetEvent("EvolutionAssetImportProposed", revisionStream, snapshotHead(snapshot, revisionStream).Sequence+2, command, intent, planIndex, now, payload)
		if err != nil {
			return Result{}, err
		}
		events = append(events, event)
		planIndex++
	}
	candidateSequence := snapshotHead(snapshot, candidateStream).Sequence + 1
	if command.SourceScope == SourceScopePromoted &&
		(!validID(command.SourceRunID) || command.SourceRunGeneration < 1 ||
			!validDigest(command.SourceRunDigest) || len(command.SourceEvidenceIDs) == 0 ||
			len(command.SourceEvidenceIDs) != len(command.SourceEvidenceDigests) ||
			!validDigestList(command.SourceEvidenceDigests)) {
		return Result{}, ErrInvalidInput
	}
	candidate := EvolutionCandidate{CandidateID: command.CandidateID, AssetKind: command.AssetKind, DefinitionID: command.DefinitionID, RevisionID: command.RevisionID, SourceScope: command.SourceScope, SourceRunID: command.SourceRunID, SourceRunGeneration: command.SourceRunGeneration, SourceRunDigest: command.SourceRunDigest, SourceEvidenceIDs: nonNilStrings(command.SourceEvidenceIDs), SourceEvidenceDigests: nonNilStrings(command.SourceEvidenceDigests), RedactedSummary: command.RedactedSummary, RedactedSummaryDigest: sha256Text([]byte(command.RedactedSummary)), ScopeDifference: command.ScopeDifference, ExpectedBenefit: command.ExpectedBenefit, Risk: command.Risk, RequiredEvaluationIDs: nonNilStrings(command.RequiredEvaluationIDs)}
	event, err = newAssetEvent("EvolutionAssetCandidateCreated", candidateStream, candidateSequence, command, intent, planIndex, now, candidateCreatedPayload{SchemaVersion: 1, OperationID: command.OperationID, CandidateID: candidate.CandidateID, AssetKind: candidate.AssetKind, DefinitionID: candidate.DefinitionID, RevisionID: candidate.RevisionID, SourceScope: candidate.SourceScope, SourceRunID: candidate.SourceRunID, SourceRunGeneration: candidate.SourceRunGeneration, SourceRunDigest: candidate.SourceRunDigest, SourceEvidenceIDs: nonNilStrings(candidate.SourceEvidenceIDs), SourceEvidenceDigests: nonNilStrings(candidate.SourceEvidenceDigests), RedactedSummary: candidate.RedactedSummary, RedactedSummaryDigest: candidate.RedactedSummaryDigest, ScopeDifference: candidate.ScopeDifference, ExpectedBenefit: candidate.ExpectedBenefit, Risk: candidate.Risk, RequiredEvaluationIDs: nonNilStrings(candidate.RequiredEvaluationIDs)})
	if err != nil {
		return Result{}, err
	}
	events = append(events, event)
	planIndex++
	if command.SourceScope == SourceScopePromoted {
		payload := runPromotionPayload{
			SchemaVersion: 1, OperationID: command.OperationID,
			CandidateID: command.CandidateID, AssetKind: command.AssetKind,
			DefinitionID: command.DefinitionID, RevisionID: command.RevisionID,
			SourceRunID:           command.SourceRunID,
			SourceRunGeneration:   command.SourceRunGeneration,
			SourceRunDigest:       command.SourceRunDigest,
			SourceEvidenceIDs:     nonNilStrings(command.SourceEvidenceIDs),
			SourceEvidenceDigests: nonNilStrings(command.SourceEvidenceDigests),
			RedactedSummaryDigest: sha256Text([]byte(command.RedactedSummary)),
		}
		promotionEvent, buildErr := newAssetEvent(
			"EvolutionRunPromotionProposed", candidateStream, candidateSequence+1,
			command, intent, planIndex, now, payload,
		)
		if buildErr != nil {
			return Result{}, buildErr
		}
		events = append(events, promotionEvent)
	}
	committed, err := authority.append(ctx, snapshot, events)
	if err != nil {
		return Result{}, err
	}
	replayed, err := replay(append(snapshot.Events(), committed...))
	if err != nil {
		return Result{}, err
	}
	return authority.resultFrom(replayed, committed, command, streams), nil
}

func (authority *Authority) InstantiateTemplate(ctx context.Context, command Command) (Result, error) {
	command.Action = "instantiate_template"
	if err := authority.validateCommon(ctx, command); err != nil {
		return Result{}, err
	}
	if !validAssetKind(command.AssetKind) || command.AssetKind == AssetKindSkill || !validID(command.DefinitionID) || !validID(command.RevisionID) || command.TemplateOutput == "" || !validDigest(command.ArtifactDigest) || !validDigest(command.ParametersDigest) {
		return Result{}, ErrInvalidInput
	}
	stream := revisionStream(command.AssetKind, command.DefinitionID, command.RevisionID)
	activation := activationStream(command.AssetKind, command.DefinitionID)
	snapshot, state, err := authority.readState(ctx, []string{stream, activation}, command.ExpectedStreamHeads)
	if err != nil {
		return Result{}, err
	}
	revision, ok := state.snapshot.Revisions[assetRevisionKey(command.AssetKind, command.DefinitionID, command.RevisionID)]
	if !ok || revision.ArtifactDigest != command.ArtifactDigest || state.archived[assetRevisionKey(command.AssetKind, command.DefinitionID, command.RevisionID)] {
		return Result{}, ErrDenied
	}
	if err := authority.validateTemplateArtifact(ctx, command); err != nil {
		return Result{}, err
	}
	output := TemplateInstantiation{AssetKind: command.AssetKind, TemplateOutput: command.TemplateOutput, OutputCandidateID: command.CandidateID, OutputDigest: command.ParametersDigest, CreatedAt: authority.now().UTC()}
	if authority.templateOutputs == nil {
		return Result{}, ErrDenied
	}
	if err := authority.templateOutputs.CreateTemplateOutput(ctx, output); err != nil {
		return Result{}, err
	}
	intent, _ := CanonicalIntentDigest(command)
	payload := struct {
		SchemaVersion     int            `json:"schema_version"`
		OperationID       string         `json:"operation_id"`
		AssetKind         AssetKind      `json:"asset_kind"`
		DefinitionID      string         `json:"definition_id"`
		RevisionID        string         `json:"revision_id"`
		RevisionDigest    string         `json:"revision_digest"`
		TemplateOutput    TemplateOutput `json:"template_output"`
		ParameterDigest   string         `json:"parameter_digest"`
		OutputCandidateID string         `json:"output_candidate_id"`
		OutputDigest      string         `json:"output_digest"`
	}{1, command.OperationID, command.AssetKind, command.DefinitionID, command.RevisionID, revision.ArtifactDigest, command.TemplateOutput, command.ParametersDigest, command.CandidateID, command.ParametersDigest}
	event, err := newAssetEvent("EvolutionTemplateInstantiated", stream, snapshotHead(snapshot, stream).Sequence+1, command, intent, 0, authority.now().UTC(), payload)
	if err != nil {
		return Result{}, err
	}
	committed, err := authority.append(ctx, snapshot, []journal.Event{event})
	if err != nil {
		return Result{}, err
	}
	return Result{EventIDs: eventIDs(committed), TemplateInstantiation: output, ViewVersion: authority.viewVersion(), Heads: snapshot.Heads()}, nil
}

func (authority *Authority) validateTemplateArtifact(
	ctx context.Context,
	command Command,
) error {
	if authority.templateArtifacts == nil {
		return ErrDenied
	}
	contract, err := authority.templateArtifacts.ResolveTemplateArtifact(
		ctx,
		TemplateArtifactRequest{
			AssetKind: command.AssetKind, DefinitionID: command.DefinitionID,
			RevisionID: command.RevisionID, ArtifactDigest: command.ArtifactDigest,
		},
	)
	if err != nil || contract.AssetKind != command.AssetKind ||
		contract.DefinitionID != command.DefinitionID ||
		contract.RevisionID != command.RevisionID ||
		contract.TemplateOutput != command.TemplateOutput ||
		command.ParameterSchemaDigest != "" && contract.ParameterSchemaDigest != command.ParameterSchemaDigest ||
		command.PermissionCeilingDigest != "" && contract.PermissionCeilingDigest != command.PermissionCeilingDigest ||
		command.ScopeCeilingDigest != "" && contract.ScopeCeilingDigest != command.ScopeCeilingDigest ||
		!validDigest(contract.ParameterSchemaDigest) ||
		!validDigest(contract.PermissionCeilingDigest) ||
		!validDigest(contract.ScopeCeilingDigest) {
		return ErrDenied
	}
	return nil
}

func (authority *Authority) PromoteRun(ctx context.Context, command Command) (Result, error) {
	command.Action = "promote_run"
	if err := authority.validateCommon(ctx, command); err != nil {
		return Result{}, err
	}
	if authority.promotion == nil {
		return Result{}, ErrDenied
	}
	source, err := authority.promotion.ResolvePromotion(ctx, PromotionRequest{
		RunID: command.SourceRunID, RunGeneration: command.SourceRunGeneration,
		EvidenceIDs: append([]string(nil), command.SourceEvidenceIDs...),
		AssetKind:   command.AssetKind, DefinitionID: command.DefinitionID,
		RevisionID: command.RevisionID, RedactedSummary: command.RedactedSummary,
	})
	if err != nil {
		return Result{}, ErrDenied
	}
	if !source.Terminal || !source.Accepted || !source.EvidenceAccepted || source.RunID != command.SourceRunID || source.RunGeneration != command.SourceRunGeneration || source.RunDigest != command.SourceRunDigest || !reflect.DeepEqual(source.EvidenceIDs, command.SourceEvidenceIDs) || !reflect.DeepEqual(source.EvidenceDigests, command.SourceEvidenceDigests) {
		return Result{}, ErrDenied
	}
	if !validDigest(source.ArtifactDigest) || !validDigest(source.ContentDigest) || !validDigest(source.ProvenanceDigest) {
		return Result{}, ErrDenied
	}
	command.ArtifactDigest = source.ArtifactDigest
	command.ContentDigest = source.ContentDigest
	command.SourceReferenceDigest = source.RunDigest
	command.ProvenanceDigest = source.ProvenanceDigest
	command.Dependencies = nonNilStrings(source.Dependencies)
	command.CompatibleCapabilities = nonNilStrings(source.CompatibleRuntimeCapabilities)
	command.Name, command.Description, command.Scope = source.Name, source.Description, source.Scope
	command.SourceScope = SourceScopePromoted
	return authority.create(ctx, command, false)
}

func (authority *Authority) RecordEvaluation(ctx context.Context, command Command) (Result, error) {
	command.Action = "record_evaluation"
	if err := authority.validateCommon(ctx, command); err != nil {
		return Result{}, err
	}
	if command.Evaluation == nil {
		return Result{}, ErrInvalidInput
	}
	evaluation := *command.Evaluation
	if command.EvaluationID != "" && command.EvaluationID != evaluation.EvaluationID {
		return Result{}, ErrInvalidInput
	}
	command.EvaluationID = evaluation.EvaluationID
	if !validEvaluation(evaluation) || evaluation.CandidateID != command.CandidateID ||
		evaluation.CandidateRevisionID != command.RevisionID {
		return Result{}, ErrInvalidInput
	}
	streams := []string{
		candidateStream(command.CandidateID),
		revisionStream(command.AssetKind, command.DefinitionID, command.RevisionID),
		evaluationStream(evaluation.EvaluationID),
	}
	snapshot, state, err := authority.readState(ctx, streams, command.ExpectedStreamHeads)
	if err != nil {
		return Result{}, err
	}
	intent, err := CanonicalIntentDigest(command)
	if err != nil {
		return Result{}, err
	}
	if existing, exists := state.snapshot.Evaluations[evaluation.EvaluationID]; exists {
		committed, exact := exactOperationEvents(snapshot.Events(), command.OperationID, intent)
		if exact && reflect.DeepEqual(existing, evaluation) {
			return Result{EventIDs: eventIDs(committed), Evaluation: existing, Candidate: state.snapshot.Candidates[command.CandidateID], ViewVersion: authority.viewVersion(), Heads: snapshot.Heads()}, nil
		}
		return Result{}, ErrConflict
	}
	candidate, exists := state.snapshot.Candidates[command.CandidateID]
	if !exists || candidate.AssetKind != command.AssetKind || candidate.DefinitionID != command.DefinitionID || candidate.RevisionID != command.RevisionID || candidate.Decision != "" {
		return Result{}, ErrConflict
	}
	revision, exists := state.snapshot.Revisions[assetRevisionKey(command.AssetKind, command.DefinitionID, command.RevisionID)]
	if !exists {
		return Result{}, ErrNotFound
	}
	if revision.ArtifactDigest != evaluation.CandidateDigest || command.ArtifactDigest != "" && command.ArtifactDigest != evaluation.CandidateDigest {
		return Result{}, ErrDigestMismatch
	}
	event, err := newAssetEvent(
		"EvolutionAssetEvaluationRecorded",
		evaluationStream(evaluation.EvaluationID),
		snapshotHead(snapshot, evaluationStream(evaluation.EvaluationID)).Sequence+1,
		command,
		intent,
		0,
		authority.now().UTC(),
		newEvaluationPayload(command.OperationID, evaluation),
	)
	if err != nil {
		return Result{}, err
	}
	committed, err := authority.append(ctx, snapshot, []journal.Event{event})
	if err != nil {
		return Result{}, err
	}
	replayed, err := replay(append(snapshot.Events(), committed...))
	if err != nil {
		return Result{}, err
	}
	return Result{EventIDs: eventIDs(committed), Evaluation: replayed.snapshot.Evaluations[evaluation.EvaluationID], Candidate: replayed.snapshot.Candidates[command.CandidateID], ViewVersion: authority.viewVersion(), Heads: snapshot.Heads()}, nil
}

func (authority *Authority) SetBinding(ctx context.Context, command Command) (Result, error) {
	command.Action = "set_binding"
	if err := authority.validateCommon(ctx, command); err != nil {
		return Result{}, err
	}
	if authority.subjects == nil || !validSubject(command.Subject) {
		return Result{}, ErrDenied
	}
	resolved, err := authority.subjects.ResolveBindingSubject(ctx, command.Subject)
	if err != nil {
		return Result{}, ErrDenied
	}
	if resolved.GenerationID != command.Subject.GenerationID {
		return Result{}, ErrStaleGeneration
	}
	if resolved != command.Subject {
		return Result{}, ErrConflict
	}
	subjectDigest, err := CanonicalSubjectIdentityDigest(command.Subject)
	if err != nil {
		return Result{}, err
	}
	setDigest, err := CanonicalAssetRevisionSetDigest(command.Bindings)
	if err != nil {
		return Result{}, err
	}
	if command.AssetRevisionSetDigest != "" && command.AssetRevisionSetDigest != setDigest {
		return Result{}, ErrDigestMismatch
	}
	streams := []string{bindingStream(command.Subject.SubjectKind, subjectDigest)}
	for _, binding := range command.Bindings {
		streams = append(streams, revisionStream(binding.AssetKind, binding.DefinitionID, binding.RevisionID), activationStream(binding.AssetKind, binding.DefinitionID))
	}
	snapshot, state, err := authority.readState(ctx, streams, command.ExpectedStreamHeads)
	if err != nil {
		return Result{}, err
	}
	for _, binding := range command.Bindings {
		key := assetRevisionKey(binding.AssetKind, binding.DefinitionID, binding.RevisionID)
		revision, ok := state.snapshot.Revisions[key]
		if !ok {
			return Result{}, ErrConflict
		}
		if revision.ArtifactDigest != binding.SHA256Digest {
			return Result{}, ErrDigestMismatch
		}
		if state.archived[key] || state.active[assetDefinitionKey(binding.AssetKind, binding.DefinitionID)] != binding.RevisionID {
			return Result{}, ErrDenied
		}
	}
	intent, _ := CanonicalIntentDigest(command)
	stream := bindingStream(command.Subject.SubjectKind, subjectDigest)
	payload := bindingPayload{
		SchemaVersion: 1, OperationID: command.OperationID,
		SubjectKind: command.Subject.SubjectKind, SubjectID: command.Subject.SubjectID,
		SubjectVersion: command.Subject.SubjectVersion, SubjectDigest: command.Subject.SubjectDigest,
		SubjectScope: command.Subject.Scope, SubjectProjectID: command.Subject.ProjectID,
		SubjectGenerationID: command.Subject.GenerationID, SubjectIdentityDigest: subjectDigest,
		Bindings: append([]ExactAssetRevisionBinding(nil), command.Bindings...), AssetRevisionSetDigest: setDigest,
		BindingRevision: snapshotHead(snapshot, stream).Sequence + 1,
	}
	event, err := newAssetEvent("EvolutionAssetBindingSetCommitted", stream, snapshotHead(snapshot, stream).Sequence+1, command, intent, 0, authority.now().UTC(), payload)
	if err != nil {
		return Result{}, err
	}
	committed, err := authority.append(ctx, snapshot, []journal.Event{event})
	if err != nil {
		return Result{}, err
	}
	return Result{EventIDs: eventIDs(committed), Binding: EvolutionAssetBindingRecord{
		SchemaVersion: 1, SubjectKind: command.Subject.SubjectKind, SubjectID: command.Subject.SubjectID,
		SubjectVersion: command.Subject.SubjectVersion, SubjectDigest: command.Subject.SubjectDigest,
		SubjectScope: command.Subject.Scope, SubjectProjectID: command.Subject.ProjectID,
		SubjectGenerationID: command.Subject.GenerationID, SubjectIdentityDigest: subjectDigest,
		Bindings: append([]ExactAssetRevisionBinding(nil), command.Bindings...), AssetRevisionSetDigest: setDigest,
		BindingRevision: committed[0].Seq, LastEventID: committed[0].ID, LastJourneyID: command.JourneyID,
	}, ViewVersion: authority.viewVersion(), Heads: snapshot.Heads()}, nil
}

func (authority *Authority) ActivateCandidate(ctx context.Context, command Command) (Result, error) {
	command.Action = "activate"
	return authority.decide(ctx, command, "EvolutionAssetCandidateActivated")
}
func (authority *Authority) RejectCandidate(ctx context.Context, command Command) (Result, error) {
	command.Action = "reject"
	return authority.decide(ctx, command, "EvolutionAssetCandidateRejected")
}
func (authority *Authority) RetainCandidate(ctx context.Context, command Command) (Result, error) {
	command.Action = "retain"
	return authority.decide(ctx, command, "EvolutionAssetCandidateRetained")
}

func (authority *Authority) decide(ctx context.Context, command Command, eventType string) (Result, error) {
	if err := authority.validateCommon(ctx, command); err != nil {
		return Result{}, err
	}
	if command.DecisionSource != "user_explicit" {
		return Result{}, ErrDenied
	}
	if eventType != "EvolutionAssetCandidateActivated" && !validAssetReasonCode(command.Action, command.ReasonCode) {
		return Result{}, ErrInvalidInput
	}
	streams := []string{candidateStream(command.CandidateID), revisionStream(command.AssetKind, command.DefinitionID, command.RevisionID), activationStream(command.AssetKind, command.DefinitionID), definitionStream(command.AssetKind, command.DefinitionID)}
	for _, evaluationID := range command.RequiredEvaluationIDs {
		streams = append(streams, evaluationStream(evaluationID))
	}
	snapshot, state, err := authority.readState(ctx, streams, command.ExpectedStreamHeads)
	if err != nil {
		return Result{}, err
	}
	candidate, ok := state.snapshot.Candidates[command.CandidateID]
	if !ok || candidate.AssetKind != command.AssetKind || candidate.DefinitionID != command.DefinitionID || candidate.RevisionID != command.RevisionID || candidate.Decision != "" {
		return Result{}, fmt.Errorf("%w: candidate=%#v found=%t command=%s/%s/%s", ErrConflict, candidate, ok, command.AssetKind, command.DefinitionID, command.RevisionID)
	}
	revision, ok := state.snapshot.Revisions[assetRevisionKey(command.AssetKind, command.DefinitionID, command.RevisionID)]
	if !ok {
		return Result{}, ErrNotFound
	}
	if command.ArtifactDigest != "" && revision.ArtifactDigest != command.ArtifactDigest {
		return Result{}, ErrDigestMismatch
	}
	if state.archived[assetRevisionKey(command.AssetKind, command.DefinitionID, command.RevisionID)] {
		return Result{}, ErrDenied
	}
	if eventType == "EvolutionAssetCandidateActivated" {
		expectedEvaluations := uniqueSorted(candidate.RequiredEvaluationIDs)
		providedEvaluations := uniqueSorted(command.RequiredEvaluationIDs)
		if len(providedEvaluations) != len(command.RequiredEvaluationIDs) || !reflect.DeepEqual(expectedEvaluations, providedEvaluations) {
			return Result{}, ErrDenied
		}
		for _, evaluationID := range providedEvaluations {
			evaluation, exists := state.snapshot.Evaluations[evaluationID]
			if !exists || evaluation.CandidateID != candidate.CandidateID ||
				evaluation.CandidateDigest != revision.ArtifactDigest ||
				evaluation.QualityResult != "pass" || evaluation.CompatibilityResult != "compatible" ||
				evaluation.SecurityResult != "pass" || evaluation.RegressionResult == "regressed" {
				return Result{}, ErrDenied
			}
		}
		current := state.active[assetDefinitionKey(command.AssetKind, command.DefinitionID)]
		if command.ExpectedPreviousRevisionID != "" && command.ExpectedPreviousRevisionID != current {
			return Result{}, ErrConflict
		}
	}
	intent, _ := CanonicalIntentDigest(command)
	stream := candidateStream(command.CandidateID)
	payload := any(decisionPayload{SchemaVersion: 1, OperationID: command.OperationID, CandidateID: command.CandidateID, AssetKind: command.AssetKind, DefinitionID: command.DefinitionID, RevisionID: command.RevisionID, RevisionDigest: revision.ArtifactDigest, ReasonCode: command.ReasonCode, DecisionSource: "user_explicit"})
	if eventType == "EvolutionAssetCandidateActivated" {
		stream = activationStream(command.AssetKind, command.DefinitionID)
		payload = activatedPayload{SchemaVersion: 1, OperationID: command.OperationID, AssetKind: command.AssetKind, CandidateID: command.CandidateID, DefinitionID: command.DefinitionID, RevisionID: command.RevisionID, RevisionDigest: revision.ArtifactDigest, ExpectedPreviousRevisionID: command.ExpectedPreviousRevisionID, EvaluationIDs: nonNilStrings(command.RequiredEvaluationIDs), DecisionSource: "user_explicit"}
	}
	event, err := newAssetEvent(eventType, stream, snapshotHead(snapshot, stream).Sequence+1, command, intent, 0, authority.now().UTC(), payload)
	if err != nil {
		return Result{}, err
	}
	committed, err := authority.append(ctx, snapshot, []journal.Event{event})
	if err != nil {
		return Result{}, err
	}
	if eventType == "EvolutionAssetCandidateActivated" {
		candidate.Decision = "activate"
		candidate.DecisionEventID = committed[0].ID
		revision.Lifecycle = LifecycleActive
	}
	return Result{EventIDs: eventIDs(committed), Candidate: candidate, Revision: revision, ViewVersion: authority.viewVersion(), Heads: snapshot.Heads()}, nil
}

func (authority *Authority) ArchiveRevision(ctx context.Context, command Command) (Result, error) {
	command.Action = "archive"
	return authority.revisionLifecycle(ctx, command, true)
}
func (authority *Authority) RestoreRevision(ctx context.Context, command Command) (Result, error) {
	command.Action = "restore"
	return authority.revisionLifecycle(ctx, command, false)
}
func (authority *Authority) revisionLifecycle(ctx context.Context, command Command, archive bool) (Result, error) {
	if err := authority.validateCommon(ctx, command); err != nil {
		return Result{}, err
	}
	if !validAssetReasonCode(command.Action, command.ReasonCode) {
		return Result{}, ErrInvalidInput
	}
	stream := revisionStream(command.AssetKind, command.DefinitionID, command.RevisionID)
	activation := activationStream(command.AssetKind, command.DefinitionID)
	snapshot, state, err := authority.readState(ctx, []string{stream, activation}, command.ExpectedStreamHeads)
	if err != nil {
		return Result{}, err
	}
	revision, ok := state.snapshot.Revisions[assetRevisionKey(command.AssetKind, command.DefinitionID, command.RevisionID)]
	if !ok {
		return Result{}, ErrNotFound
	}
	if state.archived[assetRevisionKey(command.AssetKind, command.DefinitionID, command.RevisionID)] == archive {
		return Result{}, ErrConflict
	}
	eventType := "EvolutionAssetRevisionArchived"
	if command.ArtifactDigest == "" || command.ArtifactDigest != revision.ArtifactDigest {
		return Result{}, ErrDigestMismatch
	}
	previousLifecycle := revision.Lifecycle
	if state.active[assetDefinitionKey(command.AssetKind, command.DefinitionID)] == command.RevisionID {
		previousLifecycle = LifecycleActive
	}
	payload := revisionLifecyclePayload{SchemaVersion: 1, OperationID: command.OperationID, AssetKind: command.AssetKind, DefinitionID: command.DefinitionID, RevisionID: command.RevisionID, RevisionDigest: revision.ArtifactDigest, PreviousLifecycle: previousLifecycle, ReasonCode: command.ReasonCode}
	if !archive {
		eventType = "EvolutionAssetRevisionRestored"
		payload.PreviousLifecycle = ""
		payload.RestoredLifecycle = LifecycleCandidate
	}
	intent, _ := CanonicalIntentDigest(command)
	event, err := newAssetEvent(eventType, stream, snapshotHead(snapshot, stream).Sequence+1, command, intent, 0, authority.now().UTC(), payload)
	if err != nil {
		return Result{}, err
	}
	committed, err := authority.append(ctx, snapshot, []journal.Event{event})
	if err != nil {
		return Result{}, err
	}
	if archive {
		revision.Lifecycle = LifecycleArchived
	} else {
		revision.Lifecycle = LifecycleCandidate
	}
	return Result{EventIDs: eventIDs(committed), Revision: revision, ViewVersion: authority.viewVersion()}, nil
}

func (authority *Authority) RollbackActivation(ctx context.Context, command Command) (Result, error) {
	command.Action = "rollback"
	if err := authority.validateCommon(ctx, command); err != nil {
		return Result{}, err
	}
	if command.DecisionSource != "user_explicit" || !validAssetReasonCode(command.Action, command.ReasonCode) {
		return Result{}, ErrInvalidInput
	}
	activation := activationStream(command.AssetKind, command.DefinitionID)
	targetStream := revisionStream(command.AssetKind, command.DefinitionID, command.TargetRevisionID)
	streams := []string{activation, targetStream}
	snapshot, state, err := authority.readState(ctx, streams, command.ExpectedStreamHeads)
	if err != nil {
		return Result{}, err
	}
	key := assetDefinitionKey(command.AssetKind, command.DefinitionID)
	from := state.active[key]
	if from == "" {
		return Result{}, ErrConflict
	}
	if command.RevisionID != "" && command.RevisionID != from ||
		command.ArtifactDigest != "" && command.ArtifactDigest != state.activeDigests[key] {
		return Result{}, ErrConflict
	}
	target, ok := state.snapshot.Revisions[assetRevisionKey(command.AssetKind, command.DefinitionID, command.TargetRevisionID)]
	if !ok || state.archived[assetRevisionKey(command.AssetKind, command.DefinitionID, command.TargetRevisionID)] {
		return Result{}, ErrDenied
	}
	if target.ArtifactDigest != command.TargetRevisionDigest {
		return Result{}, ErrDigestMismatch
	}
	payload := rollbackPayload{SchemaVersion: 1, OperationID: command.OperationID, AssetKind: command.AssetKind, DefinitionID: command.DefinitionID, FromRevisionID: from, ToRevisionID: command.TargetRevisionID, FromDigest: state.activeDigests[key], ToDigest: target.ArtifactDigest, EvaluationIDs: nonNilStrings(command.RequiredEvaluationIDs), ReasonCode: command.ReasonCode, DecisionSource: "user_explicit"}
	intent, _ := CanonicalIntentDigest(command)
	event, err := newAssetEvent("EvolutionAssetActivationRolledBack", activation, snapshotHead(snapshot, activation).Sequence+1, command, intent, 0, authority.now().UTC(), payload)
	if err != nil {
		return Result{}, err
	}
	committed, err := authority.append(ctx, snapshot, []journal.Event{event})
	if err != nil {
		return Result{}, err
	}
	target.Lifecycle = LifecycleActive
	return Result{EventIDs: eventIDs(committed), Revision: target, ViewVersion: authority.viewVersion()}, nil
}

func (authority *Authority) PublishMaterialization(ctx context.Context, command Command) (Result, error) {
	prepared, err := authority.PrepareMaterialization(ctx, command)
	if err != nil {
		return Result{}, err
	}
	expectations := make([]journal.StreamHeadExpectation, 0, len(prepared.heads))
	for _, head := range prepared.heads {
		expectations = append(expectations, journal.StreamHeadExpectation{
			StreamID: head.StreamID, Sequence: head.Sequence,
		})
	}
	committed, err := authority.store.AppendBatchIfStreamHeads(
		ctx, expectations, []journal.Event{prepared.Event()},
	)
	if err != nil {
		if errors.Is(err, journal.ErrStreamHeadConflict) ||
			errors.Is(err, journal.ErrIdempotencyConflict) ||
			errors.Is(err, journal.ErrSequenceConflict) ||
			errors.Is(err, journal.ErrPartialEventBatchConflict) {
			return Result{}, fmt.Errorf("%w: journal: %v", ErrConflict, err)
		}
		return Result{}, err
	}
	return Result{
		EventIDs:                      eventIDs(committed),
		MaterializationManifestDigest: command.ManifestArtifactDigest,
		ViewVersion:                   authority.viewVersion(),
	}, nil
}

func (authority *Authority) PrepareMaterialization(
	ctx context.Context,
	command Command,
) (PreparedMaterialization, error) {
	command.Action = "publish_materialization"
	if err := authority.validateCommon(ctx, command); err != nil {
		return PreparedMaterialization{}, err
	}
	if !validID(command.TeamExecutionID) || !validID(command.LogicalNodeID) || !validID(command.RunID) ||
		command.AttemptNumber < 1 || command.Generation < 1 || !validID(command.RuntimeInstanceID) ||
		!validDigest(command.RuntimeIdentityDigest) || command.Capability != "loom.skill-materialization.pi.v1" ||
		!validDigest(command.ManifestArtifactDigest) || !validDigest(command.MaterializationRootDigest) ||
		len(command.Bindings) < 1 || len(command.Bindings) > 32 {
		return PreparedMaterialization{}, ErrInvalidInput
	}
	setDigest, err := CanonicalAssetRevisionSetDigest(command.Bindings)
	if err != nil || setDigest != command.AssetRevisionSetDigest {
		return PreparedMaterialization{}, ErrDigestMismatch
	}
	stream := materializationStream(command.RunID, command.AttemptNumber, command.Generation)
	streams := []string{stream}
	for _, binding := range command.Bindings {
		streams = append(streams, revisionStream(binding.AssetKind, binding.DefinitionID, binding.RevisionID), activationStream(binding.AssetKind, binding.DefinitionID))
	}
	snapshot, state, err := authority.readState(ctx, streams, command.ExpectedStreamHeads)
	if err != nil {
		return PreparedMaterialization{}, err
	}
	if existing := state.snapshot.Materializations[materializationKey(command.RunID, command.AttemptNumber, command.Generation)]; existing.RunID != "" {
		intent, digestErr := CanonicalIntentDigest(command)
		if digestErr != nil {
			return PreparedMaterialization{}, digestErr
		}
		committed, exact := exactOperationEvents(snapshot.Events(), command.OperationID, intent)
		if exact {
			if len(committed) != 1 {
				return PreparedMaterialization{}, ErrConflict
			}
			return PreparedMaterialization{
				event: committed[0], heads: snapshot.Heads(), command: command,
				alreadyCommitted: true,
			}, nil
		}
		return PreparedMaterialization{}, ErrConflict
	}
	for _, binding := range command.Bindings {
		revision, ok := state.snapshot.Revisions[assetRevisionKey(binding.AssetKind, binding.DefinitionID, binding.RevisionID)]
		if !ok || revision.ArtifactDigest != binding.SHA256Digest || state.active[assetDefinitionKey(binding.AssetKind, binding.DefinitionID)] != binding.RevisionID || state.archived[assetRevisionKey(binding.AssetKind, binding.DefinitionID, binding.RevisionID)] {
			return PreparedMaterialization{}, ErrDenied
		}
	}
	intent, err := CanonicalIntentDigest(command)
	if err != nil {
		return PreparedMaterialization{}, err
	}
	payload := materializationPublishedPayload{SchemaVersion: 1, OperationID: command.OperationID, TeamExecutionID: command.TeamExecutionID, LogicalNodeID: command.LogicalNodeID, RunID: command.RunID, AttemptNumber: command.AttemptNumber, Generation: command.Generation, RuntimeInstanceID: command.RuntimeInstanceID, RuntimeIdentityDigest: command.RuntimeIdentityDigest, Capability: command.Capability, AssetRevisionBindings: append([]ExactAssetRevisionBinding(nil), command.Bindings...), AssetRevisionSetDigest: setDigest, ManifestArtifactDigest: command.ManifestArtifactDigest, MaterializationRootDigest: command.MaterializationRootDigest}
	event, err := newAssetEvent("RuntimeSkillMaterializationPublished", stream, snapshotHead(snapshot, stream).Sequence+1, command, intent, 0, authority.now().UTC(), payload)
	if err != nil {
		return PreparedMaterialization{}, err
	}
	return PreparedMaterialization{
		event: event, heads: snapshot.Heads(), command: command,
	}, nil
}

func (authority *Authority) CleanMaterialization(ctx context.Context, command Command) (Result, error) {
	command.Action = "clean_materialization"
	if err := authority.validateCommon(ctx, command); err != nil {
		return Result{}, err
	}
	if !validID(command.RunID) || command.AttemptNumber < 1 || command.Generation < 1 || !validDigest(command.ManifestArtifactDigest) || !validDigest(command.MaterializationRootDigest) || command.CleanupResult != "removed" {
		return Result{}, ErrInvalidInput
	}
	stream := materializationStream(command.RunID, command.AttemptNumber, command.Generation)
	snapshot, state, err := authority.readState(ctx, []string{stream}, command.ExpectedStreamHeads)
	if err != nil {
		return Result{}, err
	}
	intent, err := CanonicalIntentDigest(command)
	if err != nil {
		return Result{}, err
	}
	record := state.snapshot.Materializations[materializationKey(command.RunID, command.AttemptNumber, command.Generation)]
	if record.Cleaned {
		committed, exact := exactOperationEvents(snapshot.Events(), command.OperationID, intent)
		if exact {
			return Result{
				EventIDs:                      eventIDs(committed),
				MaterializationManifestDigest: command.ManifestArtifactDigest,
				ViewVersion:                   authority.viewVersion(),
			}, nil
		}
		return Result{}, ErrConflict
	}
	if record.RunID == "" || record.ManifestArtifactDigest != command.ManifestArtifactDigest || record.MaterializationRootDigest != command.MaterializationRootDigest {
		return Result{}, ErrConflict
	}
	payload := materializationCleanedPayload{SchemaVersion: 1, OperationID: command.OperationID, RunID: command.RunID, AttemptNumber: command.AttemptNumber, Generation: command.Generation, ManifestArtifactDigest: command.ManifestArtifactDigest, MaterializationRootDigest: command.MaterializationRootDigest, CleanupResult: "removed"}
	event, err := newAssetEvent("RuntimeSkillMaterializationCleaned", stream, snapshotHead(snapshot, stream).Sequence+1, command, intent, 0, authority.now().UTC(), payload)
	if err != nil {
		return Result{}, err
	}
	committed, err := authority.append(ctx, snapshot, []journal.Event{event})
	if err != nil {
		return Result{}, err
	}
	return Result{EventIDs: eventIDs(committed), MaterializationManifestDigest: command.ManifestArtifactDigest, ViewVersion: authority.viewVersion()}, nil
}

func (authority *Authority) validateCommon(ctx context.Context, command Command) error {
	if authority == nil || authority.store == nil || ctx == nil {
		return ErrInvalidInput
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validID(command.OperationID) || !validJourneyUUID(command.JourneyID) {
		return ErrInvalidInput
	}
	current := authority.viewVersion()
	if current != "" && command.ExpectedViewVersion != current {
		return ErrStaleView
	}
	return nil
}
func (authority *Authority) readState(ctx context.Context, streams []string, expected []journal.StreamHead) (journal.StreamSetSnapshot, replayState, error) {
	streams = uniqueSorted(streams)
	snapshot, err := authority.store.ReadStreamSet(ctx, streams)
	if err != nil {
		return journal.StreamSetSnapshot{}, replayState{}, err
	}
	if len(expected) > 0 && !equalHeads(expected, snapshot.Heads()) {
		return journal.StreamSetSnapshot{}, replayState{}, ErrConflict
	}
	state, err := replay(snapshot.Events())
	return snapshot, state, err
}
func (authority *Authority) append(ctx context.Context, snapshot journal.StreamSetSnapshot, events []journal.Event) ([]journal.Event, error) {
	expectations := make([]journal.StreamHeadExpectation, 0, len(snapshot.Heads()))
	for _, head := range snapshot.Heads() {
		expectations = append(expectations, journal.StreamHeadExpectation{StreamID: head.StreamID, Sequence: head.Sequence})
	}
	committed, err := authority.store.AppendBatchIfStreamHeads(ctx, expectations, events)
	if errors.Is(err, journal.ErrStreamHeadConflict) || errors.Is(err, journal.ErrIdempotencyConflict) || errors.Is(err, journal.ErrSequenceConflict) || errors.Is(err, journal.ErrPartialEventBatchConflict) {
		return nil, fmt.Errorf("%w: journal: %v", ErrConflict, err)
	}
	return committed, err
}
func (authority *Authority) resultFrom(state replayState, events []journal.Event, command Command, streams []string) Result {
	return Result{EventIDs: eventIDs(events), Definition: state.snapshot.Definitions[assetDefinitionKey(command.AssetKind, command.DefinitionID)], Revision: state.snapshot.Revisions[assetRevisionKey(command.AssetKind, command.DefinitionID, command.RevisionID)], Candidate: state.snapshot.Candidates[command.CandidateID], ViewVersion: authority.viewVersion()}
}
func newAssetEvent(eventType, stream string, sequence int64, command Command, intent string, index int, now time.Time, payload any) (journal.Event, error) {
	eventID, key, err := EventIdentity(eventType, stream, command.OperationID, intent, index)
	if err != nil {
		return journal.Event{}, err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return journal.Event{}, ErrInvalidInput
	}
	return journal.Event{ID: eventID, StreamID: stream, Seq: sequence, IdempotencyKey: key, Type: eventType, SchemaVersion: 1, EmittedAt: now, CorrelationID: command.JourneyID, PayloadJSON: data}, nil
}
func eventIDs(events []journal.Event) []string {
	ids := make([]string, len(events))
	for index, event := range events {
		ids[index] = event.ID
	}
	return ids
}
func definitionStream(kind AssetKind, definitionID string) string {
	return "evolution-asset-definition/" + string(kind) + "/" + definitionID
}
func revisionStream(kind AssetKind, definitionID, revisionID string) string {
	return "evolution-asset-revision/" + string(kind) + "/" + definitionID + "/" + revisionID
}
func candidateStream(candidateID string) string { return "evolution-asset-candidate/" + candidateID }
func evaluationStream(evaluationID string) string {
	return "evolution-asset-evaluation/" + evaluationID
}
func activationStream(kind AssetKind, definitionID string) string {
	return "evolution-asset-activation/" + string(kind) + "/" + definitionID
}
func bindingStream(kind, digest string) string {
	return "evolution-asset-binding/" + kind + "/" + digest
}
func materializationStream(runID string, attempt int, generation int64) string {
	return fmt.Sprintf("runtime-skill-materialization/%s/%d/%d", runID, attempt, generation)
}
func snapshotHead(snapshot journal.StreamSetSnapshot, streamID string) journal.StreamHead {
	head, _ := snapshot.Head(streamID)
	return head
}
func uniqueSorted(values []string) []string {
	set := map[string]struct{}{}
	for _, value := range values {
		if value != "" {
			set[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
func equalHeads(left, right []journal.StreamHead) bool {
	left = append([]journal.StreamHead(nil), left...)
	right = append([]journal.StreamHead(nil), right...)
	sort.Slice(left, func(i, j int) bool { return left[i].StreamID < left[j].StreamID })
	sort.Slice(right, func(i, j int) bool { return right[i].StreamID < right[j].StreamID })
	return reflect.DeepEqual(left, right)
}

func validEvaluation(record EvaluationRecord) bool {
	if !validID(record.EvaluationID) || !validID(record.CandidateID) ||
		(record.FixtureKind != "historical" && record.FixtureKind != "synthetic") ||
		!validDigest(record.FixtureDigest) || !validID(record.BaselineRevisionID) ||
		!validDigest(record.BaselineDigest) || !validID(record.CandidateRevisionID) ||
		!validDigest(record.CandidateDigest) || record.QualityResult == "" ||
		record.FailureCount < 0 || record.CaseCount < 1 ||
		record.FailureCount > record.CaseCount ||
		!validEvaluationResult(record.QualityResult, "pass", "fail", "partial") ||
		!validEvaluationResult(record.CompatibilityResult, "compatible", "incompatible", "partial") ||
		record.ApplicableScope == "" ||
		!validEvaluationResult(record.RegressionResult, "improved", "equivalent", "regressed", "unknown") ||
		!validEvaluationResult(record.SecurityResult, "pass", "fail", "partial") ||
		!validID(record.EvidenceID) ||
		!validDigest(record.EvidenceDigest) {
		return false
	}
	if record.UsageObserved != (record.UsageValue != nil) ||
		record.CostObserved != (record.CostValue != nil) {
		return false
	}
	return (record.UsageValue == nil || *record.UsageValue >= 0) &&
		(record.CostValue == nil || *record.CostValue >= 0)
}

func validAssetReasonCode(action, reason string) bool {
	switch action {
	case "reject":
		return reason == "user_rejected" || reason == "risk_unacceptable" || reason == "evaluation_failed"
	case "retain":
		return reason == "keep_for_later" || reason == "superseded"
	case "archive":
		return reason == "archive_requested"
	case "restore":
		return reason == "restore_requested"
	case "rollback":
		return reason == "rollback_requested"
	}
	return false
}

func newEvaluationPayload(operationID string, record EvaluationRecord) evaluationPayload {
	usage, cost := int64(0), int64(0)
	if record.UsageValue != nil {
		usage = *record.UsageValue
	}
	if record.CostValue != nil {
		cost = *record.CostValue
	}
	currency := ""
	if record.CostObserved {
		currency = "USD"
	}
	return evaluationPayload{
		SchemaVersion: 1, OperationID: operationID,
		EvaluationID: record.EvaluationID, CandidateID: record.CandidateID,
		FixtureKind: record.FixtureKind, FixtureDigest: record.FixtureDigest,
		BaselineRevisionID: record.BaselineRevisionID, BaselineDigest: record.BaselineDigest,
		CandidateRevisionID: record.CandidateRevisionID, CandidateDigest: record.CandidateDigest,
		QualityResult: record.QualityResult, FailureCount: record.FailureCount, CaseCount: record.CaseCount,
		UsageObserved: record.UsageObserved, UsageMicrounits: usage,
		CostObserved: record.CostObserved, CostMicrounits: cost, CostCurrency: currency,
		CompatibilityResult: record.CompatibilityResult, ApplicableScope: record.ApplicableScope,
		RegressionResult: record.RegressionResult, SecurityResult: record.SecurityResult,
		EvidenceID: record.EvidenceID, EvidenceDigest: record.EvidenceDigest,
	}
}

func validDigestList(values []string) bool {
	for _, value := range values {
		if !validDigest(value) {
			return false
		}
	}
	return true
}
func validJourneyUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' || value[14] != '4' || value[19] != '8' && value[19] != '9' && value[19] != 'a' && value[19] != 'b' {
		return false
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}

func exactOperationEvents(events []journal.Event, operationID, intentDigest string) ([]journal.Event, bool) {
	prefix := "p3a/" + operationID + "/"
	matched := make([]journal.Event, 0)
	for _, event := range events {
		if !strings.HasPrefix(event.IdempotencyKey, prefix) {
			continue
		}
		parts := strings.Split(event.IdempotencyKey, "/")
		if len(parts) != 4 {
			return nil, false
		}
		index, err := strconv.Atoi(parts[3])
		if err != nil {
			return nil, false
		}
		eventID, key, err := EventIdentity(event.Type, event.StreamID, operationID, intentDigest, index)
		if err != nil || event.ID != eventID || event.IdempotencyKey != key {
			return nil, false
		}
		matched = append(matched, event)
	}
	sort.Slice(matched, func(i, j int) bool {
		leftParts := strings.Split(matched[i].IdempotencyKey, "/")
		rightParts := strings.Split(matched[j].IdempotencyKey, "/")
		left, _ := strconv.Atoi(leftParts[len(leftParts)-1])
		right, _ := strconv.Atoi(rightParts[len(rightParts)-1])
		return left < right
	})
	return matched, len(matched) > 0
}
