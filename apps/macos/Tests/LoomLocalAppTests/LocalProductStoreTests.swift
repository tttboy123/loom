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
                )
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

    func testTimelineLoadsEveryBoundedPageBeforePublishingAuthoritativeHistory() async throws {
        let snapshot = try timelineSnapshot(teamIDs: ["team-1"])
        let first = try timelinePage(
            teamID: "team-1",
            nextCursor: "cursor-1",
            hasMore: true,
            records: [
                timelineRecord(
                    deliveryID: "delivery-source-evidence",
                    kind: "evidence_available",
                    teamID: "team-1"
                )
            ]
        )
        let second = try timelinePage(
            teamID: "team-1",
            nextCursor: "cursor-2",
            hasMore: false,
            records: [
                timelineRecord(
                    deliveryID: "delivery-verifier-evidence",
                    kind: "evidence_available",
                    teamID: "team-1"
                ),
                timelineRecord(
                    deliveryID: "delivery-terminal",
                    kind: "team_terminal",
                    teamID: "team-1",
                    status: "succeeded"
                ),
            ]
        )
        let client = TimelinePagingStubClient(
            snapshot: snapshot,
            pages: ["": first, "cursor-1": second]
        )
        let store = LocalProductStore(client: client)

        await store.refresh()
        store.selectTeam(snapshot.teams[0])
        await store.activateSelectedTeam()

        XCTAssertEqual(
            client.timelineRequests,
            [
                .init(teamID: "team-1", cursor: "", limit: 64),
                .init(teamID: "team-1", cursor: "cursor-1", limit: 64),
            ]
        )
        XCTAssertEqual(store.timelineState, .loaded)
        XCTAssertEqual(
            store.timeline?.records.map(\.deliveryID),
            [
                "delivery-source-evidence",
                "delivery-verifier-evidence",
                "delivery-terminal",
            ]
        )
        XCTAssertEqual(store.timeline?.records.last?.payload.status, "succeeded")
        XCTAssertEqual(store.timeline?.nextCursor, "cursor-2")
        XCTAssertEqual(store.timeline?.hasMore, false)
        XCTAssertNil(store.timeline?.gap)
    }

    func testTimelineRejectsNestedIdentityDriftDuplicateDeliveryAndCursorReplay() async throws {
        let snapshot = try timelineSnapshot(teamIDs: ["team-1"])
        let validRecord = timelineRecord(
            deliveryID: "delivery-1",
            kind: "evidence_available",
            teamID: "team-1"
        )
        let cases: [(String, [String: LocalProductTimelinePage])] = [
            (
                "first_page_schema_drift",
                [
                    "": try timelinePage(
                        schemaVersion: 2,
                        teamID: "team-1",
                        hasMore: false,
                        records: [validRecord]
                    )
                ]
            ),
            (
                "first_page_team_drift",
                [
                    "": try timelinePage(
                        teamID: "team-other",
                        hasMore: false,
                        records: [
                            timelineRecord(
                                deliveryID: "delivery-1",
                                kind: "evidence_available",
                                teamID: "team-other"
                            )
                        ]
                    )
                ]
            ),
            (
                "first_page_empty_view",
                [
                    "": try timelinePage(
                        teamID: "team-1",
                        viewVersion: "",
                        hasMore: false,
                        records: [validRecord]
                    )
                ]
            ),
            (
                "first_page_board_schema_drift",
                [
                    "": try timelinePage(
                        teamID: "team-1",
                        boardSchemaVersion: 2,
                        hasMore: false,
                        records: [validRecord]
                    )
                ]
            ),
            (
                "first_page_board_team_drift",
                [
                    "": try timelinePage(
                        teamID: "team-1",
                        boardTeamID: "team-other",
                        hasMore: false,
                        records: [validRecord]
                    )
                ]
            ),
            (
                "first_page_board_view_drift",
                [
                    "": try timelinePage(
                        teamID: "team-1",
                        boardViewVersion: "view-other",
                        hasMore: false,
                        records: [validRecord]
                    )
                ]
            ),
            (
                "first_page_attention_schema_drift",
                [
                    "": try timelinePage(
                        teamID: "team-1",
                        hasMore: false,
                        records: [validRecord],
                        attention: [timelineAttention(
                            schemaVersion: 2,
                            attentionID: "attention-1",
                            teamID: "team-1"
                        )]
                    )
                ]
            ),
            (
                "first_page_attention_team_drift",
                [
                    "": try timelinePage(
                        teamID: "team-1",
                        hasMore: false,
                        records: [validRecord],
                        attention: [timelineAttention(
                            attentionID: "attention-1",
                            teamID: "team-other"
                        )]
                    )
                ]
            ),
            (
                "first_page_empty_attention_id",
                [
                    "": try timelinePage(
                        teamID: "team-1",
                        hasMore: false,
                        records: [validRecord],
                        attention: [timelineAttention(
                            attentionID: "",
                            teamID: "team-1"
                        )]
                    )
                ]
            ),
            (
                "first_page_duplicate_attention_id",
                [
                    "": try timelinePage(
                        teamID: "team-1",
                        hasMore: false,
                        records: [validRecord],
                        attention: [
                            timelineAttention(
                                attentionID: "attention-1",
                                teamID: "team-1"
                            ),
                            timelineAttention(
                                attentionID: "attention-1",
                                teamID: "team-1"
                            ),
                        ]
                    )
                ]
            ),
            (
                "first_page_record_schema_drift",
                [
                    "": try timelinePage(
                        teamID: "team-1",
                        hasMore: false,
                        records: [timelineRecord(
                            schemaVersion: 2,
                            deliveryID: "delivery-1",
                            kind: "evidence_available",
                            teamID: "team-1"
                        )]
                    )
                ]
            ),
            (
                "first_page_record_team_drift",
                [
                    "": try timelinePage(
                        teamID: "team-1",
                        hasMore: false,
                        records: [timelineRecord(
                            deliveryID: "delivery-1",
                            kind: "evidence_available",
                            teamID: "team-other"
                        )]
                    )
                ]
            ),
            (
                "first_page_empty_delivery_id",
                [
                    "": try timelinePage(
                        teamID: "team-1",
                        hasMore: false,
                        records: [timelineRecord(
                            deliveryID: "",
                            kind: "evidence_available",
                            teamID: "team-1"
                        )]
                    )
                ]
            ),
            (
                "later_page_board_drift",
                [
                    "": try timelinePage(
                        teamID: "team-1",
                        nextCursor: "cursor-1",
                        hasMore: true,
                        records: [validRecord]
                    ),
                    "cursor-1": try timelinePage(
                        teamID: "team-1",
                        boardStatus: "failed",
                        hasMore: false,
                        records: []
                    ),
                ]
            ),
            (
                "later_page_attention_drift",
                [
                    "": try timelinePage(
                        teamID: "team-1",
                        nextCursor: "cursor-1",
                        hasMore: true,
                        records: [validRecord]
                    ),
                    "cursor-1": try timelinePage(
                        teamID: "team-1",
                        hasMore: false,
                        records: [],
                        attention: [timelineAttention(
                            attentionID: "attention-1",
                            teamID: "team-1"
                        )]
                    ),
                ]
            ),
            (
                "later_page_view_drift",
                [
                    "": try timelinePage(
                        teamID: "team-1",
                        nextCursor: "cursor-1",
                        hasMore: true,
                        records: [validRecord]
                    ),
                    "cursor-1": try timelinePage(
                        teamID: "team-1",
                        viewVersion: "view-other",
                        hasMore: false,
                        records: []
                    ),
                ]
            ),
            (
                "duplicate_delivery",
                [
                    "": try timelinePage(
                        teamID: "team-1",
                        nextCursor: "cursor-1",
                        hasMore: true,
                        records: [validRecord]
                    ),
                    "cursor-1": try timelinePage(
                        teamID: "team-1",
                        hasMore: false,
                        records: [validRecord]
                    ),
                ]
            ),
            (
                "replayed_cursor",
                [
                    "": try timelinePage(
                        teamID: "team-1",
                        nextCursor: "cursor-1",
                        hasMore: true,
                        records: [validRecord]
                    ),
                    "cursor-1": try timelinePage(
                        teamID: "team-1",
                        nextCursor: "cursor-1",
                        hasMore: true,
                        records: []
                    ),
                ]
            ),
            (
                "empty_continuation",
                [
                    "": try timelinePage(
                        teamID: "team-1",
                        nextCursor: "",
                        hasMore: true,
                        records: [validRecord]
                    )
                ]
            ),
        ]

        for (name, pages) in cases {
            let client = TimelinePagingStubClient(snapshot: snapshot, pages: pages)
            let store = LocalProductStore(client: client)
            await store.refresh()
            store.selectTeam(snapshot.teams[0])
            await store.activateSelectedTeam()

            XCTAssertNil(store.timeline, name)
            XCTAssertEqual(store.timelineState, .unavailable, name)
        }
    }

    func testTimelineRejectsGapAndBoundedPageOverflowWithoutPartialPublication() async throws {
        let snapshot = try timelineSnapshot(teamIDs: ["team-1"])
        let gapClient = TimelinePagingStubClient(
            snapshot: snapshot,
            pages: [
                "": try timelinePage(
                    teamID: "team-1",
                    hasMore: false,
                    gap: true,
                    records: []
                )
            ]
        )
        let gapStore = LocalProductStore(client: gapClient)
        await gapStore.refresh()
        gapStore.selectTeam(snapshot.teams[0])
        await gapStore.activateSelectedTeam()
        XCTAssertNil(gapStore.timeline)
        XCTAssertEqual(gapStore.timelineState, .unavailable)

        var pages: [String: LocalProductTimelinePage] = [:]
        for index in 0..<8 {
            let cursor = index == 0 ? "" : "cursor-\(index)"
            pages[cursor] = try timelinePage(
                teamID: "team-1",
                nextCursor: "cursor-\(index + 1)",
                hasMore: true,
                records: [
                    timelineRecord(
                        deliveryID: "delivery-\(index)",
                        kind: "node_scheduled",
                        teamID: "team-1"
                    )
                ]
            )
        }
        let overflowClient = TimelinePagingStubClient(snapshot: snapshot, pages: pages)
        let overflowStore = LocalProductStore(client: overflowClient)
        await overflowStore.refresh()
        overflowStore.selectTeam(snapshot.teams[0])
        await overflowStore.activateSelectedTeam()

        XCTAssertEqual(overflowClient.timelineRequests.count, 8)
        XCTAssertNil(overflowStore.timeline)
        XCTAssertEqual(overflowStore.timelineState, .unavailable)
    }

    func testTimelineEnforcesExact512RecordBoundary() async throws {
        let snapshot = try timelineSnapshot(teamIDs: ["team-1"])
        func pages(finalHasMore: Bool, finalRecordCount: Int = 64) throws
            -> [String: LocalProductTimelinePage]
        {
            var result: [String: LocalProductTimelinePage] = [:]
            var deliveryIndex = 0
            for pageIndex in 0..<8 {
                let cursor = pageIndex == 0 ? "" : "cursor-\(pageIndex)"
                let count = pageIndex == 7 ? finalRecordCount : 64
                let records = (0..<count).map { _ in
                    defer { deliveryIndex += 1 }
                    return timelineRecord(
                        deliveryID: "delivery-\(deliveryIndex)",
                        kind: "node_scheduled",
                        teamID: "team-1"
                    )
                }
                result[cursor] = try timelinePage(
                    teamID: "team-1",
                    nextCursor: "cursor-\(pageIndex + 1)",
                    hasMore: pageIndex == 7 ? finalHasMore : true,
                    records: records
                )
            }
            return result
        }

        let acceptedClient = TimelinePagingStubClient(
            snapshot: snapshot,
            pages: try pages(finalHasMore: false)
        )
        let acceptedStore = LocalProductStore(client: acceptedClient)
        await acceptedStore.refresh()
        acceptedStore.selectTeam(snapshot.teams[0])
        await acceptedStore.activateSelectedTeam()
        XCTAssertEqual(acceptedClient.timelineRequests.count, 8)
        XCTAssertEqual(acceptedStore.timelineState, .loaded)
        XCTAssertEqual(acceptedStore.timeline?.records.count, 512)

        for (name, testPages) in [
            ("512_with_continuation", try pages(finalHasMore: true)),
            ("513_final", try pages(finalHasMore: false, finalRecordCount: 65)),
        ] {
            let client = TimelinePagingStubClient(snapshot: snapshot, pages: testPages)
            let store = LocalProductStore(client: client)
            await store.refresh()
            store.selectTeam(snapshot.teams[0])
            await store.activateSelectedTeam()
            XCTAssertNil(store.timeline, name)
            XCTAssertEqual(store.timelineState, .unavailable, name)
        }
    }

    func testNewTeamSelectionFencesLateTimelineSuccessAndError() async throws {
        let snapshot = try timelineSnapshot(teamIDs: ["team-1", "team-2"])
        for lateResult in [
            Result<LocalProductTimelinePage, Error>.success(
                try timelinePage(teamID: "team-1", hasMore: false, records: [])
            ),
            .failure(LocalProductClientError.unavailable),
        ] {
            let client = SuspendedTimelineStubClient(
                snapshot: snapshot,
                immediatePages: [
                    "team-2": try timelinePage(
                        teamID: "team-2",
                        hasMore: false,
                        records: [
                            timelineRecord(
                                deliveryID: "delivery-team-2",
                                kind: "team_terminal",
                                teamID: "team-2",
                                status: "succeeded"
                            )
                        ]
                    )
                ]
            )
            let store = LocalProductStore(client: client)
            await store.refresh()
            store.selectTeam(snapshot.teams[0])
            let staleLoad = Task { await store.activateSelectedTeam() }
            await client.waitForSuspendedRequest()

            store.selectTeam(snapshot.teams[1])
            await store.activateSelectedTeam()
            client.resolveSuspended(with: lateResult)
            await staleLoad.value

            XCTAssertEqual(store.selectedTeamID, "team-2")
            XCTAssertEqual(store.timeline?.teamInstanceID, "team-2")
            XCTAssertEqual(store.timeline?.records.map(\.deliveryID), ["delivery-team-2"])
            XCTAssertEqual(store.timelineState, .loaded)
            XCTAssertEqual(store.connectionState, .online)
        }
    }

    func testCancelledTimelineLoadCannotPublishLateSuccessOrError() async throws {
        let snapshot = try timelineSnapshot(teamIDs: ["team-1"])
        for lateResult in [
            Result<LocalProductTimelinePage, Error>.success(
                try timelinePage(teamID: "team-1", hasMore: false, records: [])
            ),
            .failure(LocalProductClientError.unavailable),
        ] {
            let client = SuspendedTimelineStubClient(snapshot: snapshot)
            let store = LocalProductStore(client: client)
            await store.refresh()
            store.selectTeam(snapshot.teams[0])
            let load = Task { await store.activateSelectedTeam() }
            await client.waitForSuspendedRequest()

            load.cancel()
            client.resolveSuspended(with: lateResult)
            await load.value

            XCTAssertNil(store.timeline)
            XCTAssertEqual(store.timelineState, .idle)
            XCTAssertEqual(store.connectionState, .online)
        }
    }

    func testLeavingMissionFencesLateTimelineSuccess() async throws {
        let snapshot = try LocalProductWire.decodeSnapshot(
            Data(MissionOrchestrationTests.snapshotJSON.utf8)
        )
        let client = SuspendedTimelineStubClient(snapshot: snapshot)
        let store = LocalProductStore(client: client)
        await store.refresh()
        let load = Task {
            await store.openMissionAndActivate("mission/team-1")
        }
        await client.waitForSuspendedRequest()

        store.showMissionBoard()
        client.resolveSuspended(
            with: .success(
                try timelinePage(teamID: "team-1", hasMore: false, records: [])
            )
        )
        await load.value

        XCTAssertEqual(store.workbench.route, .board)
        XCTAssertNil(store.timeline)
        XCTAssertEqual(store.timelineState, .idle)
        XCTAssertEqual(store.connectionState, .online)
    }

    func testNewMissionSelectionFencesLateTimelineSuccessAndError() async throws {
        let snapshot = try twoMissionSnapshot()
        for lateResult in [
            Result<LocalProductTimelinePage, Error>.success(
                try timelinePage(teamID: "team-1", hasMore: false, records: [])
            ),
            .failure(LocalProductClientError.unavailable),
        ] {
            let client = SuspendedTimelineStubClient(
                snapshot: snapshot,
                immediatePages: [
                    "team-2": try timelinePage(
                        teamID: "team-2",
                        hasMore: false,
                        records: [timelineRecord(
                            deliveryID: "delivery-team-2",
                            kind: "team_terminal",
                            teamID: "team-2",
                            status: "succeeded"
                        )]
                    )
                ]
            )
            let store = LocalProductStore(client: client)
            await store.refresh()
            let staleLoad = Task {
                await store.openMissionAndActivate("mission/team-1")
            }
            await client.waitForSuspendedRequest()

            await store.openMissionAndActivate("mission/team-2")
            client.resolveSuspended(with: lateResult)
            await staleLoad.value

            XCTAssertEqual(store.workbench.route, .mission("mission/team-2"))
            XCTAssertEqual(store.timeline?.teamInstanceID, "team-2")
            XCTAssertEqual(store.timeline?.records.map(\.deliveryID), ["delivery-team-2"])
            XCTAssertEqual(store.timelineState, .loaded)
            XCTAssertEqual(store.connectionState, .online)
        }
    }

    func testNewerLoadForSameSelectionFencesOlderSuccessAndError() async throws {
        let snapshot = try timelineSnapshot(teamIDs: ["team-1"])
        for staleResult in [
            Result<LocalProductTimelinePage, Error>.success(
                try timelinePage(teamID: "team-1", hasMore: false, records: [])
            ),
            .failure(LocalProductClientError.unavailable),
        ] {
            let client = SequencedSuspendedTimelineStubClient(snapshot: snapshot)
            let store = LocalProductStore(client: client)
            await store.refresh()
            store.selectTeam(snapshot.teams[0])
            let staleLoad = Task { await store.activateSelectedTeam() }
            await client.waitForRequestCount(1)
            let currentLoad = Task { await store.activateSelectedTeam() }
            await client.waitForRequestCount(2)

            client.resolveRequest(
                at: 1,
                with: .success(
                    try timelinePage(
                        teamID: "team-1",
                        hasMore: false,
                        records: [timelineRecord(
                            deliveryID: "delivery-current",
                            kind: "team_terminal",
                            teamID: "team-1",
                            status: "succeeded"
                        )]
                    )
                )
            )
            await currentLoad.value
            client.resolveRequest(at: 0, with: staleResult)
            await staleLoad.value

            XCTAssertEqual(store.timeline?.records.map(\.deliveryID), ["delivery-current"])
            XCTAssertEqual(store.timelineState, .loaded)
            XCTAssertEqual(store.connectionState, .online)
        }
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
        XCTAssertEqual(store.miniMaxVerificationStatus, "Verified")
        XCTAssertFalse(store.isVerifyingMiniMax)
    }

    func testVerifyMiniMaxRejectsMismatchedAuthoritativeRefresh() async throws {
        let client = try ProviderSetupStubClient(
            miniMaxConfigured: true,
            mismatchedMiniMaxRefresh: true
        )
        let store = LocalProductStore(client: client)
        await store.refreshSetup()

        await store.verifyMiniMax()

        XCTAssertNil(store.credentialStatus)
        XCTAssertEqual(store.setupSnapshot?.miniMax.revision, 1)
        XCTAssertEqual(store.miniMaxVerificationStatus, "Unavailable")
        XCTAssertFalse(store.isVerifyingMiniMax)
        XCTAssertEqual(
            store.setupState,
            .unavailable(reason: "invalid_response")
        )
    }

    func testVerifyMiniMaxPresentsRejectedAndUnavailableTerminalStates() async throws {
        for (status, reason, expected) in [
            ("rejected", "provider_rejected", "Rejected"),
            ("rejected", "timeout", "Unavailable"),
			("rejected", "unavailable", "Unavailable"),
        ] {
            let client = try ProviderSetupStubClient(
                miniMaxConfigured: true,
                miniMaxTerminalStatus: status,
                miniMaxTerminalReason: reason
            )
            let store = LocalProductStore(client: client)
            await store.refreshSetup()
            await store.verifyMiniMax()

            XCTAssertEqual(store.miniMaxVerificationStatus, expected)
            XCTAssertEqual(store.credentialStatus?.status, status)
            XCTAssertEqual(store.credentialStatus?.reason, reason)
            XCTAssertEqual(store.setupState, .ready)
        }
    }

    func testVerifyMiniMaxPresentsConflictWithoutInventingSuccess() async throws {
        let client = try ProviderSetupStubClient(
            miniMaxConfigured: true,
            miniMaxVerifyError: LocalIPCRemoteError(
                code: .conflict,
                recoverable: true
            )
        )
        let store = LocalProductStore(client: client)
        await store.refreshSetup()
        await store.verifyMiniMax()

        XCTAssertEqual(store.miniMaxVerificationStatus, "Conflict")
        XCTAssertNil(store.credentialStatus)
        XCTAssertEqual(store.setupSnapshot?.miniMax.revision, 1)
        XCTAssertEqual(store.setupState, .unavailable(reason: "conflict"))
    }

    func testConfirmedExecutableTeamRefreshesAuthoritativeProductView() async throws {
        let client = try ProviderSetupStubClient(materializesTeam: true)
        let store = LocalProductStore(client: client)

        await store.refresh()
        await store.startBlankBuilder()
        XCTAssertTrue(try XCTUnwrap(store.builderSession).canConfirm)

        await store.confirmBuilder()

        XCTAssertEqual(client.productSnapshotRequestCount, 2)
        XCTAssertTrue(try XCTUnwrap(store.lastConfirmation).teamInstanceCreated)
        XCTAssertEqual(store.snapshot?.teams.count, 1)
        XCTAssertEqual(store.snapshot?.teams.first?.teamInstanceID, "team-instance-fixture")
        XCTAssertTrue(store.snapshot?.teams.first?.executable == true)
    }

    func testPreparedDecisionReadAndNotNowRemainPresentationOnly() async throws {
        let client = try DecisionStubClient()
        let store = LocalProductStore(client: client)
        let read = client.command(operation: "read", action: "read")

        await store.openPreparedDecision(read)
        XCTAssertEqual(store.activeDecisionSheet?.kind, .authorization)
        XCTAssertEqual(client.readCount, 1)
        XCTAssertEqual(client.submitCount, 0)

        await store.submitDecisionAction("not_now")
        XCTAssertNil(store.activeDecisionSheet)
        XCTAssertEqual(client.submitCount, 1)
        XCTAssertEqual(client.lastCommand?.operation, "defer")
        XCTAssertEqual(client.lastCommand?.action, "not_now")
    }

    func testUnpreparedReviewGateIsReadOnlyAndNeverCallsDecisionClient() async throws {
        let client = try DecisionStubClient()
        let store = LocalProductStore(client: client)
        let mission = LocalProductMissionSummary(
            missionID: "mission/team-review-missing",
            teamInstanceID: "team-review-missing",
            title: "Review missing Evidence",
            sourceKind: "team_execution",
            lane: "Review",
            status: "ready_for_review",
            priority: "normal",
            planDigest: String(repeating: "a", count: 64),
            simple: true,
            nodeCount: 1,
            completedNodeCount: 0,
            activeNodeCount: 0,
            reviewNodeCount: 1,
            attentionCount: 1,
            currentNodeID: "main",
            lastMilestone: "Review required",
            teamPulse: [],
            topology: []
        )

        store.openReadOnlyReviewDecision(for: mission)
        XCTAssertEqual(store.activeDecisionSheet?.kind, .review)
        XCTAssertEqual(store.activeDecisionSheet?.prepared, false)
        XCTAssertEqual(store.activeDecisionSheet?.preparedActions, [])

        await store.submitDecisionAction("accept_result")
        XCTAssertNotNil(store.activeDecisionSheet)
        XCTAssertEqual(client.readCount, 0)
        XCTAssertEqual(client.submitCount, 0)

        await store.submitDecisionAction("not_now")
        XCTAssertNil(store.activeDecisionSheet)
        XCTAssertEqual(client.readCount, 0)
        XCTAssertEqual(client.submitCount, 0)
    }

    func testMissionPreflightAndStartUseVisibleConfirmedTeamWithoutTypedIDs() async throws {
        let json = LocalProductModelsTests.snapshotJSON.replacingOccurrences(
            of: "\"teams\":[]",
            with: """
                "teams":[{
                  "team_instance_id":"team-1",
                  "display_name":"Coding Team",
                  "source_kind":"saved_team",
                  "state":"created",
                  "confirmed":true,
                  "executable":true,
                  "read_only":false
                }]
                """
        ).replacingOccurrences(
            of: "\"view_version\":\"view-1\"",
            with: "\"view_version\":\"\(String(repeating: "b", count: 64))\""
        )
        let snapshot = try LocalProductWire.decodeSnapshot(Data(json.utf8))
        let client = ExecutionStubClient(snapshot: snapshot)
        let store = LocalProductStore(client: client)
        await store.refresh()

        await store.preflightMission(
            objective: "Implement the bounded change",
            team: snapshot.teams[0],
            workPackage: LocalProductWorkPackageOption.coding
        )

        XCTAssertEqual(store.executionState, .ready)
        XCTAssertEqual(store.executionPreflight?.teamInstanceID, "team-1")
        XCTAssertEqual(client.commands.map(\.operation), ["preflight"])
        XCTAssertEqual(client.commands[0].missionID, "mission/team-1")
        XCTAssertEqual(client.commands[0].workPackageID, "work-package.coding")

        await store.startPreflightedMission()

        XCTAssertEqual(store.executionState, .running)
        XCTAssertEqual(client.commands.map(\.operation), ["preflight", "start"])
        XCTAssertEqual(client.commands[1].preflightDigest, String(repeating: "d", count: 64))
        XCTAssertEqual(client.timelineRequestCount, 1)
        XCTAssertEqual(store.timelineState, .loaded)
        XCTAssertEqual(store.timeline?.records.first?.payload.textDelta, "Compiling checks")
        XCTAssertEqual(
            store.workbench.route,
            .mission(try XCTUnwrap(store.executionResult?.missionID))
        )
        XCTAssertTrue(store.missionCancelAvailable)

        await store.cancelCurrentMission()

        XCTAssertEqual(store.executionState, .cancelled)
        XCTAssertEqual(client.commands.map(\.operation), ["preflight", "start", "control"])
        XCTAssertEqual(client.commands[2].controlAction, "cancel")
        XCTAssertEqual(client.commands[2].logicalNodeID, "main")
        XCTAssertEqual(client.commands[2].attemptNumber, 1)
        XCTAssertEqual(client.commands[2].claimGeneration, 3)
        XCTAssertEqual(client.commands[2].expectedViewVersion, store.snapshot?.viewVersion)
    }
}

private struct TimelineRequest: Equatable {
    let teamID: String
    let cursor: String
    let limit: Int
}

private final class TimelinePagingStubClient: LocalProductClientProtocol {
    private let fixedSnapshot: LocalProductSnapshot
    private let pages: [String: LocalProductTimelinePage]
    private(set) var timelineRequests: [TimelineRequest] = []

    init(snapshot: LocalProductSnapshot, pages: [String: LocalProductTimelinePage]) {
        fixedSnapshot = snapshot
        self.pages = pages
    }

    func snapshot(limit: Int) async throws -> LocalProductSnapshot { fixedSnapshot }

    func timeline(
        teamInstanceID: String,
        cursor: String,
        limit: Int
    ) async throws -> LocalProductTimelinePage {
        timelineRequests.append(.init(teamID: teamInstanceID, cursor: cursor, limit: limit))
        guard let page = pages[cursor] else {
            throw LocalProductClientError.notFound
        }
        return page
    }
}

private final class SuspendedTimelineStubClient: LocalProductClientProtocol {
    private let fixedSnapshot: LocalProductSnapshot
    private let immediatePages: [String: LocalProductTimelinePage]
    private var suspendedContinuation:
        CheckedContinuation<LocalProductTimelinePage, Error>?

    init(
        snapshot: LocalProductSnapshot,
        immediatePages: [String: LocalProductTimelinePage] = [:]
    ) {
        fixedSnapshot = snapshot
        self.immediatePages = immediatePages
    }

    func snapshot(limit: Int) async throws -> LocalProductSnapshot { fixedSnapshot }

    func timeline(
        teamInstanceID: String,
        cursor: String,
        limit: Int
    ) async throws -> LocalProductTimelinePage {
        if let immediate = immediatePages[teamInstanceID] { return immediate }
        return try await withCheckedThrowingContinuation { continuation in
            suspendedContinuation = continuation
        }
    }

    func waitForSuspendedRequest() async {
        while suspendedContinuation == nil { await Task.yield() }
    }

    func resolveSuspended(with result: Result<LocalProductTimelinePage, Error>) {
        let continuation = suspendedContinuation
        suspendedContinuation = nil
        continuation?.resume(with: result)
    }
}

private final class SequencedSuspendedTimelineStubClient: LocalProductClientProtocol {
    private let fixedSnapshot: LocalProductSnapshot
    private var continuations: [CheckedContinuation<LocalProductTimelinePage, Error>?] = []

    init(snapshot: LocalProductSnapshot) { fixedSnapshot = snapshot }

    func snapshot(limit: Int) async throws -> LocalProductSnapshot { fixedSnapshot }

    func timeline(
        teamInstanceID: String,
        cursor: String,
        limit: Int
    ) async throws -> LocalProductTimelinePage {
        return try await withCheckedThrowingContinuation { continuation in
            continuations.append(continuation)
        }
    }

    func waitForRequestCount(_ count: Int) async {
        while continuations.count < count { await Task.yield() }
    }

    func resolveRequest(
        at index: Int,
        with result: Result<LocalProductTimelinePage, Error>
    ) {
        let continuation = continuations[index]
        continuations[index] = nil
        continuation?.resume(with: result)
    }
}

private func timelineSnapshot(teamIDs: [String]) throws -> LocalProductSnapshot {
    let teams = teamIDs.map { teamID in
        """
        {"team_instance_id":"\(teamID)","display_name":"\(teamID)",
         "source_kind":"saved","state":"ready","confirmed":true,
         "executable":true,"read_only":false}
        """
    }.joined(separator: ",")
    let json = LocalProductModelsTests.snapshotJSON.replacingOccurrences(
        of: "\"teams\":[]",
        with: "\"teams\":[\(teams)]"
    )
    return try LocalProductWire.decodeSnapshot(Data(json.utf8))
}

private func twoMissionSnapshot() throws -> LocalProductSnapshot {
    var object = try XCTUnwrap(
        JSONSerialization.jsonObject(
            with: Data(MissionOrchestrationTests.snapshotJSON.utf8)
        ) as? [String: Any]
    )
    var missions = try XCTUnwrap(object["missions"] as? [[String: Any]])
    var second = try XCTUnwrap(missions.first)
    second["mission_id"] = "mission/team-2"
    second["team_instance_id"] = "team-2"
    second["title"] = "Second Mission"
    missions.append(second)
    object["missions"] = missions
    return try LocalProductWire.decodeSnapshot(
        JSONSerialization.data(withJSONObject: object, options: [.sortedKeys])
    )
}

private func timelineRecord(
    schemaVersion: Int = 1,
    deliveryID: String,
    kind: String,
    teamID: String,
    status: String = ""
) -> String {
    """
    {"schema_version":\(schemaVersion),"delivery_id":"\(deliveryID)","kind":"\(kind)",
     "authority":"journal","team_instance_id":"\(teamID)",
     "logical_node_id":"main","attempt_number":2,
     "source_stream_id":"work-item/work-1","source_sequence":1,
     "source_event_id":"event-1","occurred_at":"2026-08-03T00:00:00Z",
     "cursor":"record-cursor","payload":{"status":"\(status)","reason_code":"",
      "action":"","warning_code":"","retry_at":"","text_delta":"",
      "evidence_digest":"\(String(repeating: "a", count: 64))",
      "cost":{"observed":false,"amount_microunits":null,"currency":""}}}
    """
}

private func timelineAttention(
    schemaVersion: Int = 1,
    attentionID: String,
    teamID: String
) -> String {
    """
    {"schema_version":\(schemaVersion),"attention_id":"\(attentionID)","kind":"blocked",
     "severity":"warning","team_instance_id":"\(teamID)",
     "logical_node_id":"main","work_item_id":"work-1",
     "approval_request_id":"","runtime_instance_id":"runtime-1",
     "status":"blocked","occurred_at":"2026-08-03T00:00:00Z",
     "action_required":"Review the blocked node"}
    """
}

private func timelinePage(
    schemaVersion: Int = 1,
    teamID: String,
    viewVersion: String = "view-1",
    boardTeamID: String? = nil,
    boardViewVersion: String? = nil,
    boardSchemaVersion: Int = 1,
    boardStatus: String = "succeeded",
    nextCursor: String = "final-cursor",
    hasMore: Bool,
    gap: Bool = false,
    records: [String],
    attention: [String] = []
) throws -> LocalProductTimelinePage {
    let gapJSON = gap
        ? """
          {"schema_version":1,"delivery_id":"gap-1","kind":"stream_gap",
           "team_instance_id":"\(teamID)","reason":"cursor_conflict",
           "previous_cursor_digest":"","current_view_version":"\(viewVersion)",
           "artifact_available":false,"artifact_digest":"","recoverable":true,
           "occurred_at":"2026-08-03T00:00:00Z"}
          """
        : "null"
    let json = """
    {"schema_version":1,"team_instance_id":"\(teamID)",
     "view_version":"\(viewVersion)","next_cursor":"\(nextCursor)",
     "has_more":\(hasMore),"gap":\(gapJSON),"records":[\(records.joined(separator: ","))],
     "board":{"schema_version":\(boardSchemaVersion),"team_instance_id":"\(boardTeamID ?? teamID)",
      "plan_digest":"","status":"\(boardStatus)",
      "view_version":"\(boardViewVersion ?? viewVersion)","nodes":[],
      "cost":{"observed":false,"amount_microunits":null,"currency":""}},
     "attention":[\(attention.joined(separator: ","))]}
    """
    let decoded = try LocalProductWire.decodeTimeline(Data(json.utf8))
    guard schemaVersion != decoded.schemaVersion else { return decoded }
    return LocalProductTimelinePage(
        schemaVersion: schemaVersion,
        teamInstanceID: decoded.teamInstanceID,
        viewVersion: decoded.viewVersion,
        nextCursor: decoded.nextCursor,
        hasMore: decoded.hasMore,
        gap: decoded.gap,
        records: decoded.records,
        board: decoded.board,
        attention: decoded.attention
    )
}

private final class ExecutionStubClient:
    LocalProductClientProtocol,
    LocalProductExecutionClientProtocol
{
    private(set) var fixedSnapshot: LocalProductSnapshot
    private(set) var commands: [LocalProductExecutionCommand] = []
    private(set) var timelineRequestCount = 0

    init(snapshot: LocalProductSnapshot) { fixedSnapshot = snapshot }

    func snapshot(limit: Int) async throws -> LocalProductSnapshot { fixedSnapshot }

    func timeline(
        teamInstanceID: String,
        cursor: String,
        limit: Int
    ) async throws -> LocalProductTimelinePage {
        timelineRequestCount += 1
        let body = """
            {"schema_version":1,"team_instance_id":"\(teamInstanceID)",
             "view_version":"\(fixedSnapshot.viewVersion)","next_cursor":"",
             "has_more":false,"gap":null,"records":[{
              "schema_version":1,"delivery_id":"delivery-1","kind":"tentative_output",
              "authority":"tentative","team_instance_id":"\(teamInstanceID)",
              "logical_node_id":"main","attempt_number":1,"source_stream_id":"run/run-1",
              "source_sequence":1,"source_event_id":"","occurred_at":"2026-08-01T00:00:00Z",
              "cursor":"","payload":{"status":"","reason_code":"","action":"",
               "warning_code":"","retry_at":"","text_delta":"Compiling checks",
               "evidence_digest":"","cost":{"observed":false,"amount_microunits":null,"currency":""}}}],
             "board":{"schema_version":1,"team_instance_id":"\(teamInstanceID)",
              "plan_digest":"\(String(repeating: "c", count: 64))","status":"running",
              "view_version":"\(fixedSnapshot.viewVersion)","nodes":[{
               "logical_node_id":"main","status":"running","dependency_satisfied":true,
               "current_attempt":1,"work_item_id":"work-1","run_id":"run-1",
               "runtime_instance_id":"runtime-pi","agent_instance_id":"agent-main",
               "verification_status":"pending","recovery_action":"","retry_at":""}],
              "cost":{"observed":false,"amount_microunits":null,"currency":""}},"attention":[]}
            """
        return try LocalProductWire.decodeTimeline(Data(body.utf8))
    }

    func executeMission(
        _ command: LocalProductExecutionCommand
    ) async throws -> LocalProductExecutionEnvelope {
        commands.append(command)
        let body: String
        if command.operation == "preflight" {
            body = """
                {"schema_version":1,"operation":"preflight","preflight":{
                  "schema_version":1,"mission_id":"\(command.missionID)",
                  "team_instance_id":"\(command.teamInstanceID)",
                  "work_package_id":"\(command.workPackageID)",
                  "work_package_digest":"\(command.workPackageDigest)",
                  "view_version":"\(command.expectedViewVersion)",
                  "plan_digest":"\(String(repeating: "c", count: 64))",
                  "preflight_digest":"\(String(repeating: "d", count: 64))",
                  "expires_at":"2026-08-01T12:05:00Z",
                  "runtime_instance_id":"runtime-pi","runtime_profile_id":"pi-default",
                  "model_id":"qwen","auth_mode":"brokered","capacity_available":1,
                  "budget_status":"unavailable","side_effects":[],
                  "permission_scopes":["workspace"],"approval_points":["before_start"],
                  "nodes":[{"logical_node_id":"main","title":"Implement the bounded change",
                    "role":"main","depends_on":[],"max_attempts":2}]}}
                """
        } else if command.operation == "start" {
            let postStartJSON = MissionOrchestrationTests.snapshotJSON
                .replacingOccurrences(of: "mission/team-1", with: command.missionID)
                .replacingOccurrences(
                    of: "\"teams\":[]",
                    with: """
                        "teams":[{"team_instance_id":"\(command.teamInstanceID)",
                         "display_name":"Coding Team","source_kind":"saved_team",
                         "state":"created","confirmed":true,"executable":true,"read_only":false}]
                        """
                )
                .replacingOccurrences(
                    of: "\"runs\":[]",
                    with: """
                        "runs":[{"run_id":"run-1","work_item_id":"work-1",
                         "phase":"running","terminal_status":"","terminal_reason":"",
                         "runtime_instance_id":"runtime-pi","agent_instance_id":"agent-main",
                         "claim_generation":3}]
                        """
                )
            fixedSnapshot = try LocalProductWire.decodeSnapshot(Data(postStartJSON.utf8))
            body = """
                {"schema_version":1,"operation":"start","result":{
                  "schema_version":1,"mission_id":"\(command.missionID)",
                  "team_instance_id":"\(command.teamInstanceID)","status":"running",
                  "view_version":"\(String(repeating: "e", count: 64))",
                  "execution_digest":"\(String(repeating: "f", count: 64))"}}
                """
        } else {
            let cancelled = MissionOrchestrationTests.snapshotJSON
                .replacingOccurrences(of: "mission/team-1", with: command.missionID)
                .replacingOccurrences(
                    of: "\"teams\":[]",
                    with: """
                        "teams":[{"team_instance_id":"\(command.teamInstanceID)",
                         "display_name":"Coding Team","source_kind":"saved_team",
                         "state":"created","confirmed":true,"executable":true,"read_only":false}]
                        """
                )
                .replacingOccurrences(
                    of: "\"runs\":[]",
                    with: """
                        "runs":[{"run_id":"run-1","work_item_id":"work-1",
                         "phase":"terminal","terminal_status":"cancelled","terminal_reason":"cancelled",
                         "runtime_instance_id":"runtime-pi","agent_instance_id":"agent-main",
                         "claim_generation":3}]
                        """
                )
                .replacingOccurrences(of: "human_required", with: "cancelled")
            fixedSnapshot = try LocalProductWire.decodeSnapshot(Data(cancelled.utf8))
            body = """
                {"schema_version":1,"operation":"control","result":{
                  "schema_version":1,"mission_id":"\(command.missionID)",
                  "team_instance_id":"\(command.teamInstanceID)","status":"cancelled",
                  "view_version":"\(fixedSnapshot.viewVersion)",
                  "execution_digest":"\(command.executionDigest)"}}
                """
        }
        return try LocalProductExecutionWire.decodeEnvelope(Data(body.utf8))
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

private final class DecisionStubClient:
    LocalProductClientProtocol,
    LocalProductDecisionClientProtocol
{
    private(set) var readCount = 0
    private(set) var submitCount = 0
    private(set) var lastCommand: LocalProductDecisionCommand?
    private let sheet: LocalProductDecisionSheet

    init() throws {
        sheet = try LocalProductDecisionWire.decodeSheet(
            Data(
                """
                {
                  "schema_version":1,
                  "kind":"authorization",
                  "mission_id":"mission/team-1",
                  "team_instance_id":"team-1",
                  "view_version":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
                  "decision_id":"decision-1",
                  "decision_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
                  "title":"Authorization required",
                  "summary":"Review the prepared command.",
                  "requester":"Release Team",
                  "target":"repository",
                  "command_type":"local process",
                  "network_access":"none",
                  "credential_access":"none",
                  "permission_scope":"this Mission",
                  "attempt_scope":"Attempt 1",
                  "expected_evidence":"accepted Evidence",
                  "technical_details":[],
                  "actions":["not_now","deny","edit_scope","allow_once"],
                  "prepared_actions":["deny","allow_once"],
                  "prepared":true,
                  "logical_node_id":"main",
                  "attempt_number":1,
                  "claim_generation":1
                }
                """.utf8
            )
        )
    }

    func command(
        operation: String,
        action: String
    ) -> LocalProductDecisionCommand {
        LocalProductDecisionCommand(
            operation: operation,
            kind: sheet.kind,
            action: action,
            missionID: sheet.missionID,
            teamInstanceID: sheet.teamInstanceID,
            viewVersion: sheet.viewVersion,
            decisionID: sheet.decisionID,
            decisionDigest: sheet.decisionDigest,
            logicalNodeID: sheet.logicalNodeID,
            attemptNumber: sheet.attemptNumber,
            claimGeneration: sheet.claimGeneration,
            correlationID: "11111111-1111-4111-8111-111111111111"
        )
    }

    func snapshot(limit: Int) async throws -> LocalProductSnapshot {
        .empty(viewVersion: "view-2")
    }

    func timeline(
        teamInstanceID: String,
        cursor: String,
        limit: Int
    ) async throws -> LocalProductTimelinePage {
        throw LocalProductClientError.notFound
    }

    func readMissionDecision(
        _ command: LocalProductDecisionCommand
    ) async throws -> LocalProductDecisionSheet {
        readCount += 1
        lastCommand = command
        return sheet
    }

    func decideMission(
        _ command: LocalProductDecisionCommand
    ) async throws -> LocalProductDecisionResult {
        submitCount += 1
        lastCommand = command
        return try LocalProductDecisionWire.decodeResult(
            Data(
                """
                {
                  "schema_version":1,
                  "mission_id":"mission/team-1",
                  "decision_id":"decision-1",
                  "status":"pending",
                  "authoritative":false,
                  "view_version":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
                }
                """.utf8
            )
        )
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
    private let mismatchedMiniMaxRefresh: Bool
    private let miniMaxTerminalStatus: String
    private let miniMaxTerminalReason: String
    private let miniMaxVerifyError: LocalIPCRemoteError?
    private let materializesTeam: Bool
    private var builderConfirmed = false
    private(set) var setupRequestCount = 0
    private(set) var productSnapshotRequestCount = 0
    private(set) var connectRequestCount = 0
    private(set) var verifyRequestCount = 0

    init(
        connectsAfterStart: Bool = true,
        miniMaxConfigured: Bool = false,
        mismatchedMiniMaxRefresh: Bool = false,
        miniMaxTerminalStatus: String = "verified",
        miniMaxTerminalReason: String = "",
        miniMaxVerifyError: LocalIPCRemoteError? = nil,
        materializesTeam: Bool = false
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
        self.mismatchedMiniMaxRefresh = mismatchedMiniMaxRefresh
        self.miniMaxTerminalStatus = miniMaxTerminalStatus
        self.miniMaxTerminalReason = miniMaxTerminalReason
        self.miniMaxVerifyError = miniMaxVerifyError
        self.materializesTeam = materializesTeam
    }

    func snapshot(limit: Int) async throws -> LocalProductSnapshot {
        productSnapshotRequestCount += 1
        guard materializesTeam, builderConfirmed else {
            return .empty(viewVersion: "view-1")
        }
        let json = LocalProductModelsTests.snapshotJSON.replacingOccurrences(
            of: "\"teams\":[]",
            with: """
                "teams":[{
                  "team_instance_id":"team-instance-fixture",
                  "display_name":"Controlled Team",
                  "source_kind":"saved_team",
                  "state":"created",
                  "confirmed":true,
                  "executable":true,
                  "read_only":false
                }]
                """
        )
        return try LocalProductWire.decodeSnapshot(Data(json.utf8))
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
                miniMaxStatus: verifyRequestCount == 0
                    ? "configured"
                    : miniMaxTerminalStatus,
                miniMaxRevision: verifyRequestCount == 0
                    ? 1
                    : (mismatchedMiniMaxRefresh ? 3 : 2),
                miniMaxReason: verifyRequestCount == 0
                    ? ""
                    : miniMaxTerminalReason
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
        guard materializesTeam else {
            throw LocalProductClientError.unavailable
        }
        return try LocalProductSetupWire.decodeBuilderSession(
            Data(
                """
                {
                  "schema_version":1,"draft_id":"draft-fixture","revision":4,
                  "source":"blank","catalog_digest":"\(String(repeating: "b", count: 64))",
                  "view_version":"\(String(repeating: "e", count: 64))",
                  "content_digest":"\(String(repeating: "c", count: 64))",
                  "binding_digest":"\(String(repeating: "d", count: 64))",
                  "question":{"id":"","prompt":"","options":[]},
                  "preview":{"name":"Controlled Team","purpose":"One bounded Mission",
                    "roles":[],"permissions":[],"resources":[],"compatibility_gaps":[],
                    "requested_concurrency":1,"maximum_budget_credits":100,
                    "estimated_maximum_cost":"up to 100 credits"},
                  "can_confirm":true
                }
                """.utf8
            )
        )
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
        guard materializesTeam, session.canConfirm else {
            throw LocalProductClientError.unavailable
        }
        builderConfirmed = true
        return try LocalProductSetupWire.decodeBuilderConfirmation(
            Data(
                """
                {"team_definition_id":"team-fixture","team_definition_version":1,
                 "team_definition_digest":"\(String(repeating: "f", count: 64))",
                 "status":"active","team_instance_created":true,"run_created":false}
                """.utf8
            )
        )
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
            revision == 1
        else {
            throw LocalProductClientError.invalidRequest
        }
        verifyRequestCount += 1
        if let miniMaxVerifyError {
            throw miniMaxVerifyError
        }
        return try LocalProductSetupWire.decodeCredentialResult(
            Data(
                """
                {
                  "provider_id": "minimax",
                  "revision": 2,
                  "status": "\(miniMaxTerminalStatus)",
                  "reason": "\(miniMaxTerminalReason)"
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
        miniMaxRevision: Int64 = 0,
        miniMaxReason: String = ""
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
                    "reason": "\(miniMaxReason)"
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

extension LocalProductProviderConnectResult {
    fileprivate static func fixture(status: String) -> Self {
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
