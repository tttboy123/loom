import AppKit
import CryptoKit
import SwiftUI
import XCTest
@testable import LoomLocalAppCore
@testable import LoomLocalAppUI

@MainActor
final class LocalProductExperienceViewTests: XCTestCase {
    func testExplicitRecoveryTargetMeetsNativeAccessibilityMinimum() {
        XCTAssertGreaterThanOrEqual(
            LoomGraphite.minimumActionTarget,
            44
        )
    }

    func testMissionRailRendersTeamsAttentionAndHistoryComparePages()
        async throws
    {
        let snapshot = try ExperienceFixtures.populatedSnapshot()
        XCTAssertEqual(
            missionDisplayTitle(
                candidate: "team-first",
                missionID: "mission/team-first",
                teamInstanceID: "team-first",
                teams: snapshot.teams
            ),
            "Release Team"
        )
        for run in snapshot.runs {
            XCTAssertNotEqual(
                missionRuntimeDisplayName(run: run, snapshot: snapshot),
                run.runtimeInstanceID
            )
        }
        let store = LocalProductStore(
            client: ExperienceViewStubClient(results: [.success(snapshot)])
        )
        await store.refresh()

        store.showMissionTeams()
        let teams = try XCTUnwrap(render(
            MissionWorkbench(store: store, showRail: false),
            colorScheme: .light,
            dynamicTypeSize: .large
        ))
        store.showMissionAttention()
        let attention = try XCTUnwrap(render(
            MissionWorkbench(store: store, showRail: false),
            colorScheme: .light,
            dynamicTypeSize: .large
        ))
        store.showMissionLibrary()
        let library = try XCTUnwrap(render(
            MissionWorkbench(store: store, showRail: false),
            colorScheme: .light,
            dynamicTypeSize: .large
        ))

        XCTAssertEqual(teams.pixelWidth, 1_100)
        XCTAssertEqual(attention.pixelWidth, 1_100)
        XCTAssertEqual(library.pixelWidth, 1_100)
        XCTAssertEqual(Set([teams.digest, attention.digest, library.digest]).count, 3)
        XCTAssertGreaterThan(teams.png.count, 20_000)
        XCTAssertGreaterThan(attention.png.count, 20_000)
        XCTAssertGreaterThan(library.png.count, 20_000)
    }

    func testMissionInspectorTabsExposeDistinctSafeReadOnlyContent() throws {
        let snapshot = try LocalProductWire.decodeSnapshot(
            Data(MissionOrchestrationTests.snapshotJSON.utf8)
        )
        let mission = try XCTUnwrap(snapshot.missions.first)
        let timeline = try LocalProductWire.decodeTimeline(Data("""
        {
          "schema_version":1,
          "team_instance_id":"team-1",
          "view_version":"view-1",
          "next_cursor":"",
          "has_more":false,
          "gap":null,
          "records":[
            {
              "schema_version":1,
              "delivery_id":"delivery-evidence",
              "kind":"evidence_available",
              "authority":"journal",
              "team_instance_id":"team-1",
              "logical_node_id":"main",
              "attempt_number":1,
              "source_stream_id":"evidence/internal-evidence-id",
              "source_sequence":1,
              "source_event_id":"internal-event-id",
              "occurred_at":"2026-08-03T00:00:00Z",
              "cursor":"cursor-1",
              "payload":{
                "status":"",
                "reason_code":"",
                "action":"",
                "warning_code":"",
                "retry_at":"",
                "text_delta":"",
                "evidence_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
                "cost":{"observed":false,"amount_microunits":null,"currency":""}
              }
            },
            {
              "schema_version":1,
              "delivery_id":"delivery-verification",
              "kind":"verification_recorded",
              "authority":"journal",
              "team_instance_id":"team-1",
              "logical_node_id":"main",
              "attempt_number":1,
              "source_stream_id":"work-item/internal-work-id",
              "source_sequence":1,
              "source_event_id":"internal-verification-event",
              "occurred_at":"2026-08-03T00:00:01Z",
              "cursor":"cursor-2",
              "payload":{
                "status":"accepted",
                "reason_code":"",
                "action":"",
                "warning_code":"",
                "retry_at":"",
                "text_delta":"",
                "evidence_digest":"",
                "cost":{"observed":false,"amount_microunits":null,"currency":""}
              }
            }
          ],
          "board":{
            "schema_version":1,
            "team_instance_id":"team-1",
            "plan_digest":"",
            "status":"succeeded",
            "view_version":"view-1",
            "nodes":[],
            "cost":{"observed":false,"amount_microunits":null,"currency":""}
          },
          "attention":[]
        }
        """.utf8))

        let sections = MissionInspectorTab.allCases.map {
            missionInspectorSection(
                tab: $0,
                record: mission,
                timeline: timeline
            )
        }
        XCTAssertEqual(
            sections.map(\.heading),
            ["Team Pulse", "Plan", "Changes", "Evidence"]
        )
        XCTAssertEqual(Set(sections.map(\.rows)).count, 4)
        XCTAssertTrue(sections[0].rows.joined().contains("Attempt 1"))
        XCTAssertTrue(sections[1].rows.joined().contains("Main"))
        XCTAssertEqual(
            sections[2].rows,
            ["Verification recorded · Main · Attempt 1"]
        )
        XCTAssertEqual(
            sections[3].rows,
            ["Evidence 1 · Main · Attempt 1"]
        )
        for section in sections {
            let visible = section.rows.joined(separator: " ")
            XCTAssertFalse(visible.contains("internal-"))
            XCTAssertFalse(visible.contains(String(repeating: "a", count: 64)))
        }

        let unavailable = missionInspectorSection(
            tab: .evidence,
            record: mission,
            timeline: nil
        )
        XCTAssertEqual(unavailable.rows, [])
        XCTAssertEqual(
            unavailable.emptyMessage,
            "Evidence is unavailable until complete authoritative activity loads."
        )

        let encodedTimeline = try XCTUnwrap(
            String(
                data: JSONEncoder().encode(timeline),
                encoding: .utf8
            )
        )
        let incompleteTimeline = try LocalProductWire.decodeTimeline(
            Data(encodedTimeline.replacingOccurrences(
                of: "\"has_more\":false",
                with: "\"has_more\":true"
            ).utf8)
        )
        let gapTimeline = try LocalProductWire.decodeTimeline(
            Data(encodedTimeline.replacingOccurrences(
                of: "\"gap\":null",
                with: """
                \"gap\":{
                  \"schema_version\":1,
                  \"delivery_id\":\"stream-gap\",
                  \"kind\":\"stream_gap\",
                  \"team_instance_id\":\"team-1\",
                  \"reason\":\"cursor_stale\",
                  \"previous_cursor_digest\":\"\",
                  \"current_view_version\":\"view-1\",
                  \"artifact_available\":false,
                  \"artifact_digest\":\"\",
                  \"recoverable\":true,
                  \"occurred_at\":\"2026-08-03T00:00:02Z\"
                }
                """
            ).utf8)
        )
        for partialTimeline in [incompleteTimeline, gapTimeline] {
            for tab in [MissionInspectorTab.changes, .evidence] {
                let partial = missionInspectorSection(
                    tab: tab,
                    record: mission,
                    timeline: partialTimeline
                )
                XCTAssertEqual(partial.rows, [])
                XCTAssertTrue(
                    partial.emptyMessage.contains("unavailable"),
                    "\(tab) must not present incomplete history as empty"
                )
            }
        }
    }

    func testSideTaskDrawerAlwaysPresentsEveryContractField() throws {
        let digest = String(repeating: "a", count: 64)
        let sideTask = try JSONDecoder().decode(
            LocalProductSideTaskSummary.self,
            from: Data("""
            {
              "side_task_id":"side-1",
              "parent_mission_id":"mission/team-1",
              "parent_team_instance_id":"team-1",
              "parent_task_id":"work-1",
              "parent_run_id":"run-1",
              "parent_claim_generation":1,
              "parent_execution_digest":"\(digest)",
              "side_execution_team_instance_id":"team-side-1",
              "purpose":"diagnosis",
              "mode":"decision_required",
              "title":"Diagnose the failure",
              "status":"decision_required",
              "source_generation":1,
              "handoff_version":1,
              "handoff_digest":"\(digest)",
              "summary_artifact_digest":"\(digest)",
              "what_happened":"Authorized result",
              "authorized_findings":[],
              "evidence_references":[],
              "artifact_references":[],
              "risk":"medium",
              "uncertainties":[],
              "scope_delta":[],
              "decision_options":["absorb","discard"],
              "recommended_option":"",
              "recommendation_authority":"proposal_only",
              "usage_observed":false,
              "usage_microunits":0,
              "usage_currency":"",
              "decision_deadline":"2026-08-03T16:00:00Z",
              "available_decisions":["absorb","discard"],
              "effect_status":"none"
            }
            """.utf8)
        )

        let presentation = sideTaskDrawerPresentation(sideTask)
        XCTAssertEqual(presentation.purpose, "Purpose · Diagnosis")
        XCTAssertEqual(
            presentation.uncertainty,
            "Uncertainty · None recorded"
        )
        XCTAssertEqual(
            presentation.scopeDelta,
            "Scope · No parent scope expansion"
        )
        XCTAssertEqual(
            presentation.nextAction,
            "Next action · Choose an explicit parent decision"
        )

        var decidedObject = try XCTUnwrap(
            JSONSerialization.jsonObject(
                with: JSONEncoder().encode(sideTask)
            ) as? [String: Any]
        )
        decidedObject["status"] = "decided"
        decidedObject["available_decisions"] = []

        decidedObject["effect_status"] = "pending"
        let pending = try JSONDecoder().decode(
            LocalProductSideTaskSummary.self,
            from: JSONSerialization.data(withJSONObject: decidedObject)
        )
        XCTAssertEqual(
            sideTaskDrawerPresentation(pending).nextAction,
            "Next action · Wait for the authorized parent effect"
        )

        decidedObject["effect_status"] = "completed"
        let completed = try JSONDecoder().decode(
            LocalProductSideTaskSummary.self,
            from: JSONSerialization.data(withJSONObject: decidedObject)
        )
        XCTAssertEqual(
            sideTaskDrawerPresentation(completed).nextAction,
            "Next action · Parent effect completed"
        )

        decidedObject["effect_status"] = "none"
        let noEffect = try JSONDecoder().decode(
            LocalProductSideTaskSummary.self,
            from: JSONSerialization.data(withJSONObject: decidedObject)
        )
        XCTAssertEqual(
            sideTaskDrawerPresentation(noEffect).nextAction,
            "Next action · Parent decision recorded; no parent effect required"
        )
    }

    func testMissionWorkspaceSnapshotPresentationFailsClosed() {
        XCTAssertEqual(
            missionSnapshotPresentation(
                connection: .online,
                hasSnapshot: true
            ),
            .current
        )
        XCTAssertEqual(
            missionSnapshotPresentation(
                connection: .partial(reason: "partial_view"),
                hasSnapshot: true
            ),
            .partial("partial_view")
        )
        XCTAssertEqual(
            missionSnapshotPresentation(
                connection: .stale(reason: "stale_view"),
                hasSnapshot: true
            ),
            .preserved("stale_view")
        )
        XCTAssertEqual(
            missionSnapshotPresentation(
                connection: .offline(reason: "state_unavailable"),
                hasSnapshot: true
            ),
            .preserved("state_unavailable")
        )
        XCTAssertEqual(
            missionSnapshotPresentation(
                connection: .online,
                hasSnapshot: false
            ),
            .unavailable("state_unavailable")
        )

        let current = missionAttentionEmptyCopy(presentation: .current)
        XCTAssertEqual(current.title, "Nothing needs you")
        let partial = missionAttentionEmptyCopy(
            presentation: .partial("partial_view")
        )
        XCTAssertEqual(partial.title, "No recorded attention in this view")
        XCTAssertTrue(partial.detail.contains("Some items may be missing"))
        let preserved = missionAttentionEmptyCopy(
            presentation: .preserved("state_unavailable")
        )
        XCTAssertEqual(
            preserved.title,
            "No recorded attention in this view"
        )
        XCTAssertTrue(preserved.detail.contains("New items may be missing"))
    }

    func testMinimumLayoutHasNoRequiredMotion() async throws {
        let snapshot = try ExperienceFixtures.populatedSnapshot()
        let store = LocalProductStore(
            client: ExperienceViewStubClient(results: [.success(snapshot)])
        )
        await store.refresh()
        let standard = try XCTUnwrap(
            render(
                ContentView(store: store, refreshOnAppear: false),
                colorScheme: .light,
                dynamicTypeSize: .large,
                width: 720,
                height: 560
            )
        )
        XCTAssertEqual(standard.pixelWidth, 720)
        XCTAssertEqual(standard.pixelHeight, 560)
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .appendingPathComponent(
                "../../Sources/LoomLocalAppUI/ContentView.swift"
            )
            .standardizedFileURL
        let source = try String(contentsOf: sourceURL, encoding: .utf8)
        for requiredMotion in ["withAnimation", ".animation(", "TimelineView"] {
            XCTAssertFalse(
                source.contains(requiredMotion),
                "Primary workspace requires motion through \(requiredMotion)"
            )
        }
    }

    func testRealContentViewRendersNonconnectingStateAndAppearanceMatrix()
        async throws
    {
        let populated = try ExperienceFixtures.populatedSnapshot()
        let partial = try ExperienceFixtures.snapshot(partial: true)
        let stale = try ExperienceFixtures.snapshot(stale: true)

        let fixtures: [ExperienceViewFixture] = [
            .loading(),
            .connected(snapshot: .empty(viewVersion: "empty")),
            .connected(snapshot: populated),
            .connected(snapshot: partial),
            .connected(snapshot: stale),
            .offline(),
            .offlinePreserving(populated),
            .fatal(),
        ]

        var stateDigests = Set<String>()
        var digestOwners: [String: String] = [:]
        for fixture in fixtures {
            let store = LocalProductStore(client: fixture.client)
            for _ in 0..<fixture.refreshCount {
                await store.refresh()
            }
            XCTAssertEqual(
                LocalProductExperience(
                    snapshot: store.snapshot,
                    connectionState: store.connectionState
                ).state,
                fixture.expectedState
            )
            for scheme in [ColorScheme.light, .dark] {
                let rendered = render(
                    ContentView(
                        store: store,
                        refreshOnAppear: false
                    ),
                    colorScheme: scheme,
                    dynamicTypeSize: .large
                )
                let image = try XCTUnwrap(
                    rendered,
                    "\(fixture.name) failed in \(scheme)"
                )
                XCTAssertEqual(image.pixelWidth, 1_100)
                XCTAssertEqual(image.pixelHeight, 720)
                XCTAssertGreaterThan(image.png.count, 20_000)
                XCTAssertGreaterThan(image.colorBucketCount, 12)
                XCTAssertGreaterThan(
                    image.sidebarContrastPixelCount,
                    180,
                    "\(fixture.name) rendered a visually empty sidebar in \(scheme)"
                )
                let owner = "\(fixture.name)-\(scheme)"
                if let previous = digestOwners[image.digest] {
                    XCTFail(
                        "duplicate rendered state \(owner) matches \(previous)"
                    )
                } else {
                    digestOwners[image.digest] = owner
                    stateDigests.insert(image.digest)
                }
            }
        }
        XCTAssertEqual(stateDigests.count, fixtures.count * 2)

        let accessibilityStore = LocalProductStore(
            client: ExperienceViewStubClient(results: [.success(populated)])
        )
        await accessibilityStore.refresh()
        let accessibilityImage = try XCTUnwrap(
            render(
                ContentView(
                    store: accessibilityStore,
                    refreshOnAppear: false
                ),
                colorScheme: .light,
                dynamicTypeSize: .accessibility3,
                width: 780
            )
        )
        XCTAssertEqual(accessibilityImage.pixelWidth, 780)
        XCTAssertEqual(accessibilityImage.pixelHeight, 720)
        XCTAssertGreaterThan(accessibilityImage.png.count, 20_000)
        XCTAssertGreaterThan(accessibilityImage.colorBucketCount, 12)
        XCTAssertGreaterThan(
            accessibilityImage.sidebarContrastPixelCount,
            180
        )
        XCTAssertFalse(stateDigests.contains(accessibilityImage.digest))
    }

    func testExportPopulatedPreviewOnlyAtAuthorizedEvidencePath()
        async throws
    {
        guard let requestedPath = ProcessInfo.processInfo.environment[
            "LOOM_UI_PREVIEW_PATH"
        ], let compactRequestedPath = ProcessInfo.processInfo.environment[
            "LOOM_UI_COMPACT_PREVIEW_PATH"
        ], let darkRequestedPath = ProcessInfo.processInfo.environment[
            "LOOM_UI_DARK_PREVIEW_PATH"
        ], let darkCompactRequestedPath = ProcessInfo.processInfo.environment[
            "LOOM_UI_DARK_COMPACT_PREVIEW_PATH"
        ], let missionRoomRequestedPath = ProcessInfo.processInfo.environment[
            "LOOM_UI_MISSION_ROOM_PATH"
        ] else {
            throw XCTSkip(
                "Wide and compact preview export is enabled only for visual audit"
            )
        }
        let requestedURL = try validatedPreviewURL(
            requestedPath: requestedPath,
            expectedURL: expectedPreviewURL(
                currentDirectory: FileManager.default.currentDirectoryPath
            )
        )
        let compactRequestedURL = try validatedPreviewURL(
            requestedPath: compactRequestedPath,
            expectedURL: expectedCompactPreviewURL(
                currentDirectory: FileManager.default.currentDirectoryPath
            )
        )
        let darkRequestedURL = try validatedPreviewURL(
            requestedPath: darkRequestedPath,
            expectedURL: expectedDarkPreviewURL(
                currentDirectory: FileManager.default.currentDirectoryPath
            )
        )
        let darkCompactRequestedURL = try validatedPreviewURL(
            requestedPath: darkCompactRequestedPath,
            expectedURL: expectedDarkCompactPreviewURL(
                currentDirectory: FileManager.default.currentDirectoryPath
            )
        )
        let missionRoomRequestedURL = try validatedPreviewURL(
            requestedPath: missionRoomRequestedPath,
            expectedURL: expectedMissionRoomPreviewURL(
                currentDirectory: FileManager.default.currentDirectoryPath
            )
        )

        let snapshot = try LocalProductWire.decodeSnapshot(
            Data(MissionOrchestrationTests.snapshotJSON.utf8)
        )
        let store = LocalProductStore(
            client: ExperienceViewStubClient(results: [.success(snapshot)])
        )
        await store.refresh()
        guard let rendered = render(
            ContentView(
                store: store,
                refreshOnAppear: false
            ),
            colorScheme: .light,
            dynamicTypeSize: .large
        ) else {
            return XCTFail("Could not encode the non-connecting preview")
        }
        try rendered.png.write(to: requestedURL, options: .atomic)
        XCTAssertGreaterThan(rendered.png.count, 20_000)
        let compact = try XCTUnwrap(
            render(
                ContentView(
                    store: store,
                    refreshOnAppear: false
                ),
                colorScheme: .light,
                dynamicTypeSize: .large,
                width: 780,
                height: 720
            )
        )
        try compact.png.write(to: compactRequestedURL, options: .atomic)
        XCTAssertGreaterThan(compact.png.count, 20_000)
        XCTAssertNotEqual(rendered.digest, compact.digest)
        let dark = try XCTUnwrap(
            render(
                ContentView(store: store, refreshOnAppear: false),
                colorScheme: .dark,
                dynamicTypeSize: .large
            )
        )
        try dark.png.write(to: darkRequestedURL, options: .atomic)
        let darkCompact = try XCTUnwrap(
            render(
                ContentView(store: store, refreshOnAppear: false),
                colorScheme: .dark,
                dynamicTypeSize: .large,
                width: 780,
                height: 720
            )
        )
        try darkCompact.png.write(
            to: darkCompactRequestedURL,
            options: .atomic
        )
        XCTAssertNotEqual(dark.digest, rendered.digest)
        XCTAssertNotEqual(darkCompact.digest, compact.digest)
        store.openMission("mission/team-1")
        let missionRoom = try XCTUnwrap(
            render(
                ContentView(store: store, refreshOnAppear: false),
                colorScheme: .light,
                dynamicTypeSize: .large
            )
        )
        try missionRoom.png.write(
            to: missionRoomRequestedURL,
            options: .atomic
        )
        XCTAssertNotEqual(missionRoom.digest, rendered.digest)
    }

    private func render<Root: View>(
        _ view: Root,
        colorScheme: ColorScheme,
        dynamicTypeSize: DynamicTypeSize,
        width: Int = 1_100,
        height: Int = 720
    ) -> RenderedPreview? {
        let rootView = view
                .frame(width: CGFloat(width), height: CGFloat(height))
                .environment(\.colorScheme, colorScheme)
                .environment(\.dynamicTypeSize, dynamicTypeSize)
        let hostingView = NSHostingView(rootView: rootView)
        hostingView.frame = NSRect(
            origin: .zero,
            size: CGSize(width: width, height: height)
        )
        hostingView.layoutSubtreeIfNeeded()
        guard let bitmap = NSBitmapImageRep(
            bitmapDataPlanes: nil,
            pixelsWide: width,
            pixelsHigh: height,
            bitsPerSample: 8,
            samplesPerPixel: 4,
            hasAlpha: true,
            isPlanar: false,
            colorSpaceName: .deviceRGB,
            bytesPerRow: 0,
            bitsPerPixel: 0
        ) else {
            return nil
        }
        bitmap.size = hostingView.bounds.size
        hostingView.cacheDisplay(
            in: hostingView.bounds,
            to: bitmap
        )
        guard let png = bitmap.representation(
            using: .png,
            properties: [:]
        ) else {
            return nil
        }
        guard bitmap.pixelsWide == width,
              bitmap.pixelsHigh == height else {
            return nil
        }
        var buckets = Set<UInt32>()
        var sidebarContrastPixelCount = 0
        let step = 12
        for y in stride(from: 0, to: bitmap.pixelsHigh, by: step) {
            for x in stride(from: 0, to: bitmap.pixelsWide, by: step) {
                guard let color = bitmap.colorAt(x: x, y: y)?
                    .usingColorSpace(.deviceRGB) else {
                    continue
                }
                let red = UInt32((color.redComponent * 31).rounded())
                let green = UInt32((color.greenComponent * 31).rounded())
                let blue = UInt32((color.blueComponent * 31).rounded())
                let alpha = UInt32((color.alphaComponent * 31).rounded())
                buckets.insert(
                    red << 15 | green << 10 | blue << 5 | alpha
                )
            }
        }
        if let reference = bitmap.colorAt(x: 24, y: 100)?
            .usingColorSpace(.deviceRGB) {
            for y in stride(from: 100, to: min(620, height), by: 2) {
                for x in stride(from: 24, to: min(190, width), by: 2) {
                    guard let color = bitmap.colorAt(x: x, y: y)?
                        .usingColorSpace(.deviceRGB) else {
                        continue
                    }
                    let contrast =
                        abs(color.redComponent - reference.redComponent) +
                        abs(color.greenComponent - reference.greenComponent) +
                        abs(color.blueComponent - reference.blueComponent)
                    if contrast > 0.24 {
                        sidebarContrastPixelCount += 1
                    }
                }
            }
        }
        return RenderedPreview(
            png: png,
            digest: SHA256.hash(data: png)
                .map { String(format: "%02x", $0) }
                .joined(),
            colorBucketCount: buckets.count,
            sidebarContrastPixelCount: sidebarContrastPixelCount,
            pixelWidth: bitmap.pixelsWide,
            pixelHeight: bitmap.pixelsHigh
        )
    }

    private func validatedPreviewURL(
        requestedPath: String,
        expectedURL: URL
    ) throws -> URL {
        let requestedURL = URL(fileURLWithPath: requestedPath)
            .standardizedFileURL
        let standardizedExpectedURL = expectedURL.standardizedFileURL
        guard requestedURL == standardizedExpectedURL else {
            throw PreviewPathError.outsideOwnedEvidence
        }
        let expectedParent = standardizedExpectedURL
            .deletingLastPathComponent()
        guard expectedParent == expectedParent
            .resolvingSymlinksInPath()
            .standardizedFileURL else {
            throw PreviewPathError.symlinkEscape
        }
        if let values = try? requestedURL.resourceValues(
            forKeys: [.isSymbolicLinkKey]
        ), values.isSymbolicLink == true {
            throw PreviewPathError.symlinkEscape
        }
        return requestedURL
    }

    private func expectedPreviewURL(currentDirectory: String) -> URL {
        URL(fileURLWithPath: currentDirectory)
            .appendingPathComponent(
                "../../.loom-evidence/phase2a/P2A-W2/" +
                    "mission-workbench-wide-light.png"
            )
            .standardizedFileURL
    }

    private func expectedCompactPreviewURL(
        currentDirectory: String
    ) -> URL {
        URL(fileURLWithPath: currentDirectory)
            .appendingPathComponent(
                "../../.loom-evidence/phase2a/P2A-W2/" +
                    "mission-workbench-compact-light.png"
            )
            .standardizedFileURL
    }

    private func expectedDarkPreviewURL(currentDirectory: String) -> URL {
        URL(fileURLWithPath: currentDirectory)
            .appendingPathComponent(
                "../../.loom-evidence/phase2a/P2A-W2/" +
                    "mission-workbench-wide-dark.png"
            )
            .standardizedFileURL
    }

    private func expectedDarkCompactPreviewURL(
        currentDirectory: String
    ) -> URL {
        URL(fileURLWithPath: currentDirectory)
            .appendingPathComponent(
                "../../.loom-evidence/phase2a/P2A-W2/" +
                    "mission-workbench-compact-dark.png"
            )
            .standardizedFileURL
    }

    private func expectedMissionRoomPreviewURL(
        currentDirectory: String
    ) -> URL {
        URL(fileURLWithPath: currentDirectory)
            .appendingPathComponent(
                "../../.loom-evidence/phase2a/P2A-W2/" +
                    "mission-room-wide-light.png"
            )
            .standardizedFileURL
    }

    private func expectedProviderPreviewURL(
        currentDirectory: String
    ) -> URL {
        URL(fileURLWithPath: currentDirectory)
            .appendingPathComponent(
                "../../.loom-evidence/phase2a/P2A-W2/" +
                    "provider-connection-directory.png"
            )
            .standardizedFileURL
    }

    func testPreviewPathValidationRejectsMismatchAndSymlink() throws {
        let currentDirectory = FileManager.default.currentDirectoryPath
        XCTAssertThrowsError(
            try validatedPreviewURL(
                requestedPath: "/private/tmp/not-owned.png",
                expectedURL: expectedPreviewURL(
                    currentDirectory: currentDirectory
                )
            )
        )

        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(
            at: root,
            withIntermediateDirectories: false
        )
        defer { try? FileManager.default.removeItem(at: root) }
        let outside = root.appendingPathComponent("outside.png")
        let expected = root.appendingPathComponent("expected.png")
        try FileManager.default.createSymbolicLink(
            at: expected,
            withDestinationURL: outside
        )
        defer { try? FileManager.default.removeItem(at: expected) }
        XCTAssertThrowsError(
            try validatedPreviewURL(
                requestedPath: expected.path,
                expectedURL: expected
            )
        )

        let outsideDirectory = root.appendingPathComponent("outside")
        try FileManager.default.createDirectory(
            at: outsideDirectory,
            withIntermediateDirectories: false
        )
        let linkedParent = root.appendingPathComponent("linked-parent")
        try FileManager.default.createSymbolicLink(
            at: linkedParent,
            withDestinationURL: outsideDirectory
        )
        let parentEscaped = linkedParent.appendingPathComponent("preview.png")
        XCTAssertThrowsError(
            try validatedPreviewURL(
                requestedPath: parentEscaped.path,
                expectedURL: parentEscaped
            )
        )
    }
}

private struct RenderedPreview {
    let png: Data
    let digest: String
    let colorBucketCount: Int
    let sidebarContrastPixelCount: Int
    let pixelWidth: Int
    let pixelHeight: Int
}

private enum PreviewPathError: Error {
    case outsideOwnedEvidence
    case symlinkEscape
}

private struct ExperienceViewFixture {
    let name: String
    let client: ExperienceViewStubClient
    let refreshCount: Int
    let expectedState: LocalProductExperienceState

    static func loading() -> Self {
        Self(
            name: "loading",
            client: ExperienceViewStubClient(
                results: [.success(.empty(viewVersion: "loading"))]
            ),
            refreshCount: 0,
            expectedState: .loading
        )
    }

    static func connected(snapshot: LocalProductSnapshot) -> Self {
        Self(
            name: snapshot.partial
                ? "partial"
                : snapshot.stale ? "stale" : "connected",
            client: ExperienceViewStubClient(results: [.success(snapshot)]),
            refreshCount: 1,
            expectedState: snapshot.partial
                ? .partial
                : snapshot.stale
                    ? .stalePreserved
                    : snapshot == .empty(viewVersion: "empty")
                        ? .connectedEmpty
                        : .connectedPopulated
        )
    }

    static func offline() -> Self {
        Self(
            name: "offline",
            client: ExperienceViewStubClient(
                results: [.failure(LocalProductClientError.unavailable)]
            ),
            refreshCount: 1,
            expectedState: .offline
        )
    }

    static func offlinePreserving(_ snapshot: LocalProductSnapshot) -> Self {
        Self(
            name: "offline-preserved",
            client: ExperienceViewStubClient(
                results: [
                    .success(snapshot),
                    .failure(LocalProductClientError.unavailable),
                ]
            ),
            refreshCount: 2,
            expectedState: .offlinePreserved
        )
    }

    static func fatal() -> Self {
        Self(
            name: "fatal",
            client: ExperienceViewStubClient(
                results: [
                    .failure(
                        LocalIPCRemoteError(
                            code: .unauthorizedPeer,
                            recoverable: false
                        )
                    ),
                ]
            ),
            refreshCount: 1,
            expectedState: .fatal
        )
    }
}

private final class ExperienceViewStubClient: LocalProductClientProtocol {
    private var results: [Result<LocalProductSnapshot, Error>]

    init(results: [Result<LocalProductSnapshot, Error>]) {
        self.results = results
    }

    func snapshot(limit: Int) async throws -> LocalProductSnapshot {
        guard !results.isEmpty else {
            throw LocalProductClientError.unavailable
        }
        return try results.removeFirst().get()
    }

    func timeline(
        teamInstanceID: String,
        cursor: String,
        limit: Int
    ) async throws -> LocalProductTimelinePage {
        throw LocalProductClientError.notFound
    }
}
