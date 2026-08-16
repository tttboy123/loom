# P2D-COMP2-C Work Execution Ownership V3

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / COMP2-C CONTINUES  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

`loom-work` now constructs and owns governed Execution beside Queue, Workers,
and Integration during Bundle Start. Its typed route set contains the existing
local Execution API and a separate private tool-execution port limited to
`Execute` and `RemoteToolKinds`. Product construction no longer creates or owns
the Execution Evidence store, decision recorder, sandbox gate, Adapter,
recovery replay, service, or API.

The factory opens an owner-only Execution Evidence store, constructs the
existing Adapter with the same approval, sandbox and optional remote-executor
semantics, and completes `ReplayPending` before the Bundle may become Ready.
Construction and recovery failures preserve the exact `build_execution` or
`build_execution_recovery` stage through Composition. Any partial owner is
closed on rollback.

`loom-agent-runtime` starts after Work and obtains only the bounded private tool
port from the typed Work slot. It does not receive a concrete Adapter through
Capability Context. Reverse shutdown closes Agent Runtime before Work; Work
waits for in-flight route calls, revokes the Execution API and tool port, and
closes the Evidence owner exactly once. This also removes the successful-start
Execution Evidence lifetime leak from the product root.

No service object, request content, Journal writer, StateWriter, terminal
authority, policy/grant authority, Vault root, credential, VMK, Prompt,
transcript, Provider body, tool result, or Context Capsule is stored in
Capability Context. Existing Frozen Execution Binding and Attempt authority
checks remain the execution boundary.

## Verification

```text
go test ./cmd/loomd -run '^(TestCOMP2CWork|TestCOMP2CAgentRuntime|TestCOMP2CProductionBuilder)' -count=1
go test ./cmd/loomd -run '^(TestCOMP2C|TestBridgeExecutionHook|TestProductMissionExecution)' -count=1
go test -race ./internal/composition ./cmd/loomd -run '^(TestCOMP1|TestCOMP2[ABCD]|TestBw1ExecutionHandlerWiredThroughComposition|TestWBridge|TestProductMissionExecutionCompositionReconcilesExactJournalLineageBeforeIPC)' -count=10
go test ./cmd/loomd -count=1
go test ./...
go vet ./...
git diff --check
```

Focused tests prove atomic Ready, exact construction/recovery error identity,
recovery-before-admission, failed-Start rollback, Execution route and tool-port
revocation, Assets -> Work -> Agent Runtime startup order, reverse cleanup, and
idempotent close. AST checks prove production construction no longer owns the
Execution Evidence store, Adapter, or local Execution API. Full daemon and
repository suites preserve existing dispatch, approval, sandbox, remote-tool,
Agent execution, shutdown, diagnostic, and privacy behavior.

## Open boundary

This does not complete `loom-work` or COMP2-C/D. At this V3 boundary,
Production, governance/work authority, saved-Team setup/read routes,
Conversation construction, observability handoff, and deeper Capability Context
scopes remained open. Production ownership was subsequently migrated and source
verified by `P2D-COMP2-C-work-production-ownership-v4.md`; the other listed
boundaries remain open. The
operational store retains its documented pre-composition bootstrap role until
Conversation migration can preserve all startup failures.

COMP2-E, installed CV6, mixed-Team ATL9, and all live UI gates remain open. No
App was built, signed, installed, or launched. No credential, Provider, network,
user workspace, or external tool was accessed.
