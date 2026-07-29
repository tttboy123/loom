# P2A-W1 Native App Host Contract Revision

**Date**: 2026-07-28
**Status**: FROZEN — fresh independent Contract Re-review `PASS`
**Parent**: P2A-W1 Local App Shell and Read Experience Contract
**Decision**: proposed ADR-0012
**Exit Amendment**: Phase 2A Exit Contract Amendment 1
**WorkItem count**: unchanged; this is P2A-W1, not Reopen 5 or P2A-W4

## 1. Vertical closure

This Revision replaces only the failed Terminal-hosted ordinary-user launch
surface. It adds a native macOS read client and reproducible user-level
`Loom.app` bundle over the already-reviewed W1 daemon IPC.

The existing Bubble Tea implementation, CLI, local API, Journal, Projection,
daemon composition, and fail-closed Reopen 4 result remain preserved. No
Terminal-hosted canary is retried.

The revised W1 exits only when one reviewed Candidate proves together:

1. a Finder-launchable `Loom.app` opens one independently addressable native
   window without Terminal;
2. Home, Runtimes, Teams, Runs/History, Evidence, Compare, Attention, and Team
   Timeline use the current typed daemon API directly;
3. the real one-Event resident Journal renders one Pi Runtime, zero Teams, and
   truthful zero-record/selection states;
4. Team selection is user-visible and typed internally;
5. quit/relaunch and one daemon restart recover the same canonical view;
6. the app writes no client cache and starts no subprocess;
7. Bubble Tea and native fixtures agree on the same bounded screen semantics;
8. installation and rollback preserve exact prior app/daemon bytes and modes.

## 2. Exact owned files

This Revision may create only:

```text
apps/macos/Package.swift
apps/macos/Resources/Info.plist
apps/macos/Sources/LoomLocalApp/LoomLocalApp.swift
apps/macos/Sources/LoomLocalApp/ContentView.swift
apps/macos/Sources/LoomLocalAppCore/LocalIPCClient.swift
apps/macos/Sources/LoomLocalAppCore/LocalProductModels.swift
apps/macos/Sources/LoomLocalAppCore/LocalProductStore.swift
apps/macos/Sources/LoomLocalAppCore/SafeText.swift
apps/macos/Sources/LoomLocalAppContractProbe/main.swift
apps/macos/Tests/LoomLocalAppTests/LocalIPCClientTests.swift
apps/macos/Tests/LoomLocalAppTests/LocalProductModelsTests.swift
apps/macos/Tests/LoomLocalAppTests/LocalProductStoreTests.swift
apps/macos/Tests/LoomLocalAppTests/SafeTextTests.swift
internal/localipc/swift_contract_test.go
scripts/build-loom-local-app.sh
scripts/test-build-loom-local-app.sh
scripts/install-loom-local-app.sh
scripts/test-install-loom-local-app.sh
```

It may modify only:

```text
scripts/install-loom-local-product.sh
scripts/test-install-loom-local-product.sh
docs/CURRENT.md
docs/adr/0011-tui-first-local-product-over-versioned-daemon-ipc.md
docs/adr/0012-native-app-host-over-shared-daemon-ipc.md
docs/adr/README.md
.loom-evidence/phase2a/EXIT-CONTRACT-AMENDMENT-1-NATIVE-APP-HOST.md
.loom-evidence/phase2a/P2A-W1/native-app-host-contract-revision.md
.loom-evidence/phase2a/P2A-W1/native-app-host-*.md
```

No Go production package, daemon protocol, API model, Journal, Projection,
StateWriter, Scheduler, Credential Broker, Runtime, Supervisor, authorization,
Evidence, or Phase 1 file is reopened. The one new Go file is a darwin-only
cross-language component test; it cannot ship in either binary. An unexpected
need stops for
`HUMAN_REQUIRED`; it does not create another thin Amendment or WorkItem.

## 3. Dependency and platform lock

- Toolchain: the installed Apple Swift `6.3.3`.
- Package language mode: Swift 5 compatibility under
  `swift-tools-version: 6.0`.
- Minimum platform: macOS 14.
- Architecture for the controlled product: `arm64`.
- Frameworks: Apple SwiftUI, Foundation, AppKit, Darwin only.
- Third-party Swift packages: none.
- Bundle identifier: `com.earendilworks.loom.local`.
- Bundle name: `Loom`.
- App destination: the explicit user-owned `/Users/lune/Applications/Loom.app`
  for the controlled live gate.
- Daemon socket:
  `/Users/lune/Library/Application Support/Loom/run/loomd.sock`.

The build is local and ad-hoc signed for the controlled user account. No App
Store, Developer ID, notarization, network fetch, package registry, update
service, or release publication is part of W1.

## 4. Direct IPC compatibility

The Swift client implements the accepted `localipc` v1 contract exactly:

- one private AF_UNIX connection per request;
- four-byte big-endian unsigned body length;
- request maximum `65,536` bytes;
- response maximum `524,288` bytes;
- protocol version `1`;
- bounded request ID;
- methods limited to `ping`, `snapshot`, and `timeline_page`;
- exactly one JSON object frame and EOF;
- five-second connection/request deadline;
- no duplicate JSON keys, unknown fields, invalid UTF-8, trailing bytes, empty
  body, mismatched request ID, invalid result/error shape, or oversized frame;
- closed typed error mapping for invalid request, unsupported version, unknown
  method, unauthorized peer, unsupported platform, not found, cursor conflict,
  stream gap, state unavailable, timeout, busy, and internal failure.

Before connect, the client proves:

- absolute canonical socket path;
- controlled `0700` non-symlink parent owned by uid `501`;
- socket file is an actual Unix socket, non-symlink, owned by uid `501`, mode
  `0600`;
- no caller-supplied path in the ordinary app.

The app never implements business validation or authoritative writes.

## 5. Native read model and UI

The app has one `@MainActor` in-memory store. A successful snapshot atomically
replaces copied state. A failed refresh preserves the prior immutable view and
marks it stale/offline with a closed reason. No snapshot or timeline is written
to disk, preferences, defaults, Keychain, cache, or logs.

The window uses:

- a user-visible sidebar for Home, Runtimes, Teams, Runs, Evidence, Compare,
  Attention, and Team Timeline;
- accessible labels and keyboard focus;
- bounded lists with stable visible names;
- loading, empty, partial, stale, offline, recoverable conflict, and fatal
  states;
- safe plain-text rendering that removes terminal escapes, control characters,
  bidi controls, and unbounded content;
- an exact zero-Team message and direct Timeline selection instruction;
- internal typed Team selection with no displayed/typed database path, socket,
  cursor, or service command.

Enter/activation on an empty Team list performs no timeline request. Selecting
an actual Team performs one bounded timeline request. Reconnect never
resubmits a mutation because W1 has none.

## 6. Process, state, and secret exclusions

Static and runtime gates require:

- no `Process`, `NSTask`, shell, CLI, bridge, workspace discovery,
  `launchctl`, SQLite, SQL, Provider, Runtime process, HTTP, TCP, or WebView
  implementation in the app;
- no filesystem write from app code;
- no `UserDefaults`, `CacheStore`, Core Data, SQLite, or custom persistence;
- no Provider key/value read, prompt injection, environment forwarding, raw
  Grant, hidden reasoning, or Evidence payload storage;
- the first native entrypoint action removes the closed Provider marker-name
  set with `unsetenv` before constructing the store or any view, without
  reading, copying, logging, comparing, or restoring a value;
- no inherited Provider marker visible in the settled native app process;
- no credential, internal path, raw ID, terminal escape, or bidi control in
  accessibility text or screenshots.

The historical Cockpit source is not an implementation dependency and is not
copied into the Candidate.

## 7. Packaging and rollback

The local build script:

1. builds the exact Swift package in release mode without network dependencies;
2. creates a private staged `.app` with the reviewed Info.plist and executable;
3. enforces directory `0700`, executable `0700`, and regular file `0600`
   permissions before install;
4. ad-hoc signs the complete bundle;
5. verifies bundle identifier, executable, architecture, signature, and
   secret-negative static surfaces.

The app installer:

- accepts only an absolute, regular, user-owned reviewed bundle;
- owns only the explicit user Applications destination;
- rejects symlinked parents, bundle, executable, or rollback targets;
- installs atomically and retains exact recoverable prior bytes when present;
- removes only its own transaction temporaries on failure;
- never changes daemon, LaunchAgent, Journal, Provider, or credential state.

The existing daemon installer remains independently rollback-safe. A controlled
live transaction composes the two reviewed installers and restores both exact
pre-states on any gate failure.

## 8. Mandatory RED

Before product implementation, tests must fail because:

1. no native Swift package or app target exists;
2. no direct UDS Swift client can interoperate with a locked Go loopback server;
3. no strict protocol decoder rejects duplicate/unknown/trailing/oversized
   input;
4. no store preserves a prior view across failed refresh;
5. no zero-Team native navigation exists;
6. no safe-text renderer exists;
7. no reproducible `.app` build/install/rollback path exists.

The RED evidence must also prove the historical bridge/cache source is not
silently copied.

## 9. Deterministic GREEN

Mandatory verification after implementation:

```text
cd apps/macos && swift test
cd apps/macos && swift build -c release
scripts/test-build-loom-local-app.sh
scripts/test-install-loom-local-app.sh
scripts/test-install-loom-local-product.sh
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go mod verify
git diff --check
git diff --cached --check
```

Additional component proof must:

- start the real Go W1 localipc server on a private fixture socket;
- run the separate non-GUI Swift contract probe against that exact server;
- compare canonical snapshot/timeline JSON semantics with the Go client;
- reject malformed frames and exercise every closed Go v1 error variant:
  `invalid_request`, `unsupported_version`, `unknown_method`,
  `unauthorized_peer`, `unsupported_platform`, `not_found`,
  `cursor_conflict`, `stream_gap`, `state_unavailable`, `timeout`, `busy`, and
  `internal`;
- prove no Event, stream head, Journal byte, daemon configuration, or external
  process changes.

Static scans are supporting evidence only; runtime fixture and app tests are
mandatory.

## 10. Native controlled live gate

Contract Review, RED, implementation, deterministic verification, and
Implementation Review do not activate live execution.

After fresh independent Implementation Review `PASS`, one native-app live
canary requires a new explicit user message authorizing:

```text
P2A-W1 Native App Host and one controlled native-window canary
```

The canary must:

1. revalidate exact original and Candidate app/daemon hashes and modes;
2. install atomically and bootstrap the Candidate daemon once;
3. use Computer Use on `com.earendilworks.loom.local`, never Terminal;
4. inspect Home, real Pi Runtime, zero Teams, direct Timeline instruction,
   Runs, Evidence, Compare, and Attention;
5. activate the empty Team list and prove no timeline request or state change;
6. quit/relaunch the app;
7. restart the daemon once and recover the identical canonical view;
8. prove process, accessibility, screenshot, log, Journal, and evidence
   secret-negative;
9. preserve only on full PASS, otherwise roll back app and daemon exactly.

The old Reopen 4 allowance remains `0`. This gate is unavailable until all new
review and activation conditions close. There is no hidden retry.

## 11. Exit

The Revision is accepted only after:

1. fresh independent Contract Review `PASS`;
2. mandatory RED captured;
3. deterministic GREEN and complete matrix `PASS`;
4. fresh independent Implementation Review `PASS`;
5. one explicitly activated native-window live canary `PASS`;
6. fresh independent Result-Evidence Review `PASS`.

Only then may P2A-W1 be committed and P2A-W2 unlocked.

## 12. Contract Review Repair 1

Contract Review 1 returned `FAIL` because the Swift closed typed error set
omitted the real Go v1 `unsupported_platform` response emitted by peer
credential failure.

This repair changes only the contract:

1. section 4 now names `unsupported_platform` in the exact closed set;
2. section 9 now enumerates every Go v1 safe error code for the
   Go-server/Swift-probe component proof.

No owned file, product behavior, dependency, live authority, WorkItem count, or
Reopen 4 invocation accounting changed. Fresh independent Contract Re-review is
required before RED or implementation.
