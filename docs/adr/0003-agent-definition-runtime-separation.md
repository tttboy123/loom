# ADR-0003: Agent definition and runtime separation

**Date**: 2026-07-24
**Status**: accepted
**Deciders**: lune, Codex

## Context

Users need project Agents, reusable personal Agents and task-specific generated Agents. Binding a role directly to one model or Agent CLI makes definitions hard to reuse and prevents cost or availability changes. Leaderless teams also leave task decomposition and terminal synthesis ambiguous.

## Decision

Loom separates model-independent AgentDefinition from RuntimeProfile and binds them only in AgentInstance. Every TeamInstance has exactly one Main Agent responsible for decomposition, dispatch and synthesis; execution and independent verification remain delegated to bounded SubAgents.

## Alternatives Considered

### Alternative 1: Bind each Agent definition to a model and Provider

- **Pros**: Simple configuration and predictable runtime.
- **Cons**: Duplicated role definitions and poor portability.
- **Why not**: Users must be able to change models without rewriting responsibilities and acceptance boundaries.

### Alternative 2: Peer-to-peer team without a Main Agent

- **Pros**: No central planning bottleneck.
- **Cons**: Ambiguous ownership of dependencies, retries and final synthesis.
- **Why not**: Loom needs one visible coordination entry while still allowing many execution Agents.

## Consequences

### Positive

- Roles are reusable across Providers and Agent CLIs.
- Team coordination has one accountable owner without creating a universal executor.

### Negative

- Runtime compatibility and capability matching become explicit scheduling concerns.
- Main Agent availability can delay coordination even when workers are healthy.

### Risks

- Main Agent could accumulate execution privileges; mitigate by denying delivery-write tools and routing actual work to SubAgents.
