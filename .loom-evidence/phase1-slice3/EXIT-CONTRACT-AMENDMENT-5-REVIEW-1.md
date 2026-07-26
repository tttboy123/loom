# Slice 3 Exit Contract Amendment 5 Review 1

Reviewer: independent read-only contract reviewer

Date: 2026-07-26

## Findings

None.

## Evidence

- The accepted S3-W5 contract requires crash/reopen recovery and exact Run /
  Evidence identity reuse, while current authorized Frames and canonical
  Evidence bytes exist only in the application process between managed
  execution and Artifact publication.
- The accepted content-addressed Store exposes publish-by-digest but no durable
  attempt-to-digest receipt. The proposed private capture/receipt boundary is
  therefore necessary rather than an optional cache optimization.
- Work Authority already supports expired claimed-Run reclaim at a higher
  generation, but Team attempt state remains bound to the dispatch generation.
  The exact Team rebound fact is necessary to preserve stale-generation fencing.
- The reopened scope is narrow: three new same-package Evidence files plus
  already-owned S3-W5 Work, Projection, App, evidence, and ADR-0009 files.
- The capture remains a private delivery spool, never execution, terminal,
  acceptance, Projection, Journal, or checkpoint authority.
- TECH-PLAN permits content-addressed artifacts plus SQLite Evidence metadata
  and requires higher-generation reclaim with old-Grant revocation. The
  amendment follows those boundaries.
- W6, daemon/resident activation, client delivery, model/process checkpoint,
  Provider fallback, second scheduler, second StateWriter, and second authority
  remain excluded.
- `git diff --check` passed.

VERDICT: PASS
