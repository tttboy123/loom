import Foundation

public enum LocalProductAgentInputMode: String, CaseIterable, Codable, Sendable, Identifiable {
  case queue
  case steer
  case inject

  public var id: String { rawValue }
}

public struct LocalProductAgentInputRequest: Codable, Equatable, Sendable {
  public let schemaVersion: Int
  public let segmentID: String
  public let agentInstanceID: String
  public let workItemID: String
  public let runID: String
  public let claimGeneration: Int64
  public let mode: LocalProductAgentInputMode
  public let contextScope: String
  public let scopeTargetID: String
  public let content: Data

  enum CodingKeys: String, CodingKey {
    case schemaVersion = "schema_version"
    case segmentID = "segment_id"
    case agentInstanceID = "agent_instance_id"
    case workItemID = "work_item_id"
    case runID = "run_id"
    case claimGeneration = "claim_generation"
    case mode
    case contextScope = "context_scope"
    case scopeTargetID = "scope_target_id"
    case content
  }

  public init(
    segmentID: String,
    agentInstanceID: String,
    workItemID: String,
    runID: String,
    claimGeneration: Int64,
    mode: LocalProductAgentInputMode,
    contextScope: String = "agent_private",
    scopeTargetID: String = "",
    content: Data
  ) {
    schemaVersion = 1
    self.segmentID = segmentID
    self.agentInstanceID = agentInstanceID
    self.workItemID = workItemID
    self.runID = runID
    self.claimGeneration = claimGeneration
    self.mode = mode
    self.contextScope = contextScope
    self.scopeTargetID = scopeTargetID
    self.content = content
  }

  var isValid: Bool {
    schemaVersion == 1
      && !segmentID.isEmpty && !agentInstanceID.isEmpty
      && !workItemID.isEmpty && !runID.isEmpty
      && claimGeneration > 0
      && contextScope == "agent_private" && scopeTargetID.isEmpty
      && (1...32_768).contains(content.count)
      && String(data: content, encoding: .utf8) != nil
      && !content.contains(0)
  }
}

public struct LocalProductAgentInputReceipt: Codable, Equatable, Sendable {
  public let schemaVersion: Int
  public let incidentID: String
  public let inputID: String
  public let mode: LocalProductAgentInputMode
  public let orderKey: Int64
  public let targetTurnID: String
  public let targetTurnSequence: Int
  public let targetStepID: String
  public let targetStepSequence: Int

  enum CodingKeys: String, CodingKey {
    case schemaVersion = "schema_version"
    case incidentID = "incident_id"
    case inputID = "input_id"
    case mode
    case orderKey = "order_key"
    case targetTurnID = "target_turn_id"
    case targetTurnSequence = "target_turn_sequence"
    case targetStepID = "target_step_id"
    case targetStepSequence = "target_step_sequence"
  }

  public init(from decoder: Decoder) throws {
    try rejectUnknownKeys(
      decoder,
      allowed: [
        "schema_version", "incident_id", "input_id", "mode", "order_key",
        "target_turn_id", "target_turn_sequence", "target_step_id",
        "target_step_sequence",
      ])
    let values = try decoder.container(keyedBy: CodingKeys.self)
    schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
    incidentID = try values.decode(String.self, forKey: .incidentID)
    inputID = try values.decode(String.self, forKey: .inputID)
    mode = try values.decode(LocalProductAgentInputMode.self, forKey: .mode)
    orderKey = try values.decode(Int64.self, forKey: .orderKey)
    targetTurnID = try values.decodeIfPresent(String.self, forKey: .targetTurnID) ?? ""
    targetTurnSequence = try values.decodeIfPresent(Int.self, forKey: .targetTurnSequence) ?? 0
    targetStepID = try values.decodeIfPresent(String.self, forKey: .targetStepID) ?? ""
    targetStepSequence = try values.decodeIfPresent(Int.self, forKey: .targetStepSequence) ?? 0
    let queueTarget = mode == .queue
      && !targetTurnID.isEmpty && targetTurnSequence > 0
      && targetStepID.isEmpty && targetStepSequence == 0
    let stepTarget = mode != .queue
      && targetTurnID.isEmpty && targetTurnSequence == 0
      && !targetStepID.isEmpty && targetStepSequence > 0
    guard schemaVersion == 1, !incidentID.isEmpty, !inputID.isEmpty,
      orderKey > 0, queueTarget || stepTarget
    else {
      throw LocalProductWireError.invalidJSON
    }
  }
}

public protocol LocalProductAgentInputClientProtocol: AnyObject {
  func sendAgentInput(
    _ request: LocalProductAgentInputRequest,
    incidentID: String
  ) async throws -> LocalProductAgentInputReceipt
}

public enum LocalProductAgentInputWire {
  public static func decodeReceipt(_ data: Data) throws -> LocalProductAgentInputReceipt {
    do {
      try StrictJSONScanner.validate(data)
      return try JSONDecoder().decode(LocalProductAgentInputReceipt.self, from: data)
    } catch let error as LocalProductWireError {
      throw error
    } catch {
      throw LocalProductWireError.invalidJSON
    }
  }
}
