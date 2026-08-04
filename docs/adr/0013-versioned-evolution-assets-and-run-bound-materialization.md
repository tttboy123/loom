# ADR-0013: Versioned evolution assets and Run-bound Runtime materialization

**Date**: 2026-08-03
**Status**: accepted
**Deciders**: Product Owner, Loom Architecture Controller

## Context

Loom already records exact Skill references in saved-Team configuration, keeps
Journal facts authoritative, stores immutable Evidence by digest and exposes
versioned GlobalReadView snapshots. The executable path currently drops the
Skill revision set before ExecutionPlan and Run creation, while Pi execution
explicitly disables Skills. Phase 3A also requires reusable Skills/Templates,
accepted-Run promotion and Evaluation without allowing a model, Runtime,
Sidecar or materialized filesystem copy to become activation authority.

The Phase 3A Entry Audit and independently accepted bounded Entry Amendment
permit one vertical P3A-W1 to close exact lineage, private native
materialization and real GUI/TUI journey correlation. They prohibit a second
Journal, writer service, Projection authority, scheduler, asset database or
thin follow-up WorkItem.

## Decision

Loom uses Journal-authoritative, immutable-revision evolution assets. Asset
bytes are immutable Artifacts; lifecycle, provenance, evaluation and explicit
user decisions are append-only facts; Projection and GlobalReadView are
rebuildable read models.

The binding path is:

```text
active exact revision set
-> confirmed Team/Agent/WorkPackage binding
-> immutable ExecutionPlan
-> Team dispatch CAS
-> Run + Attempt lineage
-> private Runtime-native materialization manifest
```

The following rules are binding:

1. `SkillDefinition` provides stable identity. Each `SkillRevision` has an
   immutable canonical digest, source/provenance, bounded scope, dependencies,
   compatible Runtime capabilities and risk. Agent, Team, WorkPackage and
   RecoveryStrategy templates use the same immutable-revision discipline.
2. Lifecycle facts use `draft`, `candidate`, `active` and `archived`. Import,
   promotion and Evaluation never activate an asset. Activation, rejection,
   retention, archive, restore and rollback require an explicit user command
   and stream-head CAS; concurrent commands have one winner.
3. Third-party imports enter as Candidates. Template instantiation produces a
   Team Draft, WorkPackage Candidate or other typed Candidate only. It cannot
   directly create a TeamInstance, Run, Grant or expanded permission.
4. A Run may be promoted only when its terminal state and accepted Evidence are
   authoritative and its Run/generation/Grant/Runtime/input lineage is
   complete. Promotion records a redacted provenance summary and source
   digests, never raw Grant, credential, sensitive prompt, hidden reasoning or
   per-token output.
5. Historical and synthetic Evaluations are immutable Evidence containing
   baseline, quality, cost/usage when observed, failure rate, compatibility,
   scope, regression and security results. Evaluation informs an explicit user
   decision; it is not execution or activation authority.
6. Every new Run and Attempt copies a canonical sorted exact revision set plus
   manifest digest before execution. Mutable aliases such as `active` are not
   resolved after dispatch. Updating or rolling back an asset never changes an
   already-bound Run.
7. Runtime materialization is capability-gated and private to one
   Run/Attempt/generation. Loom writes verified bytes into a fresh private
   root, atomically publishes a canonical manifest, never overwrites
   repository/user Skills, and deterministically cleans or rebuilds only its
   own root. Incompatibility, identity drift and partial failure fail closed;
   no fallback revision is silently selected.
8. Pi advertises `loom.skill-materialization.pi.v1` only when the installed
   Runtime supports the reviewed private-path contract. `--no-skills` remains
   until an exact private materialization path has passed compatibility tests.
9. GUI and TUI use the same production daemon/application/Journal path. A
   strict `journey_id` correlates IPC, logs, Journal `CorrelationID`,
   Projection, Evidence and cleanup but never becomes authority, CAS,
   idempotency, generation, Grant or digest input.
10. P3A-W1 is complete only after a real native-window and real-PTY journey over
    one daemon/root passes Product Result and Operational/Trace Review. Client
    unit tests, mocks, direct service calls and visual-only fixtures cannot
    substitute.

## Alternatives Considered

### Alternative 1: Store mutable Skills directly in Runtime-native paths

- **Pros**: Minimal adapter work and immediate Runtime discovery.
- **Cons**: Runtime files become a second state authority, updates affect
  running work, collisions can overwrite user/repository assets and rollback
  cannot prove exact bytes.
- **Why not**: It violates immutable Run lineage and Loom's one-authority rule.

### Alternative 2: Resolve the currently active revision at execution time

- **Pros**: Small Run schema and convenient global updates.
- **Cons**: Retries and restarts may execute different bytes; rollback changes
  in-flight behavior and Evidence cannot identify the executed revision.
- **Why not**: A Run must freeze exact revision and digest before dispatch.

### Alternative 3: Let accepted Runs or Evaluations auto-promote and activate

- **Pros**: Faster self-improvement loop.
- **Cons**: Model/Sidecar output becomes authority, regressions can activate
  silently and permissions may expand without review.
- **Why not**: Evolution output is Candidate-only and user activation is
  explicit.

### Alternative 4: Deliver the asset library as backend-only WorkItems

- **Pros**: Smaller code reviews and easier unit testing.
- **Cons**: Recreates writer/adapter/coordinator micro-splitting and allows a
  feature to be called complete without a user-operable cross-client journey.
- **Why not**: The mandatory Exit Gate requires one vertical GUI+TUI capability.

## Consequences

### Positive

- Every Run and Evidence chain can prove the exact reusable asset bytes used.
- Runtime-native Skills remain compatible without making Runtime storage
  authoritative.
- Import, promotion, Evaluation, activation and rollback are auditable and
  replay-safe.
- GUI, TUI and operational inspection observe the same authoritative facts.

### Negative

- Execution planning and Run/Attempt payloads become larger and require strict
  migration compatibility.
- Private materialization adds filesystem, cleanup and crash-recovery testing.
- Real native-window and PTY journeys make acceptance slower and more
  operationally demanding.

### Risks

- **Materialized files escape their root.** Mitigation: clean absolute roots,
  no-follow traversal, ownership/mode checks, collision rejection, atomic
  rename and deletion only of digest-bound Loom roots.
- **Asset status becomes a second execution switch.** Mitigation: activation is
  a Journal fact and execution copies exact revisions into authority before
  materialization.
- **Promotion leaks sensitive Run content.** Mitigation: allowlisted redacted
  summaries, immutable digest references and negative disclosure tests.
- **Journey correlation becomes command identity.** Mitigation: separate
  `request_id`, operation/idempotency key, expected heads, generation and
  `journey_id` validators with byte-level replay tests.
- **Client parity becomes visual parity only.** Mitigation: both clients must
  execute the same production IPC journey and jointly prove Journal,
  Projection, SQLite, Evidence and cleanup behavior.
