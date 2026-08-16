# Phase 2C Replacement Journey Attempt 018 - Failed

Attempt 018 is preserved and must not be promoted.

The Repair 16 source-locked binaries used the main Journey UUID
`6383d1f0-c01c-44ab-aa18-ac812fed6965` and the isolated decision UUID
`11567479-dbf1-42ff-870f-476beafa0547`. Every main TUI IPC record used the
main UUID and every decision IPC record used the decision UUID. J1-J6 passed:
chat remained non-authoritative, folder context survived daemon restart with
no sentinel leak, Team and Mission authority required their explicit actions,
and the decision Journal contained exactly one `ApprovalDecided` plus one
`WorkItemApprovalResolved` fact with `75/75/75` unique identities and SQLite
integrity `ok`.

J7 failed after the controlled offline checkpoint. The main Mission had
legitimately reached `ready_for_review` with one succeeded work Run and one
failed `insufficient_evidence` verifier Run. Controlled `online -> offline`
projection appended exactly one `RuntimeInstanceStatusChanged`, exposed one
`restore_runtime` Attention action, and replayed byte-identically at
`138/138/138` identities with SQLite integrity `ok`. The ordinary recovery
daemon then failed closed in `build_execution` on repeated attempts. A
read-only diagnostic build against a temporary source copy exposed the wrapped
cause as `mission execution conflict`.

The source defect is in `ResumeProjectedMissions`: it accepts only `running`
and `awaiting_recovery` as resumable nonterminal states and rejects the valid
quiescent `ready_for_review` state. A daemon therefore cannot restart after a
Mission reaches human review. This violates J7/J8 and opens Repair 17. The
repair must preserve `ready_for_review` without launching a Run, retain exact
resume behavior for `running` and `awaiting_recovery`, and continue to reject
unknown nonterminal states.

J8-J10 were not run after the blocking daemon startup failure. The macOS
desktop also remained locked, so no native AX, keyboard, or visual claims are
made. Temporary app/global defaults were restored, temporary processes and
sockets are absent, and the resident demo daemon was not modified.
