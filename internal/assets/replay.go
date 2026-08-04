package assets

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"loom-pi-rebuild/internal/journal"
)

type Snapshot struct {
	Definitions      map[string]SkillDefinition
	Revisions        map[string]SkillRevision
	Candidates       map[string]EvolutionCandidate
	Evaluations      map[string]EvaluationRecord
	Bindings         map[string]EvolutionAssetBindingRecord
	Materializations map[string]RuntimeSkillMaterializationRecord
}

type RuntimeSkillMaterializationRecord struct {
	SchemaVersion             int                         `json:"schema_version"`
	TeamExecutionID           string                      `json:"team_execution_id"`
	LogicalNodeID             string                      `json:"logical_node_id"`
	RunID                     string                      `json:"run_id"`
	AttemptNumber             int                         `json:"attempt_number"`
	Generation                int64                       `json:"generation"`
	RuntimeInstanceID         string                      `json:"runtime_instance_id"`
	RuntimeIdentityDigest     string                      `json:"runtime_identity_digest"`
	Capability                string                      `json:"capability"`
	AssetRevisionBindings     []ExactAssetRevisionBinding `json:"asset_revision_bindings"`
	AssetRevisionSetDigest    string                      `json:"asset_revision_set_digest"`
	ManifestArtifactDigest    string                      `json:"manifest_artifact_digest"`
	MaterializationRootDigest string                      `json:"materialization_root_digest"`
	JourneyID                 string                      `json:"journey_id"`
	Cleaned                   bool                        `json:"cleaned"`
	CleanupResult             string                      `json:"cleanup_result"`
	LastEventID               string                      `json:"last_event_id"`
}

type replayState struct {
	snapshot      Snapshot
	active        map[string]string
	activeDigests map[string]string
	archived      map[string]bool
}

func Replay(events []journal.Event) (Snapshot, error) {
	state, err := replay(events)
	if err != nil {
		return Snapshot{}, err
	}
	return cloneSnapshot(state.snapshot), nil
}

func replay(events []journal.Event) (replayState, error) {
	state := replayState{snapshot: emptyAssetSnapshot(), active: map[string]string{}, activeDigests: map[string]string{}, archived: map[string]bool{}}
	for _, event := range events {
		if event.SchemaVersion != 1 || hasDuplicateJSONKeys(event.PayloadJSON) {
			return replayState{}, ErrInvalidInput
		}
		var common commonPayload
		if err := json.Unmarshal(event.PayloadJSON, &common); err != nil || common.SchemaVersion != 1 || !validID(common.OperationID) {
			return replayState{}, ErrInvalidInput
		}
		switch event.Type {
		case "EvolutionAssetDefinitionCreated":
			var payload definitionCreatedPayload
			if err := decodeStrict(event.PayloadJSON, &payload); err != nil {
				return replayState{}, err
			}
			if !validAssetKind(payload.AssetKind) || !validID(payload.DefinitionID) ||
				!validName(payload.Name) || !validBoundedText(payload.Description) ||
				payload.Scope == "" {
				return replayState{}, ErrInvalidInput
			}
			key := assetDefinitionKey(payload.AssetKind, payload.DefinitionID)
			definition := SkillDefinition{DefinitionID: payload.DefinitionID, Name: payload.Name, Description: payload.Description, Scope: payload.Scope, CreatedEventID: event.ID, Lifecycle: LifecycleCandidate, Head: event.Seq}
			if existing, ok := state.snapshot.Definitions[key]; ok {
				if existing.CreatedEventID != "" && existing.CreatedEventID != event.ID {
					return replayState{}, ErrConflict
				}
				definition.ActiveRevisionID = existing.ActiveRevisionID
				if existing.ActiveRevisionID != "" {
					definition.Lifecycle = LifecycleActive
				}
			}
			state.snapshot.Definitions[key] = definition
		case "EvolutionAssetRevisionCreated":
			var payload revisionCreatedPayload
			if err := decodeStrict(event.PayloadJSON, &payload); err != nil {
				return replayState{}, err
			}
			if !validAssetKind(payload.AssetKind) || !validID(payload.DefinitionID) ||
				!validID(payload.RevisionID) || !validDigest(payload.ArtifactDigest) ||
				!validDigest(payload.ContentDigest) ||
				!validDigest(payload.SourceReferenceDigest) ||
				!validDigest(payload.ProvenanceDigest) ||
				!validSourceScope(payload.SourceScope) || !validRisk(payload.Risk) ||
				payload.Lifecycle != LifecycleCandidate ||
				len(payload.Dependencies) > 32 ||
				len(payload.CompatibleRuntimeCapabilities) > 32 {
				return replayState{}, ErrInvalidInput
			}
			key := assetRevisionKey(payload.AssetKind, payload.DefinitionID, payload.RevisionID)
			revision := SkillRevision{AssetKind: payload.AssetKind, DefinitionID: payload.DefinitionID, RevisionID: payload.RevisionID, ArtifactDigest: payload.ArtifactDigest, ContentDigest: payload.ContentDigest, SourceScope: payload.SourceScope, SourceReferenceDigest: payload.SourceReferenceDigest, ProvenanceDigest: payload.ProvenanceDigest, Dependencies: nonNilStrings(payload.Dependencies), CompatibleRuntimeCapabilities: nonNilStrings(payload.CompatibleRuntimeCapabilities), Risk: payload.Risk, Lifecycle: payload.Lifecycle, CreatedEventID: event.ID}
			revision.TemplateOutput = templateOutputForKind(payload.AssetKind)
			if state.active[assetDefinitionKey(payload.AssetKind, payload.DefinitionID)] == payload.RevisionID {
				revision.Lifecycle = LifecycleActive
			}
			if existing, ok := state.snapshot.Revisions[key]; ok && existing.CreatedEventID != event.ID {
				return replayState{}, ErrConflict
			}
			state.snapshot.Revisions[key] = revision
			definitionKey := assetDefinitionKey(payload.AssetKind, payload.DefinitionID)
			definition := state.snapshot.Definitions[definitionKey]
			definition.LatestRevisionID = payload.RevisionID
			state.snapshot.Definitions[definitionKey] = definition
		case "EvolutionAssetCandidateCreated":
			var payload candidateCreatedPayload
			if err := decodeStrict(event.PayloadJSON, &payload); err != nil {
				return replayState{}, err
			}
			if !validID(payload.CandidateID) || !validAssetKind(payload.AssetKind) ||
				!validID(payload.DefinitionID) || !validID(payload.RevisionID) ||
				!validSourceScope(payload.SourceScope) || !validRisk(payload.Risk) ||
				len(payload.SourceEvidenceIDs) != len(payload.SourceEvidenceDigests) ||
				!validDigestList(payload.SourceEvidenceDigests) ||
				!validBoundedText(payload.RedactedSummary) ||
				len(payload.RequiredEvaluationIDs) > 64 {
				return replayState{}, ErrInvalidInput
			}
			if payload.SourceScope == SourceScopePromoted {
				if !validID(payload.SourceRunID) || payload.SourceRunGeneration < 1 ||
					!validDigest(payload.SourceRunDigest) ||
					len(payload.SourceEvidenceIDs) == 0 {
					return replayState{}, ErrInvalidInput
				}
			} else if payload.SourceRunID != "" || payload.SourceRunGeneration != 0 ||
				payload.SourceRunDigest != "" ||
				len(payload.SourceEvidenceIDs) != 0 ||
				len(payload.SourceEvidenceDigests) != 0 {
				return replayState{}, ErrInvalidInput
			}
			if sha256Text([]byte(payload.RedactedSummary)) != payload.RedactedSummaryDigest {
				return replayState{}, ErrInvalidInput
			}
			candidate := EvolutionCandidate{
				CandidateID: payload.CandidateID, AssetKind: payload.AssetKind,
				DefinitionID: payload.DefinitionID, RevisionID: payload.RevisionID,
				SourceScope: payload.SourceScope, SourceRunID: payload.SourceRunID,
				SourceRunGeneration:   payload.SourceRunGeneration,
				SourceRunDigest:       payload.SourceRunDigest,
				SourceEvidenceIDs:     payload.SourceEvidenceIDs,
				SourceEvidenceDigests: payload.SourceEvidenceDigests,
				RedactedSummary:       payload.RedactedSummary,
				RedactedSummaryDigest: payload.RedactedSummaryDigest,
				ScopeDifference:       payload.ScopeDifference,
				ExpectedBenefit:       payload.ExpectedBenefit,
				Risk:                  payload.Risk, RequiredEvaluationIDs: payload.RequiredEvaluationIDs,
			}
			if existing, exists := state.snapshot.Candidates[candidate.CandidateID]; exists {
				if existing.CandidateID != "" {
					return replayState{}, ErrConflict
				}
				candidate.Decision = existing.Decision
				candidate.DecisionEventID = existing.DecisionEventID
			}
			state.snapshot.Candidates[candidate.CandidateID] = candidate
		case "EvolutionAssetCandidateActivated":
			var payload activatedPayload
			if err := decodeStrict(event.PayloadJSON, &payload); err != nil {
				return replayState{}, err
			}
			if !validAssetKind(payload.AssetKind) || !validID(payload.CandidateID) ||
				!validID(payload.DefinitionID) || !validID(payload.RevisionID) ||
				!validDigest(payload.RevisionDigest) ||
				payload.DecisionSource != "user_explicit" ||
				payload.ExpectedPreviousRevisionID != "" &&
					!validID(payload.ExpectedPreviousRevisionID) ||
				len(payload.EvaluationIDs) > 64 {
				return replayState{}, ErrInvalidInput
			}
			candidate := state.snapshot.Candidates[payload.CandidateID]
			candidate.Decision = "activate"
			candidate.DecisionEventID = event.ID
			state.snapshot.Candidates[payload.CandidateID] = candidate
			key := assetDefinitionKey(payload.AssetKind, payload.DefinitionID)
			previousRevisionID := state.active[key]
			if previousRevisionID != "" && previousRevisionID != payload.RevisionID {
				previousKey := assetRevisionKey(payload.AssetKind, payload.DefinitionID, previousRevisionID)
				if previous, found := state.snapshot.Revisions[previousKey]; found && previous.Lifecycle == LifecycleActive {
					previous.Lifecycle = LifecycleCandidate
					state.snapshot.Revisions[previousKey] = previous
				}
			}
			state.active[key] = payload.RevisionID
			state.activeDigests[key] = payload.RevisionDigest
			revisionKey := assetRevisionKey(payload.AssetKind, payload.DefinitionID, payload.RevisionID)
			if revision, found := state.snapshot.Revisions[revisionKey]; found {
				revision.Lifecycle = LifecycleActive
				state.snapshot.Revisions[revisionKey] = revision
			}
			definition := state.snapshot.Definitions[key]
			definition.ActiveRevisionID = payload.RevisionID
			definition.Lifecycle = LifecycleActive
			state.snapshot.Definitions[key] = definition
		case "EvolutionAssetCandidateRejected", "EvolutionAssetCandidateRetained":
			var payload decisionPayload
			if err := decodeStrict(event.PayloadJSON, &payload); err != nil {
				return replayState{}, err
			}
			if !validID(payload.CandidateID) || !validAssetKind(payload.AssetKind) ||
				!validID(payload.DefinitionID) || !validID(payload.RevisionID) ||
				!validDigest(payload.RevisionDigest) ||
				payload.DecisionSource != "user_explicit" {
				return replayState{}, ErrInvalidInput
			}
			action := "retain"
			if event.Type == "EvolutionAssetCandidateRejected" {
				action = "reject"
			}
			if !validAssetReasonCode(action, payload.ReasonCode) {
				return replayState{}, ErrInvalidInput
			}
			candidate := state.snapshot.Candidates[payload.CandidateID]
			if event.Type == "EvolutionAssetCandidateRejected" {
				candidate.Decision = "reject"
			} else {
				candidate.Decision = "retain"
			}
			candidate.DecisionEventID = event.ID
			state.snapshot.Candidates[payload.CandidateID] = candidate
		case "EvolutionAssetRevisionArchived":
			var payload revisionLifecyclePayload
			if err := decodeStrict(event.PayloadJSON, &payload); err != nil {
				return replayState{}, err
			}
			if !validReplayLifecycle(payload, true) {
				return replayState{}, ErrInvalidInput
			}
			state.archived[assetRevisionKey(payload.AssetKind, payload.DefinitionID, payload.RevisionID)] = true
			revisionKey := assetRevisionKey(payload.AssetKind, payload.DefinitionID, payload.RevisionID)
			revision := state.snapshot.Revisions[revisionKey]
			revision.Lifecycle = LifecycleArchived
			state.snapshot.Revisions[revisionKey] = revision
		case "EvolutionAssetRevisionRestored":
			var payload revisionLifecyclePayload
			if err := decodeStrict(event.PayloadJSON, &payload); err != nil {
				return replayState{}, err
			}
			if !validReplayLifecycle(payload, false) {
				return replayState{}, ErrInvalidInput
			}
			state.archived[assetRevisionKey(payload.AssetKind, payload.DefinitionID, payload.RevisionID)] = false
			revisionKey := assetRevisionKey(payload.AssetKind, payload.DefinitionID, payload.RevisionID)
			revision := state.snapshot.Revisions[revisionKey]
			revision.Lifecycle = payload.RestoredLifecycle
			state.snapshot.Revisions[revisionKey] = revision
		case "EvolutionAssetActivationRolledBack":
			var payload rollbackPayload
			if err := decodeStrict(event.PayloadJSON, &payload); err != nil {
				return replayState{}, err
			}
			if !validAssetKind(payload.AssetKind) || !validID(payload.DefinitionID) ||
				!validID(payload.FromRevisionID) || !validID(payload.ToRevisionID) ||
				!validDigest(payload.FromDigest) || !validDigest(payload.ToDigest) ||
				payload.DecisionSource != "user_explicit" ||
				!validAssetReasonCode("rollback", payload.ReasonCode) ||
				len(payload.EvaluationIDs) > 64 {
				return replayState{}, ErrInvalidInput
			}
			key := assetDefinitionKey(payload.AssetKind, payload.DefinitionID)
			fromKey := assetRevisionKey(payload.AssetKind, payload.DefinitionID, payload.FromRevisionID)
			if from, found := state.snapshot.Revisions[fromKey]; found && from.Lifecycle == LifecycleActive {
				from.Lifecycle = LifecycleCandidate
				state.snapshot.Revisions[fromKey] = from
			}
			state.active[key] = payload.ToRevisionID
			state.activeDigests[key] = payload.ToDigest
			toKey := assetRevisionKey(payload.AssetKind, payload.DefinitionID, payload.ToRevisionID)
			if to, found := state.snapshot.Revisions[toKey]; found {
				to.Lifecycle = LifecycleActive
				state.snapshot.Revisions[toKey] = to
			}
			definition := state.snapshot.Definitions[key]
			definition.ActiveRevisionID = payload.ToRevisionID
			definition.Lifecycle = LifecycleActive
			state.snapshot.Definitions[key] = definition
		case "EvolutionAssetEvaluationRecorded":
			var payload evaluationPayload
			if err := decodeStrict(event.PayloadJSON, &payload); err != nil {
				return replayState{}, err
			}
			if !validReplayEvaluation(payload) {
				return replayState{}, ErrInvalidInput
			}
			var usageValue, costValue *int64
			if payload.UsageObserved {
				value := payload.UsageMicrounits
				usageValue = &value
			}
			if payload.CostObserved {
				value := payload.CostMicrounits
				costValue = &value
			}
			state.snapshot.Evaluations[payload.EvaluationID] = EvaluationRecord{
				EvaluationID: payload.EvaluationID, CandidateID: payload.CandidateID,
				FixtureKind: payload.FixtureKind, FixtureDigest: payload.FixtureDigest,
				BaselineRevisionID: payload.BaselineRevisionID, BaselineDigest: payload.BaselineDigest,
				CandidateRevisionID: payload.CandidateRevisionID, CandidateDigest: payload.CandidateDigest,
				QualityResult: payload.QualityResult, FailureCount: payload.FailureCount, CaseCount: payload.CaseCount,
				UsageObserved: payload.UsageObserved, UsageValue: usageValue,
				CostObserved: payload.CostObserved, CostValue: costValue,
				CompatibilityResult: payload.CompatibilityResult, ApplicableScope: payload.ApplicableScope,
				RegressionResult: payload.RegressionResult, SecurityResult: payload.SecurityResult,
				EvidenceID: payload.EvidenceID, EvidenceDigest: payload.EvidenceDigest,
			}
			candidate := state.snapshot.Candidates[payload.CandidateID]
			candidate.RequiredEvaluationIDs = uniqueSorted(append(candidate.RequiredEvaluationIDs, payload.EvaluationID))
			state.snapshot.Candidates[payload.CandidateID] = candidate
		case "EvolutionAssetBindingSetCommitted":
			var payload bindingPayload
			if err := decodeStrict(event.PayloadJSON, &payload); err != nil {
				return replayState{}, err
			}
			subject := SubjectIdentity{SubjectKind: payload.SubjectKind, SubjectID: payload.SubjectID, SubjectVersion: payload.SubjectVersion, SubjectDigest: payload.SubjectDigest, Scope: payload.SubjectScope, ProjectID: payload.SubjectProjectID, GenerationID: payload.SubjectGenerationID}
			canonicalDigest, digestErr := CanonicalSubjectIdentityDigest(subject)
			setDigest, setErr := CanonicalAssetRevisionSetDigest(payload.Bindings)
			if digestErr != nil || setErr != nil || canonicalDigest != payload.SubjectIdentityDigest || setDigest != payload.AssetRevisionSetDigest {
				return replayState{}, ErrInvalidInput
			}
			state.snapshot.Bindings[payload.SubjectIdentityDigest] = EvolutionAssetBindingRecord{
				SchemaVersion: 1, SubjectKind: subject.SubjectKind, SubjectID: subject.SubjectID,
				SubjectVersion: subject.SubjectVersion, SubjectDigest: subject.SubjectDigest,
				SubjectScope: subject.Scope, SubjectProjectID: subject.ProjectID,
				SubjectGenerationID: subject.GenerationID, SubjectIdentityDigest: payload.SubjectIdentityDigest,
				Bindings:               append([]ExactAssetRevisionBinding(nil), payload.Bindings...),
				AssetRevisionSetDigest: payload.AssetRevisionSetDigest, BindingRevision: payload.BindingRevision,
				LastEventID: event.ID, LastJourneyID: event.CorrelationID,
			}
		case "EvolutionAssetImportProposed":
			var payload importProposedPayload
			if err := decodeStrict(event.PayloadJSON, &payload); err != nil {
				return replayState{}, err
			}
			if !validAssetKind(payload.AssetKind) || !validID(payload.CandidateID) ||
				!validID(payload.DefinitionID) || !validID(payload.RevisionID) ||
				!validDigest(payload.ExternalSourceDigest) ||
				!validDigest(payload.ProvenanceDigest) || !validRisk(payload.Risk) {
				return replayState{}, ErrInvalidInput
			}
		case "EvolutionTemplateInstantiated":
			var payload templateInstantiatedPayload
			if err := decodeStrict(event.PayloadJSON, &payload); err != nil {
				return replayState{}, err
			}
			if !validAssetKind(payload.AssetKind) || payload.AssetKind == AssetKindSkill ||
				!validID(payload.DefinitionID) || !validID(payload.RevisionID) ||
				!validDigest(payload.RevisionDigest) ||
				!validDigest(payload.ParameterDigest) ||
				!validDigest(payload.OutputDigest) ||
				!validID(payload.OutputCandidateID) ||
				payload.TemplateOutput != templateOutputForKind(payload.AssetKind) {
				return replayState{}, ErrInvalidInput
			}
			if revision, found := state.snapshot.Revisions[assetRevisionKey(payload.AssetKind, payload.DefinitionID, payload.RevisionID)]; !found || revision.ArtifactDigest != payload.RevisionDigest {
				return replayState{}, ErrInvalidInput
			}
		case "EvolutionRunPromotionProposed":
			var payload runPromotionPayload
			if err := decodeStrict(event.PayloadJSON, &payload); err != nil {
				return replayState{}, err
			}
			if !validAssetKind(payload.AssetKind) || !validID(payload.CandidateID) ||
				!validID(payload.DefinitionID) || !validID(payload.RevisionID) ||
				!validID(payload.SourceRunID) || payload.SourceRunGeneration < 1 ||
				!validDigest(payload.SourceRunDigest) ||
				!validDigest(payload.RedactedSummaryDigest) ||
				len(payload.SourceEvidenceIDs) != len(payload.SourceEvidenceDigests) ||
				!validDigestList(payload.SourceEvidenceDigests) {
				return replayState{}, ErrInvalidInput
			}
		case "RuntimeSkillMaterializationPublished":
			var payload materializationPublishedPayload
			if err := decodeStrict(event.PayloadJSON, &payload); err != nil ||
				!validMaterializationFact(payload) {
				return replayState{}, ErrInvalidInput
			}
			key := materializationKey(payload.RunID, payload.AttemptNumber, payload.Generation)
			if _, exists := state.snapshot.Materializations[key]; exists {
				return replayState{}, ErrConflict
			}
			state.snapshot.Materializations[key] = RuntimeSkillMaterializationRecord{
				SchemaVersion: 1, TeamExecutionID: payload.TeamExecutionID, LogicalNodeID: payload.LogicalNodeID,
				RunID: payload.RunID, AttemptNumber: payload.AttemptNumber, Generation: payload.Generation,
				RuntimeInstanceID: payload.RuntimeInstanceID, RuntimeIdentityDigest: payload.RuntimeIdentityDigest,
				Capability: payload.Capability, AssetRevisionBindings: append([]ExactAssetRevisionBinding(nil), payload.AssetRevisionBindings...),
				AssetRevisionSetDigest: payload.AssetRevisionSetDigest, ManifestArtifactDigest: payload.ManifestArtifactDigest,
				MaterializationRootDigest: payload.MaterializationRootDigest, JourneyID: event.CorrelationID,
				LastEventID: event.ID,
			}
		case "RuntimeSkillMaterializationCleaned":
			var payload materializationCleanedPayload
			if err := decodeStrict(event.PayloadJSON, &payload); err != nil ||
				!validID(payload.RunID) || payload.AttemptNumber < 1 ||
				payload.Generation < 1 ||
				!validDigest(payload.ManifestArtifactDigest) ||
				!validDigest(payload.MaterializationRootDigest) ||
				payload.CleanupResult != "removed" {
				return replayState{}, ErrInvalidInput
			}
			key := materializationKey(payload.RunID, payload.AttemptNumber, payload.Generation)
			record, exists := state.snapshot.Materializations[key]
			if !exists || record.ManifestArtifactDigest != payload.ManifestArtifactDigest || record.MaterializationRootDigest != payload.MaterializationRootDigest || record.Cleaned {
				return replayState{}, ErrConflict
			}
			record.Cleaned, record.CleanupResult, record.LastEventID = true, payload.CleanupResult, event.ID
			state.snapshot.Materializations[key] = record
		}
	}
	return state, nil
}

type commonPayload struct {
	SchemaVersion int    `json:"schema_version"`
	OperationID   string `json:"operation_id"`
}
type definitionCreatedPayload struct {
	SchemaVersion int       `json:"schema_version"`
	OperationID   string    `json:"operation_id"`
	AssetKind     AssetKind `json:"asset_kind"`
	DefinitionID  string    `json:"definition_id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Scope         string    `json:"subject_scope"`
}
type revisionCreatedPayload struct {
	SchemaVersion                 int         `json:"schema_version"`
	OperationID                   string      `json:"operation_id"`
	AssetKind                     AssetKind   `json:"asset_kind"`
	DefinitionID                  string      `json:"definition_id"`
	RevisionID                    string      `json:"revision_id"`
	ArtifactDigest                string      `json:"artifact_digest"`
	ContentDigest                 string      `json:"content_digest"`
	SourceScope                   SourceScope `json:"source_scope"`
	SourceReferenceDigest         string      `json:"source_reference_digest"`
	ProvenanceDigest              string      `json:"provenance_digest"`
	Dependencies                  []string    `json:"dependencies"`
	CompatibleRuntimeCapabilities []string    `json:"compatible_runtime_capabilities"`
	Risk                          Risk        `json:"risk"`
	Lifecycle                     Lifecycle   `json:"lifecycle"`
}
type candidateCreatedPayload struct {
	SchemaVersion         int         `json:"schema_version"`
	OperationID           string      `json:"operation_id"`
	CandidateID           string      `json:"candidate_id"`
	AssetKind             AssetKind   `json:"asset_kind"`
	DefinitionID          string      `json:"definition_id"`
	RevisionID            string      `json:"revision_id"`
	SourceScope           SourceScope `json:"source_scope"`
	SourceRunID           string      `json:"source_run_id"`
	SourceRunGeneration   int64       `json:"source_run_generation"`
	SourceRunDigest       string      `json:"source_run_digest"`
	SourceEvidenceIDs     []string    `json:"source_evidence_ids"`
	SourceEvidenceDigests []string    `json:"source_evidence_digests"`
	RedactedSummary       string      `json:"redacted_summary"`
	RedactedSummaryDigest string      `json:"redacted_summary_digest"`
	ScopeDifference       string      `json:"scope_difference"`
	ExpectedBenefit       string      `json:"expected_benefit"`
	Risk                  Risk        `json:"risk"`
	RequiredEvaluationIDs []string    `json:"required_evaluation_ids"`
}
type activatedPayload struct {
	SchemaVersion              int       `json:"schema_version"`
	OperationID                string    `json:"operation_id"`
	AssetKind                  AssetKind `json:"asset_kind"`
	CandidateID                string    `json:"candidate_id"`
	DefinitionID               string    `json:"definition_id"`
	RevisionID                 string    `json:"revision_id"`
	RevisionDigest             string    `json:"revision_digest"`
	ExpectedPreviousRevisionID string    `json:"expected_previous_revision_id"`
	EvaluationIDs              []string  `json:"evaluation_ids"`
	DecisionSource             string    `json:"decision_source"`
}
type decisionPayload struct {
	SchemaVersion  int       `json:"schema_version"`
	OperationID    string    `json:"operation_id"`
	CandidateID    string    `json:"candidate_id"`
	AssetKind      AssetKind `json:"asset_kind"`
	DefinitionID   string    `json:"definition_id"`
	RevisionID     string    `json:"revision_id"`
	RevisionDigest string    `json:"revision_digest"`
	ReasonCode     string    `json:"reason_code"`
	DecisionSource string    `json:"decision_source"`
}
type revisionLifecyclePayload struct {
	SchemaVersion     int       `json:"schema_version"`
	OperationID       string    `json:"operation_id"`
	AssetKind         AssetKind `json:"asset_kind"`
	DefinitionID      string    `json:"definition_id"`
	RevisionID        string    `json:"revision_id"`
	RevisionDigest    string    `json:"revision_digest"`
	PreviousLifecycle Lifecycle `json:"previous_lifecycle,omitempty"`
	RestoredLifecycle Lifecycle `json:"restored_lifecycle,omitempty"`
	ReasonCode        string    `json:"reason_code"`
}
type rollbackPayload struct {
	SchemaVersion  int       `json:"schema_version"`
	OperationID    string    `json:"operation_id"`
	AssetKind      AssetKind `json:"asset_kind"`
	DefinitionID   string    `json:"definition_id"`
	FromRevisionID string    `json:"from_revision_id"`
	ToRevisionID   string    `json:"to_revision_id"`
	FromDigest     string    `json:"from_digest"`
	ToDigest       string    `json:"to_digest"`
	EvaluationIDs  []string  `json:"evaluation_ids"`
	ReasonCode     string    `json:"reason_code"`
	DecisionSource string    `json:"decision_source"`
}
type evaluationPayload struct {
	SchemaVersion       int    `json:"schema_version"`
	OperationID         string `json:"operation_id"`
	EvaluationID        string `json:"evaluation_id"`
	CandidateID         string `json:"candidate_id"`
	FixtureKind         string `json:"fixture_kind"`
	FixtureDigest       string `json:"fixture_digest"`
	BaselineRevisionID  string `json:"baseline_revision_id"`
	BaselineDigest      string `json:"baseline_digest"`
	CandidateRevisionID string `json:"candidate_revision_id"`
	CandidateDigest     string `json:"candidate_digest"`
	QualityResult       string `json:"quality_result"`
	FailureCount        int    `json:"failure_count"`
	CaseCount           int    `json:"case_count"`
	UsageObserved       bool   `json:"usage_observed"`
	UsageMicrounits     int64  `json:"usage_microunits"`
	CostObserved        bool   `json:"cost_observed"`
	CostMicrounits      int64  `json:"cost_microunits"`
	CostCurrency        string `json:"cost_currency"`
	CompatibilityResult string `json:"compatibility_result"`
	ApplicableScope     string `json:"applicable_scope"`
	RegressionResult    string `json:"regression_result"`
	SecurityResult      string `json:"security_result"`
	EvidenceID          string `json:"evidence_id"`
	EvidenceDigest      string `json:"evidence_digest"`
}
type bindingPayload struct {
	SchemaVersion          int                         `json:"schema_version"`
	OperationID            string                      `json:"operation_id"`
	SubjectKind            string                      `json:"subject_kind"`
	SubjectID              string                      `json:"subject_id"`
	SubjectVersion         int64                       `json:"subject_version"`
	SubjectDigest          string                      `json:"subject_digest"`
	SubjectScope           string                      `json:"subject_scope"`
	SubjectProjectID       string                      `json:"subject_project_id"`
	SubjectGenerationID    string                      `json:"subject_generation_id"`
	SubjectIdentityDigest  string                      `json:"subject_identity_digest"`
	Bindings               []ExactAssetRevisionBinding `json:"asset_revision_bindings"`
	AssetRevisionSetDigest string                      `json:"asset_revision_set_digest"`
	BindingRevision        int64                       `json:"binding_revision"`
}

type runPromotionPayload struct {
	SchemaVersion         int       `json:"schema_version"`
	OperationID           string    `json:"operation_id"`
	CandidateID           string    `json:"candidate_id"`
	AssetKind             AssetKind `json:"asset_kind"`
	DefinitionID          string    `json:"definition_id"`
	RevisionID            string    `json:"revision_id"`
	SourceRunID           string    `json:"source_run_id"`
	SourceRunGeneration   int64     `json:"source_run_generation"`
	SourceRunDigest       string    `json:"source_run_digest"`
	SourceEvidenceIDs     []string  `json:"source_evidence_ids"`
	SourceEvidenceDigests []string  `json:"source_evidence_digests"`
	RedactedSummaryDigest string    `json:"redacted_summary_digest"`
}

type importProposedPayload struct {
	SchemaVersion        int       `json:"schema_version"`
	OperationID          string    `json:"operation_id"`
	CandidateID          string    `json:"candidate_id"`
	AssetKind            AssetKind `json:"asset_kind"`
	DefinitionID         string    `json:"definition_id"`
	RevisionID           string    `json:"revision_id"`
	ExternalSourceDigest string    `json:"external_source_digest"`
	ProvenanceDigest     string    `json:"provenance_digest"`
	Risk                 Risk      `json:"risk"`
}

type templateInstantiatedPayload struct {
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
}

type materializationPublishedPayload struct {
	SchemaVersion             int                         `json:"schema_version"`
	OperationID               string                      `json:"operation_id"`
	TeamExecutionID           string                      `json:"team_execution_id"`
	LogicalNodeID             string                      `json:"logical_node_id"`
	RunID                     string                      `json:"run_id"`
	AttemptNumber             int                         `json:"attempt_number"`
	Generation                int64                       `json:"generation"`
	RuntimeInstanceID         string                      `json:"runtime_instance_id"`
	RuntimeIdentityDigest     string                      `json:"runtime_identity_digest"`
	Capability                string                      `json:"capability"`
	AssetRevisionBindings     []ExactAssetRevisionBinding `json:"asset_revision_bindings"`
	AssetRevisionSetDigest    string                      `json:"asset_revision_set_digest"`
	ManifestArtifactDigest    string                      `json:"manifest_artifact_digest"`
	MaterializationRootDigest string                      `json:"materialization_root_digest"`
}

type materializationCleanedPayload struct {
	SchemaVersion             int    `json:"schema_version"`
	OperationID               string `json:"operation_id"`
	RunID                     string `json:"run_id"`
	AttemptNumber             int    `json:"attempt_number"`
	Generation                int64  `json:"generation"`
	ManifestArtifactDigest    string `json:"manifest_artifact_digest"`
	MaterializationRootDigest string `json:"materialization_root_digest"`
	CleanupResult             string `json:"cleanup_result"`
}

func materializationKey(runID string, attempt int, generation int64) string {
	return fmt.Sprintf("%s/%d/%d", runID, attempt, generation)
}

func templateOutputForKind(kind AssetKind) TemplateOutput {
	switch kind {
	case AssetKindAgentTemplate:
		return TemplateOutputAgentCandidate
	case AssetKindTeamTemplate:
		return TemplateOutputTeamDraft
	case AssetKindWorkPackageTemplate:
		return TemplateOutputWorkPackageCandidate
	case AssetKindRecoveryStrategyTemplate:
		return TemplateOutputRecoveryStrategyCandidate
	default:
		return ""
	}
}

func validLifecycleValue(value Lifecycle) bool {
	switch value {
	case LifecycleDraft, LifecycleCandidate, LifecycleActive, LifecycleArchived:
		return true
	}
	return false
}

func validReplayLifecycle(payload revisionLifecyclePayload, archive bool) bool {
	if !validAssetKind(payload.AssetKind) || !validID(payload.DefinitionID) ||
		!validID(payload.RevisionID) || !validDigest(payload.RevisionDigest) {
		return false
	}
	if archive {
		if payload.RestoredLifecycle != "" ||
			payload.PreviousLifecycle != LifecycleCandidate &&
				payload.PreviousLifecycle != LifecycleActive {
			return false
		}
		return validAssetReasonCode("archive", payload.ReasonCode)
	}
	if payload.PreviousLifecycle != "" || payload.RestoredLifecycle != LifecycleCandidate {
		return false
	}
	return validAssetReasonCode("restore", payload.ReasonCode)
}

func validReplayEvaluation(payload evaluationPayload) bool {
	if !validID(payload.EvaluationID) || !validID(payload.CandidateID) ||
		payload.FixtureKind != "historical" && payload.FixtureKind != "synthetic" ||
		!validDigest(payload.FixtureDigest) || !validID(payload.BaselineRevisionID) ||
		!validDigest(payload.BaselineDigest) || !validID(payload.CandidateRevisionID) ||
		!validDigest(payload.CandidateDigest) ||
		!validEvaluationResult(payload.QualityResult, "pass", "fail", "partial") ||
		payload.FailureCount < 0 || payload.CaseCount < 1 ||
		payload.FailureCount > payload.CaseCount ||
		!validEvaluationResult(payload.CompatibilityResult, "compatible", "incompatible", "partial") ||
		payload.ApplicableScope == "" || len(payload.ApplicableScope) > 4096 ||
		!validEvaluationResult(payload.RegressionResult, "improved", "equivalent", "regressed", "unknown") ||
		!validEvaluationResult(payload.SecurityResult, "pass", "fail", "partial") ||
		!validID(payload.EvidenceID) || !validDigest(payload.EvidenceDigest) {
		return false
	}
	if payload.UsageObserved && payload.UsageMicrounits < 0 ||
		!payload.UsageObserved && payload.UsageMicrounits != 0 {
		return false
	}
	if payload.CostObserved &&
		(payload.CostMicrounits < 0 || !validThreeLetterCurrency(payload.CostCurrency)) {
		return false
	}
	if !payload.CostObserved &&
		(payload.CostMicrounits != 0 || payload.CostCurrency != "") {
		return false
	}
	return true
}

func validThreeLetterCurrency(value string) bool {
	if len(value) != 3 {
		return false
	}
	for _, character := range value {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}

func validMaterializationFact(payload materializationPublishedPayload) bool {
	if payload.Capability != "loom.skill-materialization.pi.v1" ||
		!validID(payload.TeamExecutionID) || !validID(payload.LogicalNodeID) ||
		!validID(payload.RunID) || payload.AttemptNumber < 1 ||
		payload.Generation < 1 || !validID(payload.RuntimeInstanceID) ||
		!validDigest(payload.RuntimeIdentityDigest) ||
		!validDigest(payload.ManifestArtifactDigest) ||
		!validDigest(payload.MaterializationRootDigest) {
		return false
	}
	if len(payload.AssetRevisionBindings) > 32 ||
		len(payload.AssetRevisionBindings) == 0 ||
		!validDigest(payload.AssetRevisionSetDigest) {
		return false
	}
	digest, err := CanonicalAssetRevisionSetDigest(payload.AssetRevisionBindings)
	return err == nil && digest == payload.AssetRevisionSetDigest
}

func validEvaluationResult(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func decodeStrict(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return ErrInvalidInput
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return ErrInvalidInput
	}
	return nil
}
func hasDuplicateJSONKeys(data []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	duplicate, err := scanJSONValue(decoder)
	return err != nil || duplicate || decoder.Decode(&struct{}{}) != io.EOF
}
func scanJSONValue(decoder *json.Decoder) (bool, error) {
	token, err := decoder.Token()
	if err != nil {
		return false, err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return false, nil
	}
	switch delimiter {
	case '{':
		seen := map[string]struct{}{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return false, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return false, ErrInvalidInput
			}
			if _, exists := seen[key]; exists {
				return true, nil
			}
			seen[key] = struct{}{}
			duplicate, err := scanJSONValue(decoder)
			if err != nil || duplicate {
				return duplicate, err
			}
		}
		_, err = decoder.Token()
		return false, err
	case '[':
		for decoder.More() {
			duplicate, err := scanJSONValue(decoder)
			if err != nil || duplicate {
				return duplicate, err
			}
		}
		_, err = decoder.Token()
		return false, err
	}
	return false, fmt.Errorf("%w: delimiter", ErrInvalidInput)
}
func emptyAssetSnapshot() Snapshot {
	return Snapshot{Definitions: map[string]SkillDefinition{}, Revisions: map[string]SkillRevision{}, Candidates: map[string]EvolutionCandidate{}, Evaluations: map[string]EvaluationRecord{}, Bindings: map[string]EvolutionAssetBindingRecord{}, Materializations: map[string]RuntimeSkillMaterializationRecord{}}
}
func cloneSnapshot(snapshot Snapshot) Snapshot {
	cloned := emptyAssetSnapshot()
	for key, value := range snapshot.Definitions {
		cloned.Definitions[key] = value
	}
	for key, value := range snapshot.Revisions {
		value.Dependencies = nonNilStrings(value.Dependencies)
		value.CompatibleRuntimeCapabilities = nonNilStrings(value.CompatibleRuntimeCapabilities)
		cloned.Revisions[key] = value
	}
	for key, value := range snapshot.Candidates {
		value.SourceEvidenceIDs = nonNilStrings(value.SourceEvidenceIDs)
		value.SourceEvidenceDigests = nonNilStrings(value.SourceEvidenceDigests)
		value.RequiredEvaluationIDs = nonNilStrings(value.RequiredEvaluationIDs)
		cloned.Candidates[key] = value
	}
	for key, value := range snapshot.Evaluations {
		cloned.Evaluations[key] = value
	}
	for key, value := range snapshot.Bindings {
		value.Bindings = append([]ExactAssetRevisionBinding{}, value.Bindings...)
		cloned.Bindings[key] = value
	}
	for key, value := range snapshot.Materializations {
		value.AssetRevisionBindings = append([]ExactAssetRevisionBinding{}, value.AssetRevisionBindings...)
		cloned.Materializations[key] = value
	}
	return cloned
}
func assetDefinitionKey(kind AssetKind, definitionID string) string {
	return string(kind) + "/" + definitionID
}
func assetRevisionKey(kind AssetKind, definitionID, revisionID string) string {
	return assetDefinitionKey(kind, definitionID) + "/" + revisionID
}
