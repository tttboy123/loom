# P3A-W1 Live Stack Smoke Validation (daemon + IPC + PTY TUI)

Date: `2026-08-04`

Status: `EVIDENCE — NON-GUI PRODUCTION STACK VALIDATED LIVE`

## What was validated

A real controlled journey daemon was started from a prepared journey root
(`/private/tmp/loom-p3a-happy3-…`, scenario
`happy-create-evaluate-activate-bind-execute-clean`, journey UUID
`b79b9c98-bb9d-444a-a04f-d82716bf7257`) with:

- `LOOM_P3A_CONTROLLED_JOURNEY_MANIFEST` harness (state/socket/evidence
  binding, fault none);
- the locked Pi fixture (`pi` 0.82.1, node-v24.16.0, llama-b10107,
  qwen2.5-coder-1.5b GGUF; all four locked digests verified);
- the reviewed local-model private root (`phase1-live`);
- a real PTY `loom app --socket` TUI driven with real keystrokes
  (Tab×8 Board → … → Evolution Assets, then `q`).

Observed:

1. Daemon starts and serves the Unix socket; the harness wrote
   `ipc/request-response-summary.jsonl` and `daemon/structured-log.jsonl`
   with the journey UUID on every record.
2. The production Swift contract probe (`LoomLocalAppContractProbe
   --assets JOURNEY`) read the authoritative `evolution_asset_snapshot`
   through the daemon (empty view, `viewVersion` returned) with exact
   request/response digests and `client_kind=gui`.
3. The production PTY TUI connected, rendered all screens and reached
   `Evolution Assets` (showing the journey UUID); its `snapshot` and
   `evolution_asset_snapshot` calls are recorded in the IPC/daemon logs as
   `client_kind=tui`.
4. Clean shutdown removed `loomd.sock` and `loomd.sock.lock`; postflight
   shows zero processes/sockets/leases/temps and no open handle on the
   state lock (closed state-lock file per runbook).

## Defects found and fixed (owned files, regression-tested)

1. `cmd/loomd/product_daemon.go` `productJourneyPathWithin` rejected a
   journey root equal to the evidence root (relative path `.`), so every
   prepared journey root failed daemon startup with `build_ipc`. Fixed to
   accept the root itself; regression assertion added in
   `TestP3AControlledJourneyHarnessWritesSanitizedRequestAndDaemonLogs`.
   The live daemon start above passes with the fix.

Observation (not fixed): `validDaemonBuildFailureReason` does not list
`build_assets`, so asset-build failures surface as `build_unknown`. Fixing it
would require editing unowned `cmd/loomd/run.go`/`run_test.go`; a bounded
owned-path amendment would be needed. Left as a documented diagnostic
limitation to keep the Candidate inside the reviewed owned-path allowlist.

Operational notes: journey roots must be under `/private/tmp` (not `/tmp`,
which is a symlink and fails the private-path/EvalSymlinks checks); the
local-model private root must contain the llama executable and model
(`phase1-live`), matching the locked component manifest.

This change post-dates the final Implementation Review PASS and is covered by
the current unit suite and this live smoke evidence; it must be included in
the Whole-Candidate Review scope.
