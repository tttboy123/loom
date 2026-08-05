import SwiftUI

// B-W1 read-only execution explorer for the native app.
public struct ExecutionExplorerView: View {
    public let snapshot: ExecutionSnapshot

    public init(snapshot: ExecutionSnapshot) {
        self.snapshot = snapshot
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("Execution · bounded execution adapter")
                .font(.headline)
            if snapshot.records.isEmpty {
                Text("No executions yet. Propose a tool call from the TUI.")
                    .font(.caption)
                    .foregroundColor(.secondary)
            }
            List(snapshot.records, id: \.executionID) { record in
                VStack(alignment: .leading, spacing: 3) {
                    Text("\(record.jobID) · \(record.tool) · \(record.status)")
                        .font(.body)
                    Text("execution \(record.executionID) · exit \(record.exitCode ?? -1)")
                        .font(.caption)
                        .foregroundColor(.secondary)
                    if let denial = record.denialReason, !denial.isEmpty {
                        Text("Denied · \(denial)").font(.caption).foregroundColor(.red)
                    }
                    if let failure = record.failureReason, !failure.isEmpty {
                        Text("Failed · \(failure)").font(.caption).foregroundColor(.red)
                    }
                }
                .padding(.vertical, 2)
            }
        }
        .padding()
    }
}
