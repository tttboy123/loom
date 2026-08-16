# P2D-W2C/W2D Production Recovery Lifecycle V19B

Status: `SOURCE VERIFIED / AUTHENTICATED IPC AND SWIFT GOVERNANCE OPEN`

Date: 2026-08-14

## Boundary

V19B moves the V19A recovery closure into the production mission Bundle without
exposing a public recovery command.

When Agent Inbox recovery is configured, the Bundle now owns one recovery
decision authority and one completion coordinator. The composition path builds
the Loom Native restart-capability resolver, encrypted checkpoint continuation,
active-Attempt registry attachment, exact original-grant closure and Run
terminal/accounting authority from the already configured production stores and
adapters. It does not add a second Provider client or bypass frozen bindings.

Before a recovered continuation is dispatched, the Team observer factory
rebuilds the authoritative projection and locates exactly one Team node Attempt
by WorkItem, Run, Claim generation, Runtime and Agent. The restored observer
uses the existing Team frame collector, so authorized output again reaches the
same evidence and NodeOutput projection path as ordinary execution. Missing or
ambiguous ownership fails closed.

Startup also reconciles the two-authority crash window left by a process exit
after Run terminal commit but before original-grant revocation. It scans
authoritative Runs and grants, revokes only one exact unrevoked grant whose full
execution tuple matches a terminal Run, ignores already revoked grants, and is
idempotent on replay. A mismatched or duplicate pending grant stops startup.
The reconciler has no Runtime, adapter, credential or Provider-dispatch port.

Each successful startup repair writes a content-free
`authorization_reconcile` operational record at
`agent_attempt_reconcile`. It includes only the startup Incident ID, frozen
Provider/Account/Model identifiers, Work/Run/generation, Runtime, Agent and
Execution Binding digest. It contains no Capsule, Prompt, transcript, Provider
body, credential, grant token or ciphertext. Success records remain operational
audit data and are not projected as Team board failures.

## Security properties

- Recovery services are owned by the mission Bundle and reuse the existing
  protected Journal, Run, authorization, Vault lease, Capsule and evidence
  authorities.
- Runtime continuation remains limited to the exact consumed one-use recovery
  grant and frozen Loom Native checkpoint binding.
- The original grant bearer token is never reconstructed.
- Team evidence/output observation is restored before frame authority is
  created; observer rejection prevents terminal authority.
- Terminal/grant reconciliation cannot redispatch a Provider request.
- A stale grant is revoked only after exact WorkItem, Run, Claim,
  generation, Runtime and Agent equality; ambiguity and tuple drift fail closed.
- Operational repair evidence is persistent, bounded and content-free, and it
  does not manufacture a user-visible failure.

## Source

- `cmd/loomd/product_agent_attempt_recovery_lifecycle.go`
- `cmd/loomd/product_agent_attempt_recovery_observer.go`
- `cmd/loomd/product_agent_attempt_recovery_reconcile.go`
- `cmd/loomd/product_agent_attempt_recovery_completion.go`
- `cmd/loomd/product_daemon.go`
- `cmd/loomd/operational_diagnostics.go`
- `internal/app/team_execution.go`
- `internal/supervisor/recovery_frame_authority.go`
- `cmd/loomd/product_agent_attempt_recovery_completion_test.go`
- `cmd/loomd/operational_diagnostics_test.go`

## Verification

The diagnostic RED failed only on the missing reconciliation outcome and
operational-record symbols. GREEN proves exact repair identity, idempotent
replay, revoked-grant exclusion, content-free persistent audit and separation
from failure projection. The existing V19A suite continues to prove frame,
observer, accounting, terminal and grant closure.

Passed:

```text
go test -race ./cmd/loomd \
  -run 'TestProductAttemptRecoveryCompletion|TestProductAuthorizationRecoveryGrantClosure|TestProductAttemptRecoveryTerminalReconciliation|TestProductLoomNativeAttemptRecoveryResumesExactEncryptedState' \
  -count=10

go test ./internal/app ./internal/supervisor -count=1
go test ./cmd/loomd -count=1
go test -p 1 ./... -count=1
go vet ./...
git diff --check
```

The exact touched Go file set produced no output from `gofmt -l`.

## Open gates

V19B deliberately exposes no IPC route and no Swift action. V19C must add an
authenticated read-only preview, explicit confirmation bound to the exact
candidate/capability digests, one-use resume consumption, visible non-secret
operation state and stale-command handling. Until then, the production Bundle
owns the recovery lifecycle but a user cannot invoke it.

Codex, Claude Code and Pi restart reattachment, installed CV6, mixed-Team ATL9,
broader ATL3-ATL8 work and COMP2-E remain open under the sole
`ACTIVE / PARTIAL` Phase 2D Goal. No App, network, real Provider, real
credential, user workspace or external Runtime was accessed.
