# P2A-W1 Native App Host Live Pre-Bootstrap Repair 1 Implementation Review 1

Date: 2026-07-28  
Reviewer: independent read-only Reviewer `p2a_w1_implementation_review3`  
Verdict: `FAIL`

## Finding

Severity: `HIGH`

The install transaction armed `cleanup` for `EXIT`, `HUP`, `INT`, and `TERM`,
but that cleanup removed only temporary files. It did not call
`restore_transaction`, and signal delivery did not terminate the installer.
A signal between the first live file swap and final trap removal could leave a
partially updated resident generation or continue execution after signal
delivery.

The rollback transaction restored snapshots from its generic cleanup when
active, but likewise used the same handler for signals and normal exit; a
signal could restore and then allow the body to continue. Both paths violate
the frozen fail-closed transaction boundary.

## Reviewer reproduction

The Reviewer independently reproduced:

- focused installer fixture: `PASS`;
- shell syntax: `PASS`;
- exact live resident dry-run: non-mutating `PASS`;
- diff checks and empty staging: `PASS`.

The legacy pair/triple compatibility logic, optional-launcher state model,
symlink rejection, no fabricated prior launcher, and existing deterministic
fixtures were otherwise coherent.

## Required repair

The same Repair must:

1. capture deterministic signal RED inside the install mutation window;
2. capture deterministic signal RED inside the rollback mutation window;
3. restore the complete pre-transaction generation on either signal;
4. terminate with a fixed nonzero result instead of continuing;
5. preserve the sentinel guard so test hooks cannot be activated accidentally;
6. repeat the complete verification and fresh independent Review.

No live authority is implied. The Candidate bootstrap count remains `0` and
the native allowance remains `1`, unconsumed.
