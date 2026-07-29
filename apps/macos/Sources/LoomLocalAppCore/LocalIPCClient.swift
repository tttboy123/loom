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
    let ok: Bool
    let result: JSONValue?
    let error: ErrorEnvelope?

    enum CodingKeys: String, CodingKey {
        case version
        case requestID = "request_id"
        case ok, result, error
    }

    init(from decoder: Decoder) throws {
        try rejectIPCUnknownKeys(
            decoder,
            allowed: ["version", "request_id", "ok", "result", "error"]
        )
        let values = try decoder.container(keyedBy: CodingKeys.self)
        version = try values.decode(Int.self, forKey: .version)
        requestID = try values.decode(String.self, forKey: .requestID)
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
        expectedRequestID: String
    ) throws -> Data {
        do {
            try StrictJSONScanner.validate(data)
            let envelope = try JSONDecoder().decode(ResponseEnvelope.self, from: data)
            guard envelope.version == 1,
                  envelope.requestID == expectedRequestID,
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

private struct IPCRequest<Params: Encodable>: Encodable {
    let version = 1
    let requestID: String
    let method: String
    let params: Params

    enum CodingKeys: String, CodingKey {
        case version
        case requestID = "request_id"
        case method, params
    }
}

public final class LocalIPCClient: LocalProductClientProtocol {
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
              cursor.isEmpty || Self.validIdentifier(cursor),
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

    public func ping() async throws -> Bool {
        let result = try await call(method: "ping", params: EmptyParams())
        try StrictJSONScanner.validate(result)
        let decoded = try JSONDecoder().decode(PingResult.self, from: result)
        return decoded.protocolVersion == 1 && decoded.available &&
            !decoded.buildID.isEmpty
    }

    private func call<Params: Encodable>(
        method: String,
        params: Params
    ) async throws -> Data {
        let id = requestID()
        guard LocalIPCWire.validRequestID(id),
              ["ping", "snapshot", "timeline_page"].contains(method) else {
            throw LocalProductClientError.invalidRequest
        }
        let request = IPCRequest(
            requestID: id,
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
                request: framed
            )
        }.value
        return try LocalIPCWire.decodeResponse(
            response,
            expectedRequestID: id
        )
    }

    private static func exchange(path: String, request: Data) throws -> Data {
        guard validateSocketPath(path) else {
            throw LocalProductClientError.invalidSocket
        }
        let descriptor = socket(AF_UNIX, SOCK_STREAM, 0)
        guard descriptor >= 0 else {
            throw LocalProductClientError.unavailable
        }
        defer { Darwin.close(descriptor) }

        var timeout = timeval(tv_sec: 5, tv_usec: 0)
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
