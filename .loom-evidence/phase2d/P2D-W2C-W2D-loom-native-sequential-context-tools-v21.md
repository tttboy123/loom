# P2D-W2C/W2D Loom Native Sequential Context Tools V21

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / GENERAL TOOLS AND INSTALLED LIVE OPEN  
**Related**: `contracts/P2D-W2C-agent-attempt-binding-dispatch.md`, `contracts/P2D-W2D-observability-governance.md`

## Accepted boundary

The built-in Loom Native OpenAI-compatible adapter can now execute up to four
strictly sequential `loom_read_context` calls during one Provider exchange:

```text
frozen Agent Attempt
  -> Provider model request
  -> Context ToolCall sequence 1
  -> encrypted result accepted
  -> successful Provider continuation
  -> result 1 delivered
  -> Context ToolCall sequence 2
  -> encrypted result accepted
  -> successful Provider continuation
  -> result 2 delivered
  -> final model output
```

All rounds retain the same immutable Route Segment, Frozen Execution Binding,
Provider Account, credential reference/revision, Model and credential lease.
Every Context retrieval receives a distinct call and payload identity. The
Attempt Loop counts only calls without authoritative delivery proof against the
parallel slot, so `accepted` remains blocking while `delivered` releases the
next strictly sequential call. Total ToolCall sequence and the four-call budget
remain CAS-fenced.

The adapter retains prior assistant tool-call and mutable tool-result messages
only inside the bounded exchange and clears owned result buffers on exit. It
adds usage from every Provider round to one Run accounting value. The request
after four calls does not advertise the Context tool; a Provider that still
returns a fifth call is rejected as nonretryable `context_retrieval_denied`.

The product integration test composes the real Loom Native adapter behind the
product Attempt Loop and proves two Provider continuations produce two exact
admitted/dispatched/accepted/delivered lineages with
`provider_continuation` proof and a successful terminal Step/Turn. The separate
authority test proves an accepted-only result cannot release the serial slot.

Context content, credential bytes and Authorization values are absent from
Journal and Bridge frames. Authorization is removed after every request and
the three Provider rounds aggregate to the expected 55 input, 10 output and 65
total tokens.

## Verification

```text
go test ./internal/work ./internal/runtime/nativeadapter ./cmd/loomd -run 'TestAttemptLoopAuthorityReleasesSequentialSlotOnlyAfterDelivery|TestProductAttemptLoopRuntimeGovernsSequentialContextDelivery|TestProductLoomNativeGovernsSequentialProviderContextContinuations|TestLoomNativeDeepSeekSupportsBoundedSequentialContextToolCalls|TestLoomNativeDeepSeekRejectsContextToolCallBeyondAttemptBound' -count=1 -v
go test -race ./internal/work ./internal/runtime/nativeadapter ./cmd/loomd -run 'TestAttemptLoopAuthority(ReleasesSequentialSlotOnlyAfterDelivery|SerializesConcurrentToolCallSequence)|TestProductAttemptLoopRuntimeGoverns(Sequential)?ContextDelivery|TestProductLoomNativeGovernsSequentialProviderContextContinuations|TestLoomNativeDeepSeek(SupportsBoundedSequentialContextToolCalls|RejectsContextToolCallBeyondAttemptBound|ReturnsScopedContextOnlyToSameProviderAttempt|ResumesPendingContextAcrossProviderCallIDDrift)' -count=10
go test ./internal/work -count=1
go test ./internal/runtime/nativeadapter -count=1
go test ./cmd/loomd -count=1
go vet ./internal/work ./internal/runtime/nativeadapter ./cmd/loomd
git diff --check
```

The first product-level RED failed on the second `Prepare` with an Attempt-loop
conflict. The implementation then passed focused integration, the ten-run race
matrix and all three complete package suites. Verification used only local
fixtures and simulated HTTP responses.

Two supplemental repository-wide runs were not counted as passing gates. The
parallel run timed out in the unrelated Pi local-model health fixture; that
exact test then passed alone. The serial run passed Pi but one existing
`internal/localipc` production-socket Journey returned `local product
unavailable`; that exact test passed 10 isolated runs. A subsequent complete
`internal/localipc` run hit the same transient result in a different socket
round-trip fixture, which also passed 10 isolated runs. V21's affected package,
race, vet and diff gates remain green, while the pre-existing package-level
local-socket instability remains an explicit repository residual.

## Remaining boundary

This slice repeats only the scoped Context retrieval tool. It does not enable
Read, Grep, Web, MCP, arbitrary Provider-native tools, parallel ToolCalls,
additional Runtime transports, installed App execution or real Provider calls.
Trusted observed-result continuation, replacement-Attempt dispatch, installed
CV6, mixed-Team ATL9 and COMP2-E remain open under the sole
`ACTIVE / PARTIAL` Phase 2D Goal.

No App was built, signed, installed, launched or changed. No real credential,
Provider, network, user workspace or external Runtime was accessed.
