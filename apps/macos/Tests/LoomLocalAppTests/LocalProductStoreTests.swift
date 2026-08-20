import Foundation
import XCTest

@testable import LoomLocalAppCore

@MainActor
final class LocalProductStoreTests: XCTestCase {
  func testCredentialVaultLockAndUnlockRefreshAuthoritativeSetup() async throws {
    let client = try ProviderSetupStubClient()
    let store = LocalProductStore(client: client)

    await store.setCredentialVaultLocked(true)

    XCTAssertEqual(client.vaultLockRequestCount, 1)
    XCTAssertEqual(client.setupRequestCount, 1)
    XCTAssertEqual(store.credentialVaultOperationDetail, "Vault locked")
    XCTAssertFalse(store.credentialVaultOperationFailed)

    await store.setCredentialVaultLocked(false)

    XCTAssertEqual(client.vaultUnlockRequestCount, 1)
    XCTAssertEqual(client.setupRequestCount, 2)
    XCTAssertEqual(store.credentialVaultOperationDetail, "Vault unlocked")
    XCTAssertFalse(store.isUpdatingCredentialVaultLock)
  }

  func testCredentialVaultUnlockShowsStageAndIncidentOnFailure() async throws {
    let client = try ProviderSetupStubClient(
      vaultUnlockError: LocalIPCRemoteError(
        code: .credentialUnavailable,
        recoverable: true,
        stage: .vaultOpen,
        incidentID: "vault-unlock-incident-1"
      )
    )
    let store = LocalProductStore(client: client)

    await store.setCredentialVaultLocked(false)

    XCTAssertTrue(store.credentialVaultOperationFailed)
    XCTAssertEqual(
      store.credentialVaultOperationDetail,
      "Vault Open failed · Incident vault-unlock-incident-1"
    )
    XCTAssertEqual(client.setupRequestCount, 0)
    XCTAssertFalse(store.isUpdatingCredentialVaultLock)
  }

  func testCredentialVaultRotationRefreshesAuthoritativeSetup() async throws {
    let client = try ProviderSetupStubClient()
    let store = LocalProductStore(client: client)

    await store.rotateCredentialVault()

    XCTAssertEqual(client.vaultRotationRequestCount, 1)
    XCTAssertEqual(client.setupRequestCount, 1)
    XCTAssertEqual(store.credentialVaultOperationDetail, "Vault key rotated")
    XCTAssertFalse(store.credentialVaultOperationFailed)
    XCTAssertFalse(store.isRotatingCredentialVault)
  }

  func testCredentialVaultRotationShowsStageAndIncidentOnFailure() async throws {
    let client = try ProviderSetupStubClient(
      vaultRotationError: LocalIPCRemoteError(
        code: .credentialUnavailable,
        recoverable: true,
        stage: .vaultRotation,
        incidentID: "vault-incident-1"
      )
    )
    let store = LocalProductStore(client: client)

    await store.rotateCredentialVault()

    XCTAssertTrue(store.credentialVaultOperationFailed)
    XCTAssertEqual(
      store.credentialVaultOperationDetail,
      "Vault Rotation failed · Incident vault-incident-1"
    )
    XCTAssertFalse(store.isRotatingCredentialVault)
  }

  func testCredentialVaultRecoveryResetRefreshesSetupAndRequiresReentry() async throws {
    let client = try ProviderSetupStubClient()
    let store = LocalProductStore(client: client)

    await store.resetCredentialVault()

    XCTAssertEqual(client.vaultResetRequestCount, 1)
    XCTAssertEqual(client.setupRequestCount, 1)
    XCTAssertEqual(
      store.credentialVaultOperationDetail,
      "Vault reset complete · Re-enter each Provider Account key"
    )
    XCTAssertFalse(store.credentialVaultOperationFailed)
    XCTAssertFalse(store.isResettingCredentialVault)
  }

  func testCredentialVaultRecoveryResetShowsStageAndIncidentOnFailure() async throws {
    let client = try ProviderSetupStubClient(
      vaultResetError: LocalIPCRemoteError(
        code: .credentialUnavailable,
        recoverable: false,
        stage: .vaultRecovery,
        incidentID: "vault-reset-incident-1"
      )
    )
    let store = LocalProductStore(client: client)

    await store.resetCredentialVault()

    XCTAssertTrue(store.credentialVaultOperationFailed)
    XCTAssertEqual(
      store.credentialVaultOperationDetail,
      "Vault Recovery failed · Incident vault-reset-incident-1"
    )
    XCTAssertEqual(client.setupRequestCount, 0)
    XCTAssertFalse(store.isResettingCredentialVault)
  }

  func testCredentialVaultEncryptedExportReportsBoundedResult() async throws {
    let client = try ProviderSetupStubClient()
    let store = LocalProductStore(client: client)

    let succeeded = await store.exportCredentialVault(
      passphrase: "store backup passphrase",
      destination: "/tmp/Loom-Credential-Vault.loomvault"
    )

    XCTAssertTrue(succeeded)
    XCTAssertEqual(client.vaultExportRequestCount, 1)
    XCTAssertEqual(
      client.vaultExportDestinations,
      ["/tmp/Loom-Credential-Vault.loomvault"]
    )
    XCTAssertEqual(
      store.credentialVaultOperationDetail,
      "Encrypted backup saved · 2 credentials"
    )
    XCTAssertFalse(store.isExportingCredentialVault)
  }

  func testCredentialVaultEncryptedExportShowsStageAndIncidentOnFailure() async throws {
    let client = try ProviderSetupStubClient(
      vaultExportError: LocalIPCRemoteError(
        code: .credentialUnavailable,
        recoverable: true,
        stage: .vaultExport,
        incidentID: "vault-export-incident-1"
      )
    )
    let store = LocalProductStore(client: client)

    let succeeded = await store.exportCredentialVault(
      passphrase: "store backup passphrase",
      destination: "/tmp/Loom-Credential-Vault.loomvault"
    )

    XCTAssertFalse(succeeded)
    XCTAssertTrue(store.credentialVaultOperationFailed)
    XCTAssertEqual(
      store.credentialVaultOperationDetail,
      "Vault Export failed · Incident vault-export-incident-1"
    )
    XCTAssertFalse(store.isExportingCredentialVault)
  }

  func testDeepSeekConnectUsesGenericCredentialPathAndShowsSpecificFailure() async throws {
    let client = try ProviderSetupStubClient(
      deepSeekEnabled: true,
      deepSeekConfigureError: LocalIPCRemoteError(
        code: .credentialUnavailable,
        recoverable: false
      )
    )
    let store = LocalProductStore(client: client)
    await store.refreshSetup()

    await store.connectProvider(providerID: "deepseek", secret: "sk-test")

    XCTAssertEqual(client.genericConfigureProviderIDs, ["deepseek"])
    XCTAssertEqual(store.providerOperationStatus["deepseek"], "Credential Vault unavailable")
    XCTAssertEqual(
      store.providerOperationDetail["deepseek"],
      "Loom could not safely complete this Credential Vault operation. Review diagnostics and try again."
    )
  }

  func testDeepSeekCredentialHelperAuthorizationFailureIsActionable() async throws {
    let client = try ProviderSetupStubClient(
      deepSeekEnabled: true,
      deepSeekConfigureError: LocalIPCRemoteError(
        code: .credentialUnavailable,
        recoverable: true,
        stage: .helperAuthorization,
        incidentID: "loom-swift-11111111-1111-4111-8111-111111111111"
      )
    )
    let store = LocalProductStore(client: client)
    await store.refreshSetup()

    await store.connectProvider(providerID: "deepseek", secret: "sk-test")

    XCTAssertEqual(store.providerOperationStatus["deepseek"], "Credential Vault unavailable")
    XCTAssertEqual(
      store.providerOperationDetail["deepseek"],
      "macOS could not verify Loom's bundled credential helper. Reinstall or reopen Loom, then try again."
    )
    XCTAssertEqual(
      store.providerOperationIncidentID["deepseek"],
      "loom-swift-11111111-1111-4111-8111-111111111111"
    )
    XCTAssertEqual(
      store.providerOperationStage["deepseek"],
      "Credential helper authorization"
    )
    XCTAssertEqual(store.providerOperationRetryable["deepseek"], true)
  }

  func testDeepSeekGenericConnectConfiguresThenVerifies() async throws {
    let client = try ProviderSetupStubClient(deepSeekEnabled: true)
    let store = LocalProductStore(client: client)
    await store.refreshSetup()

    await store.connectProvider(providerID: "deepseek", secret: "sk-test")

    XCTAssertEqual(client.genericConfigureProviderIDs, ["deepseek"])
    XCTAssertEqual(client.genericVerifyProviderIDs, ["deepseek"])
    XCTAssertEqual(store.providerOperationStatus["deepseek"], "Verified")
    XCTAssertEqual(
      store.setupSnapshot?.providers.first(where: {
        $0.providerID == "deepseek"
      })?.revision,
      2
    )
  }

  func testDeepSeekMigrationReentryImportsIntoVaultThenVerifies() async throws {
    let client = try ProviderSetupStubClient(
      deepSeekEnabled: true,
      deepSeekMigrationRequired: true
    )
    let store = LocalProductStore(client: client)
    await store.refreshSetup()

    XCTAssertEqual(
      store.setupSnapshot?.providerAccounts.first?.status,
      "migration_required"
    )
    await store.replaceProvider(providerID: "deepseek", secret: "sk-test")

    XCTAssertEqual(client.genericReplaceProviderIDs, ["deepseek"])
    XCTAssertEqual(client.genericVerifyProviderIDs, ["deepseek"])
    XCTAssertEqual(
      store.setupSnapshot?.providerAccounts.first?.status,
      "verified"
    )
    XCTAssertEqual(store.providerOperationStatus["deepseek"], "Verified")
  }

  func testIndependentProviderAccountConnectUsesAccountScopedPath() async throws {
    let client = try ProviderSetupStubClient(deepSeekEnabled: true)
    let store = LocalProductStore(client: client)
    await store.refreshSetup()

    await store.connectProvider(
      providerID: "deepseek",
      providerAccountID: "deepseek.work",
      secret: "sk-test"
    )

    XCTAssertEqual(client.genericConfigureAccountIDs, ["deepseek.work"])
    XCTAssertEqual(client.genericVerifyAccountIDs, ["deepseek.work"])
    XCTAssertEqual(store.providerOperationStatus["deepseek.work"], "Verified")
    XCTAssertEqual(
      store.setupSnapshot?.providerAccounts.first(where: {
        $0.providerAccountID == "deepseek.work"
      })?.revision,
      2
    )
    XCTAssertEqual(
      store.setupSnapshot?.providers.first?.revision,
      0,
      "a non-primary account must not replace the Provider primary projection"
    )
  }

  func testProviderAccountPolicyRefreshesExactAuthoritativeAccount() async throws {
    let client = try ProviderSetupStubClient(deepSeekEnabled: true)
    let store = LocalProductStore(client: client)
    await store.refreshSetup()
    await store.connectProvider(
      providerID: "deepseek",
      providerAccountID: "deepseek.work",
      secret: "sk-test"
    )

    let succeeded = await store.configureProviderAccountPolicy(
      providerID: "deepseek",
      providerAccountID: "deepseek.work",
      expectedRevision: 0,
      maximumConcurrentAttempts: 4,
      dispatchWindowSeconds: 60,
      maximumDispatchStarts: 20,
      maximumAssignedBudgetUnits: 12_000,
      trustDomain: "external_provider",
      retentionMode: "zero_data_retention",
      dataRegion: "apac"
    )

    XCTAssertTrue(succeeded)
    XCTAssertEqual(client.providerPolicyRequests.count, 1)
    XCTAssertEqual(client.providerPolicyRequests.first?.providerAccountID, "deepseek.work")
    XCTAssertEqual(client.providerPolicyRequests.first?.expectedRevision, 0)
    let account = store.setupSnapshot?.providerAccounts.first(where: {
      $0.providerID == "deepseek" && $0.providerAccountID == "deepseek.work"
    })
    XCTAssertEqual(account?.policyAvailable, true)
    XCTAssertEqual(account?.policyRevision, 1)
    XCTAssertEqual(account?.maximumConcurrentAttempts, 4)
    XCTAssertEqual(account?.policyVersion, 2)
    XCTAssertEqual(account?.retentionMode, "zero_data_retention")
    XCTAssertEqual(store.providerPolicyOperationDetail["deepseek.work"], "Limits saved")
    XCTAssertFalse(store.providerPolicyAccountsInFlight.contains("deepseek.work"))
  }

  func testProviderAccountPolicyConflictDoesNotTakeSetupOffline() async throws {
    let client = try ProviderSetupStubClient(
      deepSeekEnabled: true,
      providerPolicyError: LocalIPCRemoteError(
        code: .conflict,
        recoverable: true,
        incidentID: "policy-incident-1"
      )
    )
    let store = LocalProductStore(client: client)
    await store.refreshSetup()
    await store.connectProvider(
      providerID: "deepseek",
      providerAccountID: "deepseek.work",
      secret: "sk-test"
    )

    let succeeded = await store.configureProviderAccountPolicy(
      providerID: "deepseek",
      providerAccountID: "deepseek.work",
      expectedRevision: 0,
      maximumConcurrentAttempts: 4,
      dispatchWindowSeconds: 60,
      maximumDispatchStarts: 20,
      maximumAssignedBudgetUnits: 12_000,
      trustDomain: "external_provider",
      retentionMode: "provider_default",
      dataRegion: "global"
    )

    XCTAssertFalse(succeeded)
    XCTAssertEqual(store.setupState, .ready)
    XCTAssertEqual(
      store.providerPolicyOperationDetail["deepseek.work"],
      "Limits changed elsewhere. Reload this account and try again · Incident policy-incident-1"
    )
  }

  func testProviderModelRateCardRefreshesExactAuthoritativeModel() async throws {
    let client = try ProviderSetupStubClient(deepSeekEnabled: true)
    let store = LocalProductStore(client: client)
    await store.refreshSetup()
    await store.connectProvider(
      providerID: "deepseek", providerAccountID: "deepseek.work", secret: "sk-test"
    )

    let succeeded = await store.configureProviderModelRateCard(
      providerID: "deepseek", providerAccountID: "deepseek.work",
      modelID: "deepseek-chat", expectedRevision: 0, currency: "USD",
      inputTokenBasis: "input_includes_cache",
      inputMicrounitsPerMillion: 270_000,
      outputMicrounitsPerMillion: 1_100_000,
      cacheReadMicrounitsPerMillion: 70_000,
      cacheWriteMicrounitsPerMillion: 0
    )

    XCTAssertTrue(succeeded)
    XCTAssertEqual(client.providerRateCardRequests.count, 1)
    let account = store.setupSnapshot?.providerAccounts.first {
      $0.providerAccountID == "deepseek.work"
    }
    XCTAssertEqual(account?.rateCards.first?.modelID, "deepseek-chat")
    XCTAssertEqual(account?.rateCards.first?.revision, 1)
    XCTAssertEqual(account?.rateCards.first?.currency, "USD")
    XCTAssertEqual(
      store.providerRateCardOperationDetail["deepseek.work\u{1f}deepseek-chat"],
      "Rate card saved"
    )
  }

  func testProviderModelRateCardConflictDoesNotTakeSetupOffline() async throws {
    let client = try ProviderSetupStubClient(
      deepSeekEnabled: true,
      providerRateCardError: LocalIPCRemoteError(
        code: .conflict, recoverable: true,
        incidentID: "rate-card-incident-1"
      )
    )
    let store = LocalProductStore(client: client)
    await store.refreshSetup()
    await store.connectProvider(
      providerID: "deepseek", providerAccountID: "deepseek.work", secret: "sk-test"
    )

    let succeeded = await store.configureProviderModelRateCard(
      providerID: "deepseek", providerAccountID: "deepseek.work",
      modelID: "deepseek-chat", expectedRevision: 0, currency: "USD",
      inputTokenBasis: "input_excludes_cache",
      inputMicrounitsPerMillion: 270_000,
      outputMicrounitsPerMillion: 1_100_000,
      cacheReadMicrounitsPerMillion: 70_000,
      cacheWriteMicrounitsPerMillion: 0
    )

    XCTAssertFalse(succeeded)
    XCTAssertEqual(store.setupState, .ready)
    XCTAssertEqual(
      store.providerRateCardOperationDetail["deepseek.work\u{1f}deepseek-chat"],
      "This rate card changed elsewhere. Reload and try again · Incident rate-card-incident-1"
    )
  }

  func testRemoteToolEnrollmentRefreshesAndRevokesExactAccountRecord() async throws {
    let client = try ProviderSetupStubClient(
      deepSeekEnabled: true,
      remoteToolEnrollmentConfigured: true
    )
    let store = LocalProductStore(client: client)
    await store.refreshSetup()
    await store.connectProvider(
      providerID: "deepseek", providerAccountID: "deepseek.work", secret: "sk-test"
    )

    let configured = await store.configureRemoteToolBackend(
      LocalProductRemoteToolBackendEnrollmentCommand.fixture(expectedRevision: 1)
    )
    XCTAssertTrue(configured)
    XCTAssertEqual(client.remoteToolConfigureRequests.count, 1)
    XCTAssertEqual(
      store.setupSnapshot?.providerAccounts.first?.remoteToolBackends.first?.status,
      "active"
    )
    XCTAssertEqual(
      store.setupSnapshot?.providerAccounts.first?.remoteToolBackends.first?.revision,
      2
    )
    XCTAssertEqual(
      store.remoteToolEnrollmentOperationDetail["mcp-deepseek-work"],
      "Remote tool saved"
    )

    let revoked = await store.revokeRemoteToolBackend(
      LocalProductRemoteToolBackendEnrollmentRevokeCommand(
        enrollmentID: "mcp-deepseek-work",
        providerID: "deepseek",
        providerAccountID: "deepseek.work",
        expectedRevision: 2
      )
    )
    XCTAssertTrue(revoked)
    XCTAssertEqual(client.remoteToolRevokeRequests.count, 1)
    XCTAssertEqual(
      store.setupSnapshot?.providerAccounts.first?.remoteToolBackends.first?.status,
      "revoked"
    )
    XCTAssertEqual(
      store.remoteToolEnrollmentOperationDetail["mcp-deepseek-work"],
      "Remote tool revoked"
    )
    XCTAssertEqual(store.setupState, .ready)
  }

  func testRemoteToolEnrollmentConflictDoesNotTakeSetupOffline() async throws {
    let client = try ProviderSetupStubClient(
      deepSeekEnabled: true,
      remoteToolEnrollmentConfigured: true,
      remoteToolConfigureError: LocalIPCRemoteError(
        code: .staleView, recoverable: true,
        incidentID: "remote-tool-incident-1"
      )
    )
    let store = LocalProductStore(client: client)
    await store.refreshSetup()
    await store.connectProvider(
      providerID: "deepseek", providerAccountID: "deepseek.work", secret: "sk-test"
    )

    let configured = await store.configureRemoteToolBackend(
      LocalProductRemoteToolBackendEnrollmentCommand.fixture(expectedRevision: 1)
    )
    XCTAssertFalse(configured)
    XCTAssertEqual(store.setupState, .ready)
    XCTAssertEqual(
      store.remoteToolEnrollmentOperationDetail["mcp-deepseek-work"],
      "Remote tool policy changed. Reload this account and try again · Incident remote-tool-incident-1"
    )
    XCTAssertEqual(
      store.remoteToolEnrollmentIncidentID["mcp-deepseek-work"],
      "remote-tool-incident-1"
    )
    XCTAssertEqual(store.remoteToolEnrollmentRetryable["mcp-deepseek-work"], true)
  }

  func testWorkspaceAttentionUsesActionContextAndKindFallback() async throws {
    let longAction = String(repeating: "a", count: 60)
    let attention = """
      "attention":[
        {"schema_version":1,"attention_id":"attention-runtime",
         "kind":"runtime_offline","severity":"warning",
         "team_instance_id":"","logical_node_id":"",
         "work_item_id":"","approval_request_id":"",
         "runtime_instance_id":"runtime-1","status":"open",
         "occurred_at":"2026-08-09T00:00:00Z",
         "action_required":"\\u001b[31mrestore_runtime"},
        {"schema_version":1,"attention_id":"attention-inspect",
         "kind":"inspect_failure","severity":"warning",
         "team_instance_id":"team-1","logical_node_id":"main",
         "work_item_id":"work-1","approval_request_id":"",
         "runtime_instance_id":"runtime-1","status":"open",
         "occurred_at":"2026-08-09T00:00:01Z",
         "action_required":""},
        {"schema_version":1,"attention_id":"attention-osc",
         "kind":"review_team","severity":"warning",
         "team_instance_id":"team-1","logical_node_id":"main",
         "work_item_id":"work-1","approval_request_id":"",
         "runtime_instance_id":"runtime-1","status":"open",
         "occurred_at":"2026-08-09T00:00:02Z",
         "action_required":"\\u001b]8;;https://invalid.example\\u0007review_team\\u001b]8;;\\u0007"},
        {"schema_version":1,"attention_id":"attention-bidi",
         "kind":"inspect_logs","severity":"warning",
         "team_instance_id":"team-1","logical_node_id":"main",
         "work_item_id":"work-1","approval_request_id":"",
         "runtime_instance_id":"runtime-1","status":"open",
         "occurred_at":"2026-08-09T00:00:03Z",
         "action_required":"\\u202einspect_logs"},
        {"schema_version":1,"attention_id":"attention-bounded",
         "kind":"inspect_logs","severity":"warning",
         "team_instance_id":"team-1","logical_node_id":"main",
         "work_item_id":"work-1","approval_request_id":"",
         "runtime_instance_id":"runtime-1","status":"open",
         "occurred_at":"2026-08-09T00:00:04Z",
         "action_required":"\(longAction)"}
      ]
      """
    let json = LocalProductModelsTests.snapshotJSON.replacingOccurrences(
      of: "\"attention\":[]",
      with: attention
    )
    let snapshot = try LocalProductWire.decodeSnapshot(Data(json.utf8))
    let store = LocalProductStore(
      client: StubLocalProductClient(snapshots: [.success(snapshot)])
    )

    await store.refresh()

    let titles = store.workspace.tasks
      .filter { $0.kind == .attention }
      .map(\.title)
    XCTAssertEqual(
      titles,
      [
        "Restore Runtime",
        "Inspect Failure",
        "Review Team",
        "Inspect Logs",
        String(repeating: "a", count: 48).capitalized + "…",
      ]
    )
    XCTAssertFalse(titles.contains("Needs your attention"))
  }

  func testRestartedStoreDecidesFromAuthoritativeSideTaskExecutionBinding() async throws {
    let fixture = try restartedSideTaskFixture()
    let client = RestartedSideTaskStubClient(
      snapshot: fixture.snapshot,
      timeline: fixture.timeline
    )
    let store = LocalProductStore(client: client)

    await store.refresh()
    await store.openMissionAndActivate("mission/team-1")
    XCTAssertNil(store.executionResult)
    let sideTask = try XCTUnwrap(store.snapshot?.sideTasks.first)

    await store.decideSideTask(sideTask, decision: "discard")

    let request = try XCTUnwrap(client.decisions.first)
    XCTAssertEqual(request.parentExecutionDigest, fixture.parentExecutionDigest)
    XCTAssertEqual(request.sideTaskID, sideTask.sideTaskID)
    XCTAssertEqual(request.parentClaimGeneration, 2)
    XCTAssertEqual(request.effectDigest.count, 64)
  }

  func testEmptyTeamActivationDoesNotRequestTimeline() async {
    let client = StubLocalProductClient(
      snapshots: [.success(.empty(viewVersion: "view-1"))]
    )
    let store = LocalProductStore(client: client)

    await store.refresh()
    await store.activateSelectedTeam()

    XCTAssertEqual(client.timelineRequestCount, 0)
    XCTAssertEqual(
      store.teamTimelineMessage,
      "Select a Team from Teams to open its authoritative timeline."
    )
    XCTAssertEqual(
      LocalProductStore.teamsEmptyMessage,
      "No Teams exist in this Journal yet."
    )
  }

  func testFailedRefreshPreservesPriorViewAndMarksOffline() async {
    let original = LocalProductSnapshot.empty(viewVersion: "view-1")
    let client = StubLocalProductClient(
      snapshots: [
        .success(original),
        .failure(LocalProductClientError.unavailable),
      ]
    )
    let store = LocalProductStore(client: client)

    await store.refresh()
    await store.refresh()

    XCTAssertEqual(store.snapshot, original)
    XCTAssertEqual(store.connectionState, .offline(reason: "unavailable"))
  }

  func testConnectWithRetryWaitsForBundledServiceSocket() async {
    let expected = LocalProductSnapshot.empty(viewVersion: "view-ready")
    let client = StubLocalProductClient(
      snapshots: [
        .failure(LocalProductClientError.unavailable),
        .failure(LocalProductClientError.unavailable),
        .success(expected),
      ]
    )
    let store = LocalProductStore(client: client)

    await store.connectWithRetry(maxAttempts: 4, delayNanoseconds: 0)

    XCTAssertEqual(store.snapshot, expected)
    XCTAssertEqual(store.connectionState, .online)
    XCTAssertEqual(client.snapshotRequestCount, 3)
  }

  func testRefreshPreservesSelectedTaskDraftAndInspector() async throws {
    let json = LocalProductModelsTests.snapshotJSON.replacingOccurrences(
      of: "\"teams\":[]",
      with: """
        "teams":[{
          "team_instance_id":"team-1",
          "display_name":"Release review",
          "source_kind":"saved",
          "state":"ready",
          "confirmed":true,
          "executable":true,
          "read_only":false
        }]
        """
    )
    let snapshot = try LocalProductWire.decodeSnapshot(Data(json.utf8))
    let client = StubLocalProductClient(
      snapshots: [
        .success(snapshot),
        .failure(LocalProductClientError.unavailable),
        .success(snapshot),
      ]
    )
    let store = LocalProductStore(client: client)
    await store.refresh()
    let task = try XCTUnwrap(
      store.workspace.tasks.first(where: { $0.kind == .team })
    )
    store.selectWorkspaceTask(task.id)
    store.updateComposerDraft("Keep this local note")
    store.selectInspector(.changes)

    await store.refresh()
    XCTAssertEqual(store.workspace.selectedTaskID, task.id)
    XCTAssertEqual(
      store.workspace.selectedContinuity.composerDraft,
      "Keep this local note"
    )
    XCTAssertEqual(store.workspace.selectedContinuity.inspector, .changes)

    await store.refresh()
    XCTAssertEqual(store.workspace.selectedTaskID, task.id)
    XCTAssertEqual(
      store.workspace.selectedContinuity.composerDraft,
      "Keep this local note"
    )
    XCTAssertEqual(store.workspace.selectedContinuity.inspector, .changes)
  }

  func testSelectingVisibleTeamPerformsOneBoundedTimelineRequest() async throws {
    let json = LocalProductModelsTests.snapshotJSON.replacingOccurrences(
      of: "\"teams\":[]",
      with: """
        "teams":[{
          "team_instance_id":"team-1",
          "display_name":"Review Team",
          "source_kind":"saved",
          "state":"ready",
          "confirmed":true,
          "executable":true,
          "read_only":false
        }]
        """
    )
    let snapshot = try LocalProductWire.decodeSnapshot(Data(json.utf8))
    let client = StubLocalProductClient(snapshots: [.success(snapshot)])
    let store = LocalProductStore(client: client)

    await store.refresh()
    store.selectTeam(snapshot.teams[0])
    XCTAssertEqual(store.selectedSection, .teams)
    await store.activateSelectedTeam()

    XCTAssertEqual(client.timelineRequestCount, 1)
    XCTAssertEqual(client.lastTimelineLimit, 64)
    XCTAssertEqual(client.lastTimelineTeamID, "team-1")
  }

  func testWorkspaceTeamSelectionLoadsItsBoundedActivity() async throws {
    let json = LocalProductModelsTests.snapshotJSON.replacingOccurrences(
      of: "\"teams\":[]",
      with: """
        "teams":[{
          "team_instance_id":"team-1",
          "display_name":"Review Team",
          "source_kind":"saved",
          "state":"ready",
          "confirmed":true,
          "executable":true,
          "read_only":false
        }]
        """
    )
    let snapshot = try LocalProductWire.decodeSnapshot(Data(json.utf8))
    let client = StubLocalProductClient(snapshots: [.success(snapshot)])
    let store = LocalProductStore(client: client)

    await store.refresh()
    let task = try XCTUnwrap(
      store.workspace.tasks.first(where: { $0.kind == .team })
    )
    store.selectWorkspaceTask(task.id)
    await store.activateWorkspaceTask()

    XCTAssertEqual(client.timelineRequestCount, 1)
    XCTAssertEqual(client.lastTimelineLimit, 64)
    XCTAssertEqual(client.lastTimelineTeamID, "team-1")
  }

  func testNonrecoverableDaemonErrorIsFatal() async {
    let client = StubLocalProductClient(
      snapshots: [
        .failure(
          LocalIPCRemoteError(
            code: .unauthorizedPeer,
            recoverable: false
          )
        )
      ]
    )
    let store = LocalProductStore(client: client)

    await store.refresh()

    XCTAssertEqual(
      store.connectionState,
      .fatal(reason: "unauthorized_peer")
    )
  }

  func testTimelineFailureClosesLoadingPresentation() async throws {
    let json = LocalProductModelsTests.snapshotJSON.replacingOccurrences(
      of: "\"teams\":[]",
      with: """
        "teams":[{
          "team_instance_id":"team-1",
          "display_name":"Review Team",
          "source_kind":"saved",
          "state":"ready",
          "confirmed":true,
          "executable":true,
          "read_only":false
        }]
        """
    )
    let snapshot = try LocalProductWire.decodeSnapshot(Data(json.utf8))
    let client = StubLocalProductClient(snapshots: [.success(snapshot)])
    let store = LocalProductStore(client: client)

    await store.refresh()
    store.selectTeam(snapshot.teams[0])
    XCTAssertEqual(store.timelineState, .loading)

    await store.activateSelectedTeam()

    XCTAssertEqual(store.timelineState, .unavailable)
    XCTAssertNil(store.timeline)
  }

  func testTimelineLoadsEveryBoundedPageBeforePublishingAuthoritativeHistory() async throws {
    let snapshot = try timelineSnapshot(teamIDs: ["team-1"])
    let first = try timelinePage(
      teamID: "team-1",
      nextCursor: "cursor-1",
      hasMore: true,
      records: [
        timelineRecord(
          deliveryID: "delivery-source-evidence",
          kind: "evidence_available",
          teamID: "team-1"
        )
      ]
    )
    let second = try timelinePage(
      teamID: "team-1",
      nextCursor: "cursor-2",
      hasMore: false,
      records: [
        timelineRecord(
          deliveryID: "delivery-verifier-evidence",
          kind: "evidence_available",
          teamID: "team-1"
        ),
        timelineRecord(
          deliveryID: "delivery-terminal",
          kind: "team_terminal",
          teamID: "team-1",
          status: "succeeded"
        ),
      ]
    )
    let client = TimelinePagingStubClient(
      snapshot: snapshot,
      pages: ["": first, "cursor-1": second]
    )
    let store = LocalProductStore(client: client)

    await store.refresh()
    store.selectTeam(snapshot.teams[0])
    await store.activateSelectedTeam()

    XCTAssertEqual(
      client.timelineRequests,
      [
        .init(teamID: "team-1", cursor: "", limit: 64),
        .init(teamID: "team-1", cursor: "cursor-1", limit: 64),
      ]
    )
    XCTAssertEqual(store.timelineState, .loaded)
    XCTAssertEqual(
      store.timeline?.records.map(\.deliveryID),
      [
        "delivery-source-evidence",
        "delivery-verifier-evidence",
        "delivery-terminal",
      ]
    )
    XCTAssertEqual(store.timeline?.records.last?.payload.status, "succeeded")
    XCTAssertEqual(store.timeline?.nextCursor, "cursor-2")
    XCTAssertEqual(store.timeline?.hasMore, false)
    XCTAssertNil(store.timeline?.gap)
  }

  func testTimelineRejectsNestedIdentityDriftDuplicateDeliveryAndCursorReplay() async throws {
    let snapshot = try timelineSnapshot(teamIDs: ["team-1"])
    let validRecord = timelineRecord(
      deliveryID: "delivery-1",
      kind: "evidence_available",
      teamID: "team-1"
    )
    let cases: [(String, [String: LocalProductTimelinePage])] = [
      (
        "first_page_schema_drift",
        [
          "": try timelinePage(
            schemaVersion: 2,
            teamID: "team-1",
            hasMore: false,
            records: [validRecord]
          )
        ]
      ),
      (
        "first_page_team_drift",
        [
          "": try timelinePage(
            teamID: "team-other",
            hasMore: false,
            records: [
              timelineRecord(
                deliveryID: "delivery-1",
                kind: "evidence_available",
                teamID: "team-other"
              )
            ]
          )
        ]
      ),
      (
        "first_page_empty_view",
        [
          "": try timelinePage(
            teamID: "team-1",
            viewVersion: "",
            hasMore: false,
            records: [validRecord]
          )
        ]
      ),
      (
        "first_page_board_schema_drift",
        [
          "": try timelinePage(
            teamID: "team-1",
            boardSchemaVersion: 2,
            hasMore: false,
            records: [validRecord]
          )
        ]
      ),
      (
        "first_page_board_team_drift",
        [
          "": try timelinePage(
            teamID: "team-1",
            boardTeamID: "team-other",
            hasMore: false,
            records: [validRecord]
          )
        ]
      ),
      (
        "first_page_board_view_drift",
        [
          "": try timelinePage(
            teamID: "team-1",
            boardViewVersion: "view-other",
            hasMore: false,
            records: [validRecord]
          )
        ]
      ),
      (
        "first_page_attention_schema_drift",
        [
          "": try timelinePage(
            teamID: "team-1",
            hasMore: false,
            records: [validRecord],
            attention: [
              timelineAttention(
                schemaVersion: 2,
                attentionID: "attention-1",
                teamID: "team-1"
              )
            ]
          )
        ]
      ),
      (
        "first_page_attention_team_drift",
        [
          "": try timelinePage(
            teamID: "team-1",
            hasMore: false,
            records: [validRecord],
            attention: [
              timelineAttention(
                attentionID: "attention-1",
                teamID: "team-other"
              )
            ]
          )
        ]
      ),
      (
        "first_page_empty_attention_id",
        [
          "": try timelinePage(
            teamID: "team-1",
            hasMore: false,
            records: [validRecord],
            attention: [
              timelineAttention(
                attentionID: "",
                teamID: "team-1"
              )
            ]
          )
        ]
      ),
      (
        "first_page_duplicate_attention_id",
        [
          "": try timelinePage(
            teamID: "team-1",
            hasMore: false,
            records: [validRecord],
            attention: [
              timelineAttention(
                attentionID: "attention-1",
                teamID: "team-1"
              ),
              timelineAttention(
                attentionID: "attention-1",
                teamID: "team-1"
              ),
            ]
          )
        ]
      ),
      (
        "first_page_record_schema_drift",
        [
          "": try timelinePage(
            teamID: "team-1",
            hasMore: false,
            records: [
              timelineRecord(
                schemaVersion: 2,
                deliveryID: "delivery-1",
                kind: "evidence_available",
                teamID: "team-1"
              )
            ]
          )
        ]
      ),
      (
        "first_page_record_team_drift",
        [
          "": try timelinePage(
            teamID: "team-1",
            hasMore: false,
            records: [
              timelineRecord(
                deliveryID: "delivery-1",
                kind: "evidence_available",
                teamID: "team-other"
              )
            ]
          )
        ]
      ),
      (
        "first_page_empty_delivery_id",
        [
          "": try timelinePage(
            teamID: "team-1",
            hasMore: false,
            records: [
              timelineRecord(
                deliveryID: "",
                kind: "evidence_available",
                teamID: "team-1"
              )
            ]
          )
        ]
      ),
      (
        "later_page_board_drift",
        [
          "": try timelinePage(
            teamID: "team-1",
            nextCursor: "cursor-1",
            hasMore: true,
            records: [validRecord]
          ),
          "cursor-1": try timelinePage(
            teamID: "team-1",
            boardStatus: "failed",
            hasMore: false,
            records: []
          ),
        ]
      ),
      (
        "later_page_attention_drift",
        [
          "": try timelinePage(
            teamID: "team-1",
            nextCursor: "cursor-1",
            hasMore: true,
            records: [validRecord]
          ),
          "cursor-1": try timelinePage(
            teamID: "team-1",
            hasMore: false,
            records: [],
            attention: [
              timelineAttention(
                attentionID: "attention-1",
                teamID: "team-1"
              )
            ]
          ),
        ]
      ),
      (
        "later_page_view_drift",
        [
          "": try timelinePage(
            teamID: "team-1",
            nextCursor: "cursor-1",
            hasMore: true,
            records: [validRecord]
          ),
          "cursor-1": try timelinePage(
            teamID: "team-1",
            viewVersion: "view-other",
            hasMore: false,
            records: []
          ),
        ]
      ),
      (
        "duplicate_delivery",
        [
          "": try timelinePage(
            teamID: "team-1",
            nextCursor: "cursor-1",
            hasMore: true,
            records: [validRecord]
          ),
          "cursor-1": try timelinePage(
            teamID: "team-1",
            hasMore: false,
            records: [validRecord]
          ),
        ]
      ),
      (
        "replayed_cursor",
        [
          "": try timelinePage(
            teamID: "team-1",
            nextCursor: "cursor-1",
            hasMore: true,
            records: [validRecord]
          ),
          "cursor-1": try timelinePage(
            teamID: "team-1",
            nextCursor: "cursor-1",
            hasMore: true,
            records: []
          ),
        ]
      ),
      (
        "empty_continuation",
        [
          "": try timelinePage(
            teamID: "team-1",
            nextCursor: "",
            hasMore: true,
            records: [validRecord]
          )
        ]
      ),
    ]

    for (name, pages) in cases {
      let client = TimelinePagingStubClient(snapshot: snapshot, pages: pages)
      let store = LocalProductStore(client: client)
      await store.refresh()
      store.selectTeam(snapshot.teams[0])
      await store.activateSelectedTeam()

      XCTAssertNil(store.timeline, name)
      XCTAssertEqual(store.timelineState, .unavailable, name)
    }
  }

  func testTimelineRejectsGapAndBoundedPageOverflowWithoutPartialPublication() async throws {
    let snapshot = try timelineSnapshot(teamIDs: ["team-1"])
    let gapClient = TimelinePagingStubClient(
      snapshot: snapshot,
      pages: [
        "": try timelinePage(
          teamID: "team-1",
          hasMore: false,
          gap: true,
          records: []
        )
      ]
    )
    let gapStore = LocalProductStore(client: gapClient)
    await gapStore.refresh()
    gapStore.selectTeam(snapshot.teams[0])
    await gapStore.activateSelectedTeam()
    XCTAssertNil(gapStore.timeline)
    XCTAssertEqual(gapStore.timelineState, .unavailable)

    var pages: [String: LocalProductTimelinePage] = [:]
    for index in 0..<8 {
      let cursor = index == 0 ? "" : "cursor-\(index)"
      pages[cursor] = try timelinePage(
        teamID: "team-1",
        nextCursor: "cursor-\(index + 1)",
        hasMore: true,
        records: [
          timelineRecord(
            deliveryID: "delivery-\(index)",
            kind: "node_scheduled",
            teamID: "team-1"
          )
        ]
      )
    }
    let overflowClient = TimelinePagingStubClient(snapshot: snapshot, pages: pages)
    let overflowStore = LocalProductStore(client: overflowClient)
    await overflowStore.refresh()
    overflowStore.selectTeam(snapshot.teams[0])
    await overflowStore.activateSelectedTeam()

    XCTAssertEqual(overflowClient.timelineRequests.count, 8)
    XCTAssertNil(overflowStore.timeline)
    XCTAssertEqual(overflowStore.timelineState, .unavailable)
  }

  func testTimelineEnforcesExact512RecordBoundary() async throws {
    let snapshot = try timelineSnapshot(teamIDs: ["team-1"])
    func pages(finalHasMore: Bool, finalRecordCount: Int = 64) throws
      -> [String: LocalProductTimelinePage]
    {
      var result: [String: LocalProductTimelinePage] = [:]
      var deliveryIndex = 0
      for pageIndex in 0..<8 {
        let cursor = pageIndex == 0 ? "" : "cursor-\(pageIndex)"
        let count = pageIndex == 7 ? finalRecordCount : 64
        let records = (0..<count).map { _ in
          defer { deliveryIndex += 1 }
          return timelineRecord(
            deliveryID: "delivery-\(deliveryIndex)",
            kind: "node_scheduled",
            teamID: "team-1"
          )
        }
        result[cursor] = try timelinePage(
          teamID: "team-1",
          nextCursor: "cursor-\(pageIndex + 1)",
          hasMore: pageIndex == 7 ? finalHasMore : true,
          records: records
        )
      }
      return result
    }

    let acceptedClient = TimelinePagingStubClient(
      snapshot: snapshot,
      pages: try pages(finalHasMore: false)
    )
    let acceptedStore = LocalProductStore(client: acceptedClient)
    await acceptedStore.refresh()
    acceptedStore.selectTeam(snapshot.teams[0])
    await acceptedStore.activateSelectedTeam()
    XCTAssertEqual(acceptedClient.timelineRequests.count, 8)
    XCTAssertEqual(acceptedStore.timelineState, .loaded)
    XCTAssertEqual(acceptedStore.timeline?.records.count, 512)

    for (name, testPages) in [
      ("512_with_continuation", try pages(finalHasMore: true)),
      ("513_final", try pages(finalHasMore: false, finalRecordCount: 65)),
    ] {
      let client = TimelinePagingStubClient(snapshot: snapshot, pages: testPages)
      let store = LocalProductStore(client: client)
      await store.refresh()
      store.selectTeam(snapshot.teams[0])
      await store.activateSelectedTeam()
      XCTAssertNil(store.timeline, name)
      XCTAssertEqual(store.timelineState, .unavailable, name)
    }
  }

  func testNewTeamSelectionFencesLateTimelineSuccessAndError() async throws {
    let snapshot = try timelineSnapshot(teamIDs: ["team-1", "team-2"])
    for lateResult in [
      Result<LocalProductTimelinePage, Error>.success(
        try timelinePage(teamID: "team-1", hasMore: false, records: [])
      ),
      .failure(LocalProductClientError.unavailable),
    ] {
      let client = SuspendedTimelineStubClient(
        snapshot: snapshot,
        immediatePages: [
          "team-2": try timelinePage(
            teamID: "team-2",
            hasMore: false,
            records: [
              timelineRecord(
                deliveryID: "delivery-team-2",
                kind: "team_terminal",
                teamID: "team-2",
                status: "succeeded"
              )
            ]
          )
        ]
      )
      let store = LocalProductStore(client: client)
      await store.refresh()
      store.selectTeam(snapshot.teams[0])
      let staleLoad = Task { await store.activateSelectedTeam() }
      await client.waitForSuspendedRequest()

      store.selectTeam(snapshot.teams[1])
      await store.activateSelectedTeam()
      await client.resolveSuspended(with: lateResult)
      await staleLoad.value

      XCTAssertEqual(store.selectedTeamID, "team-2")
      XCTAssertEqual(store.timeline?.teamInstanceID, "team-2")
      XCTAssertEqual(store.timeline?.records.map(\.deliveryID), ["delivery-team-2"])
      XCTAssertEqual(store.timelineState, .loaded)
      XCTAssertEqual(store.connectionState, .online)
    }
  }

  func testCancelledTimelineLoadCannotPublishLateSuccessOrError() async throws {
    let snapshot = try timelineSnapshot(teamIDs: ["team-1"])
    for lateResult in [
      Result<LocalProductTimelinePage, Error>.success(
        try timelinePage(teamID: "team-1", hasMore: false, records: [])
      ),
      .failure(LocalProductClientError.unavailable),
    ] {
      let client = SuspendedTimelineStubClient(snapshot: snapshot)
      let store = LocalProductStore(client: client)
      await store.refresh()
      store.selectTeam(snapshot.teams[0])
      let load = Task { await store.activateSelectedTeam() }
      await client.waitForSuspendedRequest()

      load.cancel()
      await client.resolveSuspended(with: lateResult)
      await load.value

      XCTAssertNil(store.timeline)
      XCTAssertEqual(store.timelineState, .idle)
      XCTAssertEqual(store.connectionState, .online)
    }
  }

  func testLeavingMissionFencesLateTimelineSuccess() async throws {
    let snapshot = try LocalProductWire.decodeSnapshot(
      Data(MissionOrchestrationTests.snapshotJSON.utf8)
    )
    let client = SuspendedTimelineStubClient(snapshot: snapshot)
    let store = LocalProductStore(client: client)
    await store.refresh()
    let load = Task {
      await store.openMissionAndActivate("mission/team-1")
    }
    await client.waitForSuspendedRequest()

    store.showMissionBoard()
    await client.resolveSuspended(
      with: .success(
        try timelinePage(teamID: "team-1", hasMore: false, records: [])
      )
    )
    await load.value

    XCTAssertEqual(store.workbench.route, .board)
    XCTAssertNil(store.timeline)
    XCTAssertEqual(store.timelineState, .idle)
    XCTAssertEqual(store.connectionState, .online)
  }

  func testNewMissionSelectionFencesLateTimelineSuccessAndError() async throws {
    let snapshot = try twoMissionSnapshot()
    for lateResult in [
      Result<LocalProductTimelinePage, Error>.success(
        try timelinePage(teamID: "team-1", hasMore: false, records: [])
      ),
      .failure(LocalProductClientError.unavailable),
    ] {
      let client = SuspendedTimelineStubClient(
        snapshot: snapshot,
        immediatePages: [
          "team-2": try timelinePage(
            teamID: "team-2",
            hasMore: false,
            records: [
              timelineRecord(
                deliveryID: "delivery-team-2",
                kind: "team_terminal",
                teamID: "team-2",
                status: "succeeded"
              )
            ]
          )
        ]
      )
      let store = LocalProductStore(client: client)
      await store.refresh()
      let staleLoad = Task {
        await store.openMissionAndActivate("mission/team-1")
      }
      await client.waitForSuspendedRequest()

      await store.openMissionAndActivate("mission/team-2")
      await client.resolveSuspended(with: lateResult)
      await staleLoad.value

      XCTAssertEqual(store.workbench.route, .mission("mission/team-2"))
      XCTAssertEqual(store.timeline?.teamInstanceID, "team-2")
      XCTAssertEqual(store.timeline?.records.map(\.deliveryID), ["delivery-team-2"])
      XCTAssertEqual(store.timelineState, .loaded)
      XCTAssertEqual(store.connectionState, .online)
    }
  }

  func testNewerLoadForSameSelectionFencesOlderSuccessAndError() async throws {
    let snapshot = try timelineSnapshot(teamIDs: ["team-1"])
    for staleResult in [
      Result<LocalProductTimelinePage, Error>.success(
        try timelinePage(teamID: "team-1", hasMore: false, records: [])
      ),
      .failure(LocalProductClientError.unavailable),
    ] {
      let client = SequencedSuspendedTimelineStubClient(snapshot: snapshot)
      let store = LocalProductStore(client: client)
      await store.refresh()
      store.selectTeam(snapshot.teams[0])
      let staleLoad = Task { await store.activateSelectedTeam() }
      await client.waitForRequestCount(1)
      let currentLoad = Task { await store.activateSelectedTeam() }
      await client.waitForRequestCount(2)

      await client.resolveRequest(
        at: 1,
        with: .success(
          try timelinePage(
            teamID: "team-1",
            hasMore: false,
            records: [
              timelineRecord(
                deliveryID: "delivery-current",
                kind: "team_terminal",
                teamID: "team-1",
                status: "succeeded"
              )
            ]
          )
        )
      )
      await currentLoad.value
      await client.resolveRequest(at: 0, with: staleResult)
      await staleLoad.value

      XCTAssertEqual(store.timeline?.records.map(\.deliveryID), ["delivery-current"])
      XCTAssertEqual(store.timelineState, .loaded)
      XCTAssertEqual(store.connectionState, .online)
    }
  }

  func testConnectCodexDelegatesThenRefreshesToAvailable() async throws {
    let client = try ProviderSetupStubClient()
    let store = LocalProductStore(client: client)

    await store.refreshSetup()
    XCTAssertEqual(store.setupSnapshot?.codex.status, "not_logged_in")

    await store.connectCodex()

    XCTAssertEqual(client.connectRequestCount, 1)
    XCTAssertEqual(client.setupRequestCount, 2)
    XCTAssertEqual(
      store.providerConnectionStatus,
      LocalProductProviderConnectResult.fixture(status: "started")
    )
    XCTAssertEqual(store.setupSnapshot?.codex.status, "available")
    XCTAssertEqual(store.setupState, .ready)
  }

  func testConnectCodexPollingCancelsWithoutInventingFailure() async throws {
    let client = try ProviderSetupStubClient(connectsAfterStart: false)
    let store = LocalProductStore(client: client)
    await store.refreshSetup()

    let connection = Task { await store.connectCodex() }
    await Task.yield()
    connection.cancel()
    await connection.value

    XCTAssertEqual(client.connectRequestCount, 1)
    XCTAssertEqual(store.setupSnapshot?.codex.status, "not_logged_in")
    XCTAssertEqual(store.setupState, .ready)
  }

  func testVerifyMiniMaxPresentsReturnedTerminalRevisionAndRefreshes() async throws {
    let client = try ProviderSetupStubClient(miniMaxConfigured: true)
    let store = LocalProductStore(client: client)
    await store.refreshSetup()

    XCTAssertEqual(store.setupSnapshot?.miniMax.revision, 1)
    XCTAssertEqual(store.setupSnapshot?.miniMax.status, "configured")

    await store.verifyMiniMax()

    XCTAssertEqual(client.verifyRequestCount, 1)
    XCTAssertEqual(store.credentialStatus?.revision, 2)
    XCTAssertEqual(store.credentialStatus?.status, "verified")
    XCTAssertEqual(store.setupSnapshot?.miniMax.revision, 2)
    XCTAssertEqual(store.setupSnapshot?.miniMax.status, "verified")
    XCTAssertEqual(store.setupState, .ready)
    XCTAssertEqual(store.miniMaxVerificationStatus, "Verified")
    XCTAssertFalse(store.isVerifyingMiniMax)
  }

  func testVerifyMiniMaxRejectsMismatchedAuthoritativeRefresh() async throws {
    let client = try ProviderSetupStubClient(
      miniMaxConfigured: true,
      mismatchedMiniMaxRefresh: true
    )
    let store = LocalProductStore(client: client)
    await store.refreshSetup()

    await store.verifyMiniMax()

    XCTAssertNil(store.credentialStatus)
    XCTAssertEqual(store.setupSnapshot?.miniMax.revision, 1)
    XCTAssertEqual(store.miniMaxVerificationStatus, "Unavailable")
    XCTAssertFalse(store.isVerifyingMiniMax)
    XCTAssertEqual(
      store.setupState,
      .unavailable(reason: "invalid_response")
    )
  }

  func testVerifyMiniMaxPresentsRejectedAndUnavailableTerminalStates() async throws {
    for (status, reason, expected) in [
      ("rejected", "provider_rejected", "Rejected"),
      ("rejected", "timeout", "Unavailable"),
      ("rejected", "unavailable", "Unavailable"),
    ] {
      let client = try ProviderSetupStubClient(
        miniMaxConfigured: true,
        miniMaxTerminalStatus: status,
        miniMaxTerminalReason: reason
      )
      let store = LocalProductStore(client: client)
      await store.refreshSetup()
      await store.verifyMiniMax()

      XCTAssertEqual(store.miniMaxVerificationStatus, expected)
      XCTAssertEqual(store.credentialStatus?.status, status)
      XCTAssertEqual(store.credentialStatus?.reason, reason)
      XCTAssertEqual(store.setupState, .ready)
    }
  }

  func testVerifyMiniMaxPresentsConflictWithoutInventingSuccess() async throws {
    let client = try ProviderSetupStubClient(
      miniMaxConfigured: true,
      miniMaxVerifyError: LocalIPCRemoteError(
        code: .conflict,
        recoverable: true
      )
    )
    let store = LocalProductStore(client: client)
    await store.refreshSetup()
    await store.verifyMiniMax()

    XCTAssertEqual(store.miniMaxVerificationStatus, "Conflict")
    XCTAssertNil(store.credentialStatus)
    XCTAssertEqual(store.setupSnapshot?.miniMax.revision, 1)
    XCTAssertEqual(store.setupState, .unavailable(reason: "conflict"))
  }

  func testConfirmedExecutableTeamRefreshesAuthoritativeProductView() async throws {
    let client = try ProviderSetupStubClient(materializesTeam: true)
    let store = LocalProductStore(client: client)

    await store.refresh()
    await store.startBlankBuilder()
    XCTAssertTrue(try XCTUnwrap(store.builderSession).canConfirm)

    await store.confirmBuilder()

    XCTAssertEqual(client.productSnapshotRequestCount, 2)
    XCTAssertTrue(try XCTUnwrap(store.lastConfirmation).teamInstanceCreated)
    XCTAssertEqual(store.snapshot?.teams.count, 1)
    XCTAssertEqual(store.snapshot?.teams.first?.teamInstanceID, "team-instance-fixture")
    XCTAssertTrue(store.snapshot?.teams.first?.executable == true)
  }

  func testConfirmBuilderCommittingAppliesUncommittedFieldsFirst() async throws {
    let client = try ProviderSetupStubClient(materializesTeam: true)
    let store = LocalProductStore(client: client)

    await store.refresh()
    await store.startBlankBuilder()
    XCTAssertTrue(try XCTUnwrap(store.builderSession).canConfirm)

    // Type name + purpose without the Return-commit step, then Confirm.
    await store.confirmBuilderCommitting(
      name: "UI Mission Team",
      purpose: "Verify the governed journey"
    )

    // The uncommitted name/purpose edits were sent to the daemon before confirm.
    XCTAssertEqual(
      client.builderEdits.map(\.field),
      ["team_name", "purpose"]
    )
    XCTAssertEqual(client.builderEdits.first?.value, "UI Mission Team")
    XCTAssertEqual(client.builderEdits.last?.value, "Verify the governed journey")
    XCTAssertEqual(client.builderConfirmRequestCount, 1)
    XCTAssertEqual(store.snapshot?.teams.count, 1)
  }

  func testConfirmBuilderCommittingSkipsUnchangedFields() async throws {
    let client = try ProviderSetupStubClient(materializesTeam: true)
    let store = LocalProductStore(client: client)

    await store.refresh()
    await store.startBlankBuilder()
    // Preview name is "Controlled Team"; passing the same value must not edit.
    await store.confirmBuilderCommitting(
      name: "Controlled Team",
      purpose: "One bounded Mission"
    )

    XCTAssertTrue(client.builderEdits.isEmpty)
    XCTAssertEqual(client.builderConfirmRequestCount, 1)
  }

  func testExecutableTeamsExposesOnlyConfirmedRunnableTeams() async throws {
    let client = try ProviderSetupStubClient(materializesTeam: true)
    let store = LocalProductStore(client: client)

    await store.refresh()
    XCTAssertTrue(store.executableTeams.isEmpty)

    await store.startBlankBuilder()
    await store.confirmBuilder()

    XCTAssertEqual(store.executableTeams.count, 1)
    XCTAssertEqual(store.executableTeams.first?.teamInstanceID, "team-instance-fixture")
  }

  func testBuilderForwardsIndependentExecutionProfileEdit() async throws {
    let client = try ProviderSetupStubClient(materializesTeam: true)
    let store = LocalProductStore(client: client)

    await store.refreshSetup()
    await store.startBlankBuilder()
    await store.editBuilder(field: "main_reasoning_effort", value: "medium")

    XCTAssertEqual(client.builderEdits.count, 1)
    XCTAssertEqual(client.builderEdits.first?.field, "main_reasoning_effort")
    XCTAssertEqual(client.builderEdits.first?.value, "medium")
    XCTAssertEqual(store.setupState, .ready)
  }

  func testBuilderForwardsExplicitFallbackRouteEdit() async throws {
    let client = try ProviderSetupStubClient(materializesTeam: true)
    let store = LocalProductStore(client: client)

    await store.refreshSetup()
    await store.startBlankBuilder()
    await store.editBuilder(field: "main_fallback_role", value: "role-main-backup")

    XCTAssertEqual(client.builderEdits.count, 1)
    XCTAssertEqual(client.builderEdits.first?.field, "main_fallback_role")
    XCTAssertEqual(client.builderEdits.first?.value, "role-main-backup")
    XCTAssertEqual(store.setupState, .ready)
  }

  func testBuilderForwardsProviderAccountRouteEdit() async throws {
    let client = try ProviderSetupStubClient(materializesTeam: true)
    let store = LocalProductStore(client: client)

    await store.refreshSetup()
    await store.startBlankBuilder()
    let originalDraftID = try XCTUnwrap(store.builderSession?.draftID)

    await store.editBuilder(
      field: "main_provider_account_route",
      value: "role-main-backup"
    )

    XCTAssertEqual(client.builderEdits.count, 1)
    XCTAssertEqual(client.builderEdits.first?.field, "main_provider_account_route")
    XCTAssertEqual(client.builderEdits.first?.value, "role-main-backup")
    XCTAssertEqual(store.builderSession?.draftID, originalDraftID)
    XCTAssertEqual(store.setupState, .ready)
  }

  func testBuilderRequiresAndForwardsStableSubagentTarget() async throws {
    let client = try ProviderSetupStubClient(materializesTeam: true)
    let store = LocalProductStore(client: client)

    await store.refreshSetup()
    await store.startBlankBuilder()
    await store.editBuilder(
      field: "subagent_provider_account_route",
      value: "role-sub-backup"
    )
    XCTAssertTrue(client.builderEdits.isEmpty)
    XCTAssertEqual(store.setupState, .unavailable(reason: "invalid_request"))

    await store.editBuilder(
      field: "subagent_provider_account_route",
      value: "role-sub-backup",
      roleAgentDefinitionID: "agent-sub"
    )
    XCTAssertEqual(client.targetedBuilderEdits.count, 1)
    XCTAssertEqual(
      client.targetedBuilderEdits.first?.roleAgentDefinitionID,
      "agent-sub"
    )
    XCTAssertEqual(store.setupState, .ready)
  }

  func testStaleBuilderConfirmationRefreshesOnceAndDisablesOldDraft() async throws {
    let client = try ProviderSetupStubClient(
      materializesTeam: true,
      builderConfirmError: LocalIPCRemoteError(
        code: .conflict,
        recoverable: true
      )
    )
    let store = LocalProductStore(client: client)

    await store.refreshSetup()
    await store.startBlankBuilder()
    await store.confirmBuilder()

    XCTAssertEqual(client.builderConfirmRequestCount, 1)
    XCTAssertEqual(client.setupRequestCount, 2)
    XCTAssertNil(store.builderSession)
    XCTAssertNil(store.lastConfirmation)
    XCTAssertEqual(store.setupState, .unavailable(reason: "team_draft_stale"))
    XCTAssertEqual(
      store.builderRecoveryMessage,
      "This Agent Team draft changed in another window. Review the latest Teams, then start a new draft."
    )

    await store.confirmBuilder()

    XCTAssertEqual(client.builderConfirmRequestCount, 1)
    XCTAssertEqual(client.setupRequestCount, 2)
    XCTAssertEqual(store.setupState, .unavailable(reason: "team_draft_stale"))
  }

  func testMissingBuilderConfirmationRefreshesOnceAndDisablesExpiredDraft() async throws {
    let client = try ProviderSetupStubClient(
      materializesTeam: true,
      builderConfirmError: LocalIPCRemoteError(
        code: .notFound,
        recoverable: true
      )
    )
    let store = LocalProductStore(client: client)

    await store.refreshSetup()
    await store.startBlankBuilder()
    await store.confirmBuilder()

    XCTAssertEqual(client.builderConfirmRequestCount, 1)
    XCTAssertEqual(client.setupRequestCount, 2)
    XCTAssertNil(store.builderSession)
    XCTAssertNil(store.lastConfirmation)
    XCTAssertEqual(store.setupState, .unavailable(reason: "team_draft_expired"))
    XCTAssertEqual(
      store.builderRecoveryMessage,
      "This Agent Team draft is no longer available. Review the latest Teams, then start a new draft."
    )

    await store.confirmBuilder()

    XCTAssertEqual(client.builderConfirmRequestCount, 1)
    XCTAssertEqual(client.setupRequestCount, 2)
    XCTAssertEqual(store.setupState, .unavailable(reason: "team_draft_expired"))
  }

  func testPreparedDecisionReadAndNotNowRemainPresentationOnly() async throws {
    let client = try DecisionStubClient()
    let store = LocalProductStore(client: client)
    let read = client.command(operation: "read", action: "read")

    await store.openPreparedDecision(read)
    XCTAssertEqual(store.activeDecisionSheet?.kind, .authorization)
    XCTAssertEqual(client.readCount, 1)
    XCTAssertEqual(client.submitCount, 0)

    await store.submitDecisionAction("not_now")
    XCTAssertNil(store.activeDecisionSheet)
    XCTAssertEqual(client.submitCount, 1)
    XCTAssertEqual(client.lastCommand?.operation, "defer")
    XCTAssertEqual(client.lastCommand?.action, "not_now")
  }

  func testPreparedDecisionSubmitsOnlyExactPreparedAction() async throws {
    let client = try DecisionStubClient()
    let store = LocalProductStore(client: client)

    await store.openPreparedDecision(client.command(operation: "read", action: "read"))
    await store.submitDecisionAction("allow_once")

    XCTAssertNil(store.activeDecisionSheet)
    XCTAssertEqual(client.submitCount, 1)
    XCTAssertEqual(client.lastCommand?.operation, "submit")
    XCTAssertEqual(client.lastCommand?.action, "allow_once")
    XCTAssertEqual(client.lastCommand?.decisionID, "decision-1")
  }

  func testPreparedDecisionRejectsUnpreparedActionWithoutIPC() async throws {
    let client = try DecisionStubClient()
    let store = LocalProductStore(client: client)

    await store.openPreparedDecision(client.command(operation: "read", action: "read"))
    await store.submitDecisionAction("shell_exec")

    XCTAssertNotNil(store.activeDecisionSheet)
    XCTAssertEqual(client.submitCount, 0)
  }

  func testPreparedDecisionKeepsSheetForNonAuthoritativeSubmitResult() async throws {
    let client = try DecisionStubClient(authoritativeSubmit: false)
    let store = LocalProductStore(client: client)

    await store.openPreparedDecision(client.command(operation: "read", action: "read"))
    await store.submitDecisionAction("allow_once")

    XCTAssertNotNil(store.activeDecisionSheet)
    XCTAssertEqual(client.submitCount, 1)
    XCTAssertEqual(client.lastCommand?.operation, "submit")
  }

  func testPreparedDecisionKeepsSheetForIdentityDriftedSubmitResult() async throws {
    let client = try DecisionStubClient(resultDecisionID: "decision-other")
    let store = LocalProductStore(client: client)

    await store.openPreparedDecision(client.command(operation: "read", action: "read"))
    await store.submitDecisionAction("allow_once")

    XCTAssertNotNil(store.activeDecisionSheet)
    XCTAssertEqual(client.submitCount, 1)
    XCTAssertEqual(client.lastCommand?.decisionID, "decision-1")
  }

  func testUnpreparedReviewGateIsReadOnlyAndNeverCallsDecisionClient() async throws {
    let client = try DecisionStubClient()
    let store = LocalProductStore(client: client)
    let mission = LocalProductMissionSummary(
      missionID: "mission/team-review-missing",
      teamInstanceID: "team-review-missing",
      title: "Review missing Evidence",
      sourceKind: "team_execution",
      lane: "Review",
      status: "ready_for_review",
      priority: "normal",
      planDigest: String(repeating: "a", count: 64),
      simple: true,
      nodeCount: 1,
      completedNodeCount: 0,
      activeNodeCount: 0,
      reviewNodeCount: 1,
      attentionCount: 1,
      currentNodeID: "main",
      lastMilestone: "Review required",
      teamPulse: [],
      topology: []
    )

    store.openReadOnlyReviewDecision(for: mission)
    XCTAssertEqual(store.activeDecisionSheet?.kind, .review)
    XCTAssertEqual(store.activeDecisionSheet?.prepared, false)
    XCTAssertEqual(store.activeDecisionSheet?.preparedActions, [])

    await store.submitDecisionAction("accept_result")
    XCTAssertNotNil(store.activeDecisionSheet)
    XCTAssertEqual(client.readCount, 0)
    XCTAssertEqual(client.submitCount, 0)

    await store.submitDecisionAction("not_now")
    XCTAssertNil(store.activeDecisionSheet)
    XCTAssertEqual(client.readCount, 0)
    XCTAssertEqual(client.submitCount, 0)
  }

  func testMissionPreflightAndStartUseVisibleConfirmedTeamWithoutTypedIDs() async throws {
    let json = LocalProductModelsTests.snapshotJSON.replacingOccurrences(
      of: "\"teams\":[]",
      with: """
        "teams":[{
          "team_instance_id":"team-1",
          "display_name":"Coding Team",
          "source_kind":"saved_team",
          "state":"created",
          "confirmed":true,
          "executable":true,
          "read_only":false
        }]
        """
    ).replacingOccurrences(
      of: "\"view_version\":\"view-1\"",
      with: "\"view_version\":\"\(String(repeating: "b", count: 64))\""
    )
    let snapshot = try LocalProductWire.decodeSnapshot(Data(json.utf8))
    let client = ExecutionStubClient(snapshot: snapshot)
    let store = LocalProductStore(client: client)
    await store.refresh()

    let freshViewVersion = String(repeating: "f", count: 64)
    let freshJSON = json.replacingOccurrences(
      of: "\"view_version\":\"\(String(repeating: "b", count: 64))\"",
      with: "\"view_version\":\"\(freshViewVersion)\""
    )
    let freshSnapshot = try LocalProductWire.decodeSnapshot(Data(freshJSON.utf8))
    client.replaceSnapshot(freshSnapshot)
    client.replaceSnapshotAfterPreflight(
      LocalProductSnapshot(
        schemaVersion: 2,
        viewVersion: freshSnapshot.viewVersion,
        partial: freshSnapshot.partial,
        stale: freshSnapshot.stale,
        reason: freshSnapshot.reason,
        health: freshSnapshot.health,
        runtimes: freshSnapshot.runtimes,
        teams: freshSnapshot.teams,
        missions: freshSnapshot.missions,
        runs: freshSnapshot.runs,
        evidence: freshSnapshot.evidence,
        attention: freshSnapshot.attention,
        preparedDecisions: [
          LocalProductDecisionCommand(
            operation: "read", kind: .fallback, action: "read",
            missionID: "mission/team-1", teamInstanceID: "team-1",
            viewVersion: freshViewVersion,
            decisionID: "fallback-decision-1",
            decisionDigest: String(repeating: "e", count: 64),
            logicalNodeID: "main", attemptNumber: 2, claimGeneration: 0,
            correlationID: "00000000-0000-4000-8000-000000000000"
          )
        ],
        sideTasks: freshSnapshot.sideTasks,
        runtimePage: freshSnapshot.runtimePage,
        teamPage: freshSnapshot.teamPage,
        missionPage: freshSnapshot.missionPage,
        runPage: freshSnapshot.runPage,
        evidencePage: freshSnapshot.evidencePage
      )
    )

    await store.preflightMission(
      objective: "Implement the bounded change",
      team: snapshot.teams[0],
      workPackage: LocalProductWorkPackageOption.coding,
      confirmedConstraints: ["Do not change public APIs"],
      acceptedDecisions: ["Use the existing execution adapter"]
    )

    XCTAssertEqual(store.executionState, .ready)
    XCTAssertEqual(store.executionPreflight?.teamInstanceID, "team-1")
    XCTAssertEqual(client.commands.map(\.operation), ["preflight"])
    XCTAssertEqual(client.commands[0].missionID, "mission/team-1")
    XCTAssertEqual(client.commands[0].workPackageID, "work-package.coding")
    XCTAssertEqual(client.commands[0].contextVersion, 1)
    XCTAssertEqual(client.commands[0].confirmedConstraints, ["Do not change public APIs"])
    XCTAssertEqual(
      client.commands[0].acceptedDecisions,
      ["Use the existing execution adapter"]
    )
    XCTAssertEqual(client.commands[0].expectedViewVersion, freshViewVersion)
    XCTAssertEqual(
      store.preparedDecisionCommand(for: "mission/team-1", kind: .fallback)?.decisionID,
      "fallback-decision-1"
    )
    XCTAssertEqual(client.snapshotRequestCount, 3)

    await store.startPreflightedMission()

    XCTAssertEqual(store.executionState, .running)
    XCTAssertNil(store.executionPreflight)
    XCTAssertEqual(client.commands.map(\.operation), ["preflight", "start"])
    XCTAssertEqual(client.commands[1].preflightDigest, String(repeating: "d", count: 64))
    XCTAssertEqual(
      client.commands[1].confirmedConstraints,
      client.commands[0].confirmedConstraints
    )
    XCTAssertEqual(client.commands[1].acceptedDecisions, client.commands[0].acceptedDecisions)
    XCTAssertEqual(client.timelineRequestCount, 1)
    XCTAssertEqual(store.timelineState, .loaded)
    XCTAssertEqual(store.timeline?.records.first?.payload.textDelta, "Compiling checks")
    XCTAssertEqual(
      store.workbench.route,
      .mission(try XCTUnwrap(store.executionResult?.missionID))
    )
    XCTAssertTrue(store.missionCancelAvailable)

    await store.startPreflightedMission()

    XCTAssertEqual(store.executionState, .running)
    XCTAssertEqual(client.commands.map(\.operation), ["preflight", "start"])

    await store.cancelCurrentMission()

    XCTAssertEqual(store.executionState, .cancelled)
    XCTAssertEqual(client.commands.map(\.operation), ["preflight", "start", "control"])
    XCTAssertEqual(client.commands[2].controlAction, "cancel")
    XCTAssertEqual(client.commands[2].contextVersion, 0)
    XCTAssertTrue(client.commands[2].confirmedConstraints.isEmpty)
    XCTAssertTrue(client.commands[2].acceptedDecisions.isEmpty)
    XCTAssertEqual(client.commands[2].logicalNodeID, "main")
    XCTAssertEqual(client.commands[2].attemptNumber, 1)
    XCTAssertEqual(client.commands[2].claimGeneration, 3)
    XCTAssertEqual(client.commands[2].expectedViewVersion, store.snapshot?.viewVersion)
  }

  func testMissionStartReachesSucceededAndOpensMissionRoom() async throws {
    let json = LocalProductModelsTests.snapshotJSON.replacingOccurrences(
      of: "\"teams\":[]",
      with: """
        "teams":[{
          "team_instance_id":"team-1",
          "display_name":"Coding Team",
          "source_kind":"saved_team",
          "state":"created",
          "confirmed":true,
          "executable":true,
          "read_only":false
        }]
        """
    ).replacingOccurrences(
      of: "\"view_version\":\"view-1\"",
      with: "\"view_version\":\"\(String(repeating: "b", count: 64))\""
    )
    let snapshot = try LocalProductWire.decodeSnapshot(Data(json.utf8))
    let client = ExecutionStubClient(
      snapshot: snapshot,
      startResultStatus: "succeeded"
    )
    let store = LocalProductStore(client: client)
    await store.refresh()

    await store.preflightMission(
      objective: "Implement the bounded change",
      team: snapshot.teams[0],
      workPackage: LocalProductWorkPackageOption.coding
    )
    XCTAssertEqual(store.executionState, .ready)

    await store.startPreflightedMission()

    XCTAssertEqual(store.executionState, .succeeded)
    XCTAssertNil(store.executionPreflight)
    XCTAssertEqual(client.commands.map(\.operation), ["preflight", "start"])
    let missionID = try XCTUnwrap(store.executionResult?.missionID)
    XCTAssertEqual(store.workbench.route, .mission(missionID))
  }

  func testMissionStartSurfacesRemoteExecutionErrorCodeNotGenericUnavailable()
    async throws
  {
    let snapshot = try executableMissionSnapshot()
    let remoteError = LocalIPCRemoteError(
      code: .busy,
      recoverable: true,
      stage: .daemonAdmission,
      incidentID: "incident-mission-busy-1",
      safeMessage: "UI Mission Team already has a running Mission."
    )
    let client = ExecutionStubClient(
      snapshot: snapshot,
      startError: remoteError
    )
    let store = LocalProductStore(client: client)
    await store.refresh()

    await store.preflightMission(
      objective: "Implement the bounded change",
      team: snapshot.teams[0],
      workPackage: .coding
    )
    XCTAssertEqual(store.executionState, .ready)

    await store.startPreflightedMission()

    // The daemon's real error code must survive the client boundary instead
    // of collapsing into the generic "unavailable".
    XCTAssertEqual(store.executionState, .failed(reason: "busy"))
    XCTAssertEqual(client.commands.map(\.operation), ["preflight", "start"])
  }

  func testMissionStartSurfacesTransportFailureAsUnavailableWhenUnknown()
    async throws
  {
    let snapshot = try executableMissionSnapshot()
    struct UnknownError: Error {}
    let client = ExecutionStubClient(
      snapshot: snapshot,
      startError: UnknownError()
    )
    let store = LocalProductStore(client: client)
    await store.refresh()

    await store.preflightMission(
      objective: "Implement the bounded change",
      team: snapshot.teams[0],
      workPackage: .coding
    )
    XCTAssertEqual(store.executionState, .ready)

    await store.startPreflightedMission()

    XCTAssertEqual(store.executionState, .failed(reason: "unavailable"))
  }

  func testMissionContextEditInvalidatesReadyPreflightBeforeStart() async throws {
    let snapshot = try executableMissionSnapshot()
    let client = ExecutionStubClient(snapshot: snapshot)
    let store = LocalProductStore(client: client)
    await store.refresh()

    await store.preflightMission(
      objective: "Implement the bounded change",
      team: snapshot.teams[0],
      workPackage: .coding,
      confirmedConstraints: ["Do not change public APIs"],
      acceptedDecisions: ["Use the existing execution adapter"]
    )
    XCTAssertEqual(store.executionState, .ready)
    XCTAssertNotNil(store.executionPreflight)

    store.invalidateMissionPreflight()

    XCTAssertEqual(store.executionState, .idle)
    XCTAssertNil(store.executionPreflight)
    await store.startPreflightedMission()
    XCTAssertEqual(store.executionState, .failed(reason: "preflight_required"))
    XCTAssertEqual(client.commands.map(\.operation), ["preflight"])
  }

  func testMissionContextEditFencesLatePreflightResult() async throws {
    let snapshot = try executableMissionSnapshot()
    let client = ExecutionStubClient(snapshot: snapshot, suspendPreflight: true)
    let store = LocalProductStore(client: client)
    await store.refresh()

    let request = Task { @MainActor in
      await store.preflightMission(
        objective: "Implement the bounded change",
        team: snapshot.teams[0],
        workPackage: .coding,
        confirmedConstraints: ["Do not change public APIs"],
        acceptedDecisions: ["Use the existing execution adapter"]
      )
    }
    await client.waitForPreflightStart()

    store.invalidateMissionPreflight()
    client.releasePreflight()
    await request.value

    XCTAssertEqual(store.executionState, .idle)
    XCTAssertNil(store.executionPreflight)
    XCTAssertEqual(client.commands.map(\.operation), ["preflight"])
  }

  func testMissionPreflightRejectsDuplicateAuthorityAcrossContextClasses() async throws {
    let snapshot = try executableMissionSnapshot()
    let client = ExecutionStubClient(snapshot: snapshot)
    let store = LocalProductStore(client: client)
    await store.refresh()

    await store.preflightMission(
      objective: "Implement the bounded change",
      team: snapshot.teams[0],
      workPackage: .coding,
      confirmedConstraints: ["Keep the public API stable"],
      acceptedDecisions: ["Keep the public API stable"]
    )

    XCTAssertEqual(store.executionState, .failed(reason: "preflight_unavailable"))
    XCTAssertNil(store.executionPreflight)
    XCTAssertTrue(client.commands.isEmpty)
  }

  func testMissionPreflightKeepsBlockedAgentVisibleAndRefusesStart() async throws {
    let json = LocalProductModelsTests.snapshotJSON.replacingOccurrences(
      of: "\"teams\":[]",
      with: """
        "teams":[{
          "team_instance_id":"team-1","display_name":"Coding Team",
          "source_kind":"saved_team","state":"created",
          "confirmed":true,"executable":true,"read_only":false
        }]
        """
    )
    let snapshot = try LocalProductWire.decodeSnapshot(Data(json.utf8))
    let client = ExecutionStubClient(snapshot: snapshot, preflightNodeStatus: "blocked")
    let store = LocalProductStore(client: client)
    await store.refresh()

    await store.preflightMission(
      objective: "Implement the bounded change",
      team: snapshot.teams[0],
      workPackage: .coding
    )

    XCTAssertEqual(store.executionState, .failed(reason: "agent_preflight_blocked"))
    XCTAssertEqual(store.executionPreflight?.nodes.first?.status, "blocked")
    XCTAssertFalse(store.executionPreflight?.nodes.first?.blockReason.isEmpty ?? true)

    await store.startPreflightedMission()

    XCTAssertEqual(client.commands.map(\.operation), ["preflight"])
    XCTAssertEqual(store.executionState, .failed(reason: "preflight_required"))
  }

  func testMissionPreflightStartsHealthySiblingWhilePeerIsBlocked() async throws {
    let json = LocalProductModelsTests.snapshotJSON.replacingOccurrences(
      of: "\"teams\":[]",
      with: """
        "teams":[{
          "team_instance_id":"team-1","display_name":"Coding Team",
          "source_kind":"saved_team","state":"created",
          "confirmed":true,"executable":true,"read_only":false
        }]
        """
    )
    let snapshot = try LocalProductWire.decodeSnapshot(Data(json.utf8))
    let client = ExecutionStubClient(
      snapshot: snapshot,
      preflightNodeStatus: "blocked",
      additionalPreflightNodeStatus: "ready"
    )
    let store = LocalProductStore(client: client)
    await store.refresh()

    await store.preflightMission(
      objective: "Implement independent bounded changes",
      team: snapshot.teams[0],
      workPackage: .coding
    )

    XCTAssertEqual(store.executionState, .ready)
    XCTAssertEqual(store.executionPreflight?.nodes.map(\.status), ["blocked", "ready"])

    await store.startPreflightedMission()

    XCTAssertEqual(client.commands.map(\.operation), ["preflight", "start"])
    XCTAssertEqual(store.executionState, .running)
  }

  func testMissionPreflightExpiresBeforeStartWhenAuthorityViewChanges() async throws {
    let baseJSON = LocalProductModelsTests.snapshotJSON.replacingOccurrences(
      of: "\"teams\":[]",
      with: """
        "teams":[{
          "team_instance_id":"team-1","display_name":"Coding Team",
          "source_kind":"saved_team","state":"created",
          "confirmed":true,"executable":true,"read_only":false
        }]
        """
    )
    let firstView = String(repeating: "a", count: 64)
    let readyView = String(repeating: "b", count: 64)
    let newerView = String(repeating: "c", count: 64)
    let first = try LocalProductWire.decodeSnapshot(
      Data(
        baseJSON.replacingOccurrences(
          of: "\"view_version\":\"view-1\"",
          with: "\"view_version\":\"\(firstView)\""
        ).utf8
      )
    )
    let client = ExecutionStubClient(snapshot: first)
    let store = LocalProductStore(client: client)
    await store.refresh()

    client.replaceSnapshot(
      try LocalProductWire.decodeSnapshot(
        Data(
          baseJSON.replacingOccurrences(
            of: "\"view_version\":\"view-1\"",
            with: "\"view_version\":\"\(readyView)\""
          ).utf8
        )
      )
    )
    await store.preflightMission(
      objective: "Implement the bounded change",
      team: first.teams[0],
      workPackage: .coding
    )
    XCTAssertEqual(store.executionState, .ready)
    XCTAssertEqual(store.executionPreflight?.viewVersion, readyView)

    client.replaceSnapshot(
      try LocalProductWire.decodeSnapshot(
        Data(
          baseJSON.replacingOccurrences(
            of: "\"view_version\":\"view-1\"",
            with: "\"view_version\":\"\(newerView)\""
          ).utf8
        )
      )
    )
    await store.refresh()

    XCTAssertNil(store.executionPreflight)
    XCTAssertEqual(store.executionState, .failed(reason: "preflight_expired"))
    await store.startPreflightedMission()
    XCTAssertEqual(client.commands.map(\.operation), ["preflight"])
  }

  func testPlainChatMessageDoesNotCreateTeamOrMission() async {
    let client = ChatRecordingClient()
    let store = LocalProductStore(client: client)

    await store.sendChatMessage("Plan a vacation")

    XCTAssertEqual(client.recordedThread?.messages.count, 2)
    XCTAssertEqual(client.recordedThread?.messages.first?.role, "user")
    XCTAssertEqual(client.recordedThread?.messages.last?.role, "loom")
    XCTAssertNil(store.builderSession)
    XCTAssertNil(store.snapshot)
    XCTAssertFalse(client.setupStarted)
  }

  func testChatMessageAppearsWhileConversationRuntimeIsResponding() async {
    let client = ChatRecordingClient(blockSend: true)
    let store = LocalProductStore(client: client)

    let sending = Task { await store.sendChatMessage("hello") }
    await client.waitForSendStart()

    XCTAssertTrue(store.isSendingChatMessage)
    XCTAssertEqual(store.chatThread?.messages.last?.role, "user")
    XCTAssertEqual(store.chatThread?.messages.last?.content, "hello")
    XCTAssertFalse(store.chatThread?.canReply ?? true)

    client.releaseSend()
    await sending.value

    XCTAssertFalse(store.isSendingChatMessage)
    XCTAssertEqual(store.chatThread?.messages.last?.role, "loom")
  }

  func testExplicitAgentTriggerRequiresConfirmationAndDoesNotAutoCreateTeam() async {
    let client = ChatRecordingClient()
    let store = LocalProductStore(client: client)

    await store.startBlankBuilder()

    XCTAssertNil(store.builderSession)
    XCTAssertNil(store.lastConfirmation)
    XCTAssertNil(store.snapshot)
    XCTAssertFalse(client.setupStarted)
  }

  func testConversationProfileSwitchKeepsVisibleThreadAndCreatesRouteSegment() async throws {
    let client = ChatRecordingClient()
    let setup = try conversationProfileSetupSnapshot()
    let store = LocalProductStore(
      client: client,
      initialSetupSnapshot: setup
    )
    store.selectConversationProfile(
      "conversation-deepseek-deepseek-chat-r2"
    )
    let firstThreadID = store.currentChatThreadID()
    await store.sendChatMessage("hello")

    XCTAssertEqual(
      client.recordedProfileID,
      "conversation-deepseek-deepseek-chat-r2"
    )
    XCTAssertEqual(store.chatThread?.profileID, client.recordedProfileID)

    store.selectConversationProfile(
      "conversation-openai-codex-default-v1"
    )
    store.conversationContextMode = .summaryOnly
    await store.sendChatMessage("continue with Codex")

    XCTAssertEqual(store.currentChatThreadID(), firstThreadID)
    XCTAssertEqual(client.recordedThreadID, firstThreadID)
    XCTAssertEqual(client.recordedContextMode, .summaryOnly)
    XCTAssertEqual(store.chatThread?.segments.count, 2)
    XCTAssertEqual(
      store.chatThread?.segments.last?.profileID, "conversation-openai-codex-default-v1")

    store.selectConversationProfile(
      "conversation-anthropic-claude-sonnet-5-account-work-r5"
    )
    store.conversationContextMode = .summaryOnly
    await store.sendChatMessage("continue with Anthropic")

    XCTAssertEqual(store.currentChatThreadID(), firstThreadID)
    XCTAssertEqual(client.recordedThreadID, firstThreadID)
    XCTAssertEqual(
      client.recordedProfileID,
      "conversation-anthropic-claude-sonnet-5-account-work-r5"
    )
    XCTAssertEqual(store.chatThread?.segments.count, 3)
    XCTAssertEqual(
      store.chatThread?.segments.last?.profileID,
      "conversation-anthropic-claude-sonnet-5-account-work-r5"
    )
  }

  func testMultipleConversationSessionsSwitchAndAutoTitle() async throws {
    let client = ChatRecordingClient()
    let store = LocalProductStore(
      client: client,
      initialSetupSnapshot: try conversationProfileSetupSnapshot(),
      chatSessionsFileURL: FileManager.default.temporaryDirectory
        .appendingPathComponent("loom-sessions-autoTitle-\(UUID().uuidString).json")
    )
    store.selectConversationProfile("conversation-deepseek-deepseek-chat-r2")

    let firstThreadID = store.currentChatThreadID()
    XCTAssertEqual(store.chatSessions.count, 1)
    XCTAssertEqual(store.selectedChatSessionID, firstThreadID)

    // A new conversation gets a distinct thread and becomes active.
    store.newConversation()
    let secondThreadID = store.currentChatThreadID()
    XCTAssertNotEqual(secondThreadID, firstThreadID)
    XCTAssertEqual(store.chatSessions.count, 2)
    XCTAssertEqual(store.selectedChatSessionID, secondThreadID)

    // Sending in the new conversation records it against the new thread and
    // auto-titles the session from the first user message.
    await store.sendChatMessage("Fix the flaky login test")
    XCTAssertEqual(client.recordedThreadID, secondThreadID)
    let secondTitle = try XCTUnwrap(
      store.chatSessions.first { $0.threadID == secondThreadID }?.title
    )
    XCTAssertEqual(secondTitle, "Fix the flaky login test")

    // Switching back restores the first conversation.
    store.selectChatSession(firstThreadID)
    XCTAssertEqual(store.currentChatThreadID(), firstThreadID)
    XCTAssertEqual(store.selectedChatSessionID, firstThreadID)
    XCTAssertEqual(store.chatSessions.count, 2)
  }

  func testNewConversationStartsWithEmptyThread() async throws {
    let client = ChatRecordingClient()
    let store = LocalProductStore(
      client: client,
      initialSetupSnapshot: try conversationProfileSetupSnapshot(),
      chatSessionsFileURL: FileManager.default.temporaryDirectory
        .appendingPathComponent("loom-sessions-newEmpty-\(UUID().uuidString).json")
    )
    let firstThreadID = store.currentChatThreadID()
    store.newConversation()
    let secondThreadID = store.currentChatThreadID()
    XCTAssertNotEqual(secondThreadID, firstThreadID)
    XCTAssertEqual(store.selectedChatSessionID, secondThreadID)
    // The new conversation is distinct and not yet materialized.
    XCTAssertTrue(
      store.chatThread?.messages.isEmpty ?? true
    )
  }

  func testConversationRouteTransitionRequiresConfirmationAfterHistory() async throws {
    let client = ChatRecordingClient()
    let store = LocalProductStore(
      client: client,
      initialSetupSnapshot: try conversationProfileSetupSnapshot()
    )
    store.selectConversationProfile("conversation-deepseek-deepseek-chat-r2")
    await store.sendChatMessage("hello")

    let transition = try XCTUnwrap(
      store.requestConversationProfileSelection(
        "conversation-openai-codex-default-v1"
      )
    )

    XCTAssertEqual(
      store.selectedConversationProfileID,
      "conversation-deepseek-deepseek-chat-r2"
    )
    XCTAssertEqual(
      transition.source.profileID,
      "conversation-deepseek-deepseek-chat-r2"
    )
    XCTAssertEqual(
      transition.target.profileID,
      "conversation-openai-codex-default-v1"
    )

    XCTAssertTrue(
      store.confirmConversationRouteTransition(
        transition,
        contextMode: .summaryOnly
      )
    )
    // Regression: after confirming a profile switch, sending must NOT
    // re-trigger the Change-conversation-route sheet.
    XCTAssertNil(store.requestConversationDispatchTransition())
    await store.sendChatMessage("continue with Codex")

    XCTAssertEqual(client.recordedContextMode, .summaryOnly)
    XCTAssertEqual(
      client.recordedProfileID,
      "conversation-openai-codex-default-v1"
    )
  }

  func testConversationRouteTransitionFailsClosedAfterSelectionDrift() async throws {
    let client = ChatRecordingClient()
    let store = LocalProductStore(
      client: client,
      initialSetupSnapshot: try conversationProfileSetupSnapshot()
    )
    store.selectConversationProfile("conversation-deepseek-deepseek-chat-r2")
    await store.sendChatMessage("hello")
    let transition = try XCTUnwrap(
      store.requestConversationProfileSelection(
        "conversation-openai-codex-default-v1"
      )
    )

    store.selectConversationProfile(
      "conversation-anthropic-claude-sonnet-5-account-work-r5"
    )

    XCTAssertFalse(
      store.confirmConversationRouteTransition(
        transition,
        contextMode: .continueWithContext
      )
    )
    XCTAssertEqual(
      store.selectedConversationProfileID,
      "conversation-anthropic-claude-sonnet-5-account-work-r5"
    )
  }

  func testConversationPolicyDriftRequiresConfirmedNewSegmentAfterRestart() async throws {
    let setup = try LocalProductSetupWire.decodeSnapshot(Data("""
      {"schema_version":1,"view_version":"view-policy-drift",
       "codex":{"provider_id":"codex","auth_mode":"native_auth","credential_reference":"","revision":0,"status":"available","reason":""},
       "minimax":{"provider_id":"minimax","auth_mode":"brokered","credential_reference":"","revision":0,"status":"unconfigured","reason":""},
       "providers":[],"provider_accounts":[],
       "conversation_profiles":[{
         "profile_id":"conversation-deepseek-deepseek-chat-r7",
         "harness_adapter":"loom-native","provider_id":"deepseek",
         "provider_account_id":"deepseek.work","display_name":"DeepSeek",
         "protocol":"openai_compatible","model_id":"deepseek-chat",
         "auth_mode":"brokered","credential_revision":7,
         "policy_version":2,"policy_revision":4,
         "policy_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
         "trust_domain":"enterprise_tenant",
         "retention_mode":"zero_data_retention","data_region":"apac"
       }],
       "runtimes":[],"saved_teams":[],"templates":[],"role_options":[],
       "skills":[],"permissions":[],"resources":[]}
      """.utf8))
    let currentBinding = LocalProductConversationExecutionBinding(
      providerID: "deepseek", providerAccountID: "deepseek.work",
      providerAccountPolicyVersion: 2,
      providerAccountPolicyRevision: 4,
      providerAccountPolicyDigest: String(repeating: "b", count: 64),
      trustDomain: "enterprise_tenant",
      retentionMode: "zero_data_retention", dataRegion: "apac"
    )
    let client = ChatRecordingClient(responseExecutionBinding: currentBinding)
    let store = LocalProductStore(client: client, initialSetupSnapshot: setup)
    let threadID = store.currentChatThreadID()
    client.replaceRecordedThread(LocalProductChatThread(
      threadID: threadID,
      profileID: "conversation-deepseek-deepseek-chat-r7",
      segments: [LocalProductConversationSegment(
        segmentID: "segment-1",
        profileID: "conversation-deepseek-deepseek-chat-r7",
        contextMode: .startClean,
        contextCapsuleDigest: String(repeating: "c", count: 64),
        executionBinding: LocalProductConversationExecutionBinding(
          providerID: "deepseek", providerAccountID: "deepseek.work",
          providerAccountPolicyVersion: 2,
          providerAccountPolicyRevision: 3,
          providerAccountPolicyDigest: String(repeating: "a", count: 64),
          trustDomain: "external_provider",
          retentionMode: "provider_default", dataRegion: "global"
        ),
        bindingDigest: String(repeating: "d", count: 64)
      )],
      messages: [LocalProductChatMessage(
        messageID: "msg-1", segmentID: "segment-1", role: "user",
        content: "existing", tentative: false
      )],
      canReply: true,
      requiresConfirmation: false
    ))
    await store.loadChatThread()

    let transition = try XCTUnwrap(store.requestConversationDispatchTransition())
    XCTAssertTrue(transition.rebindsCurrentRoute)
    XCTAssertEqual(
      transition.sourceExecutionBinding?.providerAccountPolicyRevision, 3
    )
    XCTAssertEqual(transition.target.policyRevision, 4)
    XCTAssertTrue(store.confirmConversationRouteTransition(
      transition,
      contextMode: .summaryOnly
    ))

    await store.sendChatMessage("continue under the new policy")
    XCTAssertEqual(client.recordedContextMode, .summaryOnly)
    XCTAssertEqual(client.recordedExpectedExecutionBinding, currentBinding)
    XCTAssertEqual(client.recordedThreadID, threadID)
    XCTAssertNil(store.requestConversationDispatchTransition())
  }

  func testBlankConversationProfileSelectionDoesNotRequireTransitionReview() throws {
    let store = LocalProductStore(
      client: ChatRecordingClient(),
      initialSetupSnapshot: try conversationProfileSetupSnapshot()
    )

    XCTAssertNil(
      store.requestConversationProfileSelection(
        "conversation-deepseek-deepseek-chat-r2"
      )
    )
    XCTAssertEqual(
      store.selectedConversationProfileID,
      "conversation-deepseek-deepseek-chat-r2"
    )
  }

  func testConversationProfileSwitchKeepsUnloadedPersistedThreadAnchor() async throws {
    let setup = try conversationProfileSetupSnapshot()
    let client = ChatRecordingClient()
    let store = LocalProductStore(client: client, initialSetupSnapshot: setup)
    let persistedThreadID = store.currentChatThreadID()
    client.replaceRecordedThread(
      LocalProductChatThread(
        threadID: persistedThreadID,
        profileID: "conversation-openai-codex-default-v1",
        messages: [
          LocalProductChatMessage(
            messageID: "old-1",
            role: "user",
            content: "old message",
            tentative: false
          )
        ],
        canReply: true,
        requiresConfirmation: false
      )
    )

    store.selectConversationProfile(
      "conversation-deepseek-deepseek-chat-r2"
    )
    let deepSeekThreadID = store.currentChatThreadID()
    await store.sendChatMessage("hello DeepSeek")

    XCTAssertEqual(deepSeekThreadID, persistedThreadID)
    XCTAssertEqual(client.recordedThreadID, deepSeekThreadID)
    XCTAssertEqual(client.recordedContextMode, .summaryOnly)
    XCTAssertEqual(
      client.recordedProfileID,
      "conversation-deepseek-deepseek-chat-r2"
    )
  }

  func testStaleCodexThreadLoadCannotOverwriteUserDeepSeekSelection() async throws {
    let setup = try conversationProfileSetupSnapshot()
    let client = ChatRecordingClient(blockLoad: true)
    let store = LocalProductStore(client: client, initialSetupSnapshot: setup)
    let persistedThreadID = store.currentChatThreadID()
    client.replaceRecordedThread(
      LocalProductChatThread(
        threadID: persistedThreadID,
        profileID: "conversation-openai-codex-default-v1",
        messages: [
          LocalProductChatMessage(
            messageID: "old-1",
            role: "user",
            content: "old message",
            tentative: false
          )
        ],
        canReply: true,
        requiresConfirmation: false
      )
    )
    let loading = Task { await store.loadChatThread() }
    await client.waitForLoadStart()

    store.selectConversationProfile(
      "conversation-deepseek-deepseek-chat-r2"
    )
    let deepSeekThreadID = store.currentChatThreadID()
    client.releaseLoad()
    await loading.value

    XCTAssertEqual(
      store.selectedConversationProfileID,
      "conversation-deepseek-deepseek-chat-r2"
    )
    XCTAssertEqual(store.currentChatThreadID(), deepSeekThreadID)
    XCTAssertNil(store.chatThread)
  }

  func testChatLoadProjectsMigrationFailureWithIncidentAndRecoveryAction() async {
    let incidentID = "loom-migration-11111111-1111-4111-8111-111111111111"
    let client = ChatRecordingClient(
      recordedThread: LocalProductChatThread(
        threadID: "unused",
        messages: [],
        canReply: false,
        requiresConfirmation: false,
        availabilityFailure: LocalProductChatAvailabilityFailure(
          code: .stateUnavailable,
          stage: .migrationCommit,
          incidentID: incidentID,
          retryable: false
        )
      )
    )
    let store = LocalProductStore(client: client)
    client.replaceRecordedThread(
      LocalProductChatThread(
        threadID: store.currentChatThreadID(),
        messages: [],
        canReply: false,
        requiresConfirmation: false,
        availabilityFailure: LocalProductChatAvailabilityFailure(
          code: .stateUnavailable,
          stage: .migrationCommit,
          incidentID: incidentID,
          retryable: false
        )
      )
    )

    await store.loadChatThread()

    XCTAssertEqual(store.chatOperationFailure?.code, .stateUnavailable)
    XCTAssertEqual(store.chatOperationFailure?.stage, .migrationCommit)
    XCTAssertEqual(store.chatOperationFailure?.incidentID, incidentID)
    XCTAssertFalse(store.chatOperationFailure?.recoverable ?? true)
    XCTAssertEqual(store.chatOperationFailure?.title, "Conversation migration incomplete")
    XCTAssertTrue(
      store.chatOperationFailure?.detail.contains("reopen Loom") ?? false
    )
  }

  func testConversationVaultEncryptFailureShowsActionableGuidance() async throws {
    let setup = try conversationProfileSetupSnapshot()
    let incidentID = "loom-chat-vault-encrypt-e2e"
    let client = ChatRecordingClient(
      recordedThread: LocalProductChatThread(
        threadID: "thread-vault-encrypt",
        messages: [],
        canReply: false,
        requiresConfirmation: false
      ),
      sendError: LocalIPCRemoteError(
        code: .stateUnavailable,
        recoverable: true,
        stage: .vaultEncrypt,
        incidentID: incidentID
      )
    )
    let store = LocalProductStore(client: client, initialSetupSnapshot: setup)
    client.replaceRecordedThread(LocalProductChatThread(
      threadID: store.currentChatThreadID(),
      messages: [],
      canReply: true,
      requiresConfirmation: false
    ))
    store.selectConversationProfile("conversation-deepseek-deepseek-chat-r2")

    await store.sendChatMessage("hello from vault encrypt test")

    XCTAssertEqual(store.chatOperationFailure?.code, .stateUnavailable)
    XCTAssertEqual(store.chatOperationFailure?.stage, .vaultEncrypt)
    XCTAssertEqual(store.chatOperationFailure?.incidentID, incidentID)
    XCTAssertEqual(
      store.chatOperationFailure?.title,
      "Conversation context could not be secured"
    )
    XCTAssertTrue(
      store.chatOperationFailure?.detail.contains("Credential Vault") ?? false
    )
    XCTAssertTrue(
      store.chatOperationFailure?.detail.contains("draft is preserved") ?? false
    )
  }

  func testCodexUsageLimitShowsActionableGuidance() async throws {
    let setup = try conversationProfileSetupSnapshot()
    let incidentID = "loom-chat-codex-usage-limit-e2e"
    let client = ChatRecordingClient(
      recordedThread: LocalProductChatThread(
        threadID: "thread-codex-limit",
        messages: [],
        canReply: false,
        requiresConfirmation: false
      ),
      sendError: LocalIPCRemoteError(
        code: .providerInsufficientBalance,
        recoverable: false,
        stage: .providerConnect,
        incidentID: incidentID
      )
    )
    let store = LocalProductStore(client: client, initialSetupSnapshot: setup)
    client.replaceRecordedThread(LocalProductChatThread(
      threadID: store.currentChatThreadID(),
      messages: [],
      canReply: true,
      requiresConfirmation: false
    ))
    store.selectConversationProfile("conversation-deepseek-deepseek-chat-r2")

    await store.sendChatMessage("hello from codex usage test")

    XCTAssertEqual(store.chatOperationFailure?.code, .providerInsufficientBalance)
    XCTAssertEqual(store.chatOperationFailure?.stage, .providerConnect)
    XCTAssertEqual(
      store.chatOperationFailure?.title,
      "Codex account usage limit reached"
    )
    XCTAssertTrue(
      store.chatOperationFailure?.detail.contains("DeepSeek V4") ?? false
    )
  }

  func testConversationModelProviderCredentialRequiredShowsActionableGuidance() async throws {
    let setup = try conversationProfileSetupSnapshot()
    let incidentID = "loom-chat-provider-auth-connect-e2e"
    let client = ChatRecordingClient(
      recordedThread: LocalProductChatThread(
        threadID: "thread-provider-auth",
        messages: [],
        canReply: false,
        requiresConfirmation: false
      ),
      sendError: LocalIPCRemoteError(
        code: .providerAuth,
        recoverable: false,
        stage: .providerConnect,
        incidentID: incidentID
      )
    )
    let store = LocalProductStore(client: client, initialSetupSnapshot: setup)
    client.replaceRecordedThread(LocalProductChatThread(
      threadID: store.currentChatThreadID(),
      messages: [],
      canReply: true,
      requiresConfirmation: false
    ))
    store.selectConversationProfile("conversation-deepseek-deepseek-chat-r2")

    await store.sendChatMessage("hello from provider auth test")

    XCTAssertEqual(store.chatOperationFailure?.code, .providerAuth)
    XCTAssertEqual(store.chatOperationFailure?.stage, .providerConnect)
    XCTAssertEqual(
      store.chatOperationFailure?.title,
      "Model Provider credential required"
    )
    XCTAssertTrue(
      store.chatOperationFailure?.detail.contains("no verified Loom account")
        ?? false
    )
  }

  func testConversationProviderConnectFailureShowsActionableGuidance() async throws {
    let setup = try conversationProfileSetupSnapshot()
    let incidentID = "loom-chat-provider-connect-e2e"
    let client = ChatRecordingClient(
      recordedThread: LocalProductChatThread(
        threadID: "thread-provider-connect",
        messages: [],
        canReply: false,
        requiresConfirmation: false
      ),
      sendError: LocalIPCRemoteError(
        code: .providerUnavailable,
        recoverable: true,
        stage: .providerConnect,
        incidentID: incidentID
      )
    )
    let store = LocalProductStore(client: client, initialSetupSnapshot: setup)
    client.replaceRecordedThread(LocalProductChatThread(
      threadID: store.currentChatThreadID(),
      messages: [],
      canReply: true,
      requiresConfirmation: false
    ))
    store.selectConversationProfile("conversation-deepseek-deepseek-chat-r2")

    await store.sendChatMessage("hello from provider connect test")

    XCTAssertEqual(store.chatOperationFailure?.code, .providerUnavailable)
    XCTAssertEqual(store.chatOperationFailure?.stage, .providerConnect)
    XCTAssertEqual(store.chatOperationFailure?.incidentID, incidentID)
    XCTAssertEqual(
      store.chatOperationFailure?.title,
      "Conversation Provider could not start"
    )
    XCTAssertTrue(
      store.chatOperationFailure?.detail.contains("Runtime & Providers") ?? false
    )
  }

  func testConversationProfileConflictIsVisibleAndPreservesDraft() async throws {
    let setup = try conversationProfileSetupSnapshot()
    let incidentID = "loom-chat-profile-conflict"
    let codexThread = LocalProductChatThread(
      threadID: "placeholder",
      profileID: "conversation-openai-codex-default-v1",
      segments: [LocalProductConversationSegment(
        segmentID: "segment-1",
        profileID: "conversation-openai-codex-default-v1",
        contextMode: .startClean,
        contextCapsuleDigest: String(repeating: "a", count: 64),
        bindingDigest: String(repeating: "b", count: 64)
      )],
      messages: [LocalProductChatMessage(
        messageID: "old-1", segmentID: "segment-1", role: "user",
        content: "old message", tentative: false
      )],
      canReply: true,
      requiresConfirmation: false
    )
    let client = ChatRecordingClient(
      recordedThread: codexThread,
      sendError: LocalIPCRemoteError(
        code: .conflict,
        recoverable: true,
        stage: .conversationDispatch,
        incidentID: incidentID
      )
    )
    let store = LocalProductStore(client: client, initialSetupSnapshot: setup)
    let threadID = store.currentChatThreadID()
    client.replaceRecordedThread(LocalProductChatThread(
      threadID: threadID,
      profileID: codexThread.profileID,
      segments: codexThread.segments,
      messages: codexThread.messages,
      canReply: true,
      requiresConfirmation: false
    ))
    store.selectConversationProfile(
      "conversation-deepseek-deepseek-chat-r2"
    )

    await store.sendChatMessage("retry this message")

    XCTAssertEqual(
      store.workspace.selectedContinuity.composerDraft,
      "retry this message"
    )
    XCTAssertEqual(store.chatOperationFailure?.code, .conflict)
    XCTAssertEqual(store.chatOperationFailure?.stage, .conversationDispatch)
    XCTAssertEqual(store.chatOperationFailure?.incidentID, incidentID)
    XCTAssertTrue(store.chatOperationFailure?.recoverable ?? false)
    XCTAssertEqual(store.currentChatThreadID(), threadID)
    XCTAssertEqual(
      store.selectedConversationProfileID,
      "conversation-deepseek-deepseek-chat-r2"
    )
    XCTAssertEqual(
      store.chatThread?.profileID,
      "conversation-openai-codex-default-v1"
    )

    let transition = try XCTUnwrap(store.requestConversationDispatchTransition())
    XCTAssertEqual(
      transition.source.profileID,
      "conversation-openai-codex-default-v1"
    )
    XCTAssertEqual(
      transition.target.profileID,
      "conversation-deepseek-deepseek-chat-r2"
    )
    XCTAssertTrue(store.confirmConversationRouteTransition(
      transition,
      contextMode: .summaryOnly
    ))
    XCTAssertEqual(store.currentChatThreadID(), threadID)
  }

  func testProviderAttemptFailureIsVisibleAndPreservesAuthoritativeThreadAndDraft() async throws {
    let setup = try conversationProfileSetupSnapshot()
    let client = ChatRecordingClient(
	  attemptFailureCode: "provider_insufficient_balance",
      attemptFailureStage: "provider_http",
	  attemptHTTPStatus: 402,
	  attemptProviderCode: "insufficient_balance",
	  attemptFailureMessage: "Provider account has insufficient balance.",
      attemptRetryable: false
    )
    let store = LocalProductStore(client: client, initialSetupSnapshot: setup)
    store.selectConversationProfile(
      "conversation-deepseek-deepseek-chat-r2"
    )

    await store.sendChatMessage("retry this provider request")

    XCTAssertEqual(store.chatThread?.attempts.last?.status, "failed")
    XCTAssertEqual(
      store.workspace.selectedContinuity.composerDraft,
      "retry this provider request"
    )
	XCTAssertEqual(store.chatOperationFailure?.code, .providerInsufficientBalance)
    XCTAssertEqual(store.chatOperationFailure?.stage, .providerHTTP)
	XCTAssertEqual(store.chatOperationFailure?.httpStatus, 402)
	XCTAssertEqual(store.chatOperationFailure?.providerCode, "insufficient_balance")
	XCTAssertEqual(
	  store.chatOperationFailure?.detail,
	  "Provider account has insufficient balance. HTTP 402 · insufficient_balance"
	)
    XCTAssertFalse(store.chatOperationFailure?.recoverable ?? true)
    XCTAssertEqual(
      store.chatOperationFailure?.incidentID,
      store.chatThread?.attempts.last?.incidentID
    )
  }

  func testProviderRouteFailuresOfferSwitchProviderRecovery() {
    let failure = LocalProductChatOperationFailure(
      code: .providerInsufficientBalance,
      stage: .providerConnect,
      recoverable: false,
      incidentID: "loom-chat-recovery-test",
      title: "Provider balance required",
      detail: "Add funds to the selected Provider Account, then retry."
    )
    XCTAssertTrue(failure.isRouteRecoveryAvailable)

    let auth = LocalProductChatOperationFailure(
      code: .providerAuth,
      stage: .providerConnect,
      recoverable: false,
      incidentID: "loom-chat-recovery-auth",
      title: "Provider authentication failed",
      detail: "Reconnect the selected Provider Account."
    )
    XCTAssertTrue(auth.isRouteRecoveryAvailable)

    let model = LocalProductChatOperationFailure(
      code: .providerModelUnavailable,
      stage: .providerHTTP,
      recoverable: false,
      incidentID: "loom-chat-recovery-model",
      title: "Model unavailable",
      detail: "Select a model available to this Provider Account."
    )
    XCTAssertTrue(model.isRouteRecoveryAvailable)

    // A rate limit is recoverable in place and must NOT suggest switching.
    let rate = LocalProductChatOperationFailure(
      code: .providerRateLimit,
      stage: .providerHTTP,
      recoverable: true,
      incidentID: "loom-chat-recovery-rate",
      title: "Provider rate limit reached",
      detail: "Retry after the limit resets."
    )
    XCTAssertFalse(rate.isRouteRecoveryAvailable)

    // A transport failure is recoverable in place and must NOT suggest switching.
    let transport = LocalProductChatOperationFailure(
      code: .stateUnavailable,
      stage: .udsTransport,
      recoverable: true,
      incidentID: "loom-chat-recovery-transport",
      title: "Local service unavailable",
      detail: "Reopen Loom or retry."
    )
    XCTAssertFalse(transport.isRouteRecoveryAvailable)
  }

  func testBuilderConfirmationBlockedMessageExplainsActionableRecovery() throws {
    let preview = try builderPreviewFixture(compatibilityGaps: [])
    // Uncommitted edits: tell the user to press Return.
    XCTAssertTrue(
      preview.confirmationBlockedMessage(
        uncommittedName: "Draft Name",
        uncommittedPurpose: ""
      ).contains("Press Return")
    )
    // Committed fields with compatibility gaps: tell the user to resolve them.
    let gappy = try builderPreviewFixture(
      compatibilityGaps: ["unverified credential"]
    )
    XCTAssertTrue(
      gappy.confirmationBlockedMessage(
        uncommittedName: "",
        uncommittedPurpose: ""
      ).contains("compatibility issue")
    )
    // No edits and no gaps: generic required-fields hint.
    XCTAssertTrue(
      preview.confirmationBlockedMessage(
        uncommittedName: "",
        uncommittedPurpose: ""
      ).contains("required fields")
    )
  }

  private func builderPreviewFixture(
    compatibilityGaps: [String]
  ) throws -> LocalProductBuilderPreview {
    let data = try JSONSerialization.data(withJSONObject: [
      "name": "Current Team",
      "purpose": "Current purpose",
      "roles": [],
      "permissions": [],
      "resources": [],
      "compatibility_gaps": compatibilityGaps,
      "requested_concurrency": 1,
      "maximum_budget_credits": 100,
      "estimated_maximum_cost": "up to 100 credits",
    ])
    return try JSONDecoder().decode(
      LocalProductBuilderPreview.self,
      from: data
    )
  }

  private final class ChatRecordingClient: LocalProductClientProtocol {
    private(set) var recordedThread: LocalProductChatThread?
    private(set) var recordedProfileID = ""
    private(set) var recordedThreadID = ""
    private(set) var recordedContextMode: LocalProductConversationContextMode?
    private(set) var recordedExpectedExecutionBinding:
      LocalProductConversationExecutionBinding?
    private(set) var setupStarted = false
    private let blockSend: Bool
    private let blockLoad: Bool
    private let sendError: Error?
    var failNextSend: Error?
    private let attemptFailureCode: String?
    private let attemptFailureStage: String?
	private let attemptHTTPStatus: Int
	private let attemptProviderCode: String
	private let attemptFailureMessage: String
	private let attemptRetryAfterSeconds: Int64
    private let attemptRetryable: Bool
    private let responseExecutionBinding: LocalProductConversationExecutionBinding?
    private var sendStarted = false
    private var loadStarted = false
    private var sendContinuation: CheckedContinuation<Void, Never>?
    private var loadContinuation: CheckedContinuation<Void, Never>?

    init(
      blockSend: Bool = false,
      blockLoad: Bool = false,
      recordedThread: LocalProductChatThread? = nil,
      sendError: Error? = nil,
      attemptFailureCode: String? = nil,
      attemptFailureStage: String? = nil,
	  attemptHTTPStatus: Int = 0,
	  attemptProviderCode: String = "",
	  attemptFailureMessage: String = "",
      attemptRetryAfterSeconds: Int64 = 0,
      attemptRetryable: Bool = false,
      responseExecutionBinding: LocalProductConversationExecutionBinding? = nil
    ) {
      self.blockSend = blockSend
      self.blockLoad = blockLoad
      self.recordedThread = recordedThread
      self.sendError = sendError
      self.attemptFailureCode = attemptFailureCode
      self.attemptFailureStage = attemptFailureStage
	  self.attemptHTTPStatus = attemptHTTPStatus
	  self.attemptProviderCode = attemptProviderCode
	  self.attemptFailureMessage = attemptFailureMessage
	  self.attemptRetryAfterSeconds = attemptRetryAfterSeconds
      self.attemptRetryable = attemptRetryable
      self.responseExecutionBinding = responseExecutionBinding
    }

    func waitForSendStart() async {
      while !sendStarted {
        await Task.yield()
      }
    }

    func releaseSend() {
      sendContinuation?.resume()
      sendContinuation = nil
    }

    func waitForLoadStart() async {
      while !loadStarted {
        await Task.yield()
      }
    }

    func releaseLoad() {
      loadContinuation?.resume()
      loadContinuation = nil
    }

    func replaceRecordedThread(_ thread: LocalProductChatThread) {
      recordedThread = thread
    }

    func snapshot(limit: Int) async throws -> LocalProductSnapshot {
      throw LocalProductClientError.unavailable
    }

    func timeline(
      teamInstanceID: String,
      cursor: String,
      limit: Int
    ) async throws -> LocalProductTimelinePage {
      throw LocalProductClientError.unavailable
    }

    func chatThread(threadID: String) async throws -> LocalProductChatThread {
      loadStarted = true
      if blockLoad {
        await withCheckedContinuation { continuation in
          loadContinuation = continuation
        }
      }
      return recordedThread
        ?? LocalProductChatThread(
          threadID: threadID,
          messages: [],
          canReply: true,
          requiresConfirmation: false
        )
    }

    func sendChatMessage(threadID: String, content: String) async throws -> LocalProductChatThread {
      try await sendChatMessage(
        threadID: threadID,
        content: content,
        profileID: ""
      )
    }

    func sendChatMessage(
      threadID: String,
      content: String,
      profileID: String
    ) async throws -> LocalProductChatThread {
      try await sendChatMessage(
        threadID: threadID,
        content: content,
        profileID: profileID,
        contextMode: nil,
        incidentID: "test-chat"
      )
    }

    func sendChatMessage(
      threadID: String,
      content: String,
      profileID: String,
      contextMode: LocalProductConversationContextMode?,
      incidentID: String
    ) async throws -> LocalProductChatThread {
      try await sendChatMessage(
        threadID: threadID,
        content: content,
        profileID: profileID,
        contextMode: contextMode,
        expectedExecutionBinding: nil,
        incidentID: incidentID
      )
    }

    func sendChatMessage(
      threadID: String,
      content: String,
      profileID: String,
      contextMode: LocalProductConversationContextMode?,
      expectedExecutionBinding: LocalProductConversationExecutionBinding?,
      incidentID: String
    ) async throws -> LocalProductChatThread {
      sendStarted = true
      recordedThreadID = threadID
      recordedProfileID = profileID
      recordedContextMode = contextMode
      recordedExpectedExecutionBinding = expectedExecutionBinding
      if blockSend {
        await withCheckedContinuation { continuation in
          sendContinuation = continuation
        }
      }
      if let sendError {
        throw sendError
      }
      if let failNextSend {
        self.failNextSend = nil
        throw failNextSend
      }
      let existing = recordedThread?.threadID == threadID ? recordedThread : nil
      var segments = existing?.segments ?? []
      if segments.isEmpty || contextMode != nil {
        segments.append(
          LocalProductConversationSegment(
            segmentID: "segment-\(segments.count + 1)",
            profileID: profileID,
            contextMode: contextMode ?? .startClean,
            contextCapsuleDigest: String(repeating: "a", count: 64),
            executionBinding: responseExecutionBinding,
            bindingDigest: String(repeating: "b", count: 64)
          ))
      }
      let segmentID = segments.last?.segmentID ?? "segment-1"
      let user = LocalProductChatMessage(
        messageID: "1",
        segmentID: segmentID,
        role: "user",
        content: content,
        tentative: false
      )
      let loom = LocalProductChatMessage(
        messageID: "2",
        segmentID: segmentID,
        role: "loom",
        content: "Loom received: \(content)",
        tentative: false
      )
      let thread = LocalProductChatThread(
        threadID: threadID,
        profileID: profileID,
        segments: segments,
        attempts: attemptFailureCode.map { code in
          [
            LocalProductConversationAttempt(
              attemptID: "attempt-1",
              segmentID: segmentID,
              profileID: profileID,
              modelID: nil,
              reasoningEffort: nil,
              contextMode: contextMode ?? .startClean,
              contextCapsuleDigest: String(repeating: "a", count: 64),
              executionBinding: responseExecutionBinding,
              bindingDigest: String(repeating: "b", count: 64),
              incidentID: incidentID,
              status: "failed",
              failureCode: code,
              failureStage: attemptFailureStage ?? "conversation_dispatch",
			  httpStatus: attemptHTTPStatus,
			  providerCode: attemptProviderCode,
			  failureMessage: attemptFailureMessage,
			  retryAfterSeconds: attemptRetryAfterSeconds,
              retryable: attemptRetryable
            )
          ]
        } ?? [],
        messages: (existing?.messages ?? []) + [user, loom],
        canReply: true,
        requiresConfirmation: false
      )
      recordedThread = thread
      return thread
    }

    func setupSnapshot() async throws -> LocalProductSetupSnapshot {
      setupStarted = true
      throw LocalProductClientError.unavailable
    }
  }

  private func conversationProfileSetupSnapshot() throws -> LocalProductSetupSnapshot {
    return try LocalProductSetupWire.decodeSnapshot(
      Data(
        #"""
        {
          "schema_version":1,
          "view_version":"view-profile",
          "codex":{"provider_id":"codex","auth_mode":"native_auth","credential_reference":"","revision":0,"status":"available","reason":""},
          "minimax":{"provider_id":"minimax","auth_mode":"brokered","credential_reference":"","revision":0,"status":"unconfigured","reason":""},
          "providers":[],
          "conversation_profiles":[
            {"profile_id":"conversation-openai-codex-default-v1","harness_adapter":"codex","provider_id":"openai","provider_account_id":"","display_name":"Codex","protocol":"openai_responses","model_id":"codex-default","auth_mode":"native_auth","credential_revision":0},
            {"profile_id":"conversation-anthropic-claude-sonnet-5-account-work-r5","harness_adapter":"loom-native","provider_id":"anthropic","provider_account_id":"anthropic.work","display_name":"Anthropic","protocol":"anthropic_messages","model_id":"claude-sonnet-5","auth_mode":"brokered","credential_revision":5},
            {"profile_id":"conversation-deepseek-deepseek-chat-r2","harness_adapter":"loom-native","provider_id":"deepseek","provider_account_id":"deepseek.primary","display_name":"DeepSeek","protocol":"openai_compatible","model_id":"deepseek-chat","auth_mode":"brokered","credential_revision":2}
          ],
          "runtimes":[],"saved_teams":[],"templates":[],"role_options":[],
          "skills":[],"permissions":[],"resources":[]
        }
        """#.utf8))
  }

  func testAgentAttemptRecoveryRequiresConfirmationBeforeResume() async throws {
    let client = AgentRecoveryStubClient()
    let store = LocalProductStore(client: client)

    await store.previewAgentAttemptRecoveries()

    let candidate = try XCTUnwrap(store.agentRecoveryCandidates.first)
    XCTAssertEqual(candidate.providerAccountID, "deepseek.primary")
    XCTAssertTrue(store.canConfirmAgentAttemptRecovery(candidate))
    XCTAssertFalse(store.canResumeAgentAttemptRecovery(candidate))
    XCTAssertEqual(client.requests.map(\.operation), [.preview])

    await store.resumeAgentAttemptRecovery(candidate)

    XCTAssertEqual(client.requests.map(\.operation), [.preview])
    XCTAssertEqual(store.agentRecoveryFailure?.code, .humanRequired)
    XCTAssertEqual(store.agentRecoveryFailure?.stage, .agentAttemptReconcile)

    await store.confirmAgentAttemptRecovery(candidate, principalID: "local-user")

    XCTAssertEqual(client.requests.map(\.operation), [.preview, .confirm])
    XCTAssertTrue(store.canResumeAgentAttemptRecovery(candidate))
    XCTAssertFalse(store.canConfirmAgentAttemptRecovery(candidate))
    XCTAssertNil(store.agentRecoveryFailure)

    await store.resumeAgentAttemptRecovery(candidate)

    XCTAssertEqual(client.requests.map(\.operation), [.preview, .confirm, .resume])
    XCTAssertTrue(store.agentRecoveryCandidates.isEmpty)
    XCTAssertFalse(store.canConfirmAgentAttemptRecovery(candidate))
    XCTAssertFalse(store.canResumeAgentAttemptRecovery(candidate))
    XCTAssertNil(store.agentRecoveryFailure)
    XCTAssertEqual(client.snapshotRequestCount, 1)
  }

  func testAgentAttemptRecoveryRequiresFreshPreviewAfterResumeConflict() async throws {
    let client = AgentRecoveryStubClient(
      resumeError: LocalIPCRemoteError(
        code: .conflict, recoverable: true,
        stage: .agentAttemptReconcile,
        incidentID: "loom-agent-recovery-conflict"
      )
    )
    let store = LocalProductStore(client: client)
    await store.previewAgentAttemptRecoveries()
    let candidate = try XCTUnwrap(store.agentRecoveryCandidates.first)
    await store.confirmAgentAttemptRecovery(candidate, principalID: "local-user")

    await store.resumeAgentAttemptRecovery(candidate)

    XCTAssertTrue(store.agentRecoveryCandidates.isEmpty)
    XCTAssertFalse(store.canResumeAgentAttemptRecovery(candidate))
    XCTAssertEqual(store.agentRecoveryFailure?.code, .conflict)
    XCTAssertEqual(store.agentRecoveryFailure?.stage, .agentAttemptReconcile)
    XCTAssertEqual(
      store.agentRecoveryFailure?.incidentID,
      "loom-agent-recovery-conflict"
    )
    XCTAssertTrue(store.agentRecoveryFailure?.detail.contains("Refresh") ?? false)
  }

  func testAgentAttemptRecoveryDoesNotDispatchDuplicateResume() async throws {
    let client = AgentRecoveryStubClient(suspendResume: true)
    let store = LocalProductStore(client: client)
    await store.previewAgentAttemptRecoveries()
    let candidate = try XCTUnwrap(store.agentRecoveryCandidates.first)
    await store.confirmAgentAttemptRecovery(candidate, principalID: "local-user")

    let first = Task { @MainActor in
      await store.resumeAgentAttemptRecovery(candidate)
    }
    await client.waitForResumeStart()
    await store.resumeAgentAttemptRecovery(candidate)

    XCTAssertEqual(client.requests.map(\.operation), [.preview, .confirm, .resume])
    client.releaseResume()
    await first.value
    XCTAssertTrue(store.agentRecoveryCandidates.isEmpty)
    XCTAssertNil(store.agentRecoveryFailure)
  }

  func testToolRecoveryRequiresFreshPreviewAndAdvertisedAction() async throws {
    let client = ToolRecoveryStubClient()
    let store = LocalProductStore(client: client)

    await store.resolveToolRecovery(
      try client.candidate(), action: .abortAttempt
    )

    XCTAssertTrue(client.requests.isEmpty)
    XCTAssertEqual(store.toolRecoveryFailure?.code, .humanRequired)

    await store.previewToolRecoveries()
    let candidate = try XCTUnwrap(store.toolRecoveryCandidates.first)
    XCTAssertTrue(store.canResolveToolRecovery(candidate, action: .abortAttempt))
    XCTAssertFalse(store.canResolveToolRecovery(candidate, action: .retryInNewAttempt))

    await store.resolveToolRecovery(candidate, action: .retryInNewAttempt)

    XCTAssertEqual(client.requests.map(\.operation), [.preview])
    XCTAssertEqual(store.toolRecoveryFailure?.code, .humanRequired)
  }

  func testToolRecoveryAbortConsumesExactCandidateAndRefreshesProjection() async throws {
    let client = ToolRecoveryStubClient()
    let store = LocalProductStore(client: client)
    await store.previewToolRecoveries()
    let candidate = try XCTUnwrap(store.toolRecoveryCandidates.first)

    await store.resolveToolRecovery(candidate, action: .abortAttempt)

    XCTAssertEqual(client.requests.map(\.operation), [.preview, .resolve])
    XCTAssertEqual(client.requests.last?.candidateDigest, candidate.candidateDigest)
    XCTAssertEqual(client.requests.last?.action, .abortAttempt)
    XCTAssertTrue(store.toolRecoveryCandidates.isEmpty)
    XCTAssertEqual(
      store.toolRecoveryDecisions[candidate.candidateDigest]?.action,
      .abortAttempt
    )
    XCTAssertEqual(client.executionSnapshotRequestCount, 1)
    XCTAssertNil(store.toolRecoveryFailure)
  }

  func testToolRecoveryConflictDiscardsCandidateAndRequiresNewPreview() async throws {
    let client = ToolRecoveryStubClient(
      resolveError: LocalIPCRemoteError(
        code: .conflict, recoverable: true, stage: .toolRecovery,
        incidentID: "loom-tool-recovery-conflict"
      )
    )
    let store = LocalProductStore(client: client)
    await store.previewToolRecoveries()
    let candidate = try XCTUnwrap(store.toolRecoveryCandidates.first)

    await store.resolveToolRecovery(candidate, action: .abortAttempt)

    XCTAssertTrue(store.toolRecoveryCandidates.isEmpty)
    XCTAssertFalse(store.canResolveToolRecovery(candidate, action: .abortAttempt))
    XCTAssertNil(store.toolRecoveryDecisions[candidate.candidateDigest])
    XCTAssertEqual(store.toolRecoveryFailure?.code, .conflict)
    XCTAssertEqual(store.toolRecoveryFailure?.stage, .toolRecovery)
    XCTAssertEqual(
      store.toolRecoveryFailure?.incidentID,
      "loom-tool-recovery-conflict"
    )
    XCTAssertTrue(store.toolRecoveryFailure?.detail.contains("Refresh") ?? false)
  }

  func testToolRecoveryDoesNotDispatchDuplicateResolution() async throws {
    let client = ToolRecoveryStubClient(suspendResolve: true)
    let store = LocalProductStore(client: client)
    await store.previewToolRecoveries()
    let candidate = try XCTUnwrap(store.toolRecoveryCandidates.first)

    let first = Task { @MainActor in
      await store.resolveToolRecovery(candidate, action: .abortAttempt)
    }
    await client.waitForResolveStart()
    await store.resolveToolRecovery(candidate, action: .abortAttempt)

    XCTAssertEqual(client.requests.map(\.operation), [.preview, .resolve])
    client.releaseResolve()
    await first.value
    XCTAssertTrue(store.toolRecoveryCandidates.isEmpty)
    XCTAssertNil(store.toolRecoveryFailure)
  }


  func testConversationThreeLayerDefaultsAreAlwaysPresent() throws {
    let client = ChatRecordingClient()
    let store = LocalProductStore(
      client: client,
      initialSetupSnapshot: try conversationProfileSetupSnapshot()
    )
    XCTAssertEqual(store.selectedConversationProfile?.providerID, "openai")
    XCTAssertEqual(store.effectiveConversationModelID, "codex-default")
    XCTAssertTrue(
      localProductConversationModels(providerID: "openai")
        .contains { $0.modelID == "codex-default" }
    )
    XCTAssertEqual(store.effectiveConversationReasoningEffort, "")

    store.selectConversationProfile("conversation-deepseek-deepseek-chat-r2")
    XCTAssertEqual(store.effectiveConversationModelID, "deepseek-chat")
    XCTAssertEqual(store.effectiveConversationReasoningEffort, "")

    let openCodeSnapshot = try LocalProductSetupWire.decodeSnapshot(
      Data(
        #"""
        {
          "schema_version":1,
          "view_version":"view-opencode",
          "codex":{"provider_id":"codex","auth_mode":"native_auth","credential_reference":"","revision":0,"status":"available","reason":""},
          "minimax":{"provider_id":"minimax","auth_mode":"brokered","credential_reference":"","revision":0,"status":"unconfigured","reason":""},
          "providers":[],
          "conversation_profiles":[
            {"profile_id":"conversation-opencode-default-v1","harness_adapter":"opencode","provider_id":"opencode","provider_account_id":"","display_name":"OpenCode","protocol":"opencode_agent","model_id":"opencode/deepseek-v4-flash-free","auth_mode":"native_auth","credential_revision":0}
          ],
          "runtimes":[],"saved_teams":[],"templates":[],"role_options":[],
          "skills":[],"permissions":[],"resources":[]
        }
        """#.utf8
      )
    )
    let openCodeStore = LocalProductStore(client: client, initialSetupSnapshot: openCodeSnapshot)
    // The OpenCode profile defaults to its own hosted free-tier model, which
    // needs no verified Provider credential.
    XCTAssertEqual(
      openCodeStore.effectiveConversationModelID,
      "opencode/deepseek-v4-flash-free"
    )
    XCTAssertEqual(openCodeStore.effectiveConversationReasoningEffort, "low")
    // A cross-Provider model whose owning Provider is not verified is refused
    // at selection time instead of failing at send time.
    openCodeStore.selectConversationModel("deepseek/deepseek-chat")
    XCTAssertEqual(
      openCodeStore.effectiveConversationModelID,
      "opencode/deepseek-v4-flash-free"
    )
    // Effort-capable models expose their exact CLI values.
    openCodeStore.selectConversationModel("opencode/deepseek-v4-flash-free")
    XCTAssertEqual(openCodeStore.effectiveConversationReasoningEffort, "low")
    openCodeStore.selectConversationReasoningEffort("max")
    XCTAssertEqual(openCodeStore.effectiveConversationReasoningEffort, "max")
  }

  func testConversationConflictSelfHealsAndRetriesWithCurrentBinding() async throws {
    let client = ChatRecordingClient()
    let store = LocalProductStore(
      client: client,
      initialSetupSnapshot: try conversationProfileSetupSnapshot()
    )
    store.selectConversationProfile("conversation-deepseek-deepseek-chat-r2")
    await store.sendChatMessage("hello")

    client.failNextSend = LocalIPCRemoteError(
      code: .conflict,
      recoverable: true,
      stage: .conversationDispatch,
      incidentID: "loom-chat-test-conflict"
    )
    await store.sendChatMessage("continue after conflict")

    // The conflict was self-healed: the retry passed the current binding and
    // a context mode, and no failure surfaced.
    XCTAssertNil(store.chatOperationFailure)
    XCTAssertEqual(
      client.recordedProfileID,
      "conversation-deepseek-deepseek-chat-r2"
    )
    XCTAssertNotNil(client.recordedContextMode)
    XCTAssertNotNil(client.recordedExpectedExecutionBinding)
    XCTAssertFalse(store.isSendingChatMessage)
  }
  func testChatSessionsSurviveRestartViaRegistryFile() async throws {
    let fileURL = FileManager.default.temporaryDirectory
      .appendingPathComponent("loom-sessions-restart-\\(UUID().uuidString).json")
    let first = LocalProductStore(
      client: ChatRecordingClient(),
      initialSetupSnapshot: try conversationProfileSetupSnapshot(),
      chatSessionsFileURL: fileURL
    )
    let firstThreadID = first.currentChatThreadID()
    first.newConversation()
    let secondThreadID = first.currentChatThreadID()
    XCTAssertNotEqual(secondThreadID, firstThreadID)
    XCTAssertEqual(first.chatSessions.count, 2)

    // A brand-new store instance (fresh App launch) reads the same file and
    // restores both conversations plus the previously selected one.
    let second = LocalProductStore(
      client: ChatRecordingClient(),
      initialSetupSnapshot: try conversationProfileSetupSnapshot(),
      chatSessionsFileURL: fileURL
    )
    XCTAssertEqual(second.chatSessions.count, 2)
    XCTAssertTrue(second.chatSessions.contains { $0.threadID == firstThreadID })
    XCTAssertTrue(second.chatSessions.contains { $0.threadID == secondThreadID })
    XCTAssertEqual(second.selectedChatSessionID, secondThreadID)
    XCTAssertEqual(second.currentChatThreadID(), secondThreadID)
    try? FileManager.default.removeItem(at: fileURL)
  }
}

private final class ToolRecoveryStubClient:
  LocalProductClientProtocol,
  LocalProductToolRecoveryClientProtocol,
  LocalProductExecutionSnapshotClientProtocol
{
  private(set) var requests: [LocalProductToolRecoveryRequest] = []
  private(set) var executionSnapshotRequestCount = 0
  private let resolveError: Error?
  private let suspendResolve: Bool
  private var resolveStarted = false
  private var resolveContinuation: CheckedContinuation<Void, Never>?

  init(resolveError: Error? = nil, suspendResolve: Bool = false) {
    self.resolveError = resolveError
    self.suspendResolve = suspendResolve
  }

  func waitForResolveStart() async {
    while !resolveStarted || resolveContinuation == nil { await Task.yield() }
  }

  func releaseResolve() {
    resolveContinuation?.resume()
    resolveContinuation = nil
  }

  func snapshot(limit: Int) async throws -> LocalProductSnapshot {
    .empty(viewVersion: String(repeating: "f", count: 64))
  }

  func timeline(teamInstanceID: String, cursor: String, limit: Int) async throws
    -> LocalProductTimelinePage
  {
    throw LocalProductClientError.notFound
  }

  func executionSnapshot(journeyID: String) async throws -> ExecutionSnapshot {
    executionSnapshotRequestCount += 1
    return ExecutionSnapshot(viewVersion: String(repeating: "e", count: 64))
  }

  func candidate() throws -> LocalProductToolRecoveryCandidate {
    let response = try LocalProductToolRecoveryWire.decodeResponse(
      Data(previewJSON(incidentID: "loom-tool-recovery-fixture").utf8)
    )
    return try XCTUnwrap(response.candidates.first)
  }

  func recoverToolCall(
    _ request: LocalProductToolRecoveryRequest,
    incidentID: String
  ) async throws -> LocalProductToolRecoveryResponse {
    requests.append(request)
    switch request.operation {
    case .preview:
      return try LocalProductToolRecoveryWire.decodeResponse(
        Data(previewJSON(incidentID: incidentID).utf8)
      )
    case .resolve:
      if let resolveError { throw resolveError }
      if suspendResolve {
        resolveStarted = true
        await withCheckedContinuation { continuation in
          resolveContinuation = continuation
        }
      }
      return try LocalProductToolRecoveryWire.decodeResponse(
        Data(
          """
          {"schema_version":1,"incident_id":"\(incidentID)","operation":"resolve","candidates":[],"decision":{"schema_version":1,"decision_id":"\(request.decisionID)","action":"abort_attempt","candidate_digest":"\(request.candidateDigest)","execution_id":"execution-1"}}
          """.utf8
        )
      )
    }
  }

  private func previewJSON(incidentID: String) -> String {
    """
    {"schema_version":1,"incident_id":"\(incidentID)","operation":"preview","candidates":[{"schema_version":1,"status":"available","candidate_digest":"\(String(repeating: "a", count: 64))","execution_id":"execution-1","job_id":"job-1","call_digest":"\(String(repeating: "b", count: 64))","tool":"write_file","generation":2,"operation_id":"operation-1","incident_id":"incident-original-1","recovery_code":"side_effect_unknown","recovery_required_at":"2026-08-14T12:00:00Z","available_actions":["abort_attempt"]}]}
    """
  }
}

private final class AgentRecoveryStubClient:
  LocalProductClientProtocol,
  LocalProductAgentRecoveryClientProtocol
{
  private(set) var requests: [LocalProductAgentRecoveryRequest] = []
  private(set) var snapshotRequestCount = 0
  private let resumeError: Error?
  private let suspendResume: Bool
  private var resumeStarted = false
  private var resumeContinuation: CheckedContinuation<Void, Never>?

  init(resumeError: Error? = nil, suspendResume: Bool = false) {
    self.resumeError = resumeError
    self.suspendResume = suspendResume
  }

  func waitForResumeStart() async {
    while !resumeStarted || resumeContinuation == nil { await Task.yield() }
  }

  func releaseResume() {
    resumeContinuation?.resume()
    resumeContinuation = nil
  }

  func snapshot(limit: Int) async throws -> LocalProductSnapshot {
    snapshotRequestCount += 1
    return .empty(viewVersion: String(repeating: "f", count: 64))
  }

  func timeline(teamInstanceID: String, cursor: String, limit: Int) async throws
    -> LocalProductTimelinePage
  {
    throw LocalProductClientError.notFound
  }

  func recoverAgentAttempt(
    _ request: LocalProductAgentRecoveryRequest,
    incidentID: String
  ) async throws -> LocalProductAgentRecoveryResponse {
    requests.append(request)
    let candidateDigest = String(repeating: "a", count: 64)
    let capabilityDigest = String(repeating: "b", count: 64)
    let data: Data
    switch request.operation {
    case .preview:
      data = Data(
        """
        {"schema_version":1,"incident_id":"\(incidentID)","operation":"preview","candidates":[{"schema_version":1,"status":"available","action":"resume_pre_model","candidate_digest":"\(candidateDigest)","capability_digest":"\(capabilityDigest)","attempt_id":"attempt-1","team_instance_id":"team-1","segment_id":"segment-1","work_item_id":"work-1","run_id":"run-1","claim_generation":3,"runtime_instance_id":"runtime-1","agent_instance_id":"agent-1","harness_adapter":"loom-native","provider_id":"deepseek","provider_account_id":"deepseek.primary","model_id":"deepseek-chat","credential_revision":4}]}
        """.utf8
      )
    case .confirm:
      data = Data(
        """
        {"schema_version":1,"incident_id":"\(incidentID)","operation":"confirm","decision":{"schema_version":1,"decision_id":"\(request.decisionID)","action":"resume_pre_model","candidate_digest":"\(request.candidateDigest)","capability_digest":"\(request.capabilityDigest)"}}
        """.utf8
      )
    case .resume:
      if let resumeError { throw resumeError }
      if suspendResume {
        if resumeContinuation != nil {
          throw LocalIPCRemoteError(
            code: .conflict, recoverable: true,
            stage: .agentAttemptReconcile, incidentID: incidentID
          )
        }
        resumeStarted = true
        await withCheckedContinuation { continuation in
          resumeContinuation = continuation
        }
      }
      data = Data(
        """
        {"schema_version":1,"incident_id":"\(incidentID)","operation":"resume","resume":{"schema_version":1,"status":"completed","decision_id":"\(request.decisionID)","candidate_digest":"\(request.candidateDigest)","capability_digest":"\(request.capabilityDigest)","attempt_id":"attempt-1","runtime_instance_id":"runtime-1"}}
        """.utf8
      )
    }
    return try LocalProductAgentRecoveryWire.decodeResponse(data)
  }
}

private func restartedSideTaskFixture() throws -> (
  snapshot: LocalProductSnapshot,
  timeline: LocalProductTimelinePage,
  parentExecutionDigest: String
) {
  let view = String(repeating: "a", count: 64)
  let parentExecution = String(repeating: "b", count: 64)
  let handoff = String(repeating: "c", count: 64)
  let artifact = String(repeating: "d", count: 64)
  let snapshotJSON = """
    {"schema_version":3,"view_version":"\(view)","partial":false,"stale":false,"reason":"",
     "health":{"daemon":"serving_request","journal":"available","projection":"current"},
     "runtimes":[],"teams":[{"team_instance_id":"team-1","display_name":"Parent Team",
      "source_kind":"saved","state":"ready","confirmed":true,"executable":true,"read_only":false}],
     "missions":[{"schema_version":1,"mission_id":"mission/team-1","team_instance_id":"team-1",
      "title":"Parent Mission","source_kind":"saved_team","lane":"Review","status":"human_required",
      "priority":"normal","plan_digest":"\(String(repeating: "e", count: 64))","simple":true,
      "node_count":1,"completed_node_count":1,"active_node_count":0,"review_node_count":0,
      "attention_count":1,"current_node_id":"main","last_milestone":"Side-task decision required",
      "team_pulse":[],"topology":[]}],
     "runs":[{"run_id":"run-1","work_item_id":"work-1","phase":"terminal","terminal_status":"succeeded",
      "terminal_reason":"","runtime_instance_id":"runtime-1","agent_instance_id":"agent-1","claim_generation":2}],
     "evidence":[],"attention":[],"prepared_decisions":[],
     "side_tasks":[{"side_task_id":"side-1","parent_mission_id":"mission/team-1",
      "parent_team_instance_id":"team-1","parent_task_id":"work-1","parent_run_id":"run-1",
      "parent_claim_generation":2,"parent_execution_digest":"\(parentExecution)",
      "side_execution_team_instance_id":"team-side-1","purpose":"research","mode":"decision_required",
      "title":"Bounded research","status":"decision_required","source_generation":1,"handoff_version":1,
      "handoff_digest":"\(handoff)","summary_artifact_digest":"\(artifact)","what_happened":"Authorized result",
      "authorized_findings":[],"evidence_references":[],"artifact_references":[],"risk":"low",
      "uncertainties":[],"scope_delta":[],"decision_options":["discard"],"recommended_option":"discard",
      "recommendation_authority":"proposal_only","usage_observed":false,"usage_microunits":0,
      "usage_currency":"","decision_deadline":"2026-08-03T16:00:00Z","available_decisions":["discard"],
      "effect_status":"none"}],
     "runtime_page":{"next_cursor":"","has_more":false},"team_page":{"next_cursor":"","has_more":false},
     "mission_page":{"next_cursor":"","has_more":false},"run_page":{"next_cursor":"","has_more":false},
     "evidence_page":{"next_cursor":"","has_more":false}}
    """
  let timelineJSON = """
    {"schema_version":1,"team_instance_id":"team-1","view_version":"\(view)","next_cursor":"",
     "has_more":false,"gap":null,"records":[],"board":{"schema_version":1,"team_instance_id":"team-1",
      "plan_digest":"\(String(repeating: "e", count: 64))","status":"succeeded","view_version":"\(view)",
      "nodes":[{"logical_node_id":"main","status":"succeeded","dependency_satisfied":true,
       "current_attempt":1,"work_item_id":"work-1","run_id":"run-1","runtime_instance_id":"runtime-1",
       "agent_instance_id":"agent-1","verification_status":"accepted","recovery_action":"","retry_at":""}],
      "cost":{"observed":false,"amount_microunits":null,"currency":""}},"attention":[]}
    """
  return (
    try LocalProductWire.decodeSnapshot(Data(snapshotJSON.utf8)),
    try LocalProductWire.decodeTimeline(Data(timelineJSON.utf8)),
    parentExecution
  )
}

private final class RestartedSideTaskStubClient:
  LocalProductClientProtocol,
  LocalProductHandoffClientProtocol
{
  private let fixedSnapshot: LocalProductSnapshot
  private let fixedTimeline: LocalProductTimelinePage
  private(set) var decisions: [LocalProductSideTaskDecisionRequest] = []

  init(snapshot: LocalProductSnapshot, timeline: LocalProductTimelinePage) {
    fixedSnapshot = snapshot
    fixedTimeline = timeline
  }

  func snapshot(limit: Int) async throws -> LocalProductSnapshot { fixedSnapshot }
  func timeline(teamInstanceID: String, cursor: String, limit: Int) async throws
    -> LocalProductTimelinePage
  {
    fixedTimeline
  }
  func proposeSideTask(_ request: LocalProductSideTaskProposalRequest) async throws
    -> LocalProductSideTaskProposalResult
  {
    throw LocalProductClientError.unavailable
  }
  func createSideTask(_ request: LocalProductSideTaskCreateRequest) async throws
    -> LocalProductSideTaskCreateResult
  {
    throw LocalProductClientError.unavailable
  }
  func readSideTask(_ request: LocalProductSideTaskReadRequest) async throws
    -> LocalProductSideTaskReadResult
  {
    throw LocalProductClientError.unavailable
  }
  func decideSideTask(_ request: LocalProductSideTaskDecisionRequest) async throws
    -> LocalProductSideTaskDecisionResult
  {
    decisions.append(request)
    let body = """
      {"schema_version":1,"side_task_id":"\(request.sideTaskID)","decision":"\(request.decision)",
       "status":"decided","effect_status":"none","context_packet_digest":"",
       "continuation_execution_team_instance_id":"","view_version":"\(fixedSnapshot.viewVersion)"}
      """
    return try LocalProductHandoffWire.decodeDecision(Data(body.utf8))
  }
}

private struct TimelineRequest: Equatable {
  let teamID: String
  let cursor: String
  let limit: Int
}

private final class TimelinePagingStubClient: LocalProductClientProtocol {
  private let fixedSnapshot: LocalProductSnapshot
  private let pages: [String: LocalProductTimelinePage]
  private(set) var timelineRequests: [TimelineRequest] = []

  init(snapshot: LocalProductSnapshot, pages: [String: LocalProductTimelinePage]) {
    fixedSnapshot = snapshot
    self.pages = pages
  }

  func snapshot(limit: Int) async throws -> LocalProductSnapshot { fixedSnapshot }

  func timeline(
    teamInstanceID: String,
    cursor: String,
    limit: Int
  ) async throws -> LocalProductTimelinePage {
    timelineRequests.append(.init(teamID: teamInstanceID, cursor: cursor, limit: limit))
    guard let page = pages[cursor] else {
      throw LocalProductClientError.notFound
    }
    return page
  }
}

private actor SuspendedTimelineStubClient: LocalProductClientProtocol {
  private let fixedSnapshot: LocalProductSnapshot
  private let immediatePages: [String: LocalProductTimelinePage]
  private var suspendedContinuation: CheckedContinuation<LocalProductTimelinePage, Error>?

  init(
    snapshot: LocalProductSnapshot,
    immediatePages: [String: LocalProductTimelinePage] = [:]
  ) {
    fixedSnapshot = snapshot
    self.immediatePages = immediatePages
  }

  func snapshot(limit: Int) async throws -> LocalProductSnapshot { fixedSnapshot }

  func timeline(
    teamInstanceID: String,
    cursor: String,
    limit: Int
  ) async throws -> LocalProductTimelinePage {
    if let immediate = immediatePages[teamInstanceID] { return immediate }
    return try await withCheckedThrowingContinuation { continuation in
      suspendedContinuation = continuation
    }
  }

  func waitForSuspendedRequest() async {
    while suspendedContinuation == nil { await Task.yield() }
  }

  func resolveSuspended(with result: Result<LocalProductTimelinePage, Error>) {
    let continuation = suspendedContinuation
    suspendedContinuation = nil
    continuation?.resume(with: result)
  }
}

private actor SequencedSuspendedTimelineStubClient: LocalProductClientProtocol {
  private let fixedSnapshot: LocalProductSnapshot
  private var continuations: [CheckedContinuation<LocalProductTimelinePage, Error>?] = []

  init(snapshot: LocalProductSnapshot) { fixedSnapshot = snapshot }

  func snapshot(limit: Int) async throws -> LocalProductSnapshot { fixedSnapshot }

  func timeline(
    teamInstanceID: String,
    cursor: String,
    limit: Int
  ) async throws -> LocalProductTimelinePage {
    return try await withCheckedThrowingContinuation { continuation in
      continuations.append(continuation)
    }
  }

  func waitForRequestCount(_ count: Int) async {
    while continuations.count < count { await Task.yield() }
  }

  func resolveRequest(
    at index: Int,
    with result: Result<LocalProductTimelinePage, Error>
  ) {
    let continuation = continuations[index]
    continuations[index] = nil
    continuation?.resume(with: result)
  }
}

private func timelineSnapshot(teamIDs: [String]) throws -> LocalProductSnapshot {
  let teams = teamIDs.map { teamID in
    """
    {"team_instance_id":"\(teamID)","display_name":"\(teamID)",
     "source_kind":"saved","state":"ready","confirmed":true,
     "executable":true,"read_only":false}
    """
  }.joined(separator: ",")
  let json = LocalProductModelsTests.snapshotJSON.replacingOccurrences(
    of: "\"teams\":[]",
    with: "\"teams\":[\(teams)]"
  )
  return try LocalProductWire.decodeSnapshot(Data(json.utf8))
}

private func twoMissionSnapshot() throws -> LocalProductSnapshot {
  var object = try XCTUnwrap(
    JSONSerialization.jsonObject(
      with: Data(MissionOrchestrationTests.snapshotJSON.utf8)
    ) as? [String: Any]
  )
  var missions = try XCTUnwrap(object["missions"] as? [[String: Any]])
  var second = try XCTUnwrap(missions.first)
  second["mission_id"] = "mission/team-2"
  second["team_instance_id"] = "team-2"
  second["title"] = "Second Mission"
  missions.append(second)
  object["missions"] = missions
  return try LocalProductWire.decodeSnapshot(
    JSONSerialization.data(withJSONObject: object, options: [.sortedKeys])
  )
}

private func timelineRecord(
  schemaVersion: Int = 1,
  deliveryID: String,
  kind: String,
  teamID: String,
  status: String = ""
) -> String {
  """
  {"schema_version":\(schemaVersion),"delivery_id":"\(deliveryID)","kind":"\(kind)",
   "authority":"journal","team_instance_id":"\(teamID)",
   "logical_node_id":"main","attempt_number":2,
   "source_stream_id":"work-item/work-1","source_sequence":1,
   "source_event_id":"event-1","occurred_at":"2026-08-03T00:00:00Z",
   "cursor":"record-cursor","payload":{"status":"\(status)","reason_code":"",
    "action":"","warning_code":"","retry_at":"","text_delta":"",
    "evidence_digest":"\(String(repeating: "a", count: 64))",
    "cost":{"observed":false,"amount_microunits":null,"currency":""}}}
  """
}

private func timelineAttention(
  schemaVersion: Int = 1,
  attentionID: String,
  teamID: String
) -> String {
  """
  {"schema_version":\(schemaVersion),"attention_id":"\(attentionID)","kind":"blocked",
   "severity":"warning","team_instance_id":"\(teamID)",
   "logical_node_id":"main","work_item_id":"work-1",
   "approval_request_id":"","runtime_instance_id":"runtime-1",
   "status":"blocked","occurred_at":"2026-08-03T00:00:00Z",
   "action_required":"Review the blocked node"}
  """
}

private func timelinePage(
  schemaVersion: Int = 1,
  teamID: String,
  viewVersion: String = "view-1",
  boardTeamID: String? = nil,
  boardViewVersion: String? = nil,
  boardSchemaVersion: Int = 1,
  boardStatus: String = "succeeded",
  nextCursor: String = "final-cursor",
  hasMore: Bool,
  gap: Bool = false,
  records: [String],
  attention: [String] = []
) throws -> LocalProductTimelinePage {
  let gapJSON =
    gap
    ? """
    {"schema_version":1,"delivery_id":"gap-1","kind":"stream_gap",
     "team_instance_id":"\(teamID)","reason":"cursor_conflict",
     "previous_cursor_digest":"","current_view_version":"\(viewVersion)",
     "artifact_available":false,"artifact_digest":"","recoverable":true,
     "occurred_at":"2026-08-03T00:00:00Z"}
    """
    : "null"
  let json = """
    {"schema_version":1,"team_instance_id":"\(teamID)",
     "view_version":"\(viewVersion)","next_cursor":"\(nextCursor)",
     "has_more":\(hasMore),"gap":\(gapJSON),"records":[\(records.joined(separator: ","))],
     "board":{"schema_version":\(boardSchemaVersion),"team_instance_id":"\(boardTeamID ?? teamID)",
      "plan_digest":"","status":"\(boardStatus)",
      "view_version":"\(boardViewVersion ?? viewVersion)","nodes":[],
      "cost":{"observed":false,"amount_microunits":null,"currency":""}},
     "attention":[\(attention.joined(separator: ","))]}
    """
  let decoded = try LocalProductWire.decodeTimeline(Data(json.utf8))
  guard schemaVersion != decoded.schemaVersion else { return decoded }
  return LocalProductTimelinePage(
    schemaVersion: schemaVersion,
    teamInstanceID: decoded.teamInstanceID,
    viewVersion: decoded.viewVersion,
    nextCursor: decoded.nextCursor,
    hasMore: decoded.hasMore,
    gap: decoded.gap,
    records: decoded.records,
    board: decoded.board,
    attention: decoded.attention
  )
}

private func executableMissionSnapshot() throws -> LocalProductSnapshot {
  let viewVersion = String(repeating: "b", count: 64)
  let json = LocalProductModelsTests.snapshotJSON.replacingOccurrences(
    of: "\"teams\":[]",
    with: """
      "teams":[{
        "team_instance_id":"team-1","display_name":"Coding Team",
        "source_kind":"saved_team","state":"created",
        "confirmed":true,"executable":true,"read_only":false
      }]
      """
  ).replacingOccurrences(
    of: "\"view_version\":\"view-1\"",
    with: "\"view_version\":\"\(viewVersion)\""
  )
  return try LocalProductWire.decodeSnapshot(Data(json.utf8))
}

private final class ExecutionStubClient:
  LocalProductClientProtocol,
  LocalProductExecutionClientProtocol
{
  private(set) var fixedSnapshot: LocalProductSnapshot
  private(set) var commands: [LocalProductExecutionCommand] = []
  private(set) var timelineRequestCount = 0
  private(set) var snapshotRequestCount = 0
  private let preflightNodeStatus: String
  private let additionalPreflightNodeStatus: String?
  private let suspendPreflight: Bool
  private let startResultStatus: String
  private let startError: Error?
  private var snapshotAfterPreflight: LocalProductSnapshot?
  private var preflightStarted = false
  private var preflightContinuation: CheckedContinuation<Void, Never>?

  init(
    snapshot: LocalProductSnapshot,
    preflightNodeStatus: String = "ready",
    additionalPreflightNodeStatus: String? = nil,
    suspendPreflight: Bool = false,
    startResultStatus: String = "running",
    startError: Error? = nil
  ) {
    fixedSnapshot = snapshot
    self.preflightNodeStatus = preflightNodeStatus
    self.additionalPreflightNodeStatus = additionalPreflightNodeStatus
    self.suspendPreflight = suspendPreflight
    self.startResultStatus = startResultStatus
    self.startError = startError
  }

  func waitForPreflightStart() async {
    while !preflightStarted || preflightContinuation == nil { await Task.yield() }
  }

  func releasePreflight() {
    preflightContinuation?.resume()
    preflightContinuation = nil
  }

  func replaceSnapshot(_ snapshot: LocalProductSnapshot) { fixedSnapshot = snapshot }

  func replaceSnapshotAfterPreflight(_ snapshot: LocalProductSnapshot) {
    snapshotAfterPreflight = snapshot
  }

  func snapshot(limit: Int) async throws -> LocalProductSnapshot {
    snapshotRequestCount += 1
    return fixedSnapshot
  }

  func timeline(
    teamInstanceID: String,
    cursor: String,
    limit: Int
  ) async throws -> LocalProductTimelinePage {
    timelineRequestCount += 1
    let body = """
      {"schema_version":1,"team_instance_id":"\(teamInstanceID)",
       "view_version":"\(fixedSnapshot.viewVersion)","next_cursor":"",
       "has_more":false,"gap":null,"records":[{
        "schema_version":1,"delivery_id":"delivery-1","kind":"tentative_output",
        "authority":"tentative","team_instance_id":"\(teamInstanceID)",
        "logical_node_id":"main","attempt_number":1,"source_stream_id":"run/run-1",
        "source_sequence":1,"source_event_id":"","occurred_at":"2026-08-01T00:00:00Z",
        "cursor":"","payload":{"status":"","reason_code":"","action":"",
         "warning_code":"","retry_at":"","text_delta":"Compiling checks",
         "evidence_digest":"","cost":{"observed":false,"amount_microunits":null,"currency":""}}}],
       "board":{"schema_version":1,"team_instance_id":"\(teamInstanceID)",
        "plan_digest":"\(String(repeating: "c", count: 64))","status":"running",
        "view_version":"\(fixedSnapshot.viewVersion)","nodes":[{
         "logical_node_id":"main","status":"running","dependency_satisfied":true,
         "current_attempt":1,"work_item_id":"work-1","run_id":"run-1",
         "runtime_instance_id":"runtime-pi","agent_instance_id":"agent-main",
         "verification_status":"pending","recovery_action":"","retry_at":""}],
        "cost":{"observed":false,"amount_microunits":null,"currency":""}},"attention":[]}
      """
    return try LocalProductWire.decodeTimeline(Data(body.utf8))
  }

  func executeMission(
    _ command: LocalProductExecutionCommand
  ) async throws -> LocalProductExecutionEnvelope {
    commands.append(command)
    let body: String
    if command.operation == "preflight" {
      preflightStarted = true
      if suspendPreflight {
        await withCheckedContinuation { continuation in
          preflightContinuation = continuation
        }
      }
      if let snapshotAfterPreflight {
        fixedSnapshot = snapshotAfterPreflight
        self.snapshotAfterPreflight = nil
      }
      let statuses = [preflightNodeStatus] + [additionalPreflightNodeStatus].compactMap { $0 }
      let nodes = statuses.enumerated().map { index, status in
        let logicalNodeID = index == 0 ? "main" : "healthy-sibling"
        let role = index == 0 ? "main" : "reviewer"
        return """
          {"logical_node_id":"\(logicalNodeID)",
           "title":"Implement the bounded change","role":"\(role)",
           "depends_on":[],"max_attempts":2,
           "harness_adapter":"codex","provider_id":"openai",
           "provider_account_id":"openai.primary","model_id":"gpt-5.5-codex",
           "auth_mode":"brokered","credential_revision":3,
           "reasoning_effort":"high","timeout_seconds":120,
           "budget_credits":40,"capabilities":["reasoning_effort","tools"],
           "status":"\(status)",
           "block_reason":"\(status == "ready" ? "" : "Credential is not verified. Reconnect this Provider Account.")"}
          """
      }.joined(separator: ",")
      body = """
        {"schema_version":1,"operation":"preflight","preflight":{
          "schema_version":1,"mission_id":"\(command.missionID)",
          "team_instance_id":"\(command.teamInstanceID)",
          "work_package_id":"\(command.workPackageID)",
          "work_package_digest":"\(command.workPackageDigest)",
          "view_version":"\(command.expectedViewVersion)",
          "plan_digest":"\(String(repeating: "c", count: 64))",
          "preflight_digest":"\(String(repeating: "d", count: 64))",
          "expires_at":"2026-08-01T12:05:00Z",
          "runtime_instance_id":"runtime-pi","runtime_profile_id":"pi-default",
          "model_id":"qwen","auth_mode":"brokered","capacity_available":1,
          "budget_status":"unavailable","side_effects":[],
          "permission_scopes":["workspace"],"approval_points":["before_start"],
          "nodes":[\(nodes)]}}
        """
    } else if command.operation == "start" {
      if let startError {
        throw startError
      }
      let postStartJSON = MissionOrchestrationTests.snapshotJSON
        .replacingOccurrences(of: "mission/team-1", with: command.missionID)
        .replacingOccurrences(
          of: "\"teams\":[]",
          with: """
            "teams":[{"team_instance_id":"\(command.teamInstanceID)",
             "display_name":"Coding Team","source_kind":"saved_team",
             "state":"created","confirmed":true,"executable":true,"read_only":false}]
            """
        )
        .replacingOccurrences(
          of: "\"runs\":[]",
          with: """
            "runs":[{"run_id":"run-1","work_item_id":"work-1",
             "phase":"running","terminal_status":"","terminal_reason":"",
             "runtime_instance_id":"runtime-pi","agent_instance_id":"agent-main",
             "claim_generation":3}]
            """
        )
      fixedSnapshot = try LocalProductWire.decodeSnapshot(Data(postStartJSON.utf8))
      body = """
        {"schema_version":1,"operation":"start","result":{
          "schema_version":1,"mission_id":"\(command.missionID)",
          "team_instance_id":"\(command.teamInstanceID)","status":"\(startResultStatus)",
          "view_version":"\(String(repeating: "e", count: 64))",
          "execution_digest":"\(String(repeating: "f", count: 64))"}}
        """
    } else {
      let cancelled = MissionOrchestrationTests.snapshotJSON
        .replacingOccurrences(of: "mission/team-1", with: command.missionID)
        .replacingOccurrences(
          of: "\"teams\":[]",
          with: """
            "teams":[{"team_instance_id":"\(command.teamInstanceID)",
             "display_name":"Coding Team","source_kind":"saved_team",
             "state":"created","confirmed":true,"executable":true,"read_only":false}]
            """
        )
        .replacingOccurrences(
          of: "\"runs\":[]",
          with: """
            "runs":[{"run_id":"run-1","work_item_id":"work-1",
             "phase":"terminal","terminal_status":"cancelled","terminal_reason":"cancelled",
             "runtime_instance_id":"runtime-pi","agent_instance_id":"agent-main",
             "claim_generation":3}]
            """
        )
        .replacingOccurrences(of: "human_required", with: "cancelled")
      fixedSnapshot = try LocalProductWire.decodeSnapshot(Data(cancelled.utf8))
      body = """
        {"schema_version":1,"operation":"control","result":{
          "schema_version":1,"mission_id":"\(command.missionID)",
          "team_instance_id":"\(command.teamInstanceID)","status":"cancelled",
          "view_version":"\(fixedSnapshot.viewVersion)",
          "execution_digest":"\(command.executionDigest)"}}
        """
    }
    return try LocalProductExecutionWire.decodeEnvelope(Data(body.utf8))
  }
}

final class StubLocalProductClient: LocalProductClientProtocol {
  private var snapshots: [Result<LocalProductSnapshot, Error>]
  private(set) var snapshotRequestCount = 0
  private(set) var timelineRequestCount = 0
  private(set) var lastTimelineLimit: Int?
  private(set) var lastTimelineTeamID: String?

  init(snapshots: [Result<LocalProductSnapshot, Error>]) {
    self.snapshots = snapshots
  }

  func snapshot(limit: Int) async throws -> LocalProductSnapshot {
    snapshotRequestCount += 1
    guard !snapshots.isEmpty else {
      throw LocalProductClientError.unavailable
    }
    return try snapshots.removeFirst().get()
  }

  func timeline(
    teamInstanceID: String,
    cursor: String,
    limit: Int
  ) async throws -> LocalProductTimelinePage {
    timelineRequestCount += 1
    lastTimelineLimit = limit
    lastTimelineTeamID = teamInstanceID
    throw LocalProductClientError.notFound
  }
}

private final class DecisionStubClient:
  LocalProductClientProtocol,
  LocalProductDecisionClientProtocol
{
  private(set) var readCount = 0
  private(set) var submitCount = 0
  private(set) var lastCommand: LocalProductDecisionCommand?
  private let sheet: LocalProductDecisionSheet
  private let authoritativeSubmit: Bool
  private let resultDecisionID: String

  init(
    authoritativeSubmit: Bool = true,
    resultDecisionID: String = "decision-1"
  ) throws {
    self.authoritativeSubmit = authoritativeSubmit
    self.resultDecisionID = resultDecisionID
    sheet = try LocalProductDecisionWire.decodeSheet(
      Data(
        """
        {
          "schema_version":1,
          "kind":"authorization",
          "mission_id":"mission/team-1",
          "team_instance_id":"team-1",
          "view_version":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
          "decision_id":"decision-1",
          "decision_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
          "title":"Authorization required",
          "summary":"Review the prepared command.",
          "requester":"Release Team",
          "target":"repository",
          "command_type":"local process",
          "network_access":"none",
          "credential_access":"none",
          "permission_scope":"this Mission",
          "attempt_scope":"Attempt 1",
          "expected_evidence":"accepted Evidence",
          "technical_details":[],
          "actions":["not_now","deny","edit_scope","allow_once"],
          "prepared_actions":["deny","allow_once"],
          "prepared":true,
          "logical_node_id":"main",
          "attempt_number":1,
          "claim_generation":1
        }
        """.utf8
      )
    )
  }

  func command(
    operation: String,
    action: String
  ) -> LocalProductDecisionCommand {
    LocalProductDecisionCommand(
      operation: operation,
      kind: sheet.kind,
      action: action,
      missionID: sheet.missionID,
      teamInstanceID: sheet.teamInstanceID,
      viewVersion: sheet.viewVersion,
      decisionID: sheet.decisionID,
      decisionDigest: sheet.decisionDigest,
      logicalNodeID: sheet.logicalNodeID,
      attemptNumber: sheet.attemptNumber,
      claimGeneration: sheet.claimGeneration,
      correlationID: "11111111-1111-4111-8111-111111111111"
    )
  }

  func snapshot(limit: Int) async throws -> LocalProductSnapshot {
    .empty(viewVersion: "view-2")
  }

  func timeline(
    teamInstanceID: String,
    cursor: String,
    limit: Int
  ) async throws -> LocalProductTimelinePage {
    throw LocalProductClientError.notFound
  }

  func readMissionDecision(
    _ command: LocalProductDecisionCommand
  ) async throws -> LocalProductDecisionSheet {
    readCount += 1
    lastCommand = command
    return sheet
  }

  func decideMission(
    _ command: LocalProductDecisionCommand
  ) async throws -> LocalProductDecisionResult {
    submitCount += 1
    lastCommand = command
    let authoritative = command.operation == "submit" && authoritativeSubmit
    let status = authoritative ? "accepted" : "pending"
    return try LocalProductDecisionWire.decodeResult(
      Data(
        """
        {
          "schema_version":1,
          "mission_id":"mission/team-1",
          "decision_id":"\(resultDecisionID)",
          "status":"\(status)",
          "authoritative":\(authoritative),
          "view_version":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
        }
        """.utf8
      )
    )
  }
}

final class ProviderSetupStubClient:
  LocalProductClientProtocol,
  LocalProductSetupClientProtocol
{
  private let disconnected: LocalProductSetupSnapshot
  private let connected: LocalProductSetupSnapshot
  private let connectsAfterStart: Bool
  private let miniMaxConfigured: Bool
  private let mismatchedMiniMaxRefresh: Bool
  private let miniMaxTerminalStatus: String
  private let miniMaxTerminalReason: String
  private let miniMaxVerifyError: LocalIPCRemoteError?
  private let materializesTeam: Bool
  private let builderConfirmError: LocalIPCRemoteError?
  private let deepSeekEnabled: Bool
  private let deepSeekConfigureError: LocalIPCRemoteError?
  private let vaultRotationError: LocalIPCRemoteError?
  private let vaultLockError: LocalIPCRemoteError?
  private let vaultUnlockError: LocalIPCRemoteError?
  private let vaultResetError: LocalIPCRemoteError?
  private let vaultExportError: LocalIPCRemoteError?
  private let providerPolicyError: LocalIPCRemoteError?
  private let providerRateCardError: LocalIPCRemoteError?
  private let remoteToolConfigureError: LocalIPCRemoteError?
  private var deepSeekNeedsMigration: Bool
  private var builderConfirmed = false
  private var deepSeekRevision: Int64 = 0
  private var deepSeekAccountID = "deepseek.primary"
  private var providerPolicyRevision: Int64 = 0
  private var providerPolicyTrustDomain = ""
  private var providerPolicyRetentionMode = ""
  private var providerPolicyDataRegion = ""
  private var providerRateCardRevision: Int64 = 0
  private var providerRateCardInputBasis = "input_excludes_cache"
  private var providerRateCardInput: Int64 = 0
  private var providerRateCardOutput: Int64 = 0
  private var providerRateCardCacheRead: Int64 = 0
  private var providerRateCardCacheWrite: Int64 = 0
  private var remoteToolEnrollmentRevision: Int64
  private var remoteToolEnrollmentStatus: String
  private(set) var setupRequestCount = 0
  private(set) var productSnapshotRequestCount = 0
  private(set) var connectRequestCount = 0
  private(set) var verifyRequestCount = 0
  private(set) var builderConfirmRequestCount = 0
  private(set) var builderEdits: [(field: String, value: String)] = []
  private(set) var targetedBuilderEdits: [(
    field: String,
    value: String,
    roleAgentDefinitionID: String
  )] = []
  private(set) var genericConfigureProviderIDs: [String] = []
  private(set) var genericVerifyProviderIDs: [String] = []
  private(set) var genericConfigureAccountIDs: [String] = []
  private(set) var genericVerifyAccountIDs: [String] = []
  private(set) var genericReplaceProviderIDs: [String] = []
  private(set) var genericReplaceAccountIDs: [String] = []
  private(set) var vaultRotationRequestCount = 0
  private(set) var vaultLockRequestCount = 0
  private(set) var vaultUnlockRequestCount = 0
  private(set) var vaultResetRequestCount = 0
  private(set) var vaultExportRequestCount = 0
  private(set) var vaultExportDestinations: [String] = []
  private(set) var providerPolicyRequests: [(
    providerID: String,
    providerAccountID: String,
    expectedRevision: Int64,
    maximumConcurrentAttempts: Int,
    dispatchWindowSeconds: Int64,
    maximumDispatchStarts: Int,
    maximumAssignedBudgetUnits: Int64,
    trustDomain: String,
    retentionMode: String,
    dataRegion: String
  )] = []
  private(set) var providerRateCardRequests: [(
    providerID: String,
    providerAccountID: String,
    modelID: String,
    expectedRevision: Int64
  )] = []
  private(set) var remoteToolConfigureRequests: [
    LocalProductRemoteToolBackendEnrollmentCommand
  ] = []
  private(set) var remoteToolRevokeRequests: [
    LocalProductRemoteToolBackendEnrollmentRevokeCommand
  ] = []

  init(
    connectsAfterStart: Bool = true,
    miniMaxConfigured: Bool = false,
    mismatchedMiniMaxRefresh: Bool = false,
    miniMaxTerminalStatus: String = "verified",
    miniMaxTerminalReason: String = "",
    miniMaxVerifyError: LocalIPCRemoteError? = nil,
    materializesTeam: Bool = false,
    builderConfirmError: LocalIPCRemoteError? = nil,
    deepSeekEnabled: Bool = false,
    deepSeekConfigureError: LocalIPCRemoteError? = nil,
    deepSeekMigrationRequired: Bool = false,
    vaultRotationError: LocalIPCRemoteError? = nil,
    vaultLockError: LocalIPCRemoteError? = nil,
    vaultUnlockError: LocalIPCRemoteError? = nil,
    vaultResetError: LocalIPCRemoteError? = nil,
    vaultExportError: LocalIPCRemoteError? = nil,
    providerPolicyError: LocalIPCRemoteError? = nil,
    providerRateCardError: LocalIPCRemoteError? = nil,
    remoteToolEnrollmentConfigured: Bool = false,
    remoteToolConfigureError: LocalIPCRemoteError? = nil
  ) throws {
    disconnected = try Self.snapshot(
      codexStatus: "not_logged_in",
      miniMaxStatus: miniMaxConfigured ? "configured" : "unconfigured",
      miniMaxRevision: miniMaxConfigured ? 1 : 0
    )
    connected = try Self.snapshot(
      codexStatus: "available",
      miniMaxStatus: miniMaxConfigured ? "configured" : "unconfigured",
      miniMaxRevision: miniMaxConfigured ? 1 : 0
    )
    self.connectsAfterStart = connectsAfterStart
    self.miniMaxConfigured = miniMaxConfigured
    self.mismatchedMiniMaxRefresh = mismatchedMiniMaxRefresh
    self.miniMaxTerminalStatus = miniMaxTerminalStatus
    self.miniMaxTerminalReason = miniMaxTerminalReason
    self.miniMaxVerifyError = miniMaxVerifyError
    self.materializesTeam = materializesTeam
    self.builderConfirmError = builderConfirmError
    self.deepSeekEnabled = deepSeekEnabled
    self.deepSeekConfigureError = deepSeekConfigureError
    self.vaultRotationError = vaultRotationError
    self.vaultLockError = vaultLockError
    self.vaultUnlockError = vaultUnlockError
    self.vaultResetError = vaultResetError
    self.vaultExportError = vaultExportError
    self.providerPolicyError = providerPolicyError
    self.providerRateCardError = providerRateCardError
    self.remoteToolConfigureError = remoteToolConfigureError
    remoteToolEnrollmentRevision = remoteToolEnrollmentConfigured ? 1 : 0
    remoteToolEnrollmentStatus = remoteToolEnrollmentConfigured ? "active" : ""
    if remoteToolEnrollmentConfigured {
      providerPolicyRevision = 3
      providerPolicyTrustDomain = "external_provider"
      providerPolicyRetentionMode = "zero_data_retention"
      providerPolicyDataRegion = "apac"
    }
    deepSeekNeedsMigration = deepSeekMigrationRequired
    if deepSeekMigrationRequired { deepSeekRevision = 3 }
  }

  func snapshot(limit: Int) async throws -> LocalProductSnapshot {
    productSnapshotRequestCount += 1
    guard materializesTeam, builderConfirmed else {
      return .empty(viewVersion: "view-1")
    }
    let json = LocalProductModelsTests.snapshotJSON.replacingOccurrences(
      of: "\"teams\":[]",
      with: """
        "teams":[{
          "team_instance_id":"team-instance-fixture",
          "display_name":"Controlled Team",
          "source_kind":"saved_team",
          "state":"created",
          "confirmed":true,
          "executable":true,
          "read_only":false
        }]
        """
    )
    return try LocalProductWire.decodeSnapshot(Data(json.utf8))
  }

  func timeline(
    teamInstanceID: String,
    cursor: String,
    limit: Int
  ) async throws -> LocalProductTimelinePage {
    throw LocalProductClientError.notFound
  }

  func setupSnapshot() async throws -> LocalProductSetupSnapshot {
    setupRequestCount += 1
    if deepSeekEnabled {
      return try Self.snapshot(
        codexStatus: "not_logged_in",
        deepSeekRevision: deepSeekRevision,
        deepSeekAccountID: deepSeekAccountID,
        deepSeekStatus: deepSeekNeedsMigration ? "migration_required" : nil,
        providerPolicyRevision: providerPolicyRevision,
        providerPolicyTrustDomain: providerPolicyTrustDomain,
        providerPolicyRetentionMode: providerPolicyRetentionMode,
        providerPolicyDataRegion: providerPolicyDataRegion,
        providerRateCardRevision: providerRateCardRevision,
        providerRateCardInputBasis: providerRateCardInputBasis,
        providerRateCardInput: providerRateCardInput,
        providerRateCardOutput: providerRateCardOutput,
        providerRateCardCacheRead: providerRateCardCacheRead,
        providerRateCardCacheWrite: providerRateCardCacheWrite,
        remoteToolEnrollmentRevision: remoteToolEnrollmentRevision,
        remoteToolEnrollmentStatus: remoteToolEnrollmentStatus
      )
    }
    if miniMaxConfigured {
      return try Self.snapshot(
        codexStatus: "not_logged_in",
        miniMaxStatus: verifyRequestCount == 0
          ? "configured"
          : miniMaxTerminalStatus,
        miniMaxRevision: verifyRequestCount == 0
          ? 1
          : (mismatchedMiniMaxRefresh ? 3 : 2),
        miniMaxReason: verifyRequestCount == 0
          ? ""
          : miniMaxTerminalReason
      )
    }
    return setupRequestCount == 1 || !connectsAfterStart
      ? disconnected
      : connected
  }

  func configureProviderAccountPolicy(
    providerID: String,
    providerAccountID: String,
    expectedRevision: Int64,
    maximumConcurrentAttempts: Int,
    dispatchWindowSeconds: Int64,
    maximumDispatchStarts: Int,
    maximumAssignedBudgetUnits: Int64,
    trustDomain: String,
    retentionMode: String,
    dataRegion: String
  ) async throws -> LocalProductProviderAccountPolicyResult {
    providerPolicyRequests.append((
      providerID,
      providerAccountID,
      expectedRevision,
      maximumConcurrentAttempts,
      dispatchWindowSeconds,
      maximumDispatchStarts,
      maximumAssignedBudgetUnits,
      trustDomain,
      retentionMode,
      dataRegion
    ))
    if let providerPolicyError { throw providerPolicyError }
    guard deepSeekEnabled, providerID == "deepseek",
      providerAccountID == deepSeekAccountID,
      expectedRevision == providerPolicyRevision
    else {
      throw LocalProductClientError.invalidRequest
    }
    providerPolicyRevision += 1
    providerPolicyTrustDomain = trustDomain
    providerPolicyRetentionMode = retentionMode
    providerPolicyDataRegion = dataRegion
    return try JSONDecoder().decode(
      LocalProductProviderAccountPolicyResult.self,
      from: Data(
        """
        {"policy_available":true,"policy_version":2,"provider_id":"deepseek","provider_account_id":"\(deepSeekAccountID)","revision":\(providerPolicyRevision),"policy_digest":"\(String(repeating: "b", count: 64))","maximum_concurrent_attempts":\(maximumConcurrentAttempts),"dispatch_window_seconds":\(dispatchWindowSeconds),"maximum_dispatch_starts":\(maximumDispatchStarts),"maximum_assigned_budget_units":\(maximumAssignedBudgetUnits),"trust_domain":"\(trustDomain)","retention_mode":"\(retentionMode)","data_region":"\(dataRegion)","configured_at":"2026-08-11T05:00:00Z"}
        """.utf8
      )
    )
  }

  func configureProviderModelRateCard(
    providerID: String,
    providerAccountID: String,
    modelID: String,
    expectedRevision: Int64,
    currency: String,
    inputTokenBasis: String,
    inputMicrounitsPerMillion: Int64,
    outputMicrounitsPerMillion: Int64,
    cacheReadMicrounitsPerMillion: Int64,
    cacheWriteMicrounitsPerMillion: Int64
  ) async throws -> LocalProductProviderModelRateCardResult {
    providerRateCardRequests.append((
      providerID, providerAccountID, modelID, expectedRevision
    ))
    if let providerRateCardError { throw providerRateCardError }
    guard deepSeekEnabled, providerID == "deepseek",
      providerAccountID == deepSeekAccountID, modelID == "deepseek-chat",
      expectedRevision == providerRateCardRevision, currency == "USD"
    else { throw LocalProductClientError.invalidRequest }
    providerRateCardRevision += 1
    providerRateCardInputBasis = inputTokenBasis
    providerRateCardInput = inputMicrounitsPerMillion
    providerRateCardOutput = outputMicrounitsPerMillion
    providerRateCardCacheRead = cacheReadMicrounitsPerMillion
    providerRateCardCacheWrite = cacheWriteMicrounitsPerMillion
    return try JSONDecoder().decode(
      LocalProductProviderModelRateCardResult.self,
      from: Data(
        """
        {"provider_id":"deepseek","provider_account_id":"\(deepSeekAccountID)","model_id":"deepseek-chat","revision":\(providerRateCardRevision),"rate_card_digest":"\(String(repeating: "c", count: 64))","currency":"USD","input_token_basis":"\(inputTokenBasis)","input_microunits_per_million":\(inputMicrounitsPerMillion),"output_microunits_per_million":\(outputMicrounitsPerMillion),"cache_read_microunits_per_million":\(cacheReadMicrounitsPerMillion),"cache_write_microunits_per_million":\(cacheWriteMicrounitsPerMillion),"rounding_mode":"ceiling_per_attempt","configured_at":"2026-08-12T05:00:00Z"}
        """.utf8
      )
    )
  }

  func configureRemoteToolBackend(
    _ command: LocalProductRemoteToolBackendEnrollmentCommand
  ) async throws -> LocalProductRemoteToolBackendEnrollmentResult {
    remoteToolConfigureRequests.append(command)
    if let remoteToolConfigureError { throw remoteToolConfigureError }
    guard command.providerID == "deepseek",
      command.providerAccountID == deepSeekAccountID,
      command.expectedRevision == remoteToolEnrollmentRevision
    else { throw LocalProductClientError.invalidRequest }
    remoteToolEnrollmentRevision += 1
    remoteToolEnrollmentStatus = "active"
    return try remoteToolResult(command: command)
  }

  func revokeRemoteToolBackend(
    _ command: LocalProductRemoteToolBackendEnrollmentRevokeCommand
  ) async throws -> LocalProductRemoteToolBackendEnrollmentResult {
    remoteToolRevokeRequests.append(command)
    guard command.providerID == "deepseek",
      command.providerAccountID == deepSeekAccountID,
      command.expectedRevision == remoteToolEnrollmentRevision,
      remoteToolEnrollmentRevision > 0
    else { throw LocalProductClientError.invalidRequest }
    remoteToolEnrollmentRevision += 1
    remoteToolEnrollmentStatus = "revoked"
    return try remoteToolResult(
      command: .fixture(expectedRevision: command.expectedRevision)
    )
  }

  private func remoteToolResult(
    command: LocalProductRemoteToolBackendEnrollmentCommand
  ) throws -> LocalProductRemoteToolBackendEnrollmentResult {
    try JSONDecoder().decode(
      LocalProductRemoteToolBackendEnrollmentResult.self,
      from: Data(
        """
        {"enrollment_available":true,"enrollment_version":1,"enrollment_id":"\(command.enrollmentID)","backend_kind":"\(command.backendKind)","adapter_id":"\(command.adapterID)","provider_id":"\(command.providerID)","provider_account_id":"\(command.providerAccountID)","provider_account_policy_version":\(command.providerAccountPolicyVersion),"provider_account_policy_revision":\(command.providerAccountPolicyRevision),"provider_account_policy_digest":"\(command.providerAccountPolicyDigest)","policy_current":true,"endpoint_fingerprint":"\(command.endpointFingerprint)","mcp_server_id":"\(command.mcpServerID)","allowed_tools":["get_issue","search_docs"],"revision":\(remoteToolEnrollmentRevision),"status":"\(remoteToolEnrollmentStatus)","maximum_concurrent_calls":\(command.maximumConcurrentCalls),"maximum_calls_per_attempt":\(command.maximumCallsPerAttempt),"timeout_seconds":\(command.timeoutSeconds),"maximum_result_bytes":\(command.maximumResultBytes),"maximum_budget_units":\(command.maximumBudgetUnits),"configured_at":"2026-08-15T05:00:00Z","enrollment_digest":"\(String(repeating: remoteToolEnrollmentStatus == "active" ? "d" : "e", count: 64))"}
        """.utf8
      )
    )
  }

  func rotateCredentialVault() async throws {
    vaultRotationRequestCount += 1
    if let vaultRotationError { throw vaultRotationError }
  }

  func lockCredentialVault() async throws {
    vaultLockRequestCount += 1
    if let vaultLockError { throw vaultLockError }
  }

  func unlockCredentialVault() async throws {
    vaultUnlockRequestCount += 1
    if let vaultUnlockError { throw vaultUnlockError }
  }

  func resetCredentialVault() async throws {
    vaultResetRequestCount += 1
    if let vaultResetError { throw vaultResetError }
  }

  func exportCredentialVault(
    passphrase: String,
    destination: String
  ) async throws -> LocalProductCredentialVaultExportResult {
    vaultExportRequestCount += 1
    vaultExportDestinations.append(destination)
    if let vaultExportError { throw vaultExportError }
    return try JSONDecoder().decode(
      LocalProductCredentialVaultExportResult.self,
      from: Data(
        """
        {
          "schema_version":1,
          "file_path":"\(destination)",
          "digest":"\(String(repeating: "a", count: 64))",
          "credential_count":2
        }
        """.utf8
      )
    )
  }

  func connectCodex() async throws -> LocalProductProviderConnectResult {
    connectRequestCount += 1
    return .fixture(status: "started")
  }

  func startBuilder(
    source: String,
    sourceID: String,
    sourceVersion: Int,
    sourceDigest: String
  ) async throws -> LocalProductBuilderSession {
    guard materializesTeam else {
      throw LocalProductClientError.unavailable
    }
    return try LocalProductSetupWire.decodeBuilderSession(
      Data(
        """
        {
          "schema_version":1,"draft_id":"draft-fixture","revision":4,
          "source":"blank","catalog_digest":"\(String(repeating: "b", count: 64))",
          "view_version":"\(String(repeating: "e", count: 64))",
          "content_digest":"\(String(repeating: "c", count: 64))",
          "binding_digest":"\(String(repeating: "d", count: 64))",
          "question":{"id":"","prompt":"","options":[]},
          "preview":{"name":"Controlled Team","purpose":"One bounded Mission",
            "roles":[],"permissions":[],"resources":[],"compatibility_gaps":[],
            "requested_concurrency":1,"maximum_budget_credits":100,
            "estimated_maximum_cost":"up to 100 credits"},
          "can_confirm":true
        }
        """.utf8
      )
    )
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
    builderEdits.append((field, value))
    return session
  }

  func editBuilder(
    session: LocalProductBuilderSession,
    field: String,
    value: String,
    roleAgentDefinitionID: String
  ) async throws -> LocalProductBuilderSession {
    targetedBuilderEdits.append((field, value, roleAgentDefinitionID))
    return session
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
    builderConfirmRequestCount += 1
    if let builderConfirmError {
      throw builderConfirmError
    }
    guard materializesTeam, session.canConfirm else {
      throw LocalProductClientError.unavailable
    }
    builderConfirmed = true
    return try LocalProductSetupWire.decodeBuilderConfirmation(
      Data(
        """
        {"team_definition_id":"team-fixture","team_definition_version":1,
         "team_definition_digest":"\(String(repeating: "f", count: 64))",
         "status":"active","team_instance_created":true,"run_created":false}
        """.utf8
      )
    )
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

  func configureCredential(
    providerID: String,
    secret: String
  ) async throws -> LocalProductCredentialSetupResult {
    genericConfigureProviderIDs.append(providerID)
    if let deepSeekConfigureError { throw deepSeekConfigureError }
    guard deepSeekEnabled, providerID == "deepseek", secret == "sk-test" else {
      throw LocalProductClientError.invalidRequest
    }
    deepSeekRevision = 1
    return try LocalProductSetupWire.decodeCredentialResult(
      Data(
        #"{"provider_id":"deepseek","revision":1,"status":"configured","reason":""}"#.utf8
      ))
  }

  func configureCredential(
    providerID: String,
    providerAccountID: String,
    secret: String
  ) async throws -> LocalProductCredentialSetupResult {
    genericConfigureAccountIDs.append(providerAccountID)
    guard deepSeekEnabled, providerID == "deepseek", secret == "sk-test",
      LocalIPCClient.validProviderAccountID(
        providerAccountID,
        providerID: providerID
      )
    else {
      throw LocalProductClientError.invalidRequest
    }
    deepSeekAccountID = providerAccountID
    deepSeekRevision = 1
    return try LocalProductSetupWire.decodeCredentialResult(
      Data(
        #"{"provider_id":"deepseek","revision":1,"status":"configured","reason":""}"#.utf8
      ))
  }

  func verifyCredential(
    providerID: String,
    reference: String,
    revision: Int64
  ) async throws -> LocalProductCredentialSetupResult {
    genericVerifyProviderIDs.append(providerID)
    guard deepSeekEnabled, providerID == "deepseek",
      reference == "credential-ref-deepseek-1",
      revision == deepSeekRevision, revision > 0
    else {
      throw LocalProductClientError.invalidRequest
    }
    deepSeekRevision = revision + 1
    deepSeekNeedsMigration = false
    return try LocalProductSetupWire.decodeCredentialResult(
      Data(
        "{\"provider_id\":\"deepseek\",\"revision\":\(deepSeekRevision),\"status\":\"verified\",\"reason\":\"\"}"
          .utf8
      ))
  }

  func verifyCredential(
    providerID: String,
    providerAccountID: String,
    reference: String,
    revision: Int64
  ) async throws -> LocalProductCredentialSetupResult {
    genericVerifyAccountIDs.append(providerAccountID)
    guard deepSeekEnabled, providerID == "deepseek",
      providerAccountID == deepSeekAccountID,
      reference == "credential-ref-deepseek-1",
      revision == deepSeekRevision, revision > 0
    else {
      throw LocalProductClientError.invalidRequest
    }
    deepSeekRevision = revision + 1
    deepSeekNeedsMigration = false
    return try LocalProductSetupWire.decodeCredentialResult(
      Data(
        "{\"provider_id\":\"deepseek\",\"revision\":\(deepSeekRevision),\"status\":\"verified\",\"reason\":\"\"}"
          .utf8
      ))
  }

  func replaceCredential(
    providerID: String,
    reference: String,
    revision: Int64,
    secret: String
  ) async throws -> LocalProductCredentialSetupResult {
    genericReplaceProviderIDs.append(providerID)
    guard deepSeekEnabled, providerID == "deepseek",
      reference == "credential-ref-deepseek-1",
      revision == deepSeekRevision, secret == "sk-test"
    else {
      throw LocalProductClientError.invalidRequest
    }
    deepSeekRevision = revision + 1
    deepSeekNeedsMigration = false
    return try LocalProductSetupWire.decodeCredentialResult(
      Data(
        "{\"provider_id\":\"deepseek\",\"revision\":\(deepSeekRevision),\"status\":\"configured\",\"reason\":\"\"}"
          .utf8
      ))
  }

  func replaceCredential(
    providerID: String,
    providerAccountID: String,
    reference: String,
    revision: Int64,
    secret: String
  ) async throws -> LocalProductCredentialSetupResult {
    genericReplaceAccountIDs.append(providerAccountID)
    guard providerAccountID == deepSeekAccountID else {
      throw LocalProductClientError.invalidRequest
    }
    return try await replaceCredential(
      providerID: providerID,
      reference: reference,
      revision: revision,
      secret: secret
    )
  }

  func verifyMiniMax(
    reference: String,
    revision: Int64
  ) async throws -> LocalProductCredentialSetupResult {
    guard miniMaxConfigured,
      reference == "credential-ref-1",
      revision == 1
    else {
      throw LocalProductClientError.invalidRequest
    }
    verifyRequestCount += 1
    if let miniMaxVerifyError {
      throw miniMaxVerifyError
    }
    return try LocalProductSetupWire.decodeCredentialResult(
      Data(
        """
        {
          "provider_id": "minimax",
          "revision": 2,
          "status": "\(miniMaxTerminalStatus)",
          "reason": "\(miniMaxTerminalReason)"
        }
        """.utf8
      )
    )
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

  private static func snapshot(
    codexStatus: String,
    miniMaxStatus: String = "unconfigured",
    miniMaxRevision: Int64 = 0,
    miniMaxReason: String = "",
    deepSeekRevision: Int64 = 0,
    deepSeekAccountID: String = "deepseek.primary",
    deepSeekStatus: String? = nil,
    providerPolicyRevision: Int64 = 0,
    providerPolicyTrustDomain: String = "",
    providerPolicyRetentionMode: String = "",
    providerPolicyDataRegion: String = "",
    providerRateCardRevision: Int64 = 0,
    providerRateCardInputBasis: String = "input_excludes_cache",
    providerRateCardInput: Int64 = 0,
    providerRateCardOutput: Int64 = 0,
    providerRateCardCacheRead: Int64 = 0,
    providerRateCardCacheWrite: Int64 = 0,
    remoteToolEnrollmentRevision: Int64 = 0,
    remoteToolEnrollmentStatus: String = ""
  ) throws -> LocalProductSetupSnapshot {
    let rateCardsJSON = providerRateCardRevision > 0
      ? """
        [{"model_id":"deepseek-chat","revision":\(providerRateCardRevision),"rate_card_digest":"\(String(repeating: "c", count: 64))","currency":"USD","input_token_basis":"\(providerRateCardInputBasis)","input_microunits_per_million":\(providerRateCardInput),"output_microunits_per_million":\(providerRateCardOutput),"cache_read_microunits_per_million":\(providerRateCardCacheRead),"cache_write_microunits_per_million":\(providerRateCardCacheWrite),"rounding_mode":"ceiling_per_attempt","configured_at":"2026-08-12T05:00:00Z"}]
        """
      : "[]"
    let remoteToolsJSON = remoteToolEnrollmentRevision > 0
      ? """
        [{"enrollment_version":1,"enrollment_id":"mcp-deepseek-work","backend_kind":"mcp_server","adapter_id":"builtin.mcp.stdio.v1","provider_account_policy_version":2,"provider_account_policy_revision":3,"provider_account_policy_digest":"\(String(repeating: "b", count: 64))","policy_current":true,"endpoint_fingerprint":"\(String(repeating: "c", count: 64))","mcp_server_id":"work-tools","allowed_tools":["get_issue","search_docs"],"revision":\(remoteToolEnrollmentRevision),"status":"\(remoteToolEnrollmentStatus)","maximum_concurrent_calls":2,"maximum_calls_per_attempt":4,"timeout_seconds":30,"maximum_result_bytes":32768,"maximum_budget_units":2000,"configured_at":"2026-08-15T05:00:00Z","enrollment_digest":"\(String(repeating: remoteToolEnrollmentStatus == "active" ? "d" : "e", count: 64))"}]
        """
      : "[]"
    let providerAccountsJSON = deepSeekRevision > 0
      ? """
        [{"provider_id":"deepseek","provider_account_id":"\(deepSeekAccountID)","auth_mode":"brokered","credential_reference":"credential-ref-deepseek-1","revision":\(deepSeekRevision),"status":"\(deepSeekStatus ?? (deepSeekRevision == 1 ? "configured" : "verified"))","reason":"\(deepSeekStatus == "migration_required" ? "vault_entry_missing" : "")","policy_available":\(providerPolicyRevision > 0),"policy_version":\(providerPolicyRevision > 0 ? 2 : 0),"policy_revision":\(providerPolicyRevision),"policy_digest":"\(providerPolicyRevision > 0 ? String(repeating: "b", count: 64) : "")","maximum_concurrent_attempts":\(providerPolicyRevision > 0 ? 4 : 0),"dispatch_window_seconds":\(providerPolicyRevision > 0 ? 60 : 0),"maximum_dispatch_starts":\(providerPolicyRevision > 0 ? 20 : 0),"maximum_assigned_budget_units":\(providerPolicyRevision > 0 ? 12000 : 0),"trust_domain":"\(providerPolicyTrustDomain)","retention_mode":"\(providerPolicyRetentionMode)","data_region":"\(providerPolicyDataRegion)","rate_cards":\(rateCardsJSON),"remote_tool_backends":\(remoteToolsJSON)}]
        """
      : "[]"
    return try LocalProductSetupWire.decodeSnapshot(
      Data(
        """
        {
          "schema_version": 1,
          "view_version": "\(String(repeating: "a", count: 64))",
          "codex": {
            "provider_id": "codex",
            "auth_mode": "native_auth",
            "credential_reference": "",
            "revision": 0,
            "status": "\(codexStatus)",
            "reason": "\(codexStatus == "available" ? "" : "not_logged_in")"
          },
          "minimax": {
            "provider_id": "minimax",
            "auth_mode": "brokered",
            "credential_reference": "\(miniMaxRevision > 0 ? "credential-ref-1" : "")",
            "revision": \(miniMaxRevision),
            "status": "\(miniMaxStatus)",
            "reason": "\(miniMaxReason)"
          },
          "providers": [
            {
              "provider_id":"deepseek","display_name":"DeepSeek",
              "category":"official","protocol":"openai_compatible",
              "auth_mode":"brokered","connection_kind":"api_key",
              "credential_reference":"\(deepSeekAccountID == "deepseek.primary" && deepSeekRevision > 0 ? "credential-ref-deepseek-1" : "")",
              "revision":\(deepSeekAccountID == "deepseek.primary" ? deepSeekRevision : 0),
              "status":"\(deepSeekStatus ?? (deepSeekAccountID != "deepseek.primary" || deepSeekRevision == 0 ? "unconfigured" : (deepSeekRevision == 1 ? "configured" : "verified")))",
              "reason":"\(deepSeekStatus == "migration_required" ? "vault_entry_missing" : "")","supports_model_discovery":true
            }
          ],
          "provider_accounts": \(providerAccountsJSON),
          "conversation_profiles": [],
          "runtimes": [],
          "saved_teams": [],
          "templates": [],
          "role_options": [],
          "skills": [],
          "permissions": [],
          "resources": []
        }
        """.utf8
      )
    )
  }
}

private extension LocalProductRemoteToolBackendEnrollmentCommand {
  static func fixture(expectedRevision: Int64) -> Self {
    Self(
      enrollmentID: "mcp-deepseek-work",
      backendKind: "mcp_server",
      adapterID: "builtin.mcp.stdio.v1",
      providerID: "deepseek",
      providerAccountID: "deepseek.work",
      providerAccountPolicyVersion: 2,
      providerAccountPolicyRevision: 3,
      providerAccountPolicyDigest: String(repeating: "b", count: 64),
      endpointFingerprint: String(repeating: "c", count: 64),
      mcpServerID: "work-tools",
      allowedTools: ["get_issue", "search_docs"],
      expectedRevision: expectedRevision,
      maximumConcurrentCalls: 2,
      maximumCallsPerAttempt: 4,
      timeoutSeconds: 30,
      maximumResultBytes: 32_768,
      maximumBudgetUnits: 2_000
    )
  }
}

extension LocalProductProviderConnectResult {
  fileprivate static func fixture(status: String) -> Self {
    try! LocalProductSetupWire.decodeProviderConnectResult(
      Data(
        """
        {
          "provider_id": "codex",
          "auth_mode": "native_auth",
          "status": "\(status)"
        }
        """.utf8
      )
    )
  }
}
