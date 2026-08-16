# P2D-W2C/W2D ToolCall Recovery Decision Governance V20

Status: `SOURCE VERIFIED / TRUSTED RESOLVERS AND INSTALLED LIVE OPEN`

Date: 2026-08-14

## Boundary

V20 closes the source-level governance gap for a ToolCall whose side effect is
unknown after daemon interruption. It is separate from V19C Agent Attempt
restart recovery. The original ToolCall is terminalized and is never
automatically rerun.

One content-free authority exposes three closed actions:

- `abort_attempt` closes the affected Attempt at the uncertain boundary;
- `accept_observed_effect` requires a trusted observation resolver and freezes
  exact evidence, observation, output and changed-files digests;
- `retry_in_new_attempt` requires a trusted replacement resolver and freezes a
  distinct Attempt, Run, Execution Binding and Context Capsule.

Production currently composes no observation or replacement resolver, so its
Preview advertises only `abort_attempt`. This is intentional fail-closed
behavior. The other actions cannot appear in Swift unless the daemon advertises
them from installed trusted capabilities.

## Authority and replay

The domain-separated candidate digest binds execution, Job, call, tool,
generation, operation, original Incident, allow/recovery timestamps, recovery
code/action and the recovery-required event. Resolve replays that exact
candidate, then appends one schema-v2 `ToolExecutionRecoveryResolved` fact with
a stream-head CAS.

Concurrent decisions have one winner. Exact replay of the winning decision is
idempotent; a distinct action, decision or candidate conflicts. Resolved
statuses are `recovery_aborted`, `recovery_effect_accepted` and
`recovery_retry_authorized`. All are terminal to the execution Adapter, so none
can invoke the original executor. Typed-nil resolver interfaces are normalized
to absent capabilities and cannot advertise authority or panic during Resolve.

The authority has no Tool Gateway, Runtime, Provider, credential or content
payload dependency. Journal facts contain the authenticated principal and
non-secret identity/digests required for authority replay, but no command,
path, Prompt, Tool result body, Provider body or secret.

## Authenticated route and Swift governance

The Work Bundle owns a typed `tool_recovery` route with distinct read-only
`preview` and authority-writing `resolve` operations. It is admitted through
the existing authenticated private UDS and the typed route registry. Daemon and
App operational diagnostics share the request Incident ID and record only the
operation, safe stage, elapsed time, result, stable error and retryability.

The Swift Store requires a fresh exact candidate and an action present in its
`available_actions`. It suppresses duplicate in-flight decisions. A failed or
uncertain Resolve discards the local candidate and decision so the user must
Preview again rather than assume the command was uncommitted.

Mission Inspector presents ToolCall recovery separately from Agent restart
recovery. It displays only non-secret tool, Job and Incident identity. The
action menu is daemon-driven, every action requires confirmation, and the
confirmation states that Loom will not rerun the original ToolCall. Failures
offer Refresh, View diagnostics and Copy incident ID.

## Source

- `internal/execution/tool_recovery_authority.go`
- `internal/execution/replay.go`
- `internal/execution/adapter.go`
- `cmd/loomd/product_tool_recovery_route.go`
- `cmd/loomd/product_composition_work.go`
- `cmd/loomd/product_route_registry.go`
- `cmd/loomd/product_daemon.go`
- `cmd/loomd/operational_diagnostics.go`
- `internal/localipc/protocol.go`
- `internal/localipc/server.go`
- `apps/macos/Sources/LoomLocalAppCore/LocalProductToolRecoveryModels.swift`
- `apps/macos/Sources/LoomLocalAppCore/LocalIPCClient.swift`
- `apps/macos/Sources/LoomLocalAppCore/LocalOperationalDiagnostics.swift`
- `apps/macos/Sources/LoomLocalAppCore/LocalProductStore.swift`
- `apps/macos/Sources/LoomLocalAppCore/LocalExecutionModels.swift`
- `apps/macos/Sources/LoomLocalAppUI/MissionWorkbench.swift`

## Verification

Passed:

```text
go test ./internal/execution ./internal/projection \
  -run 'TestToolRecoveryAuthority|TestToolRecoveryResolvedReplay|TestRebuildAcceptsOnlyKnownDedicatedAuthoritySchemaV2Events' \
  -count=1
go test -race ./internal/execution ./internal/projection ./cmd/loomd \
  -run 'TestToolRecoveryAuthority|TestToolRecoveryResolvedReplay|TestProductToolRecovery|TestToolRecoveryMethod|TestProductOperationalDiagnosticsRecordsContentFreeToolRecoveryRequest|TestProductWorkBundle' \
  -count=10
go test ./internal/execution ./internal/projection ./internal/localipc -count=1
go test ./cmd/loomd -count=1
swift test --package-path apps/macos
go vet ./cmd/loomd ./internal/execution ./internal/projection ./internal/localipc
git diff --check
```

The full Swift suite passed 223 XCTest cases plus 10 Swift Testing cases, with
one visual-export-only XCTest skipped by design and no failures. The first full
Go run found a stale COMP2C Work Bundle fixture that lacked the new typed route;
the parity fixture was repaired and its route revocation is now asserted.

## Open gates

This is source verification. Trusted observation and replacement-Attempt
resolvers are not composed in production. Accepting an observed result does not
yet deliver result content to a model, and authorizing a replacement does not
start it. Installed App recovery, real daemon restart, real Tool side-effect
observation, CV6, mixed-Team ATL9, remaining ATL3-ATL8 work and COMP2-E remain
open under the sole `ACTIVE / PARTIAL` Phase 2D Goal.

No App, network, real Provider, real credential, user workspace or external
Runtime was accessed.
