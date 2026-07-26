# S3-W5 Contract Amendment 2 Review 1

- Date: `2026-07-26`
- Baseline: `cd1e594`
- Reviewer: fresh independent Reviewer
- Subject:
  `.loom-evidence/phase1-slice3/S3-W5/contract-amendment-2.md`

## Findings

- The owned scope matches Slice 3 Exit Contract Amendment 3 exactly. Only the
  four accepted S3-W4 Supervisor/Pi Adapter files are reopened.
- The Frame trust order is closed and implementable: Adapter decode, Supervisor
  bridge/binding/sequence validation, immutable candidate construction,
  `AgentGrant.Authorize`, bound-stream commit, then optional observer delivery.
- Attempt recovery is deterministic and bounded to three attempts. It uses
  explicit `retry_at`, performs no sleeping or hidden retry, and does not infer
  output semantics inside the scheduler.
- Every retry receives independent WorkItem, Run, generation, Grant, and
  Evidence lineage while retaining one logical node identity.
- Projection and `GlobalReadView` remain rebuildable caches rather than a
  second state authority.
- Observable output is limited to authorized Evidence artifact payloads; raw
  Grants, credentials, and hidden reasoning remain excluded.
- The mandatory RED matrix and controlled canary cover the amendment's trust,
  concurrency, recovery, and immutability boundaries.
- Slice 4 recovery policy and Slice 5 client delivery are explicitly deferred;
  no S3-W6 or thin follow-up WorkItem is created.
- `git diff --check` passed for the reviewed governance candidate.

## Verdict

The amendment is bounded, preserves the accepted authority model, and provides
an implementable vertical contract for S3-W5.

VERDICT: PASS
