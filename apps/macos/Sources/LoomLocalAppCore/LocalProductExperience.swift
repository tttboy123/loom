import Foundation

public enum LocalProductExperienceState: Equatable, Sendable {
    case loading
    case connectedEmpty
    case connectedPopulated
    case partial
    case stalePreserved
    case offline
    case offlinePreserved
    case fatal
}

public enum LocalProductHomeGroup: String, CaseIterable, Sendable {
    case attention
    case workActivity
    case teams
    case systemReadiness
}

public enum LocalProductCopy {
    public static let homeTitle = "Your local agent workspace"
    public static let homeSubtitle =
        "Follow delegated work, team activity, and items that need you."
    public static let workTitle = "Work"
    public static let workSubtitle =
        "Runs and accepted evidence from the current local view."
    public static let workActivity = "Work activity"
    public static let evidence = "Accepted evidence"
    public static let teamsTitle = "Teams"
    public static let teamsSubtitle =
        "Choose a team to inspect its authoritative timeline."
    public static let inboxTitle = "Inbox"
    public static let inboxSubtitle = "Only items that need a human decision."
    public static let systemTitle = "System"
    public static let systemSubtitle =
        "Local service health and discovered runtimes."
    public static let noAttention = "Nothing needs your attention."
    public static let noWork = "No work activity is available."
    public static let noTeams = "No teams are available yet."
    public static let noEvidence = "No accepted evidence is available."
    public static let noRuntimes = "No runtimes have been discovered."
    public static let refresh = "Refresh"
    public static let selectTeam = "Select a team to view its timeline."

    public static let all = [
        homeTitle,
        homeSubtitle,
        workTitle,
        workSubtitle,
        workActivity,
        evidence,
        teamsTitle,
        teamsSubtitle,
        inboxTitle,
        inboxSubtitle,
        systemTitle,
        systemSubtitle,
        noAttention,
        noWork,
        noTeams,
        noEvidence,
        noRuntimes,
        refresh,
        selectTeam,
    ]
}

public struct LocalProductConnectionPresentation: Equatable, Sendable {
    public let title: String
    public let detail: String
    public let systemImage: String
    public let offersRefresh: Bool
}

public struct LocalProductExperience: Equatable, Sendable {
    public let state: LocalProductExperienceState
    public let homeGroups: [LocalProductHomeGroup]
    public let workActivity: [LocalProductRunSummary]
    public let acceptedEvidence: [LocalProductEvidenceSummary]
    public let attention: [LocalProductAttention]
    public let teams: [LocalProductTeamSummary]
    public let runtimes: [LocalProductRuntimeSummary]
    public let connection: LocalProductConnectionPresentation

    public init(
        snapshot: LocalProductSnapshot?,
        connectionState: LocalProductConnectionState
    ) {
        homeGroups = [
            .attention,
            .workActivity,
            .teams,
            .systemReadiness,
        ]
        workActivity = snapshot?.runs ?? []
        acceptedEvidence = snapshot?.evidence ?? []
        attention = snapshot?.attention ?? []
        teams = snapshot?.teams ?? []
        runtimes = snapshot?.runtimes ?? []

        switch connectionState {
        case .loading:
            state = .loading
            connection = .init(
                title: "Connecting to Loom",
                detail: "Checking the local service.",
                systemImage: "arrow.triangle.2.circlepath",
                offersRefresh: false
            )
        case .online:
            let hasContent = [
                workActivity.count,
                acceptedEvidence.count,
                attention.count,
                teams.count,
                runtimes.count,
            ].contains(where: { $0 > 0 })
            state = hasContent ? .connectedPopulated : .connectedEmpty
            connection = .init(
                title: "Local service connected",
                detail: "This view comes from the resident Loom service.",
                systemImage: "checkmark.circle.fill",
                offersRefresh: false
            )
        case .partial:
            state = .partial
            connection = .init(
                title: "Some information is unavailable",
                detail: "Loom is showing the information it could safely load.",
                systemImage: "exclamationmark.triangle.fill",
                offersRefresh: true
            )
        case .stale:
            state = .stalePreserved
            connection = .init(
                title: "Showing the last known view",
                detail: "Refresh when the local service is available.",
                systemImage: "clock.badge.exclamationmark",
                offersRefresh: true
            )
        case .offline:
            if snapshot == nil {
                state = .offline
                connection = .init(
                    title: "Loom is not reachable",
                    detail: "Check the local service, then refresh this view.",
                    systemImage: "bolt.slash.fill",
                    offersRefresh: true
                )
            } else {
                state = .offlinePreserved
                connection = .init(
                    title: "Loom is temporarily offline",
                    detail: "Your last loaded view is still available.",
                    systemImage: "bolt.slash.fill",
                    offersRefresh: true
                )
            }
        case .fatal:
            state = .fatal
            connection = .init(
                title: "Connection needs attention",
                detail: "Loom rejected the local connection. Check the service.",
                systemImage: "exclamationmark.octagon.fill",
                offersRefresh: true
            )
        }
    }

    public var attentionActions: [String] {
        attention.map { item in
            let action = SafeText.sanitize(item.actionRequired, limit: 160)
            return action.isEmpty ? "Review this item" : action
        }
    }

    public static func visibleName(
        _ candidate: String,
        internalID: String,
        fallback: String
    ) -> String {
        let safeCandidate = SafeText.sanitize(candidate, limit: 96)
        let safeInternalID = SafeText.sanitize(internalID, limit: 96)
        if safeCandidate.isEmpty || safeCandidate == safeInternalID {
            return fallback
        }
        return safeCandidate
    }
}
