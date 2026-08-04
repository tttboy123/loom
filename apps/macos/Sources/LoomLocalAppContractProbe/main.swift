import Darwin
import Foundation
import LoomLocalAppCore

private struct ProbeOutput: Encodable {
    let snapshot: LocalProductSnapshot
    let timeline: LocalProductTimelinePage?
}

private struct ProbeDecisionOutput: Encodable {
    let kind: String
    let missionID: String
    let actions: [String]
    let prepared: Bool
}

private struct ProbeExecutionOutput: Encodable {
    let preflightDigest: String
    let expiresAt: String
    let status: String
    let executionDigest: String
}

private struct ProbeSideTaskOutput: Encodable {
    let sideTaskID: String
    let status: String
    let viewVersion: String
    let availableDecisions: [String]
}

private struct ProbeAssetOutput: Encodable {
    let viewVersion: String
    let definitions: Int
    let revisions: Int
    let candidates: Int
    let evaluations: Int
    let bindings: Int
    let materializations: Int
}

private struct ProbeAssetActionOutput: Encodable {
    let action: String
    let operationID: String
    let eventIDs: [String]
}

private struct ProbeQueueOutput: Encodable {
    let viewVersion: String
    let jobs: Int
    let gaps: Int
    let successors: Int
    let jobIDs: [String]
}

private struct ProbeQueueActionOutput: Encodable {
    let action: String
    let operationID: String
    let eventIDs: [String]
    let jobID: String?
    let gapID: String?
    let successorProposalID: String?
    let disposition: String?
    let status: String?
}

private struct ProbeWorkersOutput: Encodable {
    let viewVersion: String
    let attempts: Int
    let active: Int
    let lanes: [String: Int]
}

private struct ProbeWorkersCommandOutput: Encodable {
    let action: String
    let operationID: String
    let eventIDs: [String]
    let attemptID: String?
    let jobID: String?
    let generation: Int64?
    let lane: String?
    let disposition: String?
}

@main
enum LoomLocalAppContractProbe {
    static func main() async {
        do {
            let arguments = CommandLine.arguments
            guard arguments.count == 3 || arguments.count == 4 || arguments.count == 5
                    || arguments.count == 6 || arguments.count == 7,
                arguments[1] == "--socket",
                arguments.count == 3 || arguments.count == 4 && arguments[3] == "--execution"
                    || arguments[3] == "--team" || arguments[3] == "--team-all"
                    || arguments[3] == "--decision"
                    || arguments.count == 5 && arguments[3] == "--assets"
                    || arguments.count == 5 && arguments[3] == "--queue-snapshot"
                    || arguments.count == 5 && arguments[3] == "--workers-snapshot"
                    || arguments.count == 7 && arguments[3] == "--workers-command"
                    || (arguments.count == 6 || arguments.count == 7) && arguments[3] == "--asset-action"
                    || arguments.count == 6 && (arguments[3] == "--queue-create-job"
                        || arguments[3] == "--queue-gap-observe"
                        || arguments[3] == "--queue-successor-compile")
                    || arguments.count == 6 && arguments[3] == "--side-task-read"
            else {
                throw LocalProductClientError.invalidRequest
            }
            let client = try LocalIPCClient(
                socketPath: arguments[2],
                requestID: { "loom-swift-contract-probe" }
            )
            guard try await client.ping() else {
                throw LocalProductClientError.invalidResponse
            }
            if arguments.count == 5 && arguments[3] == "--assets" {
                let snapshot = try await client.evolutionAssetSnapshot(
                    journeyID: arguments[4], cursor: "", limit: 64
                )
                let encoded = try JSONEncoder().encode(ProbeAssetOutput(
                    viewVersion: snapshot.viewVersion,
                    definitions: snapshot.definitions.count,
                    revisions: snapshot.revisions.count,
                    candidates: snapshot.candidates.count,
                    evaluations: snapshot.evaluations.count,
                    bindings: snapshot.bindings.count,
                    materializations: snapshot.materializations.count
                ))
                guard let output = String(data: encoded, encoding: .utf8) else {
                    throw LocalProductClientError.invalidResponse
                }
                print(output)
                return
            }
            if arguments.count == 5 && arguments[3] == "--queue-snapshot" {
                let snapshot = try await client.queueSnapshot(
                    journeyID: arguments[4], cursor: "", limit: 64
                )
                let encoded = try JSONEncoder().encode(ProbeQueueOutput(
                    viewVersion: snapshot.viewVersion,
                    jobs: snapshot.jobs.count,
                    gaps: snapshot.gaps.count,
                    successors: snapshot.successors.count,
                    jobIDs: snapshot.jobs.map(\.jobID)
                ))
                guard let output = String(data: encoded, encoding: .utf8) else {
                    throw LocalProductClientError.invalidResponse
                }
                print(output)
                return
            }
            if arguments.count == 5 && arguments[3] == "--workers-snapshot" {
                let snapshot = try await client.workersSnapshot(
                    journeyID: arguments[4], cursor: "", limit: 64
                )
                let encoded = try JSONEncoder().encode(ProbeWorkersOutput(
                    viewVersion: snapshot.viewVersion,
                    attempts: snapshot.attempts.count,
                    active: snapshot.activeWorkers.count,
                    lanes: snapshot.lanes
                ))
                guard let output = String(data: encoded, encoding: .utf8) else {
                    throw LocalProductClientError.invalidResponse
                }
                print(output)
                return
            }
            if arguments.count == 7 && arguments[3] == "--workers-command" {
                let journeyID = arguments[4]
                let action = arguments[5]
                let input = try loadJSONObject(arguments[6])
                let receipt = try await client.workersCommand(
                    journeyID: journeyID,
                    operationID: "sf2-" + UUID().uuidString.lowercased(),
                    action: action,
                    input: input
                )
                let encoded = try JSONEncoder().encode(ProbeWorkersCommandOutput(
                    action: receipt.action,
                    operationID: receipt.operationID,
                    eventIDs: receipt.eventIDs,
                    attemptID: receipt.attemptID,
                    jobID: receipt.jobID,
                    generation: receipt.generation,
                    lane: receipt.lane,
                    disposition: receipt.disposition
                ))
                guard let output = String(data: encoded, encoding: .utf8) else {
                    throw LocalProductClientError.invalidResponse
                }
                print(output)
                return
            }
            if arguments.count == 6 && arguments[3] == "--queue-create-job" {
                let journeyID = arguments[4]
                let input = try loadJSONFile(arguments[5], as: QueueJobSubmission.self)
                let receipt = try await client.queueCommand(QueueCommand(
                    operationID: "queue-op-" + input.jobID,
                    action: "create_job",
                    journeyID: journeyID,
                    input: input
                ))
                guard receipt.action == "create_job" else {
                    throw LocalProductClientError.invalidResponse
                }
                try printQueueAction(receipt)
                return
            }
            if arguments.count == 6 && arguments[3] == "--queue-gap-observe" {
                let journeyID = arguments[4]
                let input = try loadJSONFile(arguments[5], as: QueueGapProposalSubmission.self)
                let receipt = try await client.queueCommand(QueueCommand(
                    operationID: "queue-op-gap-" + String(input.sourceDigests.joined().prefix(12)),
                    action: "gap_observe",
                    journeyID: journeyID,
                    input: input
                ))
                guard receipt.action == "gap_observe" else {
                    throw LocalProductClientError.invalidResponse
                }
                try printQueueAction(receipt)
                return
            }
            if arguments.count == 6 && arguments[3] == "--queue-successor-compile" {
                let journeyID = arguments[4]
                let input = try loadJSONFile(arguments[5], as: QueueSuccessorCompileRequest.self)
                let receipt = try await client.queueCommand(QueueCommand(
                    operationID: "queue-op-spr-" + String(input.gapID.prefix(12)),
                    action: "successor_compile",
                    journeyID: journeyID,
                    input: input
                ))
                guard receipt.action == "successor_compile" else {
                    throw LocalProductClientError.invalidResponse
                }
                try printQueueAction(receipt)
                return
            }
            if (arguments.count == 6 || arguments.count == 7) && arguments[3] == "--asset-action" {
                let journeyID = arguments[4]
                let action = arguments[5]
                let variant = arguments.count == 7 ? arguments[6] : ""
                let snapshot = try await client.evolutionAssetSnapshot(
                    journeyID: journeyID, cursor: "", limit: 64
                )
                let command = try assetCommand(
                    action: action, journeyID: journeyID, snapshot: snapshot,
                    variant: variant
                )
                let receipt = try await client.evolutionAssetCommand(command)
                guard receipt.operationID == command.operationID,
                      receipt.action == command.action,
                      !receipt.eventIDs.isEmpty else {
                    throw LocalProductClientError.invalidResponse
                }
                let encoded = try JSONEncoder().encode(ProbeAssetActionOutput(
                    action: receipt.action,
                    operationID: receipt.operationID,
                    eventIDs: receipt.eventIDs
                ))
                guard let output = String(data: encoded, encoding: .utf8) else {
                    throw LocalProductClientError.invalidResponse
                }
                print(output)
                return
            }
            if arguments.count == 6 && arguments[3] == "--side-task-read" {
                let result = try await client.readSideTask(
                    LocalProductSideTaskReadRequest(
                        sideTaskID: arguments[4],
                        expectedViewVersion: arguments[5],
                        correlationID: "33333333-3333-4333-8333-333333333333"
                    )
                )
                let encoded = try JSONEncoder().encode(
                    ProbeSideTaskOutput(
                        sideTaskID: result.summary.sideTaskID,
                        status: result.summary.status,
                        viewVersion: result.viewVersion,
                        availableDecisions: result.summary.availableDecisions
                    )
                )
                guard let output = String(data: encoded, encoding: .utf8) else {
                    throw LocalProductClientError.invalidResponse
                }
                print(output)
                return
            }
            if arguments.count == 4 {
                let preflightEnvelope = try await client.executeMission(
                    .preflight(
                        missionID: "mission/team-swift-contract",
                        teamInstanceID: "team-swift-contract",
                        workPackageID: "work-package.coding",
                        workPackageDigest: String(repeating: "a", count: 64),
                        objective: "Verify the strict Swift execution contract",
                        expectedViewVersion: String(repeating: "b", count: 64),
                        correlationID: "11111111-1111-4111-8111-111111111111"
                    )
                )
                guard let preflight = preflightEnvelope.preflight else {
                    throw LocalProductClientError.invalidResponse
                }
                let startEnvelope = try await client.executeMission(
                    .start(
                        preflight: preflight,
                        objective: "Verify the strict Swift execution contract",
                        correlationID: "22222222-2222-4222-8222-222222222222"
                    )
                )
                guard let result = startEnvelope.result else {
                    throw LocalProductClientError.invalidResponse
                }
                let encoded = try JSONEncoder().encode(
                    ProbeExecutionOutput(
                        preflightDigest: preflight.preflightDigest,
                        expiresAt: preflight.expiresAt,
                        status: result.status,
                        executionDigest: result.executionDigest
                    )
                )
                guard let output = String(data: encoded, encoding: .utf8) else {
                    throw LocalProductClientError.invalidResponse
                }
                print(output)
                return
            }
            if arguments.count == 5 && arguments[3] == "--decision" {
                let kind = try decisionKind(arguments[4])
                let sheet = try await client.readMissionDecision(
                    LocalProductDecisionCommand(
                        operation: "read",
                        kind: kind,
                        action: "read",
                        missionID: "mission/team-1",
                        teamInstanceID: "team-1",
                        viewVersion: String(repeating: "a", count: 64),
                        decisionID: "decision-1",
                        decisionDigest: String(repeating: "b", count: 64),
                        logicalNodeID: "main",
                        attemptNumber: 1,
                        claimGeneration: 1,
                        correlationID:
                            "11111111-1111-4111-8111-111111111111"
                    )
                )
                let encoded = try JSONEncoder().encode(
                    ProbeDecisionOutput(
                        kind: sheet.kind.rawValue,
                        missionID: sheet.missionID,
                        actions: sheet.actions,
                        prepared: sheet.prepared
                    )
                )
                guard let output = String(data: encoded, encoding: .utf8) else {
                    throw LocalProductClientError.invalidResponse
                }
                print(output)
                return
            }
            if arguments.count == 5 && arguments[3] == "--team-all" {
                let store = LocalProductStore(client: client)
                await store.refresh()
                guard let team = store.snapshot?.teams.first(where: {
                    $0.teamInstanceID == arguments[4]
                }) else {
                    throw LocalProductClientError.invalidResponse
                }
                store.selectTeam(team)
                await store.activateSelectedTeam()
                guard store.timelineState == .loaded,
                      let timeline = store.timeline,
                      let snapshot = store.snapshot else {
                    throw LocalProductClientError.invalidResponse
                }
                let encoder = JSONEncoder()
                encoder.outputFormatting = [.sortedKeys]
                let encoded = try encoder.encode(
                    ProbeOutput(snapshot: snapshot, timeline: timeline)
                )
                guard let output = String(data: encoded, encoding: .utf8) else {
                    throw LocalProductClientError.invalidResponse
                }
                print(output)
                return
            }
            let snapshot = try await client.snapshot(limit: 64)
            let timeline: LocalProductTimelinePage?
            if arguments.count == 5 {
                timeline = try await client.timeline(
                    teamInstanceID: arguments[4],
                    cursor: "",
                    limit: 64
                )
            } else {
                timeline = nil
            }
            let encoder = JSONEncoder()
            encoder.outputFormatting = [.sortedKeys]
            let encoded = try encoder.encode(
                ProbeOutput(snapshot: snapshot, timeline: timeline)
            )
            guard let output = String(data: encoded, encoding: .utf8) else {
                throw LocalProductClientError.invalidResponse
            }
            print(output)
        } catch let error as LocalIPCRemoteError {
            print("error:\(error.code.rawValue):\(error.recoverable)")
            Darwin.exit(2)
        } catch let error as LocalProductClientError {
            print("error:\(error.rawValue)")
            Darwin.exit(2)
        } catch {
            print("error:invalid_response")
            Darwin.exit(2)
        }
    }

    private static func loadJSONFile<Value: Decodable>(
        _ path: String,
        as type: Value.Type
    ) throws -> Value {
        let data = try Data(contentsOf: URL(fileURLWithPath: path))
        return try JSONDecoder().decode(type, from: data)
    }

    private static func loadJSONObject(_ path: String) throws -> [String: Any] {
        let data = try Data(contentsOf: URL(fileURLWithPath: path))
        guard let object = try JSONSerialization.jsonObject(with: data) as? [String: Any] else {
            throw LocalProductClientError.invalidRequest
        }
        return object
    }

    private static func printQueueAction(_ receipt: QueueCommandReceipt) throws {
        let encoded = try JSONEncoder().encode(ProbeQueueActionOutput(
            action: receipt.action,
            operationID: receipt.operationID,
            eventIDs: receipt.eventIDs,
            jobID: receipt.jobID,
            gapID: receipt.gapID,
            successorProposalID: receipt.successorProposalID,
            disposition: receipt.disposition,
            status: receipt.status
        ))
        guard let output = String(data: encoded, encoding: .utf8) else {
            throw LocalProductClientError.invalidResponse
        }
        print(output)
    }

    private static func decisionKind(
        _ value: String
    ) throws -> LocalProductDecisionKind {
        guard let kind = LocalProductDecisionKind(rawValue: value) else {
            throw LocalProductClientError.invalidRequest
        }
        return kind
    }

    private static func assetCommand(
        action: String,
        journeyID: String,
        snapshot: EvolutionAssetSnapshot,
        variant: String
    ) throws -> EvolutionAssetCommand {
        let operationID = UUID().uuidString.lowercased()
        func applyVariant(_ command: EvolutionAssetCommand) -> EvolutionAssetCommand {
            switch variant {
            case "stale_view":
                return EvolutionAssetCommand(
                    action: command.action, operationID: command.operationID,
                    journeyID: command.journeyID,
                    expectedViewVersion: "stale-view-version",
                    expectedStreamHeads: command.expectedStreamHeads,
                    decisionSource: command.decisionSource,
                    assetKind: command.assetKind,
                    definitionID: command.definitionID,
                    revisionID: command.revisionID,
                    candidateID: command.candidateID,
                    name: command.name, description: command.description,
                    scope: command.scope, sourcePath: command.sourcePath,
                    artifactDigest: command.artifactDigest,
                    contentDigest: command.contentDigest,
                    sourceScope: command.sourceScope,
                    sourceReferenceDigest: command.sourceReferenceDigest,
                    provenanceDigest: command.provenanceDigest,
                    risk: command.risk, reasonCode: command.reasonCode,
                    expectedPreviousRevisionID: command.expectedPreviousRevisionID,
                    targetRevisionID: command.targetRevisionID,
                    targetRevisionDigest: command.targetRevisionDigest,
                    fromRevisionID: command.fromRevisionID,
                    fromDigest: command.fromDigest,
                    evaluationIDs: command.evaluationIDs,
                    evaluationID: command.evaluationID,
                    fixtureKind: command.fixtureKind,
                    fixtureDigest: command.fixtureDigest,
                    baselineRevisionID: command.baselineRevisionID,
                    baselineDigest: command.baselineDigest,
                    candidateRevisionID: command.candidateRevisionID,
                    candidateDigest: command.candidateDigest,
                    requestedCaseIDs: command.requestedCaseIDs,
                    subject: command.subject, bindings: command.bindings,
                    assetRevisionSetDigest: command.assetRevisionSetDigest,
                    sourceRunID: command.sourceRunID,
                    sourceRunGeneration: command.sourceRunGeneration,
                    sourceRunDigest: command.sourceRunDigest,
                    sourceEvidenceIDs: command.sourceEvidenceIDs,
                    sourceEvidenceDigests: command.sourceEvidenceDigests,
                    redactedSummary: command.redactedSummary,
                    redactedSummaryDigest: command.redactedSummaryDigest,
                    scopeDifference: command.scopeDifference,
                    expectedBenefit: command.expectedBenefit,
                    templateOutput: command.templateOutput,
                    parameterSchemaDigest: command.parameterSchemaDigest,
                    permissionCeilingDigest: command.permissionCeilingDigest,
                    scopeCeilingDigest: command.scopeCeilingDigest,
                    parameterValues: command.parameterValues,
                    parameterDigest: command.parameterDigest
                )
            case "wrong_digest":
                return EvolutionAssetCommand(
                    action: command.action, operationID: command.operationID,
                    journeyID: command.journeyID,
                    expectedViewVersion: command.expectedViewVersion,
                    expectedStreamHeads: command.expectedStreamHeads,
                    decisionSource: command.decisionSource,
                    assetKind: command.assetKind,
                    definitionID: command.definitionID,
                    revisionID: command.revisionID,
                    candidateID: command.candidateID,
                    name: command.name, description: command.description,
                    scope: command.scope, sourcePath: command.sourcePath,
                    artifactDigest: String(repeating: "f", count: 64),
                    contentDigest: command.contentDigest,
                    sourceScope: command.sourceScope,
                    sourceReferenceDigest: command.sourceReferenceDigest,
                    provenanceDigest: command.provenanceDigest,
                    risk: command.risk, reasonCode: command.reasonCode,
                    expectedPreviousRevisionID: command.expectedPreviousRevisionID,
                    targetRevisionID: command.targetRevisionID,
                    targetRevisionDigest: command.targetRevisionDigest,
                    fromRevisionID: command.fromRevisionID,
                    fromDigest: command.fromDigest,
                    evaluationIDs: command.evaluationIDs,
                    evaluationID: command.evaluationID,
                    fixtureKind: command.fixtureKind,
                    fixtureDigest: command.fixtureDigest,
                    baselineRevisionID: command.baselineRevisionID,
                    baselineDigest: command.baselineDigest,
                    candidateRevisionID: command.candidateRevisionID,
                    candidateDigest: command.candidateDigest,
                    requestedCaseIDs: command.requestedCaseIDs,
                    subject: command.subject, bindings: command.bindings,
                    assetRevisionSetDigest: command.assetRevisionSetDigest,
                    sourceRunID: command.sourceRunID,
                    sourceRunGeneration: command.sourceRunGeneration,
                    sourceRunDigest: command.sourceRunDigest,
                    sourceEvidenceIDs: command.sourceEvidenceIDs,
                    sourceEvidenceDigests: command.sourceEvidenceDigests,
                    redactedSummary: command.redactedSummary,
                    redactedSummaryDigest: command.redactedSummaryDigest,
                    scopeDifference: command.scopeDifference,
                    expectedBenefit: command.expectedBenefit,
                    templateOutput: command.templateOutput,
                    parameterSchemaDigest: command.parameterSchemaDigest,
                    permissionCeilingDigest: command.permissionCeilingDigest,
                    scopeCeilingDigest: command.scopeCeilingDigest,
                    parameterValues: command.parameterValues,
                    parameterDigest: command.parameterDigest
                )
            default:
                return command
            }
        }
        switch action {
        case "create_skill":
            guard let sourcePath = ProcessInfo.processInfo.environment["LOOM_PROBE_SOURCE"] else {
                throw LocalProductClientError.invalidRequest
            }
            let name = (sourcePath as NSString).lastPathComponent
                .replacingOccurrences(of: (sourcePath as NSString).pathExtension.isEmpty ? "" : "." + (sourcePath as NSString).pathExtension, with: "")
            let identity = LocalProductStore.sha256Text(
                try String(contentsOfFile: sourcePath, encoding: .utf8)
            )
            let definitionID = "skill-" + String(identity.prefix(16))
            let digests = try LocalProductStore.canonicalEvolutionAssetDigests(
                kind: "skill", definitionID: definitionID,
                revisionID: "revision-1", sourcePath: sourcePath
            )
            return EvolutionAssetCommand(
                action: "create_skill", operationID: operationID,
                journeyID: journeyID, expectedViewVersion: snapshot.viewVersion,
                assetKind: .skill, definitionID: definitionID,
                revisionID: "revision-1", name: name,
                description: "Reviewed local Candidate", scope: "project",
                sourcePath: sourcePath, artifactDigest: digests.artifact,
                contentDigest: digests.content, sourceScope: "local",
                sourceReferenceDigest: digests.artifact,
                provenanceDigest: digests.artifact, risk: "low"
            )
        case "reject", "retain":
            guard let candidate = snapshot.candidates.first(where: { $0.decision.isEmpty }),
                  let revision = snapshot.revisions.first(where: {
                      $0.definitionID == candidate.definitionID && $0.revisionID == candidate.revisionID
                  }) else {
                throw LocalProductClientError.invalidRequest
            }
            let retain = action == "retain"
            return EvolutionAssetCommand(
                action: action, operationID: operationID,
                journeyID: journeyID, expectedViewVersion: snapshot.viewVersion,
                decisionSource: "user_explicit",
                assetKind: candidate.assetKind,
                definitionID: candidate.definitionID,
                revisionID: candidate.revisionID,
                candidateID: candidate.candidateID,
                artifactDigest: revision.artifactDigest,
                reasonCode: retain ? "keep_for_later" : "user_rejected"
            )
        case "record_evaluation":
            guard let candidate = snapshot.candidates.first(where: { $0.decision.isEmpty }),
                  let revision = snapshot.revisions.first(where: {
                      $0.definitionID == candidate.definitionID && $0.revisionID == candidate.revisionID
                  }) else {
                throw LocalProductClientError.invalidRequest
            }
            let baseline = snapshot.revisions.first(where: {
                $0.definitionID == candidate.definitionID &&
                    $0.revisionID == snapshot.definitions.first(where: {
                        $0.definitionID == candidate.definitionID
                    })?.activeRevisionID
            }) ?? revision
            let caseIDs = ["artifact_digest", "runtime_compatibility", "security_boundary"]
            let fixture = "{\"schema_version\":1,\"fixture_kind\":\"synthetic\",\"case_ids\":[\"artifact_digest\",\"runtime_compatibility\",\"security_boundary\"],\"expected\":{\"quality_result\":\"pass\",\"failure_count\":0,\"usage_observed\":true,\"usage_microunits\":250,\"cost_observed\":true,\"cost_microunits\":1250,\"cost_currency\":\"USD\",\"compatibility_result\":\"compatible\",\"applicable_scope\":\"bounded_fixture\",\"regression_result\":\"equivalent\",\"security_result\":\"pass\"}}"
            let fixtureDigest = LocalProductStore.sha256Text(fixture)
            return applyVariant(EvolutionAssetCommand(
                action: "record_evaluation", operationID: operationID,
                journeyID: journeyID, expectedViewVersion: snapshot.viewVersion,
                candidateID: candidate.candidateID,
                evaluationID: UUID().uuidString.lowercased(),
                fixtureKind: "synthetic", fixtureDigest: fixtureDigest,
                baselineRevisionID: baseline.revisionID,
                baselineDigest: baseline.artifactDigest,
                candidateRevisionID: revision.revisionID,
                candidateDigest: revision.artifactDigest,
                requestedCaseIDs: caseIDs
            ))
        case "activate":
            guard let candidate = snapshot.candidates.first(where: { $0.decision.isEmpty }),
                  let revision = snapshot.revisions.first(where: {
                      $0.definitionID == candidate.definitionID && $0.revisionID == candidate.revisionID
                  }) else {
                throw LocalProductClientError.invalidRequest
            }
            let expectedPrevious = snapshot.definitions.first(where: {
                $0.definitionID == candidate.definitionID
            })?.activeRevisionID ?? ""
            return applyVariant(EvolutionAssetCommand(
                action: "activate", operationID: operationID,
                journeyID: journeyID, expectedViewVersion: snapshot.viewVersion,
                decisionSource: "user_explicit",
                assetKind: candidate.assetKind,
                definitionID: candidate.definitionID,
                revisionID: candidate.revisionID,
                candidateID: candidate.candidateID,
                artifactDigest: revision.artifactDigest,
                expectedPreviousRevisionID: expectedPrevious,
                evaluationIDs: candidate.requiredEvaluationIDs
            ))
        case "set_binding":
            guard let definition = snapshot.definitions.first,
                  !definition.activeRevisionID.isEmpty,
                  let revision = snapshot.revisions.first(where: {
                      $0.definitionID == definition.definitionID &&
                          $0.revisionID == definition.activeRevisionID
                  }),
                  let subject = snapshot.bindingSubjects.first(where: {
                      $0.subjectKind == "work_package" && $0.subjectID == "work-package.coding"
                  }) else {
                throw LocalProductClientError.invalidRequest
            }
            let binding = EvolutionAssetExactBinding(
                assetKind: revision.assetKind,
                definitionID: revision.definitionID,
                revisionID: revision.revisionID,
                sha256Digest: revision.artifactDigest,
                sourceScope: revision.sourceScope
            )
            func quoted(_ value: String) throws -> String {
                let encoded = try JSONEncoder().encode(value)
                guard let text = String(data: encoded, encoding: .utf8) else {
                    throw LocalProductClientError.invalidRequest
                }
                return text
            }
            let value = "[{\"asset_kind\":\(try quoted(revision.assetKind.rawValue)),\"definition_id\":\(try quoted(revision.definitionID)),\"revision_id\":\(try quoted(revision.revisionID)),\"sha256_digest\":\(try quoted(revision.artifactDigest)),\"source_scope\":\(try quoted(revision.sourceScope))}]"
            let setDigest = LocalProductStore.sha256Text(value)
            return applyVariant(EvolutionAssetCommand(
                action: "set_binding", operationID: operationID,
                journeyID: journeyID, expectedViewVersion: snapshot.viewVersion,
                subject: subject, bindings: [binding],
                assetRevisionSetDigest: setDigest
            ))
        default:
            throw LocalProductClientError.invalidRequest
        }
    }
}
