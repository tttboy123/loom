# P2D-W1/W2D CC Switch Import and Runtime Recovery — Build 92

Status: `INSTALLED VERIFIED SLICE / PHASE 2D PARTIAL`

Date: 2026-08-22

Installed product: `/Users/lune/Applications/Loom.app` v0.5.3 build 92

## Boundary

This slice keeps Harness Runtime discovery independent from Provider Account
readiness and adds a privacy-bounded, explicit one-time CC Switch import path.
CC Switch discovery projects non-secret candidates into Runtime & Providers;
the private UDS import command carries only candidate, Provider, and account
identity. The API key is leased inside the daemon import boundary, committed to
the Loom-owned Credential Vault, verified, and zeroized. No normal conversation
or Agent Attempt reads CC Switch or macOS Keychain.

Exact official endpoint matches can be imported after user confirmation.
Compatible custom endpoints remain `custom_endpoint_review`; Loom does not
silently label them as official Kimi, MiniMax, or other Provider accounts.

## Installed evidence

- The App cold-started its bundled daemon without a manual service command.
- The canonical daemon argv includes the discovered Codex, OpenCode, and Claude
  executables plus the non-secret read-only CC Switch database path.
- Installed `setup_snapshot` returned 25 Providers, 6 online Harness Runtimes,
  4 Conversation Profiles, and 5 non-secret import candidates.
- DeepSeek and MiniMax remained verified. Codex, Claude Code, Loom Native,
  OpenCode, and Pi Runtime rows remained visible after restart.
- A real installed DeepSeek conversation returned the bounded acceptance reply.
- CC Switch storage is owner-only: directory `0700`, database `0600`.
- App SHA-256:
  `939a5b2bd9fb47d1c9c3ac68f5d2e3276d95c6c5c695b2a31325ea524704d5ae`
- Bundled daemon SHA-256:
  `1ae5f6075dc0bbeedeb29121d4cf7c902819ed4846a3c2777be5974dc10d1863`
- Strict deep bundle signature verification passed.

The graphical login session remained locked and produced a black screenshot,
so this slice does not claim unlocked installed visual acceptance.

## Source verification

- `go test ./... -p 1 -count=1` — pass
- `go vet ./...` — pass
- macOS `swift test` — 293 tests, 1 intentional skip, 0 failures; 16 strict
  Swift Testing contracts passed
- two-build native packaging fixture — pass
- private UDS import regression — pass, including verified revision projection,
  secret zeroization, and rejection of secret-bearing IPC fields
- `git diff --check` — pass before evidence update

## Remaining Phase 2D gates

- custom endpoint metadata, egress policy, and explicit approval
- unlocked installed Runtime & Providers visual walk-through
- four-Harness/four-Provider installed Team
- complete account-local auth/rate-limit/timeout/corruption matrix
- explicit fallback, complete accounting UI, and remaining Capsule/privacy gates
