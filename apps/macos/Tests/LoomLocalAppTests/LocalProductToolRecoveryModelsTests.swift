import Foundation
import XCTest

@testable import LoomLocalAppCore

final class LocalProductToolRecoveryModelsTests: XCTestCase {
  func testDecodesStrictContentFreePreview() throws {
    let data = Data(
      #"{"schema_version":1,"incident_id":"loom-tool-recovery-1","operation":"preview","candidates":[{"schema_version":1,"status":"available","candidate_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","execution_id":"execution-1","job_id":"work-1","call_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","tool":"bash","generation":3,"operation_id":"operation-1","incident_id":"incident-original","recovery_code":"side_effect_unknown","recovery_required_at":"2026-08-14T20:00:00Z","available_actions":["abort_attempt"]}]}"#.utf8
    )
    let response = try LocalProductToolRecoveryWire.decodeResponse(data)
    XCTAssertEqual(response.operation, .preview)
    XCTAssertEqual(response.candidates.count, 1)
    XCTAssertEqual(response.candidates[0].status, .available)
    XCTAssertEqual(response.candidates[0].availableActions, [.abortAttempt])
    XCTAssertNil(response.decision)

    let unknown = Data(
      #"{"schema_version":1,"incident_id":"loom-tool-recovery-1","operation":"preview","candidates":[],"prompt":"forbidden"}"#.utf8
    )
    XCTAssertThrowsError(try LocalProductToolRecoveryWire.decodeResponse(unknown))
  }

  func testRequestsAndActionSpecificDecisionsAreStrict() throws {
    let digest = String(repeating: "a", count: 64)
    XCTAssertTrue(LocalProductToolRecoveryRequest.preview.isValid)
    XCTAssertTrue(
      LocalProductToolRecoveryRequest.resolve(
        decisionID: "11111111-1111-4111-8111-111111111111",
        principalID: "local-user", action: .abortAttempt,
        candidateDigest: digest
      ).isValid
    )

    let resolved = Data(
      #"{"schema_version":1,"incident_id":"loom-tool-recovery-2","operation":"resolve","decision":{"schema_version":1,"decision_id":"11111111-1111-4111-8111-111111111111","action":"retry_in_new_attempt","candidate_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","execution_id":"execution-1","replacement_attempt_id":"attempt-2","replacement_run_id":"run-2","execution_binding_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","context_capsule_digest":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}}"#.utf8
    )
    let decision = try XCTUnwrap(
      LocalProductToolRecoveryWire.decodeResponse(resolved).decision
    )
    XCTAssertEqual(decision.action, .retryInNewAttempt)
    XCTAssertEqual(decision.replacementAttemptID, "attempt-2")

    let mixed = Data(
      #"{"schema_version":1,"incident_id":"loom-tool-recovery-3","operation":"resolve","decision":{"schema_version":1,"decision_id":"11111111-1111-4111-8111-111111111111","action":"abort_attempt","candidate_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","execution_id":"execution-1","evidence_id":"not-allowed"}}"#.utf8
    )
    XCTAssertThrowsError(try LocalProductToolRecoveryWire.decodeResponse(mixed))
  }
}
