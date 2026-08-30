import AppKit
import SwiftUI
import XCTest
@testable import LoomLocalAppCore
@testable import LoomLocalAppUI

@MainActor
private var retainedMissionWorkbenchWindows: [NSWindow] = []

@MainActor
final class LoomGraphiteViewTests: XCTestCase {
    func testActiveAndBlockedMissionsRemainVisibleWhenTeamIsNotExecutable() {
        XCTAssertTrue(missionHistoryIncludesMission(
            lane: .orchestrating,
            hideCompleted: true,
            teamExecutable: false
        ))
        XCTAssertTrue(missionHistoryIncludesMission(
            lane: .review,
            hideCompleted: true,
            teamExecutable: false
        ))
        XCTAssertFalse(missionHistoryIncludesMission(
            lane: .complete,
            hideCompleted: true,
            teamExecutable: false
        ))
    }

    func testDisabledPreflightExplainsRuntimeAndProviderRecoveryPath() {
        XCTAssertEqual(
            missionRuntimeRecoveryReason(
                hasExecutableTeam: false,
                executionReachable: true,
                setupState: .ready
            ),
            "No confirmed Team currently has executable Runtime and Provider bindings. Open Runtime & Providers to reconnect them, then retry."
        )
        XCTAssertEqual(
            missionRuntimeRecoveryReason(
                hasExecutableTeam: true,
                executionReachable: false,
                setupState: .ready
            ),
            "Mission preflight cannot reach Loom's local execution service. Open Runtime & Providers to refresh local service readiness, then retry."
        )
    }

    func testUnavailableSetupSnapshotUsesRecoverableServiceState() {
        let unavailable = missionSetupRecoveryPresentation(
            .unavailable(reason: "setup_unavailable")
        )
        XCTAssertEqual(unavailable.title, "Runtime & Providers unavailable")
        XCTAssertTrue(unavailable.detail.contains("local setup service"))
        XCTAssertTrue(unavailable.detail.contains("Try Again"))
        XCTAssertFalse(unavailable.isLoading)
        XCTAssertNotEqual(unavailable.title, "No providers found")

        XCTAssertTrue(
            missionSetupRecoveryPresentation(.loading).isLoading
        )
    }

    func testRuntimeAvailabilityExplainsDetectedHarnessWithoutAgentAccount() {
        XCTAssertEqual(
            runtimeAgentAvailabilityDetail(
                adapterType: "codex",
                modelIDs: ["gpt-5.5-codex"],
                roleOptionCount: 0
            ),
            "Detected · Connect OpenAI in Model Providers to use with Agent Teams"
        )
        XCTAssertEqual(
            runtimeAgentAvailabilityDetail(
                adapterType: "claude-code",
                modelIDs: ["claude-sonnet"],
                roleOptionCount: 0,
                hasConversationRoute: false
            ),
            "Detected · Sign in to Claude Code for Conversation; connect Anthropic for Agent Teams"
        )
        XCTAssertEqual(
            runtimeAgentAvailabilityDetail(
                adapterType: "claude-code",
                modelIDs: ["claude-sonnet"],
                roleOptionCount: 2,
                hasConversationRoute: false
            ),
            "Agent Teams ready · Sign in to Claude Code for Conversation"
        )
        XCTAssertEqual(
            runtimeAgentAvailabilityDetail(
                adapterType: "opencode",
                modelIDs: ["deepseek/deepseek-chat"],
                roleOptionCount: 2
            ),
            "deepseek/deepseek-chat · Available to Agent Teams"
        )
        XCTAssertEqual(
            runtimeAgentAvailabilityDetail(
                adapterType: "pi-cli",
                modelIDs: ["loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m"],
                roleOptionCount: 1,
                hasConversationRoute: false
            ),
            "Detected · Local model required for Conversation"
        )
        XCTAssertEqual(
            runtimeNativeAuthActionTitle(
                adapterType: "claude-code",
                status: "online",
                hasConversationRoute: false
            ),
            "Sign In"
        )
        XCTAssertNil(
            runtimeNativeAuthActionTitle(
                adapterType: "claude-code",
                status: "online",
                hasConversationRoute: true
            )
        )
        XCTAssertNil(
            runtimeNativeAuthActionTitle(
                adapterType: "codex",
                status: "online",
                hasConversationRoute: false
            )
        )
        XCTAssertEqual(
            runtimeAvailabilityStatus(
                adapterType: "claude-code",
                status: "online",
                hasConversationRoute: false
            ),
            "sign_in_required"
        )
        XCTAssertEqual(
            runtimeAvailabilityStatus(
                adapterType: "claude-code",
                status: "online",
                hasConversationRoute: true
            ),
            "online"
        )
        XCTAssertEqual(
            runtimeAvailabilityStatus(
                adapterType: "opencode",
                status: "online",
                hasConversationRoute: false
            ),
            "online"
        )
        XCTAssertEqual(
            runtimeNativeAuthActivity(
                adapterType: "claude-code",
                inFlight: true,
                operationStatus: "Waiting for sign in"
            ),
            .waiting
        )
        XCTAssertEqual(
            runtimeNativeAuthActivity(
                adapterType: "claude-code",
                inFlight: true,
                operationStatus: "Cancelling sign in"
            ),
            .cancelling
        )
        XCTAssertEqual(
            runtimeNativeAuthActivity(
                adapterType: "opencode",
                inFlight: true,
                operationStatus: "Waiting for sign in"
            ),
            .idle
        )
        XCTAssertEqual(
            runtimeNativeAuthRecoveryDetail(
                stage: "Conversation profile publish",
                retryable: true
            ),
            "Stage: Conversation profile publish · Retry available"
        )
        XCTAssertEqual(
            runtimeNativeAuthRecoveryDetail(
                stage: "Local service admission",
                retryable: false
            ),
            "Stage: Local service admission · Manual action required"
        )
        XCTAssertNil(
            runtimeNativeAuthRecoveryDetail(stage: nil, retryable: nil)
        )
    }

    func testConversationVaultRecoveryActionUsesClosedFailureAllowlist() {
        XCTAssertTrue(conversationVaultRecoveryAvailable(
            stage: .vaultDecrypt,
            recoverable: false
        ))
        XCTAssertTrue(conversationVaultRecoveryAvailable(
            stage: .vaultAADValidation,
            recoverable: false
        ))
        XCTAssertFalse(conversationVaultRecoveryAvailable(
            stage: .credentialLeaseIssue,
            recoverable: false
        ))
        // Vault-stage failures offer recovery even when retryable: retrying
        // without unlocking keeps failing with the same locked state.
        XCTAssertTrue(conversationVaultRecoveryAvailable(
            stage: .vaultDecrypt,
            recoverable: true
        ))
        XCTAssertTrue(conversationVaultRecoveryAvailable(
            stage: .vaultEncrypt,
            recoverable: true
        ))
        XCTAssertFalse(conversationVaultRecoveryAvailable(
            stage: .providerConnect,
            recoverable: true
        ))
    }

    func testCompletedToolActivityUsesClosedReadableNamesAndEightItemLimit()
        throws
    {
        let expectedNames = [
            "loom.sessions.search": "Find conversations",
            "loom.sessions.align.preview": "Align conversations",
            "loom.missions.create.preview": "Create Mission",
            "loom.missions.continue.preview": "Continue Mission",
            "loom.teams.create.preview": "Create Agent Team",
            "loom.roundtables.open.preview": "Open RoundTable",
            "loom.missions.search": "Find Missions",
            "loom.missions.status": "Mission status",
            "loom.teams.search": "Find Agent Teams",
            "loom.teams.status": "Agent Team status",
            "loom.roundtables.status": "RoundTable status",
            "loom.governance.needs_you": "Needs You",
            "loom.runtimes.status": "Runtime status",
            "loom.providers.status": "Provider status",
            "loom.diagnostics.incident": "Incident diagnostics",
            "loom.workspace.status": "Workspace status",
            "loom.conversation.route.status": "Conversation route",
            "loom.library.search": "Search library",
            "loom.conversation.route.change.preview": "Change route",
            "loom.conversation.model.change.preview": "Change model",
            "loom.conversation.reasoning.change.preview": "Change reasoning",
            "loom.workspace.choose.preview": "Choose workspace",
            "loom.teams.edit.preview": "Edit Agent Team",
            "loom.roundtables.pause.preview": "Pause RoundTable",
            "loom.roundtables.steer.preview": "Guide RoundTable",
            "loom.roundtables.retry.preview": "Retry RoundTable seat",
            "loom.roundtables.skip.preview": "Skip RoundTable seat",
            "loom.roundtables.replace.preview": "Replace RoundTable seat",
        ]
        XCTAssertEqual(expectedNames.count, 28)
        for (toolID, expectedName) in expectedNames {
            XCTAssertEqual(
                conversationCompletedToolDisplayName(toolID),
                expectedName
            )
        }
        XCTAssertEqual(
            conversationCompletedToolDisplayName("loom.unknown.tool"),
            "Loom tool"
        )

        let nineToolIDs = [
            "loom.sessions.search",
            "loom.missions.search",
            "loom.teams.search",
            "loom.roundtables.status",
            "loom.governance.needs_you",
            "loom.runtimes.status",
            "loom.providers.status",
            "loom.workspace.status",
            "loom.library.search",
        ]
        let activity = try XCTUnwrap(
            conversationCompletedToolActivityText(nineToolIDs)
        )
        XCTAssertTrue(activity.hasPrefix("Used Loom tools: "))
        XCTAssertTrue(activity.contains("Workspace status"))
        XCTAssertFalse(activity.contains("Search library"))
        XCTAssertNil(conversationCompletedToolActivityText([]))
    }

    func testCompletedToolActivityUsesExactReplyAttemptIdentity() throws {
        let digest = String(repeating: "a", count: 64)
        let attempt = LocalProductConversationAttempt(
            attemptID: "attempt-tool-exact",
            segmentID: "segment-tool-exact",
            profileID: "conversation-codex",
            contextMode: .startClean,
            contextCapsuleDigest: digest,
            completedControlTools: [
                try LocalProductConversationCompletedTool(
                    toolID: "loom.runtimes.status",
                    toolVersion: 1,
                    effect: "read"
                ),
            ],
            bindingDigest: digest,
            status: "succeeded",
            failureCode: ""
        )
        let reply = LocalProductChatMessage(
            messageID: "message-tool-exact",
            segmentID: "segment-tool-exact",
            attemptID: attempt.attemptID,
            role: "loom",
            content: "Runtime status checked.",
            tentative: true
        )
        let thread = LocalProductChatThread(
            threadID: "thread-tool-exact",
            attempts: [attempt],
            messages: [
                LocalProductChatMessage(
                    messageID: "message-explicit-user",
                    segmentID: "segment-tool-exact",
                    role: "user",
                    content: "Use an Agent Team",
                    tentative: false
                ),
                LocalProductChatMessage(
                    messageID: "message-explicit-proposal",
                    segmentID: "segment-tool-exact",
                    role: "proposal",
                    content: "Open Team review.",
                    tentative: true
                ),
                reply,
            ],
            canReply: true,
            requiresConfirmation: false
        )

        XCTAssertEqual(
            conversationAttemptForMessage(reply, in: thread)?.attemptID,
            attempt.attemptID
        )
        let drifted = LocalProductChatMessage(
            messageID: reply.messageID,
            segmentID: "segment-other",
            attemptID: attempt.attemptID,
            role: reply.role,
            content: reply.content,
            tentative: reply.tentative
        )
        XCTAssertNil(conversationAttemptForMessage(drifted, in: thread))

        let confirmation = LocalProductChatMessage(
            messageID: "message-confirmation",
            segmentID: attempt.segmentID,
            role: "confirmation",
            content: "Loom action approved.",
            tentative: false
        )
        XCTAssertNil(conversationAttemptForMessage(confirmation, in: thread))
    }

    func testCompletedToolActivityRendersAtCompactAndAccessibilitySizes()
        throws
    {
        let toolIDs = [
            "loom.sessions.align.preview",
            "loom.missions.create.preview",
            "loom.teams.create.preview",
            "loom.roundtables.open.preview",
            "loom.runtimes.status",
            "loom.providers.status",
            "loom.diagnostics.incident",
            "loom.workspace.status",
        ]
        for (width, height, typeSize) in [
            (560.0, 120.0, DynamicTypeSize.large),
            (320.0, 260.0, DynamicTypeSize.accessibility2),
        ] {
            let size = NSSize(width: width, height: height)
            let hosting = NSHostingView(
                rootView: ConversationCompletedToolActivity(
                    attemptID: "attempt-tools-1",
                    toolIDs: toolIDs
                )
                .environment(\.dynamicTypeSize, typeSize)
                .padding(12)
                .frame(width: width, height: height, alignment: .topLeading)
                .background(Color(nsColor: .windowBackgroundColor))
            )
            hosting.frame = NSRect(origin: .zero, size: size)
            hosting.layoutSubtreeIfNeeded()

            let bitmap = try XCTUnwrap(
                hosting.bitmapImageRepForCachingDisplay(in: hosting.bounds)
            )
            hosting.cacheDisplay(in: hosting.bounds, to: bitmap)
            let imageData = try XCTUnwrap(
                bitmap.representation(using: .png, properties: [:])
            )
            XCTAssertGreaterThan(imageData.count, 1_000)
            XCTAssertLessThanOrEqual(hosting.fittingSize.width, width)
            XCTAssertLessThanOrEqual(hosting.fittingSize.height, height)

            if let captureRoot = ProcessInfo.processInfo.environment[
                "LOOM_PHASE7_SCREENSHOT_DIR"
            ] {
                let directory = URL(fileURLWithPath: captureRoot, isDirectory: true)
                try FileManager.default.createDirectory(
                    at: directory,
                    withIntermediateDirectories: true
                )
                try imageData.write(
                    to: directory.appendingPathComponent(
                        "completed-tool-activity-\(Int(width)).png"
                    )
                )
            }
        }

        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/LoomWorkspaceShell.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)
        let activitySource = try XCTUnwrap(
            source.components(separatedBy: "struct ConversationCompletedToolActivity")
                .dropFirst()
                .first?
                .components(separatedBy: "public enum LoomRecentTaskActionLabel")
                .first
        )
        XCTAssertTrue(activitySource.contains("wrench.and.screwdriver"))
        XCTAssertTrue(activitySource.contains("loom.conversation.tool-activity."))
        XCTAssertTrue(activitySource.contains(".accessibilityLabel(activityText)"))
        XCTAssertFalse(activitySource.contains(".background("))
        XCTAssertFalse(activitySource.contains(".overlay("))
        XCTAssertFalse(activitySource.localizedCaseInsensitiveContains("arguments"))
        XCTAssertFalse(activitySource.localizedCaseInsensitiveContains("result"))
        XCTAssertFalse(activitySource.localizedCaseInsensitiveContains("prompt"))
        XCTAssertTrue(source.contains("private func conversationAttempt("))
        XCTAssertTrue(source.contains("let segmentAttempts = thread.attempts.filter"))
        XCTAssertTrue(source.contains("attempt.completedControlTools.map(\\.toolID)"))
    }

    func testProposalDecisionFailureUsesRefreshInsteadOfResendingComposer() throws {
        let failure = LocalProductChatOperationFailure(
            code: .conflict,
            stage: .controlProposalConfirm,
            recoverable: true,
            incidentID: "loom-chat-proposal-conflict",
            title: "Proposal changed",
            detail: "Refresh it."
        )
        XCTAssertTrue(failure.isProposalDecisionRecoveryAvailable)

        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/LoomWorkspaceShell.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)
        XCTAssertTrue(source.contains("if failure.isProposalDecisionRecoveryAvailable"))
        XCTAssertTrue(source.contains("Label(\"Refresh proposal\", systemImage: \"arrow.clockwise\")"))
        XCTAssertTrue(source.contains("Task { await store.loadChatThread() }"))
    }

    func testChatFailureActionsFitCompactAccessibilityWidth() throws {
        let failure = LocalProductChatOperationFailure(
            code: .providerUnavailable,
            stage: .providerConnect,
            recoverable: true,
            incidentID: "loom-chat-compact-failure",
            title: "Provider needs attention",
            detail: "Unlock the Vault or switch to another configured Provider Account."
        )
        let width = 360.0
        let height = 560.0
        let size = NSSize(width: width, height: height)
        let hosting = NSHostingView(
            rootView: ConversationChatFailureBanner(
                failure: failure,
                vaultRecoveryAvailable: true,
                routeRecoveryAvailable: true,
                isSending: false,
                isUpdatingVault: false,
                onPrimaryRecovery: {},
                onUnlockVault: {},
                onOpenVault: {},
                onSwitchProvider: {},
                onDiagnostics: {},
                onCopyIncident: {}
            )
            .dynamicTypeSize(.accessibility3)
            .padding(12)
            .frame(width: width, height: height, alignment: .topLeading)
            .background(Color(nsColor: .windowBackgroundColor))
        )
        hosting.frame = NSRect(origin: .zero, size: size)
        hosting.layoutSubtreeIfNeeded()
        let bitmap = try XCTUnwrap(
            hosting.bitmapImageRepForCachingDisplay(in: hosting.bounds)
        )
        hosting.cacheDisplay(in: hosting.bounds, to: bitmap)
        XCTAssertLessThanOrEqual(hosting.fittingSize.width, width)
        XCTAssertGreaterThan(
            try XCTUnwrap(bitmap.representation(using: .png, properties: [:])).count,
            2_000
        )
    }

    func testConversationProfileMenuLabelIncludesExactProviderAccount() throws {
        let profile = try JSONDecoder().decode(
            LocalProductConversationProfile.self,
            from: Data(
                #"{"profile_id":"conversation-deepseek-deepseek-chat-account-work-r7","harness_adapter":"loom-native","provider_id":"deepseek","provider_account_id":"deepseek.work","display_name":"DeepSeek","protocol":"openai_compatible","model_id":"deepseek-chat","auth_mode":"brokered","credential_revision":7}"#.utf8
            )
        )
        // Route labels show the three independent bindings in execution order.
        XCTAssertEqual(
            conversationProfileMenuLabel(profile),
            "Loom Native · DeepSeek · Work"
        )
        XCTAssertEqual(conversationRouteOptionLabel(profile), "DeepSeek · Work")
    }

    func testConversationProfileMenuLabelDoesNotTreatOpenCodeRouteAsProvider() throws {
        let profile = try JSONDecoder().decode(
            LocalProductConversationProfile.self,
            from: Data(
                #"{"profile_id":"conversation-opencode-deepseek-r2","harness_adapter":"opencode","provider_id":"deepseek","provider_account_id":"deepseek.primary","display_name":"DeepSeek","protocol":"opencode_agent","model_id":"deepseek/deepseek-chat","auth_mode":"brokered","credential_revision":2}"#.utf8
            )
        )
        XCTAssertEqual(
            conversationProfileMenuLabel(profile),
            "OpenCode · DeepSeek"
        )
        XCTAssertEqual(conversationRouteOptionLabel(profile), "DeepSeek")
        XCTAssertFalse(conversationProfileMenuLabel(profile).contains("deepseek.primary"))
    }

    func testConversationRouteMenuGroupsHarnessesAndNamesNativeOpenCode() throws {
        let profiles = try JSONDecoder().decode(
            [LocalProductConversationProfile].self,
            from: Data(
                #"""
                [
                {"profile_id":"conversation-deepseek-r2","harness_adapter":"loom-native","provider_id":"deepseek","provider_account_id":"deepseek.primary","display_name":"DeepSeek","protocol":"openai_compatible","model_id":"deepseek-chat","auth_mode":"brokered","credential_revision":2},
                {"profile_id":"conversation-opencode-deepseek-r2","harness_adapter":"opencode","provider_id":"deepseek","provider_account_id":"deepseek.primary","display_name":"DeepSeek","protocol":"opencode_agent","model_id":"deepseek/deepseek-chat","auth_mode":"brokered","credential_revision":2},
                {"profile_id":"conversation-opencode-default-v1","harness_adapter":"opencode","provider_id":"opencode","provider_account_id":"","display_name":"OpenCode","protocol":"opencode_agent","model_id":"opencode/big-pickle","auth_mode":"native_auth","credential_revision":0},
                {"profile_id":"conversation-openai-codex-default-v1","harness_adapter":"codex","provider_id":"openai","provider_account_id":"","display_name":"OpenAI","protocol":"openai_responses","model_id":"codex-default","auth_mode":"native_auth","credential_revision":0}
                ]
                """#.utf8
            )
        )
        let groups = conversationRouteMenuGroups(profiles)
        XCTAssertEqual(groups.map(\.displayName), ["Loom Native", "OpenCode", "Codex"])
        XCTAssertEqual(groups.map { $0.profiles.count }, [1, 2, 1])
        XCTAssertEqual(conversationRouteOptionLabel(groups[1].profiles[1]), "Built-in")
        XCTAssertEqual(conversationProfileMenuLabel(groups[1].profiles[1]), "OpenCode · Built-in")
    }

    func testProviderAccountIdentifiersAreStableAndBounded() {
        XCTAssertEqual(
            providerAccountIdentifier(providerID: "deepseek", name: "Work Team"),
            "deepseek.work-team"
        )
        XCTAssertEqual(
            providerAccountIdentifier(
                providerID: "deepseek",
                name: "  Build / Review...Team  "
            ),
            "deepseek.build-review-team"
        )
        XCTAssertNil(
            providerAccountIdentifier(providerID: "deepseek", name: "@#$")
        )
    }

    func testProviderAccountDisplayNamesKeepProviderScopeOutOfTheLabel() {
        XCTAssertEqual(
            providerAccountDisplayName("deepseek.primary", providerID: "deepseek"),
            "Primary"
        )
        XCTAssertEqual(
            providerAccountDisplayName("deepseek.work-team", providerID: "deepseek"),
            "Work Team"
        )
        XCTAssertEqual(
            providerAccountDisplayName("openai.primary", providerID: "deepseek"),
            "Account"
        )
    }

    func testMissionWorkbenchHasNoInertVisibleActions() throws {
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/MissionWorkbench.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)
        let inertButton = try NSRegularExpression(
            pattern: #"Button\([^\n]*\)\s*\{\s*\}"#
        )
        let fullRange = NSRange(source.startIndex..., in: source)

        XCTAssertNil(
            inertButton.firstMatch(in: source, range: fullRange),
            "MissionWorkbench contains a visible Button with an empty action"
        )
        XCTAssertFalse(
            source.contains(".disabled(true)"),
            "MissionWorkbench contains a permanently disabled placeholder action"
        )
		XCTAssertTrue(source.contains("missionPreflightFallbackRoute"))
		XCTAssertTrue(source.contains("Fallback approval required"))
        XCTAssertTrue(source.contains("section.canReviewRetry(at: index)"))
        XCTAssertTrue(source.contains("section.canRecoverCredentialVault(at: index)"))
        XCTAssertTrue(source.contains("section.canViewDiagnostics(at: index)"))
        XCTAssertTrue(source.contains("Review retry"))
        XCTAssertTrue(source.contains("Open Credential Vault recovery"))
        XCTAssertTrue(source.contains("View Agent diagnostics"))
        XCTAssertTrue(source.contains("store.showMissionAttention()"))
        XCTAssertTrue(source.contains("showProviders = true"))
        XCTAssertTrue(source.contains("Open Credential Vault"))
        XCTAssertTrue(source.contains("prepareVaultDiagnosticPreview()"))
        XCTAssertTrue(source.contains("runProviderFailureLabScenario(result.scenario)"))
        XCTAssertTrue(source.contains("Copy incident ID"))
        XCTAssertTrue(source.contains("View diagnostics for"))
        XCTAssertTrue(source.contains("Exact endpoint"))
        XCTAssertTrue(source.contains("presentation.endpoint"))
        XCTAssertTrue(source.contains("Confirmed constraints"))
        XCTAssertTrue(source.contains("Accepted decisions"))
        XCTAssertTrue(source.contains("I confirm this Mission context"))
        XCTAssertTrue(source.contains("store.invalidateMissionPreflight()"))
        XCTAssertTrue(source.contains("if !confirmed { store.invalidateMissionPreflight() }"))
        XCTAssertTrue(source.contains(".disabled(!missionContextConfirmed"))
        XCTAssertTrue(source.contains("confirmedConstraints: confirmedMissionConstraints"))
        XCTAssertTrue(source.contains("acceptedDecisions: acceptedMissionDecisions"))
        XCTAssertEqual(
            LoomDecisionSheet.visibleActions([
                "not_now", "deny", "edit_scope", "allow_once",
            ]),
            ["not_now", "deny", "allow_once"]
        )
    }

    func testProviderDiagnosticsUsePreviewedRedactedExport() throws {
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/MissionWorkbench.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)

        XCTAssertTrue(source.contains("prepareDiagnosticPreview()"))
        XCTAssertTrue(source.contains("prepareVaultDiagnosticPreview()"))
        XCTAssertTrue(source.contains("Preview privacy-safe Vault diagnostics"))
        XCTAssertTrue(source.contains("await store.rotateCredentialVault()"))
        XCTAssertTrue(source.contains("Rotate the Vault wrapping key"))
        XCTAssertTrue(source.contains("await store.resetCredentialVault()"))
        XCTAssertTrue(source.contains("This permanently removes the unusable encrypted Vault"))
        XCTAssertTrue(source.contains("CredentialVaultExportSheet"))
        XCTAssertTrue(source.contains("Export a passphrase-protected Credential Vault backup"))
        XCTAssertTrue(source.contains("The backup cannot be recovered without this passphrase."))
        XCTAssertTrue(source.contains("DiagnosticBundlePreviewSheet"))
        XCTAssertTrue(source.contains("LocalDiagnosticBundleInput("))
        XCTAssertTrue(source.contains("try exporter.export(preview, to: destination)"))
        XCTAssertFalse(source.contains("NSWorkspace.shared.open(diagnosticsDirectory)"))
    }

    func testEmptyConversationKeepsConversationAsThePrimaryAction() throws {
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/LoomWorkspaceShell.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)

        XCTAssertTrue(source.contains("Text(\"What are you working on?\")"))
        XCTAssertTrue(source.contains("Text(\"New task\")"))
        XCTAssertTrue(source.contains(".accessibilityLabel(\"Message Loom\")"))
        XCTAssertTrue(source.contains(
            ".accessibilityHint(\"Describe a task, ask a question, or type slash for commands\")"
        ))
        XCTAssertTrue(source.contains(
            ".accessibilityIdentifier(\"loom.conversation.composer\")"
        ))
        XCTAssertFalse(source.contains("teamReady ? \"Start Mission\" : \"Use Agent Team\""))
        XCTAssertFalse(source.contains("private var quickStartGuide"))
    }

    func testPhase5ShellUsesProgressiveConversationFirstNavigation() throws {
        XCTAssertEqual(
            LoomWorkspaceNavigationItem.primary,
            [.home, .work, .attention]
        )
        XCTAssertEqual(
            LoomWorkspaceNavigationItem.governance,
            [.teams, .roundtable]
        )
        XCTAssertEqual(LoomWorkspaceNavigationItem.tools, [.library])
        XCTAssertEqual(LoomWorkspaceNavigationItem.work.rawValue, "Missions")
        XCTAssertEqual(LoomWorkspaceNavigationItem.attention.rawValue, "Needs You")

        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/LoomWorkspaceShell.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)

        XCTAssertTrue(source.contains(
            "loomRailConversationLimit(compact: compact)"
        ))
        XCTAssertTrue(source.contains("if conversationLimit > 0"))
        XCTAssertTrue(source.contains("Text(\"More conversations\")"))
        XCTAssertTrue(source.contains(
            "railSectionTitle(\"GOVERN\", compact: compact)"
        ))
        XCTAssertTrue(source.contains("case .work: return activeMissionCount"))
        XCTAssertTrue(source.contains("case .attention: return activeAttentionCount"))
        XCTAssertTrue(source.contains("case .teams: return store.snapshot?.teams.count ?? 0"))
        XCTAssertTrue(source.contains(
            "case .library: return store.snapshot?.evidence.count ?? 0"
        ))
        XCTAssertTrue(source.contains("navigationHasRecordedActivity(item)"))
        XCTAssertTrue(source.contains(".accessibilityLabel(\"Recorded activity\")"))
        XCTAssertTrue(source.contains(
            "Label(\"Start Mission\", systemImage: \"flag.checkered\")"
        ))
        XCTAssertTrue(source.contains(
            "Label(\"Use Agent Team\", systemImage: \"person.3\")"
        ))
        XCTAssertFalse(source.contains(
            "Text(\"Agent work requires review and confirmation.\")"
        ))
    }

    func testPhase5WorkspaceShellRendersAtDesktopAndCompactWidths()
        async throws
    {
        let snapshot = try ExperienceFixtures.populatedSnapshot()
        XCTAssertEqual(snapshot.teams.count, 1)
        XCTAssertEqual(snapshot.evidence.count, 2)
        for width in [1_280.0, 720.0] {
            let fixtureRoot = FileManager.default.temporaryDirectory
                .appendingPathComponent(UUID().uuidString, isDirectory: true)
            let store = LocalProductStore(
                client: MissionWorkbenchStubClient(snapshot: snapshot),
                chatSessionsFileURL: fixtureRoot.appendingPathComponent(
                    "chat-sessions.json"
                )
            )
            await store.refresh()

            let size = NSSize(width: width, height: 760)
            let hosting = NSHostingView(
                rootView: LoomWorkspaceShell(store: store)
                    .frame(width: width, height: size.height)
            )
            hosting.frame = NSRect(origin: .zero, size: size)
            hosting.layoutSubtreeIfNeeded()

            let bitmap = try XCTUnwrap(
                hosting.bitmapImageRepForCachingDisplay(in: hosting.bounds)
            )
            hosting.cacheDisplay(in: hosting.bounds, to: bitmap)
            XCTAssertEqual(bitmap.size.width, width)
            XCTAssertEqual(bitmap.size.height, size.height)
            XCTAssertGreaterThanOrEqual(bitmap.pixelsWide, Int(width))
            XCTAssertGreaterThanOrEqual(bitmap.pixelsHigh, Int(size.height))
            XCTAssertGreaterThan(hosting.fittingSize.width, 500)
            XCTAssertLessThanOrEqual(hosting.fittingSize.width, width)

            if let captureRoot = ProcessInfo.processInfo.environment[
                "LOOM_PHASE5_SCREENSHOT_DIR"
            ] {
                let directory = URL(fileURLWithPath: captureRoot, isDirectory: true)
                try FileManager.default.createDirectory(
                    at: directory,
                    withIntermediateDirectories: true
                )
                let imageData = try XCTUnwrap(
                    bitmap.representation(using: .png, properties: [:])
                )
                try imageData.write(
                    to: directory.appendingPathComponent("shell-\(Int(width)).png")
                )
            }
        }
    }

    func testPhase6WorkspaceShellRendersSlashCommandsAtDesktopAndCompactWidths()
        async throws
    {
        let snapshot = try ExperienceFixtures.populatedSnapshot()
        for width in [1_280.0, 720.0] {
            let fixtureRoot = FileManager.default.temporaryDirectory
                .appendingPathComponent(UUID().uuidString, isDirectory: true)
            let store = LocalProductStore(
                client: MissionWorkbenchStubClient(snapshot: snapshot),
                chatSessionsFileURL: fixtureRoot.appendingPathComponent(
                    "chat-sessions.json"
                )
            )
            await store.refresh()
            store.updateComposerDraft("/")

            let size = NSSize(width: width, height: 760)
            let hosting = NSHostingView(
                rootView: LoomWorkspaceShell(store: store)
                    .frame(width: width, height: size.height)
            )
            hosting.frame = NSRect(origin: .zero, size: size)
            hosting.layoutSubtreeIfNeeded()

            let bitmap = try XCTUnwrap(
                hosting.bitmapImageRepForCachingDisplay(in: hosting.bounds)
            )
            hosting.cacheDisplay(in: hosting.bounds, to: bitmap)
            XCTAssertEqual(bitmap.size.width, width)
            XCTAssertEqual(bitmap.size.height, size.height)
            XCTAssertGreaterThanOrEqual(bitmap.pixelsWide, Int(width))
            XCTAssertGreaterThanOrEqual(bitmap.pixelsHigh, Int(size.height))
            XCTAssertLessThanOrEqual(hosting.fittingSize.width, width)

            if let captureRoot = ProcessInfo.processInfo.environment[
                "LOOM_PHASE6_SCREENSHOT_DIR"
            ] {
                let directory = URL(fileURLWithPath: captureRoot, isDirectory: true)
                try FileManager.default.createDirectory(
                    at: directory,
                    withIntermediateDirectories: true
                )
                let imageData = try XCTUnwrap(
                    bitmap.representation(using: .png, properties: [:])
                )
                try imageData.write(
                    to: directory.appendingPathComponent(
                        "shell-commands-\(Int(width)).png"
                    )
                )
            }
        }
    }

    func testConversationControlProposalCardRendersAtCompactAndAccessibilitySizes()
        throws
    {
        let digest = String(repeating: "a", count: 64)
        let proposal = try JSONDecoder().decode(
            LocalProductConversationControlProposal.self,
            from: Data(
                """
                {
                  "schema_version":1,
                  "proposal_id":"proposal-ui-1",
                  "tool_id":"loom.sessions.align.preview",
                  "tool_version":1,
                  "confirmation":"user",
                  "target_conversation_id":"session-current",
                  "target_content_digest":"\(digest)",
                  "sources":[
                    {
                      "conversation_id":"session-one",
                      "title":"Planning and accepted constraints",
                      "content_digest":"\(digest)",
                      "message_count":4
                    },
                    {
                      "conversation_id":"session-two",
                      "title":"Implementation results and verification",
                      "content_digest":"\(digest)",
                      "message_count":6
                    }
                  ],
                  "context_mode":"summary_only",
                  "catalog_digest":"\(digest)",
                  "segment_id":"segment-1",
                  "attempt_id":"attempt-1",
                  "message_id":"message-1",
                  "status":"pending",
                  "created_at":"2026-08-28T12:00:00Z",
                  "expires_at":"2026-08-28T12:05:00Z",
                  "proposal_digest":"\(digest)"
                }
                """.utf8
            )
        )
        let fixtures: [(Double, Double, DynamicTypeSize)] = [
            (560, 280, .large),
            (360, 390, .accessibility2),
        ]
        for (width, height, typeSize) in fixtures {
            var decision: LocalProductChatControlDecision?
            let size = NSSize(width: width, height: height)
            let hosting = NSHostingView(
                rootView: ConversationControlProposalCard(
                    proposal: proposal,
                    alignment: nil,
                    inFlight: false,
                    onDecision: { decision = $0 }
                )
                .environment(\.dynamicTypeSize, typeSize)
                .environment(\.colorScheme, .light)
                .padding(12)
                .frame(width: width, height: height, alignment: .topLeading)
                .background(Color(nsColor: .windowBackgroundColor))
            )
            hosting.frame = NSRect(origin: .zero, size: size)
            hosting.layoutSubtreeIfNeeded()

            let bitmap = try XCTUnwrap(
                hosting.bitmapImageRepForCachingDisplay(in: hosting.bounds)
            )
            hosting.cacheDisplay(in: hosting.bounds, to: bitmap)
            let imageData = try XCTUnwrap(
                bitmap.representation(using: .png, properties: [:])
            )
            XCTAssertEqual(bitmap.size.width, width)
            XCTAssertEqual(bitmap.size.height, height)
            XCTAssertGreaterThan(imageData.count, 2_000)
            XCTAssertLessThanOrEqual(hosting.fittingSize.width, width)
            XCTAssertNil(decision)

            if let captureRoot = ProcessInfo.processInfo.environment[
                "LOOM_PHASE7_SCREENSHOT_DIR"
            ] {
                let directory = URL(fileURLWithPath: captureRoot, isDirectory: true)
                try FileManager.default.createDirectory(
                    at: directory,
                    withIntermediateDirectories: true
                )
                try imageData.write(
                    to: directory.appendingPathComponent(
                        "control-proposal-\(Int(width)).png"
                    )
                )
            }
        }

        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/LoomWorkspaceShell.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)
        XCTAssertTrue(source.contains("loom.conversation.control-proposal.confirm"))
        XCTAssertTrue(source.contains("loom.conversation.control-proposal.cancel"))
        XCTAssertTrue(source.contains("Your next message starts the aligned segment."))
    }

    func testConversationActionProposalCardRendersAtCompactAndAccessibilitySizes()
        throws
    {
        let digest = String(repeating: "a", count: 64)
        let proposal = try JSONDecoder().decode(
            LocalProductConversationActionProposal.self,
            from: Data(
                """
                {
                  "schema_version":1,"proposal_id":"proposal-action-ui-1",
                  "tool_id":"loom.teams.create.preview","tool_version":1,
                  "confirmation":"user","action":"team",
                  "argument":"Create a planner, implementer, and reviewer with independent Runtime bindings.",
                  "target_conversation_id":"session-current","target_content_digest":"\(digest)",
                  "segment_id":"segment-1","attempt_id":"attempt-1","message_id":"message-1",
                  "status":"pending","created_at":"2026-08-28T12:00:00Z",
                  "expires_at":"2026-08-28T12:05:00Z","proposal_digest":"\(digest)"
                }
                """.utf8
            )
        )
        for (width, height, typeSize) in [
            (560.0, 260.0, DynamicTypeSize.large),
            (360.0, 380.0, DynamicTypeSize.accessibility2),
        ] {
            var decision: LocalProductChatControlDecision?
            let size = NSSize(width: width, height: height)
            let hosting = NSHostingView(
                rootView: ConversationActionProposalCard(
                    proposal: proposal,
                    inFlight: false,
                    onDecision: { decision = $0 }
                )
                .environment(\.dynamicTypeSize, typeSize)
                .padding(12)
                .frame(width: width, height: height, alignment: .topLeading)
                .background(Color(nsColor: .windowBackgroundColor))
            )
            hosting.frame = NSRect(origin: .zero, size: size)
            hosting.layoutSubtreeIfNeeded()
            let bitmap = try XCTUnwrap(
                hosting.bitmapImageRepForCachingDisplay(in: hosting.bounds)
            )
            hosting.cacheDisplay(in: hosting.bounds, to: bitmap)
            let imageData = try XCTUnwrap(
                bitmap.representation(using: .png, properties: [:])
            )
            XCTAssertGreaterThan(imageData.count, 2_000)
            XCTAssertLessThanOrEqual(hosting.fittingSize.width, width)
            XCTAssertNil(decision)
        }

        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/LoomWorkspaceShell.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)
        XCTAssertTrue(source.contains("loom.conversation.action-proposal.confirm"))
        XCTAssertTrue(source.contains("Review every role, Runtime, Provider Account and model"))
    }

    func testTerminalProposalCardsExplainOutcomeAndRecoveryAtCompactSize() throws {
        XCTAssertEqual(
            conversationProposalRecoveryMessage(status: .cancelled, hasDecisionReceipt: true),
            "Nothing changed. Ask Loom to prepare a fresh proposal when you want to try again."
        )
        XCTAssertEqual(
            conversationProposalRecoveryMessage(status: .expired, hasDecisionReceipt: true),
            "This proposal expired for safety. Nothing changed; ask Loom to prepare a fresh one."
        )
        XCTAssertEqual(
            conversationProposalRecoveryMessage(status: .confirmed, hasDecisionReceipt: false),
            "This historical approval is read-only. Ask Loom to prepare it again."
        )

        let digest = String(repeating: "a", count: 64)
        let action = try JSONDecoder().decode(
            LocalProductConversationActionProposal.self,
            from: Data(
                """
                {
                  "schema_version":1,"proposal_id":"proposal-action-cancelled-ui-1",
                  "tool_id":"loom.teams.create.preview","tool_version":1,
                  "confirmation":"user","action":"team","argument":"Review the team",
                  "target_conversation_id":"session-current","target_content_digest":"\(digest)",
                  "segment_id":"segment-1","attempt_id":"attempt-1","message_id":"message-1",
                  "status":"cancelled","created_at":"2026-08-28T12:00:00Z",
                  "expires_at":"2026-08-28T12:05:00Z","proposal_digest":"\(digest)"
                }
                """.utf8
            )
        )
        let alignment = try JSONDecoder().decode(
            LocalProductConversationControlProposal.self,
            from: Data(
                """
                {
                  "schema_version":1,"proposal_id":"proposal-align-expired-ui-1",
                  "tool_id":"loom.sessions.align.preview","tool_version":1,
                  "confirmation":"user","target_conversation_id":"session-current",
                  "target_content_digest":"\(digest)",
                  "sources":[
                    {"conversation_id":"session-one","title":"Planning",
                     "content_digest":"\(digest)","message_count":2}
                  ],
                  "context_mode":"summary_only","catalog_digest":"\(digest)",
                  "segment_id":"segment-1","attempt_id":"attempt-1","message_id":"message-1",
                  "status":"expired","created_at":"2026-08-28T12:00:00Z",
                  "expires_at":"2026-08-28T12:05:00Z","proposal_digest":"\(digest)"
                }
                """.utf8
            )
        )
        let width = 360.0
        let height = 700.0
        let size = NSSize(width: width, height: height)
        let hosting = NSHostingView(
            rootView: VStack(spacing: 12) {
                ConversationActionProposalCard(
                    proposal: action,
                    inFlight: false,
                    onDecision: { _ in }
                )
                ConversationControlProposalCard(
                    proposal: alignment,
                    alignment: nil,
                    inFlight: false,
                    onDecision: { _ in }
                )
            }
            .dynamicTypeSize(.accessibility3)
            .padding(12)
            .frame(width: width, height: height, alignment: .topLeading)
            .background(Color(nsColor: .windowBackgroundColor))
        )
        hosting.frame = NSRect(origin: .zero, size: size)
        hosting.layoutSubtreeIfNeeded()
        let bitmap = try XCTUnwrap(
            hosting.bitmapImageRepForCachingDisplay(in: hosting.bounds)
        )
        hosting.cacheDisplay(in: hosting.bounds, to: bitmap)
        XCTAssertLessThanOrEqual(hosting.fittingSize.width, width)
        XCTAssertGreaterThan(
            try XCTUnwrap(bitmap.representation(using: .png, properties: [:])).count,
            2_000
        )
    }

    func testConfirmedModelToolCardRendersRestartRecoveryAtCompactSizes() throws {
        let digest = String(repeating: "b", count: 64)
        let proposal = try JSONDecoder().decode(
            LocalProductConversationActionProposal.self,
            from: Data(
                """
                {
                  "schema_version":1,"proposal_id":"proposal-route-ui-1",
                  "tool_id":"loom.conversation.route.change.preview","tool_version":1,
                  "confirmation":"user","action":"route","argument":"",
                  "payload":{"profile_id":"conversation-deepseek-primary-r3"},
                  "target_conversation_id":"session-current","target_content_digest":"\(digest)",
                  "segment_id":"segment-1","attempt_id":"attempt-1","message_id":"message-1",
                  "status":"confirmed","created_at":"2026-08-28T12:00:00Z",
                  "expires_at":"2026-08-28T12:05:00Z","proposal_digest":"\(digest)"
                }
                """.utf8
            )
        )
        for (width, height, typeSize) in [
            (560.0, 250.0, DynamicTypeSize.large),
            (360.0, 380.0, DynamicTypeSize.accessibility2),
        ] {
            var decision: LocalProductChatControlDecision?
            let size = NSSize(width: width, height: height)
            let hosting = NSHostingView(
                rootView: ConversationActionProposalCard(
                    proposal: proposal,
                    inFlight: false,
                    onDecision: { decision = $0 }
                )
                .environment(\.dynamicTypeSize, typeSize)
                .padding(12)
                .frame(width: width, height: height, alignment: .topLeading)
                .background(Color(nsColor: .windowBackgroundColor))
            )
            hosting.frame = NSRect(origin: .zero, size: size)
            hosting.layoutSubtreeIfNeeded()
            let bitmap = try XCTUnwrap(
                hosting.bitmapImageRepForCachingDisplay(in: hosting.bounds)
            )
            hosting.cacheDisplay(in: hosting.bounds, to: bitmap)
            let imageData = try XCTUnwrap(
                bitmap.representation(using: .png, properties: [:])
            )
            XCTAssertGreaterThan(imageData.count, 2_000)
            XCTAssertLessThanOrEqual(hosting.fittingSize.width, width)
            XCTAssertNil(decision)
        }

        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/LoomWorkspaceShell.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)
        XCTAssertTrue(source.contains("loom.conversation.action-proposal.resume"))
        XCTAssertTrue(source.contains("control-\\(proposal.proposalID)"))
        XCTAssertTrue(source.contains("The model never receives its path."))
        XCTAssertTrue(source.contains("canExecuteConversationActionProposal(proposal)"))
        XCTAssertTrue(source.contains("inFlight || !proposal.isConfirmable"))
        XCTAssertTrue(source.contains("Cancel this outdated proposal"))
        XCTAssertTrue(source.contains("inFlight || !confirmable"))
        XCTAssertTrue(source.contains("Cancel this outdated alignment"))
        XCTAssertTrue(source.contains("proposal.payload?.missionID"))
        XCTAssertTrue(source.contains("await openMissionContext(missionID)"))
    }

    func testConfirmedModelToolReceiptRendersIncidentAtCompactSizes() throws {
        let proposalDigest = String(repeating: "a", count: 64)
        let bindingDigest = String(repeating: "b", count: 64)
        let capsuleDigest = String(repeating: "c", count: 64)
        let workspaceDigest = String(repeating: "d", count: 64)
        let registryDigest = String(repeating: "e", count: 64)
        let receiptDigest = String(repeating: "f", count: 64)
        let proposal = try JSONDecoder().decode(
            LocalProductConversationActionProposal.self,
            from: Data(
                """
                {
                  "schema_version":2,"proposal_id":"proposal-route-receipt-ui-1",
                  "tool_id":"loom.conversation.route.change.preview","tool_version":2,
                  "confirmation":"user","action":"route","argument":"",
                  "payload":{"profile_id":"conversation-deepseek-primary-r3"},
                  "route":{"harness_adapter":"codex","provider_id":"openai",
                    "provider_account_id":"openai.primary","credential_revision":4,
                    "model_id":"gpt-5.6-sol","reasoning_effort":"max",
                    "execution_binding_digest":"\(bindingDigest)",
                    "context_capsule_digest":"\(capsuleDigest)"},
                  "workspace":{"workspace_id":"workspace-primary",
                    "workspace_digest":"\(workspaceDigest)"},
                  "registry_digest":"\(registryDigest)",
                  "incident_id":"incident-route-proposal-1",
                  "target_conversation_id":"session-current",
                  "target_content_digest":"\(proposalDigest)",
                  "segment_id":"segment-1","attempt_id":"attempt-2",
                  "message_id":"message-2","status":"confirmed",
                  "created_at":"2026-08-28T12:00:00Z",
                  "expires_at":"2026-08-28T12:05:00Z",
                  "proposal_digest":"\(proposalDigest)"
                }
                """.utf8
            )
        )
        let receipt = try JSONDecoder().decode(
            LocalProductConversationProposalDecisionReceipt.self,
            from: Data(
                """
                {
                  "schema_version":1,"proposal_id":"proposal-route-receipt-ui-1",
                  "proposal_digest":"\(proposalDigest)",
                  "tool_id":"loom.conversation.route.change.preview",
                  "decision":"confirm","decision_incident_id":"incident-route-confirm-1",
                  "target_conversation_id":"session-current",
                  "segment_id":"segment-1","attempt_id":"attempt-2",
                  "registry_digest":"\(registryDigest)",
                  "workspace_digest":"\(workspaceDigest)",
                  "execution_binding_digest":"\(bindingDigest)",
                  "context_capsule_digest":"\(capsuleDigest)",
                  "decided_at":"2026-08-28T12:01:00Z",
                  "receipt_digest":"\(receiptDigest)"
                }
                """.utf8
            )
        )

        for (width, height, typeSize) in [
            (560.0, 300.0, DynamicTypeSize.large),
            (360.0, 460.0, DynamicTypeSize.accessibility2),
        ] {
            let size = NSSize(width: width, height: height)
            let hosting = NSHostingView(
                rootView: ConversationActionProposalCard(
                    proposal: proposal,
                    inFlight: false,
                    decisionReceipt: receipt,
                    onDecision: { _ in }
                )
                .environment(\.dynamicTypeSize, typeSize)
                .padding(12)
                .frame(width: width, height: height, alignment: .topLeading)
                .background(Color(nsColor: .windowBackgroundColor))
            )
            hosting.frame = NSRect(origin: .zero, size: size)
            hosting.layoutSubtreeIfNeeded()
            let bitmap = try XCTUnwrap(
                hosting.bitmapImageRepForCachingDisplay(in: hosting.bounds)
            )
            hosting.cacheDisplay(in: hosting.bounds, to: bitmap)
            let imageData = try XCTUnwrap(
                bitmap.representation(using: .png, properties: [:])
            )
            XCTAssertGreaterThan(imageData.count, 2_000)
            XCTAssertLessThanOrEqual(hosting.fittingSize.width, width)

            if let captureRoot = ProcessInfo.processInfo.environment[
                "LOOM_PHASE7_SCREENSHOT_DIR"
            ] {
                let directory = URL(fileURLWithPath: captureRoot, isDirectory: true)
                try FileManager.default.createDirectory(
                    at: directory,
                    withIntermediateDirectories: true
                )
                try imageData.write(
                    to: directory.appendingPathComponent(
                        "action-proposal-receipt-\(Int(width)).png"
                    )
                )
            }
        }

        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/LoomWorkspaceShell.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)
        XCTAssertTrue(source.contains("proposalDecisionIncidentRow"))
        XCTAssertTrue(source.contains("Copy incident ID"))
        XCTAssertTrue(source.contains("historical approval is read-only"))
    }

    func testV2MissionToolCardNamesTheExactTargetAtCompactSizes() throws {
        let digest = String(repeating: "a", count: 64)
        let bindingDigest = String(repeating: "b", count: 64)
        let capsuleDigest = String(repeating: "c", count: 64)
        let workspaceDigest = String(repeating: "d", count: 64)
        let registryDigest = String(repeating: "e", count: 64)
        let proposal = try JSONDecoder().decode(
            LocalProductConversationActionProposal.self,
            from: Data(
                """
                {
                  "schema_version":2,"proposal_id":"proposal-mission-v2-ui-1",
                  "tool_id":"loom.missions.continue.preview","tool_version":2,
                  "confirmation":"user","action":"continue_mission",
                  "argument":"Retry only the failed accessibility verification while preserving the accepted design constraints.",
                  "payload":{"mission_id":"mission-release-7"},
                  "route":{"harness_adapter":"codex","provider_id":"openai",
                    "provider_account_id":"openai.primary","credential_revision":4,
                    "model_id":"gpt-5.6-sol","reasoning_effort":"max",
                    "execution_binding_digest":"\(bindingDigest)",
                    "context_capsule_digest":"\(capsuleDigest)"},
                  "workspace":{"workspace_id":"workspace-primary",
                    "workspace_digest":"\(workspaceDigest)"},
                  "registry_digest":"\(registryDigest)",
                  "incident_id":"incident-mission-v2-ui-1",
                  "target_conversation_id":"session-current",
                  "target_content_digest":"\(digest)",
                  "segment_id":"segment-1","attempt_id":"attempt-1",
                  "message_id":"message-1","status":"pending",
                  "created_at":"2026-08-28T12:00:00Z",
                  "expires_at":"2026-08-28T12:05:00Z",
                  "proposal_digest":"\(digest)"
                }
                """.utf8
            )
        )

        for (width, height, typeSize) in [
            (560.0, 300.0, DynamicTypeSize.large),
            (360.0, 440.0, DynamicTypeSize.accessibility2),
        ] {
            var decision: LocalProductChatControlDecision?
            let size = NSSize(width: width, height: height)
            let hosting = NSHostingView(
                rootView: ConversationActionProposalCard(
                    proposal: proposal,
                    inFlight: false,
                    targetDisplayName: "Release accessibility pass",
                    onDecision: { decision = $0 }
                )
                .environment(\.dynamicTypeSize, typeSize)
                .environment(\.colorScheme, .light)
                .padding(12)
                .frame(width: width, height: height, alignment: .topLeading)
                .background(Color(nsColor: .windowBackgroundColor))
            )
            hosting.frame = NSRect(origin: .zero, size: size)
            hosting.layoutSubtreeIfNeeded()
            let bitmap = try XCTUnwrap(
                hosting.bitmapImageRepForCachingDisplay(in: hosting.bounds)
            )
            hosting.cacheDisplay(in: hosting.bounds, to: bitmap)
            let imageData = try XCTUnwrap(
                bitmap.representation(using: .png, properties: [:])
            )
            XCTAssertGreaterThan(imageData.count, 2_000)
            XCTAssertLessThanOrEqual(hosting.fittingSize.width, width)
            XCTAssertNil(decision)

            if let captureRoot = ProcessInfo.processInfo.environment[
                "LOOM_PHASE7_SCREENSHOT_DIR"
            ] {
                let directory = URL(fileURLWithPath: captureRoot, isDirectory: true)
                try FileManager.default.createDirectory(
                    at: directory,
                    withIntermediateDirectories: true
                )
                try imageData.write(
                    to: directory.appendingPathComponent(
                        "action-proposal-v2-mission-\(Int(width)).png"
                    )
                )
            }
        }
    }

    func testPhase5CompactMissionRendersAtLargestAccessibilityText()
        async throws
    {
        let snapshot = try phase5MissionContextSnapshot()
        let fixtureRoot = FileManager.default.temporaryDirectory
            .appendingPathComponent(UUID().uuidString, isDirectory: true)
        let store = LocalProductStore(
            client: MissionWorkbenchStubClient(snapshot: snapshot),
            chatSessionsFileURL: fixtureRoot.appendingPathComponent(
                "chat-sessions.json"
            )
        )
        await store.refresh()
        let missionOpened = await store.openMissionAndActivate(
            "mission/team-first"
        )
        XCTAssertTrue(missionOpened)

        let size = NSSize(width: 900, height: 760)
        let hosting = NSHostingView(
            rootView: LoomWorkspaceShell(
                store: store,
                initialGovernanceDestination: .mission
            )
            .dynamicTypeSize(.accessibility5)
            .frame(width: size.width, height: size.height)
        )
        hosting.frame = NSRect(origin: .zero, size: size)
        hosting.layoutSubtreeIfNeeded()

        let bitmap = try XCTUnwrap(
            hosting.bitmapImageRepForCachingDisplay(in: hosting.bounds)
        )
        hosting.cacheDisplay(in: hosting.bounds, to: bitmap)
        XCTAssertEqual(bitmap.size.width, size.width)
        XCTAssertEqual(bitmap.size.height, size.height)
        XCTAssertGreaterThanOrEqual(bitmap.pixelsWide, Int(size.width))
        XCTAssertGreaterThanOrEqual(bitmap.pixelsHigh, Int(size.height))
        XCTAssertLessThanOrEqual(hosting.fittingSize.width, size.width)

        if let captureRoot = ProcessInfo.processInfo.environment[
            "LOOM_PHASE5_SCREENSHOT_DIR"
        ] {
            let directory = URL(fileURLWithPath: captureRoot, isDirectory: true)
            try FileManager.default.createDirectory(
                at: directory,
                withIntermediateDirectories: true
            )
            let imageData = try XCTUnwrap(
                bitmap.representation(using: .png, properties: [:])
            )
            try imageData.write(
                to: directory.appendingPathComponent(
                    "shell-compact-largest-text.png"
                )
            )
        }
    }

    func testPhase5MissionAndRoundTableContextsRenderBesideConversation()
        async throws
    {
        let snapshot = try phase5MissionContextSnapshot()
        let fixtureRoot = FileManager.default.temporaryDirectory
            .appendingPathComponent(UUID().uuidString, isDirectory: true)
        let store = LocalProductStore(
            client: MissionWorkbenchStubClient(snapshot: snapshot),
            chatSessionsFileURL: fixtureRoot.appendingPathComponent(
                "chat-sessions.json"
            )
        )
        await store.refresh()
        let missionOpened = await store.openMissionAndActivate(
            "mission/team-first"
        )
        XCTAssertTrue(missionOpened)

        let cases: [(LoomGovernanceDestination, Double)] = [
            (.mission, 1_280),
            (.mission, 900),
            (.roundtable, 1_280),
            (.roundtable, 900),
        ]
        for (destination, width) in cases {
            let size = NSSize(width: width, height: 760)
            let hosting = NSHostingView(
                rootView: LoomWorkspaceShell(
                    store: store,
                    initialGovernanceDestination: destination
                )
                .frame(width: width, height: size.height)
            )
            hosting.frame = NSRect(origin: .zero, size: size)
            hosting.layoutSubtreeIfNeeded()

            let bitmap = try XCTUnwrap(
                hosting.bitmapImageRepForCachingDisplay(in: hosting.bounds)
            )
            hosting.cacheDisplay(in: hosting.bounds, to: bitmap)
            XCTAssertEqual(bitmap.size.width, width)
            XCTAssertEqual(bitmap.size.height, size.height)
            XCTAssertGreaterThan(hosting.fittingSize.width, 500)
            XCTAssertLessThanOrEqual(hosting.fittingSize.width, width)

            if let captureRoot = ProcessInfo.processInfo.environment[
                "LOOM_PHASE5_SCREENSHOT_DIR"
            ] {
                let directory = URL(
                    fileURLWithPath: captureRoot,
                    isDirectory: true
                )
                try FileManager.default.createDirectory(
                    at: directory,
                    withIntermediateDirectories: true
                )
                let imageData = try XCTUnwrap(
                    bitmap.representation(using: .png, properties: [:])
                )
                try imageData.write(
                    to: directory.appendingPathComponent(
                        "\(destination.rawValue.lowercased())-\(Int(width)).png"
                    )
                )
            }
        }
    }

    private func phase5MissionContextSnapshot() throws
        -> LocalProductSnapshot
    {
        let base = try ExperienceFixtures.populatedSnapshot()
        let digest = String(repeating: "a", count: 64)
        let mission = try JSONDecoder().decode(
            LocalProductMissionSummary.self,
            from: Data(
                """
                {
                  "schema_version":1,
                  "mission_id":"mission/team-first",
                  "team_instance_id":"team-first",
                  "title":"Restore provider access and verify the release workflow",
                  "source_kind":"team_execution",
                  "lane":"Review",
                  "status":"blocked",
                  "priority":"high",
                  "plan_digest":"\(digest)",
                  "simple":false,
                  "node_count":2,
                  "completed_node_count":1,
                  "active_node_count":0,
                  "review_node_count":0,
                  "attention_count":1,
                  "current_node_id":"review",
                  "last_milestone":"The implementation completed, but provider verification needs user guidance.",
                  "block_reason":"The selected Provider Account requires review before a new Attempt can run.",
                  "team_pulse":[],
                  "topology":[]
                }
                """.utf8
            )
        )
        return LocalProductSnapshot(
            schemaVersion: 2,
            viewVersion: base.viewVersion,
            health: base.health,
            runtimes: base.runtimes,
            teams: base.teams,
            missions: [mission],
            runs: base.runs,
            evidence: base.evidence,
            attention: base.attention
        )
    }

    func testContentViewStartsSetupRecoveryAlongsideInitialRefreshes() throws {
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/ContentView.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)
        let recovery = try XCTUnwrap(
            source.range(of: "async let recovery: Void = store.reconnectWhileUnavailable()")
        )
        let setup = try XCTUnwrap(
            source.range(of: "async let setup: Void = store.refreshSetup()")
        )
        let refreshJoin = try XCTUnwrap(
            source.range(of: "_ = await (setup, permissions, executions, production)")
        )
        let recoveryJoin = try XCTUnwrap(source.range(of: "await recovery"))

        XCTAssertLessThan(recovery.lowerBound, setup.lowerBound)
        XCTAssertLessThan(setup.lowerBound, refreshJoin.lowerBound)
        XCTAssertLessThan(refreshJoin.lowerBound, recoveryJoin.lowerBound)
    }

    func testRegisteredServiceUsesBoundedStartupProbeBeforeBundledFallback() throws {
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalApp/LoomLocalApp.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)

        XCTAssertTrue(source.contains("maxAttempts: 12"))
        XCTAssertTrue(source.contains("delayNanoseconds: 250_000_000"))
        XCTAssertTrue(source.contains("guard await serviceProcessHost.start()"))
        XCTAssertTrue(source.contains("if await serviceProcessHost.waitForDefaultSocket()"))
    }

    func testAdoptedServiceSocketIsMonitoredAcrossRapidAppRestart() throws {
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalApp/LocalServiceProcessHost.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)

        XCTAssertTrue(source.contains("adoptedSocketMonitorTask"))
        XCTAssertTrue(source.contains("scheduleAdoptedServiceMonitor()"))
        XCTAssertTrue(source.contains("guard !socketIsReachable else { continue }"))
        XCTAssertTrue(source.contains("_ = await self.start()"))
    }

    func testRoundtableSupportsDragDropAndSeatRemoval() throws {
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/RoundtableWorkbench.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)

        XCTAssertTrue(source.contains(".draggable(candidate.id)"))
        XCTAssertTrue(source.contains(".dropDestination(for: String.self)"))
        XCTAssertTrue(source.contains("addRoundtableAgents([candidate])"))
        XCTAssertTrue(source.contains("addRoundtableAgents(candidates)"))
        XCTAssertTrue(source.contains("RoundTable Agent drop zone"))
        XCTAssertTrue(source.contains(".accessibilityElement(children: .contain)"))
        XCTAssertTrue(source.contains("The original discussion prompt remains protected"))
        XCTAssertTrue(source.contains("retireRoundtableAgent(candidate)"))
        XCTAssertTrue(source.contains("store.roundtableRetireSeat("))
        XCTAssertTrue(source.contains("Seats are frozen after the round opens"))
        XCTAssertTrue(source.contains("2 minimum · 6 maximum"))
        XCTAssertTrue(source.contains("Lead"))
        XCTAssertTrue(source.contains("Participant \\(index)"))
        XCTAssertTrue(source.contains("RoundTable starts from a Mission"))
        XCTAssertTrue(source.contains("onOpenMissions?()"))
        XCTAssertTrue(source.contains("Start Mission discussion"))
        XCTAssertTrue(source.contains("Imported archive · Read only"))
        XCTAssertTrue(source.contains("This file is a read-only record."))
        XCTAssertTrue(source.contains("if importedArchive"))
        XCTAssertTrue(source.contains("view.session.concluded && !importedArchive"))
        let concludedStatus = try XCTUnwrap(
            source.range(of: "else if view?.session.concluded == true")
        )
        let inputNeededStatus = try XCTUnwrap(
            source.range(of: "else if activeAttemptFingerprint.isEmpty && hasInterventionNeeded")
        )
        XCTAssertLessThan(concludedStatus.lowerBound, inputNeededStatus.lowerBound)
        XCTAssertTrue(source.contains("Discussion concluded"))
        XCTAssertTrue(source.contains("Explicit Retry is running"))
        XCTAssertTrue(source.contains(".submitLabel(.send)"))
        XCTAssertTrue(source.contains("Copy Agent incident ID"))
        XCTAssertTrue(source.contains("Other Agents remain available"))
        XCTAssertTrue(source.contains("Replace or skip this Agent"))
        XCTAssertTrue(source.contains("Start synthesis round"))
        XCTAssertTrue(source.contains("loom.roundtable.synthesis-prompt"))
        XCTAssertTrue(source.contains(".onKeyPress(.return)"))
        XCTAssertTrue(source.contains("submitRoundtableIntervention(candidate, attempt: attempt)"))
		XCTAssertTrue(source.contains("roundtableLiveInputState"))
		XCTAssertTrue(source.contains("Retry after completion"))
		XCTAssertTrue(source.contains("Preparing live controls"))
		XCTAssertTrue(source.contains("This Runtime cannot accept live guidance"))
    }

    func testStandaloneRoundtableRoutesIntoMissionInsteadOfLegacyLedgerCreation() throws {
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/LoomWorkspaceShell.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)

        XCTAssertTrue(source.contains("RoundtableWorkbench("))
        XCTAssertTrue(source.contains("missionLink: pendingRoundtableMissionLink"))
        XCTAssertTrue(source.contains("selectNavigation(.work)"))
        XCTAssertTrue(source.contains("Mission RoundTable"))
    }

    func testMissionRoundtableCandidatesUseExactTeamRolesBeforeRuntimeFallback() {
        let candidates = [
            LocalRoundtableAgentCandidate(
                id: "main:a:p1", teamRoleID: "planner", seatID: "seat-planner",
                displayName: "Planner", responsibility: "Plan", routeSummary: "Route A",
                runtimeInstanceID: "runtime-shared"
            ),
            LocalRoundtableAgentCandidate(
                id: "subagent:b:p2", teamRoleID: "reviewer", seatID: "seat-reviewer",
                displayName: "Reviewer", responsibility: "Review", routeSummary: "Route B",
                runtimeInstanceID: "runtime-shared"
            ),
            LocalRoundtableAgentCandidate(
                id: "subagent:c:p3", teamRoleID: "other-team-role", seatID: "seat-other",
                displayName: "Other Team", responsibility: "Other", routeSummary: "Route C",
                runtimeInstanceID: "runtime-shared"
            ),
        ]
        let link = LocalRoundtableMissionLink(
            conversationID: "conversation-1", missionID: "mission/team-1",
            teamInstanceID: "team-1", title: "Discuss",
            teamRoleIDs: ["planner", "reviewer"], runtimeInstanceIDs: ["runtime-shared"]
        )

        XCTAssertEqual(
            roundtableMissionAgentCandidates(candidates, link: link).map(\.teamRoleID),
            ["planner", "reviewer"]
        )

        let topologyLink = LocalRoundtableMissionLink(
            conversationID: "conversation-1", missionID: "mission/team-1",
            teamInstanceID: "team-1", title: "Discuss",
            teamRoleIDs: ["main", "mission-role-generated"],
            runtimeInstanceIDs: ["runtime-shared"]
        )
        XCTAssertEqual(
            roundtableMissionAgentCandidates(candidates, link: topologyLink)
                .map(\.teamRoleID),
            ["planner", "reviewer", "other-team-role"]
        )
    }

    func testConversationContextMeterUsesFrozenSegmentBudget() throws {
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/LoomWorkspaceShell.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)

        XCTAssertTrue(source.contains("Context capacity"))
        XCTAssertTrue(source.contains("Exact capacity"))
        XCTAssertTrue(source.contains("Estimated capacity"))
        XCTAssertTrue(source.contains("Capacity unavailable"))
        XCTAssertFalse(source.contains("Estimated shared context"))
        XCTAssertTrue(source.contains("disclosure.contextTokenCount"))
        XCTAssertTrue(source.contains("disclosure.contextTokenBudget"))
        XCTAssertTrue(source.contains("disclosure.contextWindowTokens"))
        XCTAssertTrue(source.contains("disclosure.reservedOutputTokens"))
        XCTAssertTrue(source.contains("disclosure.adapterToolOverheadTokens"))
        XCTAssertTrue(source.contains("disclosure.admittedInputBudgetTokens"))
        XCTAssertTrue(source.contains("disclosure.contextCapacityContributions"))
        XCTAssertTrue(source.contains("Budget omissions"))
        XCTAssertFalse(source.contains("disclosureItem.content"))
        XCTAssertFalse(source.contains("sourceRef"))
        XCTAssertFalse(source.contains("itemID"))
    }

    func testConversationContextCapacityDetailsFit560AtLargeDynamicType() {
        let contributions = [
            LocalProductContextCapacityContribution(
                priority: 0,
                sourceType: "authority",
                admittedItemCount: 2,
                admittedTokenCount: 2_400,
                budgetOmittedItemCount: 0,
                budgetOmittedTokenCount: 0
            ),
            LocalProductContextCapacityContribution(
                priority: 3,
                sourceType: "model_output",
                admittedItemCount: 1,
                admittedTokenCount: 600,
                budgetOmittedItemCount: 2,
                budgetOmittedTokenCount: 900
            ),
        ]
        let segment = LocalProductConversationSegment(
            segmentID: "segment-capacity",
            profileID: "conversation-deepseek",
            contextMode: .summaryOnly,
            contextCapsuleDigest: String(repeating: "a", count: 64),
            disclosureReceiptDigest: String(repeating: "c", count: 64),
            disclosedContextCount: 3,
            omittedContextCount: 3,
            contextTokenBudget: 120_000,
            contextTokenCount: 3_000,
            contextCapacityStatus: .exact,
            contextWindowTokens: 128_000,
            reservedOutputTokens: 8_192,
            adapterToolOverheadTokens: 1_024,
            admittedInputBudgetTokens: 118_784,
            contextTokenCounterID: "loom-token-counter",
            contextTokenCounterVersion: "v1",
            admittedContributionTokens: 3_000,
            budgetOmittedContributionTokens: 900,
            contextCapacityContributions: contributions,
            bindingDigest: String(repeating: "b", count: 64)
        )
        let hosting = NSHostingView(
            rootView: ScrollView {
                ConversationContextCapacityDetails(disclosure: segment)
                    .padding(16)
            }
            .environment(\.dynamicTypeSize, .accessibility3)
            .frame(width: 560, height: 520)
        )
        hosting.frame = NSRect(x: 0, y: 0, width: 560, height: 520)
        hosting.layoutSubtreeIfNeeded()

        XCTAssertLessThanOrEqual(hosting.fittingSize.width, 560)
        XCTAssertGreaterThan(hosting.fittingSize.height, 200)
        XCTAssertNotNil(hosting.bitmapImageRepForCachingDisplay(in: hosting.bounds))
    }

    func testConversationLinkedMissionUsesAsyncActivationAndAccessibleTarget() throws {
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/LoomWorkspaceShell.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)
        let linkedMissionStart = try XCTUnwrap(
            source.range(of: "ForEach(linkedMissions.prefix(8))")
        )
        let linkedMissionEnd = try XCTUnwrap(
            source.range(of: "Divider()", range: linkedMissionStart.upperBound..<source.endIndex)
        )
        let linkedMissionSurface = String(
            source[linkedMissionStart.lowerBound..<linkedMissionEnd.lowerBound]
        )

        XCTAssertTrue(linkedMissionSurface.contains("await openMissionContext(mission)"))
        XCTAssertFalse(linkedMissionSurface.contains("store.openMission(mission.missionID)"))
        XCTAssertFalse(
            linkedMissionSurface.contains("fullGovernancePresentation = .workbench")
        )
        XCTAssertTrue(linkedMissionSurface.contains(".loomActionTarget()"))
        XCTAssertTrue(source.contains("await openMissionContext(mission.missionID)"))
        XCTAssertTrue(source.contains("await store.openMissionAndActivate(missionID)"))
        XCTAssertTrue(source.contains("governance.open(.mission)"))
    }

    func testPhase5MissionContextKeepsLiveOutputAndInterventionBesideConversation() throws {
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/LoomWorkspaceShell.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)

        XCTAssertTrue(source.contains("private var missionContextPanel: some View"))
        XCTAssertTrue(source.contains("missionActivityEntries("))
        XCTAssertTrue(source.contains("Blocked Mission intervention"))
        XCTAssertTrue(source.contains("Tell the Team what to change or try next"))
        XCTAssertTrue(source.contains(
            ".accessibilityLabel(\"Mission continuation guidance\")"
        ))
        XCTAssertTrue(source.contains(
            ".accessibilityIdentifier(\"loom.mission.continuation-guidance\")"
        ))
        XCTAssertTrue(source.contains("Open full Mission details"))
        XCTAssertTrue(source.contains("startRoundTableFromMission"))
        XCTAssertTrue(source.contains("alignConversationWithMission"))
        XCTAssertTrue(source.contains("missionLink: pendingRoundtableMissionLink"))
        XCTAssertTrue(source.contains("failureDiagnosticAvailable"))
        XCTAssertTrue(source.contains("Copy Agent incident ID"))
    }

    func testPhase5RoundTableContextRestoresLiveProgressAndRecovery() throws {
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/LoomWorkspaceShell.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)

        XCTAssertTrue(source.contains("private var roundtableContextPanel: some View"))
        XCTAssertTrue(source.contains("roundtableSessionID("))
        XCTAssertTrue(source.contains("roundtableSnapshot("))
        XCTAssertTrue(source.contains(
            "if !store.roundtableIsPreparing, let loadError"
        ))
        XCTAssertTrue(source.contains("for attempt in 0..<90"))
        XCTAssertTrue(source.contains(
            "The discussion is taking longer to restore. Retry without leaving this Mission."
        ))
        XCTAssertTrue(source.contains("Agent discussion"))
        XCTAssertTrue(source.contains("Copy incident ID"))
        XCTAssertTrue(source.contains("roundtableOperationFailure"))
        XCTAssertTrue(source.contains("View RoundTable diagnostics"))
        XCTAssertTrue(source.contains("Copy RoundTable incident ID"))
        XCTAssertTrue(source.contains("retryRoundtableContextSeat"))
        XCTAssertTrue(source.contains("steerRoundtableContextSeat"))
        XCTAssertTrue(source.contains(
            ".accessibilityLabel(\"Guidance for \\(seat.displayName)\")"
        ))
        XCTAssertTrue(source.contains(
            ".accessibilityLabel(\"Retry guidance for \\(seat.displayName)\")"
        ))
        XCTAssertTrue(source.contains(
            "loom.roundtable.live-guidance.\\(attempt.seatID)"
        ))
        XCTAssertTrue(source.contains(
            "loom.roundtable.retry-guidance.\\(attempt.seatID)"
        ))
        XCTAssertTrue(source.contains("Open full discussion"))
        XCTAssertTrue(source.contains("roundtableContextAttributedOutput"))
        XCTAssertTrue(source.contains("Show full result"))
        XCTAssertTrue(source.contains("Show less"))
        XCTAssertTrue(source.contains("roundtableContextPreviewLimit"))
        XCTAssertTrue(source.contains(
            "Show full result from \\(seat.displayName)"
        ))
        XCTAssertTrue(source.contains("Agent result"))
        XCTAssertTrue(source.contains("Synthesize Agent results"))
        XCTAssertTrue(source.contains("Refine synthesis"))
        XCTAssertTrue(source.contains("startRoundtableContextSynthesis"))
        XCTAssertTrue(source.contains(
            "loom.roundtable.context-synthesis-prompt"
        ))
        XCTAssertTrue(source.contains("Accept Lead result"))
        XCTAssertTrue(source.contains("Accept Lead synthesis"))
        XCTAssertTrue(source.contains("Follow-up round"))
        XCTAssertFalse(source.contains("visible updates"))
        XCTAssertTrue(source.contains("governance.open(.roundtable)"))
        XCTAssertTrue(source.contains("Section(\"Current Mission\")"))
        XCTAssertTrue(source.contains("Section(\"Workspace\")"))
        XCTAssertFalse(
            source.contains("ForEach(LoomGovernanceDestination.allCases)")
        )
    }

    func testPhase5RoundTableContextKeepsActionsInTheKeyboardFocusChain() throws {
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/LoomWorkspaceShell.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)

        XCTAssertTrue(source.contains(
            "Button(resultExpanded ? \"Show less\" : \"Show full result\")"
        ))
        XCTAssertTrue(source.contains(
            "                    .buttonStyle(.plain)\n"
                + "                    .font(.caption)\n"
                + "                    .accessibilityLabel("
        ))
        XCTAssertTrue(source.contains(
            "Label(\n"
                + "                    \"Open full discussion\",\n"
                + "                    systemImage: \"arrow.up.right.square\"\n"
                + "                )\n"
                + "            }\n"
                + "            .buttonStyle(.plain)"
        ))
        XCTAssertFalse(source.contains(".focusable()"))
    }

    func testPhase5MissionListAndDetailsHaveVisibleKeyboardDestinations() throws {
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/LoomWorkspaceShell.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)

        XCTAssertTrue(source.contains(
            "Button {\n"
                + "                store.showMissionBoard()\n"
                + "                fullGovernancePresentation = .workbench\n"
                + "                governance.open(.board)\n"
                + "            } label: {\n"
                + "                Label(\"All Missions\", systemImage: \"list.bullet\")"
        ))
        XCTAssertTrue(source.contains(
            "Label(\"Details\", systemImage: \"arrow.up.right.square\")\n"
                + "                    }\n"
                + "                    .buttonStyle(.plain)\n"
                + "                    .accessibilityLabel(\"Open full Mission details\")"
        ))
    }

    func testPhase5MissionFailureOffersRetryDiagnosticsAndIncidentActions() throws {
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/MissionWorkbench.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)

        XCTAssertTrue(source.contains("store.missionOperationFailure"))
        XCTAssertTrue(source.contains("Label(\"Retry preflight\", systemImage: \"arrow.clockwise\")"))
        XCTAssertTrue(source.contains("Label(\"View diagnostics\", systemImage: \"doc.text.magnifyingglass\")"))
        XCTAssertTrue(source.contains("Copy Mission incident ID"))
        XCTAssertTrue(source.contains(".sheet(item: $missionDiagnosticPreview)"))
        XCTAssertTrue(source.contains("prepareMissionDiagnosticPreview()"))
        XCTAssertTrue(source.contains("if store.missionOperationFailure == nil"))
    }

    func testPhase5FullRoundTableUsesStructuredRecoverableErrors() throws {
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/RoundtableWorkbench.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)

        XCTAssertTrue(source.contains("store.roundtableOperationFailure"))
        XCTAssertTrue(source.contains("View RoundTable diagnostics"))
        XCTAssertTrue(source.contains("Copy RoundTable incident ID"))
        XCTAssertTrue(source.contains("failure.recoveryAction"))
        XCTAssertTrue(source.contains("failure.recoverable"))
    }

    func testPhase5DiagnosticPreviewFailureStaysVisibleAndRecoverable() throws {
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/LoomWorkspaceShell.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)

        XCTAssertTrue(source.contains("diagnosticPreparationError"))
        XCTAssertTrue(source.contains("Diagnostics could not be prepared"))
        XCTAssertTrue(source.contains("Open Runtime & Providers"))
        XCTAssertTrue(source.contains("fullGovernancePresentation = .runtimeProviders"))
        XCTAssertFalse(source.contains(
            "} catch {\n                fullGovernancePresentation = .runtimeProviders"
        ))
    }

    func testPhase5FolderSelectionFailureOffersInPlaceRetry() throws {
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/LoomWorkspaceShell.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)

        XCTAssertTrue(source.contains("folderSelectionError"))
        XCTAssertTrue(source.contains("Folder could not be opened"))
        XCTAssertTrue(source.contains("Button(\"Try Again\")"))
        XCTAssertTrue(source.contains("NSUserCancelledError"))
        XCTAssertFalse(source.contains(
            "guard case .success(let urls) = result, let url = urls.first else { return }"
        ))
    }

    func testPhase5NewTaskCommandUsesTheConversationAction() throws {
        let testsURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
        let appSource = try String(
            contentsOf: testsURL
                .appendingPathComponent("Sources/LoomLocalApp/LoomLocalApp.swift"),
            encoding: .utf8
        )
        let shellSource = try String(
            contentsOf: testsURL
                .appendingPathComponent("Sources/LoomLocalAppUI/LoomWorkspaceShell.swift"),
            encoding: .utf8
        )

        XCTAssertTrue(appSource.contains("CommandGroup(replacing: .newItem)"))
        XCTAssertTrue(appSource.contains("Button(\"New Task\")"))
        XCTAssertTrue(appSource.contains(
            ".keyboardShortcut(\"n\", modifiers: .command)"
        ))
        XCTAssertTrue(appSource.contains(".loomNewTaskRequested"))
        XCTAssertTrue(shellSource.contains("startNewTask()"))
        XCTAssertTrue(shellSource.contains(
            "NotificationCenter.default.publisher(for: .loomNewTaskRequested)"
        ))
        XCTAssertTrue(shellSource.contains("composerFocused = true"))
    }

  func testWorkspaceShellConsumesAgentTeamBuilderSession() throws {
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/LoomWorkspaceShell.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)

        XCTAssertTrue(source.contains("if let session = store.builderSession"))
        XCTAssertTrue(source.contains("await store.answerBuilder(option.id)"))
        XCTAssertTrue(source.contains("await store.confirmBuilderCommitting("))
        XCTAssertTrue(source.contains("store.cancelBuilder()"))
        XCTAssertTrue(source.contains("store.builderRecoveryMessage"))
        XCTAssertTrue(source.contains("Loom proposal, untrusted"))
        XCTAssertTrue(source.contains(#"message.role == "proposal""#))
		XCTAssertTrue(source.contains("conversationDisclosureSummary"))
		XCTAssertTrue(source.contains("Context shared"))
		XCTAssertTrue(source.contains("Disclosure receipt"))
		XCTAssertTrue(source.contains("disclosureReceiptDigest"))
		XCTAssertTrue(source.contains("Inspect context"))
		XCTAssertTrue(source.contains("loadConversationContextDisclosure"))
		XCTAssertTrue(source.contains("conversationContextDisclosureIdentity(for: disclosure)"))
		XCTAssertFalse(source.contains("conversationContextDisclosures[\n                                    disclosure.contextCapsuleDigest"))
		XCTAssertTrue(source.contains("omissionReason"))
		XCTAssertTrue(source.contains("Agent can retrieve by scope"))
		XCTAssertFalse(source.contains("Scoped retrieval available"))
		XCTAssertFalse(source.contains("disclosureItem.content"))
		XCTAssertTrue(source.contains("Provider account"))
		XCTAssertTrue(source.contains("Account policy"))
		XCTAssertTrue(source.contains("Trust domain"))
		XCTAssertTrue(source.contains("Retention"))
		XCTAssertTrue(source.contains("Data region"))
		XCTAssertTrue(source.contains("Text(\"Incident \\(failure.incidentID)\")"))
		XCTAssertFalse(source.contains(#"message.tentative ? "Loom proposal""#))
		XCTAssertTrue(source.contains("LocalProductRoleOptionChoice.all"))
		XCTAssertTrue(source.contains("await store.editBuilder("))
		XCTAssertTrue(source.contains("Parallel routes"))
		XCTAssertTrue(source.contains("_parallel_route_add"))
		XCTAssertTrue(source.contains("_parallel_route_remove"))
		XCTAssertTrue(source.contains("Synthesis ·"))
		XCTAssertTrue(source.contains("choice.providerAccount"))
		XCTAssertTrue(source.contains("review.credential"))
		XCTAssertTrue(source.contains("review.reasoning"))
		XCTAssertTrue(source.contains("review.timeout"))
		XCTAssertTrue(source.contains("review.budget"))
		XCTAssertTrue(source.contains("builderRoleProfileControls"))
		XCTAssertTrue(source.contains("LocalProductRoleOptionChoice.fallbacks"))
		XCTAssertTrue(source.contains("_fallback_role"))
		XCTAssertTrue(source.contains("Fallback route"))
		XCTAssertTrue(source.contains("Approval required before route change"))
		XCTAssertTrue(source.contains("Choose Harness"))
		XCTAssertTrue(source.contains("Choose Provider Account"))
		XCTAssertTrue(source.contains("Choose Agent role"))
		XCTAssertTrue(source.contains("_provider_account_route"))
		XCTAssertTrue(source.contains("_model"))
		XCTAssertTrue(source.contains("_reasoning_effort"))
		XCTAssertTrue(source.contains("_timeout_seconds"))
		XCTAssertTrue(source.contains("_budget_units"))
    }

    func testChatProposalEscalatesToMissionWithPrefilledObjective() throws {
    let sourceURL = URL(fileURLWithPath: #filePath)
      .deletingLastPathComponent()
      .deletingLastPathComponent()
      .deletingLastPathComponent()
      .appendingPathComponent("Sources/LoomLocalAppUI/LoomWorkspaceShell.swift")
    let source = try String(contentsOf: sourceURL, encoding: .utf8)

    // A Loom proposal must offer a one-click governed escalation.
    XCTAssertTrue(source.contains(#"Label("Run as Mission", systemImage: "play.circle")"#))
    XCTAssertTrue(source.contains("runMissionFromChat(message)"))
    XCTAssertTrue(source.contains("pendingMissionObjective = objective"))
    XCTAssertTrue(source.contains("fullGovernancePresentation = .newMission"))
    XCTAssertTrue(source.contains("chatMissionObjective("))
  }

  func testNewMissionSheetAutoSelectsFirstExecutableTeamAndExplainsDisabledReview() throws {
    let sourceURL = URL(fileURLWithPath: #filePath)
      .deletingLastPathComponent()
      .deletingLastPathComponent()
      .deletingLastPathComponent()
      .appendingPathComponent("Sources/LoomLocalAppUI/MissionWorkbench.swift")
    let source = try String(contentsOf: sourceURL, encoding: .utf8)

    // Every entry path into the New Mission sheet uses the same preparation
    // path. A new Mission remains distinct from the explicit continuation path.
    XCTAssertTrue(
      source.contains("prepareNewMission()")
    )
    XCTAssertTrue(source.contains("newMissionStartsNewAttempt = false"))
    XCTAssertTrue(source.contains(".disabled(newMissionStartsNewAttempt)"))
    // The disabled Review-preflight action must explain what is missing.
    XCTAssertTrue(source.contains("missionReviewBlockedReason"))
    XCTAssertTrue(source.contains("Enter a Mission objective first."))
    XCTAssertTrue(source.contains("Select a Team to run this Mission."))
    XCTAssertTrue(source.contains("Confirm the Mission context to enable preflight review."))
  }

  func testWorkspaceShellOffersMissionFromConversationContext() throws {
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/LoomWorkspaceShell.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)

        XCTAssertTrue(source.contains(
            "Label(\"Start Mission\", systemImage: \"flag.checkered\")"
        ))
        XCTAssertTrue(source.contains("fullGovernancePresentation = .newMission"))
        XCTAssertTrue(source.contains("showNewMissionInitially: presentation == .newMission"))
    }

    func testMissionStartReturnsToExactConversationMissionContext() throws {
        let sourceRoot = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI")
        let workbench = try String(
            contentsOf: sourceRoot.appendingPathComponent("MissionWorkbench.swift"),
            encoding: .utf8
        )
        let shell = try String(
            contentsOf: sourceRoot.appendingPathComponent("LoomWorkspaceShell.swift"),
            encoding: .utf8
        )

        XCTAssertTrue(workbench.contains(
            "onReturnToConversation: ((String) -> Void)? = nil"
        ))
        XCTAssertGreaterThanOrEqual(
            workbench.components(separatedBy: "onReturnToConversation(missionID)").count - 1,
            2,
            "Both successful Mission start and Open conversation must carry the exact Mission ID"
        )
        XCTAssertTrue(shell.contains("onReturnToConversation: { missionID in"))
        XCTAssertTrue(shell.contains("fullGovernancePresentation = nil"))
        XCTAssertTrue(shell.contains("await openMissionContext(missionID)"))
    }

    func testConversationRouteSwitchUsesExplicitSegmentReview() throws {
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/LoomWorkspaceShell.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)

        XCTAssertTrue(source.contains("requestConversationProfileSelection"))
        XCTAssertTrue(source.contains("requestConversationDispatchTransition"))
        XCTAssertGreaterThanOrEqual(
            source.components(separatedBy: "sendConversationDraft()").count - 1,
            3
        )
        XCTAssertTrue(source.contains("Update conversation route policy"))
        XCTAssertTrue(source.contains("Confirm new Segment"))
        XCTAssertTrue(source.contains("Confirm route change"))
        XCTAssertTrue(source.contains("Create new Segment"))
        XCTAssertTrue(source.contains("Continue with context"))
        XCTAssertTrue(source.contains("Summary only"))
        XCTAssertTrue(source.contains("Start clean"))
        XCTAssertTrue(source.contains("Disclosure receipt"))
        XCTAssertTrue(source.contains("The route changed while you were reviewing"))
        XCTAssertTrue(source.contains("Credential r\\(revision)"))
        XCTAssertFalse(source.contains("set: store.selectConversationProfile"))
        XCTAssertFalse(source.contains("Context transfer mode"))
    }

    func testConversationRouteTransitionSheetRendersAcrossAppearances() throws {
        let source = try JSONDecoder().decode(
            LocalProductConversationProfile.self,
            from: Data(
                #"{"profile_id":"conversation-deepseek-deepseek-chat-account-work-r7","harness_adapter":"loom-native","provider_id":"deepseek","provider_account_id":"deepseek.work","display_name":"DeepSeek","protocol":"openai_compatible","model_id":"deepseek-chat","auth_mode":"brokered","credential_revision":7}"#.utf8
            )
        )
        let target = try JSONDecoder().decode(
            LocalProductConversationProfile.self,
            from: Data(
                #"{"profile_id":"conversation-openai-codex-default-v1","harness_adapter":"codex","provider_id":"openai","provider_account_id":"","display_name":"OpenAI","protocol":"codex_native","model_id":"gpt-5.5-codex","auth_mode":"native_auth","credential_revision":0}"#.utf8
            )
        )
        let transition = LocalProductConversationRouteTransition(
            id: "transition-1",
            source: source,
            target: target,
            threadID: "thread-1",
            sourceExecutionBinding: LocalProductConversationExecutionBinding(
                schemaVersion: 4,
                harnessAdapter: "loom-native",
                providerID: "deepseek",
                providerAccountID: "deepseek.work",
                credentialRevision: 7,
                modelID: "deepseek-chat",
                providerAccountPolicyVersion: 2,
                providerAccountPolicyRevision: 3,
                providerAccountPolicyDigest: String(repeating: "a", count: 64),
                trustDomain: "external_provider",
                retentionMode: "provider_default",
                dataRegion: "global"
            ),
            generation: 1
        )

        for (appearance, typeSize) in [
            (ColorScheme.light, DynamicTypeSize.large),
            (ColorScheme.dark, DynamicTypeSize.accessibility2),
        ] {
            var confirmedMode: LocalProductConversationContextMode?
            let hosting = NSHostingView(
                rootView: ConversationRouteTransitionSheet(
                    transition: transition,
                    onConfirm: { mode, _ in
                        confirmedMode = mode
                        return true
                    },
                    onCancel: {}
                )
                .environment(\.colorScheme, appearance)
                .environment(\.dynamicTypeSize, typeSize)
                .frame(width: 560, height: 520)
            )
            hosting.frame = NSRect(x: 0, y: 0, width: 560, height: 520)
            hosting.layoutSubtreeIfNeeded()

            XCTAssertGreaterThan(hosting.fittingSize.width, 480)
            XCTAssertGreaterThan(hosting.fittingSize.height, 460)
            XCTAssertNil(confirmedMode)
        }
    }

    func testConversationRouteTransitionTrustBoundaryAcknowledgementGate() throws {
        let source = try JSONDecoder().decode(
            LocalProductConversationProfile.self,
            from: Data(
                #"{"profile_id":"conversation-deepseek-work-r7","harness_adapter":"loom-native","provider_id":"deepseek","provider_account_id":"deepseek.work","display_name":"DeepSeek","protocol":"openai_compatible","model_id":"deepseek-chat","auth_mode":"brokered","credential_revision":7,"policy_version":2,"policy_revision":4,"policy_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","trust_domain":"enterprise_tenant","retention_mode":"zero_data_retention","data_region":"apac"}"#.utf8
            )
        )
        let target = try JSONDecoder().decode(
            LocalProductConversationProfile.self,
            from: Data(
                #"{"profile_id":"conversation-anthropic-work-r5","harness_adapter":"loom-native","provider_id":"anthropic","provider_account_id":"anthropic.work","display_name":"Anthropic","protocol":"anthropic_messages","model_id":"claude-sonnet-5","auth_mode":"brokered","credential_revision":5,"policy_version":2,"policy_revision":8,"policy_digest":"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee","trust_domain":"enterprise_tenant","retention_mode":"zero_data_retention","data_region":"apac"}"#.utf8
            )
        )
        let frozenBinding = LocalProductConversationExecutionBinding(
            schemaVersion: 4,
            harnessAdapter: "loom-native",
            providerID: "deepseek",
            providerAccountID: "deepseek.work",
            credentialRevision: 7,
            modelID: "deepseek-chat",
            providerAccountPolicyVersion: 2,
            providerAccountPolicyRevision: 3,
            providerAccountPolicyDigest: String(repeating: "a", count: 64),
            trustDomain: "external_provider",
            retentionMode: "provider_default",
            dataRegion: "global"
        )
        let changed = LocalProductConversationRouteTransition(
            source: source,
            target: target,
            threadID: "thread-1",
            sourceExecutionBinding: frozenBinding,
            generation: 1
        )
        let unchangedBinding = LocalProductConversationExecutionBinding(
            schemaVersion: 4,
            harnessAdapter: "loom-native",
            providerID: "deepseek",
            providerAccountID: "deepseek.work",
            credentialRevision: 7,
            modelID: "deepseek-chat",
            providerAccountPolicyVersion: 2,
            providerAccountPolicyRevision: 4,
            providerAccountPolicyDigest: String(repeating: "b", count: 64),
            trustDomain: "enterprise_tenant",
            retentionMode: "zero_data_retention",
            dataRegion: "apac"
        )
        let unchanged = LocalProductConversationRouteTransition(
            source: source,
            target: target,
            threadID: "thread-legacy",
            sourceExecutionBinding: unchangedBinding,
            generation: 2
        )
        let legacy = LocalProductConversationRouteTransition(
            source: source,
            target: target,
            threadID: "thread-missing-authority",
            generation: 3
        )

        XCTAssertEqual(
            changed.trustBoundaryChanges.map {
                conversationTrustBoundaryDimensionLabel($0.dimension)
            },
            ["Trust domain", "Retention mode", "Data region"]
        )
        XCTAssertFalse(conversationRouteTransitionConfirmationEnabled(
            changed,
            trustBoundaryAcknowledged: false
        ))
        XCTAssertTrue(conversationRouteTransitionConfirmationEnabled(
            changed,
            trustBoundaryAcknowledged: true
        ))
        XCTAssertTrue(unchanged.trustBoundaryChanges.isEmpty)
        XCTAssertTrue(conversationRouteTransitionConfirmationEnabled(
            unchanged,
            trustBoundaryAcknowledged: false
        ))
        XCTAssertEqual(legacy.trustBoundaryChanges.count, 3)
        XCTAssertFalse(conversationRouteTransitionConfirmationEnabled(
            legacy,
            trustBoundaryAcknowledged: false
        ))
        XCTAssertFalse(conversationRouteTransitionConfirmationEnabled(
            legacy,
            trustBoundaryAcknowledged: true
        ))
    }

    func testConversationRouteTransitionSheetNamesTrustBoundaryReview() throws {
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/LoomWorkspaceShell.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)

        XCTAssertTrue(source.contains("Trust-boundary changes"))
        XCTAssertTrue(source.contains("I acknowledge these trust-boundary changes"))
        XCTAssertTrue(source.contains("Frozen source authority is unavailable"))
        XCTAssertTrue(source.contains("Start new conversation"))
        XCTAssertTrue(source.contains("onStartNewConversation"))
        XCTAssertTrue(source.contains("store.newConversation()"))
        XCTAssertTrue(
            source.contains("store.selectConversationProfile(transition.target.profileID)")
        )
        XCTAssertFalse(source.contains("Reload this conversation before changing its route"))
        XCTAssertTrue(source.contains("conversationTrustBoundaryDimensionLabel(change.dimension)"))
        XCTAssertTrue(source.contains("trustBoundaryAcknowledged: trustBoundaryAcknowledged"))
        XCTAssertTrue(source.contains("Button(\"Cancel\", action: onCancel)"))
    }

    func testWorkspaceShellNamesEveryLiveJourneyAction() throws {
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/LoomWorkspaceShell.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)

        XCTAssertTrue(source.contains(".help(\"Try Again\")"))
        XCTAssertTrue(source.contains(
            "Label(\"Choose folder\", systemImage: \"folder\")"
        ))
        XCTAssertTrue(source.contains(
            "Label(\"Use Agent Team\", systemImage: \"person.3\")"
        ))
        XCTAssertTrue(source.contains(
            "Label(\"Start Mission\", systemImage: \"flag.checkered\")"
        ))
        XCTAssertTrue(source.contains(".help(\"Add folder, Agent Team, or Mission\")"))
        XCTAssertTrue(source.contains(".accessibilityLabel(actionLabel)"))
        XCTAssertTrue(source.contains(".help(actionLabel)"))
    }

    func testRecentTaskActionLabelSanitizesBoundsAndFallsBack() {
        let hostileTitle =
            "\u{001B}[31m" + String(repeating: "A", count: 80) + "\tunsafe"

        XCTAssertEqual(
            LoomRecentTaskActionLabel.make(
                for: hostileTitle,
                subtitle: "Succeeded",
                position: 2
            ),
            "Open recent task 2, " + String(repeating: "A", count: 48)
                + "…, Succeeded"
        )
        XCTAssertEqual(
            LoomRecentTaskActionLabel.make(
                for: "\u{0000}\n\t",
                subtitle: "\u{001B}\n",
                position: 0
            ),
            "Open recent task 1"
        )
        XCTAssertNotEqual(
            LoomRecentTaskActionLabel.make(
                for: "Recent work",
                subtitle: "Succeeded",
                position: 1
            ),
            LoomRecentTaskActionLabel.make(
                for: "Recent work",
                subtitle: "Succeeded",
                position: 2
            )
        )
    }

    func testMissionWorkbenchUsesNativeWindowAndProviderIsReachable()
        async throws
    {
        let snapshot = try LocalProductWire.decodeSnapshot(
            Data(MissionOrchestrationTests.snapshotJSON.utf8)
        )
        let store = LocalProductStore(
            client: MissionWorkbenchStubClient(snapshot: snapshot)
        )
        await store.refresh()
        await store.refreshSetup()

        let window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 1_100, height: 720),
            styleMask: [.titled, .closable, .resizable],
            backing: .buffered,
            defer: false
        )
        window.contentView = NSHostingView(
            rootView: ContentView(store: store, refreshOnAppear: false)
        )
        window.layoutIfNeeded()

        XCTAssertEqual(store.workbench.route, .board)
        XCTAssertTrue(store.providerManagementReachable)
        XCTAssertGreaterThanOrEqual(
            LoomGraphite.minimumActionTarget,
            44
        )
        XCTAssertNotNil(window.contentView)
        retainedMissionWorkbenchWindows.append(window)
    }

    func testMissionWorkbenchCanPresentProvidersAsInitialDestination()
        async throws
    {
        let snapshot = try LocalProductWire.decodeSnapshot(
            Data(MissionOrchestrationTests.snapshotJSON.utf8)
        )
        let store = LocalProductStore(
            client: MissionWorkbenchStubClient(snapshot: snapshot)
        )
        await store.refresh()
        await store.refreshSetup()

        let window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 1_100, height: 720),
            styleMask: [.titled, .closable, .resizable],
            backing: .buffered,
            defer: false
        )
        window.contentView = NSHostingView(
            rootView: MissionWorkbench(
                store: store,
                showProvidersInitially: true
            )
        )
        window.makeKeyAndOrderFront(nil)
        window.layoutIfNeeded()
        try await Task.sleep(for: .milliseconds(200))

        XCTAssertEqual(window.sheets.count, 1)
        retainedMissionWorkbenchWindows.append(window)
    }

    func testClaudeSignInRowRendersAtAccessibilitySizeAndCancellationIsWired()
        async throws
    {
        let snapshot = try LocalProductWire.decodeSnapshot(
            Data(MissionOrchestrationTests.snapshotJSON.utf8)
        )
        let client = MissionWorkbenchStubClient(
            snapshot: snapshot,
            includeClaudeRuntime: true
        )
        let store = LocalProductStore(client: client)
        await store.refresh()
        await store.refreshSetup()
        store.startClaudeCodeSignIn()
        for _ in 0..<100 where !store.providersInFlight.contains("claude-code") {
            await Task.yield()
        }
        XCTAssertTrue(store.providersInFlight.contains("claude-code"))
        XCTAssertNotNil(store.providerOperationIncidentID["claude-code"])

        let window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 1_100, height: 760),
            styleMask: [.titled, .closable, .resizable],
            backing: .buffered,
            defer: false
        )
        window.contentView = NSHostingView(
            rootView: MissionWorkbench(
                store: store,
                showProvidersInitially: true
            )
            .environment(\.dynamicTypeSize, .accessibility2)
        )
        window.makeKeyAndOrderFront(nil)
        window.layoutIfNeeded()
        try await Task.sleep(for: .milliseconds(200))

        let sheet = try XCTUnwrap(window.sheets.first)
        sheet.layoutIfNeeded()
        let content = try XCTUnwrap(sheet.contentView)
        content.layoutSubtreeIfNeeded()
        let bitmap = try XCTUnwrap(
            content.bitmapImageRepForCachingDisplay(in: content.bounds)
        )
        content.cacheDisplay(in: content.bounds, to: bitmap)
        let imageData = try XCTUnwrap(
            bitmap.representation(using: .png, properties: [:])
        )
        XCTAssertGreaterThanOrEqual(sheet.contentLayoutRect.width, 720)
        XCTAssertLessThanOrEqual(content.fittingSize.width, sheet.contentLayoutRect.width + 1)
        XCTAssertGreaterThan(imageData.count, 5_000)

        await store.cancelClaudeCodeSignIn()
        XCTAssertEqual(client.claudeCancelRequestCount, 1)
        XCTAssertFalse(store.providersInFlight.contains("claude-code"))

        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/MissionWorkbench.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)
        XCTAssertTrue(source.contains("isPresented: $showProviders"))
        XCTAssertTrue(source.contains("onDismiss:"))
        XCTAssertTrue(source.contains("await store.cancelClaudeCodeSignIn()"))

        window.endSheet(sheet)
        window.contentView = nil
    }

    func testNewMissionContextRendersInNativeSheetAcrossAppearances()
        async throws
    {
        let snapshot = try LocalProductWire.decodeSnapshot(
            Data(MissionOrchestrationTests.snapshotJSON.utf8)
        )
        let fixtures: [(ColorScheme, DynamicTypeSize)] = [
            (.light, .large),
            (.dark, .accessibility2),
        ]

        for (appearance, typeSize) in fixtures {
            let store = LocalProductStore(
                client: MissionWorkbenchStubClient(snapshot: snapshot)
            )
            await store.refresh()
            let window = NSWindow(
                contentRect: NSRect(x: 0, y: 0, width: 920, height: 760),
                styleMask: [.titled, .closable, .resizable],
                backing: .buffered,
                defer: false
            )
            window.contentView = NSHostingView(
                rootView: MissionWorkbench(
                    store: store,
                    showNewMissionInitially: true
                )
                .environment(\.colorScheme, appearance)
                .environment(\.dynamicTypeSize, typeSize)
            )
            window.makeKeyAndOrderFront(nil)
            window.layoutIfNeeded()
            try await Task.sleep(for: .milliseconds(200))

            let sheet = try XCTUnwrap(window.sheets.first)
            sheet.layoutIfNeeded()
            XCTAssertGreaterThanOrEqual(sheet.frame.width, 660)
            XCTAssertGreaterThanOrEqual(sheet.frame.height, 640)
            XCTAssertNotNil(sheet.contentView)
            XCTAssertGreaterThan(sheet.contentLayoutRect.width, 620)
            XCTAssertGreaterThan(sheet.contentLayoutRect.height, 600)
            retainedMissionWorkbenchWindows.append(window)
        }
    }

    func testAllDecisionTemplatesUseOneNativeSheetHost() throws {
        let fixtures: [(String, [String])] = [
            (
                "authorization",
                ["not_now", "deny", "edit_scope", "allow_once",
                 "allow_for_mission"]
            ),
            ("review", ["not_now", "request_changes", "accept_result"]),
            (
                "recovery",
                ["not_now", "stop_mission", "edit_scope",
                 "start_new_attempt"]
            ),
        ]
        for (kind, actions) in fixtures {
            let sheet = try LocalProductDecisionWire.decodeSheet(
                Data(decisionSheetJSON(kind: kind, actions: actions).utf8)
            )
            switch kind {
            case "authorization":
                XCTAssertEqual(
                    sheet.summary,
                    "Run the verified local test command."
                )
            case "review":
                XCTAssertEqual(sheet.networkAccess, "Independent Reviewer: PASS")
            case "recovery":
                XCTAssertEqual(
                    sheet.attemptScope,
                    "Attempt 2 · fresh generation"
                )
                XCTAssertEqual(sheet.target, "No Team or Provider change")
            default:
                XCTFail("unexpected decision kind \(kind)")
            }
            let window = NSWindow(
                contentRect: NSRect(x: 0, y: 0, width: 680, height: 560),
                styleMask: [.titled, .closable],
                backing: .buffered,
                defer: false
            )
            var openedEvidence = false
            let decisionView = LoomDecisionSheet(
                    sheet: sheet,
                    onAction: { _ in },
                    onOpenEvidence: { openedEvidence = true },
                    onClose: {}
                )
            if kind == "review" {
                decisionView.onOpenEvidence()
                XCTAssertTrue(openedEvidence)
            }
            window.contentView = NSHostingView(
                rootView: decisionView
                .environment(\.colorScheme, kind == "review" ? .dark : .light)
                .frame(width: 680, height: 560)
            )
            window.layoutIfNeeded()
            XCTAssertNotNil(window.contentView, "\(kind) sheet did not render")
            XCTAssertGreaterThanOrEqual(window.frame.width, 620)
            XCTAssertGreaterThan(
                window.contentView?.fittingSize.width ?? 0,
                320
            )
            try exportDecisionSheetIfRequested(window, kind: kind)
            retainedMissionWorkbenchWindows.append(window)
        }
    }

    private func exportDecisionSheetIfRequested(
        _ window: NSWindow,
        kind: String
    ) throws {
        guard let requestedDirectory =
            ProcessInfo.processInfo.environment[
                "LOOM_DECISION_SCREENSHOT_DIR"
            ] else {
            return
        }
        let current = URL(
            fileURLWithPath: FileManager.default.currentDirectoryPath
        )
        let expected = current.appendingPathComponent(
            "../../.loom-evidence/phase2a/P2A-W2"
        ).standardizedFileURL
        let requested = URL(fileURLWithPath: requestedDirectory)
            .standardizedFileURL
        guard requested == expected,
              requested == requested.resolvingSymlinksInPath()
                .standardizedFileURL else {
            throw LocalProductClientError.invalidRequest
        }
        guard let view = window.contentView else {
            throw LocalProductClientError.invalidResponse
        }
        view.frame = NSRect(x: 0, y: 0, width: 680, height: 560)
        view.layoutSubtreeIfNeeded()
        guard let bitmap = view.bitmapImageRepForCachingDisplay(
            in: view.bounds
        ) else {
            throw LocalProductClientError.invalidResponse
        }
        view.cacheDisplay(in: view.bounds, to: bitmap)
        guard let png = bitmap.representation(using: .png, properties: [:]),
              png.count > 20_000 else {
            throw LocalProductClientError.invalidResponse
        }
        try png.write(
            to: requested.appendingPathComponent(
                "decision-\(kind).png"
            ),
            options: .atomic
        )
    }

    private func decisionSheetJSON(
        kind: String,
        actions: [String]
    ) -> String {
        let title: String
        let summary: String
        let target: String
        let commandType: String
        let networkAccess: String
        let permissionScope: String
        let attemptScope: String
        let expectedEvidence: String
        switch kind {
        case "authorization":
            title = "Authorization required"
            summary = "Run the verified local test command."
            target = "Controlled local workspace"
            commandType = "Execute bounded local test"
            networkAccess = "none"
            permissionScope = "Repository read/write · this Mission"
            attemptScope = "Attempt 1"
            expectedEvidence = "Authoritative approval fact"
        case "review":
            title = "Review result"
            summary = "Validated API contract changes."
            target = "Exact prepared node result"
            commandType = "Focused and repository checks: PASS"
            networkAccess = "Independent Reviewer: PASS"
            permissionScope = "No new permission"
            attemptScope = "Attempt 1"
            expectedEvidence = "Evidence receipt accepted for Attempt 1"
        case "recovery":
            title = "Recovery decision"
            summary =
                "Attempt 1 stopped after deterministic verification failed."
            target = "No Team or Provider change"
            commandType = "Fresh attempt with generation fencing"
            networkAccess = "none"
            permissionScope = "No permission or budget increase"
            attemptScope = "Attempt 2 · fresh generation"
            expectedEvidence =
                "Attempt 1 output and Evidence retained by digest"
        default:
            preconditionFailure("unsupported decision kind")
        }
        let encodedActions = actions
            .map { "\"\($0)\"" }
            .joined(separator: ",")
        let encodedPreparedActions = actions
            .filter { !["not_now", "edit_scope"].contains($0) }
            .map { "\"\($0)\"" }
            .joined(separator: ",")
        return """
        {
          "schema_version":1,
          "kind":"\(kind)",
          "mission_id":"mission/team-1",
          "team_instance_id":"team-1",
          "view_version":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
          "decision_id":"decision-1",
          "decision_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
          "title":"\(title)",
          "summary":"\(summary)",
          "requester":"Release Team",
          "target":"\(target)",
          "command_type":"\(commandType)",
          "network_access":"\(networkAccess)",
          "credential_access":"none",
          "permission_scope":"\(permissionScope)",
          "attempt_scope":"\(attemptScope)",
          "expected_evidence":"\(expectedEvidence)",
          "technical_details":["Prepared by owning authority"],
          "actions":[\(encodedActions)],
          "prepared_actions":[\(encodedPreparedActions)],
          "prepared":true,
          "logical_node_id":"main",
          "attempt_number":1,
          "claim_generation":1
        }
        """
    }
}

private final class MissionWorkbenchStubClient:
    LocalProductClientProtocol,
    LocalProductSetupClientProtocol,
    LocalProductExecutionClientProtocol
{
    let snapshotValue: LocalProductSnapshot
    let includeClaudeRuntime: Bool
    private(set) var claudeCancelRequestCount = 0

    init(
        snapshot: LocalProductSnapshot,
        includeClaudeRuntime: Bool = false
    ) {
        snapshotValue = snapshot
        self.includeClaudeRuntime = includeClaudeRuntime
    }

    func snapshot(limit: Int) async throws -> LocalProductSnapshot {
        snapshotValue
    }

    func timeline(
        teamInstanceID: String,
        cursor: String,
        limit: Int
    ) async throws -> LocalProductTimelinePage {
        throw LocalProductClientError.notFound
    }

    func executeMission(
        _ command: LocalProductExecutionCommand
    ) async throws -> LocalProductExecutionEnvelope {
        throw LocalProductClientError.unavailable
    }

    func setupSnapshot() async throws -> LocalProductSetupSnapshot {
        let runtimes = includeClaudeRuntime
            ? """
              [{
                "runtime_instance_id":"claude-code-system",
                "display_name":"Claude Code",
                "adapter_type":"claude-code",
                "executable_version":"2.1.196",
                "status":"online",
                "capacity":3,
                "model_ids":["claude-sonnet-4-6"],
                "observed_capabilities":["conversation"],
                "source_probe_id":"claude-code-system"
              }]
              """
            : "[]"
        return try LocalProductSetupWire.decodeSnapshot(
            Data(
                """
                {
                  "schema_version":1,
                  "view_version":"view-setup",
                  "codex":{"provider_id":"codex","auth_mode":"native_auth","credential_reference":"","revision":0,"status":"available","reason":""},
                  "minimax":{"provider_id":"minimax","auth_mode":"brokered","credential_reference":"configured","revision":1,"status":"verified","reason":""},
                  "runtimes":\(runtimes),
                  "saved_teams":[],
                  "templates":[],
                  "role_options":[],
                  "skills":[],
                  "permissions":[],
                  "resources":[]
                }
                """.utf8
            )
        )
    }

    func connectCodex() async throws -> LocalProductProviderConnectResult {
        throw LocalProductClientError.unavailable
    }

    func connectClaudeCode(
        incidentID: String
    ) async throws -> LocalProductProviderConnectResult {
        guard includeClaudeRuntime else {
            throw LocalProductClientError.unavailable
        }
        return try LocalProductSetupWire.decodeProviderConnectResult(
            Data(
                #"{"provider_id":"claude-code","auth_mode":"native_auth","status":"started"}"#.utf8
            )
        )
    }

    func cancelClaudeCode(
        incidentID: String
    ) async throws -> LocalProductProviderConnectResult {
        guard includeClaudeRuntime else {
            throw LocalProductClientError.unavailable
        }
        claudeCancelRequestCount += 1
        return try LocalProductSetupWire.decodeProviderConnectResult(
            Data(
                #"{"provider_id":"claude-code","auth_mode":"native_auth","status":"cancelled"}"#.utf8
            )
        )
    }

    func startBuilder(
        source: String,
        sourceID: String,
        sourceVersion: Int,
        sourceDigest: String
    ) async throws -> LocalProductBuilderSession {
        throw LocalProductClientError.unavailable
    }

    func answerBuilder(
        session: LocalProductBuilderSession,
        answer: String
    ) async throws -> LocalProductBuilderSession {
        throw LocalProductClientError.unavailable
    }

    func editBuilder(
        session: LocalProductBuilderSession,
        field: String,
        value: String
    ) async throws -> LocalProductBuilderSession {
        throw LocalProductClientError.unavailable
    }

    func validateBuilder(
        session: LocalProductBuilderSession
    ) async throws -> LocalProductBuilderSession {
        throw LocalProductClientError.unavailable
    }

    func confirmBuilder(
        session: LocalProductBuilderSession,
        definitionID: String
    ) async throws -> LocalProductBuilderConfirmation {
        throw LocalProductClientError.unavailable
    }

    func archiveTeam(
        definitionID: String,
        expectedHead: Int64
    ) async throws -> LocalProductSetupSavedTeam {
        throw LocalProductClientError.unavailable
    }

    func restoreTeam(
        definitionID: String,
        expectedHead: Int64
    ) async throws -> LocalProductSetupSavedTeam {
        throw LocalProductClientError.unavailable
    }

    func configureMiniMax(
        secret: String
    ) async throws -> LocalProductCredentialSetupResult {
        throw LocalProductClientError.unavailable
    }

    func verifyMiniMax(
        reference: String,
        revision: Int64
    ) async throws -> LocalProductCredentialSetupResult {
        throw LocalProductClientError.unavailable
    }

    func replaceMiniMax(
        reference: String,
        revision: Int64,
        secret: String
    ) async throws -> LocalProductCredentialSetupResult {
        throw LocalProductClientError.unavailable
    }

    func revokeMiniMax(
        reference: String,
        revision: Int64
    ) async throws -> LocalProductCredentialSetupResult {
        throw LocalProductClientError.unavailable
    }
}

    func testVaultEncryptFailureAlwaysOffersVaultRecovery() {
        // A locked vault surfaces as vault_encrypt / state_unavailable with
        // retryable=true; the recovery action must still appear so the user
        // can unlock and continue.
        XCTAssertTrue(
            conversationVaultRecoveryAvailable(
                stage: .vaultEncrypt,
                recoverable: true
            )
        )
        XCTAssertTrue(
            conversationVaultRecoveryAvailable(
                stage: .vaultKeyLoad,
                recoverable: false
            )
        )
        XCTAssertFalse(
            conversationVaultRecoveryAvailable(
                stage: .providerConnect,
                recoverable: true
            )
        )
    }
