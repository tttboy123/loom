# P3A-W1 Contract Repair 2: Injective Binding and Event Identity

Date: `2026-08-03`

Status: `FROZEN — INDEPENDENT CONTRACT RE-REVIEW REQUIRED`

Parent Repair 1 SHA-256:
`5a2b16778b5cccb989496f1918d0d81a59b140feb38e55716dbd7574d6a25a00`

Re-review 2 verdict: `P0=0`, `P1=2`, `P2=0`, dual `FAIL`.

This Repair supersedes only Repair 1's subject identity fields/stream formula
and Event-code derivation sentence. All other parent/Repair 1 requirements
remain frozen. It creates no new WorkItem, owned file or authority.

## 1. Injective subject identity

The binding subject/record/Event/command schemas replace `subject_scope` alone
with the exact ordered identity fields:

```text
subject_kind
subject_id
subject_version
subject_digest
subject_scope
subject_project_id
subject_generation_id
subject_identity_digest
```

Closed scopes and identity rules are:

```text
agent_definition:
  project   -> project_id required, generation_id empty
  reusable  -> project_id empty, generation_id empty
  transient -> project_id empty, generation_id required

team_definition:
  project   -> project_id required, generation_id empty
  reusable  -> project_id empty, generation_id empty

work_package:
  builtin   -> project_id empty, generation_id empty
```

No other kind/scope combination is valid. The authoritative resolver signature
includes every field above except the derived digest. It reconstructs the
current typed subject, returns the exact scope identity and verifies version and
subject digest before any binding replay or write.

`subject_identity_digest` is lowercase SHA-256 of canonical ordered JSON:

```json
{"schema_version":1,"subject_kind":"...","subject_id":"...","subject_version":1,"subject_digest":"<sha256>","subject_scope":"...","subject_project_id":"","subject_generation_id":""}
```

The only binding stream formula is now:

```text
evolution-asset-binding/<subject_kind>/<subject_identity_digest>
```

The kind is included for inspection; the digest includes the full exact
identity, so same ID across scope/version/digest cannot share a stream. The
binding Event payload, IPC `set_binding` input, projected record and execution
source-head set all include `subject_project_id`, `subject_generation_id` and
`subject_identity_digest` immediately after `subject_scope`.

Changing a subject version or digest produces a new binding stream. The prior
binding remains historical and cannot apply because execution resolves the
current full subject identity and requires the matching digest stream. Team
subjects also CAS their current Team-definition source stream.

## 2. Sole Event-code authority

The prose derivation rule is deleted. The explicit Event-code mapping table in
Repair 1 is the sole authority; no prefix transformation is performed in
production or tests. Unknown Event type or missing table entry fails contract
construction before Journal write.

The Event ID and idempotency formulas remain unchanged and take the literal
mapped code.

### Frozen golden identities

For the exact UTF-8 seed bytes with no final newline:

```text
1\nEvolutionTemplateInstantiated\nevolution-asset-revision/team_template/team.demo/rev.1\nop-template-001\naaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n0
```

the SHA-256 is:

```text
a62662704f64e9a4cad50ebea4ba3c00237c85d10949af39d0944dcac94cefc1
```

and exact identities are:

```text
event_code = template_instantiated
event_id = p3a-template_instantiated-a62662704f64e9a4cad50ebea4ba3c00
idempotency_key = p3a/op-template-001/template_instantiated/0
```

For the exact seed:

```text
1\nEvolutionRunPromotionProposed\nevolution-asset-candidate/candidate.demo\nop-promotion-001\nbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\n3
```

the SHA-256 is:

```text
f49957f1cbcf6b4331a8f75a7be855bcb41c2afd5703dfe5a869133d19745d42
```

and exact identities are:

```text
event_code = run_promotion_proposed
event_id = p3a-run_promotion_proposed-f49957f1cbcf6b4331a8f75a7be855bc
idempotency_key = p3a/op-promotion-001/run_promotion_proposed/3
```

Mandatory RED must assert these exact bytes and identities plus every remaining
table row before implementation.

## 3. Re-review acceptance

Fresh independent Re-review must return `P0=0`, `P1=0`, `P2=0`,
Product/Authority `PASS` and Operational/Trace Governance `PASS`, proving both
prior P1s closed without weakening any parent/Repair 1 rule. Until then product
code and RED remain locked.

VERDICT: `FROZEN — PENDING RE-REVIEW`
