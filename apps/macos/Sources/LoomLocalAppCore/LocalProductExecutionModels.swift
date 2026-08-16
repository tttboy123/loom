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

private func validExecutionRoute(
  harnessAdapter: String,
  providerID: String,
  providerAccountID: String,
  modelID: String,
  authMode: String,
  credentialRevision: Int,
  reasoningEffort: String,
  timeoutSeconds: Int,
  budgetCredits: Int?,
  capabilities: [String],
  status: String,
  blockReason: String
) -> Bool {
  let validModelID = (1...256).contains(modelID.utf8.count) &&
    modelID.unicodeScalars.allSatisfy { $0.value >= 0x21 && $0.value <= 0x7e }
  let validCapabilities = capabilities == Array(Set(capabilities)).sorted() &&
    capabilities.allSatisfy(LocalIPCClient.validIdentifier)
  let validStatus = status == "ready"
    ? blockReason.isEmpty
    : status == "blocked" && !blockReason.isEmpty
  guard LocalIPCClient.validIdentifier(harnessAdapter),
        LocalIPCClient.validIdentifier(providerID), validModelID,
        timeoutSeconds > 0, budgetCredits.map({ $0 >= 0 }) ?? true,
        validCapabilities, validStatus else {
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
  guard reasoningEffort.isEmpty ||
    ((1...32).contains(reasoningEffort.utf8.count) &&
      LocalIPCClient.validIdentifier(reasoningEffort) &&
      capabilities.contains("reasoning_effort")) else {
    return false
  }
  return true
}

public struct LocalProductExecutionCommand: Encodable, Equatable, Sendable {
  public let schemaVersion: Int
  public let operation: String
  public let missionID: String
  public let teamInstanceID: String
  public let workPackageID: String
  public let workPackageDigest: String
  public let objective: String
  public let contextVersion: Int
  public let confirmedConstraints: [String]
  public let acceptedDecisions: [String]
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
    case contextVersion = "context_version"
    case confirmedConstraints = "confirmed_constraints"
    case acceptedDecisions = "accepted_decisions"
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
    confirmedConstraints: [String] = [],
    acceptedDecisions: [String] = [],
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
      contextVersion: 1,
      confirmedConstraints: confirmedConstraints,
      acceptedDecisions: acceptedDecisions,
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
    confirmedConstraints: [String] = [],
    acceptedDecisions: [String] = [],
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
      contextVersion: 1,
      confirmedConstraints: confirmedConstraints,
      acceptedDecisions: acceptedDecisions,
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
      contextVersion: 0,
      confirmedConstraints: [],
      acceptedDecisions: [],
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
    if contextVersion != 0 {
      try values.encode(contextVersion, forKey: .contextVersion)
      try values.encode(confirmedConstraints, forKey: .confirmedConstraints)
      try values.encode(acceptedDecisions, forKey: .acceptedDecisions)
    }
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
	public let kind: String
	public let routeGroupID: String
  public let dependsOn: [String]
  public let maxAttempts: Int
  public let harnessAdapter: String
  public let providerID: String
  public let providerAccountID: String
  public let modelID: String
  public let authMode: String
  public let credentialRevision: Int
  public let reasoningEffort: String
  public let timeoutSeconds: Int
  public let budgetCredits: Int?
  public let capabilities: [String]
  public let status: String
  public let blockReason: String
	public let fallbackConfigured: Bool
	public let fallbackHarnessAdapter: String
	public let fallbackProviderID: String
	public let fallbackProviderAccountID: String
	public let fallbackModelID: String
	public let fallbackAuthMode: String
	public let fallbackCredentialRevision: Int
	public let fallbackReasoningEffort: String
	public let fallbackTimeoutSeconds: Int
	public let fallbackBudgetCredits: Int?
	public let fallbackCapabilities: [String]
	public let fallbackStatus: String
	public let fallbackBlockReason: String
	public let fallbackApprovalRequired: Bool
	public let fallbackApprovalAvailable: Bool
	public let fallbackApprovalVersion: Int
	public let remoteToolEnrollmentAvailable: Bool
	public let remoteToolEnrollmentID: String
	public let remoteToolBackendKind: String
	public let remoteToolBindingDigest: String

  enum CodingKeys: String, CodingKey {
    case logicalNodeID = "logical_node_id"
    case title, role, kind
	case routeGroupID = "route_group_id"
    case dependsOn = "depends_on"
    case maxAttempts = "max_attempts"
    case harnessAdapter = "harness_adapter"
    case providerID = "provider_id"
    case providerAccountID = "provider_account_id"
    case modelID = "model_id"
    case authMode = "auth_mode"
    case credentialRevision = "credential_revision"
    case reasoningEffort = "reasoning_effort"
    case timeoutSeconds = "timeout_seconds"
    case budgetCredits = "budget_credits"
    case capabilities, status
    case blockReason = "block_reason"
	case fallbackConfigured = "fallback_configured"
	case fallbackHarnessAdapter = "fallback_harness_adapter"
	case fallbackProviderID = "fallback_provider_id"
	case fallbackProviderAccountID = "fallback_provider_account_id"
	case fallbackModelID = "fallback_model_id"
	case fallbackAuthMode = "fallback_auth_mode"
	case fallbackCredentialRevision = "fallback_credential_revision"
	case fallbackReasoningEffort = "fallback_reasoning_effort"
	case fallbackTimeoutSeconds = "fallback_timeout_seconds"
	case fallbackBudgetCredits = "fallback_budget_credits"
	case fallbackCapabilities = "fallback_capabilities"
	case fallbackStatus = "fallback_status"
	case fallbackBlockReason = "fallback_block_reason"
	case fallbackApprovalRequired = "fallback_approval_required"
	case fallbackApprovalAvailable = "fallback_approval_available"
	case fallbackApprovalVersion = "fallback_approval_version"
	case remoteToolEnrollmentAvailable = "remote_tool_enrollment_available"
	case remoteToolEnrollmentID = "remote_tool_enrollment_id"
	case remoteToolBackendKind = "remote_tool_backend_kind"
	case remoteToolBindingDigest = "remote_tool_binding_digest"
  }

  public init(from decoder: Decoder) throws {
    try rejectExecutionUnknownKeys(
      decoder,
      allowed: [
        "logical_node_id", "title", "role", "kind", "route_group_id",
		"depends_on", "max_attempts",
        "harness_adapter", "provider_id", "provider_account_id", "model_id",
		"auth_mode", "credential_revision", "reasoning_effort", "timeout_seconds",
        "budget_credits", "capabilities", "status", "block_reason",
		"fallback_configured", "fallback_harness_adapter",
		"fallback_provider_id", "fallback_provider_account_id",
		"fallback_model_id", "fallback_auth_mode", "fallback_credential_revision",
		"fallback_reasoning_effort", "fallback_timeout_seconds",
		"fallback_budget_credits",
		"fallback_capabilities", "fallback_status",
		"fallback_block_reason", "fallback_approval_required",
		"fallback_approval_available", "fallback_approval_version",
		"remote_tool_enrollment_available", "remote_tool_enrollment_id",
		"remote_tool_backend_kind", "remote_tool_binding_digest",
      ])
    let values = try decoder.container(keyedBy: CodingKeys.self)
    logicalNodeID = try values.decode(String.self, forKey: .logicalNodeID)
    title = try values.decode(String.self, forKey: .title)
    role = try values.decode(String.self, forKey: .role)
	kind = try values.decodeIfPresent(String.self, forKey: .kind) ?? ""
	routeGroupID = try values.decodeIfPresent(
	  String.self, forKey: .routeGroupID
	) ?? ""
    dependsOn = try values.decode([String].self, forKey: .dependsOn)
    maxAttempts = try values.decode(Int.self, forKey: .maxAttempts)
    harnessAdapter = try values.decodeIfPresent(String.self, forKey: .harnessAdapter) ?? ""
    providerID = try values.decodeIfPresent(String.self, forKey: .providerID) ?? ""
    providerAccountID = try values.decodeIfPresent(String.self, forKey: .providerAccountID) ?? ""
    modelID = try values.decodeIfPresent(String.self, forKey: .modelID) ?? ""
    authMode = try values.decodeIfPresent(String.self, forKey: .authMode) ?? ""
    credentialRevision = try values.decodeIfPresent(Int.self, forKey: .credentialRevision) ?? 0
    reasoningEffort = try values.decodeIfPresent(String.self, forKey: .reasoningEffort) ?? ""
    timeoutSeconds = try values.decodeIfPresent(Int.self, forKey: .timeoutSeconds) ?? 0
    budgetCredits = try values.decodeIfPresent(Int.self, forKey: .budgetCredits)
    capabilities = try values.decodeIfPresent([String].self, forKey: .capabilities) ?? []
    status = try values.decodeIfPresent(String.self, forKey: .status) ?? "ready"
    blockReason = try values.decodeIfPresent(String.self, forKey: .blockReason) ?? ""
	fallbackConfigured = try values.decodeIfPresent(
	  Bool.self, forKey: .fallbackConfigured
	) ?? false
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
	  Int.self, forKey: .fallbackCredentialRevision
	) ?? 0
	fallbackReasoningEffort = try values.decodeIfPresent(
	  String.self, forKey: .fallbackReasoningEffort
	) ?? ""
	fallbackTimeoutSeconds = try values.decodeIfPresent(
	  Int.self, forKey: .fallbackTimeoutSeconds
	) ?? 0
	fallbackBudgetCredits = try values.decodeIfPresent(
	  Int.self, forKey: .fallbackBudgetCredits
	)
	fallbackCapabilities = try values.decodeIfPresent(
	  [String].self, forKey: .fallbackCapabilities
	) ?? []
	fallbackStatus = try values.decodeIfPresent(
	  String.self, forKey: .fallbackStatus
	) ?? ""
	fallbackBlockReason = try values.decodeIfPresent(
	  String.self, forKey: .fallbackBlockReason
	) ?? ""
	fallbackApprovalRequired = try values.decodeIfPresent(
	  Bool.self, forKey: .fallbackApprovalRequired
	) ?? false
	fallbackApprovalAvailable = try values.decodeIfPresent(
	  Bool.self, forKey: .fallbackApprovalAvailable
	) ?? false
	fallbackApprovalVersion = try values.decodeIfPresent(
	  Int.self, forKey: .fallbackApprovalVersion
	) ?? 0
	remoteToolEnrollmentAvailable = try values.decodeIfPresent(
	  Bool.self, forKey: .remoteToolEnrollmentAvailable
	) ?? false
	remoteToolEnrollmentID = try values.decodeIfPresent(
	  String.self, forKey: .remoteToolEnrollmentID
	) ?? ""
	remoteToolBackendKind = try values.decodeIfPresent(
	  String.self, forKey: .remoteToolBackendKind
	) ?? ""
	remoteToolBindingDigest = try values.decodeIfPresent(
	  String.self, forKey: .remoteToolBindingDigest
	) ?? ""
	guard validExecutionRoute(
	  harnessAdapter: harnessAdapter,
	  providerID: providerID,
	  providerAccountID: providerAccountID,
	  modelID: modelID,
	  authMode: authMode,
	  credentialRevision: credentialRevision,
	  reasoningEffort: reasoningEffort,
	  timeoutSeconds: timeoutSeconds,
	  budgetCredits: budgetCredits,
	  capabilities: capabilities,
	  status: status,
	  blockReason: blockReason
	) else { throw LocalProductWireError.invalidJSON }
	let completeFallback = validExecutionRoute(
	  harnessAdapter: fallbackHarnessAdapter,
	  providerID: fallbackProviderID,
	  providerAccountID: fallbackProviderAccountID,
	  modelID: fallbackModelID,
	  authMode: fallbackAuthMode,
	  credentialRevision: fallbackCredentialRevision,
	  reasoningEffort: fallbackReasoningEffort,
	  timeoutSeconds: fallbackTimeoutSeconds,
	  budgetCredits: fallbackBudgetCredits,
	  capabilities: fallbackCapabilities,
	  status: fallbackStatus,
	  blockReason: fallbackBlockReason
	) && fallbackApprovalRequired &&
	  (!fallbackApprovalAvailable || fallbackApprovalVersion > 0)
	let emptyFallback = fallbackHarnessAdapter.isEmpty &&
	  fallbackProviderID.isEmpty && fallbackProviderAccountID.isEmpty &&
	  fallbackModelID.isEmpty && fallbackAuthMode.isEmpty &&
	  fallbackCredentialRevision == 0 && fallbackReasoningEffort.isEmpty &&
	  fallbackTimeoutSeconds == 0 && fallbackBudgetCredits == nil &&
	  fallbackCapabilities.isEmpty && fallbackStatus.isEmpty &&
	  fallbackBlockReason.isEmpty && !fallbackApprovalRequired &&
	  !fallbackApprovalAvailable && fallbackApprovalVersion == 0
	guard (fallbackConfigured && completeFallback) ||
	  (!fallbackConfigured && emptyFallback) else {
	  throw LocalProductWireError.invalidJSON
	}
	guard fallbackApprovalAvailable || fallbackApprovalVersion == 0 else {
	  throw LocalProductWireError.invalidJSON
	}
	let validRouteTopology = kind.isEmpty
	  ? routeGroupID.isEmpty
	  : (kind == "route_sibling" || kind == "aggregation") &&
		LocalIPCClient.validIdentifier(routeGroupID) && !fallbackConfigured
	guard validRouteTopology,
	  kind != "aggregation" || dependsOn.count >= 2 else {
	  throw LocalProductWireError.invalidJSON
	}
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
