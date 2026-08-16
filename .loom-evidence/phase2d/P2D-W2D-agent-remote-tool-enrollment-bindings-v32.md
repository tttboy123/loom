# P2D-W2D Agent Remote Tool Enrollment Bindings V32

Status: `SOURCE VERIFIED / EXECUTION PROFILE ENROLLMENT SELECTION, PREFLIGHT,
FROZEN ATTEMPT BINDING AND WORK BUNDLE MATERIALIZATION BOUNDARY PRESENT /
INSTALLED LIVE OPEN`

Date: 2026-08-16

## Acceptance boundary

V32 closes the authoritative per-Agent remote-tool chain:

`ExecutionProfile -> Agent-selected Enrollment -> Preflight -> Frozen Attempt
Binding -> Work Bundle materialization`

It does not construct a real network Search/MCP transport, publish a remote
capability, run an installed App, use a Provider, real credential, external MCP
process or user workspace, or perform installed-live Web/MCP, mixed-Team ATL9
or failure-isolation-matrix acceptance. Default production still publishes no
remote tool capability.

## Implemented source

### ExecutionProfile and Frozen Attempt Binding (`internal/runtime`)

- `RuntimeProfile` and `FrozenExecutionBinding` carry an optional
  both-or-neither `RemoteToolEnrollmentID` / `RemoteToolEnrollmentDigest` pair.
  Either alone or a malformed digest fails `FreezeExecutionBinding`.
- The enrollment pair participates in a new explicit digest domain
  `loom.frozen-execution-binding.v3` (used whenever an Enrollment is bound), so
  replay, retry, recovery and fallback reject silent Enrollment drift. The v2
  reasoning-effort and legacy digest domains are preserved for profiles without
  an Enrollment.
- `ValidateFrozenExecutionBinding` round-trips the Enrollment pair; mutating
  the digest or ID after freeze fails validation.

### Agent-selected Enrollment (`internal/work`, `internal/app` setup)

- `work.AgentRemoteToolEnrollmentSelection` plus
  `work.FreezeAgentRemoteToolEnrollment` is the pure authority that freezes a
  content-free per-Agent Enrollment binding: Enrollment ID and digest, backend
  kind, adapter, Provider Account, policy lineage, allowlist digest, bounded
  limits and a deterministic binding digest. Revoked, policy-drifted,
  account-mismatched, digest-conflicted, adapter-unsupported, not-found and
  invalid inputs each fail closed with a distinct sentinel.
- `work.RemoteToolBackendCatalog` / `work.BuiltInRemoteToolBackendCatalog` is
  the trusted built-in backend candidate registry. It lists exactly
  `builtin.search.deepseek.v1` (web_search) and `builtin.mcp.stdio.v1`
  (mcp_server); any other Adapter ID is rejected at preflight and
  materialization.
- The builder adds `main_remote_tool_enrollment` /
  `subagent_remote_tool_enrollment` edits accepting `none` or
  `<enrollment_id>:<digest>`. Selection validates the authoritative projection
  (account match, active, policy-current, digest match, adapter in the trusted
  catalog) before freezing the pair into a new role profile. A Provider Account
  route change clears the prior Enrollment so a stale cross-account binding
  cannot survive.

### Preflight (`internal/app`)

- `resolveMissionExecutionEnrollment` validates the selected Enrollment from
  the authoritative read view during per-Agent preflight. Closed block codes:
  `tool_enrollment_incomplete`, `tool_enrollment_unavailable`,
  `tool_enrollment_revoked`, `tool_enrollment_policy_drift`,
  `tool_enrollment_revision_conflict`, `tool_enrollment_adapter_unsupported` —
  each with a safe reason, stage and retryability. Only the affected Agent
  blocks; peers and the Team are never relabelled offline.
- Preflight freezes the `work.FrozenAgentRemoteToolEnrollment` and projects
  `remote_tool_enrollment_available`, `enrollment_id`, `backend_kind` and
  `binding_digest` onto the role binding and the Board node preview (digest
  only; no tool names or Enrollment input).

### Work Bundle materialization (`internal/toolbroker/enrollment`)

- `Materialize` is the only boundary that builds a typed
  `execution.RemoteToolExecutor` from a persisted Enrollment. It requires a
  valid, active, policy-current Enrollment whose Adapter ID is in the trusted
  catalog with the matching backend kind, plus the injected typed ports
  (`SearchBackend` / `MCPToolClient`). Unknown adapters, revoked records,
  drifted policy, missing ports, kind mismatch and invalid limits fail closed
  before any call can be made.
- The materialized executor carries the exact MCP allowlist and bounded limits
  from the Enrollment. Default production supplies no ports, so no capability
  is ever exposed without an explicit trusted transport.

### Swift client governance

- `LocalProductSetupRoleOption`, `LocalProductBuilderRole`,
  `LocalProductBuilderExecutionRoute` and `LocalProductExecutionNodePreview`
  strictly decode the new non-secret fields, reject unknown keys and enforce
  the both-or-neither Enrollment pair.
- `LocalProductStore` admits the new builder edit fields and exposes
  `editBuilderRemoteToolEnrollment` / `clearBuilderRemoteToolEnrollment`.
- The native Agent editor adds a "Remote tools" picker bound to the Agent's
  Provider Account: active + policy-current Enrollments or "No remote tools".
  It sends only `<enrollment_id>:<digest>` or `none`; it never constructs an
  Adapter, endpoint, plugin or secret.

## Deliberate exclusions

- No arbitrary Adapter ID, raw endpoint, remote Bundle URL, dynamic plugin or
  hot-replacement path is added to any surface.
- No real Search or MCP transport is added; materialization still requires
  injected typed ports. Installed Web/MCP execution, mixed-Team ATL9, complete
  accounting UI and the per-Agent failure isolation matrix remain Phase 2D exit
  gates.
- No key, Authorization header, Prompt, Provider response, query, MCP argument,
  result body, tool name list, endpoint fingerprint or unbounded stdout/stderr
  enters the Journal, operational diagnostics, preflight record, Board or
  capsule.

## Verification

Passed:

- `go test ./internal/runtime ./internal/work ./internal/toolbroker/...
  ./internal/state ./internal/projection ./internal/app` (count=1).
- `go test -race ./internal/runtime ./internal/work ./internal/toolbroker/...
  ./internal/state ./internal/projection` and focused `internal/app` race.
- `go test ./cmd/loomd` complete daemon suite.
- `go vet` on all touched packages; `gofmt -l` clean on touched files;
  `git diff --check` clean; `go build ./...` clean.
- `swift test --package-path apps/macos`: `227` XCTest cases (`1` intentional
  visual-export skip) and `15` Swift Testing contracts pass, including new
  strict role/route/node Enrollment decode tests and store field admission.
- RED-first: new authority, runtime binding digest, materialization and
  preflight tests failed on missing symbols/behaviour before implementation.
- Full serial `go test ./...` re-run (results below).

## Privacy and live status

No App bundle was built, signed, launched or installed. No network request,
external MCP process, Provider, real credential, user workspace or runtime
tool side effect was used. Installed Loom remains outside this source-only
gate.

Phase 2D remains the sole `ACTIVE / PARTIAL` Goal. Remaining Phase 2D exit
gates: installed credential import, real conversation, mixed Provider Team
and the single-Agent failure isolation matrix with installed Web/MCP
diagnostics, accounting UI and ATL9 acceptance.
