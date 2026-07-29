import SwiftUI
import LoomLocalAppCore

public enum LoomDesign {
    public static let panelRadius: CGFloat = 16
    public static let minimumActionTarget: CGFloat = 44
    public static let contentWidth: CGFloat = 920
}

public struct ContentView: View {
    @ObservedObject private var store: LocalProductStore
    private let refreshOnAppear: Bool
    @State private var builderAnswer = ""
    @State private var builderEditField = ""
    @State private var builderEditValue = ""
    @State private var miniMaxSecret = ""
    @State private var showTeamBuilder = false

    public init(
        store: LocalProductStore,
        refreshOnAppear: Bool = true
    ) {
        self.store = store
        self.refreshOnAppear = refreshOnAppear
    }

    public var body: some View {
        HSplitView {
            sidebar
            NavigationStack {
                detail
            }
            .frame(minWidth: 560)
        }
        .task {
            if refreshOnAppear {
                await store.refresh()
            }
        }
        .sheet(isPresented: $showTeamBuilder) {
            teamBuilder
                .frame(minWidth: 760, minHeight: 620)
        }
    }

    private var experience: LocalProductExperience {
        LocalProductExperience(
            snapshot: store.snapshot,
            connectionState: store.connectionState
        )
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
                        "Opens the Candidate Team Builder"
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
        }
    }

    private var teamBuilder: some View {
        WorkspaceScroll {
            PageHeader(
                title: "Team Builder",
                subtitle:
                    "Create a Candidate team, review exact bindings, then confirm."
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
        if let setup = store.setupSnapshot {
            let hasMiniMaxCredential =
                !setup.miniMax.credentialReference.isEmpty &&
                setup.miniMax.revision > 0 &&
                setup.miniMax.status != "revoked"
            SectionHeading(
                title: "Providers",
                detail: "Authentication stays outside Team and Run authority"
            )
            LoomPanel {
                VStack(alignment: .leading, spacing: 14) {
                    Label(
                        "Codex · \(humanized(setup.codex.status)) · " +
                            setup.codex.authMode,
                        systemImage: "person.crop.circle.badge.checkmark"
                    )
                    Divider()
                    HStack(alignment: .center, spacing: 12) {
                        VStack(alignment: .leading, spacing: 4) {
                            Text("MiniMax")
                                .font(.headline)
                            Text(
                                "\(humanized(setup.miniMax.status)) · " +
                                    setup.miniMax.authMode
                            )
                            .foregroundStyle(.secondary)
                        }
                        Spacer()
                        SecureField("API key", text: $miniMaxSecret)
                            .textFieldStyle(.roundedBorder)
                            .frame(maxWidth: 280)
                            .privacySensitive()
                            .accessibilityLabel("MiniMax API key")
                        if hasMiniMaxCredential {
                            Button("Test") {
                                Task { await store.verifyMiniMax() }
                            }
                            .frame(minHeight: LoomDesign.minimumActionTarget)
                            Button("Replace") {
                                let secret = miniMaxSecret
                                miniMaxSecret = ""
                                Task {
                                    await store.replaceMiniMax(secret: secret)
                                }
                            }
                            .disabled(miniMaxSecret.isEmpty)
                            .frame(minHeight: LoomDesign.minimumActionTarget)
                            Button("Revoke", role: .destructive) {
                                miniMaxSecret = ""
                                Task { await store.revokeMiniMax() }
                            }
                            .frame(minHeight: LoomDesign.minimumActionTarget)
                        } else {
                            Button("Store securely") {
                                let secret = miniMaxSecret
                                miniMaxSecret = ""
                                Task {
                                    await store.configureMiniMax(secret: secret)
                                }
                            }
                            .disabled(miniMaxSecret.isEmpty)
                            .frame(minHeight: LoomDesign.minimumActionTarget)
                        }
                    }
                    if let status = store.credentialStatus {
                        Text(
                            "MiniMax credential · " +
                                humanized(status.status)
                        )
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    }
                }
            }
        }
    }

    @ViewBuilder
    private var setupAssetsPanel: some View {
        if let setup = store.setupSnapshot,
           !setup.savedTeams.isEmpty || !setup.templates.isEmpty
        {
            SectionHeading(
                title: "Saved teams and templates",
                detail: "Every selection opens a Candidate for review"
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
                                Button("Open Candidate") {
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
            title: "Candidate team",
            detail: "Nothing executes before a later explicit Run action"
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
                                TextField("Answer", text: $builderAnswer)
                                    .textFieldStyle(.roundedBorder)
                                Button("Continue") {
                                    let answer = builderAnswer
                                    builderAnswer = ""
                                    Task {
                                        await store.answerBuilder(answer)
                                    }
                                }
                                .disabled(builderAnswer.isEmpty)
                                .frame(
                                    minHeight: LoomDesign.minimumActionTarget
                                )
                            } else {
                                ForEach(session.question.options) { option in
                                    Button(option.label) {
                                        Task {
                                            await store.answerBuilder(option.id)
                                        }
                                    }
                                    .frame(
                                        minHeight: LoomDesign.minimumActionTarget
                                    )
                                }
                            }
                            Button("Cancel Candidate", role: .cancel) {
                                builderAnswer = ""
                                store.cancelBuilder()
                            }
                            .frame(minHeight: LoomDesign.minimumActionTarget)
                        } else if session.canConfirm {
                            Text(session.preview.purpose)
                                .foregroundStyle(.secondary)
                            Text(session.preview.estimatedMaximumCost)
                                .font(.callout)
                                .foregroundStyle(.secondary)
                            HStack {
                                Button("Edit name") {
                                    builderEditField = "team_name"
                                    builderEditValue = session.preview.name
                                }
                                Button("Edit purpose") {
                                    builderEditField = "purpose"
                                    builderEditValue = session.preview.purpose
                                }
                                Button("Cancel Candidate", role: .cancel) {
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
                                "Saves the TeamDefinition and does not start a Run"
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
                                "shows exact runtime and cost bindings."
                        )
                        .foregroundStyle(.secondary)
                        Button("Create Candidate team") {
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
