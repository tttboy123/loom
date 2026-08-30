import Foundation

// Governed Handoff / Roundtable wire models. The daemon stamps EmittedAt
// server-side, so requests carry only client-authored correlation IDs and
// identities. The response is the full digest-bound session view.

public struct LocalRoundtableSessionContext: Codable, Equatable, Sendable {
    public let conversationID: String
    public let missionID: String
    public let teamID: String
    public let teamVersion: Int
    public let workspaceID: String

    enum CodingKeys: String, CodingKey {
        case conversationID = "conversation_id"
        case missionID = "mission_id"
        case teamID = "team_id"
        case teamVersion = "team_version"
        case workspaceID = "workspace_id"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(decoder, allowed: [
            "conversation_id", "mission_id", "team_id", "team_version", "workspace_id",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        conversationID = try values.decode(String.self, forKey: .conversationID)
        missionID = try values.decode(String.self, forKey: .missionID)
        teamID = try values.decode(String.self, forKey: .teamID)
        teamVersion = try values.decode(Int.self, forKey: .teamVersion)
        workspaceID = try values.decode(String.self, forKey: .workspaceID)
        guard !conversationID.isEmpty, !missionID.isEmpty, !teamID.isEmpty,
              teamVersion > 0, !workspaceID.isEmpty else {
            throw LocalProductWireError.invalidJSON
        }
    }
}

public struct LocalRoundtableSeatExecutionBinding: Codable, Equatable, Sendable {
    public let profileID: String
    public let harnessAdapter: String
    public let runtimeInstanceID: String
    public let providerID: String
    public let providerAccountID: String
    public let modelID: String
    public let authMode: String
    public let endpointFingerprint: String
    public let credentialReference: String
    public let credentialRevision: Int64
    public let reasoningEffort: String
    public let timeoutNanoseconds: Int64
    public let budget: Int64?
    public let capabilities: [String]
    public let remoteToolEnrollmentID: String
    public let remoteToolEnrollmentDigest: String
    public let bindingDigest: String

    enum CodingKeys: String, CodingKey {
        case profileID = "profile_id"
        case harnessAdapter = "harness_adapter"
        case runtimeInstanceID = "runtime_instance_id"
        case providerID = "provider_id"
        case providerAccountID = "provider_account_id"
        case modelID = "model_id"
        case authMode = "auth_mode"
        case endpointFingerprint = "endpoint_fingerprint"
        case credentialReference = "credential_reference"
        case credentialRevision = "credential_revision"
        case reasoningEffort = "reasoning_effort"
        case timeoutNanoseconds = "timeout_nanoseconds"
        case budget, capabilities
        case remoteToolEnrollmentID = "remote_tool_enrollment_id"
        case remoteToolEnrollmentDigest = "remote_tool_enrollment_digest"
        case bindingDigest = "binding_digest"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(decoder, allowed: [
            "profile_id", "harness_adapter", "runtime_instance_id", "provider_id",
            "provider_account_id", "model_id", "auth_mode", "endpoint_fingerprint",
            "credential_reference", "credential_revision", "reasoning_effort",
            "timeout_nanoseconds", "budget", "capabilities",
            "remote_tool_enrollment_id", "remote_tool_enrollment_digest", "binding_digest",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        profileID = try values.decode(String.self, forKey: .profileID)
        harnessAdapter = try values.decode(String.self, forKey: .harnessAdapter)
        runtimeInstanceID = try values.decode(String.self, forKey: .runtimeInstanceID)
        providerID = try values.decode(String.self, forKey: .providerID)
        providerAccountID = try values.decode(String.self, forKey: .providerAccountID)
        modelID = try values.decode(String.self, forKey: .modelID)
        authMode = try values.decode(String.self, forKey: .authMode)
        endpointFingerprint = try values.decode(String.self, forKey: .endpointFingerprint)
        credentialReference = try values.decode(String.self, forKey: .credentialReference)
        credentialRevision = try values.decode(Int64.self, forKey: .credentialRevision)
        reasoningEffort = try values.decode(String.self, forKey: .reasoningEffort)
        timeoutNanoseconds = try values.decode(Int64.self, forKey: .timeoutNanoseconds)
        budget = try values.decodeIfPresent(Int64.self, forKey: .budget)
        capabilities = try values.decode([String].self, forKey: .capabilities)
        remoteToolEnrollmentID = try values.decode(String.self, forKey: .remoteToolEnrollmentID)
        remoteToolEnrollmentDigest = try values.decode(
            String.self, forKey: .remoteToolEnrollmentDigest
        )
        bindingDigest = try values.decode(String.self, forKey: .bindingDigest)
        let remotePairValid = remoteToolEnrollmentID.isEmpty
            ? remoteToolEnrollmentDigest.isEmpty
            : remoteToolEnrollmentDigest.count == 64
        guard !profileID.isEmpty, !harnessAdapter.isEmpty, !runtimeInstanceID.isEmpty,
              !providerID.isEmpty, !modelID.isEmpty, timeoutNanoseconds > 0,
              bindingDigest.count == 64,
              Set(capabilities).count == capabilities.count,
              budget.map({ $0 >= 0 }) ?? true,
              remotePairValid else {
            throw LocalProductWireError.invalidJSON
        }
    }
}

public struct LocalRoundtableFrozenSeatBinding: Codable, Equatable, Sendable {
    public let agentDefinitionID: String
    public let teamRoleKind: String
    public let runtimeProfileID: String
    public let executionBinding: LocalRoundtableSeatExecutionBinding
    public let membershipRevision: Int
    public let bindingDigest: String

    enum CodingKeys: String, CodingKey {
        case agentDefinitionID = "agent_definition_id"
        case teamRoleKind = "team_role_kind"
        case runtimeProfileID = "runtime_profile_id"
        case executionBinding = "execution_binding"
        case membershipRevision = "membership_revision"
        case bindingDigest = "binding_digest"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(decoder, allowed: [
            "agent_definition_id", "team_role_kind", "runtime_profile_id",
            "execution_binding", "membership_revision", "binding_digest",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        agentDefinitionID = try values.decode(String.self, forKey: .agentDefinitionID)
        teamRoleKind = try values.decode(String.self, forKey: .teamRoleKind)
        runtimeProfileID = try values.decode(String.self, forKey: .runtimeProfileID)
        executionBinding = try values.decode(
            LocalRoundtableSeatExecutionBinding.self, forKey: .executionBinding
        )
        membershipRevision = try values.decode(Int.self, forKey: .membershipRevision)
        bindingDigest = try values.decode(String.self, forKey: .bindingDigest)
        guard !agentDefinitionID.isEmpty,
              ["main", "subagent"].contains(teamRoleKind),
              runtimeProfileID == executionBinding.profileID,
              membershipRevision > 0, bindingDigest.count == 64 else {
            throw LocalProductWireError.invalidJSON
        }
    }
}

public struct LocalRoundtableSeat: Codable, Equatable, Sendable {
    public let id: String
    public let displayName: String
    public let available: Bool
    public let binding: LocalRoundtableFrozenSeatBinding?

    enum CodingKeys: String, CodingKey {
        case id
        case displayName = "display_name"
        case available
        case binding
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(
            decoder, allowed: ["id", "display_name", "available", "binding"]
        )
        let values = try decoder.container(keyedBy: CodingKeys.self)
        id = try values.decode(String.self, forKey: .id)
        displayName = try values.decode(String.self, forKey: .displayName)
        available = try values.decode(Bool.self, forKey: .available)
        binding = try values.decodeIfPresent(
            LocalRoundtableFrozenSeatBinding.self, forKey: .binding
        )
        guard !id.isEmpty, !displayName.isEmpty else {
            throw LocalProductWireError.invalidJSON
        }
    }
}

public struct LocalRoundtableMessage: Codable, Equatable, Sendable {
    public let id: String
    public let roundID: String
    public let writerSeat: String
    public let targetSeat: String
    public let body: String
    public let artifactRefs: [String]
    public let bodyDigest: String
    public let status: String
    public let proposedAt: String
    public let relayedAt: String
    public let acknowledgedAt: String

    enum CodingKeys: String, CodingKey {
        case id
        case roundID = "round_id"
        case writerSeat = "writer_seat"
        case targetSeat = "target_seat"
        case body
        case artifactRefs = "artifact_refs"
        case bodyDigest = "body_digest"
        case status
        case proposedAt = "proposed_at"
        case relayedAt = "relayed_at"
        case acknowledgedAt = "acknowledged_at"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(
            decoder,
            allowed: [
                "id", "round_id", "writer_seat", "target_seat", "body",
                "artifact_refs", "body_digest", "status", "proposed_at",
                "relayed_at", "acknowledged_at",
            ]
        )
        let values = try decoder.container(keyedBy: CodingKeys.self)
        id = try values.decode(String.self, forKey: .id)
        roundID = try values.decode(String.self, forKey: .roundID)
        writerSeat = try values.decode(String.self, forKey: .writerSeat)
        targetSeat = try values.decode(String.self, forKey: .targetSeat)
        body = try values.decode(String.self, forKey: .body)
        artifactRefs = try values.decode([String].self, forKey: .artifactRefs)
        bodyDigest = try values.decode(String.self, forKey: .bodyDigest)
        status = try values.decode(String.self, forKey: .status)
        proposedAt = try values.decode(String.self, forKey: .proposedAt)
        relayedAt = try values.decode(String.self, forKey: .relayedAt)
        acknowledgedAt = try values.decode(String.self, forKey: .acknowledgedAt)
        let statuses: Set<String> = ["pending", "relayed", "acknowledged", "inserted", "dropped"]
        guard !id.isEmpty, !roundID.isEmpty, !writerSeat.isEmpty, !targetSeat.isEmpty,
              bodyDigest.count == 64, statuses.contains(status),
              artifactRefs.allSatisfy({ $0.count == 64 }) else {
            throw LocalProductWireError.invalidJSON
        }
    }
}

public struct LocalRoundtableRound: Codable, Equatable, Sendable {
    public let id: String
    public let sequence: Int
    public let messageCount: Int
    public let messages: [LocalRoundtableMessage]
    public let pauseRequested: Bool

    enum CodingKeys: String, CodingKey {
        case id
        case sequence
        case messageCount = "message_count"
        case messages
        case pauseRequested = "pause_requested"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(
            decoder,
            allowed: ["id", "sequence", "message_count", "messages", "pause_requested"]
        )
        let values = try decoder.container(keyedBy: CodingKeys.self)
        id = try values.decode(String.self, forKey: .id)
        sequence = try values.decode(Int.self, forKey: .sequence)
        messageCount = try values.decode(Int.self, forKey: .messageCount)
        messages = try values.decode([LocalRoundtableMessage].self, forKey: .messages)
        pauseRequested = try values.decodeIfPresent(Bool.self, forKey: .pauseRequested) ?? false
        guard !id.isEmpty, sequence > 0, messageCount == messages.count else {
            throw LocalProductWireError.invalidJSON
        }
    }
}

public struct LocalRoundtableSession: Codable, Equatable, Sendable {
    public let id: String
    public let moderatorSeat: String
    public let title: String
    public let createdAt: String
    public let concluded: Bool
    public let context: LocalRoundtableSessionContext?

    enum CodingKeys: String, CodingKey {
        case id
        case moderatorSeat = "moderator_seat"
        case title
        case createdAt = "created_at"
        case concluded
        case context
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(
            decoder,
            allowed: [
                "id", "moderator_seat", "title", "created_at", "concluded", "context",
            ]
        )
        let values = try decoder.container(keyedBy: CodingKeys.self)
        id = try values.decode(String.self, forKey: .id)
        moderatorSeat = try values.decode(String.self, forKey: .moderatorSeat)
        title = try values.decode(String.self, forKey: .title)
        createdAt = try values.decode(String.self, forKey: .createdAt)
        concluded = try values.decode(Bool.self, forKey: .concluded)
        context = try values.decodeIfPresent(
            LocalRoundtableSessionContext.self, forKey: .context
        )
        guard !id.isEmpty, !moderatorSeat.isEmpty, !title.isEmpty,
              !createdAt.isEmpty else {
            throw LocalProductWireError.invalidJSON
        }
    }
}

public struct LocalRoundtableSeatAttempt: Codable, Equatable, Sendable {
    public let attemptID: String
    public let roundID: String
    public let seatID: String
    public let attemptNumber: Int
    public let executionTeamID: String
    public let workItemID: String
    public let runID: String
    public let segmentID: String
    public let claimGeneration: Int64
    public let runtimeInstanceID: String
    public let agentInstanceID: String
    public let membershipRevision: Int
    public let seatBindingDigest: String
    public let executionBindingDigest: String
    public let contextCapsuleDigest: String
    public let payloadReference: String
    public let status: String
    public let outputDigest: String
    public let incidentID: String
    public let failureCode: String
    public let failureStage: String
    public let retryable: Bool
    public let startedAt: String
    public let completedAt: String

    enum CodingKeys: String, CodingKey {
        case attemptID = "attempt_id"
        case roundID = "round_id"
        case seatID = "seat_id"
        case attemptNumber = "attempt_number"
        case executionTeamID = "execution_team_id"
        case workItemID = "work_item_id"
        case runID = "run_id"
        case segmentID = "segment_id"
        case claimGeneration = "claim_generation"
        case runtimeInstanceID = "runtime_instance_id"
        case agentInstanceID = "agent_instance_id"
        case membershipRevision = "membership_revision"
        case seatBindingDigest = "seat_binding_digest"
        case executionBindingDigest = "execution_binding_digest"
        case contextCapsuleDigest = "context_capsule_digest"
        case payloadReference = "payload_reference"
        case status, retryable
        case outputDigest = "output_digest"
        case incidentID = "incident_id"
        case failureCode = "failure_code"
        case failureStage = "failure_stage"
        case startedAt = "started_at"
        case completedAt = "completed_at"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(decoder, allowed: [
            "attempt_id", "round_id", "seat_id", "attempt_number",
            "execution_team_id", "work_item_id", "run_id", "segment_id", "claim_generation",
            "runtime_instance_id", "agent_instance_id",
            "membership_revision", "seat_binding_digest", "context_capsule_digest",
            "execution_binding_digest",
            "payload_reference", "status", "output_digest", "incident_id",
            "failure_code", "failure_stage", "retryable", "started_at", "completed_at",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        attemptID = try values.decode(String.self, forKey: .attemptID)
        roundID = try values.decode(String.self, forKey: .roundID)
        seatID = try values.decode(String.self, forKey: .seatID)
        attemptNumber = try values.decode(Int.self, forKey: .attemptNumber)
        executionTeamID = try values.decode(String.self, forKey: .executionTeamID)
        workItemID = try values.decode(String.self, forKey: .workItemID)
        runID = try values.decode(String.self, forKey: .runID)
        segmentID = try values.decode(String.self, forKey: .segmentID)
        claimGeneration = try values.decode(Int64.self, forKey: .claimGeneration)
        runtimeInstanceID = try values.decode(String.self, forKey: .runtimeInstanceID)
        agentInstanceID = try values.decode(String.self, forKey: .agentInstanceID)
        membershipRevision = try values.decode(Int.self, forKey: .membershipRevision)
        seatBindingDigest = try values.decode(String.self, forKey: .seatBindingDigest)
        executionBindingDigest = try values.decode(String.self, forKey: .executionBindingDigest)
        contextCapsuleDigest = try values.decode(String.self, forKey: .contextCapsuleDigest)
        payloadReference = try values.decode(String.self, forKey: .payloadReference)
        status = try values.decode(String.self, forKey: .status)
        outputDigest = try values.decode(String.self, forKey: .outputDigest)
        incidentID = try values.decode(String.self, forKey: .incidentID)
        failureCode = try values.decode(String.self, forKey: .failureCode)
        failureStage = try values.decode(String.self, forKey: .failureStage)
        retryable = try values.decode(Bool.self, forKey: .retryable)
        startedAt = try values.decode(String.self, forKey: .startedAt)
        completedAt = try values.decode(String.self, forKey: .completedAt)
        let common = !attemptID.isEmpty && !roundID.isEmpty && !seatID.isEmpty &&
            attemptNumber > 0 && !executionTeamID.isEmpty && !workItemID.isEmpty &&
            !runID.isEmpty && !segmentID.isEmpty && claimGeneration > 0 && !runtimeInstanceID.isEmpty &&
            !agentInstanceID.isEmpty && membershipRevision > 0 &&
            seatBindingDigest.count == 64 && executionBindingDigest.count == 64 &&
            contextCapsuleDigest.count == 64 &&
            !startedAt.isEmpty
        let terminalValid: Bool
        switch status {
        case "running":
            terminalValid = payloadReference.isEmpty && outputDigest.isEmpty &&
                incidentID.isEmpty && failureCode.isEmpty && failureStage.isEmpty && !retryable
        case "succeeded":
            terminalValid = !payloadReference.isEmpty && outputDigest.count == 64 &&
                incidentID.isEmpty && failureCode.isEmpty && failureStage.isEmpty && !retryable
        case "failed":
            terminalValid = payloadReference.isEmpty && outputDigest.isEmpty &&
                !incidentID.isEmpty && !failureCode.isEmpty && !failureStage.isEmpty
        case "cancelled":
            terminalValid = payloadReference.isEmpty && outputDigest.isEmpty &&
                incidentID.isEmpty && failureCode.isEmpty && failureStage.isEmpty && !retryable
        default:
            terminalValid = false
        }
        guard common, terminalValid else {
            throw LocalProductWireError.invalidJSON
        }
    }
}

public struct LocalRoundtableIntervention: Codable, Equatable, Sendable {
    public let id: String
    public let kind: String
    public let roundID: String
    public let moderatorSeat: String
    public let seatID: String
    public let attemptID: String
    public let inputID: String
    public let requestedAttemptNumber: Int
    public let incidentID: String
    public let failureCode: String
    public let failureStage: String
    public let retryable: Bool
    public let requestedAt: String
    public let digest: String

    enum CodingKeys: String, CodingKey {
        case id, kind
        case roundID = "round_id"
        case moderatorSeat = "moderator_seat"
        case seatID = "seat_id"
        case attemptID = "attempt_id"
        case inputID = "input_id"
        case requestedAttemptNumber = "requested_attempt_number"
        case incidentID = "incident_id"
        case failureCode = "failure_code"
        case failureStage = "failure_stage"
        case retryable
        case requestedAt = "requested_at"
        case digest
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(decoder, allowed: [
            "id", "kind", "round_id", "moderator_seat", "seat_id", "attempt_id",
            "input_id", "content_digest", "requested_attempt_number",
            "previous_membership_revision", "membership_revision",
            "previous_binding_digest", "seat_binding_digest", "incident_id",
            "failure_code", "failure_stage", "retryable", "requested_at", "digest",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        id = try values.decode(String.self, forKey: .id)
        kind = try values.decode(String.self, forKey: .kind)
        roundID = try values.decode(String.self, forKey: .roundID)
        moderatorSeat = try values.decode(String.self, forKey: .moderatorSeat)
        seatID = try values.decodeIfPresent(String.self, forKey: .seatID) ?? ""
        attemptID = try values.decodeIfPresent(String.self, forKey: .attemptID) ?? ""
        inputID = try values.decodeIfPresent(String.self, forKey: .inputID) ?? ""
        requestedAttemptNumber = try values.decodeIfPresent(Int.self, forKey: .requestedAttemptNumber) ?? 0
        incidentID = try values.decodeIfPresent(String.self, forKey: .incidentID) ?? ""
        failureCode = try values.decodeIfPresent(String.self, forKey: .failureCode) ?? ""
        failureStage = try values.decodeIfPresent(String.self, forKey: .failureStage) ?? ""
        retryable = try values.decodeIfPresent(Bool.self, forKey: .retryable) ?? false
        requestedAt = try values.decode(String.self, forKey: .requestedAt)
        digest = try values.decode(String.self, forKey: .digest)
        let kinds: Set<String> = [
            "pause_round", "steer", "retry_seat", "skip_seat", "replace_seat",
            "cancel_seat_attempt", "round_dispatch_failure",
        ]
        guard !id.isEmpty, kinds.contains(kind), !roundID.isEmpty,
              !requestedAt.isEmpty, digest.count == 64,
              kind == "cancel_seat_attempt" ? moderatorSeat.isEmpty : !moderatorSeat.isEmpty else {
            throw LocalProductWireError.invalidJSON
        }
    }
}

public struct LocalRoundtableSeatDelivery: Codable, Equatable, Sendable {
    public let attemptID: String
    public let seatID: String
    public let status: String
    public let body: String
    public let agentInputCapability: String
    public let updatedAt: String

    enum CodingKeys: String, CodingKey {
        case attemptID = "attempt_id"
        case seatID = "seat_id"
        case status, body
        case agentInputCapability = "agent_input_capability"
        case updatedAt = "updated_at"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(
            decoder,
            allowed: [
                "attempt_id", "seat_id", "status", "body",
                "agent_input_capability", "updated_at",
            ]
        )
        let values = try decoder.container(keyedBy: CodingKeys.self)
        attemptID = try values.decode(String.self, forKey: .attemptID)
        seatID = try values.decode(String.self, forKey: .seatID)
        status = try values.decode(String.self, forKey: .status)
        body = try values.decode(String.self, forKey: .body)
        agentInputCapability = try values.decodeIfPresent(
            String.self, forKey: .agentInputCapability
        ) ?? ""
        updatedAt = try values.decode(String.self, forKey: .updatedAt)
        guard !attemptID.isEmpty, !seatID.isEmpty,
              ["running", "succeeded", "failed", "cancelled"].contains(status),
              body.utf8.count <= 30 * 1024, !updatedAt.isEmpty,
              agentInputCapability.isEmpty ||
                (status == "running" && ["pending", "available", "unavailable"].contains(agentInputCapability)) else {
            throw LocalProductWireError.invalidJSON
        }
    }
}

public struct LocalRoundtableView: Codable, Equatable, Sendable {
    public let session: LocalRoundtableSession
    public let seats: [String: LocalRoundtableSeat]
    public let rounds: [LocalRoundtableRound]
    public let messages: [String: LocalRoundtableMessage]
    public let attempts: [String: LocalRoundtableSeatAttempt]
    public let interventions: [String: LocalRoundtableIntervention]
    public let deliveries: [String: LocalRoundtableSeatDelivery]
    public let digest: String

    enum CodingKeys: String, CodingKey {
        case session, seats, rounds, messages, attempts, interventions, deliveries, digest
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(
            decoder,
            allowed: ["session", "seats", "rounds", "messages", "attempts", "interventions", "deliveries", "digest"]
        )
        let values = try decoder.container(keyedBy: CodingKeys.self)
        session = try values.decode(LocalRoundtableSession.self, forKey: .session)
        seats = try values.decode([String: LocalRoundtableSeat].self, forKey: .seats)
        rounds = try values.decode([LocalRoundtableRound].self, forKey: .rounds)
        messages = try values.decode([String: LocalRoundtableMessage].self, forKey: .messages)
        attempts = try values.decodeIfPresent(
            [String: LocalRoundtableSeatAttempt].self, forKey: .attempts
        ) ?? [:]
        interventions = try values.decodeIfPresent(
            [String: LocalRoundtableIntervention].self, forKey: .interventions
        ) ?? [:]
        deliveries = try values.decodeIfPresent(
            [String: LocalRoundtableSeatDelivery].self, forKey: .deliveries
        ) ?? [:]
        digest = try values.decode(String.self, forKey: .digest)
        guard digest.count == 64, !seats.isEmpty else {
            throw LocalProductWireError.invalidJSON
        }
    }
}

public struct LocalRoundtableExportReceipt: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let sessionID: String
    public let exportID: String
    public let digest: String
    public let notBefore: String
    public let expiresAt: String

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case sessionID = "session_id"
        case exportID = "export_id"
        case digest
        case notBefore = "not_before"
        case expiresAt = "expires_at"
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(decoder, allowed: [
            "schema_version", "session_id", "export_id", "digest", "not_before", "expires_at",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
        sessionID = try values.decode(String.self, forKey: .sessionID)
        exportID = try values.decode(String.self, forKey: .exportID)
        digest = try values.decode(String.self, forKey: .digest)
        notBefore = try values.decode(String.self, forKey: .notBefore)
        expiresAt = try values.decode(String.self, forKey: .expiresAt)
        guard schemaVersion == 1, !sessionID.isEmpty, !exportID.isEmpty,
              digest.count == 64, !notBefore.isEmpty, !expiresAt.isEmpty else {
            throw LocalProductWireError.invalidJSON
        }
    }
}

public struct LocalRoundtableExportDocument: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let export: LocalRoundtableExportReceipt
    public let document: Data

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case export, document
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(decoder, allowed: ["schema_version", "export", "document"])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
        export = try values.decode(LocalRoundtableExportReceipt.self, forKey: .export)
        document = try values.decode(Data.self, forKey: .document)
        guard schemaVersion == 1, document.count > 0, document.count <= 1_048_576 else {
            throw LocalProductWireError.invalidJSON
        }
    }
}

public struct LocalRoundtableImportResult: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let sessionID: String
    public let importID: String
    public let contractDigest: String
    public let viewDigest: String
    public let recorded: Bool
    public let view: LocalRoundtableView

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case sessionID = "session_id"
        case importID = "import_id"
        case contractDigest = "contract_digest"
        case viewDigest = "view_digest"
        case recorded, view
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(decoder, allowed: [
            "schema_version", "session_id", "import_id", "contract_digest",
            "view_digest", "recorded", "view",
        ])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = try values.decode(Int.self, forKey: .schemaVersion)
        sessionID = try values.decode(String.self, forKey: .sessionID)
        importID = try values.decode(String.self, forKey: .importID)
        contractDigest = try values.decode(String.self, forKey: .contractDigest)
        viewDigest = try values.decode(String.self, forKey: .viewDigest)
        recorded = try values.decode(Bool.self, forKey: .recorded)
        view = try values.decode(LocalRoundtableView.self, forKey: .view)
        guard schemaVersion == 1, !sessionID.isEmpty, !importID.isEmpty,
              contractDigest.count == 64, viewDigest.count == 64,
              view.session.id == sessionID else {
            throw LocalProductWireError.invalidJSON
        }
    }
}

public struct LocalRoundtableSessionCreateRequest: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let sessionID: String
    public let moderatorSeat: String
    public let title: String
    public let link: LocalRoundtableSessionLinkRequest?
    public let correlationID: String

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case sessionID = "session_id"
        case moderatorSeat = "moderator_seat"
        case title
        case link
        case correlationID = "correlation_id"
    }

    public init(
        schemaVersion: Int,
        sessionID: String,
        moderatorSeat: String,
        title: String,
        link: LocalRoundtableSessionLinkRequest? = nil,
        correlationID: String
    ) {
        self.schemaVersion = schemaVersion
        self.sessionID = sessionID
        self.moderatorSeat = moderatorSeat
        self.title = title
        self.link = link
        self.correlationID = correlationID
    }
}

public struct LocalRoundtableSessionLinkRequest: Codable, Equatable, Sendable {
    public let conversationID: String
    public let missionID: String
    public let teamInstanceID: String

    enum CodingKeys: String, CodingKey {
        case conversationID = "conversation_id"
        case missionID = "mission_id"
        case teamInstanceID = "team_instance_id"
    }

    public init(conversationID: String, missionID: String, teamInstanceID: String) {
        self.conversationID = conversationID
        self.missionID = missionID
        self.teamInstanceID = teamInstanceID
    }
}

public struct LocalRoundtableAddSeatRequest: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let sessionID: String
    public let seatID: String
    public let displayName: String
    public let selection: LocalRoundtableSeatBindingRequest?
    public let correlationID: String

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case sessionID = "session_id"
        case seatID = "seat_id"
        case displayName = "display_name"
        case selection
        case correlationID = "correlation_id"
    }

    public init(
        schemaVersion: Int,
        sessionID: String,
        seatID: String,
        displayName: String,
        selection: LocalRoundtableSeatBindingRequest? = nil,
        correlationID: String
    ) {
        self.schemaVersion = schemaVersion
        self.sessionID = sessionID
        self.seatID = seatID
        self.displayName = displayName
        self.selection = selection
        self.correlationID = correlationID
    }
}

public struct LocalRoundtableSeatBindingRequest: Codable, Equatable, Sendable {
    public let agentDefinitionID: String
    public let teamRoleKind: String
    public let runtimeProfileID: String

    enum CodingKeys: String, CodingKey {
        case agentDefinitionID = "agent_definition_id"
        case teamRoleKind = "team_role_kind"
        case runtimeProfileID = "runtime_profile_id"
    }

    public init(
        agentDefinitionID: String,
        teamRoleKind: String,
        runtimeProfileID: String
    ) {
        self.agentDefinitionID = agentDefinitionID
        self.teamRoleKind = teamRoleKind
        self.runtimeProfileID = runtimeProfileID
    }
}

public struct LocalRoundtableRetireSeatRequest: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let sessionID: String
    public let seatID: String
    public let moderatorSeat: String
    public let correlationID: String

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case sessionID = "session_id"
        case seatID = "seat_id"
        case moderatorSeat = "moderator_seat"
        case correlationID = "correlation_id"
    }
}

public struct LocalRoundtableOpenRoundRequest: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let sessionID: String
    public let roundID: String
    public let moderatorSeat: String
    public let prompt: String
    public let correlationID: String

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case sessionID = "session_id"
        case roundID = "round_id"
        case moderatorSeat = "moderator_seat"
        case prompt
        case correlationID = "correlation_id"
    }

    public init(
        schemaVersion: Int,
        sessionID: String,
        roundID: String,
        moderatorSeat: String,
        prompt: String = "",
        correlationID: String
    ) {
        self.schemaVersion = schemaVersion
        self.sessionID = sessionID
        self.roundID = roundID
        self.moderatorSeat = moderatorSeat
        self.prompt = prompt
        self.correlationID = correlationID
    }
}

public struct LocalRoundtablePauseRoundRequest: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let sessionID: String
    public let roundID: String
    public let interventionID: String
    public let moderatorSeat: String
    public let correlationID: String

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case sessionID = "session_id"
        case roundID = "round_id"
        case interventionID = "intervention_id"
        case moderatorSeat = "moderator_seat"
        case correlationID = "correlation_id"
    }
}

public struct LocalRoundtableSteerSeatRequest: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let sessionID: String
    public let roundID: String
    public let interventionID: String
    public let moderatorSeat: String
    public let seatID: String
    public let attemptID: String
    public let guidance: Data
    public let correlationID: String

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case sessionID = "session_id"
        case roundID = "round_id"
        case interventionID = "intervention_id"
        case moderatorSeat = "moderator_seat"
        case seatID = "seat_id"
        case attemptID = "attempt_id"
        case guidance
        case correlationID = "correlation_id"
    }
}

public struct LocalRoundtableRetrySeatRequest: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let sessionID: String
    public let roundID: String
    public let interventionID: String
    public let moderatorSeat: String
    public let seatID: String
    public let attemptID: String
    public let expectedMembershipRevision: Int?
    public let expectedSeatBindingDigest: String?
    public let guidance: String
    public let correlationID: String

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case sessionID = "session_id"
        case roundID = "round_id"
        case interventionID = "intervention_id"
        case moderatorSeat = "moderator_seat"
        case seatID = "seat_id"
        case attemptID = "attempt_id"
        case expectedMembershipRevision = "expected_membership_revision"
        case expectedSeatBindingDigest = "expected_seat_binding_digest"
        case guidance
        case correlationID = "correlation_id"
    }

    public init(
        schemaVersion: Int,
        sessionID: String,
        roundID: String,
        interventionID: String,
        moderatorSeat: String,
        seatID: String,
        attemptID: String,
        expectedMembershipRevision: Int? = nil,
        expectedSeatBindingDigest: String? = nil,
        guidance: String,
        correlationID: String
    ) {
        self.schemaVersion = schemaVersion
        self.sessionID = sessionID
        self.roundID = roundID
        self.interventionID = interventionID
        self.moderatorSeat = moderatorSeat
        self.seatID = seatID
        self.attemptID = attemptID
        self.expectedMembershipRevision = expectedMembershipRevision
        self.expectedSeatBindingDigest = expectedSeatBindingDigest
        self.guidance = guidance
        self.correlationID = correlationID
    }
}

public struct LocalRoundtableSkipSeatRequest: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let sessionID: String
    public let roundID: String
    public let interventionID: String
    public let moderatorSeat: String
    public let seatID: String
    public let expectedMembershipRevision: Int?
    public let expectedSeatBindingDigest: String?
    public let correlationID: String

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case sessionID = "session_id"
        case roundID = "round_id"
        case interventionID = "intervention_id"
        case moderatorSeat = "moderator_seat"
        case seatID = "seat_id"
        case expectedMembershipRevision = "expected_membership_revision"
        case expectedSeatBindingDigest = "expected_seat_binding_digest"
        case correlationID = "correlation_id"
    }

    public init(
        schemaVersion: Int,
        sessionID: String,
        roundID: String,
        interventionID: String,
        moderatorSeat: String,
        seatID: String,
        expectedMembershipRevision: Int? = nil,
        expectedSeatBindingDigest: String? = nil,
        correlationID: String
    ) {
        self.schemaVersion = schemaVersion
        self.sessionID = sessionID
        self.roundID = roundID
        self.interventionID = interventionID
        self.moderatorSeat = moderatorSeat
        self.seatID = seatID
        self.expectedMembershipRevision = expectedMembershipRevision
        self.expectedSeatBindingDigest = expectedSeatBindingDigest
        self.correlationID = correlationID
    }
}

public struct LocalRoundtableReplaceSeatRequest: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let sessionID: String
    public let roundID: String
    public let interventionID: String
    public let moderatorSeat: String
    public let seatID: String
    public let displayName: String
    public let selection: LocalRoundtableSeatBindingRequest
    public let correlationID: String

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case sessionID = "session_id"
        case roundID = "round_id"
        case interventionID = "intervention_id"
        case moderatorSeat = "moderator_seat"
        case seatID = "seat_id"
        case displayName = "display_name"
        case selection
        case correlationID = "correlation_id"
    }
}

public struct LocalRoundtableExportRequest: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let sessionID: String
    public let correlationID: String

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case sessionID = "session_id"
        case correlationID = "correlation_id"
    }
}

public struct LocalRoundtableImportRequest: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let document: Data
    public let correlationID: String

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case document
        case correlationID = "correlation_id"
    }
}

public struct LocalRoundtableProposeMessageRequest: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let sessionID: String
    public let roundID: String
    public let messageID: String
    public let writerSeat: String
    public let targetSeat: String
    public let body: String
    public let artifactRefs: [String]
    public let correlationID: String

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case sessionID = "session_id"
        case roundID = "round_id"
        case messageID = "message_id"
        case writerSeat = "writer_seat"
        case targetSeat = "target_seat"
        case body
        case artifactRefs = "artifact_refs"
        case correlationID = "correlation_id"
    }
}

public struct LocalRoundtableRelayMessageRequest: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let sessionID: String
    public let messageID: String
    public let moderatorSeat: String
    public let correlationID: String

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case sessionID = "session_id"
        case messageID = "message_id"
        case moderatorSeat = "moderator_seat"
        case correlationID = "correlation_id"
    }
}

public struct LocalRoundtableAckMessageRequest: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let sessionID: String
    public let messageID: String
    public let seatID: String
    public let correlationID: String

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case sessionID = "session_id"
        case messageID = "message_id"
        case seatID = "seat_id"
        case correlationID = "correlation_id"
    }
}

public struct LocalRoundtableInsertMessageRequest: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let sessionID: String
    public let messageID: String
    public let moderatorSeat: String
    public let correlationID: String

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case sessionID = "session_id"
        case messageID = "message_id"
        case moderatorSeat = "moderator_seat"
        case correlationID = "correlation_id"
    }
}

public struct LocalRoundtableDropMessageRequest: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let sessionID: String
    public let messageID: String
    public let moderatorSeat: String
    public let correlationID: String

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case sessionID = "session_id"
        case messageID = "message_id"
        case moderatorSeat = "moderator_seat"
        case correlationID = "correlation_id"
    }
}

public struct LocalRoundtableConcludeRequest: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let sessionID: String
    public let moderatorSeat: String
    public let correlationID: String

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case sessionID = "session_id"
        case moderatorSeat = "moderator_seat"
        case correlationID = "correlation_id"
    }
}

public struct LocalRoundtableSnapshotRequest: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let sessionID: String

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case sessionID = "session_id"
    }
}

public enum LocalRoundtableWire {
    public static func decodeView(_ data: Data) throws -> LocalRoundtableView {
        do {
            try StrictJSONScanner.validate(data)
            let view = try JSONDecoder().decode(LocalRoundtableView.self, from: data)
            let seatIDs = Set(view.seats.keys)
            let referencedSeats = Set(view.messages.values.map(\.writerSeat))
                .union(view.messages.values.map(\.targetSeat))
                .union([view.session.moderatorSeat])
            guard seatIDs == Set(view.seats.values.map(\.id)),
                  referencedSeats.isSubset(of: seatIDs) else {
                throw LocalProductWireError.invalidJSON
            }
            return view
        } catch let error as LocalProductWireError {
            throw error
        } catch {
            throw LocalProductWireError.invalidJSON
        }
    }

    public static func decodeExport(_ data: Data) throws -> LocalRoundtableExportDocument {
        try decodeStrict(LocalRoundtableExportDocument.self, data: data)
    }

    public static func decodeImport(_ data: Data) throws -> LocalRoundtableImportResult {
        try decodeStrict(LocalRoundtableImportResult.self, data: data)
    }

    private static func decodeStrict<T: Decodable>(_ type: T.Type, data: Data) throws -> T {
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
