# P3A-W1 Owned-path Repair Re-review 2

Date: `2026-08-04`

Reviewer: fresh independent read-only Reviewer (Codex CLI, read-only sandbox)

Reviewed Repair SHA-256:
`279a7526528d4b9c4a1ca7739f3b3203e9c5db9f5702efc8a26ced8b7c561f21`

## Verdict

```text
P0 = 0
P1 = 0
P2 = 0
Product/Authority: PASS
Operational/Trace Governance: PASS
Overall Repair: PASS
```

## Closure evidence

- Review 1's P2 is closed: the legacy `else` branch resets all three digest
  strings to `""`, matching the attempt-projection convention.
- Regression RED `TestP3ARunProjectionLegacyReclaimClearsAllLineageFields`
  (owned `internal/projection/projection_test.go`) reproduces the stale-digest
  symptom on pre-fix code and passes post-fix; the full
  `internal/projection` suite is green.
- The Swift probe remains read-only through the production
  `evolution_asset_snapshot` method.
- Working-tree accounting confirms no other product path outside the
  allowlist plus the two Repair seams; documented exclusions hold; no P3A-W2.

## Non-blocking observations (acknowledged)

1. Repair §2's recorded `run_authority.go` digest (`01540aab…`) predates the
   closure fix; the current working-tree digest is `582baa50…`. The delta is
   documented in Repair §5 and is reconcilable.
2. Repair §1 narrative credits `run_authority.go` with projecting
   `RuntimeSkillMaterializationPublished` facts; that file carries no such
   handler (facts flow through the exact lineage fields on
   `TeamReadySetDispatched`/`RunClaimed`). Narrative only; no ownership or
   seam impact.

Neither observation blocks the closure.

VERDICT: `PASS`
