import Foundation
import CryptoKit
import XCTest
@testable import LoomLocalAppCore

final class LocalProductAssetModelsTests: XCTestCase {
    func testSnapshotDecodesExactEmptyCollectionsAndJourneyIdentity() throws {
        let data = Data(#"{"view_version":"view-1","next_cursor":"","definitions":[],"revisions":[],"candidates":[],"evaluations":[],"bindings":[],"materializations":[],"binding_subjects":[],"promotion_sources":[]}"#.utf8)
        let snapshot = try EvolutionAssetWire.decodeSnapshot(data)
        XCTAssertEqual(snapshot.viewVersion, "view-1")
        XCTAssertEqual(snapshot.records, [])
    }

    func testSnapshotAndReceiptRejectUnknownOrMissingCollectionFields() {
        let unknown = Data(#"{"view_version":"view-1","next_cursor":"","definitions":[],"revisions":[],"candidates":[],"evaluations":[],"bindings":[],"materializations":[],"binding_subjects":[],"promotion_sources":[],"unknown":true}"#.utf8)
        XCTAssertThrowsError(try EvolutionAssetWire.decodeSnapshot(unknown))

        let missingEvents = Data(#"{"operation_id":"op","action":"create_skill","view_version":"view-2"}"#.utf8)
        XCTAssertThrowsError(try EvolutionAssetWire.decodeReceipt(missingEvents))
    }

    func testIPCResponseRequiresExactJourneyAndRejectsDualIdentityDrift() throws {
        let response = Data(#"{"version":1,"request_id":"request-1","journey_id":"123e4567-e89b-42d3-a456-426614174000","ok":true,"result":{},"error":null}"#.utf8)
        _ = try LocalIPCWire.decodeResponse(
            response,
            expectedRequestID: "request-1",
            expectedJourneyID: "123e4567-e89b-42d3-a456-426614174000"
        )
        XCTAssertThrowsError(
            try LocalIPCWire.decodeResponse(
                response,
                expectedRequestID: "request-1",
                expectedJourneyID: "223e4567-e89b-42d3-a456-426614174000"
            )
        )
    }

    func testPopulatedMaterializationAndBindingSubjectDecodeStrictly() throws {
        let digest = String(repeating: "a", count: 64)
        let root = String(repeating: "b", count: 64)
        let json = """
        {"view_version":"view-1","next_cursor":"","definitions":[],"revisions":[],"candidates":[],"evaluations":[],"bindings":[],"materializations":[{"schema_version":1,"team_execution_id":"team-1","logical_node_id":"main","run_id":"run-1","attempt_number":1,"generation":2,"runtime_instance_id":"runtime-1","runtime_identity_digest":"\(digest)","capability":"loom.skill-materialization.pi.v1","asset_revision_bindings":[{"asset_kind":"skill","definition_id":"skill-1","revision_id":"revision-1","sha256_digest":"\(digest)","source_scope":"local"}],"asset_revision_set_digest":"\(digest)","manifest_artifact_digest":"\(digest)","materialization_root_digest":"\(root)","journey_id":"123e4567-e89b-42d3-a456-426614174000","cleaned":true,"cleanup_result":"removed","last_event_id":"event-1"}],"binding_subjects":[{"subject_kind":"work_package","subject_id":"work-package.coding","subject_version":1,"subject_digest":"\(digest)","subject_scope":"builtin","subject_project_id":"","subject_generation_id":"","subject_identity_digest":"\(root)"}],"promotion_sources":[]}
        """
        let snapshot = try EvolutionAssetWire.decodeSnapshot(Data(json.utf8))
        XCTAssertEqual(snapshot.materializations.first?.cleanupResult, "removed")
        XCTAssertEqual(snapshot.materializations.first?.assetRevisionBindings.count, 1)
        XCTAssertEqual(snapshot.bindingSubjects.first?.subjectID, "work-package.coding")
    }

    func testEvaluationAndBindingCommandsEncodeClosedInputShapes() throws {
        let digest = String(repeating: "a", count: 64)
        let root = String(repeating: "b", count: 64)
        let subjectJSON = """
        {"view_version":"view-1","next_cursor":"","definitions":[],"revisions":[],"candidates":[],"evaluations":[],"bindings":[],"materializations":[],"binding_subjects":[{"subject_kind":"work_package","subject_id":"work-package.coding","subject_version":1,"subject_digest":"\(digest)","subject_scope":"builtin","subject_project_id":"","subject_generation_id":"","subject_identity_digest":"\(root)"}],"promotion_sources":[]}
        """
        let subject = try EvolutionAssetWire.decodeSnapshot(Data(subjectJSON.utf8)).bindingSubjects[0]
        let binding = EvolutionAssetExactBinding(
            assetKind: .skill, definitionID: "skill-1", revisionID: "revision-1",
            sha256Digest: digest, sourceScope: "local"
        )
        let bind = EvolutionAssetCommand(
            action: "set_binding", operationID: "operation-1",
            journeyID: "123e4567-e89b-42d3-a456-426614174000",
            expectedViewVersion: digest, subject: subject, bindings: [binding],
            assetRevisionSetDigest: root
        )
        let bindObject = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(bind)) as? [String: Any])
        XCTAssertEqual(Set(bindObject.keys), Set([
            "subject_kind", "subject_id", "subject_version", "subject_digest",
            "subject_scope", "subject_project_id", "subject_generation_id",
            "subject_identity_digest", "asset_revision_bindings", "asset_revision_set_digest",
        ]))
        let evaluation = EvolutionAssetCommand(
            action: "record_evaluation", operationID: "operation-2",
            journeyID: "123e4567-e89b-42d3-a456-426614174000",
            expectedViewVersion: digest, candidateID: "candidate-1",
            evaluationID: "evaluation-1", fixtureKind: "synthetic",
            fixtureDigest: root, baselineRevisionID: "revision-0", baselineDigest: digest,
            candidateRevisionID: "revision-1", candidateDigest: digest,
            requestedCaseIDs: ["artifact_digest"]
        )
        let evaluationObject = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(evaluation)) as? [String: Any])
        XCTAssertEqual(Set(evaluationObject.keys), Set([
            "evaluation_id", "candidate_id", "fixture_kind", "fixture_digest",
            "baseline_revision_id", "baseline_digest", "candidate_revision_id",
            "candidate_digest", "requested_case_ids",
        ]))
    }

    func testPromotionSourceAndCommandUseExactAcceptedRunLineage() throws {
        let digest = String(repeating: "a", count: 64)
        let evidenceDigest = String(repeating: "b", count: 64)
        let json = """
        {"view_version":"\(digest)","next_cursor":"","definitions":[],"revisions":[],"candidates":[],"evaluations":[],"bindings":[],"materializations":[],"binding_subjects":[],"promotion_sources":[{"run_id":"run-accepted","run_generation":3,"run_digest":"\(digest)","evidence_ids":["evidence-1"],"evidence_digests":["\(evidenceDigest)"]}]}
        """
        let source = try XCTUnwrap(
            EvolutionAssetWire.decodeSnapshot(Data(json.utf8)).promotionSources.first
        )
        let summary = "Accepted run run-accepted promoted by explicit user action."
        let summaryDigest = SHA256.hash(data: Data(summary.utf8)).map {
            String(format: "%02x", $0)
        }.joined()
        let command = EvolutionAssetCommand(
            action: "promote_run", operationID: "operation-promote",
            journeyID: "123e4567-e89b-42d3-a456-426614174000",
            expectedViewVersion: digest, assetKind: .skill,
            definitionID: "skill.promoted", revisionID: "revision.1",
            candidateID: "candidate-promoted", risk: "medium",
            sourceRunID: source.runID, sourceRunGeneration: source.runGeneration,
            sourceRunDigest: source.runDigest, sourceEvidenceIDs: source.evidenceIDs,
            sourceEvidenceDigests: source.evidenceDigests,
            redactedSummary: summary, redactedSummaryDigest: summaryDigest,
            scopeDifference: "new project-scoped Candidate",
            expectedBenefit: "reuse accepted terminal behavior"
        )
        let object = try XCTUnwrap(
            JSONSerialization.jsonObject(with: JSONEncoder().encode(command)) as? [String: Any]
        )
        XCTAssertEqual(Set(object.keys), Set([
            "candidate_id", "asset_kind", "definition_id", "revision_id",
            "source_run_id", "source_run_generation", "source_run_digest",
            "source_evidence_ids", "source_evidence_digests", "redacted_summary",
            "redacted_summary_digest", "scope_difference", "expected_benefit", "risk",
        ]))
        XCTAssertEqual(object["source_run_generation"] as? Int, 3)
        XCTAssertEqual(object["source_evidence_ids"] as? [String], ["evidence-1"])
    }

    func testImportTemplateAndInstantiationCommandsHaveClosedShapes() throws {
        let digest = String(repeating: "a", count: 64)
        let imported = EvolutionAssetCommand(
            action: "import_skill", operationID: "operation-import",
            journeyID: "123e4567-e89b-42d3-a456-426614174000",
            expectedViewVersion: digest, definitionID: "skill.imported",
            revisionID: "revision.1", candidateID: "candidate-imported",
            name: "Imported", description: "Reviewed import", scope: "project",
            sourcePath: "/private/tmp/SKILL.md", artifactDigest: digest,
            contentDigest: digest, sourceReferenceDigest: digest,
            provenanceDigest: digest, risk: "medium"
        )
        let importedObject = try XCTUnwrap(
            JSONSerialization.jsonObject(with: JSONEncoder().encode(imported)) as? [String: Any]
        )
        XCTAssertEqual(Set(importedObject.keys), Set([
            "candidate_id", "definition_id", "revision_id", "name", "description",
            "subject_scope", "source_path", "supplied_artifact_digest",
            "supplied_content_digest", "external_source_digest", "provenance_digest",
            "dependencies", "compatible_runtime_capabilities", "risk",
        ]))

        let template = EvolutionAssetCommand(
            action: "create_template", operationID: "operation-template",
            journeyID: "123e4567-e89b-42d3-a456-426614174000",
            expectedViewVersion: digest, assetKind: .teamTemplate,
            definitionID: "team.template", revisionID: "revision.1",
            name: "Team Template", description: "Candidate-only", scope: "project",
            sourcePath: "/private/tmp/template.md", artifactDigest: digest,
            contentDigest: digest, risk: "medium", templateOutput: "team_draft",
            parameterSchemaDigest: digest, permissionCeilingDigest: digest,
            scopeCeilingDigest: digest
        )
        let templateObject = try XCTUnwrap(
            JSONSerialization.jsonObject(with: JSONEncoder().encode(template)) as? [String: Any]
        )
        XCTAssertEqual(Set(templateObject.keys), Set([
            "asset_kind", "definition_id", "revision_id", "name", "description",
            "subject_scope", "source_path", "supplied_artifact_digest",
            "supplied_content_digest", "template_output", "parameter_schema_digest",
            "permission_ceiling_digest", "scope_ceiling_digest",
            "compatible_runtime_capabilities", "risk",
        ]))

        let instantiate = EvolutionAssetCommand(
            action: "instantiate_template", operationID: "operation-instantiate",
            journeyID: "123e4567-e89b-42d3-a456-426614174000",
            expectedViewVersion: digest, assetKind: .teamTemplate,
            definitionID: "team.template", revisionID: "revision.1",
            artifactDigest: digest, parameterValues: [], parameterDigest: digest
        )
        let instantiateObject = try XCTUnwrap(
            JSONSerialization.jsonObject(with: JSONEncoder().encode(instantiate)) as? [String: Any]
        )
        XCTAssertEqual(Set(instantiateObject.keys), Set([
            "asset_kind", "definition_id", "revision_id", "revision_digest",
            "parameter_values", "parameter_digest",
        ]))
    }

    @MainActor
    func testCanonicalEvolutionAssetDigestsMatchGoGoldenBytes() throws {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: false)
        defer { try? FileManager.default.removeItem(at: root) }
        let source = root.appendingPathComponent("SKILL.md")
        try Data("# App Skill\n".utf8).write(to: source, options: .atomic)
        let digests = try LocalProductStore.canonicalEvolutionAssetDigests(
            kind: "skill", definitionID: "skill-app",
            revisionID: "revision-1", sourcePath: source.path
        )
        XCTAssertEqual(digests.content, "df76251cc14686fab4dccc638295cfaa8b1a32622752e8b2c337e2138c3c95d5")
        XCTAssertEqual(digests.artifact, "a75161eb8a32f0517511033e4fc8c7948748dad21524498b7af91da29c3eba6e")
    }

    @MainActor
    func testCanonicalTemplateDigestsMatchGoGoldenBytes() throws {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: false)
        defer { try? FileManager.default.removeItem(at: root) }
        let source = root.appendingPathComponent("template.md")
        try Data("# Candidate template\n".utf8).write(to: source, options: .atomic)
        let digests = try LocalProductStore.canonicalEvolutionTemplateDigests(
            kind: "team_template", definitionID: "team-template-1",
            revisionID: "revision-1", sourcePath: source.path,
            templateOutput: "team_draft",
            parameterSchemaDigest: String(repeating: "a", count: 64),
            permissionCeilingDigest: String(repeating: "b", count: 64),
            scopeCeilingDigest: String(repeating: "c", count: 64)
        )
        XCTAssertEqual(digests.artifact, "513c6b3d9f7f5beeee4b6c32e1960daa012ee089dca263b912cc84231c3cb3dd")
        XCTAssertEqual(digests.content, "ca082691078b4b1710ee549833b769f6021c376535a8068f40015f4cf1d72757")
    }
}
