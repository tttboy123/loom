# P5 Build 221 RoundTable correctness checkpoint

Status: `INSTALLED PARTIAL / LATEST-ROUND AUTHORITY ACCEPTED / REPLACEMENT PROJECTION ACCEPTED / RESTART CONTINUITY ACCEPTED / PHASE 5 OPEN`

## Installed identity

- product: Loom `0.5.3` Build 221;
- App SHA-256: `3e58b77b2da2d1b9107a23689ba38205139f29cf3d87d4cf52e9ee4d5a527c40`;
- bundled daemon SHA-256: `7316981fc6894ad69df6dae66432f64a87fdcfb115b2f047e6b08801e272672a`;
- strict deep bundle signature verification: passed;
- App PID after restart: `7764`;
- managed daemon PID after restart: `7809`, parent PID `7764`;
- installed private Socket is owned by the managed daemon.

## Closed correctness defects

### Latest round owns conclusion readiness

RoundTable previously selected a seat's latest Attempt by attempt number across
the entire Session, then checked whether that Attempt belonged to the latest
round. Because attempt numbers restart for each round, a failed retry number 2
from Round 1 could incorrectly hide a successful attempt number 1 from Round 2
and block conclusion.

Build 221 first freezes the latest round ID and only compares Attempts from
that round. The daemon and macOS readiness projection now use the same rule.
The regression matrix creates a failed Round 1 retry number 2, opens Round 2,
succeeds both active Agent attempts at number 1 and concludes successfully.

### Replaced seat projects the current Agent

RoundTable previously rebuilt a seat row from the pre-open catalog candidate
whose generated seat ID matched the original seat. After governed Replace, the
route came from the new frozen binding but the visible Agent name and role could
still come from the old candidate.

Build 221 resolves active bound seats by the authoritative
`AgentDefinitionID + TeamRoleKind + RuntimeProfileID` tuple. The seat ID remains
stable for intervention and Journal lineage, while display name, role and route
come from the replacement candidate plus the current frozen binding. A strict
Swift regression verifies `Reviewer` with its MiniMax route replaces the old
Writer identity on the same seat.

No historical Attempt, seat membership receipt or frozen execution binding is
rewritten by either fix.

## Verification

- complete Go repository: passed;
- `go vet ./...`: passed;
- focused `internal/roundtable`, `internal/app` and `cmd/loomd` matrix: passed;
- complete macOS suite: 410 XCTest cases, two conditional skips, zero failures;
- Swift Testing: 20 contract cases, zero failures;
- latest-round daemon regression: red before the fix, green after the fix;
- latest-round and replacement-projection Swift regressions: red before the
  fix, green after the fix;
- `git diff --check`: passed.

## Installed continuity

Build 221 was transactionally installed over Build 220. At 900 x 760, the App
restored the existing blocked Mission and exposed its labeled intervention
input. Selecting RoundTable restored the exact Mission-linked concluded Session
with two MiniMax Agent results, frozen routes and readable output while keeping
the conversation mounted.

After a full App and managed-daemon restart, the daemon appeared as the App's
child within the two-second process probe. The RoundTable showed an explicit
preparing state and restored the exact concluded Session within the twenty-
second accessibility probe. No `invalid_request`, blank governance surface or
wrong Mission appeared.

The installed historical Session contains one round and no Replace event, so
the cross-round and replacement mutations are accepted by deterministic
authority/projection regressions rather than represented as a fabricated live
mutation. Build identity, restart projection and compact rendering are verified
through the installed bundle.

## Visual evidence

- `P5-BUILD221-COMPACT-ROUNDTABLE.jpeg`
  - SHA-256: `0f963e90bbaa945b32f74c861d51862cc51ab16eca57cd960ef3eccdfba96617`
- `P5-BUILD221-RESTARTED-ROUNDTABLE.jpeg`
  - SHA-256: `6d4c7267b14aef3f63e1b0682ade2c1c707f34115d5df704ece13b42cbe959cc`

No credential was changed and no paid Provider request was started.

## Open acceptance gates

- a new real Mission execution with visible incremental Agent output and user
  intervention;
- a new real Mission-linked multi-round RoundTable through conclusion;
- an installed governed Replace journey that visibly changes the seat identity
  and starts its new Attempt;
- complete keyboard traversal with macOS Full Keyboard Access enabled;
- broader installed conversation, Mission and RoundTable failure recovery;
- repeated startup distribution and further Agent Runtime preparation work.

Build 221 accepts the two RoundTable correctness boundaries and preserves the
Build 220 compact/accessibility acceptance. It does not complete Phase 5.
