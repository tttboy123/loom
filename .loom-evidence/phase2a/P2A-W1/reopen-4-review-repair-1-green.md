# P2A-W1 Reopen 4 Review Repair 1 GREEN

**Date**: 2026-07-28
**Status**: `GREEN — PENDING FRESH IMPLEMENTATION RE-REVIEW`
**Live action**: none

## Findings closed

1. Loading a zero-Team snapshot now clears `currentTeam` and the copied
   timeline. The Timeline screen renders the Team-selection instruction, and
   `r` refreshes the snapshot rather than requesting the stale Team.
2. Decimal validation now rejects every all-zero PID string, including `0`,
   `00`, and `000000`, with closed status `invalid_pid`.

## Focused verification

```text
go test ./internal/tui \
  -run '^TestModelTruthfullyHandlesJournalWithNoTeams$' -count=20
go test ./internal/tui -count=1
scripts/test-p2a-w1-live-evidence.sh
go test ./internal/localipc \
  -run '^(TestFileIdentityIncludesFileKind|TestServerReclaimsOnlyStaleSocketAndPreservesReplacement)$' \
  -count=100
go test -race ./internal/localipc \
  -run '^TestServerReclaimsOnlyStaleSocketAndPreservesReplacement$' \
  -count=30
```

All exited `0`.

## Fresh post-repair complete matrix

All commands ran after Review Repair 1 and exited `0`:

```text
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
GOPROXY=https://goproxy.cn,direct \
  GOSUMDB=sum.golang.google.cn go mod tidy
go mod verify
scripts/test-install-loom-local-product.sh
scripts/test-p2a-w1-live-evidence.sh
go test -count=10 ./internal/localipc ./cmd/loomd
go test -race -count=3 ./internal/localipc ./cmd/loomd
GOOS=windows GOARCH=amd64 go test -c ./internal/tui
GOOS=windows GOARCH=amd64 go test -c ./cmd/loom
go test -cover -count=1 ./internal/localipc
go test -cover -count=1 ./internal/tui
git diff --check
git diff --cached --check
```

Coverage is `80.9%` for `internal/localipc` and `87.0%` for `internal/tui`.

## Active Candidate build lock

Two independent private builds used:

```text
go build -trimpath -buildvcs=false -o <root>/loom ./cmd/loom
go build -trimpath -buildvcs=false -o <root>/loomd ./cmd/loomd
```

Both produced identical bytes:

```text
loom   7ba4b41d143ea64fef7cafbfc463f4524ecd3790f93291a38a9c1812278d6ffd
loomd  f9529cf62a1bccbebe4d812dc7408624d2c1ee4c138b92fe69f1dfa497280bbb
```

The retained first build is:

```text
/tmp/loom-p2a-w1-reopen4-repair1-build-a.KkCrvA
```

Both binaries are uid `501`, mode `0700`. Earlier Candidate hashes are
superseded and may not be installed.

## Active source/helper lock

```text
internal/tui/model.go
3abb396cf76fb5477197dc3ba79c12cb06415fb4eec89b8f3f645519ac2d834a

internal/tui/model_test.go
0ce361adb309e42917cbaf2874d8d7c3e4d76c7c8caf8b2f12617760c9621f28

internal/localipc/socket.go
33a02f677df378048bd3db282c91f029c64b5b263383996bf0f12359b3a5dda4

internal/localipc/socket_test.go
16d5e1ececa71d0d02c136f9a215d1d791edf8e455231ef34032683a41695970

scripts/p2a-w1-live-evidence.sh
259426557b65c57a72d4ff587df462b96204ed57964ecf67c2379d3a05c953eb

scripts/test-p2a-w1-live-evidence.sh
fd4c317104ad43f0aaf3c06153d8cd350c2d9771262c1a590cf7e46fb5376ce2
```

The scripts are uid `501`, mode `0700`. Staged diff is empty. Installed
observer/state remain unchanged.

This GREEN does not activate installation, a replacement canary, Computer Use,
commit, or P2A-W2.
