# Phase 2C Repair 11 Source Review

**Review type**: independent read-only implementation review  
**Verdict**: FAIL  
**Counts**: P0=0, P1=1, P2=1

## Findings

- P1: the original two-command `tea.Batch` allowed product snapshot and
  permission-attention completions to update the screen independently. Loading
  could become false after only one source completed, and a failed permission
  refresh retained stale actionable permission rows.
- P2: the original focused test executed only the successful `r` batch in one
  order. It did not cover entry from either direction, operation without a
  permission capability, partial failure, or overlapping and out-of-order
  refresh completions.

## Required Resolution

Refresh the two rendered sources as one generation-bound result, keep loading
truthful until the complete result settles, ignore older generations, clear
permission actions on either source failure, and add the missing causal test
matrix. No finding was raised against native SafeText mapping, snapshot-failure
preservation, scope, or authority boundaries.

This failed review is preserved. It does not authorize a source lock, journey,
ADR-0015 acceptance, or Phase 2C acceptance.
