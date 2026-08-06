
 import LoomLocalAppCore
 import SwiftUI
 
 // MARK: - Navigation model
 
 public enum LoomWorkspaceNavigationItem: String, CaseIterable, Identifiable, Sendable {
     case home = "Home"
     case work = "Work"
     case teams = "Teams"
     case attention = "Attention"
     case library = "Library"
     case runtimes = "Runtimes"
 
     public var id: String { rawValue }
 
     public var symbol: String {
         switch self {
         case .home: return "bubble.left.fill"
         case .work: return "square.3.layers.3d"
         case .teams: return "person.3"
         case .attention: return "exclamationmark.triangle"
         case .library: return "books.vertical"
         case .runtimes: return "cpu"
         }
     }
 
     public var accessibilityLabel: String {
         switch self {
         case .home: return "Chat"
         case .work: return "Mission Board"
         case .teams: return "Teams"
         case .attention: return "Attention"
         case .library: return "Library"
         case .runtimes: return "Runtimes"
         }
     }
 }
 
 // MARK: - Shell
 
 public struct LoomWorkspaceShell: View {
     @ObservedObject private var store: LocalProductStore
     @State private var selectedNavigation: LoomWorkspaceNavigationItem = .home
     @State private var showProviders = false
 
     public init(store: LocalProductStore) {
         self.store = store
     }
 
 
    private static func navigationItemFor(route: MissionWorkbenchRoute) -> LoomWorkspaceNavigationItem {
        switch route {
        case .teams:
            return .teams
        case .attention:
            return .attention
        case .library:
            return .library
        case .board, .mission:
            return .work
        }
    }

    public var body: some View {
         HSplitView {
             navigationRail
                 .frame(
                     minWidth: LoomGraphite.railWidth,
                     idealWidth: LoomGraphite.railWidth,
                     maxWidth: LoomGraphite.railWidth
                 )
 
             centerWorkspace
                 .frame(minWidth: 400)
 
             if selectedNavigation != .home {
                 governancePanel
                     .frame(minWidth: 520)
             }
         }
         .background(LoomGraphite.canvas)
         .sheet(isPresented: $showProviders) {
             providerManagementPlaceholder
                 .frame(minWidth: 560, minHeight: 380)
         }
         .accessibilityElement(children: .contain)
         .accessibilityLabel("Loom workspace")
     }
 
     // MARK: - Left rail
 
     private var navigationRail: some View {
         VStack(alignment: .leading, spacing: 0) {
             HStack(spacing: 9) {
                 Image(systemName: "square.3.layers.3d")
                     .foregroundStyle(LoomGraphite.accent)
                 Text("Loom")
                     .font(.headline)
                 Spacer()
             }
             .padding(.horizontal, 14)
             .frame(height: 52)
             .accessibilityElement(children: .combine)
             .accessibilityLabel("Loom")
 
             Divider()
                 .background(LoomGraphite.separator)
 
             ScrollView {
                 VStack(alignment: .leading, spacing: 6) {
                     ForEach(LoomWorkspaceNavigationItem.allCases) { item in
                         navigationButton(item)
                     }
                 }
                 .padding(.horizontal, 10)
                 .padding(.vertical, 12)
             }
 
             Spacer()
 
             Divider()
                 .background(LoomGraphite.separator)
 
             Button {
                 showProviders = true
             } label: {
                 HStack(spacing: 10) {
                     Image(systemName: "gearshape.2")
                         .frame(width: 20)
                     Text("Providers")
                         .font(.callout)
                     Spacer()
                 }
                 .padding(.horizontal, 10)
                 .frame(height: 36)
                 .frame(maxWidth: .infinity, alignment: .leading)
                 .background(Color.clear, in: RoundedRectangle(cornerRadius: 9))
             }
             .buttonStyle(.plain)
             .loomActionTarget()
             .accessibilityLabel("Providers")
             .padding(.horizontal, 10)
             .padding(.vertical, 10)
         }
         .background(LoomGraphite.rail)
     }
 
     private func navigationButton(_ item: LoomWorkspaceNavigationItem) -> some View {
         let selected = selectedNavigation == item
         return Button {
             selectedNavigation = item
             syncWorkbenchRoute(for: item)
         } label: {
             HStack(spacing: 10) {
                 Image(systemName: item.symbol)
                     .frame(width: 20)
                     .foregroundStyle(selected ? LoomGraphite.accent : Color.secondary)
                 Text(item.rawValue)
                     .font(.callout.weight(selected ? .semibold : .regular))
                 Spacer()
             }
             .padding(.horizontal, 10)
             .frame(height: 36)
             .frame(maxWidth: .infinity, alignment: .leading)
             .background(
                 selected ? LoomGraphite.accentMuted : Color.clear,
                 in: RoundedRectangle(cornerRadius: 9, style: .continuous)
             )
             .overlay {
                 if selected {
                     RoundedRectangle(cornerRadius: 9, style: .continuous)
                         .stroke(LoomGraphite.accent.opacity(0.45), lineWidth: 1)
                 }
             }
         }
         .buttonStyle(.plain)
         .loomActionTarget()
         .accessibilityLabel(item.accessibilityLabel)
         .accessibilityAddTraits(selected ? .isSelected : [])
         .help(item.rawValue)
     }
 
     private func syncWorkbenchRoute(for item: LoomWorkspaceNavigationItem) {
         switch item {
         case .work:
             store.showMissionBoard()
         case .teams:
             store.showMissionTeams()
         case .attention:
             store.showMissionAttention()
         case .library:
             store.showMissionLibrary()
         case .home, .runtimes:
             break
         }
     }
 
     // MARK: - Center workspace
 
     private var centerWorkspace: some View {
         VStack(spacing: 0) {
             centerHeader
                 .padding(.horizontal, 18)
                 .padding(.vertical, 12)
                 .background(LoomGraphite.surface)
 
             connectionStripIfNeeded
 
             ScrollView {
                 VStack(alignment: .leading, spacing: 18) {
                     welcomeCard
                     recentWorkCard
                 }
                 .frame(maxWidth: 760, alignment: .leading)
                 .padding(.horizontal, 24)
                 .padding(.vertical, 24)
                 .frame(maxWidth: .infinity)
             }
 
             Spacer()
 
             composer
                 .padding(.horizontal, 18)
                 .padding(.bottom, 14)
                 .background(LoomGraphite.surface)
         }
         .background(LoomGraphite.raised)
     }
 
     private var centerHeader: some View {
         HStack(spacing: 10) {
             VStack(alignment: .leading, spacing: 2) {
                 Text("What would you like to do?")
                     .font(.headline)
                 Text("Describe a task or start a conversation.")
                     .font(.caption)
                     .foregroundStyle(Color.secondary)
             }
             Spacer()
             Button {
                 selectedNavigation = .work
                 store.showMissionBoard()
             } label: {
                 Image(systemName: "square.3.layers.3d")
                     .frame(width: 36, height: 36)
             }
             .buttonStyle(.plain)
             .accessibilityLabel("Open Mission Board")
             .help("Open Mission Board")
         }
     }
 
     @ViewBuilder
     private var connectionStripIfNeeded: some View {
         if experience.state != .connectedEmpty,
            experience.state != .connectedPopulated {
             connectionStrip
         }
     }
 
     private var connectionStrip: some View {
         HStack(spacing: 10) {
             connectionIndicator
             Text(experience.connection.title)
                 .font(.callout)
             Spacer()
             if experience.connection.offersRefresh {
                 Button {
                     Task {
                         await store.refresh()
                     }
                 } label: {
                     Text("Retry")
                 }
             }
         }
         .padding(.horizontal, 18)
         .padding(.vertical, 10)
         .background(stripBackground)
     }
 
     private var connectionIndicator: some View {
         Circle()
             .fill(indicatorColor)
             .frame(width: 10, height: 10)
     }
 
     private var stripBackground: some View {
         switch experience.state {
         case .offline, .offlinePreserved:
             return LoomGraphite.statusWarning.opacity(0.12)
         case .fatal:
             return LoomGraphite.statusDanger.opacity(0.12)
         default:
             return LoomGraphite.accentMuted
         }
     }
 
     private var indicatorColor: Color {
         switch experience.state {
         case .loading:
             return LoomGraphite.accent
         case .connectedEmpty, .connectedPopulated:
             return LoomGraphite.statusSuccess
         case .partial, .stalePreserved:
             return LoomGraphite.statusWarning
         case .offline, .offlinePreserved, .fatal:
             return LoomGraphite.statusDanger
         }
     }
 
     private var welcomeCard: some View {
         VStack(alignment: .leading, spacing: 8) {
             Text("Start with a task")
                 .font(.title3.weight(.semibold))
             Text("Describe the outcome you want. Loom stays in conversation mode until you explicitly choose to use an Agent Team.")
                 .foregroundStyle(Color.secondary)
                 .fixedSize(horizontal: false, vertical: true)
             HStack(spacing: 12) {
                 Button {
                     openFolder()
                 } label: {
                     Label("Open Folder…", systemImage: "folder")
                 }
                 .buttonStyle(.borderedProminent)
                 .tint(LoomGraphite.accent)
                 .loomActionTarget()
                 .accessibilityLabel("Open Folder")
                 .help("Open a local folder for this task")
 
                 Button {
                     Task { await store.startBlankBuilder() }
                 } label: {
                     Label("Use Agent Team", systemImage: "person.3")
                 }
                 .buttonStyle(.bordered)
                 .loomActionTarget()
                 .accessibilityLabel("Use Agent Team")
                 .help("Create or load a team for this task")
             }
             .padding(.top, 4)
         }
         .padding(16)
         .background(
             LoomGraphite.surface,
             in: RoundedRectangle(cornerRadius: LoomGraphite.cardRadius, style: .continuous)
         )
         .overlay {
             RoundedRectangle(cornerRadius: LoomGraphite.cardRadius, style: .continuous)
                 .stroke(LoomGraphite.separator, lineWidth: 1)
         }
     }
 
     private var recentWorkCard: some View {
         VStack(alignment: .leading, spacing: 8) {
             Text("Recent")
                 .font(.headline)
             if store.workspace.tasks.count > 1 {
                 ForEach(store.workspace.tasks.dropFirst()) { task in
                     recentWorkButton(task)
                 }
             } else {
                 Text("No recent work yet.")
                     .font(.callout)
                     .foregroundStyle(Color.secondary)
             }
         }
         .padding(16)
         .frame(maxWidth: .infinity, alignment: .leading)
         .background(
             LoomGraphite.surface,
             in: RoundedRectangle(cornerRadius: LoomGraphite.cardRadius, style: .continuous)
         )
         .overlay {
             RoundedRectangle(cornerRadius: LoomGraphite.cardRadius, style: .continuous)
                 .stroke(LoomGraphite.separator, lineWidth: 1)
         }
     }
 
     private func recentWorkButton(_ task: LocalProductWorkspaceTask) -> some View {
         Button {
             Task {
                 store.selectWorkspaceTask(task.id)
                 await store.activateWorkspaceTask()
             }
         } label: {
             HStack(alignment: .top, spacing: 10) {
                 Image(systemName: taskIcon(task.kind))
                     .foregroundStyle(Color.secondary)
                     .frame(width: 20)
                 VStack(alignment: .leading, spacing: 3) {
                     Text(task.title)
                         .font(.callout.weight(.semibold))
                         .lineLimit(1)
                     Text(task.subtitle)
                         .font(.caption)
                         .foregroundStyle(Color.secondary)
                         .lineLimit(2)
                 }
                 Spacer()
             }
             .frame(maxWidth: .infinity, alignment: .leading)
             .contentShape(Rectangle())
         }
         .buttonStyle(.plain)
         .accessibilityLabel("Open recent task \(task.title)")
     }
 
     private var composer: some View {
         VStack(spacing: 8) {
             HStack(alignment: .bottom, spacing: 10) {
                 TextField(
                     "Describe a task…",
                     text: Binding(
                         get: { store.workspace.selectedContinuity.composerDraft },
                         set: store.updateComposerDraft
                     ),
                     axis: .vertical
                 )
                 .textFieldStyle(.plain)
                 .lineLimit(1...5)
                 .padding(.horizontal, 12)
                 .padding(.vertical, 10)
                 .background(
                     LoomGraphite.canvas,
                     in: RoundedRectangle(cornerRadius: 12, style: .continuous)
                 )
                 .overlay {
                     RoundedRectangle(cornerRadius: 12, style: .continuous)
                         .stroke(LoomGraphite.separator, lineWidth: 1)
                 }
 
                 Button {
                     Task {
                         await store.startBlankBuilder()
                     }
                 } label: {
                     Image(systemName: "arrow.up")
                         .font(.body.weight(.semibold))
                         .foregroundStyle(LoomGraphite.onAccent)
                         .frame(width: 36, height: 36)
                         .background(
                             sendButtonEnabled ? LoomGraphite.accent : LoomGraphite.accent.opacity(0.35),
                             in: Circle()
                         )
                 }
                 .buttonStyle(.plain)
                 .disabled(!sendButtonEnabled)
                 .accessibilityLabel("Continue")
                 .help("Continue with this task")
             }
             Text("Review and confirmation are required before anything is saved.")
                 .font(.caption2)
                 .foregroundStyle(Color.secondary)
         }
     }
 
     private var sendButtonEnabled: Bool {
         !store.workspace.selectedContinuity.composerDraft
             .trimmingCharacters(in: .whitespacesAndNewlines)
             .isEmpty
     }
 
     // MARK: - Right governance panel
 
     @ViewBuilder
     private var governancePanel: some View {
         switch selectedNavigation {
         case .home:
             EmptyView()
         case .work, .teams, .attention, .library:
             MissionWorkbench(store: store, showRail: false)
         case .runtimes:
             runtimePanel
         }
     }
 
     private var runtimePanel: some View {
         VStack(spacing: 0) {
             HStack {
                 Text("Runtimes")
                     .font(.headline)
                 Spacer()
             }
             .padding(.horizontal, 16)
             .frame(height: 50)
             .background(LoomGraphite.surface)
             Divider()
             ScrollView {
                 VStack(alignment: .leading, spacing: 12) {
                     if (store.snapshot?.runtimes ?? []).isEmpty {
                         Text("No runtimes discovered yet.")
                             .foregroundStyle(Color.secondary)
                     } else {
                         ForEach(store.snapshot?.runtimes ?? []) { runtime in
                             RuntimeRow(runtime: runtime)
                         }
                     }
                 }
                 .padding(16)
             }
         }
         .background(LoomGraphite.surface)
     }
 
     private var providerManagementPlaceholder: some View {
         VStack(spacing: 16) {
             Text("Provider Management")
                 .font(.title2)
             Text("Provider connection and setup will be managed here.")
                 .foregroundStyle(Color.secondary)
             Button("Done") {
                 showProviders = false
             }
             .keyboardShortcut(.defaultAction)
         }
         .padding(24)
         .frame(minWidth: 300, minHeight: 200)
     }
 
     // MARK: - Helpers
 
     private var experience: LocalProductExperience {
         LocalProductExperience(
             snapshot: store.snapshot,
             connectionState: store.connectionState
         )
     }
 
     private func taskIcon(_ kind: LocalProductWorkspaceTaskKind) -> String {
         switch kind {
         case .draft: return "square.and.pencil"
         case .history: return "clock.arrow.circlepath"
         case .team: return "person.3"
         case .attention: return "exclamationmark.triangle"
         }
     }
 
     private func openFolder() {
         // TODO(P2C-W1): wire NSOpenPanel and scoped preflight
         // For now this is a visible affordance that does not write state.
         Task {
             await store.refresh()
         }
     }
 }
 
 // Reuse RuntimeRow from ContentView when available; otherwise provide a local
 // fallback to keep the panel self-contained.
 
 private struct RuntimeRow: View {
     let runtime: LocalProductRuntimeSummary
 
     var body: some View {
         HStack(alignment: .top, spacing: 12) {
             Image(systemName: "cpu")
                 .foregroundStyle(Color.secondary)
                 .accessibilityHidden(true)
             VStack(alignment: .leading, spacing: 4) {
                 Text(LocalProductExperience.visibleName(
                     runtime.displayName,
                     internalID: runtime.runtimeInstanceID,
                     fallback: "Runtime"
                 ))
                 .font(.headline)
                let runtimeSummary = humanized(runtime.status) + " · " +
                    humanized(runtime.adapterType) + " · " +
                    "Capacity \(runtime.capacity)"
                Text(runtimeSummary)
                    .foregroundStyle(Color.secondary)
                 if !runtime.executableVersion.isEmpty {
                     Text(
                         "Version " +
                             SafeText.sanitize(runtime.executableVersion, limit: 64)
                     )
                     .font(.caption)
                     .foregroundStyle(Color(nsColor: .tertiaryLabelColor))
                 }
             }
             Spacer()
         }
         .padding(.vertical, 8)
         .accessibilityElement(children: .combine)
     }
 
     private func humanized(
         _ value: String,
         fallback: String = "Available"
     ) -> String {
         let safe = SafeText.sanitize(value, limit: 96)
             .replacingOccurrences(of: "_", with: " ")
             .replacingOccurrences(of: "-", with: " ")
             .trimmingCharacters(in: .whitespaces)
         guard !safe.isEmpty else { return fallback }
         return safe.prefix(1).uppercased() + safe.dropFirst()
     }
 }
 
