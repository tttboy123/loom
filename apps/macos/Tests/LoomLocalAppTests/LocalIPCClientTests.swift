import Darwin
import Foundation
import XCTest
@testable import LoomLocalAppCore

final class LocalIPCClientTests: XCTestCase {
    func testRealClientAdvertisesDecisionProtocolForNativeStoreComposition() throws {
        let client: LocalProductClientProtocol =
            try Self.makeClientBackedByPrivateSocket()

        XCTAssertNotNil(client as? LocalProductDecisionClientProtocol)
        XCTAssertNotNil(client as? LocalProductExecutionClientProtocol)
        XCTAssertNotNil(client as? LocalProductHandoffClientProtocol)
    }

    func testFrameUsesFourByteBigEndianLength() throws {
        let body = Data("{}".utf8)
        let framed = try LocalIPCWire.frame(body, maximum: 65_536)

        XCTAssertEqual(Array(framed.prefix(4)), [0, 0, 0, 2])
        XCTAssertEqual(framed.dropFirst(4), body)
    }

    func testResponseRequiresMatchingIdentityAndExactShape() throws {
        let valid = """
        {"version":1,"request_id":"request-1","ok":true,"result":{"value":1},"error":null}
        """
        let result = try LocalIPCWire.decodeResponse(
            Data(valid.utf8),
            expectedRequestID: "request-1"
        )
        XCTAssertEqual(String(data: result, encoding: .utf8), "{\"value\":1}")

        let mismatch = valid.replacingOccurrences(
            of: "request-1",
            with: "request-2"
        )
        XCTAssertThrowsError(
            try LocalIPCWire.decodeResponse(
                Data(mismatch.utf8),
                expectedRequestID: "request-1"
            )
        )
    }

    func testResponseRejectsDuplicateUnknownTrailingAndOversizedData() {
        let duplicate = """
        {"version":1,"version":1,"request_id":"request-1","ok":true,"result":{},"error":null}
        """
        let unknown = """
        {"version":1,"request_id":"request-1","ok":true,"result":{},"error":null,"extra":1}
        """
        let trailing = """
        {"version":1,"request_id":"request-1","ok":true,"result":{},"error":null}x
        """

        for body in [duplicate, unknown, trailing] {
            XCTAssertThrowsError(
                try LocalIPCWire.decodeResponse(
                    Data(body.utf8),
                    expectedRequestID: "request-1"
                )
            )
        }
        XCTAssertThrowsError(
            try LocalIPCWire.frame(
                Data(repeating: 0x61, count: 65_537),
                maximum: 65_536
            )
        )
    }

    func testClosedRemoteErrorSetIncludesEveryGoV1Code() {
        let expected = Set([
            "invalid_request",
            "unsupported_version",
            "unknown_method",
            "unauthorized_peer",
            "unsupported_platform",
            "not_found",
            "cursor_conflict",
            "stream_gap",
            "conflict",
			"capability_gap",
			"stale_view",
			"stale_generation",
			"digest_mismatch",
			"human_required",
            "incompatible",
            "denied",
            "credential_unavailable",
            "credential_rejected",
            "credential_rollback_failed",
            "state_unavailable",
            "timeout",
            "busy",
            "internal",
        ])
        XCTAssertEqual(Set(LocalIPCRemoteError.Code.allCases.map(\.rawValue)), expected)
    }

    func testRequestIDGrammarIsStrictASCII() {
        XCTAssertTrue(LocalIPCWire.validRequestID("AZaz09._:-"))
        XCTAssertFalse(LocalIPCWire.validRequestID("réquest-1"))
        XCTAssertFalse(LocalIPCWire.validRequestID("request/1"))
    }

    func testTimelineCursorUsesCanonicalRawURLBoundaryNotBusinessIdentifierLimit() {
        let longCanonical = Data(repeating: 0x61, count: 1_024)
            .base64EncodedString()
            .replacingOccurrences(of: "+", with: "-")
            .replacingOccurrences(of: "/", with: "_")
            .replacingOccurrences(of: "=", with: "")

        XCTAssertGreaterThan(longCanonical.utf8.count, 256)
        XCTAssertTrue(LocalIPCClient.validTimelineCursor(""))
        XCTAssertTrue(LocalIPCClient.validTimelineCursor(longCanonical))
        XCTAssertFalse(LocalIPCClient.validIdentifier(longCanonical))
    }

    func testTimelineCursorRejectsNoncanonicalAndOversizedRawURL() {
        let nonzeroTrailingBits = "AB"
        let invalidModulus = "A"
        let oversized = String(repeating: "A", count: 32_769)
        for invalid in [
            nonzeroTrailingBits,
            invalidModulus,
            "YQ==",
            "Y Q",
            "YQ\n",
            "réel",
            oversized,
        ] {
            XCTAssertFalse(
                LocalIPCClient.validTimelineCursor(invalid),
                invalid.debugDescription
            )
        }
        XCTAssertTrue(LocalIPCClient.validTimelineCursor("YQ"))
    }

    func testCredentialVerifyAloneReceivesExtendedRequestTimeout() {
        XCTAssertEqual(
            LocalIPCClient.requestTimeoutSeconds(for: "credential_verify"),
            10
        )
        XCTAssertEqual(
            LocalIPCClient.requestTimeoutSeconds(for: "mission_execution"),
            10
        )
        for method in [
            "ping", "snapshot", "timeline_page", "setup_snapshot",
            "codex_connect", "builder_start", "builder_answer",
            "builder_edit", "builder_validate", "builder_confirm",
            "team_archive", "team_restore", "credential_configure",
            "credential_replace", "credential_revoke",
        ] {
            XCTAssertEqual(
                LocalIPCClient.requestTimeoutSeconds(for: method),
                5,
                method
            )
        }
    }

    func testClientAcceptsPrivateOwnedUnixSocket() throws {
        let root = URL(fileURLWithPath: "/private/tmp")
            .appendingPathComponent("loom-swift-\(UUID().uuidString.prefix(8))")
        try FileManager.default.createDirectory(
            at: root,
            withIntermediateDirectories: false,
            attributes: [.posixPermissions: 0o700]
        )
        defer { try? FileManager.default.removeItem(at: root) }
        let path = root.appendingPathComponent("loomd.sock").path
        let descriptor = socket(AF_UNIX, SOCK_STREAM, 0)
        XCTAssertGreaterThanOrEqual(descriptor, 0)
        defer { Darwin.close(descriptor) }
        var address = sockaddr_un()
        address.sun_family = sa_family_t(AF_UNIX)
        let bytes = Array(path.utf8CString)
        withUnsafeMutableBytes(of: &address.sun_path) {
            $0.copyBytes(from: bytes.map { UInt8(bitPattern: $0) })
        }
        let result = withUnsafePointer(to: &address) {
            $0.withMemoryRebound(to: sockaddr.self, capacity: 1) {
                Darwin.bind(
                    descriptor,
                    $0,
                    socklen_t(MemoryLayout<sockaddr_un>.size)
                )
            }
        }
        XCTAssertEqual(result, 0)
        XCTAssertEqual(chmod(path, 0o600), 0)
        XCTAssertTrue(path.hasPrefix("/"))
        XCTAssertFalse(path.contains("/../"))
        XCTAssertLessThanOrEqual(path.utf8.count, 96)
        XCTAssertEqual(URL(fileURLWithPath: path).lastPathComponent, "loomd.sock")
        var parentStat = stat()
        var socketStat = stat()
        XCTAssertEqual(lstat(root.path, &parentStat), 0)
        XCTAssertEqual(parentStat.st_mode & S_IFMT, S_IFDIR)
        XCTAssertEqual(parentStat.st_mode & 0o777, 0o700)
        XCTAssertEqual(parentStat.st_uid, geteuid())
        XCTAssertEqual(lstat(path, &socketStat), 0)
        XCTAssertEqual(socketStat.st_mode & S_IFMT, S_IFSOCK)
        XCTAssertEqual(socketStat.st_mode & 0o777, 0o600)
        XCTAssertEqual(socketStat.st_uid, geteuid())
        if let resolved = realpath(root.path, nil) {
            XCTAssertEqual(String(cString: resolved), root.path)
            free(resolved)
        } else {
            XCTFail("realpath failed")
        }
        XCTAssertNoThrow(try LocalIPCClient(socketPath: path))
    }

    private static func makeClientBackedByPrivateSocket() throws -> LocalIPCClient {
        let root = URL(fileURLWithPath: "/private/tmp")
            .appendingPathComponent("loom-swift-\(UUID().uuidString.prefix(8))")
        try FileManager.default.createDirectory(
            at: root,
            withIntermediateDirectories: false,
            attributes: [.posixPermissions: 0o700]
        )
        let path = root.appendingPathComponent("loomd.sock").path
        let descriptor = socket(AF_UNIX, SOCK_STREAM, 0)
        XCTAssertGreaterThanOrEqual(descriptor, 0)
        var address = sockaddr_un()
        address.sun_family = sa_family_t(AF_UNIX)
        let bytes = Array(path.utf8CString)
        withUnsafeMutableBytes(of: &address.sun_path) {
            $0.copyBytes(from: bytes.map { UInt8(bitPattern: $0) })
        }
        let result = withUnsafePointer(to: &address) {
            $0.withMemoryRebound(to: sockaddr.self, capacity: 1) {
                Darwin.bind(
                    descriptor,
                    $0,
                    socklen_t(MemoryLayout<sockaddr_un>.size)
                )
            }
        }
        XCTAssertEqual(result, 0)
        XCTAssertEqual(chmod(path, 0o600), 0)
        let client = try LocalIPCClient(socketPath: path)
        Darwin.close(descriptor)
        try FileManager.default.removeItem(at: root)
        return client
    }
}
