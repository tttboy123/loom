# P2D-W2D Account-local Provider Timeout Classification

**Date**: 2026-08-12  
**Status**: source verified; installed live acceptance open  
**Parent Goal**: Phase 2D, the sole active product Goal

## Accepted boundary

Agent Provider timeouts are projected on the exact frozen Attempt instead of
becoming a Team-level `Unavailable` condition:

- request transport timeout: `timeout / provider_connect / retryable`;
- bounded response read timeout: `timeout / provider_http / retryable`;
- outer Attempt cancellation/deadline: existing runtime timeout authority.

Native DeepSeek/Kimi/MiniMax-compatible adapters and the Claude/Codex bounded
gateway use the same safe code with the stage above. Operational diagnostics
and Team Board bind Incident ID, Provider, non-secret Provider Account, and
Model. Cross-account, unknown-stage, or malformed records fail closed. Healthy
peer Agents receive no diagnostic.

The diagnostic excludes credentials, credential reference, endpoint
fingerprint, Authorization, Prompt, Capsule/transcript content, Provider body,
and hidden reasoning. It cannot authorize retry or fallback.

## Vault alignment

The accepted Loom Credential Vault architecture is unchanged. Normal
Conversation and Agent dispatch use exact account/reference/revision leases
from the in-process Vault. Keychain is only an optional explicit one-time
migration source and no plaintext fallback exists. CV6 installed multi-turn,
restart, revoke/rotation isolation, no-helper counter, and mixed-Team gates
remain open.

## Verification

- focused native and Harness timeout RED/GREEN: pass;
- affected native, Harness, API, and daemon packages: pass;
- affected race packages: pass;
- `go test -p 1 ./... -count=1 -timeout=15m`: pass;
- `go vet ./...`: pass;
- `git diff --check`: pass;
- `swift test --package-path apps/macos`: 194 XCTest, one intentional visual
  preview/export skip, zero failures; eight Swift Testing contracts, zero
  failures.

No App was installed or launched and no credential or Provider call was used.
Installed Loom remains v0.5.2 build 39. Build 54 is frozen as a separate
unlaunched, uninstalled static Candidate at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build54-candidate-2026-08-12/BUILD-MANIFEST.md`.
Its release, arm64, deep-signature, owner-only/no-symlink, timeout-contract,
and ZIP byte/mode-equivalence gates pass. This cannot satisfy installed CV6
acceptance.
