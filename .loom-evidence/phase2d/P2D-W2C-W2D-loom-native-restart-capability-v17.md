# P2D-W2C/W2D Loom Native Restart Capability V17

Status: `SOURCE VERIFIED / PRODUCTION CONTINUATION AND UI OPEN`

Date: 2026-08-14

## Boundary

This slice gives only the built-in Loom Native Runtime an explicit
`loom-owned-checkpoint/v1` restart conformance contract. It does not claim that
a Provider-native session survived daemon restart. Codex, Claude Code and Pi do
not implement this contract.

The product resolver accepts only a valid V10 `pre_model_resume` outcome and
revalidates:

- the complete frozen Execution Binding and exact Runtime instance;
- the built-in Loom Native adapter's Provider, Provider Account, Model,
  endpoint fingerprint and credential reference/revision binding;
- the encrypted Context Capsule's content-free authority for the exact
  Conversation, Team, Agent, Provider Account and Model;
- the Route Segment, previous output checkpoint and recovered input IDs through
  a domain-separated session-binding digest.

The resolver does not read Capsule content. Capability resolution and V16
identity reattachment do not call `RuntimeAdapter.Execute`, acquire a credential
lease, issue HTTP, or dispatch a Provider request.

## Fail-closed behavior

- Unsupported or non-conformant Runtimes cannot mint a capability.
- Duplicate Runtime identities and duplicate exact Capsule authorities fail.
- Binding, Capsule, Segment or checkpoint substitution changes or invalidates
  the capability.
- Capsule authority drift after V15 consumption prevents V16 attachment and
  active-Attempt registration.
- An unrelated corrupt Capsule authority in the same Conversation does not
  block the exact Agent, preserving failure isolation.

## Source

- `internal/runtime/agent_input.go`
- `internal/runtime/nativeadapter/deepseek_adapter.go`
- `internal/work/agent_attempt_recovery_authority.go`
- `cmd/loomd/product_loom_native_attempt_recovery.go`
- `cmd/loomd/product_agent_attempt_recovery_runtime.go`

## Verification

Passed:

```text
go test ./internal/runtime/nativeadapter ./internal/work ./cmd/loomd \
  -run 'TestLoomNativeAdapterAdvertisesExactRestartCheckpointConformance|TestProductLoomNativeAttemptRecovery|TestAgentAttemptRecovery|TestAgentAttemptRestart' \
  -count=1

go test -race ./internal/runtime/nativeadapter ./internal/work ./cmd/loomd \
  -run 'TestLoomNativeAdapterAdvertisesExactRestartCheckpointConformance|TestProductLoomNativeAttemptRecovery|TestAgentAttemptRecovery|TestAgentAttemptRestart' \
  -count=10

go test -p 1 ./... -count=1
go vet ./...
```

## Open gates

The daemon does not yet construct this resolver/reattacher as a production
recovery service. No recovered continuation request is assembled or dispatched,
and no authenticated IPC or Swift Resume action is exposed. The next slice must
reconstruct the exact Loom-owned Capsule dispatch and AgentInputSource, admit
one new ModelRequest only at the existing Attempt Loop authority boundary, and
retain all V10 uncertain-Provider fail-closed rules.

Installed CV6, mixed-Team ATL9, accounting completion and COMP2-E remain open.
No App, network, Provider, real credential, user workspace or external Runtime
was accessed.
