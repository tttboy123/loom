# P2A-W2 Vertical Native Journey Closure Implementation Review

**Date**: 2026-07-30
**Baseline**: `326f797656f3f39725471bdbdda91a3efbfe5a88`
**Scope**: staged same-W2 Vertical Native Journey Closure Repair Candidate
**Reviewer boundary**: fresh independent, read-only
**Verdict**: `PASS`

## Findings

- P0: none.
- P1: none.
- P2: none.

The Reviewer inspected only the staged diff against the frozen repair
contract. It made no edit, stage or commit and ran no live component, Keychain
or Provider call, installed Pi or Codex process, daemon signal or unrelated
working-tree action.

## Evidence checked

- Dynamic setup catalogs rebuild from Projection and the current immutable
  `GlobalReadView` before snapshot and Builder operations.
- Production setup catalogs derive from authoritative Runtime projection data,
  including the exact legacy `model_ids:null` followed by a different
  model-capable Runtime fixture.
- Builder sessions freeze their exact catalog, domain catalog, view and
  revision and reject later drift.
- An observed MiniMax verification appends one terminal fact through a
  cancellation-independent context bounded to one second, with no retry path.
- Secret Store failure before Provider observation appends zero facts.
- Keychain reads explicitly prohibit authentication UI.
- Only `credential_verify` receives the exact ten-second Go and Swift request
  timeout; every other local IPC method remains at five seconds.
- Swift presents the returned terminal verification revision/status and then
  refreshes projected native state.
- Product shutdown joins local IPC, observer, setup and database owners in
  order and closes each owner exactly once.
- Every staged file is inside the frozen ownership boundary. The secret scan
  found only the intentional `PRIVATE_SECRET` test/evidence literal.

## Reviewer commands

The Reviewer reproduced the focused Go and Swift cases for catalog drift,
credential terminal commit, Secret Store failure, Keychain non-interactivity,
IPC deadlines and joins, real setup refresh, private-UDS Candidate confirmation
and deterministic shutdown. It also ran staged diff ownership, whitespace and
secret-negative scans. All reproduced checks passed.

The Reviewer inspected the complete deterministic matrix evidence and did not
rerun that already recorded full matrix.

## Result

The repaired Candidate satisfies the frozen same-W2 vertical contract with no
severity finding. Complete deterministic GREEN plus this fresh independent
Implementation Review `PASS` unlock exactly one controlled native-window live
lineage:

```text
p2a-w2-live-20260730-005
```

No retry or second live canary is authorized. P2A-W2 remains unaccepted until
the live result receives fresh independent Result-Evidence Review `PASS`;
P2A-W3 remains locked and no P2A-W4 exists.
