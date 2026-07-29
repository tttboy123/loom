# P2A-W1 Native App Final Exit Closure RED

**Date**: 2026-07-28  
**Result**: `RED — EXPECTED FAILURE`  
**Live authority**: none  
**Live mutation**: none

## Test first

The behavioral fixture was added before the governed transaction harness:

```text
.loom-evidence/phase2a/P2A-W1/native-app-final-exit-transaction-test.sh
```

It specifies:

- owned mode-`0700` run-root preparation before fake bootstrap;
- pre-bootstrap failure with allowance unconsumed and exact rollback;
- successful initial bootstrap followed by readiness failure with allowance
  consumed, exactly one rollback, and no retry;
- successful fixture lifecycle with one classified restart;
- sentinel, canonical-path, symlink, mode, owner-when-supported, ambient
  override, unknown decision, and unknown argument rejection;
- bounded, secret-negative terminal output.

The fixture contains no live service label, Journal access, app installation,
Computer Use, Provider/Runtime action, or product-source mutation.

## Exact RED

Syntax:

```text
sh -n .loom-evidence/phase2a/P2A-W1/native-app-final-exit-transaction-test.sh
exit 0
```

Behavior:

```text
./.loom-evidence/phase2a/P2A-W1/native-app-final-exit-transaction-test.sh
exit 1
RED: governed final exit transaction harness is missing
```

A first outer capture command used zsh's read-only variable name `status` and
failed after invoking the test. It changed no file or external state. The
capture was repeated with `test_exit`, producing the exact verified RED above.

## External-state proof

After RED:

- original `loom`: `60c90ada...698`, mode `0755`, uid `501`;
- original `loomd`: `e5ab283c...14a`, mode `0755`, uid `501`;
- wrapper: `ff556602...3e4`, mode `0700`, uid `501`;
- plist: `2a6dc3e1...1f4`, mode `0600`, uid `501`;
- SQLite: `91ae07e0...8a4`, mode `0600`, uid `501`;
- observer: running;
- target-process five-marker count: `0`;
- SQLite integrity/Event count: `ok` / `1`;
- crash inventory: exact two historical reports;
- Candidate app/run/launcher/native process and Swift cache: absent;
- Git staging: empty.

## Gate

RED authorizes implementation only of:

```text
.loom-evidence/phase2a/P2A-W1/native-app-final-exit-transaction.sh
```

It authorizes no product edit, install, bootstrap, restart, native-window
canary, commit, or P2A-W2 work.
