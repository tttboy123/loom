# Phase 3A Gate 1 Contract Repair 1 Re-review

Date: `2026-08-03`

Reviewer: fresh independent read-only Reviewer

Repair 1 SHA-256:
`5a2b16778b5cccb989496f1918d0d81a59b140feb38e55716dbd7574d6a25a00`

The Reviewer edited no file, staged nothing and ran no product/live/network
action.

## Findings

```text
P0 = 0
P1 = 2
P2 = 0
```

### P1-1: binding stream identity is not injective

The binding record carried subject scope/version/digest, but the stream formula
used only subject kind and ID. Current Agent and Team catalogs legitimately
reuse IDs across reusable, project and transient scopes, so those bindings
could collide.

### P1-2: Event-code derivation contradicts its explicit table

The prose said to remove only `EvolutionAsset` or `RuntimeSkill`, while the
explicit table also removed `Evolution` from Template and RunPromotion Events.
Because Event code enters Event ID and idempotency key, the identity was not
exact.

## Verdicts

```text
Product/Authority: FAIL
Operational/Trace Governance: FAIL
```

Gate 1 remains closed pending an injective canonical subject identity and one
sole Event-code authority with frozen golden IDs.

VERDICT: `FAIL`
