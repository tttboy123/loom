# ADR-0011: TUI-first local product over versioned daemon IPC

**Date**: 2026-07-28
**Status**: accepted
**Deciders**: Product Owner, Loom Architecture Controller

## Context

Phase 1 proved Loom's local execution kernel, authority model, Runtime
observation, controlled Team execution, authorized output, Evidence, timeline,
and recovery through code, tests, and a controlled Pi live canary. Those
capabilities are currently exposed primarily through Go packages and finite CLI
commands. A normal local user still needs internal identifiers, database paths,
cursors, environment variables, or service-manager commands to complete common
work.

`TECH-PLAN.md` already assigns a Bubble Tea TUI and Credential Broker to Phase
2. ADR-0002 requires clients to use versioned daemon APIs and preserves the
Event Journal as the only state authority. ADR-0004 requires truthful
`brokered`, `provider_ephemeral`, and `native_auth` authentication modes.
ADR-0008 and ADR-0009 preserve stream-head CAS, immutable read views,
generation fencing, authorized tentative output, durable terminal facts, and
explicit recovery.

Phase 2A therefore needs a user-facing product boundary without creating a
second application, state store, scheduler, policy engine, or execution path.
It must also retain a finite CLI for headless automation, diagnosis, recovery,
and support.

## Decision

Loom's primary Phase 2A local product surface is a Bubble Tea TUI backed by a
private, versioned daemon IPC API.

The architecture is:

```text
TUI client ─┐
            ├─> versioned local daemon API ─> application services
CLI client ─┘                                  ├─> policy / grants
                                               ├─> StateWriter
                                               ├─> projections
                                               └─> Event Journal
```

The following rules are binding:

1. **TUI-first product surface.** Home, Runtime health, Team building,
   controlled execution, Runs, Evidence, History, Compare, Attention, approval,
   recovery, and provider onboarding are designed as user journeys in the TUI.
   A normal user must not need a terminal for those journeys.
2. **CLI remains operational.** The CLI remains supported for headless
   automation, deterministic fixtures, diagnosis, export, migration, recovery,
   and support. Product parity means that every ordinary user journey is
   available in the TUI; it does not require every low-level CLI flag or
   forensic operation to become a screen.
3. **One application and one authority.** TUI, CLI, and later clients call the
   same application services and therefore the same policy, Grant,
   StateWriter, Projection, and Event Journal boundaries. The TUI must not
   parse CLI output, issue shell commands as an application protocol, write
   SQLite directly, or maintain an authoritative client-side cache.
4. **Private versioned IPC.** Local clients communicate with the resident
   daemon through a bounded, versioned Unix-domain-socket API on the controlled
   macOS product path. The socket, parent directory, request size, response
   size, cursors, timeouts, peer access, and error mapping are fail-closed.
   Phase 2A does not expose a public TCP listener.
5. **Read state is replaceable.** A TUI model may hold immutable copied view
   data and tentative in-memory deltas for rendering. Durable state, reconnect,
   warnings, retries, degraded states, terminal facts, and Evidence resolve
   from the Journal and rebuildable projections.
6. **Commands are explicit.** Mutating actions use typed application commands,
   preview their authority and compatibility effects, require the applicable
   confirmation or approval, and surface canonical conflict/rejection results.
   Client reconnect never silently resubmits a command.
7. **Streaming is non-authoritative.** Authorized text deltas are visibly
   tentative. Slow consumers may receive coalesced text and a `stream_gap`
   marker, but warning, retry, degraded, blocked, human-required, approval, and
   terminal milestones cannot be silently discarded. Terminal output becomes
   authoritative only through accepted Evidence and Journal facts.
8. **Authentication is truthful and secret-safe.** Provider onboarding reports
   the actual ADR-0004 authentication mode. Brokered secrets live in the OS
   Secret Store and are resolved only for a bounded Run; native Codex OAuth is
   observed as native authentication, not copied into Loom state. Secrets do
   not enter source, command arguments, prompts, logs, Journal facts, Evidence,
   Agent definitions, screenshots, or TUI state snapshots.
9. **User-level packaging.** Phase 2A owns a user-level installation and
   resident-daemon lifecycle suitable for the local product. Installation,
   status, start, stop, restart, upgrade compatibility, and uninstall/recovery
   operations remain inspectable through operational tooling. Packaging does
   not grant autonomous execution.
10. **Three vertical WorkItems only.** Phase 2A is delivered by exactly
    P2A-W1, P2A-W2, and P2A-W3 as frozen in the Phase 2A Exit Contract. Thin
    shell, adapter, writer, coordinator, screen, or button WorkItems are
    forbidden.

## Alternatives Considered

### Alternative 1: Keep the CLI as the primary product

- **Pros**: Lowest implementation cost and strong scriptability.
- **Cons**: Requires users to know internal identifiers, environment setup,
  cursors, database paths, and service commands; does not deliver a local
  product experience.
- **Why not**: It fails Phase 2A's ordinary-user journey and the existing
  Phase 2 TUI commitment.

### Alternative 2: Build the TUI by invoking and parsing CLI commands

- **Pros**: Reuses current command binaries with little initial refactoring.
- **Cons**: Creates an unstable text protocol, duplicates error interpretation,
  complicates cancellation and streaming, and risks divergent authority
  behavior.
- **Why not**: CLI output is a presentation, not an application API.

### Alternative 3: Let the TUI read or write SQLite directly

- **Pros**: Simple local reads and low apparent latency.
- **Cons**: Bypasses application policy, CAS, grants, migrations, projections,
  and the one-writer boundary; couples presentation to storage.
- **Why not**: It violates ADR-0002 and the Event Journal authority invariant.

### Alternative 4: Start Phase 2A with a browser or cloud service

- **Pros**: Familiar graphical interaction and remote access.
- **Cons**: Adds deployment, public-network, identity, multi-user, and cloud
  synchronization boundaries before the local product is closed.
- **Why not**: Web, cloud synchronization, and multi-user delivery are outside
  Phase 2A.

## Consequences

### Positive

- Ordinary local workflows no longer require terminal knowledge.
- CLI automation remains stable without becoming the product's interaction
  model.
- All clients share one policy and state-transition path.
- The daemon API becomes a reusable boundary for later desktop or web clients
  without making those clients state authorities.
- Restart and reconnect behavior is derived from durable facts instead of
  client memory.

### Negative

- The resident daemon gains a security-sensitive local IPC surface.
- TUI state machines, focus, accessibility, terminal capability, cancellation,
  and slow-consumer behavior require dedicated deterministic testing.
- Existing application operations may need typed service facades instead of
  relying on CLI wiring.
- macOS Keychain and user-level service packaging add platform-specific
  integration work while the core model remains portable.

### Risks

- **IPC becomes a second command authority.** Mitigation: handlers only decode,
  validate, authorize, and call existing typed application services.
- **TUI caches become stale or authoritative.** Mitigation: immutable view
  versions, canonical cursors, reconnect refresh, and replaceable copied state.
- **Secrets leak through display or diagnostics.** Mitigation: opaque secret
  references, redacted structured errors, screenshot-safe rendering, and
  explicit negative tests.
- **Scope expands into one screen per internal primitive.** Mitigation: the
  frozen three-WorkItem Exit Contract and journey-based acceptance.
- **Resident service implies autonomy.** Mitigation: observation and IPC may be
  resident, but execution still requires explicit triggers, policy, grants,
  approvals, and generation-fenced authority.
