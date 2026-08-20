import XCTest
@testable import LoomLocalAppCore
@testable import LoomLocalAppUI

final class LoomWorkspaceShellStateTests: XCTestCase {
    func testGovernancePanelCanOpenPinSwitchAndCloseIndependently() {
        var panel = LoomGovernancePanelState()
        XCTAssertEqual(panel.mode, .hidden)

        panel.open(.team)
        XCTAssertEqual(panel.mode, .visible)
        XCTAssertEqual(panel.destination, .team)

        panel.togglePin()
        panel.open(.evidence)
        XCTAssertEqual(panel.mode, .pinned)
        XCTAssertEqual(panel.destination, .evidence)

        panel.close()
        XCTAssertEqual(panel.mode, .hidden)
        XCTAssertEqual(panel.destination, .evidence)
    }

    func testEscapeDismissesVisibleAndPinnedGovernancePanel() {
        var panel = LoomGovernancePanelState()

        XCTAssertFalse(panel.dismissIfPresented())
        XCTAssertEqual(panel.mode, .hidden)

        panel.open(.overview)
        XCTAssertTrue(panel.dismissIfPresented())
        XCTAssertEqual(panel.mode, .hidden)

        panel.open(.attention)
        panel.togglePin()
        XCTAssertEqual(panel.mode, .pinned)
        XCTAssertTrue(panel.dismissIfPresented())
        XCTAssertEqual(panel.mode, .hidden)
        XCTAssertEqual(panel.destination, .attention)
    }

    func testGovernanceSwitcherIncludesTopologyAndTimelineAsRealViews() {
        let destinations = LoomGovernanceDestination.allCases

        XCTAssertTrue(destinations.contains(.topology))
        XCTAssertTrue(destinations.contains(.timeline))
        XCTAssertEqual(
            Set(destinations.map(\.rawValue)).count,
            destinations.count,
            "Every governance view needs a unique accessible label"
        )
    }

    func testLayoutProtectsConversationAndUsesOverlayAtCompactWidth() {
        let wide = LoomWorkspaceLayoutPolicy.metrics(
            for: 1_440,
            panelMode: .visible
        )
        XCTAssertEqual(wide.railWidth, 192)
        XCTAssertEqual(wide.panelPlacement, .split)
        XCTAssertEqual(wide.panelWidth, 360)
        XCTAssertGreaterThanOrEqual(wide.conversationMinimumWidth, 560)

        let medium = LoomWorkspaceLayoutPolicy.metrics(
            for: 1_080,
            panelMode: .visible
        )
        XCTAssertEqual(medium.railWidth, 64)
        XCTAssertEqual(medium.panelPlacement, .split)
        XCTAssertGreaterThanOrEqual(medium.conversationMinimumWidth, 500)

        let compact = LoomWorkspaceLayoutPolicy.metrics(
            for: 900,
            panelMode: .pinned
        )
        XCTAssertEqual(compact.railWidth, 64)
        XCTAssertEqual(compact.panelPlacement, .overlay)
        XCTAssertEqual(compact.panelWidth, 360)
        XCTAssertGreaterThanOrEqual(compact.conversationMinimumWidth, 500)

        let hidden = LoomWorkspaceLayoutPolicy.metrics(
            for: 900,
            panelMode: .hidden
        )
        XCTAssertEqual(hidden.panelPlacement, .hidden)
    }

    func testSideTaskStatusLabelMapsRawCodesToReadableCopy() {
        XCTAssertEqual(sideTaskStatusLabelText("invalid_request"),
                       "Loom rejected the side task request. Review the fields and try again.")
        XCTAssertEqual(sideTaskStatusLabelText("state_unavailable"),
                       "Side tasks are unavailable right now. Try again.")
        XCTAssertEqual(sideTaskStatusLabelText("capability_gap"),
                       "This Agent Team cannot run that kind of side task yet.")
        XCTAssertEqual(sideTaskStatusLabelText("Proposing"), "Proposing")
        XCTAssertEqual(sideTaskStatusLabelText("Idle"), "Idle")
        XCTAssertTrue(sideTaskStatusLabelText("unexpected_code").contains("unexpected_code"))
    }

    func testConversationDisambiguationSuffixDistinguishesSameDayAndOlder() {
        let calendar = Calendar.current
        let now = Date()
        let today = calendar.date(byAdding: .minute, value: -5, to: now)!
        let older = calendar.date(byAdding: .day, value: -3, to: now)!
        let suffixToday = conversationDisambiguationSuffix(for: today, now: now)
        let suffixOlder = conversationDisambiguationSuffix(for: older, now: now)
        XCTAssertFalse(suffixToday.isEmpty)
        XCTAssertFalse(suffixOlder.isEmpty)
        XCTAssertNotEqual(suffixToday, suffixOlder)
    }

    func testConversationTranscriptTextBuildsReadableCopyableThread() {
        let messages = [
            LocalProductChatMessage(
                messageID: "m1", segmentID: "s1", role: "user",
                content: "First question", tentative: false
            ),
            LocalProductChatMessage(
                messageID: "m2", segmentID: "s1", role: "loom",
                content: "First answer", tentative: false
            ),
            LocalProductChatMessage(
                messageID: "m3", segmentID: "s1", role: "proposal",
                content: "A proposed next step", tentative: true
            ),
        ]
        let text = conversationTranscriptText(messages)
        XCTAssertEqual(
            text,
            "You: First question\n\nLoom: First answer\n\nLoom proposal: A proposed next step"
        )
        XCTAssertEqual(conversationTranscriptText([]), "")
    }
    func testChatMissionObjectivePrefersProposalAndTruncatesLongText() {
        let thread = LocalProductChatThread(
            threadID: "t1",
            messages: [
                LocalProductChatMessage(
                    messageID: "u1", role: "user", content: "Original ask", tentative: false
                ),
                LocalProductChatMessage(
                    messageID: "p1", role: "proposal", content: "Proposed plan", tentative: true
                ),
            ],
            canReply: true, requiresConfirmation: false
        )
        let proposal = LocalProductChatMessage(
            messageID: "p1", role: "proposal", content: "Proposed plan", tentative: true
        )
        XCTAssertEqual(
            chatMissionObjectiveText(around: proposal, thread: thread),
            "Proposed plan"
        )
        // Empty proposal falls back to the latest user message.
        let emptyProposal = LocalProductChatMessage(
            messageID: "p2", role: "proposal", content: "", tentative: true
        )
        XCTAssertEqual(
            chatMissionObjectiveText(around: emptyProposal, thread: thread),
            "Original ask"
        )
        // Over-long proposal is truncated to the 4096 limit.
        let longProposal = LocalProductChatMessage(
            messageID: "p3", role: "proposal",
            content: String(repeating: "x", count: 5000), tentative: true
        )
        let objective = chatMissionObjectiveText(around: longProposal, thread: thread)
        XCTAssertEqual(objective.count, 4096)
    }


    func testCountActiveMissionsExcludesArchivedTeamMissions() throws {
        let teams = try JSONDecoder().decode(
            [LocalProductTeamSummary].self,
            from: Data("""
            [
              {"team_instance_id":"team-exec","display_name":"Exec","source_kind":"saved_team","state":"created","confirmed":true,"executable":true,"read_only":false},
              {"team_instance_id":"team-archived","display_name":"Archived","source_kind":"saved_team","state":"created","confirmed":true,"executable":false,"read_only":false}
            ]
            """.utf8)
        )
        let digest = String(repeating: "a", count: 64)
        func mission(_ team: String, _ lane: String) -> LocalProductMissionSummary {
            try! JSONDecoder().decode(
                LocalProductMissionSummary.self,
                from: Data("""
                {"schema_version":1,"mission_id":"mission/\(team)","team_instance_id":"\(team)","title":"M","source_kind":"team_execution","lane":"\(lane)","status":"blocked","priority":"normal","plan_digest":"\(digest)","simple":true,"node_count":1,"completed_node_count":0,"active_node_count":0,"review_node_count":0,"attention_count":1,"current_node_id":"main","last_milestone":"blocked","team_pulse":[],"topology":[]}
                """.utf8)
            )
        }
        let missions = [
            mission("team-exec", "Orchestrating"),
            mission("team-archived", "Orchestrating"),
            mission("team-exec", "Complete"),
            mission("team-archived", "Complete"),
        ]
        XCTAssertEqual(
            countActiveMissions(teams: teams, missions: missions),
            1
        )
    }



    func testCountActiveAttentionExcludesHistoricalAndArchivedTeams() throws {
        let teams = try JSONDecoder().decode(
            [LocalProductTeamSummary].self,
            from: Data("""
            [
              {"team_instance_id":"team-exec","display_name":"Exec","source_kind":"saved_team","state":"created","confirmed":true,"executable":true,"read_only":false},
              {"team_instance_id":"team-historical","display_name":"Historical","source_kind":"saved_team","state":"created","confirmed":true,"executable":true,"read_only":false},
              {"team_instance_id":"team-archived","display_name":"Archived","source_kind":"saved_team","state":"created","confirmed":true,"executable":false,"read_only":false}
            ]
            """.utf8)
        )
        let digest = String(repeating: "a", count: 64)
        func mission(_ team: String, _ lane: String) -> LocalProductMissionSummary {
            try! JSONDecoder().decode(
                LocalProductMissionSummary.self,
                from: Data("""
                {"schema_version":1,"mission_id":"mission/\(team)","team_instance_id":"\(team)","title":"M","source_kind":"team_execution","lane":"\(lane)","status":"blocked","priority":"normal","plan_digest":"\(digest)","simple":true,"node_count":1,"completed_node_count":0,"active_node_count":0,"review_node_count":0,"attention_count":1,"current_node_id":"main","last_milestone":"blocked","team_pulse":[],"topology":[]}
                """.utf8)
            )
        }
        let missions = [
            mission("team-exec", "Orchestrating"),
            mission("team-historical", "Complete"),
            mission("team-archived", "Orchestrating"),
        ]
        let attention = try JSONDecoder().decode(
            [LocalProductAttention].self,
            from: Data("""
            [
              {"schema_version":1,"attention_id":"a1","kind":"verification_failed","severity":"critical","team_instance_id":"team-exec","logical_node_id":"main","work_item_id":"w1","approval_request_id":"","runtime_instance_id":"r1","status":"failed","occurred_at":"2026-08-20T00:00:00Z","action_required":"inspect_failure"},
              {"schema_version":1,"attention_id":"a2","kind":"verification_failed","severity":"critical","team_instance_id":"team-historical","logical_node_id":"main","work_item_id":"w2","approval_request_id":"","runtime_instance_id":"r1","status":"failed","occurred_at":"2026-08-20T00:00:00Z","action_required":"inspect_failure"},
              {"schema_version":1,"attention_id":"a3","kind":"blocked","severity":"critical","team_instance_id":"team-archived","logical_node_id":"main","work_item_id":"w3","approval_request_id":"","runtime_instance_id":"r1","status":"blocked","occurred_at":"2026-08-20T00:00:00Z","action_required":"inspect_failure"}
            ]
            """.utf8)
        )
        // Only a1 is actionable: a2 sits on a Team whose Mission is already
        // Complete (history), and a3 sits on an archived (non-executable) Team.
        XCTAssertEqual(
            countActiveAttention(teams: teams, missions: missions, attention: attention),
            1
        )
    }

    func testActionableAttentionSplitsActionableFromHistory() throws {
        let teams = try JSONDecoder().decode(
            [LocalProductTeamSummary].self,
            from: Data("""
            [
              {"team_instance_id":"team-exec","display_name":"Exec","source_kind":"saved_team","state":"created","confirmed":true,"executable":true,"read_only":false},
              {"team_instance_id":"team-archived","display_name":"Archived","source_kind":"saved_team","state":"created","confirmed":true,"executable":false,"read_only":false}
            ]
            """.utf8)
        )
        let digest = String(repeating: "a", count: 64)
        func mission(_ team: String, _ lane: String) -> LocalProductMissionSummary {
            try! JSONDecoder().decode(
                LocalProductMissionSummary.self,
                from: Data("""
                {"schema_version":1,"mission_id":"mission/\(team)","team_instance_id":"\(team)","title":"M","source_kind":"team_execution","lane":"\(lane)","status":"blocked","priority":"normal","plan_digest":"\(digest)","simple":true,"node_count":1,"completed_node_count":0,"active_node_count":0,"review_node_count":0,"attention_count":1,"current_node_id":"main","last_milestone":"blocked","team_pulse":[],"topology":[]}
                """.utf8)
            )
        }
        let missions = [
            mission("team-exec", "Orchestrating"),
            mission("team-archived", "Orchestrating"),
        ]
        func attention(_ id: String, _ team: String) -> LocalProductAttention {
            try! JSONDecoder().decode(
                LocalProductAttention.self,
                from: Data("""
                {"schema_version":1,"attention_id":"\(id)","kind":"verification_failed","severity":"critical","team_instance_id":"\(team)","logical_node_id":"main","work_item_id":"w","approval_request_id":"","runtime_instance_id":"r1","status":"failed","occurred_at":"2026-08-20T00:00:00Z","action_required":"inspect_failure"}
                """.utf8)
            )
        }
        let items = [
            attention("a1", "team-exec"),
            attention("a2", "team-archived"),
        ]
        let actionable = actionableAttention(
            teams: teams, missions: missions, attention: items
        )
        let historical = historicalAttention(
            teams: teams, missions: missions, attention: items
        )
        XCTAssertEqual(actionable.map(\.attentionID), ["a1"])
        XCTAssertEqual(historical.map(\.attentionID), ["a2"])
        XCTAssertEqual(
            attentionActionTitle(actionRequired: "inspect_failure", kind: "verification_failed"),
            "Inspect failure"
        )
        XCTAssertEqual(
            attentionActionTitle(actionRequired: "", kind: "blocked"),
            "Blocked"
        )
        XCTAssertEqual(attentionActionTitle(actionRequired: "", kind: ""), "Needs your attention")
    }

    func testMissionBoardDetailTextIsNotContradictory() {
        // A finished Mission sits in the "Complete" lane with a terminal
        // outcome; "Complete · failed" reads contradictory, so the detail
        // should lead with the humanized outcome instead.
        XCTAssertEqual(missionBoardDetailText(lane: "Complete", status: "failed"), "Failed")
        XCTAssertEqual(missionBoardDetailText(lane: "Complete", status: "blocked"), "Blocked")
        XCTAssertEqual(missionBoardDetailText(lane: "Complete", status: "succeeded"), "Succeeded")
        // Active lanes keep both the lane and the humanized state.
        XCTAssertEqual(
            missionBoardDetailText(lane: "Orchestrating", status: "running"),
            "Orchestrating · Running"
        )
        XCTAssertEqual(
            missionBoardDetailText(lane: "Review", status: "blocked"),
            "Review · Blocked"
        )
        XCTAssertEqual(missionBoardDetailText(lane: "Proposed", status: ""), "Proposed")
        XCTAssertEqual(missionBoardDetailText(lane: "", status: "failed"), "Failed")
    }

}
