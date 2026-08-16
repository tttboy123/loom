# P2D-W2C Attempt Loop Authority v1

Status: `SOURCE VERIFIED / RUNTIME INTEGRATION OPEN`

Date: 2026-08-13

## Scope

This slice closes the first ATL1/ATL2 authority boundary inside the existing
Phase 2D Goal. It does not create another Goal and does not claim that a normal
installed Runtime can use the generalized Tool Loop yet.

The audit found that the bounded Attempt Payload fact stream was keyed only by
Run, generation, Execution Binding, and Capsule. A second ToolCall in the same
Attempt therefore conflicted with the first. The compatible repair preserves
the legacy sequence-1 stream and adds call-scoped v2 streams for sequence 2 and
later. A separate non-secret call index is committed atomically with each new
accepted fact, enforcing monotonic, gap-free sequence ownership under races.

## Authority

`AttemptLoopAuthority` freezes:

- exact Attempt/Run, Team/Conversation, Agent, Runtime, claim, and generation;
- Execution Binding and Context Capsule digests;
- permission-profile ID, generation, and digest;
- the capability-set digest, checked against the Run's frozen capabilities;
- tool-schema-set digest and budget-policy version;
- bounded Turn, Step, ToolCall, parallelism, result-size, and timeout limits;
- one Incident ID inherited from the exact running Run.

The append-only stream records `AttemptLoopStarted`, `TurnStarted`,
`StepStarted`, `ModelRequestAdmitted`, `ToolCallAdmitted`,
`ToolDispatchCommitted`, `StepEnded`, and `TurnEnded`. Existing
`ToolResultAccepted` and `ToolResultDelivered` facts remain the result
authority. For generalized loop use, the accepted fact's causation must match
the exact dispatch event before delivery or Step completion is admitted.

Parallel calls require explicit `parallel` mode and distinct conflict-scope
digests. An `exclusive` call is a barrier. Unknown tools, duplicate call IDs,
sequence races, scope overlap, budget overflow, stale generation, binding or
capability substitution, premature result acceptance, and closed-shape replay
drift fail closed.

## Privacy and recovery

Journal payloads contain only opaque IDs, generations, modes, bounded policy
metadata, and SHA-256 digests. Prompt text, ToolCall arguments, Tool result
content, credentials, Authorization headers, and Provider responses remain out
of the Journal. Content continues to live only in the encrypted Attempt Payload
Store.

The authority reconstructs the same deep-copied Turn/Step/ToolCall snapshot
after Authority restart and after the parent Run becomes terminal. Tampered
unknown fields fail strict replay. Concurrent calls competing for one sequence
produce one winner and one authority conflict.

## Verification

- focused and complete `internal/work` tests
- 10-run race matrix for Attempt Loop and multi-call payload authority
- affected Runtime adapter and `cmd/loomd` package tests
- affected-package `go vet`
- full repository `go test ./...`
- `git diff --check`

## Remaining gates

Production daemon composition, RuntimeContract/Tool Gateway integration,
one-shot approval and sandbox evidence, Queue/Steer/Inject inbox authority,
operational diagnostics, Swift governance projection/UI, general result TTL and
compaction, installed CV6, and the live four-Provider Team matrix remain open.
Phase 2D remains the sole `ACTIVE / PARTIAL` Goal.
