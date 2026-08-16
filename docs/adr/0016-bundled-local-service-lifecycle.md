# ADR-0016: Bundled local service lifecycle

**Date**: 2026-08-09
**Status**: accepted
**Deciders**: Product Owner, Loom Architecture Controller

## Context

ADR-0012 made the native client a thin UI over the shared daemon IPC and
correctly prohibited shell, CLI, `launchctl`, Provider, and Runtime process
control in the client. The Phase 2C local delivery nevertheless contained only
`Loom.app`'s Swift executable. Opening that bundle without a separately started
`loomd` produced a dead-end "Local service unavailable" screen.

The user should not need to know that `loomd` exists. App and service versions
must move together, and opening Loom must establish the local application
service before the first product read.

## Decision

The macOS product bundle contains the matching `loomd` executable and a
user-level LaunchAgent descriptor.

1. A Developer ID signed distribution uses `SMAppService` to register the
   bundled LaunchAgent. `launchd` owns its lifecycle and starts it immediately.
2. An ad-hoc signed local development delivery may not be accepted by
   `SMAppService`. While that App is open, it may start only its own bundled
   `loomd` using fixed arguments. The daemon receives the App PID and cancels
   itself when that parent exits.
3. Neither path invokes a shell, `launchctl`, a repository script, an external
   daemon path, or user-supplied process arguments.
4. The helper resolves only the current user's private Loom directories, the
   installed Pi Runtime, bounded Runtime identity, Node/Codex executable
   directories, and the standard
   `~/Library/Application Support/Loom/run/loomd.sock` endpoint.
5. The App waits for a real Socket and performs bounded IPC retries before
   presenting an unavailable state. Registration success alone is not treated
   as service health.
6. The daemon remains the sole Journal, Projection, policy, Scheduler, and
   execution authority. Bundling and lifecycle ownership do not move authority
   into Swift.

## Consequences

- Double-clicking `Loom.app` is the complete local startup journey.
- App and daemon bytes are code-signed as one delivery and cannot drift during
  an ordinary update.
- Local ad-hoc builds remain usable without weakening the signed public release
  path or installing an unreviewed legacy LaunchAgent.
- The fallback daemon is available only while its parent App is alive; a
  notarized public release must use the `SMAppService` path for resident use.
- Build and installer fixtures must validate the helper executable, LaunchAgent
  plist, modes, architecture, signature, fixed arguments, and default Socket
  contract.

## Alternatives Considered

### Require the user to start `loomd`

Rejected. It exposes an internal component and makes the normal GUI journey
fail before conversation begins.

### Let Swift call `launchctl` or a shell installer

Rejected. It violates ADR-0012, adds injection and path drift risk, and makes
the UI a service manager.

### Ship App and daemon as unrelated downloads

Rejected. Their IPC and authority contracts are versioned together; unrelated
installation permits incompatible combinations.
