# S2-W36 Implementation Repair 2 Frozen Contract

- WorkItem: `S2-W36`
- Repair: `2`
- Risk: Strict
- Status: `REPAIR_CONTRACT_FROZEN`
- Parent contract SHA-256:
  `391e5c7917bf3745185219cc4b4ed3bcce2c5e48d39f3a043bc31c99697d3e0c`
- Repair 1 contract SHA-256:
  `2fde8d736ed23091c8abb92ed387c1f045d206e6f66c93c1d5b8fb9632b9551a`
- Review 2 SHA-256:
  `e48b8c123edb2944c6ee853f5ebc32f4cabf9f2835b8b95243458f9ac04f2d8d`
- Problem analysis:
  `.loom-evidence/phase1-slice2/S2-W36/problem-analysis-1.md`
- Frozen product SHA-256:
  `a9421c3b58de00a9a9d44d02f9c511cdf603bf9423a78b18237ea783bd269213`
- Frozen pre-repair test SHA-256:
  `8ce88b02ac23da4699da36ee17ad78908cc1517aab158978b03a45f4649d94e2`
- Frozen branch/head: `codex/loom-platform-slice2` at `c8ecc2a`

## Scope

Test-only repair. Developer may modify only:

- `internal/app/runtime_observer_test.go`
- `.loom-evidence/phase1-slice2/S2-W36/deliverable.md`

Controller may add Repair 2 review evidence and update non-historical
`docs/CURRENT.md`/`PROGRESS.md` status. Product
`internal/app/runtime_observer.go` must remain byte-for-byte unchanged at the
frozen SHA.

## Exact repair

Replace or consolidate only the existing configured-discovery failure proof:

1. name the direct case `configured_discovery_failure`;
2. use the existing `appDiscoveryFactory` with a named pointer and a
   `BuildProbe` callback that returns a sentinel error;
3. use named pointers to existing recording discovery and status committers;
4. construct one prepared observer and call `observer.RunOnce` exactly once;
5. assert `errors.Is` for the sentinel;
6. assert all five outputs are zero; and
7. assert exact calls `factory/discovery/status = 1/0/0`.

The `1/0/0` tuple is the acceptance fact: one configured-discovery attempt, no
observer retry, no selected writer, and no opposite/fallback writer. Do not add
a fake type, product branch, API, or lower-layer call.

## Mandatory Repair RED

Before editing the behavior case, add a separate non-substitutive coverage
guard that reads `runtime_observer_test.go` and reconstructs these four markers
from split strings so the guard cannot satisfy itself:

```text
configured_discovery_failure
factory.calls != 1
discoveryCommitter.calls != 0
statusCommitter.calls != 0
```

Run the focused S2-W36 command. RED must fail only on all four missing behavior
markers. Then add the exact behavior assertions. Marker presence cannot
substitute for the runtime sentinel, five-zero, and `1/0/0` assertions.

## GREEN and checks

Rerun the complete parent strict matrix: focused, app, impact,
focused-race-50, repository, repository-race, vet, formatting, diff, exact
scope, and product/test hashes. Deliverable and fresh Reviewer must explicitly
record the runtime call tuple `1/0/0`, not merely case presence.

No product edit, API change, new dependency, lower-layer composition,
projection rebuild, Journal/SQLite/Event metadata, retry, scheduler/ticker/
goroutine/daemon/config, Runtime activation, external action, Phase 2, or
Slice 3 behavior is authorized.

VERDICT: REPAIR_CONTRACT_FROZEN
