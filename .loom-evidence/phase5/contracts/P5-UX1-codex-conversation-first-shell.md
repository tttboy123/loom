# P5-UX1 / P5-UX2 / P5-UX3 - Codex-inspired conversation-first shell

Status: `ACCEPTED / INSTALLED / PHASE 5 COMPLETE`

## Goal

Make the conversation the stable working context while preserving Loom's
governed Mission, Team and RoundTable authority. A new user can start typing
without first choosing or understanding those objects. Governance remains
available from the current context and becomes more explicit only when the user
starts governed work.

## Evidence boundary

The reference is the locally installed Codex desktop app:

- bundle identifier: `com.openai.codex`
- short version: `26.818.31338`
- build: `6892`

The local bundle exposes a thread-centered shell, compact-window mode,
responsive sidebar width, a composer utility bar, local/cloud run-location
selection, workspace/working-tree starting state and thread side-panel tabs.
Its design tokens use a 4 px spacing base, 14 px body text, restrained surface
and border contrast, a responsive sidebar near 275 px, and one composed input
surface whose advanced controls are menus. These observations are design
evidence, not a claim that Loom should copy Codex source or its product model.

Installed Loom Build 211 was inspected through the accessibility tree and
desktop screenshots. The baseline exposed:

- seven peer navigation destinations before the recent conversation list;
- more than ninety visible historical conversations in one scroll surface;
- duplicate navigation between the rail, header conversation menu and right
  governance inspector;
- folder, Team, Mission, Route and Model controls simultaneously above the
  first empty composer;
- full-sheet Mission and RoundTable workbenches that interrupt conversation
  continuity;
- an empty state that foregrounds system activity instead of the user's task.

## Interaction principles

1. Conversation is the default and durable center.
2. `New task` creates a blank conversation; it does not create a Mission.
3. Primary navigation contains Chat, Missions and Needs You.
4. Agent Teams and RoundTable are governance capabilities, not peer chat modes.
5. Library and Runtime & Providers are tools and health surfaces.
6. The rail shows a bounded recent set; older conversations remain reachable
   through an explicit overflow menu and the conversation switcher.
7. Folder, Agent Team and Mission actions share one composer add menu.
8. Route and Model remain visible because they materially change the next
   execution binding; advanced policy detail stays in menus and the inspector.
9. Mission, Team and RoundTable state must progressively appear beside the
   current conversation rather than forcing an unexplained mode switch.
10. Every running, blocked and terminal state must have visible progress,
    result or recovery at the point where the user is already working.

## WorkItems

### P5-UX1 - Reference and click-path audit

- freeze local Codex and Loom evidence;
- define information hierarchy, terminology and responsive behavior;
- keep Phase 4/2D authority and security boundaries unchanged.

### P5-UX2 - Conversation-first shell

- one `New task` entry;
- grouped primary, governance and tool navigation;
- bounded recent conversation list with overflow;
- one composed input surface with progressive add actions;
- desktop and compact-window accessibility coverage.

### P5-UX3 - In-context governed work

- Mission opens as the current task's work context rather than an isolated
  application mode;
- Team and RoundTable are reachable from the Mission/conversation relationship;
- preserve frozen bindings, explicit confirmation and RoundTable seat authority.

The first P5-UX3 source slice keeps the conversation mounted while a linked
Mission opens in the right context panel. It reuses the authoritative Mission
route and Team timeline, displays bounded live Agent output with the frozen
Harness/Provider Account/Model route, and exposes blocked guidance as a new
audited Attempt review. Decision and Needs You entry paths resolve to the same
Mission context. Team selection stays in the context panel and loads its
authoritative timeline. RoundTable receives the current Conversation, Mission
and Team link; the full Mission workbench remains a secondary details action.

RoundTable now has its own conversation-side governance destination. It
restores the exact Mission-linked Session, polls only while visible Agent
Attempts are running, and projects each seat's frozen Harness, Provider
Account and Model with bounded visible output. Running seats can receive
capability-gated Steer input; failed or cancelled seats expose stage, code,
Incident ID and explicit Retry guidance. Pause and Accept conclusion continue
to use the existing Moderator authority. The full workbench is a secondary
setup/details path rather than the only place to observe progress.
Installed Build 214 also renders bounded Agent output as safe inline Markdown:
formatting is readable, external links are removed, and the round summary
counts actual Agent results rather than presenting an empty activity count.

### P5-UX4 - Live work and recovery

- one status language for conversation, Mission, Team and RoundTable;
- streaming Agent output, results, incidents and blocked intervention stay in
  the current work context;
- no silent failures and no generic offline/unavailable collapse.

The first P5-UX4 source slice uses the same status colors and language across
Mission and RoundTable. Mission node failures now show safe stage/code,
terminal reason, Incident ID, retryability and recovery action next to the
Agent output. RoundTable Session/Mission mismatches fail closed with an
actionable message instead of remaining in an indefinite loading state.

### P5-UX5 - Installed acceptance

- desktop and compact-window screenshots with no clipping or overlap;
- keyboard navigation, VoiceOver labels and large text checks;
- restart restores the same conversation and linked governed work;
- real conversation, Mission intervention and RoundTable workflow complete in
  the installed App without terminal assistance.

## First-slice acceptance

- App opens in a blank, focusable conversation.
- `New task` never creates governed state.
- no more than twelve conversations occupy the rail at once.
- Mission and Team remain reachable from the composer's `+` menu.
- Runtime & Providers remains reachable through the health footer.
- all former navigation destinations remain reachable.
- linked Mission selection does not replace or cover the conversation;
- running Mission activity and blocked intervention appear in the context panel;
- Mission-linked RoundTable receives the exact current Mission relationship;
- Mission and RoundTable contexts restore directly without covering the conversation;
- RoundTable Agent progress, Steer, Retry, Pause and conclusion review remain governed;
- macOS source tests, view tests and compact-width rendering pass.

## Non-goals for the first slice

- no change to Mission, Team, RoundTable or Journal authority;
- no Provider, credential, dispatch or execution-binding change;
- no remote publication, credential mutation or paid live execution;
- no claim that the partial P5-UX3/P5-UX4 installation closes P5-UX5.

## Source verification

- complete macOS suite: 415 XCTest cases, two conditional skips, zero failures;
- Swift Testing: 20 contract cases, zero failures;
- desktop 1,280 x 760 and compact 720 x 760 SwiftUI bitmap rendering;
- Mission and RoundTable context rendering at 1,280 x 760 and overlay-width
  900 x 760 with the conversation still mounted;
- visual review caught and closed compact-rail section-title wrapping;
- the existing eight-state light/dark appearance matrix remains unique and
  nonblank after moving background activity out of the empty conversation;
- complete Go repository, `go vet ./...` and the focused startup race matrix;
- `git diff --check` passes.

The P5-UX3 source candidate also verifies that a linked Mission opens through
`openMissionAndActivate`, keeps the conversation visible, follows only the
visible Mission while it is live, retains explicit failure recovery, and does
not weaken Mission, Attempt, RoundTable or Journal authority.

## Installed checkpoints

Loom `0.5.3` Build 227 was the installed candidate for this Mission slice. The
App and bundled daemon pass strict deep signature verification and run with the
daemon bound to the App parent PID. Desktop accessibility inspection verifies
the Conversation-first first screen and the real click path from a linked
Mission to its RoundTable without replacing the conversation. The existing
concluded RoundTable visibly projects two independent MiniMax Agent results,
their frozen Harness, Provider Account and Model, and a secondary full-detail
action.

Build 218 introduced deferred Agent Runtime construction after the private product
service becomes available. Three cold starts placed socket readiness near
5.2-6.4 seconds. A final accessibility-timed run showed the App window at
0.120 seconds and the conversation Route and Model controls at 9.480 seconds,
while the background Agent Runtime finished at 40.604 seconds and the exact
Mission-linked RoundTable restored at 41.349 seconds. No user-visible
`invalid_request` appeared. Socket readiness is not treated as conversation
readiness; the Route and Model controls are the installed usability marker.

Build 220 established the compact and accessibility boundary. It reduces a
900 x 760 rail to stable work and governance destinations; historical
conversations remain in the header
switcher instead of appearing as twelve indistinguishable compact glyphs. The
installed Mission and RoundTable overlays preserve the conversation, pass
visual inspection without clipping, close with Escape and restore the exact
Mission-linked Session after App/daemon restart. The conversation composer and
all in-context Mission/RoundTable guidance fields now expose stable accessible
labels, hints and identifiers. A 900 x 760 Mission render at the largest
SwiftUI accessibility text size also passes.

Build 221 additionally fixes two RoundTable correctness defects found during
review. Conclusion readiness now selects the latest Attempt inside the latest
round, so an older higher-numbered retry cannot mask a newer successful round.
Active seat rows now resolve the current Agent identity from the authoritative
frozen binding, so governed Replace cannot leave the old Agent name or role next
to the new route. Daemon and Swift regressions were red before these fixes and
green after them.

Build 227 adds a real new-Mission vertical slice on top of that checkpoint.
The App materializes a fresh TeamInstance when the chosen saved Team already
has Mission history, so a new title and objective cannot silently reopen the
old Mission. A two-Agent MiniMax Mission completed under the fresh identity and
returned to its linked conversation. Terminal Agent output is exposed only
through a digest-checked, bounded board projection sourced from accepted
Evidence. It survives an App/daemon cold restart even when the incremental
timeline is empty, without synthesizing Journal events or changing pagination.
The inspector, linked Mission list, intervention and RoundTable relationship
all prefer the user-visible Mission presentation title over the saved Team
name. Installed evidence is recorded in
`../live/P5-BUILD227-MISSION-CONTINUITY.md`.

Build 221 persists only the inspector destination, mode and bounded Mission
ID. Once the authoritative Snapshot is available, it revalidates that Mission
through the
existing activation path and restores the same RoundTable and Agent results;
stale references fail closed. Agent Runtime preparation is now a neutral,
retryable state instead of the rejected Build 217 red placeholder error.
Complete keyboard traversal with macOS Full Keyboard Access, broader installed
failure recovery and a new real multi-round RoundTable run remain open. Evidence is
recorded in `../live/P5-BUILD220-COMPACT-ACCESSIBILITY.md` and
`../live/P5-BUILD221-ROUNDTABLE-CORRECTNESS.md`. Build 227 is not
Phase 5 completion.

Build 229 adds an installed RoundTable continuity checkpoint. Retry restores
the original authoritative discussion prompt from the prior Attempt's exact
encrypted Context Capsule after restart-safe lookup; user guidance enters the
new Capsule separately as a role-scoped confirmed constraint with intervention
provenance. The new Attempt retains the frozen seat binding while receiving a
new execution, Run, Segment and Capsule identity. Neither content value enters
the RoundTable Journal. Full-workbench failure cards now retain stage, recovery
language and copyable Incident ID, and conclusion responses retain projected
Agent output instead of clearing it until the next Snapshot.

The installed run also makes the remaining deliberation-context gate explicit:
the original question is present, but relevant prior Agent result artifacts are
not yet disclosed to the synthesizing Attempt. Phase 5 therefore still requires
a fresh substantive run that carries those outputs only as bounded,
provenance-bearing `untrusted_model_output` context and produces an accepted
conclusion after real peer contributions. Build 229 remains a checkpoint, not
Phase 5 completion. Evidence is recorded in
`../live/P5-BUILD229-ROUNDTABLE-CONTINUITY.md`.

Build 247 is the current installed Phase 5 candidate. It completes the unlocked
installed click gate left open by Build 243: both independently bound Agent
results retain identity, route and status before a bounded summary; summary
cuts occur only after full Markdown rendering and at complete word boundaries;
and accessible `Show full result` / `Show less` controls work in place. The
Conversation-side and full RoundTable surfaces also share structured operation
failure evidence with stage, retryability, recovery action, diagnostics and
Incident ID. A full App/daemon restart restored the same RoundTable, both
collapsed summaries and composer focus. Complete keyboard traversal with macOS
Full Keyboard Access and installed failure-injection coverage remain open, so
Build 247 is not Phase 5 completion. Evidence is recorded in
`../live/P5-BUILD247-RECOVERABLE-ERRORS-AND-RESULT-SUMMARIES.md`.

Build 253 closes the remaining installed boundary. Full Keyboard Access
traversal covers the composer, progressive add/route/model controls,
governance inspector, Mission intervention and RoundTable result/detail paths
without focus traps. Controlled UDS fault injection verifies actionable,
correlation-aware Conversation, Mission and RoundTable recovery without
Provider dispatch. Mission diagnostics work inside the existing review sheet,
and a rapid-restart regression restores an owned bundled daemon when an adopted
Socket disappears with the previous App. The complete macOS suite passes 432
XCTest cases with two conditional skips plus 20 Swift Testing contract cases.
Package identity, setup projection and installed evidence are recorded in
`../live/P5-BUILD253-CONVERSATION-FIRST-UX-ACCEPTANCE.md`.
