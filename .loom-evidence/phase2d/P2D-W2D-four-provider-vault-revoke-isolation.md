# P2D-W2D four-Provider Vault revoke and corruption isolation

**Status**: `CURRENT / SOURCE CANDIDATE`  
**Date**: 2026-08-12  
**Goal**: Phase 2D only

## User outcome

A Team can retain three runnable Agents when the exact credential revision for
a fourth Agent is revoked or its encrypted Vault record is corrupted. The
affected Agent becomes blocked with a specific credential failure; Loom does
not relabel every Agent as offline or unavailable.

## Exercised matrix

The controlled Team coordinator test freezes four independent bindings:

| Agent | Harness | Provider | Provider Account | Model | Credential revision |
| --- | --- | --- | --- | --- | --- |
| Main | Codex | OpenAI | `openai.main` | `gpt-5.5-codex` | 3 |
| Sub-agent A | Claude Code | Anthropic | `anthropic.sub-a` | `claude-sonnet-5` | 7 |
| Sub-agent B | Loom Native | Kimi | `kimi.sub-b` | `kimi-k2.6` | 11 |
| Sub-agent C | Loom Native | MiniMax | `minimax.sub-c` | `MiniMax-M3` | 13 |

Each Adapter acquires a short-lived lease through the real
`CredentialLeaseManager` using the exact frozen Provider, Provider Account,
credential reference, and revision. The test revokes only the MiniMax identity
before dispatch. OpenAI, Anthropic, and Kimi complete successfully; MiniMax is
blocked as `credential_unavailable`. The Team terminal state may be blocked,
but each Agent retains its own authoritative result.

The Board diagnostic regression separately proves that
`credential_lease_revoke / credential_unavailable / retryable` is attached
only to the exact MiniMax Agent Attempt. Existing Provider auth, rate-limit,
and timeout diagnostics remain account-local. A diagnostic read failure
preserves authoritative Agent state instead of inventing a Team-wide outage.

## Encrypted-record corruption matrix

A second controlled run writes all four exact identities into a real
owner-only `LocalKeyFile` plus encrypted SQLite `VaultStore`, closes the Store,
and verifies that none of the four plaintext markers occur in the database
bytes. It then alters only the MiniMax ciphertext through an independent
SQLite connection and reopens the Vault using the original key file.

All four Adapters acquire through one production `CredentialLeaseManager`.
The corrupted MiniMax record fails closed at `vault_decrypt` and its Agent is
blocked as `credential_unavailable`; the OpenAI, Anthropic, and Kimi records
still decrypt through exact binding leases and those Agents succeed. The Board
regression separately proves that `vault_decrypt / credential_unavailable /
not retryable` reaches only an exact Incident, Provider, Provider Account, and
Model match.

A production-composition diagnostic test now drives a corrupted encrypted
DeepSeek record through VaultStore, CredentialLeaseManager, exact per-Agent
credential access, the Loom Native Adapter, persistent operational diagnostics,
and the exact Board diagnostic query. No Provider request is issued. The
persisted result is `vault_decrypt / credential_unavailable / not retryable`
for only the matching Incident, Provider, Provider Account, and Model.

## Privacy and authority

- No real API Key, Provider request, environment credential, or installed App
  is used.
- Test secrets are acquired only through leases and are not included in a
  binding, Journal fact, Board projection, diagnostic, or error. Caller-owned
  test buffers and database scan buffers are cleared.
- Credential reference remains an internal frozen execution identity; the
  public Board receives only non-secret Provider Account and revision data.
- Operational diagnostics enrich presentation only. They do not authorize
  retry, fallback, credential replacement, or dispatch.

## Rotation and restart continuity matrix

A third controlled run encrypts all four exact bindings, acquires a pre-rotation
OpenAI lease, and executes the production rotation transaction through
`WithRotationBarrier`, DEK rewrap, pending LocalKeyFile promotion, and canonical
key adoption. The old lease is revoked. The Vault and lease manager are then
closed and reopened from the promoted key file and encrypted database.

All four Agents acquire fresh leases after restart and succeed. Their frozen
binding digests and credential revisions are byte-for-byte unchanged from the
pre-rotation values. The Vault key version advances to 2 and no pending key file
remains. Rotation changes only wrapping authority; it does not silently switch
Provider, Provider Account, Credential Reference, Model, Harness, or revision.

## Explicit Vault-backed fallback matrix

A separate recovery run corrupts an OpenAI primary credential record and keeps
an independently encrypted Anthropic fallback record healthy. The primary
Attempt acquires only its exact OpenAI account/reference/revision and fails at
`vault_decrypt`. A second Attempt is created only because a versioned fallback
approval binds the primary and target binding digests. It acquires only the
Anthropic fallback account and revision, carries a new Role Context Capsule,
and succeeds. An unapproved binding change remains rejected before dispatch.

The four-Provider matrix also covers an account fallback without sacrificing
healthy peers. Codex/OpenAI primary revision 3 is corrupted; a separately
encrypted and explicitly approved `openai.main-backup` revision 5 becomes the
main Agent's second Attempt. Claude Code/Anthropic, Loom Native/Kimi, and Loom
Native/MiniMax each retain one successful Attempt. The run observes exactly
five complete Vault identities once each: primary, approved fallback, and the
three peer accounts. Provider ID alone is never used for credential lookup.

The main primary Attempt records `credential_unavailable`; its approved target
freezes a different binding digest, account, revision, and Capsule digest. All
four Agents finish successfully and no secret marker reaches the Journal. This
is source composition for explicit fallback and mixed-Team isolation, not an
installed Provider fallback claim.

Accounting follows the same frozen Attempt identities. The failed
`openai.main` credential Attempt has no accounting fact. The successful
`openai.main-backup` revision 5 Attempt records its own 55-token,
5,000-microunit Provider-reported fixture. Anthropic, Kimi, and MiniMax retain
their separate 77/7,000, 121/11,000, and 143/13,000 token/microunit facts. The
projection never attributes fallback usage or cost to the failed primary
account. These are controlled accounting facts, not real Provider invoices.

## Verification

- Focused coordinator and Board regressions pass ten consecutive runs.
- The same focused matrix passes ten runs under the Go race detector.
- Full `go test ./... -count=1` passes.
- `go vet ./...`, `git diff --check`, and complete Swift verification pass.
- Swift reports 198 XCTest cases with one intentional skip plus nine Swift
  Testing contracts, all with zero failures.
- The production diagnostic and retry-governance increment is packaged as
  unlaunched, uninstalled v0.5.2 build 60. An independent production rebuild is
  byte-identical for the App executable, bundled daemon, and `Info.plist`.
- The later fallback additions are test-only. Their focused normal and race
  matrices each pass ten runs, full Go/vet/diff/non-disclosure gates pass, and
  a fresh production rebuild remains byte-identical to build 60. Swift was not
  rerun for this test-only increment; the unchanged production source retains
  the build-60 Swift result above.
- A subsequent production UI increment adds an Agent-scoped key action for
  durable, non-retryable Vault failures. It uses a closed Vault-stage allowlist
  and opens the existing Runtime & Providers recovery surface without changing
  Agent authority. Full Swift now reports 199 XCTest cases with one intentional
  skip plus nine Swift Testing contracts. Full Go, focused ten-run race,
  vet/diff/non-disclosure, reproducible release, independent byte rebuild, and
  ZIP byte/mode gates pass. This increment is build 61; build 60 is historical.

## Open live gates

This closes source-level composition for exact lease revoke and real encrypted
record corruption plus post-rotation restart continuity, not CV6. Installed
Loom remains v0.5.2 build 39. The installed four-Provider Team, installed
per-account revoke/corruption behavior, counter-backed no-helper path, real
Provider calls after restart/rotation, accounting, and approved fallback remain
open under the sole `ACTIVE / PARTIAL` Phase 2D Goal.
