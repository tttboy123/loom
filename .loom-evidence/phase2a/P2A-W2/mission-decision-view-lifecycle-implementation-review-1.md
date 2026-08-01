# P2A-W2 Final Mission Decision View Lifecycle — Implementation Review 1

**Date**: 2026-08-01

**Review type**: fresh independent read-only Implementation Review

**Verdict**: `FAIL`

## Findings

```text
P0 = none
P1 = 1
P2 = 1
```

### P1 — mandatory vertical test stops at the handler

`TestProductDaemonSnapshotRebindsPreparedDecisionsAfterRuntimeDiscovery`
uses a real Journal, Runtime discovery, `LocalProductReadService` and product
handler, but calls that handler with `localipc.Request` values directly. It
does not prove the new lifecycle through the real Go IPC server's Unix socket,
framing and client.

Existing real-server tests do not close this exact gap: the production-runner
fixture does not perform snapshot -> discovery -> rebound snapshot, while the
strict Swift real-server fixture uses a static handler rather than the real
read service.

Required repair: route the existing lifecycle test through
`localipc.NewServer` and `localipc.NewClient`, read both snapshots over the
socket, commit the Runtime discovery between them, and verify old-command
conflict/current-command success over the same socket.

### P2 — combined digest method text is inaccurate

All five file hashes and the recorded combined digest reproduce in source-lock
file order. The JSON description says the lines were sorted; literal lexical
sorting produces a different digest. The method text must say file order or
the digest must be regenerated using the stated method.

## Accepted implementation analysis

The Reviewer found the production semantics correct:

- successful snapshot refresh/rebind is all-or-nothing;
- Projection-failure cached-pair preservation is fail-closed;
- submission independently refreshes before authority;
- no new Journal/Rules/Work/Grant/Evidence authority or hidden retry exists;
- Swift decoder, IPC method and production daemon assembly did not change;
- the owned diff is exactly five files and excludes unrelated dirt.

Reviewer-focused app/API/daemon tests, race tests, strict Swift real-server
fixture and `git diff --check` passed.

## Gate result

Live and Candidate commit remain locked. Repair 1 must stay inside the existing
test file and evidence/source-lock boundary, then repeat deterministic
verification and fresh independent Implementation Re-review.
