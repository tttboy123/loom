# ADR-0010: Pi RPC translation behind the Loom bridge

**Date**: 2026-07-27
**Status**: accepted — Contract Review 8 PASS
**Deciders**: lune, Codex

## Context

Phase 1 engineering accepts `loom.bridge.v1` as the managed execution boundary,
but the reviewed installed Pi Coding Agent 0.82.1 does not speak that protocol.
It exposes a separate strict JSONL RPC protocol. Treating Pi RPC output as if it
were Bridge Frames bypasses run binding, sequence, Grant authorization, private
attempt capture, and terminal reconciliation.

The installed Pi metadata output also changed its no-model diagnostic from one
exact line to a three-line 0.82.1 form. Loom currently rejects that valid
diagnostic. Finally, the final live gate has no configured Provider and may not
use a credentialed or network model.

## Decision

Loom adds a Pi-0.82.1-specific RPC translation adapter behind the existing
`supervisor.RuntimeAdapter` interface. The adapter:

- accepts one closed, bounded Loom dispatch payload;
- launches the bound Pi executable in RPC mode with sessions, tools, project
  resources, extensions, Skills, templates, themes, and context files disabled;
- sends one correlated Pi `prompt` command;
- binds the locked Pi Agent-loop mapping in which Provider start becomes
  top-level assistant `message_start`, only text deltas become
  `message_update`, and Provider done/error becomes assistant `message_end`;
- translates only that allowlisted Pi lifecycle and assistant text stream into
  newly constructed `loom.bridge.v1` Ack, Event, Evidence, and Result Frames;
- passes every constructed Frame through the existing Supervisor-owned
  `FrameSink`, so Bridge binding, sequence, Grant authorization, private capture,
  observer ordering, and terminal reconciliation remain authoritative; and
- never passes a raw Grant, credential, Pi session, tool event, hidden reasoning,
  or untrusted Pi identity across the boundary.

The adapter reports type `pi-cli`, matching the installed Runtime discovery
fact. The accepted direct-Bridge Pi test adapter remains unchanged and is not
used by the final live gate. `loom.bridge.v1` itself is unchanged.

The final live gate uses one short-lived, locally managed llama.cpp server bound
only to `127.0.0.1`, a pre-downloaded and digest-verified Qwen GGUF, and an exact
private Pi `models.json`. Pi and llama.cpp both run in offline mode. Pi tools are
disabled, the model server Web UI and built-in tools are disabled, and neither
process becomes resident.

The local-model boundary takes an explicit current-user-owned 0700 private root;
it is not inferred from two leaf paths. One shared read-only inspector is used
by the pre-live manifest and server start. It validates lexical and resolved
containment, walks the directory chain without accepting symlink components,
requires private descendant directories to remain current-user-owned 0700,
opens leaves no-follow on Unix, matches descriptor/path identity, hashes the
opened files, and revalidates the full chain immediately before launch.

The no-model parser accepts only the legacy exact line or the exact bounded Pi
0.82.1 three-line diagnostic shape. Diagnostic paths are validated and
discarded; they do not become Runtime facts or evidence.

## Alternatives Considered

### Alternative 1: Modify Pi or require Pi to emit `loom.bridge.v1`

- **Pros**: Reuses the existing direct-Bridge adapter.
- **Cons**: Requires a fork or extension inside the untrusted Runtime and makes
  Loom binding semantics dependent on Runtime code.
- **Why not**: Loom must construct and authorize its own authority-bearing
  Frames at the trust boundary.

### Alternative 2: Treat Pi RPC JSON as Bridge Frames

- **Pros**: Minimal adapter code.
- **Cons**: Pi RPC lacks Loom WorkItem, Run, generation, Runtime, Agent, sequence,
  Grant, and terminal-ack semantics.
- **Why not**: It bypasses accepted Supervisor and ADR-0009 invariants.

### Alternative 3: Use a credentialed remote Provider for the final gate

- **Pros**: Higher model quality and no local inference installation.
- **Cons**: Requires credentials, network traffic, Provider authority, and
  potentially paid external work.
- **Why not**: The user authorized a controlled local offline model and the
  existing live manifest forbids network and external effects.

### Alternative 4: Run a persistent local model service

- **Pros**: Faster repeated runs.
- **Cons**: Introduces resident lifecycle, open local service duration, and
  additional operational authority.
- **Why not**: The final gate needs one bounded canary, not a daemon.

## Consequences

### Positive

- Installed Pi 0.82.1 can participate without changing the Bridge protocol.
- Existing Supervisor, Grant, capture, Evidence, and terminal authorities remain
  the only execution acceptance path.
- The live model is reproducible by exact binary and model digests and requires
  no credential.
- Pi text can be observed tentatively without publishing hidden reasoning or
  tool output.

### Negative

- Loom owns a version-specific RPC decoder and must review Pi RPC changes before
  upgrading Pi.
- The local model and llama.cpp assets consume disk and add a short-lived local
  process.
- Application-level offline and loopback controls are not an OS network
  sandbox; the gate therefore also disables every Pi resource/tool path and
  rejects non-loopback configuration.

### Risks

- A Pi RPC schema change could be misinterpreted. Unknown, duplicate-key,
  oversized, out-of-order, retry, compaction, extension-UI, tool, or error
  records fail closed.
- A stale executable/model could be substituted. Exact file bindings and
  SHA-256 digests plus explicit-root directory/leaf identity, owner, mode,
  symlink, and containment are revalidated before process start. The final OS
  exec and llama.cpp model handoff still reopen pathname strings because the
  current Go/llama.cpp interface is not descriptor-bound; the remaining
  same-user micro-race is disclosed and bounded by the 0700 tree, two
  validations, short lifetime, and fail-closed cleanup.
- A local process could reach the loopback server during its short lifetime.
  The server binds only `127.0.0.1`, has one slot, no UI/tools, a bounded
  lifetime, and is terminated after the canary.
- Model output is nondeterministic and untrusted. Only bounded valid UTF-8 text
  becomes tentative Event payload; terminal success still requires the existing
  Supervisor and Evidence lifecycle.
