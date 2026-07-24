# ADR-0002: Local modular monolith and event projections

**Date**: 2026-07-24
**Status**: accepted
**Deciders**: lune, Codex

## Context

Phase 1 must coordinate processes, approvals, evidence and recovery on one workstation. Splitting the control plane into network services would introduce distributed failure modes before the product contract is proven. Mutable tables alone would make duplicate messages, crash recovery and audit reconstruction difficult.

## Decision

Loom uses a local Go modular monolith as the only authority for state transitions. It appends idempotent facts to an Event Journal, updates rebuildable read models in SQLite transactions and atomically publishes large Evidence to a daemon-owned content-addressed Artifact Store; Client, Agent Runtime and Evolution Sidecar access state only through versioned daemon APIs.

## Alternatives Considered

### Alternative 1: Microservices from Phase 1

- **Pros**: Independent scaling and deployment boundaries.
- **Cons**: Network coordination, distributed transactions and operational overhead.
- **Why not**: Initial workloads are local and do not justify distributed control-plane complexity.

### Alternative 2: Mutable task tables without an Event Journal

- **Pros**: Smaller initial Schema and familiar CRUD implementation.
- **Cons**: Weak replay, audit, idempotency and failure reconstruction.
- **Why not**: Agent runs and approvals need durable causality, not only their latest status.

## Consequences

### Positive

- One deployable daemon keeps local setup and transactions simple.
- Events support crash recovery, audit and reproducible projections.
- Digest-bound artifacts keep large Evidence immutable without bloating SQLite.

### Negative

- Internal module boundaries require discipline because the compiler does not enforce process separation.
- Event and projection migrations must be versioned together.

### Risks

- The monolith could become tightly coupled; mitigate with explicit domain ports and dependency-direction checks.
