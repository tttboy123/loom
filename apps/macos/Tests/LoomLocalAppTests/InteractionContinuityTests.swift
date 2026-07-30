import XCTest
@testable import LoomLocalAppCore

final class InteractionContinuityTests: XCTestCase {
    func testWorkspaceStartsWithBlankTaskInsteadOfDashboard() {
        let workspace = LocalProductWorkspaceState()

        XCTAssertEqual(
            workspace.selectedTaskID,
            LocalProductWorkspaceState.newTaskID
        )
        XCTAssertEqual(workspace.tasks.first?.kind, .draft)
        XCTAssertEqual(workspace.tasks.first?.title, "New task")
    }

    func testTaskSwitchPreservesIndependentComposerAndInspectorState() {
        var workspace = LocalProductWorkspaceState()
        let draftID = workspace.selectedTaskID
        workspace.updateComposerDraft("Review the release candidate")
        workspace.selectInspector(.team)
        workspace.updateThreadAnchor("builder-purpose")
        workspace.setDeveloperDetailsExpanded(true)

        let history = LocalProductWorkspaceTask(
            id: "history:release",
            title: "Release review",
            subtitle: "Ready",
            kind: .history
        )
        workspace.mergeAuthoritativeTasks([history])
        workspace.selectTask(history.id)
        workspace.updateComposerDraft("Follow up on verification")
        workspace.selectInspector(.evidence)
        workspace.updateThreadAnchor("terminal")
        workspace.setDeveloperDetailsExpanded(false)

        workspace.selectTask(draftID)
        XCTAssertEqual(
            workspace.selectedContinuity.composerDraft,
            "Review the release candidate"
        )
        XCTAssertEqual(workspace.selectedContinuity.inspector, .team)
        XCTAssertEqual(
            workspace.selectedContinuity.threadAnchor,
            "builder-purpose"
        )
        XCTAssertTrue(workspace.selectedContinuity.developerDetailsExpanded)

        workspace.selectTask(history.id)
        XCTAssertEqual(
            workspace.selectedContinuity.composerDraft,
            "Follow up on verification"
        )
        XCTAssertEqual(workspace.selectedContinuity.inspector, .evidence)
        XCTAssertEqual(workspace.selectedContinuity.threadAnchor, "terminal")
        XCTAssertFalse(workspace.selectedContinuity.developerDetailsExpanded)
    }

    func testPrimaryFlowCopyExcludesInternalAuthorityLanguage() {
        let forbidden = [
            "Candidate",
            "TeamDefinition",
            "TeamInstance",
            "AgentInstance",
            "Runtime instance ID",
            "stream head",
            "stream revision",
            "catalog digest",
            "binding digest",
            "credential reference",
            "source-ordered",
            "Journal authority",
            "Projection authority",
        ]

        for value in LocalProductInteractionCopy.primaryFlow {
            for term in forbidden {
                XCTAssertFalse(
                    value.localizedCaseInsensitiveContains(term),
                    "\(value) exposes \(term)"
                )
            }
        }
    }

    func testTaskFilterKeepsCurrentTaskVisible() {
        var workspace = LocalProductWorkspaceState()
        let alpha = LocalProductWorkspaceTask(
            id: "team:alpha",
            title: "Alpha review",
            subtitle: "Ready",
            kind: .team
        )
        let beta = LocalProductWorkspaceTask(
            id: "team:beta",
            title: "Beta migration",
            subtitle: "Ready",
            kind: .team
        )
        workspace.mergeAuthoritativeTasks([alpha, beta])
        workspace.selectTask(alpha.id)

        XCTAssertEqual(
            workspace.filteredTasks(matching: "beta").map(\.id),
            [alpha.id, beta.id]
        )
    }

    @MainActor
    func testPreflightReviewIncludesBoundProviderModelAndLimits() throws {
        let session = try LocalProductSetupWire.decodeBuilderSession(
            Data(
                """
                {
                  "schema_version":1,
                  "draft_id":"draft-1",
                  "revision":5,
                  "source":"blank",
                  "catalog_digest":"",
                  "view_version":"",
                  "content_digest":"",
                  "binding_digest":"",
                  "question":{"id":"","prompt":"","options":[]},
                  "preview":{
                    "name":"Release team",
                    "purpose":"Review the release",
                    "roles":[{
                      "kind":"main",
                      "agent_definition_id":"agent-main",
                      "agent_version":1,
                      "agent_scope":"project",
                      "display_name":"Coordinator",
                      "responsibility":"Coordinate",
                      "runtime":{
                        "runtime_instance_id":"runtime-pi",
                        "display_name":"Pi Coding Agent",
                        "adapter_type":"pi",
                        "executable_version":"0.82.1",
                        "status":"online",
                        "capacity":1,
                        "model_id":"codex-model",
                        "model_ids":["codex-model"],
                        "observed_capabilities":[],
                        "source_probe_id":"probe"
                      },
                      "runtime_profile_id":"profile",
                      "model_id":"codex-model",
                      "auth_mode":"native_auth",
                      "skills":[],
                      "permission_ids":["repo.read"],
                      "resource_ids":[],
                      "compatible":true,
                      "compatibility_reason":""
                    }],
                    "permissions":["repo.read"],
                    "resources":[],
                    "compatibility_gaps":[],
                    "requested_concurrency":1,
                    "maximum_budget_credits":200,
                    "estimated_maximum_cost":"Up to 2 credits"
                  },
                  "can_confirm":true
                }
                """.utf8
            )
        )
        let review = LocalProductPreflightReview(preview: session.preview)

        XCTAssertEqual(review.roles.first?.provider, "Codex")
        XCTAssertEqual(review.roles.first?.model, "codex-model")
        XCTAssertEqual(review.roles.first?.auth, "Native auth")
        XCTAssertEqual(review.permissions, ["Repo read"])
        XCTAssertEqual(review.compatibility, "Compatible")
        XCTAssertEqual(review.maximumCost, "Up to 2 credits")
        XCTAssertEqual(review.maximumBudget, "200 credits")

        let setup = try LocalProductSetupWire.decodeSnapshot(
            Data(
                """
                {
                  "schema_version":1,
                  "view_version":"view-1",
                  "codex":{
                    "provider_id":"codex",
                    "auth_mode":"native_auth",
                    "credential_reference":"",
                    "revision":0,
                    "status":"available",
                    "reason":""
                  },
                  "minimax":{
                    "provider_id":"minimax",
                    "auth_mode":"brokered",
                    "credential_reference":"",
                    "revision":0,
                    "status":"unconfigured",
                    "reason":""
                  },
                  "runtimes":[{
                    "runtime_instance_id":"runtime-pi",
                    "display_name":"Pi Coding Agent",
                    "adapter_type":"pi",
                    "executable_version":"0.82.1",
                    "status":"online",
                    "capacity":1,
                    "model_id":"codex-model",
                    "model_ids":["codex-model"],
                    "observed_capabilities":[],
                    "source_probe_id":"probe"
                  },{
                    "runtime_instance_id":"runtime-alt",
                    "display_name":"Alternate Runtime",
                    "adapter_type":"pi",
                    "executable_version":"0.82.1",
                    "status":"online",
                    "capacity":1,
                    "model_id":"alternate-model",
                    "model_ids":["alternate-model"],
                    "observed_capabilities":[],
                    "source_probe_id":"probe"
                  }],
                  "saved_teams":[],
                  "templates":[],
                  "role_options":[{
                    "id":"coordinator",
                    "kind":"main",
                    "agent_definition_id":"agent-main",
                    "runtime_profile_id":"profile",
                    "runtime_instance_id":"runtime-pi",
                    "skill_revision_ids":[],
                    "permission_ids":["repo.read"],
                    "resource_ids":[],
                    "responsibility":"Coordinate"
                  },{
                    "id":"reviewer",
                    "kind":"main",
                    "agent_definition_id":"agent-main",
                    "runtime_profile_id":"profile-alt",
                    "runtime_instance_id":"runtime-alt",
                    "skill_revision_ids":[],
                    "permission_ids":[],
                    "resource_ids":[],
                    "responsibility":"Review"
                  }],
                  "skills":[],
                  "permissions":["repo.read"],
                  "resources":[]
                }
                """.utf8
            )
        )
        let choices = LocalProductRoleOptionChoice.all(
            setup: setup,
            preview: session.preview
        )
        let choice = try XCTUnwrap(
            choices.first(where: { $0.id == "coordinator" })
        )
        XCTAssertEqual(choice.field, "main_role")
        XCTAssertEqual(choice.provider, "Codex")
        XCTAssertEqual(choice.model, "codex-model")
        XCTAssertEqual(choice.auth, "Native auth")
        XCTAssertTrue(choice.isCurrent)
        let alternate = try XCTUnwrap(
            choices.first(where: { $0.id == "reviewer" })
        )
        XCTAssertEqual(alternate.model, "alternate-model")
        XCTAssertEqual(alternate.provider, "")
        XCTAssertEqual(alternate.auth, "")
        XCTAssertFalse(alternate.isCurrent)
        XCTAssertTrue(LocalProductStore.allowsBuilderEditField("main_role"))
        XCTAssertTrue(
            LocalProductStore.allowsBuilderEditField("subagent_role")
        )
    }
}
