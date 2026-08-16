# G6 — Accounting and governance UI (installed-live)

Status: `PASS` (installed-live, real paid calls)

Date: 2026-08-16

## What was executed

1. Installed the App, daemon online. Credential Vault unlocked; DeepSeek +
   MiniMax broker accounts verified.
2. `provider_account_policy_configure` + `provider_model_rate_card_configure`
   for `deepseek.primary` (deepseek-chat) and `minimax.primary` (MiniMax-M3):
   exact concurrency/budget ceilings + disclosure + USD rate card
   (`rate_card_estimate`).
3. `LOOM_LIVE_TEAM_E2E=1 go test ./cmd/loomd/ -run
   TestLiveMixedProviderTeamE2E -count=1 -v`
   - Builds + confirms the 4-Agent mixed Provider Team, preflight `4/4 ready`,
     `mission_execution start` returns `running`.
   - `timeline_page` board `provider_accounts` rows asserted for every account:
     attempt count, failed/rate-limited counts, error-rate basis points,
     budget, policy revision + digest, concurrency/dispatch/budget ceilings,
     accounting coverage (`accounting_attempt_count <= attempt_count`),
     token usage (input/output/cache/total), per-attempt cost rows
     (`provider_reported` vs `rate_card_estimate`), and the explicit
     accounting-incomplete state where attempts failed before usage.

## Result

```
preflight nodes=4 ready=4
mission result: status=running
account deepseek/deepseek.primary attempts=1 accounting=0 policyRev=1 maxConcurrent=4
account minimax/minimax.primary attempts=2 accounting=1 usage=1 cost=1
  tokens=2898 budgetUnits=0 policyRev=1 maxConcurrent=4 overflow=false
--- PASS: TestLiveMixedProviderTeamE2E (11.21s)
```

The MiniMax account completed one real attempt with exact token usage
(input/output/cache) and a `rate_card_estimate` USD cost; the policy revision
and concurrency/budget ceilings are projected into the board; failed attempts
explicitly carry zero accounting coverage (accounting-incomplete state).

## Fixes that unblocked this gate (V34)

- `internal/localipc` `validMethod` was missing
  `provider_model_rate_card_configure` and the two remote-tool-enrollment
  methods, so the daemon rejected them with `unknown_method` before the
  handler ran. Added them (plus the Swift client allowlist).
- `internal/api` `maxTentativeDelta` was 2 KiB; the loom-native adapter
  publishes the full bounded model response as one `MessageEvent` delta, so
  real responses were rejected as `invalid node output` and attempts stayed
  `running` forever. Raised to 256 KiB.

## Safety

No credential, Prompt, Provider body, or user workspace entered source, logs,
or evidence. Evidence files are owner-only (`0600`).
