import CryptoKit
import Darwin
import Foundation

public struct LocalDiagnosticProviderState: Codable, Equatable, Sendable {
    public let providerID: String
    public let authMode: String
    public let revision: Int64
    public let status: String
    public let reason: String

    public init(
        providerID: String,
        authMode: String,
        revision: Int64,
        status: String,
        reason: String
    ) {
        self.providerID = providerID
        self.authMode = authMode
        self.revision = revision
        self.status = status
        self.reason = reason
    }

    enum CodingKeys: String, CodingKey {
        case providerID = "provider_id"
        case authMode = "auth_mode"
        case revision, status, reason
    }
}

public struct LocalDiagnosticProfileState: Codable, Equatable, Sendable {
    public let profileID: String
    public let providerID: String
    public let modelID: String
    public let authMode: String
    public let credentialRevision: Int64

    public init(
        profileID: String,
        providerID: String,
        modelID: String,
        authMode: String,
        credentialRevision: Int64
    ) {
        self.profileID = profileID
        self.providerID = providerID
        self.modelID = modelID
        self.authMode = authMode
        self.credentialRevision = credentialRevision
    }

    enum CodingKeys: String, CodingKey {
        case profileID = "profile_id"
        case providerID = "provider_id"
        case modelID = "model_id"
        case authMode = "auth_mode"
        case credentialRevision = "credential_revision"
    }
}

public struct LocalDiagnosticAgentState: Codable, Equatable, Sendable {
    public let agentInstanceID: String
    public let status: String
    public let harnessAdapter: String
    public let providerID: String
    public let providerAccountID: String
    public let modelID: String
    public let reasoningEffort: String
    public let credentialRevision: Int64
    public let terminalReason: String

    public init(
        agentInstanceID: String,
        status: String,
        harnessAdapter: String,
        providerID: String,
        providerAccountID: String,
        modelID: String,
        reasoningEffort: String = "",
        credentialRevision: Int64,
        terminalReason: String
    ) {
        self.agentInstanceID = agentInstanceID
        self.status = status
        self.harnessAdapter = harnessAdapter
        self.providerID = providerID
        self.providerAccountID = providerAccountID
        self.modelID = modelID
        self.reasoningEffort = reasoningEffort
        self.credentialRevision = credentialRevision
        self.terminalReason = terminalReason
    }

    enum CodingKeys: String, CodingKey {
        case agentInstanceID = "agent_instance_id"
        case status
        case harnessAdapter = "harness_adapter"
        case providerID = "provider_id"
        case providerAccountID = "provider_account_id"
        case modelID = "model_id"
        case reasoningEffort = "reasoning_effort"
        case credentialRevision = "credential_revision"
        case terminalReason = "terminal_reason"
    }
}

public struct LocalDiagnosticBundleInput: Equatable, Sendable {
    public let providers: [LocalDiagnosticProviderState]
    public let profiles: [LocalDiagnosticProfileState]
    public let agents: [LocalDiagnosticAgentState]

    public init(
        providers: [LocalDiagnosticProviderState] = [],
        profiles: [LocalDiagnosticProfileState] = [],
        agents: [LocalDiagnosticAgentState] = []
    ) {
        self.providers = providers
        self.profiles = profiles
        self.agents = agents
    }

    public init(
        setupSnapshot: LocalProductSetupSnapshot?,
        board: LocalProductBoard?
    ) {
        providers = setupSnapshot?.providers.map {
            LocalDiagnosticProviderState(
                providerID: $0.providerID,
                authMode: $0.authMode,
                revision: $0.revision,
                status: $0.status,
                reason: $0.reason
            )
        } ?? []
        profiles = setupSnapshot?.conversationProfiles.map {
            LocalDiagnosticProfileState(
                profileID: $0.profileID,
                providerID: $0.providerID,
                modelID: $0.modelID,
                authMode: $0.authMode,
                credentialRevision: $0.credentialRevision
            )
        } ?? []
        agents = board?.nodes.map {
            LocalDiagnosticAgentState(
                agentInstanceID: $0.agentInstanceID,
                status: $0.status,
                harnessAdapter: $0.harnessAdapter,
                providerID: $0.providerID,
                providerAccountID: $0.providerAccountID,
                modelID: $0.modelID,
                reasoningEffort: $0.reasoningEffort,
                credentialRevision: $0.credentialRevision,
                terminalReason: $0.terminalReason
            )
        } ?? []
    }
}

public struct LocalDiagnosticBinarySummary: Codable, Equatable, Sendable {
    public let version: String
    public let build: String
    public let sha256: String
}

public struct LocalDiagnosticConsoleSummary: Codable, Equatable, Sendable {
    public let available: Bool
    public let byteCount: Int64

    enum CodingKeys: String, CodingKey {
        case available
        case byteCount = "byte_count"
    }
}

public struct LocalDiagnosticEvent: Codable, Equatable, Sendable {
    public let source: String
    public let occurredAt: String
    public let incidentID: String
    public let operation: String
    public let credentialRuntime: String?
    public let credentialHelperSpawnAttempts: UInt64?
    public let providerID: String
    public let providerAccountID: String
    public let modelID: String
    public let threadID: String
    public let profileID: String
    public let stage: String
    public let elapsedMilliseconds: Int64
    public let result: String
    public let errorCode: String?
    public let retryable: Bool

    enum CodingKeys: String, CodingKey {
        case source
        case occurredAt = "occurred_at"
        case incidentID = "incident_id"
        case operation
        case credentialRuntime = "credential_runtime"
        case credentialHelperSpawnAttempts = "credential_helper_spawn_attempts"
        case providerID = "provider_id"
        case providerAccountID = "provider_account_id"
        case modelID = "model_id"
        case threadID = "thread_id"
        case profileID = "profile_id"
        case stage
        case elapsedMilliseconds = "elapsed_ms"
        case result
        case errorCode = "error_code"
        case retryable
    }
}

public struct LocalDiagnosticBundlePreview:
    Codable, Equatable, Sendable, Identifiable
{
    public var id: String { generatedAt }
    public let schemaVersion: Int
    public let generatedAt: String
    public let app: LocalDiagnosticBinarySummary
    public let daemon: LocalDiagnosticBinarySummary
    public let socketHealthy: Bool
    public let console: LocalDiagnosticConsoleSummary
    public let providers: [LocalDiagnosticProviderState]
    public let profiles: [LocalDiagnosticProfileState]
    public let agents: [LocalDiagnosticAgentState]
    public let events: [LocalDiagnosticEvent]
    public let diagnosticFilesSkipped: Int
    public let excluded: [String]

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case generatedAt = "generated_at"
        case app, daemon
        case socketHealthy = "socket_healthy"
        case console, providers, profiles, agents, events
        case diagnosticFilesSkipped = "diagnostic_files_skipped"
        case excluded
    }
}

public final class LocalDiagnosticBundleExporter: @unchecked Sendable {
    private struct StoredEvent: Decodable {
        let schemaVersion: Int
        let occurredAt: String
        let incidentID: String
        let operation: String
        let credentialRuntime: String?
        let credentialHelperSpawnAttempts: UInt64?
        let providerID: String?
        let providerAccountID: String?
        let modelID: String?
        let threadID: String?
        let profileID: String?
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
            case credentialRuntime = "credential_runtime"
            case credentialHelperSpawnAttempts = "credential_helper_spawn_attempts"
            case providerID = "provider_id"
            case providerAccountID = "provider_account_id"
            case modelID = "model_id"
            case threadID = "thread_id"
            case profileID = "profile_id"
            case stage
            case elapsedMilliseconds = "elapsed_ms"
            case result
            case errorCode = "error_code"
            case retryable
        }
    }

    private let diagnosticsDirectory: URL
    private let appExecutable: URL
    private let daemonExecutable: URL
    private let appVersion: String
    private let appBuild: String
    private let socketHealthy: @Sendable () -> Bool
    private let now: @Sendable () -> Date

    public static func installed() throws -> LocalDiagnosticBundleExporter {
        guard let appExecutable = Bundle.main.executableURL else {
            throw LocalProductClientError.unavailable
        }
        let daemon = Bundle.main.bundleURL
            .appendingPathComponent("Contents/Library/Helpers/loomd")
        let socket = FileManager.default.homeDirectoryForCurrentUser
            .appendingPathComponent("Library/Application Support/Loom/run/loomd.sock")
        return try LocalDiagnosticBundleExporter(
            diagnosticsDirectory: FileManager.default.homeDirectoryForCurrentUser
                .appendingPathComponent("Library/Application Support/Loom/diagnostics"),
            appExecutable: appExecutable,
            daemonExecutable: daemon,
            appVersion: Bundle.main.object(
                forInfoDictionaryKey: "CFBundleShortVersionString"
            ) as? String ?? "unknown",
            appBuild: Bundle.main.object(
                forInfoDictionaryKey: "CFBundleVersion"
            ) as? String ?? "unknown",
            socketHealthy: { Self.healthySocket(at: socket.path) },
            now: { Date() }
        )
    }

    public init(
        diagnosticsDirectory: URL,
        appExecutable: URL,
        daemonExecutable: URL,
        appVersion: String,
        appBuild: String,
        socketHealthy: @escaping @Sendable () -> Bool,
        now: @escaping @Sendable () -> Date
    ) throws {
        guard diagnosticsDirectory.isFileURL,
              diagnosticsDirectory.path.hasPrefix("/"),
              appExecutable.isFileURL,
              daemonExecutable.isFileURL,
              Self.safeToken(appVersion, maximum: 64),
              Self.safeToken(appBuild, maximum: 64) else {
            throw LocalProductClientError.invalidRequest
        }
        self.diagnosticsDirectory = diagnosticsDirectory
        self.appExecutable = appExecutable
        self.daemonExecutable = daemonExecutable
        self.appVersion = appVersion
        self.appBuild = appBuild
        self.socketHealthy = socketHealthy
        self.now = now
    }

    public func preview(
        input: LocalDiagnosticBundleInput
    ) throws -> LocalDiagnosticBundlePreview {
        let eventResult = readEvents()
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return LocalDiagnosticBundlePreview(
            schemaVersion: 1,
            generatedAt: formatter.string(from: now()),
            app: .init(
                version: appVersion,
                build: appBuild,
                sha256: try Self.fileSHA256(appExecutable)
            ),
            daemon: .init(
                version: appVersion,
                build: appBuild,
                sha256: try Self.fileSHA256(daemonExecutable)
            ),
            socketHealthy: socketHealthy(),
            console: consoleSummary(),
            providers: input.providers.filter(Self.validProvider),
            profiles: input.profiles.filter(Self.validProfile),
            agents: input.agents.filter(Self.validAgent),
            events: eventResult.events,
            diagnosticFilesSkipped: eventResult.skipped,
            excluded: [
                "api_keys_and_credentials",
                "authorization_headers",
                "environment_credentials",
                "prompts_and_conversations",
                "provider_response_bodies",
                "raw_daemon_console",
            ]
        )
    }

    public func export(
        _ preview: LocalDiagnosticBundlePreview,
        to destination: URL
    ) throws {
        guard destination.isFileURL, destination.path.hasPrefix("/") else {
            throw LocalProductClientError.invalidRequest
        }
        let parent = destination.deletingLastPathComponent()
        let values = try parent.resourceValues(
            forKeys: [.isDirectoryKey, .isSymbolicLinkKey]
        )
        guard values.isDirectory == true, values.isSymbolicLink != true else {
            throw LocalProductClientError.unavailable
        }
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys, .withoutEscapingSlashes]
        var data = try encoder.encode(preview)
        data.append(0x0A)
        let descriptor = destination.path.withCString {
            Darwin.open(
                $0,
                O_WRONLY | O_CREAT | O_TRUNC | O_NOFOLLOW,
                S_IRUSR | S_IWUSR
            )
        }
        guard descriptor >= 0 else { throw LocalProductClientError.unavailable }
        defer { Darwin.close(descriptor) }
        var status = stat()
        guard fstat(descriptor, &status) == 0,
              status.st_uid == geteuid(),
              status.st_mode & S_IFMT == S_IFREG,
              status.st_nlink == 1,
              fchmod(descriptor, S_IRUSR | S_IWUSR) == 0 else {
            throw LocalProductClientError.unavailable
        }
        try data.withUnsafeBytes { bytes in
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
        guard fsync(descriptor) == 0 else {
            throw LocalProductClientError.unavailable
        }
    }

    private func readEvents() -> (events: [LocalDiagnosticEvent], skipped: Int) {
        let sources = [
            ("app", "app-operational.jsonl.1"),
            ("app", "app-operational.jsonl"),
            ("daemon", "operational.jsonl.1"),
            ("daemon", "operational.jsonl"),
        ]
        var events: [LocalDiagnosticEvent] = []
        var skipped = 0
        for (source, name) in sources {
            let path = diagnosticsDirectory.appendingPathComponent(name).path
            var status = stat()
            if lstat(path, &status) != 0 {
                if errno != ENOENT { skipped += 1 }
                continue
            }
            guard let data = Self.readPrivateDiagnostic(path) else {
                skipped += 1
                continue
            }
            for line in data.split(separator: 0x0A) where !line.isEmpty {
                guard line.count <= 4_096,
                      let stored = try? JSONDecoder().decode(StoredEvent.self, from: line),
                      let event = Self.safeEvent(stored, source: source) else {
                    skipped += 1
                    continue
                }
                events.append(event)
            }
        }
        events.sort {
            ($0.occurredAt, $0.incidentID, $0.source) <
                ($1.occurredAt, $1.incidentID, $1.source)
        }
        return (Array(events.suffix(100)), skipped)
    }

    private func consoleSummary() -> LocalDiagnosticConsoleSummary {
        let path = diagnosticsDirectory.appendingPathComponent("loomd-console.log").path
        var status = stat()
        guard lstat(path, &status) == 0,
              status.st_uid == geteuid(),
              status.st_mode & S_IFMT == S_IFREG,
              status.st_mode & 0o777 == 0o600,
              status.st_size >= 0 else {
            return .init(available: false, byteCount: 0)
        }
        return .init(available: true, byteCount: status.st_size)
    }

    private static func safeEvent(
        _ stored: StoredEvent,
        source: String
    ) -> LocalDiagnosticEvent? {
        let providerID = stored.providerID ?? ""
        let credentialRuntime = stored.credentialRuntime ?? ""
        let providerAccountID = stored.providerAccountID ?? ""
        let modelID = stored.modelID ?? ""
        let threadID = stored.threadID ?? ""
        let profileID = stored.profileID ?? ""
        guard stored.schemaVersion == 1,
              ["app", "daemon"].contains(source),
              safeTimestamp(stored.occurredAt),
              validIncidentID(stored.incidentID),
              safeToken(stored.operation, maximum: 64),
              credentialRuntime.isEmpty ||
                ["vault", "explicit_legacy"].contains(credentialRuntime),
              providerID.isEmpty || safeToken(providerID, maximum: 64),
              providerAccountID.isEmpty || safeToken(providerAccountID, maximum: 128),
              modelID.isEmpty || safeToken(modelID, maximum: 256),
              threadID.isEmpty || safeToken(threadID, maximum: 256),
              profileID.isEmpty || safeToken(profileID, maximum: 256),
              safeStage(stored.stage),
              stored.elapsedMilliseconds >= 0,
              stored.elapsedMilliseconds <= 86_400_000,
              ["succeeded", "failed"].contains(stored.result),
              stored.errorCode == nil || safeToken(stored.errorCode!, maximum: 64),
              (stored.result == "succeeded" && stored.errorCode == nil && !stored.retryable ||
                stored.result == "failed" && stored.errorCode != nil) else {
            return nil
        }
        return LocalDiagnosticEvent(
            source: source,
            occurredAt: stored.occurredAt,
            incidentID: stored.incidentID,
            operation: stored.operation,
            credentialRuntime: credentialRuntime.isEmpty ? nil : credentialRuntime,
            credentialHelperSpawnAttempts: stored.credentialHelperSpawnAttempts,
            providerID: providerID,
            providerAccountID: providerAccountID,
            modelID: modelID,
            threadID: threadID,
            profileID: profileID,
            stage: stored.stage,
            elapsedMilliseconds: stored.elapsedMilliseconds,
            result: stored.result,
            errorCode: stored.errorCode,
            retryable: stored.retryable
        )
    }

    private static func validProvider(_ state: LocalDiagnosticProviderState) -> Bool {
        safeToken(state.providerID, maximum: 64) &&
            safeToken(state.authMode, maximum: 64) &&
            state.revision >= 0 &&
            safeToken(state.status, maximum: 64) &&
            (state.reason.isEmpty || safeToken(state.reason, maximum: 64))
    }

    private static func validProfile(_ state: LocalDiagnosticProfileState) -> Bool {
        safeToken(state.profileID, maximum: 128) &&
            safeToken(state.providerID, maximum: 64) &&
            safeToken(state.modelID, maximum: 128) &&
            safeToken(state.authMode, maximum: 64) &&
            state.credentialRevision >= 0
    }

    private static func validAgent(_ state: LocalDiagnosticAgentState) -> Bool {
        safeToken(state.agentInstanceID, maximum: 128) &&
            safeToken(state.status, maximum: 64) &&
            (state.harnessAdapter.isEmpty || safeToken(state.harnessAdapter, maximum: 64)) &&
            (state.providerID.isEmpty || safeToken(state.providerID, maximum: 64)) &&
            (state.providerAccountID.isEmpty || safeToken(state.providerAccountID, maximum: 128)) &&
            (state.modelID.isEmpty || safeToken(state.modelID, maximum: 128)) &&
            (state.reasoningEffort.isEmpty || safeToken(state.reasoningEffort, maximum: 32)) &&
            state.credentialRevision >= 0 &&
            (state.terminalReason.isEmpty || safeToken(state.terminalReason, maximum: 64))
    }

    private static func safeStage(_ value: String) -> Bool {
        [
            "input_admission", "uds_transport", "daemon_admission",
            "helper_validation", "helper_start", "helper_authorization",
            "helper_request", "helper_timeout", "helper_response", "helper_exit",
            "keychain_access", "metadata_commit", "projection_refresh",
            "provider_dns", "provider_tls", "provider_connect", "provider_http",
            "provider_auth", "provider_rate_limit", "profile_publish",
            "conversation_dispatch", "agent_attempt_dispatch",
        ].contains(value)
    }

    private static func safeTimestamp(_ value: String) -> Bool {
        value.utf8.count <= 40 &&
            value.allSatisfy { character in
                character.isNumber || "-:.TZ+".contains(character)
            }
    }

    private static func validIncidentID(_ value: String) -> Bool {
        !value.isEmpty && value.utf8.count <= 64 && value.allSatisfy {
            $0.isASCII && ($0.isLetter || $0.isNumber || "._:-".contains($0))
        }
    }

    private static func safeToken(_ value: String, maximum: Int) -> Bool {
        !value.isEmpty && value.utf8.count <= maximum && value.allSatisfy {
            $0.isASCII && ($0.isLetter || $0.isNumber || "._:/@+-".contains($0))
        }
    }

    private static func healthySocket(at path: String) -> Bool {
        var status = stat()
        return lstat(path, &status) == 0 &&
            status.st_uid == geteuid() &&
            status.st_mode & S_IFMT == S_IFSOCK
    }

    private static func fileSHA256(_ url: URL) throws -> String {
        let descriptor = url.path.withCString {
            Darwin.open($0, O_RDONLY | O_NOFOLLOW)
        }
        guard descriptor >= 0 else { throw LocalProductClientError.unavailable }
        defer { Darwin.close(descriptor) }
        var status = stat()
        guard fstat(descriptor, &status) == 0,
              status.st_uid == geteuid(),
              status.st_mode & S_IFMT == S_IFREG,
              status.st_size >= 0,
              status.st_size <= 256 * 1_024 * 1_024 else {
            throw LocalProductClientError.unavailable
        }
        var digest = SHA256()
        var buffer = [UInt8](repeating: 0, count: 64 * 1_024)
        while true {
            let count = Darwin.read(descriptor, &buffer, buffer.count)
            guard count >= 0 else { throw LocalProductClientError.unavailable }
            if count == 0 { break }
            digest.update(data: Data(buffer.prefix(count)))
        }
        return digest.finalize().map { String(format: "%02x", $0) }.joined()
    }

    private static func readPrivateDiagnostic(_ path: String) -> Data? {
        let descriptor = path.withCString {
            Darwin.open($0, O_RDONLY | O_NOFOLLOW)
        }
        guard descriptor >= 0 else { return nil }
        defer { Darwin.close(descriptor) }
        var status = stat()
        guard fstat(descriptor, &status) == 0,
              status.st_uid == geteuid(),
              status.st_mode & S_IFMT == S_IFREG,
              status.st_mode & 0o777 == 0o600,
              status.st_size >= 0,
              status.st_size <= 512 * 1_024 else {
            return nil
        }
        var data = Data()
        data.reserveCapacity(Int(status.st_size))
        var buffer = [UInt8](repeating: 0, count: 16 * 1_024)
        while true {
            let count = Darwin.read(descriptor, &buffer, buffer.count)
            guard count >= 0 else { return nil }
            if count == 0 { break }
            guard data.count + count <= 512 * 1_024 else { return nil }
            data.append(contentsOf: buffer.prefix(count))
        }
        return data
    }
}
