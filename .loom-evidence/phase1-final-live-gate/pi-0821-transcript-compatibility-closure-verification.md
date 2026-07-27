# Final Live Gate Pi 0.82.1 Transcript Compatibility Closure Verification

Status: IMPLEMENTATION REVIEW 3 PASS — READY FOR ATOMIC COMMIT

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
