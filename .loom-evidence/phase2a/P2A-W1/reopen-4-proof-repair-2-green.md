# P2A-W1 Reopen 4 Proof Repair 2 GREEN

**Date**: 2026-07-28
**Status**: `GREEN — PENDING FRESH IMPLEMENTATION RE-REVIEW`
**Live action**: none

## Proof repair

Every Server lifecycle test now waits on the Server's exact `Ready()` channel
before asserting active lifecycle behavior. The existing path-polling helper is
retained only for client transport tests and no longer establishes Server
readiness.

No product source, timeout, retry policy, error assertion, socket cleanup, or
authority behavior changed in Proof Repair 2.

## High-load verification

The following ran concurrently and passed:

```text
go test -race ./internal/localipc -count=50
go test -race ./internal/localipc \
  -run '^TestServerReclaimsOnlyStaleSocketAndPreservesReplacement$' \
  -count=200
```

The complete non-race local-IPC package then passed:

```text
go test ./internal/localipc -count=100
```

Fresh post-proof repository gates passed:

```text
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go mod verify
scripts/test-install-loom-local-product.sh
scripts/test-p2a-w1-live-evidence.sh
git diff --check
git diff --cached --check
```

## Candidate/source lock

Proof Repair 2 changes test code only. A fresh reproducible build retained the
active Candidate hashes:

```text
loom   7ba4b41d143ea64fef7cafbfc463f4524ecd3790f93291a38a9c1812278d6ffd
loomd  f9529cf62a1bccbebe4d812dc7408624d2c1ee4c138b92fe69f1dfa497280bbb
```

The updated test file lock is:

```text
internal/localipc/server_test.go
95249849c6168fafe2739c2788f13375514551d76080c06c8b7212ac4626839a
```

Staged diff is empty. Installed observer/state remain unchanged. This GREEN
does not activate a replacement canary, install, Computer Use, commit, or
P2A-W2.
