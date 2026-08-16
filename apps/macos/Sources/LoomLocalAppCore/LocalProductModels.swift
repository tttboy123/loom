import Foundation

public enum LocalProductWireError: Error, Equatable {
    case invalidJSON
    case invalidValue
    case unsupportedSchema
    case unknownField
}

private struct AnyCodingKey: CodingKey {
    let stringValue: String
    let intValue: Int? = nil

    init?(stringValue: String) {
        self.stringValue = stringValue
    }

    init?(intValue: Int) {
        return nil
    }
}

func rejectUnknownKeys(
    _ decoder: Decoder,
    allowed: Set<String>
) throws {
    let container = try decoder.container(keyedBy: AnyCodingKey.self)
    if container.allKeys.contains(where: { !allowed.contains($0.stringValue) }) {
        throw LocalProductWireError.unknownField
    }
}

private func validBoardCurrency(_ value: String) -> Bool {
    value.utf8.count == 3 && value.unicodeScalars.allSatisfy {
        $0.value >= 65 && $0.value <= 90
    }
}

private func validBoardUsage(
    observed: Bool,
    input: Int64,
    output: Int64,
    cacheRead: Int64,
    cacheWrite: Int64,
    total: Int64
) -> Bool {
    if !observed {
        return input == 0 && output == 0 && cacheRead == 0 && cacheWrite == 0
            && total == 0
    }
    return input >= 0 && output >= 0 && cacheRead >= 0 && cacheWrite >= 0
        && input <= Int64.max - output && total == input + output
}

private func validBoardCost(
    observed: Bool,
    microunits: Int64,
    currency: String
) -> Bool {
    observed
        ? microunits >= 0 && validBoardCurrency(currency)
        : microunits == 0 && currency.isEmpty
}

private func validBoardCostSource(observed: Bool, source: String) -> Bool {
    if !observed { return source.isEmpty }
    return [
		"provider_reported", "harness_reported", "rate_card_estimate",
        "legacy_unspecified",
    ].contains(source)
}

public struct LocalProductPageCursor: Codable, Equatable, Sendable {
    public let nextCursor: String
    public let hasMore: Bool

    enum CodingKeys: String, CodingKey, CaseIterable {
        case nextCursor = "next_cursor"
        case hasMore = "has_more"
    }

    public init(nextCursor: String = "", hasMore: Bool = false) {
        self.nextCursor = nextCursor
        self.hasMore = hasMore
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(decoder, allowed: ["next_cursor", "has_more"])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        nextCursor = try values.decode(String.self, forKey: .nextCursor)
        hasMore = try values.decode(Bool.self, forKey: .hasMore)
    }
}

public struct LocalProductRuntimeSummary: Codable, Equatable, Sendable, Identifiable {
    public var id: String { runtimeInstanceID }
    public let runtimeInstanceID: String
    public let displayName: String
    public let adapterType: String
    public let executableVersion: String
    public let status: String
    public let capacity: Int
    public let modelIDs: [String]
    public let observedCapabilities: [String]

    enum CodingKeys: String, CodingKey {
        case runtimeInstanceID = "runtime_instance_id"
        case displayName = "display_name"
        case adapterType = "adapter_type"
        case executableVersion = "executable_version"
        case status, capacity
        case modelIDs = "model_ids"
        case observedCapabilities = "observed_capabilities"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(
            decoder,
            allowed: [
                "runtime_instance_id", "display_name", "adapter_type",
                "executable_version", "status", "capacity", "model_ids",
                "observed_capabilities",
            ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        runtimeInstanceID = try values.decode(String.self, forKey: .runtimeInstanceID)
        displayName = try values.decode(String.self, forKey: .displayName)
        adapterType = try values.decode(String.self, forKey: .adapterType)
        executableVersion = try values.decode(String.self, forKey: .executableVersion)
        status = try values.decode(String.self, forKey: .status)
        capacity = try values.decode(Int.self, forKey: .capacity)
        modelIDs = try values.decode([String].self, forKey: .modelIDs)
        observedCapabilities = try values.decode(
            [String].self,
            forKey: .observedCapabilities
        )
    }
}

public struct LocalProductTeamSummary: Codable, Equatable, Sendable, Identifiable {
    public var id: String { teamInstanceID }
    public let teamInstanceID: String
    public let displayName: String
    public let sourceKind: String
    public let state: String
    public let confirmed: Bool
    public let executable: Bool
    public let readOnly: Bool

    enum CodingKeys: String, CodingKey {
        case teamInstanceID = "team_instance_id"
        case displayName = "display_name"
        case sourceKind = "source_kind"
        case state, confirmed, executable
        case readOnly = "read_only"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(
            decoder,
            allowed: [
                "team_instance_id", "display_name", "source_kind", "state",
                "confirmed", "executable", "read_only",
            ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        teamInstanceID = try values.decode(String.self, forKey: .teamInstanceID)
        displayName = try values.decode(String.self, forKey: .displayName)
        sourceKind = try values.decode(String.self, forKey: .sourceKind)
        state = try values.decode(String.self, forKey: .state)
        confirmed = try values.decode(Bool.self, forKey: .confirmed)
        executable = try values.decode(Bool.self, forKey: .executable)
        readOnly = try values.decode(Bool.self, forKey: .readOnly)
    }
}

public struct LocalProductRunSummary: Codable, Equatable, Sendable, Identifiable {
    public var id: String { runID }
    public let runID: String
    public let workItemID: String
    public let phase: String
    public let terminalStatus: String
    public let terminalReason: String
    public let runtimeInstanceID: String
    public let agentInstanceID: String
    public let claimGeneration: Int64

    enum CodingKeys: String, CodingKey {
        case runID = "run_id"
        case workItemID = "work_item_id"
        case phase
        case terminalStatus = "terminal_status"
        case terminalReason = "terminal_reason"
        case runtimeInstanceID = "runtime_instance_id"
        case agentInstanceID = "agent_instance_id"
        case claimGeneration = "claim_generation"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(
            decoder,
            allowed: [
                "run_id", "work_item_id", "phase", "terminal_status",
                "terminal_reason", "runtime_instance_id", "agent_instance_id",
                "claim_generation",
            ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        runID = try values.decode(String.self, forKey: .runID)
        workItemID = try values.decode(String.self, forKey: .workItemID)
        phase = try values.decode(String.self, forKey: .phase)
        terminalStatus = try values.decode(String.self, forKey: .terminalStatus)
        terminalReason = try values.decode(String.self, forKey: .terminalReason)
        runtimeInstanceID = try values.decode(String.self, forKey: .runtimeInstanceID)
        agentInstanceID = try values.decode(String.self, forKey: .agentInstanceID)
        claimGeneration = try values.decode(Int64.self, forKey: .claimGeneration)
    }
}

public struct LocalProductEvidenceSummary: Codable, Equatable, Sendable, Identifiable {
    public var id: String { evidenceID }
    public let evidenceID: String
    public let workItemID: String
    public let digest: String

    enum CodingKeys: String, CodingKey {
        case evidenceID = "evidence_id"
        case workItemID = "work_item_id"
        case digest
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(
            decoder,
            allowed: ["evidence_id", "work_item_id", "digest"]
        )
        let values = try decoder.container(keyedBy: CodingKeys.self)
        evidenceID = try values.decode(String.self, forKey: .evidenceID)
        workItemID = try values.decode(String.self, forKey: .workItemID)
        digest = try values.decode(String.self, forKey: .digest)
    }
}

public struct LocalProductAttention: Codable, Equatable, Sendable, Identifiable {
    public var id: String { attentionID }
    public let schemaVersion: Int
    public let attentionID: String
    public let kind: String
    public let severity: String
    public let teamInstanceID: String
    public let logicalNodeID: String
    public let workItemID: String
    public let approvalRequestID: String
    public let runtimeInstanceID: String
    public let status: String
    public let occurredAt: String
    public let actionRequired: String

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case attentionID = "attention_id"
        case kind, severity
        case teamInstanceID = "team_instance_id"
        case logicalNodeID = "logical_node_id"
        case workItemID = "work_item_id"
        case approvalRequestID = "approval_request_id"
        case runtimeInstanceID = "runtime_instance_id"
        case status
        case occurredAt = "occurred_at"
        case actionRequired = "action_required"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(
            decoder,
            allowed: [
                "schema_version", "attention_id", "kind", "severity",
                "team_instance_id", "logical_node_id", "work_item_id",
                "approval_request_id", "runtime_instance_id", "status",
                "occurred_at", "action_required",
            ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
        attentionID = try values.decode(String.self, forKey: .attentionID)
        kind = try values.decode(String.self, forKey: .kind)
        severity = try values.decode(String.self, forKey: .severity)
        teamInstanceID = try values.decode(String.self, forKey: .teamInstanceID)
        logicalNodeID = try values.decode(String.self, forKey: .logicalNodeID)
        workItemID = try values.decode(String.self, forKey: .workItemID)
        approvalRequestID = try values.decode(String.self, forKey: .approvalRequestID)
        runtimeInstanceID = try values.decode(String.self, forKey: .runtimeInstanceID)
        status = try values.decode(String.self, forKey: .status)
        occurredAt = try values.decode(String.self, forKey: .occurredAt)
        actionRequired = try values.decode(String.self, forKey: .actionRequired)
    }
}

public struct LocalProductMissionPulse:
    Codable, Equatable, Sendable, Identifiable
{
    public var id: String { nodeID }
    public let agentInstanceID: String
    public let runtimeInstanceID: String
    public let role: String
    public let state: String
    public let nodeID: String
    public let attemptNumber: Int

    enum CodingKeys: String, CodingKey {
        case agentInstanceID = "agent_instance_id"
        case runtimeInstanceID = "runtime_instance_id"
        case role, state
        case nodeID = "node_id"
        case attemptNumber = "attempt_number"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(
            decoder,
            allowed: [
                "agent_instance_id", "runtime_instance_id", "role", "state",
                "node_id", "attempt_number",
            ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        agentInstanceID = try values.decode(String.self, forKey: .agentInstanceID)
        runtimeInstanceID = try values.decode(
            String.self,
            forKey: .runtimeInstanceID
        )
        role = try values.decode(String.self, forKey: .role)
        state = try values.decode(String.self, forKey: .state)
        nodeID = try values.decode(String.self, forKey: .nodeID)
        attemptNumber = try values.decode(Int.self, forKey: .attemptNumber)
    }
}

public struct LocalProductMissionNode:
    Codable, Equatable, Sendable, Identifiable
{
    public var id: String { logicalNodeID }
    public let logicalNodeID: String
    public let title: String
    public let agentInstanceID: String
    public let runtimeInstanceID: String
    public let role: String
    public let dependsOn: [String]
    public let maxAttempts: Int
    public let status: String
    public let attemptNumber: Int
    public let ready: Bool

    enum CodingKeys: String, CodingKey {
        case logicalNodeID = "logical_node_id"
        case title
        case agentInstanceID = "agent_instance_id"
        case runtimeInstanceID = "runtime_instance_id"
        case role
        case dependsOn = "depends_on"
        case maxAttempts = "max_attempts"
        case status
        case attemptNumber = "attempt_number"
        case ready
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(
            decoder,
            allowed: [
                "logical_node_id", "title", "agent_instance_id",
                "runtime_instance_id", "role", "depends_on", "max_attempts",
                "status", "attempt_number", "ready",
            ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        logicalNodeID = try values.decode(String.self, forKey: .logicalNodeID)
        title = try values.decode(String.self, forKey: .title)
        agentInstanceID = try values.decode(String.self, forKey: .agentInstanceID)
        runtimeInstanceID = try values.decode(
            String.self,
            forKey: .runtimeInstanceID
        )
        role = try values.decode(String.self, forKey: .role)
        dependsOn = try values.decode([String].self, forKey: .dependsOn)
        maxAttempts = try values.decode(Int.self, forKey: .maxAttempts)
        status = try values.decode(String.self, forKey: .status)
        attemptNumber = try values.decode(Int.self, forKey: .attemptNumber)
        ready = try values.decode(Bool.self, forKey: .ready)
    }
}

public struct LocalProductMissionSummary:
    Codable, Equatable, Sendable, Identifiable
{
    public var id: String { missionID }
    public let schemaVersion: Int
    public let missionID: String
    public let teamInstanceID: String
    public let title: String
    public let sourceKind: String
    public let lane: String
    public let status: String
    public let priority: String
    public let planDigest: String
    public let simple: Bool
    public let nodeCount: Int
    public let completedNodeCount: Int
    public let activeNodeCount: Int
    public let reviewNodeCount: Int
    public let attentionCount: Int
    public let currentNodeID: String
    public let lastMilestone: String
    public let teamPulse: [LocalProductMissionPulse]
    public let topology: [LocalProductMissionNode]

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case missionID = "mission_id"
        case teamInstanceID = "team_instance_id"
        case title
        case sourceKind = "source_kind"
        case lane, status, priority
        case planDigest = "plan_digest"
        case simple
        case nodeCount = "node_count"
        case completedNodeCount = "completed_node_count"
        case activeNodeCount = "active_node_count"
        case reviewNodeCount = "review_node_count"
        case attentionCount = "attention_count"
        case currentNodeID = "current_node_id"
        case lastMilestone = "last_milestone"
        case teamPulse = "team_pulse"
        case topology
    }

    public init(
        schemaVersion: Int = 1,
        missionID: String,
        teamInstanceID: String,
        title: String,
        sourceKind: String,
        lane: String,
        status: String,
        priority: String,
        planDigest: String,
        simple: Bool,
        nodeCount: Int,
        completedNodeCount: Int,
        activeNodeCount: Int,
        reviewNodeCount: Int,
        attentionCount: Int,
        currentNodeID: String,
        lastMilestone: String,
        teamPulse: [LocalProductMissionPulse],
        topology: [LocalProductMissionNode]
    ) {
        self.schemaVersion = schemaVersion
        self.missionID = missionID
        self.teamInstanceID = teamInstanceID
        self.title = title
        self.sourceKind = sourceKind
        self.lane = lane
        self.status = status
        self.priority = priority
        self.planDigest = planDigest
        self.simple = simple
        self.nodeCount = nodeCount
        self.completedNodeCount = completedNodeCount
        self.activeNodeCount = activeNodeCount
        self.reviewNodeCount = reviewNodeCount
        self.attentionCount = attentionCount
        self.currentNodeID = currentNodeID
        self.lastMilestone = lastMilestone
        self.teamPulse = teamPulse
        self.topology = topology
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(
            decoder,
            allowed: [
                "schema_version", "mission_id", "team_instance_id", "title",
                "source_kind", "lane", "status", "priority", "plan_digest",
                "simple", "node_count", "completed_node_count",
                "active_node_count", "review_node_count", "attention_count",
                "current_node_id", "last_milestone", "team_pulse", "topology",
            ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
        guard schemaVersion == 1 else {
            throw LocalProductWireError.unsupportedSchema
        }
        missionID = try values.decode(String.self, forKey: .missionID)
        teamInstanceID = try values.decode(String.self, forKey: .teamInstanceID)
        title = try values.decode(String.self, forKey: .title)
        sourceKind = try values.decode(String.self, forKey: .sourceKind)
        lane = try values.decode(String.self, forKey: .lane)
        guard
            ["Proposed", "Ready", "Orchestrating", "Review", "Complete"]
                .contains(lane)
        else {
            throw LocalProductWireError.invalidJSON
        }
        status = try values.decode(String.self, forKey: .status)
        priority = try values.decode(String.self, forKey: .priority)
        planDigest = try values.decode(String.self, forKey: .planDigest)
        simple = try values.decode(Bool.self, forKey: .simple)
        nodeCount = try values.decode(Int.self, forKey: .nodeCount)
        completedNodeCount = try values.decode(Int.self, forKey: .completedNodeCount)
        activeNodeCount = try values.decode(Int.self, forKey: .activeNodeCount)
        reviewNodeCount = try values.decode(Int.self, forKey: .reviewNodeCount)
        attentionCount = try values.decode(Int.self, forKey: .attentionCount)
        currentNodeID = try values.decode(String.self, forKey: .currentNodeID)
        lastMilestone = try values.decode(String.self, forKey: .lastMilestone)
        teamPulse = try values.decode(
            [LocalProductMissionPulse].self,
            forKey: .teamPulse
        )
        topology = try values.decode(
            [LocalProductMissionNode].self,
            forKey: .topology
        )
    }
}

public struct LocalProductHealth: Codable, Equatable, Sendable {
    public let daemon: String
    public let journal: String
    public let projection: String

    enum CodingKeys: String, CodingKey {
        case daemon, journal, projection
    }

    public init(
        daemon: String,
        journal: String,
        projection: String
    ) {
        self.daemon = daemon
        self.journal = journal
        self.projection = projection
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(
            decoder,
            allowed: ["daemon", "journal", "projection"]
        )
        let values = try decoder.container(keyedBy: CodingKeys.self)
        daemon = try values.decode(String.self, forKey: .daemon)
        journal = try values.decode(String.self, forKey: .journal)
        projection = try values.decode(String.self, forKey: .projection)
        guard daemon == "serving_request",
            journal == "available",
            projection == "current" || projection == "stale"
        else {
            throw LocalProductWireError.invalidJSON
        }
    }

    public static let unknown = Self(
        daemon: "unknown",
        journal: "unknown",
        projection: "unknown"
    )
}

public struct LocalProductSnapshot: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let viewVersion: String
    public let partial: Bool
    public let stale: Bool
    public let reason: String
    public let health: LocalProductHealth
    public let runtimes: [LocalProductRuntimeSummary]
    public let teams: [LocalProductTeamSummary]
    public let missions: [LocalProductMissionSummary]
    public let runs: [LocalProductRunSummary]
    public let evidence: [LocalProductEvidenceSummary]
    public let attention: [LocalProductAttention]
    public let preparedDecisions: [LocalProductDecisionCommand]
    public let sideTasks: [LocalProductSideTaskSummary]
    public let runtimePage: LocalProductPageCursor
    public let teamPage: LocalProductPageCursor
    public let missionPage: LocalProductPageCursor
    public let runPage: LocalProductPageCursor
    public let evidencePage: LocalProductPageCursor

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case viewVersion = "view_version"
        case partial, stale, reason, health, runtimes, teams, missions, runs, evidence
        case attention
        case preparedDecisions = "prepared_decisions"
        case sideTasks = "side_tasks"
        case runtimePage = "runtime_page"
        case teamPage = "team_page"
        case missionPage = "mission_page"
        case runPage = "run_page"
        case evidencePage = "evidence_page"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(
            decoder,
            allowed: [
                "schema_version", "view_version", "partial", "stale", "reason",
                "health", "runtimes", "teams", "missions", "runs", "evidence", "attention",
                "prepared_decisions", "runtime_page", "team_page", "mission_page",
                "run_page", "evidence_page", "side_tasks",
            ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
        guard schemaVersion == 1 || schemaVersion == 2 || schemaVersion == 3 else {
            throw LocalProductWireError.unsupportedSchema
        }
        viewVersion = try values.decode(String.self, forKey: .viewVersion)
        partial = try values.decode(Bool.self, forKey: .partial)
        stale = try values.decode(Bool.self, forKey: .stale)
        reason = try values.decode(String.self, forKey: .reason)
        if schemaVersion >= 2 {
            health =
                try values.decodeIfPresent(
                    LocalProductHealth.self,
                    forKey: .health
                ) ?? .unknown
        } else {
            health = .unknown
        }
        runtimes = try values.decode([LocalProductRuntimeSummary].self, forKey: .runtimes)
        teams = try values.decode([LocalProductTeamSummary].self, forKey: .teams)
        if schemaVersion >= 2 {
            missions = try values.decode(
                [LocalProductMissionSummary].self,
                forKey: .missions
            )
        } else {
            missions = []
        }
        runs = try values.decode([LocalProductRunSummary].self, forKey: .runs)
        evidence = try values.decode([LocalProductEvidenceSummary].self, forKey: .evidence)
        attention = try values.decode([LocalProductAttention].self, forKey: .attention)
        if schemaVersion >= 2 {
            preparedDecisions = try values.decode(
                [LocalProductDecisionCommand].self,
                forKey: .preparedDecisions
            )
        } else {
            preparedDecisions = []
        }
        if schemaVersion >= 3 {
            sideTasks = try values.decode([LocalProductSideTaskSummary].self, forKey: .sideTasks)
        } else {
            sideTasks = []
        }
        runtimePage = try values.decode(LocalProductPageCursor.self, forKey: .runtimePage)
        teamPage = try values.decode(LocalProductPageCursor.self, forKey: .teamPage)
        if schemaVersion >= 2 {
            missionPage = try values.decode(
                LocalProductPageCursor.self,
                forKey: .missionPage
            )
        } else {
            missionPage = .init()
        }
        runPage = try values.decode(LocalProductPageCursor.self, forKey: .runPage)
        evidencePage = try values.decode(LocalProductPageCursor.self, forKey: .evidencePage)
    }

    public init(
        schemaVersion: Int = 1,
        viewVersion: String,
        partial: Bool = false,
        stale: Bool = false,
        reason: String = "",
        health: LocalProductHealth = .unknown,
        runtimes: [LocalProductRuntimeSummary] = [],
        teams: [LocalProductTeamSummary] = [],
        missions: [LocalProductMissionSummary] = [],
        runs: [LocalProductRunSummary] = [],
        evidence: [LocalProductEvidenceSummary] = [],
        attention: [LocalProductAttention] = [],
        preparedDecisions: [LocalProductDecisionCommand] = [],
        sideTasks: [LocalProductSideTaskSummary] = [],
        runtimePage: LocalProductPageCursor = .init(),
        teamPage: LocalProductPageCursor = .init(),
        missionPage: LocalProductPageCursor = .init(),
        runPage: LocalProductPageCursor = .init(),
        evidencePage: LocalProductPageCursor = .init()
    ) {
        self.schemaVersion = schemaVersion
        self.viewVersion = viewVersion
        self.partial = partial
        self.stale = stale
        self.reason = reason
        self.health = health
        self.runtimes = runtimes
        self.teams = teams
        self.missions = missions
        self.runs = runs
        self.evidence = evidence
        self.attention = attention
        self.preparedDecisions = preparedDecisions
        self.sideTasks = sideTasks
        self.runtimePage = runtimePage
        self.teamPage = teamPage
        self.missionPage = missionPage
        self.runPage = runPage
        self.evidencePage = evidencePage
    }

    public func encode(to encoder: Encoder) throws {
        var values = encoder.container(keyedBy: CodingKeys.self)
        try values.encode(schemaVersion, forKey: .schemaVersion)
        try values.encode(viewVersion, forKey: .viewVersion)
        try values.encode(partial, forKey: .partial)
        try values.encode(stale, forKey: .stale)
        try values.encode(reason, forKey: .reason)
        if schemaVersion >= 2 {
            try values.encode(health, forKey: .health)
        }
        try values.encode(runtimes, forKey: .runtimes)
        try values.encode(teams, forKey: .teams)
        if schemaVersion >= 2 {
            try values.encode(missions, forKey: .missions)
        }
        try values.encode(runs, forKey: .runs)
        try values.encode(evidence, forKey: .evidence)
        try values.encode(attention, forKey: .attention)
        if schemaVersion >= 2 {
            try values.encode(preparedDecisions, forKey: .preparedDecisions)
        }
        if schemaVersion >= 3 {
            try values.encode(sideTasks, forKey: .sideTasks)
        }
        try values.encode(runtimePage, forKey: .runtimePage)
        try values.encode(teamPage, forKey: .teamPage)
        if schemaVersion >= 2 {
            try values.encode(missionPage, forKey: .missionPage)
        }
        try values.encode(runPage, forKey: .runPage)
        try values.encode(evidencePage, forKey: .evidencePage)
    }

    public static func empty(viewVersion: String) -> Self {
        .init(viewVersion: viewVersion)
    }
}

public struct LocalProductCost: Codable, Equatable, Sendable {
    public let observed: Bool
    public let amountMicrounits: Int64?
    public let currency: String

    enum CodingKeys: String, CodingKey {
        case observed
        case amountMicrounits = "amount_microunits"
        case currency
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(
            decoder,
            allowed: ["observed", "amount_microunits", "currency"]
        )
        let values = try decoder.container(keyedBy: CodingKeys.self)
        observed = try values.decode(Bool.self, forKey: .observed)
        amountMicrounits = try values.decodeIfPresent(Int64.self, forKey: .amountMicrounits)
        currency = try values.decode(String.self, forKey: .currency)
    }

    public func encode(to encoder: Encoder) throws {
        var values = encoder.container(keyedBy: CodingKeys.self)
        try values.encode(observed, forKey: .observed)
        if let amountMicrounits {
            try values.encode(amountMicrounits, forKey: .amountMicrounits)
        } else {
            try values.encodeNil(forKey: .amountMicrounits)
        }
        try values.encode(currency, forKey: .currency)
    }
}

public struct LocalProductNode: Codable, Equatable, Sendable, Identifiable {
    public var id: String { logicalNodeID }
    public let logicalNodeID: String
    public let nodeKind: String
    public let routeGroupID: String
    public let status: String
    public let dependencySatisfied: Bool
    public let currentAttempt: Int
    public let workItemID: String
    public let runID: String
    public let runtimeInstanceID: String
    public let agentInstanceID: String
    public let verificationStatus: String
    public let recoveryAction: String
    public let retryAt: String
    public let fallbackConfigured: Bool
    public let recoveryApprovalRequired: Bool
    public let fallbackApprovalAvailable: Bool
    public let fallbackApprovalVersion: Int
    public let fallbackConsumed: Bool
    public let executionBindingAvailable: Bool
    public let harnessAdapter: String
    public let providerID: String
    public let providerAccountID: String
    public let modelID: String
    public let reasoningEffort: String
    public let timeoutNanoseconds: Int64
    public let bindingBudgetCredits: Int64?
    public let capabilities: [String]
    public let credentialRevision: Int64
    public let contextCapsuleAvailable: Bool
    public let contextCapsuleDigest: String
    public let routeSegmentAvailable: Bool
    public let routeSegmentID: String
    public let routeSegmentDigest: String
    public let contextDisclosureReceiptDigest: String
    public let contextAdapterID: String
    public let disclosurePolicyID: String
    public let disclosurePolicyVersion: Int
    public let contextTokenBudget: Int
    public let contextTokenCount: Int
    public let contextDisclosedCount: Int
    public let contextOmissionCount: Int
    public let providerAccountPolicyAvailable: Bool
    public let providerAccountPolicyVersion: Int
    public let providerAccountPolicyRevision: Int64
    public let providerAccountPolicyDigest: String
    public let providerAccountTrustDomain: String
    public let providerAccountRetentionMode: String
    public let providerAccountDataRegion: String
    public let providerAccountAssignedBudgetUnits: Int64
    public let providerModelRateCardAvailable: Bool
    public let providerModelRateCardRevision: Int64
    public let providerModelRateCardDigest: String
    public let providerModelRateCardCurrency: String
    public let providerModelRateCardInputBasis: String
    public let terminalReason: String
    public let incidentID: String
    public let failureDiagnosticAvailable: Bool
    public let failureStage: String
    public let failureCode: String
    public let failureRetryable: Bool
    public let testReportAvailable: Bool
    public let testReportCount: Int
    public let testReportPassedCount: Int
    public let testReportFailedCount: Int
    public let testReportSetDigest: String
    public let latestTestRunner: String
    public let latestTestScope: String
    public let latestTestOutcome: String
    public let latestTestReportDigest: String
    public let accountingAvailable: Bool
    public let usageObserved: Bool
    public let inputTokens: Int64
    public let outputTokens: Int64
    public let cacheReadTokens: Int64
    public let cacheWriteTokens: Int64
    public let totalTokens: Int64
    public let costObserved: Bool
    public let costMicrounits: Int64
    public let costCurrency: String
    public let costSource: String

    enum CodingKeys: String, CodingKey {
        case logicalNodeID = "logical_node_id"
        case nodeKind = "node_kind"
        case routeGroupID = "route_group_id"
        case status
        case dependencySatisfied = "dependency_satisfied"
        case currentAttempt = "current_attempt"
        case workItemID = "work_item_id"
        case runID = "run_id"
        case runtimeInstanceID = "runtime_instance_id"
        case agentInstanceID = "agent_instance_id"
        case verificationStatus = "verification_status"
        case recoveryAction = "recovery_action"
        case retryAt = "retry_at"
        case fallbackConfigured = "fallback_configured"
        case recoveryApprovalRequired = "recovery_approval_required"
        case fallbackApprovalAvailable = "fallback_approval_available"
        case fallbackApprovalVersion = "fallback_approval_version"
        case fallbackConsumed = "fallback_consumed"
        case executionBindingAvailable = "execution_binding_available"
        case harnessAdapter = "harness_adapter"
        case providerID = "provider_id"
        case providerAccountID = "provider_account_id"
        case modelID = "model_id"
        case reasoningEffort = "reasoning_effort"
        case timeoutNanoseconds = "timeout_nanoseconds"
        case bindingBudgetCredits = "binding_budget_credits"
        case capabilities
        case credentialRevision = "credential_revision"
        case contextCapsuleAvailable = "context_capsule_available"
        case contextCapsuleDigest = "context_capsule_digest"
        case routeSegmentAvailable = "route_segment_available"
        case routeSegmentID = "route_segment_id"
        case routeSegmentDigest = "route_segment_digest"
        case contextDisclosureReceiptDigest = "context_disclosure_receipt_digest"
        case contextAdapterID = "context_adapter_id"
        case disclosurePolicyID = "disclosure_policy_id"
        case disclosurePolicyVersion = "disclosure_policy_version"
        case contextTokenBudget = "context_token_budget"
        case contextTokenCount = "context_token_count"
        case contextDisclosedCount = "context_disclosed_count"
        case contextOmissionCount = "context_omission_count"
        case providerAccountPolicyAvailable = "provider_account_policy_available"
        case providerAccountPolicyVersion = "provider_account_policy_version"
        case providerAccountPolicyRevision = "provider_account_policy_revision"
        case providerAccountPolicyDigest = "provider_account_policy_digest"
        case providerAccountTrustDomain = "provider_account_trust_domain"
        case providerAccountRetentionMode = "provider_account_retention_mode"
        case providerAccountDataRegion = "provider_account_data_region"
        case providerAccountAssignedBudgetUnits = "provider_account_assigned_budget_units"
        case providerModelRateCardAvailable = "provider_model_rate_card_available"
        case providerModelRateCardRevision = "provider_model_rate_card_revision"
        case providerModelRateCardDigest = "provider_model_rate_card_digest"
        case providerModelRateCardCurrency = "provider_model_rate_card_currency"
        case providerModelRateCardInputBasis = "provider_model_rate_card_input_basis"
        case terminalReason = "terminal_reason"
        case incidentID = "incident_id"
        case failureDiagnosticAvailable = "failure_diagnostic_available"
        case failureStage = "failure_stage"
        case failureCode = "failure_code"
        case failureRetryable = "failure_retryable"
        case testReportAvailable = "test_report_available"
        case testReportCount = "test_report_count"
        case testReportPassedCount = "test_report_passed_count"
        case testReportFailedCount = "test_report_failed_count"
        case testReportSetDigest = "test_report_set_digest"
        case latestTestRunner = "latest_test_runner"
        case latestTestScope = "latest_test_scope"
        case latestTestOutcome = "latest_test_outcome"
        case latestTestReportDigest = "latest_test_report_digest"
        case accountingAvailable = "accounting_available"
        case usageObserved = "usage_observed"
        case inputTokens = "input_tokens"
        case outputTokens = "output_tokens"
        case cacheReadTokens = "cache_read_tokens"
        case cacheWriteTokens = "cache_write_tokens"
        case totalTokens = "total_tokens"
        case costObserved = "cost_observed"
        case costMicrounits = "cost_microunits"
        case costCurrency = "cost_currency"
        case costSource = "cost_source"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(
            decoder,
            allowed: [
                "logical_node_id", "node_kind", "route_group_id",
                "status", "dependency_satisfied",
                "current_attempt", "work_item_id", "run_id",
                "runtime_instance_id", "agent_instance_id",
                "verification_status", "recovery_action", "retry_at",
                "fallback_configured", "recovery_approval_required",
                "fallback_approval_available", "fallback_approval_version",
                "fallback_consumed",
                "execution_binding_available", "harness_adapter", "provider_id",
                "provider_account_id", "model_id", "reasoning_effort",
                "timeout_nanoseconds", "binding_budget_credits", "capabilities",
                "credential_revision", "provider_account_policy_available",
                "context_capsule_available", "context_capsule_digest",
                "route_segment_available", "route_segment_id", "route_segment_digest",
                "context_disclosure_receipt_digest", "context_adapter_id",
                "disclosure_policy_id", "disclosure_policy_version",
                "context_token_budget", "context_token_count",
                "context_disclosed_count", "context_omission_count",
                "provider_account_policy_version", "provider_account_policy_revision",
                "provider_account_policy_digest", "provider_account_trust_domain",
                "provider_account_retention_mode", "provider_account_data_region",
                "provider_account_assigned_budget_units", "terminal_reason", "incident_id",
                "provider_model_rate_card_available",
                "provider_model_rate_card_revision", "provider_model_rate_card_digest",
                "provider_model_rate_card_currency", "provider_model_rate_card_input_basis",
                "failure_diagnostic_available", "failure_stage", "failure_code",
                "failure_retryable",
                "test_report_available", "test_report_count",
                "test_report_passed_count", "test_report_failed_count",
                "test_report_set_digest", "latest_test_runner",
                "latest_test_scope", "latest_test_outcome",
                "latest_test_report_digest",
                "accounting_available", "usage_observed",
                "input_tokens", "output_tokens", "cache_read_tokens",
                "cache_write_tokens", "total_tokens", "cost_observed",
                "cost_microunits", "cost_currency", "cost_source",
            ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        logicalNodeID = try values.decode(String.self, forKey: .logicalNodeID)
        nodeKind = try values.decodeIfPresent(String.self, forKey: .nodeKind) ?? ""
        routeGroupID = try values.decodeIfPresent(
            String.self, forKey: .routeGroupID
        ) ?? ""
        guard ["", "route_sibling", "aggregation"].contains(nodeKind),
            nodeKind.isEmpty
                ? routeGroupID.isEmpty
                : LocalIPCClient.validIdentifier(routeGroupID)
        else {
            throw LocalProductWireError.invalidJSON
        }
        status = try values.decode(String.self, forKey: .status)
        dependencySatisfied = try values.decode(Bool.self, forKey: .dependencySatisfied)
        currentAttempt = try values.decode(Int.self, forKey: .currentAttempt)
        workItemID = try values.decode(String.self, forKey: .workItemID)
        runID = try values.decode(String.self, forKey: .runID)
        runtimeInstanceID = try values.decode(String.self, forKey: .runtimeInstanceID)
        agentInstanceID = try values.decode(String.self, forKey: .agentInstanceID)
        verificationStatus = try values.decode(String.self, forKey: .verificationStatus)
        recoveryAction = try values.decode(String.self, forKey: .recoveryAction)
        retryAt = try values.decode(String.self, forKey: .retryAt)
        fallbackConfigured = try values.decodeIfPresent(
            Bool.self, forKey: .fallbackConfigured
        ) ?? false
        recoveryApprovalRequired = try values.decodeIfPresent(
            Bool.self, forKey: .recoveryApprovalRequired
        ) ?? false
        fallbackApprovalAvailable = try values.decodeIfPresent(
            Bool.self, forKey: .fallbackApprovalAvailable
        ) ?? false
        fallbackApprovalVersion = try values.decodeIfPresent(
            Int.self, forKey: .fallbackApprovalVersion
        ) ?? 0
        fallbackConsumed = try values.decodeIfPresent(
            Bool.self, forKey: .fallbackConsumed
        ) ?? false
        guard fallbackApprovalVersion >= 0,
            fallbackConfigured || (
                !recoveryApprovalRequired && !fallbackApprovalAvailable
                    && fallbackApprovalVersion == 0 && !fallbackConsumed
            ),
            fallbackApprovalAvailable == (fallbackApprovalVersion > 0),
            !fallbackConsumed || !recoveryApprovalRequired
                || fallbackApprovalAvailable
        else {
            throw LocalProductWireError.invalidJSON
        }
        executionBindingAvailable = try values.decodeIfPresent(
            Bool.self, forKey: .executionBindingAvailable
        ) ?? false
        harnessAdapter = try values.decodeIfPresent(
            String.self, forKey: .harnessAdapter
        ) ?? ""
        providerID = try values.decodeIfPresent(String.self, forKey: .providerID) ?? ""
        providerAccountID = try values.decodeIfPresent(
            String.self, forKey: .providerAccountID
        ) ?? ""
        modelID = try values.decodeIfPresent(String.self, forKey: .modelID) ?? ""
        reasoningEffort = try values.decodeIfPresent(
            String.self, forKey: .reasoningEffort
        ) ?? ""
        timeoutNanoseconds = try values.decodeIfPresent(
            Int64.self, forKey: .timeoutNanoseconds
        ) ?? 0
        bindingBudgetCredits = try values.decodeIfPresent(
            Int64.self, forKey: .bindingBudgetCredits
        )
        capabilities = try values.decodeIfPresent(
            [String].self, forKey: .capabilities
        ) ?? []
        credentialRevision = try values.decodeIfPresent(
            Int64.self, forKey: .credentialRevision
        ) ?? 0
        let validCapabilities = capabilities.allSatisfy(LocalIPCClient.validIdentifier)
            && Set(capabilities).count == capabilities.count
            && capabilities == capabilities.sorted()
        let validBindingIdentity = LocalIPCClient.validIdentifier(harnessAdapter)
            && LocalIPCClient.validIdentifier(providerID)
            && LocalIPCClient.validIdentifier(modelID)
            && (reasoningEffort.isEmpty || LocalIPCClient.validIdentifier(reasoningEffort))
            && timeoutNanoseconds > 0
            && (bindingBudgetCredits.map { $0 >= 0 } ?? true)
            && validCapabilities
        let validCredentialIdentity = providerAccountID.isEmpty
            ? credentialRevision == 0
            : LocalIPCClient.validProviderAccountID(
                providerAccountID,
                providerID: providerID
            ) && credentialRevision > 0
        guard executionBindingAvailable
            ? validBindingIdentity && validCredentialIdentity
            : harnessAdapter.isEmpty && providerID.isEmpty
                && providerAccountID.isEmpty && modelID.isEmpty
                && reasoningEffort.isEmpty && timeoutNanoseconds == 0
                && bindingBudgetCredits == nil && capabilities.isEmpty
                && credentialRevision == 0
        else {
            throw LocalProductWireError.invalidJSON
        }
        contextCapsuleAvailable = try values.decodeIfPresent(
            Bool.self, forKey: .contextCapsuleAvailable
        ) ?? false
        contextCapsuleDigest = try values.decodeIfPresent(
            String.self, forKey: .contextCapsuleDigest
        ) ?? ""
        routeSegmentAvailable = try values.decodeIfPresent(
            Bool.self, forKey: .routeSegmentAvailable
        ) ?? false
        routeSegmentID = try values.decodeIfPresent(
            String.self, forKey: .routeSegmentID
        ) ?? ""
        routeSegmentDigest = try values.decodeIfPresent(
            String.self, forKey: .routeSegmentDigest
        ) ?? ""
        contextDisclosureReceiptDigest = try values.decodeIfPresent(
            String.self, forKey: .contextDisclosureReceiptDigest
        ) ?? ""
        contextAdapterID = try values.decodeIfPresent(
            String.self, forKey: .contextAdapterID
        ) ?? ""
        disclosurePolicyID = try values.decodeIfPresent(
            String.self, forKey: .disclosurePolicyID
        ) ?? ""
        disclosurePolicyVersion = try values.decodeIfPresent(
            Int.self, forKey: .disclosurePolicyVersion
        ) ?? 0
        contextTokenBudget = try values.decodeIfPresent(
            Int.self, forKey: .contextTokenBudget
        ) ?? 0
        contextTokenCount = try values.decodeIfPresent(
            Int.self, forKey: .contextTokenCount
        ) ?? 0
        contextDisclosedCount = try values.decodeIfPresent(
            Int.self, forKey: .contextDisclosedCount
        ) ?? 0
        contextOmissionCount = try values.decodeIfPresent(
            Int.self, forKey: .contextOmissionCount
        ) ?? 0
        let validContextDigest: (String) -> Bool = { digest in
            digest.count == 64 && digest.allSatisfy {
                ($0 >= "0" && $0 <= "9") || ($0 >= "a" && $0 <= "f")
            }
        }
        guard contextCapsuleAvailable
            ? executionBindingAvailable
                && validContextDigest(contextCapsuleDigest)
                && validContextDigest(contextDisclosureReceiptDigest)
                && LocalIPCClient.validIdentifier(contextAdapterID)
                && LocalIPCClient.validIdentifier(disclosurePolicyID)
                && disclosurePolicyVersion > 0
                && contextTokenBudget > 0
                && contextTokenCount >= 0
                && contextTokenCount <= contextTokenBudget
                && contextDisclosedCount >= 0 && contextOmissionCount >= 0
                && contextDisclosedCount + contextOmissionCount > 0
            : contextCapsuleDigest.isEmpty
                && contextDisclosureReceiptDigest.isEmpty
                && contextAdapterID.isEmpty && disclosurePolicyID.isEmpty
                && disclosurePolicyVersion == 0 && contextTokenBudget == 0
                && contextTokenCount == 0 && contextDisclosedCount == 0
                && contextOmissionCount == 0
        else {
            throw LocalProductWireError.invalidJSON
        }
        guard routeSegmentAvailable
            ? executionBindingAvailable && contextCapsuleAvailable
                && LocalIPCClient.validIdentifier(routeSegmentID)
                && validContextDigest(routeSegmentDigest)
            : routeSegmentID.isEmpty && routeSegmentDigest.isEmpty
        else {
            throw LocalProductWireError.invalidJSON
        }
        providerAccountPolicyAvailable = try values.decodeIfPresent(
            Bool.self, forKey: .providerAccountPolicyAvailable
        ) ?? false
        providerAccountPolicyVersion = try values.decodeIfPresent(
            Int.self, forKey: .providerAccountPolicyVersion
        ) ?? 0
        providerAccountPolicyRevision = try values.decodeIfPresent(
            Int64.self, forKey: .providerAccountPolicyRevision
        ) ?? 0
        providerAccountPolicyDigest = try values.decodeIfPresent(
            String.self, forKey: .providerAccountPolicyDigest
        ) ?? ""
        providerAccountTrustDomain = try values.decodeIfPresent(
            String.self, forKey: .providerAccountTrustDomain
        ) ?? ""
        providerAccountRetentionMode = try values.decodeIfPresent(
            String.self, forKey: .providerAccountRetentionMode
        ) ?? ""
        providerAccountDataRegion = try values.decodeIfPresent(
            String.self, forKey: .providerAccountDataRegion
        ) ?? ""
        providerAccountAssignedBudgetUnits = try values.decodeIfPresent(
            Int64.self, forKey: .providerAccountAssignedBudgetUnits
        ) ?? 0
        let validPolicyDigest = providerAccountPolicyDigest.count == 64
            && providerAccountPolicyDigest.allSatisfy {
                ($0 >= "0" && $0 <= "9") || ($0 >= "a" && $0 <= "f")
            }
        let validPolicyDisclosure: Bool
        switch providerAccountPolicyVersion {
        case 1:
            validPolicyDisclosure = providerAccountTrustDomain.isEmpty
                && providerAccountRetentionMode.isEmpty
                && providerAccountDataRegion.isEmpty
        case 2:
            validPolicyDisclosure = [
                "external_provider", "enterprise_tenant", "local_runtime",
            ].contains(providerAccountTrustDomain)
                && [
                    "provider_default", "zero_data_retention", "limited_retention",
                ].contains(providerAccountRetentionMode)
                && ["global", "us", "eu", "apac", "local"]
                    .contains(providerAccountDataRegion)
        default:
            validPolicyDisclosure = false
        }
        guard providerAccountAssignedBudgetUnits >= 0,
            !providerAccountPolicyAvailable || executionBindingAvailable,
            providerAccountPolicyAvailable
                ? providerAccountPolicyRevision > 0 && validPolicyDigest
                    && validPolicyDisclosure
                : providerAccountPolicyVersion == 0
                    && providerAccountPolicyRevision == 0
                    && providerAccountPolicyDigest.isEmpty
                    && providerAccountTrustDomain.isEmpty
                    && providerAccountRetentionMode.isEmpty
                    && providerAccountDataRegion.isEmpty
                    && providerAccountAssignedBudgetUnits == 0
        else {
            throw LocalProductWireError.invalidJSON
        }
        providerModelRateCardAvailable = try values.decodeIfPresent(
            Bool.self, forKey: .providerModelRateCardAvailable
        ) ?? false
        providerModelRateCardRevision = try values.decodeIfPresent(
            Int64.self, forKey: .providerModelRateCardRevision
        ) ?? 0
        providerModelRateCardDigest = try values.decodeIfPresent(
            String.self, forKey: .providerModelRateCardDigest
        ) ?? ""
        providerModelRateCardCurrency = try values.decodeIfPresent(
            String.self, forKey: .providerModelRateCardCurrency
        ) ?? ""
        providerModelRateCardInputBasis = try values.decodeIfPresent(
            String.self, forKey: .providerModelRateCardInputBasis
        ) ?? ""
        let validRateCardDigest = providerModelRateCardDigest.count == 64
            && providerModelRateCardDigest.allSatisfy {
                ($0 >= "0" && $0 <= "9") || ($0 >= "a" && $0 <= "f")
            }
        let validRateCardBasis = [
            "input_includes_cache", "input_excludes_cache",
        ].contains(providerModelRateCardInputBasis)
        guard !providerModelRateCardAvailable || executionBindingAvailable,
            providerModelRateCardAvailable
                ? providerModelRateCardRevision > 0 && validRateCardDigest
                    && validBoardCurrency(providerModelRateCardCurrency)
                    && validRateCardBasis
                : providerModelRateCardRevision == 0
                    && providerModelRateCardDigest.isEmpty
                    && providerModelRateCardCurrency.isEmpty
                    && providerModelRateCardInputBasis.isEmpty
        else {
            throw LocalProductWireError.invalidJSON
        }
        terminalReason = try values.decodeIfPresent(
            String.self, forKey: .terminalReason
        ) ?? ""
        incidentID = try values.decodeIfPresent(
            String.self, forKey: .incidentID
        ) ?? ""
        guard incidentID.isEmpty || LocalIPCWire.validRequestID(incidentID) else {
            throw LocalProductWireError.invalidJSON
        }
        failureDiagnosticAvailable = try values.decodeIfPresent(
            Bool.self, forKey: .failureDiagnosticAvailable
        ) ?? false
        failureStage = try values.decodeIfPresent(String.self, forKey: .failureStage) ?? ""
        failureCode = try values.decodeIfPresent(String.self, forKey: .failureCode) ?? ""
        failureRetryable = try values.decodeIfPresent(
            Bool.self, forKey: .failureRetryable
        ) ?? false
        let validFailureCode = !failureCode.isEmpty && failureCode.utf8.count <= 64
            && failureCode.utf8.allSatisfy { byte in
                (97...122).contains(byte) || byte == 95
            }
        guard failureDiagnosticAvailable
            ? executionBindingAvailable && !incidentID.isEmpty
                && LocalIPCRemoteError.Stage(rawValue: failureStage) != nil
                && validFailureCode
            : failureStage.isEmpty && failureCode.isEmpty && !failureRetryable
        else {
            throw LocalProductWireError.invalidJSON
        }
        testReportAvailable = try values.decodeIfPresent(
            Bool.self, forKey: .testReportAvailable
        ) ?? false
        testReportCount = try values.decodeIfPresent(
            Int.self, forKey: .testReportCount
        ) ?? 0
        testReportPassedCount = try values.decodeIfPresent(
            Int.self, forKey: .testReportPassedCount
        ) ?? 0
        testReportFailedCount = try values.decodeIfPresent(
            Int.self, forKey: .testReportFailedCount
        ) ?? 0
        testReportSetDigest = try values.decodeIfPresent(
            String.self, forKey: .testReportSetDigest
        ) ?? ""
        latestTestRunner = try values.decodeIfPresent(
            String.self, forKey: .latestTestRunner
        ) ?? ""
        latestTestScope = try values.decodeIfPresent(
            String.self, forKey: .latestTestScope
        ) ?? ""
        latestTestOutcome = try values.decodeIfPresent(
            String.self, forKey: .latestTestOutcome
        ) ?? ""
        latestTestReportDigest = try values.decodeIfPresent(
            String.self, forKey: .latestTestReportDigest
        ) ?? ""
        let validTestRunner = [
            "go_test", "swift_test", "cargo_test", "pytest", "npm_test",
            "pnpm_test", "yarn_test", "bun_test",
        ].contains(latestTestRunner)
        let validTestScope = ["default", "all", "selected"].contains(latestTestScope)
        let validTestOutcome = ["passed", "failed"].contains(latestTestOutcome)
        guard testReportAvailable
            ? executionBindingAvailable && testReportCount > 0 && testReportCount <= 256
                && testReportPassedCount >= 0 && testReportFailedCount >= 0
                && testReportPassedCount + testReportFailedCount == testReportCount
                && validContextDigest(testReportSetDigest)
                && validTestRunner && validTestScope && validTestOutcome
                && validContextDigest(latestTestReportDigest)
            : testReportCount == 0 && testReportPassedCount == 0
                && testReportFailedCount == 0 && testReportSetDigest.isEmpty
                && latestTestRunner.isEmpty && latestTestScope.isEmpty
                && latestTestOutcome.isEmpty && latestTestReportDigest.isEmpty
        else {
            throw LocalProductWireError.invalidJSON
        }
        accountingAvailable = try values.decodeIfPresent(
            Bool.self, forKey: .accountingAvailable
        ) ?? false
        usageObserved = try values.decodeIfPresent(Bool.self, forKey: .usageObserved) ?? false
        inputTokens = try values.decodeIfPresent(Int64.self, forKey: .inputTokens) ?? 0
        outputTokens = try values.decodeIfPresent(Int64.self, forKey: .outputTokens) ?? 0
        cacheReadTokens = try values.decodeIfPresent(Int64.self, forKey: .cacheReadTokens) ?? 0
        cacheWriteTokens = try values.decodeIfPresent(Int64.self, forKey: .cacheWriteTokens) ?? 0
        totalTokens = try values.decodeIfPresent(Int64.self, forKey: .totalTokens) ?? 0
        costObserved = try values.decodeIfPresent(Bool.self, forKey: .costObserved) ?? false
        costMicrounits = try values.decodeIfPresent(Int64.self, forKey: .costMicrounits) ?? 0
        costCurrency = try values.decodeIfPresent(String.self, forKey: .costCurrency) ?? ""
        costSource = try values.decodeIfPresent(String.self, forKey: .costSource) ?? ""
        let validTerminalReason = terminalReason.isEmpty ||
            (failureDiagnosticAvailable && terminalReason.utf8.count <= 512
                && terminalReason == terminalReason.trimmingCharacters(
                    in: .whitespacesAndNewlines
                )
                && terminalReason.unicodeScalars.allSatisfy {
                    !CharacterSet.controlCharacters.contains($0)
                }) || LocalIPCClient.validIdentifier(terminalReason)
        guard validTerminalReason,
            accountingAvailable
                ? validBoardUsage(
                    observed: usageObserved,
                    input: inputTokens,
                    output: outputTokens,
                    cacheRead: cacheReadTokens,
                    cacheWrite: cacheWriteTokens,
                    total: totalTokens
                ) && validBoardCost(
                    observed: costObserved,
                    microunits: costMicrounits,
                    currency: costCurrency
                ) && validBoardCostSource(observed: costObserved, source: costSource)
                : !usageObserved && inputTokens == 0 && outputTokens == 0
                    && cacheReadTokens == 0 && cacheWriteTokens == 0
                    && totalTokens == 0 && !costObserved && costMicrounits == 0
                    && costCurrency.isEmpty && costSource.isEmpty
        else {
            throw LocalProductWireError.invalidJSON
        }
        guard costSource != "rate_card_estimate" ||
            (providerModelRateCardAvailable
                && costCurrency == providerModelRateCardCurrency)
        else {
            throw LocalProductWireError.invalidJSON
        }
    }
}

public struct LocalProductBoard: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let teamInstanceID: String
    public let planDigest: String
    public let status: String
    public let viewVersion: String
    public let nodes: [LocalProductNode]
    public let providerAccounts: [LocalProductProviderAccountAccounting]
    public let cost: LocalProductCost

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case teamInstanceID = "team_instance_id"
        case planDigest = "plan_digest"
        case status
        case viewVersion = "view_version"
        case nodes
        case providerAccounts = "provider_accounts"
        case cost
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(
            decoder,
            allowed: [
                "schema_version", "team_instance_id", "plan_digest", "status",
                "view_version", "nodes", "provider_accounts", "cost",
            ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
        teamInstanceID = try values.decode(String.self, forKey: .teamInstanceID)
        planDigest = try values.decode(String.self, forKey: .planDigest)
        status = try values.decode(String.self, forKey: .status)
        viewVersion = try values.decode(String.self, forKey: .viewVersion)
        nodes = try values.decode([LocalProductNode].self, forKey: .nodes)
        providerAccounts = try values.decodeIfPresent(
            [LocalProductProviderAccountAccounting].self,
            forKey: .providerAccounts
        ) ?? []
        cost = try values.decode(LocalProductCost.self, forKey: .cost)
    }
}

public struct LocalProductProviderAccountCost: Codable, Equatable, Sendable {
    public let currency: String
    public let source: String
    public let amountMicrounits: Int64

    enum CodingKeys: String, CodingKey {
        case currency
        case source
        case amountMicrounits = "amount_microunits"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(decoder, allowed: ["currency", "source", "amount_microunits"])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        currency = try values.decode(String.self, forKey: .currency)
        source = try values.decode(String.self, forKey: .source)
        amountMicrounits = try values.decode(Int64.self, forKey: .amountMicrounits)
        guard validBoardCurrency(currency), validBoardCostSource(observed: true, source: source),
            amountMicrounits >= 0 else {
            throw LocalProductWireError.invalidJSON
        }
    }
}

public struct LocalProductProviderAccountAccounting: Codable, Equatable, Sendable, Identifiable {
    public var id: String { "\(providerID):\(providerAccountID)" }
    public let providerID: String
    public let providerAccountID: String
    public let activeAttempts: Int
    public let attemptCount: Int
    public let failedAttempts: Int
    public let rateLimitedAttempts: Int
    public let errorRateBasisPoints: Int
    public let budgetAttemptCount: Int
    public let budgetUnits: Int64
    public let policyAvailable: Bool
    public let policyRevision: Int64
    public let policyDigest: String
    public let maximumConcurrentAttempts: Int
    public let dispatchWindowSeconds: Int64
    public let maximumDispatchStarts: Int
    public let maximumAssignedBudgetUnits: Int64
    public let activeAssignedBudgetUnits: Int64
    public let accountingAttemptCount: Int
    public let usageAttemptCount: Int
    public let inputTokens: Int64
    public let outputTokens: Int64
    public let cacheReadTokens: Int64
    public let cacheWriteTokens: Int64
    public let totalTokens: Int64
    public let costAttemptCount: Int
    public let costs: [LocalProductProviderAccountCost]
    public let aggregationOverflow: Bool

    enum CodingKeys: String, CodingKey {
        case providerID = "provider_id"
        case providerAccountID = "provider_account_id"
        case activeAttempts = "active_attempts"
        case attemptCount = "attempt_count"
        case failedAttempts = "failed_attempts"
        case rateLimitedAttempts = "rate_limited_attempts"
        case errorRateBasisPoints = "error_rate_basis_points"
        case budgetAttemptCount = "budget_attempt_count"
        case budgetUnits = "budget_units"
        case policyAvailable = "policy_available"
        case policyRevision = "policy_revision"
        case policyDigest = "policy_digest"
        case maximumConcurrentAttempts = "maximum_concurrent_attempts"
        case dispatchWindowSeconds = "dispatch_window_seconds"
        case maximumDispatchStarts = "maximum_dispatch_starts"
        case maximumAssignedBudgetUnits = "maximum_assigned_budget_units"
        case activeAssignedBudgetUnits = "active_assigned_budget_units"
        case accountingAttemptCount = "accounting_attempt_count"
        case usageAttemptCount = "usage_attempt_count"
        case inputTokens = "input_tokens"
        case outputTokens = "output_tokens"
        case cacheReadTokens = "cache_read_tokens"
        case cacheWriteTokens = "cache_write_tokens"
        case totalTokens = "total_tokens"
        case costAttemptCount = "cost_attempt_count"
        case costs
        case aggregationOverflow = "aggregation_overflow"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(
            decoder,
            allowed: [
                "provider_id", "provider_account_id", "active_attempts",
                "attempt_count", "failed_attempts", "rate_limited_attempts",
                "error_rate_basis_points", "budget_attempt_count", "budget_units",
                "policy_available", "policy_revision", "policy_digest",
                "maximum_concurrent_attempts", "dispatch_window_seconds",
                "maximum_dispatch_starts", "maximum_assigned_budget_units",
                "active_assigned_budget_units",
                "accounting_attempt_count", "usage_attempt_count", "input_tokens",
                "output_tokens", "cache_read_tokens", "cache_write_tokens",
                "total_tokens", "cost_attempt_count", "costs",
                "aggregation_overflow",
            ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        providerID = try values.decode(String.self, forKey: .providerID)
        providerAccountID = try values.decode(String.self, forKey: .providerAccountID)
        activeAttempts = try values.decode(Int.self, forKey: .activeAttempts)
        attemptCount = try values.decode(Int.self, forKey: .attemptCount)
        failedAttempts = try values.decode(Int.self, forKey: .failedAttempts)
        rateLimitedAttempts = try values.decode(Int.self, forKey: .rateLimitedAttempts)
        errorRateBasisPoints = try values.decode(Int.self, forKey: .errorRateBasisPoints)
        budgetAttemptCount = try values.decode(Int.self, forKey: .budgetAttemptCount)
        budgetUnits = try values.decode(Int64.self, forKey: .budgetUnits)
        policyAvailable = try values.decodeIfPresent(Bool.self, forKey: .policyAvailable) ?? false
        policyRevision = try values.decodeIfPresent(Int64.self, forKey: .policyRevision) ?? 0
        policyDigest = try values.decodeIfPresent(String.self, forKey: .policyDigest) ?? ""
        maximumConcurrentAttempts = try values.decodeIfPresent(
            Int.self, forKey: .maximumConcurrentAttempts
        ) ?? 0
        dispatchWindowSeconds = try values.decodeIfPresent(
            Int64.self, forKey: .dispatchWindowSeconds
        ) ?? 0
        maximumDispatchStarts = try values.decodeIfPresent(
            Int.self, forKey: .maximumDispatchStarts
        ) ?? 0
        maximumAssignedBudgetUnits = try values.decodeIfPresent(
            Int64.self, forKey: .maximumAssignedBudgetUnits
        ) ?? 0
        activeAssignedBudgetUnits = try values.decodeIfPresent(
            Int64.self, forKey: .activeAssignedBudgetUnits
        ) ?? 0
        let validPolicyDigest = policyDigest.count == 64 && policyDigest.allSatisfy {
            ($0 >= "0" && $0 <= "9") || ($0 >= "a" && $0 <= "f")
        }
        guard activeAssignedBudgetUnits >= 0,
            policyAvailable
                ? policyRevision > 0 && validPolicyDigest
                    && maximumConcurrentAttempts > 0
                    && dispatchWindowSeconds > 0
                    && maximumDispatchStarts > 0
                    && maximumAssignedBudgetUnits >= activeAssignedBudgetUnits
                : policyRevision == 0 && policyDigest.isEmpty
                    && maximumConcurrentAttempts == 0
                    && dispatchWindowSeconds == 0
                    && maximumDispatchStarts == 0
                    && maximumAssignedBudgetUnits == 0
                    && activeAssignedBudgetUnits == 0
        else {
            throw LocalProductWireError.invalidJSON
        }
        accountingAttemptCount = try values.decode(Int.self, forKey: .accountingAttemptCount)
        usageAttemptCount = try values.decode(Int.self, forKey: .usageAttemptCount)
        inputTokens = try values.decode(Int64.self, forKey: .inputTokens)
        outputTokens = try values.decode(Int64.self, forKey: .outputTokens)
        cacheReadTokens = try values.decode(Int64.self, forKey: .cacheReadTokens)
        cacheWriteTokens = try values.decode(Int64.self, forKey: .cacheWriteTokens)
        totalTokens = try values.decode(Int64.self, forKey: .totalTokens)
        costAttemptCount = try values.decode(Int.self, forKey: .costAttemptCount)
        costs = try values.decode([LocalProductProviderAccountCost].self, forKey: .costs)
        aggregationOverflow = try values.decode(Bool.self, forKey: .aggregationOverflow)
        let expectedErrorRate: Int
        if attemptCount == 0 {
            expectedErrorRate = 0
        } else if failedAttempts <= Int.max / 10_000 {
            expectedErrorRate = failedAttempts * 10_000 / attemptCount
        } else {
            throw LocalProductWireError.invalidJSON
        }
        let costKeys = costs.map { "\($0.currency)\u{0}\($0.source)" }
        guard LocalIPCClient.validIdentifier(providerID),
            LocalIPCClient.validProviderAccountID(
                providerAccountID,
                providerID: providerID
            ),
            activeAttempts >= 0, attemptCount >= 0, failedAttempts >= 0,
            rateLimitedAttempts >= 0, budgetAttemptCount >= 0, budgetUnits >= 0,
            activeAttempts <= attemptCount, failedAttempts <= attemptCount,
            rateLimitedAttempts <= failedAttempts,
            budgetAttemptCount <= attemptCount,
            errorRateBasisPoints == expectedErrorRate,
            accountingAttemptCount >= 0, accountingAttemptCount <= attemptCount,
            usageAttemptCount >= 0, usageAttemptCount <= accountingAttemptCount,
            costAttemptCount >= 0, costAttemptCount <= accountingAttemptCount,
            costs.count <= costAttemptCount,
            costKeys == costKeys.sorted(), Set(costKeys).count == costKeys.count,
            inputTokens >= 0, outputTokens >= 0, cacheReadTokens >= 0,
            cacheWriteTokens >= 0, totalTokens >= 0,
            usageAttemptCount > 0 || (
                inputTokens == 0 && outputTokens == 0 && cacheReadTokens == 0
                    && cacheWriteTokens == 0 && totalTokens == 0
            ),
            aggregationOverflow || (
                inputTokens <= Int64.max - outputTokens
                    && totalTokens == inputTokens + outputTokens
            ),
            costAttemptCount > 0 || costs.isEmpty
        else {
            throw LocalProductWireError.invalidJSON
        }
    }
}

public struct LocalProductTimelinePayload: Codable, Equatable, Sendable {
    public let status: String
    public let reasonCode: String
    public let action: String
    public let warningCode: String
    public let retryAt: String
    public let textDelta: String
    public let evidenceDigest: String
    public let cost: LocalProductCost

    enum CodingKeys: String, CodingKey {
        case status
        case reasonCode = "reason_code"
        case action
        case warningCode = "warning_code"
        case retryAt = "retry_at"
        case textDelta = "text_delta"
        case evidenceDigest = "evidence_digest"
        case cost
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(
            decoder,
            allowed: [
                "status", "reason_code", "action", "warning_code", "retry_at",
                "text_delta", "evidence_digest", "cost",
            ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        status = try values.decode(String.self, forKey: .status)
        reasonCode = try values.decode(String.self, forKey: .reasonCode)
        action = try values.decode(String.self, forKey: .action)
        warningCode = try values.decode(String.self, forKey: .warningCode)
        retryAt = try values.decode(String.self, forKey: .retryAt)
        textDelta = try values.decode(String.self, forKey: .textDelta)
        evidenceDigest = try values.decode(String.self, forKey: .evidenceDigest)
        cost = try values.decode(LocalProductCost.self, forKey: .cost)
    }
}

public struct LocalProductTimelineRecord: Codable, Equatable, Sendable, Identifiable {
    public var id: String { deliveryID }
    public let schemaVersion: Int
    public let deliveryID: String
    public let kind: String
    public let authority: String
    public let teamInstanceID: String
    public let logicalNodeID: String
    public let attemptNumber: Int
    public let sourceStreamID: String
    public let sourceSequence: Int64
    public let sourceEventID: String
    public let occurredAt: String
    public let cursor: String
    public let payload: LocalProductTimelinePayload

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case deliveryID = "delivery_id"
        case kind, authority
        case teamInstanceID = "team_instance_id"
        case logicalNodeID = "logical_node_id"
        case attemptNumber = "attempt_number"
        case sourceStreamID = "source_stream_id"
        case sourceSequence = "source_sequence"
        case sourceEventID = "source_event_id"
        case occurredAt = "occurred_at"
        case cursor, payload
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(
            decoder,
            allowed: [
                "schema_version", "delivery_id", "kind", "authority",
                "team_instance_id", "logical_node_id", "attempt_number",
                "source_stream_id", "source_sequence", "source_event_id",
                "occurred_at", "cursor", "payload",
            ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
        deliveryID = try values.decode(String.self, forKey: .deliveryID)
        kind = try values.decode(String.self, forKey: .kind)
        authority = try values.decode(String.self, forKey: .authority)
        teamInstanceID = try values.decode(String.self, forKey: .teamInstanceID)
        logicalNodeID = try values.decode(String.self, forKey: .logicalNodeID)
        attemptNumber = try values.decode(Int.self, forKey: .attemptNumber)
        sourceStreamID = try values.decode(String.self, forKey: .sourceStreamID)
        sourceSequence = try values.decode(Int64.self, forKey: .sourceSequence)
        sourceEventID = try values.decode(String.self, forKey: .sourceEventID)
        occurredAt = try values.decode(String.self, forKey: .occurredAt)
        cursor = try values.decode(String.self, forKey: .cursor)
        payload = try values.decode(LocalProductTimelinePayload.self, forKey: .payload)
    }
}

public struct LocalProductStreamGap: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let deliveryID: String
    public let kind: String
    public let teamInstanceID: String
    public let reason: String
    public let previousCursorDigest: String
    public let currentViewVersion: String
    public let artifactAvailable: Bool
    public let artifactDigest: String
    public let recoverable: Bool
    public let occurredAt: String

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case deliveryID = "delivery_id"
        case kind
        case teamInstanceID = "team_instance_id"
        case reason
        case previousCursorDigest = "previous_cursor_digest"
        case currentViewVersion = "current_view_version"
        case artifactAvailable = "artifact_available"
        case artifactDigest = "artifact_digest"
        case recoverable
        case occurredAt = "occurred_at"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(
            decoder,
            allowed: [
                "schema_version", "delivery_id", "kind", "team_instance_id",
                "reason", "previous_cursor_digest", "current_view_version",
                "artifact_available", "artifact_digest", "recoverable",
                "occurred_at",
            ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
        deliveryID = try values.decode(String.self, forKey: .deliveryID)
        kind = try values.decode(String.self, forKey: .kind)
        teamInstanceID = try values.decode(String.self, forKey: .teamInstanceID)
        reason = try values.decode(String.self, forKey: .reason)
        previousCursorDigest = try values.decode(String.self, forKey: .previousCursorDigest)
        currentViewVersion = try values.decode(String.self, forKey: .currentViewVersion)
        artifactAvailable = try values.decode(Bool.self, forKey: .artifactAvailable)
        artifactDigest = try values.decode(String.self, forKey: .artifactDigest)
        recoverable = try values.decode(Bool.self, forKey: .recoverable)
        occurredAt = try values.decode(String.self, forKey: .occurredAt)
    }
}

public struct LocalProductTimelinePage: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let teamInstanceID: String
    public let viewVersion: String
    public let nextCursor: String
    public let hasMore: Bool
    public let gap: LocalProductStreamGap?
    public let records: [LocalProductTimelineRecord]
    public let board: LocalProductBoard
    public let attention: [LocalProductAttention]

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case teamInstanceID = "team_instance_id"
        case viewVersion = "view_version"
        case nextCursor = "next_cursor"
        case hasMore = "has_more"
        case gap, records, board, attention
    }

    public init(
        schemaVersion: Int,
        teamInstanceID: String,
        viewVersion: String,
        nextCursor: String,
        hasMore: Bool,
        gap: LocalProductStreamGap?,
        records: [LocalProductTimelineRecord],
        board: LocalProductBoard,
        attention: [LocalProductAttention]
    ) {
        self.schemaVersion = schemaVersion
        self.teamInstanceID = teamInstanceID
        self.viewVersion = viewVersion
        self.nextCursor = nextCursor
        self.hasMore = hasMore
        self.gap = gap
        self.records = records
        self.board = board
        self.attention = attention
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(
            decoder,
            allowed: [
                "schema_version", "team_instance_id", "view_version",
                "next_cursor", "has_more", "gap", "records", "board",
                "attention",
            ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
        guard schemaVersion == 1 else {
            throw LocalProductWireError.unsupportedSchema
        }
        teamInstanceID = try values.decode(String.self, forKey: .teamInstanceID)
        viewVersion = try values.decode(String.self, forKey: .viewVersion)
        nextCursor = try values.decode(String.self, forKey: .nextCursor)
        hasMore = try values.decode(Bool.self, forKey: .hasMore)
        gap = try values.decodeIfPresent(LocalProductStreamGap.self, forKey: .gap)
        records = try values.decode([LocalProductTimelineRecord].self, forKey: .records)
        board = try values.decode(LocalProductBoard.self, forKey: .board)
        attention = try values.decode([LocalProductAttention].self, forKey: .attention)
    }

    public func encode(to encoder: Encoder) throws {
        var values = encoder.container(keyedBy: CodingKeys.self)
        try values.encode(schemaVersion, forKey: .schemaVersion)
        try values.encode(teamInstanceID, forKey: .teamInstanceID)
        try values.encode(viewVersion, forKey: .viewVersion)
        try values.encode(nextCursor, forKey: .nextCursor)
        try values.encode(hasMore, forKey: .hasMore)
        if let gap {
            try values.encode(gap, forKey: .gap)
        } else {
            try values.encodeNil(forKey: .gap)
        }
        try values.encode(records, forKey: .records)
        try values.encode(board, forKey: .board)
        try values.encode(attention, forKey: .attention)
    }
}

public enum LocalProductChatRole: String, Codable, Sendable {
    case user = "user"
    case loom = "loom"
    case proposal = "proposal"
    case confirmation = "confirmation"
}

public enum LocalProductConversationContextMode:
    String, Codable, CaseIterable, Identifiable, Sendable
{
    case continueWithContext = "continue_with_context"
    case summaryOnly = "summary_only"
    case startClean = "start_clean"

    public var id: String { rawValue }
}

public struct LocalProductChatMessage: Codable, Equatable, Hashable, Sendable {
    public static let toolShapedWarning = "The conversation runtime returned tool-shaped text. Loom did not execute it."

    public let messageID: String
    public let segmentID: String
    public let role: String
    public let content: String
    public let tentative: Bool

    enum CodingKeys: String, CodingKey {
        case messageID = "message_id"
        case segmentID = "segment_id"
        case role, content, tentative
    }

    public init(
        messageID: String,
        segmentID: String = "",
        role: String,
        content: String,
        tentative: Bool
    ) {
        self.messageID = messageID
        self.segmentID = segmentID
        self.role = role
        self.content = content
        self.tentative = tentative
    }

    public init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        messageID = try values.decode(String.self, forKey: .messageID)
        segmentID = try values.decodeIfPresent(
            String.self,
            forKey: .segmentID
        ) ?? ""
        role = try values.decode(String.self, forKey: .role)
        content = try values.decode(String.self, forKey: .content)
        tentative = try values.decode(Bool.self, forKey: .tentative)
    }

    public var displayContent: String {
        guard tentative, Self.isToolShaped(content) else { return content }
        return Self.toolShapedWarning
    }

    private static func isToolShaped(_ content: String) -> Bool {
        var body = content.trimmingCharacters(in: .whitespacesAndNewlines)
        if body.hasPrefix("```"), body.hasSuffix("```"),
           let newline = body.firstIndex(of: "\n") {
            body = String(body[body.index(after: newline)..<body.index(body.endIndex, offsetBy: -3)])
                .trimmingCharacters(in: .whitespacesAndNewlines)
        }
        guard let first = body.first, let last = body.last,
              (first == "{" && last == "}") || (first == "[" && last == "]") else {
            return false
        }
        if let data = body.data(using: .utf8),
           (try? JSONSerialization.jsonObject(with: data)) != nil {
            return true
        }
        let lower = body.lowercased()
        return [
            "tool", "tool_name", "arguments", "command", "path",
            "function_call", "tool_call",
        ].contains { lower.contains("\"\($0)\"") || lower.contains("\($0):") }
    }
}

public struct LocalProductConversationSegment: Codable, Equatable, Sendable {
    public let segmentID: String
    public let profileID: String
    public let contextMode: LocalProductConversationContextMode
    public let contextCapsuleDigest: String
    public let disclosureReceiptDigest: String
    public let disclosedContextCount: Int
    public let omittedContextCount: Int
    public let executionBinding: LocalProductConversationExecutionBinding?
    public let bindingDigest: String

    enum CodingKeys: String, CodingKey {
        case segmentID = "segment_id"
        case profileID = "profile_id"
        case contextMode = "context_mode"
        case contextCapsuleDigest = "context_capsule_digest"
        case disclosureReceiptDigest = "disclosure_receipt_digest"
        case disclosedContextCount = "disclosed_context_count"
        case omittedContextCount = "omitted_context_count"
        case executionBinding = "execution_binding"
        case bindingDigest = "binding_digest"
    }

    public init(
        segmentID: String,
        profileID: String,
        contextMode: LocalProductConversationContextMode,
        contextCapsuleDigest: String,
        disclosureReceiptDigest: String = "",
        disclosedContextCount: Int = 0,
        omittedContextCount: Int = 0,
        executionBinding: LocalProductConversationExecutionBinding? = nil,
        bindingDigest: String
    ) {
        self.segmentID = segmentID
        self.profileID = profileID
        self.contextMode = contextMode
        self.contextCapsuleDigest = contextCapsuleDigest
        self.disclosureReceiptDigest = disclosureReceiptDigest
        self.disclosedContextCount = disclosedContextCount
        self.omittedContextCount = omittedContextCount
        self.executionBinding = executionBinding
        self.bindingDigest = bindingDigest
    }

    public init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        segmentID = try values.decode(String.self, forKey: .segmentID)
        profileID = try values.decode(String.self, forKey: .profileID)
        contextMode = try values.decode(
            LocalProductConversationContextMode.self,
            forKey: .contextMode
        )
        contextCapsuleDigest = try values.decode(
            String.self,
            forKey: .contextCapsuleDigest
        )
        disclosureReceiptDigest = try values.decodeIfPresent(
            String.self,
            forKey: .disclosureReceiptDigest
        ) ?? ""
        disclosedContextCount = try values.decodeIfPresent(
            Int.self,
            forKey: .disclosedContextCount
        ) ?? 0
        omittedContextCount = try values.decodeIfPresent(
            Int.self,
            forKey: .omittedContextCount
        ) ?? 0
        executionBinding = try values.decodeIfPresent(
            LocalProductConversationExecutionBinding.self,
            forKey: .executionBinding
        )
        bindingDigest = try values.decode(String.self, forKey: .bindingDigest)
        guard validConversationDisclosure(
            disclosureReceiptDigest,
            disclosed: disclosedContextCount,
            omitted: omittedContextCount
        ) else {
            throw LocalProductClientError.invalidResponse
        }
    }
}

public struct LocalProductConversationExecutionBinding:
    Codable, Equatable, Sendable
{
    public let schemaVersion: Int
    public let providerID: String
    public let providerAccountID: String
    public let providerAccountPolicyVersion: Int
    public let providerAccountPolicyRevision: Int64
    public let providerAccountPolicyDigest: String
    public let trustDomain: String
    public let retentionMode: String
    public let dataRegion: String

    enum CodingKeys: String, CodingKey, CaseIterable {
        case schemaVersion = "schema_version"
        case providerID = "provider_id"
        case providerAccountID = "provider_account_id"
        case providerAccountPolicyVersion = "provider_account_policy_version"
        case providerAccountPolicyRevision = "provider_account_policy_revision"
        case providerAccountPolicyDigest = "provider_account_policy_digest"
        case trustDomain = "trust_domain"
        case retentionMode = "retention_mode"
        case dataRegion = "data_region"
    }

    public init(
        schemaVersion: Int = 3,
        providerID: String,
        providerAccountID: String = "",
        providerAccountPolicyVersion: Int = 0,
        providerAccountPolicyRevision: Int64 = 0,
        providerAccountPolicyDigest: String = "",
        trustDomain: String = "",
        retentionMode: String = "",
        dataRegion: String = ""
    ) {
        self.schemaVersion = schemaVersion
        self.providerID = providerID
        self.providerAccountID = providerAccountID
        self.providerAccountPolicyVersion = providerAccountPolicyVersion
        self.providerAccountPolicyRevision = providerAccountPolicyRevision
        self.providerAccountPolicyDigest = providerAccountPolicyDigest
        self.trustDomain = trustDomain
        self.retentionMode = retentionMode
        self.dataRegion = dataRegion
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(
            decoder,
            allowed: Set(CodingKeys.allCases.map(\.rawValue))
        )
        let values = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
        providerID = try values.decode(String.self, forKey: .providerID)
        providerAccountID = try values.decodeIfPresent(
            String.self, forKey: .providerAccountID
        ) ?? ""
        providerAccountPolicyVersion = try values.decodeIfPresent(
            Int.self, forKey: .providerAccountPolicyVersion
        ) ?? 0
        providerAccountPolicyRevision = try values.decodeIfPresent(
            Int64.self, forKey: .providerAccountPolicyRevision
        ) ?? 0
        providerAccountPolicyDigest = try values.decodeIfPresent(
            String.self, forKey: .providerAccountPolicyDigest
        ) ?? ""
        trustDomain = try values.decodeIfPresent(
            String.self, forKey: .trustDomain
        ) ?? ""
        retentionMode = try values.decodeIfPresent(
            String.self, forKey: .retentionMode
        ) ?? ""
        dataRegion = try values.decodeIfPresent(
            String.self, forKey: .dataRegion
        ) ?? ""
        guard validConversationExecutionBinding(self) else {
            throw LocalProductClientError.invalidResponse
        }
    }
}

public struct LocalProductConversationAttempt: Codable, Equatable, Sendable {
    public let attemptID: String
    public let segmentID: String
    public let profileID: String
    public let contextMode: LocalProductConversationContextMode
    public let contextCapsuleDigest: String
    public let disclosureReceiptDigest: String
    public let disclosedContextCount: Int
    public let omittedContextCount: Int
    public let executionBinding: LocalProductConversationExecutionBinding?
    public let bindingDigest: String
    public let incidentID: String
    public let status: String
    public let failureCode: String
    public let failureStage: String
	public let httpStatus: Int
	public let providerCode: String
	public let failureMessage: String
	public let retryAfterSeconds: Int64
    public let retryable: Bool

    enum CodingKeys: String, CodingKey {
        case attemptID = "attempt_id"
        case segmentID = "segment_id"
        case profileID = "profile_id"
        case contextMode = "context_mode"
        case contextCapsuleDigest = "context_capsule_digest"
        case disclosureReceiptDigest = "disclosure_receipt_digest"
        case disclosedContextCount = "disclosed_context_count"
        case omittedContextCount = "omitted_context_count"
        case executionBinding = "execution_binding"
        case bindingDigest = "binding_digest"
        case incidentID = "incident_id"
        case status
        case failureCode = "failure_code"
        case failureStage = "failure_stage"
		case httpStatus = "http_status"
		case providerCode = "provider_code"
		case failureMessage = "failure_message"
		case retryAfterSeconds = "retry_after_seconds"
        case retryable
    }

    public init(
        attemptID: String,
        segmentID: String,
        profileID: String,
        contextMode: LocalProductConversationContextMode,
        contextCapsuleDigest: String,
        disclosureReceiptDigest: String = "",
        disclosedContextCount: Int = 0,
        omittedContextCount: Int = 0,
        executionBinding: LocalProductConversationExecutionBinding? = nil,
        bindingDigest: String,
        incidentID: String = "",
        status: String,
        failureCode: String,
        failureStage: String = "",
		httpStatus: Int = 0,
		providerCode: String = "",
		failureMessage: String = "",
		retryAfterSeconds: Int64 = 0,
        retryable: Bool = false
    ) {
        self.attemptID = attemptID
        self.segmentID = segmentID
        self.profileID = profileID
        self.contextMode = contextMode
        self.contextCapsuleDigest = contextCapsuleDigest
        self.disclosureReceiptDigest = disclosureReceiptDigest
        self.disclosedContextCount = disclosedContextCount
        self.omittedContextCount = omittedContextCount
        self.executionBinding = executionBinding
        self.bindingDigest = bindingDigest
        self.incidentID = incidentID
        self.status = status
        self.failureCode = failureCode
        self.failureStage = failureStage
		self.httpStatus = httpStatus
		self.providerCode = providerCode
		self.failureMessage = failureMessage
		self.retryAfterSeconds = retryAfterSeconds
        self.retryable = retryable
    }

    public init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        attemptID = try values.decode(String.self, forKey: .attemptID)
        segmentID = try values.decode(String.self, forKey: .segmentID)
        profileID = try values.decode(String.self, forKey: .profileID)
        contextMode = try values.decode(
            LocalProductConversationContextMode.self,
            forKey: .contextMode
        )
        contextCapsuleDigest = try values.decode(
            String.self,
            forKey: .contextCapsuleDigest
        )
        disclosureReceiptDigest = try values.decodeIfPresent(
            String.self,
            forKey: .disclosureReceiptDigest
        ) ?? ""
        disclosedContextCount = try values.decodeIfPresent(
            Int.self,
            forKey: .disclosedContextCount
        ) ?? 0
        omittedContextCount = try values.decodeIfPresent(
            Int.self,
            forKey: .omittedContextCount
        ) ?? 0
        executionBinding = try values.decodeIfPresent(
            LocalProductConversationExecutionBinding.self,
            forKey: .executionBinding
        )
        bindingDigest = try values.decode(String.self, forKey: .bindingDigest)
        incidentID = try values.decodeIfPresent(
            String.self,
            forKey: .incidentID
        ) ?? ""
        status = try values.decode(String.self, forKey: .status)
        failureCode = try values.decode(String.self, forKey: .failureCode)
        failureStage = try values.decodeIfPresent(
            String.self,
            forKey: .failureStage
        ) ?? ""
		httpStatus = try values.decodeIfPresent(Int.self, forKey: .httpStatus) ?? 0
		providerCode = try values.decodeIfPresent(String.self, forKey: .providerCode) ?? ""
		failureMessage = try values.decodeIfPresent(String.self, forKey: .failureMessage) ?? ""
		retryAfterSeconds = try values.decodeIfPresent(
			Int64.self,
			forKey: .retryAfterSeconds
		) ?? 0
        retryable = try values.decodeIfPresent(
            Bool.self,
            forKey: .retryable
        ) ?? false
        guard validConversationDisclosure(
                disclosureReceiptDigest,
                disclosed: disclosedContextCount,
                omitted: omittedContextCount
              ),
              incidentID.isEmpty || LocalIPCWire.validRequestID(incidentID),
              ["dispatching", "succeeded", "failed"].contains(status),
			  Self.validFailure(
				status: status,
				code: failureCode,
				stage: failureStage,
				httpStatus: httpStatus,
				providerCode: providerCode,
				failureMessage: failureMessage,
				retryAfterSeconds: retryAfterSeconds,
				retryable: retryable
			  ) else {
            throw LocalProductClientError.invalidResponse
        }
    }

    private static func validFailure(
        status: String,
        code: String,
        stage: String,
		httpStatus: Int,
		providerCode: String,
		failureMessage: String,
		retryAfterSeconds: Int64,
        retryable: Bool
    ) -> Bool {
        if status != "failed" {
			return code.isEmpty && stage.isEmpty && httpStatus == 0 &&
				providerCode.isEmpty && failureMessage.isEmpty &&
				retryAfterSeconds == 0 && !retryable
        }
        let codes = Set([
            "conversation_unavailable", "invalid_response", "invalid_request",
            "credential_unavailable", "provider_auth", "provider_rate_limit",
			"provider_rejected", "provider_insufficient_balance",
			"provider_model_unavailable", "provider_invalid_request",
			"provider_unavailable", "state_unavailable", "timeout",
        ])
        let stages = Set([
            "conversation_dispatch", "credential_lease_issue",
            "credential_lease_expire", "credential_lease_revoke", "vault_decrypt",
            "vault_aad_validation", "provider_dns", "provider_tls",
            "provider_connect", "provider_http", "provider_auth",
            "provider_rate_limit",
        ])
		guard codes.contains(code),
			(httpStatus == 0 || (100...599).contains(httpStatus)),
			Self.validProviderCode(providerCode),
			Self.validFailureMessage(failureMessage),
			(0...86_400).contains(retryAfterSeconds) else { return false }
        if stage.isEmpty {
			return (code == "conversation_unavailable" || code == "invalid_response") &&
				httpStatus == 0 && providerCode.isEmpty && failureMessage.isEmpty &&
				retryAfterSeconds == 0
        }
        return stages.contains(stage)
    }

	private static func validProviderCode(_ value: String) -> Bool {
		guard value.utf8.count <= 64 else { return false }
		return value.utf8.allSatisfy { byte in
			(48...57).contains(byte) || (65...90).contains(byte) ||
				(97...122).contains(byte) || [45, 46, 95].contains(byte)
		}
	}

	private static func validFailureMessage(_ value: String) -> Bool {
		guard value.utf8.count <= 256 else { return false }
		return value.unicodeScalars.allSatisfy { scalar in
			scalar.value >= 0x20 && scalar.value != 0x7f
		}
	}
}

private func validConversationDisclosure(
    _ receiptDigest: String,
    disclosed: Int,
    omitted: Int
) -> Bool {
    if receiptDigest.isEmpty {
        return disclosed == 0 && omitted == 0
    }
    let digestIsValid = receiptDigest.utf8.count == 64 &&
        receiptDigest.utf8.allSatisfy { byte in
            (48...57).contains(byte) || (97...102).contains(byte)
        }
    return digestIsValid && (1...256).contains(disclosed) &&
        (0...256).contains(omitted)
}

private func validConversationExecutionBinding(
    _ binding: LocalProductConversationExecutionBinding
) -> Bool {
    guard binding.schemaVersion == 3,
          LocalIPCClient.validIdentifier(binding.providerID) else {
        return false
    }
    let digestIsValid = binding.providerAccountPolicyDigest.utf8.count == 64 &&
        binding.providerAccountPolicyDigest.utf8.allSatisfy { byte in
            (48...57).contains(byte) || (97...102).contains(byte)
        }
    if binding.providerAccountID.isEmpty {
        return binding.providerAccountPolicyVersion == 0 &&
            binding.providerAccountPolicyRevision == 0 &&
            binding.providerAccountPolicyDigest.isEmpty &&
            binding.trustDomain.isEmpty && binding.retentionMode.isEmpty &&
            binding.dataRegion.isEmpty
    }
    guard LocalIPCClient.validProviderAccountID(
        binding.providerAccountID,
        providerID: binding.providerID
    ) else { return false }
    switch binding.providerAccountPolicyVersion {
    case 0:
        return binding.providerAccountPolicyRevision == 0 &&
            binding.providerAccountPolicyDigest.isEmpty &&
            binding.trustDomain.isEmpty && binding.retentionMode.isEmpty &&
            binding.dataRegion.isEmpty
    case 1:
        return binding.providerAccountPolicyRevision > 0 && digestIsValid &&
            binding.trustDomain.isEmpty && binding.retentionMode.isEmpty &&
            binding.dataRegion.isEmpty
    case 2:
        return binding.providerAccountPolicyRevision > 0 && digestIsValid &&
            ["external_provider", "enterprise_tenant", "local_runtime"]
                .contains(binding.trustDomain) &&
            ["provider_default", "zero_data_retention", "limited_retention"]
                .contains(binding.retentionMode) &&
            ["global", "us", "eu", "apac", "local"]
                .contains(binding.dataRegion)
    default:
        return false
    }
}

public struct LocalProductChatAvailabilityFailure: Codable, Equatable, Sendable {
    public let code: LocalIPCRemoteError.Code
    public let stage: LocalIPCRemoteError.Stage
    public let incidentID: String
    public let retryable: Bool

    enum CodingKeys: String, CodingKey {
        case code, stage
        case incidentID = "incident_id"
        case retryable
    }

    public init(
        code: LocalIPCRemoteError.Code,
        stage: LocalIPCRemoteError.Stage,
        incidentID: String,
        retryable: Bool
    ) {
        self.code = code
        self.stage = stage
        self.incidentID = incidentID
        self.retryable = retryable
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(
            decoder,
            allowed: ["code", "stage", "incident_id", "retryable"]
        )
        let values = try decoder.container(keyedBy: CodingKeys.self)
        code = try values.decode(LocalIPCRemoteError.Code.self, forKey: .code)
        stage = try values.decode(LocalIPCRemoteError.Stage.self, forKey: .stage)
        incidentID = try values.decode(String.self, forKey: .incidentID)
        retryable = try values.decode(Bool.self, forKey: .retryable)
        guard code == .stateUnavailable,
              [.migrationRead, .migrationCommit, .migrationCleanup].contains(stage),
              LocalIPCWire.validRequestID(incidentID)
        else {
            throw LocalProductClientError.invalidResponse
        }
    }
}

// LocalProductChatSession is the app-level registry entry for one conversation
// (one backend thread). It lets the user create and switch between multiple
// distinct conversations instead of a single implicit thread.
public struct LocalProductChatSession: Codable, Equatable, Identifiable, Sendable {
  public var id: String { threadID }
  public let threadID: String
  public var title: String
  public var createdAt: Date
  public var updatedAt: Date

  public init(
    threadID: String,
    title: String,
    createdAt: Date = Date(),
    updatedAt: Date = Date()
  ) {
    self.threadID = threadID
    self.title = title
    self.createdAt = createdAt
    self.updatedAt = updatedAt
  }
}

public struct LocalProductChatThread: Codable, Equatable, Sendable {
    public let threadID: String
    public let profileID: String
    public let segments: [LocalProductConversationSegment]
    public let attempts: [LocalProductConversationAttempt]
    public let messages: [LocalProductChatMessage]
    public let canReply: Bool
    public let requiresConfirmation: Bool
    public let availabilityFailure: LocalProductChatAvailabilityFailure?

    enum CodingKeys: String, CodingKey {
        case threadID = "thread_id"
        case profileID = "profile_id"
        case segments, attempts
        case messages
        case canReply = "can_reply"
        case requiresConfirmation = "requires_confirmation"
        case availabilityFailure = "availability_failure"
    }

    public init(
        threadID: String,
        profileID: String = "",
        segments: [LocalProductConversationSegment] = [],
        attempts: [LocalProductConversationAttempt] = [],
        messages: [LocalProductChatMessage],
        canReply: Bool,
        requiresConfirmation: Bool,
        availabilityFailure: LocalProductChatAvailabilityFailure? = nil
    ) {
        self.threadID = threadID
        self.profileID = profileID
        self.segments = segments
        self.attempts = attempts
        self.messages = messages
        self.canReply = canReply
        self.requiresConfirmation = requiresConfirmation
        self.availabilityFailure = availabilityFailure
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(
            decoder,
            allowed: [
                "thread_id", "profile_id", "segments", "attempts", "messages",
                "can_reply", "requires_confirmation", "availability_failure",
            ]
        )
        let values = try decoder.container(keyedBy: CodingKeys.self)
        threadID = try values.decode(String.self, forKey: .threadID)
        profileID = try values.decodeIfPresent(
            String.self,
            forKey: .profileID
        ) ?? ""
        segments = try values.decodeIfPresent(
            [LocalProductConversationSegment].self,
            forKey: .segments
        ) ?? []
        attempts = try values.decodeIfPresent(
            [LocalProductConversationAttempt].self,
            forKey: .attempts
        ) ?? []
        messages = try values.decode(
            [LocalProductChatMessage].self,
            forKey: .messages
        )
        canReply = try values.decode(Bool.self, forKey: .canReply)
        requiresConfirmation = try values.decode(
            Bool.self,
            forKey: .requiresConfirmation
        )
        availabilityFailure = try values.decodeIfPresent(
            LocalProductChatAvailabilityFailure.self,
            forKey: .availabilityFailure
        )
        if availabilityFailure != nil && canReply {
            throw LocalProductClientError.invalidResponse
        }
    }
}

public struct LocalProductChatThreadRequest: Encodable, Sendable {
    public let threadID: String

    enum CodingKeys: String, CodingKey {
        case threadID = "thread_id"
    }
}

public struct LocalProductChatMessageRequest: Encodable, Sendable {
    public let threadID: String
    public let content: String
    public let profileID: String
    public let modelID: String
    public let reasoningEffort: String
    public let contextMode: LocalProductConversationContextMode?
    public let expectedExecutionBinding: LocalProductConversationExecutionBinding?

    public init(
        threadID: String,
        content: String,
        profileID: String = "",
        modelID: String = "",
        reasoningEffort: String = "",
        contextMode: LocalProductConversationContextMode? = nil,
        expectedExecutionBinding: LocalProductConversationExecutionBinding? = nil
    ) {
        self.threadID = threadID
        self.content = content
        self.profileID = profileID
        self.modelID = modelID
        self.reasoningEffort = reasoningEffort
        self.contextMode = contextMode
        self.expectedExecutionBinding = expectedExecutionBinding
    }

    enum CodingKeys: String, CodingKey {
        case threadID = "thread_id"
        case content
        case profileID = "profile_id"
        case modelID = "model_id"
        case reasoningEffort = "reasoning_effort"
        case contextMode = "context_mode"
        case expectedExecutionBinding = "expected_execution_binding"
    }
}

public enum LocalProductWire {
    public static func decodeSnapshot(_ data: Data) throws -> LocalProductSnapshot {
        try decode(LocalProductSnapshot.self, from: data)
    }

    public static func decodeTimeline(_ data: Data) throws -> LocalProductTimelinePage {
        try decode(LocalProductTimelinePage.self, from: data)
    }

    public static func decodeChatThread(_ data: Data) throws -> LocalProductChatThread {
        try decode(LocalProductChatThread.self, from: data)
    }

    private static func decode<T: Decodable>(
        _ type: T.Type,
        from data: Data
    ) throws -> T {
        do {
            try StrictJSONScanner.validate(data)
            return try JSONDecoder().decode(type, from: data)
        } catch let error as LocalProductWireError {
            throw error
        } catch {
            throw LocalProductWireError.invalidJSON
        }
    }
}

enum StrictJSONScanner {
    static func validate(_ data: Data) throws {
        guard !data.isEmpty, String(data: data, encoding: .utf8) != nil else {
            throw LocalProductWireError.invalidJSON
        }
        var parser = Parser(bytes: Array(data))
        try parser.parseValue()
        parser.skipWhitespace()
        guard parser.index == parser.bytes.count else {
            throw LocalProductWireError.invalidJSON
        }
    }

    private struct Parser {
        let bytes: [UInt8]
        var index = 0

        mutating func skipWhitespace() {
            while index < bytes.count,
                [9, 10, 13, 32].contains(bytes[index])
            {
                index += 1
            }
        }

        mutating func parseValue() throws {
            skipWhitespace()
            guard index < bytes.count else { throw LocalProductWireError.invalidJSON }
            switch bytes[index] {
            case 0x7B: try parseObject()
            case 0x5B: try parseArray()
            case 0x22: _ = try parseString()
            case 0x74: try consume("true")
            case 0x66: try consume("false")
            case 0x6E: try consume("null")
            default: try parseNumber()
            }
        }

        mutating func parseObject() throws {
            index += 1
            skipWhitespace()
            if consumeIf(0x7D) { return }
            var keys = Set<String>()
            while true {
                skipWhitespace()
                let key = try parseString()
                guard keys.insert(key).inserted else {
                    throw LocalProductWireError.invalidJSON
                }
                skipWhitespace()
                guard consumeIf(0x3A) else { throw LocalProductWireError.invalidJSON }
                try parseValue()
                skipWhitespace()
                if consumeIf(0x7D) { return }
                guard consumeIf(0x2C) else { throw LocalProductWireError.invalidJSON }
            }
        }

        mutating func parseArray() throws {
            index += 1
            skipWhitespace()
            if consumeIf(0x5D) { return }
            while true {
                try parseValue()
                skipWhitespace()
                if consumeIf(0x5D) { return }
                guard consumeIf(0x2C) else { throw LocalProductWireError.invalidJSON }
            }
        }

        mutating func parseString() throws -> String {
            guard consumeIf(0x22) else { throw LocalProductWireError.invalidJSON }
            let start = index - 1
            var escaped = false
            while index < bytes.count {
                let byte = bytes[index]
                index += 1
                if escaped {
                    escaped = false
                    if byte == 0x75 {
                        guard index + 4 <= bytes.count else {
                            throw LocalProductWireError.invalidJSON
                        }
                        index += 4
                    }
                    continue
                }
                if byte == 0x5C {
                    escaped = true
                } else if byte == 0x22 {
                    let encoded = Data(bytes[start..<index])
                    guard
                        let decoded = try JSONSerialization.jsonObject(
                            with: Data("[\(String(decoding: encoded, as: UTF8.self))]".utf8)
                        ) as? [String],
                        let value = decoded.first
                    else {
                        throw LocalProductWireError.invalidJSON
                    }
                    return value
                } else if byte < 0x20 {
                    throw LocalProductWireError.invalidJSON
                }
            }
            throw LocalProductWireError.invalidJSON
        }

        mutating func parseNumber() throws {
            let start = index
            while index < bytes.count,
                bytes[index] == 0x2D || bytes[index] == 0x2B || bytes[index] == 0x2E
                    || bytes[index] == 0x65 || bytes[index] == 0x45
                    || (bytes[index] >= 0x30 && bytes[index] <= 0x39)
            {
                index += 1
            }
            guard index > start else { throw LocalProductWireError.invalidJSON }
            let token = String(decoding: bytes[start..<index], as: UTF8.self)
            guard Double(token) != nil else { throw LocalProductWireError.invalidJSON }
        }

        mutating func consume(_ literal: String) throws {
            let expected = Array(literal.utf8)
            guard index + expected.count <= bytes.count,
                Array(bytes[index..<(index + expected.count)]) == expected
            else {
                throw LocalProductWireError.invalidJSON
            }
            index += expected.count
        }

        mutating func consumeIf(_ byte: UInt8) -> Bool {
            guard index < bytes.count, bytes[index] == byte else { return false }
            index += 1
            return true
        }
    }
}

// MARK: - Conversation model catalog (Provider -> Model -> Reasoning Effort)

public struct LocalProductConversationModelOption: Identifiable, Equatable, Sendable {
    public var id: String { modelID }
    public let modelID: String
    public let displayName: String
    public let reasoningEfforts: [String]
}

public func localProductConversationModels(
    providerID: String
) -> [LocalProductConversationModelOption] {
    switch providerID {
    case "openai":
        // Grounded in the installed Codex CLI 0.144.1 model catalog
        // (~/.codex/models_cache.json): per-model reasoning levels plus the
        // native cc-switch DeepSeek V4 models (none/high).
        return [
            .init(modelID: "codex-default", displayName: "Codex default", reasoningEfforts: []),
            .init(
                modelID: "gpt-5.6-sol",
                displayName: "GPT-5.6 Sol",
                reasoningEfforts: ["low", "medium", "high", "xhigh", "max", "ultra"]
            ),
            .init(
                modelID: "gpt-5.6-terra",
                displayName: "GPT-5.6 Terra",
                reasoningEfforts: ["low", "medium", "high", "xhigh", "max", "ultra"]
            ),
            .init(
                modelID: "gpt-5.6-luna",
                displayName: "GPT-5.6 Luna",
                reasoningEfforts: ["low", "medium", "high", "xhigh", "max"]
            ),
            .init(
                modelID: "gpt-5.5",
                displayName: "GPT-5.5",
                reasoningEfforts: ["low", "medium", "high", "xhigh"]
            ),
            .init(
                modelID: "gpt-5.4",
                displayName: "GPT-5.4",
                reasoningEfforts: ["low", "medium", "high", "xhigh"]
            ),
            .init(
                modelID: "gpt-5.4-mini",
                displayName: "GPT-5.4 Mini",
                reasoningEfforts: ["low", "medium", "high", "xhigh"]
            ),
            .init(
                modelID: "gpt-5.3-codex-spark",
                displayName: "GPT-5.3 Codex Spark",
                reasoningEfforts: ["low", "medium", "high", "xhigh"]
            ),
            .init(
                modelID: "codex-auto-review",
                displayName: "Codex Auto Review",
                reasoningEfforts: ["low", "medium", "high", "xhigh", "max"]
            ),
            .init(
                modelID: "deepseek-v4-flash",
                displayName: "DeepSeek V4 Flash",
                reasoningEfforts: ["none", "high"]
            ),
            .init(
                modelID: "deepseek-v4-pro",
                displayName: "DeepSeek V4 Pro",
                reasoningEfforts: ["none", "high"]
            ),
        ]
    case "deepseek":
        return [
            .init(
                modelID: "deepseek-v4-flash",
                displayName: "DeepSeek V4 Flash",
                reasoningEfforts: ["low", "high", "max"]
            ),
            .init(
                modelID: "deepseek-v4-pro",
                displayName: "DeepSeek V4 Pro",
                reasoningEfforts: ["low", "high", "max"]
            ),
            .init(
                modelID: "deepseek-chat",
                displayName: "DeepSeek Chat (legacy alias)",
                reasoningEfforts: []
            ),
            .init(
                modelID: "deepseek-reasoner",
                displayName: "DeepSeek Reasoner (legacy alias)",
                reasoningEfforts: []
            ),
        ]
    case "kimi":
        return [
            .init(
                modelID: "kimi-k3",
                displayName: "Kimi K3",
                reasoningEfforts: ["low", "high", "max"]
            ),
            .init(modelID: "kimi-k2.6", displayName: "Kimi K2.6", reasoningEfforts: []),
        ]
    case "minimax":
        return [.init(modelID: "MiniMax-M3", displayName: "MiniMax M3", reasoningEfforts: [])]
    case "anthropic":
        return [.init(modelID: "claude-sonnet-5", displayName: "Claude Sonnet 5", reasoningEfforts: [])]
    case "opencode":
        // Grounded in the installed OpenCode CLI 1.18.3 model catalog
        // (`opencode models`): provider prefixes and reasoning efforts follow
        // the CLI (zai/ for GLM with ZHIPU_API_KEY, opencode/* free tier).
        return [
            .init(
                modelID: "deepseek/deepseek-chat",
                displayName: "DeepSeek Chat",
                reasoningEfforts: []
            ),
            .init(
                modelID: "deepseek/deepseek-reasoner",
                displayName: "DeepSeek Reasoner",
                reasoningEfforts: []
            ),
            .init(
                modelID: "deepseek/deepseek-v4-flash",
                displayName: "DeepSeek V4 Flash",
                reasoningEfforts: ["low", "high", "max"]
            ),
            .init(
                modelID: "deepseek/deepseek-v4-pro",
                displayName: "DeepSeek V4 Pro",
                reasoningEfforts: ["high", "max"]
            ),
            .init(
                modelID: "minimax/MiniMax-M2.7",
                displayName: "MiniMax M2.7",
                reasoningEfforts: []
            ),
            .init(
                modelID: "minimax/MiniMax-M3",
                displayName: "MiniMax M3",
                reasoningEfforts: []
            ),
            .init(
                modelID: "zai/glm-4.5",
                displayName: "Zhipu GLM-4.5",
                reasoningEfforts: []
            ),
            .init(
                modelID: "zai/glm-5.2",
                displayName: "Zhipu GLM-5.2",
                reasoningEfforts: ["high", "max"]
            ),
            .init(
                modelID: "opencode/deepseek-v4-flash-free",
                displayName: "DeepSeek V4 Flash (OpenCode)",
                reasoningEfforts: ["low", "high", "max"]
            ),
        ]
    default:
        return []
    }
}

public func localProductConversationReasoningEfforts(
    providerID: String,
    modelID: String
) -> [String] {
    localProductConversationModels(providerID: providerID)
        .first { $0.modelID == modelID }?
        .reasoningEfforts ?? []
}
