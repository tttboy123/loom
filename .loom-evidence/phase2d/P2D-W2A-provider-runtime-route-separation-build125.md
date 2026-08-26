# P2D-W2A Provider, Runtime and Route Separation — Build 125

Date: 2026-08-24

Status: `SOURCE + INSTALLED VERIFIED / PHASE 2D PARTIAL`

## Problem

OpenCode is a Harness Runtime, while DeepSeek and MiniMax are Model Providers.
The Setup backend projected OpenCode into the Provider directory and generated
account-scoped Profile display names such as `OpenCode · DeepSeek`. The
Conversation control was also labelled Provider, making executable Routes look
like duplicate Provider entities.

## Implemented boundary

- `provider.ModelCatalog` excludes `native_runtime` descriptors.
- Setup Provider Directory is built only from that Model Provider catalog.
- OpenCode Profile availability is resolved from the online Runtime inventory,
  not from a synthetic Provider row.
- Setup normalizes every Conversation Profile display name from `provider_id`
  immediately before projection. Constructor mistakes cannot publish composite
  Harness/Provider names.
- The macOS control is labelled Route and renders Harness, Provider and exact
  Provider Account in execution order. Model remains a separate control.
- The contract probe exports only non-secret Route identity fields for installed
  inspection.

## Verification

- `go test ./internal/provider ./internal/app ./internal/api ./cmd/loomd -count=1`
  passed on the final source; `cmd/loomd` completed in 239.268 seconds.
- `swift test --package-path apps/macos` passed 330 tests with 2 skipped and no
  failures.
- `scripts/test-build-loom-local-app.sh` passed the two-build reproducibility
  fixture.
- Build 125 (`0.5.3`) was ad-hoc signed, installed at
  `/Users/lune/Applications/Loom.app`, launched, and created its private UDS.
- Installed snapshot: 24 Providers, 7 Runtimes, 6 Conversation Routes, 5 safe
  import candidates. Provider IDs do not include `opencode`; Runtime IDs include
  `runtime.opencode.local`.
- Installed Route projection includes Loom Native + DeepSeek, Loom Native +
  MiniMax, OpenCode + DeepSeek, OpenCode + MiniMax, OpenCode native and
  Codex + OpenAI. Composite Profiles retain exact account/revision/model binding
  without becoming Provider rows.
- UI inspection showed OpenCode under Agent Runtimes. Searching Model Providers
  for `opencode` returned `No matching providers`.

## Installed identity

- App build: `0.5.3 (125)`
- App executable SHA-256:
  `a9074e749b7e43f141fa601bc42e4f217fcabc71fa0b85046e29313cbf5c2fcb`
- Bundled daemon SHA-256:
  `0d2c0fd42f5de5b474cdeddda3a74b57de165dac0d6e2837489de3a30d7a3bcc`

No API Key, Prompt, Provider response body, ciphertext, nonce or Authorization
header was read or written during this verification. No external Provider call
was made for Build 125.
