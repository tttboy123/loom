# P3A-W1 Implementation Repair Review 2

Date: `2026-08-04`

Reviewer: fresh independent read-only Reviewer (Codex CLI, read-only sandbox)

## Verdict

```text
P0 = 0
P1 = 0
P2 = 1
Product/Authority: FAIL
Operational/Trace Governance: PASS
Overall Implementation Repair: FAIL
```

## P2-2 (remaining replay gaps)

`validReplayEvaluation`/`validReplayLifecycle` were updated but two replay-only
gaps remained: `observed=false ⇒ microunits=0` was not enforced, the
`applicable_scope` 4096 bound was missing, and
`RuntimeSkillMaterializationPublished` replay validated only the capability
string. Regression coverage did not yet cover the other result sets,
currencies, `restored_lifecycle`, unobserved-with-value or materialization
field formats.
