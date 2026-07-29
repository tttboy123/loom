# P2A-W1 Reopen 4 Implementation Review 2

**Date**: 2026-07-28
**Reviewer**: independent read-only Reviewer
**Verdict**: `FAIL`
**Live action**: none

## Required finding

The real replacement proof remained timing-sensitive under concurrent race
verification. The test deliberately created and retained a stale socket before
starting `Serve`, then called `waitForSocket`, which returned immediately for
that stale path. It could therefore remove/replace the path before the new
Server completed stale-socket reclamation and listener readiness.

The Reviewer observed:

```text
Serve() replacement error = context canceled
```

under the frozen race count. A later serial pass did not erase the earlier
failure.

## Verified closed

- zero-Team refresh clears stale Team/timeline state;
- `r` cannot re-request that stale timeline;
- all-zero PIDs return `invalid_pid`;
- file-kind identity and active source/Candidate hashes are exact.

The required repair is an exact `Server.Ready()` barrier in the already-owned
test. Timeout extension, assertion weakening, retry, or product behavior change
is not authorized.
