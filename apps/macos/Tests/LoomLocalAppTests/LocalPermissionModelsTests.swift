import XCTest
@testable import LoomLocalAppCore

final class LocalPermissionModelsTests: XCTestCase {
    func testDecodePermissionSnapshot() throws {
        let json = """
        {
          "view_version": "v1",
          "profiles": [
            {
              "profile_id": "profile-a",
              "generation": 1,
              "digest": "d1",
              "mode": "default",
              "rules": [
                {"rule_id": "r-1", "scope": "project", "scope_id": "p1",
                 "action": "allow", "tool": "Bash", "pattern": "go test *"}
              ],
              "owned_paths": ["src/**"]
            }
          ],
          "bindings": [
            {"job_id": "job-a", "profile_id": "profile-a", "profile_digest": "d1",
             "profile_generation": 1, "bound_at": "2026-08-05T12:00:00Z"}
          ],
          "rules": [
            {"rule_id": "r-1", "scope": "project", "scope_id": "p1",
             "action": "allow", "tool": "Bash", "pattern": "go test *"}
          ],
          "grants": [],
          "activations": [],
          "admin_lock": true
        }
        """.data(using: .utf8)!
        let snapshot = try PermissionWire.decodeSnapshot(json)
        XCTAssertEqual(snapshot.viewVersion, "v1")
        XCTAssertEqual(snapshot.profiles.count, 1)
        XCTAssertEqual(snapshot.profiles[0].profileID, "profile-a")
        XCTAssertEqual(snapshot.profiles[0].rules[0].pattern, "go test *")
        XCTAssertEqual(snapshot.bindings[0].jobID, "job-a")
        XCTAssertEqual(snapshot.bindings[0].profileGeneration, 1)
        XCTAssertTrue(snapshot.adminLock)
    }

    func testDecodePermissionAttention() throws {
        let json = """
        {
          "view_version": "v1",
          "decisions": [
            {
              "job_id": "job-a",
              "approval_id": "approval-1",
              "verdict": "ask",
              "reason": "needs approval",
              "authorization_path": "resolve in Attention Inbox",
              "recorded_at": "2026-08-05T12:00:00Z"
            }
          ],
          "approvals": [
            {
              "approval_id": "approval-2",
              "digest": "digest-2",
              "job_id": "job-a",
              "status": "pending",
              "command": "curl https://example.com"
            }
          ]
        }
        """.data(using: .utf8)!
        let attention = try PermissionWire.decodeAttention(json)
        XCTAssertEqual(attention.decisions.count, 1)
        XCTAssertEqual(attention.decisions[0].jobID, "job-a")
        XCTAssertEqual(attention.decisions[0].verdict, "ask")
        XCTAssertEqual(attention.approvals.count, 1)
        XCTAssertEqual(attention.approvals[0].approvalID, "approval-2")
        XCTAssertEqual(attention.approvals[0].status, "pending")
        XCTAssertEqual(attention.approvals[0].command, "curl https://example.com")
    }

}
