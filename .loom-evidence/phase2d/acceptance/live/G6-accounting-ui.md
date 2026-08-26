# G6 — Accounting and governance UI (installed-live)

Status: `SOURCE COMPLETE / PARTIAL INSTALLED LIVE MATRIX`

Date: 2026-08-23

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

Build 111 completes the account-row presentation contract: input, output,
non-zero cache read/write and total Tokens are distinct; zero cache fields are
omitted; usage coverage remains explicit. The same row renders exact error
rate and rate-limited count, accounting coverage, cost source, policy revision,
concurrency, dispatch and budget ceilings, and incomplete accounting. Focused
view tests and the full macOS suite pass.

Installed visual review confirms the Mission list opens one workflow, the Team
inspector remains separate, every Mission rail destination has a distinct
VoiceOver label, and the Team workspace shows only the latest same-source
visible configuration while preserving older executions in Mission history.
The four-Provider visual and an approved fallback accounting event remain open
because the installed directory currently has only DeepSeek and MiniMax
verified broker accounts plus OpenCode native execution.

Build 112 now projects explicit fallback consumption and Attempt 2 scheduling.
The fallback did not complete because the available Provider returned
`provider_http`, so a successful fallback cost/token row remains open rather
than being synthesized from the recovery event.

Build 113 repeated that installed lineage after the result-commit hardening:
Attempt 1 terminalized, approval version 1 was consumed and Attempt 2 was
account-local. MiniMax again ended at `provider_http`, so no successful token or
cost row is inferred. Native visual review of the new Context inspector could
not run while macOS was locked and remains explicitly open.

Build 115 completes a fresh installed 4-Agent / 2-Provider Team after the
COMP2-E route-module migration. All four nodes succeeded. DeepSeek carried two
accounted attempts, 8,208 total tokens, two usage rows and two cost rows;
MiniMax independently carried two accounted attempts, 5,480 total tokens, two
usage rows and two cost rows. Both account rows reported zero failed and
rate-limited attempts. A brokered OpenCode + DeepSeek Team and a native-auth
OpenCode Team also completed successfully. The four-distinct-Provider matrix
remains open.

Build 117 closes the native Mission/Team visual gate. A slash-qualified
`opencode/big-pickle` binding now passes the same strict model-ID contract as
setup and execution. Installed Mission Team Pulse renders both native OpenCode
Agents with status, Attempt, route, limits, Context and Incident. Teams renders
current configuration rows for OpenCode, DeepSeek and MiniMax without
collapsing historical execution into the configuration directory.

Build 117 also closes the approved fallback accounting event. During a real
installed MCP call, the OpenCode source Attempt failed after exact Enrollment
revocation. The approved Loom Native + DeepSeek Attempt 2 succeeded with 2,792
total tokens and a `rate_card_estimate` cost row; the unaffected Loom Native
peer independently succeeded with 3,359 tokens and its own accounting row. The
Team terminal state was `succeeded`. Only the four-distinct-Provider visual and
account matrix remain open.

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
