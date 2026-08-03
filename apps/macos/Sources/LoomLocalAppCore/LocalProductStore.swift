import Foundation
import SwiftUI
import CryptoKit

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
        let newTask =
            tasks.first(where: { $0.id == Self.newTaskID })
            ?? LocalProductWorkspaceTask(
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
            task.id == selected || task.title.localizedCaseInsensitiveContains(normalized)
                || task.subtitle.localizedCaseInsensitiveContains(normalized)
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

public enum LocalProductExecutionState: Equatable, Sendable {
    case idle
    case preflighting
    case ready
    case starting
    case cancelling
    case running
    case awaitingRecovery
    case succeeded
    case cancelled
    case failed(reason: String)
}

public struct LocalProductWorkPackageOption: Identifiable, Equatable, Sendable {
    public let id: String
    public let title: String
    public let digest: String

    public static let coding = Self(
        id: "work-package.coding",
        title: "Coding",
        digest: "4eea514fca13aa241cd004277e31c9c1fe296d34646ceafd618809b6b22c8a4f"
    )
    public static let knowledge = Self(
        id: "work-package.knowledge",
        title: "Knowledge",
        digest: "5a834baad4a0e4c557d96883f242860f4d136de208dd6721b85d8c2b8c5a0393"
    )
    public static let accepted = [coding, knowledge]
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

public protocol LocalProductHandoffClientProtocol {
    func proposeSideTask(
        _ request: LocalProductSideTaskProposalRequest
    ) async throws -> LocalProductSideTaskProposalResult
    func createSideTask(
        _ request: LocalProductSideTaskCreateRequest
    ) async throws -> LocalProductSideTaskCreateResult
    func readSideTask(
        _ request: LocalProductSideTaskReadRequest
    ) async throws -> LocalProductSideTaskReadResult
    func decideSideTask(
        _ request: LocalProductSideTaskDecisionRequest
    ) async throws -> LocalProductSideTaskDecisionResult
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
    @Published public private(set) var miniMaxVerificationStatus = "Idle"
    @Published public private(set) var isVerifyingMiniMax = false
    @Published public private(set) var providerConnectionStatus: LocalProductProviderConnectResult?
    @Published public private(set) var workspace = LocalProductWorkspaceState()
    @Published public private(set) var workbench = MissionWorkspaceState()
    @Published public private(set) var activeDecisionSheet: LocalProductDecisionSheet?
    @Published public private(set) var executionState: LocalProductExecutionState = .idle
    @Published public private(set) var executionPreflight: LocalProductExecutionPreflight?
    @Published public private(set) var executionResult: LocalProductExecutionResult?
    @Published public private(set) var sideTaskProposal: LocalProductSideTaskProposalResult?
    @Published public private(set) var sideTaskOperationStatus = "Idle"
    @Published public var selectedSection: LocalProductSection = .home
    @Published public var selectedTeamID: String?

    private let client: LocalProductClientProtocol
    private let setupClient: LocalProductSetupClientProtocol?
    private let decisionClient: LocalProductDecisionClientProtocol?
    private let executionClient: LocalProductExecutionClientProtocol?
    private let handoffClient: LocalProductHandoffClientProtocol?
    private var executionObjective = ""
    private var pendingSideTaskProposalRequest: LocalProductSideTaskProposalRequest?
    private var timelineLoadGeneration: UInt64 = 0

    private static let timelinePageLimit = 64
    private static let maximumTimelinePages = 8
    private static let maximumTimelineRecords = 512

    private enum TimelineLoadSelection: Equatable {
        case team(String)
        case mission(id: String, teamID: String)
    }

    public init(client: LocalProductClientProtocol) {
        self.client = client
        setupClient = client as? LocalProductSetupClientProtocol
        decisionClient = client as? LocalProductDecisionClientProtocol
        executionClient = client as? LocalProductExecutionClientProtocol
        handoffClient = client as? LocalProductHandoffClientProtocol
    }

    public var providerManagementReachable: Bool {
        setupClient != nil
    }

    public var missionExecutionReachable: Bool { executionClient != nil }

    public var sideTaskHandoffReachable: Bool { handoffClient != nil }

    public func canCreateSideTask(for missionID: String) -> Bool {
        sideTaskParentBinding(missionID: missionID) != nil
    }

    public func proposeSideTask(
        missionID: String,
        purpose: String,
        mode: String,
        title: String,
        authorizedRequest: String
    ) async {
        let boundedTitle = title.trimmingCharacters(in: .whitespacesAndNewlines)
        let boundedRequest = authorizedRequest.trimmingCharacters(in: .whitespacesAndNewlines)
        guard let handoffClient, let snapshot,
              let binding = sideTaskParentBinding(missionID: missionID),
              (1...128).contains(boundedTitle.utf8.count),
              (1...4_096).contains(boundedRequest.utf8.count),
              ["research", "comparison", "diagnosis", "verification", "read_only_review"].contains(purpose),
              ["report_only", "decision_required", "merge_candidate"].contains(mode)
        else {
            sideTaskOperationStatus = "proposal_unavailable"
            return
        }
        let request = LocalProductSideTaskProposalRequest(
            parentMissionID: missionID,
            parentTeamInstanceID: binding.teamInstanceID,
            parentTaskID: binding.workItemID,
            parentRunID: binding.runID,
            parentClaimGeneration: binding.claimGeneration,
            parentExecutionDigest: binding.executionDigest,
            purpose: purpose,
            mode: mode,
            title: boundedTitle,
            authorizedRequest: boundedRequest,
            permissionScopes: [],
            decisionTimeoutSeconds: mode == "report_only" ? 0 : 900,
            expectedViewVersion: snapshot.viewVersion,
            correlationID: UUID().uuidString.lowercased()
        )
        sideTaskOperationStatus = "Proposing"
        do {
            let proposal = try await handoffClient.proposeSideTask(request)
            guard proposal.viewVersion == snapshot.viewVersion,
                  proposal.purpose == purpose, proposal.mode == mode,
                  proposal.title == boundedTitle else {
                throw LocalProductClientError.invalidResponse
            }
            pendingSideTaskProposalRequest = request
            sideTaskProposal = proposal
            sideTaskOperationStatus = "Confirmation required"
        } catch {
            pendingSideTaskProposalRequest = nil
            sideTaskProposal = nil
            sideTaskOperationStatus = closedClientReason(error)
        }
    }

    public func confirmSideTaskProposal() async {
        guard let handoffClient, let proposal = sideTaskProposal,
              let request = pendingSideTaskProposalRequest else {
            sideTaskOperationStatus = "proposal_required"
            return
        }
        sideTaskOperationStatus = "Creating"
        do {
            let result = try await handoffClient.createSideTask(
                LocalProductSideTaskCreateRequest(
                    proposal: request,
                    proposalDigest: proposal.proposalDigest,
                    confirmed: true
                )
            )
            guard result.proposalDigest == proposal.proposalDigest else {
                throw LocalProductClientError.invalidResponse
            }
            pendingSideTaskProposalRequest = nil
            sideTaskProposal = nil
            sideTaskOperationStatus = missionHumanSideTaskStatus(result.status)
            await refresh()
        } catch {
            sideTaskOperationStatus = closedClientReason(error)
        }
    }

    public func discardSideTaskProposal() {
        pendingSideTaskProposalRequest = nil
        sideTaskProposal = nil
        sideTaskOperationStatus = "Idle"
    }

    public func decideSideTask(
        _ sideTask: LocalProductSideTaskSummary,
        decision: String
    ) async {
        guard let handoffClient, let snapshot,
              sideTask.availableDecisions.contains(decision),
              let binding = projectedSideTaskParentBinding(
                  missionID: sideTask.parentMissionID,
                  teamInstanceID: sideTask.parentTeamInstanceID,
                  workItemID: sideTask.parentTaskID,
                  runID: sideTask.parentRunID,
                  executionDigest: sideTask.parentExecutionDigest
              ),
              binding.workItemID == sideTask.parentTaskID,
              binding.runID == sideTask.parentRunID,
              binding.claimGeneration == sideTask.parentClaimGeneration,
              binding.executionDigest == sideTask.parentExecutionDigest
        else {
            sideTaskOperationStatus = "decision_unavailable"
            return
        }
        let effectDigest = sideTaskDigest(
            [
                "parent-effect-v1", sideTask.sideTaskID, sideTask.parentTeamInstanceID,
                sideTask.parentTaskID, sideTask.parentRunID,
                String(sideTask.parentClaimGeneration), sideTask.parentExecutionDigest,
                sideTask.handoffDigest, decision,
            ]
        )
        let request = LocalProductSideTaskDecisionRequest(
            sideTask: sideTask,
            parentLogicalNodeID: binding.logicalNodeID,
            parentAttemptNumber: binding.attemptNumber,
            parentExecutionDigest: sideTask.parentExecutionDigest,
            decision: decision,
            effectDigest: effectDigest,
            expectedViewVersion: snapshot.viewVersion,
            correlationID: UUID().uuidString.lowercased()
        )
        sideTaskOperationStatus = "Applying decision"
        do {
            let result = try await handoffClient.decideSideTask(request)
            guard result.sideTaskID == sideTask.sideTaskID,
                  result.decision == decision else {
                throw LocalProductClientError.invalidResponse
            }
            sideTaskOperationStatus = missionHumanSideTaskStatus(result.status)
            await refresh()
        } catch {
            sideTaskOperationStatus = closedClientReason(error)
        }
    }

    private struct SideTaskParentBinding {
        let teamInstanceID: String
        let logicalNodeID: String
        let attemptNumber: Int
        let workItemID: String
        let runID: String
        let claimGeneration: Int64
        let executionDigest: String
    }

    private func sideTaskParentBinding(
        missionID: String,
        workItemID: String = "",
        runID: String = ""
    ) -> SideTaskParentBinding? {
        guard let executionResult,
              executionResult.missionID == missionID,
              let mission = snapshot?.missions.first(where: { $0.missionID == missionID }),
              executionResult.teamInstanceID == mission.teamInstanceID
        else { return nil }
        return projectedSideTaskParentBinding(
            missionID: missionID,
            teamInstanceID: mission.teamInstanceID,
            workItemID: workItemID,
            runID: runID,
            executionDigest: executionResult.executionDigest
        )
    }

    private func projectedSideTaskParentBinding(
        missionID: String,
        teamInstanceID: String,
        workItemID: String,
        runID: String,
        executionDigest: String
    ) -> SideTaskParentBinding? {
        guard let snapshot, let timeline,
              executionDigest.count == 64,
              let mission = snapshot.missions.first(where: { $0.missionID == missionID }),
              mission.teamInstanceID == teamInstanceID,
              timeline.teamInstanceID == teamInstanceID,
              timeline.board.teamInstanceID == teamInstanceID,
              timeline.viewVersion == snapshot.viewVersion,
              let node = timeline.board.nodes.first(where: {
                  !$0.workItemID.isEmpty && !$0.runID.isEmpty && $0.currentAttempt > 0
                      && (workItemID.isEmpty || $0.workItemID == workItemID)
                      && (runID.isEmpty || $0.runID == runID)
              }),
              let run = snapshot.runs.first(where: {
                  $0.runID == node.runID && $0.workItemID == node.workItemID
              }) else { return nil }
        return SideTaskParentBinding(
            teamInstanceID: teamInstanceID,
            logicalNodeID: node.logicalNodeID,
            attemptNumber: node.currentAttempt,
            workItemID: node.workItemID,
            runID: node.runID,
            claimGeneration: run.claimGeneration,
            executionDigest: executionDigest
        )
    }

    private func sideTaskDigest(_ fields: [String]) -> String {
        SHA256.hash(data: Data(fields.joined(separator: "\0").utf8))
            .map { String(format: "%02x", $0) }.joined()
    }

    private func missionHumanSideTaskStatus(_ value: String) -> String {
        value.replacingOccurrences(of: "_", with: " ").capitalized
    }

    public var missionCancelAvailable: Bool {
        currentMissionCancelBinding() != nil
    }

    public func preflightMission(
        objective: String,
        team: LocalProductTeamSummary,
        workPackage: LocalProductWorkPackageOption
    ) async {
        let bounded = objective.trimmingCharacters(in: .whitespacesAndNewlines)
        guard let executionClient,
            let snapshot,
            team.confirmed, team.executable, !team.readOnly,
            snapshot.teams.contains(where: {
                $0.teamInstanceID == team.teamInstanceID && $0.confirmed && $0.executable
                    && !$0.readOnly
            }),
            (1...4_096).contains(bounded.utf8.count),
            workPackage == .coding || workPackage == .knowledge
        else {
            executionState = .failed(reason: "preflight_unavailable")
            return
        }
        executionState = .preflighting
        executionPreflight = nil
        executionResult = nil
        let missionID = "mission/\(team.teamInstanceID)"
        let command = LocalProductExecutionCommand.preflight(
            missionID: missionID,
            teamInstanceID: team.teamInstanceID,
            workPackageID: workPackage.id,
            workPackageDigest: workPackage.digest,
            objective: bounded,
            expectedViewVersion: snapshot.viewVersion,
            correlationID: UUID().uuidString.lowercased()
        )
        do {
            let envelope = try await executionClient.executeMission(command)
            guard let preflight = envelope.preflight else {
                throw LocalProductClientError.invalidResponse
            }
            executionObjective = bounded
            executionPreflight = preflight
            executionState = .ready
        } catch {
            executionState = .failed(reason: closedClientReason(error))
        }
    }

    public func startPreflightedMission() async {
        guard let executionClient,
            let preflight = executionPreflight,
            !executionObjective.isEmpty
        else {
            executionState = .failed(reason: "preflight_required")
            return
        }

        executionState = .starting
        do {
            let command = LocalProductExecutionCommand.start(
                preflight: preflight,
                objective: executionObjective,
                correlationID: UUID().uuidString.lowercased()
            )
            let envelope = try await executionClient.executeMission(command)
            guard let result = envelope.result else {
                throw LocalProductClientError.invalidResponse
            }
            executionResult = result
            switch result.status {
            case "running": executionState = .running
            case "awaiting_recovery": executionState = .awaitingRecovery
            case "succeeded": executionState = .succeeded
            default: executionState = .failed(reason: result.status)
            }
            await refresh()
            guard
                snapshot?.missions.contains(where: {
                    $0.missionID == result.missionID && $0.teamInstanceID == result.teamInstanceID
                }) == true
            else {
                executionState = .failed(reason: "projection_visibility_failed")
                return
            }
            await openMissionAndActivate(result.missionID)
        } catch {
            executionState = .failed(reason: closedClientReason(error))
        }
    }

    public func cancelCurrentMission() async {
        guard let executionClient,
            let result = executionResult,
            let binding = currentMissionCancelBinding()
        else {
            executionState = .failed(reason: "cancel_unavailable")
            return
        }
        executionState = .cancelling
        let command = LocalProductExecutionCommand.control(
            missionID: result.missionID,
            teamInstanceID: result.teamInstanceID,
            expectedViewVersion: binding.viewVersion,
            action: "cancel",
            executionDigest: result.executionDigest,
            logicalNodeID: binding.logicalNodeID,
            attemptNumber: binding.attemptNumber,
            claimGeneration: binding.claimGeneration,
            correlationID: UUID().uuidString.lowercased()
        )
        do {
            let envelope = try await executionClient.executeMission(command)
            guard let cancelled = envelope.result,
                cancelled.status == "cancelled",
                cancelled.missionID == result.missionID,
                cancelled.teamInstanceID == result.teamInstanceID,
                cancelled.executionDigest == result.executionDigest
            else {
                throw LocalProductClientError.invalidResponse
            }
            executionResult = cancelled
            await refreshMission(cancelled.missionID)
            guard
                snapshot?.missions.contains(where: {
                    $0.missionID == cancelled.missionID && $0.status == "cancelled"
                }) == true
            else {
                executionState = .failed(reason: "cancel_visibility_failed")
                return
            }
            executionState = .cancelled
        } catch {
            executionState = .failed(reason: closedClientReason(error))
        }
    }

    private struct MissionCancelBinding {
        let viewVersion: String
        let logicalNodeID: String
        let attemptNumber: Int
        let claimGeneration: Int64
    }

    private func currentMissionCancelBinding() -> MissionCancelBinding? {
        guard let result = executionResult,
            let snapshot,
            let timeline,
            case .mission(let selectedMissionID) = workbench.route,
            selectedMissionID == result.missionID,
            timeline.teamInstanceID == result.teamInstanceID,
            timeline.board.teamInstanceID == result.teamInstanceID,
            timeline.viewVersion == snapshot.viewVersion,
            timeline.board.viewVersion == snapshot.viewVersion,
            result.executionDigest.count == 64
        else { return nil }
        for node in timeline.board.nodes
        where
            node.currentAttempt > 0 && !node.runID.isEmpty
            && !(["succeeded", "failed", "cancelled"].contains(node.status))
        {
            guard
                let run = snapshot.runs.first(where: {
                    $0.runID == node.runID && $0.claimGeneration > 0 && $0.terminalStatus.isEmpty
                })
            else { continue }
            return MissionCancelBinding(
                viewVersion: snapshot.viewVersion,
                logicalNodeID: node.logicalNodeID,
                attemptNumber: node.currentAttempt,
                claimGeneration: run.claimGeneration
            )
        }
        return nil
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
                !next.teams.contains(where: { $0.teamInstanceID == selectedTeamID })
            {
                invalidateTimelineLoad()
                self.selectedTeamID = nil
                timeline = nil
                timelineState = .idle
            }
        } catch let remote as LocalIPCRemoteError {
            connectionState =
                remote.recoverable
                ? .offline(reason: remote.code.rawValue)
                : .fatal(reason: remote.code.rawValue)
        } catch {
            connectionState = .offline(reason: closedClientReason(error))
        }
    }

    public func selectTeam(_ team: LocalProductTeamSummary) {
        invalidateTimelineLoad()
        selectedTeamID = team.teamInstanceID
        selectedSection = .teams
        timeline = nil
        timelineState = .loading
    }

    public func activateSelectedTeam() async {
        guard let selectedTeamID,
            snapshot?.teams.contains(where: {
                $0.teamInstanceID == selectedTeamID
            }) == true
        else {
            timelineState = .idle
            return
        }
        await loadTimeline(
            teamInstanceID: selectedTeamID,
            selection: .team(selectedTeamID)
        )
    }

    private func loadTimeline(
        teamInstanceID: String,
        selection: TimelineLoadSelection
    ) async {
        let generation = beginTimelineLoad()
        timeline = nil
        timelineState = .loading
        do {
            var cursor = ""
            var seenCursors = Set<String>()
            var seenDeliveryIDs = Set<String>()
            var records: [LocalProductTimelineRecord] = []
            var firstPage: LocalProductTimelinePage?

            for pageIndex in 0..<Self.maximumTimelinePages {
                try Task.checkCancellation()
                let page = try await client.timeline(
                    teamInstanceID: teamInstanceID,
                    cursor: cursor,
                    limit: Self.timelinePageLimit
                )
                try Task.checkCancellation()
                guard timelineLoadIsCurrent(
                    generation: generation,
                    selection: selection
                ) else { return }
                try Self.validateTimelinePage(
                    page,
                    teamInstanceID: teamInstanceID,
                    firstPage: firstPage,
                    seenDeliveryIDs: &seenDeliveryIDs
                )
                if firstPage == nil { firstPage = page }
                records.append(contentsOf: page.records)
                guard records.count <= Self.maximumTimelineRecords else {
                    throw LocalProductClientError.invalidResponse
                }
                if !page.hasMore {
                    guard let firstPage else {
                        throw LocalProductClientError.invalidResponse
                    }
                    timeline = LocalProductTimelinePage(
                        schemaVersion: firstPage.schemaVersion,
                        teamInstanceID: teamInstanceID,
                        viewVersion: firstPage.viewVersion,
                        nextCursor: page.nextCursor,
                        hasMore: false,
                        gap: nil,
                        records: records,
                        board: firstPage.board,
                        attention: firstPage.attention
                    )
                    timelineState = .loaded
                    return
                }
                guard pageIndex + 1 < Self.maximumTimelinePages,
                      records.count < Self.maximumTimelineRecords,
                      !page.nextCursor.isEmpty,
                      page.nextCursor != cursor,
                      seenCursors.insert(page.nextCursor).inserted else {
                    throw LocalProductClientError.invalidResponse
                }
                cursor = page.nextCursor
            }
            throw LocalProductClientError.invalidResponse
        } catch is CancellationError {
            guard timelineLoadIsCurrent(
                generation: generation,
                selection: selection
            ) else { return }
            timeline = nil
            timelineState = .idle
        } catch let remote as LocalIPCRemoteError {
            if Task.isCancelled {
                guard timelineLoadIsCurrent(
                    generation: generation,
                    selection: selection
                ) else { return }
                timeline = nil
                timelineState = .idle
                return
            }
            guard timelineLoadIsCurrent(
                generation: generation,
                selection: selection
            ) else { return }
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
            if Task.isCancelled {
                guard timelineLoadIsCurrent(
                    generation: generation,
                    selection: selection
                ) else { return }
                timeline = nil
                timelineState = .idle
                return
            }
            guard timelineLoadIsCurrent(
                generation: generation,
                selection: selection
            ) else { return }
            connectionState = .offline(reason: closedClientReason(error))
            timelineState = .unavailable
        }
    }

    private func beginTimelineLoad() -> UInt64 {
        timelineLoadGeneration &+= 1
        return timelineLoadGeneration
    }

    private func invalidateTimelineLoad() {
        timelineLoadGeneration &+= 1
    }

    private func closeTimelineLoadPresentation() {
        invalidateTimelineLoad()
        timeline = nil
        timelineState = .idle
    }

    private func timelineLoadIsCurrent(
        generation: UInt64,
        selection: TimelineLoadSelection
    ) -> Bool {
        guard generation == timelineLoadGeneration else { return false }
        switch selection {
        case .team(let teamID):
            return selectedTeamID == teamID
        case .mission(let missionID, _):
            return workbench.route == .mission(missionID)
        }
    }

    private static func validateTimelinePage(
        _ page: LocalProductTimelinePage,
        teamInstanceID: String,
        firstPage: LocalProductTimelinePage?,
        seenDeliveryIDs: inout Set<String>
    ) throws {
        guard page.schemaVersion == 1,
              page.teamInstanceID == teamInstanceID,
              !page.viewVersion.isEmpty,
              page.gap == nil,
              page.board.schemaVersion == 1,
              page.board.teamInstanceID == teamInstanceID,
              page.board.viewVersion == page.viewVersion else {
            throw LocalProductClientError.invalidResponse
        }
        var attentionIDs = Set<String>()
        for attention in page.attention {
            guard attention.schemaVersion == 1,
                  attention.teamInstanceID == teamInstanceID,
                  !attention.attentionID.isEmpty,
                  attentionIDs.insert(attention.attentionID).inserted else {
                throw LocalProductClientError.invalidResponse
            }
        }
        if let firstPage {
            guard page.viewVersion == firstPage.viewVersion,
                  page.board == firstPage.board,
                  page.attention == firstPage.attention else {
                throw LocalProductClientError.invalidResponse
            }
        }
        for record in page.records {
            guard record.schemaVersion == 1,
                  record.teamInstanceID == teamInstanceID,
                  !record.deliveryID.isEmpty,
                  seenDeliveryIDs.insert(record.deliveryID).inserted else {
                throw LocalProductClientError.invalidResponse
            }
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
            setupState =
                remote.recoverable
                ? .unavailable(reason: remote.code.rawValue)
                : .fatal(reason: remote.code.rawValue)
        } catch {
            setupState = .unavailable(reason: closedClientReason(error))
        }
    }

    public func openMission(_ id: String) {
        invalidateTimelineLoad()
        workbench.openMission(id)
    }

    public func openMissionAndActivate(_ id: String) async {
        guard
            let mission = snapshot?.missions.first(where: {
                $0.missionID == id
            })
        else { return }
        workbench.openMission(id)
        await loadTimeline(
            teamInstanceID: mission.teamInstanceID,
            selection: .mission(id: id, teamID: mission.teamInstanceID)
        )
    }

    public func refreshMission(_ id: String) async {
        await refresh()
        await openMissionAndActivate(id)
    }

    public func showMissionBoard() {
        closeTimelineLoadPresentation()
        workbench.showBoard()
    }

    public func showMissionTeams() {
        closeTimelineLoadPresentation()
        workbench.showTeams()
    }

    public func showMissionAttention() {
        closeTimelineLoadPresentation()
        workbench.showAttention()
    }

    public func showMissionLibrary() {
        closeTimelineLoadPresentation()
        workbench.showLibrary()
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
            ) == nil
        else {
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
            summary:
                "This Mission is waiting for review, but no authoritative acceptance command is prepared.",
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
        let operation =
            action == "not_now" || action == "edit_scope"
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
            if lastConfirmation?.teamInstanceCreated == true {
                await refresh()
            }
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
        invalidateTimelineLoad()
        workspace.selectTask(id)
    }

    public func activateWorkspaceTask() async {
        let prefix = "team:"
        guard workspace.selectedTaskID.hasPrefix(prefix),
            let team = snapshot?.teams.first(where: {
                "team:\($0.teamInstanceID)" == workspace.selectedTaskID
            })
        else {
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
            provider.revision > 0,
            !isVerifyingMiniMax
        else {
            setupState = .unavailable(reason: "credential_unavailable")
            return
        }
        let expectedRevision = provider.revision + 1
        isVerifyingMiniMax = true
        miniMaxVerificationStatus = "Testing"
        defer { isVerifyingMiniMax = false }
        setupState = .loading
        do {
            let result = try await setupClient.verifyMiniMax(
                reference: provider.credentialReference,
                revision: provider.revision
            )
            guard result.providerID == "minimax",
                result.revision == expectedRevision,
                let terminal = miniMaxTerminalStatus(result)
            else {
                throw LocalProductClientError.invalidResponse
            }
            let refreshed = try await setupClient.setupSnapshot()
            guard refreshed.miniMax.providerID == "minimax",
                refreshed.miniMax.credentialReference == provider.credentialReference,
                refreshed.miniMax.revision == expectedRevision,
                refreshed.miniMax.status == result.status,
                refreshed.miniMax.reason == result.reason
            else {
                throw LocalProductClientError.invalidResponse
            }
            credentialStatus = result
            setupSnapshot = refreshed
            miniMaxVerificationStatus = terminal
            reconcileWorkspace()
            setupState = .ready
        } catch {
            if let remote = error as? LocalIPCRemoteError,
                remote.code == .conflict
            {
                miniMaxVerificationStatus = "Conflict"
            } else {
                miniMaxVerificationStatus = "Unavailable"
            }
            handleSetupError(error)
        }
    }

    private func miniMaxTerminalStatus(
        _ result: LocalProductCredentialSetupResult
    ) -> String? {
        switch (result.status, result.reason) {
        case ("verified", ""):
            return "Verified"
        case ("rejected", "provider_rejected"):
            return "Rejected"
        case ("rejected", "unavailable"), ("rejected", "timeout"):
            return "Unavailable"
        default:
            return nil
        }
    }

    public func replaceMiniMax(secret: String) async {
        guard let setupClient,
            let provider = setupSnapshot?.miniMax,
            !provider.credentialReference.isEmpty,
            provider.revision > 0
        else {
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
            provider.revision > 0
        else {
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
            setupState =
                remote.recoverable
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
            !CharacterSet.controlCharacters.contains(scalar)
                && !Self.workspaceBidiOverrides.contains(scalar.value)
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
            case .invalidJSON, .invalidValue, .unknownField: return "invalid_response"
            }
        }
        return "unavailable"
    }
}
