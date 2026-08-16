# P2D-W2D Remote Tool Enrollment Client Governance V31

Status: `SOURCE VERIFIED / TRUSTED NEW ENROLLMENT, AGENT PREFLIGHT, RUNTIME
MATERIALIZATION AND INSTALLED LIVE OPEN`

Date: 2026-08-15

## Acceptance boundary

V31 closes the native client governance gap for an existing authoritative V30
Remote Tool Backend Enrollment. It does not create a trusted backend candidate,
construct a Search/MCP client, publish a remote capability, bind one to an
Agent, or perform installed-live Web/MCP execution.

The default production composition remains remote-tool unavailable.

## Implemented source

- `LocalProductRemoteToolBackendEnrollmentCommand` and its revoke command are
  strict, account scoped and non-secret. They bind the exact Provider Account
  Policy lineage, Enrollment identity, endpoint fingerprint, MCP allowlist and
  bounded limits.
- `LocalProductRemoteToolBackendEnrollmentResult` rejects unknown fields,
  invalid Search/MCP shapes, malformed digests and non-positive revisions before
  deriving an expected prior revision. An `Int64.min` regression proves the
  decoder fails instead of trapping on subtraction.
- `LocalIPCClient` sends the exact
  `remote_tool_backend_enrollment_configure` and
  `remote_tool_backend_enrollment_revoke` wire shapes. It validates the trusted
  response against the requested Provider, account, Enrollment and revision and
  emits only the existing safe operational correlation record.
- `LocalProductStore` permits configure only for a record already present in the
  current authoritative setup snapshot. Adapter, endpoint fingerprint, backend
  kind and MCP server identity cannot be substituted by client state. Success
  requires an exact post-operation projection refresh.
- Store failures remain local to the Enrollment and expose an actionable detail,
  safe stage, retryability and Incident ID. They do not change setup connection
  state or mark a Team offline.
- `ProviderAccountPolicySheet` projects active/revoked Web Search and MCP rows,
  limits and policy currency. Its native editor supports bounded limit and MCP
  allowlist changes, explicit stale-policy rebind, revoked-record restore and
  confirmed revoke. Retry, redacted diagnostic preview and Incident ID copy are
  available after a failed operation.

## Deliberate exclusions

- There is no Add button, arbitrary Adapter ID, raw endpoint field, remote
  Bundle URL, dynamic plugin or hot-replacement path.
- A new Enrollment cannot be authored until Loom has a trusted built-in backend
  candidate registry whose values can be materialized into typed Work Bundle
  ports.
- Persisted records do not yet grant an Agent a capability or alter a frozen
  Attempt.
- No key, credential reference body, Authorization header, raw endpoint, Prompt,
  Provider response, query, MCP arguments or result content is added to Journal
  or operational diagnostics.

## Verification

Passed:

- `swift test --package-path apps/macos`
  - `226` XCTest cases passed, `1` existing visual-export test skipped.
  - `12` Swift Testing contracts passed.
- Focused strict model, wire, Store and Provider Account governance rendering
  tests, including configure, revoke, policy conflict and invalid revision.
- `go test ./internal/localipc -run '^TestStrictSwiftClientReadsSetupAndStartsCandidateFromRealGoServer$' -count=1`
- `go test ./cmd/loomd -run 'RemoteToolBackendEnrollment|RemoteToolEnrollment' -count=1`
- `git diff --check`

One first full Swift run failed because the new malicious-revision test mutation
was accidentally placed in the preceding valid fixture and therefore performed
no replacement. The test was moved to the intended strict result decoder and
the full suite then passed. This is retained as an honest test-authoring failure,
not rewritten as a product failure or hidden from the evidence.

## Privacy and live status

No App bundle was built, signed, launched or installed. No network request,
external MCP process, Provider, real credential, user workspace or runtime tool
side effect was used. Installed Loom remains outside this source-only gate.

Phase 2D remains the sole `ACTIVE / PARTIAL` Goal. Next source work is the
authoritative per-Agent remote-tool preflight plus trusted built-in backend
candidate and runtime materialization boundary; installed Web/MCP and mixed-Team
ATL9 acceptance remain final gates.
