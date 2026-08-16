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
    public var recoveryRequiredAt: String?
    public var recoveryCode: String?
    public var recoveryAction: String?
    public var recoveryDecisionID: String?
    public var recoveryDecision: String?
    public var recoveryResolvedAt: String?
    public var recoveryEvidenceID: String?
    public var recoveryObservationDigest: String?
    public var recoveryReplacementAttemptID: String?
    public var recoveryReplacementRunID: String?

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
        errorCode: String? = nil,
        recoveryRequiredAt: String? = nil,
        recoveryCode: String? = nil,
        recoveryAction: String? = nil,
        recoveryDecisionID: String? = nil,
        recoveryDecision: String? = nil,
        recoveryResolvedAt: String? = nil,
        recoveryEvidenceID: String? = nil,
        recoveryObservationDigest: String? = nil,
        recoveryReplacementAttemptID: String? = nil,
        recoveryReplacementRunID: String? = nil
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
        self.recoveryRequiredAt = recoveryRequiredAt
        self.recoveryCode = recoveryCode
        self.recoveryAction = recoveryAction
        self.recoveryDecisionID = recoveryDecisionID
        self.recoveryDecision = recoveryDecision
        self.recoveryResolvedAt = recoveryResolvedAt
        self.recoveryEvidenceID = recoveryEvidenceID
        self.recoveryObservationDigest = recoveryObservationDigest
        self.recoveryReplacementAttemptID = recoveryReplacementAttemptID
        self.recoveryReplacementRunID = recoveryReplacementRunID
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
        case recoveryRequiredAt = "recovery_required_at"
        case recoveryCode = "recovery_code"
        case recoveryAction = "recovery_action"
        case recoveryDecisionID = "recovery_decision_id"
        case recoveryDecision = "recovery_decision"
        case recoveryResolvedAt = "recovery_resolved_at"
        case recoveryEvidenceID = "recovery_evidence_id"
        case recoveryObservationDigest = "recovery_observation_digest"
        case recoveryReplacementAttemptID = "recovery_replacement_attempt_id"
        case recoveryReplacementRunID = "recovery_replacement_run_id"
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
