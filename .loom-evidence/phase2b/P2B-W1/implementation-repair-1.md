# P2B-W1 Implementation Repair 1

Date: 2026-08-03

The first independent Implementation Review returned `FAIL` with zero P0,
six P1 and zero P2 findings. No canary was consumed. This bounded repair stays
inside the frozen P2B-W1 owned paths and closes those findings as follows:

1. Generic Mission recovery now consults the authoritative Side-task
   continuation gate before compiling a projected Team. A nonterminal
   synthetic Side-task child remains reserved for `ReconcileSideTasks`, which
   re-derives the frozen child plan and binding.
2. `propose` and `create` validate the supplied parent execution digest through
   the current Phase 2A execution backend before any Artifact or Journal write.
   Wrong but syntactically valid digests fail with `digest_mismatch` and zero
   Event change.
3. Parent-handoff projection now requires schema v1, contiguous per-stream
   sequence and the complete frozen Side-task, decision, handoff, parent,
   generation, digest and effect tuple. Any failure preserves the prior view.
4. Input, summary and ContextPacket Artifacts now enforce the frozen semantic
   schemas, UTF-8 byte and collection bounds, non-null sorted unique required
   collections, closed enums, exact reference kinds and digests, UTC times,
   usage semantics, canonical bytes and duplicate-key rejection.
5. Effect completion now binds terminal Evidence to the exact current Attempt
   of the one-node continuation Team, or to the exact authorized node/current
   Attempt of the cancellation Team. Evidence from another or earlier Attempt
   cannot complete the effect.
6. The exact Candidate manifest excludes the pre-existing modified
   `internal/projection/team_execution_test.go`. That file is neither owned,
   reviewed nor staged by P2B-W1.

Post-repair verification passed:

- focused Go app/work/projection/daemon tests;
- serial whole-repository Go tests;
- serial whole-repository Go race tests;
- `go vet ./...` and `go mod tidy -diff`;
- repeated Side-task CAS/projection race tests, count 20;
- vertical deterministic loopback test, count 2;
- Swift tests: 79 XCTest, one visual-only skip, plus four Swift Testing tests;
- Swift Thread Sanitizer with the same result; and
- Swift Release build.

Implementation Review must be repeated against the exact Candidate boundary.
The controlled offline canary remains locked until a zero-finding PASS.
