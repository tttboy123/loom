# P2A-W3 Timeline Lineage Repair 4 Implementation Review 1

Date: 2026-08-03

Verdict: `FAIL`

## Findings

- P0: none.
- P1: an explicitly present `"logical_node_id":""` survived
  `safeEventFields`, but the validator checked only non-empty values and could
  therefore treat malformed present metadata as omitted.
- P2: the new test called the validation helper directly and did not
  permanently exercise the full
  `safeEventFields → resolveDeliveryLineage → mapAuthoritativeRecords` path for
  attempt-only `WorkItemRejected` plus explicit-empty rejection.

The Reviewer independently reproduced the two file hashes and combined source
lock `a41f59f9ab90911c95c34cbf1d2496cc91575af94a6953398191a51ad77137dd`,
ran the focused and complete `internal/api` tests, and found no schema, writer,
authority, CAS, Grant, generation, Evidence, or P2A-W4 expansion.

root005 is not authorized. Repair remains inside P2A-W3.

