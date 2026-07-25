# S2-W35 Implementation Repair 1 Contract

- WorkItem: `S2-W35`
- Repair: `1`
- Risk: Strict
- Status: `REPAIR_CONTRACT_FROZEN`
- Frozen branch/head: `codex/loom-platform-slice2` at `a6eb816`
- Parent contract SHA-256:
  `a9071190a1dd8a6e391ec0144006b6ab4627317fd6ca08ff7b6c2836a1f722f8`
- Frozen product SHA-256:
  `084098029b6256cd2a8e741709fc044cb83e37adc204ba1d51cdaee8ceabfe04`

## Scope

Test-only. Modify only:

- `internal/app/runtime_observation_projected_test.go`
- S2-W35 Repair 1 evidence
- `docs/CURRENT.md`
- Controller-owned non-historical `PROGRESS.md` changes

Product must remain byte-for-byte unchanged.

## Required repair

Extend the existing real SQLite chain to directly prove:

1. exactly three persisted Events;
2. ordered Event types are discovery, discovery, status;
3. per-stream sequences are exactly `1`, `2`, `3`;
4. retry returns exact same selected commit Events/digests without new rows;
5. final rebuild yields the expected inventory change, online status,
   discovery sequence `2`, and status sequence `3`; and
6. opposite committers remain unused.

## Mandatory Repair RED

First add only
`TestRunProjectedConfiguredRuntimeObservationOnceSQLiteExactEventCoverage`,
which reads the local test source and requires canonical markers for the
`event_type, seq` query and final discovery/status sequence assertions. It must
fail before the behavior assertions are added and cannot substitute for them.

Then add the exact behavior assertions and rerun the complete parent matrix.
Product SHA must remain unchanged. Fresh independent Repair 1 implementation
review is mandatory before acceptance.

VERDICT: REPAIR_CONTRACT_FROZEN
