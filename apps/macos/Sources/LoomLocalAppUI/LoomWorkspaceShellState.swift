import Foundation

public extension Notification.Name {
    static let loomOpenFolderRequested = Notification.Name(
        "LoomOpenFolderRequested"
    )
}

public enum LoomGovernancePanelMode: Equatable, Sendable {
    case hidden
    case visible
    case pinned
}

public enum LoomGovernanceDestination: String, CaseIterable, Identifiable, Sendable {
    case overview = "Overview"
    case board = "Board"
    case team = "Teams"
    case topology = "Topology"
    case timeline = "Timeline"
    case decisions = "Decisions"
    case evidence = "Evidence"
    case runtimes = "Runtimes"
    case attention = "Attention"
    case library = "Library"

    public var id: String { rawValue }
}

public struct LoomGovernancePanelState: Equatable, Sendable {
    public private(set) var mode: LoomGovernancePanelMode
    public private(set) var destination: LoomGovernanceDestination

    public init(
        mode: LoomGovernancePanelMode = .hidden,
        destination: LoomGovernanceDestination = .overview
    ) {
        self.mode = mode
        self.destination = destination
    }

    public mutating func open(_ destination: LoomGovernanceDestination) {
        self.destination = destination
        if mode == .hidden {
            mode = .visible
        }
    }

    public mutating func close() {
        mode = .hidden
    }

    @discardableResult
    public mutating func dismissIfPresented() -> Bool {
        guard mode != .hidden else { return false }
        close()
        return true
    }

    public mutating func togglePin() {
        switch mode {
        case .hidden, .visible:
            mode = .pinned
        case .pinned:
            mode = .visible
        }
    }
}

public enum LoomGovernancePanelPlacement: Equatable, Sendable {
    case hidden
    case split
    case overlay
}

public struct LoomWorkspaceLayoutMetrics: Equatable, Sendable {
    public let railWidth: Double
    public let conversationMinimumWidth: Double
    public let panelWidth: Double
    public let panelPlacement: LoomGovernancePanelPlacement
}

public enum LoomWorkspaceLayoutPolicy {
    public static func metrics(
        for width: Double,
        panelMode: LoomGovernancePanelMode
    ) -> LoomWorkspaceLayoutMetrics {
        let panelVisible = panelMode != .hidden
        let compact = width < 1_040 || (panelVisible && width < 1_240)
        let placement: LoomGovernancePanelPlacement
        if !panelVisible {
            placement = .hidden
        } else if width < 1_040 {
            placement = .overlay
        } else {
            placement = .split
        }
        return LoomWorkspaceLayoutMetrics(
            railWidth: compact ? 64 : 192,
            conversationMinimumWidth: compact ? 500 : 560,
            panelWidth: 360,
            panelPlacement: placement
        )
    }
}
