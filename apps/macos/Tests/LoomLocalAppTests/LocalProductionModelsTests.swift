import XCTest
@testable import LoomLocalAppCore

final class LocalProductionModelsTests: XCTestCase {
    func testDecodeProductionSnapshot() throws {
        let json = """
        {
          "view_version": "v1",
          "activated": true,
          "activated_at": "2026-08-05T12:00:00Z",
          "target_mode": "default",
          "recovery": {
            "degraded": false,
            "last_activated": true,
            "config_digest_mismatch": false
          },
          "last_preview": "digest-1"
        }
        """.data(using: .utf8)!
        let snapshot = try ProductionWire.decodeSnapshot(json)
        XCTAssertTrue(snapshot.activated)
        XCTAssertEqual(snapshot.targetMode, "default")
        XCTAssertFalse(snapshot.recovery.degraded)
        XCTAssertTrue(snapshot.recovery.lastActivated)
        XCTAssertEqual(snapshot.lastPreview, "digest-1")
    }

    func testDecodeDegradedProductionSnapshot() throws {
        let json = """
        {
          "view_version": "v1",
          "activated": true,
          "recovery": {
            "degraded": true,
            "reason": "config mismatch",
            "last_activated": true,
            "config_digest_mismatch": true
          }
        }
        """.data(using: .utf8)!
        let snapshot = try ProductionWire.decodeSnapshot(json)
        XCTAssertTrue(snapshot.recovery.degraded)
        XCTAssertEqual(snapshot.recovery.reason, "config mismatch")
    }
}
