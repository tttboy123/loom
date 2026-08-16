 import SwiftUI
 import LoomLocalAppCore

 public struct ContentView: View {
     @ObservedObject private var store: LocalProductStore
     private let refreshOnAppear: Bool
     private let prepareLocalService: () async -> LocalServiceBootstrapOutcome

     public init(
         store: LocalProductStore,
         refreshOnAppear: Bool = true,
         prepareLocalService: @escaping () async -> LocalServiceBootstrapOutcome = {
             .enabled
         }
     ) {
         self.store = store
         self.refreshOnAppear = refreshOnAppear
         self.prepareLocalService = prepareLocalService
     }

     public var body: some View {
         LoomWorkspaceShell(store: store)
             .task {
                 if refreshOnAppear {
                     let service = await prepareLocalService()
                     await store.connectWithRetry(
                         maxAttempts: service.shouldWaitForConnection ? 12 : 1
                     )
                     async let setup: Void = store.refreshSetup()
                     async let permissions: Void = store.refreshPermissions()
                     async let executions: Void = store.refreshExecutions()
                     async let production: Void = store.refreshProduction()
                     _ = await (setup, permissions, executions, production)
                 }
             }
             .frame(minWidth: 900, minHeight: 580)
     }
 }
