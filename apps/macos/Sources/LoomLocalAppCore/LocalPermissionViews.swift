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
        VStack(alignment: .leading, spacing: 8) {
            Text("Permissions · tool-call authorization")
                .font(.headline)
            if snapshot.adminLock {
                Text("Admin lock enabled — bypass permissions is blocked")
                    .font(.caption)
                    .foregroundColor(.red)
            }
            List {
                Section("Profiles") {
                    ForEach(snapshot.profiles, id: \.profileID) { profile in
                        Text("\(profile.profileID) · gen \(profile.generation) · \(profile.mode)")
                    }
                }
                Section("Bindings") {
                    ForEach(snapshot.bindings, id: \.jobID) { binding in
                        Text("\(binding.jobID) -> \(binding.profileID)")
                    }
                }
                Section("Rules") {
                    ForEach(snapshot.rules, id: \.ruleID) { rule in
                        Text("\(rule.action) \(rule.tool) \(rule.pattern)")
                    }
                }
                Section("Attention") {
                    ForEach(attention.decisions, id: \.jobID) { decision in
                        Text("\(decision.jobID) · \(decision.reason)")
                    }
                }
            }
        }
        .padding()
    }
}
