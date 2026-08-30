import Foundation

public extension Notification.Name {
    static let loomNewTaskRequested = Notification.Name(
        "LoomNewTaskRequested"
    )
    static let loomOpenFolderRequested = Notification.Name(
        "LoomOpenFolderRequested"
    )
}

public enum LoomGovernancePanelMode: String, Equatable, Sendable {
    case hidden
    case visible
    case pinned
}

public enum LoomGovernanceDestination: String, CaseIterable, Identifiable, Sendable {
    case overview = "Overview"
    case board = "Missions"
    case mission = "Mission"
    case roundtable = "RoundTable"
    case team = "Agent Team"
    case topology = "Team map"
    case timeline = "Activity"
    case decisions = "Decisions"
    case evidence = "Results"
    case runtimes = "Runtime & Providers"
    case attention = "Needs You"
    case library = "Library"

    public var id: String { rawValue }

    public static let missionContext: [Self] = [
        .mission, .roundtable, .timeline, .decisions, .evidence, .topology,
    ]
    public static let workspace: [Self] = [
        .board, .team, .attention, .library, .runtimes,
    ]
    public static let secondary: [Self] = [
        .overview, .topology, .timeline, .decisions, .evidence,
    ]
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

struct LoomWorkspaceRestorationSelection: Equatable, Sendable {
    let destination: LoomGovernanceDestination
    let mode: LoomGovernancePanelMode
    let missionID: String
}

func loomWorkspaceRestorationSelection(
    destinationRawValue: String,
    modeRawValue: String,
    missionID: String,
    availableMissionIDs: Set<String>
) -> LoomWorkspaceRestorationSelection? {
    guard let destination = LoomGovernanceDestination(
              rawValue: destinationRawValue
          ),
          let mode = LoomGovernancePanelMode(rawValue: modeRawValue),
          mode != .hidden else {
        return nil
    }
    let missionContext = LoomGovernanceDestination.missionContext.contains(
        destination
    )
    if missionContext {
        guard !missionID.isEmpty,
              missionID.utf8.count <= 256,
              availableMissionIDs.contains(missionID) else {
            return nil
        }
    }
    return LoomWorkspaceRestorationSelection(
        destination: destination,
        mode: mode,
        missionID: missionContext ? missionID : ""
    )
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

func loomRailConversationLimit(compact: Bool) -> Int {
    compact ? 0 : 12
}

let roundtableContextPreviewLimit = 420

private func roundtablePreviewCharacterIsWhitespace(_ character: Character) -> Bool {
    character.unicodeScalars.allSatisfy {
        CharacterSet.whitespacesAndNewlines.contains($0)
    }
}

private func roundtablePreviewSummaryEnd(
    in attributed: AttributedString,
    proposed: AttributedString.Index
) -> AttributedString.Index {
    var end = proposed
    if end > attributed.startIndex, end < attributed.endIndex {
        let previous = attributed.characters.index(before: end)
        if !roundtablePreviewCharacterIsWhitespace(attributed.characters[previous]),
           !roundtablePreviewCharacterIsWhitespace(attributed.characters[end]) {
            var candidate = end
            while candidate > attributed.startIndex {
                let before = attributed.characters.index(before: candidate)
                if roundtablePreviewCharacterIsWhitespace(attributed.characters[before]) {
                    end = before
                    break
                }
                candidate = before
            }
        }
    }
    while end > attributed.startIndex {
        let previous = attributed.characters.index(before: end)
        guard roundtablePreviewCharacterIsWhitespace(attributed.characters[previous]) else {
            break
        }
        end = previous
    }
    return end
}

private func roundtablePreviewActionStart(
    in attributed: AttributedString,
    proposed: AttributedString.Index
) -> AttributedString.Index {
    var start = proposed
    if start > attributed.startIndex, start < attributed.endIndex {
        let previous = attributed.characters.index(before: start)
        if !roundtablePreviewCharacterIsWhitespace(attributed.characters[previous]),
           !roundtablePreviewCharacterIsWhitespace(attributed.characters[start]) {
            var candidate = start
            while candidate < attributed.endIndex,
                  !roundtablePreviewCharacterIsWhitespace(attributed.characters[candidate]) {
                candidate = attributed.characters.index(after: candidate)
            }
            if candidate < attributed.endIndex {
                start = candidate
            }
        }
    }
    while start < attributed.endIndex,
          roundtablePreviewCharacterIsWhitespace(attributed.characters[start]) {
        start = attributed.characters.index(after: start)
    }
    return start
}

func roundtableContextAttributedOutput(
    _ body: String,
    expanded: Bool = false,
    limit: Int = 1_200
) -> AttributedString {
    var attributed = (try? AttributedString(
        markdown: body,
        options: .init(interpretedSyntax: .inlineOnlyPreservingWhitespace)
    )) ?? AttributedString(body)
    for run in attributed.runs where run.link != nil {
        attributed[run.range].link = nil
    }

    guard !expanded, attributed.characters.count > limit else {
        return attributed
    }

    let separator = AttributedString("\n…\n")
    let safeLimit = max(limit, 0)
    guard safeLimit > separator.characters.count else {
        let end = attributed.characters.index(
            attributed.startIndex,
            offsetBy: safeLimit
        )
        return AttributedString(attributed[attributed.startIndex..<end])
    }

    let available = safeLimit - separator.characters.count
    let summaryCount = (available * 2) / 3
    let actionCount = available - summaryCount
    let summaryEnd = roundtablePreviewSummaryEnd(
        in: attributed,
        proposed: attributed.characters.index(
            attributed.startIndex,
            offsetBy: summaryCount
        )
    )
    let actionStart = roundtablePreviewActionStart(
        in: attributed,
        proposed: attributed.characters.index(
            attributed.endIndex,
            offsetBy: -actionCount
        )
    )
    var preview = AttributedString(attributed[attributed.startIndex..<summaryEnd])
    preview.append(separator)
    preview.append(AttributedString(attributed[actionStart..<attributed.endIndex]))
    return preview
}
