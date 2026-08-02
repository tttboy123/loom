# P2A-W3 Complete Live Compatibility Contract Review 1

**Date**: 2026-08-02  
**Reviewed contract SHA-256**:
`646ea08e4cc8b3ee4a416ef09a0658a6a51e658e095e20e869ae5832deadb63f`  
**Role**: fresh independent read-only Reviewer  
**Verdict**: `FAIL / IMPLEMENTATION LOCKED`

## Findings

### P1 — strict MiniMax operation ownership was ambiguous

The contract required a strict `operation_id` on `credential_verify`, but did
not state whether enforcement belongs to the generic `internal/localipc`
protocol or the product-daemon method-parameter decoder. The owned files did
not include `internal/localipc/protocol.go` and its tests.

Required repair: freeze enforcement at the existing exact product-daemon params
decoder and its daemon tests, with the strict Swift real-server fixture proving
the cross-language wire. Generic localipc framing remains unchanged.

### P1 — observer transient predicate was not exact

The contract used the phrase `closed process-unavailable`, while current source
has exact timeout, process-failed, stderr, output-limit, invalid-output,
duplicate, binding and write error classes. Treating generic process failure as
recoverable could hide executable or integrity failures.

Required repair: only exact wrapped Pi metadata timeout errors for version and
list-models may be contained. Process-failed, binding drift, stderr,
output-limit, invalid/unknown output, duplicate identity, projection/writer,
socket and shutdown failures remain fatal.

## P2 guidance

- Freeze one small typed qualified-model parser that requires exactly one slash
  and exact provider/model equality; do not repeat inline string splitting.
- Same-operation MiniMax recovery must bind deterministic command identity to
  the existing Journal/Event idempotency-key path. It must not use an
  in-memory-only cache.

## Verified review inputs

- repository identity and HEAD matched the contract;
- all 20 pre-reopen file hashes matched;
- parent contract, Repair 3 source lock, Implementation Review 4 and remaining
  live Result Review hashes matched;
- live remained locked and no file was modified by the Reviewer.

**GATE**: `DO NOT IMPLEMENT UNTIL CONTRACT REPAIR RE-REVIEW PASS`
