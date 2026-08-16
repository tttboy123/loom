import XCTest
@testable import LoomLocalAppCore

final class LocalProductDecisionModelsTests: XCTestCase {
    func testStrictDecisionWireDecodesAllFourSheetKinds() throws {
        for kind in ["authorization", "review", "recovery", "fallback"] {
            let decision = try LocalProductDecisionWire.decodeSheet(
                Data(Self.sheetJSON(kind: kind).utf8)
            )
            XCTAssertEqual(decision.kind.rawValue, kind)
            XCTAssertEqual(decision.missionID, "mission/team-1")
            XCTAssertFalse(decision.actions.isEmpty)
            let expected = kind == "fallback"
                ? ["reject_fallback", "approve_fallback"]
                : ["deny", "allow_once"]
            XCTAssertEqual(decision.preparedActions, expected)
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
        let actions = kind == "fallback"
            ? "\"not_now\",\"reject_fallback\",\"approve_fallback\""
            : "\"not_now\",\"deny\",\"allow_once\""
        let prepared = kind == "fallback"
            ? "\"reject_fallback\",\"approve_fallback\""
            : "\"deny\",\"allow_once\""
        return """
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
          "actions":[\(actions)],
          "prepared_actions":[\(prepared)],
          "prepared":true,
          "logical_node_id":"main",
          "attempt_number":1,
          "claim_generation":1
        }
        """
    }
}
