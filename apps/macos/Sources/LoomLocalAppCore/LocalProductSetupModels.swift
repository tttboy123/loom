import Foundation

private struct SetupAnyCodingKey: CodingKey {
    let stringValue: String
    let intValue: Int? = nil

    init?(stringValue: String) {
        self.stringValue = stringValue
    }

    init?(intValue: Int) {
        return nil
    }
}

private func rejectUnknownSetupKeys(
    _ decoder: Decoder,
    allowed: Set<String>
) throws {
    let container = try decoder.container(keyedBy: SetupAnyCodingKey.self)
    if container.allKeys.contains(where: { !allowed.contains($0.stringValue) }) {
        throw LocalProductWireError.unknownField
    }
}

public struct LocalProductProviderSetupStatus:
    Codable, Equatable, Sendable
{
    public let providerID: String
    public let authMode: String
    public let credentialReference: String
    public let revision: Int64
    public let status: String
    public let reason: String

    enum CodingKeys: String, CodingKey {
        case providerID = "provider_id"
        case authMode = "auth_mode"
        case credentialReference = "credential_reference"
        case revision, status, reason
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownSetupKeys(
            decoder,
            allowed: [
                "provider_id", "auth_mode", "credential_reference",
                "revision", "status", "reason",
            ]
        )
        let values = try decoder.container(keyedBy: CodingKeys.self)
        providerID = try values.decode(String.self, forKey: .providerID)
        authMode = try values.decode(String.self, forKey: .authMode)
        credentialReference = try values.decode(
            String.self,
            forKey: .credentialReference
        )
        revision = try values.decode(Int64.self, forKey: .revision)
        status = try values.decode(String.self, forKey: .status)
        reason = try values.decode(String.self, forKey: .reason)
    }
}

public struct LocalProductSetupRuntime:
    Codable, Equatable, Sendable, Identifiable
{
    public var id: String { runtimeInstanceID }
    public let runtimeInstanceID: String
    public let displayName: String
    public let adapterType: String
    public let executableVersion: String
    public let status: String
    public let capacity: Int
    public let modelID: String
    public let modelIDs: [String]
    public let observedCapabilities: [String]
    public let sourceProbeID: String

    enum CodingKeys: String, CodingKey {
        case runtimeInstanceID = "runtime_instance_id"
        case displayName = "display_name"
        case adapterType = "adapter_type"
        case executableVersion = "executable_version"
        case status, capacity
        case modelID = "model_id"
        case modelIDs = "model_ids"
        case observedCapabilities = "observed_capabilities"
        case sourceProbeID = "source_probe_id"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownSetupKeys(decoder, allowed: [
            "runtime_instance_id", "display_name", "adapter_type",
            "executable_version", "status", "capacity", "model_id",
            "model_ids", "observed_capabilities", "source_probe_id",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        runtimeInstanceID = try values.decode(String.self, forKey: .runtimeInstanceID)
        displayName = try values.decode(String.self, forKey: .displayName)
        adapterType = try values.decode(String.self, forKey: .adapterType)
        executableVersion = try values.decode(String.self, forKey: .executableVersion)
        status = try values.decode(String.self, forKey: .status)
        capacity = try values.decode(Int.self, forKey: .capacity)
        modelID = try values.decodeIfPresent(String.self, forKey: .modelID) ?? ""
        modelIDs = try values.decode([String].self, forKey: .modelIDs)
        observedCapabilities = try values.decode(
            [String].self,
            forKey: .observedCapabilities
        )
        sourceProbeID = try values.decode(String.self, forKey: .sourceProbeID)
    }
}

public struct LocalProductSetupSavedTeam:
    Codable, Equatable, Sendable, Identifiable
{
    public let id: String
    public let version: Int
    public let name: String
    public let status: String
    public let definitionDigest: String
    public let streamHead: Int64

    enum CodingKeys: String, CodingKey {
        case id, version, name, status
        case definitionDigest = "definition_digest"
        case streamHead = "stream_head"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownSetupKeys(
            decoder,
            allowed: [
                "id", "version", "name", "status", "definition_digest",
                "stream_head",
            ]
        )
        let values = try decoder.container(keyedBy: CodingKeys.self)
        id = try values.decode(String.self, forKey: .id)
        version = try values.decode(Int.self, forKey: .version)
        name = try values.decode(String.self, forKey: .name)
        status = try values.decode(String.self, forKey: .status)
        definitionDigest = try values.decode(String.self, forKey: .definitionDigest)
        streamHead = try values.decode(Int64.self, forKey: .streamHead)
    }
}

public struct LocalProductSetupTemplate:
    Codable, Equatable, Sendable, Identifiable
{
    public let id: String
    public let version: Int
    public let digest: String
    public let name: String
    public let purpose: String
    public let mainRoleID: String
    public let subagentRoleIDs: [String]
    public let requestedConcurrency: Int
    public let maximumBudgetCredits: Int

    enum CodingKeys: String, CodingKey {
        case id, version, digest, name, purpose
        case mainRoleID = "main_role_id"
        case subagentRoleIDs = "subagent_role_ids"
        case requestedConcurrency = "requested_concurrency"
        case maximumBudgetCredits = "maximum_budget_credits"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownSetupKeys(decoder, allowed: [
            "id", "version", "digest", "name", "purpose", "main_role_id",
            "subagent_role_ids", "requested_concurrency",
            "maximum_budget_credits",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        id = try values.decode(String.self, forKey: .id)
        version = try values.decode(Int.self, forKey: .version)
        digest = try values.decode(String.self, forKey: .digest)
        name = try values.decode(String.self, forKey: .name)
        purpose = try values.decode(String.self, forKey: .purpose)
        mainRoleID = try values.decode(String.self, forKey: .mainRoleID)
        subagentRoleIDs = try values.decode([String].self, forKey: .subagentRoleIDs)
        requestedConcurrency = try values.decode(
            Int.self,
            forKey: .requestedConcurrency
        )
        maximumBudgetCredits = try values.decode(
            Int.self,
            forKey: .maximumBudgetCredits
        )
    }
}

public struct LocalProductSetupRoleOption:
    Codable, Equatable, Sendable, Identifiable
{
    public let id: String
    public let kind: String
    public let agentDefinitionID: String
    public let runtimeProfileID: String
    public let runtimeInstanceID: String
    public let skillRevisionIDs: [String]
    public let permissionIDs: [String]
    public let resourceIDs: [String]
    public let responsibility: String

    enum CodingKeys: String, CodingKey {
        case id, kind
        case agentDefinitionID = "agent_definition_id"
        case runtimeProfileID = "runtime_profile_id"
        case runtimeInstanceID = "runtime_instance_id"
        case skillRevisionIDs = "skill_revision_ids"
        case permissionIDs = "permission_ids"
        case resourceIDs = "resource_ids"
        case responsibility
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownSetupKeys(decoder, allowed: [
            "id", "kind", "agent_definition_id", "runtime_profile_id",
            "runtime_instance_id", "skill_revision_ids", "permission_ids",
            "resource_ids", "responsibility",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        id = try values.decode(String.self, forKey: .id)
        kind = try values.decode(String.self, forKey: .kind)
        agentDefinitionID = try values.decode(String.self, forKey: .agentDefinitionID)
        runtimeProfileID = try values.decode(String.self, forKey: .runtimeProfileID)
        runtimeInstanceID = try values.decode(String.self, forKey: .runtimeInstanceID)
        skillRevisionIDs = try values.decode([String].self, forKey: .skillRevisionIDs)
        permissionIDs = try values.decode([String].self, forKey: .permissionIDs)
        resourceIDs = try values.decode([String].self, forKey: .resourceIDs)
        responsibility = try values.decode(String.self, forKey: .responsibility)
    }
}

public struct LocalProductSetupSkill:
    Codable, Equatable, Sendable, Identifiable
{
    public var id: String { "\(skillID)@\(revision)" }
    public let skillID: String
    public let revision: Int
    public let digest: String
    public let sourceScope: String
    public let risk: String
    public let compatibleRuntimes: [String]

    enum CodingKeys: String, CodingKey {
        case skillID = "id"
        case revision, digest
        case sourceScope = "source_scope"
        case risk
        case compatibleRuntimes = "compatible_runtimes"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownSetupKeys(decoder, allowed: [
            "id", "revision", "digest", "source_scope", "risk",
            "compatible_runtimes",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        skillID = try values.decode(String.self, forKey: .skillID)
        revision = try values.decode(Int.self, forKey: .revision)
        digest = try values.decode(String.self, forKey: .digest)
        sourceScope = try values.decode(String.self, forKey: .sourceScope)
        risk = try values.decode(String.self, forKey: .risk)
        compatibleRuntimes = try values.decode(
            [String].self,
            forKey: .compatibleRuntimes
        )
    }
}

public struct LocalProductSetupResource:
    Codable, Equatable, Sendable, Identifiable
{
    public let id: String
    public let kind: String

    public init(from decoder: Decoder) throws {
        try rejectUnknownSetupKeys(decoder, allowed: ["id", "kind"])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        id = try values.decode(String.self, forKey: .id)
        kind = try values.decode(String.self, forKey: .kind)
    }

    enum CodingKeys: String, CodingKey {
        case id, kind
    }
}

public struct LocalProductSetupSnapshot: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let viewVersion: String
    public let codex: LocalProductProviderSetupStatus
    public let miniMax: LocalProductProviderSetupStatus
    public let runtimes: [LocalProductSetupRuntime]
    public let savedTeams: [LocalProductSetupSavedTeam]
    public let templates: [LocalProductSetupTemplate]
    public let roleOptions: [LocalProductSetupRoleOption]
    public let skills: [LocalProductSetupSkill]
    public let permissions: [String]
    public let resources: [LocalProductSetupResource]

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case viewVersion = "view_version"
        case codex, runtimes, templates, skills, permissions, resources
        case miniMax = "minimax"
        case savedTeams = "saved_teams"
        case roleOptions = "role_options"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownSetupKeys(decoder, allowed: [
            "schema_version", "view_version", "codex", "minimax", "runtimes",
            "saved_teams", "templates", "role_options", "skills",
            "permissions", "resources",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
        guard schemaVersion == 1 else {
            throw LocalProductWireError.unsupportedSchema
        }
        viewVersion = try values.decode(String.self, forKey: .viewVersion)
        codex = try values.decode(
            LocalProductProviderSetupStatus.self,
            forKey: .codex
        )
        miniMax = try values.decode(
            LocalProductProviderSetupStatus.self,
            forKey: .miniMax
        )
        runtimes = try values.decode(
            [LocalProductSetupRuntime].self,
            forKey: .runtimes
        )
        savedTeams = try values.decode(
            [LocalProductSetupSavedTeam].self,
            forKey: .savedTeams
        )
        templates = try values.decode(
            [LocalProductSetupTemplate].self,
            forKey: .templates
        )
        roleOptions = try values.decode(
            [LocalProductSetupRoleOption].self,
            forKey: .roleOptions
        )
        skills = try values.decode(
            [LocalProductSetupSkill].self,
            forKey: .skills
        )
        permissions = try values.decode([String].self, forKey: .permissions)
        resources = try values.decode(
            [LocalProductSetupResource].self,
            forKey: .resources
        )
    }
}

public struct LocalProductBuilderQuestionOption:
    Codable, Equatable, Sendable, Identifiable
{
    public let id: String
    public let label: String

    public init(from decoder: Decoder) throws {
        try rejectUnknownSetupKeys(decoder, allowed: ["id", "label"])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        id = try values.decode(String.self, forKey: .id)
        label = try values.decode(String.self, forKey: .label)
    }

    enum CodingKeys: String, CodingKey {
        case id, label
    }
}

public struct LocalProductBuilderQuestion:
    Codable, Equatable, Sendable
{
    public let id: String
    public let prompt: String
    public let options: [LocalProductBuilderQuestionOption]

    public init(from decoder: Decoder) throws {
        try rejectUnknownSetupKeys(
            decoder,
            allowed: ["id", "prompt", "options"]
        )
        let values = try decoder.container(keyedBy: CodingKeys.self)
        id = try values.decode(String.self, forKey: .id)
        prompt = try values.decode(String.self, forKey: .prompt)
        options = try values.decode(
            [LocalProductBuilderQuestionOption].self,
            forKey: .options
        )
    }

    enum CodingKeys: String, CodingKey {
        case id, prompt, options
    }
}

public struct LocalProductBuilderRole:
    Codable, Equatable, Sendable, Identifiable
{
    public var id: String { "\(kind):\(agentDefinitionID)" }
    public let kind: String
    public let agentDefinitionID: String
    public let agentVersion: Int
    public let agentScope: String
    public let displayName: String
    public let responsibility: String
    public let runtime: LocalProductSetupRuntime
    public let runtimeProfileID: String
    public let modelID: String
    public let authMode: String
    public let skills: [LocalProductSetupSkill]
    public let permissionIDs: [String]
    public let resourceIDs: [String]
    public let compatible: Bool
    public let compatibilityReason: String

    enum CodingKeys: String, CodingKey {
        case kind
        case agentDefinitionID = "agent_definition_id"
        case agentVersion = "agent_version"
        case agentScope = "agent_scope"
        case displayName = "display_name"
        case responsibility, runtime
        case runtimeProfileID = "runtime_profile_id"
        case modelID = "model_id"
        case authMode = "auth_mode"
        case skills
        case permissionIDs = "permission_ids"
        case resourceIDs = "resource_ids"
        case compatible
        case compatibilityReason = "compatibility_reason"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownSetupKeys(decoder, allowed: [
            "kind", "agent_definition_id", "agent_version", "agent_scope",
            "display_name", "responsibility", "runtime", "runtime_profile_id",
            "model_id", "auth_mode", "skills", "permission_ids",
            "resource_ids", "compatible", "compatibility_reason",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        kind = try values.decode(String.self, forKey: .kind)
        agentDefinitionID = try values.decode(String.self, forKey: .agentDefinitionID)
        agentVersion = try values.decode(Int.self, forKey: .agentVersion)
        agentScope = try values.decode(String.self, forKey: .agentScope)
        displayName = try values.decode(String.self, forKey: .displayName)
        responsibility = try values.decode(String.self, forKey: .responsibility)
        runtime = try values.decode(LocalProductSetupRuntime.self, forKey: .runtime)
        runtimeProfileID = try values.decode(String.self, forKey: .runtimeProfileID)
        modelID = try values.decode(String.self, forKey: .modelID)
        authMode = try values.decode(String.self, forKey: .authMode)
        skills = try values.decode([LocalProductSetupSkill].self, forKey: .skills)
        permissionIDs = try values.decode([String].self, forKey: .permissionIDs)
        resourceIDs = try values.decode([String].self, forKey: .resourceIDs)
        compatible = try values.decode(Bool.self, forKey: .compatible)
        compatibilityReason = try values.decode(
            String.self,
            forKey: .compatibilityReason
        )
    }
}

public struct LocalProductBuilderPreview: Codable, Equatable, Sendable {
    public let name: String
    public let purpose: String
    public let roles: [LocalProductBuilderRole]
    public let permissions: [String]
    public let resources: [String]
    public let compatibilityGaps: [String]
    public let requestedConcurrency: Int
    public let maximumBudgetCredits: Int
    public let estimatedMaximumCost: String

    enum CodingKeys: String, CodingKey {
        case name, purpose, roles, permissions, resources
        case compatibilityGaps = "compatibility_gaps"
        case requestedConcurrency = "requested_concurrency"
        case maximumBudgetCredits = "maximum_budget_credits"
        case estimatedMaximumCost = "estimated_maximum_cost"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownSetupKeys(decoder, allowed: [
            "name", "purpose", "roles", "permissions", "resources",
            "compatibility_gaps", "requested_concurrency",
            "maximum_budget_credits", "estimated_maximum_cost",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        name = try values.decode(String.self, forKey: .name)
        purpose = try values.decode(String.self, forKey: .purpose)
        roles = try values.decode([LocalProductBuilderRole].self, forKey: .roles)
        permissions = try values.decode([String].self, forKey: .permissions)
        resources = try values.decode([String].self, forKey: .resources)
        compatibilityGaps = try values.decode(
            [String].self,
            forKey: .compatibilityGaps
        )
        requestedConcurrency = try values.decode(
            Int.self,
            forKey: .requestedConcurrency
        )
        maximumBudgetCredits = try values.decode(
            Int.self,
            forKey: .maximumBudgetCredits
        )
        estimatedMaximumCost = try values.decode(
            String.self,
            forKey: .estimatedMaximumCost
        )
    }
}

public struct LocalProductBuilderSession: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let draftID: String
    public let revision: Int
    public let source: String
    public let catalogDigest: String
    public let viewVersion: String
    public let contentDigest: String
    public let bindingDigest: String
    public let question: LocalProductBuilderQuestion
    public let preview: LocalProductBuilderPreview
    public let canConfirm: Bool

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case draftID = "draft_id"
        case revision, source
        case catalogDigest = "catalog_digest"
        case viewVersion = "view_version"
        case contentDigest = "content_digest"
        case bindingDigest = "binding_digest"
        case question, preview
        case canConfirm = "can_confirm"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownSetupKeys(decoder, allowed: [
            "schema_version", "draft_id", "revision", "source",
            "catalog_digest", "view_version", "content_digest",
            "binding_digest", "question", "preview", "can_confirm",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
        guard schemaVersion == 1 else {
            throw LocalProductWireError.unsupportedSchema
        }
        draftID = try values.decode(String.self, forKey: .draftID)
        revision = try values.decode(Int.self, forKey: .revision)
        source = try values.decode(String.self, forKey: .source)
        catalogDigest = try values.decode(String.self, forKey: .catalogDigest)
        viewVersion = try values.decode(String.self, forKey: .viewVersion)
        contentDigest = try values.decode(String.self, forKey: .contentDigest)
        bindingDigest = try values.decode(String.self, forKey: .bindingDigest)
        question = try values.decode(
            LocalProductBuilderQuestion.self,
            forKey: .question
        )
        preview = try values.decode(
            LocalProductBuilderPreview.self,
            forKey: .preview
        )
        canConfirm = try values.decode(Bool.self, forKey: .canConfirm)
    }
}

public struct LocalProductBuilderConfirmation:
    Codable, Equatable, Sendable
{
    public let teamDefinitionID: String
    public let teamDefinitionVersion: Int
    public let teamDefinitionDigest: String
    public let status: String
    public let teamInstanceCreated: Bool
    public let runCreated: Bool

    enum CodingKeys: String, CodingKey {
        case teamDefinitionID = "team_definition_id"
        case teamDefinitionVersion = "team_definition_version"
        case teamDefinitionDigest = "team_definition_digest"
        case status
        case teamInstanceCreated = "team_instance_created"
        case runCreated = "run_created"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownSetupKeys(decoder, allowed: [
            "team_definition_id", "team_definition_version",
            "team_definition_digest", "status", "team_instance_created",
            "run_created",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        teamDefinitionID = try values.decode(
            String.self,
            forKey: .teamDefinitionID
        )
        teamDefinitionVersion = try values.decode(
            Int.self,
            forKey: .teamDefinitionVersion
        )
        teamDefinitionDigest = try values.decode(
            String.self,
            forKey: .teamDefinitionDigest
        )
        status = try values.decode(String.self, forKey: .status)
        teamInstanceCreated = try values.decode(
            Bool.self,
            forKey: .teamInstanceCreated
        )
        runCreated = try values.decode(Bool.self, forKey: .runCreated)
    }
}

public struct LocalProductCredentialSetupResult:
    Codable, Equatable, Sendable
{
    public let providerID: String
    public let revision: Int64
    public let status: String
    public let reason: String

    enum CodingKeys: String, CodingKey {
        case providerID = "provider_id"
        case revision, status, reason
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownSetupKeys(
            decoder,
            allowed: ["provider_id", "revision", "status", "reason"]
        )
        let values = try decoder.container(keyedBy: CodingKeys.self)
        providerID = try values.decode(String.self, forKey: .providerID)
        revision = try values.decode(Int64.self, forKey: .revision)
        status = try values.decode(String.self, forKey: .status)
        reason = try values.decode(String.self, forKey: .reason)
    }
}

public enum LocalProductSetupWire {
    public static func decodeSnapshot(
        _ data: Data
    ) throws -> LocalProductSetupSnapshot {
        try decode(LocalProductSetupSnapshot.self, data)
    }

    public static func decodeBuilderSession(
        _ data: Data
    ) throws -> LocalProductBuilderSession {
        try decode(LocalProductBuilderSession.self, data)
    }

    public static func decodeBuilderConfirmation(
        _ data: Data
    ) throws -> LocalProductBuilderConfirmation {
        try decode(LocalProductBuilderConfirmation.self, data)
    }

    public static func decodeCredentialResult(
        _ data: Data
    ) throws -> LocalProductCredentialSetupResult {
        try decode(LocalProductCredentialSetupResult.self, data)
    }

    public static func decodeSavedTeam(
        _ data: Data
    ) throws -> LocalProductSetupSavedTeam {
        try decode(LocalProductSetupSavedTeam.self, data)
    }

    private static func decode<T: Decodable>(
        _ type: T.Type,
        _ data: Data
    ) throws -> T {
        do {
            return try JSONDecoder().decode(type, from: data)
        } catch let error as LocalProductWireError {
            throw error
        } catch {
            throw LocalProductWireError.invalidJSON
        }
    }
}
