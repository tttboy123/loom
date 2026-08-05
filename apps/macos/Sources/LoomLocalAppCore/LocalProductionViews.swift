import SwiftUI

// C-W1 read-only production status view for the native app.
public struct ProductionStatusView: View {
    public let snapshot: ProductionSnapshot

    public init(snapshot: ProductionSnapshot) {
        self.snapshot = snapshot
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("Production · resident daemon")
                .font(.headline)
            Text(snapshot.activated
                 ? "Activated · \(snapshot.targetMode ?? "default")"
                 : "Inactive")
                .font(.body)
            if snapshot.recovery.degraded {
                Text("DEGRADED read-only · \(snapshot.recovery.reason ?? "config mismatch")")
                    .font(.caption)
                    .foregroundColor(.red)
            }
            Text("View version \(snapshot.viewVersion)")
                .font(.caption)
                .foregroundColor(.secondary)
        }
        .padding()
    }
}
