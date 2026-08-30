# P5 Build 216 installed checkpoint

Status: `INSTALLED PARTIAL / RESTART CONTINUITY ACCEPTED / STARTUP P1 OPEN`

## Installed identity

- product: Loom `0.5.3` Build 216;
- App SHA-256: `13bdc7056ceb4a272442bec5e2d7a0a2c940d4e9185fe675425f5b753ea47e6a`;
- bundled daemon SHA-256: `9795d053beb888ac02c4b4db8c2b43e54182ef8c2ef67286d9b5a0a298ffc23e`;
- strict deep bundle signature verification: passed;
- managed App/daemon parent relationship: passed.

## Source verification

- complete macOS suite: 406 XCTest cases, two conditional skips, zero failures;
- Swift Testing: 20 contract cases, zero failures;
- restart selection tests require an exact authoritative Mission match and fail
  closed for stale references;
- `git diff --check`: passed before this evidence update.

## Installed restart continuity

Build 216 stores only three non-secret workspace restoration fields: inspector
destination, inspector mode and Mission ID. It does not persist Prompt text,
Agent output, Provider material or credentials in that preference record.

After a full App and managed-daemon cold restart, the App first presented a
focusable, truthful service-starting state. When the authoritative Snapshot
became available, the stored Mission ID was revalidated through the existing
Mission activation path and the same Mission-linked RoundTable restored with
its two Agent results and frozen routes. The visible context was not restored
from an untrusted cached Mission or RoundTable projection.

Screenshot:
`P5-BUILD216-RESTORED-ROUNDTABLE.jpeg`

Screenshot SHA-256:
`77c37ca8e099468b863c454d00771b45593d8978ad9cbc4e8f2f000964542fb7`

## Open acceptance gates

- first focusable UI appeared in about 1.4-2.5 seconds, but observed private
  socket readiness varied from about 17 seconds to more than 70 seconds;
- cold daemon construction therefore remains a P1 startup gap;
- compact installed rendering, full keyboard and large-text checks, installed
  failure intervention and a new real Mission/RoundTable workflow remain open;
- no credential was changed and no paid Provider call was started for this
  checkpoint.

Build 216 accepts exact restart-context continuity only. It is not Phase 5
completion.
