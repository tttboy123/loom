# P2D-W1E/W1F/W1G and W2D Failure Lab - Build 121

Status: `SOURCE + INSTALLED CONTROLLED-LIVE VERIFIED / PHASE 2D PARTIAL`

Date: 2026-08-23

## User-visible result

Loom `v0.5.3 (121)` keeps the complete installed directory and gives endpoint
review and failure isolation explicit governance surfaces. The strict Swift
setup probe observed 25 Providers, 7 Runtimes, 4 Conversation Profiles and 5
privacy-safe CC Switch import candidates. A real installed OpenCode
Conversation returned exact `E2E-OK`.

The endpoint sheet identifies source, Provider, protocol, origin, models,
endpoint fingerprint and review-policy digest. Approval grants permission only
for that exact destination for 15 minutes; it does not import or send a secret.
Fingerprint and policy values are copyable without expanding the layout.

Failure Lab presents six isolated scenarios with safe stage/code, retryability,
recovery action and Incident ID. Every scenario proves that a separate healthy
peer account succeeds:

| Scenario | Stage | Code | Retryable |
| --- | --- | --- | --- |
| Auth | `provider_auth` | `provider_auth` | no |
| Rate limit | `provider_rate_limit` | `provider_rate_limit` | yes |
| Timeout | `provider_connect` | `timeout` | yes |
| Insufficient balance | `provider_http` | `provider_insufficient_balance` | no |
| Corrupt Vault record | `vault_read` | `corrupt_vault_record` | no |
| Revision conflict | `agent_attempt_dispatch` | `credential_revision_conflict` | no |

## Contracts and implementation

- W1E validates exact HTTPS/public destinations, rejects URL credentials,
  redirects, proxy use, unsafe DNS/address results and policy drift, and freezes
  versioned policy/fingerprint digests.
- W1F appends requested and approved authority facts to the Journal with exact
  candidate/policy binding, revision checks and expiry; replay and restart do
  not recreate authority.
- W1G binds import candidates to deterministic digests, provides strict Swift
  decoding/review UI and account-scoped custom OpenAI-compatible verification.
  Setup drops an invalid optional candidate instead of hiding Providers,
  Runtimes or valid peers.
- FL1 supplies bounded loopback transport and credential-free temporary
  accounts. FL2 exposes a private-UDS route, strict client/store models and the
  native Failure Lab governance UI.

Two installation-loop defects were found and fixed. The App's socket health
probe now uses non-blocking connect, preserving ownership/type checks without a
UI stall. CC Switch source identity freezes device/inode/owner rather than file
size, so legitimate in-place SQLite growth remains visible; inode replacement,
symlink, ownership, mode and hardlink violations still fail closed.

## Installed verification

- Bundle: `/Users/lune/Applications/Loom.app`, `0.5.3 (121)`, strict deep
  code-sign verification passed.
- App executable SHA-256:
  `c4822ecfb23d1e71aaa6bba65d9535ca82d42fa321f70bca57393d3fa4008ca6`.
- Bundled daemon SHA-256:
  `b046216336b6fd702c5e476d08313060da87278f3c5533f1f715c4744d7ee338`.
- `TestLiveInstalledEndpointFailureLabAndCatalog` passed all six scenarios over
  the installed private UDS and asserted the 25/7 catalog plus bound candidate
  digests.
- `TestLiveOpenCodeConversationE2E` passed with exact `E2E-OK`.
- Live CC Switch discovery found five non-secret candidates after secure
  in-place database growth.
- Installed visual inspection showed all 7 Runtimes online, all 5 CC Switch
  candidates, the secretless Endpoint Review sheet, and six Failure Lab results
  with `healthy peer succeeded`; no endpoint approval was submitted.

## Source verification

- `go test ./... -count=1` passed.
- Focused race tests across CC Switch, endpoint policy/approval, Failure Lab,
  setup and daemon passed.
- `go vet ./...` passed.
- The complete macOS Swift suite passed 317 tests with 2 intentional opt-in
  skips and 0 failures; all 18 strict wire/model contracts passed.
- Reproducible App build, installer failure/signal/rollback fixtures, Phase 2D
  acceptance harness, strict signing and `git diff --check` passed.

## Remaining Phase 2D boundary

W1E/W1F and controlled FL1/FL2 are verified. W1G remains partial at the live
credential boundary: no real custom endpoint credential was imported or sent,
and an executable Agent Attempt with that custom frozen binding was not claimed.
The installed four-distinct-Provider Team also remains open until verified
OpenAI, Anthropic and Kimi Vault accounts are available. Phase 2D remains the
only `ACTIVE / PARTIAL` Goal.

No API key, Authorization header, credential reference, Prompt, conversation
content, Provider body, endpoint URL or user workspace content is present in
this evidence.
