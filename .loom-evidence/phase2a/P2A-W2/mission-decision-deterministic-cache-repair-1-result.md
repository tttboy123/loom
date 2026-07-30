# P2A-W2 Mission Decision Deterministic Cache Repair 1 Result

**Date**: 2026-07-30
**Verdict**: `PASS`
**Live effect**: none; live remains locked

## Recoverable quarantine

Immediately before the transaction, no SwiftPM, Go test or compiler process
and no open handle referenced `apps/macos/.build`.

The Controller created a private `0700` quarantine parent and atomically
renamed the exact generated cache to:

```text
/Users/lune/Library/Application Support/Loom/
p2a-w2-mission-workbench-decision-client-reopen-001/
quarantine/apps-macos-build-after-deterministic-matrix
```

The cache root remained on device `16777229` with inode `76185010`, owner
`501`, mode `0755` and observed size `88M`. No file was deleted, recursively
rewritten, restored or reused.

Symlink-safe pre/post manifests match byte-for-byte:

```text
lstat entries        173
regular files        147
symlinks                1

lstat digest
8042ddc6ef1aa2a5750d1fb1ae3f43a4ef51f592d40e17c226bffdbb2aa52eb9

regular-file digest
ae1e19795cdc0cfcb0058aea4d921dcaf3d680709b1e7796f88e90919f660a62

symlink digest
82e73ecca12088c98442df2f7118363fa1aa80f5380b2f5bb45456bd0d740080
```

The manifest records the `release` symlink link text and does not follow it.
The repository `apps/macos/.build` path is absent.

## Controlled SwiftPM routing

The bounded command:

```text
SWIFTPM_BUILD_DIR=<attempt>/swiftpm-go-contract
swift build --package-path apps/macos -c release --arch arm64 --show-bin-path
```

returned exactly a path beneath:

```text
/Users/lune/Library/Application Support/Loom/
p2a-w2-mission-workbench-decision-client-reopen-001/
swiftpm-go-contract/arm64-apple-macosx/release
```

and did not recreate the repository cache.

## Sequential rerun

With the same controlled `SWIFTPM_BUILD_DIR`:

- real Go Server to strict Swift prepared-decision fixture: `PASS`;
- `go test -count=1 ./...`: `PASS`;
- `go test -count=1 -race ./...`: `PASS`;
- `go vet ./...`: `PASS`;
- focused prepared Authorization/Review/Recovery, stale view/generation,
  concurrent winner and replay rejection: `PASS`;
- focused fail-closed daemon registry and controlled Mission fixture: `PASS`.

`apps/macos/.build` remained absent after every command.

## Scope and stop state

Product diff remains exactly:

```text
apps/macos/Sources/LoomLocalAppCore/LocalIPCClient.swift
apps/macos/Tests/LoomLocalAppTests/LocalIPCClientTests.swift
```

There is no controlled daemon, native app, TUI, product socket, Attempt 003
state or live fixture. This Repair is ready for fresh independent Result
Review; it does not unlock live by itself.
