import XCTest
@testable import LoomLocalAppCore

final class LocalProductDecisionModelsTests: XCTestCase {
    func testStrictDecisionWireDecodesAllThreeSheetKinds() throws {
        for kind in ["authorization", "review", "recovery"] {
            let decision = try LocalProductDecisionWire.decodeSheet(
                Data(Self.sheetJSON(kind: kind).utf8)
            )
            XCTAssertEqual(decision.kind.rawValue, kind)
            XCTAssertEqual(decision.missionID, "mission/team-1")
            XCTAssertFalse(decision.actions.isEmpty)
            XCTAssertEqual(
                decision.preparedActions,
                ["deny", "allow_once"]
            )
        }
    }

    func testDecisionWireRejectsUnknownFieldsAndNullCollections() {
        let unknown = Self.sheetJSON(kind: "authorization")
            .replacingOccurrences(
                of: "\"actions\":[",
                with: "\"unknown\":true,\"actions\":["
            )
        XCTAssertThrowsError(
            try LocalProductDecisionWire.decodeSheet(Data(unknown.utf8))
        )
        let nullActions = Self.sheetJSON(kind: "authorization")
            .replacingOccurrences(
                of: "\"actions\":[\"not_now\",\"deny\",\"allow_once\"]",
                with: "\"actions\":null"
            )
        XCTAssertThrowsError(
            try LocalProductDecisionWire.decodeSheet(
                Data(nullActions.utf8)
            )
        )
        let nullPreparedActions = Self.sheetJSON(kind: "authorization")
            .replacingOccurrences(
                of: "\"prepared_actions\":[\"deny\",\"allow_once\"]",
                with: "\"prepared_actions\":null"
            )
        XCTAssertThrowsError(
            try LocalProductDecisionWire.decodeSheet(
                Data(nullPreparedActions.utf8)
            )
        )
    }

    private static func sheetJSON(kind: String) -> String {
        """
        {
          "schema_version":1,
          "kind":"\(kind)",
          "mission_id":"mission/team-1",
          "team_instance_id":"team-1",
          "view_version":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
          "decision_id":"decision-1",
          "decision_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
          "title":"Decision required",
          "summary":"Review the bounded action.",
          "requester":"Release Team",
          "target":"repository",
          "command_type":"read",
          "network_access":"none",
          "credential_access":"none",
          "permission_scope":"repo.read",
          "attempt_scope":"attempt 1",
          "expected_evidence":"verified result",
          "technical_details":[],
          "actions":["not_now","deny","allow_once"],
          "prepared_actions":["deny","allow_once"],
          "prepared":true,
          "logical_node_id":"main",
          "attempt_number":1,
          "claim_generation":1
        }
        """
    }
}
