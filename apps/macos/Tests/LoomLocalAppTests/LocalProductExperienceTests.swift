import Foundation
import XCTest
@testable import LoomLocalAppCore

final class LocalProductExperienceTests: XCTestCase {
    func testPrimaryDestinationsAreTaskFirstAndBounded() {
        XCTAssertEqual(
            LocalProductWorkspaceState().tasks.map(\.title),
            ["New task"]
        )
        XCTAssertEqual(
            LocalProductInspectorTab.allCases.map(\.rawValue),
            ["Team", "Context", "Changes", "Evidence"]
        )
        XCTAssertFalse(
            LocalProductInteractionCopy.primaryFlow.contains("Home")
        )
        XCTAssertFalse(
            LocalProductInteractionCopy.primaryFlow.contains("System")
        )
    }

    func testHomeAndWorkPreserveFrozenSourceOrder() throws {
        let snapshot = try ExperienceFixtures.populatedSnapshot()
        let experience = LocalProductExperience(
            snapshot: snapshot,
            connectionState: .online
        )

        XCTAssertEqual(
            experience.homeGroups,
            [.attention, .workActivity, .teams, .systemReadiness]
        )
        XCTAssertEqual(
            experience.workActivity.map(\.runID),
            ["run-first", "run-second"]
        )
        XCTAssertEqual(
            experience.acceptedEvidence.map(\.evidenceID),
            ["evidence-first", "evidence-second"]
        )
        XCTAssertEqual(
            experience.attentionActions,
            ["Review blocked work"]
        )
        XCTAssertFalse(
            LocalProductInteractionCopy.primaryFlow.contains("Compare")
        )
    }

    func testPresentationStatesDistinguishAvailabilityAndPreservedViews() {
        let empty = LocalProductSnapshot.empty(viewVersion: "view-empty")

        XCTAssertEqual(
            LocalProductExperience(
                snapshot: nil,
                connectionState: .loading
            ).state,
            .loading
        )
        XCTAssertEqual(
            LocalProductExperience(
                snapshot: empty,
                connectionState: .online
            ).state,
            .connectedEmpty
        )
        XCTAssertEqual(
            LocalProductExperience(
                snapshot: nil,
                connectionState: .offline(reason: "unavailable")
            ).state,
            .offline
        )
        XCTAssertEqual(
            LocalProductExperience(
                snapshot: empty,
                connectionState: .offline(reason: "unavailable")
            ).state,
            .offlinePreserved
        )
        XCTAssertEqual(
            LocalProductExperience(
                snapshot: empty,
                connectionState: .stale(reason: "stale_view")
            ).state,
            .stalePreserved
        )
        XCTAssertEqual(
            LocalProductExperience(
                snapshot: empty,
                connectionState: .partial(reason: "partial_view")
            ).state,
            .partial
        )
        XCTAssertEqual(
            LocalProductExperience(
                snapshot: nil,
                connectionState: .fatal(reason: "unauthorized_peer")
            ).state,
            .fatal
        )
    }

    func testVisibleFrozenCopyIsFunctionalAndContainsNoEmDash() {
        XCTAssertEqual(LocalProductCopy.workActivity, "Work activity")
        XCTAssertEqual(
            LocalProductCopy.homeTitle,
            "Your local agent workspace"
        )
        XCTAssertTrue(
            LocalProductCopy.all.allSatisfy {
                !$0.contains("—") && !$0.contains("–")
            }
        )
    }

    func testVisibleNameFallsBackWhenDecoratedValueSanitizesToInternalID() {
        XCTAssertEqual(
            LocalProductExperience.visibleName(
                "\u{001B}[31mteam-internal\u{001B}[0m",
                internalID: "team-internal",
                fallback: "Team"
            ),
            "Team"
        )
        XCTAssertEqual(
            LocalProductExperience.visibleName(
                "Release Team",
                internalID: "team-internal",
                fallback: "Team"
            ),
            "Release Team"
        )
    }
}

enum ExperienceFixtures {
    static func populatedSnapshot() throws -> LocalProductSnapshot {
        var json = LocalProductModelsTests.snapshotJSON
        json = json.replacingOccurrences(
            of: "\"teams\":[]",
            with: """
            "teams":[{
              "team_instance_id":"team-first",
              "display_name":"Release Team",
              "source_kind":"saved",
              "state":"ready",
              "confirmed":true,
              "executable":true,
              "read_only":false
            }]
            """
        )
        json = json.replacingOccurrences(
            of: "\"runs\":[]",
            with: """
            "runs":[{
              "run_id":"run-first",
              "work_item_id":"work-first",
              "phase":"verification",
              "terminal_status":"",
              "terminal_reason":"",
              "runtime_instance_id":"runtime-first",
              "agent_instance_id":"agent-first",
              "claim_generation":1
            },{
              "run_id":"run-second",
              "work_item_id":"work-second",
              "phase":"complete",
              "terminal_status":"accepted",
              "terminal_reason":"",
              "runtime_instance_id":"runtime-second",
              "agent_instance_id":"agent-second",
              "claim_generation":2
            }]
            """
        )
        json = json.replacingOccurrences(
            of: "\"evidence\":[]",
            with: """
            "evidence":[{
              "evidence_id":"evidence-first",
              "work_item_id":"work-first",
              "digest":"digest-first"
            },{
              "evidence_id":"evidence-second",
              "work_item_id":"work-second",
              "digest":"digest-second"
            }]
            """
        )
        json = json.replacingOccurrences(
            of: "\"attention\":[]",
            with: """
            "attention":[{
              "schema_version":1,
              "attention_id":"attention-first",
              "kind":"blocked",
              "severity":"high",
              "team_instance_id":"team-first",
              "logical_node_id":"node-first",
              "work_item_id":"work-first",
              "approval_request_id":"",
              "runtime_instance_id":"",
              "status":"open",
              "occurred_at":"2026-07-29T00:00:00Z",
              "action_required":"Review blocked work"
            }]
            """
        )
        return try LocalProductWire.decodeSnapshot(Data(json.utf8))
    }

    static func snapshot(
        partial: Bool = false,
        stale: Bool = false
    ) throws -> LocalProductSnapshot {
        var json = LocalProductModelsTests.snapshotJSON
        json = json.replacingOccurrences(
            of: "\"partial\":false",
            with: "\"partial\":\(partial)"
        )
        json = json.replacingOccurrences(
            of: "\"stale\":false",
            with: "\"stale\":\(stale)"
        )
        let reason: String
        if stale {
            reason = "stale_view"
        } else if partial {
            reason = "partial_view"
        } else {
            reason = ""
        }
        json = json.replacingOccurrences(
            of: "\"reason\":\"\"",
            with: "\"reason\":\"\(reason)\""
        )
        return try LocalProductWire.decodeSnapshot(Data(json.utf8))
    }
}
