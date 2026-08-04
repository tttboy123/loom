# SF-W2 Implementation Progress (RED → focused green → journey verified)

Date: `2026-08-05`

Status: `CORE IMPLEMENTED + JOURNEY VERIFIED PASS; REVIEWS + COMMIT PENDING`

Baseline: SF-W1 `1abf033c`; Gate 1 governance set committed.

## Governed amendment

`SF-W2-OWNED-FILE-AMENDMENT-1.md` reopens only the additive delivery/evidence
wiring surfaces (`cmd/loomd/product_daemon.go` + test, `sf1_queue_wire_test.go`,
`internal/tui/model.go` + test, probe `main.swift`, `LocalIPCClient.swift`).

## RED (mandatory failing tests first)

RED tests target the new schedule/work/api surfaces (lease expiry, stale
generation, capacity oversell, one-claim-per-worker, hidden-infinite-retry
impossible, test-failure-to-Repair, reviewer-write denial, repair
aging/fairness, crash reclamation with exactly one new generation,
duplicate-attempt idempotency). Tests compile-fail before implementation and
pass after (same pattern as SF-W1).

## Implementation

- `internal/schedule` — `model.go` (Lane/Attempt/Worker/Pool/Router/Failure
  classes, Backoff), `lease.go` (Lease expiry/generation),
  `worker_pool.go` (per-lane capacity, one claim per worker, no oversell),
  `recovery.go` (bounded retry/backoff policy), `reconciler.go`
  (Journal-rebuilt attempt/lease projection, exactly one reclamation).
- `internal/work/worker_execution.go` — authoritative Attempt events via
  `AppendBatchIfStreamHeads` CAS (`AttemptClaimed`, `AttemptResultRecorded`,
  `AttemptCrashed` with crash seam/cardinality, `LeaseReclaimed` +
  `GenerationAdvanced` + `AttemptClaimed` for exactly one new generation,
  `StaleResultRejected`; Reviewer write denied).
- `internal/app/local_workers.go` + `internal/api/local_workers.go` —
  `workers_snapshot` / `workers_command` wire surface.
- `internal/localipc/protocol.go` — additive `workers_snapshot`/
  `workers_command` journey methods; `cmd/loomd/product_daemon.go` routing;
  `cmd/loomd/sf2_workers_wire_test.go` real-socket wire test.
- `internal/tui/workers.go` + model wiring — `Workers` screen (Tab
  navigation, `r` refresh) for the real PTY TUI.
- `LocalWorkerModels.swift` + `LocalIPCClient.callJourneyRaw` + probe
  `--workers-snapshot`/`--workers-command` modes + Swift model tests.

## Cross-client journey (COMPLETE — VERIFIED PASS)

`/private/tmp/sf2-journey-final` (journey
`d0d5f598-617b-4bb9-b0bb-81162c83dbbc`) verified by
`scripts/verify-sf2-cross-client-journey.sh` (PASS):

- Two independent workers claimed two Candidates in parallel (2 active).
- Worker A crashed at the after-CAS seam; Reconciler reclaimed job-a with
  exactly one new generation (gen 2, repair lane).
- Stale result rejected with zero side effects.
- `test_defect` failure recorded (Repair routing, never Integration).
- Reviewer write denied.
- Restart/reconnect checkpoint rebuilt the identical 3-attempt / 2-active
  state from the Journal with no duplicates.

Journal event set (exact): AttemptClaimed ×3, AttemptCrashed ×1,
LeaseReclaimed ×1, GenerationAdvanced ×1, StaleResultRejected ×1,
AttemptResultRecorded ×1 + platform init. Deterministic matrix green
(Go full/race/vet/tidy/gofmt; Swift full 94+4).

Next: implementation / dual-Result / Whole-Candidate reviews, exact staging
per `SF-W2-SOURCE-LOCK.json` (29 paths, digest `a9007cbe…`), single SF-W2
atomic local commit; then SF-W3.
