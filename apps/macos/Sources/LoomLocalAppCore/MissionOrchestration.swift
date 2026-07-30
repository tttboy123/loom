import Foundation

public enum MissionLane:
    String, CaseIterable, Equatable, Sendable
{
    case proposed = "Proposed"
    case ready = "Ready"
    case orchestrating = "Orchestrating"
    case review = "Review"
    case complete = "Complete"
}

public enum MissionWorkbenchRoute: Equatable, Sendable {
    case board
    case mission(String)
}

public enum MissionInspectorTab:
    String, CaseIterable, Identifiable, Sendable
{
    case team = "Team"
    case plan = "Plan"
    case changes = "Changes"
    case evidence = "Evidence"

    public var id: String { rawValue }
}

public enum MissionPermissionMode:
    String, CaseIterable, Identifiable, Equatable, Sendable
{
    case planOnly = "Plan only"
    case guided = "Guided"
    case delegated = "Delegated"

    public var id: String { rawValue }
}

public struct MissionContinuity: Equatable, Sendable {
    public var composerDraft: String
    public var permissionMode: MissionPermissionMode
    public var inspector: MissionInspectorTab
    public var threadAnchor: String
    public var developerDetailsExpanded: Bool
    public var inspectorVisible: Bool

    public init(
        composerDraft: String = "",
        permissionMode: MissionPermissionMode = .guided,
        inspector: MissionInspectorTab = .team,
        threadAnchor: String = "start",
        developerDetailsExpanded: Bool = false,
        inspectorVisible: Bool = true
    ) {
        self.composerDraft = composerDraft
        self.permissionMode = permissionMode
        self.inspector = inspector
        self.threadAnchor = threadAnchor
        self.developerDetailsExpanded = developerDetailsExpanded
        self.inspectorVisible = inspectorVisible
    }
}

public struct MissionListItem: Identifiable, Equatable, Sendable {
    public let id: String
    public let title: String
    public let lane: MissionLane
    public let status: String

    public init(
        id: String,
        title: String,
        lane: MissionLane,
        status: String
    ) {
        self.id = id
        self.title = title
        self.lane = lane
        self.status = status
    }
}

public struct MissionWorkspaceState: Equatable, Sendable {
    public private(set) var route: MissionWorkbenchRoute = .board
    public private(set) var selectedMissionID: String?
    public private(set) var missions: [MissionListItem] = []
    public private(set) var boardLane: MissionLane?
    public private(set) var boardFilter = ""
    public private(set) var boardAnchor = "top"
    private var continuityByMission: [String: MissionContinuity] = [:]

    public init() {}

    public var selectedMission: MissionListItem? {
        guard let selectedMissionID else { return nil }
        return missions.first(where: { $0.id == selectedMissionID })
    }

    public var selectedContinuity: MissionContinuity {
        guard let selectedMissionID else { return MissionContinuity() }
        return continuityByMission[selectedMissionID] ?? MissionContinuity()
    }

    public mutating func mergeMissions(_ next: [MissionListItem]) {
        var seen = Set<String>()
        missions = next.filter {
            !$0.id.isEmpty && seen.insert($0.id).inserted
        }
        for mission in missions where continuityByMission[mission.id] == nil {
            continuityByMission[mission.id] = MissionContinuity()
        }
        continuityByMission = continuityByMission.filter { id, _ in
            missions.contains(where: { $0.id == id })
        }
        if let selectedMissionID,
           !missions.contains(where: { $0.id == selectedMissionID }) {
            self.selectedMissionID = nil
            route = .board
        }
    }

    public mutating func openMission(_ id: String) {
        guard missions.contains(where: { $0.id == id }) else { return }
        selectedMissionID = id
        if continuityByMission[id] == nil {
            continuityByMission[id] = MissionContinuity()
        }
        route = .mission(id)
    }

    public mutating func showBoard() {
        route = .board
    }

    public mutating func selectBoardLane(_ lane: MissionLane?) {
        boardLane = lane
    }

    public mutating func updateBoardFilter(_ value: String) {
        boardFilter = String(value.prefix(256))
    }

    public mutating func updateBoardAnchor(_ value: String) {
        boardAnchor = String(value.prefix(256))
    }

    public mutating func updateComposerDraft(_ value: String) {
        mutateSelected { $0.composerDraft = String(value.prefix(4_096)) }
    }

    public mutating func selectPermissionMode(
        _ mode: MissionPermissionMode
    ) {
        mutateSelected { $0.permissionMode = mode }
    }

    public mutating func selectInspector(_ tab: MissionInspectorTab) {
        mutateSelected { $0.inspector = tab }
    }

    public mutating func updateThreadAnchor(_ value: String) {
        mutateSelected { $0.threadAnchor = String(value.prefix(256)) }
    }

    public mutating func setDeveloperDetailsExpanded(_ expanded: Bool) {
        mutateSelected { $0.developerDetailsExpanded = expanded }
    }

    public mutating func setInspectorVisible(_ visible: Bool) {
        mutateSelected { $0.inspectorVisible = visible }
    }

    private mutating func mutateSelected(
        _ mutate: (inout MissionContinuity) -> Void
    ) {
        guard let selectedMissionID else { return }
        var continuity =
            continuityByMission[selectedMissionID] ?? MissionContinuity()
        mutate(&continuity)
        continuityByMission[selectedMissionID] = continuity
    }
}

public extension LocalProductMissionSummary {
    var listItem: MissionListItem? {
        guard let lane = MissionLane(rawValue: lane) else { return nil }
        return MissionListItem(
            id: missionID,
            title: title,
            lane: lane,
            status: status
        )
    }
}
