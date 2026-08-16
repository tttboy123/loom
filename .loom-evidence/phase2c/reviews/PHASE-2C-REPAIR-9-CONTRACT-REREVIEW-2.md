# Phase 2C Repair 9 Independent Re-review 2

**Date**: 2026-08-08  
**Mode**: read-only independent final source-lock review  
**Verdict**: `FAIL_P0_0_P1_1_P2_1`

## Findings

1. **P1 - Native stale preflight survived authoritative refresh.**
   `refresh()` replaced the product snapshot without comparing an unconsumed
   `executionPreflight.viewVersion` with the new authoritative view. A later
   Store Start could still submit that stale preflight. There was no Native
   regression for ready preflight, newer authoritative refresh, then zero Start
   request.
2. **P2 - TUI repeated Start lacked an explicit no-request regression.**
   The implementation appeared guarded after `missionStartedMsg` cleared the
   preflight, but Repair 9 now explicitly requires a repeated TUI start call to
   remain inert and preserve accepted running state.

## Required Closure

- Expire and clear a Native unconsumed preflight whenever a successful
  authoritative refresh changes its view version; preserve already accepted
  execution results.
- Prove a subsequent Native Start sends no request and reports
  `preflight_expired`.
- Prove a repeated TUI Start after accepted refresh produces no command and
  preserves the running result.
- Rerun independent review before source-lock generation.

The reviewer ran `go test ./internal/tui` and
`swift test --filter LocalProductStoreTests`; both passed on the pre-closure
bytes. Exact decision submit, typed stale/missing draft recovery, visible copy,
post-success preflight consumption, prepared-action validation, and
authoritative identity-matched decision results passed review. This FAIL
authorizes no source lock or Journey.
