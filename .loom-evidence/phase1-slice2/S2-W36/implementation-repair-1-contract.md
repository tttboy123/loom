# S2-W36 Implementation Repair 1 Frozen Contract

- WorkItem: `S2-W36`
- Repair: `1`
- Risk: Strict
- Status: `REPAIR_CONTRACT_FROZEN`
- Parent contract SHA-256:
  `391e5c7917bf3745185219cc4b4ed3bcce2c5e48d39f3a043bc31c99697d3e0c`
- Review 1:
  `.loom-evidence/phase1-slice2/S2-W36/implementation-review-1.md`
- Frozen product SHA-256:
  `a9421c3b58de00a9a9d44d02f9c511cdf603bf9423a78b18237ea783bd269213`
- Frozen branch/head: `codex/loom-platform-slice2` at `c8ecc2a`

## Scope

Test-only repair. Developer may modify only:

- `internal/app/runtime_observer_test.go`
- `.loom-evidence/phase1-slice2/S2-W36/deliverable.md`

Controller may add Repair 1 review evidence and update non-historical
`docs/CURRENT.md`/`PROGRESS.md` status. Product
`internal/app/runtime_observer.go` must remain byte-for-byte unchanged at the
frozen SHA.

## Required direct error matrix

Add prepared-observer behavior cases that directly call `observer.RunOnce` and
prove exact accepted error propagation, five zero outputs, and no retry:

1. canceled context before observation;
2. expired deadline before observation;
3. configured discovery failure;
4. invalid projected identity drift;
5. selected discovery path with missing committer;
6. selected discovery path with typed-nil committer;
7. selected discovery committer returned error;
8. selected discovery result mismatch;
9. selected status path with missing committer;
10. selected status path with typed-nil committer;
11. selected status committer returned error;
12. selected status result mismatch; and
13. cancellation after a selected discovery write.

Every case must assert all five outputs zero. Context/pre-planning cases must
assert no committer call. Selected-path cases must assert exactly one selected
committer call when a concrete committer is supplied, zero opposite calls, and
no fallback or implicit retry.

Existing nil-context, configured-discovery, positive-path, mutation, explicit
retry, SQLite, and static tests remain. Consolidation is allowed only if the
required cases remain direct and named.

## Mandatory Repair RED

Before adding the behavior matrix, add a non-substitutive coverage guard that
reads `runtime_observer_test.go` and requires these canonical case markers:

```text
canceled_context
expired_deadline
invalid_identity_drift
missing_discovery_committer
typed_nil_discovery_committer
discovery_committer_error
discovery_result_mismatch
missing_status_committer
typed_nil_status_committer
status_committer_error
status_result_mismatch
canceled_after_discovery_write
```

Run the focused S2-W36 command and record RED failing only on the missing
markers. The marker guard does not substitute for behavior assertions.

## GREEN and checks

Add the direct behavior cases, then rerun the complete parent strict matrix:
focused, app, impact, focused-race-50, repository, repository-race, vet,
formatting, diff, and exact scope. Recompute the test hash and prove the product
hash remains unchanged.

No product edit, API change, new dependency, lower-layer composition,
projection rebuild, Journal/SQLite/Event metadata, retry, scheduler/ticker/
goroutine/daemon/config, Runtime activation, external action, Phase 2, or
Slice 3 behavior is authorized.

VERDICT: REPAIR_CONTRACT_FROZEN
