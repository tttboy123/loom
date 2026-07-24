# S2-W10 Contract Repair 2

- Attempt: 2 of 3
- Base repaired contract SHA256:
  `5b6e9c660c0a871549da0e45be55d2a2bbb05e54f1e0ad370665a949597917b0`
- Repair scope: contract and contract-review evidence only
- Product/test implementation: not authorized before fresh contract PASS

## Required repair

Repair 2 supersedes only the incorrect positivity clause in Repair 1:

1. accepted `RequestedBudget` is non-negative and may be zero;
2. accepted `RequestedConcurrency` is positive;
3. budget remains at or below `BudgetCeiling`;
4. concurrency remains at or below `ConcurrencyCeiling`; and
5. mandatory RED covers zero-budget success, negative-budget failure propagated
   from S2-W3 validation, overflow failure, exact preservation, source/plan
   mismatch rejection, and digest sensitivity.

All other Repair 1 changes and S2-W10 boundaries remain frozen.

VERDICT: REPAIR_FROZEN
