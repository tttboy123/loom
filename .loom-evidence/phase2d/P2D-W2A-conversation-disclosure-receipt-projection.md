# P2D-W2A Conversation Disclosure Receipt Projection

Date: 2026-08-12

Status: `CURRENT / SOURCE VERIFIED / BUILD 45 PACKAGED / NOT INSTALLED`

## User-visible result

Ordinary Loom Conversations now project the non-secret Context disclosure
receipt frozen for each immutable Route Segment. The first visible turn in a
Segment shows a native expandable summary with the number of context items
shared, the number omitted, and the receipt digest. It does not expose Prompt
text, transcript content, Provider responses, credentials, ciphertext, nonces,
or hidden reasoning.

## Frozen runtime contract

- Segment, Attempt, and Provider responder request carry the same bounded
  `disclosure_receipt_digest`, `disclosed_context_count`, and
  `omitted_context_count` for the dispatch that created them.
- The execution binding digest schema is version 2 and binds the Capsule
  digest, disclosure receipt, disclosed count, and omitted count beside the
  Segment, Profile, and context mode.
- New persisted records recompute this binding during validation. Receipt or
  count substitution with an unchanged binding fails closed.
- Existing records that predate disclosure fields remain readable only when
  the receipt is absent and both counts are zero. Their existing binding digest
  is preserved rather than silently rewritten.
- Schema-1 plaintext migration generates a current receipt and v2 binding when
  it creates the immutable Segment.

## Verification

The following gates passed on branch `codex/loom-platform-slice2` from baseline
HEAD `651f156afda37a8e703cbc0396f9f38b7912600b`:

- focused Go persistence, immutable Segment, disclosure-drift rejection, and
  20-run stability tests;
- `go test -p 1 ./...`;
- `go test -race ./internal/api ./cmd/loomd`;
- `go vet ./...`;
- `git diff --check`;
- complete Swift tests: 181 XCTest cases, one intentional visual-preview skip,
  and eight Swift Testing contracts, with zero failures.

The regression matrix covers restart preservation, old-wire decoding without
the new fields, missing-receipt/nonzero-count contradictions, zero-disclosed
receipts, receipt substitution, count substitution, immutable source Segment
preservation, and responder-request propagation.

## Boundary

This source was written after the frozen uninstalled v0.5.2 build 44 Candidate
and is not present in build 44. It is packaged as the unlaunched, uninstalled
v0.5.2 build 45 Candidate at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build45-candidate-2026-08-12/BUILD-MANIFEST.md`.
Release, arm64, deep-signature, owner-only/no-symlink, wire-contract, and ZIP
byte-equivalence checks pass. Installed Loom remains v0.5.2 build 39.

This increment does not close full deterministic token packing, scoped
retrieval, disclosure approval for trust-domain changes, installed encrypted
migration/restart, DeepSeek-to-Anthropic transition, sibling Attempts and
aggregation, or the mixed-Team live matrix. Phase 2D remains `ACTIVE / PARTIAL`.
