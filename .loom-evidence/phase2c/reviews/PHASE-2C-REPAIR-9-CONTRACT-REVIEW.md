# Phase 2C Repair 9 Independent Review

**Date**: 2026-08-08  
**Mode**: read-only independent contract and implementation review  
**Verdict**: `FAIL_P0_0_P1_2_P2_1`

## Findings

1. **P1 - Native Mission preflight remained reusable after Start.**
   `startPreflightedMission()` stored the accepted result and refreshed but did
   not clear `executionPreflight`. A second Store call could therefore submit
   the consumed start preflight again. Existing tests did not assert preflight
   consumption or no second request.
2. **P1 - Native Store did not enforce exact prepared actions.**
   `submitDecisionAction` converted any action other than `not_now` or
   `edit_scope` into operation `submit`, even when the action was absent from
   the authoritative sheet's `preparedActions` or `actions`. UI button state is
   not a sufficient Store command boundary.
3. **P2 - Native decision results were not validated before dismissal.**
   A decoded but non-authoritative or identity-mismatched result caused a
   refresh and sheet dismissal. The TUI already validates this boundary, but
   the Native stub encoded `authoritative: false` and had no real prepared
   submit regression.

## Required Closure

- Clear Native `executionPreflight` before the post-start refresh and prove a
  second Store call sends no start request or overwrites accepted running state.
- Require defer actions to be listed actions and submit actions to be in both
  listed and prepared action sets before constructing IPC.
- For operation `submit`, require an authoritative result matching the exact
  Mission and Decision identity before refresh or dismissal. Keep the prepared
  sheet visible on invalid result.
- Add Native tests for exact submit, unprepared-action rejection, and invalid
  authoritative result. Rerun independent review before source-lock generation.

The TUI Repair 9 changes were directionally correct. No test was run by the
reviewer, and earlier Phase 2C changes outside Repair 9 were intentionally not
reviewed. This FAIL authorizes no source lock or Journey.
