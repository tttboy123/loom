# S2-W38 Implementation Repair 1 Contract

- WorkItem: `S2-W38`
- Repair type: test/evidence only
- Risk: Strict
- Status: `REPAIR_CONTRACT_FROZEN`
- Repairs:
  `.loom-evidence/phase1-slice2/S2-W38/implementation-review-1.md`
- Frozen branch/head: `codex/loom-platform-slice2` at
  `9175f9426f1e153b05fbc6af6e2fbf7b27103b6e`
- Locked product SHA-256:
  `33d7e46283a257d1b7cb555b77a94f5491405db0d42654c934183ba58790c9c0`
- Frozen pre-repair test SHA-256:
  `258c84a075629882391cd75938e6bf71cb71a6f92f14b12ef0938fd683a49029`
- Product repair attempt: `0/3`

## Purpose

Close only the missing direct success-path runtime proof. Production behavior,
API, imports, error semantics, projection binding, trust boundary, and all
existing tests remain unchanged.

## Mandatory Repair RED

First add only a non-substitutive source coverage guard to
`internal/app/runtime_observation_loop_test.go`.

The guard must reconstruct the following four unique literals from split
fragments so its own source does not satisfy the check:

- `s2_w38_success_none_exact_order`
- `s2_w38_success_discovery_exact_order`
- `s2_w38_success_status_exact_order`
- `s2_w38_pre_post_refresh_observable`

It must require each complete literal exactly once in the test source.
Focused Repair RED must fail only because all four case-local behavior markers
are missing. Product hash must remain byte-for-byte unchanged.

## Required test-only repair

Add one direct
`TestRunProjectionSynchronizedRuntimeObservationOnceSuccessPathsAndOrder`
group with three subtests.

### None

- Seed Runtime A discovery sequence 1 in the real temporary Journal, but do not
  rebuild the bound projection.
- On factory build, prove pre-refresh has exposed Runtime A.
- During the factory call, append a distinct Runtime B discovery sequence 1 to
  the Journal, then return Runtime A unchanged.
- Prove exact none tuple, trigger/factory/probe counts `1/1/1`, both accepted
  writers `0/0`, and post-refresh exposure of Runtime B.

### Discovery

- Seed discovery sequence 1 without rebuilding the bound projection.
- Prove the factory observes sequence 1 after pre-refresh.
- Return a mixed inventory/status observation that must select discovery.
- Use the real prepared discovery committer behind one recording wrapper.
- Prove exact trace `trigger→factory→probe→discovery commit`, exact counts,
  opposite status writer zero, and post-refresh discovery sequence 2 with the
  changed inventory/status facts.

### Status

- Seed discovery sequence 1 without rebuilding the bound projection.
- Prove the factory observes the prior status after pre-refresh.
- Return unchanged inventory with one status change.
- Use the real prepared status committer behind one recording wrapper.
- Prove exact trace `trigger→factory→probe→status commit`, exact counts,
  opposite discovery writer zero, and post-refresh status sequence 2 with the
  changed status fact.

The group must include the four frozen unique markers exactly once across its
case-local names/assertions. It may reuse accepted test helpers and add only
unexported test helpers. It must not weaken or remove an existing assertion.

## Scope and checks

Allowed behavioral edit:

- `internal/app/runtime_observation_loop_test.go`

Allowed evidence/status edits:

- `.loom-evidence/phase1-slice2/S2-W38/**`
- `docs/CURRENT.md`
- Controller-owned non-historical `PROGRESS.md` hunks

`internal/app/runtime_observation_loop.go` is locked byte-for-byte.

After GREEN, run the complete original strict matrix: focused, app, impact,
focused-race-50, repository, repository-race, vet, formatting, diff, exact
scope, and product-hash verification.

Fresh Repair 1 contract and implementation Reviewer `PASS` are mandatory before
acceptance or local commit.

VERDICT: REPAIR_CONTRACT_FROZEN
