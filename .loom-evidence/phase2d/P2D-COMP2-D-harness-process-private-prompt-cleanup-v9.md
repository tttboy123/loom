# P2D-COMP2-D Harness Process and Private Prompt Cleanup V9

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / COMP2-D PARTIAL  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

The Claude Code and Codex Harness process runners now treat their private
system-prompt materialization and cleanup as one fail-closed Attempt operation:

```text
Agent Attempt execution context
  -> exact credential lease / loopback Attempt gateway token
  -> owner-only temporary system-prompt file
  -> Harness child process group
  -> cleanup file and private directory
```

Both runners previously deferred `cleanupPrompt()` without observing its error.
A blocked or replaced cleanup path could therefore leave sensitive system
instructions on disk while the Harness invocation returned success. The new
RED replaces the prompt file with a non-empty directory during command
execution and proves that both Claude Code and Codex previously ignored the
cleanup failure.

Each gateway callback now has a named error result and joins prompt cleanup into
every return path. Cleanup failure is `ErrHarnessProtocol` even if the Harness
output itself is valid. Existing Provider/auth/rate-limit/process errors remain
joined rather than overwritten.

A second test uses the real system command runner for both Harnesses. Each child
starts a long-lived descendant and records only its PID. Cancelling the Attempt
context kills the complete process group, returns `context.Canceled`, and
removes `.loom-private/loom-system-prompt.txt` plus its owner-only directory.
The child cannot exit naturally within the test bound, so successful return
proves cancellation cleanup.

No Prompt content, Provider key, gateway token, child output, or environment is
written to diagnostics, Journal, Evidence, or this acceptance artifact.

## Verification

```text
go test ./internal/runtime/harnessadapter -run '^TestHarnessProcessesFailClosedWhenSystemPromptCleanupFails$' -count=1
go test ./internal/runtime/harnessadapter -run 'Test(HarnessAttemptCancellationReapsProcessAndPrivatePrompt|HarnessProcessesFailClosedWhenSystemPromptCleanupFails)' -count=1 -v
go test -race ./internal/runtime/harnessadapter -run 'Test(HarnessAttemptCancellationReapsProcessAndPrivatePrompt|HarnessProcessesFailClosedWhenSystemPromptCleanupFails)' -count=10
go test ./internal/runtime/harnessadapter -count=1
go test ./... -count=1
go vet ./...
git diff --check
```

The first command was captured RED before implementation and GREEN after the
error join. All final commands pass; the 10-run race suite reports no races.

## Remaining boundary

This verifies local process-group and private system-prompt cleanup for the
current Claude Code and Codex Harness runners. It does not claim live Provider
dispatch, installed CLI compatibility, full Attempt gateway/Context MCP
shutdown under crash, Harness-owned user workspace rollback, or other Runtime
adapters. Those source/live gates remain open with Provider native-handle
adoption, COMP2-E, CV6, ATL9, UI/accounting, and installed acceptance.

No App was built, signed, installed, launched, or changed. No real credential,
Provider, network, user workspace, Claude CLI, or Codex CLI was accessed.
