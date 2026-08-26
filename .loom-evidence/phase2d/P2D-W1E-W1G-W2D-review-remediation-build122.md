# Build 122 Codex review remediation

Date: 2026-08-23

Status: `SOURCE + INSTALLED CONTROLLED-LIVE VERIFIED / PHASE 2D PARTIAL`

## User impact

Build 122 closes the catalog-disappearance and App-freeze risks found in the
Codex review without reducing Loom's Provider or Runtime surface. The installed
strict Swift contract decodes 25 Providers, 7 Runtimes, 4 Conversation Profiles
and 5 privacy-safe CC Switch import candidates. OpenCode remains visible and
available to Agent Teams.

An invalid CC Switch import candidate is now isolated from the authoritative
Setup catalog. Every candidate that reaches Swift carries its candidate digest,
endpoint fingerprint, endpoint-review policy version and policy digest. A bad
candidate is dropped instead of making the whole Setup Snapshot undecodable.

The App-owned daemon Socket probe is non-blocking and bounded to 100 ms. It
validates the socket owner, type and path, handles connect completion through
`poll` and `SO_ERROR`, removes only an owned stale Socket, and schedules daemon
recovery after an unexpected child exit. It no longer performs an unbounded
blocking `connect` on `MainActor`.

## Endpoint governance remediation

- Endpoint approval is account-scoped and versioned. Expired approval can be
  renewed after restart through a new immutable approval generation.
- Approving a different endpoint candidate for the same Provider Account
  supersedes the previous approval. The verifier fails closed on ambiguity.
- Swift caches approval by candidate digest plus Provider Account ID, so one
  account cannot accidentally consume another account's authority.
- The native review sheet presents the exact endpoint as selectable text in
  addition to origin, protocol, model list, fingerprint and policy bindings.
- Endpoint review remains secretless. Approval does not import a credential or
  execute a Provider request.

## Failure-isolation remediation

Failure Lab now runs both the target Agent and its healthy peer over a bounded,
credential-free loopback execution path. The daemon assigns distinct opaque
temporary account IDs and rejects arbitrary production-account hints. Strict
Swift decoding enforces the exact scenario-to-stage/code/retryability mapping.

The installed governance UI exposes per-Incident recovery guidance, Retry only
for retryable scenarios, View diagnostics and Copy incident ID. The six
controlled scenarios remain deliberately synthetic: they prove routing,
projection, error governance and peer isolation, but do not claim corruption of
a real Vault record or a real Provider failure.

## Verification

- `go vet ./...`: pass.
- Focused endpoint, Setup and Failure Lab tests: pass.
- Focused race tests for endpoint review, custom endpoint verification, Failure
  Lab and Setup Snapshot: pass.
- `go test -p 4 ./... -count=1`: pass.
- `swift test --package-path apps/macos`: 317 XCTest cases, 2 intentional skips,
  0 failures; 18 Swift Testing cases pass.
- `scripts/test-build-loom-local-app.sh`: two reproducible production builds and
  native App build fixture pass.
- Strict bundle signing and installer dry-run: pass.
- Installed private-UDS Failure Lab acceptance: pass before and after daemon
  restart.
- Installed daemon self-recovery: pass. The App replaced the terminated child,
  restored the owner-only Socket and returned the same 25/7/4/5 strict Setup
  projection.
- Installed native UI inspection: Vault Unlocked; 7 Runtimes Online including
  OpenCode; 25 Providers visible; all six Failure Lab results show contained
  peer state, safe Incident data and governed recovery actions.
- `git diff --check`: pass at the Build 122 verification boundary.

One unrestricted-parallelism repository run observed a single metadata probe
timeout while the Provider and Runtime catalogs remained present. The exact
test then passed five consecutive runs, and the repository's bounded
`-p 4` gate passed. This result is preserved as load sensitivity, not rewritten
as a catalog regression.

## Installed artifact

- App: `/Users/lune/Applications/Loom.app`
- Version: `0.5.3 (122)`
- App executable SHA-256:
  `8eb5b832fd5d1091dc91642f1266316728365f2d771a0c62390de66619d07b5a`
- Bundled daemon SHA-256:
  `94779aac4640e37af0f2dd8b53600fafc3fa5678747eced0549bf0a58f7271f2`

## Remaining Phase 2D gates

This checkpoint does not close Phase 2D. A real approved custom-endpoint
credential and executable frozen binding remain an operator-provided live gate.
The installed four-distinct-Provider Team matrix still requires independently
verified OpenAI, Anthropic, Kimi and MiniMax/DeepSeek accounts. The controlled
Failure Lab is evidence for governance and isolation, not a substitute for
those real-account acceptance cells.
