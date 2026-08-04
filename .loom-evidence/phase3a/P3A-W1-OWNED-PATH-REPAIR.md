# P3A-W1 Owned-path Repair: Run Projection Lineage and Swift Contract Probe

Date: `2026-08-04`

Status: `FROZEN — INDEPENDENT AMENDMENT REVIEW REQUIRED`

WorkItem: the existing and only `P3A-W1`. This Repair does not create
`P3A-W2`, a new authority, a new database, a second writer or a thin
adapter/coordinator WorkItem.

Reviewed parent set:

- P3A-W1 Contract SHA-256:
  `12b8c1f2595c16cb7faabe889982182635b45d3d836f0ccc32e139357629fc9c`;
- Contract Repair 1 SHA-256:
  `5a2b16778b5cccb989496f1918d0d81a59b140feb38e55716dbd7574d6a25a00`;
- Contract Repair 2 SHA-256:
  `b59b178b80bb1146a0e0e803846d097cf9db648ee7b7b9dbda0e102325127796`;
- Exact Lineage/Materialization/Journey Amendment SHA-256:
  `5e333ffd14c2a077a4330e5c2e095de6c7302297878d98eefc87e0936c82054d`;
- Amendment Review 1: `P0=0`, `P1=0`, `P2=0`, dual `PASS`.

This Repair supersedes only the owned-path closure named below. Every
unchanged parent/Amendment security, authority, exact-schema, RED,
verification, cross-client, review, exclusion and single-commit requirement
remains frozen. No product, live or client action may rely on this Repair
until a fresh independent read-only Reviewer returns `P0=P1=P2=0` and dual
`PASS`.

## 1. Discovery that requires the Repair

The reviewed Exact Lineage Amendment §2.2 freezes four Attempt-specific
lineage fields (`asset_revision_bindings`, `asset_revision_set_digest`,
`materialization_manifest_digest`, `materialization_root_digest`) and requires
them to be copied byte-for-byte into "Team Attempt and Run Projection
records". The accepted Run read model that projects `RunClaimed` payloads and
`RuntimeSkillMaterializationPublished` facts lives in
`internal/projection/run_authority.go`, which the parent Contract owned-path
list omitted (it owned the authority counterpart
`internal/work/run_authority.go` but not the projection counterpart).

The mandatory strict Go/Swift journey wire identity evidence also reuses the
accepted production Swift contract probe
(`apps/macos/Sources/LoomLocalAppContractProbe/main.swift`, an accepted
Phase 2A path) by adding one read-only `--assets` surface backed by the
production `evolution_asset_snapshot` IPC method. That file was likewise not
listed in the parent owned-path allowlist.

Both edits are minimal, contract-required seams. Reverting them would break
the frozen Amendment §2.2 projection-lineage requirement and remove the
strict Swift journey-wire proof; moving the behavior elsewhere would create a
duplicate projection or probe and violate the single-authority/no-second-read
model rule.

## 2. Exact owned-path addition

The P3A-W1 owned-path allowlist is extended by exactly these two existing
files:

```text
internal/projection/run_authority.go
apps/macos/Sources/LoomLocalAppContractProbe/main.swift
```

Current SHA-256 at freeze time:

```text
internal/projection/run_authority.go
  01540aabd26987b14fa043a429c7a80dd94a176324e0693006b09d2f2256729f
apps/macos/Sources/LoomLocalAppContractProbe/main.swift
  60e5af37c5679ba1d22c409d85cbce2680fdaf765ade94bf50dd936e61a7789a
```

Permission is limited to:

1. `internal/projection/run_authority.go`: deep-copy and project the four
   frozen Attempt lineage fields from `RunClaimed` payloads and
   `RuntimeSkillMaterializationPublished` facts, explicit
   `asset_lineage_available=false` for legacy facts, rebuild/malformed-fact
   preservation identical to the rest of the accepted Run projection, and
   error-context-only wrapping that changes no authority semantics.
2. `apps/macos/Sources/LoomLocalAppContractProbe/main.swift`: one read-only
   `--assets` probe branch calling the production `evolution_asset_snapshot`
   method with the canonical journey UUID and printing a bounded count/view
   summary. It performs no mutation, no direct Journal/SQLite access and no
   service bypass.

No other path is added. `internal/projection/team_execution_test.go`, the
pre-existing dirty files (`AGENTS.md`, `PROGRESS.md`, `README.md`,
`.loom-evidence/phase1-final-live-gate/**`) and every excluded user path
remain unowned, unedited and unstaged.

## 3. No boundary expansion

This Repair adds no schema, Event, stream, capability, IPC method, Journal
store, Evidence store, credential, Provider, dependency, migration or client
surface beyond the reviewed Amendment. The Team dispatch authority remains
the sole writer. The probe reads only through the production daemon socket.

## 4. Independent Review acceptance

The Repair passes only if a fresh read-only Reviewer proves:

1. both additions are the minimal contract-required seams and no other path
   changed outside the allowlist;
2. the Run projection edit preserves accepted authority semantics, legacy
   readability and malformed-fact old-view preservation;
3. the Swift probe is read-only, uses the production IPC method and cannot
   bypass authority or mutate state;
4. no P3A-W2, dependency, migration, staging, push, merge, network or live
   action is authorized by Review PASS.

Any blocking P0/P1/P2 finding returns `FAIL` and keeps the existing gates
closed.

## 5. Review 1 closure (P2-1)

Fresh independent Review 1 (2026-08-04) returned `P0=0`, `P1=0`, `P2=1`,
Product/Authority `FAIL`, Operational/Trace Governance `PASS`. The single P2:

> Legacy branch does not fully reset Run lineage to empty. When a `RunClaimed`
> payload carries no lineage fields, the projection sets
> `AssetLineageAvailable=false` and `AssetRevisionBindings=[]` but does not
> clear `AssetRevisionSetDigest`, `MaterializationManifestDigest` or
> `MaterializationRootDigest`. A legacy reclaim replayed onto a run record that
> previously carried lineage would expose stale digest strings while reporting
> `available=false`, unlike the sibling attempt projection.

Closure (inside the two Repair-owned files only):

1. `internal/projection/run_authority.go`: the legacy `else` branch now resets
   all three digest strings to `""`, matching the attempt-projection
   convention and Amendment §2.2 empty/false semantics.
2. Regression RED added in the owned `internal/projection/projection_test.go`:
   `TestP3ARunProjectionLegacyReclaimClearsAllLineageFields` — a generation-2
   legacy `RunClaimed` after a lineage-bearing generation-1 claim must project
   `asset_lineage_available=false`, zero bindings and all three empty digest
   strings. It failed on the pre-fix code with exactly the stale-digest
   symptom and passes after the fix; the full `internal/projection` suite is
   green.

Status: `FROZEN — PENDING RE-REVIEW`

VERDICT: `FROZEN — PENDING REVIEW`
