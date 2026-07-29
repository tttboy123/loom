# P2A-W1 Native App Launchability Implementation Review

**Date**: `2026-07-28`
**Reviewer**: fresh independent read-only Reviewer
**Verdict**: `PASS`
**Findings**: none

## File and behavior assessment

The Reviewer confirmed the exact two-file closure:

- both builder link invocations use `-Xlinker -reproducible`;
- neither `-no_uuid` nor `-random_uuid` remains;
- `strip -S` remains;
- exactly one final timestamp-free ad-hoc bundle signature remains;
- two package-reset builds require one nonzero identical arm64 UUID;
- N_OSO/debug symbol-table records are rejected;
- signed executable, Info.plist, CodeResources, complete file set, modes, and
  canonical bundle manifest are byte-identical;
- the private exact-child smoke is UUID/architecture/signature-gated;
- child cleanup and crash-report inventory are bounded.

No Swift, Go, plist, installer, daemon, IPC, Journal, Projection, Provider,
Runtime, authority, WorkItem, or live boundary was reopened.

## Independent verification

Passed:

```text
scripts/test-build-loom-local-app.sh
two independent clean Candidate builds
cd apps/macos && swift test
cd apps/macos && swift build -c release
scripts/test-install-loom-local-app.sh
scripts/test-install-loom-local-product.sh
go test -count=1 ./internal/app -run TestLocalRuntimeObservationDaemonRealSQLiteRestart
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go mod verify
shell syntax, diff, source/linker/security scans
```

The Reviewer reproduced:

```text
arm64 LC_UUID
CE91F84E-4333-35DB-B493-88FADCBC6EC1

LoomLocalApp SHA-256
f473684cd6026cd3fd8b80955b2ac81a40a6222f963a72782dc65fd5b65bc06b

Info.plist SHA-256
554a6c8b990a75b5e86318a2a4e4ed7002fd2b8c4063c6a6071e5900d5132ba5

CodeResources SHA-256
6686de10a28a2fe11b36cbb86dcbacc827cfc4ea116b4dabf1845e5aee629e9b

canonical complete bundle manifest
e29c1b6c3720492f57cbab4eade293ac1b3cc208caf15f47cd9d4402424a1cbd
```

A redundant `/bin/cmp` reviewer subcheck was unavailable because that exact
path does not exist on this Mac. It was not counted; independent exact hashes
and canonical manifest equality carried the byte-identity proof.

## External-state verification

- exactly two preserved canary crash reports, hashes unchanged;
- original observer `running`;
- actual target-process Provider marker count `0`;
- resident SQLite original hash, integrity `ok`, Event count `1`;
- Candidate app/run/socket/launcher/native process absent;
- Swift package cache absent after reset;
- Git staging empty.

## Boundary

This `PASS` closes only the deterministic launchability and reproducibility
implementation.

It does not authorize installation, Computer Use, another native-window
canary, commit, acceptance, or P2A-W2. Native allowance remains `0`.
