import Foundation
import XCTest
@testable import LoomLocalAppUI
@testable import LoomLocalAppCore

final class LocalRoundtableModelsTests: XCTestCase {
    private var digest: String { String(repeating: "a", count: 64) }

    func testRoundtableDocumentReaderStopsAtOneMegabyteBoundary() throws {
        let directory = FileManager.default.temporaryDirectory
            .appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: directory) }

        let validURL = directory.appendingPathComponent("valid.loom-roundtable")
        let valid = Data(repeating: 0x61, count: 1_048_576)
        try valid.write(to: validURL)
        XCTAssertEqual(
            try readBoundedRoundtableDocument(at: validURL),
            valid
        )

        let oversizedURL = directory.appendingPathComponent("oversized.loom-roundtable")
        try Data(repeating: 0x62, count: 1_048_577).write(to: oversizedURL)
        XCTAssertThrowsError(try readBoundedRoundtableDocument(at: oversizedURL))

        let emptyURL = directory.appendingPathComponent("empty.loom-roundtable")
        try Data().write(to: emptyURL)
        XCTAssertThrowsError(try readBoundedRoundtableDocument(at: emptyURL))
    }

    private func viewJSON() -> String {
        """
        {"session":{"id":"session-1","moderator_seat":"seat-moderator","title":"Diagnosis handoff","created_at":"2026-08-16T12:00:00Z","concluded":false},"seats":{"seat-moderator":{"id":"seat-moderator","display_name":"Moderator","available":true},"seat-writer":{"id":"seat-writer","display_name":"Writer Seat","available":true},"seat-target":{"id":"seat-target","display_name":"Target Seat","available":true}},"rounds":[{"id":"round-1","sequence":1,"message_count":1,"messages":[{"id":"msg-1","round_id":"round-1","writer_seat":"seat-writer","target_seat":"seat-target","body":"Diagnosis: bounded.","artifact_refs":[],"body_digest":"\(digest)","status":"inserted","proposed_at":"2026-08-16T12:00:04Z","relayed_at":"2026-08-16T12:00:06Z","acknowledged_at":"2026-08-16T12:00:08Z"}]}],"messages":{"msg-1":{"id":"msg-1","round_id":"round-1","writer_seat":"seat-writer","target_seat":"seat-target","body":"Diagnosis: bounded.","artifact_refs":[],"body_digest":"\(digest)","status":"inserted","proposed_at":"2026-08-16T12:00:04Z","relayed_at":"2026-08-16T12:00:06Z","acknowledged_at":"2026-08-16T12:00:08Z"}},"digest":"\(digest)"}
        """
    }

    private func linkedViewJSON() -> String {
        let context = """
        "context":{"conversation_id":"conversation-1","mission_id":"mission/team-1","team_id":"team-1","team_version":3,"workspace_id":"project-1"}
        """
        let session = viewJSON().replacingOccurrences(
            of: #""concluded":false}"#,
            with: "\"concluded\":false,\(context)}"
        )
        let execution = """
        {"profile_id":"profile-deepseek","harness_adapter":"loom-native","runtime_instance_id":"runtime-deepseek","provider_id":"deepseek","provider_account_id":"deepseek.primary","model_id":"deepseek-chat","auth_mode":"brokered","endpoint_fingerprint":"\(digest)","credential_reference":"credential-ref-deepseek-primary","credential_revision":3,"reasoning_effort":"","timeout_nanoseconds":60000000000,"budget":null,"capabilities":["text"],"remote_tool_enrollment_id":"","remote_tool_enrollment_digest":"","binding_digest":"\(digest)"}
        """
        let binding = """
        "binding":{"agent_definition_id":"agent-writer","team_role_kind":"main","runtime_profile_id":"profile-deepseek","execution_binding":\(execution),"membership_revision":1,"binding_digest":"\(digest)"}
        """
        return session.replacingOccurrences(
            of: #""seat-writer":{"id":"seat-writer","display_name":"Writer Seat","available":true}"#,
            with: "\"seat-writer\":{\"id\":\"seat-writer\",\"display_name\":\"Writer Seat\",\"available\":true,\(binding)}"
        )
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

    func testMissionLinkedRoundtableDecodesFrozenSeatAndUsesAuthoritativeRoute() throws {
        let view = try LocalRoundtableWire.decodeView(Data(linkedViewJSON().utf8))
        XCTAssertEqual(view.session.context?.missionID, "mission/team-1")
        XCTAssertEqual(view.session.context?.teamVersion, 3)
        XCTAssertEqual(
            view.seats["seat-writer"]?.binding?.executionBinding.providerAccountID,
            "deepseek.primary"
        )
        let catalog = [
            LocalRoundtableAgentCandidate(
                id: "main:agent-writer:profile-deepseek",
                seatID: "seat-writer",
                displayName: "Writer",
                responsibility: "Write",
                routeSummary: "Stale route",
                agentDefinitionID: "agent-writer",
                teamRoleKind: "main",
                runtimeProfileID: "profile-deepseek"
            ),
        ]
        let ordered = roundtableOrderedAgentCandidates(
            catalog, in: view, preferredSeatOrder: ["seat-writer"]
        )
        XCTAssertEqual(
            ordered.first?.routeSummary,
            "Loom Native · DeepSeek · deepseek.primary · deepseek-chat"
        )

        let unknownBinding = linkedViewJSON().replacingOccurrences(
            of: "\"membership_revision\":1",
            with: "\"membership_revision\":1,\"provider_override\":true"
        )
        XCTAssertThrowsError(
            try LocalRoundtableWire.decodeView(Data(unknownBinding.utf8))
        )
    }

    func testConversationRoundTableTargetMatchesExactSeatBinding() throws {
        let view = try LocalRoundtableWire.decodeView(Data(linkedViewJSON().utf8))
        let payload = try JSONDecoder().decode(
            LocalProductConversationActionPayload.self,
            from: Data(
                """
                {"session_id":"session-1","round_id":"round-1","seat_id":"seat-writer",\
                "membership_revision":1,"seat_binding_digest":"\(digest)"}
                """.utf8
            )
        )
        XCTAssertTrue(
            payload.matchesFrozenRoundTableTarget(in: view, requiresAttempt: false)
        )

        let drifted = try LocalRoundtableWire.decodeView(
            Data(
                linkedViewJSON().replacingOccurrences(
                    of: "\"membership_revision\":1",
                    with: "\"membership_revision\":2"
                ).utf8
            )
        )
        XCTAssertFalse(
            payload.matchesFrozenRoundTableTarget(in: drifted, requiresAttempt: false)
        )
    }

    func testSkipSeatRequestEncodesExpectedFrozenBinding() throws {
        let request = LocalRoundtableSkipSeatRequest(
            schemaVersion: 1,
            sessionID: "session-1",
            roundID: "round-1",
            interventionID: "intervention-skip-1",
            moderatorSeat: "seat-moderator",
            seatID: "seat-writer",
            expectedMembershipRevision: 3,
            expectedSeatBindingDigest: digest,
            correlationID: "incident-skip-1"
        )
        let object = try XCTUnwrap(
            JSONSerialization.jsonObject(with: JSONEncoder().encode(request))
                as? [String: Any]
        )
        XCTAssertEqual(object["expected_membership_revision"] as? Int, 3)
        XCTAssertEqual(object["expected_seat_binding_digest"] as? String, digest)
    }

    func testRetrySeatRequestEncodesExpectedFrozenBinding() throws {
        let request = LocalRoundtableRetrySeatRequest(
            schemaVersion: 1,
            sessionID: "session-1",
            roundID: "round-1",
            interventionID: "intervention-retry-1",
            moderatorSeat: "seat-moderator",
            seatID: "seat-writer",
            attemptID: "attempt-writer-1",
            expectedMembershipRevision: 3,
            expectedSeatBindingDigest: digest,
            guidance: "Retry within the reviewed binding.",
            correlationID: "incident-retry-1"
        )
        let object = try XCTUnwrap(
            JSONSerialization.jsonObject(with: JSONEncoder().encode(request))
                as? [String: Any]
        )
        XCTAssertEqual(object["expected_membership_revision"] as? Int, 3)
        XCTAssertEqual(object["expected_seat_binding_digest"] as? String, digest)
    }

    func testLatestRoundWinsOverOlderHigherAttemptNumber() throws {
        var object = try XCTUnwrap(
            JSONSerialization.jsonObject(with: Data(linkedViewJSON().utf8))
                as? [String: Any]
        )
        object["rounds"] = [
            ["id": "round-1", "sequence": 1, "message_count": 0, "messages": []],
            ["id": "round-2", "sequence": 2, "message_count": 0, "messages": []],
        ]
        object["messages"] = [String: Any]()

        func attempt(
            id: String,
            roundID: String,
            seatID: String,
            number: Int,
            status: String,
            startedAt: String,
            completedAt: String
        ) -> [String: Any] {
            [
                "attempt_id": id, "round_id": roundID, "seat_id": seatID,
                "attempt_number": number, "execution_team_id": "roundtable-team-1",
                "work_item_id": "work-\(id)", "run_id": "run-\(id)",
                "segment_id": "segment-\(id)", "claim_generation": 1,
                "runtime_instance_id": "runtime-deepseek",
                "agent_instance_id": "agent-\(seatID)", "membership_revision": 1,
                "seat_binding_digest": digest, "execution_binding_digest": digest,
                "context_capsule_digest": digest, "payload_reference": "payload-\(id)",
                "status": status, "output_digest": digest, "incident_id": "",
                "failure_code": "", "failure_stage": "", "retryable": false,
                "started_at": startedAt, "completed_at": completedAt,
            ]
        }
        var failedTarget = attempt(
            id: "target-r2-1", roundID: "round-2", seatID: "seat-target",
            number: 1, status: "failed", startedAt: "2026-08-16T12:00:07Z",
            completedAt: "2026-08-16T12:00:08Z"
        )
        failedTarget["payload_reference"] = ""
        failedTarget["output_digest"] = ""
        failedTarget["incident_id"] = "11111111-1111-4111-8111-111111111111"
        failedTarget["failure_code"] = "provider_rejected"
        failedTarget["failure_stage"] = "provider_http"
        failedTarget["retryable"] = true
        object["attempts"] = [
            "writer-r1-2": attempt(
                id: "writer-r1-2", roundID: "round-1", seatID: "seat-writer",
                number: 2, status: "succeeded", startedAt: "2026-08-16T12:00:05Z",
                completedAt: "2026-08-16T12:00:06Z"
            ),
            "writer-r2-1": attempt(
                id: "writer-r2-1", roundID: "round-2", seatID: "seat-writer",
                number: 1, status: "succeeded", startedAt: "2026-08-16T12:00:07Z",
                completedAt: "2026-08-16T12:00:08Z"
            ),
            "target-r2-1": failedTarget,
            "target-r2-2": attempt(
                id: "target-r2-2", roundID: "round-2", seatID: "seat-target",
                number: 2, status: "succeeded", startedAt: "2026-08-16T12:00:09Z",
                completedAt: "2026-08-16T12:00:10Z"
            ),
        ]
        object["deliveries"] = [
            "writer-r2-1": [
                "attempt_id": "writer-r2-1", "seat_id": "seat-writer",
                "status": "succeeded", "body": "Latest governed conclusion",
                "updated_at": "2026-08-16T12:00:08Z",
            ],
            "target-r2-2": [
                "attempt_id": "target-r2-2", "seat_id": "seat-target",
                "status": "succeeded", "body": "Independent peer result",
                "updated_at": "2026-08-16T12:00:10Z",
            ],
        ]
        object["interventions"] = [String: Any]()

        let view = try LocalRoundtableWire.decodeView(
            JSONSerialization.data(withJSONObject: object, options: [.sortedKeys])
        )
        XCTAssertEqual(
            roundtableLatestAttempt(in: view, seatID: "seat-writer")?.attemptID,
            "writer-r2-1"
        )
        XCTAssertTrue(roundtableReadyToAcceptCandidate(view))
        XCTAssertTrue(roundtableCanStartSynthesis(view))
        XCTAssertFalse(roundtableNeedsIntervention(view))
        XCTAssertEqual(roundtableNextRoundID(view), "round-3")
        XCTAssertTrue(roundtableDefaultSynthesisPrompt.contains("Lead Agent"))
        XCTAssertTrue(roundtableDefaultSynthesisPrompt.contains("Other Agents"))
    }

    func testReplacedSeatProjectsReplacementIdentityAndFrozenRoute() throws {
        let replaced = linkedViewJSON()
            .replacingOccurrences(of: "\"display_name\":\"Writer Seat\"", with: "\"display_name\":\"Reviewer\"")
            .replacingOccurrences(of: "\"agent_definition_id\":\"agent-writer\"", with: "\"agent_definition_id\":\"agent-reviewer\"")
            .replacingOccurrences(of: "\"team_role_kind\":\"main\"", with: "\"team_role_kind\":\"subagent\"")
            .replacingOccurrences(of: "profile-deepseek", with: "profile-minimax")
            .replacingOccurrences(of: "loom-native", with: "opencode")
            .replacingOccurrences(of: "runtime-deepseek", with: "runtime-minimax")
            .replacingOccurrences(of: "deepseek.primary", with: "minimax.primary")
            .replacingOccurrences(of: "deepseek-chat", with: "MiniMax-M2.1")
            .replacingOccurrences(of: "deepseek", with: "minimax")
        let view = try LocalRoundtableWire.decodeView(Data(replaced.utf8))
        let catalog = [
            LocalRoundtableAgentCandidate(
                id: "main:agent-writer:profile-deepseek", teamRoleID: "planner",
                seatID: "seat-writer", displayName: "Writer", responsibility: "Plan",
                routeSummary: "Stale writer route", agentDefinitionID: "agent-writer",
                teamRoleKind: "main", runtimeProfileID: "profile-deepseek"
            ),
            LocalRoundtableAgentCandidate(
                id: "subagent:agent-reviewer:profile-minimax", teamRoleID: "reviewer",
                seatID: "seat-reviewer", displayName: "Reviewer", responsibility: "Review",
                routeSummary: "Stale reviewer route", agentDefinitionID: "agent-reviewer",
                teamRoleKind: "subagent", runtimeProfileID: "profile-minimax"
            ),
        ]

        let projected = roundtableOrderedAgentCandidates(
            catalog, in: view, preferredSeatOrder: ["seat-writer"]
        )

        XCTAssertEqual(projected.first?.seatID, "seat-writer")
        XCTAssertEqual(projected.first?.displayName, "Reviewer")
        XCTAssertEqual(projected.first?.teamRoleID, "reviewer")
        XCTAssertEqual(projected.first?.agentDefinitionID, "agent-reviewer")
        XCTAssertEqual(
            projected.first?.routeSummary,
            "OpenCode · MiniMax · minimax.primary · MiniMax-M2.1"
        )
    }

    func testRestoredRoundtableMustMatchExactMissionTeamAndConversation() throws {
        let view = try LocalRoundtableWire.decodeView(Data(linkedViewJSON().utf8))
        let exact = LocalRoundtableMissionLink(
            conversationID: "conversation-1", missionID: "mission/team-1",
            teamInstanceID: "team-1", title: "Diagnosis"
        )
        XCTAssertTrue(roundtableSessionMatchesMission(view, link: exact))
        XCTAssertFalse(
            roundtableSessionMatchesMission(
                view,
                link: LocalRoundtableMissionLink(
                    conversationID: "conversation-2", missionID: "mission/team-1",
                    teamInstanceID: "team-1", title: "Diagnosis"
                )
            )
        )
        XCTAssertFalse(
            roundtableSessionMatchesMission(
                view,
                link: LocalRoundtableMissionLink(
                    conversationID: "conversation-1", missionID: "mission/team-2",
                    teamInstanceID: "team-1", title: "Diagnosis"
                )
            )
        )
        XCTAssertFalse(
            roundtableSessionMatchesMission(
                view,
                link: LocalRoundtableMissionLink(
                    conversationID: "conversation-1", missionID: "mission/team-1",
                    teamInstanceID: "team-2", title: "Diagnosis"
                )
            )
        )
    }

    func testRoundtableDiscussionDefaultsAreNeutralAndMissionScoped() {
        XCTAssertEqual(roundtableInitialDiscussionPrompt(missionLink: nil), "")

        let linked = LocalRoundtableMissionLink(
            conversationID: "conversation-1",
            missionID: "mission/team-1",
            teamInstanceID: "team-1",
            title: "Discuss: Mission"
        )
        XCTAssertEqual(
            roundtableInitialDiscussionPrompt(missionLink: linked),
            roundtableDefaultMissionPrompt
        )
        XCTAssertFalse(
            roundtableDefaultMissionPrompt.localizedCaseInsensitiveContains(
                "daemon"
            )
        )

        let supplied = LocalRoundtableMissionLink(
            conversationID: "conversation-1",
            missionID: "mission/team-1",
            teamInstanceID: "team-1",
            title: "Discuss: Mission",
            initialPrompt: "Compare the two proposed approaches."
        )
        XCTAssertEqual(
            roundtableInitialDiscussionPrompt(missionLink: supplied),
            "Compare the two proposed approaches."
        )
    }

    func testRoundtableDecodesSeatAttemptMetadataWithoutContent() throws {
        let attempt = """
        "attempts":{"attempt-writer-1":{"attempt_id":"attempt-writer-1","round_id":"round-1","seat_id":"seat-writer","attempt_number":1,"execution_team_id":"roundtable-team-1","work_item_id":"work-writer-1","run_id":"run-writer-1","segment_id":"segment-writer-1","claim_generation":1,"runtime_instance_id":"runtime-1","agent_instance_id":"roundtable-agent-writer","membership_revision":1,"seat_binding_digest":"\(digest)","execution_binding_digest":"\(digest)","context_capsule_digest":"\(digest)","payload_reference":"roundtable-payload-writer-1","status":"succeeded","output_digest":"\(digest)","incident_id":"","failure_code":"","failure_stage":"","retryable":false,"started_at":"2026-08-16T12:00:09Z","completed_at":"2026-08-16T12:00:10Z"}},"deliveries":{"attempt-writer-1":{"attempt_id":"attempt-writer-1","seat_id":"seat-writer","status":"succeeded","body":"Visible governed output","updated_at":"2026-08-16T12:00:10Z"}},
        """
        let json = linkedViewJSON().replacingOccurrences(
            of: "\"digest\":\"\(digest)\"}",
            with: "\(attempt)\"digest\":\"\(digest)\"}"
        )
        let view = try LocalRoundtableWire.decodeView(Data(json.utf8))
        let decoded = try XCTUnwrap(view.attempts["attempt-writer-1"])
        XCTAssertEqual(decoded.status, "succeeded")
        XCTAssertEqual(decoded.payloadReference, "roundtable-payload-writer-1")
        XCTAssertEqual(
            view.deliveries["attempt-writer-1"]?.body,
            "Visible governed output"
        )
        XCTAssertEqual(
            roundtableLatestAttempt(in: view, seatID: "seat-writer")?.attemptID,
            "attempt-writer-1"
        )
        XCTAssertEqual(
            roundtableModeratorCandidate(in: view)?.attemptID,
            "attempt-writer-1"
        )
        XCTAssertFalse(roundtableReadyToAcceptCandidate(view))
        XCTAssertFalse(json.contains("model output"))

        let leakedField = json.replacingOccurrences(
            of: "\"output_digest\":\"\(digest)\"",
            with: "\"output_digest\":\"\(digest)\",\"output_text\":\"not allowed\""
        )
        XCTAssertThrowsError(
            try LocalRoundtableWire.decodeView(Data(leakedField.utf8))
        )
    }

    func testLiveSteerUsesProjectedAttemptCapability() throws {
        let attempt = """
        "attempts":{"attempt-writer-live":{"attempt_id":"attempt-writer-live","round_id":"round-1","seat_id":"seat-writer","attempt_number":1,"execution_team_id":"roundtable-team-1","work_item_id":"work-writer-1","run_id":"run-writer-1","segment_id":"segment-writer-1","claim_generation":1,"runtime_instance_id":"runtime-deepseek","agent_instance_id":"roundtable-agent-writer","membership_revision":1,"seat_binding_digest":"\(digest)","execution_binding_digest":"\(digest)","context_capsule_digest":"\(digest)","payload_reference":"","status":"running","output_digest":"","incident_id":"","failure_code":"","failure_stage":"","retryable":false,"started_at":"2026-08-16T12:00:09Z","completed_at":""}},"interventions":{},"deliveries":{"attempt-writer-live":{"attempt_id":"attempt-writer-live","seat_id":"seat-writer","status":"running","body":"","agent_input_capability":"available","updated_at":"2026-08-16T12:00:10Z"}},
        """
        let availableJSON = linkedViewJSON().replacingOccurrences(
            of: "\"digest\":\"\(digest)\"}",
            with: "\(attempt)\"digest\":\"\(digest)\"}"
        )
        let available = try LocalRoundtableWire.decodeView(Data(availableJSON.utf8))
        XCTAssertEqual(
            roundtableLiveInputState(available, seatID: "seat-writer"),
            .available
        )

        let unavailable = try LocalRoundtableWire.decodeView(
            Data(availableJSON.replacingOccurrences(
                of: "\"agent_input_capability\":\"available\"",
                with: "\"agent_input_capability\":\"unavailable\""
            ).utf8)
        )
        XCTAssertEqual(
            roundtableLiveInputState(unavailable, seatID: "seat-writer"),
            .unavailable
        )

        let pending = try LocalRoundtableWire.decodeView(
            Data(availableJSON.replacingOccurrences(
                of: ",\"agent_input_capability\":\"available\"",
                with: ""
            ).utf8)
        )
        XCTAssertEqual(
            roundtableLiveInputState(pending, seatID: "seat-writer"),
            .pending
        )
    }

    func testRetiredRoundtableSeatIsNotProjectedAsActive() throws {
        let activeView = try LocalRoundtableWire.decodeView(Data(viewJSON().utf8))
        XCTAssertTrue(roundtableSeatIsActive(activeView.seats["seat-writer"]))
        XCTAssertEqual(roundtableActiveSeatCount(activeView), 3)
        XCTAssertEqual(roundtableActiveParticipantCount(activeView), 2)
        XCTAssertEqual(roundtableSeatSummary(activeView), "2 Agents + Moderator")

        let retiredJSON = viewJSON().replacingOccurrences(
            of: #""seat-writer":{"id":"seat-writer","display_name":"Writer Seat","available":true}"#,
            with: #""seat-writer":{"id":"seat-writer","display_name":"Writer Seat","available":false}"#
        )
        let retiredView = try LocalRoundtableWire.decodeView(Data(retiredJSON.utf8))
        XCTAssertFalse(roundtableSeatIsActive(retiredView.seats["seat-writer"]))
        XCTAssertFalse(roundtableSeatIsActive(nil))
        XCTAssertEqual(roundtableActiveSeatCount(retiredView), 2)
        XCTAssertEqual(roundtableActiveParticipantCount(retiredView), 1)
        XCTAssertEqual(roundtableSeatSummary(retiredView), "1 Agent + Moderator")

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

        let linked = LocalRoundtableSessionCreateRequest(
            schemaVersion: 1, sessionID: "session-linked",
            moderatorSeat: "seat-moderator", title: "Mission review",
            link: LocalRoundtableSessionLinkRequest(
                conversationID: "conversation-1", missionID: "mission/team-1",
                teamInstanceID: "team-1"
            ),
            correlationID: correlation
        )
        let linkedObject = try XCTUnwrap(
            JSONSerialization.jsonObject(
                with: try JSONEncoder().encode(linked)
            ) as? [String: Any]
        )
        let link = try XCTUnwrap(linkedObject["link"] as? [String: Any])
        XCTAssertEqual(Set(link.keys), [
            "conversation_id", "mission_id", "team_instance_id",
        ])

        let add = LocalRoundtableAddSeatRequest(
            schemaVersion: 1, sessionID: "session-linked", seatID: "seat-writer",
            displayName: "Writer",
            selection: LocalRoundtableSeatBindingRequest(
                agentDefinitionID: "agent-writer", teamRoleKind: "main",
                runtimeProfileID: "profile-deepseek"
            ),
            correlationID: correlation
        )
        let addObject = try XCTUnwrap(
            JSONSerialization.jsonObject(
                with: try JSONEncoder().encode(add)
            ) as? [String: Any]
        )
        let selection = try XCTUnwrap(addObject["selection"] as? [String: Any])
        XCTAssertEqual(Set(selection.keys), [
            "agent_definition_id", "team_role_kind", "runtime_profile_id",
        ])
        XCTAssertNil(selection["provider_account_id"])
        XCTAssertNil(selection["credential_reference"])

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

    func testRoundtableInterventionRequestsUseClosedContentBoundaries() throws {
        let correlation = "11111111-1111-4111-8111-111111111111"
        let pause = LocalRoundtablePauseRoundRequest(
            schemaVersion: 1, sessionID: "session-1", roundID: "round-1",
            interventionID: "intervention-pause-1", moderatorSeat: "seat-moderator",
            correlationID: correlation
        )
        let pauseObject = try XCTUnwrap(
            JSONSerialization.jsonObject(with: JSONEncoder().encode(pause)) as? [String: Any]
        )
        XCTAssertEqual(Set(pauseObject.keys), [
            "schema_version", "session_id", "round_id", "intervention_id",
            "moderator_seat", "correlation_id",
        ])

        let steer = LocalRoundtableSteerSeatRequest(
            schemaVersion: 1, sessionID: "session-1", roundID: "round-1",
            interventionID: "intervention-steer-1", moderatorSeat: "seat-moderator",
            seatID: "seat-writer", attemptID: "attempt-writer-1",
            guidance: Data("private steering guidance".utf8), correlationID: correlation
        )
        let steerObject = try XCTUnwrap(
            JSONSerialization.jsonObject(with: JSONEncoder().encode(steer)) as? [String: Any]
        )
        XCTAssertEqual(Set(steerObject.keys), [
            "schema_version", "session_id", "round_id", "intervention_id",
            "moderator_seat", "seat_id", "attempt_id", "guidance", "correlation_id",
        ])
        XCTAssertEqual(
            steerObject["guidance"] as? String,
            Data("private steering guidance".utf8).base64EncodedString()
        )
        XCTAssertFalse(String(data: try JSONEncoder().encode(steer), encoding: .utf8)?.contains("private steering guidance") ?? true)
    }

    func testRoundtableExportImportWireIsStrictAndBounded() throws {
        let correlation = "11111111-1111-4111-8111-111111111111"
        let document = Data(#"{"schema_version":1}"#.utf8)
        let request = LocalRoundtableImportRequest(
            schemaVersion: 1, document: document, correlationID: correlation
        )
        let object = try XCTUnwrap(
            JSONSerialization.jsonObject(with: JSONEncoder().encode(request)) as? [String: Any]
        )
        XCTAssertEqual(Set(object.keys), ["schema_version", "document", "correlation_id"])
        XCTAssertEqual(object["document"] as? String, document.base64EncodedString())

        let json = """
        {"schema_version":1,"export":{"schema_version":1,"session_id":"session-1","export_id":"export-1","digest":"\(digest)","not_before":"2026-08-26T12:00:00Z","expires_at":"2026-08-27T12:00:00Z"},"document":"\(document.base64EncodedString())"}
        """
        let decoded = try LocalRoundtableWire.decodeExport(Data(json.utf8))
        XCTAssertEqual(decoded.export.digest, digest)
        XCTAssertEqual(decoded.document, document)
        XCTAssertThrowsError(
            try LocalRoundtableWire.decodeExport(
                Data(json.replacingOccurrences(of: "}", with: ",\"unknown\":true}", options: [], range: json.range(of: "}", options: .backwards)).utf8)
            )
        )
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
