# P2D-COMP1 / COMP2-E - Journal rollback and positional route removal v1

**Date**: 2026-08-23
**Status**: SOURCE + INSTALLED VERIFIED / COMP2-E CLOSED
**Goal**: Phase 2D only

## Accepted boundary

- A real SQLite-backed Journal fact remains present while a later Bundle Ready
  hook fails, during reverse Effect rollback, and after rollback completes.
- `localProductHandlerWithComposition` and its fifteen positional service
  dependencies are removed from production and tests.
- Existing test callers now build the closed `productRouteServices` aggregate.
- `localProductHandlerWithDecision` normalizes nil concrete API pointers before
  assigning typed route interfaces, preserving unavailable-route behavior.
- Every declared route returns the same unavailable response through the typed
  handler and declarative route registry when services are absent.
- The obsolete `legacy.product.dispatch` symbol is removed.
- `productRouteServices` and `newProductRouteHandler` now belong to the bounded
  `product_route_handler.go` module; an AST ownership gate prevents either
  declaration from returning to `product_daemon.go`.

## Verification

- `TestCOMP1EffectRollbackDoesNotRevertCommittedJournalFacts`: pass.
- COMP2-E typed-handler parity and positional-handler AST gates: pass.
- Focused race tests for COMP1, COMP2-E, and Mission unavailable routing: pass.
- `go test ./cmd/loomd -count=1`: pass.
- `go test -p 4 ./... -count=1`: pass.
- `go vet ./...`: pass.
- `git diff --check`: pass.

## Installed Build 114

- Candidate and installed bundle: Loom `v0.5.3 (114)`.
- Native App SHA-256:
  `8f692bcf6f0102b0bd5355b2b8f12728bdb1d56048c0ac2089777ccc4c12e063`.
- Bundled daemon SHA-256:
  `5ec88525ce837863c543760101a68210e2e03a1af5955783c1ea45b9f6b37d16`.
- Reproducible build, installer rollback fixture, install dry-run, strict nested
  signing, arm64 identity, and transactional install: pass.
- Cold start and a second cold restart each produced a new App-owned daemon
  with canonical state, isolation, Runtime, Socket, and managed-parent argv.
- Before and after restart, setup returned 25 Providers, 7 Runtimes, 5 safe
  credential-import candidates, and 4 Conversation Profiles. DeepSeek and
  MiniMax remained verified; OpenCode remained available; all Runtimes online.
- A real installed OpenCode Conversation returned exact `E2E-OK` before and
  after the cold restart with the Loom Vault unlocked.
- macOS XCTest: 313 pass; strict Swift wire contracts: 16 pass.

## Installed Build 115 route-module parity

- Candidate and installed bundle: Loom `v0.5.3 (115)`.
- Native App SHA-256:
  `111fbf9bcf58d321cd8143b25f6f9baea127b1733435866ae0cfe67d6dba0468`.
- Bundled daemon SHA-256:
  `e16279ad410b4b12521f683f3537cef617aea776b027ff17b5acf20fabc4817a`.
- Reproducible build, installer rollback, strict signing, transaction install,
  cold start and cold restart: pass.
- Before and after restart, setup remained at 25 Providers, 7 Runtimes, 5 safe
  import candidates and 4 Conversation Profiles, with all Runtimes online.
- Real OpenCode Conversation returned exact `E2E-OK` before and after restart.

## Installed Build 117 closed production entry

- Production activation accepts one exact typed construction and no direct
  handler; the direct-handler facade remains test-only as a parity oracle.
- Candidate and installed bundle: Loom `v0.5.3 (117)`.
- Native App SHA-256:
  `4f6f80f6588d235c0a074072f3651415cd2409be022c94a727c6fc62a323a64e`.
- Bundled daemon SHA-256:
  `db0c1228e9df2485196f980a34c07773160985794559058abc4e3317e7519200`.
- Full Go, focused race, vet, Swift, strict-wire, reproducible-build,
  installer-rollback, signing, transaction-install and cold-restart gates pass.
- Before and after restart, setup remained at 25 Providers, 7 Runtimes and
  4 Conversation Profiles with the Vault unlocked; a real OpenCode
  Conversation returned exact `E2E-OK`.
- COMP2-E is closed. COMP2 remains partial only through its independently
  tracked C/D scope-migration boundaries.

## Fallback live observation

- One explicit DeepSeek fallback run consumed approval version 1 after exact
  Enrollment revocation, completed its Provider Run successfully, and projected
  5,947 total tokens with account-level accounting.
- Its independent verifier returned safe reason `criteria_not_satisfied`, so
  the WorkItem was rejected and the Team did not succeed.
- A subsequent retry did not produce the required initial OpenCode ToolCall
  within the bounded five-minute window. This remains an open live gate rather
  than a simulated pass.

## Remaining boundary

This does not complete Phase 2D. The installed four-distinct-Provider Team,
successful approved fallback with accounting, and remaining controlled
account-local failure cells remain open. The native Mission/Team visual gate is
closed separately by
`P2D-W2B-W2D-mission-team-binding-ui-build117.md`.
