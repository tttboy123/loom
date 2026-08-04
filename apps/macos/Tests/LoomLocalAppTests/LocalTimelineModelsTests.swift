import XCTest
@testable import LoomLocalAppCore

final class LocalTimelineModelsTests: XCTestCase {
    func testIntegrationSnapshotDecodesExactClosedShape() throws {
        let json = """
        {
          "view_version": "sf3-view",
          "releases": [
            {
              "release_id": "release-1",
              "candidate_id": "candidate-1",
              "target_branch": "main",
              "base_commit": "base",
              "source_digest": "src",
              "evidence_digest": "ev",
              "dependency_digests": [],
              "status": "published",
              "adopted_by_run_id": null,
              "created_at": "2026-08-05T00:00:00Z",
              "correlation_id": "journey-1"
            }
          ],
          "canaries": [
            {
              "canary_id": "canary-1",
              "run_id": "run-1",
              "runtime_instance_id": "runtime.pi.earendil-works.0.82.1",
              "model_id": "model-1",
              "skill_digest": "skill",
              "status": "completed",
              "evidence_digest": null,
              "started_at": "2026-08-05T00:00:00Z",
              "finished_at": null,
              "correlation_id": "journey-1"
            }
          ],
          "timeline": [],
          "attention": []
        }
        """.data(using: .utf8)!
        let snapshot = try JSONDecoder().decode(LocalIntegrationSnapshot.self, from: json)
        XCTAssertEqual(snapshot.releases.count, 1)
        XCTAssertEqual(snapshot.releases[0].targetBranch, "main")
        XCTAssertEqual(snapshot.canaries[0].status, "completed")
    }

    func testIntegrationCommandReceiptDecodesClosedShape() throws {
        let json = """
        {
          "operation_id": "op-1",
          "action": "integrate",
          "event_ids": ["sf3-event"],
          "release_id": "release-1",
          "canary_id": null,
          "disposition": "integrated"
        }
        """.data(using: .utf8)!
        let receipt = try JSONDecoder().decode(LocalIntegrationCommandReceipt.self, from: json)
        XCTAssertEqual(receipt.disposition, "integrated")
        XCTAssertEqual(receipt.releaseID, "release-1")
    }
}
