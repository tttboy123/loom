import SwiftUI

// C-W1 read-only production status view for the native app.
public struct ProductionStatusView: View {
    public let snapshot: ProductionSnapshot

    public init(snapshot: ProductionSnapshot) {
        self.snapshot = snapshot
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("Production · resident daemon")
                .font(.title3.weight(.semibold))
                .foregroundStyle(LoomGraphite.textPrimary)
            Text(snapshot.activated
                 ? "Activated · \(snapshot.targetMode ?? "default")"
                 : "Inactive")
                .font(.body)
                .foregroundStyle(LoomGraphite.textSecondary)
            if snapshot.recovery.degraded {
                Text("DEGRADED read-only · \(snapshot.recovery.reason ?? "config mismatch")")
                    .font(.caption)
                    .foregroundStyle(LoomGraphite.statusDanger)
            }
            Text("View version \(snapshot.viewVersion)")
                .font(.caption)
                .foregroundStyle(LoomGraphite.textMuted)
                .monospaced()
                .lineLimit(1)
        }
        .padding(16)
        .background(LoomGraphite.surface)
    }
}
