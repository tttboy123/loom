# P2A-W2 Mandatory RED

**Date**: 2026-07-29
**Status**: PASS — genuine behavioral RED preserved
**Frozen contract SHA-256**:
`f2e7a4d27866da4ca5f08623db6d1082587f8fd4e986f3c39f1d839dd0609a55`

## Scope

RED was added before product implementation across the exact frozen W2 owned
test surfaces:

- Credential Broker lifecycle, rollback, closed errors, and mutable-buffer
  clearing;
- Codex native-auth status-only observation and executable identity failure;
- fixed non-generative MiniMax verifier and redacted rejection;
- TeamDefinition and non-secret credential-metadata Journal CAS;
- Projection rebuild, copied read view, archive/restore, and view version;
- blank/template/saved Builder, one question at a time, stale edit, exact
  preview, explicit confirmation, and no execution fact;
- setup API canonical arrays;
- Go daemon handler and Bubble Tea no-terminal flow;
- strict Swift setup snapshot/session decoding and closed IPC error set.

## Commands

```text
go test ./internal/credentials ./internal/provider ./internal/state \
  ./internal/projection ./internal/app ./internal/api ./internal/tui \
  ./cmd/loomd \
  -run 'CredentialBroker|CodexNativeAuth|MiniMax|LocalProductSetup|TeamBuilder|StrictSetup' \
  -count=1
```

Result: exit `1`, expected.

Representative missing frozen symbols:

```text
internal/credentials:
  undefined: VerificationStatus
  undefined: MetadataCommand
  undefined: NewCredentialBroker

internal/provider:
  undefined: CodexStatusProcessResult
  undefined: NewCodexNativeAuthObserver
  undefined: NewMiniMaxCredentialVerifier

internal/app/api/tui/cmd:
  undefined: app.SetupSnapshot
  undefined: app.BuilderSessionView
  undefined: app.BuilderStartCommand
```

```text
swift test --package-path apps/macos \
  --filter 'setupSnapshot|builderSession|setupWire|testClosedRemoteErrorSet'
```

Result: exit `1`, expected.

Representative missing frozen symbol:

```text
cannot find 'LocalProductSetupWire' in scope
```

## RED integrity

- Failures are missing frozen W2 symbols/behavior, not syntax or dependency
  errors.
- Existing packages built until reaching the missing W2 interfaces.
- Swift built the existing app/core/UI targets before the missing W2 wire type.
- No network, Provider, Keychain, Codex process, installed app, resident daemon,
  launchctl, Runtime, model, user SQLite, staging, or commit action occurred.
- The previous chat-pasted MiniMax secret was not read or used.

Mandatory RED is complete. Minimal implementation may begin inside the exact
owned files.
