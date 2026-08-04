import XCTest
@testable import LoomLocalAppCore

final class LocalWorkerModelsTests: XCTestCase {
    func testWorkersSnapshotDecodesExactClosedShape() throws {
        let json = """
        {
          "view_version": "sf2-view",
          "attempts": [
            {
              "attempt_id": "attempt-1",
              "job_id": "job-a",
              "generation": 1,
              "lease_expires_at": "2026-08-05T00:01:00Z",
              "claim_cas": "op-1",
              "status": "claimed",
              "crash_seam": "",
              "crash_effect_cardinality": "",
              "failure_class": "",
              "evidence_digests": [],
              "candidate_branch": "codex/candidate-a",
              "candidate_worktree": "/tmp/candidate-a",
              "started_at": "2026-08-05T00:00:00Z",
              "finished_at": "",
              "lane": "development"
            }
          ],
          "active_workers": [
            {"worker_id": "w1", "lane": "development", "attempt_id": "attempt-1", "job_id": "job-a", "generation": 1}
          ],
          "repair_wait_age": 0,
          "lanes": {"development": 1}
        }
        """.data(using: .utf8)!
        let snapshot = try JSONDecoder().decode(LocalWorkersSnapshot.self, from: json)
        XCTAssertEqual(snapshot.attempts.count, 1)
        XCTAssertEqual(snapshot.attempts[0].generation, 1)
        XCTAssertEqual(snapshot.activeWorkers[0].workerID, "w1")
        XCTAssertEqual(snapshot.lanes["development"], 1)
    }

    func testWorkersCommandReceiptDecodesClosedShape() throws {
        let json = """
        {
          "operation_id": "op-1",
          "action": "claim",
          "view_version": "sf2-view",
          "event_ids": ["sf2-event"],
          "attempt_id": "attempt-1",
          "job_id": "job-a",
          "generation": 1,
          "lane": "development",
          "disposition": "claimed"
        }
        """.data(using: .utf8)!
        let receipt = try JSONDecoder().decode(LocalWorkersCommandReceipt.self, from: json)
        XCTAssertEqual(receipt.action, "claim")
        XCTAssertEqual(receipt.generation, 1)
        XCTAssertEqual(receipt.disposition, "claimed")
    }
}
