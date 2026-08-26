import Foundation
import XCTest

@testable import LoomLocalAppCore

final class LocalDiagnosticBundleTests: XCTestCase {
    func testPreviewAndExportContainOnlyAllowlistedSupportData() throws {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("loom-diagnostic-bundle-\(UUID().uuidString)")
        let diagnostics = root.appendingPathComponent("diagnostics")
        try FileManager.default.createDirectory(
            at: diagnostics,
            withIntermediateDirectories: true,
            attributes: [.posixPermissions: 0o700]
        )
        defer { try? FileManager.default.removeItem(at: root) }

        let app = root.appendingPathComponent("LoomLocalApp")
        let daemon = root.appendingPathComponent("loomd")
        try Data("app-binary".utf8).write(to: app)
        try Data("daemon-binary".utf8).write(to: daemon)
        try FileManager.default.setAttributes(
            [.posixPermissions: 0o700],
            ofItemAtPath: app.path
        )
        try FileManager.default.setAttributes(
            [.posixPermissions: 0o700],
            ofItemAtPath: daemon.path
        )

        let incidentID = "loom-swift-33333333-3333-4333-8333-333333333333"
        let conversationIncidentID = "loom-chat-44444444-4444-4444-8444-444444444444"
        let line = """
        {"schema_version":1,"occurred_at":"2026-08-10T03:00:00.000Z","incident_id":"\(incidentID)","operation":"agent_attempt","credential_runtime":"vault","credential_helper_spawn_attempts":0,"provider_id":"deepseek","provider_account_id":"deepseek.primary","model_id":"deepseek-chat","stage":"provider_auth","elapsed_ms":12,"result":"failed","error_code":"provider_auth","retryable":false}
        {"schema_version":1,"occurred_at":"2026-08-10T03:00:01.000Z","incident_id":"\(conversationIncidentID)","operation":"chat_message","credential_runtime":"vault","thread_id":"thread-deepseek-r3","profile_id":"conversation-deepseek-deepseek-chat-r3","stage":"conversation_dispatch","elapsed_ms":18,"result":"failed","error_code":"profile_conflict","retryable":true}
        """
        let diagnosticPath = diagnostics.appendingPathComponent("operational.jsonl")
        try Data((line + "\n").utf8).write(to: diagnosticPath)
        try FileManager.default.setAttributes(
            [.posixPermissions: 0o600],
            ofItemAtPath: diagnosticPath.path
        )
        let consolePath = diagnostics.appendingPathComponent("loomd-console.log")
        try Data("prompt and provider body must not be exported".utf8).write(to: consolePath)
        try FileManager.default.setAttributes(
            [.posixPermissions: 0o600],
            ofItemAtPath: consolePath.path
        )

        let exporter = try LocalDiagnosticBundleExporter(
            diagnosticsDirectory: diagnostics,
            appExecutable: app,
            daemonExecutable: daemon,
            appVersion: "0.5.2",
            appBuild: "9",
            socketHealthy: { true },
            now: { Date(timeIntervalSince1970: 1_786_276_800) }
        )
        let input = LocalDiagnosticBundleInput(
            providers: [
                .init(
                    providerID: "deepseek",
                    authMode: "api_key",
                    revision: 4,
                    status: "verified",
                    reason: ""
                ),
            ],
            profiles: [
                .init(
                    profileID: "conversation.deepseek",
                    providerID: "deepseek",
                    modelID: "deepseek-chat",
                    authMode: "api_key",
                    credentialRevision: 4
                ),
            ],
            agents: [
                .init(
                    agentInstanceID: "agent-coder",
                    status: "blocked",
                    harnessAdapter: "pi",
                    providerID: "deepseek",
                    providerAccountID: "deepseek.primary",
                    modelID: "deepseek-chat",
                    reasoningEffort: "high",
                    credentialRevision: 4,
                    terminalReason: "provider_rejected"
                ),
            ]
        )

        let preview = try exporter.preview(input: input)
        XCTAssertTrue(preview.socketHealthy)
        XCTAssertEqual(preview.events.map(\.incidentID), [incidentID, conversationIncidentID])
        XCTAssertEqual(preview.events.map(\.providerAccountID), ["deepseek.primary", ""])
        XCTAssertEqual(
            preview.events.map { $0.credentialRuntime ?? "" },
            ["vault", "vault"]
        )
        XCTAssertEqual(
            preview.events.map(\.credentialHelperSpawnAttempts),
            [0, nil]
        )
        XCTAssertEqual(preview.events.map(\.modelID), ["deepseek-chat", ""])
        XCTAssertEqual(preview.events.map(\.threadID), ["", "thread-deepseek-r3"])
        XCTAssertEqual(
            preview.events.map(\.profileID),
            ["", "conversation-deepseek-deepseek-chat-r3"]
        )
        XCTAssertEqual(preview.providers.count, 1)
        XCTAssertEqual(preview.profiles.count, 1)
        XCTAssertEqual(preview.agents.count, 1)
        XCTAssertEqual(preview.agents.first?.reasoningEffort, "high")
        XCTAssertEqual(preview.console.byteCount, 45)
        XCTAssertEqual(
            preview.app.sha256,
            "be7b6a4be40e8cb5fcd22ec506300e6252182bb874fb17d33396772a23957256"
        )
        XCTAssertEqual(
            preview.daemon.sha256,
            "e51564a9b2f14e4a47c553606d39a4c687a42cd8fb0ece2bc5182b9f638e2f5b"
        )
        XCTAssertTrue(preview.excluded.contains("api_keys_and_credentials"))
        XCTAssertTrue(preview.excluded.contains("prompts_and_conversations"))
        XCTAssertTrue(preview.excluded.contains("provider_response_bodies"))

        let destination = root.appendingPathComponent("loom-diagnostics.json")
        try exporter.export(preview, to: destination)
        let attributes = try FileManager.default.attributesOfItem(atPath: destination.path)
        XCTAssertEqual((attributes[.posixPermissions] as? NSNumber)?.intValue, 0o600)
        let text = try XCTUnwrap(String(contentsOf: destination, encoding: .utf8))
        XCTAssertTrue(text.contains(incidentID))
        XCTAssertTrue(text.contains(conversationIncidentID))
        XCTAssertTrue(text.contains("thread-deepseek-r3"))
        XCTAssertTrue(text.contains("conversation-deepseek-deepseek-chat-r3"))
        XCTAssertFalse(text.contains("must-not-survive"))
        XCTAssertFalse(text.contains("prompt and provider body"))
        XCTAssertFalse(text.contains("credential_reference"))
        XCTAssertFalse(text.contains("\"authorization_header\""))
        XCTAssertFalse(text.contains("Bearer "))
    }

    func testGatewayV3RoundTripPreservesPrivacySafeAuthority() throws {
        let record = gatewayRecord(version: 3)
        let fixture = try makeFixture(records: [record])
        defer { try? FileManager.default.removeItem(at: fixture.root) }

        let preview = try fixture.exporter.preview(input: .init())
        let event = try XCTUnwrap(preview.events.first)

        XCTAssertEqual(preview.events.count, 1)
        XCTAssertEqual(event.gatewayEventSchemaVersion, 3)
        XCTAssertEqual(event.gatewayInstanceID, "gateway-claude-1")
        XCTAssertEqual(event.gatewayConfiguredHarnessVersion, 3)
        XCTAssertEqual(event.gatewayBackendVersion, 5)
        XCTAssertEqual(event.gatewayEventSequence, 7)
        XCTAssertEqual(event.gatewayEventType, "response_completed")
        XCTAssertEqual(event.sessionID, "session-claude-1")
        XCTAssertEqual(event.harnessID, "loom-native")
        XCTAssertEqual(event.backendID, "backend.segment.loom-native")
        XCTAssertEqual(event.threadID, "conversation-claude-1")
        XCTAssertEqual(event.segmentID, "segment-claude-1")
        XCTAssertEqual(event.workspaceID, "workspace-claude-1")
        XCTAssertEqual(event.workspaceDigest, digest("a"))
        XCTAssertEqual(event.executionBindingDigest, digest("b"))
        XCTAssertEqual(event.providerID, "deepseek")
        XCTAssertEqual(event.providerAccountID, "deepseek.primary")
        XCTAssertEqual(event.credentialRevision, 7)
        XCTAssertEqual(event.modelID, "deepseek-chat")
        XCTAssertEqual(event.reasoningEffort, "high")
        XCTAssertEqual(event.segmentContextCapsuleDigest, digest("c"))
        XCTAssertEqual(event.contextCapsuleDigest, digest("d"))
        XCTAssertEqual(event.governancePolicyDigest, digest("e"))
        XCTAssertEqual(event.routeTransitionReviewDigest, digest("f"))
        XCTAssertEqual(event.responseID, "response-claude-1")

        let destination = fixture.root.appendingPathComponent("gateway-v2.json")
        try fixture.exporter.export(preview, to: destination)
        let exported = try Data(contentsOf: destination)
        let decoded = try JSONDecoder().decode(LocalDiagnosticBundlePreview.self, from: exported)
        XCTAssertEqual(decoded, preview)

        let text = try XCTUnwrap(String(data: exported, encoding: .utf8))
        for key in [
            "gateway_event_schema_version", "gateway_configured_harness_version",
            "gateway_instance_id",
            "gateway_backend_version", "gateway_event_sequence", "gateway_event_type",
            "workspace_digest", "execution_binding_digest",
            "segment_context_capsule_digest", "context_capsule_digest",
            "governance_policy_digest", "route_transition_review_digest", "response_id",
        ] {
            XCTAssertTrue(text.contains("\"\(key)\""), "missing \(key)")
        }
    }

    func testDiagnosticExportRetainsBoundedGatewayWindowForInstalledAcceptance() throws {
        let records = (1...150).map { sequence in
            var record = gatewayRecord(version: 3)
            record["gateway_event_sequence"] = sequence
            record["incident_id"] = "incident-gateway-\(sequence)"
            record["response_id"] = "response-gateway-\(sequence)"
            return record
        }
        let fixture = try makeFixture(records: records)
        defer { try? FileManager.default.removeItem(at: fixture.root) }

        let preview = try fixture.exporter.preview(input: .init())

        XCTAssertEqual(preview.events.count, 150)
        XCTAssertEqual(
            Set(preview.events.compactMap(\.gatewayEventSequence)),
            Set(1...150)
        )
    }

    func testGatewayV3RejectsInvalidAuthorityAndForbiddenFields() throws {
        var records: [[String: Any]] = []
        for key in [
            "gateway_instance_id",
            "workspace_digest", "execution_binding_digest",
            "segment_context_capsule_digest", "context_capsule_digest",
            "governance_policy_digest",
        ] {
            var record = gatewayRecord(version: 3)
            record.removeValue(forKey: key)
            records.append(record)
        }
        var invalidRevision = gatewayRecord(version: 3)
        invalidRevision["credential_revision"] = 0
        records.append(invalidRevision)
        for value in ["", "not-a-digest"] {
            var invalidReview = gatewayRecord(version: 3)
            invalidReview["route_transition_review_digest"] = value
            records.append(invalidReview)
        }

        for (key, value) in [
            ("content", "private prompt"),
            ("workspace_path", "/private/workspace"),
            ("credential_reference", "vault://provider/account"),
            ("provider_body", "private provider response"),
        ] {
            var record = gatewayRecord(version: 3)
            record[key] = value
            records.append(record)
        }

        let fixture = try makeFixture(records: records)
        defer { try? FileManager.default.removeItem(at: fixture.root) }

        let preview = try fixture.exporter.preview(input: .init())
        XCTAssertTrue(preview.events.isEmpty)
        XCTAssertEqual(preview.diagnosticFilesSkipped, records.count)
    }

    func testGatewayV3RejectsUnknownFieldsBeforeDecoding() throws {
        var unknownScalar = gatewayRecord(version: 3)
        unknownScalar["future_gateway_authority"] = "must-not-be-stripped"
        var unknownObject = gatewayRecord(version: 3)
        unknownObject["metadata"] = ["safe_label": "still-unknown"]

        let fixture = try makeFixture(records: [unknownScalar, unknownObject])
        defer { try? FileManager.default.removeItem(at: fixture.root) }

        let preview = try fixture.exporter.preview(input: .init())
        XCTAssertTrue(preview.events.isEmpty)
        XCTAssertEqual(preview.diagnosticFilesSkipped, 2)
    }

    func testGatewayLegacyRecordsAllowUnknownFieldsButRejectNestedContent() throws {
        var compatibleV1 = gatewayRecord(version: 1)
        compatibleV1["legacy_extension"] = ["label": "compatible"]
        var compatibleV2 = gatewayRecord(version: 2)
        compatibleV2["legacy_extension"] = true
        var nestedV1 = gatewayRecord(version: 1)
        nestedV1["legacy_extension"] = ["details": ["content": "private prompt"]]
        var nestedV2 = gatewayRecord(version: 2)
        nestedV2["legacy_extension"] = [
            ["label": "safe"],
            ["authorization": "Bearer private-token"],
        ]

        let fixture = try makeFixture(
            records: [compatibleV1, compatibleV2, nestedV1, nestedV2]
        )
        defer { try? FileManager.default.removeItem(at: fixture.root) }

        let preview = try fixture.exporter.preview(input: .init())
        XCTAssertEqual(preview.events.map(\.gatewayEventSchemaVersion), [1, 2])
        XCTAssertEqual(preview.diagnosticFilesSkipped, 2)
    }

    func testGatewayV3PreservesNativeAuthorityExactly() throws {
        let record = nativeGatewayRecord()
        let fixture = try makeFixture(records: [record])
        defer { try? FileManager.default.removeItem(at: fixture.root) }

        let preview = try fixture.exporter.preview(input: .init())
        let event = try XCTUnwrap(preview.events.first)
        XCTAssertEqual(event.providerAccountID, "")
        XCTAssertEqual(event.credentialRevision, 0)
        XCTAssertEqual(event.governancePolicyDigest, "")
        XCTAssertNil(event.routeTransitionReviewDigest)

        let destination = fixture.root.appendingPathComponent("gateway-native.json")
        try fixture.exporter.export(preview, to: destination)
        let object = try XCTUnwrap(
            JSONSerialization.jsonObject(with: Data(contentsOf: destination))
                as? [String: Any]
        )
        let events = try XCTUnwrap(object["events"] as? [[String: Any]])
        let exported = try XCTUnwrap(events.first)
        XCTAssertEqual(exported["provider_account_id"] as? String, "")
        XCTAssertEqual(exported["credential_revision"] as? Int, 0)
        XCTAssertEqual(exported["governance_policy_digest"] as? String, "")
        XCTAssertNil(exported["route_transition_review_digest"])
    }

    func testGatewayV3RejectsMixedNativeAndBrokeredAuthority() throws {
        var nativeRevision = nativeGatewayRecord()
        nativeRevision["credential_revision"] = 1

        var nativePolicy = nativeGatewayRecord()
        nativePolicy["governance_policy_digest"] = digest("e")

        var brokeredRevision = gatewayRecord(version: 3)
        brokeredRevision["credential_revision"] = 0

        var nativeWithBrokeredAuthority = nativeGatewayRecord()
        nativeWithBrokeredAuthority["provider_account_id"] = "openai.primary"
        nativeWithBrokeredAuthority["credential_revision"] = 7
        nativeWithBrokeredAuthority["governance_policy_digest"] = digest("e")

        var brokeredWithNativeAuthority = gatewayRecord(version: 3)
        brokeredWithNativeAuthority["provider_account_id"] = ""
        brokeredWithNativeAuthority["credential_revision"] = 0
        brokeredWithNativeAuthority["governance_policy_digest"] = ""

        var legacyCodexBackend = nativeGatewayRecord()
        legacyCodexBackend["backend_id"] = "backend.segment.codex"

        let fixture = try makeFixture(
            records: [
                nativeRevision, nativePolicy, brokeredRevision,
                nativeWithBrokeredAuthority, brokeredWithNativeAuthority,
                legacyCodexBackend,
            ]
        )
        defer { try? FileManager.default.removeItem(at: fixture.root) }

        let preview = try fixture.exporter.preview(input: .init())
        XCTAssertTrue(preview.events.isEmpty)
        XCTAssertEqual(preview.diagnosticFilesSkipped, 6)
    }

    func testGatewayV3PreservesBrokeredAbsentPolicyAuthority() throws {
        var record = gatewayRecord(version: 3)
        record["governance_policy_digest"] = ""
        let fixture = try makeFixture(records: [record])
        defer { try? FileManager.default.removeItem(at: fixture.root) }

        let preview = try fixture.exporter.preview(input: .init())
        let event = try XCTUnwrap(preview.events.first)
        XCTAssertEqual(event.providerAccountID, "deepseek.primary")
        XCTAssertEqual(event.credentialRevision, 7)
        XCTAssertEqual(event.governancePolicyDigest, "")
        XCTAssertEqual(event.routeTransitionReviewDigest, digest("f"))
    }

    func testGatewayV1RecordRemainsCompatible() throws {
        let record = gatewayRecord(version: 1)
        let fixture = try makeFixture(records: [record])
        defer { try? FileManager.default.removeItem(at: fixture.root) }

        let preview = try fixture.exporter.preview(input: .init())
        let event = try XCTUnwrap(preview.events.first)

        XCTAssertEqual(preview.events.count, 1)
        XCTAssertEqual(event.gatewayEventSchemaVersion, 1)
        XCTAssertEqual(event.gatewayInstanceID, "")
        XCTAssertEqual(event.gatewayEventSequence, 7)
        XCTAssertEqual(event.sessionID, "session-claude-1")
        XCTAssertEqual(event.workspaceID, "workspace-claude-1")
        XCTAssertEqual(event.providerID, "")
        XCTAssertNil(event.credentialRevision)
        XCTAssertEqual(event.workspaceDigest, "")
        XCTAssertEqual(event.contextCapsuleDigest, "")
    }

    func testGatewayV2RecordRemainsCompatibleWithoutInstanceIdentity() throws {
        let record = gatewayRecord(version: 2)
        let fixture = try makeFixture(records: [record])
        defer { try? FileManager.default.removeItem(at: fixture.root) }

        let preview = try fixture.exporter.preview(input: .init())
        let event = try XCTUnwrap(preview.events.first)

        XCTAssertEqual(event.gatewayEventSchemaVersion, 2)
        XCTAssertEqual(event.gatewayInstanceID, "")
        XCTAssertEqual(event.providerAccountID, "deepseek.primary")
        XCTAssertEqual(event.contextCapsuleDigest, digest("d"))
    }

    func testUnsafeDiagnosticFileIsExcludedInsteadOfRead() throws {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("loom-diagnostic-bundle-\(UUID().uuidString)")
        let diagnostics = root.appendingPathComponent("diagnostics")
        try FileManager.default.createDirectory(
            at: diagnostics,
            withIntermediateDirectories: true,
            attributes: [.posixPermissions: 0o700]
        )
        defer { try? FileManager.default.removeItem(at: root) }
        let app = root.appendingPathComponent("app")
        let daemon = root.appendingPathComponent("daemon")
        try Data("app".utf8).write(to: app)
        try Data("daemon".utf8).write(to: daemon)
        let outside = root.appendingPathComponent("outside.jsonl")
        try Data("secret".utf8).write(to: outside)
        try FileManager.default.createSymbolicLink(
            at: diagnostics.appendingPathComponent("app-operational.jsonl"),
            withDestinationURL: outside
        )

        let exporter = try LocalDiagnosticBundleExporter(
            diagnosticsDirectory: diagnostics,
            appExecutable: app,
            daemonExecutable: daemon,
            appVersion: "0.5.2",
            appBuild: "9",
            socketHealthy: { false },
            now: { Date(timeIntervalSince1970: 1_786_276_800) }
        )
        let preview = try exporter.preview(input: .init())

        XCTAssertFalse(preview.socketHealthy)
        XCTAssertTrue(preview.events.isEmpty)
        XCTAssertEqual(preview.diagnosticFilesSkipped, 1)
    }

    private func gatewayRecord(version: Int) -> [String: Any] {
        var record: [String: Any] = [
            "schema_version": 1,
            "occurred_at": "2026-08-24T11:12:13.000000014Z",
            "incident_id": "incident-claude-1",
            "operation": "chat_message",
            "credential_runtime": "vault",
            "credential_helper_spawn_attempts": 0,
            "gateway_event_schema_version": version,
            "gateway_configured_harness_version": 3,
            "gateway_backend_version": 5,
            "gateway_event_sequence": 7,
            "gateway_event_type": "response_completed",
            "session_id": "session-claude-1",
            "harness_id": "loom-native",
            "backend_id": "backend.segment.loom-native",
            "thread_id": "conversation-claude-1",
            "segment_id": "segment-claude-1",
            "workspace_id": "workspace-claude-1",
            "stage": "conversation_dispatch",
            "elapsed_ms": 0,
            "result": "succeeded",
            "retryable": false,
            "response_id": "response-claude-1",
        ]
        if version >= 2 {
            record.merge([
                "workspace_digest": digest("a"),
                "execution_binding_digest": digest("b"),
                "provider_id": "deepseek",
                "provider_account_id": "deepseek.primary",
                "credential_revision": 7,
                "model_id": "deepseek-chat",
                "reasoning_effort": "high",
                "segment_context_capsule_digest": digest("c"),
                "context_capsule_digest": digest("d"),
                "governance_policy_digest": digest("e"),
            ]) { _, new in new }
        }
        if version >= 3 {
            record["gateway_instance_id"] = "gateway-claude-1"
            record["route_transition_review_digest"] = digest("f")
        }
        return record
    }

    private func nativeGatewayRecord() -> [String: Any] {
        var record = gatewayRecord(version: 3)
        record["harness_id"] = "codex"
        record["backend_id"] = "backend.codex.app-server"
        record["provider_id"] = "openai"
        record["provider_account_id"] = ""
        record["credential_revision"] = 0
        record["model_id"] = "codex"
        record["governance_policy_digest"] = ""
        record.removeValue(forKey: "route_transition_review_digest")
        return record
    }

    private func digest(_ character: Character) -> String {
        String(repeating: String(character), count: 64)
    }

    private func makeFixture(
        records: [[String: Any]]
    ) throws -> (
        root: URL,
        exporter: LocalDiagnosticBundleExporter
    ) {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("loom-diagnostic-gateway-\(UUID().uuidString)")
        let diagnostics = root.appendingPathComponent("diagnostics")
        try FileManager.default.createDirectory(
            at: diagnostics,
            withIntermediateDirectories: true,
            attributes: [.posixPermissions: 0o700]
        )
        let lines = try records.map {
            let data = try JSONSerialization.data(withJSONObject: $0, options: [.sortedKeys])
            return try XCTUnwrap(String(data: data, encoding: .utf8))
        }.joined(separator: "\n") + "\n"
        let diagnosticPath = diagnostics.appendingPathComponent("operational.jsonl")
        try Data(lines.utf8).write(to: diagnosticPath)
        try FileManager.default.setAttributes(
            [.posixPermissions: 0o600],
            ofItemAtPath: diagnosticPath.path
        )
        let app = root.appendingPathComponent("app")
        let daemon = root.appendingPathComponent("daemon")
        try Data("app".utf8).write(to: app)
        try Data("daemon".utf8).write(to: daemon)
        let exporter = try LocalDiagnosticBundleExporter(
            diagnosticsDirectory: diagnostics,
            appExecutable: app,
            daemonExecutable: daemon,
            appVersion: "0.5.3",
            appBuild: "127",
            socketHealthy: { true },
            now: { Date(timeIntervalSince1970: 1_787_500_800) }
        )
        return (root, exporter)
    }
}
