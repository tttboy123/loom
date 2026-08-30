# P5 Build 220 compact and accessibility checkpoint

Status: `INSTALLED PARTIAL / COMPACT ACCEPTED / CORE INPUT ACCESSIBILITY ACCEPTED / RESTART CONTINUITY ACCEPTED / PHASE 5 OPEN`

## Installed identity

- product: Loom `0.5.3` Build 220;
- App SHA-256: `db1c1548fbb0968c697a909d243ecc769d76bf8b7899f8a631fb36729f2a38ad`;
- bundled daemon SHA-256: `585c2e72f4cb4ecb6815d68d4016373bd6a315bada6fe5fa19fd23333ed4830f`;
- strict deep bundle signature verification: passed;
- managed App/daemon parent relationship: passed;
- credential runtime: Loom Vault, with zero credential-helper spawn attempts.

The daemon hash is unchanged from Build 218. Build 220 changes the macOS
conversation shell, compact navigation and accessibility metadata; it does not
change Provider dispatch, credentials or execution authority.

## Source verification

- complete macOS suite: 408 XCTest cases, two conditional skips, zero
  failures;
- Swift Testing: 20 contract cases, zero failures;
- focused 900 x 760 Mission render at the largest SwiftUI accessibility text
  size: passed and visually inspected;
- `git diff --check`: passed;
- complete Go repository and `go vet ./...`: remain green from Build 218; the
  installed daemon binary is byte-identical.

## Compact-window acceptance

The installed App was resized to its 900 x 760 minimum window. Compact mode now
keeps only the stable work and governance destinations in the rail. It removes
the twelve indistinguishable historical conversation glyphs from that narrow
surface; all conversation history remains available through the existing
`Switch conversation` menu in the header. Wide mode retains the bounded twelve
item recent list.

The conversation remains mounted underneath the compact governance overlay.
Mission and RoundTable content do not overlap the rail, header or composer, and
Escape closes the overlay without discarding the conversation. The installed
RoundTable view shows its exact linked Mission, two frozen Agent routes and
their restored results. The installed blocked Mission view keeps the concrete
failure, Agent state, continuation input, `Review & continue`, RoundTable and
Details actions visible in one scrollable context.

The largest-accessibility-text render keeps the Mission title, progress,
blocking reason, continuation input and recovery action readable at 900 x 760.
No horizontal overflow or control overlap was observed.

## Accessibility contract

The installed accessibility tree now exposes named primary inputs:

- conversation composer: `Message Loom`;
- Mission intervention: `Mission continuation guidance`;
- running RoundTable seat input: `Guidance for <Agent>`;
- RoundTable retry input: `Retry guidance for <Agent>`.

The corresponding inputs have stable Loom accessibility identifiers and
action-oriented hints. In compact conversation mode, the App exposes 21
interactive elements instead of the previous 38 because the duplicate history
buttons are absent. Remaining unlabeled accessibility nodes belong to macOS
window controls or scroll infrastructure, not Loom command or input controls.

Full keyboard traversal is not accepted by this checkpoint. On the test system,
macOS Full Keyboard Access is disabled and Tab remains in the multiline
composer. Escape dismissal and direct accessibility actions pass, but complete
Tab-order acceptance requires an environment with Full Keyboard Access enabled.

## Cold start and restart continuity

A cold installed launch measured:

- first App window: 1.064 seconds;
- private daemon socket: 4.056 seconds;
- Agent Runtime completion observed by the probe: 17.926 seconds;
- daemon-recorded Agent Runtime elapsed time: 14,343 milliseconds;
- Agent Runtime Incident ID:
  `loom-agent-runtime-7403febc-2d32-4b8b-a260-a675cc4f18cd`.

After the App and daemon restarted, Build 220 restored the exact known
Mission-linked concluded RoundTable at 900 x 760. The accessibility probe found
the process, window, route, model and RoundTable controls, with no preparing or
invalid state. Restoration continues to revalidate the persisted non-secret
Mission ID against the authoritative Snapshot; stale references fail closed.

## Click-path acceptance

The installed accessibility action path verified:

1. opening standalone RoundTable shows the bounded recovery message `Open a
   Mission before starting a RoundTable` and a `Show Missions` action;
2. `Show Missions` exposes the current Mission without replacing the
   conversation;
3. opening the Mission shows its blocked state and labeled continuation input;
4. selecting RoundTable restores the exact Mission-linked Session and Agent
   results;
5. Escape returns to the conversation.

No credential was changed and no paid Provider request was started.

## Visual evidence

- `P5-BUILD220-COMPACT-ROUNDTABLE.jpeg`
  - SHA-256: `668276947404f511aac795db7c3128d1f8564e56ab361c6607c62bf4a728b753`
- `P5-BUILD220-COMPACT-MISSION-INTERVENTION.jpeg`
  - SHA-256: `36bc7922ffa91117be15078fc5d1b6595e20bebe5941d0d3e5d9ac392a6d572f`
- `build220-large-text/shell-compact-largest-text.png`
  - SHA-256: `9cdade96ecb435e7d7c0f516bb7a7b57ee5af19647b7d6db97778357a93faaed`

Build 219 was an intermediate installation. It proved compact history cleanup
and the conversation composer label, then accessibility inspection found an
unlabeled Mission continuation field. Build 220 adds labels and identifiers for
all in-context Mission and RoundTable guidance fields and supersedes Build 219.

## Open acceptance gates

- complete keyboard traversal with macOS Full Keyboard Access enabled;
- a new real Mission execution with visible incremental Agent output and user
  intervention;
- a new real Mission-linked RoundTable run through conclusion acceptance;
- installed conversation, Mission and RoundTable failure-path recovery beyond
  the currently restored blocked Mission;
- repeated cold-start distribution and further Agent Runtime preparation
  performance work.

Build 220 accepts compact installed rendering, core input accessibility and
restart continuity. It does not complete Phase 5.
