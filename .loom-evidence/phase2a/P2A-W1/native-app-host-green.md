# P2A-W1 Native App Host GREEN

**Date**: 2026-07-28
**Status**: deterministic implementation `GREEN`
**Live status**: not authorized; not installed or launched
**Current gate**: fresh independent Implementation Review

## Implemented vertical boundary

The Candidate now contains:

- a dependency-free Swift 5-compatibility package under Swift tools 6.0;
- a native SwiftUI `Loom.app` target and independently addressable window;
- a direct Darwin AF_UNIX client for the existing `localipc` v1 daemon API;
- strict snapshot and Team timeline models with duplicate/unknown/trailing/
  UTF-8/frame/identity/shape/size rejection;
- the exact twelve closed Go v1 remote error codes, including
  `unsupported_platform`;
- an in-memory `@MainActor` store that atomically replaces successful views and
  preserves a prior view on refresh failure;
- the shared Bubble Tea zero-Team and direct Timeline instructions;
- bounded ANSI/OSC/control/bidi-safe rendering;
- Home, Runtimes, Teams, Runs/History, Evidence, Compare, Attention, and Team
  Timeline screens;
- a non-GUI Swift contract probe;
- a reproducible ad-hoc-signed arm64 bundle build;
- a user-owned app installer with dry-run, symlink rejection, atomic
  replacement, exact prior bundle retention, injected-failure restoration,
  and rollback.

The native entrypoint removes only the frozen five Provider marker names with
`unsetenv` before constructing the app/store/view and never reads or restores
their values.

## TDD closure

The mandatory RED is recorded in `native-app-host-red.md`. The initial Swift
test failed because `LoomLocalAppCore` did not exist. The packaging tests then
failed because the builder and installer did not exist.

After minimal implementation:

```text
cd apps/macos && swift test
```

passed `17` XCTest cases after Review Repair 1. They cover:

- big-endian framing and size bounds;
- exact response identity/shape;
- exact ASCII request-ID grammar;
- duplicate, unknown, trailing, and oversized response rejection;
- all twelve remote error codes;
- private owned `0600` Unix socket acceptance;
- exact nested snapshot/timeline schema;
- prior-view preservation after refresh failure;
- zero-Team no-request behavior;
- one bounded request after visible Team selection;
- explicit fatal classification for nonrecoverable daemon errors;
- ANSI, OSC, C0, bidi, and content-length sanitization.

```text
cd apps/macos && swift build -c release
```

passed for the native app and contract probe.

## Cross-language component proof

The Darwin-only `internal/localipc/swift_contract_test.go`:

1. builds the exact release Swift probe;
2. starts the real Go W1 `localipc.Server` on a private `0700` fixture root and
   `0600` socket;
3. proves ping, snapshot, and timeline semantic agreement;
4. exercises every safe Go v1 remote error code;
5. runs deterministic malicious loopback responses and rejects duplicate keys,
   unknown fields, trailing bytes, oversized frames, mismatched request IDs,
   and invalid result/error shapes.

The component changes no Event, stream head, Journal, daemon configuration, or
external process.

## Packaging and installer proof

```text
scripts/test-build-loom-local-app.sh
scripts/test-install-loom-local-app.sh
scripts/test-install-loom-local-product.sh
```

all returned `PASS`.

The app build fixture proves bundle identifier
`com.earendilworks.loom.local`, bundle name `Loom`, arm64 executable, strict
ad-hoc signature, `0700` directories/executable, `0600` regular resources,
reproducible executable/plist bytes, no symlinks, and the frozen static
exclusions.

The app installer fixture proves no-op dry run, first install, replacement,
retained prior bundle, injected post-swap failure and signal restoration,
rollback to exact prior file digests, and symlink-target rejection.

## Complete repository verification

All mandatory commands passed:

```text
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go mod verify
git diff --check
git diff --cached --check
```

`go test` passed all `23` repository packages. The full race matrix passed,
including the Go/Swift component tests. `go mod verify` reported
`all modules verified`. Git staging was and remains empty.

Swift build products were removed with `swift package reset` after
verification; no `.build` artifact is part of the Candidate.

## Authority and exclusion audit

- no Go production file was reopened;
- no daemon, Journal, Projection, StateWriter, Scheduler, Runtime, Provider,
  Grant, Evidence authority, or credential state changed;
- no `Process`, `NSTask`, shell, CLI parsing, bridge, workspace discovery,
  `launchctl`, SQLite, SQL, HTTP, TCP, WebView, `UserDefaults`, or client cache
  exists in the native product source;
- no historical Cockpit source was copied;
- the app has no mutation method;
- no app or Candidate daemon was installed, bootstrapped, restarted, or
  launched;
- Reopen 4 remains consumed and unchanged;
- P2A-W2 remains locked.

Fresh independent Implementation Review is required. Even a Review `PASS`
does not itself authorize a controlled native-window canary; the exact
post-Review user phrase in the frozen contract remains mandatory.
