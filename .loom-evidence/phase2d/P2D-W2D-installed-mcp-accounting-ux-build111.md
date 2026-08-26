# Phase 2D build 111 — installed MCP, accounting and workflow UX

Status: `CURRENT / VERIFIED SLICE`

Date: 2026-08-22

## Installed boundary

- App: `/Users/lune/Applications/Loom.app`
- Version: `0.5.3 (111)`
- Bundle and nested daemon pass strict ad-hoc signature verification.
- The App owns the canonical daemon child. The daemon restores the typed MCP
  stdio helper from the owner-only Loom isolation root.
- Setup remains 25 Providers, 7 online Runtimes and 4 Conversation Profiles;
  Vault status is `unlocked`.

## Real execution

- A post-install OpenCode conversation returned exact `E2E-OK`.
- The installed MCP Mission performed the exact bound stdio ToolCall and
  committed matching content-free proposed/completed Journal facts.
- Mid-flight Enrollment revoke cancelled only the bound OpenCode Agent and
  produced no result commit; an unbound Loom Native peer completed normally.
- Post-revoke preflight blocked only the bound Agent with the exact Enrollment
  reason. No Team-global offline state was published.

## Governance and UX

- Provider Account rows show attempts, exact errors/rate limits, accounting
  coverage, separate input/output/cache/total Tokens, cost source, policy
  revision, concurrency, dispatch and budget ceilings, and incomplete state.
- Mission remains a title list. Opening a title shows one workflow plus a
  separate Team/Plan/Changes/Evidence inspector.
- Mission navigation exposes distinct accessibility labels for every entry.
- Team configuration history is deduplicated by newest same-source visible
  name; authoritative historical executions remain available in Mission
  history.
- Runtime & Providers visibly contains Claude Code, Codex, three Loom Native
  routes, OpenCode and Pi plus the full Provider directory.

## Verification

- `go test ./... -count=1`: pass after the MCP/runtime changes.
- `go vet ./...`: pass.
- macOS XCTest: 309 pass, one intentional visual-export skip.
- Swift strict wire contracts: 16 pass.
- build/install fixture and live acceptance script: pass.
- build 111 strict signature, dry-run install, cold launch and process identity:
  pass.
- focused account-row presentation tests: pass.
- `git diff --check`: pass.

## Remaining Phase 2D boundary

The installed directory has verified DeepSeek and MiniMax broker accounts and
OpenCode native execution, but no verified OpenAI, Anthropic or Kimi Provider
Accounts and no safe import candidates. A four-Harness/four-Provider Team,
Provider-injected installed auth/rate-limit/timeout cells, approved fallback,
and the corresponding four-Provider visual cannot be claimed from this state.
No credential, Prompt, Provider response or MCP argument/result body is present
in this evidence.
