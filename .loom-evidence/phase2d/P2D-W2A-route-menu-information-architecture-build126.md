# P2D-W2A Route Menu Information Architecture — Build 126

Date: 2026-08-24

Status: `SOURCE + INSTALLED VERIFIED / PHASE 2D PARTIAL`

## Problem

Build 125 separated Provider, Runtime and Route in the authoritative data model,
but the flat Conversation Route menu still repeated Harness, Provider and raw
account identifiers on every row. The result looked like six duplicate Provider
choices and exposed implementation labels such as `deepseek.primary`.

## Implemented boundary

- The picker groups Route options under `Loom Native`, `OpenCode` and `Codex`
  Harness headings.
- Route rows contain Provider identity and only a human-readable non-default
  account name. Primary account IDs are omitted.
- The OpenCode route that does not bind an external Provider is named `Built-in`.
- The collapsed control remains a compact Harness + Route label. Model remains a
  separate control and the immutable execution binding is unchanged.
- Stable grouping and label helpers have focused regression coverage.

## Verification

- Focused Route-label and Harness-group tests passed.
- `swift test --package-path apps/macos` passed 331 tests with 2 skipped and no
  failures.
- Build 126 (`0.5.3`) was ad-hoc signed, installed at
  `/Users/lune/Applications/Loom.app`, launched and reached local-service ready.
- Installed accessibility inspection returned the exact menu hierarchy:
  `Loom Native` with DeepSeek and MiniMax; `OpenCode` with DeepSeek, MiniMax and
  Built-in; `Codex` with OpenAI.
- The installed menu contained neither `deepseek.primary` nor
  `minimax.primary`.
- The installed daemon uses the same verified Build 125 binary, so this release
  changes presentation only and does not alter Provider dispatch or credentials.

## Installed identity

- App build: `0.5.3 (126)`
- App executable SHA-256:
  `8071e8dc15d884e41bd04b2d7aca9e8d4bb12de36176a9b2504e5b0ab9da6f0c`
- Bundled daemon SHA-256:
  `0d2c0fd42f5de5b474cdeddda3a74b57de165dac0d6e2837489de3a30d7a3bcc`

No credential, Prompt, Provider response body or external Provider request was
used during this verification.
