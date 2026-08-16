# P2D-W2C/W2D Codex app-server Agent Input continuation v11

Date: 2026-08-14  
Status: `SOURCE VERIFIED / SAME-PROCESS ONLY / INSTALLED LIVE OPEN`

## Boundary

Codex Agent Attempts with a durable Agent Input source may now keep one
version-locked `codex app-server` process and one ephemeral Codex thread across
multiple Loom Queue/Steer/Inject rounds. The frozen Loom Attempt, Route Segment,
Execution Binding, Provider Account, credential revision, model, Context MCP
lease and credential-gateway lease do not change between turns.

This slice does not persist or reattach a Codex native thread after daemon
restart. It does not run the installed Codex binary, Provider network, Loom App,
or installed-live matrix.

## Local protocol lock

Read-only inspection identified the selected Codex package as `0.144.1` and
bound continuation conformance to the exact executable digest:

`sha256:29915529b97697def1a957b0505e770aa6a45744435d62fc263e98d7619e167a`

The local desktop app source confirms newline-delimited JSON, `initialize`,
`thread/start`, repeated `turn/start`, `turn/started`, `item/completed`,
`thread/tokenUsage/updated`, and `turn/completed`. Loom rejects unknown server
requests and notifications, response-ID substitution, thread/turn substitution,
invalid status and accounting drift.

## Implementation

- A shared system session runner starts one exact executable in a private
  process group, exposes bounded JSONL read/write, clears caller-owned input,
  bounds stdout/stderr, preserves executable identity checks, and reaps the
  process group on timeout, cancellation or abort.
- Codex initializes one ephemeral app-server thread and sends each admitted Loom
  input as a new `turn/start` on that same thread. It does not misuse
  `turn/steer`, which is reserved for an already active Codex turn.
- A completed assistant message plus exact Turn completion is the only Agent
  Input checkpoint. `tokenUsage.last` is bound to the same thread/turn and
  accumulated across Loom rounds.
- The Adapter advertises `AgentInputConsumer` only when both the persistent
  runner and exact executable conformance lock pass. Drift fails before
  credential access. Requests without Agent Inputs retain the existing
  one-shot `codex exec --ephemeral` behavior.
- Prompt and JSON wire buffers are mutable and cleared at their bounded
  lifetimes. Raw stderr is discarded; Prompt, Provider body and secret do not
  enter argv, diagnostics, Journal or Evidence.

## Verification

Mandatory RED failed first on the missing system session runner, missing
Adapter continuation contract, and legal pre-response `turn/started`
interleaving. GREEN covers two Loom rounds on one thread, substitution
rejection, bounded process I/O, process-group cancellation, output overflow,
accounting aggregation and one-shot compatibility.

Passed:

```text
go test ./internal/runtime/harnessadapter -count=1
go test -race ./internal/runtime/harnessadapter -count=1
go vet ./internal/runtime/harnessadapter
go test ./internal/runtime/harnessadapter -run 'TestHarnessAttemptCancellationReapsProcessAndPrivatePrompt/claude-code' -count=10
go test -p 1 ./internal/runtime/... ./internal/supervisor/... ./cmd/loomd -count=1
```

One earlier parallel product run recorded a real failure in the pre-existing
three-second Claude cancellation fixture while other packages were under high
load. The focused test then passed ten consecutive runs and the complete
affected matrix passed serially. No production timeout was relaxed.

## Open gates

Codex native thread persistence/reattachment, explicit V10 restart resume
authority and UI, installed Codex conformance, Provider live response, CV6,
ATL9 mixed-Team acceptance, COMP2-E and final accounting/fallback governance
remain open under the sole Phase 2D Goal.
