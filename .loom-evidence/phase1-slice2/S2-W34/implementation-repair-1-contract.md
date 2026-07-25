# S2-W34 Implementation Repair 1 Contract

- WorkItem: `S2-W34`
- Repair: `1`
- Risk: Strict
- Status: `REPAIR_CONTRACT_FROZEN`
- Frozen branch/head: `codex/loom-platform-slice2` at `affd2a6`
- Parent contract SHA-256:
  `d999672c8c265bbcfd923412681ae632eebc7eb2d2795a9ea1e3933ff3d51d32`
- Review 1 SHA-256:
  `a1654b76e69335e758262b31e2863b0c9e99a817fc534b70268c5cdebbf58459`
- Frozen product SHA-256:
  `26a1a706c8c058d48d5304aa5eddbb7fa2008cc16ff9e02c0eb9edff8974d7ea`

## Scope

Test-only. Modify only:

- `internal/app/runtime_observation_cycle_test.go`
- S2-W34 Repair 1 evidence
- `docs/CURRENT.md`
- Controller-owned non-historical `PROGRESS.md` changes

`internal/app/runtime_observation_cycle.go` must remain byte-for-byte unchanged.

## Required repair

Close the sole Review 1 gap with direct S2-W34 boundary cases for:

1. 33 factories rejected before any factory call;
2. typed-nil factory rejection;
3. `present=true` with nil probe rejection;
4. `present=false` with nonnil probe rejection;
5. probe observation source error propagation; and
6. invalid discovered observation/model propagation.

Every case must return the discovery snapshot and all four S2-W33 outputs zero
and make zero discovery/status committer calls. Exact accepted S2-W26 error
identity/wrapping remains authoritative.

## Mandatory Repair RED

First add only a focused coverage guard named
`TestRunConfiguredRuntimeObservationOnceS2W26ErrorMatrixCoverage` that requires
the six canonical case markers:

- `oversized_factory_set`
- `typed_nil_factory`
- `present_nil_probe`
- `absent_non_nil_probe`
- `probe_source_error`
- `invalid_discovered_observation`

Before adding the cases, the guard must fail on the missing markers. Then add
the six actual behavior cases and make the guard plus complete focused suite
GREEN. The guard must inspect only the local test source and cannot substitute
for the behavior assertions.

## Checks

Rerun the complete parent strict matrix. Verify:

- frozen product SHA is unchanged;
- all six new cases exercise the S2-W34 coordinator;
- all five outputs are zero on error;
- neither committer is called;
- no production/API/authority/scope change exists.

Fresh independent Repair 1 implementation review is required before acceptance.

VERDICT: REPAIR_CONTRACT_FROZEN
