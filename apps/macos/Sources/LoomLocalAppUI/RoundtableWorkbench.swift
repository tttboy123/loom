import SwiftUI
import LoomLocalAppCore

// Governed Handoff / Roundtable workbench: a self-guiding dual-seat
// moderator journey. The moderator creates a session, adds a writer and a
// target seat, opens a round, then walks the message through
// propose -> relay -> acknowledge -> insert and finally concludes with a
// digest-bound AlignmentSummary artifact.
public struct RoundtableWorkbench: View {
    @ObservedObject private var store: LocalProductStore
    @Environment(\.dismiss) private var dismiss

    @State private var sessionID = RoundtableWorkbench.buildSuggestedSessionID()
    @State private var resumeSessionID = ""
    @State private var title = "Diagnosis handoff"
    @State private var messageBody = "Diagnosis: the daemon build_execution path is over-constrained; recommend a bounded retry."
    @State private var view: LocalRoundtableView?
    @State private var busy = false

    private let moderatorSeat = "seat-moderator"
    private let writerSeat = "seat-writer"
    private let targetSeat = "seat-target"
    private let roundID = "round-1"
    private let messageID = "msg-1"

    public init(store: LocalProductStore) {
        self.store = store
    }

    /// Pre-fill a usable, time-based session id so the moderator does not have
    /// to hand-craft one. Users can still overwrite it before creating.
    static func buildSuggestedSessionID(
        from date: Date = Date()
    ) -> String {
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: "en_US_POSIX")
        formatter.timeZone = .current
        formatter.dateFormat = "yyyyMMdd-HHmm"
        return "rt-" + formatter.string(from: date)
    }

    private enum Step: Int, CaseIterable {
        case create = 0
        case seats
        case round
        case propose
        case relay
        case acknowledge
        case insert
        case conclude

        var title: String {
            switch self {
            case .create: return "Create session"
            case .seats: return "Add writer + target seats"
            case .round: return "Open round"
            case .propose: return "Propose (writer)"
            case .relay: return "Relay (moderator)"
            case .acknowledge: return "Acknowledge (target)"
            case .insert: return "Insert (moderator)"
            case .conclude: return "Conclude (moderator)"
            }
        }

        var symbol: String {
            switch self {
            case .create: return "plus.circle"
            case .seats: return "person.2"
            case .round: return "circle.dashed"
            case .propose: return "square.and.pencil"
            case .relay: return "arrow.right.circle"
            case .acknowledge: return "checkmark.circle"
            case .insert: return "text.badge.checkmark"
            case .conclude: return "flag.checkered"
            }
        }
    }

    private var currentStep: Step {
        guard let view else { return .create }
        if view.session.concluded { return .conclude }
        if view.seats.count < 3 { return .seats }
        if view.rounds.isEmpty { return .round }
        if view.messages.isEmpty { return .propose }
        guard let message = view.messages[messageID] else { return .propose }
        switch message.status {
        case "pending": return .relay
        case "relayed": return .acknowledge
        case "acknowledged": return .insert
        default: return .conclude
        }
    }

    private func stepState(_ step: Step) -> StepState {
        let current = currentStep.rawValue
        let stepIndex = step.rawValue
        if stepIndex < current { return .completed }
        if stepIndex == current { return .current }
        return .pending
    }

    public var body: some View {
        VStack(spacing: 0) {
            header
            Divider()
            ScrollView {
                VStack(alignment: .leading, spacing: 16) {
                    if store.roundtableError != nil {
                        errorBanner
                    }
                    if view == nil {
                        createSessionForm
                    } else {
                        journeyPanel
                        sessionSnapshot
                    }
                }
                .padding(20)
                .frame(maxWidth: .infinity, alignment: .leading)
            }
            .background(LoomGraphite.canvas)
        }
        .background(LoomGraphite.canvas)
        .onAppear {
            if view == nil, let lastID = store.roundtableLastSessionID {
                Task {
                    if let loaded = await store.roundtableLoadSession(
                        sessionID: lastID
                    ) {
                        view = loaded
                        sessionID = loaded.session.id
                    }
                }
            }
        }
    }

    private var header: some View {
        HStack(spacing: 10) {
            Image(systemName: "person.2.wave.2")
                .foregroundStyle(LoomGraphite.accent)
            VStack(alignment: .leading, spacing: 2) {
                Text("Roundtable")
                    .font(.headline)
                Text("Governed handoff ledger")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            Spacer()
            if let view {
                Text(view.session.concluded ? "Concluded" : "Open")
                    .font(.caption.weight(.semibold))
                    .padding(.horizontal, 10)
                    .padding(.vertical, 4)
                    .background(
                        view.session.concluded
                            ? LoomGraphite.accent.opacity(0.16)
                            : Color.green.opacity(0.16),
                        in: Capsule()
                    )
            }
            Button("Close") { dismiss() }
                .keyboardShortcut(.cancelAction)
        }
        .padding(.horizontal, 18)
        .frame(height: 52)
        .background(LoomGraphite.surface)
    }

    private var errorBanner: some View {
        HStack(alignment: .top, spacing: 10) {
            Image(systemName: "exclamationmark.triangle.fill")
                .foregroundStyle(.orange)
            Text(store.roundtableError ?? "")
                .font(.callout)
                .textSelection(.enabled)
            Spacer()
        }
        .padding(12)
        .background(Color.orange.opacity(0.12), in: RoundedRectangle(cornerRadius: 10))
    }

    private var createSessionForm: some View {
        VStack(alignment: .leading, spacing: 14) {
            Text("New governed handoff")
                .font(.title3.weight(.semibold))
            Text("Create a session, then walk one bounded message through propose → relay → acknowledge → insert → conclude. Each hop is confirmed by a different seat; the final summary is stored as a digest-bound evidence artifact.")
                .font(.callout)
                .foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)
            TextField("Session ID (e.g. rt-diagnosis-1)", text: $sessionID)
                .textFieldStyle(.roundedBorder)
            TextField("Title", text: $title)
                .textFieldStyle(.roundedBorder)
            HStack {
                Button {
                    createSession()
                } label: {
                    if busy {
                        ProgressView().controlSize(.small)
                    } else {
                        Label("Create session", systemImage: "plus.circle")
                    }
                }
                .buttonStyle(.borderedProminent)
                .disabled(busy || sessionID.trimmingCharacters(in: .whitespaces).isEmpty)
                .help("Creates the roundtable session with the moderator seat")
            }

            Divider()

            VStack(alignment: .leading, spacing: 10) {
                Text("Resume a session")
                    .font(.subheadline.weight(.semibold))
                Text("Closing this panel keeps the session on the daemon. Reopen it by ID to continue or inspect the concluded summary.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)
                HStack(spacing: 8) {
                    TextField("Session ID to open", text: $resumeSessionID)
                        .textFieldStyle(.roundedBorder)
                    Button {
                        openSession()
                    } label: {
                        Label("Open", systemImage: "folder")
                    }
                    .buttonStyle(.bordered)
                    .disabled(busy || resumeSessionID.trimmingCharacters(in: .whitespaces).isEmpty)
                    .help("Opens the existing roundtable session by ID")
                }
                if let lastID = store.roundtableLastSessionID,
                   lastID != resumeSessionID {
                    Button {
                        resumeSessionID = lastID
                        openSession()
                    } label: {
                        Label("Resume last: \(lastID)", systemImage: "arrow.counterclockwise")
                    }
                    .buttonStyle(.plain)
                    .font(.caption)
                    .help("Reopens the most recently used session")
                }
            }
        }
        .padding(18)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(LoomGraphite.raised, in: RoundedRectangle(cornerRadius: 12))
    }

    private func openSession() {
        let id = resumeSessionID.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !id.isEmpty else { return }
        busy = true
        Task {
            defer { busy = false }
            if let loaded = await store.roundtableLoadSession(sessionID: id) {
                view = loaded
                sessionID = loaded.session.id
            }
        }
    }

    private var journeyPanel: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack {
                Text("Journey")
                    .font(.title3.weight(.semibold))
                Spacer()
                if currentStep != .conclude {
                    Button {
                        Task { await runNextStep() }
                    } label: {
                        Label(
                            currentStep == .create ? "Start" : "Next: \(currentStep.title)",
                            systemImage: "arrow.right.circle"
                        )
                    }
                    .buttonStyle(.borderedProminent)
                    .disabled(busy)
                }
                Button {
                    Task { await runFullJourney() }
                } label: {
                    Label("Run full journey", systemImage: "play.fill")
                }
                .buttonStyle(.bordered)
                .disabled(busy || view?.session.concluded == true)
            }
            VStack(spacing: 8) {
                ForEach(Step.allCases, id: \.self) { step in
                    journeyRow(step)
                }
            }
        }
        .padding(18)
        .background(LoomGraphite.raised, in: RoundedRectangle(cornerRadius: 12))
    }

    private func journeyRow(_ step: Step) -> some View {
        let state = stepState(step)
        return HStack(spacing: 10) {
            Image(systemName: state.symbol)
                .foregroundStyle(state.color)
                .frame(width: 22)
            Text(step.title)
                .font(.callout)
                .foregroundStyle(state == .pending ? Color.secondary : Color.primary)
            Spacer()
            if state == .current {
                Text("Next")
                    .font(.caption2.weight(.semibold))
                    .foregroundStyle(LoomGraphite.accent)
            } else if state == .completed {
                Text("Done")
                    .font(.caption2.weight(.semibold))
                    .foregroundStyle(.green)
            }
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 8)
        .background(
            state == .current ? LoomGraphite.accent.opacity(0.10) : Color.clear,
            in: RoundedRectangle(cornerRadius: 8)
        )
    }

    private var sessionSnapshot: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Session state")
                .font(.title3.weight(.semibold))
            if let view {
                HStack(spacing: 16) {
                    Label("\(view.seats.count) seats", systemImage: "person.2")
                        .font(.callout)
                    Label("\(view.rounds.count) rounds", systemImage: "circle.dashed")
                        .font(.callout)
                    Label("\(view.messages.count) messages", systemImage: "bubble.left")
                        .font(.callout)
                    Spacer()
                    Text("digest \(String(view.digest.prefix(12)))…")
                        .font(.caption.monospaced())
                        .foregroundStyle(.secondary)
                        .help(view.digest)
                }
                ForEach(view.seats.values.sorted(by: { $0.id < $1.id }), id: \.id) { seat in
                    HStack(spacing: 10) {
                        Image(systemName: seat.available ? "circle.fill" : "circle.slash")
                            .font(.caption2)
                            .foregroundStyle(seat.available ? Color.green : Color.secondary)
                        Text(seat.displayName)
                            .font(.callout)
                        Text(seat.id)
                            .font(.caption.monospaced())
                            .foregroundStyle(.secondary)
                        if seat.id == view.session.moderatorSeat {
                            Text("moderator")
                                .font(.caption2.weight(.semibold))
                                .padding(.horizontal, 8)
                                .padding(.vertical, 2)
                                .background(LoomGraphite.accent.opacity(0.14), in: Capsule())
                        }
                        Spacer()
                    }
                    .padding(.vertical, 4)
                }
                if !view.messages.isEmpty {
                    Divider()
                    ForEach(view.rounds, id: \.id) { round in
                        ForEach(round.messages, id: \.id) { message in
                            messageRow(message)
                        }
                    }
                }
            }
        }
        .padding(18)
        .background(LoomGraphite.raised, in: RoundedRectangle(cornerRadius: 12))
    }

    private func messageRow(_ message: LocalRoundtableMessage) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack(spacing: 8) {
                Text(message.writerSeat)
                    .font(.caption.monospaced())
                Image(systemName: "arrow.right")
                    .font(.caption2)
                    .foregroundStyle(.secondary)
                Text(message.targetSeat)
                    .font(.caption.monospaced())
                Spacer()
                Text(message.status)
                    .font(.caption.weight(.semibold))
                    .padding(.horizontal, 8)
                    .padding(.vertical, 2)
                    .background(
                        statusColor(message.status).opacity(0.14),
                        in: Capsule()
                    )
            }
            Text(message.body)
                .font(.callout)
                .textSelection(.enabled)
            Text("body digest \(String(message.bodyDigest.prefix(12)))…")
                .font(.caption2.monospaced())
                .foregroundStyle(.secondary)
        }
        .padding(10)
        .background(LoomGraphite.canvas, in: RoundedRectangle(cornerRadius: 8))
    }

    private func statusColor(_ status: String) -> Color {
        switch status {
        case "pending": return .orange
        case "relayed": return .blue
        case "acknowledged": return .purple
        case "inserted": return .green
        case "dropped": return .red
        default: return .gray
        }
    }

    private func createSession() {
        let id = sessionID.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !id.isEmpty else { return }
        busy = true
        Task {
            defer { busy = false }
            view = await store.roundtableCreateSession(
                sessionID: id, title: title
            )
            if let view {
                _ = await store.roundtableAddSeat(
                    sessionID: view.session.id, seatID: writerSeat,
                    displayName: "Writer Seat"
                )
                _ = await store.roundtableAddSeat(
                    sessionID: view.session.id, seatID: targetSeat,
                    displayName: "Target Seat"
                )
                self.view = await store.roundtableSnapshot(
                    sessionID: view.session.id
                )
            }
        }
    }

    private func runNextStep() async {
        guard let view else {
            createSession()
            return
        }
        busy = true
        defer { busy = false }
        let sessionID = view.session.id
        switch currentStep {
        case .create, .seats:
            _ = await store.roundtableAddSeat(
                sessionID: sessionID, seatID: writerSeat, displayName: "Writer Seat"
            )
            _ = await store.roundtableAddSeat(
                sessionID: sessionID, seatID: targetSeat, displayName: "Target Seat"
            )
        case .round:
            _ = await store.roundtableOpenRound(
                sessionID: sessionID, roundID: roundID
            )
        case .propose:
            _ = await store.roundtableProposeMessage(
                sessionID: sessionID, roundID: roundID, messageID: messageID,
                body: messageBody, writerSeat: writerSeat, targetSeat: targetSeat
            )
        case .relay:
            _ = await store.roundtableRelayMessage(
                sessionID: sessionID, messageID: messageID
            )
        case .acknowledge:
            _ = await store.roundtableAckMessage(
                sessionID: sessionID, messageID: messageID, seatID: targetSeat
            )
        case .insert:
            _ = await store.roundtableInsertMessage(
                sessionID: sessionID, messageID: messageID
            )
        case .conclude:
            _ = await store.roundtableConclude(sessionID: sessionID)
        }
        self.view = await store.roundtableSnapshot(sessionID: sessionID)
    }

    private func runFullJourney() async {
        busy = true
        defer { busy = false }
        if view == nil {
            let id = sessionID.trimmingCharacters(in: .whitespacesAndNewlines)
            guard !id.isEmpty else { return }
            view = await store.roundtableCreateSession(
                sessionID: id, title: title
            )
        }
        guard view != nil else { return }
        var attempts = 0
        // Drive every step to the end: the loop must also run the final
        // Conclude (moderator) step so the full journey actually concludes
        // instead of parking on the last action with no visible way to finish.
        while !(view?.session.concluded ?? false) && attempts < 13 {
            await runNextStep()
            attempts += 1
            if store.roundtableError != nil { break }
        }
    }
}

private enum StepState {
    case pending
    case current
    case completed

    var symbol: String {
        switch self {
        case .pending: return "circle"
        case .current: return "circle.circle"
        case .completed: return "checkmark.circle.fill"
        }
    }

    var color: Color {
        switch self {
        case .pending: return Color.secondary
        case .current: return LoomGraphite.accent
        case .completed: return Color.green
        }
    }
}
