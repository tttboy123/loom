# G4 — Single-Agent failure isolation matrix (installed-live)

Status: `PASS` (installed-live, real paid calls)

Date: 2026-08-16

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

`TestPhase2DPerAgentFailureIsolationMatrix` (V34) drives
`provider_rate_limited` / `provider_timeout` / `provider_insufficient_balance`
/ `credential_unavailable` cells at source level; this live gate drives the
`credential_unavailable` cell installed-live and confirms peers proceed.

## Safety

No credential, Prompt, Provider body, or user workspace entered source, logs,
or evidence. Evidence files are owner-only (`0600`).
