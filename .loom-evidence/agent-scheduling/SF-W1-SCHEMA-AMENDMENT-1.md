# SF-W1 Schema Amendment 1 (bounded, frozen)

Date: `2026-08-04`

Status: `FROZEN — INDEPENDENT AMENDMENT REVIEW REQUIRED`

WorkItem: the only and existing `SF-W1`. This Amendment creates no SF-W4, no
thin WorkItem, no new authority, no second database, and no external action.

## 1. Reason

The accepted `SF-EXIT-CONTRACT.md` §5 `QueueJob` record is field-exact but
omits the admission-compiled fields the same Contract requires the
Decomposition Compiler to validate: vertical capability (rule 6), exit
conditions (rule 7), verification and integration strategy (rules 7–8), and
protected-authority path claims (rule 4, fail closed). Without persisting
them on the Job, later lanes (SF-W2 dispatch, SF-W3 integration) cannot read
the compiled strategy from the authoritative queue projection, and
`GATE1-CONTRACT-REVIEW-2.md` §7(rule 7) "every Job has frozen exit conditions
and verification strategy" has no record home.

## 2. Supersession (bounded)

This Amendment amends only the §5 `QueueJob` record in
`SF-EXIT-CONTRACT.md` by adding the five fields below (additive). No existing
field name, type, or semantic changes; no other record, event payload rule,
authority boundary, RED, journey, or acceptance item changes.

## 3. QueueJob record additions (additive)

```json
{
  "capability_kind": "string",
  "exit_conditions": ["string"],
  "verification_strategy": "string",
  "integration_strategy": "string",
  "protected_authority_paths": ["string"]
}
```

Semantics: `capability_kind` is the vertical capability the Job must
produce (wrapper-only/adapter-only/coordinator-only/visual-only claims are
rejected by admission); `exit_conditions` are the frozen acceptance
conditions (absence is a compile error per Decomposition Compiler rule 7);
`verification_strategy` and `integration_strategy` name the frozen
verification and target-branch integration strategy (rule 8); and
`protected_authority_paths` are explicit claims on protected authority paths
that fail closed unless a separately reviewed human-governed contract
permits them (rule 4). The `QueueJobCreated` payload rule (§5) is unchanged:
payload = the amended record fields + `correlation_id`/`evidence_digests`.

## 4. Not claimed

No new authority, schema-version bump of accepted P3A records, second
writer/database, push/merge, network, migration, or change to any RED,
journey, or acceptance item.

## 5. Independent review acceptance

PASS requires a fresh read-only Reviewer to prove: bounded supersession;
additive field set with no renamed/retyped existing field; rule 7
("absence is a compile error") now has a record home; no RED/journey/
acceptance/authority change.

VERDICT: `FROZEN — PENDING REVIEW`
