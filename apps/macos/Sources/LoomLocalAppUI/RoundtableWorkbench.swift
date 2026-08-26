import SwiftUI
import LoomLocalAppCore

public struct LocalRoundtableAgentCandidate: Equatable, Identifiable, Sendable {
    public let id: String
    public let seatID: String
    public let displayName: String
    public let responsibility: String
    public let routeSummary: String

    public static func candidates(
        from setup: LocalProductSetupSnapshot
    ) -> [Self] {
        var seen = Set<String>()
        return setup.roleOptions.compactMap { option -> Self? in
            guard option.kind == "main" || option.kind == "subagent" else {
                return nil
            }
            let identity = "\(option.kind):\(option.agentDefinitionID):\(option.runtimeProfileID)"
            guard seen.insert(identity).inserted else { return nil }
            return Self(
                id: identity,
                seatID: seatID(for: identity),
                displayName: option.responsibility.trimmingCharacters(
                    in: .whitespacesAndNewlines
                ).isEmpty ? option.agentDefinitionID : option.responsibility,
                responsibility: option.responsibility,
                routeSummary: [
                    displayName(for: option.harnessAdapter, fallback: "Configured harness"),
                    displayProvider(option.providerID, authMode: option.authMode),
                    option.providerAccountID,
                    option.modelID,
                ].filter { !$0.isEmpty }.joined(separator: " · ")
            )
        }
    }

    public static func seatID(for identity: String) -> String {
        let normalized = identity.lowercased().map { character in
            character.isLetter || character.isNumber ? character : "-"
        }
        let suffix = String(normalized.prefix(112)).trimmingCharacters(
            in: CharacterSet(charactersIn: "-")
        )
        return "agent-" + (suffix.isEmpty ? "candidate" : suffix)
    }

    private static func displayName(
        for value: String,
        fallback: String
    ) -> String {
        let cleaned = value.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !cleaned.isEmpty else { return fallback }
        if cleaned.lowercased() == "opencode" { return "OpenCode" }
        return cleaned
            .replacingOccurrences(of: "_", with: " ")
            .replacingOccurrences(of: "-", with: " ")
            .split(separator: " ")
            .map { $0.prefix(1).uppercased() + $0.dropFirst() }
            .joined(separator: " ")
    }

    private static func displayProvider(
        _ providerID: String,
        authMode: String
    ) -> String {
        switch providerID.lowercased() {
        case "opencode": return "OpenCode"
        case "openai": return "OpenAI"
        case "anthropic": return "Anthropic"
        case "deepseek": return "DeepSeek"
        case "moonshot", "kimi": return "Kimi"
        case "minimax": return "MiniMax"
        case "": return authMode.isEmpty ? "Provider pending" : displayName(for: authMode, fallback: "Provider")
        default: return providerID
        }
    }
}

func roundtableSeatIsActive(_ seat: LocalRoundtableSeat?) -> Bool {
    seat?.available == true
}

func roundtableActiveSeatCount(_ view: LocalRoundtableView) -> Int {
    view.seats.values.filter(\.available).count
}

func roundtableActiveParticipantCount(_ view: LocalRoundtableView) -> Int {
    view.seats.values.filter {
        $0.available && $0.id != view.session.moderatorSeat
    }.count
}

func roundtableAvailableAgentCandidates(
    _ candidates: [LocalRoundtableAgentCandidate],
    in view: LocalRoundtableView
) -> [LocalRoundtableAgentCandidate] {
    guard view.rounds.isEmpty else { return [] }
    return candidates.filter { candidate in
        !roundtableSeatIsActive(view.seats[candidate.seatID])
    }
}

func roundtableOrderedAgentCandidates(
    _ candidates: [LocalRoundtableAgentCandidate],
    in view: LocalRoundtableView,
    preferredSeatOrder: [String]
) -> [LocalRoundtableAgentCandidate] {
    let configured = candidates.filter {
        roundtableSeatIsActive(view.seats[$0.seatID])
    }
    let configuredSeatIDs = Set(configured.map(\.seatID))
    let legacy = view.seats.values
        .filter { $0.id != view.session.moderatorSeat && $0.available }
        .filter { !configuredSeatIDs.contains($0.id) }
        .sorted { $0.id < $1.id }
        .map {
            LocalRoundtableAgentCandidate(
                id: "legacy:\($0.id)",
                seatID: $0.id,
                displayName: $0.displayName,
                responsibility: "",
                routeSummary: "Existing Roundtable seat"
            )
        }
    let active = configured + legacy
    let candidatesBySeatID = Dictionary(
        active.map { ($0.seatID, $0) },
        uniquingKeysWith: { first, _ in first }
    )
    var seen = Set<String>()
    let preferred: [LocalRoundtableAgentCandidate] = preferredSeatOrder.compactMap { seatID in
        guard seen.insert(seatID).inserted else { return nil }
        return candidatesBySeatID[seatID]
    }
    return preferred + active.filter { seen.insert($0.seatID).inserted }
}

// Governed Handoff / Roundtable workbench: a self-guiding multi-seat
// moderator journey. The moderator creates a session, adds configured Agents
// to writer and target seats, opens a round, then walks the message through
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
    @State private var isDropTargeted = false
    @State private var addedSeatOrder: [String] = []

    private let moderatorSeat = "seat-moderator"
    private let roundID = "round-1"
    private let messageID = "msg-1"

    public init(store: LocalProductStore) {
        self.store = store
    }

    private var agentCandidates: [LocalRoundtableAgentCandidate] {
        guard let setup = store.setupSnapshot else { return [] }
        return LocalRoundtableAgentCandidate.candidates(from: setup)
    }

    private var addedAgentCandidates: [LocalRoundtableAgentCandidate] {
        guard let view else { return [] }
        return roundtableOrderedAgentCandidates(
            agentCandidates,
            in: view,
            preferredSeatOrder: addedSeatOrder
        )
    }

    private var availableAgentCandidates: [LocalRoundtableAgentCandidate] {
        guard let view else { return agentCandidates }
        return roundtableAvailableAgentCandidates(agentCandidates, in: view)
    }

    private var writerSeat: String {
        addedAgentCandidates.first?.seatID ?? "seat-writer"
    }

    private var targetSeat: String {
        addedAgentCandidates.dropFirst().first?.seatID ?? "seat-target"
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

    /// Produce a readable record of what a concluded discussion actually
    /// decided — the visible counterpart to the digest-bound AlignmentSummary
    /// artifact. Returns an empty array while the session is still open.
    static func localRoundtableResultLines(
        _ view: LocalRoundtableView
    ) -> [String] {
        guard view.session.concluded else { return [] }
        var lines: [String] = []
        lines.append("Concluded: \(view.session.title)")
        let roundIDs = view.rounds.sorted {
            $0.sequence < $1.sequence
        }
        for round in roundIDs {
            lines.append("Round \(round.sequence) (\(round.id))")
            for message in round.messages {
                lines.append(
                    "[\(message.status)] \(message.writerSeat) → \(message.targetSeat): \(message.body)"
                )
            }
        }
        return lines
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
            case .seats: return "Add two Agents"
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
        if roundtableActiveParticipantCount(view) < 2 { return .seats }
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
                        agentSeatComposer
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

    private var agentSeatComposer: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(alignment: .firstTextBaseline) {
                VStack(alignment: .leading, spacing: 3) {
                    Text("Add Agents to this Roundtable")
                        .font(.title3.weight(.semibold))
                    Text("Drag a configured Agent into the drop zone. Each seat keeps its own Harness, Provider Account, and Model binding.")
                        .font(.callout)
                        .foregroundStyle(.secondary)
                        .fixedSize(horizontal: false, vertical: true)
                }
                Spacer()
                Text("\(addedAgentCandidates.count)/2 required")
                    .font(.caption.weight(.semibold))
                    .foregroundStyle(
                        addedAgentCandidates.count >= 2
                            ? Color.green
                            : LoomGraphite.accent
                    )
            }

            HStack(alignment: .top, spacing: 12) {
                VStack(alignment: .leading, spacing: 8) {
                    Text("Available Agents")
                        .font(.caption.weight(.semibold))
                        .foregroundStyle(.secondary)
                    if availableAgentCandidates.isEmpty {
                        ContentUnavailableView(
                            "No more configured Agents",
                            systemImage: "person.crop.circle.badge.checkmark",
                            description: Text("Configure an Agent Team first, or reopen this session after adding a new role.")
                        )
                        .frame(minHeight: 112)
                    } else {
                        ForEach(availableAgentCandidates) { candidate in
                            agentCandidateRow(candidate)
                        }
                    }
                }
                .frame(maxWidth: .infinity, alignment: .leading)

                VStack(alignment: .leading, spacing: 8) {
                    Text("RoundTable seats")
                        .font(.caption.weight(.semibold))
                        .foregroundStyle(.secondary)
                    VStack(alignment: .leading, spacing: 8) {
                        if addedAgentCandidates.isEmpty {
                            Text("Drop Agents here")
                                .font(.callout.weight(.semibold))
                                .foregroundStyle(LoomGraphite.accent)
                            Text("The first Agent writes. The second receives and acknowledges.")
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        } else {
                            ForEach(Array(addedAgentCandidates.enumerated()), id: \.element.id) { index, candidate in
                                HStack(spacing: 8) {
                                    Image(systemName: index == 0 ? "square.and.pencil" : "checkmark.bubble")
                                        .foregroundStyle(LoomGraphite.accent)
                                    VStack(alignment: .leading, spacing: 2) {
                                        Text(index == 0 ? "Writer" : "Target")
                                            .font(.caption.weight(.semibold))
                                        Text(candidate.displayName)
                                            .font(.callout)
                                        Text(candidate.routeSummary)
                                            .font(.caption2)
                                            .foregroundStyle(.secondary)
                                            .lineLimit(2)
                                    }
                                    Spacer()
                                    Button {
                                        retireRoundtableAgent(candidate)
                                    } label: {
                                        Image(systemName: "xmark")
                                    }
                                    .buttonStyle(.plain)
                                    .disabled(busy || view?.rounds.isEmpty == false)
                                    .help(
                                        view?.rounds.isEmpty == false
                                            ? "Seats are frozen after the round opens"
                                            : "Remove \(candidate.displayName) from this RoundTable"
                                    )
                                    .accessibilityLabel(
                                        "Remove \(candidate.displayName) from RoundTable"
                                    )
                                }
                                .padding(9)
                                .background(LoomGraphite.canvas, in: RoundedRectangle(cornerRadius: 8))
                            }
                            if addedAgentCandidates.count < 2 {
                                Text("Add one more Agent to open the round.")
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                            }
                        }
                    }
                    .frame(maxWidth: .infinity, minHeight: 112, alignment: .leading)
                    .padding(12)
                    .background(
                        isDropTargeted
                            ? LoomGraphite.accent.opacity(0.14)
                            : LoomGraphite.canvas,
                        in: RoundedRectangle(cornerRadius: 10)
                    )
                    .overlay {
                        RoundedRectangle(cornerRadius: 10)
                            .stroke(
                                isDropTargeted
                                    ? LoomGraphite.accent
                                    : LoomGraphite.separator,
                                style: StrokeStyle(lineWidth: 1, dash: [6, 4])
                            )
                    }
                    .dropDestination(for: String.self) { ids, _ in
                        let candidates = ids.compactMap { id in
                            agentCandidates.first { $0.id == id }
                        }
                        addRoundtableAgents(candidates)
                        return !candidates.isEmpty
                    } isTargeted: { targeted in
                        isDropTargeted = targeted
                    }
                    .accessibilityLabel("RoundTable Agent drop zone")
                    .accessibilityHint("Drag a configured Agent here, or use the Add button beside an Agent.")
                }
                .frame(maxWidth: .infinity, alignment: .leading)
            }
        }
        .padding(18)
        .background(LoomGraphite.raised, in: RoundedRectangle(cornerRadius: 12))
    }

    private func agentCandidateRow(
        _ candidate: LocalRoundtableAgentCandidate
    ) -> some View {
        HStack(spacing: 9) {
            Image(systemName: "person.crop.circle")
                .foregroundStyle(LoomGraphite.accent)
            VStack(alignment: .leading, spacing: 2) {
                Text(candidate.displayName)
                    .font(.callout.weight(.semibold))
                Text(candidate.routeSummary)
                    .font(.caption2)
                    .foregroundStyle(.secondary)
                    .lineLimit(2)
            }
            Spacer(minLength: 8)
            Button {
                addRoundtableAgents([candidate])
            } label: {
                Label("Add", systemImage: "plus")
            }
            .buttonStyle(.bordered)
            .controlSize(.small)
            .disabled(busy)
            .help("Add this Agent to the RoundTable")
            .accessibilityLabel("Add \(candidate.displayName) to RoundTable")
        }
        .padding(9)
        .background(LoomGraphite.canvas, in: RoundedRectangle(cornerRadius: 8))
        .contentShape(Rectangle())
        .draggable(candidate.id)
        .accessibilityElement(children: .contain)
        .accessibilityHint("Drag this Agent into the RoundTable seats, or activate Add.")
    }

    private var createSessionForm: some View {
        VStack(alignment: .leading, spacing: 14) {
            Text("New governed handoff")
                .font(.title3.weight(.semibold))
            Text("Create a session, then drag two configured Agents into the seats below. Loom walks one bounded message through propose → relay → acknowledge → insert → conclude.")
                .font(.callout)
                .foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)
            HStack(spacing: 8) {
                Image(systemName: "wand.and.stars")
                    .foregroundStyle(LoomGraphite.accent)
                VStack(alignment: .leading, spacing: 2) {
                    Text("Session ID generated automatically")
                        .font(.caption.weight(.semibold))
                    Text(sessionID)
                        .font(.caption.monospaced())
                        .foregroundStyle(.secondary)
                        .textSelection(.enabled)
                }
                Spacer()
                Button {
                    sessionID = Self.buildSuggestedSessionID(from: Date())
                } label: {
                    Image(systemName: "arrow.clockwise")
                }
                .buttonStyle(.plain)
                .accessibilityLabel("Generate a new Roundtable session ID")
                .help("Generate a new session ID")
            }
            .padding(10)
            .background(LoomGraphite.canvas, in: RoundedRectangle(cornerRadius: 8))
            TextField("Title", text: $title)
                .textFieldStyle(.roundedBorder)
            Text("Discussion message")
                .font(.subheadline.weight(.semibold))
            TextEditor(text: $messageBody)
                .font(.callout)
                .frame(minHeight: 72)
                .scrollContentBackground(.hidden)
                .padding(6)
                .background(
                    RoundedRectangle(cornerRadius: 6)
                        .fill(LoomGraphite.raised)
                        .overlay(
                            RoundedRectangle(cornerRadius: 6)
                                .stroke(LoomGraphite.accent.opacity(0.35), lineWidth: 1)
                        )
                )
                .textSelection(.enabled)
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

            DisclosureGroup("Open an existing session") {
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
                        Label("Resume last", systemImage: "arrow.counterclockwise")
                    }
                    .buttonStyle(.plain)
                    .font(.caption)
                    .help("Reopens the most recently used session")
                }
                }
                .padding(.top, 8)
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
                            currentStep == .create
                                ? "Start"
                                : currentStep == .seats
                                    ? "Add Agents above"
                                    : "Next: \(currentStep.title)",
                            systemImage: "arrow.right.circle"
                        )
                    }
                    .buttonStyle(.borderedProminent)
                    .disabled(busy || currentStep == .seats)
                }
                Button {
                    Task { await runFullJourney() }
                } label: {
                    Label("Run full journey", systemImage: "play.fill")
                }
                .buttonStyle(.bordered)
                .disabled(
                    busy || view?.session.concluded == true ||
                    addedAgentCandidates.count < 2
                )
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
                    Label("\(roundtableActiveSeatCount(view)) seats", systemImage: "person.2")
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
                if view.session.concluded {
                    resultPanel(view)
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

    private func resultPanel(_ view: LocalRoundtableView) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 8) {
                Image(systemName: "flag.checkered")
                    .foregroundStyle(.green)
                Text("Alignment Summary: Result")
                    .font(.subheadline.weight(.semibold))
                Spacer()
            }
            ForEach(
                RoundtableWorkbench.localRoundtableResultLines(view),
                id: \.self
            ) { line in
                Text(line)
                    .font(.callout.monospaced())
                    .textSelection(.enabled)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
        }
        .padding(12)
        .background(Color.green.opacity(0.10), in: RoundedRectangle(cornerRadius: 10))
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
            return
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

    private func addRoundtableAgents(
        _ candidates: [LocalRoundtableAgentCandidate]
    ) {
        guard let view else { return }
        let newCandidates = roundtableAvailableAgentCandidates(
            candidates,
            in: view
        )
        guard !newCandidates.isEmpty else { return }
        busy = true
        Task {
            defer { busy = false }
            var latest = self.view
            var confirmedSeatOrder = addedAgentCandidates.map(\.seatID)
            for candidate in newCandidates {
                guard let current = latest,
                      !roundtableSeatIsActive(
                          current.seats[candidate.seatID]
                      ) else { continue }
                guard let updated = await store.roundtableAddSeat(
                    sessionID: current.session.id,
                    seatID: candidate.seatID,
                    displayName: candidate.displayName
                ) else {
                    continue
                }
                latest = updated
                guard roundtableSeatIsActive(
                    updated.seats[candidate.seatID]
                ) else { continue }
                confirmedSeatOrder.removeAll { $0 == candidate.seatID }
                confirmedSeatOrder.append(candidate.seatID)
            }
            if let latest {
                self.view = latest
                addedSeatOrder = confirmedSeatOrder.filter {
                    roundtableSeatIsActive(latest.seats[$0])
                }
            }
        }
    }

    private func retireRoundtableAgent(
        _ candidate: LocalRoundtableAgentCandidate
    ) {
        guard let view, view.rounds.isEmpty else { return }
        busy = true
        Task {
            defer { busy = false }
            if let latest = await store.roundtableRetireSeat(
                sessionID: view.session.id,
                seatID: candidate.seatID,
                moderatorSeat: moderatorSeat
            ) {
                self.view = latest
                addedSeatOrder = addedSeatOrder.filter {
                    roundtableSeatIsActive(latest.seats[$0])
                }
            }
        }
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
