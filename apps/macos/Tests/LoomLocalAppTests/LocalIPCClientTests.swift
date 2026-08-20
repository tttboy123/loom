import Darwin
import Foundation
import XCTest
@testable import LoomLocalAppCore

final class LocalIPCClientTests: XCTestCase {
    func testConversationAttemptFailureBecomesOperationalError() {
        let incidentID = "loom-chat-11111111-1111-4111-8111-111111111111"
        let attempt = LocalProductConversationAttempt(
            attemptID: "attempt-1",
            segmentID: "segment-1",
            profileID: "conversation-deepseek-r6",
            modelID: "deepseek-v4-flash",
            reasoningEffort: "high",
            contextMode: .summaryOnly,
            contextCapsuleDigest: String(repeating: "a", count: 64),
            bindingDigest: String(repeating: "b", count: 64),
            incidentID: incidentID,
            status: "failed",
			failureCode: "provider_rate_limit",
			failureStage: "provider_rate_limit",
			httpStatus: 429,
			providerCode: "rate_limit_exceeded",
			failureMessage: "Provider rate limit reached.",
			retryAfterSeconds: 18,
			retryable: true
        )
        let failure = LocalIPCClient.conversationAttemptError(
            LocalProductChatThread(
                threadID: "thread-deepseek",
                attempts: [attempt],
                messages: [],
                canReply: true,
                requiresConfirmation: false
            ),
            incidentID: incidentID
        )

		XCTAssertEqual(failure?.code, .providerRateLimit)
		XCTAssertEqual(failure?.stage, .providerRateLimit)
		XCTAssertEqual(failure?.httpStatus, 429)
		XCTAssertEqual(failure?.providerCode, "rate_limit_exceeded")
		XCTAssertEqual(failure?.safeMessage, "Provider rate limit reached.")
		XCTAssertEqual(failure?.retryAfterSeconds, 18)
		XCTAssertTrue(failure?.recoverable ?? false)
        XCTAssertEqual(failure?.incidentID, incidentID)
    }

    func testProviderAccountIdentityIsScopedToProvider() {
        XCTAssertTrue(
            LocalIPCClient.validProviderAccountID(
                "deepseek.work",
                providerID: "deepseek"
            )
        )
        XCTAssertTrue(
            LocalIPCClient.validProviderAccountID(
                "deepseek.team.primary",
                providerID: "deepseek"
            )
        )
        for value in [
            "openai.work", "deepseek", "deepseek.", "deepseek..work",
            "deepseek.Work", "deepseek.work_secret",
        ] {
            XCTAssertFalse(
                LocalIPCClient.validProviderAccountID(
                    value,
                    providerID: "deepseek"
                ),
                value
            )
        }
    }

    func testCredentialSecretNormalizationTrimsOnlyASCIIEdges() {
        XCTAssertEqual(
            LocalIPCClient.normalizedCredentialSecret(" \t\r\nsk-deepseek-test\r\n "),
            "sk-deepseek-test"
        )
        XCTAssertEqual(
            LocalIPCClient.normalizedCredentialSecret("sk-deep seek"),
            "sk-deep seek"
        )
        XCTAssertNil(
            LocalIPCClient.normalizedCredentialSecret("sk-deep\nseek")
        )
        XCTAssertNil(LocalIPCClient.normalizedCredentialSecret(" \r\n\t"))
    }

    func testRealClientAdvertisesDecisionProtocolForNativeStoreComposition() throws {
        let client: LocalProductClientProtocol =
            try Self.makeClientBackedByPrivateSocket()

        XCTAssertNotNil(client as? LocalProductDecisionClientProtocol)
        XCTAssertNotNil(client as? LocalProductExecutionClientProtocol)
        XCTAssertNotNil(client as? LocalProductAgentRecoveryClientProtocol)
        XCTAssertNotNil(client as? LocalProductAgentInputClientProtocol)
        XCTAssertNotNil(client as? LocalProductHandoffClientProtocol)
    }

    func testAgentInputRequestUsesBase64ContentAndStrictReceipt() throws {
        let request = LocalProductAgentInputRequest(
            segmentID: "segment-1",
            agentInstanceID: "agent-1",
            workItemID: "work-1",
            runID: "run-1",
            claimGeneration: 3,
            mode: .inject,
            content: Data("review the failing test".utf8)
        )
        let encoded = try JSONEncoder().encode(request)
        let object = try XCTUnwrap(
            JSONSerialization.jsonObject(with: encoded) as? [String: Any]
        )
        XCTAssertEqual(
            Set(object.keys),
            Set([
                "schema_version", "segment_id", "agent_instance_id",
                "work_item_id", "run_id", "claim_generation", "mode",
                "context_scope", "scope_target_id", "content",
            ])
        )
        XCTAssertEqual(object["content"] as? String, "cmV2aWV3IHRoZSBmYWlsaW5nIHRlc3Q=")

        let receipt = try LocalProductAgentInputWire.decodeReceipt(Data("""
        {"schema_version":1,"incident_id":"agent-input-1","input_id":"input-1","mode":"inject","order_key":2,"target_step_id":"step-2","target_step_sequence":2}
        """.utf8))
        XCTAssertEqual(receipt.mode, .inject)
        XCTAssertEqual(receipt.targetStepSequence, 2)

        XCTAssertThrowsError(
            try LocalProductAgentInputWire.decodeReceipt(Data("""
            {"schema_version":1,"incident_id":"agent-input-1","input_id":"input-1","mode":"inject","order_key":2,"target_step_id":"step-2","target_step_sequence":2,"content":"forbidden"}
            """.utf8))
        )
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

    func testRemoteCredentialErrorDecodesOnlyKnownSafeStage() throws {
        let staged = """
        {"version":1,"request_id":"request-1","ok":false,"result":null,"error":{"code":"credential_unavailable","message":"credential unavailable","recoverable":true,"stage":"helper_authorization"}}
        """
        XCTAssertThrowsError(
            try LocalIPCWire.decodeResponse(
                Data(staged.utf8),
                expectedRequestID: "request-1"
            )
        ) { error in
            XCTAssertEqual(
                error as? LocalIPCRemoteError,
                LocalIPCRemoteError(
                    code: .credentialUnavailable,
                    recoverable: true,
                    stage: .helperAuthorization,
                    incidentID: "request-1"
                )
            )
        }

        let unknown = staged.replacingOccurrences(
            of: "helper_authorization",
            with: "private-path"
        )
        XCTAssertThrowsError(
            try LocalIPCWire.decodeResponse(
                Data(unknown.utf8),
                expectedRequestID: "request-1"
            )
        ) { error in
            XCTAssertEqual(error as? LocalProductClientError, .invalidResponse)
        }
    }

    func testVaultCredentialStagesDecodeThroughClosedWireSet() throws {
        let stages: [LocalIPCRemoteError.Stage] = [
            .vaultKeyLoad, .vaultOpen, .vaultEncrypt, .vaultCommit,
            .vaultDecrypt, .vaultAADValidation, .vaultRotation, .vaultRecovery,
            .vaultExport,
            .credentialLeaseIssue, .credentialLeaseExpire,
            .credentialLeaseRevoke, .migrationRead, .migrationCommit,
            .migrationCleanup,
        ]
        for stage in stages {
            let body = """
            {"version":1,"request_id":"request-vault","ok":false,"result":null,"error":{"code":"credential_unavailable","message":"credential unavailable","recoverable":true,"stage":"\(stage.rawValue)"}}
            """
            XCTAssertThrowsError(
                try LocalIPCWire.decodeResponse(
                    Data(body.utf8),
                    expectedRequestID: "request-vault"
                )
            ) { error in
                XCTAssertEqual(
                    (error as? LocalIPCRemoteError)?.stage,
                    stage
                )
            }
        }
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
            "conversation_unavailable",
            "conversation_limit",
            "invalid_response",
            "provider_auth",
            "provider_rate_limit",
            "provider_rejected",
			"provider_insufficient_balance",
			"provider_model_unavailable",
			"provider_invalid_request",
			"provider_unavailable",
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

    func testLongOperationsReceiveExtendedRequestTimeout() {
        XCTAssertEqual(LocalIPCClient.maximumRequestTimeoutSeconds, 55)
        for method in [
            "credential_verify", "credential_vault_rotate",
            "credential_vault_lock", "credential_vault_unlock",
            "credential_vault_reset",
            "credential_vault_export",
            "mission_execution",
        ] {
            XCTAssertEqual(
                LocalIPCClient.requestTimeoutSeconds(for: method),
                15,
                method
            )
        }
        for method in ["chat_message", "agent_attempt_recovery"] {
            XCTAssertEqual(
                LocalIPCClient.requestTimeoutSeconds(for: method),
                55,
                method
            )
        }
        for method in [
            "ping", "snapshot", "timeline_page", "setup_snapshot",
            "codex_connect", "builder_start", "builder_answer",
            "builder_edit", "builder_validate", "builder_confirm",
            "team_archive", "team_restore", "credential_configure",
            "credential_replace", "credential_revoke",
            "provider_account_policy_configure",
            "provider_model_rate_card_configure", "chat_thread",
        ] {
            XCTAssertEqual(
                LocalIPCClient.requestTimeoutSeconds(for: method),
                5,
                method
            )
        }
    }

    func testProviderAccountPolicyRequestUsesExactSafeWireKeys() throws {
        let body = try JSONEncoder().encode(
            ProviderAccountPolicyParams(
                providerID: "deepseek",
                providerAccountID: "deepseek.work",
                expectedRevision: 2,
                maximumConcurrentAttempts: 4,
                dispatchWindowSeconds: 60,
                maximumDispatchStarts: 20,
                maximumAssignedBudgetUnits: 12_000,
                trustDomain: "external_provider",
                retentionMode: "zero_data_retention",
                dataRegion: "apac",
                operationID: "policy-operation-1"
            )
        )
        let object = try XCTUnwrap(
            JSONSerialization.jsonObject(with: body) as? [String: Any]
        )

        XCTAssertEqual(
            Set(object.keys),
            Set([
                "provider_id", "provider_account_id", "expected_revision",
                "maximum_concurrent_attempts", "dispatch_window_seconds",
                "maximum_dispatch_starts", "maximum_assigned_budget_units",
                "trust_domain", "retention_mode", "data_region",
                "operation_id",
            ])
        )
        XCTAssertEqual(object["provider_account_id"] as? String, "deepseek.work")
        XCTAssertEqual(object["expected_revision"] as? Int, 2)
        XCTAssertEqual(object["maximum_concurrent_attempts"] as? Int, 4)
        XCTAssertEqual(object["trust_domain"] as? String, "external_provider")
        XCTAssertEqual(object["retention_mode"] as? String, "zero_data_retention")
        XCTAssertEqual(object["data_region"] as? String, "apac")
        XCTAssertNil(object["secret"])
        XCTAssertNil(object["credential_reference"])
        XCTAssertNil(object["policy_digest"])
    }

    func testProviderModelRateCardRequestUsesExactSafeWireKeys() throws {
        let body = try JSONEncoder().encode(
            ProviderModelRateCardParams(
                providerID: "deepseek",
                providerAccountID: "deepseek.work",
                modelID: "deepseek-chat",
                expectedRevision: 2,
                currency: "USD",
                inputTokenBasis: "input_includes_cache",
                inputMicrounitsPerMillion: 270_000,
                outputMicrounitsPerMillion: 1_100_000,
                cacheReadMicrounitsPerMillion: 70_000,
                cacheWriteMicrounitsPerMillion: 0,
                roundingMode: "ceiling_per_attempt",
                operationID: "rate-card-operation-1"
            )
        )
        let object = try XCTUnwrap(
            JSONSerialization.jsonObject(with: body) as? [String: Any]
        )
        XCTAssertEqual(
            Set(object.keys),
            Set([
                "provider_id", "provider_account_id", "model_id",
                "expected_revision", "currency", "input_token_basis",
                "input_microunits_per_million",
                "output_microunits_per_million",
                "cache_read_microunits_per_million",
                "cache_write_microunits_per_million", "rounding_mode",
                "operation_id",
            ])
        )
        XCTAssertEqual(object["model_id"] as? String, "deepseek-chat")
        XCTAssertEqual(object["expected_revision"] as? Int, 2)
        XCTAssertNil(object["secret"])
        XCTAssertNil(object["credential_reference"])
        XCTAssertNil(object["rate_card_digest"])
    }

    func testRemoteToolBackendEnrollmentRequestsUseExactSafeWireKeys() throws {
        let configure = try JSONEncoder().encode(
            RemoteToolBackendEnrollmentParams(
                enrollmentID: "mcp-deepseek-work",
                backendKind: "mcp_server",
                adapterID: "builtin.mcp.stdio.v1",
                providerID: "deepseek",
                providerAccountID: "deepseek.work",
                providerAccountPolicyVersion: 2,
                providerAccountPolicyRevision: 3,
                providerAccountPolicyDigest: String(repeating: "a", count: 64),
                endpointFingerprint: String(repeating: "b", count: 64),
                mcpServerID: "work-tools",
                allowedTools: ["get_issue", "search_docs"],
                expectedRevision: 1,
                maximumConcurrentCalls: 2,
                maximumCallsPerAttempt: 4,
                timeoutSeconds: 30,
                maximumResultBytes: 32_768,
                maximumBudgetUnits: 2_000,
                operationID: "remote-tool-operation-1"
            )
        )
        let configureObject = try XCTUnwrap(
            JSONSerialization.jsonObject(with: configure) as? [String: Any]
        )
        XCTAssertEqual(
            Set(configureObject.keys),
            Set([
                "enrollment_id", "backend_kind", "adapter_id", "provider_id",
                "provider_account_id", "provider_account_policy_version",
                "provider_account_policy_revision", "provider_account_policy_digest",
                "endpoint_fingerprint", "mcp_server_id", "allowed_tools",
                "expected_revision", "maximum_concurrent_calls",
                "maximum_calls_per_attempt", "timeout_seconds",
                "maximum_result_bytes", "maximum_budget_units", "operation_id",
            ])
        )
        XCTAssertNil(configureObject["secret"])
        XCTAssertNil(configureObject["endpoint"])
        XCTAssertNil(configureObject["credential_reference"])

        let revoke = try JSONEncoder().encode(
            RemoteToolBackendEnrollmentRevokeParams(
                enrollmentID: "mcp-deepseek-work",
                providerID: "deepseek",
                providerAccountID: "deepseek.work",
                expectedRevision: 2,
                operationID: "remote-tool-revoke-1"
            )
        )
        let revokeObject = try XCTUnwrap(
            JSONSerialization.jsonObject(with: revoke) as? [String: Any]
        )
        XCTAssertEqual(
            Set(revokeObject.keys),
            Set([
                "enrollment_id", "provider_id", "provider_account_id",
                "expected_revision", "operation_id",
            ])
        )
        XCTAssertNil(revokeObject["secret"])
        XCTAssertNil(revokeObject["endpoint_fingerprint"])
    }

    func testChatMessageRequestUsesExactDaemonWireKeys() throws {
        let binding = LocalProductConversationExecutionBinding(
            providerID: "deepseek",
            providerAccountID: "deepseek.primary",
            providerAccountPolicyVersion: 2,
            providerAccountPolicyRevision: 4,
            providerAccountPolicyDigest: String(repeating: "a", count: 64),
            trustDomain: "external_provider",
            retentionMode: "zero_data_retention",
            dataRegion: "apac"
        )
        let body = try JSONEncoder().encode(
            LocalProductChatMessageRequest(
                threadID: "thread-1",
                content: "hello",
                profileID: "conversation-deepseek-deepseek-chat-r2",
                contextMode: .summaryOnly,
                expectedExecutionBinding: binding
            )
        )
        let object = try XCTUnwrap(
            JSONSerialization.jsonObject(with: body) as? [String: Any]
        )

        XCTAssertEqual(
            Set(object.keys),
            Set([
                "thread_id", "content", "profile_id", "model_id",
                "reasoning_effort", "context_mode",
                "expected_execution_binding",
            ])
        )
        XCTAssertEqual(object["thread_id"] as? String, "thread-1")
        XCTAssertEqual(object["content"] as? String, "hello")
        XCTAssertEqual(
            object["profile_id"] as? String,
            "conversation-deepseek-deepseek-chat-r2"
        )
        XCTAssertEqual(object["context_mode"] as? String, "summary_only")
        let encodedBinding = try XCTUnwrap(
            object["expected_execution_binding"] as? [String: Any]
        )
        XCTAssertEqual(
            encodedBinding["provider_account_policy_revision"] as? Int,
            4
        )
        XCTAssertEqual(
            encodedBinding["provider_account_id"] as? String,
            "deepseek.primary"
        )
        XCTAssertNil(object["threadID"])
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

func testOpenCodeModelIdentityPassesClientValidation() {
    XCTAssertTrue(LocalIPCClient.validModelID("deepseek/deepseek-chat"))
    XCTAssertTrue(LocalIPCClient.validModelID("openai/gpt-5.5"))
    XCTAssertTrue(LocalIPCClient.validModelID("minimax/MiniMax-M3"))
    XCTAssertFalse(LocalIPCClient.validModelID("bad model"))
    XCTAssertFalse(LocalIPCClient.validModelID(""))
    XCTAssertTrue(LocalIPCClient.validIdentifier("high"))
    XCTAssertFalse(LocalIPCClient.validIdentifier("deepseek/deepseek-chat"))
}
