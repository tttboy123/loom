import Darwin
import SwiftUI
import LoomLocalAppCore
import LoomLocalAppUI

private let closedProviderMarkerNames = [
    "DEEPSEEK_BASE_URL",
    "STEPFUN_BASE_URL",
    "MINIMAX_BASE_URL",
    "DEEPSEEK_API_KEY",
    "STEPFUN_API_KEY",
]

@main
enum LoomLocalAppEntry {
    static func main() {
        for name in closedProviderMarkerNames {
            unsetenv(name)
        }
        LoomApplication.main()
    }
}

private struct LoomApplication: App {
    @StateObject private var store: LocalProductStore

    init() {
        let client: LocalProductClientProtocol
        do {
            client = try LocalIPCClient.defaultClient()
        } catch {
            client = UnavailableLocalProductClient()
        }
        _store = StateObject(wrappedValue: LocalProductStore(client: client))
    }

    var body: some Scene {
        WindowGroup("Loom") {
            ContentView(store: store)
                .frame(minWidth: 900, minHeight: 580)
        }
        .defaultSize(width: 1100, height: 720)
    }
}

private struct UnavailableLocalProductClient: LocalProductClientProtocol {
    func snapshot(limit: Int) async throws -> LocalProductSnapshot {
        throw LocalProductClientError.unavailable
    }

    func timeline(
        teamInstanceID: String,
        cursor: String,
        limit: Int
    ) async throws -> LocalProductTimelinePage {
        throw LocalProductClientError.unavailable
    }
}
