# P2D-W2D Provider Account Governance Visibility

**Date**: 2026-08-12  
**Status**: source verified; installed live accounting open  
**Parent Goal**: Phase 2D, the sole active product Goal

## User-facing boundary

Team Pulse now gives each exact Provider Account a distinct governance row:

- Provider and non-secret Provider Account identity;
- failed and total Attempts plus exact error rate;
- rate-limited Attempt count;
- accounting-covered versus total Attempts;
- total tokens and currency-specific cost;
- account policy revision, active concurrency, dispatch window, and assigned
  budget ceiling;
- explicit accounting-incomplete state.

Costs are rendered from integer microunits as exact decimal currency values;
error rate is rendered from integer basis points. Agent rows and account rows
use separate system icons and accessibility labels. Text wraps vertically in
the existing unframed Inspector list.

## Authority and privacy

The App consumes the Board aggregation produced from each Attempt's frozen
Provider ID plus Provider Account ID. It does not merge same-Provider accounts
or resolve credentials by Provider ID alone. The display is observational and
cannot approve retry or fallback.

Credential reference, endpoint fingerprint, secret bytes, Authorization,
Prompt, Capsule/transcript content, Provider body, and hidden reasoning remain
excluded.

## Verification

- RED product fixture failed for missing Provider name, error rate, rate-limit
  count, accounting coverage, and readable cost;
- GREEN Team Pulse and deterministic formatting tests: pass;
- complete `swift test --package-path apps/macos`: 195 XCTest, one intentional
  visual preview/export skip, zero failures; eight Swift Testing contracts,
  zero failures;
- `go test ./internal/api ./internal/projection ./internal/app -count=1`: pass;
- `git diff --check`: pass.

No App was installed or launched and no Provider or credential was used. Real
installed cost observation, failure isolation, fallback, mixed-Team execution,
and Credential Vault CV6 remain open. Build 55 is frozen as an unlaunched,
uninstalled Candidate at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build55-candidate-2026-08-12/BUILD-MANIFEST.md`;
its static package gates cannot replace installed acceptance.
