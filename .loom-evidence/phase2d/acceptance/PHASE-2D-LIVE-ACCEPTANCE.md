# Phase 2D Installed-Live Acceptance Matrix and Runbook

Status: `READY TO EXECUTE / NOT YET RUN`

Date: 2026-08-16

Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

This runbook is the executable path for every remaining Phase 2D completion
gate. The source chain behind each gate is source-verified (V30-V32 and
predecessors); what is left is installed-live execution with a real App bundle,
real Provider credentials and an approved operator.

## Operator prerequisites

1. **Build and install the App** (unsigned, ad-hoc signed):
   - `scripts/build-loom-local-app.sh --output /absolute/path/Loom.app`
   - `scripts/install-loom-local-app.sh --app /absolute/path/Loom.app --destination /Applications/Loom.app`
   - Gate: `scripts/test-build-loom-local-app.sh` and
     `scripts/test-install-loom-local-app.sh` PASS before installing.
2. **Start the App** and unlock the Loom-owned Credential Vault (explicit
   Unlock; no Keychain fallback in normal operation).
3. **Real Provider credentials**, one per account:
   - OpenAI/Codex native runtime, Anthropic (brokered), DeepSeek (brokered),
     Kimi (brokered), MiniMax (brokered). Configure and **Verify** each in the
     Provider Account surface; the exact credential revision is frozen per
     Agent and Conversation Profile.
4. **Runtime harnesses** available to the App: Codex CLI, Claude Code CLI, and
   the Loom Pi runtime (or their configured discovery paths).
5. **Explicit operator approval** for: installing the App, network calls to
   the selected Providers, Web Search and MCP Enrollments, and live agent
   execution. Nothing in this runbook runs without that approval; no credential
   ever enters source, scripts, logs or evidence.

## Gates and pass criteria

### G1 — Installed credential import (CV6)

- Configure an API key in the Provider Account sheet for each provider;
  verify; restart the App; Unlock the vault; confirm the account remains
  configured with the same credential reference and revision.
- Revoke one account; restart; confirm revocation persists and no helper/
  Keychain path is used in normal operation.
- Evidence: `.loom-evidence/phase2d/acceptance/G1-installed-vault.md`.

### G2 — Real conversation

- Open the App, pick a Conversation Profile bound to a verified account
  (e.g., DeepSeek `conversation-deepseek-deepseek-chat-account-<id>-r<rev>`),
  send a user message, receive a bounded model reply in the same thread.
- The thread shows one Attempt with the frozen Execution Binding, an Incident
  correlation, and content-free diagnostics; no Prompt, key or Provider body
  in diagnostics or the Journal.
- Evidence: `.loom-evidence/phase2d/acceptance/G2-real-conversation.md`.

### G3 — Mixed Provider Team (ATL9)

- Build a 4-Agent Team where each Agent binds an independent Provider Account
  and model (OpenAI/Codex, Anthropic/Claude, DeepSeek, Kimi or MiniMax) with
  exact credential revisions and reasoning effort/limits.
- Run the Team to completion; every Agent Attempt uses its own Vault lease and
  frozen binding; the Team board shows per-Agent and per-Account rows.
- Evidence: `.loom-evidence/phase2d/acceptance/G3-mixed-team-atl9.md`.

### G4 — Single-Agent failure isolation matrix

For each cell, run the mixed Team and inject the failure on **one** sub-agent's
account while peers stay healthy:

| Failure class | Expected | Source proof |
|---|---|---|
| Credential revoked | only that Agent blocks; peers succeed; Team never globally offline | `TestFourProviderTeamRevokedCredentialIsolatesOneAgent` |
| Corrupt vault record | only that Agent blocks; peers succeed | `TestFourProviderTeamCorruptVaultRecordIsolatesOneAgent` |
| Credential revision conflict | only that Agent blocks; peers succeed | `TestPhase2DTeamExecutionUsesOnlyExactApprovedFallbackBinding` family |
| Provider rate limit | only that Agent gets the retryable rate-limit diagnostic; peers unaffected | `P2D-W2D-account-local-provider-timeout-classification` focused tests + `TestPhase2DPerAgentFailureIsolationMatrix` |
| Provider timeout | only that Agent gets `timeout/provider_http/retryable`; peers unaffected | same |
| Enrollment revoked / policy drift | only the bound Agent's preflight blocks | V32 preflight tests |

- Source proof: `TestPhase2DPerAgentFailureIsolationMatrix` (V34) drives
  `provider_rate_limited` / `provider_timeout` / `provider_insufficient_balance`
  / `credential_unavailable` cells; affected Agent `blocked`/`failed` with the
  exact reason, all peers `succeeded`.
- Live pass: the affected Agent row shows a closed diagnostic, retry review
  and Incident copy; healthy peer and accounting rows inherit no actions.
- Evidence: `.loom-evidence/phase2d/acceptance/G4-failure-isolation-matrix.md`.

### G5 — Installed Web/MCP diagnostics

- Configure a Web Search Enrollment and an MCP Enrollment under a verified
  account; bind one to an Agent; run an Attempt that uses the tool; verify the
  bounded result commit, tool diagnostics and Incident correlation.
- Source proof (V35): the production daemon composition materializes persisted
  active + policy-current Enrollments through the trusted Work Bundle boundary
  (`newProductRemoteToolExecutorsFromEnrollments` + composite executor);
  revoked/drift/unsupported/port-less records fail closed per-Enrollment and
  never expose a capability.
- Revoke the Enrollment mid-flight; confirm only the bound Agent is affected
  and the default production exposes no unbound remote capability.
- Evidence: `.loom-evidence/phase2d/acceptance/G5-web-mcp-diagnostics.md`.

### G6 — Accounting and governance UI

- After G3/G4, confirm each Provider Account row shows attempts, exact error
  rate, rate-limited count, accounting coverage, tokens, currency-specific
  cost (`provider_reported` vs `rate_card_estimate`), policy revision,
  concurrency and budget ceiling, plus the explicit accounting-incomplete
  state where applicable.
- Evidence: `.loom-evidence/phase2d/acceptance/G6-accounting-ui.md`.

## Evidence rules

- All evidence files are owner-only (`0600`) and privacy-safe: versions and
  hashes, process/socket health, non-secret Provider/Profile/Agent binding
  state, and allowlisted diagnostics only.
- Never include API keys, Authorization headers, credential references,
  endpoint fingerprints, Prompts, conversation content, Provider bodies, MCP
  arguments/result bodies or raw console.
- Each gate records Incident IDs; the same ID may correlate diagnostics with
  authoritative Journal facts but never authorizes execution.

## Harness

`scripts/phase2d-live-acceptance.sh` is a source library that:
- checks the installed App/daemon prerequisites without touching credentials;
- exports a bounded privacy-safe evidence bundle for each gate;
- prints a structured PASS/OPEN summary.
`scripts/test-phase2d-live-acceptance.sh` validates the library in source-only
mode (no side effects).

## Definition of done

All six gates PASS with evidence files, the full source verification suite is
green, and `docs/CURRENT.md` records Phase 2D as `COMPLETE / INSTALLED LIVE`.
Default production remote-tool capability remains unpublished unless G5 is
explicitly approved and executed.
