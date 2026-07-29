# P2A-W2 Provider Connection and Delegated OAuth RED

**Date**: 2026-07-30
**Status**: PRESERVED

Command:

```text
go test ./internal/provider
```

The package failed to compile only because the frozen delegated OAuth symbols
were absent:

```text
undefined: NewSystemCodexLoginController
undefined: CodexLoginControllerConfig
undefined: CodexLoginStarted
undefined: ErrCodexLoginBusy
```

The new regression also freezes the observed Codex 0.144.1 stderr-only success
line and ambiguous dual-channel rejection. No real Codex process, OAuth,
browser, network, Keychain, installed credential, daemon, or GUI action
occurred.

Application and IPC RED commands:

```text
go test ./internal/app
go test ./internal/api
go test ./internal/localipc
go test ./cmd/loomd
```

They failed only on the absent connection seam, application delegation,
`codex_connect` method allowlist, and exact daemon handler result. The existing
read, Team Builder, credential, and authority tests remained unchanged.

Swift model/store RED:

```text
cd apps/macos
swift test
```

The build failed only because these frozen symbols were absent:

```text
LocalProductProviderConnectResult
LocalProductSetupWire.decodeProviderConnectResult
LocalProductSetupClientProtocol.connectCodex
LocalProductStore.connectCodex
LocalProductStore.providerConnectionStatus
```

Swift product-surface RED:

```text
swift test --filter \
  LocalProductExperienceViewTests.testProviderDirectoryUsesConnectThenCapabilitySpecificManagement
```

It failed only because the capability-specific `Connect`/`Manage` presentation
contract was absent. All REDs were deterministic fixtures. They did not invoke
the installed Codex CLI, open OAuth, inspect or mutate credentials, contact a
Provider, or activate the resident daemon.
