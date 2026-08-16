import Foundation

public enum LocalProductAgentRecoveryOperation: String, Codable, Sendable {
  case preview
  case confirm
  case resume
}

public enum LocalProductAgentRecoveryCandidateStatus: String, Codable, Sendable {
  case available
  case authorized
  case consumed
}

public struct LocalProductAgentRecoveryRequest: Codable, Equatable, Sendable {
  public let schemaVersion: Int
  public let operation: LocalProductAgentRecoveryOperation
  public let decisionID: String
  public let principalID: String
  public let candidateDigest: String
  public let capabilityDigest: String

  enum CodingKeys: String, CodingKey {
    case schemaVersion = "schema_version"
    case operation
    case decisionID = "decision_id"
    case principalID = "principal_id"
    case candidateDigest = "candidate_digest"
    case capabilityDigest = "capability_digest"
  }

  public init(
    operation: LocalProductAgentRecoveryOperation,
    decisionID: String,
    principalID: String,
    candidateDigest: String,
    capabilityDigest: String
  ) {
    schemaVersion = 1
    self.operation = operation
    self.decisionID = decisionID
    self.principalID = principalID
    self.candidateDigest = candidateDigest
    self.capabilityDigest = capabilityDigest
  }

  public static var preview: Self {
    Self(
      operation: .preview, decisionID: "", principalID: "",
      candidateDigest: "", capabilityDigest: ""
    )
  }

  public static func confirm(
    decisionID: String,
    principalID: String,
    candidateDigest: String,
    capabilityDigest: String
  ) -> Self {
    Self(
      operation: .confirm, decisionID: decisionID, principalID: principalID,
      candidateDigest: candidateDigest, capabilityDigest: capabilityDigest
    )
  }

  public static func resume(
    decisionID: String,
    candidateDigest: String,
    capabilityDigest: String
  ) -> Self {
    Self(
      operation: .resume, decisionID: decisionID, principalID: "",
      candidateDigest: candidateDigest, capabilityDigest: capabilityDigest
    )
  }

  var isValid: Bool {
    guard schemaVersion == 1 else { return false }
    switch operation {
    case .preview:
      return decisionID.isEmpty && principalID.isEmpty
        && candidateDigest.isEmpty && capabilityDigest.isEmpty
    case .confirm:
      return Self.validDecisionID(decisionID) && Self.validIdentifier(principalID, maximum: 128)
        && Self.validDigest(candidateDigest) && Self.validDigest(capabilityDigest)
    case .resume:
      return Self.validDecisionID(decisionID) && principalID.isEmpty
        && Self.validDigest(candidateDigest) && Self.validDigest(capabilityDigest)
    }
  }

  fileprivate static func validDigest(_ value: String) -> Bool {
    value.utf8.count == 64 && value.allSatisfy { ("0"..."9").contains($0) || ("a"..."f").contains($0) }
  }

  fileprivate static func validIdentifier(_ value: String, maximum: Int = 256) -> Bool {
    !value.isEmpty && value.utf8.count <= maximum
      && value.unicodeScalars.allSatisfy {
        CharacterSet.alphanumerics.contains($0) || "-_.:/".unicodeScalars.contains($0)
      }
  }

  fileprivate static func validDecisionID(_ value: String) -> Bool {
    guard let uuid = UUID(uuidString: value), uuid.uuidString.lowercased() == value,
      value.utf8.count == 36
    else { return false }
    let version = value[value.index(value.startIndex, offsetBy: 14)]
    let variant = value[value.index(value.startIndex, offsetBy: 19)]
    return version == "4" && "89ab".contains(variant)
  }

  fileprivate static func validIncidentID(_ value: String) -> Bool {
    validIdentifier(value, maximum: 64)
  }
}

public struct LocalProductAgentRecoveryCandidate: Codable, Equatable, Sendable {
  public let schemaVersion: Int
  public let status: LocalProductAgentRecoveryCandidateStatus
  public let action: String
  public let decisionID: String
  public let candidateDigest: String
  public let capabilityDigest: String
  public let attemptID: String
  public let teamInstanceID: String
  public let segmentID: String
  public let workItemID: String
  public let runID: String
  public let claimGeneration: Int64
  public let runtimeInstanceID: String
  public let agentInstanceID: String
  public let harnessAdapter: String
  public let providerID: String
  public let providerAccountID: String
  public let modelID: String
  public let credentialRevision: Int64

  enum CodingKeys: String, CodingKey {
    case schemaVersion = "schema_version"
    case status, action
    case decisionID = "decision_id"
    case candidateDigest = "candidate_digest"
    case capabilityDigest = "capability_digest"
    case attemptID = "attempt_id"
    case teamInstanceID = "team_instance_id"
    case segmentID = "segment_id"
    case workItemID = "work_item_id"
    case runID = "run_id"
    case claimGeneration = "claim_generation"
    case runtimeInstanceID = "runtime_instance_id"
    case agentInstanceID = "agent_instance_id"
    case harnessAdapter = "harness_adapter"
    case providerID = "provider_id"
    case providerAccountID = "provider_account_id"
    case modelID = "model_id"
    case credentialRevision = "credential_revision"
  }

  public init(from decoder: Decoder) throws {
    try rejectUnknownKeys(
      decoder,
      allowed: [
        "schema_version", "status", "action", "decision_id", "candidate_digest",
        "capability_digest", "attempt_id", "team_instance_id", "segment_id",
        "work_item_id", "run_id", "claim_generation", "runtime_instance_id",
        "agent_instance_id", "harness_adapter", "provider_id",
        "provider_account_id", "model_id", "credential_revision",
      ])
    let values = try decoder.container(keyedBy: CodingKeys.self)
    schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
    status = try values.decode(LocalProductAgentRecoveryCandidateStatus.self, forKey: .status)
    action = try values.decode(String.self, forKey: .action)
    decisionID = try values.decodeIfPresent(String.self, forKey: .decisionID) ?? ""
    candidateDigest = try values.decode(String.self, forKey: .candidateDigest)
    capabilityDigest = try values.decode(String.self, forKey: .capabilityDigest)
    attemptID = try values.decode(String.self, forKey: .attemptID)
    teamInstanceID = try values.decode(String.self, forKey: .teamInstanceID)
    segmentID = try values.decode(String.self, forKey: .segmentID)
    workItemID = try values.decode(String.self, forKey: .workItemID)
    runID = try values.decode(String.self, forKey: .runID)
    claimGeneration = try values.decode(Int64.self, forKey: .claimGeneration)
    runtimeInstanceID = try values.decode(String.self, forKey: .runtimeInstanceID)
    agentInstanceID = try values.decode(String.self, forKey: .agentInstanceID)
    harnessAdapter = try values.decode(String.self, forKey: .harnessAdapter)
    providerID = try values.decode(String.self, forKey: .providerID)
    providerAccountID = try values.decode(String.self, forKey: .providerAccountID)
    modelID = try values.decode(String.self, forKey: .modelID)
    credentialRevision = try values.decode(Int64.self, forKey: .credentialRevision)
    let decisionStateValid = status == .available ? decisionID.isEmpty
      : LocalProductAgentRecoveryRequest.validDecisionID(decisionID)
    guard schemaVersion == 1, action == "resume_pre_model", decisionStateValid,
      LocalProductAgentRecoveryRequest.validDigest(candidateDigest),
      LocalProductAgentRecoveryRequest.validDigest(capabilityDigest),
      LocalProductAgentRecoveryRequest.validIdentifier(attemptID),
      LocalProductAgentRecoveryRequest.validIdentifier(teamInstanceID),
      LocalProductAgentRecoveryRequest.validIdentifier(segmentID),
      LocalProductAgentRecoveryRequest.validIdentifier(workItemID),
      LocalProductAgentRecoveryRequest.validIdentifier(runID), claimGeneration > 0,
      LocalProductAgentRecoveryRequest.validIdentifier(runtimeInstanceID),
      LocalProductAgentRecoveryRequest.validIdentifier(agentInstanceID),
      LocalProductAgentRecoveryRequest.validIdentifier(harnessAdapter),
      LocalProductAgentRecoveryRequest.validIdentifier(providerID, maximum: 64),
      LocalProductAgentRecoveryRequest.validIdentifier(providerAccountID, maximum: 128),
      LocalProductAgentRecoveryRequest.validIdentifier(modelID), credentialRevision > 0
    else { throw LocalProductWireError.invalidJSON }
  }
}

public struct LocalProductAgentRecoveryDecision: Codable, Equatable, Sendable {
  public let schemaVersion: Int
  public let decisionID: String
  public let action: String
  public let candidateDigest: String
  public let capabilityDigest: String

  enum CodingKeys: String, CodingKey {
    case schemaVersion = "schema_version"
    case decisionID = "decision_id"
    case action
    case candidateDigest = "candidate_digest"
    case capabilityDigest = "capability_digest"
  }

  public init(from decoder: Decoder) throws {
    try rejectUnknownKeys(
      decoder,
      allowed: [
        "schema_version", "decision_id", "action", "candidate_digest",
        "capability_digest",
      ])
    let values = try decoder.container(keyedBy: CodingKeys.self)
    schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
    decisionID = try values.decode(String.self, forKey: .decisionID)
    action = try values.decode(String.self, forKey: .action)
    candidateDigest = try values.decode(String.self, forKey: .candidateDigest)
    capabilityDigest = try values.decode(String.self, forKey: .capabilityDigest)
    guard schemaVersion == 1,
      LocalProductAgentRecoveryRequest.validDecisionID(decisionID),
      action == "resume_pre_model",
      LocalProductAgentRecoveryRequest.validDigest(candidateDigest),
      LocalProductAgentRecoveryRequest.validDigest(capabilityDigest)
    else { throw LocalProductWireError.invalidJSON }
  }
}

public struct LocalProductAgentRecoveryResume: Codable, Equatable, Sendable {
  public let schemaVersion: Int
  public let status: String
  public let decisionID: String
  public let candidateDigest: String
  public let capabilityDigest: String
  public let attemptID: String
  public let runtimeInstanceID: String

  enum CodingKeys: String, CodingKey {
    case schemaVersion = "schema_version"
    case status
    case decisionID = "decision_id"
    case candidateDigest = "candidate_digest"
    case capabilityDigest = "capability_digest"
    case attemptID = "attempt_id"
    case runtimeInstanceID = "runtime_instance_id"
  }

  public init(from decoder: Decoder) throws {
    try rejectUnknownKeys(
      decoder,
      allowed: [
        "schema_version", "status", "decision_id", "candidate_digest",
        "capability_digest", "attempt_id", "runtime_instance_id",
      ])
    let values = try decoder.container(keyedBy: CodingKeys.self)
    schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
    status = try values.decode(String.self, forKey: .status)
    decisionID = try values.decode(String.self, forKey: .decisionID)
    candidateDigest = try values.decode(String.self, forKey: .candidateDigest)
    capabilityDigest = try values.decode(String.self, forKey: .capabilityDigest)
    attemptID = try values.decode(String.self, forKey: .attemptID)
    runtimeInstanceID = try values.decode(String.self, forKey: .runtimeInstanceID)
    guard schemaVersion == 1, status == "completed",
      LocalProductAgentRecoveryRequest.validDecisionID(decisionID),
      LocalProductAgentRecoveryRequest.validDigest(candidateDigest),
      LocalProductAgentRecoveryRequest.validDigest(capabilityDigest),
      LocalProductAgentRecoveryRequest.validIdentifier(attemptID),
      LocalProductAgentRecoveryRequest.validIdentifier(runtimeInstanceID)
    else { throw LocalProductWireError.invalidJSON }
  }
}

public struct LocalProductAgentRecoveryResponse: Codable, Equatable, Sendable {
  public let schemaVersion: Int
  public let incidentID: String
  public let operation: LocalProductAgentRecoveryOperation
  public let candidates: [LocalProductAgentRecoveryCandidate]
  public let decision: LocalProductAgentRecoveryDecision?
  public let resume: LocalProductAgentRecoveryResume?

  enum CodingKeys: String, CodingKey {
    case schemaVersion = "schema_version"
    case incidentID = "incident_id"
    case operation, candidates, decision, resume
  }

  public init(from decoder: Decoder) throws {
    try rejectUnknownKeys(
      decoder,
      allowed: ["schema_version", "incident_id", "operation", "candidates", "decision", "resume"]
    )
    let values = try decoder.container(keyedBy: CodingKeys.self)
    schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
    incidentID = try values.decode(String.self, forKey: .incidentID)
    operation = try values.decode(LocalProductAgentRecoveryOperation.self, forKey: .operation)
    candidates = try values.decodeIfPresent(
      [LocalProductAgentRecoveryCandidate].self, forKey: .candidates) ?? []
    decision = try values.decodeIfPresent(LocalProductAgentRecoveryDecision.self, forKey: .decision)
    resume = try values.decodeIfPresent(LocalProductAgentRecoveryResume.self, forKey: .resume)
    let shapeValid: Bool
    switch operation {
    case .preview:
      shapeValid = decision == nil && resume == nil
    case .confirm:
      shapeValid = candidates.isEmpty && decision != nil && resume == nil
    case .resume:
      shapeValid = candidates.isEmpty && decision == nil && resume != nil
    }
    guard schemaVersion == 1,
      LocalProductAgentRecoveryRequest.validIncidentID(incidentID), shapeValid
    else { throw LocalProductWireError.invalidJSON }
  }
}

public protocol LocalProductAgentRecoveryClientProtocol: AnyObject {
  func recoverAgentAttempt(
    _ request: LocalProductAgentRecoveryRequest,
    incidentID: String
  ) async throws -> LocalProductAgentRecoveryResponse
}

public enum LocalProductAgentRecoveryWire {
  public static func decodeResponse(_ data: Data) throws -> LocalProductAgentRecoveryResponse {
    do {
      try StrictJSONScanner.validate(data)
      return try JSONDecoder().decode(LocalProductAgentRecoveryResponse.self, from: data)
    } catch let error as LocalProductWireError {
      throw error
    } catch {
      throw LocalProductWireError.invalidJSON
    }
  }
}
