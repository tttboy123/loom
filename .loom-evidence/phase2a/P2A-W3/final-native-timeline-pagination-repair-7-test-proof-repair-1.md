# P2A-W3 Final Native Timeline Pagination Repair 7 Test Proof Repair 1

**Date**: 2026-08-03  
**Status**: `FROZEN`  
**Parent**: `final-native-timeline-pagination-repair-7-contract.md` plus Contract Repairs 1 and 2  
**Trigger**: `final-native-timeline-pagination-repair-7-implementation-review-1.md`

This is a test-evidence repair inside the already reviewed Repair 7 boundary.
It is not an Amendment, WorkItem, authority expansion or production behavior
change.

## Exact repair

1. Extend Swift Store tests to prove:
   - exactly 512 records on a final eighth page succeeds;
   - 512 records with continuation and 513 records fail closed;
   - cancellation fences late success and late error;
   - Mission-to-Mission switching fences late success and late error;
   - a newer load for the same selection fences an older result;
   - page, Board, Attention and record schema/Team/view/identity failures each
     publish no partial Timeline.
2. Extend the exact authoritative Go-to-Swift test to freeze all Journal Events
   and derived stream heads after test-only padding and before the Store probe,
   compare both after the probe, and assert exact ordered globally unique
   delivery IDs for the complete aggregate.

## Exact owned files

- `apps/macos/Tests/LoomLocalAppTests/LocalProductStoreTests.swift`
- `cmd/loomd/product_daemon_test.go`
- `.loom-evidence/phase2a/P2A-W3/**`
- `docs/CURRENT.md`

Production Swift and Go files remain locked. No daemon, Provider, native app,
walkthrough, Journal authority, Projection, schema, staging or commit is
authorized by this repair. After focused and full verification, refresh the
exact Repair 7 source lock and obtain a fresh independent Implementation
Re-review PASS before the single replacement walkthrough.
