# Final Live Gate Repeated-Failure Problem Analysis

Date: 2026-07-27

Scope: read-only analysis after Implementation Review 2

## Root cause

The locked Pi 0.82.1 Agent loop maps Provider `start` to top-level assistant
`message_start`; only text/thinking/tool deltas become `message_update`; and
Provider `done`/`error` becomes top-level assistant `message_end`. RPC mode
forwards the resulting AgentSession events. The synthetic contract and fixture
incorrectly exposed nested start/done updates.

The local-model boundary inferred a lexical common root and validated leaf
files, but did not establish an explicit root or validate every parent
component against symlink, mode, owner, and realpath escape.

## Bounded correction

Technical Contract Correction 2:

- freezes the exact top-level Pi event sequence and final-message success check;
- adds explicit `PrivateRoot`;
- adds a read-only shared binding inspector;
- walks and snapshots the full directory/leaf chain, uses Unix no-follow leaf
  opens, hashes opened descriptors, and revalidates immediately before start;
- makes the pre-live manifest consume that same resolved binding; and
- discloses the residual same-user pathname-reopen race rather than claiming
  descriptor-bound execution that the current Go/llama.cpp interface lacks.

No product change, external materialization, or live execution is permitted
until fresh independent Contract Review 3 passes.

ANALYSIS: COMPLETE
