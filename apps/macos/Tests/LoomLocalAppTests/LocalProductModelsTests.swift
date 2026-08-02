import Foundation
import XCTest
@testable import LoomLocalAppCore

final class LocalProductModelsTests: XCTestCase {
    func testStrictSnapshotDecodingAcceptsExactSchema() throws {
        let snapshot = try LocalProductWire.decodeSnapshot(
            Data(Self.snapshotJSON.utf8)
        )

        XCTAssertEqual(snapshot.schemaVersion, 1)
        XCTAssertEqual(snapshot.viewVersion, "view-1")
        XCTAssertEqual(snapshot.runtimes.map(\.runtimeInstanceID), ["pi-local"])
        XCTAssertTrue(snapshot.teams.isEmpty)
        XCTAssertEqual(snapshot.health.projection, "unknown")
    }

    func testStrictSnapshotV2RoundTripsBoundedServiceHealth() throws {
        let snapshot = try LocalProductWire.decodeSnapshot(
            Data(Self.snapshotV2JSON.utf8)
        )

        XCTAssertEqual(snapshot.schemaVersion, 2)
        XCTAssertEqual(snapshot.health.daemon, "serving_request")
        XCTAssertEqual(snapshot.health.journal, "available")
        XCTAssertEqual(snapshot.health.projection, "current")

        let missingHealth = Self.snapshotV2JSON.replacingOccurrences(
            of: "\"health\":{\"daemon\":\"serving_request\",\"journal\":\"available\",\"projection\":\"current\"},",
            with: ""
        )
        let legacy = try LocalProductWire.decodeSnapshot(
            Data(missingHealth.utf8)
        )
        XCTAssertEqual(legacy.health, .unknown)
    }

    func testStrictSnapshotDecodingRejectsUnknownAndDuplicateKeys() {
        let unknown = Self.snapshotJSON.replacingOccurrences(
            of: "\"reason\":\"\",",
            with: "\"reason\":\"\",\"unexpected\":true,"
        )
        XCTAssertThrowsError(
            try LocalProductWire.decodeSnapshot(Data(unknown.utf8))
        )

        let duplicate = Self.snapshotJSON.replacingOccurrences(
            of: "\"view_version\":\"view-1\",",
            with: "\"view_version\":\"view-1\",\"view_version\":\"other\","
        )
        XCTAssertThrowsError(
            try LocalProductWire.decodeSnapshot(Data(duplicate.utf8))
        )
    }

    func testStrictTimelineDecodingRejectsNestedUnknownKeys() {
        let invalid = """
        {
          "schema_version":1,
          "team_instance_id":"team-1",
          "view_version":"view-1",
          "next_cursor":"",
          "has_more":false,
          "gap":null,
          "records":[],
          "board":{
            "schema_version":1,
            "team_instance_id":"team-1",
            "plan_digest":"",
            "status":"ready",
            "view_version":"view-1",
            "nodes":[],
            "cost":{"observed":false,"amount_microunits":null,"currency":""},
            "unexpected":true
          },
          "attention":[]
        }
        """
        XCTAssertThrowsError(
            try LocalProductWire.decodeTimeline(Data(invalid.utf8))
        )
    }

    static let snapshotJSON = """
    {
      "schema_version":1,
      "view_version":"view-1",
      "partial":false,
      "stale":false,
      "reason":"",
      "runtimes":[{
        "runtime_instance_id":"pi-local",
        "display_name":"Pi",
        "adapter_type":"pi",
        "executable_version":"0.82.1",
        "status":"online",
        "capacity":2,
        "model_ids":["qwen"],
        "observed_capabilities":["streaming"]
      }],
      "teams":[],
      "runs":[],
      "evidence":[],
      "attention":[],
      "runtime_page":{"next_cursor":"pi-local","has_more":false},
      "team_page":{"next_cursor":"","has_more":false},
      "run_page":{"next_cursor":"","has_more":false},
      "evidence_page":{"next_cursor":"","has_more":false}
    }
    """

    static let snapshotV2JSON = """
    {
      "schema_version":2,
      "view_version":"view-3",
      "partial":false,
      "stale":false,
      "reason":"",
      "health":{"daemon":"serving_request","journal":"available","projection":"current"},
      "runtimes":[],
      "teams":[],
      "missions":[],
      "runs":[],
      "evidence":[],
      "attention":[],
      "prepared_decisions":[],
      "runtime_page":{"next_cursor":"","has_more":false},
      "team_page":{"next_cursor":"","has_more":false},
      "mission_page":{"next_cursor":"","has_more":false},
      "run_page":{"next_cursor":"","has_more":false},
      "evidence_page":{"next_cursor":"","has_more":false}
    }
    """
}
