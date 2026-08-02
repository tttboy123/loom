# P2A-W3 Authoritative Terminal and Observer Live Result Review

**Date**: 2026-08-02  
**Reviewer**: fresh independent read-only Reviewer  
**Result-lock SHA-256**:
`64e5c53dd3010892a1e4942a9abc16d369840043690737031bd64d6f2967ed94`  
**Evidence verdict**: `PASS WITH P2 LIMITATIONS`  
**Product verdict**: `FAIL / HUMAN_REQUIRED`

## Findings

### P0

None.

### P1

1. Neither live lineage entered the product socket. Both returned exit code `3`
   and the public line `daemon unavailable`, with no Journal delta. P2A-W3
   cannot be accepted; final walkthrough, commit, and another canary are
   ineligible.
2. The common classification is supported:
   `product_defect / codex_native_auth_symlink_identity_during_setup_construction`.
   Both manifests supply the symlinked npm Codex path. The product setup calls
   the native-auth constructor before execution/server construction; that
   constructor uses `Lstat` and requires the supplied path itself to be a
   regular executable. `run()` then masks the builder leaf as generic
   `daemon unavailable`.

### P2

1. The retained attempt roots do not contain an independent raw stderr or
   start-counter transcript. The historical one-start/no-retry facts therefore
   rely on the Controller result records. Current local state contains no
   contradiction: both manifest-copy hashes match, databases are unchanged,
   socket/lock are absent, isolation is empty, and no attempt process remains.
2. The Repair 2 source lock references the older
   `complete-live-failure-diagnosis.md`; the new result lock separately
   references `authoritative-live-construction-failure-diagnosis.md`. Both
   distinct hashes match their current bytes.

## Independent checks

- result lock, Repair 2 source lock, Implementation Re-review 3, both manifests,
  both result documents, both diagnoses, all owned sources, and locked authority
  hashes match;
- MiniMax SQLite is unchanged at
  `b0312739ba7b24961b9e8a698e4cbf436e9cace8758a1d2a058e050aec475e22`,
  integrity is `ok`, and its fact counts remain 1 configured, 3 verified, and
  1 Runtime discovery;
- Pi SQLite is unchanged at
  `677624b624b68ddd908939766752177a802bd90b657075133ef845e67e5461a2`,
  integrity is `ok`, and its exact six facts remain;
- state lock files are zero-byte mode 0600 and unheld; no attempt-root process,
  product socket, or product lock remains;
- the unrelated `demo-resident` daemon remains the only excluded resident;
- bounded scans found no raw secret, API key, credential payload, or token.

## Gate decision

The next permitted action is one new governed P2A-W3 repair boundary for
canonical Codex executable identity plus safe build-stage attribution, followed
by deterministic gates and a fresh independent Implementation Review.

Forbidden: reuse or retry of `minimax-003`/`pi-003`, final walkthrough, commit,
additional live action before a newly reviewed manifest, or creation of
P2A-W4.
