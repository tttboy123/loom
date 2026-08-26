# P2D-W2D Mission, Team and RoundTable UX V50

Date: 2026-08-20
Status: SOURCE VERIFIED / INSTALLED LIVE OPEN
Scope: native macOS Mission, Team and RoundTable interaction surfaces

## User problem addressed

Mission and Team did not give the user a direct next action, and RoundTable
created generic Writer/Target seats instead of letting the user compose the
participants. OpenCode was implemented in the runtime/provider layer but was
not consistently legible as an Agent route in this flow.

## Implemented

1. Added `LocalRoundtableAgentCandidate`, derived from the authoritative setup
   role catalog. Each candidate displays Harness, Provider, Provider Account and
   Model without exposing credentials.
2. Added native SwiftUI drag and drop from the Agent palette to the RoundTable
   seat zone. Added the equivalent per-row `Add` button for keyboard and
   assistive technology users. The first accepted Agent is Writer and the
   second is Target.
3. RoundTable creation now creates only the moderator seat. The UI waits for
   the user to choose Agents instead of silently inserting generic seats.
4. Kept compatibility for old sessions whose seats are `seat-writer` and
   `seat-target` but no longer have a current setup-catalog match.
5. Added a direct `Start Mission` action to executable Team rows. Read-only
   Teams remain visibly non-executable and do not expose a dead action.
6. Kept OpenCode as an existing supported route. This slice improves its
   discoverability and does not add a second runtime implementation.
7. Updated Team Agent and Provider Account route menus to include the Harness
   in their route summaries, with canonical `OpenCode` display text. Changed
   the RoundTable journey step to `Add two Agents` and kept the Team card's
   mission action reachable as a contained accessibility child.

## Verification

- `swift test --package-path apps/macos`: 282 XCTest passed, 1 existing visual
  export test skipped by its explicit visual-audit gate; 15 Swift Testing tests
  passed.
- `go test ./internal/roundtable ./internal/localipc ./internal/app
  ./internal/projection ./internal/work ./internal/runtime`: passed.
- `git diff --check`: passed.
- Added regression coverage for deterministic, bounded and control-character
  free RoundTable Agent seat IDs.

## Boundaries

This is a source/UI verification slice. No API key, credential, Prompt,
Provider response, network request, App reinstall or installed live gate was
used. The existing OpenCode live evidence remains the source of its prior
runtime support. Phase 2D remains `ACTIVE / PARTIAL`; installed CV6, mixed-Team
ATL9, failure isolation, accounting and COMP2-E gates remain open.

## Installed follow-up: 2026-08-21

Bundle: `/Users/lune/Applications/Loom.app` (rebuilt from the current worktree).

- Cold launch started the bundled daemon automatically and reached `Local service
  ready` / `Chat is ready with DeepSeek`.
- Team inspector rows now expose the real state of the Team's existing Mission.
  Existing `Succeeded`, `Failed`, and `Blocked` records show `Open Mission`; only
  a Team without a Mission record shows `Start Mission`.
- RoundTable live path completed `Create session` → `Add two Agents` using the
  visible `Add` controls. The installed screen showed `2/2 required`, Writer,
  Target, DeepSeek and OpenCode route summaries, including Harness, Provider
  Account, and Model.
- Mission cards now render a non-secret block reason when the authoritative
  projection supplies one, and Mission Outcome continues to link to Inspector
  recovery/diagnostics.

The live Mission start attempt reached preflight but hit an authoritative state
conflict from an existing Team Mission. The UI now makes that state explicit and
opens the existing Mission instead of claiming a new run started. This remains
an open Mission/new-Attempt lifecycle item, not a credential or daemon startup
failure. No key, prompt, provider response, or secret-bearing diagnostic was
read or written.

## Explicit new Attempt slice: 2026-08-21

The Mission command contract now has an explicit non-secret `new_attempt` flag.
When it is approved, the authority appends `TeamExecutionReopened` after the
old terminal event, then plans the fresh execution. Projection replay resets
only the current read model; the Journal retains the previous terminal lineage.
Restarted Work/Run identities are salted by the new correlation ID. A normal
Start request still returns the existing terminal result/conflict and cannot
silently mutate or rerun the old Mission.

The installed Teams workbench exposes `New Attempt` for terminal existing
Missions and explains that the old Mission is preserved. Source verification
passed Go work/app/projection/api suites and the full Swift suite (282 XCTest,
1 existing visual skip, 15 Swift Testing). v55 was rebuilt and installed at
`/Users/lune/Applications/Loom.app`; cold launch automatically started the
bundled daemon and restored verified DeepSeek chat. The real provider-backed
new-Attempt execution gate remains open until objective entry, preflight,
start, authoritative reopen/plan projection, and a live reply are observed.

## Live diagnostic checkpoint (2026-08-21)

The preflight lease now owns the frozen non-secret compilation, including the
resolved execution plan and context capsule references. Start checks the same
read-model version and command digest before dispatch, preserving fail-closed
behavior without reinterpreting dynamic preflight status. The selected
historical Mission reached `new_attempt=true` preflight but failed at
`dispatch_admission` because a persisted Role Context Capsule was invalid.
This is a governed diagnostic, not an offline or credential failure; the
installed live gate remains `PARTIAL / OPEN` pending a fresh valid capsule and
real provider reply.

## UX iteration checkpoint: 2026-08-21

- Team draft now shows the detected Harness inventory and status before role
  confirmation. OpenCode is explicitly described as a native Harness with its
  own authentication boundary, so absence or failure is actionable rather than
  appearing as an unsupported Provider.
- After a Team is confirmed, the Teams inspector keeps a visible `Start Mission
  with this Team` continuation. Users no longer have to rediscover the Team or
  lose the governance context created in the previous step.
- Roundtable creation generates the session ID automatically. Agent assignment
  is centered on configured Agent cards with drag-and-drop into the Writer and
  Target seats; the Add button and accessibility hint remain as non-gesture
  alternatives. Existing-session ID entry is secondary and collapsed.
- Swift verification after this iteration: 282 XCTest passed, 1 existing visual
  export test skipped by its explicit gate, and 15 Swift Testing tests passed.

This remains a UI/interaction improvement slice. It does not change the
governance boundary: only configured role options can be added, each seat keeps
its frozen route summary, and Mission execution still requires the existing
preflight and confirmation gates.

## Installed host recovery and startup deadline: 2026-08-21

The bundled App host now observes managed `loomd` termination and automatically
restarts that child after a bounded backoff. It only restarts the child it owns;
it does not take over an external daemon. `setup_snapshot` uses the extended IPC
request window so projection/runtime recovery is not presented as a permanent
unavailable state.

Installed live check: the rebuilt `/Users/lune/Applications/Loom.app` exposed
four conversation profiles and passed the DeepSeek network and OpenCode
conversation probes. After the bundled daemon was deliberately terminated, the
same App spawned a new canonical daemon and, after the recovery window, passed
the same DeepSeek probe again. No secret, prompt, or provider response body was
added to the evidence.

## Installed startup/observer checkpoint: 2026-08-21

The installed startup path now validates the canonical UDS instead of treating
the socket file's existence as health. A stale user-owned socket/lock is
boundedly cleaned before the bundled daemon is started, and the App waits in a
loading state for service readiness. Optional Pi metadata process failures are
contained as runtime-observation degradation so they cannot take down Product
IPC; state, IPC, and binding-construction failures remain fatal.

Installed verification passed cold-start daemon auto-launch, four conversation
profiles, DeepSeek network probe, and the OpenCode conversation gate (`E2E-OK`).
The state database is now opened in SQLite WAL mode with a single Product core
connection, and the installed live matrix subsequently passed consecutive
governance writes, first Mission, `ready_for_review`, refreshed preflight, and
explicit New Attempt (`running`). The mixed DeepSeek/MiniMax Team gate and
MiniMax credential failure isolation gate also passed. A view conflict now
causes the live harness to re-preflight; the Swift UI refreshes and asks for
Review preflight again rather than silently starting a changed plan.

This slice remains `PARTIAL` for the broader Phase 2D scope; Conversation
Segment/Capsule expansion and the remaining Runtime matrix are still open.

## Runtime status correction: 2026-08-21

An installed `setup_snapshot` audit showed the daemon's canonical healthy
runtime status is `online` for Loom Native, OpenCode, and Pi. The first UI
inventory pass only accepted `ready/running/available`, which falsely rendered
OpenCode as unavailable. The status mapping now includes `online`; Swift tests
and a fresh release install passed, followed by the installed DeepSeek IPC and
network probe.

## Attempt recovery closure checkpoint: 2026-08-21

The installed daemon now closes an abandoned `claimed` Run whose prepare lease
has expired with the authoritative failed reason
`agent_attempt_recovery_required`. This uses the normal terminal transaction,
releases runtime and Provider Account capacity, preserves Journal lineage, and
does not fabricate Agent output or dispatch a Provider call. Active or
non-expired Runs remain untouched.

Installed verification after cold launch recorded the recovery terminal fact at
2026-08-21 07:34:29. The App's bundled daemon became reachable again and the
DeepSeek IPC/network probe passed. Full `go test ./... -count=1` passed. The
remaining Mission new-Attempt gate is now an explicit governed preflight block
for the selected historical Team, not a stale lease or daemon availability
failure.

## New Attempt state semantics checkpoint: 2026-08-21

Added an installed-live helper that creates a fresh executable Team, runs its
first Mission, then submits an explicit `new_attempt` preflight/start against
the same Team instance. The daemon now waits for the new Work/Run/Grant lineage
before reading the projection, allows `ready_for_review` only through the
explicit RestartTerminal path, and maps incomplete dispatch to a retryable
`dispatch_incomplete` conflict instead of `state_unavailable`. Mission Board
also exposes New Attempt for `ready_for_review`.

The fresh installed run wrote TeamExecutionReopened and a new Attempt reserve
set to the authoritative Journal. The remaining blocker is real provider/tool
failure producing directly blocked nodes: the second start still needs a
terminal blocked projection rather than an incomplete dispatch response. This
is recorded as PARTIAL; no historical archived Team is used as a live fixture.

## Global connection banner correction: 2026-08-21

The installed snapshot was audited without reading secrets or message bodies:
local IPC was reachable, four conversation profiles were available, and the
network conversation probe passed for the configured brokered profile. The
only partial flag was `team_page.has_more=true`, representing normal Team
pagination. The UI previously promoted that page overflow to a global
"Some information is unavailable" banner, which made a healthy conversation
look offline. The connection-state policy now treats all page cursors as
normal online pagination; only observer/side-task/unknown degradation keeps
the global strip. Swift tests passed (283 XCTest, one explicit visual-export
skip; 15 Swift Testing), and the rebuilt installed bundle passed the network
probe. Phase 2D remains PARTIAL.

## Installed Provider profile recovery: 2026-08-21

Cold launch showed a recoverable startup race in which the main Product
snapshot became healthy before the independent setup/profile request. Without
retrying that surface, the conversation quick-start incorrectly asked the
user to connect a Provider even though verified profiles existed. The
resident monitor now retries idle or recoverably unavailable setup state while
the Product connection is online. A rebuilt installed App cold launch
recovered to four profiles, and the visible quick-start state became
`Chat is ready with DeepSeek · deepseek.primary`; the local IPC and real
network conversation probes passed. Swift passed 284 XCTest (one explicit
visual-export skip) and 15 Swift Testing tests. Phase 2D remains PARTIAL.
