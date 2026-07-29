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
