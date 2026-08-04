import XCTest
import Foundation

@testable import LoomLocalAppCore

final class LocalQueueModelsTests: XCTestCase {
    func testQueueSnapshotDecodesProductionWireShape() throws {
        let wire = """
        {
          "view_version": "v1",
          "next_cursor": "",
          "jobs": [{
            "job_id": "job-a",
            "source": "user_queued",
            "dag_node_id": "node-a",
            "dependencies": [],
            "status": "admitted",
            "lane": "development",
            "owned_paths": ["internal/queue/model.go"],
            "mutex_keys": [],
            "resource_claims": {"runtime": "pi", "slots": 1, "model": "m"},
            "attempt_count": 0,
            "max_attempts": 3,
            "capability_kind": "feature",
            "exit_conditions": ["focused tests pass"],
            "verification_strategy": "focused + race",
            "integration_strategy": "single-integrator",
            "protected_authority_paths": [],
            "created_at": "2026-08-04T20:00:00Z",
            "correlation_id": "123e4567-e89b-42d3-a456-426614174000"
          }],
          "gaps": [],
          "successors": []
        }
        """.data(using: .utf8)!
        let snapshot = try JSONDecoder().decode(QueueSnapshot.self, from: wire)
        XCTAssertEqual(snapshot.jobs.count, 1)
        XCTAssertEqual(snapshot.jobs[0].jobID, "job-a")
        XCTAssertEqual(snapshot.jobs[0].status, "admitted")
        XCTAssertEqual(snapshot.jobs[0].ownedPaths, ["internal/queue/model.go"])
        XCTAssertEqual(snapshot.jobs[0].resourceClaims.slots, 1)
    }

    func testQueueCommandReceiptDecodesProductionWireShape() throws {
        let wire = """
        {
          "operation_id": "op-1",
          "action": "create_job",
          "view_version": "v1",
          "event_ids": ["sf1-abc"],
          "job_id": "job-a",
          "status": "admitted"
        }
        """.data(using: .utf8)!
        let receipt = try JSONDecoder().decode(QueueCommandReceipt.self, from: wire)
        XCTAssertEqual(receipt.operationID, "op-1")
        XCTAssertEqual(receipt.jobID, "job-a")
        XCTAssertEqual(receipt.status, "admitted")
    }

    func testQueueJobSubmissionEncodesExactWireKeys() throws {
        let submission = QueueJobSubmission(
            jobID: "job-a",
            source: "user_queued",
            dagNodeID: "node-a",
            dependencies: [],
            ownedPaths: ["internal/queue/model.go"],
            mutexKeys: [],
            resourceClaims: QueueResourceClaims(runtime: "pi", slots: 1, model: "m"),
            maxAttempts: 3,
            capabilityKind: "feature",
            exitConditions: ["focused tests pass"],
            verificationStrategy: "focused + race",
            integrationStrategy: "single-integrator",
            protectedAuthorityPaths: [],
            eligibilityAuthority: "user"
        )
        let data = try JSONEncoder().encode(submission)
        let object = try JSONSerialization.jsonObject(with: data) as! [String: Any]
        XCTAssertEqual(object["job_id"] as? String, "job-a")
        XCTAssertEqual(object["eligibility_authority"] as? String, "user")
        XCTAssertEqual((object["owned_paths"] as? [String])?.first, "internal/queue/model.go")
    }
}
