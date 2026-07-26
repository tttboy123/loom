# Phase 1 Slice 5 Exit Contract

Status: FROZEN — independent Exit Contract Repair Review 2 PASS.

- Branch: `codex/loom-platform-slice2`
- Frozen baseline candidate: `006db8c`
- Date: `2026-07-26`
- Authority: `TECH-PLAN.md` sections 4.2, 6, 13, 14, 15, and 16
- Roadmap amendment:
  `.loom-evidence/plan-amendments/2026-07-26-post-slice3-capability-roadmap.md`
- Scope: local observation/timeline delivery, Attention/read-only board,
  WorkPackage parity, and the Phase 1 engineering demo boundary

This contract freezes the final Phase 1 engineering Slice before product work.
It authorizes no installed Runtime execution, Provider/model traffic,
credentials, network, daemon/resident activation, external action, push,
merge, release, publication, or final user sign-off.

## Accepted prerequisites

Slices 1 through 4 provide accepted:

- explicit conversation versus Agent routing;
- append-only Journal, bounded transactions, immutable Evidence, Projection,
  and immutable `GlobalReadView`;
- Agent/Runtime/Team Draft/saved-Team catalogs and explicit confirmation;
- one real Pi Runtime adapter and local discovery contract, without activation;
- WorkItem/Run/Claim/AgentGrant/Supervisor/attempt capture authority;
- deterministic Team DAG execution with one Main and at most two SubAgents;
- authorized tentative Frame observation;
- customer Rules, durable approval, output classification, bounded recovery,
  independent verification, and terminal-once WorkItem Done.

Those authorities remain closed. Slice 5 consumes them through reviewed APIs.
No timeline, cursor, Attention item, board row, CLI state, WorkPackage, or Demo
surface becomes execution, approval, retry, verification, or completion
authority.

## Exit capabilities

Slice 5 engineering exits only when every engineering capability is `DONE`:

| Capability | Baseline | Required Slice 5 result |
|---|---|---|
| Versioned local event interface | `MISSING` | Bounded typed local delivery records with schema version, stable event/cursor identity, exact Team/node/attempt lineage, and no second state authority |
| Tentative node output | `PARTIAL` | Existing authorized `NodeOutputObserver` is bound to a Team stream; text deltas remain bounded memory-only and explicitly tentative; verifier/raw unauthorized Frames never publish |
| Authoritative timeline and reconnect | `MISSING` | Started/approval/retry/warning/degraded/blocked/human-required/verification/terminal records are derived only from Journal/Projection; bounded cursor/`Last-Event-ID` reconnect combines exact Journal position with current `GlobalReadView` |
| Slow-consumer recovery | `MISSING` | Tentative text may coalesce; authoritative facts remain Journal-backed and are never memory-queue authority; invalid/stale/overflow cursor returns `stream_gap` plus current view/artifact digest |
| Local board and Attention CLI | `PARTIAL` | Read-only deterministic Team/Task/Observation/Governance timeline plus Attention items; unknown Cost is explicitly unavailable, never invented; no direct SQL outside Journal/Projection readers |
| WorkPackage parity | `MISSING` | One bounded Coding and one bounded knowledge-work package differ only in recommended AgentDefinitions, tool categories, default verifier, Evidence types, and default customer Rule templates; both use the same Team/Work/Grant/Evidence/approval/recovery/acceptance state machine |
| Phase 1 engineering demo | `MISSING` | Controlled local SQLite/Supervisor demo proves both WorkPackages, reconnect, crash/restart, tentative-versus-authoritative delivery, approval/recovery/verification/Done, and no duplicate side effect |

The real installed-Runtime task required by final Phase 1 product acceptance is
a separate final human gate. Engineering may reach
`READY_FOR_FINAL_USER_SIGNOFF`; it may not mark Phase 1 `COMPLETE`, perform the
live Run, or sign for the user.

## Maximum WorkItems

At most two Slice 5 product WorkItems are permitted:

1. `S5-W1 Local Observation Stream and CLI Timeline Integration`
   - one user-observable read/delivery vertical boundary;
   - bounded Journal cursor page, versioned local event records, Team-bound
     authorized tentative deltas, `GlobalReadView` snapshot/reconnect,
     slow-consumer behavior, Attention derivation, and read-only CLI timeline;
   - no new writer, scheduler, approval, recovery, acceptance, API server,
     WebSocket, Web/TUI, notification delivery, daemon activation, or external
     action.
2. `S5-W2 WorkPackage and Phase 1 Engineering Demo Integration`
   - one domain-template and final controlled integration boundary;
   - immutable bounded Coding and knowledge WorkPackages;
   - the same accepted state machine and authority sequence for both;
   - controlled local SQLite/Supervisor crash/restart and delivery canary;
   - a non-executing live-demo manifest/checklist for the final user gate;
   - no installed Runtime execution, Provider/model traffic, credentials, or
     self-declared final user acceptance.

No cursor-reader-only, queue-only, Attention-only, CLI-wrapper-only,
WorkPackage-codec-only, Demo-fixture-only, S5-W3, or other thin WorkItem may be
added. Helpers, adapters, Projection accessors, CLI wiring, and tests belong
inside the applicable vertical WorkItem.

## WorkItem admission and sizing

Before RED, each S5-W1/S5-W2 contract must freeze:

- exact new and reopened product, test, CLI, governance, and fixture files;
- every new/reused authority API and the package dependency direction;
- all public typed errors and their `errors.Is` mapping;
- exact local delivery, cursor, gap, Attention, board, WorkPackage, and Demo
  record schemas plus canonical encodings;
- every deterministic ID/digest input in order;
- exact cursor bytes, stream-count, page-size, record-size, queue-size,
  coalescing, subscriber, WorkPackage-field, and Demo-step numeric limits;
- Journal read consistency, cursor validation, replay/projection agreement,
  deep-copy, old-view preservation, and explicit absence of a new CAS/writer;
- exact CLI commands, flags, JSON/JSONL output shape, exit codes, read-only
  database behavior, sanitization, cancellation, and reconnect semantics;
- mandatory behavioral RED, focused/impact/full/race/vet/format/Windows/scope
  checks, private SQLite/Supervisor fixtures, and contract-to-test traceability;
  and
- exact trust boundary, final live-gate exclusions, and dependency/change
  audit.

The child contract may reopen an accepted file only when the behavior cannot
be closed through its accepted API. It must list the exact hunk purpose. Any
additional writer, migration, dependency, product package, executable,
WorkItem, live action, or owned file requires a reviewed amendment before
editing. If the vertical capability cannot close within S5-W1/S5-W2 and their
numeric bounds, stop `HUMAN_REQUIRED`; do not create S5-W3 or silently weaken a
guarantee.

## Frozen delivery semantics

### Local event identity

- Every delivery record is an immutable copied value with exact schema
  version, kind, delivery ID, Team/node/attempt lineage when applicable,
  authoritative/tentative marker, occurrence time, cursor, and bounded safe
  payload.
- Delivery IDs are deterministic over the source Journal Event identity for
  authoritative records and the already-authorized Frame identity/sequence
  for tentative records.
- The cursor is opaque and versioned. It binds the Team ID, a complete bounded
  vector of related Journal stream heads, exact head Event identities, the
  related-stream scope digest, and the observed `GlobalReadView` version.
  Cursor validation is fail-closed.
- A cursor is delivery state only. It never changes Journal, Projection,
  WorkItem, Run, approval, recovery, verification, Team, or Evidence state.

### Authoritative records

- Authoritative milestones are read from exact append-only Journal Events and
  mapped to a closed safe vocabulary. Unknown Events remain queryable through
  Projection but are not reinterpreted as timeline authority.
- Journal pages use a transaction-consistent related-stream read. The reader
  returns copies and never writes, migrates, vacuums, or acquires execution
  authority.
- A subscription is bound to exactly one TeamInstance. Its related-stream
  scope is re-derived from the current copied `GlobalReadView` and contains
  only the Team execution stream plus referenced source/verifier
  WorkItem/Run/Grant/Evidence and ApprovalRequest streams.
- Phase 1 bounds the union of cursor and currently related streams to at most
  96, a cursor to at most 32 KiB, one page to at most 128 delivery records,
  and one record to at most 8 KiB. Exceeding a bound returns a typed gap; it
  never falls back to `ReadAll`.
- A cursor head entry is exactly:

  ```text
  stream_id, sequence, event_id
  ```

  Entries are sorted by `stream_id`, unique, and include sequence zero with an
  empty Event ID for a not-yet-created related stream. Non-zero entries must
  resolve to the exact immutable Event at that stream sequence.
- The inclusion predicate is per stream:

  ```text
  event.seq > cursor_head[stream_id].sequence
  ```

  Newly related streams absent from the old cursor use sequence zero. Streams
  present in the old cursor remain in the union even when no longer current.
  This prevents a later append in a lexically earlier stream from being
  skipped.
- Included Events are merged deterministically by
  `(emitted_at, stream_id, seq, id)`. A page cursor advances only the entries
  represented by returned Events; it never jumps to the current database
  heads past undelivered records.
- Reconnect first validates every supplied vector entry and the Team/scope
  bounds. It then combines the later bounded Journal page with the current
  immutable `GlobalReadView`. A changed view version is normal and is returned
  as the new snapshot version; it is not itself a cursor failure.
- Cursor mismatch, compaction/position mismatch, malformed facts, or a
  non-contiguous page fails to `stream_gap`/typed error; it never silently
  invents continuity.
- Approval, retry, degraded, blocked, human-required, verification, rejection,
  Done, and terminal records cannot be dropped by an in-memory queue because
  the Journal remains their delivery source.

### Tentative records

- Accepted private attempt capture remains durable and occurs before the
  delivery observer, exactly as frozen by ADR-0009. “Memory-only” applies only
  to tentative client-delivery records, never to Evidence attempt capture.
- Only `supervisor.AuthorizedFrame` values received after Adapter decode,
  Supervisor binding/sequence checks, `AgentGrant.Authorize`, and
  `BoundRunStream` may reach the Team-bound observer.
- Verifier output is excluded from the node-output observer.
- Only bounded safe text/event deltas may publish. Raw Grant/token, credential,
  prompt, hidden reasoning, direct personal identifier, unrestricted artifact
  path, malformed Frame, old generation, wrong Team binding, or unknown
  message type fails closed.
- Tentative delivery deltas are memory-only and never appended to Journal or
  Sidecar. They may coalesce per Team/node/attempt while preserving byte and
  item ceilings.
- The Team-bound observer performs only bounded validation plus a non-blocking
  in-memory enqueue/coalesce operation after private capture. It never waits
  on a client channel.
- Subscriber cancellation, queue overflow, slowness, absence, or delivery
  context cancellation is absorbed as delivery state and never returned as an
  observer error that could fail the underlying Run. Overflow drops/coalesces
  only tentative delivery records, records one bounded pending `stream_gap`,
  and resumes from authoritative Journal/Projection state.
- A malformed authorized value or impossible bound Team/node/attempt mapping
  still returns the frozen typed delivery-input error and fails closed; tests
  distinguish this programming/authority violation from subscriber state.
- Verifier execution never receives this Team-bound observer and therefore
  never publishes tentative verifier output.

### Exact stream gap

A gap is a non-authoritative delivery-control record containing exactly:

```text
schema_version, delivery_id, kind="stream_gap", team_instance_id,
reason, previous_cursor_digest, current_view_version,
artifact_available, artifact_digest, recoverable=true, occurred_at
```

`reason` is one of `invalid_cursor`, `cursor_conflict`, `scope_overflow`,
`page_overflow`, or `tentative_overflow`. `artifact_digest` is the latest
accepted Evidence digest visible for the Team when available; missing and a
legitimate digest are never conflated. The gap cannot approve, retry, verify,
complete, or mutate state.

### Attention and board

- Attention contains only items requiring human action or explicit awareness:
  pending/expired approval, blocked, human-required, retry exhausted, Runtime
  offline, verification failed/rejected, and stream gap.
- Attention and board rows are deterministic projections over copied
  `GlobalReadView`/timeline data, sorted stably, and deep-copied.
- Notifications, client presence, read cursor, acknowledgement, or CLI output
  cannot approve, retry, verify, complete, dispatch, or create an Agent.
- Cost/model/provider values absent from authoritative facts are rendered
  exactly as unavailable/not-observed. Zero and missing are not conflated.

## Frozen WorkPackage semantics

`WorkPackage` is an immutable versioned value containing only:

- stable package ID/version/digest and domain kind;
- recommended exact AgentDefinition IDs;
- bounded sorted tool categories;
- default verifier key;
- bounded Evidence type vocabulary; and
- bounded default customer Rule template IDs.

Constructors validate UTF-8, control characters, duplicates, ordering,
version, size, allowed domain kind, and canonical digest. Accessors return
copies.

Slice 5 supplies exactly one Coding and one knowledge-work package. Neither
package:

- creates a Team Draft, TeamInstance, AgentInstance, WorkItem, Run, Grant,
  approval, or Rule;
- activates a Runtime, Skill, template, or permission;
- contains credentials, prompts, hidden reasoning, executable scripts, or raw
  artifacts; or
- changes the accepted orchestration/state machine.

The controlled demo binds either package to the same explicit Team request and
accepted authority sequence. The only differences are the frozen package
fields and fixture deliverable/Evidence vocabulary.

## Controlled engineering demo

The S5-W2 canary uses private temporary roots, local SQLite, deterministic
clocks/identities, and in-process Supervisor adapters. It proves:

- explicit Agent routing and accepted Team binding;
- Coding and knowledge WorkPackages use the same coordinator/state machine;
- approval pause/reopen/resume;
- one Main plus up to two SubAgents and dependency ordering;
- bounded retry/exhaustion and distinct attempt lineage;
- authorized tentative delta before terminal, followed by Journal-backed
  authoritative verification/Done;
- independent Verifier isolation for a high-risk node;
- client disconnect/reconnect from cursor plus `GlobalReadView`;
- crash/restart from durable Journal/Evidence without duplicate Run, Grant,
  Evidence, Done, or external side effect;
- malformed/stale cursor, generation, Frame, Projection, and WorkPackage fail
  closed while the prior view remains available; and
- private state/artifact path modes remain `0700`/`0600`.

The canary is hermetic engineering evidence. It does not prove an installed
Pi process, Provider/model response, customer credentials, network readiness,
or final user satisfaction.

## Final live user gate

After both WorkItems, all engineering checks, and a fresh whole-Slice Reviewer
pass, Controller may set:

```text
Phase 1: READY_FOR_FINAL_USER_SIGNOFF
```

Then execution stops and presents:

- exact committed chain and 22-item Phase 1 acceptance audit;
- exact local test/race/vet/format/scope/trust evidence;
- a redacted non-executing live-demo manifest naming the selected saved Team,
  WorkPackage, RuntimeInstance capability, bounded task, approval expectations,
  Evidence location, stop/cancel procedure, and rollback/recovery checks; and
- the explicit permissions still required to run a real installed Runtime
  task and obtain the user's final review/sign-off.

The assistant cannot perform or fabricate the final live task, external
effects, credentials, or user signature under the current authorization.

## Mandatory RED and verification

Every WorkItem follows:

```text
reviewed frozen contract
→ mandatory behavioral RED
→ minimal GREEN
→ focused and impact tests
→ full repository and repository-race
→ vet / format / diff / scope / trust audit
→ fresh independent implementation Review
→ exact local atomic commit
```

Slice 5 additionally requires:

- cursor/page property tests and repeated reconnect tests;
- bounded queue/coalescing/slow-consumer concurrency and race tests;
- CLI read-only/sanitization tests;
- WorkPackage mutation/digest/parity tests;
- controlled SQLite/Supervisor disconnect/restart canary; and
- Windows compilation for all new pure/read-only packages and CLI surfaces.

## Slice exit gate

Before `READY_FOR_FINAL_USER_SIGNOFF`:

1. exactly S5-W1 and S5-W2 are accepted and locally committed;
2. every engineering exit capability is `DONE`;
3. every final deliverable ends `VERDICT: PASS`;
4. full repository, repository-race, vet, format, scope, trust-boundary,
   authority, secret, Evidence, cursor, and private-mode audits pass;
5. the controlled engineering demo passes without live Runtime/Provider use;
6. a fresh independent whole-Slice Reviewer returns `PASS`; and
7. `docs/CURRENT.md` says `READY_FOR_FINAL_USER_SIGNOFF`, not `COMPLETE`.

Phase 2 and any live/installed Runtime task remain closed until the user
explicitly performs the final review/sign-off and grants any required live
execution authority.

## Explicit exclusions

Slice 5 does not add or authorize:

- a second Journal, StateWriter, Projection, Team, Work, Grant, Evidence,
  approval, recovery, verification, or acceptance authority;
- direct SQL outside the accepted Journal/Projection read boundary;
- per-token Journal/Sidecar storage or tentative delta persistence;
- WebSocket, HTTP server, TUI, Web UI, remote callback, external notification,
  cloud sync, multi-user visibility, marketplace, or Autopilot;
- Credential Broker, Provider/model fallback, session/context/process-image
  checkpoint, or incremental Projection checkpoint;
- installed user Runtime execution, a real Agent task, daemon/resident
  activation, network, credentials, paid/external work, or publication;
- Phase 2/3 product implementation, S5-W3, push, merge, rebase, reset, force,
  release, or final user sign-off.

VERDICT: PASS
