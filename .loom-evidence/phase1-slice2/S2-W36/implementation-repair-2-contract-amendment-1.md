# S2-W36 Implementation Repair 2 Contract Amendment 1

- WorkItem: `S2-W36`
- Repair: `2`
- Amendment: `1`
- Status: `CONTRACT_AMENDMENT_FROZEN`
- Parent Repair 2 contract SHA-256:
  `bc1e7be880e3ab0dc864dda96b707b2a5e256002d23b222bd050d8edd2de5c68`
- Contract Review 1:
  `.loom-evidence/phase1-slice2/S2-W36/implementation-repair-2-contract-review-1.md`
- Frozen branch/head: `codex/loom-platform-slice2` at `c8ecc2a`

## Amendment

The Repair 2 behavior, scope, product lock, and checks remain unchanged. Replace
only the Mandatory Repair RED marker specification with the following
case-local unique markers:

```text
configured_discovery_failure
configuredFactory.calls != 1
configuredDiscoveryCommitter.calls != 0
configuredStatusCommitter.calls != 0
```

The behavior case must use those exact local variable names for the existing
factory and recording committer pointers. The three call-count assertions must
be in the named `configured_discovery_failure` subtest after its one
`observer.RunOnce` call.

The separate source-reading guard must reconstruct all four strings from split
literals. Each full marker is absent from the frozen pre-repair test SHA
`8ce88b02ac23da4699da36ee17ad78908cc1517aab158978b03a45f4649d94e2`.
Mandatory RED must therefore fail on all four markers, eliminating the global
marker collision found by Contract Review 1.

The marker guard remains non-substitutive. GREEN still requires the runtime
sentinel, five-zero outputs, and exact local call tuple `1/0/0`.

VERDICT: CONTRACT_AMENDMENT_FROZEN
