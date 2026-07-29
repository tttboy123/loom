# P2A-W1 Atomic Commit Verification

**Date**: 2026-07-29  
**Purpose**: fresh verification before the Product Owner-authorized local
atomic P2A-W1 checkpoint commit

## Passing checks

The following checks completed with exit status `0`:

```text
GOPROXY=off GOTOOLCHAIN=local go test \
  ./internal/projection ./internal/api ./internal/localipc ./internal/tui \
  ./cmd/loom ./cmd/loomd
GOPROXY=off GOTOOLCHAIN=local go test ./...
GOPROXY=off GOTOOLCHAIN=local go test -race ./...
GOPROXY=off GOTOOLCHAIN=local go vet ./...
GOPROXY=off GOTOOLCHAIN=local go mod tidy -diff
GOPROXY=off GOTOOLCHAIN=local go mod verify
gofmt -d cmd/loom cmd/loomd internal/api internal/localipc \
  internal/projection internal/tui
git diff --check
cd apps/macos && swift test
cd apps/macos && swift build -c release
cd apps/macos && swift test --sanitize=thread
scripts/test-install-loom-local-product.sh
scripts/test-build-loom-local-app.sh
scripts/test-install-loom-local-app.sh
scripts/test-p2a-w1-live-evidence.sh
.loom-evidence/phase2a/P2A-W1/native-app-final-exit-transaction-test.sh
```

Both normal and Thread Sanitizer Swift suites executed `27` tests with zero
failures and one governed visual-export skip.

The frozen Windows command used `go test` and therefore attempted to execute
Windows `.exe` files on macOS, producing the expected host
`exec format error`. Its actual cross-platform intent was reproduced with:

```text
GOOS=windows GOARCH=amd64 go test -c ./internal/tui
GOOS=windows GOARCH=amd64 go test -c ./cmd/loom
```

Both outputs were verified as `PE32+` x86-64 Windows executables and deleted
from the private temporary directory.

## Preserved historical failure

The following historical live-transaction fixture exits nonzero at its
complete source-lock assertion:

```text
.loom-evidence/phase2a/P2A-W1/local-product-live-closure-transaction-test.sh
```

That result is required by the current lineage:

1. the fixture binds the earlier failed live Candidate and its exact immutable
   source-lock;
2. the later independently reviewed Authoritative Collection Wire
   Normalization Reopen legitimately changed three Go inputs bound by that old
   lock;
3. the reopen explicitly prohibited regenerating the prior live Candidate,
   source-lock, manifest, or allowance;
4. rewriting those historical hashes would falsely reinterpret the consumed
   failed canary as a current executable Candidate.

The old fixture and hashes therefore remain unchanged. This commit is a
reviewed deterministic P2A-W1 implementation checkpoint with a preserved
failed live result, not a passing or newly authorized live Candidate.
