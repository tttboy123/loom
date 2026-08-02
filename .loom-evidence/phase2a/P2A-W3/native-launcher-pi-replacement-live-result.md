# P2A-W3 Native Launcher Pi Replacement Live Result

**Date**: 2026-08-03  
**Attempt**: `phase2a-w3-live-20260802-pi-005`  
**Manifest SHA-256**:
`6eb40813739faff661b3e93dfd7b5dea395bc11d903a0a118186494b479a4d92`  
**Canonical invocation SHA-256**:
`d91c670f9e0be2695e26c6175a34acb44d2c60e92d92d62bc55fde386a115a65`  
**Allowance**: consumed once; no retry  
**Verdict**: `FAIL — SOURCE + VERIFIER CLOSED / TEAM TERMINAL ABSENT`

## Exact launch and product action

The launch transaction independently reproduced the frozen canonical digest,
32-token count, first `--state` token and exact allowlisted flag multiset before
creating the single start marker. It passed that array directly after exact
loomd with no positional subcommand. One binary invocation published the
private product socket; daemon stderr remained empty.

Computer Use opened the exact signed native bundle. The ordinary New Mission
surface selected saved Team `P2AW3ControlledTeam`, Coding, and objective:

```text
Ship the reviewed release
```

One and only one read-only preflight returned:

```text
1 node · native_auth · qwen2.5-coder-1.5b-instruct-q4-k-m
Capacity available: 1
Permissions: artifact.read, repository.read, workspace.edit, workspace.test
Approvals: rule.template.coding-review, rule.template.protected-write
Preflight is ready. Starting still requires your click.
```

The Controller clicked `Start Mission` exactly once. No retry, direct IPC
command, review decision, prepared decision, control action, compaction or
Provider request occurred.

## Successful bounded execution layers

The authority appended one Team plan, one node schedule and one ready-set
dispatch. The source Main and independent Verifier each received distinct
WorkItem, Run, generation, Grant and Evidence lineage. Both Runs committed
`succeeded`, both capacity reservations were released, and both Grants were
revoked terminally.

Exact key facts include:

```text
TeamExecutionPlanned       1
TeamNodeAttemptScheduled   1
TeamReadySetDispatched     1
WorkItemCreated            2
WorkItemAssigned           2
RunClaimed                 2
RunStarted                 2
RunTerminalCommitted       2
AgentGrantIssued           2
AgentGrantRevoked          2
RuntimeCapacityReserved    2
RuntimeCapacityReleased    2
EvidenceSubmitted          2
TeamNodeAttemptTerminal    1
```

The source node terminal is `succeeded`, output classification
`valid_nonempty`, with Evidence digest
`4986e4e383ee4f9affa2826af45b13be320828fba0b714f97bf2872f3bb3393a`.
The independent Verifier Evidence digest is
`dba87f3fa90b2e477707f18838837f40e3750757fdb2ccd6ecb0397fabd4d343`.

The native product visibly reached `Ready For Review`, rendered tentative
output `Done`, and stated that accepted Evidence references were available.
This correctly preserves the invariant that an executor cannot mark itself
done.

## Gate-blocking terminal gap

After both terminal Runs and both Evidence submissions, the Journal stopped at
44 events. It contains exactly zero:

```text
TeamNodeAcceptanceCommitted
VerificationDecisionCommitted
TeamExecutionTerminal
```

One read-only native refresh still showed `Ready For Review`; no later Event
appeared. The manifest requires one canonical Team terminal and therefore
fails even though all lower execution layers completed successfully. The
Controller did not use the read-only Review Gate to manufacture authority or
reinterpret `Ready For Review` as terminal success.

## Shutdown and postflight

The native app quit and one exact interrupt stopped the single daemon with exit
`0`. Its only stdout record was one no-write Runtime observation cycle; stderr
is empty. SQLite integrity is `ok`; final SHA-256 is:

```text
ddf6ddf10280d2d37b9aff26c1b58bb462ab7033466883f2b87e9330d07dca75
```

Product socket and IPC lock are absent, isolation is empty, no attempt
daemon/native/Pi/llama process remains, and the ordinary retained SQLite lock
file has no holder. The bounded textual evidence scan found no API-key or
bearer-token pattern. The unrelated `demo-resident` observer was not signalled
or modified.

**VERDICT**:
`FAIL / LIVE EXECUTION PROVED THROUGH TWO EVIDENCE LINEAGES / CANONICAL TEAM TERMINAL MISSING / NO RETRY`
