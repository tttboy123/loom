# P2D-W2D Pi governed Read/Grep content v1

**Status**: SOURCE VERIFIED / INSTALLED LIVE OPEN  
**Date**: 2026-08-14  
**Goal**: sole Phase 2D Goal

## Acceptance boundary

This increment adds bounded Read/Grep execution without widening Journal,
Evidence, diagnostics or RunStream into content stores. The Unix executor walks
the workspace with descriptor-relative `openat` and `O_NOFOLLOW`, accepts only
single-link regular UTF-8 files, bounds source and returned bytes, revalidates
file identity/content, and zeroizes owned buffers. Grep pattern syntax and path
binding are validated before dispatch.

After the exact Attempt ToolCall dispatch is committed, the result content is
written to the Conversation-DEK Attempt Payload with the exact UTF-8 text
content type. `ToolResultAccepted` freezes its digest. The private Pi extension
may decrypt only that pending binding, recomputes the digest, and returns the
text in Pi's native `toolResult`. Ordinary Loom Bridge frames and tool audit
remain content/path free. Delivery requires a validated second-turn final
assistant output and `harness_final_output` proof.

The execution Adapter never caches plaintext. Same-process and restart replay
may re-read a local read-only target only when the output digest is unchanged;
drift returns `ErrExecutionContent`. Any failure after content acquisition and
before successful ownership transfer zeroizes the result buffer.

## Verification

```text
go test ./internal/work ./internal/permissions ./internal/execution ./internal/runtime/piadapter ./cmd/loomd -count=1
PASS

go test -race ./internal/execution -run 'TestP2D(Read|InvalidGrep)' -count=10
PASS

go test -race ./internal/runtime/piadapter -run 'TestPi(RPCToolProtocolAcceptsBoundRead|ToolExtensionReturnsBoundRead)' -count=10
PASS

go test -race ./internal/runtime/piadapter -run TestPiRPCReadToolExecuteReturnsPrivateContentToChildOnly -count=10
PASS

go test -race ./cmd/loomd -run TestProductAttemptLoopRuntimeEncryptsReadAndGrepContentUntilHarnessDelivery -count=10
PASS

go test ./... -count=1
PASS

go vet ./...
PASS

git diff --check
PASS
```

Regression coverage includes dispatch-before-access, invalid Grep preflight,
descriptor/path/symlink/binary/size controls, encrypted Read and Grep payloads,
content-free Journal and Evidence, cache/restart digest revalidation, result
zeroization after commit/replay failure, Pi content-digest substitution
rejection, managed child-process private Read delivery and Bridge/audit
non-disclosure.

## Open boundary

The actual managed Pi child-process canary now proves Read content delivery and
second-turn continuation in addition to Bash. Multiple sequential ToolCalls,
Web/MCP, other Runtime adapters, sandbox reports, recovery commands,
Queue/Steer/Inject, Swift governance and installed CV6/live mixed-Team
acceptance remain open.

Web/MCP may not use read-only reconstruction. Their result bytes must be
encrypted durably before the execution terminal fact so restart never repeats a
completed remote call merely to recover content. No App bundle was built,
signed, launched or installed, and no real credential, Provider, user
workspace, network or external tool was accessed.
