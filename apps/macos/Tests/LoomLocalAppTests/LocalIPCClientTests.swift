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

    func testMissionExecutionStagesDecodeThroughClosedWireSet() throws {
        let stages: [LocalIPCRemoteError.Stage] = [
            .preflightLease, .viewDrift, .preflightDigest,
            .parentContinuation, .flightConflict, .dispatchAdmission,
            .dispatchTeamAuthority, .dispatchCapacity, .dispatchAttemptValidation,
            .dispatchViewConflict, .dispatchIdentityUnavailable,
            .dispatchRecoveryRequired, .dispatchValidation,
			.dispatchContextValidation, .dispatchIncomplete,
			.workspacePublication, .workspacePublicationInputValidation,
			.workspacePublicationSourceSnapshot, .workspacePublicationSourceDrift,
			.workspacePublicationStageCreate, .workspacePublicationChangeValidation,
			.workspacePublicationChangeConflict, .workspacePublicationDestructiveChange,
			.workspacePublicationStageWrite, .workspacePublicationApply,
			.workspacePublicationFinalDigest, .workspacePublicationSync,
			.workspacePublicationMainNodeMissing, .workspacePublicationMainCandidateMissing,
			.workspacePublicationMainChangesMissing, .workspacePublicationMainDigestMissing,
			.workspacePublicationCancelled,
        ]
        for stage in stages {
            let body = """
            {"version":1,"request_id":"request-mission","ok":false,"result":null,"error":{"code":"conflict","message":"conflict","recoverable":true,"stage":"\(stage.rawValue)"}}
            """
            XCTAssertThrowsError(
                try LocalIPCWire.decodeResponse(
                    Data(body.utf8),
                    expectedRequestID: "request-mission"
                )
            ) { error in
                XCTAssertEqual((error as? LocalIPCRemoteError)?.code, .conflict)
                XCTAssertEqual((error as? LocalIPCRemoteError)?.stage, stage)
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
			"workspace_publish_failed",
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
        XCTAssertEqual(LocalIPCClient.maximumRequestTimeoutSeconds, 1_810)
        for method in [
            "credential_verify", "credential_vault_rotate",
            "credential_vault_lock", "credential_vault_unlock",
            "credential_vault_reset",
            "credential_vault_export",
        ] {
            XCTAssertEqual(
                LocalIPCClient.requestTimeoutSeconds(for: method),
                15,
                method
            )
        }
        XCTAssertEqual(
            LocalIPCClient.requestTimeoutSeconds(for: "chat_message"),
            1_810
        )
        XCTAssertEqual(
            LocalIPCClient.requestTimeoutSeconds(for: "chat_response_cancel"),
            2
        )
        XCTAssertEqual(
            LocalIPCClient.requestTimeoutSeconds(for: "agent_attempt_recovery"),
            55
        )
        XCTAssertEqual(
            LocalIPCClient.requestTimeoutSeconds(for: "mission_execution"),
            185
        )
        XCTAssertEqual(
            LocalIPCClient.requestTimeoutSeconds(for: "setup_snapshot"),
            185
        )
        for method in [
            "ping", "snapshot", "timeline_page",
            "codex_connect", "builder_start", "builder_answer",
            "builder_edit", "builder_validate", "builder_confirm",
            "team_archive", "team_restore", "credential_configure",
            "credential_replace", "credential_revoke",
            "provider_account_policy_configure",
            "provider_model_rate_card_configure", "chat_thread",
            "chat_context_disclosure",
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
            schemaVersion: 4,
            harnessAdapter: "loom-native",
            providerID: "deepseek",
            providerAccountID: "deepseek.primary",
            credentialRevision: 2,
            modelID: "deepseek-chat",
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
                expectedExecutionBinding: binding,
                trustBoundaryAcknowledgement:
                    LocalProductTrustBoundaryAcknowledgement(
                        schemaVersion: 3,
                        sourceSegmentID: "segment-1",
                        sourceBindingDigest: String(repeating: "b", count: 64),
                        targetProfileID: "conversation-deepseek-deepseek-chat-r2",
                        targetExecutionBinding: binding,
                        contextMode: .summaryOnly,
                        acknowledged: true,
                        reviewDigest: String(repeating: "c", count: 64)
                    )
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
                "trust_boundary_acknowledgement",
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
        XCTAssertEqual(encodedBinding["harness_adapter"] as? String, "loom-native")
        XCTAssertEqual(encodedBinding["credential_revision"] as? Int, 2)
        XCTAssertEqual(encodedBinding["model_id"] as? String, "deepseek-chat")
        let acknowledgement = try XCTUnwrap(
            object["trust_boundary_acknowledgement"] as? [String: Any]
        )
        XCTAssertEqual(
            Set(acknowledgement.keys),
            Set([
                "schema_version", "source_segment_id", "source_binding_digest",
                "target_profile_id", "target_execution_binding",
                "target_reasoning_effort", "context_mode", "acknowledged",
                "review_digest",
            ])
        )
        XCTAssertEqual(acknowledgement["source_segment_id"] as? String, "segment-1")
        XCTAssertEqual(acknowledgement["schema_version"] as? Int, 3)
        XCTAssertEqual(
            acknowledgement["target_profile_id"] as? String,
            "conversation-deepseek-deepseek-chat-r2"
        )
        XCTAssertEqual(acknowledgement["target_reasoning_effort"] as? String, "")
        XCTAssertEqual(acknowledgement["context_mode"] as? String, "summary_only")
        XCTAssertEqual(acknowledgement["acknowledged"] as? Bool, true)
        XCTAssertEqual(
            acknowledgement["review_digest"] as? String,
            String(repeating: "c", count: 64)
        )
        XCTAssertNil(object["threadID"])
    }

    func testTrustBoundaryReviewDigestMatchesDaemonCanonicalVector() throws {
        let sourceBinding = LocalProductConversationExecutionBinding(
            schemaVersion: 4, harnessAdapter: "loom-native",
            providerID: "deepseek", providerAccountID: "deepseek.work",
            credentialRevision: 7, modelID: "deepseek-chat",
            providerAccountPolicyVersion: 2,
            providerAccountPolicyRevision: 3,
            providerAccountPolicyDigest: String(repeating: "a", count: 64),
            trustDomain: "external_provider",
            retentionMode: "provider_default", dataRegion: "global"
        )
        let targetBinding = LocalProductConversationExecutionBinding(
            schemaVersion: 4, harnessAdapter: "claude-code",
            providerID: "anthropic", providerAccountID: "anthropic.work",
            credentialRevision: 5, modelID: "claude-sonnet-4",
            providerAccountPolicyVersion: 2,
            providerAccountPolicyRevision: 4,
            providerAccountPolicyDigest: String(repeating: "b", count: 64),
            trustDomain: "enterprise_tenant",
            retentionMode: "zero_data_retention", dataRegion: "apac"
        )
        let source = LocalProductConversationSegment(
            segmentID: "segment-1",
            profileID: "conversation-deepseek-work-r7",
            contextMode: .startClean,
            contextCapsuleDigest: String(repeating: "c", count: 64),
            executionBinding: sourceBinding,
            bindingDigest: "04c575cf749b3ee6196aa1ba100b2b0a0ecd67edf52414666f662de40eb123fe"
        )
        let review = try XCTUnwrap(
            LocalProductTrustBoundaryAcknowledgement.reviewed(
                threadID: "thread-trust-review",
                sourceSegment: source,
                targetProfileID: "conversation-anthropic-work-r5",
                targetExecutionBinding: targetBinding,
                targetReasoningEffort: "",
                contextMode: .summaryOnly
            )
        )
        XCTAssertEqual(
            review.reviewDigest,
            "fcc87e6a321b2f7b303831173b63c928a394f5abe0519df5efcb94ae01474398"
        )
        XCTAssertEqual(review.schemaVersion, 3)
        XCTAssertEqual(review.targetProfileID, "conversation-anthropic-work-r5")
        let differentProfile = try XCTUnwrap(
            LocalProductTrustBoundaryAcknowledgement.reviewed(
                threadID: "thread-trust-review",
                sourceSegment: source,
                targetProfileID: "conversation-anthropic-work-r6",
                targetExecutionBinding: targetBinding,
                targetReasoningEffort: "",
                contextMode: .summaryOnly
            )
        )
        XCTAssertNotEqual(differentProfile.reviewDigest, review.reviewDigest)
    }

    func testChatContextDisclosureRequestUsesExactSafeWireKeys() throws {
        let body = try JSONEncoder().encode(
            LocalProductContextDisclosureRequest(
                threadID: "thread-1",
                segmentID: "segment-2"
            )
        )
        let object = try XCTUnwrap(
            JSONSerialization.jsonObject(with: body) as? [String: Any]
        )
        XCTAssertEqual(Set(object.keys), Set(["thread_id", "segment_id"]))
        XCTAssertEqual(object["thread_id"] as? String, "thread-1")
        XCTAssertEqual(object["segment_id"] as? String, "segment-2")
        XCTAssertNil(object["context_capsule_digest"])
        XCTAssertNil(object["content"])
        XCTAssertNil(object["prompt"])
    }

    func testChatResponseCancelRequestUsesOnlyResponseIdentity() throws {
        let body = try JSONEncoder().encode(
            ChatResponseCancelParams(
                threadID: "thread-1",
                incidentID: "loom-chat-11111111-1111-4111-8111-111111111111"
            )
        )
        let object = try XCTUnwrap(
            JSONSerialization.jsonObject(with: body) as? [String: Any]
        )

        XCTAssertEqual(Set(object.keys), Set(["thread_id", "incident_id"]))
        XCTAssertEqual(object["thread_id"] as? String, "thread-1")
        XCTAssertEqual(
            object["incident_id"] as? String,
            "loom-chat-11111111-1111-4111-8111-111111111111"
        )
        XCTAssertNil(object["content"])
        XCTAssertNil(object["prompt"])
        XCTAssertNil(object["profile_id"])
    }

    func testChatResponseCancelAcknowledgementIsExactAndMetadataOnly() throws {
        let acknowledgement = try JSONDecoder().decode(
            ChatResponseCancelAcknowledgement.self,
            from: Data(
                """
                {"thread_id":"thread-1","incident_id":"loom-chat-1","cancelled":true}
                """.utf8
            )
        )
        XCTAssertEqual(acknowledgement.threadID, "thread-1")
        XCTAssertEqual(acknowledgement.incidentID, "loom-chat-1")
        XCTAssertTrue(acknowledgement.cancelled)

        for body in [
            "{\"thread_id\":\"thread-1\",\"incident_id\":\"loom-chat-1\",\"cancelled\":false}",
            "{\"thread_id\":\"thread-1\",\"incident_id\":\"loom-chat-1\",\"cancelled\":true,\"content\":\"private\"}",
            "{\"thread_id\":\"thread-1\",\"incident_id\":\"loom-chat-1\",\"cancelled\":true,\"messages\":[]}",
        ] {
            XCTAssertThrowsError(
                try JSONDecoder().decode(
                    ChatResponseCancelAcknowledgement.self,
                    from: Data(body.utf8)
                ),
                body
            )
        }
    }

    func testChatMessageDecoderPreservesCancelledAttemptFromFinalThread() throws {
        let thread = try LocalIPCClient.decodeChatThreadResponse(
            Data(
                """
                {
                  "thread_id":"thread-1","profile_id":"profile-1",
                  "attempts":[{
                    "attempt_id":"attempt-1","segment_id":"segment-1",
                    "profile_id":"profile-1","context_mode":"start_clean",
                    "context_capsule_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
                    "binding_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
                    "incident_id":"loom-chat-1","status":"cancelled","failure_code":""
                  }],
                  "messages":[],"can_reply":true,"requires_confirmation":false
                }
                """.utf8
            )
        )

        XCTAssertEqual(thread.attempts.count, 1)
        XCTAssertEqual(thread.attempts[0].status, "cancelled")
        XCTAssertEqual(thread.attempts[0].incidentID, "loom-chat-1")
        XCTAssertEqual(thread.threadID, "thread-1")
    }

    func testChatMessageDecoderRejectsCancelledAttemptWithFailureMetadata() {
        XCTAssertThrowsError(
            try LocalIPCClient.decodeChatThreadResponse(
                Data(
                    """
                    {
                      "thread_id":"thread-1","profile_id":"profile-1",
                      "attempts":[{
                        "attempt_id":"attempt-1","segment_id":"segment-1",
                        "profile_id":"profile-1","context_mode":"start_clean",
                        "context_capsule_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
                        "binding_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
                        "incident_id":"loom-chat-1","status":"cancelled",
                        "failure_code":"timeout","failure_stage":"conversation_dispatch",
                        "retryable":true
                      }],
                      "messages":[],"can_reply":true,"requires_confirmation":false
                    }
                    """.utf8
                )
            )
        )
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

    func testClientCreatedBeforeRunDirectoryRecoversWhenDaemonAppears() async throws {
        let root = URL(fileURLWithPath: "/private/tmp")
            .appendingPathComponent("loom-swift-\(UUID().uuidString.prefix(8))")
        let diagnosticsRoot = URL(fileURLWithPath: "/private/tmp")
            .appendingPathComponent("loom-swift-diagnostics-\(UUID().uuidString.prefix(8))")
        let runDirectory = root.appendingPathComponent("run")
        let path = runDirectory.appendingPathComponent("loomd.sock").path
        defer {
            if FileManager.default.fileExists(atPath: root.path) {
                try? FileManager.default.removeItem(at: root)
            }
        }
        defer {
            if FileManager.default.fileExists(atPath: diagnosticsRoot.path) {
                try? FileManager.default.removeItem(at: diagnosticsRoot)
            }
        }

        let client: LocalIPCClient
        do {
            client = try LocalIPCClient(
                socketPath: path,
                requestID: { "bootstrap-recovery-1" },
                operationalDiagnostics: LocalOperationalDiagnostics(
                    directory: diagnosticsRoot,
                    maximumBytes: 512,
                    now: { Date(timeIntervalSince1970: 0) }
                ),
                recordsInstalledDiagnostics: false
            )
        } catch {
            XCTFail("client construction must survive a missing run directory: \(error)")
            return
        }
        let boundary: LocalProductClientProtocol = client
        XCTAssertNotNil(boundary as? LocalProductDecisionClientProtocol)
        XCTAssertNotNil(boundary as? LocalProductSetupClientProtocol)
        XCTAssertNotNil(boundary as? LocalProductExecutionClientProtocol)
        XCTAssertNotNil(boundary as? LocalProductAgentRecoveryClientProtocol)
        XCTAssertNotNil(boundary as? LocalProductToolRecoveryClientProtocol)
        XCTAssertNotNil(boundary as? LocalProductAgentInputClientProtocol)
        XCTAssertNotNil(boundary as? LocalProductHandoffClientProtocol)
        XCTAssertNotNil(boundary as? LocalRoundtableClientProtocol)
        XCTAssertNotNil(boundary as? LocalProductAssetClientProtocol)
        XCTAssertNotNil(boundary as? LocalProductPermissionClientProtocol)
        XCTAssertNotNil(boundary as? LocalProductExecutionSnapshotClientProtocol)
        XCTAssertNotNil(boundary as? LocalProductProductionSnapshotClientProtocol)

        try FileManager.default.createDirectory(
            at: root,
            withIntermediateDirectories: false,
            attributes: [.posixPermissions: 0o700]
        )
        try FileManager.default.createDirectory(
            at: runDirectory,
            withIntermediateDirectories: false,
            attributes: [.posixPermissions: 0o700]
        )
        let server = try Self.makeListeningSocket(at: path, permissions: 0o600)
        defer { Darwin.close(server) }
        let responseTask = Task.detached {
            try Self.servePingOnce(server: server)
        }

        let available = try await client.ping()
        XCTAssertTrue(available)
        try await responseTask.value
    }

    func testReachableSameOwnerSocketWithMalformedModeIsNotTrusted() throws {
        let root = URL(fileURLWithPath: "/private/tmp")
            .appendingPathComponent("loom-swift-\(UUID().uuidString.prefix(8))")
        try FileManager.default.createDirectory(
            at: root,
            withIntermediateDirectories: false,
            attributes: [.posixPermissions: 0o700]
        )
        defer { try? FileManager.default.removeItem(at: root) }
        let path = root.appendingPathComponent("loomd.sock").path
        let server = try Self.makeListeningSocket(at: path, permissions: 0o640)
        defer { Darwin.close(server) }

        let probe = Darwin.socket(AF_UNIX, SOCK_STREAM, 0)
        XCTAssertGreaterThanOrEqual(probe, 0)
        defer { Darwin.close(probe) }
        var address = sockaddr_un()
        address.sun_family = sa_family_t(AF_UNIX)
        let bytes = Array(path.utf8CString)
        withUnsafeMutableBytes(of: &address.sun_path) {
            $0.copyBytes(from: bytes.map { UInt8(bitPattern: $0) })
        }
        let connected = withUnsafePointer(to: &address) {
            $0.withMemoryRebound(to: sockaddr.self, capacity: 1) {
                Darwin.connect(
                    probe,
                    $0,
                    socklen_t(MemoryLayout<sockaddr_un>.size)
                )
            }
        }
        XCTAssertEqual(connected, 0, "fixture socket must be reachable")
        XCTAssertFalse(LocalIPCClient.isTrustedSocket(at: path))
    }

    private static func makeListeningSocket(
        at path: String,
        permissions: mode_t
    ) throws -> Int32 {
        let descriptor = Darwin.socket(AF_UNIX, SOCK_STREAM, 0)
        guard descriptor >= 0 else { throw LocalProductClientError.unavailable }
        var address = sockaddr_un()
        address.sun_family = sa_family_t(AF_UNIX)
        let bytes = Array(path.utf8CString)
        guard bytes.count <= MemoryLayout.size(ofValue: address.sun_path) else {
            Darwin.close(descriptor)
            throw LocalProductClientError.invalidSocket
        }
        withUnsafeMutableBytes(of: &address.sun_path) {
            $0.copyBytes(from: bytes.map { UInt8(bitPattern: $0) })
        }
        let bound = withUnsafePointer(to: &address) {
            $0.withMemoryRebound(to: sockaddr.self, capacity: 1) {
                Darwin.bind(
                    descriptor,
                    $0,
                    socklen_t(MemoryLayout<sockaddr_un>.size)
                )
            }
        }
        guard bound == 0,
              chmod(path, permissions) == 0,
              Darwin.listen(descriptor, 1) == 0 else {
            Darwin.close(descriptor)
            throw LocalProductClientError.unavailable
        }
        return descriptor
    }

    private static func servePingOnce(server: Int32) throws {
        let connection = Darwin.accept(server, nil, nil)
        guard connection >= 0 else { throw LocalProductClientError.unavailable }
        defer { Darwin.close(connection) }
        var request = Data()
        var buffer = [UInt8](repeating: 0, count: 4_096)
        while true {
            let count = Darwin.recv(connection, &buffer, buffer.count, 0)
            if count == 0 { break }
            guard count > 0 else { throw LocalProductClientError.unavailable }
            request.append(buffer, count: count)
        }
        let body = try LocalIPCWire.unframe(
            request,
            maximum: LocalIPCClient.requestMaximum
        )
        let object = try XCTUnwrap(
            JSONSerialization.jsonObject(with: body) as? [String: Any]
        )
        let requestID = try XCTUnwrap(object["request_id"] as? String)
        let response = try JSONSerialization.data(withJSONObject: [
            "version": 1,
            "request_id": requestID,
            "ok": true,
            "result": [
                "protocol_version": 1,
                "available": true,
                "build_id": "bootstrap-test",
            ],
        ])
        let framed = try LocalIPCWire.frame(
            response,
            maximum: LocalIPCClient.responseMaximum
        )
        try framed.withUnsafeBytes { bytes in
            var sent = 0
            while sent < bytes.count {
                let count = Darwin.send(
                    connection,
                    bytes.baseAddress!.advanced(by: sent),
                    bytes.count - sent,
                    0
                )
                guard count > 0 else { throw LocalProductClientError.unavailable }
                sent += count
            }
        }
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
