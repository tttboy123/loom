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
        {"schema_version":1,"occurred_at":"2026-08-10T03:00:00.000Z","incident_id":"\(incidentID)","operation":"agent_attempt","credential_runtime":"vault","credential_helper_spawn_attempts":0,"provider_id":"deepseek","provider_account_id":"deepseek.primary","model_id":"deepseek-chat","stage":"provider_auth","elapsed_ms":12,"result":"failed","error_code":"provider_auth","retryable":false,"secret":"must-not-survive"}
        {"schema_version":1,"occurred_at":"2026-08-10T03:00:01.000Z","incident_id":"\(conversationIncidentID)","operation":"chat_message","credential_runtime":"vault","thread_id":"thread-deepseek-r3","profile_id":"conversation-deepseek-deepseek-chat-r3","stage":"conversation_dispatch","elapsed_ms":18,"result":"failed","error_code":"profile_conflict","retryable":true,"content":"must-not-survive"}
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
}
