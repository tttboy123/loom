# Phase 2A Local Product Experience Exit Contract

**Date**: 2026-07-28
**Status**: FROZEN — fresh independent Contract Review PASS
**Authority**: Product Owner authorization and Goal

## 1. Purpose

Phase 2A converts the accepted Phase 1 execution kernel into a local product
that an ordinary user can operate through a TUI without entering Team IDs,
SQLite paths, cursors, `launchctl` commands, or Provider environment variables.

This is a vertical product slice, not a UI wrapper over CLI commands. TUI, CLI,
and the local daemon API must reuse the same typed application services and the
same policy, Grant, StateWriter, Projection, and Event Journal authority.

This contract permits exactly three WorkItems:

1. `P2A-W1 Local App Shell and Read Experience`
2. `P2A-W2 Team Builder and Provider Onboarding`
3. `P2A-W3 Controlled Execution Experience`

There is no P2A-W4. A screen, button, adapter, writer, coordinator, API handler,
or command forwarder is not an independently governable Phase 2A WorkItem.

## 2. Baseline and preserved state

- Repository:
  `/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild`
- Baseline branch: `codex/loom-platform-slice2`
- Baseline commit: `c30a9bc272f8ef1fdd809f80691cf3c6d04e6617`
- Phase 1 status: `COMPLETE`, with final user sign-off on 2026-07-28.
- Existing accepted decisions: ADR-0001 through ADR-0010.
- Candidate decision: ADR-0011.
- The existing user-level Runtime observer LaunchAgent is running and is not
  stopped, replaced, or reconfigured by contract drafting.
- All pre-existing modified and untracked paths are user-owned and excluded
  unless a later reviewed WorkItem contract names an exact overlapping file.
  No excluded path may be silently staged, reformatted, deleted, or absorbed.

The current resident observer is evidence of a running local service, not proof
that the Phase 2A IPC, TUI, provider onboarding, or execution experience exists.

## 3. Authority and dependency invariants

The accepted dependency direction is:

```text
TUI / CLI
    │
    ▼
bounded versioned local daemon API
    │
    ▼
typed application services
    │
    ├── policy / approval / Grant
    ├── Team / WorkPackage / execution coordination
    ├── StateWriter / stream-head CAS
    ├── rebuildable Projection / GlobalReadView
    └── append-only Event Journal
```

All three WorkItems must preserve these rules:

1. The Event Journal remains the only durable state authority.
2. Projections, the GlobalReadView, the TUI model, cursor caches, and local
   subscriptions are replaceable read state.
3. The TUI never parses CLI text, shells out to Loom commands as its protocol,
   accesses SQLite directly, or owns a StateWriter.
4. The daemon API never invents business state. It validates bounded transport
   input and calls typed application services.
5. `AppendBatchIfStreamHeads` and already-accepted authoritative writers remain
   the only applicable write boundaries.
6. Scheduler dispatch remains at-least-once with idempotent CAS,
   lease/generation fencing, and explicit conflict. No client hides an infinite
   retry.
7. Supervisor, AgentGrant, BoundRunStream, Evidence, Verifier, and terminal
   aggregation boundaries remain unchanged unless an exact reviewed amendment
   reopens them.
8. An executor may submit `ready_for_review`; it cannot mark its own WorkItem
   `done`.
9. Conversation and draft generation remain non-authoritative. A Team Draft
   requires explicit user confirmation before TeamInstance or Run creation.
10. A resident daemon does not grant standing execution authority.

## 4. Product and platform assumptions

1. Bubble Tea is the Phase 2A TUI framework named by `TECH-PLAN.md`. Its
   `Model`, `Update`, `View`, commands, external messages, and context
   cancellation are used as presentation mechanics only.
2. The controlled live delivery platform is the current Apple Silicon macOS
   user account. Pure models, application services, and protocol types remain
   portable; macOS Keychain and LaunchAgent adapters are platform-specific.
3. The daemon local API uses a private Unix-domain socket under a user-owned
   `0700` parent. Socket permissions, peer access, protocol version, message
   bounds, deadlines, cancellation, and structured error codes are frozen in
   P2A-W1 before implementation.
4. No public TCP port, browser server, remote callback, or cloud synchronization
   is introduced.
5. Existing Phase 1 data must remain readable. Destructive migrations are not
   authorized by this contract.
6. Provider live canaries may incur only the explicitly authorized, bounded
   requests frozen in their manifests. No background or autonomous Provider
   traffic is allowed.
7. The API may expose multiple presentation views, but not a second domain
   model or second queue.

## 5. Exit capabilities

Phase 2A is complete only when every row is `DONE`.

| ID | Required capability | Exit state |
|---|---|---|
| PX-01 | Installing and launching the local product opens the TUI and reaches the existing resident daemon without terminal configuration. | MISSING |
| PX-02 | Home shows truthful daemon, Journal, projection, Runtime, Provider, Team, Run, Attention, and recovery status without direct database reads. | MISSING |
| PX-03 | Runtime, Teams, Runs, History, Evidence, Compare, and Attention are navigable, bounded, and rebuild from Journal/Projection state. | MISSING |
| PX-04 | Timeline accepts user-visible selection rather than typed Team IDs and closes the current real-canary missing-Team projection incompatibility without weakening Team scope validation. | MISSING |
| PX-05 | Cursor reconnect and slow-consumer handling preserve warning, retry, degraded, blocked, human-required, approval, and terminal milestones; tentative text may be coalesced with an explicit gap. | MISSING |
| PX-06 | A once-per-question builder creates and edits a Team Draft from blank, template, or saved input; no TeamInstance or Run exists before explicit confirmation. | MISSING |
| PX-07 | Saved teams load with exact Agent, model, Runtime, Skill revision, permission, compatibility, and cost previews; conflicts fail closed. | MISSING |
| PX-08 | Codex OAuth is displayed and selected truthfully as `native_auth`, without copying OAuth material into Loom. | MISSING |
| PX-09 | MiniMax onboarding stores a secret only through OS Secret Store/Credential Broker, displays `brokered`, supports revoke/replace, and never persists or renders the secret. | MISSING |
| PX-10 | Pi local execution remains an isolated Runtime path with exact metadata, capability, model, and compatibility disclosure. | MISSING |
| PX-11 | A user creates a WorkPackage, confirms a Team, and starts one controlled DAG execution without internal identifiers or Provider environment variables. | MISSING |
| PX-12 | The execution view shows node/attempt/Run lineage, authorized tentative output, approval, retry, degraded, blocked, human-required, Evidence, and canonical terminal result. | MISSING |
| PX-13 | Pause, cancel, restart, bounded recovery, stale-generation rejection, and restart/reconnect are explicit and do not duplicate Run, Evidence, dispatch, or side effects. | MISSING |
| PX-14 | Run History and Compare explain inputs, Runtime/model/Skill revisions, authorization mode, usage/cost when available, Evidence, terminal state, and failure/recovery reasons. | MISSING |
| PX-15 | Attention contains only actionable approval, blocked, human-required, retry-exhausted, Runtime-offline, and verification-failed items and remains a projection. | MISSING |
| PX-16 | User-level packaging manages install/status/start/stop/restart/recovery without granting autonomy; operational CLI remains available. | MISSING |
| PX-17 | Codex OAuth, MiniMax, and Pi each pass one fresh isolated controlled live canary through the product path. | MISSING |
| PX-18 | A complete no-terminal user journey and fresh independent whole-Phase2A review pass, followed by explicit user sign-off. | MISSING |

## 6. WorkItem boundaries

### P2A-W1 Local App Shell and Read Experience

W1 must close one runnable read-oriented product, not a collection of UI
stubs. Its child contract freezes the exact owned files and protocol schema
before RED.

It must deliver together:

- a Bubble Tea app shell with accessible keyboard navigation, focus, resize,
  empty, loading, partial, stale, conflict, offline, and fatal states;
- a bounded, versioned local daemon IPC transport and typed client that call
  application read services rather than CLI or SQLite;
- Home, Runtime, Teams, Runs, History, Evidence, Compare, and Attention read
  experiences;
- Team-bound timeline selection that repairs the real Phase 1 canary
  incompatibility by projecting or reconstructing the missing accepted Team
  relation, never by skipping scope checks;
- canonical cursor reconnect, cancellation, slow-consumer gap handling, and
  daemon restart recovery;
- user-level launch/install/exit behavior sufficient to start and close the
  read product without affecting execution authority;
- focused model/protocol/application tests and a local fixture E2E proving the
  TUI uses no terminal command or direct database access.

W1 does not onboard Provider secrets, create/confirm Team drafts, or execute a
WorkPackage.

### P2A-W2 Team Builder and Provider Onboarding

W2 must close the complete pre-execution user journey.

It must deliver together:

- conversational once-per-question Team Draft creation from blank, a saved
  team, or a versioned template input;
- user-visible edit, validation, save, load, archive/restore where already
  supported, and explicit confirmation;
- exact Agent, Runtime, model, Skill revision/digest, permission, resource,
  compatibility, and estimated-cost preview;
- proof that generation and edits remain Draft/Candidate-only until explicit
  confirmation and cannot create a TeamInstance, Run, Grant, or execution fact;
- Runtime capability matrix and truthful authentication-mode presentation;
- Codex OAuth native-auth discovery/selection without token extraction;
- MiniMax Secret Store and Credential Broker onboarding, test, replace, revoke,
  unavailable, denied, and redacted-error flows;
- Pi local Runtime selection with exact capability/model/metadata disclosure;
- a fixture E2E for builder/confirmation and separate secret-negative tests.

W2 may add the minimum Credential Broker and OS Secret Store implementation
required by ADR-0004. It must not store the previously pasted chat secret or
read credentials from conversation history. The live MiniMax canary requires a
fresh user-provided or already securely stored credential at its frozen live
gate.

W2 does not dispatch a production Run.

### P2A-W3 Controlled Execution Experience

W3 must close the complete controlled execution product journey.

It must deliver together:

- WorkPackage creation and selection of a confirmed Team;
- explicit preflight, authority, compatibility, cost/budget, and side-effect
  preview before start;
- DAG, logical node, Attempt, Run, generation, Runtime, Grant status, and
  capacity views derived from accepted execution state;
- authorized tentative streaming marked as tentative, with no raw Grant,
  credential, hidden reasoning, or unaccepted terminal claim;
- approval, rejection, pause, cancel, retry, fallback/degraded, blocked,
  human-required, restart, and recovery controls that invoke existing typed
  policies rather than UI-local policy;
- source and independent Verifier Evidence, canonical terminal aggregation,
  History, Compare, Attention, and accepted final result;
- user-level resident-daemon lifecycle wiring needed for reconnect and recovery;
- exact-once and stale-generation/CAS conflict evidence under concurrent
  callers;
- fresh isolated controlled Codex OAuth, MiniMax, and Pi live canaries and a
  full no-terminal product walkthrough.

W3 does not add recovery semantics not already decided by accepted policy. A
missing semantic policy must stop at a reviewed amendment rather than be
invented in the scheduler or TUI.

## 7. Mandatory user journeys

### Journey A: First local launch

1. User launches Loom without setting environment variables.
2. TUI connects to or clearly starts the user-level daemon.
3. Home reports actual daemon, Journal, projection, Runtime, and Provider state.
4. User can inspect Runtime capabilities, prior Runs, Evidence, and Attention.
5. Exit leaves no orphan client and does not stop authorized resident
   observation unless the user explicitly chooses service stop.

### Journey B: Build and confirm a Team

1. User chooses blank, saved, or template-backed creation.
2. Loom asks exactly one bounded question at a time.
3. Draft view explains Agents, Runtime/model/Skill bindings, permissions,
   compatibility, authentication modes, and estimated cost.
4. Back, cancel, and save-draft produce no TeamInstance or Run.
5. Explicit confirmation creates the accepted Team through the existing
   application authority.

### Journey C: Configure Providers safely

1. Codex login state is detected and labeled `native_auth`.
2. MiniMax setup writes an opaque secret to OS Secret Store and a non-secret
   reference through the Broker.
3. Pi local Runtime remains usable without a network Provider credential.
4. Status, failure, revoke, and replace flows never reveal secret material.

### Journey D: Controlled execution and recovery

1. User creates a WorkPackage and selects a confirmed Team.
2. Preflight explains scope, Runtime, permissions, budget, and approval points.
3. User explicitly starts execution.
4. TUI shows authoritative milestones and tentative authorized deltas.
5. At least one controlled path exercises approval or bounded recovery.
6. Client restart reconnects to the same lineage without resubmitting start.
7. Source and Verifier Evidence produce a canonical terminal result.
8. History, Compare, and Attention reflect the same Journal-authoritative
   outcome.

At final acceptance, these journeys must be demonstrated without entering a
Team ID, SQLite path, cursor, `launchctl` command, or Provider environment
variable.

## 8. Security and privacy acceptance

The Candidate must prove:

- private parent directories are `0700`; databases, manifests containing local
  state, and secret-reference files are `0600` where applicable;
- socket access is user-local and fail-closed;
- protocol messages have explicit version, kind, size, count, and deadline
  bounds;
- unknown fields or kinds that affect authority are rejected;
- no raw Provider secret, OAuth token, Grant, credential header, hidden
  reasoning, or full sensitive prompt appears in source, process arguments,
  environment snapshots, logs, Journal, Evidence, AgentDefinition, screenshots,
  panic text, or structured errors;
- secret comparison scans include encoded and partial forms without printing
  the secret;
- native OAuth material is never imported into Loom;
- TUI paste, terminal escape sequences, untrusted names, and Runtime output are
  rendered safely;
- reconnect cannot replay a mutation;
- stale view, cursor, stream head, generation, Grant, and attempt fail closed;
- no duplicate dispatch, Run, Evidence, approval, or external side effect is
  produced by retry, concurrent clients, daemon restart, or client restart.

## 9. Verification and review gates

### Phase 2A governance gate

Before product code:

1. ADR-0011 and this Exit Contract receive fresh independent Contract Review.
2. All findings are repaired in the same governance Candidate.
3. Review returns `PASS`.
4. ADR-0011 becomes `accepted` and this contract becomes `FROZEN`.

### Per-WorkItem gate

Each WorkItem follows:

1. Freeze one child contract with exact owned files, imports, protocol changes,
   acceptance tests, live exclusions, and rollback.
2. Fresh independent Contract Review returns `PASS`.
3. Capture behavioral RED before product implementation.
4. Implement only the frozen vertical boundary.
5. Run focused tests, deterministic TUI/protocol E2E, impact tests, race where
   concurrency exists, whole-repository tests, whole-repository race, vet,
   format, module/dependency, migration, platform compile, scope, authority,
   secret, and trust-boundary checks in proportion to risk.
6. Fresh independent Implementation Review returns `PASS`.
7. Create one local atomic commit containing only that WorkItem.

No wrapper-only repair WorkItem may be created. Review repairs remain within
the current WorkItem unless they require a reviewed authority amendment.

### Whole-Phase 2A gate

After W3:

1. Rebuild projections from the accepted baseline Journal.
2. Run the full repository, race, vet, format, dependency, migration, protocol,
   security, packaging, and deterministic product E2E matrix.
3. Prove all PX rows with paths, commands, digests, and bounded evidence.
4. Run exactly one accepted fresh isolated live canary for each of Codex OAuth,
   MiniMax, and Pi under separately frozen manifests.
5. Demonstrate the complete no-terminal user journey through the TUI.
6. Obtain fresh independent whole-Phase2A Review `PASS`.
7. Obtain explicit user review/sign-off.

Hermetic tests, fixture E2E, an active daemon process, or a Provider response
alone cannot be reported as product completion.

## 10. Live-canary rules

Each Provider/Runtime canary must freeze:

- exact installed Runtime and version;
- exact Provider/model and actual authentication mode;
- isolated `0700` attempt root and `0600` state/manifest files;
- maximum requests, tokens, duration, cost, retries, and external effects;
- exact Team, WorkPackage, node, Attempt, Run, generation, Grant, and Evidence
  lineage;
- no-fallback/no-hidden-retry behavior;
- expected Journal facts and projection results;
- cleanup and preservation of failed evidence.

Codex OAuth, MiniMax, and Pi evidence must remain distinct. Success for one does
not imply success for another. A failed canary is preserved and stops that
lineage; it is not silently retried.

## 11. Exclusions

Phase 2A does not deliver:

- browser UI, public API, cloud synchronization, remote callback, or
  multi-user roles;
- Autopilot, standing orders, schedules, webhook-triggered autonomous
  execution, or implicit automation from chat;
- a second Scheduler, queue database, Event Journal, StateWriter, Projection,
  Evidence store, Session authority, or capability authority;
- third-party Skill auto-activation, a Skill marketplace, or execution of an
  unreviewed imported asset;
- incremental Projection checkpoints or model-context/process-image
  checkpoints;
- Provider session resume as state authority;
- every-token Journal writes, raw hidden reasoning, or raw credentials;
- FastContext, ClickHouse, remote observability infrastructure, or unrelated
  Phase 2 draft-bucket items;
- push, merge, PR, release, publication, or production-wide activation.

## 12. Amendment and stop conditions

A reviewed amendment is mandatory before:

- expanding an accepted authority or reopening an owned boundary outside the
  active WorkItem;
- changing Journal, StateWriter, CAS, Grant, Evidence, Verifier, or scheduler
  semantics;
- introducing a destructive or non-backward-compatible migration;
- adding a fourth WorkItem or splitting a thin adapter/screen/handler item;
- exposing network access beyond the private local socket;
- storing credentials outside the OS Secret Store/Broker boundary;
- widening a live canary after failure.

Stop as `HUMAN_REQUIRED` when:

- OS Secret Store cannot be used safely;
- a Provider action requires new paid/external authorization;
- destructive migration or irreversible user-state loss is required;
- exact live credentials or OAuth consent require user interaction;
- the same blocking condition survives three consecutive governed attempts.

The Controller must not claim a missing capability, weaken a test to pass,
discard failed evidence, or silently expand scope.

## 13. Completion rule

Phase 2A becomes `COMPLETE` only when:

- ADR-0011 is accepted;
- exactly W1, W2, and W3 are locally atomically committed;
- every PX row is `DONE`;
- all three controlled live canaries pass;
- whole-Phase2A Review is `PASS`; and
- the user explicitly signs off.

Until then, status is `PARTIAL` and must name the current gate.
