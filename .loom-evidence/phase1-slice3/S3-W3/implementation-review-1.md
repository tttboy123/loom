# S3-W3 Implementation Review 1

- Baseline: `5517a06`
- Candidate: uncommitted S3-W3 owned scope
- Reviewer role: fresh independent read-only implementation reviewer
- Date: `2026-07-26`

## Findings

1. `P1` — S3-W3 narrowed the accepted S3-W2 opaque-ID language. S3-W2 accepts
   trimmed valid UTF-8 without control characters, while S3-W3 accepted only
   ASCII alphanumeric plus `._:-/`. A legitimate accepted Run could therefore
   be ungrantable.
2. `P1` — operational `Authority.Snapshot` rebuilt Grant streams without
   verifying that each persisted Run stream/sequence/Event reference named an
   exact historical Run Event. Projection replay checked the reference, but
   the operational authority did not fail closed.
3. `P2` — Grant writes normalized only `journal.ErrStreamHeadConflict`.
   Sequence, idempotency, and partial-batch conflicts could escape the frozen
   `ErrGrantAuthorityConflict` surface.
4. `P2` — the shared projection and Grant decoders rejected unknown/missing
   fields but allowed duplicate JSON object member names.

The reviewer independently ran focused tests, repository tests, and vet; all
passed, confirming that these were missing discriminating cases rather than
already-detected failures. No files were edited by the reviewer.

VERDICT: FAIL
