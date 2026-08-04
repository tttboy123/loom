# SF-W1 cross-client journey

This runbook is the cross-client Exit Gate journey for the single `SF-W1`
(Queue State/Projection + Admission/Eligibility/Conflict Arbiter). It follows
the accepted alternative-verification method (Product Owner instruction
2026-08-04): production native window launched with `--socket --journey-id`,
real PTY TUI, and mutations/reads through the production Swift client over
the real daemon socket; Computer-Use-driven window automation is skipped.

## Non-negotiable boundary

Every scenario uses:

```text
real native Loom window + production Swift client
real PTY + production Bubble Tea model
→ production IPC clients
→ one real private Unix socket
→ production daemon handler and application services
→ the single Event Journal (AppendBatchIfStreamHeads CAS)
→ queue projection rebuilt from the Journal
```

No AppleScript, ViewModel injection, direct service call, direct Projection
or SQLite mutation, visual-only fixture, Preview, launch-only proof or
accessibility-tree-only proof may cause product behavior. The GUI evidence
surface follows the frozen `P3A-W1-GUI-EVIDENCE-SURFACE-AMENDMENT.md`
(app launch reads, checkpoint-bound screenshots, production Swift client
records; no claim that the main-window pixels differ per journey). The
journey drives no execution, no Pi/llama process, no Provider, no network
and no credential access; Runtime discovery is read-only against the locked
fixture.

## Scenario (single, frozen)

`sf1-queue-admission-conflict-gap` — user queues two Jobs sharing one owned
path; both clients observe admission and the Conflict Arbiter serializing
the second at dispatch; a third self-dependent (DAG-cycle) Job is rejected
with a visible typed `denied` error and zero Journal facts; one authorized
failed-Run observation produces exactly one digest-bound `GapProposal`
(`gap_id`, disposition `observe`); a duplicate observation converges on the
same `gap_id` (`merge_duplicate`, zero new facts); the daemon is restarted
mid-journey and both clients observe the rebuilt queue projection (2 Jobs /
1 Gap) through their own production reads with no duplicate facts.

## Prepare the root

Build the reviewed root-local `loom`, `loomd` and native App artifacts from
the final source first. Create a private root (0700) under `/private/tmp`,
freeze the journey UUID (UUID v4) and the binary/source-lock digests in
`manifest/journey-context.json`, then start the production daemon:

```sh
LOOM_P3A_CONTROLLED_JOURNEY_MANIFEST=/ROOT/manifest/journey-harness.json \
  /ROOT/bin/loomd \
  --state /ROOT/state/loom.db \
  --isolation-root /ROOT/isolation \
  --runtime-dir /REVIEWED/PI/BIN \
  --runtime-dir /REVIEWED/NODE/BIN \
  --probe-id sf1-JOURNEY \
  --instance-id runtime.pi.earendil-works.0.82.1 \
  --device-id local-mac \
  --display-name 'Pi 0.82.1' \
  --interval 1s \
  --process-timeout 10s \
  --socket /ROOT/loomd.sock \
  --local-model-private-root /REVIEWED/LOCAL-MODEL-ROOT \
  --local-model-executable /REVIEWED/llama-server \
  --local-model-path /REVIEWED/model.gguf
```

Start the real PTY with append recording:

```sh
LOOM_JOURNEY_ID=JOURNEY \
  /usr/bin/script -aq /ROOT/tui/transcript.txt \
  /ROOT/bin/loom app --socket /ROOT/loomd.sock
```

Navigate to the Queue screen with Tab (ScreenQueue is the last screen after
Evolution Assets), refresh with `r`, quit with `q`. Every real key is
recorded in `tui/keystrokes.jsonl` (sequence, monotonic_offset_micros, key,
screen_id, expected/observed visible state).

Start the root-local native executable with
`--socket /ROOT/loomd.sock --journey-id JOURNEY`. The app's production
`LocalIPCClient.defaultClient()` performs the launch reads
(`setup_snapshot` + `snapshot` with `loom-swift-<uuid>` request IDs) against
the same socket. Drive the queue actions through the production Swift client
via `LoomLocalAppContractProbe` over the real socket:

```sh
/ROOT/probe --socket /ROOT/loomd.sock --journey-id JOURNEY \
  --queue-create-job JOURNEY /ROOT/inputs/job-a.json
/ROOT/probe --socket /ROOT/loomd.sock --journey-id JOURNEY \
  --queue-create-job JOURNEY /ROOT/inputs/job-b-shared-path.json
/ROOT/probe --socket /ROOT/loomd.sock --journey-id JOURNEY \
  --queue-create-job JOURNEY /ROOT/inputs/job-c-dag-cycle.json
/ROOT/probe --socket /ROOT/loomd.sock --journey-id JOURNEY \
  --queue-gap-observe JOURNEY /ROOT/inputs/gap-observe.json
/ROOT/probe --socket /ROOT/loomd.sock --journey-id JOURNEY \
  --queue-gap-observe JOURNEY /ROOT/inputs/gap-observe.json   # duplicate
/ROOT/probe --socket /ROOT/loomd.sock --journey-id JOURNEY \
  --queue-snapshot JOURNEY                                     # cross-observe
```

After each action capture a real window screenshot with `screencapture`
(0600 under `gui/screenshots/`) and append the exact action record
(sequence, monotonic_offset_micros, action, control_id, input_digest,
expected_visible_state, observed_visible_state,
`screenshot_relative_path`) to `gui/actions.jsonl`.

## Restart/reconnect checkpoint

Quit the TUI with its real `q` key, close the native process, stop the
daemon cleanly, then start the daemon again on the same state path with the
same flags. Reconnect the PTY TUI (same transcript) and the native app (or
the production Swift client probe `--queue-snapshot`); both clients must
observe the rebuilt 2-Job / 1-Gap queue projection with no duplicate facts.
The daemon structured log continues sequence numbers monotonically across
the restart; per-instance `monotonic_offset_micros` restart at the second
daemon's own uptime.

## Evidence package and verification

The root carries the frozen §8 evidence bundle (0700 root, 0600 files):
`manifest.json` (canonical evidence list with SHA-256/size/mode and
`manifest_digest`), `result.md`, `gui/actions.jsonl` + screenshots,
`tui/transcript.txt` + `keystrokes.jsonl`, `timeline.jsonl`,
`ipc/request-response-summary.jsonl`, `daemon/structured-log.jsonl`,
`journal/event-summary.json`, `journal/stream-heads.json`,
`journal/sqlite-summary.json`, `projection/summary.json`,
`artifacts/digest-verification.json`, `processes/preflight.json`,
`processes/postflight.json`, `processes/cleanup-proof.txt`,
`source/source-lock.json`, `source/runtime-fixture.json`,
`manifest/journey-context.json`, `manifest/journey-harness.json`,
`manifest/preflight-source-lock.json` and `manifest/tui-plan.json`.

Final verification (read-only, fail-closed):

```sh
scripts/verify-sf1-cross-client-journey.sh /ROOT
```

The verifier fails closed on missing GUI or PTY evidence, wrong modes or
digests, invalid journey identity, non-PASS result axes, SQLite
integrity/uniqueness/stream gaps, projection mismatch, journey drift,
missing GUI/TUI IPC traffic, missing production-app launch reads, secret-like
text, or socket/lock/process residue.
