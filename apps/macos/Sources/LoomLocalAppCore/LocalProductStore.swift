import SwiftUI

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

@MainActor
public final class LocalProductStore: ObservableObject {
    @Published public private(set) var snapshot: LocalProductSnapshot?
    @Published public private(set) var timeline: LocalProductTimelinePage?
    @Published public private(set) var timelineState: LocalProductTimelineState = .idle
    @Published public private(set) var connectionState: LocalProductConnectionState = .loading
    @Published public var selectedSection: LocalProductSection = .home
    @Published public var selectedTeamID: String?

    private let client: LocalProductClientProtocol

    public init(client: LocalProductClientProtocol) {
        self.client = client
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
