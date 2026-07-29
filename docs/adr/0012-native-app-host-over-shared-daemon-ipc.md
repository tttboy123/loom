# ADR-0012: Native app host over the shared daemon IPC

**Date**: 2026-07-28
**Status**: accepted
**Deciders**: Product Owner, Loom Architecture Controller

## Context

ADR-0011 correctly makes the local product TUI-first and requires ordinary
journeys to work without terminal knowledge. P2A-W1 implemented the Bubble Tea
client and shared daemon IPC, but its installed `Loom.command` necessarily
opened macOS Terminal. The only governed live canary proved every non-UI gate,
then failed because the controlled Computer Use surface cannot read or operate
Terminal. CLI or PTY evidence cannot substitute for a real ordinary-user
product window.

A historical `Loom Cockpit.app` and source snapshot exist outside this
repository. They prove that a native SwiftUI shell is feasible, but their
transport shells out to bridge scripts, discovers a repository workspace, and
contains a persistent client cache. Those choices conflict with ADR-0011 and
the current single-authority contract.

## Decision

The default macOS Phase 2A launch surface is a user-level native `.app` that
calls the same private, versioned daemon Unix-domain-socket API directly.
Bubble Tea remains the equivalent text-mode product client for SSH, recovery,
diagnosis, and terminal-oriented users; it is not the mechanism used by the
native app.

The native app:

1. implements the bounded `localipc` v1 frame and JSON schema directly;
2. calls only the existing `ping`, `snapshot`, and `timeline_page` methods in
   P2A-W1;
3. holds only replaceable in-memory copied view state;
4. never invokes Loom CLI, shell, bridge script, `launchctl`, Provider, or
   Runtime process;
5. never reads SQLite or a repository workspace;
6. never creates a second cache, Journal, queue, scheduler, policy, writer, or
   command authority;
7. derives selection from user-visible records and never requires a typed Team
   ID, cursor, socket path, or database path;
8. is installed as an independently addressable macOS application window so
   accessibility and Computer Use can verify the ordinary-user journey.

This refines ADR-0011 rules 1 and 9; its one-authority, typed-IPC, CLI,
authentication, streaming, and three-WorkItem rules remain accepted.

## Alternatives Considered

### Alternative 1: Accept CLI or PTY evidence for the Terminal-hosted TUI

- **Pros**: No additional product code.
- **Cons**: Does not prove the ordinary user journey and contradicts the
  no-terminal product objective.
- **Why not**: The failed live gate already proved this evidence substitution
  would be untruthful.

### Alternative 2: Embed or parse Bubble Tea terminal output in a native shell

- **Pros**: Reuses rendered text and navigation logic.
- **Cons**: Adds terminal emulation or ANSI parsing as a second presentation
  protocol and couples the app to terminal behavior.
- **Why not**: The daemon IPC is the accepted application protocol; rendered
  terminal output is not.

### Alternative 3: Import the historical Loom Cockpit implementation

- **Pros**: Existing SwiftUI views, tests, and app-bundle precedent.
- **Cons**: Its `Process` bridge, workspace discovery, runtime-host scripts,
  and persistent cache predate the current Journal/Projection/IPC authority
  boundary.
- **Why not**: Reusing that transport or state model would violate ADR-0011.
  Visual layout ideas may be referenced, but product code is rewritten against
  the current typed API.

### Alternative 4: Start a browser or local HTTP product

- **Pros**: Familiar UI and easy automation.
- **Cons**: Adds a public-network-shaped boundary, browser lifecycle, and later
  cloud assumptions during a local-only phase.
- **Why not**: Web and cloud surfaces remain explicitly outside Phase 2A.

## Consequences

### Positive

- The normal macOS user launches Loom without Terminal.
- Computer Use and accessibility operate an independently addressable product
  window.
- Native and Bubble Tea clients share the same application API and authority.
- The existing Go daemon, projection, policy, and Journal remain unchanged.

### Negative

- W1 gains a small platform-specific Swift package and bundle build path.
- The local IPC schema must have matching Go and Swift compatibility tests.
- Read-screen presentation exists twice and requires shared acceptance
  fixtures to prevent semantic drift.

### Risks

- **The native app becomes a second authority.** Mitigation: read-only W1
  methods, no persistent client cache, and static checks excluding process,
  SQLite, bridge, and StateWriter access.
- **Swift protocol decoding diverges.** Mitigation: exact framing bounds,
  duplicate/unknown-field rejection, deterministic Go-server/Swift-client
  component tests, and version mismatch closure.
- **Historical Cockpit code leaks old architecture into W1.** Mitigation:
  historical sources are evidence/reference-only; imported transport, cache,
  workspace, and runtime-host code is contractually forbidden.
- **A native window expands Phase 2A governance.** Mitigation: it remains
  inside P2A-W1; P2A-W2 and P2A-W3 are unchanged and no W4 exists.
