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
  let rows: [String]
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
    rows: [String],
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
    self.rows = rows
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
      timeline.teamInstanceID == record.teamInstanceID,
      timeline.gap == nil,
      !timeline.hasMore
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
          title,
          missionHumanStatus(node.status),
          node.currentAttempt > 0 ? "Attempt \(node.currentAttempt)" : "Not started",
        ]
        if let kind = missionNodeKindLabel(node) {
          details.append(kind)
        }
        if node.executionBindingAvailable {
          details.append(missionHumanStatus(node.harnessAdapter))
          details.append(
            "\(missionHumanStatus(node.providerID)) / \(node.providerAccountID)"
          )
          details.append(node.modelID)
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
        return details.joined(separator: " · ")
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
        "\($0.role.capitalized) · \(missionHumanStatus($0.state)) · Attempt \($0.attemptNumber)"
      }
      incidentIDs = rows.map { _ in nil }
      diagnosticActions = rows.map { _ in false }
      retryReviewActions = rows.map { _ in false }
      credentialVaultRecoveryActions = rows.map { _ in false }
      agentRecoveryActions = rows.map { _ in false }
      rowSystemImages = rows.map { _ in "person.crop.circle" }
      rowAccessibilityLabels = rows.map { _ in "Agent status" }
    }
    rows.append(
      contentsOf: providerAccounts.map { account in
        var details = [
          "\(missionHumanStatus(account.providerID)) / \(account.providerAccountID)",
          "\(account.failedAttempts)/\(account.attemptCount) failed",
          missionErrorRateLabel(basisPoints: account.errorRateBasisPoints),
          "\(account.rateLimitedAttempts) rate limited",
          "Accounting \(account.accountingAttemptCount)/\(account.attemptCount) attempts",
        ]
        if account.usageAttemptCount > 0 {
          details.append("\(account.totalTokens) tokens")
        }
        if account.policyAvailable {
          details.append("Policy r\(account.policyRevision)")
          details.append(
            "\(account.activeAttempts)/\(account.maximumConcurrentAttempts) active"
          )
          details.append(
            "\(account.maximumDispatchStarts) starts / \(account.dispatchWindowSeconds)s"
          )
          details.append(
            "Budget \(account.activeAssignedBudgetUnits)/\(account.maximumAssignedBudgetUnits)"
          )
        } else {
          details.append("\(account.activeAttempts) active")
        }
        if account.aggregationOverflow {
          details.append("Accounting incomplete")
        }
        details.append(
          contentsOf: account.costs.map {
				missionCostSourceLabel(
					source: $0.source,
					microunits: $0.amountMicrounits,
					currency: $0.currency
				)
          })
        return details.joined(separator: " · ")
      })
    incidentIDs.append(contentsOf: providerAccounts.map { _ in nil })
    diagnosticActions.append(contentsOf: providerAccounts.map { _ in false })
    retryReviewActions.append(contentsOf: providerAccounts.map { _ in false })
    credentialVaultRecoveryActions.append(
      contentsOf: providerAccounts.map { _ in false }
    )
    agentRecoveryActions.append(contentsOf: providerAccounts.map { _ in false })
    rowSystemImages.append(contentsOf: providerAccounts.map { _ in "server.rack" })
    rowAccessibilityLabels.append(
      contentsOf: providerAccounts.map { _ in "Provider Account governance" }
    )
    return MissionInspectorSection(
      heading: "Team Pulse",
      rows: rows,
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

public struct MissionWorkbench: View {
  @ObservedObject private var store: LocalProductStore
  @State private var showProviders = false
  @State private var providerSearchText = ""
  @State private var providerCategory = "all"
  @State private var selectedProvider: LocalProductProviderDirectoryEntry?
  @State private var vaultDiagnosticPreview: LocalDiagnosticBundlePreview?
  @State private var vaultDiagnosticExporter: LocalDiagnosticBundleExporter?
  @State private var vaultDiagnosticError: String?
  @State private var preparingVaultDiagnostics = false
  @State private var confirmVaultRecoveryReset = false
  @State private var showVaultExport = false
  @State private var showNewMission = false
  @State private var showNewSideTask = false
  @State private var newMissionObjective = ""
  @State private var confirmedMissionConstraints: [String] = []
  @State private var acceptedMissionDecisions: [String] = []
  @State private var missionContextConfirmed = false
  @State private var newMissionTeamID = ""
  @State private var newMissionWorkPackageID =
    LocalProductWorkPackageOption.coding.id
  @State private var sideTaskPurpose = "research"
  @State private var sideTaskMode = "report_only"
  @State private var sideTaskTitle = ""
  @State private var sideTaskRequest = ""
  @State private var agentInputDrafts: [String: String] = [:]
  @State private var agentInputModes: [String: LocalProductAgentInputMode] = [:]
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

  public init(
    store: LocalProductStore,
    showRail: Bool = true,
    showProvidersInitially: Bool = false,
    showNewMissionInitially: Bool = false,
    initialMissionObjective: String? = nil
  ) {
    self.store = store
    self.showRail = showRail
    self.initialMissionObjective = initialMissionObjective
    _showProviders = State(initialValue: showProvidersInitially)
    _showNewMission = State(initialValue: showNewMissionInitially)
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
        if newMissionTeamID.isEmpty {
          newMissionTeamID = executableTeams.first?.teamInstanceID ?? ""
        }
        showNewMission = true
      }
      railButton("My Missions", systemImage: "rectangle.3.group") {
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
    .accessibilityLabel("Mission navigation")
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
          Text("New Mission")
            .font(.title2.weight(.semibold))
          Text("A Mission is one bounded piece of work that an Agent Team carries out for you, with review before anything runs.")
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
                Image(systemName: "person.3.fill")
                  .font(.system(size: 26, weight: .regular))
                  .foregroundStyle(LoomGraphite.accent)
                  .accessibilityHidden(true)
                VStack(alignment: .leading, spacing: 6) {
                  Text("You need an Agent Team first")
                    .font(.headline)
                  Text(
                    "Missions run on Agent Teams. Create and confirm a Team, then come back here to start a Mission."
                  )
                  .foregroundStyle(.secondary)
                  .fixedSize(horizontal: false, vertical: true)
                }
              }
              HStack(spacing: 10) {
                Button {
                  showNewMission = false
                  store.showMissionTeams()
                  Task { await store.startBlankBuilder() }
                } label: {
                  Label("Create Agent Team", systemImage: "plus")
                }
                .buttonStyle(.borderedProminent)
                .accessibilityLabel("Create Agent Team")
                .help("Create Agent Team")

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
      // Pre-select the first executable Team so "Review preflight" is not
      // silently disabled for every entry path (rail and welcome "Start
      // Mission" both land here). The Team picker still lets the user change it.
      if newMissionTeamID.isEmpty {
        newMissionTeamID = executableTeams.first?.teamInstanceID ?? ""
      }
    }
    .onChange(of: newMissionObjective) { _, _ in missionContextDidChange() }
    .onChange(of: confirmedMissionConstraints) { _, _ in missionContextDidChange() }
    .onChange(of: acceptedMissionDecisions) { _, _ in missionContextDidChange() }
    .onChange(of: newMissionTeamID) { _, _ in missionContextDidChange() }
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
          Button("Start Mission") {
            Task {
              await store.startPreflightedMission()
              if store.executionState == .running
                || store.executionState == .awaitingRecovery
                || store.executionState == .succeeded
              {
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
                acceptedDecisions: acceptedMissionDecisions
              )
            }
          }
          .buttonStyle(.borderedProminent)
          .disabled(
            newMissionObjective.trimmingCharacters(
              in: .whitespacesAndNewlines
            ).isEmpty || newMissionTeamID.isEmpty || !store.missionExecutionReachable
              || !missionContextConfirmed || missionContextValidationMessage != nil
              || store.executionState == .preflighting || store.executionState == .starting
          )
          if newMissionObjective.trimmingCharacters(
            in: .whitespacesAndNewlines
          ).isEmpty || newMissionTeamID.isEmpty || !missionContextConfirmed
          {
            Text(missionReviewBlockedReason)
              .font(.caption2)
              .foregroundStyle(.secondary)
              .fixedSize(horizontal: false, vertical: true)
              .accessibilityLabel("Review preflight blocked. \(missionReviewBlockedReason)")
          }
          if missionFailureShowsMissionBoard {
            Button("Open Mission Board") {
              showNewMission = false
              store.showMissionBoard()
            }
            .buttonStyle(.bordered)
            .accessibilityLabel("Open Mission Board to inspect the running Mission")
          }
        }
    }
  }

  private var missionReviewBlockedReason: String {
    if newMissionObjective.trimmingCharacters(
      in: .whitespacesAndNewlines
    ).isEmpty {
      return "Enter a Mission objective first."
    }
    if newMissionTeamID.isEmpty {
      return "Select a Team to run this Mission."
    }
    if let message = missionContextValidationMessage {
      return message
    }
    return "Confirm the Mission context to enable preflight review."
  }

  private var missionContextCanConfirm: Bool {
    !newMissionObjective.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
      && missionContextValidationMessage == nil
  }

  private var missionContextValidationMessage: String? {
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
    switch reason {
    case "busy":
      return "This Team already has a running Mission. Open the Mission Board to follow it, or wait for it to finish."
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
      return "The Mission started, but Loom could not yet show it. Open the Mission Board in a moment."
    case "invalid_response":
      return "Loom received an invalid execution response. Retry, or open the Mission Board to inspect state."
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
  }

  private var orchestrationBoard: some View {
    VStack(spacing: 0) {
      workbenchToolbar(
        title: "Mission Board",
        subtitle: "Authoritative orchestration state"
      )
      HStack(spacing: 8) {
        Label("Board", systemImage: "rectangle.3.group")
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
        .help("Hide the Complete lane to focus on active Missions")
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
          icon: "rectangle.3.group",
          title: "No Missions yet",
          detail: emptyBoardDetail
        )
      } else {
        ScrollView(.horizontal, showsIndicators: true) {
          HStack(alignment: .top, spacing: 12) {
            ForEach(MissionLane.allCases, id: \.self) { lane in
              if !(store.workbench.boardHideCompleted && lane == .complete) {
                missionLane(lane)
              }
            }
          }
          .padding(16)
        }
        HStack(spacing: 6) {
          Image(systemName: "arrow.left.and.right")
          Text("Scroll horizontally to inspect all five lanes")
          Spacer()
          Text("5 lanes")
            .monospacedDigit()
        }
        .font(.caption2)
        .foregroundStyle(.secondary)
        .padding(.horizontal, 18)
        .frame(height: 26)
        .accessibilityLabel(
          "Mission Board has five horizontally scrollable lanes"
        )
      }
    }
    .background(LoomGraphite.canvas)
  }

  private func missionLane(_ lane: MissionLane) -> some View {
    let missions = store.workbench.missions.filter {
      $0.lane == lane
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
           mission.status == "blocked" {
          Label(
            missionHumanStatus(blockReason),
            systemImage: "exclamationmark.triangle"
          )
          .font(.caption2)
          .foregroundStyle(.orange)
          .lineLimit(2)
          .fixedSize(horizontal: false, vertical: true)
          .accessibilityLabel("Blocked because \(missionHumanStatus(blockReason))")
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
      "\(displayTitle), \(mission.lane.rawValue), \(humanStatus(mission.status))"
    )
  }

  private func missionRoom(_ id: String) -> some View {
    HSplitView {
      missionList
        .frame(minWidth: 190, idealWidth: 220, maxWidth: 260)
      missionCenter(id)
        .frame(minWidth: 390)
      if store.workbench.selectedContinuity.inspectorVisible {
        missionInspector(id)
          .frame(
            minWidth: 240,
            idealWidth: LoomGraphite.inspectorWidth,
            maxWidth: 340
          )
      }
    }
  }

  private var missionList: some View {
    VStack(alignment: .leading, spacing: 5) {
      Button {
        store.showMissionBoard()
      } label: {
        Label("Back to Board", systemImage: "chevron.left")
      }
      .buttonStyle(.plain)
      .loomActionTarget()
      .padding(.horizontal, 12)
      Divider()
      ScrollView {
        LazyVStack(spacing: 3) {
          ForEach(store.workbench.missions) { mission in
            let record = missionRecord(mission.id)
            Button {
              Task { await store.openMissionAndActivate(mission.id) }
            } label: {
              VStack(alignment: .leading, spacing: 3) {
                Text(
                  missionDisplayTitle(
                    candidate: record?.title ?? mission.title,
                    missionID: record?.missionID ?? mission.id,
                    teamInstanceID: record?.teamInstanceID ?? "",
                    teams: store.snapshot?.teams ?? []
                  )
                )
                .font(.callout.weight(.medium))
                .lineLimit(1)
                Text(mission.lane.rawValue)
                  .font(.caption)
                  .foregroundStyle(.secondary)
              }
              .padding(9)
              .frame(maxWidth: .infinity, alignment: .leading)
              .background(
                mission.id == store.workbench.selectedMissionID
                  ? LoomGraphite.accent.opacity(0.12)
                  : Color.clear,
                in: RoundedRectangle(cornerRadius: 8)
              )
            }
            .buttonStyle(.plain)
          }
        }
        .padding(8)
      }
    }
    .background(LoomGraphite.rail)
  }

  private func missionCenter(_ id: String) -> some View {
    let record = missionRecord(id)
    return VStack(spacing: 0) {
      workbenchToolbar(
        title: missionDisplayTitle(
          candidate: record?.title ?? "",
          missionID: record?.missionID ?? id,
          teamInstanceID: record?.teamInstanceID ?? "",
          teams: store.snapshot?.teams ?? []
        ),
        subtitle: record.map {
          "\($0.lane) · \(humanStatus($0.status))"
        } ?? "Unavailable"
      )
      ScrollView {
        LazyVStack(alignment: .leading, spacing: 14) {
          contentBlock(
            title: "Outcome",
            icon: "scope",
            text: record?.lastMilestone
              ?? "No authoritative Mission record is available."
          )
          contentBlock(
            title: "Plan",
            icon: "point.3.connected.trianglepath.dotted",
            text: record.map {
              "\($0.nodeCount) node(s), \($0.completedNodeCount) complete, \($0.reviewNodeCount) in review."
            } ?? "No plan is available."
          )
	          missionActivity(record)
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
      Divider()
      Text(section.heading)
        .font(.subheadline.weight(.semibold))
      if section.rows.isEmpty {
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
              .disabled(!store.canCreateSideTask(for: id))
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
    subtitle: String
  ) -> some View {
    HStack {
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
    let records = store.timeline?.records ?? []
    VStack(alignment: .leading, spacing: 10) {
      Label("Activity", systemImage: "waveform.path.ecg")
        .font(.subheadline.weight(.semibold))
      if let gap = store.timeline?.gap {
        Label(
          "Some live activity is unavailable · \(humanStatus(gap.reason))",
          systemImage: "exclamationmark.triangle"
        )
        .font(.caption)
        .foregroundStyle(.orange)
      }
      if records.isEmpty {
        Text(
          mission?.activeNodeCount == 0
            ? "No active authorized output."
            : "Authorized activity is in progress. Live output is tentative until Evidence is accepted."
        )
        .foregroundStyle(.secondary)
      } else {
        ForEach(records) { item in
          VStack(alignment: .leading, spacing: 4) {
            if !item.payload.textDelta.isEmpty {
              Label("Tentative output", systemImage: "text.bubble")
                .font(.caption.weight(.semibold))
                .foregroundStyle(.orange)
              Text(item.payload.textDelta)
                .textSelection(.enabled)
            } else {
              Text(
                humanStatus(
                  item.payload.status.isEmpty
                    ? item.kind
                    : item.payload.status
                )
              )
              .font(.callout.weight(.medium))
              if !item.payload.warningCode.isEmpty {
                Text(humanStatus(item.payload.warningCode))
                  .font(.caption)
                  .foregroundStyle(.orange)
              }
            }
          }
          .padding(.vertical, 3)
        }
      }
      Text("Tentative text is not authoritative until terminal Evidence is accepted.")
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
              ForEach(snapshot.teams) { team in
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
                        ? "Confirmed · Available for explicit Mission start"
                        : "Read-only · Not available for execution")
                      : "Preserved record · Refresh before execution"
                  )
                  .font(.callout)
                  .foregroundStyle(.secondary)
                  Text("Source · \(humanStatus(team.sourceKind))")
                    .font(.caption)
                    .foregroundStyle(.secondary)
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
          LazyVStack(alignment: .leading, spacing: 18) {
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
          if let vault = store.setupSnapshot?.credentialVault {
            providerSectionHeader("CREDENTIAL VAULT")
            credentialVaultRow(vault)
          }

          providerSectionHeader("AGENT RUNTIMES")
          runtimeProviderRow

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

          if filteredProviderDirectory.isEmpty {
            ContentUnavailableView(
              "No providers found",
              systemImage: "magnifyingglass"
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
        .padding(24)
      }
    }
    .background(LoomGraphite.canvas)
    .sheet(item: $selectedProvider) { provider in
      ProviderCredentialSheet(store: store, provider: provider)
        .frame(minWidth: 520, minHeight: 500)
    }
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
    .padding(.vertical, 8)
  }

  private func providerSectionHeader(_ title: String) -> some View {
    Text(title)
      .font(.caption.weight(.semibold))
      .foregroundStyle(.secondary)
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
