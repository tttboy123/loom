# P2A-W2 Provider Connection and Delegated OAuth Amendment

**Date**: 2026-07-30
**Status**: FROZEN — fresh independent Contract Review PASS
**Parent**: `P2A-W2 Team Builder and Provider Onboarding Contract`
**Product direction**: Provider onboarding must follow an App-native
connection-directory flow like the Codex plugin directory rather than exposing
terminal-oriented configuration as the primary product interaction.

## 1. Scope

This amendment reopens only the existing P2A-W2 Provider onboarding boundary.
It creates no P2A-W4 and does not unlock or modify P2A-W3.

It closes:

1. the confirmed Codex 0.144.1 status-channel compatibility defect;
2. App-native Codex delegated OAuth initiation through the exact configured
   official Codex executable;
3. a unified native Provider connection surface with capability-specific
   actions;
4. removal of the always-visible MiniMax API-key field from the Team Builder
   page in favor of an explicit connection-management sheet.

It does not implement Team execution, Provider fallback, generic third-party
OAuth, a second credential store, token import/export, or model inference.

## 2. Authentication capability model

Provider connection behavior is capability-specific:

- **Codex** uses delegated `native_auth`. Loom may start exactly
  `codex login` through the configured, identity-bound official executable.
  Codex owns its OAuth browser flow and credential storage. Loom never receives,
  parses, copies, persists, logs, or revokes the OAuth token.
- **MiniMax** continues to use the reviewed `brokered` API-key path because its
  public general API contract uses Bearer API keys. The secret remains in the
  macOS Keychain through the existing Credential Broker.

The shared UI pattern does not imply a shared authentication protocol. A
Provider may expose `Connect`, `Manage`, `Test`, or `Disconnect` only when its
reviewed capability supports that action.

## 3. Codex delegated login boundary

The daemon-owned Codex connection controller:

- binds the exact absolute regular executable and its containing directory;
- revalidates executable identity before launch;
- invokes exactly `codex login` with a fixed minimal environment;
- captures no OAuth token, browser callback, device code, stdout, or stderr;
- starts at most one login process at a time;
- uses a bounded lifetime and process-group cancellation;
- owns and joins the child process on daemon shutdown;
- returns only closed safe success state: `started`, `already_connected`;
- reports `busy` and `unavailable` only through the existing safe error path,
  never as a successful result;
- performs no automatic retry and no `logout`;
- observes completion only through the existing closed `login status` observer.

The request is an explicit user action. It is never triggered by snapshot
refresh, Team Builder start, daemon startup, Runtime discovery, or retry logic.

## 4. Status compatibility

Codex 0.144.1 emits the exact successful line
`Logged in using ChatGPT` on stderr with empty stdout and exit code zero.

The observer may accept an allowlisted complete status line from exactly one
output channel when the other channel is empty. It must reject:

- both channels non-empty;
- partial, multiline, leading/trailing-space, or unknown output;
- non-zero success status;
- output beyond the existing bound;
- any raw output crossing the Provider/application/IPC boundary.

## 5. IPC and application boundary

P2A-W2 protocol version 1 adds one exact method:

```text
codex_connect
```

It takes `{}` and returns a strict result containing only:

```text
provider_id = codex
auth_mode   = native_auth
status      = started | already_connected
```

Safe failures map to existing closed errors `busy`, `timeout`,
`state_unavailable`, or `internal`. Unknown fields and unsupported Provider
actions fail closed.

The application service checks current status first. When already connected it
returns `already_connected` without launching a process.

## 6. Native product behavior

The Team Builder Provider area becomes a connection list:

- one row/card per Provider;
- icon, human name, authentication description, and closed status;
- one primary action: `Connect` when disconnected or `Manage` when connected;
- immediate loading feedback after activation;
- Codex `Connect` delegates to the daemon and then performs bounded snapshot
  refresh until connected, failed, or timed out;
- MiniMax `Connect`/`Manage` opens a native sheet containing the existing
  masked Keychain actions;
- destructive revoke remains explicit and separate;
- keyboard, VoiceOver, reduced-motion, narrow-window and cancellation behavior
  remain usable.

No terminal, shell command, filesystem path, API-key environment variable,
internal Provider ID, credential reference, raw reason, or token is shown to an
ordinary user.

## 7. Exact owned files

This amendment reopens only files already owned by P2A-W2:

```text
internal/provider/codex_native_auth.go
internal/provider/codex_native_auth_test.go
internal/app/local_product_setup.go
internal/app/local_product_setup_test.go
internal/api/local_product_setup.go
internal/api/local_product_setup_test.go
internal/localipc/protocol.go
internal/localipc/protocol_test.go
internal/localipc/swift_contract_test.go
cmd/loomd/product_daemon.go
cmd/loomd/product_daemon_test.go
apps/macos/Sources/LoomLocalAppCore/LocalIPCClient.swift
apps/macos/Sources/LoomLocalAppCore/LocalProductSetupModels.swift
apps/macos/Sources/LoomLocalAppCore/LocalProductStore.swift
apps/macos/Sources/LoomLocalAppUI/ContentView.swift
apps/macos/Tests/LoomLocalAppTests/LocalIPCClientTests.swift
apps/macos/Tests/LoomLocalAppTests/LocalProductSetupModelsTests.swift
apps/macos/Tests/LoomLocalAppTests/LocalProductStoreTests.swift
apps/macos/Tests/LoomLocalAppTests/LocalProductExperienceViewTests.swift
docs/CURRENT.md
.loom-evidence/phase2a/P2A-W2/
```

No Journal schema, migration, Runtime adapter, Agent/Team authority, scheduler,
execution, Phase 1 evidence, user-owned dirty file, or installed credential is
owned.

## 8. RED and verification

Before implementation, tests must fail for:

1. exact stderr-only Codex 0.144.1 success compatibility;
2. rejection of ambiguous/both-channel and unknown output;
3. exact delegated `codex login` launch, identity fencing, singleton start,
   bounded cancellation, daemon shutdown join, and zero raw-output exposure;
4. already-connected no-launch behavior;
5. strict `codex_connect` IPC request/result;
6. Swift Provider connection UI state and MiniMax management-sheet behavior.

Verification must include focused Go/Swift tests, complete Go and race suites,
vet, module/format/diff checks, Swift debug/release/TSan, secret-negative scans,
and exact owned-file review. Deterministic tests use only fixtures and may not
start real OAuth, open a browser, mutate installed Codex credentials, touch the
resident daemon, or perform a live Provider request.

## 9. Exit

This amendment is complete only after mandatory RED, deterministic GREEN,
fresh independent Implementation Review, and one user-visible native fixture
showing the connection-directory behavior. Any real OAuth launch requires a
separate controlled live authorization after Implementation Review.

Passing this amendment does not repair the separately observed empty Pi model
catalog, reinterpret the prior secret-source result, accept P2A-W2, or unlock
P2A-W3.
