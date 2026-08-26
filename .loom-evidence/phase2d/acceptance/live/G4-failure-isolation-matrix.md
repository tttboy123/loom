# G4 — Single-Agent failure isolation matrix (installed-live)

Status: `PASS / INSTALLED CONTROLLED SIX-CELL + LIVE CREDENTIAL/ENROLLMENT
ISOLATION / ADDITIONAL REAL-ACCOUNT MATRIX DEFERRED`

Date: 2026-08-23

The completed MiniMax credential and Enrollment isolation evidence below stays
accepted. Additional real Provider Account revoke or rate-limit execution moved
to a later Phase on 2026-08-24; no future Phase is active yet.

## What was executed

1. Installed the App, daemon online. DeepSeek + MiniMax broker accounts
   verified; Provider Account Policies + Model Rate Cards configured.
2. Built + confirmed the 4-Agent mixed-provider Team (DeepSeek main + MiniMax
   bounded worker + DeepSeek reviewer + MiniMax researcher).
3. Revoked the MiniMax broker credential.
4. `preflight` → the two MiniMax-bound Agents blocked with the exact reason
   (`Credential is not verified. Reconnect this Provider Account.`) while the
   two DeepSeek-bound peers stayed `ready`.
5. `mission_execution start` dispatched the healthy DeepSeek peers (real paid
   calls); the MiniMax nodes stayed blocked.
6. `timeline_page` board → DeepSeek account carried attempts (peers proceeded);
   MiniMax account carried blocked attempts with no accounting (isolated).

## Result

```
node main provider=deepseek status=ready
node mission-role-... provider=deepseek status=ready
node mission-role-... provider=minimax status=blocked
  block="Credential is not verified. Reconnect this Provider Account."
node mission-role-... provider=minimax status=blocked (same reason)
isolation row deepseek/deepseek.primary attempts=1
--- PASS: TestLiveSingleAgentFailureIsolationE2E
```

The affected account's Agents fail closed with the exact credential reason;
healthy peers dispatch and no global offline state appears; the accounting
board isolates the failure to the MiniMax account.

## Source proof (matrix cells)

`TestPhase2DPerAgentFailureIsolationMatrix` now drives `provider_auth`,
`provider_rate_limit`, `timeout`, `provider_insufficient_balance`, and
`credential_unavailable` at source level. Every cell asserts the exact frozen
Provider Account on the affected Agent, the exact classified reason, and
successful peers. The installed credential revoke gate drives
`credential_unavailable`; the installed MCP gate separately proves an
in-flight Enrollment revoke cancels only the bound OpenCode Agent and does not
leak a result while the Loom Native peer succeeds.

Build 112 extends that live slice through recovery: the revoked Agent's failed
ToolExecution, Step and Turn all terminalize; fallback approval version 1 is
consumed and Attempt 2 is scheduled without changing the healthy Agent. The
fallback Provider returned `provider_http`, so successful fallback isolation is
not inferred from dispatch alone.

Build 113 closes the post-backend commit race in source: exact Enrollment
authority is checked before encrypted write and again before acceptance. A
revoke between those checks deletes the exact payload and commits no accepted
or completed result fact. Focused deterministic and race tests pass. The
installed MCP run again reached the private helper, revoked only the bound
Attempt, consumed approval and scheduled Attempt 2; MiniMax remained isolated
at `provider_http` while no unapproved route was selected.

Build 117 closes terminal approved fallback in the installed App. The gate
waited until the exact OpenCode MCP helper call was active, revoked that
Enrollment revision, and observed only the bound Attempt 1 fail. Explicit
approval version 1 selected a new Loom Native + DeepSeek Attempt 2, which passed
independent verification and recorded 2,792 tokens. The healthy Loom Native
peer succeeded independently with 3,359 tokens and the Team terminalized
`succeeded`. The source OpenCode WorkItem retained exactly one scoped permission
binding; no unbound capability or silent route change was introduced.

Build 121 closes the remaining controlled matrix through the installed App's
real private UDS. A credential-free, loopback-only Failure Lab transport ran
each target against a separate healthy peer account:

| Scenario | Target stage/code | Retryable | Peer |
| --- | --- | --- | --- |
| Auth | `provider_auth/provider_auth` | no | succeeded |
| Rate limit | `provider_rate_limit/provider_rate_limit` | yes | succeeded |
| Timeout | `provider_connect/timeout` | yes | succeeded |
| Insufficient balance | `provider_http/provider_insufficient_balance` | no | succeeded |
| Corrupt Vault record | `vault_aad_validation/corrupt_vault_record` | no | succeeded |
| Credential revision conflict | `agent_attempt_dispatch/credential_revision_conflict` | no | succeeded |

The installed Failure Lab UI projected the affected account, safe stage/code,
retryability, recovery action and Incident ID for every cell; it never marked
the healthy peer or whole Team unavailable. This is a controlled local failure
proof, not a mutation of a real Provider account and not evidence for the still
open four-distinct-Provider Team gate.

## Safety

No credential, Prompt, Provider body, or user workspace entered source, logs,
or evidence. Evidence files are owner-only (`0600`).
