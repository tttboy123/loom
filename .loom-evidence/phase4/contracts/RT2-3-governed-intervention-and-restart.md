# RT2-3 Governed Intervention And Restart

Status: `ACCEPTED / INSTALLED BUILD 211`

## User outcome

A Mission RoundTable no longer becomes inert while Agents are running or after
one seat fails. The user can pause the round, steer a running Agent, retry a
failed or cancelled seat, skip a seat, or replace its frozen role selection.
Every control produces visible state and survives restart.

## Authority boundary

- Pause, Steer, Retry, Skip, Replace, Cancel and dispatch failure are versioned,
  idempotent RoundTable facts.
- Journal facts contain only identities, digests, safe stage/code metadata and
  timestamps. Guidance, prompt, Provider body and visible output are excluded.
- Retry creates a new Attempt, Segment, Run and Context Capsule. The prior
  Attempt and its Incident remain immutable.
- Replace increments membership revision exactly once and resolves the new
  binding server-side.
- Restart terminalizes stale running Attempts as cancelled. It never silently
  resumes Provider work.

## Source gates

- `go test ./internal/roundtable`
- focused daemon route, partial-result, pause, retry and restart tests
- strict Swift view/request/error decoding
- Mission UI exposes Pause, Steer, Retry, Skip, Replace and intervention history

Installed acceptance covers Pause, independent Retry, Skip/Replace authority,
restart reconciliation and live Steer. Build 208 fixed a capability-loss defect
in the product Runtime wrapper: Loom Native now freezes an 8-turn/16-step input
budget, admits and consumes Steer into Step 2, while OpenCode reports a
non-retryable `capability_gap` in seconds instead of waiting for readiness it
cannot provide. Build 211 removes Harness-name inference entirely: the daemon
projects `pending`, `available` or `unavailable` from the exact active Attempt,
and the macOS workbench enables Steer only for `available`. The daemon also
enforces the same 4,096-byte guidance boundary as the UI and Retry path.
