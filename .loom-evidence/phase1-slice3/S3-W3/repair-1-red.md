# S3-W3 Repair 1 Mandatory RED

- Baseline: `5517a06`
- Repair Contract SHA256:
  `b15d742d8b2f8ec312d03acaf79d2051e4840fefcd21fdf29bbafc50399734e5`
- Date: `2026-07-26`
- Product files changed for Repair 1 before RED: none

All four Repair markers occur exactly once.

Command:

```text
go test ./internal/authorization ./internal/projection -count=1
```

Result: exit `1`, as required.

The current Candidate failed exactly on the frozen Repair gaps:

- `Issue` rejected opaque IDs already accepted by S3-W2;
- `Authority.Snapshot` accepted missing, wrong-stream, and wrong-sequence
  historical Run references;
- three Journal collision sentinels escaped without
  `ErrGrantAuthorityConflict`;
- Grant authority and projection decoders accepted identical duplicate JSON
  keys at root and nested levels.

The projection trailing-top-level-value check already passed. No existing
behavior failed and no infrastructure/source-mutation error was used as RED
evidence.

VERDICT: PASS
