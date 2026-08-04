# SF-W1 Implementation Progress (RED → focused green → full matrix + journey)

Date: `2026-08-04`

Status: `IN PROGRESS — FULL MATRIX GREEN + JOURNEY COMPLETE; REVIEWS + COMMIT PENDING`

Baseline: P3A-W1 `7d5f0b01`; Gate 0 PASS; Gate 1 Contract Re-review 2 PASS
(`GATE1-CONTRACT-REVIEW-2.md`, P0=P1=P2=0); docs-only Gate 1 governance
commit `737c58c8`.

## Governed amendments frozen and reviewed (P0=P1=P2=0 each)

- `SF-W1-OWNED-FILE-AMENDMENT-1.md` + Review 1 PASS — additive-owned
  delivery wiring required by the frozen journey: `cmd/loomd/product_daemon.go`
  (queue API build/wire + `queue_snapshot`/`queue_command` routing),
  `cmd/loomd/product_daemon_test.go`, `internal/tui/model.go` + `model_test.go`
  (additive `ScreenQueue` wiring), `LocalIPCClient.swift` (additive
  `call`/`callJourney` visibility widening), and probe `main.swift` (queue
  action modes).
- `SF-W1-SCHEMA-AMENDMENT-1.md` + Review 1 PASS — `QueueJob` record gains
  `capability_kind`, `exit_conditions`, `verification_strategy`,
  `integration_strategy`, `protected_authority_paths` (Decomposition
  Compiler rules 4/6/7/8 record home).
- `SF-W1-SCHEMA-AMENDMENT-2.md` + Review 1 PASS — field-exact
  `SuccessorProposal` record for `SuccessorProposalCreated` (read-only
  compilation; no WorkItem side effect).
- `SF-W1-OWNED-FILE-AMENDMENT-2.md` + Review 1 PASS — additive-only
  allowlist additions required by the SF-W1 atomic commit:
  `cmd/loomd/run.go` (adds exactly `build_assets` and `build_queue` to
  `validDaemonBuildFailureReason`; `build_assets` completes the
  P3A-committed reason that the accepted P3A commit never added to the
  validator), `cmd/loomd/sf1_queue_wire_test.go` (new real-socket queue
  wire test), `scripts/verify-sf1-cross-client-journey.sh` (journey
  verifier, P3A evidence-substrate precedent) and
  `docs/runbooks/sf1-cross-client-journey.md` (journey runbook, P3A
  runbook precedent).

## RED (mandatory failing tests first)

RED tests were written before implementation in `internal/queue/` (model,
projection, conflict arbiter, admission, gap proposal) and ran red:
`go test ./internal/queue/` failed to compile with `undefined: JobSubmission`,
`undefined: CompileSubmission`, `undefined: ErrDenied`, `undefined:
CompiledJob`, `undefined: JobStatus`, `undefined: Lane` (exact RED evidence,
same pattern as the accepted P3A missing-symbol RED).

## Implementation (all frozen owned files + additive wiring)

- `internal/queue/model.go` — JobStatus/Lane enums, QueueJob record (amended
  §5), JobSubmission, validation.
- `internal/queue/projection.go` — Journal replay → queue projection
  (QueueJobCreated/Admitted/Cancelled, GapProposalCreated,
  SuccessorProposalCreated).
- `internal/queue/admission.go` — eligibility, thin-capability rejection,
  exit/verification/integration strategy required, protected-authority
  fail-closed, DAG cycle + duplicate-active-work rejection.
- `internal/queue/conflict_arbiter.go` — owned-path / mutex / resource
  (no-oversell) parallelism decisions.
- `internal/queue/gap_proposal.go` — digest-bound `gap_id`, duplicate
  convergence, read-only successor compilation with stale-evidence
  rejection.

## Cross-client journey (COMPLETE — VERIFIED PASS)

The mandatory SF-W1 GUI+TUI journey ran end to end on a private root
(`/private/tmp/sf1-journey-final`, journey
`70862374-d9cd-4ac6-a02f-2be68344bdac`) using the accepted alternative
verification method: production Swift client over the real daemon socket,
real PTY TUI, real native window launched with `--socket --journey-id` and
captured with `screencapture`. Verified by
`scripts/verify-sf1-cross-client-journey.sh` (PASS).

Journey assertions observed and recorded:
- Two Jobs sharing `internal/queue/model.go` were queued and admitted;
  the Conflict Arbiter serialized the shared path at dispatch (both
  clients observed 2 admitted Jobs in the Queue Projection).
- A self-dependent (DAG cycle) Job was rejected with a typed `denied`
  error and zero Journal facts.
- One authorized failed-Run observation produced exactly one digest-bound
  `GapProposal` (`gap_id 48054c36…`, disposition `observe`); a duplicate
  observation converged on the same `gap_id` (`merge_duplicate`, zero new
  facts).
- Restart/reconnect checkpoint: daemon restarted mid-journey; the Queue
  Projection rebuilt from the Event Journal and both clients observed the
  identical 2-Job / 1-Gap state with no duplicates.

Defects found and fixed during the journey (regression-tested):
1. Swift `callJourney` method allowlist omitted `queue_snapshot`/
   `queue_command` (`LocalIPCClient.swift`) — probe could not reach the
   daemon.
2. Queue projection `Replay` rejected unrelated Journal event types
   (identity index, runtime discovery) — the shared-Journal read model now
   skips unrelated events and still fails closed on malformed queue events
   (`internal/queue/projection.go` + tests).
3. Empty slice fields marshalled as `null` broke the strict Swift models —
   `JobSubmission.compiled()` normalizes empty slices to `[]` and the
   duplicate-gap receipt returns `event_ids: []` (`internal/queue/model.go`,
   `internal/app/local_queue.go`).

Next: implementation / dual-Result / Whole-Candidate reviews, then exact
staging and the single SF-W1 atomic local commit; then SF-W2.

The SF-W1 Implementation Review 1 (PASS, `SF-W1-IMPLEMENTATION-REVIEW-1.md`)
and Owned-File Amendment 2 Review 1 (PASS,
`SF-W1-OWNED-FILE-AMENDMENT-2-REVIEW-1.md`) are landed; the SF-W1 source
lock was regenerated post-verdict (digest `02fca97a…` recomputes from the
tree). Remaining: dual-Result and Whole-Candidate reviews, then exact
staging and the single SF-W1 atomic local commit.
  rejection.
- `internal/projection/queue.go` — separate rebuildable queue read model
  (P3A core `projection.go` untouched).
- `internal/app/local_queue.go` — service over the real Journal:
  `create_job`/`cancel_job`/`gap_observe`/`successor_compile` appended only
  via `AppendBatchIfStreamHeads` CAS; snapshots replay the Journal.
- `internal/api/local_queue.go` — `queue_snapshot` / `queue_command` API.
- `internal/localipc/protocol.go` — additive `queue_snapshot`/`queue_command`
  (validMethod + requiresJourney).
- `internal/tui/queue.go` + additive `model.go` wiring — `Queue` screen
  (jobs/gaps/successors) with Tab + r refresh in the real PTY TUI.
- `cmd/loomd/product_daemon.go` + `run.go` — queue API build/wire, routing,
  error-code mapping (`invalid_request`/`conflict`/`denied`), `build_queue`
  failure reason.
- `apps/macos/Sources/LoomLocalAppCore/LocalQueueModels.swift` + probe
  modes — production Swift client `queueSnapshot`/`queueCommand` over the
  real socket; probe `--queue-snapshot`, `--queue-create-job`,
  `--queue-gap-observe`, `--queue-successor-compile`.

## Verification (focused, green)

- `go test ./...` full suite PASS (includes `cmd/loomd` 64s suite).
- `go test -race` on `internal/queue`, `internal/api`, `internal/app`,
  `internal/tui` PASS.
- `go vet ./...` clean; `gofmt` clean on all owned files; `go mod tidy` no
  diff.
- Swift full `swift build` + `swift test` PASS (incl. new
  `LocalQueueModelsTests`).
- New daemon wire test `TestSF1QueueWireOverSocket` PASS: real socket →
  `queue_command` create_job (2 events) → `queue_snapshot` (1 admitted job)
  → duplicate DAG node → `conflict` code → gap_observe duplicate → converge
  on one `gap_id` with `merge_duplicate`.

## Verification (full deterministic matrix, green)

- `go test -count=1 ./...` PASS (incl. `cmd/loomd` 65s suite).
- `GOFLAGS=-p=1 go test -count=1 -race ./...` PASS (all packages incl.
  `cmd/loomd` 106s race suite, `internal/queue`, `internal/api`,
  `internal/app`, `internal/tui`, `internal/localipc`,
  `internal/projection`).
- `GOFLAGS=-p=1 go vet ./...` PASS; `gofmt` clean on all owned files;
  `go mod tidy` no diff.
- Swift full `swift test` (scratch `/tmp/sf1-swift/debug`): 92 tests,
  1 intentional visual-export skip, 0 failures; `swift test
  --sanitize=thread` (scratch `/tmp/sf1-swift/tsan`) PASS; `swift build
  -c release` (scratch `/tmp/sf1-swift/release`) PASS.

## RED coverage (SF-W1 mandatory set)

| RED | Proof |
|---|---|
| unmet eligibility not admitted | admission_test + app protected/eligibility tests |
| owned-path conflict not parallel | conflict_arbiter_test |
| same mutex not parallel | conflict_arbiter_test |
| DAG cycle rejected | admission_test (ErrDAGCycle) |
| duplicate active work rejected | app concurrent-create single-winner test |
| unauthorized/stale evidence cannot create successor | gap_proposal_test + app successor test |
| duplicate gap observations converge on one gap_id | gap_proposal_test + app gap test |
| Journal rebuild reproduces queue projection | projection_test + app rebuild assertion |
| protected-authority path claim fails closed | admission_test + app test |

## Next steps

1. Implementation Review, dual Result (Product + Operational/Trace)
   reviews, Whole-Candidate Review, exact staging, one atomic local commit.
2. SF-W2 and SF-W3 under the same gates, then the controlled canary and the
   whole-slice review.

## Evidence completion closure (2026-08-04)

The SF-W1 journey root `/private/tmp/sf1-journey-final` (journey
`70862374-d9cd-4ac6-a02f-2be68344bdac`) was completed to the §8 evidence
standard (P3A re-freeze precedent): `manifest.json` (23 evidence files,
self-consistent `manifest_digest`), `journal/event-summary.json`,
`journal/stream-heads.json`, `journal/sqlite-summary.json` (integrity ok,
8 events, 5 journey-correlated, zero duplicates/gaps/FK violations),
`source/source-lock.json` (SF-W1 final source inventory), frozen
`source/runtime-fixture.json`, `manifest/tui-plan.json`,
`manifest/preflight-source-lock.json`, corrected
`manifest/journey-context.json` (real baseline commit `7d5f0b01d5…`,
final binary digests, Gate 1 set digest `0e008ac9…`, SF-W1 amendment
digests, `network_allowed=false`,
`provider_credentials_present=false`) and
`projection/summary.json` `view_version` `071cc441…` (computed from the
journal stream heads; the same method reproduces the accepted P3A happy
root's `view_version` exactly). `scripts/verify-sf1-cross-client-journey.sh`
now fails closed on manifest self-digest, evidence digest/mode identity,
journal summary consistency, journey-context binary digests and the
canonical view version; re-verified PASS (exit 0).

VERDICT: `SF-W1 FULL MATRIX GREEN + JOURNEY COMPLETE — REVIEWS + COMMIT PENDING`
