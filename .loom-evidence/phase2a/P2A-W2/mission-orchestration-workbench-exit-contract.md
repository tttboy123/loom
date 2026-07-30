# P2A-W2 Mission Orchestration Workbench Exit Contract

**Date**: 2026-07-30
**Status**: FROZEN — Contract Repair 2 Re-review PASS
**Authority**: explicit Product Owner authorization
**Reopens**: the complete `P2A-W2 Team Builder and Provider Onboarding` exit
**Freezes**: the Phase 2A Interaction Continuity Contract and Mission
Orchestration Workbench Extra Goal
**Baseline**: `bcd27c1b37527e3caed3c2b1e5ec6194f2590512`
**Risk**: STRICT — native product interaction, local IPC mutation, Rules/Work
authority delegation, Evidence-gated completion, recovery generation fencing,
Provider credential boundary and native live evidence

## 1. Authorization, precedence and one vertical boundary

The Product Owner authorized one complete P2A-W2 Exit Contract reopen and one
Phase 2A Interaction Continuity Contract. This document is the only active
P2A-W2 product closure. It supersedes the earlier prohibition on further
single-point Amendments while preserving every historical contract, Candidate,
failed canary and review as immutable evidence.

This is one vertical WorkItem. It:

- creates no `P2A-W2a`, `P2A-W2b`, `P2A-W4` or wrapper-only WorkItem;
- keeps P2A-W3 locked until this complete exit passes;
- does not reinterpret live attempt 007, whose reviewed result remains
  `FAIL — NATIVE_PROVIDER_MANAGEMENT_UNREACHABLE`;
- permits no further point Amendment after this contract passes Review;
- changes no Event Journal schema and no accepted Event payload;
- changes no Grant, Evidence, StateWriter, Scheduler, Supervisor, Runtime
  execution or Provider authority;
- adds no second Journal, Projection authority, queue, scheduler, mission
  database, execution protocol or client-side authority;
- permits exactly one post-Review controlled native-window vertical canary.

If this exact owned boundary cannot close the capability without changing an
accepted Event, Grant or Evidence authority, work stops `HUMAN_REQUIRED`.

## 2. Product outcome

Loom becomes a local Mission orchestration workbench:

```text
Workspace Orchestration Board
  -> Mission
     -> Mission Room
        -> Plan / Node / Attempt / Run
        -> Team Pulse
        -> Evidence Inspector
        -> Loom Decision Sheet
```

The ordinary-user promise is:

> I give one Mission to one Team. Loom plans, orchestrates, schedules,
> supervises, recovers and verifies it within explicit authority.

The public information architecture uses:

- **Mission**: a user-visible Projection facade over an existing
  WorkPackage/TeamExecution lineage;
- **Team**: the existing confirmed TeamInstance and its AgentInstances;
- **Plan**: the existing deterministic Team execution plan;
- **Node**: an existing logical TeamExecution node;
- **Attempt**: one generation-fenced execution attempt;
- **Run**: the existing authoritative Run;
- **Evidence**: the existing digest-bound acceptance input;
- **Orchestration Board**: Workspace-level Mission overview;
- **Mission Room**: one Mission's plan, activity, output and review surface.

Mission is not a new domain authority. Its stable public ID is derived from the
existing TeamExecution/WorkPackage lineage and cannot be used to invent a Run,
Evidence item, Agent, status or progress value.

## 3. Preserved authority boundaries

The only state path remains:

```text
Event Journal
  -> rebuildable Projection / GlobalReadView
  -> LocalProductReadService
  -> private versioned Go IPC
  -> strict Swift client and Bubble Tea client
```

Mutation remains:

```text
explicit user action
  -> strict local IPC command
  -> prepared authority command lookup
  -> existing Rules or Work authority
  -> Journal CAS
  -> Projection refresh
  -> final UI movement
```

Rules:

1. UI pending state is presentation only.
2. A Mission card moves finally only after a new Projection version confirms
   the authoritative fact.
3. A stale view, stale request digest, stale claim generation, stale Grant,
   missing Evidence or missing prepared command fails closed with zero write.
4. Board drag/drop expresses intent only. It never writes a status directly.
5. An Agent may reach `ready_for_review`; it cannot mark a WorkItem or Mission
   complete.
6. Completion requires the accepted Work authority path and exact Evidence
   lineage already enforced by `CommitTeamNodeAcceptance`.
7. Recovery requires an already prepared, deterministic
   `TeamRecoveryInput`; the product surface cannot reconstruct or invent a
   RecoveryPolicy from projection digests.
8. No client receives a raw Grant, credential, authorization presentation,
   hidden reasoning, raw Provider response or unbounded output.

## 4. Prepared decision boundary

P2A-W2 may add a product command facade, but it is not an authority.

The facade accepts only a closed decision command containing:

- schema version;
- decision kind;
- Mission/Team identity;
- exact Projection view version;
- exact decision/request ID and digest;
- exact logical Node, Attempt and claim generation when applicable;
- one closed action;
- one unique correlation ID.

The single new protocol method is exactly:

```text
mission_decision
```

Its closed operation field selects read, defer or submit behavior. No
`decision_*`, per-sheet, alias or version-suffixed method is permitted.

The facade delegates only to a prepared backend supplied by the owning
authority:

- `Authorization`: an existing pending `rules.ApprovalRequestRecord` and exact
  `rules.ApprovalDecisionRequest`;
- `Review Gate`: an existing fully prepared acceptance command whose source
  and verifier Evidence already pass the accepted Work authority validation;
- `Recovery Decision`: an existing fully prepared deterministic
  `work.TeamRecoveryInput`.

The facade must never:

- rebuild private domain types from display strings;
- infer a missing policy, contract, receipt, Grant or generation;
- write an Event directly;
- retry a conflict;
- convert `Not now` into `Deny`;
- treat sheet dismissal as authorization;
- broaden a RuleSet, Grant or Mission scope;
- synthesize a prepared command merely because a card is visible.

`Allow for this Mission` is enabled only when the authoritative prepared
request is already bound to this exact Team/Mission scope. It authorizes no
future Mission, does not create an ambient grant, and cannot widen the prepared
scope. Otherwise the action is unavailable and `Edit scope` returns to the
non-authoritative proposal flow.

Production data without a prepared command remains truthfully visible in
`Needs You`, with mutation actions disabled. Deterministic and live fixtures
may inject prepared commands only by constructing the real accepted Rules/Work
authority inputs and Journal lineage; a fake writer or UI-only state transition
is prohibited.

## 5. Workspace Orchestration Board

The native app opens to the Orchestration Board, not Home/System/Provider.

The left rail contains:

- `New Mission`;
- `My Missions`;
- `Teams`;
- `Needs You`;
- secondary access to Library, Runtime and Provider settings.

The main surface provides `Board`, `Topology`, `Timeline` and `Capacity`
views. The default Board uses the exact stable lifecycle lanes derived from
existing state:

1. `Proposed`
2. `Ready`
3. `Orchestrating`
4. `Review`
5. `Complete`

`Blocked`, `Retrying` and `Needs You` are not lifecycle lanes. They appear only
as Mission-card status, Attention filters, Timeline facts or Mission Room
inline decision requests.

Every Mission card shows only bounded, useful information:

- Mission ID, title/source and priority;
- one explicit semantic status;
- Team presence;
- plan/node progress derived from authoritative counts;
- current Node or Gate;
- last authoritative milestone;
- warning/attention marker when applicable.

Cards do not expose raw Runtime IDs, model internals, Grants, stream heads,
full digests or complete Evidence by default. A simple one-node Mission hides
the DAG miniature; a multi-node Mission shows dependencies and parallel-ready
state.

The Board must have real empty, partial, stale, unavailable and populated
states. Empty data never renders fabricated Missions or progress.

## 6. Team Pulse, Topology, Timeline and Capacity

The rebuildable TeamExecution Projection may be extended with copied plan
metadata already present in accepted Events:

- Node title;
- bound AgentInstance and RuntimeInstance IDs;
- role;
- dependencies;
- maximum attempts.

No new Event field is added.

Team Pulse resolves these references through the same GlobalReadView and shows:

- Main and SubAgent role;
- idle, ready, working, waiting, review, blocked or terminal state;
- current Mission/Node;
- current Attempt number;
- no capacity value beyond the authoritative Runtime capacity.

Topology renders dependency, parallel, waiting and Gate relationships. Timeline
uses existing source-ordered delivery records and authority labels. Capacity
shows observed Runtime capacity and active counts without inventing allocation.
Color is never the only state signal.

## 7. Mission Room

Opening a Mission replaces the Board center with a focused three-column
workspace:

- left: Mission list and Team presence;
- center: Mission outcome, plan, activity blocks and Composer;
- right: contextual Inspector.

The center supports dedicated content blocks for:

- authoritative milestones;
- tentative authorized frame output, clearly labelled tentative;
- tool/action records;
- files and Diff summaries when present in existing output;
- tests and verification;
- warnings, recovery and terminal state;
- Evidence references.

It does not fabricate streaming, file changes, tests or Evidence. If those
records do not exist, it renders a truthful empty state.

The Inspector tabs are:

- `Team`
- `Plan`
- `Changes`
- `Evidence`

Developer details are collapsed by default and may reveal only safe IDs and
digests already present in the read model.

Board -> Mission Room -> Board restores:

- selected Mission;
- Board lane/filter;
- scroll anchor;
- per-Mission Composer draft;
- selected Inspector tab;
- expanded developer-detail state;
- Inspector visibility.

## 8. Team Builder and Provider reachability

The accepted one-question Team Builder remains Candidate-only until explicit
`Save Team`.

Saving a Team and starting a Mission remain different actions. P2A-W2 must not
display `Start Work` unless an accepted authority path exists.

The attempt-007 defect is a mandatory regression:

- Provider status and `Manage` are reachable from the ordinary workbench;
- a non-nil setup snapshot does not hide Provider management;
- MiniMax `Test` is reachable without a terminal or IPC bypass;
- Codex OAuth remains delegated to the fixed Codex executable;
- MiniMax secrets remain only in masked input -> private IPC -> Broker ->
  Keychain;
- Provider/System remains a secondary surface, never the default entrance.

## 9. Loom Decision Sheet

All three templates use one native macOS Sheet host with custom Loom Graphite
content. A one-line system Alert is not accepted.

### 9.1 Authorization

Shows:

- requested action;
- Mission;
- requesting Team/Agent;
- target path/resource;
- command type;
- network and credential access;
- permission scope;
- Attempt or time boundary;
- expected Evidence;
- collapsed technical details.

Actions:

- `Not now`
- `Deny`
- `Edit scope`
- `Allow once`
- `Allow for this Mission`

Closing the sheet is `Not now`. A high-risk action cannot be approved by an
unmodified Return key.

### 9.2 Review Gate

Shows:

- change summary;
- tests and verification;
- Reviewer result;
- Evidence and Open Evidence;
- `Request changes`;
- `Accept result`.

`Accept result` is unavailable when terminal Run, accepted source Evidence,
required verifier Evidence or a current prepared acceptance command is
missing.

### 9.3 Recovery Decision

Shows:

- why the current Attempt stopped;
- retained output/Evidence;
- whether a fresh Attempt and generation will be created;
- Team/Provider/permission/budget changes;
- bounded retry, replan, degraded, blocked and stop semantics;
- `Stop Mission`;
- `Edit recovery`;
- `Start new attempt`.

There is no hidden retry. `Start new attempt` succeeds only through a prepared
deterministic RecoveryDecision and existing Work authority. The returned
Projection must show a new Attempt number and a fresh claim generation before
the UI treats recovery as confirmed.

## 10. Permission modes

The Composer exposes one bounded permission mode:

- `Plan only`: read and plan; no mutation, command or network.
- `Guided`: low-risk reads may proceed; mutation, command, network and scope
  expansion ask.
- `Delegated`: only within the exact confirmed Mission Grant; deletion,
  publish, send, payment, credential access and scope expansion always ask.

This selection is a proposal until existing policy/Grant authority confirms it.
`Delegated` is never unlimited authorization.

## 11. Loom Graphite visual system

Design reading:

> Native macOS technical-team orchestration workbench; Multica-first Board,
> Codex-first Mission Room, Claude/Codex-inspired decisions, Loom-owned
> authority semantics.

Frozen dials:

```text
DESIGN_VARIANCE  = 5
MOTION_INTENSITY = 3
VISUAL_DENSITY   = 7
```

Rules:

- neutral graphite foundations and one cobalt-blue product accent;
- semantic Light/Dark tokens with equal hierarchy;
- SF Pro system typography and SF Mono for Mission/Attempt IDs and digests;
- SF Symbols, never emoji UI icons;
- approximately 10-point Mission-card radius;
- minimal panel nesting and no default Settings-form appearance;
- 150–300 ms motivated state transitions;
- Reduce Motion removes nonessential movement without functional loss;
- minimum 44-point primary action targets;
- complete keyboard focus, visible focus and VoiceOver labels/order;
- ordinary text meets WCAG AA;
- destructive/high-risk actions use full names and explicit confirmation;
- drag operations have a keyboard alternative.

No third-party brand asset or pixel-identical copy is permitted.

## 12. TUI continuity

GUI and TUI use the same Mission facade and IPC data.

The primary TUI hierarchy is:

```text
Lanes | Missions | Mission Detail
```

Key behavior:

- `g b`: Board;
- `g t`: current Mission;
- `/`: Mission filter;
- `Enter`: open Mission;
- `a`: open applicable Approval;
- `Esc`: return and restore position;
- full keyboard access and visible selected/focus state.

The TUI does not draw a dense ASCII imitation of GUI cards and does not expose
CLI subcommands as the primary interaction. The same deterministic fixture
must map to the same Mission lane, Team/Node/Attempt semantics, attention state
and decision availability in both clients.

## 13. Exact owned files

Only the following existing product files may be modified:

### Go read facade, IPC and daemon

1. `internal/projection/global_read_view.go`
2. `internal/projection/global_read_view_test.go`
3. `internal/projection/team_execution.go`
4. `internal/projection/team_execution_test.go`
5. `internal/api/team_execution_stream.go`
6. `internal/api/team_execution_stream_test.go`
7. `internal/api/local_product_read.go`
8. `internal/api/local_product_read_test.go`
9. `cmd/loomd/product_daemon.go`
10. `cmd/loomd/product_daemon_test.go`
11. `internal/localipc/protocol.go`
12. `internal/localipc/protocol_test.go`
13. `internal/localipc/swift_contract_test.go`

### TUI

14. `internal/tui/model.go`
15. `internal/tui/model_test.go`
16. `internal/tui/program.go`
17. `internal/tui/program_test.go`

### Native app

18. `apps/macos/Sources/LoomLocalApp/LoomLocalApp.swift`
19. `apps/macos/Sources/LoomLocalAppContractProbe/main.swift`
20. `apps/macos/Sources/LoomLocalAppCore/InteractionContinuity.swift`
21. `apps/macos/Sources/LoomLocalAppCore/LocalIPCClient.swift`
22. `apps/macos/Sources/LoomLocalAppCore/LocalProductExperience.swift`
23. `apps/macos/Sources/LoomLocalAppCore/LocalProductModels.swift`
24. `apps/macos/Sources/LoomLocalAppCore/LocalProductStore.swift`
25. `apps/macos/Sources/LoomLocalAppUI/ContentView.swift`
26. `apps/macos/Tests/LoomLocalAppTests/InteractionContinuityTests.swift`
27. `apps/macos/Tests/LoomLocalAppTests/LocalIPCClientTests.swift`
28. `apps/macos/Tests/LoomLocalAppTests/LocalProductExperienceTests.swift`
29. `apps/macos/Tests/LoomLocalAppTests/LocalProductExperienceViewTests.swift`
30. `apps/macos/Tests/LoomLocalAppTests/LocalProductModelsTests.swift`
31. `apps/macos/Tests/LoomLocalAppTests/LocalProductStoreTests.swift`

### Current-state record

32. `docs/CURRENT.md`

The Candidate may create only:

1. `internal/api/local_product_mission.go`
2. `internal/api/local_product_mission_test.go`
3. `internal/api/local_product_decision.go`
4. `internal/api/local_product_decision_test.go`
5. `internal/app/local_product_decision.go`
6. `internal/app/local_product_decision_test.go`
7. `apps/macos/Sources/LoomLocalAppCore/MissionOrchestration.swift`
8. `apps/macos/Sources/LoomLocalAppCore/LocalProductDecisionModels.swift`
9. `apps/macos/Sources/LoomLocalAppUI/LoomGraphite.swift`
10. `apps/macos/Sources/LoomLocalAppUI/MissionWorkbench.swift`
11. `apps/macos/Tests/LoomLocalAppTests/MissionOrchestrationTests.swift`
12. `apps/macos/Tests/LoomLocalAppTests/LocalProductDecisionModelsTests.swift`
13. `apps/macos/Tests/LoomLocalAppTests/LoomGraphiteViewTests.swift`
14. evidence only under `.loom-evidence/phase2a/P2A-W2/`

No other file is owned. In particular, this contract does not own:

- `internal/journal/*`;
- `internal/rules/authority.go`;
- `internal/work/*`;
- `internal/authorization/*`;
- `internal/evidence/*`;
- Event migrations;
- Agent execution adapters;
- Scheduler implementation;
- Phase 1 evidence;
- installed app/LaunchAgent state;
- user-owned dirty or untracked files.

## 14. Mandatory RED

After fresh Contract Review PASS, tests must fail for missing behavior, not
syntax or fixture setup:

1. a real Journal/Projection fixture produces ordered Mission lanes and cards;
2. simple and multi-node Topology retain dependency and Agent bindings;
3. stale projection refresh preserves the previous Mission view;
4. missing terminal Run/Evidence/prepared command rejects completion with zero
   Event delta;
5. stale view and stale generation reject decision submission;
6. two concurrent submissions produce one authority winner and one conflict or
   exact idempotent result, never duplicate facts;
7. `Not now` and sheet dismissal write nothing;
8. `Deny` writes only the existing authoritative rejection path;
9. prepared recovery creates fresh Attempt and generation through existing
   authority;
10. the Swift strict decoder accepts the exact Go mission/decision wire and
    rejects unknown fields, null collections and identity drift;
11. the native default route is Board, with Board/Mission/Inspector continuity;
12. Provider `Manage` and MiniMax `Test` are reachable with non-nil setup;
13. TUI uses Mission terminology and `g b`/`g t` continuity;
14. GUI/TUI map the same fixture to identical lane and decision availability;
15. Light/Dark, compact/wide, keyboard, VoiceOver and Reduce Motion contract
    probes fail before implementation;
16. the Go IPC protocol rejects the frozen decision method before Repair
    implementation, then accepts only the exact reviewed method name while
    continuing to reject unknown `decision_*` names before daemon dispatch.

## 15. Deterministic verification matrix

Before Implementation Review:

- relevant focused Go tests;
- relevant focused Swift tests;
- `swift test --package-path apps/macos`;
- Swift Thread Sanitizer tests;
- Swift Release build;
- `go test ./...`;
- `go test -race ./...`;
- `go vet ./...`;
- `gofmt` and repository diff checks;
- TUI deterministic tests;
- real `LocalProductReadService -> Go IPC Server -> Swift Client` fixture;
- strict request/response, unknown-field and collection-wire tests;
- stale view/generation/Grant and missing Evidence tests;
- CAS concurrency and no-hidden-retry tests;
- Light and Dark native-window inspection;
- wide and compact native-window inspection;
- keyboard, visible focus, VoiceOver order and Reduce Motion inspection;
- secret/raw-Grant/hidden-reasoning negative scans;
- socket/lock atomic cleanup transaction tests;
- exact owned-file and excluded-dirt checks.

The Controller must record:

- mandatory RED evidence;
- deterministic GREEN evidence;
- source lock and Candidate manifest;
- GUI/TUI semantic-fixture comparison;
- wide/compact Light/Dark screenshots;
- Authorization, Review and Recovery sheet screenshots;
- TUI screenshot;
- accessibility and visual audit.

## 16. Independent reviews and commit gate

Required in order:

1. fresh independent Contract Review `PASS`;
2. mandatory RED;
3. scoped implementation and full deterministic verification;
4. fresh independent Implementation Review `PASS`;
5. fresh independent Visual Review `PASS`;
6. one atomic Candidate commit containing only owned files;
7. exact post-commit preflight.

Implementation and Visual Review must be performed by fresh read-only
Reviewers. They may not modify the Candidate. Any blocking finding is repaired
inside this same contract and re-reviewed. It does not create a new Amendment
or W4.

Before both Reviews pass, visual evidence may use only a deterministic,
non-installed AppKit `NSWindow` fixture with in-memory decoded wire fixtures,
no daemon, product socket, Provider, Keychain or authority mutation. It must
exercise the real product views and window behavior, not a static image or
web mock.

No production native-app launch or installation, daemon, product socket,
Provider call, Keychain mutation, prepared authority command or live canary is
permitted before both Reviews pass and the exact Candidate is committed.

## 17. Single controlled native-window canary

After both Reviews PASS, the already granted Product Owner authorization allows
one fresh isolated native-window lineage. It must:

1. create a new private `0700` attempt root and `0600` SQLite copy;
2. use an independent manifest and exact committed binaries;
3. start exactly one controlled product daemon;
4. open the Workspace Orchestration Board;
5. create or view a real Journal-backed Mission fixture;
6. inspect Team Pulse and Topology;
7. open Mission Room, modify its local draft/Inspector state, return to Board
   and restore that state;
8. reach Provider Manage and perform the contract's bounded Provider check
   without terminal/IPC UI bypass;
9. open Authorization, prove `Not now`/`Deny` write no execution fact, then
   prove one legal prepared authorization produces the existing authoritative
   confirmation;
10. open Review Gate and prove missing Evidence cannot complete;
11. open Recovery Decision and prove one prepared recovery creates a fresh
    Attempt and generation;
12. prove the GUI and TUI show the same Mission/Team/Node/Attempt state;
13. prove no secret, raw Grant or hidden reasoning is visible or persisted;
14. close the native app and daemon normally;
15. prove exact socket/lock cleanup, no orphan process, empty isolation,
    SQLite integrity, source immutability and bounded Event delta.

There is no retry, alternate executable, direct SQLite edit, terminal-as-UI
bypass, manual socket/lock deletion or second canary. Any failure consumes the
lineage and stops `HUMAN_REQUIRED`.

## 18. Exit

P2A-W2 and this Extra Goal are accepted only when:

- Mission/Team/Plan/Node/Attempt/Run/Evidence terminology is frozen;
- Orchestration Board is the default product entry;
- Mission Card, Team Pulse, Topology, Timeline and Capacity use real
  Projection data;
- Board, Mission Room and Inspector round-trip continuity passes;
- all three Decision Sheet templates and their fail-closed command paths pass;
- the Provider reachability regression passes;
- no authority boundary is widened;
- GUI and TUI share one mental model and fixture semantics;
- Loom Graphite passes Light/Dark, compact/wide and independent Visual Review;
- keyboard, VoiceOver, Reduce Motion and 44-point targets pass;
- full deterministic verification passes;
- Contract, Implementation and Visual Reviews pass;
- the atomic Candidate commit exists;
- the single controlled canary and fresh Result-Evidence Review pass;
- excluded dirt remains unmodified and unstaged.

Only then may P2A-W3 governance resume. P2A-W4 does not exist.
