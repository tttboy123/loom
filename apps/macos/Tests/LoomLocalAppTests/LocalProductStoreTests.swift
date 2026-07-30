import Foundation
import XCTest
@testable import LoomLocalAppCore

@MainActor
final class LocalProductStoreTests: XCTestCase {
    func testEmptyTeamActivationDoesNotRequestTimeline() async {
        let client = StubLocalProductClient(
            snapshots: [.success(.empty(viewVersion: "view-1"))]
        )
        let store = LocalProductStore(client: client)

        await store.refresh()
        await store.activateSelectedTeam()

        XCTAssertEqual(client.timelineRequestCount, 0)
        XCTAssertEqual(
            store.teamTimelineMessage,
            "Select a Team from Teams to open its authoritative timeline."
        )
        XCTAssertEqual(
            LocalProductStore.teamsEmptyMessage,
            "No Teams exist in this Journal yet."
        )
    }

    func testFailedRefreshPreservesPriorViewAndMarksOffline() async {
        let original = LocalProductSnapshot.empty(viewVersion: "view-1")
        let client = StubLocalProductClient(
            snapshots: [
                .success(original),
                .failure(LocalProductClientError.unavailable),
            ]
        )
        let store = LocalProductStore(client: client)

        await store.refresh()
        await store.refresh()

        XCTAssertEqual(store.snapshot, original)
        XCTAssertEqual(store.connectionState, .offline(reason: "unavailable"))
    }

    func testRefreshPreservesSelectedTaskDraftAndInspector() async throws {
        let json = LocalProductModelsTests.snapshotJSON.replacingOccurrences(
            of: "\"teams\":[]",
            with: """
            "teams":[{
              "team_instance_id":"team-1",
              "display_name":"Release review",
              "source_kind":"saved",
              "state":"ready",
              "confirmed":true,
              "executable":true,
              "read_only":false
            }]
            """
        )
        let snapshot = try LocalProductWire.decodeSnapshot(Data(json.utf8))
        let client = StubLocalProductClient(
            snapshots: [
                .success(snapshot),
                .failure(LocalProductClientError.unavailable),
                .success(snapshot),
            ]
        )
        let store = LocalProductStore(client: client)
        await store.refresh()
        let task = try XCTUnwrap(
            store.workspace.tasks.first(where: { $0.kind == .team })
        )
        store.selectWorkspaceTask(task.id)
        store.updateComposerDraft("Keep this local note")
        store.selectInspector(.changes)

        await store.refresh()
        XCTAssertEqual(store.workspace.selectedTaskID, task.id)
        XCTAssertEqual(
            store.workspace.selectedContinuity.composerDraft,
            "Keep this local note"
        )
        XCTAssertEqual(store.workspace.selectedContinuity.inspector, .changes)

        await store.refresh()
        XCTAssertEqual(store.workspace.selectedTaskID, task.id)
        XCTAssertEqual(
            store.workspace.selectedContinuity.composerDraft,
            "Keep this local note"
        )
        XCTAssertEqual(store.workspace.selectedContinuity.inspector, .changes)
    }

    func testSelectingVisibleTeamPerformsOneBoundedTimelineRequest() async throws {
        let json = LocalProductModelsTests.snapshotJSON.replacingOccurrences(
            of: "\"teams\":[]",
            with: """
            "teams":[{
              "team_instance_id":"team-1",
              "display_name":"Review Team",
              "source_kind":"saved",
              "state":"ready",
              "confirmed":true,
              "executable":true,
              "read_only":false
            }]
            """
        )
        let snapshot = try LocalProductWire.decodeSnapshot(Data(json.utf8))
        let client = StubLocalProductClient(snapshots: [.success(snapshot)])
        let store = LocalProductStore(client: client)

        await store.refresh()
        store.selectTeam(snapshot.teams[0])
        XCTAssertEqual(store.selectedSection, .teams)
        await store.activateSelectedTeam()

        XCTAssertEqual(client.timelineRequestCount, 1)
        XCTAssertEqual(client.lastTimelineLimit, 64)
        XCTAssertEqual(client.lastTimelineTeamID, "team-1")
    }

    func testWorkspaceTeamSelectionLoadsItsBoundedActivity() async throws {
        let json = LocalProductModelsTests.snapshotJSON.replacingOccurrences(
            of: "\"teams\":[]",
            with: """
            "teams":[{
              "team_instance_id":"team-1",
              "display_name":"Review Team",
              "source_kind":"saved",
              "state":"ready",
              "confirmed":true,
              "executable":true,
              "read_only":false
            }]
            """
        )
        let snapshot = try LocalProductWire.decodeSnapshot(Data(json.utf8))
        let client = StubLocalProductClient(snapshots: [.success(snapshot)])
        let store = LocalProductStore(client: client)

        await store.refresh()
        let task = try XCTUnwrap(
            store.workspace.tasks.first(where: { $0.kind == .team })
        )
        store.selectWorkspaceTask(task.id)
        await store.activateWorkspaceTask()

        XCTAssertEqual(client.timelineRequestCount, 1)
        XCTAssertEqual(client.lastTimelineLimit, 64)
        XCTAssertEqual(client.lastTimelineTeamID, "team-1")
    }

    func testNonrecoverableDaemonErrorIsFatal() async {
        let client = StubLocalProductClient(
            snapshots: [
                .failure(
                    LocalIPCRemoteError(
                        code: .unauthorizedPeer,
                        recoverable: false
                    )
                ),
            ]
        )
        let store = LocalProductStore(client: client)

        await store.refresh()

        XCTAssertEqual(
            store.connectionState,
            .fatal(reason: "unauthorized_peer")
        )
    }

    func testTimelineFailureClosesLoadingPresentation() async throws {
        let json = LocalProductModelsTests.snapshotJSON.replacingOccurrences(
            of: "\"teams\":[]",
            with: """
            "teams":[{
              "team_instance_id":"team-1",
              "display_name":"Review Team",
              "source_kind":"saved",
              "state":"ready",
              "confirmed":true,
              "executable":true,
              "read_only":false
            }]
            """
        )
        let snapshot = try LocalProductWire.decodeSnapshot(Data(json.utf8))
        let client = StubLocalProductClient(snapshots: [.success(snapshot)])
        let store = LocalProductStore(client: client)

        await store.refresh()
        store.selectTeam(snapshot.teams[0])
        XCTAssertEqual(store.timelineState, .loading)

        await store.activateSelectedTeam()

        XCTAssertEqual(store.timelineState, .unavailable)
        XCTAssertNil(store.timeline)
    }

    func testConnectCodexDelegatesThenRefreshesToAvailable() async throws {
        let client = try ProviderSetupStubClient()
        let store = LocalProductStore(client: client)

        await store.refreshSetup()
        XCTAssertEqual(store.setupSnapshot?.codex.status, "not_logged_in")

        await store.connectCodex()

        XCTAssertEqual(client.connectRequestCount, 1)
        XCTAssertEqual(client.setupRequestCount, 2)
        XCTAssertEqual(
            store.providerConnectionStatus,
            LocalProductProviderConnectResult.fixture(status: "started")
        )
        XCTAssertEqual(store.setupSnapshot?.codex.status, "available")
        XCTAssertEqual(store.setupState, .ready)
    }

    func testConnectCodexPollingCancelsWithoutInventingFailure() async throws {
        let client = try ProviderSetupStubClient(connectsAfterStart: false)
        let store = LocalProductStore(client: client)
        await store.refreshSetup()

        let connection = Task { await store.connectCodex() }
        await Task.yield()
        connection.cancel()
        await connection.value

        XCTAssertEqual(client.connectRequestCount, 1)
        XCTAssertEqual(store.setupSnapshot?.codex.status, "not_logged_in")
        XCTAssertEqual(store.setupState, .ready)
    }

    func testVerifyMiniMaxPresentsReturnedTerminalRevisionAndRefreshes() async throws {
        let client = try ProviderSetupStubClient(miniMaxConfigured: true)
        let store = LocalProductStore(client: client)
        await store.refreshSetup()

        XCTAssertEqual(store.setupSnapshot?.miniMax.revision, 1)
        XCTAssertEqual(store.setupSnapshot?.miniMax.status, "configured")

        await store.verifyMiniMax()

        XCTAssertEqual(client.verifyRequestCount, 1)
        XCTAssertEqual(store.credentialStatus?.revision, 2)
        XCTAssertEqual(store.credentialStatus?.status, "verified")
        XCTAssertEqual(store.setupSnapshot?.miniMax.revision, 2)
        XCTAssertEqual(store.setupSnapshot?.miniMax.status, "verified")
        XCTAssertEqual(store.setupState, .ready)
    }
}

private final class StubLocalProductClient: LocalProductClientProtocol {
    private var snapshots: [Result<LocalProductSnapshot, Error>]
    private(set) var timelineRequestCount = 0
    private(set) var lastTimelineLimit: Int?
    private(set) var lastTimelineTeamID: String?

    init(snapshots: [Result<LocalProductSnapshot, Error>]) {
        self.snapshots = snapshots
    }

    func snapshot(limit: Int) async throws -> LocalProductSnapshot {
        guard !snapshots.isEmpty else {
            throw LocalProductClientError.unavailable
        }
        return try snapshots.removeFirst().get()
    }

    func timeline(
        teamInstanceID: String,
        cursor: String,
        limit: Int
    ) async throws -> LocalProductTimelinePage {
        timelineRequestCount += 1
        lastTimelineLimit = limit
        lastTimelineTeamID = teamInstanceID
        throw LocalProductClientError.notFound
    }
}

final class ProviderSetupStubClient:
    LocalProductClientProtocol,
    LocalProductSetupClientProtocol
{
    private let disconnected: LocalProductSetupSnapshot
    private let connected: LocalProductSetupSnapshot
    private let connectsAfterStart: Bool
    private let miniMaxConfigured: Bool
    private(set) var setupRequestCount = 0
    private(set) var connectRequestCount = 0
    private(set) var verifyRequestCount = 0

    init(
        connectsAfterStart: Bool = true,
        miniMaxConfigured: Bool = false
    ) throws {
        disconnected = try Self.snapshot(
            codexStatus: "not_logged_in",
            miniMaxStatus: miniMaxConfigured ? "configured" : "unconfigured",
            miniMaxRevision: miniMaxConfigured ? 1 : 0
        )
        connected = try Self.snapshot(
            codexStatus: "available",
            miniMaxStatus: miniMaxConfigured ? "configured" : "unconfigured",
            miniMaxRevision: miniMaxConfigured ? 1 : 0
        )
        self.connectsAfterStart = connectsAfterStart
        self.miniMaxConfigured = miniMaxConfigured
    }

    func snapshot(limit: Int) async throws -> LocalProductSnapshot {
        .empty(viewVersion: "view-1")
    }

    func timeline(
        teamInstanceID: String,
        cursor: String,
        limit: Int
    ) async throws -> LocalProductTimelinePage {
        throw LocalProductClientError.notFound
    }

    func setupSnapshot() async throws -> LocalProductSetupSnapshot {
        setupRequestCount += 1
        if miniMaxConfigured {
            return try Self.snapshot(
                codexStatus: "not_logged_in",
                miniMaxStatus: verifyRequestCount == 0 ? "configured" : "verified",
                miniMaxRevision: verifyRequestCount == 0 ? 1 : 2
            )
        }
        return setupRequestCount == 1 || !connectsAfterStart
            ? disconnected
            : connected
    }

    func connectCodex() async throws -> LocalProductProviderConnectResult {
        connectRequestCount += 1
        return .fixture(status: "started")
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
        guard miniMaxConfigured,
              reference == "credential-ref-1",
              revision == 1 else {
            throw LocalProductClientError.invalidRequest
        }
        verifyRequestCount += 1
        return try LocalProductSetupWire.decodeCredentialResult(
            Data(
                """
                {
                  "provider_id": "minimax",
                  "revision": 2,
                  "status": "verified",
                  "reason": ""
                }
                """.utf8
            )
        )
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

    private static func snapshot(
        codexStatus: String,
        miniMaxStatus: String = "unconfigured",
        miniMaxRevision: Int64 = 0
    ) throws -> LocalProductSetupSnapshot {
        try LocalProductSetupWire.decodeSnapshot(
            Data(
                """
                {
                  "schema_version": 1,
                  "view_version": "\(String(repeating: "a", count: 64))",
                  "codex": {
                    "provider_id": "codex",
                    "auth_mode": "native_auth",
                    "credential_reference": "",
                    "revision": 0,
                    "status": "\(codexStatus)",
                    "reason": "\(codexStatus == "available" ? "" : "not_logged_in")"
                  },
                  "minimax": {
                    "provider_id": "minimax",
                    "auth_mode": "brokered",
                    "credential_reference": "\(miniMaxRevision > 0 ? "credential-ref-1" : "")",
                    "revision": \(miniMaxRevision),
                    "status": "\(miniMaxStatus)",
                    "reason": ""
                  },
                  "runtimes": [],
                  "saved_teams": [],
                  "templates": [],
                  "role_options": [],
                  "skills": [],
                  "permissions": [],
                  "resources": []
                }
                """.utf8
            )
        )
    }
}

private extension LocalProductProviderConnectResult {
    static func fixture(status: String) -> Self {
        try! LocalProductSetupWire.decodeProviderConnectResult(
            Data(
                """
                {
                  "provider_id": "codex",
                  "auth_mode": "native_auth",
                  "status": "\(status)"
                }
                """.utf8
            )
        )
    }
}
