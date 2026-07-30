import AppKit
import SwiftUI
import XCTest
@testable import LoomLocalAppCore
@testable import LoomLocalAppUI

@MainActor
private var retainedMissionWorkbenchWindows: [NSWindow] = []

@MainActor
final class LoomGraphiteViewTests: XCTestCase {
    func testMissionWorkbenchUsesNativeWindowAndProviderIsReachable()
        async throws
    {
        let snapshot = try LocalProductWire.decodeSnapshot(
            Data(MissionOrchestrationTests.snapshotJSON.utf8)
        )
        let store = LocalProductStore(
            client: MissionWorkbenchStubClient(snapshot: snapshot)
        )
        await store.refresh()
        await store.refreshSetup()

        let window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 1_100, height: 720),
            styleMask: [.titled, .closable, .resizable],
            backing: .buffered,
            defer: false
        )
        window.contentView = NSHostingView(
            rootView: ContentView(store: store, refreshOnAppear: false)
        )
        window.layoutIfNeeded()

        XCTAssertEqual(store.workbench.route, .board)
        XCTAssertTrue(store.providerManagementReachable)
        XCTAssertGreaterThanOrEqual(
            LoomGraphite.minimumActionTarget,
            44
        )
        XCTAssertNotNil(window.contentView)
        retainedMissionWorkbenchWindows.append(window)
    }

    func testAllDecisionTemplatesUseOneNativeSheetHost() throws {
        let fixtures: [(String, [String])] = [
            (
                "authorization",
                ["not_now", "deny", "edit_scope", "allow_once",
                 "allow_for_mission"]
            ),
            ("review", ["not_now", "request_changes", "accept_result"]),
            (
                "recovery",
                ["not_now", "stop_mission", "edit_scope",
                 "start_new_attempt"]
            ),
        ]
        for (kind, actions) in fixtures {
            let sheet = try LocalProductDecisionWire.decodeSheet(
                Data(decisionSheetJSON(kind: kind, actions: actions).utf8)
            )
            switch kind {
            case "authorization":
                XCTAssertEqual(
                    sheet.summary,
                    "Run the verified local test command."
                )
            case "review":
                XCTAssertEqual(sheet.networkAccess, "Independent Reviewer: PASS")
            case "recovery":
                XCTAssertEqual(
                    sheet.attemptScope,
                    "Attempt 2 · fresh generation"
                )
                XCTAssertEqual(sheet.target, "No Team or Provider change")
            default:
                XCTFail("unexpected decision kind \(kind)")
            }
            let window = NSWindow(
                contentRect: NSRect(x: 0, y: 0, width: 680, height: 560),
                styleMask: [.titled, .closable],
                backing: .buffered,
                defer: false
            )
            window.contentView = NSHostingView(
                rootView: LoomDecisionSheet(
                    sheet: sheet,
                    onAction: { _ in },
                    onClose: {}
                )
                .environment(\.colorScheme, kind == "review" ? .dark : .light)
                .frame(width: 680, height: 560)
            )
            window.layoutIfNeeded()
            XCTAssertNotNil(window.contentView, "\(kind) sheet did not render")
            XCTAssertGreaterThanOrEqual(window.frame.width, 620)
            XCTAssertGreaterThan(
                window.contentView?.fittingSize.width ?? 0,
                320
            )
            try exportDecisionSheetIfRequested(window, kind: kind)
            retainedMissionWorkbenchWindows.append(window)
        }
    }

    private func exportDecisionSheetIfRequested(
        _ window: NSWindow,
        kind: String
    ) throws {
        guard let requestedDirectory =
            ProcessInfo.processInfo.environment[
                "LOOM_DECISION_SCREENSHOT_DIR"
            ] else {
            return
        }
        let current = URL(
            fileURLWithPath: FileManager.default.currentDirectoryPath
        )
        let expected = current.appendingPathComponent(
            "../../.loom-evidence/phase2a/P2A-W2"
        ).standardizedFileURL
        let requested = URL(fileURLWithPath: requestedDirectory)
            .standardizedFileURL
        guard requested == expected,
              requested == requested.resolvingSymlinksInPath()
                .standardizedFileURL else {
            throw LocalProductClientError.invalidRequest
        }
        guard let view = window.contentView else {
            throw LocalProductClientError.invalidResponse
        }
        view.frame = NSRect(x: 0, y: 0, width: 680, height: 560)
        view.layoutSubtreeIfNeeded()
        guard let bitmap = view.bitmapImageRepForCachingDisplay(
            in: view.bounds
        ) else {
            throw LocalProductClientError.invalidResponse
        }
        view.cacheDisplay(in: view.bounds, to: bitmap)
        guard let png = bitmap.representation(using: .png, properties: [:]),
              png.count > 20_000 else {
            throw LocalProductClientError.invalidResponse
        }
        try png.write(
            to: requested.appendingPathComponent(
                "decision-\(kind).png"
            ),
            options: .atomic
        )
    }

    private func decisionSheetJSON(
        kind: String,
        actions: [String]
    ) -> String {
        let title: String
        let summary: String
        let target: String
        let commandType: String
        let networkAccess: String
        let permissionScope: String
        let attemptScope: String
        let expectedEvidence: String
        switch kind {
        case "authorization":
            title = "Authorization required"
            summary = "Run the verified local test command."
            target = "Controlled local workspace"
            commandType = "Execute bounded local test"
            networkAccess = "none"
            permissionScope = "Repository read/write · this Mission"
            attemptScope = "Attempt 1"
            expectedEvidence = "Authoritative approval fact"
        case "review":
            title = "Review result"
            summary = "Validated API contract changes."
            target = "Exact prepared node result"
            commandType = "Focused and repository checks: PASS"
            networkAccess = "Independent Reviewer: PASS"
            permissionScope = "No new permission"
            attemptScope = "Attempt 1"
            expectedEvidence = "Evidence receipt accepted for Attempt 1"
        case "recovery":
            title = "Recovery decision"
            summary =
                "Attempt 1 stopped after deterministic verification failed."
            target = "No Team or Provider change"
            commandType = "Fresh attempt with generation fencing"
            networkAccess = "none"
            permissionScope = "No permission or budget increase"
            attemptScope = "Attempt 2 · fresh generation"
            expectedEvidence =
                "Attempt 1 output and Evidence retained by digest"
        default:
            preconditionFailure("unsupported decision kind")
        }
        let encodedActions = actions
            .map { "\"\($0)\"" }
            .joined(separator: ",")
        let encodedPreparedActions = actions
            .filter { !["not_now", "edit_scope"].contains($0) }
            .map { "\"\($0)\"" }
            .joined(separator: ",")
        return """
        {
          "schema_version":1,
          "kind":"\(kind)",
          "mission_id":"mission/team-1",
          "team_instance_id":"team-1",
          "view_version":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
          "decision_id":"decision-1",
          "decision_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
          "title":"\(title)",
          "summary":"\(summary)",
          "requester":"Release Team",
          "target":"\(target)",
          "command_type":"\(commandType)",
          "network_access":"\(networkAccess)",
          "credential_access":"none",
          "permission_scope":"\(permissionScope)",
          "attempt_scope":"\(attemptScope)",
          "expected_evidence":"\(expectedEvidence)",
          "technical_details":["Prepared by owning authority"],
          "actions":[\(encodedActions)],
          "prepared_actions":[\(encodedPreparedActions)],
          "prepared":true,
          "logical_node_id":"main",
          "attempt_number":1,
          "claim_generation":1
        }
        """
    }
}

private struct MissionWorkbenchStubClient:
    LocalProductClientProtocol,
    LocalProductSetupClientProtocol
{
    let snapshotValue: LocalProductSnapshot

    init(snapshot: LocalProductSnapshot) {
        snapshotValue = snapshot
    }

    func snapshot(limit: Int) async throws -> LocalProductSnapshot {
        snapshotValue
    }

    func timeline(
        teamInstanceID: String,
        cursor: String,
        limit: Int
    ) async throws -> LocalProductTimelinePage {
        throw LocalProductClientError.notFound
    }

    func setupSnapshot() async throws -> LocalProductSetupSnapshot {
        try LocalProductSetupWire.decodeSnapshot(
            Data(
                """
                {
                  "schema_version":1,
                  "view_version":"view-setup",
                  "codex":{"provider_id":"codex","auth_mode":"native_auth","credential_reference":"","revision":0,"status":"available","reason":""},
                  "minimax":{"provider_id":"minimax","auth_mode":"brokered","credential_reference":"configured","revision":1,"status":"verified","reason":""},
                  "runtimes":[],
                  "saved_teams":[],
                  "templates":[],
                  "role_options":[],
                  "skills":[],
                  "permissions":[],
                  "resources":[]
                }
                """.utf8
            )
        )
    }

    func connectCodex() async throws -> LocalProductProviderConnectResult {
        throw LocalProductClientError.unavailable
    }

    func startBuilder(
        source: String,
        sourceID: String,
        sourceVersion: Int,
        sourceDigest: String
    ) async throws -> LocalProductBuilderSession {
        throw LocalProductClientError.unavailable
    }

    func answerBuilder(
        session: LocalProductBuilderSession,
        answer: String
    ) async throws -> LocalProductBuilderSession {
        throw LocalProductClientError.unavailable
    }

    func editBuilder(
        session: LocalProductBuilderSession,
        field: String,
        value: String
    ) async throws -> LocalProductBuilderSession {
        throw LocalProductClientError.unavailable
    }

    func validateBuilder(
        session: LocalProductBuilderSession
    ) async throws -> LocalProductBuilderSession {
        throw LocalProductClientError.unavailable
    }

    func confirmBuilder(
        session: LocalProductBuilderSession,
        definitionID: String
    ) async throws -> LocalProductBuilderConfirmation {
        throw LocalProductClientError.unavailable
    }

    func archiveTeam(
        definitionID: String,
        expectedHead: Int64
    ) async throws -> LocalProductSetupSavedTeam {
        throw LocalProductClientError.unavailable
    }

    func restoreTeam(
        definitionID: String,
        expectedHead: Int64
    ) async throws -> LocalProductSetupSavedTeam {
        throw LocalProductClientError.unavailable
    }

    func configureMiniMax(
        secret: String
    ) async throws -> LocalProductCredentialSetupResult {
        throw LocalProductClientError.unavailable
    }

    func verifyMiniMax(
        reference: String,
        revision: Int64
    ) async throws -> LocalProductCredentialSetupResult {
        throw LocalProductClientError.unavailable
    }

    func replaceMiniMax(
        reference: String,
        revision: Int64,
        secret: String
    ) async throws -> LocalProductCredentialSetupResult {
        throw LocalProductClientError.unavailable
    }

    func revokeMiniMax(
        reference: String,
        revision: Int64
    ) async throws -> LocalProductCredentialSetupResult {
        throw LocalProductClientError.unavailable
    }
}
