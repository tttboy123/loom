# P2A-W2 Interaction Continuity and Exit Reopen Contract

**Date**: 2026-07-30
**Status**: FROZEN — Contract Repair 1 Re-review PASS
**Authority**: explicit Product Owner authorization
**Parent**: Phase 2A Local Product Experience Exit Contract
**Reopens**: the complete `P2A-W2 Team Builder and Provider Onboarding` exit
**Risk**: STRICT — ordinary-user interaction, local IPC ownership, OS Secret
Store, Provider observation, TeamDefinition authority and native live evidence

## 1. Authorization and precedence

The Product Owner explicitly authorized one complete P2A-W2 Exit Contract
reopen and one `Phase 2A Interaction Continuity Contract`.

This document is the only active P2A-W2 closure contract. It replaces the
earlier no-retry stop for attempt 006 and consolidates the remaining UI,
product-socket ownership and live-proof work into one vertical Candidate.
Historical contracts, failed canaries and reviews remain immutable evidence.
They are not rewritten or reclassified.

This reopen:

- creates no `P2A-W2a`, `P2A-W2b` or `P2A-W4`;
- permits no new single-point Amendment after this contract freezes;
- keeps `P2A-W3 Controlled Execution Experience` locked until the complete W2
  exit below passes;
- does not reinterpret attempt 006, whose reviewed verdict remains
  `FAIL — PREEXISTING_PRODUCT_LOCK_BLOCKED_LOCAL_IPC`;
- does not change the Event Journal, StateWriter, Projection, policy, Grant,
  Evidence, Scheduler, Supervisor, Runtime execution or Provider authority;
- permits exactly one post-Implementation-Review vertical live canary.

If the frozen owned-file boundary or an accepted authority invariant is
insufficient, work stops `HUMAN_REQUIRED`. It is not split into another narrow
Amendment.

## 2. Baseline and preserved state

- Repository and Git top-level:
  `/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild`
- Branch: `codex/loom-platform-slice2`
- Reopen baseline commit:
  `0546509d2413cff49fdf4a9c795d52b83b84cfa2`
- Accepted W2 credential-helper Candidate:
  `47d5f56358d310fd561d2e5a089c79ac543a2cf8`
- Attempt-006 result digest:
  `adb5b72e527465dd0eb482659afabd3b01123c918b72b1db715000263be64415`
- Attempt-006 result-review digest:
  `24566522c022f04ec770a1d4d084ed7c465731b347b2d9049aac44548939c37c`
- Frozen source database:
  `/Users/lune/Library/Application Support/Loom/phase2a-w2-live-20260730-002/state/loom.db`
- Frozen source database digest:
  `b0312739ba7b24961b9e8a698e4cbf436e9cace8758a1d2a058e050aec475e22`
- Existing product lock observed at freeze:
  `/Users/lune/Library/Application Support/Loom/run/loomd.sock.lock`,
  regular `0600`, uid `501`, zero bytes, inode `74706302`, born
  `2026-07-30T04:02:27+0800`, with the sibling product socket absent.
- The separate resident Runtime observer LaunchAgent is not stopped,
  signalled, reconfigured, replaced or claimed as this Candidate.
- All pre-existing modified and untracked paths remain user-owned and excluded.

No product socket or lock is removed while drafting or reviewing this contract.
The exact stale pair is consumed only by the reviewed product transaction
during the single live canary.

## 3. Product and design reading

Reading this as a structural redesign of a native macOS AI workbench for users
trained by Claude, Codex, Multica and Grok, with a calm, content-first,
developer-grade interaction language and native SwiftUI/macOS behavior.

The frozen design dials are:

```text
DESIGN_VARIANCE  = 4
MOTION_INTENSITY = 3
VISUAL_DENSITY   = 6
```

The current product audit records:

- **Preserve**: native macOS window and controls, system typography and icons,
  shared Go daemon IPC, inline safe connection states, 44-point action targets,
  keyboard refresh, light/dark system adaptation and strict Swift decoding.
- **Retire**: `Home / Work / Teams / Inbox / System` as the primary information
  architecture, the Home card wall, the separate modal-first Team Builder,
  setup-first Provider directory, repeated panel chrome, and internal
  authority terminology in the ordinary-user path.
- **Structural defect**: the user must currently learn Loom's internal object
  model before stating a task. Provider setup, Team building and timeline
  observation are separate destinations rather than one continuous task.

The local design-system search suggested AI-native minimal chrome and fast
streaming feedback. Its newsletter layout, web font and raw web palette are
explicitly non-authoritative for this native app.

## 4. One vertical W2 capability

The Candidate replaces the dashboard-first path with one task-first
conversation workspace:

```text
task or recent work
-> intent in the central conversation
-> just-in-time Provider, model, Team and permission choices
-> one-question-at-a-time bounded Team Builder
-> human-readable preflight
-> explicit Team confirmation
-> authoritative saved result in the same conversation
-> restart/reconnect reconstruction
```

W2 remains pre-execution. The primary action saves a TeamDefinition only. It
does not create a TeamInstance, AgentInstance, WorkItem, Run, Grant, Evidence,
dispatch or execution fact. The center surface may present existing
authoritative historical Runs and timeline records, but it may not start,
retry, approve or accept execution. W3 will add those commands through a
separate reviewed contract after W2 exits.

An unconfirmed task conversation is replaceable presentation state. It is not
an Event, WorkItem, TeamDefinition or second authority. Service or app restart
may discard it. Confirmed Teams and historical activity reconstruct only from
the daemon Projection and Journal-backed application services.

## 5. Native three-column workspace

At ordinary desktop width, the native app presents:

### 5.1 Left: tasks and history

- one prominent `New task` action;
- a `Local workspace` group, without inventing a new Project authority;
- recent task/history rows derived from copied authoritative Teams, Runs and
  attention records;
- one selected row and compact semantic status;
- search/filter available without replacing the current task;
- Provider/System settings reachable from a standard Settings command, not a
  top-level product destination.

The left column does not show raw Team IDs, Runtime IDs, stream heads, cursors,
database paths or service-manager controls.

### 5.2 Center: conversation and progress

- launch resumes the most recent selected task when one exists, otherwise a
  blank composer;
- the composer asks what the user wants to accomplish;
- a new task maps the user's bounded intent into the existing Candidate-only
  Team Builder purpose flow without a model call;
- Builder questions, answers, validation, safe Provider status and final
  confirmation appear in one chronological thread;
- exactly one Builder question is actionable at a time;
- conflict, unavailable, blocked and recovery states appear inline at the
  action that caused them;
- the final W2 result leads with the saved Team's human name and what is ready,
  not with an Event type or internal identifier;
- historical authoritative milestones and tentative presentation messages are
  visually and semantically distinct.

The W2 transcript is not model chat. It must not fabricate assistant text,
streaming tokens, hidden reasoning or execution progress.

### 5.3 Right: contextual inspector

The inspector is visible at ordinary desktop width and collapsible on smaller
windows. It contains contextual `Team`, `Context`, `Changes` and `Evidence`
surfaces:

- `Team` shows human names, roles, selected Provider/model, compatibility,
  permissions and bounded cost preview;
- `Context` shows only bounded resource pointers already present in the W2
  catalog, never resource bodies;
- `Changes` and `Evidence` are read-only historical views in W2 and clearly
  state when execution has not started;
- exact digests, internal IDs and transport detail are hidden by default and
  may appear only in an explicit `Developer details` disclosure.

Changing tasks preserves each task's in-memory composer draft, selected
inspector tab, expanded disclosures and thread position for the current app
session. Switching among compatible role options shown with their bound
Provider/model labels requires no more than two user actions and does not clear
the task intent or Builder answers.

### 5.4 Adaptive behavior

- at wide widths all three columns are independently resizable within bounded
  minimums;
- at medium widths the inspector becomes a toolbar-controlled side panel;
- at the minimum supported window size, tasks and inspector use native
  navigation/disclosure while the conversation remains primary;
- no horizontal clipping, nested ambiguous scrolling or inaccessible hidden
  action is accepted.

## 6. Text-mode continuity

Bubble Tea remains the supported text-mode product for SSH, recovery and
terminal-oriented users. It adopts the same task-first vocabulary and
interaction order:

- the initial screen is `Tasks`, not `Home`;
- recent work and a new-task action are primary;
- the conversation/Builder occupies the main pane;
- Team/Provider/context detail is secondary and toggled explicitly;
- narrow terminals collapse secondary detail without losing the current
  question, input or error;
- task switching preserves the current in-memory draft and selection;
- no screen exposes ordinary users to raw IDs or authority jargon.

The TUI is not the native app's transport and neither client parses the other's
rendered output. Both continue to call the same daemon IPC and application
services.

## 7. Provider and Team Builder continuity

- Provider connection is just-in-time beside the composer and in Settings; it
  is not a prerequisite dashboard.
- Codex continues to use native-auth status and delegated login. Loom never
  copies Codex OAuth material.
- MiniMax continues to use the process-owned Credential Broker and macOS
  Keychain. Secret entry is masked, cleared after the request and never placed
  in app snapshots, task state, transcript, clipboard, logs or evidence.
- Provider status and failure copy uses only reviewed closed reason mappings.
- Team source, Runtime, model, roles, permissions, resources, concurrency and
  budget continue through the accepted W2 Builder and catalog validation.
- Provider/model controls are a presentation of compatible existing role
  options. Selecting one submits the role-option ID through the existing
  `builder_edit` main/subagent role fields and shows its already-bound Runtime
  profile, model and auth mode. The client does not invent an independent model
  selection, protocol field, catalog or authority.
- The UI may use human display names as selections, but every command binds the
  exact current IDs, versions and digests supplied by the service.
- The preflight summary is one human-readable review surface. It includes
  Provider/model, roles, permissions, compatibility and maximum budget, then
  one explicit `Save team` action.
- Confirmation retains the existing exact revision/view/binding-digest CAS and
  appends only one `TeamDefinitionSaved` fact.
- Client reconnect never automatically repeats a mutating command.

## 8. Interaction and presentation acceptance

The native and text clients must satisfy:

1. one primary action per state;
2. full keyboard navigation in visual order and visible focus;
3. native controls and system icons with minimum 44-by-44-point hit regions;
4. VoiceOver labels, hints and selected/expanded/disabled traits;
5. Dynamic Type or system text scaling without loss of the actionable question;
6. semantic light/dark colors with WCAG AA text contrast;
7. only motivated state feedback, normally 150-250 ms, and no required motion;
8. Reduce Motion produces an instant, stable equivalent;
9. loading, empty, offline, stale, partial, conflict, denied, invalid and fatal
   states have local recovery guidance;
10. destructive/revoke actions remain visually separated and confirmed;
11. task selection, drafts and inspector state survive task switches in the
    same app session;
12. refresh/reconnect preserves the last good copied view until a valid
    replacement arrives.

Ordinary-user visible copy in the primary flow must not contain:

```text
Candidate
TeamDefinition
TeamInstance
AgentInstance
Runtime instance ID
stream head
stream revision
catalog digest
binding digest
credential reference
source-ordered
Journal authority
Projection authority
```

Those terms may exist in tests, code symbols, protocols, evidence and an
explicit Developer details disclosure. They may not be required to complete the
journey.

## 9. Product socket and lock acquisition transaction

The default product socket and its sibling lock are one ownership transaction.
The Candidate must preserve fail-closed behavior while safely reclaiming an
abandoned exact pair.

### 9.1 New owner

For an absent lock:

1. validate the absolute clean socket path and private, owned, non-symlink
   `0700` parent;
2. create the exact sibling lock with exclusive no-follow/no-replace semantics;
3. require a regular, single-link, zero-byte, owned `0600` file;
4. acquire a non-blocking exclusive kernel advisory lock and hold it for the
   complete listener lifetime;
5. revalidate the path identity against the opened descriptor before creating
   the listener.

### 9.2 Existing lock

For an existing lock:

1. open without following symlinks;
2. require the exact regular, single-link, zero-byte, owned `0600` shape;
3. acquire a non-blocking exclusive kernel advisory lock; contention returns a
   closed `busy/local_ipc` result and changes nothing;
4. revalidate device, inode, file kind, owner, mode, link count and size through
   both descriptor and path;
5. inspect the sibling socket:
   - a successful bounded local dial proves a live owner and rejects cleanup;
   - only a private owned `0600` Unix socket with exact stable identity and
     `ECONNREFUSED` or `ENOENT` may be reclaimed;
   - an absent socket is eligible;
   - symlink, regular file, wrong owner/mode, identity drift, timeout,
     permission error or ambiguous dial failure rejects cleanup;
6. remove only the exact stale socket identity, when present;
7. remove only the exact lock identity while retaining the advisory lock on its
   opened inode;
8. compete once to create and kernel-lock a fresh exact lock. A concurrent
   winner causes a closed conflict and no listener;
9. revalidate the fresh lock immediately before and after listener creation.

This transaction uses no `lsof`, PID-name guess, timestamp heuristic, ambient
path, shell command or blind `os.Remove`.

### 9.3 Shutdown

Normal close:

1. stops acceptance and cancels/joins active handlers;
2. closes the listener;
3. removes only the stored exact socket identity;
4. removes only the stored exact lock identity;
5. releases the kernel advisory lock and closes its descriptor;
6. returns any cleanup failure without deleting a replacement.

Concurrent startup, stale-pair reclaim, cancellation and close must produce at
most one listener and must never remove a live, foreign or replacement path.

## 10. Exact owned files

Only these product files may change:

```text
internal/localipc/socket.go
internal/localipc/socket_test.go
internal/localipc/server.go
internal/localipc/server_test.go
cmd/loomd/product_daemon_test.go
internal/tui/model.go
internal/tui/model_test.go
internal/tui/program.go
internal/tui/program_test.go
apps/macos/Sources/LoomLocalAppCore/LocalProductExperience.swift
apps/macos/Sources/LoomLocalAppCore/LocalProductStore.swift
apps/macos/Sources/LoomLocalAppCore/InteractionContinuity.swift
apps/macos/Sources/LoomLocalAppUI/ContentView.swift
apps/macos/Tests/LoomLocalAppTests/LocalProductExperienceTests.swift
apps/macos/Tests/LoomLocalAppTests/LocalProductExperienceViewTests.swift
apps/macos/Tests/LoomLocalAppTests/LocalProductStoreTests.swift
apps/macos/Tests/LoomLocalAppTests/InteractionContinuityTests.swift
docs/CURRENT.md
```

Evidence may change only under:

```text
.loom-evidence/phase2a/P2A-W2/
```

An owned file may remain unchanged. No dependency, protocol method, IPC JSON
schema, Journal schema/Event payload, StateWriter, Projection, Credential
Broker, Provider verifier, Team domain, Runtime adapter, execution, Grant,
Evidence, Scheduler, daemon LaunchAgent, installer or release surface is owned.

### 10.1 Contract Repair 1

Fresh independent Contract Review 1 returned `FAIL` with one P1 and one P2:

- the shutdown contract requires exact lock-path removal before release of the
  advisory lock, but the original ownership list omitted the current close
  implementation in `internal/localipc/server.go`;
- Mandatory RED used `unowned` ambiguously even though wrong-owner locks must
  always remain fail-closed.

Repair 1 adds only `internal/localipc/server.go` to the exact boundary, retains
the stronger shutdown ordering, and requires a focused concurrent-contender
test proving close cannot remove a live or replacement lock after advisory-lock
release. It also names the attempt-006-shaped input an `abandoned owned` lock.
Wrong-owner paths remain explicit rejection cases.

No interaction, authority, protocol, credential, Provider, live or exit
requirement changes.

## 11. Mandatory RED

Before product implementation, deterministic tests must prove the current
Candidate fails the frozen requirements:

1. native launch selects the dashboard `Home` instead of a task conversation;
2. current primary navigation exposes `Home/Work/Teams/Inbox/System`;
3. Team Builder is modal/setup-first instead of inline and task-first;
4. switching work cannot preserve independent composer/inspector/thread state;
5. Provider/model choices are not available just in time near the composer;
6. primary copy exposes at least the audited internal terms;
7. the TUI starts at `Home` and exposes Runtime/Team object navigation before a
   task;
8. an abandoned owned valid zero-byte product lock with an absent socket blocks
   startup;
9. active advisory lock, live socket, foreign/symlink/regular targets, identity
   replacement and concurrent reclaim are rejected without deletion;
10. an integrated product-daemon fixture cannot start from the exact
    attempt-006 stale-lock shape and cleanly remove its owned pair.

RED uses only temporary private directories, fixture IPC, fake setup services
and copied model data. It does not remove the real product lock, access
Keychain, contact a Provider, run installed Codex/Pi/llama, open the installed
app or signal a resident service.

## 12. Deterministic GREEN and visual proof

Required proof before Implementation Review:

- focused socket/server/product-daemon tests, including at least 50 concurrent
  startup/reclaim repetitions and race execution;
- a focused close-order/concurrent-contender test proving exact lock removal
  completes while the owner still holds the advisory lock and never removes a
  live or replacement path after release;
- exact active-owner, legacy-unlocked-lock, stale-socket, missing-socket,
  symlink, hard-link, wrong-owner where supported, wrong-mode, nonzero-size,
  identity-drift, replacement-preservation and cancellation matrices;
- native Store tests for per-task draft, selection, inspector, refresh and
  reconnect continuity;
- native presentation tests for wide, medium and minimum supported layouts;
- primary-flow copy allowlist and internal-term negative test;
- keyboard, focus, accessibility, Reduce Motion, Dynamic Type and light/dark
  fixtures;
- Bubble Tea wide/narrow task-first flow, task switch, question, inline error
  and state-preservation fixtures;
- real `LocalProductReadService -> Go IPC Server -> Swift Client` fixture
  remains strict and passes;
- `swift test` in debug and thread-sanitizer configurations;
- Swift release build;
- focused Go tests and race tests;
- serial complete `go test -p 1 ./... -count=1`;
- serial complete `go test -race -p 1 ./... -count=1`;
- `go vet ./...`;
- `go mod tidy -diff` and `go mod verify`;
- `gofmt`, `swift-format` where already configured, and `git diff --check`;
- secret-negative and no-new-authority scans;
- exact owned-file and excluded-dirt audit.

A deterministic native preview must be captured under the W2 evidence
directory at wide and compact widths. The visual audit must compare it against
the frozen three-column hierarchy and copy rules. Fixture screenshots contain
no secret, raw credential reference, private path or internal identifier.

## 13. Gate and commit sequence

The mandatory order is:

```text
frozen contract
-> fresh independent Contract Review PASS
-> mandatory RED
-> implementation
-> deterministic GREEN and visual audit
-> fresh independent Implementation Review PASS
-> one atomic Candidate commit
-> exact live preflight
-> one complete vertical live canary
-> fresh independent Result-Evidence Review PASS
-> W2 Exit acceptance update
```

The Contract Reviewer and Implementation Reviewer are read-only and independent
of implementation. A review failure is repaired only inside this same complete
contract and re-reviewed; it is not converted into a new Amendment or WorkItem.

The atomic Candidate commit includes only reviewed owned product files, W2
evidence and `docs/CURRENT.md`. Excluded user dirt remains untouched.

## 14. One complete vertical live canary

Only deterministic GREEN, fresh Implementation Review `PASS`, the atomic
Candidate commit and an exact preflight unlock one lineage:

```text
p2a-w2-live-20260730-007
```

There is one daemon start and no retry. It uses:

- a fresh private `0700` attempt root and regular `0600` database clone;
- the exact frozen attempt-002 source database and digest;
- exact post-Review daemon and native-app artifacts;
- the already reviewed Pi, Node, Codex, llama-server and GGUF identities;
- the default product socket path;
- the exact pre-existing lock identity recorded in section 2, with no
  Controller pre-deletion;
- the existing credential only through the Broker and OS Secret Store;
- no resident Runtime-observer signal or configuration mutation.

The native journey, driven through the ordinary product window, must prove:

1. the Candidate daemon atomically reclaims the exact abandoned lock and
   becomes the only product listener;
2. the app opens directly into the task-first conversation workspace;
3. the user creates one bounded task intent without entering an internal ID;
4. Codex appears through native-auth status without copying OAuth material;
5. exactly one MiniMax `Test` produces exactly one new closed
   `ProviderCredentialVerified` fact;
6. the user selects Provider/model/Team choices in context and completes the
   one-question-at-a-time flow;
7. one human-readable preflight and one explicit save produce exactly one
   `TeamDefinitionSaved` fact;
8. no TeamInstance, AgentInstance, WorkItem, Run, Grant, Evidence, dispatch or
   execution fact is created;
9. switching away and back preserves the in-session draft/inspector/thread
   state before confirmation;
10. closing and reopening the native app against the same daemon reconstructs
    Provider, Runtime, saved Team and historical thread context from the
    authoritative read view, while no unconfirmed client draft is claimed
    durable;
11. normal `SIGINT` joins the daemon and helpers without `SIGKILL`, removes only
    the owned socket/lock pair, leaves no attempt/Pi/llama/app process, and
    leaves isolation empty;
12. final SQLite integrity, exact Event delta, artifact identities,
    permissions, source immutability and secret-negative checks pass.

The canary records one wide three-column screenshot and one compact adaptive
screenshot after secret entry is no longer visible. It records no hidden
reasoning, secret, raw Provider body, OAuth material or private credential
reference.

Any failure consumes the canary and stops `HUMAN_REQUIRED`. There is no second
canary, argument change, alternate executable, manual lock deletion or
single-point follow-up.

## 15. Exit

P2A-W2 is accepted only when:

- every contract requirement is implemented inside the exact boundary;
- deterministic and race matrices pass;
- native visual audit and TUI continuity fixtures pass;
- Contract and Implementation Reviews are fresh independent `PASS`;
- the atomic Candidate commit exists;
- the single vertical canary passes in full;
- fresh independent Result-Evidence Review returns `PASS`;
- an ordinary user completes the W2 journey without terminal use or internal
  identifiers;
- no secret leak, duplicate authority, hidden retry, execution fact, active
  socket/lock deletion or excluded-dirt mutation occurs.

Only then may the separate `P2A-W3 Controlled Execution Experience` contract be
frozen. There is no P2A-W4.
