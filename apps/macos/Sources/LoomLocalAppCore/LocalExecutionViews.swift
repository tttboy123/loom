import SwiftUI

// B-W1 read-only execution explorer for the native app.
public struct ExecutionExplorerView: View {
    public let snapshot: ExecutionSnapshot

    public init(snapshot: ExecutionSnapshot) {
        self.snapshot = snapshot
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("Execution · bounded execution adapter")
                .font(.title3.weight(.semibold))
                .foregroundStyle(LoomGraphite.textPrimary)
            if snapshot.records.isEmpty {
                Text("No executions yet. Propose a tool call from the TUI.")
                    .font(.caption)
                    .foregroundStyle(LoomGraphite.textSecondary)
            }
            List(snapshot.records, id: \.executionID) { record in
                VStack(alignment: .leading, spacing: 4) {
                    Text("\(record.jobID) · \(record.tool) · \(record.status)")
                        .font(.body)
                        .foregroundStyle(LoomGraphite.textPrimary)
                    Text("execution \(record.executionID) · exit \(record.exitCode ?? -1)")
                        .font(.caption)
                        .foregroundStyle(LoomGraphite.textSecondary)
                        .monospaced()
                    if let denial = record.denialReason, !denial.isEmpty {
                        Text("Denied · \(denial)")
                            .font(.caption)
                            .foregroundStyle(LoomGraphite.statusDanger)
                    }
                    if let failure = record.failureReason, !failure.isEmpty {
                        Text("Failed · \(failure)")
                            .font(.caption)
                            .foregroundStyle(LoomGraphite.statusDanger)
                    }
                    if record.status == "recovery_required" {
                        Text("Result unknown · Review before retrying")
                            .font(.caption.weight(.semibold))
                            .foregroundStyle(LoomGraphite.statusWarning)
                        Text("Incident \(record.journeyID)")
                            .font(.caption2)
                            .foregroundStyle(LoomGraphite.textSecondary)
                            .monospaced()
                    }
                }
                .padding(.vertical, 4)
            }
        }
        .padding(16)
        .background(LoomGraphite.surface)
    }
}
