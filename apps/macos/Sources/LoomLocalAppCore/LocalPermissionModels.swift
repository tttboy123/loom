import Foundation

// B-P1 production Swift permission surface. Reads flow over the real daemon
// socket through the production LocalIPCClient framing. These models are
// projections of the Journal-authoritative permission facts and are never
// an authority themselves. The Swift client is read-only for permissions
// (permissions_snapshot / permissions_attention); all writes happen through
// the TUI / Go service layer.

public struct PermissionProfile: Codable, Equatable, Sendable {
    public var profileID: String
    public var generation: Int64
    public var digest: String
    public var mode: String
    public var rules: [PermissionRule]
    public var ownedPaths: [String]

    public init(
        profileID: String = "",
        generation: Int64 = 0,
        digest: String = "",
        mode: String = "",
        rules: [PermissionRule] = [],
        ownedPaths: [String] = []
    ) {
        self.profileID = profileID
        self.generation = generation
        self.digest = digest
        self.mode = mode
        self.rules = rules
        self.ownedPaths = ownedPaths
    }

    enum CodingKeys: String, CodingKey {
        case profileID = "profile_id"
        case generation, digest, mode, rules
        case ownedPaths = "owned_paths"
    }
}

public struct PermissionRule: Codable, Equatable, Sendable {
    public var ruleID: String
    public var scope: String
    public var scopeID: String
    public var action: String
    public var tool: String
    public var pattern: String

    public init(
        ruleID: String = "",
        scope: String = "",
        scopeID: String = "",
        action: String = "",
        tool: String = "",
        pattern: String = ""
    ) {
        self.ruleID = ruleID
        self.scope = scope
        self.scopeID = scopeID
        self.action = action
        self.tool = tool
        self.pattern = pattern
    }

    enum CodingKeys: String, CodingKey {
        case ruleID = "rule_id"
        case scope
        case scopeID = "scope_id"
        case action, tool, pattern
    }
}

public struct PermissionJobBinding: Codable, Equatable, Sendable {
    public var jobID: String
    public var profileID: String
    public var profileDigest: String
    public var profileGeneration: Int64
    public var boundAt: String

    public init(
        jobID: String = "",
        profileID: String = "",
        profileDigest: String = "",
        profileGeneration: Int64 = 0,
        boundAt: String = ""
    ) {
        self.jobID = jobID
        self.profileID = profileID
        self.profileDigest = profileDigest
        self.profileGeneration = profileGeneration
        self.boundAt = boundAt
    }

    enum CodingKeys: String, CodingKey {
        case jobID = "job_id"
        case profileID = "profile_id"
        case profileDigest = "profile_digest"
        case profileGeneration = "profile_generation"
        case boundAt = "bound_at"
    }
}

public struct PermissionGrant: Codable, Equatable, Sendable {
    public var grantID: String
    public var scope: String
    public var scopeID: String
    public var tool: String
    public var pattern: String
    public var issuedAt: String
    public var revokedAt: String

    public init(
        grantID: String = "",
        scope: String = "",
        scopeID: String = "",
        tool: String = "",
        pattern: String = "",
        issuedAt: String = "",
        revokedAt: String = ""
    ) {
        self.grantID = grantID
        self.scope = scope
        self.scopeID = scopeID
        self.tool = tool
        self.pattern = pattern
        self.issuedAt = issuedAt
        self.revokedAt = revokedAt
    }

    enum CodingKeys: String, CodingKey {
        case grantID = "grant_id"
        case scope
        case scopeID = "scope_id"
        case tool, pattern
        case issuedAt = "issued_at"
        case revokedAt = "revoked_at"
    }
}

public struct PermissionActivation: Codable, Equatable, Sendable {
    public var mode: String
    public var scope: String
    public var scopeID: String
    public var activatedAt: String
    public var deactivatedAt: String
    public var authorizedBy: String

    public init(
        mode: String = "",
        scope: String = "",
        scopeID: String = "",
        activatedAt: String = "",
        deactivatedAt: String = "",
        authorizedBy: String = ""
    ) {
        self.mode = mode
        self.scope = scope
        self.scopeID = scopeID
        self.activatedAt = activatedAt
        self.deactivatedAt = deactivatedAt
        self.authorizedBy = authorizedBy
    }

    enum CodingKeys: String, CodingKey {
        case mode, scope
        case scopeID = "scope_id"
        case activatedAt = "activated_at"
        case deactivatedAt = "deactivated_at"
        case authorizedBy = "authorized_by"
    }
}

public struct PermissionSnapshot: Codable, Equatable, Sendable {
    public var viewVersion: String
    public var profiles: [PermissionProfile]
    public var bindings: [PermissionJobBinding]
    public var rules: [PermissionRule]
    public var grants: [PermissionGrant]
    public var activations: [PermissionActivation]
    public var adminLock: Bool

    public init(
        viewVersion: String = "",
        profiles: [PermissionProfile] = [],
        bindings: [PermissionJobBinding] = [],
        rules: [PermissionRule] = [],
        grants: [PermissionGrant] = [],
        activations: [PermissionActivation] = [],
        adminLock: Bool = false
    ) {
        self.viewVersion = viewVersion
        self.profiles = profiles
        self.bindings = bindings
        self.rules = rules
        self.grants = grants
        self.activations = activations
        self.adminLock = adminLock
    }

    enum CodingKeys: String, CodingKey {
        case viewVersion = "view_version"
        case profiles, bindings, rules, grants, activations
        case adminLock = "admin_lock"
    }
}

public struct PermissionDecisionView: Codable, Equatable, Sendable {
    public var jobID: String
    public var approvalID: String
    public var verdict: String
    public var reason: String
    public var authorizationPath: String
    public var recordedAt: String

    public init(
        jobID: String = "",
        approvalID: String = "",
        verdict: String = "",
        reason: String = "",
        authorizationPath: String = "",
        recordedAt: String = ""
    ) {
        self.jobID = jobID
        self.approvalID = approvalID
        self.verdict = verdict
        self.reason = reason
        self.authorizationPath = authorizationPath
        self.recordedAt = recordedAt
    }

    enum CodingKeys: String, CodingKey {
        case jobID = "job_id"
        case approvalID = "approval_id"
        case verdict, reason
        case authorizationPath = "authorization_path"
        case recordedAt = "recorded_at"
    }
}

public struct PermissionAttention: Codable, Equatable, Sendable {
    public var viewVersion: String
    public var decisions: [PermissionDecisionView]

    public init(
        viewVersion: String = "",
        decisions: [PermissionDecisionView] = []
    ) {
        self.viewVersion = viewVersion
        self.decisions = decisions
    }

    enum CodingKeys: String, CodingKey {
        case viewVersion = "view_version"
        case decisions
    }
}

public enum PermissionWire {
    public static func decodeSnapshot(_ data: Data) throws -> PermissionSnapshot {
        try decode(data)
    }

    public static func decodeAttention(_ data: Data) throws -> PermissionAttention {
        try decode(data)
    }

    private static func decode<T: Decodable>(_ data: Data) throws -> T {
        do {
            return try JSONDecoder().decode(T.self, from: data)
        } catch {
            throw LocalProductClientError.invalidResponse
        }
    }
}

public protocol LocalProductPermissionClientProtocol {
    func permissionsSnapshot(
        journeyID: String
    ) async throws -> PermissionSnapshot
    func permissionsAttention(
        journeyID: String
    ) async throws -> PermissionAttention
}

extension LocalIPCClient: LocalProductPermissionClientProtocol {}
