# P2A-W3 Authoritative Acceptance/Recovery Pi Live Result

**Date**: 2026-08-03  
**Attempt**: `phase2a-w3-live-20260803-pi-006`  
**Manifest SHA-256**: `c765a4f48cf2185418af592e22e523d4d8348ed3bbc930399238e4c1c220586f`  
**Canonical invocation SHA-256**: `1d80ca8604a8cf9b233fbfdc2ebd41c3b5492f5cf17af680ffab8f3bb276c715`  
**Allowance**: consumed once; no replacement or Controller retry  
**Verdict**: `PASS — BOUNDED RECOVERY + AUTHORITATIVE TEAM TERMINAL CLOSED`

## Exact product transaction

The Controller reproduced the frozen executable plus 32-token argv digest,
created the single start marker and invoked exact loomd once. The private
product socket became mode `0600`; daemon stderr remained empty.

Computer Use opened the exact signed Pi-006 native bundle and used only the
ordinary product surface:

- `New Mission`;
- saved Team `P2AW3ControlledTeam`;
- work type `Coding`;
- objective `Ship the reviewed release`;
- one `Review preflight`;
- one `Start Mission`.

The single preflight rendered:

```text
1 node · native_auth · qwen2.5-coder-1.5b-instruct-q4-k-m
Capacity available: 1
Permissions: artifact.read, repository.read, workspace.edit, workspace.test
Approvals: rule.template.coding-review, rule.template.protected-write
Preflight is ready. Starting still requires your click.
```

No direct IPC decision, second Start, second preflight, Provider request,
compaction or Controller retry occurred.

## Live bounded recovery and terminal closure

Attempt 1 completed its source Run and independent Verifier Run with distinct
WorkItem, Run, generation, Grant and Evidence identities. Work Authority then
committed one timestamp-bearing rejected acceptance. Rules selected one explicit
bounded `retry`, and Work Authority atomically committed
`TeamNodeRecoveryRecorded` plus the exact attempt-2 schedule.

Attempt 2 created a wholly new source and independent Verifier lineage. Both
terminal Runs and both Evidence submissions completed; Work Authority committed
the accepted verification/outcome/node transaction and the one canonical Team
terminal in the same Authority instant.

The key Team stream order is exact:

```text
row  7  TeamExecutionPlanned
row  8  TeamNodeAttemptScheduled        attempt 1
row 14  TeamReadySetDispatched          attempt 1
row 37  TeamNodeAttemptTerminal         attempt 1 succeeded
row 55  WorkItemVerificationCommitted   attempt 1 rejected
row 56  WorkItemRejected                attempt 1
row 57  TeamNodeAcceptanceCommitted     attempt 1 rejected
row 58  TeamNodeRecoveryRecorded        retry
row 59  TeamNodeAttemptScheduled        attempt 2
row 65  TeamReadySetDispatched          attempt 2
row 82  TeamNodeAttemptTerminal         attempt 2 succeeded
row 100 WorkItemVerificationCommitted   attempt 2 accepted
row 101 WorkItemDone                    attempt 2
row 102 TeamNodeAcceptanceCommitted     attempt 2 accepted
row 103 TeamExecutionTerminal           succeeded
```

Authority nanosecond timestamps advance across the real operations:

```text
plan/attempt-1 dispatch  1785697283688370000
attempt-1 acceptance     1785697288011253000
recovery transaction     1785697288012713000
attempt-2 acceptance     1785697289587743000
Team terminal            1785697289587743000
```

The explicit Journal-recorded recovery is not a hidden live retry. There was
still one daemon invocation, one preflight and one Start. It additionally
live-proves the repaired acceptance-to-recovery boundary rather than bypassing
it.

## Exact authority totals

Final SQLite contains `103` Events and integrity is `ok`:

```text
TeamExecutionPlanned          1
TeamNodeAttemptScheduled      2
TeamReadySetDispatched        2
TeamNodeAttemptTerminal       2
WorkItemCreated               4
RunClaimed                    4
RunStarted                    4
RunTerminalCommitted          4
AgentGrantIssued              4
AgentGrantRevoked             4
RuntimeCapacityReserved       4
RuntimeCapacityReleased       4
EvidenceSubmitted             4
WorkItemVerificationCommitted 2
TeamNodeAcceptanceCommitted   2
TeamNodeRecoveryRecorded      1
WorkItemRejected              1
WorkItemDone                  1
TeamExecutionTerminal         1
```

Every capacity reservation is followed by its matching release before the next
Run reserves capacity; capacity was never oversold. All four Run, Grant and
Evidence identities are distinct. The native read-only refresh after row 103
left the Event count at `103`, proving zero reconnect/refresh writes.

## User-visible terminal

One native read-only refresh rendered:

```text
Complete · Succeeded
Outcome Mission completed
Plan 1 node(s), 1 complete, 0 in review.
Accepted Evidence references are available in the Inspector.
Main Terminal · Attempt 2
```

The terminal screenshot is retained as mode `0600` attempt evidence with
SHA-256 `88fcca809d62d6987ea8aa6e6accf0e55e83cc0a5db9c57edcb5773c1f89b60b`.

## Shutdown and postflight

Computer Use quit the native app. One exact interrupt stopped the daemon with
exit `0`. Its only stdout record is one no-write Runtime observation cycle;
stderr is empty.

```text
final SQLite   3af352562fbc85cb47c9509096655f2cda4a05ddbb999702bc764ab90f77ed56
daemon stdout  16e83254742da5fe0bc665dfb61f6eb462f793621de2bb6128293535c0e5fb4d
daemon stderr  e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

The product socket and IPC lock are absent, isolation is empty, no Pi-006
daemon/native/Pi/llama process or state lock holder remains, and the unrelated
`demo-resident` observer was not signalled or modified. Bounded DB/evidence
scans found no API-key, bearer-token or credential pattern.

**FINAL LIVE VERDICT**:
`PASS / SOURCE + VERIFIER + BOUNDED RECOVERY + AUTHORITY ACCEPTANCE + EXACTLY ONE TEAM TERMINAL / NO CONTROLLER RETRY`
