# ADR-0004: Credential Broker and authentication modes

**Date**: 2026-07-24
**Status**: accepted
**Deciders**: lune, Codex

## Context

Provider APIs authenticate credentials issued by the Provider or an officially trusted identity system. A token generated only by Loom cannot be sent directly to an arbitrary Provider. Some Agent CLIs also use native subscription login that Loom cannot replace or inspect.

## Decision

Loom supports three explicit authentication modes: `brokered`, `provider_ephemeral` and `native_auth`. In brokered mode an Agent presents a Run-scoped local Grant to Loom, and the Credential Broker resolves the real secret and proxies the Provider request; the UI and Evidence always report the actual mode.

## Alternatives Considered

### Alternative 1: Inject raw Provider Keys into every Agent

- **Pros**: Broad compatibility and direct Provider access.
- **Cons**: Agents and child processes can read, log or transfer long-lived credentials.
- **Why not**: It contradicts the promised credential isolation boundary.

### Alternative 2: Use a Loom token directly with every Provider

- **Pros**: Uniform task-scoped token semantics.
- **Cons**: Providers do not trust or validate Loom-issued tokens.
- **Why not**: It is technically invalid unless Loom is the verifying proxy or the Provider supplies an official exchange.

## Consequences

### Positive

- Brokered Agents do not receive raw Provider credentials.
- Native CLI authentication remains supported without overstating its isolation level.

### Negative

- Brokered mode must implement streaming, errors, usage accounting and Provider protocol compatibility.
- Not every Agent CLI can redirect requests to the Broker.

### Risks

- A compromised Broker has high-value access; mitigate with OS Secret Store integration, loopback or UDS binding, hashed Grants, strict logs and immediate revocation.
