 import SwiftUI
 import LoomLocalAppCore

 public struct ContentView: View {
     @ObservedObject private var store: LocalProductStore
     private let refreshOnAppear: Bool

     public init(
         store: LocalProductStore,
         refreshOnAppear: Bool = true
     ) {
         self.store = store
         self.refreshOnAppear = refreshOnAppear
     }

     public var body: some View {
         LoomWorkspaceShell(store: store)
             .task {
                 if refreshOnAppear {
                     async let read: Void = store.refresh()
                     async let setup: Void = store.refreshSetup()
                     async let permissions: Void = store.refreshPermissions()
                     async let executions: Void = store.refreshExecutions()
                     async let production: Void = store.refreshProduction()
                     _ = await (read, setup, permissions, executions, production)
                 }
             }
             .frame(minWidth: 1080, minHeight: 680)
     }
 }
