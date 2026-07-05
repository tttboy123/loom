# Loom Protocol Layer (Phase C + E)

> **Status**: implemented in `devkit/protocol.py`.
> **Date**: 2026-07-05.
> **Scope**: in-process A2A + MCP stub. HTTP / WebSocket transports
> are explicitly out of scope for Phase C (see ADR 0015).
> **See also**: [ADR 0015](architecture-decisions/0015-phase-e-unify-and-migrate.md).

## What this layer is

`devkit/protocol.py` exposes two protocol-shaped surfaces in a single
module:

1. **A2A** — agents declare an `AgentCard` (identity + capability list
   + endpoint) and exchange `AgentMessage` envelopes. `send_message`
   routes to the target agent's registered handler via an in-process
   callable registry. **No** HTTP, **no** asyncio, **no** websockets —
   those are out of scope for Phase C.
2. **MCP** — tools (`ToolDescriptor`) and resources
   (`ResourceDescriptor`) are declared in the same shape MCP clients
   expect. `invoke_tool(name, arguments)` and `read_resource(uri)`
   look up handlers in in-process callable registries. The five
   `loom://` resources and four `loom.*` tools are auto-registered so
   external clients (VSCode, Claude Desktop, …) can introspect Loom
   state out of the box.

All mutations on the in-memory registries are protected by
`threading.Lock` so the server can be embedded in the existing
single-process Loom daemon without races. Schema validation is
delegated to `jsonschema.Draft202012Validator` against the schema JSON
files in `devkit/protocol_schemas/`.

## Surface

### Dataclasses (the boundary objects)

- `AgentCard` — A2A agent identity (id, name, capabilities, endpoint).
- `AgentMessage` — A2A envelope (kind, message_id, from_agent,
  to_agent, spec, correlation_id).
- `ToolDescriptor` / `ToolInvocationResult` — MCP tool metadata +
  result.
- `ResourceDescriptor` / `ResourceReadResult` — MCP resource
  metadata + read result.
- `MessageDeliveryResult` — return shape of `send_message`.

### Public functions

- `validate(kind, payload)` — JSON-Schema validation against the
  matching schema in `devkit/protocol_schemas/`.
- `register_default_agents(server)` — registers the three Loom A2A
  agents (`observer`, `triager`, `repairer`); idempotent.
- `write_run_protocol_bundle(run_dir, run_id, objective, *, ...)` —
  single canonical writer of `protocol_bundle.json` (Draft 2020-12
  envelope, atomic, fail-open).
- `default_server()` / `reset_default_server()` — process-wide
  singleton (lazily constructed on first call).

### `ProtocolServer` methods

- A2A: `register_agent`, `unregister_agent`, `list_agents`,
  `get_agent`, `on_message(agent_id)(handler)`, `send_message`.
- MCP tools: `register_tool`, `list_tools`, `invoke_tool`.
- MCP resources: `register_resource`, `list_resources`,
  `read_resource`, `snapshot`.

The constructor accepts `auto_register_loom=True` (default) and
`auto_register_agents=True` (default). Tests that want a clean slate
pass both as `False`.

## Default agents (registered on `default_server()`)

| Agent id | Capabilities | Handler body |
|---|---|---|
| `observer` | `backlog.read`, `runs.read`, `evidence.read`, `events.read` | `devkit.observer.snapshot(**body)` |
| `triager` | `incident.classify`, `findings.write`, `backlog.read` | `devkit.triager.triage(snap)` |
| `repairer` | `incident.dispatch`, `repair.execute`, `backlog.read` | `devkit.repairer.dispatch(incident, **body)` |

Each handler uses lazy imports — loading the server does not load the
agent modules. This keeps `from devkit import protocol` cheap.

## Hot-path policy

**Internal callers — same process — go direct.**

Inside the Loom daemon, code that needs to:

- **Observe** state → calls `devkit.observer.snapshot(...)`
  directly. Not `server.send_message(observer, {...})`.
- **Triage** an incident → calls `devkit.triager.triage(snap)`
  directly. Not `server.send_message(triager, {...})`.
- **Dispatch repair** → calls `devkit.repairer.dispatch(incident,
  **body)` directly. Not `server.send_message(repairer, {...})`.

The `ProtocolServer` A2A / MCP wrapping exists for **external**
callers — future cross-process scenarios, an external MCP client
(Claude Desktop, VSCode), or a sibling Loom cluster member. Wrapping
an in-process call adds JSON envelope serialization + handler lookup +
lock acquisition, all of which is wasted work for an in-process
caller.

When the wrapping **is** justified:

- An external process wants to read `loom://backlog` → `server.read_resource(uri)`.
- A future cross-process repair dispatcher wants to send an A2A
  message → `server.send_message(to_agent=repairer, ...)`.
- An MCP client wants to invoke `loom.dispatch_incident` →
  `server.invoke_tool(...)`.

The decision rule: **is the caller in the same Python process?** If
yes, call the agent / tool / resource function directly. If no, go
through `ProtocolServer`.

See ADR 0015 for the architectural rationale (the protocol layer was
introduced in Phase C for interop; internal hot paths were never
intended to round-trip through it).

## Atomic writes and schema validation

Every file this layer writes is atomic (`.tmp + os.replace`) and
schema-validated:

- `evidence_packet.json` (via `evidence_writer.write_run_evidence`).
- `verdict.json` (via `gatekeeper.write_verdict`).
- `protocol_bundle.json` (via `write_run_protocol_bundle`).
- `lease.json` (via `scheduler.claim_lease`).
- `auto-lease.json` (via the autopilot outer loop).

Failures are logged via `devkit.protocol` / `devkit.gatekeeper`
loggers — never silently swallowed. The `try/except Exception: pass`
wrappers in `rdloop.py` are a separate (legacy) concern, not part of
this layer.

## Verified end-to-end (Phase E smoke test, 2026-07-05)

```text
[1] ProtocolServer(auto_register_loom=True, auto_register_agents=True)
    → list_agents() returns observer + triager + repairer.
[2] send_message(to_agent=observer, spec_kind=snapshot) returns
    a payload with the same shape as observer.snapshot().
[3] read_resource(loom://backlog) returns the backlog summary.
[4] write_run_protocol_bundle(run_dir, run_id, objective) writes
    a Draft 2020-12-valid envelope; target is atomic.
```

All 410+ tests pass (cumulative across Phase A–F).

## Known limitations

1. **No HTTP / WebSocket transport.** Out-of-process callers cannot
   reach `ProtocolServer` directly today. Phase F may add a
   thin FastAPI/uvicorn shim.
2. **No agent-to-agent handoff persistence.** `send_message` returns
   the result; it does not log to the event log. A future patch can
   wire `MessageDeliveryResult` into `devkit/events.jsonl`.
3. **Single-process lock semantics.** The `threading.Lock`s are
   in-process; cross-process coordination is the lease layer's
   responsibility.