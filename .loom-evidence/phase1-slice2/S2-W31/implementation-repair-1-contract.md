# S2-W31 Implementation Repair 1 Contract

- WorkItem: `S2-W31`
- Status: `REPAIR_CONTRACT_FROZEN`
- Active contract SHA-256:
  `887d8a75237c7e578e966435f2e9701d2a5c3fec6d1b6729af7dcaf055c4749c`
- Pre-repair product SHA-256:
  `bb9b96a6c13f7e6d23a7ce61450143078a7b4a916a77f59c8b50059be0d6a40f`
- Pre-repair test SHA-256:
  `86d1db5476f7d3d7056d461dca4ce9a8d9ec890e198a31d613717a900f55b8e3`
- Trigger: S2-W31 Implementation Review 1 `FAIL`

## Scope

Test-only repair. `internal/app/runtime_status_projection.go` must remain
byte-for-byte unchanged.

## Required proof

1. Add a complete canonical status-bearing projection fixture and prove S2-W31
   transitions use its `StatusEventID` and `StatusSequence`, not discovery
   provenance. Include consecutive-status and rediscovery-zero-status cases if
   needed to prove the selector semantics without duplicating S2-W25 logic.
2. Add a table that mutates exactly one projection defect per case and always
   proves unchanged `projection.ErrInvalidRuntimeStatusBaselineProjection`, two
   zero Candidates, and zero committer calls. Cover at minimum: oversized map,
   key mismatch, invalid Runtime core/identity, noncanonical model IDs, invalid
   discovery digest/time/Event provenance, partial status group, invalid status
   sequence relationship, and cross-record Event identity reuse.
3. Add S2-W29 propagation cases for invalid discovery, stable-identity drift,
   committer sentinel, commit-result mismatch, and a deterministic context
   error after baseline construction. Every error returns two zero Candidates;
   pre-commit failures make zero committer calls and committer failures make
   exactly one.
4. Extend mutation proof to projection nested slices, committer-received
   reconciliation accessors, and returned reconciliation/commit Event
   accessors.

## Mandatory Repair RED

Add the required tests before any product change. At least one status-bearing
provenance assertion must fail against a deliberately discovery-only helper or
missing fixture proof, and the expanded focused suite must demonstrate the
Review 1 evidence gap. Product changes are forbidden.

## Verification

Rerun the complete active-contract strict matrix. Fresh independent Repair 1
implementation review `PASS` is required before acceptance or local commit.
This is Repair 1 of at most three.

VERDICT: REPAIR_CONTRACT_FROZEN
