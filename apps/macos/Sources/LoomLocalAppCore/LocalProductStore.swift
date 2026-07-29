import Foundation
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

public protocol LocalProductSetupClientProtocol {
    func setupSnapshot() async throws -> LocalProductSetupSnapshot
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
    @Published public var selectedSection: LocalProductSection = .home
    @Published public var selectedTeamID: String?

    private let client: LocalProductClientProtocol
    private let setupClient: LocalProductSetupClientProtocol?

    public init(client: LocalProductClientProtocol) {
        self.client = client
        setupClient = client as? LocalProductSetupClientProtocol
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

    public func refreshSetup() async {
        guard let setupClient else {
            setupState = .unavailable(reason: "setup_unavailable")
            return
        }
        setupState = .loading
        do {
            setupSnapshot = try await setupClient.setupSnapshot()
            setupState = .ready
        } catch let remote as LocalIPCRemoteError {
            setupState = remote.recoverable
                ? .unavailable(reason: remote.code.rawValue)
                : .fatal(reason: remote.code.rawValue)
        } catch {
            setupState = .unavailable(reason: closedClientReason(error))
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
            setupState = .ready
        } catch {
            handleSetupError(error)
        }
    }

    public func editBuilder(field: String, value: String) async {
        guard let setupClient, let builderSession,
              ["team_name", "purpose"].contains(field),
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

    public func cancelBuilder() {
        builderSession = nil
        lastConfirmation = nil
        setupState = .ready
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
