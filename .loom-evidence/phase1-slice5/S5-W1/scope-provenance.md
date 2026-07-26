# S5-W1 Scope Provenance

Date: 2026-07-26
Baseline: `006db8c`
Candidate: S5-W1 only

## Shared-worktree provenance

The committed Slice 4 whole-Slice review at
`.loom-evidence/phase1-slice4/WHOLE-SLICE-REVIEW-1.md` records that unrelated
worktree changes remained outside the Slice 4 committed chain. Those same
user-owned paths are still present:

- `AGENTS.md`
- `PROGRESS.md`
- `.codex/**`
- `.loom-drafts/**`
- `.loom-evidence/phase1-slice3/POST-S3-W5-QUEUED-CONTRACT-INPUTS.md`

Their presence in `git status` is not evidence that S5-W1 owns them. They
remain excluded from the Candidate and must remain unstaged and uncommitted.
No cleanup, revert, deletion, or adoption of those paths is authorized.

## Exact S5-W1 Candidate

The Candidate consists only of:

- `cmd/loom/main_test.go`
- `cmd/loom/query.go`
- `docs/CURRENT.md` (S5-W1 Controller status only)
- `internal/api/team_execution_stream.go`
- `internal/api/team_execution_stream_test.go`
- `internal/app/team_execution.go`
- `internal/app/team_execution_test.go`
- `internal/app/team_execution_stream_test.go`
- `internal/journal/journal_test.go`
- `internal/journal/store.go`
- `internal/projection/global_read_view.go`
- `internal/projection/global_read_view_test.go`
- `.loom-evidence/phase1-slice5/**`

The Git index was empty when this provenance was captured. Any eventual
pre-commit staging must name only the paths above, inspect the cached diff,
and prove that every user-owned path remains absent from the index.

## Review rule

Fresh review must assess the exact Candidate ownership set above against the
frozen contract. A dirty shared worktree is not itself a scope violation;
adopting, editing for S5-W1, staging, or committing an excluded path would be.

VERDICT: PASS
