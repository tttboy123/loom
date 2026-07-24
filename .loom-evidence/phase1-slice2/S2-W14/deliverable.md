# S2-W14 Deliverable

- WorkItem: `S2-W14`
- Title: Atomic Event Journal Batch Append
- Risk: Strict
- Base branch/head: `codex/loom-platform-slice2` at `89dbff3`
- Contract SHA-256:
  `6618f29e7bc8817624e82e115325cc2c9f754db5e6f2d5d06d708dca9a6383f4`
- Product SHA-256:
  `59e6df0f81448baf0a6004434fb7adef0d05e0a3f3d65d00e091cace1ccbdda1`
- Test SHA-256:
  `2099c961ee5012b2f2950733f618cc219b51f593af8b7af950b50d29291bda5f`

## Delivered boundary

`AppendBatch` adds one bounded generic Journal transaction primitive:

- batches contain 1 through 32 accepted immutable Events;
- every Event and internal idempotency/stream-sequence key is validated before
  opening a write transaction;
- committed-state classification is completed before insertion;
- conflict priority is deterministic: idempotency, sequence, then partial
  existing batch;
- all new Events commit once in input order or all roll back;
- exact full-batch and reordered retries return original committed Events; and
- input/result payload bytes are copied.

The implementation changes no migration, Event fields, accepted `Append`
signature, projection, Team/Agent payload, resource, process, or execution
surface.

## Contract review

Fresh independent contract review returned `PASS` with no blocking findings.
It confirmed the accepted schema already provides all required constraints and
no migration is needed.

Evidence:
`.loom-evidence/phase1-slice2/S2-W14/contract-review.md`

## Mandatory RED

The focused command exited `1` only because frozen `AppendBatch` and batch error
symbols were missing. There was no test syntax, dependency, existing-code, or
environment failure.

## Controller verification

All commands passed with exit code `0`:

```text
go test ./internal/journal -run 'TestAppendBatch' -count=1
go test ./internal/journal -count=1
go test -race ./internal/journal -run 'TestAppendBatch' -count=50
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
```

Additional results:

- `gofmt -d internal/journal/store.go internal/journal/batch_test.go`: no output
- `git diff --check`: no output
- `find . -type d -name '__pycache__' -print`: no output
- migration files and accepted Event fields/`Append` signature remain unchanged
- production imports add no filesystem, network, environment, sync/goroutine,
  Team, Agent, WorkItem, Run, grant, or execution surface
- accepted Slice 1 and S2-W1 through S2-W13 tests remain green
- branch/head remained `codex/loom-platform-slice2` at `89dbff3`

## Coverage and trust-boundary evidence

- Tests prove single/multi-stream success, exact retry, reordered retry, stable
  input-order return, and exact persisted row/stream order.
- Tests prove partial-existing, idempotency, and sequence conflicts write
  nothing, plus idempotency-over-sequence precedence under both input orders.
- Tests prove empty/oversized/invalid/internal-duplicate preflight failure,
  late insert rollback, cancelled context, closed database, and commit failure
  return zero output.
- Repeated race tests prove concurrent identical submissions commit exactly one
  fact set and never report partial success.
- Tests prove caller/result payload mutation cannot alter committed facts.
- Static checks exclude schema, domain payload, resource, goroutine, and
  execution widening.

## Implementation review

Fresh independent implementation review returned `PASS` with no blocking
findings. It confirmed real transaction atomicity, deterministic conflict
priority, exact/reordered retry semantics, zero-output failure behavior,
concurrent identical-batch safety, S1-W2 compatibility, and absence of schema
or authority widening.

Evidence:
`.loom-evidence/phase1-slice2/S2-W14/implementation-review.md`

VERDICT: PASS
