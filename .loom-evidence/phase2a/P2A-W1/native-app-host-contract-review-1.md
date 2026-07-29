# P2A-W1 Native App Host Contract Review 1

**Date**: 2026-07-28
**Reviewer**: fresh independent read-only Reviewer
**Verdict**: `FAIL`

## Finding

### Medium: Swift protocol contract omitted one real Go v1 error code

The Revision required a closed typed error mapping but omitted
`unsupported_platform`.

That code is:

- part of the accepted W1 IPC safe-error set;
- defined by the actual Go `localipc` implementation; and
- emitted when server-side peer credential inspection is unavailable.

The proposed Swift contract and its “all closed protocol variants” proof could
therefore pass without covering one valid daemon response.

## Required repair

Add `unsupported_platform` to:

1. the exact Swift closed typed error set;
2. malformed/closed error compatibility tests; and
3. the Go-server/Swift-probe component matrix.

No RED, implementation, build, install, live canary, P2A-W2, or P2A-W4 work is
authorized by this failed Review.

## Non-blocking assessment

The Reviewer found the remaining direction correctly scoped:

- one vertical P2A-W1 revision;
- Reopen 4 remains failed with allowance `0`;
- native state remains in-memory/read-only over the existing daemon UDS;
- historical Cockpit bridge/cache/workspace behavior is quarantined;
- no live authority is silently granted.
