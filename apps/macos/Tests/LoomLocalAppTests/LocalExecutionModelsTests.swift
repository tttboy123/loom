import XCTest
@testable import LoomLocalAppCore

final class LocalExecutionModelsTests: XCTestCase {
    func testDecodeExecutionSnapshot() throws {
        let json = """
        {
          "view_version": "v1",
          "records": [
            {
              "execution_id": "exec-1",
              "job_id": "job-a",
              "call_digest": "digest-1",
              "tool": "Bash",
              "command": "printf hi",
              "path": "",
              "generation": 1,
              "operation_id": "op-1",
              "journey_id": "11111111-1111-4111-8111-111111111111",
              "proposed_at": "2026-08-05T12:00:00Z",
              "allowed_at": "2026-08-05T12:00:01Z",
              "exit_code": 0,
              "output_digest": "out",
              "changed_files_digest": "changed",
              "evidence_id": "ev-1",
              "duration_ms": 5,
              "status": "completed",
              "completed_at": "2026-08-05T12:00:02Z"
            }
          ]
        }
        """.data(using: .utf8)!
        let snapshot = try ExecutionWire.decodeSnapshot(json)
        XCTAssertEqual(snapshot.viewVersion, "v1")
        XCTAssertEqual(snapshot.records.count, 1)
        XCTAssertEqual(snapshot.records[0].executionID, "exec-1")
        XCTAssertEqual(snapshot.records[0].tool, "Bash")
        XCTAssertEqual(snapshot.records[0].status, "completed")
        XCTAssertEqual(snapshot.records[0].exitCode, 0)
    }

    func testDecodeRecoveryRequiredExecution() throws {
        let json = """
        {
          "view_version": "v-recovery",
          "records": [{
            "execution_id": "exec-recovery",
            "job_id": "job-recovery",
            "call_digest": "digest-recovery",
            "tool": "Bash",
            "generation": 2,
            "operation_id": "op-recovery",
            "journey_id": "incident-recovery-1",
            "proposed_at": "2026-08-14T11:00:00Z",
            "allowed_at": "2026-08-14T11:00:01Z",
            "status": "recovery_required",
            "recovery_required_at": "2026-08-14T11:00:02Z",
            "recovery_code": "side_effect_unknown",
            "recovery_action": "resolve_tool_recovery"
          }]
        }
        """.data(using: .utf8)!
        let record = try XCTUnwrap(ExecutionWire.decodeSnapshot(json).records.first)
        XCTAssertEqual(record.status, "recovery_required")
        XCTAssertEqual(record.recoveryCode, "side_effect_unknown")
        XCTAssertEqual(record.recoveryAction, "resolve_tool_recovery")
        XCTAssertEqual(record.journeyID, "incident-recovery-1")
        XCTAssertNil(record.command)
        XCTAssertNil(record.path)
    }

    func testDecodeResolvedToolRecoveryMetadata() throws {
        let decisionID = "44444444-4444-4444-8444-444444444444"
        let observationDigest = String(repeating: "d", count: 64)
        let snapshot = try ExecutionWire.decodeSnapshot(Data(
            """
            {"view_version":"view-recovery-resolved","records":[{"execution_id":"exec-1","job_id":"job-1","call_digest":"\(String(repeating: "a", count: 64))","tool":"write_file","generation":2,"operation_id":"operation-1","journey_id":"incident-1","proposed_at":"2026-08-14T10:00:00Z","status":"recovery_effect_accepted","recovery_required_at":"2026-08-14T10:01:00Z","recovery_code":"side_effect_unknown","recovery_action":"resolve_tool_recovery","recovery_decision_id":"\(decisionID)","recovery_decision":"accept_observed_effect","recovery_resolved_at":"2026-08-14T10:02:00Z","recovery_evidence_id":"evidence-1","recovery_observation_digest":"\(observationDigest)"}]}
            """.utf8
        ))

        let record = try XCTUnwrap(snapshot.records.first)
        XCTAssertEqual(record.status, "recovery_effect_accepted")
        XCTAssertEqual(record.recoveryDecisionID, decisionID)
        XCTAssertEqual(record.recoveryDecision, "accept_observed_effect")
        XCTAssertEqual(record.recoveryEvidenceID, "evidence-1")
        XCTAssertEqual(record.recoveryObservationDigest, observationDigest)
    }
}
