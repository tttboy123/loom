import Foundation
import XCTest
@testable import LoomLocalAppCore

final class LocalRoundtableModelsTests: XCTestCase {
    private var digest: String { String(repeating: "a", count: 64) }

    private func viewJSON() -> String {
        """
        {"session":{"id":"session-1","moderator_seat":"seat-moderator","title":"Diagnosis handoff","created_at":"2026-08-16T12:00:00Z","concluded":false},"seats":{"seat-moderator":{"id":"seat-moderator","display_name":"Moderator","available":true},"seat-writer":{"id":"seat-writer","display_name":"Writer Seat","available":true},"seat-target":{"id":"seat-target","display_name":"Target Seat","available":true}},"rounds":[{"id":"round-1","sequence":1,"message_count":1,"messages":[{"id":"msg-1","round_id":"round-1","writer_seat":"seat-writer","target_seat":"seat-target","body":"Diagnosis: bounded.","artifact_refs":[],"body_digest":"\(digest)","status":"inserted","proposed_at":"2026-08-16T12:00:04Z","relayed_at":"2026-08-16T12:00:06Z","acknowledged_at":"2026-08-16T12:00:08Z"}]}],"messages":{"msg-1":{"id":"msg-1","round_id":"round-1","writer_seat":"seat-writer","target_seat":"seat-target","body":"Diagnosis: bounded.","artifact_refs":[],"body_digest":"\(digest)","status":"inserted","proposed_at":"2026-08-16T12:00:04Z","relayed_at":"2026-08-16T12:00:06Z","acknowledged_at":"2026-08-16T12:00:08Z"}},"digest":"\(digest)"}
        """
    }

    func testRoundtableViewDecodesFullLifecycleStrictly() throws {
        let view = try LocalRoundtableWire.decodeView(Data(viewJSON().utf8))
        XCTAssertEqual(view.session.moderatorSeat, "seat-moderator")
        XCTAssertEqual(view.seats.count, 3)
        XCTAssertEqual(view.rounds.first?.messageCount, 1)
        XCTAssertEqual(view.messages["msg-1"]?.status, "inserted")
        XCTAssertEqual(view.rounds.first?.messages.first?.writerSeat, "seat-writer")
        XCTAssertEqual(view.digest, digest)
    }

    func testRoundtableViewRejectsUnknownKeysNullCollectionsAndBadDigests() throws {
        let unknown = viewJSON().replacingOccurrences(
            of: "\"digest\":\"\(digest)\"",
            with: "\"digest\":\"\(digest)\",\"unknown\":true"
        )
        XCTAssertThrowsError(try LocalRoundtableWire.decodeView(Data(unknown.utf8)))

        let nullMessages = viewJSON().replacingOccurrences(
            of: "\"artifact_refs\":[]",
            with: "\"artifact_refs\":null"
        )
        XCTAssertThrowsError(try LocalRoundtableWire.decodeView(Data(nullMessages.utf8)))

        let badStatus = viewJSON().replacingOccurrences(
            of: "\"status\":\"inserted\"",
            with: "\"status\":\"unknown\""
        )
        XCTAssertThrowsError(try LocalRoundtableWire.decodeView(Data(badStatus.utf8)))

        let shortDigest = viewJSON().replacingOccurrences(
            of: "\"body_digest\":\"\(digest)\"",
            with: "\"body_digest\":\"short\""
        )
        XCTAssertThrowsError(try LocalRoundtableWire.decodeView(Data(shortDigest.utf8)))
    }

    func testRoundtableRequestsEncodeEveryGovernedField() throws {
        let correlation = "11111111-1111-4111-8111-111111111111"
        let create = LocalRoundtableSessionCreateRequest(
            schemaVersion: 1, sessionID: "session-1",
            moderatorSeat: "seat-moderator", title: "Diagnosis handoff",
            correlationID: correlation
        )
        let encoded = try JSONEncoder().encode(create)
        let object = try XCTUnwrap(
            JSONSerialization.jsonObject(with: encoded) as? [String: Any]
        )
        XCTAssertEqual(Set(object.keys), [
            "schema_version", "session_id", "moderator_seat", "title", "correlation_id",
        ])
        XCTAssertEqual(object["schema_version"] as? Int, 1)
        XCTAssertEqual(object["correlation_id"] as? String, correlation)

        let propose = LocalRoundtableProposeMessageRequest(
            schemaVersion: 1, sessionID: "session-1", roundID: "round-1",
            messageID: "msg-1", writerSeat: "seat-writer", targetSeat: "seat-target",
            body: "Diagnosis: bounded.", artifactRefs: [], correlationID: correlation
        )
        let proposedObject = try XCTUnwrap(
            JSONSerialization.jsonObject(
                with: try JSONEncoder().encode(propose)
            ) as? [String: Any]
        )
        XCTAssertEqual(Set(proposedObject.keys), [
            "schema_version", "session_id", "round_id", "message_id",
            "writer_seat", "target_seat", "body", "artifact_refs", "correlation_id",
        ])
    }
}
