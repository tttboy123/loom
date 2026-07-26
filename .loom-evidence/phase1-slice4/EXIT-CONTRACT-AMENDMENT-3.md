# Phase 1 Slice 4 Exit Contract Amendment 3

Status: FROZEN — independent Contract Repair Review 2 PASS.

Date: 2026-07-26

Parent: `EXIT-CONTRACT.md`

## Name

S4-W3 Verification and Acceptance Authority Integration Boundary

## Necessity

Accepted S3/S4 code already owns every boundary S4-W3 must extend:

- `work.Authority` owns WorkItem/Run state, generation fencing, Runtime
  capacity, and the only WorkItem Journal writer;
- Team execution owns the exact node/attempt-to-WorkItem/Run relationship and
  currently treats a valid output classification as node success;
- `TeamCoordinator` owns the controlled managed execution, Grant, durable
  attempt capture, classification, recovery, and restart sequence;
- Projection and `GlobalReadView` own the rebuildable read surface.

Implementing S4-W3 only in new files would either leave `ready_for_review`
without a legal terminal transition or create a second acceptance/Team writer.
Both violate the frozen Slice 4 exit contract. The accepted files below must be
reopened as one vertical Candidate.

## Exact reopened scope

S4-W3 may reopen only:

- `internal/rules/recovery_policy.go`
- `internal/rules/recovery_policy_test.go`
- `internal/work/run_authority.go`
- `internal/work/run_authority_test.go`
- `internal/work/team_execution_authority.go`
- `internal/work/team_execution_authority_test.go`
- `internal/app/team_execution.go`
- `internal/app/team_execution_test.go`
- `internal/projection/projection.go`
- `internal/projection/projection_test.go`
- `internal/projection/run_authority.go`
- `internal/projection/run_authority_test.go`
- `internal/projection/team_execution.go`
- `internal/projection/team_execution_test.go`
- `internal/projection/global_read_view.go`
- `internal/projection/global_read_view_test.go`

S4-W3 may add only:

- `internal/verification/acceptance.go`
- `internal/verification/acceptance_test.go`
- `internal/work/verification_authority.go`
- `internal/work/verification_authority_test.go`
- `.loom-evidence/phase1-slice4/S4-W3/**`
- the Controller-owned S4-W3 hunk in `docs/CURRENT.md`
- this amendment and its review evidence.

No other product or governance file is owned.

## Permitted changes

The reopened files may change only to:

1. freeze a versioned, immutable per-node AcceptanceContract and risk route
   before the first Team attempt;
2. keep a contract-valid executor result at `ready_for_review`, never `done`;
3. create a deterministic, separate verifier WorkItem/Run identity when the
   frozen risk route requires independent verification;
4. execute that verifier through the accepted Claim, Grant, Supervisor,
   authorized Frame, AttemptCapture, and Evidence receipt boundaries;
5. decode a bounded verifier Candidate, never a direct authority result;
6. validate deterministic and independent verification inputs inside the
   existing `work.Authority`;
7. append verification facts plus accepted/rejected/Done and Team-node
   consequences in one `AppendBatchIfStreamHeads` transaction over the exact
   touched streams;
8. route a verified rejection through the existing bounded recovery policy
   with an explicit verification-rejection trigger and no hidden retry;
9. rebuild and deep-copy the new verification/acceptance lineage in the
   existing Projection and `GlobalReadView`; and
10. add controlled local SQLite/Supervisor fixtures covering the complete
    Slice 4 exit proof.

## Preserved authority

- `AppendBatchIfStreamHeads` remains the only state mutation primitive.
- `work.Authority` remains the only WorkItem acceptance writer.
- `evidence.Store` remains the immutable artifact authority; it is consumed
  through existing Store-returned receipts and is not reopened.
- `authorization.Authority` remains the Grant authority and is not reopened.
- The verifier has no source WorkItem, Team, Journal, Projection, or Done write
  access. Its output is a Candidate bound to its own Run/Grant/Evidence.
- Projection and `GlobalReadView` remain rebuildable caches.
- Exact retries remain separate Run/generation/Grant/Evidence lineages.
- Existing Bridge v1, Supervisor binding/sequence checks, and authorized Frame
  order remain unchanged.

## Explicit exclusions

This amendment does not permit:

- a second Journal, StateWriter, acceptance authority, Team writer, Evidence
  store, Projection, scheduler, or coordinator;
- changes to `internal/evidence`, `internal/authorization`,
  `internal/supervisor`, `internal/runtime`, or Bridge v1;
- executor `done`, verifier self-acceptance, caller-supplied accepted booleans,
  Projection-authoritative acceptance, or client/notification authority;
- Provider/model fallback, credentials, raw Grant, hidden reasoning, raw
  verifier output, per-token Events, or direct personal identifiers in
  Journal/Projection;
- real Runtime/Provider/network traffic, daemon/API/CLI/Web/TUI activation,
  model-context/session/process checkpoint, autonomy, or external action;
- dependency changes, S4-W4, Slice 5 implementation, Phase 2 implementation,
  push, merge, rebase, reset, release, or publication.

## Admission gate

This amendment and the exact S4-W3 contract require fresh independent Contract
Review `PASS` before mandatory RED. Any requested file outside the exact list,
or any inability to preserve one-writer acceptance, requires another reviewed
bounded amendment or `HUMAN_REQUIRED`; it must not create S4-W4.
