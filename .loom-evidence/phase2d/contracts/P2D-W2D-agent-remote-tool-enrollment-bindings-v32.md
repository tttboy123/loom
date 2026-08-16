# P2D-W2D Contract V32: ExecutionProfile, Agent Enrollment Selection, Preflight, Frozen Attempt Binding, Work Bundle Materialization

Status: `ACTIVE / PARTIAL`

Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

Version line: `v32`

Authority: P2D-W2D, P2D-W2C Agent Attempt binding and dispatch, V30 persisted
Remote Tool Backend Enrollment, V31 Enrollment client governance, and Product
Owner confirmation on 2026-08-16.

## Objective

Close the authoritative per-Agent remote-tool chain so that an ExecutionProfile
may bind exactly one Provider Account-scoped Remote Tool Backend Enrollment,
per-Agent preflight validates that binding before any Attempt, the frozen
Attempt binding and replay/retry reject enrollment drift, and a trusted
built-in backend candidate registry plus runtime materialization boundary is
the only path from a persisted Enrollment to a typed Work Bundle client port.

Default production still publishes no remote tool capability. No App bundle,
network request, external MCP process, Provider, real credential, user
workspace or installed-live acceptance is performed by this slice.

## Required chain

`ExecutionProfile (per-Agent) -> Agent-selected Enrollment -> Preflight ->
Frozen Attempt Binding -> Work Bundle materialization`

1. **ExecutionProfile**: the per-Agent ExecutionProfile (runtime profile +
   frozen binding) may carry an optional, both-or-neither
   `remote_tool_enrollment_id` / `remote_tool_enrollment_digest` pair. The pair
   participates in the frozen binding digest (a new explicit digest domain) so
   replay, retry, recovery and fallback reject silent enrollment drift.

2. **Agent-selected Enrollment**: a per-Agent selection freezes the exact
   authoritative Enrollment identity and digest under the same Provider
   Account as the Agent. The setup builder accepts `none` or
   `<enrollment_id>:<digest>`, validates the selection against the current
   authoritative projection (active, policy-current, account match, digest
   match, adapter in the trusted catalog), and otherwise fails closed.

3. **Preflight**: per-Agent preflight validates the selected Enrollment from
   the authoritative read view before an Attempt may be dispatched. Revoked,
   policy-drifted, account-mismatched, digest-conflicted, adapter-unsupported
   or unavailable enrollments block only that Agent with a closed safe
   diagnostic (code, stage, retryability, Incident correlation). One failed
   enrollment never marks the Team or peer Agents offline.

4. **Frozen Attempt Binding**: the FrozenExecutionBinding and the content-free
   per-Agent enrollment binding (enrollment ID, enrollment digest, backend
   kind, adapter, policy lineage, allowlist digest, limits, binding digest)
   are frozen together. Journal facts, replay, projection and the Board carry
   only these safe records; tool names and enrollment input never enter them.

5. **Work Bundle materialization**: a trusted built-in backend candidate
   registry enumerates the only materializable Adapter IDs
   (`builtin.search.deepseek.v1`, `builtin.mcp.stdio.v1`). A materialization
   boundary consumes a persisted, active, policy-current Enrollment and the
   injected typed ports (SearchBackend / MCPToolClient) and returns an
   `execution.RemoteToolExecutor` with the exact allowlist and bounded limits.
   Unknown adapters, revoked records, drifted policy, missing ports or invalid
   limits fail closed before any call can be made.

## Deliberate exclusions

- No new backend creation form; the Swift UI still has no arbitrary Adapter ID,
  raw endpoint, dynamic plugin or hot-replacement path.
- No real network Search or MCP transport is added; materialization still
  requires injected typed ports that default production does not provide.
- No installed-live Web/MCP, mixed-Team ATL9, accounting UI completion or
  per-Agent failure isolation matrix execution. Those remain Phase 2D exit
  gates.
- No secret, Authorization header, Prompt, Provider response, query, MCP
  argument, result body, tool name list or unbounded stdout/stderr enters the
  Journal, operational diagnostics, capsule, preflight record or Board.

## Acceptance

1. RED tests fail on missing symbols/behaviour for the enrollment binding
   authority, runtime profile/binding digest, materialization boundary and
   per-Agent preflight before implementation.
2. Go focused, package, race, repository, vet, format and diff checks pass.
3. The complete Go repository suite and the complete macOS package suite pass
   (no new regressions beyond the recorded pre-existing full-load harness
   flake).
4. The per-Agent enrollment preflight blocks revoked / drifted / mismatched /
   conflicted / unsupported / unavailable enrollments with safe closed codes
   and never blocks peer Agents.
5. Default production composition remains remote-tool unavailable.

Evidence: `../P2D-W2D-agent-remote-tool-enrollment-bindings-v32.md`.
