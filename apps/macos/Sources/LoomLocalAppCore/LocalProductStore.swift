import Foundation
import SwiftUI

public enum LocalProductWorkspaceTaskKind: String, Equatable, Sendable {
    case draft
    case history
    case team
    case attention
}

public enum LocalProductInspectorTab: String, CaseIterable, Identifiable, Sendable {
    case team = "Team"
    case context = "Context"
    case changes = "Changes"
    case evidence = "Evidence"

    public var id: String { rawValue }
}

public struct LocalProductTaskContinuity: Equatable, Sendable {
    public var composerDraft: String
    public var inspector: LocalProductInspectorTab
    public var threadAnchor: String
    public var developerDetailsExpanded: Bool

    public init(
        composerDraft: String = "",
        inspector: LocalProductInspectorTab = .team,
        threadAnchor: String = "start",
        developerDetailsExpanded: Bool = false
    ) {
        self.composerDraft = composerDraft
        self.inspector = inspector
        self.threadAnchor = threadAnchor
        self.developerDetailsExpanded = developerDetailsExpanded
    }
}

public struct LocalProductWorkspaceTask: Identifiable, Equatable, Sendable {
    public let id: String
    public let title: String
    public let subtitle: String
    public let kind: LocalProductWorkspaceTaskKind

    public init(
        id: String,
        title: String,
        subtitle: String,
        kind: LocalProductWorkspaceTaskKind
    ) {
        self.id = id
        self.title = title
        self.subtitle = subtitle
        self.kind = kind
    }
}

public struct LocalProductWorkspaceState: Equatable, Sendable {
    public static let newTaskID = "local:new-task"

    public private(set) var selectedTaskID: String
    public private(set) var tasks: [LocalProductWorkspaceTask]
    private var continuityByTask: [String: LocalProductTaskContinuity]

    public init() {
        let newTask = LocalProductWorkspaceTask(
            id: Self.newTaskID,
            title: "New task",
            subtitle: "Describe what you want to accomplish",
            kind: .draft
        )
        selectedTaskID = Self.newTaskID
        tasks = [newTask]
        continuityByTask = [Self.newTaskID: LocalProductTaskContinuity()]
    }

    public var selectedTask: LocalProductWorkspaceTask {
        tasks.first(where: { $0.id == selectedTaskID }) ?? tasks[0]
    }

    public var selectedContinuity: LocalProductTaskContinuity {
        continuityByTask[selectedTaskID] ?? LocalProductTaskContinuity()
    }

    public mutating func selectTask(_ id: String) {
        guard tasks.contains(where: { $0.id == id }) else { return }
        selectedTaskID = id
        if continuityByTask[id] == nil {
            continuityByTask[id] = LocalProductTaskContinuity()
        }
    }

    public mutating func updateComposerDraft(_ value: String) {
        mutateSelected { $0.composerDraft = String(value.prefix(4_096)) }
    }

    public mutating func selectInspector(_ inspector: LocalProductInspectorTab) {
        mutateSelected { $0.inspector = inspector }
    }

    public mutating func updateThreadAnchor(_ value: String) {
        mutateSelected { $0.threadAnchor = String(value.prefix(256)) }
    }

    public mutating func setDeveloperDetailsExpanded(_ expanded: Bool) {
        mutateSelected { $0.developerDetailsExpanded = expanded }
    }

    public mutating func mergeAuthoritativeTasks(
        _ authoritative: [LocalProductWorkspaceTask]
    ) {
        let newTask = tasks.first(where: { $0.id == Self.newTaskID }) ??
            LocalProductWorkspaceTask(
                id: Self.newTaskID,
                title: "New task",
                subtitle: "Describe what you want to accomplish",
                kind: .draft
            )
        var seen = Set([Self.newTaskID])
        let bounded = authoritative.filter { task in
            !task.id.isEmpty && seen.insert(task.id).inserted
        }
        tasks = [newTask] + bounded
        for task in tasks where continuityByTask[task.id] == nil {
            continuityByTask[task.id] = LocalProductTaskContinuity()
        }
        continuityByTask = continuityByTask.filter { id, _ in
            tasks.contains(where: { $0.id == id })
        }
        if !tasks.contains(where: { $0.id == selectedTaskID }) {
            selectedTaskID = Self.newTaskID
        }
    }

    public func filteredTasks(matching query: String) -> [LocalProductWorkspaceTask] {
        let normalized = query.trimmingCharacters(
            in: .whitespacesAndNewlines
        )
        guard !normalized.isEmpty else { return tasks }
        let selected = selectedTaskID
        return tasks.filter { task in
            task.id == selected ||
                task.title.localizedCaseInsensitiveContains(normalized) ||
                task.subtitle.localizedCaseInsensitiveContains(normalized)
        }
    }

    private mutating func mutateSelected(
        _ mutation: (inout LocalProductTaskContinuity) -> Void
    ) {
        var continuity = selectedContinuity
        mutation(&continuity)
        continuityByTask[selectedTaskID] = continuity
    }
}

public enum LocalProductConnectionState: Equatable, Sendable {
    case loading
    case online
    case partial(reason: String)
    case stale(reason: String)
    case offline(reason: String)
    case fatal(reason: String)
}

public enum LocalProductTimelineState: Equatable, Sendable {
    case idle
    case loading
    case loaded
    case unavailable
    case fatal
}

public enum LocalProductSetupState: Equatable, Sendable {
    case idle
    case loading
    case ready
    case unavailable(reason: String)
    case fatal(reason: String)
}

public enum LocalProductSection: String, CaseIterable, Identifiable, Sendable {
    case home = "Home"
    case work = "Work"
    case teams = "Teams"
    case inbox = "Inbox"
    case system = "System"

    public var id: String { rawValue }
}

public protocol LocalProductClientProtocol {
    func snapshot(limit: Int) async throws -> LocalProductSnapshot
    func timeline(
        teamInstanceID: String,
        cursor: String,
        limit: Int
    ) async throws -> LocalProductTimelinePage
}

public protocol LocalProductDecisionClientProtocol {
    func readMissionDecision(
        _ command: LocalProductDecisionCommand
    ) async throws -> LocalProductDecisionSheet
    func decideMission(
        _ command: LocalProductDecisionCommand
    ) async throws -> LocalProductDecisionResult
}

public protocol LocalProductSetupClientProtocol {
    func setupSnapshot() async throws -> LocalProductSetupSnapshot
    func connectCodex() async throws -> LocalProductProviderConnectResult
    func startBuilder(
        source: String,
        sourceID: String,
        sourceVersion: Int,
        sourceDigest: String
    ) async throws -> LocalProductBuilderSession
    func answerBuilder(
        session: LocalProductBuilderSession,
        answer: String
    ) async throws -> LocalProductBuilderSession
    func editBuilder(
        session: LocalProductBuilderSession,
        field: String,
        value: String
    ) async throws -> LocalProductBuilderSession
    func validateBuilder(
        session: LocalProductBuilderSession
    ) async throws -> LocalProductBuilderSession
    func confirmBuilder(
        session: LocalProductBuilderSession,
        definitionID: String
    ) async throws -> LocalProductBuilderConfirmation
    func archiveTeam(
        definitionID: String,
        expectedHead: Int64
    ) async throws -> LocalProductSetupSavedTeam
    func restoreTeam(
        definitionID: String,
        expectedHead: Int64
    ) async throws -> LocalProductSetupSavedTeam
    func configureMiniMax(
        secret: String
    ) async throws -> LocalProductCredentialSetupResult
    func verifyMiniMax(
        reference: String,
        revision: Int64
    ) async throws -> LocalProductCredentialSetupResult
    func replaceMiniMax(
        reference: String,
        revision: Int64,
        secret: String
    ) async throws -> LocalProductCredentialSetupResult
    func revokeMiniMax(
        reference: String,
        revision: Int64
    ) async throws -> LocalProductCredentialSetupResult
}

@MainActor
public final class LocalProductStore: ObservableObject {
    @Published public private(set) var snapshot: LocalProductSnapshot?
    @Published public private(set) var timeline: LocalProductTimelinePage?
    @Published public private(set) var timelineState: LocalProductTimelineState = .idle
    @Published public private(set) var connectionState: LocalProductConnectionState = .loading
    @Published public private(set) var setupSnapshot: LocalProductSetupSnapshot?
    @Published public private(set) var builderSession: LocalProductBuilderSession?
    @Published public private(set) var setupState: LocalProductSetupState = .idle
    @Published public private(set) var lastConfirmation: LocalProductBuilderConfirmation?
    @Published public private(set) var credentialStatus: LocalProductCredentialSetupResult?
    @Published public private(set) var providerConnectionStatus:
        LocalProductProviderConnectResult?
    @Published public private(set) var workspace = LocalProductWorkspaceState()
    @Published public private(set) var workbench = MissionWorkspaceState()
    @Published public private(set) var activeDecisionSheet:
        LocalProductDecisionSheet?
    @Published public var selectedSection: LocalProductSection = .home
    @Published public var selectedTeamID: String?

    private let client: LocalProductClientProtocol
    private let setupClient: LocalProductSetupClientProtocol?
    private let decisionClient: LocalProductDecisionClientProtocol?

    public init(client: LocalProductClientProtocol) {
        self.client = client
        setupClient = client as? LocalProductSetupClientProtocol
        decisionClient = client as? LocalProductDecisionClientProtocol
    }

    public var providerManagementReachable: Bool {
        setupClient != nil
    }

    public var teamTimelineMessage: String {
        switch timelineState {
        case .idle:
            return "Select a Team from Teams to open its authoritative timeline."
        case .loading:
            return "Loading Team timeline."
        case .loaded:
            return ""
        case .unavailable:
            return "Team timeline is unavailable."
        case .fatal:
            return "Team timeline needs attention."
        }
    }

    public static let teamsEmptyMessage = "No Teams exist in this Journal yet."

    public func refresh() async {
        if snapshot == nil {
            connectionState = .loading
        }
        do {
            let next = try await client.snapshot(limit: 64)
            snapshot = next
            reconcileWorkspace()
            reconcileMissions()
            if next.stale {
                connectionState = .stale(
                    reason: closedReason(next.reason, fallback: "stale_view")
                )
            } else if next.partial {
                connectionState = .partial(
                    reason: closedReason(next.reason, fallback: "partial_view")
                )
            } else {
                connectionState = .online
            }
            if let selectedTeamID,
               !next.teams.contains(where: { $0.teamInstanceID == selectedTeamID }) {
                self.selectedTeamID = nil
                timeline = nil
                timelineState = .idle
            }
        } catch let remote as LocalIPCRemoteError {
            connectionState = remote.recoverable
                ? .offline(reason: remote.code.rawValue)
                : .fatal(reason: remote.code.rawValue)
        } catch {
            connectionState = .offline(reason: closedClientReason(error))
        }
    }

    public func selectTeam(_ team: LocalProductTeamSummary) {
        selectedTeamID = team.teamInstanceID
        selectedSection = .teams
        timeline = nil
        timelineState = .loading
    }

    public func activateSelectedTeam() async {
        guard let selectedTeamID,
              snapshot?.teams.contains(where: {
                  $0.teamInstanceID == selectedTeamID
              }) == true else {
            timelineState = .idle
            return
        }
        do {
            timeline = try await client.timeline(
                teamInstanceID: selectedTeamID,
                cursor: "",
                limit: 64
            )
            timelineState = .loaded
        } catch let remote as LocalIPCRemoteError {
            if remote.code == .cursorConflict || remote.code == .streamGap {
                connectionState = .stale(reason: remote.code.rawValue)
                timelineState = .unavailable
            } else if !remote.recoverable {
                connectionState = .fatal(reason: remote.code.rawValue)
                timelineState = .fatal
            } else {
                connectionState = .offline(reason: remote.code.rawValue)
                timelineState = .unavailable
            }
        } catch {
            connectionState = .offline(reason: closedClientReason(error))
            timelineState = .unavailable
        }
    }

    public func refreshSetup() async {
        guard let setupClient else {
            setupState = .unavailable(reason: "setup_unavailable")
            return
        }
        setupState = .loading
        do {
            setupSnapshot = try await setupClient.setupSnapshot()
            reconcileWorkspace()
            setupState = .ready
        } catch let remote as LocalIPCRemoteError {
            setupState = remote.recoverable
                ? .unavailable(reason: remote.code.rawValue)
                : .fatal(reason: remote.code.rawValue)
        } catch {
            setupState = .unavailable(reason: closedClientReason(error))
        }
    }

    public func openMission(_ id: String) {
        workbench.openMission(id)
    }

    public func showMissionBoard() {
        workbench.showBoard()
    }

    public func updateMissionBoardFilter(_ value: String) {
        workbench.updateBoardFilter(value)
    }

    public func updateMissionComposerDraft(_ value: String) {
        workbench.updateComposerDraft(value)
    }

    public func selectMissionPermissionMode(_ mode: MissionPermissionMode) {
        workbench.selectPermissionMode(mode)
    }

    public func selectMissionInspector(_ tab: MissionInspectorTab) {
        workbench.selectInspector(tab)
    }

    public func setMissionInspectorVisible(_ visible: Bool) {
        workbench.setInspectorVisible(visible)
    }

    public func decideMission(
        _ command: LocalProductDecisionCommand
    ) async throws -> LocalProductDecisionResult {
        guard let decisionClient else {
            throw LocalProductClientError.unavailable
        }
        let result = try await decisionClient.decideMission(command)
        await refresh()
        return result
    }

    public func openPreparedDecision(
        _ command: LocalProductDecisionCommand
    ) async {
        guard let decisionClient, command.operation == "read" else {
            activeDecisionSheet = nil
            return
        }
        do {
            activeDecisionSheet = try await decisionClient.readMissionDecision(
                command
            )
        } catch {
            activeDecisionSheet = nil
        }
    }

    public func openReadOnlyReviewDecision(
        for mission: LocalProductMissionSummary
    ) {
        guard mission.lane == "Review",
              preparedDecisionCommand(
                for: mission.missionID,
                kind: .review
              ) == nil else {
            activeDecisionSheet = nil
            return
        }
        activeDecisionSheet = LocalProductDecisionSheet(
            kind: .review,
            missionID: mission.missionID,
            teamInstanceID: mission.teamInstanceID,
            viewVersion: snapshot?.viewVersion ?? "",
            decisionID: "unprepared-review-\(mission.missionID)",
            decisionDigest: "",
            title: "Review Gate unavailable",
            summary: "This Mission is waiting for review, but no authoritative acceptance command is prepared.",
            requester: "Loom authority",
            target: mission.title,
            commandType: "No terminal verification is available",
            networkAccess: "Not accepted",
            credentialAccess: "none",
            permissionScope: "read-only",
            attemptScope: mission.currentNodeID.isEmpty
                ? "No current Attempt"
                : "Current node \(mission.currentNodeID)",
            expectedEvidence: "Accepted terminal Evidence is required before completion",
            technicalDetails: [
                "No Journal mutation is available from this sheet.",
                "Accept Result remains disabled until the authority prepares an exact command.",
            ],
            actions: ["not_now", "request_changes", "accept_result"],
            preparedActions: [],
            prepared: false,
            logicalNodeID: mission.currentNodeID,
            attemptNumber: 0,
            claimGeneration: 0
        )
    }

    public func preparedDecisionCommand(
        for missionID: String,
        kind: LocalProductDecisionKind? = nil
    ) -> LocalProductDecisionCommand? {
        snapshot?.preparedDecisions.first {
            $0.missionID == missionID && (kind == nil || $0.kind == kind)
        }
    }

    public func dismissDecisionSheet() {
        activeDecisionSheet = nil
    }

    public func submitDecisionAction(_ action: String) async {
        guard let sheet = activeDecisionSheet else { return }
        if !sheet.prepared {
            if action == "not_now" || action == "edit_scope" {
                activeDecisionSheet = nil
            }
            return
        }
        let operation = action == "not_now" || action == "edit_scope"
            ? "defer"
            : "submit"
        let command = LocalProductDecisionCommand(
            operation: operation,
            kind: sheet.kind,
            action: action,
            missionID: sheet.missionID,
            teamInstanceID: sheet.teamInstanceID,
            viewVersion: sheet.viewVersion,
            decisionID: sheet.decisionID,
            decisionDigest: sheet.decisionDigest,
            logicalNodeID: sheet.logicalNodeID,
            attemptNumber: sheet.attemptNumber,
            claimGeneration: sheet.claimGeneration,
            correlationID: UUID().uuidString.lowercased()
        )
        do {
            _ = try await decideMission(command)
            activeDecisionSheet = nil
        } catch {
            // Keep the exact prepared sheet visible after conflict/failure.
        }
    }

    public func connectCodex() async {
        guard let setupClient else {
            setupState = .unavailable(reason: "setup_unavailable")
            return
        }
        setupState = .loading
        do {
            providerConnectionStatus = try await setupClient.connectCodex()
            for attempt in 0..<120 {
                let next = try await setupClient.setupSnapshot()
                setupSnapshot = next
                reconcileWorkspace()
                if next.codex.status == "available" {
                    setupState = .ready
                    return
                }
                if attempt < 119 {
                    try await Task<Never, Never>.sleep(
                        nanoseconds: 1_000_000_000
                    )
                }
            }
            setupState = .unavailable(reason: "timeout")
        } catch is CancellationError {
            setupState = .ready
        } catch {
            handleSetupError(error)
        }
    }

    public func startBlankBuilder() async {
        await startBuilder(
            source: "blank",
            sourceID: "",
            sourceVersion: 0,
            sourceDigest: ""
        )
    }

    public func startBuilder(
        source: String,
        sourceID: String,
        sourceVersion: Int,
        sourceDigest: String
    ) async {
        guard let setupClient else {
            setupState = .unavailable(reason: "setup_unavailable")
            return
        }
        setupState = .loading
        do {
            builderSession = try await setupClient.startBuilder(
                source: source,
                sourceID: sourceID,
                sourceVersion: sourceVersion,
                sourceDigest: sourceDigest
            )
            lastConfirmation = nil
            setupState = .ready
        } catch {
            handleSetupError(error)
        }
    }

    public func startBuilder(from team: LocalProductSetupSavedTeam) async {
        guard team.status == "active" else {
            setupState = .unavailable(reason: "incompatible")
            return
        }
        await startBuilder(
            source: "saved_team",
            sourceID: team.id,
            sourceVersion: team.version,
            sourceDigest: team.definitionDigest
        )
    }

    public func startBuilder(from template: LocalProductSetupTemplate) async {
        await startBuilder(
            source: "template",
            sourceID: template.id,
            sourceVersion: template.version,
            sourceDigest: template.digest
        )
    }

    public func answerBuilder(_ answer: String) async {
        guard let setupClient, let builderSession else {
            setupState = .unavailable(reason: "builder_unavailable")
            return
        }
        setupState = .loading
        do {
            self.builderSession = try await setupClient.answerBuilder(
                session: builderSession,
                answer: answer
            )
            setupState = .ready
        } catch {
            handleSetupError(error)
        }
    }

    public func confirmBuilder() async {
        guard let setupClient, let builderSession, builderSession.canConfirm else {
            setupState = .unavailable(reason: "confirmation_unavailable")
            return
        }
        setupState = .loading
        do {
            let definitionID = "team-\(UUID().uuidString.lowercased())"
            lastConfirmation = try await setupClient.confirmBuilder(
                session: builderSession,
                definitionID: definitionID
            )
            self.builderSession = nil
            setupSnapshot = try await setupClient.setupSnapshot()
            reconcileWorkspace()
            setupState = .ready
        } catch {
            handleSetupError(error)
        }
    }

    public func editBuilder(field: String, value: String) async {
        guard let setupClient, let builderSession,
              Self.allowsBuilderEditField(field),
              !value.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
        else {
            setupState = .unavailable(reason: "invalid_request")
            return
        }
        setupState = .loading
        do {
            self.builderSession = try await setupClient.editBuilder(
                session: builderSession,
                field: field,
                value: value
            )
            setupState = .ready
        } catch {
            handleSetupError(error)
        }
    }

    public static func allowsBuilderEditField(_ field: String) -> Bool {
        ["team_name", "purpose", "main_role", "subagent_role"].contains(field)
    }

    public func cancelBuilder() {
        builderSession = nil
        lastConfirmation = nil
        setupState = .ready
    }

    public func selectWorkspaceTask(_ id: String) {
        workspace.selectTask(id)
    }

    public func activateWorkspaceTask() async {
        let prefix = "team:"
        guard workspace.selectedTaskID.hasPrefix(prefix),
              let team = snapshot?.teams.first(where: {
                  "team:\($0.teamInstanceID)" == workspace.selectedTaskID
              }) else {
            return
        }
        selectTeam(team)
        await activateSelectedTeam()
    }

    public func updateComposerDraft(_ value: String) {
        workspace.updateComposerDraft(value)
    }

    public func selectInspector(_ inspector: LocalProductInspectorTab) {
        workspace.selectInspector(inspector)
    }

    public func setDeveloperDetailsExpanded(_ expanded: Bool) {
        workspace.setDeveloperDetailsExpanded(expanded)
    }

    public func archiveTeam(_ team: LocalProductSetupSavedTeam) async {
        await setTeamStatus(team, restoring: false)
    }

    public func restoreTeam(_ team: LocalProductSetupSavedTeam) async {
        await setTeamStatus(team, restoring: true)
    }

    private func setTeamStatus(
        _ team: LocalProductSetupSavedTeam,
        restoring: Bool
    ) async {
        guard let setupClient, team.streamHead > 0 else {
            setupState = .unavailable(reason: "invalid_request")
            return
        }
        setupState = .loading
        do {
            if restoring {
                _ = try await setupClient.restoreTeam(
                    definitionID: team.id,
                    expectedHead: team.streamHead
                )
            } else {
                _ = try await setupClient.archiveTeam(
                    definitionID: team.id,
                    expectedHead: team.streamHead
                )
            }
            setupSnapshot = try await setupClient.setupSnapshot()
            reconcileWorkspace()
            setupState = .ready
        } catch {
            handleSetupError(error)
        }
    }

    public func configureMiniMax(secret: String) async {
        guard let setupClient else {
            setupState = .unavailable(reason: "credential_unavailable")
            return
        }
        setupState = .loading
        do {
            credentialStatus = try await setupClient.configureMiniMax(
                secret: secret
            )
            setupSnapshot = try await setupClient.setupSnapshot()
            reconcileWorkspace()
            setupState = .ready
        } catch {
            handleSetupError(error)
        }
    }

    public func verifyMiniMax() async {
        guard let setupClient,
              let provider = setupSnapshot?.miniMax,
              !provider.credentialReference.isEmpty,
              provider.revision > 0 else {
            setupState = .unavailable(reason: "credential_unavailable")
            return
        }
        setupState = .loading
        do {
            credentialStatus = try await setupClient.verifyMiniMax(
                reference: provider.credentialReference,
                revision: provider.revision
            )
            setupSnapshot = try await setupClient.setupSnapshot()
            reconcileWorkspace()
            setupState = .ready
        } catch {
            handleSetupError(error)
        }
    }

    public func replaceMiniMax(secret: String) async {
        guard let setupClient,
              let provider = setupSnapshot?.miniMax,
              !provider.credentialReference.isEmpty,
              provider.revision > 0 else {
            setupState = .unavailable(reason: "credential_unavailable")
            return
        }
        setupState = .loading
        do {
            credentialStatus = try await setupClient.replaceMiniMax(
                reference: provider.credentialReference,
                revision: provider.revision,
                secret: secret
            )
            setupSnapshot = try await setupClient.setupSnapshot()
            reconcileWorkspace()
            setupState = .ready
        } catch {
            handleSetupError(error)
        }
    }

    public func revokeMiniMax() async {
        guard let setupClient,
              let provider = setupSnapshot?.miniMax,
              !provider.credentialReference.isEmpty,
              provider.revision > 0 else {
            setupState = .unavailable(reason: "credential_unavailable")
            return
        }
        setupState = .loading
        do {
            credentialStatus = try await setupClient.revokeMiniMax(
                reference: provider.credentialReference,
                revision: provider.revision
            )
            setupSnapshot = try await setupClient.setupSnapshot()
            reconcileWorkspace()
            setupState = .ready
        } catch {
            handleSetupError(error)
        }
    }

    private func handleSetupError(_ error: Error) {
        if let remote = error as? LocalIPCRemoteError {
            setupState = remote.recoverable
                ? .unavailable(reason: remote.code.rawValue)
                : .fatal(reason: remote.code.rawValue)
        } else {
            setupState = .unavailable(reason: closedClientReason(error))
        }
    }

    private func reconcileWorkspace() {
        let teams = (snapshot?.teams ?? []).map { team in
            LocalProductWorkspaceTask(
                id: "team:\(team.teamInstanceID)",
                title: workspaceVisibleName(
                    team.displayName,
                    internalID: team.teamInstanceID,
                    fallback: "Saved team"
                ),
                subtitle: humanWorkspaceStatus(team.state),
                kind: .team
            )
        }
        let runs = (snapshot?.runs ?? []).map { run in
            LocalProductWorkspaceTask(
                id: "run:\(run.runID)",
                title: "Recent work",
                subtitle: humanWorkspaceStatus(
                    run.terminalStatus.isEmpty ? run.phase : run.terminalStatus
                ),
                kind: .history
            )
        }
        let attention = (snapshot?.attention ?? []).map { item in
            LocalProductWorkspaceTask(
                id: "attention:\(item.attentionID)",
                title: "Needs your attention",
                subtitle: humanWorkspaceStatus(item.status),
                kind: .attention
            )
        }
        let saved = (setupSnapshot?.savedTeams ?? []).map { team in
            LocalProductWorkspaceTask(
                id: "saved:\(team.id)",
                title: team.name.isEmpty ? "Saved team" : team.name,
                subtitle: humanWorkspaceStatus(team.status),
                kind: .team
            )
        }
        workspace.mergeAuthoritativeTasks(attention + runs + teams + saved)
    }

    private func reconcileMissions() {
        workbench.mergeMissions(
            snapshot?.missions.compactMap(\.listItem) ?? []
        )
    }

    private func humanWorkspaceStatus(_ value: String) -> String {
        value.replacingOccurrences(of: "_", with: " ").capitalized
    }

    private func workspaceVisibleName(
        _ candidate: String,
        internalID: String,
        fallback: String
    ) -> String {
        let bounded = String(candidate.prefix(96))
        let safe = bounded.unicodeScalars.filter { scalar in
            !CharacterSet.controlCharacters.contains(scalar) &&
                !Self.workspaceBidiOverrides.contains(scalar.value)
        }
        let visible = String(String.UnicodeScalarView(safe))
            .trimmingCharacters(in: .whitespacesAndNewlines)
        return visible.isEmpty || visible == internalID ? fallback : visible
    }

    private static let workspaceBidiOverrides: Set<UInt32> = [
        0x202A, 0x202B, 0x202C, 0x202D, 0x202E,
        0x2066, 0x2067, 0x2068, 0x2069,
    ]

    private func closedReason(_ value: String, fallback: String) -> String {
        let allowed = Set([
            "projection_refresh_failed", "partial_view", "stale_view",
        ])
        return allowed.contains(value) ? value : fallback
    }

    private func closedClientReason(_ error: Error) -> String {
        if let clientError = error as? LocalProductClientError {
            return clientError.rawValue
        }
        if let wireError = error as? LocalProductWireError {
            switch wireError {
            case .unsupportedSchema: return "unsupported_schema"
            case .invalidJSON, .unknownField: return "invalid_response"
            }
        }
        return "unavailable"
    }
}
