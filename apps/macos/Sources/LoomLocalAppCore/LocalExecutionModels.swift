import Foundation

// B-W1 production Swift execution surface. Read-only: renders the
// Journal-authoritative execution projection; all tool proposals and
// approvals are performed through the TUI / Go service layer.

public struct ExecutionRecord: Codable, Equatable, Sendable {
    public var executionID: String
    public var jobID: String
    public var callDigest: String
    public var tool: String
    public var command: String?
    public var path: String?
    public var generation: Int64
    public var operationID: String
    public var journeyID: String
    public var proposedAt: String
    public var allowedAt: String?
    public var deniedAt: String?
    public var denialReason: String?
    public var exitCode: Int?
    public var outputDigest: String?
    public var changedFilesDigest: String?
    public var evidenceID: String?
    public var durationMS: Int64?
    public var status: String
    public var completedAt: String?
    public var failedAt: String?
    public var failureReason: String?
    public var errorCode: String?

    public init(
        executionID: String = "",
        jobID: String = "",
        callDigest: String = "",
        tool: String = "",
        command: String? = nil,
        path: String? = nil,
        generation: Int64 = 0,
        operationID: String = "",
        journeyID: String = "",
        proposedAt: String = "",
        allowedAt: String? = nil,
        deniedAt: String? = nil,
        denialReason: String? = nil,
        exitCode: Int? = nil,
        outputDigest: String? = nil,
        changedFilesDigest: String? = nil,
        evidenceID: String? = nil,
        durationMS: Int64? = nil,
        status: String = "",
        completedAt: String? = nil,
        failedAt: String? = nil,
        failureReason: String? = nil,
        errorCode: String? = nil
    ) {
        self.executionID = executionID
        self.jobID = jobID
        self.callDigest = callDigest
        self.tool = tool
        self.command = command
        self.path = path
        self.generation = generation
        self.operationID = operationID
        self.journeyID = journeyID
        self.proposedAt = proposedAt
        self.allowedAt = allowedAt
        self.deniedAt = deniedAt
        self.denialReason = denialReason
        self.exitCode = exitCode
        self.outputDigest = outputDigest
        self.changedFilesDigest = changedFilesDigest
        self.evidenceID = evidenceID
        self.durationMS = durationMS
        self.status = status
        self.completedAt = completedAt
        self.failedAt = failedAt
        self.failureReason = failureReason
        self.errorCode = errorCode
    }

    enum CodingKeys: String, CodingKey {
        case executionID = "execution_id"
        case jobID = "job_id"
        case callDigest = "call_digest"
        case tool, command, path, generation
        case operationID = "operation_id"
        case journeyID = "journey_id"
        case proposedAt = "proposed_at"
        case allowedAt = "allowed_at"
        case deniedAt = "denied_at"
        case denialReason = "denial_reason"
        case exitCode = "exit_code"
        case outputDigest = "output_digest"
        case changedFilesDigest = "changed_files_digest"
        case evidenceID = "evidence_id"
        case durationMS = "duration_ms"
        case status
        case completedAt = "completed_at"
        case failedAt = "failed_at"
        case failureReason = "failure_reason"
        case errorCode = "error_code"
    }
}

public struct ExecutionSnapshot: Codable, Equatable, Sendable {
    public var viewVersion: String
    public var records: [ExecutionRecord]

    public init(viewVersion: String = "", records: [ExecutionRecord] = []) {
        self.viewVersion = viewVersion
        self.records = records
    }

    enum CodingKeys: String, CodingKey {
        case viewVersion = "view_version"
        case records
    }
}

public enum ExecutionWire {
    public static func decodeSnapshot(_ data: Data) throws -> ExecutionSnapshot {
        do {
            return try JSONDecoder().decode(ExecutionSnapshot.self, from: data)
        } catch {
            throw LocalProductClientError.invalidResponse
        }
    }
}

public protocol LocalProductExecutionSnapshotClientProtocol {
    func executionSnapshot(journeyID: String) async throws -> ExecutionSnapshot
}

extension LocalIPCClient: LocalProductExecutionSnapshotClientProtocol {}
