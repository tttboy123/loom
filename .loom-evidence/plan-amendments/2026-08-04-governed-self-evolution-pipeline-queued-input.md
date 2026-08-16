# Governed Self-Evolution Pipeline Queued Input

Status: `QUEUED` — non-authoritative and non-executing.

Date: 2026-08-04

Authority input: the Product Owner requested that Loom's future roadmap adopt a
governed feedback loop in which observed capability gaps may become successor
development work and, after independent verification and authorized
integration, a later Run may use the new versioned capability.

This file records future contract input only. It does not amend or expand the
active Phase 3A Candidate, authorize implementation, dispatch an Agent, create
a WorkItem, modify source authority, merge a Candidate, activate an asset,
start a daemon or Runtime, call a Provider, or claim that end-to-end
self-evolution is currently delivered.

## Relationship to the accepted roadmap

This input supplements, rather than replaces, the queued `v0.4.0` Agent
Scheduling Framework in
[`2026-08-01-interaction-handoff-auth-routing-scheduling-queued-input.md`](2026-08-01-interaction-handoff-auth-routing-scheduling-queued-input.md).

- Phase 3A remains responsible for versioned evolution assets, Candidate
  evaluation, explicit activation, exact revision/digest binding, archive,
  restore, and rollback.
- `v0.4.0` remains responsible for the Journal-authoritative development queue,
  Admission, Development, Repair, deterministic Test, read-only Review,
  single-writer Integration, and controlled self-host canary.
- This supplement closes the feedback boundary between those releases: an
  observed gap becomes a governed successor proposal, and an accepted
  integration becomes a versioned capability that only a later Run may select.

Before implementation, one reviewed roadmap/Exit Contract amendment must
reconcile this input with `PRODUCT-PLAN.md`, `TECH-PLAN.md`, the accepted Phase
3A result, and the existing `v0.4.0` queue. It must not create a second
Scheduler, Journal, StateWriter, Projection, queue database, asset authority,
or integration writer.

## Product outcome

Loom should support a **Governed Self-Evolution Pipeline**:

```text
authorized Run / Evidence / Review / user feedback
-> Gap Proposal
-> Successor WorkItem Proposal
-> Admission Decision
-> isolated Candidate development
-> deterministic Test and Evidence
-> independent read-only Review
-> single-writer Integration Candidate
-> explicit integration and activation authority
-> versioned release
-> later Run binds the exact new revision and digest
```

The product outcome is governed improvement, not autonomous self-ownership.
Models and Agents may observe, propose, implement, test, and submit Candidates;
they do not gain authority to accept their own work, merge the target branch,
activate a release, or rewrite the safety boundary.

## Gap-to-successor boundary

### Permitted gap sources

A gap proposal may be derived only from authorized and addressable inputs:

- terminal Run and accepted/rejected Evidence;
- deterministic Test or independent Review findings;
- explicit user feedback;
- retry exhaustion, capability gaps, recovery outcomes, or integration
  conflicts recorded as authoritative facts; and
- aggregated, privacy-safe product metrics whose provenance and version are
  available for inspection.

Raw hidden reasoning, credentials, raw Grants, unapproved transcript content,
and unverified model assertions are not valid gap evidence.

### Gap Proposal

Every proposal must bind at least:

- a stable `gap_id`, source type, source IDs, and source digests;
- affected capability, observed behavior, expected behavior, and user impact;
- confidence, uncertainty, reproducibility, and privacy classification;
- proposed scope, owned-path/resource claims, risk class, and rollback idea;
- duplicate/supersession relationship to existing gaps and work; and
- an explicit disposition: `observe`, `reject`, `merge_duplicate`,
  `human_required`, or `propose_successor`.

Gap discovery is read-only. It cannot create or dispatch development work by
itself.

### Successor WorkItem Proposal

A successor is created only after Admission validates:

- user or previously accepted bounded-policy authority;
- absence of duplicate active work;
- DAG readiness and explicit exit conditions;
- vertical product value rather than wrapper-only decomposition;
- owned-path, authority, mutex, budget, network, privacy, and resource
  compatibility;
- deterministic verification and independent Review strategy; and
- integration, release, rollback, and later-Run adoption strategy.

The successor receives independent WorkItem, Run, Attempt, generation, Grant,
Evidence, Candidate, branch, and worktree lineage. It does not inherit broader
authority from the source Run or gap.

## Development and integration authority

| Role | Permitted | Forbidden |
|---|---|---|
| Gap observer | Submit a digest-bound Gap Proposal | Create work, dispatch, modify source, or activate |
| Planner/Admission | Compile and validate a successor Candidate | Bypass conflicts, approvals, or exit conditions |
| Developer worker | Modify frozen owned files in its isolated Candidate | Write the target branch or mark its own work accepted |
| Test executor | Run deterministic commands and persist result Evidence | Reinterpret product acceptance or merge |
| Reviewer | Read Candidate, tests, Evidence, and authority trace | Modify product files or approve its own implementation |
| Integrator | Apply one reviewed Candidate to the target branch | Expand scope, hide conflicts, or become a second Scheduler |
| Activation authority | Publish/activate an exact accepted version | Hot-patch a running Team, Attempt, or Runtime |

At-least-once dispatch must remain idempotent under CAS, lease, and generation
fencing. Late or stale results are rejected. Hidden infinite retry, silent
Provider fallback, automatic conflict resolution, and self-review are
forbidden.

## Self-modification safety classes

The future Exit Contract must classify change targets before dispatch.

### Eligible only under normal governed development

- user-interface and presentation modules;
- ordinary product/application behavior;
- Runtime adapters that do not expand authority;
- tests, fixtures, documentation, Skills, templates, and bounded tooling; and
- performance or reliability repairs that preserve accepted authority.

Eligibility is not automatic permission. Owned files, tests, Review,
Integration, and activation gates still apply.

### Protected authority requiring a separately reviewed human-governed contract

- root policy and permission validators;
- Grant, credential, secret-store, and identity boundaries;
- Event Journal, authoritative writers, Evidence acceptance, and generation
  fencing;
- Scheduler Admission and Integration authority;
- sandbox escape controls, network policy, and approval semantics; and
- code that decides whether self-evolution itself may proceed.

No online Agent, Sidecar, model, or self-host cycle may silently add these files
to its owned scope or weaken their tests. Some protected changes may remain
human-only even after `v0.4.0`.

## Adoption by a later Run

Integration and adoption are separate authoritative transitions.

1. A reviewed Candidate may be integrated by the single target-branch writer.
2. Integration produces a versioned build/asset Candidate with source,
   Evidence, and dependency digests.
3. Required release verification and user/activation authority decide whether
   that version becomes eligible.
4. Running Teams and Attempts keep their original exact bindings.
5. Only a new Attempt or later Run may bind the new version under current
   policy and capability checks.
6. Failure or regression creates a new gap/repair lineage or an explicit
   rollback; it never rewrites historical facts.

Session IDs, model context, process memory, Projection checkpoints, and
uncommitted worktrees are not capability-version authority.

## Delivery shape

Do not add a fourth thin WorkItem to the existing `v0.4.0` plan. Incorporate
this closure into its three vertical boundaries:

1. **Queue State/Projection and Admission** also owns Gap Proposal,
   deduplication, successor compilation, eligibility, and conflict arbitration.
2. **Ephemeral Worker Pools and routing** also owns isolated successor lineage,
   bounded repair, deterministic Test, read-only Review, and stale-result
   rejection.
3. **Single-writer Integration and controlled self-host canary** also owns
   versioned release handoff, later-Run exact binding, rollback proof, Timeline,
   and Attention/HumanRequired presentation.

Do not split gap writer, successor adapter, test coordinator, review forwarder,
release wrapper, or adoption selector into separately governed thin WorkItems.

## Minimum acceptance

The eventual vertical contract must prove all of the following with one
controlled self-host development journey:

1. An authorized failed or incomplete Run produces exactly one digest-bound Gap
   Proposal and no WorkItem side effect.
2. Duplicate observations converge on the same gap; stale or unauthorized
   evidence cannot create a successor.
3. Explicit Admission creates exactly one isolated successor lineage with
   frozen scope and exit conditions.
4. A permitted Loom module is modified in an independent branch/worktree by an
   ephemeral Developer worker.
5. Deterministic Test persists reproducible Evidence; a classified failure
   enters bounded Repair rather than hidden retry.
6. The Developer cannot act as Reviewer, and the read-only Reviewer cannot
   modify product or target-branch state.
7. Only the Integrator may update the target branch; a competing or stale
   integration loses through CAS/generation fencing without duplicate effects.
8. Protected authority and out-of-scope file modifications fail closed.
9. The accepted result becomes an exact versioned release Candidate; it is not
   hot-loaded into the current Attempt.
10. A later Run demonstrably binds the new revision/digest and exercises the
    new capability; rollback restores the prior eligible version without
    rewriting Run or Evidence history.
11. Crash/restart reconstructs gaps, queue state, leases, Candidates,
    decisions, and adoption state from the Event Journal and immutable
    artifacts.
12. GUI and TUI expose the same authoritative gap, successor, Test, Review,
    integration, activation, and rollback timeline through the production IPC
    chain; neither client becomes a second authority.

The canary must preserve screenshots or recording, TUI transcript, user-action
timeline, structured daemon logs, Journal/SQLite summary, Evidence/artifact
digests, exact release/adoption bindings, and process/socket/lock cleanup proof.

## Explicit non-goals

- self-approval, self-merge, or self-activation;
- hot modification of a running Team, Run, Attempt, Runtime, or Grant;
- an Agent that permanently polls for its own work;
- a second queue database, Scheduler, Journal, Projection, StateWriter, or asset
  authority;
- automatic modification of protected authority boundaries;
- using hidden reasoning, credentials, raw Grants, or every output token as
  evolution input;
- model-context or process-image checkpointing as state authority; and
- declaring a capability delivered from unit tests, fixtures, or a launch-only
  client check without the controlled vertical journey.

## Governance sequence

After Phase 3A reaches an accepted safe checkpoint:

1. reconcile the roadmap and accepted authority boundaries;
2. freeze one `v0.4.0` Exit Contract covering the complete feedback loop and
   the existing three vertical delivery boundaries;
3. obtain an independent Contract Review `PASS`;
4. establish RED for gap-to-successor, isolated development, integration, and
   later-Run adoption;
5. implement without thin wrapper WorkItems or silent scope expansion;
6. run focused, impact, race, security, replay/restart, cross-client journey,
   and controlled self-host verification;
7. obtain a fresh independent Implementation Review `PASS`; and
8. update delivery status only from accepted evidence.

No part of this queued input is implementation or live-execution authority.

VERDICT: `QUEUED`
