# ADR-0007: Discovery-priority Runtime observation writes

**Date**: 2026-07-25
**Status**: accepted
**Deciders**: lune, Codex

## Context

One automatic Runtime observation can contain both inventory changes and status
changes. `RuntimeInstanceDiscovered` and `RuntimeInstanceStatusChanged` append
to the same per-Runtime stream and consume caller-authoritative Event
sequences. Silently invoking both accepted writers for one observation would
create ambiguous ordering, stale previous-Event references, or a second
sequence authority.

Runtime absence is also not proof that an installed Runtime is offline. A probe
can be unavailable, omitted, disabled, or temporarily unable to observe an
instance.

## Decision

Before any write, Loom classifies each complete accepted observation cycle as
exactly one of `none`, `discovery`, or `status`.

- Any new Runtime or change to observed non-status inventory makes the whole
  non-empty observation a `discovery` write.
- Inventory means display name, executable version, observed capabilities,
  capacity, model inventory, or source probe. Stable device/adapter identity
  drift is an error, not rediscovery.
- `discovery` takes precedence when the same observation also contains status
  changes. The cycle never appends both Event types.
- Only when inventory is unchanged and at least one present Runtime status
  changed does the cycle become a `status` write.
- An unchanged observation is `none`.
- Missing projected Runtimes are ignored for write selection. Empty or partial
  observation never fabricates offline, removed, disabled, incompatible, or
  status-changed facts.

The classification is a deterministic Candidate, not write authority.
Schedulers, daemon lifecycle, Event metadata preparation, retries, and the
actual writer invocation remain separate bounded WorkItems.

## Alternatives Considered

### Alternative 1: Allocate shared sequences and split one cycle into both writes

- **Pros**: Inventory and status changes retain separate Event types.
- **Cons**: Requires a new cross-writer sequence allocator, ordering contract,
  and atomic coordination boundary.
- **Why not**: It creates another authority before Phase 1 proves the simpler
  single-writer cycle and substantially enlarges the failure surface.

### Alternative 2: Always write discovery

- **Pros**: One simple path and every observation is a complete snapshot.
- **Cons**: Repeated status-only observations erase status provenance and emit
  unnecessary inventory Events.
- **Why not**: Accepted status reconciliation and status projection would no
  longer govern status-only changes.

### Alternative 3: Status takes precedence

- **Pros**: Preserves a compact status history.
- **Cons**: New or changed inventory could be omitted while status advances.
- **Why not**: Runtime compatibility and model/capability visibility require
  current inventory before later binding or execution decisions.

## Consequences

### Positive

- One observation has one unambiguous writer and no competing sequence owner.
- Inventory is current before status-only optimization is allowed.
- Absence remains fail-closed and cannot silently disable a Runtime.
- Later scheduling can consume a small deterministic plan without owning
  classification rules.

### Negative

- Mixed inventory/status observations are represented only by discovery Events;
  rediscovery intentionally resets status-event provenance to the new complete
  observation.
- A full non-empty observation is required for safe discovery-priority writes.

### Risks

- An unstable probe could cause repeated discovery writes by changing inventory
  fields. Canonical Runtime validation, deterministic comparison, and bounded
  probe contracts mitigate this.
- A future need for separate same-cycle Events would require a superseding ADR
  with one explicit sequence and atomicity authority.
