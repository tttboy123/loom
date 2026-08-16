# P2D-W2D Persisted Remote Tool Backend Enrollment V30

Status: `SOURCE VERIFIED / CLIENT EDITING, RUNTIME MATERIALIZATION AND INSTALLED LIVE OPEN`
Date: 2026-08-15
Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

## Acceptance boundary

This slice makes Search and MCP backend Enrollment a versioned, account-policy-
bound governance fact that survives restart and is readable by the strict App
setup contract. It does not create a Search/MCP client, grant an Agent a remote
tool capability, perform network access or provide the user-facing editor.

## Authority and isolation

- `RemoteToolBackendEnrollment` supports only `web_search` and `mcp_server`
  records, with strict backend-specific shapes and bounded limits.
- Every revision binds one Provider ID, Provider Account ID, exact Provider
  Account Policy version/revision/digest, built-in adapter ID and SHA-256
  endpoint fingerprint. Raw endpoints and credentials are not accepted.
- MCP Enrollment additionally freezes one server ID and a sorted, unique,
  bounded tool allowlist. Search Enrollment rejects MCP fields.
- Configure atomically compares the Enrollment stream head and exact Provider
  Account Policy stream head. Policy drift, replay corruption, cross-account
  substitution and concurrent updates fail closed.
- Revoke is independently versioned and does not require the policy to remain
  current, allowing capability withdrawal after a policy update.
- Multiple Provider Accounts remain separate. A configured or verified
  account credential projection is required before product configuration.

## Product projection and diagnostics

- The Governance Bundle owns Enrollment authority and exposes it to Setup via a
  typed revocable port; protected Journal and policy authority stay in Core.
- Setup projects deterministic active/revoked records under the exact Provider
  Account and computes `policy_current` against the latest authority state.
- Authenticated private-UDS configure/revoke methods inject the request ID as
  correlation and preserve `invalid_request`, `conflict`, `stale_view`,
  `not_found` and `state_unavailable` distinctions.
- Operational diagnostics include only Incident ID, operation, Provider/
  Account, stage, duration, result and retryability. Operation input,
  Enrollment ID, endpoint fingerprint, MCP allowlist, secret, Prompt and
  Provider content are excluded.
- The Swift setup decoder remains backward compatible when the collection is
  absent, but strictly validates policy currency, identifiers, digests,
  timestamps, limits and Search/MCP shape when records are present.

## Verification

Passed:

```text
go test ./internal/work -run '^TestRemoteToolBackendEnrollment' -count=1
go test ./internal/projection -run '^TestRemoteToolBackendEnrollmentProjection' -count=1
focused internal/app Enrollment and account-isolation tests
focused cmd/loomd Enrollment wire and safe operational-diagnostic tests
go test -race ./internal/work ./internal/projection ./internal/app -run 'RemoteToolBackendEnrollment' -count=10
go test -race ./cmd/loomd -run 'Test(ProductOperationalDiagnosticsRecordsSafeRemoteToolEnrollmentEvent|RemoteToolBackendEnrollmentWire)' -count=10
go test ./internal/work ./internal/projection ./internal/app ./internal/api ./internal/toolbroker ./internal/execution -count=1
go test ./cmd/loomd -count=1
go test ./internal/localipc -run '^TestStrictSwiftClientReadsSetupAndStartsCandidateFromRealGoServer$' -count=1
go test -p 1 ./... -count=1
swift test --package-path apps/macos
go vet ./internal/work ./internal/projection ./internal/app ./internal/api ./cmd/loomd
gofmt, privacy search and git diff checks on the affected source set
```

The complete macOS package result is `223` XCTest cases with `1` skipped and
zero failures plus `11` Swift Testing cases with zero failures. The fresh
serial Go run passed every package; `cmd/loomd` completed in 248.445 seconds
and `internal/localipc` completed in 355.063 seconds.

## Remaining

- Swift configure/revoke client methods, confirmation and safe editing UI;
- persisted Enrollment to built-in Search/MCP client materialization;
- exact per-Agent capability/preflight projection and Attempt freeze;
- installed Search/MCP diagnostics, account-local failure isolation and live
  continuation acceptance;
- installed CV6, mixed-Team ATL9 and COMP2-E.

Default production still supplies no remote-tool configuration and publishes
no Web/MCP capability. No App, network, external MCP, Provider, real
credential, user workspace or external Runtime was accessed in this slice.
