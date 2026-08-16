# P2D-W2C/W2D Pi Sequential Tool Continuation V2

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / ATL BREADTH PARTIAL  
**Related**: `P2D-COMP2-D-sequential-tool-turn-scope-v5.md`, `P2D-W2D-pi-native-tool-continuation-v1.md`

## Accepted boundary

A real local managed child now completes two bounded sequential governed
ToolCalls through one Pi RPC invocation:

```text
managed Pi-compatible child
  -> private Tool UDS call, sequence 1
  -> distinct execution/payload lineage 1
  -> private Tool UDS call, sequence 2
  -> distinct execution/payload lineage 2
  -> validated final assistant output
  -> final-output ACK for result 1 and result 2
```

The child process is launched through the production Pi RPC process path. It
receives only the generated extension path, parses the private socket and
capability, makes two separate peer-attested UDS connections, consumes each
content-free result, and emits the complete three-turn transcript. One child
process and one Attempt-scoped extension own the whole continuation.

Each call receives an explicit sequence through `ToolCallSequence`. The hook
returns distinct execution IDs, payload IDs, call IDs, result digests, and
delivery sequence. The Adapter accepts the final output only after both result
messages match the resolved extension calls, then acknowledges both with
`harness_final_output` proof. Audit output contains two content-free result
entries; Bridge frames contain neither commands nor payload authority IDs.

The first managed-child run was RED because the sequential fixture embedded a
unit-only prompt constant. The production parser correctly rejected the real
rendered Role Context prompt as an identity mismatch. The fixture now requires
the exact child prompt while preserving its existing unit wrapper; no
production authority or protocol was relaxed.

The separate COMP2-D V5 evidence already proves sequence 1 and 2 advance the
Attempt-owned Turn controller from Turn 1 to Turn 2 to Turn 3 only after
governed result persistence. This artifact proves the real managed-child
continuation boundary; it does not duplicate or replace that authority proof.

No command, path, Prompt, Context Capsule content, tool result content,
credential, Provider response, capability value, payload authority, or child
output is written to Journal, diagnostics, Evidence, or this artifact.

## Verification

```text
go test ./internal/runtime/piadapter -run '^TestPiRPCSequentialToolExecuteUsesOneManagedChildAndDistinctLineage$' -count=1 -v
go test ./internal/runtime/piadapter -run 'Test(PiRPCSequentialToolExecute|PiRPCToolProtocolAcceptsBoundedSequential|PiToolExtensionAcceptsBoundedSequential)' -count=1 -v
go test -race ./internal/runtime/piadapter -run '^TestPiRPCSequentialToolExecuteUsesOneManagedChildAndDistinctLineage$' -count=10
go test ./internal/runtime/piadapter -count=1
go test ./... -count=1
go vet ./...
git diff --check
```

The first focused run was RED on prompt identity and GREEN after the test
fixture accepted the exact managed-child prompt. The focused protocol group,
10-run race, full Pi Adapter, full repository, vet, and diff checks pass.

## Remaining boundary

This is a deterministic local Pi-compatible child, not the locked official Pi
0.82.1 binary, a real Provider, or an installed App. It does not yet combine
the product daemon's real Tool hook and Composition Turn controller with two
real tool backends in one live invocation. Parallel ToolCalls, Claude/Codex and
Loom Native tool transports, Queue/Steer/Inject, recovery commands, production
Web/MCP backends, accounting/UI, CV6, ATL9, and installed acceptance remain
open.

No App was built, signed, installed, launched, or changed. No real credential,
Provider, network, user workspace, official Pi binary, or MCP server was
accessed.
