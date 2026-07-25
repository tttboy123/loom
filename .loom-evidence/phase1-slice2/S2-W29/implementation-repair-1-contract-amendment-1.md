# S2-W29 Implementation Repair 1 Contract Amendment 1

- WorkItem: `S2-W29`
- Status: `REPAIR_CONTRACT_AMENDMENT_FROZEN`
- Amends:
  `.loom-evidence/phase1-slice2/S2-W29/implementation-repair-1-contract.md`
- Trigger: Repair 1 Contract Review 1 `FAIL`

## Superseded clause

This amendment replaces only Repair 1 §2's private result-interface mechanism.
The delayed-cancellation repair, static assertions, mandatory Repair RED,
preserved behavior, verification, and authority boundaries remain unchanged.

## Frozen primitive-fact mechanism

Do not add a product interface and do not import `internal/journal`.

Instead, add one private product record containing only already extracted
primitive validation facts:

```text
committed bool
sourceReconciliationDigest string
baselineDigest string
sourceDiscoveryDigest string
eventCount int
eventAccessorCount int
commitDigest string
```

The existing concrete-result validator must:

1. accept the concrete `state.RuntimeStatusCommitCandidate`;
2. call each of its seven public accessors exactly as the active contract
   requires;
3. reduce `len(commit.Events())` to `eventAccessorCount` immediately;
4. pass only the primitive record to a private pure fact validator; and
5. return that validator's result.

The pure fact validator compares the primitive record to the reconciliation
Candidate. It imports no Journal type and grants no new authority.

## Isolated proof

Add a direct table over one known-valid primitive fact record. The valid record
must pass. Each case mutates exactly one field:

- committed flag;
- reconciliation digest;
- baseline digest;
- discovery digest;
- Event count;
- Event accessor count; and
- digest shape.

Every one-field mutation must fail while all other fields remain valid.
Existing coordinator-level tests continue to prove the concrete extractor and
validator are used on the public path.

## Preserved boundary

The product import set remains standard library plus `internal/runtime` and
`internal/state`. No Journal/projection/config/scheduler/daemon/adapter import,
public API change, metadata allocation, Event construction, or new authority is
permitted.

VERDICT: REPAIR_CONTRACT_AMENDMENT_FROZEN
