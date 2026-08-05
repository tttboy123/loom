# v0.4.1-W2 cross-client journey — two real parallel development Candidates

This runbook is the framework-driven parallel-development proof for `W2` of
the frozen `V0.4.1-CONTRACT.md`. It uses the v0.4.1 product-line operations
(W1) and the accepted v0.4.0 scheduling framework (SF-W1/W2/W3) exactly as
shipped; W2 adds no product code. Verification follows the accepted
alternative-verification method: production Swift client over the real
daemon socket, real PTY TUI, real native window launched with
`--socket --journey-id`; Computer-Use-driven window automation is skipped.

## Scenario assertions (frozen)

1. **Two Jobs** — the production Swift client creates Job A
   (`internal/tui/model.go`, owned path distinct) and Job B
   (`internal/tui/queue.go`, owned path distinct) through
   `queue_command create_job`; both are admitted with no
   owned-path/mutex/resource conflict (`QueueJobCreated` +
   `QueueJobAdmitted` ×2).
2. **Parallel claims** — the real PTY TUI claims Job A while the production
   Swift client claims Job B with Job A still open: two `AttemptClaimed`
   facts precede any result fact, so the Event Journal proves two
   independent workers (each on its own Candidate branch/worktree)
   executing at the same time.
3. **Test-before-development-complete overlap** — worker B's deterministic
   test failure is recorded while worker A's development attempt is still
   open.
4. **Seven-class routing** — B1 fails with `product_defect`; the schedule
   Router routes that class to the Repair lane; a fresh worker claims the
   same Job in `lane=repair` and succeeds (`AttemptResultRecorded
   succeeded` + `CandidateReadyForReview`).
5. **Read-only Reviewer verdicts** — both Candidates receive a Reviewer
   verdict PASS (`ReviewVerdictRecorded` ×2; one via the TUI for A, one via
   the Swift client for B2).
6. **Single Integrator, exactly one versioned release** — the TUI
   integrates Candidate A (`IntegrationStarted` + `CandidateIntegrated`,
   one published release); the Swift client's integration of Candidate B on
   the same target branch is rejected by the CAS with zero new events;
   a later Run adopts the release (`ReleaseAdoptedByLaterRun`).
7. **Restart/reconnect checkpoint** — the daemon restarts mid-journey and
   the production Swift client reads back the rebuilt state from the
   Journal (queue jobs=2, worker attempts=3, integration releases=1) with
   no duplicate facts.
8. **Evidence hygiene** — frozen evidence schema, one unique journey_id,
   0700/0600 permissions, postflight arrays empty, no secret-like
   material, `scripts/verify-v041-w2-cross-client-journey.sh` PASS.

## Driving commands

```sh
# jobs (production Swift client)
PROBE=apps/macos/.build/arm64-apple-macosx/release/LoomLocalAppContractProbe
$PROBE --socket ROOT/loomd.sock --queue-create-job JOURNEY job-a.json
$PROBE --socket ROOT/loomd.sock --queue-create-job JOURNEY job-b.json

# worker B lifecycle (production Swift client; attempt IDs come from the
# claim receipts)
$PROBE --socket ROOT/loomd.sock --workers-command JOURNEY claim   wb-claim.json
$PROBE --socket ROOT/loomd.sock --workers-command JOURNEY result  wb-fail.json
$PROBE --socket ROOT/loomd.sock --workers-command JOURNEY claim   wb2-claim.json
$PROBE --socket ROOT/loomd.sock --workers-command JOURNEY result  wb2-ok.json
$PROBE --socket ROOT/loomd.sock --workers-command JOURNEY result  wb2-review.json

# single-writer proof (expected rejection: error:conflict, exit 2)
$PROBE --socket ROOT/loomd.sock --integration-command JOURNEY integrate b2-integrate.json

# restart read-back
$PROBE --socket ROOT/loomd.sock --queue-snapshot JOURNEY
$PROBE --socket ROOT/loomd.sock --workers-snapshot JOURNEY
$PROBE --socket ROOT/loomd.sock --integration-snapshot JOURNEY
```

Worker A is driven by the real PTY TUI (Queue → Workers → Integration
screens; `c` claim, `t` deterministic test success, `v` read-only Reviewer
PASS, `i` integrate, `a` adopt, `r` refresh, `q` quit). The native window
is launched with `open ROOT/app/Loom.app --args --socket ROOT/loomd.sock
--journey-id JOURNEY` and captured with `screencapture`.

## Freeze / verify

```sh
scripts/verify-v041-w2-cross-client-journey.sh ROOT
```

The verify script additionally proves: two queue jobs, three worker
attempts (development ×2 + repair ×1), exactly one release, the exact
Journal fact set, two claims before any result, the first result classified
`product_defect`, exactly one repair-lane claim, and exactly one
integration sequence (the competing integration left zero events).

## Consumed journey

`/private/tmp/v041-w2-journey`, journey_id
`63d4ea90-588d-4359-bb0c-942a2c767020`, verified PASS. This is a consumed
journey root: it is evidence, not a re-runnable fixture. A re-run requires
an independent Review PASS and a new journey_id/root.
