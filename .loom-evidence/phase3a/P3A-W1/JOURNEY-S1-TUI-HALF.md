# P3A-W1 Journey Scenario 1 — TUI Half (real PTY)

Date: `2026-08-04`

Status: `PARTIAL JOURNEY — TUI HALF COMPLETE, GUI HALF PENDING`

## Journey root (frozen, continue from here)

```text
root:       /private/tmp/loom-p3a-s1-1785820678
journey_id: abd7a03e-e3ab-461d-9318-6487f76b9887
scenario:   happy-create-evaluate-activate-bind-execute-clean
binaries:   loom/loomd (root-local, digests frozen in manifest)
app:        root-local signed Loom.app
```

## What the real PTY TUI did (production stack, journey-correlated)

1. Connected to the production daemon over the private socket; Board →
   New Mission → Mission → Team Builder → Runs → Compare → Attention →
   Team Timeline → Evolution Assets navigation.
2. Created a local Skill Candidate from
   `source/s.md` (`n` → absolute path → enter):
   - `EvolutionAssetDefinitionCreated`
   - `EvolutionAssetRevisionCreated`
   - `EvolutionAssetCandidateCreated`
   - IPC: `evolution_asset_command create_skill response_ok=true`
   - Probe read: `definitions=1 revisions=1 candidates=1` with new viewVersion.
3. Built and saved a Team (`JourneyTeam`; Main Coordinator + Subagent Bounded
   Worker, local model `loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m`,
   compatible, no execution):
   - `TeamDefinitionSaved`, `TeamInstanceCreated`, `AgentInstanceCreated`.

Daemon observer ran 103 cycles with 1 discovery event and 102 no-write
cycles; clean shutdown removed `loomd.sock`/lock; postflight processes,
sockets, leases and temps are empty.

## Evidence in the root

```text
tui/transcript.txt                9,359 bytes (real PTY, all screens)
tui/keystrokes.jsonl              21 records, journey_id bound
ipc/request-response-summary.jsonl  (tui snapshot/asset reads + create_skill)
daemon/structured-log.jsonl         (journey_id on every record)
journal/…                          (events above; freeze script produces summaries)
processes/preflight.json / postflight.json
state/loom.db                      (7+ events, integrity ok)
```

Non-GUI evidence package completed on `2026-08-04` (result.md PARTIAL draft,
timeline.jsonl 57 records, projection/summary.json via production probe,
artifacts/digest-verification.json 1/1 match, processes pre/postflight,
cleanup-proof). The GUI-capable session only needs to add GUI evidence and
run freeze/verify on this root.

## GUI half pending (Alternative Verification per Product Owner direction)

Continue the SAME root and journey UUID:

1. restart the daemon with the frozen manifest/binaries (same args as
   `LIVE-STACK-SMOKE-VALIDATION.md`),
2. launch the root-local app with `--journey-id abd7a03e-…`,
3. drive the GUI side through the production Swift client
   (`LocalIPCClient`/`LocalProductStore` over the real socket): evaluate the
   Candidate (record_evaluation with a content-addressed fixture artifact),
   activate, bind to the saved Team, preflight and start a Mission, observe
   materialization/execution/cleanup; capture real window screenshots with
   `screencapture` at checkpoints,
4. cross-observe at least one GUI mutation in TUI and one TUI mutation in GUI,
5. produce `gui/actions.jsonl` + screenshots, then
   `scripts/run-phase3a-cross-client-journey.sh freeze --product-result PASS
   --trace-result PASS` and `verify`.

Failure of the GUI half keeps this root `PARTIAL`; a replacement requires a
new journey ID and root after Implementation Re-review (Alternative
Verification Amendment §6-§7).
