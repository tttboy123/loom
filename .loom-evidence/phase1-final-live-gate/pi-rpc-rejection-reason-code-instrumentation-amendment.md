# Final Live Gate Pi RPC Rejection Reason-Code Instrumentation Amendment

Status: CONTRACT AND IMPLEMENTATION REVIEWS `PASS` — LOCAL ATOMIC COMMIT AUTHORIZED; PRE-LIVE REVALIDATION FOLLOWS

- Amendment: `PHASE1-FINAL-LIVE-PI-RPC-REJECTION-REASON-CODE-1`
- Risk: `HIGH`
- Baseline: `a4e6f73cdbcf2dcdc986a5c04eadf9b5c32360ca`
- Date: `2026-07-27`
- Depends on:
  - Final Live Gate Pi RPC Progressive Assistant Identity Contract and
    Implementation Reviews `PASS`;
  - local atomic compatibility commit
    `a4e6f73cdbcf2dcdc986a5c04eadf9b5c32360ca`;
  - the progressive-identity controlled canary's reviewed `FAIL` evidence,
    with its one invocation consumed and no retry performed;
  - fresh read-only repeated-failure problem analysis concluding that the
    exact rejected predicate is not recoverable from retained evidence; and
  - explicit user authorization on `2026-07-27` to freeze, review, and
    implement this reason-code-only amendment and, only after a fresh
    Implementation Reviewer `PASS`, run exactly one new isolated controlled
    diagnostic live canary.

## Proven failure boundary

The progressive-identity canary accepted:

- the correlated Pi RPC command response;
- `agent_start`;
- one `turn_start`;
- the exact user `message_start` and `message_end`; and
- the initial assistant `message_start`.

It failed before any accepted assistant text-delta Frame or assistant terminal:

```text
response=true agent=true turns=1 message=true done=false settled=false
```

The authoritative failure lineage contains one authorized Ack Frame, zero
output Frames, zero output payload bytes, no AdapterResult, and failed terminal,
Grant-revocation, capacity-release, and digest-bound Evidence facts. No raw RPC
transcript was captured or retained.

The locked installed Pi `0.82.1` source proves that the next record may expose
one of several distinct boundaries:

- `text_start` with a newly populated `responseId`;
- `text_start` or a later update with `responseModel` when the Provider stream
  reports a model string different from the requested model;
- a `thinking_*` or `toolcall_*` event;
- a text delta rejected before Frame publication; or
- an empty, error, length, or otherwise non-accepted terminal without a
  preceding accepted text delta.

The current six-field debug state cannot distinguish these predicates. Static
source analysis ranks first-update `responseModel` as the strongest actionable
hypothesis, but does not prove that it occurred. No behavioral widening is
therefore justified.

## Owned files

Product/test ownership is limited to:

- `internal/runtime/piadapter/rpc_bridge_adapter.go`
- `internal/runtime/piadapter/rpc_bridge_adapter_test.go`
- `internal/app/final_live_gate_live_test.go`

Governance evidence ownership is limited to:

- this amendment;
- `pi-rpc-rejection-reason-code-instrumentation-contract-review.md`;
- `pi-rpc-rejection-reason-code-instrumentation-verification.md`;
- `pi-rpc-rejection-reason-code-instrumentation-implementation-review.md`;
- after the implementation commit and immediately before the one authorized
  invocation,
  `resolved-live-manifest-rejection-diagnostic-canary.json`;
- after that invocation, `rejection-diagnostic-live-canary.md`; and
- the private materialization status for bounded result synchronization only.

All pre-existing final-live evidence is a read-only input, including
`contract.md`, `contract-review.md`, `source-lock.json`,
`source-provenance-amendment.md`, `live-canary.md`,
`replacement-live-canary.md`, `additional-live-canary.md`,
`progressive-identity-live-canary.md`, every earlier resolved manifest, and
all earlier contract, verification, and review evidence. Every existing private
attempt root and its SQLite/Evidence descendants is read-only. This amendment
must not rename, overwrite, append to, migrate, or delete any of them.

No other Go file, dependency, module, ADR, policy, validator, StateWriter,
Journal, Projection, Bridge v1 protocol, Supervisor, Grant, Evidence authority,
CLI, daemon, scheduler, Runtime installation, model, source lock, credential,
environment file, or user-owned dirty file is owned.

## Frozen diagnostic contract

### Observability only

This amendment must preserve the exact accepted and rejected transcript sets at
baseline. For every input transcript:

```text
baseline accepts  <=> instrumented Candidate accepts
baseline rejects  <=> instrumented Candidate rejects
```

It must not:

- permit `responseModel`;
- relax response-ID, timestamp, usage, content, event, terminal, identity, or
  exact-key validation;
- accept thinking, tools, extensions, retries, compaction recovery, non-`stop`
  completion, Provider/model substitution, or unknown records;
- change Bridge/binding/sequence validation or `AgentGrant.Authorize`;
- change Frame payloads, order, count, digest, or Evidence semantics;
- add a retry, fallback, Provider/model switch, parser compatibility mode, or
  transcript logger; or
- make diagnostic output a new state authority.

### Closed diagnostic vocabulary

The implementation must add internal, closed, string-backed enums. No enum may
accept caller-supplied or transcript-derived text.

Allowed phases:

```text
assistant_update
assistant_message_end
turn_end
agent_end
agent_settled
```

Allowed event values:

```text
text_start
text_delta
text_end
thinking_start
thinking_delta
thinking_end
toolcall_start
toolcall_delta
toolcall_end
message_end
turn_end
agent_end
agent_settled
unknown
```

Allowed reason codes:

```text
event_shape
event_kind_unsupported
content_index
message_partial_mismatch
assistant_message_schema
response_id_transition
response_model_present
timestamp_identity
usage_schema
usage_progression
text_content_progression
delta_policy
terminal_stop_reason
terminal_identity
frame_sink
```

The diagnostic error must:

- preserve `errors.Is(err, ErrPiRPCProtocol) == true`;
- use the exact bounded rendering
  `Pi RPC protocol failed: phase=<phase> event=<event> reason=<reason>`;
- contain exactly one allowed phase, event, and reason;
- describe only the first rejected record/predicate; and
- remain inspectable through normal Go error wrapping and joining.

There is no `other`, free-form detail, numeric ordinal, raw field name/value,
model value, response ID, usage value, path, or Provider payload slot.

### Deterministic first-rejection classification

Within the proven post-assistant-start boundary, classification must follow the
same short-circuit acceptance order as the baseline. It may refactor an
internal boolean validator into a closed-result validator only when tests prove
acceptance equivalence.

For assistant updates, the precedence is:

1. event object/type and exact event shape;
2. supported event kind;
3. text lifecycle progression;
4. content index;
5. top-level message versus nested partial semantic equality;
6. assistant message schema;
7. `responseModel` presence;
8. response-ID transition;
9. timestamp identity;
10. usage schema and monotonic usage-field presence;
11. text/delta policy; and
12. Frame construction or FrameSink authorization.

Where multiple invalid predicates coexist, the first predicate in this order
is the only reported reason. Tests must freeze this precedence without changing
the rejection result.

For assistant `message_end`, `turn_end`, `agent_end`, and `agent_settled`, the
diagnostic must distinguish only the corresponding closed phase/event and the
applicable existing reason family. It must not serialize a terminal message or
any terminal field value.

An output-size failure remains `ErrPiRPCOutputTooLarge`. Cleanup and process
errors retain their accepted error identities. The reason-code error is only
for `ErrPiRPCProtocol` rejection inside the frozen post-assistant-start
transcript boundary.

### Non-disclosure

The diagnostic error, repository evidence, private Evidence, and live-test
output must never contain:

- a raw JSON record or fragment;
- transcript text, text delta, model output, or hidden reasoning;
- the prompt or system prompt;
- raw Grant material, credentials, or environment values;
- response IDs, model/provider strings, usage numbers, timestamps, Event IDs,
  Run IDs, or filesystem paths; or
- any arbitrary text derived from Pi, llama.cpp, a Provider, or a fixture.

The live result may retain only the closed enum triple, existing bounded state
booleans/counts, digests, modes, ownership, Event types/statuses, and
authorization accounting.

## Fresh isolated diagnostic-canary boundary

Only after a fresh Implementation Reviewer returns `PASS` and the Candidate is
atomically committed may one new invocation run.

The new invocation must:

1. create one unpredictable direct-child attempt directory under the already
   validated private root, with prefix
   `controlled-canary-rejection-diagnostic-`;
2. require exact current-user ownership, non-symlink directory type, and mode
   `0700`;
3. create all metadata isolation, Runtime home/temp, bounded source, exclusive
   pre-created `0600` SQLite, and Evidence only below that new root;
4. never select, reopen, migrate, or append to any prior attempt;
5. retain the exact no-retry/no-compaction settings and all accepted Final Live
   Gate bindings;
6. exclusively create the independent sanitized manifest
   `resolved-live-manifest-rejection-diagnostic-canary.json`;
7. use exactly one opt-in test invocation; and
8. retain the new attempt and evidence whether the result succeeds or fails.

Every prior manifest and attempt must remain byte-for-byte unchanged. The new
manifest must contain no private path, mirror URL, prompt text, raw Grant,
credential, hidden reasoning, model output, or diagnostic field value derived
from the Runtime.

Authorization accounting is exact: this amendment authorizes one invocation.
After that invocation, remaining authorization is zero. Any test failure,
timeout, protocol rejection, Runtime/model failure, cleanup defect, ambiguous
result, or success consumes the authorization and stops the line immediately.
There is no retry, fallback, compaction, model/provider switch, parser widening,
or second invocation.

## Mandatory RED

After Contract Reviewer `PASS` and before production/harness changes, tests
must encode:

1. exact reason-code errors for first-update `responseModel`, missing or invalid
   response-ID transition, timestamp drift, malformed usage, usage-field
   regression, top-level/nested partial mismatch, unsupported thinking/tool
   event, invalid content index, invalid/empty delta, and text progression;
2. exact closed phase/event/reason output with
   `errors.Is(err, ErrPiRPCProtocol) == true`;
3. deterministic precedence when two invalid predicates coexist;
4. reason-coded assistant `message_end`, `turn_end`, `agent_end`, and
   `agent_settled` rejection;
5. acceptance/rejection equivalence for the existing progressive transcript
   and complete adversarial matrix;
6. absence of fixture text, prompt, raw Grant, response/model values, arbitrary
   Runtime text, JSON fragments, numeric field values, and private paths from
   every reason-coded error; and
7. the distinct diagnostic manifest name and exact
   `controlled-canary-rejection-diagnostic-*` fresh-root prefix.

Focused RED commands:

```text
go test ./internal/runtime/piadapter \
  -run '^TestPiRPC(RejectionReasonCodes|RejectionReasonCodeNonDisclosure|RejectionReasonCodePrecedence)$' \
  -count=1

go test ./internal/app \
  -run '^TestFinalLiveRejectionDiagnosticCanaryIsolation$' \
  -count=1
```

The adapter RED must fail because baseline returns only the coarse
`ErrPiRPCProtocol` plus six-field debug state. The app RED must fail because
baseline still uses the progressive-identity manifest and attempt-root prefix.

RED output itself must not contain a private absolute path, prompt, credential,
raw RPC record, raw Grant, hidden reasoning, or model output.

## Minimal GREEN

Implementation is limited to:

- closed internal phase, event, and reason enums;
- one bounded error type that unwraps to `ErrPiRPCProtocol`;
- deterministic first-rejection classification in the frozen
  post-assistant-start boundary;
- hermetic reason-code, precedence, non-disclosure, and transcript-equivalence
  tests;
- changing only the opt-in live harness manifest filename and fresh
  attempt-root prefix; and
- no other behavior change or refactor.

No generic compatibility framework, free-form logger, telemetry system,
transcript capture, Provider-specific acceptance rule, Bridge v1 change,
scheduler, daemon, schema migration, new dependency, or unrelated cleanup is
allowed.

## Verification matrix

Before Implementation Review:

```text
go test ./internal/runtime/piadapter \
  -run '^TestPiRPC(RejectionReasonCodes|RejectionReasonCodeNonDisclosure|RejectionReasonCodePrecedence)$' \
  -count=1

go test ./internal/app \
  -run '^TestFinalLiveRejectionDiagnosticCanaryIsolation$' \
  -count=1

go test ./internal/runtime/piadapter ./internal/app -count=1

go test -race ./internal/runtime/piadapter \
  -run '^TestPiRPC(RejectionReasonCodes|RejectionReasonCodeNonDisclosure|RejectionReasonCodePrecedence)$' \
  -count=30

go test -race ./internal/app \
  -run '^TestFinalLiveRejectionDiagnosticCanaryIsolation$' \
  -count=30

go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go mod verify
gofmt -d \
  internal/runtime/piadapter/rpc_bridge_adapter.go \
  internal/runtime/piadapter/rpc_bridge_adapter_test.go \
  internal/app/final_live_gate_live_test.go
git diff --check
GOOS=windows GOARCH=amd64 go test -exec=true \
  ./internal/runtime/piadapter ./internal/app
```

Scope and equivalence checks must prove:

- only the exact three owned Go files and this amendment's evidence changed in
  this lineage;
- every pre-existing accepted fixture still succeeds;
- every pre-existing rejected fixture still fails with the same primary error
  identity;
- no module/dependency, Bridge v1, Journal/Projection/StateWriter, policy,
  Grant/Evidence authority, Runtime/model/source-lock, credential, `.env`,
  daemon, scheduler, CLI, or queued product scope changed;
- no live Runtime/model/canary ran during RED, GREEN, or review;
- all historical evidence and private-attempt hashes remain unchanged; and
- no user-owned dirty file is staged or committed.

## Review, commit, and live gates

1. Fresh Contract Reviewer must return `PASS` before RED.
2. Fresh Implementation Reviewer must inspect the complete Candidate, genuine
   RED and GREEN evidence, exact acceptance equivalence, closed-vocabulary
   non-disclosure, scope, trust boundary, and historical hash preservation.
3. Any Critical or Important finding blocks commit and live execution.
4. After Reviewer `PASS`, stage only owned implementation/tests and this
   amendment's pre-live governance evidence, review the staged diff, and create
   one local atomic commit. No push or merge is authorized.
5. Immediately before live execution, revalidate installed Pi/llama/model and
   locked source hashes, private ownership/modes, port/process cleanup,
   historical evidence/attempt hashes, authorization environment, and absence
   of any pre-existing new manifest or attempt.
6. Create the independent sanitized manifest, then execute the exact opt-in
   test once.
7. Stop immediately after that invocation. A fresh read-only result-evidence
   Reviewer must audit invocation count, closed enum vocabulary,
   non-disclosure, SQLite/Event lineage, modes, digests, cleanup, historical
   preservation, and the stated verdict.

## Exit state

This amendment is diagnostic only and cannot by itself reach
`READY_FOR_FINAL_USER_SIGNOFF`.

If the controlled canary fails with one valid closed reason code and fresh
result-evidence Review `PASS`, the state becomes:

```text
HUMAN_REQUIRED — DIAGNOSED — NO RETRY
```

Any behavioral compatibility change then requires a separately frozen and
reviewed amendment. The reason code alone never authorizes acceptance
widening.

If the controlled canary succeeds despite no acceptance change, the repeated
historical failures make the result nondeterministic. The state becomes:

```text
HUMAN_REQUIRED — NONDETERMINISTIC RESULT — NO RETRY
```

If the invocation lacks exactly one valid enum triple, leaks disallowed data,
fails cleanup/evidence checks, or is otherwise ambiguous, the state remains:

```text
HUMAN_REQUIRED — NO RETRY
```

No outcome grants final user sign-off, Phase 1 completion, production
activation, autonomous execution, daemon activation, push, merge, or release.
