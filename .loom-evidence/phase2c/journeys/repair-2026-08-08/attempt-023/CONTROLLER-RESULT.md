# Phase 2C Replacement J9/J10 - Attempt 023 Controller Result

**Controller verdict**: `PASS`; independent journey and UI review pending.

Attempt 023 is the mandatory replacement J9/J10 run authorized by Repair 19.
It uses the Repair 20 source lock and signed Release. It does not rerun or
replace Attempt 022 J1-J8. Carrying those earlier authority journeys remains
subject to an exact independent review of the Repair 19 UI-label-only delta and
Repair 20 test-only recovery-classification delta.

## Binding

- Journey UUID: `d6f1b491-bcbc-43a0-9ea7-09a397aa7038`
- source-lock SHA-256:
  `7ad412e6b0122f3f37dfbbdaa3b5127f22d8b850963a1f1949fd14bf38ec2f1b`
- ordered 51-path source digest:
  `6fa2f269fefdaed41e60da0c64faf8f173924c427f23c1c5b0937693fab6cfa7`
- signed Release executable SHA-256:
  `a9b02d9e60afa96f8c79e18a73e8e8ca15bde61b1564ae0e1d5086dc7203bc4f`
- signed Release ordered bundle digest:
  `cbef8c096504bb4c6264833c8c0afc0ed5414e94952a8076cf6a5dbeb78a6844`
- journey CLI SHA-256:
  `0911fabbc17ce9aeef5ea20974a980b7c2c818d709300d174f34ca526c633183`
- journey daemon SHA-256:
  `217a10b143064f36451932d50bc56330005d035d7a705cf13da7db8b95a2d71c`

## J9 - Keyboard And Accessibility

PASS.

- The wide signed-Release AX audit found 17 enabled product controls and zero
  unlabeled product controls. Standard close, minimize, full-screen, and zoom
  window chrome is classified separately by native AX subrole.
- Four displayed Recent actions have four unique, non-empty descriptions. Each
  exact description equals its AX help:
  - `Open recent task 1, Recent work, Succeeded`
  - `Open recent task 2, Recent work, Cancelled`
  - `Open recent task 3, Attempt 022 Review Team, Created`
  - `Open recent task 4, Attempt 022 Review Team, Active`
- Real Tab navigation produced a visible focus ring on the governance control.
- Keyboard Space opened the governance inspector. Its AX tree exposed labeled
  Pin and Close controls while preserving the conversation lane.
- Real Shift-Tab moved focus to the labeled service-state control.
- Escape with Loom activated removed the governance Pin and Close controls.
- The Work destination opened the Board inspector. The full Mission Board
  opened as a second native workspace window; its audit found 29 product
  controls and zero unlabeled product controls.
- Escape reduced the native window count from two to one and returned to the
  preserved Board inspector.

## J10 - Visual Matrix

PASS.

The same signed Release was captured in light and real system dark appearances
at 900x640, 1080x720, and 1440x850 points. At 900 points the left navigation
collapses to its icon rail. At 1080 and 1440 points it expands to labeled
navigation and the four Recent rows. Across all six images:

- conversation remains the primary central surface;
- governance remains a secondary right-side or explicitly expanded surface;
- composer, service banner, navigation, and messages do not overlap;
- no text or controls are clipped;
- spacing, status colors, selection, and action dimensions remain stable;
- dark-mode AX still reports 17 product controls, zero unlabeled controls, and
  four unique Recent labels.

## Authority And Postflight

- The daemon log has 48 ordered request/response records: 24 `received` and 24
  `pass`.
- The IPC summary has 24 records and zero failed responses: 20 GUI and 4 TUI.
- Only read-only `snapshot`, `chat_thread`, `setup_snapshot`,
  `permissions_snapshot`, and `permissions_attention` methods were called.
- SQLite integrity is `ok`. The database SHA-256 remains
  `02b9e9a0f6f0256a0d0e2eb5c183680b9c130bde75a112f770a0dd997cfefd75`.
- The chat-thread file SHA-256 remains
  `3e14c75a53c0224bb3ecaa42ca83712d5c221d46677584056573c7302f9602eb`.
- Final service health is daemon serving, Journal available, and projection
  current. `observer_version_timeout` is the truthful partial-observer banner;
  it does not change the authoritative online Runtime fact or represent an
  offline state.
- The test app, daemon, socket, and socket lock are absent. System appearance is
  restored to light; global keyboard mode and app appearance override are both
  absent. The resident daemon was not modified or stopped.

This Controller result accepts only Attempt 023 J9/J10. It does not by itself
accept carried J1-J8, ADR-0015, Phase 2C, Product Owner sign-off, a commit, or
publication.
