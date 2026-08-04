# P3A-W1 Owned-path Repair Review 1

Date: `2026-08-04`

Reviewer: fresh independent read-only Reviewer (Codex CLI, read-only sandbox)

Reviewed Repair: `.loom-evidence/phase3a/P3A-W1-OWNED-PATH-REPAIR.md`

## Verdict

```text
P0 = 0
P1 = 0
P2 = 1
Product/Authority: FAIL
Operational/Trace Governance: PASS
Overall Repair: FAIL
```

## P2-1 finding

The legacy `RunClaimed` projection branch set `AssetLineageAvailable=false` and
`AssetRevisionBindings=[]` but did not clear `AssetRevisionSetDigest`,
`MaterializationManifestDigest` or `MaterializationRootDigest`. A legacy
reclaim replayed onto a run record that previously carried lineage would
expose stale digest strings while reporting `available=false`, unlike the
sibling attempt projection and Amendment §2.2 empty/false semantics.

## Verified closures (non-blocking context)

- Parent Contract/Repair/Amendment digests match disk; the two seam files'
  freeze-time digests matched disk.
- Run projection edit otherwise adds only the four frozen lineage fields with
  all-or-nothing validation, deep copies and malformed-fact old-view
  preservation; no authority/schema change.
- Swift `--assets` probe is read-only through the production
  `evolution_asset_snapshot` method; no mutation or bypass.
- No other product path changed outside the allowlist plus the two seams;
  documented exclusions hold; no P3A-W2.

The Reviewer could not execute Go/Swift tests in the strictly read-only
sandbox; compilation was verified statically. Closure and re-review are
recorded in `P3A-W1-OWNED-PATH-REPAIR.md` §5 and
`P3A-W1-OWNED-PATH-REPAIR-REVIEW-2.md`.
