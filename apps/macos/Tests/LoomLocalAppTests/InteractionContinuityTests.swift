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

    func testConversationThreadIdentityIsStableAndIsolatedPerTask() {
        var workspace = LocalProductWorkspaceState()
        let draftThread = workspace.currentThreadID()

        let history = LocalProductWorkspaceTask(
            id: "history:release",
            title: "Release review",
            subtitle: "Ready",
            kind: .history
        )
        workspace.mergeAuthoritativeTasks([history])
        workspace.selectTask(history.id)
        let historyThread = workspace.currentThreadID()

        XCTAssertTrue(draftThread.hasPrefix("thread-"))
        XCTAssertTrue(historyThread.hasPrefix("thread-"))
        XCTAssertNotEqual(draftThread, historyThread)

        workspace.selectTask(LocalProductWorkspaceState.newTaskID)
        XCTAssertEqual(workspace.currentThreadID(), draftThread)
    }

    func testConversationProfileSelectionIsIsolatedPerTask() {
        var workspace = LocalProductWorkspaceState()
        workspace.selectConversationProfile(
            "conversation-deepseek-deepseek-chat-r2"
        )
        XCTAssertEqual(
            workspace.selectedContinuity.conversationProfileID,
            "conversation-deepseek-deepseek-chat-r2"
        )

        workspace.mergeAuthoritativeTasks([
            LocalProductWorkspaceTask(
                id: "task-second",
                title: "Second",
                subtitle: "Conversation",
                kind: .draft
            ),
        ])
        workspace.selectTask("task-second")
        XCTAssertEqual(workspace.selectedContinuity.conversationProfileID, "")
    }

    func testFolderSelectionKeepsOnlySafeDisplayHandle() {
        var workspace = LocalProductWorkspaceState()

        workspace.selectFolderDisplayName("  loom-pi-rebuild\n/private/path  ")

        XCTAssertEqual(workspace.selectedFolderDisplayName, "loom-pi-rebuild private path")
        XCTAssertFalse(workspace.selectedFolderDisplayName?.contains("/") ?? true)
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
                      "harness_adapter":"pi",
                      "provider_id":"openai",
                      "provider_account_id":"openai.primary",
                      "model_id":"codex-model",
                      "auth_mode":"brokered",
                      "credential_revision":3,
                      "reasoning_effort":"high",
                      "timeout_milliseconds":45000,
                      "budget_available":true,
                      "budget_units":80,
                      "required_capabilities":["reasoning_effort","workspace.edit"],
                      "fallback_configured":true,
                      "fallback_runtime_profile_id":"profile-alt",
                      "fallback_harness_adapter":"pi",
                      "fallback_provider_id":"anthropic",
                      "fallback_provider_account_id":"anthropic.review",
                      "fallback_auth_mode":"brokered",
                      "fallback_model_id":"claude-sonnet",
                      "fallback_credential_revision":7,
                      "fallback_reasoning_effort":"low",
                      "fallback_timeout_milliseconds":30000,
                      "fallback_budget_available":true,
                      "fallback_budget_units":40,
                      "fallback_required_capabilities":["reasoning_effort","rpc"],
                      "fallback_approval_required":true,
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

		XCTAssertEqual(review.roles.first?.provider, "OpenAI")
		XCTAssertEqual(review.roles.first?.providerAccount, "openai.primary")
		XCTAssertEqual(review.roles.first?.harness, "Pi")
		XCTAssertEqual(review.roles.first?.model, "codex-model")
		XCTAssertEqual(review.roles.first?.auth, "Brokered")
		XCTAssertEqual(review.roles.first?.credential, "Credential v3")
		XCTAssertEqual(review.roles.first?.reasoning, "High reasoning")
		XCTAssertEqual(review.roles.first?.timeout, "45s timeout")
		XCTAssertEqual(review.roles.first?.budget, "80 units")
		XCTAssertEqual(
			review.roles.first?.fallback,
			"Pi · Anthropic / anthropic.review · claude-sonnet"
		)
		XCTAssertEqual(review.roles.first?.fallbackApproval, "Approval required")
		XCTAssertEqual(review.roles.first?.fallbackAuth, "Brokered")
		XCTAssertEqual(review.roles.first?.fallbackCredential, "Credential v7")
		XCTAssertEqual(review.roles.first?.fallbackReasoning, "Low reasoning")
		XCTAssertEqual(review.roles.first?.fallbackTimeout, "30s timeout")
		XCTAssertEqual(review.roles.first?.fallbackBudget, "40 units")
		XCTAssertEqual(
			review.roles.first?.fallbackCapabilities,
			["Reasoning effort", "Rpc"]
		)
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
                    "model_ids":["codex-model","claude-sonnet"],
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
                    "harness_adapter":"pi",
                    "provider_id":"openai",
                    "provider_account_id":"openai.primary",
                    "model_id":"codex-model",
                    "auth_mode":"brokered",
                    "credential_revision":3,
                    "reasoning_effort":"high",
                    "timeout_milliseconds":45000,
                    "budget_available":true,
                    "budget_units":80,
                    "required_capabilities":["workspace.edit"],
                    "skill_revision_ids":[],
                    "permission_ids":["repo.read"],
                    "resource_ids":[],
                    "responsibility":"Coordinate"
                  },{
                    "id":"reviewer",
                    "kind":"main",
                    "agent_definition_id":"agent-main",
                    "runtime_profile_id":"profile-alt",
                    "runtime_instance_id":"runtime-pi",
                    "harness_adapter":"pi",
                    "provider_id":"anthropic",
                    "provider_account_id":"anthropic.review",
                    "model_id":"claude-sonnet",
                    "auth_mode":"brokered",
                    "credential_revision":7,
                    "reasoning_effort":"low",
                    "timeout_milliseconds":30000,
                    "budget_available":true,
                    "budget_units":40,
                    "required_capabilities":[],
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
		XCTAssertEqual(choice.provider, "OpenAI")
		XCTAssertEqual(choice.providerAccount, "openai.primary")
        XCTAssertEqual(choice.model, "codex-model")
        XCTAssertEqual(choice.auth, "Brokered")
		XCTAssertEqual(choice.reasoning, "High reasoning")
        XCTAssertTrue(choice.isCurrent)
        let alternate = try XCTUnwrap(
            choices.first(where: { $0.id == "reviewer" })
        )
		XCTAssertEqual(alternate.model, "claude-sonnet")
		XCTAssertEqual(alternate.provider, "Anthropic")
		XCTAssertEqual(alternate.providerAccount, "anthropic.review")
		XCTAssertEqual(alternate.auth, "Brokered")
		XCTAssertEqual(alternate.reasoning, "Low reasoning")
        XCTAssertFalse(alternate.isCurrent)
		let fallbackChoices = LocalProductRoleOptionChoice.fallbacks(
			setup: setup,
			role: try XCTUnwrap(session.preview.roles.first)
		)
		XCTAssertEqual(fallbackChoices.map(\.id), ["reviewer"])
		XCTAssertEqual(fallbackChoices.first?.field, "main_fallback_role")
		XCTAssertTrue(try XCTUnwrap(fallbackChoices.first).isCurrent)
		let accountRoutes = LocalProductRoleOptionChoice.routes(
			setup: setup,
			role: try XCTUnwrap(session.preview.roles.first),
			field: "main_provider_account_route"
		)
		XCTAssertEqual(accountRoutes.map(\.id), ["coordinator", "reviewer"])
		XCTAssertEqual(
			accountRoutes.map(\.field),
			["main_provider_account_route", "main_provider_account_route"]
		)
		XCTAssertEqual(accountRoutes.map(\.providerAccount), ["openai.primary", "anthropic.review"])
		XCTAssertTrue(try XCTUnwrap(accountRoutes.first).isCurrent)
		let encodedSession = try JSONEncoder().encode(session)
		var root = try XCTUnwrap(
			JSONSerialization.jsonObject(with: encodedSession) as? [String: Any]
		)
		var customProfileRoot = root
		var customProfilePreview = try XCTUnwrap(
			customProfileRoot["preview"] as? [String: Any]
		)
		var customProfileRoles = try XCTUnwrap(
			customProfilePreview["roles"] as? [[String: Any]]
		)
		customProfileRoles[0]["runtime_profile_id"] = "loom-profile-custom"
		customProfilePreview["roles"] = customProfileRoles
		customProfileRoot["preview"] = customProfilePreview
		let customProfileSession = try LocalProductSetupWire.decodeBuilderSession(
			JSONSerialization.data(withJSONObject: customProfileRoot)
		)
		let customAccountRoutes = LocalProductRoleOptionChoice.routes(
			setup: setup,
			role: try XCTUnwrap(customProfileSession.preview.roles.first),
			field: "main_provider_account_route"
		)
		XCTAssertTrue(try XCTUnwrap(customAccountRoutes.first).isCurrent)
		var preview = try XCTUnwrap(root["preview"] as? [String: Any])
		var roles = try XCTUnwrap(preview["roles"] as? [[String: Any]])
		roles[0]["fallback_approval_required"] = false
		preview["roles"] = roles
		root["preview"] = preview
		let inconsistentFallback = try JSONSerialization.data(withJSONObject: root)
		XCTAssertThrowsError(
			try LocalProductSetupWire.decodeBuilderSession(inconsistentFallback)
		)
        XCTAssertTrue(LocalProductStore.allowsBuilderEditField("main_role"))
        XCTAssertTrue(
            LocalProductStore.allowsBuilderEditField("subagent_role")
        )
		for field in [
			"subagent_add", "subagent_remove",
			"main_fallback_role", "subagent_fallback_role",
			"main_parallel_route_add", "subagent_parallel_route_add",
			"main_parallel_route_remove", "subagent_parallel_route_remove",
			"main_harness_route", "subagent_harness_route",
			"main_provider_account_route", "subagent_provider_account_route",
			"main_model", "subagent_model",
			"main_reasoning_effort", "subagent_reasoning_effort",
			"main_timeout_seconds", "subagent_timeout_seconds",
			"main_budget_units", "subagent_budget_units",
		] {
			XCTAssertTrue(LocalProductStore.allowsBuilderEditField(field), field)
		}
		XCTAssertFalse(LocalProductStore.allowsBuilderEditField("provider"))
		XCTAssertTrue(
			LocalProductStore.validBuilderEditTarget(
				field: "subagent_model",
				roleAgentDefinitionID: "agent-reviewer"
			)
		)
		XCTAssertFalse(
			LocalProductStore.validBuilderEditTarget(
				field: "subagent_model",
				roleAgentDefinitionID: ""
			)
		)
		XCTAssertFalse(
			LocalProductStore.validBuilderEditTarget(
				field: "main_model",
				roleAgentDefinitionID: "agent-main"
			)
		)
    }

    func testProviderNameUsesCanonicalDisplayNames() {
        XCTAssertEqual(
            LocalProductRoleReview.providerName(providerID: "opencode", authMode: "native_auth"),
            "OpenCode"
        )
        XCTAssertEqual(
            LocalProductRoleReview.providerName(providerID: "deepseek", authMode: "brokered"),
            "DeepSeek"
        )
        XCTAssertEqual(
            LocalProductRoleReview.providerName(providerID: "minimax", authMode: "brokered"),
            "MiniMax"
        )
    }


}
