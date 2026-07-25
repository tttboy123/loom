# S2-W23 Contract Repair 1

- Repair type: authority correction before product RED
- Original contract SHA-256:
  `9c8669e9db0c42a77a67dc31f5674d0fbb1fc4ea0c6f532c18eb6d68f0963767`
- Product changes: none
- Owned-file changes: none
- Acceptance expansion: none

## Frozen correction

- Every `RuntimeStatusEventInput.Seq` must equal exactly
  `transition.PreviousSequence + 1`.
- Any lower, equal, or greater sequence fails before the appender is called.
- The writer still does not allocate sequences or read the current stream head.
- Accepted Journal remains authority for idempotency conflicts and whether the
  exact next `(stream_id, seq)` is already occupied.
- Real-Journal tests must prove that an occupied exact next sequence rejects a
  stale or repeated status Candidate without adding a row.

This correction prevents S2-W23 from creating a stream gap that accepted
projection replay cannot rebuild. It changes no payload, Candidate, ownership,
dependency, projection, scheduling, activation, or later Slice boundary.

STATUS: CONTRACT_REPAIR_FROZEN
