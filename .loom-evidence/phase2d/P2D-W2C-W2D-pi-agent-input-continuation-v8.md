# P2D-W2C/W2D Pi Agent Input Continuation V8

Status: `SOURCE VERIFIED / MANAGED PI-COMPATIBLE CHILD / INSTALLED LIVE OPEN`

Date: 2026-08-14

## Acceptance boundary

This slice closes the first external Runtime consumption path for the durable
Agent Inbox. After Pi accepts one complete assistant output and emits
`agent_settled`, the Adapter requests the next exact Queue/Steer/Inject batch
using the SHA-256 digest of those accepted assistant bytes. If input is
available, it sends a second native Pi RPC `prompt` to the same managed child
process. It does not restart Pi, replace the Execution Binding, open another
credential lease, or create a second terminal authority.

The second request ID is the daemon-owned Step ID. Prompt bytes are rendered by
the shared Runtime renderer, encoded without converting the input payload into
an immutable Go string, and cleared after the pipe write. The owned Inbox batch,
round prompt, assistant snapshots, and cloned accounting messages are also
cleared at their bounded lifetime.

Bridge sequence and record limits remain monotonic across prompts. Only the
first prompt emits `MessageAck`; all accepted deltas retain one sequence, and
the Adapter emits one Evidence/Result terminal after the final prompt.
Harness-reported accounting is combined across every accepted Pi prompt.

The managed-child regression reads both prompt lines in one shell process,
requires exact JSON bytes for the Agent-input prompt, emits two distinct strict
Pi lifecycles, and proves distinct model-output checkpoints, one terminal,
combined accounting, content-negative Bridge/audit data, and zeroized input.

## Fail-closed boundary

The first prompt retains its frozen Context/Tool capability behavior. A later
Agent-input prompt currently uses the strict text lifecycle inside the same Pi
process. If Pi attempts another Context or Tool event in that continuation, the
Adapter rejects it; this slice does not silently mint another capability use or
claim Context/Tool plus Inbox composition. Official locked Pi 0.82.1 multi-
prompt conformance is not exercised here.

Codex and Claude Code Agent-input transports, daemon-restart checkpoint
reconstruction, post-ModelRequest uncertain recovery, installed CV6, mixed-
Team ATL9, COMP2-E, and real Runtime/App acceptance remain open.

## RED and verification

RED:

```text
TestPiRPCBridgeConsumesAgentInputInSameManagedProcess
Pi RPC adapter does not advertise governed Agent input consumption
```

GREEN gates:

```text
go test -race ./internal/runtime ./internal/runtime/nativeadapter \
  ./internal/runtime/piadapter \
  -run 'Test(RenderAgentInputUsesOneDeterministicMutableEnvelope|LoomNativeConsumesStepAndQueueInputsWithinOneFrozenAttempt|PiRPCBridgeConsumesAgentInputInSameManagedProcess)$' \
  -count=10

go test ./internal/runtime ./internal/runtime/nativeadapter \
  ./internal/runtime/piadapter -count=1

go vet ./...
go test -p 1 ./... -count=1
git diff --check
```

All gates passed. No App, daemon installation, network, real Provider,
credential, user workspace, or external Pi Runtime was accessed.
