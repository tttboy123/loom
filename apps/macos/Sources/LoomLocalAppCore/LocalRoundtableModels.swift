import Foundation

// Governed Handoff / Roundtable wire models. The daemon stamps EmittedAt
// server-side, so requests carry only client-authored correlation IDs and
// identities. The response is the full digest-bound session view.

public struct LocalRoundtableSeat: Codable, Equatable, Sendable {
    public let id: String
    public let displayName: String
    public let available: Bool

    enum CodingKeys: String, CodingKey {
        case id
        case displayName = "display_name"
        case available
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(decoder, allowed: ["id", "display_name", "available"])
        let values = try decoder.container(keyedBy: CodingKeys.self)
        id = try values.decode(String.self, forKey: .id)
        displayName = try values.decode(String.self, forKey: .displayName)
        available = try values.decode(Bool.self, forKey: .available)
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

    enum CodingKeys: String, CodingKey {
        case id
        case sequence
        case messageCount = "message_count"
        case messages
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(
            decoder,
            allowed: ["id", "sequence", "message_count", "messages"]
        )
        let values = try decoder.container(keyedBy: CodingKeys.self)
        id = try values.decode(String.self, forKey: .id)
        sequence = try values.decode(Int.self, forKey: .sequence)
        messageCount = try values.decode(Int.self, forKey: .messageCount)
        messages = try values.decode([LocalRoundtableMessage].self, forKey: .messages)
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

    enum CodingKeys: String, CodingKey {
        case id
        case moderatorSeat = "moderator_seat"
        case title
        case createdAt = "created_at"
        case concluded
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(
            decoder,
            allowed: ["id", "moderator_seat", "title", "created_at", "concluded"]
        )
        let values = try decoder.container(keyedBy: CodingKeys.self)
        id = try values.decode(String.self, forKey: .id)
        moderatorSeat = try values.decode(String.self, forKey: .moderatorSeat)
        title = try values.decode(String.self, forKey: .title)
        createdAt = try values.decode(String.self, forKey: .createdAt)
        concluded = try values.decode(Bool.self, forKey: .concluded)
        guard !id.isEmpty, !moderatorSeat.isEmpty, !title.isEmpty,
              !createdAt.isEmpty else {
            throw LocalProductWireError.invalidJSON
        }
    }
}

public struct LocalRoundtableView: Codable, Equatable, Sendable {
    public let session: LocalRoundtableSession
    public let seats: [String: LocalRoundtableSeat]
    public let rounds: [LocalRoundtableRound]
    public let messages: [String: LocalRoundtableMessage]
    public let digest: String

    enum CodingKeys: String, CodingKey {
        case session, seats, rounds, messages, digest
    }

    public init(from decoder: Decoder) throws {
        try rejectUnknownKeys(
            decoder,
            allowed: ["session", "seats", "rounds", "messages", "digest"]
        )
        let values = try decoder.container(keyedBy: CodingKeys.self)
        session = try values.decode(LocalRoundtableSession.self, forKey: .session)
        seats = try values.decode([String: LocalRoundtableSeat].self, forKey: .seats)
        rounds = try values.decode([LocalRoundtableRound].self, forKey: .rounds)
        messages = try values.decode([String: LocalRoundtableMessage].self, forKey: .messages)
        digest = try values.decode(String.self, forKey: .digest)
        guard digest.count == 64, !seats.isEmpty else {
            throw LocalProductWireError.invalidJSON
        }
    }
}

public struct LocalRoundtableSessionCreateRequest: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let sessionID: String
    public let moderatorSeat: String
    public let title: String
    public let correlationID: String

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case sessionID = "session_id"
        case moderatorSeat = "moderator_seat"
        case title
        case correlationID = "correlation_id"
    }
}

public struct LocalRoundtableAddSeatRequest: Codable, Equatable, Sendable {
    public let schemaVersion: Int
    public let sessionID: String
    public let seatID: String
    public let displayName: String
    public let correlationID: String

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case sessionID = "session_id"
        case seatID = "seat_id"
        case displayName = "display_name"
        case correlationID = "correlation_id"
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
    public let correlationID: String

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case sessionID = "session_id"
        case roundID = "round_id"
        case moderatorSeat = "moderator_seat"
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
}
