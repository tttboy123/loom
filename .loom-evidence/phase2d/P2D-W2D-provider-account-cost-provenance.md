# P2D-W2D Provider Account cost provenance

**Status**: `CURRENT / SOURCE CANDIDATE`  
**Date**: 2026-08-12  
**Goal**: Phase 2D only

## User outcome

Team Pulse no longer presents every observed cost as an unexplained number.
Each Agent Attempt and exact Provider Account row identifies whether the cost
was reported by the Provider or by its Harness. Values with the same currency
but different sources remain separate.

## Authority boundary

- New Run terminal cost facts accept only `provider_reported` or
  `harness_reported`.
- Historical events with no `cost_source` field replay as
  `legacy_unspecified`; an explicitly empty or legacy source cannot be newly
  committed.
- Projection distinguishes a missing JSON field from explicit empty or JSON
  `null` values.
- `rate_card_estimate` is accepted only when the exact Attempt freezes an
  immutable Provider Account plus Model Rate Card revision, digest, currency,
  token basis, rates, and rounding rule. Missing, stale, or cross-account cards
  fail closed.
- No Provider price is hardcoded and no estimate is presented as observed.

## Current producers

- Claude Code process protocol: `harness_reported`.
- Pi RPC usage protocol: `harness_reported`.
- Loom Native OpenAI-compatible DeepSeek/Kimi/MiniMax: token usage observed,
  Provider cost remains unobserved; an explicitly configured frozen Rate Card
  may produce a separately labeled `rate_card_estimate`.

## Verification

- Focused authority, Projection, API, Harness, Pi, App, Supervisor, and Swift
  RED/GREEN tests pass.
- Full `go test ./... -count=1` passes, including real Swift-Go IPC contract
  probes.
- Affected race packages pass.
- `go vet ./...` and `git diff --check` pass.
- Complete Swift passes 198 XCTest cases with one intentional visual
  preview/export skip and nine Swift Testing contracts, all with zero
  failures.

## Open live gates

This is not installed real-cost evidence. Rate Card authority and source UI are
implemented in the source Candidate, but installed Provider Account cost,
account-local revocation/auth/rate-limit/timeout, approved fallback, mixed-Team
execution, and Credential Vault CV6 remain open.

## Rate Card authority advancement

The append-only Rate Card authority is keyed by exact Provider, Provider
Account, and Model. Each immutable revision freezes a digest, currency, token
basis, integer microunit rates, and the `ceiling_per_attempt` rounding rule.
Provider Account setup and Swift UI configure and project these non-secret
facts without shipping official prices or silently selecting a card.

Run Claim contract v2 freezes the complete Rate Card with the execution
binding. Terminal accounting may estimate only from that frozen value and
observed token usage. Provider- or Harness-reported cost takes precedence and
is never overwritten by an estimate. The Board and Team Pulse expose Rate Card
revision, currency, basis, and source while excluding credentials and secrets.

A replay repair now discovers the exact frozen execution binding before
loading its account policy, capacity, and Rate Card streams, then performs one
strict Run replay. This prevents a valid v2 Claim from failing during a
selective pre-read while retaining final CAS conflict detection.

The combined source is packaged as unlaunched, uninstalled v0.5.2 build 58 at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build58-candidate-2026-08-12/BUILD-MANIFEST.md`.
Installed Loom remains build 39; installed real cost and CV6 gates remain open.
