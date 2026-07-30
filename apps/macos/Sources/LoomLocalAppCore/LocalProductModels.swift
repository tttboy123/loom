import Foundation

public enum LocalProductWireError: Error, Equatable {
    case invalidJSON
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

private func rejectUnknownKeys(
    _ decoder: Decoder,
    allowed: Set<String>
) throws {
    let container = try decoder.container(keyedBy: AnyCodingKey.self)
    if container.allKeys.contains(where: { !allowed.contains($0.stringValue) }) {
        throw LocalProductWireError.unknownField
    }
}

public struct LocalProductPageCursor: Codable, Equatable, Sendable {
    public let nextCursor: String
    public let hasMore: Bool

    enum CodingKeys: String, CodingKey {
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
        try rejectUnknownKeys(decoder, allowed: [
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
        try rejectUnknownKeys(decoder, allowed: [
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
        try rejectUnknownKeys(decoder, allowed: [
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
        try rejectUnknownKeys(decoder, allowed: [
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
        try rejectUnknownKeys(decoder, allowed: [
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
        try rejectUnknownKeys(decoder, allowed: [
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
        try rejectUnknownKeys(decoder, allowed: [
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
        guard ["Proposed", "Ready", "Orchestrating", "Review", "Complete"]
            .contains(lane) else {
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

public struct LocalProductSnapshot: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let viewVersion: String
    public let partial: Bool
    public let stale: Bool
    public let reason: String
    public let runtimes: [LocalProductRuntimeSummary]
    public let teams: [LocalProductTeamSummary]
    public let missions: [LocalProductMissionSummary]
    public let runs: [LocalProductRunSummary]
    public let evidence: [LocalProductEvidenceSummary]
    public let attention: [LocalProductAttention]
    public let preparedDecisions: [LocalProductDecisionCommand]
    public let runtimePage: LocalProductPageCursor
    public let teamPage: LocalProductPageCursor
    public let missionPage: LocalProductPageCursor
    public let runPage: LocalProductPageCursor
    public let evidencePage: LocalProductPageCursor

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case viewVersion = "view_version"
        case partial, stale, reason, runtimes, teams, missions, runs, evidence
        case attention
        case preparedDecisions = "prepared_decisions"
        case runtimePage = "runtime_page"
        case teamPage = "team_page"
        case missionPage = "mission_page"
        case runPage = "run_page"
        case evidencePage = "evidence_page"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(decoder, allowed: [
            "schema_version", "view_version", "partial", "stale", "reason",
            "runtimes", "teams", "missions", "runs", "evidence", "attention",
            "prepared_decisions", "runtime_page", "team_page", "mission_page",
            "run_page", "evidence_page",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
        guard schemaVersion == 1 || schemaVersion == 2 else {
            throw LocalProductWireError.unsupportedSchema
        }
        viewVersion = try values.decode(String.self, forKey: .viewVersion)
        partial = try values.decode(Bool.self, forKey: .partial)
        stale = try values.decode(Bool.self, forKey: .stale)
        reason = try values.decode(String.self, forKey: .reason)
        runtimes = try values.decode([LocalProductRuntimeSummary].self, forKey: .runtimes)
        teams = try values.decode([LocalProductTeamSummary].self, forKey: .teams)
        if schemaVersion == 2 {
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
        if schemaVersion == 2 {
            preparedDecisions = try values.decode(
                [LocalProductDecisionCommand].self,
                forKey: .preparedDecisions
            )
        } else {
            preparedDecisions = []
        }
        runtimePage = try values.decode(LocalProductPageCursor.self, forKey: .runtimePage)
        teamPage = try values.decode(LocalProductPageCursor.self, forKey: .teamPage)
        if schemaVersion == 2 {
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
        runtimes: [LocalProductRuntimeSummary] = [],
        teams: [LocalProductTeamSummary] = [],
        missions: [LocalProductMissionSummary] = [],
        runs: [LocalProductRunSummary] = [],
        evidence: [LocalProductEvidenceSummary] = [],
        attention: [LocalProductAttention] = [],
        preparedDecisions: [LocalProductDecisionCommand] = [],
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
        self.runtimes = runtimes
        self.teams = teams
        self.missions = missions
        self.runs = runs
        self.evidence = evidence
        self.attention = attention
        self.preparedDecisions = preparedDecisions
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
        try values.encode(runtimes, forKey: .runtimes)
        try values.encode(teams, forKey: .teams)
        if schemaVersion == 2 {
            try values.encode(missions, forKey: .missions)
        }
        try values.encode(runs, forKey: .runs)
        try values.encode(evidence, forKey: .evidence)
        try values.encode(attention, forKey: .attention)
        if schemaVersion == 2 {
            try values.encode(preparedDecisions, forKey: .preparedDecisions)
        }
        try values.encode(runtimePage, forKey: .runtimePage)
        try values.encode(teamPage, forKey: .teamPage)
        if schemaVersion == 2 {
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

    enum CodingKeys: String, CodingKey {
        case logicalNodeID = "logical_node_id"
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
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(decoder, allowed: [
            "logical_node_id", "status", "dependency_satisfied",
            "current_attempt", "work_item_id", "run_id",
            "runtime_instance_id", "agent_instance_id",
            "verification_status", "recovery_action", "retry_at",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        logicalNodeID = try values.decode(String.self, forKey: .logicalNodeID)
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
    }
}

public struct LocalProductBoard: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let teamInstanceID: String
    public let planDigest: String
    public let status: String
    public let viewVersion: String
    public let nodes: [LocalProductNode]
    public let cost: LocalProductCost

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case teamInstanceID = "team_instance_id"
        case planDigest = "plan_digest"
        case status
        case viewVersion = "view_version"
        case nodes, cost
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(decoder, allowed: [
            "schema_version", "team_instance_id", "plan_digest", "status",
            "view_version", "nodes", "cost",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
        teamInstanceID = try values.decode(String.self, forKey: .teamInstanceID)
        planDigest = try values.decode(String.self, forKey: .planDigest)
        status = try values.decode(String.self, forKey: .status)
        viewVersion = try values.decode(String.self, forKey: .viewVersion)
        nodes = try values.decode([LocalProductNode].self, forKey: .nodes)
        cost = try values.decode(LocalProductCost.self, forKey: .cost)
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
        try rejectUnknownKeys(decoder, allowed: [
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
        try rejectUnknownKeys(decoder, allowed: [
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
        try rejectUnknownKeys(decoder, allowed: [
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

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(decoder, allowed: [
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

public enum LocalProductWire {
    public static func decodeSnapshot(_ data: Data) throws -> LocalProductSnapshot {
        try decode(LocalProductSnapshot.self, from: data)
    }

    public static func decodeTimeline(_ data: Data) throws -> LocalProductTimelinePage {
        try decode(LocalProductTimelinePage.self, from: data)
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
                  [9, 10, 13, 32].contains(bytes[index]) {
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
                    guard let decoded = try JSONSerialization.jsonObject(
                        with: Data("[\(String(decoding: encoded, as: UTF8.self))]".utf8)
                    ) as? [String],
                    let value = decoded.first else {
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
                  bytes[index] == 0x2D ||
                    bytes[index] == 0x2B ||
                    bytes[index] == 0x2E ||
                    bytes[index] == 0x65 ||
                    bytes[index] == 0x45 ||
                    (bytes[index] >= 0x30 && bytes[index] <= 0x39) {
                index += 1
            }
            guard index > start else { throw LocalProductWireError.invalidJSON }
            let token = String(decoding: bytes[start..<index], as: UTF8.self)
            guard Double(token) != nil else { throw LocalProductWireError.invalidJSON }
        }

        mutating func consume(_ literal: String) throws {
            let expected = Array(literal.utf8)
            guard index + expected.count <= bytes.count,
                  Array(bytes[index..<(index + expected.count)]) == expected else {
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
