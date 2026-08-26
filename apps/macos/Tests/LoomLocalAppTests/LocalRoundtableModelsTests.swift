import Foundation
import XCTest
@testable import LoomLocalAppUI
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

    func testRetiredRoundtableSeatIsNotProjectedAsActive() throws {
        let activeView = try LocalRoundtableWire.decodeView(Data(viewJSON().utf8))
        XCTAssertTrue(roundtableSeatIsActive(activeView.seats["seat-writer"]))
        XCTAssertEqual(roundtableActiveSeatCount(activeView), 3)
        XCTAssertEqual(roundtableActiveParticipantCount(activeView), 2)

        let retiredJSON = viewJSON().replacingOccurrences(
            of: #""seat-writer":{"id":"seat-writer","display_name":"Writer Seat","available":true}"#,
            with: #""seat-writer":{"id":"seat-writer","display_name":"Writer Seat","available":false}"#
        )
        let retiredView = try LocalRoundtableWire.decodeView(Data(retiredJSON.utf8))
        XCTAssertFalse(roundtableSeatIsActive(retiredView.seats["seat-writer"]))
        XCTAssertFalse(roundtableSeatIsActive(nil))
        XCTAssertEqual(roundtableActiveSeatCount(retiredView), 2)
        XCTAssertEqual(roundtableActiveParticipantCount(retiredView), 1)

        let candidate = LocalRoundtableAgentCandidate(
            id: "main:reviewer:profile-1",
            seatID: "seat-writer",
            displayName: "Reviewer",
            responsibility: "Review",
            routeSummary: "OpenCode · DeepSeek"
        )
        XCTAssertTrue(
            roundtableAvailableAgentCandidates(
                [candidate], in: activeView
            ).isEmpty
        )
        XCTAssertEqual(
            roundtableAvailableAgentCandidates(
                [candidate], in: retiredView
            ),
            []
        )

        let preRoundRetiredJSON = """
        {"session":{"id":"session-1","moderator_seat":"seat-moderator","title":"Diagnosis handoff","created_at":"2026-08-16T12:00:00Z","concluded":false},"seats":{"seat-moderator":{"id":"seat-moderator","display_name":"Moderator","available":true},"seat-writer":{"id":"seat-writer","display_name":"Writer Seat","available":false}},"rounds":[],"messages":{},"digest":"\(digest)"}
        """
        let preRoundRetiredView = try LocalRoundtableWire.decodeView(
            Data(preRoundRetiredJSON.utf8)
        )
        XCTAssertEqual(
            roundtableAvailableAgentCandidates(
                [candidate], in: preRoundRetiredView
            ),
            [candidate]
        )
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

    func testSuggestedSessionIDIsUsableAndChanges() throws {
        let a = RoundtableWorkbench.buildSuggestedSessionID(from: Date(timeIntervalSince1970: 1_700_000_000))
        XCTAssertFalse(a.isEmpty)
        XCTAssertNil(a.rangeOfCharacter(from: CharacterSet.whitespacesAndNewlines))
        XCTAssertLessThanOrEqual(a.count, 128)
        let pattern = try NSRegularExpression(pattern: #"^rt-\d{8}-\d{4}$"#)
        let range = NSRange(a.startIndex..<a.endIndex, in: a)
        XCTAssertNotNil(pattern.firstMatch(in: a, range: range))

        let b = RoundtableWorkbench.buildSuggestedSessionID(from: Date(timeIntervalSince1970: 1_700_000_120))
        XCTAssertNotEqual(a, b)
    }

    func testRoundtableAgentSeatIDsAreDeterministicAndDaemonSafe() {
        let identity = "subagent:opencode/open-code reviewer/profile/1"
        let seatID = LocalRoundtableAgentCandidate.seatID(for: identity)
        XCTAssertEqual(
            seatID,
            LocalRoundtableAgentCandidate.seatID(for: identity)
        )
        XCTAssertTrue(seatID.hasPrefix("agent-"))
        XCTAssertLessThanOrEqual(seatID.utf8.count, 128)
        XCTAssertNil(seatID.rangeOfCharacter(from: .whitespacesAndNewlines))
        XCTAssertNil(seatID.rangeOfCharacter(from: .controlCharacters))
    }

    func testRoundtableSeatsFollowConfirmedUserAddOrder() throws {
        let catalog = [
            LocalRoundtableAgentCandidate(
                id: "main:catalog-first:profile-1",
                seatID: "seat-writer",
                displayName: "Catalog First",
                responsibility: "Write",
                routeSummary: "Loom Native · DeepSeek"
            ),
            LocalRoundtableAgentCandidate(
                id: "subagent:catalog-second:profile-2",
                seatID: "seat-target",
                displayName: "Catalog Second",
                responsibility: "Review",
                routeSummary: "OpenCode · MiniMax"
            ),
        ]
        let view = try LocalRoundtableWire.decodeView(Data(viewJSON().utf8))

        let ordered = roundtableOrderedAgentCandidates(
            catalog,
            in: view,
            preferredSeatOrder: ["seat-target", "seat-writer"]
        )

        XCTAssertEqual(ordered.map(\.seatID), ["seat-target", "seat-writer"])
        XCTAssertEqual(ordered.first?.displayName, "Catalog Second")
    }

    func testRoundtableSeatOrderCannotReactivateRetiredSeat() throws {
        let catalog = [
            LocalRoundtableAgentCandidate(
                id: "main:writer:profile-1",
                seatID: "seat-writer",
                displayName: "Writer",
                responsibility: "Write",
                routeSummary: "Loom Native · DeepSeek"
            ),
            LocalRoundtableAgentCandidate(
                id: "subagent:target:profile-2",
                seatID: "seat-target",
                displayName: "Target",
                responsibility: "Review",
                routeSummary: "OpenCode · MiniMax"
            ),
        ]
        let retiredJSON = viewJSON().replacingOccurrences(
            of: #""seat-writer":{"id":"seat-writer","display_name":"Writer Seat","available":true}"#,
            with: #""seat-writer":{"id":"seat-writer","display_name":"Writer Seat","available":false}"#
        )
        let view = try LocalRoundtableWire.decodeView(Data(retiredJSON.utf8))

        let ordered = roundtableOrderedAgentCandidates(
            catalog,
            in: view,
            preferredSeatOrder: ["seat-writer", "seat-target"]
        )

        XCTAssertEqual(ordered.map(\.seatID), ["seat-target"])
    }

    // The workbench must surface the actual discussion result, not just a
    // dense digest hash. When a session is concluded, produce readable lines
    // that show each round and message status + body so the user knows what
    // the discussion concluded.
    func testLocalRoundtableResultSurfacesConcludedDiscussion() throws {
        var json = viewJSON()
        json = json.replacingOccurrences(
            of: "\"concluded\":false", with: "\"concluded\":true"
        )
        let view = try LocalRoundtableWire.decodeView(Data(json.utf8))
        let lines = RoundtableWorkbench.localRoundtableResultLines(view)
        XCTAssertTrue(lines.contains { $0.contains("Concluded") })
        XCTAssertTrue(lines.contains { $0.contains("Diagnosis handoff") })
        XCTAssertTrue(lines.contains { $0.contains("Round 1") })
        XCTAssertTrue(lines.contains { $0.contains("inserted") })
        XCTAssertTrue(lines.contains { $0.contains("seat-writer") })
        XCTAssertTrue(lines.contains { $0.contains("seat-target") })
        XCTAssertTrue(lines.contains { $0.contains("Diagnosis: bounded.") })
    }

    // An open (not-yet-concluded) session has no final result to show.
    func testLocalRoundtableResultEmptyWhenNotConcluded() throws {
        let view = try LocalRoundtableWire.decodeView(Data(viewJSON().utf8))
        XCTAssertTrue(RoundtableWorkbench.localRoundtableResultLines(view).isEmpty)
    }
}
