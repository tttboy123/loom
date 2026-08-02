# P2A-W3 Authoritative Acceptance Time Terminal Contract Repair 1

**Date**: 2026-08-03  
**Status**: `FROZEN / PENDING INDEPENDENT REPAIR REVIEW`  
**Parent contract SHA-256**:
`d43ea00569669effa642a3cb8e80ee5e0f5957fedb5799f7e3cceb964c6ffc11`  
**Contract Review 1 SHA-256**:
`8ad39aa99ea6617c491d90a7ab78b76736f43e4532b65ea962d9ce6c8b2d3bf9`  
**New WorkItem**: none; `P2A-W4` does not exist

This Repair 1 changes only the two ambiguities identified by Contract Review 1.
All unmodified parent requirements remain binding.

## 1. Compatibility field is non-authoritative

For this bounded repair, `TeamNodeAcceptanceInput.Decision` may remain in the Go
struct solely so the locked prepared Review Gate caller continues to compile.
It is deprecated compatibility input, not a Proposal, Candidate or authority.

Work Authority must:

- neither require that field to be valid nor read its kind, digest, time,
  verifier digest or contract binding for a new acceptance;
- obtain one operation instant and derive the sole authoritative
  `AcceptanceDecision` from the accepted contract, deterministic result and
  optional Verifier Candidate;
- use only that derived decision for Event IDs, payloads, outcome kind,
  recovery trigger/digest routing, node state and terminal aggregation;
- ignore zero, stale, valid-but-different-time, mismatched-kind and malformed
  compatibility values identically;
- recognize an existing exact acceptance from committed authoritative facts and
  exact non-time semantic/lineage inputs, without recomputing a new current-time
  digest.

`internal/app/team_execution.go` must stop creating a final decision and may
leave the compatibility field zero. The locked
`internal/app/local_product_decision.go` may continue populating it; Authority
must ignore it, so no out-of-scope production edit is required.

Mandatory tests prove all compatibility-field mutations produce byte-identical
new authority Events under the same authority clock and cannot alter success,
rejection, decided time, digest, idempotent replay or recovery scheduling.

This is a temporary source-compatibility seam, not permanent dual authority. A
later API cleanup may remove the field under its own contract, but is not
required for P2A-W3.

## 2. Exact event name

All RED, verification, live and review checks use the actual authority Event:

```text
WorkItemVerificationCommitted
```

`VerificationDecisionCommitted` in the historical Result Review is a label
error only and must not be queried or introduced as a new Event type. The
consumed live facts remain unchanged.

## 3. Scope and gates

The parent's exact five owned files remain sufficient and unchanged. No
`local_product_decision` source/test, Event schema, Journal, Projection or other
authority file is reopened.

Contract Review 1 remains a historical FAIL. Implementation may begin only
after a fresh independent Repair Review confirms parent plus Repair 1 are
jointly complete and unambiguous. Contract freeze and Repair Review authorize
no live action, staging, commit or P2A-W4.
