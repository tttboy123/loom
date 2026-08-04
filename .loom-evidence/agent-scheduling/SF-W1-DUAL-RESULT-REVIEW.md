# SF-W1 Dual Result Review (PASS)

Date: `2026-08-04`

Scope: the SF-W1 cross-client journey root `/private/tmp/sf1-journey-final`
(journey `70862374-d9cd-4ac6-a02f-2be68344bdac`), Product Result and
Operational and Trace Behavior, independent read-only verification.

## Verdict

```text
P0 = 0
P1 = 0
P2 = 0
Product Result: PASS
Operational and Trace Behavior: PASS
```

## Product Result (verified)

- Journal contains exactly 2 `QueueJobCreated` + 2 `QueueJobAdmitted` + 1
  `GapProposalCreated` and ZERO events for the rejected DAG-cycle job.
- Two Jobs sharing `internal/queue/model.go` are both admitted with the
  Conflict Arbiter serializing the shared path at dispatch (visible in the
  Queue Projection and both clients).
- One authorized failed-Run observation produced exactly one digest-bound
  `GapProposal` (gap_id `48054c36…`, disposition `observe`); a duplicate
  observation converged on the same `gap_id` (`merge_duplicate`, zero new
  facts).
- Projection `jobs=2 gaps=1 successors=0` with `matches_journal=true`;
  `result.md` truthfully states all assertions; daemon/IPC logs
  corroborate every action.

## Operational and Trace Behavior (verified)

- §8-style evidence schema: gui/actions.jsonl, tui/keystrokes.jsonl,
  timeline.jsonl, ipc/request-response-summary.jsonl,
  daemon/structured-log.jsonl, processes pre/post, projection summary and
  artifacts digest verification with the frozen keysets; all JSONL valid
  and non-empty.
- Zero journey drift: every daemon and IPC record carries journey
  `70862374-d9cd-4ac6-a02f-2be68344bdac`.
- Dual-client coverage: 9 gui + 8 tui IPC records with ≥2 app-originated
  `loom-swift-<uuid>` rows (production app connected to the daemon).
- Real PTY transcript contains "Loom ·" (9 markers) and the Queue screen
  with both job IDs (2 sessions, including the post-restart reconnect);
  real 0600 screenshot present.
- Postflight processes/sockets/locks/leases/temps all empty with
  cleanup-proof; no socket/lock residue; 0700/0600 permissions; secret
  hygiene clean.
- Restart/reconnect checkpoint: after the daemon restart the Queue
  Projection rebuilt from the Journal and both the probe snapshot and the
  reconnected TUI observed the identical 2-Job/1-Gap state with no
  duplicates.
- `scripts/verify-sf1-cross-client-journey.sh` returns PASS.

VERDICT: `PASS`
