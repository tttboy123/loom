# Phase 2C Replacement Journey Attempt 015 - Failed

Attempt 015 is preserved and must not be promoted.

The Repair 14 source-locked signed Release passed J1-J8. The main Journey UUID
was `fb479af9-6dc1-4c8e-bddf-caa63123ba60`; the isolated approved-decision
fixture used `003f5df2-5d07-459d-932a-fba4b73ad7d4`. The run proved ordinary
no-folder chat without authority, safe first folder chat after daemon restart,
explicit Team confirmation and Mission start, exact `ApprovalDecided` plus
`WorkItemApprovalResolved` facts, authoritative Runtime
`online -> offline -> online` recovery, in-place Attention refresh, and
byte-identical restart rebuilds. Main Journal identities were
`261/261/261`; decision identities were `75/75/75`; both SQLite integrity
checks returned `ok`. The folder sentinel did not appear in any other regular
journey evidence file.

Live native J9 failed. Six Recent rows were enabled `AXButton` elements with an
`AXPress` action but empty help, title, and value. Their parent was the product
scroll area, not `AXScrollBar`; the actual scroll controls were separately
identified by `AXIncrementArrow`, `AXDecrementArrow`, `AXIncrementPage`, and
`AXDecrementPage`. Directly pressing the first unlabeled element selected the
visible `Inspect Failure` Recent row and produced a visible focus ring. These
are therefore unlabeled product actions, violating J9's unique human-label
requirement. J10 was not run after this blocking failure.

Repair 15 invalidates the Repair 14 source lock and is limited to adding dynamic
Recent-row help in `LoomWorkspaceShell.swift` plus regression coverage in the
existing `LoomGraphiteViewTests.swift` path. The app and both temporary daemons
shut down cleanly, both sockets are absent, global keyboard and appearance
defaults were restored to their original absent state, and the resident demo
daemon remained untouched.

Temporary evidence roots remain read-only:

- `/private/tmp/loom-phase2c-chat-20260808-015`
- `/private/tmp/loom-phase2c-decision-20260808-015`
- `/private/tmp/loom-phase2c-decision-20260808-015-deny-rehearsal`

The `deny-rehearsal` root is explicitly non-canonical. An initial keyboard
rehearsal used direction keys that do not choose a decision and therefore
submitted `Deny`; its copied manifest also retains paths to the original setup
root. It is preserved only as excluded harness history. The canonical decision
fixture was rebuilt independently at
`/private/tmp/loom-phase2c-decision-20260808-015`, selected `Allow once` with
the supported `]` key, and is the only decision root used by the J6 claims and
the `75/75/75` identity count above.
