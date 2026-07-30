import Foundation

public enum LocalProductDecisionKind:
    String, Codable, CaseIterable, Sendable
{
    case authorization
    case review
    case recovery
}

public struct LocalProductDecisionSheet:
    Decodable, Equatable, Sendable, Identifiable
{
    public var id: String { decisionID }
    public let schemaVersion: Int
    public let kind: LocalProductDecisionKind
    public let missionID: String
    public let teamInstanceID: String
    public let viewVersion: String
    public let decisionID: String
    public let decisionDigest: String
    public let title: String
    public let summary: String
    public let requester: String
    public let target: String
    public let commandType: String
    public let networkAccess: String
    public let credentialAccess: String
    public let permissionScope: String
    public let attemptScope: String
    public let expectedEvidence: String
    public let technicalDetails: [String]
    public let actions: [String]
    public let preparedActions: [String]
    public let prepared: Bool
    public let logicalNodeID: String
    public let attemptNumber: Int
    public let claimGeneration: Int64

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case kind
        case missionID = "mission_id"
        case teamInstanceID = "team_instance_id"
        case viewVersion = "view_version"
        case decisionID = "decision_id"
        case decisionDigest = "decision_digest"
        case title, summary, requester, target
        case commandType = "command_type"
        case networkAccess = "network_access"
        case credentialAccess = "credential_access"
        case permissionScope = "permission_scope"
        case attemptScope = "attempt_scope"
        case expectedEvidence = "expected_evidence"
        case technicalDetails = "technical_details"
        case actions
        case preparedActions = "prepared_actions"
        case prepared
        case logicalNodeID = "logical_node_id"
        case attemptNumber = "attempt_number"
        case claimGeneration = "claim_generation"
    }

    public init(
        schemaVersion: Int = 1,
        kind: LocalProductDecisionKind,
        missionID: String,
        teamInstanceID: String,
        viewVersion: String,
        decisionID: String,
        decisionDigest: String,
        title: String,
        summary: String,
        requester: String,
        target: String,
        commandType: String,
        networkAccess: String,
        credentialAccess: String,
        permissionScope: String,
        attemptScope: String,
        expectedEvidence: String,
        technicalDetails: [String],
        actions: [String],
        preparedActions: [String],
        prepared: Bool,
        logicalNodeID: String,
        attemptNumber: Int,
        claimGeneration: Int64
    ) {
        self.schemaVersion = schemaVersion
        self.kind = kind
        self.missionID = missionID
        self.teamInstanceID = teamInstanceID
        self.viewVersion = viewVersion
        self.decisionID = decisionID
        self.decisionDigest = decisionDigest
        self.title = title
        self.summary = summary
        self.requester = requester
        self.target = target
        self.commandType = commandType
        self.networkAccess = networkAccess
        self.credentialAccess = credentialAccess
        self.permissionScope = permissionScope
        self.attemptScope = attemptScope
        self.expectedEvidence = expectedEvidence
        self.technicalDetails = technicalDetails
        self.actions = actions
        self.preparedActions = preparedActions
        self.prepared = prepared
        self.logicalNodeID = logicalNodeID
        self.attemptNumber = attemptNumber
        self.claimGeneration = claimGeneration
    }

    public init(from decoder: Decoder) throws {
        try rejectDecisionUnknownKeys(decoder, allowed: [
            "schema_version", "kind", "mission_id", "team_instance_id",
            "view_version", "decision_id", "decision_digest", "title",
            "summary", "requester", "target", "command_type",
            "network_access", "credential_access", "permission_scope",
            "attempt_scope", "expected_evidence", "technical_details",
            "actions", "prepared_actions", "prepared", "logical_node_id", "attempt_number",
            "claim_generation",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
        guard schemaVersion == 1 else {
            throw LocalProductWireError.unsupportedSchema
        }
        kind = try values.decode(LocalProductDecisionKind.self, forKey: .kind)
        missionID = try values.decode(String.self, forKey: .missionID)
        teamInstanceID = try values.decode(String.self, forKey: .teamInstanceID)
        viewVersion = try values.decode(String.self, forKey: .viewVersion)
        decisionID = try values.decode(String.self, forKey: .decisionID)
        decisionDigest = try values.decode(String.self, forKey: .decisionDigest)
        title = try values.decode(String.self, forKey: .title)
        summary = try values.decode(String.self, forKey: .summary)
        requester = try values.decode(String.self, forKey: .requester)
        target = try values.decode(String.self, forKey: .target)
        commandType = try values.decode(String.self, forKey: .commandType)
        networkAccess = try values.decode(String.self, forKey: .networkAccess)
        credentialAccess = try values.decode(
            String.self,
            forKey: .credentialAccess
        )
        permissionScope = try values.decode(
            String.self,
            forKey: .permissionScope
        )
        attemptScope = try values.decode(String.self, forKey: .attemptScope)
        expectedEvidence = try values.decode(
            String.self,
            forKey: .expectedEvidence
        )
        technicalDetails = try values.decode(
            [String].self,
            forKey: .technicalDetails
        )
        actions = try values.decode([String].self, forKey: .actions)
        preparedActions = try values.decode(
            [String].self,
            forKey: .preparedActions
        )
        prepared = try values.decode(Bool.self, forKey: .prepared)
        logicalNodeID = try values.decode(String.self, forKey: .logicalNodeID)
        attemptNumber = try values.decode(Int.self, forKey: .attemptNumber)
        claimGeneration = try values.decode(
            Int64.self,
            forKey: .claimGeneration
        )
    }
}

public struct LocalProductDecisionCommand:
    Codable, Equatable, Sendable
{
    public let schemaVersion: Int
    public let operation: String
    public let kind: LocalProductDecisionKind
    public let action: String
    public let missionID: String
    public let teamInstanceID: String
    public let viewVersion: String
    public let decisionID: String
    public let decisionDigest: String
    public let logicalNodeID: String
    public let attemptNumber: Int
    public let claimGeneration: Int64
    public let correlationID: String

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case operation, kind, action
        case missionID = "mission_id"
        case teamInstanceID = "team_instance_id"
        case viewVersion = "view_version"
        case decisionID = "decision_id"
        case decisionDigest = "decision_digest"
        case logicalNodeID = "logical_node_id"
        case attemptNumber = "attempt_number"
        case claimGeneration = "claim_generation"
        case correlationID = "correlation_id"
    }

    public init(
        schemaVersion: Int = 1,
        operation: String = "submit",
        kind: LocalProductDecisionKind,
        action: String,
        missionID: String,
        teamInstanceID: String,
        viewVersion: String,
        decisionID: String,
        decisionDigest: String,
        logicalNodeID: String,
        attemptNumber: Int,
        claimGeneration: Int64,
        correlationID: String
    ) {
        self.schemaVersion = schemaVersion
        self.operation = operation
        self.kind = kind
        self.action = action
        self.missionID = missionID
        self.teamInstanceID = teamInstanceID
        self.viewVersion = viewVersion
        self.decisionID = decisionID
        self.decisionDigest = decisionDigest
        self.logicalNodeID = logicalNodeID
        self.attemptNumber = attemptNumber
        self.claimGeneration = claimGeneration
        self.correlationID = correlationID
    }

    public init(from decoder: Decoder) throws {
        try rejectDecisionUnknownKeys(decoder, allowed: [
            "schema_version", "operation", "kind", "action", "mission_id",
            "team_instance_id", "view_version", "decision_id",
            "decision_digest", "logical_node_id", "attempt_number",
            "claim_generation", "correlation_id",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
        guard schemaVersion == 1 else {
            throw LocalProductWireError.unsupportedSchema
        }
        operation = try values.decode(String.self, forKey: .operation)
        kind = try values.decode(LocalProductDecisionKind.self, forKey: .kind)
        action = try values.decode(String.self, forKey: .action)
        missionID = try values.decode(String.self, forKey: .missionID)
        teamInstanceID = try values.decode(String.self, forKey: .teamInstanceID)
        viewVersion = try values.decode(String.self, forKey: .viewVersion)
        decisionID = try values.decode(String.self, forKey: .decisionID)
        decisionDigest = try values.decode(String.self, forKey: .decisionDigest)
        logicalNodeID = try values.decode(String.self, forKey: .logicalNodeID)
        attemptNumber = try values.decode(Int.self, forKey: .attemptNumber)
        claimGeneration = try values.decode(Int64.self, forKey: .claimGeneration)
        correlationID = try values.decode(String.self, forKey: .correlationID)
    }
}

public struct LocalProductDecisionResult:
    Decodable, Equatable, Sendable
{
    public let schemaVersion: Int
    public let missionID: String
    public let decisionID: String
    public let status: String
    public let authoritative: Bool
    public let viewVersion: String

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case missionID = "mission_id"
        case decisionID = "decision_id"
        case status, authoritative
        case viewVersion = "view_version"
    }

    public init(from decoder: Decoder) throws {
        try rejectDecisionUnknownKeys(decoder, allowed: [
            "schema_version", "mission_id", "decision_id", "status",
            "authoritative", "view_version",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
        guard schemaVersion == 1 else {
            throw LocalProductWireError.unsupportedSchema
        }
        missionID = try values.decode(String.self, forKey: .missionID)
        decisionID = try values.decode(String.self, forKey: .decisionID)
        status = try values.decode(String.self, forKey: .status)
        authoritative = try values.decode(Bool.self, forKey: .authoritative)
        viewVersion = try values.decode(String.self, forKey: .viewVersion)
    }
}

public enum LocalProductDecisionWire {
    public static func decodeSheet(
        _ data: Data
    ) throws -> LocalProductDecisionSheet {
        try decode(LocalProductDecisionSheet.self, from: data)
    }

    public static func decodeResult(
        _ data: Data
    ) throws -> LocalProductDecisionResult {
        try decode(LocalProductDecisionResult.self, from: data)
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

private struct DecisionCodingKey: CodingKey {
    let stringValue: String
    let intValue: Int? = nil
    init?(stringValue: String) { self.stringValue = stringValue }
    init?(intValue: Int) { return nil }
}

private func rejectDecisionUnknownKeys(
    _ decoder: Decoder,
    allowed: Set<String>
) throws {
    let values = try decoder.container(keyedBy: DecisionCodingKey.self)
    if values.allKeys.contains(where: { !allowed.contains($0.stringValue) }) {
        throw LocalProductWireError.unknownField
    }
}
