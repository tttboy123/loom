import Foundation

// SF-W1 production Swift queue client surface. All reads and mutations flow
// over the real daemon socket through the production LocalIPCClient framing;
// the models are projections of the Journal-authoritative queue and are
// never an authority themselves.

public struct QueueResourceClaims: Codable, Equatable, Sendable {
    public var runtime: String
    public var slots: Int
    public var model: String

    public init(runtime: String = "", slots: Int = 1, model: String = "") {
        self.runtime = runtime
        self.slots = slots
        self.model = model
    }
}

public struct QueueJob: Codable, Equatable, Sendable {
    public var jobID: String
    public var source: String
    public var dagNodeID: String
    public var dependencies: [String]
    public var status: String
    public var lane: String
    public var ownedPaths: [String]
    public var mutexKeys: [String]
    public var resourceClaims: QueueResourceClaims
    public var attemptCount: Int
    public var maxAttempts: Int
    public var capabilityKind: String
    public var exitConditions: [String]
    public var verificationStrategy: String
    public var integrationStrategy: String
    public var protectedAuthorityPaths: [String]
    public var createdAt: String
    public var correlationID: String

    enum CodingKeys: String, CodingKey {
        case jobID = "job_id"
        case source
        case dagNodeID = "dag_node_id"
        case dependencies, status, lane
        case ownedPaths = "owned_paths"
        case mutexKeys = "mutex_keys"
        case resourceClaims = "resource_claims"
        case attemptCount = "attempt_count"
        case maxAttempts = "max_attempts"
        case capabilityKind = "capability_kind"
        case exitConditions = "exit_conditions"
        case verificationStrategy = "verification_strategy"
        case integrationStrategy = "integration_strategy"
        case protectedAuthorityPaths = "protected_authority_paths"
        case createdAt = "created_at"
        case correlationID = "correlation_id"
    }
}

public struct QueueGapProposal: Codable, Equatable, Sendable {
    public var gapID: String
    public var affectedCapability: String
    public var observedBehavior: String
    public var disposition: String

    enum CodingKeys: String, CodingKey {
        case gapID = "gap_id"
        case affectedCapability = "affected_capability"
        case observedBehavior = "observed_behavior"
        case disposition
    }
}

public struct QueueSuccessorProposal: Codable, Equatable, Sendable {
    public var successorProposalID: String
    public var gapID: String

    enum CodingKeys: String, CodingKey {
        case successorProposalID = "successor_proposal_id"
        case gapID = "gap_id"
    }
}

public struct QueueSnapshot: Codable, Equatable, Sendable {
    public var viewVersion: String
    public var nextCursor: String
    public var jobs: [QueueJob]
    public var gaps: [QueueGapProposal]
    public var successors: [QueueSuccessorProposal]

    enum CodingKeys: String, CodingKey {
        case viewVersion = "view_version"
        case nextCursor = "next_cursor"
        case jobs, gaps, successors
    }
}

public struct QueueJobSubmission: Codable, Equatable, Sendable {
    public var jobID: String
    public var source: String
    public var dagNodeID: String
    public var dependencies: [String]
    public var ownedPaths: [String]
    public var mutexKeys: [String]
    public var resourceClaims: QueueResourceClaims
    public var maxAttempts: Int
    public var capabilityKind: String
    public var exitConditions: [String]
    public var verificationStrategy: String
    public var integrationStrategy: String
    public var protectedAuthorityPaths: [String]
    public var eligibilityAuthority: String

    enum CodingKeys: String, CodingKey {
        case jobID = "job_id"
        case source
        case dagNodeID = "dag_node_id"
        case dependencies
        case ownedPaths = "owned_paths"
        case mutexKeys = "mutex_keys"
        case resourceClaims = "resource_claims"
        case maxAttempts = "max_attempts"
        case capabilityKind = "capability_kind"
        case exitConditions = "exit_conditions"
        case verificationStrategy = "verification_strategy"
        case integrationStrategy = "integration_strategy"
        case protectedAuthorityPaths = "protected_authority_paths"
        case eligibilityAuthority = "eligibility_authority"
    }
}

public struct QueueGapProposalSubmission: Codable, Equatable, Sendable {
    public var sourceType: String
    public var sourceIDs: [String]
    public var sourceDigests: [String]
    public var affectedCapability: String
    public var observedBehavior: String
    public var expectedBehavior: String
    public var userImpact: String
    public var confidence: String
    public var uncertainty: String?
    public var reproducibility: String
    public var privacyClass: String
    public var proposedScope: String
    public var ownedPathClaims: [String]
    public var resourceClaims: QueueResourceClaims
    public var riskClass: String
    public var rollbackIdea: String?
    public var disposition: String?

    enum CodingKeys: String, CodingKey {
        case sourceType = "source_type"
        case sourceIDs = "source_ids"
        case sourceDigests = "source_digests"
        case affectedCapability = "affected_capability"
        case observedBehavior = "observed_behavior"
        case expectedBehavior = "expected_behavior"
        case userImpact = "user_impact"
        case confidence, uncertainty, reproducibility
        case privacyClass = "privacy_classification"
        case proposedScope = "proposed_scope"
        case ownedPathClaims = "owned_path_claims"
        case resourceClaims = "resource_claims"
        case riskClass = "risk_class"
        case rollbackIdea = "rollback_idea"
        case disposition
    }
}

public struct QueueSuccessorCompileRequest: Codable, Equatable, Sendable {
    public var gapID: String
    public var expectedSourceDigests: [String]
    public var submission: QueueJobSubmission

    enum CodingKeys: String, CodingKey {
        case gapID = "gap_id"
        case expectedSourceDigests = "expected_source_digests"
        case submission
    }
}

public struct QueueCommand<Input: Encodable>: Encodable, Sendable {
    public var operationID: String
    public var action: String
    public var journeyID: String
    public var input: Input

    public init(
        operationID: String,
        action: String,
        journeyID: String,
        input: Input
    ) {
        self.operationID = operationID
        self.action = action
        self.journeyID = journeyID
        self.input = input
    }
}

public struct QueueCommandReceipt: Codable, Equatable, Sendable {
    public var operationID: String
    public var action: String
    public var viewVersion: String
    public var eventIDs: [String]
    public var jobID: String?
    public var gapID: String?
    public var successorProposalID: String?
    public var disposition: String?
    public var status: String?
    public var conflictReason: String?

    enum CodingKeys: String, CodingKey {
        case operationID = "operation_id"
        case action
        case viewVersion = "view_version"
        case eventIDs = "event_ids"
        case jobID = "job_id"
        case gapID = "gap_id"
        case successorProposalID = "successor_proposal_id"
        case disposition, status
        case conflictReason = "conflict_reason"
    }
}

private struct QueueSnapshotParams: Encodable {
    let cursor: String
    let limit: Int
}

private struct QueueCommandParams<Input: Encodable>: Encodable {
    let operationID: String
    let action: String
    let input: Input

    enum CodingKeys: String, CodingKey {
        case operationID = "operation_id"
        case action, input
    }
}

private enum QueueWire {
    static func decodeSnapshot(_ data: Data) throws -> QueueSnapshot {
        let snapshot = try JSONDecoder().decode(QueueSnapshot.self, from: data)
        guard !snapshot.viewVersion.isEmpty else {
            throw LocalProductClientError.invalidResponse
        }
        return snapshot
    }

    static func decodeReceipt(_ data: Data) throws -> QueueCommandReceipt {
        let receipt = try JSONDecoder().decode(QueueCommandReceipt.self, from: data)
        guard !receipt.operationID.isEmpty else {
            throw LocalProductClientError.invalidResponse
        }
        return receipt
    }
}

public extension LocalIPCClient {
    func queueSnapshot(
        journeyID: String,
        cursor: String = "",
        limit: Int = 64
    ) async throws -> QueueSnapshot {
        guard Self.validJourneyID(journeyID),
              (1...256).contains(limit),
              cursor.utf8.count <= 64 else {
            throw LocalProductClientError.invalidRequest
        }
        let result = try await callJourney(
            journeyID: journeyID,
            method: "queue_snapshot",
            params: QueueSnapshotParams(cursor: cursor, limit: limit)
        )
        return try QueueWire.decodeSnapshot(result)
    }

    func queueCommand<Input: Encodable>(
        _ command: QueueCommand<Input>
    ) async throws -> QueueCommandReceipt {
        guard Self.validJourneyID(command.journeyID),
              !command.operationID.isEmpty,
              command.action == "create_job" || command.action == "cancel_job"
                || command.action == "gap_observe" || command.action == "successor_compile" else {
            throw LocalProductClientError.invalidRequest
        }
        let result = try await callJourney(
            journeyID: command.journeyID,
            method: "queue_command",
            params: QueueCommandParams(
                operationID: command.operationID,
                action: command.action,
                input: command.input
            )
        )
        return try QueueWire.decodeReceipt(result)
    }
}
