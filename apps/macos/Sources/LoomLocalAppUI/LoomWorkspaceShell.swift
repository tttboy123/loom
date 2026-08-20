import LoomLocalAppCore
import AppKit
import Foundation
import SwiftUI
import UniformTypeIdentifiers

func conversationProfileMenuLabel(
    _ profile: LocalProductConversationProfile
) -> String {
    // The Provider layer must not carry a model; Model is a separate layer.
    [profile.displayName, profile.providerAccountID]
        .filter { !$0.isEmpty }
        .joined(separator: " · ")
}

struct ConversationRouteTransitionSheet: View {
    let transition: LocalProductConversationRouteTransition
    let onConfirm: (LocalProductConversationContextMode) -> Bool
    let onCancel: () -> Void
    @State private var contextMode: LocalProductConversationContextMode
    @State private var confirmationFailed = false

    init(
        transition: LocalProductConversationRouteTransition,
        initialContextMode: LocalProductConversationContextMode = .summaryOnly,
        onConfirm: @escaping (LocalProductConversationContextMode) -> Bool,
        onCancel: @escaping () -> Void
    ) {
        self.transition = transition
        self.onConfirm = onConfirm
        self.onCancel = onCancel
        _contextMode = State(initialValue: initialContextMode)
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            VStack(alignment: .leading, spacing: 5) {
                Text(
                    transition.rebindsCurrentRoute
                        ? "Update conversation route policy"
                        : "Change conversation route"
                )
                    .font(.title2.weight(.semibold))
                Text("Review the new execution binding and context disclosure before continuing.")
                    .foregroundStyle(.secondary)
            }
            .padding(24)

            Divider()

            ScrollView {
                VStack(alignment: .leading, spacing: 20) {
                    routeRow(
                        label: transition.rebindsCurrentRoute
                            ? "Frozen route policy"
                            : "Current route",
                        profile: transition.source,
                        executionBinding: transition.sourceExecutionBinding,
                        showsFrozenPolicy: transition.rebindsCurrentRoute
                    )

                    HStack(spacing: 8) {
                        Image(systemName: "arrow.down")
                            .foregroundStyle(.secondary)
                        Text("Create new Segment")
                            .font(.subheadline.weight(.semibold))
                    }
                    .accessibilityElement(children: .combine)

                    routeRow(
                        label: transition.rebindsCurrentRoute
                            ? "Current account policy"
                            : "New route",
                        profile: transition.target
                    )

                    Divider()

                    VStack(alignment: .leading, spacing: 10) {
                        Text("Context to share")
                            .font(.headline)
                        contextModeOption(
                            .continueWithContext,
                            title: "Continue with context",
                            detail: "Share policy-approved conversation, workspace, and artifact context."
                        )
                        contextModeOption(
                            .summaryOnly,
                            title: "Summary only",
                            detail: "Share the Goal, confirmed constraints, decisions, current state, and necessary artifacts."
                        )
                        contextModeOption(
                            .startClean,
                            title: "Start clean",
                            detail: "Share only Loom policy, Agent identity, and your next message."
                        )
                    }

                    Label(
                        "Loom applies account disclosure policy and records a Disclosure receipt. Omitted context remains visible after dispatch.",
                        systemImage: "checkmark.shield"
                    )
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)

                    Text("Provider-native session state and credentials are never reused across the new Segment.")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .fixedSize(horizontal: false, vertical: true)

                    if confirmationFailed {
                        Label(
                            "The route changed while you were reviewing. Close this sheet and choose it again.",
                            systemImage: "arrow.triangle.2.circlepath"
                        )
                        .font(.caption)
                        .foregroundStyle(LoomGraphite.statusWarning)
                        .fixedSize(horizontal: false, vertical: true)
                        .accessibilityLabel(
                            "Route changed while reviewing. Close and choose the route again."
                        )
                    }
                }
                .padding(24)
                .frame(maxWidth: .infinity, alignment: .leading)
            }

            Divider()

            HStack {
                Button("Cancel", action: onCancel)
                Spacer()
                Button(
                    transition.rebindsCurrentRoute
                        ? "Confirm new Segment"
                        : "Confirm route change"
                ) {
                    confirmationFailed = !onConfirm(contextMode)
                }
                .buttonStyle(.borderedProminent)
            }
            .padding(.horizontal, 24)
            .padding(.vertical, 16)
        }
    }

    private func routeRow(
        label: String,
        profile: LocalProductConversationProfile,
        executionBinding: LocalProductConversationExecutionBinding? = nil,
        showsFrozenPolicy: Bool = false
    ) -> some View {
        VStack(alignment: .leading, spacing: 5) {
            Text(label)
                .font(.caption.weight(.semibold))
                .foregroundStyle(.secondary)
            Text(profile.displayName)
                .font(.headline)
            Text(bindingLabel(profile))
                .font(.caption)
                .foregroundStyle(.secondary)
                .textSelection(.enabled)
            if showsFrozenPolicy {
                Text(
                    executionBinding.map(policyLabel)
                        ?? "Account policy unspecified (legacy Segment)"
                )
                    .font(.caption2)
                    .foregroundStyle(.secondary)
            } else if profile.authMode == "brokered" {
                Text(policyLabel(profile))
                    .font(.caption2)
                    .foregroundStyle(.secondary)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private func contextModeOption(
        _ mode: LocalProductConversationContextMode,
        title: String,
        detail: String
    ) -> some View {
        Button {
            contextMode = mode
            confirmationFailed = false
        } label: {
            HStack(alignment: .top, spacing: 10) {
                Image(
                    systemName: contextMode == mode
                        ? "largecircle.fill.circle"
                        : "circle"
                )
                .foregroundStyle(contextMode == mode ? LoomGraphite.accent : Color.secondary)
                VStack(alignment: .leading, spacing: 3) {
                    Text(title)
                        .foregroundStyle(.primary)
                    Text(detail)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .fixedSize(horizontal: false, vertical: true)
                }
                Spacer(minLength: 0)
            }
            .frame(minHeight: LoomGraphite.minimumActionTarget)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityLabel("\(title). \(detail)")
    }

    private func bindingLabel(_ profile: LocalProductConversationProfile) -> String {
        let account = profile.providerAccountID.isEmpty
            ? profile.providerID
            : profile.providerAccountID
        var parts = [harnessLabel(profile.harnessAdapter), account, profile.modelID]
            .filter { !$0.isEmpty }
        if profile.credentialRevision > 0 {
            parts.append("Credential r\(profile.credentialRevision)")
        }
        return parts
            .filter { !$0.isEmpty }
            .joined(separator: " · ")
    }

    private func policyLabel(
        _ binding: LocalProductConversationExecutionBinding
    ) -> String {
        guard binding.providerAccountPolicyVersion == 2 else {
            return "Account policy unspecified"
        }
        return "Policy r\(binding.providerAccountPolicyRevision) · \(displayPolicy(binding.trustDomain)) · \(displayPolicy(binding.retentionMode)) · \(displayPolicy(binding.dataRegion))"
    }

    private func policyLabel(_ profile: LocalProductConversationProfile) -> String {
        guard profile.policyVersion == 2 else {
            return "Account policy unspecified"
        }
        return "Policy r\(profile.policyRevision) · \(displayPolicy(profile.trustDomain)) · \(displayPolicy(profile.retentionMode)) · \(displayPolicy(profile.dataRegion))"
    }

    private func displayPolicy(_ value: String) -> String {
        value.split(separator: "_").map { $0.capitalized }.joined(separator: " ")
    }

    private func harnessLabel(_ harnessAdapter: String) -> String {
        switch harnessAdapter {
        case "codex": return "Codex"
        case "claude-code": return "Claude Code"
        case "loom-native": return "Loom Native"
        case "pi": return "Pi"
        default: return harnessAdapter
        }
    }
}

public enum LoomWorkspaceNavigationItem: String, CaseIterable, Identifiable, Sendable {
    case home = "Chat"
    case work = "Work"
    case teams = "Teams"
    case roundtable = "Roundtable"
    case attention = "Attention"
    case library = "Library"
    case runtimes = "Runtimes"

    public var id: String { rawValue }

    public var symbol: String {
        switch self {
        case .home: return "bubble.left.fill"
        case .work: return "square.3.layers.3d"
        case .teams: return "person.3"
        case .roundtable: return "person.2.wave.2"
        case .attention: return "exclamationmark.triangle"
        case .library: return "books.vertical"
        case .runtimes: return "cpu"
        }
    }

    public var accessibilityLabel: String {
        switch self {
        case .home: return "Conversation"
        case .work: return "Mission Board"
        case .teams: return "Agent Teams"
        case .roundtable: return "Governed handoff roundtable"
        case .attention: return "Items needing attention"
        case .library: return "Agent library"
        case .runtimes: return "Runtime health"
        }
    }
}

private enum LoomFullGovernancePresentation: String, Identifiable {
    case workbench
    case newMission
    case runtimeProviders
    case roundtable

    var id: String { rawValue }
}

func conversationVaultRecoveryAvailable(
    stage: LocalIPCRemoteError.Stage,
    recoverable: Bool
) -> Bool {
    // Vault-stage failures always offer vault recovery, even when retryable:
    // retrying without unlocking keeps failing with the same locked state.
    _ = recoverable
    return [
        .vaultKeyLoad,
        .vaultOpen,
        .vaultEncrypt,
        .vaultCommit,
        .vaultDecrypt,
        .vaultAADValidation,
        .vaultRotation,
        .vaultRecovery,
    ].contains(stage)
}

public enum LoomRecentTaskActionLabel {
    public static func make(
        for title: String,
        subtitle: String,
        position: Int
    ) -> String {
        let safeTitle = SafeText.sanitize(title, limit: 48)
            .trimmingCharacters(in: .whitespacesAndNewlines)
        let safeSubtitle = SafeText.sanitize(subtitle, limit: 32)
            .trimmingCharacters(in: .whitespacesAndNewlines)
        return (["Open recent task \(max(position, 1))", safeTitle, safeSubtitle]
            .filter { !$0.isEmpty })
            .joined(separator: ", ")
    }
}

public struct LoomWorkspaceShell: View {
    private static let chatTimelineBottomID = "loom-chat-timeline-bottom"

    @ObservedObject private var store: LocalProductStore
    @State private var selectedNavigation: LoomWorkspaceNavigationItem = .home
    @State private var governance = LoomGovernancePanelState()
    @State private var fullGovernancePresentation: LoomFullGovernancePresentation?
    @State private var showFolderImporter = false
    @State private var builderAnswer = ""
    @State private var builderName = ""
    @State private var builderPurpose = ""
    @State private var pendingMissionObjective = ""
    @State private var diagnosticPreview: LocalDiagnosticBundlePreview?
    @State private var diagnosticExporter: LocalDiagnosticBundleExporter?
    @State private var pendingConversationRouteTransition:
        LocalProductConversationRouteTransition?
    @FocusState private var composerFocused: Bool

    public init(store: LocalProductStore) {
        self.store = store
    }

    public var body: some View {
        GeometryReader { proxy in
            let metrics = LoomWorkspaceLayoutPolicy.metrics(
                for: proxy.size.width,
                panelMode: governance.mode
            )

            ZStack(alignment: .trailing) {
                HSplitView {
                    navigationRail(compact: metrics.railWidth == 64)
                        .frame(
                            minWidth: metrics.railWidth,
                            idealWidth: metrics.railWidth,
                            maxWidth: metrics.railWidth
                        )

                    centerWorkspace
                        .frame(minWidth: metrics.conversationMinimumWidth)

                    if metrics.panelPlacement == .split {
                        governanceInspector
                            .frame(
                                minWidth: 320,
                                idealWidth: metrics.panelWidth,
                                maxWidth: 420
                            )
                    }
                }

                if metrics.panelPlacement == .overlay {
                    Color.black.opacity(0.16)
                        .contentShape(Rectangle())
                        .onTapGesture { governance.close() }
                        .accessibilityHidden(true)

                    governanceInspector
                        .frame(width: metrics.panelWidth)
                        .background(LoomGraphite.surface)
                        .shadow(color: .black.opacity(0.22), radius: 18, x: -4)
                        .transition(.move(edge: .trailing).combined(with: .opacity))
                }
            }
            .animation(.easeOut(duration: 0.18), value: governance.mode)
        }
        .background(LoomGraphite.canvas)
        .fileImporter(
            isPresented: $showFolderImporter,
            allowedContentTypes: [.folder],
            allowsMultipleSelection: false,
            onCompletion: handleFolderSelection
        )
        .sheet(item: $fullGovernancePresentation) { presentation in
            if presentation == .roundtable {
                RoundtableWorkbench(store: store)
                    .frame(minWidth: 860, minHeight: 620)
            } else {
                MissionWorkbench(
                    store: store,
                    showProvidersInitially: presentation == .runtimeProviders,
                    showNewMissionInitially: presentation == .newMission,
                    initialMissionObjective: pendingMissionObjective
                )
                    .frame(minWidth: 1_080, minHeight: 680)
            }
        }
        .sheet(item: $diagnosticPreview) { preview in
            if let diagnosticExporter {
                DiagnosticBundlePreviewSheet(
                    preview: preview,
                    exporter: diagnosticExporter
                )
            }
        }
        .sheet(item: $pendingConversationRouteTransition) { transition in
            ConversationRouteTransitionSheet(
                transition: transition,
                initialContextMode: .summaryOnly,
                onConfirm: { mode in
                    let confirmed = store.confirmConversationRouteTransition(
                        transition,
                        contextMode: mode
                    )
                    if confirmed { pendingConversationRouteTransition = nil }
                    return confirmed
                },
                onCancel: { pendingConversationRouteTransition = nil }
            )
                .frame(minWidth: 560, minHeight: 520)
        }
        .onReceive(
            NotificationCenter.default.publisher(for: .loomOpenFolderRequested)
        ) { _ in
            showFolderImporter = true
        }
        .onExitCommand {
            governance.dismissIfPresented()
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel("Loom workspace")
    }

    private func navigationRail(compact: Bool) -> some View {
        VStack(alignment: .leading, spacing: 0) {
            HStack(spacing: 9) {
                Image(systemName: "square.3.layers.3d")
                    .foregroundStyle(LoomGraphite.accent)
                if !compact {
                    Text("Loom")
                        .font(.headline)
                    Spacer()
                }
            }
            .padding(.horizontal, compact ? 20 : 14)
            .frame(height: 52)
            .accessibilityElement(children: .combine)
            .accessibilityLabel("Loom")

            Divider()
                .background(LoomGraphite.separator)

            ScrollView {
                VStack(alignment: .leading, spacing: 6) {
                    Button {
                        store.showMissionBoard()
                        fullGovernancePresentation = .newMission
                    } label: {
                        HStack(spacing: 10) {
                            Image(systemName: "plus.square")
                                .frame(width: 20)
                            if !compact {
                                Text("New Mission")
                                Spacer()
                            }
                        }
                        .padding(.horizontal, 10)
                        .frame(height: 40)
                        .frame(
                            maxWidth: .infinity,
                            alignment: compact ? .center : .leading
                        )
                    }
                    .buttonStyle(.plain)
                    .accessibilityLabel("New Mission")
                    .help("New Mission")

                    ForEach(LoomWorkspaceNavigationItem.allCases) { item in
                        navigationButton(item, compact: compact)
                    }

                    railSectionTitle("CONVERSATIONS")
                    ForEach(store.chatSessions) { session in
                        conversationButton(session, compact: compact)
                    }
                    Button {
                        store.newConversation()
                    } label: {
                        HStack(spacing: 10) {
                            Image(systemName: "square.and.pencil")
                                .frame(width: 20)
                            if !compact {
                                Text("New Conversation")
                                Spacer()
                            }
                        }
                        .padding(.horizontal, 10)
                        .frame(height: 34)
                        .frame(
                            maxWidth: .infinity,
                            alignment: compact ? .center : .leading
                        )
                    }
                    .buttonStyle(.plain)
                    .accessibilityLabel("New Conversation")
                    .help("New Conversation")

                    railSectionTitle("AGENT TEAMS")
                    // Only runnable (non-archived) Teams belong in the rail;
                    // archived Teams remain visible in the Teams governance view.
                    let teams = store.snapshot?.teams.filter(\.executable) ?? []
                    if teams.isEmpty {
                        if !compact {
                            Text("No Agent Teams yet")
                                .font(.caption)
                                .foregroundStyle(.tertiary)
                                .padding(.horizontal, 10)
                                .frame(height: 28, alignment: .leading)
                        }
                    } else {
                        ForEach(teams) { team in
                            teamRailButton(team, compact: compact)
                        }
                    }
                    Button {
                        selectNavigation(.teams)
                        Task { await store.startBlankBuilder() }
                    } label: {
                        HStack(spacing: 10) {
                            Image(systemName: "plus")
                                .frame(width: 20)
                            if !compact {
                                Text("New Agent Team")
                                Spacer()
                            }
                        }
                        .padding(.horizontal, 10)
                        .frame(height: 32)
                        .frame(
                            maxWidth: .infinity,
                            alignment: compact ? .center : .leading
                        )
                    }
                    .buttonStyle(.plain)
                    .accessibilityLabel("New Agent Team")
                    .help("New Agent Team")

                    if !compact, store.workspace.tasks.count > 1 {
                        railSectionTitle("RECENT")
                        ForEach(
                            Array(
                                store.workspace.tasks.dropFirst().prefix(6).enumerated()
                            ),
                            id: \.element.id
                        ) { item in
                            recentTaskButton(item.element, position: item.offset + 1)
                        }
                    }
                }
                .padding(.horizontal, compact ? 8 : 10)
                .padding(.vertical, 12)
            }

            Spacer()

            Divider()
                .background(LoomGraphite.separator)

            Button {
                governance.open(.runtimes)
                selectedNavigation = .runtimes
            } label: {
                HStack(spacing: 10) {
                    Image(systemName: "waveform.path.ecg")
                        .frame(width: 20)
                    if !compact {
                        Text(serviceFooterTitle)
                            .font(.caption)
                            .lineLimit(1)
                        Spacer()
                    }
                }
                .padding(.horizontal, compact ? 10 : 10)
                .frame(height: 44)
                .frame(maxWidth: .infinity, alignment: compact ? .center : .leading)
            }
            .buttonStyle(.plain)
            .accessibilityLabel("Open local service and runtime health")
            .help(serviceFooterTitle)
            .padding(.horizontal, 8)
            .padding(.vertical, 6)
        }
        .background(LoomGraphite.rail)
    }

    private func navigationButton(
        _ item: LoomWorkspaceNavigationItem,
        compact: Bool
    ) -> some View {
        let selected = selectedNavigation == item
        return Button {
            selectNavigation(item)
        } label: {
            HStack(spacing: 10) {
                Image(systemName: item.symbol)
                    .frame(width: 20)
                    .foregroundStyle(selected ? LoomGraphite.accent : Color.secondary)
                if !compact {
                    Text(item.rawValue)
                        .font(.callout.weight(selected ? .semibold : .regular))
                    Spacer()
                }
            }
            .padding(.horizontal, compact ? 10 : 10)
            .frame(height: 40)
            .frame(maxWidth: .infinity, alignment: compact ? .center : .leading)
            .background(
                selected ? LoomGraphite.accentMuted : Color.clear,
                in: RoundedRectangle(cornerRadius: 7, style: .continuous)
            )
        }
        .buttonStyle(.plain)
        .accessibilityLabel(item.accessibilityLabel)
        .accessibilityAddTraits(selected ? .isSelected : [])
        .help(item.rawValue)
    }

    private func selectNavigation(_ item: LoomWorkspaceNavigationItem) {
        selectedNavigation = item
        switch item {
        case .home:
            if governance.mode != .pinned {
                governance.close()
            }
        case .work:
            store.showMissionBoard()
            governance.open(.board)
        case .teams:
            store.showMissionTeams()
            governance.open(.team)
        case .roundtable:
            fullGovernancePresentation = .roundtable
        case .attention:
            store.showMissionAttention()
            governance.open(.attention)
        case .library:
            store.showMissionLibrary()
            governance.open(.library)
        case .runtimes:
            governance.open(.runtimes)
        }
    }

    private func railSectionTitle(_ title: String) -> some View {
        Text(title)
            .font(.caption2.weight(.semibold))
            .foregroundStyle(.secondary)
            .padding(.horizontal, 10)
            .padding(.top, 16)
            .padding(.bottom, 3)
    }

    /// Sidebar titles come from the first user message, so several
    /// conversations can share the same label (e.g. "hello"). When another
    /// session has the same title, append a short date so they are
    /// distinguishable at a glance without reading the relative time.
    private func conversationButtonTitle(
        for session: LocalProductChatSession
    ) -> String {
        let sameTitle = store.chatSessions.filter {
            $0.title == session.title
        }
        guard sameTitle.count > 1 else { return session.title }
        return "\(session.title) · \(conversationDisambiguationSuffix(for: session.createdAt))"
    }

    private func conversationButton(
        _ session: LocalProductChatSession,
        compact: Bool
    ) -> some View {
        let active = session.threadID == store.selectedChatSessionID
        return Button {
            store.selectChatSession(session.threadID)
        } label: {
            HStack(spacing: 10) {
                Image(systemName: active ? "bubble.left.fill" : "bubble.left")
                    .foregroundStyle(
                        active ? LoomGraphite.accent : Color.secondary
                    )
                    .frame(width: 20)
                if !compact {
                    VStack(alignment: .leading, spacing: 1) {
                        Text(conversationButtonTitle(for: session))
                            .font(.caption)
                            .foregroundStyle(active ? Color.primary : Color.secondary)
                            .lineLimit(1)
                        Text(Self.conversationRelativeTime(session.updatedAt))
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                            .lineLimit(1)
                    }
                    Spacer()
                    if active {
                        Image(systemName: "checkmark")
                            .font(.caption2)
                            .foregroundStyle(LoomGraphite.accent)
                    }
                }
            }
            .padding(.horizontal, 10)
            .frame(height: 38)
            .frame(maxWidth: .infinity, alignment: compact ? .center : .leading)
            .background(
                active ? LoomGraphite.accent.opacity(0.12) : Color.clear
            )
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityLabel("Conversation \(session.title)")
        .contextMenu {
            Button(role: .destructive) {
                Task { await store.deleteChatSession(session.threadID) }
            } label: {
                Label("Delete Conversation", systemImage: "trash")
            }
        }
    }

    private func teamRailButton(
        _ team: LocalProductTeamSummary,
        compact: Bool
    ) -> some View {
        let active = store.selectedTeamID == team.teamInstanceID
        return Button {
            store.selectTeam(team)
            governance.open(.team)
            selectedNavigation = .teams
        } label: {
            HStack(spacing: 10) {
                Image(systemName: "person.2")
                    .frame(width: 20)
                    .foregroundStyle(
                        active ? LoomGraphite.accent : Color.secondary
                    )
                if !compact {
                    Text(
                        LocalProductExperience.visibleName(
                            team.displayName,
                            internalID: team.teamInstanceID,
                            fallback: "Agent Team"
                        )
                    )
                    .font(.caption)
                    .foregroundStyle(active ? Color.primary : Color.secondary)
                    .lineLimit(1)
                    Spacer()
                }
            }
            .padding(.horizontal, 10)
            .frame(height: 34)
            .frame(maxWidth: .infinity, alignment: compact ? .center : .leading)
            .background(
                active ? LoomGraphite.accentMuted : Color.clear,
                in: RoundedRectangle(cornerRadius: 7, style: .continuous)
            )
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityLabel("Open Agent Team \(team.displayName)")
        .help(team.displayName)
    }

    private static func conversationRelativeTime(_ date: Date) -> String {
        let elapsed = Date().timeIntervalSince(date)
        let formatter = RelativeDateTimeFormatter()
        formatter.unitsStyle = .abbreviated
        if elapsed < 60 {
            return "just now"
        }
        return formatter.localizedString(for: date, relativeTo: Date())
    }

    private func recentTaskButton(
        _ task: LocalProductWorkspaceTask,
        position: Int
    ) -> some View {
        let actionLabel = LoomRecentTaskActionLabel.make(
            for: task.title,
            subtitle: task.subtitle,
            position: position
        )
        return Button {
            Task {
                store.selectWorkspaceTask(task.id)
                await store.activateWorkspaceTask()
            }
        } label: {
            VStack(alignment: .leading, spacing: 2) {
                Text(task.title)
                    .font(.callout)
                    .lineLimit(1)
                Text(task.subtitle)
                    .font(.caption2)
                    .foregroundStyle(.secondary)
                    .lineLimit(1)
            }
            .padding(.horizontal, 10)
            .frame(height: 42)
            .frame(maxWidth: .infinity, alignment: .leading)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityLabel(actionLabel)
        .help(actionLabel)
    }

    private var centerWorkspace: some View {
        VStack(spacing: 0) {
            centerHeader
                .padding(.horizontal, 18)
                .background(LoomGraphite.surface)

            connectionStripIfNeeded

            ScrollViewReader { timeline in
                ScrollView {
                    VStack(spacing: 0) {
                        Group {
                            if store.chatThread?.messages.isEmpty ?? true {
                                emptyConversation
                            } else {
                                chatTimeline
                            }
                        }
                        .frame(maxWidth: 760, alignment: .leading)
                        .padding(.horizontal, 24)
                        .padding(.vertical, 28)
                        .frame(maxWidth: .infinity, minHeight: 360)

                        Color.clear
                            .frame(height: 1)
                            .id(Self.chatTimelineBottomID)
                    }
                }
                .onAppear {
                    timeline.scrollTo(Self.chatTimelineBottomID, anchor: .bottom)
                }
                .onChange(of: store.chatThread?.messages.last?.messageID) { _, _ in
                    withAnimation(.easeOut(duration: 0.2)) {
                        timeline.scrollTo(Self.chatTimelineBottomID, anchor: .bottom)
                    }
                }
            }

            composer
                .padding(.horizontal, 18)
                .padding(.bottom, 14)
                .background(LoomGraphite.surface)
        }
        .background(LoomGraphite.raised)
        .task { await store.loadChatThread() }
    }

    private var centerHeader: some View {
        HStack(spacing: 10) {
            VStack(alignment: .leading, spacing: 2) {
                HStack(spacing: 6) {
                    Menu {
                        ForEach(store.chatSessions) { session in
                            Button {
                                store.selectChatSession(session.threadID)
                            } label: {
                                if session.threadID
                                  == store.selectedChatSessionID
                                {
                                    Label(session.title, systemImage: "checkmark")
                                } else {
                                    Text(session.title)
                                }
                            }
                        }
                        Divider()
                        Button {
                            store.newConversation()
                        } label: {
                            Label("New Conversation", systemImage: "square.and.pencil")
                        }
                    } label: {
                        HStack(spacing: 6) {
                            Text(
                                store.selectedChatSession?.title
                                  ?? "Conversation"
                            )
                            .font(.headline)
                            .lineLimit(1)
                            Image(systemName: "chevron.down")
                                .font(.caption2)
                                .foregroundStyle(.secondary)
                        }
                        .contentShape(Rectangle())
                    }
                    .menuStyle(.borderlessButton)
                    .fixedSize()
                    .accessibilityLabel("Switch conversation")

                    Button {
                        copyConversationTranscript()
                    } label: {
                        Image(systemName: "doc.on.doc")
                            .frame(width: 22, height: 22)
                    }
                    .buttonStyle(.plain)
                    .help("Copy the whole conversation")
                    .accessibilityLabel("Copy conversation")
                    .disabled(store.chatThread?.messages.isEmpty ?? true)

                    Button {
                        store.newConversation()
                    } label: {
                        Image(systemName: "square.and.pencil")
                            .frame(width: 22, height: 22)
                    }
                    .buttonStyle(.plain)
                    .help("New Conversation")
                    .accessibilityLabel("New Conversation")
                }
                if let folder = store.workspace.selectedFolderDisplayName {
                    Label(folder, systemImage: "folder")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .lineLimit(1)
                } else {
                    Text(store.workspace.selectedTask.title)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .lineLimit(1)
                }
            }
            Spacer()
            Button {
                if governance.mode == .hidden {
                    governance.open(.overview)
                } else {
                    governance.close()
                }
            } label: {
                Image(systemName: "sidebar.trailing")
                    .frame(width: 36, height: 36)
            }
            .buttonStyle(.plain)
            .accessibilityLabel(
                governance.mode == .hidden
                    ? "Open governance inspector"
                    : "Close governance inspector"
            )
            .help("Governance inspector")
        }
        .frame(height: 52)
    }

    @ViewBuilder
    private var connectionStripIfNeeded: some View {
        if experience.state != .connectedEmpty,
           experience.state != .connectedPopulated {
            HStack(spacing: 10) {
                Image(systemName: experience.connection.systemImage)
                    .foregroundStyle(indicatorColor)
                    .accessibilityHidden(true)
                VStack(alignment: .leading, spacing: 1) {
                    Text(experience.connection.title)
                        .font(.callout.weight(.medium))
                    Text(experience.connection.detail)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                Spacer()
                if experience.connection.offersRefresh {
                    Button("Try Again") {
                        Task { await store.refresh() }
                    }
                    .help("Try Again")
                }
            }
            .padding(.horizontal, 18)
            .padding(.vertical, 9)
            .background(stripBackground)
        }
    }

    /// First-run quick start: show the current conversation Provider status and
    /// the 3-step path so a new user knows what to do next without reading docs.
    @ViewBuilder
    private var quickStartGuide: some View {
        let profile = store.selectedConversationProfile
        let providerReady = profile != nil
        let folderReady = store.workspace.selectedFolderDisplayName != nil
        let teamReady = !(store.snapshot?.teams.isEmpty ?? true)

        VStack(alignment: .leading, spacing: 10) {
            if providerReady, let profile {
                Label(
                    "Chat is ready with \(conversationProfileMenuLabel(profile))",
                    systemImage: "checkmark.circle.fill"
                )
                .foregroundStyle(LoomGraphite.statusSuccess)
            } else {
                Label(
                    "Connect a Provider (Runtime & Providers) to start chatting",
                    systemImage: "exclamationmark.triangle"
                )
                .foregroundStyle(LoomGraphite.statusWarning)
            }
            Label(
                folderReady ? "Folder: \(store.workspace.selectedFolderDisplayName ?? "")" : "1. Open Folder so work has a home",
                systemImage: folderReady ? "checkmark.circle.fill" : "folder"
            )
            .foregroundStyle(folderReady ? LoomGraphite.statusSuccess : Color.secondary)
            Label(
                teamReady ? "Agent Team ready" : "2. Create an Agent Team when the work needs governed execution",
                systemImage: teamReady ? "checkmark.circle.fill" : "person.3"
            )
            .foregroundStyle(teamReady ? LoomGraphite.statusSuccess : Color.secondary)
        }
        .font(.callout)
        .padding(12)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(
            LoomGraphite.surface,
            in: RoundedRectangle(cornerRadius: 10, style: .continuous)
        )
        .accessibilityElement(children: .combine)
        .accessibilityLabel("Quick start: \(providerReady ? "chat ready" : "connect a provider"), \(folderReady ? "folder ready" : "open a folder"), \(teamReady ? "team ready" : "create a team when needed")")
    }

    private var emptyConversation: some View {
        let teamReady = !(store.snapshot?.teams.isEmpty ?? true)
        return VStack(alignment: .leading, spacing: 14) {
            Image(systemName: "bubble.left.and.text.bubble.right")
                .font(.system(size: 26, weight: .regular))
                .foregroundStyle(LoomGraphite.accent)
                .accessibilityHidden(true)
            Text("What are we working on?")
                .font(.title2.weight(.semibold))
            Text("Describe the outcome in your own words. Bring in an Agent Team only when the work needs governed execution.")
                .foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)

            quickStartGuide

            if hasGovernanceActivity {
                HStack(spacing: 14) {
                    Label("\(activeMissionCount) active", systemImage: "bolt")
                    Label(
                        "\(store.snapshot?.attention.count ?? 0) need you",
                        systemImage: "exclamationmark.bubble"
                    )
                }
                .font(.caption)
                .foregroundStyle(.secondary)
                .accessibilityElement(children: .combine)
                .accessibilityLabel(
                    "\(activeMissionCount) active missions, \(store.snapshot?.attention.count ?? 0) items need you"
                )
            }
            HStack(spacing: 10) {
                Button {
                    showFolderImporter = true
                } label: {
                    Label("Open Folder...", systemImage: "folder")
                }
                .buttonStyle(.bordered)
                .accessibilityLabel("Open Folder")
                .help("Open Folder")

                // With a confirmed Team the primary action is to run a
                // Mission; "Use Agent Team" would start another builder and
                // dead-end the user who already has a Team.
                Button {
                    if teamReady {
                        fullGovernancePresentation = .newMission
                    } else {
                        governance.open(.team)
                        Task { await store.startBlankBuilder() }
                    }
                } label: {
                    Label(
                        teamReady ? "Start Mission" : "Use Agent Team",
                        systemImage: teamReady
                            ? "flag.checkered"
                            : "person.3"
                    )
                }
                .buttonStyle(.borderedProminent)
                .accessibilityLabel(teamReady ? "Start Mission" : "Use Agent Team")
                .help(teamReady ? "Start a governed Mission with your Team" : "Use Agent Team")
            }
        }
        .frame(maxWidth: 560, alignment: .leading)
        .padding(.top, 48)
    }

    private var chatTimeline: some View {
        // A non-lazy VStack keeps every message in one selectable region so a
        // user can drag across the whole conversation and copy several
        // messages together (LazyVStack splits rows into separate hosting
        // views, which breaks cross-message text selection on macOS).
        VStack(alignment: .leading, spacing: 18) {
            ForEach(store.chatThread?.messages ?? [], id: \.messageID) { message in
                let isAgentProposal = message.role == "proposal"
                HStack(alignment: .top, spacing: 10) {
                    Image(systemName: message.role == "user" ? "person.crop.circle" : "sparkles")
                        .foregroundStyle(
                            message.role == "user" ? LoomGraphite.accent : Color.secondary
                        )
                        .frame(width: 22)
                        .accessibilityHidden(true)
                    VStack(alignment: .leading, spacing: 5) {
                        Text(message.role == "user" ? "You" : isAgentProposal ? "Loom proposal" : "Loom")
                            .font(.caption.weight(.semibold))
                            .foregroundStyle(
                                message.role == "user"
                                    ? LoomGraphite.accent
                                    : isAgentProposal ? Color.secondary : Color.primary
                            )
                        if let route = conversationRouteLabel(for: message) {
                            Label(route, systemImage: "point.3.connected.trianglepath.dotted")
                                .font(.caption2)
                                .foregroundStyle(.tertiary)
                                .lineLimit(2)
                        }
                        if let disclosure = conversationDisclosureSummary(for: message) {
                            DisclosureGroup {
                                LabeledContent(
                                    "Shared context",
                                    value: "\(disclosure.disclosedContextCount) items"
                                )
                                LabeledContent(
                                    "Omitted context",
                                    value: "\(disclosure.omittedContextCount) items"
                                )
                                VStack(alignment: .leading, spacing: 3) {
                                    Text("Disclosure receipt")
                                        .foregroundStyle(.secondary)
                                    Text(disclosure.disclosureReceiptDigest)
                                        .font(.system(.caption2, design: .monospaced))
                                        .textSelection(.enabled)
                                        .lineLimit(2)
                                }
                                if let binding = disclosure.executionBinding {
                                    Divider()
                                    LabeledContent(
                                        "Provider account",
                                        value: binding.providerAccountID.isEmpty
                                            ? binding.providerID
                                            : binding.providerAccountID
                                    )
                                    LabeledContent(
                                        "Account policy",
                                        value: binding.providerAccountPolicyVersion == 0
                                            ? "Unspecified"
                                            : "v\(binding.providerAccountPolicyVersion) · r\(binding.providerAccountPolicyRevision)"
                                    )
                                    if binding.providerAccountPolicyVersion == 2 {
                                        LabeledContent(
                                            "Trust domain",
                                            value: conversationPolicyLabel(binding.trustDomain)
                                        )
                                        LabeledContent(
                                            "Retention",
                                            value: conversationPolicyLabel(binding.retentionMode)
                                        )
                                        LabeledContent(
                                            "Data region",
                                            value: conversationPolicyLabel(binding.dataRegion)
                                        )
                                    }
                                }
                            } label: {
                                Label(
                                    "Context shared \(disclosure.disclosedContextCount) · \(disclosure.omittedContextCount) omitted",
                                    systemImage: "doc.text.magnifyingglass"
                                )
                            }
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                        }
                        HStack(alignment: .top, spacing: 6) {
                            Text(message.displayContent)
                                .font(.body)
                                .foregroundStyle(isAgentProposal ? Color.secondary : Color.primary)
                                .textSelection(.enabled)
                                .fixedSize(horizontal: false, vertical: true)
                                .contextMenu {
                                    Button {
                                        copyChatMessage(message.displayContent)
                                    } label: {
                                        Label("Copy", systemImage: "doc.on.doc")
                                    }
                                }
                            Button {
                                copyChatMessage(message.displayContent)
                            } label: {
                                Image(systemName: "doc.on.doc")
                                    .font(.caption2)
                                    .foregroundStyle(.tertiary)
                            }
                            .buttonStyle(.plain)
                            .help("Copy message text")
                        }
                        if isAgentProposal {
                            if !store.executableTeams.isEmpty {
                                Button {
                                    runMissionFromChat(message)
                                } label: {
                                    Label("Run as Mission", systemImage: "play.circle")
                                }
                                .buttonStyle(.borderedProminent)
                                .controlSize(.small)
                                .accessibilityLabel("Run this conversation as a Mission")
                                .help("Pre-fill a Mission from this conversation and start it")
                            }
                            Button {
                                governance.open(.team)
                                Task { await store.startBlankBuilder() }
                            } label: {
                                Label("Create Agent Team", systemImage: "person.3.badge.plus")
                            }
                            .buttonStyle(.bordered)
                            .controlSize(.small)
                            .accessibilityLabel("Create Agent Team")
                            .help("Start a governed Agent Team to execute this work")
                        }
                    }
                    Spacer(minLength: 0)
                }
                .accessibilityElement(children: .combine)
                .accessibilityLabel(
                    message.role == "user"
                        ? "You: \(message.content)"
                        : isAgentProposal
                            ? "Loom proposal, untrusted: \(message.displayContent)"
                            : "Loom: \(message.displayContent)"
                )
            }
            if store.isSendingChatMessage {
                HStack(alignment: .top, spacing: 10) {
                    Image(systemName: "sparkles")
                        .foregroundStyle(.secondary)
                        .frame(width: 22)
                    VStack(alignment: .leading, spacing: 7) {
                        Text("Loom")
                            .font(.caption.weight(.semibold))
                            .foregroundStyle(.secondary)
                        ProgressView()
                            .controlSize(.small)
                            .accessibilityLabel("Loom is responding")
                    }
                    Spacer(minLength: 0)
                }
            }
        }
        .textSelection(.enabled)
    }

    private var composer: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 8) {
                Button {
                    showFolderImporter = true
                } label: {
                    Image(systemName: "folder")
                        .frame(width: 28, height: 28)
                }
                .buttonStyle(.plain)
                .accessibilityLabel("Choose conversation folder")
                .help("Choose folder")

                Button {
                    governance.open(.team)
                    Task { await store.startBlankBuilder() }
                } label: {
                    Image(systemName: "person.3")
                        .frame(width: 28, height: 28)
                }
                .buttonStyle(.plain)
                .accessibilityLabel("Use Agent Team")
                .help("Use Agent Team")

                Button {
                    store.showMissionBoard()
                    fullGovernancePresentation = .newMission
                } label: {
                    Image(systemName: "plus.square")
                        .frame(width: 28, height: 28)
                }
                .buttonStyle(.plain)
                .accessibilityLabel("New Mission")
                .help("New Mission")

                if let folder = store.workspace.selectedFolderDisplayName {
                    Text(folder)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .lineLimit(1)
                }
                Spacer()

                if !store.availableConversationProfiles.isEmpty {
                    conversationSelectionControls
                }
            }

            if let notice = store.conversationModelSelectionNotice {
                HStack(alignment: .top, spacing: 6) {
                    Image(systemName: "exclamationmark.triangle")
                        .font(.caption)
                        .foregroundStyle(.orange)
                    Text(notice)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .fixedSize(horizontal: false, vertical: true)
                        .textSelection(.enabled)
                    Spacer()
                }
                .padding(.horizontal, 2)
                .padding(.bottom, 2)
                .accessibilityLabel("Model selection notice")
            }

            HStack(alignment: .bottom, spacing: 10) {
                TextField(
                    "Ask Loom or describe a task",
                    text: Binding(
                        get: { store.workspace.selectedContinuity.composerDraft },
                        set: store.updateComposerDraft
                    ),
                    axis: .vertical
                )
                .textFieldStyle(.plain)
                .focused($composerFocused)
                .lineLimit(1...6)
                .padding(.horizontal, 12)
                .padding(.vertical, 10)
                .background(
                    LoomGraphite.canvas,
                    in: RoundedRectangle(cornerRadius: 8, style: .continuous)
                )
                .overlay {
                    RoundedRectangle(cornerRadius: 8, style: .continuous)
                        .stroke(LoomGraphite.separator, lineWidth: 1)
                }
                .onAppear { composerFocused = true }
                .onSubmit {
                    sendConversationDraft()
                }

                Button {
                    sendConversationDraft()
                } label: {
                    Image(systemName: "arrow.up")
                        .font(.body.weight(.semibold))
                        .foregroundStyle(LoomGraphite.onAccent)
                        .frame(width: 40, height: 40)
                        .background(
                            sendButtonEnabled
                                ? LoomGraphite.accent
                                : LoomGraphite.accent.opacity(0.35),
                            in: Circle()
                        )
                }
                .buttonStyle(.plain)
                .disabled(!sendButtonEnabled)
                .accessibilityLabel("Send message")
                .help("Send message")
            }

            if let failure = store.chatOperationFailure {
                chatFailureBanner(failure)
            }

            Text("Agent work requires review and confirmation.")
                .font(.caption2)
                .foregroundStyle(.secondary)
        }
    }

    private func requestConversationProfileSelection(_ profileID: String) {
        pendingConversationRouteTransition =
            store.requestConversationProfileSelection(profileID)
    }

    private func uniqueConversationProviders()
        -> [LocalProductConversationProfile]
    {
        var seen = Set<String>()
        return store.availableConversationProfiles.filter {
            seen.insert($0.providerID).inserted
        }
    }

    @ViewBuilder
    private var conversationSelectionControls: some View {
        let selectedProfile = store.selectedConversationProfile
        let providerID = selectedProfile?.providerID ?? ""
        let models = localProductConversationModels(providerID: providerID)
        let effectiveModelID = store.effectiveConversationModelID
        let effectiveReasoning = store.effectiveConversationReasoningEffort
        let reasoningEfforts = localProductConversationReasoningEfforts(
            providerID: providerID,
            modelID: effectiveModelID
        )

        Menu {
            ForEach(uniqueConversationProviders()) { profile in
                Button {
                    requestConversationProfileSelection(profile.profileID)
                } label: {
                    Label(
                        conversationProfileMenuLabel(profile),
                        systemImage: profile.profileID == store.selectedConversationProfileID
                            ? "checkmark"
                            : "circle"
                    )
                }
            }
        } label: {
            Label(
                selectedProfile.map(conversationProfileMenuLabel) ?? "Provider",
                systemImage: "globe"
            )
        }
        .menuStyle(.borderlessButton)
        .fixedSize()
        .accessibilityLabel("Choose Provider")
        .help("Provider depends on the configured account")

        if !models.isEmpty {
            Menu {
                ForEach(models) { model in
                    let available = store.isConversationModelAvailable(
                        providerID: providerID,
                        modelID: model.modelID
                    )
                    Button {
                        // Keep unavailable models clickable so the user learns
                        // exactly why the model cannot run (the store records a
                        // selection notice with the owning Provider instead of
                        // silently doing nothing).
                        store.selectConversationModel(model.modelID)
                    } label: {
                        Label(
                            model.displayName,
                            systemImage: model.modelID == effectiveModelID
                                ? "checkmark"
                                : "circle"
                        )
                        .foregroundStyle(available ? Color.primary : Color.secondary)
                    }
                    .help(
                        available
                            ? "Model depends on the Provider"
                            : (store.conversationModelUnavailableReason(
                                providerID: providerID,
                                modelID: model.modelID
                              ) ?? "Model unavailable")
                    )
                }
                if models.contains(where: { model in
                    !store.isConversationModelAvailable(
                        providerID: providerID,
                        modelID: model.modelID
                    )
                }) {
                    Divider()
                    Text(
                        "Models whose Provider credential is not verified are shown dimmed. Choose one to see why it cannot run."
                    )
                    .font(.caption2)
                    .foregroundStyle(.secondary)
                }
            } label: {
                Label(
                    conversationModelDisplayName(
                        providerID: providerID,
                        modelID: effectiveModelID
                    ),
                    systemImage: "cpu"
                )
            }
            .menuStyle(.borderlessButton)
            .fixedSize()
            .accessibilityLabel("Choose Model")
            .help("Model depends on the Provider")
        }

        if !reasoningEfforts.isEmpty {
            Menu {
                Button {
                    store.selectConversationReasoningEffort("")
                } label: {
                    Label(
                        "Provider default",
                        systemImage: effectiveReasoning.isEmpty
                            ? "checkmark"
                            : "circle"
                    )
                }
                Divider()
                ForEach(reasoningEfforts, id: \.self) { effort in
                    Button {
                        store.selectConversationReasoningEffort(effort)
                    } label: {
                        Label(
                            conversationReasoningEffortLabel(effort),
                            systemImage: effort == effectiveReasoning
                                ? "checkmark"
                                : "circle"
                        )
                    }
                }
            } label: {
                Label(
                    effectiveReasoning.isEmpty
                        ? "Reasoning"
                        : conversationReasoningEffortLabel(effectiveReasoning),
                    systemImage: "brain"
                )
            }
            .menuStyle(.borderlessButton)
            .fixedSize()
            .accessibilityLabel("Choose reasoning effort")
            .help("Reasoning effort depends on the Model")
        }
    }

    private func conversationModelDisplayName(
        providerID: String,
        modelID: String
    ) -> String {
        if let model = localProductConversationModels(providerID: providerID)
            .first(where: { $0.modelID == modelID })
        {
            return model.displayName
        }
        return modelID.isEmpty ? "Model" : modelID
    }

    private func conversationReasoningEffortLabel(_ effort: String) -> String {
        switch effort {
        case "none": return "None"
        case "minimal": return "Minimal"
        case "low": return "Low"
        case "medium": return "Medium"
        case "high": return "High"
        case "xhigh": return "Extra high"
        case "max": return "Max"
        case "ultra": return "Ultra"
        default: return effort.capitalized
        }
    }

    private func runMissionFromChat(_ message: LocalProductChatMessage) {
        let objective = chatMissionObjectiveText(
            around: message,
            thread: store.chatThread
        )
        pendingMissionObjective = objective
        store.showMissionBoard()
        fullGovernancePresentation = .newMission
    }

    /// Builds a Mission objective from the conversation around a proposal:
    /// the proposal's own content first, then the latest user message.
    private func chatMissionObjective(
        around message: LocalProductChatMessage,
        thread: LocalProductChatThread?
    ) -> String {
        chatMissionObjectiveText(around: message, thread: thread)
    }

    private func copyChatMessage(_ text: String) {
        let pasteboard = NSPasteboard.general
        pasteboard.clearContents()
        pasteboard.setString(text, forType: .string)
    }

    /// Copies the whole visible conversation as a readable transcript so the
    /// user never has to copy messages one by one.
    private func copyConversationTranscript() {
        guard let thread = store.chatThread else { return }
        let text = conversationTranscriptText(thread.messages)
        guard !text.isEmpty else { return }
        let pasteboard = NSPasteboard.general
        pasteboard.clearContents()
        pasteboard.setString(text, forType: .string)
    }

    private func sendConversationDraft() {
        if let transition = store.requestConversationDispatchTransition() {
            pendingConversationRouteTransition = transition
            return
        }
        Task {
            await store.sendChatMessage(
                store.workspace.selectedContinuity.composerDraft
            )
        }
    }


    private func conversationRouteLabel(
        for message: LocalProductChatMessage
    ) -> String? {
        guard !message.segmentID.isEmpty,
              let segment = store.chatThread?.segments.first(where: {
                  $0.segmentID == message.segmentID
              }),
              let profile = store.availableConversationProfiles.first(where: {
                  $0.profileID == segment.profileID
              }) else {
            return nil
        }
        let harness: String
        switch profile.harnessAdapter {
        case "codex": harness = "Codex"
        case "claude-code": harness = "Claude Code"
        case "loom-native": harness = "Loom Native"
        case "pi": harness = "Pi"
        default: harness = profile.harnessAdapter
        }
        let account = profile.providerAccountID.isEmpty
            ? profile.providerID
            : profile.providerAccountID
        let actualModel = conversationAttemptModelLabel(for: message)
        // Prefer the model and reasoning effort actually used for this reply
        // (per-attempt) over the profile default so the label never shows a
        // stale "aligned" model after a mid-conversation switch.
        let model = actualModel ?? profile.modelID
        return [harness, account, model]
            .filter { !$0.isEmpty }
            .joined(separator: " · ")
    }

    /// Returns "model · reasoning-effort" for the attempt that produced this
    /// reply, matching each reply to the user turn it answers (one attempt is
    /// recorded per dispatched user message within the same segment).
    private func conversationAttemptModelLabel(
        for message: LocalProductChatMessage
    ) -> String? {
        guard message.role != "user",
              let thread = store.chatThread else {
            return nil
        }
        var userTurnsBefore = 0
        var foundSelf = false
        for candidate in thread.messages {
            if candidate.segmentID != message.segmentID {
                continue
            }
            if candidate.messageID == message.messageID {
                foundSelf = true
                break
            }
            if candidate.role == "user" {
                userTurnsBefore += 1
            }
        }
        guard foundSelf, userTurnsBefore > 0,
              userTurnsBefore <= thread.attempts.count,
              thread.attempts[userTurnsBefore - 1].segmentID == message.segmentID
        else {
            return nil
        }
        let attempt = thread.attempts[userTurnsBefore - 1]
        let model = (attempt.modelID ?? "").trimmingCharacters(in: .whitespaces)
        let effort = (attempt.reasoningEffort ?? "").trimmingCharacters(in: .whitespaces)
        if model.isEmpty && effort.isEmpty {
            return nil
        }
        return [model, effort]
            .filter { !$0.isEmpty }
            .joined(separator: " · ")
    }

    private func conversationDisclosureSummary(
        for message: LocalProductChatMessage
    ) -> LocalProductConversationSegment? {
        guard !message.segmentID.isEmpty,
              let thread = store.chatThread,
              thread.messages.first(where: {
                  $0.segmentID == message.segmentID
              })?.messageID == message.messageID,
              let segment = thread.segments.first(where: {
                  $0.segmentID == message.segmentID
              }),
              !segment.disclosureReceiptDigest.isEmpty else {
            return nil
        }
        return segment
    }

    private func conversationPolicyLabel(_ value: String) -> String {
        value.split(separator: "_")
            .map { $0.capitalized }
            .joined(separator: " ")
    }

    private func chatFailureBanner(
        _ failure: LocalProductChatOperationFailure
    ) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(alignment: .firstTextBaseline, spacing: 8) {
                Image(systemName: "exclamationmark.triangle.fill")
                    .foregroundStyle(LoomGraphite.statusDanger)
                Text(failure.title)
                    .font(.callout.weight(.semibold))
                Spacer()
                Text(chatStageLabel(failure.stage))
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            Text(failure.detail)
                .font(.caption)
                .foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)
            Text("Incident \(failure.incidentID)")
                .font(.caption2.monospaced())
                .foregroundStyle(.secondary)
                .textSelection(.enabled)
            HStack(spacing: 8) {
                if failure.recoverable {
                    Button {
                        sendConversationDraft()
                    } label: {
                        Label("Retry", systemImage: "arrow.clockwise")
                    }
                    .disabled(store.isSendingChatMessage)
                }
                if conversationVaultRecoveryAvailable(
                    stage: failure.stage,
                    recoverable: failure.recoverable
                ) {
                    Button {
                        Task {
                            await store.setCredentialVaultLocked(false)
                            await store.loadChatThread()
                        }
                    } label: {
                        Label("Unlock Vault", systemImage: "lock.open")
                    }
                    .disabled(store.isUpdatingCredentialVaultLock)
                    Button {
                        fullGovernancePresentation = .runtimeProviders
                    } label: {
                        Label("Open Credential Vault", systemImage: "key")
                    }
                }
                if failure.isRouteRecoveryAvailable {
                    Button {
                        fullGovernancePresentation = .runtimeProviders
                    } label: {
                        Label("Switch Provider", systemImage: "arrow.triangle.swap")
                    }
                }
                Button {
                    prepareChatDiagnosticPreview()
                } label: {
                    Label(
                        "View diagnostics",
                        systemImage: "doc.text.magnifyingglass"
                    )
                }
                Spacer()
                Button {
                    NSPasteboard.general.clearContents()
                    NSPasteboard.general.setString(
                        failure.incidentID,
                        forType: .string
                    )
                } label: {
                    Image(systemName: "doc.on.doc")
                }
                .buttonStyle(.plain)
                .accessibilityLabel("Copy incident ID")
                .help("Copy incident ID")
            }
            .controlSize(.small)
        }
        .padding(10)
        .background(LoomGraphite.statusDanger.opacity(0.08))
        .overlay(alignment: .leading) {
            Rectangle()
                .fill(LoomGraphite.statusDanger)
                .frame(width: 2)
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel(
            "\(failure.title). \(failure.detail). Incident \(failure.incidentID)"
        )
    }

    private func chatStageLabel(
        _ stage: LocalIPCRemoteError.Stage
    ) -> String {
        switch stage {
        case .inputAdmission: return "Input admission"
        case .udsTransport: return "Local service transport"
        case .conversationDispatch: return "Conversation dispatch"
        default: return stage.rawValue.replacingOccurrences(
            of: "_",
            with: " "
        ).capitalized
        }
    }

    private func prepareChatDiagnosticPreview() {
        let input = LocalDiagnosticBundleInput(
            setupSnapshot: store.setupSnapshot,
            board: store.timeline?.board
        )
        Task {
            do {
                let exporter = try LocalDiagnosticBundleExporter.installed()
                let preview = try await Task.detached {
                    try exporter.preview(input: input)
                }.value
                diagnosticExporter = exporter
                diagnosticPreview = preview
            } catch {
                fullGovernancePresentation = .runtimeProviders
            }
        }
    }

    private var governanceInspector: some View {
        VStack(spacing: 0) {
            governanceHeader
            Divider()
            ScrollView {
                governanceContent
                    .padding(16)
            }
            if governance.destination == .team, store.builderSession != nil {
                Divider()
                teamBuilderConfirmFooter
            }
        }
        .background(LoomGraphite.surface)
        .accessibilityElement(children: .contain)
        .accessibilityLabel("\(governance.destination.rawValue) governance inspector")
    }

    private var governanceHeader: some View {
        HStack(spacing: 8) {
            Menu {
                ForEach(LoomGovernanceDestination.allCases) { destination in
                    Button {
                        governance.open(destination)
                    } label: {
                        Label(destination.rawValue, systemImage: governanceSymbol(destination))
                    }
                }
            } label: {
                Label(
                    governance.destination.rawValue,
                    systemImage: governanceSymbol(governance.destination)
                )
                .font(.headline)
            }
            .menuStyle(.borderlessButton)
            .accessibilityLabel("Choose governance view")

            Spacer()

            Button {
                governance.togglePin()
            } label: {
                Image(systemName: governance.mode == .pinned ? "pin.fill" : "pin")
                    .frame(width: 32, height: 32)
            }
            .buttonStyle(.plain)
            .accessibilityLabel(governance.mode == .pinned ? "Unpin inspector" : "Pin inspector")
            .help(governance.mode == .pinned ? "Unpin inspector" : "Pin inspector")

            Button {
                governance.close()
            } label: {
                Image(systemName: "xmark")
                    .frame(width: 32, height: 32)
            }
            .buttonStyle(.plain)
            .accessibilityLabel("Close governance inspector")
            .help("Close")
        }
        .padding(.horizontal, 14)
        .frame(height: 52)
    }

    @ViewBuilder
    private var governanceContent: some View {
        switch governance.destination {
        case .overview:
            overviewPanel
        case .board:
            boardPanel
        case .team:
            teamPanel
        case .topology:
            topologyPanel
        case .timeline:
            timelinePanel
        case .decisions:
            decisionsPanel
        case .evidence:
            evidencePanel
        case .runtimes:
            runtimePanel
        case .attention:
            attentionPanel
        case .library:
            libraryPanel
        }
    }

    private var overviewPanel: some View {
        VStack(alignment: .leading, spacing: 0) {
            metricRow("Active missions", value: "\(activeMissionCount)", symbol: "bolt")
            metricRow("Needs you", value: "\(store.snapshot?.attention.count ?? 0)", symbol: "exclamationmark.bubble")
            metricRow("Teams", value: "\(store.snapshot?.teams.count ?? 0)", symbol: "person.3")
            metricRow("Accepted evidence", value: "\(store.snapshot?.evidence.count ?? 0)", symbol: "checkmark.seal")
        }
    }

    private var boardPanel: some View {
        VStack(alignment: .leading, spacing: 12) {
            if (store.snapshot?.missions ?? []).isEmpty {
                emptyPanelState("No missions yet", symbol: "square.3.layers.3d")
            } else {
                ForEach((store.snapshot?.missions ?? []).prefix(8)) { mission in
                    Button {
                        store.openMission(mission.missionID)
                        fullGovernancePresentation = .workbench
                    } label: {
                        inspectorRow(
                            title: mission.title,
                            detail: "\(mission.lane) · \(mission.status)",
                            symbol: mission.attentionCount > 0
                                ? "exclamationmark.circle.fill"
                                : "circle.dashed"
                        )
                    }
                    .buttonStyle(.plain)
                }
            }
            Divider()
            Button {
                store.showMissionBoard()
                fullGovernancePresentation = .workbench
            } label: {
                Label("Open full Mission Board", systemImage: "arrow.up.left.and.arrow.down.right")
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
            .buttonStyle(.bordered)
        }
    }

    @ViewBuilder
    private var teamPanel: some View {
        if let session = store.builderSession {
            teamBuilderPanel(session)
        } else {
            confirmedTeamsPanel
        }
    }

    private var confirmedTeamsPanel: some View {
        VStack(alignment: .leading, spacing: 0) {
            if let recoveryMessage = store.builderRecoveryMessage {
                Label(recoveryMessage, systemImage: "arrow.clockwise")
                    .font(.caption)
                    .foregroundStyle(LoomGraphite.statusWarning)
                    .fixedSize(horizontal: false, vertical: true)
                    .padding(.bottom, 12)
                    .accessibilityLabel("Agent Team draft expired. \(recoveryMessage)")
            }
            if (store.snapshot?.teams ?? []).isEmpty {
                emptyPanelState("No Agent Teams yet", symbol: "person.3")
            } else {
                ForEach(store.snapshot?.teams ?? []) { team in
                    inspectorRow(
                        title: LocalProductExperience.visibleName(
                            team.displayName,
                            internalID: team.teamInstanceID,
                            fallback: "Agent Team"
                        ),
                        detail: humanized(team.state),
                        symbol: "person.3"
                    )
                }
            }
            Button {
                Task { await store.startBlankBuilder() }
            } label: {
                Label("Create Agent Team", systemImage: "plus")
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
            .buttonStyle(.bordered)
            .padding(.top, 14)
        }
    }

    private func teamBuilderPanel(
        _ session: LocalProductBuilderSession
    ) -> some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack(alignment: .top) {
                VStack(alignment: .leading, spacing: 3) {
                    Text("Agent Team draft")
                        .font(.headline)
                    Text("Review every field before confirmation.")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                Spacer()
                Button {
                    builderAnswer = ""
                    store.cancelBuilder()
                } label: {
                    Image(systemName: "xmark")
                        .frame(width: 28, height: 28)
                }
                .buttonStyle(.plain)
                .accessibilityLabel("Cancel Agent Team draft")
                .help("Cancel draft")
            }

            if !session.question.id.isEmpty {
                Text(session.question.prompt)
                    .font(.callout.weight(.medium))
                    .fixedSize(horizontal: false, vertical: true)
                if session.question.options.isEmpty {
                    HStack(spacing: 8) {
                        TextField("Answer", text: $builderAnswer)
                            .textFieldStyle(.roundedBorder)
                        Button {
                            let answer = builderAnswer
                            builderAnswer = ""
                            Task { await store.answerBuilder(answer) }
                        } label: {
                            Image(systemName: "arrow.right")
                        }
                        .buttonStyle(.borderedProminent)
                        .disabled(
                            builderAnswer.trimmingCharacters(
                                in: .whitespacesAndNewlines
                            ).isEmpty || store.setupState == .loading
                        )
                        .accessibilityLabel("Submit Team draft answer")
                    }
                } else {
                    ForEach(session.question.options) { option in
                        Button {
                            Task { await store.answerBuilder(option.id) }
                        } label: {
                            HStack {
                                Text(option.label)
                                Spacer()
                                Image(systemName: "chevron.right")
                            }
                            .frame(maxWidth: .infinity, alignment: .leading)
                        }
                        .buttonStyle(.bordered)
                        .disabled(store.setupState == .loading)
                    }
                }
                Divider()
            }

            VStack(alignment: .leading, spacing: 8) {
                TextField(
                    "Team name",
                    text: Binding(
                        get: {
                            builderName.isEmpty ? session.preview.name : builderName
                        },
                        set: { builderName = $0 }
                    )
                )
                .textFieldStyle(.roundedBorder)
                .onSubmit {
                    let value = builderName.trimmingCharacters(in: .whitespacesAndNewlines)
                    guard !value.isEmpty, value != session.preview.name else { return }
                    Task { await store.editBuilder(field: "team_name", value: value) }
                }
                .disabled(store.setupState == .loading)
                .accessibilityLabel("Agent Team name")

                TextField(
                    "Bounded purpose",
                    text: Binding(
                        get: {
                            builderPurpose.isEmpty ? session.preview.purpose : builderPurpose
                        },
                        set: { builderPurpose = $0 }
                    )
                )
                .textFieldStyle(.roundedBorder)
                .onSubmit {
                    let value = builderPurpose.trimmingCharacters(in: .whitespacesAndNewlines)
                    guard !value.isEmpty, value != session.preview.purpose else { return }
                    Task { await store.editBuilder(field: "purpose", value: value) }
                }
                .disabled(store.setupState == .loading)
                .accessibilityLabel("Agent Team bounded purpose")
                Label(
                    "\(session.preview.roles.count) agents · concurrency \(session.preview.requestedConcurrency)",
                    systemImage: "person.3"
                )
                Label(
                    session.preview.estimatedMaximumCost,
                    systemImage: "gauge.with.dots.needle.50percent"
                )
                .foregroundStyle(.secondary)
                if !session.preview.compatibilityGaps.isEmpty {
                    Label(
                        "\(session.preview.compatibilityGaps.count) compatibility issues",
                        systemImage: "exclamationmark.triangle"
                    )
                    .foregroundStyle(LoomGraphite.statusWarning)
                }
            }
            .font(.caption)

			ForEach(session.preview.roles) { role in
				builderRoleEditor(role, session: session)
			}

			let availableAgents = builderAvailableAgentChoices(session)
			if !availableAgents.isEmpty && session.preview.roles.count < 9 {
				Menu {
					ForEach(availableAgents) { choice in
						Button {
							Task {
								await store.editBuilder(
									field: "subagent_add",
									value: choice.id
								)
							}
						} label: {
							Text(builderRoleChoiceLabel(choice))
						}
					}
				} label: {
					Label("Add Agent", systemImage: "plus")
				}
				.controlSize(.small)
				.disabled(store.setupState == .loading)
				.help("Add an independently configured Agent")
			}

            if store.setupState == .loading {
                ProgressView()
                    .controlSize(.small)
                    .accessibilityLabel("Updating Agent Team draft")
            }

        }
    }

    @ViewBuilder
    private var teamBuilderConfirmFooter: some View {
        if let session = store.builderSession {
            VStack(alignment: .leading, spacing: 8) {
                Button {
                    Task {
                        await store.confirmBuilderCommitting(
                            name: builderName,
                            purpose: builderPurpose
                        )
                    }
                } label: {
                    Label("Confirm Agent Team", systemImage: "checkmark")
                        .frame(maxWidth: .infinity)
                }
                .buttonStyle(.borderedProminent)
                .disabled(!session.canConfirm || store.setupState == .loading)
                .accessibilityLabel("Confirm Agent Team and create governed Team")
                if !session.canConfirm && store.setupState != .loading {
                    builderConfirmBlockedHint(
                        session: session,
                        uncommittedName: builderName,
                        uncommittedPurpose: builderPurpose
                    )
                }
            }
            .padding(16)
        }
    }

    private func builderConfirmBlockedHint(
        session: LocalProductBuilderSession,
        uncommittedName: String,
        uncommittedPurpose: String
    ) -> some View {
        let message = session.preview.confirmationBlockedMessage(
            uncommittedName: uncommittedName,
            uncommittedPurpose: uncommittedPurpose
        )
        return Label(message, systemImage: "info.circle")
            .font(.caption2)
            .foregroundStyle(.secondary)
            .fixedSize(horizontal: false, vertical: true)
            .accessibilityLabel("Team confirmation blocked. \(message)")
    }

	private func builderRoleEditor(
		_ role: LocalProductBuilderRole,
		session: LocalProductBuilderSession
	) -> some View {
		let review = LocalProductRoleReview(role: role)
		let field = role.kind == "main" ? "main_role" : "subagent_role"
		let roleTarget = role.kind == "subagent" ? role.agentDefinitionID : ""
		let choices = store.setupSnapshot.map {
			LocalProductRoleOptionChoice.all(
				setup: $0,
				preview: session.preview
			).filter {
				$0.field == field &&
				$0.agentDefinitionID == role.agentDefinitionID
			}
		} ?? []
		return VStack(alignment: .leading, spacing: 7) {
			HStack(spacing: 8) {
				VStack(alignment: .leading, spacing: 2) {
					Text(review.name.isEmpty ? review.kind : review.name)
						.font(.callout.weight(.semibold))
					Text(review.kind)
						.font(.caption)
						.foregroundStyle(.secondary)
				}
				Spacer(minLength: 8)
				Menu {
					ForEach(choices) { choice in
						Button {
							Task {
								await store.editBuilder(
									field: choice.field,
									value: choice.id,
									roleAgentDefinitionID: roleTarget
								)
							}
						} label: {
							Label(
								builderRoleChoiceLabel(choice),
								systemImage: choice.isCurrent
									? "checkmark"
									: "circle"
							)
						}
					}
				} label: {
					Image(systemName: "slider.horizontal.3")
						.frame(width: 28, height: 28)
				}
				.menuStyle(.borderlessButton)
				.disabled(choices.isEmpty || store.setupState == .loading)
				.accessibilityLabel("Choose \(review.kind) Agent role")
				.help("Choose Agent role")
				if role.kind == "subagent" &&
					session.preview.roles.filter({ $0.kind == "subagent" }).count > 1 {
					Button {
						Task {
							await store.editBuilder(
								field: "subagent_remove",
								value: "remove",
								roleAgentDefinitionID: role.agentDefinitionID
							)
						}
					} label: {
						Image(systemName: "trash")
							.frame(width: 28, height: 28)
					}
					.buttonStyle(.plain)
					.accessibilityLabel("Remove \(review.name) Agent")
					.help("Remove Agent")
				}
			}

			Text(
				[
					review.harness,
					review.providerAccount.isEmpty
						? review.provider
						: "\(review.provider) / \(review.providerAccount)",
					review.model,
				].filter { !$0.isEmpty }.joined(separator: " · ")
			)
			.font(.caption)
			.foregroundStyle(.secondary)
			.fixedSize(horizontal: false, vertical: true)

			Text(
				[review.credential, review.reasoning, review.timeout, review.budget]
					.filter { !$0.isEmpty }
					.joined(separator: " · ")
			)
			.font(.caption2)
			.foregroundStyle(.tertiary)
			.fixedSize(horizontal: false, vertical: true)

			builderRoleProfileControls(role, review: review)

			if !role.compatible {
				Label(review.compatibility, systemImage: "exclamationmark.triangle")
					.font(.caption)
					.foregroundStyle(LoomGraphite.statusWarning)
			}
			Divider()
		}
	}

	@ViewBuilder
	private func builderRoleProfileControls(
		_ role: LocalProductBuilderRole,
		review: LocalProductRoleReview
	) -> some View {
		let prefix = role.kind == "main" ? "main" : "subagent"
		let roleTarget = role.kind == "subagent" ? role.agentDefinitionID : ""
		let timeoutSeconds = max(1, Int(role.timeoutMilliseconds / 1_000))
		let fallbackChoices = store.setupSnapshot.map {
			LocalProductRoleOptionChoice.fallbacks(setup: $0, role: role)
		} ?? []
		let selectedParallelProfiles = Set(
			role.parallelRoutes.dropFirst().map(\.runtimeProfileID)
		)
		let parallelChoices = fallbackChoices.filter {
			!selectedParallelProfiles.contains($0.runtimeProfileID)
		}
		let harnessRoutes = uniqueBuilderRouteChoices(
			store.setupSnapshot.map {
				LocalProductRoleOptionChoice.routes(
					setup: $0,
					role: role,
					field: "\(prefix)_harness_route"
				)
			} ?? [],
			identity: { $0.harness }
		)
		let accountRoutes = uniqueBuilderRouteChoices(
			store.setupSnapshot.map {
				LocalProductRoleOptionChoice.routes(
					setup: $0,
					role: role,
					field: "\(prefix)_provider_account_route"
				)
			} ?? [],
			identity: {
				"\($0.provider)\u{0}\($0.providerAccount)\u{0}\($0.credential)\u{0}\($0.model)"
			}
		)
		VStack(alignment: .leading, spacing: 6) {
			HStack(spacing: 8) {
				Label("Parallel routes", systemImage: "square.stack.3d.up")
					.font(.caption)
				Spacer(minLength: 8)
				Menu {
					ForEach(parallelChoices) { choice in
						Button {
							Task {
								await store.editBuilder(
									field: "\(prefix)_parallel_route_add",
									value: choice.id,
									roleAgentDefinitionID: roleTarget
								)
							}
						} label: {
							Label(
								builderProviderAccountChoiceLabel(choice),
								systemImage: "plus"
							)
						}
					}
				} label: {
					Label(
						role.parallelRouteSetVersion == 1
							? "\(role.parallelRoutes.count) routes"
							: "Single route",
						systemImage: "plus.circle"
					)
				}
				.controlSize(.small)
				.disabled(
					parallelChoices.isEmpty || role.parallelRoutes.count >= 3 ||
					role.fallbackConfigured || store.setupState == .loading
				)
				.help("Add a Provider route for parallel execution")
			}
			if role.parallelRouteSetVersion == 1 {
				ForEach(Array(role.parallelRoutes.dropFirst())) { route in
					HStack(spacing: 8) {
						Text([
							route.providerAccountID.isEmpty
								? route.providerID
								: route.providerAccountID,
							route.modelID,
						].filter { !$0.isEmpty }.joined(separator: " · "))
						.font(.caption2)
						.foregroundStyle(.secondary)
						.lineLimit(1)
						Spacer(minLength: 8)
						if let choice = fallbackChoices.first(where: {
							$0.runtimeProfileID == route.runtimeProfileID
						}) {
							Button {
								Task {
									await store.editBuilder(
										field: "\(prefix)_parallel_route_remove",
										value: choice.id,
										roleAgentDefinitionID: roleTarget
									)
								}
							} label: {
								Image(systemName: "trash")
							}
							.buttonStyle(.borderless)
							.help("Remove parallel route")
						}
					}
				}
				Label(
					"Synthesis · \(role.synthesisRoute.modelID)",
					systemImage: "arrow.triangle.merge"
				)
				.font(.caption2)
				.foregroundStyle(.secondary)
			}

			HStack(spacing: 8) {
				Label("Fallback route", systemImage: "arrow.triangle.branch")
					.font(.caption)
				Spacer(minLength: 8)
				Menu {
					Button {
						Task {
							await store.editBuilder(
								field: "\(prefix)_fallback_role",
								value: "none",
								roleAgentDefinitionID: roleTarget
							)
						}
					} label: {
						Label(
							"No fallback",
							systemImage: role.fallbackConfigured
								? "circle"
								: "checkmark"
						)
					}
					Divider()
					ForEach(fallbackChoices) { choice in
						Button {
							Task {
								await store.editBuilder(
									field: choice.field,
									value: choice.id,
									roleAgentDefinitionID: roleTarget
								)
							}
						} label: {
							Label(
								builderRoleChoiceLabel(choice),
								systemImage: choice.isCurrent
									? "checkmark"
									: "circle"
							)
						}
					}
				} label: {
					Text(role.fallbackConfigured ? "Configured" : "None")
				}
				.controlSize(.small)
				.disabled(
					store.setupState == .loading || role.parallelRouteSetVersion == 1
				)
				.help("Choose an explicit fallback execution profile")
			}
			if role.fallbackConfigured {
				Text(review.fallback)
					.font(.caption2)
					.foregroundStyle(.secondary)
					.fixedSize(horizontal: false, vertical: true)
				Label(
					"Approval required before route change",
					systemImage: "hand.raised"
				)
				.font(.caption2)
				.foregroundStyle(LoomGraphite.statusWarning)
			}

			HStack(spacing: 8) {
				Menu {
					ForEach(harnessRoutes) { choice in
						Button {
							Task {
								await store.editBuilder(
									field: choice.field,
									value: choice.id,
									roleAgentDefinitionID: roleTarget
								)
							}
						} label: {
							Label(
								choice.harness,
								systemImage: choice.isCurrent ? "checkmark" : "circle"
							)
						}
					}
				} label: {
					Label(review.harness, systemImage: "wrench.and.screwdriver")
				}
				.controlSize(.small)
				.disabled(harnessRoutes.isEmpty)
				.help("Choose Harness")

				Menu {
					ForEach(accountRoutes) { choice in
						Button {
							Task {
								await store.editBuilder(
									field: choice.field,
									value: choice.id,
									roleAgentDefinitionID: roleTarget
								)
							}
						} label: {
							Label(
								builderProviderAccountChoiceLabel(choice),
								systemImage: choice.isCurrent ? "checkmark" : "circle"
							)
						}
					}
				} label: {
					Label(
						review.providerAccount.isEmpty
							? review.provider
							: review.providerAccount,
						systemImage: "key.horizontal"
					)
				}
				.controlSize(.small)
				.disabled(accountRoutes.isEmpty)
				.help("Choose Provider Account")
				Spacer(minLength: 0)
			}

			HStack(spacing: 8) {
				if role.runtime.modelIDs.count > 1 {
					Menu {
						ForEach(role.runtime.modelIDs, id: \.self) { model in
							Button(model) {
								Task {
									await store.editBuilder(
										field: "\(prefix)_model",
										value: model,
										roleAgentDefinitionID: roleTarget
									)
								}
							}
						}
					} label: {
						Label(review.model, systemImage: "cpu")
					}
					.controlSize(.small)
					.help("Choose model")
				}
				if role.requiredCapabilities.contains("reasoning_effort") {
					Menu {
						ForEach([
							("Provider default", "provider_default"),
							("Low", "low"),
							("Medium", "medium"),
							("High", "high"),
						], id: \.1) { option in
							Button(option.0) {
								Task {
									await store.editBuilder(
										field: "\(prefix)_reasoning_effort",
										value: option.1,
										roleAgentDefinitionID: roleTarget
									)
								}
							}
						}
					} label: {
						Label(review.reasoning, systemImage: "brain")
					}
					.controlSize(.small)
					.help("Choose reasoning effort")
				}
				Spacer(minLength: 0)
			}

			HStack(spacing: 8) {
				Label("Remote tools", systemImage: "globe")
					.font(.caption)
				Spacer(minLength: 8)
				Menu {
					Button {
						Task {
							await store.clearBuilderRemoteToolEnrollment(
								roleAgentDefinitionID: roleTarget
							)
						}
					} label: {
						Label(
							"No remote tools",
							systemImage: role.remoteToolEnrollmentID.isEmpty
								? "checkmark"
								: "circle"
						)
					}
					Divider()
					ForEach(builderEnrollmentChoices(role)) { enrollment in
						Button {
							Task {
								await store.editBuilderRemoteToolEnrollment(
									enrollment: enrollment,
									roleAgentDefinitionID: roleTarget
								)
							}
						} label: {
							Label(
								builderEnrollmentChoiceLabel(enrollment),
								systemImage: enrollment.enrollmentID ==
									role.remoteToolEnrollmentID
									? "checkmark"
									: "circle"
							)
						}
					}
				} label: {
					Label(
						builderEnrollmentChoiceSummary(
							role: role,
							enrollments: builderEnrollmentChoices(role)
						),
						systemImage: "globe.badge.checkmark"
					)
				}
				.controlSize(.small)
				.disabled(
					store.setupState == .loading ||
					builderEnrollmentChoices(role).isEmpty
				)
				.help("Bind a remote tool Enrollment to this Agent")
			}

			HStack(spacing: 12) {
				Stepper(
					value: Binding(
						get: { timeoutSeconds },
						set: { value in
							Task {
								await store.editBuilder(
									field: "\(prefix)_timeout_seconds",
									value: String(value),
									roleAgentDefinitionID: roleTarget
								)
							}
						}
					),
					in: 15...3_600,
					step: 15
				) {
					Label(review.timeout, systemImage: "timer")
						.font(.caption)
				}
				.controlSize(.small)

				Toggle(
					isOn: Binding(
						get: { role.budgetAvailable },
						set: { enabled in
							Task {
								await store.editBuilder(
									field: "\(prefix)_budget_units",
									value: enabled
										? String(max(1, role.budgetUnits))
										: "none",
									roleAgentDefinitionID: roleTarget
								)
							}
						}
					)
				) {
					Text(role.budgetAvailable ? review.budget : "No budget")
						.font(.caption)
				}
				.toggleStyle(.switch)
				.controlSize(.mini)
			}

			if role.budgetAvailable {
				Stepper(
					value: Binding(
						get: { max(1, role.budgetUnits) },
						set: { value in
							Task {
								await store.editBuilder(
									field: "\(prefix)_budget_units",
									value: String(value),
									roleAgentDefinitionID: roleTarget
								)
							}
						}
					),
					in: 1...1_000_000_000,
					step: 5
				) {
					Label(review.budget, systemImage: "banknote")
						.font(.caption)
				}
				.controlSize(.small)
			}
		}
		.disabled(store.setupState == .loading)
	}

	private func builderRoleChoiceLabel(
		_ choice: LocalProductRoleOptionChoice
	) -> String {
		let account = choice.providerAccount.isEmpty
			? choice.provider
			: choice.providerAccount
		return [choice.responsibility, account, choice.model]
			.filter { !$0.isEmpty }
			.joined(separator: " · ")
	}

	private func builderAvailableAgentChoices(
		_ session: LocalProductBuilderSession
	) -> [LocalProductRoleOptionChoice] {
		guard let setup = store.setupSnapshot else { return [] }
		let selected = Set(session.preview.roles.map(\.agentDefinitionID))
		var seen = Set<String>()
		return LocalProductRoleOptionChoice.all(
			setup: setup,
			preview: session.preview
		).filter { choice in
			guard choice.field == "subagent_role",
				!selected.contains(choice.agentDefinitionID),
				!seen.contains(choice.agentDefinitionID) else {
				return false
			}
			seen.insert(choice.agentDefinitionID)
			return true
		}
	}

	private func builderProviderAccountChoiceLabel(
		_ choice: LocalProductRoleOptionChoice
	) -> String {
		let account = choice.providerAccount.isEmpty
			? choice.provider
			: "\(choice.provider) / \(choice.providerAccount)"
		return [account, choice.credential, choice.model]
			.filter { !$0.isEmpty }
			.joined(separator: " · ")
	}

	private func uniqueBuilderRouteChoices(
		_ choices: [LocalProductRoleOptionChoice],
		identity: (LocalProductRoleOptionChoice) -> String
	) -> [LocalProductRoleOptionChoice] {
		var seen: Set<String> = []
		return choices.filter { seen.insert(identity($0)).inserted }
	}

	private func builderEnrollmentChoices(
		_ role: LocalProductBuilderRole
	) -> [LocalProductRemoteToolBackendEnrollment] {
		guard let snapshot = store.setupSnapshot else { return [] }
		let account = snapshot.providerAccounts.first {
			$0.providerID == role.providerID &&
			$0.providerAccountID == role.providerAccountID
		}
		return (account?.remoteToolBackends ?? []).filter {
			$0.status == "active" && $0.policyCurrent
		}
	}

	private func builderEnrollmentChoiceLabel(
		_ enrollment: LocalProductRemoteToolBackendEnrollment
	) -> String {
		let kind = enrollment.backendKind == "mcp_server"
			? "MCP"
			: "Web Search"
		let server = enrollment.mcpServerID.isEmpty
			? ""
			: " · \(enrollment.mcpServerID)"
		return "\(kind) · \(enrollment.enrollmentID)\(server)"
	}

	private func builderEnrollmentChoiceSummary(
		role: LocalProductBuilderRole,
		enrollments: [LocalProductRemoteToolBackendEnrollment]
	) -> String {
		if let enrollment = enrollments.first(where: {
			$0.enrollmentID == role.remoteToolEnrollmentID
		}) {
			return builderEnrollmentChoiceLabel(enrollment)
		}
		return role.remoteToolEnrollmentID.isEmpty
			? "No remote tools"
			: "Review binding"
	}

    private var topologyPanel: some View {
        VStack(alignment: .leading, spacing: 12) {
            let missions = (store.snapshot?.missions ?? []).filter {
                !$0.topology.isEmpty
            }
            if missions.isEmpty {
                emptyPanelState("No Team topology yet", symbol: "point.3.connected.trianglepath.dotted")
            } else {
                ForEach(missions.prefix(4)) { mission in
                    Text(mission.title)
                        .font(.caption.weight(.semibold))
                        .foregroundStyle(.secondary)
                    ForEach(Array(mission.topology.prefix(5).enumerated()), id: \.offset) { _, node in
                        inspectorRow(
                            title: LocalProductExperience.visibleName(
                                node.title,
                                internalID: node.logicalNodeID,
                                fallback: humanized(node.role)
                            ),
                            detail: node.dependsOn.isEmpty
                                ? humanized(node.role)
                                : "\(humanized(node.role)) · \(node.dependsOn.count) dependencies",
                            symbol: "point.3.connected.trianglepath.dotted"
                        )
                    }
                }
            }
        }
    }

    private var timelinePanel: some View {
        VStack(alignment: .leading, spacing: 0) {
            let runs = store.snapshot?.runs ?? []
            if runs.isEmpty {
                emptyPanelState("No Agent activity yet", symbol: "clock.arrow.circlepath")
            } else {
                ForEach(runs.prefix(10)) { run in
                    inspectorRow(
                        title: humanized(run.phase, fallback: "Agent activity"),
                        detail: run.terminalStatus.isEmpty
                            ? "In progress"
                            : humanized(run.terminalStatus),
                        symbol: "clock.arrow.circlepath"
                    )
                }
            }
        }
    }

    private var decisionsPanel: some View {
        VStack(alignment: .leading, spacing: 0) {
            let decisions = store.snapshot?.preparedDecisions ?? []
            if decisions.isEmpty {
                emptyPanelState("No decisions need review", symbol: "checkmark.circle")
            } else {
                ForEach(decisions, id: \.decisionID) { decision in
                    Button {
                        store.openMission(decision.missionID)
                        fullGovernancePresentation = .workbench
                        Task { await store.openPreparedDecision(decision) }
                    } label: {
                        inspectorRow(
                            title: humanized(decision.kind.rawValue),
                            detail: humanized(decision.action),
                            symbol: "questionmark.diamond"
                        )
                    }
                    .buttonStyle(.plain)
                    .accessibilityLabel("Open \(humanized(decision.kind.rawValue)) decision")
                }
            }
        }
    }

    private var evidencePanel: some View {
        VStack(alignment: .leading, spacing: 0) {
            if (store.snapshot?.evidence ?? []).isEmpty {
                emptyPanelState("No accepted evidence yet", symbol: "checkmark.seal")
            } else {
                ForEach(store.snapshot?.evidence ?? []) { evidence in
                    inspectorRow(
                        title: "Accepted evidence",
                        detail: SafeText.sanitize(evidence.workItemID, limit: 96),
                        symbol: "checkmark.seal"
                    )
                }
            }
        }
    }

    private var runtimePanel: some View {
        VStack(alignment: .leading, spacing: 0) {
            if (store.snapshot?.runtimes ?? []).isEmpty {
                emptyPanelState("No runtimes discovered", symbol: "cpu")
            } else {
                ForEach(store.snapshot?.runtimes ?? []) { runtime in
                    inspectorRow(
                        title: LocalProductExperience.visibleName(
                            runtime.displayName,
                            internalID: runtime.runtimeInstanceID,
                            fallback: "Runtime"
                        ),
                        detail: "\(humanized(runtime.status)) · Capacity \(runtime.capacity)",
                        symbol: "cpu"
                    )
                }
            }
            Divider()
                .padding(.vertical, 12)
            Button {
                fullGovernancePresentation = .runtimeProviders
            } label: {
                Label("Manage Runtimes & Providers", systemImage: "slider.horizontal.3")
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
            .buttonStyle(.bordered)
        }
    }

    private var attentionPanel: some View {
        VStack(alignment: .leading, spacing: 0) {
            if (store.snapshot?.attention ?? []).isEmpty {
                emptyPanelState("Nothing needs your attention", symbol: "checkmark.circle")
            } else {
                ForEach(store.snapshot?.attention ?? []) { item in
                    Button {
                        store.showMissionAttention()
                        fullGovernancePresentation = .workbench
                    } label: {
                        inspectorRow(
                            title: SafeText.sanitize(item.actionRequired, limit: 120),
                            detail: humanized(item.severity),
                            symbol: "exclamationmark.triangle"
                        )
                    }
                    .buttonStyle(.plain)
                    .accessibilityLabel("Open item needing attention")
                }
            }
        }
    }

    private var libraryPanel: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Agent definitions, skills, and governed evolution assets.")
                .foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)
            Button {
                store.showMissionLibrary()
                fullGovernancePresentation = .workbench
            } label: {
                Label("Open Library", systemImage: "books.vertical")
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
            .buttonStyle(.bordered)
        }
    }

    private func metricRow(_ title: String, value: String, symbol: String) -> some View {
        HStack(spacing: 10) {
            Image(systemName: symbol)
                .foregroundStyle(.secondary)
                .frame(width: 20)
            Text(title)
            Spacer()
            Text(value)
                .font(.body.monospacedDigit().weight(.semibold))
        }
        .frame(minHeight: 44)
        .overlay(alignment: .bottom) { Divider() }
        .accessibilityElement(children: .combine)
    }

    private func inspectorRow(title: String, detail: String, symbol: String) -> some View {
        HStack(alignment: .top, spacing: 10) {
            Image(systemName: symbol)
                .foregroundStyle(.secondary)
                .frame(width: 20)
            VStack(alignment: .leading, spacing: 3) {
                Text(title)
                    .font(.callout.weight(.medium))
                    .lineLimit(2)
                Text(detail)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .lineLimit(2)
            }
            Spacer(minLength: 0)
        }
        .padding(.vertical, 10)
        .frame(maxWidth: .infinity, alignment: .leading)
        .overlay(alignment: .bottom) { Divider() }
        .contentShape(Rectangle())
        .accessibilityElement(children: .combine)
    }

    private func emptyPanelState(_ title: String, symbol: String) -> some View {
        VStack(spacing: 10) {
            Image(systemName: symbol)
                .font(.title2)
                .foregroundStyle(.secondary)
            Text(title)
                .font(.callout)
                .foregroundStyle(.secondary)
        }
        .frame(maxWidth: .infinity, minHeight: 180)
        .accessibilityElement(children: .combine)
    }

    private var experience: LocalProductExperience {
        LocalProductExperience(
            snapshot: store.snapshot,
            connectionState: store.connectionState
        )
    }

    private var serviceFooterTitle: String {
        switch experience.state {
        case .connectedEmpty, .connectedPopulated:
            return "Local service ready"
        default:
            return experience.connection.title
        }
    }

    private var activeMissionCount: Int {
        countActiveMissions(
            teams: store.snapshot?.teams ?? [],
            missions: store.snapshot?.missions ?? []
        )
    }

    /// A Mission is actionable only while its Team is executable; archiving a
    /// Team moves its Missions out of the active count and active board lanes
    /// (they stay visible in the Complete lane as history).
    private func missionTeamExecutable(
        _ mission: LocalProductMissionSummary
    ) -> Bool {
        guard !mission.teamInstanceID.isEmpty,
              let teams = store.snapshot?.teams else {
            return false
        }
        return teams.contains {
            $0.teamInstanceID == mission.teamInstanceID && $0.executable
        }
    }

    private var hasGovernanceActivity: Bool {
        guard let snapshot = store.snapshot else { return false }
        return !snapshot.missions.isEmpty
            || !snapshot.teams.isEmpty
            || !snapshot.attention.isEmpty
            || !snapshot.runs.isEmpty
            || !snapshot.evidence.isEmpty
            || !snapshot.runtimes.isEmpty
    }

    private var sendButtonEnabled: Bool {
        !store.isSendingChatMessage
            && !store.workspace.selectedContinuity.composerDraft
            .trimmingCharacters(in: .whitespacesAndNewlines)
            .isEmpty
    }

    private var stripBackground: Color {
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

    private func governanceSymbol(_ destination: LoomGovernanceDestination) -> String {
        switch destination {
        case .overview: return "chart.bar.xaxis"
        case .board: return "square.3.layers.3d"
        case .team: return "person.3"
        case .topology: return "point.3.connected.trianglepath.dotted"
        case .timeline: return "clock.arrow.circlepath"
        case .decisions: return "questionmark.diamond"
        case .evidence: return "checkmark.seal"
        case .runtimes: return "cpu"
        case .attention: return "exclamationmark.triangle"
        case .library: return "books.vertical"
        }
    }

    private func humanized(_ value: String, fallback: String = "Available") -> String {
        let safe = SafeText.sanitize(value, limit: 96)
            .replacingOccurrences(of: "_", with: " ")
            .replacingOccurrences(of: "-", with: " ")
            .trimmingCharacters(in: .whitespaces)
        guard !safe.isEmpty else { return fallback }
        return safe.prefix(1).uppercased() + safe.dropFirst()
    }

    private func handleFolderSelection(_ result: Result<[URL], Error>) {
        guard case .success(let urls) = result, let url = urls.first else { return }
        let scoped = url.startAccessingSecurityScopedResource()
        defer {
            if scoped {
                url.stopAccessingSecurityScopedResource()
            }
        }
        store.selectWorkspaceFolderDisplayName(url.lastPathComponent)
    }
}

/// Builds a copyable transcript of a conversation: one "Speaker: content" line
/// per message, blank-line separated, so the whole thread can be copied at once.
func conversationTranscriptText(_ messages: [LocalProductChatMessage]) -> String {
    let parts = messages.map { message -> String in
        let speaker: String
        switch message.role {
        case "user": speaker = "You"
        case "proposal": speaker = "Loom proposal"
        default: speaker = "Loom"
        }
        return "\(speaker): \(message.displayContent)"
    }
    return parts.joined(separator: "\n\n")
}

/// Short date suffix appended to a sidebar conversation title when another
/// session shares the same auto-title: time-of-day for today, otherwise the
/// month + day (for example "hello · 4:30 PM" / "hello · Aug 20").
func conversationDisambiguationSuffix(for date: Date, now: Date = Date()) -> String {
    let formatter = DateFormatter()
    if Calendar.current.isDate(date, inSameDayAs: now) {
        formatter.dateFormat = "h:mm a"
    } else {
        formatter.dateFormat = "MMM d"
    }
    return formatter.string(from: date)
}

/// Builds a Mission objective from a chat proposal, falling back to the
/// latest user message. A proposal longer than the Mission objective limit is
/// truncated to the limit so the New Mission sheet starts with a useful draft
/// instead of an empty field.
func chatMissionObjectiveText(
    around message: LocalProductChatMessage,
    thread: LocalProductChatThread?
) -> String {
    let limit = 4_096
    let trimmedProposal = message.displayContent.trimmingCharacters(
        in: .whitespacesAndNewlines
    )
    if !trimmedProposal.isEmpty {
        return String(trimmedProposal.prefix(limit))
    }
    for candidate in (thread?.messages ?? []).reversed() {
        if candidate.role == "user" {
            let value = candidate.displayContent.trimmingCharacters(
                in: .whitespacesAndNewlines
            )
            if !value.isEmpty {
                return String(value.prefix(limit))
            }
        }
    }
    return ""
}


/// Counts Missions a user can actually act on: not Complete and on an
/// executable (non-archived) Team. Archiving a Team moves its Missions out of
/// the active count.
func countActiveMissions(
  teams: [LocalProductTeamSummary],
  missions: [LocalProductMissionSummary]
) -> Int {
  let executableTeams = Set(
    teams.filter { $0.executable }.map { $0.teamInstanceID }
  )
  return missions.filter {
    $0.lane != "Complete" && executableTeams.contains($0.teamInstanceID)
  }.count
}
