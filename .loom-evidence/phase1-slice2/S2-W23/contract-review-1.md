# S2-W23 Contract Review 1

- Reviewer: fresh independent read-only contract Reviewer
- Reviewed contract SHA-256:
  `9c8669e9db0c42a77a67dc31f5674d0fbb1fc4ea0c6f532c18eb6d68f0963767`
- Branch/head: `codex/loom-platform-slice2` at `b0cf75f`
- Result: bounded contract repair required

## Finding

The original contract allowed any caller sequence strictly greater than the
transition's previous discovery sequence and described the Journal as final
authority for stale or occupied stream sequences.

Accepted Journal validation rejects nonpositive or occupied stream sequences
but does not enforce contiguous per-stream sequence. Accepted projection replay
does enforce contiguity before applying known or unknown Event types. Therefore
an unoccupied sequence gap such as previous `1` to new `3` could append
successfully and later make projection rebuild fail with `ErrSequenceGap`.

The same rule also allowed a fresh ID/key and later unoccupied sequence to
append a repeated status fact derived from the same stale S2-W22 baseline.
Journal cannot identify that semantic staleness when both key and sequence are
fresh.

## Required bounded correction

- Require each caller sequence to equal exactly
  `transition.PreviousSequence + 1`.
- Reject both non-advancing sequences and sequences greater than the exact next
  value before append.
- Limit Journal authority wording to idempotency conflict and an occupied exact
  next stream sequence.
- Add real-Journal proof that an occupied exact next sequence rejects a stale
  or repeated Candidate without adding a row.

No other blocking contract finding was reported. No product file was created or
changed.

VERDICT: REPAIR
