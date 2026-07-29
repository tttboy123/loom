# P2A-W1 Native App Final Exit Closure Implementation Review

**Date**: 2026-07-29  
**Final verdict**: `PASS`  
**Findings**: none  
**Review mode**: fresh, independent, read-only  
**Live authority**: none

## Review history

The first review returned `FAIL` on two harness-only findings:

1. fixture counter writes could follow a pre-existing symlink outside the
   private fixture root;
2. production preflight did not reject a pre-existing
   `Loom.command.previous`.

Repair 1 introduced validated atomic counter writes, three outside-byte
regressions, and the missing launcher-backup preflight predicate.

The Repair 1 re-review confirmed those fixes but returned `FAIL` on two
additional boundaries:

1. inherited command-control variables could affect Python and Git commands;
2. dangling launcher and binary-backup symlinks could satisfy `! -e`-only
   absence checks.

Repair 2 added the clean `env -i` re-execution boundary, allowlist validation,
ambient-command regression cases, and exact `! -e` plus `! -L` checks.

Historical `FAIL` verdicts remain true for the reviewed revisions. This final
verdict applies only to Repair 2.

## Independent findings

None.

The Reviewer independently confirmed:

- the harness re-executes through an absolute clean-environment boundary
  before fixture or production dispatch;
- the child validates its environment allowlist and removes its internal
  marker;
- `BASH_ENV`, `ENV`, `PYTHONPATH`, `GIT_INDEX_FILE`, and the installer
  signal-control variable cannot affect the reviewed commands;
- a forged clean marker plus an injected variable fails closed;
- `Loom.command`, `Loom.command.previous`, `loom.previous`, and
  `loomd.previous` require both nonexistence and non-symlink status in
  production preflight and rollback proof;
- fixture counter writes reject symlink/non-regular/foreign-owned/wrong-mode
  targets and the three outside files remain byte-identical;
- rollback is armed before run-root or product mutation;
- initial bootstrap accounting occurs immediately after the one successful
  command and before readiness;
- no initial-bootstrap backedge exists;
- the lifecycle restart follows only `ui_verified`;
- preserve follows only `result_review_pass` and final private-root
  validation;
- EOF, unknown decisions, and signals retain rollback;
- production remains locked without a passing activation record.

## Independent commands

```text
sh -n native-app-final-exit-transaction.sh
PASS

sh -n native-app-final-exit-transaction-test.sh
PASS

native-app-final-exit-transaction-test.sh
final exit transaction fixture PASS

env -i PATH=/usr/bin:/bin:/usr/sbin:/sbin \
  native-app-final-exit-transaction-test.sh
final exit transaction fixture PASS

production invocation without activation record
exit 20
FINAL_EXIT_RESULT result=preflight_failure initial_bootstrap_calls=0 consumed=0 rollback_count=0 restart_count=0

git diff --check
PASS

git diff --cached --check
PASS

cached diff
empty
```

## Authority

This `PASS` accepts only the deterministic harness implementation. It grants no
installation, bootstrap, daemon restart, Computer Use action, native-window
canary, Result-Evidence Review, commit, or P2A-W2 authority.

