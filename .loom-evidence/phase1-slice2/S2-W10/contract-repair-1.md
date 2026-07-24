# S2-W10 Contract Repair 1

- Attempt: 1 of 3
- Base contract SHA256:
  `72a98b3d02bb9fd2221e5eb56b695bbb3ed1a9c436b9fcd4a6e796f174872327`
- Repair scope: contract and contract-review evidence only
- Product/test implementation: not authorized before fresh contract PASS

## Required repair

1. Add exact accepted `RequestedBudget` and `RequestedConcurrency` fields to
   the instantiation-plan Candidate.
2. Copy those values from the accepted Draft references.
3. Keep catalog budget/concurrency ceilings as separate fields.
4. Include requested values and ceilings in the plan digest and validation
   comparison.
5. Require requested values to remain positive and within their exact catalog
   ceilings.
6. Add mandatory RED proof for preservation, isolation, digest sensitivity,
   source mismatch, and ceiling overflow.

All other S2-W10 boundaries remain frozen.

VERDICT: REPAIR_FROZEN
