# Phase 1 Slice 3 Exit Contract

- Frozen branch: `codex/loom-platform-slice2`
- Frozen baseline: `7b1726e`
- Date: `2026-07-25`
- Authority: `TECH-PLAN.md` sections 7, 8, 11, 14, and 15
- Scope: Slice 3 real-execution control plane

This governance contract fixes Slice 3's exit list before product work starts.
It does not itself authorize execution, Runtime activation, credentials,
Provider/model calls, or a resident service.

## Accepted prerequisites

Slice 2 provides accepted:

- AgentDefinition, RuntimeProfile, RuntimeInstance, Team Draft, saved-Team, and
  exactly-one-Main domain records;
- atomic Journal append, StateWriter patterns, rebuildable projection, and
  immutable Evidence Artifact Store;
- isolated Pi metadata discovery and a non-activating local observation daemon.

These boundaries remain closed. Slice 3 may consume them but must not reopen or
duplicate their authorities.

## Exit capabilities

Slice 3 exits only when all are `DONE`:

| Capability | Baseline | Required Slice 3 result |
|---|---|---|
| Bridge v1 trust boundary | `MISSING` | Exact bounded JSON Lines envelope validation for every TECH-PLAN message type, immutable payload access, fail-closed protocol/version/identity/time/sequence handling, and a bound per-Run stream Candidate |
| Run and WorkItem authority | `MISSING` | Atomic creation/assignment/claim, monotonic `claim_generation`, prepare lease, late-generation rejection, terminal-once commit, and rebuildable projections |
| AgentGrant authority | `MISSING` | Random task-level grant material, hash-only persistence, Run/Agent/generation/operation/expiry binding, revocation, and fail-closed validation; no Client/Daemon identity inheritance |
| Managed execution boundary | `MISSING` | Private managed workspace, source-unchanged checks, one production Runtime Adapter over configured executable/stdio, process-group ownership, bounded Bridge session, cancel/timeout cleanup, and supervisor-generated failure terminal |
| Team DAG execution | `MISSING` | Exactly one Main plus at most two SubAgents, explicit WorkItems, one independent Run/Grant/Evidence lineage per active SubAgent, dependency ordering, and deterministic terminal aggregation |
| Controlled integration proof | `MISSING` | Real local SQLite/Journal/projection/Evidence execution with a deterministic Bridge-compatible fixture, restart/recovery, stale-generation rejection, cancel/timeout/no-orphan proof, and no user Runtime/model/credential activation claim |

## Maximum WorkItems

At most five Slice 3 product WorkItems are permitted:

1. `S3-W1 Bridge v1 Wire and Bound Run Stream`
   - new untrusted protocol boundary;
   - exact TECH-PLAN envelope and stream invariants;
   - no process, Journal, Run mutation, Grant, or execution.
2. `S3-W2 Run Claim, Lease, Terminal, and Projection Authority`
   - one persistence/transaction authority boundary;
   - WorkItem/Run Events, StateWriter, generation/lease/terminal rules, and
     rebuildable read model in one Candidate.
3. `S3-W3 AgentGrant Authority`
   - one credential-like local security boundary;
   - issuance, hash-only storage, validation, expiry, generation binding, and
     revocation in one Candidate.
4. `S3-W4 Managed Workspace Runtime Adapter and Supervisor`
   - one filesystem/process boundary;
   - configured real adapter, Bridge session, workspace/source checks,
     cancellation, timeout, cleanup, and failure terminal in one Candidate.
5. `S3-W5 Team DAG Execution Integration`
   - one user-observable vertical capability;
   - Main/SubAgent WorkItems, Runs, Grants, Evidence, dependency scheduling,
     restart/recovery, and controlled local end-to-end canary.

No constructor-only, one-call forwarding, payload-wrapper, scheduler-only,
writer-selection, or coordinator-only WorkItem may be added. Internal helpers,
ports, codecs, writers, projections, adapters, and coordinators required by one
listed boundary belong inside that WorkItem.

## WorkItem admission rule

A Candidate must introduce at least one:

- authoritative persistence transaction;
- security/token boundary;
- untrusted protocol or process/filesystem boundary; or
- user-observable end-to-end execution capability.

If a listed WorkItem cannot close coherently, freeze one reviewed amendment to
this exit contract or stop `HUMAN_REQUIRED`. Do not route around it by adding
W6 or thin intermediate WorkItems.

## Execution and live boundaries

“Production Runtime Adapter” means real production code against an explicitly
configured executable and stdio contract. Until separately authorized, tests
and controlled canaries use only deterministic local fixtures in private
temporary roots. They do not use an installed user Runtime, Provider/model
call, user credentials, ambient `PATH`/`HOME`, resident service, or autonomous
execution.

The later final Phase 1 real-task demo remains a separate user gate.

## Slice exit gate

Before Slice 4:

1. all five listed WorkItems are accepted and locally committed;
2. every exit capability is `DONE`;
3. all deliverables end with `VERDICT: PASS`;
4. full repository, repository-race, vet, format, scope, trust-boundary, and
   evidence audits pass;
5. controlled fixture integration proves restart, stale generation, terminal,
   cancel/timeout, cleanup, and projection recovery;
6. a fresh independent whole-Slice Reviewer returns `PASS`.

Slice 4 remains closed until all six conditions hold.

## Explicit non-authority

This contract does not authorize:

- Phase 2;
- real user Runtime or Agent activation before a separate live gate;
- Provider/model calls, credentials, or real Broker delegation;
- root policy or existing authoritative writer mutation outside a frozen
  WorkItem;
- push, merge, rebase, reset, force, release, publication, or service install.

VERDICT: PASS
