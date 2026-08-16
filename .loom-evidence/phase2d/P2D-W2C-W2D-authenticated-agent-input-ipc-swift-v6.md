# P2D-W2C/W2D Authenticated Agent Input IPC and Swift Governance V6

Status: `SOURCE VERIFIED / INSTALLED LIVE OPEN`

Date: 2026-08-14

## Result

The durable encrypted Agent Inbox now has one authenticated product admission
path and one native per-Agent control surface. The private UDS route is
`agent_input`; its request ID is the Incident ID and deterministic idempotency
anchor. The client supplies only the public active identity, mode, context
scope, and bounded UTF-8 bytes. The daemon resolves the exact active Attempt,
freezes the authoritative target, order, input/payload IDs, Execution Binding
and Capsule lineage, then commits through the existing Inbox coordinator.

The daemon never accepts client-supplied internal binding, order, Turn, Step,
payload, or authority IDs. Same-Incident replay with the same identity and
content is idempotent; content, mode, identity, generation, or target
substitution fails closed. Request plaintext is cleared on every return path.

## Composition and lifecycle

Mission construction now owns one shared Attempt Loop authority, Agent Inbox
coordinator, and active-Attempt registry for the mission Bundle lifetime. The
same instances are injected into per-run executors and the authenticated IPC
route. This closes the detached-registry failure mode where the runtime could
register an Attempt in a different in-memory projection from the ingress.

The `loom-agent-runtime` typed route manifest publishes `agent_input` only when
the encrypted Inbox capability is present. Recovery, unbound, closed, stale,
terminal, and generation-drift states fail closed.

## Swift governance

Swift has a strict request and content-free receipt contract. `Data` is encoded
as base64 on the wire, receipt decoding rejects unknown fields, and the caller
freezes one Incident ID across client diagnostics and daemon admission.

Mission center renders controls only for an exact active Board Agent whose
snapshot/timeline generation, Run, WorkItem, Agent, claim generation, Execution
Binding, Capsule, and Route Segment agree. Queue, Steer, and Inject are a
segmented per-Agent choice. Draft, in-flight state, receipt, and failure remain
isolated per Agent. Failures retain the draft and show a safe stage, recovery
action, and copyable Incident ID.

App diagnostics persist only Incident, Segment/Agent/WorkItem/Run/generation,
mode, stage, elapsed time, result, controlled error code, and retryability. A
privacy regression proves the input body and the `content` field do not enter
`app-operational.jsonl`. Daemon diagnostics use the existing content-free Agent
Attempt record and the new closed `agent_input_admission` stage.

## Verification

- Focused Go Agent Inbox/route/real authenticated UDS/diagnostic race matrix:
  passed for 10 runs.
- Focused Swift operational diagnostics: 7 passed.
- Full Swift package: 205 XCTest cases passed, one intentional visual-audit
  skip, plus 10 Swift Testing contracts passed.
- `go vet ./...`: passed.
- `go test -p 1 ./... -count=1`: passed.
- The first parallel full-Go run correctly failed because the explicit
  Go-driven `swiftc` probe omitted the new Agent Input model file. The source
  list was repaired and the exact probe plus full serial repository run passed.
- That same pressure run observed one Codex harness child-start timeout. The
  exact harness cancellation test passed 10 isolated runs and the full serial
  repository run; no production relaxation was made.

## Open gates

This is not installed or live acceptance. Crash between Inbox authority and
Runtime consumption, Pi/Codex/Claude Queue/Steer/Inject consumption, installed
App interaction, real Provider calls, CV6, ATL9 mixed-Team failure isolation,
and COMP2-E remain open. No App, network, real credential, Provider, user
workspace, or external Runtime was accessed for this result.
