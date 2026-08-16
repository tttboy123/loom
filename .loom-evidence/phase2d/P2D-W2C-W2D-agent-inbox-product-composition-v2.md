# P2D-W2C/W2D Agent Inbox Product Composition V2

Status: `SOURCE VERIFIED / RUNTIME CONSUMPTION AND PRODUCT INGRESS OPEN`

Date: 2026-08-14

## User-visible conclusion

The durable Agent Inbox is now part of the real product composition graph. The
Loom Vault Bundle exposes the encrypted Inbox store and the mission executor
constructs one coordinator over the same Attempt Loop authority used by the
Runtime adapter. This is still not an installed Queue, Steer or Inject feature:
there is no client ingress or model-input consumption path yet.

## Implemented boundary

- `productCredentialVaultRuntime` and its recovery runtime implement the
  `agentinbox.Store` contract without exposing plaintext outside the daemon.
- The versioned Vault Bundle owns the Inbox store port. Its bounded route slot
  forwards put/read/list/consume/delete operations only while the Bundle is
  bound and open; recovery, unbound and closed states fail closed.
- A rejected store write clears the caller-owned sensitive payload before
  returning.
- Product daemon construction reuses the Vault Bundle slot already supplied as
  the Context Capsule and Attempt Payload store. When that slot also exposes
  Agent Inbox storage, `newProductMissionExecutor` composes one
  `AgentInboxAuthority` and `AgentInboxCoordinator` over the same
  `AttemptLoopAuthority`.
- Inbox composition without the Attempt Payload/Loop authority is rejected as
  invalid mission execution. It cannot create a second or detached terminal
  authority.
- Closing the product facade revokes the bounded Inbox port. Reads after close
  fail rather than retaining a stale Vault capability.

## Acceptance evidence

- RED first failed because the Vault Bundle slot did not implement the Agent
  Inbox store surface.
- The real Vault-backed Bundle test writes, reads and consumes an encrypted
  Agent input through the product route slot, then proves facade close revokes
  access.
- Recovery/unavailable tests prove sensitive input bytes are zeroized on a
  rejected write.
- Mission executor construction proves the coordinator and deferred Runtime
  configuration share one instance and rejects a store without the Attempt
  authority dependency.
- Focused repetition, focused race, full daemon, repository Go, vet, format and
  diff checks pass.

## Verification

Passed:

```text
go test ./cmd/loomd -run 'TestProductMissionExecutorComposesAgentInbox|TestCOMP2CVault' -count=10
go test -race ./cmd/loomd -run 'TestProductMissionExecutorComposesAgentInbox|TestCOMP2CVault' -count=10
go test ./cmd/loomd -count=1
go test ./... -count=1
go vet ./...
gofmt -d cmd/loomd/product_credential_vault.go cmd/loomd/product_composition_vault.go cmd/loomd/product_composition_vault_test.go cmd/loomd/product_daemon.go cmd/loomd/product_attempt_loop_runtime_test.go
git diff --check -- <composition and evidence files>
```

## Open gates

- Add an authoritative active-Attempt resolver. Client input must identify an
  exact current Agent/Run generation and may not supply or rewrite a frozen
  internal binding.
- Extend the product Attempt Runtime beyond its current one-Turn/one-Step
  envelope. Queue must be consumed into the next Turn; Steer and Inject must be
  consumed into the exact next Step.
- Decrypt only the consumed input set during model-input assembly, preserve
  context scope, and zeroize every plaintext buffer after the Runtime accepts
  or rejects it.
- Only after that consumption path exists may authenticated daemon IPC and
  Swift governance controls expose Queue, Steer and Inject.
- Cross-Runtime conformance, diagnostics, restart behavior, installed CV6 and
  mixed-Team ATL9 remain open.

No App was built, signed, launched or installed. No network, Provider, real
credential, user workspace or external Runtime was accessed. Phase 2D remains
the sole `ACTIVE / PARTIAL` Goal.
