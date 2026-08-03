import Foundation

public struct LocalProductSideTaskEvidenceReference: Codable, Equatable, Sendable {
    public let evidenceID: String
    public let digest: String
    public let kind: String
    enum CodingKeys: String, CodingKey { case evidenceID = "evidence_id"; case digest, kind }
    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(decoder, allowed: ["evidence_id", "digest", "kind"])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        evidenceID = try values.decode(String.self, forKey: .evidenceID)
        digest = try values.decode(String.self, forKey: .digest)
        kind = try values.decode(String.self, forKey: .kind)
        guard !evidenceID.isEmpty, digest.count == 64, kind == "source" || kind == "verifier" else { throw LocalProductWireError.invalidJSON }
    }
}

public struct LocalProductSideTaskArtifactReference: Codable, Equatable, Sendable {
    public let digest: String
    public let kind: String
    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(decoder, allowed: ["digest", "kind"])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        digest = try values.decode(String.self, forKey: .digest); kind = try values.decode(String.self, forKey: .kind)
        guard digest.count == 64, !kind.isEmpty else { throw LocalProductWireError.invalidJSON }
    }
}

public struct LocalProductSideTaskSummary: Codable, Equatable, Sendable {
    public let sideTaskID: String
    public let parentMissionID: String
    public let parentTeamInstanceID: String
    public let parentTaskID: String
    public let parentRunID: String
    public let parentClaimGeneration: Int64
    public let parentExecutionDigest: String
    public let sideExecutionTeamInstanceID: String
    public let purpose: String
    public let mode: String
    public let title: String
    public let status: String
    public let sourceGeneration: Int64
    public let handoffVersion: Int
    public let handoffDigest: String
    public let summaryArtifactDigest: String
    public let whatHappened: String
    public let authorizedFindings: [String]
    public let evidenceReferences: [LocalProductSideTaskEvidenceReference]
    public let artifactReferences: [LocalProductSideTaskArtifactReference]
    public let risk: String
    public let uncertainties: [String]
    public let scopeDelta: [String]
    public let decisionOptions: [String]
    public let recommendedOption: String
    public let recommendationAuthority: String
    public let usageObserved: Bool
    public let usageMicrounits: Int64
    public let usageCurrency: String
    public let decisionDeadline: String
    public let availableDecisions: [String]
    public let effectStatus: String

    enum CodingKeys: String, CodingKey {
        case sideTaskID = "side_task_id"; case parentMissionID = "parent_mission_id"
        case parentTeamInstanceID = "parent_team_instance_id"; case parentTaskID = "parent_task_id"
        case parentRunID = "parent_run_id"; case parentClaimGeneration = "parent_claim_generation"
        case parentExecutionDigest = "parent_execution_digest"
        case sideExecutionTeamInstanceID = "side_execution_team_instance_id"
        case purpose, mode, title, status; case sourceGeneration = "source_generation"
        case handoffVersion = "handoff_version"; case handoffDigest = "handoff_digest"
        case summaryArtifactDigest = "summary_artifact_digest"; case whatHappened = "what_happened"
        case authorizedFindings = "authorized_findings"; case evidenceReferences = "evidence_references"
        case artifactReferences = "artifact_references"; case risk, uncertainties
        case scopeDelta = "scope_delta"; case decisionOptions = "decision_options"
        case recommendedOption = "recommended_option"; case recommendationAuthority = "recommendation_authority"
        case usageObserved = "usage_observed"; case usageMicrounits = "usage_microunits"
        case usageCurrency = "usage_currency"; case decisionDeadline = "decision_deadline"
        case availableDecisions = "available_decisions"; case effectStatus = "effect_status"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(decoder, allowed: Set(CodingKeys.allCases.map(\.rawValue)))
        let v = try decoder.container(keyedBy: CodingKeys.self)
        sideTaskID = try v.decode(String.self, forKey: .sideTaskID); parentMissionID = try v.decode(String.self, forKey: .parentMissionID); parentTeamInstanceID = try v.decode(String.self, forKey: .parentTeamInstanceID); parentTaskID = try v.decode(String.self, forKey: .parentTaskID); parentRunID = try v.decode(String.self, forKey: .parentRunID); parentClaimGeneration = try v.decode(Int64.self, forKey: .parentClaimGeneration); parentExecutionDigest = try v.decode(String.self, forKey: .parentExecutionDigest); sideExecutionTeamInstanceID = try v.decode(String.self, forKey: .sideExecutionTeamInstanceID)
        purpose = try v.decode(String.self, forKey: .purpose); mode = try v.decode(String.self, forKey: .mode); title = try v.decode(String.self, forKey: .title); status = try v.decode(String.self, forKey: .status); sourceGeneration = try v.decode(Int64.self, forKey: .sourceGeneration); handoffVersion = try v.decode(Int.self, forKey: .handoffVersion); handoffDigest = try v.decode(String.self, forKey: .handoffDigest); summaryArtifactDigest = try v.decode(String.self, forKey: .summaryArtifactDigest); whatHappened = try v.decode(String.self, forKey: .whatHappened)
        authorizedFindings = try v.decode([String].self, forKey: .authorizedFindings); evidenceReferences = try v.decode([LocalProductSideTaskEvidenceReference].self, forKey: .evidenceReferences); artifactReferences = try v.decode([LocalProductSideTaskArtifactReference].self, forKey: .artifactReferences); risk = try v.decode(String.self, forKey: .risk); uncertainties = try v.decode([String].self, forKey: .uncertainties); scopeDelta = try v.decode([String].self, forKey: .scopeDelta); decisionOptions = try v.decode([String].self, forKey: .decisionOptions); recommendedOption = try v.decode(String.self, forKey: .recommendedOption); recommendationAuthority = try v.decode(String.self, forKey: .recommendationAuthority); usageObserved = try v.decode(Bool.self, forKey: .usageObserved); usageMicrounits = try v.decode(Int64.self, forKey: .usageMicrounits); usageCurrency = try v.decode(String.self, forKey: .usageCurrency); decisionDeadline = try v.decode(String.self, forKey: .decisionDeadline); availableDecisions = try v.decode([String].self, forKey: .availableDecisions); effectStatus = try v.decode(String.self, forKey: .effectStatus)
        let purposes: Set<String> = ["research", "comparison", "diagnosis", "verification", "read_only_review"]
        let modes: Set<String> = ["report_only", "decision_required", "merge_candidate"]
        let effects: Set<String> = ["none", "pending", "completed", "human_required"]
        guard !sideTaskID.isEmpty, parentExecutionDigest.count == 64,
              purposes.contains(purpose), modes.contains(mode), effects.contains(effectStatus),
              parentClaimGeneration >= 0, sourceGeneration >= 0, handoffVersion >= 0 else { throw LocalProductWireError.invalidJSON }
    }
}

extension LocalProductSideTaskSummary.CodingKeys: CaseIterable {}

public struct LocalProductSideTaskProposalRequest: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let operation: String
    public let parentMissionID: String
    public let parentTeamInstanceID: String
    public let parentTaskID: String
    public let parentRunID: String
    public let parentClaimGeneration: Int64
    public let parentExecutionDigest: String
    public let purpose: String
    public let mode: String
    public let title: String
    public let authorizedRequest: String
    public let permissionScopes: [String]
    public let decisionTimeoutSeconds: Int64
    public let expectedViewVersion: String
    public let correlationID: String

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"; case operation
        case parentMissionID = "parent_mission_id"; case parentTeamInstanceID = "parent_team_instance_id"
        case parentTaskID = "parent_task_id"; case parentRunID = "parent_run_id"
        case parentClaimGeneration = "parent_claim_generation"
        case parentExecutionDigest = "parent_execution_digest"; case purpose, mode, title
        case authorizedRequest = "authorized_request"; case permissionScopes = "permission_scopes"
        case decisionTimeoutSeconds = "decision_timeout_seconds"
        case expectedViewVersion = "expected_view_version"; case correlationID = "correlation_id"
    }

    public init(
        parentMissionID: String, parentTeamInstanceID: String, parentTaskID: String,
        parentRunID: String, parentClaimGeneration: Int64, parentExecutionDigest: String,
        purpose: String, mode: String,
        title: String, authorizedRequest: String, permissionScopes: [String],
        decisionTimeoutSeconds: Int64, expectedViewVersion: String, correlationID: String
    ) {
        schemaVersion = 1; operation = "propose"; self.parentMissionID = parentMissionID
        self.parentTeamInstanceID = parentTeamInstanceID; self.parentTaskID = parentTaskID
        self.parentRunID = parentRunID; self.parentClaimGeneration = parentClaimGeneration
        self.parentExecutionDigest = parentExecutionDigest
        self.purpose = purpose; self.mode = mode; self.title = title
        self.authorizedRequest = authorizedRequest; self.permissionScopes = permissionScopes
        self.decisionTimeoutSeconds = decisionTimeoutSeconds
        self.expectedViewVersion = expectedViewVersion; self.correlationID = correlationID
    }
}

public struct LocalProductSideTaskProposalResult: Decodable, Equatable, Sendable {
    public let schemaVersion: Int; public let status: String; public let proposalDigest: String
    public let viewVersion: String; public let purpose: String; public let mode: String
    public let title: String; public let permissionScopes: [String]
    public let decisionTimeoutSeconds: Int64; public let requiresConfirmation: Bool
    public let policyAvailable: Bool
    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"; case status; case proposalDigest = "proposal_digest"
        case viewVersion = "view_version"; case purpose, mode, title
        case permissionScopes = "permission_scopes"; case decisionTimeoutSeconds = "decision_timeout_seconds"
        case requiresConfirmation = "requires_confirmation"; case policyAvailable = "policy_available"
    }
    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(decoder, allowed: Set(CodingKeys.allCases.map(\.rawValue)))
        let v = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = try v.decode(Int.self, forKey: .schemaVersion); status = try v.decode(String.self, forKey: .status)
        proposalDigest = try v.decode(String.self, forKey: .proposalDigest); viewVersion = try v.decode(String.self, forKey: .viewVersion)
        purpose = try v.decode(String.self, forKey: .purpose); mode = try v.decode(String.self, forKey: .mode); title = try v.decode(String.self, forKey: .title)
        permissionScopes = try v.decode([String].self, forKey: .permissionScopes); decisionTimeoutSeconds = try v.decode(Int64.self, forKey: .decisionTimeoutSeconds)
        requiresConfirmation = try v.decode(Bool.self, forKey: .requiresConfirmation); policyAvailable = try v.decode(Bool.self, forKey: .policyAvailable)
        guard schemaVersion == 1, status == "proposal", proposalDigest.count == 64, viewVersion.count == 64,
              requiresConfirmation, !policyAvailable else { throw LocalProductWireError.invalidJSON }
    }
}
extension LocalProductSideTaskProposalResult.CodingKeys: CaseIterable {}

public struct LocalProductSideTaskCreateRequest: Codable, Equatable, Sendable {
    public let schemaVersion = 1; public let operation = "create"
    public let parentMissionID: String; public let parentTeamInstanceID: String
    public let parentTaskID: String; public let parentRunID: String
    public let parentClaimGeneration: Int64; public let parentExecutionDigest: String
    public let purpose: String; public let mode: String
    public let title: String; public let authorizedRequest: String; public let permissionScopes: [String]
    public let decisionTimeoutSeconds: Int64; public let expectedViewVersion: String
    public let correlationID: String; public let proposalDigest: String; public let confirmed: Bool
    public let policyStreamID = ""; public let policyVersion = 0; public let policyDigest = ""
    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"; case operation
        case parentMissionID = "parent_mission_id"; case parentTeamInstanceID = "parent_team_instance_id"
        case parentTaskID = "parent_task_id"; case parentRunID = "parent_run_id"
        case parentClaimGeneration = "parent_claim_generation"
        case parentExecutionDigest = "parent_execution_digest"; case purpose, mode, title
        case authorizedRequest = "authorized_request"; case permissionScopes = "permission_scopes"
        case decisionTimeoutSeconds = "decision_timeout_seconds"; case expectedViewVersion = "expected_view_version"
        case correlationID = "correlation_id"; case proposalDigest = "proposal_digest"; case confirmed
        case policyStreamID = "policy_stream_id"; case policyVersion = "policy_version"; case policyDigest = "policy_digest"
    }
    public init(proposal: LocalProductSideTaskProposalRequest, proposalDigest: String, confirmed: Bool) {
        parentMissionID = proposal.parentMissionID; parentTeamInstanceID = proposal.parentTeamInstanceID
        parentTaskID = proposal.parentTaskID; parentRunID = proposal.parentRunID
        parentClaimGeneration = proposal.parentClaimGeneration
        parentExecutionDigest = proposal.parentExecutionDigest
        purpose = proposal.purpose; mode = proposal.mode
        title = proposal.title; authorizedRequest = proposal.authorizedRequest
        permissionScopes = proposal.permissionScopes; decisionTimeoutSeconds = proposal.decisionTimeoutSeconds
        expectedViewVersion = proposal.expectedViewVersion; correlationID = proposal.correlationID
        self.proposalDigest = proposalDigest; self.confirmed = confirmed
    }
}

public struct LocalProductSideTaskCreateResult: Decodable, Equatable, Sendable {
    public let schemaVersion: Int; public let sideTaskID: String; public let status: String
    public let viewVersion: String; public let sideExecutionTeamInstanceID: String
    public let proposalDigest: String; public let handoffVersion: Int; public let handoffDigest: String
    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"; case sideTaskID = "side_task_id"; case status
        case viewVersion = "view_version"; case sideExecutionTeamInstanceID = "side_execution_team_instance_id"
        case proposalDigest = "proposal_digest"; case handoffVersion = "handoff_version"; case handoffDigest = "handoff_digest"
    }
    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(decoder, allowed: Set(CodingKeys.allCases.map(\.rawValue)))
        let v = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = try v.decode(Int.self, forKey: .schemaVersion); sideTaskID = try v.decode(String.self, forKey: .sideTaskID)
        status = try v.decode(String.self, forKey: .status); viewVersion = try v.decode(String.self, forKey: .viewVersion)
        sideExecutionTeamInstanceID = try v.decode(String.self, forKey: .sideExecutionTeamInstanceID)
        proposalDigest = try v.decode(String.self, forKey: .proposalDigest); handoffVersion = try v.decode(Int.self, forKey: .handoffVersion)
        handoffDigest = try v.decode(String.self, forKey: .handoffDigest)
        guard schemaVersion == 1, !sideTaskID.isEmpty, !sideExecutionTeamInstanceID.isEmpty,
              proposalDigest.count == 64, viewVersion.count == 64, handoffVersion >= 0,
              (handoffVersion == 0 ? handoffDigest.isEmpty : handoffDigest.count == 64) else { throw LocalProductWireError.invalidJSON }
    }
}
extension LocalProductSideTaskCreateResult.CodingKeys: CaseIterable {}

public struct LocalProductSideTaskReadRequest: Codable, Equatable, Sendable {
    public let schemaVersion = 1; public let operation = "read"; public let sideTaskID: String
    public let expectedViewVersion: String; public let correlationID: String
    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"; case operation; case sideTaskID = "side_task_id"
        case expectedViewVersion = "expected_view_version"; case correlationID = "correlation_id"
    }
    public init(sideTaskID: String, expectedViewVersion: String, correlationID: String) {
        self.sideTaskID = sideTaskID; self.expectedViewVersion = expectedViewVersion; self.correlationID = correlationID
    }
}

public struct LocalProductSideTaskReadResult: Equatable, Sendable {
    public let schemaVersion: Int
    public let summary: LocalProductSideTaskSummary
    public let viewVersion: String
}

public struct LocalProductSideTaskDecisionRequest: Codable, Equatable, Sendable {
    public let schemaVersion = 1; public let operation = "decide"
    public let sideTaskID: String; public let parentMissionID: String; public let parentTeamInstanceID: String
    public let parentTaskID: String; public let parentRunID: String; public let parentLogicalNodeID: String
    public let parentAttemptNumber: Int; public let parentClaimGeneration: Int64; public let parentExecutionDigest: String
    public let sideTaskGeneration: Int64; public let handoffVersion: Int; public let handoffDigest: String
    public let decision: String; public let effectDigest: String; public let expectedViewVersion: String; public let correlationID: String
    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"; case operation; case sideTaskID = "side_task_id"
        case parentMissionID = "parent_mission_id"; case parentTeamInstanceID = "parent_team_instance_id"
        case parentTaskID = "parent_task_id"; case parentRunID = "parent_run_id"
        case parentLogicalNodeID = "parent_logical_node_id"; case parentAttemptNumber = "parent_attempt_number"
        case parentClaimGeneration = "parent_claim_generation"; case parentExecutionDigest = "parent_execution_digest"
        case sideTaskGeneration = "side_task_generation"; case handoffVersion = "handoff_version"
        case handoffDigest = "handoff_digest"; case decision; case effectDigest = "effect_digest"
        case expectedViewVersion = "expected_view_version"; case correlationID = "correlation_id"
    }
    public init(
        sideTask: LocalProductSideTaskSummary, parentLogicalNodeID: String,
        parentAttemptNumber: Int, parentExecutionDigest: String, decision: String,
        effectDigest: String, expectedViewVersion: String, correlationID: String
    ) {
        sideTaskID = sideTask.sideTaskID; parentMissionID = sideTask.parentMissionID
        parentTeamInstanceID = sideTask.parentTeamInstanceID; parentTaskID = sideTask.parentTaskID
        parentRunID = sideTask.parentRunID; self.parentLogicalNodeID = parentLogicalNodeID
        self.parentAttemptNumber = parentAttemptNumber; parentClaimGeneration = sideTask.parentClaimGeneration
        self.parentExecutionDigest = parentExecutionDigest; sideTaskGeneration = sideTask.sourceGeneration
        handoffVersion = sideTask.handoffVersion; handoffDigest = sideTask.handoffDigest
        self.decision = decision; self.effectDigest = effectDigest
        self.expectedViewVersion = expectedViewVersion; self.correlationID = correlationID
    }
}

public struct LocalProductSideTaskDecisionResult: Decodable, Equatable, Sendable {
    public let schemaVersion: Int; public let sideTaskID: String; public let decision: String
    public let status: String; public let effectStatus: String; public let contextPacketDigest: String
    public let continuationExecutionTeamInstanceID: String; public let viewVersion: String
    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"; case sideTaskID = "side_task_id"; case decision, status
        case effectStatus = "effect_status"; case contextPacketDigest = "context_packet_digest"
        case continuationExecutionTeamInstanceID = "continuation_execution_team_instance_id"
        case viewVersion = "view_version"
    }
    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(decoder, allowed: Set(CodingKeys.allCases.map(\.rawValue)))
        let v = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = try v.decode(Int.self, forKey: .schemaVersion); sideTaskID = try v.decode(String.self, forKey: .sideTaskID)
        decision = try v.decode(String.self, forKey: .decision); status = try v.decode(String.self, forKey: .status)
        effectStatus = try v.decode(String.self, forKey: .effectStatus); contextPacketDigest = try v.decode(String.self, forKey: .contextPacketDigest)
        continuationExecutionTeamInstanceID = try v.decode(String.self, forKey: .continuationExecutionTeamInstanceID)
        viewVersion = try v.decode(String.self, forKey: .viewVersion)
        let decisions: Set<String> = ["absorb", "continue", "request_followup", "pivot", "discard", "archive", "cancel_parent"]
        let effects: Set<String> = ["none", "pending", "completed", "human_required"]
        guard schemaVersion == 1, !sideTaskID.isEmpty, decisions.contains(decision), effects.contains(effectStatus),
              viewVersion.count == 64, contextPacketDigest.isEmpty || contextPacketDigest.count == 64 else { throw LocalProductWireError.invalidJSON }
    }
}
extension LocalProductSideTaskDecisionResult.CodingKeys: CaseIterable {}

public enum LocalProductHandoffWire {
    public static func decodeProposal(_ data: Data) throws -> LocalProductSideTaskProposalResult { try decode(data) }
    public static func decodeCreate(_ data: Data) throws -> LocalProductSideTaskCreateResult { try decode(data) }
    public static func decodeRead(_ data: Data) throws -> LocalProductSideTaskReadResult {
        do {
            try StrictJSONScanner.validate(data)
            guard var object = try JSONSerialization.jsonObject(with: data) as? [String: Any],
                  Set(object.keys) == sideTaskReadKeys,
                  let schemaVersion = object.removeValue(forKey: "schema_version") as? Int,
                  schemaVersion == 1,
                  let viewVersion = object.removeValue(forKey: "view_version") as? String,
                  viewVersion.count == 64 else { throw LocalProductWireError.invalidJSON }
            let summaryData = try JSONSerialization.data(withJSONObject: object, options: [.sortedKeys])
            let summary = try JSONDecoder().decode(LocalProductSideTaskSummary.self, from: summaryData)
            return LocalProductSideTaskReadResult(
                schemaVersion: schemaVersion, summary: summary, viewVersion: viewVersion
            )
        } catch let error as LocalProductWireError { throw error }
        catch { throw LocalProductWireError.invalidJSON }
    }
    public static func decodeDecision(_ data: Data) throws -> LocalProductSideTaskDecisionResult { try decode(data) }
    private static func decode<T: Decodable>(_ data: Data) throws -> T {
        do {
            try StrictJSONScanner.validate(data)
            return try JSONDecoder().decode(T.self, from: data)
        } catch let error as LocalProductWireError { throw error }
        catch { throw LocalProductWireError.invalidJSON }
    }

    private static let sideTaskReadKeys: Set<String> = [
        "schema_version", "side_task_id", "parent_mission_id", "parent_team_instance_id",
        "parent_task_id", "parent_run_id", "parent_claim_generation", "parent_execution_digest",
        "side_execution_team_instance_id", "purpose", "mode", "title", "status",
        "source_generation", "handoff_version", "handoff_digest", "summary_artifact_digest",
        "what_happened", "authorized_findings", "evidence_references", "artifact_references",
        "risk", "uncertainties", "scope_delta", "decision_options", "recommended_option",
        "recommendation_authority", "usage_observed", "usage_microunits", "usage_currency",
        "decision_deadline", "available_decisions", "effect_status", "view_version",
    ]
}
