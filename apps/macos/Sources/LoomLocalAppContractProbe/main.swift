import Darwin
import Foundation
import LoomLocalAppCore

private struct ProbeOutput: Encodable {
    let snapshot: LocalProductSnapshot
    let timeline: LocalProductTimelinePage?
}

@main
enum LoomLocalAppContractProbe {
    static func main() async {
        do {
            let arguments = CommandLine.arguments
            guard (arguments.count == 3 || arguments.count == 5),
                  arguments[1] == "--socket",
                  arguments.count == 3 || arguments[3] == "--team" else {
                throw LocalProductClientError.invalidRequest
            }
            let client = try LocalIPCClient(
                socketPath: arguments[2],
                requestID: { "loom-swift-contract-probe" }
            )
            guard try await client.ping() else {
                throw LocalProductClientError.invalidResponse
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
}
