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

@main
enum LoomLocalAppContractProbe {
    static func main() async {
        do {
            let arguments = CommandLine.arguments
            guard arguments.count == 3 || arguments.count == 4 || arguments.count == 5,
                arguments[1] == "--socket",
                arguments.count == 3 || arguments.count == 4 && arguments[3] == "--execution"
                    || arguments[3] == "--team" || arguments[3] == "--team-all"
                    || arguments[3] == "--decision"
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

    private static func decisionKind(
        _ value: String
    ) throws -> LocalProductDecisionKind {
        guard let kind = LocalProductDecisionKind(rawValue: value) else {
            throw LocalProductClientError.invalidRequest
        }
        return kind
    }
}
