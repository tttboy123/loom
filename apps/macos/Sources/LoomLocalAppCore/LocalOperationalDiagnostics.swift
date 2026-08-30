import Darwin
import Foundation

final class LocalOperationalDiagnostics: @unchecked Sendable {
    private struct CredentialRecord: Encodable {
        let schemaVersion = 1
        let occurredAt: String
        let incidentID: String
        let operation: String
        let providerID: String
        let providerAccountID: String?
        let stage: String
        let elapsedMilliseconds: Int64
        let result: String
        let errorCode: String?
        let retryable: Bool

        enum CodingKeys: String, CodingKey {
            case schemaVersion = "schema_version"
            case occurredAt = "occurred_at"
            case incidentID = "incident_id"
            case operation
            case providerID = "provider_id"
            case providerAccountID = "provider_account_id"
            case stage
            case elapsedMilliseconds = "elapsed_ms"
            case result
            case errorCode = "error_code"
            case retryable
        }
    }

    private struct ConversationRecord: Encodable {
        let schemaVersion = 1
        let occurredAt: String
        let incidentID: String
        let operation = "chat_message"
        let threadID: String
        let profileID: String
        let stage: String
        let elapsedMilliseconds: Int64
        let result: String
        let errorCode: String?
		let httpStatus: Int?
		let providerErrorCode: String?
		let retryAfterSeconds: Int64?
        let retryable: Bool

        enum CodingKeys: String, CodingKey {
            case schemaVersion = "schema_version"
            case occurredAt = "occurred_at"
            case incidentID = "incident_id"
            case operation
            case threadID = "thread_id"
            case profileID = "profile_id"
            case stage
            case elapsedMilliseconds = "elapsed_ms"
            case result
            case errorCode = "error_code"
			case httpStatus = "http_status"
			case providerErrorCode = "provider_error_code"
			case retryAfterSeconds = "retry_after_seconds"
            case retryable
        }
    }

    private struct MissionExecutionRecord: Encodable {
        let schemaVersion = 1
        let occurredAt: String
        let incidentID: String
        let operation = "mission_execution"
        let executionOperation: String
        let stage: String
        let elapsedMilliseconds: Int64
        let result: String
        let errorCode: String?
        let retryable: Bool

        enum CodingKeys: String, CodingKey {
            case schemaVersion = "schema_version"
            case occurredAt = "occurred_at"
            case incidentID = "incident_id"
            case operation
            case executionOperation = "execution_operation"
            case stage
            case elapsedMilliseconds = "elapsed_ms"
            case result
            case errorCode = "error_code"
            case retryable
        }
    }

    private struct AgentInputRecord: Encodable {
        let schemaVersion = 1
        let occurredAt: String
        let incidentID: String
        let operation = "agent_input"
        let segmentID: String
        let agentInstanceID: String
        let workItemID: String
        let runID: String
        let claimGeneration: Int64
        let mode: String
        let stage: String
        let elapsedMilliseconds: Int64
        let result: String
        let errorCode: String?
        let retryable: Bool

        enum CodingKeys: String, CodingKey {
            case schemaVersion = "schema_version"
            case occurredAt = "occurred_at"
            case incidentID = "incident_id"
            case operation
            case segmentID = "segment_id"
            case agentInstanceID = "agent_instance_id"
            case workItemID = "work_item_id"
            case runID = "run_id"
            case claimGeneration = "claim_generation"
            case mode, stage
            case elapsedMilliseconds = "elapsed_ms"
            case result
            case errorCode = "error_code"
            case retryable
        }
    }

    private struct AgentRecoveryRecord: Encodable {
        let schemaVersion = 1
        let occurredAt: String
        let incidentID: String
        let operation = "agent_attempt_recovery"
        let recoveryOperation: String
        let stage: String
        let elapsedMilliseconds: Int64
        let result: String
        let errorCode: String?
        let retryable: Bool

        enum CodingKeys: String, CodingKey {
            case schemaVersion = "schema_version"
            case occurredAt = "occurred_at"
            case incidentID = "incident_id"
            case operation
            case recoveryOperation = "recovery_operation"
            case stage
            case elapsedMilliseconds = "elapsed_ms"
            case result
            case errorCode = "error_code"
            case retryable
        }
    }

    private struct ToolRecoveryRecord: Encodable {
        let schemaVersion = 1
        let occurredAt: String
        let incidentID: String
        let operation = "tool_recovery"
        let recoveryOperation: String
        let stage: String
        let elapsedMilliseconds: Int64
        let result: String
        let errorCode: String?
        let retryable: Bool

        enum CodingKeys: String, CodingKey {
            case schemaVersion = "schema_version"
            case occurredAt = "occurred_at"
            case incidentID = "incident_id"
            case operation
            case recoveryOperation = "recovery_operation"
            case stage
            case elapsedMilliseconds = "elapsed_ms"
            case result
            case errorCode = "error_code"
            case retryable
        }
    }

    private let path: String
    private let maximumBytes: Int64
    private let now: @Sendable () -> Date
    private let lock = NSLock()

    static func installedStore() throws -> LocalOperationalDiagnostics {
        try LocalOperationalDiagnostics(
            directory: FileManager.default.homeDirectoryForCurrentUser
                .appendingPathComponent("Library/Application Support/Loom/diagnostics"),
            maximumBytes: 512 * 1_024,
            now: { Date() }
        )
    }

    init(
        directory: URL,
        maximumBytes: Int64,
        now: @escaping @Sendable () -> Date
    ) throws {
        guard directory.isFileURL, directory.path.hasPrefix("/"), maximumBytes >= 512 else {
            throw LocalProductClientError.invalidRequest
        }
        try FileManager.default.createDirectory(
            at: directory,
            withIntermediateDirectories: true,
            attributes: [.posixPermissions: 0o700]
        )
        let values = try directory.resourceValues(forKeys: [.isDirectoryKey, .isSymbolicLinkKey])
        guard values.isDirectory == true, values.isSymbolicLink != true else {
            throw LocalProductClientError.unavailable
        }
        try FileManager.default.setAttributes(
            [.posixPermissions: 0o700],
            ofItemAtPath: directory.path
        )
        path = directory.appendingPathComponent("app-operational.jsonl").path
        self.maximumBytes = maximumBytes
        self.now = now
    }

    func recordCredential(
        incidentID: String,
        operation: String,
        providerID: String,
        providerAccountID: String? = nil,
        stage: LocalIPCRemoteError.Stage,
        result: String,
        errorCode: LocalIPCRemoteError.Code?,
        retryable: Bool,
        elapsedMilliseconds: Int64
    ) throws {
        guard LocalIPCWire.validRequestID(incidentID),
              Self.validOperation(operation),
              LocalIPCClient.validIdentifier(providerID),
              providerAccountID == nil || LocalIPCClient.validProviderAccountID(
                providerAccountID!,
                providerID: providerID
              ),
              elapsedMilliseconds >= 0,
			  (result == "succeeded" && errorCode == nil && !retryable ||
                result == "failed" && errorCode != nil) else {
            throw LocalProductClientError.invalidRequest
        }
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        let record = CredentialRecord(
            occurredAt: formatter.string(from: now()),
            incidentID: incidentID,
            operation: operation,
            providerID: providerID,
            providerAccountID: providerAccountID,
            stage: stage.rawValue,
            elapsedMilliseconds: elapsedMilliseconds,
            result: result,
            errorCode: errorCode?.rawValue,
            retryable: retryable
        )
        try append(record)
    }

    func recordConversation(
        incidentID: String,
        threadID: String,
        profileID: String,
        stage: LocalIPCRemoteError.Stage,
        result: String,
        errorCode: LocalIPCRemoteError.Code?,
		httpStatus: Int = 0,
		providerErrorCode: String = "",
		retryAfterSeconds: Int64 = 0,
        retryable: Bool,
        elapsedMilliseconds: Int64
    ) throws {
        guard LocalIPCWire.validRequestID(incidentID),
              LocalIPCClient.validIdentifier(threadID),
              profileID.isEmpty || LocalIPCClient.validIdentifier(profileID),
              elapsedMilliseconds >= 0,
			  (httpStatus == 0 || (100...599).contains(httpStatus)),
			  Self.validProviderErrorCode(providerErrorCode),
			  (0...86_400).contains(retryAfterSeconds),
			  (result == "succeeded" && errorCode == nil && httpStatus == 0 &&
				providerErrorCode.isEmpty && retryAfterSeconds == 0 && !retryable ||
                result == "failed" && errorCode != nil) else {
            throw LocalProductClientError.invalidRequest
        }
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        try append(ConversationRecord(
            occurredAt: formatter.string(from: now()),
            incidentID: incidentID,
            threadID: threadID,
            profileID: profileID,
            stage: stage.rawValue,
            elapsedMilliseconds: elapsedMilliseconds,
            result: result,
            errorCode: errorCode?.rawValue,
			httpStatus: httpStatus == 0 ? nil : httpStatus,
			providerErrorCode: providerErrorCode.isEmpty ? nil : providerErrorCode,
			retryAfterSeconds: retryAfterSeconds == 0 ? nil : retryAfterSeconds,
            retryable: retryable
        ))
    }

    func recordMissionExecution(
        incidentID: String,
        executionOperation: String,
        stage: LocalIPCRemoteError.Stage,
        result: String,
        errorCode: LocalIPCRemoteError.Code?,
        retryable: Bool,
        elapsedMilliseconds: Int64
    ) throws {
        guard LocalIPCWire.validRequestID(incidentID),
              ["preflight", "start", "control"].contains(executionOperation),
              elapsedMilliseconds >= 0,
              (result == "succeeded" && errorCode == nil && !retryable
                || result == "failed" && errorCode != nil) else {
            throw LocalProductClientError.invalidRequest
        }
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        try append(MissionExecutionRecord(
            occurredAt: formatter.string(from: now()),
            incidentID: incidentID,
            executionOperation: executionOperation,
            stage: stage.rawValue,
            elapsedMilliseconds: elapsedMilliseconds,
            result: result,
            errorCode: errorCode?.rawValue,
            retryable: retryable
        ))
    }

    func recordAgentInput(
        incidentID: String,
        request: LocalProductAgentInputRequest,
        stage: LocalIPCRemoteError.Stage,
        result: String,
        errorCode: LocalIPCRemoteError.Code?,
        retryable: Bool,
        elapsedMilliseconds: Int64
    ) throws {
        guard LocalIPCWire.validRequestID(incidentID), request.isValid,
              elapsedMilliseconds >= 0,
              (result == "succeeded" && errorCode == nil && !retryable
                || result == "failed" && errorCode != nil) else {
            throw LocalProductClientError.invalidRequest
        }
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        try append(AgentInputRecord(
            occurredAt: formatter.string(from: now()),
            incidentID: incidentID,
            segmentID: request.segmentID,
            agentInstanceID: request.agentInstanceID,
            workItemID: request.workItemID,
            runID: request.runID,
            claimGeneration: request.claimGeneration,
            mode: request.mode.rawValue,
            stage: stage.rawValue,
            elapsedMilliseconds: elapsedMilliseconds,
            result: result,
            errorCode: errorCode?.rawValue,
            retryable: retryable
        ))
    }

    func recordAgentRecovery(
        incidentID: String,
        operation: LocalProductAgentRecoveryOperation,
        stage: LocalIPCRemoteError.Stage,
        result: String,
        errorCode: LocalIPCRemoteError.Code?,
        retryable: Bool,
        elapsedMilliseconds: Int64
    ) throws {
        guard LocalIPCWire.validRequestID(incidentID),
              elapsedMilliseconds >= 0,
              (result == "succeeded" && errorCode == nil && !retryable
                || result == "failed" && errorCode != nil) else {
            throw LocalProductClientError.invalidRequest
        }
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        try append(AgentRecoveryRecord(
            occurredAt: formatter.string(from: now()),
            incidentID: incidentID,
            recoveryOperation: operation.rawValue,
            stage: stage.rawValue,
            elapsedMilliseconds: elapsedMilliseconds,
            result: result,
            errorCode: errorCode?.rawValue,
            retryable: retryable
        ))
    }

    func recordToolRecovery(
        incidentID: String,
        operation: LocalProductToolRecoveryOperation,
        stage: LocalIPCRemoteError.Stage,
        result: String,
        errorCode: LocalIPCRemoteError.Code?,
        retryable: Bool,
        elapsedMilliseconds: Int64
    ) throws {
        guard LocalIPCWire.validRequestID(incidentID),
              elapsedMilliseconds >= 0,
              (result == "succeeded" && errorCode == nil && !retryable
                || result == "failed" && errorCode != nil) else {
            throw LocalProductClientError.invalidRequest
        }
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        try append(ToolRecoveryRecord(
            occurredAt: formatter.string(from: now()),
            incidentID: incidentID,
            recoveryOperation: operation.rawValue,
            stage: stage.rawValue,
            elapsedMilliseconds: elapsedMilliseconds,
            result: result,
            errorCode: errorCode?.rawValue,
            retryable: retryable
        ))
    }

	private static func validProviderErrorCode(_ value: String) -> Bool {
		guard value.utf8.count <= 64 else { return false }
		return value.utf8.allSatisfy { byte in
			(48...57).contains(byte) || (65...90).contains(byte) ||
				(97...122).contains(byte) || [45, 46, 95].contains(byte)
		}
	}

    private func append<Record: Encodable>(_ record: Record) throws {
        var line = try JSONEncoder().encode(record)
        line.append(0x0A)
        guard line.count <= maximumBytes else {
            throw LocalProductClientError.invalidRequest
        }

        lock.lock()
        defer { lock.unlock() }
        try rotateIfNeeded(incomingBytes: Int64(line.count))
        let descriptor = path.withCString {
            Darwin.open($0, O_WRONLY | O_CREAT | O_APPEND | O_NOFOLLOW, S_IRUSR | S_IWUSR)
        }
        guard descriptor >= 0 else { throw LocalProductClientError.unavailable }
        defer { Darwin.close(descriptor) }
        var status = stat()
        guard fstat(descriptor, &status) == 0,
              status.st_uid == geteuid(),
              status.st_mode & S_IFMT == S_IFREG,
              fchmod(descriptor, S_IRUSR | S_IWUSR) == 0 else {
            throw LocalProductClientError.unavailable
        }
        try line.withUnsafeBytes { bytes in
            var written = 0
            while written < bytes.count {
                let count = Darwin.write(
                    descriptor,
                    bytes.baseAddress!.advanced(by: written),
                    bytes.count - written
                )
                guard count > 0 else { throw LocalProductClientError.unavailable }
                written += count
            }
        }
        guard fsync(descriptor) == 0 else { throw LocalProductClientError.unavailable }
    }

    private func rotateIfNeeded(incomingBytes: Int64) throws {
        var status = stat()
        if lstat(path, &status) != 0 {
            guard errno == ENOENT else { throw LocalProductClientError.unavailable }
            return
        }
        guard status.st_uid == geteuid(), status.st_mode & S_IFMT == S_IFREG else {
            throw LocalProductClientError.unavailable
        }
        guard status.st_size + incomingBytes > maximumBytes else { return }
        let rotated = path + ".1"
        var rotatedStatus = stat()
        if lstat(rotated, &rotatedStatus) == 0 {
            guard rotatedStatus.st_uid == geteuid(),
                  rotatedStatus.st_mode & S_IFMT == S_IFREG,
                  unlink(rotated) == 0 else {
                throw LocalProductClientError.unavailable
            }
        } else if errno != ENOENT {
            throw LocalProductClientError.unavailable
        }
        guard rename(path, rotated) == 0,
              chmod(rotated, S_IRUSR | S_IWUSR) == 0 else {
            throw LocalProductClientError.unavailable
        }
    }

    private static func validOperation(_ operation: String) -> Bool {
        switch operation {
        case "credential_configure", "credential_verify", "credential_replace", "credential_revoke",
             "provider_account_policy_configure", "claude_code_cancel", "claude_code_connect":
            return true
        default:
            return false
        }
    }
}
