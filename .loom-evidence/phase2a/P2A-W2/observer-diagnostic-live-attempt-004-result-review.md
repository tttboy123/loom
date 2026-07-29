# P2A-W2 Observer Diagnostic Live Attempt 004 Result-Evidence Review

**Date**: 2026-07-30
**Mode**: fresh independent read-only
**Verdict**: `PASS` for the recorded failed outcome

## Findings

- P0: none.
- P1: none.
- P2: the retained live artifact does not independently preserve the exact
  Codex `login status` stdout/stderr line and count. The compatibility claim is
  accepted only at the narrow boundary that the locked Codex 0.144.1 process
  passed the production observer and the native UI showed `Available`. This
  does not prove model execution, task dispatch or broader Provider behavior.
- P2: the result file did not freeze a reproducible pre/post resident daemon
  PID and binary-hash tuple. Current read-only inspection found a separate
  `demo-resident/bin/loomd` process at PID `12632` and no attempt-004 process.
  This is an evidence-precision gap, not a blocker to the failed verdict.

## Independently reproduced facts

- attempt root exists, mode `0700`, uid `501`;
- attempt database is regular `0600`, uid `501`, SHA-256
  `5fa9eb4f3f8e836f9ea9421ecd93a7153213f9d01533453922d8cbb94d3f203e`,
  with SQLite `integrity_check=ok`;
- source attempt-002 database is regular `0600`, uid `501`, SHA-256
  `b0312739ba7b24961b9e8a698e4cbf436e9cace8758a1d2a058e050aec475e22`,
  with SQLite `integrity_check=ok` and five Events;
- attempt Event counts are one `ProviderCredentialConfigured`, three
  `ProviderCredentialVerified`, two `RuntimeInstanceDiscovered`, total six;
- the only delta is the new
  `runtime_instance:runtime.pi.earendil-works.0.82.1` stream at sequence `1`;
- zero TeamDefinition, TeamInstance, AgentInstance, WorkItem, Run, Grant,
  Evidence and dispatch-class Events;
- exact attempt daemon and native executable hashes match the result;
- frozen Codex, Pi, Node, llama-server and GGUF identities match;
- isolation is empty; product socket and lock are absent;
- no attempt-004 app, daemon, Pi or llama process remains.

## Conclusion

`FAIL — NATIVE_JOURNEY_INCOMPATIBLE_AND_SHUTDOWN_STALLED` is accurate. The sole
replacement canary is consumed. P2A-W2 remains unaccepted, P2A-W3 remains
locked, no P2A-W4 exists, and no retry or additional single-point amendment is
authorized.

The Reviewer edited, staged and committed nothing, launched no product/live
surface, and did not access credential payloads, references, Keychain or
network.
