# v0.4.1-W1 cross-client journey

This runbook is the product-line scheduling closure journey for `W1` of the
frozen `V0.4.1-CONTRACT.md`. It follows the accepted alternative-verification
method (Product Owner instruction 2026-08-04): the production native window
launched with `--socket --journey-id`, the real PTY TUI, and mutations/reads
through the production Swift client over the real daemon socket.
Computer-Use-driven window automation remains skipped.

## Scenario assertions (frozen)

1. **Queue screen operation** — the real PTY TUI creates a Job into the
   Queue through the production `queue_command create_job` action
   (`QueueJobCreated` + `QueueJobAdmitted`, admission via the existing
   admission/eligibility rules).
2. **Workers screen operations** — the TUI explicitly claims a worker for
   that Job (`workers_command claim`, `AttemptClaimed` generation 1),
   records a deterministic test success (`workers_command result`,
   `AttemptResultRecorded succeeded` + `CandidateReadyForReview`), and
   records the read-only Reviewer verdict (`ReviewVerdictRecorded PASS`).
3. **Integration screen operations** — the TUI integrates the reviewed
   Candidate through the single Integrator (`IntegrationStarted` +
   `CandidateIntegrated`, versioned release), binds a later Run
   (`ReleaseAdoptedByLaterRun`), and rolls back to the prior version
   (`ReleaseRolledBack`) without rewriting history.
4. **Dual-client observation** — the production Swift probe (native client
   over the real socket) reads queue/workers/integration snapshots after
   each mutation and decodes the exact closed shape (empty collections
   decode, no null-array failure).
5. **Restart/reconnect checkpoint** — the daemon restarts mid-journey and
   both clients observe the rebuilt queue/workers/integration state from
   the Event Journal with no duplicate facts.
6. **Evidence hygiene** — §8-style evidence schema, one unique journey_id
   across client/IPC/Daemon log/Journal/timeline, 0700/0600 permissions,
   postflight processes/sockets/locks/leases/temps empty, no secret-like
   material, `scripts/verify-v041-w1-cross-client-journey.sh` PASS.

## Driving commands (production Swift client over the real socket)

The TUI operations are driven from a real PTY (`/usr/bin/script` transcript
plus a keystroke recorder). The production native window is launched with:

```sh
open "$ROOT/app/Loom.app" --args --socket "$ROOT/loomd.sock" \
  --journey-id "$JOURNEY_ID"
```

The production Swift probe performs the native read-backs:

```sh
PROBE=apps/macos/.build/arm64-apple-macosx/release/LoomLocalAppContractProbe
$PROBE --socket "$ROOT/loomd.sock" --queue-snapshot "$JOURNEY_ID"
$PROBE --socket "$ROOT/loomd.sock" --workers-snapshot "$JOURNEY_ID"
$PROBE --socket "$ROOT/loomd.sock" --integration-snapshot "$JOURNEY_ID"
```

The daemon runs in a PTY with the frozen runtime-dir / probe-id /
local-model trio; the harness manifest purpose is
`phase3a-cross-client-e2e`. The TUI screen cycle navigates Queue → Workers →
Integration with `tab`; operations are `n` (queue a job path), `c` (claim),
`t` (test success), `v` (review PASS), `i` (integrate), `a` (adopt),
`b` (rollback), `r` (refresh), `q` (quit).

## Freeze / verify

```sh
scripts/verify-v041-w1-cross-client-journey.sh "$ROOT"
```

The verify script checks the frozen evidence schema, journey-id drift,
dual-client IPC (gui + tui) with production app rows, TUI transcript
presence, screenshots, SQLite integrity, zero duplicate Event IDs /
idempotency keys, zero stream gaps, projection-journal agreement, empty
postflight arrays, socket/lock cleanup, and secret hygiene.

## Consumed journey

`/private/tmp/v041-w1-journey`, journey_id
`105723c8-0595-4f6d-9890-0b411f16cb01`, verified PASS. This is a consumed
journey root: it is evidence, not a re-runnable fixture. A re-run requires
an independent Review PASS and a new journey_id/root.
