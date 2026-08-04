import Foundation

public enum EvolutionAssetKind: String, Codable, CaseIterable, Sendable {
    case skill
    case agentTemplate = "agent_template"
    case teamTemplate = "team_template"
    case workPackageTemplate = "work_package_template"
    case recoveryStrategyTemplate = "recovery_strategy_template"
}

public enum EvolutionAssetLifecycle: String, Codable, CaseIterable, Sendable {
    case draft, candidate, active, archived
}

public struct EvolutionAssetDefinition: Decodable, Equatable, Identifiable, Sendable {
    public let definitionID: String
    public let name: String
    public let description: String
    public let scope: String
    public let createdEventID: String
    public let latestRevisionID: String
    public let lifecycle: EvolutionAssetLifecycle
    public let activeRevisionID: String
    public let head: Int64
    public var id: String { definitionID }

    enum CodingKeys: String, CodingKey {
        case definitionID = "definition_id", name, description, scope
        case createdEventID = "created_event_id"
        case latestRevisionID = "latest_revision_id"
        case lifecycle
        case activeRevisionID = "active_revision_id"
        case head
    }

    public init(from decoder: Decoder) throws {
        try rejectEvolutionAssetUnknownKeys(decoder, allowed: Set(CodingKeys.allCases.map(\.rawValue)))
        let values = try decoder.container(keyedBy: CodingKeys.self)
        definitionID = try values.decode(String.self, forKey: .definitionID)
        name = try values.decode(String.self, forKey: .name)
        description = try values.decode(String.self, forKey: .description)
        scope = try values.decode(String.self, forKey: .scope)
        createdEventID = try values.decode(String.self, forKey: .createdEventID)
        latestRevisionID = try values.decode(String.self, forKey: .latestRevisionID)
        lifecycle = try values.decode(EvolutionAssetLifecycle.self, forKey: .lifecycle)
        activeRevisionID = try values.decode(String.self, forKey: .activeRevisionID)
        head = try values.decode(Int64.self, forKey: .head)
    }
}

extension EvolutionAssetDefinition.CodingKeys: CaseIterable {}

public struct EvolutionAssetRevision: Decodable, Equatable, Identifiable, Sendable {
    public let assetKind: EvolutionAssetKind
    public let definitionID: String
    public let revisionID: String
    public let artifactDigest: String
    public let contentDigest: String
    public let sourceScope: String
    public let sourceReferenceDigest: String
    public let provenanceDigest: String
    public let dependencies: [String]
    public let compatibleRuntimeCapabilities: [String]
    public let risk: String
    public let lifecycle: EvolutionAssetLifecycle
    public let createdEventID: String
    public let templateOutput: String?
    public let parameterSchemaDigest: String?
    public let permissionCeilingDigest: String?
    public let scopeCeilingDigest: String?
    public var id: String { "\(definitionID)/\(revisionID)" }

    enum CodingKeys: String, CodingKey, CaseIterable {
        case assetKind = "asset_kind", definitionID = "definition_id"
        case revisionID = "revision_id", artifactDigest = "artifact_digest"
        case contentDigest = "content_digest", sourceScope = "source_scope"
        case sourceReferenceDigest = "source_reference_digest"
        case provenanceDigest = "provenance_digest", dependencies
        case compatibleRuntimeCapabilities = "compatible_runtime_capabilities"
        case risk, lifecycle, createdEventID = "created_event_id"
        case templateOutput = "template_output"
        case parameterSchemaDigest = "parameter_schema_digest"
        case permissionCeilingDigest = "permission_ceiling_digest"
        case scopeCeilingDigest = "scope_ceiling_digest"
    }

    public init(from decoder: Decoder) throws {
        try rejectEvolutionAssetUnknownKeys(decoder, allowed: Set(CodingKeys.allCases.map(\.rawValue)))
        let values = try decoder.container(keyedBy: CodingKeys.self)
        assetKind = try values.decode(EvolutionAssetKind.self, forKey: .assetKind)
        definitionID = try values.decode(String.self, forKey: .definitionID)
        revisionID = try values.decode(String.self, forKey: .revisionID)
        artifactDigest = try values.decode(String.self, forKey: .artifactDigest)
        contentDigest = try values.decode(String.self, forKey: .contentDigest)
        sourceScope = try values.decode(String.self, forKey: .sourceScope)
        sourceReferenceDigest = try values.decode(String.self, forKey: .sourceReferenceDigest)
        provenanceDigest = try values.decode(String.self, forKey: .provenanceDigest)
        dependencies = try values.decode([String].self, forKey: .dependencies)
        compatibleRuntimeCapabilities = try values.decode([String].self, forKey: .compatibleRuntimeCapabilities)
        risk = try values.decode(String.self, forKey: .risk)
        lifecycle = try values.decode(EvolutionAssetLifecycle.self, forKey: .lifecycle)
        createdEventID = try values.decode(String.self, forKey: .createdEventID)
        templateOutput = try values.decodeIfPresent(String.self, forKey: .templateOutput)
        parameterSchemaDigest = try values.decodeIfPresent(String.self, forKey: .parameterSchemaDigest)
        permissionCeilingDigest = try values.decodeIfPresent(String.self, forKey: .permissionCeilingDigest)
        scopeCeilingDigest = try values.decodeIfPresent(String.self, forKey: .scopeCeilingDigest)
    }
}

public struct EvolutionAssetSummary: Decodable, Equatable, Identifiable, Sendable {
    public let definition: EvolutionAssetDefinition
    public let revisions: [EvolutionAssetRevision]
    public var id: String { definition.id }
    enum CodingKeys: String, CodingKey, CaseIterable { case definition, revisions }
    public init(from decoder: Decoder) throws {
        try rejectEvolutionAssetUnknownKeys(decoder, allowed: Set(CodingKeys.allCases.map(\.rawValue)))
        let values = try decoder.container(keyedBy: CodingKeys.self)
        definition = try values.decode(EvolutionAssetDefinition.self, forKey: .definition)
        revisions = try values.decode([EvolutionAssetRevision].self, forKey: .revisions)
    }
}

public struct EvolutionAssetCandidate: Decodable, Equatable, Identifiable, Sendable {
    public let candidateID: String
    public let assetKind: EvolutionAssetKind
    public let definitionID: String
    public let revisionID: String
    public let sourceScope: String
    public let sourceRunID: String
    public let sourceRunGeneration: Int64
    public let sourceRunDigest: String
    public let sourceEvidenceIDs: [String]
    public let sourceEvidenceDigests: [String]
    public let redactedSummary: String
    public let redactedSummaryDigest: String
    public let scopeDifference: String
    public let expectedBenefit: String
    public let risk: String
    public let requiredEvaluationIDs: [String]
    public let decision: String
    public let decisionEventID: String
    public var id: String { candidateID }
    enum CodingKeys: String, CodingKey, CaseIterable {
        case candidateID = "candidate_id", assetKind = "asset_kind"
        case definitionID = "definition_id", revisionID = "revision_id"
        case sourceScope = "source_scope", sourceRunID = "source_run_id"
        case sourceRunGeneration = "source_run_generation"
        case sourceRunDigest = "source_run_digest"
        case sourceEvidenceIDs = "source_evidence_ids"
        case sourceEvidenceDigests = "source_evidence_digests"
        case redactedSummary = "redacted_summary", redactedSummaryDigest = "redacted_summary_digest"
        case scopeDifference = "scope_difference"
        case expectedBenefit = "expected_benefit", risk
        case requiredEvaluationIDs = "required_evaluation_ids"
        case decision, decisionEventID = "decision_event_id"
    }
    public init(from decoder: Decoder) throws {
        try rejectEvolutionAssetUnknownKeys(decoder, allowed: Set(CodingKeys.allCases.map(\.rawValue)))
        let values = try decoder.container(keyedBy: CodingKeys.self)
        candidateID = try values.decode(String.self, forKey: .candidateID)
        assetKind = try values.decode(EvolutionAssetKind.self, forKey: .assetKind)
        definitionID = try values.decode(String.self, forKey: .definitionID)
        revisionID = try values.decode(String.self, forKey: .revisionID)
        sourceScope = try values.decode(String.self, forKey: .sourceScope)
        sourceRunID = try values.decode(String.self, forKey: .sourceRunID)
        sourceRunGeneration = try values.decode(Int64.self, forKey: .sourceRunGeneration)
        sourceRunDigest = try values.decode(String.self, forKey: .sourceRunDigest)
        sourceEvidenceIDs = try values.decode([String].self, forKey: .sourceEvidenceIDs)
        sourceEvidenceDigests = try values.decode([String].self, forKey: .sourceEvidenceDigests)
        redactedSummary = try values.decode(String.self, forKey: .redactedSummary)
        redactedSummaryDigest = try values.decode(String.self, forKey: .redactedSummaryDigest)
        scopeDifference = try values.decode(String.self, forKey: .scopeDifference)
        expectedBenefit = try values.decode(String.self, forKey: .expectedBenefit)
        risk = try values.decode(String.self, forKey: .risk)
        requiredEvaluationIDs = try values.decode([String].self, forKey: .requiredEvaluationIDs)
        decision = try values.decode(String.self, forKey: .decision)
        decisionEventID = try values.decode(String.self, forKey: .decisionEventID)
    }
}

public struct EvolutionAssetEvaluation: Decodable, Equatable, Identifiable, Sendable {
    public let evaluationID: String
    public let candidateID: String
    public let fixtureKind: String
    public let fixtureDigest: String
    public let baselineRevisionID: String
    public let baselineDigest: String
    public let candidateRevisionID: String
    public let candidateDigest: String
    public let qualityResult: String
    public let failureCount: Int
    public let caseCount: Int
    public let usageObserved: Bool
    public let usageValue: Int64?
    public let costObserved: Bool
    public let costValue: Int64?
    public let compatibilityResult: String
    public let applicableScope: String
    public let regressionResult: String
    public let securityResult: String
    public let evidenceID: String
    public let evidenceDigest: String
    public var id: String { evaluationID }
    enum CodingKeys: String, CodingKey, CaseIterable {
        case evaluationID = "evaluation_id", candidateID = "candidate_id"
        case fixtureKind = "fixture_kind", fixtureDigest = "fixture_digest"
        case baselineRevisionID = "baseline_revision_id", baselineDigest = "baseline_digest"
        case candidateRevisionID = "candidate_revision_id", candidateDigest = "candidate_digest"
        case qualityResult = "quality_result", failureCount = "failure_count", caseCount = "case_count"
        case usageObserved = "usage_observed", usageValue = "usage_value"
        case costObserved = "cost_observed", costValue = "cost_value"
        case compatibilityResult = "compatibility_result", applicableScope = "applicable_scope"
        case regressionResult = "regression_result", securityResult = "security_result"
        case evidenceID = "evidence_id", evidenceDigest = "evidence_digest"
    }
    public init(from decoder: Decoder) throws {
        try rejectEvolutionAssetUnknownKeys(decoder, allowed: Set(CodingKeys.allCases.map(\.rawValue)))
        let values = try decoder.container(keyedBy: CodingKeys.self)
        evaluationID = try values.decode(String.self, forKey: .evaluationID)
        candidateID = try values.decode(String.self, forKey: .candidateID)
        fixtureKind = try values.decode(String.self, forKey: .fixtureKind)
        fixtureDigest = try values.decode(String.self, forKey: .fixtureDigest)
        baselineRevisionID = try values.decode(String.self, forKey: .baselineRevisionID)
        baselineDigest = try values.decode(String.self, forKey: .baselineDigest)
        candidateRevisionID = try values.decode(String.self, forKey: .candidateRevisionID)
        candidateDigest = try values.decode(String.self, forKey: .candidateDigest)
        qualityResult = try values.decode(String.self, forKey: .qualityResult)
        failureCount = try values.decode(Int.self, forKey: .failureCount)
        caseCount = try values.decode(Int.self, forKey: .caseCount)
        usageObserved = try values.decode(Bool.self, forKey: .usageObserved)
        usageValue = try values.decodeIfPresent(Int64.self, forKey: .usageValue)
        costObserved = try values.decode(Bool.self, forKey: .costObserved)
        costValue = try values.decodeIfPresent(Int64.self, forKey: .costValue)
        compatibilityResult = try values.decode(String.self, forKey: .compatibilityResult)
        applicableScope = try values.decode(String.self, forKey: .applicableScope)
        regressionResult = try values.decode(String.self, forKey: .regressionResult)
        securityResult = try values.decode(String.self, forKey: .securityResult)
        evidenceID = try values.decode(String.self, forKey: .evidenceID)
        evidenceDigest = try values.decode(String.self, forKey: .evidenceDigest)
        if usageObserved != (usageValue != nil) || costObserved != (costValue != nil) {
            throw LocalProductClientError.invalidResponse
        }
    }
}

public struct EvolutionAssetBindingRecord: Decodable, Equatable, Identifiable, Sendable {
    public let schemaVersion: Int
    public let subjectKind: String
    public let subjectID: String
    public let subjectVersion: Int64
    public let subjectDigest: String
    public let subjectScope: String
    public let subjectProjectID: String
    public let subjectGenerationID: String
    public let subjectIdentityDigest: String
    public let assetRevisionBindings: [EvolutionAssetExactBinding]
    public let assetRevisionSetDigest: String
    public let bindingRevision: Int64
    public let lastEventID: String
    public let lastJourneyID: String
    public var id: String { subjectIdentityDigest }
    enum CodingKeys: String, CodingKey, CaseIterable {
        case schemaVersion = "schema_version", subjectKind = "subject_kind"
        case subjectID = "subject_id", subjectVersion = "subject_version"
        case subjectDigest = "subject_digest", subjectScope = "subject_scope"
        case subjectProjectID = "subject_project_id", subjectGenerationID = "subject_generation_id"
        case subjectIdentityDigest = "subject_identity_digest"
        case assetRevisionBindings = "asset_revision_bindings"
        case assetRevisionSetDigest = "asset_revision_set_digest"
        case bindingRevision = "binding_revision", lastEventID = "last_event_id"
        case lastJourneyID = "last_journey_id"
    }
    public init(from decoder: Decoder) throws {
        try rejectEvolutionAssetUnknownKeys(decoder, allowed: Set(CodingKeys.allCases.map(\.rawValue)))
        let values = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
        subjectKind = try values.decode(String.self, forKey: .subjectKind)
        subjectID = try values.decode(String.self, forKey: .subjectID)
        subjectVersion = try values.decode(Int64.self, forKey: .subjectVersion)
        subjectDigest = try values.decode(String.self, forKey: .subjectDigest)
        subjectScope = try values.decode(String.self, forKey: .subjectScope)
        subjectProjectID = try values.decode(String.self, forKey: .subjectProjectID)
        subjectGenerationID = try values.decode(String.self, forKey: .subjectGenerationID)
        assetRevisionBindings = try values.decode([EvolutionAssetExactBinding].self, forKey: .assetRevisionBindings)
        subjectIdentityDigest = try values.decode(String.self, forKey: .subjectIdentityDigest)
        assetRevisionSetDigest = try values.decode(String.self, forKey: .assetRevisionSetDigest)
        bindingRevision = try values.decode(Int64.self, forKey: .bindingRevision)
        lastEventID = try values.decode(String.self, forKey: .lastEventID)
        lastJourneyID = try values.decode(String.self, forKey: .lastJourneyID)
    }
}

public struct EvolutionAssetSubjectIdentity: Decodable, Equatable, Sendable {
    public let subjectKind: String
    public let subjectID: String
    public let subjectVersion: Int64
    public let subjectDigest: String
    public let subjectScope: String
    public let subjectProjectID: String
    public let subjectGenerationID: String
    enum CodingKeys: String, CodingKey {
        case subjectKind = "subject_kind", subjectID = "subject_id"
        case subjectVersion = "subject_version", subjectDigest = "subject_digest"
        case subjectScope = "subject_scope", subjectProjectID = "subject_project_id"
        case subjectGenerationID = "subject_generation_id"
    }
}

public struct EvolutionAssetBindingSubject: Decodable, Equatable, Identifiable, Sendable {
    public let subjectKind: String
    public let subjectID: String
    public let subjectVersion: Int64
    public let subjectDigest: String
    public let subjectScope: String
    public let subjectProjectID: String
    public let subjectGenerationID: String
    public let subjectIdentityDigest: String
    public var id: String { subjectIdentityDigest }
    enum CodingKeys: String, CodingKey, CaseIterable {
        case subjectKind = "subject_kind", subjectID = "subject_id"
        case subjectVersion = "subject_version", subjectDigest = "subject_digest"
        case subjectScope = "subject_scope", subjectProjectID = "subject_project_id"
        case subjectGenerationID = "subject_generation_id"
        case subjectIdentityDigest = "subject_identity_digest"
    }
    public init(from decoder: Decoder) throws {
        try rejectEvolutionAssetUnknownKeys(decoder, allowed: Set(CodingKeys.allCases.map(\.rawValue)))
        let values = try decoder.container(keyedBy: CodingKeys.self)
        subjectKind = try values.decode(String.self, forKey: .subjectKind)
        subjectID = try values.decode(String.self, forKey: .subjectID)
        subjectVersion = try values.decode(Int64.self, forKey: .subjectVersion)
        subjectDigest = try values.decode(String.self, forKey: .subjectDigest)
        subjectScope = try values.decode(String.self, forKey: .subjectScope)
        subjectProjectID = try values.decode(String.self, forKey: .subjectProjectID)
        subjectGenerationID = try values.decode(String.self, forKey: .subjectGenerationID)
        subjectIdentityDigest = try values.decode(String.self, forKey: .subjectIdentityDigest)
    }
}

public struct EvolutionAssetPromotionSource: Decodable, Equatable, Identifiable, Sendable {
    public let runID: String
    public let runGeneration: Int64
    public let runDigest: String
    public let evidenceIDs: [String]
    public let evidenceDigests: [String]
    public var id: String { "\(runID)/\(runGeneration)" }
    enum CodingKeys: String, CodingKey, CaseIterable {
        case runID = "run_id", runGeneration = "run_generation"
        case runDigest = "run_digest", evidenceIDs = "evidence_ids"
        case evidenceDigests = "evidence_digests"
    }
    public init(from decoder: Decoder) throws {
        try rejectEvolutionAssetUnknownKeys(decoder, allowed: Set(CodingKeys.allCases.map(\.rawValue)))
        let values = try decoder.container(keyedBy: CodingKeys.self)
        runID = try values.decode(String.self, forKey: .runID)
        runGeneration = try values.decode(Int64.self, forKey: .runGeneration)
        runDigest = try values.decode(String.self, forKey: .runDigest)
        evidenceIDs = try values.decode([String].self, forKey: .evidenceIDs)
        evidenceDigests = try values.decode([String].self, forKey: .evidenceDigests)
    }
}

public struct EvolutionAssetExactBinding: Codable, Equatable, Sendable {
    public let assetKind: EvolutionAssetKind
    public let definitionID: String
    public let revisionID: String
    public let sha256Digest: String
    public let sourceScope: String
    enum CodingKeys: String, CodingKey {
        case assetKind = "asset_kind", definitionID = "definition_id"
        case revisionID = "revision_id", sha256Digest = "sha256_digest"
        case sourceScope = "source_scope"
    }
    public init(
        assetKind: EvolutionAssetKind, definitionID: String,
        revisionID: String, sha256Digest: String, sourceScope: String
    ) {
        self.assetKind = assetKind; self.definitionID = definitionID
        self.revisionID = revisionID; self.sha256Digest = sha256Digest
        self.sourceScope = sourceScope
    }
}

public struct EvolutionAssetParameterValue: Codable, Equatable, Sendable {
    public let name: String
    public let value: String
    public init(name: String, value: String) { self.name = name; self.value = value }
}

public struct EvolutionAssetMaterialization: Decodable, Equatable, Identifiable, Sendable {
    public let schemaVersion: Int
    public let teamExecutionID: String
    public let logicalNodeID: String
    public let runID: String
    public let attemptNumber: Int
    public let generation: Int64
    public let runtimeInstanceID: String
    public let runtimeIdentityDigest: String
    public let capability: String
    public let assetRevisionBindings: [EvolutionAssetExactBinding]
    public let assetRevisionSetDigest: String
    public let manifestArtifactDigest: String
    public let materializationRootDigest: String
    public let journeyID: String
    public let cleaned: Bool
    public let cleanupResult: String
    public let lastEventID: String
    public var id: String { "\(runID)/\(attemptNumber)/\(generation)" }
    enum CodingKeys: String, CodingKey, CaseIterable {
        case schemaVersion = "schema_version"
        case teamExecutionID = "team_execution_id", logicalNodeID = "logical_node_id"
        case runID = "run_id", attemptNumber = "attempt_number", generation
        case runtimeInstanceID = "runtime_instance_id"
        case runtimeIdentityDigest = "runtime_identity_digest", capability
        case assetRevisionBindings = "asset_revision_bindings"
        case assetRevisionSetDigest = "asset_revision_set_digest"
        case manifestArtifactDigest = "manifest_artifact_digest"
        case materializationRootDigest = "materialization_root_digest"
        case journeyID = "journey_id", cleaned, cleanupResult = "cleanup_result"
        case lastEventID = "last_event_id"
    }
    public init(from decoder: Decoder) throws {
        try rejectEvolutionAssetUnknownKeys(decoder, allowed: Set(CodingKeys.allCases.map(\.rawValue)))
        let values = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
        teamExecutionID = try values.decode(String.self, forKey: .teamExecutionID)
        logicalNodeID = try values.decode(String.self, forKey: .logicalNodeID)
        runID = try values.decode(String.self, forKey: .runID)
        attemptNumber = try values.decode(Int.self, forKey: .attemptNumber)
        generation = try values.decode(Int64.self, forKey: .generation)
        runtimeInstanceID = try values.decode(String.self, forKey: .runtimeInstanceID)
        runtimeIdentityDigest = try values.decode(String.self, forKey: .runtimeIdentityDigest)
        capability = try values.decode(String.self, forKey: .capability)
        assetRevisionBindings = try values.decode([EvolutionAssetExactBinding].self, forKey: .assetRevisionBindings)
        assetRevisionSetDigest = try values.decode(String.self, forKey: .assetRevisionSetDigest)
        manifestArtifactDigest = try values.decode(String.self, forKey: .manifestArtifactDigest)
        materializationRootDigest = try values.decode(String.self, forKey: .materializationRootDigest)
        journeyID = try values.decode(String.self, forKey: .journeyID)
        cleaned = try values.decode(Bool.self, forKey: .cleaned)
        cleanupResult = try values.decode(String.self, forKey: .cleanupResult)
        lastEventID = try values.decode(String.self, forKey: .lastEventID)
    }
}

public struct EvolutionAssetSnapshot: Decodable, Equatable, Sendable {
    public let viewVersion: String
    public let nextCursor: String
    public let definitions: [EvolutionAssetDefinition]
    public let revisions: [EvolutionAssetRevision]
    public let candidates: [EvolutionAssetCandidate]
    public let evaluations: [EvolutionAssetEvaluation]
    public let bindings: [EvolutionAssetBindingRecord]
    public let materializations: [EvolutionAssetMaterialization]
    public let bindingSubjects: [EvolutionAssetBindingSubject]
    public let promotionSources: [EvolutionAssetPromotionSource]

    public var hasMore: Bool { !nextCursor.isEmpty }
    public var records: [EvolutionAssetSummary] {
        definitions.map { definition in
            EvolutionAssetSummary(
                definition: definition,
                revisions: revisions.filter { $0.definitionID == definition.definitionID }
            )
        }
    }
    enum CodingKeys: String, CodingKey, CaseIterable {
        case viewVersion = "view_version", nextCursor = "next_cursor"
        case definitions, revisions, candidates, evaluations, bindings, materializations
        case bindingSubjects = "binding_subjects"
        case promotionSources = "promotion_sources"
    }
    public init(from decoder: Decoder) throws {
        try rejectEvolutionAssetUnknownKeys(decoder, allowed: Set(CodingKeys.allCases.map(\.rawValue)))
        let values = try decoder.container(keyedBy: CodingKeys.self)
        viewVersion = try values.decode(String.self, forKey: .viewVersion)
        nextCursor = try values.decode(String.self, forKey: .nextCursor)
        definitions = try values.decode([EvolutionAssetDefinition].self, forKey: .definitions)
        revisions = try values.decode([EvolutionAssetRevision].self, forKey: .revisions)
        candidates = try values.decode([EvolutionAssetCandidate].self, forKey: .candidates)
        evaluations = try values.decode([EvolutionAssetEvaluation].self, forKey: .evaluations)
        bindings = try values.decode([EvolutionAssetBindingRecord].self, forKey: .bindings)
        materializations = try values.decode([EvolutionAssetMaterialization].self, forKey: .materializations)
        bindingSubjects = try values.decode([EvolutionAssetBindingSubject].self, forKey: .bindingSubjects)
        promotionSources = try values.decode([EvolutionAssetPromotionSource].self, forKey: .promotionSources)
    }
}

extension EvolutionAssetSummary {
    init(definition: EvolutionAssetDefinition, revisions: [EvolutionAssetRevision]) {
        self.definition = definition
        self.revisions = revisions
    }
}

public struct EvolutionAssetDiff: Decodable, Equatable, Sendable {
    public let definitionID: String
    public let leftRevisionID: String
    public let leftDigest: String
    public let rightRevisionID: String
    public let rightDigest: String
    public let changes: [EvolutionAssetDiffChange]
    enum CodingKeys: String, CodingKey, CaseIterable {
        case definitionID = "definition_id", leftRevisionID = "left_revision_id"
        case leftDigest = "left_digest", rightRevisionID = "right_revision_id"
        case rightDigest = "right_digest", changes
    }
    public init(from decoder: Decoder) throws {
        try rejectEvolutionAssetUnknownKeys(decoder, allowed: Set(CodingKeys.allCases.map(\.rawValue)))
        let values = try decoder.container(keyedBy: CodingKeys.self)
        definitionID = try values.decode(String.self, forKey: .definitionID)
        leftRevisionID = try values.decode(String.self, forKey: .leftRevisionID)
        leftDigest = try values.decode(String.self, forKey: .leftDigest)
        rightRevisionID = try values.decode(String.self, forKey: .rightRevisionID)
        rightDigest = try values.decode(String.self, forKey: .rightDigest)
        changes = try values.decode([EvolutionAssetDiffChange].self, forKey: .changes)
    }
}

public struct EvolutionAssetDiffChange: Decodable, Equatable, Sendable {
    public let kind: String
    public let relativePath: String
    public let leftDigest: String
    public let rightDigest: String
    enum CodingKeys: String, CodingKey, CaseIterable {
        case kind, relativePath = "relative_path", leftDigest = "left_digest", rightDigest = "right_digest"
    }
    public init(from decoder: Decoder) throws {
        try rejectEvolutionAssetUnknownKeys(decoder, allowed: Set(CodingKeys.allCases.map(\.rawValue)))
        let values = try decoder.container(keyedBy: CodingKeys.self)
        kind = try values.decode(String.self, forKey: .kind)
        relativePath = try values.decode(String.self, forKey: .relativePath)
        leftDigest = try values.decode(String.self, forKey: .leftDigest)
        rightDigest = try values.decode(String.self, forKey: .rightDigest)
    }
}

public struct EvolutionAssetStreamHead: Codable, Equatable, Sendable {
    public let streamID: String
    public let sequence: Int64
    public let eventID: String
    enum CodingKeys: String, CodingKey { case streamID = "stream_id", sequence, eventID = "event_id" }
}

public struct EvolutionAssetCommand: Encodable, Equatable, Sendable {
    public let action: String
    public let operationID: String
    public let journeyID: String
    public let expectedViewVersion: String
    public let expectedStreamHeads: [EvolutionAssetStreamHead]
    public let decisionSource: String?
    public let assetKind: EvolutionAssetKind?
    public let definitionID: String?
    public let revisionID: String?
    public let candidateID: String?
    public let name: String?
    public let description: String?
    public let scope: String?
    public let sourcePath: String?
    public let artifactDigest: String?
    public let contentDigest: String?
    public let sourceScope: String?
    public let sourceReferenceDigest: String?
    public let provenanceDigest: String?
    public let risk: String?
    public let reasonCode: String?
    public let expectedPreviousRevisionID: String
    public let targetRevisionID: String?
    public let targetRevisionDigest: String?
    public let fromRevisionID: String?
    public let fromDigest: String?
    public let evaluationIDs: [String]
    public let evaluationID: String?
    public let fixtureKind: String?
    public let fixtureDigest: String?
    public let baselineRevisionID: String?
    public let baselineDigest: String?
    public let candidateRevisionID: String?
    public let candidateDigest: String?
    public let requestedCaseIDs: [String]
    public let subject: EvolutionAssetBindingSubject?
    public let bindings: [EvolutionAssetExactBinding]
    public let assetRevisionSetDigest: String?
    public let sourceRunID: String?
    public let sourceRunGeneration: Int64?
    public let sourceRunDigest: String?
    public let sourceEvidenceIDs: [String]
    public let sourceEvidenceDigests: [String]
    public let redactedSummary: String?
    public let redactedSummaryDigest: String?
    public let scopeDifference: String?
    public let expectedBenefit: String?
    public let templateOutput: String?
    public let parameterSchemaDigest: String?
    public let permissionCeilingDigest: String?
    public let scopeCeilingDigest: String?
    public let parameterValues: [EvolutionAssetParameterValue]
    public let parameterDigest: String?

    public init(
        action: String, operationID: String, journeyID: String,
        expectedViewVersion: String,
        expectedStreamHeads: [EvolutionAssetStreamHead] = [],
        decisionSource: String? = nil, assetKind: EvolutionAssetKind? = nil,
        definitionID: String? = nil, revisionID: String? = nil,
        candidateID: String? = nil, name: String? = nil,
        description: String? = nil, scope: String? = nil, sourcePath: String? = nil,
        artifactDigest: String? = nil, contentDigest: String? = nil,
        sourceScope: String? = nil, sourceReferenceDigest: String? = nil,
        provenanceDigest: String? = nil, risk: String? = nil,
        reasonCode: String? = nil, expectedPreviousRevisionID: String = "",
        targetRevisionID: String? = nil,
        targetRevisionDigest: String? = nil, fromRevisionID: String? = nil,
        fromDigest: String? = nil, evaluationIDs: [String] = [],
        evaluationID: String? = nil, fixtureKind: String? = nil,
        fixtureDigest: String? = nil, baselineRevisionID: String? = nil,
        baselineDigest: String? = nil, candidateRevisionID: String? = nil,
        candidateDigest: String? = nil, requestedCaseIDs: [String] = [],
        subject: EvolutionAssetBindingSubject? = nil,
        bindings: [EvolutionAssetExactBinding] = [],
        assetRevisionSetDigest: String? = nil,
        sourceRunID: String? = nil, sourceRunGeneration: Int64? = nil,
        sourceRunDigest: String? = nil, sourceEvidenceIDs: [String] = [],
        sourceEvidenceDigests: [String] = [], redactedSummary: String? = nil,
        redactedSummaryDigest: String? = nil, scopeDifference: String? = nil,
        expectedBenefit: String? = nil, templateOutput: String? = nil,
        parameterSchemaDigest: String? = nil,
        permissionCeilingDigest: String? = nil,
        scopeCeilingDigest: String? = nil,
        parameterValues: [EvolutionAssetParameterValue] = [],
        parameterDigest: String? = nil
    ) {
        self.action = action; self.operationID = operationID; self.journeyID = journeyID
        self.expectedViewVersion = expectedViewVersion
        self.expectedStreamHeads = expectedStreamHeads; self.decisionSource = decisionSource
        self.assetKind = assetKind; self.definitionID = definitionID; self.revisionID = revisionID
        self.candidateID = candidateID; self.name = name; self.description = description
        self.scope = scope; self.artifactDigest = artifactDigest; self.contentDigest = contentDigest
        self.sourcePath = sourcePath
        self.sourceScope = sourceScope; self.sourceReferenceDigest = sourceReferenceDigest
        self.provenanceDigest = provenanceDigest; self.risk = risk; self.reasonCode = reasonCode
        self.expectedPreviousRevisionID = expectedPreviousRevisionID
        self.targetRevisionID = targetRevisionID; self.targetRevisionDigest = targetRevisionDigest
        self.fromRevisionID = fromRevisionID; self.fromDigest = fromDigest
        self.evaluationIDs = evaluationIDs
        self.evaluationID = evaluationID; self.fixtureKind = fixtureKind
        self.fixtureDigest = fixtureDigest; self.baselineRevisionID = baselineRevisionID
        self.baselineDigest = baselineDigest; self.candidateRevisionID = candidateRevisionID
        self.candidateDigest = candidateDigest; self.requestedCaseIDs = requestedCaseIDs
        self.subject = subject; self.bindings = bindings
        self.assetRevisionSetDigest = assetRevisionSetDigest
        self.sourceRunID = sourceRunID; self.sourceRunGeneration = sourceRunGeneration
        self.sourceRunDigest = sourceRunDigest; self.sourceEvidenceIDs = sourceEvidenceIDs
        self.sourceEvidenceDigests = sourceEvidenceDigests
        self.redactedSummary = redactedSummary
        self.redactedSummaryDigest = redactedSummaryDigest
        self.scopeDifference = scopeDifference; self.expectedBenefit = expectedBenefit
        self.templateOutput = templateOutput
        self.parameterSchemaDigest = parameterSchemaDigest
        self.permissionCeilingDigest = permissionCeilingDigest
        self.scopeCeilingDigest = scopeCeilingDigest
        self.parameterValues = parameterValues; self.parameterDigest = parameterDigest
    }

    enum CodingKeys: String, CodingKey {
        case candidateID = "candidate_id", assetKind = "asset_kind"
        case definitionID = "definition_id", revisionID = "revision_id"
        case name, description, subjectScope = "subject_scope", sourcePath = "source_path"
        case suppliedArtifactDigest = "supplied_artifact_digest"
        case suppliedContentDigest = "supplied_content_digest"
        case externalSourceDigest = "external_source_digest"
        case provenanceDigest = "provenance_digest"
        case dependencies, compatibleRuntimeCapabilities = "compatible_runtime_capabilities"
        case risk, revisionDigest = "revision_digest"
        case expectedPreviousRevisionID = "expected_previous_revision_id"
        case evaluationIDs = "evaluation_ids", reasonCode = "reason_code"
        case fromRevisionID = "from_revision_id", fromDigest = "from_digest"
        case toRevisionID = "to_revision_id", toDigest = "to_digest"
        case evaluationID = "evaluation_id", fixtureKind = "fixture_kind"
        case fixtureDigest = "fixture_digest", baselineRevisionID = "baseline_revision_id"
        case baselineDigest = "baseline_digest", candidateRevisionID = "candidate_revision_id"
        case candidateDigest = "candidate_digest", requestedCaseIDs = "requested_case_ids"
        case subjectKind = "subject_kind", subjectID = "subject_id"
        case subjectVersion = "subject_version", subjectDigest = "subject_digest"
        case subjectProjectID = "subject_project_id", subjectGenerationID = "subject_generation_id"
        case subjectIdentityDigest = "subject_identity_digest"
        case assetRevisionBindings = "asset_revision_bindings"
        case assetRevisionSetDigest = "asset_revision_set_digest"
        case sourceRunID = "source_run_id", sourceRunGeneration = "source_run_generation"
        case sourceRunDigest = "source_run_digest"
        case sourceEvidenceIDs = "source_evidence_ids"
        case sourceEvidenceDigests = "source_evidence_digests"
        case redactedSummary = "redacted_summary"
        case redactedSummaryDigest = "redacted_summary_digest"
        case scopeDifference = "scope_difference", expectedBenefit = "expected_benefit"
        case templateOutput = "template_output"
        case parameterSchemaDigest = "parameter_schema_digest"
        case permissionCeilingDigest = "permission_ceiling_digest"
        case scopeCeilingDigest = "scope_ceiling_digest"
        case parameterValues = "parameter_values", parameterDigest = "parameter_digest"
    }

    public func encode(to encoder: Encoder) throws {
        var values = encoder.container(keyedBy: CodingKeys.self)
        switch action {
        case "create_skill":
            try values.encode(required(definitionID), forKey: .definitionID)
            try values.encode(required(revisionID), forKey: .revisionID)
            try values.encode(required(name), forKey: .name)
            try values.encode(description ?? "", forKey: .description)
            try values.encode(required(scope), forKey: .subjectScope)
            try values.encode(required(sourcePath), forKey: .sourcePath)
            try values.encode(required(artifactDigest), forKey: .suppliedArtifactDigest)
            try values.encode(required(contentDigest), forKey: .suppliedContentDigest)
            try values.encode([String](), forKey: .dependencies)
            try values.encode([String](), forKey: .compatibleRuntimeCapabilities)
            try values.encode(required(risk), forKey: .risk)
        case "import_skill":
            try values.encode(required(candidateID), forKey: .candidateID)
            try values.encode(required(definitionID), forKey: .definitionID)
            try values.encode(required(revisionID), forKey: .revisionID)
            try values.encode(required(name), forKey: .name)
            try values.encode(description ?? "", forKey: .description)
            try values.encode(required(scope), forKey: .subjectScope)
            try values.encode(required(sourcePath), forKey: .sourcePath)
            try values.encode(required(artifactDigest), forKey: .suppliedArtifactDigest)
            try values.encode(required(contentDigest), forKey: .suppliedContentDigest)
            try values.encode(required(sourceReferenceDigest), forKey: .externalSourceDigest)
            try values.encode(required(provenanceDigest), forKey: .provenanceDigest)
            try values.encode([String](), forKey: .dependencies)
            try values.encode([String](), forKey: .compatibleRuntimeCapabilities)
            try values.encode(required(risk), forKey: .risk)
        case "create_template":
            try values.encode(required(assetKind), forKey: .assetKind)
            try values.encode(required(definitionID), forKey: .definitionID)
            try values.encode(required(revisionID), forKey: .revisionID)
            try values.encode(required(name), forKey: .name)
            try values.encode(description ?? "", forKey: .description)
            try values.encode(required(scope), forKey: .subjectScope)
            try values.encode(required(sourcePath), forKey: .sourcePath)
            try values.encode(required(artifactDigest), forKey: .suppliedArtifactDigest)
            try values.encode(required(contentDigest), forKey: .suppliedContentDigest)
            try values.encode(required(templateOutput), forKey: .templateOutput)
            try values.encode(required(parameterSchemaDigest), forKey: .parameterSchemaDigest)
            try values.encode(required(permissionCeilingDigest), forKey: .permissionCeilingDigest)
            try values.encode(required(scopeCeilingDigest), forKey: .scopeCeilingDigest)
            try values.encode([String](), forKey: .compatibleRuntimeCapabilities)
            try values.encode(required(risk), forKey: .risk)
        case "instantiate_template":
            try values.encode(required(assetKind), forKey: .assetKind)
            try values.encode(required(definitionID), forKey: .definitionID)
            try values.encode(required(revisionID), forKey: .revisionID)
            try values.encode(required(artifactDigest), forKey: .revisionDigest)
            try values.encode(parameterValues, forKey: .parameterValues)
            try values.encode(required(parameterDigest), forKey: .parameterDigest)
        case "activate":
            try values.encode(required(candidateID), forKey: .candidateID)
            try values.encode(required(assetKind), forKey: .assetKind)
            try values.encode(required(definitionID), forKey: .definitionID)
            try values.encode(required(revisionID), forKey: .revisionID)
            try values.encode(required(artifactDigest), forKey: .revisionDigest)
            try values.encode(expectedPreviousRevisionID, forKey: .expectedPreviousRevisionID)
            try values.encode(evaluationIDs, forKey: .evaluationIDs)
        case "archive", "restore":
            try values.encode(required(assetKind), forKey: .assetKind)
            try values.encode(required(definitionID), forKey: .definitionID)
            try values.encode(required(revisionID), forKey: .revisionID)
            try values.encode(required(artifactDigest), forKey: .revisionDigest)
            try values.encode(required(reasonCode), forKey: .reasonCode)
        case "reject", "retain":
            try values.encode(required(candidateID), forKey: .candidateID)
            try values.encode(required(assetKind), forKey: .assetKind)
            try values.encode(required(definitionID), forKey: .definitionID)
            try values.encode(required(revisionID), forKey: .revisionID)
            try values.encode(required(artifactDigest), forKey: .revisionDigest)
            try values.encode(required(reasonCode), forKey: .reasonCode)
        case "rollback":
            try values.encode(required(assetKind), forKey: .assetKind)
            try values.encode(required(definitionID), forKey: .definitionID)
            try values.encode(required(fromRevisionID), forKey: .fromRevisionID)
            try values.encode(required(fromDigest), forKey: .fromDigest)
            try values.encode(required(targetRevisionID), forKey: .toRevisionID)
            try values.encode(required(targetRevisionDigest), forKey: .toDigest)
            try values.encode(evaluationIDs, forKey: .evaluationIDs)
            try values.encode(required(reasonCode), forKey: .reasonCode)
        case "record_evaluation":
            try values.encode(required(evaluationID), forKey: .evaluationID)
            try values.encode(required(candidateID), forKey: .candidateID)
            try values.encode(required(fixtureKind), forKey: .fixtureKind)
            try values.encode(required(fixtureDigest), forKey: .fixtureDigest)
            try values.encode(required(baselineRevisionID), forKey: .baselineRevisionID)
            try values.encode(required(baselineDigest), forKey: .baselineDigest)
            try values.encode(required(candidateRevisionID), forKey: .candidateRevisionID)
            try values.encode(required(candidateDigest), forKey: .candidateDigest)
            try values.encode(requestedCaseIDs, forKey: .requestedCaseIDs)
        case "set_binding":
            let exact = try required(subject)
            try values.encode(exact.subjectKind, forKey: .subjectKind)
            try values.encode(exact.subjectID, forKey: .subjectID)
            try values.encode(exact.subjectVersion, forKey: .subjectVersion)
            try values.encode(exact.subjectDigest, forKey: .subjectDigest)
            try values.encode(exact.subjectScope, forKey: .subjectScope)
            try values.encode(exact.subjectProjectID, forKey: .subjectProjectID)
            try values.encode(exact.subjectGenerationID, forKey: .subjectGenerationID)
            try values.encode(exact.subjectIdentityDigest, forKey: .subjectIdentityDigest)
            try values.encode(bindings, forKey: .assetRevisionBindings)
            try values.encode(required(assetRevisionSetDigest), forKey: .assetRevisionSetDigest)
        case "promote_run":
            try values.encode(required(candidateID), forKey: .candidateID)
            try values.encode(required(assetKind), forKey: .assetKind)
            try values.encode(required(definitionID), forKey: .definitionID)
            try values.encode(required(revisionID), forKey: .revisionID)
            try values.encode(required(sourceRunID), forKey: .sourceRunID)
            try values.encode(required(sourceRunGeneration), forKey: .sourceRunGeneration)
            try values.encode(required(sourceRunDigest), forKey: .sourceRunDigest)
            try values.encode(sourceEvidenceIDs, forKey: .sourceEvidenceIDs)
            try values.encode(sourceEvidenceDigests, forKey: .sourceEvidenceDigests)
            try values.encode(required(redactedSummary), forKey: .redactedSummary)
            try values.encode(required(redactedSummaryDigest), forKey: .redactedSummaryDigest)
            try values.encode(required(scopeDifference), forKey: .scopeDifference)
            try values.encode(required(expectedBenefit), forKey: .expectedBenefit)
            try values.encode(required(risk), forKey: .risk)
        default:
            throw LocalProductClientError.invalidRequest
        }
    }

    private func required<T>(_ value: T?) throws -> T {
        guard let value else { throw LocalProductClientError.invalidRequest }
        return value
    }
}

public struct EvolutionAssetCommandReceipt: Decodable, Equatable, Sendable {
    public let operationID: String
    public let action: String
    public let viewVersion: String
    public let eventIDs: [String]
    public let definitionID: String
    public let revisionID: String
    public let candidateID: String
    public let evaluationID: String
    public let subjectKind: String
    public let subjectID: String
    public let assetRevisionSetDigest: String
    public let materializationManifestDigest: String
    public let status: String
    enum CodingKeys: String, CodingKey, CaseIterable {
        case operationID = "operation_id", action, viewVersion = "view_version"
        case eventIDs = "event_ids", definitionID = "definition_id", revisionID = "revision_id"
        case candidateID = "candidate_id", evaluationID = "evaluation_id"
        case subjectKind = "subject_kind", subjectID = "subject_id"
        case assetRevisionSetDigest = "asset_revision_set_digest"
        case materializationManifestDigest = "materialization_manifest_digest", status
    }
    public init(from decoder: Decoder) throws {
        try rejectEvolutionAssetUnknownKeys(decoder, allowed: Set(CodingKeys.allCases.map(\.rawValue)))
        let values = try decoder.container(keyedBy: CodingKeys.self)
        operationID = try values.decode(String.self, forKey: .operationID)
        action = try values.decode(String.self, forKey: .action)
        viewVersion = try values.decode(String.self, forKey: .viewVersion)
        eventIDs = try values.decode([String].self, forKey: .eventIDs)
        definitionID = try values.decode(String.self, forKey: .definitionID)
        revisionID = try values.decode(String.self, forKey: .revisionID)
        candidateID = try values.decode(String.self, forKey: .candidateID)
        evaluationID = try values.decode(String.self, forKey: .evaluationID)
        subjectKind = try values.decode(String.self, forKey: .subjectKind)
        subjectID = try values.decode(String.self, forKey: .subjectID)
        assetRevisionSetDigest = try values.decode(String.self, forKey: .assetRevisionSetDigest)
        materializationManifestDigest = try values.decode(String.self, forKey: .materializationManifestDigest)
        status = try values.decode(String.self, forKey: .status)
    }
}

public protocol LocalProductAssetClientProtocol {
    func evolutionAssetSnapshot(journeyID: String, cursor: String, limit: Int) async throws -> EvolutionAssetSnapshot
    func evolutionAssetDiff(journeyID: String, definitionID: String, left: EvolutionAssetRevision, right: EvolutionAssetRevision) async throws -> EvolutionAssetDiff
    func evolutionAssetCommand(_ command: EvolutionAssetCommand) async throws -> EvolutionAssetCommandReceipt
}

public enum EvolutionAssetWire {
    public static func decodeSnapshot(_ data: Data) throws -> EvolutionAssetSnapshot { try decode(data) }
    public static func decodeDiff(_ data: Data) throws -> EvolutionAssetDiff { try decode(data) }
    public static func decodeReceipt(_ data: Data) throws -> EvolutionAssetCommandReceipt { try decode(data) }
    private static func decode<T: Decodable>(_ data: Data) throws -> T {
        do {
            return try JSONDecoder().decode(T.self, from: data)
        } catch {
            throw LocalProductClientError.invalidResponse
        }
    }
}

private struct EvolutionAssetCodingKey: CodingKey {
    let stringValue: String
    let intValue: Int? = nil
    init?(stringValue: String) { self.stringValue = stringValue }
    init?(intValue: Int) { return nil }
}

private func rejectEvolutionAssetUnknownKeys(_ decoder: Decoder, allowed: Set<String>) throws {
    let values = try decoder.container(keyedBy: EvolutionAssetCodingKey.self)
    if values.allKeys.contains(where: { !allowed.contains($0.stringValue) }) {
        throw LocalProductClientError.invalidResponse
    }
}
