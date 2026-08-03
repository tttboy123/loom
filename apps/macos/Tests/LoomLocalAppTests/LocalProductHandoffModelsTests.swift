import Foundation
import XCTest
@testable import LoomLocalAppCore

final class LocalProductHandoffModelsTests: XCTestCase {
    func testSchemaThreeSideTaskIsStrictAndCollectionsRejectNull() throws {
        let digest = String(repeating: "a", count: 64)
        let item = """
        {"side_task_id":"side-1","parent_mission_id":"mission/team-1","parent_team_instance_id":"team-1","parent_task_id":"work-1","parent_run_id":"run-1","parent_claim_generation":1,"parent_execution_digest":"\(digest)","side_execution_team_instance_id":"team-side-1","purpose":"research","mode":"report_only","title":"Research","status":"report_delivered","source_generation":1,"handoff_version":1,"handoff_digest":"\(digest)","summary_artifact_digest":"\(digest)","what_happened":"Authorized result","authorized_findings":[],"evidence_references":[],"artifact_references":[],"risk":"low","uncertainties":[],"scope_delta":[],"decision_options":[],"recommended_option":"","recommendation_authority":"proposal_only","usage_observed":false,"usage_microunits":0,"usage_currency":"","decision_deadline":"","available_decisions":[],"effect_status":"none"}
        """
        let base = """
        {"schema_version":3,"view_version":"\(digest)","partial":false,"stale":false,"reason":"","health":{"daemon":"serving_request","journal":"available","projection":"current"},"runtimes":[],"teams":[],"missions":[],"runs":[],"evidence":[],"attention":[],"prepared_decisions":[],"side_tasks":[\(item)],"runtime_page":{"next_cursor":"","has_more":false},"team_page":{"next_cursor":"","has_more":false},"mission_page":{"next_cursor":"","has_more":false},"run_page":{"next_cursor":"","has_more":false},"evidence_page":{"next_cursor":"","has_more":false}}
        """
        let decoded = try LocalProductWire.decodeSnapshot(Data(base.utf8))
        XCTAssertEqual(decoded.sideTasks.first?.sideTaskID, "side-1")
        XCTAssertThrowsError(try LocalProductWire.decodeSnapshot(Data(base.replacingOccurrences(of: "\"authorized_findings\":[]", with: "\"authorized_findings\":null").utf8)))
        XCTAssertThrowsError(try LocalProductWire.decodeSnapshot(Data(base.replacingOccurrences(of: "\"effect_status\":\"none\"", with: "\"effect_status\":\"none\",\"unknown\":true").utf8)))
    }

    func testStrictProposalAndDecisionResultsRejectUnknownAndNullCollections() throws {
        let digest = String(repeating: "a", count: 64)
        let proposal = """
        {"schema_version":1,"status":"proposal","proposal_digest":"\(digest)","view_version":"\(digest)","purpose":"research","mode":"report_only","title":"Research","permission_scopes":[],"decision_timeout_seconds":0,"requires_confirmation":true,"policy_available":false}
        """
        let decoded = try LocalProductHandoffWire.decodeProposal(Data(proposal.utf8))
        XCTAssertEqual(decoded.proposalDigest, digest)
        XCTAssertThrowsError(
            try LocalProductHandoffWire.decodeProposal(
                Data(proposal.replacingOccurrences(of: "\"permission_scopes\":[]", with: "\"permission_scopes\":null").utf8)
            )
        )
        XCTAssertThrowsError(
            try LocalProductHandoffWire.decodeProposal(
                Data(proposal.replacingOccurrences(of: "\"policy_available\":false", with: "\"policy_available\":false,\"unknown\":true").utf8)
            )
        )

        let decision = """
        {"schema_version":1,"side_task_id":"side-1","decision":"discard","status":"decided","effect_status":"none","context_packet_digest":"","continuation_execution_team_instance_id":"","view_version":"\(digest)"}
        """
        XCTAssertEqual(
            try LocalProductHandoffWire.decodeDecision(Data(decision.utf8)).decision,
            "discard"
        )
    }

    func testCreateRequestEncodesEveryRequiredFieldWithoutNull() throws {
        let digest = String(repeating: "a", count: 64)
        let proposal = LocalProductSideTaskProposalRequest(
            parentMissionID: "mission/team-1", parentTeamInstanceID: "team-1",
            parentTaskID: "work-1", parentRunID: "run-1", parentClaimGeneration: 2,
            parentExecutionDigest: digest,
            purpose: "verification", mode: "decision_required", title: "Verify",
            authorizedRequest: "Return one bounded result.", permissionScopes: [],
            decisionTimeoutSeconds: 900, expectedViewVersion: digest,
            correlationID: "11111111-1111-4111-8111-111111111111"
        )
        let encoded = try JSONEncoder().encode(
            LocalProductSideTaskCreateRequest(
                proposal: proposal, proposalDigest: digest, confirmed: true
            )
        )
        try StrictJSONScanner.validate(encoded)
        let object = try XCTUnwrap(JSONSerialization.jsonObject(with: encoded) as? [String: Any])
        XCTAssertEqual(Set(object.keys), Set([
            "schema_version", "operation", "parent_mission_id", "parent_team_instance_id",
            "parent_task_id", "parent_run_id", "parent_claim_generation", "parent_execution_digest",
            "purpose", "mode",
            "title", "authorized_request", "permission_scopes", "decision_timeout_seconds",
            "expected_view_version", "correlation_id", "proposal_digest", "confirmed",
            "policy_stream_id", "policy_version", "policy_digest",
        ]))
        XCTAssertEqual(object["operation"] as? String, "create")
        XCTAssertEqual(object["confirmed"] as? Bool, true)
        XCTAssertFalse(String(data: encoded, encoding: .utf8)?.contains("null") ?? true)
    }
}
