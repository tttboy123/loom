# P3A-W1 Implementation Repair: Exact Schema Freeze and Review Closure

Date: `2026-08-04`

Status: `FROZEN — INDEPENDENT AMENDMENT REVIEW REQUIRED`

WorkItem: the existing and only `P3A-W1`. This Repair does not create
`P3A-W2`, a new authority, a new database, a second writer or a thin
adapter/coordinator WorkItem.

Reviewed parent set: P3A-W1 Contract (`12b8c1f2…fc9c`), Contract Repair 1
(`5a2b1677…a00`), Contract Repair 2 (`b59b178b…796`), Exact Lineage/
Materialization/Journey Amendment (`5e333ffd…54d`) + its Review 1, and
Owned-path Repair (`279a7526…f21`) + its Reviews 1-2.

This Repair supersedes only the clauses named below. Every unchanged
parent/Amendment security, authority, owned-path, RED, verification,
cross-client, review, exclusion and single-commit requirement remains frozen.

## 1. Independent Implementation Review 1 findings

Fresh independent Implementation Review 1 (2026-08-04) returned
`P0=0`, `P1=2`, `P2=4`, Product/Authority `FAIL`,
Operational/Trace Governance `PASS`:

```text
P1-1 EvolutionTemplateInstantiated omits frozen revision_digest; replay never
     decodes/validates ImportProposed, TemplateInstantiated or
     RunPromotionProposed facts.
P1-2 Archiving an active revision reads only the revision stream, so
     state.active is empty: previous_lifecycle is recorded as candidate and
     the activation stream head is outside the CAS.
P2-1 Frozen §3 bounds not enforced: name 1-128 bytes, description <=4096 with
     control/bidi/terminal-escape rejection, dependencies <=32,
     compatible_runtime_capabilities <=32, asset_revision_bindings <=32.
P2-2 Replay never validates closed enum values (risk, asset_kind, lifecycle,
     source_scope, template_output, decision_source, fixture_kind, result
     strings), so malformed facts do not fail rebuild.
P2-3 EvolutionAssetCandidateCreated embeds decision/decision_event_id fields
     absent from the frozen event schema; snapshot result carries
     binding_subjects[]/promotion_sources[] beyond the frozen result fields.
P2-4 record_evaluation hardcodes pass/zero results instead of delivering
     EA-07 evaluation semantics.
```

## 2. Code closures (owned paths only)

### P1-1

- `internal/assets/authority.go`: `EvolutionTemplateInstantiated` payload now
  carries exact `revision_digest` immediately after `revision_id` (value =
  revision ArtifactDigest), matching Repair 1 §5.
- `internal/assets/replay.go`: `EvolutionAssetImportProposed`,
  `EvolutionTemplateInstantiated` and `EvolutionRunPromotionProposed` are now
  strictly decoded and validated (IDs, digests, closed enums, cross-fact
  revision-digest match for template facts); malformed facts fail rebuild.

### P1-2

- `internal/assets/authority.go` `revisionLifecycle`: reads the activation
  stream in the same `ReadStreamSet` and CAS expectation set; an active
  revision archives with `previous_lifecycle=active`; the archive/restore
  transaction now includes the activation stream head.

### P2-1

- `internal/assets/canonical.go`: `validName` (1-128 UTF-8 bytes) and
  `validBoundedText` (<=4096 bytes) reject control characters, terminal
  escapes (`ESC`/C0/`DEL`) and bidi override/isolation controls;
  `CanonicalAssetRevisionSetJSON` caps bindings at 32.
- `internal/assets/authority.go` `create`: enforces name/description/redacted
  summary bounds and dependency/capability caps (<=32).
- `internal/runtime/piadapter/skill_materialization.go`: materialization plan
  binding cap 32.

### P2-2

- `internal/assets/replay.go`: every replayed fact family now validates its
  closed enums and value constraints (definition/revision/candidate/decision/
  lifecycle/rollback/evaluation/import/template/promotion/materialization),
  so unknown enum values and malformed payloads fail rebuild and preserve the
  previously published view.

### P2-3

- `internal/assets/replay.go` + `authority.go`: `EvolutionAssetCandidateCreated`
  now uses an exact payload struct that omits `decision` and
  `decision_event_id` (they appear only in later decision facts and in the
  projection record).
- Snapshot wire result keeps `binding_subjects[]` and `promotion_sources[]`
  and freezes them here (see §3); Go and Swift clients already agree.

### P2-4

- `internal/app/local_product_assets.go`: `record_evaluation` derives all
  measured results from a content-addressed deterministic evaluation fixture
  artifact (see §4) instead of fabricating pass/zero; the evaluation Evidence
  records the fixture-declared results, and the activation gate continues to
  require `quality=pass`, `compatibility=compatible`, `security=pass` and
  `regression != regressed`.

## 3. Frozen snapshot wire additions (supersedes Repair 1 §6 snapshot result)

The `evolution_asset_snapshot` result is exactly:

```text
view_version, next_cursor, definitions[], revisions[], candidates[],
evaluations[], bindings[], materializations[], binding_subjects[],
promotion_sources[]
```

`binding_subjects[]` entry, exact order:

```text
subject_kind, subject_id, subject_version, subject_digest, subject_scope,
subject_project_id, subject_generation_id, subject_identity_digest
```

`promotion_sources[]` entry, exact order:

```text
run_id, run_generation, run_digest, evidence_ids[], evidence_digests[]
```

Both arrays encode `[]` when empty; unknown/duplicate fields fail closed.

## 4. Frozen evaluation fixture artifact schema

`record_evaluation` reads the content-addressed fixture Artifact identified by
`fixture_digest` and requires exact bytes, exact case identity
(`case_ids` sorted equals `requested_case_ids`), `schema_version=1`, and
`fixture_kind` equal to the command value. Exact fields:

```text
schema_version, fixture_kind, case_ids[], expected{
  quality_result, failure_count, usage_observed, usage_microunits,
  cost_observed, cost_microunits, cost_currency,
  compatibility_result, applicable_scope, regression_result, security_result
}
```

Closed values:

```text
quality_result = pass | fail | partial
compatibility_result = compatible | incompatible | partial
regression_result = improved | equivalent | regressed | unknown
security_result = pass | fail | partial
observed=false requires microunits=0 and currency empty; observed=true
requires non-negative microunits and, for cost, a three-letter uppercase
currency (e.g. USD).
```

`failure_count` is `0..len(case_ids)`; `applicable_scope` is non-empty and at
most 4096 UTF-8 bytes; `usage_microunits`/`cost_microunits` are non-negative
when observed and zero otherwise. Replay additionally enforces the same closed
result sets, `previous_lifecycle=candidate|active`, `restored_lifecycle=
candidate` and the currency rule, so unknown enum values fail rebuild and
preserve the previously published view. The evaluation Evidence
Artifact records the exact fixture-declared results, usage/cost and case
identity and is immutable.

## 5. No boundary expansion

No new Journal, store, Event type, stream, authority, capability, dependency,
migration or client surface is added. The Team dispatch authority remains the
sole writer; materialization and evaluation remain Candidate/Evidence-only.

## 6. Independent Review acceptance

The Repair passes only if a fresh read-only Reviewer proves:

1. each of the six findings is closed by the exact code change above, with
   regression tests in owned files;
2. the two schema freezes (§3-§4) are exact and add no authority/security gap;
3. no path outside the P3A-W1 allowlist (plus Owned-path Repair additions)
   changed;
4. no P3A-W2, dependency, migration, staging, push, merge, network or live
   action is authorized by Review PASS.

Any blocking P0/P1/P2 finding returns `FAIL` and keeps the gates closed.

## 7. Review 1 and Review 2 closure (P2-2)

Fresh independent Implementation Repair Review 1 (2026-08-04) returned
`P0=0`, `P1=0`, `P2=1` with five closures exact and P2-2 partial: replay
accepted unknown evaluation result strings/cost currencies and non-frozen
lifecycle roles. Review 2 found two remaining replay-only gaps and a
materialization validation gap. All are now closed:

1. `validReplayEvaluation` enforces the frozen closed sets
   (`quality pass|fail|partial`, `compatibility compatible|incompatible|
   partial`, `regression improved|equivalent|regressed|unknown`,
   `security pass|fail|partial`), `observed=false` requires microunits zero
   and currency empty, `observed=true` requires non-negative microunits and a
   three-letter uppercase currency, and `applicable_scope` is bounded to 4096
   UTF-8 bytes.
2. `validReplayLifecycle` requires `previous_lifecycle=candidate|active` for
   archive and `restored_lifecycle=candidate` for restore.
3. `validMaterializationFact` validates `RuntimeSkillMaterializationPublished`
   team/logical/run/runtime IDs, attempt/generation bounds, all digests, the
   1..32 binding cap and canonical set-digest consistency;
   `PrepareMaterialization` rejects empty binding sets (materialization always
   requires at least one exact bound revision), so write and replay agree;
   `RuntimeSkillMaterializationCleaned` validates run ID, attempt/generation
   and manifest/root digests.
4. Regression coverage in `TestP3AReplayRejectsInvalidEnumsAndTemplatePayloads`
   now includes unknown risk, mismatched template digest, unknown
   quality_result, invalid `previous_lifecycle`, unobserved-with-value
   usage and a malformed materialization fact.

Re-review 3 additionally closed the empty-binding divergence: replay and
`PrepareMaterialization` both require a non-empty 1..32 binding set
(`TestP3AMaterializationRejectsEmptyBindingSet`), so write and replay agree
and materialization always binds at least one exact revision.

## 8. Final Implementation Review closure (P2-1/P2-2)

Fresh independent final Implementation Review returned `P0=0`, `P1=0`,
`P2=3`, Product/Authority `PASS`, Operational/Trace `PASS`. The three P2s
were:

```text
P2-1 internal/assets/replay_test.go was listed in Contract §2 but never created.
P2-2 replay did not enforce CandidateCreated promoted-only fields or verify
     redacted_summary_digest == sha256(redacted_summary).
P2-3 internal/projection/team_execution_test.go carries a documented
     pre-existing unstaged 2-line user change; it must never be staged.
```

Closures:

1. `internal/assets/replay_test.go` is now created and owns replay coverage
   (`TestP3AReplayRebuildsExactSnapshotFromCommittedFacts`,
   `TestP3AReplayRejectsInvalidEnumsAndTemplatePayloads`,
   `TestP3AReplayRejectsLocalCandidateWithPromotedFieldsOrDigestMismatch` and
   the `validReplayEvent` helper), matching Contract §2 exactly.
2. Replay `EvolutionAssetCandidateCreated` now enforces the frozen
   promoted-only field rules (promoted requires non-empty source Run identity,
   generation >= 1, run digest and evidence; local/import requires empty
   source Run fields, generation 0 and empty evidence arrays) and verifies
   `redacted_summary_digest == sha256(redacted_summary)`; malformed facts fail
   rebuild with old-view preservation.
3. P2-3 is recorded: the excluded file stays unstaged in the atomic commit.

The authority write path (`validEvaluation`, `decodeEvaluationFixture`) and
replay now enforce identical bounds, so malformed facts fail rebuild and
preserve the previously published view.

VERDICT: `FROZEN — PENDING REVIEW`
