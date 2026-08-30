# RT2-4 / RT2-5 Acceptance And Portability

Status: `ACCEPTED / INSTALLED BUILD 211`

## Candidate acceptance

The latest successful main-seat Attempt in the latest round is the Moderator
candidate. The workbench displays its visible output, but Loom publishes no
AlignmentSummary until the user explicitly selects `Accept conclusion`.
Unresolved peers require Retry, Replace or Skip first.

The published summary binds the Mission context, candidate Attempt, encrypted
payload reference and output digest, frozen execution binding digest and
Context Capsule digest. Model output remains outside Journal metadata.

## Local portability

- Export is available only for a concluded session.
- The portable document is bounded to 1 MiB and expires after one day.
- Import validates exact schema, digest chain, validity window, provenance and
  equality of top-level Session/Moderator/conclusion identity with the embedded
  AlignmentSummary.
- Imported records are read-only and preserve Mission context.
- Invalid, expired or changed files fail as `export_invalid`.

## Installed gate

Installed acceptance ran a real two-seat RoundTable, preserved a seat-local
failure and a successful peer, exercised Pause/Steer/Retry, accepted the
Moderator candidate, published the digest-bound AlignmentSummary, exported a
private bounded document, reopened it read-only after restart, and rejected
invalid portability input. Build 208 is the real Provider intervention/runtime
checkpoint. Build 211 additionally proves bounded file reading and rejects
re-digested identity substitution before read-only reconstruction. The retained
export artifact and installed evidence are under `.loom-evidence/phase4/live/`.
