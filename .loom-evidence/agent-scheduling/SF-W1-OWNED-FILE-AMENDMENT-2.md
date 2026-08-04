# SF-W1 Owned-File Amendment 2 (bounded, frozen)

Date: `2026-08-04`

Status: `FROZEN — INDEPENDENT AMENDMENT REVIEW REQUIRED`

WorkItem: the only and existing `SF-W1` (Queue State/Projection +
Admission/Eligibility/Conflict Arbiter). This Amendment creates no SF-W4,
no thin WorkItem, no new authority, no second database, no new schema, and
no external action.

## 1. Reason

The accepted `SF-WORKITEMS.md` SF-W1 allowlist (as amended by
`SF-W1-OWNED-FILE-AMENDMENT-1.md`, independent Review 1 PASS) still cannot
deliver the frozen SF-W1 Candidate as one atomic local commit. Four
delivery-critical paths are absent from the allowlist:

1. `cmd/loomd/run.go` — the daemon's
   `validDaemonBuildFailureReason` list is the validator for
   `newDaemonBuildFailure` reasons. The P3A-committed
   `cmd/loomd/product_daemon.go` already calls
   `newDaemonBuildFailure("build_assets", …)`, but the accepted P3A commit
   never added `build_assets` to this validator (the reason silently
   degraded to `build_unknown`). SF-W1's queue API build wiring calls
   `newDaemonBuildFailure("build_queue", …)` for the same path; both
   additive reason strings must be recognized or the queue build failure
   reports `build_unknown`.
2. `cmd/loomd/sf1_queue_wire_test.go` — the real-socket queue wire test
   (`TestSF1QueueWireOverSocket`) must live in package `main` under
   `cmd/loomd` to exercise the production
   `localProductHandlerWithComposition` dispatch over a real Unix socket.
   The Amendment 1 allowlist addition covered `product_daemon_test.go`
   only; this separate new test file was not named.
3. `scripts/verify-sf1-cross-client-journey.sh` — the frozen §8-conformant
   journey verifier. The accepted P3A candidate committed its journey
   verifier (`scripts/verify-phase3a-cross-client-journey.sh`) as part of
   the evidence substrate; SF-W1's counterpart is a new path absent from
   the allowlist.
4. `docs/runbooks/sf1-cross-client-journey.md` — the SF-W1 journey runbook
   mirroring the accepted P3A runbook
   (`docs/runbooks/phase3a-cross-client-journey.md`), documenting the
   frozen alternative-evidence journey and the verify command.

## 2. Supersession (bounded)

This Amendment amends only the `SF-WORKITEMS.md` SF-W1 owned-file allowlist
by adding the four paths below with additive-only carve-outs. It changes no
other WorkItem allowlist, no contract schema, no authority boundary, no
RED, no journey definition, no verification matrix, and no acceptance item.
`SF-EXIT-CONTRACT.md` and ADR-0014 remain in full force.

## 3. Allowlist additions (additive-only)

1. `cmd/loomd/run.go` — additive: add exactly two strings (`build_assets`,
   `build_queue`) to `validDaemonBuildFailureReason`. `build_assets`
   completes the P3A-committed reason already emitted by
   `product_daemon.go`; `build_queue` serves the SF-W1 queue API build/wire.
   No other behavior, reason, or validation semantic changes.
2. `cmd/loomd/sf1_queue_wire_test.go` — new: real-socket queue wire test
   (`TestSF1QueueWireOverSocket`) driving
   `localProductHandlerWithComposition` over a real Unix socket with the
   production `internal/localipc` client, covering `queue_command`
   create/DAG-cycle-conflict and `queue_snapshot` rebuild/gap-convergence
   assertions.
3. `scripts/verify-sf1-cross-client-journey.sh` — new: read-only journey
   verifier mirroring the accepted P3A verifier structure (0700/0600
   evidence modes, journey-ID drift checks, dual-client traffic, production
   app launch reads, PTY transcript, screenshot presence, SQLite integrity,
   no duplicate Event/idempotency keys, no stream gaps, projection
   `matches_journal`, postflight cleanliness).
4. `docs/runbooks/sf1-cross-client-journey.md` — new: runbook documenting
   the SF-W1 journey (two shared-owned-path Jobs admitted with Conflict
   Arbiter serialization, DAG-cycle typed rejection, one digest-bound Gap
   Proposal with duplicate convergence, daemon restart/reconnect
   checkpoint) and the verification command, following the accepted P3A
   runbook format.

Each file remains governed by the same additive rule used for
`internal/localipc/protocol.go` and the Amendment 1 files: new additions
only; existing behavior, wire format, and accepted P3A semantics are never
reopened.

## 4. Not claimed

This Amendment does not authorize any product-behavior change beyond the
additive wiring above, any authority/schema/credential expansion, any
second writer/database, any push/merge, any network or paid action, any
migration, or any modification of accepted P2A/P2B/P3A contracts or journey
evidence.

## 5. Independent review acceptance

The Amendment passes only if a fresh read-only Reviewer proves: bounded
supersession; the four additions are strictly additive wiring/evidence
substrate required by the frozen SF-W1 Candidate and its atomic commit; the
`build_assets` completion is a factual P3A validator gap and not a product
behavior change; no authority/schema/RED/journey/acceptance change; no
SF-W4 or thin WorkItem created.

VERDICT: `FROZEN — PENDING REVIEW`
