import Foundation

public enum LocalProductToolRecoveryOperation: String, Codable, Sendable {
  case preview
  case resolve
}

public enum LocalProductToolRecoveryAction: String, Codable, Sendable {
  case abortAttempt = "abort_attempt"
  case acceptObservedEffect = "accept_observed_effect"
  case retryInNewAttempt = "retry_in_new_attempt"
}

public enum LocalProductToolRecoveryCandidateStatus: String, Codable, Sendable {
  case available
  case resolved
}

public struct LocalProductToolRecoveryRequest: Codable, Equatable, Sendable {
  public let schemaVersion: Int
  public let operation: LocalProductToolRecoveryOperation
  public let decisionID: String
  public let principalID: String
  public let action: LocalProductToolRecoveryAction?
  public let candidateDigest: String

  enum CodingKeys: String, CodingKey {
    case schemaVersion = "schema_version"
    case operation
    case decisionID = "decision_id"
    case principalID = "principal_id"
    case action
    case candidateDigest = "candidate_digest"
  }

  public init(
    operation: LocalProductToolRecoveryOperation,
    decisionID: String,
    principalID: String,
    action: LocalProductToolRecoveryAction?,
    candidateDigest: String
  ) {
    schemaVersion = 1
    self.operation = operation
    self.decisionID = decisionID
    self.principalID = principalID
    self.action = action
    self.candidateDigest = candidateDigest
  }

  public static var preview: Self {
    Self(
      operation: .preview, decisionID: "", principalID: "", action: nil,
      candidateDigest: ""
    )
  }

  public static func resolve(
    decisionID: String,
    principalID: String,
    action: LocalProductToolRecoveryAction,
    candidateDigest: String
  ) -> Self {
    Self(
      operation: .resolve, decisionID: decisionID, principalID: principalID,
      action: action, candidateDigest: candidateDigest
    )
  }

  var isValid: Bool {
    guard schemaVersion == 1 else { return false }
    switch operation {
    case .preview:
      return decisionID.isEmpty && principalID.isEmpty && action == nil
        && candidateDigest.isEmpty
    case .resolve:
      return ToolRecoveryWireValidation.validDecisionID(decisionID)
        && ToolRecoveryWireValidation.validIdentifier(principalID, maximum: 128)
        && action != nil && ToolRecoveryWireValidation.validDigest(candidateDigest)
    }
  }
}

public struct LocalProductToolRecoveryCandidate: Codable, Equatable, Sendable {
  public let schemaVersion: Int
  public let status: LocalProductToolRecoveryCandidateStatus
  public let candidateDigest: String
  public let decisionID: String
  public let decision: LocalProductToolRecoveryAction?
  public let executionID: String
  public let jobID: String
  public let callDigest: String
  public let tool: String
  public let generation: Int64
  public let operationID: String
  public let incidentID: String
  public let recoveryCode: String
  public let recoveryRequiredAt: String
  public let availableActions: [LocalProductToolRecoveryAction]

  enum CodingKeys: String, CodingKey {
    case schemaVersion = "schema_version"
    case status
    case candidateDigest = "candidate_digest"
    case decisionID = "decision_id"
    case decision
    case executionID = "execution_id"
    case jobID = "job_id"
    case callDigest = "call_digest"
    case tool, generation
    case operationID = "operation_id"
    case incidentID = "incident_id"
    case recoveryCode = "recovery_code"
    case recoveryRequiredAt = "recovery_required_at"
    case availableActions = "available_actions"
  }

  public init(from decoder: Decoder) throws {
    try rejectUnknownKeys(
      decoder,
      allowed: [
        "schema_version", "status", "candidate_digest", "decision_id", "decision",
        "execution_id", "job_id", "call_digest", "tool", "generation",
        "operation_id", "incident_id", "recovery_code", "recovery_required_at",
        "available_actions",
      ])
    let values = try decoder.container(keyedBy: CodingKeys.self)
    schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
    status = try values.decode(LocalProductToolRecoveryCandidateStatus.self, forKey: .status)
    candidateDigest = try values.decode(String.self, forKey: .candidateDigest)
    decisionID = try values.decodeIfPresent(String.self, forKey: .decisionID) ?? ""
    decision = try values.decodeIfPresent(LocalProductToolRecoveryAction.self, forKey: .decision)
    executionID = try values.decode(String.self, forKey: .executionID)
    jobID = try values.decode(String.self, forKey: .jobID)
    callDigest = try values.decode(String.self, forKey: .callDigest)
    tool = try values.decode(String.self, forKey: .tool)
    generation = try values.decode(Int64.self, forKey: .generation)
    operationID = try values.decode(String.self, forKey: .operationID)
    incidentID = try values.decode(String.self, forKey: .incidentID)
    recoveryCode = try values.decode(String.self, forKey: .recoveryCode)
    recoveryRequiredAt = try values.decode(String.self, forKey: .recoveryRequiredAt)
    availableActions = try values.decodeIfPresent(
      [LocalProductToolRecoveryAction].self, forKey: .availableActions) ?? []
    let uniqueActions = Set(availableActions)
    let stateValid = status == .available
      ? decisionID.isEmpty && decision == nil && !availableActions.isEmpty
      : ToolRecoveryWireValidation.validDecisionID(decisionID)
        && decision != nil && availableActions.isEmpty
    guard schemaVersion == 1, stateValid,
      uniqueActions.count == availableActions.count,
      ToolRecoveryWireValidation.validDigest(candidateDigest),
      ToolRecoveryWireValidation.validIdentifier(executionID),
      ToolRecoveryWireValidation.validIdentifier(jobID),
      ToolRecoveryWireValidation.validDigest(callDigest),
      ToolRecoveryWireValidation.validIdentifier(tool, maximum: 64), generation > 0,
      ToolRecoveryWireValidation.validIdentifier(operationID),
      ToolRecoveryWireValidation.validIdentifier(incidentID),
      recoveryCode == "side_effect_unknown", !recoveryRequiredAt.isEmpty
    else { throw LocalProductWireError.invalidJSON }
  }
}

public struct LocalProductToolRecoveryDecision: Codable, Equatable, Sendable {
  public let schemaVersion: Int
  public let decisionID: String
  public let action: LocalProductToolRecoveryAction
  public let candidateDigest: String
  public let executionID: String
  public let evidenceID: String
  public let observationDigest: String
  public let outputDigest: String
  public let changedFilesDigest: String
  public let replacementAttemptID: String
  public let replacementRunID: String
  public let executionBindingDigest: String
  public let contextCapsuleDigest: String

  enum CodingKeys: String, CodingKey {
    case schemaVersion = "schema_version"
    case decisionID = "decision_id"
    case action
    case candidateDigest = "candidate_digest"
    case executionID = "execution_id"
    case evidenceID = "evidence_id"
    case observationDigest = "observation_digest"
    case outputDigest = "output_digest"
    case changedFilesDigest = "changed_files_digest"
    case replacementAttemptID = "replacement_attempt_id"
    case replacementRunID = "replacement_run_id"
    case executionBindingDigest = "execution_binding_digest"
    case contextCapsuleDigest = "context_capsule_digest"
  }

  public init(from decoder: Decoder) throws {
    try rejectUnknownKeys(
      decoder,
      allowed: [
        "schema_version", "decision_id", "action", "candidate_digest",
        "execution_id", "evidence_id", "observation_digest", "output_digest",
        "changed_files_digest", "replacement_attempt_id", "replacement_run_id",
        "execution_binding_digest", "context_capsule_digest",
      ])
    let values = try decoder.container(keyedBy: CodingKeys.self)
    schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
    decisionID = try values.decode(String.self, forKey: .decisionID)
    action = try values.decode(LocalProductToolRecoveryAction.self, forKey: .action)
    candidateDigest = try values.decode(String.self, forKey: .candidateDigest)
    executionID = try values.decode(String.self, forKey: .executionID)
    evidenceID = try values.decodeIfPresent(String.self, forKey: .evidenceID) ?? ""
    observationDigest = try values.decodeIfPresent(String.self, forKey: .observationDigest) ?? ""
    outputDigest = try values.decodeIfPresent(String.self, forKey: .outputDigest) ?? ""
    changedFilesDigest = try values.decodeIfPresent(String.self, forKey: .changedFilesDigest) ?? ""
    replacementAttemptID = try values.decodeIfPresent(String.self, forKey: .replacementAttemptID) ?? ""
    replacementRunID = try values.decodeIfPresent(String.self, forKey: .replacementRunID) ?? ""
    executionBindingDigest = try values.decodeIfPresent(String.self, forKey: .executionBindingDigest) ?? ""
    contextCapsuleDigest = try values.decodeIfPresent(String.self, forKey: .contextCapsuleDigest) ?? ""
    guard schemaVersion == 1,
      ToolRecoveryWireValidation.validDecisionID(decisionID),
      ToolRecoveryWireValidation.validDigest(candidateDigest),
      ToolRecoveryWireValidation.validIdentifier(executionID), validActionShape
    else { throw LocalProductWireError.invalidJSON }
  }

  private var validActionShape: Bool {
    switch action {
    case .abortAttempt:
      return evidenceID.isEmpty && observationDigest.isEmpty && outputDigest.isEmpty
        && changedFilesDigest.isEmpty && replacementAttemptID.isEmpty
        && replacementRunID.isEmpty && executionBindingDigest.isEmpty
        && contextCapsuleDigest.isEmpty
    case .acceptObservedEffect:
      return ToolRecoveryWireValidation.validIdentifier(evidenceID)
        && ToolRecoveryWireValidation.validDigest(observationDigest)
        && ToolRecoveryWireValidation.validSHA256Digest(outputDigest)
        && ToolRecoveryWireValidation.validSHA256Digest(changedFilesDigest)
        && replacementAttemptID.isEmpty && replacementRunID.isEmpty
        && executionBindingDigest.isEmpty && contextCapsuleDigest.isEmpty
    case .retryInNewAttempt:
      return evidenceID.isEmpty && observationDigest.isEmpty && outputDigest.isEmpty
        && changedFilesDigest.isEmpty
        && ToolRecoveryWireValidation.validIdentifier(replacementAttemptID)
        && ToolRecoveryWireValidation.validIdentifier(replacementRunID)
        && ToolRecoveryWireValidation.validDigest(executionBindingDigest)
        && ToolRecoveryWireValidation.validDigest(contextCapsuleDigest)
    }
  }
}

public struct LocalProductToolRecoveryResponse: Codable, Equatable, Sendable {
  public let schemaVersion: Int
  public let incidentID: String
  public let operation: LocalProductToolRecoveryOperation
  public let candidates: [LocalProductToolRecoveryCandidate]
  public let decision: LocalProductToolRecoveryDecision?

  enum CodingKeys: String, CodingKey {
    case schemaVersion = "schema_version"
    case incidentID = "incident_id"
    case operation, candidates, decision
  }

  public init(from decoder: Decoder) throws {
    try rejectUnknownKeys(
      decoder,
      allowed: ["schema_version", "incident_id", "operation", "candidates", "decision"]
    )
    let values = try decoder.container(keyedBy: CodingKeys.self)
    schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
    incidentID = try values.decode(String.self, forKey: .incidentID)
    operation = try values.decode(LocalProductToolRecoveryOperation.self, forKey: .operation)
    candidates = try values.decodeIfPresent(
      [LocalProductToolRecoveryCandidate].self, forKey: .candidates) ?? []
    decision = try values.decodeIfPresent(LocalProductToolRecoveryDecision.self, forKey: .decision)
    let shapeValid = operation == .preview
      ? decision == nil
      : candidates.isEmpty && decision != nil
    guard schemaVersion == 1,
      ToolRecoveryWireValidation.validIncidentID(incidentID), shapeValid
    else { throw LocalProductWireError.invalidJSON }
  }
}

public protocol LocalProductToolRecoveryClientProtocol: AnyObject {
  func recoverToolCall(
    _ request: LocalProductToolRecoveryRequest,
    incidentID: String
  ) async throws -> LocalProductToolRecoveryResponse
}

public enum LocalProductToolRecoveryWire {
  public static func decodeResponse(_ data: Data) throws -> LocalProductToolRecoveryResponse {
    do {
      try StrictJSONScanner.validate(data)
      return try JSONDecoder().decode(LocalProductToolRecoveryResponse.self, from: data)
    } catch let error as LocalProductWireError {
      throw error
    } catch {
      throw LocalProductWireError.invalidJSON
    }
  }
}

private enum ToolRecoveryWireValidation {
  static func validDigest(_ value: String) -> Bool {
    value.utf8.count == 64
      && value.allSatisfy { ("0"..."9").contains($0) || ("a"..."f").contains($0) }
  }

  static func validSHA256Digest(_ value: String) -> Bool {
    value.hasPrefix("sha256:") && validDigest(String(value.dropFirst(7)))
  }

  static func validIdentifier(_ value: String, maximum: Int = 256) -> Bool {
    !value.isEmpty && value.utf8.count <= maximum
      && value.unicodeScalars.allSatisfy {
        CharacterSet.alphanumerics.contains($0) || "-_.:/".unicodeScalars.contains($0)
      }
  }

  static func validDecisionID(_ value: String) -> Bool {
    guard let uuid = UUID(uuidString: value), uuid.uuidString.lowercased() == value,
      value.utf8.count == 36
    else { return false }
    let version = value[value.index(value.startIndex, offsetBy: 14)]
    let variant = value[value.index(value.startIndex, offsetBy: 19)]
    return version == "4" && "89ab".contains(variant)
  }

  static func validIncidentID(_ value: String) -> Bool {
    validIdentifier(value, maximum: 64)
  }
}
