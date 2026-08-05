# Gate 0 Audit — B-P1 Execution Permission Pipeline

Date: 2026-08-05

## 1. Repository identity verification

| Check | Command | Result |
|---|---|---|
| physical cwd | `pwd` | `/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild` |
| Git top-level | `git rev-parse --show-toplevel` | identical to cwd |
| branch | `git branch --show-current` | `codex/loom-platform-slice2` |
| HEAD | `git rev-parse HEAD` | `a6806751` |
| docs/CURRENT.md record | tail section | formal product documentation published at `a6806751`; previous v0.4.1 slice accepted |
| staging state | `git diff --cached --name-only` | empty (nothing staged) |

Result: identity check PASS — physical cwd, Git top-level, branch and HEAD agree
with `docs/CURRENT.md`.

## 2. Dirty / untracked boundary (exclusion list)

Pre-existing dirty files (untouched by this slice):

- Root docs: `AGENTS.md`, `PROGRESS.md`, `README.md`
- `.loom-evidence/phase1-final-live-gate/*` (historical July content)
- Untracked: `.codex/**`, `.loom-drafts/**`, `apps/macos/.build/**`

New this slice (planned governance + evidence, to be committed only at its own
atomic gates): `.loom-evidence/execution-permissions/*`.

Product code lock: `git diff HEAD` over product paths (excluding the above and
`internal/projection/team_execution_test.go`) is empty at Gate 0.

## 3. Prerequisite capability spot-verification (against existing accepted code)

| Capability | Evidence |
|---|---|
| Event Journal authority + `AppendBatchIfStreamHeads` | `internal/journal/store.go` (Append/AppendBatch/AppendBatchIfStreamHeads + ErrStreamHeadConflict) |
| Rebuildable projection pattern | `internal/queue/projection.go` (Replay), `internal/projection/queue.go` wrapper |
| Lease/generation fencing | `internal/work/run_authority.go` (claim lease, generation) |
| waiting_approval lifecycle | `internal/projection/approval.go`, `internal/rules/authority.go` |
| AgentGrant authority pattern | `internal/authorization/authority.go` (store-owned authority, CAS append) |
| strict IPC surface | `internal/localipc/protocol.go` (method allowlist), `cmd/loomd/product_daemon.go` dispatch |
| Native client read surface | `apps/macos/Sources/LoomLocalAppCore/LocalIPCClient.swift`, `LocalQueueModels.swift` |
| Alternative GUI verification substrate | accepted P3A-W1 alternative-verification + GUI-evidence-surface amendments |

Result: all prerequisites present on the accepted baseline; no new dependency,
credential, network or paid action required.

## 4. Scope confirmation

This slice lands exactly one vertical WorkItem (B-P1 Execution Permission
Pipeline) per the frozen Gate 1 contract. B (execution adapter) and C
(production landing) are explicitly out of scope and will be re-analyzed after
B-P1 acceptance.
