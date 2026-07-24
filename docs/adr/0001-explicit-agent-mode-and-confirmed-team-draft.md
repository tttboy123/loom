# ADR-0001: Explicit Agent mode and confirmed Team Draft

**Date**: 2026-07-24
**Status**: accepted
**Deciders**: lune, Codex

## Context

Loom supports both ordinary conversation and coordinated Agent execution. Automatically creating a team for every complex user request would add cost, delay and unexpected side effects to normal conversation. When the user has not defined a team, the model still needs a safe way to propose the roles, tasks and execution envelope.

## Decision

Loom enters team resolution only after an explicit Agent trigger. A user-selected complete Team loads directly; a model-generated or model-completed team becomes a versioned Team Draft, asks one material question at a time and creates Agent instances only after user confirmation.

## Alternatives Considered

### Alternative 1: Automatically create a team for every task

- **Pros**: Minimal explicit setup and maximum automation.
- **Cons**: Ordinary conversation can unexpectedly create processes, cost and approval requests.
- **Why not**: Task complexity is not reliable evidence that the user intended Agent execution.

### Alternative 2: Require users to define every team manually

- **Pros**: Predictable membership and no model-generated configuration.
- **Cons**: High setup cost and poor first-use experience.
- **Why not**: It prevents Loom from helping users discover a suitable team while retaining final control.

## Consequences

### Positive

- Ordinary conversation remains lightweight and predictable.
- Generated teams are inspectable before resources or permissions are allocated.

### Negative

- The client and daemon must preserve an explicit mode boundary.
- Team Draft revisions add an interaction step before first execution.

### Risks

- A vague UI action could accidentally imply Agent intent; mitigate with structured trigger types and a visible mode indicator.
