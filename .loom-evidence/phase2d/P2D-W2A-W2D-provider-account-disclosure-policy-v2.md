# P2D-W2A/W2D Provider Account Disclosure Policy v2

**Date**: 2026-08-12  
**Status**: `CURRENT / SOURCE ACCEPTED`; Segment binding and installed CV6 remain open

## User impact

Runtime & Providers now records an explicit, versioned disclosure policy for
each Provider Account alongside capacity and budget limits. The policy uses
closed values for trust domain, retention mode, and data region. The UI labels
these as user-selected account policy and does not claim Loom independently
certified Provider retention or region guarantees.

Legacy v1 account-limit events remain byte-for-byte replayable. They display as
disclosure policy unspecified. A new configuration writes v2 only when all
three disclosure fields are present and valid; partial or unknown values fail
closed.

## Authority and projection

- Provider Account policy v2 includes `trust_domain`, `retention_mode`, and
  `data_region` in its canonical digest.
- The v1 digest algorithm and historical payload remain unchanged.
- Setup mutation requires response plus authoritative projection refresh before
  Swift publishes success.
- Provider Account directory and Conversation Profile project the exact policy
  version, revision, digest, and closed disclosure values for that account.
- Accounts without v2 policy never inherit another account's policy and cannot
  masquerade as ZDR or region-scoped.

No credential, endpoint, Prompt, conversation body, Provider response, or
secret enters the policy event or setup projection.

## Verification

- v1 replay and v2 authority/digest/invalid-enum tests pass.
- Account-local projection and exact Conversation Profile projection pass.
- daemon closed-wire correlation and secret-field rejection pass.
- Swift strict result/directory/profile decoding, store projection refresh, and
  native disclosure Picker tests pass.
- Complete Swift passes 200 XCTest cases with one intentional visual-export
  skip plus nine Swift Testing contracts.
- Full Go, five focused race runs, and `go vet ./...` pass.

## Open boundary

This slice does not yet freeze the policy revision/digest on ordinary
Conversation Segment and Attempt records. That is the next W2A/W2D slice and
must use daemon-resolved Profile policy, not client-supplied authority.
Installed Loom remains build 39; no real credential, installed App, or Provider
was touched, and installed CV6 remains open.

## Frozen Candidate

The production increment is frozen as unlaunched, uninstalled v0.5.2 build 63
at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build63-candidate-2026-08-12/BUILD-MANIFEST.md`.
An independent retained rebuild made after `swift package reset` and an
independent ZIP extraction are byte- and mode-identical to the Candidate. Build
62 remains historical evidence for Conversation Vault failure governance.
