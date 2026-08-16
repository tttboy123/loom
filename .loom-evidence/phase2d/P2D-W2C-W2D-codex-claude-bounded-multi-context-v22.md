# P2D-W2C/W2D Codex and Claude Bounded Multi-Context V22

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / GENERAL TOOLS AND INSTALLED LIVE OPEN  
**Related**: `contracts/P2D-W2C-agent-attempt-binding-dispatch.md`, `contracts/P2D-W2D-observability-governance.md`

## Accepted boundary

Codex and Claude Code may now issue up to four distinct governed
`loom_read_context` calls through their Attempt-scoped private MCP service:

```text
frozen Harness Attempt
  -> Context call 1 -> encrypted payload 1 accepted/pending
  -> Context call 2 -> encrypted payload 2 accepted/pending
  -> Context call 3 -> encrypted payload 3 accepted/pending
  -> Context call 4 -> encrypted payload 4 accepted/pending
  -> validated Harness final output
  -> payloads 1..4 delivered in sequence with harness_final_output proof
```

A later MCP call is not treated as proof that the Harness consumed an earlier
result. All prepared results remain pending until the adapter has validated the
Harness process result. The single strong `harness_final_output` proof then
acknowledges every exact binding in monotonic order. An acknowledgement failure
can be retried from the first unacknowledged binding without acknowledging an
already delivered binding again.

The service rejects a duplicate proposal, a fifth call, a concurrent call, a
call after sealing, and final-output acknowledgement while a Prepare is still
in flight. A failed Prepare seals the service fail closed. Metadata exposes
only counts and protocol/method state; it does not contain Context content.

Product Attempt Loop policy now distinguishes proof models:

- Codex and Claude Code receive four `parallel` pending slots. Conflict scope
  still rejects two pending reads of the same scoped Context item.
- Loom Native, Pi and unknown/default Runtime types retain one `exclusive`
  slot. Loom Native V21 still requires `provider_continuation` delivery before
  the next Context call.

The production authority test observes two Codex Context calls simultaneously
at `accepted/pending`, then verifies both become `delivered` only with
`harness_final_output`. Adapter tests exercise the same two-call lifecycle
through the real private MCP HTTP boundary for both Codex and Claude Code.

## Verification

```text
go test ./internal/runtime/harnessadapter -count=1
go test ./cmd/loomd -count=1
go test ./cmd/loomd -run 'TestProductAttemptLoop(ContextToolPolicySeparatesHarnessAndNativeProofs|KeepsHarnessContextPendingUntilFinalOutput|RuntimeGovernsSequentialContextDelivery|RuntimeGovernsContextDelivery)' -count=10
go test -race ./internal/runtime/harnessadapter ./cmd/loomd -run 'TestHarnessContextMCP(BatchesFourDistinctReadsUntilHarnessFinalOutput|AcknowledgementRetryResumesAtFailedBinding|RejectsFinalOutputAckDuringPreparation)|TestHarnessAdaptersAcknowledgeMultipleContextReadsOnlyAfterFinalOutput|TestProductAttemptLoop(ContextToolPolicySeparatesHarnessAndNativeProofs|KeepsHarnessContextPendingUntilFinalOutput|RuntimeGovernsSequentialContextDelivery)' -count=10
go vet ./internal/runtime/harnessadapter ./cmd/loomd
git diff --check -- internal/runtime/harnessadapter/context_mcp.go internal/runtime/harnessadapter/context_mcp_test.go internal/runtime/harnessadapter/codex_adapter_test.go cmd/loomd/product_attempt_loop_runtime.go cmd/loomd/product_attempt_loop_runtime_test.go
```

The first RED failed because call two was rejected by the former one-call MCP
state. The product RED failed because no Runtime-specific Context policy
existed. The final focused, complete-package, ten-run race, vet and formatting
gates pass. Verification used only local fixtures and loopback private HTTP.

## Remaining boundary

This slice enables repeated scoped Context retrieval only. It does not enable
general Read/Grep/Web/MCP tools, arbitrary Provider-native tools, sibling
parallel effects, installed App execution or real Provider calls. Codex and
Claude restart reattachment, installed CV6, mixed-Team ATL9, accounting/UI and
COMP2-E remain open under the sole `ACTIVE / PARTIAL` Phase 2D Goal.

No App was built, signed, installed, launched or changed. No real credential,
Provider, external network, user workspace or external Runtime was accessed.
