# P2D-W2C/W2D Durable Agent Inbox V1

Status: `SOURCE VERIFIED / DAEMON INGRESS AND INSTALLED LIVE OPEN`

Date: 2026-08-14

## User-visible conclusion

Loom now has a source-verified durable authority and encrypted storage core for
Agent Queue, Steer and Inject input. This does not yet expose those controls in
the installed App. Product daemon ingress, Runtime context assembly and Swift
governance UI remain required before a user can rely on the feature.

## Implemented boundary

- `internal/agentinbox` defines immutable mode, context scope, target and
  encrypted-payload bindings. Each input freezes Conversation, Segment, Agent,
  WorkItem, Run, claim generation, Runtime, Execution Binding digest, Capsule
  digest, ordering key, target Turn/Step and content digest.
- `AgentInboxAuthority` persists only non-content `AgentInputAdmitted` and
  `AgentInputConsumed` facts. It validates the current Run generation and reads
  Attempt Loop plus Inbox streams from one SQLite read transaction.
- Queue targets the next Turn and cannot skip an earlier queued input. Its
  consumption fact and `TurnStarted` are one CAS batch over Run, Attempt Loop
  and Inbox stream heads.
- Steer and Inject target the nearest next admissible Step. Neither admission
  wakes a Step. Exact ordered consumption and `StepStarted` are one CAS batch;
  omitted, substituted, mixed-state, closed or stale targets fail closed.
- Context scopes cover `conversation_shared`, `team_shared`, `agent_private`,
  `role_restricted`, `artifact_scoped` and `secret_reference_only`. Restricted
  scopes require an exact non-secret target ID.
- `VaultStore` stores input bytes in `encrypted_agent_inputs` under the existing
  per-Conversation DEK using AES-256-GCM. Canonical AAD covers every frozen
  binding field and status. Plaintext never enters the Journal schema.
- `AgentInboxCoordinator` is the public mutation surface: encrypted payload is
  written before authority admission, known-uncommitted ciphertext is rolled
  back, unknown commit outcomes remain pending, and restart reconciliation
  aligns pending/consumed storage with authoritative facts without inventing
  authority.

## Acceptance evidence

- Mandatory RED failed only on missing Inbox authority symbols.
- Active-Turn Queue admission stays non-waking and consumes into the exact next
  Turn after the prior Turn reaches a stable successful end.
- Steer and Inject stay non-waking, preserve order and consume only into their
  exact next Step.
- Concurrent identical consumption is idempotent. Cross-stream reads use
  `ReadStreamSet`, closing the inconsistent Loop/Inbox snapshot race found by
  the first concurrency run.
- Restart replay reconstructs input status and target lineage. Coordinator
  reconciliation repairs Journal-committed/storage-pending consumption.
- Vault tests prove encrypted-at-rest persistence across restart, exact status
  transition, order-key conflict, target/Segment substitution rejection and
  AAD tamper failure.
- Journal negative scans include synthetic input, Prompt and Authorization
  sentinels; only content type and SHA-256 digest are serializable authority
  fields.

## Verification

Passed:

```text
go test -race ./internal/work ./internal/credentials/vault -run 'TestAgentInbox' -count=10
go test ./internal/agentinbox ./internal/work ./internal/credentials/vault ./cmd/loomd -count=1
go test ./... -count=1
go test -race ./internal/work ./internal/credentials/vault -count=1
go vet ./...
gofmt -d <changed Go files>
git diff --check -- internal/credentials/vault/vault_store.go
```

## Open gates

- Compose the Inbox coordinator into the product daemon and add authenticated
  IPC admission. Do not reuse the legacy developer work queue.
- Extend the Runtime Contract so Loop-owned model-input assembly decrypts only
  exact pending inputs, closes/zeroizes them after use and records consumption
  acknowledgement.
- Preserve user drafts and return an explicit decision when stale Steer could
  fall back to Queue; never perform that fallback silently.
- Add operational Incident stages and Swift Queue/Steer/Inject controls with
  per-Agent target/status projection.
- Run the same conformance through Loom Native and at least one managed external
  Runtime, then complete installed CV6/ATL9 mixed-Team acceptance.

No App was built, signed, launched or installed. No network, Provider, real
credential, user workspace or external Runtime was accessed. Phase 2D remains
the sole `ACTIVE / PARTIAL` Goal.
