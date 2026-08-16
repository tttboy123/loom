# P2D-W2C Scoped Context Retrieval Broker v1

**Date**: 2026-08-12  
**Status**: `CURRENT / SOURCE VERIFIED / POST-BUILD-64`  
**Goal**: unified Phase 2D only  
**WorkItem**: existing P2D-W2C with P2D-W2D diagnostics

## Result

Loom now preserves token-budget omissions as encrypted, separately serialized
retrieval material inside the existing Conversation-DEK Context Capsule
envelope. The canonical Capsule body, Capsule digest, disclosure receipt, and
dispatch bytes remain unchanged. Only items omitted for `budget_exceeded` are
eligible. `policy_filtered`, `access_denied`, credential references, hidden
reasoning, Provider bodies, and secrets are never retrievable.

Every read is bound to the daemon-frozen Capsule Authority and Attempt identity:
WorkItem, Run, claim generation, Runtime instance, execution-binding digest,
Agent, Role, item ID, content digest, and optional artifact reference. Scope
checks are exact for conversation/team shared, Agent private, role restricted,
and artifact scoped items. A model proposal can name an item and digest but
cannot assert or replace these identities.

The broker records a content-free operational diagnostic before releasing the
mutable plaintext result. Audit failure fails closed and zeroizes the result.
Allow and deny records carry Incident ID, stage, outcome, binding and Capsule
digests, item identity, Agent/Role, and optional artifact identity; they do not
carry the retrieved content, Prompt, conversation text, Provider response, or
credential material.

## Encrypted restart and compatibility

Context Capsule envelope v2 stores the retrievable supplement in the same
encrypted Capsule ciphertext. A real LocalKeyFile Vault close/reopen can issue
the same exact scoped read, while raw database scans do not find the omitted
plaintext. Structured v1 records can be upgraded to v2 only with exact
Authority and dispatch equivalence. v2 downgrade is rejected. Legacy
dispatch-only records retain their controlled upgrade path.

The Team coordinator carries the immutable Capsule Authority into managed
execution. The product daemon creates a `ScopedRetriever` only when the
production Capsule runtime supplies both the Vault retrieval store and the
operational auditor. Supervisor validation always binds Capsule Authority to the
frozen execution binding and dispatch. Explicit legacy/test runtimes without a
Vault broker may consume already-disclosed Capsule content but receive no
retrieval capability; a retriever without Authority is rejected.

## Verification

- Capsule scope, digest, omission-reason, zeroization, and audit-failure tests
  pass.
- Vault v2 close/reopen, v1-to-v2 upgrade, v2 downgrade rejection, and plaintext
  database scan tests pass.
- Supervisor exact-capability, authority-only, and foreign-Capsule rejection
  tests pass for ten focused runs.
- `internal/contextcapsule`, `internal/credentials/vault`, `internal/supervisor`,
  `internal/app`, and `internal/api` package tests pass.
- The two timing-sensitive daemon vertical tests pass for three focused runs
  after one initial full-package run observed cold-start timeouts.
- Loom Native DeepSeek, Kimi, and MiniMax Adapter tests prove one exact
  `loom_read_context` proposal, broker read, native tool-result message, and
  second Provider response inside the same Attempt. Retrieved content appears
  only in the second Provider request and never in Bridge frames or diagnostics.
- Loom Native runtime discovery publishes the explicit `context_retrieval`
  capability, and generated DeepSeek, Kimi, and MiniMax Execution Profiles
  require it. The daemon freezes and checks that capability before creating a
  broker; the Adapter independently rejects a retriever without the frozen
  capability and a capable Attempt without its broker before credential access.
- Exact historical Loom Native runtime records with an empty capability set are
  upgraded by one append-only rediscovery event. Unknown capabilities or any
  other runtime identity drift remain rejected, migration is idempotent, and
  existing Team bindings are not rewritten or silently upgraded.
- Duplicate JSON keys, unknown tool fields, identity/digest/classification/body
  drift, secret markers, denied reads, and a second tool request fail closed.
- Affected Native Adapter and prompting race tests pass for ten runs; affected
  vet and diff checks pass.
- The complete affected Runtime, prompting, Capsule, Vault, Supervisor,
  Native Adapter, app, API, and daemon race matrix passes for three runs.
- `go test ./... -count=1`, `go vet ./...`, targeted format checks,
  `git diff --check`, and `go mod verify` pass after capability freeze and
  historical-runtime migration were added.

The repository-wide non-race Go, vet, diff, format, and module gates are green.
The race gate is intentionally scoped to all affected packages rather than an
uncached whole-repository race run.

## Open boundary

This increment completes the daemon-side encrypted retrieval source,
Attempt-bound broker, and a bounded Loom Native OpenAI-compatible result wire.
DeepSeek, Kimi, and MiniMax may request at most one scoped omission and consume
it in one second Provider round. The result is not projected as a Bridge event,
Journal fact, Evidence payload, or diagnostic body.

The tool exists only when the exact frozen binding contains
`context_retrieval`. Historical bindings without that capability continue with
no tool. Runtime rediscovery makes the capability available to newly generated
Profiles; it does not mutate previously frozen execution authority.

This is not the complete generic Attempt Tool Loop. The post-build-64 source now
also contains Pi's fixed private extension/result transport, verified separately
in `P2D-W2C-pi-context-retrieval-transport-v1.md`. Codex and Claude Code still
require Attempt-scoped MCP transport. Persisted result delivery, crash-resume,
multiple tool rounds, and installed real-Provider evidence also remain open.

Build 64 predates this source. No bundle was built, launched, or installed.
Installed Loom remains v0.5.2 build 39 and was not modified. Phase 2D remains
the sole `ACTIVE / PARTIAL` Goal.
