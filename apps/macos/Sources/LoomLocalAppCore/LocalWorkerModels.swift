import Foundation

public struct LocalWorkerAttempt: Codable, Equatable, Sendable {
    public var attemptID: String
    public var jobID: String
    public var generation: Int64
    public var leaseExpiresAt: String
    public var claimCAS: String
    public var status: String
    public var crashSeam: String
    public var crashEffectCardinality: String
    public var failureClass: String
    public var evidenceDigests: [String]
    public var candidateBranch: String
    public var candidateWorktree: String
    public var startedAt: String
    public var finishedAt: String
    public var lane: String

    enum CodingKeys: String, CodingKey {
        case attemptID = "attempt_id"
        case jobID = "job_id"
        case generation
        case leaseExpiresAt = "lease_expires_at"
        case claimCAS = "claim_cas"
        case status
        case crashSeam = "crash_seam"
        case crashEffectCardinality = "crash_effect_cardinality"
        case failureClass = "failure_class"
        case evidenceDigests = "evidence_digests"
        case candidateBranch = "candidate_branch"
        case candidateWorktree = "candidate_worktree"
        case startedAt = "started_at"
        case finishedAt = "finished_at"
        case lane
    }
}

public struct LocalActiveWorker: Codable, Equatable, Sendable {
    public var workerID: String
    public var lane: String
    public var attemptID: String
    public var jobID: String
    public var generation: Int64

    enum CodingKeys: String, CodingKey {
        case workerID = "worker_id"
        case lane
        case attemptID = "attempt_id"
        case jobID = "job_id"
        case generation
    }
}

public struct LocalWorkersSnapshot: Codable, Equatable, Sendable {
    public var viewVersion: String
    public var attempts: [LocalWorkerAttempt]
    public var activeWorkers: [LocalActiveWorker]
    public var repairWaitAge: Int
    public var lanes: [String: Int]

    enum CodingKeys: String, CodingKey {
        case viewVersion = "view_version"
        case attempts
        case activeWorkers = "active_workers"
        case repairWaitAge = "repair_wait_age"
        case lanes
    }
}

public struct LocalWorkersCommandReceipt: Codable, Equatable, Sendable {
    public var operationID: String
    public var action: String
    public var viewVersion: String
    public var eventIDs: [String]
    public var attemptID: String?
    public var jobID: String?
    public var generation: Int64?
    public var lane: String?
    public var disposition: String?

    enum CodingKeys: String, CodingKey {
        case operationID = "operation_id"
        case action
        case viewVersion = "view_version"
        case eventIDs = "event_ids"
        case attemptID = "attempt_id"
        case jobID = "job_id"
        case generation
        case lane
        case disposition
    }
}

public extension LocalIPCClient {
    func workersSnapshot(
        journeyID: String,
        cursor: String = "",
        limit: Int = 64
    ) async throws -> LocalWorkersSnapshot {
        guard Self.validJourneyID(journeyID), (1...256).contains(limit) else {
            throw LocalProductClientError.invalidRequest
        }
        let params: [String: Any] = ["cursor": cursor, "limit": limit]
        let result = try await callJourneyRaw(
            journeyID: journeyID,
            method: "workers_snapshot",
            params: params
        )
        return try JSONDecoder().decode(LocalWorkersSnapshot.self, from: result)
    }

    func workersCommand(
        journeyID: String,
        operationID: String,
        action: String,
        input: [String: Any]
    ) async throws -> LocalWorkersCommandReceipt {
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
            method: "workers_command",
            params: params
        )
        let receipt = try JSONDecoder().decode(LocalWorkersCommandReceipt.self, from: result)
        guard !receipt.operationID.isEmpty else {
            throw LocalProductClientError.invalidResponse
        }
        return receipt
    }
}
