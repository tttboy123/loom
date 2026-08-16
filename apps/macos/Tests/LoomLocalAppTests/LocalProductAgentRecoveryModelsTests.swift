import Foundation
import XCTest

@testable import LoomLocalAppCore

final class LocalProductAgentRecoveryModelsTests: XCTestCase {
  func testDecodesContentFreePreviewAndRejectsUnknownFields() throws {
    let data = Data(
      #"{"schema_version":1,"incident_id":"loom-recovery-1","operation":"preview","candidates":[{"schema_version":1,"status":"available","action":"resume_pre_model","candidate_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","capability_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","attempt_id":"attempt-1","team_instance_id":"team-1","segment_id":"segment-1","work_item_id":"work-1","run_id":"run-1","claim_generation":3,"runtime_instance_id":"runtime-1","agent_instance_id":"agent-1","harness_adapter":"loom-native","provider_id":"deepseek","provider_account_id":"deepseek.primary","model_id":"deepseek-chat","credential_revision":4}]}"#.utf8
    )
    let response = try LocalProductAgentRecoveryWire.decodeResponse(data)
    XCTAssertEqual(response.operation, .preview)
    XCTAssertEqual(response.incidentID, "loom-recovery-1")
    XCTAssertEqual(response.candidates.count, 1)
    XCTAssertEqual(response.candidates[0].providerAccountID, "deepseek.primary")
    XCTAssertEqual(response.candidates[0].credentialRevision, 4)
    XCTAssertNil(response.decision)
    XCTAssertNil(response.resume)

    let unknown = Data(
      #"{"schema_version":1,"incident_id":"loom-recovery-1","operation":"preview","candidates":[],"prompt":"forbidden"}"#.utf8
    )
    XCTAssertThrowsError(try LocalProductAgentRecoveryWire.decodeResponse(unknown))
  }

  func testRequestsFreezeThreeDistinctOperations() {
    let digestA = String(repeating: "a", count: 64)
    let digestB = String(repeating: "b", count: 64)
    XCTAssertTrue(LocalProductAgentRecoveryRequest.preview.isValid)
    XCTAssertTrue(
      LocalProductAgentRecoveryRequest.confirm(
        decisionID: "11111111-1111-4111-8111-111111111111",
        principalID: "local-user", candidateDigest: digestA,
        capabilityDigest: digestB
      ).isValid
    )
    XCTAssertTrue(
      LocalProductAgentRecoveryRequest.resume(
        decisionID: "11111111-1111-4111-8111-111111111111",
        candidateDigest: digestA, capabilityDigest: digestB
      ).isValid
    )
    XCTAssertFalse(
      LocalProductAgentRecoveryRequest(
        operation: .preview, decisionID: "unexpected", principalID: "",
        candidateDigest: "", capabilityDigest: ""
      ).isValid
    )
  }

  func testDecisionAndResumeRejectUnknownOrMalformedFields() throws {
    let decision = Data(
      #"{"schema_version":1,"incident_id":"loom-recovery-1","operation":"confirm","decision":{"schema_version":1,"decision_id":"11111111-1111-4111-8111-111111111111","action":"resume_pre_model","candidate_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","capability_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}}"#.utf8
    )
    XCTAssertEqual(
      try LocalProductAgentRecoveryWire.decodeResponse(decision).decision?.action,
      "resume_pre_model"
    )

    let unknownDecisionField = Data(
      #"{"schema_version":1,"incident_id":"loom-recovery-1","operation":"confirm","decision":{"schema_version":1,"decision_id":"11111111-1111-4111-8111-111111111111","action":"resume_pre_model","candidate_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","capability_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","prompt":"forbidden"}}"#.utf8
    )
    XCTAssertThrowsError(
      try LocalProductAgentRecoveryWire.decodeResponse(unknownDecisionField)
    )

    let malformedResume = Data(
      #"{"schema_version":1,"incident_id":"loom-recovery-2","operation":"resume","resume":{"schema_version":1,"status":"completed","decision_id":"11111111-1111-4111-8111-111111111111","candidate_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","capability_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","attempt_id":"","runtime_instance_id":"runtime-1"}}"#.utf8
    )
    XCTAssertThrowsError(try LocalProductAgentRecoveryWire.decodeResponse(malformedResume))
  }
}
