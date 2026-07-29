# P2A-W1 Native App Host Controlled Live Canary Result Repair 1

**Date**: `2026-07-28`
**Status**: `PASS — ROLLBACK EVIDENCE REPAIRED`
**Candidate action**: none
**Additional canary/bootstrap/restart**: none

## Repair

Result Review 1 proved that the exact original `loom` and `loomd` bytes had
been restored with mode `0700` rather than the frozen original mode `0755`.

The repair first revalidated the exact original hashes, then changed only the
two installed original file modes to `0755`:

```text
loom
60c90ada487dc74b4670320664ed8ce6434c81be815667d651fe7212b2d47698
mode 0755

loomd
e5ab283c75473619094442eb349324b2bf09b8f59250ca1b7fd8b85fffb0014a
mode 0755
```

The original observer remained `running` and Git staging remained empty.
No file bytes, xattrs, plist, Journal, Candidate path, app, socket, process, or
authority state changed.

This is completion of the frozen rollback, not a product implementation
repair, Candidate invocation, hidden retry, or new live allowance.

Fresh independent Result-Evidence Re-review is required. P2A-W1 remains
`FAIL — ROLLED_BACK — HUMAN_REQUIRED`; P2A-W2 remains locked.
