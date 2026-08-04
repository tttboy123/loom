import Foundation

public struct LocalReleaseCandidate: Codable, Equatable, Sendable {
    public var releaseID: String
    public var candidateID: String
    public var targetBranch: String
    public var baseCommit: String
    public var sourceDigest: String
    public var evidenceDigest: String
    public var dependencyDigests: [String]
    public var status: String
    public var adoptedByRunID: String?
    public var createdAt: String
    public var correlationID: String

    enum CodingKeys: String, CodingKey {
        case releaseID = "release_id"
        case candidateID = "candidate_id"
        case targetBranch = "target_branch"
        case baseCommit = "base_commit"
        case sourceDigest = "source_digest"
        case evidenceDigest = "evidence_digest"
        case dependencyDigests = "dependency_digests"
        case status
        case adoptedByRunID = "adopted_by_run_id"
        case createdAt = "created_at"
        case correlationID = "correlation_id"
    }
}

public struct LocalCanaryRun: Codable, Equatable, Sendable {
    public var canaryID: String
    public var runID: String
    public var runtimeInstanceID: String
    public var modelID: String
    public var skillDigest: String
    public var status: String
    public var evidenceDigest: String?
    public var startedAt: String
    public var finishedAt: String?
    public var correlationID: String

    enum CodingKeys: String, CodingKey {
        case canaryID = "canary_id"
        case runID = "run_id"
        case runtimeInstanceID = "runtime_instance_id"
        case modelID = "model_id"
        case skillDigest = "skill_digest"
        case status
        case evidenceDigest = "evidence_digest"
        case startedAt = "started_at"
        case finishedAt = "finished_at"
        case correlationID = "correlation_id"
    }
}

public struct LocalNodeOutputFrame: Codable, Equatable, Sendable {
    public var frameID: String
    public var attemptID: String
    public var generation: Int64
    public var nodeID: String
    public var kind: String
    public var content: String
    public var authorized: Bool
    public var publishedAt: String
    public var correlationID: String

    enum CodingKeys: String, CodingKey {
        case frameID = "frame_id"
        case attemptID = "attempt_id"
        case generation
        case nodeID = "node_id"
        case kind, content, authorized
        case publishedAt = "published_at"
        case correlationID = "correlation_id"
    }
}

public struct LocalAttentionItem: Codable, Equatable, Sendable {
    public var kind: String
    public var sourceID: String
    public var summary: String
    public var lane: String

    enum CodingKeys: String, CodingKey {
        case kind
        case sourceID = "source_id"
        case summary, lane
    }
}

public struct LocalIntegrationSnapshot: Codable, Equatable, Sendable {
    public var viewVersion: String
    public var releases: [LocalReleaseCandidate]
    public var canaries: [LocalCanaryRun]
    public var timeline: [LocalNodeOutputFrame]
    public var attention: [LocalAttentionItem]

    enum CodingKeys: String, CodingKey {
        case viewVersion = "view_version"
        case releases, canaries, timeline, attention
    }
}

public extension LocalIPCClient {
    func integrationSnapshot(journeyID: String) async throws -> LocalIntegrationSnapshot {
        guard Self.validJourneyID(journeyID) else {
            throw LocalProductClientError.invalidRequest
        }
        let params: [String: Any] = [:]
        let result = try await callJourneyRaw(
            journeyID: journeyID,
            method: "integration_snapshot",
            params: params
        )
        return try JSONDecoder().decode(LocalIntegrationSnapshot.self, from: result)
    }

    func integrationCommand(
        journeyID: String,
        operationID: String,
        action: String,
        input: [String: Any]
    ) async throws -> LocalIntegrationCommandReceipt {
        guard Self.validJourneyID(journeyID), !operationID.isEmpty else {
            throw LocalProductClientError.invalidRequest
        }
        let params: [String: Any] = [
            "operation_id": operationID,
            "action": action,
            "input": input,
        ]
        let result = try await callJourneyRaw(
            journeyID: journeyID,
            method: "integration_command",
            params: params
        )
        let receipt = try JSONDecoder().decode(LocalIntegrationCommandReceipt.self, from: result)
        guard !receipt.operationID.isEmpty else {
            throw LocalProductClientError.invalidResponse
        }
        return receipt
    }
}

public struct LocalIntegrationCommandReceipt: Codable, Equatable, Sendable {
    public var operationID: String
    public var action: String
    public var eventIDs: [String]
    public var releaseID: String?
    public var canaryID: String?
    public var disposition: String?

    enum CodingKeys: String, CodingKey {
        case operationID = "operation_id"
        case action
        case eventIDs = "event_ids"
        case releaseID = "release_id"
        case canaryID = "canary_id"
        case disposition
    }
}
