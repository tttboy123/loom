# S2-W25 Implementation Repair 1 Contract

- WorkItem: `S2-W25`
- Risk: Strict
- Repair count: `1`
- Status: `REPAIR_CONTRACT_FROZEN`
- Trigger: `implementation-review-1.md`
- Product/test state before repair review: unchanged from reviewed Candidate
- Active amended S2-W25 contract SHA-256:
  `d47939a110667b640cb3f264a7edb74557ee9e0988e99928549cd358cd297a49`
- Pre-repair product/test SHA-256:
  `runtime_status_baseline.go=d1ba17f8...38927`,
  `runtime_status_baseline_test.go=33d30de9...ea42f`

## Owned files

- `internal/projection/runtime_status_baseline.go`
- `internal/projection/runtime_status_baseline_test.go`
- `.loom-evidence/phase1-slice2/S2-W25/deliverable.md`

All other accepted and Candidate product/test files remain unchanged. Controller
evidence/docs remain outside Developer ownership.

## Frozen repair

Track one Runtime owner for the combined set of every visible:

```text
DiscoveryEventID
StatusEventID
StatusPreviousEventID
```

For status-free records only discovery identity is registered. For complete
status records all three identities are registered.

The same Event ID may repeat within the same Runtime record only for the
already-valid first-status discovery/previous alias. Any same-role or cross-role
reuse by a different Runtime record returns `nil` plus
`ErrInvalidRuntimeStatusBaselineProjection`.

The repair must not change per-record validation, adapter output, S2-W22
reconciliation/digests, S2-W23/S2-W24 behavior, or any Event/Journal authority.

## Mandatory Repair RED

Before product repair, add table-driven tests for all nine cross-record role
combinations:

```text
discovery -> discovery
discovery -> status
discovery -> previous
status -> discovery
status -> status
status -> previous
previous -> discovery
previous -> status
previous -> previous
```

Each must fail on the reviewed Candidate by returning a non-error baseline.
After the minimal repair, every case must return no partial baseline and an
error matching `ErrInvalidRuntimeStatusBaselineProjection`.

The accepted same-record first-status discovery/previous alias remains GREEN.

## Checks

Rerun the complete repaired S2-W25 contract matrix without omission. Product
imports and all explicit exclusions remain unchanged.

VERDICT: REPAIR_CONTRACT_FROZEN
