package assets

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

var eventCodes = map[string]string{
	"EvolutionAssetDefinitionCreated":      "definition_created",
	"EvolutionAssetRevisionCreated":        "revision_created",
	"EvolutionAssetImportProposed":         "import_proposed",
	"EvolutionAssetCandidateCreated":       "candidate_created",
	"EvolutionAssetEvaluationRecorded":     "evaluation_recorded",
	"EvolutionAssetBindingSetCommitted":    "binding_set_committed",
	"EvolutionAssetCandidateActivated":     "candidate_activated",
	"EvolutionAssetCandidateRejected":      "candidate_rejected",
	"EvolutionAssetCandidateRetained":      "candidate_retained",
	"EvolutionAssetRevisionArchived":       "revision_archived",
	"EvolutionAssetRevisionRestored":       "revision_restored",
	"EvolutionAssetActivationRolledBack":   "activation_rolled_back",
	"EvolutionTemplateInstantiated":        "template_instantiated",
	"EvolutionRunPromotionProposed":        "run_promotion_proposed",
	"RuntimeSkillMaterializationPublished": "materialization_published",
	"RuntimeSkillMaterializationCleaned":   "materialization_cleaned",
}

func CanonicalIntentDigest(command Command) (string, error) {
	data, err := CanonicalIntentJSON(command)
	if err != nil {
		return "", err
	}
	return sha256Text(data), nil
}

func CanonicalIntentJSON(command Command) ([]byte, error) {
	if !validID(command.OperationID) || !validAction(command.Action) {
		return nil, ErrInvalidInput
	}
	input, err := canonicalIntentInput(command)
	if err != nil {
		return nil, err
	}
	data, err := marshalCanonical(struct {
		SchemaVersion int    `json:"schema_version"`
		OperationID   string `json:"operation_id"`
		Action        string `json:"action"`
		Input         any    `json:"input"`
	}{1, command.OperationID, command.Action, input})
	if err != nil {
		return nil, ErrInvalidInput
	}
	return data, nil
}

func canonicalIntentInput(command Command) (any, error) {
	switch command.Action {
	case "create_skill":
		return struct {
			DefinitionID                  string   `json:"definition_id"`
			RevisionID                    string   `json:"revision_id"`
			Name                          string   `json:"name"`
			Description                   string   `json:"description"`
			SubjectScope                  string   `json:"subject_scope"`
			SuppliedArtifactDigest        string   `json:"supplied_artifact_digest"`
			SuppliedContentDigest         string   `json:"supplied_content_digest"`
			Dependencies                  []string `json:"dependencies"`
			CompatibleRuntimeCapabilities []string `json:"compatible_runtime_capabilities"`
			Risk                          Risk     `json:"risk"`
		}{command.DefinitionID, command.RevisionID, command.Name, command.Description, command.Scope, command.ArtifactDigest, command.ContentDigest, nonNilStrings(command.Dependencies), nonNilStrings(command.CompatibleCapabilities), command.Risk}, nil
	case "import_skill":
		return struct {
			CandidateID                   string   `json:"candidate_id"`
			DefinitionID                  string   `json:"definition_id"`
			RevisionID                    string   `json:"revision_id"`
			Name                          string   `json:"name"`
			Description                   string   `json:"description"`
			SubjectScope                  string   `json:"subject_scope"`
			SuppliedArtifactDigest        string   `json:"supplied_artifact_digest"`
			SuppliedContentDigest         string   `json:"supplied_content_digest"`
			ExternalSourceDigest          string   `json:"external_source_digest"`
			ProvenanceDigest              string   `json:"provenance_digest"`
			Dependencies                  []string `json:"dependencies"`
			CompatibleRuntimeCapabilities []string `json:"compatible_runtime_capabilities"`
			Risk                          Risk     `json:"risk"`
		}{command.CandidateID, command.DefinitionID, command.RevisionID, command.Name, command.Description, command.Scope, command.ArtifactDigest, command.ContentDigest, command.SourceReferenceDigest, command.ProvenanceDigest, nonNilStrings(command.Dependencies), nonNilStrings(command.CompatibleCapabilities), command.Risk}, nil
	case "create_template":
		return struct {
			AssetKind                     AssetKind      `json:"asset_kind"`
			DefinitionID                  string         `json:"definition_id"`
			RevisionID                    string         `json:"revision_id"`
			Name                          string         `json:"name"`
			Description                   string         `json:"description"`
			SubjectScope                  string         `json:"subject_scope"`
			SuppliedArtifactDigest        string         `json:"supplied_artifact_digest"`
			SuppliedContentDigest         string         `json:"supplied_content_digest"`
			TemplateOutput                TemplateOutput `json:"template_output"`
			ParameterSchemaDigest         string         `json:"parameter_schema_digest"`
			PermissionCeilingDigest       string         `json:"permission_ceiling_digest"`
			ScopeCeilingDigest            string         `json:"scope_ceiling_digest"`
			CompatibleRuntimeCapabilities []string       `json:"compatible_runtime_capabilities"`
			Risk                          Risk           `json:"risk"`
		}{command.AssetKind, command.DefinitionID, command.RevisionID, command.Name, command.Description, command.Scope, command.ArtifactDigest, command.ContentDigest, command.TemplateOutput, command.ParameterSchemaDigest, command.PermissionCeilingDigest, command.ScopeCeilingDigest, nonNilStrings(command.CompatibleCapabilities), command.Risk}, nil
	case "instantiate_template":
		return struct {
			AssetKind       AssetKind        `json:"asset_kind"`
			DefinitionID    string           `json:"definition_id"`
			RevisionID      string           `json:"revision_id"`
			RevisionDigest  string           `json:"revision_digest"`
			ParameterValues []ParameterValue `json:"parameter_values"`
			ParameterDigest string           `json:"parameter_digest"`
		}{command.AssetKind, command.DefinitionID, command.RevisionID, command.ArtifactDigest, nonNilParameterValues(command.ParameterValues), command.ParametersDigest}, nil
	case "promote_run":
		return struct {
			CandidateID           string    `json:"candidate_id"`
			AssetKind             AssetKind `json:"asset_kind"`
			DefinitionID          string    `json:"definition_id"`
			RevisionID            string    `json:"revision_id"`
			SourceRunID           string    `json:"source_run_id"`
			SourceRunGeneration   int64     `json:"source_run_generation"`
			SourceRunDigest       string    `json:"source_run_digest"`
			SourceEvidenceIDs     []string  `json:"source_evidence_ids"`
			SourceEvidenceDigests []string  `json:"source_evidence_digests"`
			RedactedSummary       string    `json:"redacted_summary"`
			RedactedSummaryDigest string    `json:"redacted_summary_digest"`
			ScopeDifference       string    `json:"scope_difference"`
			ExpectedBenefit       string    `json:"expected_benefit"`
			Risk                  Risk      `json:"risk"`
		}{command.CandidateID, command.AssetKind, command.DefinitionID, command.RevisionID, command.SourceRunID, command.SourceRunGeneration, command.SourceRunDigest, nonNilStrings(command.SourceEvidenceIDs), nonNilStrings(command.SourceEvidenceDigests), command.RedactedSummary, sha256Text([]byte(command.RedactedSummary)), command.ScopeDifference, command.ExpectedBenefit, command.Risk}, nil
	case "record_evaluation":
		if command.Evaluation == nil {
			return nil, ErrInvalidInput
		}
		value := command.Evaluation
		return struct {
			EvaluationID        string   `json:"evaluation_id"`
			CandidateID         string   `json:"candidate_id"`
			FixtureKind         string   `json:"fixture_kind"`
			FixtureDigest       string   `json:"fixture_digest"`
			BaselineRevisionID  string   `json:"baseline_revision_id"`
			BaselineDigest      string   `json:"baseline_digest"`
			CandidateRevisionID string   `json:"candidate_revision_id"`
			CandidateDigest     string   `json:"candidate_digest"`
			RequestedCaseIDs    []string `json:"requested_case_ids"`
		}{value.EvaluationID, value.CandidateID, value.FixtureKind, value.FixtureDigest, value.BaselineRevisionID, value.BaselineDigest, value.CandidateRevisionID, value.CandidateDigest, nonNilStrings(command.RequestedCaseIDs)}, nil
	case "set_binding":
		return struct {
			SubjectKind            string                      `json:"subject_kind"`
			SubjectID              string                      `json:"subject_id"`
			SubjectVersion         int64                       `json:"subject_version"`
			SubjectDigest          string                      `json:"subject_digest"`
			SubjectScope           string                      `json:"subject_scope"`
			SubjectProjectID       string                      `json:"subject_project_id"`
			SubjectGenerationID    string                      `json:"subject_generation_id"`
			SubjectIdentityDigest  string                      `json:"subject_identity_digest"`
			AssetRevisionBindings  []ExactAssetRevisionBinding `json:"asset_revision_bindings"`
			AssetRevisionSetDigest string                      `json:"asset_revision_set_digest"`
		}{command.Subject.SubjectKind, command.Subject.SubjectID, command.Subject.SubjectVersion, command.Subject.SubjectDigest, command.Subject.Scope, command.Subject.ProjectID, command.Subject.GenerationID, mustSubjectIdentityDigest(command.Subject), nonNilBindings(command.Bindings), command.AssetRevisionSetDigest}, nil
	case "activate":
		return struct {
			CandidateID                string    `json:"candidate_id"`
			AssetKind                  AssetKind `json:"asset_kind"`
			DefinitionID               string    `json:"definition_id"`
			RevisionID                 string    `json:"revision_id"`
			RevisionDigest             string    `json:"revision_digest"`
			ExpectedPreviousRevisionID string    `json:"expected_previous_revision_id"`
			EvaluationIDs              []string  `json:"evaluation_ids"`
		}{command.CandidateID, command.AssetKind, command.DefinitionID, command.RevisionID, command.ArtifactDigest, command.ExpectedPreviousRevisionID, nonNilStrings(command.RequiredEvaluationIDs)}, nil
	case "reject", "retain":
		return struct {
			CandidateID    string    `json:"candidate_id"`
			AssetKind      AssetKind `json:"asset_kind"`
			DefinitionID   string    `json:"definition_id"`
			RevisionID     string    `json:"revision_id"`
			RevisionDigest string    `json:"revision_digest"`
			ReasonCode     string    `json:"reason_code"`
		}{command.CandidateID, command.AssetKind, command.DefinitionID, command.RevisionID, command.ArtifactDigest, command.ReasonCode}, nil
	case "archive", "restore":
		return struct {
			AssetKind      AssetKind `json:"asset_kind"`
			DefinitionID   string    `json:"definition_id"`
			RevisionID     string    `json:"revision_id"`
			RevisionDigest string    `json:"revision_digest"`
			ReasonCode     string    `json:"reason_code"`
		}{command.AssetKind, command.DefinitionID, command.RevisionID, command.ArtifactDigest, command.ReasonCode}, nil
	case "rollback":
		return struct {
			AssetKind      AssetKind `json:"asset_kind"`
			DefinitionID   string    `json:"definition_id"`
			FromRevisionID string    `json:"from_revision_id"`
			FromDigest     string    `json:"from_digest"`
			ToRevisionID   string    `json:"to_revision_id"`
			ToDigest       string    `json:"to_digest"`
			EvaluationIDs  []string  `json:"evaluation_ids"`
			ReasonCode     string    `json:"reason_code"`
		}{command.AssetKind, command.DefinitionID, command.RevisionID, command.ArtifactDigest, command.TargetRevisionID, command.TargetRevisionDigest, nonNilStrings(command.RequiredEvaluationIDs), command.ReasonCode}, nil
	case "publish_materialization":
		return struct {
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
		}{command.TeamExecutionID, command.LogicalNodeID, command.RunID, command.AttemptNumber, command.Generation, command.RuntimeInstanceID, command.RuntimeIdentityDigest, command.Capability, nonNilBindings(command.Bindings), command.AssetRevisionSetDigest, command.ManifestArtifactDigest, command.MaterializationRootDigest}, nil
	case "clean_materialization":
		return struct {
			RunID                     string `json:"run_id"`
			AttemptNumber             int    `json:"attempt_number"`
			Generation                int64  `json:"generation"`
			ManifestArtifactDigest    string `json:"manifest_artifact_digest"`
			MaterializationRootDigest string `json:"materialization_root_digest"`
			CleanupResult             string `json:"cleanup_result"`
		}{command.RunID, command.AttemptNumber, command.Generation, command.ManifestArtifactDigest, command.MaterializationRootDigest, command.CleanupResult}, nil
	default:
		return nil, ErrInvalidInput
	}
}

func nonNilParameterValues(values []ParameterValue) []ParameterValue {
	if values == nil {
		return []ParameterValue{}
	}
	return append([]ParameterValue{}, values...)
}

func nonNilBindings(values []ExactAssetRevisionBinding) []ExactAssetRevisionBinding {
	if values == nil {
		return []ExactAssetRevisionBinding{}
	}
	return append([]ExactAssetRevisionBinding{}, values...)
}

func mustSubjectIdentityDigest(subject SubjectIdentity) string {
	digest, _ := CanonicalSubjectIdentityDigest(subject)
	return digest
}

func CanonicalAssetRevisionSetDigest(bindings []ExactAssetRevisionBinding) (string, error) {
	data, err := CanonicalAssetRevisionSetJSON(bindings)
	if err != nil {
		return "", err
	}
	return sha256Text(data), nil
}

func CanonicalAssetRevisionSetJSON(bindings []ExactAssetRevisionBinding) ([]byte, error) {
	if len(bindings) > 32 {
		return nil, ErrInvalidInput
	}
	ordered := append([]ExactAssetRevisionBinding(nil), bindings...)
	sort.Slice(ordered, func(i, j int) bool {
		left, right := ordered[i], ordered[j]
		if left.AssetKind != right.AssetKind {
			return left.AssetKind < right.AssetKind
		}
		if left.DefinitionID != right.DefinitionID {
			return left.DefinitionID < right.DefinitionID
		}
		return left.RevisionID < right.RevisionID
	})
	for index, binding := range ordered {
		if !validAssetKind(binding.AssetKind) || !validID(binding.DefinitionID) ||
			!validID(binding.RevisionID) || !validDigest(binding.SHA256Digest) ||
			!validSourceScope(binding.SourceScope) {
			return nil, ErrInvalidInput
		}
		if index > 0 && ordered[index-1].AssetKind == binding.AssetKind &&
			ordered[index-1].DefinitionID == binding.DefinitionID {
			return nil, ErrConflict
		}
	}
	if ordered == nil {
		ordered = []ExactAssetRevisionBinding{}
	}
	return marshalCanonical(ordered)
}

func CanonicalSubjectIdentityDigest(subject SubjectIdentity) (string, error) {
	data, err := CanonicalSubjectIdentityJSON(subject)
	if err != nil {
		return "", err
	}
	return sha256Text(data), nil
}

func CanonicalSubjectIdentityJSON(subject SubjectIdentity) ([]byte, error) {
	if !validSubject(subject) {
		return nil, ErrInvalidInput
	}
	return marshalCanonical(struct {
		SchemaVersion       int    `json:"schema_version"`
		SubjectKind         string `json:"subject_kind"`
		SubjectID           string `json:"subject_id"`
		SubjectVersion      int64  `json:"subject_version"`
		SubjectDigest       string `json:"subject_digest"`
		SubjectScope        string `json:"subject_scope"`
		SubjectProjectID    string `json:"subject_project_id"`
		SubjectGenerationID string `json:"subject_generation_id"`
	}{1, subject.SubjectKind, subject.SubjectID, subject.SubjectVersion,
		subject.SubjectDigest, subject.Scope, subject.ProjectID, subject.GenerationID})
}

func EventIdentity(eventType, streamID, operationID, intentDigest string, index int) (string, string, error) {
	code, ok := eventCodes[eventType]
	if !ok || streamID == "" || !validID(operationID) || !validDigest(intentDigest) || index < 0 {
		return "", "", ErrInvalidInput
	}
	seed := fmt.Sprintf("1\n%s\n%s\n%s\n%s\n%d", eventType, streamID, operationID, intentDigest, index)
	digest := sha256Text([]byte(seed))
	return "p3a-" + code + "-" + digest[:32],
		fmt.Sprintf("p3a/%s/%s/%d", operationID, code, index), nil
}

func marshalCanonical(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte("\n")), nil
}

func sha256Text(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validID(value string) bool {
	if len(value) < 1 || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z' ||
			character >= '0' && character <= '9' || strings.ContainsRune("._:-", character) {
			continue
		}
		return false
	}
	return true
}

func validDigest(value string) bool {
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

func validName(value string) bool {
	return len(value) >= 1 && len(value) <= 128 && !containsUnsafeTextRune(value)
}

func validBoundedText(value string) bool {
	return len(value) <= 4096 && !containsUnsafeTextRune(value)
}

func containsUnsafeTextRune(value string) bool {
	for _, character := range value {
		if character < 0x20 || character == 0x7f || character == 0x1b {
			return true
		}
		// Bidi override/isolation controls.
		if character >= 0x202a && character <= 0x202e ||
			character >= 0x2066 && character <= 0x2069 {
			return true
		}
	}
	return false
}

func validAssetKind(kind AssetKind) bool {
	switch kind {
	case AssetKindSkill, AssetKindAgentTemplate, AssetKindTeamTemplate, AssetKindWorkPackageTemplate, AssetKindRecoveryStrategyTemplate:
		return true
	}
	return false
}
func validSourceScope(scope SourceScope) bool {
	return scope == SourceScopeLocal || scope == SourceScopeImported || scope == SourceScopePromoted
}
func validRisk(risk Risk) bool {
	return risk == RiskLow || risk == RiskMedium || risk == RiskHigh || risk == RiskCritical
}
func validAction(action string) bool {
	switch action {
	case "create_skill", "import_skill", "create_template", "instantiate_template", "promote_run", "record_evaluation", "set_binding", "activate", "reject", "retain", "archive", "restore", "rollback", "publish_materialization", "clean_materialization":
		return true
	}
	return false
}
func validSubject(subject SubjectIdentity) bool {
	if !validID(subject.SubjectID) || subject.SubjectVersion < 1 || !validDigest(subject.SubjectDigest) {
		return false
	}
	switch subject.SubjectKind {
	case "agent_definition":
		switch subject.Scope {
		case "project":
			return validID(subject.ProjectID) && subject.GenerationID == ""
		case "reusable":
			return subject.ProjectID == "" && subject.GenerationID == ""
		case "transient":
			return subject.ProjectID == "" && validID(subject.GenerationID)
		}
	case "team_definition":
		switch subject.Scope {
		case "project":
			return validID(subject.ProjectID) && subject.GenerationID == ""
		case "reusable":
			return subject.ProjectID == "" && subject.GenerationID == ""
		}
	case "work_package":
		return subject.Scope == "builtin" && subject.ProjectID == "" && subject.GenerationID == ""
	}
	return false
}
func nonNilStrings(values []string) []string {
	result := make([]string, len(values))
	copy(result, values)
	return result
}
