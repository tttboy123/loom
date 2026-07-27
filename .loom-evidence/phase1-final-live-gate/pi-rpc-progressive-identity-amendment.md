# Final Live Gate Pi RPC Progressive Assistant Identity Amendment

Status: CONTRACT AND IMPLEMENTATION REVIEWS `PASS` — LOCAL ATOMIC COMMIT AUTHORIZED; PRE-LIVE REVALIDATION FOLLOWS

- Amendment: `PHASE1-FINAL-LIVE-PI-RPC-PROGRESSIVE-IDENTITY-1`
- Risk: `HIGH`
- Baseline: `e32f65035ec7c477bbee9e60cffa7c0989b028ce`
- Date: `2026-07-27`
- Depends on:
  - Final Live Gate Pi RPC Schema and Fresh Attempt Isolation Contract and
    Implementation Reviews `PASS`;
  - local atomic compatibility commit
    `e32f65035ec7c477bbee9e60cffa7c0989b028ce`;
  - the additional controlled canary's reviewed `FAIL` evidence, with its
    authorization consumed and no retry performed; and
  - explicit user authorization on `2026-07-27` to freeze, review, and
    implement this amendment, and, only after fresh Implementation Reviewer
    `PASS`, run exactly one new isolated controlled live canary.

## Exact failure and source-bound diagnosis

The one authorized additional canary passed the response, Agent, turn, user
message, and assistant `message_start` gates, then failed closed before an
accepted assistant terminal:

```text
response=true agent=true turns=1 message=true done=false settled=false
```

The locked installed Pi `0.82.1` source has the exact hashes frozen in
`source-lock.json`. Its `openai-completions` adapter:

1. creates the initial assistant partial without `responseId`;
2. emits `message_start` for that initial partial;
3. receives the first Provider chunk and adds the bounded non-empty chunk ID
   as `responseId` before emitting the first assistant text update;
4. may add final usage field `reasoning` when Provider usage arrives; and
5. forwards each progressive partial through Pi Agent Core and RPC mode.

The accepted Loom adapter currently serializes assistant identity at
`message_start`, including the empty `responseId` and the absence of optional
usage fields, then requires byte-equivalent identity on the first text update.
It therefore rejects the legitimate Pi `0.82.1` absent-to-present
`responseId` transition before output can become terminal.

This is a source-level diagnosis bound to the installed locked hashes. It does
not claim that a raw live RPC transcript was captured or persisted.

## Owned files

Product/test ownership is limited to:

- `internal/runtime/piadapter/rpc_bridge_adapter.go`
- `internal/runtime/piadapter/rpc_bridge_adapter_test.go`
- `internal/app/final_live_gate_live_test.go`

Governance evidence ownership is limited to:

- this amendment;
- `pi-rpc-progressive-identity-contract-review.md`;
- `pi-rpc-progressive-identity-verification.md`;
- `pi-rpc-progressive-identity-implementation-review.md`;
- after the implementation commit and immediately before the one authorized
  invocation,
  `resolved-live-manifest-progressive-identity-canary.json`;
- after that invocation, `progressive-identity-live-canary.md`; and
- the private materialization status for bounded result synchronization only.

All pre-existing final-live evidence is a read-only input, including
`additional-live-canary.md`,
`resolved-live-manifest-additional-canary.json`, `live-canary.md`,
`replacement-live-canary.md`, `resolved-live-manifest.json`, `contract.md`,
`contract-review.md`, and `source-lock.json`. Every existing private attempt
root and its SQLite/Evidence descendants is read-only. This amendment must not
rename, overwrite, append to, migrate, or delete any of them.

No other Go file, dependency, module, ADR, policy, validator, StateWriter,
Journal, Projection, Bridge v1 protocol, Supervisor, Grant, Evidence authority,
CLI, daemon, scheduler, Runtime installation, model, source lock, credential,
environment file, or user-owned dirty file is owned.

## Frozen progressive assistant identity state machine

The implementation must replace the single serialized identity comparison with
one bounded, transcript-local identity state. It must not introduce a generic
compatibility framework.

### Immutable fields

At assistant `message_start`, Loom must validate and bind:

- `role == "assistant"`;
- `api == "openai-completions"`;
- `provider == "loom-local"`;
- `model == "qwen2.5-coder-1.5b-instruct-q4-k-m"`; and
- the exact finite non-negative numeric `timestamp`.

These values must remain semantically identical at every `message_update`,
`message_end`, `turn_end`, and `agent_end`. Any removal, type change, value
change, normalization, duplicate key, or replacement fails closed.

### Initial assistant

The initial `message_start` must:

- contain exactly zero content blocks;
- have valid usage;
- omit `responseId`;
- omit `responseModel`;
- omit usage field `cacheWrite1h`; and
- omit usage field `reasoning`.

The exact Pi `0.82.1` locked local path supplies this initial form. A
pre-populated or empty-string `responseId`, any `responseModel`, or either
optional usage field at `message_start` fails closed.

### Response ID late binding

The first accepted assistant `message_update` must be `text_start`. Its
top-level `message` and nested `partial` must remain semantically equal and must
introduce exactly one bounded, non-empty `responseId`.

Loom must then bind that exact `responseId`. Every later assistant partial and
terminal message must contain the same value. Missing, empty, oversized,
changed, duplicated, removed, reintroduced, or late-first-introduced
`responseId` fails closed.

Only this single transition is allowed:

```text
message_start: responseId absent
    -> first text_start update: responseId present and bound
    -> every later assistant/terminal record: exact same responseId
```

### Response model

`responseModel` must remain absent for the entire locked local transcript.
Installed Pi adds it only when the Provider reports a model different from the
requested model. Any presence therefore represents model substitution and
fails closed, even if the value resembles the configured model ID.

### Usage schema progression

Every usage object must independently pass the existing exact numeric and
allowed-key validator. Numeric usage values may progress as Pi receives
Provider chunks; they are not identity fields.

For the exact locked local path:

- `cacheWrite1h` must remain absent for the entire transcript;
- `reasoning` may make at most one monotonic schema transition from absent to
  present on an assistant update;
- after `reasoning` first appears, it must remain present on every later
  assistant partial and terminal message; and
- removal, reappearance after removal, duplicate keys, invalid values, or any
  other usage-schema change fails closed.

`reasoning` is not required to appear when the Provider supplies no terminal
usage. If it appears, its numeric value may still progress subject to the
existing usage validator.

### Existing transcript and authority checks remain exact

The amendment must preserve:

- exact top-level `message` to nested `partial` semantic equality;
- exactly one text block, bounded exact accumulated text, and content-index
  zero;
- frame Bridge/binding/sequence validation and `AgentGrant.Authorize` before
  any observer delivery;
- strict rejection of tools, thinking, extensions, unknown or duplicate
  fields, assistant errors, prompt drift, identity drift, nested Provider
  `start`/`done`/`error`, retries, compaction recovery, non-`stop` completion,
  oversized output, and Grant disclosure;
- exact final assistant semantic equality across `message_end`, `turn_end`,
  and `agent_end`;
- `willRetry == false`; and
- terminal `agent_settled` sequencing.

No raw RPC transcript, model output, raw Grant, credential, hidden reasoning,
or private absolute path may be added to Journal or repository evidence.

## Fresh isolated controlled-canary boundary

Only after a fresh Implementation Reviewer returns `PASS` and the Candidate is
atomically committed may one new invocation run.

The new invocation must:

1. create one unpredictable direct-child attempt directory under the already
   validated private root, with prefix
   `controlled-canary-progressive-identity-`;
2. require exact current-user ownership, non-symlink directory type, and mode
   `0700`;
3. create all metadata isolation, Runtime home/temp, bounded source, exclusive
   `0600` SQLite, and Evidence only below that new root;
4. never select, reopen, migrate, or append to any prior attempt;
5. retain the exact no-retry/no-compaction settings and all accepted
   Final Live Gate bindings;
6. exclusively create the independent sanitized manifest
   `resolved-live-manifest-progressive-identity-canary.json`;
7. use exactly one opt-in test invocation; and
8. retain the new attempt and evidence whether the result succeeds or fails.

The prior additional manifest and attempt must remain byte-for-byte unchanged.
The manifest must contain no private path, mirror URL, prompt text, raw Grant,
credential, hidden reasoning, or model output.

Authorization accounting is exact: this amendment authorizes one invocation.
After that invocation, remaining authorization is zero. Any test failure,
timeout, protocol rejection, Runtime/model failure, cleanup defect, or
ambiguous result consumes the authorization and stops the line immediately.
There is no retry, fallback, compaction, model/provider switch, parser widening,
or second invocation.

## Mandatory RED

After Contract Reviewer `PASS` and before production/harness changes, tests
must encode:

1. one exact Pi `0.82.1` progressive assistant transcript:
   `message_start` without `responseId`, first `text_start` with one
   `responseId`, later partials with the same value, optional monotonic
   `reasoning` appearance, and exact terminal equality;
2. rejection of response-ID prepopulation, absence at `text_start`, removal,
   mutation, empty/oversized values, and later first introduction;
3. rejection of any `responseModel`;
4. rejection of immutable-field drift and top-level/nested partial mismatch;
5. rejection of `cacheWrite1h` and `reasoning` removal/reappearance;
6. preservation of the existing tool/thinking/retry/unknown/error/size/Grant
   rejection matrix; and
7. a distinct progressive-identity manifest name and exact
   `controlled-canary-progressive-identity-*` fresh-root prefix.

Focused RED commands:

```text
go test ./internal/runtime/piadapter \
  -run '^TestPiRPC(Pi0821ProgressiveAssistantIdentity|ProgressiveAssistantIdentityRejections)$' \
  -count=1

go test ./internal/app \
  -run '^TestFinalLiveProgressiveIdentityCanaryIsolation$' \
  -count=1
```

The valid progressive transcript test must fail on baseline because baseline
rejects absent-to-present `responseId`. The app test must fail because baseline
still uses the previous manifest name and attempt-root prefix.

RED output must not contain a private absolute path, prompt, credential, raw
RPC record, raw Grant, hidden reasoning, or model output.

## Minimal GREEN

Implementation is limited to:

- one bounded assistant-identity state with immutable identity fields,
  first-update response-ID binding, absent `responseModel`, absent
  `cacheWrite1h`, and monotonic optional `reasoning` presence;
- routing existing assistant transcript phases through that state;
- exact Pi `0.82.1` progressive and adversarial fixtures;
- changing the opt-in live harness to the new manifest filename and fresh
  attempt-root prefix; and
- no other behavior change or refactor.

No broad Pi schema compatibility mode, Provider-specific abstraction, generic
streaming identity framework, retry coordinator, transcript logger, Bridge v1
change, scheduler, daemon, schema migration, new dependency, or unrelated
cleanup is allowed.

## Verification matrix

Before Implementation Review:

```text
go test ./internal/runtime/piadapter \
  -run '^TestPiRPC(Pi0821ProgressiveAssistantIdentity|ProgressiveAssistantIdentityRejections)$' \
  -count=1

go test ./internal/app \
  -run '^TestFinalLiveProgressiveIdentityCanaryIsolation$' \
  -count=1

go test ./internal/runtime/piadapter ./internal/app -count=1

go test -race ./internal/runtime/piadapter \
  -run '^TestPiRPC(Pi0821ProgressiveAssistantIdentity|ProgressiveAssistantIdentityRejections)$' \
  -count=30

go test -race ./internal/app \
  -run '^TestFinalLiveProgressiveIdentityCanaryIsolation$' \
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

Scope checks must prove:

- only the exact three owned Go files and new amendment evidence changed in
  this lineage;
- no module/dependency, Bridge v1, Journal/Projection/StateWriter, policy,
  Grant/Evidence authority, Runtime/model/source-lock, credential, `.env`,
  daemon, scheduler, CLI, or queued product scope changed;
- no live Runtime/model/canary ran during RED, GREEN, or review;
- all historical evidence and private-attempt hashes remain unchanged; and
- no user-owned dirty file is staged or committed.

## Review, commit, and live gates

1. Fresh Contract Reviewer must return `PASS` before RED.
2. Fresh Implementation Reviewer must inspect the complete Candidate, RED and
   GREEN evidence, source-bound semantics, scope, trust boundary, and
   historical hash preservation.
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
   Reviewer must audit invocation count, SQLite/Event lineage, modes, digests,
   sanitization, cleanup, historical preservation, and the stated verdict.

## Exit state

On a successful live result and fresh evidence-review `PASS`, this amendment
may advance only to:

```text
READY_FOR_FINAL_USER_SIGNOFF
```

It does not grant final user sign-off, Phase 1 completion, production
activation, autonomous execution, daemon activation, push, merge, or release.

On any failure or ambiguity:

```text
HUMAN_REQUIRED — NO RETRY
```

The authorization is consumed and the line stops.
