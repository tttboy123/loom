# P2A-W1 Native App Host Live Pre-Bootstrap Repair 1 Implementation Re-review 2

Date: 2026-07-28  
Reviewer: independent read-only Reviewer `p2a_w1_implementation_review3`  
Verdict: `PASS`  
Findings: none

## Independent conclusion

The prior high-severity signal finding is closed.

The Reviewer confirmed:

- install and rollback use separate normal-exit cleanup and
  `HUP`/`INT`/`TERM` handlers;
- an active transaction restores its complete snapshot;
- signal handlers ignore reentrant signals and terminate with status `98`
  instead of resuming mutation;
- both deterministic signal hooks require the regular non-symlink sentinel;
- legacy binary-pair install does not fabricate a prior launcher;
- optional launcher state swaps correctly between pair and triple generations;
- existing clean install, update, failure, rollback, TUI, symlink, and
  secret-negative fixtures remain intact;
- exact live resident dry-run remains non-mutating;
- the repair stays inside frozen P2A-W1 packaging scope.

## Reviewer commands

The Reviewer independently reproduced:

```text
scripts/test-install-loom-local-product.sh
  installer fixture PASS

sh -n scripts/install-loom-local-product.sh \
  scripts/test-install-loom-local-product.sh
  PASS

guarded signal environment without sentinel
  PASS — no hook activation

exact live resident dry-run
  PASS — non-mutating

git diff --check
git diff --cached --check
staged path count
  PASS — staging count 0
```

## Activation accounting

The Reviewer performed no install, bootout, bootstrap, UI launch, Computer Use,
or live mutation. Candidate bootstrap count remains `0`; the native allowance
remains `1`, unconsumed.

This PASS closes the Repair's deterministic and implementation-review gates.
It does not reuse the pre-repair activation. The frozen contract requires a
new exact post-Review user message:

```text
P2A-W1 Native App Host and one controlled native-window canary
```
