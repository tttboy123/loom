import Foundation
import XCTest
@testable import LoomLocalAppCore

final class LocalProductModelsTests: XCTestCase {
    func testRoundTableActionProposalFreezesSeatBindingAndRejectsPartialPair() throws {
        let digest = String(repeating: "a", count: 64)
        func proposalJSON(payload: String) -> Data {
            Data(
                """
                {
                  "schema_version":2,"proposal_id":"proposal-skip-bound-1",
                  "tool_id":"loom.roundtables.skip.preview","tool_version":2,
                  "confirmation":"user","action":"roundtable_skip","argument":"",
                  "payload":\(payload),
                  "route":{"harness_adapter":"codex","provider_id":"openai",
                    "credential_revision":0,"model_id":"codex-default",
                    "execution_binding_digest":"\(digest)",
                    "context_capsule_digest":"\(digest)"},
                  "workspace":{"workspace_id":"workspace-primary",
                    "workspace_digest":"\(digest)"},
                  "registry_digest":"\(digest)","incident_id":"incident-skip-1",
                  "target_conversation_id":"conversation-1",
                  "target_content_digest":"\(digest)","segment_id":"segment-1",
                  "attempt_id":"attempt-1","message_id":"message-1",
                  "status":"pending","created_at":"2026-08-29T10:00:00Z",
                  "expires_at":"2026-08-29T10:05:00Z","proposal_digest":"\(digest)"
                }
                """.utf8
            )
        }

        let bound = try JSONDecoder().decode(
            LocalProductConversationActionProposal.self,
            from: proposalJSON(
                payload: """
                {"session_id":"session+本地","round_id":"round#评审","seat_id":"seat+c++/评审",\
                "membership_revision":3,"seat_binding_digest":"\(digest)"}
                """
            )
        )
        XCTAssertTrue(bound.payload?.hasFrozenRoundTableSeatBinding == true)
        XCTAssertEqual(bound.payload?.membershipRevision, 3)
        XCTAssertEqual(bound.payload?.seatBindingDigest, digest)
        XCTAssertEqual(bound.payload?.seatID, "seat+c++/评审")

        let legacy = try JSONDecoder().decode(
            LocalProductConversationActionProposal.self,
            from: proposalJSON(
                payload: #"{"session_id":"session-1","round_id":"round-1","seat_id":"seat-writer"}"#
            )
        )
        XCTAssertFalse(legacy.payload?.hasFrozenRoundTableSeatBinding == true)

        for partial in [
            #"{"session_id":"session-1","round_id":"round-1","seat_id":"seat-writer","membership_revision":3}"#,
            """
            {"session_id":"session-1","round_id":"round-1","seat_id":"seat-writer",\
            "seat_binding_digest":"\(digest)"}
            """,
        ] {
            XCTAssertThrowsError(
                try JSONDecoder().decode(
                    LocalProductConversationActionProposal.self,
                    from: proposalJSON(payload: partial)
                )
            )
        }
    }

    func testSetupSnapshotDecodesProviderDirectory() throws {
        let data = Data(#"""
        {
          "schema_version":1,
          "view_version":"view-1",
          "codex":{"provider_id":"codex","auth_mode":"native_auth","credential_reference":"","revision":0,"status":"available","reason":""},
          "minimax":{"provider_id":"minimax","auth_mode":"brokered","credential_reference":"","revision":0,"status":"unconfigured","reason":""},
          "providers":[
            {"provider_id":"openai","display_name":"OpenAI","category":"official","protocol":"openai_responses","auth_mode":"brokered","connection_kind":"api_key","credential_reference":"","revision":0,"status":"unconfigured","reason":"","supports_model_discovery":true},
            {"provider_id":"deepseek","display_name":"DeepSeek","category":"official","protocol":"openai_compatible","auth_mode":"brokered","connection_kind":"api_key","credential_reference":"","revision":0,"status":"unconfigured","reason":"","supports_model_discovery":true}
          ],
          "conversation_profiles":[
            {"profile_id":"conversation-openai-codex-default-v1","harness_adapter":"codex","provider_id":"openai","provider_account_id":"","display_name":"OpenAI","protocol":"openai_responses","model_id":"codex-default","auth_mode":"native_auth","credential_revision":0},
            {"profile_id":"conversation-anthropic-claude-sonnet-5-account-work-r5","harness_adapter":"loom-native","provider_id":"anthropic","provider_account_id":"anthropic.work","display_name":"Anthropic","protocol":"anthropic_messages","model_id":"claude-sonnet-5","auth_mode":"brokered","credential_revision":5},
            {"profile_id":"conversation-deepseek-deepseek-chat-r2","harness_adapter":"loom-native","provider_id":"deepseek","provider_account_id":"deepseek.primary","display_name":"DeepSeek","protocol":"openai_compatible","model_id":"deepseek-chat","auth_mode":"brokered","credential_revision":2}
          ],
          "runtimes":[],"saved_teams":[],"templates":[],"role_options":[],
          "skills":[],"permissions":[],"resources":[]
        }
        """#.utf8)

        let snapshot = try LocalProductSetupWire.decodeSnapshot(data)
        XCTAssertEqual(snapshot.providers.map(\.providerID), ["openai", "deepseek"])
        XCTAssertEqual(snapshot.providers[1].protocolFamily, "openai_compatible")
        XCTAssertEqual(snapshot.providers[1].status, "unconfigured")
        XCTAssertEqual(
            snapshot.conversationProfiles.map(\.profileID),
            [
                "conversation-openai-codex-default-v1",
                "conversation-anthropic-claude-sonnet-5-account-work-r5",
                "conversation-deepseek-deepseek-chat-r2",
            ]
        )
        XCTAssertEqual(snapshot.conversationProfiles[1].protocolFamily, "anthropic_messages")
        XCTAssertEqual(snapshot.conversationProfiles[1].providerAccountID, "anthropic.work")
        XCTAssertEqual(snapshot.conversationProfiles[1].modelID, "claude-sonnet-5")
        XCTAssertEqual(snapshot.conversationProfiles[2].modelID, "deepseek-chat")
        XCTAssertEqual(snapshot.conversationProfiles[2].harnessAdapter, "loom-native")
        XCTAssertEqual(snapshot.conversationProfiles[2].providerAccountID, "deepseek.primary")
    }

    func testStrictSnapshotDecodingAcceptsExactSchema() throws {
        let snapshot = try LocalProductWire.decodeSnapshot(
            Data(Self.snapshotJSON.utf8)
        )

        XCTAssertEqual(snapshot.schemaVersion, 1)
        XCTAssertEqual(snapshot.viewVersion, "view-1")
        XCTAssertEqual(snapshot.runtimes.map(\.runtimeInstanceID), ["pi-local"])
        XCTAssertTrue(snapshot.teams.isEmpty)
        XCTAssertEqual(snapshot.health.projection, "unknown")
    }

    func testStrictSnapshotV2RoundTripsBoundedServiceHealth() throws {
        let snapshot = try LocalProductWire.decodeSnapshot(
            Data(Self.snapshotV2JSON.utf8)
        )

        XCTAssertEqual(snapshot.schemaVersion, 2)
        XCTAssertEqual(snapshot.health.daemon, "serving_request")
        XCTAssertEqual(snapshot.health.journal, "available")
        XCTAssertEqual(snapshot.health.projection, "current")

        let missingHealth = Self.snapshotV2JSON.replacingOccurrences(
            of: "\"health\":{\"daemon\":\"serving_request\",\"journal\":\"available\",\"projection\":\"current\"},",
            with: ""
        )
        let legacy = try LocalProductWire.decodeSnapshot(
            Data(missingHealth.utf8)
        )
        XCTAssertEqual(legacy.health, .unknown)
    }

    func testStrictSnapshotDecodingRejectsUnknownAndDuplicateKeys() {
        let unknown = Self.snapshotJSON.replacingOccurrences(
            of: "\"reason\":\"\",",
            with: "\"reason\":\"\",\"unexpected\":true,"
        )
        XCTAssertThrowsError(
            try LocalProductWire.decodeSnapshot(Data(unknown.utf8))
        )

        let duplicate = Self.snapshotJSON.replacingOccurrences(
            of: "\"view_version\":\"view-1\",",
            with: "\"view_version\":\"view-1\",\"view_version\":\"other\","
        )
        XCTAssertThrowsError(
            try LocalProductWire.decodeSnapshot(Data(duplicate.utf8))
        )
    }

    func testToolShapedTentativeChatUsesFixedNonActionableContent() throws {
        let thread = try LocalProductWire.decodeChatThread(
            Data(
                """
                {
                  "thread_id":"thread-1",
                  "profile_id":"conversation-deepseek-deepseek-chat-r2",
                  "segments":[{
                    "segment_id":"segment-1",
                    "profile_id":"conversation-deepseek-deepseek-chat-r2",
                    "context_mode":"start_clean",
                    "context_capsule_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
                    "disclosure_receipt_digest":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
                    "disclosed_context_count":1,
                    "omitted_context_count":0,
                    "context_token_budget":2048,
                    "context_token_count":7,
                    "binding_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
                    "created_at":"2026-08-10T00:00:00Z"
                  }],
                  "attempts":[],
                  "messages":[{
                    "message_id":"message-1",
                    "segment_id":"segment-1",
                    "role":"loom",
                    "content":"{\\"tool_name\\":\\"read_file\\",\\"path\\":\\"/private/sentinel\\"}",
                    "tentative":true
                  }],
                  "can_reply":true,
                  "requires_confirmation":false
                }
                """.utf8
            )
        )

        XCTAssertEqual(
            thread.messages[0].displayContent,
            LocalProductChatMessage.toolShapedWarning
        )
        XCTAssertEqual(
            thread.profileID,
            "conversation-deepseek-deepseek-chat-r2"
        )
        XCTAssertEqual(thread.segments.first?.segmentID, "segment-1")
        XCTAssertEqual(
            thread.segments.first?.disclosureReceiptDigest,
            String(repeating: "c", count: 64)
        )
        XCTAssertEqual(thread.segments.first?.disclosedContextCount, 1)
        XCTAssertEqual(thread.segments.first?.omittedContextCount, 0)
        XCTAssertEqual(thread.segments.first?.contextTokenBudget, 2048)
        XCTAssertEqual(thread.segments.first?.contextTokenCount, 7)
        XCTAssertEqual(thread.messages.first?.segmentID, "segment-1")
        XCTAssertFalse(thread.messages[0].displayContent.contains("/private/sentinel"))
    }

    func testConversationExecutionBindingDecodesV2PolicyAndRejectsSubstitution() throws {
        let binding = """
        {
          "schema_version":3,
          "provider_id":"deepseek",
          "provider_account_id":"deepseek.work",
          "provider_account_policy_version":2,
          "provider_account_policy_revision":4,
          "provider_account_policy_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
          "trust_domain":"enterprise_tenant",
          "retention_mode":"zero_data_retention",
          "data_region":"apac"
        }
        """
        let decoded = try JSONDecoder().decode(
            LocalProductConversationExecutionBinding.self,
            from: Data(binding.utf8)
        )
        XCTAssertEqual(decoded.providerAccountID, "deepseek.work")
        XCTAssertEqual(decoded.providerAccountPolicyRevision, 4)
        XCTAssertEqual(decoded.trustDomain, "enterprise_tenant")
        XCTAssertEqual(decoded.retentionMode, "zero_data_retention")
        XCTAssertEqual(decoded.dataRegion, "apac")

        for invalid in [
            binding.replacingOccurrences(
                of: "deepseek.work", with: "openai.work"
            ),
            binding.replacingOccurrences(
                of: "zero_data_retention", with: "forever"
            ),
            binding.replacingOccurrences(
                of: String(repeating: "a", count: 64),
                with: "not-a-digest"
            ),
            binding.replacingOccurrences(
                of: "\"schema_version\":3,",
                with: "\"schema_version\":3,\"secret_preview\":\"forbidden\","
            ),
        ] {
            XCTAssertThrowsError(
                try JSONDecoder().decode(
                    LocalProductConversationExecutionBinding.self,
                    from: Data(invalid.utf8)
                )
            )
        }
    }

    func testConversationExecutionBindingV4FreezesHarnessCredentialAndModel() throws {
        let binding = """
        {
          "schema_version":4,
          "harness_adapter":"opencode",
          "provider_id":"openai",
          "provider_account_id":"openai.team",
          "credential_revision":9,
          "model_id":"openai/gpt-5.5",
          "provider_account_policy_version":0,
          "provider_account_policy_revision":0,
          "provider_account_policy_digest":"",
          "trust_domain":"",
          "retention_mode":"",
          "data_region":""
        }
        """
        let decoded = try JSONDecoder().decode(
            LocalProductConversationExecutionBinding.self,
            from: Data(binding.utf8)
        )
        XCTAssertEqual(decoded.harnessAdapter, "opencode")
        XCTAssertEqual(decoded.providerAccountID, "openai.team")
        XCTAssertEqual(decoded.credentialRevision, 9)
        XCTAssertEqual(decoded.modelID, "openai/gpt-5.5")

        for invalid in [
            binding.replacingOccurrences(
                of: "\"credential_revision\":9",
                with: "\"credential_revision\":0"
            ),
            binding.replacingOccurrences(
                of: "\"harness_adapter\":\"opencode\"",
                with: "\"harness_adapter\":\"\""
            ),
            binding.replacingOccurrences(
                of: "\"model_id\":\"openai/gpt-5.5\"",
                with: "\"model_id\":\"\""
            ),
        ] {
            XCTAssertThrowsError(
                try JSONDecoder().decode(
                    LocalProductConversationExecutionBinding.self,
                    from: Data(invalid.utf8)
                )
            )
        }
    }

    func testConversationAttemptDecodesSafeProviderFailureAndIncident() throws {
        let thread = try LocalProductWire.decodeChatThread(
            Data(
                """
                {
                  "thread_id":"thread-deepseek",
                  "profile_id":"conversation-deepseek-deepseek-chat-r6",
                  "segments":[],
                  "attempts":[{
                    "attempt_id":"attempt-1",
                    "segment_id":"segment-1",
                    "profile_id":"conversation-deepseek-deepseek-chat-r6",
                    "context_mode":"summary_only",
                    "context_capsule_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
                    "disclosure_receipt_digest":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
                    "disclosed_context_count":2,
                    "omitted_context_count":3,
                    "context_token_budget":4096,
                    "context_token_count":21,
                    "route_transition_review_digest":"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
                    "binding_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
                    "incident_id":"loom-chat-11111111-1111-4111-8111-111111111111",
                    "status":"failed",
                    "failure_code":"provider_rate_limit",
                    "failure_stage":"provider_http",
                    "http_status":429,
                    "provider_code":"rate_limit_exceeded",
                    "failure_message":"Provider rate limit reached.",
                    "retry_after_seconds":18,
                    "retryable":true,
                    "started_at":"2026-08-11T04:00:00Z",
                    "completed_at":"2026-08-11T04:00:01Z"
                  }],
                  "messages":[],
                  "can_reply":true,
                  "requires_confirmation":false
                }
                """.utf8
            )
        )

        let attempt = try XCTUnwrap(thread.attempts.first)
        XCTAssertEqual(
            attempt.disclosureReceiptDigest,
            String(repeating: "c", count: 64)
        )
        XCTAssertEqual(attempt.disclosedContextCount, 2)
        XCTAssertEqual(attempt.omittedContextCount, 3)
        XCTAssertEqual(attempt.contextTokenBudget, 4096)
        XCTAssertEqual(attempt.contextTokenCount, 21)
        XCTAssertEqual(
            attempt.routeTransitionReviewDigest,
            String(repeating: "d", count: 64)
        )
        XCTAssertEqual(attempt.incidentID, "loom-chat-11111111-1111-4111-8111-111111111111")
        XCTAssertEqual(attempt.failureCode, "provider_rate_limit")
        XCTAssertEqual(attempt.failureStage, "provider_http")
        XCTAssertEqual(attempt.httpStatus, 429)
        XCTAssertEqual(attempt.providerCode, "rate_limit_exceeded")
        XCTAssertEqual(attempt.failureMessage, "Provider rate limit reached.")
        XCTAssertEqual(attempt.retryAfterSeconds, 18)
        XCTAssertTrue(attempt.retryable)
    }

    func testConversationAttemptDecodesCompletedControlTools() throws {
        let thread = try LocalProductWire.decodeChatThread(
            Data(
                Self.chatThreadWithCompletedTools(
                    """
                    [
                      {"tool_id":"loom.runtimes.status","tool_version":1,"effect":"read"},
                      {"tool_id":"loom.missions.create.preview","tool_version":2,"effect":"proposal"}
                    ]
                    """
                ).utf8
            )
        )

        let tools = try XCTUnwrap(thread.attempts.first?.completedControlTools)
		XCTAssertEqual(thread.messages.first?.attemptID, "attempt-completed-tools")
        XCTAssertEqual(
            tools,
            [
                try LocalProductConversationCompletedTool(
                    toolID: "loom.runtimes.status",
                    toolVersion: 1,
                    effect: "read"
                ),
                try LocalProductConversationCompletedTool(
                    toolID: "loom.missions.create.preview",
                    toolVersion: 2,
                    effect: "proposal"
                ),
            ]
        )
    }

    func testCompletedControlToolStrictlyRejectsUnknownAndPrivacyFields() {
        for field in [
            "\"unexpected\":true",
            "\"arguments\":{\"mission\":\"private\"}",
            "\"result\":\"private provider output\"",
            "\"content\":\"private conversation content\"",
        ] {
            let tool = """
                [{"tool_id":"loom.runtimes.status","tool_version":1,"effect":"read",\(field)}]
                """
            XCTAssertThrowsError(
                try LocalProductWire.decodeChatThread(
                    Data(Self.chatThreadWithCompletedTools(tool).utf8)
                ),
                "accepted forbidden completed Tool field: \(field)"
            )
        }
    }

    func testCompletedControlToolRejectsInvalidEffectVersionAndID() {
        let invalidTools = [
            "[{\"tool_id\":\"loom.runtimes.status\",\"tool_version\":1,\"effect\":\"execute\"}]",
            "[{\"tool_id\":\"loom.runtimes.status\",\"tool_version\":0,\"effect\":\"read\"}]",
            "[{\"tool_id\":\"loom.runtimes.status\",\"tool_version\":2,\"effect\":\"read\"}]",
            "[{\"tool_id\":\"loom.runtimes.status\",\"tool_version\":1,\"effect\":\"proposal\"}]",
            "[{\"tool_id\":\"loom.unknown.status\",\"tool_version\":1,\"effect\":\"read\"}]",
            "[{\"tool_id\":\"Loom.Runtime.Status\",\"tool_version\":1,\"effect\":\"read\"}]",
            "[{\"tool_id\":\"loom runtime status\",\"tool_version\":1,\"effect\":\"read\"}]",
            "[{\"tool_id\":\"\",\"tool_version\":1,\"effect\":\"read\"}]",
            "[{\"tool_id\":\"\(String(repeating: "a", count: 129))\",\"tool_version\":1,\"effect\":\"read\"}]",
        ]

        for tool in invalidTools {
            XCTAssertThrowsError(
                try LocalProductWire.decodeChatThread(
                    Data(Self.chatThreadWithCompletedTools(tool).utf8)
                ),
                "accepted invalid completed Tool: \(tool)"
            )
        }
    }

    func testCompletedControlToolAdmitsExactlyTheBuiltinRegistrySurface() throws {
        let admitted: [(String, Int, String)] = [
            ("loom.sessions.search", 1, "read"),
            ("loom.sessions.align.preview", 2, "proposal"),
            ("loom.missions.create.preview", 2, "proposal"),
            ("loom.missions.continue.preview", 2, "proposal"),
            ("loom.teams.create.preview", 2, "proposal"),
            ("loom.roundtables.open.preview", 2, "proposal"),
            ("loom.missions.search", 1, "read"),
            ("loom.missions.status", 1, "read"),
            ("loom.teams.search", 1, "read"),
            ("loom.teams.status", 1, "read"),
            ("loom.roundtables.status", 1, "read"),
            ("loom.governance.needs_you", 1, "read"),
            ("loom.runtimes.status", 1, "read"),
            ("loom.providers.status", 1, "read"),
            ("loom.diagnostics.incident", 1, "read"),
            ("loom.workspace.status", 1, "read"),
            ("loom.conversation.route.status", 1, "read"),
            ("loom.library.search", 1, "read"),
            ("loom.conversation.route.change.preview", 2, "proposal"),
            ("loom.conversation.model.change.preview", 2, "proposal"),
            ("loom.conversation.reasoning.change.preview", 2, "proposal"),
            ("loom.workspace.choose.preview", 2, "proposal"),
            ("loom.teams.edit.preview", 2, "proposal"),
            ("loom.roundtables.pause.preview", 2, "proposal"),
            ("loom.roundtables.steer.preview", 2, "proposal"),
            ("loom.roundtables.retry.preview", 2, "proposal"),
            ("loom.roundtables.skip.preview", 2, "proposal"),
            ("loom.roundtables.replace.preview", 2, "proposal"),
        ]
        XCTAssertEqual(admitted.count, 28)
        for (toolID, version, effect) in admitted {
            XCTAssertNoThrow(
                try LocalProductConversationCompletedTool(
                    toolID: toolID,
                    toolVersion: version,
                    effect: effect
                )
            )
        }
    }

	func testConversationMessageRejectsInvalidAttemptIdentity() {
		let invalid = Self.chatThreadWithCompletedTools("[]")
			.replacingOccurrences(
				of: "\"attempt_id\":\"attempt-completed-tools\"",
				with: "\"attempt_id\":\"Attempt With Spaces\""
			)
		XCTAssertThrowsError(
			try LocalProductWire.decodeChatThread(Data(invalid.utf8))
		)
	}

    func testConversationAttemptRejectsMoreThanEightCompletedControlTools() {
        let tool = """
            {"tool_id":"loom.runtimes.status","tool_version":1,"effect":"read"}
            """
        let tools = "[" + Array(repeating: tool, count: 9).joined(separator: ",") + "]"

        XCTAssertThrowsError(
            try LocalProductWire.decodeChatThread(
                Data(Self.chatThreadWithCompletedTools(tools).utf8)
            )
        )
    }

    func testConversationContextDisclosureDecodesMetadataOnlyAndRejectsContent() throws {
        let source = """
        {
          "schema_version":1,
          "thread_id":"thread-deepseek",
          "segment_id":"segment-2",
          "context_capsule_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
          "disclosure_receipt_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
          "disclosed":[{
            "kind":"confirmed_user_constraint",
            "trust":"authoritative",
            "scope":"conversation_shared",
            "token_count":12,
            "omission_reason":"",
            "retrievable":false
          }],
          "omitted":[{
            "kind":"untrusted_model_output",
            "trust":"untrusted",
            "scope":"conversation_shared",
            "token_count":24,
            "omission_reason":"budget_exceeded",
            "retrievable":true
          }]
        }
        """
        let disclosure = try LocalProductWire.decodeContextDisclosure(
            Data(source.utf8)
        )
        XCTAssertEqual(disclosure.schemaVersion, 1)
        XCTAssertEqual(disclosure.threadID, "thread-deepseek")
        XCTAssertEqual(disclosure.segmentID, "segment-2")
        XCTAssertEqual(disclosure.disclosed.first?.kind, "confirmed_user_constraint")
        XCTAssertEqual(disclosure.omitted.first?.omissionReason, "budget_exceeded")
        XCTAssertTrue(disclosure.omitted.first?.retrievable ?? false)

        let withContent = source.replacingOccurrences(
            of: "\"token_count\":24,",
            with: "\"token_count\":24,\"content\":\"must-not-cross-wire\","
        )
        XCTAssertThrowsError(
            try LocalProductWire.decodeContextDisclosure(Data(withContent.utf8))
        )
    }

    func testConversationContextDisclosureDecodesUnavailableCapacityMetadata() throws {
        let source = """
        {
          "schema_version":1,
          "thread_id":"thread-deepseek",
          "segment_id":"segment-2",
          "context_capsule_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
          "disclosure_receipt_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
          "context_capacity_status":"unavailable",
          "admitted_input_budget_tokens":8192,
          "context_token_counter_id":"loom-unicode-rune-quarter-estimate",
          "context_token_counter_version":"v1",
          "admitted_contribution_tokens":12,
          "context_capacity_contributions":[{
            "priority":1,
            "source_type":"authority",
            "admitted_item_count":1,
            "admitted_token_count":12,
            "budget_omitted_item_count":0,
            "budget_omitted_token_count":0
          }],
          "disclosed":[{
            "kind":"confirmed_user_constraint",
            "trust":"authoritative",
            "scope":"conversation_shared",
            "token_count":12,
            "omission_reason":"",
            "retrievable":false
          }],
          "omitted":[]
        }
        """

        let disclosure = try LocalProductWire.decodeContextDisclosure(
            Data(source.utf8)
        )

        XCTAssertEqual(disclosure.contextCapacityStatus, .unavailable)
        XCTAssertEqual(disclosure.contextWindowTokens, 0)
        XCTAssertEqual(disclosure.admittedInputBudgetTokens, 8192)
        XCTAssertEqual(disclosure.contextTokenCounterID, "loom-unicode-rune-quarter-estimate")
        XCTAssertEqual(disclosure.admittedContributionTokens, 12)
        XCTAssertEqual(disclosure.contextCapacityContributions.count, 1)
    }

    func testChatThreadNestedObjectsRejectUnknownPrivacyFields() {
        let source = """
        {
          "thread_id":"thread-strict",
          "profile_id":"conversation-deepseek",
          "segments":[{
            "segment_id":"segment-1",
            "profile_id":"conversation-deepseek",
            "context_mode":"start_clean",
            "context_capsule_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
            "binding_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
            "created_at":"2026-08-23T00:00:00Z"
          }],
          "attempts":[{
            "attempt_id":"attempt-1",
            "segment_id":"segment-1",
            "profile_id":"conversation-deepseek",
            "context_mode":"start_clean",
            "context_capsule_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
            "binding_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
            "status":"succeeded",
            "failure_code":"",
            "retryable":false,
            "started_at":"2026-08-23T00:00:00Z",
            "completed_at":"2026-08-23T00:00:01Z"
          }],
          "messages":[{
            "message_id":"message-1",
            "segment_id":"segment-1",
            "role":"loom",
            "content":"safe visible reply",
            "tentative":false,
            "created_at":"2026-08-23T00:00:01Z"
          }],
          "can_reply":true,
          "requires_confirmation":false
        }
        """

        for poisoned in [
            source.replacingOccurrences(
                of: "\"tentative\":false,",
                with: "\"tentative\":false,\"prompt\":\"forbidden\","
            ),
            source.replacingOccurrences(
                of: "\"created_at\":\"2026-08-23T00:00:00Z\"",
                with: "\"created_at\":\"2026-08-23T00:00:00Z\",\"provider_body\":\"forbidden\""
            ),
            source.replacingOccurrences(
                of: "\"completed_at\":\"2026-08-23T00:00:01Z\"",
                with: "\"completed_at\":\"2026-08-23T00:00:01Z\",\"secret\":\"forbidden\""
            ),
        ] {
            XCTAssertThrowsError(
                try LocalProductWire.decodeChatThread(Data(poisoned.utf8))
            )
        }
    }

    func testMissionSummaryDecodesBlockReason() throws {
        let data = """
        {
          "schema_version": 1,
          "mission_id": "mission/team-blocked-1",
          "team_instance_id": "team-blocked-1",
          "title": "Blocked Mission",
          "source_kind": "team_execution",
          "lane": "Orchestrating",
          "status": "blocked",
          "priority": "normal",
          "plan_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
          "simple": true,
          "node_count": 1,
          "completed_node_count": 0,
          "active_node_count": 0,
          "review_node_count": 0,
          "attention_count": 1,
          "current_node_id": "main",
          "last_milestone": "blocked",
          "block_reason": "context_retrieval_denied",
          "team_pulse": [],
          "topology": []
        }
        """
        let decoded = try JSONDecoder().decode(
            LocalProductMissionSummary.self,
            from: Data(data.utf8)
        )
        XCTAssertEqual(decoded.status, "blocked")
        XCTAssertEqual(decoded.blockReason, "context_retrieval_denied")
    }

    func testConversationAttemptDecodesActualModelAndReasoningEffort() throws {
        let thread = try LocalProductWire.decodeChatThread(
            Data(
                """
                {
                  "thread_id":"thread-deepseek",
                  "profile_id":"conversation-deepseek-deepseek-chat-r6",
                  "segments":[],
                  "attempts":[{
                    "attempt_id":"attempt-1",
                    "segment_id":"segment-1",
                    "profile_id":"conversation-deepseek-deepseek-chat-r6",
                    "model_id":"deepseek-v4-pro",
                    "reasoning_effort":"high",
                    "context_mode":"summary_only",
                    "context_capsule_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
                    "disclosure_receipt_digest":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
                    "disclosed_context_count":1,
                    "omitted_context_count":1,
                    "binding_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
                    "status":"succeeded",
                    "failure_code":"",
                    "started_at":"2026-08-11T04:00:00Z"
                  }],
                  "messages":[],
                  "can_reply":true,
                  "requires_confirmation":false
                }
                """.utf8
            )
        )
        let attempt = try XCTUnwrap(thread.attempts.first)
        XCTAssertEqual(attempt.modelID, "deepseek-v4-pro")
        XCTAssertEqual(attempt.reasoningEffort, "high")
    }

    func testConversationAttemptDecodesMissingModelAndEffortAsNil() throws {
        let thread = try LocalProductWire.decodeChatThread(
            Data(
                """
                {
                  "thread_id":"thread-deepseek",
                  "profile_id":"conversation-deepseek-deepseek-chat-r6",
                  "segments":[],
                  "attempts":[{
                    "attempt_id":"attempt-1",
                    "segment_id":"segment-1",
                    "profile_id":"conversation-deepseek-deepseek-chat-r6",
                    "context_mode":"summary_only",
                    "context_capsule_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
                    "disclosure_receipt_digest":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
                    "disclosed_context_count":1,
                    "omitted_context_count":1,
                    "binding_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
                    "status":"succeeded",
                    "failure_code":"",
                    "started_at":"2026-08-11T04:00:00Z"
                  }],
                  "messages":[],
                  "can_reply":true,
                  "requires_confirmation":false
                }
                """.utf8
            )
        )
        let attempt = try XCTUnwrap(thread.attempts.first)
        XCTAssertNil(attempt.modelID)
        XCTAssertNil(attempt.reasoningEffort)
        XCTAssertTrue(attempt.completedControlTools.isEmpty)
    }

    func testConversationDisclosureDecodingKeepsLegacyCompatibilityAndRejectsDrift() throws {
        let legacy = """
            {
              "thread_id":"thread-legacy-disclosure",
              "profile_id":"conversation-deepseek-deepseek-chat-r3",
              "segments":[{
                "segment_id":"segment-1",
                "profile_id":"conversation-deepseek-deepseek-chat-r3",
                "context_mode":"start_clean",
                "context_capsule_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
                "binding_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
              }],
              "attempts":[{
                "attempt_id":"attempt-1",
                "segment_id":"segment-1",
                "profile_id":"conversation-deepseek-deepseek-chat-r3",
                "context_mode":"start_clean",
                "context_capsule_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
                "binding_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
                "status":"succeeded",
                "failure_code":""
              }],
              "messages":[],
              "can_reply":true,
              "requires_confirmation":false
            }
            """
        let decoded = try LocalProductWire.decodeChatThread(Data(legacy.utf8))
        XCTAssertEqual(decoded.segments.first?.disclosureReceiptDigest, "")
        XCTAssertEqual(decoded.segments.first?.disclosedContextCount, 0)
        XCTAssertEqual(decoded.segments.first?.contextTokenBudget, 0)
        XCTAssertEqual(decoded.segments.first?.contextTokenCount, 0)
        XCTAssertEqual(decoded.attempts.first?.omittedContextCount, 0)

        let missingReceipt = legacy.replacingOccurrences(
            of: "\"binding_digest\":\"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\"",
            with: "\"disclosed_context_count\":1,\"binding_digest\":\"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\""
        )
        XCTAssertThrowsError(
            try LocalProductWire.decodeChatThread(Data(missingReceipt.utf8))
        )

        let zeroDisclosed = legacy.replacingOccurrences(
            of: "\"binding_digest\":\"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\"",
            with: "\"disclosure_receipt_digest\":\"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc\",\"binding_digest\":\"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\""
        )
        XCTAssertThrowsError(
            try LocalProductWire.decodeChatThread(Data(zeroDisclosed.utf8))
        )

        let invalidTokenProjection = legacy.replacingOccurrences(
            of: "\"binding_digest\":\"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\"",
            with: "\"context_token_budget\":1,\"context_token_count\":2,\"binding_digest\":\"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\""
        )
        XCTAssertThrowsError(
            try LocalProductWire.decodeChatThread(Data(invalidTokenProjection.utf8))
        )
    }

    func testConversationCapacityProjectionDecodesExactSegmentAndAttempt() throws {
        let thread = try LocalProductWire.decodeChatThread(
            Data(Self.capacityThreadJSON(status: "exact", window: 128_000).utf8)
        )
        let segment = try XCTUnwrap(thread.segments.first)
        let attempt = try XCTUnwrap(thread.attempts.first)

        XCTAssertEqual(segment.contextCapacityStatus, .exact)
        XCTAssertEqual(segment.contextWindowTokens, 128_000)
        XCTAssertEqual(segment.reservedOutputTokens, 8_192)
        XCTAssertEqual(segment.adapterToolOverheadTokens, 1_024)
        XCTAssertEqual(segment.admittedInputBudgetTokens, 118_784)
        XCTAssertEqual(segment.contextTokenCounterID, "loom-token-counter")
        XCTAssertEqual(segment.contextTokenCounterVersion, "v1")
        XCTAssertEqual(segment.admittedContributionTokens, 3_000)
        XCTAssertEqual(segment.budgetOmittedContributionTokens, 400)
        XCTAssertEqual(segment.contextCapacityContributions.count, 2)
        XCTAssertEqual(segment.contextCapacityContributions[1].priority, 1)
        XCTAssertEqual(segment.contextCapacityContributions[1].sourceType, "observation")
        XCTAssertEqual(attempt.contextCapacityStatus, .exact)
        XCTAssertEqual(attempt.contextCapacityContributions, segment.contextCapacityContributions)
    }

    func testConversationCapacityProjectionDecodesEstimatedAndUnavailable() throws {
        let estimated = try LocalProductWire.decodeChatThread(
            Data(Self.capacityThreadJSON(status: "estimated", window: 128_000).utf8)
        )
        XCTAssertEqual(estimated.segments.first?.contextCapacityStatus, .estimated)
        XCTAssertEqual(estimated.segments.first?.admittedInputBudgetTokens, 118_784)

        let unavailableJSON = Self.capacityThreadJSON(
            status: "unavailable",
            window: nil,
            reservedOutput: 0,
            adapterOverhead: 0,
            admittedBudget: 120_000
        )
        let unavailable = try LocalProductWire.decodeChatThread(Data(unavailableJSON.utf8))
        let unavailableSegment = try XCTUnwrap(unavailable.segments.first)
        XCTAssertEqual(unavailableSegment.contextCapacityStatus, .unavailable)
        XCTAssertEqual(unavailableSegment.contextWindowTokens, 0)
        XCTAssertEqual(unavailableSegment.contextTokenBudget, 120_000)
        XCTAssertEqual(unavailableSegment.admittedInputBudgetTokens, 120_000)

        let goUnavailableWire = unavailableJSON
            .replacingOccurrences(of: "\"omitted_context_count\":1", with: "\"omitted_context_count\":0")
            .replacingOccurrences(of: "\"reserved_output_tokens\":0,", with: "")
            .replacingOccurrences(of: "\"adapter_tool_overhead_tokens\":0,", with: "")
            .replacingOccurrences(of: "\"budget_omitted_contribution_tokens\":400,", with: "")
            .replacingOccurrences(of: "\"budget_omitted_item_count\":1,\"budget_omitted_token_count\":400", with: "\"budget_omitted_item_count\":0,\"budget_omitted_token_count\":0")
        XCTAssertNoThrow(
            try LocalProductWire.decodeChatThread(Data(goUnavailableWire.utf8)),
            "Swift must accept zero-valued capacity fields omitted by Go"
        )

        let goEstimatedZeroWire = Self.capacityThreadJSON(
            status: "estimated",
            window: 128_000,
            reservedOutput: 0,
            adapterOverhead: 0,
            admittedBudget: 120_000
        )
            .replacingOccurrences(of: "\"omitted_context_count\":1", with: "\"omitted_context_count\":0")
            .replacingOccurrences(of: "\"reserved_output_tokens\":0,", with: "")
            .replacingOccurrences(of: "\"adapter_tool_overhead_tokens\":0,", with: "")
            .replacingOccurrences(of: "\"budget_omitted_contribution_tokens\":400,", with: "")
            .replacingOccurrences(of: "\"budget_omitted_item_count\":1,\"budget_omitted_token_count\":400", with: "\"budget_omitted_item_count\":0,\"budget_omitted_token_count\":0")
        XCTAssertNoThrow(
            try LocalProductWire.decodeChatThread(Data(goEstimatedZeroWire.utf8)),
            "Estimated capacity must accept omitted semantic zeros"
        )

        let explicitZeroWindow = Self.capacityThreadJSON(
            status: "unavailable",
            window: 0,
            reservedOutput: 0,
            adapterOverhead: 0,
            admittedBudget: 120_000
        )
        XCTAssertNoThrow(
            try LocalProductWire.decodeChatThread(Data(explicitZeroWindow.utf8))
        )
        let inventedWindow = explicitZeroWindow.replacingOccurrences(
            of: "\"context_window_tokens\":0",
            with: "\"context_window_tokens\":128000"
        )
        XCTAssertThrowsError(
            try LocalProductWire.decodeChatThread(Data(inventedWindow.utf8))
        )
    }

    func testConversationCapacityProjectionAllows512AggregateItemsAndPolicyOmissions() throws {
        let aggregate512 = Self.capacityThreadJSON(status: "exact", window: 128_000)
            .replacingOccurrences(
                of: "\"disclosed_context_count\":2,\"omitted_context_count\":1",
                with: "\"disclosed_context_count\":300,\"omitted_context_count\":212"
            )
            .replacingOccurrences(
                of: "\"admitted_item_count\":1,\"admitted_token_count\":1000",
                with: "\"admitted_item_count\":150,\"admitted_token_count\":1000"
            )
            .replacingOccurrences(
                of: "\"admitted_item_count\":1,\"admitted_token_count\":2000,\"budget_omitted_item_count\":1",
                with: "\"admitted_item_count\":150,\"admitted_token_count\":2000,\"budget_omitted_item_count\":212"
            )
        XCTAssertNoThrow(
            try LocalProductWire.decodeChatThread(Data(aggregate512.utf8))
        )

        let includesPolicyOmission = Self.capacityThreadJSON(
            status: "exact", window: 128_000
        ).replacingOccurrences(
            of: "\"disclosed_context_count\":2,\"omitted_context_count\":1",
            with: "\"disclosed_context_count\":2,\"omitted_context_count\":2"
        )
        XCTAssertNoThrow(
            try LocalProductWire.decodeChatThread(Data(includesPolicyOmission.utf8)),
            "Capacity contributions must not require policy/access omissions"
        )
    }

    func testConversationCapacityProjectionRejectsPartialAndTamperedAuthority() {
        let exact = Self.capacityThreadJSON(status: "exact", window: 128_000)
        let malformed = [
            exact.replacingOccurrences(of: "\"context_capacity_status\":\"exact\",", with: ""),
            exact.replacingOccurrences(of: "\"reserved_output_tokens\":8192,", with: ""),
            exact.replacingOccurrences(of: "\"reserved_output_tokens\":8192", with: "\"reserved_output_tokens\":null"),
            exact.replacingOccurrences(of: "\"context_capacity_status\":\"exact\"", with: "\"context_capacity_status\":\"guessed\""),
            exact.replacingOccurrences(of: "\"context_window_tokens\":128000", with: "\"context_window_tokens\":0"),
            exact.replacingOccurrences(of: "\"admitted_input_budget_tokens\":118784", with: "\"admitted_input_budget_tokens\":118785"),
            exact.replacingOccurrences(of: "\"admitted_contribution_tokens\":3000", with: "\"admitted_contribution_tokens\":3001"),
            exact.replacingOccurrences(of: "\"budget_omitted_contribution_tokens\":400", with: "\"budget_omitted_contribution_tokens\":401"),
            exact.replacingOccurrences(of: "\"admitted_token_count\":2000", with: "\"admitted_token_count\":2001"),
            exact.replacingOccurrences(
                of: "\"priority\":1,\"source_type\":\"observation\"",
                with: "\"priority\":0,\"source_type\":\"authority\""
            ),
            exact.replacingOccurrences(of: "\"source_type\":\"observation\"", with: "\"source_type\":\"prompt\""),
        ]
        for value in malformed {
            XCTAssertThrowsError(
                try LocalProductWire.decodeChatThread(Data(value.utf8)),
                "Capacity authority drift must fail closed"
            )
        }
    }

    func testChatThreadDecodesSafeMigrationAvailabilityFailure() throws {
        let thread = try LocalProductWire.decodeChatThread(
            Data(
                """
                {
                  "thread_id":"thread-migration",
                  "profile_id":"",
                  "segments":[],
                  "attempts":[],
                  "messages":[],
                  "can_reply":false,
                  "requires_confirmation":false,
                  "availability_failure":{
                    "code":"state_unavailable",
                    "stage":"migration_commit",
                    "incident_id":"loom-migration-11111111-1111-4111-8111-111111111111",
                    "retryable":false
                  }
                }
                """.utf8
            )
        )

        XCTAssertEqual(thread.availabilityFailure?.code, .stateUnavailable)
        XCTAssertEqual(thread.availabilityFailure?.stage, .migrationCommit)
        XCTAssertEqual(
            thread.availabilityFailure?.incidentID,
            "loom-migration-11111111-1111-4111-8111-111111111111"
        )
        XCTAssertFalse(thread.availabilityFailure?.retryable ?? true)
    }

    func testChatThreadRejectsUnsafeMigrationAvailabilityFailure() {
        let valid = """
            {
              "thread_id":"thread-migration",
              "profile_id":"",
              "segments":[],
              "attempts":[],
              "messages":[],
              "can_reply":false,
              "requires_confirmation":false,
              "availability_failure":{
                "code":"state_unavailable",
                "stage":"migration_read",
                "incident_id":"loom-migration-11111111-1111-4111-8111-111111111111",
                "retryable":false
              }
            }
            """
        let invalid = [
            valid.replacingOccurrences(
                of: "\"code\":\"state_unavailable\"",
                with: "\"code\":\"provider_unavailable\""
            ),
            valid.replacingOccurrences(
                of: "\"stage\":\"migration_read\"",
                with: "\"stage\":\"conversation_dispatch\""
            ),
            valid.replacingOccurrences(
                of: "\"incident_id\":\"loom-migration-11111111-1111-4111-8111-111111111111\"",
                with: "\"incident_id\":\"invalid incident\""
            ),
            valid.replacingOccurrences(
                of: "\"can_reply\":false",
                with: "\"can_reply\":true"
            ),
        ]
        for source in invalid {
            XCTAssertThrowsError(
                try LocalProductWire.decodeChatThread(Data(source.utf8))
            )
        }
    }

    func testStrictTimelineDecodingRejectsNestedUnknownKeys() {
        let invalid = """
        {
          "schema_version":1,
          "team_instance_id":"team-1",
          "view_version":"view-1",
          "next_cursor":"",
          "has_more":false,
          "gap":null,
          "records":[],
          "board":{
            "schema_version":1,
            "team_instance_id":"team-1",
            "plan_digest":"",
            "status":"ready",
            "view_version":"view-1",
            "nodes":[],
            "cost":{"observed":false,"amount_microunits":null,"currency":""},
            "unexpected":true
          },
          "attention":[]
        }
        """
        XCTAssertThrowsError(
            try LocalProductWire.decodeTimeline(Data(invalid.utf8))
        )
    }

    func testPhase2DTimelineNodeDecodesIndependentAgentBinding() throws {
        let timeline = try LocalProductWire.decodeTimeline(Data("""
        {
          "schema_version":1,"team_instance_id":"team-1","view_version":"view-1",
          "next_cursor":"","has_more":false,"gap":null,"records":[],
          "board":{"schema_version":1,"team_instance_id":"team-1","plan_digest":"",
            "status":"blocked","view_version":"view-1","nodes":[{
              "logical_node_id":"main","status":"blocked","dependency_satisfied":false,
              "current_attempt":1,"work_item_id":"work-1","run_id":"run-1",
              "runtime_instance_id":"runtime-1","agent_instance_id":"agent-1",
              "verification_status":"","recovery_action":"inspect_failure","retry_at":"",
              "fallback_configured":true,"recovery_approval_required":true,
              "fallback_approval_available":true,"fallback_approval_version":3,
              "fallback_consumed":false,
              "execution_binding_available":true,"harness_adapter":"claude-code",
              "provider_id":"anthropic","provider_account_id":"anthropic.production",
              "model_id":"anthropic/claude-sonnet","reasoning_effort":"high",
              "timeout_nanoseconds":300000000000,"binding_budget_credits":1200,
              "capabilities":["reasoning_effort","workspace_edit"],
              "credential_revision":7,
              "context_capsule_available":true,
              "context_capsule_digest":"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
              "route_segment_available":true,
              "route_segment_id":"segment-main-1",
              "route_segment_digest":"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
              "context_disclosure_receipt_digest":"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
              "context_adapter_id":"context:claude-code:v1",
              "disclosure_policy_id":"policy.production",
              "disclosure_policy_version":3,
              "context_token_budget":128,"context_token_count":8,
              "context_disclosed_count":1,"context_omission_count":2,
              "provider_account_policy_available":true,
              "provider_account_policy_version":2,
              "provider_account_policy_revision":3,
              "provider_account_policy_digest":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
              "provider_account_trust_domain":"external_provider",
              "provider_account_retention_mode":"limited_retention",
              "provider_account_data_region":"eu",
              "provider_account_assigned_budget_units":1000,
              "terminal_reason":"provider_rejected",
              "incident_id":"22222222-2222-4222-8222-222222222222",
              "failure_diagnostic_available":true,
              "failure_stage":"provider_auth","failure_code":"provider_rejected",
              "failure_retryable":false,
              "test_report_available":true,"test_report_count":2,
              "test_report_passed_count":1,"test_report_failed_count":1,
              "test_report_set_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
              "latest_test_runner":"go_test","latest_test_scope":"all",
              "latest_test_outcome":"passed",
              "latest_test_report_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
              "accounting_available":true,
              "usage_observed":true,"input_tokens":90,"output_tokens":10,
              "cache_read_tokens":0,"cache_write_tokens":0,"total_tokens":100,
              "cost_observed":true,"cost_microunits":450,"cost_currency":"USD",
              "cost_source":"harness_reported"
            }],"provider_accounts":[{
              "provider_id":"anthropic","provider_account_id":"anthropic.production",
              "active_attempts":0,"attempt_count":1,"failed_attempts":1,
              "rate_limited_attempts":0,"error_rate_basis_points":10000,
              "budget_attempt_count":1,"budget_units":1000,
              "policy_available":true,"policy_revision":3,
              "policy_digest":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
              "maximum_concurrent_attempts":2,"dispatch_window_seconds":60,
              "maximum_dispatch_starts":10,"maximum_assigned_budget_units":5000,
              "active_assigned_budget_units":0,
              "accounting_attempt_count":1,"usage_attempt_count":1,
              "input_tokens":90,"output_tokens":10,"cache_read_tokens":0,
              "cache_write_tokens":0,"total_tokens":100,"cost_attempt_count":1,
              "costs":[{"currency":"USD","source":"harness_reported","amount_microunits":450}],
              "aggregation_overflow":false
            }],"cost":{"observed":false,"amount_microunits":null,"currency":""}},
          "attention":[]
        }
        """.utf8))

        let node = try XCTUnwrap(timeline.board.nodes.first)
        XCTAssertTrue(node.executionBindingAvailable)
        XCTAssertEqual(node.harnessAdapter, "claude-code")
        XCTAssertEqual(node.providerAccountID, "anthropic.production")
        XCTAssertEqual(node.modelID, "anthropic/claude-sonnet")
        XCTAssertEqual(node.reasoningEffort, "high")
        XCTAssertEqual(node.timeoutNanoseconds, 300_000_000_000)
        XCTAssertEqual(node.bindingBudgetCredits, 1_200)
        XCTAssertEqual(node.capabilities, ["reasoning_effort", "workspace_edit"])
        XCTAssertEqual(node.credentialRevision, 7)
        XCTAssertTrue(node.contextCapsuleAvailable)
        XCTAssertTrue(node.routeSegmentAvailable)
        XCTAssertEqual(node.routeSegmentID, "segment-main-1")
        XCTAssertEqual(
            node.routeSegmentDigest,
            "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
        )
        XCTAssertEqual(node.contextAdapterID, "context:claude-code:v1")
        XCTAssertEqual(node.disclosurePolicyID, "policy.production")
        XCTAssertEqual(node.disclosurePolicyVersion, 3)
        XCTAssertEqual(node.contextTokenBudget, 128)
        XCTAssertEqual(node.contextTokenCount, 8)
        XCTAssertEqual(node.contextDisclosedCount, 1)
        XCTAssertEqual(node.contextOmissionCount, 2)
        XCTAssertTrue(node.providerAccountPolicyAvailable)
        XCTAssertEqual(node.providerAccountPolicyVersion, 2)
        XCTAssertEqual(node.providerAccountPolicyRevision, 3)
        XCTAssertEqual(node.providerAccountTrustDomain, "external_provider")
        XCTAssertEqual(node.providerAccountRetentionMode, "limited_retention")
        XCTAssertEqual(node.providerAccountDataRegion, "eu")
        XCTAssertEqual(node.providerAccountAssignedBudgetUnits, 1_000)
        XCTAssertEqual(node.terminalReason, "provider_rejected")
	        XCTAssertEqual(node.incidentID, "22222222-2222-4222-8222-222222222222")
        XCTAssertTrue(node.failureDiagnosticAvailable)
        XCTAssertEqual(node.failureStage, "provider_auth")
        XCTAssertEqual(node.failureCode, "provider_rejected")
        XCTAssertTrue(node.testReportAvailable)
        XCTAssertEqual(node.testReportCount, 2)
        XCTAssertEqual(node.testReportPassedCount, 1)
        XCTAssertEqual(node.testReportFailedCount, 1)
        XCTAssertEqual(node.latestTestRunner, "go_test")
        XCTAssertEqual(node.latestTestScope, "all")
        XCTAssertEqual(node.latestTestOutcome, "passed")
        XCTAssertFalse(node.failureRetryable)
        XCTAssertTrue(node.fallbackConfigured)
        XCTAssertTrue(node.recoveryApprovalRequired)
        XCTAssertTrue(node.fallbackApprovalAvailable)
        XCTAssertEqual(node.fallbackApprovalVersion, 3)
        XCTAssertFalse(node.fallbackConsumed)
        XCTAssertTrue(node.accountingAvailable)
        XCTAssertEqual(node.totalTokens, 100)
        XCTAssertTrue(node.costObserved)
        XCTAssertEqual(node.costMicrounits, 450)
        XCTAssertEqual(node.costCurrency, "USD")
		XCTAssertEqual(node.costSource, "harness_reported")
        let encoded = try XCTUnwrap(
            String(data: JSONEncoder().encode(timeline), encoding: .utf8)
        )
        for (source, replacement) in [
            (
                "\"provider_account_policy_version\":2",
                "\"provider_account_policy_version\":1"
            ),
            (
                "\"provider_account_trust_domain\":\"external_provider\"",
                "\"provider_account_trust_domain\":\"unknown\""
            ),
            (
                "\"provider_account_retention_mode\":\"limited_retention\"",
                "\"provider_account_retention_mode\":\"forever\""
            ),
            (
                "\"provider_account_data_region\":\"eu\"",
                "\"provider_account_data_region\":\"moon\""
            ),
        ] {
            XCTAssertThrowsError(
                try LocalProductWire.decodeTimeline(Data(
                    encoded.replacingOccurrences(
                        of: source,
                        with: replacement
                    ).utf8
                )),
                "Attempt disclosure substitution must fail closed: \(replacement)"
            )
        }
        let account = try XCTUnwrap(timeline.board.providerAccounts.first)
        XCTAssertEqual(account.providerAccountID, "anthropic.production")
        XCTAssertEqual(account.errorRateBasisPoints, 10_000)
        XCTAssertTrue(account.policyAvailable)
        XCTAssertEqual(account.policyRevision, 3)
        XCTAssertEqual(account.maximumConcurrentAttempts, 2)
        XCTAssertEqual(account.maximumAssignedBudgetUnits, 5_000)
        XCTAssertEqual(account.totalTokens, 100)
        XCTAssertEqual(account.costs.first?.amountMicrounits, 450)
		XCTAssertEqual(account.costs.first?.source, "harness_reported")

        let inconsistentFallback = """
        {
          "logical_node_id":"main","status":"blocked",
          "dependency_satisfied":false,"current_attempt":1,
          "work_item_id":"work-1","run_id":"run-1",
          "runtime_instance_id":"runtime-1","agent_instance_id":"agent-1",
          "verification_status":"","recovery_action":"inspect_failure","retry_at":"",
          "fallback_configured":false,"recovery_approval_required":true,
          "fallback_approval_available":true,"fallback_approval_version":0,
          "fallback_consumed":false
        }
        """
        XCTAssertThrowsError(
            try JSONDecoder().decode(
                LocalProductNode.self,
                from: Data(inconsistentFallback.utf8)
            )
        )

        let unsafeIncident = """
        {
          "schema_version":1,"team_instance_id":"team-1","view_version":"view-1",
          "next_cursor":"","has_more":false,"gap":null,"records":[],
          "board":{"schema_version":1,"team_instance_id":"team-1","plan_digest":"",
            "status":"blocked","view_version":"view-1","nodes":[{
              "logical_node_id":"main","status":"blocked","dependency_satisfied":false,
              "current_attempt":1,"work_item_id":"work-1","run_id":"run-1",
              "runtime_instance_id":"runtime-1","agent_instance_id":"agent-1",
              "verification_status":"","recovery_action":"inspect_failure","retry_at":"",
              "incident_id":"unsafe incident"
            }],"provider_accounts":[],
            "cost":{"observed":false,"amount_microunits":null,"currency":""}},
          "attention":[]
        }
        """
        XCTAssertThrowsError(
            try LocalProductWire.decodeTimeline(Data(unsafeIncident.utf8))
        )
    }

    func testPhase2DInitialBlockedAgentDecodesRouteWithoutAttempt() throws {
        let node = try JSONDecoder().decode(LocalProductNode.self, from: Data("""
        {
          "logical_node_id":"reviewer","node_kind":"route_sibling",
          "route_group_id":"route-group-reviewer","status":"blocked",
          "dependency_satisfied":false,"current_attempt":0,
          "work_item_id":"","run_id":"","runtime_instance_id":"",
          "agent_instance_id":"","verification_status":"",
          "recovery_action":"","retry_at":"",
          "execution_binding_available":true,"harness_adapter":"loom-native",
          "provider_id":"deepseek","provider_account_id":"deepseek.primary",
          "model_id":"deepseek-chat","reasoning_effort":"",
          "timeout_nanoseconds":60000000000,"binding_budget_credits":250,
          "capabilities":["chat"],"credential_revision":6,
          "terminal_reason":"Credential is not verified.",
          "incident_id":"44444444-4444-4444-8444-444444444444",
          "failure_diagnostic_available":true,
          "failure_stage":"credential_lease_issue",
          "failure_code":"credential_unavailable","failure_retryable":true,
          "accounting_available":false,"usage_observed":false,
          "input_tokens":0,"output_tokens":0,"cache_read_tokens":0,
          "cache_write_tokens":0,"total_tokens":0,"cost_observed":false,
          "cost_microunits":0,"cost_currency":""
        }
        """.utf8))

        XCTAssertEqual(node.currentAttempt, 0)
        XCTAssertEqual(node.nodeKind, "route_sibling")
        XCTAssertEqual(node.routeGroupID, "route-group-reviewer")
        XCTAssertEqual(node.providerAccountID, "deepseek.primary")
        XCTAssertEqual(node.credentialRevision, 6)
        XCTAssertEqual(node.failureStage, "credential_lease_issue")
        XCTAssertTrue(node.failureRetryable)

        let source = String(data: try JSONEncoder().encode(node), encoding: .utf8)!
        XCTAssertThrowsError(
            try JSONDecoder().decode(
                LocalProductNode.self,
                from: Data(source.replacingOccurrences(
                    of: "\"node_kind\":\"route_sibling\"",
                    with: "\"node_kind\":\"unknown\""
                ).utf8)
            )
        )
        XCTAssertThrowsError(
            try JSONDecoder().decode(
                LocalProductNode.self,
                from: Data(source.replacingOccurrences(
                    of: "\"route_group_id\":\"route-group-reviewer\"",
                    with: "\"route_group_id\":\"\""
                ).utf8)
            )
        )
    }

    func testPhase2DBoardRejectsContradictoryBindingAndAccounting() throws {
        let validNode = """
        {
          "logical_node_id":"main","status":"failed",
          "dependency_satisfied":true,"current_attempt":1,
          "work_item_id":"work-1","run_id":"run-1",
          "runtime_instance_id":"runtime-1","agent_instance_id":"agent-1",
          "verification_status":"","recovery_action":"inspect_failure","retry_at":"",
          "execution_binding_available":true,"harness_adapter":"loom-native",
          "provider_id":"deepseek","provider_account_id":"deepseek.primary",
          "model_id":"deepseek-chat","reasoning_effort":"",
          "timeout_nanoseconds":45000000000,"binding_budget_credits":500,
          "capabilities":["workspace_edit"],"credential_revision":6,
          "incident_id":"33333333-3333-4333-8333-333333333333",
          "failure_diagnostic_available":true,
          "failure_stage":"provider_auth","failure_code":"provider_rejected",
          "failure_retryable":false,
          "test_report_available":true,"test_report_count":2,
          "test_report_passed_count":2,"test_report_failed_count":0,
          "test_report_set_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
          "latest_test_runner":"go_test","latest_test_scope":"all",
          "latest_test_outcome":"passed",
          "latest_test_report_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
          "accounting_available":true,"usage_observed":true,
          "input_tokens":90,"output_tokens":10,"cache_read_tokens":0,
          "cache_write_tokens":0,"total_tokens":100,"cost_observed":true,
          "cost_microunits":450,"cost_currency":"USD",
          "cost_source":"provider_reported"
        }
        """
        XCTAssertNoThrow(
            try JSONDecoder().decode(LocalProductNode.self, from: Data(validNode.utf8))
        )

        for invalid in [
            validNode.replacingOccurrences(
                of: "\"harness_adapter\":\"loom-native\"",
                with: "\"harness_adapter\":\"\""
            ),
            validNode.replacingOccurrences(
                of: "\"test_report_available\":true",
                with: "\"test_report_available\":false"
            ),
            validNode.replacingOccurrences(
                of: "\"provider_account_id\":\"deepseek.primary\"",
                with: "\"provider_account_id\":\"anthropic.primary\""
            ),
            validNode.replacingOccurrences(
                of: "\"total_tokens\":100",
                with: "\"total_tokens\":101"
            ),
            validNode.replacingOccurrences(
                of: "\"failure_stage\":\"provider_auth\"",
                with: "\"failure_stage\":\"provider_secret_body\""
            ),
            validNode.replacingOccurrences(
                of: "\"failure_diagnostic_available\":true",
                with: "\"failure_diagnostic_available\":false"
            ),
            validNode.replacingOccurrences(
                of: "\"input_tokens\":90",
                with: "\"input_tokens\":-1"
            ),
            validNode.replacingOccurrences(
                of: "\"accounting_available\":true",
                with: "\"accounting_available\":false"
            ),
			validNode.replacingOccurrences(
				of: "\"cost_source\":\"provider_reported\"",
				with: "\"cost_source\":\"unknown\""
			),
			validNode.replacingOccurrences(
				of: "\"cost_source\":\"provider_reported\"",
				with: "\"cost_source\":\"rate_card_estimate\""
			),
            validNode.replacingOccurrences(
                of: "\"test_report_passed_count\":2",
                with: "\"test_report_passed_count\":1"
            ),
            validNode.replacingOccurrences(
                of: "\"latest_test_runner\":\"go_test\"",
                with: "\"latest_test_runner\":\"shell\""
            ),
            validNode.replacingOccurrences(
                of: "\"test_report_set_digest\":\"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\"",
                with: "\"test_report_set_digest\":\"private test output\""
            ),
            validNode.replacingOccurrences(
                of: "\"execution_binding_available\":true",
                with: "\"execution_binding_available\":false"
            ),
        ] {
            XCTAssertThrowsError(
                try JSONDecoder().decode(LocalProductNode.self, from: Data(invalid.utf8))
            )
        }

        let estimatedNode = validNode
            .replacingOccurrences(
                of: "\"cost_source\":\"provider_reported\"",
                with: "\"cost_source\":\"rate_card_estimate\""
            )
            .replacingOccurrences(
                of: "\"cost_microunits\":450,\"cost_currency\":\"USD\"",
                with: "\"cost_microunits\":450,\"cost_currency\":\"USD\"," +
                    "\"provider_model_rate_card_available\":true," +
                    "\"provider_model_rate_card_revision\":3," +
                    "\"provider_model_rate_card_digest\":\"" +
                    String(repeating: "a", count: 64) + "\"," +
                    "\"provider_model_rate_card_currency\":\"USD\"," +
                    "\"provider_model_rate_card_input_basis\":\"input_includes_cache\""
            )
        XCTAssertNoThrow(
            try JSONDecoder().decode(LocalProductNode.self, from: Data(estimatedNode.utf8))
        )
        XCTAssertThrowsError(
            try JSONDecoder().decode(
                LocalProductNode.self,
                from: Data(
                    estimatedNode.replacingOccurrences(
                        of: "\"provider_model_rate_card_currency\":\"USD\"",
                        with: "\"provider_model_rate_card_currency\":\"EUR\""
                    ).utf8
                )
            )
        )

        let inconsistentAccount = """
        {
          "provider_id":"deepseek","provider_account_id":"deepseek.primary",
          "active_attempts":0,"attempt_count":2,"failed_attempts":1,
          "rate_limited_attempts":0,"error_rate_basis_points":9000,
          "budget_attempt_count":1,"budget_units":100,
          "accounting_attempt_count":1,"usage_attempt_count":1,
          "input_tokens":90,"output_tokens":10,"cache_read_tokens":0,
          "cache_write_tokens":0,"total_tokens":100,"cost_attempt_count":1,
          "costs":[{"currency":"USD","source":"provider_reported","amount_microunits":450}],
          "aggregation_overflow":false
        }
        """
        XCTAssertThrowsError(
            try JSONDecoder().decode(
                LocalProductProviderAccountAccounting.self,
                from: Data(inconsistentAccount.utf8)
            )
        )
    }

    func testAgentAttemptRestartRecoveryStageIsStrictlyRecognized() {
        XCTAssertEqual(
            LocalIPCRemoteError.Stage(rawValue: "agent_attempt_reconcile"),
            .agentAttemptReconcile
        )
        XCTAssertNil(LocalIPCRemoteError.Stage(rawValue: "agent_attempt_resume"))
    }

    static let snapshotJSON = """
    {
      "schema_version":1,
      "view_version":"view-1",
      "partial":false,
      "stale":false,
      "reason":"",
      "runtimes":[{
        "runtime_instance_id":"pi-local",
        "display_name":"Pi",
        "adapter_type":"pi",
        "executable_version":"0.82.1",
        "status":"online",
        "capacity":2,
        "model_ids":["qwen"],
        "observed_capabilities":["streaming"]
      }],
      "teams":[],
      "runs":[],
      "evidence":[],
      "attention":[],
      "runtime_page":{"next_cursor":"pi-local","has_more":false},
      "team_page":{"next_cursor":"","has_more":false},
      "run_page":{"next_cursor":"","has_more":false},
      "evidence_page":{"next_cursor":"","has_more":false}
    }
    """

    static func chatThreadWithCompletedTools(_ completedTools: String) -> String {
        """
        {
          "thread_id":"thread-completed-tools",
          "profile_id":"conversation-deepseek-deepseek-chat-r6",
          "segments":[],
          "attempts":[{
            "attempt_id":"attempt-completed-tools",
            "segment_id":"segment-completed-tools",
            "profile_id":"conversation-deepseek-deepseek-chat-r6",
            "context_mode":"start_clean",
            "context_capsule_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
            "completed_control_tools":\(completedTools),
            "binding_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
            "status":"succeeded",
            "failure_code":"",
            "retryable":false,
            "started_at":"2026-08-29T00:00:00Z",
            "completed_at":"2026-08-29T00:00:01Z"
          }],
          "messages":[{
            "message_id":"message-completed-tools",
            "segment_id":"segment-completed-tools",
            "attempt_id":"attempt-completed-tools",
            "role":"loom",
            "content":"Status checked.",
            "tentative":true,
            "created_at":"2026-08-29T00:00:01Z"
          }],
          "can_reply":true,
          "requires_confirmation":false
        }
        """
    }

    static let snapshotV2JSON = """
    {
      "schema_version":2,
      "view_version":"view-3",
      "partial":false,
      "stale":false,
      "reason":"",
      "health":{"daemon":"serving_request","journal":"available","projection":"current"},
      "runtimes":[],
      "teams":[],
      "missions":[],
      "runs":[],
      "evidence":[],
      "attention":[],
      "prepared_decisions":[],
      "runtime_page":{"next_cursor":"","has_more":false},
      "team_page":{"next_cursor":"","has_more":false},
      "mission_page":{"next_cursor":"","has_more":false},
      "run_page":{"next_cursor":"","has_more":false},
      "evidence_page":{"next_cursor":"","has_more":false}
    }
    """

    static func capacityThreadJSON(
        status: String,
        window: Int?,
        reservedOutput: Int = 8_192,
        adapterOverhead: Int = 1_024,
        admittedBudget: Int = 118_784
    ) -> String {
        let windowField = window.map { "\"context_window_tokens\":\($0)," } ?? ""
        let capacity = """
            "context_capacity_status":"\(status)",
            \(windowField)
            "reserved_output_tokens":\(reservedOutput),
            "adapter_tool_overhead_tokens":\(adapterOverhead),
            "admitted_input_budget_tokens":\(admittedBudget),
            "context_token_counter_id":"loom-token-counter",
            "context_token_counter_version":"v1",
            "admitted_contribution_tokens":3000,
            "budget_omitted_contribution_tokens":400,
            "context_capacity_contributions":[
              {"priority":0,"source_type":"authority","admitted_item_count":1,"admitted_token_count":1000,"budget_omitted_item_count":0,"budget_omitted_token_count":0},
              {"priority":1,"source_type":"observation","admitted_item_count":1,"admitted_token_count":2000,"budget_omitted_item_count":1,"budget_omitted_token_count":400}
            ],
            """
        return """
            {
              "thread_id":"thread-capacity",
              "profile_id":"conversation-deepseek",
              "segments":[{
                "segment_id":"segment-1","profile_id":"conversation-deepseek",
                "context_mode":"summary_only",
                "context_capsule_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
                "disclosure_receipt_digest":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
                "disclosed_context_count":2,"omitted_context_count":1,
                "context_token_budget":120000,"context_token_count":3000,
                \(capacity)
                "binding_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
              }],
              "attempts":[{
                "attempt_id":"attempt-1","segment_id":"segment-1","profile_id":"conversation-deepseek",
                "context_mode":"summary_only",
                "context_capsule_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
                "disclosure_receipt_digest":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
                "disclosed_context_count":2,"omitted_context_count":1,
                "context_token_budget":120000,"context_token_count":3000,
                \(capacity)
                "binding_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
                "status":"succeeded","failure_code":""
              }],
              "messages":[],"can_reply":true,"requires_confirmation":false
            }
            """
    }
}

final class LocalConversationCatalogTests: XCTestCase {
func testConversationModelCatalogCoversCodexAndThreeLayers() {
    let codex = localProductConversationModels(providerID: "openai")
    XCTAssertTrue(codex.contains { $0.modelID == "codex-default" })
    XCTAssertFalse(codex.contains { $0.modelID == "deepseek-v4-flash" })
    XCTAssertFalse(codex.contains { $0.modelID == "deepseek-v4-pro" })
    // Grounded in the installed Codex CLI 0.144.1 catalog: GPT-5.6-Sol carries
    // low/medium/high/xhigh/max/ultra and GPT-5.5 carries low/medium/high/xhigh.
    XCTAssertEqual(
        Set(localProductConversationReasoningEfforts(providerID: "openai", modelID: "gpt-5.6-sol")),
        Set(["low", "medium", "high", "xhigh", "max", "ultra"])
    )
    XCTAssertEqual(
        Set(localProductConversationReasoningEfforts(providerID: "openai", modelID: "gpt-5.5")),
        Set(["low", "medium", "high", "xhigh"])
    )
    let opencode = localProductConversationModels(providerID: "opencode")
    XCTAssertTrue(opencode.contains { $0.modelID == "deepseek/deepseek-chat" })
    XCTAssertTrue(opencode.contains { $0.modelID == "opencode/big-pickle" })
    XCTAssertFalse(opencode.contains { $0.modelID == "opencode/deepseek-v4-flash-free" })
    // DeepSeek Chat is toggle-only in the OpenCode CLI: no effort values.
    XCTAssertTrue(
        localProductConversationReasoningEfforts(
            providerID: "opencode",
            modelID: "deepseek/deepseek-chat"
        ).isEmpty
    )
    // v4-pro exposes high/max (OpenCode CLI 1.18.3), v4-flash exposes
    // low/high/max.
    XCTAssertEqual(
        Set(localProductConversationReasoningEfforts(providerID: "opencode", modelID: "deepseek/deepseek-v4-pro")),
        Set(["high", "max"])
    )
    XCTAssertEqual(
        Set(localProductConversationReasoningEfforts(providerID: "opencode", modelID: "deepseek/deepseek-v4-flash")),
        Set(["low", "high", "max"])
    )
    // DeepSeek provider: v4 models carry low/high/max; legacy aliases none.
    XCTAssertEqual(
        Set(localProductConversationReasoningEfforts(providerID: "deepseek", modelID: "deepseek-v4-pro")),
        Set(["low", "high", "max"])
    )
    XCTAssertTrue(
        localProductConversationReasoningEfforts(providerID: "deepseek", modelID: "deepseek-chat").isEmpty
    )
	let localModels = localProductConversationModels(providerID: "loom-local")
	XCTAssertEqual(localModels.count, 1)
	XCTAssertEqual(
		localModels.first?.modelID,
		"qwen2.5-coder-1.5b-instruct-q4-k-m"
	)
	XCTAssertEqual(localModels.first?.displayName, "Qwen 2.5 Coder 1.5B (Local)")
	XCTAssertTrue(localModels.first?.reasoningEfforts.isEmpty == true)
    XCTAssertEqual(
        localProductConversationModels(providerID: "deepseek")
            .first { $0.modelID == "deepseek-chat" }?.displayName,
        "DeepSeek Chat"
    )
    XCTAssertFalse(
        localProductConversationModels(providerID: "deepseek")
            .contains { $0.displayName.localizedCaseInsensitiveContains("legacy") }
    )
    XCTAssertTrue(localProductConversationModels(providerID: "unknown").isEmpty)
}
}
