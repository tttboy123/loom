# S3-W2 Contract Review 1

- Reviewed baseline: `c21a8f1`
- Parent contract SHA-256:
  `850ab304f077f99a9d9f2cc7cbd1dbd15be17971cd731111d4fd1b2bc1f96993`
- Date: `2026-07-25`

## Blocking findings

1. The proposed Runtime stream spelling did not match accepted
   `runtime_instance:<id>` discovery/status facts, so CAS could miss a
   concurrent status change.
2. A successful Run terminal incorrectly allowed the executor path to mark its
   WorkItem `done`, bypassing `ready_for_review -> verifying -> done`.
3. Prepare-lease expiry incorrectly allowed reclaim of a running Run even
   though running liveness belongs to heartbeat/process supervision.
4. Lexicographic stream replay can encounter `run/...` before
   `work-item/...`; cross-stream prerequisites require dependency-aware
   projection rather than stream-name ordering.

No product file was changed. `git diff --check` passed.

VERDICT: FAIL
