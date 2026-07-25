# S2-W25 Contract Repair 1

- Scope: contract-only
- Product/test changes before repair review: none
- Trigger: `contract-review-1.md`

## Frozen repair

The status-provenance validation now requires discovery and previous
status-bearing provenance to match as a pair:

```text
StatusPreviousEventID == DiscoveryEventID
if and only if
StatusPreviousSequence == DiscoverySequence
```

This closes both forged directions:

1. equal sequence with a different Event ID; and
2. equal Event ID with a later sequence.

The mandatory RED matrix explicitly covers both. No adapter API, ownership,
S2-W22 amendment, digest version, authority boundary, or other contract clause
changes.

VERDICT: CONTRACT_REPAIR_FROZEN
