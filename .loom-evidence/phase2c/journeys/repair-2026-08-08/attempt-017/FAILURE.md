# Phase 2C Replacement Journey Attempt 017 - Failed

Attempt 017 is preserved and must not be promoted.

The Repair 16 source-locked build passed J1-J8 behaviorally. The main Journey
UUID was `2b5d7db4-ce51-4342-bd3d-e658733b7341`; the isolated decision fixture
used `1b9d763c-82ea-4b0e-83af-d9a3acb49279`. The run proved zero authority
before explicit Team confirmation and Mission start, exact `ApprovalDecided`
plus `WorkItemApprovalResolved` decision facts, authoritative Runtime
`online -> offline -> online` transitions, in-place Attention recovery, and
byte-identical controlled and ordinary restart rebuilds. Main Journal
identities were `212/212/212`; decision identities were `75/75/75`; both
SQLite integrity checks returned `ok`.

The Journey trace is nevertheless noncanonical. The J7 Attention TUI was
started without explicitly exporting `LOOM_JOURNEY_ID` before model
construction. Of 60 TUI IPC records, 58 used the required main UUID and two
read-only `permissions_attention` records used the generated UUID
`8b2be604-ae67-4a5e-b38b-689023a12a19`. No authority Event was caused by the
two reads, but Repair Amendment section 16 requires every replacement fixture
to supply the manifest Journey UUID before TUI model construction. This is a
journey-harness traceability failure, not a product-source defect.

J9/J10 were also unable to complete because the macOS desktop session was
locked. Waking the display showed the login screen; no locked-screen image is
retained as product evidence. Direct AX, keyboard, and visual claims are not
made from that state.

The signed app is stopped, global keyboard and appearance defaults are
restored, temporary daemons are stopped, sockets are removed, and the resident
demo daemon remains untouched. A clean Attempt 018 must set
`LOOM_JOURNEY_ID` on every TUI launch and rerun J1-J10 after the desktop is
unlocked.
