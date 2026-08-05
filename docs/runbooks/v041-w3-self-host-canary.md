# v0.4.1-W3 self-host canary — repeatable controlled canary runbook

This runbook freezes the controlled self-host canary as a repeatable product
function for `W3` of the frozen `V0.4.1-CONTRACT.md`. It uses the accepted
v0.4.0 SF-W3 canary mechanism (`internal/integration/canary.go`) through the
production Swift client over the real daemon socket, with the real PTY TUI
and the real native window as observers (alternative-verification method;
Computer-Use-driven window automation is skipped). W3 adds no product code.

## Scenario assertions (frozen)

1. **One offline Run with locked bindings** — `integration_command
   start_canary` with exact `run_id` / `runtime_instance_id` / `model_id` /
   `skill_digest` bindings records exactly `CanaryStarted` +
   `CanaryCompleted` for the run. The canary is offline: no network, no
   provider, no model call; it is a Journal-authority fact with a
   deterministic evidence digest.
2. **Idempotent CAS** — a duplicate start of the same run is rejected with
   `conflict` and zero new events (no duplicate Run/Evidence/effect).
   Capacity is 1 run per canary stream, so no oversell is possible.
3. **Projection-failure preserves the old view** — with the controlled
   projection fault active (`integration_command projection_failure_test
   {"active":true}`), a second canary is committed to the Journal but the
   integration snapshot keeps returning the old view; after the fault is
   cleared (`{"active":false}`), the rebuilt view includes the new run.
4. **Crash recovery correct** — the daemon restarts mid-journey and the
   canary state rebuilds from the Event Journal with no duplicate facts;
   both clients observe the identical state.
5. **Repeatable** — the verify script (`scripts/
   verify-v041-w3-self-host-canary.sh`) asserts the exact bindings, the
   idempotent rejection, the projection-fault client operations and the
   rebuild; a fresh journey_id/root can re-run the demonstration.

## Driving commands

```sh
PROBE=apps/macos/.build/arm64-apple-macosx/release/LoomLocalAppContractProbe
$PROBE --socket ROOT/loomd.sock --integration-command JOURNEY start_canary canary1.json
$PROBE --socket ROOT/loomd.sock --integration-command JOURNEY start_canary canary1-dup.json
$PROBE --socket ROOT/loomd.sock --integration-snapshot JOURNEY
$PROBE --socket ROOT/loomd.sock --integration-command JOURNEY projection_failure_test fault-on.json
$PROBE --socket ROOT/loomd.sock --integration-command JOURNEY start_canary canary2.json
$PROBE --socket ROOT/loomd.sock --integration-snapshot JOURNEY   # old view while fault active
$PROBE --socket ROOT/loomd.sock --integration-command JOURNEY projection_failure_test fault-off.json
$PROBE --socket ROOT/loomd.sock --integration-snapshot JOURNEY   # rebuilt view
```

Canary input bindings (frozen for the demonstration):

```json
{"run_id":"v041-w3-canary-1",
 "runtime_instance_id":"runtime.pi.earendil-works.0.82.1",
 "model_id":"loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m",
 "skill_digest":"0795cedc586127a14c03361e864d7a7d694cabdde518acc5afebc3bb1e965f73"}
```

The daemon runs with the frozen runtime-dir / probe-id / local-model trio
and the harness manifest purpose `phase3a-cross-client-e2e`; the native
window is launched with `open ROOT/app/Loom.app --args --socket
ROOT/loomd.sock --journey-id JOURNEY` and captured with `screencapture`;
the real PTY TUI observes the Integration screen.

## Freeze / verify

```sh
scripts/verify-v041-w3-self-host-canary.sh ROOT
```

## Consumed journey

`/private/tmp/v041-w3-journey`, journey_id
`fbae6a98-3a88-44f7-8d65-67b62626d55b`, verified PASS. This is a consumed
journey root: it is evidence, not a re-runnable fixture. A re-run requires
an independent Review PASS and a new journey_id/root.
