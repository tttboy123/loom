import AppKit
import LoomLocalAppCore
import SwiftUI
import UniformTypeIdentifiers

func missionRuntimeDisplayName(
  run: LocalProductRunSummary,
  snapshot: LocalProductSnapshot
) -> String {
  guard
    let runtime = snapshot.runtimes.first(where: {
      $0.runtimeInstanceID == run.runtimeInstanceID
    })
  else {
    return "Runtime unavailable"
  }
  return LocalProductExperience.visibleName(
    runtime.displayName,
    internalID: runtime.runtimeInstanceID,
    fallback: "Runtime unavailable"
  )
}

func runtimeAgentAvailabilityDetail(
  adapterType: String,
  modelIDs: [String],
  roleOptionCount: Int
) -> String {
  if roleOptionCount > 0 {
    let models = modelIDs.joined(separator: ", ")
    return models.isEmpty
      ? "Available to Agent Teams"
      : "\(models) · Available to Agent Teams"
  }
  switch adapterType {
  case "codex":
    return "Detected · Connect OpenAI in Model Providers to use with Agent Teams"
  case "claude-code":
    return "Detected · Connect Anthropic in Model Providers to use with Agent Teams"
  default:
    let models = modelIDs.joined(separator: ", ")
    return models.isEmpty ? "Detected" : models
  }
}

func missionHistoryIncludesMission(
  lane: MissionLane,
  hideCompleted: Bool,
  teamExecutable: Bool
) -> Bool {
  // Team readiness governs new execution, never the visibility of an existing
  // Mission. The parameter makes that independence explicit at the call site.
  _ = teamExecutable
  return !(hideCompleted && lane == .complete)
}

func missionRuntimeRecoveryReason(
  hasExecutableTeam: Bool,
  executionReachable: Bool,
  setupState: LocalProductSetupState
) -> String? {
  guard executionReachable else {
    return "Mission preflight cannot reach Loom's local execution service. Open Runtime & Providers to refresh local service readiness, then retry."
  }
  guard !hasExecutableTeam else { return nil }
  switch setupState {
  case .idle, .loading:
    return "Runtime and Provider readiness is still loading. Open Runtime & Providers to refresh the local setup service, then retry."
  case .unavailable, .fatal:
    return "Loom's local setup service cannot confirm an executable Team. Open Runtime & Providers, choose Try Again, then retry preflight."
  case .ready:
    return "No confirmed Team currently has executable Runtime and Provider bindings. Open Runtime & Providers to reconnect them, then retry."
  }
}

struct MissionSetupRecoveryPresentation: Equatable {
  let title: String
  let detail: String
  let isLoading: Bool
}

func missionSetupRecoveryPresentation(
  _ state: LocalProductSetupState
) -> MissionSetupRecoveryPresentation {
  switch state {
  case .idle, .loading:
    return MissionSetupRecoveryPresentation(
      title: "Loading Runtime & Providers",
      detail: "Waiting for Loom's local setup service.",
      isLoading: true
    )
  case .unavailable(let reason):
    if reason == "empty_setup" {
      return MissionSetupRecoveryPresentation(
        title: "Setup inventory is rebuilding",
        detail: "The local setup service returned an incomplete inventory. Choose Try Again to recover Runtime and Provider readiness.",
        isLoading: false
      )
    }
    return MissionSetupRecoveryPresentation(
      title: "Runtime & Providers unavailable",
      detail: "Loom's local setup service did not return a setup snapshot. Choose Try Again to reconnect and reload it.",
      isLoading: false
    )
  case .fatal:
    return MissionSetupRecoveryPresentation(
      title: "Runtime & Providers need recovery",
      detail: "The local setup service could not load setup state. Restart the local Loom service, then choose Try Again.",
      isLoading: false
    )
  case .ready:
    return MissionSetupRecoveryPresentation(
      title: "Setup inventory is rebuilding",
      detail: "Runtime and Provider setup is not available yet. Choose Try Again to reload it.",
      isLoading: false
    )
  }
}

func providerAccountIdentifier(providerID: String, name: String) -> String? {
  var slug = ""
  var pendingSeparator = false
  for scalar in name.trimmingCharacters(in: .whitespacesAndNewlines)
    .lowercased().unicodeScalars
  {
    let isAlphaNumeric =
      scalar.value >= 97 && scalar.value <= 122 || scalar.value >= 48 && scalar.value <= 57
    if isAlphaNumeric {
      if pendingSeparator && !slug.isEmpty { slug.append("-") }
      slug.unicodeScalars.append(scalar)
      pendingSeparator = false
    } else if !slug.isEmpty {
      pendingSeparator = true
    }
  }
  guard !slug.isEmpty else { return nil }
  let accountID = providerID + "." + slug
  return LocalIPCClient.validProviderAccountID(accountID, providerID: providerID)
    ? accountID
    : nil
}

func providerAccountDisplayName(_ accountID: String, providerID: String) -> String {
  guard accountID.hasPrefix(providerID + ".") else { return "Account" }
  let suffix = accountID.dropFirst(providerID.count + 1)
  return
    suffix
    .replacingOccurrences(of: ".", with: " ")
    .replacingOccurrences(of: "-", with: " ")
    .capitalized
}

private struct ProviderCredentialViewState {
  let reference: String
  let revision: Int64
  let status: String
}

private struct CredentialVaultExportSheet: View {
  @ObservedObject var store: LocalProductStore
  @Environment(\.dismiss) private var dismiss
  @State private var passphrase = ""
  @State private var confirmation = ""
  @State private var errorMessage: String?

  private var canExport: Bool {
    let count = passphrase.utf8.count
    return count >= 12 && count <= 1_024 && passphrase == confirmation
      && !store.isExportingCredentialVault
  }

  var body: some View {
    VStack(alignment: .leading, spacing: 18) {
      HStack {
        VStack(alignment: .leading, spacing: 3) {
          Text("Export encrypted backup")
            .font(.title2.weight(.semibold))
          Text("Credential Vault")
            .foregroundStyle(.secondary)
        }
        Spacer()
        Button {
          dismiss()
        } label: {
          Image(systemName: "xmark")
        }
        .buttonStyle(.plain)
        .accessibilityLabel("Close encrypted backup")
      }
      Divider()
      SecureField("Backup passphrase", text: $passphrase)
        .textFieldStyle(.roundedBorder)
      SecureField("Confirm passphrase", text: $confirmation)
        .textFieldStyle(.roundedBorder)
      Text("The backup cannot be recovered without this passphrase.")
        .font(.caption)
        .foregroundStyle(.secondary)
      if let errorMessage {
        Text(errorMessage)
          .font(.caption)
          .foregroundStyle(LoomGraphite.statusDanger)
          .textSelection(.enabled)
      }
      Spacer()
      HStack {
        Spacer()
        Button("Cancel", role: .cancel) {
          dismiss()
        }
        Button {
          chooseDestinationAndExport()
        } label: {
          if store.isExportingCredentialVault {
            ProgressView().controlSize(.small)
          } else {
            Label("Export", systemImage: "square.and.arrow.up")
          }
        }
        .buttonStyle(.borderedProminent)
        .disabled(!canExport)
      }
    }
    .padding(24)
  }

  private func chooseDestinationAndExport() {
    guard canExport else { return }
    let panel = NSSavePanel()
    panel.canCreateDirectories = true
    panel.isExtensionHidden = false
    panel.nameFieldStringValue = "Loom-Credential-Vault.loomvault"
    if let type = UTType(filenameExtension: "loomvault") {
      panel.allowedContentTypes = [type]
    }
    guard panel.runModal() == .OK, let destination = panel.url?.path else {
      return
    }
    let submittedPassphrase = passphrase
    passphrase = ""
    confirmation = ""
    errorMessage = nil
    Task {
      let succeeded = await store.exportCredentialVault(
        passphrase: submittedPassphrase,
        destination: destination
      )
      if succeeded {
        dismiss()
      } else {
        errorMessage = store.credentialVaultOperationDetail
          ?? "Encrypted backup failed"
      }
    }
  }
}

private struct ProviderCredentialSheet: View {
  @ObservedObject var store: LocalProductStore
  let provider: LocalProductProviderDirectoryEntry
  @Environment(\.dismiss) private var dismiss
  @State private var secret = ""
  @State private var selectedAccountID = ""
  @State private var creatingAccount = false
  @State private var accountName = ""
  @State private var confirmRemove = false
  @State private var diagnosticPreview: LocalDiagnosticBundlePreview?
  @State private var diagnosticExporter: LocalDiagnosticBundleExporter?
  @State private var diagnosticError: String?
  @State private var preparingDiagnostics = false
  @State private var selectedPolicyAccount: LocalProductProviderAccountDirectoryEntry?

  private var currentProvider: LocalProductProviderDirectoryEntry {
    store.setupSnapshot?.providers.first {
      $0.providerID == provider.providerID
    } ?? provider
  }

  private var accounts: [LocalProductProviderAccountDirectoryEntry] {
    (store.setupSnapshot?.providerAccounts ?? []).filter {
      $0.providerID == provider.providerID
    }
  }

  private var primaryAccountID: String { provider.providerID + ".primary" }

  private var accountIDs: [String] {
    var values = accounts.map(\.providerAccountID)
    if !values.contains(primaryAccountID) { values.append(primaryAccountID) }
    return values.sorted { lhs, rhs in
      if lhs == primaryAccountID { return true }
      if rhs == primaryAccountID { return false }
      return lhs < rhs
    }
  }

  private var defaultAccountID: String {
    accountIDs.first ?? primaryAccountID
  }

  private var proposedAccountID: String? {
    providerAccountIdentifier(providerID: provider.providerID, name: accountName)
  }

  private var activeAccountID: String {
    if creatingAccount { return proposedAccountID ?? "" }
    return selectedAccountID.isEmpty ? defaultAccountID : selectedAccountID
  }

  private var current: ProviderCredentialViewState {
    if let account = accounts.first(where: {
      $0.providerAccountID == activeAccountID
    }) {
      return ProviderCredentialViewState(
        reference: account.credentialReference,
        revision: account.revision,
        status: account.status
      )
    }
    if activeAccountID == primaryAccountID {
      return ProviderCredentialViewState(
        reference: currentProvider.credentialReference,
        revision: currentProvider.revision,
        status: currentProvider.status
      )
    }
    return ProviderCredentialViewState(
      reference: "",
      revision: 0,
      status: "unconfigured"
    )
  }

  private var connected: Bool {
    !current.reference.isEmpty && current.revision > 0 && current.status != "revoked"
  }

  private var inFlight: Bool {
    !activeAccountID.isEmpty && store.providersInFlight.contains(activeAccountID)
  }

  private var visibleStatus: String {
    store.providerOperationStatus[activeAccountID] ?? current.status
  }

  private var accountNameError: String? {
    guard creatingAccount, !accountName.isEmpty else { return nil }
    guard let proposedAccountID else { return "Use letters, numbers, or hyphens." }
    if accountIDs.contains(proposedAccountID) { return "That account already exists." }
    return nil
  }

  var body: some View {
    VStack(alignment: .leading, spacing: 0) {
      HStack(alignment: .top) {
        VStack(alignment: .leading, spacing: 4) {
          Text(provider.displayName)
            .font(.title2.weight(.semibold))
          Text(protocolLabel)
            .foregroundStyle(.secondary)
        }
        Spacer()
        Button {
          dismiss()
        } label: {
          Image(systemName: "xmark")
        }
        .buttonStyle(.plain)
        .help("Close")
        .accessibilityLabel("Close \(provider.displayName) settings")
        .loomActionTarget()
      }
      .padding(24)
      Divider()

      VStack(alignment: .leading, spacing: 18) {
        VStack(alignment: .leading, spacing: 8) {
          HStack {
            Text("PROVIDER ACCOUNT")
              .font(.caption.weight(.semibold))
              .foregroundStyle(.secondary)
            Spacer()
            Button {
              creatingAccount.toggle()
              accountName = ""
              secret = ""
            } label: {
              Image(systemName: creatingAccount ? "xmark" : "plus")
            }
            .buttonStyle(.borderless)
            .help(creatingAccount ? "Cancel new account" : "Add Provider Account")
            .accessibilityLabel(
              creatingAccount ? "Cancel new account" : "Add Provider Account"
            )
          }

          if creatingAccount {
            TextField("Account name", text: $accountName)
              .textFieldStyle(.roundedBorder)
              .disabled(inFlight)
            if let accountNameError {
              Text(accountNameError)
                .font(.caption)
                .foregroundStyle(LoomGraphite.statusDanger)
            }
          } else {
            Picker("Provider Account", selection: $selectedAccountID) {
              ForEach(accountIDs, id: \.self) { accountID in
                Text(
                  providerAccountDisplayName(
                    accountID,
                    providerID: provider.providerID
                  )
                ).tag(accountID)
              }
            }
            .labelsHidden()
            .pickerStyle(.menu)
          }

          if !activeAccountID.isEmpty {
            Text(activeAccountID)
              .font(.caption.monospaced())
              .foregroundStyle(.secondary)
              .textSelection(.enabled)
          }
        }

        Divider()

        HStack(spacing: 8) {
          Circle()
            .fill(statusColor)
            .frame(width: 8, height: 8)
          Text(displayStatus(visibleStatus))
            .font(.callout.weight(.medium))
          Spacer()
          Label("Loom Credential Vault", systemImage: "lock.fill")
            .font(.caption)
            .foregroundStyle(.secondary)
        }

        if let policy = accounts.first(where: {
          $0.providerAccountID == activeAccountID && $0.policyAvailable
        }) {
          Label(
            "\(policy.maximumConcurrentAttempts) concurrent · \(policy.maximumDispatchStarts) starts / \(policy.dispatchWindowSeconds)s · \(policy.maximumAssignedBudgetUnits) budget units",
            systemImage: "slider.horizontal.3"
          )
          .font(.caption.monospacedDigit())
          .foregroundStyle(.secondary)
          .fixedSize(horizontal: false, vertical: true)
          .accessibilityLabel(
            "Account limits: \(policy.maximumConcurrentAttempts) concurrent attempts, \(policy.maximumDispatchStarts) starts per \(policy.dispatchWindowSeconds) seconds, \(policy.maximumAssignedBudgetUnits) budget units"
          )
        }

        if let detail = store.providerOperationDetail[activeAccountID],
          !detail.isEmpty
        {
          Text(detail)
            .font(.callout)
            .foregroundStyle(.secondary)
            .fixedSize(horizontal: false, vertical: true)
        }

        if current.status == "migration_required" {
          Text(
            "Re-enter this account's API key once to move it into Loom Credential Vault. Existing Keychain data is not read during conversations."
          )
          .font(.callout)
          .foregroundStyle(.secondary)
          .fixedSize(horizontal: false, vertical: true)
        } else if current.status == "recovery_required" {
          Text(
            "Loom could not open the encrypted credential record. Restart Loom, then review diagnostics before replacing the key."
          )
          .font(.callout)
          .foregroundStyle(.secondary)
          .fixedSize(horizontal: false, vertical: true)
        }

        if let stage = store.providerOperationStage[activeAccountID] {
          HStack(spacing: 8) {
            Label("Stage: \(stage)", systemImage: "point.3.connected.trianglepath.dotted")
              .font(.caption)
              .foregroundStyle(.secondary)
            Spacer()
            if let retryable = store.providerOperationRetryable[activeAccountID] {
              Text(retryable ? "Retry available" : "Review required")
                .font(.caption.weight(.medium))
                .foregroundStyle(retryable ? Color.secondary : Color.orange)
            }
          }
        }

        if let incidentID = store.providerOperationIncidentID[activeAccountID] {
          HStack(spacing: 8) {
            Text("Incident \(incidentID)")
              .font(.caption.monospaced())
              .foregroundStyle(.secondary)
              .lineLimit(1)
              .truncationMode(.middle)
            Spacer()
            Button {
              NSPasteboard.general.clearContents()
              NSPasteboard.general.setString(incidentID, forType: .string)
            } label: {
              Image(systemName: "doc.on.doc")
            }
            .buttonStyle(.plain)
            .help("Copy incident ID")
            .accessibilityLabel("Copy incident ID")

            Button {
              prepareDiagnosticPreview()
            } label: {
              Label("View diagnostics", systemImage: "doc.text.magnifyingglass")
            }
            .buttonStyle(.borderless)
            .disabled(preparingDiagnostics)
          }
        }

        if let diagnosticError {
          Text(diagnosticError)
            .font(.caption)
            .foregroundStyle(LoomGraphite.statusDanger)
        }

        VStack(alignment: .leading, spacing: 7) {
          Text(
            current.status == "migration_required"
              ? "API KEY FOR VAULT" : (connected ? "NEW API KEY" : "API KEY")
          )
          .font(.caption.weight(.semibold))
          .foregroundStyle(.secondary)
          SecureField("API key", text: $secret)
            .textFieldStyle(.roundedBorder)
            .disabled(inFlight)
        }

        if connected {
          HStack(spacing: 10) {
            if current.status != "migration_required" && current.status != "recovery_required" {
              Button {
                let accountID = activeAccountID
                Task {
                  await store.verifyProvider(
                    providerID: provider.providerID,
                    providerAccountID: accountID
                  )
                }
              } label: {
                Label("Test", systemImage: "bolt")
              }
              .buttonStyle(.bordered)
              .disabled(inFlight)
              .loomActionTarget()
            }

            Button {
              let replacement = secret
              let accountID = activeAccountID
              secret = ""
              Task {
                await store.replaceProvider(
                  providerID: provider.providerID,
                  providerAccountID: accountID,
                  secret: replacement
                )
              }
            } label: {
              Label(
                current.status == "migration_required" ? "Move key to Vault" : "Replace key",
                systemImage: "arrow.triangle.2.circlepath"
              )
            }
            .buttonStyle(.borderedProminent)
            .disabled(inFlight || secret.isEmpty)
            .loomActionTarget()

            if let policyAccount = accounts.first(where: {
              $0.providerAccountID == activeAccountID
            }) {
              Button {
                selectedPolicyAccount = policyAccount
              } label: {
                Label("Limits", systemImage: "slider.horizontal.3")
              }
              .buttonStyle(.bordered)
              .disabled(
                inFlight || store.providerPolicyAccountsInFlight.contains(activeAccountID)
              )
              .help("Configure Provider Account limits")
              .loomActionTarget()
            }

            Spacer()
            Button(role: .destructive) {
              confirmRemove = true
            } label: {
              Image(systemName: "trash")
            }
            .buttonStyle(.bordered)
            .disabled(inFlight)
            .help("Remove credential")
            .accessibilityLabel("Remove \(provider.displayName) credential")
            .loomActionTarget()
          }
        } else {
          Button {
            let key = secret
            let accountID = activeAccountID
            secret = ""
            Task {
              await store.connectProvider(
                providerID: provider.providerID,
                providerAccountID: accountID,
                secret: key
              )
              if store.setupSnapshot?.providerAccounts.contains(where: {
                $0.providerAccountID == accountID
              }) == true {
                selectedAccountID = accountID
                creatingAccount = false
                accountName = ""
              }
            }
          } label: {
            Label("Connect & verify", systemImage: "link")
          }
          .buttonStyle(.borderedProminent)
          .disabled(
            inFlight || secret.isEmpty || activeAccountID.isEmpty || accountNameError != nil
          )
          .loomActionTarget()
        }

        Spacer(minLength: 0)
      }
      .padding(24)
    }
    .background(LoomGraphite.canvas)
    .onAppear {
      if selectedAccountID.isEmpty { selectedAccountID = defaultAccountID }
    }
    .onChange(of: accountIDs) { _, values in
      if !creatingAccount && !values.contains(selectedAccountID) {
        selectedAccountID = values.first ?? primaryAccountID
      }
    }
    .confirmationDialog(
      "Remove \(providerAccountDisplayName(activeAccountID, providerID: provider.providerID)) credential?",
      isPresented: $confirmRemove,
      titleVisibility: .visible
    ) {
      Button("Remove credential", role: .destructive) {
        let accountID = activeAccountID
        Task {
          await store.revokeProvider(
            providerID: provider.providerID,
            providerAccountID: accountID
          )
        }
      }
      Button("Cancel", role: .cancel) { confirmRemove = false }
    }
    .sheet(item: $diagnosticPreview) { preview in
      if let diagnosticExporter {
        DiagnosticBundlePreviewSheet(
          preview: preview,
          exporter: diagnosticExporter
        )
      }
    }
    .sheet(item: $selectedPolicyAccount) { account in
      ProviderAccountPolicySheet(store: store, account: account)
        .frame(minWidth: 520, minHeight: 620)
    }
  }

  private var protocolLabel: String {
    switch provider.protocolFamily {
    case "openai_responses": return "Responses API"
    case "openai_compatible": return "OpenAI-compatible"
    case "anthropic_messages": return "Messages API"
    case "gemini_generate_content": return "Gemini API"
    case "bedrock_converse": return "Bedrock Converse"
    case "vertex_generate_content": return "Vertex AI"
    case "ollama": return "Ollama API"
    default:
      return provider.protocolFamily
        .replacingOccurrences(of: "_", with: " ")
        .capitalized
    }
  }

  private var statusColor: Color {
    switch visibleStatus.lowercased() {
    case "available", "verified": return LoomGraphite.statusSuccess
    case "connecting", "testing", "replacing", "removing", "configured", "migration_required":
      return LoomGraphite.statusWarning
    case "rejected", "unavailable", "recovery_required": return LoomGraphite.statusDanger
    default: return Color.secondary.opacity(0.65)
    }
  }

  private func displayStatus(_ value: String) -> String {
    value.replacingOccurrences(of: "_", with: " ").capitalized
  }

  private func prepareDiagnosticPreview() {
    guard !preparingDiagnostics else { return }
    preparingDiagnostics = true
    diagnosticError = nil
    let input = LocalDiagnosticBundleInput(
      setupSnapshot: store.setupSnapshot,
      board: store.timeline?.board
    )
    Task {
      do {
        let exporter = try LocalDiagnosticBundleExporter.installed()
        let preview = try await Task.detached {
          try exporter.preview(input: input)
        }.value
        diagnosticExporter = exporter
        diagnosticPreview = preview
      } catch {
        diagnosticError = "Diagnostics could not be prepared."
      }
      preparingDiagnostics = false
    }
  }
}

struct ProviderAccountPolicySheet: View {
  @ObservedObject var store: LocalProductStore
  let account: LocalProductProviderAccountDirectoryEntry
  @Environment(\.dismiss) private var dismiss
  @State private var maximumConcurrentAttempts: Int
  @State private var dispatchWindowSeconds: Int64
  @State private var maximumDispatchStarts: Int
  @State private var maximumAssignedBudgetUnits: Int64
  @State private var trustDomain: String
  @State private var retentionMode: String
  @State private var dataRegion: String
  @State private var rateCardEditor: ProviderModelRateCardEditorSelection?
  @State private var remoteToolEditor: LocalProductRemoteToolBackendEnrollment?
  @State private var pendingRemoteToolRevoke: LocalProductRemoteToolBackendEnrollment?

  init(
    store: LocalProductStore,
    account: LocalProductProviderAccountDirectoryEntry
  ) {
    self.store = store
    self.account = account
    _maximumConcurrentAttempts = State(
      initialValue: account.policyAvailable ? account.maximumConcurrentAttempts : 1
    )
    _dispatchWindowSeconds = State(
      initialValue: account.policyAvailable ? account.dispatchWindowSeconds : 60
    )
    _maximumDispatchStarts = State(
      initialValue: account.policyAvailable ? account.maximumDispatchStarts : 60
    )
    _maximumAssignedBudgetUnits = State(
      initialValue: account.policyAvailable ? account.maximumAssignedBudgetUnits : 0
    )
    _trustDomain = State(initialValue: account.disclosurePolicyConfigured
      ? account.trustDomain : "external_provider")
    _retentionMode = State(initialValue: account.disclosurePolicyConfigured
      ? account.retentionMode : "provider_default")
    _dataRegion = State(initialValue: account.disclosurePolicyConfigured
      ? account.dataRegion : "global")
  }

  private var isSaving: Bool {
    store.providerPolicyAccountsInFlight.contains(account.providerAccountID)
  }

  private var currentAccount: LocalProductProviderAccountDirectoryEntry {
    store.setupSnapshot?.providerAccounts.first {
      $0.providerID == account.providerID
        && $0.providerAccountID == account.providerAccountID
    } ?? account
  }

  private var valid: Bool {
    (1...64).contains(maximumConcurrentAttempts)
      && (1...86_400).contains(dispatchWindowSeconds)
      && (1...1_000_000).contains(maximumDispatchStarts)
      && maximumAssignedBudgetUnits >= 0
      && ["external_provider", "enterprise_tenant", "local_runtime"].contains(trustDomain)
      && ["provider_default", "zero_data_retention", "limited_retention"].contains(retentionMode)
      && ["global", "us", "eu", "apac", "local"].contains(dataRegion)
  }

  var body: some View {
    VStack(alignment: .leading, spacing: 0) {
      HStack(alignment: .top) {
        VStack(alignment: .leading, spacing: 4) {
          Text("Account limits")
            .font(.title2.weight(.semibold))
          Text(account.providerAccountID)
            .font(.caption.monospaced())
            .foregroundStyle(.secondary)
            .textSelection(.enabled)
        }
        Spacer()
        Button {
          dismiss()
        } label: {
          Image(systemName: "xmark")
        }
        .buttonStyle(.plain)
        .help("Close")
        .accessibilityLabel("Close account limits")
        .loomActionTarget()
      }
      .padding(24)
      Divider()

      Form {
        Section("Capacity") {
          Stepper(value: $maximumConcurrentAttempts, in: 1...64) {
            LabeledContent(
              "Concurrent Agent Attempts",
              value: maximumConcurrentAttempts.formatted()
            )
          }
          TextField(
            "Dispatch window (seconds)",
            value: $dispatchWindowSeconds,
            format: .number
          )
          TextField(
            "Starts per window",
            value: $maximumDispatchStarts,
            format: .number
          )
        }

        Section("Budget") {
          TextField(
            "Maximum assigned budget units",
            value: $maximumAssignedBudgetUnits,
            format: .number
          )
          Text("The combined frozen budgets of active Attempts cannot exceed this account ceiling.")
            .font(.caption)
            .foregroundStyle(.secondary)
            .fixedSize(horizontal: false, vertical: true)
          Text("A zero ceiling permits only Attempts whose assigned budget is also zero.")
            .font(.caption)
            .foregroundStyle(.secondary)
            .fixedSize(horizontal: false, vertical: true)
        }

        Section("Account disclosure policy") {
          Picker("Trust domain", selection: $trustDomain) {
            Text("External provider").tag("external_provider")
            Text("Enterprise tenant").tag("enterprise_tenant")
            Text("Local runtime").tag("local_runtime")
          }
          Picker("Retention", selection: $retentionMode) {
            Text("Provider default").tag("provider_default")
            Text("Zero data retention").tag("zero_data_retention")
            Text("Limited retention").tag("limited_retention")
          }
          Picker("Data region", selection: $dataRegion) {
            Text("Global").tag("global")
            Text("United States").tag("us")
            Text("European Union").tag("eu")
            Text("Asia Pacific").tag("apac")
            Text("Local").tag("local")
          }
          Text("These values record the account policy you selected. Loom does not independently certify Provider retention or region guarantees.")
            .font(.caption)
            .foregroundStyle(.secondary)
            .fixedSize(horizontal: false, vertical: true)
        }

        Section {
          if currentAccount.rateCards.isEmpty {
            Text("No model rates configured")
              .foregroundStyle(.secondary)
          } else {
            ForEach(currentAccount.rateCards) { rateCard in
              HStack(spacing: 12) {
                VStack(alignment: .leading, spacing: 2) {
                  Text(rateCard.modelID)
                    .lineLimit(1)
                  Text("Revision \(rateCard.revision) · \(rateCard.currency)")
                    .font(.caption.monospacedDigit())
                    .foregroundStyle(.secondary)
                }
                Spacer()
                Button {
                  rateCardEditor = ProviderModelRateCardEditorSelection(
                    rateCard: rateCard
                  )
                } label: {
                  Image(systemName: "pencil")
                }
                .buttonStyle(.borderless)
                .help("Edit \(rateCard.modelID) rate card")
                .accessibilityLabel("Edit \(rateCard.modelID) rate card")
              }
            }
          }
        } header: {
          HStack {
            Text("Rate cards")
            Spacer()
            Button {
              rateCardEditor = ProviderModelRateCardEditorSelection(rateCard: nil)
            } label: {
              Image(systemName: "plus")
            }
            .buttonStyle(.borderless)
            .help("Add model rate card")
            .accessibilityLabel("Add model rate card")
          }
        }

        Section("Remote tools") {
          if currentAccount.remoteToolBackends.isEmpty {
            Text("No enrolled remote tools")
              .foregroundStyle(.secondary)
          } else {
            ForEach(currentAccount.remoteToolBackends) { enrollment in
              HStack(alignment: .center, spacing: 12) {
                Image(
                  systemName: enrollment.backendKind == "web_search"
                    ? "magnifyingglass" : "point.3.connected.trianglepath.dotted"
                )
                .foregroundStyle(.secondary)
                .frame(width: 18)

                VStack(alignment: .leading, spacing: 3) {
                  HStack(spacing: 6) {
                    Text(remoteToolDisplayName(enrollment))
                      .lineLimit(1)
                    Text(enrollment.status == "active" ? "Active" : "Revoked")
                      .font(.caption.weight(.medium))
                      .foregroundStyle(
                        enrollment.status == "active"
                          ? LoomGraphite.statusSuccess : Color.secondary
                      )
                  }
                  Text(remoteToolSummary(enrollment))
                    .font(.caption.monospacedDigit())
                    .foregroundStyle(.secondary)
                    .lineLimit(2)
                  if !enrollment.policyCurrent {
                    Label("Policy changed", systemImage: "arrow.triangle.2.circlepath")
                      .font(.caption.weight(.medium))
                      .foregroundStyle(LoomGraphite.statusWarning)
                  }
                  if let detail = store.remoteToolEnrollmentOperationDetail[
                    enrollment.enrollmentID
                  ], !detail.isEmpty {
                    Text(detail)
                      .font(.caption)
                      .foregroundStyle(
                        detail == "Remote tool saved"
                          || detail == "Remote tool revoked"
                          ? LoomGraphite.statusSuccess : LoomGraphite.statusDanger
                      )
                      .fixedSize(horizontal: false, vertical: true)
                  }
                }

                Spacer(minLength: 8)
                Button {
                  remoteToolEditor = enrollment
                } label: {
                  Image(systemName: "slider.horizontal.3")
                }
                .buttonStyle(.borderless)
                .disabled(
                  isSaving
                    || store.remoteToolEnrollmentsInFlight.contains(
                      enrollment.enrollmentID
                    )
                )
                .help("Edit remote tool limits")
                .accessibilityLabel("Edit \(remoteToolDisplayName(enrollment))")

                if enrollment.status == "active" {
                  Button(role: .destructive) {
                    pendingRemoteToolRevoke = enrollment
                  } label: {
                    Image(systemName: "trash")
                  }
                  .buttonStyle(.borderless)
                  .disabled(
                    isSaving
                      || store.remoteToolEnrollmentsInFlight.contains(
                        enrollment.enrollmentID
                      )
                  )
                  .help("Revoke remote tool")
                  .accessibilityLabel("Revoke \(remoteToolDisplayName(enrollment))")
                }
              }
            }
          }
        }

        if let detail = store.providerPolicyOperationDetail[account.providerAccountID],
          !detail.isEmpty
        {
          Section {
            Label(
              detail,
              systemImage: detail == "Limits saved"
                ? "checkmark.circle.fill" : "exclamationmark.triangle.fill"
            )
            .foregroundStyle(
              detail == "Limits saved"
                ? LoomGraphite.statusSuccess : LoomGraphite.statusDanger
            )
          }
        }
      }
      .formStyle(.grouped)

      Divider()
      HStack(spacing: 10) {
        if account.policyAvailable {
          Text("Policy revision \(account.policyRevision)")
            .font(.caption.monospacedDigit())
            .foregroundStyle(.secondary)
        }
        Spacer()
        Button("Cancel") { dismiss() }
          .keyboardShortcut(.cancelAction)
        Button {
          save()
        } label: {
          if isSaving {
            ProgressView()
              .controlSize(.small)
          } else {
            Text("Save limits")
          }
        }
        .buttonStyle(.borderedProminent)
        .keyboardShortcut(.defaultAction)
        .disabled(!valid || isSaving)
      }
      .padding(20)
    }
    .background(LoomGraphite.canvas)
    .sheet(item: $rateCardEditor) { selection in
      ProviderModelRateCardSheet(
        store: store,
        account: currentAccount,
        rateCard: selection.rateCard
      )
      .frame(minWidth: 540, minHeight: 620)
    }
    .sheet(item: $remoteToolEditor) { enrollment in
      ProviderRemoteToolEnrollmentSheet(
        store: store,
        account: currentAccount,
        enrollment: enrollment
      )
      .frame(minWidth: 540, minHeight: 620)
    }
    .confirmationDialog(
      "Revoke \(pendingRemoteToolRevoke.map(remoteToolDisplayName) ?? "remote tool")?",
      isPresented: Binding(
        get: { pendingRemoteToolRevoke != nil },
        set: { if !$0 { pendingRemoteToolRevoke = nil } }
      ),
      titleVisibility: .visible
    ) {
      Button("Revoke remote tool", role: .destructive) {
        guard let enrollment = pendingRemoteToolRevoke else { return }
        pendingRemoteToolRevoke = nil
        Task {
          _ = await store.revokeRemoteToolBackend(
            LocalProductRemoteToolBackendEnrollmentRevokeCommand(
              enrollmentID: enrollment.enrollmentID,
              providerID: account.providerID,
              providerAccountID: account.providerAccountID,
              expectedRevision: enrollment.revision
            )
          )
        }
      }
      Button("Cancel", role: .cancel) { pendingRemoteToolRevoke = nil }
    } message: {
      Text("This revokes the account-scoped Enrollment. Existing Attempt bindings are unchanged.")
    }
  }

  private func remoteToolDisplayName(
    _ enrollment: LocalProductRemoteToolBackendEnrollment
  ) -> String {
    enrollment.backendKind == "web_search"
      ? "Web Search"
      : enrollment.mcpServerID.replacingOccurrences(of: "-", with: " ").capitalized
  }

  private func remoteToolSummary(
    _ enrollment: LocalProductRemoteToolBackendEnrollment
  ) -> String {
    let identity = enrollment.backendKind == "mcp_server"
      ? "\(enrollment.allowedTools.count) tools"
      : enrollment.adapterID
    return "\(identity) · \(enrollment.maximumCallsPerAttempt) calls / Attempt · \(enrollment.timeoutSeconds)s"
  }

  private func save() {
    guard valid, !isSaving else { return }
    Task {
      let succeeded = await store.configureProviderAccountPolicy(
        providerID: account.providerID,
        providerAccountID: account.providerAccountID,
        expectedRevision: account.policyAvailable ? account.policyRevision : 0,
        maximumConcurrentAttempts: maximumConcurrentAttempts,
        dispatchWindowSeconds: dispatchWindowSeconds,
        maximumDispatchStarts: maximumDispatchStarts,
        maximumAssignedBudgetUnits: maximumAssignedBudgetUnits,
        trustDomain: trustDomain,
        retentionMode: retentionMode,
        dataRegion: dataRegion
      )
      if succeeded { dismiss() }
    }
  }
}

private struct ProviderModelRateCardEditorSelection: Identifiable {
  let id = UUID()
  let rateCard: LocalProductProviderModelRateCard?
}

private struct ProviderModelRateCardSheet: View {
  @ObservedObject var store: LocalProductStore
  let account: LocalProductProviderAccountDirectoryEntry
  let rateCard: LocalProductProviderModelRateCard?
  @Environment(\.dismiss) private var dismiss
  @State private var modelID: String
  @State private var currency: String
  @State private var inputTokenBasis: String
  @State private var inputRate: Int64
  @State private var outputRate: Int64
  @State private var cacheReadRate: Int64
  @State private var cacheWriteRate: Int64

  init(
    store: LocalProductStore,
    account: LocalProductProviderAccountDirectoryEntry,
    rateCard: LocalProductProviderModelRateCard?
  ) {
    self.store = store
    self.account = account
    self.rateCard = rateCard
    _modelID = State(initialValue: rateCard?.modelID ?? "")
    _currency = State(initialValue: rateCard?.currency ?? "USD")
    _inputTokenBasis = State(
      initialValue: rateCard?.inputTokenBasis ?? "input_excludes_cache"
    )
    _inputRate = State(initialValue: rateCard?.inputMicrounitsPerMillion ?? 0)
    _outputRate = State(initialValue: rateCard?.outputMicrounitsPerMillion ?? 0)
    _cacheReadRate = State(
      initialValue: rateCard?.cacheReadMicrounitsPerMillion ?? 0
    )
    _cacheWriteRate = State(
      initialValue: rateCard?.cacheWriteMicrounitsPerMillion ?? 0
    )
  }

  private var operationKey: String {
    account.providerAccountID + "\u{1f}" + modelID
  }

  private var isSaving: Bool {
    store.providerRateCardsInFlight.contains(operationKey)
  }

  private var valid: Bool {
    LocalIPCClient.validModelID(modelID)
      && currency.utf8.count == 3
      && currency.utf8.allSatisfy { $0 >= 0x41 && $0 <= 0x5a }
      && [inputRate, outputRate, cacheReadRate, cacheWriteRate].allSatisfy {
        (0...1_000_000_000_000).contains($0)
      }
  }

  var body: some View {
    VStack(alignment: .leading, spacing: 0) {
      HStack(alignment: .top) {
        VStack(alignment: .leading, spacing: 4) {
          Text(rateCard == nil ? "Add rate card" : "Edit rate card")
            .font(.title2.weight(.semibold))
          Text(account.providerAccountID)
            .font(.caption.monospaced())
            .foregroundStyle(.secondary)
            .textSelection(.enabled)
        }
        Spacer()
        Button { dismiss() } label: {
          Image(systemName: "xmark")
        }
        .buttonStyle(.plain)
        .help("Close")
        .accessibilityLabel("Close rate card")
        .loomActionTarget()
      }
      .padding(24)
      Divider()

      Form {
        Section("Model") {
          TextField("Model ID", text: $modelID)
            .disabled(rateCard != nil)
          TextField("Currency", text: $currency)
            .onChange(of: currency) { _, value in
              let normalized = String(value.uppercased().prefix(3))
              if normalized != value { currency = normalized }
            }
          Picker("Input tokens", selection: $inputTokenBasis) {
            Text("Includes cached").tag("input_includes_cache")
            Text("Excludes cached").tag("input_excludes_cache")
          }
          .pickerStyle(.segmented)
        }

        Section("Microunits per 1M tokens") {
          TextField("Input", value: $inputRate, format: .number)
          TextField("Output", value: $outputRate, format: .number)
          TextField("Cache read", value: $cacheReadRate, format: .number)
          TextField("Cache write", value: $cacheWriteRate, format: .number)
        }

        Section("Rounding") {
          LabeledContent("Mode", value: "Ceiling per Attempt")
        }

        if let detail = store.providerRateCardOperationDetail[operationKey],
          !detail.isEmpty
        {
          Section {
            Label(
              detail,
              systemImage: detail == "Rate card saved"
                ? "checkmark.circle.fill" : "exclamationmark.triangle.fill"
            )
            .foregroundStyle(
              detail == "Rate card saved"
                ? LoomGraphite.statusSuccess : LoomGraphite.statusDanger
            )
          }
        }
      }
      .formStyle(.grouped)

      Divider()
      HStack {
        if let rateCard {
          Text("Revision \(rateCard.revision)")
            .font(.caption.monospacedDigit())
            .foregroundStyle(.secondary)
        }
        Spacer()
        Button("Cancel") { dismiss() }
          .keyboardShortcut(.cancelAction)
        Button {
          save()
        } label: {
          if isSaving {
            ProgressView().controlSize(.small)
          } else {
            Text("Save rate card")
          }
        }
        .buttonStyle(.borderedProminent)
        .keyboardShortcut(.defaultAction)
        .disabled(!valid || isSaving)
      }
      .padding(20)
    }
    .background(LoomGraphite.canvas)
  }

  private func save() {
    guard valid, !isSaving else { return }
    Task {
      let succeeded = await store.configureProviderModelRateCard(
        providerID: account.providerID,
        providerAccountID: account.providerAccountID,
        modelID: modelID,
        expectedRevision: rateCard?.revision ?? 0,
        currency: currency,
        inputTokenBasis: inputTokenBasis,
        inputMicrounitsPerMillion: inputRate,
        outputMicrounitsPerMillion: outputRate,
        cacheReadMicrounitsPerMillion: cacheReadRate,
        cacheWriteMicrounitsPerMillion: cacheWriteRate
      )
      if succeeded { dismiss() }
    }
  }
}

private struct ProviderRemoteToolEnrollmentSheet: View {
  @ObservedObject var store: LocalProductStore
  let account: LocalProductProviderAccountDirectoryEntry
  let enrollment: LocalProductRemoteToolBackendEnrollment
  @Environment(\.dismiss) private var dismiss
  @State private var maximumConcurrentCalls: Int
  @State private var maximumCallsPerAttempt: Int
  @State private var timeoutSeconds: Int64
  @State private var maximumResultBytes: Int
  @State private var maximumBudgetUnits: Int64
  @State private var allowedToolsText: String
  @State private var diagnosticPreview: LocalDiagnosticBundlePreview?
  @State private var diagnosticExporter: LocalDiagnosticBundleExporter?
  @State private var diagnosticError: String?
  @State private var preparingDiagnostics = false

  init(
    store: LocalProductStore,
    account: LocalProductProviderAccountDirectoryEntry,
    enrollment: LocalProductRemoteToolBackendEnrollment
  ) {
    self.store = store
    self.account = account
    self.enrollment = enrollment
    _maximumConcurrentCalls = State(initialValue: enrollment.maximumConcurrentCalls)
    _maximumCallsPerAttempt = State(initialValue: enrollment.maximumCallsPerAttempt)
    _timeoutSeconds = State(initialValue: enrollment.timeoutSeconds)
    _maximumResultBytes = State(initialValue: enrollment.maximumResultBytes)
    _maximumBudgetUnits = State(initialValue: enrollment.maximumBudgetUnits)
    _allowedToolsText = State(initialValue: enrollment.allowedTools.joined(separator: ", "))
  }

  private var currentAccount: LocalProductProviderAccountDirectoryEntry {
    store.setupSnapshot?.providerAccounts.first {
      $0.providerID == account.providerID
        && $0.providerAccountID == account.providerAccountID
    } ?? account
  }

  private var currentEnrollment: LocalProductRemoteToolBackendEnrollment {
    currentAccount.remoteToolBackends.first {
      $0.enrollmentID == enrollment.enrollmentID
    } ?? enrollment
  }

  private var normalizedTools: [String] {
    allowedToolsText.split(separator: ",", omittingEmptySubsequences: false)
      .map { $0.trimmingCharacters(in: .whitespacesAndNewlines) }
      .filter { !$0.isEmpty }
      .sorted()
  }

  private var command: LocalProductRemoteToolBackendEnrollmentCommand {
    LocalProductRemoteToolBackendEnrollmentCommand(
      enrollmentID: currentEnrollment.enrollmentID,
      backendKind: currentEnrollment.backendKind,
      adapterID: currentEnrollment.adapterID,
      providerID: currentAccount.providerID,
      providerAccountID: currentAccount.providerAccountID,
      providerAccountPolicyVersion: currentAccount.policyVersion,
      providerAccountPolicyRevision: currentAccount.policyRevision,
      providerAccountPolicyDigest: currentAccount.policyDigest,
      endpointFingerprint: currentEnrollment.endpointFingerprint,
      mcpServerID: currentEnrollment.mcpServerID,
      allowedTools: currentEnrollment.backendKind == "mcp_server"
        ? normalizedTools : [],
      expectedRevision: currentEnrollment.revision,
      maximumConcurrentCalls: maximumConcurrentCalls,
      maximumCallsPerAttempt: maximumCallsPerAttempt,
      timeoutSeconds: timeoutSeconds,
      maximumResultBytes: maximumResultBytes,
      maximumBudgetUnits: maximumBudgetUnits
    )
  }

  private var isSaving: Bool {
    store.remoteToolEnrollmentsInFlight.contains(enrollment.enrollmentID)
  }

  var body: some View {
    VStack(alignment: .leading, spacing: 0) {
      HStack(alignment: .top) {
        VStack(alignment: .leading, spacing: 4) {
          Text(currentEnrollment.backendKind == "web_search" ? "Web Search" : "MCP tools")
            .font(.title2.weight(.semibold))
          Text(currentAccount.providerAccountID)
            .font(.caption.monospaced())
            .foregroundStyle(.secondary)
            .textSelection(.enabled)
        }
        Spacer()
        Button { dismiss() } label: {
          Image(systemName: "xmark")
        }
        .buttonStyle(.plain)
        .help("Close")
        .accessibilityLabel("Close remote tool settings")
        .loomActionTarget()
      }
      .padding(24)
      Divider()

      Form {
        Section("Identity") {
          LabeledContent("Enrollment", value: currentEnrollment.enrollmentID)
          LabeledContent("Adapter", value: currentEnrollment.adapterID)
          if currentEnrollment.backendKind == "mcp_server" {
            LabeledContent("MCP server", value: currentEnrollment.mcpServerID)
          }
          LabeledContent(
            "Policy",
            value: currentEnrollment.policyCurrent
              ? "Current" : "Rebind to revision \(currentAccount.policyRevision)"
          )
        }

        Section("Limits") {
          Stepper(value: $maximumConcurrentCalls, in: 1...16) {
            LabeledContent(
              "Concurrent calls", value: maximumConcurrentCalls.formatted()
            )
          }
          Stepper(value: $maximumCallsPerAttempt, in: 1...16) {
            LabeledContent(
              "Calls per Attempt", value: maximumCallsPerAttempt.formatted()
            )
          }
          TextField("Timeout (seconds)", value: $timeoutSeconds, format: .number)
          TextField("Maximum result bytes", value: $maximumResultBytes, format: .number)
          TextField("Maximum budget units", value: $maximumBudgetUnits, format: .number)
        }

        if currentEnrollment.backendKind == "mcp_server" {
          Section("MCP allowlist") {
            TextField("Allowed tools", text: $allowedToolsText)
              .textFieldStyle(.roundedBorder)
          }
        }

        if let detail = store.remoteToolEnrollmentOperationDetail[
          enrollment.enrollmentID
        ], !detail.isEmpty {
          Section {
            Label(
              detail,
              systemImage: detail == "Remote tool saved"
                ? "checkmark.circle.fill" : "exclamationmark.triangle.fill"
            )
            .foregroundStyle(
              detail == "Remote tool saved"
                ? LoomGraphite.statusSuccess : LoomGraphite.statusDanger
            )
          }
        }

        if let stage = store.remoteToolEnrollmentStage[enrollment.enrollmentID] {
          Section {
            Label("Stage: \(stage)", systemImage: "point.3.connected.trianglepath.dotted")
              .font(.caption)
              .foregroundStyle(.secondary)
          }
        }

        if let incidentID = store.remoteToolEnrollmentIncidentID[
          enrollment.enrollmentID
        ] {
          Section {
            HStack(spacing: 10) {
              Text("Incident \(incidentID)")
                .font(.caption.monospaced())
                .foregroundStyle(.secondary)
                .lineLimit(1)
                .truncationMode(.middle)
              Spacer()
              Button {
                NSPasteboard.general.clearContents()
                NSPasteboard.general.setString(incidentID, forType: .string)
              } label: {
                Image(systemName: "doc.on.doc")
              }
              .buttonStyle(.borderless)
              .help("Copy incident ID")
              .accessibilityLabel("Copy incident ID")

              Button {
                prepareDiagnosticPreview()
              } label: {
                Label("View diagnostics", systemImage: "doc.text.magnifyingglass")
              }
              .buttonStyle(.borderless)
              .disabled(preparingDiagnostics)
            }
          }
        }

        if let diagnosticError {
          Section {
            Text(diagnosticError)
              .font(.caption)
              .foregroundStyle(LoomGraphite.statusDanger)
          }
        }
      }
      .formStyle(.grouped)

      Divider()
      HStack {
        Text("Revision \(currentEnrollment.revision)")
          .font(.caption.monospacedDigit())
          .foregroundStyle(.secondary)
        Spacer()
        Button("Cancel") { dismiss() }
          .keyboardShortcut(.cancelAction)
        Button {
          save()
        } label: {
          if isSaving {
            ProgressView().controlSize(.small)
          } else {
            Text(saveLabel)
          }
        }
        .buttonStyle(.borderedProminent)
        .keyboardShortcut(.defaultAction)
        .disabled(!command.valid || isSaving || !currentAccount.policyAvailable)
      }
      .padding(20)
    }
    .background(LoomGraphite.canvas)
    .sheet(item: $diagnosticPreview) { preview in
      if let diagnosticExporter {
        DiagnosticBundlePreviewSheet(preview: preview, exporter: diagnosticExporter)
      }
    }
  }

  private var saveLabel: String {
    if let detail = store.remoteToolEnrollmentOperationDetail[enrollment.enrollmentID],
      detail != "Remote tool saved", detail != "Remote tool revoked"
    {
      return store.remoteToolEnrollmentRetryable[enrollment.enrollmentID] == true
        ? "Retry" : "Review & save"
    }
    if currentEnrollment.status == "revoked" { return "Restore & save" }
    if !currentEnrollment.policyCurrent { return "Rebind & save" }
    return "Save remote tool"
  }

  private func save() {
    guard command.valid, !isSaving, currentAccount.policyAvailable else { return }
    let submitted = command
    Task {
      if await store.configureRemoteToolBackend(submitted) {
        dismiss()
      }
    }
  }

  private func prepareDiagnosticPreview() {
    guard !preparingDiagnostics else { return }
    preparingDiagnostics = true
    diagnosticError = nil
    let input = LocalDiagnosticBundleInput(
      setupSnapshot: store.setupSnapshot,
      board: store.timeline?.board
    )
    Task {
      do {
        let exporter = try LocalDiagnosticBundleExporter.installed()
        let preview = try await Task.detached {
          try exporter.preview(input: input)
        }.value
        diagnosticExporter = exporter
        diagnosticPreview = preview
      } catch {
        diagnosticError = "Diagnostics could not be prepared."
      }
      preparingDiagnostics = false
    }
  }
}

struct DiagnosticBundlePreviewSheet: View {
  let preview: LocalDiagnosticBundlePreview
  let exporter: LocalDiagnosticBundleExporter
  @Environment(\.dismiss) private var dismiss
  @State private var exportError: String?

  var body: some View {
    VStack(alignment: .leading, spacing: 0) {
      HStack {
        VStack(alignment: .leading, spacing: 3) {
          Text("Diagnostics preview")
            .font(.title2.weight(.semibold))
          Text(preview.generatedAt)
            .font(.caption.monospaced())
            .foregroundStyle(.secondary)
        }
        Spacer()
        Button {
          dismiss()
        } label: {
          Image(systemName: "xmark")
        }
        .buttonStyle(.plain)
        .help("Close")
        .accessibilityLabel("Close diagnostics preview")
      }
      .padding(20)
      Divider()

      Form {
        Section("Runtime") {
          LabeledContent("App", value: "v\(preview.app.version) (\(preview.app.build))")
          LabeledContent("App SHA-256", value: abbreviated(preview.app.sha256))
          LabeledContent("Daemon SHA-256", value: abbreviated(preview.daemon.sha256))
          LabeledContent(
            "Local service", value: preview.socketHealthy ? "Available" : "Unavailable")
          LabeledContent(
            "Daemon console",
            value: preview.console.available
              ? ByteCountFormatter.string(
                fromByteCount: preview.console.byteCount,
                countStyle: .file
              )
              : "Unavailable"
          )
        }

        Section("Included") {
          LabeledContent("Provider states", value: "\(preview.providers.count)")
          LabeledContent("Conversation profiles", value: "\(preview.profiles.count)")
          LabeledContent("Agent bindings", value: "\(preview.agents.count)")
          LabeledContent("Recent safe events", value: "\(preview.events.count)")
          if preview.diagnosticFilesSkipped > 0 {
            LabeledContent(
              "Unsafe or malformed records skipped",
              value: "\(preview.diagnosticFilesSkipped)"
            )
          }
        }

        Section("Excluded") {
          ForEach(preview.excluded, id: \.self) { value in
            Label(excludedLabel(value), systemImage: "minus.circle")
          }
        }

        if let exportError {
          Section {
            Text(exportError)
              .foregroundStyle(LoomGraphite.statusDanger)
          }
        }
      }
      .formStyle(.grouped)

      Divider()
      HStack {
        Spacer()
        Button("Cancel") { dismiss() }
        Button {
          exportBundle()
        } label: {
          Label("Export", systemImage: "square.and.arrow.up")
        }
        .buttonStyle(.borderedProminent)
      }
      .padding(16)
    }
    .frame(width: 540, height: 560)
    .background(LoomGraphite.canvas)
  }

  private func exportBundle() {
    let panel = NSSavePanel()
    panel.allowedContentTypes = [.json]
    panel.canCreateDirectories = true
    panel.nameFieldStringValue = "loom-diagnostics.json"
    guard panel.runModal() == .OK, let destination = panel.url else { return }
    do {
      try exporter.export(preview, to: destination)
      dismiss()
      NSWorkspace.shared.activateFileViewerSelecting([destination])
    } catch {
      exportError = "The diagnostics bundle could not be exported."
    }
  }

  private func abbreviated(_ digest: String) -> String {
    guard digest.count > 20 else { return digest }
    return "\(digest.prefix(12))...\(digest.suffix(8))"
  }

  private func excludedLabel(_ value: String) -> String {
    value.replacingOccurrences(of: "_", with: " ").capitalized
  }
}

func missionDisplayTitle(
  candidate: String,
  missionID: String,
  teamInstanceID: String,
  teams: [LocalProductTeamSummary]
) -> String {
  let safeCandidate = SafeText.sanitize(candidate, limit: 96)
  let safeMissionID = SafeText.sanitize(missionID, limit: 96)
  let safeTeamID = SafeText.sanitize(teamInstanceID, limit: 96)
  if !safeCandidate.isEmpty,
    safeCandidate != safeMissionID,
    safeCandidate != safeTeamID
  {
    return safeCandidate
  }
  guard
    let team = teams.first(where: {
      $0.teamInstanceID == teamInstanceID
    })
  else {
    return "Mission"
  }
  return LocalProductExperience.visibleName(
    team.displayName,
    internalID: team.teamInstanceID,
    fallback: "Mission"
  )
}

func suggestedMissionTitle(from objective: String) -> String {
  let firstLine = objective
    .split(whereSeparator: { $0.isNewline })
    .first
    .map(String.init) ?? ""
  let compact = firstLine
    .trimmingCharacters(in: .whitespacesAndNewlines)
    .split(whereSeparator: { $0.isWhitespace })
    .joined(separator: " ")
  return String(compact.prefix(72))
}

func missionNodeDisplayTitle(
  nodeID: String,
  topology: [LocalProductMissionNode],
  pulse: [LocalProductMissionPulse]
) -> String {
  if let node = topology.first(where: { $0.logicalNodeID == nodeID }) {
    let title = LocalProductExperience.visibleName(
      node.title,
      internalID: node.logicalNodeID,
      fallback: ""
    )
    if !title.isEmpty { return title }
    let role = SafeText.sanitize(node.role, limit: 48)
    if !role.isEmpty { return role.capitalized }
  }
  if let role = pulse.first(where: { $0.nodeID == nodeID })?.role {
    let safeRole = SafeText.sanitize(role, limit: 48)
    if !safeRole.isEmpty { return safeRole.capitalized }
  }
  return nodeID.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    ? "No current step"
    : "Mission step"
}

func missionHumanStatus(_ value: String) -> String {
  value.replacingOccurrences(of: "_", with: " ").capitalized
}

func missionPlanSummary(
  nodeCount: Int,
  completedNodeCount: Int,
  reviewNodeCount: Int
) -> String {
  let unit = nodeCount == 1 ? "step" : "steps"
  return "\(nodeCount) \(unit) · \(completedNodeCount) complete · \(reviewNodeCount) in review"
}

struct MissionResultPresentation: Equatable {
  let openURL: URL
  let revealURL: URL
  let isWebResult: Bool
}

struct MissionContinuationPreparation: Equatable {
  let title: String
  let objective: String
  let teamInstanceID: String
  let newAttempt: Bool
}

func missionContinuationAcceptsInput(status: String) -> Bool {
  ["blocked", "failed", "cancelled"].contains(status)
}

func missionContinuationPreparation(
  status: String,
  title: String,
  teamInstanceID: String,
  draft: String
) -> MissionContinuationPreparation? {
  guard missionContinuationAcceptsInput(status: status) else { return nil }
  let boundedTitle = title.trimmingCharacters(in: .whitespacesAndNewlines)
  let objective = draft.trimmingCharacters(in: .whitespacesAndNewlines)
  guard !boundedTitle.isEmpty, boundedTitle.utf8.count <= 96,
    !teamInstanceID.isEmpty,
    (1...4_096).contains(objective.utf8.count)
  else { return nil }
  return MissionContinuationPreparation(
    title: boundedTitle,
    objective: objective,
    teamInstanceID: teamInstanceID,
    newAttempt: true
  )
}

func missionContinuationSheetTitle(newAttempt: Bool) -> String {
  newAttempt ? "Continue Mission" : "New Mission"
}

struct MissionActivityEntry: Equatable, Identifiable {
  let id: String
  let logicalNodeID: String
  let attemptNumber: Int
  let title: String
  let route: String
  let status: String
  let text: String
  let isTentative: Bool
  let isTruncated: Bool
}

private struct MissionActivityKey: Hashable {
  let logicalNodeID: String
  let attemptNumber: Int
}

func missionActivityNeedsRefresh(status: String) -> Bool {
  !["succeeded", "failed", "cancelled", "blocked", "human_required"]
    .contains(status)
}

func missionActivityVisibleText(
  _ text: String,
  expanded: Bool,
  limit: Int = 2_400
) -> String {
  guard !expanded, limit > 1, text.count > limit else { return text }
  return "…" + text.suffix(limit - 1)
}

func missionActivityEntries(
  mission: LocalProductMissionSummary?,
  timeline: LocalProductTimelinePage?
) -> [MissionActivityEntry] {
  let records = (timeline?.records ?? []).sorted {
    if $0.occurredAt != $1.occurredAt { return $0.occurredAt < $1.occurredAt }
    return $0.sourceSequence < $1.sourceSequence
  }
  var keys: [MissionActivityKey] = []
  var textByKey: [MissionActivityKey: String] = [:]
  var tentativeByKey: [MissionActivityKey: Bool] = [:]
  var statusByKey: [MissionActivityKey: String] = [:]

  for record in records {
    let key = MissionActivityKey(
      logicalNodeID: record.logicalNodeID,
      attemptNumber: max(record.attemptNumber, 0)
    )
    if textByKey[key] == nil { keys.append(key) }
    if !record.payload.textDelta.isEmpty {
      textByKey[key, default: ""].append(record.payload.textDelta)
    }
    tentativeByKey[key, default: false] =
      tentativeByKey[key, default: false] || record.authority == "tentative"
    if !record.payload.status.isEmpty {
      statusByKey[key] = record.payload.status
    }
  }

  for node in timeline?.board.nodes ?? [] {
    let key = MissionActivityKey(
      logicalNodeID: node.logicalNodeID,
      attemptNumber: max(node.currentAttempt, 0)
    )
    if textByKey[key] == nil {
      keys.append(key)
      textByKey[key] = ""
    }
  }

  return keys.map { key in
    let node = timeline?.board.nodes.first(where: {
      $0.logicalNodeID == key.logicalNodeID
        && ($0.currentAttempt == key.attemptNumber || key.attemptNumber == 0)
    })
    let rawText = textByKey[key] ?? ""
    let textLimit = 24_000
    let truncated = rawText.count > textLimit
    let route = node.map { node in
      let account = node.providerAccountID.isEmpty
        ? node.providerID : node.providerAccountID
      return [node.harnessAdapter, account, node.modelID]
        .filter { !$0.isEmpty }
        .joined(separator: " · ")
    } ?? ""
    let projectedStatus = node?.status
      ?? mission?.teamPulse.first(where: {
        $0.nodeID == key.logicalNodeID
          && ($0.attemptNumber == key.attemptNumber || key.attemptNumber == 0)
      })?.state
      ?? statusByKey[key]
      ?? "working"
    return MissionActivityEntry(
      id: "\(key.logicalNodeID):\(key.attemptNumber)",
      logicalNodeID: key.logicalNodeID,
      attemptNumber: key.attemptNumber,
      title: missionNodeDisplayTitle(
        nodeID: key.logicalNodeID,
        topology: mission?.topology ?? [],
        pulse: mission?.teamPulse ?? []
      ),
      route: route,
      status: projectedStatus,
      text: String(rawText.prefix(textLimit)),
      isTentative: tentativeByKey[key] ?? false,
      isTruncated: truncated
    )
  }
}

func missionResultPresentation(
  workspacePath: String,
  fileManager: FileManager = .default
) -> MissionResultPresentation? {
  let trimmed = workspacePath.trimmingCharacters(in: .whitespacesAndNewlines)
  guard !trimmed.isEmpty, trimmed == workspacePath, trimmed.hasPrefix("/") else {
    return nil
  }
  let workspaceURL = URL(fileURLWithPath: trimmed).standardizedFileURL
  guard workspaceURL.path == trimmed else { return nil }
  var isDirectory: ObjCBool = false
  guard fileManager.fileExists(
    atPath: workspaceURL.path,
    isDirectory: &isDirectory
  ), isDirectory.boolValue else { return nil }

  let indexURL = workspaceURL.appendingPathComponent("index.html").standardizedFileURL
  var indexIsDirectory: ObjCBool = false
  if fileManager.fileExists(atPath: indexURL.path, isDirectory: &indexIsDirectory),
    !indexIsDirectory.boolValue
  {
    return MissionResultPresentation(
      openURL: indexURL,
      revealURL: indexURL,
      isWebResult: true
    )
  }
  return MissionResultPresentation(
    openURL: workspaceURL,
    revealURL: workspaceURL,
    isWebResult: false
  )
}

func missionProviderPolicyValue(_ value: String) -> String {
  switch value {
  case "external_provider": return "External provider"
  case "enterprise_tenant": return "Enterprise tenant"
  case "local_runtime": return "Local runtime"
  case "provider_default": return "Provider default"
  case "zero_data_retention": return "Zero data retention"
  case "limited_retention": return "Limited retention"
  case "us": return "US"
  case "eu": return "EU"
  case "apac": return "APAC"
  default: return missionHumanStatus(value)
  }
}

struct SideTaskDrawerPresentation: Equatable {
  let purpose: String
  let uncertainty: String
  let scopeDelta: String
  let nextAction: String
}

func sideTaskDrawerPresentation(
  _ sideTask: LocalProductSideTaskSummary
) -> SideTaskDrawerPresentation {
  let uncertainty =
    sideTask.uncertainties.isEmpty
    ? "None recorded"
    : sideTask.uncertainties.joined(separator: "; ")
  let scopeDelta =
    sideTask.scopeDelta.isEmpty
    ? "No parent scope expansion"
    : sideTask.scopeDelta.joined(separator: "; ")
  let nextAction: String
  if !sideTask.availableDecisions.isEmpty {
    nextAction = "Choose an explicit parent decision"
  } else if sideTask.status == "report_delivered" {
    nextAction = "Review the authorized report"
  } else if sideTask.effectStatus == "pending" {
    nextAction = "Wait for the authorized parent effect"
  } else if sideTask.effectStatus == "completed" {
    nextAction = "Parent effect completed"
  } else if sideTask.status == "decided" {
    nextAction = "Parent decision recorded; no parent effect required"
  } else if sideTask.status == "admitted" || sideTask.status == "running" {
    nextAction = "Wait for the authorized handoff"
  } else {
    nextAction = "Review the authoritative Side-task status"
  }
  return SideTaskDrawerPresentation(
    purpose: "Purpose · \(missionHumanStatus(sideTask.purpose))",
    uncertainty: "Uncertainty · \(uncertainty)",
    scopeDelta: "Scope · \(scopeDelta)",
    nextAction: "Next action · \(nextAction)"
  )
}

enum MissionSnapshotPresentation: Equatable {
  case current
  case partial(String)
  case preserved(String)
  case unavailable(String)

  var isCurrent: Bool { self == .current }

  var notice: String? {
    switch self {
    case .current:
      return nil
    case .partial(let reason):
      return "Partial view · \(reason). Some items may be missing."
    case .preserved(let reason):
      return "Showing a preserved view · \(reason). New items may be missing."
    case .unavailable(let reason):
      return "Current view unavailable · \(reason)."
    }
  }
}

func missionSnapshotPresentation(
  connection: LocalProductConnectionState,
  hasSnapshot: Bool
) -> MissionSnapshotPresentation {
  guard hasSnapshot else {
    switch connection {
    case .partial(let reason), .stale(let reason),
      .offline(let reason), .fatal(let reason):
      return .unavailable(reason)
    case .loading:
      return .unavailable("loading")
    case .online:
      return .unavailable("state_unavailable")
    }
  }
  switch connection {
  case .online:
    return .current
  case .partial(let reason):
    return .partial(reason)
  case .stale(let reason), .offline(let reason), .fatal(let reason):
    return .preserved(reason)
  case .loading:
    return .preserved("refreshing")
  }
}

struct MissionAttentionEmptyCopy: Equatable {
  let icon: String
  let title: String
  let detail: String
}

func missionAttentionEmptyCopy(
  presentation: MissionSnapshotPresentation
) -> MissionAttentionEmptyCopy {
  if presentation.isCurrent {
    return MissionAttentionEmptyCopy(
      icon: "checkmark.circle",
      title: "Nothing needs you",
      detail: "No approval, blocked, retry-exhausted or verification action is pending."
    )
  }
  return MissionAttentionEmptyCopy(
    icon: "clock.arrow.circlepath",
    title: "No recorded attention in this view",
    detail: presentation.notice ?? "Current Attention is unavailable."
  )
}

struct MissionInspectorSection: Equatable {
  let heading: String
  let notice: String?
  let rows: [String]
  let providerAccountRows: [MissionProviderAccountGovernanceRow]
  let emptyMessage: String
  let incidentIDs: [String?]
  let diagnosticActions: [Bool]
  let retryReviewActions: [Bool]
  let credentialVaultRecoveryActions: [Bool]
  let agentRecoveryActions: [Bool]
  let rowSystemImages: [String]
  let rowAccessibilityLabels: [String]

  init(
    heading: String,
    notice: String? = nil,
    rows: [String],
    providerAccountRows: [MissionProviderAccountGovernanceRow] = [],
    emptyMessage: String,
    incidentIDs: [String?] = [],
    diagnosticActions: [Bool] = [],
    retryReviewActions: [Bool] = [],
    credentialVaultRecoveryActions: [Bool] = [],
    agentRecoveryActions: [Bool] = [],
    rowSystemImages: [String] = [],
    rowAccessibilityLabels: [String] = []
  ) {
    self.heading = heading
    self.notice = notice
    self.rows = rows
    self.providerAccountRows = providerAccountRows
    self.emptyMessage = emptyMessage
    self.incidentIDs = incidentIDs
    self.diagnosticActions = diagnosticActions
    self.retryReviewActions = retryReviewActions
    self.credentialVaultRecoveryActions = credentialVaultRecoveryActions
    self.agentRecoveryActions = agentRecoveryActions
    self.rowSystemImages = rowSystemImages
    self.rowAccessibilityLabels = rowAccessibilityLabels
  }

  func incidentID(at index: Int) -> String? {
    guard incidentIDs.indices.contains(index) else { return nil }
    return incidentIDs[index]
  }

  func canViewDiagnostics(at index: Int) -> Bool {
    diagnosticActions.indices.contains(index) && diagnosticActions[index]
  }

  func canReviewRetry(at index: Int) -> Bool {
    retryReviewActions.indices.contains(index) && retryReviewActions[index]
  }

  func canRecoverCredentialVault(at index: Int) -> Bool {
    credentialVaultRecoveryActions.indices.contains(index)
      && credentialVaultRecoveryActions[index]
  }

  func canRecoverAgentAttempt(at index: Int) -> Bool {
    agentRecoveryActions.indices.contains(index) && agentRecoveryActions[index]
  }

  func systemImage(at index: Int, fallback: String) -> String {
    guard rowSystemImages.indices.contains(index) else { return fallback }
    return rowSystemImages[index]
  }

  func accessibilityLabel(at index: Int) -> String? {
    guard rowAccessibilityLabels.indices.contains(index) else { return nil }
    return rowAccessibilityLabels[index]
  }
}

struct MissionProviderAccountGovernanceRow: Equatable, Identifiable {
  var id: String { "\(providerName):\(providerAccountID)" }
  let providerName: String
  let providerAccountID: String
  let attempts: String
  let reliability: String
  let accountingCoverage: String
  let tokens: String
  let costSource: String
  let policyRevision: String
  let concurrencyCeiling: String
  let dispatchCeiling: String
  let budgetCeiling: String
  let completeness: String
  let isIncomplete: Bool

  var accessibilityLabel: String {
    "\(providerName) \(providerAccountID) Provider Account governance, \(completeness)"
  }
}

func missionProviderAccountTokenSummary(
  inputTokens: Int64,
  outputTokens: Int64,
  cacheReadTokens: Int64,
  cacheWriteTokens: Int64,
  totalTokens: Int64,
  usageAttemptCount: Int,
  attemptCount: Int
) -> String {
  var components = ["Input \(inputTokens)", "Output \(outputTokens)"]
  if cacheReadTokens > 0 {
    components.append("Cache read \(cacheReadTokens)")
  }
  if cacheWriteTokens > 0 {
    components.append("Cache write \(cacheWriteTokens)")
  }
  components.append("Total \(totalTokens)")
  components.append("usage on \(usageAttemptCount)/\(attemptCount) attempts")
  return components.joined(separator: " · ")
}

func missionProviderAccountGovernanceRow(
  _ account: LocalProductProviderAccountAccounting
) -> MissionProviderAccountGovernanceRow {
  let attemptNoun = account.attemptCount == 1 ? "attempt" : "attempts"
  let costs = account.costs.map {
    missionCostSourceLabel(
      source: $0.source,
      microunits: $0.amountMicrounits,
      currency: $0.currency
    )
  }.joined(separator: " + ")
  let policyAvailable = account.policyAvailable
  let isIncomplete = account.aggregationOverflow
    || account.accountingAttemptCount < account.attemptCount
  return MissionProviderAccountGovernanceRow(
    providerName: missionHumanStatus(account.providerID),
    providerAccountID: account.providerAccountID,
    attempts: "\(account.attemptCount) \(attemptNoun) · \(account.failedAttempts) failed",
    reliability: "\(missionErrorRateLabel(basisPoints: account.errorRateBasisPoints)) · \(account.rateLimitedAttempts) rate limited",
    accountingCoverage: "\(account.accountingAttemptCount)/\(account.attemptCount) attempts accounted",
    tokens: missionProviderAccountTokenSummary(
      inputTokens: account.inputTokens,
      outputTokens: account.outputTokens,
      cacheReadTokens: account.cacheReadTokens,
      cacheWriteTokens: account.cacheWriteTokens,
      totalTokens: account.totalTokens,
      usageAttemptCount: account.usageAttemptCount,
      attemptCount: account.attemptCount
    ),
    costSource: "\(costs.isEmpty ? "Not reported" : costs) · cost on \(account.costAttemptCount)/\(account.attemptCount) attempts",
    policyRevision: policyAvailable ? "Revision \(account.policyRevision)" : "Unavailable",
    concurrencyCeiling: policyAvailable
      ? "\(account.activeAttempts) active / \(account.maximumConcurrentAttempts) maximum"
      : "Unavailable",
    dispatchCeiling: policyAvailable
      ? "\(account.maximumDispatchStarts) starts / \(account.dispatchWindowSeconds)s"
      : "Unavailable",
    budgetCeiling: policyAvailable
      ? "\(account.activeAssignedBudgetUnits) active / \(account.maximumAssignedBudgetUnits) maximum"
      : "Unavailable",
    completeness: isIncomplete ? "Incomplete accounting" : "Complete accounting",
    isIncomplete: isIncomplete
  )
}

func missionVaultRecoveryAvailable(
  stage: String,
  diagnosticAvailable: Bool,
  retryable: Bool
) -> Bool {
  guard diagnosticAvailable, !retryable else { return false }
  return [
    "vault_key_load",
    "vault_open",
    "vault_decrypt",
    "vault_aad_validation",
    "vault_rotation",
    "vault_recovery",
  ].contains(stage)
}

func missionAttemptRecoveryAvailable(
  stage: String,
  code: String,
  diagnosticAvailable: Bool,
  retryable: Bool
) -> Bool {
  diagnosticAvailable && !retryable && stage == "agent_attempt_reconcile"
    && code == "agent_input_resume_required"
}

func missionAgentFailureLabel(
  stage: String,
  code: String,
  retryable: Bool
) -> String {
  guard stage == "agent_attempt_reconcile" else {
    return retryable ? "Retry available" : "Manual recovery"
  }
  switch code {
  case "agent_input_resume_required":
    return "Resume approval required"
  case "provider_outcome_uncertain":
    return "Provider outcome uncertain"
  case "agent_input_recovery_unavailable":
    return "Encrypted input unavailable"
  case "agent_input_recovery_conflict":
    return "Recovery state conflict"
  default:
    return "Manual recovery"
  }
}

func missionNodeKindLabel(_ node: LocalProductNode) -> String? {
  switch node.nodeKind {
  case "route_sibling":
    return "Parallel provider route"
  case "aggregation":
    return "Synthesis"
  default:
    return nil
  }
}

func teamAgentTitle(_ agent: LocalProductTeamAgentSummary) -> String {
  agent.roleKind == "main" ? "Main Agent" : "Subagent"
}

func teamAgentRouteLabel(_ agent: LocalProductTeamAgentSummary) -> String {
  let account = agent.providerAccountID.isEmpty
    ? missionHumanStatus(agent.providerID)
    : "\(missionHumanStatus(agent.providerID)) / \(agent.providerAccountID)"
  return [missionHumanStatus(agent.harnessAdapter), account, agent.modelID]
    .filter { !$0.isEmpty }
    .joined(separator: " · ")
}

func missionInspectorSection(
  tab: MissionInspectorTab,
  record: LocalProductMissionSummary?,
  timeline: LocalProductTimelinePage?
) -> MissionInspectorSection {
  switch tab {
  case .team:
    let boardNodes: [LocalProductNode]
    let providerAccounts: [LocalProductProviderAccountAccounting]
    if let record,
      let timeline,
      timeline.teamInstanceID == record.teamInstanceID
    {
      boardNodes = timeline.board.nodes
      providerAccounts = timeline.board.providerAccounts
    } else {
      boardNodes = []
      providerAccounts = []
    }
    var rows: [String]
    var incidentIDs: [String?]
    var diagnosticActions: [Bool]
    var retryReviewActions: [Bool]
    var credentialVaultRecoveryActions: [Bool]
    var agentRecoveryActions: [Bool]
    var rowSystemImages: [String]
    var rowAccessibilityLabels: [String]
    if boardNodes.contains(where: \.executionBindingAvailable) {
      rows = boardNodes.map { node in
        let title = missionNodeDisplayTitle(
          nodeID: node.logicalNodeID,
          topology: record?.topology ?? [],
          pulse: record?.teamPulse ?? []
        )
        var details = [
          [
            title,
            missionHumanStatus(node.status),
            node.currentAttempt > 0 ? "Attempt \(node.currentAttempt)" : "Not started",
          ].joined(separator: " · ")
        ]
        if let kind = missionNodeKindLabel(node) {
          details.append(kind)
        }
        if node.executionBindingAvailable {
          let account = node.providerAccountID.isEmpty
            ? "Native auth"
            : "\(missionHumanStatus(node.providerID)) / \(node.providerAccountID)"
          details.append(
            [missionHumanStatus(node.harnessAdapter), account, node.modelID]
              .filter { !$0.isEmpty }
              .joined(separator: " · ")
          )
          details.append(
            node.reasoningEffort.isEmpty
              ? "Provider default"
              : "\(missionHumanStatus(node.reasoningEffort)) reasoning"
          )
          details.append(
            node.providerAccountID.isEmpty
              ? "Native auth"
              : "Credential v\(node.credentialRevision)"
          )
          details.append("Timeout \(node.timeoutNanoseconds / 1_000_000_000)s")
          if let budget = node.bindingBudgetCredits {
            details.append("Binding budget \(budget)")
          }
          if !node.capabilities.isEmpty {
            details.append("Capabilities \(node.capabilities.joined(separator: ", "))")
          }
          if node.providerAccountPolicyAvailable {
            details.append(
              "Account policy v\(node.providerAccountPolicyVersion) r\(node.providerAccountPolicyRevision)"
            )
            if node.providerAccountPolicyVersion == 2 {
              details.append(
                "Trust \(missionProviderPolicyValue(node.providerAccountTrustDomain))"
              )
              details.append(
                "Retention \(missionProviderPolicyValue(node.providerAccountRetentionMode))"
              )
              details.append(
                "Region \(missionProviderPolicyValue(node.providerAccountDataRegion))"
              )
            } else {
              details.append("Disclosure unspecified")
            }
            details.append("Assigned budget \(node.providerAccountAssignedBudgetUnits)")
          }
          if node.providerModelRateCardAvailable {
            details.append(
              "Rate card r\(node.providerModelRateCardRevision) \(node.providerModelRateCardCurrency)"
            )
          }
          if node.contextCapsuleAvailable {
            details.append("Context policy r\(node.disclosurePolicyVersion)")
            details.append(
              "Context \(node.contextTokenCount)/\(node.contextTokenBudget) tokens"
            )
            if node.contextOmissionCount > 0 {
              details.append("\(node.contextOmissionCount) context omissions")
            }
          }
        }
        if let fallback = missionFallbackGovernanceLabel(node) {
          details.append(fallback)
        }
        if !node.terminalReason.isEmpty {
          details.append(missionHumanStatus(node.terminalReason))
        }
        if node.failureDiagnosticAvailable {
          details.append(missionHumanStatus(node.failureStage))
          details.append(
            missionAgentFailureLabel(
              stage: node.failureStage,
              code: node.failureCode,
              retryable: node.failureRetryable
            )
          )
        }
        if node.testReportAvailable {
          details.append(
            "Tests \(node.testReportPassedCount) passed, \(node.testReportFailedCount) failed"
          )
          details.append(
            "Latest \(missionHumanStatus(node.latestTestRunner)) "
              + "\(missionHumanStatus(node.latestTestScope)) "
              + missionHumanStatus(node.latestTestOutcome)
          )
        }
        if !node.incidentID.isEmpty {
          details.append("Incident \(node.incidentID.prefix(8))")
        }
        if node.accountingAvailable && node.usageObserved {
          details.append("\(node.totalTokens) tokens")
        }
        if node.accountingAvailable && node.costObserved {
			details.append(
				missionCostSourceLabel(
					source: node.costSource,
					microunits: node.costMicrounits,
					currency: node.costCurrency
				)
			)
        }
        return details.joined(separator: "\n")
      }
      incidentIDs = boardNodes.map {
        $0.incidentID.isEmpty ? nil : $0.incidentID
      }
      diagnosticActions = boardNodes.map(\.failureDiagnosticAvailable)
      retryReviewActions = boardNodes.map {
        $0.failureDiagnosticAvailable && $0.failureRetryable
      }
      credentialVaultRecoveryActions = boardNodes.map {
        missionVaultRecoveryAvailable(
          stage: $0.failureStage,
          diagnosticAvailable: $0.failureDiagnosticAvailable,
          retryable: $0.failureRetryable
        )
      }
      agentRecoveryActions = boardNodes.map {
        missionAttemptRecoveryAvailable(
          stage: $0.failureStage,
          code: $0.failureCode,
          diagnosticAvailable: $0.failureDiagnosticAvailable,
          retryable: $0.failureRetryable
        )
      }
      rowSystemImages = boardNodes.map { node in
        node.nodeKind == "aggregation" ? "arrow.triangle.merge" : "person.crop.circle"
      }
      rowAccessibilityLabels = boardNodes.map { node in
        node.nodeKind == "aggregation" ? "Synthesis status" : "Agent status"
      }
    } else {
      rows = (record?.teamPulse ?? []).map {
        "\($0.role.capitalized) · \(missionHumanStatus($0.state)) · Attempt \($0.attemptNumber)\nBinding unavailable · Refresh Mission"
      }
      incidentIDs = rows.map { _ in nil }
      diagnosticActions = rows.map { _ in false }
      retryReviewActions = rows.map { _ in false }
      credentialVaultRecoveryActions = rows.map { _ in false }
      agentRecoveryActions = rows.map { _ in false }
      rowSystemImages = rows.map { _ in "person.crop.circle" }
      rowAccessibilityLabels = rows.map { _ in "Agent status" }
    }
    return MissionInspectorSection(
      heading: "Team Pulse",
      notice: timeline?.gap == nil
        ? nil
        : "Activity history is incomplete. Current Agent bindings are shown; refresh before relying on Changes or Evidence.",
      rows: rows,
      providerAccountRows: providerAccounts.map(missionProviderAccountGovernanceRow),
      emptyMessage: "No Team presence is available.",
      incidentIDs: incidentIDs,
      diagnosticActions: diagnosticActions,
      retryReviewActions: retryReviewActions,
      credentialVaultRecoveryActions: credentialVaultRecoveryActions,
      agentRecoveryActions: agentRecoveryActions,
      rowSystemImages: rowSystemImages,
      rowAccessibilityLabels: rowAccessibilityLabels
    )
  case .plan:
    let rows = (record?.topology ?? []).map { node in
      let title = missionNodeDisplayTitle(
        nodeID: node.logicalNodeID,
        topology: record?.topology ?? [],
        pulse: record?.teamPulse ?? []
      )
      let attempt =
        node.attemptNumber > 0
        ? " · Attempt \(node.attemptNumber)"
        : ""
      return "\(title) · \(missionHumanStatus(node.status))\(attempt)"
    }
    return MissionInspectorSection(
      heading: "Plan",
      rows: rows,
      emptyMessage: "No authoritative Plan is available."
    )
  case .changes:
    guard let record,
      let timeline,
      timeline.teamInstanceID == record.teamInstanceID,
      timeline.gap == nil,
      !timeline.hasMore
    else {
      return MissionInspectorSection(
        heading: "Changes",
        rows: [],
        emptyMessage: "Changes are unavailable until complete authoritative activity loads."
      )
    }
    let labels = [
      "ready_for_review": "Ready for review",
      "verification_recorded": "Verification recorded",
      "verification_rejected": "Verification rejected",
      "node_acceptance": "Acceptance recorded",
    ]
    let rows = timeline.records.compactMap { item -> String? in
      guard item.authority == "journal",
        item.teamInstanceID == record.teamInstanceID,
        let label = labels[item.kind]
      else { return nil }
      let node = missionNodeDisplayTitle(
        nodeID: item.logicalNodeID,
        topology: record.topology,
        pulse: record.teamPulse
      )
      let attempt =
        item.attemptNumber > 0
        ? " · Attempt \(item.attemptNumber)"
        : ""
      return "\(label) · \(node)\(attempt)"
    }
    return MissionInspectorSection(
      heading: "Changes",
      rows: rows,
      emptyMessage: "No review or acceptance changes are recorded."
    )
  case .evidence:
    guard let record,
      let timeline,
      timeline.teamInstanceID == record.teamInstanceID,
      timeline.gap == nil,
      !timeline.hasMore
    else {
      return MissionInspectorSection(
        heading: "Evidence",
        rows: [],
        emptyMessage: "Evidence is unavailable until complete authoritative activity loads."
      )
    }
    var ordinal = 0
    let rows = timeline.records.compactMap { item -> String? in
      guard item.kind == "evidence_available",
        item.authority == "journal",
        item.teamInstanceID == record.teamInstanceID
      else {
        return nil
      }
      ordinal += 1
      let node = missionNodeDisplayTitle(
        nodeID: item.logicalNodeID,
        topology: record.topology,
        pulse: record.teamPulse
      )
      let attempt =
        item.attemptNumber > 0
        ? " · Attempt \(item.attemptNumber)"
        : ""
      return "Evidence \(ordinal) · \(node)\(attempt)"
    }
    return MissionInspectorSection(
      heading: "Evidence",
      rows: rows,
      emptyMessage: "No Evidence reference is recorded."
    )
  }
}

func missionErrorRateLabel(basisPoints: Int) -> String {
  let whole = basisPoints / 100
  let remainder = basisPoints % 100
  if remainder == 0 {
    return "\(whole)% error rate"
  }
  if remainder % 10 == 0 {
    return "\(whole).\(remainder / 10)% error rate"
  }
  return "\(whole).\(String(format: "%02d", remainder))% error rate"
}

func missionCostLabel(microunits: Int64, currency: String) -> String {
  let whole = microunits / 1_000_000
  let remainder = microunits % 1_000_000
  guard remainder != 0 else {
    return "Cost \(whole) \(currency)"
  }
  let fractional = String(format: "%06lld", remainder)
    .replacingOccurrences(of: "0+$", with: "", options: .regularExpression)
  return "Cost \(whole).\(fractional) \(currency)"
}

func missionCostSourceLabel(source: String, microunits: Int64, currency: String) -> String {
	let amount = missionCostLabel(microunits: microunits, currency: currency)
		.replacingOccurrences(of: "Cost ", with: "")
	switch source {
	case "provider_reported":
		return "Provider reported cost \(amount)"
	case "harness_reported":
		return "Harness reported cost \(amount)"
	case "rate_card_estimate":
		return "Rate-card estimate \(amount)"
	case "legacy_unspecified":
		return "Legacy source cost \(amount)"
	default:
		return "Cost source unavailable"
	}
}

func missionFallbackGovernanceLabel(_ node: LocalProductNode) -> String? {
  guard node.fallbackConfigured else { return nil }
  if node.fallbackConsumed {
    return "Fallback executed"
  }
  if node.recoveryApprovalRequired {
    if node.fallbackApprovalAvailable {
      return "Fallback approved v\(node.fallbackApprovalVersion)"
    }
    return "Fallback approval required"
  }
  return "Fallback configured"
}

private struct PendingEvolutionAssetMutation {
  let action: String
  let candidateID: String
  let definitionID: String
  let revisionID: String
}

private enum MissionContextItemKind {
  case constraint
  case decision

  var title: String {
    switch self {
    case .constraint: return "Confirmed constraints"
    case .decision: return "Accepted decisions"
    }
  }

  var singularTitle: String {
    switch self {
    case .constraint: return "Constraint"
    case .decision: return "Decision"
    }
  }

  var removeLabel: String {
    switch self {
    case .constraint: return "constraint"
    case .decision: return "decision"
    }
  }
}

private struct PendingToolRecoveryDecision: Identifiable {
  let candidate: LocalProductToolRecoveryCandidate
  let action: LocalProductToolRecoveryAction

  var id: String { candidate.candidateDigest + ":" + action.rawValue }
}

func missionToolRecoveryActionLabel(_ action: LocalProductToolRecoveryAction) -> String {
  switch action {
  case .abortAttempt: return "Abort Attempt"
  case .acceptObservedEffect: return "Accept observed effect"
  case .retryInNewAttempt: return "Retry in new Attempt"
  }
}

func missionToolRecoveryActionIcon(_ action: LocalProductToolRecoveryAction) -> String {
  switch action {
  case .abortAttempt: return "stop.circle"
  case .acceptObservedEffect: return "checkmark.seal"
  case .retryInNewAttempt: return "arrow.triangle.2.circlepath"
  }
}

func missionToolRecoveryConfirmationDetail(
  _ action: LocalProductToolRecoveryAction
) -> String {
  switch action {
  case .abortAttempt:
    return "Close this Attempt at the uncertain side-effect boundary. Loom will not rerun the original ToolCall."
  case .acceptObservedEffect:
    return "Accept only the trusted observation bound to this candidate. Loom will not rerun the original ToolCall."
  case .retryInNewAttempt:
    return "Authorize the trusted replacement Attempt. The original ToolCall remains closed and will not be rerun."
  }
}

func resolvedMissionTeamID(
  preferred: String?,
  executableTeamIDs: [String]
) -> String {
  if let preferred, executableTeamIDs.contains(preferred) {
    return preferred
  }
  return executableTeamIDs.first ?? ""
}

func missionShouldStartNewAttempt(
  teamInstanceID: String,
  missions: [LocalProductMissionSummary]
) -> Bool {
  let matching = missions.filter { $0.teamInstanceID == teamInstanceID }
  guard !matching.isEmpty else { return false }
  let terminalStatuses: Set<String> = [
    "succeeded", "failed", "cancelled", "degraded", "blocked",
    "human_required", "ready_for_review",
  ]
  return matching.allSatisfy { terminalStatuses.contains($0.status) }
}

struct EndpointReviewPresentation: Equatable {
  let source: String
  let provider: String
  let protocolName: String
  let origin: String
  let endpoint: String
  let models: [String]
  let credentialEgressRisk: String
  let reviewButtonAccessibilityLabel: String
  let sheetAccessibilityLabel: String
}

private func endpointReviewOrigin(_ endpoint: String) -> String {
  guard let parts = URLComponents(string: endpoint),
    let scheme = parts.scheme,
    let host = parts.host
  else { return endpoint }
  if let port = parts.port {
    return "\(scheme)://\(host):\(port)"
  }
  return "\(scheme)://\(host)"
}

func endpointReviewPresentation(
  _ candidate: LocalProductCredentialImportCandidate
) -> EndpointReviewPresentation {
  let origin = endpointReviewOrigin(candidate.endpoint)
  return EndpointReviewPresentation(
    source: candidate.sourceApplication,
    provider: "\(candidate.displayName) (\(candidate.targetProviderID))",
    protocolName: candidate.protocolName,
    origin: origin,
    endpoint: candidate.endpoint,
    models: candidate.modelIDs,
    credentialEgressRisk:
      "Approving this endpoint records permission for this exact destination. It does not import or send a credential. A separate import step is required before Loom can authenticate to \(origin).",
    reviewButtonAccessibilityLabel:
      "Review custom endpoint for \(candidate.displayName)",
    sheetAccessibilityLabel:
      "Endpoint review for \(candidate.displayName) from \(candidate.sourceApplication), credential egress pending"
  )
}

struct EndpointReviewConfirmationSheet: View {
  @Environment(\.dismiss) private var dismiss
  let candidate: LocalProductCredentialImportCandidate
  let onApprove: () -> Void

  private var presentation: EndpointReviewPresentation {
    endpointReviewPresentation(candidate)
  }

  var body: some View {
    VStack(alignment: .leading, spacing: 0) {
      HStack(alignment: .top, spacing: 16) {
        VStack(alignment: .leading, spacing: 4) {
          Text("Review Endpoint")
            .font(.title2.weight(.semibold))
          Text("Confirm the exact destination before any credential import can be proposed.")
            .foregroundStyle(.secondary)
            .fixedSize(horizontal: false, vertical: true)
        }
        Spacer(minLength: 12)
        Button {
          dismiss()
        } label: {
          Image(systemName: "xmark")
        }
        .buttonStyle(.plain)
        .accessibilityLabel("Close endpoint review")
        .help("Close")
        .loomActionTarget()
      }
      .padding(24)

      Divider()

      ScrollView {
        VStack(alignment: .leading, spacing: 0) {
          VStack(alignment: .leading, spacing: 8) {
            Label("Credential egress risk", systemImage: "exclamationmark.shield.fill")
              .font(.headline)
              .foregroundStyle(LoomGraphite.statusWarning)
            Text(presentation.credentialEgressRisk)
              .fixedSize(horizontal: false, vertical: true)
          }
          .frame(maxWidth: .infinity, alignment: .leading)
          .padding(20)
          .background(LoomGraphite.statusWarning.opacity(0.09))

          endpointReviewField("Source", value: presentation.source)
          Divider()
          endpointReviewField("Provider", value: presentation.provider)
          Divider()
          endpointReviewField("Protocol", value: presentation.protocolName)
          Divider()
          endpointReviewField("Origin", value: presentation.origin)
          Divider()
          endpointReviewField(
            "Exact endpoint",
            value: presentation.endpoint,
            monospaced: true,
            copyable: true,
            multiline: true
          )
          Divider()

          VStack(alignment: .leading, spacing: 8) {
            Text("Models")
              .font(.caption.weight(.semibold))
              .foregroundStyle(.secondary)
            ForEach(Array(presentation.models.enumerated()), id: \.offset) { _, model in
              Text(model)
                .textSelection(.enabled)
                .fixedSize(horizontal: false, vertical: true)
            }
          }
          .frame(maxWidth: .infinity, alignment: .leading)
          .padding(.vertical, 14)

          Divider()
          endpointReviewField(
            "Endpoint fingerprint",
            value: candidate.endpointFingerprint,
            monospaced: true,
            copyable: true
          )
          Divider()
          endpointReviewField(
            "Review policy",
            value: "Version \(candidate.reviewPolicyVersion) · \(candidate.reviewPolicyDigest)",
            monospaced: true,
            copyable: true
          )
        }
        .padding(.horizontal, 24)
      }

      Divider()

      HStack(spacing: 12) {
        Spacer(minLength: 12)
        Button("Cancel") {
          dismiss()
        }
        .buttonStyle(.bordered)
        Button {
          onApprove()
          dismiss()
        } label: {
          Label("Approve Endpoint", systemImage: "checkmark.shield")
        }
        .buttonStyle(.borderedProminent)
        .accessibilityLabel("Approve exact endpoint for this Provider Account")
        .help("Validate the endpoint and record a versioned approval")
      }
      .padding(.horizontal, 24)
      .padding(.vertical, 16)
    }
    .frame(minWidth: 520, idealWidth: 600, minHeight: 560, idealHeight: 680)
    .background(LoomGraphite.canvas)
    .accessibilityElement(children: .contain)
    .accessibilityLabel(presentation.sheetAccessibilityLabel)
  }

  private func endpointReviewField(
    _ label: String,
    value: String,
    monospaced: Bool = false,
    copyable: Bool = false,
    multiline: Bool = false
  ) -> some View {
    VStack(alignment: .leading, spacing: 6) {
      Text(label)
        .font(.caption.weight(.semibold))
        .foregroundStyle(.secondary)
      HStack(spacing: 8) {
        Text(value)
          .font(monospaced ? .system(.body, design: .monospaced) : .body)
          .textSelection(.enabled)
          .lineLimit(multiline ? nil : (copyable ? 1 : nil))
          .truncationMode(.middle)
          .fixedSize(horizontal: false, vertical: multiline)
          .help(copyable ? value : "")
        if copyable {
          Button {
            NSPasteboard.general.clearContents()
            NSPasteboard.general.setString(value, forType: .string)
          } label: {
            Image(systemName: "doc.on.doc")
          }
          .buttonStyle(.plain)
          .accessibilityLabel("Copy \(label.lowercased())")
          .help("Copy \(label.lowercased())")
          .loomActionTarget()
        }
      }
    }
    .frame(maxWidth: .infinity, alignment: .leading)
    .padding(.vertical, 14)
  }
}

public struct MissionWorkbench: View {
  @Environment(\.dismiss) private var dismiss
  @ObservedObject private var store: LocalProductStore
  @State private var showProviders = false
  @State private var providerSearchText = ""
  @State private var providerCategory = "all"
  @State private var selectedProvider: LocalProductProviderDirectoryEntry?
  @State private var pendingCredentialImport:
    LocalProductCredentialImportCandidate?
  @State private var pendingEndpointReview:
    LocalProductCredentialImportCandidate?
  @State private var vaultDiagnosticPreview: LocalDiagnosticBundlePreview?
  @State private var vaultDiagnosticExporter: LocalDiagnosticBundleExporter?
  @State private var vaultDiagnosticError: String?
  @State private var preparingVaultDiagnostics = false
  @State private var confirmVaultRecoveryReset = false
  @State private var showVaultExport = false
  @State private var showNewMission = false
  @State private var newMissionTitle = ""
  @State private var showNewSideTask = false
  @State private var newMissionObjective = ""
  @State private var confirmedMissionConstraints: [String] = []
  @State private var acceptedMissionDecisions: [String] = []
  @State private var missionContextConfirmed = false
  @State private var newMissionTeamID = ""
  @State private var newMissionStartsNewAttempt = false
  @State private var linkNewMissionToConversation = true
  @State private var newMissionWorkPackageID =
    LocalProductWorkPackageOption.coding.id
  @State private var sideTaskPurpose = "research"
  @State private var sideTaskMode = "report_only"
  @State private var sideTaskTitle = ""
  @State private var sideTaskRequest = ""
  @State private var agentInputDrafts: [String: String] = [:]
  @State private var agentInputModes: [String: LocalProductAgentInputMode] = [:]
  @State private var expandedMissionActivityIDs = Set<String>()
  @State private var pendingAgentRecovery: LocalProductAgentRecoveryCandidate?
  @State private var pendingToolRecovery: PendingToolRecoveryDecision?
  @State private var showCreateAsset = false
  @State private var assetDefinitionID = "skill.local"
  @State private var assetRevisionID = "revision.1"
  @State private var assetName = "Local Skill"
  @State private var assetDescription = "Reviewed local evolution Candidate"
  @State private var assetSourcePath = ""
  @State private var assetCreationKind = "create_skill"
  @State private var assetSearchText = ""
  @State private var pendingAssetMutation: PendingEvolutionAssetMutation?

  private let showRail: Bool
  private let initialMissionObjective: String?
  private let initialConversationThreadID: String?
  private let initialConversationTitle: String?

  public init(
    store: LocalProductStore,
    showRail: Bool = true,
    showProvidersInitially: Bool = false,
    showNewMissionInitially: Bool = false,
    initialMissionTeamID: String? = nil,
    initialMissionObjective: String? = nil,
    initialConversationThreadID: String? = nil,
    initialConversationTitle: String? = nil
  ) {
    self.store = store
    self.showRail = showRail
    self.initialMissionObjective = initialMissionObjective
    self.initialConversationThreadID = initialConversationThreadID
    self.initialConversationTitle = initialConversationTitle
    _showProviders = State(initialValue: showProvidersInitially)
    _showNewMission = State(initialValue: showNewMissionInitially)
    _newMissionTeamID = State(initialValue: initialMissionTeamID ?? "")
    _newMissionStartsNewAttempt = State(
      initialValue: missionShouldStartNewAttempt(
        teamInstanceID: initialMissionTeamID ?? "",
        missions: store.snapshot?.missions ?? []
      )
    )
  }

  public var body: some View {
    HSplitView {
      if showRail {
        workbenchRail
          .frame(
            minWidth: LoomGraphite.railWidth,
            idealWidth: LoomGraphite.railWidth,
            maxWidth: LoomGraphite.railWidth
          )
      }
      switch store.workbench.route {
      case .board:
        orchestrationBoard
          .frame(minWidth: 560)
      case .mission(let id):
        missionRoom(id)
          .frame(minWidth: 560)
      case .teams:
        teamsWorkspace
          .frame(minWidth: 560)
      case .attention:
        attentionWorkspace
          .frame(minWidth: 560)
      case .library:
        libraryWorkspace
          .frame(minWidth: 560)
      }
    }
    .background(LoomGraphite.canvas)
    .sheet(isPresented: $showProviders) {
      providerManagement
        .frame(minWidth: 720, minHeight: 620)
    }
    .sheet(item: $vaultDiagnosticPreview) { preview in
      if let vaultDiagnosticExporter {
        DiagnosticBundlePreviewSheet(
          preview: preview,
          exporter: vaultDiagnosticExporter
        )
      }
    }
    .sheet(isPresented: $showVaultExport) {
      CredentialVaultExportSheet(store: store)
        .frame(minWidth: 480, minHeight: 340)
    }
    .confirmationDialog(
      "Reset Credential Vault?",
      isPresented: $confirmVaultRecoveryReset,
      titleVisibility: .visible
    ) {
      Button("Reset Vault", role: .destructive) {
        Task { await store.resetCredentialVault() }
      }
      Button("Cancel", role: .cancel) {
        confirmVaultRecoveryReset = false
      }
    } message: {
      Text(
        "This permanently removes the unusable encrypted Vault. Provider Accounts remain, but each API key must be re-entered."
      )
    }
    .confirmationDialog(
      pendingCredentialImport.map { "Import \($0.displayName)?" }
        ?? "Import Provider credential?",
      isPresented: Binding(
        get: { pendingCredentialImport != nil },
        set: { if !$0 { pendingCredentialImport = nil } }
      ),
      titleVisibility: .visible
    ) {
      if let candidate = pendingCredentialImport {
        Button("Import to Loom Vault") {
          pendingCredentialImport = nil
          Task { await store.importCredentialCandidate(candidate) }
        }
      }
      Button("Cancel", role: .cancel) { pendingCredentialImport = nil }
    } message: {
      if let candidate = pendingCredentialImport {
        Text(
          "Loom will read only this \(candidate.sourceApplication) credential once, encrypt it in the Loom Credential Vault, verify \(candidate.targetProviderID), and clear the plaintext lease."
        )
      }
    }
    .confirmationDialog(
      "Authorize Agent recovery?",
      isPresented: Binding(
        get: { pendingAgentRecovery != nil },
        set: { if !$0 { pendingAgentRecovery = nil } }
      ),
      titleVisibility: .visible
    ) {
      Button("Authorize recovery") {
        guard let candidate = pendingAgentRecovery else { return }
        pendingAgentRecovery = nil
        Task {
          await store.confirmAgentAttemptRecovery(candidate)
        }
      }
      Button("Cancel", role: .cancel) { pendingAgentRecovery = nil }
    } message: {
      if let candidate = pendingAgentRecovery {
        Text(
          "Authorize \(missionHumanStatus(candidate.harnessAdapter)) with \(candidate.providerAccountID), \(candidate.modelID), credential v\(candidate.credentialRevision). Provider work starts only after a separate Resume action."
        )
      }
    }
    .confirmationDialog(
      pendingToolRecovery.map { missionToolRecoveryActionLabel($0.action) + "?" }
        ?? "Resolve ToolCall recovery?",
      isPresented: Binding(
        get: { pendingToolRecovery != nil },
        set: { if !$0 { pendingToolRecovery = nil } }
      ),
      titleVisibility: .visible
    ) {
      if let pending = pendingToolRecovery {
        Button(
          missionToolRecoveryActionLabel(pending.action),
          role: pending.action == .abortAttempt ? .destructive : nil
        ) {
          pendingToolRecovery = nil
          Task {
            await store.resolveToolRecovery(
              pending.candidate, action: pending.action
            )
          }
        }
      }
      Button("Cancel", role: .cancel) { pendingToolRecovery = nil }
    } message: {
      if let pending = pendingToolRecovery {
        Text(missionToolRecoveryConfirmationDetail(pending.action))
      }
    }
    .sheet(isPresented: $showNewMission) {
      newMissionSheet
        .frame(minWidth: 660, minHeight: 640)
    }
    .sheet(isPresented: $showNewSideTask) {
      if case .mission(let missionID) = store.workbench.route {
        newSideTaskSheet(missionID)
          .frame(minWidth: 560, minHeight: 500)
      }
    }
    .sheet(isPresented: $showCreateAsset) {
      createEvolutionAssetSheet
        .frame(minWidth: 620, minHeight: 520)
    }
    .confirmationDialog(
      pendingAssetMutation.map { "Confirm \($0.action)?" } ?? "Confirm asset change?",
      isPresented: Binding(
        get: { pendingAssetMutation != nil },
        set: { if !$0 { pendingAssetMutation = nil } }
      ),
      titleVisibility: .visible
    ) {
      Button(
        pendingAssetMutation?.action ?? "Confirm",
        role: pendingAssetMutation?.action == "Reject" ? .destructive : nil
      ) {
        confirmEvolutionAssetMutation()
      }
      Button("Cancel", role: .cancel) { pendingAssetMutation = nil }
    } message: {
      Text("This explicit action writes authoritative Journal facts. Cancel writes nothing.")
    }
    .sheet(
      item: Binding(
        get: { store.activeDecisionSheet },
        set: { value in
          if value == nil {
            store.dismissDecisionSheet()
          }
        }
      )
    ) { sheet in
      LoomDecisionSheet(
        sheet: sheet,
        onAction: { action in
          Task { await store.submitDecisionAction(action) }
        },
        onOpenEvidence: {
          store.dismissDecisionSheet()
          Task {
            await store.openMissionAndActivate(sheet.missionID)
            store.selectMissionInspector(.evidence)
            store.setMissionInspectorVisible(true)
          }
        },
        onClose: { store.dismissDecisionSheet() }
      )
      .frame(minWidth: 620, idealWidth: 680, minHeight: 520)
    }
    .accessibilityElement(children: .contain)
    .accessibilityLabel(
      showRail
        ? "Loom Mission orchestration workbench"
        : "Loom governance panel")
  }

  private var workbenchRail: some View {
    VStack(alignment: .leading, spacing: 0) {
      HStack(spacing: 9) {
        Image(systemName: "square.3.layers.3d")
          .foregroundStyle(LoomGraphite.accent)
        Text("Loom")
          .font(.headline)
        Spacer()
      }
      .padding(.horizontal, 14)
      .frame(height: 52)

      railButton("New Mission", systemImage: "plus.square") {
        prepareNewMission()
        showNewMission = true
      }
      railButton("Missions", systemImage: "list.bullet.rectangle") {
        store.showMissionBoard()
      }
      railButton("Teams", systemImage: "person.3") {
        store.showMissionTeams()
      }
      railButton("Needs You", systemImage: "exclamationmark.bubble") {
        store.showMissionAttention()
      }

      Divider().padding(.vertical, 10)

      Text("WORKSPACE")
        .font(.caption2.weight(.semibold))
        .foregroundStyle(.secondary)
        .tracking(0.8)
        .padding(.horizontal, 14)
        .padding(.bottom, 4)
      railButton("Library", systemImage: "books.vertical") {
        store.showMissionLibrary()
      }
      railButton("Runtime & Providers", systemImage: "switch.2") {
        showProviders = true
      }

      Spacer()
    }
    .background(LoomGraphite.rail)
    .accessibilityElement(children: .contain)
  }

  private var executableTeams: [LocalProductTeamSummary] {
    store.executableTeams
  }

  private var selectedWorkPackage: LocalProductWorkPackageOption {
    LocalProductWorkPackageOption.accepted.first {
      $0.id == newMissionWorkPackageID
    } ?? .coding
  }

  private var newMissionSheet: some View {
    VStack(alignment: .leading, spacing: 0) {
      HStack {
        VStack(alignment: .leading, spacing: 4) {
          Text(missionContinuationSheetTitle(newAttempt: newMissionStartsNewAttempt))
            .font(.title2.weight(.semibold))
          Text(
            newMissionStartsNewAttempt
              ? "Review a fresh, separately audited Attempt in this Mission. The previous Attempt, its output and its Incident remain unchanged."
              : "A Mission is one bounded piece of work that an Agent Team carries out for you, with review before anything runs."
          )
            .foregroundStyle(.secondary)
            .fixedSize(horizontal: false, vertical: true)
        }
        Spacer()
        Button("Close") { showNewMission = false }
      }
      .padding(24)

      Divider()

      ScrollView {
        VStack(alignment: .leading, spacing: 18) {
          if executableTeams.isEmpty {
            VStack(alignment: .leading, spacing: 14) {
              HStack(alignment: .top, spacing: 12) {
                Image(systemName: "exclamationmark.arrow.triangle.2.circlepath")
                  .font(.system(size: 26, weight: .regular))
                  .foregroundStyle(LoomGraphite.accent)
                  .accessibilityHidden(true)
                VStack(alignment: .leading, spacing: 6) {
                  Text(
                    (store.snapshot?.teams.isEmpty ?? true)
                      ? "No executable Agent Team yet"
                      : "Agent Team needs Runtime & Provider recovery"
                  )
                    .font(.headline)
                  Text(runtimeRecoveryReason ?? "Runtime and Provider readiness is unavailable.")
                  .foregroundStyle(.secondary)
                  .fixedSize(horizontal: false, vertical: true)
                }
              }
              HStack(spacing: 10) {
                Button(action: openRuntimeProviderRecovery) {
                  Label("Open Runtime & Providers", systemImage: "switch.2")
                }
                .buttonStyle(.borderedProminent)
                .accessibilityLabel("Open Runtime and Providers recovery")
                .help("Open Runtime & Providers")

                if store.snapshot?.teams.isEmpty ?? true {
                  Button {
                    showNewMission = false
                    store.showMissionTeams()
                    Task { await store.startBlankBuilder() }
                  } label: {
                    Label("Create Agent Team", systemImage: "plus")
                  }
                  .buttonStyle(.bordered)
                  .accessibilityLabel("Create Agent Team")
                  .help("Create Agent Team")
                }

                Button("Close") { showNewMission = false }
                  .buttonStyle(.bordered)
              }
            }
            .padding(.vertical, 16)
          } else {
            Picker("Work type", selection: $newMissionWorkPackageID) {
              ForEach(LocalProductWorkPackageOption.accepted) { option in
                Text(option.title).tag(option.id)
              }
            }
            .pickerStyle(.segmented)

            Picker("Team", selection: $newMissionTeamID) {
              ForEach(executableTeams, id: \.teamInstanceID) { team in
                Text(
                  LocalProductExperience.visibleName(
                    team.displayName,
                    internalID: team.teamInstanceID,
                    fallback: "Confirmed Team"
                  )
                )
                .tag(team.teamInstanceID)
              }
            }

            VStack(alignment: .leading, spacing: 7) {
              Text("Mission title")
                .font(.subheadline.weight(.semibold))
              TextField("A short name for this workflow", text: $newMissionTitle)
                .textFieldStyle(.roundedBorder)

              Text("Mission objective")
                .font(.subheadline.weight(.semibold))
              TextField(
                "What should this Mission accomplish?",
                text: $newMissionObjective,
                axis: .vertical
              )
              .lineLimit(3...6)
              .textFieldStyle(.roundedBorder)
            }

            if let conversationThreadID = initialConversationThreadID,
              !conversationThreadID.isEmpty
            {
              Toggle(isOn: $linkNewMissionToConversation) {
                Label(
                  "Link to \(initialConversationTitle ?? "current conversation")",
                  systemImage: "bubble.left.and.bubble.right"
                )
              }
              .toggleStyle(.checkbox)
              .help("Adds a navigation link between this Mission and its source conversation")
            }

            Divider()

            missionContextEditor

            if let preflight = store.executionPreflight {
              Divider()
              missionPreflightReview(preflight)
            }
          }
        }
        .padding(24)
        .frame(maxWidth: .infinity, alignment: .leading)
      }
      .disabled(store.executionState == .starting)

      Divider()
      newMissionCommandBar
        .padding(.horizontal, 24)
        .padding(.vertical, 16)
    }
    .onAppear {
      if newMissionObjective.isEmpty, let prefill = initialMissionObjective {
        newMissionObjective = prefill
      }
      if newMissionTitle.isEmpty {
        newMissionTitle = suggestedMissionTitle(from: newMissionObjective)
      }
      // Pre-select the first executable Team so "Review preflight" is not
      // silently disabled for every entry path (rail and welcome "Start
      // Mission" both land here). The Team picker still lets the user change it.
      newMissionTeamID = resolvedMissionTeamID(
        preferred: newMissionTeamID,
        executableTeamIDs: executableTeams.map(\.teamInstanceID)
      )
      newMissionStartsNewAttempt = missionShouldStartNewAttempt(
        teamInstanceID: newMissionTeamID,
        missions: store.snapshot?.missions ?? []
      )
    }
    .onChange(of: newMissionObjective) { previous, current in
      if newMissionTitle.isEmpty ||
        newMissionTitle == suggestedMissionTitle(from: previous)
      {
        newMissionTitle = suggestedMissionTitle(from: current)
      }
      missionContextDidChange()
    }
    .onChange(of: newMissionTitle) { _, _ in missionContextDidChange() }
    .onChange(of: confirmedMissionConstraints) { _, _ in missionContextDidChange() }
    .onChange(of: acceptedMissionDecisions) { _, _ in missionContextDidChange() }
    .onChange(of: newMissionTeamID) { _, teamInstanceID in
      newMissionStartsNewAttempt = missionShouldStartNewAttempt(
        teamInstanceID: teamInstanceID,
        missions: store.snapshot?.missions ?? []
      )
      missionContextDidChange()
    }
    .onChange(of: newMissionWorkPackageID) { _, _ in missionContextDidChange() }
    .onChange(of: missionContextConfirmed) { _, confirmed in
      if !confirmed { store.invalidateMissionPreflight() }
    }
  }

  private var missionContextEditor: some View {
    VStack(alignment: .leading, spacing: 14) {
      HStack(alignment: .firstTextBaseline) {
        Label("Mission context", systemImage: "list.bullet.clipboard")
          .font(.headline)
        Spacer()
        Text("v1 · ordered authority")
          .font(.caption)
          .foregroundStyle(.secondary)
      }

      missionContextList(
        kind: .constraint,
        values: $confirmedMissionConstraints,
        emptyMessage: "No additional constraints.",
        addLabel: "Add constraint"
      )
      missionContextList(
        kind: .decision,
        values: $acceptedMissionDecisions,
        emptyMessage: "No prior decisions accepted.",
        addLabel: "Add decision"
      )

      if let validation = missionContextValidationMessage {
        Label(validation, systemImage: "exclamationmark.circle")
          .font(.caption)
          .foregroundStyle(.red)
      }

      Toggle("I confirm this Mission context", isOn: $missionContextConfirmed)
        .toggleStyle(.checkbox)
        .disabled(!missionContextCanConfirm || store.executionState == .starting)

      Text("Only the objective, confirmed constraints and accepted decisions above are frozen into preflight.")
        .font(.caption)
        .foregroundStyle(.secondary)
        .fixedSize(horizontal: false, vertical: true)
    }
  }

  private func missionContextList(
    kind: MissionContextItemKind,
    values: Binding<[String]>,
    emptyMessage: String,
    addLabel: String
  ) -> some View {
    VStack(alignment: .leading, spacing: 7) {
      Text(kind.title)
        .font(.subheadline.weight(.semibold))

      if values.wrappedValue.isEmpty {
        Text(emptyMessage)
          .font(.caption)
          .foregroundStyle(.secondary)
      } else {
        ForEach(Array(values.wrappedValue.indices), id: \.self) { index in
          HStack(spacing: 8) {
            Text("\(index + 1)")
              .font(.caption.monospacedDigit())
              .foregroundStyle(.secondary)
              .frame(width: 18, alignment: .trailing)
            TextField(
              kind.singularTitle,
              text: Binding(
                get: { values.wrappedValue[index] },
                set: { values.wrappedValue[index] = $0 }
              )
            )
            .textFieldStyle(.roundedBorder)
            Button {
              values.wrappedValue.remove(at: index)
            } label: {
              Image(systemName: "trash")
            }
            .buttonStyle(.borderless)
            .help("Remove \(kind.removeLabel)")
            .accessibilityLabel("Remove \(kind.removeLabel) \(index + 1)")
          }
        }
      }

      Button {
        guard values.wrappedValue.count < 64 else { return }
        values.wrappedValue.append("")
      } label: {
        Label(addLabel, systemImage: "plus")
      }
      .buttonStyle(.borderless)
      .disabled(values.wrappedValue.count >= 64 || store.executionState == .starting)
    }
  }

  private func missionPreflightReview(
    _ preflight: LocalProductExecutionPreflight
  ) -> some View {
    VStack(alignment: .leading, spacing: 10) {
          HStack {
            Label(
              "Preflight · \(preflight.nodes.count) Agent",
              systemImage: "checkmark.shield"
            )
            .font(.headline)
            Spacer()
            Text("Capacity \(preflight.capacityAvailable)")
              .font(.caption)
              .foregroundStyle(.secondary)
          }

          VStack(alignment: .leading, spacing: 0) {
            ForEach(Array(preflight.nodes.enumerated()), id: \.element.logicalNodeID) {
              index, node in
              if index > 0 { Divider() }
              missionPreflightAgentRow(node)
                .padding(.vertical, 9)
            }
          }

          Text("Permissions · \(preflight.permissionScopes.joined(separator: ", "))")
            .font(.caption)
            .foregroundStyle(.secondary)
            .lineLimit(2)
          Text("Approvals · \(preflight.approvalPoints.joined(separator: ", "))")
            .font(.caption)
            .foregroundStyle(.secondary)
            .lineLimit(2)
    }
    .frame(maxWidth: .infinity, alignment: .leading)
  }

  private var newMissionCommandBar: some View {
    HStack {
        Text(executionStateLabel)
          .font(.caption)
          .foregroundStyle(.secondary)
          .lineLimit(2)
        Spacer()
        if store.executionState == .ready {
          Button(newMissionStartsNewAttempt ? "Start new Attempt" : "Start Mission") {
            Task {
              let missionID = store.executionPreflight?.missionID
              await store.startPreflightedMission()
              if store.executionState == .running
                || store.executionState == .awaitingRecovery
                || store.executionState == .succeeded
              {
                if let missionID {
                  store.saveMissionPresentation(
                    missionID: missionID,
                    title: newMissionTitle,
                    conversationThreadID: linkNewMissionToConversation
                      ? initialConversationThreadID
                      : nil
                  )
                  if newMissionStartsNewAttempt,
                    store.workbench.selectedMissionID == missionID
                  {
                    store.updateMissionComposerDraft("")
                  }
                }
                showNewMission = false
              }
            }
          }
          .buttonStyle(.borderedProminent)
          .disabled(!missionContextConfirmed || store.executionState == .starting)
        } else {
          Button("Review preflight") {
            guard
              let team = executableTeams.first(where: {
                $0.teamInstanceID == newMissionTeamID
              })
            else { return }
            Task {
              await store.preflightMission(
                objective: newMissionObjective,
                team: team,
                workPackage: selectedWorkPackage,
                confirmedConstraints: confirmedMissionConstraints,
                acceptedDecisions: acceptedMissionDecisions,
                newAttempt: newMissionStartsNewAttempt
              )
            }
          }
          .buttonStyle(.borderedProminent)
          .disabled(missionReviewDisabled)
          if missionReviewDisabled {
            Text(missionReviewBlockedReason)
              .font(.caption2)
              .foregroundStyle(.secondary)
              .fixedSize(horizontal: false, vertical: true)
              .accessibilityLabel("Review preflight blocked. \(missionReviewBlockedReason)")
          }
          if runtimeRecoveryReason != nil {
            Button(action: openRuntimeProviderRecovery) {
              Label("Runtime & Providers", systemImage: "switch.2")
            }
            .buttonStyle(.bordered)
            .accessibilityLabel("Open Runtime and Providers recovery")
            .help("Open Runtime & Providers")
          }
          if missionFailureShowsMissionBoard {
            Button("Open Missions") {
              showNewMission = false
              store.showMissionBoard()
            }
            .buttonStyle(.bordered)
            .accessibilityLabel("Open Missions to inspect the running Mission")
          }
        }
    }
  }

  private var missionReviewDisabled: Bool {
    runtimeRecoveryReason != nil
      || newMissionObjective.trimmingCharacters(
        in: .whitespacesAndNewlines
      ).isEmpty
      || newMissionTitle.trimmingCharacters(
        in: .whitespacesAndNewlines
      ).isEmpty
      || newMissionTeamID.isEmpty
      || !missionContextConfirmed
      || missionContextValidationMessage != nil
      || store.executionState == .preflighting
      || store.executionState == .starting
  }

  private var runtimeRecoveryReason: String? {
    missionRuntimeRecoveryReason(
      hasExecutableTeam: !executableTeams.isEmpty,
      executionReachable: store.missionExecutionReachable,
      setupState: store.setupState
    )
  }

  private var missionReviewBlockedReason: String {
    if let runtimeRecoveryReason { return runtimeRecoveryReason }
    if newMissionObjective.trimmingCharacters(
      in: .whitespacesAndNewlines
    ).isEmpty {
      return "Enter a Mission objective first."
    }
    if newMissionTitle.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
      return "Enter a Mission title first."
    }
    if newMissionTeamID.isEmpty {
      return "Select a Team to run this Mission."
    }
    if let message = missionContextValidationMessage {
      return message
    }
    return "Confirm the Mission context to enable preflight review."
  }

  private func openRuntimeProviderRecovery() {
    showNewMission = false
    DispatchQueue.main.async {
      showProviders = true
    }
  }

  private func prepareNewMission(preferredTeamID: String? = nil) {
    newMissionTeamID = resolvedMissionTeamID(
      preferred: preferredTeamID ?? newMissionTeamID,
      executableTeamIDs: executableTeams.map(\.teamInstanceID)
    )
    newMissionStartsNewAttempt = missionShouldStartNewAttempt(
      teamInstanceID: newMissionTeamID,
      missions: store.snapshot?.missions ?? []
    )
  }

  private var missionContextCanConfirm: Bool {
    !newMissionTitle.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
      && !newMissionObjective.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
      && missionContextValidationMessage == nil
  }

  private var missionContextValidationMessage: String? {
    let title = newMissionTitle.trimmingCharacters(in: .whitespacesAndNewlines)
    if !newMissionTitle.isEmpty && title != newMissionTitle {
      return "Remove leading or trailing whitespace from the Mission title."
    }
    if title.utf8.count > 96 { return "The Mission title is too long." }
    let objective = newMissionObjective.trimmingCharacters(in: .whitespacesAndNewlines)
    if !newMissionObjective.isEmpty && objective != newMissionObjective {
      return "Remove leading or trailing whitespace from the Mission objective."
    }
    if objective.utf8.count > 4_096 { return "The Mission objective is too long." }

    let context = confirmedMissionConstraints + acceptedMissionDecisions
    if context.count > 128 { return "Keep Mission context to 64 constraints and 64 decisions." }
    for value in context {
      let trimmed = value.trimmingCharacters(in: .whitespacesAndNewlines)
      if trimmed.isEmpty { return "Remove empty context rows or enter their text." }
      if trimmed != value { return "Remove leading or trailing whitespace from Mission context." }
      if value.utf8.count > 4_096 { return "A Mission context item is too long." }
    }
    if Set(context).count != context.count {
      return "Each constraint and decision must be unique."
    }
    return nil
  }

  private func missionContextDidChange() {
    missionContextConfirmed = false
    store.invalidateMissionPreflight()
  }

  private func missionPreflightAgentRow(
    _ node: LocalProductExecutionNodePreview
  ) -> some View {
    HStack(alignment: .top, spacing: 10) {
      Image(
        systemName: node.status == "ready" ? "checkmark.circle.fill" : "exclamationmark.circle.fill"
      )
      .foregroundStyle(node.status == "ready" ? .green : .orange)
      .frame(width: 18, height: 18)

      VStack(alignment: .leading, spacing: 4) {
        HStack(spacing: 7) {
          Text(node.title)
            .font(.subheadline.weight(.semibold))
            .lineLimit(1)
          Text(node.role == "main" ? "Coordinator" : "Agent")
            .font(.caption2.weight(.medium))
            .foregroundStyle(.secondary)
        }

        Text(missionPreflightRoute(node))
          .font(.caption)
          .foregroundStyle(.primary)
          .lineLimit(1)

        Text(missionPreflightLimits(node))
          .font(.caption2)
          .foregroundStyle(.secondary)
          .lineLimit(2)

		if node.fallbackConfigured {
		  Text("Fallback · \(missionPreflightFallbackRoute(node))")
			.font(.caption2)
			.foregroundStyle(
			  node.fallbackStatus == "ready"
				? Color.secondary
				: LoomGraphite.statusWarning
			)
			.lineLimit(2)
		  Text(missionPreflightFallbackLimits(node))
			.font(.caption2)
			.foregroundStyle(.secondary)
			.lineLimit(2)
		  Text(
			node.fallbackApprovalAvailable
			  ? "Fallback approved v\(node.fallbackApprovalVersion)"
			  : "Fallback approval required"
		  )
			.font(.caption2)
			.foregroundStyle(.secondary)
		  if !node.fallbackBlockReason.isEmpty {
			Text(node.fallbackBlockReason)
			  .font(.caption2)
			  .foregroundStyle(.orange)
			  .lineLimit(2)
		  }
		}

        if !node.blockReason.isEmpty {
          Text(node.blockReason)
            .font(.caption)
            .foregroundStyle(.orange)
            .lineLimit(2)
        }
      }
      Spacer(minLength: 8)
    }
    .frame(maxWidth: .infinity, alignment: .leading)
  }

  private func missionPreflightRoute(
    _ node: LocalProductExecutionNodePreview
  ) -> String {
    let provider = node.providerAccountID.isEmpty
      ? node.providerID
      : "\(node.providerID) / \(node.providerAccountID)"
    return [node.harnessAdapter, provider, node.modelID]
      .filter { !$0.isEmpty }
      .joined(separator: " · ")
  }

  private func missionPreflightLimits(
    _ node: LocalProductExecutionNodePreview
  ) -> String {
    var values: [String] = []
    if !node.authMode.isEmpty {
      values.append(node.authMode == "native_auth" ? "Native auth" : "Brokered auth")
    }
    if node.credentialRevision > 0 {
      values.append("Credential r\(node.credentialRevision)")
    }
    if !node.reasoningEffort.isEmpty {
      values.append("Reasoning \(node.reasoningEffort)")
    }
    if node.timeoutSeconds > 0 {
      values.append("Timeout \(node.timeoutSeconds)s")
    }
    if let budget = node.budgetCredits {
      values.append("Budget \(budget)")
    }
    if !node.capabilities.isEmpty {
      values.append(node.capabilities.joined(separator: ", "))
    }
    return values.joined(separator: " · ")
  }

	private func missionPreflightFallbackRoute(
	  _ node: LocalProductExecutionNodePreview
	) -> String {
	  let provider = node.fallbackProviderAccountID.isEmpty
		? node.fallbackProviderID
		: "\(node.fallbackProviderID) / \(node.fallbackProviderAccountID)"
	  return [
		node.fallbackHarnessAdapter,
		provider,
		node.fallbackModelID,
	  ]
	  .filter { !$0.isEmpty }
	  .joined(separator: " · ")
	}

	private func missionPreflightFallbackLimits(
	  _ node: LocalProductExecutionNodePreview
	) -> String {
	  var values: [String] = []
	  if !node.fallbackAuthMode.isEmpty {
		values.append(
		  node.fallbackAuthMode == "native_auth" ? "Native auth" : "Brokered auth"
		)
	  }
	  if node.fallbackCredentialRevision > 0 {
		values.append("Credential r\(node.fallbackCredentialRevision)")
	  }
	  if !node.fallbackReasoningEffort.isEmpty {
		values.append("Reasoning \(node.fallbackReasoningEffort)")
	  }
	  if node.fallbackTimeoutSeconds > 0 {
		values.append("Timeout \(node.fallbackTimeoutSeconds)s")
	  }
	  if let budget = node.fallbackBudgetCredits {
		values.append("Budget \(budget)")
	  }
	  if !node.fallbackCapabilities.isEmpty {
		values.append(node.fallbackCapabilities.joined(separator: ", "))
	  }
	  return values.joined(separator: " · ")
	}

  private var executionStateLabel: String {
    switch store.executionState {
    case .idle: return "Nothing runs before you review preflight."
    case .preflighting: return "Checking Team, Runtime, capacity and access…"
    case .ready: return "Preflight is ready. Starting still requires your click."
    case .starting: return "Starting the authoritative execution…"
    case .cancelling: return "Cancelling the exact current Attempt…"
    case .running: return "Mission is running. Follow it in the Mission Room."
    case .awaitingRecovery: return "Mission is waiting for bounded recovery."
    case .succeeded: return "Mission completed with accepted Evidence."
    case .cancelled: return "Mission was cancelled with authoritative terminal Evidence."
    case .failed(let reason): return missionFailureLabel(reason)
    }
  }

  /// Maps daemon execution-rejection codes to user-facing, actionable copy so
  /// a failed start never shows a cryptic raw error. Unknown codes still show
  /// the stable reason so diagnostics remain searchable.
  private func missionFailureLabel(_ reason: String) -> String {
    if reason.hasPrefix("conflict.") {
      let stage = String(reason.dropFirst("conflict.".count))
      switch stage {
      case "preflight_lease", "view_drift", "preflight_digest":
        return "The reviewed preflight changed or expired. Review it again, then start."
      case "dispatch_capacity":
        return "The selected Runtime is at capacity. Retry when the current work finishes."
      case "dispatch_team_authority":
        return "The previous Attempt has not fully settled yet. Refresh Missions, then start a new audited Attempt."
      case "dispatch_recovery_required":
        return "This Team has an interrupted Attempt that needs recovery before a new run. Open Missions to recover or cancel it."
      case "dispatch_admission", "dispatch_attempt_validation",
        "dispatch_view_conflict", "dispatch_identity_unavailable",
        "dispatch_validation", "dispatch_context_validation",
        "dispatch_incomplete", "flight_conflict", "parent_continuation":
        return "Loom could not admit this Team run at \(stage.replacingOccurrences(of: "_", with: " ")). Open Missions to inspect the current Attempt, then retry."
      default:
        return "The Mission state changed at \(stage.replacingOccurrences(of: "_", with: " ")). Review preflight, then retry."
      }
    }
    switch reason {
    case "busy":
      return "This Team already has a running Mission. Open Missions to follow it, or wait for it to finish."
    case "conflict":
      return "The Mission state changed (for example this Team is already running). Review preflight, then retry."
    case "stale_view":
      return "Loom's view changed. Review preflight again, then retry."
    case "stale_generation":
      return "The Mission plan moved on. Review preflight again, then retry."
    case "state_unavailable":
      return "Execution is unavailable right now. Check Runtime & Providers, then retry."
    case "timeout":
      return "Starting the Mission timed out. Retry."
    case "preflight_expired":
      return "Preflight expired. Review preflight again, then retry."
    case "preflight_required":
      return "Review preflight before starting the Mission."
    case "projection_visibility_failed":
      return "The Mission started, but Loom could not yet show it. Open Missions in a moment."
    case "invalid_response":
      return "Loom received an invalid execution response. Retry, or open Missions to inspect state."
    default:
      return "Mission could not proceed: \(reason)"
    }
  }

  private var missionFailureShowsMissionBoard: Bool {
    guard case .failed(let reason) = store.executionState else { return false }
    return ["busy", "conflict", "projection_visibility_failed"].contains(reason)
  }

  private func railButton(
    _ title: String,
    systemImage: String,
    action: @escaping () -> Void
  ) -> some View {
    Button(action: action) {
      Label(title, systemImage: systemImage)
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, 14)
    }
    .buttonStyle(.plain)
    .loomActionTarget()
    .contentShape(Rectangle())
    .accessibilityLabel(title)
  }

  private var orchestrationBoard: some View {
    VStack(spacing: 0) {
      workbenchToolbar(
        title: "Missions",
        subtitle: "One Mission, one governed workflow"
      )
      HStack(spacing: 8) {
        Label("Mission history", systemImage: "list.bullet.rectangle")
          .foregroundStyle(LoomGraphite.accent)
        Spacer()
        Toggle(
          "Hide completed",
          isOn: Binding(
            get: { store.workbench.boardHideCompleted },
            set: { store.updateMissionBoardHideCompleted($0) }
          )
        )
        .toggleStyle(.checkbox)
        .font(.caption)
        .help("Hide completed Missions")
        .accessibilityLabel("Hide completed Missions")
        TextField("Filter Missions", text: missionFilter)
          .textFieldStyle(.roundedBorder)
          .frame(width: 190)
          .accessibilityLabel("Filter Missions")
      }
      .font(.callout.weight(.medium))
      .padding(.horizontal, 18)
      .frame(height: 46)

      if let authorityBanner {
        HStack(spacing: 8) {
          Image(systemName: "info.circle")
          Text(authorityBanner)
            .font(.caption)
          Spacer()
        }
        .foregroundStyle(connectionColor)
        .padding(.horizontal, 18)
        .frame(minHeight: 34)
        .background(connectionColor.opacity(0.08))
        .accessibilityLabel(authorityBanner)
      }

      if store.snapshot == nil {
        truthfulState(
          icon: "bolt.horizontal.circle",
          title: "Connecting to Loom",
          detail: "The last authoritative view remains visible if refresh fails."
        )
      } else if store.workbench.missions.isEmpty {
        truthfulState(
          icon: "list.bullet.rectangle",
          title: "No Missions yet",
          detail: emptyBoardDetail
        )
      } else if visibleMissionIndexItems.isEmpty {
        truthfulState(
          icon: "line.3.horizontal.decrease.circle",
          title: "No matching Missions",
          detail: "Clear the filter or show completed Missions."
        )
      } else {
        ScrollView {
          LazyVStack(spacing: 0) {
            ForEach(Array(visibleMissionIndexItems.enumerated()), id: \.element.id) {
              index, mission in
              if index > 0 { Divider().padding(.leading, 54) }
              missionIndexRow(mission)
            }
          }
          .padding(.horizontal, 18)
          .padding(.vertical, 8)
        }
        HStack {
          Text("\(visibleMissionIndexItems.count) Mission(s)")
            .monospacedDigit()
          Spacer()
          Text("Open one to inspect its workflow and Agent Team")
        }
        .font(.caption2)
        .foregroundStyle(.secondary)
        .padding(.horizontal, 18)
        .frame(height: 26)
      }
    }
    .background(LoomGraphite.canvas)
  }

  private var visibleMissionIndexItems: [MissionListItem] {
    let archivedTeams = Set(
      (store.snapshot?.teams ?? []).filter { !$0.executable }
        .map { $0.teamInstanceID }
    )
    return store.workbench.missions.filter { mission in
      let teamExecutable = !missionOnNonExecutableTeam(mission, archivedTeams)
      if !missionHistoryIncludesMission(
        lane: mission.lane,
        hideCompleted: store.workbench.boardHideCompleted,
        teamExecutable: teamExecutable
      ) {
        return false
      }
      let query = store.workbench.boardFilter
        .trimmingCharacters(in: .whitespacesAndNewlines)
      guard !query.isEmpty else { return true }
      let record = missionRecord(mission.id)
      return presentedMissionTitle(mission, record: record)
        .localizedCaseInsensitiveContains(query)
        || store.conversationLinkedToMission(mission.id)?.title
          .localizedCaseInsensitiveContains(query) == true
    }
  }

  private func missionIndexRow(_ mission: MissionListItem) -> some View {
    let record = missionRecord(mission.id)
    let linkedConversation = store.conversationLinkedToMission(mission.id)
    return Button {
      Task { await store.openMissionAndActivate(mission.id) }
    } label: {
      HStack(alignment: .top, spacing: 12) {
        Circle()
          .fill(missionIndexStatusColor(mission.status))
          .frame(width: 9, height: 9)
          .padding(.top, 7)
          .accessibilityHidden(true)
        VStack(alignment: .leading, spacing: 5) {
          Text(presentedMissionTitle(mission, record: record))
            .font(.body.weight(.semibold))
            .lineLimit(2)
          HStack(spacing: 12) {
            Label(
              linkedConversation?.title ?? "No conversation link",
              systemImage: linkedConversation == nil
                ? "bubble.left" : "bubble.left.fill"
            )
            if let record {
              Label(
                missionTeamDisplayName(record),
                systemImage: "person.3"
              )
            }
          }
          .font(.caption)
          .foregroundStyle(.secondary)
          .lineLimit(1)
          if let record {
            Text(
              "\(record.completedNodeCount)/\(record.nodeCount) steps · \(record.lastMilestone)"
            )
            .font(.caption2)
            .foregroundStyle(.secondary)
            .lineLimit(1)
          }
        }
        Spacer(minLength: 12)
        VStack(alignment: .trailing, spacing: 7) {
          Text(missionBoardDetailText(
            lane: mission.lane.rawValue,
            status: mission.status
          ))
          .font(.caption.weight(.medium))
          .foregroundStyle(missionIndexStatusColor(mission.status))
          Image(systemName: "chevron.right")
            .font(.caption.weight(.semibold))
            .foregroundStyle(.tertiary)
        }
      }
      .padding(.vertical, 13)
      .contentShape(Rectangle())
    }
    .buttonStyle(.plain)
    .accessibilityLabel(
      "\(presentedMissionTitle(mission, record: record)), \(humanStatus(mission.status))"
    )
  }

  private func presentedMissionTitle(
    _ mission: MissionListItem,
    record: LocalProductMissionSummary?
  ) -> String {
    if let title = store.missionPresentations[mission.id]?.title, !title.isEmpty {
      return title
    }
    return missionDisplayTitle(
      candidate: record?.title ?? mission.title,
      missionID: record?.missionID ?? mission.id,
      teamInstanceID: record?.teamInstanceID ?? "",
      teams: store.snapshot?.teams ?? []
    )
  }

  private func missionTeamDisplayName(
    _ record: LocalProductMissionSummary
  ) -> String {
    guard let team = store.snapshot?.teams.first(where: {
      $0.teamInstanceID == record.teamInstanceID
    }) else { return "Agent Team" }
    return LocalProductExperience.visibleName(
      team.displayName,
      internalID: team.teamInstanceID,
      fallback: "Agent Team"
    )
  }

  private func missionIndexStatusColor(_ status: String) -> Color {
    switch status {
    case "succeeded": return .green
    case "running", "orchestrating", "ready_for_review": return LoomGraphite.accent
    case "blocked", "human_required": return .orange
    case "failed", "cancelled": return .red
    default: return .secondary
    }
  }

  /// Mission id is "mission/<teamID>". Team readiness is supplied separately
  /// so Mission history never inherits the Team's current execution state.
  private func missionOnNonExecutableTeam(
    _ item: MissionListItem,
    _ nonExecutableTeams: Set<String>
  ) -> Bool {
    guard item.id.hasPrefix("mission/") else { return false }
    let teamID = String(item.id.dropFirst("mission/".count))
    return nonExecutableTeams.contains(teamID)
  }

  private func missionLane(_ lane: MissionLane) -> some View {
    let archivedTeams = Set(
      (store.snapshot?.teams ?? []).filter { !$0.executable }
        .map { $0.teamInstanceID }
    )
    let missions = store.workbench.missions.filter {
      $0.lane == lane
        && missionHistoryIncludesMission(
          lane: $0.lane,
          hideCompleted: false,
          teamExecutable: !missionOnNonExecutableTeam($0, archivedTeams)
        )
        && (store.workbench.boardFilter.isEmpty
          || $0.title.localizedCaseInsensitiveContains(
            store.workbench.boardFilter
          ))
    }
    return VStack(alignment: .leading, spacing: 10) {
      HStack {
        Text(lane.rawValue)
          .font(.subheadline.weight(.semibold))
        Spacer()
        Text("\(missions.count)")
          .font(.caption.monospacedDigit())
          .foregroundStyle(.secondary)
      }
      .padding(.horizontal, 3)
      ForEach(missions) { mission in
        missionCard(mission)
      }
      Spacer(minLength: 0)
    }
    .padding(10)
    .frame(width: 168, alignment: .top)
    .frame(minHeight: 420, alignment: .top)
    .background(
      LoomGraphite.surface.opacity(0.62),
      in: RoundedRectangle(
        cornerRadius: LoomGraphite.cardRadius,
        style: .continuous
      )
    )
    .accessibilityElement(children: .contain)
    .accessibilityLabel("\(lane.rawValue) lane")
  }

  private func missionCard(_ mission: MissionListItem) -> some View {
    let record = missionRecord(mission.id)
    let displayTitle = missionDisplayTitle(
      candidate: record?.title ?? mission.title,
      missionID: record?.missionID ?? mission.id,
      teamInstanceID: record?.teamInstanceID ?? "",
      teams: store.snapshot?.teams ?? []
    )
    return Button {
      Task { await store.openMissionAndActivate(mission.id) }
    } label: {
      VStack(alignment: .leading, spacing: 7) {
        if let record {
          Text(
            "\(humanStatus(record.sourceKind)) · \(record.priority.capitalized) priority"
          )
          .font(.caption2)
          .foregroundStyle(.secondary)
        }
        HStack {
          Text(displayTitle)
            .font(.callout.weight(.semibold))
            .lineLimit(2)
          Spacer(minLength: 4)
          if mission.status == "human_required" || mission.status == "blocked" {
            Image(systemName: "exclamationmark.circle.fill")
              .foregroundStyle(.orange)
              .accessibilityLabel("Needs attention")
          }
        }
        Text(humanStatus(mission.status))
          .font(.caption)
          .foregroundStyle(.secondary)
        if let blockReason = record?.blockReason,
           !blockReason.isEmpty,
           ["blocked", "failed"].contains(mission.status) {
          Label(
            missionHumanStatus(blockReason),
            systemImage: "exclamationmark.triangle"
          )
          .font(.caption2)
          .foregroundStyle(.orange)
          .lineLimit(2)
          .fixedSize(horizontal: false, vertical: true)
          .accessibilityLabel(
            "Mission failure reason: \(missionHumanStatus(blockReason))"
          )
        }
        if let record {
          Group {
            if let pulse = record.teamPulse.first {
              Label(
                "Team · \(pulse.role.capitalized) · \(humanStatus(pulse.state))",
                systemImage: "person.2"
              )
              .lineLimit(2)
            } else {
              Label(
                "No Team presence",
                systemImage: "person.2.slash"
              )
            }
          }
          .font(.caption2)
          .foregroundStyle(.secondary)
          .fixedSize(horizontal: false, vertical: true)
          Label(
            "\(record.teamPulse.first.map { "Attempt \($0.attemptNumber)" } ?? "No active attempt") · \(record.completedNodeCount)/\(record.nodeCount) nodes · \(missionNodeDisplayTitle(nodeID: record.currentNodeID, topology: record.topology, pulse: record.teamPulse))",
            systemImage: "point.3.connected.trianglepath.dotted"
          )
          .font(.caption2)
          .foregroundStyle(.secondary)
          .lineLimit(2)
          .fixedSize(horizontal: false, vertical: true)
          Text(record.lastMilestone)
            .font(.caption2)
            .foregroundStyle(.secondary)
            .lineLimit(2)
        }
      }
      .padding(12)
      .frame(maxWidth: .infinity, alignment: .leading)
      .background(
        LoomGraphite.raised,
        in: RoundedRectangle(
          cornerRadius: LoomGraphite.cardRadius,
          style: .continuous
        )
      )
      .overlay {
        RoundedRectangle(
          cornerRadius: LoomGraphite.cardRadius,
          style: .continuous
        )
        .stroke(LoomGraphite.separator.opacity(0.7), lineWidth: 1)
      }
    }
    .buttonStyle(.plain)
    .accessibilityLabel(
      "\(displayTitle), \(missionBoardDetailText(lane: mission.lane.rawValue, status: mission.status))"
    )
  }

  private func missionRoom(_ id: String) -> some View {
    let status = missionRecord(id)?.status ?? "loading"
    return HSplitView {
      missionCenter(id)
        .frame(minWidth: 560)
      if store.workbench.selectedContinuity.inspectorVisible {
        missionInspector(id)
          .frame(
            minWidth: 240,
            idealWidth: LoomGraphite.inspectorWidth,
            maxWidth: 340
          )
      }
    }
    .task(id: "\(id):\(status)") {
      guard missionActivityNeedsRefresh(status: status) else { return }
      await store.followVisibleMissionActivity(id)
    }
  }

  private func missionCenter(_ id: String) -> some View {
    let record = missionRecord(id)
    let mission = store.workbench.missions.first(where: { $0.id == id })
    let linkedConversation = store.conversationLinkedToMission(id)
    return VStack(spacing: 0) {
      workbenchToolbar(
        title: mission.map { presentedMissionTitle($0, record: record) }
          ?? "Mission",
        subtitle: record.map {
          missionBoardDetailText(
            lane: $0.lane,
            status: $0.status
          )
        } ?? "Unavailable",
        backAction: { store.showMissionBoard() }
      )
      if let linkedConversation {
        HStack(spacing: 10) {
          Label(
            "From conversation · \(linkedConversation.title)",
            systemImage: "bubble.left.fill"
          )
          .font(.caption)
          .foregroundStyle(.secondary)
          .lineLimit(1)
          Spacer()
          Button("Open conversation") {
            store.selectChatSession(linkedConversation.threadID)
            dismiss()
          }
          .buttonStyle(.borderless)
        }
        .padding(.horizontal, 18)
        .frame(minHeight: 38)
        .background(LoomGraphite.surface)
      }
      if record?.status == "succeeded",
        let workspacePath = store.missionPresentations[id]?.workspacePath,
        let result = missionResultPresentation(workspacePath: workspacePath)
      {
        missionResultBar(result)
      }
      ScrollView {
        LazyVStack(alignment: .leading, spacing: 14) {
          missionOutcomeBlock(record)
          missionActivity(record)
          contentBlock(
            title: "Plan",
            icon: "point.3.connected.trianglepath.dotted",
            text: record.map {
              missionPlanSummary(
                nodeCount: $0.nodeCount,
                completedNodeCount: $0.completedNodeCount,
                reviewNodeCount: $0.reviewNodeCount
              )
            } ?? "No plan is available."
          )
	          agentInputControls(record)
	          if store.missionCancelAvailable {
            Button("Cancel Mission", role: .destructive) {
              Task { await store.cancelCurrentMission() }
            }
            .buttonStyle(.bordered)
            .help("Cancels only the exact current authorized Attempt.")
          }
          contentBlock(
            title: "Evidence",
            icon: "checkmark.seal",
            text: store.snapshot?.evidence.isEmpty == false
              ? "Accepted Evidence references are available in the Inspector."
              : "No accepted Evidence is available yet."
          )
        }
        .padding(18)
      }
      if let record, missionContinuationAcceptsInput(status: record.status) {
        missionContinuationBar(id: id, record: record)
      }
      Divider()
      VStack(alignment: .leading, spacing: 8) {
        HStack(spacing: 6) {
          Text("Permission")
            .font(.caption.weight(.semibold))
            .foregroundStyle(.secondary)
          ForEach(MissionPermissionMode.allCases) { mode in
            Button(mode.rawValue) {
              store.selectMissionPermissionMode(mode)
            }
            .buttonStyle(.plain)
            .font(.caption.weight(.medium))
            .padding(.horizontal, 9)
            .frame(minHeight: 28)
            .background(
              store.workbench.selectedContinuity.permissionMode == mode
                ? LoomGraphite.accent.opacity(0.15)
                : LoomGraphite.surface,
              in: RoundedRectangle(cornerRadius: 7)
            )
            .overlay {
              RoundedRectangle(cornerRadius: 7)
                .stroke(
                  store.workbench.selectedContinuity
                    .permissionMode == mode
                    ? LoomGraphite.accent
                    : LoomGraphite.separator,
                  lineWidth: 1
                )
            }
          }
          Spacer()
        }
        Text("A mode is only a proposal until Loom confirms authority.")
          .font(.caption2)
          .foregroundStyle(.secondary)
      }
      .padding(12)
    }
    .background(LoomGraphite.canvas)
  }

  private func missionContinuationBar(
    id: String,
    record: LocalProductMissionSummary
  ) -> some View {
    let mission = store.workbench.missions.first(where: { $0.id == id })
    let title = mission.map { presentedMissionTitle($0, record: record) }
      ?? missionDisplayTitle(
        candidate: record.title,
        missionID: record.missionID,
        teamInstanceID: record.teamInstanceID,
        teams: store.snapshot?.teams ?? []
      )
    let draft = store.workbench.selectedContinuity.composerDraft
    let preparation = missionContinuationPreparation(
      status: record.status,
      title: title,
      teamInstanceID: record.teamInstanceID,
      draft: draft
    )
    let teamAvailable = executableTeams.contains {
      $0.teamInstanceID == record.teamInstanceID
    }

    return VStack(alignment: .leading, spacing: 8) {
      HStack(spacing: 8) {
        Image(systemName: "arrow.triangle.branch")
          .foregroundStyle(LoomGraphite.statusWarning)
          .accessibilityHidden(true)
        Text("Continue this Mission")
          .font(.callout.weight(.semibold))
        Spacer()
        Text("New audited Attempt")
          .font(.caption2.weight(.medium))
          .foregroundStyle(.secondary)
      }

      HStack(alignment: .bottom, spacing: 8) {
        TextField(
          "Tell the Team what to change or try next",
          text: Binding(
            get: { store.workbench.selectedContinuity.composerDraft },
            set: store.updateMissionComposerDraft
          ),
          axis: .vertical
        )
        .lineLimit(1...4)
        .textFieldStyle(.plain)

        Button {
          guard let preparation else { return }
          prepareMissionContinuation(
            preparation,
            linkedToConversation: store.conversationLinkedToMission(id) != nil
          )
        } label: {
          Label("Review & continue", systemImage: "arrow.right.circle.fill")
        }
        .buttonStyle(.borderedProminent)
        .disabled(preparation == nil || !teamAvailable)
        .accessibilityLabel("Review a new audited Attempt with this guidance")
        .help("Review the exact Team binding and context before starting")
      }
      .padding(.horizontal, 10)
      .padding(.vertical, 8)
      .background(LoomGraphite.canvas, in: RoundedRectangle(cornerRadius: 7))
      .overlay {
        RoundedRectangle(cornerRadius: 7)
          .stroke(LoomGraphite.separator, lineWidth: 1)
      }

      HStack(spacing: 10) {
        Text(
          teamAvailable
            ? "Your guidance becomes the objective of a new Attempt. Previous output and diagnostics stay available."
            : "This Team is not currently executable. Restore its Runtime or Provider before continuing."
        )
        .font(.caption2)
        .foregroundStyle(.secondary)
        .fixedSize(horizontal: false, vertical: true)
        Spacer(minLength: 8)
        if !teamAvailable {
          Button("Runtime & Providers", systemImage: "switch.2") {
            showProviders = true
          }
          .buttonStyle(.borderless)
          .font(.caption)
        }
      }
    }
    .padding(.horizontal, 18)
    .padding(.vertical, 12)
    .background(LoomGraphite.surface)
    .accessibilityElement(children: .contain)
    .accessibilityLabel("Blocked Mission intervention")
  }

  private func prepareMissionContinuation(
    _ preparation: MissionContinuationPreparation,
    linkedToConversation: Bool
  ) {
    store.invalidateMissionPreflight()
    newMissionTeamID = preparation.teamInstanceID
    newMissionStartsNewAttempt = preparation.newAttempt
    newMissionTitle = preparation.title
    newMissionObjective = preparation.objective
    confirmedMissionConstraints = []
    acceptedMissionDecisions = []
    missionContextConfirmed = false
    linkNewMissionToConversation = linkedToConversation
    showNewMission = true
  }

  private func missionResultBar(
    _ result: MissionResultPresentation
  ) -> some View {
    HStack(spacing: 10) {
      Label(
        result.isWebResult ? "Web result ready" : "Result ready",
        systemImage: "checkmark.circle.fill"
      )
      .font(.callout.weight(.medium))
      .foregroundStyle(LoomGraphite.statusSuccess)
      Spacer()
      Button {
        NSWorkspace.shared.open(result.openURL)
      } label: {
        Label(
          "Open result",
          systemImage: result.isWebResult ? "safari" : "folder"
        )
      }
      .buttonStyle(.borderedProminent)
      .accessibilityLabel("Open Mission result")
      .help(result.isWebResult ? "Open index.html in your browser" : "Open output folder")

      Button {
        NSWorkspace.shared.activateFileViewerSelecting([result.revealURL])
      } label: {
        Image(systemName: "folder.badge.gearshape")
      }
      .buttonStyle(.bordered)
      .accessibilityLabel("Show Mission result in Finder")
      .help("Show in Finder")
    }
    .padding(.horizontal, 18)
    .frame(minHeight: 48)
    .background(LoomGraphite.surface)
  }

  private func missionInspector(_ id: String) -> some View {
    let record = missionRecord(id)
    let section = missionInspectorSection(
      tab: store.workbench.selectedContinuity.inspector,
      record: record,
      timeline: store.timeline
    )
    return VStack(alignment: .leading, spacing: 12) {
      HStack {
        Text("Inspector")
          .font(.headline)
        Spacer()
        Button {
          store.setMissionInspectorVisible(false)
        } label: {
          Image(systemName: "sidebar.right")
        }
        .buttonStyle(.plain)
        .loomActionTarget()
        .accessibilityLabel("Hide Inspector")
      }
      Picker(
        "Inspector",
        selection: Binding(
          get: { store.workbench.selectedContinuity.inspector },
          set: { store.selectMissionInspector($0) }
        )
      ) {
        ForEach(MissionInspectorTab.allCases) {
          Text($0.rawValue).tag($0)
        }
      }
      .pickerStyle(.segmented)
      .labelsHidden()
      Divider()
      Text(section.heading)
        .font(.subheadline.weight(.semibold))
      if let notice = section.notice {
        Label(notice, systemImage: "exclamationmark.triangle")
          .font(.caption)
          .foregroundStyle(LoomGraphite.statusWarning)
          .fixedSize(horizontal: false, vertical: true)
      }
      if section.rows.isEmpty && section.providerAccountRows.isEmpty {
        Text(section.emptyMessage)
          .font(.caption)
          .foregroundStyle(.secondary)
      } else {
        ForEach(Array(section.rows.enumerated()), id: \.offset) { index, row in
          HStack {
            Image(
              systemName: section.systemImage(
                at: index,
                fallback: inspectorSectionIcon
              )
            )
              .foregroundStyle(LoomGraphite.accent)
              .accessibilityLabel(
                section.accessibilityLabel(at: index) ?? section.heading
              )
            Text(row)
              .font(.caption)
              .fixedSize(horizontal: false, vertical: true)
            Spacer()
            if section.canReviewRetry(at: index) {
              Button {
                store.showMissionAttention()
              } label: {
                Image(systemName: "arrow.clockwise")
              }
              .buttonStyle(.plain)
              .loomActionTarget()
              .accessibilityLabel("Review retry")
              .help("Review retry in Attention")
            }
            if section.canRecoverCredentialVault(at: index) {
              Button {
                showProviders = true
              } label: {
                Image(systemName: "key")
              }
              .buttonStyle(.plain)
              .loomActionTarget()
              .accessibilityLabel("Open Credential Vault recovery")
              .help("Open Credential Vault recovery")
            }
            if section.canRecoverAgentAttempt(at: index) {
              Button {
                Task { await store.previewAgentAttemptRecoveries() }
              } label: {
                Image(systemName: "arrow.clockwise")
              }
              .buttonStyle(.plain)
              .loomActionTarget()
              .disabled(store.isPreviewingAgentRecoveries)
              .accessibilityLabel("Review Agent recovery")
              .help("Review the frozen Agent recovery candidate")
            }
            if section.canViewDiagnostics(at: index) {
              Button {
                prepareVaultDiagnosticPreview()
              } label: {
                Image(systemName: "stethoscope")
              }
              .buttonStyle(.plain)
              .loomActionTarget()
              .disabled(preparingVaultDiagnostics)
              .accessibilityLabel("View Agent diagnostics")
              .help("View privacy-safe Agent diagnostics")
            }
            if let incidentID = section.incidentID(at: index) {
              Button {
                NSPasteboard.general.clearContents()
                NSPasteboard.general.setString(incidentID, forType: .string)
              } label: {
                Image(systemName: "doc.on.doc")
              }
              .buttonStyle(.plain)
              .loomActionTarget()
              .accessibilityLabel("Copy incident ID")
              .help("Copy incident ID")
            }
          }
        }
        ForEach(section.providerAccountRows) { account in
          providerAccountGovernanceSurface(account)
        }
      }
      if store.workbench.selectedContinuity.inspector == .team {
        agentRecoveryInspector(record: record)
        toolRecoveryInspector()
      }
      if let vaultDiagnosticError {
        Text(vaultDiagnosticError)
          .font(.caption)
          .foregroundStyle(LoomGraphite.statusDanger)
      }
      Divider()
      Text("Side-tasks")
        .font(.subheadline.weight(.semibold))
      Button("New side task", systemImage: "plus.bubble") {
        store.discardSideTaskProposal()
        showNewSideTask = true
      }
      .buttonStyle(.borderedProminent)
      .disabled(!store.canCreateSideTask(for: id))
      .help(
        store.canCreateSideTask(for: id)
          ? "Propose a bounded child task, then confirm it explicitly."
          : "Open the current authoritative Mission timeline before creating a side task."
      )
      let sideTasks = (store.snapshot?.sideTasks ?? []).filter {
        $0.parentMissionID == id
      }
      if sideTasks.isEmpty {
        Text("No Side-task handoff is recorded.")
          .font(.caption)
          .foregroundStyle(.secondary)
      } else {
        ForEach(sideTasks, id: \.sideTaskID) { sideTask in
          let presentation = sideTaskDrawerPresentation(sideTask)
          VStack(alignment: .leading, spacing: 3) {
            Text(sideTask.title)
              .font(.caption.weight(.semibold))
            Text(presentation.purpose)
              .font(.caption2)
              .foregroundStyle(.secondary)
            Text("\(missionHumanStatus(sideTask.mode)) · \(missionHumanStatus(sideTask.status))")
              .font(.caption2)
              .foregroundStyle(.secondary)
            if !sideTask.whatHappened.isEmpty {
              Text(sideTask.whatHappened)
                .font(.caption)
                .lineLimit(3)
            }
            ForEach(Array(sideTask.authorizedFindings.prefix(3)), id: \.self) { finding in
              Text("Finding · \(finding)")
                .font(.caption2)
                .lineLimit(2)
            }
            Text(
              "Risk · \(missionHumanStatus(sideTask.risk)) · Evidence \(sideTask.evidenceReferences.count) · Artifacts \(sideTask.artifactReferences.count)"
            )
            .font(.caption2)
            .foregroundStyle(.secondary)
            if let evidence = sideTask.evidenceReferences.first {
              Text("Evidence ref · \(missionHumanStatus(evidence.kind)) · \(evidence.evidenceID)")
                .font(.caption2)
                .foregroundStyle(.secondary)
                .lineLimit(1)
            }
            if let artifact = sideTask.artifactReferences.first {
              Text(
                "Artifact ref · \(missionHumanStatus(artifact.kind)) · \(String(artifact.digest.prefix(12)))…"
              )
              .font(.caption2)
              .foregroundStyle(.secondary)
            }
            Text(
              sideTask.usageObserved
                ? "Usage · \(sideTask.usageMicrounits) microunits \(sideTask.usageCurrency)"
                : "Usage · Not observed"
            )
            .font(.caption2)
            .foregroundStyle(.secondary)
            Text(presentation.uncertainty)
              .font(.caption2)
              .foregroundStyle(.secondary)
            Text(presentation.scopeDelta)
              .font(.caption2)
              .foregroundStyle(.secondary)
            Text(presentation.nextAction)
              .font(.caption2)
              .foregroundStyle(.secondary)
            if !sideTask.availableDecisions.isEmpty {
              Menu("Choose next action") {
                ForEach(sideTask.availableDecisions, id: \.self) { decision in
                  Button(missionHumanStatus(decision)) {
                    Task { await store.decideSideTask(sideTask, decision: decision) }
                  }
                }
              }
              .disabled(!store.canDecideSideTask(sideTask))
            }
          }
          .frame(maxWidth: .infinity, alignment: .leading)
          .padding(8)
          .background(LoomGraphite.surface, in: RoundedRectangle(cornerRadius: 8))
        }
      }
      Divider()
      Text("Decision availability")
        .font(.subheadline.weight(.semibold))
      if let command = store.preparedDecisionCommand(for: id) {
        Button(decisionCTA(command.kind)) {
          Task { await store.openPreparedDecision(command) }
        }
        .buttonStyle(.borderedProminent)
        .loomActionTarget()
        .accessibilityLabel(decisionCTA(command.kind))
        Text(
          "Bound to \(record.map { missionNodeDisplayTitle(nodeID: command.logicalNodeID, topology: $0.topology, pulse: $0.teamPulse) } ?? "Mission step") · Attempt \(command.attemptNumber)"
        )
        .font(.caption)
        .foregroundStyle(.secondary)
      } else {
        if let record, record.lane == "Review" {
          Button("Open Review Gate") {
            store.openReadOnlyReviewDecision(for: record)
          }
          .buttonStyle(.bordered)
          .loomActionTarget()
          .accessibilityLabel("Open read-only Review Gate")
        }
        Text(
          record?.attentionCount ?? 0 > 0
            ? "Needs You · visible read-only until an authoritative prepared command exists"
            : "No pending prepared decision"
        )
        .font(.caption)
        .foregroundStyle(.secondary)
      }
      Spacer()
    }
    .padding(14)
    .background(LoomGraphite.surface)
  }

  private func providerAccountGovernanceSurface(
    _ account: MissionProviderAccountGovernanceRow
  ) -> some View {
    VStack(alignment: .leading, spacing: 7) {
      HStack(alignment: .firstTextBaseline, spacing: 8) {
        Image(systemName: "server.rack")
          .foregroundStyle(LoomGraphite.accent)
          .accessibilityHidden(true)
        VStack(alignment: .leading, spacing: 1) {
          Text(account.providerName)
            .font(.caption.weight(.semibold))
          Text(account.providerAccountID)
            .font(.caption2.monospaced())
            .foregroundStyle(.secondary)
        }
        Spacer(minLength: 4)
        Label(
          account.completeness,
          systemImage: account.isIncomplete
            ? "exclamationmark.triangle.fill" : "checkmark.circle.fill"
        )
        .font(.caption2.weight(.medium))
        .foregroundStyle(
          account.isIncomplete ? LoomGraphite.statusWarning : LoomGraphite.statusSuccess
        )
      }
      providerAccountMetric("Attempts", account.attempts)
      providerAccountMetric("Errors", account.reliability)
      providerAccountMetric("Accounting", account.accountingCoverage)
      providerAccountMetric("Tokens", account.tokens)
      providerAccountMetric("Cost source", account.costSource)
      providerAccountMetric("Policy", account.policyRevision)
      providerAccountMetric("Concurrency", account.concurrencyCeiling)
      providerAccountMetric("Dispatch", account.dispatchCeiling)
      providerAccountMetric("Budget", account.budgetCeiling)
    }
    .padding(.vertical, 8)
    .overlay(alignment: .bottom) { Divider() }
    .accessibilityElement(children: .contain)
    .accessibilityLabel(account.accessibilityLabel)
  }

  private func providerAccountMetric(_ label: String, _ value: String) -> some View {
    HStack(alignment: .firstTextBaseline, spacing: 8) {
      Text(label)
        .font(.caption2)
        .foregroundStyle(.secondary)
        .frame(width: 66, alignment: .leading)
      Text(value)
        .font(.caption)
        .monospacedDigit()
        .fixedSize(horizontal: false, vertical: true)
      Spacer(minLength: 0)
    }
  }

  private func newSideTaskSheet(_ missionID: String) -> some View {
    VStack(alignment: .leading, spacing: 16) {
      HStack {
        VStack(alignment: .leading, spacing: 4) {
          Text("New side task").font(.title2.weight(.semibold))
          Text("Review the zero-write proposal before Loom starts an independent child run.")
            .font(.callout).foregroundStyle(.secondary)
        }
        Spacer()
        Button("Close") { showNewSideTask = false }
      }
      Picker("Purpose", selection: $sideTaskPurpose) {
        ForEach(
          ["research", "comparison", "diagnosis", "verification", "read_only_review"], id: \.self
        ) {
          Text(missionHumanStatus($0)).tag($0)
        }
      }
      Picker("Mode", selection: $sideTaskMode) {
        ForEach(["report_only", "decision_required", "merge_candidate"], id: \.self) {
          Text(missionHumanStatus($0)).tag($0)
        }
      }
      TextField("Short title", text: $sideTaskTitle)
        .textFieldStyle(.roundedBorder)
      TextEditor(text: $sideTaskRequest)
        .frame(minHeight: 150)
        .overlay(RoundedRectangle(cornerRadius: 8).stroke(LoomGraphite.separator))
      Text(sideTaskStatusLabelText(store.sideTaskOperationStatus))
        .font(.caption).foregroundStyle(.secondary)
      HStack {
        Spacer()
        if store.sideTaskProposal == nil {
          Button("Review proposal") {
            Task {
              await store.proposeSideTask(
                missionID: missionID,
                purpose: sideTaskPurpose,
                mode: sideTaskMode,
                title: sideTaskTitle,
                authorizedRequest: sideTaskRequest
              )
            }
          }
          .buttonStyle(.borderedProminent)
          .disabled(
            sideTaskTitle.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
              || sideTaskRequest.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
        } else {
          Button("Back") { store.discardSideTaskProposal() }
          Button("Confirm and run") {
            Task { await store.confirmSideTaskProposal() }
          }
          .buttonStyle(.borderedProminent)
        }
      }
    }
    .padding(24)
    .accessibilityLabel("Side-task proposal and confirmation")
  }

  private var inspectorSectionIcon: String {
    switch store.workbench.selectedContinuity.inspector {
    case .team: return "person.crop.circle"
    case .plan: return "point.3.connected.trianglepath.dotted"
    case .changes: return "arrow.triangle.2.circlepath"
    case .evidence: return "checkmark.seal"
    }
  }

  @ViewBuilder
  private func agentRecoveryInspector(
    record: LocalProductMissionSummary?
  ) -> some View {
    let candidates = store.agentRecoveryCandidates.filter {
      record == nil || $0.teamInstanceID == record?.teamInstanceID
    }
    if !candidates.isEmpty || store.agentRecoveryFailure != nil {
      Divider()
      HStack(spacing: 7) {
        Text("Recovery")
          .font(.subheadline.weight(.semibold))
        Spacer()
        Button {
          Task { await store.previewAgentAttemptRecoveries() }
        } label: {
          Image(systemName: "arrow.clockwise")
        }
        .buttonStyle(.plain)
        .loomActionTarget()
        .disabled(store.isPreviewingAgentRecoveries)
        .accessibilityLabel("Refresh Agent recoveries")
        .help("Refresh authoritative recovery candidates")
      }
      ForEach(candidates, id: \.candidateDigest) { candidate in
        HStack(alignment: .top, spacing: 9) {
          Image(systemName: "exclamationmark.triangle")
            .foregroundStyle(LoomGraphite.statusWarning)
            .frame(width: 18, height: 18)
            .accessibilityHidden(true)
          VStack(alignment: .leading, spacing: 3) {
            Text(missionHumanStatus(candidate.agentInstanceID))
              .font(.caption.weight(.semibold))
            Text(
              "\(missionHumanStatus(candidate.harnessAdapter)) · \(candidate.providerAccountID) · \(candidate.modelID) · Credential v\(candidate.credentialRevision)"
            )
            .font(.caption2)
            .foregroundStyle(.secondary)
            .fixedSize(horizontal: false, vertical: true)
          }
          Spacer(minLength: 8)
          if candidate.status == .consumed {
            Image(systemName: "checkmark.circle")
              .foregroundStyle(.secondary)
              .accessibilityLabel("Agent recovery already started")
              .help("This recovery decision was already consumed")
          } else if store.canResumeAgentAttemptRecovery(candidate) {
            Button {
              Task { await store.resumeAgentAttemptRecovery(candidate) }
            } label: {
              Image(systemName: "play.fill")
            }
            .buttonStyle(.plain)
            .loomActionTarget()
            .disabled(store.agentRecoveriesInFlight.contains(candidate.candidateDigest))
            .accessibilityLabel("Resume Agent Attempt")
            .help("Resume this explicitly authorized Agent Attempt")
          } else if store.canConfirmAgentAttemptRecovery(candidate) {
            Button {
              pendingAgentRecovery = candidate
            } label: {
              Image(systemName: "checkmark.shield")
            }
            .buttonStyle(.plain)
            .loomActionTarget()
            .disabled(store.agentRecoveriesInFlight.contains(candidate.candidateDigest))
            .accessibilityLabel("Review Agent recovery confirmation")
            .help("Review and confirm this exact recovery binding")
          } else if store.agentRecoveriesInFlight.contains(candidate.candidateDigest) {
            ProgressView()
              .controlSize(.small)
              .accessibilityLabel("Agent recovery in progress")
          } else {
            Image(systemName: "lock")
              .foregroundStyle(.secondary)
              .accessibilityLabel("Agent recovery unavailable")
              .help("Refresh the authoritative recovery state")
          }
        }
        .padding(.vertical, 4)
      }
      if let failure = store.agentRecoveryFailure {
        VStack(alignment: .leading, spacing: 5) {
          Text(failure.title)
            .font(.caption.weight(.semibold))
            .foregroundStyle(LoomGraphite.statusDanger)
          Text(failure.detail)
            .font(.caption2)
            .foregroundStyle(.secondary)
            .fixedSize(horizontal: false, vertical: true)
          HStack(spacing: 10) {
            if failure.recoverable {
              Button("Refresh", systemImage: "arrow.clockwise") {
                Task { await store.previewAgentAttemptRecoveries() }
              }
              .buttonStyle(.plain)
              .font(.caption)
            }
            if !failure.incidentID.isEmpty {
              Button("Copy incident ID", systemImage: "doc.on.doc") {
                NSPasteboard.general.clearContents()
                NSPasteboard.general.setString(failure.incidentID, forType: .string)
              }
              .buttonStyle(.plain)
              .font(.caption)
            }
          }
        }
      }
    }
  }

  @ViewBuilder
  private func toolRecoveryInspector() -> some View {
    if store.toolRecoveryReachable {
      Divider()
      HStack(spacing: 7) {
        Text("ToolCall recovery")
          .font(.subheadline.weight(.semibold))
        Spacer()
        Button {
          Task { await store.previewToolRecoveries() }
        } label: {
          Image(systemName: "arrow.clockwise")
        }
        .buttonStyle(.plain)
        .loomActionTarget()
        .disabled(
          store.isPreviewingToolRecoveries || !store.toolRecoveriesInFlight.isEmpty
        )
        .accessibilityLabel("Refresh ToolCall recoveries")
        .help("Refresh authoritative content-free ToolCall recovery candidates")
      }
      if store.toolRecoveryCandidates.isEmpty && store.toolRecoveryFailure == nil {
        Text("No unresolved ToolCalls in the current authoritative view.")
          .font(.caption)
          .foregroundStyle(.secondary)
      }
      ForEach(store.toolRecoveryCandidates, id: \.candidateDigest) { candidate in
        HStack(alignment: .top, spacing: 9) {
          Image(systemName: "exclamationmark.shield")
            .foregroundStyle(LoomGraphite.statusWarning)
            .frame(width: 18, height: 18)
            .accessibilityHidden(true)
          VStack(alignment: .leading, spacing: 3) {
            Text(missionHumanStatus(candidate.tool))
              .font(.caption.weight(.semibold))
            Text("Job \(candidate.jobID) · side effect unknown")
              .font(.caption2)
              .foregroundStyle(.secondary)
              .fixedSize(horizontal: false, vertical: true)
            HStack(spacing: 8) {
              Text("Incident \(candidate.incidentID)")
                .font(.caption2)
                .foregroundStyle(.secondary)
                .lineLimit(1)
              Button {
                NSPasteboard.general.clearContents()
                NSPasteboard.general.setString(candidate.incidentID, forType: .string)
              } label: {
                Image(systemName: "doc.on.doc")
              }
              .buttonStyle(.plain)
              .accessibilityLabel("Copy original ToolCall incident ID")
              .help("Copy original ToolCall incident ID")
            }
          }
          Spacer(minLength: 8)
          if candidate.status == .resolved, let decision = candidate.decision {
            Image(systemName: "checkmark.circle")
              .foregroundStyle(.secondary)
              .accessibilityLabel(
                "ToolCall recovery resolved: \(missionToolRecoveryActionLabel(decision))"
              )
              .help("Resolved: \(missionToolRecoveryActionLabel(decision))")
          } else if store.toolRecoveriesInFlight.contains(candidate.candidateDigest) {
            ProgressView()
              .controlSize(.small)
              .accessibilityLabel("ToolCall recovery in progress")
          } else {
            Menu {
              ForEach(candidate.availableActions, id: \.rawValue) { action in
                Button {
                  pendingToolRecovery = PendingToolRecoveryDecision(
                    candidate: candidate, action: action
                  )
                } label: {
                  Label(
                    missionToolRecoveryActionLabel(action),
                    systemImage: missionToolRecoveryActionIcon(action)
                  )
                }
                .disabled(!store.canResolveToolRecovery(candidate, action: action))
              }
            } label: {
              Image(systemName: "ellipsis.circle")
            }
            .menuStyle(.borderlessButton)
            .fixedSize()
            .accessibilityLabel("Choose ToolCall recovery action")
            .help("Choose an action advertised by the recovery authority")
          }
        }
        .padding(.vertical, 4)
      }
      if let failure = store.toolRecoveryFailure {
        VStack(alignment: .leading, spacing: 5) {
          Text(failure.title)
            .font(.caption.weight(.semibold))
            .foregroundStyle(LoomGraphite.statusDanger)
          Text(failure.detail)
            .font(.caption2)
            .foregroundStyle(.secondary)
            .fixedSize(horizontal: false, vertical: true)
          HStack(spacing: 10) {
            Button("Refresh", systemImage: "arrow.clockwise") {
              Task { await store.previewToolRecoveries() }
            }
            .buttonStyle(.plain)
            .font(.caption)
            Button("View diagnostics", systemImage: "stethoscope") {
              prepareVaultDiagnosticPreview()
            }
            .buttonStyle(.plain)
            .font(.caption)
            .disabled(preparingVaultDiagnostics)
            if !failure.incidentID.isEmpty {
              Button("Copy incident ID", systemImage: "doc.on.doc") {
                NSPasteboard.general.clearContents()
                NSPasteboard.general.setString(failure.incidentID, forType: .string)
              }
              .buttonStyle(.plain)
              .font(.caption)
            }
          }
        }
      }
    }
  }

  private func workbenchToolbar(
    title: String,
    subtitle: String,
    backAction: (() -> Void)? = nil
  ) -> some View {
    HStack {
      if let backAction {
        Button(action: backAction) {
          Image(systemName: "chevron.left")
            .frame(width: 28, height: 28)
        }
        .buttonStyle(.plain)
        .help("All Missions")
        .accessibilityLabel("Back to all Missions")
      }
      VStack(alignment: .leading, spacing: 2) {
        Text(title)
          .font(.title3.weight(.semibold))
        Text(subtitle)
          .font(.caption)
          .foregroundStyle(.secondary)
      }
      Spacer()
      if !store.workbench.selectedContinuity.inspectorVisible,
        case .mission = store.workbench.route
      {
        Button {
          store.setMissionInspectorVisible(true)
        } label: {
          Image(systemName: "sidebar.right")
        }
        .buttonStyle(.plain)
        .loomActionTarget()
        .accessibilityLabel("Show Inspector")
      }
      Button {
        Task {
          switch store.workbench.route {
          case .mission(let id):
            await store.refreshMission(id)
          case .board, .teams, .attention, .library:
            await store.refresh()
          }
        }
      } label: {
        Image(systemName: "arrow.clockwise")
      }
      .buttonStyle(.plain)
      .loomActionTarget()
      .accessibilityLabel("Refresh authoritative view")
    }
    .padding(.horizontal, 18)
    .frame(height: 58)
    .background(.bar)
  }

  /// Mission room Outcome card: the milestone plus, when the Mission is
  /// blocked or failed, the concrete reason and the next step, so a user
  /// opening the room immediately sees why and what to do.
  private func missionOutcomeBlock(
    _ record: LocalProductMissionSummary?
  ) -> some View {
    let milestone = record?.lastMilestone
      ?? "No authoritative Mission record is available."
    let reason = (record?.blockReason ?? "")
      .trimmingCharacters(in: .whitespacesAndNewlines)
    let needsReason =
      record?.status == "blocked" || record?.status == "failed"
    return VStack(alignment: .leading, spacing: 8) {
      Label("Outcome", systemImage: "scope")
        .font(.subheadline.weight(.semibold))
      Text(milestone)
        .foregroundStyle(.secondary)
        .textSelection(.enabled)
      if needsReason, !reason.isEmpty {
        Label(
          missionHumanStatus(reason),
          systemImage: "exclamationmark.triangle"
        )
        .font(.callout.weight(.medium))
        .foregroundStyle(.orange)
        .textSelection(.enabled)
        Text("This Attempt cannot proceed on its own. Enter guidance below to review a new Attempt, or open the Inspector for node-level failure details.")
          .font(.caption)
          .foregroundStyle(.secondary)
          .fixedSize(horizontal: false, vertical: true)
      }
    }
    .padding(14)
    .frame(maxWidth: .infinity, alignment: .leading)
    .background(
      LoomGraphite.surface,
      in: RoundedRectangle(
        cornerRadius: LoomGraphite.cardRadius,
        style: .continuous
      )
    )
  }

  private func contentBlock(
    title: String,
    icon: String,
    text: String
  ) -> some View {
    VStack(alignment: .leading, spacing: 8) {
      Label(title, systemImage: icon)
        .font(.subheadline.weight(.semibold))
      Text(text)
        .foregroundStyle(.secondary)
        .textSelection(.enabled)
    }
    .padding(14)
    .frame(maxWidth: .infinity, alignment: .leading)
    .background(
      LoomGraphite.surface,
      in: RoundedRectangle(
        cornerRadius: LoomGraphite.cardRadius,
        style: .continuous
      )
    )
  }

  @ViewBuilder
  private func missionActivity(
    _ mission: LocalProductMissionSummary?
  ) -> some View {
    let entries = missionActivityEntries(mission: mission, timeline: store.timeline)
    let isLive = mission.map { missionActivityNeedsRefresh(status: $0.status) } ?? false
    VStack(alignment: .leading, spacing: 10) {
      HStack(spacing: 8) {
        Label("Agent conversation", systemImage: "bubble.left.and.text.bubble.right")
          .font(.subheadline.weight(.semibold))
        Spacer()
        if isLive {
          ProgressView()
            .controlSize(.small)
            .accessibilityLabel("Mission activity is updating")
          Text("Live")
            .font(.caption.weight(.medium))
            .foregroundStyle(LoomGraphite.accent)
        }
      }
      if let mission, mission.nodeCount > 0 {
        ProgressView(
          value: Double(min(mission.completedNodeCount, mission.nodeCount)),
          total: Double(mission.nodeCount)
        )
        .accessibilityLabel(
          "\(mission.completedNodeCount) of \(mission.nodeCount) Mission steps complete"
        )
      }
      if let gap = store.timeline?.gap {
        Label(
          "Some live activity is unavailable · \(humanStatus(gap.reason))",
          systemImage: "exclamationmark.triangle"
        )
        .font(.caption)
        .foregroundStyle(.orange)
      }
      if entries.isEmpty {
        HStack(spacing: 8) {
          if isLive { ProgressView().controlSize(.small) }
          Text(
            mission?.activeNodeCount == 0
              ? "No Agent activity is available yet."
              : "The Agent Team is starting its first step."
          )
          .foregroundStyle(.secondary)
        }
      } else {
        ForEach(Array(entries.enumerated()), id: \.element.id) { index, entry in
          if index > 0 { Divider() }
          VStack(alignment: .leading, spacing: 7) {
            HStack(alignment: .firstTextBaseline, spacing: 8) {
              Image(systemName: "person.crop.circle")
                .foregroundStyle(LoomGraphite.accent)
                .accessibilityHidden(true)
              VStack(alignment: .leading, spacing: 2) {
                Text(entry.title)
                  .font(.callout.weight(.semibold))
                if !entry.route.isEmpty {
                  Text(entry.route)
                    .font(.caption2)
                    .foregroundStyle(.secondary)
                    .lineLimit(2)
                }
              }
              Spacer(minLength: 8)
              Label(
                missionHumanStatus(entry.status),
                systemImage: missionActivityStatusIcon(entry.status)
              )
              .font(.caption)
              .foregroundStyle(missionActivityStatusColor(entry.status))
            }
            if entry.text.isEmpty {
              HStack(spacing: 8) {
                if isLive && !["succeeded", "failed", "cancelled", "blocked"]
                  .contains(entry.status)
                {
                  ProgressView().controlSize(.small)
                }
                Text("Waiting for this Agent's first visible update.")
                  .font(.callout)
                  .foregroundStyle(.secondary)
              }
            } else {
              Text(
                missionActivityVisibleText(
                  entry.text,
                  expanded: expandedMissionActivityIDs.contains(entry.id)
                )
              )
                .font(.body)
                .lineSpacing(3)
                .lineLimit(
                  expandedMissionActivityIDs.contains(entry.id) ? nil : 12
                )
                .textSelection(.enabled)
              if entry.text.count > 800 {
                Button(
                  expandedMissionActivityIDs.contains(entry.id)
                    ? "Show less" : "Show full update"
                ) {
                  if expandedMissionActivityIDs.contains(entry.id) {
                    expandedMissionActivityIDs.remove(entry.id)
                  } else {
                    expandedMissionActivityIDs.insert(entry.id)
                  }
                }
                .buttonStyle(.borderless)
              }
              if entry.isTruncated {
                Label(
                  "This live view reached its display limit; the final artifact remains available separately.",
                  systemImage: "ellipsis.circle"
                )
                .font(.caption2)
                .foregroundStyle(.secondary)
              }
            }
          }
          .padding(.vertical, 5)
        }
      }
      Text("Visible Agent output is tentative until terminal Evidence is accepted.")
        .font(.caption2)
        .foregroundStyle(.secondary)
    }
    .padding(14)
    .frame(maxWidth: .infinity, alignment: .leading)
    .background(
      LoomGraphite.surface,
      in: RoundedRectangle(
        cornerRadius: LoomGraphite.cardRadius,
        style: .continuous
      )
    )
  }

  private func missionActivityStatusIcon(_ status: String) -> String {
    switch status {
    case "succeeded": return "checkmark.circle.fill"
    case "failed", "cancelled": return "xmark.circle.fill"
    case "blocked", "human_required": return "exclamationmark.circle.fill"
    default: return "arrow.triangle.2.circlepath"
    }
  }

  private func missionActivityStatusColor(_ status: String) -> Color {
    switch status {
    case "succeeded": return LoomGraphite.statusSuccess
    case "failed", "cancelled": return LoomGraphite.statusDanger
    case "blocked", "human_required": return LoomGraphite.statusWarning
    default: return LoomGraphite.accent
    }
  }

  @ViewBuilder
  private func agentInputControls(_ mission: LocalProductMissionSummary?) -> some View {
    let nodes = (store.timeline?.board.nodes ?? []).filter {
      $0.currentAttempt > 0 && $0.routeSegmentAvailable
        && $0.executionBindingAvailable && $0.contextCapsuleAvailable
        && !["succeeded", "failed", "cancelled", "blocked"].contains($0.status)
    }
    if store.agentInputReachable && !nodes.isEmpty {
      VStack(alignment: .leading, spacing: 10) {
        Label("Agent input", systemImage: "arrow.turn.down.right")
          .font(.subheadline.weight(.semibold))
        ForEach(nodes) { node in
          agentInputRow(node, mission: mission)
        }
      }
      .frame(maxWidth: .infinity, alignment: .leading)
    }
  }

  private func agentInputRow(
    _ node: LocalProductNode,
    mission: LocalProductMissionSummary?
  ) -> some View {
    let mode = agentInputModes[node.logicalNodeID] ?? .queue
    let draft = agentInputDrafts[node.logicalNodeID] ?? ""
    let inFlight = store.agentInputsInFlight.contains(node.logicalNodeID)
    return VStack(alignment: .leading, spacing: 9) {
      HStack(alignment: .firstTextBaseline, spacing: 8) {
        VStack(alignment: .leading, spacing: 2) {
          Text(
            missionNodeDisplayTitle(
              nodeID: node.logicalNodeID,
              topology: mission?.topology ?? [],
              pulse: mission?.teamPulse ?? []
            )
          )
          .font(.callout.weight(.semibold))
          Text(
            "\(missionHumanStatus(node.harnessAdapter)) · \(missionHumanStatus(node.providerID)) · \(node.modelID)"
          )
          .font(.caption)
          .foregroundStyle(.secondary)
          .lineLimit(1)
        }
        Spacer(minLength: 8)
        if inFlight {
          ProgressView()
            .controlSize(.small)
            .accessibilityLabel("Sending Agent input")
        } else {
          Text(missionHumanStatus(node.status))
            .font(.caption.weight(.medium))
            .foregroundStyle(.secondary)
        }
      }

      Picker("Input mode", selection: agentInputModeBinding(for: node.logicalNodeID)) {
        ForEach(LocalProductAgentInputMode.allCases) { option in
          Text(option.rawValue.capitalized).tag(option)
        }
      }
      .pickerStyle(.segmented)
      .labelsHidden()
      .disabled(inFlight)

      HStack(spacing: 8) {
        TextField("Message Agent", text: agentInputDraftBinding(for: node.logicalNodeID))
          .textFieldStyle(.plain)
          .lineLimit(1...4)
          .onSubmit { submitAgentInput(node) }
        Button {
          submitAgentInput(node)
        } label: {
          Image(systemName: "arrow.up.circle.fill")
            .font(.title3)
        }
        .buttonStyle(.plain)
        .foregroundStyle(LoomGraphite.accent)
        .disabled(
          inFlight || !store.canSendAgentInput(to: node)
            || draft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
        )
        .accessibilityLabel("Send \(mode.rawValue) input")
        .help("Send \(mode.rawValue) input to this Agent")
      }
      .padding(.horizontal, 10)
      .frame(minHeight: 38)
      .background(LoomGraphite.canvas, in: RoundedRectangle(cornerRadius: 7))
      .overlay {
        RoundedRectangle(cornerRadius: 7)
          .stroke(LoomGraphite.separator, lineWidth: 1)
      }

      if let failure = store.agentInputFailures[node.logicalNodeID] {
        HStack(spacing: 7) {
          Image(systemName: "exclamationmark.triangle.fill")
            .foregroundStyle(.orange)
          Text(failure.detail)
            .font(.caption)
            .foregroundStyle(.secondary)
            .fixedSize(horizontal: false, vertical: true)
          Spacer(minLength: 4)
          if !failure.incidentID.isEmpty {
            Button {
              NSPasteboard.general.clearContents()
              NSPasteboard.general.setString(failure.incidentID, forType: .string)
            } label: {
              Image(systemName: "doc.on.doc")
            }
            .buttonStyle(.plain)
            .accessibilityLabel("Copy incident ID")
            .help("Copy incident ID")
          }
        }
      } else if let receipt = store.agentInputReceipts[node.logicalNodeID] {
        Label(agentInputReceiptLabel(receipt), systemImage: "checkmark.circle.fill")
          .font(.caption)
          .foregroundStyle(.secondary)
      }
    }
    .padding(12)
    .background(
      LoomGraphite.surface,
      in: RoundedRectangle(cornerRadius: LoomGraphite.cardRadius, style: .continuous)
    )
  }

  private func agentInputDraftBinding(for nodeID: String) -> Binding<String> {
    Binding(
      get: { agentInputDrafts[nodeID] ?? "" },
      set: { agentInputDrafts[nodeID] = String($0.prefix(32_768)) }
    )
  }

  private func agentInputModeBinding(
    for nodeID: String
  ) -> Binding<LocalProductAgentInputMode> {
    Binding(
      get: { agentInputModes[nodeID] ?? .queue },
      set: { agentInputModes[nodeID] = $0 }
    )
  }

  private func submitAgentInput(_ node: LocalProductNode) {
    let mode = agentInputModes[node.logicalNodeID] ?? .queue
    let content = agentInputDrafts[node.logicalNodeID] ?? ""
    let previousReceipt = store.agentInputReceipts[node.logicalNodeID]?.inputID
    Task {
      await store.sendAgentInput(to: node, mode: mode, content: content)
      if let receipt = store.agentInputReceipts[node.logicalNodeID],
        receipt.inputID != previousReceipt
      {
        agentInputDrafts[node.logicalNodeID] = ""
      }
    }
  }

  private func agentInputReceiptLabel(_ receipt: LocalProductAgentInputReceipt) -> String {
    if receipt.mode == .queue {
      return "Queued for turn \(receipt.targetTurnSequence)"
    }
    return "\(receipt.mode.rawValue.capitalized) scheduled for step \(receipt.targetStepSequence)"
  }

  private func truthfulState(
    icon: String,
    title: String,
    detail: String
  ) -> some View {
    ContentUnavailableView(
      title,
      systemImage: icon,
      description: Text(detail)
    )
    .frame(maxWidth: .infinity, maxHeight: .infinity)
  }

  private var teamsWorkspace: some View {
    VStack(spacing: 0) {
      workbenchToolbar(
        title: "Teams",
        subtitle: "Confirmed Team records"
      )
      workspaceReadBanner
      if let snapshot = store.snapshot {
        if snapshot.teams.isEmpty {
          truthfulState(
            icon: "person.3",
            title: "No confirmed Teams",
            detail: "Create and confirm a Team before starting a Mission."
          )
        } else {
          ScrollView {
            LazyVStack(alignment: .leading, spacing: 12) {
              ForEach(currentTeamConfigurations(snapshot.teams)) { team in
                let existingMission = snapshot.missions.first {
                  $0.teamInstanceID == team.teamInstanceID
                }
                VStack(alignment: .leading, spacing: 8) {
                  HStack {
                    Text(
                      LocalProductExperience.visibleName(
                        team.displayName,
                        internalID: team.teamInstanceID,
                        fallback: "Confirmed Team"
                      )
                    )
                    .font(.headline)
                    Spacer()
                    Text(humanStatus(team.state))
                      .font(.caption.weight(.semibold))
                      .foregroundStyle(
                        team.executable && !team.readOnly
                          ? .green
                          : .secondary
                      )
                  }
                  Text(
                    workspacePresentation.isCurrent
                      ? (team.executable && !team.readOnly
                        ? (existingMission == nil
                          ? "Confirmed · Available for explicit Mission start"
                          : "Confirmed · Existing Mission available in the Board")
                        : "Read-only · Not available for execution")
                      : "Preserved record · Refresh before execution"
                  )
                  .font(.callout)
                  .foregroundStyle(.secondary)
                  Text("Source · \(humanStatus(team.sourceKind))")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                  if team.agents.isEmpty {
                    Label(
                      "Agent composition unavailable",
                      systemImage: "person.crop.circle.badge.questionmark"
                    )
                    .font(.caption)
                    .foregroundStyle(.secondary)
                  } else {
                    VStack(alignment: .leading, spacing: 0) {
                      ForEach(Array(team.agents.enumerated()), id: \.element.id) { index, agent in
                        if index > 0 {
                          Divider()
                        }
                        HStack(alignment: .top, spacing: 10) {
                          Image(systemName: agent.roleKind == "main"
                            ? "person.crop.circle.fill" : "person.crop.circle")
                            .foregroundStyle(LoomGraphite.accent)
                            .frame(width: 18, height: 18)
                            .accessibilityHidden(true)
                          VStack(alignment: .leading, spacing: 3) {
                            Text(teamAgentTitle(agent))
                              .font(.caption.weight(.semibold))
                            Text(teamAgentRouteLabel(agent))
                              .font(.caption2)
                              .foregroundStyle(.secondary)
                              .fixedSize(horizontal: false, vertical: true)
                          }
                          Spacer(minLength: 8)
                          Label(
                            missionHumanStatus(agent.bindingStatus),
                            systemImage: agent.bindingStatus == "configured"
                              ? "checkmark.circle" : "exclamationmark.triangle"
                          )
                          .font(.caption2.weight(.medium))
                          .foregroundStyle(
                            agent.bindingStatus == "configured"
                              ? Color.secondary : LoomGraphite.statusWarning
                          )
                        }
                        .padding(.vertical, 7)
                        .accessibilityElement(children: .combine)
                      }
                    }
                  }
                  HStack(spacing: 8) {
                    Label(
                      team.executable && !team.readOnly
                        ? (existingMission == nil
                          ? "Ready for a Mission"
                          : "Mission · \(missionHumanStatus(existingMission?.status ?? ""))")
                        : "Read-only Team",
                      systemImage: team.executable && !team.readOnly
                        ? (existingMission == nil
                          ? "play.circle"
                          : "arrow.up.right.square")
                        : "lock"
                    )
                    .font(.caption.weight(.medium))
                    .foregroundStyle(
                      team.executable && !team.readOnly
                        ? LoomGraphite.accent
                        : Color.secondary
                    )
                    Spacer()
                    if team.executable && !team.readOnly {
                      Button {
                        if let existingMission {
                          Task {
                            await store.openMissionAndActivate(existingMission.missionID)
                          }
                        } else {
                          newMissionStartsNewAttempt = false
                          newMissionTeamID = team.teamInstanceID
                          showNewMission = true
                        }
                      } label: {
                        Label(
                          existingMission == nil ? "Start Mission" : "Open Mission",
                          systemImage: existingMission == nil
                            ? "arrow.right.circle" : "arrow.up.right.square"
                        )
                      }
                      .buttonStyle(.bordered)
                      .controlSize(.small)
                      .accessibilityLabel(
                        existingMission == nil
                          ? "Start a Mission with this Team"
                          : "Open the existing Mission for this Team"
                      )
                      .help(
                        existingMission == nil
                          ? "Start a Mission with this Team"
                          : "Open the existing Mission for this Team"
                      )
                      if let existingMission,
                         ["failed", "blocked", "cancelled", "degraded", "human_required", "ready_for_review"].contains(existingMission.status) {
                        Button {
                          newMissionStartsNewAttempt = true
                          newMissionTeamID = team.teamInstanceID
                          newMissionObjective = ""
                          showNewMission = true
                        } label: {
                          Label("New Attempt", systemImage: "plus.circle")
                        }
                        .buttonStyle(.bordered)
                        .controlSize(.small)
                        .accessibilityLabel("Start a new audited Attempt with this Team")
                        .help("Start a new audited Attempt with this Team")
                      }
                    }
                  }
                }
                .padding(14)
                .background(
                  LoomGraphite.surface,
                  in: RoundedRectangle(
                    cornerRadius: LoomGraphite.cardRadius,
                    style: .continuous
                  )
                )
                .accessibilityElement(children: .contain)
              }
            }
            .padding(18)
          }
        }
      } else {
        truthfulState(
          icon: "person.3",
          title: "Teams unavailable",
          detail: connectionLabel
        )
      }
    }
    .background(LoomGraphite.canvas)
  }

  private var attentionWorkspace: some View {
    VStack(spacing: 0) {
      workbenchToolbar(
        title: "Needs You",
        subtitle: "Approvals and blocked decisions"
      )
      workspaceReadBanner
      if let snapshot = store.snapshot {
        if snapshot.attention.isEmpty {
          let copy = missionAttentionEmptyCopy(
            presentation: workspacePresentation
          )
          truthfulState(
            icon: copy.icon,
            title: copy.title,
            detail: copy.detail
          )
        } else {
          ScrollView {
            LazyVStack(alignment: .leading, spacing: 12) {
              ForEach(snapshot.attention) { item in
                Button {
                  Task { await store.openMissionAndActivate(
                    "mission/\(item.teamInstanceID)"
                  ) }
                } label: {
                  VStack(alignment: .leading, spacing: 7) {
                    HStack {
                      Text(humanStatus(item.kind))
                        .font(.headline)
                      Spacer()
                      Text(humanStatus(item.severity))
                        .font(.caption.weight(.semibold))
                        .foregroundStyle(.orange)
                    }
                    Text(humanStatus(item.status))
                      .foregroundStyle(.secondary)
                    if !item.actionRequired.isEmpty {
                      Text(item.actionRequired)
                        .font(.callout)
                    }
                    Label("Open Mission", systemImage: "arrow.right.circle")
                      .font(.caption.weight(.medium))
                      .foregroundStyle(LoomGraphite.accent)
                  }
                  .frame(maxWidth: .infinity, alignment: .leading)
                  .padding(14)
                  .background(
                    LoomGraphite.surface,
                    in: RoundedRectangle(
                      cornerRadius: LoomGraphite.cardRadius,
                      style: .continuous
                    )
                  )
                  .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                .accessibilityLabel(
                  "\(humanStatus(item.kind)): \(item.actionRequired). Open Mission"
                )
                .help("Open the Mission that needs attention")
              }
            }
            .padding(18)
          }
        }
      } else {
        truthfulState(
          icon: "exclamationmark.bubble",
          title: "Attention unavailable",
          detail: connectionLabel
        )
      }
    }
    .background(LoomGraphite.canvas)
  }

  private var libraryWorkspace: some View {
    VStack(spacing: 0) {
      workbenchToolbar(
        title: "Library",
        subtitle: "Evolution Assets, History and Compare"
      )
      workspaceReadBanner
      if let snapshot = store.snapshot {
        ScrollView {
          // Non-lazy so the capped-history footnote stays in the view hierarchy
          // for assistive access and for users who have not scrolled to the end.
          VStack(alignment: .leading, spacing: 18) {
            HStack {
              VStack(alignment: .leading, spacing: 3) {
                Text("Assets")
                  .font(.title3.weight(.semibold))
                Text("Exact revision lineage · Candidate-only until explicit activation")
                  .font(.caption)
                  .foregroundStyle(.secondary)
              }
              Spacer()
              Button("Refresh") { Task { await store.loadEvolutionAssets() } }
              Button("Create Candidate") { showCreateAsset = true }
                .buttonStyle(.borderedProminent)
                .disabled(!store.evolutionAssetReachable)
            }
            Text("Journey · \(store.evolutionAssetJourneyID) · \(store.evolutionAssetStatus)")
              .font(.caption2.monospaced())
              .foregroundStyle(.secondary)
            if let assets = store.evolutionAssets, !assets.promotionSources.isEmpty {
              VStack(alignment: .leading, spacing: 8) {
                Text("Accepted Run promotion sources").font(.headline)
                ForEach(assets.promotionSources) { source in
                  HStack {
                    VStack(alignment: .leading, spacing: 2) {
                      Text("\(source.runID) · generation \(source.runGeneration)")
                        .font(.caption.monospaced())
                      Text(
                        "Run digest \(source.runDigest) · Evidence \(source.evidenceIDs.joined(separator: ", "))"
                      )
                      .font(.caption2.monospaced())
                      .foregroundStyle(.secondary)
                      .textSelection(.enabled)
                    }
                    Spacer()
                    Button("Promote to Candidate") {
                      pendingAssetMutation = PendingEvolutionAssetMutation(
                        action: "Promote", candidateID: source.runID,
                        definitionID: "", revisionID: ""
                      )
                    }
                  }
                }
              }
            }
            TextField("Search assets", text: $assetSearchText)
              .textFieldStyle(.roundedBorder)
            if let assets = store.evolutionAssets, !assets.records.isEmpty {
              ForEach(
                assets.records.filter {
                  assetSearchText.isEmpty
                    || $0.definition.name.localizedCaseInsensitiveContains(assetSearchText)
                    || $0.definition.description.localizedCaseInsensitiveContains(assetSearchText)
                    || $0.definition.definitionID.localizedCaseInsensitiveContains(assetSearchText)
                }
              ) { asset in
                VStack(alignment: .leading, spacing: 8) {
                  HStack {
                    Text(
                      asset.definition.name.isEmpty
                        ? asset.definition.definitionID : asset.definition.name
                    )
                    .font(.headline)
                    Spacer()
                    Text(asset.definition.lifecycle.rawValue.capitalized)
                      .font(.caption.weight(.semibold))
                  }
                  Text(asset.definition.description)
                    .foregroundStyle(.secondary)
                  ForEach(asset.revisions) { revision in
                    VStack(alignment: .leading, spacing: 4) {
                      Text("Exact revision · \(revision.revisionID)")
                        .font(.callout.weight(.medium))
                      Text("Digest · \(revision.artifactDigest)")
                        .font(.caption2.monospaced())
                        .textSelection(.enabled)
                      Text(
                        "Risk · \(revision.risk) · Runtime compatibility · \(revision.compatibleRuntimeCapabilities.joined(separator: ", "))"
                      )
                      .font(.caption)
                      .foregroundStyle(.secondary)
                    }
                  }
                  let undecidedCandidate = assets.candidates.first {
                    $0.definitionID == asset.definition.definitionID && $0.decision.isEmpty
                  }
                  let latestRevision = asset.revisions.first {
                    $0.revisionID == asset.definition.latestRevisionID
                  }
                  let rollbackTarget = asset.revisions.first {
                    $0.revisionID != asset.definition.activeRevisionID && $0.lifecycle != .archived
                  }
                  HStack {
                    Button("Evaluate") {
                      if let candidate = undecidedCandidate {
                        pendingAssetMutation = PendingEvolutionAssetMutation(
                          action: "Evaluate", candidateID: candidate.candidateID,
                          definitionID: candidate.definitionID, revisionID: candidate.revisionID
                        )
                      }
                    }
                    .disabled(undecidedCandidate == nil)
                    Button("Activate") {
                      if let candidate = undecidedCandidate {
                        pendingAssetMutation = PendingEvolutionAssetMutation(
                          action: "Activate", candidateID: candidate.candidateID,
                          definitionID: candidate.definitionID, revisionID: candidate.revisionID
                        )
                      }
                    }
                    .disabled(undecidedCandidate == nil)
                    .help("Explicitly activate the exact Candidate revision.")
                    Button("Keep") {
                      if let candidate = undecidedCandidate {
                        pendingAssetMutation = PendingEvolutionAssetMutation(
                          action: "Keep", candidateID: candidate.candidateID,
                          definitionID: candidate.definitionID, revisionID: candidate.revisionID
                        )
                      }
                    }
                    .disabled(undecidedCandidate == nil)
                    Button("Reject", role: .destructive) {
                      if let candidate = undecidedCandidate {
                        pendingAssetMutation = PendingEvolutionAssetMutation(
                          action: "Reject", candidateID: candidate.candidateID,
                          definitionID: candidate.definitionID, revisionID: candidate.revisionID
                        )
                      }
                    }
                    .disabled(undecidedCandidate == nil)
                    Button(latestRevision?.lifecycle == .archived ? "Restore" : "Archive") {
                      if let revision = latestRevision {
                        pendingAssetMutation = PendingEvolutionAssetMutation(
                          action: revision.lifecycle == .archived ? "Restore" : "Archive",
                          candidateID: "", definitionID: revision.definitionID,
                          revisionID: revision.revisionID
                        )
                      }
                    }
                    .disabled(latestRevision == nil)
                    Button("Rollback") {
                      if let target = rollbackTarget {
                        pendingAssetMutation = PendingEvolutionAssetMutation(
                          action: "Rollback", candidateID: "",
                          definitionID: asset.definition.definitionID,
                          revisionID: target.revisionID
                        )
                      }
                    }
                    .disabled(asset.definition.activeRevisionID.isEmpty || rollbackTarget == nil)
                    .help("Rollback uses the exact prior revision digest.")
                    Button("Bind to Coding") {
                      if let active = asset.revisions.first(where: {
                        $0.revisionID == asset.definition.activeRevisionID
                      }) {
                        pendingAssetMutation = PendingEvolutionAssetMutation(
                          action: "Bind", candidateID: "",
                          definitionID: active.definitionID,
                          revisionID: active.revisionID
                        )
                      }
                    }
                    .disabled(
                      asset.definition.activeRevisionID.isEmpty
                        || !assets.bindingSubjects.contains(where: {
                          $0.subjectKind == "work_package" && $0.subjectID == "work-package.coding"
                        })
                    )
                    if latestRevision?.assetKind != .skill {
                      Button("Instantiate") {
                        if let revision = latestRevision {
                          pendingAssetMutation = PendingEvolutionAssetMutation(
                            action: "Instantiate", candidateID: "",
                            definitionID: revision.definitionID,
                            revisionID: revision.revisionID
                          )
                        }
                      }
                      .disabled(latestRevision == nil || latestRevision?.lifecycle == .archived)
                    }
                    if asset.revisions.count > 1 {
                      Button("Compare") {
                        Task {
                          await store.compareEvolutionRevisions(
                            definitionID: asset.definition.definitionID,
                            left: asset.revisions[0], right: asset.revisions[1]
                          )
                        }
                      }
                    }
                    Spacer()
                    Label("Materialization", systemImage: "shippingbox")
                      .font(.caption)
                      .foregroundStyle(.secondary)
                  }
                }
                .padding(14)
                .background(
                  LoomGraphite.surface,
                  in: RoundedRectangle(cornerRadius: LoomGraphite.cardRadius, style: .continuous)
                )
              }
              if assets.hasMore {
                Button("Next page") {
                  Task { await store.loadEvolutionAssets(cursor: assets.nextCursor) }
                }
              }
              if let diff = store.evolutionAssetDiff {
                VStack(alignment: .leading, spacing: 4) {
                  Text("Revision diff · \(diff.leftRevisionID) → \(diff.rightRevisionID)")
                    .font(.headline)
                  ForEach(Array(diff.changes.enumerated()), id: \.offset) { _, change in
                    Text("\(change.kind) · \(change.relativePath)")
                      .font(.caption.monospaced())
                  }
                }
              }
              if !assets.evaluations.isEmpty {
                Text("Evaluations").font(.headline)
                ForEach(assets.evaluations) { evaluation in
                  let usageText = evaluation.usageObserved ? "observed" : "unknown"
                  let costText = evaluation.costObserved ? "observed" : "unknown"
                  Text(
                    "\(evaluation.evaluationID) · \(evaluation.fixtureKind) · \(evaluation.qualityResult) · usage \(usageText) · cost \(costText)"
                  )
                  .font(.caption)
                }
              }
              if !assets.bindings.isEmpty {
                Text("Exact bindings").font(.headline)
                ForEach(assets.bindings) { binding in
                  Text(
                    "\(binding.subjectKind) · \(binding.subjectID) · \(binding.assetRevisionSetDigest)"
                  )
                  .font(.caption2.monospaced())
                }
              }
              if !assets.materializations.isEmpty {
                Text("Run materialization lineage").font(.headline)
                ForEach(assets.materializations) { materialization in
                  let state = materialization.cleaned ? "cleaned" : "published"
                  Text(
                    "\(materialization.runID) · attempt \(materialization.attemptNumber) · generation \(materialization.generation) · \(state)"
                  )
                  .font(.caption2.monospaced())
                }
              }
            } else {
              Text(
                store.evolutionAssetReachable
                  ? "No Evolution Assets yet. Create a reviewed Candidate to begin."
                  : "Evolution Assets are unavailable from this client connection."
              )
              .foregroundStyle(.secondary)
            }

            Divider()
            Text("History")
              .font(.title3.weight(.semibold))
            if snapshot.runs.isEmpty {
              Text("No Run history is available.")
                .foregroundStyle(.secondary)
            } else {
              ForEach(Array(snapshot.runs.reversed())) { run in
                historyRunCard(run, snapshot: snapshot)
              }
              if snapshot.runPage.hasMore {
                Text("Showing the 64 most recent Runs · older Runs remain in the Journal timeline.")
                  .font(.caption2)
                  .foregroundStyle(.secondary)
                  .padding(.top, 2)
              }
            }

            Divider()
            Text("Compare")
              .font(.title3.weight(.semibold))
            if snapshot.runs.count < 2 {
              Text("Two authoritative Runs are required for comparison.")
                .foregroundStyle(.secondary)
            } else {
              let pair = Array(snapshot.runs.suffix(2))
              HStack(alignment: .top, spacing: 12) {
                comparisonColumn(
                  label: "Previous",
                  run: pair[0],
                  snapshot: snapshot
                )
                comparisonColumn(
                  label: "Current",
                  run: pair[1],
                  snapshot: snapshot
                )
              }
            }
          }
          .padding(18)
        }
      } else {
        truthfulState(
          icon: "books.vertical",
          title: "History unavailable",
          detail: connectionLabel
        )
      }
    }
    .background(LoomGraphite.canvas)
  }

  private var createEvolutionAssetSheet: some View {
    VStack(alignment: .leading, spacing: 16) {
      HStack {
        VStack(alignment: .leading, spacing: 4) {
          Text("Create Evolution Asset Candidate").font(.title2.weight(.semibold))
          Text("This writes a versioned Candidate. It does not activate or run it.")
            .foregroundStyle(.secondary)
        }
        Spacer()
        Button("Cancel") { showCreateAsset = false }
      }
      Picker("Asset type", selection: $assetCreationKind) {
        Text("Local Skill").tag("create_skill")
        Text("Reviewed Skill Import").tag("import_skill")
        Text("Agent Template").tag("agent_template")
        Text("Team Template").tag("team_template")
        Text("Work Package Template").tag("work_package_template")
        Text("Recovery Strategy Template").tag("recovery_strategy_template")
      }
      TextField("Definition ID", text: $assetDefinitionID)
      TextField("Revision ID", text: $assetRevisionID)
      TextField("Name", text: $assetName)
      TextField("Description", text: $assetDescription, axis: .vertical)
        .lineLimit(3...6)
      TextField("Absolute local source path", text: $assetSourcePath)
        .font(.body.monospaced())
      Spacer()
      HStack {
        Text("Back or Cancel never writes.")
          .font(.caption)
          .foregroundStyle(.secondary)
        Spacer()
        Button("Create Candidate") {
          Task {
            if assetCreationKind == "create_skill" {
              await store.createEvolutionSkill(
                definitionID: assetDefinitionID, revisionID: assetRevisionID,
                name: assetName, description: assetDescription,
                sourcePath: assetSourcePath
              )
            } else if assetCreationKind == "import_skill" {
              await store.importEvolutionSkill(
                definitionID: assetDefinitionID, revisionID: assetRevisionID,
                name: assetName, description: assetDescription,
                sourcePath: assetSourcePath
              )
            } else if let kind = EvolutionAssetKind(rawValue: assetCreationKind) {
              let outputs: [EvolutionAssetKind: String] = [
                .agentTemplate: "agent_candidate",
                .teamTemplate: "team_draft",
                .workPackageTemplate: "work_package_candidate",
                .recoveryStrategyTemplate: "recovery_strategy_candidate",
              ]
              if let output = outputs[kind] {
                await store.createEvolutionTemplate(
                  kind: kind, output: output,
                  definitionID: assetDefinitionID, revisionID: assetRevisionID,
                  name: assetName, description: assetDescription,
                  sourcePath: assetSourcePath
                )
              }
            }
            if store.evolutionAssetStatus == "Current" {
              showCreateAsset = false
            }
          }
        }
        .buttonStyle(.borderedProminent)
        .disabled(assetName.isEmpty || !assetSourcePath.hasPrefix("/"))
      }
    }
    .padding(24)
    .accessibilityLabel("Evolution Asset Candidate creation")
  }

  private func confirmEvolutionAssetMutation() {
    guard let mutation = pendingAssetMutation, let snapshot = store.evolutionAssets else { return }
    pendingAssetMutation = nil
    Task {
      switch mutation.action {
      case "Activate", "Keep", "Reject", "Evaluate":
        guard
          let candidate = snapshot.candidates.first(where: {
            $0.candidateID == mutation.candidateID && $0.definitionID == mutation.definitionID
              && $0.revisionID == mutation.revisionID
          })
        else { return }
        if mutation.action == "Evaluate" {
          await store.evaluateEvolutionCandidate(candidate)
        } else if mutation.action == "Activate" {
          await store.activateEvolutionCandidate(candidate)
        } else {
          await store.decideEvolutionCandidate(candidate, retain: mutation.action == "Keep")
        }
      case "Archive", "Restore":
        guard
          let revision = snapshot.revisions.first(where: {
            $0.definitionID == mutation.definitionID && $0.revisionID == mutation.revisionID
          })
        else { return }
        await store.setEvolutionRevisionArchived(revision, archived: mutation.action == "Archive")
      case "Rollback":
        guard
          let definition = snapshot.definitions.first(where: {
            $0.definitionID == mutation.definitionID
          }),
          let revision = snapshot.revisions.first(where: {
            $0.definitionID == mutation.definitionID && $0.revisionID == mutation.revisionID
          })
        else { return }
        await store.rollbackEvolutionAsset(definition: definition, target: revision)
      case "Bind":
        guard
          let revision = snapshot.revisions.first(where: {
            $0.definitionID == mutation.definitionID && $0.revisionID == mutation.revisionID
          }),
          let subject = snapshot.bindingSubjects.first(where: {
            $0.subjectKind == "work_package" && $0.subjectID == "work-package.coding"
          })
        else { return }
        await store.bindEvolutionRevision(revision, to: subject)
      case "Promote":
        guard
          let source = snapshot.promotionSources.first(where: {
            $0.runID == mutation.candidateID
          })
        else { return }
        await store.promoteEvolutionRun(source)
      case "Instantiate":
        guard
          let revision = snapshot.revisions.first(where: {
            $0.definitionID == mutation.definitionID && $0.revisionID == mutation.revisionID
          })
        else { return }
        await store.instantiateEvolutionTemplate(revision)
      default:
        return
      }
    }
  }

  private func historyRunCard(
    _ run: LocalProductRunSummary,
    snapshot: LocalProductSnapshot
  ) -> some View {
    HStack(alignment: .top, spacing: 12) {
      Image(systemName: "clock.arrow.circlepath")
        .foregroundStyle(LoomGraphite.accent)
      VStack(alignment: .leading, spacing: 5) {
        Text(humanStatus(run.phase))
          .font(.headline)
        Text(
          run.terminalStatus.isEmpty
            ? "In progress"
            : humanStatus(run.terminalStatus)
        )
        .foregroundStyle(.secondary)
        Text(
          "Runtime · \(missionRuntimeDisplayName(run: run, snapshot: snapshot)) · Attempt generation \(run.claimGeneration)"
        )
        .font(.caption)
        .foregroundStyle(.secondary)
        Text("Accepted Evidence · \(evidenceCount(for: run, in: snapshot))")
          .font(.caption)
          .foregroundStyle(.secondary)
      }
      Spacer()
    }
    .padding(14)
    .background(
      LoomGraphite.surface,
      in: RoundedRectangle(
        cornerRadius: LoomGraphite.cardRadius,
        style: .continuous
      )
    )
    .accessibilityElement(children: .combine)
  }

  private func comparisonColumn(
    label: String,
    run: LocalProductRunSummary,
    snapshot: LocalProductSnapshot
  ) -> some View {
    VStack(alignment: .leading, spacing: 7) {
      Text(label)
        .font(.caption.weight(.semibold))
        .foregroundStyle(.secondary)
      Text(humanStatus(run.terminalStatus.isEmpty ? run.phase : run.terminalStatus))
        .font(.headline)
      Text("Runtime · \(missionRuntimeDisplayName(run: run, snapshot: snapshot))")
        .font(.caption)
      Text("Attempt generation · \(run.claimGeneration)")
        .font(.caption)
      Text("Evidence · \(evidenceCount(for: run, in: snapshot))")
        .font(.caption)
    }
    .frame(maxWidth: .infinity, alignment: .leading)
    .padding(14)
    .background(
      LoomGraphite.surface,
      in: RoundedRectangle(
        cornerRadius: LoomGraphite.cardRadius,
        style: .continuous
      )
    )
    .accessibilityElement(children: .combine)
  }

  private func evidenceCount(
    for run: LocalProductRunSummary,
    in snapshot: LocalProductSnapshot
  ) -> Int {
    snapshot.evidence.filter { $0.workItemID == run.workItemID }.count
  }

  private var workspacePresentation: MissionSnapshotPresentation {
    missionSnapshotPresentation(
      connection: store.connectionState,
      hasSnapshot: store.snapshot != nil
    )
  }

  @ViewBuilder
  private var workspaceReadBanner: some View {
    if let notice = workspacePresentation.notice {
      Label(notice, systemImage: "clock.arrow.circlepath")
        .font(.caption)
        .foregroundStyle(.orange)
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, 18)
        .padding(.vertical, 9)
        .background(Color.orange.opacity(0.08))
    }
  }

  private var providerManagement: some View {
    VStack(alignment: .leading, spacing: 0) {
      HStack {
        VStack(alignment: .leading, spacing: 3) {
          Text("Runtime & Providers")
            .font(.title2.weight(.semibold))
          Text("Execution clients and model connections")
            .foregroundStyle(.secondary)
        }
        Spacer()
        Button {
          Task { await store.refreshSetup() }
        } label: {
          Image(systemName: "arrow.clockwise")
        }
        .buttonStyle(.bordered)
        .accessibilityLabel("Refresh runtimes and providers")
        .help("Refresh runtimes and providers")
        .loomActionTarget()
        Button {
          showProviders = false
        } label: {
          Image(systemName: "xmark")
        }
        .buttonStyle(.plain)
        .help("Close")
        .accessibilityLabel("Close Runtime and Providers")
        .loomActionTarget()
      }
      .padding(.horizontal, 24)
      .padding(.vertical, 18)
      Divider()

      ScrollView {
        VStack(alignment: .leading, spacing: 20) {
          if store.setupSnapshot == nil {
            setupSnapshotRecoveryState
          } else {
            if let vault = store.setupSnapshot?.credentialVault {
              providerSectionHeader("CREDENTIAL VAULT")
              credentialVaultRow(vault)
            }

            providerSectionHeader("AGENT RUNTIMES")
            runtimeProviderRow

            providerSectionHeader("FAILURE ISOLATION")
            failureLabPanel

            if let candidates = store.setupSnapshot?.credentialImportCandidates,
              !candidates.isEmpty
            {
              providerSectionHeader("FOUND IN CC SWITCH")
              LazyVStack(spacing: 0) {
                ForEach(candidates) { candidate in
                  credentialImportRow(candidate)
                  if candidate.id != candidates.last?.id {
                    Divider().padding(.leading, 48)
                  }
                }
              }
            }

            providerSectionHeader("MODEL PROVIDERS")
            HStack(spacing: 10) {
              HStack(spacing: 8) {
                Image(systemName: "magnifyingglass")
                  .foregroundStyle(.secondary)
                TextField("Search providers", text: $providerSearchText)
                  .textFieldStyle(.plain)
              }
              .padding(.horizontal, 10)
              .frame(height: 36)
              .background(LoomGraphite.surface)
              .clipShape(RoundedRectangle(cornerRadius: 6))

              Picker("Category", selection: $providerCategory) {
                Text("All").tag("all")
                Text("Official").tag("official")
                Text("Cloud").tag("cloud")
                Text("Gateways").tag("gateway")
                Text("Local").tag("local")
                Text("Custom").tag("custom")
              }
              .labelsHidden()
              .pickerStyle(.menu)
              .frame(width: 132)
            }

            if store.setupSnapshot?.providers.isEmpty == true {
              setupSnapshotRecoveryState
            } else if filteredProviderDirectory.isEmpty {
              ContentUnavailableView(
                "No matching providers",
                systemImage: "magnifyingglass",
                description: Text("Clear the search or choose another category.")
              )
              .frame(maxWidth: .infinity, minHeight: 180)
            } else {
              LazyVStack(spacing: 0) {
                ForEach(filteredProviderDirectory) { provider in
                  providerDirectoryRow(provider)
                  if provider.id != filteredProviderDirectory.last?.id {
                    Divider().padding(.leading, 48)
                  }
                }
              }
            }
          }
        }
        .padding(24)
      }
    }
    .background(LoomGraphite.canvas)
    .task {
      await store.refreshSetup()
    }
    .sheet(item: $selectedProvider) { provider in
      ProviderCredentialSheet(store: store, provider: provider)
        .frame(minWidth: 520, minHeight: 500)
    }
    .sheet(item: $pendingEndpointReview) { candidate in
      EndpointReviewConfirmationSheet(candidate: candidate) {
        Task {
          await store.approveEndpointCandidate(candidate)
        }
      }
    }
  }

  private var setupSnapshotRecoveryState: some View {
    let presentation = missionSetupRecoveryPresentation(store.setupState)
    return VStack(spacing: 12) {
      Image(
        systemName: presentation.isLoading
          ? "arrow.triangle.2.circlepath" : "wrench.and.screwdriver"
      )
      .font(.system(size: 28))
      .foregroundStyle(LoomGraphite.accent)
      if presentation.isLoading {
        ProgressView()
          .controlSize(.small)
          .accessibilityLabel("Loading Runtime and Providers setup")
      }
      Text(presentation.title)
        .font(.headline)
      Text(presentation.detail)
        .font(.subheadline)
        .foregroundStyle(.secondary)
        .multilineTextAlignment(.center)
        .fixedSize(horizontal: false, vertical: true)
      if !presentation.isLoading {
        Button {
          Task { await store.refreshSetup() }
        } label: {
          Label("Try Again", systemImage: "arrow.clockwise")
        }
        .buttonStyle(.borderedProminent)
        .accessibilityLabel("Try Runtime and Providers setup again")
        .help("Reload Runtime & Providers from the local setup service")
      }
    }
    .frame(maxWidth: .infinity, minHeight: 260)
    .padding(24)
  }

  private func credentialVaultRow(
    _ vault: LocalProductCredentialVaultStatus
  ) -> some View {
    VStack(alignment: .leading, spacing: 4) {
      HStack(spacing: 12) {
        Image(
          systemName: vault.status == "unlocked"
            ? "lock.open.fill" : "lock.fill"
        )
        .foregroundStyle(providerStatusColor(vault.status))
        .frame(width: 32, height: 32)
        VStack(alignment: .leading, spacing: 2) {
          Text("Loom Credential Vault").font(.headline)
          Text(credentialVaultDetail(vault))
            .font(.caption)
            .foregroundStyle(.secondary)
        }
        Spacer()
        providerStatusLabel(vault.status)
        if vault.status == "recovery_required" {
          Button(role: .destructive) {
            confirmVaultRecoveryReset = true
          } label: {
            if store.isResettingCredentialVault {
              ProgressView().controlSize(.small)
            } else {
              Label("Reset Vault", systemImage: "trash")
            }
          }
          .buttonStyle(.bordered)
          .disabled(store.isResettingCredentialVault)
          .help("Crypto-erase the unusable Vault and re-enter Provider Account keys")
          .loomActionTarget()
        } else {
          Button {
            Task {
              await store.setCredentialVaultLocked(
                vault.status == "unlocked"
              )
            }
          } label: {
            if store.isUpdatingCredentialVaultLock {
              ProgressView().controlSize(.small)
            } else {
              Image(
                systemName: vault.status == "locked"
                  ? "lock.open" : "lock"
              )
            }
          }
          .buttonStyle(.bordered)
          .accessibilityLabel(vault.status == "locked" ? "Unlock Vault" : "Lock Vault")
          .disabled(
            !["unlocked", "locked"].contains(vault.status) || store.isUpdatingCredentialVaultLock
              || store.isRotatingCredentialVault || store.isResettingCredentialVault
          )
          .help(vault.status == "locked" ? "Unlock the Vault" : "Lock the Vault now")
          .loomActionTarget()
          Button {
            Task { await store.rotateCredentialVault() }
          } label: {
            if store.isRotatingCredentialVault {
              ProgressView().controlSize(.small)
            } else {
              Label("Rotate key", systemImage: "arrow.triangle.2.circlepath")
            }
          }
          .buttonStyle(.bordered)
          .disabled(
            vault.status != "unlocked" || store.isRotatingCredentialVault
              || store.isUpdatingCredentialVaultLock || store.isResettingCredentialVault
          )
          .help("Rotate the Vault wrapping key")
          .loomActionTarget()
          Button {
            showVaultExport = true
          } label: {
            Image(systemName: "square.and.arrow.up")
          }
          .buttonStyle(.bordered)
          .accessibilityLabel("Export encrypted Vault backup")
          .disabled(
            vault.status != "unlocked" || store.isExportingCredentialVault
              || store.isRotatingCredentialVault || store.isUpdatingCredentialVaultLock
          )
          .help("Export a passphrase-protected Credential Vault backup")
          .loomActionTarget()
        }
        Button {
          prepareVaultDiagnosticPreview()
        } label: {
          if preparingVaultDiagnostics {
            ProgressView().controlSize(.small)
          } else {
            Label("View diagnostics", systemImage: "stethoscope")
          }
        }
        .buttonStyle(.bordered)
        .disabled(
          preparingVaultDiagnostics || store.isResettingCredentialVault
            || store.isExportingCredentialVault
        )
        .help("Preview privacy-safe Vault diagnostics")
        .loomActionTarget()
      }
      if let vaultDiagnosticError {
        Text(vaultDiagnosticError)
          .font(.caption)
          .foregroundStyle(LoomGraphite.statusDanger)
          .padding(.leading, 44)
      }
      if let detail = store.credentialVaultOperationDetail {
        Text(detail)
          .font(.caption)
          .foregroundStyle(
            store.credentialVaultOperationFailed
              ? LoomGraphite.statusDanger : LoomGraphite.statusSuccess
          )
          .padding(.leading, 44)
          .textSelection(.enabled)
      }
    }
    .padding(.vertical, 8)
  }

  private func credentialVaultDetail(
    _ vault: LocalProductCredentialVaultStatus
  ) -> String {
    let mode = vault.storageMode
      .replacingOccurrences(of: "_", with: " ")
      .capitalized
    if vault.status == "locked" {
      return "\(mode) · Unlock required"
    }
    if vault.recoveryRequiredAccounts > 0 {
      let count = vault.recoveryRequiredAccounts
      let action = count == 1 ? "account needs" : "accounts need"
      return "\(mode) · \(count) \(action) recovery"
    }
    if vault.status == "recovery_required" {
      return "\(mode) · Vault recovery required"
    }
    if vault.migrationRequiredAccounts > 0 {
      let count = vault.migrationRequiredAccounts
      let action = count == 1 ? "account needs" : "accounts need"
      return "\(mode) · \(count) \(action) migration"
    }
    if vault.status == "migration_required" {
      return "\(mode) · Credential migration required"
    }
    return "\(mode) · Encrypted local storage"
  }

  private func prepareVaultDiagnosticPreview() {
    guard !preparingVaultDiagnostics else { return }
    preparingVaultDiagnostics = true
    vaultDiagnosticError = nil
    let input = LocalDiagnosticBundleInput(
      setupSnapshot: store.setupSnapshot,
      board: store.timeline?.board
    )
    Task {
      do {
        let exporter = try LocalDiagnosticBundleExporter.installed()
        let preview = try await Task.detached {
          try exporter.preview(input: input)
        }.value
        vaultDiagnosticExporter = exporter
        vaultDiagnosticPreview = preview
      } catch {
        vaultDiagnosticError = "Diagnostics could not be prepared."
      }
      preparingVaultDiagnostics = false
    }
  }

  private var runtimeProviderRow: some View {
    VStack(alignment: .leading, spacing: 4) {
      HStack(spacing: 12) {
        Image(systemName: "terminal")
          .foregroundStyle(LoomGraphite.accent)
          .frame(width: 32, height: 32)
          .background(LoomGraphite.accentMuted)
          .clipShape(RoundedRectangle(cornerRadius: 6))
        VStack(alignment: .leading, spacing: 2) {
          Text("Codex").font(.headline)
          Text("Native login · read-only conversation")
            .font(.caption)
            .foregroundStyle(.secondary)
        }
        Spacer()
        providerStatusLabel(
          store.setupSnapshot?.codex.status ?? "unavailable"
        )
        Button {
          Task { await store.connectCodex() }
        } label: {
          Label(
            store.setupSnapshot?.codex.status == "available"
              ? "Reconnect" : "Connect",
            systemImage: "arrow.triangle.2.circlepath"
          )
        }
        .buttonStyle(.bordered)
        .loomActionTarget()
      }
      if let runtimes = store.setupSnapshot?.runtimes, !runtimes.isEmpty {
        ForEach(runtimes) { runtime in
          setupRuntimeRow(runtime)
        }
      }
    }
    .padding(.vertical, 8)
  }

  private func setupRuntimeRow(
    _ runtime: LocalProductSetupRuntime
  ) -> some View {
    HStack(spacing: 12) {
      Image(systemName: "cpu")
        .foregroundStyle(providerStatusColor(runtime.status))
        .frame(width: 32, height: 32)
        .background(providerStatusColor(runtime.status).opacity(0.10))
        .clipShape(RoundedRectangle(cornerRadius: 6))
      VStack(alignment: .leading, spacing: 2) {
        Text(runtime.displayName)
          .font(.body.weight(.medium))
        Text(
          runtimeAgentAvailabilityDetail(
            adapterType: runtime.adapterType,
            modelIDs: runtime.modelIDs,
            roleOptionCount: store.setupSnapshot?.roleOptions.filter {
              $0.runtimeInstanceID == runtime.runtimeInstanceID
            }.count ?? 0
          )
        )
        .font(.caption)
        .foregroundStyle(.secondary)
        .lineLimit(1)
      }
      Spacer()
      providerStatusLabel(runtime.status)
    }
    .padding(.leading, 44)
    .padding(.vertical, 5)
  }

  private func providerSectionHeader(_ title: String) -> some View {
    Text(title)
      .font(.caption.weight(.semibold))
      .foregroundStyle(.secondary)
  }

  private func credentialImportRow(
    _ candidate: LocalProductCredentialImportCandidate
  ) -> some View {
    let accountID = candidate.targetProviderID + ".primary"
    let connected = credentialImportAccountConnected(candidate)
    let inFlight = store.providersInFlight.contains(accountID)
    return HStack(spacing: 12) {
      Image(systemName: "tray.and.arrow.down")
        .foregroundStyle(
          candidate.importMode == "exact_provider"
            ? LoomGraphite.accent : LoomGraphite.statusWarning
        )
        .frame(width: 32, height: 32)
        .background(
          (candidate.importMode == "exact_provider"
            ? LoomGraphite.accent : LoomGraphite.statusWarning).opacity(0.10)
        )
        .clipShape(RoundedRectangle(cornerRadius: 6))
      VStack(alignment: .leading, spacing: 3) {
        HStack(spacing: 7) {
          Text(candidate.displayName)
            .font(.body.weight(.medium))
          if candidate.current {
            Text("Current")
              .font(.caption2.weight(.semibold))
              .foregroundStyle(.secondary)
          }
        }
        Text(credentialImportDetail(candidate))
          .font(.caption)
          .foregroundStyle(.secondary)
          .lineLimit(2)
        if let detail = store.providerOperationDetail[accountID] {
          Text(detail)
            .font(.caption)
            .foregroundStyle(
              store.providerOperationStatus[accountID] == "Verified"
                ? LoomGraphite.statusSuccess : LoomGraphite.statusDanger
            )
            .textSelection(.enabled)
        }
      }
      Spacer(minLength: 12)
      if connected {
        Label("Connected", systemImage: "checkmark.circle.fill")
          .font(.caption)
          .foregroundStyle(LoomGraphite.statusSuccess)
      } else if candidate.importMode == "custom_endpoint_review" {
        if store.approvedEndpointReview(
          candidateDigest: candidate.candidateDigest,
          providerAccountID: accountID
        ) != nil {
          Button {
            Task { await store.importCredentialCandidate(candidate) }
          } label: {
            Label("Import", systemImage: "tray.and.arrow.down")
          }
          .buttonStyle(.bordered)
          .help("Import this approved endpoint credential into the Loom Vault")
          .loomActionTarget()
        } else if inFlight {
          ProgressView()
            .controlSize(.small)
            .accessibilityLabel("Reviewing \(candidate.displayName)")
        } else {
          Button {
            pendingEndpointReview = candidate
          } label: {
            Label("Review", systemImage: "shield.lefthalf.filled")
          }
          .buttonStyle(.bordered)
          .accessibilityLabel(
            endpointReviewPresentation(candidate).reviewButtonAccessibilityLabel
          )
          .help("Review endpoint and credential egress risk")
          .loomActionTarget()
        }
      } else if inFlight {
        ProgressView()
          .controlSize(.small)
          .accessibilityLabel("Importing \(candidate.displayName)")
      } else {
        Button {
          pendingCredentialImport = candidate
        } label: {
          Label("Import", systemImage: "tray.and.arrow.down")
        }
        .buttonStyle(.bordered)
        .disabled(!candidate.credentialAvailable)
        .help("Import this credential into the Loom Vault")
        .loomActionTarget()
      }
    }
    .padding(.vertical, 10)
  }

  private var failureLabPanel: some View {
    VStack(alignment: .leading, spacing: 12) {
      HStack(alignment: .center, spacing: 12) {
        Image(systemName: "waveform.path.ecg.rectangle")
          .foregroundStyle(LoomGraphite.accent)
          .frame(width: 32, height: 32)
        VStack(alignment: .leading, spacing: 3) {
          Text("Provider Failure Lab")
            .font(.body.weight(.medium))
          Text("Runs six local, credential-free failure cells against isolated temporary accounts.")
            .font(.caption)
            .foregroundStyle(.secondary)
            .fixedSize(horizontal: false, vertical: true)
        }
        Spacer(minLength: 12)
        if store.failureLabInFlight {
          ProgressView()
            .controlSize(.small)
            .accessibilityLabel("Running Provider failure isolation suite")
        } else {
          Button {
            Task { await store.runProviderFailureLabSuite() }
          } label: {
            Label("Run 6 checks", systemImage: "play.fill")
          }
          .buttonStyle(.bordered)
          .help("Run the local Provider failure isolation suite")
          .loomActionTarget()
        }
      }
      if let failure = store.failureLabError {
        Label(failure, systemImage: "exclamationmark.triangle.fill")
          .font(.caption)
          .foregroundStyle(LoomGraphite.statusDanger)
          .textSelection(.enabled)
      }
      ForEach(store.failureLabResults) { result in
        let target = result.agents[0]
        let peer = result.agents[1]
        HStack(alignment: .top, spacing: 10) {
          Image(systemName: "checkmark.circle.fill")
            .foregroundStyle(LoomGraphite.statusSuccess)
          VStack(alignment: .leading, spacing: 3) {
            Text(humanStatus(result.scenario))
              .font(.caption.weight(.semibold))
            Text("\(target.stage ?? "unknown") · \(target.code ?? "unknown") · healthy peer \(peer.status)")
              .font(.caption)
              .foregroundStyle(.secondary)
            Text("Incident \(result.incidentID)")
              .font(.caption2.monospaced())
              .foregroundStyle(.secondary)
              .textSelection(.enabled)
            Text(failureLabRecoveryAction(result.scenario))
              .font(.caption2)
              .foregroundStyle(.secondary)
          }
          Spacer(minLength: 8)
          VStack(alignment: .trailing, spacing: 6) {
            Text(target.retryable ? "Retryable" : "Contained")
              .font(.caption2.weight(.semibold))
              .foregroundStyle(
                target.retryable ? LoomGraphite.statusWarning : LoomGraphite.statusSuccess
              )
            HStack(spacing: 8) {
              if target.retryable {
                Button {
                  Task { await store.runProviderFailureLabScenario(result.scenario) }
                } label: {
                  Image(systemName: "arrow.clockwise")
                }
                .buttonStyle(.plain)
                .disabled(store.failureLabInFlight)
                .help("Retry this check")
                .accessibilityLabel("Retry \(humanStatus(result.scenario)) check")
              }
              Button {
                prepareVaultDiagnosticPreview()
              } label: {
                Image(systemName: "stethoscope")
              }
              .buttonStyle(.plain)
              .disabled(preparingVaultDiagnostics)
              .help("View diagnostics")
              .accessibilityLabel("View diagnostics for \(humanStatus(result.scenario))")
              Button {
                NSPasteboard.general.clearContents()
                NSPasteboard.general.setString(result.incidentID, forType: .string)
              } label: {
                Image(systemName: "doc.on.doc")
              }
              .buttonStyle(.plain)
              .help("Copy incident ID")
              .accessibilityLabel("Copy incident ID")
            }
          }
        }
        .accessibilityElement(children: .combine)
      }
    }
    .padding(.vertical, 8)
  }

  private func failureLabRecoveryAction(_ scenario: String) -> String {
    switch scenario {
    case "auth": return "Recovery: review the account credential."
    case "rate_limit": return "Recovery: wait for the account window, then retry."
    case "timeout": return "Recovery: check connectivity, then retry."
    case "insufficient_balance": return "Recovery: review the account balance or budget."
    case "corrupt_vault_record": return "Recovery: re-enter this account credential."
    case "revision_conflict": return "Recovery: refresh and freeze a new binding."
    default: return "Recovery: inspect the incident diagnostics."
    }
  }

  private func credentialImportAccountConnected(
    _ candidate: LocalProductCredentialImportCandidate
  ) -> Bool {
    let accountID = candidate.targetProviderID + ".primary"
    if (store.setupSnapshot?.providerAccounts ?? []).contains(where: {
      $0.providerID == candidate.targetProviderID &&
        $0.providerAccountID == accountID &&
        !$0.credentialReference.isEmpty && $0.revision > 0
    }) {
      return true
    }
    return (store.setupSnapshot?.providers ?? []).contains(where: {
      $0.providerID == candidate.targetProviderID &&
        !$0.credentialReference.isEmpty && $0.revision > 0
    })
  }

  private func credentialImportDetail(
    _ candidate: LocalProductCredentialImportCandidate
  ) -> String {
    let model = candidate.modelIDs.first ?? "No model"
    let extra = candidate.modelIDs.count > 1
      ? " +\(candidate.modelIDs.count - 1)" : ""
    if candidate.importMode == "custom_endpoint_review" {
      return "\(candidate.endpoint) · \(model)\(extra) · Credential egress is not approved"
    }
    return "\(candidate.targetProviderID) · \(candidate.endpoint) · \(model)\(extra)"
  }

  private var filteredProviderDirectory: [LocalProductProviderDirectoryEntry] {
    let query = providerSearchText.trimmingCharacters(
      in: .whitespacesAndNewlines
    ).lowercased()
    return (store.setupSnapshot?.providers ?? []).filter { provider in
      let categoryMatches = providerCategory == "all" || provider.category == providerCategory
      let searchMatches =
        query.isEmpty || provider.displayName.lowercased().contains(query)
        || provider.providerID.lowercased().contains(query)
        || provider.protocolFamily.lowercased().contains(query)
      return categoryMatches && searchMatches
    }
  }

  private func providerDirectoryRow(
    _ provider: LocalProductProviderDirectoryEntry
  ) -> some View {
    HStack(spacing: 12) {
      Image(systemName: providerSymbol(provider))
        .foregroundStyle(providerSymbolColor(provider))
        .frame(width: 32, height: 32)
        .background(providerSymbolColor(provider).opacity(0.10))
        .clipShape(RoundedRectangle(cornerRadius: 6))
      VStack(alignment: .leading, spacing: 2) {
        Text(provider.displayName)
          .font(.body.weight(.medium))
        Text(providerDirectoryDetail(provider))
          .font(.caption)
          .foregroundStyle(.secondary)
          .lineLimit(1)
      }
      Spacer(minLength: 12)
      providerStatusLabel(providerDisplayStatus(provider))
      if provider.connectionKind == "api_key" {
        Button {
          selectedProvider = provider
        } label: {
          Image(
            systemName: providerAccountCount(provider) == 0
              ? "plus" : "ellipsis")
        }
        .buttonStyle(.bordered)
        .help(
          providerAccountCount(provider) == 0
            ? "Connect \(provider.displayName)"
            : "Manage \(provider.displayName)"
        )
        .accessibilityLabel(
          providerAccountCount(provider) == 0
            ? "Connect \(provider.displayName)"
            : "Manage \(provider.displayName)"
        )
        .loomActionTarget()
      }
    }
    .padding(.vertical, 11)
    .contentShape(Rectangle())
  }

  private func providerDirectoryDetail(
    _ provider: LocalProductProviderDirectoryEntry
  ) -> String {
    let protocolName = providerProtocolLabel(provider)
    let count = providerAccountCount(provider)
    guard count > 0 else { return protocolName }
    return "\(protocolName) · \(count) \(count == 1 ? "account" : "accounts")"
  }

  private func providerAccountCount(
    _ provider: LocalProductProviderDirectoryEntry
  ) -> Int {
    let primaryAccountID = provider.providerID + ".primary"
    var accountIDs = Set(
      (store.setupSnapshot?.providerAccounts ?? []).compactMap {
        $0.providerID == provider.providerID ? $0.providerAccountID : nil
      })
    if !provider.credentialReference.isEmpty {
      accountIDs.insert(primaryAccountID)
    }
    return accountIDs.count
  }

  private func providerDisplayStatus(
    _ provider: LocalProductProviderDirectoryEntry
  ) -> String {
    store.providerOperationStatus[provider.providerID] ?? provider.status
  }

  private func providerStatusLabel(_ status: String) -> some View {
    HStack(spacing: 6) {
      Circle()
        .fill(providerStatusColor(status))
        .frame(width: 7, height: 7)
      Text(humanStatus(status))
        .font(.caption)
        .foregroundStyle(.secondary)
    }
    .accessibilityElement(children: .combine)
  }

  private func providerStatusColor(_ status: String) -> Color {
    switch status.lowercased() {
    case "available", "verified", "unlocked":
      return LoomGraphite.statusSuccess
    case "connecting", "testing", "replacing", "removing", "configured",
      "migration_required", "locked":
      return LoomGraphite.statusWarning
    case "rejected", "unavailable", "recovery_required":
      return LoomGraphite.statusDanger
    default: return Color.secondary.opacity(0.65)
    }
  }

  private func providerSymbol(
    _ provider: LocalProductProviderDirectoryEntry
  ) -> String {
    switch provider.category {
    case "cloud": return "cloud"
    case "gateway": return "arrow.triangle.branch"
    case "local": return "desktopcomputer"
    case "custom": return "slider.horizontal.3"
    default: return "sparkles"
    }
  }

  private func providerSymbolColor(
    _ provider: LocalProductProviderDirectoryEntry
  ) -> Color {
    switch provider.category {
    case "cloud": return .blue
    case "gateway": return .teal
    case "local": return .green
    case "custom": return .orange
    default: return LoomGraphite.accent
    }
  }

  private func providerProtocolLabel(
    _ provider: LocalProductProviderDirectoryEntry
  ) -> String {
    let protocolName: String
    switch provider.protocolFamily {
    case "openai_responses": protocolName = "Responses API"
    case "openai_compatible": protocolName = "OpenAI-compatible"
    case "anthropic_messages": protocolName = "Messages API"
    case "gemini_generate_content": protocolName = "Gemini API"
    case "bedrock_converse": protocolName = "Bedrock Converse"
    case "vertex_generate_content": protocolName = "Vertex AI"
    case "ollama": protocolName = "Ollama API"
    default: protocolName = humanStatus(provider.protocolFamily)
    }
    return "\(protocolName) · \(humanStatus(provider.authMode))"
  }

  private func missionRecord(
    _ id: String
  ) -> LocalProductMissionSummary? {
    store.snapshot?.missions.first(where: { $0.missionID == id })
  }

  private var missionFilter: Binding<String> {
    Binding(
      get: { store.workbench.boardFilter },
      set: { store.updateMissionBoardFilter($0) }
    )
  }

  private var connectionLabel: String {
    switch store.connectionState {
    case .loading: return "Starting local service"
    case .online: return "Local service ready"
    case .partial: return "Some information unavailable"
    case .stale: return "Showing last loaded state"
    case .offline: return "Local service unavailable"
    case .fatal: return "Local service needs attention"
    }
  }

  private var authorityBanner: String? {
    switch store.connectionState {
    case .online:
      return nil
    case .loading:
      return "Starting the Loom service on this Mac"
    case .partial:
      return "Some information could not be loaded safely"
    case .stale:
      return "Showing last loaded state until the service reconnects"
    case .offline:
      return "Local service unavailable · try again from the workspace header"
    case .fatal:
      return "Local service rejected the connection · check local setup"
    }
  }

  private var emptyBoardDetail: String {
    let teamCount = store.snapshot?.teams.count ?? 0
    let runCount = store.snapshot?.runs.count ?? 0
    if teamCount > 0 || runCount > 0 {
      return
        "No Mission execution exists. \(teamCount) Team(s) and \(runCount) historical Run(s) remain available."
    }
    return "Create a Mission, choose a confirmed Team, then review the plan before execution."
  }

  private var connectionColor: Color {
    switch store.connectionState {
    case .online: return .green
    case .partial, .stale, .loading: return .orange
    case .offline, .fatal: return .red
    }
  }

  private func humanStatus(_ value: String) -> String {
    missionHumanStatus(value)
  }

  private func decisionCTA(_ kind: LocalProductDecisionKind) -> String {
    switch kind {
    case .authorization: return "Open Authorization"
    case .review: return "Open Review Gate"
    case .recovery: return "Open Recovery Decision"
    case .fallback: return "Review Fallback Route"
    }
  }
}

public struct LoomDecisionSheet: View {
  public let sheet: LocalProductDecisionSheet
  public let onAction: (String) -> Void
  public let onOpenEvidence: () -> Void
  public let onClose: () -> Void

  @State private var technicalDetailsExpanded = false

  public init(
    sheet: LocalProductDecisionSheet,
    onAction: @escaping (String) -> Void,
    onOpenEvidence: @escaping () -> Void,
    onClose: @escaping () -> Void
  ) {
    self.sheet = sheet
    self.onAction = onAction
    self.onOpenEvidence = onOpenEvidence
    self.onClose = onClose
  }

  public var body: some View {
    VStack(alignment: .leading, spacing: 0) {
      HStack(alignment: .top, spacing: 14) {
        Image(systemName: symbol)
          .font(.title2)
          .foregroundStyle(LoomGraphite.accent)
          .frame(width: 34, height: 34)
          .accessibilityHidden(true)
        VStack(alignment: .leading, spacing: 4) {
          Text(sheet.title)
            .font(.title2.weight(.semibold))
          Text(sheet.summary)
            .foregroundStyle(.secondary)
        }
        Spacer()
        Button {
          onAction("not_now")
          onClose()
        } label: {
          Image(systemName: "xmark")
        }
        .buttonStyle(.plain)
        .loomActionTarget()
        .accessibilityLabel("Not now")
        .keyboardShortcut(.cancelAction)
      }
      .padding(24)

      Divider()

      ScrollView {
        VStack(alignment: .leading, spacing: 18) {
          identitySection
          templateSection
          DisclosureGroup(
            "Technical details",
            isExpanded: $technicalDetailsExpanded
          ) {
            VStack(alignment: .leading, spacing: 7) {
              ForEach(sheet.technicalDetails, id: \.self) {
                Text($0)
                  .font(.caption.monospaced())
                  .textSelection(.enabled)
              }
            }
            .padding(.top, 8)
          }
          .accessibilityLabel("Technical details")
        }
        .padding(24)
      }

      Divider()
      actionBar
        .padding(18)
    }
    .background(LoomGraphite.canvas)
    .accessibilityElement(children: .contain)
    .accessibilityLabel("\(kindLabel) decision sheet")
  }

  private var identitySection: some View {
    VStack(alignment: .leading, spacing: 10) {
      decisionRow("Mission", "Current Mission")
      decisionRow("Requested by", sheet.requester)
      decisionRow("Target", sheet.target)
      decisionRow("Attempt", sheet.attemptScope)
    }
    .padding(14)
    .background(
      LoomGraphite.surface,
      in: RoundedRectangle(cornerRadius: LoomGraphite.cardRadius)
    )
  }

  @ViewBuilder
  private var templateSection: some View {
    switch sheet.kind {
    case .authorization:
      decisionGrid([
        ("Requested action", sheet.summary),
        ("Command type", sheet.commandType),
        ("Network access", sheet.networkAccess),
        ("Credential access", sheet.credentialAccess),
        ("Permission scope", sheet.permissionScope),
        ("Expected Evidence", sheet.expectedEvidence),
      ])
    case .review:
      VStack(alignment: .leading, spacing: 10) {
        Text("Verification and Evidence")
          .font(.headline)
        decisionGrid([
          ("Change summary", sheet.summary),
          ("Tests and verification", sheet.commandType),
          ("Reviewer result", sheet.networkAccess),
          ("Required Evidence", sheet.expectedEvidence),
        ])
        Button("Open Evidence", action: onOpenEvidence)
          .buttonStyle(.bordered)
          .loomActionTarget()
          .disabled(sheet.expectedEvidence.isEmpty)
      }
    case .recovery:
      VStack(alignment: .leading, spacing: 10) {
        Text("Recovery boundary")
          .font(.headline)
        decisionGrid([
          ("Stopped because", sheet.summary),
          ("Retained output and Evidence", sheet.expectedEvidence),
          ("Fresh Attempt", sheet.attemptScope),
          ("Team / Provider changes", sheet.target),
          ("Permission / budget", sheet.permissionScope),
        ])
        Text(
          "No hidden retry. A new attempt is confirmed only after a fresh Attempt and claim generation appear in the authoritative Projection."
        )
        .font(.caption)
        .foregroundStyle(.secondary)
      }
    case .fallback:
      VStack(alignment: .leading, spacing: 10) {
        Text("Fallback binding transition")
          .font(.headline)
        decisionGrid([
          ("Agent", sheet.target),
          ("Binding change", sheet.commandType),
          ("Provider access", sheet.credentialAccess),
          ("Scope", sheet.permissionScope),
          ("Attempt", sheet.attemptScope),
        ])
        Text("Loom will not change Provider, account, model, or Harness unless this exact versioned route is approved.")
          .font(.caption)
          .foregroundStyle(.secondary)
      }
    }
  }

  private func decisionGrid(
    _ rows: [(String, String)]
  ) -> some View {
    VStack(alignment: .leading, spacing: 10) {
      ForEach(Array(rows.enumerated()), id: \.offset) { _, row in
        decisionRow(row.0, row.1)
      }
    }
    .padding(14)
    .background(
      LoomGraphite.surface,
      in: RoundedRectangle(cornerRadius: LoomGraphite.cardRadius)
    )
  }

  private func decisionRow(_ label: String, _ value: String) -> some View {
    HStack(alignment: .firstTextBaseline, spacing: 14) {
      Text(label)
        .font(.caption.weight(.semibold))
        .foregroundStyle(.secondary)
        .frame(width: 138, alignment: .leading)
      Text(value)
        .frame(maxWidth: .infinity, alignment: .leading)
        .textSelection(.enabled)
    }
    .accessibilityElement(children: .combine)
  }

  private var actionBar: some View {
    HStack(spacing: 10) {
      ForEach(Self.visibleActions(sheet.actions), id: \.self) { action in
        decisionButton(action)
      }
      Spacer(minLength: 0)
    }
  }

  static func visibleActions(_ actions: [String]) -> [String] {
    actions.filter { $0 != "edit_scope" }
  }

  @ViewBuilder
  private func decisionButton(_ action: String) -> some View {
    if primaryAction(action) {
      decisionButtonBase(action)
        .buttonStyle(.borderedProminent)
    } else {
      decisionButtonBase(action)
        .buttonStyle(.bordered)
    }
  }

  private func decisionButtonBase(_ action: String) -> some View {
    Button(actionLabel(action)) {
      onAction(action)
      if action == "not_now" || action == "edit_scope" {
        onClose()
      }
    }
    .loomActionTarget()
    .disabled(
      requiresPrepared(action) && (!sheet.prepared || !sheet.preparedActions.contains(action))
    )
    .accessibilityLabel(actionLabel(action))
  }

  private func requiresPrepared(_ action: String) -> Bool {
    !["not_now", "edit_scope"].contains(action)
  }

  private func primaryAction(_ action: String) -> Bool {
    [
      "allow_once", "allow_for_mission", "accept_result",
      "start_new_attempt", "approve_fallback",
    ].contains(action)
  }

  private func actionLabel(_ action: String) -> String {
    switch action {
    case "not_now": return "Not now"
    case "deny": return "Deny"
    case "edit_scope":
      return sheet.kind == .recovery ? "Edit recovery" : "Edit scope"
    case "allow_once": return "Allow once"
    case "allow_for_mission": return "Allow for this Mission"
    case "request_changes": return "Request changes"
    case "accept_result": return "Accept result"
    case "stop_mission": return "Stop Mission"
    case "start_new_attempt": return "Start new attempt"
    case "approve_fallback": return "Approve fallback"
    case "reject_fallback": return "Reject fallback"
    default: return action.replacingOccurrences(of: "_", with: " ").capitalized
    }
  }

  private var symbol: String {
    switch sheet.kind {
    case .authorization: return "lock.shield"
    case .review: return "checkmark.seal"
    case .recovery: return "arrow.triangle.2.circlepath"
    case .fallback: return "arrow.triangle.branch"
    }
  }

  private var kindLabel: String {
    switch sheet.kind {
    case .authorization: return "Authorization"
    case .review: return "Review Gate"
    case .recovery: return "Recovery"
    case .fallback: return "Fallback Route"
    }
  }
}

/// Maps raw side-task operation statuses (including daemon rejection codes)
/// to user-facing, actionable copy so a failed proposal/creation never shows a
/// cryptic error code. Friendly in-flight states pass through unchanged;
/// unknown codes keep the stable reason for diagnostics.
func sideTaskStatusLabelText(_ raw: String) -> String {
  switch raw {
  case "Idle", "Proposing", "Creating", "Confirmation required":
    return raw
  case "proposal_unavailable":
    return "A proposal is not available right now. Review the request and try again."
  case "proposal_required":
    return "Review the proposal before creating the side task."
  case "invalid_request", "invalid_response":
    return "Loom rejected the side task request. Review the fields and try again."
  case "state_unavailable", "unavailable":
    return "Side tasks are unavailable right now. Try again."
  case "conflict", "stale_view", "stale_generation":
    return "The workspace state changed. Review the request, then try again."
  case "timeout":
    return "The request timed out. Try again."
  case "capability_gap":
    return "This Agent Team cannot run that kind of side task yet."
  case "not_found":
    return "The side task or Team no longer exists. Refresh and try again."
  default:
    return "Side task could not proceed: \(raw)"
  }
}
