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
}
