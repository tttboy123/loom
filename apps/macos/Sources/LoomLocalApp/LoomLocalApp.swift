import Darwin
import os
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
    @StateObject private var serviceProcessHost: LocalServiceProcessHost
    private let serviceBootstrapper: LocalServiceBootstrapper
    private let serviceLogger = Logger(
        subsystem: "com.earendilworks.loom.local",
        category: "service-bootstrap"
    )

    init() {
        let client: LocalProductClientProtocol
        do {
            client = try LocalIPCClient.defaultClient()
        } catch {
            client = UnavailableLocalProductClient()
        }
        _store = StateObject(wrappedValue: LocalProductStore(client: client))
        _serviceProcessHost = StateObject(
            wrappedValue: LocalServiceProcessHost()
        )
        serviceBootstrapper = LocalServiceBootstrapper(
            registration: SystemLocalServiceRegistration()
        )
    }

    var body: some Scene {
        WindowGroup("Loom") {
            ContentView(
                store: store,
                prepareLocalService: prepareLocalService
            )
                .frame(minWidth: 900, minHeight: 580)
        }
        .defaultSize(width: 1100, height: 720)
        .commands {
            CommandGroup(after: .newItem) {
                Button("Open Folder...") {
                    NotificationCenter.default.post(
                        name: .loomOpenFolderRequested,
                        object: nil
                    )
                }
                .keyboardShortcut("o", modifiers: .command)
            }
        }
    }

    private func prepareLocalService() async -> LocalServiceBootstrapOutcome {
        let outcome = serviceBootstrapper.prepare()
        serviceLogger.info("managed service registration evaluated")
        switch outcome {
        case .enabled, .registered:
            if await serviceProcessHost.waitForDefaultSocket() {
                serviceLogger.info("managed service socket available")
                return outcome
            }
            serviceLogger.notice("starting bundled service fallback")
            return serviceProcessHost.start() ? .registered : .unavailable
        case .requiresApproval, .unavailable:
            serviceLogger.notice("starting foreground bundled service")
            return serviceProcessHost.start() ? .registered : outcome
        }
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

    func chatThread(threadID: String) async throws -> LocalProductChatThread {
        throw LocalProductClientError.unavailable
    }

    func sendChatMessage(threadID: String, content: String) async throws -> LocalProductChatThread {
        throw LocalProductClientError.unavailable
    }
}
