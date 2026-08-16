# Phase 2C Repair Journey Attempt 006 Failure

**Date**: 2026-08-08  
**Journey ID**: `5ff8e30b-82e8-460b-a96d-04e229a88bda`  
**Primary fixture root**: `/private/tmp/loom-phase2c-chat-20260808-006`  
**Decision fixture root**: `/private/tmp/loom-phase2c-decision-20260808-006`  
**Source-lock SHA-256**: `641437c30000a71ad5c9d73eeae08aa1f2ed5b3da7c4e7d6058513aef1eb2322`  
**Ordered source digest**: `e64e0ba41e84b04d0d0255497d94521383aaeebc8327845896d388435e49b745`  
**Outcome**: `FAIL - TUI_DECISION_OPERATION_REJECTED`

## Failure

Repair 8 passed its complete lock-bound matrix. Attempt 006 then proved the
chat-first, explicit Team, explicit Mission, governed terminalization, and
restart-projection paths. The ordinary production backend truthfully produced
no prepared authoritative decision, so J6 used the repository's built-in
controlled Mission fixture in an isolated private root. The real PTY TUI read
the prepared Authorization sheet and selected its prepared `allow_once`
action. It then sent operation `decide`. The closed daemon contract accepts
authoritative prepared actions only with operation `submit`, rejected the
request as `invalid_request`, and wrote no approval decision fact.

The defect is isolated to `internal/tui/model.go`. Native already submits
`submit`; the TUI test incorrectly expected `decide` and therefore encoded the
wire drift instead of detecting it. Repair 9 must update the causal test first,
then send the service's exact operation without changing authority validation.

## Completed Journey Evidence

- J1 passed in Native and a real PTY. Both opened directly into ordinary
  conversation without requiring a project or Team. Tentative output remained
  a non-authoritative Loom proposal. The Journal contained only three
  initialization/runtime facts.
- J2 passed in the controlled TUI path. Only the safe folder display name was
  shown, tool-shaped model text was replaced by the fixed non-actionable
  warning, and the private sentinel was absent from client, IPC, daemon, and
  state artifacts. The native system picker was not rerun because Computer-Use
  automation remained skipped, so strict dual-client J2 is unverified.
- J3 passed after restoring the contract's ten-second discovery interval.
  Native and TUI showed local-service unavailability, then recovered against
  the same restarted daemon with exactly the same three initialization/runtime
  facts and no duplicate identities.
- J4 passed in the real PTY. The explicit Agent Team builder reached a
  confirmable Coordinator plus Bounded Worker draft while the Journal remained
  at three facts. Native builder interaction was not rerun.
- J5 passed. Explicit Team confirmation added exactly `TeamDefinitionSaved`,
  `TeamInstanceCreated`, and `AgentInstanceCreated`; read-only Mission preflight
  added no fact. Explicit Start projected the full start lineage before the
  response. Main and verifier attempts terminalized with Evidence and governed
  recovery, producing one Team terminal state and a truthful blocked Mission.
- J6 failed at the exact authoritative submission boundary described above.
- J8 passed. An ordinary daemon restart rebuilt the identical view version
  `357f664dda7b97c85e37d064058c97920c0dff484ff4ad25e65c8c4805350da0`
  with one Team, one blocked Mission, four Runs, four Evidence records, and
  three attention items entirely from the Journal.
- J7 was partially observed. The TUI showed the actual Pi runtime online. The
  strict runtime-offline scenario was not induced. J9-J10 and full dual-client
  accessibility/visual review were not promoted after J6 failed.

## Additional Repair 9 Findings

1. A Team Draft is daemon-memory state. After daemon restart, confirming the
   locally rendered old draft returns typed `not_found`. The TUI currently
   shows generic `State unavailable` and retains unusable state until manual
   refresh. Native and TUI must recover from `not_found` exactly as from a stale
   `conflict`: one submit, discard, one setup refresh, actionable explanation,
   and no automatic retry.
2. After the TUI receives an authoritative `running` Mission start result, it
   keeps the consumed preflight while refreshing the snapshot. A changed view
   version then clears the accepted result and reports `preflight_expired`.
   Successful start must consume the preflight before refresh; genuine
   pre-start expiration must remain unchanged.
3. The initial 500-millisecond discovery interval was a harness
   misconfiguration that caused observer pressure and sticky health warnings.
   Restarting with the historical ten-second interval stabilized the same
   fixture. This diagnostic does not authorize product source changes.

## Authority and Shutdown Audit

- Primary Journal: `285` events, `285` unique event IDs, `285` unique
  idempotency keys, SQLite integrity `ok`.
- Controlled-decision Journal: `73` events, `73` unique event IDs, `73` unique
  idempotency keys, SQLite integrity `ok`.
- Controlled J6 wrote two `ApprovalRequested` and two
  `WorkItemApprovalPaused` facts from its seeded fixture and zero approval
  decision facts after the rejected TUI request.
- Network and credential access were both false. The private J2 sentinel
  appeared only in its source fixture.
- The Repair 8 ordered source digest revalidated exactly before shutdown.
- Both TUI sessions, both daemons, and Native stopped. Attempt 006 sockets and
  ownership locks were absent after shutdown. A separate user-owned resident
  daemon was not touched.

## Artifact Bindings

| Artifact | SHA-256 |
|---|---|
| Primary `tui/transcript.txt` | `511e9626782482daaf539c3cb48851c33faef97f5470dd3f14baf93fcee0adca` |
| Primary `ipc/request-response-summary.jsonl` | `98e190615a19aaed2cd9232e7b93ae302d6f096df934bc8d117915a3fa192e94` |
| Primary `daemon/structured-log.jsonl` | `e602b574cb81b58f28f0fd4f343dfa9af9be5e1ba21c6dc0311eef2b78cd1bb1` |
| Primary `state/loom.db` | `5b4cbfdbab360e08ef423fc987fa3b96a948b17809bb01b253d52d132758f045` |
| Primary `state/chat-threads.json` | `bff6e9586365ba90c91080643aff439f68949cbf17875441812cceed340fb5ea` |
| Decision `tui/transcript.txt` | `98de76229a3503eeaf2b99aa0000e9468c6e4e5122474d3b833e0408d91ce12e` |
| Decision `ipc/request-response-summary.jsonl` | `cf50732a8547e0b92c7c04f85c380e7f6b1d552c3276cb0b605ce4ca14a2aa92` |
| Decision `daemon/structured-log.jsonl` | `fa20c356bb33053ef669d131b75c1489009b650a808d84870b304356e5b1c569` |
| Decision `state/loom.db` | `121c0628d4bfbcd5ebdd18626c628532495ab4a45ae44b7092c7e34c24520500` |
| `source/source-lock.json` | `641437c30000a71ad5c9d73eeae08aa1f2ed5b3da7c4e7d6058513aef1eb2322` |
| `bin/loom` | `1448f77183d6b6da38c23a84b2a4ba951d0936ec98772fc5efe51ee3870aecbc` |
| `bin/loomd` | `85456ca1962cfdee8203445fc8fd66fc54c73bfce501b79f3aa51625464632be` |
| Native executable | `fd8603d6bb7ecd99aa32ba972d54b022fa0b7143f3091f51ee509833ab2534e4` |
| Native J1 chat-first screenshot | `4d4eaf966a616767b1a150d548e2a0b767406d7dc26ac165ace7a53dfdb94ceb` |
| Native J3 offline screenshot | `ec665c342c775042fe336822504b9b7218eed6dd2aa44b6279a4d77c4246350f` |
| Native J3 intermediate partial screenshot | `80267853d651bc8b99e041e9c37d91af877bc69225810028c02044440c81aace` |
| Native J3 recovered screenshot | `8537e51d1c1997c2a0ff8c1d6718541aa25899a63b54b9bbc8417d7150413e63` |
| Native J4 draft screenshot | `17556523d43e1fffceb6a0f4d93217de20ead4521bd868b41b7e1824dee367c3` |

No screenshot or partial result from this consumed attempt is promoted.
Attempts 001-006 remain immutable historical records.
