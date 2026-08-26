import XCTest
@testable import LoomLocalAppCore

final class MissionOrchestrationTests: XCTestCase {
    func testBoardUsesExactLifecycleAndKeepsAttentionAsStatus() throws {
        let snapshot = try LocalProductWire.decodeSnapshot(
            Data(Self.snapshotJSON.utf8)
        )
        XCTAssertEqual(snapshot.schemaVersion, 2)
        XCTAssertEqual(snapshot.missions.map(\.lane), ["Orchestrating"])
        XCTAssertEqual(snapshot.missions.first?.status, "human_required")
        XCTAssertFalse(
            Set(snapshot.missions.map(\.lane)).contains("Needs You")
        )
        XCTAssertEqual(snapshot.preparedDecisions.first?.kind, .authorization)
        XCTAssertEqual(
            snapshot.preparedDecisions.first?.missionID,
            "mission/team-1"
        )
    }

    func testSchemaTwoRejectsNullPreparedDecisionCollection() throws {
        var object = try XCTUnwrap(
            JSONSerialization.jsonObject(
                with: Data(Self.snapshotJSON.utf8)
            ) as? [String: Any]
        )
        object["prepared_decisions"] = NSNull()
        let invalid = try JSONSerialization.data(withJSONObject: object)
        XCTAssertThrowsError(
            try LocalProductWire.decodeSnapshot(invalid)
        )
    }

    func testBoardMissionRoundTripPreservesPerMissionContinuity() {
        var workspace = MissionWorkspaceState()
        workspace.mergeMissions([
            MissionListItem(
                id: "mission/team-1",
                title: "Ship reviewed change",
                lane: .orchestrating,
                status: "running"
            ),
            MissionListItem(
                id: "mission/team-2",
                title: "Review API",
                lane: .review,
                status: "ready_for_review"
            ),
        ])
        workspace.openMission("mission/team-1")
        workspace.updateComposerDraft("Keep the verifier independent")
        workspace.selectPermissionMode(.planOnly)
        workspace.selectInspector(.evidence)
        workspace.updateThreadAnchor("attempt-2")
        workspace.setDeveloperDetailsExpanded(true)
        workspace.showBoard()
        workspace.openMission("mission/team-2")
        workspace.updateComposerDraft("Request one more test")
        workspace.showBoard()
        workspace.openMission("mission/team-1")

        XCTAssertEqual(
            workspace.selectedContinuity.composerDraft,
            "Keep the verifier independent"
        )
        XCTAssertEqual(workspace.selectedContinuity.inspector, .evidence)
        XCTAssertEqual(
            workspace.selectedContinuity.permissionMode,
            .planOnly
        )
        XCTAssertEqual(
            workspace.selectedContinuity.threadAnchor,
            "attempt-2"
        )
        XCTAssertTrue(
            workspace.selectedContinuity.developerDetailsExpanded
        )
    }

    func testWorkspaceRailRoutesAreRealAndPreserveMissionContinuity() {
        var workspace = MissionWorkspaceState()
        workspace.mergeMissions([
            MissionListItem(
                id: "mission/team-1",
                title: "Ship reviewed change",
                lane: .complete,
                status: "succeeded"
            ),
        ])
        workspace.openMission("mission/team-1")
        workspace.updateComposerDraft("Preserve this draft")

        workspace.showTeams()
        XCTAssertEqual(workspace.route, .teams)
        workspace.showAttention()
        XCTAssertEqual(workspace.route, .attention)
        workspace.showLibrary()
        XCTAssertEqual(workspace.route, .library)
        workspace.openMission("mission/team-1")

        XCTAssertEqual(
            workspace.selectedContinuity.composerDraft,
            "Preserve this draft"
        )
    }

    static let snapshotJSON = """
    {
      "schema_version":2,
      "view_version":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "partial":false,
      "stale":false,
      "reason":"",
      "runtimes":[],
      "teams":[],
      "missions":[{
        "schema_version":1,
        "mission_id":"mission/team-1",
        "team_instance_id":"team-1",
        "title":"Ship reviewed change",
        "source_kind":"saved_team",
        "lane":"Orchestrating",
        "status":"human_required",
        "priority":"normal",
        "plan_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
        "simple":true,
        "node_count":1,
        "completed_node_count":0,
        "active_node_count":0,
        "review_node_count":0,
        "attention_count":1,
        "current_node_id":"main",
        "last_milestone":"Human decision required",
        "team_pulse":[{
          "agent_instance_id":"agent-main",
          "runtime_instance_id":"runtime-pi",
          "role":"main",
          "state":"waiting",
          "node_id":"main",
          "attempt_number":1
        }],
        "topology":[{
          "logical_node_id":"main",
          "title":"Main",
          "agent_instance_id":"agent-main",
          "runtime_instance_id":"runtime-pi",
          "role":"main",
          "depends_on":[],
          "max_attempts":2,
          "status":"human_required",
          "attempt_number":1,
          "ready":false
        }]
      }],
      "runs":[],
      "evidence":[],
      "attention":[],
      "prepared_decisions":[{
        "schema_version":1,
        "operation":"read",
        "kind":"authorization",
        "action":"read",
        "mission_id":"mission/team-1",
        "team_instance_id":"team-1",
        "view_version":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
        "decision_id":"decision-1",
        "decision_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
        "logical_node_id":"main",
        "attempt_number":1,
        "claim_generation":1,
        "correlation_id":"00000000-0000-4000-8000-000000000000"
      }],
      "runtime_page":{"next_cursor":"","has_more":false},
      "team_page":{"next_cursor":"","has_more":false},
      "mission_page":{"next_cursor":"mission/team-1","has_more":false},
      "run_page":{"next_cursor":"","has_more":false},
      "evidence_page":{"next_cursor":"","has_more":false}
    }
    """
    func testMissionWorkspaceBoardHideCompletedTogglesLane() {
        var workspace = MissionWorkspaceState()
        XCTAssertFalse(workspace.boardHideCompleted)
        workspace.updateBoardHideCompleted(true)
        XCTAssertTrue(workspace.boardHideCompleted)
        workspace.updateBoardHideCompleted(false)
        XCTAssertFalse(workspace.boardHideCompleted)
    }

}
