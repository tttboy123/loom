import Darwin
import Foundation

public enum LocalProductClientError: String, Error, Equatable, Sendable {
    case invalidRequest = "invalid_request"
    case invalidSocket = "invalid_socket"
    case invalidResponse = "invalid_response"
    case unavailable = "unavailable"
    case timeout = "timeout"
    case notFound = "not_found"
}

public struct LocalIPCRemoteError: Error, Equatable, Sendable {
    public enum Code: String, CaseIterable, Codable, Sendable {
        case invalidRequest = "invalid_request"
        case unsupportedVersion = "unsupported_version"
        case unknownMethod = "unknown_method"
        case unauthorizedPeer = "unauthorized_peer"
        case unsupportedPlatform = "unsupported_platform"
        case notFound = "not_found"
        case conflict
        case capabilityGap = "capability_gap"
        case staleView = "stale_view"
        case staleGeneration = "stale_generation"
        case digestMismatch = "digest_mismatch"
        case humanRequired = "human_required"
        case incompatible
        case denied
        case credentialUnavailable = "credential_unavailable"
        case credentialRejected = "credential_rejected"
        case credentialRollbackFailed = "credential_rollback_failed"
        case conversationUnavailable = "conversation_unavailable"
        case conversationLimit = "conversation_limit"
        case invalidResponse = "invalid_response"
        case providerAuth = "provider_auth"
        case providerRateLimit = "provider_rate_limit"
        case providerRejected = "provider_rejected"
		case providerInsufficientBalance = "provider_insufficient_balance"
		case providerModelUnavailable = "provider_model_unavailable"
		case providerInvalidRequest = "provider_invalid_request"
		case providerUnavailable = "provider_unavailable"
        case cursorConflict = "cursor_conflict"
        case streamGap = "stream_gap"
        case stateUnavailable = "state_unavailable"
        case timeout
        case busy
        case `internal`
    }

    public enum Stage: String, Codable, Sendable {
        case inputAdmission = "input_admission"
        case udsTransport = "uds_transport"
        case daemonAdmission = "daemon_admission"
        case helperValidation = "helper_validation"
        case helperStart = "helper_start"
        case helperAuthorization = "helper_authorization"
        case helperRequest = "helper_request"
        case helperTimeout = "helper_timeout"
        case helperResponse = "helper_response"
        case helperExit = "helper_exit"
        case keychainAccess = "keychain_access"
        case metadataCommit = "metadata_commit"
        case projectionRefresh = "projection_refresh"
        case vaultKeyLoad = "vault_key_load"
        case vaultOpen = "vault_open"
        case vaultEncrypt = "vault_encrypt"
        case vaultCommit = "vault_commit"
        case vaultDecrypt = "vault_decrypt"
        case vaultAADValidation = "vault_aad_validation"
        case vaultRotation = "vault_rotation"
        case vaultRecovery = "vault_recovery"
        case vaultExport = "vault_export"
        case credentialLeaseIssue = "credential_lease_issue"
        case credentialLeaseExpire = "credential_lease_expire"
        case credentialLeaseRevoke = "credential_lease_revoke"
        case migrationRead = "migration_read"
        case migrationCommit = "migration_commit"
        case migrationCleanup = "migration_cleanup"
        case providerDNS = "provider_dns"
        case providerTLS = "provider_tls"
        case providerConnect = "provider_connect"
        case providerHTTP = "provider_http"
        case providerAuth = "provider_auth"
        case providerRateLimit = "provider_rate_limit"
        case profilePublish = "profile_publish"
        case conversationDispatch = "conversation_dispatch"
        case agentAttemptDispatch = "agent_attempt_dispatch"
        case agentAttemptReconcile = "agent_attempt_reconcile"
        case agentInputAdmission = "agent_input_admission"
		case toolRecovery = "tool_recovery"
    }

    public let code: Code
    public let recoverable: Bool
    public let stage: Stage?
    public let incidentID: String?
	public let httpStatus: Int
	public let providerCode: String
	public let safeMessage: String
	public let retryAfterSeconds: Int64

    public init(
        code: Code,
        recoverable: Bool,
        stage: Stage? = nil,
		incidentID: String? = nil,
		httpStatus: Int = 0,
		providerCode: String = "",
		safeMessage: String = "",
		retryAfterSeconds: Int64 = 0
    ) {
        self.code = code
        self.recoverable = recoverable
        self.stage = stage
        self.incidentID = incidentID
		self.httpStatus = httpStatus
		self.providerCode = providerCode
		self.safeMessage = safeMessage
		self.retryAfterSeconds = retryAfterSeconds
    }
}

private indirect enum JSONValue: Codable {
    case object([String: JSONValue])
    case array([JSONValue])
    case string(String)
    case integer(Int64)
    case number(Double)
    case bool(Bool)
    case null

    init(from decoder: Decoder) throws {
        let value = try decoder.singleValueContainer()
        if value.decodeNil() {
            self = .null
        } else if let decoded = try? value.decode([String: JSONValue].self) {
            self = .object(decoded)
        } else if let decoded = try? value.decode([JSONValue].self) {
            self = .array(decoded)
        } else if let decoded = try? value.decode(String.self) {
            self = .string(decoded)
        } else if let decoded = try? value.decode(Bool.self) {
            self = .bool(decoded)
        } else if let decoded = try? value.decode(Int64.self) {
            self = .integer(decoded)
        } else if let decoded = try? value.decode(Double.self) {
            self = .number(decoded)
        } else {
            throw LocalProductClientError.invalidResponse
        }
    }

    func encode(to encoder: Encoder) throws {
        var value = encoder.singleValueContainer()
        switch self {
        case let .object(object): try value.encode(object)
        case let .array(array): try value.encode(array)
        case let .string(string): try value.encode(string)
        case let .integer(integer): try value.encode(integer)
        case let .number(number): try value.encode(number)
        case let .bool(bool): try value.encode(bool)
        case .null: try value.encodeNil()
        }
    }

    var isObject: Bool {
        if case .object = self { return true }
        return false
    }
}

private struct ResponseEnvelope: Decodable {
    let version: Int
    let requestID: String
    let journeyID: String?
    let ok: Bool
    let result: JSONValue?
    let error: ErrorEnvelope?

    enum CodingKeys: String, CodingKey {
        case version
        case requestID = "request_id"
        case journeyID = "journey_id"
        case ok, result, error
    }

    init(from decoder: Decoder) throws {
        try rejectIPCUnknownKeys(
            decoder,
            allowed: ["version", "request_id", "journey_id", "ok", "result", "error"]
        )
        let values = try decoder.container(keyedBy: CodingKeys.self)
        version = try values.decode(Int.self, forKey: .version)
        requestID = try values.decode(String.self, forKey: .requestID)
        journeyID = try values.decodeIfPresent(String.self, forKey: .journeyID)
        ok = try values.decode(Bool.self, forKey: .ok)
        result = try values.decodeIfPresent(JSONValue.self, forKey: .result)
        error = try values.decodeIfPresent(ErrorEnvelope.self, forKey: .error)
    }
}

private struct ErrorEnvelope: Decodable {
    let code: LocalIPCRemoteError.Code
    let message: String
    let recoverable: Bool
    let stage: LocalIPCRemoteError.Stage?

    enum CodingKeys: String, CodingKey {
        case code, message, recoverable, stage
    }

    init(from decoder: Decoder) throws {
        try rejectIPCUnknownKeys(
            decoder,
            allowed: ["code", "message", "recoverable", "stage"]
        )
        let values = try decoder.container(keyedBy: CodingKeys.self)
        code = try values.decode(LocalIPCRemoteError.Code.self, forKey: .code)
        message = try values.decode(String.self, forKey: .message)
        recoverable = try values.decodeIfPresent(
            Bool.self,
            forKey: .recoverable
        ) ?? false
        stage = try values.decodeIfPresent(
            LocalIPCRemoteError.Stage.self,
            forKey: .stage
        )
    }
}

private struct IPCCodingKey: CodingKey {
    let stringValue: String
    let intValue: Int? = nil
    init?(stringValue: String) { self.stringValue = stringValue }
    init?(intValue: Int) { return nil }
}

private func rejectIPCUnknownKeys(
    _ decoder: Decoder,
    allowed: Set<String>
) throws {
    let values = try decoder.container(keyedBy: IPCCodingKey.self)
    if values.allKeys.contains(where: { !allowed.contains($0.stringValue) }) {
        throw LocalProductClientError.invalidResponse
    }
}

public enum LocalIPCWire {
    public static func frame(_ body: Data, maximum: Int) throws -> Data {
        guard maximum > 0, !body.isEmpty, body.count <= maximum,
              body.count <= Int(UInt32.max) else {
            throw LocalProductClientError.invalidRequest
        }
        var length = UInt32(body.count).bigEndian
        var framed = Data(bytes: &length, count: MemoryLayout<UInt32>.size)
        framed.append(body)
        return framed
    }

    public static func unframe(_ data: Data, maximum: Int) throws -> Data {
        guard maximum > 0, data.count >= 4 else {
            throw LocalProductClientError.invalidResponse
        }
        let size = data.prefix(4).reduce(UInt32(0)) {
            ($0 << 8) | UInt32($1)
        }
        guard size > 0, size <= UInt32(maximum),
              data.count == Int(size) + 4 else {
            throw LocalProductClientError.invalidResponse
        }
        return data.dropFirst(4)
    }

    public static func decodeResponse(
        _ data: Data,
        expectedRequestID: String,
        expectedJourneyID: String? = nil
    ) throws -> Data {
        do {
            try StrictJSONScanner.validate(data)
            let envelope = try JSONDecoder().decode(ResponseEnvelope.self, from: data)
            guard envelope.version == 1,
                  envelope.requestID == expectedRequestID,
                  envelope.journeyID == expectedJourneyID,
                  validRequestID(expectedRequestID) else {
                throw LocalProductClientError.invalidResponse
            }
            if envelope.ok {
                guard envelope.error == nil,
                      let result = envelope.result,
                      result.isObject else {
                    throw LocalProductClientError.invalidResponse
                }
                return try JSONEncoder().encode(result)
            }
            guard envelope.result == nil, let error = envelope.error else {
                throw LocalProductClientError.invalidResponse
            }
            throw LocalIPCRemoteError(
                code: error.code,
                recoverable: error.recoverable,
                stage: error.stage,
                incidentID: envelope.requestID
            )
        } catch let error as LocalIPCRemoteError {
            throw error
        } catch let error as LocalProductClientError {
            throw error
        } catch {
            throw LocalProductClientError.invalidResponse
        }
    }

    static func validRequestID(_ value: String) -> Bool {
        guard (1...64).contains(value.utf8.count) else { return false }
        return value.utf8.allSatisfy { byte in
            (byte >= 0x41 && byte <= 0x5A) ||
                (byte >= 0x61 && byte <= 0x7A) ||
                (byte >= 0x30 && byte <= 0x39) ||
                byte == 0x2E ||
                byte == 0x5F ||
                byte == 0x3A ||
                byte == 0x2D
        }
    }
}

private struct SnapshotParams: Encodable {
    let afterTeamID = ""
    let afterRuntimeID = ""
    let afterRunID = ""
    let afterEvidenceID = ""
    let limit: Int

    enum CodingKeys: String, CodingKey {
        case afterTeamID = "after_team_id"
        case afterRuntimeID = "after_runtime_id"
        case afterRunID = "after_run_id"
        case afterEvidenceID = "after_evidence_id"
        case limit
    }
}

private struct TimelineParams: Encodable {
    let teamInstanceID: String
    let cursor: String
    let limit: Int

    enum CodingKeys: String, CodingKey {
        case teamInstanceID = "team_instance_id"
        case cursor, limit
    }
}

private struct EvolutionAssetSnapshotParams: Encodable {
    let cursor: String
    let limit: Int
    let assetKind = ""
    let lifecycle = ""
    let searchText = ""

    enum CodingKeys: String, CodingKey {
        case cursor, limit
        case assetKind = "asset_kind", lifecycle, searchText = "search_text"
    }
}

private struct PermissionSnapshotParams: Encodable {}

private struct EvolutionAssetDiffParams: Encodable {
    let definitionID: String
    let leftRevisionID: String
    let leftDigest: String
    let rightRevisionID: String
    let rightDigest: String

    enum CodingKeys: String, CodingKey {
        case definitionID = "definition_id"
        case leftRevisionID = "left_revision_id", leftDigest = "left_digest"
        case rightRevisionID = "right_revision_id", rightDigest = "right_digest"
    }
}

private struct EvolutionAssetCommandParams: Encodable {
    let operationID: String
    let action: String
    let expectedViewVersion: String
    let expectedStreamHeads: [EvolutionAssetStreamHead]
    let input: EvolutionAssetCommand
    enum CodingKeys: String, CodingKey {
        case operationID = "operation_id", action
        case expectedViewVersion = "expected_view_version"
        case expectedStreamHeads = "expected_stream_heads", input
    }
}

private struct BuilderStartParams: Encodable {
    let source: String
    let sourceID: String
    let sourceVersion: Int
    let sourceDigest: String

    enum CodingKeys: String, CodingKey {
        case source
        case sourceID = "source_id"
        case sourceVersion = "source_version"
        case sourceDigest = "source_digest"
    }
}

private struct BuilderAnswerParams: Encodable {
    let draftID: String
    let expectedRevision: Int
    let catalogDigest: String
    let viewVersion: String
    let questionID: String
    let answer: String

    enum CodingKeys: String, CodingKey {
        case draftID = "draft_id"
        case expectedRevision = "expected_revision"
        case catalogDigest = "catalog_digest"
        case viewVersion = "view_version"
        case questionID = "question_id"
        case answer
    }
}

private struct BuilderEditParams: Encodable {
    let draftID: String
    let expectedRevision: Int
    let catalogDigest: String
    let viewVersion: String
    let field: String
    let value: String
	let roleAgentDefinitionID: String

    enum CodingKeys: String, CodingKey {
        case draftID = "draft_id"
        case expectedRevision = "expected_revision"
        case catalogDigest = "catalog_digest"
        case viewVersion = "view_version"
		case field, value
		case roleAgentDefinitionID = "role_agent_definition_id"
    }
}

private struct BuilderValidateParams: Encodable {
    let draftID: String
    let expectedRevision: Int
    let catalogDigest: String
    let viewVersion: String

    enum CodingKeys: String, CodingKey {
        case draftID = "draft_id"
        case expectedRevision = "expected_revision"
        case catalogDigest = "catalog_digest"
        case viewVersion = "view_version"
    }
}

private struct BuilderConfirmParams: Encodable {
    let draftID: String
    let expectedRevision: Int
    let catalogDigest: String
    let viewVersion: String
    let bindingDigest: String
    let definitionID: String
    let scope: String
    let projectID: String
    let confirm: Bool

    enum CodingKeys: String, CodingKey {
        case draftID = "draft_id"
        case expectedRevision = "expected_revision"
        case catalogDigest = "catalog_digest"
        case viewVersion = "view_version"
        case bindingDigest = "binding_digest"
        case definitionID = "definition_id"
        case scope
        case projectID = "project_id"
        case confirm
    }
}

private struct TeamStatusParams: Encodable {
    let definitionID: String
    let expectedHead: Int64

    enum CodingKeys: String, CodingKey {
        case definitionID = "definition_id"
        case expectedHead = "expected_head"
    }
}

private struct CredentialParams: Encodable {
    let providerID: String
    let providerAccountID: String
    let credentialReference: String
    let expectedRevision: Int64
    let operationID: String?
    let secret: String

    enum CodingKeys: String, CodingKey {
        case providerID = "provider_id"
        case providerAccountID = "provider_account_id"
        case credentialReference = "credential_reference"
        case expectedRevision = "expected_revision"
        case operationID = "operation_id"
        case secret
    }
}

struct ProviderAccountPolicyParams: Encodable {
    let providerID: String
    let providerAccountID: String
    let expectedRevision: Int64
    let maximumConcurrentAttempts: Int
    let dispatchWindowSeconds: Int64
    let maximumDispatchStarts: Int
    let maximumAssignedBudgetUnits: Int64
    let trustDomain: String
    let retentionMode: String
    let dataRegion: String
    let operationID: String

    enum CodingKeys: String, CodingKey {
        case providerID = "provider_id"
        case providerAccountID = "provider_account_id"
        case expectedRevision = "expected_revision"
        case maximumConcurrentAttempts = "maximum_concurrent_attempts"
        case dispatchWindowSeconds = "dispatch_window_seconds"
        case maximumDispatchStarts = "maximum_dispatch_starts"
        case maximumAssignedBudgetUnits = "maximum_assigned_budget_units"
        case trustDomain = "trust_domain"
        case retentionMode = "retention_mode"
        case dataRegion = "data_region"
        case operationID = "operation_id"
    }
}

struct ProviderModelRateCardParams: Encodable {
    let providerID: String
    let providerAccountID: String
    let modelID: String
    let expectedRevision: Int64
    let currency: String
    let inputTokenBasis: String
    let inputMicrounitsPerMillion: Int64
    let outputMicrounitsPerMillion: Int64
    let cacheReadMicrounitsPerMillion: Int64
    let cacheWriteMicrounitsPerMillion: Int64
    let roundingMode: String
    let operationID: String

    enum CodingKeys: String, CodingKey {
        case providerID = "provider_id"
        case providerAccountID = "provider_account_id"
        case modelID = "model_id"
        case expectedRevision = "expected_revision"
        case currency
        case inputTokenBasis = "input_token_basis"
        case inputMicrounitsPerMillion = "input_microunits_per_million"
        case outputMicrounitsPerMillion = "output_microunits_per_million"
        case cacheReadMicrounitsPerMillion = "cache_read_microunits_per_million"
        case cacheWriteMicrounitsPerMillion = "cache_write_microunits_per_million"
        case roundingMode = "rounding_mode"
        case operationID = "operation_id"
    }
}

struct RemoteToolBackendEnrollmentParams: Encodable {
    let enrollmentID: String
    let backendKind: String
    let adapterID: String
    let providerID: String
    let providerAccountID: String
    let providerAccountPolicyVersion: Int
    let providerAccountPolicyRevision: Int64
    let providerAccountPolicyDigest: String
    let endpointFingerprint: String
    let mcpServerID: String
    let allowedTools: [String]
    let expectedRevision: Int64
    let maximumConcurrentCalls: Int
    let maximumCallsPerAttempt: Int
    let timeoutSeconds: Int64
    let maximumResultBytes: Int
    let maximumBudgetUnits: Int64
    let operationID: String

    enum CodingKeys: String, CodingKey {
        case enrollmentID = "enrollment_id"
        case backendKind = "backend_kind"
        case adapterID = "adapter_id"
        case providerID = "provider_id"
        case providerAccountID = "provider_account_id"
        case providerAccountPolicyVersion = "provider_account_policy_version"
        case providerAccountPolicyRevision = "provider_account_policy_revision"
        case providerAccountPolicyDigest = "provider_account_policy_digest"
        case endpointFingerprint = "endpoint_fingerprint"
        case mcpServerID = "mcp_server_id"
        case allowedTools = "allowed_tools"
        case expectedRevision = "expected_revision"
        case maximumConcurrentCalls = "maximum_concurrent_calls"
        case maximumCallsPerAttempt = "maximum_calls_per_attempt"
        case timeoutSeconds = "timeout_seconds"
        case maximumResultBytes = "maximum_result_bytes"
        case maximumBudgetUnits = "maximum_budget_units"
        case operationID = "operation_id"
    }
}

struct RemoteToolBackendEnrollmentRevokeParams: Encodable {
    let enrollmentID: String
    let providerID: String
    let providerAccountID: String
    let expectedRevision: Int64
    let operationID: String

    enum CodingKeys: String, CodingKey {
        case enrollmentID = "enrollment_id"
        case providerID = "provider_id"
        case providerAccountID = "provider_account_id"
        case expectedRevision = "expected_revision"
        case operationID = "operation_id"
    }
}

private struct CredentialVaultResetParams: Encodable {
    let confirmation: String
}

private struct CredentialVaultExportParams: Encodable {
    let passphrase: Data
    let destination: String
}

private struct IPCRequest<Params: Encodable>: Encodable {
    let version = 1
    let requestID: String
    let journeyID: String?
    let method: String
    let params: Params

    enum CodingKeys: String, CodingKey {
        case version
        case requestID = "request_id"
        case journeyID = "journey_id"
        case method, params
    }
}

public final class LocalIPCClient:
    LocalProductClientProtocol,
    LocalProductDecisionClientProtocol,
    LocalProductSetupClientProtocol,
    LocalProductExecutionClientProtocol,
    LocalProductAgentRecoveryClientProtocol,
	LocalProductToolRecoveryClientProtocol,
    LocalProductAgentInputClientProtocol,
    LocalProductHandoffClientProtocol,
    LocalProductAssetClientProtocol
{
    public static let requestMaximum = 65_536
    public static let responseMaximum = 524_288
    public static let maximumRequestTimeoutSeconds = 55
    private let socketPath: String
    private let requestID: @Sendable () -> String
    private let operationalDiagnostics: LocalOperationalDiagnostics
    private let recordsInstalledDiagnostics: Bool

    public convenience init(
        socketPath: String,
        requestID: @escaping @Sendable () -> String = {
            "loom-swift-\(UUID().uuidString.lowercased())"
        }
    ) throws {
        try self.init(
            socketPath: socketPath,
            requestID: requestID,
            operationalDiagnostics: LocalOperationalDiagnostics.installedStore(),
            recordsInstalledDiagnostics: Self.isInstalledSocketPath(socketPath)
        )
    }

    init(
        socketPath: String,
        requestID: @escaping @Sendable () -> String,
        operationalDiagnostics: LocalOperationalDiagnostics,
        recordsInstalledDiagnostics: Bool = true
    ) throws {
        guard Self.validateSocketConfiguration(socketPath) else {
            throw LocalProductClientError.invalidSocket
        }
        self.socketPath = socketPath
        self.requestID = requestID
        self.operationalDiagnostics = operationalDiagnostics
        self.recordsInstalledDiagnostics = recordsInstalledDiagnostics
    }

    public static func defaultClient() throws -> LocalIPCClient {
        let arguments = CommandLine.arguments
        if let socketIndex = arguments.firstIndex(of: "--socket") {
            guard arguments.filter({ $0 == "--socket" }).count == 1,
                  socketIndex + 1 < arguments.count else {
                throw LocalProductClientError.invalidSocket
            }
            return try LocalIPCClient(socketPath: arguments[socketIndex + 1])
        }
        let home = FileManager.default.homeDirectoryForCurrentUser
            .resolvingSymlinksInPath()
            .standardizedFileURL
        return try LocalIPCClient(
            socketPath: home
                .appendingPathComponent("Library/Application Support/Loom/run")
                .appendingPathComponent("loomd.sock")
                .path
        )
    }

    public func snapshot(limit: Int) async throws -> LocalProductSnapshot {
        guard (1...64).contains(limit) else {
            throw LocalProductClientError.invalidRequest
        }
        let result = try await call(
            method: "snapshot",
            params: SnapshotParams(limit: limit)
        )
        return try LocalProductWire.decodeSnapshot(result)
    }

    public func timeline(
        teamInstanceID: String,
        cursor: String,
        limit: Int
    ) async throws -> LocalProductTimelinePage {
        guard Self.validIdentifier(teamInstanceID),
              Self.validTimelineCursor(cursor),
              (1...64).contains(limit) else {
            throw LocalProductClientError.invalidRequest
        }
        let result = try await call(
            method: "timeline_page",
            params: TimelineParams(
                teamInstanceID: teamInstanceID,
                cursor: cursor,
                limit: limit
            )
        )
        return try LocalProductWire.decodeTimeline(result)
    }

    public func evolutionAssetSnapshot(
        journeyID: String,
        cursor: String = "",
        limit: Int = 64
    ) async throws -> EvolutionAssetSnapshot {
        guard Self.validJourneyID(journeyID), (1...64).contains(limit), cursor.utf8.count <= 64 else {
            throw LocalProductClientError.invalidRequest
        }
        let result = try await callJourney(
            journeyID: journeyID,
            method: "evolution_asset_snapshot",
            params: EvolutionAssetSnapshotParams(cursor: cursor, limit: limit)
        )
        let snapshot = try EvolutionAssetWire.decodeSnapshot(result)
        return snapshot
    }

    public func evolutionAssetDiff(
        journeyID: String,
        definitionID: String,
        left: EvolutionAssetRevision,
        right: EvolutionAssetRevision
    ) async throws -> EvolutionAssetDiff {
        guard Self.validJourneyID(journeyID), Self.validIdentifier(definitionID),
              left.definitionID == definitionID, right.definitionID == definitionID else {
            throw LocalProductClientError.invalidRequest
        }
        let result = try await callJourney(
            journeyID: journeyID,
            method: "evolution_asset_diff",
            params: EvolutionAssetDiffParams(
                definitionID: definitionID,
                leftRevisionID: left.revisionID, leftDigest: left.artifactDigest,
                rightRevisionID: right.revisionID, rightDigest: right.artifactDigest
            )
        )
        let diff = try EvolutionAssetWire.decodeDiff(result)
        guard diff.definitionID == definitionID,
              diff.leftRevisionID == left.revisionID, diff.leftDigest == left.artifactDigest,
              diff.rightRevisionID == right.revisionID, diff.rightDigest == right.artifactDigest else {
            throw LocalProductClientError.invalidResponse
        }
        return diff
    }

    public func evolutionAssetCommand(
        _ command: EvolutionAssetCommand
    ) async throws -> EvolutionAssetCommandReceipt {
        guard Self.validJourneyID(command.journeyID) else {
            throw LocalProductClientError.invalidRequest
        }
        let result = try await callJourney(
            journeyID: command.journeyID,
            method: "evolution_asset_command",
            params: EvolutionAssetCommandParams(
                operationID: command.operationID, action: command.action,
                expectedViewVersion: command.expectedViewVersion,
                expectedStreamHeads: command.expectedStreamHeads, input: command
            )
        )
        let receipt = try EvolutionAssetWire.decodeReceipt(result)
        guard receipt.operationID == command.operationID, receipt.action == command.action else {
            throw LocalProductClientError.invalidResponse
        }
        return receipt
    }

    public func permissionsSnapshot(
        journeyID: String
    ) async throws -> PermissionSnapshot {
        guard Self.validJourneyID(journeyID) else {
            throw LocalProductClientError.invalidRequest
        }
        let result = try await callJourney(
            journeyID: journeyID,
            method: "permissions_snapshot",
            params: PermissionSnapshotParams()
        )
        return try PermissionWire.decodeSnapshot(result)
    }

    public func permissionsAttention(
        journeyID: String
    ) async throws -> PermissionAttention {
        guard Self.validJourneyID(journeyID) else {
            throw LocalProductClientError.invalidRequest
        }
        let result = try await callJourney(
            journeyID: journeyID,
            method: "permissions_attention",
            params: PermissionSnapshotParams()
        )
        return try PermissionWire.decodeAttention(result)
    }

    public func executionSnapshot(
        journeyID: String
    ) async throws -> ExecutionSnapshot {
        guard Self.validJourneyID(journeyID) else {
            throw LocalProductClientError.invalidRequest
        }
        let result = try await callJourney(
            journeyID: journeyID,
            method: "execution_snapshot",
            params: PermissionSnapshotParams()
        )
        return try ExecutionWire.decodeSnapshot(result)
    }

    public func productionSnapshot(
        journeyID: String
    ) async throws -> ProductionSnapshot {
        guard Self.validJourneyID(journeyID) else {
            throw LocalProductClientError.invalidRequest
        }
        let result = try await callJourney(
            journeyID: journeyID,
            method: "production_snapshot",
            params: PermissionSnapshotParams()
        )
        return try ProductionWire.decodeSnapshot(result)
    }

    public func decideMission(
        _ command: LocalProductDecisionCommand
    ) async throws -> LocalProductDecisionResult {
        let result = try await call(
            method: "mission_decision",
            params: command
        )
        return try LocalProductDecisionWire.decodeResult(result)
    }

    public func readMissionDecision(
        _ command: LocalProductDecisionCommand
    ) async throws -> LocalProductDecisionSheet {
        guard command.operation == "read", command.action == "read" else {
            throw LocalProductClientError.invalidRequest
        }
        let result = try await call(
            method: "mission_decision",
            params: command
        )
        return try LocalProductDecisionWire.decodeSheet(result)
    }

    public func executeMission(
        _ command: LocalProductExecutionCommand
    ) async throws -> LocalProductExecutionEnvelope {
        let result = try await call(
            method: "mission_execution",
            params: command
        )
        let envelope = try LocalProductExecutionWire.decodeEnvelope(result)
        guard envelope.operation == command.operation else {
            throw LocalProductClientError.invalidResponse
        }
        if let preflight = envelope.preflight {
            guard preflight.missionID == command.missionID,
                  preflight.teamInstanceID == command.teamInstanceID,
                  preflight.workPackageID == command.workPackageID,
                  preflight.workPackageDigest == command.workPackageDigest,
                  preflight.viewVersion == command.expectedViewVersion else {
                throw LocalProductClientError.invalidResponse
            }
        }
        if let execution = envelope.result {
            guard execution.missionID == command.missionID,
                  execution.teamInstanceID == command.teamInstanceID else {
                throw LocalProductClientError.invalidResponse
            }
        }
        return envelope
    }

    public func sendAgentInput(
        _ request: LocalProductAgentInputRequest,
        incidentID: String
    ) async throws -> LocalProductAgentInputReceipt {
		let started = Date()
        guard request.isValid, LocalIPCWire.validRequestID(incidentID) else {
            throw LocalProductClientError.invalidRequest
        }
		do {
			let result = try await call(
				method: "agent_input",
				params: request,
				incidentID: incidentID
			)
			let receipt = try LocalProductAgentInputWire.decodeReceipt(result)
			guard receipt.incidentID == incidentID, receipt.mode == request.mode else {
				throw LocalProductClientError.invalidResponse
			}
			recordAgentInput(
				request: request, incidentID: incidentID, started: started,
				result: "succeeded", failure: nil
			)
			return receipt
		} catch {
			let failure: LocalIPCRemoteError
			if let remote = error as? LocalIPCRemoteError {
				failure = remote
			} else {
				failure = LocalIPCRemoteError(
					code: error as? LocalProductClientError == .invalidRequest
						? .invalidRequest : .stateUnavailable,
					recoverable: error as? LocalProductClientError != .invalidRequest,
					stage: error as? LocalProductClientError == .invalidRequest
						? .inputAdmission : .udsTransport,
					incidentID: incidentID
				)
			}
			recordAgentInput(
				request: request, incidentID: incidentID, started: started,
				result: "failed", failure: failure
			)
			throw failure
		}
    }

    public func recoverAgentAttempt(
        _ request: LocalProductAgentRecoveryRequest,
        incidentID: String
    ) async throws -> LocalProductAgentRecoveryResponse {
        let started = Date()
        guard request.isValid, LocalIPCWire.validRequestID(incidentID) else {
            let failure = LocalIPCRemoteError(
                code: .invalidRequest, recoverable: false,
                stage: .inputAdmission,
                incidentID: LocalIPCWire.validRequestID(incidentID) ? incidentID : nil
            )
            recordAgentRecovery(
                request: request, incidentID: incidentID, started: started,
                result: "failed", failure: failure
            )
            throw failure
        }
        do {
            let result = try await call(
                method: "agent_attempt_recovery",
                params: request,
                incidentID: incidentID
            )
            let response = try LocalProductAgentRecoveryWire.decodeResponse(result)
            guard response.incidentID == incidentID,
                  response.operation == request.operation else {
                throw LocalProductClientError.invalidResponse
            }
            recordAgentRecovery(
                request: request, incidentID: incidentID, started: started,
                result: "succeeded", failure: nil
            )
            return response
        } catch {
            let failure = Self.agentRecoveryOperationError(error, incidentID: incidentID)
            recordAgentRecovery(
                request: request, incidentID: incidentID, started: started,
                result: "failed", failure: failure
            )
            throw failure
        }
    }

    public func recoverToolCall(
        _ request: LocalProductToolRecoveryRequest,
        incidentID: String
    ) async throws -> LocalProductToolRecoveryResponse {
        let started = Date()
        guard request.isValid, LocalIPCWire.validRequestID(incidentID) else {
            let failure = LocalIPCRemoteError(
                code: .invalidRequest, recoverable: false,
                stage: .inputAdmission,
                incidentID: LocalIPCWire.validRequestID(incidentID) ? incidentID : nil
            )
            recordToolRecovery(
                request: request, incidentID: incidentID, started: started,
                result: "failed", failure: failure
            )
            throw failure
        }
        do {
            let result = try await call(
                method: "tool_recovery", params: request, incidentID: incidentID
            )
            let response = try LocalProductToolRecoveryWire.decodeResponse(result)
            guard response.incidentID == incidentID,
                  response.operation == request.operation else {
                throw LocalProductClientError.invalidResponse
            }
            recordToolRecovery(
                request: request, incidentID: incidentID, started: started,
                result: "succeeded", failure: nil
            )
            return response
        } catch {
            let failure = Self.agentRecoveryOperationError(error, incidentID: incidentID)
            recordToolRecovery(
                request: request, incidentID: incidentID, started: started,
                result: "failed", failure: failure
            )
            throw failure
        }
    }

    public func proposeSideTask(
        _ request: LocalProductSideTaskProposalRequest
    ) async throws -> LocalProductSideTaskProposalResult {
        let result = try await call(method: "side_task_handoff", params: request)
        return try LocalProductHandoffWire.decodeProposal(result)
    }

    public func createSideTask(
        _ request: LocalProductSideTaskCreateRequest
    ) async throws -> LocalProductSideTaskCreateResult {
        let result = try await call(method: "side_task_handoff", params: request)
        return try LocalProductHandoffWire.decodeCreate(result)
    }

    public func readSideTask(
        _ request: LocalProductSideTaskReadRequest
    ) async throws -> LocalProductSideTaskReadResult {
        let result = try await call(method: "side_task_handoff", params: request)
        return try LocalProductHandoffWire.decodeRead(result)
    }

    public func decideSideTask(
        _ request: LocalProductSideTaskDecisionRequest
    ) async throws -> LocalProductSideTaskDecisionResult {
        let result = try await call(method: "side_task_handoff", params: request)
        return try LocalProductHandoffWire.decodeDecision(result)
    }

    public func setupSnapshot() async throws -> LocalProductSetupSnapshot {
        let result = try await call(
            method: "setup_snapshot",
            params: EmptyParams()
        )
        return try LocalProductSetupWire.decodeSnapshot(result)
    }

    public func configureProviderAccountPolicy(
        providerID: String,
        providerAccountID: String,
        expectedRevision: Int64,
        maximumConcurrentAttempts: Int,
        dispatchWindowSeconds: Int64,
        maximumDispatchStarts: Int,
        maximumAssignedBudgetUnits: Int64,
        trustDomain: String,
        retentionMode: String,
        dataRegion: String
    ) async throws -> LocalProductProviderAccountPolicyResult {
        let operation = "provider_account_policy_configure"
        let started = Date()
        let incidentID = requestID()
        let diagnosticAccountID = Self.validProviderAccountID(
            providerAccountID,
            providerID: providerID
        ) ? providerAccountID : nil
        guard Self.validIdentifier(providerID),
              Self.validProviderAccountID(
                providerAccountID,
                providerID: providerID
              ),
              expectedRevision >= 0,
              expectedRevision < Int64(UInt32.max),
              (1...64).contains(maximumConcurrentAttempts),
              (1...86_400).contains(dispatchWindowSeconds),
              (1...1_000_000).contains(maximumDispatchStarts),
              maximumAssignedBudgetUnits >= 0 else {
            let failure = LocalIPCRemoteError(
                code: .invalidRequest,
                recoverable: false,
                stage: .inputAdmission,
                incidentID: incidentID
            )
            recordCredentialFailure(
                operation: operation,
                providerID: providerID,
                providerAccountID: diagnosticAccountID,
                failure: failure,
                incidentID: incidentID,
                started: started
            )
            throw failure
        }
        guard ["external_provider", "enterprise_tenant", "local_runtime"].contains(trustDomain),
              ["provider_default", "zero_data_retention", "limited_retention"].contains(retentionMode),
              ["global", "us", "eu", "apac", "local"].contains(dataRegion) else {
            throw LocalIPCRemoteError(
                code: .invalidRequest, recoverable: false,
                stage: .inputAdmission, incidentID: incidentID
            )
        }
        do {
            let result = try await call(
                method: operation,
                params: ProviderAccountPolicyParams(
                    providerID: providerID,
                    providerAccountID: providerAccountID,
                    expectedRevision: expectedRevision,
                    maximumConcurrentAttempts: maximumConcurrentAttempts,
                    dispatchWindowSeconds: dispatchWindowSeconds,
                    maximumDispatchStarts: maximumDispatchStarts,
                    maximumAssignedBudgetUnits: maximumAssignedBudgetUnits,
                    trustDomain: trustDomain,
                    retentionMode: retentionMode,
                    dataRegion: dataRegion,
                    operationID: "provider-policy-\(UUID().uuidString.lowercased())"
                ),
                incidentID: incidentID
            )
            let decoded = try JSONDecoder().decode(
                LocalProductProviderAccountPolicyResult.self,
                from: result
            )
            guard decoded.providerID == providerID,
                  decoded.providerAccountID == providerAccountID,
                  decoded.revision == expectedRevision + 1,
                  decoded.maximumConcurrentAttempts == maximumConcurrentAttempts,
                  decoded.dispatchWindowSeconds == dispatchWindowSeconds,
                  decoded.maximumDispatchStarts == maximumDispatchStarts,
                  decoded.maximumAssignedBudgetUnits == maximumAssignedBudgetUnits else {
                throw LocalProductClientError.invalidResponse
            }
            guard decoded.policyVersion == 2,
                  decoded.trustDomain == trustDomain,
                  decoded.retentionMode == retentionMode,
                  decoded.dataRegion == dataRegion else {
                throw LocalProductClientError.invalidResponse
            }
            if recordsInstalledDiagnostics {
                try? operationalDiagnostics.recordCredential(
                    incidentID: incidentID,
                    operation: operation,
                    providerID: providerID,
                    providerAccountID: providerAccountID,
                    stage: .projectionRefresh,
                    result: "succeeded",
                    errorCode: nil,
                    retryable: false,
                    elapsedMilliseconds: Self.elapsedMilliseconds(since: started)
                )
            }
            return decoded
        } catch {
            let failure = Self.credentialOperationError(error, incidentID: incidentID)
            recordCredentialFailure(
                operation: operation,
                providerID: providerID,
                providerAccountID: providerAccountID,
                failure: failure,
                incidentID: incidentID,
                started: started
            )
            throw failure
        }
    }

    public func configureProviderModelRateCard(
        providerID: String,
        providerAccountID: String,
        modelID: String,
        expectedRevision: Int64,
        currency: String,
        inputTokenBasis: String,
        inputMicrounitsPerMillion: Int64,
        outputMicrounitsPerMillion: Int64,
        cacheReadMicrounitsPerMillion: Int64,
        cacheWriteMicrounitsPerMillion: Int64
    ) async throws -> LocalProductProviderModelRateCardResult {
        let operation = "provider_model_rate_card_configure"
        let started = Date()
        let incidentID = requestID()
        let rates = [
            inputMicrounitsPerMillion, outputMicrounitsPerMillion,
            cacheReadMicrounitsPerMillion, cacheWriteMicrounitsPerMillion,
        ]
        guard Self.validIdentifier(providerID),
              Self.validProviderAccountID(providerAccountID, providerID: providerID),
              Self.validModelID(modelID),
              expectedRevision >= 0, expectedRevision < Int64(UInt32.max),
              currency.utf8.count == 3,
              currency.utf8.allSatisfy({ $0 >= 0x41 && $0 <= 0x5a }),
              ["input_includes_cache", "input_excludes_cache"].contains(inputTokenBasis),
              rates.allSatisfy({ (0...1_000_000_000_000).contains($0) }) else {
            let failure = LocalIPCRemoteError(
                code: .invalidRequest, recoverable: false,
                stage: .inputAdmission, incidentID: incidentID
            )
            recordCredentialFailure(
                operation: operation, providerID: providerID,
                providerAccountID: Self.validProviderAccountID(
                    providerAccountID, providerID: providerID
                ) ? providerAccountID : nil,
                failure: failure, incidentID: incidentID, started: started
            )
            throw failure
        }
        do {
            let result = try await call(
                method: operation,
                params: ProviderModelRateCardParams(
                    providerID: providerID, providerAccountID: providerAccountID,
                    modelID: modelID, expectedRevision: expectedRevision,
                    currency: currency, inputTokenBasis: inputTokenBasis,
                    inputMicrounitsPerMillion: inputMicrounitsPerMillion,
                    outputMicrounitsPerMillion: outputMicrounitsPerMillion,
                    cacheReadMicrounitsPerMillion: cacheReadMicrounitsPerMillion,
                    cacheWriteMicrounitsPerMillion: cacheWriteMicrounitsPerMillion,
                    roundingMode: "ceiling_per_attempt",
                    operationID: "provider-rate-card-\(UUID().uuidString.lowercased())"
                ),
                incidentID: incidentID
            )
            let decoded = try JSONDecoder().decode(
                LocalProductProviderModelRateCardResult.self, from: result
            )
            guard decoded.providerID == providerID,
                  decoded.providerAccountID == providerAccountID,
                  decoded.modelID == modelID,
                  decoded.revision == expectedRevision + 1,
                  decoded.currency == currency,
                  decoded.inputTokenBasis == inputTokenBasis,
                  decoded.inputMicrounitsPerMillion == inputMicrounitsPerMillion,
                  decoded.outputMicrounitsPerMillion == outputMicrounitsPerMillion,
                  decoded.cacheReadMicrounitsPerMillion == cacheReadMicrounitsPerMillion,
                  decoded.cacheWriteMicrounitsPerMillion == cacheWriteMicrounitsPerMillion else {
                throw LocalProductClientError.invalidResponse
            }
            if recordsInstalledDiagnostics {
                try? operationalDiagnostics.recordCredential(
                    incidentID: incidentID, operation: operation,
                    providerID: providerID, providerAccountID: providerAccountID,
                    stage: .projectionRefresh, result: "succeeded",
                    errorCode: nil, retryable: false,
                    elapsedMilliseconds: Self.elapsedMilliseconds(since: started)
                )
            }
            return decoded
        } catch {
            let failure = Self.credentialOperationError(error, incidentID: incidentID)
            recordCredentialFailure(
                operation: operation, providerID: providerID,
                providerAccountID: providerAccountID, failure: failure,
                incidentID: incidentID, started: started
            )
            throw failure
        }
    }

    public func configureRemoteToolBackend(
        _ command: LocalProductRemoteToolBackendEnrollmentCommand
    ) async throws -> LocalProductRemoteToolBackendEnrollmentResult {
        let operation = "remote_tool_backend_enrollment_configure"
        let started = Date()
        let incidentID = requestID()
        guard command.valid else {
            let failure = LocalIPCRemoteError(
                code: .invalidRequest, recoverable: false,
                stage: .inputAdmission, incidentID: incidentID
            )
            recordCredentialFailure(
                operation: operation, providerID: command.providerID,
                providerAccountID: Self.validProviderAccountID(
                    command.providerAccountID,
                    providerID: command.providerID
                ) ? command.providerAccountID : nil,
                failure: failure, incidentID: incidentID, started: started
            )
            throw failure
        }
        do {
            let result = try await call(
                method: operation,
                params: RemoteToolBackendEnrollmentParams(
                    enrollmentID: command.enrollmentID,
                    backendKind: command.backendKind,
                    adapterID: command.adapterID,
                    providerID: command.providerID,
                    providerAccountID: command.providerAccountID,
                    providerAccountPolicyVersion: command.providerAccountPolicyVersion,
                    providerAccountPolicyRevision: command.providerAccountPolicyRevision,
                    providerAccountPolicyDigest: command.providerAccountPolicyDigest,
                    endpointFingerprint: command.endpointFingerprint,
                    mcpServerID: command.mcpServerID,
                    allowedTools: command.allowedTools,
                    expectedRevision: command.expectedRevision,
                    maximumConcurrentCalls: command.maximumConcurrentCalls,
                    maximumCallsPerAttempt: command.maximumCallsPerAttempt,
                    timeoutSeconds: command.timeoutSeconds,
                    maximumResultBytes: command.maximumResultBytes,
                    maximumBudgetUnits: command.maximumBudgetUnits,
                    operationID: "remote-tool-enrollment-\(UUID().uuidString.lowercased())"
                ),
                incidentID: incidentID
            )
            let decoded = try JSONDecoder().decode(
                LocalProductRemoteToolBackendEnrollmentResult.self,
                from: result
            )
            guard Self.remoteToolResult(decoded, matches: command),
                  decoded.status == "active", decoded.policyCurrent else {
                throw LocalProductClientError.invalidResponse
            }
            recordRemoteToolSuccess(
                operation: operation, command: command,
                incidentID: incidentID, started: started
            )
            return decoded
        } catch {
            let failure = Self.credentialOperationError(error, incidentID: incidentID)
            recordCredentialFailure(
                operation: operation, providerID: command.providerID,
                providerAccountID: command.providerAccountID,
                failure: failure, incidentID: incidentID, started: started
            )
            throw failure
        }
    }

    public func revokeRemoteToolBackend(
        _ command: LocalProductRemoteToolBackendEnrollmentRevokeCommand
    ) async throws -> LocalProductRemoteToolBackendEnrollmentResult {
        let operation = "remote_tool_backend_enrollment_revoke"
        let started = Date()
        let incidentID = requestID()
        guard command.valid else {
            let failure = LocalIPCRemoteError(
                code: .invalidRequest, recoverable: false,
                stage: .inputAdmission, incidentID: incidentID
            )
            recordCredentialFailure(
                operation: operation, providerID: command.providerID,
                providerAccountID: nil, failure: failure,
                incidentID: incidentID, started: started
            )
            throw failure
        }
        do {
            let result = try await call(
                method: operation,
                params: RemoteToolBackendEnrollmentRevokeParams(
                    enrollmentID: command.enrollmentID,
                    providerID: command.providerID,
                    providerAccountID: command.providerAccountID,
                    expectedRevision: command.expectedRevision,
                    operationID: "remote-tool-revoke-\(UUID().uuidString.lowercased())"
                ),
                incidentID: incidentID
            )
            let decoded = try JSONDecoder().decode(
                LocalProductRemoteToolBackendEnrollmentResult.self,
                from: result
            )
            guard decoded.enrollmentID == command.enrollmentID,
                  decoded.providerID == command.providerID,
                  decoded.providerAccountID == command.providerAccountID,
                  decoded.revision == command.expectedRevision + 1,
                  decoded.status == "revoked" else {
                throw LocalProductClientError.invalidResponse
            }
            if recordsInstalledDiagnostics {
                try? operationalDiagnostics.recordCredential(
                    incidentID: incidentID, operation: operation,
                    providerID: command.providerID,
                    providerAccountID: command.providerAccountID,
                    stage: .projectionRefresh, result: "succeeded",
                    errorCode: nil, retryable: false,
                    elapsedMilliseconds: Self.elapsedMilliseconds(since: started)
                )
            }
            return decoded
        } catch {
            let failure = Self.credentialOperationError(error, incidentID: incidentID)
            recordCredentialFailure(
                operation: operation, providerID: command.providerID,
                providerAccountID: command.providerAccountID,
                failure: failure, incidentID: incidentID, started: started
            )
            throw failure
        }
    }

    private static func remoteToolResult(
        _ result: LocalProductRemoteToolBackendEnrollmentResult,
        matches command: LocalProductRemoteToolBackendEnrollmentCommand
    ) -> Bool {
        result.enrollmentID == command.enrollmentID
            && result.backendKind == command.backendKind
            && result.adapterID == command.adapterID
            && result.providerID == command.providerID
            && result.providerAccountID == command.providerAccountID
            && result.providerAccountPolicyVersion
                == command.providerAccountPolicyVersion
            && result.providerAccountPolicyRevision
                == command.providerAccountPolicyRevision
            && result.providerAccountPolicyDigest
                == command.providerAccountPolicyDigest
            && result.endpointFingerprint == command.endpointFingerprint
            && result.mcpServerID == command.mcpServerID
            && result.allowedTools == command.allowedTools
            && result.revision == command.expectedRevision + 1
            && result.maximumConcurrentCalls == command.maximumConcurrentCalls
            && result.maximumCallsPerAttempt == command.maximumCallsPerAttempt
            && result.timeoutSeconds == command.timeoutSeconds
            && result.maximumResultBytes == command.maximumResultBytes
            && result.maximumBudgetUnits == command.maximumBudgetUnits
    }

    private func recordRemoteToolSuccess(
        operation: String,
        command: LocalProductRemoteToolBackendEnrollmentCommand,
        incidentID: String,
        started: Date
    ) {
        guard recordsInstalledDiagnostics else { return }
        try? operationalDiagnostics.recordCredential(
            incidentID: incidentID, operation: operation,
            providerID: command.providerID,
            providerAccountID: command.providerAccountID,
            stage: .projectionRefresh, result: "succeeded",
            errorCode: nil, retryable: false,
            elapsedMilliseconds: Self.elapsedMilliseconds(since: started)
        )
    }

    public func rotateCredentialVault() async throws {
        _ = try await call(
            method: "credential_vault_rotate",
            params: EmptyParams()
        )
    }

    public func lockCredentialVault() async throws {
        _ = try await call(
            method: "credential_vault_lock",
            params: EmptyParams()
        )
    }

    public func unlockCredentialVault() async throws {
        _ = try await call(
            method: "credential_vault_unlock",
            params: EmptyParams()
        )
    }

    public func resetCredentialVault() async throws {
        _ = try await call(
            method: "credential_vault_reset",
            params: CredentialVaultResetParams(
                confirmation: "reset_recovery_vault"
            )
        )
    }

    public func exportCredentialVault(
        passphrase: String,
        destination: String
    ) async throws -> LocalProductCredentialVaultExportResult {
        var passphraseData = Data(passphrase.utf8)
        defer { passphraseData.resetBytes(in: 0..<passphraseData.count) }
        guard passphraseData.count >= 12, passphraseData.count <= 1_024,
              destination.hasPrefix("/"), destination.hasSuffix(".loomvault"),
              destination.utf8.count <= 1_024 else {
            throw LocalProductClientError.invalidRequest
        }
        let result = try await call(
            method: "credential_vault_export",
            params: CredentialVaultExportParams(
                passphrase: passphraseData,
                destination: destination
            )
        )
        return try JSONDecoder().decode(
            LocalProductCredentialVaultExportResult.self,
            from: result
        )
    }

    public func connectCodex() async throws -> LocalProductProviderConnectResult {
        let result = try await call(
            method: "codex_connect",
            params: EmptyParams()
        )
        return try LocalProductSetupWire.decodeProviderConnectResult(result)
    }

    public func startBuilder(
        source: String = "blank",
        sourceID: String = "",
        sourceVersion: Int = 0,
        sourceDigest: String = ""
    ) async throws -> LocalProductBuilderSession {
        guard ["blank", "saved_team", "template"].contains(source),
              sourceID.isEmpty || Self.validIdentifier(sourceID),
              sourceVersion >= 0,
              sourceDigest.isEmpty || Self.validDigest(sourceDigest) else {
            throw LocalProductClientError.invalidRequest
        }
        let result = try await call(
            method: "builder_start",
            params: BuilderStartParams(
                source: source,
                sourceID: sourceID,
                sourceVersion: sourceVersion,
                sourceDigest: sourceDigest
            )
        )
        return try LocalProductSetupWire.decodeBuilderSession(result)
    }

    public func answerBuilder(
        session: LocalProductBuilderSession,
        answer: String
    ) async throws -> LocalProductBuilderSession {
        guard Self.validBuilderText(answer),
              !session.question.id.isEmpty else {
            throw LocalProductClientError.invalidRequest
        }
        let result = try await call(
            method: "builder_answer",
            params: BuilderAnswerParams(
                draftID: session.draftID,
                expectedRevision: session.revision,
                catalogDigest: session.catalogDigest,
                viewVersion: session.viewVersion,
                questionID: session.question.id,
                answer: answer
            )
        )
        return try LocalProductSetupWire.decodeBuilderSession(result)
    }

    public func editBuilder(
        session: LocalProductBuilderSession,
        field: String,
        value: String
    ) async throws -> LocalProductBuilderSession {
		return try await editBuilder(
			session: session,
			field: field,
			value: value,
			roleAgentDefinitionID: ""
		)
	}

	public func editBuilder(
		session: LocalProductBuilderSession,
		field: String,
		value: String,
		roleAgentDefinitionID: String
	) async throws -> LocalProductBuilderSession {
		guard LocalProductStore.allowsBuilderEditField(field),
			LocalProductStore.validBuilderEditTarget(
				field: field,
				roleAgentDefinitionID: roleAgentDefinitionID
			),
			Self.validBuilderText(value) else {
            throw LocalProductClientError.invalidRequest
        }
        let result = try await call(
            method: "builder_edit",
            params: BuilderEditParams(
                draftID: session.draftID,
                expectedRevision: session.revision,
                catalogDigest: session.catalogDigest,
                viewVersion: session.viewVersion,
                field: field,
				value: value,
				roleAgentDefinitionID: roleAgentDefinitionID
            )
        )
        return try LocalProductSetupWire.decodeBuilderSession(result)
    }

    public func validateBuilder(
        session: LocalProductBuilderSession
    ) async throws -> LocalProductBuilderSession {
        let result = try await call(
            method: "builder_validate",
            params: BuilderValidateParams(
                draftID: session.draftID,
                expectedRevision: session.revision,
                catalogDigest: session.catalogDigest,
                viewVersion: session.viewVersion
            )
        )
        return try LocalProductSetupWire.decodeBuilderSession(result)
    }

    public func confirmBuilder(
        session: LocalProductBuilderSession,
        definitionID: String
    ) async throws -> LocalProductBuilderConfirmation {
        guard session.canConfirm,
              Self.validIdentifier(definitionID),
              Self.validDigest(session.bindingDigest) else {
            throw LocalProductClientError.invalidRequest
        }
        let result = try await call(
            method: "builder_confirm",
            params: BuilderConfirmParams(
                draftID: session.draftID,
                expectedRevision: session.revision,
                catalogDigest: session.catalogDigest,
                viewVersion: session.viewVersion,
                bindingDigest: session.bindingDigest,
                definitionID: definitionID,
                scope: "reusable",
                projectID: "",
                confirm: true
            )
        )
        return try LocalProductSetupWire.decodeBuilderConfirmation(result)
    }

    public func archiveTeam(
        definitionID: String,
        expectedHead: Int64
    ) async throws -> LocalProductSetupSavedTeam {
        try await teamStatus(
            method: "team_archive",
            definitionID: definitionID,
            expectedHead: expectedHead
        )
    }

    public func restoreTeam(
        definitionID: String,
        expectedHead: Int64
    ) async throws -> LocalProductSetupSavedTeam {
        try await teamStatus(
            method: "team_restore",
            definitionID: definitionID,
            expectedHead: expectedHead
        )
    }

    public func configureMiniMax(
        secret: String
    ) async throws -> LocalProductCredentialSetupResult {
        try await configureCredential(providerID: "minimax", secret: secret)
    }

    public func configureCredential(
        providerID: String,
        secret: String
    ) async throws -> LocalProductCredentialSetupResult {
        try await configureCredential(
            providerID: providerID,
            providerAccountID: providerID + ".primary",
            secret: secret
        )
    }

    public func configureCredential(
        providerID: String,
        providerAccountID: String,
        secret: String
    ) async throws -> LocalProductCredentialSetupResult {
        let started = Date()
        let incidentID = requestID()
        guard Self.validIdentifier(providerID),
              Self.validProviderAccountID(providerAccountID, providerID: providerID),
              let normalizedSecret = Self.normalizedCredentialSecret(secret) else {
            let failure = LocalIPCRemoteError(
                code: .invalidRequest,
                recoverable: false,
                stage: .inputAdmission,
                incidentID: incidentID
            )
            recordCredentialFailure(
                operation: "credential_configure",
                providerID: providerID,
                providerAccountID: providerAccountID,
                failure: failure,
                incidentID: incidentID,
                started: started
            )
            throw failure
        }
        do {
            let result = try await call(
                method: "credential_configure",
                params: CredentialParams(
                    providerID: providerID,
                    providerAccountID: providerAccountID,
                    credentialReference: "",
                    expectedRevision: 0,
                    operationID: nil,
                    secret: normalizedSecret
                ),
                incidentID: incidentID
            )
            let decoded = try LocalProductSetupWire.decodeCredentialResult(result)
            recordCredentialResult(
                operation: "credential_configure",
                providerID: providerID,
                providerAccountID: providerAccountID,
                result: decoded,
                incidentID: incidentID,
                started: started
            )
            return decoded
        } catch {
            let failure = Self.credentialOperationError(error, incidentID: incidentID)
            recordCredentialFailure(
                operation: "credential_configure",
                providerID: providerID,
                providerAccountID: providerAccountID,
                failure: failure,
                incidentID: incidentID,
                started: started
            )
            throw failure
        }
    }

    public func verifyMiniMax(
        reference: String,
        revision: Int64
    ) async throws -> LocalProductCredentialSetupResult {
        return try await verifyCredential(
            providerID: "minimax",
            reference: reference,
            revision: revision
        )
    }

    public func verifyCredential(
        providerID: String,
        reference: String,
        revision: Int64
    ) async throws -> LocalProductCredentialSetupResult {
        try await verifyCredential(
            providerID: providerID,
            providerAccountID: providerID + ".primary",
            reference: reference,
            revision: revision
        )
    }

    public func verifyCredential(
        providerID: String,
        providerAccountID: String,
        reference: String,
        revision: Int64
    ) async throws -> LocalProductCredentialSetupResult {
        let operationID = UUID().uuidString.lowercased()
        return try await credentialMutation(
            method: "credential_verify",
            providerID: providerID,
            providerAccountID: providerAccountID,
            reference: reference,
            revision: revision,
            operationID: operationID,
            secret: ""
        )
    }

    public func replaceMiniMax(
        reference: String,
        revision: Int64,
        secret: String
    ) async throws -> LocalProductCredentialSetupResult {
        try await replaceCredential(
            providerID: "minimax",
            reference: reference,
            revision: revision,
            secret: secret
        )
    }

    public func replaceCredential(
        providerID: String,
        reference: String,
        revision: Int64,
        secret: String
    ) async throws -> LocalProductCredentialSetupResult {
        try await replaceCredential(
            providerID: providerID,
            providerAccountID: providerID + ".primary",
            reference: reference,
            revision: revision,
            secret: secret
        )
    }

    public func replaceCredential(
        providerID: String,
        providerAccountID: String,
        reference: String,
        revision: Int64,
        secret: String
    ) async throws -> LocalProductCredentialSetupResult {
        let started = Date()
        let incidentID = requestID()
        guard Self.validIdentifier(providerID),
              Self.validProviderAccountID(providerAccountID, providerID: providerID),
              let normalizedSecret = Self.normalizedCredentialSecret(secret) else {
            let failure = LocalIPCRemoteError(
                code: .invalidRequest,
                recoverable: false,
                stage: .inputAdmission,
                incidentID: incidentID
            )
            recordCredentialFailure(
                operation: "credential_replace",
                providerID: providerID,
                providerAccountID: providerAccountID,
                failure: failure,
                incidentID: incidentID,
                started: started
            )
            throw failure
        }
        return try await credentialMutation(
            method: "credential_replace",
            providerID: providerID,
            providerAccountID: providerAccountID,
            reference: reference,
            revision: revision,
            incidentID: incidentID,
            started: started,
            secret: normalizedSecret
        )
    }

    public func revokeMiniMax(
        reference: String,
        revision: Int64
    ) async throws -> LocalProductCredentialSetupResult {
        try await revokeCredential(
            providerID: "minimax",
            reference: reference,
            revision: revision
        )
    }

    public func revokeCredential(
        providerID: String,
        reference: String,
        revision: Int64
    ) async throws -> LocalProductCredentialSetupResult {
        try await revokeCredential(
            providerID: providerID,
            providerAccountID: providerID + ".primary",
            reference: reference,
            revision: revision
        )
    }

    public func revokeCredential(
        providerID: String,
        providerAccountID: String,
        reference: String,
        revision: Int64
    ) async throws -> LocalProductCredentialSetupResult {
        try await credentialMutation(
            method: "credential_revoke",
            providerID: providerID,
            providerAccountID: providerAccountID,
            reference: reference,
            revision: revision,
            secret: ""
        )
    }

    public func ping() async throws -> Bool {
        let result = try await call(method: "ping", params: EmptyParams())
        try StrictJSONScanner.validate(result)
        let decoded = try JSONDecoder().decode(PingResult.self, from: result)
        return decoded.protocolVersion == 1 && decoded.available &&
            !decoded.buildID.isEmpty
    }

    public func chatThread(threadID: String) async throws -> LocalProductChatThread {
        guard Self.validIdentifier(threadID) else {
            throw LocalProductClientError.invalidRequest
        }
        let result = try await call(
            method: "chat_thread",
            params: LocalProductChatThreadRequest(threadID: threadID)
        )
        return try LocalProductWire.decodeChatThread(result)
    }

    public func deleteChatThread(threadID: String) async throws {
        guard Self.validIdentifier(threadID) else {
            throw LocalProductClientError.invalidRequest
        }
        _ = try await call(
            method: "chat_thread_delete",
            params: LocalProductChatThreadRequest(threadID: threadID)
        )
    }

    public func sendChatMessage(threadID: String, content: String) async throws -> LocalProductChatThread {
        try await sendChatMessage(
            threadID: threadID,
            content: content,
            profileID: ""
        )
    }

    public func sendChatMessage(
        threadID: String,
        content: String,
        profileID: String
    ) async throws -> LocalProductChatThread {
        try await sendChatMessage(
            threadID: threadID,
            content: content,
            profileID: profileID,
            incidentID: requestID()
        )
    }

    public func sendChatMessage(
        threadID: String,
        content: String,
        profileID: String,
        incidentID: String
    ) async throws -> LocalProductChatThread {
        try await sendChatMessage(
            threadID: threadID,
            content: content,
            profileID: profileID,
            contextMode: nil,
            incidentID: incidentID
        )
    }

    public func sendChatMessage(
        threadID: String,
        content: String,
        profileID: String,
        contextMode: LocalProductConversationContextMode?,
        incidentID: String
    ) async throws -> LocalProductChatThread {
        try await sendChatMessage(
            threadID: threadID,
            content: content,
            profileID: profileID,
            contextMode: contextMode,
            expectedExecutionBinding: nil,
            incidentID: incidentID
        )
    }

    public func sendChatMessage(
        threadID: String,
        content: String,
        profileID: String,
        contextMode: LocalProductConversationContextMode?,
        expectedExecutionBinding: LocalProductConversationExecutionBinding?,
        incidentID: String
    ) async throws -> LocalProductChatThread {
        try await sendChatMessage(
            threadID: threadID,
            content: content,
            profileID: profileID,
            modelID: "",
            reasoningEffort: "",
            contextMode: contextMode,
            expectedExecutionBinding: expectedExecutionBinding,
            incidentID: incidentID
        )
    }

    public func sendChatMessage(
        threadID: String,
        content: String,
        profileID: String,
        modelID: String,
        reasoningEffort: String,
        contextMode: LocalProductConversationContextMode?,
        expectedExecutionBinding: LocalProductConversationExecutionBinding?,
        incidentID: String
    ) async throws -> LocalProductChatThread {
        let started = Date()
        guard Self.validIdentifier(threadID),
              profileID.isEmpty || Self.validIdentifier(profileID),
              modelID.isEmpty || Self.validModelID(modelID),
              reasoningEffort.isEmpty || Self.validIdentifier(reasoningEffort),
              !content.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty,
              LocalIPCWire.validRequestID(incidentID) else {
            let failure = LocalIPCRemoteError(
                code: .invalidRequest,
                recoverable: false,
                stage: .inputAdmission,
                incidentID: incidentID
            )
            recordConversationFailure(
                threadID: threadID,
                profileID: profileID,
                failure: failure,
                incidentID: incidentID,
                started: started
            )
            throw failure
        }
        do {
            let result = try await call(
                method: "chat_message",
                params: LocalProductChatMessageRequest(
                    threadID: threadID,
                    content: content,
                    profileID: profileID,
                    modelID: modelID,
                    reasoningEffort: reasoningEffort,
                    contextMode: contextMode,
                    expectedExecutionBinding: expectedExecutionBinding
                ),
                incidentID: incidentID
            )
            let thread = try LocalProductWire.decodeChatThread(result)
            if let failure = Self.conversationAttemptError(
                thread,
                incidentID: incidentID
            ) {
                recordConversationFailure(
                    threadID: threadID,
                    profileID: profileID,
                    failure: failure,
                    incidentID: incidentID,
                    started: started
                )
            } else {
                recordConversationResult(
                    threadID: threadID,
                    profileID: profileID,
                    incidentID: incidentID,
                    started: started
                )
            }
            return thread
        } catch {
            let failure = Self.conversationOperationError(
                error,
                incidentID: incidentID
            )
            recordConversationFailure(
                threadID: threadID,
                profileID: profileID,
                failure: failure,
                incidentID: incidentID,
                started: started
            )
            throw failure
        }
    }

    static func conversationAttemptError(
        _ thread: LocalProductChatThread,
        incidentID: String
    ) -> LocalIPCRemoteError? {
        guard let attempt = thread.attempts.last(where: {
            $0.incidentID == incidentID && $0.status == "failed"
        }) else {
            return nil
        }
        return LocalIPCRemoteError(
            code: LocalIPCRemoteError.Code(rawValue: attempt.failureCode)
                ?? .stateUnavailable,
            recoverable: attempt.retryable,
            stage: LocalIPCRemoteError.Stage(rawValue: attempt.failureStage)
                ?? .conversationDispatch,
			incidentID: incidentID,
			httpStatus: attempt.httpStatus,
			providerCode: attempt.providerCode,
			safeMessage: attempt.failureMessage,
			retryAfterSeconds: attempt.retryAfterSeconds
        )
    }

    func call<Params: Encodable>(
        method: String,
        params: Params,
        incidentID: String? = nil
    ) async throws -> Data {
        let id = incidentID ?? requestID()
        let methods = Set([
            "ping", "agent_attempt_recovery", "tool_recovery", "agent_input", "snapshot", "timeline_page", "setup_snapshot",
            "codex_connect", "provider_account_policy_configure",
            "provider_model_rate_card_configure",
            "remote_tool_backend_enrollment_configure",
            "remote_tool_backend_enrollment_revoke",
            "builder_start", "builder_answer", "builder_edit",
            "builder_validate", "builder_confirm", "team_archive",
            "team_restore", "credential_configure", "credential_verify",
            "credential_replace", "credential_revoke", "credential_vault_rotate",
            "credential_vault_lock", "credential_vault_unlock", "credential_vault_reset",
            "credential_vault_export",
            "mission_decision",
            "mission_execution",
            "side_task_handoff",
            "chat_thread",
            "chat_thread_delete",
            "chat_message",
        ])
        guard LocalIPCWire.validRequestID(id), methods.contains(method) else {
            throw LocalProductClientError.invalidRequest
        }
        let request = IPCRequest(
            requestID: id,
            journeyID: nil,
            method: method,
            params: params
        )
        let body = try JSONEncoder().encode(request)
        try StrictJSONScanner.validate(body)
        let framed = try LocalIPCWire.frame(
            body,
            maximum: Self.requestMaximum
        )
        let response = try await Task.detached {
            try Self.exchange(
                path: self.socketPath,
                request: framed,
                timeoutSeconds: Self.requestTimeoutSeconds(for: method)
            )
        }.value
        return try LocalIPCWire.decodeResponse(
            response,
            expectedRequestID: id
        )
    }

    func callJourney<Params: Encodable>(
        journeyID: String,
        method: String,
        params: Params
    ) async throws -> Data {
        let id = requestID()
        let methods: Set<String> = [
            "evolution_asset_snapshot", "evolution_asset_diff", "evolution_asset_command",
            "queue_snapshot", "queue_command",
            "workers_snapshot", "workers_command",
            "permissions_snapshot", "permissions_attention",
        ]
        guard LocalIPCWire.validRequestID(id), Self.validJourneyID(journeyID),
              methods.contains(method) else {
            throw LocalProductClientError.invalidRequest
        }
        let body = try JSONEncoder().encode(
            IPCRequest(requestID: id, journeyID: journeyID, method: method, params: params)
        )
        try StrictJSONScanner.validate(body)
        let response = try await Task.detached {
            try Self.exchange(
                path: self.socketPath,
                request: try LocalIPCWire.frame(body, maximum: Self.requestMaximum),
                timeoutSeconds: Self.requestTimeoutSeconds(for: method)
            )
        }.value
        return try LocalIPCWire.decodeResponse(
            response,
            expectedRequestID: id,
            expectedJourneyID: journeyID
        )
    }

    func callJourneyRaw(
        journeyID: String,
        method: String,
        params: [String: Any]
    ) async throws -> Data {
        let id = requestID()
        let methods: Set<String> = [
            "workers_snapshot", "workers_command",
            "integration_snapshot", "integration_command",
        ]
        guard LocalIPCWire.validRequestID(id), Self.validJourneyID(journeyID),
              methods.contains(method) else {
            throw LocalProductClientError.invalidRequest
        }
        let body = try JSONSerialization.data(withJSONObject: [
            "version": 1,
            "request_id": id,
            "journey_id": journeyID,
            "method": method,
            "params": params,
        ])
        try StrictJSONScanner.validate(body)
        let response = try await Task.detached {
            try Self.exchange(
                path: self.socketPath,
                request: try LocalIPCWire.frame(body, maximum: Self.requestMaximum),
                timeoutSeconds: Self.requestTimeoutSeconds(for: method)
            )
        }.value
        return try LocalIPCWire.decodeResponse(
            response,
            expectedRequestID: id,
            expectedJourneyID: journeyID
        )
    }

    static func requestTimeoutSeconds(for method: String) -> Int {
        switch method {
        case "chat_message", "agent_attempt_recovery":
            return 55
        case "credential_verify", "credential_vault_rotate",
             "credential_vault_lock", "credential_vault_unlock", "credential_vault_reset",
             "credential_vault_export",
			 "mission_execution", "tool_recovery":
            return 15
        default:
            return 5
        }
    }

    static func validJourneyID(_ value: String) -> Bool {
        guard let uuid = UUID(uuidString: value), uuid.uuidString.lowercased() == value,
              value[value.index(value.startIndex, offsetBy: 14)] == "4" else { return false }
        let variant = value[value.index(value.startIndex, offsetBy: 19)]
        return "89ab".contains(variant)
    }

    private static func exchange(
        path: String,
        request: Data,
        timeoutSeconds: Int
    ) throws -> Data {
        guard validateSocketPath(path) else {
            throw LocalProductClientError.invalidSocket
        }
        guard (1...maximumRequestTimeoutSeconds).contains(timeoutSeconds) else {
            throw LocalProductClientError.invalidRequest
        }
        let descriptor = socket(AF_UNIX, SOCK_STREAM, 0)
        guard descriptor >= 0 else {
            throw LocalProductClientError.unavailable
        }
        defer { Darwin.close(descriptor) }

        var timeout = timeval(tv_sec: timeoutSeconds, tv_usec: 0)
        guard setsockopt(
            descriptor,
            SOL_SOCKET,
            SO_SNDTIMEO,
            &timeout,
            socklen_t(MemoryLayout<timeval>.size)
        ) == 0,
        setsockopt(
            descriptor,
            SOL_SOCKET,
            SO_RCVTIMEO,
            &timeout,
            socklen_t(MemoryLayout<timeval>.size)
        ) == 0 else {
            throw LocalProductClientError.unavailable
        }

        var address = sockaddr_un()
        address.sun_family = sa_family_t(AF_UNIX)
        let pathBytes = Array(path.utf8CString)
        guard pathBytes.count <= MemoryLayout.size(ofValue: address.sun_path) else {
            throw LocalProductClientError.invalidSocket
        }
        withUnsafeMutableBytes(of: &address.sun_path) { target in
            target.copyBytes(from: pathBytes.map { UInt8(bitPattern: $0) })
        }
        let connected = withUnsafePointer(to: &address) {
            $0.withMemoryRebound(to: sockaddr.self, capacity: 1) {
                Darwin.connect(
                    descriptor,
                    $0,
                    socklen_t(MemoryLayout<sockaddr_un>.size)
                )
            }
        }
        guard connected == 0 else {
            throw errno == EAGAIN || errno == ETIMEDOUT
                ? LocalProductClientError.timeout
                : LocalProductClientError.unavailable
        }
        try request.withUnsafeBytes { bytes in
            var sent = 0
            while sent < bytes.count {
                let count = Darwin.send(
                    descriptor,
                    bytes.baseAddress!.advanced(by: sent),
                    bytes.count - sent,
                    0
                )
                guard count > 0 else {
                    throw LocalProductClientError.unavailable
                }
                sent += count
            }
        }
        guard Darwin.shutdown(descriptor, SHUT_WR) == 0 else {
            throw LocalProductClientError.unavailable
        }

        var received = Data()
        var buffer = [UInt8](repeating: 0, count: 8_192)
        while true {
            let count = Darwin.recv(descriptor, &buffer, buffer.count, 0)
            if count == 0 { break }
            guard count > 0 else {
                throw errno == EAGAIN || errno == ETIMEDOUT
                    ? LocalProductClientError.timeout
                    : LocalProductClientError.unavailable
            }
            received.append(buffer, count: count)
            guard received.count <= responseMaximum + 4 else {
                throw LocalProductClientError.invalidResponse
            }
        }
        return try LocalIPCWire.unframe(
            received,
            maximum: responseMaximum
        )
    }

    static func validateSocketPath(_ path: String) -> Bool {
        validateSocketConfiguration(path, requireSocket: true)
    }

    private static func validateSocketConfiguration(
        _ path: String,
        requireSocket: Bool = false
    ) -> Bool {
        guard path.hasPrefix("/"), !path.hasSuffix("/"),
              !path.contains("//"),
              !path.split(separator: "/", omittingEmptySubsequences: true)
                .contains(where: { $0 == "." || $0 == ".." }),
              path.utf8.count <= 96,
              URL(fileURLWithPath: path).lastPathComponent == "loomd.sock" else {
            return false
        }
        let parent = URL(fileURLWithPath: path).deletingLastPathComponent().path
        var parentStat = stat()
        guard lstat(parent, &parentStat) == 0,
              (parentStat.st_mode & S_IFMT) == S_IFDIR,
              (parentStat.st_mode & 0o777) == 0o700,
              parentStat.st_uid == geteuid() else {
            return false
        }
        guard let resolvedPointer = realpath(parent, nil) else {
            return false
        }
        let resolved = String(cString: resolvedPointer)
        free(resolvedPointer)
        guard resolved == parent else { return false }

        var socketStat = stat()
        if lstat(path, &socketStat) != 0 {
            return !requireSocket && errno == ENOENT
        }
        return (socketStat.st_mode & S_IFMT) == S_IFSOCK &&
            (socketStat.st_mode & 0o777) == 0o600 &&
            socketStat.st_uid == geteuid()
    }

    static func validIdentifier(_ value: String) -> Bool {
        guard (1...256).contains(value.utf8.count) else { return false }
        return value.unicodeScalars.allSatisfy {
            CharacterSet.alphanumerics.contains($0) ||
                $0 == "." || $0 == "_" || $0 == ":" || $0 == "-"
        }
    }

    public static func validModelID(_ value: String) -> Bool {
        guard (1...256).contains(value.utf8.count),
              value == value.trimmingCharacters(in: .whitespacesAndNewlines) else {
            return false
        }
        return value.unicodeScalars.allSatisfy {
            !CharacterSet.controlCharacters.contains($0)
        }
    }

    public static func validProviderAccountID(
        _ value: String,
        providerID: String
    ) -> Bool {
        guard value.hasPrefix(providerID + "."), value.utf8.count <= 128 else {
            return false
        }
        let suffix = value.dropFirst(providerID.count + 1)
        guard !suffix.isEmpty,
              suffix.first != ".", suffix.last != ".",
              suffix.first != "-", suffix.last != "-" else {
            return false
        }
        var previousSeparator = false
        for scalar in suffix.unicodeScalars {
            let isAlphaNumeric = scalar.value >= 97 && scalar.value <= 122 ||
                scalar.value >= 48 && scalar.value <= 57
            let isSeparator = scalar == "." || scalar == "-"
            if isAlphaNumeric {
                previousSeparator = false
            } else if isSeparator && !previousSeparator {
                previousSeparator = true
            } else {
                return false
            }
        }
        return true
    }

    static func validTimelineCursor(_ value: String) -> Bool {
        if value.isEmpty { return true }
        guard (1...(32 << 10)).contains(value.utf8.count),
              value.unicodeScalars.allSatisfy({ scalar in
                  let code = scalar.value
                  return code >= 65 && code <= 90
                      || code >= 97 && code <= 122
                      || code >= 48 && code <= 57
                      || code == 45 || code == 95
              }) else {
            return false
        }
        var standard = value
            .replacingOccurrences(of: "-", with: "+")
            .replacingOccurrences(of: "_", with: "/")
        let remainder = standard.utf8.count % 4
        guard remainder != 1 else { return false }
        if remainder != 0 {
            standard += String(repeating: "=", count: 4 - remainder)
        }
        guard let decoded = Data(base64Encoded: standard) else { return false }
        let canonical = decoded.base64EncodedString()
            .replacingOccurrences(of: "+", with: "-")
            .replacingOccurrences(of: "/", with: "_")
            .replacingOccurrences(of: "=", with: "")
        return canonical == value
    }

    private func teamStatus(
        method: String,
        definitionID: String,
        expectedHead: Int64
    ) async throws -> LocalProductSetupSavedTeam {
        guard Self.validIdentifier(definitionID), expectedHead > 0 else {
            throw LocalProductClientError.invalidRequest
        }
        let result = try await call(
            method: method,
            params: TeamStatusParams(
                definitionID: definitionID,
                expectedHead: expectedHead
            )
        )
        return try LocalProductSetupWire.decodeSavedTeam(result)
    }

    private func credentialMutation(
        method: String,
        providerID: String,
        providerAccountID: String,
        reference: String,
        revision: Int64,
        operationID: String? = nil,
        incidentID providedIncidentID: String? = nil,
        started providedStart: Date? = nil,
        secret: String
    ) async throws -> LocalProductCredentialSetupResult {
        let started = providedStart ?? Date()
        let incidentID = providedIncidentID ?? requestID()
        guard Self.validIdentifier(providerID),
              Self.validProviderAccountID(providerAccountID, providerID: providerID),
              Self.validIdentifier(reference), revision > 0 else {
            let failure = LocalIPCRemoteError(
                code: .invalidRequest,
                recoverable: false,
                stage: .inputAdmission,
                incidentID: incidentID
            )
            recordCredentialFailure(
                operation: method,
                providerID: providerID,
                providerAccountID: providerAccountID,
                failure: failure,
                incidentID: incidentID,
                started: started
            )
            throw failure
        }
        do {
            let result = try await call(
                method: method,
                params: CredentialParams(
                    providerID: providerID,
                    providerAccountID: providerAccountID,
                    credentialReference: reference,
                    expectedRevision: revision,
                    operationID: operationID,
                    secret: secret
                ),
                incidentID: incidentID
            )
            let decoded = try LocalProductSetupWire.decodeCredentialResult(result)
            recordCredentialResult(
                operation: method,
                providerID: providerID,
                providerAccountID: providerAccountID,
                result: decoded,
                incidentID: incidentID,
                started: started
            )
            return decoded
        } catch {
            let failure = Self.credentialOperationError(error, incidentID: incidentID)
            recordCredentialFailure(
                operation: method,
                providerID: providerID,
                providerAccountID: providerAccountID,
                failure: failure,
                incidentID: incidentID,
                started: started
            )
            throw failure
        }
    }

    private func recordCredentialResult(
        operation: String,
        providerID: String,
        providerAccountID: String,
        result: LocalProductCredentialSetupResult,
        incidentID: String,
        started: Date
    ) {
        guard recordsInstalledDiagnostics else { return }
        if result.status == "rejected" {
            let timeout = result.reason == "timeout"
            try? operationalDiagnostics.recordCredential(
                incidentID: incidentID,
                operation: operation,
                providerID: providerID,
                providerAccountID: providerAccountID,
                stage: timeout ? .providerConnect : .providerAuth,
                result: "failed",
                errorCode: timeout ? .timeout : .credentialRejected,
                retryable: timeout,
                elapsedMilliseconds: Self.elapsedMilliseconds(since: started)
            )
            return
        }
        try? operationalDiagnostics.recordCredential(
            incidentID: incidentID,
            operation: operation,
            providerID: providerID,
            providerAccountID: providerAccountID,
            stage: .projectionRefresh,
            result: "succeeded",
            errorCode: nil,
            retryable: false,
            elapsedMilliseconds: Self.elapsedMilliseconds(since: started)
        )
    }

    private func recordCredentialFailure(
        operation: String,
        providerID: String,
        providerAccountID: String?,
        failure: Error,
        incidentID: String,
        started: Date
    ) {
        guard recordsInstalledDiagnostics else { return }
        guard let remote = failure as? LocalIPCRemoteError else { return }
        try? operationalDiagnostics.recordCredential(
            incidentID: incidentID,
            operation: operation,
            providerID: providerID,
            providerAccountID: providerAccountID,
            stage: remote.stage ?? .daemonAdmission,
            result: "failed",
            errorCode: remote.code,
            retryable: remote.recoverable,
            elapsedMilliseconds: Self.elapsedMilliseconds(since: started)
        )
    }

    private func recordConversationResult(
        threadID: String,
        profileID: String,
        incidentID: String,
        started: Date
    ) {
        guard recordsInstalledDiagnostics else { return }
        try? operationalDiagnostics.recordConversation(
            incidentID: incidentID,
            threadID: threadID,
            profileID: profileID,
            stage: .conversationDispatch,
            result: "succeeded",
            errorCode: nil,
            retryable: false,
            elapsedMilliseconds: Self.elapsedMilliseconds(since: started)
        )
    }

	private func recordAgentInput(
		request: LocalProductAgentInputRequest,
		incidentID: String,
		started: Date,
		result: String,
		failure: LocalIPCRemoteError?
	) {
		guard recordsInstalledDiagnostics else { return }
		try? operationalDiagnostics.recordAgentInput(
			incidentID: incidentID,
			request: request,
			stage: failure?.stage ?? .agentInputAdmission,
			result: result,
			errorCode: failure?.code,
			retryable: failure?.recoverable ?? false,
			elapsedMilliseconds: Self.elapsedMilliseconds(since: started)
		)
	}

    private func recordAgentRecovery(
        request: LocalProductAgentRecoveryRequest,
        incidentID: String,
        started: Date,
        result: String,
        failure: LocalIPCRemoteError?
    ) {
        guard recordsInstalledDiagnostics,
              LocalIPCWire.validRequestID(incidentID), request.isValid else { return }
        try? operationalDiagnostics.recordAgentRecovery(
            incidentID: incidentID,
            operation: request.operation,
            stage: failure?.stage ?? .agentAttemptReconcile,
            result: result,
            errorCode: failure?.code,
            retryable: failure?.recoverable ?? false,
            elapsedMilliseconds: Self.elapsedMilliseconds(since: started)
        )
    }

    private func recordToolRecovery(
        request: LocalProductToolRecoveryRequest,
        incidentID: String,
        started: Date,
        result: String,
        failure: LocalIPCRemoteError?
    ) {
        guard recordsInstalledDiagnostics,
              LocalIPCWire.validRequestID(incidentID), request.isValid else { return }
        try? operationalDiagnostics.recordToolRecovery(
            incidentID: incidentID,
            operation: request.operation,
            stage: failure?.stage ?? .toolRecovery,
            result: result,
            errorCode: failure?.code,
            retryable: failure?.recoverable ?? false,
            elapsedMilliseconds: Self.elapsedMilliseconds(since: started)
        )
    }

    private static func agentRecoveryOperationError(
        _ error: Error,
        incidentID: String
    ) -> LocalIPCRemoteError {
        if let remote = error as? LocalIPCRemoteError { return remote }
        guard let local = error as? LocalProductClientError else {
            return LocalIPCRemoteError(
                code: .internal, recoverable: true,
                stage: .udsTransport, incidentID: incidentID
            )
        }
        switch local {
        case .invalidRequest:
            return LocalIPCRemoteError(
                code: .invalidRequest, recoverable: false,
                stage: .inputAdmission, incidentID: incidentID
            )
        case .timeout:
            return LocalIPCRemoteError(
                code: .timeout, recoverable: true,
                stage: .udsTransport, incidentID: incidentID
            )
        case .invalidResponse:
            return LocalIPCRemoteError(
                code: .invalidResponse, recoverable: true,
                stage: .udsTransport, incidentID: incidentID
            )
        case .invalidSocket, .unavailable, .notFound:
            return LocalIPCRemoteError(
                code: .stateUnavailable, recoverable: true,
                stage: .udsTransport, incidentID: incidentID
            )
        }
    }

    private func recordConversationFailure(
        threadID: String,
        profileID: String,
        failure: Error,
        incidentID: String,
        started: Date
    ) {
        guard recordsInstalledDiagnostics,
              let remote = failure as? LocalIPCRemoteError else {
            return
        }
        try? operationalDiagnostics.recordConversation(
            incidentID: incidentID,
            threadID: threadID,
            profileID: profileID,
            stage: remote.stage ?? .conversationDispatch,
            result: "failed",
            errorCode: remote.code,
			httpStatus: remote.httpStatus,
			providerErrorCode: remote.providerCode,
			retryAfterSeconds: remote.retryAfterSeconds,
            retryable: remote.recoverable,
            elapsedMilliseconds: Self.elapsedMilliseconds(since: started)
        )
    }

    private static func elapsedMilliseconds(since started: Date) -> Int64 {
        max(Int64(Date().timeIntervalSince(started) * 1_000), 0)
    }

    private static func isInstalledSocketPath(_ path: String) -> Bool {
        let expected = FileManager.default.homeDirectoryForCurrentUser
            .resolvingSymlinksInPath()
            .standardizedFileURL
            .appendingPathComponent("Library/Application Support/Loom/run/loomd.sock")
            .path
        return URL(fileURLWithPath: path).standardizedFileURL.path == expected
    }

    private static func credentialOperationError(
        _ error: Error,
        incidentID: String
    ) -> Error {
        if let remote = error as? LocalIPCRemoteError { return remote }
        guard let local = error as? LocalProductClientError else {
            return LocalIPCRemoteError(
                code: .internal,
                recoverable: true,
                stage: .udsTransport,
                incidentID: incidentID
            )
        }
        switch local {
        case .invalidRequest:
            return LocalIPCRemoteError(
                code: .invalidRequest,
                recoverable: false,
                stage: .inputAdmission,
                incidentID: incidentID
            )
        case .timeout:
            return LocalIPCRemoteError(
                code: .timeout,
                recoverable: true,
                stage: .udsTransport,
                incidentID: incidentID
            )
        case .invalidSocket, .invalidResponse, .unavailable, .notFound:
            return LocalIPCRemoteError(
                code: .stateUnavailable,
                recoverable: true,
                stage: .udsTransport,
                incidentID: incidentID
            )
        }
    }

    private static func conversationOperationError(
        _ error: Error,
        incidentID: String
    ) -> Error {
        if let remote = error as? LocalIPCRemoteError { return remote }
        guard let local = error as? LocalProductClientError else {
            return LocalIPCRemoteError(
                code: .internal,
                recoverable: true,
                stage: .udsTransport,
                incidentID: incidentID
            )
        }
        switch local {
        case .invalidRequest:
            return LocalIPCRemoteError(
                code: .invalidRequest,
                recoverable: false,
                stage: .inputAdmission,
                incidentID: incidentID
            )
        case .timeout:
            return LocalIPCRemoteError(
                code: .timeout,
                recoverable: true,
                stage: .udsTransport,
                incidentID: incidentID
            )
        case .invalidSocket, .invalidResponse, .unavailable, .notFound:
            return LocalIPCRemoteError(
                code: .stateUnavailable,
                recoverable: true,
                stage: .udsTransport,
                incidentID: incidentID
            )
        }
    }

    private static func validDigest(_ value: String) -> Bool {
        value.utf8.count == 64 && value.utf8.allSatisfy {
            ($0 >= 0x30 && $0 <= 0x39) || ($0 >= 0x61 && $0 <= 0x66)
        }
    }

    private static func validBuilderText(_ value: String) -> Bool {
        guard (1...2_048).contains(value.utf8.count),
              value == value.trimmingCharacters(in: .whitespacesAndNewlines)
        else {
            return false
        }
        return value.unicodeScalars.allSatisfy {
            !CharacterSet.controlCharacters.contains($0)
        }
    }

    static func normalizedCredentialSecret(_ value: String) -> String? {
        let bytes = Array(value.utf8)
        var lower = bytes.startIndex
        var upper = bytes.endIndex
        while lower < upper && credentialEdgeWhitespace(bytes[lower]) {
            lower += 1
        }
        while upper > lower && credentialEdgeWhitespace(bytes[upper - 1]) {
            upper -= 1
        }
        guard let normalized = String(
            data: Data(bytes[lower..<upper]),
            encoding: .utf8
        ), validSecret(normalized) else {
            return nil
        }
        return normalized
    }

    private static func credentialEdgeWhitespace(_ byte: UInt8) -> Bool {
        byte == 0x20 || byte == 0x09 || byte == 0x0a || byte == 0x0d
    }

    private static func validSecret(_ value: String) -> Bool {
        (1...8_192).contains(value.utf8.count) &&
            !value.unicodeScalars.contains(where: {
                CharacterSet.controlCharacters.contains($0)
            })
    }
}

private struct EmptyParams: Encodable {}

private struct PingResult: Decodable {
    let protocolVersion: Int
    let available: Bool
    let buildID: String

    enum CodingKeys: String, CodingKey {
        case protocolVersion = "protocol_version"
        case available
        case buildID = "build_id"
    }

    init(from decoder: Decoder) throws {
        try rejectIPCUnknownKeys(
            decoder,
            allowed: ["protocol_version", "available", "build_id"]
        )
        let values = try decoder.container(keyedBy: CodingKeys.self)
        protocolVersion = try values.decode(Int.self, forKey: .protocolVersion)
        available = try values.decode(Bool.self, forKey: .available)
        buildID = try values.decode(String.self, forKey: .buildID)
    }
}
