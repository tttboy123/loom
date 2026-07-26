# S3-W5 Bounded Product Repair 1

- Date: `2026-07-26`
- Parent: `implementation-review-1.md`
- Scope: same Candidate lineage; no owned-file expansion

## RED

Tests were added before production repair.

```text
go test ./internal/work \
  -run TestTeamAttemptRebindFencesExpiredGenerationAndIsIdempotent -count=1
```

Observed:

```text
wrong previous rebound retry error = <nil>
```

```text
go test ./internal/app \
  -run '^TestTeamCoordinatorRecoversDurableAttemptWindows/(missing|divergent)' \
  -count=1
```

Observed:

```text
missing capture rejects later same-generation Run activity:
  Run() error = Team execution incomplete

divergent capture conflicts before lease expiry:
  Run() error = Team execution incomplete
```

## Repair

- Exact rebound retry now locates the committed rebound Event and requires the
  complete old/new binding plus correlation to match the retry input.
- Absent capture now requires the validated Run stream head to remain sequence
  one, the initial `RunClaimed`; a lease extension or any later Run Event fails
  closed before filesystem mutation.
- Existing capture binding is checked before the lease-expiry branch. The only
  accepted split is Team/capture generation N with Run N+1, or Team N with
  capture/Run N+1.
- Added fault injection at both accepted split boundaries. It exposed that the
  Coordinator initially waited on the new N+1 lease before completing the
  missing capture/Team rebound. Repair now waits only while Run and Team remain
  at the same generation; an already-accepted N+1 reclaim immediately
  completes the remaining cross-store sequence and executes once.
- `AGENTS.md` remains user-owned and excluded from staging.

## Focused GREEN

```text
go test ./internal/work \
  -run TestTeamAttemptRebindFencesExpiredGenerationAndIsIdempotent -count=1
go test ./internal/app \
  -run '^TestTeamCoordinatorRecoversDurableAttemptWindows$' -count=1
```

Result: PASS.

The expanded restart test now covers:

- terminal capture finalization and repeated reopen exact-once;
- already-finalized receipt with missing metadata;
- immediate post-dispatch absent-capture repair;
- later same-generation Run activity rejection;
- divergent pre-expiry capture rejection;
- expired claimed recovery with old Grant revocation;
- crash after Run reclaim before capture rebind;
- crash after capture rebind before Team rebound; and
- indeterminate running state with zero re-execution.

Post-repair package and race matrices passed:

```text
go test ./internal/evidence ./internal/journal ./internal/teams \
  ./internal/work ./internal/authorization ./internal/projection \
  ./internal/supervisor ./internal/runtime/piadapter ./internal/app -count=1

go test -race ./internal/evidence ./internal/work \
  ./internal/authorization ./internal/projection ./internal/app -count=1

go test -race ./internal/app \
  -run '^(TestTeamDAGExecutionControlledCanary|TestTeamCoordinatorRecoversDurableAttemptWindows)$' \
  -count=5
```

Result: PASS.

Fresh independent Repair 1 Review 2 returned `PASS` with no findings. The full
post-repair repository and race matrices also passed.

VERDICT: PASS
