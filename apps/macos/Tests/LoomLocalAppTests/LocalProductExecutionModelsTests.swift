import Foundation
import XCTest

@testable import LoomLocalAppCore

final class LocalProductExecutionModelsTests: XCTestCase {
  func testStrictPreflightEnvelopeBindsExactProductIdentity() throws {
    let envelope = try LocalProductExecutionWire.decodeEnvelope(
      Data(Self.preflightEnvelope.utf8)
    )

    XCTAssertEqual(envelope.operation, "preflight")
    XCTAssertNil(envelope.result)
    XCTAssertEqual(envelope.preflight?.workPackageID, "work-package.coding")
    XCTAssertEqual(envelope.preflight?.nodes.map(\.logicalNodeID), ["main"])
    XCTAssertEqual(envelope.preflight?.sideEffects, [])
    XCTAssertEqual(envelope.preflight?.nodes.first?.harnessAdapter, "codex")
    XCTAssertEqual(envelope.preflight?.nodes.first?.providerAccountID, "openai.primary")
    XCTAssertEqual(envelope.preflight?.nodes.first?.authMode, "brokered")
    XCTAssertEqual(envelope.preflight?.nodes.first?.credentialRevision, 3)
    XCTAssertEqual(envelope.preflight?.nodes.first?.status, "ready")
	XCTAssertTrue(envelope.preflight?.nodes.first?.fallbackConfigured == true)
	XCTAssertEqual(
		envelope.preflight?.nodes.first?.fallbackProviderAccountID,
		"anthropic.backup"
	)
	XCTAssertEqual(envelope.preflight?.nodes.first?.fallbackStatus, "ready")
	XCTAssertEqual(envelope.preflight?.nodes.first?.fallbackAuthMode, "brokered")
	XCTAssertEqual(envelope.preflight?.nodes.first?.fallbackReasoningEffort, "low")
	XCTAssertEqual(envelope.preflight?.nodes.first?.fallbackTimeoutSeconds, 90)
	XCTAssertEqual(envelope.preflight?.nodes.first?.fallbackBudgetCredits, 20)
	XCTAssertTrue(
		envelope.preflight?.nodes.first?.fallbackApprovalRequired == true
	)
	XCTAssertTrue(
		envelope.preflight?.nodes.first?.fallbackApprovalAvailable == true
	)
	XCTAssertEqual(envelope.preflight?.nodes.first?.fallbackApprovalVersion, 1)

    let unknown = Self.preflightEnvelope.replacingOccurrences(
      of: "\"operation\":\"preflight\",",
      with: "\"operation\":\"preflight\",\"unknown\":true,"
    )
    XCTAssertThrowsError(
      try LocalProductExecutionWire.decodeEnvelope(Data(unknown.utf8))
    )
	let unapprovedFallback = Self.preflightEnvelope.replacingOccurrences(
		of: "\"fallback_approval_required\":true",
		with: "\"fallback_approval_required\":false"
	)
	XCTAssertThrowsError(
		try LocalProductExecutionWire.decodeEnvelope(Data(unapprovedFallback.utf8))
	)
	for malformed in [
		Self.preflightEnvelope.replacingOccurrences(
			of: "\"provider_account_id\":\"openai.primary\"",
			with: "\"provider_account_id\":\"\""
		),
		Self.preflightEnvelope.replacingOccurrences(
			of: "\"auth_mode\":\"brokered\"",
			with: "\"auth_mode\":\"native_auth\""
		),
		Self.preflightEnvelope.replacingOccurrences(
			of: "\"fallback_credential_revision\":5",
			with: "\"fallback_credential_revision\":0"
		),
		Self.preflightEnvelope.replacingOccurrences(
			of: "\"fallback_timeout_seconds\":90",
			with: "\"fallback_timeout_seconds\":0"
		),
	] {
		XCTAssertThrowsError(
			try LocalProductExecutionWire.decodeEnvelope(Data(malformed.utf8))
		)
	}
	let nativeFallback = Self.preflightEnvelope
		.replacingOccurrences(
			of: "\"fallback_auth_mode\":\"brokered\"",
			with: "\"fallback_auth_mode\":\"native_auth\""
		)
		.replacingOccurrences(
			of: "\"fallback_provider_account_id\":\"anthropic.backup\"",
			with: "\"fallback_provider_account_id\":\"\""
		)
		.replacingOccurrences(
			of: "\"fallback_credential_revision\":5",
			with: "\"fallback_credential_revision\":0"
		)
	XCTAssertNoThrow(
		try LocalProductExecutionWire.decodeEnvelope(Data(nativeFallback.utf8))
	)
  }

  func testPreflightNodeDecodesRemoteToolEnrollmentBinding() throws {
    let withEnrollment = Self.preflightEnvelope.replacingOccurrences(
      of: "\"fallback_approval_version\":1",
      with: "\"fallback_approval_version\":1,\n"
        + "\"remote_tool_enrollment_available\":true,\n"
        + "\"remote_tool_enrollment_id\":\"enr-search\",\n"
        + "\"remote_tool_backend_kind\":\"web_search\",\n"
        + "\"remote_tool_binding_digest\":\"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee\""
    )
    let envelope = try LocalProductExecutionWire.decodeEnvelope(
      Data(withEnrollment.utf8)
    )
    let node = try XCTUnwrap(envelope.preflight?.nodes.first)
    XCTAssertTrue(node.remoteToolEnrollmentAvailable)
    XCTAssertEqual(node.remoteToolEnrollmentID, "enr-search")
    XCTAssertEqual(node.remoteToolBackendKind, "web_search")
    XCTAssertEqual(
      node.remoteToolBindingDigest,
      String(repeating: "e", count: 64)
    )
    let digest64 = String(repeating: "e", count: 64)
    let unknown = withEnrollment.replacingOccurrences(
      of: "\"remote_tool_binding_digest\":\"\(digest64)\"",
      with: "\"remote_tool_binding_digest\":\"\(digest64)\",\"raw_endpoint\":\"https://x\""
    )
    XCTAssertThrowsError(
      try LocalProductExecutionWire.decodeEnvelope(Data(unknown.utf8))
    )
  }

  func testMissionExecutionCommandEncodesClosedPreflightShape() throws {
    let command = LocalProductExecutionCommand.preflight(
      missionID: "mission/team-1",
      teamInstanceID: "team-1",
      workPackageID: "work-package.coding",
      workPackageDigest: String(repeating: "a", count: 64),
      objective: "Implement bounded change",
      confirmedConstraints: ["Do not change public APIs"],
      acceptedDecisions: ["Use the existing execution adapter"],
      expectedViewVersion: String(repeating: "b", count: 64),
      correlationID: "11111111-1111-4111-8111-111111111111"
    )
    let object = try XCTUnwrap(
      JSONSerialization.jsonObject(with: JSONEncoder().encode(command))
        as? [String: Any]
    )

    XCTAssertEqual(object["schema_version"] as? Int, 1)
    XCTAssertEqual(object["operation"] as? String, "preflight")
    XCTAssertEqual(object["context_version"] as? Int, 1)
    XCTAssertEqual(
      object["confirmed_constraints"] as? [String],
      ["Do not change public APIs"]
    )
    XCTAssertEqual(
      object["accepted_decisions"] as? [String],
      ["Use the existing execution adapter"]
    )
    XCTAssertNil(object["control_action"])
    XCTAssertNil(object["preflight_digest"])
  }

  func testMissionExecutionControlOmitsMissionContext() throws {
    let command = LocalProductExecutionCommand.control(
      missionID: "mission/team-1",
      teamInstanceID: "team-1",
      expectedViewVersion: String(repeating: "b", count: 64),
      action: "cancel",
      executionDigest: String(repeating: "c", count: 64),
      logicalNodeID: "main",
      attemptNumber: 1,
      claimGeneration: 1,
      correlationID: "11111111-1111-4111-8111-111111111111"
    )
    let object = try XCTUnwrap(
      JSONSerialization.jsonObject(with: JSONEncoder().encode(command))
        as? [String: Any]
    )
    XCTAssertNil(object["context_version"])
    XCTAssertNil(object["confirmed_constraints"])
    XCTAssertNil(object["accepted_decisions"])
  }

  func testMissionExecutionResultAcceptsOnlyProjectedClosedStatuses() throws {
    let valid = """
      {"schema_version":1,"operation":"start","result":{
        "schema_version":1,"mission_id":"mission/team-1","team_instance_id":"team-1",
        "status":"awaiting_recovery",
        "view_version":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
        "execution_digest":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
      }}
      """
    XCTAssertEqual(
      try LocalProductExecutionWire.decodeEnvelope(Data(valid.utf8)).result?.status,
      "awaiting_recovery"
    )

    let unknown = valid.replacingOccurrences(
      of: "\"awaiting_recovery\"",
      with: "\"guessed_status\""
    )
    XCTAssertThrowsError(
      try LocalProductExecutionWire.decodeEnvelope(Data(unknown.utf8))
    )
  }

  static let preflightEnvelope = """
    {
      "schema_version":1,
      "operation":"preflight",
      "preflight":{
        "schema_version":1,
        "mission_id":"mission/team-1",
        "team_instance_id":"team-1",
        "work_package_id":"work-package.coding",
        "work_package_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
        "view_version":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
        "plan_digest":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
        "preflight_digest":"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
        "expires_at":"2026-08-01T12:05:00Z",
        "runtime_instance_id":"runtime-pi",
        "runtime_profile_id":"pi-default",
        "model_id":"qwen",
        "auth_mode":"brokered",
        "capacity_available":1,
        "budget_status":"unavailable",
        "side_effects":[],
        "permission_scopes":["workspace"],
        "approval_points":["before_start"],
        "nodes":[{
          "logical_node_id":"main",
          "title":"Implement bounded change",
          "role":"main",
          "depends_on":[],
          "max_attempts":2,
          "harness_adapter":"codex",
          "provider_id":"openai",
          "provider_account_id":"openai.primary",
          "auth_mode":"brokered",
          "model_id":"gpt-5.5-codex",
          "credential_revision":3,
          "reasoning_effort":"high",
          "timeout_seconds":120,
          "budget_credits":40,
          "capabilities":["reasoning_effort","tools"],
          "status":"ready",
          "block_reason":"",
          "fallback_configured":true,
          "fallback_harness_adapter":"claude-code",
          "fallback_provider_id":"anthropic",
          "fallback_provider_account_id":"anthropic.backup",
          "fallback_auth_mode":"brokered",
          "fallback_model_id":"claude-sonnet",
          "fallback_credential_revision":5,
          "fallback_reasoning_effort":"low",
          "fallback_timeout_seconds":90,
          "fallback_budget_credits":20,
          "fallback_capabilities":["chat","reasoning_effort"],
          "fallback_status":"ready",
          "fallback_block_reason":"",
          "fallback_approval_required":true,
          "fallback_approval_available":true,
          "fallback_approval_version":1
        }]
      }
    }
    """
}
