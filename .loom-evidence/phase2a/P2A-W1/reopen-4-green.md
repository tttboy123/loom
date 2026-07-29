# P2A-W1 Reopen 4 Deterministic GREEN Evidence

**Date**: 2026-07-28
**Status**: `SUPERSEDED BY REVIEW REPAIR 1 GREEN`
**Live action**: none

## Closed behavior

- A zero-Team snapshot now renders:

  ```text
  No Teams exist in this Journal yet.
  ```

- Enter on an empty Teams screen issues no command or timeline request.
- Direct Team Timeline navigation without a selected Team renders:

  ```text
  Select a Team from Teams to open its authoritative timeline.
  ```

- Loaded records, board, Attention, and gap pages retain their prior rendering.
- Saved-Team and strict `historical_execution_only` typed selection behavior is
  unchanged.
- The live-evidence helper accepts one decimal PID, invokes target-only
  `ps eww -p PID -o command=`, requires one row, emits only closed status, and
  locks live execution to `/bin/ps`.
- File cleanup identity now includes `Lstat` file kind in addition to device
  and inode. A same-device/inode regular replacement can no longer equal the
  original socket identity.

## RED to GREEN

These focused commands now pass:

```text
go test ./internal/tui \
  -run '^TestModelTruthfullyHandlesJournalWithNoTeams$' -count=20
scripts/test-p2a-w1-live-evidence.sh
go test ./internal/localipc \
  -run '^TestFileIdentityIncludesFileKind$' -count=1
go test ./internal/localipc \
  -run '^TestServerReclaimsOnlyStaleSocketAndPreservesReplacement$' \
  -count=100
go test -race ./internal/localipc \
  -run '^TestServerReclaimsOnlyStaleSocketAndPreservesReplacement$' \
  -count=30
```

The real replacement stress gate passed without retry, timeout increase, or
weakened assertion.

## Complete verification

The following ran sequentially after Repair A and exited `0`:

```text
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
GOPROXY=https://goproxy.cn,direct \
  GOSUMDB=sum.golang.google.cn go mod tidy
go mod verify
/bin/sh -n \
  scripts/install-loom-local-product.sh \
  scripts/test-install-loom-local-product.sh \
  scripts/p2a-w1-live-evidence.sh \
  scripts/test-p2a-w1-live-evidence.sh
scripts/test-install-loom-local-product.sh
scripts/test-p2a-w1-live-evidence.sh
GOOS=windows GOARCH=amd64 go test -c ./internal/tui
GOOS=windows GOARCH=amd64 go test -c ./cmd/loom
go test -count=10 ./internal/localipc ./cmd/loomd
go test -race -count=3 ./internal/localipc ./cmd/loomd
go test -cover -count=1 ./internal/localipc
go test -cover -count=1 ./internal/tui
git diff --check
git diff --cached --check
```

Coverage:

```text
internal/localipc  80.9%
internal/tui       86.9%
```

An earlier deliberately concurrent verification batch exposed both the
replacement identity defect and one resource-contended handler-start timeout.
Those failures were not overwritten or treated as PASS. Repair A closed the
identity defect; the frozen verification commands were then rerun sequentially
and the complete non-race, race, count, and coverage gates passed.

## Target-process host proof

The durable helper inspected the currently restored observer through the exact
target-only method and returned only:

```text
clean
```

The target-only command selected one row. The former dash-`e` form selected
1,090 host rows in the same read-only check. No row, environment, marker match,
argument, or value was emitted.

## Reproducible Candidate build lock

Two independent private build directories used:

```text
go build -trimpath -buildvcs=false -o <root>/loom ./cmd/loom
go build -trimpath -buildvcs=false -o <root>/loomd ./cmd/loomd
```

Both builds produced identical bytes:

```text
loom   b094536f8f9f4ed593ba9a468870c3c1cb97422651de335efe235ee4ad485a20
loomd  f9529cf62a1bccbebe4d812dc7408624d2c1ee4c138b92fe69f1dfa497280bbb
```

Both files are uid `501`, mode `0700`. The retained first reviewed build is:

```text
/tmp/loom-p2a-w1-reopen4-build-a.SrYegS
```

## Source/helper lock

```text
internal/tui/model.go
7475efe031e72eecbba5484f1bc1017059934babd61a294563cf30933a8e46a0

internal/tui/model_test.go
863c3f9f452c51ed58c32d672d8ababb922ad855d052c038d4d89b65cb1e1efd

internal/localipc/socket.go
33a02f677df378048bd3db282c91f029c64b5b263383996bf0f12359b3a5dda4

internal/localipc/socket_test.go
16d5e1ececa71d0d02c136f9a215d1d791edf8e455231ef34032683a41695970

scripts/p2a-w1-live-evidence.sh
faf176cccf6c9e5e0ff9ecaaf318c9d81fc6ba90ac06f3121cbe689f737f7079

scripts/test-p2a-w1-live-evidence.sh
5d8edfec4de42687bb76ddf9f32ee7ec0e57e2588885b1efec7fd3dddd401d10
```

Both scripts are uid `501`, mode `0700`.

## Boundary audit

- TUI has no SQL, SQLite, Journal, Projection, shell, service-manager, or HTTP
  dependency.
- Local IPC has no Journal, Projection, StateWriter, CLI, daemon, or execution
  authority dependency.
- The helper contains the exact `eww -p` form and no `-eww` form.
- The fake helper test contains one sentinel marker/value solely to prove
  non-disclosure; no real credential or Provider value is present.
- No source, process argument, log, Journal, Evidence, screenshot, installed
  state, LaunchAgent, SQLite, Provider, Runtime, Team, or WorkPackage changed.
- Staged diff remains empty. Pre-existing user-owned worktree changes remain
  preserved.

Reopen 4 is not live-activated. This GREEN does not authorize installation,
bootstrap, Computer Use, commit, P2A-W2, or a replacement canary.

Implementation Review 1 found stale Team selection across a later zero-Team
snapshot and incomplete all-zero PID validation. The original Candidate hashes
above are retained as historical evidence and are not eligible for live use.
