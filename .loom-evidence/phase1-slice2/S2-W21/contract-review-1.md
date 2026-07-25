# S2-W21 Contract Review 1

- Reviewer: fresh independent read-only contract Reviewer
- Reviewed contract SHA-256:
  `58b58a0635bf3f29685c16dfc8819704458bce6de5ee27814d7fbbbd1df986ff`
- Branch/head: `codex/loom-platform-slice2` at `501ac33`
- Result: bounded contract repair required

## Finding

The acceptance criteria required rejection of changed correlation, Event ID,
and idempotency-key values, but accepted S2-W20 deliberately treats these as
caller-owned metadata. No authoritative payload field binds their exact values.
Only emptiness, immutable-content conflicts under reused IDs/keys, and stream
sequence rules are currently decidable.

Implementing the original wording would invent a second metadata authority or
change accepted S2-W20 semantics. The contract must instead require rejection
of empty values and accepted replay conflicts while explicitly permitting
alternate nonempty unique caller-owned metadata.

No other blocking contract finding was reported.

VERDICT: REPAIR
