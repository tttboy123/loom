# P2A-W2 Credential Transaction and Shutdown Amendment Review 1

**Date**: 2026-07-30
**Reviewer boundary**: fresh independent, read-only
**Verdict**: `FAIL — CONTRACT REPAIR REQUIRED`

## Findings

- P0: none.
- P1: inherited pipes and a hidden helper argument do not authenticate the
  caller. The helper could otherwise become a directly invocable raw Keychain
  read/write interface. Activation must require the exact normal product-daemon
  parent lineage and deterministic direct-invocation rejection.
- P1: lost-response recovery cannot prevent a second Provider request using
  only the existing fresh per-call command ID. The contract must either own a
  stable identity through every layer or freeze a pre-Provider stale-terminal
  recovery check.
- P2: the helper wall-clock deadline is unspecified and must leave room for the
  five-second Provider and one-second terminal commit inside the ten-second
  verification request.
- P2: the diagnosis proves an unbounded Keychain owner consistent with both
  live failures, but no retained goroutine dump proves it was the exact stalled
  attempt-005 frame. The wording must not overstate causality.

Implementation remains locked pending Contract Repair 1 and fresh independent
Re-review.
