import Foundation
import XCTest

@testable import LoomLocalAppCore

final class LocalProductExecutionModelsTests: XCTestCase {
    func testStrictPreflightEnvelopeBindsExactProductIdentity() throws {
        let envelope = try LocalProductExecutionWire.decodeEnvelope(
            Data(Self.preflightEnvelope.utf8)
        )

        XCTAssertEqual(envelope.operation, "preflight")
        XCTAssertNil(envelope.result)
        XCTAssertEqual(envelope.preflight?.workPackageID, "work-package.coding")
        XCTAssertEqual(envelope.preflight?.nodes.map(\.logicalNodeID), ["main"])
        XCTAssertEqual(envelope.preflight?.sideEffects, [])

        let unknown = Self.preflightEnvelope.replacingOccurrences(
            of: "\"operation\":\"preflight\",",
            with: "\"operation\":\"preflight\",\"unknown\":true,"
        )
        XCTAssertThrowsError(
            try LocalProductExecutionWire.decodeEnvelope(Data(unknown.utf8))
        )
    }

    func testMissionExecutionCommandEncodesClosedPreflightShape() throws {
        let command = LocalProductExecutionCommand.preflight(
            missionID: "mission/team-1",
            teamInstanceID: "team-1",
            workPackageID: "work-package.coding",
            workPackageDigest: String(repeating: "a", count: 64),
            objective: "Implement bounded change",
            expectedViewVersion: String(repeating: "b", count: 64),
            correlationID: "11111111-1111-4111-8111-111111111111"
        )
        let object = try XCTUnwrap(
            JSONSerialization.jsonObject(with: JSONEncoder().encode(command))
                as? [String: Any]
        )

        XCTAssertEqual(object["schema_version"] as? Int, 1)
        XCTAssertEqual(object["operation"] as? String, "preflight")
        XCTAssertNil(object["control_action"])
        XCTAssertNil(object["preflight_digest"])
    }

    func testMissionExecutionResultAcceptsOnlyProjectedClosedStatuses() throws {
        let valid = """
            {"schema_version":1,"operation":"start","result":{
              "schema_version":1,"mission_id":"mission/team-1","team_instance_id":"team-1",
              "status":"awaiting_recovery",
              "view_version":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
              "execution_digest":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
            }}
            """
        XCTAssertEqual(
            try LocalProductExecutionWire.decodeEnvelope(Data(valid.utf8)).result?.status,
            "awaiting_recovery"
        )

        let unknown = valid.replacingOccurrences(
            of: "\"awaiting_recovery\"",
            with: "\"guessed_status\""
        )
        XCTAssertThrowsError(
            try LocalProductExecutionWire.decodeEnvelope(Data(unknown.utf8))
        )
    }

    static let preflightEnvelope = """
        {
          "schema_version":1,
          "operation":"preflight",
          "preflight":{
            "schema_version":1,
            "mission_id":"mission/team-1",
            "team_instance_id":"team-1",
            "work_package_id":"work-package.coding",
            "work_package_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
            "view_version":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
            "plan_digest":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
            "preflight_digest":"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
            "expires_at":"2026-08-01T12:05:00Z",
            "runtime_instance_id":"runtime-pi",
            "runtime_profile_id":"pi-default",
            "model_id":"qwen",
            "auth_mode":"brokered",
            "capacity_available":1,
            "budget_status":"unavailable",
            "side_effects":[],
            "permission_scopes":["workspace"],
            "approval_points":["before_start"],
            "nodes":[{
              "logical_node_id":"main",
              "title":"Implement bounded change",
              "role":"main",
              "depends_on":[],
              "max_attempts":2
            }]
          }
        }
        """
}
