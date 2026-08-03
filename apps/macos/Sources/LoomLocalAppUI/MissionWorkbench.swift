import LoomLocalAppCore
import SwiftUI

func missionRuntimeDisplayName(
    run: LocalProductRunSummary,
    snapshot: LocalProductSnapshot
) -> String {
    guard let runtime = snapshot.runtimes.first(where: {
        $0.runtimeInstanceID == run.runtimeInstanceID
    }) else {
        return "Runtime unavailable"
    }
    return LocalProductExperience.visibleName(
        runtime.displayName,
        internalID: runtime.runtimeInstanceID,
        fallback: "Runtime unavailable"
    )
}

func missionDisplayTitle(
    candidate: String,
    missionID: String,
    teamInstanceID: String,
    teams: [LocalProductTeamSummary]
) -> String {
    let safeCandidate = SafeText.sanitize(candidate, limit: 96)
    let safeMissionID = SafeText.sanitize(missionID, limit: 96)
    let safeTeamID = SafeText.sanitize(teamInstanceID, limit: 96)
    if !safeCandidate.isEmpty,
        safeCandidate != safeMissionID,
        safeCandidate != safeTeamID
    {
        return safeCandidate
    }
    guard let team = teams.first(where: {
        $0.teamInstanceID == teamInstanceID
    }) else {
        return "Mission"
    }
    return LocalProductExperience.visibleName(
        team.displayName,
        internalID: team.teamInstanceID,
        fallback: "Mission"
    )
}

func missionNodeDisplayTitle(
    nodeID: String,
    topology: [LocalProductMissionNode],
    pulse: [LocalProductMissionPulse]
) -> String {
    if let node = topology.first(where: { $0.logicalNodeID == nodeID }) {
        let title = LocalProductExperience.visibleName(
            node.title,
            internalID: node.logicalNodeID,
            fallback: ""
        )
        if !title.isEmpty { return title }
        let role = SafeText.sanitize(node.role, limit: 48)
        if !role.isEmpty { return role.capitalized }
    }
    if let role = pulse.first(where: { $0.nodeID == nodeID })?.role {
        let safeRole = SafeText.sanitize(role, limit: 48)
        if !safeRole.isEmpty { return safeRole.capitalized }
    }
    return nodeID.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
        ? "No current step"
        : "Mission step"
}

func missionHumanStatus(_ value: String) -> String {
    value.replacingOccurrences(of: "_", with: " ").capitalized
}

struct SideTaskDrawerPresentation: Equatable {
    let purpose: String
    let uncertainty: String
    let scopeDelta: String
    let nextAction: String
}

func sideTaskDrawerPresentation(
    _ sideTask: LocalProductSideTaskSummary
) -> SideTaskDrawerPresentation {
    let uncertainty = sideTask.uncertainties.isEmpty
        ? "None recorded"
        : sideTask.uncertainties.joined(separator: "; ")
    let scopeDelta = sideTask.scopeDelta.isEmpty
        ? "No parent scope expansion"
        : sideTask.scopeDelta.joined(separator: "; ")
    let nextAction: String
    if !sideTask.availableDecisions.isEmpty {
        nextAction = "Choose an explicit parent decision"
    } else if sideTask.status == "report_delivered" {
        nextAction = "Review the authorized report"
    } else if sideTask.effectStatus == "pending" {
        nextAction = "Wait for the authorized parent effect"
    } else if sideTask.effectStatus == "completed" {
        nextAction = "Parent effect completed"
    } else if sideTask.status == "decided" {
        nextAction = "Parent decision recorded; no parent effect required"
    } else if sideTask.status == "admitted" || sideTask.status == "running" {
        nextAction = "Wait for the authorized handoff"
    } else {
        nextAction = "Review the authoritative Side-task status"
    }
    return SideTaskDrawerPresentation(
        purpose: "Purpose · \(missionHumanStatus(sideTask.purpose))",
        uncertainty: "Uncertainty · \(uncertainty)",
        scopeDelta: "Scope · \(scopeDelta)",
        nextAction: "Next action · \(nextAction)"
    )
}

enum MissionSnapshotPresentation: Equatable {
    case current
    case partial(String)
    case preserved(String)
    case unavailable(String)

    var isCurrent: Bool { self == .current }

    var notice: String? {
        switch self {
        case .current:
            return nil
        case .partial(let reason):
            return "Partial view · \(reason). Some items may be missing."
        case .preserved(let reason):
            return "Showing a preserved view · \(reason). New items may be missing."
        case .unavailable(let reason):
            return "Current view unavailable · \(reason)."
        }
    }
}

func missionSnapshotPresentation(
    connection: LocalProductConnectionState,
    hasSnapshot: Bool
) -> MissionSnapshotPresentation {
    guard hasSnapshot else {
        switch connection {
        case .partial(let reason), .stale(let reason),
            .offline(let reason), .fatal(let reason):
            return .unavailable(reason)
        case .loading:
            return .unavailable("loading")
        case .online:
            return .unavailable("state_unavailable")
        }
    }
    switch connection {
    case .online:
        return .current
    case .partial(let reason):
        return .partial(reason)
    case .stale(let reason), .offline(let reason), .fatal(let reason):
        return .preserved(reason)
    case .loading:
        return .preserved("refreshing")
    }
}

struct MissionAttentionEmptyCopy: Equatable {
    let icon: String
    let title: String
    let detail: String
}

func missionAttentionEmptyCopy(
    presentation: MissionSnapshotPresentation
) -> MissionAttentionEmptyCopy {
    if presentation.isCurrent {
        return MissionAttentionEmptyCopy(
            icon: "checkmark.circle",
            title: "Nothing needs you",
            detail: "No approval, blocked, retry-exhausted or verification action is pending."
        )
    }
    return MissionAttentionEmptyCopy(
        icon: "clock.arrow.circlepath",
        title: "No recorded attention in this view",
        detail: presentation.notice ?? "Current Attention is unavailable."
    )
}

struct MissionInspectorSection: Equatable {
    let heading: String
    let rows: [String]
    let emptyMessage: String
}

func missionInspectorSection(
    tab: MissionInspectorTab,
    record: LocalProductMissionSummary?,
    timeline: LocalProductTimelinePage?
) -> MissionInspectorSection {
    switch tab {
    case .team:
        let rows = (record?.teamPulse ?? []).map {
            "\($0.role.capitalized) · \(missionHumanStatus($0.state)) · Attempt \($0.attemptNumber)"
        }
        return MissionInspectorSection(
            heading: "Team Pulse",
            rows: rows,
            emptyMessage: "No Team presence is available."
        )
    case .plan:
        let rows = (record?.topology ?? []).map { node in
            let title = missionNodeDisplayTitle(
                nodeID: node.logicalNodeID,
                topology: record?.topology ?? [],
                pulse: record?.teamPulse ?? []
            )
            let attempt = node.attemptNumber > 0
                ? " · Attempt \(node.attemptNumber)"
                : ""
            return "\(title) · \(missionHumanStatus(node.status))\(attempt)"
        }
        return MissionInspectorSection(
            heading: "Plan",
            rows: rows,
            emptyMessage: "No authoritative Plan is available."
        )
    case .changes:
        guard let record,
              let timeline,
              timeline.teamInstanceID == record.teamInstanceID,
              timeline.gap == nil,
              !timeline.hasMore else {
            return MissionInspectorSection(
                heading: "Changes",
                rows: [],
                emptyMessage: "Changes are unavailable until complete authoritative activity loads."
            )
        }
        let labels = [
            "ready_for_review": "Ready for review",
            "verification_recorded": "Verification recorded",
            "verification_rejected": "Verification rejected",
            "node_acceptance": "Acceptance recorded",
        ]
        let rows = timeline.records.compactMap { item -> String? in
            guard item.authority == "journal",
                  item.teamInstanceID == record.teamInstanceID,
                  let label = labels[item.kind] else { return nil }
            let node = missionNodeDisplayTitle(
                nodeID: item.logicalNodeID,
                topology: record.topology,
                pulse: record.teamPulse
            )
            let attempt = item.attemptNumber > 0
                ? " · Attempt \(item.attemptNumber)"
                : ""
            return "\(label) · \(node)\(attempt)"
        }
        return MissionInspectorSection(
            heading: "Changes",
            rows: rows,
            emptyMessage: "No review or acceptance changes are recorded."
        )
    case .evidence:
        guard let record,
              let timeline,
              timeline.teamInstanceID == record.teamInstanceID,
              timeline.gap == nil,
              !timeline.hasMore else {
            return MissionInspectorSection(
                heading: "Evidence",
                rows: [],
                emptyMessage: "Evidence is unavailable until complete authoritative activity loads."
            )
        }
        var ordinal = 0
        let rows = timeline.records.compactMap { item -> String? in
            guard item.kind == "evidence_available",
                  item.authority == "journal",
                  item.teamInstanceID == record.teamInstanceID else {
                return nil
            }
            ordinal += 1
            let node = missionNodeDisplayTitle(
                nodeID: item.logicalNodeID,
                topology: record.topology,
                pulse: record.teamPulse
            )
            let attempt = item.attemptNumber > 0
                ? " · Attempt \(item.attemptNumber)"
                : ""
            return "Evidence \(ordinal) · \(node)\(attempt)"
        }
        return MissionInspectorSection(
            heading: "Evidence",
            rows: rows,
            emptyMessage: "No Evidence reference is recorded."
        )
    }
}

public struct MissionWorkbench: View {
    @ObservedObject private var store: LocalProductStore
    @State private var showProviders = false
    @State private var showNewMission = false
    @State private var showNewSideTask = false
    @State private var newMissionObjective = ""
    @State private var newMissionTeamID = ""
    @State private var newMissionWorkPackageID =
        LocalProductWorkPackageOption.coding.id
    @State private var sideTaskPurpose = "research"
    @State private var sideTaskMode = "report_only"
    @State private var sideTaskTitle = ""
    @State private var sideTaskRequest = ""

    public init(store: LocalProductStore) {
        self.store = store
    }

    public var body: some View {
        HSplitView {
            workbenchRail
                .frame(
                    minWidth: LoomGraphite.railWidth,
                    idealWidth: LoomGraphite.railWidth,
                    maxWidth: LoomGraphite.railWidth
                )
            switch store.workbench.route {
            case .board:
                orchestrationBoard
                    .frame(minWidth: 560)
            case .mission(let id):
                missionRoom(id)
                    .frame(minWidth: 560)
            case .teams:
                teamsWorkspace
                    .frame(minWidth: 560)
            case .attention:
                attentionWorkspace
                    .frame(minWidth: 560)
            case .library:
                libraryWorkspace
                    .frame(minWidth: 560)
            }
        }
        .background(LoomGraphite.canvas)
        .sheet(isPresented: $showProviders) {
            providerManagement
                .frame(minWidth: 560, minHeight: 380)
        }
        .sheet(isPresented: $showNewMission) {
            newMissionSheet
                .frame(minWidth: 620, minHeight: 520)
        }
        .sheet(isPresented: $showNewSideTask) {
            if case .mission(let missionID) = store.workbench.route {
                newSideTaskSheet(missionID)
                    .frame(minWidth: 560, minHeight: 500)
            }
        }
        .sheet(
            item: Binding(
                get: { store.activeDecisionSheet },
                set: { value in
                    if value == nil {
                        store.dismissDecisionSheet()
                    }
                }
            )
        ) { sheet in
            LoomDecisionSheet(
                sheet: sheet,
                onAction: { action in
                    Task { await store.submitDecisionAction(action) }
                },
                onClose: { store.dismissDecisionSheet() }
            )
            .frame(minWidth: 620, idealWidth: 680, minHeight: 520)
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel("Loom Mission orchestration workbench")
    }

    private var workbenchRail: some View {
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

            railButton("New Mission", systemImage: "plus.square") {
                if newMissionTeamID.isEmpty {
                    newMissionTeamID = executableTeams.first?.teamInstanceID ?? ""
                }
                showNewMission = true
            }
            railButton("My Missions", systemImage: "rectangle.3.group") {
                store.showMissionBoard()
            }
            railButton("Teams", systemImage: "person.3") {
                store.showMissionTeams()
            }
            railButton("Needs You", systemImage: "exclamationmark.bubble") {
                store.showMissionAttention()
            }

            Divider().padding(.vertical, 10)

            Text("WORKSPACE")
                .font(.caption2.weight(.semibold))
                .foregroundStyle(.secondary)
                .tracking(0.8)
                .padding(.horizontal, 14)
                .padding(.bottom, 4)
            railButton("Library", systemImage: "books.vertical") {
                store.showMissionLibrary()
            }
            railButton("Runtime & Providers", systemImage: "switch.2") {
                showProviders = true
            }

            Spacer()
            connectionFooter
        }
        .background(LoomGraphite.rail)
        .accessibilityLabel("Mission navigation")
    }

    private var executableTeams: [LocalProductTeamSummary] {
        (store.snapshot?.teams ?? []).filter {
            $0.confirmed && $0.executable && !$0.readOnly
        }
    }

    private var selectedWorkPackage: LocalProductWorkPackageOption {
        LocalProductWorkPackageOption.accepted.first {
            $0.id == newMissionWorkPackageID
        } ?? .coding
    }

    private var newMissionSheet: some View {
        VStack(alignment: .leading, spacing: 18) {
            HStack {
                VStack(alignment: .leading, spacing: 4) {
                    Text("New Mission")
                        .font(.title2.weight(.semibold))
                    Text("Choose the work and Team, review exact access, then start explicitly.")
                        .foregroundStyle(.secondary)
                }
                Spacer()
                Button("Close") { showNewMission = false }
            }

            Picker("Work type", selection: $newMissionWorkPackageID) {
                ForEach(LocalProductWorkPackageOption.accepted) { option in
                    Text(option.title).tag(option.id)
                }
            }
            .pickerStyle(.segmented)

            Picker("Team", selection: $newMissionTeamID) {
                if executableTeams.isEmpty {
                    Text("No confirmed Team available").tag("")
                }
                ForEach(executableTeams, id: \.teamInstanceID) { team in
                    Text(LocalProductExperience.visibleName(
                        team.displayName,
                        internalID: team.teamInstanceID,
                        fallback: "Confirmed Team"
                    ))
                    .tag(team.teamInstanceID)
                }
            }

            TextField(
                "What should this Mission accomplish?",
                text: $newMissionObjective,
                axis: .vertical
            )
            .lineLimit(4...8)
            .textFieldStyle(.roundedBorder)

            if let preflight = store.executionPreflight {
                GroupBox("Preflight") {
                    VStack(alignment: .leading, spacing: 8) {
                        Label(
                            "\(preflight.nodes.count) node · \(preflight.authMode) · \(preflight.modelID)",
                            systemImage: "checkmark.shield"
                        )
                        Text("Capacity available: \(preflight.capacityAvailable)")
                        Text("Permissions: \(preflight.permissionScopes.joined(separator: ", "))")
                        Text("Approvals: \(preflight.approvalPoints.joined(separator: ", "))")
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                }
            }

            Spacer()
            HStack {
                Text(executionStateLabel)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                Spacer()
                if store.executionState == .ready {
                    Button("Start Mission") {
                        Task {
                            await store.startPreflightedMission()
                            if store.executionState == .running
                                || store.executionState == .awaitingRecovery
                                || store.executionState == .succeeded
                            {
                                showNewMission = false
                            }
                        }
                    }
                    .buttonStyle(.borderedProminent)
                } else {
                    Button("Review preflight") {
                        guard
                            let team = executableTeams.first(where: {
                                $0.teamInstanceID == newMissionTeamID
                            })
                        else { return }
                        Task {
                            await store.preflightMission(
                                objective: newMissionObjective,
                                team: team,
                                workPackage: selectedWorkPackage
                            )
                        }
                    }
                    .buttonStyle(.borderedProminent)
                    .disabled(
                        newMissionObjective.trimmingCharacters(
                            in: .whitespacesAndNewlines
                        ).isEmpty || newMissionTeamID.isEmpty || !store.missionExecutionReachable
                    )
                }
            }
        }
        .padding(24)
    }

    private var executionStateLabel: String {
        switch store.executionState {
        case .idle: return "Nothing runs before you review preflight."
        case .preflighting: return "Checking Team, Runtime, capacity and access…"
        case .ready: return "Preflight is ready. Starting still requires your click."
        case .starting: return "Starting the authoritative execution…"
        case .cancelling: return "Cancelling the exact current Attempt…"
        case .running: return "Mission is running. Follow it in the Mission Room."
        case .awaitingRecovery: return "Mission is waiting for bounded recovery."
        case .succeeded: return "Mission completed with accepted Evidence."
        case .cancelled: return "Mission was cancelled with authoritative terminal Evidence."
        case .failed(let reason): return "Mission could not proceed: \(reason)"
        }
    }

    private func railButton(
        _ title: String,
        systemImage: String,
        action: @escaping () -> Void
    ) -> some View {
        Button(action: action) {
            Label(title, systemImage: systemImage)
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(.horizontal, 14)
        }
        .buttonStyle(.plain)
        .loomActionTarget()
        .contentShape(Rectangle())
    }

    private var connectionFooter: some View {
        VStack(alignment: .leading, spacing: 4) {
            Divider()
            HStack(spacing: 7) {
                Circle()
                    .fill(connectionColor)
                    .frame(width: 8, height: 8)
                    .accessibilityHidden(true)
                Text(connectionLabel)
                    .font(.caption)
                Spacer()
            }
            .padding(12)
        }
    }

    private var orchestrationBoard: some View {
        VStack(spacing: 0) {
            workbenchToolbar(
                title: "Mission Board",
                subtitle: "Authoritative orchestration state"
            )
            HStack(spacing: 8) {
                Label("Board", systemImage: "rectangle.3.group")
                    .foregroundStyle(LoomGraphite.accent)
                Text("Topology")
                Text("Timeline")
                Text("Capacity")
                Spacer()
                TextField("Filter Missions", text: missionFilter)
                    .textFieldStyle(.roundedBorder)
                    .frame(width: 190)
                    .accessibilityLabel("Filter Missions")
            }
            .font(.callout.weight(.medium))
            .padding(.horizontal, 18)
            .frame(height: 46)

            if let authorityBanner {
                HStack(spacing: 8) {
                    Image(systemName: "info.circle")
                    Text(authorityBanner)
                        .font(.caption)
                    Spacer()
                }
                .foregroundStyle(connectionColor)
                .padding(.horizontal, 18)
                .frame(minHeight: 34)
                .background(connectionColor.opacity(0.08))
                .accessibilityLabel(authorityBanner)
            }

            if store.snapshot == nil {
                truthfulState(
                    icon: "bolt.horizontal.circle",
                    title: "Connecting to Loom",
                    detail: "The last authoritative view remains visible if refresh fails."
                )
            } else if store.workbench.missions.isEmpty {
                truthfulState(
                    icon: "rectangle.3.group",
                    title: "No Missions yet",
                    detail: emptyBoardDetail
                )
            } else {
                ScrollView(.horizontal, showsIndicators: true) {
                    HStack(alignment: .top, spacing: 12) {
                        ForEach(MissionLane.allCases, id: \.self) { lane in
                            missionLane(lane)
                        }
                    }
                    .padding(16)
                }
                HStack(spacing: 6) {
                    Image(systemName: "arrow.left.and.right")
                    Text("Scroll horizontally to inspect all five lanes")
                    Spacer()
                    Text("5 lanes")
                        .monospacedDigit()
                }
                .font(.caption2)
                .foregroundStyle(.secondary)
                .padding(.horizontal, 18)
                .frame(height: 26)
                .accessibilityLabel(
                    "Mission Board has five horizontally scrollable lanes"
                )
            }
        }
        .background(LoomGraphite.canvas)
    }

    private func missionLane(_ lane: MissionLane) -> some View {
        let missions = store.workbench.missions.filter {
            $0.lane == lane
                && (store.workbench.boardFilter.isEmpty
                    || $0.title.localizedCaseInsensitiveContains(
                        store.workbench.boardFilter
                    ))
        }
        return VStack(alignment: .leading, spacing: 10) {
            HStack {
                Text(lane.rawValue)
                    .font(.subheadline.weight(.semibold))
                Spacer()
                Text("\(missions.count)")
                    .font(.caption.monospacedDigit())
                    .foregroundStyle(.secondary)
            }
            .padding(.horizontal, 3)
            ForEach(missions) { mission in
                missionCard(mission)
            }
            Spacer(minLength: 0)
        }
        .padding(10)
        .frame(width: 168, alignment: .top)
        .frame(minHeight: 420, alignment: .top)
        .background(
            LoomGraphite.surface.opacity(0.62),
            in: RoundedRectangle(
                cornerRadius: LoomGraphite.cardRadius,
                style: .continuous
            )
        )
        .accessibilityElement(children: .contain)
        .accessibilityLabel("\(lane.rawValue) lane")
    }

    private func missionCard(_ mission: MissionListItem) -> some View {
        let record = missionRecord(mission.id)
        let displayTitle = missionDisplayTitle(
            candidate: record?.title ?? mission.title,
            missionID: record?.missionID ?? mission.id,
            teamInstanceID: record?.teamInstanceID ?? "",
            teams: store.snapshot?.teams ?? []
        )
        return Button {
            Task { await store.openMissionAndActivate(mission.id) }
        } label: {
            VStack(alignment: .leading, spacing: 7) {
                if let record {
                    Text(
                        "\(humanStatus(record.sourceKind)) · \(record.priority.capitalized) priority"
                    )
                    .font(.caption2)
                    .foregroundStyle(.secondary)
                }
                HStack {
                    Text(displayTitle)
                        .font(.callout.weight(.semibold))
                        .lineLimit(2)
                    Spacer(minLength: 4)
                    if mission.status == "human_required" || mission.status == "blocked" {
                        Image(systemName: "exclamationmark.circle.fill")
                            .foregroundStyle(.orange)
                            .accessibilityLabel("Needs attention")
                    }
                }
                Text(humanStatus(mission.status))
                    .font(.caption)
                    .foregroundStyle(.secondary)
                if let record {
                    Group {
                        if let pulse = record.teamPulse.first {
                            Label(
                                "Team · \(pulse.role.capitalized) · \(humanStatus(pulse.state))",
                                systemImage: "person.2"
                            )
                            .lineLimit(2)
                        } else {
                            Label(
                                "No Team presence",
                                systemImage: "person.2.slash"
                            )
                        }
                    }
                    .font(.caption2)
                    .foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)
                    Label(
                        "\(record.teamPulse.first.map { "Attempt \($0.attemptNumber)" } ?? "No active attempt") · \(record.completedNodeCount)/\(record.nodeCount) nodes · \(missionNodeDisplayTitle(nodeID: record.currentNodeID, topology: record.topology, pulse: record.teamPulse))",
                        systemImage: "point.3.connected.trianglepath.dotted"
                    )
                    .font(.caption2)
                    .foregroundStyle(.secondary)
                    .lineLimit(2)
                    .fixedSize(horizontal: false, vertical: true)
                    Text(record.lastMilestone)
                        .font(.caption2)
                        .foregroundStyle(.secondary)
                        .lineLimit(2)
                }
            }
            .padding(12)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(
                LoomGraphite.raised,
                in: RoundedRectangle(
                    cornerRadius: LoomGraphite.cardRadius,
                    style: .continuous
                )
            )
            .overlay {
                RoundedRectangle(
                    cornerRadius: LoomGraphite.cardRadius,
                    style: .continuous
                )
                .stroke(LoomGraphite.separator.opacity(0.7), lineWidth: 1)
            }
        }
        .buttonStyle(.plain)
        .accessibilityLabel(
            "\(displayTitle), \(mission.lane.rawValue), \(humanStatus(mission.status))"
        )
    }

    private func missionRoom(_ id: String) -> some View {
        HSplitView {
            missionList
                .frame(minWidth: 190, idealWidth: 220, maxWidth: 260)
            missionCenter(id)
                .frame(minWidth: 390)
            if store.workbench.selectedContinuity.inspectorVisible {
                missionInspector(id)
                    .frame(
                        minWidth: 240,
                        idealWidth: LoomGraphite.inspectorWidth,
                        maxWidth: 340
                    )
            }
        }
    }

    private var missionList: some View {
        VStack(alignment: .leading, spacing: 5) {
            Button {
                store.showMissionBoard()
            } label: {
                Label("Back to Board", systemImage: "chevron.left")
            }
            .buttonStyle(.plain)
            .loomActionTarget()
            .padding(.horizontal, 12)
            Divider()
            ScrollView {
                LazyVStack(spacing: 3) {
                    ForEach(store.workbench.missions) { mission in
                        let record = missionRecord(mission.id)
                        Button {
                            Task { await store.openMissionAndActivate(mission.id) }
                        } label: {
                            VStack(alignment: .leading, spacing: 3) {
                                Text(missionDisplayTitle(
                                    candidate: record?.title ?? mission.title,
                                    missionID: record?.missionID ?? mission.id,
                                    teamInstanceID: record?.teamInstanceID ?? "",
                                    teams: store.snapshot?.teams ?? []
                                ))
                                    .font(.callout.weight(.medium))
                                    .lineLimit(1)
                                Text(mission.lane.rawValue)
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                            }
                            .padding(9)
                            .frame(maxWidth: .infinity, alignment: .leading)
                            .background(
                                mission.id == store.workbench.selectedMissionID
                                    ? LoomGraphite.accent.opacity(0.12)
                                    : Color.clear,
                                in: RoundedRectangle(cornerRadius: 8)
                            )
                        }
                        .buttonStyle(.plain)
                    }
                }
                .padding(8)
            }
        }
        .background(LoomGraphite.rail)
    }

    private func missionCenter(_ id: String) -> some View {
        let record = missionRecord(id)
        return VStack(spacing: 0) {
            workbenchToolbar(
                title: missionDisplayTitle(
                    candidate: record?.title ?? "",
                    missionID: record?.missionID ?? id,
                    teamInstanceID: record?.teamInstanceID ?? "",
                    teams: store.snapshot?.teams ?? []
                ),
                subtitle: record.map {
                    "\($0.lane) · \(humanStatus($0.status))"
                } ?? "Unavailable"
            )
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 14) {
                    contentBlock(
                        title: "Outcome",
                        icon: "scope",
                        text: record?.lastMilestone
                            ?? "No authoritative Mission record is available."
                    )
                    contentBlock(
                        title: "Plan",
                        icon: "point.3.connected.trianglepath.dotted",
                        text: record.map {
                            "\($0.nodeCount) node(s), \($0.completedNodeCount) complete, \($0.reviewNodeCount) in review."
                        } ?? "No plan is available."
                    )
                    missionActivity(record)
                    if store.missionCancelAvailable {
                        Button("Cancel Mission", role: .destructive) {
                            Task { await store.cancelCurrentMission() }
                        }
                        .buttonStyle(.bordered)
                        .help("Cancels only the exact current authorized Attempt.")
                    }
                    contentBlock(
                        title: "Evidence",
                        icon: "checkmark.seal",
                        text: store.snapshot?.evidence.isEmpty == false
                            ? "Accepted Evidence references are available in the Inspector."
                            : "No accepted Evidence is available yet."
                    )
                }
                .padding(18)
            }
            Divider()
            VStack(alignment: .leading, spacing: 8) {
                HStack(spacing: 6) {
                    Text("Permission")
                        .font(.caption.weight(.semibold))
                        .foregroundStyle(.secondary)
                    ForEach(MissionPermissionMode.allCases) { mode in
                        Button(mode.rawValue) {
                            store.selectMissionPermissionMode(mode)
                        }
                        .buttonStyle(.plain)
                        .font(.caption.weight(.medium))
                        .padding(.horizontal, 9)
                        .frame(minHeight: 28)
                        .background(
                            store.workbench.selectedContinuity.permissionMode == mode
                                ? LoomGraphite.accent.opacity(0.15)
                                : LoomGraphite.surface,
                            in: RoundedRectangle(cornerRadius: 7)
                        )
                        .overlay {
                            RoundedRectangle(cornerRadius: 7)
                                .stroke(
                                    store.workbench.selectedContinuity
                                        .permissionMode == mode
                                        ? LoomGraphite.accent
                                        : LoomGraphite.separator,
                                    lineWidth: 1
                                )
                        }
                    }
                    Spacer()
                }
                Text("A mode is only a proposal until Loom confirms authority.")
                    .font(.caption2)
                    .foregroundStyle(.secondary)
                HStack(alignment: .bottom, spacing: 10) {
                    TextField(
                        "Message this Mission",
                        text: Binding(
                            get: {
                                store.workbench.selectedContinuity.composerDraft
                            },
                            set: { store.updateMissionComposerDraft($0) }
                        ),
                        axis: .vertical
                    )
                    Button("Send", systemImage: "arrow.up.circle.fill") {}
                        .buttonStyle(.borderedProminent)
                        .disabled(true)
                        .help(
                            "Composer remains disabled until an accepted Mission messaging authority exists."
                        )
                }
            }
            .padding(12)
        }
        .background(LoomGraphite.canvas)
    }

    private func missionInspector(_ id: String) -> some View {
        let record = missionRecord(id)
        let section = missionInspectorSection(
            tab: store.workbench.selectedContinuity.inspector,
            record: record,
            timeline: store.timeline
        )
        return VStack(alignment: .leading, spacing: 12) {
            HStack {
                Text("Inspector")
                    .font(.headline)
                Spacer()
                Button {
                    store.setMissionInspectorVisible(false)
                } label: {
                    Image(systemName: "sidebar.right")
                }
                .buttonStyle(.plain)
                .loomActionTarget()
                .accessibilityLabel("Hide Inspector")
            }
            Picker(
                "Inspector",
                selection: Binding(
                    get: { store.workbench.selectedContinuity.inspector },
                    set: { store.selectMissionInspector($0) }
                )
            ) {
                ForEach(MissionInspectorTab.allCases) {
                    Text($0.rawValue).tag($0)
                }
            }
            .pickerStyle(.segmented)
            Divider()
            Text(section.heading)
                .font(.subheadline.weight(.semibold))
            if section.rows.isEmpty {
                Text(section.emptyMessage)
                    .font(.caption)
                    .foregroundStyle(.secondary)
            } else {
                ForEach(Array(section.rows.enumerated()), id: \.offset) { _, row in
                    HStack {
                        Image(systemName: inspectorSectionIcon)
                            .foregroundStyle(LoomGraphite.accent)
                        Text(row)
                            .font(.caption)
                            .fixedSize(horizontal: false, vertical: true)
                        Spacer()
                    }
                }
            }
            Divider()
            Text("Side-tasks")
                .font(.subheadline.weight(.semibold))
            Button("New side task", systemImage: "plus.bubble") {
                store.discardSideTaskProposal()
                showNewSideTask = true
            }
            .buttonStyle(.borderedProminent)
            .disabled(!store.canCreateSideTask(for: id))
            .help(
                store.canCreateSideTask(for: id)
                    ? "Propose a bounded child task, then confirm it explicitly."
                    : "Open the current authoritative Mission timeline before creating a side task."
            )
            let sideTasks = (store.snapshot?.sideTasks ?? []).filter {
                $0.parentMissionID == id
            }
            if sideTasks.isEmpty {
                Text("No Side-task handoff is recorded.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            } else {
                ForEach(sideTasks, id: \.sideTaskID) { sideTask in
                    let presentation = sideTaskDrawerPresentation(sideTask)
                    VStack(alignment: .leading, spacing: 3) {
                        Text(sideTask.title)
                            .font(.caption.weight(.semibold))
                        Text(presentation.purpose)
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                        Text("\(missionHumanStatus(sideTask.mode)) · \(missionHumanStatus(sideTask.status))")
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                        if !sideTask.whatHappened.isEmpty {
                            Text(sideTask.whatHappened)
                                .font(.caption)
                                .lineLimit(3)
                        }
                        ForEach(Array(sideTask.authorizedFindings.prefix(3)), id: \.self) { finding in
                            Text("Finding · \(finding)")
                                .font(.caption2)
                                .lineLimit(2)
                        }
                        Text("Risk · \(missionHumanStatus(sideTask.risk)) · Evidence \(sideTask.evidenceReferences.count) · Artifacts \(sideTask.artifactReferences.count)")
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                        if let evidence = sideTask.evidenceReferences.first {
                            Text("Evidence ref · \(missionHumanStatus(evidence.kind)) · \(evidence.evidenceID)")
                                .font(.caption2)
                                .foregroundStyle(.secondary)
                                .lineLimit(1)
                        }
                        if let artifact = sideTask.artifactReferences.first {
                            Text("Artifact ref · \(missionHumanStatus(artifact.kind)) · \(String(artifact.digest.prefix(12)))…")
                                .font(.caption2)
                                .foregroundStyle(.secondary)
                        }
                        Text(
                            sideTask.usageObserved
                                ? "Usage · \(sideTask.usageMicrounits) microunits \(sideTask.usageCurrency)"
                                : "Usage · Not observed"
                        )
                        .font(.caption2)
                        .foregroundStyle(.secondary)
                        Text(presentation.uncertainty)
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                        Text(presentation.scopeDelta)
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                        Text(presentation.nextAction)
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                        if !sideTask.availableDecisions.isEmpty {
                            Menu("Choose next action") {
                                ForEach(sideTask.availableDecisions, id: \.self) { decision in
                                    Button(missionHumanStatus(decision)) {
                                        Task { await store.decideSideTask(sideTask, decision: decision) }
                                    }
                                }
                            }
                            .disabled(!store.canCreateSideTask(for: id))
                        }
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .padding(8)
                    .background(LoomGraphite.surface, in: RoundedRectangle(cornerRadius: 8))
                }
            }
            Divider()
            Text("Decision availability")
                .font(.subheadline.weight(.semibold))
            if let command = store.preparedDecisionCommand(for: id) {
                Button(decisionCTA(command.kind)) {
                    Task { await store.openPreparedDecision(command) }
                }
                .buttonStyle(.borderedProminent)
                .loomActionTarget()
                .accessibilityLabel(decisionCTA(command.kind))
                Text(
                    "Bound to \(record.map { missionNodeDisplayTitle(nodeID: command.logicalNodeID, topology: $0.topology, pulse: $0.teamPulse) } ?? "Mission step") · Attempt \(command.attemptNumber)"
                )
                .font(.caption)
                .foregroundStyle(.secondary)
            } else {
                if let record, record.lane == "Review" {
                    Button("Open Review Gate") {
                        store.openReadOnlyReviewDecision(for: record)
                    }
                    .buttonStyle(.bordered)
                    .loomActionTarget()
                    .accessibilityLabel("Open read-only Review Gate")
                }
                Text(
                    record?.attentionCount ?? 0 > 0
                        ? "Needs You · visible read-only until an authoritative prepared command exists"
                        : "No pending prepared decision"
                )
                .font(.caption)
                .foregroundStyle(.secondary)
            }
            Spacer()
        }
        .padding(14)
        .background(LoomGraphite.surface)
    }

    private func newSideTaskSheet(_ missionID: String) -> some View {
        VStack(alignment: .leading, spacing: 16) {
            HStack {
                VStack(alignment: .leading, spacing: 4) {
                    Text("New side task").font(.title2.weight(.semibold))
                    Text("Review the zero-write proposal before Loom starts an independent child run.")
                        .font(.callout).foregroundStyle(.secondary)
                }
                Spacer()
                Button("Close") { showNewSideTask = false }
            }
            Picker("Purpose", selection: $sideTaskPurpose) {
                ForEach(["research", "comparison", "diagnosis", "verification", "read_only_review"], id: \.self) {
                    Text(missionHumanStatus($0)).tag($0)
                }
            }
            Picker("Mode", selection: $sideTaskMode) {
                ForEach(["report_only", "decision_required", "merge_candidate"], id: \.self) {
                    Text(missionHumanStatus($0)).tag($0)
                }
            }
            TextField("Short title", text: $sideTaskTitle)
                .textFieldStyle(.roundedBorder)
            TextEditor(text: $sideTaskRequest)
                .frame(minHeight: 150)
                .overlay(RoundedRectangle(cornerRadius: 8).stroke(LoomGraphite.separator))
            Text(store.sideTaskOperationStatus)
                .font(.caption).foregroundStyle(.secondary)
            HStack {
                Spacer()
                if store.sideTaskProposal == nil {
                    Button("Review proposal") {
                        Task {
                            await store.proposeSideTask(
                                missionID: missionID,
                                purpose: sideTaskPurpose,
                                mode: sideTaskMode,
                                title: sideTaskTitle,
                                authorizedRequest: sideTaskRequest
                            )
                        }
                    }
                    .buttonStyle(.borderedProminent)
                    .disabled(sideTaskTitle.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty || sideTaskRequest.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
                } else {
                    Button("Back") { store.discardSideTaskProposal() }
                    Button("Confirm and run") {
                        Task { await store.confirmSideTaskProposal() }
                    }
                    .buttonStyle(.borderedProminent)
                }
            }
        }
        .padding(24)
        .accessibilityLabel("Side-task proposal and confirmation")
    }

    private var inspectorSectionIcon: String {
        switch store.workbench.selectedContinuity.inspector {
        case .team: return "person.crop.circle"
        case .plan: return "point.3.connected.trianglepath.dotted"
        case .changes: return "arrow.triangle.2.circlepath"
        case .evidence: return "checkmark.seal"
        }
    }

    private func workbenchToolbar(
        title: String,
        subtitle: String
    ) -> some View {
        HStack {
            VStack(alignment: .leading, spacing: 2) {
                Text(title)
                    .font(.title3.weight(.semibold))
                Text(subtitle)
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            Spacer()
            if !store.workbench.selectedContinuity.inspectorVisible,
                case .mission = store.workbench.route
            {
                Button {
                    store.setMissionInspectorVisible(true)
                } label: {
                    Image(systemName: "sidebar.right")
                }
                .buttonStyle(.plain)
                .loomActionTarget()
                .accessibilityLabel("Show Inspector")
            }
            Button {
                Task {
                    switch store.workbench.route {
                    case .mission(let id):
                        await store.refreshMission(id)
                    case .board, .teams, .attention, .library:
                        await store.refresh()
                    }
                }
            } label: {
                Image(systemName: "arrow.clockwise")
            }
            .buttonStyle(.plain)
            .loomActionTarget()
            .accessibilityLabel("Refresh authoritative view")
        }
        .padding(.horizontal, 18)
        .frame(height: 58)
        .background(.bar)
    }

    private func contentBlock(
        title: String,
        icon: String,
        text: String
    ) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Label(title, systemImage: icon)
                .font(.subheadline.weight(.semibold))
            Text(text)
                .foregroundStyle(.secondary)
                .textSelection(.enabled)
        }
        .padding(14)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(
            LoomGraphite.surface,
            in: RoundedRectangle(
                cornerRadius: LoomGraphite.cardRadius,
                style: .continuous
            )
        )
    }

    @ViewBuilder
    private func missionActivity(
        _ mission: LocalProductMissionSummary?
    ) -> some View {
        let records = store.timeline?.records ?? []
        VStack(alignment: .leading, spacing: 10) {
            Label("Activity", systemImage: "waveform.path.ecg")
                .font(.subheadline.weight(.semibold))
            if let gap = store.timeline?.gap {
                Label(
                    "Some live activity is unavailable · \(humanStatus(gap.reason))",
                    systemImage: "exclamationmark.triangle"
                )
                .font(.caption)
                .foregroundStyle(.orange)
            }
            if records.isEmpty {
                Text(
                    mission?.activeNodeCount == 0
                        ? "No active authorized output."
                        : "Authorized activity is in progress. Live output is tentative until Evidence is accepted."
                )
                .foregroundStyle(.secondary)
            } else {
                ForEach(records) { item in
                    VStack(alignment: .leading, spacing: 4) {
                        if !item.payload.textDelta.isEmpty {
                            Label("Tentative output", systemImage: "text.bubble")
                                .font(.caption.weight(.semibold))
                                .foregroundStyle(.orange)
                            Text(item.payload.textDelta)
                                .textSelection(.enabled)
                        } else {
                            Text(
                                humanStatus(
                                    item.payload.status.isEmpty
                                        ? item.kind
                                        : item.payload.status
                                )
                            )
                            .font(.callout.weight(.medium))
                            if !item.payload.warningCode.isEmpty {
                                Text(humanStatus(item.payload.warningCode))
                                    .font(.caption)
                                    .foregroundStyle(.orange)
                            }
                        }
                    }
                    .padding(.vertical, 3)
                }
            }
            Text("Tentative text is not authoritative until terminal Evidence is accepted.")
                .font(.caption2)
                .foregroundStyle(.secondary)
        }
        .padding(14)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(
            LoomGraphite.surface,
            in: RoundedRectangle(
                cornerRadius: LoomGraphite.cardRadius,
                style: .continuous
            )
        )
    }

    private func truthfulState(
        icon: String,
        title: String,
        detail: String
    ) -> some View {
        ContentUnavailableView(
            title,
            systemImage: icon,
            description: Text(detail)
        )
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }

    private var teamsWorkspace: some View {
        VStack(spacing: 0) {
            workbenchToolbar(
                title: "Teams",
                subtitle: "Confirmed Team records"
            )
            workspaceReadBanner
            if let snapshot = store.snapshot {
                if snapshot.teams.isEmpty {
                    truthfulState(
                        icon: "person.3",
                        title: "No confirmed Teams",
                        detail: "Create and confirm a Team before starting a Mission."
                    )
                } else {
                    ScrollView {
                        LazyVStack(alignment: .leading, spacing: 12) {
                            ForEach(snapshot.teams) { team in
                                VStack(alignment: .leading, spacing: 8) {
                                    HStack {
                                        Text(LocalProductExperience.visibleName(
                                            team.displayName,
                                            internalID: team.teamInstanceID,
                                            fallback: "Confirmed Team"
                                        ))
                                            .font(.headline)
                                        Spacer()
                                        Text(humanStatus(team.state))
                                            .font(.caption.weight(.semibold))
                                            .foregroundStyle(
                                                team.executable && !team.readOnly
                                                    ? .green
                                                    : .secondary
                                            )
                                    }
                                    Text(
                                        workspacePresentation.isCurrent
                                            ? (team.executable && !team.readOnly
                                            ? "Confirmed · Available for explicit Mission start"
                                            : "Read-only · Not available for execution")
                                            : "Preserved record · Refresh before execution"
                                    )
                                    .font(.callout)
                                    .foregroundStyle(.secondary)
                                    Text("Source · \(humanStatus(team.sourceKind))")
                                        .font(.caption)
                                        .foregroundStyle(.secondary)
                                }
                                .padding(14)
                                .background(
                                    LoomGraphite.surface,
                                    in: RoundedRectangle(
                                        cornerRadius: LoomGraphite.cardRadius,
                                        style: .continuous
                                    )
                                )
                                .accessibilityElement(children: .combine)
                            }
                        }
                        .padding(18)
                    }
                }
            } else {
                truthfulState(
                    icon: "person.3",
                    title: "Teams unavailable",
                    detail: connectionLabel
                )
            }
        }
        .background(LoomGraphite.canvas)
    }

    private var attentionWorkspace: some View {
        VStack(spacing: 0) {
            workbenchToolbar(
                title: "Needs You",
                subtitle: "Approvals and blocked decisions"
            )
            workspaceReadBanner
            if let snapshot = store.snapshot {
                if snapshot.attention.isEmpty {
                    let copy = missionAttentionEmptyCopy(
                        presentation: workspacePresentation
                    )
                    truthfulState(
                        icon: copy.icon,
                        title: copy.title,
                        detail: copy.detail
                    )
                } else {
                    ScrollView {
                        LazyVStack(alignment: .leading, spacing: 12) {
                            ForEach(snapshot.attention) { item in
                                VStack(alignment: .leading, spacing: 7) {
                                    HStack {
                                        Text(humanStatus(item.kind))
                                            .font(.headline)
                                        Spacer()
                                        Text(humanStatus(item.severity))
                                            .font(.caption.weight(.semibold))
                                            .foregroundStyle(.orange)
                                    }
                                    Text(humanStatus(item.status))
                                        .foregroundStyle(.secondary)
                                    if !item.actionRequired.isEmpty {
                                        Text(item.actionRequired)
                                            .font(.callout)
                                    }
                                }
                                .padding(14)
                                .background(
                                    LoomGraphite.surface,
                                    in: RoundedRectangle(
                                        cornerRadius: LoomGraphite.cardRadius,
                                        style: .continuous
                                    )
                                )
                                .accessibilityElement(children: .combine)
                            }
                        }
                        .padding(18)
                    }
                }
            } else {
                truthfulState(
                    icon: "exclamationmark.bubble",
                    title: "Attention unavailable",
                    detail: connectionLabel
                )
            }
        }
        .background(LoomGraphite.canvas)
    }

    private var libraryWorkspace: some View {
        VStack(spacing: 0) {
            workbenchToolbar(
                title: "Library",
                subtitle: "History and Compare"
            )
            workspaceReadBanner
            if let snapshot = store.snapshot {
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 18) {
                        Text("History")
                            .font(.title3.weight(.semibold))
                        if snapshot.runs.isEmpty {
                            Text("No Run history is available.")
                                .foregroundStyle(.secondary)
                        } else {
                            ForEach(Array(snapshot.runs.reversed())) { run in
                                historyRunCard(run, snapshot: snapshot)
                            }
                        }

                        Divider()
                        Text("Compare")
                            .font(.title3.weight(.semibold))
                        if snapshot.runs.count < 2 {
                            Text("Two authoritative Runs are required for comparison.")
                                .foregroundStyle(.secondary)
                        } else {
                            let pair = Array(snapshot.runs.suffix(2))
                            HStack(alignment: .top, spacing: 12) {
                                comparisonColumn(
                                    label: "Previous",
                                    run: pair[0],
                                    snapshot: snapshot
                                )
                                comparisonColumn(
                                    label: "Current",
                                    run: pair[1],
                                    snapshot: snapshot
                                )
                            }
                        }
                    }
                    .padding(18)
                }
            } else {
                truthfulState(
                    icon: "books.vertical",
                    title: "History unavailable",
                    detail: connectionLabel
                )
            }
        }
        .background(LoomGraphite.canvas)
    }

    private func historyRunCard(
        _ run: LocalProductRunSummary,
        snapshot: LocalProductSnapshot
    ) -> some View {
        HStack(alignment: .top, spacing: 12) {
            Image(systemName: "clock.arrow.circlepath")
                .foregroundStyle(LoomGraphite.accent)
            VStack(alignment: .leading, spacing: 5) {
                Text(humanStatus(run.phase))
                    .font(.headline)
                Text(
                    run.terminalStatus.isEmpty
                        ? "In progress"
                        : humanStatus(run.terminalStatus)
                )
                .foregroundStyle(.secondary)
                Text(
                    "Runtime · \(missionRuntimeDisplayName(run: run, snapshot: snapshot)) · Attempt generation \(run.claimGeneration)"
                )
                .font(.caption)
                .foregroundStyle(.secondary)
                Text("Accepted Evidence · \(evidenceCount(for: run, in: snapshot))")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            Spacer()
        }
        .padding(14)
        .background(
            LoomGraphite.surface,
            in: RoundedRectangle(
                cornerRadius: LoomGraphite.cardRadius,
                style: .continuous
            )
        )
        .accessibilityElement(children: .combine)
    }

    private func comparisonColumn(
        label: String,
        run: LocalProductRunSummary,
        snapshot: LocalProductSnapshot
    ) -> some View {
        VStack(alignment: .leading, spacing: 7) {
            Text(label)
                .font(.caption.weight(.semibold))
                .foregroundStyle(.secondary)
            Text(humanStatus(run.terminalStatus.isEmpty ? run.phase : run.terminalStatus))
                .font(.headline)
            Text("Runtime · \(missionRuntimeDisplayName(run: run, snapshot: snapshot))")
                .font(.caption)
            Text("Attempt generation · \(run.claimGeneration)")
                .font(.caption)
            Text("Evidence · \(evidenceCount(for: run, in: snapshot))")
                .font(.caption)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(14)
        .background(
            LoomGraphite.surface,
            in: RoundedRectangle(
                cornerRadius: LoomGraphite.cardRadius,
                style: .continuous
            )
        )
        .accessibilityElement(children: .combine)
    }

    private func evidenceCount(
        for run: LocalProductRunSummary,
        in snapshot: LocalProductSnapshot
    ) -> Int {
        snapshot.evidence.filter { $0.workItemID == run.workItemID }.count
    }

    private var workspacePresentation: MissionSnapshotPresentation {
        missionSnapshotPresentation(
            connection: store.connectionState,
            hasSnapshot: store.snapshot != nil
        )
    }

    @ViewBuilder
    private var workspaceReadBanner: some View {
        if let notice = workspacePresentation.notice {
            Label(notice, systemImage: "clock.arrow.circlepath")
                .font(.caption)
                .foregroundStyle(.orange)
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(.horizontal, 18)
                .padding(.vertical, 9)
                .background(Color.orange.opacity(0.08))
        }
    }

    private var providerManagement: some View {
        VStack(alignment: .leading, spacing: 18) {
            HStack {
                VStack(alignment: .leading, spacing: 3) {
                    Text("Runtime & Providers")
                        .font(.title2.weight(.semibold))
                    Text("Secondary workspace settings")
                        .foregroundStyle(.secondary)
                }
                Spacer()
                Button("Done") { showProviders = false }
                    .keyboardShortcut(.cancelAction)
            }
            Divider()
            providerRow(
                title: "Codex",
                status: store.setupSnapshot?.codex.status ?? "unavailable",
                action: store.setupSnapshot?.codex.status == "available"
                    ? "Manage"
                    : "Connect"
            ) {
                Task { await store.connectCodex() }
            }
            providerRow(
                title: "MiniMax",
                status: store.miniMaxVerificationStatus == "Idle"
                    ? (store.setupSnapshot?.miniMax.status ?? "unavailable")
                    : store.miniMaxVerificationStatus,
                action: "Test"
            ) {
                Task { await store.verifyMiniMax() }
            }
            .disabled(store.isVerifyingMiniMax)
            Spacer()
            Text(
                "Credentials stay in the private Broker and Keychain boundary. Loom never displays the raw secret."
            )
            .font(.caption)
            .foregroundStyle(.secondary)
        }
        .padding(24)
    }

    private func providerRow(
        title: String,
        status: String,
        action: String,
        perform: @escaping () -> Void
    ) -> some View {
        HStack(spacing: 12) {
            Image(systemName: "cpu")
                .frame(width: 28)
            VStack(alignment: .leading, spacing: 3) {
                Text(title)
                    .font(.headline)
                Text(humanStatus(status))
                    .foregroundStyle(.secondary)
            }
            Spacer()
            Button(action, action: perform)
                .buttonStyle(.bordered)
                .loomActionTarget()
        }
        .padding(14)
        .background(
            LoomGraphite.surface,
            in: RoundedRectangle(cornerRadius: LoomGraphite.cardRadius)
        )
    }

    private func missionRecord(
        _ id: String
    ) -> LocalProductMissionSummary? {
        store.snapshot?.missions.first(where: { $0.missionID == id })
    }

    private var missionFilter: Binding<String> {
        Binding(
            get: { store.workbench.boardFilter },
            set: { store.updateMissionBoardFilter($0) }
        )
    }

    private var connectionLabel: String {
        switch store.connectionState {
        case .loading: return "Connecting"
        case .online: return "Authoritative"
        case .partial: return "Partial view"
        case .stale: return "Stale view"
        case .offline: return "Offline"
        case .fatal: return "Needs attention"
        }
    }

    private var authorityBanner: String? {
        switch store.connectionState {
        case .online:
            return nil
        case .loading:
            return "Connecting to the authoritative Journal view"
        case .partial(let reason):
            return "Partial authoritative view · \(reason)"
        case .stale(let reason):
            return "Showing preserved view · \(reason)"
        case .offline(let reason):
            return "Loom is offline · \(reason)"
        case .fatal(let reason):
            return "Workbench needs attention · \(reason)"
        }
    }

    private var emptyBoardDetail: String {
        let teamCount = store.snapshot?.teams.count ?? 0
        let runCount = store.snapshot?.runs.count ?? 0
        if teamCount > 0 || runCount > 0 {
            return
                "No Mission execution exists. \(teamCount) Team(s) and \(runCount) historical Run(s) remain available."
        }
        return "Create a Mission, choose a confirmed Team, then review the plan before execution."
    }

    private var connectionColor: Color {
        switch store.connectionState {
        case .online: return .green
        case .partial, .stale, .loading: return .orange
        case .offline, .fatal: return .red
        }
    }

    private func humanStatus(_ value: String) -> String {
        missionHumanStatus(value)
    }

    private func decisionCTA(_ kind: LocalProductDecisionKind) -> String {
        switch kind {
        case .authorization: return "Open Authorization"
        case .review: return "Open Review Gate"
        case .recovery: return "Open Recovery Decision"
        }
    }
}

public struct LoomDecisionSheet: View {
    public let sheet: LocalProductDecisionSheet
    public let onAction: (String) -> Void
    public let onClose: () -> Void

    @State private var technicalDetailsExpanded = false

    public init(
        sheet: LocalProductDecisionSheet,
        onAction: @escaping (String) -> Void,
        onClose: @escaping () -> Void
    ) {
        self.sheet = sheet
        self.onAction = onAction
        self.onClose = onClose
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            HStack(alignment: .top, spacing: 14) {
                Image(systemName: symbol)
                    .font(.title2)
                    .foregroundStyle(LoomGraphite.accent)
                    .frame(width: 34, height: 34)
                    .accessibilityHidden(true)
                VStack(alignment: .leading, spacing: 4) {
                    Text(sheet.title)
                        .font(.title2.weight(.semibold))
                    Text(sheet.summary)
                        .foregroundStyle(.secondary)
                }
                Spacer()
                Button {
                    onAction("not_now")
                    onClose()
                } label: {
                    Image(systemName: "xmark")
                }
                .buttonStyle(.plain)
                .loomActionTarget()
                .accessibilityLabel("Not now")
                .keyboardShortcut(.cancelAction)
            }
            .padding(24)

            Divider()

            ScrollView {
                VStack(alignment: .leading, spacing: 18) {
                    identitySection
                    templateSection
                    DisclosureGroup(
                        "Technical details",
                        isExpanded: $technicalDetailsExpanded
                    ) {
                        VStack(alignment: .leading, spacing: 7) {
                            ForEach(sheet.technicalDetails, id: \.self) {
                                Text($0)
                                    .font(.caption.monospaced())
                                    .textSelection(.enabled)
                            }
                        }
                        .padding(.top, 8)
                    }
                    .accessibilityLabel("Technical details")
                }
                .padding(24)
            }

            Divider()
            actionBar
                .padding(18)
        }
        .background(LoomGraphite.canvas)
        .accessibilityElement(children: .contain)
        .accessibilityLabel("\(kindLabel) decision sheet")
    }

    private var identitySection: some View {
        VStack(alignment: .leading, spacing: 10) {
            decisionRow("Mission", "Current Mission")
            decisionRow("Requested by", sheet.requester)
            decisionRow("Target", sheet.target)
            decisionRow("Attempt", sheet.attemptScope)
        }
        .padding(14)
        .background(
            LoomGraphite.surface,
            in: RoundedRectangle(cornerRadius: LoomGraphite.cardRadius)
        )
    }

    @ViewBuilder
    private var templateSection: some View {
        switch sheet.kind {
        case .authorization:
            decisionGrid([
                ("Requested action", sheet.summary),
                ("Command type", sheet.commandType),
                ("Network access", sheet.networkAccess),
                ("Credential access", sheet.credentialAccess),
                ("Permission scope", sheet.permissionScope),
                ("Expected Evidence", sheet.expectedEvidence),
            ])
        case .review:
            VStack(alignment: .leading, spacing: 10) {
                Text("Verification and Evidence")
                    .font(.headline)
                decisionGrid([
                    ("Change summary", sheet.summary),
                    ("Tests and verification", sheet.commandType),
                    ("Reviewer result", sheet.networkAccess),
                    ("Required Evidence", sheet.expectedEvidence),
                ])
                Button("Open Evidence") {}
                    .buttonStyle(.bordered)
                    .loomActionTarget()
                    .disabled(sheet.expectedEvidence.isEmpty)
            }
        case .recovery:
            VStack(alignment: .leading, spacing: 10) {
                Text("Recovery boundary")
                    .font(.headline)
                decisionGrid([
                    ("Stopped because", sheet.summary),
                    ("Retained output and Evidence", sheet.expectedEvidence),
                    ("Fresh Attempt", sheet.attemptScope),
                    ("Team / Provider changes", sheet.target),
                    ("Permission / budget", sheet.permissionScope),
                ])
                Text(
                    "No hidden retry. A new attempt is confirmed only after a fresh Attempt and claim generation appear in the authoritative Projection."
                )
                .font(.caption)
                .foregroundStyle(.secondary)
            }
        }
    }

    private func decisionGrid(
        _ rows: [(String, String)]
    ) -> some View {
        VStack(alignment: .leading, spacing: 10) {
            ForEach(Array(rows.enumerated()), id: \.offset) { _, row in
                decisionRow(row.0, row.1)
            }
        }
        .padding(14)
        .background(
            LoomGraphite.surface,
            in: RoundedRectangle(cornerRadius: LoomGraphite.cardRadius)
        )
    }

    private func decisionRow(_ label: String, _ value: String) -> some View {
        HStack(alignment: .firstTextBaseline, spacing: 14) {
            Text(label)
                .font(.caption.weight(.semibold))
                .foregroundStyle(.secondary)
                .frame(width: 138, alignment: .leading)
            Text(value)
                .frame(maxWidth: .infinity, alignment: .leading)
                .textSelection(.enabled)
        }
        .accessibilityElement(children: .combine)
    }

    private var actionBar: some View {
        HStack(spacing: 10) {
            ForEach(sheet.actions, id: \.self) { action in
                decisionButton(action)
            }
            Spacer(minLength: 0)
        }
    }

    @ViewBuilder
    private func decisionButton(_ action: String) -> some View {
        if primaryAction(action) {
            decisionButtonBase(action)
                .buttonStyle(.borderedProminent)
        } else {
            decisionButtonBase(action)
                .buttonStyle(.bordered)
        }
    }

    private func decisionButtonBase(_ action: String) -> some View {
        Button(actionLabel(action)) {
            onAction(action)
            if action == "not_now" || action == "edit_scope" {
                onClose()
            }
        }
        .loomActionTarget()
        .disabled(
            requiresPrepared(action) && (!sheet.prepared || !sheet.preparedActions.contains(action))
        )
        .accessibilityLabel(actionLabel(action))
    }

    private func requiresPrepared(_ action: String) -> Bool {
        !["not_now", "edit_scope"].contains(action)
    }

    private func primaryAction(_ action: String) -> Bool {
        [
            "allow_once", "allow_for_mission", "accept_result",
            "start_new_attempt",
        ].contains(action)
    }

    private func actionLabel(_ action: String) -> String {
        switch action {
        case "not_now": return "Not now"
        case "deny": return "Deny"
        case "edit_scope":
            return sheet.kind == .recovery ? "Edit recovery" : "Edit scope"
        case "allow_once": return "Allow once"
        case "allow_for_mission": return "Allow for this Mission"
        case "request_changes": return "Request changes"
        case "accept_result": return "Accept result"
        case "stop_mission": return "Stop Mission"
        case "start_new_attempt": return "Start new attempt"
        default: return action.replacingOccurrences(of: "_", with: " ").capitalized
        }
    }

    private var symbol: String {
        switch sheet.kind {
        case .authorization: return "lock.shield"
        case .review: return "checkmark.seal"
        case .recovery: return "arrow.triangle.2.circlepath"
        }
    }

    private var kindLabel: String {
        switch sheet.kind {
        case .authorization: return "Authorization"
        case .review: return "Review Gate"
        case .recovery: return "Recovery"
        }
    }
}
