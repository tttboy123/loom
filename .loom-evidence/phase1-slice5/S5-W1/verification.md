# S5-W1 Verification

Date: 2026-07-26
Baseline: `006db8c`
Candidate status: `accepted_for_local_atomic_commit`

## Required checks

Focused and impact:

```text
go test ./internal/journal ./internal/projection ./internal/api ./internal/app ./cmd/loom
PASS

go test ./internal/work ./internal/rules ./internal/verification ./internal/supervisor ./internal/runtime/piadapter
PASS
```

Repeated race:

```text
go test -race -count=10 ./internal/journal ./internal/projection ./internal/api ./internal/app ./cmd/loom
PASS

journal 7.751s
projection 5.908s
api 12.016s
app 110.195s
cmd/loom 4.436s
```

Repository:

```text
go test ./...
PASS

go test -race ./...
PASS

go vet ./...
PASS
```

Static and platform:

```text
gofmt -d <all S5-W1 frozen Go files>
PASS (empty output)

git diff --check
PASS

GOOS=windows GOARCH=amd64 go test ./internal/journal ./internal/projection ./internal/api ./cmd/loom
COMPILE PASS; host execution then reports expected macOS exec-format failure

GOOS=windows GOARCH=amd64 go test -exec=true ./internal/journal ./internal/projection ./internal/api ./cmd/loom
PASS
```

The first Windows command produced `.exe` test binaries successfully and then
macOS could not execute them. `-exec=true` is the bounded Go cross-compilation
check that preserves compilation while suppressing foreign-binary execution.

## Scope, dependency, and authority audit

- The only reopened production files are the frozen Journal, Projection, API,
  app freshness, and CLI files.
- `internal/api` imports no SQLite driver, `database/sql`, Work authority,
  Rule/Verification authority, adapter, network package, or writer.
- `cmd/loom` owns the read-only SQLite handle; the API contains no SQL.
- Journal remains the sole fact authority and Projection remains rebuildable.
- The app amendment performs one read-only rebuild at each accepted call site,
  with no retry or partial-view publication.
- The high-risk external canary executes an independent verifier while proving
  that only the source execution can publish tentative Team output. Both
  source and verifier authoritative lineage remain reconstructable.
- Known payload retry times/digests and every WorkItem source/verifier
  Evidence relation fail closed unless their canonical value and exact
  Team-attempt binding are valid.
- Tentative state is bounded process memory only. No delta/token/raw Grant,
  credential, hidden reasoning, or raw payload is written to Journal,
  Evidence, Projection, CLI stderr, or disk.
- `go.mod`, `go.sum`, and `migrations/**` are unchanged.
- No dependency, migration, executable, daemon, listener, Provider/model
  traffic, installed Runtime, credentials, external action, Web/TUI, S5-W2, or
  S5-W3 capability was added or activated.
- Pre-existing `AGENTS.md`, `PROGRESS.md`, `.codex/**`, `.loom-drafts/**`, and
  the post-S3 scratch queue remain excluded from the Candidate.

VERDICT: PASS
