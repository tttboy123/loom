import SwiftUI
import LoomLocalAppCore

public enum LoomDesign {
    public static let panelRadius: CGFloat = 16
    public static let minimumActionTarget: CGFloat = 44
    public static let contentWidth: CGFloat = 920
}

enum ProviderConnectionKind {
    case codex
    case miniMax
}

enum ProviderConnectionPrimaryAction: Equatable {
    case connect
    case manage
}

func providerConnectionPrimaryAction(
    provider: ProviderConnectionKind,
    connected: Bool
) -> ProviderConnectionPrimaryAction {
    switch (provider, connected) {
    case (.codex, false), (.miniMax, false):
        return .connect
    case (.codex, true), (.miniMax, true):
        return .manage
    }
}

public struct ContentView: View {
    @ObservedObject private var store: LocalProductStore
    private let refreshOnAppear: Bool
    @State private var builderAnswer = ""
    @State private var builderEditField = ""
    @State private var builderEditValue = ""
    @State private var showTeamBuilder = false
    @State private var showInspector = false
    @State private var taskQuery = ""

    public init(
        store: LocalProductStore,
        refreshOnAppear: Bool = true
    ) {
        self.store = store
        self.refreshOnAppear = refreshOnAppear
    }

    public var body: some View {
        MissionWorkbench(store: store)
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
        .frame(minWidth: 780, minHeight: 580)
    }

    private var experience: LocalProductExperience {
        LocalProductExperience(
            snapshot: store.snapshot,
            connectionState: store.connectionState
        )
    }

    private var taskSidebar: some View {
        VStack(spacing: 0) {
            HStack {
                VStack(alignment: .leading, spacing: 2) {
                    Text("Loom")
                        .font(.title2.weight(.semibold))
                    Text("Local workspace")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                Spacer()
                Button {
                    store.selectWorkspaceTask(
                        LocalProductWorkspaceState.newTaskID
                    )
                } label: {
                    Image(systemName: "square.and.pencil")
                        .frame(
                            minWidth: LoomDesign.minimumActionTarget,
                            minHeight: LoomDesign.minimumActionTarget
                        )
                }
                .buttonStyle(.plain)
                .help("New task")
                .accessibilityLabel("New task")
            }
            .padding(.horizontal, 14)
            .padding(.top, 16)
            .padding(.bottom, 10)

            Divider()

            HStack(spacing: 7) {
                Image(systemName: "magnifyingglass")
                    .foregroundStyle(.secondary)
                    .accessibilityHidden(true)
                TextField("Search tasks", text: $taskQuery)
                    .textFieldStyle(.plain)
            }
            .padding(.horizontal, 10)
            .frame(height: 32)
            .background(
                Color(nsColor: .controlBackgroundColor),
                in: RoundedRectangle(cornerRadius: 8, style: .continuous)
            )
            .padding(.horizontal, 10)
            .padding(.top, 10)

            ScrollView {
                LazyVStack(alignment: .leading, spacing: 4) {
                    Text("TASKS")
                        .font(.caption2.weight(.semibold))
                        .foregroundStyle(.secondary)
                        .tracking(0.8)
                        .padding(.horizontal, 10)
                        .padding(.top, 12)
                    ForEach(
                        store.workspace.filteredTasks(matching: taskQuery)
                    ) { task in
                        workspaceTaskButton(task)
                    }
                }
                .padding(.horizontal, 8)
                .padding(.bottom, 12)
            }

            Divider()
            sidebarStatus
        }
        .background(Color(nsColor: .underPageBackgroundColor))
        .accessibilityLabel("Tasks and recent work")
    }

    private func workspaceTaskButton(
        _ task: LocalProductWorkspaceTask
    ) -> some View {
        let selected = task.id == store.workspace.selectedTaskID
        return Button {
            Task {
                store.selectWorkspaceTask(task.id)
                await store.activateWorkspaceTask()
            }
        } label: {
            HStack(alignment: .top, spacing: 9) {
                Image(systemName: taskIcon(task.kind))
                    .font(.callout)
                    .foregroundStyle(
                        selected ? Color.accentColor : Color.secondary
                    )
                    .frame(width: 18)
                    .padding(.top, 2)
                VStack(alignment: .leading, spacing: 3) {
                    Text(task.title)
                        .font(.callout.weight(selected ? .semibold : .regular))
                        .lineLimit(1)
                    Text(task.subtitle)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .lineLimit(2)
                }
                Spacer(minLength: 0)
            }
            .padding(.horizontal, 9)
            .padding(.vertical, 9)
            .frame(maxWidth: .infinity, alignment: .leading)
            .contentShape(Rectangle())
            .background(
                selected ? Color.accentColor.opacity(0.12) : Color.clear,
                in: RoundedRectangle(cornerRadius: 9, style: .continuous)
            )
        }
        .buttonStyle(.plain)
        .accessibilityAddTraits(selected ? .isSelected : [])
    }

    private var conversationWorkspace: some View {
        VStack(spacing: 0) {
            HStack(spacing: 10) {
                VStack(alignment: .leading, spacing: 2) {
                    Text(store.workspace.selectedTask.title)
                        .font(.headline)
                    Text(store.workspace.selectedTask.subtitle)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                Spacer()
                if store.workspace.selectedTask.kind != .draft {
                    Text("Read only")
                        .font(.caption.weight(.medium))
                        .foregroundStyle(.secondary)
                        .padding(.horizontal, 8)
                        .padding(.vertical, 4)
                        .background(.quaternary, in: Capsule())
                }
                Button {
                    showInspector.toggle()
                } label: {
                    Label("Inspector", systemImage: "sidebar.right")
                        .labelStyle(.iconOnly)
                        .frame(
                            minWidth: LoomDesign.minimumActionTarget,
                            minHeight: LoomDesign.minimumActionTarget
                        )
                }
                .buttonStyle(.plain)
                .help("Show inspector")
            }
            .padding(.horizontal, 18)
            .padding(.vertical, 10)
            .background(.bar)

            if experience.state != .connectedEmpty,
               experience.state != .connectedPopulated {
                connectionStrip
            }

            ScrollViewReader { _ in
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 18) {
                        if store.workspace.selectedTask.kind == .draft {
                            conversationWelcome
                            if store.setupSnapshot == nil {
                                setupProviderPanel
                            }
                            builderPanel
                        } else {
                            historicalConversation
                        }
                    }
                    .frame(
                        maxWidth: 760,
                        alignment: .leading
                    )
                    .padding(.horizontal, 24)
                    .padding(.vertical, 24)
                    .frame(maxWidth: .infinity)
                }
            }

            if store.workspace.selectedTask.kind == .draft {
                composer
            }
        }
        .background(Color(nsColor: .textBackgroundColor))
    }

    private var conversationWelcome: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("What would you like Loom to help with?")
                .font(.title2.weight(.semibold))
            Text(
                "Describe the outcome. Loom will guide you through the " +
                    "team, provider, model, permissions, and final review."
            )
            .foregroundStyle(.secondary)
            .fixedSize(horizontal: false, vertical: true)
        }
        .accessibilityElement(children: .combine)
    }

    private var historicalConversation: some View {
        VStack(alignment: .leading, spacing: 12) {
            Label(
                "This summary comes from your local history.",
                systemImage: "clock.arrow.circlepath"
            )
            .font(.headline)
            Text(
                "Open the inspector for team context, changes, and accepted " +
                    "evidence. Historical items remain read only here."
            )
            .foregroundStyle(.secondary)
            if let timeline = store.timeline, !timeline.records.isEmpty {
                ForEach(timeline.records) { record in
                    TimelineRow(record: record)
                }
            } else {
                Text("No additional activity is available.")
                    .foregroundStyle(.secondary)
            }
        }
    }

    private var composer: some View {
        VStack(spacing: 8) {
            Divider()
            HStack(alignment: .bottom, spacing: 10) {
                TextField(
                    "Describe a task...",
                    text: Binding(
                        get: {
                            store.workspace.selectedContinuity.composerDraft
                        },
                        set: store.updateComposerDraft
                    ),
                    axis: .vertical
                )
                .textFieldStyle(.plain)
                .lineLimit(1...5)
                .padding(.horizontal, 12)
                .padding(.vertical, 10)
                .background(
                    Color(nsColor: .controlBackgroundColor),
                    in: RoundedRectangle(cornerRadius: 12, style: .continuous)
                )
                .overlay {
                    RoundedRectangle(cornerRadius: 12, style: .continuous)
                        .stroke(.separator, lineWidth: 1)
                }

                Button {
                    Task {
                        if store.builderSession == nil {
                            await store.startBlankBuilder()
                        }
                    }
                } label: {
                    Image(systemName: "arrow.up")
                        .font(.body.weight(.semibold))
                        .foregroundStyle(.white)
                        .frame(width: 36, height: 36)
                        .background(Color.accentColor, in: Circle())
                }
                .buttonStyle(.plain)
                .disabled(
                    store.workspace.selectedContinuity.composerDraft
                        .trimmingCharacters(in: .whitespacesAndNewlines)
                        .isEmpty
                )
                .accessibilityLabel("Continue")
            }
            Text("Review and confirmation are required before anything is saved.")
                .font(.caption2)
                .foregroundStyle(.secondary)
        }
        .padding(.horizontal, 18)
        .padding(.bottom, 14)
        .background(.bar)
    }

    private var workspaceInspector: some View {
        VStack(spacing: 0) {
            HStack {
                Text("Inspector")
                    .font(.headline)
                Spacer()
            }
            .padding(.horizontal, 16)
            .frame(height: 50)
            .background(.bar)

            Picker(
                "Inspector",
                selection: Binding(
                    get: { store.workspace.selectedContinuity.inspector },
                    set: store.selectInspector
                )
            ) {
                ForEach(LocalProductInspectorTab.allCases) { tab in
                    Text(tab.rawValue).tag(tab)
                }
            }
            .pickerStyle(.segmented)
            .labelsHidden()
            .padding(12)

            Divider()
            ScrollView {
                inspectorContent
                    .padding(16)
            }
        }
        .background(Color(nsColor: .controlBackgroundColor))
    }

    @ViewBuilder
    private var inspectorContent: some View {
        switch store.workspace.selectedContinuity.inspector {
        case .team:
            inspectorSection(
                title: "Team",
                detail: store.builderSession?.preview.name.isEmpty == false
                    ? store.builderSession?.preview.name ?? "New team"
                    : "Build a team for this task",
                symbol: "person.3"
            )
            if let setup = store.setupSnapshot {
                VStack(alignment: .leading, spacing: 8) {
                    providerLine(
                        name: "Codex",
                        status: setup.codex.status
                    )
                    providerLine(
                        name: "MiniMax",
                        status: setup.miniMax.status
                    )
                }
                .padding(.top, 14)
            }
        case .context:
            inspectorSection(
                title: "Context",
                detail: "Only approved local resource pointers appear here.",
                symbol: "doc.text.magnifyingglass"
            )
        case .changes:
            inspectorSection(
                title: "Changes",
                detail: "No work has started yet.",
                symbol: "arrow.triangle.branch"
            )
        case .evidence:
            inspectorSection(
                title: "Evidence",
                detail: "Accepted results will appear after future work runs.",
                symbol: "checkmark.seal"
            )
        }

        DisclosureGroup(
            "Developer details",
            isExpanded: Binding(
                get: {
                    store.workspace.selectedContinuity
                        .developerDetailsExpanded
                },
                set: store.setDeveloperDetailsExpanded
            )
        ) {
            Text("Local diagnostic identifiers are hidden in normal use.")
                .font(.caption)
                .foregroundStyle(.secondary)
                .padding(.top, 8)
        }
        .padding(.top, 20)
    }

    private func inspectorSection(
        title: String,
        detail: String,
        symbol: String
    ) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Label(title, systemImage: symbol)
                .font(.headline)
            Text(detail)
                .font(.callout)
                .foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private func providerLine(name: String, status: String) -> some View {
        HStack {
            Circle()
                .fill(status == "available" || status == "verified"
                    ? Color.green
                    : Color.secondary)
                .frame(width: 7, height: 7)
            Text(name)
            Spacer()
            Text(humanized(status))
                .font(.caption)
                .foregroundStyle(.secondary)
        }
    }

    private func taskIcon(_ kind: LocalProductWorkspaceTaskKind) -> String {
        switch kind {
        case .draft: return "square.and.pencil"
        case .history: return "clock"
        case .team: return "person.3"
        case .attention: return "exclamationmark.circle"
        }
    }

    private var sidebar: some View {
        VStack(spacing: 0) {
            VStack(alignment: .leading, spacing: 3) {
                Text("Loom")
                    .font(.title2.weight(.bold))
                Text("Local agent workspace")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.horizontal, 16)
            .padding(.top, 22)
            .padding(.bottom, 14)

            VStack(spacing: 4) {
                ForEach(LocalProductSection.allCases) { section in
                    sidebarButton(section)
                }
            }
            .padding(.horizontal, 10)

            Spacer(minLength: 12)
            Divider()
            sidebarStatus
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(Color(nsColor: .underPageBackgroundColor))
        .frame(minWidth: 180, idealWidth: 210, maxWidth: 250)
        .accessibilityLabel("Loom workspace")
    }

    private func sidebarButton(
        _ section: LocalProductSection
    ) -> some View {
        let selected = store.selectedSection == section
        return Button {
            store.selectedSection = section
        } label: {
            HStack(spacing: 10) {
                Image(systemName: icon(for: section))
                    .frame(width: 20)
                    .accessibilityHidden(true)
                Text(section.rawValue)
                    .font(.body.weight(selected ? .semibold : .regular))
                Spacer(minLength: 0)
            }
            .foregroundStyle(selected ? Color.accentColor : Color.primary)
            .padding(.horizontal, 10)
            .frame(
                maxWidth: .infinity,
                minHeight: LoomDesign.minimumActionTarget,
                alignment: .leading
            )
            .contentShape(Rectangle())
            .background(
                selected ? Color.accentColor.opacity(0.14) : Color.clear,
                in: RoundedRectangle(cornerRadius: 10, style: .continuous)
            )
        }
        .buttonStyle(.plain)
        .accessibilityLabel(section.rawValue)
        .accessibilityAddTraits(selected ? .isSelected : [])
    }

    private var sidebarStatus: some View {
        HStack(spacing: 9) {
            Image(systemName: experience.connection.systemImage)
                .foregroundStyle(statusColor)
                .accessibilityHidden(true)
            Text(experience.connection.title)
                .font(.caption)
                .foregroundStyle(.secondary)
            Spacer(minLength: 0)
        }
        .padding(.horizontal, 14)
        .padding(.vertical, 12)
        .background(.bar)
        .accessibilityElement(children: .combine)
    }

    private var detail: some View {
        VStack(spacing: 0) {
            if experience.state != .connectedEmpty,
               experience.state != .connectedPopulated {
                connectionStrip
            }
            content
        }
        .background(Color(nsColor: .windowBackgroundColor))
        .navigationTitle(store.selectedSection.rawValue)
        .toolbar {
            if store.selectedSection == .teams {
                ToolbarItem {
                    Button {
                        showTeamBuilder = true
                    } label: {
                        Label("Create team", systemImage: "plus")
                    }
                    .accessibilityHint(
                        "Opens Team Builder"
                    )
                }
            }
            ToolbarItem {
                refreshButton
            }
        }
    }

    @ViewBuilder
    private var content: some View {
        switch store.selectedSection {
        case .home:
            home
        case .work:
            work
        case .teams:
            teams
        case .inbox:
            inbox
        case .system:
            system
        case .execution:
            execution
        case .production:
            production
        }
    }

    private var execution: some View {
        WorkspaceScroll {
            PageHeader(
                title: "Execution",
                subtitle: "Bounded execution adapter · journal-authoritative tool executions"
            )
            if let snapshot = store.executionSnapshot {
                ExecutionExplorerView(snapshot: snapshot)
            } else {
                EmptyPanel(
                    title: "Execution unavailable",
                    detail: "The daemon did not return an execution snapshot."
                )
            }
        }
        .task {
            await store.refreshExecutions()
        }
    }

    private var production: some View {
        WorkspaceScroll {
            PageHeader(
                title: "Production",
                subtitle: "Resident daemon activation and recovery status"
            )
            if let snapshot = store.productionSnapshot {
                ProductionStatusView(snapshot: snapshot)
            } else {
                EmptyPanel(
                    title: "Production unavailable",
                    detail: "The daemon did not return a production snapshot."
                )
            }
        }
        .task {
            await store.refreshProduction()
        }
    }

    private var teamBuilder: some View {
        WorkspaceScroll {
            PageHeader(
                title: "Team Builder",
                subtitle:
                    "Build a team, review its setup, then confirm."
            )
            setupProviderPanel
            setupRuntimePanel
            setupAssetsPanel
            builderPanel
        }
        .task {
            if store.setupSnapshot == nil {
                await store.refreshSetup()
            }
        }
    }

    @ViewBuilder
    private var setupProviderPanel: some View {
        ProviderConnectionDirectory(store: store)
    }

    @ViewBuilder
    private var setupAssetsPanel: some View {
        if let setup = store.setupSnapshot,
           !setup.savedTeams.isEmpty || !setup.templates.isEmpty
        {
            SectionHeading(
                title: "Saved teams and templates",
                detail: "Every selection opens a review before saving"
            )
            LoomPanel {
                VStack(alignment: .leading, spacing: 12) {
                    ForEach(setup.savedTeams) { team in
                        HStack {
                            VStack(alignment: .leading, spacing: 3) {
                                Text(team.name).font(.headline)
                                Text("Saved team · \(humanized(team.status))")
                                    .foregroundStyle(.secondary)
                            }
                            Spacer()
                            if team.status == "active" {
                                Button("Open in Builder") {
                                    Task { await store.startBuilder(from: team) }
                                }
                                Button("Archive") {
                                    Task { await store.archiveTeam(team) }
                                }
                            } else {
                                Button("Restore") {
                                    Task { await store.restoreTeam(team) }
                                }
                            }
                        }
                        .frame(minHeight: LoomDesign.minimumActionTarget)
                    }
                    if !setup.savedTeams.isEmpty && !setup.templates.isEmpty {
                        Divider()
                    }
                    ForEach(setup.templates) { template in
                        HStack {
                            VStack(alignment: .leading, spacing: 3) {
                                Text(template.name).font(.headline)
                                Text("Versioned template")
                                    .foregroundStyle(.secondary)
                            }
                            Spacer()
                            Button("Use Template") {
                                Task {
                                    await store.startBuilder(from: template)
                                }
                            }
                        }
                        .frame(minHeight: LoomDesign.minimumActionTarget)
                    }
                }
            }
        }
    }

    @ViewBuilder
    private var setupRuntimePanel: some View {
        if let setup = store.setupSnapshot {
            SectionHeading(
                title: "Available runtimes",
                detail: "Exact local capabilities and models"
            )
            if setup.runtimes.isEmpty {
                EmptyPanel(
                    title: "No compatible runtime is available.",
                    detail: "Loom will not invent a binding."
                )
            } else {
                LoomPanel {
                    VStack(spacing: 0) {
                        ForEach(
                            Array(setup.runtimes.enumerated()),
                            id: \.element.id
                        ) { offset, runtime in
                            HStack {
                                VStack(alignment: .leading, spacing: 4) {
                                    Text(runtime.displayName)
                                        .font(.headline)
                                    Text(
                                        "\(runtime.adapterType) · " +
                                            "\(runtime.executableVersion) · " +
                                            humanized(runtime.status)
                                    )
                                    .foregroundStyle(.secondary)
                                }
                                Spacer()
                                Text(runtime.modelIDs.joined(separator: ", "))
                                    .foregroundStyle(.secondary)
                            }
                            .padding(.vertical, 7)
                            if offset < setup.runtimes.count - 1 {
                                Divider()
                            }
                        }
                    }
                }
            }
        }
    }

    @ViewBuilder
    private var builderPanel: some View {
        SectionHeading(
            title: "Team setup",
            detail: "Nothing starts until you review and confirm"
        )
        switch store.setupState {
        case .loading:
            LoadingPanel(label: "Loading Team Builder")
        case let .unavailable(reason), let .fatal(reason):
            RecoveryPanel(
                title: "Team Builder unavailable",
                detail: humanized(reason),
                systemImage: "exclamationmark.triangle"
            ) {
                Task { await store.refreshSetup() }
            }
        case .idle, .ready:
            if let session = store.builderSession {
                LoomPanel {
                    VStack(alignment: .leading, spacing: 14) {
                        Text(
                            session.preview.name.isEmpty
                                ? "New team"
                                : session.preview.name
                        )
                        .font(.title3.weight(.semibold))
                        if !session.question.prompt.isEmpty {
                            Text(session.question.prompt)
                                .font(.headline)
                            if session.question.options.isEmpty {
                                TextField(
                                    "Answer",
                                    text: resolvedBuilderAnswerBinding
                                )
                                    .textFieldStyle(.roundedBorder)
                                Button("Continue") {
                                    let answer = resolvedBuilderAnswer
                                    builderAnswer = ""
                                    Task {
                                        await store.answerBuilder(answer)
                                    }
                                }
                                .disabled(resolvedBuilderAnswer.isEmpty)
                                .frame(
                                    minHeight: LoomDesign.minimumActionTarget
                                )
                            } else {
                                ForEach(session.question.options) { option in
                                    Button {
                                        Task {
                                            await store.answerBuilder(option.id)
                                        }
                                    } label: {
                                        if let choice = roleOptionChoices(
                                            session.preview
                                        ).first(where: { $0.id == option.id }) {
                                            roleOptionLabel(
                                                choice,
                                                fallback: option.label
                                            )
                                        } else {
                                            Text(option.label)
                                        }
                                    }
                                    .frame(
                                        minHeight: LoomDesign.minimumActionTarget
                                    )
                                }
                            }
                            Button("Cancel setup", role: .cancel) {
                                builderAnswer = ""
                                store.cancelBuilder()
                            }
                            .frame(minHeight: LoomDesign.minimumActionTarget)
                        } else if session.canConfirm {
                            Text(session.preview.purpose)
                                .foregroundStyle(.secondary)
                            builderPreflight(session.preview)
                            builderRoleChoices(session.preview)
                            HStack {
                                Button("Edit name") {
                                    builderEditField = "team_name"
                                    builderEditValue = session.preview.name
                                }
                                Button("Edit purpose") {
                                    builderEditField = "purpose"
                                    builderEditValue = session.preview.purpose
                                }
                                Button("Cancel setup", role: .cancel) {
                                    builderEditField = ""
                                    builderEditValue = ""
                                    store.cancelBuilder()
                                }
                            }
                            .frame(minHeight: LoomDesign.minimumActionTarget)
                            if !builderEditField.isEmpty {
                                TextField(
                                    builderEditField == "team_name"
                                        ? "Team name"
                                        : "Purpose",
                                    text: $builderEditValue
                                )
                                .textFieldStyle(.roundedBorder)
                                HStack {
                                    Button("Save edit") {
                                        let field = builderEditField
                                        let value = builderEditValue
                                        builderEditField = ""
                                        builderEditValue = ""
                                        Task {
                                            await store.editBuilder(
                                                field: field,
                                                value: value
                                            )
                                        }
                                    }
                                    .disabled(builderEditValue.isEmpty)
                                    Button("Back", role: .cancel) {
                                        builderEditField = ""
                                        builderEditValue = ""
                                    }
                                }
                                .frame(minHeight: LoomDesign.minimumActionTarget)
                            }
                            Button("Confirm saved team") {
                                Task { await store.confirmBuilder() }
                            }
                            .buttonStyle(.borderedProminent)
                            .frame(minHeight: LoomDesign.minimumActionTarget)
                            .accessibilityHint(
                                "Saves the team and does not start work"
                            )
                        }
                    }
                }
            } else {
                LoomPanel {
                    VStack(alignment: .leading, spacing: 10) {
                        Text("Start from a blank team")
                            .font(.headline)
                        Text(
                            "Loom asks one bounded question at a time and " +
                                "shows provider, model, permissions, and cost."
                        )
                        .foregroundStyle(.secondary)
                        Button("Build a team") {
                            Task { await store.startBlankBuilder() }
                        }
                        .buttonStyle(.borderedProminent)
                        .frame(minHeight: LoomDesign.minimumActionTarget)
                    }
                }
            }
        }
    }

    private var refreshButton: some View {
        Button {
            Task { await store.refresh() }
        } label: {
            Label(LocalProductCopy.refresh, systemImage: "arrow.clockwise")
                .frame(
                    minWidth: LoomDesign.minimumActionTarget,
                    minHeight: LoomDesign.minimumActionTarget
                )
        }
        .keyboardShortcut("r", modifiers: .command)
        .accessibilityLabel("Refresh Loom")
        .accessibilityHint("Reloads the bounded read-only local view")
    }

    private func builderPreflight(
        _ preview: LocalProductBuilderPreview
    ) -> some View {
        let review = LocalProductPreflightReview(preview: preview)
        return VStack(alignment: .leading, spacing: 12) {
            Text("Review before saving")
                .font(.headline)
            ForEach(review.roles) { role in
                VStack(alignment: .leading, spacing: 4) {
                    Text("\(role.kind) · \(role.name)")
                        .font(.callout.weight(.semibold))
                    Text(
                        "\(role.provider) · \(role.model) · \(role.auth)"
                    )
                    .font(.callout)
                    Text(
                        "\(role.runtime) · \(role.compatibility)"
                    )
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    if !role.permissions.isEmpty {
                        Text("Permissions · \(role.permissions.joined(separator: ", "))")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                }
            }
            if !review.permissions.isEmpty {
                Text("Team permissions · \(review.permissions.joined(separator: ", "))")
                    .font(.callout)
            }
            Text("Compatibility · \(review.compatibility)")
                .font(.callout)
            HStack {
                Text("Maximum budget · \(review.maximumBudget)")
                Spacer()
                Text(review.maximumCost)
            }
            .font(.callout)
        }
        .padding(12)
        .background(
            Color.accentColor.opacity(0.06),
            in: RoundedRectangle(cornerRadius: 10, style: .continuous)
        )
        .accessibilityElement(children: .contain)
    }

    private func roleOptionChoices(
        _ preview: LocalProductBuilderPreview
    ) -> [LocalProductRoleOptionChoice] {
        guard let setup = store.setupSnapshot else { return [] }
        return LocalProductRoleOptionChoice.all(
            setup: setup,
            preview: preview
        )
    }

    @ViewBuilder
    private func builderRoleChoices(
        _ preview: LocalProductBuilderPreview
    ) -> some View {
        let choices = roleOptionChoices(preview)
        if !choices.isEmpty {
            VStack(alignment: .leading, spacing: 8) {
                Text("Choose roles")
                    .font(.headline)
                ForEach(choices) { choice in
                    Button {
                        Task {
                            await store.editBuilder(
                                field: choice.field,
                                value: choice.id
                            )
                        }
                    } label: {
                        roleOptionLabel(
                            choice,
                            fallback: choice.responsibility
                        )
                    }
                    .buttonStyle(.bordered)
                    .frame(minHeight: LoomDesign.minimumActionTarget)
                    .accessibilityHint(
                        choice.isCurrent
                            ? "Currently selected"
                            : "Updates the team preview without saving"
                    )
                }
            }
        }
    }

    private func roleOptionLabel(
        _ choice: LocalProductRoleOptionChoice,
        fallback: String
    ) -> some View {
        VStack(alignment: .leading, spacing: 2) {
            HStack {
                Text(choice.responsibility.isEmpty
                    ? fallback
                    : choice.responsibility)
                if choice.isCurrent {
                    Text("Selected")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
            }
            Text(choice.isCurrent
                ? "\(choice.provider) · \(choice.model) · " +
                    "\(choice.auth) · \(choice.runtime)"
                : "\(choice.model) · \(choice.runtime) · " +
                    "Select to review provider and sign-in")
            .font(.caption)
            .foregroundStyle(.secondary)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private var resolvedBuilderAnswer: String {
        if builderAnswer.isEmpty,
           store.builderSession?.question.id == "purpose" {
            return store.workspace.selectedContinuity.composerDraft
        }
        return builderAnswer
    }

    private var resolvedBuilderAnswerBinding: Binding<String> {
        Binding(
            get: { resolvedBuilderAnswer },
            set: { builderAnswer = $0 }
        )
    }

    private var connectionStrip: some View {
        HStack(spacing: 12) {
            Image(systemName: experience.connection.systemImage)
                .foregroundStyle(statusColor)
                .font(.title3)
                .accessibilityHidden(true)
            VStack(alignment: .leading, spacing: 2) {
                Text(experience.connection.title)
                    .font(.callout.weight(.semibold))
                Text(experience.connection.detail)
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            Spacer()
            if experience.connection.offersRefresh {
                Button(LocalProductCopy.refresh) {
                    Task { await store.refresh() }
                }
                .frame(minHeight: LoomDesign.minimumActionTarget)
                .accessibilityHint("Tries the bounded local read again")
            }
        }
        .padding(.horizontal, 24)
        .padding(.vertical, 10)
        .background(statusColor.opacity(0.08))
        .overlay(alignment: .bottom) {
            Divider()
        }
        .accessibilityElement(children: .contain)
    }

    private var home: some View {
        WorkspaceScroll {
            PageHeader(
                title: LocalProductCopy.homeTitle,
                subtitle: LocalProductCopy.homeSubtitle
            )

            if experience.state == .loading, store.snapshot == nil {
                LoadingPanel()
            } else if store.snapshot == nil {
                RecoveryPanel(
                    title: experience.connection.title,
                    detail: experience.connection.detail,
                    systemImage: experience.connection.systemImage
                ) {
                    Task { await store.refresh() }
                }
            } else {
                HomeAttentionSection(
                    attention: experience.attention
                )
                HomeWorkSection(
                    runs: Array(experience.workActivity.prefix(4))
                )
                HomeTeamsSection(
                    teams: Array(experience.teams.prefix(4)),
                    select: selectTeam
                )
                SystemReadinessPanel(
                    connection: experience.connection,
                    runtimes: experience.runtimes
                )
            }
        }
    }

    private var work: some View {
        WorkspaceScroll {
            PageHeader(
                title: LocalProductCopy.workTitle,
                subtitle: LocalProductCopy.workSubtitle
            )
            if store.snapshot == nil {
                RecoveryPanel(
                    title: experience.connection.title,
                    detail: experience.connection.detail,
                    systemImage: experience.connection.systemImage
                ) {
                    Task { await store.refresh() }
                }
            } else {
                SectionHeading(
                    title: LocalProductCopy.workActivity,
                    detail: "Source-ordered Runs in this view"
                )
                if experience.workActivity.isEmpty {
                    EmptyPanel(
                        title: LocalProductCopy.noWork,
                        detail: "Confirmed team activity will appear here."
                    )
                } else {
                    LoomPanel {
                        VStack(spacing: 0) {
                            ForEach(
                                Array(experience.workActivity.enumerated()),
                                id: \.element.id
                            ) { offset, run in
                                RunRow(run: run)
                                if offset < experience.workActivity.count - 1 {
                                    Divider()
                                }
                            }
                        }
                    }
                }

                SectionHeading(
                    title: LocalProductCopy.evidence,
                    detail: "Records accepted by Loom authority"
                )
                if experience.acceptedEvidence.isEmpty {
                    EmptyPanel(
                        title: LocalProductCopy.noEvidence,
                        detail: "Accepted evidence will stay attached to work."
                    )
                } else {
                    LoomPanel {
                        VStack(spacing: 0) {
                            ForEach(
                                Array(experience.acceptedEvidence.enumerated()),
                                id: \.element.id
                            ) { offset, _ in
                                EvidenceRow()
                                if offset <
                                    experience.acceptedEvidence.count - 1 {
                                    Divider()
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    private var teams: some View {
        VStack(spacing: 0) {
            PageHeader(
                title: LocalProductCopy.teamsTitle,
                subtitle: LocalProductCopy.teamsSubtitle
            )
            .padding(.horizontal, 28)
            .padding(.top, 28)
            .padding(.bottom, 20)

            if store.snapshot == nil {
                RecoveryPanel(
                    title: experience.connection.title,
                    detail: experience.connection.detail,
                    systemImage: experience.connection.systemImage
                ) {
                    Task { await store.refresh() }
                }
                .padding(.horizontal, 28)
                Spacer()
            } else if experience.teams.isEmpty {
                EmptyPanel(
                    title: LocalProductCopy.noTeams,
                    detail: "Confirmed teams will appear in this workspace."
                )
                .padding(.horizontal, 28)
                Spacer()
            } else {
                HSplitView {
                    ScrollView {
                        LazyVStack(spacing: 10) {
                            ForEach(experience.teams) { team in
                                TeamSelectionRow(
                                    team: team,
                                    selected: store.selectedTeamID ==
                                        team.teamInstanceID
                                ) {
                                    selectTeam(team)
                                }
                            }
                        }
                        .padding(20)
                    }
                    .frame(minWidth: 260, idealWidth: 300, maxWidth: 340)

                    timeline
                        .frame(maxWidth: .infinity, maxHeight: .infinity)
                }
            }
        }
    }

    @ViewBuilder
    private var timeline: some View {
        switch store.timelineState {
        case .idle:
            EmptyPanel(
                title: LocalProductCopy.selectTeam,
                detail: "The timeline stays read-only."
            )
            .padding(28)
        case .loading:
            LoadingPanel(label: "Loading team timeline")
                .padding(28)
        case .loaded:
            if let timeline = store.timeline {
                ScrollView {
                    VStack(alignment: .leading, spacing: 18) {
                        SectionHeading(
                            title: "Team timeline",
                            detail: "Authoritative status: " +
                                humanized(timeline.board.status)
                        )
                        if timeline.records.isEmpty {
                            EmptyPanel(
                                title: "No timeline activity is available.",
                                detail:
                                    "This team has no visible execution history."
                            )
                        } else {
                            LoomPanel {
                                VStack(spacing: 0) {
                                    ForEach(
                                        Array(timeline.records.enumerated()),
                                        id: \.element.id
                                    ) { offset, record in
                                        TimelineRow(record: record)
                                        if offset <
                                            timeline.records.count - 1 {
                                            Divider()
                                        }
                                    }
                                }
                            }
                        }
                    }
                    .padding(28)
                }
            } else {
                TimelineFailurePanel(fatal: false)
                    .padding(28)
            }
        case .unavailable:
            TimelineFailurePanel(fatal: false)
                .padding(28)
        case .fatal:
            TimelineFailurePanel(fatal: true)
                .padding(28)
        }
    }

    private var inbox: some View {
        WorkspaceScroll {
            PageHeader(
                title: LocalProductCopy.inboxTitle,
                subtitle: LocalProductCopy.inboxSubtitle
            )
            if store.snapshot == nil {
                RecoveryPanel(
                    title: experience.connection.title,
                    detail: experience.connection.detail,
                    systemImage: experience.connection.systemImage
                ) {
                    Task { await store.refresh() }
                }
            } else if experience.attention.isEmpty {
                EmptyPanel(
                    title: LocalProductCopy.noAttention,
                    detail: "Loom will place blocked decisions here."
                )
            } else {
                LoomPanel {
                    VStack(spacing: 0) {
                        ForEach(
                            Array(experience.attention.enumerated()),
                            id: \.element.id
                        ) { offset, item in
                            AttentionRow(item: item)
                            if offset < experience.attention.count - 1 {
                                Divider()
                            }
                        }
                    }
                }
            }
        }
    }

    private var system: some View {
        WorkspaceScroll {
            PageHeader(
                title: LocalProductCopy.systemTitle,
                subtitle: LocalProductCopy.systemSubtitle
            )
            SystemConnectionPanel(connection: experience.connection)
            SectionHeading(
                title: "Runtimes",
                detail: "Compute discovered by the local service"
            )
            if experience.runtimes.isEmpty {
                EmptyPanel(
                    title: LocalProductCopy.noRuntimes,
                    detail: "Runtime discovery remains controlled by Loom."
                )
            } else {
                LoomPanel {
                    VStack(spacing: 0) {
                        ForEach(
                            Array(experience.runtimes.enumerated()),
                            id: \.element.id
                        ) { offset, runtime in
                            RuntimeRow(runtime: runtime)
                            if offset < experience.runtimes.count - 1 {
                                Divider()
                            }
                        }
                    }
                }
            }
        }
    }

    private func selectTeam(_ team: LocalProductTeamSummary) {
        store.selectTeam(team)
        Task { await store.activateSelectedTeam() }
    }

    private var statusColor: Color {
        switch experience.state {
        case .connectedEmpty, .connectedPopulated:
            return .green
        case .loading:
            return .secondary
        case .partial, .stalePreserved:
            return .orange
        case .offline, .offlinePreserved, .fatal:
            return .red
        }
    }

    private func icon(for section: LocalProductSection) -> String {
        switch section {
        case .home: return "house"
        case .work: return "rectangle.stack"
        case .teams: return "person.3"
        case .inbox: return "tray"
        case .system: return "gearshape"
        case .execution: return "bolt"
        case .production: return "server.rack"
        }
    }
}

private struct WorkspaceScroll<Content: View>: View {
    @ViewBuilder let content: Content

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 22) {
                content
            }
            .frame(
                maxWidth: LoomDesign.contentWidth,
                alignment: .leading
            )
            .padding(28)
            .frame(maxWidth: .infinity)
        }
    }
}

private struct PageHeader: View {
    let title: String
    let subtitle: String

    var body: some View {
        VStack(alignment: .leading, spacing: 7) {
            Text(title)
                .font(.largeTitle.weight(.bold))
                .accessibilityAddTraits(.isHeader)
            Text(subtitle)
                .font(.title3)
                .foregroundStyle(.secondary)
        }
    }
}

private struct SectionHeading: View {
    let title: String
    let detail: String

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(title)
                .font(.title2.weight(.semibold))
                .accessibilityAddTraits(.isHeader)
            Text(detail)
                .font(.callout)
                .foregroundStyle(.secondary)
        }
        .padding(.top, 4)
    }
}

private struct LoomPanel<Content: View>: View {
    @ViewBuilder let content: Content

    var body: some View {
        content
            .padding(18)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(
                Color(nsColor: .controlBackgroundColor),
                in: RoundedRectangle(
                    cornerRadius: LoomDesign.panelRadius,
                    style: .continuous
                )
            )
            .overlay {
                RoundedRectangle(
                    cornerRadius: LoomDesign.panelRadius,
                    style: .continuous
                )
                .stroke(Color(nsColor: .separatorColor), lineWidth: 0.5)
            }
    }
}

struct ProviderConnectionDirectory: View {
    @ObservedObject var store: LocalProductStore
    @State private var miniMaxSecret = ""
    @State private var showCodexConnection = false
    @State private var showMiniMaxConnection = false
    @State private var codexConnectionTask: Task<Void, Never>?

    var body: some View {
        if let setup = store.setupSnapshot {
            let hasMiniMaxCredential =
                !setup.miniMax.credentialReference.isEmpty &&
                setup.miniMax.revision > 0 &&
                setup.miniMax.status != "revoked"
            let codexConnected = setup.codex.status == "available"
            SectionHeading(
                title: "Providers",
                detail: "Connect accounts once, then choose them in a team"
            )
            LoomPanel {
                VStack(spacing: 0) {
                    ProviderConnectionRow(
                        name: "Codex",
                        detail: "ChatGPT OAuth · managed by Codex",
                        status: humanized(setup.codex.status),
                        systemImage: "terminal.fill",
                        connected: codexConnected,
                        busy: setupIsLoading,
                        primaryAction: providerConnectionPrimaryAction(
                            provider: .codex,
                            connected: codexConnected
                        )
                    ) {
                        if codexConnected {
                            showCodexConnection = true
                        } else {
                            codexConnectionTask = Task {
                                await store.connectCodex()
                            }
                        }
                    }
                    Divider()
                    ProviderConnectionRow(
                        name: "MiniMax",
                        detail: "API key · stored in macOS Keychain",
                        status: humanized(setup.miniMax.status),
                        systemImage: "sparkles",
                        connected: hasMiniMaxCredential,
                        busy: setupIsLoading,
                        primaryAction: providerConnectionPrimaryAction(
                            provider: .miniMax,
                            connected: hasMiniMaxCredential
                        )
                    ) {
                        showMiniMaxConnection = true
                    }
                }
            }
            .sheet(isPresented: $showCodexConnection) {
                codexConnectionSheet(setup: setup)
            }
            .sheet(isPresented: $showMiniMaxConnection) {
                miniMaxConnectionSheet(
                    setup: setup,
                    hasCredential: hasMiniMaxCredential
                )
            }
            .onDisappear {
                codexConnectionTask?.cancel()
                codexConnectionTask = nil
            }
        }
    }

    private var setupIsLoading: Bool {
        if case .loading = store.setupState {
            return true
        }
        return false
    }

    private func codexConnectionSheet(
        setup: LocalProductSetupSnapshot
    ) -> some View {
        VStack(alignment: .leading, spacing: 20) {
            Label("Codex connection", systemImage: "terminal.fill")
                .font(.title2.weight(.semibold))
            Text(
                "Authentication is managed by the official Codex app. " +
                    "Loom checks connection status but never reads or stores " +
                    "your OAuth token."
            )
            .foregroundStyle(.secondary)
            LabeledContent("Status") {
                Text(humanized(setup.codex.status))
            }
            Divider()
            HStack {
                Button("Done") {
                    showCodexConnection = false
                }
                Spacer()
                Button("Refresh status") {
                    Task { await store.refreshSetup() }
                }
                .buttonStyle(.borderedProminent)
            }
        }
        .padding(24)
        .frame(width: 480)
        .accessibilityElement(children: .contain)
    }

    private func miniMaxConnectionSheet(
        setup: LocalProductSetupSnapshot,
        hasCredential: Bool
    ) -> some View {
        VStack(alignment: .leading, spacing: 18) {
            Label("MiniMax connection", systemImage: "sparkles")
                .font(.title2.weight(.semibold))
            Text(
                "The API key is stored in macOS Keychain. It is never added " +
                    "to teams, prompts, logs, or Loom state."
            )
            .foregroundStyle(.secondary)
            LabeledContent("Status") {
                Text(humanized(setup.miniMax.status))
            }
            SecureField("API key", text: $miniMaxSecret)
                .textFieldStyle(.roundedBorder)
                .privacySensitive()
                .accessibilityLabel("MiniMax API key")
            if let status = store.credentialStatus {
                Text("Last action · \(humanized(status.status))")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            Divider()
            HStack(spacing: 10) {
                Button("Done") {
                    miniMaxSecret = ""
                    showMiniMaxConnection = false
                }
                Spacer()
                if hasCredential {
                    Button("Test") {
                        Task { await store.verifyMiniMax() }
                    }
                    Button("Revoke", role: .destructive) {
                        miniMaxSecret = ""
                        Task { await store.revokeMiniMax() }
                    }
                    Button("Replace") {
                        let secret = miniMaxSecret
                        miniMaxSecret = ""
                        Task {
                            await store.replaceMiniMax(secret: secret)
                        }
                    }
                    .buttonStyle(.borderedProminent)
                    .disabled(miniMaxSecret.isEmpty)
                } else {
                    Button("Connect") {
                        let secret = miniMaxSecret
                        miniMaxSecret = ""
                        Task {
                            await store.configureMiniMax(secret: secret)
                        }
                    }
                    .buttonStyle(.borderedProminent)
                    .disabled(miniMaxSecret.isEmpty)
                }
            }
            .frame(minHeight: LoomDesign.minimumActionTarget)
        }
        .padding(24)
        .frame(width: 520)
        .onDisappear {
            miniMaxSecret = ""
        }
    }
}

private struct ProviderConnectionRow: View {
    let name: String
    let detail: String
    let status: String
    let systemImage: String
    let connected: Bool
    let busy: Bool
    let primaryAction: ProviderConnectionPrimaryAction
    let action: () -> Void

    var body: some View {
        HStack(spacing: 14) {
            Image(systemName: systemImage)
                .font(.title3.weight(.semibold))
                .foregroundStyle(Color.accentColor)
                .frame(width: 42, height: 42)
                .background(
                    Color.accentColor.opacity(0.12),
                    in: RoundedRectangle(cornerRadius: 11, style: .continuous)
                )
                .accessibilityHidden(true)
            VStack(alignment: .leading, spacing: 4) {
                Text(name)
                    .font(.headline)
                Text(detail)
                    .foregroundStyle(.secondary)
                    .lineLimit(2)
                Label(
                    status,
                    systemImage: connected
                        ? "checkmark.circle.fill"
                        : "circle.dashed"
                )
                .font(.caption)
                .foregroundStyle(connected ? Color.green : Color.secondary)
            }
            Spacer(minLength: 16)
            if busy {
                ProgressView()
                    .controlSize(.small)
                    .accessibilityLabel("Connecting \(name)")
            }
            switch primaryAction {
            case .connect:
                Button("Connect", action: action)
                    .buttonStyle(.borderedProminent)
                    .disabled(busy)
                    .accessibilityHint("Starts the reviewed \(name) connection")
            case .manage:
                Button("Manage", action: action)
                    .disabled(busy)
                    .accessibilityHint("Opens \(name) connection management")
            }
        }
        .padding(.vertical, 4)
        .frame(
            maxWidth: .infinity,
            minHeight: LoomDesign.minimumActionTarget,
            alignment: .leading
        )
    }
}

private struct EmptyPanel: View {
    let title: String
    let detail: String

    var body: some View {
        LoomPanel {
            VStack(alignment: .leading, spacing: 7) {
                Label(title, systemImage: "tray")
                    .font(.headline)
                Text(detail)
                    .foregroundStyle(.secondary)
            }
            .accessibilityElement(children: .combine)
        }
    }
}

private struct LoadingPanel: View {
    var label = "Loading your workspace"

    var body: some View {
        LoomPanel {
            HStack(spacing: 12) {
                ProgressView()
                    .controlSize(.small)
                Text(label)
                    .font(.headline)
            }
            .accessibilityElement(children: .combine)
        }
    }
}

private struct TimelineFailurePanel: View {
    let fatal: Bool

    var body: some View {
        EmptyPanel(
            title: fatal
                ? "Team timeline needs attention."
                : "Team timeline is unavailable.",
            detail: fatal
                ? "Check the local service before trying this team again."
                : "Select the team again after the local service recovers."
        )
    }
}

private struct RecoveryPanel: View {
    let title: String
    let detail: String
    let systemImage: String
    let refresh: () -> Void

    var body: some View {
        LoomPanel {
            HStack(alignment: .top, spacing: 14) {
                Image(systemName: systemImage)
                    .font(.title2)
                    .foregroundStyle(.red)
                    .accessibilityHidden(true)
                VStack(alignment: .leading, spacing: 8) {
                    Text(title)
                        .font(.title3.weight(.semibold))
                        .accessibilityAddTraits(.isHeader)
                    Text(detail)
                        .foregroundStyle(.secondary)
                    Button(LocalProductCopy.refresh, action: refresh)
                        .frame(
                            minHeight: LoomDesign.minimumActionTarget
                        )
                        .accessibilityHint(
                            "Tries the bounded local read again"
                        )
                }
            }
        }
    }
}

private struct HomeAttentionSection: View {
    let attention: [LocalProductAttention]

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            SectionHeading(
                title: "Needs your attention",
                detail: "Human decisions come first"
            )
            if attention.isEmpty {
                LoomPanel {
                    Label(
                        LocalProductCopy.noAttention,
                        systemImage: "checkmark.circle"
                    )
                    .foregroundStyle(.secondary)
                }
            } else {
                LoomPanel {
                    VStack(spacing: 0) {
                        ForEach(
                            Array(attention.prefix(3).enumerated()),
                            id: \.element.id
                        ) { offset, item in
                            AttentionRow(item: item)
                            if offset < min(attention.count, 3) - 1 {
                                Divider()
                            }
                        }
                    }
                }
            }
        }
    }
}

private struct HomeWorkSection: View {
    let runs: [LocalProductRunSummary]

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            SectionHeading(
                title: LocalProductCopy.workActivity,
                detail: "Source-ordered Runs in this view"
            )
            if runs.isEmpty {
                EmptyPanel(
                    title: LocalProductCopy.noWork,
                    detail: "Confirmed team activity will appear here."
                )
            } else {
                LoomPanel {
                    VStack(spacing: 0) {
                        ForEach(
                            Array(runs.enumerated()),
                            id: \.element.id
                        ) { offset, run in
                            RunRow(run: run)
                            if offset < runs.count - 1 {
                                Divider()
                            }
                        }
                    }
                }
            }
        }
    }
}

private struct HomeTeamsSection: View {
    let teams: [LocalProductTeamSummary]
    let select: (LocalProductTeamSummary) -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            SectionHeading(
                title: "Agent teams",
                detail: "Confirmed teams available to observe"
            )
            if teams.isEmpty {
                EmptyPanel(
                    title: LocalProductCopy.noTeams,
                    detail: "Confirmed teams will appear in this workspace."
                )
            } else {
                LoomPanel {
                    VStack(spacing: 0) {
                        ForEach(
                            Array(teams.enumerated()),
                            id: \.element.id
                        ) { offset, team in
                            Button {
                                select(team)
                            } label: {
                                TeamSummaryRow(team: team)
                            }
                            .buttonStyle(.plain)
                            .frame(
                                minHeight: LoomDesign.minimumActionTarget
                            )
                            if offset < teams.count - 1 {
                                Divider()
                            }
                        }
                    }
                }
            }
        }
    }
}

private struct SystemReadinessPanel: View {
    let connection: LocalProductConnectionPresentation
    let runtimes: [LocalProductRuntimeSummary]

    var body: some View {
        LoomPanel {
            HStack(spacing: 12) {
                Image(systemName: connection.systemImage)
                    .font(.title3)
                    .foregroundStyle(.secondary)
                    .accessibilityHidden(true)
                VStack(alignment: .leading, spacing: 3) {
                    Text("System readiness")
                        .font(.headline)
                    Text(
                        connection.title + ". " +
                            "\(runtimes.count) " +
                            (runtimes.count == 1 ? "runtime" : "runtimes") +
                            " visible."
                    )
                    .foregroundStyle(.secondary)
                }
            }
            .accessibilityElement(children: .combine)
        }
    }
}

private struct SystemConnectionPanel: View {
    let connection: LocalProductConnectionPresentation

    var body: some View {
        LoomPanel {
            HStack(alignment: .top, spacing: 14) {
                Image(systemName: connection.systemImage)
                    .font(.title2)
                    .foregroundStyle(.secondary)
                    .accessibilityHidden(true)
                VStack(alignment: .leading, spacing: 5) {
                    Text(connection.title)
                        .font(.headline)
                    Text(connection.detail)
                        .foregroundStyle(.secondary)
                }
            }
            .accessibilityElement(children: .combine)
        }
    }
}

private struct RunRow: View {
    let run: LocalProductRunSummary

    var body: some View {
        HStack(spacing: 12) {
            Image(systemName: run.terminalStatus.isEmpty
                ? "circle.dotted"
                : "checkmark.circle")
                .foregroundStyle(.secondary)
                .accessibilityHidden(true)
            VStack(alignment: .leading, spacing: 4) {
                Text(humanized(run.phase, fallback: "Work"))
                    .font(.headline)
                Text(
                    run.terminalStatus.isEmpty
                        ? "In progress"
                        : humanized(run.terminalStatus)
                )
                .foregroundStyle(.secondary)
            }
            Spacer()
        }
        .padding(.vertical, 8)
        .accessibilityElement(children: .combine)
    }
}

private struct EvidenceRow: View {
    var body: some View {
        HStack(spacing: 12) {
            Image(systemName: "checkmark.seal")
                .foregroundStyle(.secondary)
                .accessibilityHidden(true)
            VStack(alignment: .leading, spacing: 4) {
                Text("Accepted evidence")
                    .font(.headline)
                Text("Verified record attached to work")
                    .foregroundStyle(.secondary)
            }
            Spacer()
        }
        .padding(.vertical, 8)
        .accessibilityElement(children: .combine)
    }
}

private struct AttentionRow: View {
    let item: LocalProductAttention

    var body: some View {
        HStack(alignment: .top, spacing: 12) {
            Image(systemName: "exclamationmark.circle")
                .foregroundStyle(.orange)
                .accessibilityHidden(true)
            VStack(alignment: .leading, spacing: 4) {
                Text(action)
                    .font(.headline)
                Text("Human action required")
                    .foregroundStyle(.secondary)
            }
            Spacer()
        }
        .padding(.vertical, 8)
        .accessibilityElement(children: .combine)
    }

    private var action: String {
        let safe = SafeText.sanitize(item.actionRequired, limit: 160)
        return safe.isEmpty ? "Review this item" : safe
    }
}

private struct TeamSummaryRow: View {
    let team: LocalProductTeamSummary

    var body: some View {
        HStack(spacing: 12) {
            Image(systemName: "person.3")
                .foregroundStyle(.secondary)
                .accessibilityHidden(true)
            VStack(alignment: .leading, spacing: 4) {
                Text(LocalProductExperience.visibleName(
                    team.displayName,
                    internalID: team.teamInstanceID,
                    fallback: "Team"
                ))
                    .font(.headline)
                Text(humanized(team.state))
                    .foregroundStyle(.secondary)
            }
            Spacer()
            Image(systemName: "chevron.right")
                .font(.caption.weight(.semibold))
                .foregroundStyle(.tertiary)
                .accessibilityHidden(true)
        }
        .padding(.vertical, 8)
        .contentShape(Rectangle())
        .accessibilityElement(children: .combine)
    }
}

private struct TeamSelectionRow: View {
    let team: LocalProductTeamSummary
    let selected: Bool
    let select: () -> Void

    var body: some View {
        Button(action: select) {
            TeamSummaryRow(team: team)
                .padding(.horizontal, 14)
                .frame(
                    minHeight: LoomDesign.minimumActionTarget
                )
                .background(
                    selected
                        ? Color.accentColor.opacity(0.10)
                        : Color.clear,
                    in: RoundedRectangle(
                        cornerRadius: 12,
                        style: .continuous
                    )
                )
                .overlay {
                    if selected {
                        RoundedRectangle(
                            cornerRadius: 12,
                            style: .continuous
                        )
                        .stroke(
                            Color.accentColor.opacity(0.45),
                            lineWidth: 1
                        )
                    }
                }
        }
        .buttonStyle(.plain)
        .accessibilityAddTraits(selected ? .isSelected : [])
    }
}

private struct RuntimeRow: View {
    let runtime: LocalProductRuntimeSummary

    var body: some View {
        HStack(alignment: .top, spacing: 12) {
            Image(systemName: "cpu")
                .foregroundStyle(.secondary)
                .accessibilityHidden(true)
            VStack(alignment: .leading, spacing: 4) {
                Text(LocalProductExperience.visibleName(
                    runtime.displayName,
                    internalID: runtime.runtimeInstanceID,
                    fallback: "Runtime"
                ))
                    .font(.headline)
                Text(
                    humanized(runtime.status) + " · " +
                        humanized(runtime.adapterType) + " · " +
                        "Capacity \(runtime.capacity)"
                )
                .foregroundStyle(.secondary)
                if !runtime.executableVersion.isEmpty {
                    Text(
                        "Version " +
                            SafeText.sanitize(
                                runtime.executableVersion,
                                limit: 64
                            )
                    )
                    .font(.caption)
                    .foregroundStyle(.tertiary)
                }
            }
            Spacer()
        }
        .padding(.vertical, 8)
        .accessibilityElement(children: .combine)
    }
}

private struct TimelineRow: View {
    let record: LocalProductTimelineRecord

    var body: some View {
        HStack(alignment: .top, spacing: 12) {
            Image(systemName: "text.bubble")
                .foregroundStyle(.secondary)
                .accessibilityHidden(true)
            VStack(alignment: .leading, spacing: 4) {
                Text(humanized(record.kind, fallback: "Team update"))
                    .font(.headline)
                Text(message)
                    .foregroundStyle(.secondary)
            }
            Spacer()
        }
        .padding(.vertical, 8)
        .accessibilityElement(children: .combine)
    }

    private var message: String {
        let source = record.payload.textDelta.isEmpty
            ? record.payload.status
            : record.payload.textDelta
        let safe = SafeText.sanitize(source, limit: 512)
        return safe.isEmpty ? "Status updated" : safe
    }
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
