# P2A-W1 Reopen 4 Implementation Review 3

**Date**: 2026-07-28
**Reviewer**: independent read-only Reviewer
**Verdict**: `PASS`
**Blocking findings**: none
**Live action**: none

## Findings

1. Server lifecycle tests use the exact `Server.Ready()` barrier before active
   lifecycle assertions. Path polling remains only in client transport tests.
2. The replacement preservation proof passed under:

   ```text
   go test -race ./internal/localipc -count=50
   go test -race ./internal/localipc \
     -run '^TestServerReclaimsOnlyStaleSocketAndPreservesReplacement$' \
     -count=200
   ```

3. Prior findings remain closed:
   - zero-Team refresh clears stale Team/timeline state;
   - `r` refreshes the snapshot rather than the stale timeline;
   - all-zero PIDs return `invalid_pid`;
   - file-kind identity rejects equal-device/inode type changes.
4. Focused TUI, helper, local-IPC count 100, format, diff, and staged-empty
   checks passed independently.
5. Fresh builds match the active Candidate:

   ```text
   loom   7ba4b41d143ea64fef7cafbfc463f4524ecd3790f93291a38a9c1812278d6ffd
   loomd  f9529cf62a1bccbebe4d812dc7408624d2c1ee4c138b92fe69f1dfa497280bbb
   ```

## Gate result

Implementation Re-review 3 is `PASS`. This completes deterministic Reopen 4
implementation review only. It does not activate the replacement canary,
installation, Computer Use, staging, commit, or P2A-W2.
