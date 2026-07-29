# P2A-W1 Reopen 4 Proof Repair 2 RED

**Date**: 2026-07-28
**Status**: `RED — INDEPENDENT REVIEW REPRODUCTION`
**Live action**: none

The frozen race replacement gate failed under concurrent focused review:

```text
go test -race ./internal/localipc \
  -run '^TestServerReclaimsOnlyStaleSocketAndPreservesReplacement$' \
  -count=30
```

Relevant failure:

```text
Serve() replacement error = context canceled
```

Source inspection proves `waitForSocket` observed the intentionally retained
stale socket, not the new server lifecycle. The exact proof repair must wait on
the existing `server.Ready()` channel before replacing the path.

No product implementation, timeout, assertion, live service, installed state,
SQLite, credential, Provider, Runtime, staging, or authority changed during
this RED.
