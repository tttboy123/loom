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
        case cursorConflict = "cursor_conflict"
        case streamGap = "stream_gap"
        case stateUnavailable = "state_unavailable"
        case timeout
        case busy
        case `internal`
    }

    public let code: Code
    public let recoverable: Bool
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

    enum CodingKeys: String, CodingKey {
        case code, message, recoverable
    }

    init(from decoder: Decoder) throws {
        try rejectIPCUnknownKeys(
            decoder,
            allowed: ["code", "message", "recoverable"]
        )
        let values = try decoder.container(keyedBy: CodingKeys.self)
        code = try values.decode(LocalIPCRemoteError.Code.self, forKey: .code)
        message = try values.decode(String.self, forKey: .message)
        recoverable = try values.decodeIfPresent(
            Bool.self,
            forKey: .recoverable
        ) ?? false
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
                recoverable: error.recoverable
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

    enum CodingKeys: String, CodingKey {
        case draftID = "draft_id"
        case expectedRevision = "expected_revision"
        case catalogDigest = "catalog_digest"
        case viewVersion = "view_version"
        case field, value
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
    let credentialReference: String
    let expectedRevision: Int64
    let operationID: String?
    let secret: String

    enum CodingKeys: String, CodingKey {
        case providerID = "provider_id"
        case credentialReference = "credential_reference"
        case expectedRevision = "expected_revision"
        case operationID = "operation_id"
        case secret
    }
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
    LocalProductHandoffClientProtocol,
    LocalProductAssetClientProtocol
{
    public static let requestMaximum = 65_536
    public static let responseMaximum = 524_288
    private let socketPath: String
    private let requestID: @Sendable () -> String

    public init(
        socketPath: String,
        requestID: @escaping @Sendable () -> String = {
            "loom-swift-\(UUID().uuidString.lowercased())"
        }
    ) throws {
        guard Self.validateSocketConfiguration(socketPath) else {
            throw LocalProductClientError.invalidSocket
        }
        self.socketPath = socketPath
        self.requestID = requestID
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
        guard ["team_name", "purpose", "main_role", "subagent_role"]
            .contains(field),
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
                value: value
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
        guard Self.validSecret(secret) else {
            throw LocalProductClientError.invalidRequest
        }
        let result = try await call(
            method: "credential_configure",
            params: CredentialParams(
                providerID: "minimax",
                credentialReference: "",
                expectedRevision: 0,
                operationID: nil,
                secret: secret
            )
        )
        return try LocalProductSetupWire.decodeCredentialResult(result)
    }

    public func verifyMiniMax(
        reference: String,
        revision: Int64
    ) async throws -> LocalProductCredentialSetupResult {
        let operationID = UUID().uuidString.lowercased()
        return try await credentialMutation(
            method: "credential_verify",
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
        guard Self.validSecret(secret) else {
            throw LocalProductClientError.invalidRequest
        }
        return try await credentialMutation(
            method: "credential_replace",
            reference: reference,
            revision: revision,
            secret: secret
        )
    }

    public func revokeMiniMax(
        reference: String,
        revision: Int64
    ) async throws -> LocalProductCredentialSetupResult {
        try await credentialMutation(
            method: "credential_revoke",
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

    func call<Params: Encodable>(
        method: String,
        params: Params
    ) async throws -> Data {
        let id = requestID()
        let methods = Set([
            "ping", "snapshot", "timeline_page", "setup_snapshot",
            "codex_connect",
            "builder_start", "builder_answer", "builder_edit",
            "builder_validate", "builder_confirm", "team_archive",
            "team_restore", "credential_configure", "credential_verify",
            "credential_replace", "credential_revoke", "mission_decision",
            "mission_execution",
            "side_task_handoff",
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
        method == "credential_verify" || method == "mission_execution" ? 10 : 5
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
        guard (1...10).contains(timeoutSeconds) else {
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
        reference: String,
        revision: Int64,
        operationID: String? = nil,
        secret: String
    ) async throws -> LocalProductCredentialSetupResult {
        guard Self.validIdentifier(reference), revision > 0 else {
            throw LocalProductClientError.invalidRequest
        }
        let result = try await call(
            method: method,
            params: CredentialParams(
                providerID: "minimax",
                credentialReference: reference,
                expectedRevision: revision,
                operationID: operationID,
                secret: secret
            )
        )
        return try LocalProductSetupWire.decodeCredentialResult(result)
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
