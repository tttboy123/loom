# Phase 2C Replacement Journey Attempt 014 - Failed

Attempt 014 is preserved and must not be promoted.

The Repair 14 source-locked signed Release passed the complete deterministic
matrix. Attempt 014 then passed J1-J6 through the controlled decision fact:

- ordinary no-folder chat returned a safe untrusted proposal with zero
  Team/Mission/Run authority;
- the first folder-scoped chat after daemon restart returned safely in
  `4.813750s`, with no raw timeout and no folder-sentinel leak;
- a structured Team Draft created no fact before explicit confirmation;
- Team confirmation created one executable Team but started no work;
- a separate preflight plus explicit Start created one governed Mission; and
- the isolated decision fixture committed `ApprovalDecided` and
  `WorkItemApprovalResolved` under its own Journey UUID.

J7 evidence setup then used the real discoverable Pi runtime path while the
controlled Runtime-offline fixture was active. The daemon correctly committed
the authoritative `online -> offline` transition before IPC readiness, then its
ordinary observation immediately committed `offline -> online` before the
offline UI checkpoint. This was a journey-harness configuration error, not a
product defect, but it prevented a clean offline-state capture and added an
extra recovery fact. Attempt 014 therefore stopped without running J8-J10.

No Candidate source changed. The Repair 14 source lock remains valid and a fresh
Attempt 015 must rerun J1-J10 from empty state, using an empty runtime search
directory for the controlled offline checkpoint and the real Pi path only for
ordinary recovery.

Temporary evidence roots remain:

- `/private/tmp/loom-phase2c-chat-20260808-014`
- `/private/tmp/loom-phase2c-decision-20260808-014`
- `/private/tmp/loom-phase2c-decision-20260808-014-setup-failed-local-model`
