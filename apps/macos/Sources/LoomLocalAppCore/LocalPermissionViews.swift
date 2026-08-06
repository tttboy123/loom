import SwiftUI

// B-P1 permission surface for the native app. Read-only: renders the
// Journal-authoritative explorer/attention projections; all permission
// mutations are performed through the TUI / Go service layer.

public struct PermissionExplorerView: View {
    public let snapshot: PermissionSnapshot
    public let attention: PermissionAttention

    public init(snapshot: PermissionSnapshot, attention: PermissionAttention) {
        self.snapshot = snapshot
        self.attention = attention
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            header
            List {
                profilesSection
                bindingsSection
                rulesSection
                attentionSection
            }
            .listStyle(.inset)
        }
        .background(LoomGraphite.canvas)
    }

    private var header: some View {
        HStack(spacing: 12) {
            Image(systemName: "lock.shield")
                .font(.title2)
                .foregroundStyle(LoomGraphite.accent)
                .accessibilityHidden(true)
            VStack(alignment: .leading, spacing: 3) {
                Text("Permissions")
                    .font(.title3.weight(.semibold))
                Text("Tool-call authorization, read-only view")
                    .font(.caption)
                    .foregroundStyle(LoomGraphite.textSecondary)
            }
            Spacer()
            if snapshot.adminLock {
                Label("Admin lock", systemImage: "lock.fill")
                    .font(.caption.weight(.bold))
                    .foregroundStyle(LoomGraphite.statusWarning)
                    .padding(.horizontal, 10)
                    .padding(.vertical, 6)
                    .background(
                        LoomGraphite.statusWarning.opacity(0.14),
                    in: Capsule()
                )
                .help("Bypass permissions is blocked by the admin lock")
            }
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 12)
        .background(LoomGraphite.rail)
    }

    @ViewBuilder
    private var profilesSection: some View {
        Section("Profiles") {
            if snapshot.profiles.isEmpty {
                ContentUnavailableView(
                    "No permission profiles",
                    systemImage: "lock.open",
                    description: Text("Define one from the TUI Permissions screen.")
                        .foregroundStyle(LoomGraphite.textSecondary)
                )
            } else {
                ForEach(snapshot.profiles, id: \.profileID) { profile in
                    VStack(alignment: .leading, spacing: 4) {
                        Text(profile.profileID)
                            .font(.body.weight(.semibold))
                            .foregroundStyle(LoomGraphite.textPrimary)
                            .textSelection(.enabled)
                        Text("Generation \(profile.generation) · \(profile.mode)")
                            .font(.caption)
                            .foregroundStyle(LoomGraphite.textSecondary)
                            .monospacedDigit()
                    }
                    .padding(.vertical, 4)
                }
            }
        }
    }

    @ViewBuilder
    private var bindingsSection: some View {
        Section("Bindings") {
            if snapshot.bindings.isEmpty {
                ContentUnavailableView(
                    "No job bindings",
                    systemImage: "link",
                    description: Text("Bind a Queue job to a profile to apply its rules.")
                        .foregroundStyle(LoomGraphite.textSecondary)
                )
            } else {
                ForEach(snapshot.bindings, id: \.jobID) { binding in
                    HStack(spacing: 8) {
                        Text(binding.jobID)
                            .font(.callout.monospaced())
                            .foregroundStyle(LoomGraphite.textPrimary)
                            .lineLimit(1)
                        Image(systemName: "arrow.right")
                            .font(.caption2)
                            .foregroundStyle(LoomGraphite.textSecondary)
                            .accessibilityHidden(true)
                        Text(binding.profileID)
                            .font(.callout)
                            .foregroundStyle(LoomGraphite.textPrimary)
                            .lineLimit(1)
                    }
                    .padding(.vertical, 4)
                }
            }
        }
    }

    @ViewBuilder
    private var rulesSection: some View {
        Section("Rules") {
            if snapshot.rules.isEmpty {
                ContentUnavailableView(
                    "No permission rules",
                    systemImage: "line.3.horizontal.decrease.circle",
                    description: Text("Rules are added through the TUI or the Go service layer.")
                        .foregroundStyle(LoomGraphite.textSecondary)
                )
            } else {
                ForEach(snapshot.rules, id: \.ruleID) { rule in
                    HStack(spacing: 8) {
                        Text(rule.action)
                            .font(.caption.weight(.bold))
                            .foregroundStyle(ruleActionColor(rule.action))
                            .frame(width: 56, alignment: .leading)
                        Text("\(rule.tool) \(rule.pattern)")
                            .font(.callout.monospaced())
                            .foregroundStyle(LoomGraphite.textPrimary)
                            .lineLimit(1)
                            .textSelection(.enabled)
                    }
                    .padding(.vertical, 4)
                }
            }
        }
    }

    @ViewBuilder
    private var attentionSection: some View {
        Section("Attention") {
            if attention.approvals.isEmpty && attention.decisions.isEmpty {
                ContentUnavailableView(
                    "Nothing needs approval",
                    systemImage: "checkmark.circle",
                    description: Text("Pending tool-call approvals and decisions appear here.")
                        .foregroundStyle(LoomGraphite.textSecondary)
                )
            } else {
                ForEach(attention.approvals, id: \.approvalID) { approval in
                    Label(
                        approval.command ?? approval.status,
                        systemImage: "clock"
                    )
                    .font(.callout)
                    .foregroundStyle(LoomGraphite.textPrimary)
                    .lineLimit(1)
                }
                ForEach(attention.decisions, id: \.jobID) { decision in
                    Label(decision.reason, systemImage: "exclamationmark.circle")
                        .font(.callout)
                        .foregroundStyle(LoomGraphite.statusWarning)
                        .lineLimit(1)
                }
            }
        }
    }

    private func ruleActionColor(_ action: String) -> Color {
        switch action {
        case "deny": return LoomGraphite.statusDanger
        case "ask": return LoomGraphite.statusWarning
        default: return LoomGraphite.statusSuccess
        }
    }
}
