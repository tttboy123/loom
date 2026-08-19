import AppKit
import SwiftUI
import XCTest
@testable import LoomLocalAppCore
@testable import LoomLocalAppUI

@MainActor
private var retainedMissionWorkbenchWindows: [NSWindow] = []

@MainActor
final class LoomGraphiteViewTests: XCTestCase {
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

    func testConversationProfileMenuLabelIncludesExactProviderAccount() throws {
        let profile = try JSONDecoder().decode(
            LocalProductConversationProfile.self,
            from: Data(
                #"{"profile_id":"conversation-deepseek-deepseek-chat-account-work-r7","harness_adapter":"loom-native","provider_id":"deepseek","provider_account_id":"deepseek.work","display_name":"DeepSeek","protocol":"openai_compatible","model_id":"deepseek-chat","auth_mode":"brokered","credential_revision":7}"#.utf8
            )
        )
        // The Provider layer must not carry a model (three-layer separation).
        XCTAssertEqual(
            conversationProfileMenuLabel(profile),
            "DeepSeek · deepseek.work"
        )
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

    func testEmptyConversationOffersStartMissionWhenTeamReady() throws {
    let sourceURL = URL(fileURLWithPath: #filePath)
      .deletingLastPathComponent()
      .deletingLastPathComponent()
      .deletingLastPathComponent()
      .appendingPathComponent("Sources/LoomLocalAppUI/LoomWorkspaceShell.swift")
    let source = try String(contentsOf: sourceURL, encoding: .utf8)

    // With a confirmed Team the empty conversation must lead to a Mission,
    // not to another Team builder (a dead end for the user who already has one).
    XCTAssertTrue(source.contains("teamReady ? \"Start Mission\" : \"Use Agent Team\""))
    XCTAssertTrue(source.contains("fullGovernancePresentation = .newMission"))
    XCTAssertTrue(source.contains("startBlankBuilder()"))
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

    func testNewMissionSheetAutoSelectsFirstExecutableTeamAndExplainsDisabledReview() throws {
    let sourceURL = URL(fileURLWithPath: #filePath)
      .deletingLastPathComponent()
      .deletingLastPathComponent()
      .deletingLastPathComponent()
      .appendingPathComponent("Sources/LoomLocalAppUI/MissionWorkbench.swift")
    let source = try String(contentsOf: sourceURL, encoding: .utf8)

    // Every entry path into the New Mission sheet must pre-select the first
    // executable Team; otherwise Review preflight is silently disabled.
    XCTAssertTrue(
      source.contains("if newMissionTeamID.isEmpty {")
    )
    XCTAssertTrue(
      source.contains("newMissionTeamID = executableTeams.first?.teamInstanceID ?? \"\"")
    )
    // The disabled Review-preflight action must explain what is missing.
    XCTAssertTrue(source.contains("missionReviewBlockedReason"))
    XCTAssertTrue(source.contains("Enter a Mission objective first."))
    XCTAssertTrue(source.contains("Select a Team to run this Mission."))
    XCTAssertTrue(source.contains("Confirm the Mission context to enable preflight review."))
  }

  func testWorkspaceShellOffersNewMissionFromRailAndComposer() throws {
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/LoomWorkspaceShell.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)

        XCTAssertGreaterThanOrEqual(
            source.components(separatedBy: "fullGovernancePresentation = .newMission").count - 1,
            2
        )
        XCTAssertTrue(source.contains("showNewMissionInitially: presentation == .newMission"))
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
        XCTAssertTrue(source.contains("Credential r\\(profile.credentialRevision)"))
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
                #"{"profile_id":"conversation-openai-codex-default-v1","harness_adapter":"codex","provider_id":"openai","provider_account_id":"","display_name":"Codex","protocol":"codex_native","model_id":"gpt-5.5-codex","auth_mode":"native_auth","credential_revision":0}"#.utf8
            )
        )
        let transition = LocalProductConversationRouteTransition(
            id: "transition-1",
            source: source,
            target: target,
            threadID: "thread-1",
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
                    onConfirm: { mode in
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

    func testWorkspaceShellNamesEveryLiveJourneyAction() throws {
        let sourceURL = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Sources/LoomLocalAppUI/LoomWorkspaceShell.swift")
        let source = try String(contentsOf: sourceURL, encoding: .utf8)

        XCTAssertTrue(source.contains(".help(\"Try Again\")"))
        XCTAssertGreaterThanOrEqual(
            source.components(separatedBy: ".help(\"Open Folder\")").count - 1,
            1
        )
        XCTAssertGreaterThanOrEqual(
            source.components(separatedBy: ".help(\"Use Agent Team\")").count - 1,
            1
        )
        XCTAssertTrue(
            source.contains("\"Start a governed Mission with your Team\"")
        )
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

    init(snapshot: LocalProductSnapshot) {
        snapshotValue = snapshot
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
        try LocalProductSetupWire.decodeSnapshot(
            Data(
                """
                {
                  "schema_version":1,
                  "view_version":"view-setup",
                  "codex":{"provider_id":"codex","auth_mode":"native_auth","credential_reference":"","revision":0,"status":"available","reason":""},
                  "minimax":{"provider_id":"minimax","auth_mode":"brokered","credential_reference":"configured","revision":1,"status":"verified","reason":""},
                  "runtimes":[],
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
