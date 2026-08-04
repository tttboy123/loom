# v0.4.0 Agent Scheduling Framework — Whole-Slice Acceptance Review

Date: `2026-08-05`

Scope: the complete v0.4.0 slice (P3A-W1 baseline `7d5f0b01`; Gate 1
governance `737c58c8`; SF-W1 `1abf033c`; SF-W2 `36b6c4b8`; SF-W3
`8e888565` + repair `ad9726a5`), audited against the Product Owner goal's
minimum acceptance and stop conditions.

## Verdict

```text
Minimum acceptance: 12/12 PASS
Stop conditions: no trigger
P0 = 0  P1 = 0  P2 = 0
Overall v0.4.0 Whole-Slice Acceptance: PASS
```

## Item-by-item acceptance audit (evidence)

1. **Two independent WorkItems truly parallel** — SF-W2 journey: two
   independent workers claimed two separate Candidates in parallel
   (`/private/tmp/sf2-journey-final`, 2 active workers observed in both
   clients); the slice also ran independent review chains in parallel.
2. **Test Executor independent before Development completes** — SF-W2 test
   lane claim + `TestSF2WorkersWireOverSocket`; the failing `test_defect`
   result was recorded independently of the Developer worker.
3. **Test failure enters Repair by one of seven classes** — SF-W2 journey:
   `test_defect` failure recorded → Repair routing (never Integration).
4. **Worker crash reclaimed by lease; state rebuilt from Journal** — SF-W2:
   after-CAS crash (`crash_seam=after_cas`) reclaimed with exactly one new
   generation; restart/reconnect rebuilt the identical state.
5. **Stale generation rejected with zero side effects** — SF-W2:
   `StaleResultRejected` recorded; no effect.
6. **Same mutex / same owned path never parallel** — SF-W1 Conflict Arbiter
   (shared-owned-path jobs serialized) + SF-W2 one-claim-per-worker pool
   capacity (no oversell).
7. **Reviewer cannot write; Integrator single writer** — SF-W2
   (`review_write_denied` → `denied`) + SF-W3 (competing integration loses
   via CAS; one `CandidateIntegrated`).
8. **Queue state fully rebuildable** — SF-W1 restart/reconnect rebuilt the
   identical 2-Job/1-Gap state; SF-W2/SF-W3 rebuild checkpoints passed.
9. **Controlled canary: no duplicate Run/Evidence/effect; no oversell;
   projection failure preserves old view; crash recovery correct** — SF-W3
   (canary one-shot, duplicate blocked; release/timeline restart-rebuild;
   projection-failure preserve demonstrated in repair `ad9726a5`);
   SF-W2 (capacity never oversold).
10. **Streaming node output authorized + generation-bound; unauthorized/
    stale/malformed not published** — SF-W3 (`unauthorized`/`stale`/
    `malformed` frames rejected with zero effects; authorized frame
    published to Timeline/Attention).
11. **GUI+TUI real journeys per WorkItem + dual Result PASS P0=P1=P2=0** —
    SF-W1/2/3 each have a frozen alternative-verification journey
    (production Swift client + real PTY TUI + native window + restart
    checkpoint) with Product Result and Operational/Trace review records
    PASS.
12. **Gap-to-successor minimums** — SF-W1: one digest-bound Gap Proposal per
    authorized source, duplicate convergence on one gap_id, read-only
    successor compilation, no WorkItem side effect from discovery.

## Governance chain (all PASS)

Gate 0 identity + prerequisite audit; Gate 1 combined Contract Review
(Review-1 FAIL → Contract Repair 1 → Review-2 PASS); per-WorkItem
implementation/dual-Result/whole-candidate review records; exact staging per
each WorkItem source-lock; exclusions (incl.
`internal/projection/team_execution_test.go`) untouched; one atomic local
commit per WorkItem; no push/merge/network/paid/user-config action.

## Stop-condition check (no trigger)

Identity consistent (cwd==top-level, branch `codex/loom-platform-slice2`);
P3A baseline accepted and committed; no unreviewed authority/schema/
credential expansion; Gate 1 closed within exactly three vertical WorkItems;
no thin WorkItem; no Review FAIL; worktree dirtiness is only the documented
exclusion set.

VERDICT: `PASS`
