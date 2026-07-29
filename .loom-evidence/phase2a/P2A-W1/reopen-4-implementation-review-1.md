# P2A-W1 Reopen 4 Implementation Review 1

**Date**: 2026-07-28
**Reviewer**: independent read-only Reviewer
**Verdict**: `FAIL`
**Live action**: none

## Required findings

1. A model that selected and loaded a Team timeline did not clear
   `currentTeam` or the timeline when a later snapshot contained zero Teams.
   The stale timeline could remain rendered and `r` could request it again.
   The fresh-zero-Team test did not cover this transition.
2. The target-process helper rejected only literal `0`. All-zero decimal
   strings such as `00` were passed to `ps` and classified
   `target_unavailable` instead of the frozen `invalid_pid`.

All executed focused, package, race, format, and diff checks passed, but those
checks did not cover the two source-level gaps.

This Review authorizes no live canary, install, activation, staging, commit, or
P2A-W2 work.
