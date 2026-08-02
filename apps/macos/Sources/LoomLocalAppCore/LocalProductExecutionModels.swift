import Foundation

private struct ExecutionCodingKey: CodingKey {
    let stringValue: String
    let intValue: Int? = nil
    init?(stringValue: String) { self.stringValue = stringValue }
    init?(intValue: Int) { return nil }
}

private func rejectExecutionUnknownKeys(
    _ decoder: Decoder,
    allowed: Set<String>
) throws {
    let values = try decoder.container(keyedBy: ExecutionCodingKey.self)
    if values.allKeys.contains(where: { !allowed.contains($0.stringValue) }) {
        throw LocalProductWireError.unknownField
    }
}

public struct LocalProductExecutionCommand: Encodable, Equatable, Sendable {
    public let schemaVersion: Int
    public let operation: String
    public let missionID: String
    public let teamInstanceID: String
    public let workPackageID: String
    public let workPackageDigest: String
    public let objective: String
    public let expectedViewVersion: String
    public let preflightDigest: String
    public let controlAction: String
    public let executionDigest: String
    public let logicalNodeID: String
    public let attemptNumber: Int
    public let claimGeneration: Int64
    public let correlationID: String

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case operation
        case missionID = "mission_id"
        case teamInstanceID = "team_instance_id"
        case workPackageID = "work_package_id"
        case workPackageDigest = "work_package_digest"
        case objective
        case expectedViewVersion = "expected_view_version"
        case preflightDigest = "preflight_digest"
        case controlAction = "control_action"
        case executionDigest = "execution_digest"
        case logicalNodeID = "logical_node_id"
        case attemptNumber = "attempt_number"
        case claimGeneration = "claim_generation"
        case correlationID = "correlation_id"
    }

    public static func preflight(
        missionID: String,
        teamInstanceID: String,
        workPackageID: String,
        workPackageDigest: String,
        objective: String,
        expectedViewVersion: String,
        correlationID: String
    ) -> Self {
        .init(
            schemaVersion: 1,
            operation: "preflight",
            missionID: missionID,
            teamInstanceID: teamInstanceID,
            workPackageID: workPackageID,
            workPackageDigest: workPackageDigest,
            objective: objective,
            expectedViewVersion: expectedViewVersion,
            preflightDigest: "",
            controlAction: "",
            executionDigest: "",
            logicalNodeID: "",
            attemptNumber: 0,
            claimGeneration: 0,
            correlationID: correlationID
        )
    }

    public static func start(
        preflight: LocalProductExecutionPreflight,
        objective: String,
        correlationID: String
    ) -> Self {
        .init(
            schemaVersion: 1,
            operation: "start",
            missionID: preflight.missionID,
            teamInstanceID: preflight.teamInstanceID,
            workPackageID: preflight.workPackageID,
            workPackageDigest: preflight.workPackageDigest,
            objective: objective,
            expectedViewVersion: preflight.viewVersion,
            preflightDigest: preflight.preflightDigest,
            controlAction: "",
            executionDigest: "",
            logicalNodeID: "",
            attemptNumber: 0,
            claimGeneration: 0,
            correlationID: correlationID
        )
    }

    public static func control(
        missionID: String,
        teamInstanceID: String,
        expectedViewVersion: String,
        action: String,
        executionDigest: String,
        logicalNodeID: String,
        attemptNumber: Int,
        claimGeneration: Int64,
        correlationID: String
    ) -> Self {
        .init(
            schemaVersion: 1,
            operation: "control",
            missionID: missionID,
            teamInstanceID: teamInstanceID,
            workPackageID: "",
            workPackageDigest: "",
            objective: "",
            expectedViewVersion: expectedViewVersion,
            preflightDigest: "",
            controlAction: action,
            executionDigest: executionDigest,
            logicalNodeID: logicalNodeID,
            attemptNumber: attemptNumber,
            claimGeneration: claimGeneration,
            correlationID: correlationID
        )
    }

    public func encode(to encoder: Encoder) throws {
        var values = encoder.container(keyedBy: CodingKeys.self)
        try values.encode(schemaVersion, forKey: .schemaVersion)
        try values.encode(operation, forKey: .operation)
        try values.encode(missionID, forKey: .missionID)
        try values.encode(teamInstanceID, forKey: .teamInstanceID)
        try values.encode(expectedViewVersion, forKey: .expectedViewVersion)
        try values.encode(correlationID, forKey: .correlationID)
        for (key, value) in [
            (CodingKeys.workPackageID, workPackageID),
            (.workPackageDigest, workPackageDigest),
            (.objective, objective),
            (.preflightDigest, preflightDigest),
            (.controlAction, controlAction),
            (.executionDigest, executionDigest),
            (.logicalNodeID, logicalNodeID),
        ] where !value.isEmpty {
            try values.encode(value, forKey: key)
        }
        if attemptNumber != 0 { try values.encode(attemptNumber, forKey: .attemptNumber) }
        if claimGeneration != 0 { try values.encode(claimGeneration, forKey: .claimGeneration) }
    }
}

public struct LocalProductExecutionNodePreview: Decodable, Equatable, Sendable {
    public let logicalNodeID: String
    public let title: String
    public let role: String
    public let dependsOn: [String]
    public let maxAttempts: Int

    enum CodingKeys: String, CodingKey {
        case logicalNodeID = "logical_node_id"
        case title, role
        case dependsOn = "depends_on"
        case maxAttempts = "max_attempts"
    }

    public init(from decoder: Decoder) throws {
        try rejectExecutionUnknownKeys(
            decoder,
            allowed: [
                "logical_node_id", "title", "role", "depends_on", "max_attempts",
            ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        logicalNodeID = try values.decode(String.self, forKey: .logicalNodeID)
        title = try values.decode(String.self, forKey: .title)
        role = try values.decode(String.self, forKey: .role)
        dependsOn = try values.decode([String].self, forKey: .dependsOn)
        maxAttempts = try values.decode(Int.self, forKey: .maxAttempts)
    }
}

public struct LocalProductExecutionPreflight: Decodable, Equatable, Sendable {
    public let schemaVersion: Int
    public let missionID: String
    public let teamInstanceID: String
    public let workPackageID: String
    public let workPackageDigest: String
    public let viewVersion: String
    public let planDigest: String
    public let preflightDigest: String
    public let expiresAt: String
    public let runtimeInstanceID: String
    public let runtimeProfileID: String
    public let modelID: String
    public let authMode: String
    public let capacityAvailable: Int
    public let budgetStatus: String
    public let sideEffects: [String]
    public let permissionScopes: [String]
    public let approvalPoints: [String]
    public let nodes: [LocalProductExecutionNodePreview]

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case missionID = "mission_id"
        case teamInstanceID = "team_instance_id"
        case workPackageID = "work_package_id"
        case workPackageDigest = "work_package_digest"
        case viewVersion = "view_version"
        case planDigest = "plan_digest"
        case preflightDigest = "preflight_digest"
        case expiresAt = "expires_at"
        case runtimeInstanceID = "runtime_instance_id"
        case runtimeProfileID = "runtime_profile_id"
        case modelID = "model_id"
        case authMode = "auth_mode"
        case capacityAvailable = "capacity_available"
        case budgetStatus = "budget_status"
        case sideEffects = "side_effects"
        case permissionScopes = "permission_scopes"
        case approvalPoints = "approval_points"
        case nodes
    }

    public init(from decoder: Decoder) throws {
        try rejectExecutionUnknownKeys(decoder, allowed: Set(CodingKeys.all))
        let values = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
        guard schemaVersion == 1 else { throw LocalProductWireError.unsupportedSchema }
        missionID = try values.decode(String.self, forKey: .missionID)
        teamInstanceID = try values.decode(String.self, forKey: .teamInstanceID)
        workPackageID = try values.decode(String.self, forKey: .workPackageID)
        workPackageDigest = try values.decode(String.self, forKey: .workPackageDigest)
        viewVersion = try values.decode(String.self, forKey: .viewVersion)
        planDigest = try values.decode(String.self, forKey: .planDigest)
        preflightDigest = try values.decode(String.self, forKey: .preflightDigest)
        expiresAt = try values.decode(String.self, forKey: .expiresAt)
        runtimeInstanceID = try values.decode(String.self, forKey: .runtimeInstanceID)
        runtimeProfileID = try values.decode(String.self, forKey: .runtimeProfileID)
        modelID = try values.decode(String.self, forKey: .modelID)
        authMode = try values.decode(String.self, forKey: .authMode)
        capacityAvailable = try values.decode(Int.self, forKey: .capacityAvailable)
        budgetStatus = try values.decode(String.self, forKey: .budgetStatus)
        sideEffects = try values.decode([String].self, forKey: .sideEffects)
        permissionScopes = try values.decode([String].self, forKey: .permissionScopes)
        approvalPoints = try values.decode([String].self, forKey: .approvalPoints)
        nodes = try values.decode([LocalProductExecutionNodePreview].self, forKey: .nodes)
    }
}

extension LocalProductExecutionPreflight.CodingKeys {
    fileprivate static var all: [String] {
        [
            "schema_version", "mission_id", "team_instance_id", "work_package_id",
            "work_package_digest", "view_version", "plan_digest", "preflight_digest",
            "expires_at",
            "runtime_instance_id", "runtime_profile_id", "model_id", "auth_mode",
            "capacity_available", "budget_status", "side_effects", "permission_scopes",
            "approval_points", "nodes",
        ]
    }
}

public struct LocalProductExecutionResult: Decodable, Equatable, Sendable {
    public let schemaVersion: Int
    public let missionID: String
    public let teamInstanceID: String
    public let status: String
    public let viewVersion: String
    public let executionDigest: String

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case missionID = "mission_id"
        case teamInstanceID = "team_instance_id"
        case status
        case viewVersion = "view_version"
        case executionDigest = "execution_digest"
    }

    public init(from decoder: Decoder) throws {
        try rejectExecutionUnknownKeys(
            decoder,
            allowed: [
                "schema_version", "mission_id", "team_instance_id", "status",
                "view_version", "execution_digest",
            ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
        missionID = try values.decode(String.self, forKey: .missionID)
        teamInstanceID = try values.decode(String.self, forKey: .teamInstanceID)
        status = try values.decode(String.self, forKey: .status)
        viewVersion = try values.decode(String.self, forKey: .viewVersion)
        executionDigest = try values.decode(String.self, forKey: .executionDigest)
        guard schemaVersion == 1 else { throw LocalProductWireError.unsupportedSchema }
        guard
            [
                "running", "awaiting_recovery", "cancelled", "degraded", "blocked",
                "human_required", "succeeded", "failed",
            ].contains(status)
        else {
            throw LocalProductWireError.invalidValue
        }
    }
}

public struct LocalProductExecutionEnvelope: Decodable, Equatable, Sendable {
    public let schemaVersion: Int
    public let operation: String
    public let preflight: LocalProductExecutionPreflight?
    public let result: LocalProductExecutionResult?

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case operation, preflight, result
    }

    public init(from decoder: Decoder) throws {
        try rejectExecutionUnknownKeys(
            decoder,
            allowed: ["schema_version", "operation", "preflight", "result"]
        )
        let values = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
        operation = try values.decode(String.self, forKey: .operation)
        preflight = try values.decodeIfPresent(
            LocalProductExecutionPreflight.self,
            forKey: .preflight
        )
        result = try values.decodeIfPresent(LocalProductExecutionResult.self, forKey: .result)
        guard schemaVersion == 1 else {
            throw LocalProductWireError.unsupportedSchema
        }
        switch operation {
        case "preflight":
            guard preflight != nil, result == nil else {
                throw LocalProductWireError.invalidJSON
            }
        case "start", "control":
            guard preflight == nil, result != nil else {
                throw LocalProductWireError.invalidJSON
            }
        default:
            throw LocalProductWireError.invalidJSON
        }
    }
}

public protocol LocalProductExecutionClientProtocol: AnyObject {
    func executeMission(
        _ command: LocalProductExecutionCommand
    ) async throws -> LocalProductExecutionEnvelope
}

public enum LocalProductExecutionWire {
    public static func decodeEnvelope(_ data: Data) throws -> LocalProductExecutionEnvelope {
        do {
            try StrictJSONScanner.validate(data)
            return try JSONDecoder().decode(LocalProductExecutionEnvelope.self, from: data)
        } catch let error as LocalProductWireError {
            throw error
        } catch {
            throw LocalProductWireError.invalidJSON
        }
    }
}
