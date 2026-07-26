# S5-W1 Implementation Repair 2 GREEN

Date: 2026-07-26
Baseline: `006db8c`
Gate: fresh Implementation Repair 2 Review 3

## Repair

Implementation Review 2's two findings are closed within the frozen
`internal/api` boundary:

1. authoritative `retry_at` now accepts only empty or the exact canonical UTC
   RFC3339Nano spelling; `evidence_digest` and `source_evidence_digest` accept
   only empty or lower-case SHA-256;
2. every Team-filtered WorkItem source/verifier Evidence reference is resolved
   through the projected Evidence record and exact TeamExecution
   logical-node/attempt relation before its stream enters the cursor scope.

The safe-field test covers valid canonical values plus arbitrary, non-UTC,
malformed, and uppercase values. The controlled high-risk SQLite canary
removes the verifier Evidence fact after the valid delivery proof, rebuilds a
complete Projection, and requires the API to fail closed on the dangling
WorkItem reference.

## Verification

```text
go test ./internal/api \
  -run '^TestAuthoritativeSafeFieldsRejectAmbiguityAndIgnoreRawKeys$' \
  -count=10
PASS

go test ./internal/app \
  -run '^TestTeamExecutionStreamReceivesPostCaptureAuthorizedOutput$' \
  -count=10
PASS

go test ./internal/api ./internal/app ./internal/journal \
  ./internal/projection ./cmd/loom
PASS

go test -race -count=10 ./internal/journal ./internal/projection \
  ./internal/api ./internal/app ./cmd/loom
PASS

journal 7.751s
projection 5.908s
api 12.016s
app 110.195s
cmd/loom 4.436s

go test ./...
PASS

go test -race ./...
PASS

go vet ./...
PASS

GOOS=windows GOARCH=amd64 go test -exec=true \
  ./internal/journal ./internal/projection ./internal/api ./cmd/loom
PASS

gofmt -d <all frozen S5-W1 Go files>
PASS (empty output)

git diff --check
PASS

go mod verify
PASS
```

The Git index remains empty. Excluded shared-worktree paths remain outside the
Candidate. No dependency, migration, writer, daemon, network, installed
Runtime, Provider/model traffic, credential, external action, or S5-W2/S5-W3
scope was added.

VERDICT: PASS
