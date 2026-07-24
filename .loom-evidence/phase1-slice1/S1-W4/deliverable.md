# S1-W4 Deliverable

## 1. What changed

- Added a rebuildable projection bound to the committed SQLite Event Journal.
- Added read-only mode, WorkItem, and digest-only Evidence snapshots.
- Added canonical replay, exact-duplicate idempotency, typed conflict/gap/schema
  failures, relevant-payload validation, and supported unknown-event skipping.
- Added off-to-the-side candidate rebuilds, serialized rebuild execution,
  context-aware cancellation, atomic snapshot replacement, and deep-copy reads.
- Added real 64-character lowercase hexadecimal Evidence digest validation
  aligned with the accepted Artifact Store.

## 2. Result and exact evidence

- Initial RED failed to compile because `New`, `Snapshot`, `WorkItem`,
  `Evidence`, and the Journal source did not exist.
- Controller Repair 1 found that the public Candidate accepted arbitrary Event
  sources and accepted non-resolvable digest placeholders. Repair RED failed on
  the old `New()` / `Rebuild(ctx, Source)` API. The repair bound production
  construction to `*sql.DB`, removed public source injection, serialized
  rebuilds, and aligned digest identity.
- The first strict Reviewer reproduced a canceled-waiter race and returned
  `VERDICT: FAIL`. Controller reproduced it with `-count=200`.
- Repair 2 added the required post-gate `ctx.Err()` check.
- Controller verification after Repair 2 exited 0:
  - canceled-waiter target at `-count=200`: `ok ... 0.327s`;
  - canceled-waiter race at `-count=100`: `ok ... 1.329s`;
  - focused projection tests: `ok ... 0.320s`;
  - focused race at `-count=50`: `ok ... 2.374s`;
  - projection package full/race: `0.269s` / `1.340s`;
  - repository full/race: all packages passed;
  - `go vet ./...`, `gofmt` cleanliness, and `git diff --check` passed.
- A Reviewer then incorrectly attributed a pre-S1-W4 Controller
  `docs/CURRENT.md` update to the Developer Candidate. The Controller froze
  file provenance without changing product code.
- A new provenance-scoped strict Reviewer independently reran the high-repeat
  and complete checks, found no blocking issue, and returned `VERDICT: PASS`.

## 3. Affected files and behavior

- `internal/projection/projection.go`
- `internal/projection/projection_test.go`
- Production callers can only rebuild from committed Journal rows through the
  database-bound projection.
- A rebuild is visible only after complete successful replay; query or replay
  failure and cancellation preserve the previous snapshot.
- The projection exposes copies and has no domain mutation method, so it is not
  a second state authority.

## 4. Remaining risk and unverified boundary

- The projection is intentionally in-memory and is rebuilt on process start; no
  persisted projection migration or daemon lifecycle exists in Slice 1.
- Cross-stream replay order is the frozen `(stream_id, seq, id)` order; causal
  facts that depend on one another must use compatible stream structure.
- SQLite behavior was runtime-tested on the local Darwin host only.
- Verification used local Go 1.26.4; the Go 1.22 floor was not run with a
  separately installed toolchain.

## 5. Next executable step

Open S1-W5 only: freeze the standard read-only CLI query contract, ensure the
CLI uses the projection interface rather than direct SQLite reads, add RED
success/invalid/unavailable-state tests, and require a fresh Reviewer PASS
before closing Slice 1.

VERDICT: PASS
