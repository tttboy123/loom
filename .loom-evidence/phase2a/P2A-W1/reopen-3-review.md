# P2A-W1 Reopen 3 Independent Review

**Date**: 2026-07-28
**Reviewer**: independent read-only Reviewer
**Verdict**: `PASS`
**Blocking findings**: none
**Activation authorized**: no

## Findings

1. Reopen 3 supersedes only the impossible loaded-service zero-marker
   predicate and consumed-canary stop. It preserves the rest of W1's authority,
   one-socket, read-only, rollback, and no-Provider-execution boundaries.
2. The replacement predicate is not weaker on product-controlled surfaces:
   the disk plist remains clean, the reviewed wrapper uses `env -i`, the child
   must inherit none of the five keys, and process/log/Journal/Evidence/TUI/
   screenshot surfaces remain secret-negative.
3. Read-only checks confirm exact attribution:
   - the loaded service exposes the five named markers;
   - the user-level launchd manager holds the same five non-empty values;
   - non-emitting comparisons match;
   - the running Loom child contains none of the marker names;
   - the disk plist contains none;
   - the wrapper allowlist contains only `PATH` and `LANG`.
4. Any extra service-only marker, child inheritance, value disclosure, or
   ambient-value change fails closed.
5. No `setenv/unsetenv`, credential deletion/restoration, alternate daemon/
   socket, Provider/model/Runtime action, or authority write is introduced.
6. Installed rollback hashes, absent launcher/run directory/sockets, one-Event
   SQLite integrity, running original observer, and empty staged diff still
   match the fail-closed evidence.
7. The Reopen explicitly requires a new user authorization phrase before one
   final canary and forbids a fourth or hidden retry.

## Gate result

The Reopen contract is safe to authorize. This Review does not activate it,
does not change the current `HUMAN_REQUIRED` status, and does not permit a
third bootstrap by itself.
