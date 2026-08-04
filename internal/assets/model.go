package assets

import "time"

type AssetKind string

const (
	AssetKindSkill                    AssetKind = "skill"
	AssetKindAgentTemplate            AssetKind = "agent_template"
	AssetKindTeamTemplate             AssetKind = "team_template"
	AssetKindWorkPackageTemplate      AssetKind = "work_package_template"
	AssetKindRecoveryStrategyTemplate AssetKind = "recovery_strategy_template"
)

type Lifecycle string

const (
	LifecycleDraft     Lifecycle = "draft"
	LifecycleCandidate Lifecycle = "candidate"
	LifecycleActive    Lifecycle = "active"
	LifecycleArchived  Lifecycle = "archived"
)

type SourceScope string

const (
	SourceScopeLocal    SourceScope = "local"
	SourceScopeImported SourceScope = "imported"
	SourceScopePromoted SourceScope = "promoted"
)

type Risk string

const (
	RiskLow      Risk = "low"
	RiskMedium   Risk = "medium"
	RiskHigh     Risk = "high"
	RiskCritical Risk = "critical"
)

type TemplateOutput string

const (
	TemplateOutputAgentCandidate            TemplateOutput = "agent_candidate"
	TemplateOutputTeamDraft                 TemplateOutput = "team_draft"
	TemplateOutputWorkPackageCandidate      TemplateOutput = "work_package_candidate"
	TemplateOutputRecoveryStrategyCandidate TemplateOutput = "recovery_strategy_candidate"
)

type ParameterValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type SkillDefinition struct {
	DefinitionID     string    `json:"definition_id"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	Scope            string    `json:"scope"`
	CreatedEventID   string    `json:"created_event_id"`
	LatestRevisionID string    `json:"latest_revision_id"`
	Lifecycle        Lifecycle `json:"lifecycle"`
	ActiveRevisionID string    `json:"active_revision_id"`
	Head             int64     `json:"head"`
}

type SkillRevision struct {
	AssetKind                     AssetKind      `json:"asset_kind"`
	DefinitionID                  string         `json:"definition_id"`
	RevisionID                    string         `json:"revision_id"`
	ArtifactDigest                string         `json:"artifact_digest"`
	ContentDigest                 string         `json:"content_digest"`
	SourceScope                   SourceScope    `json:"source_scope"`
	SourceReferenceDigest         string         `json:"source_reference_digest"`
	ProvenanceDigest              string         `json:"provenance_digest"`
	Dependencies                  []string       `json:"dependencies"`
	CompatibleRuntimeCapabilities []string       `json:"compatible_runtime_capabilities"`
	Risk                          Risk           `json:"risk"`
	Lifecycle                     Lifecycle      `json:"lifecycle"`
	CreatedEventID                string         `json:"created_event_id"`
	TemplateOutput                TemplateOutput `json:"template_output,omitempty"`
	ParameterSchemaDigest         string         `json:"parameter_schema_digest,omitempty"`
	PermissionCeilingDigest       string         `json:"permission_ceiling_digest,omitempty"`
	ScopeCeilingDigest            string         `json:"scope_ceiling_digest,omitempty"`
}

type EvolutionCandidate struct {
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
	Decision              string      `json:"decision"`
	DecisionEventID       string      `json:"decision_event_id"`
}

type EvaluationRecord struct {
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
	UsageValue          *int64 `json:"usage_value,omitempty"`
	CostObserved        bool   `json:"cost_observed"`
	CostValue           *int64 `json:"cost_value,omitempty"`
	CompatibilityResult string `json:"compatibility_result"`
	ApplicableScope     string `json:"applicable_scope"`
	RegressionResult    string `json:"regression_result"`
	SecurityResult      string `json:"security_result"`
	EvidenceID          string `json:"evidence_id"`
	EvidenceDigest      string `json:"evidence_digest"`
}

type ExactAssetRevisionBinding struct {
	AssetKind    AssetKind   `json:"asset_kind"`
	DefinitionID string      `json:"definition_id"`
	RevisionID   string      `json:"revision_id"`
	SHA256Digest string      `json:"sha256_digest"`
	SourceScope  SourceScope `json:"source_scope"`
}

type SubjectIdentity struct {
	SubjectKind    string `json:"subject_kind"`
	SubjectID      string `json:"subject_id"`
	SubjectVersion int64  `json:"subject_version"`
	SubjectDigest  string `json:"subject_digest"`
	Scope          string `json:"subject_scope"`
	ProjectID      string `json:"subject_project_id"`
	GenerationID   string `json:"subject_generation_id"`
}

type EvolutionAssetBindingRecord struct {
	SchemaVersion          int                         `json:"schema_version"`
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
	LastEventID            string                      `json:"last_event_id"`
	LastJourneyID          string                      `json:"last_journey_id"`
}

type TemplateInstantiation struct {
	AssetKind         AssetKind      `json:"asset_kind"`
	TemplateOutput    TemplateOutput `json:"template_output"`
	OutputCandidateID string         `json:"output_candidate_id"`
	OutputDigest      string         `json:"output_digest"`
	CreatedAt         time.Time      `json:"created_at"`
}
