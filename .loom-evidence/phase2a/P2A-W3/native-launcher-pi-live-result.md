# P2A-W3 Native Launcher Pi Live Result

**Date**: 2026-08-02  
**Attempt**: `phase2a-w3-live-20260802-pi-004`  
**Frozen manifest SHA-256**:
`0b76124d920abdd2eac8fc9c1e1ce05e7c8f66547e074eefbabc0442ede0feaa`  
**Implementation lock**:
`f6bb68656cd70b2c9d75861bc5a77325ea488bd78dcfa49fc0c1cc41dd64d772`  
**Allowance**: consumed once; no retry  
**Verdict**: `FAIL — CONTROLLER INVOCATION ERROR BEFORE DAEMON START`

## Preflight

Pi remained unstarted until MiniMax-004 had fully stopped. Immediately before
this attempt, the exact initial SQLite, loomd, loom and native executable
hashes matched the frozen manifest; root/state/manifest modes were `0700`,
`0600`, `0600`; SQLite integrity was `ok`; the product socket/lock and start
marker were absent; and the authority contained the exact retained six facts.

## Consumed invocation

The Controller created the one start marker and invoked the exact locked loomd
binary once, but incorrectly inserted an extra `daemon` positional argument
before the manifest's frozen flag list. The binary rejected the non-manifest
command line during argument parsing:

```text
exit code: 2
stdout:    empty
stderr:    invalid input
```

This is a Controller/orchestration defect, not a Loom product failure. The
product daemon did not publish a socket, start Runtime discovery, initialize
the local model, launch Pi, construct Mission execution, serve preflight or
receive Start. No replacement command was attempted because the contract says
that a failed lineage is consumed and forbids retry.

## Authoritative postflight

The retained SQLite is byte-identical to preflight:

```text
677624b624b68ddd908939766752177a802bd90b657075133ef845e67e5461a2
```

Integrity is `ok` and complete facts remain exactly:

```text
AgentGrantIdentityIndexInitialized | 1
AgentInstanceCreated                | 1
RuntimeInstanceDiscovered           | 1
TeamDefinitionSaved                 | 1
TeamInstanceCreated                 | 1
WorkRunIdentityIndexInitialized    | 1
```

There is zero WorkItem, Run, Grant, Frame, Evidence, dispatch or TeamExecution
fact. Product socket/lock are absent, isolation is empty, attempt-controlled
Pi/llama/native/daemon processes are absent, and the textual retained evidence
contains no API-key or bearer-token pattern. The unrelated `demo-resident`
observer was not signalled or modified.

**VERDICT**: `FAIL / EVIDENCE TRUSTWORTHY / PRODUCT CLAIM NOT EXERCISED / NO RETRY`
