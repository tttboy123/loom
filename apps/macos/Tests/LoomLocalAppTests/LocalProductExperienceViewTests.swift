import AppKit
import CryptoKit
import SwiftUI
import XCTest
@testable import LoomLocalAppCore
@testable import LoomLocalAppUI

@MainActor
final class LocalProductExperienceViewTests: XCTestCase {

    func testBlockedMissionContinuationPreparesBoundedAuditedAttempt() throws {
        let preparation = try XCTUnwrap(
            missionContinuationPreparation(
                status: "blocked",
                title: "MiniMax live Agent conversation",
                teamInstanceID: "team-minimax",
                draft: "  Retry the coordinator with a shorter response.  "
            )
        )

        XCTAssertEqual(preparation.title, "MiniMax live Agent conversation")
        XCTAssertEqual(
            preparation.objective,
            "Retry the coordinator with a shorter response."
        )
        XCTAssertEqual(preparation.teamInstanceID, "team-minimax")
        XCTAssertTrue(preparation.newAttempt)
        XCTAssertEqual(missionContinuationSheetTitle(newAttempt: true), "Continue Mission")
        XCTAssertEqual(missionContinuationSheetTitle(newAttempt: false), "New Mission")
    }

    func testMissionContinuationRejectsActiveEmptyAndOversizedDrafts() {
        XCTAssertNil(
            missionContinuationPreparation(
                status: "running", title: "Active", teamInstanceID: "team-1",
                draft: "Change direction"
            )
        )
        XCTAssertNil(
            missionContinuationPreparation(
                status: "blocked", title: "Blocked", teamInstanceID: "team-1",
                draft: "   "
            )
        )
        XCTAssertNil(
            missionContinuationPreparation(
                status: "blocked", title: "Blocked", teamInstanceID: "team-1",
                draft: String(repeating: "x", count: 4_097)
            )
        )
        XCTAssertTrue(missionContinuationAcceptsInput(status: "failed"))
        XCTAssertTrue(missionContinuationAcceptsInput(status: "cancelled"))
        XCTAssertFalse(missionContinuationAcceptsInput(status: "succeeded"))
    }

    func testMissionActivityGroupsStreamingOutputByAgentAttempt() throws {
        let snapshot = try LocalProductWire.decodeSnapshot(
            Data(MissionOrchestrationTests.snapshotJSON.utf8)
        )
        let mission = try XCTUnwrap(snapshot.missions.first)
        let page = try timelinePage(
            teamID: mission.teamInstanceID,
            boardStatus: "running",
            hasMore: false,
            records: [
                timelineRecord(
                    deliveryID: "activity-1",
                    kind: "node_output_delta",
                    teamID: mission.teamInstanceID,
                    authority: "tentative",
                    logicalNodeID: mission.currentNodeID,
                    attemptNumber: 1,
                    sourceSequence: 1,
                    textDelta: "Planning the implementation. "
                ),
                timelineRecord(
                    deliveryID: "activity-2",
                    kind: "node_output_delta",
                    teamID: mission.teamInstanceID,
                    authority: "tentative",
                    logicalNodeID: mission.currentNodeID,
                    attemptNumber: 1,
                    sourceSequence: 2,
                    textDelta: "Creating the first artifact."
                ),
            ]
        )

        let entries = missionActivityEntries(mission: mission, timeline: page)

        XCTAssertEqual(entries.count, 1)
        XCTAssertEqual(entries[0].logicalNodeID, mission.currentNodeID)
        XCTAssertEqual(entries[0].attemptNumber, 1)
        XCTAssertEqual(
            entries[0].text,
            "Planning the implementation. Creating the first artifact."
        )
        XCTAssertTrue(entries[0].isTentative)
        XCTAssertTrue(missionActivityNeedsRefresh(status: "running"))
        XCTAssertFalse(missionActivityNeedsRefresh(status: "succeeded"))
        XCTAssertEqual(
            missionActivityVisibleText("first\nsecond\nlatest", expanded: false, limit: 12),
            "…cond\nlatest"
        )
        XCTAssertEqual(
            missionActivityVisibleText("first\nsecond\nlatest", expanded: true, limit: 12),
            "first\nsecond\nlatest"
        )
    }

    func testMissionActivityRestoresAcceptedOutputFromTerminalBoardNode() throws {
        let finalText = "Restart-safe accepted Agent output."
        let digest = SHA256.hash(data: Data(finalText.utf8))
            .map { String(format: "%02x", $0) }
            .joined()
        let page = try timelinePage(
            teamID: "team-restored",
            hasMore: false,
            records: [],
            nodes: [
                """
                {"logical_node_id":"main","status":"succeeded",
                 "dependency_satisfied":true,"current_attempt":1,
                 "work_item_id":"work-1","run_id":"run-1",
                 "runtime_instance_id":"runtime-1","agent_instance_id":"agent-1",
                 "verification_status":"accepted","recovery_action":"","retry_at":"",
                 "final_output_available":true,"final_output_text":"\(finalText)",
                 "final_output_digest":"\(digest)"}
                """,
            ]
        )

        let entries = missionActivityEntries(mission: nil, timeline: page)

        XCTAssertEqual(entries.count, 1)
        XCTAssertEqual(entries[0].status, "succeeded")
        XCTAssertEqual(entries[0].text, finalText)
        XCTAssertFalse(entries[0].isTentative)
        XCTAssertEqual(
            missionActivityAuthorityMessage(entries),
            "Accepted output restored from terminal Evidence."
        )
        XCTAssertEqual(
            missionActivityAuthorityMessage([
                MissionActivityEntry(
                    id: "main:2", logicalNodeID: "main", attemptNumber: 2,
                    title: "Coordinator", route: "MiniMax", status: "running",
                    text: "Working", isTentative: true, isTruncated: false
                ),
            ]),
            "Live output remains tentative until terminal Evidence is accepted."
        )
        XCTAssertNil(missionActivityAuthorityMessage([]))
    }

    func testMissionResultPresentationOpensWebEntryPointBeforeWorkspaceFolder() throws {
        let workspace = FileManager.default.temporaryDirectory
            .appendingPathComponent("loom-mission-result-\(UUID().uuidString)")
        try FileManager.default.createDirectory(
            at: workspace,
            withIntermediateDirectories: true
        )
        defer { try? FileManager.default.removeItem(at: workspace) }

        let folderResult = try XCTUnwrap(
            missionResultPresentation(workspacePath: workspace.path)
        )
        XCTAssertEqual(folderResult.openURL, workspace.standardizedFileURL)
        XCTAssertFalse(folderResult.isWebResult)

        let index = workspace.appendingPathComponent("index.html")
        try Data("<html></html>".utf8).write(to: index)

        let webResult = try XCTUnwrap(
            missionResultPresentation(workspacePath: workspace.path)
        )
        XCTAssertEqual(webResult.openURL, index.standardizedFileURL)
        XCTAssertEqual(webResult.revealURL, index.standardizedFileURL)
        XCTAssertTrue(webResult.isWebResult)
    }

    func testToolRecoveryActionsDescribeBoundedAuthorityWithoutRerun() {
        XCTAssertEqual(
            missionToolRecoveryActionLabel(.abortAttempt),
            "Abort Attempt"
        )
        XCTAssertEqual(
            missionToolRecoveryActionIcon(.acceptObservedEffect),
            "checkmark.seal"
        )
        XCTAssertTrue(
            missionToolRecoveryConfirmationDetail(.abortAttempt)
                .contains("will not rerun the original ToolCall")
        )
        XCTAssertTrue(
            missionToolRecoveryConfirmationDetail(.retryInNewAttempt)
                .contains("original ToolCall remains closed")
        )
    }

    func testAgentRestartRecoveryLabelsDoNotInventRetryAuthority() {
        XCTAssertEqual(
            missionAgentFailureLabel(
                stage: "agent_attempt_reconcile",
                code: "agent_input_resume_required",
                retryable: false
            ),
            "Resume approval required"
        )
        XCTAssertEqual(
            missionAgentFailureLabel(
                stage: "agent_attempt_reconcile",
                code: "provider_outcome_uncertain",
                retryable: false
            ),
            "Provider outcome uncertain"
        )
        XCTAssertEqual(
            missionAgentFailureLabel(
                stage: "agent_attempt_reconcile",
                code: "agent_input_recovery_unavailable",
                retryable: false
            ),
            "Encrypted input unavailable"
        )
        XCTAssertEqual(
            missionAgentFailureLabel(
                stage: "agent_attempt_reconcile",
                code: "agent_input_recovery_conflict",
                retryable: false
            ),
            "Recovery state conflict"
        )
        XCTAssertEqual(
            missionAgentFailureLabel(
                stage: "provider_connect", code: "timeout", retryable: true
            ),
            "Retry available"
        )
        let section = MissionInspectorSection(
            heading: "Team Pulse",
            rows: ["needs approval", "provider retry"],
            emptyMessage: "empty",
            agentRecoveryActions: [true, false]
        )
        XCTAssertTrue(section.canRecoverAgentAttempt(at: 0))
        XCTAssertFalse(section.canRecoverAgentAttempt(at: 1))
        XCTAssertTrue(missionAttemptRecoveryAvailable(
            stage: "agent_attempt_reconcile",
            code: "agent_input_resume_required",
            diagnosticAvailable: true,
            retryable: false
        ))
        XCTAssertFalse(missionAttemptRecoveryAvailable(
            stage: "agent_attempt_reconcile",
            code: "provider_outcome_uncertain",
            diagnosticAvailable: true,
            retryable: false
        ))
    }
    func testProviderAccountPolicySheetRendersAccountScopedGovernance() throws {
        let account = try JSONDecoder().decode(
            LocalProductProviderAccountDirectoryEntry.self,
            from: Data(
                """
                {"provider_id":"deepseek","provider_account_id":"deepseek.work","auth_mode":"brokered","credential_reference":"credential-ref-hidden","revision":3,"status":"verified","reason":"","policy_available":true,"policy_version":2,"policy_revision":2,"policy_digest":"\(String(repeating: "b", count: 64))","maximum_concurrent_attempts":4,"dispatch_window_seconds":60,"maximum_dispatch_starts":20,"maximum_assigned_budget_units":12000,"trust_domain":"external_provider","retention_mode":"zero_data_retention","data_region":"apac","rate_cards":[{"model_id":"deepseek-chat","revision":2,"rate_card_digest":"\(String(repeating: "d", count: 64))","currency":"USD","input_token_basis":"input_excludes_cache","input_microunits_per_million":270000,"output_microunits_per_million":1100000,"cache_read_microunits_per_million":70000,"cache_write_microunits_per_million":0,"rounding_mode":"ceiling_per_attempt","configured_at":"2026-08-12T05:00:00Z"}],"remote_tool_backends":[{"enrollment_version":1,"enrollment_id":"mcp-deepseek-work","backend_kind":"mcp_server","adapter_id":"builtin.mcp.stdio.v1","provider_account_policy_version":2,"provider_account_policy_revision":2,"provider_account_policy_digest":"\(String(repeating: "b", count: 64))","policy_current":true,"endpoint_fingerprint":"\(String(repeating: "c", count: 64))","mcp_server_id":"work-tools","allowed_tools":["get_issue","search_docs"],"revision":1,"status":"active","maximum_concurrent_calls":2,"maximum_calls_per_attempt":4,"timeout_seconds":30,"maximum_result_bytes":32768,"maximum_budget_units":2000,"configured_at":"2026-08-15T05:00:00Z","enrollment_digest":"\(String(repeating: "e", count: 64))"}]}
                """.utf8
            )
        )
        let store = LocalProductStore(
            client: ExperienceViewStubClient(results: [.success(.empty(viewVersion: "view-1"))])
        )

        let image = try XCTUnwrap(render(
            ProviderAccountPolicySheet(store: store, account: account),
            colorScheme: .light,
            dynamicTypeSize: .large,
            width: 520,
            height: 620
        ))

        XCTAssertEqual(image.pixelWidth, 520)
        XCTAssertEqual(image.pixelHeight, 620)
        XCTAssertGreaterThan(image.png.count, 10_000)
    }

    func testExplicitRecoveryTargetMeetsNativeAccessibilityMinimum() {
        XCTAssertGreaterThanOrEqual(
            LoomGraphite.minimumActionTarget,
            44
        )
    }

    func testMissionRailRendersTeamsAttentionAndHistoryComparePages()
        async throws
    {
        let snapshot = try ExperienceFixtures.populatedSnapshot()
        let agents = try XCTUnwrap(snapshot.teams.first?.agents)
        XCTAssertEqual(agents.count, 2)
        XCTAssertEqual(
            teamAgentRouteLabel(agents[0]),
            "Opencode · Deepseek / deepseek.primary · deepseek/deepseek-chat"
        )
        XCTAssertEqual(teamAgentTitle(agents[1]), "Subagent")
        XCTAssertEqual(
            missionDisplayTitle(
                candidate: "team-first",
                missionID: "mission/team-first",
                teamInstanceID: "team-first",
                teams: snapshot.teams
            ),
            "Release Team"
        )
        XCTAssertEqual(
            missionPresentationTitle(
                presentationTitle: "Build 225 Mission continuity",
                candidate: "Release Team",
                missionID: "mission/team-first",
                teamInstanceID: "team-first",
                teams: snapshot.teams
            ),
            "Build 225 Mission continuity"
        )
        XCTAssertEqual(
            missionPresentationTitle(
                presentationTitle: "   ",
                candidate: "team-first",
                missionID: "mission/team-first",
                teamInstanceID: "team-first",
                teams: snapshot.teams
            ),
            "Release Team"
        )
        for run in snapshot.runs {
            XCTAssertNotEqual(
                missionRuntimeDisplayName(run: run, snapshot: snapshot),
                run.runtimeInstanceID
            )
        }
        let store = LocalProductStore(
            client: ExperienceViewStubClient(results: [.success(snapshot)])
        )
        await store.refresh()

        store.showMissionTeams()
        let teams = try XCTUnwrap(render(
            MissionWorkbench(store: store, showRail: false),
            colorScheme: .light,
            dynamicTypeSize: .large
        ))
        store.showMissionAttention()
        let attention = try XCTUnwrap(render(
            MissionWorkbench(store: store, showRail: false),
            colorScheme: .light,
            dynamicTypeSize: .large
        ))
        store.showMissionLibrary()
        let library = try XCTUnwrap(render(
            MissionWorkbench(store: store, showRail: false),
            colorScheme: .light,
            dynamicTypeSize: .large
        ))

        XCTAssertEqual(teams.pixelWidth, 1_100)
        XCTAssertEqual(attention.pixelWidth, 1_100)
        XCTAssertEqual(library.pixelWidth, 1_100)
        XCTAssertEqual(Set([teams.digest, attention.digest, library.digest]).count, 3)
        XCTAssertGreaterThan(teams.png.count, 20_000)
        XCTAssertGreaterThan(attention.png.count, 20_000)
        XCTAssertGreaterThan(library.png.count, 20_000)
    }

    func testMissionInspectorTabsExposeDistinctSafeReadOnlyContent() throws {
        let snapshot = try LocalProductWire.decodeSnapshot(
            Data(MissionOrchestrationTests.snapshotJSON.utf8)
        )
        let mission = try XCTUnwrap(snapshot.missions.first)
        let timeline = try LocalProductWire.decodeTimeline(Data("""
        {
          "schema_version":1,
          "team_instance_id":"team-1",
          "view_version":"view-1",
          "next_cursor":"",
          "has_more":false,
          "gap":null,
          "records":[
            {
              "schema_version":1,
              "delivery_id":"delivery-evidence",
              "kind":"evidence_available",
              "authority":"journal",
              "team_instance_id":"team-1",
              "logical_node_id":"main",
              "attempt_number":1,
              "source_stream_id":"evidence/internal-evidence-id",
              "source_sequence":1,
              "source_event_id":"internal-event-id",
              "occurred_at":"2026-08-03T00:00:00Z",
              "cursor":"cursor-1",
              "payload":{
                "status":"",
                "reason_code":"",
                "action":"",
                "warning_code":"",
                "retry_at":"",
                "text_delta":"",
                "evidence_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
                "cost":{"observed":false,"amount_microunits":null,"currency":""}
              }
            },
            {
              "schema_version":1,
              "delivery_id":"delivery-verification",
              "kind":"verification_recorded",
              "authority":"journal",
              "team_instance_id":"team-1",
              "logical_node_id":"main",
              "attempt_number":1,
              "source_stream_id":"work-item/internal-work-id",
              "source_sequence":1,
              "source_event_id":"internal-verification-event",
              "occurred_at":"2026-08-03T00:00:01Z",
              "cursor":"cursor-2",
              "payload":{
                "status":"accepted",
                "reason_code":"",
                "action":"",
                "warning_code":"",
                "retry_at":"",
                "text_delta":"",
                "evidence_digest":"",
                "cost":{"observed":false,"amount_microunits":null,"currency":""}
              }
            }
          ],
          "board":{
            "schema_version":1,
            "team_instance_id":"team-1",
            "plan_digest":"",
            "status":"succeeded",
            "view_version":"view-1",
            "nodes":[{
              "logical_node_id":"main","node_kind":"route_sibling",
              "route_group_id":"route-group-main","status":"succeeded","dependency_satisfied":true,
              "current_attempt":1,"work_item_id":"work-1","run_id":"run-1",
              "runtime_instance_id":"runtime-1","agent_instance_id":"agent-main",
              "verification_status":"accepted","recovery_action":"","retry_at":"",
              "fallback_configured":true,"recovery_approval_required":true,
              "fallback_approval_available":true,"fallback_approval_version":3,
              "fallback_consumed":false,
              "execution_binding_available":true,"harness_adapter":"claude-code",
              "provider_id":"anthropic","provider_account_id":"anthropic.production",
              "model_id":"claude-sonnet","timeout_nanoseconds":300000000000,
              "binding_budget_credits":1200,"capabilities":["workspace_edit"],
              "credential_revision":7,
              "provider_account_policy_available":true,
              "provider_account_policy_version":2,
              "provider_account_policy_revision":3,
              "provider_account_policy_digest":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
              "provider_account_trust_domain":"external_provider",
              "provider_account_retention_mode":"limited_retention",
              "provider_account_data_region":"eu",
              "provider_account_assigned_budget_units":1000,
              "terminal_reason":"provider_rejected",
              "incident_id":"22222222-2222-4222-8222-222222222222",
              "failure_diagnostic_available":true,
              "failure_stage":"provider_auth","failure_code":"provider_rejected",
              "failure_retryable":true,
              "test_report_available":true,"test_report_count":3,
              "test_report_passed_count":2,"test_report_failed_count":1,
              "test_report_set_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
              "latest_test_runner":"go_test","latest_test_scope":"all",
              "latest_test_outcome":"passed",
              "latest_test_report_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
            },{
              "logical_node_id":"review","node_kind":"aggregation",
              "route_group_id":"route-group-main","status":"succeeded","dependency_satisfied":true,
              "current_attempt":1,"work_item_id":"work-2","run_id":"run-2",
              "runtime_instance_id":"runtime-2","agent_instance_id":"agent-review",
              "verification_status":"accepted","recovery_action":"","retry_at":"",
              "execution_binding_available":true,"harness_adapter":"codex",
              "provider_id":"openai","provider_account_id":"openai.primary",
              "model_id":"gpt-5.5-codex","timeout_nanoseconds":300000000000,
              "capabilities":["workspace_edit"],"credential_revision":5,
              "terminal_reason":"","incident_id":"",
              "failure_diagnostic_available":false,"failure_stage":"",
              "failure_code":"","failure_retryable":false
            },{
              "logical_node_id":"native","status":"blocked","dependency_satisfied":false,
              "current_attempt":0,"work_item_id":"","run_id":"",
              "runtime_instance_id":"","agent_instance_id":"",
              "verification_status":"","recovery_action":"","retry_at":"",
              "execution_binding_available":true,"harness_adapter":"codex",
              "provider_id":"openai","provider_account_id":"",
              "model_id":"codex","timeout_nanoseconds":60000000000,
              "capabilities":["workspace_edit"],"credential_revision":0,
              "terminal_reason":"Runtime is not online.",
              "incident_id":"44444444-4444-4444-8444-444444444444",
              "failure_diagnostic_available":true,
              "failure_stage":"agent_attempt_dispatch",
              "failure_code":"runtime_unavailable","failure_retryable":true
            }],
            "provider_accounts":[{
              "provider_id":"anthropic","provider_account_id":"anthropic.production",
              "active_attempts":0,"attempt_count":1,"failed_attempts":1,
              "rate_limited_attempts":0,"error_rate_basis_points":10000,
              "budget_attempt_count":1,"budget_units":100,
              "policy_available":true,"policy_revision":9,
              "policy_digest":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
              "maximum_concurrent_attempts":3,"dispatch_window_seconds":60,
              "maximum_dispatch_starts":4,"maximum_assigned_budget_units":800,
              "active_assigned_budget_units":100,
              "accounting_attempt_count":1,"usage_attempt_count":1,
              "input_tokens":90,"output_tokens":10,"cache_read_tokens":20,
              "cache_write_tokens":5,"total_tokens":100,"cost_attempt_count":1,
              "costs":[{"currency":"USD","source":"harness_reported","amount_microunits":450}],
              "aggregation_overflow":true
            }],
            "cost":{"observed":false,"amount_microunits":null,"currency":""}
          },
          "attention":[]
        }
        """.utf8))

        let sections = MissionInspectorTab.allCases.map {
            missionInspectorSection(
                tab: $0,
                record: mission,
                timeline: timeline
            )
        }
        let paginatedTimeline = LocalProductTimelinePage(
            schemaVersion: timeline.schemaVersion,
            teamInstanceID: timeline.teamInstanceID,
            viewVersion: timeline.viewVersion,
            nextCursor: "next-event-page",
            hasMore: true,
            gap: timeline.gap,
            records: timeline.records,
            board: timeline.board,
            attention: timeline.attention
        )
        let paginatedTeam = missionInspectorSection(
            tab: .team,
            record: mission,
            timeline: paginatedTimeline
        )

        XCTAssertTrue(paginatedTimeline.hasMore, "hasMore paginates events, not the board")
        XCTAssertEqual(paginatedTeam.rows.count, timeline.board.nodes.count)
        XCTAssertEqual(
            paginatedTeam.providerAccountRows.map(\.providerAccountID),
            timeline.board.providerAccounts.map(\.providerAccountID)
        )
        XCTAssertEqual(
            sections.map(\.heading),
            ["Team Pulse", "Plan", "Changes", "Evidence"]
        )
        XCTAssertEqual(Set(sections.map(\.rows)).count, 4)
        XCTAssertTrue(sections[0].rows.joined().contains("Attempt 1"))
        XCTAssertTrue(sections[0].rows[0].contains("Parallel provider route"))
        XCTAssertTrue(sections[0].rows[1].contains("Synthesis"))
        XCTAssertTrue(sections[0].rows.joined().contains("Claude-Code"))
        XCTAssertTrue(sections[0].rows.joined().contains("anthropic.production"))
        XCTAssertTrue(sections[0].rows.joined().contains("claude-sonnet"))
        XCTAssertTrue(sections[0].rows.joined().contains("Credential v7"))
        XCTAssertTrue(sections[0].rows.joined().contains("Timeout 300s"))
        XCTAssertTrue(sections[0].rows.joined().contains("Binding budget 1200"))
        XCTAssertTrue(sections[0].rows.joined().contains("workspace_edit"))
        XCTAssertTrue(sections[0].rows.joined().contains("Account policy v2 r3"))
        XCTAssertTrue(sections[0].rows.joined().contains("Trust External provider"))
        XCTAssertTrue(sections[0].rows.joined().contains("Retention Limited retention"))
        XCTAssertTrue(sections[0].rows.joined().contains("Region EU"))
        XCTAssertTrue(sections[0].rows.joined().contains("Fallback approved v3"))
        XCTAssertTrue(sections[0].rows.joined().contains("Incident 22222222"))
        XCTAssertTrue(sections[0].rows.joined().contains("Provider Auth"))
        XCTAssertTrue(sections[0].rows.joined().contains("Retry available"))
        XCTAssertTrue(sections[0].rows.joined().contains("Tests 2 passed, 1 failed"))
        XCTAssertTrue(sections[0].rows.joined().contains("Latest Go Test All Passed"))
        let accountRow = try XCTUnwrap(sections[0].providerAccountRows.first)
        XCTAssertEqual(accountRow.providerName, "Anthropic")
        XCTAssertEqual(accountRow.providerAccountID, "anthropic.production")
        XCTAssertEqual(accountRow.attempts, "1 attempt · 1 failed")
        XCTAssertEqual(accountRow.reliability, "100% error rate · 0 rate limited")
        XCTAssertEqual(accountRow.accountingCoverage, "1/1 attempts accounted")
        XCTAssertEqual(
            accountRow.tokens,
            "Input 90 · Output 10 · Cache read 20 · Cache write 5 · Total 100 · usage on 1/1 attempts"
        )
        XCTAssertEqual(
            accountRow.costSource,
            "Harness reported cost 0.00045 USD · cost on 1/1 attempts"
        )
        XCTAssertEqual(accountRow.policyRevision, "Revision 9")
        XCTAssertEqual(accountRow.concurrencyCeiling, "0 active / 3 maximum")
        XCTAssertEqual(accountRow.dispatchCeiling, "4 starts / 60s")
        XCTAssertEqual(accountRow.budgetCeiling, "100 active / 800 maximum")
        XCTAssertEqual(accountRow.completeness, "Incomplete accounting")
        XCTAssertTrue(accountRow.isIncomplete)
        XCTAssertFalse(accountRow.costSource.contains("450 microunits"))
        XCTAssertEqual(
            sections[0].systemImage(at: 0, fallback: "fallback"),
            "person.crop.circle"
        )
        XCTAssertEqual(
            sections[0].systemImage(at: 1, fallback: "fallback"),
            "arrow.triangle.merge"
        )
        XCTAssertEqual(
            sections[0].providerAccountRows.count,
            1
        )
        XCTAssertEqual(sections[0].accessibilityLabel(at: 0), "Agent status")
        XCTAssertEqual(sections[0].accessibilityLabel(at: 1), "Synthesis status")
        XCTAssertEqual(
            accountRow.accessibilityLabel,
            "Anthropic anthropic.production Provider Account governance, Incomplete accounting"
        )
        XCTAssertEqual(
            sections[0].incidentIDs.first,
            "22222222-2222-4222-8222-222222222222"
        )
        XCTAssertTrue(sections[0].canViewDiagnostics(at: 0))
        XCTAssertTrue(sections[0].canReviewRetry(at: 0))
        XCTAssertFalse(sections[0].canViewDiagnostics(at: 1))
        XCTAssertFalse(sections[0].canReviewRetry(at: 1))
        XCTAssertFalse(sections[0].canRecoverCredentialVault(at: 1))
        XCTAssertTrue(sections[0].rows[1].contains("openai.primary"))
        XCTAssertTrue(sections[0].rows[1].contains("Succeeded"))
        XCTAssertTrue(sections[0].rows[2].contains("Not started"))
        XCTAssertTrue(sections[0].rows[2].contains("Native auth"))
        XCTAssertFalse(sections[0].rows[2].contains("Attempt 0"))
        XCTAssertFalse(sections[0].rows[2].contains("Credential v0"))
        XCTAssertTrue(sections[0].canViewDiagnostics(at: 2))
        XCTAssertTrue(sections[0].canReviewRetry(at: 2))
        XCTAssertFalse(sections[0].canRecoverCredentialVault(at: 2))
        XCTAssertFalse(sections[1].canViewDiagnostics(at: 0))
        XCTAssertFalse(sections[1].canReviewRetry(at: 0))
        XCTAssertTrue(sections[1].rows.joined().contains("Main"))
        XCTAssertEqual(
            sections[2].rows,
            ["Verification recorded · Main · Attempt 1"]
        )
        XCTAssertEqual(
            sections[3].rows,
            ["Evidence 1 · Main · Attempt 1"]
        )
        for section in sections {
            let visible = section.rows.joined(separator: " ")
            XCTAssertFalse(visible.contains("internal-"))
            XCTAssertFalse(visible.contains(String(repeating: "a", count: 64)))
        }

        let unavailable = missionInspectorSection(
            tab: .evidence,
            record: mission,
            timeline: nil
        )
        XCTAssertEqual(unavailable.rows, [])
        XCTAssertFalse(unavailable.canViewDiagnostics(at: 0))
        XCTAssertFalse(unavailable.canReviewRetry(at: 0))
        XCTAssertEqual(
            unavailable.emptyMessage,
            "Evidence is unavailable until complete authoritative activity loads."
        )

        let encodedTimeline = try XCTUnwrap(
            String(
                data: JSONEncoder().encode(timeline),
                encoding: .utf8
            )
        )
        let incompleteTimeline = try LocalProductWire.decodeTimeline(
            Data(encodedTimeline.replacingOccurrences(
                of: "\"has_more\":false",
                with: "\"has_more\":true"
            ).utf8)
        )
        let gapTimeline = try LocalProductWire.decodeTimeline(
            Data(encodedTimeline.replacingOccurrences(
                of: "\"gap\":null",
                with: """
                \"gap\":{
                  \"schema_version\":1,
                  \"delivery_id\":\"stream-gap\",
                  \"kind\":\"stream_gap\",
                  \"team_instance_id\":\"team-1\",
                  \"reason\":\"cursor_stale\",
                  \"previous_cursor_digest\":\"\",
                  \"current_view_version\":\"view-1\",
                  \"artifact_available\":false,
                  \"artifact_digest\":\"\",
                  \"recoverable\":true,
                  \"occurred_at\":\"2026-08-03T00:00:02Z\"
                }
                """
            ).utf8)
        )
        for partialTimeline in [incompleteTimeline, gapTimeline] {
            for tab in [MissionInspectorTab.changes, .evidence] {
                let partial = missionInspectorSection(
                    tab: tab,
                    record: mission,
                    timeline: partialTimeline
                )
                XCTAssertEqual(partial.rows, [])
                XCTAssertTrue(
                    partial.emptyMessage.contains("unavailable"),
                    "\(tab) must not present incomplete history as empty"
                )
            }
        }
        let gapTeam = missionInspectorSection(
            tab: .team,
            record: mission,
            timeline: gapTimeline
        )
        XCTAssertEqual(gapTeam.rows.count, gapTimeline.board.nodes.count)
        XCTAssertTrue(gapTeam.rows.joined().contains("anthropic.production"))
        XCTAssertNotNil(gapTeam.notice)
        XCTAssertTrue(gapTeam.notice?.contains("Activity history is incomplete") == true)
    }

    func testVaultRecoveryActionIsLimitedToDurableVaultFailures() {
        let section = MissionInspectorSection(
            heading: "Team Pulse",
            rows: ["blocked", "healthy"],
            emptyMessage: "empty",
            credentialVaultRecoveryActions: [true, false]
        )
        XCTAssertTrue(section.canRecoverCredentialVault(at: 0))
        XCTAssertFalse(section.canRecoverCredentialVault(at: 1))
        XCTAssertFalse(section.canRecoverCredentialVault(at: 2))

        XCTAssertTrue(missionVaultRecoveryAvailable(
            stage: "vault_decrypt",
            diagnosticAvailable: true,
            retryable: false
        ))
        XCTAssertTrue(missionVaultRecoveryAvailable(
            stage: "vault_aad_validation",
            diagnosticAvailable: true,
            retryable: false
        ))
        XCTAssertFalse(missionVaultRecoveryAvailable(
            stage: "credential_lease_issue",
            diagnosticAvailable: true,
            retryable: true
        ))
        XCTAssertFalse(missionVaultRecoveryAvailable(
            stage: "provider_auth",
            diagnosticAvailable: true,
            retryable: false
        ))
        XCTAssertFalse(missionVaultRecoveryAvailable(
            stage: "vault_decrypt",
            diagnosticAvailable: false,
            retryable: false
        ))
    }

    func testProviderAccountGovernanceFormattingIsExactAndLocaleIndependent() {
        XCTAssertEqual(missionErrorRateLabel(basisPoints: 0), "0% error rate")
        XCTAssertEqual(missionErrorRateLabel(basisPoints: 1), "0.01% error rate")
        XCTAssertEqual(missionErrorRateLabel(basisPoints: 1_250), "12.5% error rate")
        XCTAssertEqual(missionErrorRateLabel(basisPoints: 10_000), "100% error rate")

        XCTAssertEqual(missionCostLabel(microunits: 0, currency: "USD"), "Cost 0 USD")
        XCTAssertEqual(missionCostLabel(microunits: 1, currency: "USD"), "Cost 0.000001 USD")
        XCTAssertEqual(missionCostLabel(microunits: 450, currency: "USD"), "Cost 0.00045 USD")
        XCTAssertEqual(missionCostLabel(microunits: 1_250_000, currency: "USD"), "Cost 1.25 USD")
		XCTAssertEqual(missionProviderPolicyValue("external_provider"), "External provider")
		XCTAssertEqual(missionProviderPolicyValue("zero_data_retention"), "Zero data retention")
		XCTAssertEqual(missionProviderPolicyValue("apac"), "APAC")
		XCTAssertEqual(
			missionCostSourceLabel(
				source: "rate_card_estimate", microunits: 450, currency: "USD"),
			"Rate-card estimate 0.00045 USD"
		)
        XCTAssertEqual(
            missionProviderAccountTokenSummary(
                inputTokens: 90,
                outputTokens: 10,
                cacheReadTokens: 0,
                cacheWriteTokens: 0,
                totalTokens: 100,
                usageAttemptCount: 1,
                attemptCount: 1
            ),
            "Input 90 · Output 10 · Total 100 · usage on 1/1 attempts"
        )
    }

    func testSideTaskDrawerAlwaysPresentsEveryContractField() throws {
        let digest = String(repeating: "a", count: 64)
        let sideTask = try JSONDecoder().decode(
            LocalProductSideTaskSummary.self,
            from: Data("""
            {
              "side_task_id":"side-1",
              "parent_mission_id":"mission/team-1",
              "parent_team_instance_id":"team-1",
              "parent_task_id":"work-1",
              "parent_run_id":"run-1",
              "parent_claim_generation":1,
              "parent_execution_digest":"\(digest)",
              "side_execution_team_instance_id":"team-side-1",
              "purpose":"diagnosis",
              "mode":"decision_required",
              "title":"Diagnose the failure",
              "status":"decision_required",
              "source_generation":1,
              "handoff_version":1,
              "handoff_digest":"\(digest)",
              "summary_artifact_digest":"\(digest)",
              "what_happened":"Authorized result",
              "authorized_findings":[],
              "evidence_references":[],
              "artifact_references":[],
              "risk":"medium",
              "uncertainties":[],
              "scope_delta":[],
              "decision_options":["absorb","discard"],
              "recommended_option":"",
              "recommendation_authority":"proposal_only",
              "usage_observed":false,
              "usage_microunits":0,
              "usage_currency":"",
              "decision_deadline":"2026-08-03T16:00:00Z",
              "available_decisions":["absorb","discard"],
              "effect_status":"none"
            }
            """.utf8)
        )

        let presentation = sideTaskDrawerPresentation(sideTask)
        XCTAssertEqual(presentation.purpose, "Purpose · Diagnosis")
        XCTAssertEqual(
            presentation.uncertainty,
            "Uncertainty · None recorded"
        )
        XCTAssertEqual(
            presentation.scopeDelta,
            "Scope · No parent scope expansion"
        )
        XCTAssertEqual(
            presentation.nextAction,
            "Next action · Choose an explicit parent decision"
        )

        var decidedObject = try XCTUnwrap(
            JSONSerialization.jsonObject(
                with: JSONEncoder().encode(sideTask)
            ) as? [String: Any]
        )
        decidedObject["status"] = "decided"
        decidedObject["available_decisions"] = []

        decidedObject["effect_status"] = "pending"
        let pending = try JSONDecoder().decode(
            LocalProductSideTaskSummary.self,
            from: JSONSerialization.data(withJSONObject: decidedObject)
        )
        XCTAssertEqual(
            sideTaskDrawerPresentation(pending).nextAction,
            "Next action · Wait for the authorized parent effect"
        )

        decidedObject["effect_status"] = "completed"
        let completed = try JSONDecoder().decode(
            LocalProductSideTaskSummary.self,
            from: JSONSerialization.data(withJSONObject: decidedObject)
        )
        XCTAssertEqual(
            sideTaskDrawerPresentation(completed).nextAction,
            "Next action · Parent effect completed"
        )

        decidedObject["effect_status"] = "none"
        let noEffect = try JSONDecoder().decode(
            LocalProductSideTaskSummary.self,
            from: JSONSerialization.data(withJSONObject: decidedObject)
        )
        XCTAssertEqual(
            sideTaskDrawerPresentation(noEffect).nextAction,
            "Next action · Parent decision recorded; no parent effect required"
        )
    }

    func testMissionWorkspaceSnapshotPresentationFailsClosed() {
        XCTAssertEqual(
            missionSnapshotPresentation(
                connection: .online,
                hasSnapshot: true
            ),
            .current
        )
        XCTAssertEqual(
            missionSnapshotPresentation(
                connection: .partial(reason: "partial_view"),
                hasSnapshot: true
            ),
            .partial("partial_view")
        )
        XCTAssertEqual(
            missionSnapshotPresentation(
                connection: .stale(reason: "stale_view"),
                hasSnapshot: true
            ),
            .preserved("stale_view")
        )
        XCTAssertEqual(
            missionSnapshotPresentation(
                connection: .offline(reason: "state_unavailable"),
                hasSnapshot: true
            ),
            .preserved("state_unavailable")
        )
        XCTAssertEqual(
            missionSnapshotPresentation(
                connection: .online,
                hasSnapshot: false
            ),
            .unavailable("state_unavailable")
        )

        let current = missionAttentionEmptyCopy(presentation: .current)
        XCTAssertEqual(current.title, "Nothing needs you")
        let partial = missionAttentionEmptyCopy(
            presentation: .partial("partial_view")
        )
        XCTAssertEqual(partial.title, "No recorded attention in this view")
        XCTAssertTrue(partial.detail.contains("Some items may be missing"))
        let preserved = missionAttentionEmptyCopy(
            presentation: .preserved("state_unavailable")
        )
        XCTAssertEqual(
            preserved.title,
            "No recorded attention in this view"
        )
        XCTAssertTrue(preserved.detail.contains("New items may be missing"))
    }

    func testMinimumLayoutHasNoRequiredMotion() async throws {
        let snapshot = try ExperienceFixtures.populatedSnapshot()
        let store = LocalProductStore(
            client: ExperienceViewStubClient(results: [.success(snapshot)])
        )
        await store.refresh()
        let standard = try XCTUnwrap(
            render(
                ContentView(store: store, refreshOnAppear: false),
                colorScheme: .light,
                dynamicTypeSize: .large,
                width: 720,
                height: 560
            )
        )
        XCTAssertEqual(standard.pixelWidth, 720)
        XCTAssertEqual(standard.pixelHeight, 560)
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .appendingPathComponent(
                "../../Sources/LoomLocalAppUI/ContentView.swift"
            )
            .standardizedFileURL
        let source = try String(contentsOf: sourceURL, encoding: .utf8)
        for requiredMotion in ["withAnimation", ".animation(", "TimelineView"] {
            XCTAssertFalse(
                source.contains(requiredMotion),
                "Primary workspace requires motion through \(requiredMotion)"
            )
        }
    }

    func testRealContentViewRendersNonconnectingStateAndAppearanceMatrix()
        async throws
    {
        let populated = try ExperienceFixtures.populatedSnapshot()
        let partial = try ExperienceFixtures.snapshot(partial: true)
        let stale = try ExperienceFixtures.snapshot(stale: true)

        let fixtures: [ExperienceViewFixture] = [
            .loading(),
            .connected(snapshot: .empty(viewVersion: "empty")),
            .connected(snapshot: populated),
            .connected(snapshot: partial),
            .connected(snapshot: stale),
            .offline(),
            .offlinePreserving(populated),
            .fatal(),
        ]

        var stateDigests = Set<String>()
        var digestOwners: [String: String] = [:]
        for fixture in fixtures {
            let store = LocalProductStore(client: fixture.client)
            for _ in 0..<fixture.refreshCount {
                await store.refresh()
            }
            XCTAssertEqual(
                LocalProductExperience(
                    snapshot: store.snapshot,
                    connectionState: store.connectionState
                ).state,
                fixture.expectedState
            )
            for scheme in [ColorScheme.light, .dark] {
                let rendered = render(
                    ContentView(
                        store: store,
                        refreshOnAppear: false
                    ),
                    colorScheme: scheme,
                    dynamicTypeSize: .large
                )
                let image = try XCTUnwrap(
                    rendered,
                    "\(fixture.name) failed in \(scheme)"
                )
                XCTAssertEqual(image.pixelWidth, 1_100)
                XCTAssertEqual(image.pixelHeight, 720)
                XCTAssertGreaterThan(image.png.count, 20_000)
                XCTAssertGreaterThan(image.colorBucketCount, 12)
                XCTAssertGreaterThan(
                    image.sidebarContrastPixelCount,
                    180,
                    "\(fixture.name) rendered a visually empty sidebar in \(scheme)"
                )
                let owner = "\(fixture.name)-\(scheme)"
                if let previous = digestOwners[image.digest] {
                    XCTFail(
                        "duplicate rendered state \(owner) matches \(previous)"
                    )
                } else {
                    digestOwners[image.digest] = owner
                    stateDigests.insert(image.digest)
                }
            }
        }
        XCTAssertEqual(stateDigests.count, fixtures.count * 2)

        let accessibilityStore = LocalProductStore(
            client: ExperienceViewStubClient(results: [.success(populated)])
        )
        await accessibilityStore.refresh()
        let accessibilityImage = try XCTUnwrap(
            render(
                ContentView(
                    store: accessibilityStore,
                    refreshOnAppear: false
                ),
                colorScheme: .light,
                dynamicTypeSize: .accessibility3,
                width: 780
            )
        )
        XCTAssertEqual(accessibilityImage.pixelWidth, 780)
        XCTAssertEqual(accessibilityImage.pixelHeight, 720)
        XCTAssertGreaterThan(accessibilityImage.png.count, 20_000)
        XCTAssertGreaterThan(accessibilityImage.colorBucketCount, 12)
        XCTAssertGreaterThan(
            accessibilityImage.sidebarContrastPixelCount,
            180
        )
        XCTAssertFalse(stateDigests.contains(accessibilityImage.digest))
    }

    func testExportPopulatedPreviewOnlyAtAuthorizedEvidencePath()
        async throws
    {
        guard let requestedPath = ProcessInfo.processInfo.environment[
            "LOOM_UI_PREVIEW_PATH"
        ], let compactRequestedPath = ProcessInfo.processInfo.environment[
            "LOOM_UI_COMPACT_PREVIEW_PATH"
        ], let darkRequestedPath = ProcessInfo.processInfo.environment[
            "LOOM_UI_DARK_PREVIEW_PATH"
        ], let darkCompactRequestedPath = ProcessInfo.processInfo.environment[
            "LOOM_UI_DARK_COMPACT_PREVIEW_PATH"
        ], let missionRoomRequestedPath = ProcessInfo.processInfo.environment[
            "LOOM_UI_MISSION_ROOM_PATH"
        ] else {
            throw XCTSkip(
                "Wide and compact preview export is enabled only for visual audit"
            )
        }
        let requestedURL = try validatedPreviewURL(
            requestedPath: requestedPath,
            expectedURL: expectedPreviewURL(
                currentDirectory: FileManager.default.currentDirectoryPath
            )
        )
        let compactRequestedURL = try validatedPreviewURL(
            requestedPath: compactRequestedPath,
            expectedURL: expectedCompactPreviewURL(
                currentDirectory: FileManager.default.currentDirectoryPath
            )
        )
        let darkRequestedURL = try validatedPreviewURL(
            requestedPath: darkRequestedPath,
            expectedURL: expectedDarkPreviewURL(
                currentDirectory: FileManager.default.currentDirectoryPath
            )
        )
        let darkCompactRequestedURL = try validatedPreviewURL(
            requestedPath: darkCompactRequestedPath,
            expectedURL: expectedDarkCompactPreviewURL(
                currentDirectory: FileManager.default.currentDirectoryPath
            )
        )
        let missionRoomRequestedURL = try validatedPreviewURL(
            requestedPath: missionRoomRequestedPath,
            expectedURL: expectedMissionRoomPreviewURL(
                currentDirectory: FileManager.default.currentDirectoryPath
            )
        )

        let snapshot = try LocalProductWire.decodeSnapshot(
            Data(MissionOrchestrationTests.snapshotJSON.utf8)
        )
        let store = LocalProductStore(
            client: ExperienceViewStubClient(results: [.success(snapshot)])
        )
        await store.refresh()
        guard let rendered = render(
            ContentView(
                store: store,
                refreshOnAppear: false
            ),
            colorScheme: .light,
            dynamicTypeSize: .large
        ) else {
            return XCTFail("Could not encode the non-connecting preview")
        }
        try rendered.png.write(to: requestedURL, options: .atomic)
        XCTAssertGreaterThan(rendered.png.count, 20_000)
        let compact = try XCTUnwrap(
            render(
                ContentView(
                    store: store,
                    refreshOnAppear: false
                ),
                colorScheme: .light,
                dynamicTypeSize: .large,
                width: 780,
                height: 720
            )
        )
        try compact.png.write(to: compactRequestedURL, options: .atomic)
        XCTAssertGreaterThan(compact.png.count, 20_000)
        XCTAssertNotEqual(rendered.digest, compact.digest)
        let dark = try XCTUnwrap(
            render(
                ContentView(store: store, refreshOnAppear: false),
                colorScheme: .dark,
                dynamicTypeSize: .large
            )
        )
        try dark.png.write(to: darkRequestedURL, options: .atomic)
        let darkCompact = try XCTUnwrap(
            render(
                ContentView(store: store, refreshOnAppear: false),
                colorScheme: .dark,
                dynamicTypeSize: .large,
                width: 780,
                height: 720
            )
        )
        try darkCompact.png.write(
            to: darkCompactRequestedURL,
            options: .atomic
        )
        XCTAssertNotEqual(dark.digest, rendered.digest)
        XCTAssertNotEqual(darkCompact.digest, compact.digest)
        store.openMission("mission/team-1")
        let missionRoom = try XCTUnwrap(
            render(
                ContentView(store: store, refreshOnAppear: false),
                colorScheme: .light,
                dynamicTypeSize: .large
            )
        )
        try missionRoom.png.write(
            to: missionRoomRequestedURL,
            options: .atomic
        )
        XCTAssertNotEqual(missionRoom.digest, rendered.digest)
    }

    private func render<Root: View>(
        _ view: Root,
        colorScheme: ColorScheme,
        dynamicTypeSize: DynamicTypeSize,
        width: Int = 1_100,
        height: Int = 720
    ) -> RenderedPreview? {
        let rootView = view
                .frame(width: CGFloat(width), height: CGFloat(height))
                .environment(\.colorScheme, colorScheme)
                .environment(\.dynamicTypeSize, dynamicTypeSize)
        let hostingView = NSHostingView(rootView: rootView)
        hostingView.frame = NSRect(
            origin: .zero,
            size: CGSize(width: width, height: height)
        )
        hostingView.layoutSubtreeIfNeeded()
        guard let bitmap = NSBitmapImageRep(
            bitmapDataPlanes: nil,
            pixelsWide: width,
            pixelsHigh: height,
            bitsPerSample: 8,
            samplesPerPixel: 4,
            hasAlpha: true,
            isPlanar: false,
            colorSpaceName: .deviceRGB,
            bytesPerRow: 0,
            bitsPerPixel: 0
        ) else {
            return nil
        }
        bitmap.size = hostingView.bounds.size
        hostingView.cacheDisplay(
            in: hostingView.bounds,
            to: bitmap
        )
        guard let png = bitmap.representation(
            using: .png,
            properties: [:]
        ) else {
            return nil
        }
        guard bitmap.pixelsWide == width,
              bitmap.pixelsHigh == height else {
            return nil
        }
        var buckets = Set<UInt32>()
        var sidebarContrastPixelCount = 0
        let step = 12
        for y in stride(from: 0, to: bitmap.pixelsHigh, by: step) {
            for x in stride(from: 0, to: bitmap.pixelsWide, by: step) {
                guard let color = bitmap.colorAt(x: x, y: y)?
                    .usingColorSpace(.deviceRGB) else {
                    continue
                }
                let red = UInt32((color.redComponent * 31).rounded())
                let green = UInt32((color.greenComponent * 31).rounded())
                let blue = UInt32((color.blueComponent * 31).rounded())
                let alpha = UInt32((color.alphaComponent * 31).rounded())
                buckets.insert(
                    red << 15 | green << 10 | blue << 5 | alpha
                )
            }
        }
        if let reference = bitmap.colorAt(x: 24, y: 100)?
            .usingColorSpace(.deviceRGB) {
            for y in stride(from: 100, to: min(620, height), by: 2) {
                for x in stride(from: 24, to: min(190, width), by: 2) {
                    guard let color = bitmap.colorAt(x: x, y: y)?
                        .usingColorSpace(.deviceRGB) else {
                        continue
                    }
                    let contrast =
                        abs(color.redComponent - reference.redComponent) +
                        abs(color.greenComponent - reference.greenComponent) +
                        abs(color.blueComponent - reference.blueComponent)
                    if contrast > 0.24 {
                        sidebarContrastPixelCount += 1
                    }
                }
            }
        }
        return RenderedPreview(
            png: png,
            digest: SHA256.hash(data: png)
                .map { String(format: "%02x", $0) }
                .joined(),
            colorBucketCount: buckets.count,
            sidebarContrastPixelCount: sidebarContrastPixelCount,
            pixelWidth: bitmap.pixelsWide,
            pixelHeight: bitmap.pixelsHigh
        )
    }

    private func validatedPreviewURL(
        requestedPath: String,
        expectedURL: URL
    ) throws -> URL {
        let requestedURL = URL(fileURLWithPath: requestedPath)
            .standardizedFileURL
        let standardizedExpectedURL = expectedURL.standardizedFileURL
        guard requestedURL == standardizedExpectedURL else {
            throw PreviewPathError.outsideOwnedEvidence
        }
        let expectedParent = standardizedExpectedURL
            .deletingLastPathComponent()
        guard expectedParent == expectedParent
            .resolvingSymlinksInPath()
            .standardizedFileURL else {
            throw PreviewPathError.symlinkEscape
        }
        if let values = try? requestedURL.resourceValues(
            forKeys: [.isSymbolicLinkKey]
        ), values.isSymbolicLink == true {
            throw PreviewPathError.symlinkEscape
        }
        return requestedURL
    }

    private func expectedPreviewURL(currentDirectory: String) -> URL {
        URL(fileURLWithPath: currentDirectory)
            .appendingPathComponent(
                "../../.loom-evidence/phase2a/P2A-W2/" +
                    "mission-workbench-wide-light.png"
            )
            .standardizedFileURL
    }

    private func expectedCompactPreviewURL(
        currentDirectory: String
    ) -> URL {
        URL(fileURLWithPath: currentDirectory)
            .appendingPathComponent(
                "../../.loom-evidence/phase2a/P2A-W2/" +
                    "mission-workbench-compact-light.png"
            )
            .standardizedFileURL
    }

    private func expectedDarkPreviewURL(currentDirectory: String) -> URL {
        URL(fileURLWithPath: currentDirectory)
            .appendingPathComponent(
                "../../.loom-evidence/phase2a/P2A-W2/" +
                    "mission-workbench-wide-dark.png"
            )
            .standardizedFileURL
    }

    private func expectedDarkCompactPreviewURL(
        currentDirectory: String
    ) -> URL {
        URL(fileURLWithPath: currentDirectory)
            .appendingPathComponent(
                "../../.loom-evidence/phase2a/P2A-W2/" +
                    "mission-workbench-compact-dark.png"
            )
            .standardizedFileURL
    }

    private func expectedMissionRoomPreviewURL(
        currentDirectory: String
    ) -> URL {
        URL(fileURLWithPath: currentDirectory)
            .appendingPathComponent(
                "../../.loom-evidence/phase2a/P2A-W2/" +
                    "mission-room-wide-light.png"
            )
            .standardizedFileURL
    }

    private func expectedProviderPreviewURL(
        currentDirectory: String
    ) -> URL {
        URL(fileURLWithPath: currentDirectory)
            .appendingPathComponent(
                "../../.loom-evidence/phase2a/P2A-W2/" +
                    "provider-connection-directory.png"
            )
            .standardizedFileURL
    }

    func testPreviewPathValidationRejectsMismatchAndSymlink() throws {
        let currentDirectory = FileManager.default.currentDirectoryPath
        XCTAssertThrowsError(
            try validatedPreviewURL(
                requestedPath: "/private/tmp/not-owned.png",
                expectedURL: expectedPreviewURL(
                    currentDirectory: currentDirectory
                )
            )
        )

        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(
            at: root,
            withIntermediateDirectories: false
        )
        defer { try? FileManager.default.removeItem(at: root) }
        let outside = root.appendingPathComponent("outside.png")
        let expected = root.appendingPathComponent("expected.png")
        try FileManager.default.createSymbolicLink(
            at: expected,
            withDestinationURL: outside
        )
        defer { try? FileManager.default.removeItem(at: expected) }
        XCTAssertThrowsError(
            try validatedPreviewURL(
                requestedPath: expected.path,
                expectedURL: expected
            )
        )

        let outsideDirectory = root.appendingPathComponent("outside")
        try FileManager.default.createDirectory(
            at: outsideDirectory,
            withIntermediateDirectories: false
        )
        let linkedParent = root.appendingPathComponent("linked-parent")
        try FileManager.default.createSymbolicLink(
            at: linkedParent,
            withDestinationURL: outsideDirectory
        )
        let parentEscaped = linkedParent.appendingPathComponent("preview.png")
        XCTAssertThrowsError(
            try validatedPreviewURL(
                requestedPath: parentEscaped.path,
                expectedURL: parentEscaped
            )
        )
    }
}

private struct RenderedPreview {
    let png: Data
    let digest: String
    let colorBucketCount: Int
    let sidebarContrastPixelCount: Int
    let pixelWidth: Int
    let pixelHeight: Int
}

private enum PreviewPathError: Error {
    case outsideOwnedEvidence
    case symlinkEscape
}

private struct ExperienceViewFixture {
    let name: String
    let client: ExperienceViewStubClient
    let refreshCount: Int
    let expectedState: LocalProductExperienceState

    static func loading() -> Self {
        Self(
            name: "loading",
            client: ExperienceViewStubClient(
                results: [.success(.empty(viewVersion: "loading"))]
            ),
            refreshCount: 0,
            expectedState: .loading
        )
    }

    static func connected(snapshot: LocalProductSnapshot) -> Self {
        Self(
            name: snapshot.partial
                ? "partial"
                : snapshot.stale ? "stale" : "connected",
            client: ExperienceViewStubClient(results: [.success(snapshot)]),
            refreshCount: 1,
            expectedState: snapshot.partial
                ? .partial
                : snapshot.stale
                    ? .stalePreserved
                    : snapshot == .empty(viewVersion: "empty")
                        ? .connectedEmpty
                        : .connectedPopulated
        )
    }

    static func offline() -> Self {
        Self(
            name: "offline",
            client: ExperienceViewStubClient(
                results: [.failure(LocalProductClientError.unavailable)]
            ),
            refreshCount: 1,
            expectedState: .offline
        )
    }

    static func offlinePreserving(_ snapshot: LocalProductSnapshot) -> Self {
        Self(
            name: "offline-preserved",
            client: ExperienceViewStubClient(
                results: [
                    .success(snapshot),
                    .failure(LocalProductClientError.unavailable),
                ]
            ),
            refreshCount: 2,
            expectedState: .offlinePreserved
        )
    }

    static func fatal() -> Self {
        Self(
            name: "fatal",
            client: ExperienceViewStubClient(
                results: [
                    .failure(
                        LocalIPCRemoteError(
                            code: .unauthorizedPeer,
                            recoverable: false
                        )
                    ),
                ]
            ),
            refreshCount: 1,
            expectedState: .fatal
        )
    }
}

private final class ExperienceViewStubClient: LocalProductClientProtocol {
    private var results: [Result<LocalProductSnapshot, Error>]

    init(results: [Result<LocalProductSnapshot, Error>]) {
        self.results = results
    }

    func snapshot(limit: Int) async throws -> LocalProductSnapshot {
        guard !results.isEmpty else {
            throw LocalProductClientError.unavailable
        }
        return try results.removeFirst().get()
    }

    func timeline(
        teamInstanceID: String,
        cursor: String,
        limit: Int
    ) async throws -> LocalProductTimelinePage {
        throw LocalProductClientError.notFound
    }
}
