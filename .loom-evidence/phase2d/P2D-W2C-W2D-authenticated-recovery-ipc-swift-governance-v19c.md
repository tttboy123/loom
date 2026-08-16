# P2D-W2C/W2D Authenticated Recovery IPC and Swift Governance V19C

Status: `SOURCE VERIFIED / INSTALLED LIVE OPEN`

Date: 2026-08-14

## Boundary

V19C exposes the V19B production recovery lifecycle through the authenticated
private UDS and an explicit Swift governance flow. It does not create a public
or automatic retry path.

The typed `agent_attempt_recovery` route has three distinct operations:

- `preview` rebuilds current recovery candidates and returns no execution
  authority;
- `confirm` binds one authenticated principal to the exact current candidate
  and restart-capability digests;
- `resume` consumes that decision once and synchronously enters the V19B
  completion path.

The Swift Store mirrors those authority boundaries. It requires a fresh
authoritative preview, refuses confirmation for a stale candidate, refuses
Resume without the exact confirmed decision, and suppresses duplicate Resume
dispatch while the first request is in flight. A failed or uncertain Resume
invalidates local candidate and decision state so the user must Preview again.

Mission Inspector shows recovery only on the exact affected Agent row. It
displays the non-secret Harness, Provider Account, Model and credential
revision, uses a shield action for confirmation, and presents a separate Resume
action. The confirmation dialog states that confirmation alone does not start
Provider work. Failures preserve actionable status and Incident correlation;
the user can refresh the preview or copy the Incident ID when allowed.

## Diagnostics and privacy

The App and daemon record the same request Incident across the recovery route.
Records contain operation, stage, elapsed time, result, stable error code and
retryability. Request diagnostics use `agent_attempt_reconcile` and contain no
candidate digest, capability digest, principal ID, Prompt, transcript, Provider
body, credential, Authorization header or API key.

Strict Swift decoding rejects unknown fields, malformed schema/action values,
invalid UUIDs, empty required identities and malformed digests. The daemon
continues to enforce authenticated local IPC and exact server-side candidate,
capability and decision reconstruction; Swift state is never execution
authority.

## Source

- `internal/work/agent_attempt_recovery_authority.go`
- `cmd/loomd/product_agent_attempt_recovery_route.go`
- `cmd/loomd/operational_diagnostics.go`
- `apps/macos/Sources/LoomLocalAppCore/LocalProductAgentRecoveryModels.swift`
- `apps/macos/Sources/LoomLocalAppCore/LocalIPCClient.swift`
- `apps/macos/Sources/LoomLocalAppCore/LocalOperationalDiagnostics.swift`
- `apps/macos/Sources/LoomLocalAppCore/LocalProductStore.swift`
- `apps/macos/Sources/LoomLocalAppUI/MissionWorkbench.swift`
- `internal/localipc/swift_contract_test.go`

## Verification

RED/GREEN covered missing request diagnostics, strict Swift decoding, explicit
confirmation before Resume, stale preview invalidation, duplicate Resume
suppression, visible recovery actions and App-side operational correlation.

Passed:

```text
go test -race ./internal/work ./cmd/loomd \
  -run 'TestAgentAttemptRecoveryAuthority|TestProductAgentAttemptRecoveryRoute|TestProductDaemonRoutesStrictAgentAttemptRecovery|TestProductAgentAttemptRecoveryTraversesAuthenticatedLocalIPC|TestProductOperationalDiagnosticsRecordsContentFreeAttemptRecoveryRequest|TestProductAttemptRecoveryCompletion|TestProductAuthorizationRecoveryGrantClosure' \
  -count=10

go test ./internal/localipc \
  -run TestStrictSwiftClientReadsSetupAndStartsCandidateFromRealGoServer \
  -count=1
swift test --package-path apps/macos
go test -p 1 ./... -count=1
go vet ./...
git diff --check
```

The complete Swift suite passed 214 tests with one visual-export-only test
skipped by design and no failures. The exact touched Go file set produced no
output from `gofmt -l`. A privacy scan of App and daemon operational diagnostics
found none of the forbidden content fields.

## Open gates

This is source verification. The installed App recovery route, real daemon
restart, real Loom Native Provider continuation and user-visible rendered flow
were not exercised. Codex, Claude Code and Pi restart reattachment, installed
CV6, mixed-Team ATL9, broader ATL3-ATL8 work and COMP2-E remain open under the
sole `ACTIVE / PARTIAL` Phase 2D Goal.

No App, network, real Provider, real credential, user workspace or external
Runtime was accessed.
