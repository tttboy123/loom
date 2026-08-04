# v0.4.0 Agent Scheduling Framework — Gate 0 Audit

Date: `2026-08-04`

Status: `GATE 0 AUDIT — PASS / READY FOR GATE 1 FREEZE`

Goal authority: Product Owner queued route input (`Agent Scheduling Framework /
并发开发流水线`, pasted-text-1.txt) with Phase 3A accepted commit as the only
baseline; reconciled with
`2026-08-01-interaction-handoff-auth-routing-scheduling-queued-input.md`
(`v0.4.0` release placement) and
`2026-08-04-governed-self-evolution-pipeline-queued-input.md` (gap-to-successor
closure folded into the same three vertical boundaries).

## 1. Repository identity (must agree before any write)

| Check | Result |
|---|---|
| physical cwd | `/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild` |
| Git top-level | `/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild` — identical |
| branch | `codex/loom-platform-slice2` |
| HEAD | `7d5f0b01d5675820f01def08b3888f56e99e841a` |
| docs/CURRENT.md record | `P3A-W1 = ACCEPTED / LOCAL ATOMIC COMMIT COMPLETE` (committed at HEAD) and the P3A-W1 commit is recorded as `7d5f0b01` (worktree record, to be committed with the next slice) |

The identity check passes: cwd == top-level == the repository named in the goal;
the P3A-W1 accepted commit is `7d5f0b01` and is recorded in `docs/CURRENT.md`
per the goal's "具体 commit 以 docs/CURRENT.md 记录为准".

## 2. Phase 3A accepted and atomically committed

- `7d5f0b01` contains the entire P3A-W1 candidate: 102 staged paths (59 changed
  or new source paths from the frozen `source-lock.json` inventory plus the 43
  `.loom-evidence/phase3a/` governance/evidence files), one local commit, no
  push/merge.
- The accepted evidence chain is committed under `.loom-evidence/phase3a/`:
  Entry Amendment, Gate 1 Contract (ADR-0013 + Phase 3A Exit Contract +
  P3A-W1 contract, Repair 2 Re-review PASS), Implementation Review chain
  (P0=P1=P2=0), Alternative Verification Amendment (PO instruction
  2026-08-04), GUI Evidence Surface Amendment (Review 4 PASS, P0=P1=P2=0),
  eight final v5 `-r2` cross-client journey roots all re-verified PASS by
  `scripts/verify-phase3a-cross-client-journey.sh`, dual Result reviews
  (Product Result V6 PASS, Operational/Trace final-generation chain PASS),
  Whole-Candidate Review PASS (P0=0 P1=0 P2=2) with both documentation
  closures applied (dual-result closure addendum; `OPERATIONAL-TRACE-REVIEW.md`
  marked `SUPERSEDED — HISTORICAL`).
- `internal/projection/team_execution_test.go` and the documented non-product
  paths remain excluded from the commit exactly as frozen.

## 3. Dirty / untracked boundary — precise exclusion list

After the atomic commit the worktree remains dirty only in the documented
exclusion set (verified by `git status --short`):

| Class | Paths |
|---|---|
| Pre-existing root docs (excluded) | `AGENTS.md`, `PROGRESS.md`, `README.md` |
| Pre-existing excluded test | `internal/projection/team_execution_test.go` (modified, never staged) |
| Phase 1 gate evidence (excluded) | `.loom-evidence/phase1-final-live-gate/**` (3 modified + untracked canary records) |
| Earlier-slice evidence (excluded) | `.loom-evidence/phase1-slice3/**` (1 queued-input file), `.loom-evidence/phase2c/**`, `.loom-evidence/plan-amendments/**` |
| User/Codex configuration and drafts (excluded) | `.codex/**`, `.loom-drafts/**` |
| Generated builds (excluded) | `apps/macos/.build/**` |
| Current-slice documentation record (owned next-slice) | `docs/CURRENT.md` (records `7d5f0b01`; carries into the v0.4.0 Gate 1 commit) |

No excluded path is staged; no candidate path is missing. The exclusion list is
precise and complete for the next work phase.

## 4. Prerequisite capability audit

| # | Prerequisite | Status | Evidence |
|---|---|---|---|
| 1 | Event Journal unique authority + `AppendBatchIfStreamHeads` | DONE | `internal/journal/store.go:256`; CAS tests in `internal/journal/journal_test.go`; P3A journals replayed and audited (SQLite integrity, zero duplicate ids/idempotency keys, zero stream gaps) |
| 2 | Projection / GlobalReadView rebuildable from Journal | DONE | `internal/projection/global_read_view.go`; P3A S7 `projection-failure-rebuild-reconnect` PASS (old view preserved pre-restart, `matches_journal=true` post-restart) |
| 3 | Dispatch CAS (single winner) | DONE | Asset authority CAS + `AppendBatchIfStreamHeads`; P3A S4 `concurrent-single-winner` PASS (exactly one CAS winner, loser `conflict`) |
| 4 | Lease / generation fencing | DONE | `internal/work/run_authority.go` (`ErrRunLeaseActive/Expired`, `ErrStaleClaimGeneration`, `claim_generation`); P3A S3 stale_view/digest rejection, S5/S6 crash seams (exit 92/93) with zero/one committed effect |
| 5 | Grant / Evidence lineage | DONE | `internal/projection/grant_authority.go`, `internal/work/verification_authority.go`, P3A artifact/evidence digest lineage, P2B side-task lineage |
| 6 | Independent worktree / path isolation | DONE (substrate) | P3A journey isolation roots with fail-closed `recoverProductJourneyIsolationRoot`; branch/worktree discipline for Candidates; per-Candidate owned-path contracts |
| 7 | Failure classification + `human_required` lane | DONE (substrate) | `human_required`/`blocked`/`degraded` lanes in `internal/work/team_execution_authority.go`, `internal/work/side_task_handoff.go`, `internal/app/local_product_execution.go`; `test_defect`/`product_defect` classifications recorded in `docs/CURRENT.md`; the v0.4.0 Exit Contract freezes the exact seven-class taxonomy (Gate 1 deliverable, see §5) |
| 8 | Real GUI/TUI cross-client substrate | DONE | Frozen Alternative Verification Amendment + GUI Evidence Surface Amendment: production native window launched with `--socket --journey-id`, real PTY TUI, production Swift client over the real socket, 8/8 journey roots verified PASS |
| 9 | Runtime capability matrix | DONE | `internal/runtime/catalog.go` (`RuntimeProfile`, `ValidateBinding`, capability checks); frozen canary fixture `phase1-live` (Pi 0.82.1 SHA `af302f23…`, node v24.16.0, llama b10107, qwen2.5-coder-1.5b, `--offline`/`--no-approve`) |

No key prerequisite is MISSING. Item 7's exact seven-class taxonomy is
deliberately frozen at Gate 1 (the goal's Exit Contract requirement), not a
Gate 0 blocker.

## 5. Gate 0 verdict and next step

```text
Identity: PASS (cwd==top-level, branch codex/loom-platform-slice2,
           HEAD 7d5f0b01 recorded in docs/CURRENT.md)
Phase 3A baseline: PASS (P3A-W1 accepted, atomically committed)
Dirty boundary: PASS (precise exclusion list, nothing staged)
Prerequisites: PASS (9/9 DONE — item 7 taxonomy freezes at Gate 1)

GATE 0 = PASS
```

Next step (Gate 1, before any product write): freeze the combined v0.4.0
governance set and obtain a fresh independent Contract Review PASS:

1. ADR-0014 — Agent Scheduling Framework (single authority architecture;
   explicitly forbids a second Scheduler/Journal/StateWriter/Projection/queue
   DB).
2. v0.4.0 Exit Contract — lane state machine + seven-class failure taxonomy,
   lease/fencing, fairness (repair aging, no starvation), integration
   single-writer, controlled self-host canary, atomic-commit scope, authority
   boundaries, Decomposition Compiler rules, RED scenarios, verification
   matrix, exact staging; reconciles the self-evolution queued input inside
   the same three vertical WorkItems (no SF-W4, no thin WorkItem).
3. SF-W1 / SF-W2 / SF-W3 WorkItem freeze (exact owned files, schemas, RED,
   per-WorkItem GUI+TUI journeys).
4. Observability (streaming node output) and bounded-recovery ownership frozen
   explicitly (goal default: fold into SF-W2/SF-W3 boundaries).

Product code remains locked until the combined Contract Review PASS.

VERDICT: `GATE 0 PASS`
