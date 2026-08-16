# ADR-0017: Codex native conversation responder

**Date**: 2026-08-09
**Status**: accepted
**Deciders**: Product Owner, Loom Architecture Controller

## Context

Phase 2C made conversation the primary Loom workspace, and v0.4.2 made the
bundled local service start with the App. A remaining gap kept the first message
from working: Codex native authentication was visible in Runtime & Providers,
but the daemon created a conversation responder only when a separate local GGUF
model was configured. A logged-in Codex installation therefore still produced
"No conversation runtime is configured."

Provider status and conversation routing must describe the same usable product
path. A user who is already logged into Codex should be able to open Loom and
start a conversation without visiting settings.

## Decision

1. An explicitly configured local model remains the preferred conversation
   responder.
2. Otherwise, when Loom resolves a supported Codex executable, the daemon
   creates a Codex native-auth conversation responder automatically. Existing
   Codex login state is reused; Loom never reads or copies the credential.
3. Each response uses an ephemeral `codex exec` invocation in a private empty
   working directory, with a read-only sandbox, no approval escalation, no
   project rules, and all shell, unified execution, browser, App, Computer Use,
   image-generation, and multi-Agent tool surfaces disabled. A fixed
   environment allowlist, two-minute timeout, and bounded input/output apply.
4. The exact Codex executable and containing directory identities are checked
   before launch and after completion. Shells, dynamic arguments, inherited API
   keys, arbitrary working directories, and persistent Codex sessions are not
   permitted by this path.
5. The bounded conversation transcript is sent as untrusted content. The
   response is always marked tentative and cannot create a Team, Mission, Run,
   Grant, Journal fact, or side effect.
6. The client immediately presents the pending user message and a response
   progress state. It blocks duplicate sends and restores the draft if IPC or
   the Provider fails.

## Consequences

- A user with an existing Codex login can double-click Loom and converse
  immediately.
- Runtime discovery may still report an empty Pi model catalog; that no longer
  incorrectly disables the independent Codex conversation path.
- A user who is not logged in still receives the explicit Codex connection
  journey in Runtime & Providers.
- Pair-programming conversation remains non-authoritative. Agent Team creation
  and governed execution continue to require their explicit triggers and
  confirmations.

## Alternatives Considered

### Require a local GGUF model

Rejected. It makes the accepted Codex native-auth setup irrelevant to the
primary conversation experience and adds a large unrelated installation step.

### Make Swift invoke Codex directly

Rejected. It would bypass daemon IPC, duplicate Provider policy in the client,
and violate the authority boundary established by ADR-0012.

### Automatically run Codex login on first message

Rejected. Login is an explicit user-facing authentication action. Loom may
observe existing status and offer Connect, but it does not silently initiate
authentication.
