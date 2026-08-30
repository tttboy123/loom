import LoomLocalAppCore
import AppKit
import Foundation
import SwiftUI
import UniformTypeIdentifiers

struct ConversationComposerActionPresentation: Equatable {
    let systemImage: String
    let accessibilityLabel: String
    let help: String
    let isEnabled: Bool
}

private enum ConversationActionPickerKind: String, Identifiable {
    case route
    case model
    case reasoning

    var id: String { rawValue }
}

private struct ConversationActionPicker: Identifiable, Equatable {
    let kind: ConversationActionPickerKind
    let query: String

    var id: String { "\(kind.rawValue):\(query)" }
}

private struct ConversationActionNotice: Identifiable, Equatable {
    let id = UUID()
    let title: String
    let detail: String
    let systemImage: String
    let isWarning: Bool
}

func conversationComposerActionPresentation(
    isSending: Bool,
    isCancelling: Bool
) -> ConversationComposerActionPresentation {
    if isSending {
        return ConversationComposerActionPresentation(
            systemImage: "stop.fill",
            accessibilityLabel: "Stop response",
            help: "Stop response",
            isEnabled: !isCancelling
        )
    }
    return ConversationComposerActionPresentation(
        systemImage: "arrow.up",
        accessibilityLabel: "Send message",
        help: "Send message",
        isEnabled: true
    )
}

func conversationProfileMenuLabel(
    _ profile: LocalProductConversationProfile
) -> String {
    [
        conversationHarnessDisplayName(profile.harnessAdapter),
        conversationRouteOptionLabel(profile),
    ]
    .filter { !$0.isEmpty }
    .joined(separator: " · ")
}

func conversationRouteOptionLabel(
    _ profile: LocalProductConversationProfile
) -> String {
    if profile.harnessAdapter == "opencode",
       profile.providerID == "opencode",
       profile.providerAccountID.isEmpty {
        return "Built-in"
    }
    var parts = [profile.displayName.isEmpty ? profile.providerID : profile.displayName]
    if !profile.providerAccountID.isEmpty {
        let account = providerAccountDisplayName(
            profile.providerAccountID,
            providerID: profile.providerID
        )
        if account != "Primary" {
            parts.append(account)
        }
    }
    return parts.filter { !$0.isEmpty }.joined(separator: " · ")
}

struct ConversationRouteMenuGroup: Identifiable {
    var id: String { harnessAdapter }
    let harnessAdapter: String
    let displayName: String
    var profiles: [LocalProductConversationProfile]
}

func conversationRouteMenuGroups(
    _ profiles: [LocalProductConversationProfile]
) -> [ConversationRouteMenuGroup] {
    var groups: [ConversationRouteMenuGroup] = []
    var indexes: [String: Int] = [:]
    for profile in profiles {
        if let index = indexes[profile.harnessAdapter] {
            groups[index].profiles.append(profile)
            continue
        }
        indexes[profile.harnessAdapter] = groups.count
        groups.append(ConversationRouteMenuGroup(
            harnessAdapter: profile.harnessAdapter,
            displayName: conversationHarnessDisplayName(profile.harnessAdapter),
            profiles: [profile]
        ))
    }
    return groups
}

private func conversationHarnessDisplayName(_ adapter: String) -> String {
    switch adapter {
    case "loom-native": return "Loom Native"
    case "claude-code": return "Claude Code"
    case "codex": return "Codex"
    case "opencode": return "OpenCode"
    case "pi", "pi-cli": return "Pi"
    default: return adapter
    }
}

func conversationTrustBoundaryDimensionLabel(
    _ dimension: LocalProductConversationTrustBoundaryDimension
) -> String {
    switch dimension {
    case .trustDomain: return "Trust domain"
    case .retentionMode: return "Retention mode"
    case .dataRegion: return "Data region"
    }
}

func conversationRouteTransitionConfirmationEnabled(
    _ transition: LocalProductConversationRouteTransition,
    trustBoundaryAcknowledged: Bool
) -> Bool {
    transition.sourceExecutionBinding != nil &&
        (transition.trustBoundaryChanges.isEmpty || trustBoundaryAcknowledged)
}

func conversationContextCapacityStatusLabel(
    _ status: LocalProductContextCapacityStatus?
) -> String {
    switch status {
    case .exact: return "Exact capacity"
    case .estimated: return "Estimated capacity"
    case .unavailable: return "Capacity unavailable"
    case nil: return "Legacy policy budget"
    }
}

struct ConversationContextCapacityDetails: View {
    let disclosure: LocalProductConversationSegment

    var body: some View {
        VStack(alignment: .leading, spacing: 7) {
            metric("Shared context", "\(disclosure.disclosedContextCount) items")
            metric("Omitted context", "\(disclosure.omittedContextCount) items")

            if let status = disclosure.contextCapacityStatus {
                Divider()
                metric("Context capacity", conversationContextCapacityStatusLabel(status))
                if status == .unavailable {
                    metric("Model window", "Unavailable")
                } else {
                    metric("Model window", "\(disclosure.contextWindowTokens) tokens")
                    metric("Reserved output", "\(disclosure.reservedOutputTokens) tokens")
                    metric(
                        "Adapter / tool overhead",
                        "\(disclosure.adapterToolOverheadTokens) tokens"
                    )
                }
                metric(
                    "Admitted input budget",
                    "\(disclosure.admittedInputBudgetTokens) of \(disclosure.contextTokenBudget) policy tokens"
                )
                metric("Shared input", "\(disclosure.contextTokenCount) tokens")
                metric(
                    "Budget omissions",
                    "\(budgetOmittedItemCount) items · \(disclosure.budgetOmittedContributionTokens) tokens"
                )
                metric(
                    "Token counter",
                    "\(disclosure.contextTokenCounterID) · \(disclosure.contextTokenCounterVersion)"
                )

                if !disclosure.contextCapacityContributions.isEmpty {
                    Text("Capacity contributions")
                        .font(.caption.weight(.semibold))
                        .padding(.top, 2)
                    ForEach(disclosure.contextCapacityContributions) { contribution in
                        VStack(alignment: .leading, spacing: 2) {
                            Text(
                                "P\(contribution.priority) · \(capacitySourceLabel(contribution.sourceType))"
                            )
                            .font(.caption)
                            Text(
                                "\(contribution.admittedItemCount) admitted · \(contribution.admittedTokenCount) tokens · \(contribution.budgetOmittedItemCount) budget omitted"
                            )
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                            .fixedSize(horizontal: false, vertical: true)
                        }
                        .accessibilityElement(children: .combine)
                    }
                }
            } else if disclosure.contextTokenBudget > 0 {
                metric(
                    "Policy input budget",
                    "\(disclosure.contextTokenCount) of \(disclosure.contextTokenBudget) tokens"
                )
                Text("Model capacity was not recorded for this Segment.")
                    .font(.caption2)
                    .foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityElement(children: .contain)
        .accessibilityLabel(
            "Context capacity: \(conversationContextCapacityStatusLabel(disclosure.contextCapacityStatus))"
        )
    }

    private var budgetOmittedItemCount: Int {
        disclosure.contextCapacityContributions.reduce(0) {
            $0 + $1.budgetOmittedItemCount
        }
    }

    @ViewBuilder
    private func metric(_ label: String, _ value: String) -> some View {
        ViewThatFits(in: .horizontal) {
            HStack(alignment: .firstTextBaseline, spacing: 12) {
                Text(label).foregroundStyle(.secondary)
                Spacer(minLength: 8)
                Text(value).multilineTextAlignment(.trailing)
            }
            VStack(alignment: .leading, spacing: 2) {
                Text(label).foregroundStyle(.secondary)
                Text(value)
            }
        }
        .font(.caption2)
        .fixedSize(horizontal: false, vertical: true)
    }

    private func capacitySourceLabel(_ value: String) -> String {
        value.split(separator: "_")
            .map { $0.capitalized }
            .joined(separator: " ")
    }
}

struct ConversationRouteTransitionSheet: View {
    let transition: LocalProductConversationRouteTransition
    let onConfirm: (LocalProductConversationContextMode, Bool) -> Bool
    let onCancel: () -> Void
    let onStartNewConversation: () -> Void
    @State private var contextMode: LocalProductConversationContextMode
    @State private var trustBoundaryAcknowledged = false
    @State private var confirmationFailed = false

    init(
        transition: LocalProductConversationRouteTransition,
        initialContextMode: LocalProductConversationContextMode = .summaryOnly,
        onConfirm: @escaping (LocalProductConversationContextMode, Bool) -> Bool,
        onCancel: @escaping () -> Void,
        onStartNewConversation: @escaping () -> Void = {}
    ) {
        self.transition = transition
        self.onConfirm = onConfirm
        self.onCancel = onCancel
        self.onStartNewConversation = onStartNewConversation
        _contextMode = State(initialValue: initialContextMode)
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            VStack(alignment: .leading, spacing: 5) {
                Text(
                    transition.rebindsCurrentRoute
                        ? "Update conversation route policy"
                        : "Change conversation route"
                )
                    .font(.title2.weight(.semibold))
                Text("Review the new execution binding and context disclosure before continuing.")
                    .foregroundStyle(.secondary)
            }
            .padding(24)

            Divider()

            ScrollView {
                VStack(alignment: .leading, spacing: 20) {
                    routeRow(
                        label: transition.rebindsCurrentRoute
                            ? "Frozen route policy"
                            : "Current route",
                        profile: transition.source,
                        executionBinding: transition.sourceExecutionBinding,
                        reasoningEffort: transition.sourceReasoningEffort,
                        showsFrozenPolicy: transition.rebindsCurrentRoute
                    )

                    HStack(spacing: 8) {
                        Image(systemName: "arrow.down")
                            .foregroundStyle(.secondary)
                        Text("Create new Segment")
                            .font(.subheadline.weight(.semibold))
                    }
                    .accessibilityElement(children: .combine)

                    routeRow(
                        label: transition.rebindsCurrentRoute
                            ? "Current account policy"
                            : "New route",
                        profile: transition.target,
                        executionBinding: transition.targetExecutionBinding,
                        reasoningEffort: transition.targetReasoningEffort,
                        showsFrozenPolicy: true
                    )

                    if transition.sourceExecutionBinding == nil {
                        VStack(alignment: .leading, spacing: 10) {
                            Label(
                                "Frozen source authority is unavailable. This legacy Segment cannot be safely rebound. Start a new conversation to use this route.",
                                systemImage: "exclamationmark.shield"
                            )
                            .font(.caption)
                            .foregroundStyle(LoomGraphite.statusDanger)
                            .fixedSize(horizontal: false, vertical: true)

                            Button("Start new conversation", action: onStartNewConversation)
                        }
                    } else if !transition.trustBoundaryChanges.isEmpty {
                        VStack(alignment: .leading, spacing: 10) {
                            Label(
                                transition.trustBoundaryAuthorityUnavailable
                                    ? "Trust authority unavailable"
                                    : "Trust-boundary changes",
                                systemImage: "shield.lefthalf.filled"
                            )
                                .font(.headline)

                            if transition.trustBoundaryAuthorityUnavailable {
                                Text("One or both routes lack complete v2 policy authority. Review all trust dimensions as unavailable or unknown.")
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                                    .fixedSize(horizontal: false, vertical: true)
                            }

                            ForEach(transition.trustBoundaryChanges) { change in
                                VStack(alignment: .leading, spacing: 3) {
                                    Text(conversationTrustBoundaryDimensionLabel(change.dimension))
                                        .font(.subheadline.weight(.semibold))
                                    Text(
                                        "\(displayPolicy(change.sourceValue)) to \(displayPolicy(change.targetValue))"
                                    )
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                                }
                            }

                            Toggle(
                                "I acknowledge these trust-boundary changes",
                                isOn: $trustBoundaryAcknowledged
                            )
                            .toggleStyle(.checkbox)
                            .accessibilityHint(
                                "Required before confirming this route change"
                            )
                        }
                    }

                    Divider()

                    VStack(alignment: .leading, spacing: 10) {
                        Text("Context to share")
                            .font(.headline)
                        contextModeOption(
                            .continueWithContext,
                            title: "Continue with context",
                            detail: "Share policy-approved conversation, workspace, and artifact context."
                        )
                        contextModeOption(
                            .summaryOnly,
                            title: "Summary only",
                            detail: "Share the Goal, confirmed constraints, decisions, current state, and necessary artifacts."
                        )
                        contextModeOption(
                            .startClean,
                            title: "Start clean",
                            detail: "Share only Loom policy, Agent identity, and your next message."
                        )
                    }

                    Label(
                        "Loom applies account disclosure policy and records a Disclosure receipt. Omitted context remains visible after dispatch.",
                        systemImage: "checkmark.shield"
                    )
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)

                    Text("Provider-native session state and credentials are never reused across the new Segment.")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .fixedSize(horizontal: false, vertical: true)

                    if confirmationFailed {
                        Label(
                            "The route changed while you were reviewing. Close this sheet and choose it again.",
                            systemImage: "arrow.triangle.2.circlepath"
                        )
                        .font(.caption)
                        .foregroundStyle(LoomGraphite.statusWarning)
                        .fixedSize(horizontal: false, vertical: true)
                        .accessibilityLabel(
                            "Route changed while reviewing. Close and choose the route again."
                        )
                    }
                }
                .padding(24)
                .frame(maxWidth: .infinity, alignment: .leading)
            }

            Divider()

            HStack {
                Button("Cancel", action: onCancel)
                Spacer()
                Button(
                    transition.rebindsCurrentRoute
                        ? "Confirm new Segment"
                        : "Confirm route change"
                ) {
                    confirmationFailed = !onConfirm(
                        contextMode,
                        trustBoundaryAcknowledged
                    )
                }
                .buttonStyle(.borderedProminent)
                .disabled(!conversationRouteTransitionConfirmationEnabled(
                    transition,
                    trustBoundaryAcknowledged: trustBoundaryAcknowledged
                ))
            }
            .padding(.horizontal, 24)
            .padding(.vertical, 16)
        }
    }

    private func routeRow(
        label: String,
        profile: LocalProductConversationProfile,
        executionBinding: LocalProductConversationExecutionBinding? = nil,
        reasoningEffort: String = "",
        showsFrozenPolicy: Bool = false
    ) -> some View {
        VStack(alignment: .leading, spacing: 5) {
            Text(label)
                .font(.caption.weight(.semibold))
                .foregroundStyle(.secondary)
            Text(profile.displayName)
                .font(.headline)
            Text(bindingLabel(
                profile,
                executionBinding: executionBinding,
                reasoningEffort: reasoningEffort
            ))
                .font(.caption)
                .foregroundStyle(.secondary)
                .textSelection(.enabled)
            if showsFrozenPolicy {
                Text(
                    executionBinding.map(policyLabel)
                        ?? "Account policy unspecified (legacy Segment)"
                )
                    .font(.caption2)
                    .foregroundStyle(.secondary)
            } else if profile.authMode == "brokered" {
                Text(policyLabel(profile))
                    .font(.caption2)
                    .foregroundStyle(.secondary)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private func contextModeOption(
        _ mode: LocalProductConversationContextMode,
        title: String,
        detail: String
    ) -> some View {
        Button {
            contextMode = mode
            confirmationFailed = false
        } label: {
            HStack(alignment: .top, spacing: 10) {
                Image(
                    systemName: contextMode == mode
                        ? "largecircle.fill.circle"
                        : "circle"
                )
                .foregroundStyle(contextMode == mode ? LoomGraphite.accent : Color.secondary)
                VStack(alignment: .leading, spacing: 3) {
                    Text(title)
                        .foregroundStyle(.primary)
                    Text(detail)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .fixedSize(horizontal: false, vertical: true)
                }
                Spacer(minLength: 0)
            }
            .frame(minHeight: LoomGraphite.minimumActionTarget)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityLabel("\(title). \(detail)")
    }

    private func bindingLabel(
        _ profile: LocalProductConversationProfile,
        executionBinding: LocalProductConversationExecutionBinding? = nil,
        reasoningEffort: String = ""
    ) -> String {
        let providerID = executionBinding?.providerID ?? profile.providerID
        let providerAccountID = executionBinding?.providerAccountID
            ?? profile.providerAccountID
        let account = providerAccountID.isEmpty ? providerID : providerAccountID
        let modelID = executionBinding?.modelID ?? profile.modelID
        let harness = executionBinding?.harnessAdapter ?? profile.harnessAdapter
        let revision = executionBinding?.credentialRevision
            ?? profile.credentialRevision
        var parts = [harnessLabel(harness), account, modelID]
            .filter { !$0.isEmpty }
        if !reasoningEffort.isEmpty {
            parts.append("Reasoning \(reasoningEffort)")
        }
        if revision > 0 {
            parts.append("Credential r\(revision)")
        }
        return parts
            .filter { !$0.isEmpty }
            .joined(separator: " · ")
    }

    private func policyLabel(
        _ binding: LocalProductConversationExecutionBinding
    ) -> String {
        guard binding.providerAccountPolicyVersion == 2 else {
            return "Account policy unspecified"
        }
        return "Policy r\(binding.providerAccountPolicyRevision) · \(displayPolicy(binding.trustDomain)) · \(displayPolicy(binding.retentionMode)) · \(displayPolicy(binding.dataRegion))"
    }

    private func policyLabel(_ profile: LocalProductConversationProfile) -> String {
        guard profile.policyVersion == 2 else {
            return "Account policy unspecified"
        }
        return "Policy r\(profile.policyRevision) · \(displayPolicy(profile.trustDomain)) · \(displayPolicy(profile.retentionMode)) · \(displayPolicy(profile.dataRegion))"
    }

    private func displayPolicy(_ value: String) -> String {
        guard !value.isEmpty else { return "Not specified" }
        return value.split(separator: "_").map { $0.capitalized }.joined(separator: " ")
    }

    private func harnessLabel(_ harnessAdapter: String) -> String {
        switch harnessAdapter {
        case "codex": return "Codex"
        case "claude-code": return "Claude Code"
        case "loom-native": return "Loom Native"
        case "pi": return "Pi"
        default: return harnessAdapter
        }
    }
}

public enum LoomWorkspaceNavigationItem: String, CaseIterable, Identifiable, Sendable {
    case home = "Chat"
    case work = "Missions"
    case teams = "Agent Teams"
    case roundtable = "RoundTable"
    case attention = "Needs You"
    case library = "Library"
    case runtimes = "Runtime & Providers"

    public var id: String { rawValue }

    public static let primary: [Self] = [.home, .work, .attention]
    public static let governance: [Self] = [.teams, .roundtable]
    public static let tools: [Self] = [.library]

    public var symbol: String {
        switch self {
        case .home: return "bubble.left.fill"
        case .work: return "square.3.layers.3d"
        case .teams: return "person.3"
        case .roundtable: return "person.2.wave.2"
        case .attention: return "exclamationmark.triangle"
        case .library: return "books.vertical"
        case .runtimes: return "cpu"
        }
    }

    public var accessibilityLabel: String {
        switch self {
        case .home: return "Conversation"
        case .work: return "Missions"
        case .teams: return "Agent Teams"
        case .roundtable: return "Mission RoundTable"
        case .attention: return "Items needing attention"
        case .library: return "Agent library"
        case .runtimes: return "Runtime health"
        }
    }
}

private enum LoomFullGovernancePresentation: String, Identifiable {
    case workbench
    case newMission
    case runtimeProviders
    case roundtable

    var id: String { rawValue }
}

private struct ChatDiagnosticPresentation: Identifiable {
    let id = UUID()
    let preview: LocalDiagnosticBundlePreview
    let exporter: LocalDiagnosticBundleExporter
}

func conversationVaultRecoveryAvailable(
    stage: LocalIPCRemoteError.Stage,
    recoverable: Bool
) -> Bool {
    // Vault-stage failures always offer vault recovery, even when retryable:
    // retrying without unlocking keeps failing with the same locked state.
    _ = recoverable
    return [
        .vaultKeyLoad,
        .vaultOpen,
        .vaultEncrypt,
        .vaultCommit,
        .vaultDecrypt,
        .vaultAADValidation,
        .vaultRotation,
        .vaultRecovery,
    ].contains(stage)
}

func conversationCompletedToolDisplayName(_ toolID: String) -> String {
    switch toolID {
    case "loom.sessions.search": return "Find conversations"
    case "loom.sessions.align.preview": return "Align conversations"
    case "loom.missions.create.preview": return "Create Mission"
    case "loom.missions.continue.preview": return "Continue Mission"
    case "loom.teams.create.preview": return "Create Agent Team"
    case "loom.roundtables.open.preview": return "Open RoundTable"
    case "loom.missions.search": return "Find Missions"
    case "loom.missions.status": return "Mission status"
    case "loom.teams.search": return "Find Agent Teams"
    case "loom.teams.status": return "Agent Team status"
    case "loom.roundtables.status": return "RoundTable status"
    case "loom.governance.needs_you": return "Needs You"
    case "loom.runtimes.status": return "Runtime status"
    case "loom.providers.status": return "Provider status"
    case "loom.diagnostics.incident": return "Incident diagnostics"
    case "loom.workspace.status": return "Workspace status"
    case "loom.conversation.route.status": return "Conversation route"
    case "loom.library.search": return "Search library"
    case "loom.conversation.route.change.preview": return "Change route"
    case "loom.conversation.model.change.preview": return "Change model"
    case "loom.conversation.reasoning.change.preview": return "Change reasoning"
    case "loom.workspace.choose.preview": return "Choose workspace"
    case "loom.teams.edit.preview": return "Edit Agent Team"
    case "loom.roundtables.pause.preview": return "Pause RoundTable"
    case "loom.roundtables.steer.preview": return "Guide RoundTable"
    case "loom.roundtables.retry.preview": return "Retry RoundTable seat"
    case "loom.roundtables.skip.preview": return "Skip RoundTable seat"
    case "loom.roundtables.replace.preview": return "Replace RoundTable seat"
    default: return "Loom tool"
    }
}

func conversationCompletedToolActivityText(_ toolIDs: [String]) -> String? {
    let names = toolIDs.prefix(8).map(conversationCompletedToolDisplayName)
    guard !names.isEmpty else { return nil }
    return "Used Loom tools: \(names.joined(separator: ", "))"
}

func conversationAttemptForMessage(
    _ message: LocalProductChatMessage,
    in thread: LocalProductChatThread
) -> LocalProductConversationAttempt? {
    guard message.role == "loom", message.tentative else { return nil }
    if !message.attemptID.isEmpty {
        return thread.attempts.first {
            $0.attemptID == message.attemptID && $0.segmentID == message.segmentID
        }
    }
    var userTurnsBefore = 0
    var foundSelf = false
    for candidate in thread.messages where candidate.segmentID == message.segmentID {
        if candidate.messageID == message.messageID {
            foundSelf = true
            break
        }
        if candidate.role == "user" {
            userTurnsBefore += 1
        }
    }
    let segmentAttempts = thread.attempts.filter {
        $0.segmentID == message.segmentID
    }
    guard foundSelf, userTurnsBefore > 0, userTurnsBefore <= segmentAttempts.count else {
        return nil
    }
    return segmentAttempts[userTurnsBefore - 1]
}

struct ConversationCompletedToolActivity: View {
    let attemptID: String
    let toolIDs: [String]

    var body: some View {
        if let activityText = conversationCompletedToolActivityText(toolIDs) {
            HStack(alignment: .firstTextBaseline, spacing: 5) {
                Image(systemName: "wrench.and.screwdriver")
                    .font(.caption2)
                    .foregroundStyle(.tertiary)
                    .frame(width: 14, alignment: .leading)
                    .accessibilityHidden(true)
                Text(activityText)
                    .font(.caption2)
                    .foregroundStyle(.secondary)
                    .multilineTextAlignment(.leading)
                    .lineLimit(nil)
                    .fixedSize(horizontal: false, vertical: true)
                    .layoutPriority(1)
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .accessibilityElement(children: .ignore)
            .accessibilityIdentifier(
                "loom.conversation.tool-activity.\(attemptID)"
            )
            .accessibilityLabel(activityText)
        }
    }
}

public enum LoomRecentTaskActionLabel {
    public static func make(
        for title: String,
        subtitle: String,
        position: Int
    ) -> String {
        let safeTitle = SafeText.sanitize(title, limit: 48)
            .trimmingCharacters(in: .whitespacesAndNewlines)
        let safeSubtitle = SafeText.sanitize(subtitle, limit: 32)
            .trimmingCharacters(in: .whitespacesAndNewlines)
        return (["Open recent task \(max(position, 1))", safeTitle, safeSubtitle]
            .filter { !$0.isEmpty })
            .joined(separator: ", ")
    }
}

@ViewBuilder
private func proposalDecisionIncidentRow(_ incidentID: String) -> some View {
    HStack(spacing: 6) {
        Text("Incident \(incidentID)")
            .font(.caption2.monospaced())
            .foregroundStyle(.secondary)
            .lineLimit(1)
            .truncationMode(.middle)
        Spacer(minLength: 4)
        Button {
            NSPasteboard.general.clearContents()
            NSPasteboard.general.setString(incidentID, forType: .string)
        } label: {
            Image(systemName: "doc.on.doc")
        }
        .buttonStyle(.plain)
        .help("Copy incident ID")
        .accessibilityLabel("Copy incident ID")
    }
}

func conversationProposalRecoveryMessage(
    status: LocalProductConversationControlProposalStatus,
    hasDecisionReceipt: Bool
) -> String? {
    switch status {
    case .cancelled:
        return "Nothing changed. Ask Loom to prepare a fresh proposal when you want to try again."
    case .expired:
        return "This proposal expired for safety. Nothing changed; ask Loom to prepare a fresh one."
    case .confirmed where !hasDecisionReceipt:
        return "This historical approval is read-only. Ask Loom to prepare it again."
    case .pending, .confirmed:
        return nil
    }
}

struct ConversationControlProposalCard: View {
    let proposal: LocalProductConversationControlProposal
    let alignment: LocalProductConversationContextAlignment?
    let inFlight: Bool
    var confirmable = true
    var decisionReceipt: LocalProductConversationProposalDecisionReceipt? = nil
    let onDecision: (LocalProductChatControlDecision) -> Void

    var body: some View {
        HStack(alignment: .top, spacing: 10) {
            Image(
                systemName: proposal.status == .confirmed
                    ? "checkmark.circle.fill"
                    : "arrow.triangle.merge"
            )
            .foregroundStyle(
                proposal.status == .confirmed
                    ? LoomGraphite.statusSuccess
                    : LoomGraphite.accent
            )
            .frame(width: 22)
            .accessibilityHidden(true)

            VStack(alignment: .leading, spacing: 10) {
                HStack(spacing: 8) {
                    Text("Align conversations")
                        .font(.callout.weight(.semibold))
                    Spacer(minLength: 12)
                    Text(statusText)
                        .font(.caption.weight(.medium))
                        .foregroundStyle(.secondary)
                }

                VStack(alignment: .leading, spacing: 5) {
                    ForEach(proposal.sources, id: \.conversationID) { source in
                        Label(source.title, systemImage: "bubble.left")
                            .font(.callout)
                            .lineLimit(2)
                    }
                }

                Label(
                    proposal.contextMode == .summaryOnly
                        ? "Goals, decisions, and user constraints only"
                        : "Approved conversation context",
                    systemImage: proposal.contextMode == .summaryOnly
                        ? "text.quote"
                        : "text.append"
                )
                .font(.caption)
                .foregroundStyle(.secondary)

                if let decisionReceipt {
                    proposalDecisionIncidentRow(decisionReceipt.decisionIncidentID)
                }

                if let recoveryMessage = conversationProposalRecoveryMessage(
                    status: proposal.status,
                    hasDecisionReceipt: decisionReceipt != nil
                ) {
                    Text(recoveryMessage)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .fixedSize(horizontal: false, vertical: true)
                }

                if proposal.status == .pending && !confirmable {
                    Text("Cancel this outdated alignment and ask Loom to prepare it again.")
                        .font(.caption)
                        .foregroundStyle(LoomGraphite.statusWarning)
                        .fixedSize(horizontal: false, vertical: true)
                }

                if proposal.status == .pending {
                    Text("Nothing changes until you confirm. Your next message starts the aligned segment.")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .fixedSize(horizontal: false, vertical: true)

                    HStack(spacing: 8) {
                        Button {
                            onDecision(.confirm)
                        } label: {
                            Label("Confirm", systemImage: "checkmark")
                        }
                        .buttonStyle(.borderedProminent)
                        .controlSize(.small)
                        .disabled(inFlight || !confirmable)
                        .help(
                            confirmable
                                ? "Confirm alignment"
                                : "This alignment no longer matches the current conversation. Cancel it and try again."
                        )
                        .accessibilityIdentifier(
                            "loom.conversation.control-proposal.confirm.\(proposal.proposalID)"
                        )

                        Button {
                            onDecision(.cancel)
                        } label: {
                            Label("Cancel", systemImage: "xmark")
                        }
                        .buttonStyle(.bordered)
                        .controlSize(.small)
                        .disabled(inFlight)
                        .accessibilityIdentifier(
                            "loom.conversation.control-proposal.cancel.\(proposal.proposalID)"
                        )

                        if inFlight {
                            ProgressView()
                                .controlSize(.small)
                                .accessibilityLabel("Saving context decision")
                        }
                    }
                }
            }
        }
        .padding(12)
        .background(Color.secondary.opacity(0.06))
        .overlay(
            RoundedRectangle(cornerRadius: 6)
                .stroke(Color.secondary.opacity(0.16), lineWidth: 1)
        )
        .clipShape(RoundedRectangle(cornerRadius: 6))
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier(
            "loom.conversation.control-proposal.\(proposal.proposalID)"
        )
    }

    private var statusText: String {
        switch proposal.status {
        case .pending:
            return confirmable ? "Needs confirmation" : "Update required"
        case .confirmed:
            return alignment?.appliedSegmentID.isEmpty == false ? "Applied" : "Approved"
        case .cancelled:
            return "Cancelled"
        case .expired:
            return "Expired"
        }
    }
}

struct ConversationActionProposalCard: View {
    let proposal: LocalProductConversationActionProposal
    let inFlight: Bool
    var targetDisplayName = ""
    var decisionReceipt: LocalProductConversationProposalDecisionReceipt? = nil
    let onDecision: (LocalProductChatControlDecision) -> Void

    var body: some View {
        HStack(alignment: .top, spacing: 10) {
            Image(systemName: proposal.status == .confirmed ? "checkmark.circle.fill" : icon)
                .foregroundStyle(
                    proposal.status == .confirmed
                        ? LoomGraphite.statusSuccess
                        : LoomGraphite.accent
                )
                .frame(width: 22)
                .accessibilityHidden(true)

            VStack(alignment: .leading, spacing: 9) {
                HStack(spacing: 8) {
                    Text(title)
                        .font(.callout.weight(.semibold))
                    Spacer(minLength: 12)
                    Text(statusText)
                        .font(.caption.weight(.medium))
                        .foregroundStyle(.secondary)
                }

                if !proposal.argument.isEmpty {
                    Text(proposal.argument)
                        .font(.callout)
                        .lineLimit(6)
                        .fixedSize(horizontal: false, vertical: true)
                }
                if !displayTargetSummary.isEmpty {
                    Text(displayTargetSummary)
                        .font(proposal.argument.isEmpty ? .callout : .caption)
                        .foregroundStyle(proposal.argument.isEmpty ? .primary : .secondary)
                        .lineLimit(6)
                        .fixedSize(horizontal: false, vertical: true)
                }

                Text(explanation)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)

                if let decisionReceipt {
                    proposalDecisionIncidentRow(decisionReceipt.decisionIncidentID)
                }

                if let recoveryMessage = conversationProposalRecoveryMessage(
                    status: proposal.status,
                    hasDecisionReceipt: decisionReceipt != nil
                ) {
                    Text(recoveryMessage)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .fixedSize(horizontal: false, vertical: true)
                }

                if proposal.status == .pending && !proposal.isConfirmable {
                    Text("Cancel this outdated proposal and ask Loom to prepare it again.")
                        .font(.caption)
                        .foregroundStyle(LoomGraphite.statusWarning)
                        .fixedSize(horizontal: false, vertical: true)
                }

                if proposal.status == .pending {
                    HStack(spacing: 8) {
                        Button {
                            onDecision(.confirm)
                        } label: {
                            Label(primaryActionTitle, systemImage: primaryActionIcon)
                        }
                        .buttonStyle(.borderedProminent)
                        .controlSize(.small)
                        .disabled(inFlight || !proposal.isConfirmable)
                        .help(
                            proposal.isConfirmable
                                ? primaryActionTitle
                                : "This proposal predates the current RoundTable binding. Cancel it and try again."
                        )
                        .accessibilityIdentifier(
                            "loom.conversation.action-proposal.confirm.\(proposal.proposalID)"
                        )

                        Button {
                            onDecision(.cancel)
                        } label: {
                            Label("Cancel", systemImage: "xmark")
                        }
                        .buttonStyle(.bordered)
                        .controlSize(.small)
                        .disabled(inFlight)
                        .accessibilityIdentifier(
                            "loom.conversation.action-proposal.cancel.\(proposal.proposalID)"
                        )

                        if inFlight {
                            ProgressView()
                                .controlSize(.small)
                                .accessibilityLabel("Saving Loom action decision")
                        }
                    }
                } else if proposal.status == .confirmed, decisionReceipt != nil {
                    Button {
                        onDecision(.confirm)
                    } label: {
                        Label(confirmedActionTitle, systemImage: "arrow.up.right")
                    }
                    .buttonStyle(.bordered)
                    .controlSize(.small)
                    .disabled(inFlight)
                    .accessibilityIdentifier(
                        "loom.conversation.action-proposal.resume.\(proposal.proposalID)"
                    )
                }
            }
        }
        .padding(12)
        .background(Color.secondary.opacity(0.06))
        .overlay(
            RoundedRectangle(cornerRadius: 6)
                .stroke(Color.secondary.opacity(0.16), lineWidth: 1)
        )
        .clipShape(RoundedRectangle(cornerRadius: 6))
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier(
            "loom.conversation.action-proposal.\(proposal.proposalID)"
        )
    }

    private var title: String {
        switch proposal.action {
        case .mission:
            return "Create Mission"
        case .continueMission:
            return "Continue Mission"
        case .team:
            return "Create Agent Team"
        case .roundTable:
            return "Open RoundTable"
        case .route:
            return "Change Route"
        case .model:
            return "Change Model"
        case .reasoning:
            return "Change Reasoning"
        case .workspace:
            return "Choose Workspace"
        case .teamEdit:
            return "Edit Agent Team"
        case .roundTablePause:
            return "Pause RoundTable"
        case .roundTableSteer:
            return "Steer Agent"
        case .roundTableRetry:
            return "Retry Agent"
        case .roundTableSkip:
            return "Skip Agent"
        case .roundTableReplace:
            return "Replace Agent"
        }
    }

    private var displayTargetSummary: String {
        if proposal.action == .continueMission || proposal.action == .roundTable,
           !targetDisplayName.isEmpty {
            return "Mission: \(targetDisplayName)"
        }
        return proposal.targetSummary
    }

    private var icon: String {
        switch proposal.action {
        case .mission:
            return "scope"
        case .continueMission:
            return "arrow.clockwise"
        case .team:
            return "person.3"
        case .roundTable:
            return "bubble.left.and.bubble.right"
        case .route:
            return "point.3.connected.trianglepath.dotted"
        case .model:
            return "cpu"
        case .reasoning:
            return "brain.head.profile"
        case .workspace:
            return "folder"
        case .teamEdit:
            return "person.3.sequence"
        case .roundTablePause:
            return "pause.circle"
        case .roundTableSteer:
            return "arrow.turn.up.right"
        case .roundTableRetry:
            return "arrow.clockwise"
        case .roundTableSkip:
            return "forward.end"
        case .roundTableReplace:
            return "person.crop.circle.badge.arrow.trianglehead.counterclockwise"
        }
    }

    private var explanation: String {
        switch proposal.action {
        case .mission:
            return "Review the workflow and Agent Team before anything runs."
        case .continueMission:
            return "Review this guidance before Loom creates a new audited Attempt."
        case .team:
            return "Review every role, Runtime, Provider Account and model before creating the Team."
        case .roundTable:
            return "Review the Mission-linked seats before deliberation starts."
        case .route:
            return "Apply this exact Route to new turns. A trust-domain change still requires disclosure review."
        case .model:
            return "Apply this exact model on the selected Route to new turns."
        case .reasoning:
            return "Apply this reasoning effort to new turns without changing earlier Segments."
        case .workspace:
            return "Open the system folder picker. The model cannot see or choose a local path."
        case .teamEdit:
            return "Open the existing Team as a review draft. No role or binding changes automatically."
        case .roundTablePause:
            return "Pause the exact active round after Loom revalidates its current state."
        case .roundTableSteer:
            return "Send this bounded guidance to the exact running Agent Attempt."
        case .roundTableRetry:
            return "Create a new audited Attempt with this bounded retry guidance."
        case .roundTableSkip:
            return "Skip only this Agent seat in the exact active round."
        case .roundTableReplace:
            return "Open replacement review for this seat. You still choose the replacement Agent and Route."
        }
    }

    private var primaryActionTitle: String {
        switch proposal.action {
        case .roundTable:
            return "Open setup"
        case .workspace:
            return "Choose folder"
        case .teamEdit:
            return "Open draft"
        case .roundTableReplace:
            return "Choose replacement"
        case .route, .model, .reasoning, .roundTablePause,
                .roundTableSteer, .roundTableRetry, .roundTableSkip:
            return "Apply"
        case .mission, .continueMission, .team:
            return "Review"
        }
    }

    private var primaryActionIcon: String {
        switch proposal.action {
        case .roundTable, .teamEdit, .roundTableReplace:
            return "arrow.up.right"
        case .workspace:
            return "folder"
        default:
            return "checkmark"
        }
    }

    private var confirmedActionTitle: String {
        switch proposal.action {
        case .roundTablePause, .roundTableSteer, .roundTableRetry,
                .roundTableSkip:
            return "Restore result"
        case .route, .model, .reasoning:
            return "Apply again"
        default:
            return "Open again"
        }
    }

    private var statusText: String {
        switch proposal.status {
        case .pending:
            return proposal.isConfirmable ? "Needs confirmation" : "Update required"
        case .confirmed:
            return "Approved"
        case .cancelled:
            return "Cancelled"
        case .expired:
            return "Expired"
        }
    }
}

private extension LocalProductConversationActionProposal {
    var targetSummary: String {
        guard let payload else { return "" }
        switch action {
        case .route:
            return "Route: \(payload.profileID)"
        case .model:
            return "Model: \(payload.modelID)"
        case .reasoning:
            return "Reasoning: \(payload.reasoningEffort)"
        case .teamEdit:
            return "Team: \(payload.teamInstanceID)\n\(payload.instruction)"
        case .continueMission, .roundTable:
            return "Mission: \(payload.missionID)"
        case .roundTablePause:
            return "Session \(payload.sessionID) · Round \(payload.roundID)"
        case .roundTableSteer, .roundTableRetry:
            return "Seat \(payload.seatID) · Attempt \(payload.attemptID)\n\(payload.guidance)"
        case .roundTableSkip, .roundTableReplace:
            return "Session \(payload.sessionID) · Round \(payload.roundID) · Seat \(payload.seatID)"
        case .mission, .team, .workspace:
            return ""
        }
    }

    var conversationActionRequest: ConversationActionRequest? {
        let actionID: ConversationActionID
        switch action {
        case .mission:
            actionID = .mission
        case .team:
            actionID = .team
        case .continueMission, .roundTable, .route, .model, .reasoning,
                .workspace, .teamEdit,
                .roundTablePause, .roundTableSteer, .roundTableRetry,
                .roundTableSkip, .roundTableReplace:
            return nil
        }
        return ConversationActionRequest(
            action: actionID,
            argument: argument,
            source: .modelTool
        )
    }
}

func conversationChatStageLabel(_ stage: LocalIPCRemoteError.Stage) -> String {
    switch stage {
    case .inputAdmission: return "Input admission"
    case .udsTransport: return "Local service transport"
    case .conversationDispatch: return "Conversation dispatch"
    default:
        return stage.rawValue.replacingOccurrences(of: "_", with: " ").capitalized
    }
}

struct ConversationChatFailureBanner: View {
    let failure: LocalProductChatOperationFailure
    let vaultRecoveryAvailable: Bool
    let routeRecoveryAvailable: Bool
    let isSending: Bool
    let isUpdatingVault: Bool
    let onPrimaryRecovery: () -> Void
    let onUnlockVault: () -> Void
    let onOpenVault: () -> Void
    let onSwitchProvider: () -> Void
    let onDiagnostics: () -> Void
    let onCopyIncident: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            ViewThatFits(in: .horizontal) {
                HStack(alignment: .firstTextBaseline, spacing: 8) {
                    failureIcon
                    Text(failure.title)
                        .font(.callout.weight(.semibold))
                    Spacer(minLength: 8)
                    stageLabel
                }
                VStack(alignment: .leading, spacing: 4) {
                    HStack(alignment: .firstTextBaseline, spacing: 8) {
                        failureIcon
                        Text(failure.title)
                            .font(.callout.weight(.semibold))
                    }
                    stageLabel
                }
            }
            Text(failure.detail)
                .font(.caption)
                .foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)
            Text("Incident \(failure.incidentID)")
                .font(.caption2.monospaced())
                .foregroundStyle(.secondary)
                .textSelection(.enabled)
            ViewThatFits(in: .horizontal) {
                HStack(spacing: 8) {
                    recoveryActions
                    diagnosticsButton
                    Spacer(minLength: 8)
                    copyIconButton
                }
                VStack(alignment: .leading, spacing: 8) {
                    recoveryActions
                    HStack(spacing: 8) {
                        diagnosticsButton
                        copyLabeledButton
                    }
                }
            }
            .controlSize(.small)
        }
        .padding(10)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(LoomGraphite.statusDanger.opacity(0.08))
        .overlay(alignment: .leading) {
            Rectangle()
                .fill(LoomGraphite.statusDanger)
                .frame(width: 2)
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel(
            "\(failure.title). \(failure.detail). Incident \(failure.incidentID)"
        )
    }

    private var failureIcon: some View {
        Image(systemName: "exclamationmark.triangle.fill")
            .foregroundStyle(LoomGraphite.statusDanger)
            .accessibilityHidden(true)
    }

    private var stageLabel: some View {
        Text(conversationChatStageLabel(failure.stage))
            .font(.caption)
            .foregroundStyle(.secondary)
    }

    @ViewBuilder
    private var recoveryActions: some View {
        if failure.isProposalDecisionRecoveryAvailable {
            Button(action: onPrimaryRecovery) {
                Label("Refresh proposal", systemImage: "arrow.clockwise")
            }
            .disabled(isSending)
        } else if failure.recoverable {
            Button(action: onPrimaryRecovery) {
                Label("Retry", systemImage: "arrow.clockwise")
            }
            .disabled(isSending)
        }
        if vaultRecoveryAvailable {
            Button(action: onUnlockVault) {
                Label("Unlock Vault", systemImage: "lock.open")
            }
            .disabled(isUpdatingVault)
            Button(action: onOpenVault) {
                Label("Open Credential Vault", systemImage: "key")
            }
        }
        if routeRecoveryAvailable {
            Button(action: onSwitchProvider) {
                Label("Switch Provider", systemImage: "arrow.triangle.swap")
            }
        }
    }

    private var diagnosticsButton: some View {
        Button(action: onDiagnostics) {
            Label("View diagnostics", systemImage: "doc.text.magnifyingglass")
        }
    }

    private var copyIconButton: some View {
        Button(action: onCopyIncident) {
            Image(systemName: "doc.on.doc")
        }
        .buttonStyle(.plain)
        .accessibilityLabel("Copy incident ID")
        .help("Copy incident ID")
    }

    private var copyLabeledButton: some View {
        Button(action: onCopyIncident) {
            Label("Copy incident ID", systemImage: "doc.on.doc")
        }
    }
}

public struct LoomWorkspaceShell: View {
    private static let chatTimelineBottomID = "loom-chat-timeline-bottom"

    @ObservedObject private var store: LocalProductStore
    @AppStorage("loom.workspace.governance.destination")
    private var storedGovernanceDestination = ""
    @AppStorage("loom.workspace.governance.mode")
    private var storedGovernanceMode = ""
    @AppStorage("loom.workspace.governance.mission")
    private var storedGovernanceMissionID = ""
    private let restoresSceneState: Bool
    @State private var selectedNavigation: LoomWorkspaceNavigationItem = .home
    @State private var governance = LoomGovernancePanelState()
    @State private var didRestoreSceneState = false
    @State private var fullGovernancePresentation: LoomFullGovernancePresentation?
    @State private var showFolderImporter = false
    @State private var folderSelectionError: String?
    @State private var builderAnswer = ""
    @State private var builderName = ""
    @State private var builderPurpose = ""
    @State private var modelTeamEditInstruction = ""
    @State private var pendingMissionObjective = ""
    @State private var pendingMissionTeamID = ""
    @State private var pendingRoundtableMissionLink: LocalRoundtableMissionLink?
    @State private var roundtableContextView: LocalRoundtableView?
    @State private var roundtableContextBusy = false
    @State private var roundtableContextError: String?
    @State private var roundtableContextDrafts: [String: String] = [:]
    @State private var roundtableSynthesisDraft = roundtableDefaultSynthesisPrompt
    @State private var expandedMissionActivityIDs = Set<String>()
    @State private var expandedRoundtableAttemptIDs = Set<String>()
    @State private var diagnosticPresentation: ChatDiagnosticPresentation?
    @State private var diagnosticPreparationError: String?
    @State private var pendingConversationRouteTransition:
        LocalProductConversationRouteTransition?
    @State private var conversationActionMenu = ConversationActionMenuState()
    @State private var conversationActionMenuForced = false
    @State private var conversationActionMenuSuppressed = false
    @State private var conversationActionPicker: ConversationActionPicker?
    @State private var conversationActionChoiceMenu = ConversationActionChoiceMenuState()
    @State private var conversationActionNotice: ConversationActionNotice?
    @State private var pendingStopAction: ConversationActionRequest?
    @FocusState private var composerFocused: Bool
    @State private var chatSelectMode = false

    public init(
        store: LocalProductStore,
        initialGovernanceDestination: LoomGovernanceDestination? = nil
    ) {
        self.store = store
        restoresSceneState = initialGovernanceDestination == nil
        guard let initialGovernanceDestination else { return }
        _governance = State(
            initialValue: LoomGovernancePanelState(
                mode: .visible,
                destination: initialGovernanceDestination
            )
        )
        _selectedNavigation = State(
            initialValue: Self.navigationItem(for: initialGovernanceDestination)
        )
    }

    public var body: some View {
        GeometryReader { proxy in
            let metrics = LoomWorkspaceLayoutPolicy.metrics(
                for: proxy.size.width,
                panelMode: governance.mode
            )

            ZStack(alignment: .trailing) {
                HSplitView {
                    navigationRail(compact: metrics.railWidth == 64)
                        .frame(
                            minWidth: metrics.railWidth,
                            idealWidth: metrics.railWidth,
                            maxWidth: metrics.railWidth
                        )

                    centerWorkspace
                        .frame(minWidth: metrics.conversationMinimumWidth)

                    if metrics.panelPlacement == .split {
                        governanceInspector
                            .frame(
                                minWidth: 320,
                                idealWidth: metrics.panelWidth,
                                maxWidth: 420
                            )
                    }
                }

                if metrics.panelPlacement == .overlay {
                    Color.black.opacity(0.16)
                        .contentShape(Rectangle())
                        .onTapGesture { governance.close() }
                        .accessibilityHidden(true)

                    governanceInspector
                        .frame(width: metrics.panelWidth)
                        .background(LoomGraphite.surface)
                        .shadow(color: .black.opacity(0.22), radius: 18, x: -4)
                        .transition(.move(edge: .trailing).combined(with: .opacity))
                }
            }
            .animation(.easeOut(duration: 0.18), value: governance.mode)
        }
        .background(LoomGraphite.canvas)
        .fileImporter(
            isPresented: $showFolderImporter,
            allowedContentTypes: [.folder],
            allowsMultipleSelection: false,
            onCompletion: handleFolderSelection
        )
        .sheet(item: $fullGovernancePresentation) { presentation in
            if presentation == .roundtable {
                RoundtableWorkbench(
                    store: store,
                    missionLink: pendingRoundtableMissionLink,
                    onOpenMissions: {
                        fullGovernancePresentation = nil
                        pendingRoundtableMissionLink = nil
                        DispatchQueue.main.async {
                            selectNavigation(.work)
                        }
                    }
                )
                    .frame(minWidth: 860, minHeight: 620)
            } else {
                MissionWorkbench(
                    store: store,
                    showProvidersInitially: presentation == .runtimeProviders,
                    showNewMissionInitially: presentation == .newMission,
                    initialMissionTeamID: presentation == .newMission
                      ? (pendingMissionTeamID.isEmpty
                        ? store.selectedTeamID : pendingMissionTeamID)
                      : nil,
                    initialMissionObjective: pendingMissionObjective,
                    initialConversationThreadID: presentation == .newMission
                      ? store.currentChatThreadID()
                      : nil,
                    initialConversationTitle: presentation == .newMission
                      ? store.selectedChatSession?.title
                      : nil,
                    onReturnToConversation: { missionID in
                        fullGovernancePresentation = nil
                        pendingMissionTeamID = ""
                        pendingMissionObjective = ""
                        Task {
                            await openMissionContext(missionID)
                        }
                    }
                )
                    .frame(minWidth: 1_080, minHeight: 680)
            }
        }
        .sheet(item: $diagnosticPresentation) { presentation in
            DiagnosticBundlePreviewSheet(
                preview: presentation.preview,
                exporter: presentation.exporter
            )
        }
        .sheet(item: $pendingConversationRouteTransition) { transition in
            ConversationRouteTransitionSheet(
                transition: transition,
                initialContextMode: .summaryOnly,
                onConfirm: { mode, trustBoundaryAcknowledged in
                    let confirmed = store.confirmConversationRouteTransition(
                        transition,
                        contextMode: mode,
                        trustBoundaryAcknowledged: trustBoundaryAcknowledged
                    )
                    if confirmed { pendingConversationRouteTransition = nil }
                    return confirmed
                },
                onCancel: { pendingConversationRouteTransition = nil },
                onStartNewConversation: {
                    pendingConversationRouteTransition = nil
                    store.newConversation()
                    store.selectConversationProfile(transition.target.profileID)
                }
            )
                .frame(minWidth: 560, minHeight: 520)
        }
        .alert(
            "Diagnostics could not be prepared",
            isPresented: Binding(
                get: { diagnosticPreparationError != nil },
                set: { visible in
                    if !visible { diagnosticPreparationError = nil }
                }
            )
        ) {
            Button("Open Runtime & Providers") {
                diagnosticPreparationError = nil
                fullGovernancePresentation = .runtimeProviders
            }
            Button("Cancel", role: .cancel) {
                diagnosticPreparationError = nil
            }
        } message: {
            Text(
                diagnosticPreparationError
                    ?? "Loom could not prepare a privacy-safe diagnostic preview."
            )
        }
        .alert(
            "Folder could not be opened",
            isPresented: Binding(
                get: { folderSelectionError != nil },
                set: { visible in
                    if !visible { folderSelectionError = nil }
                }
            )
        ) {
            Button("Try Again") {
                folderSelectionError = nil
                showFolderImporter = true
            }
            Button("Cancel", role: .cancel) {
                folderSelectionError = nil
            }
        } message: {
            Text(
                folderSelectionError
                    ?? "Loom could not access the selected workspace folder."
            )
        }
        .alert(
            "Stop the current response?",
            isPresented: Binding(
                get: { pendingStopAction != nil },
                set: { visible in
                    if !visible { pendingStopAction = nil }
                }
            )
        ) {
            Button("Stop response", role: .destructive) {
                pendingStopAction = nil
                Task { await store.cancelActiveChatResponse() }
            }
            Button("Keep running", role: .cancel) {
                pendingStopAction = nil
            }
        } message: {
            Text("Only the response in progress will stop. Your conversation and draft remain available.")
        }
        .onReceive(
            NotificationCenter.default.publisher(for: .loomNewTaskRequested)
        ) { _ in
            startNewTask()
        }
        .onReceive(
            NotificationCenter.default.publisher(for: .loomOpenFolderRequested)
        ) { _ in
            showFolderImporter = true
        }
        .task(id: store.snapshot?.viewVersion ?? "") {
            await restoreGovernanceSceneStateIfNeeded()
        }
        .onChange(of: governance) { _, _ in
            persistGovernanceSceneState()
        }
        .onChange(of: store.workbench.route) { _, _ in
            persistGovernanceSceneState()
        }
        .onExitCommand {
            governance.dismissIfPresented()
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel("Loom workspace")
    }

    private static func navigationItem(
        for destination: LoomGovernanceDestination
    ) -> LoomWorkspaceNavigationItem {
        switch destination {
        case .team: return .teams
        case .attention: return .attention
        case .library: return .library
        case .runtimes: return .runtimes
        case .overview: return .home
        case .board, .mission, .roundtable, .topology, .timeline,
             .decisions, .evidence:
            return .work
        }
    }

    @MainActor
    private func restoreGovernanceSceneStateIfNeeded() async {
        guard restoresSceneState, !didRestoreSceneState,
              let snapshot = store.snapshot else { return }
        didRestoreSceneState = true
        guard let selection = loomWorkspaceRestorationSelection(
            destinationRawValue: storedGovernanceDestination,
            modeRawValue: storedGovernanceMode,
            missionID: storedGovernanceMissionID,
            availableMissionIDs: Set(snapshot.missions.map(\.missionID))
        ) else {
            clearGovernanceSceneState()
            return
        }

        if !selection.missionID.isEmpty {
            guard await store.openMissionAndActivate(selection.missionID) else {
                clearGovernanceSceneState()
                return
            }
            alignConversationWithMission(selection.missionID)
        } else {
            switch selection.destination {
            case .board: store.showMissionBoard()
            case .team: store.showMissionTeams()
            case .attention: store.showMissionAttention()
            case .library: store.showMissionLibrary()
            default: break
            }
        }
        selectedNavigation = Self.navigationItem(for: selection.destination)
        governance = LoomGovernancePanelState(
            mode: selection.mode,
            destination: selection.destination
        )
    }

    private func persistGovernanceSceneState() {
        guard restoresSceneState else { return }
        guard governance.mode != .hidden else {
            clearGovernanceSceneState()
            return
        }
        storedGovernanceDestination = governance.destination.rawValue
        storedGovernanceMode = governance.mode.rawValue
        if LoomGovernanceDestination.missionContext.contains(
            governance.destination
        ), case .mission(let missionID) = store.workbench.route {
            storedGovernanceMissionID = String(missionID.prefix(256))
        } else if !LoomGovernanceDestination.missionContext.contains(
            governance.destination
        ) {
            storedGovernanceMissionID = ""
        }
    }

    private func clearGovernanceSceneState() {
        storedGovernanceDestination = ""
        storedGovernanceMode = ""
        storedGovernanceMissionID = ""
    }

    private func startNewTask() {
        store.newConversation()
        conversationActionMenu.clear()
        conversationActionMenuForced = false
        conversationActionMenuSuppressed = false
        conversationActionPicker = nil
        conversationActionChoiceMenu.clear()
        conversationActionNotice = nil
        pendingStopAction = nil
        selectedNavigation = .home
        if governance.mode != .pinned {
            governance.close()
        }
        composerFocused = true
    }

    private func navigationRail(compact: Bool) -> some View {
        let conversationLimit = loomRailConversationLimit(compact: compact)
        return VStack(alignment: .leading, spacing: 0) {
            HStack(spacing: 9) {
                Image(systemName: "square.3.layers.3d")
                    .foregroundStyle(LoomGraphite.accent)
                if !compact {
                    Text("Loom")
                        .font(.headline)
                    Spacer()
                }
            }
            .padding(.horizontal, compact ? 20 : 14)
            .frame(height: 52)
            .accessibilityElement(children: .combine)
            .accessibilityLabel("Loom")

            Divider()
                .background(LoomGraphite.separator)

            ScrollView {
                VStack(alignment: .leading, spacing: 6) {
                    Button {
                        startNewTask()
                    } label: {
                        HStack(spacing: 10) {
                            Image(systemName: "square.and.pencil")
                                .frame(width: 20)
                            if !compact {
                                Text("New task")
                                Spacer()
                            }
                        }
                        .padding(.horizontal, 10)
                        .frame(height: 40)
                        .frame(
                            maxWidth: .infinity,
                            alignment: compact ? .center : .leading
                        )
                    }
                    .buttonStyle(.plain)
                    .accessibilityLabel("New task")
                    .help("New conversation")

                    ForEach(LoomWorkspaceNavigationItem.primary) { item in
                        navigationButton(item, compact: compact)
                    }

                    if conversationLimit > 0 {
                        railSectionTitle("CONVERSATIONS", compact: compact)
                        ForEach(Array(store.chatSessions.prefix(conversationLimit))) { session in
                            conversationButton(session, compact: compact)
                        }
                        if store.chatSessions.count > conversationLimit {
                            Menu {
                                ForEach(Array(store.chatSessions.dropFirst(conversationLimit))) { session in
                                    Button {
                                        store.selectChatSession(session.threadID)
                                    } label: {
                                        if session.threadID == store.selectedChatSessionID {
                                            Label(
                                                conversationButtonTitle(for: session),
                                                systemImage: "checkmark"
                                            )
                                        } else {
                                            Text(conversationButtonTitle(for: session))
                                        }
                                    }
                                }
                            } label: {
                                HStack(spacing: 10) {
                                    Image(systemName: "ellipsis")
                                        .frame(width: 20)
                                    Text("More conversations")
                                    Spacer()
                                }
                                .padding(.horizontal, 10)
                                .frame(height: 34)
                                .frame(maxWidth: .infinity, alignment: .leading)
                            }
                            .menuStyle(.borderlessButton)
                            .accessibilityLabel("More conversations")
                            .help("Open an older conversation")
                        }
                    }

                    railSectionTitle("GOVERN", compact: compact)
                    ForEach(LoomWorkspaceNavigationItem.governance) { item in
                        navigationButton(item, compact: compact)
                    }

                    railSectionTitle("TOOLS", compact: compact)
                    ForEach(LoomWorkspaceNavigationItem.tools) { item in
                        navigationButton(item, compact: compact)
                    }

                    if !compact, store.workspace.tasks.count > 1 {
                        railSectionTitle("RECENT", compact: compact)
                        ForEach(
                            Array(
                                store.workspace.tasks.dropFirst().prefix(6).enumerated()
                            ),
                            id: \.element.id
                        ) { item in
                            recentTaskButton(item.element, position: item.offset + 1)
                        }
                    }
                }
                .padding(.horizontal, compact ? 8 : 10)
                .padding(.vertical, 12)
            }

            Spacer()

            Divider()
                .background(LoomGraphite.separator)

            Button {
                governance.open(.runtimes)
                selectedNavigation = .runtimes
            } label: {
                HStack(spacing: 10) {
                    Image(systemName: "waveform.path.ecg")
                        .frame(width: 20)
                    if !compact {
                        Text(serviceFooterTitle)
                            .font(.caption)
                            .lineLimit(1)
                        Spacer()
                    }
                }
                .padding(.horizontal, compact ? 10 : 10)
                .frame(height: 44)
                .frame(maxWidth: .infinity, alignment: compact ? .center : .leading)
            }
            .buttonStyle(.plain)
            .accessibilityLabel("Open local service and runtime health")
            .help(serviceFooterTitle)
            .padding(.horizontal, 8)
            .padding(.vertical, 6)
        }
        .background(LoomGraphite.rail)
    }

    private func navigationButton(
        _ item: LoomWorkspaceNavigationItem,
        compact: Bool
    ) -> some View {
        let selected = selectedNavigation == item
        let count = navigationCount(item)
        let hasRecordedActivity = navigationHasRecordedActivity(item)
        return Button {
            selectNavigation(item)
        } label: {
            HStack(spacing: 10) {
                Image(systemName: item.symbol)
                    .frame(width: 20)
                    .foregroundStyle(selected ? LoomGraphite.accent : Color.secondary)
                    .overlay(alignment: .topTrailing) {
                        if compact, (count ?? 0) > 0 || hasRecordedActivity {
                            Circle()
                                .fill(
                                    item == .attention
                                        ? LoomGraphite.statusWarning
                                        : LoomGraphite.accent
                                )
                                .frame(width: 6, height: 6)
                                .offset(x: 3, y: -2)
                                .accessibilityHidden(true)
                        }
                    }
                if !compact {
                    Text(item.rawValue)
                        .font(.callout.weight(selected ? .semibold : .regular))
                    Spacer()
                    if let count, count > 0 {
                        Text("\(count)")
                            .font(.caption2.monospacedDigit())
                            .foregroundStyle(.secondary)
                    } else if hasRecordedActivity {
                        Image(systemName: "circle.fill")
                            .font(.system(size: 5))
                            .foregroundStyle(.tertiary)
                            .accessibilityLabel("Recorded activity")
                    }
                }
            }
            .padding(.horizontal, compact ? 10 : 10)
            .frame(height: 40)
            .frame(maxWidth: .infinity, alignment: compact ? .center : .leading)
            .background(
                selected ? LoomGraphite.accentMuted : Color.clear,
                in: RoundedRectangle(cornerRadius: 7, style: .continuous)
            )
        }
        .buttonStyle(.plain)
        .accessibilityLabel(item.accessibilityLabel)
        .accessibilityAddTraits(selected ? .isSelected : [])
        .help(item.rawValue)
    }

    private func navigationCount(
        _ item: LoomWorkspaceNavigationItem
    ) -> Int? {
        switch item {
        case .work: return activeMissionCount
        case .attention: return activeAttentionCount
        case .teams: return store.snapshot?.teams.count ?? 0
        case .library: return store.snapshot?.evidence.count ?? 0
        default: return nil
        }
    }

    private func navigationHasRecordedActivity(
        _ item: LoomWorkspaceNavigationItem
    ) -> Bool {
        guard item == .work, let snapshot = store.snapshot else { return false }
        return !snapshot.missions.isEmpty
            || !snapshot.teams.isEmpty
            || !snapshot.runs.isEmpty
            || !snapshot.evidence.isEmpty
    }

    private func selectNavigation(_ item: LoomWorkspaceNavigationItem) {
        selectedNavigation = item
        switch item {
        case .home:
            if governance.mode != .pinned {
                governance.close()
            }
        case .work:
            store.showMissionBoard()
            governance.open(.board)
        case .teams:
            store.showMissionTeams()
            governance.open(.team)
        case .roundtable:
            if case .mission(let missionID) = store.workbench.route,
               let mission = store.snapshot?.missions.first(where: {
                   $0.missionID == missionID
               }) {
                pendingRoundtableMissionLink = roundtableLink(for: mission)
            } else {
                pendingRoundtableMissionLink = nil
            }
            roundtableContextView = nil
            governance.open(.roundtable)
        case .attention:
            store.showMissionAttention()
            governance.open(.attention)
        case .library:
            store.showMissionLibrary()
            governance.open(.library)
        case .runtimes:
            governance.open(.runtimes)
        }
    }

    @ViewBuilder
    private func railSectionTitle(
        _ title: String,
        compact: Bool
    ) -> some View {
        if compact {
            Color.clear
                .frame(height: 8)
                .accessibilityHidden(true)
        } else {
            Text(title)
                .font(.caption2.weight(.semibold))
                .foregroundStyle(.secondary)
                .padding(.horizontal, 10)
                .padding(.top, 16)
                .padding(.bottom, 3)
        }
    }

    /// Sidebar titles come from the first user message, so several
    /// conversations can share the same label (e.g. "hello"). When another
    /// session has the same title, append a short date so they are
    /// distinguishable at a glance without reading the relative time.
    private func conversationButtonTitle(
        for session: LocalProductChatSession
    ) -> String {
        let sameTitle = store.chatSessions.filter {
            $0.title == session.title
        }
        guard sameTitle.count > 1 else { return session.title }
        return "\(session.title) · \(conversationDisambiguationSuffix(for: session.createdAt))"
    }

    private func conversationButton(
        _ session: LocalProductChatSession,
        compact: Bool
    ) -> some View {
        let active = session.threadID == store.selectedChatSessionID
        return Button {
            store.selectChatSession(session.threadID)
        } label: {
            HStack(spacing: 10) {
                Image(systemName: active ? "bubble.left.fill" : "bubble.left")
                    .foregroundStyle(
                        active ? LoomGraphite.accent : Color.secondary
                    )
                    .frame(width: 20)
                if !compact {
                    VStack(alignment: .leading, spacing: 1) {
                        Text(conversationButtonTitle(for: session))
                            .font(.caption)
                            .foregroundStyle(active ? Color.primary : Color.secondary)
                            .lineLimit(1)
                        Text(Self.conversationRelativeTime(session.updatedAt))
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                            .lineLimit(1)
                    }
                    Spacer()
                    if active {
                        Image(systemName: "checkmark")
                            .font(.caption2)
                            .foregroundStyle(LoomGraphite.accent)
                    }
                }
            }
            .padding(.horizontal, 10)
            .frame(height: 38)
            .frame(maxWidth: .infinity, alignment: compact ? .center : .leading)
            .background(
                active ? LoomGraphite.accent.opacity(0.12) : Color.clear
            )
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityLabel("Conversation \(session.title)")
        .contextMenu {
            Button(role: .destructive) {
                Task { await store.deleteChatSession(session.threadID) }
            } label: {
                Label("Delete Conversation", systemImage: "trash")
            }
        }
    }

    private static func conversationRelativeTime(_ date: Date) -> String {
        let elapsed = Date().timeIntervalSince(date)
        let formatter = RelativeDateTimeFormatter()
        formatter.unitsStyle = .abbreviated
        if elapsed < 60 {
            return "just now"
        }
        return formatter.localizedString(for: date, relativeTo: Date())
    }

    private func recentTaskButton(
        _ task: LocalProductWorkspaceTask,
        position: Int
    ) -> some View {
        let actionLabel = LoomRecentTaskActionLabel.make(
            for: task.title,
            subtitle: task.subtitle,
            position: position
        )
        return Button {
            Task {
                store.selectWorkspaceTask(task.id)
                await store.activateWorkspaceTask()
            }
        } label: {
            VStack(alignment: .leading, spacing: 2) {
                Text(task.title)
                    .font(.callout)
                    .lineLimit(1)
                Text(task.subtitle)
                    .font(.caption2)
                    .foregroundStyle(.secondary)
                    .lineLimit(1)
            }
            .padding(.horizontal, 10)
            .frame(height: 42)
            .frame(maxWidth: .infinity, alignment: .leading)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityLabel(actionLabel)
        .help(actionLabel)
    }

    private var centerWorkspace: some View {
        VStack(spacing: 0) {
            centerHeader
                .padding(.horizontal, 18)
                .background(LoomGraphite.surface)

            connectionStripIfNeeded

            ScrollViewReader { timeline in
                ScrollView {
                    VStack(spacing: 0) {
                        Group {
                            if store.chatThread?.messages.isEmpty ?? true {
                                emptyConversation
                            } else if chatSelectMode {
                                chatTranscriptView
                            } else {
                                chatTimeline
                            }
                        }
                        .frame(maxWidth: 760, alignment: .leading)
                        .padding(.horizontal, 24)
                        .padding(.vertical, 28)
                        .frame(maxWidth: .infinity, minHeight: 360)

                        Color.clear
                            .frame(height: 1)
                            .id(Self.chatTimelineBottomID)
                    }
                }
                .onAppear {
                    timeline.scrollTo(Self.chatTimelineBottomID, anchor: .bottom)
                }
                .onChange(of: store.chatThread?.messages.last?.messageID) { _, _ in
                    withAnimation(.easeOut(duration: 0.2)) {
                        timeline.scrollTo(Self.chatTimelineBottomID, anchor: .bottom)
                    }
                }
            }

            composer
                .padding(.horizontal, 18)
                .padding(.bottom, 14)
                .background(LoomGraphite.surface)
        }
        .background(LoomGraphite.raised)
        .task { await store.loadChatThread() }
    }

    private var centerHeader: some View {
        HStack(spacing: 10) {
            VStack(alignment: .leading, spacing: 2) {
                HStack(spacing: 6) {
                    Menu {
                        ForEach(store.chatSessions) { session in
                            Button {
                                store.selectChatSession(session.threadID)
                            } label: {
                                if session.threadID
                                  == store.selectedChatSessionID
                                {
                                    Label(session.title, systemImage: "checkmark")
                                } else {
                                    Text(session.title)
                                }
                            }
                        }
                        Divider()
                        Button {
                            startNewTask()
                        } label: {
                            Label("New Conversation", systemImage: "square.and.pencil")
                        }
                    } label: {
                        HStack(spacing: 6) {
                            Text(
                                store.selectedChatSession?.title
                                  ?? "Conversation"
                            )
                            .font(.headline)
                            .lineLimit(1)
                            Image(systemName: "chevron.down")
                                .font(.caption2)
                                .foregroundStyle(.secondary)
                        }
                        .contentShape(Rectangle())
                    }
                    .menuStyle(.borderlessButton)
                    .fixedSize()
                    .accessibilityLabel("Switch conversation")

                    Button {
                        copyConversationTranscript()
                    } label: {
                        Image(systemName: "doc.on.doc")
                            .frame(width: 22, height: 22)
                    }
                    .buttonStyle(.plain)
                    .help("Copy the whole conversation")
                    .accessibilityLabel("Copy conversation")
                    .disabled(store.chatThread?.messages.isEmpty ?? true)

                    Button {
                        chatSelectMode.toggle()
                    } label: {
                        Image(
                            systemName: chatSelectMode
                                ? "bubble.left.and.bubble.right"
                                : "text.alignleft"
                        )
                            .frame(width: 22, height: 22)
                    }
                    .buttonStyle(.plain)
                    .help(
                        chatSelectMode
                            ? "Show message bubbles"
                            : "Show the whole conversation as one selectable text"
                    )
                    .accessibilityLabel(
                        chatSelectMode
                            ? "Show message bubbles"
                            : "Selectable conversation text"
                    )

                    Button {
                        startNewTask()
                    } label: {
                        Image(systemName: "square.and.pencil")
                            .frame(width: 22, height: 22)
                    }
                    .buttonStyle(.plain)
                    .help("New Conversation")
                    .accessibilityLabel("New Conversation")
                }
                if let folder = store.workspace.selectedFolderDisplayName {
                    Label(folder, systemImage: "folder")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .lineLimit(1)
                } else {
                    Text(store.workspace.selectedTask.title)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .lineLimit(1)
                }
            }
            Spacer()
            Button {
                if governance.mode == .hidden {
                    governance.open(.overview)
                } else {
                    governance.close()
                }
            } label: {
                Image(systemName: "sidebar.trailing")
                    .frame(width: 36, height: 36)
            }
            .buttonStyle(.plain)
            .accessibilityLabel(
                governance.mode == .hidden
                    ? "Open governance inspector"
                    : "Close governance inspector"
            )
            .help("Governance inspector")
        }
        .frame(height: 52)
    }

    @ViewBuilder
    private var connectionStripIfNeeded: some View {
        if experience.state != .connectedEmpty,
           experience.state != .connectedPopulated {
            HStack(spacing: 10) {
                Image(systemName: experience.connection.systemImage)
                    .foregroundStyle(indicatorColor)
                    .accessibilityHidden(true)
                VStack(alignment: .leading, spacing: 1) {
                    Text(experience.connection.title)
                        .font(.callout.weight(.medium))
                    Text(experience.connection.detail)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                Spacer()
                if experience.connection.offersRefresh {
                    Button("Try Again") {
                        Task { await store.refresh() }
                    }
                    .help("Try Again")
                }
            }
            .padding(.horizontal, 18)
            .padding(.vertical, 9)
            .background(stripBackground)
        }
    }

    private var emptyConversation: some View {
        return VStack(alignment: .leading, spacing: 14) {
            Image(systemName: "bubble.left.and.text.bubble.right")
                .font(.system(size: 26, weight: .regular))
                .foregroundStyle(LoomGraphite.accent)
                .accessibilityHidden(true)
            Text("What are you working on?")
                .font(.title2.weight(.semibold))
        }
        .frame(maxWidth: 560, alignment: .leading)
        .padding(.top, 48)
    }

    /// Selectable-text mode: the whole conversation is rendered as a single
    /// Text view, so dragging across the window selects several messages at
    /// once and copies them together. SwiftUI text selection never merges
    /// across separate Text views (each bubble is its own view), so this is
    /// the reliable way to satisfy "drag over the whole conversation to copy".
    private var chatTranscriptView: some View {
        VStack(alignment: .leading, spacing: 12) {
            Label(
                "Selectable text — drag across any part of the conversation to copy several messages at once.",
                systemImage: "text.cursor"
            )
                .font(.caption)
                .foregroundStyle(.secondary)
            Text(conversationTranscriptText(store.chatThread?.messages ?? []))
                .font(.body)
                .textSelection(.enabled)
                .fixedSize(horizontal: false, vertical: true)
        }
    }

    private var chatTimeline: some View {
        // A non-lazy VStack keeps every message in one selectable region so a
        // user can drag across the whole conversation and copy several
        // messages together (LazyVStack splits rows into separate hosting
        // views, which breaks cross-message text selection on macOS).
        VStack(alignment: .leading, spacing: 18) {
            ForEach(store.chatThread?.messages ?? [], id: \.messageID) { message in
                let isAgentProposal = message.role == "proposal"
                HStack(alignment: .top, spacing: 10) {
                    Image(systemName: message.role == "user" ? "person.crop.circle" : "sparkles")
                        .foregroundStyle(
                            message.role == "user" ? LoomGraphite.accent : Color.secondary
                        )
                        .frame(width: 22)
                        .accessibilityHidden(true)
                    VStack(alignment: .leading, spacing: 5) {
                        Text(message.role == "user" ? "You" : isAgentProposal ? "Loom proposal" : "Loom")
                            .font(.caption.weight(.semibold))
                            .foregroundStyle(
                                message.role == "user"
                                    ? LoomGraphite.accent
                                    : isAgentProposal ? Color.secondary : Color.primary
                            )
                        if let route = conversationRouteLabel(for: message) {
                            Label(route, systemImage: "point.3.connected.trianglepath.dotted")
                                .font(.caption2)
                                .foregroundStyle(.tertiary)
                                .lineLimit(2)
                        }
                        if let disclosure = conversationDisclosureSummary(for: message) {
                            DisclosureGroup {
                                ConversationContextCapacityDetails(disclosure: disclosure)
                                VStack(alignment: .leading, spacing: 3) {
                                    Text("Disclosure receipt")
                                        .foregroundStyle(.secondary)
                                    Text(disclosure.disclosureReceiptDigest)
                                        .font(.system(.caption2, design: .monospaced))
                                        .textSelection(.enabled)
                                        .lineLimit(2)
                                }
                                Divider()
                                let disclosureIdentity = store
                                    .conversationContextDisclosureIdentity(for: disclosure)
                                if let disclosureIdentity,
                                   let inspection = store.conversationContextDisclosures[
                                    disclosureIdentity
                                   ] {
                                    conversationContextInspection(inspection)
                                } else {
                                    if let disclosureIdentity,
                                       store.conversationContextDisclosuresInFlight.contains(
                                        disclosureIdentity
                                       ) {
                                        HStack(spacing: 8) {
                                            ProgressView()
                                                .controlSize(.small)
                                            Text("Inspecting context")
                                                .foregroundStyle(.secondary)
                                        }
                                        .accessibilityElement(children: .combine)
                                    } else {
                                        Button {
                                            Task {
                                                await store.loadConversationContextDisclosure(
                                                    disclosure
                                                )
                                            }
                                        } label: {
                                            Label(
                                                "Inspect context",
                                                systemImage: "doc.text.magnifyingglass"
                                            )
                                        }
                                        .buttonStyle(.borderless)
                                        .accessibilityHint(
                                            "Shows disclosed categories and omission reasons without message content"
                                        )
                                    }
                                    if let disclosureIdentity,
                                       let failure = store
                                        .conversationContextDisclosureFailures[
                                            disclosureIdentity
                                        ] {
                                        Label(
                                            "Context details unavailable · \(conversationPolicyLabel(failure))",
                                            systemImage: "exclamationmark.triangle"
                                        )
                                        .font(.caption)
                                        .foregroundStyle(LoomGraphite.statusDanger)
                                    }
                                }
                                if let binding = disclosure.executionBinding {
                                    Divider()
                                    LabeledContent(
                                        "Provider account",
                                        value: binding.providerAccountID.isEmpty
                                            ? binding.providerID
                                            : binding.providerAccountID
                                    )
                                    LabeledContent(
                                        "Account policy",
                                        value: binding.providerAccountPolicyVersion == 0
                                            ? "Unspecified"
                                            : "v\(binding.providerAccountPolicyVersion) · r\(binding.providerAccountPolicyRevision)"
                                    )
                                    if binding.providerAccountPolicyVersion == 2 {
                                        LabeledContent(
                                            "Trust domain",
                                            value: conversationPolicyLabel(binding.trustDomain)
                                        )
                                        LabeledContent(
                                            "Retention",
                                            value: conversationPolicyLabel(binding.retentionMode)
                                        )
                                        LabeledContent(
                                            "Data region",
                                            value: conversationPolicyLabel(binding.dataRegion)
                                        )
                                    }
                                }
                            } label: {
                                Label(
                                    conversationContextMeterLabel(disclosure),
                                    systemImage: "doc.text.magnifyingglass"
                                )
                            }
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                        }
                        HStack(alignment: .top, spacing: 6) {
                            Text(message.displayContent)
                                .font(.body)
                                .foregroundStyle(isAgentProposal ? Color.secondary : Color.primary)
                                .textSelection(.enabled)
                                .fixedSize(horizontal: false, vertical: true)
                                .contextMenu {
                                    Button {
                                        copyChatMessage(message.displayContent)
                                    } label: {
                                        Label("Copy", systemImage: "doc.on.doc")
                                    }
                                }
                            Button {
                                copyChatMessage(message.displayContent)
                            } label: {
                                Image(systemName: "doc.on.doc")
                                    .font(.caption2)
                                    .foregroundStyle(.tertiary)
                            }
                            .buttonStyle(.plain)
                            .help("Copy message text")
                        }
                        if isAgentProposal {
                            if !store.executableTeams.isEmpty {
                                Button {
                                    runMissionFromChat(message)
                                } label: {
                                    Label("Run as Mission", systemImage: "play.circle")
                                }
                                .buttonStyle(.borderedProminent)
                                .controlSize(.small)
                                .accessibilityLabel("Run this conversation as a Mission")
                                .help("Pre-fill a Mission from this conversation and start it")
                            }
                            Button {
                                governance.open(.team)
                                Task { await store.startBlankBuilder() }
                            } label: {
                                Label("Create Agent Team", systemImage: "person.3.badge.plus")
                            }
                            .buttonStyle(.bordered)
                            .controlSize(.small)
                            .accessibilityLabel("Create Agent Team")
                            .help("Start a governed Agent Team to execute this work")
                        }
                    }
                    Spacer(minLength: 0)
                }
                .accessibilityElement(children: .combine)
                .accessibilityLabel(
                    message.role == "user"
                        ? "You: \(message.content)"
                        : isAgentProposal
                            ? "Loom proposal, untrusted: \(message.displayContent)"
                            : "Loom: \(message.displayContent)"
                )
                if let attempt = conversationAttempt(for: message),
                   !attempt.completedControlTools.isEmpty {
                    ConversationCompletedToolActivity(
                        attemptID: attempt.attemptID,
                        toolIDs: attempt.completedControlTools.map(\.toolID)
                    )
                    .padding(.leading, 32)
                }
                ForEach(
                    (store.chatThread?.controlProposals ?? []).filter {
                        $0.messageID == message.messageID
                    }
                ) { proposal in
                    conversationControlProposal(proposal)
                }
                ForEach(
                    (store.chatThread?.actionProposals ?? []).filter {
                        $0.messageID == message.messageID
                    }
                ) { proposal in
                    conversationActionProposal(proposal)
                }
            }
            if store.isSendingChatMessage {
                HStack(alignment: .top, spacing: 10) {
                    Image(systemName: "sparkles")
                        .foregroundStyle(.secondary)
                        .frame(width: 22)
                    VStack(alignment: .leading, spacing: 7) {
                        Text("Loom")
                            .font(.caption.weight(.semibold))
                            .foregroundStyle(.secondary)
                        ProgressView()
                            .controlSize(.small)
                            .accessibilityLabel("Loom is responding")
                    }
                    Spacer(minLength: 0)
                }
            }
        }
        .textSelection(.enabled)
    }

    private func conversationControlProposal(
        _ proposal: LocalProductConversationControlProposal
    ) -> some View {
        let alignment = store.chatThread?.contextAlignments.first {
            $0.proposalID == proposal.proposalID
        }
        let inFlight = store.controlProposalDecisionsInFlight.contains(
            proposal.proposalID
        )
        return ConversationControlProposalCard(
            proposal: proposal,
            alignment: alignment,
            inFlight: inFlight,
            confirmable: store.canConfirmConversationControlProposal(proposal),
            decisionReceipt: store.chatThread?.decisionReceipt(for: proposal)
        ) { decision in
            Task {
                await store.decideConversationControlProposal(
                    proposal,
                    decision: decision
                )
            }
        }
    }

    private func conversationActionProposal(
        _ proposal: LocalProductConversationActionProposal
    ) -> some View {
        let inFlight = store.controlProposalDecisionsInFlight.contains(
            proposal.proposalID
        )
        return ConversationActionProposalCard(
            proposal: proposal,
            inFlight: inFlight,
            targetDisplayName: store.snapshot?.missions.first(where: {
                $0.missionID == proposal.payload?.missionID
            })?.title ?? "",
            decisionReceipt: store.chatThread?.decisionReceipt(for: proposal)
        ) { decision in
            Task {
                if proposal.status == .confirmed, decision == .confirm {
                    await executeConfirmedConversationActionProposal(proposal)
                    return
                }
                let confirmed = await store.decideConversationActionProposal(
                    proposal,
                    decision: decision
                )
                if confirmed {
                    await executeConfirmedConversationActionProposal(proposal)
                }
            }
        }
    }

    private var composer: some View {
        let action = conversationComposerActionPresentation(
            isSending: store.isSendingChatMessage,
            isCancelling: store.isCancellingChatResponse
        )
        let actionEnabled = store.isSendingChatMessage
            ? action.isEnabled
            : conversationComposerSubmissionEnabled
        return VStack(alignment: .leading, spacing: 8) {
            if let picker = conversationActionPicker {
                let choices = conversationActionChoices(for: picker)
                ConversationActionChoicePalette(
                    title: conversationActionPickerTitle(picker.kind),
                    choices: choices,
                    selectedID: conversationActionChoiceMenu.selectedID,
                    onSelect: { choice in
                        selectConversationActionChoice(choice, for: picker.kind)
                    },
                    onDismiss: dismissConversationActionOverlay
                )
                .frame(maxWidth: .infinity)
                .transition(.opacity.combined(with: .move(edge: .bottom)))
                .onAppear {
                    conversationActionChoiceMenu.synchronize(with: choices)
                }
                .onChange(of: choices) { _, updated in
                    conversationActionChoiceMenu.synchronize(with: updated)
                }
            } else if conversationActionMenuVisible {
                ConversationActionPalette(
                    suggestions: conversationActionSuggestions,
                    selectedID: conversationActionMenu.selectedID,
                    context: conversationActionContext,
                    onSelect: { definition in
                        let draft = store.workspace.selectedContinuity.composerDraft
                        let commandInput = draft.trimmingCharacters(
                            in: .whitespacesAndNewlines
                        ).hasPrefix("/")
                            ? draft
                            : "/\(definition.command) \(draft)"
                        let resolution = ConversationActionRouter.commandMenuResolution(
                            action: definition.id,
                            input: commandInput
                        )
                        if case .request(let request) = resolution {
                            executeConversationAction(request)
                        } else if case .rejectedActionArgument = resolution {
                            showConversationActionNotice(
                                title: "Credential text rejected",
                                detail: "Credentials can only be managed in Runtime & Providers.",
                                systemImage: "key.slash",
                                warning: true
                            )
                        }
                    },
                    onDismiss: dismissConversationActionOverlay
                )
                .frame(maxWidth: .infinity)
                .transition(.opacity.combined(with: .move(edge: .bottom)))
            }

            if let notice = conversationActionNotice {
                conversationActionNoticeView(notice)
            }

            if let notice = store.conversationModelSelectionNotice {
                HStack(alignment: .top, spacing: 6) {
                    Image(systemName: "exclamationmark.triangle")
                        .font(.caption)
                        .foregroundStyle(.orange)
                    Text(notice)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .fixedSize(horizontal: false, vertical: true)
                        .textSelection(.enabled)
                    Spacer()
                }
                .padding(.horizontal, 2)
                .padding(.bottom, 2)
                .accessibilityLabel("Model selection notice")
            }

            VStack(alignment: .leading, spacing: 6) {
                TextField(
                    "Ask Loom or describe a task",
                    text: Binding(
                        get: { store.workspace.selectedContinuity.composerDraft },
                        set: updateConversationComposerDraft
                    ),
                    axis: .vertical
                )
                .textFieldStyle(.plain)
                .focused($composerFocused)
                .lineLimit(1...6)
                .accessibilityLabel("Message Loom")
                .accessibilityHint("Describe a task, ask a question, or type slash for commands")
                .accessibilityIdentifier("loom.conversation.composer")
                .padding(.horizontal, 4)
                .padding(.top, 5)
                .onAppear {
                    composerFocused = true
                    conversationActionMenu.synchronize(
                        with: store.workspace.selectedContinuity.composerDraft
                    )
                }
                .onSubmit {
                    sendConversationDraft()
                }
                .onKeyPress(.downArrow) {
                    if let picker = conversationActionPicker {
                        conversationActionChoiceMenu.move(
                            .next,
                            within: conversationActionChoices(for: picker)
                        )
                        return .handled
                    }
                    guard conversationActionMenuVisible else { return .ignored }
                    conversationActionMenu.move(
                        .next,
                        for: conversationActionMenuInput
                    )
                    return .handled
                }
                .onKeyPress(.upArrow) {
                    if let picker = conversationActionPicker {
                        conversationActionChoiceMenu.move(
                            .previous,
                            within: conversationActionChoices(for: picker)
                        )
                        return .handled
                    }
                    guard conversationActionMenuVisible else { return .ignored }
                    conversationActionMenu.move(
                        .previous,
                        for: conversationActionMenuInput
                    )
                    return .handled
                }
                .onKeyPress(.escape) {
                    guard conversationActionMenuVisible
                            || conversationActionPicker != nil
                            || conversationActionNotice != nil else {
                        return .ignored
                    }
                    dismissConversationActionOverlay()
                    return .handled
                }
                .onKeyPress(.return) {
                    if let picker = conversationActionPicker,
                       let choice = conversationActionChoiceMenu.selectedChoice(
                           in: conversationActionChoices(for: picker)
                       ) {
                        selectConversationActionChoice(choice, for: picker.kind)
                        return .handled
                    }
                    guard conversationActionMenuVisible else { return .ignored }
                    sendConversationDraft()
                    return .handled
                }

                HStack(spacing: 8) {
                    Menu {
                        Button {
                            showFolderImporter = true
                        } label: {
                            Label("Choose folder", systemImage: "folder")
                        }
                        Button {
                            governance.open(.team)
                            Task { await store.startBlankBuilder() }
                        } label: {
                            Label("Use Agent Team", systemImage: "person.3")
                        }
                        Button {
                            pendingMissionObjective = composerNewMissionObjective(
                                thread: store.chatThread
                            )
                            pendingMissionTeamID = ""
                            store.showMissionBoard()
                            fullGovernancePresentation = .newMission
                        } label: {
                            Label("Start Mission", systemImage: "flag.checkered")
                        }
                    } label: {
                        Image(systemName: "plus")
                            .frame(width: 28, height: 28)
                    }
                    .menuStyle(.borderlessButton)
                    .accessibilityLabel("Add workspace or governed work")
                    .help("Add folder, Agent Team, or Mission")

                    Button {
                        presentConversationCommands()
                    } label: {
                        Image(systemName: "command")
                            .frame(width: 28, height: 28)
                    }
                    .buttonStyle(.plain)
                    .accessibilityLabel("Commands")
                    .help("Commands")

                    if let folder = store.workspace.selectedFolderDisplayName {
                        Label(folder, systemImage: "folder")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                            .lineLimit(1)
                    }

                    Spacer(minLength: 8)

                    if !store.availableConversationProfiles.isEmpty {
                        conversationSelectionControls
                    }

                    Button {
                        if store.isSendingChatMessage {
                            Task { await store.cancelActiveChatResponse() }
                        } else {
                            sendConversationDraft()
                        }
                    } label: {
                        Image(systemName: action.systemImage)
                            .font(.body.weight(.semibold))
                            .foregroundStyle(LoomGraphite.onAccent)
                            .frame(width: 34, height: 34)
                            .background(
                                actionEnabled
                                    ? LoomGraphite.accent
                                    : LoomGraphite.accent.opacity(0.35),
                                in: Circle()
                            )
                    }
                    .buttonStyle(.plain)
                    .disabled(!actionEnabled)
                    .accessibilityLabel(action.accessibilityLabel)
                    .help(action.help)
                }
            }
            .padding(.horizontal, 10)
            .padding(.vertical, 8)
            .background(
                LoomGraphite.canvas,
                in: RoundedRectangle(cornerRadius: 12, style: .continuous)
            )
            .overlay {
                RoundedRectangle(cornerRadius: 12, style: .continuous)
                    .stroke(LoomGraphite.separator, lineWidth: 1)
            }

            if let failure = store.chatOperationFailure {
                chatFailureBanner(failure)
            }
        }
    }

    private func requestConversationProfileSelection(_ profileID: String) {
        pendingConversationRouteTransition =
            store.requestConversationProfileSelection(profileID)
    }

    @ViewBuilder
    private var conversationSelectionControls: some View {
        let selectedProfile = store.selectedConversationProfile
        let providerID = selectedProfile?.providerID ?? ""
        let models = store.conversationModels(profile: selectedProfile)
        let effectiveModelID = store.effectiveConversationModelID
        let effectiveReasoning = store.effectiveConversationReasoningEffort
        let reasoningEfforts = models.first {
            $0.modelID == effectiveModelID
        }?.reasoningEfforts ?? []

        Menu {
            ForEach(conversationRouteMenuGroups(store.availableConversationProfiles)) { group in
                Section(group.displayName) {
                    ForEach(group.profiles) { profile in
                        Button {
                            requestConversationProfileSelection(profile.profileID)
                        } label: {
                            Label(
                                conversationRouteOptionLabel(profile),
                                systemImage: profile.profileID == store.selectedConversationProfileID
                                    ? "checkmark"
                                    : "circle"
                            )
                        }
                    }
                }
            }
        } label: {
            Label(
                selectedProfile.map(conversationProfileMenuLabel) ?? "Route",
                systemImage: "point.3.connected.trianglepath.dotted"
            )
        }
        .menuStyle(.borderlessButton)
        .fixedSize()
        .accessibilityLabel("Choose conversation route")
        .help("Choose an execution Route: Harness and Provider Account")

        if !models.isEmpty {
            Menu {
                ForEach(models) { model in
                    let available = store.isConversationModelAvailable(
                        profile: selectedProfile,
                        modelID: model.modelID
                    )
                    Button {
                        // Keep unavailable models clickable so the user learns
                        // exactly why the model cannot run (the store records a
                        // selection notice with the owning Provider instead of
                        // silently doing nothing).
                        store.selectConversationModel(model.modelID)
                    } label: {
                        Label(
                            model.displayName,
                            systemImage: model.modelID == effectiveModelID
                                ? "checkmark"
                                : "circle"
                        )
                        .foregroundStyle(available ? Color.primary : Color.secondary)
                    }
                    .help(
                        available
                            ? "Model depends on the Provider"
                            : (store.conversationModelUnavailableReason(
                                providerID: providerID,
                                modelID: model.modelID
                              ) ?? "Model unavailable")
                    )
                }
                if models.contains(where: { model in
                    !store.isConversationModelAvailable(
                        profile: selectedProfile,
                        modelID: model.modelID
                    )
                }) {
                    Divider()
                    Text(
                        "Models whose Provider credential is not verified are shown dimmed. Choose one to see why it cannot run."
                    )
                    .font(.caption2)
                    .foregroundStyle(.secondary)
                }
            } label: {
                Label(
                    conversationModelDisplayName(
                        profile: selectedProfile,
                        modelID: effectiveModelID
                    ),
                    systemImage: "cpu"
                )
            }
            .menuStyle(.borderlessButton)
            .fixedSize()
            .accessibilityLabel("Choose Model")
            .help("Model depends on the Provider")
        }

        if !reasoningEfforts.isEmpty {
            Menu {
                Button {
                    store.selectConversationReasoningEffort("")
                } label: {
                    Label(
                        "Provider default",
                        systemImage: effectiveReasoning.isEmpty
                            ? "checkmark"
                            : "circle"
                    )
                }
                Divider()
                ForEach(reasoningEfforts, id: \.self) { effort in
                    Button {
                        store.selectConversationReasoningEffort(effort)
                    } label: {
                        Label(
                            conversationReasoningEffortLabel(effort),
                            systemImage: effort == effectiveReasoning
                                ? "checkmark"
                                : "circle"
                        )
                    }
                }
            } label: {
                Label(
                    effectiveReasoning.isEmpty
                        ? "Reasoning"
                        : conversationReasoningEffortLabel(effectiveReasoning),
                    systemImage: "brain"
                )
            }
            .menuStyle(.borderlessButton)
            .fixedSize()
            .accessibilityLabel("Choose reasoning effort")
            .help("Reasoning effort depends on the Model")
        }
    }

    private func conversationModelDisplayName(
        profile: LocalProductConversationProfile?,
        modelID: String
    ) -> String {
        if let model = store.conversationModels(profile: profile)
            .first(where: { $0.modelID == modelID })
        {
            return model.displayName
        }
        return modelID.isEmpty ? "Model" : modelID
    }

    private func conversationReasoningEffortLabel(_ effort: String) -> String {
        switch effort {
        case "none": return "None"
        case "minimal": return "Minimal"
        case "low": return "Low"
        case "medium": return "Medium"
        case "high": return "High"
        case "xhigh": return "Extra high"
        case "max": return "Max"
        case "ultra": return "Ultra"
        default: return effort.capitalized
        }
    }

    private func runMissionFromChat(_ message: LocalProductChatMessage) {
        let objective = chatMissionObjectiveText(
            around: message,
            thread: store.chatThread
        )
        pendingMissionObjective = objective
        pendingMissionTeamID = ""
        store.showMissionBoard()
        fullGovernancePresentation = .newMission
    }

    /// Builds a Mission objective from the conversation around a proposal:
    /// the proposal's own content first, then the latest user message.
    private func chatMissionObjective(
        around message: LocalProductChatMessage,
        thread: LocalProductChatThread?
    ) -> String {
        chatMissionObjectiveText(around: message, thread: thread)
    }

    private func copyChatMessage(_ text: String) {
        let pasteboard = NSPasteboard.general
        pasteboard.clearContents()
        pasteboard.setString(text, forType: .string)
    }

    /// Copies the whole visible conversation as a readable transcript so the
    /// user never has to copy messages one by one.
    private func copyConversationTranscript() {
        guard let thread = store.chatThread else { return }
        let text = conversationTranscriptText(thread.messages)
        guard !text.isEmpty else { return }
        let pasteboard = NSPasteboard.general
        pasteboard.clearContents()
        pasteboard.setString(text, forType: .string)
    }

    private var conversationActionMenuVisible: Bool {
        guard conversationActionPicker == nil,
              !conversationActionMenuSuppressed else { return false }
        let draft = store.workspace.selectedContinuity.composerDraft
            .trimmingCharacters(in: .whitespacesAndNewlines)
        return conversationActionMenuForced || draft.hasPrefix("/")
    }

    private var conversationActionMenuInput: String {
        let draft = store.workspace.selectedContinuity.composerDraft
        return conversationActionMenuForced
            && !draft.trimmingCharacters(in: .whitespacesAndNewlines).hasPrefix("/")
            ? "/"
            : draft
    }

    private var conversationActionSuggestions: [ConversationActionDefinition] {
        ConversationActionCatalog.suggestions(for: conversationActionMenuInput)
    }

    private var conversationActionContext: ConversationActionContext {
        ConversationActionContext(
            isResponding: store.isSendingChatMessage,
            missionNeedsIntervention: store.workbench.selectedMission.map {
                missionContinuationAcceptsInput(status: $0.status)
            } ?? false,
            routeCount: store.availableConversationProfiles.count,
            modelCount: store.conversationModels(
                profile: store.selectedConversationProfile
            ).count,
            hasMessages: !(store.chatThread?.messages.isEmpty ?? true)
        )
    }

    private func updateConversationComposerDraft(_ value: String) {
        let previous = store.workspace.selectedContinuity.composerDraft
        store.updateComposerDraft(value)
        guard value != previous else { return }
        conversationActionMenuForced = false
        conversationActionMenuSuppressed = false
        conversationActionPicker = nil
        conversationActionChoiceMenu.clear()
        conversationActionNotice = nil
        conversationActionMenu.synchronize(with: value)
    }

    private func presentConversationCommands() {
        conversationActionMenuForced = true
        conversationActionMenuSuppressed = false
        conversationActionPicker = nil
        conversationActionChoiceMenu.clear()
        conversationActionNotice = nil
        conversationActionMenu.synchronize(with: "/")
        composerFocused = true
    }

    private func dismissConversationActionOverlay() {
        conversationActionMenuForced = false
        conversationActionMenuSuppressed = true
        conversationActionPicker = nil
        conversationActionChoiceMenu.clear()
        conversationActionNotice = nil
        conversationActionMenu.clear()
        composerFocused = true
    }

    @ViewBuilder
    private func conversationActionNoticeView(
        _ notice: ConversationActionNotice
    ) -> some View {
        HStack(alignment: .top, spacing: 9) {
            Image(systemName: notice.systemImage)
                .foregroundStyle(
                    notice.isWarning
                        ? LoomGraphite.statusWarning
                        : LoomGraphite.accent
                )
                .frame(width: 20)
            VStack(alignment: .leading, spacing: 2) {
                Text(notice.title)
                    .font(.callout.weight(.semibold))
                Text(notice.detail)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)
                    .textSelection(.enabled)
            }
            Spacer(minLength: 8)
            Button {
                conversationActionNotice = nil
            } label: {
                Image(systemName: "xmark")
                    .frame(width: 24, height: 24)
            }
            .buttonStyle(.plain)
            .accessibilityLabel("Dismiss action status")
            .help("Dismiss")
        }
        .padding(.horizontal, 10)
        .padding(.vertical, 8)
        .background(
            notice.isWarning
                ? LoomGraphite.statusWarning.opacity(0.09)
                : LoomGraphite.accentMuted,
            in: RoundedRectangle(cornerRadius: 8, style: .continuous)
        )
        .overlay {
            RoundedRectangle(cornerRadius: 8, style: .continuous)
                .stroke(LoomGraphite.separator, lineWidth: 1)
        }
        .accessibilityElement(children: .combine)
        .accessibilityIdentifier("loom.conversation.action-status")
    }

    private func showConversationActionNotice(
        title: String,
        detail: String,
        systemImage: String,
        warning: Bool = false
    ) {
        conversationActionNotice = ConversationActionNotice(
            title: title,
            detail: detail,
            systemImage: systemImage,
            isWarning: warning
        )
    }

    private func conversationActionPickerTitle(
        _ kind: ConversationActionPickerKind
    ) -> String {
        switch kind {
        case .route: return "Choose Route"
        case .model: return "Choose Model"
        case .reasoning: return "Choose Reasoning"
        }
    }

    private func presentConversationActionPicker(
        kind: ConversationActionPickerKind,
        query: String
    ) {
        let picker = ConversationActionPicker(kind: kind, query: query)
        conversationActionPicker = picker
        conversationActionChoiceMenu.synchronize(
            with: conversationActionChoices(for: picker)
        )
    }

    private func conversationActionChoices(
        for picker: ConversationActionPicker
    ) -> [ConversationActionChoice] {
        let choices: [ConversationActionChoice]
        switch picker.kind {
        case .route:
            choices = store.availableConversationProfiles.map { profile in
                ConversationActionChoice(
                    id: profile.profileID,
                    title: conversationProfileMenuLabel(profile),
                    detail: profile.modelID,
                    systemImage: "point.3.connected.trianglepath.dotted",
                    isSelected: profile.profileID
                        == store.selectedConversationProfileID
                )
            }
        case .model:
            let profile = store.selectedConversationProfile
            choices = store.conversationModels(profile: profile).map { model in
                let available = store.isConversationModelAvailable(
                    profile: profile,
                    modelID: model.modelID
                )
                return ConversationActionChoice(
                    id: model.modelID,
                    title: model.displayName,
                    detail: model.reasoningEfforts.isEmpty
                        ? "Provider default reasoning"
                        : "Reasoning: \(model.reasoningEfforts.map(conversationReasoningEffortLabel).joined(separator: ", "))",
                    systemImage: "cpu",
                    isSelected: model.modelID
                        == store.effectiveConversationModelID,
                    unavailableReason: available
                        ? nil
                        : store.conversationModelUnavailableReason(
                            providerID: profile?.providerID ?? "",
                            modelID: model.modelID
                        )
                )
            }
        case .reasoning:
            let models = store.conversationModels(
                profile: store.selectedConversationProfile
            )
            let efforts = models.first(where: {
                $0.modelID == store.effectiveConversationModelID
            })?.reasoningEfforts ?? []
            choices = [
                ConversationActionChoice(
                    id: "provider-default",
                    title: "Provider default",
                    detail: "Use the selected model's default",
                    systemImage: "brain.head.profile",
                    isSelected: store.effectiveConversationReasoningEffort.isEmpty
                ),
            ] + efforts.map { effort in
                ConversationActionChoice(
                    id: effort,
                    title: conversationReasoningEffortLabel(effort),
                    detail: "Reasoning effort for new turns",
                    systemImage: "brain.head.profile",
                    isSelected: effort
                        == store.effectiveConversationReasoningEffort
                )
            }
        }

        let query = picker.query.trimmingCharacters(in: .whitespacesAndNewlines)
            .lowercased()
        guard !query.isEmpty else { return choices }
        return choices.filter {
            $0.title.lowercased().contains(query)
                || $0.detail.lowercased().contains(query)
        }
    }

    private func selectConversationActionChoice(
        _ choice: ConversationActionChoice,
        for kind: ConversationActionPickerKind
    ) {
        switch kind {
        case .route:
            requestConversationProfileSelection(choice.id)
        case .model:
            store.selectConversationModel(choice.id)
        case .reasoning:
            store.selectConversationReasoningEffort(
                choice.id == "provider-default" ? "" : choice.id
            )
        }
        conversationActionPicker = nil
        conversationActionChoiceMenu.clear()
        conversationActionMenuSuppressed = true
        showConversationActionNotice(
            title: kind == .route ? "Route review opened" : "Selection updated",
            detail: choice.title,
            systemImage: choice.systemImage
        )
        composerFocused = true
    }

    @MainActor
    private func executeConfirmedConversationActionProposal(
        _ proposal: LocalProductConversationActionProposal
    ) async {
        guard store.canExecuteConversationActionProposal(proposal) else {
            rejectConfirmedModelTool(
                title: "This approval is no longer current",
                detail: "The Conversation Route, model, workspace authority or target changed. Ask Loom to prepare a new proposal.",
            )
            return
        }
        if let request = proposal.conversationActionRequest {
            modelTeamEditInstruction = ""
            executeConversationAction(request)
            return
        }

        switch proposal.action {
        case .continueMission:
            guard let missionID = proposal.payload?.missionID,
                  store.snapshot?.missions.contains(where: {
                      $0.missionID == missionID
                  }) == true,
                  await openMissionContext(missionID) else {
                rejectConfirmedModelTool(
                    title: "Mission is no longer available",
                    detail: "Refresh Missions, then ask Loom to prepare the guidance again."
                )
                return
            }
            store.updateMissionComposerDraft(proposal.argument)
            showConversationActionNotice(
                title: "Mission guidance ready for review",
                detail: "Review the guidance before Loom creates a new audited Attempt.",
                systemImage: "arrow.clockwise"
            )

        case .roundTable:
            guard let missionID = proposal.payload?.missionID,
                  let mission = store.snapshot?.missions.first(where: {
                      $0.missionID == missionID
                  }), await openMissionContext(missionID) else {
                rejectConfirmedModelTool(
                    title: "Mission is no longer available",
                    detail: "Refresh Missions, then ask Loom to open its RoundTable again."
                )
                return
            }
            pendingRoundtableMissionLink = roundtableLink(for: mission)
            selectNavigation(.roundtable)
            showConversationActionNotice(
                title: "RoundTable setup opened",
                detail: "Review the exact Mission-linked seats before deliberation starts.",
                systemImage: "bubble.left.and.bubble.right"
            )

        case .route:
            guard let profileID = proposal.payload?.profileID,
                  store.availableConversationProfiles.contains(where: {
                      $0.profileID == profileID
                  }) else {
                rejectConfirmedModelTool(
                    title: "Route is no longer available",
                    detail: "Refresh Runtime & Providers, then ask Loom to choose again."
                )
                return
            }
            let priorProfileID = store.selectedConversationProfileID
            requestConversationProfileSelection(profileID)
            if pendingConversationRouteTransition != nil {
                showConversationActionNotice(
                    title: "Route disclosure review opened",
                    detail: "Review the Context and trust-domain change before the new Segment starts.",
                    systemImage: "checkmark.shield"
                )
            } else if store.selectedConversationProfileID == profileID {
                showConversationActionNotice(
                    title: priorProfileID == profileID
                        ? "Route already selected" : "Route updated",
                    detail: profileID,
                    systemImage: "point.3.connected.trianglepath.dotted"
                )
            } else {
                rejectConfirmedModelTool(
                    title: "Route change could not be applied",
                    detail: "The Route changed after this proposal was created. Ask Loom to choose again."
                )
            }

        case .model:
            guard let modelID = proposal.payload?.modelID,
                  store.conversationModels(
                      profile: store.selectedConversationProfile
                  ).contains(where: { $0.modelID == modelID }) else {
                rejectConfirmedModelTool(
                    title: "Model is no longer on this Route",
                    detail: "Choose another model or ask Loom to refresh the current Route."
                )
                return
            }
            store.selectConversationModel(modelID)
            if store.effectiveConversationModelID == modelID {
                showConversationActionNotice(
                    title: "Model updated",
                    detail: modelID,
                    systemImage: "cpu"
                )
            } else {
                rejectConfirmedModelTool(
                    title: "Model could not be applied",
                    detail: store.conversationModelSelectionNotice
                        ?? "The owning Provider Account is not ready."
                )
            }

        case .reasoning:
            guard let requested = proposal.payload?.reasoningEffort else {
                rejectConfirmedModelTool(
                    title: "Reasoning choice is incomplete",
                    detail: "Ask Loom to choose the reasoning effort again."
                )
                return
            }
            let supported = store.conversationModels(
                profile: store.selectedConversationProfile
            ).first(where: {
                $0.modelID == store.effectiveConversationModelID
            })?.reasoningEfforts ?? []
            guard requested == "provider-default" || supported.contains(requested) else {
                rejectConfirmedModelTool(
                    title: "Reasoning choice is no longer available",
                    detail: "The selected model's supported reasoning levels changed."
                )
                return
            }
            store.selectConversationReasoningEffort(
                requested == "provider-default" ? "" : requested
            )
            showConversationActionNotice(
                title: "Reasoning updated",
                detail: requested == "provider-default"
                    ? "Provider default" : conversationReasoningEffortLabel(requested),
                systemImage: "brain.head.profile"
            )

        case .workspace:
            showFolderImporter = true
            showConversationActionNotice(
                title: "Choose a workspace",
                detail: "Only the folder you choose in the system picker is applied. The model never receives its path.",
                systemImage: "folder"
            )

        case .teamEdit:
            guard let payload = proposal.payload,
                  let team = store.snapshot?.teams.first(where: {
                      $0.teamInstanceID == payload.teamInstanceID
                  }),
                  let savedTeam = store.setupSnapshot?.savedTeams.first(where: {
                      $0.id == team.teamDefinitionID
                          && $0.version == team.teamDefinitionVersion
                          && $0.status == "active"
                  }) else {
                rejectConfirmedModelTool(
                    title: "Agent Team is no longer editable",
                    detail: "Open Agent Teams to choose a current saved Team."
                )
                return
            }
            store.selectTeam(team)
            selectedNavigation = .teams
            governance.open(.team)
            builderAnswer = ""
            builderName = ""
            builderPurpose = ""
            modelTeamEditInstruction = payload.instruction
            await store.startBuilder(from: savedTeam)
            if store.builderSession != nil {
                showConversationActionNotice(
                    title: "Agent Team draft opened",
                    detail: "Review the requested change and every frozen Runtime binding before confirmation.",
                    systemImage: "person.3.sequence"
                )
            } else {
                rejectConfirmedModelTool(
                    title: "Agent Team draft could not be opened",
                    detail: "Refresh Agent Teams and retry this proposal."
                )
            }

        case .roundTablePause, .roundTableSteer, .roundTableRetry,
                .roundTableSkip, .roundTableReplace:
            await executeConfirmedRoundTableAction(proposal)

        case .mission, .team:
            return
        }
        composerFocused = true
    }

    @MainActor
    private func executeConfirmedRoundTableAction(
        _ proposal: LocalProductConversationActionProposal
    ) async {
        guard let payload = proposal.payload, !roundtableContextBusy else {
            rejectConfirmedModelTool(
                title: "RoundTable is busy",
                detail: "Wait for the current intervention to finish, then retry."
            )
            return
        }
        roundtableContextBusy = true
        defer { roundtableContextBusy = false }
        let interventionID = "control-\(proposal.proposalID)"

        let requiresSeatTarget = proposal.action != .roundTablePause
        let requiresAttempt = proposal.action == .roundTableSteer
            || proposal.action == .roundTableRetry
        var frozenTargetView: LocalRoundtableView?
        if requiresSeatTarget {
            guard payload.hasFrozenRoundTableSeatBinding else {
                rejectConfirmedModelTool(
                    title: "RoundTable review is outdated",
                    detail: "Ask Loom to prepare this intervention again so the exact Agent binding can be verified."
                )
                return
            }
            guard let current = await store.roundtableLoadSession(
                sessionID: payload.sessionID
            ), payload.matchesFrozenRoundTableTarget(
                in: current,
                requiresAttempt: requiresAttempt
            ) else {
                rejectConfirmedModelTool(
                    title: "RoundTable target changed",
                    detail: store.roundtableError
                        ?? "The Agent binding changed after review. Ask Loom to refresh the intervention."
                )
                return
            }
            frozenTargetView = current
        }

        let updated: LocalRoundtableView?
        switch proposal.action {
        case .roundTablePause:
            updated = await store.roundtablePauseRound(
                sessionID: payload.sessionID,
                roundID: payload.roundID,
                interventionID: interventionID
            )
        case .roundTableSteer:
            updated = await store.roundtableSteerSeat(
                sessionID: payload.sessionID,
                roundID: payload.roundID,
                seatID: payload.seatID,
                attemptID: payload.attemptID,
                guidance: payload.guidance,
                interventionID: interventionID
            )
        case .roundTableRetry:
            updated = await store.roundtableRetrySeat(
                sessionID: payload.sessionID,
                roundID: payload.roundID,
                seatID: payload.seatID,
                attemptID: payload.attemptID,
                guidance: payload.guidance,
                expectedMembershipRevision: payload.membershipRevision,
                expectedSeatBindingDigest: payload.seatBindingDigest,
                interventionID: interventionID
            )
        case .roundTableSkip:
            updated = await store.roundtableSkipSeat(
                sessionID: payload.sessionID,
                roundID: payload.roundID,
                seatID: payload.seatID,
                expectedMembershipRevision: payload.membershipRevision,
                expectedSeatBindingDigest: payload.seatBindingDigest,
                interventionID: interventionID
            )
        case .roundTableReplace:
            updated = frozenTargetView
        case .mission, .continueMission, .team, .roundTable, .route,
                .model, .reasoning, .workspace, .teamEdit:
            updated = nil
        }

        guard let updated,
              updated.session.id == payload.sessionID,
              updated.rounds.contains(where: { $0.id == payload.roundID }),
              proposal.action == .roundTablePause
                || updated.seats[payload.seatID] != nil else {
            rejectConfirmedModelTool(
                title: "RoundTable target changed",
                detail: store.roundtableError
                    ?? "The Session, round, seat or Attempt is no longer current. Ask Loom to refresh it."
            )
            return
        }

        guard await presentConfirmedRoundTable(updated) else { return }
        if proposal.action == .roundTableReplace {
            fullGovernancePresentation = .roundtable
            showConversationActionNotice(
                title: "Replacement review opened",
                detail: "Choose the replacement for \(payload.seatID). Loom will freeze its Route before execution.",
                systemImage: "person.crop.circle.badge.arrow.trianglehead.counterclockwise"
            )
        } else {
            showConversationActionNotice(
                title: "RoundTable updated",
                detail: proposal.targetSummary,
                systemImage: "person.2.wave.2"
            )
        }
    }

    @MainActor
    private func presentConfirmedRoundTable(
        _ view: LocalRoundtableView
    ) async -> Bool {
        guard let missionID = view.session.context?.missionID,
              let mission = store.snapshot?.missions.first(where: {
                  $0.missionID == missionID
              }), await openMissionContext(missionID) else {
            rejectConfirmedModelTool(
                title: "Linked Mission is unavailable",
                detail: "Open the RoundTable from its Mission and retry."
            )
            return false
        }
        pendingRoundtableMissionLink = roundtableLink(for: mission)
        roundtableContextView = view
        roundtableContextError = nil
        selectedNavigation = .roundtable
        governance.open(.roundtable)
        return true
    }

    private func rejectConfirmedModelTool(title: String, detail: String) {
        showConversationActionNotice(
            title: title,
            detail: detail,
            systemImage: "exclamationmark.triangle",
            warning: true
        )
    }

    private func executeConversationAction(
        _ request: ConversationActionRequest
    ) {
        let availability = ConversationActionCatalog.availability(
            of: request.action,
            in: conversationActionContext
        )
        if case .unavailable(let reason) = availability {
            showConversationActionNotice(
                title: ConversationActionCatalog.definition(for: request.action).title,
                detail: reason,
                systemImage: "exclamationmark.triangle",
                warning: true
            )
            return
        }

        let currentDraft = store.workspace.selectedContinuity.composerDraft
        let commandWasTyped = currentDraft.trimmingCharacters(
            in: .whitespacesAndNewlines
        ).hasPrefix("/")
        if commandWasTyped
            || request.source != .commandMenu
            || !request.argument.isEmpty {
            store.updateComposerDraft("")
        }
        conversationActionMenu.clear()
        conversationActionMenuForced = false
        conversationActionMenuSuppressed = true
        conversationActionPicker = nil
        conversationActionChoiceMenu.clear()
        conversationActionNotice = nil

        switch request.action {
        case .help:
            presentConversationCommands()
        case .newTask:
            startNewTask()
        case .mission:
            pendingMissionObjective = request.argument.isEmpty
                ? composerNewMissionObjective(thread: store.chatThread)
                : request.argument
            pendingMissionTeamID = ""
            store.showMissionBoard()
            fullGovernancePresentation = .newMission
        case .missions:
            selectNavigation(.work)
        case .team:
            builderName = ""
            builderPurpose = request.argument
            modelTeamEditInstruction = ""
            governance.open(.team)
            Task { await store.startBlankBuilder() }
        case .roundTable:
            selectNavigation(.roundtable)
        case .continueMission:
            if !request.argument.isEmpty {
                store.updateMissionComposerDraft(request.argument)
            }
            selectedNavigation = .work
            governance.open(.mission)
        case .route:
            presentConversationActionPicker(kind: .route, query: request.argument)
        case .model:
            presentConversationActionPicker(kind: .model, query: request.argument)
        case .reasoning:
            presentConversationActionPicker(kind: .reasoning, query: request.argument)
        case .status:
            showConversationStatus()
        case .diagnostics:
            prepareChatDiagnosticPreview()
        case .stop:
            pendingStopAction = request
        case .needsYou:
            selectNavigation(.attention)
        case .library:
            selectNavigation(.library)
        case .runtimeProviders:
            fullGovernancePresentation = .runtimeProviders
        case .folder:
            showFolderImporter = true
        case .copyConversation:
            copyConversationTranscript()
            showConversationActionNotice(
                title: "Conversation copied",
                detail: "The visible transcript is on the clipboard.",
                systemImage: "doc.on.doc"
            )
        }
        composerFocused = true
    }

    private func showConversationStatus() {
        let route = store.selectedConversationProfile
            .map(conversationProfileMenuLabel) ?? "No Route"
        let model = conversationModelDisplayName(
            profile: store.selectedConversationProfile,
            modelID: store.effectiveConversationModelID
        )
        let response = store.isSendingChatMessage ? "Responding" : "Ready"
        let mission = store.workbench.selectedMission.map {
            "Mission: \($0.title) · \(missionHumanStatus($0.status))"
        }
        showConversationActionNotice(
            title: response,
            detail: ([route, model, mission].compactMap { $0 }).joined(separator: " · "),
            systemImage: "waveform.path.ecg"
        )
    }

    private func sendConversationDraft() {
        let draft = store.workspace.selectedContinuity.composerDraft
        let trimmed = draft.trimmingCharacters(in: .whitespacesAndNewlines)

        if conversationActionMenuForced,
           let selectedID = conversationActionMenu.selectedID {
            let definition = ConversationActionCatalog.definition(for: selectedID)
            let resolution = ConversationActionRouter.commandMenuResolution(
                action: selectedID,
                input: "/\(definition.command) \(draft)"
            )
            if case .request(let request) = resolution {
                executeConversationAction(request)
            } else if case .rejectedActionArgument = resolution {
                showConversationActionNotice(
                    title: "Credential text rejected",
                    detail: "Credentials can only be managed in Runtime & Providers.",
                    systemImage: "key.slash",
                    warning: true
                )
            }
            return
        }

        if trimmed.hasPrefix("/") {
            conversationActionMenu.synchronize(with: draft)
            switch conversationActionMenu.resolution(for: draft) {
            case .request(let request):
                executeConversationAction(request)
            case .rejectedActionArgument:
                showConversationActionNotice(
                    title: "Credential text rejected",
                    detail: "Credentials can only be managed in Runtime & Providers.",
                    systemImage: "key.slash",
                    warning: true
                )
            case .chat, .unknownSlashCommand:
                let token = trimmed.dropFirst().split(whereSeparator: { $0.isWhitespace })
                    .first.map(String.init) ?? ""
                showConversationActionNotice(
                    title: "Command not found",
                    detail: token.isEmpty
                        ? "No command is selected."
                        : "/\(String(token.prefix(64))) is not available.",
                    systemImage: "questionmark.circle",
                    warning: true
                )
            }
            return
        }

        switch ConversationActionRouter.resolve(draft) {
        case .request(let request):
            executeConversationAction(request)
            return
        case .unknownSlashCommand(let command):
            showConversationActionNotice(
                title: "Command not found",
                detail: "/\(String(command.prefix(64))) is not available.",
                systemImage: "questionmark.circle",
                warning: true
            )
            return
        case .rejectedActionArgument:
            showConversationActionNotice(
                title: "Credential text rejected",
                detail: "Credentials can only be managed in Runtime & Providers.",
                systemImage: "key.slash",
                warning: true
            )
            return
        case .chat:
            break
        }

        if let transition = store.requestConversationDispatchTransition() {
            pendingConversationRouteTransition = transition
            return
        }
        Task {
            await store.sendChatMessage(draft)
        }
    }


    private func conversationRouteLabel(
        for message: LocalProductChatMessage
    ) -> String? {
        guard !message.segmentID.isEmpty,
              let segment = store.chatThread?.segments.first(where: {
                  $0.segmentID == message.segmentID
              }),
              let profile = store.availableConversationProfiles.first(where: {
                  $0.profileID == segment.profileID
              }) else {
            return nil
        }
        let harness: String
        switch profile.harnessAdapter {
        case "codex": harness = "Codex"
        case "claude-code": harness = "Claude Code"
        case "loom-native": harness = "Loom Native"
        case "pi": harness = "Pi"
        default: harness = profile.harnessAdapter
        }
        let account = profile.providerAccountID.isEmpty
            ? profile.providerID
            : profile.providerAccountID
        let actualModel = conversationAttemptModelLabel(for: message)
        // Prefer the model and reasoning effort actually used for this reply
        // (per-attempt) over the profile default so the label never shows a
        // stale "aligned" model after a mid-conversation switch.
        let model = actualModel ?? profile.modelID
        return [harness, account, model]
            .filter { !$0.isEmpty }
            .joined(separator: " · ")
    }

    /// Matches a non-user reply to the dispatched user turn it answers. Each
    /// immutable Segment has its own ordered Attempt sequence.
    private func conversationAttempt(
        for message: LocalProductChatMessage
    ) -> LocalProductConversationAttempt? {
        guard let thread = store.chatThread else {
            return nil
        }
        return conversationAttemptForMessage(message, in: thread)
    }

    /// Returns "model · reasoning-effort" for the exact Attempt that produced
    /// this reply.
    private func conversationAttemptModelLabel(
        for message: LocalProductChatMessage
    ) -> String? {
        guard let attempt = conversationAttempt(for: message) else {
            return nil
        }
        let model = (attempt.modelID ?? "").trimmingCharacters(in: .whitespaces)
        let effort = (attempt.reasoningEffort ?? "").trimmingCharacters(in: .whitespaces)
        if model.isEmpty && effort.isEmpty {
            return nil
        }
        return [model, effort]
            .filter { !$0.isEmpty }
            .joined(separator: " · ")
    }

    private func conversationDisclosureSummary(
        for message: LocalProductChatMessage
    ) -> LocalProductConversationSegment? {
        guard !message.segmentID.isEmpty,
              let thread = store.chatThread,
              thread.messages.first(where: {
                  $0.segmentID == message.segmentID
              })?.messageID == message.messageID,
              let segment = thread.segments.first(where: {
                  $0.segmentID == message.segmentID
              }),
              !segment.disclosureReceiptDigest.isEmpty else {
            return nil
        }
        return segment
    }

    private func conversationContextMeterLabel(
        _ disclosure: LocalProductConversationSegment
    ) -> String {
        guard let capacityStatus = disclosure.contextCapacityStatus else {
            guard disclosure.contextTokenBudget > 0 else {
                return "Context shared \(disclosure.disclosedContextCount) · \(disclosure.omittedContextCount) omitted"
            }
            return "Policy context \(disclosure.contextTokenCount) / \(disclosure.contextTokenBudget) tokens · \(disclosure.omittedContextCount) omitted"
        }
        guard capacityStatus != .unavailable else {
            return "Capacity unavailable · policy budget \(disclosure.contextTokenBudget) tokens · \(disclosure.omittedContextCount) omitted"
        }
        guard disclosure.admittedInputBudgetTokens > 0 else {
            return "Context shared \(disclosure.disclosedContextCount) · \(disclosure.omittedContextCount) omitted"
        }
        return "\(conversationContextCapacityStatusLabel(capacityStatus)) \(disclosure.contextTokenCount) / \(disclosure.admittedInputBudgetTokens) input tokens · \(disclosure.omittedContextCount) omitted"
    }

    @ViewBuilder
    private func conversationContextInspection(
        _ disclosure: LocalProductContextDisclosure
    ) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("Shared categories")
                .font(.caption.weight(.semibold))
            ForEach(Array(disclosure.disclosed.enumerated()), id: \.offset) {
                _, disclosureItem in
                conversationContextInspectionRow(disclosureItem)
            }
            if !disclosure.omitted.isEmpty {
                Text("Omissions")
                    .font(.caption.weight(.semibold))
                    .padding(.top, 2)
                ForEach(Array(disclosure.omitted.enumerated()), id: \.offset) {
                    _, disclosureItem in
                    conversationContextInspectionRow(disclosureItem)
                }
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel(
            "Context disclosure: \(disclosure.disclosed.count) shared, \(disclosure.omitted.count) omitted"
        )
    }

    private func conversationContextInspectionRow(
        _ disclosureItem: LocalProductContextDisclosureItem
    ) -> some View {
        VStack(alignment: .leading, spacing: 2) {
            Label(
                conversationPolicyLabel(disclosureItem.kind),
                systemImage: disclosureItem.omissionReason.isEmpty
                    ? "checkmark.circle" : "minus.circle"
            )
            .font(.caption)
            Text(
                conversationContextInspectionDetail(disclosureItem)
            )
            .font(.caption2)
            .foregroundStyle(.secondary)
        }
        .accessibilityElement(children: .combine)
    }

    private func conversationContextInspectionDetail(
        _ disclosureItem: LocalProductContextDisclosureItem
    ) -> String {
        var details = [
            conversationPolicyLabel(disclosureItem.trust),
            conversationPolicyLabel(disclosureItem.scope),
            "\(disclosureItem.tokenCount) estimated tokens",
        ]
        if !disclosureItem.omissionReason.isEmpty {
            details.append(conversationPolicyLabel(disclosureItem.omissionReason))
            details.append(
                disclosureItem.retrievable ? "Agent can retrieve by scope" : "Not retrievable"
            )
        }
        return details.joined(separator: " · ")
    }

    private func conversationPolicyLabel(_ value: String) -> String {
        value.split(separator: "_")
            .map { $0.capitalized }
            .joined(separator: " ")
    }

    private func chatFailureBanner(
        _ failure: LocalProductChatOperationFailure
    ) -> some View {
        ConversationChatFailureBanner(
            failure: failure,
            vaultRecoveryAvailable: conversationVaultRecoveryAvailable(
                stage: failure.stage,
                recoverable: failure.recoverable
            ),
            routeRecoveryAvailable: failure.isRouteRecoveryAvailable,
            isSending: store.isSendingChatMessage,
            isUpdatingVault: store.isUpdatingCredentialVaultLock,
            onPrimaryRecovery: {
                if failure.isProposalDecisionRecoveryAvailable {
                    Task { await store.loadChatThread() }
                } else if failure.recoverable {
                    sendConversationDraft()
                }
            },
            onUnlockVault: {
                Task {
                    await store.setCredentialVaultLocked(false)
                    await store.loadChatThread()
                }
            },
            onOpenVault: {
                fullGovernancePresentation = .runtimeProviders
            },
            onSwitchProvider: {
                fullGovernancePresentation = .runtimeProviders
            },
            onDiagnostics: prepareChatDiagnosticPreview,
            onCopyIncident: {
                NSPasteboard.general.clearContents()
                NSPasteboard.general.setString(
                    failure.incidentID,
                    forType: .string
                )
            }
        )
    }

    private func prepareChatDiagnosticPreview() {
        let input = LocalDiagnosticBundleInput(
            setupSnapshot: store.setupSnapshot,
            board: store.timeline?.board
        )
        Task {
            do {
                let exporter = try LocalDiagnosticBundleExporter.installed()
                let preview = try await Task.detached {
                    try exporter.preview(input: input)
                }.value
                diagnosticPresentation = ChatDiagnosticPresentation(
                    preview: preview,
                    exporter: exporter
                )
                diagnosticPreparationError = nil
            } catch {
                diagnosticPreparationError =
                    "Loom could not prepare a privacy-safe diagnostic preview. Open Runtime & Providers to retry from Diagnostics."
            }
        }
    }

    private var governanceInspector: some View {
        VStack(spacing: 0) {
            governanceHeader
            Divider()
            ScrollView {
                governanceContent
                    .padding(16)
            }
            if governance.destination == .team, store.builderSession != nil {
                Divider()
                teamBuilderConfirmFooter
            }
        }
        .background(LoomGraphite.surface)
        .accessibilityElement(children: .contain)
        .accessibilityLabel("\(governance.destination.rawValue) governance inspector")
    }

    private var governanceHeader: some View {
        HStack(spacing: 8) {
            Menu {
                if case .mission = store.workbench.route {
                    Section("Current Mission") {
                        ForEach(
                            LoomGovernanceDestination.missionContext
                        ) { destination in
                            governanceDestinationButton(destination)
                        }
                    }
                }
                Section("Workspace") {
                    ForEach(
                        LoomGovernanceDestination.workspace
                    ) { destination in
                        governanceDestinationButton(destination)
                    }
                }
                Section("More") {
                    ForEach(
                        caseMissionRoute
                            ? [.overview]
                            : LoomGovernanceDestination.secondary
                    ) { destination in
                        governanceDestinationButton(destination)
                    }
                }
            } label: {
                Label(
                    governanceHeaderTitle,
                    systemImage: governanceSymbol(governance.destination)
                )
                .font(.headline)
                .lineLimit(1)
            }
            .menuStyle(.borderlessButton)
            .accessibilityLabel("Choose governance view")

            Spacer()

            Button {
                governance.togglePin()
            } label: {
                Image(systemName: governance.mode == .pinned ? "pin.fill" : "pin")
                    .frame(width: 32, height: 32)
            }
            .buttonStyle(.plain)
            .accessibilityLabel(governance.mode == .pinned ? "Unpin inspector" : "Pin inspector")
            .help(governance.mode == .pinned ? "Unpin inspector" : "Pin inspector")

            Button {
                governance.close()
            } label: {
                Image(systemName: "xmark")
                    .frame(width: 32, height: 32)
            }
            .buttonStyle(.plain)
            .accessibilityLabel("Close governance inspector")
            .help("Close")
        }
        .padding(.horizontal, 14)
        .frame(height: 52)
    }

    private var caseMissionRoute: Bool {
        if case .mission = store.workbench.route { return true }
        return false
    }

    private func governanceDestinationButton(
        _ destination: LoomGovernanceDestination
    ) -> some View {
        Button {
            governance.open(destination)
        } label: {
            Label(
                destination.rawValue,
                systemImage: governanceSymbol(destination)
            )
        }
    }

    private var governanceHeaderTitle: String {
        guard governance.destination == .mission,
              case .mission(let missionID) = store.workbench.route,
              let mission = store.snapshot?.missions.first(where: {
                  $0.missionID == missionID
              })
        else { return governance.destination.rawValue }
        return presentedMissionTitle(mission, limit: 48)
    }

    private func presentedMissionTitle(
        _ mission: LocalProductMissionSummary,
        limit: Int = 96
    ) -> String {
        SafeText.sanitize(
            missionPresentationTitle(
                presentationTitle: store.missionPresentations[
                    mission.missionID
                ]?.title,
                candidate: mission.title,
                missionID: mission.missionID,
                teamInstanceID: mission.teamInstanceID,
                teams: store.snapshot?.teams ?? []
            ),
            limit: limit
        )
    }

    @ViewBuilder
    private var governanceContent: some View {
        switch governance.destination {
        case .overview:
            overviewPanel
        case .board:
            boardPanel
        case .mission:
            missionContextPanel
        case .roundtable:
            roundtableContextPanel
        case .team:
            teamPanel
        case .topology:
            topologyPanel
        case .timeline:
            timelinePanel
        case .decisions:
            decisionsPanel
        case .evidence:
            evidencePanel
        case .runtimes:
            runtimePanel
        case .attention:
            attentionPanel
        case .library:
            libraryPanel
        }
    }

    private var overviewPanel: some View {
        VStack(alignment: .leading, spacing: 0) {
            metricRow("Active missions", value: "\(activeMissionCount)", symbol: "bolt")
            metricRow("Needs you", value: "\(activeAttentionCount)", symbol: "exclamationmark.bubble")
            metricRow("Teams", value: "\(store.snapshot?.teams.count ?? 0)", symbol: "person.3")
            metricRow("Accepted evidence", value: "\(store.snapshot?.evidence.count ?? 0)", symbol: "checkmark.seal")
        }
    }

    private var boardPanel: some View {
        let linkedMissionIDs = store.missionIDsLinkedToConversation(
            store.currentChatThreadID()
        )
        let linkedMissionIDSet = Set(linkedMissionIDs)
        let linkedMissions = (store.snapshot?.missions ?? []).filter {
            linkedMissionIDSet.contains($0.missionID)
        }.sorted {
            let lhs = linkedMissionIDs.firstIndex(of: $0.missionID) ?? .max
            let rhs = linkedMissionIDs.firstIndex(of: $1.missionID) ?? .max
            return lhs < rhs
        }
        return VStack(alignment: .leading, spacing: 12) {
            HStack {
                Text("This conversation")
                    .font(.subheadline.weight(.semibold))
                Spacer()
                Text("\(linkedMissions.count)")
                    .font(.caption.monospacedDigit())
                    .foregroundStyle(.secondary)
            }
            if linkedMissions.isEmpty {
                emptyPanelState(
                    "No Missions linked to this conversation",
                    symbol: "bubble.left.and.exclamationmark.bubble.right"
                )
            } else {
                ForEach(linkedMissions.prefix(8)) { mission in
                    let title = presentedMissionTitle(mission)
                    Button {
                        Task {
                            await openMissionContext(mission)
                        }
                    } label: {
                        inspectorRow(
                            title: title,
                            detail: missionBoardDetailText(
                                lane: mission.lane,
                                status: mission.status
                            ),
                            symbol: mission.attentionCount > 0
                                ? "exclamationmark.circle.fill"
                                : "circle.dashed"
                        )
                    }
                    .buttonStyle(.plain)
                    .loomActionTarget()
                    .accessibilityLabel("Open Mission \(title)")
                    .help("Open the current Mission timeline")
                }
            }
            Divider()
            Button {
                store.showMissionBoard()
                fullGovernancePresentation = .workbench
                governance.open(.board)
            } label: {
                Label("All Missions", systemImage: "list.bullet")
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
            .buttonStyle(.bordered)
            .loomActionTarget()
        }
    }

    @ViewBuilder
    private var missionContextPanel: some View {
        if case .mission(let missionID) = store.workbench.route,
           let mission = store.snapshot?.missions.first(where: {
               $0.missionID == missionID
           }) {
            let entries = missionActivityEntries(
                mission: mission,
                timeline: store.timeline
            )
            VStack(alignment: .leading, spacing: 14) {
                missionContextSummary(mission)

                Divider()

                missionContextActivity(entries, mission: mission)

                if missionContinuationAcceptsInput(status: mission.status) {
                    Divider()
                    missionContextIntervention(mission)
                }

                Divider()

                HStack(spacing: 8) {
                    if Set(mission.topology.map(\.logicalNodeID)).count >= 2 {
                        Button {
                            startRoundTableFromMission(mission)
                        } label: {
                            Label(
                                "RoundTable",
                                systemImage: "person.2.wave.2"
                            )
                        }
                        .buttonStyle(.bordered)
                        .help(
                            "Discuss this Mission with its frozen Agent Team"
                        )
                    } else {
                        Text("RoundTable needs at least two Agent roles")
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                            .fixedSize(horizontal: false, vertical: true)
                    }

                    Spacer(minLength: 8)

                    Button {
                        fullGovernancePresentation = .workbench
                    } label: {
                        Label("Details", systemImage: "arrow.up.right.square")
                    }
                    .buttonStyle(.plain)
                    .accessibilityLabel("Open full Mission details")
                    .help("Open full Mission details")
                }
            }
            .task(id: "\(missionID):\(mission.status)") {
                guard missionActivityNeedsRefresh(status: mission.status) else {
                    return
                }
                await store.followVisibleMissionActivity(missionID)
            }
        } else {
            VStack(alignment: .leading, spacing: 12) {
                emptyPanelState(
                    "Choose a Mission linked to this conversation",
                    symbol: "flag.checkered"
                )
                Button {
                    governance.open(.board)
                } label: {
                    Label("Show Missions", systemImage: "list.bullet")
                }
                .buttonStyle(.bordered)
            }
        }
    }

    private func missionContextSummary(
        _ mission: LocalProductMissionSummary
    ) -> some View {
        let title = presentedMissionTitle(mission)
        let reason = (mission.blockReason ?? "").trimmingCharacters(
            in: .whitespacesAndNewlines
        )
        return VStack(alignment: .leading, spacing: 9) {
            Text(title)
                .font(.headline)
                .fixedSize(horizontal: false, vertical: true)

            Label(
                missionHumanStatus(mission.status),
                systemImage: missionContextStatusSymbol(mission.status)
            )
            .font(.caption.weight(.medium))
            .foregroundStyle(missionContextStatusColor(mission.status))

            ProgressView(
                value: Double(min(mission.completedNodeCount, mission.nodeCount)),
                total: Double(max(mission.nodeCount, 1))
            )
            .accessibilityLabel(
                "\(mission.completedNodeCount) of \(mission.nodeCount) Mission steps complete"
            )

            Text(
                missionPlanSummary(
                    nodeCount: mission.nodeCount,
                    completedNodeCount: mission.completedNodeCount,
                    reviewNodeCount: mission.reviewNodeCount
                )
            )
            .font(.caption)
            .foregroundStyle(.secondary)

            if !mission.lastMilestone.isEmpty {
                Text(mission.lastMilestone)
                    .font(.callout)
                    .textSelection(.enabled)
                    .fixedSize(horizontal: false, vertical: true)
            }

            if missionContinuationAcceptsInput(status: mission.status),
               !reason.isEmpty {
                Label(
                    reason,
                    systemImage: "exclamationmark.triangle"
                )
                .font(.caption.weight(.medium))
                .foregroundStyle(LoomGraphite.statusWarning)
                .textSelection(.enabled)
                .fixedSize(horizontal: false, vertical: true)
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel(
            "Mission \(title), \(missionHumanStatus(mission.status))"
        )
    }

    private func missionContextActivity(
        _ entries: [MissionActivityEntry],
        mission: LocalProductMissionSummary
    ) -> some View {
        let isLive = missionActivityNeedsRefresh(status: mission.status)
        return VStack(alignment: .leading, spacing: 10) {
            HStack(spacing: 8) {
                Label(
                    "Agent conversation",
                    systemImage: "bubble.left.and.text.bubble.right"
                )
                .font(.subheadline.weight(.semibold))
                Spacer()
                if isLive {
                    ProgressView()
                        .controlSize(.small)
                    Text("Live")
                        .font(.caption.weight(.medium))
                        .foregroundStyle(LoomGraphite.accent)
                }
            }

            if entries.isEmpty {
                Text(
                    mission.activeNodeCount > 0
                        ? "The Agent Team is starting its first step."
                        : "No Agent activity is available yet."
                )
                .font(.callout)
                .foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)
            } else {
                ForEach(Array(entries.suffix(8).enumerated()), id: \.element.id) {
                    index, entry in
                    if index > 0 { Divider() }
                    VStack(alignment: .leading, spacing: 6) {
                        HStack(alignment: .firstTextBaseline, spacing: 7) {
                            Image(systemName: "person.crop.circle")
                                .foregroundStyle(LoomGraphite.accent)
                            VStack(alignment: .leading, spacing: 2) {
                                Text(entry.title)
                                    .font(.callout.weight(.semibold))
                                if !entry.route.isEmpty {
                                    Text(entry.route)
                                        .font(.caption2)
                                        .foregroundStyle(.secondary)
                                        .lineLimit(2)
                                }
                            }
                            Spacer(minLength: 6)
                            Text(missionHumanStatus(entry.status))
                                .font(.caption2.weight(.medium))
                                .foregroundStyle(
                                    missionContextStatusColor(entry.status)
                                )
                        }

                        if entry.text.isEmpty {
                            Text("Waiting for this Agent's first visible update.")
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        } else {
                            Text(
                                missionActivityVisibleText(
                                    entry.text,
                                    expanded: expandedMissionActivityIDs.contains(
                                        entry.id
                                    ),
                                    limit: 900
                                )
                            )
                            .font(.callout)
                            .lineSpacing(2)
                            .textSelection(.enabled)
                            .fixedSize(horizontal: false, vertical: true)

                            if entry.text.count > 900 {
                                Button(
                                    expandedMissionActivityIDs.contains(entry.id)
                                        ? "Show less" : "Show full update"
                                ) {
                                    if expandedMissionActivityIDs.contains(entry.id) {
                                        expandedMissionActivityIDs.remove(entry.id)
                                    } else {
                                        expandedMissionActivityIDs.insert(entry.id)
                                    }
                                }
                                .buttonStyle(.borderless)
                                .font(.caption)
                            }
                        }

                        if let node = store.timeline?.board.nodes.first(
                            where: {
                                $0.logicalNodeID == entry.logicalNodeID
                                    && ($0.currentAttempt == entry.attemptNumber
                                        || entry.attemptNumber == 0)
                            }
                        ), node.failureDiagnosticAvailable {
                            missionContextAgentFailure(node)
                        }
                    }
                    .padding(.vertical, 3)
                }
            }

            if let authorityMessage = missionActivityAuthorityMessage(entries) {
                Label(
                    authorityMessage,
                    systemImage: entries.contains(where: {
                        $0.isTentative && !$0.text.isEmpty
                    }) ? "clock.badge.questionmark" : "checkmark.seal.fill"
                )
                .font(.caption2)
                .foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)
            }
        }
    }

    private func missionContextAgentFailure(
        _ node: LocalProductNode
    ) -> some View {
        VStack(alignment: .leading, spacing: 7) {
            Label(
                [node.failureCode, node.failureStage]
                    .filter { !$0.isEmpty }
                    .map { missionHumanStatus($0) }
                    .joined(separator: " · "),
                systemImage: "exclamationmark.triangle"
            )
            .font(.caption.weight(.medium))
            .foregroundStyle(LoomGraphite.statusDanger)

            if !node.terminalReason.isEmpty {
                Text(node.terminalReason)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .textSelection(.enabled)
                    .fixedSize(horizontal: false, vertical: true)
            }

            HStack(spacing: 6) {
                Text("Incident \(node.incidentID)")
                    .font(.caption2.monospaced())
                    .foregroundStyle(.secondary)
                    .textSelection(.enabled)
                    .lineLimit(1)
                    .truncationMode(.middle)
                Spacer(minLength: 4)
                Button {
                    NSPasteboard.general.clearContents()
                    NSPasteboard.general.setString(
                        node.incidentID,
                        forType: .string
                    )
                } label: {
                    Image(systemName: "doc.on.doc")
                }
                .buttonStyle(.plain)
                .accessibilityLabel("Copy Agent incident ID")
                .help("Copy Agent incident ID")
            }

            Text(
                node.failureRetryable
                    ? "Retryable. Enter guidance in Continue this Mission below to review a new audited Attempt."
                    : "This Attempt cannot retry automatically. Review the failure before changing its route or Team."
            )
            .font(.caption2)
            .foregroundStyle(.secondary)
            .fixedSize(horizontal: false, vertical: true)

            if !node.recoveryAction.isEmpty {
                Text("Next: \(missionHumanStatus(node.recoveryAction))")
                    .font(.caption2.weight(.medium))
                    .foregroundStyle(.secondary)
            }
        }
        .padding(10)
        .background(LoomGraphite.statusDanger.opacity(0.07))
        .overlay(alignment: .leading) {
            Rectangle()
                .fill(LoomGraphite.statusDanger)
                .frame(width: 2)
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel(
            "Agent failure. \(missionHumanStatus(node.failureCode)). Incident \(node.incidentID)"
        )
    }

    private func missionContextIntervention(
        _ mission: LocalProductMissionSummary
    ) -> some View {
        let draft = store.workbench.selectedContinuity.composerDraft
        let preparation = missionContinuationPreparation(
            status: mission.status,
            title: presentedMissionTitle(mission),
            teamInstanceID: mission.teamInstanceID,
            draft: draft
        )
        let teamAvailable = store.executableTeams.contains {
            $0.teamInstanceID == mission.teamInstanceID
        }
        return VStack(alignment: .leading, spacing: 9) {
            Label("Continue this Mission", systemImage: "arrow.triangle.branch")
                .font(.subheadline.weight(.semibold))
                .foregroundStyle(LoomGraphite.statusWarning)

            TextField(
                "Tell the Team what to change or try next",
                text: Binding(
                    get: { store.workbench.selectedContinuity.composerDraft },
                    set: store.updateMissionComposerDraft
                ),
                axis: .vertical
            )
            .lineLimit(2...5)
            .textFieldStyle(.roundedBorder)
            .accessibilityLabel("Mission continuation guidance")
            .accessibilityHint("Tell the Agent Team what to change or try next")
            .accessibilityIdentifier("loom.mission.continuation-guidance")

            Button {
                guard let preparation else { return }
                pendingMissionObjective = preparation.objective
                pendingMissionTeamID = preparation.teamInstanceID
                fullGovernancePresentation = .newMission
            } label: {
                Label("Review & continue", systemImage: "arrow.right.circle.fill")
                    .frame(maxWidth: .infinity)
            }
            .buttonStyle(.borderedProminent)
            .disabled(preparation == nil || !teamAvailable)

            Text(
                teamAvailable
                    ? "This creates a new audited Attempt; previous output and diagnostics stay available."
                    : "Restore this Team's Runtime or Provider before continuing."
            )
            .font(.caption2)
            .foregroundStyle(.secondary)
            .fixedSize(horizontal: false, vertical: true)
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel("Blocked Mission intervention")
    }

    private func startRoundTableFromMission(
        _ mission: LocalProductMissionSummary
    ) {
        pendingRoundtableMissionLink = roundtableLink(for: mission)
        roundtableContextView = nil
        governance.open(.roundtable)
    }

    private func roundtableLink(
        for mission: LocalProductMissionSummary
    ) -> LocalRoundtableMissionLink {
        let linkedConversation = store.conversationLinkedToMission(
            mission.missionID
        )
        let conversationID = linkedConversation?.threadID
            ?? store.currentChatThreadID()
        return LocalRoundtableMissionLink(
            conversationID: conversationID.isEmpty
                ? "mission:\(mission.missionID)" : conversationID,
            missionID: mission.missionID,
            teamInstanceID: mission.teamInstanceID,
            title: "Discuss: \(presentedMissionTitle(mission))",
            teamRoleIDs: Set(
                mission.topology.map(\.logicalNodeID).filter { !$0.isEmpty }
            ),
            runtimeInstanceIDs: Set(
                mission.topology.map(\.runtimeInstanceID).filter { !$0.isEmpty }
            )
        )
    }

    private func missionContextStatusSymbol(_ status: String) -> String {
        switch status {
        case "succeeded": return "checkmark.circle.fill"
        case "failed", "cancelled": return "xmark.circle.fill"
        case "blocked", "human_required": return "exclamationmark.circle.fill"
        default: return "arrow.triangle.2.circlepath"
        }
    }

    private func missionContextStatusColor(_ status: String) -> Color {
        switch status {
        case "succeeded": return LoomGraphite.statusSuccess
        case "failed", "cancelled": return LoomGraphite.statusDanger
        case "blocked", "human_required": return LoomGraphite.statusWarning
        default: return LoomGraphite.accent
        }
    }

    @ViewBuilder
    private var roundtableContextPanel: some View {
        if case .mission(let missionID) = store.workbench.route,
           let mission = store.snapshot?.missions.first(where: {
               $0.missionID == missionID
           }) {
            let sessionID = store.roundtableSessionID(
                forMissionID: missionID
            )
            VStack(alignment: .leading, spacing: 14) {
                HStack(alignment: .firstTextBaseline, spacing: 8) {
                    Label(
                        "Agent discussion",
                        systemImage: "person.2.wave.2"
                    )
                    .font(.headline)
                    Spacer(minLength: 8)
                    if roundtableContextBusy {
                        ProgressView()
                            .controlSize(.small)
                            .accessibilityLabel("Updating RoundTable")
                    }
                }

                Text(presentedMissionTitle(mission))
                    .font(.callout.weight(.medium))
                    .foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)

                if let view = roundtableContextView,
                   view.session.context?.missionID == missionID {
                    roundtableContextSummary(view)
                    Divider()
                    roundtableContextSeats(view)
                    Divider()
                    roundtableContextCommands(view, mission: mission)
                } else if sessionID == nil {
                    Label(
                        "No RoundTable for this Mission",
                        systemImage: "person.2.slash"
                    )
                    .font(.subheadline.weight(.semibold))
                    .foregroundStyle(.secondary)
                    Text(
                        "Start one when the Team needs multiple Agent perspectives. The conversation, Mission and frozen Team routes stay linked."
                    )
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)

                    Button {
                        openFullRoundTable(for: mission)
                    } label: {
                        Label(
                            "Set up discussion",
                            systemImage: "plus.circle.fill"
                        )
                        .frame(maxWidth: .infinity)
                    }
                    .buttonStyle(.borderedProminent)
                } else if roundtableContextBusy && store.roundtableIsPreparing {
                    HStack(spacing: 8) {
                        ProgressView()
                            .controlSize(.small)
                        Text("Preparing the Agent Team")
                            .foregroundStyle(.secondary)
                    }
                    .accessibilityElement(children: .combine)
                    .accessibilityLabel("Preparing the Agent Team")
                } else if let failure = store.roundtableOperationFailure,
                          roundtableContextError == nil
                            || roundtableContextError == failure.message {
                    roundtableContextOperationFailure(
                        failure,
                        retry: {
                            Task {
                                guard let sessionID else { return }
                                await loadRoundtableContext(
                                    missionID: missionID,
                                    sessionID: sessionID
                                )
                            }
                        }
                    )
                } else if let error = roundtableContextError
                    ?? store.roundtableError,
                          !error.isEmpty {
                    Label(error, systemImage: "exclamationmark.triangle")
                        .font(.callout)
                        .foregroundStyle(LoomGraphite.statusDanger)
                        .textSelection(.enabled)
                        .fixedSize(horizontal: false, vertical: true)
                    Button {
                        Task {
                            guard let sessionID else { return }
                            await loadRoundtableContext(
                                missionID: missionID,
                                sessionID: sessionID
                            )
                        }
                    } label: {
                        Label("Retry", systemImage: "arrow.clockwise")
                    }
                    .buttonStyle(.bordered)
                } else {
                    HStack(spacing: 8) {
                        ProgressView()
                            .controlSize(.small)
                        Text("Restoring the linked discussion")
                            .foregroundStyle(.secondary)
                    }
                }
            }
            .task(id: "\(missionID):\(sessionID ?? "new")") {
                guard let sessionID else { return }
                await loadRoundtableContext(
                    missionID: missionID,
                    sessionID: sessionID
                )
            }
        } else {
            VStack(alignment: .leading, spacing: 12) {
                emptyPanelState(
                    "Open a Mission before starting a RoundTable",
                    symbol: "person.2.wave.2"
                )
                Button {
                    selectNavigation(.work)
                } label: {
                    Label("Show Missions", systemImage: "flag.checkered")
                }
                .buttonStyle(.bordered)
            }
        }
    }

    private func roundtableContextOperationFailure(
        _ failure: LocalProductRoundtableOperationFailure,
        retry: @escaping () -> Void
    ) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(alignment: .firstTextBaseline, spacing: 8) {
                Label(
                    failure.title,
                    systemImage: "exclamationmark.triangle.fill"
                )
                .font(.callout.weight(.semibold))
                .foregroundStyle(LoomGraphite.statusDanger)
                Spacer(minLength: 8)
                Text(conversationChatStageLabel(failure.stage))
                    .font(.caption2)
                    .foregroundStyle(.secondary)
            }
            Text(failure.detail)
                .font(.caption)
                .foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)
            Text(failure.recoveryAction)
                .font(.caption.weight(.medium))
                .fixedSize(horizontal: false, vertical: true)
            HStack(spacing: 8) {
                if failure.recoverable {
                    Button(action: retry) {
                        Label("Retry", systemImage: "arrow.clockwise")
                    }
                    .disabled(roundtableContextBusy)
                }
                Button {
                    prepareChatDiagnosticPreview()
                } label: {
                    Label(
                        "View RoundTable diagnostics",
                        systemImage: "doc.text.magnifyingglass"
                    )
                }
                Spacer(minLength: 6)
                Text("Incident \(failure.incidentID)")
                    .font(.caption2.monospaced())
                    .foregroundStyle(.secondary)
                    .textSelection(.enabled)
                    .lineLimit(1)
                    .truncationMode(.middle)
                Button {
                    NSPasteboard.general.clearContents()
                    NSPasteboard.general.setString(
                        failure.incidentID,
                        forType: .string
                    )
                } label: {
                    Image(systemName: "doc.on.doc")
                }
                .buttonStyle(.plain)
                .accessibilityLabel("Copy RoundTable incident ID")
                .help("Copy RoundTable incident ID")
            }
            .controlSize(.small)
        }
        .padding(10)
        .background(LoomGraphite.statusDanger.opacity(0.08))
        .overlay(alignment: .leading) {
            Rectangle()
                .fill(LoomGraphite.statusDanger)
                .frame(width: 2)
        }
    }

    private func roundtableContextSummary(
        _ view: LocalRoundtableView
    ) -> some View {
        let state = roundtableContextState(view)
        return VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 8) {
                Label(
                    state.label,
                    systemImage: state.symbol
                )
                .font(.subheadline.weight(.semibold))
                .foregroundStyle(state.color)
                Spacer()
                Text(roundtableSeatSummary(view))
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }

            if let round = view.rounds.sorted(by: {
                $0.sequence < $1.sequence
            }).last {
                let resultCount = view.attempts.values.filter { attempt in
                    attempt.roundID == round.id
                        && view.deliveries[attempt.attemptID]?.body.isEmpty == false
                }.count
                let roundLabel = round.sequence > 1
                    ? "Follow-up round \(round.sequence)"
                    : "Round \(round.sequence)"
                Text(resultCount == 0
                    ? roundLabel
                    : "\(roundLabel) · \(resultCount) Agent result\(resultCount == 1 ? "" : "s")"
                )
                .font(.caption)
                .foregroundStyle(.secondary)
            } else {
                Text("Ready to configure the first discussion round.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
        }
        .accessibilityElement(children: .combine)
        .accessibilityLabel(
            "RoundTable \(state.label), \(roundtableSeatSummary(view))"
        )
    }

    private func roundtableContextSeats(
        _ view: LocalRoundtableView
    ) -> some View {
        let seats = view.seats.values.filter(\.available).sorted {
            if $0.id == view.session.moderatorSeat { return true }
            if $1.id == view.session.moderatorSeat { return false }
            return $0.displayName.localizedCaseInsensitiveCompare(
                $1.displayName
            ) == .orderedAscending
        }
        return VStack(alignment: .leading, spacing: 10) {
            Text("Agent progress")
                .font(.subheadline.weight(.semibold))

            ForEach(Array(seats.enumerated()), id: \.element.id) {
                index, seat in
                if index > 0 { Divider() }
                roundtableContextSeat(seat, view: view)
            }
        }
    }

    @ViewBuilder
    private func roundtableContextSeat(
        _ seat: LocalRoundtableSeat,
        view: LocalRoundtableView
    ) -> some View {
        let attempt = roundtableLatestAttempt(in: view, seatID: seat.id)
        let delivery = attempt.flatMap { view.deliveries[$0.attemptID] }
        let binding = seat.binding?.executionBinding
        let resultExpanded = attempt.map {
            expandedRoundtableAttemptIDs.contains($0.attemptID)
        } ?? false
        VStack(alignment: .leading, spacing: 7) {
            HStack(alignment: .firstTextBaseline, spacing: 8) {
                Image(
                    systemName: seat.id == view.session.moderatorSeat
                        ? "person.crop.circle.badge.checkmark"
                        : "person.crop.circle"
                )
                .foregroundStyle(LoomGraphite.accent)
                VStack(alignment: .leading, spacing: 2) {
                    Text(seat.displayName)
                        .font(.callout.weight(.semibold))
                    if let binding {
                        Text(
                            [
                                binding.harnessAdapter,
                                binding.providerAccountID.isEmpty
                                    ? binding.providerID
                                    : binding.providerAccountID,
                                binding.modelID,
                            ]
                            .filter { !$0.isEmpty }
                            .joined(separator: " · ")
                        )
                        .font(.caption2)
                        .foregroundStyle(.secondary)
                        .lineLimit(2)
                    } else if seat.id == view.session.moderatorSeat {
                        Text("Moderator")
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                    }
                }
                Spacer(minLength: 6)
                if let attempt {
                    Text(missionHumanStatus(attempt.status))
                        .font(.caption2.weight(.medium))
                        .foregroundStyle(
                            missionContextStatusColor(attempt.status)
                        )
                }
            }

            if let body = delivery?.body, !body.isEmpty {
                Text(roundtableContextAttributedOutput(
                    body,
                    expanded: resultExpanded,
                    limit: roundtableContextPreviewLimit
                ))
                    .font(.callout)
                    .lineSpacing(2)
                    .textSelection(.enabled)
                    .fixedSize(horizontal: false, vertical: true)

                if let attempt, body.count > roundtableContextPreviewLimit {
                    Button(resultExpanded ? "Show less" : "Show full result") {
                        if resultExpanded {
                            expandedRoundtableAttemptIDs.remove(attempt.attemptID)
                        } else {
                            expandedRoundtableAttemptIDs.insert(attempt.attemptID)
                        }
                    }
                    .buttonStyle(.plain)
                    .font(.caption)
                    .accessibilityLabel(
                        resultExpanded
                            ? "Show less from \(seat.displayName)"
                            : "Show full result from \(seat.displayName)"
                    )
                    .accessibilityIdentifier(
                        "loom.roundtable.result-expansion.\(attempt.attemptID)"
                    )
                }
            } else if attempt?.status == "running" {
                HStack(spacing: 8) {
                    ProgressView()
                        .controlSize(.small)
                    Text("Waiting for this Agent's first visible update.")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
            }

            if let attempt, attempt.status == "running",
               roundtableLiveInputState(view, seatID: seat.id) == .available {
                roundtableContextLiveInput(attempt, seat: seat, view: view)
            }

            if let attempt,
               attempt.status == "failed" || attempt.status == "cancelled" {
                roundtableContextFailure(attempt, seat: seat, view: view)
            }
        }
        .padding(.vertical, 3)
    }

    private func roundtableContextLiveInput(
        _ attempt: LocalRoundtableSeatAttempt,
        seat: LocalRoundtableSeat,
        view: LocalRoundtableView
    ) -> some View {
        let draft = roundtableContextDraft(attempt.seatID)
        return VStack(alignment: .leading, spacing: 7) {
            TextField(
                "Guide this Agent while it is working",
                text: Binding(
                    get: {
                        roundtableContextDrafts[attempt.seatID] ?? ""
                    },
                    set: {
                        roundtableContextDrafts[attempt.seatID] = String(
                            $0.prefix(4_096)
                        )
                    }
                ),
                axis: .vertical
            )
            .lineLimit(2...4)
            .textFieldStyle(.roundedBorder)
            .accessibilityLabel("Guidance for \(seat.displayName)")
            .accessibilityHint("Send bounded guidance while this Agent is working")
            .accessibilityIdentifier(
                "loom.roundtable.live-guidance.\(attempt.seatID)"
            )

            Button {
                Task {
                    await steerRoundtableContextSeat(
                        attempt,
                        guidance: draft,
                        view: view
                    )
                }
            } label: {
                Label("Send guidance", systemImage: "arrow.turn.up.right")
            }
            .buttonStyle(.bordered)
            .controlSize(.small)
            .disabled(roundtableContextBusy || draft.isEmpty)
            .accessibilityLabel("Guide \(seat.displayName) while it is working")
        }
    }

    private func roundtableContextFailure(
        _ attempt: LocalRoundtableSeatAttempt,
        seat: LocalRoundtableSeat,
        view: LocalRoundtableView
    ) -> some View {
        let draft = roundtableContextDraft(attempt.seatID)
        return VStack(alignment: .leading, spacing: 8) {
            if !attempt.failureCode.isEmpty {
                Label(
                    [attempt.failureCode, attempt.failureStage]
                        .filter { !$0.isEmpty }
                        .map { missionHumanStatus($0) }
                        .joined(separator: " · "),
                    systemImage: "exclamationmark.triangle"
                )
                .font(.caption.weight(.medium))
                .foregroundStyle(LoomGraphite.statusDanger)
            }

            if !attempt.incidentID.isEmpty {
                HStack(spacing: 6) {
                    Text("Incident \(attempt.incidentID)")
                        .font(.caption2.monospaced())
                        .foregroundStyle(.secondary)
                        .textSelection(.enabled)
                        .lineLimit(1)
                        .truncationMode(.middle)
                    Spacer(minLength: 4)
                    Button {
                        NSPasteboard.general.clearContents()
                        NSPasteboard.general.setString(
                            attempt.incidentID,
                            forType: .string
                        )
                    } label: {
                        Image(systemName: "doc.on.doc")
                    }
                    .buttonStyle(.plain)
                    .accessibilityLabel("Copy incident ID")
                    .help("Copy incident ID")
                }
            }

            if attempt.retryable || attempt.status == "cancelled" {
                TextField(
                    "Tell this Agent what to try next",
                    text: Binding(
                        get: {
                            roundtableContextDrafts[attempt.seatID] ?? ""
                        },
                        set: {
                            roundtableContextDrafts[attempt.seatID] = String(
                                $0.prefix(4_096)
                            )
                        }
                    ),
                    axis: .vertical
                )
                .lineLimit(2...4)
                .textFieldStyle(.roundedBorder)
                .accessibilityLabel("Retry guidance for \(seat.displayName)")
                .accessibilityHint("Tell this Agent what to try differently")
                .accessibilityIdentifier(
                    "loom.roundtable.retry-guidance.\(attempt.seatID)"
                )

                Button {
                    Task {
                        await retryRoundtableContextSeat(
                            attempt,
                            guidance: draft,
                            view: view
                        )
                    }
                } label: {
                    Label("Retry Agent", systemImage: "arrow.clockwise")
                }
                .buttonStyle(.borderedProminent)
                .controlSize(.small)
                .disabled(roundtableContextBusy || draft.isEmpty)
                .accessibilityLabel("Retry \(seat.displayName) with guidance")
            }
        }
        .padding(10)
        .background(LoomGraphite.statusDanger.opacity(0.07))
        .overlay(alignment: .leading) {
            Rectangle()
                .fill(LoomGraphite.statusDanger)
                .frame(width: 2)
        }
    }

    private func roundtableContextCommands(
        _ view: LocalRoundtableView,
        mission: LocalProductMissionSummary
    ) -> some View {
        let running = view.attempts.values.contains { $0.status == "running" }
        return VStack(alignment: .leading, spacing: 10) {
            if running,
               let roundID = view.rounds.sorted(by: {
                   $0.sequence < $1.sequence
               }).last?.id {
                Button {
                    Task {
                        await pauseRoundtableContext(
                            view,
                            roundID: roundID
                        )
                    }
                } label: {
                    Label("Pause discussion", systemImage: "pause.fill")
                }
                .buttonStyle(.bordered)
                .disabled(roundtableContextBusy)
            }

            if roundtableReadyToAcceptCandidate(view),
               !view.session.concluded {
                if roundtableCanStartSynthesis(view) {
                    let isFollowUpRound = view.rounds.count > 1
                    Text(
                        isFollowUpRound
                            ? "The Lead synthesis and peer review are ready. Accept it, or refine the synthesis in another bounded round."
                            : "Two Agent results are ready. Start a synthesis round so the Lead can reconcile them before you accept a conclusion."
                    )
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)

                    TextField(
                        "What should the Team resolve next?",
                        text: $roundtableSynthesisDraft,
                        axis: .vertical
                    )
                    .lineLimit(2...4)
                    .textFieldStyle(.roundedBorder)
                    .accessibilityLabel("Synthesis round prompt")
                    .accessibilityIdentifier(
                        "loom.roundtable.context-synthesis-prompt"
                    )

                    Button {
                        Task {
                            await startRoundtableContextSynthesis(view)
                        }
                    } label: {
                        Label(
                            isFollowUpRound
                                ? "Refine synthesis" : "Synthesize Agent results",
                            systemImage: "arrow.triangle.2.circlepath"
                        )
                        .frame(maxWidth: .infinity)
                    }
                    .buttonStyle(.borderedProminent)
                    .disabled(
                        roundtableContextBusy ||
                        roundtableSynthesisDraft.trimmingCharacters(
                            in: .whitespacesAndNewlines
                        ).isEmpty || roundtableSynthesisDraft.utf8.count > 4_096
                    )
                }

                Button {
                    Task { await acceptRoundtableContextConclusion(view) }
                } label: {
                    Label(
                        view.rounds.count > 1
                            ? "Accept Lead synthesis" : "Accept Lead result",
                        systemImage: "checkmark.seal.fill"
                    )
                    .frame(maxWidth: .infinity)
                }
                .buttonStyle(.bordered)
                .disabled(roundtableContextBusy)
            }

            Button {
                openFullRoundTable(for: mission)
            } label: {
                Label(
                    "Open full discussion",
                    systemImage: "arrow.up.right.square"
                )
            }
            .buttonStyle(.plain)
        }
    }

    private func loadRoundtableContext(
        missionID: String,
        sessionID: String
    ) async {
        guard !roundtableContextBusy else { return }
        roundtableContextBusy = true
        defer { roundtableContextBusy = false }
        var loaded: LocalRoundtableView?
        for attempt in 0..<90 {
            loaded = await store.roundtableLoadSession(sessionID: sessionID)
            if loaded != nil {
                break
            }
            let loadError = store.roundtableError
            if !store.roundtableIsPreparing, let loadError,
               !loadError.isEmpty {
                roundtableContextView = nil
                roundtableContextError = loadError
                return
            }
            roundtableContextError = nil
            guard attempt < 89 else { break }
            do {
                try await Task.sleep(nanoseconds: 1_000_000_000)
            } catch {
                return
            }
        }
        guard let loaded else {
            roundtableContextView = nil
            roundtableContextError = store.roundtableError
                ?? "The discussion is taking longer to restore. Retry without leaving this Mission."
            return
        }
        guard loaded.session.context?.missionID == missionID else {
            roundtableContextView = nil
            roundtableContextError =
                "This RoundTable belongs to a different Mission. Reopen it from the correct Mission."
            return
        }
        roundtableContextView = loaded
        roundtableContextError = nil
        await followRoundtableContext(
            missionID: missionID,
            sessionID: sessionID
        )
    }

    private func followRoundtableContext(
        missionID: String,
        sessionID: String
    ) async {
        while !Task.isCancelled,
              governance.destination == .roundtable,
              case .mission(let selectedMissionID) = store.workbench.route,
              selectedMissionID == missionID,
              roundtableContextView?.session.concluded == false,
              roundtableContextView?.attempts.values.contains(where: {
                  $0.status == "running"
              }) == true {
            try? await Task.sleep(nanoseconds: 1_000_000_000)
            guard !Task.isCancelled else { return }
            if let refreshed = await store.roundtableSnapshot(
                sessionID: sessionID
            ), refreshed.session.context?.missionID == missionID {
                roundtableContextView = refreshed
                roundtableContextError = nil
            } else {
                roundtableContextError = store.roundtableError
                return
            }
        }
    }

    private func steerRoundtableContextSeat(
        _ attempt: LocalRoundtableSeatAttempt,
        guidance: String,
        view: LocalRoundtableView
    ) async {
        let bounded = guidance.trimmingCharacters(
            in: .whitespacesAndNewlines
        )
        guard !roundtableContextBusy, !bounded.isEmpty,
              bounded.utf8.count <= 4_096,
              roundtableLiveInputState(
                  view,
                  seatID: attempt.seatID
              ) == .available else { return }
        roundtableContextBusy = true
        if let updated = await store.roundtableSteerSeat(
            sessionID: view.session.id,
            roundID: attempt.roundID,
            seatID: attempt.seatID,
            attemptID: attempt.attemptID,
            guidance: bounded
        ) {
            roundtableContextView = updated
            roundtableContextDrafts[attempt.seatID] = ""
            roundtableContextError = nil
        } else {
            roundtableContextError = store.roundtableError
        }
        roundtableContextBusy = false
    }

    private func retryRoundtableContextSeat(
        _ attempt: LocalRoundtableSeatAttempt,
        guidance: String,
        view: LocalRoundtableView
    ) async {
        let bounded = guidance.trimmingCharacters(
            in: .whitespacesAndNewlines
        )
        guard !roundtableContextBusy, !bounded.isEmpty,
              bounded.utf8.count <= 4_096 else { return }
        let binding = view.seats[attempt.seatID]?.binding
        roundtableContextBusy = true
        if let updated = await store.roundtableRetrySeat(
            sessionID: view.session.id,
            roundID: attempt.roundID,
            seatID: attempt.seatID,
            attemptID: attempt.attemptID,
            guidance: bounded,
            expectedMembershipRevision: binding?.membershipRevision,
            expectedSeatBindingDigest: binding?.bindingDigest
        ) {
            roundtableContextView = updated
            roundtableContextDrafts[attempt.seatID] = ""
            roundtableContextError = nil
            roundtableContextBusy = false
            if let missionID = updated.session.context?.missionID {
                await followRoundtableContext(
                    missionID: missionID,
                    sessionID: updated.session.id
                )
            }
        } else {
            roundtableContextError = store.roundtableError
            roundtableContextBusy = false
        }
    }

    private func pauseRoundtableContext(
        _ view: LocalRoundtableView,
        roundID: String
    ) async {
        guard !roundtableContextBusy else { return }
        roundtableContextBusy = true
        defer { roundtableContextBusy = false }
        if let updated = await store.roundtablePauseRound(
            sessionID: view.session.id,
            roundID: roundID
        ) {
            roundtableContextView = updated
            roundtableContextError = nil
        } else {
            roundtableContextError = store.roundtableError
        }
    }

    private func startRoundtableContextSynthesis(
        _ view: LocalRoundtableView
    ) async {
        let prompt = roundtableSynthesisDraft.trimmingCharacters(
            in: .whitespacesAndNewlines
        )
        guard !roundtableContextBusy, roundtableCanStartSynthesis(view),
              !prompt.isEmpty, prompt.utf8.count <= 4_096 else { return }
        roundtableContextBusy = true
        if let updated = await store.roundtableOpenRound(
            sessionID: view.session.id,
            roundID: roundtableNextRoundID(view),
            prompt: prompt
        ) {
            roundtableContextView = updated
            roundtableContextError = nil
            roundtableContextBusy = false
            if let missionID = updated.session.context?.missionID {
                await followRoundtableContext(
                    missionID: missionID,
                    sessionID: updated.session.id
                )
            }
        } else {
            roundtableContextError = store.roundtableError
            roundtableContextBusy = false
        }
    }

    private func acceptRoundtableContextConclusion(
        _ view: LocalRoundtableView
    ) async {
        guard !roundtableContextBusy,
              roundtableReadyToAcceptCandidate(view) else { return }
        roundtableContextBusy = true
        defer { roundtableContextBusy = false }
        if let updated = await store.roundtableConclude(
            sessionID: view.session.id
        ) {
            roundtableContextView = updated
            roundtableContextError = nil
        } else {
            roundtableContextError = store.roundtableError
        }
    }

    private func openFullRoundTable(
        for mission: LocalProductMissionSummary
    ) {
        pendingRoundtableMissionLink = roundtableLink(for: mission)
        fullGovernancePresentation = .roundtable
    }

    private func roundtableContextDraft(_ seatID: String) -> String {
        let value = (roundtableContextDrafts[seatID] ?? "")
            .trimmingCharacters(in: .whitespacesAndNewlines)
        return value.utf8.count <= 4_096 ? value : ""
    }

    private func roundtableContextState(
        _ view: LocalRoundtableView
    ) -> (label: String, symbol: String, color: Color) {
        if view.session.concluded {
            return (
                "Concluded",
                "checkmark.seal.fill",
                LoomGraphite.statusSuccess
            )
        }
        if view.attempts.values.contains(where: { $0.status == "running" }) {
            return (
                "Agents responding",
                "arrow.triangle.2.circlepath",
                LoomGraphite.accent
            )
        }
        if roundtableNeedsIntervention(view) {
            return (
                "Needs you",
                "exclamationmark.circle.fill",
                LoomGraphite.statusWarning
            )
        }
        if view.rounds.isEmpty {
            return ("Ready", "circle.dashed", LoomGraphite.accent)
        }
        return (
            "Round complete",
            "checkmark.circle.fill",
            LoomGraphite.statusSuccess
        )
    }

    @ViewBuilder
    private var teamPanel: some View {
        if let session = store.builderSession {
            teamBuilderPanel(session)
        } else {
            confirmedTeamsPanel
        }
    }

    private var confirmedTeamsPanel: some View {
        VStack(alignment: .leading, spacing: 0) {
            if let confirmation = store.lastConfirmation,
               confirmation.teamInstanceCreated {
                VStack(alignment: .leading, spacing: 8) {
                    Label("Agent Team ready", systemImage: "checkmark.seal.fill")
                        .font(.subheadline.weight(.semibold))
                        .foregroundStyle(.green)
                    Text("The Team is available for a Mission. Start from here without losing the work you just described.")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .fixedSize(horizontal: false, vertical: true)
                    Button {
                        pendingMissionObjective = ""
                        pendingMissionTeamID = ""
                        fullGovernancePresentation = .newMission
                    } label: {
                        Label("Start Mission with this Team", systemImage: "flag.checkered")
                    }
                    .buttonStyle(.borderedProminent)
                    .controlSize(.small)
                    .loomActionTarget()
                }
                .padding(12)
                .background(Color.green.opacity(0.10), in: RoundedRectangle(cornerRadius: 8))
                .padding(.bottom, 12)
            }
            if let recoveryMessage = store.builderRecoveryMessage {
                Label(recoveryMessage, systemImage: "arrow.clockwise")
                    .font(.caption)
                    .foregroundStyle(LoomGraphite.statusWarning)
                    .fixedSize(horizontal: false, vertical: true)
                    .padding(.bottom, 12)
                    .accessibilityLabel("Agent Team draft expired. \(recoveryMessage)")
            }
            let teams = currentTeamConfigurations(store.snapshot?.teams ?? [])
            if teams.isEmpty {
                emptyPanelState("No Agent Teams yet", symbol: "person.3")
            } else {
                ForEach(teams) { team in
                    HStack(spacing: 10) {
                        Button {
                            openTeamGovernance(team)
                        } label: {
                            inspectorRow(
                                title: LocalProductExperience.visibleName(
                                    team.displayName,
                                    internalID: team.teamInstanceID,
                                    fallback: "Agent Team"
                                ),
                                detail: teamInspectorDetail(team),
                                symbol: "person.3"
                            )
                        }
                        .buttonStyle(.plain)
                        .loomActionTarget()
                        .accessibilityLabel(
                            "Open Agent Team \(team.displayName) governance"
                        )
                        .help("Open Team governance")

                        if team.confirmed && team.executable && !team.readOnly {
                            Button {
                                if let mission = teamMission(for: team) {
                                    Task { await openMissionContext(mission) }
                                } else {
                                    startMission(for: team)
                                }
                            } label: {
                                Label(
                                    teamMission(for: team) == nil
                                        ? "Start Mission" : "Open Mission",
                                    systemImage: teamMission(for: team) == nil
                                        ? "flag.checkered" : "arrow.up.right.square"
                                )
                                    .labelStyle(.titleAndIcon)
                            }
                            .buttonStyle(.borderedProminent)
                            .controlSize(.small)
                            .loomActionTarget()
                            .accessibilityLabel(
                                teamMission(for: team) == nil
                                    ? "Start Mission with \(team.displayName)"
                                    : "Open the existing Mission for \(team.displayName)"
                            )
                            .help(
                                teamMission(for: team) == nil
                                    ? "Start a governed Mission with this Team"
                                    : "Review the existing Mission before starting another"
                            )
                        }
                    }
                }
            }
            Button {
                Task { await store.startBlankBuilder() }
            } label: {
                Label("Create Agent Team", systemImage: "plus")
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
            .buttonStyle(.bordered)
            .padding(.top, 14)
            .loomActionTarget()
        }
    }

    private func openTeamGovernance(_ team: LocalProductTeamSummary) {
        store.selectTeam(team)
        governance.open(.team)
        Task { await store.activateSelectedTeam() }
    }

    private func startMission(for team: LocalProductTeamSummary) {
        store.selectTeam(team)
        pendingMissionObjective = ""
        pendingMissionTeamID = team.teamInstanceID
        fullGovernancePresentation = .newMission
    }

    private func teamInspectorDetail(_ team: LocalProductTeamSummary) -> String {
        let state = humanized(team.state)
        if team.readOnly { return "Read-only · \(state)" }
        if let mission = teamMission(for: team) {
            return "Mission · \(humanized(mission.status)) · \(state)"
        }
        if team.confirmed && team.executable { return "Ready for a Mission · \(state)" }
        return state
    }

    private func teamMission(
        for team: LocalProductTeamSummary
    ) -> LocalProductMissionSummary? {
        store.snapshot?.missions.first {
            $0.teamInstanceID == team.teamInstanceID
        }
    }

    private func teamBuilderPanel(
        _ session: LocalProductBuilderSession
    ) -> some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack(alignment: .top) {
                VStack(alignment: .leading, spacing: 3) {
                    Text("Agent Team draft")
                        .font(.headline)
                    Text("Review every field before confirmation.")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                Spacer()
                Button {
                    builderAnswer = ""
                    store.cancelBuilder()
                } label: {
                    Image(systemName: "xmark")
                        .frame(width: 28, height: 28)
                }
                .buttonStyle(.plain)
                .accessibilityLabel("Cancel Agent Team draft")
                .help("Cancel draft")
            }

            if !modelTeamEditInstruction.isEmpty {
                VStack(alignment: .leading, spacing: 4) {
                    Label("Requested change", systemImage: "sparkles")
                        .font(.caption.weight(.semibold))
                        .foregroundStyle(LoomGraphite.accent)
                    Text(modelTeamEditInstruction)
                        .font(.callout)
                        .fixedSize(horizontal: false, vertical: true)
                        .textSelection(.enabled)
                    Text("This request is guidance only. Review and edit the Team fields below before confirming.")
                        .font(.caption2)
                        .foregroundStyle(.secondary)
                        .fixedSize(horizontal: false, vertical: true)
                }
                .padding(10)
                .background(
                    LoomGraphite.accentMuted,
                    in: RoundedRectangle(cornerRadius: 6)
                )
                .accessibilityElement(children: .contain)
                .accessibilityIdentifier("loom.team.model-edit-instruction")
            }

            if !session.question.id.isEmpty {
                Text(session.question.prompt)
                    .font(.callout.weight(.medium))
                    .fixedSize(horizontal: false, vertical: true)
                if session.question.options.isEmpty {
                    HStack(spacing: 8) {
                        TextField("Answer", text: $builderAnswer)
                            .textFieldStyle(.roundedBorder)
                        Button {
                            let answer = builderAnswer
                            builderAnswer = ""
                            Task { await store.answerBuilder(answer) }
                        } label: {
                            Image(systemName: "arrow.right")
                        }
                        .buttonStyle(.borderedProminent)
                        .disabled(
                            builderAnswer.trimmingCharacters(
                                in: .whitespacesAndNewlines
                            ).isEmpty || store.setupState == .loading
                        )
                        .accessibilityLabel("Submit Team draft answer")
                    }
                } else {
                    ForEach(session.question.options) { option in
                        Button {
                            Task { await store.answerBuilder(option.id) }
                        } label: {
                            HStack {
                                Text(option.label)
                                Spacer()
                                Image(systemName: "chevron.right")
                            }
                            .frame(maxWidth: .infinity, alignment: .leading)
                        }
                        .buttonStyle(.bordered)
                        .disabled(store.setupState == .loading)
                    }
                }
                Divider()
            }

            VStack(alignment: .leading, spacing: 8) {
                TextField(
                    "Team name",
                    text: Binding(
                        get: {
                            builderName.isEmpty ? session.preview.name : builderName
                        },
                        set: { builderName = $0 }
                    )
                )
                .textFieldStyle(.roundedBorder)
                .onSubmit {
                    let value = builderName.trimmingCharacters(in: .whitespacesAndNewlines)
                    guard !value.isEmpty, value != session.preview.name else { return }
                    Task { await store.editBuilder(field: "team_name", value: value) }
                }
                .disabled(store.setupState == .loading)
                .accessibilityLabel("Agent Team name")

                TextField(
                    "Bounded purpose",
                    text: Binding(
                        get: {
                            builderPurpose.isEmpty ? session.preview.purpose : builderPurpose
                        },
                        set: { builderPurpose = $0 }
                    )
                )
                .textFieldStyle(.roundedBorder)
                .onSubmit {
                    let value = builderPurpose.trimmingCharacters(in: .whitespacesAndNewlines)
                    guard !value.isEmpty, value != session.preview.purpose else { return }
                    Task { await store.editBuilder(field: "purpose", value: value) }
                }
                .disabled(store.setupState == .loading)
                .accessibilityLabel("Agent Team bounded purpose")
                Label(
                    "\(session.preview.roles.count) agents · concurrency \(session.preview.requestedConcurrency)",
                    systemImage: "person.3"
                )
                Label(
                    session.preview.estimatedMaximumCost,
                    systemImage: "gauge.with.dots.needle.50percent"
                )
                .foregroundStyle(.secondary)
                if !session.preview.compatibilityGaps.isEmpty {
                    Label(
                        "\(session.preview.compatibilityGaps.count) compatibility issues",
                        systemImage: "exclamationmark.triangle"
                    )
                    .foregroundStyle(LoomGraphite.statusWarning)
                }
            }
            .font(.caption)

            runtimeAvailabilityPanel

			ForEach(session.preview.roles) { role in
				builderRoleEditor(role, session: session)
			}

			let availableAgents = builderAvailableAgentChoices(session)
			if !availableAgents.isEmpty && session.preview.roles.count < 9 {
				Menu {
					ForEach(availableAgents) { choice in
						Button {
							Task {
								await store.editBuilder(
									field: "subagent_add",
									value: choice.id
								)
							}
						} label: {
							Text(builderRoleChoiceLabel(choice))
						}
					}
				} label: {
					Label("Add Agent", systemImage: "plus")
				}
				.controlSize(.small)
				.disabled(store.setupState == .loading)
				.help("Add an independently configured Agent")
			}

            if store.setupState == .loading {
                ProgressView()
                    .controlSize(.small)
                    .accessibilityLabel("Updating Agent Team draft")
            }

        }
    }

    private var runtimeAvailabilityPanel: some View {
        let runtimes = store.setupSnapshot?.runtimes ?? []
        return VStack(alignment: .leading, spacing: 8) {
            HStack {
                Label("Available Harnesses", systemImage: "cpu")
                    .font(.subheadline.weight(.semibold))
                Spacer()
                Text("Choose per Agent")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            if runtimes.isEmpty {
                Label(
                    "No Harness runtime has been detected yet. Refresh Runtime & Providers before confirming this Team.",
                    systemImage: "exclamationmark.triangle"
                )
                .font(.caption)
                .foregroundStyle(LoomGraphite.statusWarning)
                .fixedSize(horizontal: false, vertical: true)
            } else {
                ForEach(runtimes) { runtime in
                    let ready = ["online", "ready", "running", "available"].contains(runtime.status)
                    HStack(spacing: 8) {
                        Image(systemName: ready ? "checkmark.circle.fill" : "circle.slash")
                            .foregroundStyle(ready ? Color.green : LoomGraphite.statusWarning)
                        VStack(alignment: .leading, spacing: 2) {
                            Text(LocalProductExperience.visibleName(
                                runtime.displayName,
                                internalID: runtime.runtimeInstanceID,
                                fallback: runtime.adapterType
                            ))
                            .font(.caption.weight(.semibold))
                            Text(
                                runtimeAvailabilityDetail(
                                    modelCount: runtime.modelIDs.count,
                                    status: runtime.status,
                                    ready: ready
                                )
                            )
                                .font(.caption2)
                                .foregroundStyle(.secondary)
                        }
                        Spacer()
                        Text(builderHarnessLabel(runtime.adapterType))
                            .font(.caption2.monospaced())
                            .foregroundStyle(.secondary)
                    }
                    .accessibilityElement(children: .combine)
                    .accessibilityLabel(
                        runtimeAvailabilityAccessibilityLabel(
                            displayName: runtime.displayName,
                            ready: ready
                        )
                    )
                }
            }
            Text("OpenCode is supported as a native Harness when its executable is detected. It uses native authentication and is separate from OpenAI/Codex credentials.")
                .font(.caption2)
                .foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)
        }
        .padding(12)
        .background(LoomGraphite.canvas, in: RoundedRectangle(cornerRadius: 8))
        .overlay {
            RoundedRectangle(cornerRadius: 8)
                .stroke(LoomGraphite.separator, lineWidth: 1)
        }
    }

    @ViewBuilder
    private var teamBuilderConfirmFooter: some View {
        if let session = store.builderSession {
            VStack(alignment: .leading, spacing: 8) {
                Button {
                    Task {
                        await store.confirmBuilderCommitting(
                            name: builderName,
                            purpose: builderPurpose
                        )
                    }
                } label: {
                    Label("Confirm Agent Team", systemImage: "checkmark")
                        .frame(maxWidth: .infinity)
                }
                .buttonStyle(.borderedProminent)
                .disabled(!session.canConfirm || store.setupState == .loading)
                .accessibilityLabel("Confirm Agent Team and create governed Team")
                if !session.canConfirm && store.setupState != .loading {
                    builderConfirmBlockedHint(
                        session: session,
                        uncommittedName: builderName,
                        uncommittedPurpose: builderPurpose
                    )
                }
            }
            .padding(16)
        }
    }

    private func builderConfirmBlockedHint(
        session: LocalProductBuilderSession,
        uncommittedName: String,
        uncommittedPurpose: String
    ) -> some View {
        let message = session.preview.confirmationBlockedMessage(
            uncommittedName: uncommittedName,
            uncommittedPurpose: uncommittedPurpose
        )
        return Label(message, systemImage: "info.circle")
            .font(.caption2)
            .foregroundStyle(.secondary)
            .fixedSize(horizontal: false, vertical: true)
            .accessibilityLabel("Team confirmation blocked. \(message)")
    }

	private func builderRoleEditor(
		_ role: LocalProductBuilderRole,
		session: LocalProductBuilderSession
	) -> some View {
		let review = LocalProductRoleReview(role: role)
		let field = role.kind == "main" ? "main_role" : "subagent_role"
		let roleTarget = role.kind == "subagent" ? role.agentDefinitionID : ""
		let choices = store.setupSnapshot.map {
			LocalProductRoleOptionChoice.all(
				setup: $0,
				preview: session.preview
			).filter {
				$0.field == field &&
				$0.agentDefinitionID == role.agentDefinitionID
			}
		} ?? []
		return VStack(alignment: .leading, spacing: 7) {
			HStack(spacing: 8) {
				VStack(alignment: .leading, spacing: 2) {
					Text(review.name.isEmpty ? review.kind : review.name)
						.font(.callout.weight(.semibold))
					Text(review.kind)
						.font(.caption)
						.foregroundStyle(.secondary)
				}
				Spacer(minLength: 8)
				Menu {
					ForEach(choices) { choice in
						Button {
							Task {
								await store.editBuilder(
									field: choice.field,
									value: choice.id,
									roleAgentDefinitionID: roleTarget
								)
							}
						} label: {
							Label(
								builderRoleChoiceLabel(choice),
								systemImage: choice.isCurrent
									? "checkmark"
									: "circle"
							)
						}
					}
				} label: {
					Image(systemName: "slider.horizontal.3")
						.frame(width: 28, height: 28)
				}
				.menuStyle(.borderlessButton)
				.disabled(choices.isEmpty || store.setupState == .loading)
				.accessibilityLabel("Choose \(review.kind) Agent role")
				.help("Choose Agent role")
				if role.kind == "subagent" &&
					session.preview.roles.filter({ $0.kind == "subagent" }).count > 1 {
					Button {
						Task {
							await store.editBuilder(
								field: "subagent_remove",
								value: "remove",
								roleAgentDefinitionID: role.agentDefinitionID
							)
						}
					} label: {
						Image(systemName: "trash")
							.frame(width: 28, height: 28)
					}
					.buttonStyle(.plain)
					.accessibilityLabel("Remove \(review.name) Agent")
					.help("Remove Agent")
				}
			}

			Text(
				[
					review.harness,
					review.providerAccount.isEmpty
						? review.provider
						: "\(review.provider) / \(review.providerAccount)",
					review.model,
				].filter { !$0.isEmpty }.joined(separator: " · ")
			)
			.font(.caption)
			.foregroundStyle(.secondary)
			.fixedSize(horizontal: false, vertical: true)

			Text(
				[review.credential, review.reasoning, review.timeout, review.budget]
					.filter { !$0.isEmpty }
					.joined(separator: " · ")
			)
			.font(.caption2)
			.foregroundStyle(.tertiary)
			.fixedSize(horizontal: false, vertical: true)

			builderRoleProfileControls(role, review: review)

			if !role.compatible {
				Label(review.compatibility, systemImage: "exclamationmark.triangle")
					.font(.caption)
					.foregroundStyle(LoomGraphite.statusWarning)
			}
			Divider()
		}
	}

	@ViewBuilder
	private func builderRoleProfileControls(
		_ role: LocalProductBuilderRole,
		review: LocalProductRoleReview
	) -> some View {
		let prefix = role.kind == "main" ? "main" : "subagent"
		let roleTarget = role.kind == "subagent" ? role.agentDefinitionID : ""
		let timeoutSeconds = max(1, Int(role.timeoutMilliseconds / 1_000))
		let fallbackChoices = store.setupSnapshot.map {
			LocalProductRoleOptionChoice.fallbacks(setup: $0, role: role)
		} ?? []
		let selectedParallelProfiles = Set(
			role.parallelRoutes.dropFirst().map(\.runtimeProfileID)
		)
		let parallelChoices = fallbackChoices.filter {
			!selectedParallelProfiles.contains($0.runtimeProfileID)
		}
		let harnessRoutes = uniqueBuilderRouteChoices(
			store.setupSnapshot.map {
				LocalProductRoleOptionChoice.routes(
					setup: $0,
					role: role,
					field: "\(prefix)_harness_route"
				)
			} ?? [],
			identity: { $0.harness }
		)
		let accountRoutes = uniqueBuilderRouteChoices(
			store.setupSnapshot.map {
				LocalProductRoleOptionChoice.routes(
					setup: $0,
					role: role,
					field: "\(prefix)_provider_account_route"
				)
			} ?? [],
			identity: {
				"\($0.provider)\u{0}\($0.providerAccount)\u{0}\($0.credential)\u{0}\($0.model)"
			}
		)
		VStack(alignment: .leading, spacing: 6) {
			HStack(spacing: 8) {
				Label("Parallel routes", systemImage: "square.stack.3d.up")
					.font(.caption)
				Spacer(minLength: 8)
				Menu {
					ForEach(parallelChoices) { choice in
						Button {
							Task {
								await store.editBuilder(
									field: "\(prefix)_parallel_route_add",
									value: choice.id,
									roleAgentDefinitionID: roleTarget
								)
							}
						} label: {
							Label(
								builderProviderAccountChoiceLabel(choice),
								systemImage: "plus"
							)
						}
					}
				} label: {
					Label(
						role.parallelRouteSetVersion == 1
							? "\(role.parallelRoutes.count) routes"
							: "Single route",
						systemImage: "plus.circle"
					)
				}
				.controlSize(.small)
				.disabled(
					parallelChoices.isEmpty || role.parallelRoutes.count >= 3 ||
					role.fallbackConfigured || store.setupState == .loading
				)
				.help("Add a Provider route for parallel execution")
			}
			if role.parallelRouteSetVersion == 1 {
				ForEach(Array(role.parallelRoutes.dropFirst())) { route in
					HStack(spacing: 8) {
						Text([
							route.providerAccountID.isEmpty
								? route.providerID
								: route.providerAccountID,
							route.modelID,
						].filter { !$0.isEmpty }.joined(separator: " · "))
						.font(.caption2)
						.foregroundStyle(.secondary)
						.lineLimit(1)
						Spacer(minLength: 8)
						if let choice = fallbackChoices.first(where: {
							$0.runtimeProfileID == route.runtimeProfileID
						}) {
							Button {
								Task {
									await store.editBuilder(
										field: "\(prefix)_parallel_route_remove",
										value: choice.id,
										roleAgentDefinitionID: roleTarget
									)
								}
							} label: {
								Image(systemName: "trash")
							}
							.buttonStyle(.borderless)
							.help("Remove parallel route")
						}
					}
				}
				Label(
					"Synthesis · \(role.synthesisRoute.modelID)",
					systemImage: "arrow.triangle.merge"
				)
				.font(.caption2)
				.foregroundStyle(.secondary)
			}

			HStack(spacing: 8) {
				Label("Fallback route", systemImage: "arrow.triangle.branch")
					.font(.caption)
				Spacer(minLength: 8)
				Menu {
					Button {
						Task {
							await store.editBuilder(
								field: "\(prefix)_fallback_role",
								value: "none",
								roleAgentDefinitionID: roleTarget
							)
						}
					} label: {
						Label(
							"No fallback",
							systemImage: role.fallbackConfigured
								? "circle"
								: "checkmark"
						)
					}
					Divider()
					ForEach(fallbackChoices) { choice in
						Button {
							Task {
								await store.editBuilder(
									field: choice.field,
									value: choice.id,
									roleAgentDefinitionID: roleTarget
								)
							}
						} label: {
							Label(
								builderRoleChoiceLabel(choice),
								systemImage: choice.isCurrent
									? "checkmark"
									: "circle"
							)
						}
					}
				} label: {
					Text(role.fallbackConfigured ? "Configured" : "None")
				}
				.controlSize(.small)
				.disabled(
					store.setupState == .loading || role.parallelRouteSetVersion == 1
				)
				.help("Choose an explicit fallback execution profile")
			}
			if role.fallbackConfigured {
				Text(review.fallback)
					.font(.caption2)
					.foregroundStyle(.secondary)
					.fixedSize(horizontal: false, vertical: true)
				Label(
					"Approval required before route change",
					systemImage: "hand.raised"
				)
				.font(.caption2)
				.foregroundStyle(LoomGraphite.statusWarning)
			}

			HStack(spacing: 8) {
				Menu {
					ForEach(harnessRoutes) { choice in
						Button {
							Task {
								await store.editBuilder(
									field: choice.field,
									value: choice.id,
									roleAgentDefinitionID: roleTarget
								)
							}
						} label: {
							Label(
								choice.harness,
								systemImage: choice.isCurrent ? "checkmark" : "circle"
							)
						}
					}
				} label: {
					Label(review.harness, systemImage: "wrench.and.screwdriver")
				}
				.controlSize(.small)
				.disabled(harnessRoutes.isEmpty)
				.help("Choose Harness")

				Menu {
					ForEach(accountRoutes) { choice in
						Button {
							Task {
								await store.editBuilder(
									field: choice.field,
									value: choice.id,
									roleAgentDefinitionID: roleTarget
								)
							}
						} label: {
							Label(
								builderProviderAccountChoiceLabel(choice),
								systemImage: choice.isCurrent ? "checkmark" : "circle"
							)
						}
					}
				} label: {
					Label(
						review.providerAccount.isEmpty
							? review.provider
							: review.providerAccount,
						systemImage: "key.horizontal"
					)
				}
				.controlSize(.small)
				.disabled(accountRoutes.isEmpty)
				.help("Choose Provider Account")
				Spacer(minLength: 0)
			}

			HStack(spacing: 8) {
				if role.runtime.modelIDs.count > 1 {
					Menu {
						ForEach(role.runtime.modelIDs, id: \.self) { model in
							Button(model) {
								Task {
									await store.editBuilder(
										field: "\(prefix)_model",
										value: model,
										roleAgentDefinitionID: roleTarget
									)
								}
							}
						}
					} label: {
						Label(review.model, systemImage: "cpu")
					}
					.controlSize(.small)
					.help("Choose model")
				}
				if role.requiredCapabilities.contains("reasoning_effort") {
					Menu {
						ForEach([
							("Provider default", "provider_default"),
							("Low", "low"),
							("Medium", "medium"),
							("High", "high"),
						], id: \.1) { option in
							Button(option.0) {
								Task {
									await store.editBuilder(
										field: "\(prefix)_reasoning_effort",
										value: option.1,
										roleAgentDefinitionID: roleTarget
									)
								}
							}
						}
					} label: {
						Label(review.reasoning, systemImage: "brain")
					}
					.controlSize(.small)
					.help("Choose reasoning effort")
				}
				Spacer(minLength: 0)
			}

			HStack(spacing: 8) {
				Label("Remote tools", systemImage: "globe")
					.font(.caption)
				Spacer(minLength: 8)
				Menu {
					Button {
						Task {
							await store.clearBuilderRemoteToolEnrollment(
								roleAgentDefinitionID: roleTarget
							)
						}
					} label: {
						Label(
							"No remote tools",
							systemImage: role.remoteToolEnrollmentID.isEmpty
								? "checkmark"
								: "circle"
						)
					}
					Divider()
					ForEach(builderEnrollmentChoices(role)) { enrollment in
						Button {
							Task {
								await store.editBuilderRemoteToolEnrollment(
									enrollment: enrollment,
									roleAgentDefinitionID: roleTarget
								)
							}
						} label: {
							Label(
								builderEnrollmentChoiceLabel(enrollment),
								systemImage: enrollment.enrollmentID ==
									role.remoteToolEnrollmentID
									? "checkmark"
									: "circle"
							)
						}
					}
				} label: {
					Label(
						builderEnrollmentChoiceSummary(
							role: role,
							enrollments: builderEnrollmentChoices(role)
						),
						systemImage: "globe.badge.checkmark"
					)
				}
				.controlSize(.small)
				.disabled(
					store.setupState == .loading ||
					builderEnrollmentChoices(role).isEmpty
				)
				.help("Bind a remote tool Enrollment to this Agent")
			}

			HStack(spacing: 12) {
				Stepper(
					value: Binding(
						get: { timeoutSeconds },
						set: { value in
							Task {
								await store.editBuilder(
									field: "\(prefix)_timeout_seconds",
									value: String(value),
									roleAgentDefinitionID: roleTarget
								)
							}
						}
					),
					in: 15...3_600,
					step: 15
				) {
					Label(review.timeout, systemImage: "timer")
						.font(.caption)
				}
				.controlSize(.small)

				Toggle(
					isOn: Binding(
						get: { role.budgetAvailable },
						set: { enabled in
							Task {
								await store.editBuilder(
									field: "\(prefix)_budget_units",
									value: enabled
										? String(max(1, role.budgetUnits))
										: "none",
									roleAgentDefinitionID: roleTarget
								)
							}
						}
					)
				) {
					Text(role.budgetAvailable ? review.budget : "No budget")
						.font(.caption)
				}
				.toggleStyle(.switch)
				.controlSize(.mini)
			}

			if role.budgetAvailable {
				Stepper(
					value: Binding(
						get: { max(1, role.budgetUnits) },
						set: { value in
							Task {
								await store.editBuilder(
									field: "\(prefix)_budget_units",
									value: String(value),
									roleAgentDefinitionID: roleTarget
								)
							}
						}
					),
					in: 1...1_000_000_000,
					step: 5
				) {
					Label(review.budget, systemImage: "banknote")
						.font(.caption)
				}
				.controlSize(.small)
			}
		}
		.disabled(store.setupState == .loading)
	}

	private func builderRoleChoiceLabel(
		_ choice: LocalProductRoleOptionChoice
	) -> String {
		let harness = builderHarnessLabel(choice.harness)
		let account = choice.providerAccount.isEmpty
			? choice.provider
			: choice.providerAccount
		return [choice.responsibility, harness, account, choice.model]
			.filter { !$0.isEmpty }
			.joined(separator: " · ")
	}

	private func builderAvailableAgentChoices(
		_ session: LocalProductBuilderSession
	) -> [LocalProductRoleOptionChoice] {
		guard let setup = store.setupSnapshot else { return [] }
		let selected = Set(session.preview.roles.map(\.agentDefinitionID))
		var seen = Set<String>()
		return LocalProductRoleOptionChoice.all(
			setup: setup,
			preview: session.preview
		).filter { choice in
			guard choice.field == "subagent_role",
				!selected.contains(choice.agentDefinitionID),
				!seen.contains(choice.agentDefinitionID) else {
				return false
			}
			seen.insert(choice.agentDefinitionID)
			return true
		}
	}

	private func builderProviderAccountChoiceLabel(
		_ choice: LocalProductRoleOptionChoice
	) -> String {
		let harness = builderHarnessLabel(choice.harness)
		let account = choice.providerAccount.isEmpty
			? choice.provider
			: "\(choice.provider) / \(choice.providerAccount)"
		return [harness, account, choice.credential, choice.model]
			.filter { !$0.isEmpty }
			.joined(separator: " · ")
	}

	private func builderHarnessLabel(_ value: String) -> String {
		switch value.lowercased() {
		case "codex": return "Codex"
		case "claude-code", "claude code": return "Claude Code"
		case "loom-native", "loom native": return "Loom Native"
		case "opencode", "open code": return "OpenCode"
		case "pi": return "Pi"
		default: return value
		}
	}

	private func uniqueBuilderRouteChoices(
		_ choices: [LocalProductRoleOptionChoice],
		identity: (LocalProductRoleOptionChoice) -> String
	) -> [LocalProductRoleOptionChoice] {
		var seen: Set<String> = []
		return choices.filter { seen.insert(identity($0)).inserted }
	}

	private func builderEnrollmentChoices(
		_ role: LocalProductBuilderRole
	) -> [LocalProductRemoteToolBackendEnrollment] {
		guard let snapshot = store.setupSnapshot else { return [] }
		let account = snapshot.providerAccounts.first {
			$0.providerID == role.providerID &&
			$0.providerAccountID == role.providerAccountID
		}
		return (account?.remoteToolBackends ?? []).filter {
			$0.status == "active" && $0.policyCurrent
		}
	}

	private func builderEnrollmentChoiceLabel(
		_ enrollment: LocalProductRemoteToolBackendEnrollment
	) -> String {
		let kind = enrollment.backendKind == "mcp_server"
			? "MCP"
			: "Web Search"
		let server = enrollment.mcpServerID.isEmpty
			? ""
			: " · \(enrollment.mcpServerID)"
		return "\(kind) · \(enrollment.enrollmentID)\(server)"
	}

	private func builderEnrollmentChoiceSummary(
		role: LocalProductBuilderRole,
		enrollments: [LocalProductRemoteToolBackendEnrollment]
	) -> String {
		if let enrollment = enrollments.first(where: {
			$0.enrollmentID == role.remoteToolEnrollmentID
		}) {
			return builderEnrollmentChoiceLabel(enrollment)
		}
		return role.remoteToolEnrollmentID.isEmpty
			? "No remote tools"
			: "Review binding"
	}

    private var topologyPanel: some View {
        VStack(alignment: .leading, spacing: 12) {
            let missions = (store.snapshot?.missions ?? []).filter {
                !$0.topology.isEmpty
            }
            if missions.isEmpty {
                emptyPanelState("No Team topology yet", symbol: "point.3.connected.trianglepath.dotted")
            } else {
                ForEach(missions.prefix(4)) { mission in
                    Text(presentedMissionTitle(mission))
                        .font(.caption.weight(.semibold))
                        .foregroundStyle(.secondary)
                    ForEach(Array(mission.topology.prefix(5).enumerated()), id: \.offset) { _, node in
                        inspectorRow(
                            title: LocalProductExperience.visibleName(
                                node.title,
                                internalID: node.logicalNodeID,
                                fallback: humanized(node.role)
                            ),
                            detail: node.dependsOn.isEmpty
                                ? humanized(node.role)
                                : "\(humanized(node.role)) · \(node.dependsOn.count) dependencies",
                            symbol: "point.3.connected.trianglepath.dotted"
                        )
                    }
                }
            }
        }
    }

    private var timelinePanel: some View {
        VStack(alignment: .leading, spacing: 0) {
            let runs = store.snapshot?.runs ?? []
            if runs.isEmpty {
                emptyPanelState("No Agent activity yet", symbol: "clock.arrow.circlepath")
            } else {
                ForEach(runs.prefix(10)) { run in
                    inspectorRow(
                        title: humanized(run.phase, fallback: "Agent activity"),
                        detail: run.terminalStatus.isEmpty
                            ? "In progress"
                            : humanized(run.terminalStatus),
                        symbol: "clock.arrow.circlepath"
                    )
                }
            }
        }
    }

    private var decisionsPanel: some View {
        VStack(alignment: .leading, spacing: 0) {
            let decisions = store.snapshot?.preparedDecisions ?? []
            if decisions.isEmpty {
                emptyPanelState("No decisions need review", symbol: "checkmark.circle")
            } else {
                ForEach(decisions, id: \.decisionID) { decision in
                    Button {
                        Task {
                            if let mission = store.snapshot?.missions.first(
                                where: { $0.missionID == decision.missionID }
                            ) {
                                await openMissionContext(mission)
                            }
                            await store.openPreparedDecision(decision)
                        }
                    } label: {
                        inspectorRow(
                            title: humanized(decision.kind.rawValue),
                            detail: humanized(decision.action),
                            symbol: "questionmark.diamond"
                        )
                    }
                    .buttonStyle(.plain)
                    .accessibilityLabel("Open \(humanized(decision.kind.rawValue)) decision")
                }
            }
        }
    }

    private var evidencePanel: some View {
        VStack(alignment: .leading, spacing: 0) {
            if (store.snapshot?.evidence ?? []).isEmpty {
                emptyPanelState("No accepted evidence yet", symbol: "checkmark.seal")
            } else {
                ForEach(store.snapshot?.evidence ?? []) { evidence in
                    inspectorRow(
                        title: "Accepted evidence",
                        detail: SafeText.sanitize(evidence.workItemID, limit: 96),
                        symbol: "checkmark.seal"
                    )
                }
                if store.snapshot?.evidencePage.hasMore == true {
                    Text("Showing the 64 most recent records")
                        .font(.caption2)
                        .foregroundStyle(.secondary)
                        .padding(.horizontal, 12)
                        .padding(.vertical, 4)
                }
            }
        }
    }

    private var runtimePanel: some View {
        VStack(alignment: .leading, spacing: 0) {
            if (store.snapshot?.runtimes ?? []).isEmpty {
                emptyPanelState("No runtimes discovered", symbol: "cpu")
            } else {
                ForEach(store.snapshot?.runtimes ?? []) { runtime in
                    inspectorRow(
                        title: LocalProductExperience.visibleName(
                            runtime.displayName,
                            internalID: runtime.runtimeInstanceID,
                            fallback: "Runtime"
                        ),
                        detail: runtimeInspectorDetail(
                            status: humanized(runtime.status),
                            capacity: runtime.capacity,
                            adapterType: runtime.adapterType,
                            hasConversationRoute: store.setupSnapshot?.conversationProfiles.contains {
                                $0.harnessAdapter == "pi"
                            } ?? false
                        ),
                        symbol: "cpu"
                    )
                }
            }
            Divider()
                .padding(.vertical, 12)
            Button {
                fullGovernancePresentation = .runtimeProviders
            } label: {
                Label("Manage Runtimes & Providers", systemImage: "slider.horizontal.3")
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
            .buttonStyle(.bordered)
        }
    }

    private var attentionPanel: some View {
        let teams = store.snapshot?.teams ?? []
        let missions = store.snapshot?.missions ?? []
        let attention = store.snapshot?.attention ?? []
        let actionable = actionableAttention(
            teams: teams, missions: missions, attention: attention
        )
        let historical = historicalAttention(
            teams: teams, missions: missions, attention: attention
        )
        return VStack(alignment: .leading, spacing: 0) {
            if attention.isEmpty {
                emptyPanelState("Nothing needs your attention", symbol: "checkmark.circle")
            } else {
                if actionable.isEmpty {
                    emptyPanelState("Nothing needs your attention", symbol: "checkmark.circle")
                } else {
                    ForEach(actionable) { item in
                        attentionRow(item)
                    }
                }
                if !historical.isEmpty {
                    Divider()
                    Label(
                        "History (\(historical.count)) — archived teams or completed Missions",
                        systemImage: "clock.arrow.circlepath"
                    )
                        .font(.caption)
                        .foregroundStyle(.tertiary)
                        .padding(.top, 8)
                    ForEach(historical.prefix(6)) { item in
                        attentionRow(item)
                            .opacity(0.62)
                    }
                    if historical.count > 6 {
                        Text("+\(historical.count - 6) more in Mission history")
                            .font(.caption2)
                            .foregroundStyle(.tertiary)
                    }
                }
            }
        }
    }

    private func attentionRow(_ item: LocalProductAttention) -> some View {
        Button {
            Task { await openAttentionContext(item) }
        } label: {
            inspectorRow(
                title: attentionActionTitle(
                    actionRequired: item.actionRequired,
                    kind: item.kind
                ),
                detail: humanized(item.severity),
                symbol: "exclamationmark.triangle"
            )
        }
        .buttonStyle(.plain)
        .accessibilityLabel("Open item needing attention")
    }

    private func openAttentionContext(
        _ item: LocalProductAttention
    ) async {
        let candidates = (store.snapshot?.missions ?? []).filter {
            $0.teamInstanceID == item.teamInstanceID
        }
        let mission = candidates.first(where: {
            $0.attentionCount > 0
                && !["succeeded", "cancelled"].contains($0.status)
        }) ?? candidates.first
        if let mission, await openMissionContext(mission) {
            return
        } else {
            store.showMissionAttention()
            governance.open(.attention)
        }
    }

    @discardableResult
    private func openMissionContext(
        _ mission: LocalProductMissionSummary
    ) async -> Bool {
        await openMissionContext(mission.missionID)
    }

    @discardableResult
    private func openMissionContext(_ missionID: String) async -> Bool {
        guard await store.openMissionAndActivate(missionID) else {
            return false
        }
        alignConversationWithMission(missionID)
        selectedNavigation = .work
        governance.open(.mission)
        return true
    }

    private func alignConversationWithMission(_ missionID: String) {
        guard let conversation = store.conversationLinkedToMission(missionID),
              conversation.threadID != store.currentChatThreadID()
        else { return }
        store.selectChatSession(conversation.threadID)
    }

    private var libraryPanel: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Agent definitions, skills, and governed evolution assets.")
                .foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)
            Button {
                store.showMissionLibrary()
                fullGovernancePresentation = .workbench
            } label: {
                Label("Open Library", systemImage: "books.vertical")
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
            .buttonStyle(.bordered)
        }
    }

    private func metricRow(_ title: String, value: String, symbol: String) -> some View {
        HStack(spacing: 10) {
            Image(systemName: symbol)
                .foregroundStyle(.secondary)
                .frame(width: 20)
            Text(title)
            Spacer()
            Text(value)
                .font(.body.monospacedDigit().weight(.semibold))
        }
        .frame(minHeight: 44)
        .overlay(alignment: .bottom) { Divider() }
        .accessibilityElement(children: .combine)
    }

    private func inspectorRow(title: String, detail: String, symbol: String) -> some View {
        HStack(alignment: .top, spacing: 10) {
            Image(systemName: symbol)
                .foregroundStyle(.secondary)
                .frame(width: 20)
            VStack(alignment: .leading, spacing: 3) {
                Text(title)
                    .font(.callout.weight(.medium))
                    .lineLimit(2)
                Text(detail)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .lineLimit(2)
            }
            Spacer(minLength: 0)
        }
        .padding(.vertical, 10)
        .frame(maxWidth: .infinity, alignment: .leading)
        .overlay(alignment: .bottom) { Divider() }
        .contentShape(Rectangle())
        .accessibilityElement(children: .combine)
    }

    private func emptyPanelState(_ title: String, symbol: String) -> some View {
        VStack(spacing: 10) {
            Image(systemName: symbol)
                .font(.title2)
                .foregroundStyle(.secondary)
            Text(title)
                .font(.callout)
                .foregroundStyle(.secondary)
        }
        .frame(maxWidth: .infinity, minHeight: 180)
        .accessibilityElement(children: .combine)
    }

    private var experience: LocalProductExperience {
        LocalProductExperience(
            snapshot: store.snapshot,
            connectionState: store.connectionState
        )
    }

    private var serviceFooterTitle: String {
        switch experience.state {
        case .connectedEmpty, .connectedPopulated:
            return "Local service ready"
        default:
            return experience.connection.title
        }
    }

    private var activeMissionCount: Int {
        countActiveMissions(
            teams: store.snapshot?.teams ?? [],
            missions: store.snapshot?.missions ?? []
        )
    }

    /// "Needs you" should only reflect actionable items: attention on an
    /// executable Team whose Mission is still active (non-Complete). Historical
    /// or archived-Team attention is not something the user must act on.
    private var activeAttentionCount: Int {
        countActiveAttention(
            teams: store.snapshot?.teams ?? [],
            missions: store.snapshot?.missions ?? [],
            attention: store.snapshot?.attention ?? []
        )
    }

    /// A Mission is actionable only while its Team is executable; archiving a
    /// Team moves its Missions out of the active count and active board lanes
    /// (they stay visible in the Complete lane as history).
    private func missionTeamExecutable(
        _ mission: LocalProductMissionSummary
    ) -> Bool {
        guard !mission.teamInstanceID.isEmpty,
              let teams = store.snapshot?.teams else {
            return false
        }
        return teams.contains {
            $0.teamInstanceID == mission.teamInstanceID && $0.executable
        }
    }

    private var sendButtonEnabled: Bool {
        LocalProductStore.canSubmitChatMessage(
            content: store.workspace.selectedContinuity.composerDraft,
            isSending: store.isSendingChatMessage,
            profileID: store.selectedConversationProfileID
        )
    }

    private var conversationComposerSubmissionEnabled: Bool {
        let draft = store.workspace.selectedContinuity.composerDraft
        guard !draft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            return false
        }
        switch ConversationActionRouter.resolve(draft) {
        case .request, .unknownSlashCommand, .rejectedActionArgument:
            return true
        case .chat:
            return draft.trimmingCharacters(in: .whitespacesAndNewlines)
                .hasPrefix("/") || sendButtonEnabled
        }
    }

    private var stripBackground: Color {
        switch experience.state {
        case .offline, .offlinePreserved:
            return LoomGraphite.statusWarning.opacity(0.12)
        case .fatal:
            return LoomGraphite.statusDanger.opacity(0.12)
        default:
            return LoomGraphite.accentMuted
        }
    }

    private var indicatorColor: Color {
        switch experience.state {
        case .loading:
            return LoomGraphite.accent
        case .connectedEmpty, .connectedPopulated:
            return LoomGraphite.statusSuccess
        case .partial, .stalePreserved:
            return LoomGraphite.statusWarning
        case .offline, .offlinePreserved, .fatal:
            return LoomGraphite.statusDanger
        }
    }

    private func governanceSymbol(_ destination: LoomGovernanceDestination) -> String {
        switch destination {
        case .overview: return "chart.bar.xaxis"
        case .board: return "square.3.layers.3d"
        case .mission: return "flag.checkered"
        case .roundtable: return "person.2.wave.2"
        case .team: return "person.3"
        case .topology: return "point.3.connected.trianglepath.dotted"
        case .timeline: return "clock.arrow.circlepath"
        case .decisions: return "questionmark.diamond"
        case .evidence: return "checkmark.seal"
        case .runtimes: return "cpu"
        case .attention: return "exclamationmark.triangle"
        case .library: return "books.vertical"
        }
    }

    private func humanized(_ value: String, fallback: String = "Available") -> String {
        let safe = SafeText.sanitize(value, limit: 96)
            .replacingOccurrences(of: "_", with: " ")
            .replacingOccurrences(of: "-", with: " ")
            .trimmingCharacters(in: .whitespaces)
        guard !safe.isEmpty else { return fallback }
        return safe.prefix(1).uppercased() + safe.dropFirst()
    }

    private func handleFolderSelection(_ result: Result<[URL], Error>) {
        let url: URL
        switch result {
        case .success(let urls):
            guard let selected = urls.first else { return }
            url = selected
        case .failure(let error):
            if (error as NSError).code == NSUserCancelledError { return }
            folderSelectionError =
                "Loom could not access the selected workspace folder. Try choosing it again."
            return
        }
        let scoped = url.startAccessingSecurityScopedResource()
        defer {
            if scoped {
                url.stopAccessingSecurityScopedResource()
            }
        }
        store.selectWorkspaceFolder(url)
        folderSelectionError = nil
    }
}

/// Humanized board detail for a Mission row. A Mission that reached the
/// "Complete" lane already tells the user it finished, so a raw
/// "Complete · failed" reads contradictory; lead with the humanized terminal
/// outcome instead ("Failed" / "Blocked" / "Succeeded"). Active lanes keep
/// both lane and state ("Orchestrating · Running").
func missionBoardDetailText(lane: String, status: String) -> String {
    let humanizedStatus = status.isEmpty
        ? ""
        : missionHumanStatus(status)
    if lane == "Complete" {
        return humanizedStatus.isEmpty ? "Complete" : humanizedStatus
    }
    if lane.isEmpty {
        return humanizedStatus
    }
    if humanizedStatus.isEmpty {
        return lane
    }
    return "\(lane) · \(humanizedStatus)"
}

func runtimeAvailabilityDetail(
    modelCount: Int,
    status: String,
    ready: Bool
) -> String {
    let count = max(modelCount, 0)
    let normalizedStatus = status.trimmingCharacters(in: .whitespacesAndNewlines)
    if ready {
        return "\(count) model\(count == 1 ? "" : "s") · \(normalizedStatus)"
    }
    return normalizedStatus.isEmpty
        ? "Unavailable · reopen Loom after fixing the executable"
        : "\(normalizedStatus) · reopen Loom after fixing the executable"
}

func runtimeInspectorDetail(
    status: String,
    capacity: Int,
    adapterType: String,
    hasConversationRoute: Bool
) -> String {
    if adapterType == "pi-cli" && !hasConversationRoute {
        return "\(status) · Local model required for Conversation"
    }
    return "\(status) · Capacity \(max(capacity, 0))"
}

func runtimeAvailabilityAccessibilityLabel(
    displayName: String,
    ready: Bool
) -> String {
    let safeName = displayName.trimmingCharacters(in: .whitespacesAndNewlines)
    return "\(safeName.isEmpty ? "Harness" : safeName), \(ready ? "available" : "unavailable")"
}

/// Builds a copyable transcript of a conversation: one "Speaker: content" line
/// per message, blank-line separated, so the whole thread can be copied at once.
func conversationTranscriptText(_ messages: [LocalProductChatMessage]) -> String {
    let parts = messages.map { message -> String in
        let speaker: String
        switch message.role {
        case "user": speaker = "You"
        case "proposal": speaker = "Loom proposal"
        default: speaker = "Loom"
        }
        return "\(speaker): \(message.displayContent)"
    }
    return parts.joined(separator: "\n\n")
}

/// Short date suffix appended to a sidebar conversation title when another
/// session shares the same auto-title: time-of-day for today, otherwise the
/// month + day (for example "hello · 4:30 PM" / "hello · Aug 20").
func conversationDisambiguationSuffix(for date: Date, now: Date = Date()) -> String {
    let formatter = DateFormatter()
    if Calendar.current.isDate(date, inSameDayAs: now) {
        formatter.dateFormat = "h:mm a"
    } else {
        formatter.dateFormat = "MMM d"
    }
    return formatter.string(from: date)
}

/// Builds a Mission objective from a chat proposal, falling back to the
/// latest user message. A proposal longer than the Mission objective limit is
/// truncated to the limit so the New Mission sheet starts with a useful draft
/// instead of an empty field.
func chatMissionObjectiveText(
    around message: LocalProductChatMessage,
    thread: LocalProductChatThread?
) -> String {
    let limit = 4_096
    let trimmedProposal = message.displayContent.trimmingCharacters(
        in: .whitespacesAndNewlines
    )
    if !trimmedProposal.isEmpty {
        return String(trimmedProposal.prefix(limit))
    }
    for candidate in (thread?.messages ?? []).reversed() {
        if candidate.role == "user" {
            let value = candidate.displayContent.trimmingCharacters(
                in: .whitespacesAndNewlines
            )
            if !value.isEmpty {
                return String(value.prefix(limit))
            }
        }
    }
    return ""
}

/// Latest conversation context to pre-fill a Mission when the user turns a
/// chat into a governed Mission from the composer or navigation rail. Prefers
/// the newest message that is a proposal or a user ask (in that order of
/// recency), so a Loom/assistant reply is never used as the Mission objective.
/// The result is truncated to the Mission objective limit. An empty or missing
/// thread, or a thread with no user ask or proposal, produces an empty
/// objective.
func composerNewMissionObjective(thread: LocalProductChatThread?) -> String {
    guard let messages = thread?.messages, !messages.isEmpty else { return "" }
    for candidate in messages.reversed() {
        if candidate.role == "proposal" || candidate.role == "user" {
            return chatMissionObjectiveText(around: candidate, thread: thread)
        }
    }
    return ""
}


/// Counts Missions a user can actually act on: not Complete and on an
/// executable (non-archived) Team. Archiving a Team moves its Missions out of
/// the active count.
func countActiveMissions(
  teams: [LocalProductTeamSummary],
  missions: [LocalProductMissionSummary]
) -> Int {
  let executableTeams = Set(
    teams.filter { $0.executable }.map { $0.teamInstanceID }
  )
  return missions.filter {
    $0.lane != "Complete" && executableTeams.contains($0.teamInstanceID)
  }.count
}


/// Attention items a user can actually act on: on an executable Team whose
/// Mission is still active (non-Complete). Items on archived Teams or on
/// Teams whose Missions are already Complete are history, not action.
func actionableAttention(
  teams: [LocalProductTeamSummary],
  missions: [LocalProductMissionSummary],
  attention: [LocalProductAttention]
) -> [LocalProductAttention] {
  let executableTeams = Set(
    teams.filter { $0.executable }.map { $0.teamInstanceID }
  )
  let activeMissionTeams = Set(
    missions.filter { $0.lane != "Complete" }.map { $0.teamInstanceID }
  )
  return attention.filter {
    executableTeams.contains($0.teamInstanceID)
      && activeMissionTeams.contains($0.teamInstanceID)
  }
}

/// The complement of `actionableAttention`: items on archived Teams or on
/// Teams whose Missions are already Complete (preserved for the governance
/// trail, but not something the user must act on).
func historicalAttention(
  teams: [LocalProductTeamSummary],
  missions: [LocalProductMissionSummary],
  attention: [LocalProductAttention]
) -> [LocalProductAttention] {
  let actionable = Set(
    actionableAttention(
      teams: teams, missions: missions, attention: attention
    ).map(\.attentionID)
  )
  return attention.filter { !actionable.contains($0.attentionID) }
}

/// One quick-start checklist step. Pending steps are plain imperatives
/// (no "1." / "2." numbering, which left an orphaned "1." when the second
/// step was already done); ready steps show their done label, substituting
/// the optional value into "%@" when present.
func quickStartStepLabel(
    ready: Bool,
    value: String,
    done: String,
    pending: String
) -> String {
    if !ready {
        return SafeText.sanitize(pending, limit: 160)
    }
    if done.contains("%@") {
        return SafeText.sanitize(done.replacingOccurrences(of: "%@", with: value), limit: 160)
    }
    return SafeText.sanitize(done, limit: 160)
}

/// Humanized title for an attention item: prefer `action_required`, fall back
/// to `kind`, then to a generic prompt.
func attentionActionTitle(actionRequired: String, kind: String) -> String {
  let safe = SafeText.sanitize(
    actionRequired.isEmpty ? kind : actionRequired,
    limit: 120
  )
    .replacingOccurrences(of: "_", with: " ")
    .replacingOccurrences(of: "-", with: " ")
    .trimmingCharacters(in: .whitespaces)
  if safe.isEmpty {
    return "Needs your attention"
  }
  return safe.prefix(1).uppercased() + safe.dropFirst()
}

/// "Needs you" should only reflect actionable items: attention on an
/// executable Team whose Mission is still active (non-Complete).
func countActiveAttention(
  teams: [LocalProductTeamSummary],
  missions: [LocalProductMissionSummary],
  attention: [LocalProductAttention]
) -> Int {
  let executableTeams = Set(
    teams.filter { $0.executable }.map { $0.teamInstanceID }
  )
  let activeMissionTeams = Set(
    missions.filter { $0.lane != "Complete" }.map { $0.teamInstanceID }
  )
  return attention.filter {
    executableTeams.contains($0.teamInstanceID)
      && activeMissionTeams.contains($0.teamInstanceID)
  }.count
}
