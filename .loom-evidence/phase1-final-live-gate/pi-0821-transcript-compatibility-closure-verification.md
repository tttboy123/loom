# Final Live Gate Pi 0.82.1 Transcript Compatibility Closure Verification

Status: REOPEN 1 IMPLEMENTATION REVIEW PASS — READY FOR ATOMIC COMMIT

- Contract: `PHASE1-FINAL-LIVE-PI-0821-TRANSCRIPT-CLOSURE-1`
- Baseline: `c7cabee40682f6e670d67b2553d85d6f8544b5c8`
- Date: `2026-07-27`
- Live canary invocations consumed by this Candidate: `0`

## Implemented boundary

The single Candidate now accepts the locked Pi `0.82.1` forward-partial
snapshot timing while retaining ordered `text_delta.delta` values as the only
assistant-output authority.

For each accepted assistant update the adapter validates, before committing
the current snapshot or delta:

- exact lifecycle state, event shape, and content index;
- semantic equality of the top-level message and nested partial after
  excluding only text, or the reviewed non-terminal relation where
  top-level usage is component-wise behind nested partial usage;
- exact assistant schema and progressive identity/usage rules;
- valid UTF-8, the configured output bound, and raw-Grant exclusion;
- byte-exact monotonic `H` prefix and candidate `D` prefix relations; and
- bounded Frame construction plus FrameSink authorization.

Top-level and nested text may differ only when one is a byte prefix of the
other; the longer text is a bounded monotonic witness, never output authority.
The only accepted usage skew is top-level-lagging, component-wise
non-decreasing nested usage during `text_start` or `text_delta`. Reverse,
crossed, disappearing optional reasoning, `cacheWrite1h`, malformed,
extra-key, and terminal usage skew are rejected.

`text_end` requires `content == M == P == W == H == D` and exact non-text
semantic equality. The existing terminal message, turn, agent, settled,
clean-exit, Frame, Evidence, and AdapterResult rules remain exact. A snapshot
or usage value creates no output bytes, Frame, digest, or Evidence.

The only new exported observation surface is the frozen by-value
`PiRPCTranscriptAudit` and send-only best-effort channel. Publication occurs
once, after complete successful result construction, through one
non-blocking send protected by a local deferred recovery. Nil, unready, full,
and closed channels do not affect execution.

## Locked Pi component

The component executed the installed Pi CLI and verified these exact bindings:

| Input | SHA-256 |
|---|---|
| Pi CLI | `af302f231437eaf6f37691bce4b34234fcb626bcb5eb3910d4fc3f6519bf78ca` |
| `event-stream.js` | `44a2498660ca61efa952ad6a3f10cc0491883411bd2b4572c9a392ec4e9553ec` |
| `openai-completions.js` | `0d50250fe2931e66e2078279a397814202e1ecddee58faf4b8bc04c278da177a` |
| Runtime instance | `runtime.pi.earendil-works.0.82.1` |

The in-process loopback server accepted exactly one loopback-only,
authenticated, streaming chat-completions request and emitted two non-empty
content records, a separate stop record, bounded usage, and `[DONE]`. Every
record was flushed immediately with no sleep or cross-record pacing.

The mandatory race repetition exposed and then verified the locked Pi mode
where nested usage can advance while top-level usage remains at the earlier
snapshot. The final implementation accepted only that reviewed relation and
still observed a non-empty forward text snapshot, two accepted delta events,
exact delta reconstruction, and one sanitized audit. It did not contact DNS,
the Internet, llama.cpp, a Provider, or a model file.

The real Pi path closed:

```text
Pi 0.82.1
→ loopback SSE
→ full RPC lifecycle
→ Pi RPC Bridge
→ Supervisor / BoundRunStream / AgentGrant
→ Ack / Event / Evidence / Result
→ immutable Evidence receipt and capture
→ Journal terminal facts
→ rebuilt succeeded Team Projection
```

The separate real-Pi observer-failure path rejected the first authorized
Event, returned no successful AdapterResult or audit, captured only Ack plus
the rejected Event in one failed Evidence lineage, revoked the Grant, released
capacity, projected blocked Team terminal state, and performed no retry.

The private `0600` Evidence artifact was decoded rather than substring-tested.
On success, every stored canonical line matched the exact ordered authorized
Ack/Event/Event/Evidence/Result Frames byte-for-byte. On observer failure it
contained exactly Ack plus the rejected-but-authorized Event, with no later
Evidence/Result. Both paths rejected prompt, raw Grant, credential marker, and
private attempt path material. Journal remained free of tentative output.

Before Pi process start, the component compared each resolved Pi/source file's
UID with the current user's UID in addition to exact hash, regular-file, and
non-group/world-writable checks. The mismatch control and Windows compile-only
gate both passed.

The mandatory single-run output contained exactly one proof line:

```text
component_proof locked_pi=true forward_partial=true sse_requests=1 team_terminal=succeeded evidence=1
```

## Verification commands

All commands ran from the authoritative checkout.

### Focused and impact

```text
go test ./internal/runtime/piadapter -run '^TestPiRPCTranscriptClosureUsageProjectionSkew$' -count=1
PASS: internal/runtime/piadapter, 2.924s

go test ./internal/runtime/piadapter -run '^TestPiRPC(TranscriptClosure|TranscriptClosurePrefixSkew|TranscriptClosureUsageProjectionSkew|TranscriptClosureRejections|TranscriptClosureFrameAuthority|TranscriptLifecycleClosure)$' -count=1
PASS: internal/runtime/piadapter, 17.488s

go test ./internal/app -run '^TestFinalLive(Pi0821TranscriptClosureCanaryIsolation|BoundFilesRequireCurrentUserOwnership)$' -count=1
PASS: internal/app, 0.327s

go test ./internal/runtime/piadapter ./internal/supervisor ./internal/authorization ./internal/evidence ./internal/app -count=1
PASS: all five packages; piadapter 49.486s, supervisor 2.855s,
authorization 1.897s, evidence 0.851s, app 10.776s
```

The full `internal/runtime/piadapter` package also passed, retaining the
pre-existing lifecycle, adversarial, identity, reason-code, cancellation,
cleanup, overflow, and fuzz-seed matrices.

### Locked Pi component

The following environment used the reviewed absolute Pi executable, reviewed
Runtime search path, and exact installed Runtime instance ID:

```text
LOOM_PI_0821_COMPONENT=1 \
LOOM_PI_0821_EXECUTABLE=<reviewed absolute Pi 0.82.1 executable> \
LOOM_PI_0821_RUNTIME_SEARCH_PATH=<reviewed absolute Runtime search path> \
LOOM_PI_0821_RUNTIME_INSTANCE_ID=runtime.pi.earendil-works.0.82.1 \
go test -v ./internal/app -run '^TestPi0821DeterministicSSETeamExecution(Closure|FailureClosure)$' -count=1
PASS: success sentinel once plus observer-failure closure, 3.484s

LOOM_PI_0821_COMPONENT=1 \
LOOM_PI_0821_EXECUTABLE=<reviewed absolute Pi 0.82.1 executable> \
LOOM_PI_0821_RUNTIME_SEARCH_PATH=<reviewed absolute Runtime search path> \
LOOM_PI_0821_RUNTIME_INSTANCE_ID=runtime.pi.earendil-works.0.82.1 \
go test -v -race ./internal/app -run '^TestPi0821DeterministicSSETeamExecution(Closure|FailureClosure)$' -count=10
PASS: 10/10 success plus 10/10 observer-failure closures, 31.842s;
exact success sentinel 10 times
```

### Race, repository, and portability

```text
go test -race ./internal/runtime/piadapter -run '^TestPiRPC(TranscriptClosure|TranscriptClosurePrefixSkew|TranscriptClosureUsageProjectionSkew|TranscriptClosureRejections|TranscriptClosureFrameAuthority|TranscriptLifecycleClosure)$' -count=30
PASS: internal/runtime/piadapter, 274.972s

go test -race ./internal/app -run '^TestFinalLivePi0821TranscriptClosureCanaryIsolation$' -count=30
PASS: internal/app, 1.696s

go test ./... -count=1
PASS: every package, exit 0

go test -race ./... -count=1
PASS: every package, exit 0

go vet ./...
PASS

go mod verify
PASS: all modules verified

GOOS=windows GOARCH=amd64 go test -exec=true ./internal/runtime/piadapter ./internal/app
PASS: both packages

gofmt -d internal/runtime/piadapter/rpc_bridge_adapter.go internal/runtime/piadapter/rpc_bridge_adapter_test.go internal/app/final_live_gate_pi0821_component_test.go internal/app/final_live_gate_live_test.go
PASS: empty output

git diff --check
PASS: empty output
```

## Candidate file digests

| Path | SHA-256 |
|---|---|
| `internal/runtime/piadapter/rpc_bridge_adapter.go` | `c5f5fd70787105c8ca80b355d55f980085fdb5a350c74cc692d0a3502e7a7343` |
| `internal/runtime/piadapter/rpc_bridge_adapter_test.go` | `3393202923c7f6b484d3a39901c66c594d93c69ecfa5af94045298fbf8e884ee` |
| `internal/app/final_live_gate_pi0821_component_test.go` | `aa7ea102b84c87f427f55697f85cbc919665a328184750d6e956c9793907f211` |
| `internal/app/final_live_gate_live_test.go` | `2898f6150b0ff9ebcba74267839875d79b50fff7a4f72afd777dfc4a14fc9c16` |

## Scope and quarantine audit

`git diff --cached --name-only` was empty before Implementation Review.
No live manifest or attempt root has been created. No live model server,
Provider, or controlled canary has been started by this Candidate.

All freeze-time quarantined hashes were rechecked and matched exactly:

- modified user/governance state:
  `contract.md=9c5e7176...`,
  `contract-review.md=7cbbeb13...`,
  `source-lock.json=e542aa31...`,
  `AGENTS.md=9196bf1a...`,
  `PROGRESS.md=49b78621...`;
- untracked historical evidence:
  `source-provenance-amendment.md=a8f9a265...`,
  `live-canary.md=2dbd1494...`,
  `replacement-live-canary.md=05434337...`,
  `additional-live-canary.md=2559ef91...`,
  `progressive-identity-live-canary.md=5a154572...`,
  `rejection-diagnostic-live-canary.md=82adaf33...`;
- historical manifests:
  `resolved-live-manifest.json=b1fb0291...`,
  `resolved-live-manifest-additional-canary.json=222d6004...`,
  `resolved-live-manifest-progressive-identity-canary.json=da04a07f...`,
  `resolved-live-manifest-rejection-diagnostic-canary.json=f0d7e25d...`.

All non-owned status entries remain in their freeze-time modified/untracked
state. Candidate staging and any later atomic commit must use only the exact
frozen allowlist.

## Gate

Implementation Review 2 findings were repaired inside the same unique
Candidate after fresh contract-only `PASS`. Fresh Implementation Review 3
returned `PASS` with no findings. The exact Candidate allowlist may now be
committed atomically. Live execution remains prohibited until the post-commit
pre-live revalidation and independent manifest are complete.

VERDICT: PASS

## Reopen 1 Repair 2 verification

Date: `2026-07-28`

Baseline:

```text
67b251cae0e3a2086163998b309b4ebb5beadca5
```

### Test-first evidence

The focused harness test was added before the helper existed. Its first run
failed to compile at all four construction assertions only with:

```text
undefined: newFinalLiveAuthoritativeClock
```

The frozen RED test hash was:

```text
e88ba7e782b9c4658640741f46733465a7be8f74b6b5229fe0f871cea369c5e1
```

No Pi, llama.cpp, model, network, manifest, attempt, or live canary ran during
that RED.

### Minimal implementation

Only `internal/app/final_live_gate_live_test.go` changed. It now:

- captures one explicit UTC authoritative snapshot;
- validates a nonzero UTC, non-drifting construction source;
- returns a closure that retains only the exact snapshot;
- binds Runtime seed, Work Authority, Grant Authority, Pi execution, dispatch
  time, and `TeamExecutionRequest.AuthoritativeTime` to that one closure;
- retains real context deadlines, startup timeouts, cancellation grace, and
  process cleanup timers; and
- selects the new independent Repair-2 manifest and attempt prefix while
  rejecting the consumed Reopen-1 identity.

No product, parser, lifecycle, authority, Journal, Evidence, Projection,
Supervisor, Bridge, retry, compaction, permission, or model behavior changed.

### Fresh verification results

```text
go test ./internal/app \
  -run '^(TestFinalLiveAuthoritativeClockBinding|TestFinalLivePi0821TranscriptClosureCanaryIsolation)$' \
  -count=1
PASS

LOOM_PI_0821_COMPONENT=1 go test -v ./internal/app \
  -run '^TestPi0821DeterministicSSETeamExecution(Closure|FailureClosure)$' \
  -count=1
PASS: locked Pi success and observer-failure paths

go test ./internal/app ./internal/work ./internal/authorization \
  ./internal/evidence ./internal/projection -count=1
PASS

go test -race ./internal/app \
  -run '^(TestFinalLiveAuthoritativeClockBinding|TestFinalLivePi0821TranscriptClosureCanaryIsolation)$' \
  -count=30
PASS

LOOM_PI_0821_COMPONENT=1 go test -v -race ./internal/app \
  -run '^TestPi0821DeterministicSSETeamExecution(Closure|FailureClosure)$' \
  -count=10
PASS

go test ./... -count=1
PASS

go test -race ./... -count=1
PASS

go vet ./...
PASS

go mod verify
PASS: all modules verified

gofmt -d internal/app/final_live_gate_live_test.go
PASS: empty output

git diff --check
PASS: empty output

GOOS=windows GOARCH=amd64 go test -exec=true ./internal/app
PASS
```

The locked-Pi component used only the reviewed installed Pi executable and
deterministic loopback SSE. It did not start llama.cpp or load the model.

### Scope, hashes, and isolation

Current controlled harness hash:

```text
internal/app/final_live_gate_live_test.go
7da6082d500a7d93182b38917d2d8ee0d98244a59419568173673d6af8fb45b2
```

Read-only authority files remained unchanged:

```text
internal/work/verification_authority.go
efce682c5ef4f6c1068c625441386b0be5abad6120bf9290eaab757949b561be

internal/app/team_execution.go
47f4810201d46f013f093562f1c792190dea419489721377ca14900952b13354
```

All five pre-existing quarantine hashes still match the contract:

```text
contract.md
9c5e71767f9983bc53089a8c22a03fcf430bd784d5e2fc602b618a9fca9e1dff

contract-review.md
7cbbeb13b4699fee65e0454329306f967bcc16a32e87aca5fe790c43454b22a1

source-lock.json
e542aa319219f486cb45538df53ff49fb03fb180b748b9661bfb2497d58658f5

AGENTS.md
9196bf1a9cda6448688b073a2217ff5e806b1bcb288506d3be7ed6e204c4691f

PROGRESS.md
49b78621fa83b26967ac3ceda305bf14ffecba779bc5c3446595557bbdbf355c
```

Staging is empty. The Repair-2 manifest and attempt prefix are absent. Port
`18427` has no listener, and no Pi/llama-server/loomd live process is
running. Repair-2 live-canary invocations consumed: `0`.

Fresh independent Implementation Review is required before staging, commit,
manifest creation, llama/model execution, or the one authorized Repair-2
canary.

VERDICT: PASS

## Reopen 1 context-alignment verification

Date: `2026-07-28`

Fresh Contract Review 3 and the genuine Reopen-1 RED preceded product
implementation. The minimal product diff is limited to:

- one named context constant: `32768`;
- one named output constant: `256`;
- Pi `modelsJSON` consuming both constants; and
- llama-server `--ctx-size` / `--n-predict` consuming the same constants.

No parser, transcript predicate, response identity, retry/compaction, Bridge,
Supervisor, Grant, Frame, Evidence, Journal, Projection, StateWriter,
terminal aggregation, Provider, model, or permission behavior changed.

The component and pre-live paths now bind the installed Pi executable plus
all three causal source files through exact digest, regular-file, safe-mode,
and current-user ownership checks. Verified source SHA-256 values:

```text
event-stream.js
44a2498660ca61efa952ad6a3f10cc0491883411bd2b4572c9a392ec4e9553ec

openai-completions.js
0d50250fe2931e66e2078279a397814202e1ecddee58faf4b8bc04c278da177a

simple-options.js
74dfde37adbd00a6af1fd707c1c5c876577793b078da9fbbd6d40bb75bfb4749
```

### Focused, component, and impact

```text
go test ./internal/runtime/piadapter \
  -run '^(TestPiRPC(ModelOutputBudget|TranscriptClosure|TranscriptClosureRejections|TranscriptLifecycleClosure)|TestPiLocalModelContextAlignment)$' \
  -count=1
PASS: internal/runtime/piadapter, 20.106s

LOOM_PI_0821_COMPONENT=1 \
LOOM_PI_0821_EXECUTABLE=<reviewed-installed-pi> \
LOOM_PI_0821_RUNTIME_SEARCH_PATH=<reviewed-node-search-path> \
LOOM_PI_0821_RUNTIME_INSTANCE_ID=runtime.pi.earendil-works.0.82.1 \
go test -v ./internal/app \
  -run '^TestPi0821DeterministicSSETeamExecution(Closure|FailureClosure)$' \
  -count=1
PASS: success and observer-failure closure, 3.502s
exact success sentinel: 1

go test ./internal/runtime/piadapter ./internal/supervisor \
  ./internal/authorization ./internal/evidence ./internal/app -count=1
PASS: all five packages; piadapter 48.663s, supervisor 2.854s,
authorization 0.436s, evidence 1.065s, app 13.597s
```

The locked-Pi success component observed one valid loopback request at the
exact bounded output budget `256` and closed the full
Supervisor/Grant/Frame/Evidence/Journal/Projection path. The separate
observer-failure path remained failed, revoked, capacity-released, and
non-retried.

### Race, repository, and portability

```text
go test -race ./internal/runtime/piadapter \
  -run '^(TestPiRPC(ModelOutputBudget|TranscriptClosure|TranscriptClosureRejections|TranscriptLifecycleClosure)|TestPiLocalModelContextAlignment)$' \
  -count=30
PASS: internal/runtime/piadapter, 144.521s

LOOM_PI_0821_COMPONENT=1 \
LOOM_PI_0821_EXECUTABLE=<reviewed-installed-pi> \
LOOM_PI_0821_RUNTIME_SEARCH_PATH=<reviewed-node-search-path> \
LOOM_PI_0821_RUNTIME_INSTANCE_ID=runtime.pi.earendil-works.0.82.1 \
go test -v -race ./internal/app \
  -run '^TestPi0821DeterministicSSETeamExecution(Closure|FailureClosure)$' \
  -count=10
PASS: 10/10 success and 10/10 observer-failure closures, 30.174s
exact success sentinel: 10

go test -race ./internal/app \
  -run '^TestFinalLivePi0821TranscriptClosureCanaryIsolation$' \
  -count=30
PASS: internal/app, 2.364s

go test ./... -count=1
PASS: every package

go test -race ./... -count=1
PASS: every package

go vet ./...
PASS

go mod verify
PASS: all modules verified

gofmt -d \
  internal/runtime/piadapter/rpc_bridge_adapter.go \
  internal/runtime/piadapter/rpc_bridge_adapter_test.go \
  internal/runtime/piadapter/local_model_server.go \
  internal/runtime/piadapter/local_model_server_test.go \
  internal/app/final_live_gate_pi0821_component_test.go \
  internal/app/final_live_gate_live_test.go
PASS: empty output

git diff --check
PASS: empty output

GOOS=windows GOARCH=amd64 go test -exec=true \
  ./internal/runtime/piadapter ./internal/app
PASS: both packages
```

### Candidate digests

| Path | SHA-256 |
|---|---|
| `internal/runtime/piadapter/rpc_bridge_adapter.go` | `da124a30998724845ec36280b7aaeace3b490e39a6d5a7fa3e6454b2bc59f11f` |
| `internal/runtime/piadapter/local_model_server.go` | `7a76c34305a76cfd7a39677a7a22f59727eb87a648ed29df8a7ed74e8d810b27` |
| `internal/runtime/piadapter/rpc_bridge_adapter_test.go` | `cdf9154c1573788288cdf36168ab0d357e682d926771fb78e546b502764605f5` |
| `internal/runtime/piadapter/local_model_server_test.go` | `b82a2286adb119a37d6471e486481bdb0348a3cff799f1241dad81f52392824a` |
| `internal/app/final_live_gate_pi0821_component_test.go` | `271d0e9d441481b69ea2f7de786dac218636200b65c383e44495dd452fc9ec5b` |
| `internal/app/final_live_gate_live_test.go` | `f4bb348da3068fe61e2dcbbead94f90c6428c74ae28cd83709098ffd661178f5` |
| unique contract | `6f6d42b24868e8bceb69ea37bba9d4576f8ed7c5b42d21455ef340eb3099c1a3` |
| contract-review ledger | `d14bdb86657f4d5dc56a94e6fb4460015e12b17e44c19c11275c0a7ea51c0cb5` |
| RED evidence | `4b50ffaaa1ad16e8a3aa8a9136e83674228a79f5a2f78b85f2d42788e2e8bc31` |
| implementation-review ledger | `790b1792b434511ed423cd448e8c02eca4c7d87917fb8810e12ebd121ffe4c9a` |

### Scope and quarantine

`git diff --cached --name-status` was empty. The exact Reopen-1 manifest and
attempt prefix were absent. No llama/model/live process or controlled live
invocation ran during implementation or verification.

Every original dirty-worktree and historical-evidence hash matched its frozen
value. The historical closure manifest/evidence remained:

```text
resolved-live-manifest-pi-0821-transcript-closure-canary.json
4dfb081363b910b2fdf041c2636ba2be7b9876ab92d103eb30a76362087e6e35

pi-0821-transcript-closure-live-canary.md
b09c365dcd761715425a98614da76aef3d707310c7bcbe6e9d2ca3dd4b91a61a
```

The retained historical attempt still matched all five frozen hashes:
SQLite `204f4f6c...`, bounded source `bfc5aca5...`, artifact `5fd50fba...`,
capture `a4c26a54...`, and receipt `aab9365d...`.

Fresh Implementation Review is required before staging, commit, manifest
creation, llama/model execution, or the one authorized Reopen-1 canary.

VERDICT: PASS
