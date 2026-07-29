# P2A-W1 Native App Launchability Deterministic GREEN

**Date**: `2026-07-28`
**Status**: `GREEN — AWAITING FRESH INDEPENDENT IMPLEMENTATION REVIEW`
**Product files changed**: exactly two
**Live authority used**: none

## Minimal implementation

The builder changed both linker invocations from:

```text
-Xlinker -no_uuid
```

to:

```text
-Xlinker -reproducible
```

It retained `-no_adhoc_codesign`, `strip -S`, and one final timestamp-free
ad-hoc bundle signature.

The build fixture now:

- rejects `-no_uuid` and `-random_uuid`;
- requires exactly two production `-reproducible` linker arguments;
- requires exactly one nonzero arm64 UUID per build;
- requires identical UUIDs across two package-reset builds;
- preserves byte-identical signed executable and complete bundle proof;
- rejects remaining N_OSO/debug symbol-table records;
- snapshots DiagnosticReports before RED and before/after launch smoke;
- launches only after UUID, architecture, and signature gates pass;
- proves the exact private executable survives one second;
- terminates and waits for the exact captured child;
- proves no new crash report appears.

No Swift, Go, plist, installer, daemon, IPC, Journal, Projection, Provider,
Runtime, authorization, or product authority file changed.

## Focused GREEN

```text
scripts/test-build-loom-local-app.sh
```

passed twice after implementation. Each run performed two complete
`swift package reset` builds and the private launch smoke.

An additional independent clean build matched the first retained build exactly:

```text
LoomLocalApp SHA-256
f473684cd6026cd3fd8b80955b2ac81a40a6222f963a72782dc65fd5b65bc06b

arm64 LC_UUID
CE91F84E-4333-35DB-B493-88FADCBC6EC1

Info.plist SHA-256
554a6c8b990a75b5e86318a2a4e4ed7002fd2b8c4063c6a6071e5900d5132ba5

CodeResources SHA-256
6686de10a28a2fe11b36cbb86dcbacc827cfc4ea116b4dabf1845e5aee629e9b

canonical complete bundle manifest
e29c1b6c3720492f57cbab4eade293ac1b3cc208caf15f47cd9d4402424a1cbd
```

The bundle is mode `0700`, executable mode `0700`, Info.plist mode `0600`,
arm64, strict-signature valid, and symlink-free.

## Complete deterministic matrix

Passed:

```text
scripts/test-build-loom-local-app.sh
cd apps/macos && swift test
cd apps/macos && swift build -c release
scripts/test-install-loom-local-app.sh
scripts/test-install-loom-local-product.sh
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go mod verify
gofmt -l <all Go files>
git diff --check
git diff --cached --check
sh -n scripts/build-loom-local-app.sh
sh -n scripts/test-build-loom-local-app.sh
```

Swift executed 17 tests with zero failures. Both native and full local-product
installer rollback fixtures passed. Full Go repository and race matrices
passed, vet passed, all modules verified, formatting and shell syntax were
clean, and the build/evidence secret scan passed.

The first full `go test -count=1 ./...` attempt stopped on the historical
five-second fake-Pi metadata fixture timeout:

```text
TestLocalRuntimeObservationDaemonRealSQLiteRestart
pi metadata command failed: version
```

This failure was preserved rather than hidden. The exact focused test then
passed in `1.89s`, and one unchanged fresh full repository command passed,
including `internal/app` and the long `internal/runtime/piadapter` package.
The production diff does not touch that test or any Go file.

## External-state closure

After all GREEN commands:

- DiagnosticReports still contain exactly the two preserved canary reports
  with their original hashes;
- no launch-smoke crash report exists;
- original observer is `running`;
- actual target-process Provider marker count is `0`;
- resident SQLite hash remains
  `91ae07e0a46868c80fd9bb7233fde9d3b9dffac1b92c49a0b25dd264d949c8a4`;
- SQLite `integrity_check=ok`, Event count `1`;
- installed Candidate app/run/socket/launcher and native process are absent;
- Swift package cache was reset;
- Git staging is empty.

## Gate

Fresh independent Implementation Review is required. This GREEN does not
authorize installation, Computer Use, another native-window canary, commit, or
P2A-W2. Native allowance remains `0`; the prior failed canary remains
historically true.
