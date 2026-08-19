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

private func validRemoteToolEnrollmentPair(
    enrollmentID: String,
    enrollmentDigest: String
) -> Bool {
    if enrollmentID.isEmpty && enrollmentDigest.isEmpty { return true }
    guard (1...128).contains(enrollmentID.utf8.count),
          LocalIPCClient.validIdentifier(enrollmentID),
          enrollmentDigest.utf8.count == 64,
          enrollmentDigest.allSatisfy(\.isHexDigit) else {
        return false
    }
    return true
}

private func validBuilderExecutionProfile(
    profileID: String,
    harnessAdapter: String,
    providerID: String,
    providerAccountID: String,
    modelID: String,
    authMode: String,
    credentialRevision: Int64,
    reasoningEffort: String,
    timeoutMilliseconds: Int64,
    budgetAvailable: Bool,
    budgetUnits: Int64,
    capabilities: [String]
) -> Bool {
    let validModelID = (1...256).contains(modelID.utf8.count) &&
        modelID.unicodeScalars.allSatisfy { $0.value >= 0x21 && $0.value <= 0x7e }
    let validCapabilities = capabilities == Array(Set(capabilities)).sorted() &&
        capabilities.allSatisfy(LocalIPCClient.validIdentifier)
    guard LocalIPCClient.validIdentifier(profileID),
          LocalIPCClient.validIdentifier(harnessAdapter),
          LocalIPCClient.validIdentifier(providerID), validModelID,
          timeoutMilliseconds > 0,
          (budgetAvailable ? budgetUnits >= 0 : budgetUnits == 0),
          validCapabilities else {
        return false
    }
    switch authMode {
    case "native_auth":
        guard providerAccountID.isEmpty, credentialRevision == 0 else { return false }
    case "brokered", "provider_ephemeral":
        guard LocalIPCClient.validProviderAccountID(
            providerAccountID,
            providerID: providerID
        ), credentialRevision > 0 else { return false }
    default:
        return false
    }
    return reasoningEffort.isEmpty ||
        ((1...32).contains(reasoningEffort.utf8.count) &&
            LocalIPCClient.validIdentifier(reasoningEffort) &&
            capabilities.contains("reasoning_effort"))
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

public struct LocalProductProviderDirectoryEntry:
    Codable, Equatable, Sendable, Identifiable
{
    public var id: String { providerID }
    public let providerID: String
    public let displayName: String
    public let category: String
    public let protocolFamily: String
    public let authMode: String
    public let connectionKind: String
    public let credentialReference: String
    public let revision: Int64
    public let status: String
    public let reason: String
    public let supportsModelDiscovery: Bool

    enum CodingKeys: String, CodingKey {
        case providerID = "provider_id"
        case displayName = "display_name"
        case category, revision, status, reason
        case authMode = "auth_mode"
        case protocolFamily = "protocol"
        case connectionKind = "connection_kind"
        case credentialReference = "credential_reference"
        case supportsModelDiscovery = "supports_model_discovery"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownSetupKeys(decoder, allowed: [
            "provider_id", "display_name", "category", "protocol",
            "auth_mode", "connection_kind", "credential_reference",
            "revision", "status", "reason", "supports_model_discovery",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        providerID = try values.decode(String.self, forKey: .providerID)
        displayName = try values.decode(String.self, forKey: .displayName)
        category = try values.decode(String.self, forKey: .category)
        protocolFamily = try values.decode(String.self, forKey: .protocolFamily)
        authMode = try values.decode(String.self, forKey: .authMode)
        connectionKind = try values.decode(String.self, forKey: .connectionKind)
        credentialReference = try values.decode(
            String.self,
            forKey: .credentialReference
        )
        revision = try values.decode(Int64.self, forKey: .revision)
        status = try values.decode(String.self, forKey: .status)
        reason = try values.decode(String.self, forKey: .reason)
        supportsModelDiscovery = try values.decode(
            Bool.self,
            forKey: .supportsModelDiscovery
        )
    }
}

public struct LocalProductProviderAccountDirectoryEntry:
    Codable, Equatable, Sendable, Identifiable
{
    public var id: String { providerAccountID }
    public let providerID: String
    public let providerAccountID: String
    public let authMode: String
    public let credentialReference: String
    public let revision: Int64
    public let status: String
    public let reason: String
    public let policyAvailable: Bool
    public let policyVersion: Int
    public let policyRevision: Int64
    public let policyDigest: String
    public let maximumConcurrentAttempts: Int
    public let dispatchWindowSeconds: Int64
    public let maximumDispatchStarts: Int
    public let maximumAssignedBudgetUnits: Int64
    public let trustDomain: String
    public let retentionMode: String
    public let dataRegion: String
    public let rateCards: [LocalProductProviderModelRateCard]
    public let remoteToolBackends: [LocalProductRemoteToolBackendEnrollment]

    enum CodingKeys: String, CodingKey {
        case providerID = "provider_id"
        case providerAccountID = "provider_account_id"
        case authMode = "auth_mode"
        case credentialReference = "credential_reference"
        case revision, status, reason
        case policyAvailable = "policy_available"
        case policyVersion = "policy_version"
        case policyRevision = "policy_revision"
        case policyDigest = "policy_digest"
        case maximumConcurrentAttempts = "maximum_concurrent_attempts"
        case dispatchWindowSeconds = "dispatch_window_seconds"
        case maximumDispatchStarts = "maximum_dispatch_starts"
        case maximumAssignedBudgetUnits = "maximum_assigned_budget_units"
        case trustDomain = "trust_domain"
        case retentionMode = "retention_mode"
        case dataRegion = "data_region"
        case rateCards = "rate_cards"
        case remoteToolBackends = "remote_tool_backends"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownSetupKeys(decoder, allowed: [
            "provider_id", "provider_account_id", "auth_mode",
            "credential_reference", "revision", "status", "reason",
            "policy_available", "policy_revision",
            "policy_version", "policy_digest",
            "maximum_concurrent_attempts", "dispatch_window_seconds",
            "maximum_dispatch_starts", "maximum_assigned_budget_units",
            "trust_domain", "retention_mode", "data_region",
            "rate_cards", "remote_tool_backends",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        providerID = try values.decode(String.self, forKey: .providerID)
        providerAccountID = try values.decode(
            String.self,
            forKey: .providerAccountID
        )
        authMode = try values.decode(String.self, forKey: .authMode)
        credentialReference = try values.decode(
            String.self,
            forKey: .credentialReference
        )
        revision = try values.decode(Int64.self, forKey: .revision)
        status = try values.decode(String.self, forKey: .status)
        reason = try values.decode(String.self, forKey: .reason)
        policyAvailable = try values.decodeIfPresent(
            Bool.self, forKey: .policyAvailable
        ) ?? false
        policyVersion = try values.decodeIfPresent(
            Int.self, forKey: .policyVersion
        ) ?? (policyAvailable ? 1 : 0)
        policyRevision = try values.decodeIfPresent(
            Int64.self, forKey: .policyRevision
        ) ?? 0
        policyDigest = try values.decodeIfPresent(
            String.self, forKey: .policyDigest
        ) ?? ""
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
        trustDomain = try values.decodeIfPresent(
            String.self, forKey: .trustDomain
        ) ?? ""
        retentionMode = try values.decodeIfPresent(
            String.self, forKey: .retentionMode
        ) ?? ""
        dataRegion = try values.decodeIfPresent(
            String.self, forKey: .dataRegion
        ) ?? ""
        rateCards = try values.decodeIfPresent(
            [LocalProductProviderModelRateCard].self, forKey: .rateCards
        ) ?? []
        remoteToolBackends = try values.decodeIfPresent(
            [LocalProductRemoteToolBackendEnrollment].self,
            forKey: .remoteToolBackends
        ) ?? []
        let validPolicy = policyAvailable
            ? [1, 2].contains(policyVersion) && policyRevision > 0
                && (policyVersion == 1
                    ? policyDigest.isEmpty || Self.validPolicyDigest(policyDigest)
                    : Self.validPolicyDigest(policyDigest))
                && (1...64).contains(maximumConcurrentAttempts)
                && (1...86_400).contains(dispatchWindowSeconds)
                && (1...1_000_000).contains(maximumDispatchStarts)
                && maximumAssignedBudgetUnits >= 0
                && Self.validDisclosurePolicy(
                    version: policyVersion,
                    trustDomain: trustDomain,
                    retentionMode: retentionMode,
                    dataRegion: dataRegion
                )
            : policyVersion == 0 && policyRevision == 0 && policyDigest.isEmpty
                && maximumConcurrentAttempts == 0
                && dispatchWindowSeconds == 0 && maximumDispatchStarts == 0
                && maximumAssignedBudgetUnits == 0 && trustDomain.isEmpty
                && retentionMode.isEmpty && dataRegion.isEmpty
        let validRemoteBackends = Set(
            remoteToolBackends.map(\.enrollmentID)
        ).count == remoteToolBackends.count
            && (policyAvailable || remoteToolBackends.isEmpty)
            && remoteToolBackends.allSatisfy { enrollment in
                !enrollment.policyCurrent || (
                    enrollment.providerAccountPolicyVersion == policyVersion
                        && enrollment.providerAccountPolicyRevision == policyRevision
                        && enrollment.providerAccountPolicyDigest == policyDigest
                )
            }
        guard validPolicy,
              Set(rateCards.map(\.modelID)).count == rateCards.count,
              validRemoteBackends else {
            throw LocalProductWireError.invalidJSON
        }
    }

    public var disclosurePolicyConfigured: Bool {
        policyAvailable && policyVersion == 2
    }

    fileprivate static func validPolicyDigest(_ value: String) -> Bool {
        value.utf8.count == 64 && value.utf8.allSatisfy {
            ($0 >= 0x30 && $0 <= 0x39) || ($0 >= 0x61 && $0 <= 0x66)
        }
    }

    fileprivate static func validDisclosurePolicy(
        version: Int,
        trustDomain: String,
        retentionMode: String,
        dataRegion: String
    ) -> Bool {
        if version == 1 {
            return trustDomain.isEmpty && retentionMode.isEmpty && dataRegion.isEmpty
        }
        return version == 2
            && ["external_provider", "enterprise_tenant", "local_runtime"].contains(trustDomain)
            && ["provider_default", "zero_data_retention", "limited_retention"].contains(retentionMode)
            && ["global", "us", "eu", "apac", "local"].contains(dataRegion)
    }
}

public struct LocalProductRemoteToolBackendEnrollment:
    Codable, Equatable, Sendable, Identifiable
{
    public var id: String { enrollmentID }
    public let enrollmentVersion: Int
    public let enrollmentID: String
    public let backendKind: String
    public let adapterID: String
    public let providerAccountPolicyVersion: Int
    public let providerAccountPolicyRevision: Int64
    public let providerAccountPolicyDigest: String
    public let policyCurrent: Bool
    public let endpointFingerprint: String
    public let mcpServerID: String
    public let allowedTools: [String]
    public let revision: Int64
    public let status: String
    public let maximumConcurrentCalls: Int
    public let maximumCallsPerAttempt: Int
    public let timeoutSeconds: Int64
    public let maximumResultBytes: Int
    public let maximumBudgetUnits: Int64
    public let configuredAt: String
    public let enrollmentDigest: String

    enum CodingKeys: String, CodingKey {
        case enrollmentVersion = "enrollment_version"
        case enrollmentID = "enrollment_id"
        case backendKind = "backend_kind"
        case adapterID = "adapter_id"
        case providerAccountPolicyVersion = "provider_account_policy_version"
        case providerAccountPolicyRevision = "provider_account_policy_revision"
        case providerAccountPolicyDigest = "provider_account_policy_digest"
        case policyCurrent = "policy_current"
        case endpointFingerprint = "endpoint_fingerprint"
        case mcpServerID = "mcp_server_id"
        case allowedTools = "allowed_tools"
        case revision, status
        case maximumConcurrentCalls = "maximum_concurrent_calls"
        case maximumCallsPerAttempt = "maximum_calls_per_attempt"
        case timeoutSeconds = "timeout_seconds"
        case maximumResultBytes = "maximum_result_bytes"
        case maximumBudgetUnits = "maximum_budget_units"
        case configuredAt = "configured_at"
        case enrollmentDigest = "enrollment_digest"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownSetupKeys(decoder, allowed: [
            "enrollment_version", "enrollment_id", "backend_kind", "adapter_id",
            "provider_account_policy_version", "provider_account_policy_revision",
            "provider_account_policy_digest", "policy_current",
            "endpoint_fingerprint", "mcp_server_id", "allowed_tools",
            "revision", "status", "maximum_concurrent_calls",
            "maximum_calls_per_attempt", "timeout_seconds",
            "maximum_result_bytes", "maximum_budget_units", "configured_at",
            "enrollment_digest",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        enrollmentVersion = try values.decode(Int.self, forKey: .enrollmentVersion)
        enrollmentID = try values.decode(String.self, forKey: .enrollmentID)
        backendKind = try values.decode(String.self, forKey: .backendKind)
        adapterID = try values.decode(String.self, forKey: .adapterID)
        providerAccountPolicyVersion = try values.decode(
            Int.self, forKey: .providerAccountPolicyVersion
        )
        providerAccountPolicyRevision = try values.decode(
            Int64.self, forKey: .providerAccountPolicyRevision
        )
        providerAccountPolicyDigest = try values.decode(
            String.self, forKey: .providerAccountPolicyDigest
        )
        policyCurrent = try values.decode(Bool.self, forKey: .policyCurrent)
        endpointFingerprint = try values.decode(
            String.self, forKey: .endpointFingerprint
        )
        mcpServerID = try values.decode(String.self, forKey: .mcpServerID)
        allowedTools = try values.decode([String].self, forKey: .allowedTools)
        revision = try values.decode(Int64.self, forKey: .revision)
        status = try values.decode(String.self, forKey: .status)
        maximumConcurrentCalls = try values.decode(
            Int.self, forKey: .maximumConcurrentCalls
        )
        maximumCallsPerAttempt = try values.decode(
            Int.self, forKey: .maximumCallsPerAttempt
        )
        timeoutSeconds = try values.decode(Int64.self, forKey: .timeoutSeconds)
        maximumResultBytes = try values.decode(Int.self, forKey: .maximumResultBytes)
        maximumBudgetUnits = try values.decode(Int64.self, forKey: .maximumBudgetUnits)
        configuredAt = try values.decode(String.self, forKey: .configuredAt)
        enrollmentDigest = try values.decode(String.self, forKey: .enrollmentDigest)

        let validTools = allowedTools.count <= 128
            && allowedTools == allowedTools.sorted()
            && Set(allowedTools).count == allowedTools.count
            && allowedTools.allSatisfy(Self.validBackendIdentifier)
        let validBackendShape = backendKind == "web_search"
            ? mcpServerID.isEmpty && allowedTools.isEmpty
            : backendKind == "mcp_server"
                && Self.validBackendIdentifier(mcpServerID) && !allowedTools.isEmpty
        guard enrollmentVersion == 1,
              Self.validBackendIdentifier(enrollmentID),
              Self.validBackendIdentifier(adapterID),
              [1, 2].contains(providerAccountPolicyVersion),
              providerAccountPolicyRevision > 0,
              Self.validDigest(providerAccountPolicyDigest),
              Self.validDigest(endpointFingerprint),
              validTools, validBackendShape,
              revision > 0, ["active", "revoked"].contains(status),
              (1...16).contains(maximumConcurrentCalls),
              (1...16).contains(maximumCallsPerAttempt),
              (1...120).contains(timeoutSeconds),
              (256...(1 << 20)).contains(maximumResultBytes),
              maximumBudgetUnits >= 0,
              LocalProductProviderAccountPolicyResult.validTimestamp(configuredAt),
              Self.validDigest(enrollmentDigest) else {
            throw LocalProductWireError.invalidJSON
        }
    }

    fileprivate static func validDigest(_ value: String) -> Bool {
        value.utf8.count == 64 && value.utf8.allSatisfy {
            ($0 >= 0x30 && $0 <= 0x39) || ($0 >= 0x61 && $0 <= 0x66)
        }
    }

    fileprivate static func validBackendIdentifier(_ value: String) -> Bool {
        guard !value.isEmpty, value.utf8.count <= 128,
              let first = value.utf8.first, let last = value.utf8.last,
              ![0x2d, 0x2e, 0x5f].contains(first),
              ![0x2d, 0x2e, 0x5f].contains(last) else {
            return false
        }
        var previousSeparator = false
        for byte in value.utf8 {
            if (byte >= 0x61 && byte <= 0x7a) || (byte >= 0x30 && byte <= 0x39) {
                previousSeparator = false
            } else if [0x2d, 0x2e, 0x5f].contains(byte), !previousSeparator {
                previousSeparator = true
            } else {
                return false
            }
        }
        return true
    }
}

public struct LocalProductRemoteToolBackendEnrollmentCommand: Equatable, Sendable {
    public let enrollmentID: String
    public let backendKind: String
    public let adapterID: String
    public let providerID: String
    public let providerAccountID: String
    public let providerAccountPolicyVersion: Int
    public let providerAccountPolicyRevision: Int64
    public let providerAccountPolicyDigest: String
    public let endpointFingerprint: String
    public let mcpServerID: String
    public let allowedTools: [String]
    public let expectedRevision: Int64
    public let maximumConcurrentCalls: Int
    public let maximumCallsPerAttempt: Int
    public let timeoutSeconds: Int64
    public let maximumResultBytes: Int
    public let maximumBudgetUnits: Int64

    public init(
        enrollmentID: String,
        backendKind: String,
        adapterID: String,
        providerID: String,
        providerAccountID: String,
        providerAccountPolicyVersion: Int,
        providerAccountPolicyRevision: Int64,
        providerAccountPolicyDigest: String,
        endpointFingerprint: String,
        mcpServerID: String,
        allowedTools: [String],
        expectedRevision: Int64,
        maximumConcurrentCalls: Int,
        maximumCallsPerAttempt: Int,
        timeoutSeconds: Int64,
        maximumResultBytes: Int,
        maximumBudgetUnits: Int64
    ) {
        self.enrollmentID = enrollmentID
        self.backendKind = backendKind
        self.adapterID = adapterID
        self.providerID = providerID
        self.providerAccountID = providerAccountID
        self.providerAccountPolicyVersion = providerAccountPolicyVersion
        self.providerAccountPolicyRevision = providerAccountPolicyRevision
        self.providerAccountPolicyDigest = providerAccountPolicyDigest
        self.endpointFingerprint = endpointFingerprint
        self.mcpServerID = mcpServerID
        self.allowedTools = allowedTools
        self.expectedRevision = expectedRevision
        self.maximumConcurrentCalls = maximumConcurrentCalls
        self.maximumCallsPerAttempt = maximumCallsPerAttempt
        self.timeoutSeconds = timeoutSeconds
        self.maximumResultBytes = maximumResultBytes
        self.maximumBudgetUnits = maximumBudgetUnits
    }

    public var valid: Bool {
        let validTools = allowedTools.count <= 128
            && allowedTools == allowedTools.sorted()
            && Set(allowedTools).count == allowedTools.count
            && allowedTools.allSatisfy(
                LocalProductRemoteToolBackendEnrollment.validBackendIdentifier
            )
        let validShape = backendKind == "web_search"
            ? mcpServerID.isEmpty && allowedTools.isEmpty
            : backendKind == "mcp_server"
                && LocalProductRemoteToolBackendEnrollment.validBackendIdentifier(mcpServerID)
                && !allowedTools.isEmpty
        return LocalProductRemoteToolBackendEnrollment.validBackendIdentifier(enrollmentID)
            && LocalProductRemoteToolBackendEnrollment.validBackendIdentifier(adapterID)
            && LocalIPCClient.validIdentifier(providerID)
            && LocalIPCClient.validProviderAccountID(
                providerAccountID,
                providerID: providerID
            )
            && [1, 2].contains(providerAccountPolicyVersion)
            && providerAccountPolicyRevision > 0
            && LocalProductRemoteToolBackendEnrollment.validDigest(
                providerAccountPolicyDigest
            )
            && LocalProductRemoteToolBackendEnrollment.validDigest(endpointFingerprint)
            && validTools && validShape
            && expectedRevision >= 0 && expectedRevision < Int64(UInt32.max)
            && (1...16).contains(maximumConcurrentCalls)
            && (1...16).contains(maximumCallsPerAttempt)
            && (1...120).contains(timeoutSeconds)
            && (256...(1 << 20)).contains(maximumResultBytes)
            && maximumBudgetUnits >= 0
    }
}

public struct LocalProductRemoteToolBackendEnrollmentRevokeCommand:
    Equatable, Sendable
{
    public let enrollmentID: String
    public let providerID: String
    public let providerAccountID: String
    public let expectedRevision: Int64

    public init(
        enrollmentID: String,
        providerID: String,
        providerAccountID: String,
        expectedRevision: Int64
    ) {
        self.enrollmentID = enrollmentID
        self.providerID = providerID
        self.providerAccountID = providerAccountID
        self.expectedRevision = expectedRevision
    }

    public var valid: Bool {
        LocalProductRemoteToolBackendEnrollment.validBackendIdentifier(enrollmentID)
            && LocalIPCClient.validIdentifier(providerID)
            && LocalIPCClient.validProviderAccountID(
                providerAccountID,
                providerID: providerID
            )
            && expectedRevision > 0 && expectedRevision < Int64(UInt32.max)
    }
}

public struct LocalProductRemoteToolBackendEnrollmentResult:
    Decodable, Equatable, Sendable
{
    public let enrollmentAvailable: Bool
    public let enrollmentVersion: Int
    public let enrollmentID: String
    public let backendKind: String
    public let adapterID: String
    public let providerID: String
    public let providerAccountID: String
    public let providerAccountPolicyVersion: Int
    public let providerAccountPolicyRevision: Int64
    public let providerAccountPolicyDigest: String
    public let policyCurrent: Bool
    public let endpointFingerprint: String
    public let mcpServerID: String
    public let allowedTools: [String]
    public let revision: Int64
    public let status: String
    public let maximumConcurrentCalls: Int
    public let maximumCallsPerAttempt: Int
    public let timeoutSeconds: Int64
    public let maximumResultBytes: Int
    public let maximumBudgetUnits: Int64
    public let configuredAt: String
    public let enrollmentDigest: String

    enum CodingKeys: String, CodingKey {
        case enrollmentAvailable = "enrollment_available"
        case enrollmentVersion = "enrollment_version"
        case enrollmentID = "enrollment_id"
        case backendKind = "backend_kind"
        case adapterID = "adapter_id"
        case providerID = "provider_id"
        case providerAccountID = "provider_account_id"
        case providerAccountPolicyVersion = "provider_account_policy_version"
        case providerAccountPolicyRevision = "provider_account_policy_revision"
        case providerAccountPolicyDigest = "provider_account_policy_digest"
        case policyCurrent = "policy_current"
        case endpointFingerprint = "endpoint_fingerprint"
        case mcpServerID = "mcp_server_id"
        case allowedTools = "allowed_tools"
        case revision, status
        case maximumConcurrentCalls = "maximum_concurrent_calls"
        case maximumCallsPerAttempt = "maximum_calls_per_attempt"
        case timeoutSeconds = "timeout_seconds"
        case maximumResultBytes = "maximum_result_bytes"
        case maximumBudgetUnits = "maximum_budget_units"
        case configuredAt = "configured_at"
        case enrollmentDigest = "enrollment_digest"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownSetupKeys(decoder, allowed: [
            "enrollment_available", "enrollment_version", "enrollment_id",
            "backend_kind", "adapter_id", "provider_id", "provider_account_id",
            "provider_account_policy_version", "provider_account_policy_revision",
            "provider_account_policy_digest", "policy_current",
            "endpoint_fingerprint", "mcp_server_id", "allowed_tools",
            "revision", "status", "maximum_concurrent_calls",
            "maximum_calls_per_attempt", "timeout_seconds",
            "maximum_result_bytes", "maximum_budget_units", "configured_at",
            "enrollment_digest",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        enrollmentAvailable = try values.decode(Bool.self, forKey: .enrollmentAvailable)
        enrollmentVersion = try values.decode(Int.self, forKey: .enrollmentVersion)
        enrollmentID = try values.decode(String.self, forKey: .enrollmentID)
        backendKind = try values.decode(String.self, forKey: .backendKind)
        adapterID = try values.decode(String.self, forKey: .adapterID)
        providerID = try values.decode(String.self, forKey: .providerID)
        providerAccountID = try values.decode(String.self, forKey: .providerAccountID)
        providerAccountPolicyVersion = try values.decode(
            Int.self, forKey: .providerAccountPolicyVersion
        )
        providerAccountPolicyRevision = try values.decode(
            Int64.self, forKey: .providerAccountPolicyRevision
        )
        providerAccountPolicyDigest = try values.decode(
            String.self, forKey: .providerAccountPolicyDigest
        )
        policyCurrent = try values.decode(Bool.self, forKey: .policyCurrent)
        endpointFingerprint = try values.decode(String.self, forKey: .endpointFingerprint)
        mcpServerID = try values.decode(String.self, forKey: .mcpServerID)
        allowedTools = try values.decode([String].self, forKey: .allowedTools)
        revision = try values.decode(Int64.self, forKey: .revision)
        status = try values.decode(String.self, forKey: .status)
        maximumConcurrentCalls = try values.decode(
            Int.self, forKey: .maximumConcurrentCalls
        )
        maximumCallsPerAttempt = try values.decode(
            Int.self, forKey: .maximumCallsPerAttempt
        )
        timeoutSeconds = try values.decode(Int64.self, forKey: .timeoutSeconds)
        maximumResultBytes = try values.decode(Int.self, forKey: .maximumResultBytes)
        maximumBudgetUnits = try values.decode(Int64.self, forKey: .maximumBudgetUnits)
        configuredAt = try values.decode(String.self, forKey: .configuredAt)
        enrollmentDigest = try values.decode(String.self, forKey: .enrollmentDigest)

        guard enrollmentAvailable, enrollmentVersion == 1, revision > 0,
              ["active", "revoked"].contains(status),
              LocalProductProviderAccountPolicyResult.validTimestamp(configuredAt),
              LocalProductRemoteToolBackendEnrollment.validDigest(enrollmentDigest) else {
            throw LocalProductWireError.invalidJSON
        }
        let command = LocalProductRemoteToolBackendEnrollmentCommand(
            enrollmentID: enrollmentID, backendKind: backendKind,
            adapterID: adapterID, providerID: providerID,
            providerAccountID: providerAccountID,
            providerAccountPolicyVersion: providerAccountPolicyVersion,
            providerAccountPolicyRevision: providerAccountPolicyRevision,
            providerAccountPolicyDigest: providerAccountPolicyDigest,
            endpointFingerprint: endpointFingerprint, mcpServerID: mcpServerID,
            allowedTools: allowedTools, expectedRevision: revision - 1,
            maximumConcurrentCalls: maximumConcurrentCalls,
            maximumCallsPerAttempt: maximumCallsPerAttempt,
            timeoutSeconds: timeoutSeconds,
            maximumResultBytes: maximumResultBytes,
            maximumBudgetUnits: maximumBudgetUnits
        )
        guard command.valid else {
            throw LocalProductWireError.invalidJSON
        }
    }
}

public struct LocalProductProviderModelRateCard:
    Codable, Equatable, Sendable, Identifiable
{
    public var id: String { modelID }
    public let modelID: String
    public let revision: Int64
    public let rateCardDigest: String
    public let currency: String
    public let inputTokenBasis: String
    public let inputMicrounitsPerMillion: Int64
    public let outputMicrounitsPerMillion: Int64
    public let cacheReadMicrounitsPerMillion: Int64
    public let cacheWriteMicrounitsPerMillion: Int64
    public let roundingMode: String
    public let configuredAt: String

    enum CodingKeys: String, CodingKey {
        case modelID = "model_id"
        case revision
        case rateCardDigest = "rate_card_digest"
        case currency
        case inputTokenBasis = "input_token_basis"
        case inputMicrounitsPerMillion = "input_microunits_per_million"
        case outputMicrounitsPerMillion = "output_microunits_per_million"
        case cacheReadMicrounitsPerMillion = "cache_read_microunits_per_million"
        case cacheWriteMicrounitsPerMillion = "cache_write_microunits_per_million"
        case roundingMode = "rounding_mode"
        case configuredAt = "configured_at"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownSetupKeys(decoder, allowed: [
            "model_id", "revision", "rate_card_digest", "currency",
            "input_token_basis", "input_microunits_per_million",
            "output_microunits_per_million",
            "cache_read_microunits_per_million",
            "cache_write_microunits_per_million", "rounding_mode",
            "configured_at",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        modelID = try values.decode(String.self, forKey: .modelID)
        revision = try values.decode(Int64.self, forKey: .revision)
        rateCardDigest = try values.decode(String.self, forKey: .rateCardDigest)
        currency = try values.decode(String.self, forKey: .currency)
        inputTokenBasis = try values.decode(String.self, forKey: .inputTokenBasis)
        inputMicrounitsPerMillion = try values.decode(
            Int64.self, forKey: .inputMicrounitsPerMillion
        )
        outputMicrounitsPerMillion = try values.decode(
            Int64.self, forKey: .outputMicrounitsPerMillion
        )
        cacheReadMicrounitsPerMillion = try values.decode(
            Int64.self, forKey: .cacheReadMicrounitsPerMillion
        )
        cacheWriteMicrounitsPerMillion = try values.decode(
            Int64.self, forKey: .cacheWriteMicrounitsPerMillion
        )
        roundingMode = try values.decode(String.self, forKey: .roundingMode)
        configuredAt = try values.decode(String.self, forKey: .configuredAt)
        guard Self.valid(
            modelID: modelID,
            revision: revision,
            digest: rateCardDigest,
            currency: currency,
            inputTokenBasis: inputTokenBasis,
            rates: [
                inputMicrounitsPerMillion, outputMicrounitsPerMillion,
                cacheReadMicrounitsPerMillion, cacheWriteMicrounitsPerMillion,
            ],
            roundingMode: roundingMode,
            configuredAt: configuredAt
        ) else {
            throw LocalProductWireError.invalidJSON
        }
    }

    fileprivate static func valid(
        modelID: String,
        revision: Int64,
        digest: String,
        currency: String,
        inputTokenBasis: String,
        rates: [Int64],
        roundingMode: String,
        configuredAt: String
    ) -> Bool {
        let validModel = !modelID.isEmpty && modelID.utf8.count <= 256
            && modelID == modelID.trimmingCharacters(in: .whitespacesAndNewlines)
            && modelID.unicodeScalars.allSatisfy { !CharacterSet.controlCharacters.contains($0) }
        let validDigest = digest.utf8.count == 64 && digest.utf8.allSatisfy {
            ($0 >= 0x30 && $0 <= 0x39) || ($0 >= 0x61 && $0 <= 0x66)
        }
        let validCurrency = currency.utf8.count == 3 && currency.utf8.allSatisfy {
            $0 >= 0x41 && $0 <= 0x5a
        }
        let validTimestamp = LocalProductProviderAccountPolicyResult.validTimestamp(
            configuredAt
        )
        return validModel && revision > 0 && validDigest && validCurrency
            && ["input_includes_cache", "input_excludes_cache"].contains(inputTokenBasis)
            && rates.allSatisfy { (0...1_000_000_000_000).contains($0) }
            && roundingMode == "ceiling_per_attempt" && validTimestamp
    }
}

public struct LocalProductProviderModelRateCardResult:
    Codable, Equatable, Sendable
{
    public let providerID: String
    public let providerAccountID: String
    public let modelID: String
    public let revision: Int64
    public let rateCardDigest: String
    public let currency: String
    public let inputTokenBasis: String
    public let inputMicrounitsPerMillion: Int64
    public let outputMicrounitsPerMillion: Int64
    public let cacheReadMicrounitsPerMillion: Int64
    public let cacheWriteMicrounitsPerMillion: Int64
    public let roundingMode: String
    public let configuredAt: String

    enum CodingKeys: String, CodingKey {
        case providerID = "provider_id"
        case providerAccountID = "provider_account_id"
        case modelID = "model_id"
        case revision
        case rateCardDigest = "rate_card_digest"
        case currency
        case inputTokenBasis = "input_token_basis"
        case inputMicrounitsPerMillion = "input_microunits_per_million"
        case outputMicrounitsPerMillion = "output_microunits_per_million"
        case cacheReadMicrounitsPerMillion = "cache_read_microunits_per_million"
        case cacheWriteMicrounitsPerMillion = "cache_write_microunits_per_million"
        case roundingMode = "rounding_mode"
        case configuredAt = "configured_at"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownSetupKeys(decoder, allowed: [
            "provider_id", "provider_account_id", "model_id", "revision",
            "rate_card_digest", "currency", "input_token_basis",
            "input_microunits_per_million", "output_microunits_per_million",
            "cache_read_microunits_per_million",
            "cache_write_microunits_per_million", "rounding_mode",
            "configured_at",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        providerID = try values.decode(String.self, forKey: .providerID)
        providerAccountID = try values.decode(String.self, forKey: .providerAccountID)
        modelID = try values.decode(String.self, forKey: .modelID)
        revision = try values.decode(Int64.self, forKey: .revision)
        rateCardDigest = try values.decode(String.self, forKey: .rateCardDigest)
        currency = try values.decode(String.self, forKey: .currency)
        inputTokenBasis = try values.decode(String.self, forKey: .inputTokenBasis)
        inputMicrounitsPerMillion = try values.decode(Int64.self, forKey: .inputMicrounitsPerMillion)
        outputMicrounitsPerMillion = try values.decode(Int64.self, forKey: .outputMicrounitsPerMillion)
        cacheReadMicrounitsPerMillion = try values.decode(Int64.self, forKey: .cacheReadMicrounitsPerMillion)
        cacheWriteMicrounitsPerMillion = try values.decode(Int64.self, forKey: .cacheWriteMicrounitsPerMillion)
        roundingMode = try values.decode(String.self, forKey: .roundingMode)
        configuredAt = try values.decode(String.self, forKey: .configuredAt)
        guard LocalIPCClient.validIdentifier(providerID),
              LocalIPCClient.validProviderAccountID(
                providerAccountID, providerID: providerID
              ),
              LocalProductProviderModelRateCard.valid(
                modelID: modelID, revision: revision, digest: rateCardDigest,
                currency: currency, inputTokenBasis: inputTokenBasis,
                rates: [inputMicrounitsPerMillion, outputMicrounitsPerMillion,
                        cacheReadMicrounitsPerMillion, cacheWriteMicrounitsPerMillion],
                roundingMode: roundingMode, configuredAt: configuredAt
              ) else {
            throw LocalProductWireError.invalidJSON
        }
    }
}

public struct LocalProductProviderAccountPolicyResult:
    Codable, Equatable, Sendable
{
    public let policyAvailable: Bool
    public let policyVersion: Int
    public let providerID: String
    public let providerAccountID: String
    public let revision: Int64
    public let policyDigest: String
    public let maximumConcurrentAttempts: Int
    public let dispatchWindowSeconds: Int64
    public let maximumDispatchStarts: Int
    public let maximumAssignedBudgetUnits: Int64
    public let trustDomain: String
    public let retentionMode: String
    public let dataRegion: String
    public let configuredAt: String

    enum CodingKeys: String, CodingKey {
        case policyAvailable = "policy_available"
        case policyVersion = "policy_version"
        case providerID = "provider_id"
        case providerAccountID = "provider_account_id"
        case revision
        case policyDigest = "policy_digest"
        case maximumConcurrentAttempts = "maximum_concurrent_attempts"
        case dispatchWindowSeconds = "dispatch_window_seconds"
        case maximumDispatchStarts = "maximum_dispatch_starts"
        case maximumAssignedBudgetUnits = "maximum_assigned_budget_units"
        case trustDomain = "trust_domain"
        case retentionMode = "retention_mode"
        case dataRegion = "data_region"
        case configuredAt = "configured_at"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownSetupKeys(decoder, allowed: [
            "policy_available", "provider_id", "provider_account_id",
            "policy_version", "revision", "policy_digest", "maximum_concurrent_attempts",
            "dispatch_window_seconds", "maximum_dispatch_starts",
            "maximum_assigned_budget_units", "trust_domain", "retention_mode",
            "data_region", "configured_at",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        policyAvailable = try values.decode(Bool.self, forKey: .policyAvailable)
        policyVersion = try values.decode(Int.self, forKey: .policyVersion)
        providerID = try values.decode(String.self, forKey: .providerID)
        providerAccountID = try values.decode(String.self, forKey: .providerAccountID)
        revision = try values.decode(Int64.self, forKey: .revision)
        policyDigest = try values.decode(String.self, forKey: .policyDigest)
        maximumConcurrentAttempts = try values.decode(
            Int.self, forKey: .maximumConcurrentAttempts
        )
        dispatchWindowSeconds = try values.decode(
            Int64.self, forKey: .dispatchWindowSeconds
        )
        maximumDispatchStarts = try values.decode(
            Int.self, forKey: .maximumDispatchStarts
        )
        maximumAssignedBudgetUnits = try values.decode(
            Int64.self, forKey: .maximumAssignedBudgetUnits
        )
        trustDomain = try values.decode(String.self, forKey: .trustDomain)
        retentionMode = try values.decode(String.self, forKey: .retentionMode)
        dataRegion = try values.decode(String.self, forKey: .dataRegion)
        configuredAt = try values.decode(String.self, forKey: .configuredAt)

        let validTimestamp = Self.validTimestamp(configuredAt)
        let validDigest = policyDigest.utf8.count == 64 && policyDigest.utf8.allSatisfy {
            ($0 >= 0x30 && $0 <= 0x39) || ($0 >= 0x61 && $0 <= 0x66)
        }
        guard policyAvailable, policyVersion == 2,
              LocalIPCClient.validIdentifier(providerID),
              LocalIPCClient.validProviderAccountID(
                providerAccountID,
                providerID: providerID
              ),
              revision > 0, validDigest,
              (1...64).contains(maximumConcurrentAttempts),
              (1...86_400).contains(dispatchWindowSeconds),
              (1...1_000_000).contains(maximumDispatchStarts),
              maximumAssignedBudgetUnits >= 0,
              LocalProductProviderAccountDirectoryEntry.validDisclosurePolicy(
                version: policyVersion, trustDomain: trustDomain,
                retentionMode: retentionMode, dataRegion: dataRegion
              ),
              validTimestamp else {
            throw LocalProductWireError.invalidJSON
        }
    }

    fileprivate static func validTimestamp(_ value: String) -> Bool {
        let internetDate = ISO8601DateFormatter()
        let fractionalDate = ISO8601DateFormatter()
        fractionalDate.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return value.hasSuffix("Z") && (
            internetDate.date(from: value) != nil
                || fractionalDate.date(from: value) != nil
        )
    }
}

public struct LocalProductCredentialVaultStatus:
    Codable, Equatable, Sendable
{
    public let schemaVersion: Int
    public let status: String
    public let storageMode: String
    public let migrationRequiredAccounts: Int
    public let recoveryRequiredAccounts: Int

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case status
        case storageMode = "storage_mode"
        case migrationRequiredAccounts = "migration_required_accounts"
        case recoveryRequiredAccounts = "recovery_required_accounts"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownSetupKeys(decoder, allowed: [
            "schema_version", "status", "storage_mode",
            "migration_required_accounts", "recovery_required_accounts",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
        status = try values.decode(String.self, forKey: .status)
        storageMode = try values.decode(String.self, forKey: .storageMode)
        migrationRequiredAccounts = try values.decode(
            Int.self, forKey: .migrationRequiredAccounts
        )
        recoveryRequiredAccounts = try values.decode(
            Int.self, forKey: .recoveryRequiredAccounts
        )
        guard schemaVersion == 1,
              ["unlocked", "locked", "migration_required", "recovery_required"]
                .contains(status),
              ["local_key_file", "passphrase", "external"].contains(storageMode),
              migrationRequiredAccounts >= 0,
              recoveryRequiredAccounts >= 0 else {
            throw LocalProductWireError.invalidJSON
        }
    }
}

public struct LocalProductCredentialVaultExportResult:
    Decodable, Equatable, Sendable
{
    public let schemaVersion: Int
    public let filePath: String
    public let digest: String
    public let credentialCount: Int

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case filePath = "file_path"
        case digest
        case credentialCount = "credential_count"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownSetupKeys(decoder, allowed: [
            "schema_version", "file_path", "digest", "credential_count",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
        filePath = try values.decode(String.self, forKey: .filePath)
        digest = try values.decode(String.self, forKey: .digest)
        credentialCount = try values.decode(Int.self, forKey: .credentialCount)
        guard schemaVersion == 1, filePath.hasPrefix("/"),
              filePath.hasSuffix(".loomvault"), filePath.utf8.count <= 1_024,
              digest.utf8.count == 64,
              digest.utf8.allSatisfy({
                  ($0 >= 48 && $0 <= 57) || ($0 >= 97 && $0 <= 102)
              }),
              credentialCount >= 0, credentialCount <= 256 else {
            throw DecodingError.dataCorrupted(
                .init(codingPath: decoder.codingPath, debugDescription: "Invalid Vault export result")
            )
        }
    }
}

public struct LocalProductConversationProfile:
    Codable, Equatable, Sendable, Identifiable
{
    public var id: String { profileID }
    public let profileID: String
    public let harnessAdapter: String
    public let providerID: String
    public let providerAccountID: String
    public let displayName: String
    public let protocolFamily: String
    public let modelID: String
    public let authMode: String
    public let credentialRevision: Int64
    public let policyVersion: Int
    public let policyRevision: Int64
    public let policyDigest: String
    public let trustDomain: String
    public let retentionMode: String
    public let dataRegion: String

    enum CodingKeys: String, CodingKey {
        case profileID = "profile_id"
        case harnessAdapter = "harness_adapter"
        case providerID = "provider_id"
        case providerAccountID = "provider_account_id"
        case displayName = "display_name"
        case protocolFamily = "protocol"
        case modelID = "model_id"
        case authMode = "auth_mode"
        case credentialRevision = "credential_revision"
        case policyVersion = "policy_version"
        case policyRevision = "policy_revision"
        case policyDigest = "policy_digest"
        case trustDomain = "trust_domain"
        case retentionMode = "retention_mode"
        case dataRegion = "data_region"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownSetupKeys(decoder, allowed: [
            "profile_id", "harness_adapter", "provider_id",
            "provider_account_id", "display_name", "protocol",
            "model_id", "auth_mode", "credential_revision",
            "policy_version", "policy_revision", "policy_digest",
            "trust_domain", "retention_mode", "data_region",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        profileID = try values.decode(String.self, forKey: .profileID)
        harnessAdapter = try values.decodeIfPresent(
            String.self,
            forKey: .harnessAdapter
        ) ?? ""
        providerID = try values.decode(String.self, forKey: .providerID)
        providerAccountID = try values.decodeIfPresent(
            String.self,
            forKey: .providerAccountID
        ) ?? ""
        displayName = try values.decode(String.self, forKey: .displayName)
        protocolFamily = try values.decode(String.self, forKey: .protocolFamily)
        modelID = try values.decode(String.self, forKey: .modelID)
        authMode = try values.decode(String.self, forKey: .authMode)
        credentialRevision = try values.decode(
            Int64.self,
            forKey: .credentialRevision
        )
        policyVersion = try values.decodeIfPresent(Int.self, forKey: .policyVersion) ?? 0
        policyRevision = try values.decodeIfPresent(Int64.self, forKey: .policyRevision) ?? 0
        policyDigest = try values.decodeIfPresent(String.self, forKey: .policyDigest) ?? ""
        trustDomain = try values.decodeIfPresent(String.self, forKey: .trustDomain) ?? ""
        retentionMode = try values.decodeIfPresent(String.self, forKey: .retentionMode) ?? ""
        dataRegion = try values.decodeIfPresent(String.self, forKey: .dataRegion) ?? ""

        let validDisplayName = (1...128).contains(displayName.utf8.count) &&
            displayName.unicodeScalars.allSatisfy {
                !CharacterSet.controlCharacters.contains($0)
            }
        let validModelID = (1...256).contains(modelID.utf8.count) &&
            modelID.unicodeScalars.allSatisfy {
                $0.value >= 0x21 && $0.value <= 0x7e
            }
        guard LocalIPCClient.validIdentifier(profileID),
              LocalIPCClient.validIdentifier(harnessAdapter),
              LocalIPCClient.validIdentifier(providerID),
              LocalIPCClient.validIdentifier(protocolFamily),
              validDisplayName, validModelID else {
            throw LocalProductWireError.invalidJSON
        }
        switch authMode {
        case "native_auth":
            guard providerAccountID.isEmpty, credentialRevision == 0,
                  policyVersion == 0, policyRevision == 0, policyDigest.isEmpty,
                  trustDomain.isEmpty, retentionMode.isEmpty, dataRegion.isEmpty else {
                throw LocalProductWireError.invalidJSON
            }
        case "brokered":
            guard LocalIPCClient.validProviderAccountID(
                providerAccountID,
                providerID: providerID
            ), credentialRevision > 0,
                  (policyVersion == 0 && policyRevision == 0 && policyDigest.isEmpty
                    && trustDomain.isEmpty && retentionMode.isEmpty && dataRegion.isEmpty)
                    || ([1, 2].contains(policyVersion) && policyRevision > 0
                      && (policyVersion == 1
                        ? policyDigest.isEmpty || LocalProductProviderAccountDirectoryEntry.validPolicyDigest(policyDigest)
                        : LocalProductProviderAccountDirectoryEntry.validPolicyDigest(policyDigest))
                      && LocalProductProviderAccountDirectoryEntry.validDisclosurePolicy(
                        version: policyVersion, trustDomain: trustDomain,
                        retentionMode: retentionMode, dataRegion: dataRegion
                      )) else {
                throw LocalProductWireError.invalidJSON
            }
        default:
            throw LocalProductWireError.invalidJSON
        }
    }
}

public struct LocalProductProviderConnectResult:
    Codable, Equatable, Sendable
{
    public let providerID: String
    public let authMode: String
    public let status: String

    enum CodingKeys: String, CodingKey {
        case providerID = "provider_id"
        case authMode = "auth_mode"
        case status
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownSetupKeys(
            decoder,
            allowed: ["provider_id", "auth_mode", "status"]
        )
        let values = try decoder.container(keyedBy: CodingKeys.self)
        providerID = try values.decode(String.self, forKey: .providerID)
        authMode = try values.decode(String.self, forKey: .authMode)
        status = try values.decode(String.self, forKey: .status)
        guard providerID == "codex",
              authMode == "native_auth",
              ["started", "already_connected"].contains(status) else {
            throw LocalProductWireError.invalidJSON
        }
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
	public let harnessAdapter: String
	public let providerID: String
	public let providerAccountID: String
	public let modelID: String
	public let authMode: String
	public let credentialRevision: Int64
	public let reasoningEffort: String
	public let timeoutMilliseconds: Int64
	public let budgetAvailable: Bool
	public let budgetUnits: Int64
	public let requiredCapabilities: [String]
	public let remoteToolEnrollmentID: String
	public let remoteToolEnrollmentDigest: String
    public let skillRevisionIDs: [String]
    public let permissionIDs: [String]
    public let resourceIDs: [String]
    public let responsibility: String

    enum CodingKeys: String, CodingKey {
        case id, kind
        case agentDefinitionID = "agent_definition_id"
        case runtimeProfileID = "runtime_profile_id"
        case runtimeInstanceID = "runtime_instance_id"
		case harnessAdapter = "harness_adapter"
		case providerID = "provider_id"
		case providerAccountID = "provider_account_id"
		case modelID = "model_id"
		case authMode = "auth_mode"
		case credentialRevision = "credential_revision"
		case reasoningEffort = "reasoning_effort"
		case timeoutMilliseconds = "timeout_milliseconds"
		case budgetAvailable = "budget_available"
		case budgetUnits = "budget_units"
		case requiredCapabilities = "required_capabilities"
		case remoteToolEnrollmentID = "remote_tool_enrollment_id"
		case remoteToolEnrollmentDigest = "remote_tool_enrollment_digest"
        case skillRevisionIDs = "skill_revision_ids"
        case permissionIDs = "permission_ids"
        case resourceIDs = "resource_ids"
        case responsibility
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownSetupKeys(decoder, allowed: [
            "id", "kind", "agent_definition_id", "runtime_profile_id",
            "runtime_instance_id", "skill_revision_ids", "permission_ids",
			"resource_ids", "responsibility", "harness_adapter",
			"provider_id", "provider_account_id", "model_id", "auth_mode",
			"credential_revision", "timeout_milliseconds", "budget_available",
			"budget_units", "required_capabilities", "reasoning_effort",
			"remote_tool_enrollment_id", "remote_tool_enrollment_digest",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        id = try values.decode(String.self, forKey: .id)
        kind = try values.decode(String.self, forKey: .kind)
        agentDefinitionID = try values.decode(String.self, forKey: .agentDefinitionID)
        runtimeProfileID = try values.decode(String.self, forKey: .runtimeProfileID)
        runtimeInstanceID = try values.decode(String.self, forKey: .runtimeInstanceID)
		harnessAdapter = try values.decodeIfPresent(String.self, forKey: .harnessAdapter) ?? ""
		providerID = try values.decodeIfPresent(String.self, forKey: .providerID) ?? ""
		providerAccountID = try values.decodeIfPresent(
			String.self, forKey: .providerAccountID
		) ?? ""
		modelID = try values.decodeIfPresent(String.self, forKey: .modelID) ?? ""
		authMode = try values.decodeIfPresent(String.self, forKey: .authMode) ?? ""
		credentialRevision = try values.decodeIfPresent(
			Int64.self, forKey: .credentialRevision
		) ?? 0
		reasoningEffort = try values.decodeIfPresent(
			String.self, forKey: .reasoningEffort
		) ?? ""
		timeoutMilliseconds = try values.decodeIfPresent(
			Int64.self, forKey: .timeoutMilliseconds
		) ?? 0
		budgetAvailable = try values.decodeIfPresent(
			Bool.self, forKey: .budgetAvailable
		) ?? false
		budgetUnits = try values.decodeIfPresent(Int64.self, forKey: .budgetUnits) ?? 0
		requiredCapabilities = try values.decodeIfPresent(
			[String].self, forKey: .requiredCapabilities
		) ?? []
		remoteToolEnrollmentID = try values.decodeIfPresent(
			String.self, forKey: .remoteToolEnrollmentID
		) ?? ""
		remoteToolEnrollmentDigest = try values.decodeIfPresent(
			String.self, forKey: .remoteToolEnrollmentDigest
		) ?? ""
		guard validRemoteToolEnrollmentPair(
			enrollmentID: remoteToolEnrollmentID,
			enrollmentDigest: remoteToolEnrollmentDigest
		) else { throw LocalProductWireError.invalidJSON }
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
    public let providers: [LocalProductProviderDirectoryEntry]
    public let providerAccounts: [LocalProductProviderAccountDirectoryEntry]
    public let credentialVault: LocalProductCredentialVaultStatus?
    public let conversationProfiles: [LocalProductConversationProfile]
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
        case codex, providers, runtimes, templates, skills, permissions, resources
        case providerAccounts = "provider_accounts"
        case credentialVault = "credential_vault"
        case conversationProfiles = "conversation_profiles"
        case miniMax = "minimax"
        case savedTeams = "saved_teams"
        case roleOptions = "role_options"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownSetupKeys(decoder, allowed: [
            "schema_version", "view_version", "codex", "minimax", "providers",
            "provider_accounts",
            "credential_vault",
            "conversation_profiles", "runtimes",
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
        providers = try values.decodeIfPresent(
            [LocalProductProviderDirectoryEntry].self,
            forKey: .providers
        ) ?? []
        providerAccounts = try values.decodeIfPresent(
            [LocalProductProviderAccountDirectoryEntry].self,
            forKey: .providerAccounts
        ) ?? []
        credentialVault = try values.decodeIfPresent(
            LocalProductCredentialVaultStatus.self,
            forKey: .credentialVault
        )
        conversationProfiles = try values.decodeIfPresent(
            [LocalProductConversationProfile].self,
            forKey: .conversationProfiles
        ) ?? []
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

public struct LocalProductBuilderExecutionRoute:
    Codable, Equatable, Sendable, Identifiable
{
	fileprivate static let empty = LocalProductBuilderExecutionRoute()
    public var id: String { runtimeProfileID }
    public let runtimeProfileID: String
    public let harnessAdapter: String
    public let providerID: String
    public let providerAccountID: String
    public let modelID: String
    public let authMode: String
    public let credentialRevision: Int64
    public let reasoningEffort: String
    public let timeoutMilliseconds: Int64
    public let budgetAvailable: Bool
    public let budgetUnits: Int64
    public let requiredCapabilities: [String]
    public let remoteToolEnrollmentID: String
    public let remoteToolEnrollmentDigest: String

    fileprivate var isValid: Bool {
        validBuilderExecutionProfile(
            profileID: runtimeProfileID,
            harnessAdapter: harnessAdapter,
            providerID: providerID,
            providerAccountID: providerAccountID,
            modelID: modelID,
            authMode: authMode,
            credentialRevision: credentialRevision,
            reasoningEffort: reasoningEffort,
            timeoutMilliseconds: timeoutMilliseconds,
            budgetAvailable: budgetAvailable,
            budgetUnits: budgetUnits,
            capabilities: requiredCapabilities
        ) && validRemoteToolEnrollmentPair(
            enrollmentID: remoteToolEnrollmentID,
            enrollmentDigest: remoteToolEnrollmentDigest
        )
    }

    fileprivate var isEmpty: Bool {
        runtimeProfileID.isEmpty && harnessAdapter.isEmpty &&
            providerID.isEmpty && providerAccountID.isEmpty && modelID.isEmpty &&
            authMode.isEmpty && credentialRevision == 0 && reasoningEffort.isEmpty &&
            timeoutMilliseconds == 0 && !budgetAvailable && budgetUnits == 0 &&
            requiredCapabilities.isEmpty && remoteToolEnrollmentID.isEmpty &&
            remoteToolEnrollmentDigest.isEmpty
    }

    enum CodingKeys: String, CodingKey {
        case runtimeProfileID = "runtime_profile_id"
        case harnessAdapter = "harness_adapter"
        case providerID = "provider_id"
        case providerAccountID = "provider_account_id"
        case modelID = "model_id"
        case authMode = "auth_mode"
        case credentialRevision = "credential_revision"
        case reasoningEffort = "reasoning_effort"
        case timeoutMilliseconds = "timeout_milliseconds"
        case budgetAvailable = "budget_available"
        case budgetUnits = "budget_units"
        case requiredCapabilities = "required_capabilities"
        case remoteToolEnrollmentID = "remote_tool_enrollment_id"
        case remoteToolEnrollmentDigest = "remote_tool_enrollment_digest"
    }

	private init() {
		runtimeProfileID = ""
		harnessAdapter = ""
		providerID = ""
		providerAccountID = ""
		modelID = ""
		authMode = ""
		credentialRevision = 0
		reasoningEffort = ""
		timeoutMilliseconds = 0
		budgetAvailable = false
		budgetUnits = 0
		requiredCapabilities = []
		remoteToolEnrollmentID = ""
		remoteToolEnrollmentDigest = ""
	}

    public init(from decoder: Decoder) throws {
        try rejectUnknownSetupKeys(decoder, allowed: [
            "runtime_profile_id", "harness_adapter", "provider_id",
            "provider_account_id", "model_id", "auth_mode",
            "credential_revision", "reasoning_effort", "timeout_milliseconds",
            "budget_available", "budget_units", "required_capabilities",
            "remote_tool_enrollment_id", "remote_tool_enrollment_digest",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        runtimeProfileID = try values.decode(String.self, forKey: .runtimeProfileID)
        harnessAdapter = try values.decode(String.self, forKey: .harnessAdapter)
        providerID = try values.decode(String.self, forKey: .providerID)
        providerAccountID = try values.decode(String.self, forKey: .providerAccountID)
        modelID = try values.decode(String.self, forKey: .modelID)
        authMode = try values.decode(String.self, forKey: .authMode)
        credentialRevision = try values.decode(Int64.self, forKey: .credentialRevision)
        reasoningEffort = try values.decode(String.self, forKey: .reasoningEffort)
        timeoutMilliseconds = try values.decode(Int64.self, forKey: .timeoutMilliseconds)
        budgetAvailable = try values.decode(Bool.self, forKey: .budgetAvailable)
        budgetUnits = try values.decode(Int64.self, forKey: .budgetUnits)
        requiredCapabilities = try values.decode(
            [String].self, forKey: .requiredCapabilities
        )
		remoteToolEnrollmentID = try values.decodeIfPresent(
			String.self, forKey: .remoteToolEnrollmentID
		) ?? ""
		remoteToolEnrollmentDigest = try values.decodeIfPresent(
			String.self, forKey: .remoteToolEnrollmentDigest
		) ?? ""
		guard isValid || isEmpty else {
			throw LocalProductWireError.invalidJSON
		}
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
	public let harnessAdapter: String
	public let providerID: String
	public let providerAccountID: String
    public let modelID: String
	public let authMode: String
	public let credentialRevision: Int64
	public let reasoningEffort: String
	public let timeoutMilliseconds: Int64
	public let budgetAvailable: Bool
	public let budgetUnits: Int64
	public let requiredCapabilities: [String]
	public let remoteToolEnrollmentID: String
	public let remoteToolEnrollmentDigest: String
	public let fallbackConfigured: Bool
	public let fallbackRuntimeProfileID: String
	public let fallbackHarnessAdapter: String
	public let fallbackProviderID: String
	public let fallbackProviderAccountID: String
	public let fallbackModelID: String
	public let fallbackAuthMode: String
	public let fallbackCredentialRevision: Int64
	public let fallbackReasoningEffort: String
	public let fallbackTimeoutMilliseconds: Int64
	public let fallbackBudgetAvailable: Bool
	public let fallbackBudgetUnits: Int64
	public let fallbackRequiredCapabilities: [String]
	public let fallbackApprovalRequired: Bool
	public let parallelRouteSetVersion: Int
	public let parallelRoutes: [LocalProductBuilderExecutionRoute]
	public let synthesisRoute: LocalProductBuilderExecutionRoute
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
		case harnessAdapter = "harness_adapter"
		case providerID = "provider_id"
		case providerAccountID = "provider_account_id"
        case modelID = "model_id"
        case authMode = "auth_mode"
		case credentialRevision = "credential_revision"
		case reasoningEffort = "reasoning_effort"
		case timeoutMilliseconds = "timeout_milliseconds"
		case budgetAvailable = "budget_available"
		case budgetUnits = "budget_units"
		case requiredCapabilities = "required_capabilities"
		case remoteToolEnrollmentID = "remote_tool_enrollment_id"
		case remoteToolEnrollmentDigest = "remote_tool_enrollment_digest"
		case fallbackConfigured = "fallback_configured"
		case fallbackRuntimeProfileID = "fallback_runtime_profile_id"
		case fallbackHarnessAdapter = "fallback_harness_adapter"
		case fallbackProviderID = "fallback_provider_id"
		case fallbackProviderAccountID = "fallback_provider_account_id"
		case fallbackModelID = "fallback_model_id"
		case fallbackAuthMode = "fallback_auth_mode"
		case fallbackCredentialRevision = "fallback_credential_revision"
		case fallbackReasoningEffort = "fallback_reasoning_effort"
		case fallbackTimeoutMilliseconds = "fallback_timeout_milliseconds"
		case fallbackBudgetAvailable = "fallback_budget_available"
		case fallbackBudgetUnits = "fallback_budget_units"
		case fallbackRequiredCapabilities = "fallback_required_capabilities"
		case fallbackApprovalRequired = "fallback_approval_required"
		case parallelRouteSetVersion = "parallel_route_set_version"
		case parallelRoutes = "parallel_routes"
		case synthesisRoute = "synthesis_route"
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
			"harness_adapter", "provider_id", "provider_account_id",
			"model_id", "auth_mode", "credential_revision", "reasoning_effort",
			"timeout_milliseconds", "budget_available", "budget_units",
			"required_capabilities", "remote_tool_enrollment_id",
			"remote_tool_enrollment_digest", "fallback_configured",
			"fallback_runtime_profile_id", "fallback_harness_adapter",
			"fallback_provider_id", "fallback_provider_account_id",
			"fallback_model_id", "fallback_auth_mode",
			"fallback_credential_revision", "fallback_reasoning_effort",
			"fallback_timeout_milliseconds", "fallback_budget_available",
			"fallback_budget_units", "fallback_required_capabilities",
			"fallback_approval_required", "skills", "permission_ids",
			"parallel_route_set_version", "parallel_routes", "synthesis_route",
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
		harnessAdapter = try values.decodeIfPresent(String.self, forKey: .harnessAdapter) ?? ""
		providerID = try values.decodeIfPresent(String.self, forKey: .providerID) ?? ""
		providerAccountID = try values.decodeIfPresent(
			String.self, forKey: .providerAccountID
		) ?? ""
        modelID = try values.decode(String.self, forKey: .modelID)
        authMode = try values.decode(String.self, forKey: .authMode)
		credentialRevision = try values.decodeIfPresent(
			Int64.self, forKey: .credentialRevision
		) ?? 0
		reasoningEffort = try values.decodeIfPresent(
			String.self, forKey: .reasoningEffort
		) ?? ""
		timeoutMilliseconds = try values.decodeIfPresent(
			Int64.self, forKey: .timeoutMilliseconds
		) ?? 0
		budgetAvailable = try values.decodeIfPresent(
			Bool.self, forKey: .budgetAvailable
		) ?? false
		budgetUnits = try values.decodeIfPresent(Int64.self, forKey: .budgetUnits) ?? 0
		requiredCapabilities = try values.decodeIfPresent(
			[String].self, forKey: .requiredCapabilities
		) ?? []
		remoteToolEnrollmentID = try values.decodeIfPresent(
			String.self, forKey: .remoteToolEnrollmentID
		) ?? ""
		remoteToolEnrollmentDigest = try values.decodeIfPresent(
			String.self, forKey: .remoteToolEnrollmentDigest
		) ?? ""
		fallbackConfigured = try values.decodeIfPresent(
			Bool.self, forKey: .fallbackConfigured
		) ?? false
		fallbackRuntimeProfileID = try values.decodeIfPresent(
			String.self, forKey: .fallbackRuntimeProfileID
		) ?? ""
		fallbackHarnessAdapter = try values.decodeIfPresent(
			String.self, forKey: .fallbackHarnessAdapter
		) ?? ""
		fallbackProviderID = try values.decodeIfPresent(
			String.self, forKey: .fallbackProviderID
		) ?? ""
		fallbackProviderAccountID = try values.decodeIfPresent(
			String.self, forKey: .fallbackProviderAccountID
		) ?? ""
		fallbackModelID = try values.decodeIfPresent(
			String.self, forKey: .fallbackModelID
		) ?? ""
		fallbackAuthMode = try values.decodeIfPresent(
			String.self, forKey: .fallbackAuthMode
		) ?? ""
		fallbackCredentialRevision = try values.decodeIfPresent(
			Int64.self, forKey: .fallbackCredentialRevision
		) ?? 0
		fallbackReasoningEffort = try values.decodeIfPresent(
			String.self, forKey: .fallbackReasoningEffort
		) ?? ""
		fallbackTimeoutMilliseconds = try values.decodeIfPresent(
			Int64.self, forKey: .fallbackTimeoutMilliseconds
		) ?? 0
		fallbackBudgetAvailable = try values.decodeIfPresent(
			Bool.self, forKey: .fallbackBudgetAvailable
		) ?? false
		fallbackBudgetUnits = try values.decodeIfPresent(
			Int64.self, forKey: .fallbackBudgetUnits
		) ?? 0
		fallbackRequiredCapabilities = try values.decodeIfPresent(
			[String].self, forKey: .fallbackRequiredCapabilities
		) ?? []
		fallbackApprovalRequired = try values.decodeIfPresent(
			Bool.self, forKey: .fallbackApprovalRequired
		) ?? false
		parallelRouteSetVersion = try values.decodeIfPresent(
			Int.self, forKey: .parallelRouteSetVersion
		) ?? 0
		parallelRoutes = try values.decodeIfPresent(
			[LocalProductBuilderExecutionRoute].self, forKey: .parallelRoutes
		) ?? []
		synthesisRoute = try values.decodeIfPresent(
			LocalProductBuilderExecutionRoute.self, forKey: .synthesisRoute
		) ?? .empty
		guard validBuilderExecutionProfile(
			profileID: runtimeProfileID,
			harnessAdapter: harnessAdapter,
			providerID: providerID,
			providerAccountID: providerAccountID,
			modelID: modelID,
			authMode: authMode,
			credentialRevision: credentialRevision,
			reasoningEffort: reasoningEffort,
			timeoutMilliseconds: timeoutMilliseconds,
			budgetAvailable: budgetAvailable,
			budgetUnits: budgetUnits,
			capabilities: requiredCapabilities
		), validRemoteToolEnrollmentPair(
			enrollmentID: remoteToolEnrollmentID,
			enrollmentDigest: remoteToolEnrollmentDigest
		) else { throw LocalProductWireError.invalidJSON }
		let hasCompleteFallback = validBuilderExecutionProfile(
			profileID: fallbackRuntimeProfileID,
			harnessAdapter: fallbackHarnessAdapter,
			providerID: fallbackProviderID,
			providerAccountID: fallbackProviderAccountID,
			modelID: fallbackModelID,
			authMode: fallbackAuthMode,
			credentialRevision: fallbackCredentialRevision,
			reasoningEffort: fallbackReasoningEffort,
			timeoutMilliseconds: fallbackTimeoutMilliseconds,
			budgetAvailable: fallbackBudgetAvailable,
			budgetUnits: fallbackBudgetUnits,
			capabilities: fallbackRequiredCapabilities
		) &&
			fallbackApprovalRequired
		let hasNoFallback = fallbackRuntimeProfileID.isEmpty &&
			fallbackHarnessAdapter.isEmpty &&
			fallbackProviderID.isEmpty &&
			fallbackProviderAccountID.isEmpty &&
			fallbackModelID.isEmpty &&
			fallbackAuthMode.isEmpty &&
			fallbackCredentialRevision == 0 &&
			fallbackReasoningEffort.isEmpty &&
			fallbackTimeoutMilliseconds == 0 &&
			!fallbackBudgetAvailable && fallbackBudgetUnits == 0 &&
			fallbackRequiredCapabilities.isEmpty &&
			!fallbackApprovalRequired
		guard (fallbackConfigured && hasCompleteFallback) ||
			(!fallbackConfigured && hasNoFallback) else {
			throw LocalProductWireError.invalidJSON
		}
		let routeProfileIDs = parallelRoutes.map(\.runtimeProfileID)
		let hasParallelRouteSet = parallelRouteSetVersion == 1 &&
			(2...3).contains(parallelRoutes.count) &&
			parallelRoutes.allSatisfy(\.isValid) &&
			Set(routeProfileIDs).count == routeProfileIDs.count &&
			parallelRoutes.first?.runtimeProfileID == runtimeProfileID &&
			synthesisRoute.isValid
		let hasNoParallelRouteSet = parallelRouteSetVersion == 0 &&
			parallelRoutes.isEmpty && synthesisRoute.isEmpty
		guard (hasParallelRouteSet || hasNoParallelRouteSet) &&
			!(fallbackConfigured && hasParallelRouteSet) else {
			throw LocalProductWireError.invalidJSON
		}
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

extension LocalProductBuilderPreview {
    /// Explains why a governed Team draft cannot be confirmed yet. The Team
    /// builder commits each field on Return; a user who types without Return
    /// sees the Confirm button disabled, so the message must say what to do.
    public func confirmationBlockedMessage(
        uncommittedName: String,
        uncommittedPurpose: String
    ) -> String {
        let nameEdited = !uncommittedName.isEmpty
            && uncommittedName != name
        let purposeEdited = !uncommittedPurpose.isEmpty
            && uncommittedPurpose != purpose
        if nameEdited || purposeEdited {
            return "Press Return in Team name / Bounded purpose to apply the changes, then confirm."
        }
        if !compatibilityGaps.isEmpty {
            let count = compatibilityGaps.count
            return "Resolve \(count) compatibility issue"
                + (count == 1 ? "" : "s")
                + " before confirming."
        }
        return "Complete the required fields to enable confirmation."
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

    public static func decodeProviderConnectResult(
        _ data: Data
    ) throws -> LocalProductProviderConnectResult {
        try decode(LocalProductProviderConnectResult.self, data)
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
