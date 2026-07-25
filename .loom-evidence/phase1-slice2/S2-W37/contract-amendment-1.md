# S2-W37 Contract Amendment 1

- WorkItem: `S2-W37`
- Amendment: `1`
- Status: `CONTRACT_AMENDMENT_FROZEN`
- Parent contract SHA-256:
  `999d9fba2022fc2b059b9394b26b04e9208ffe4b0f018707a327e937ff5f9159`
- Contract Review 1:
  `.loom-evidence/phase1-slice2/S2-W37/contract-review-1.md`
- Frozen branch/head: `codex/loom-platform-slice2` at `38891c3`

## Clarification

S2-W37 must implement nil/typed-nil trigger validation by calling the accepted
same-package `nilAppInterface(trigger)` helper from
`internal/app/runtime_discovery.go`.

That helper is already accepted, generic, side-effect-free reflection over the
interface value, and is used by accepted application boundaries for the same
validation purpose. S2-W37 does not import `reflect`, modify the helper, invoke
the trigger, recover a panic, or add a new validation authority.

The parent import whitelist remains unchanged: only `context`, `errors`,
accepted `runtime`, and accepted `state`. Static proof must require exactly one
`nilAppInterface(trigger)` call before `trigger.AwaitRuntimeObservation(ctx)`.

All other parent API, behavior, mandatory RED, checks, owned files, and
exclusions remain unchanged.

VERDICT: CONTRACT_AMENDMENT_FROZEN
