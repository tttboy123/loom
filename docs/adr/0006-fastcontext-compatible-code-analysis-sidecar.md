# ADR-0006: FastContext-compatible local code analysis support

**Date**: 2026-07-24
**Status**: accepted
**Deciders**: lune, Codex

## Context

Broad repository exploration consumes the Controller's context and slows
development, while FastContext community projects demonstrate that a small local
model can specialize in locating relevant files and line ranges. The original
Microsoft repository is unavailable and the paper is withdrawn for product IP
review, so community artifacts cannot be treated as an official product supply
chain. Existing wrappers also do not provide Loom's complete path, secret,
evidence, and authority boundaries.

## Decision

Loom's Codex development team includes a read-only `code_analyst` role with a
stable evidence contract. A pinned FastContext community model may implement
that role through an isolated, loopback-only local adapter after an explicit
Spike gate; the adapter exposes only Loom-owned `READ`, `GLOB`, and `GREP`,
validates paths and citations, and records trajectories outside the source
repository. Its output is Candidate evidence and FastContext is not a Phase 1
product dependency or state authority.

## Alternatives Considered

### Alternative 1: Let the primary Codex session explore every repository

- **Pros**: No additional runtime, model, or supply-chain dependency.
- **Cons**: Broad search consumes the main context and couples exploration noise
  to planning and synthesis.
- **Why not**: It remains the deterministic fallback, but is inefficient as the
  only path for unfamiliar or cross-module code.

### Alternative 2: Install one community MCP server unchanged

- **Pros**: Fastest apparent route to a working integration.
- **Cons**: Community projects vary in path containment, secret handling,
  network binding, trace location, dependency reproducibility, and maintenance.
- **Why not**: MCP transport does not supply Loom's governance or evidence
  boundary.

### Alternative 3: Make FastContext an autonomous solver or review authority

- **Pros**: More work could be delegated to a small local model.
- **Cons**: Retrieval-specialized output is not implementation, deterministic
  test evidence, or independent acceptance.
- **Why not**: It would expand a localization model beyond its verified role and
  violate the Controller/Developer/Reviewer separation.

## Consequences

### Positive

- Repository exploration can be local, bounded, reusable, and kept out of the
  Controller's main context.
- The backend can switch between Codex, llama.cpp, and MLX without changing the
  team role or evidence format.
- Model failures fall back to deterministic search and do not block product
  development.

### Negative

- Loom must own an adapter, validation layer, evaluation set, and local model
  lifecycle instead of relying on a single upstream package.
- Local inference consumes disk, memory, startup time, and maintenance effort.

### Risks

- Community weight provenance or licensing may remain unsuitable for
  redistribution; keep it experimental, pinned, local, and separately reviewed.
- A repository can contain path escapes, secrets, or prompt injection; enforce
  canonical root containment, secret denial, bounded tools, and zero source
  mutation outside model instructions.
- Token reduction may hide poor retrieval recall; promotion requires comparative
  Loom-specific evaluation rather than self-reported community benchmarks.
