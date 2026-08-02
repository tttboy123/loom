# P2A-W3 Native Launcher MiniMax Live Result

**Date**: 2026-08-02  
**Attempt**: `phase2a-w3-live-20260802-minimax-004`  
**Frozen manifest SHA-256**:
`889350d951fd9f7b3285a5500df9ddb5d78a7b1cf5701934b5d0d17b778c75d4`  
**Implementation lock**:
`f6bb68656cd70b2c9d75861bc5a77325ea488bd78dcfa49fc0c1cc41dd64d772`  
**Allowance**: consumed once; no retry  
**Verdict**: `PASS — ONE AUTHORITATIVE REJECTED/UNAVAILABLE TERMINAL`

## Exact live action

The Controller started the exact manifest-locked daemon once. Construction
crossed the official user-level npm Codex launcher and published the private
product socket. The exact signed native app opened successfully and displayed
the ordinary `Runtime & Providers` surface:

```text
Codex   Available
MiniMax Verified
```

Computer Use clicked `Test` exactly once. The visible MiniMax state became:

```text
Unavailable
```

No second Test, Mission preflight, Mission Start, direct IPC command, Provider
retry, compaction, model generation or local-model initialization occurred.

## Authoritative result

The initial authority had one `ProviderCredentialConfigured`, three
`ProviderCredentialVerified` and one `RuntimeInstanceDiscovered` fact. The
single Test appended exactly one next-revision terminal:

```text
stream       provider-credential/minimax
sequence     5
event type   ProviderCredentialVerified
revision     5
status       rejected
reason       unavailable
event ID     setup-6664863e73f7cdb350dea1ea04f0ed3c
```

This is an allowed fail-closed terminal. The payload contains only the opaque
credential reference and public verification metadata; it contains no raw
credential, Provider response, local path or private error.

Complete final fact counts are:

```text
ProviderCredentialConfigured | 1
ProviderCredentialVerified   | 4
RuntimeInstanceDiscovered    | 1
```

No WorkItem, Run, Grant, Frame, Evidence, dispatch, TeamExecution or Mission
fact exists. The setup-only daemon received no local-model flags, created no
execution/isolation content and made at most the one contract-permitted
Provider verification request.

## Shutdown and postflight

The native app was closed and quit. One exact interrupt stopped the single
daemon with exit `0`. It reported one ordinary no-write observation cycle:

```json
{"completed_cycles":1,"discovery_events":0,"status_events":0,"no_write_cycles":1}
```

SQLite integrity is `ok`; final SHA-256 is:

```text
8c1f1031d4aaefb23c3bed890ae2ae92ce5ee2115f0c9a024587538d7434fe59
```

The daemon stderr is empty. The product socket and lock are absent, isolation
is empty, and the daemon plus native app processes are absent. The unrelated
`demo-resident` observer was not signalled or modified.

**VERDICT**: `PASS — SINGLE TEST / SINGLE JOURNAL TERMINAL / CLEAN SHUTDOWN`
