# Hermetic Verification

Date: 2026-07-27

Verdict: `PASS — IMPLEMENTATION REVIEW PENDING`

## Mandatory matrix

All commands completed with exit status zero:

```text
go test ./internal/runtime ./internal/runtime/piadapter -count=1
go test ./internal/supervisor ./internal/app ./internal/api \
  ./internal/authorization ./internal/evidence ./internal/journal \
  ./internal/projection ./internal/teams ./internal/work -count=1
go test -race -count=10 ./internal/runtime ./internal/runtime/piadapter
go test ./...
go test -race ./...
go vet ./...
gofmt -d <all frozen Go files>
git diff --check
GOOS=windows GOARCH=amd64 go test -exec=true \
  ./internal/runtime ./internal/runtime/piadapter
go mod verify
```

The ten-round race gate completed:

```text
ok loom-pi-rebuild/internal/runtime
ok loom-pi-rebuild/internal/runtime/piadapter 235.848s
```

`go test ./...` and `go test -race ./...` passed every package. `go vet ./...`
emitted no findings. `gofmt -d` and `git diff --check` emitted no differences
or whitespace findings. Cross-compilation passed both owned packages and all Go
modules verified.

The ordinary live-test proof was:

```text
=== RUN   TestFinalLiveGatePiRPCOfflineModel
    final_live_gate_live_test.go:34: opt-in final live gate is disabled
--- SKIP: TestFinalLiveGatePiRPCOfflineModel (0.00s)
PASS
```

This is a skip proof, not live evidence.

## Boundary and adversarial verification

- behavioral RED rejects the exact valid Pi 0.82.1 diagnostic on the baseline;
- valid metadata GREEN returns no model IDs and never returns diagnostic paths;
- relative, swapped, duplicate, dot-segment, different-parent, control-byte,
  and extra-line metadata forms fail with `ErrInvalidPiMetadataOutput`;
- the RPC fixture verifies the exact owned Pi arguments and required clean
  environment while proving ambient variables and raw Grant are absent;
- correlated response, `agent_start`, balanced turn/message lifecycle, actual
  Pi 0.82.1 `partial`/`message` assistant events, `agent_end(willRetry=false)`,
  and `agent_settled` produce only Ack, text Events, digest Evidence, and Result;
- unknown/extra/duplicate keys, CR, U+2028, hidden reasoning, tool calls,
  non-stop completion, retry, mismatched text, uncorrelated response, and raw
  Grant output fail closed with no partial `AdapterResult`;
- cancellation sends one correlated abort, requires its successful response,
  and cleans the process group within the configured bound;
- 100 UTF-8 chunk-boundary permutations preserve bytes and keep Event payloads
  within 2048 bytes;
- a pre-repair decoder fuzz run completed 172,759 executions with no panic,
  followed by the current Pi-event-shape run completing 106,842 executions
  with no failure;
- local model tests verify exact llama.cpp arguments, clean environment,
  loopback health, immutable file binding, expected model digest, startup
  timeout, idempotent close, and listener removal;
- process-group containment delegates to the already accepted and race-tested
  Unix execution cleanup primitive; Windows uses the owned unsupported stub;
- production `internal/runtime/piadapter` imports no Journal, StateWriter,
  Projection, Work, Team, Grant, Evidence, database, or SQLite authority;
- only the opt-in application test composes existing authorities, and all
  terminal/Grant facts remain produced by the accepted Supervisor/coordinator;
- secret-pattern and authority-import scans found no credential material or
  new writer path; the only `loom_grant_v1.` literal is a negative Journal
  assertion in the opt-in test.

## Contract-to-test traceability

1. Installed Runtime discovery: metadata compatibility and the gated live probe.
2. Loopback model binding: local server health, digest, argument, and cleanup tests.
3. Runtime/Profile/Adapter/Run/Grant binding: existing Supervisor validation plus
   the gated one-node coordinator harness.
4. Authorized Ack: RPC success test and `FrameSink` sequence assertions.
5. Tentative output before terminal: RPC frame order and gated observer.
6. Digest-only Evidence plus Result: exact frame type/payload assertions.
7. Terminal/Grant lineage: gated coordinator and one-count Journal assertions.
8. No workspace/credential/path/token authority leak: clean environment,
   negative scans, source immutability, and Journal assertions.
9. Cancel/timeout cleanup: hermetic RPC and local-model lifecycle tests.
10. Private modes and worktree scope: 0700/0600 checks, exact owned-file audit,
    and excluded user dirt remains unstaged.

## External state

- reviewed Pi 0.82.1 remains installed user-locally;
- the selected private root contains only prior preflight material;
- no llama.cpp release asset or Qwen GGUF is present;
- no local model listener or Pi RPC process is resident;
- no live canary or model request has executed.

## Bounded Repair 1 reverification

Implementation Review 1 returned `FAIL` before external materialization. The
five findings and their bounded dispositions are preserved in
`implementation-review.md`.

The mandatory matrix was rerun after Repair 1:

```text
go test ./internal/runtime ./internal/runtime/piadapter -count=1
go test ./internal/supervisor ./internal/app ./internal/api \
  ./internal/authorization ./internal/evidence ./internal/journal \
  ./internal/projection ./internal/teams ./internal/work -count=1
go test ./...
go test -race -count=10 ./internal/runtime ./internal/runtime/piadapter
go test -race ./...
go vet ./...
gofmt -d <all frozen Go files>
git diff --check
GOOS=windows GOARCH=amd64 go test -exec=true \
  ./internal/runtime ./internal/runtime/piadapter
go mod verify
```

All completed with exit status zero. The ten-round race result was:

```text
ok loom-pi-rebuild/internal/runtime 1.614s
ok loom-pi-rebuild/internal/runtime/piadapter 255.039s
```

The fresh full-race run passed every package, including
`internal/runtime/piadapter` in 28.357 seconds. Vet, formatting, whitespace,
Windows compilation, module verification, and source-lock JSON checks emitted
no findings.

Additional repaired-boundary checks passed:

```text
go test ./internal/runtime/piadapter -run 'PiRPCBridge' -count=10
go test ./internal/runtime/piadapter -run 'PiLocalModel' -count=3
go test ./internal/runtime/piadapter -run '^$' \
  -fuzz FuzzPiRPCObjectNoPanic -fuzztime=5s
```

The fresh fuzz run completed 87,511 executions with no failure. A first
parallel self-contention attempt caused the successful fake-health fixture to
exceed its former five-second test-only startup budget while the concurrent
full suite passed the same fixture. Ten isolated repetitions then passed. The
test-only success budget was raised to ten seconds, still within the frozen
server API bound, and every mandatory command above was rerun sequentially.

The explicit normal-test live proof remains:

```text
=== RUN   TestFinalLiveGatePiRPCOfflineModel
    final_live_gate_live_test.go:56: opt-in final live gate is disabled
--- SKIP: TestFinalLiveGatePiRPCOfflineModel (0.00s)
PASS
```

No llama.cpp archive, GGUF, local Provider request, or live canary ran.

VERDICT: PASS

## Product Repair 2 reverification

Technical Correction 2 received Contract Review 8 `PASS`, bounded Product
Repair 2 was implemented, and every mandatory command was rerun serially:

```text
go test ./internal/runtime ./internal/runtime/piadapter -count=1
go test ./internal/supervisor ./internal/app ./internal/api \
  ./internal/authorization ./internal/evidence ./internal/journal \
  ./internal/projection ./internal/teams ./internal/work -count=1
go test -race -count=10 ./internal/runtime ./internal/runtime/piadapter
go test ./...
go test -race ./...
go vet ./...
gofmt -d <all frozen Go files>
git diff --check
GOOS=windows GOARCH=amd64 go test -exec=true \
  ./internal/runtime ./internal/runtime/piadapter
go mod verify
```

All commands exited zero. The ten-round race gate reported:

```text
ok loom-pi-rebuild/internal/runtime 1.607s
ok loom-pi-rebuild/internal/runtime/piadapter 272.387s
```

The full race run passed every package, including
`internal/runtime/piadapter` in 30.391 seconds. Vet, formatting, whitespace,
Windows compilation, and module verification emitted no findings.

Additional Product Repair 2 evidence:

- focused metadata/RPC/local-model compatibility: PASS;
- RPC repetition count 10: PASS in 61.038 seconds;
- local-model repetition count 3: PASS in 4.631 seconds;
- decoder fuzz: 125,244 executions, no failure;
- exact Pi CLI, Agent-loop, RPC-mode, and RPC-types digests match
  `source-lock.json`;
- Pi version remains exactly 0.82.1;
- parent/intermediate symlink, lexical/realpath escape, public ancestor,
  descendant mode, foreign owner, file replacement, digest drift, executable
  mode, and model mode fixtures fail closed;
- the shared binding inspector creates no file or process and its returned
  digests are the only llama.cpp/model digests used by the resolved manifest;
  and
- the ordinary live test remains an explicit skip because
  `LOOM_FINAL_LIVE_GATE` was not set.

The source-lock status is
`product_repair_2_verified_implementation_review_3_pending_not_materialized`.
No llama.cpp asset, GGUF, local Provider request, or live canary ran.

VERDICT: PASS — IMPLEMENTATION REVIEW 3 PENDING

## Product Repair 3 reverification

The final bounded readiness repair was followed by a fresh serial mandatory
matrix. All commands exited zero:

```text
go test ./internal/runtime ./internal/runtime/piadapter \
  -run 'Pi0821|PiRPCBridge|PiLocalModel' -count=1
go test ./internal/runtime ./internal/runtime/piadapter -count=1
go test ./internal/supervisor ./internal/app ./internal/api \
  ./internal/authorization ./internal/evidence ./internal/journal \
  ./internal/projection ./internal/teams ./internal/work -count=1
go test -race -count=10 ./internal/runtime ./internal/runtime/piadapter
go test ./...
go test -race ./...
go vet ./...
gofmt -d <all frozen Go files>
git diff --check
GOOS=windows GOARCH=amd64 go test -exec=true \
  ./internal/runtime ./internal/runtime/piadapter
go mod verify
```

The final ten-round race gate reported:

```text
ok loom-pi-rebuild/internal/runtime 1.587s
ok loom-pi-rebuild/internal/runtime/piadapter 274.509s
```

The final full race run passed every package, including
`internal/runtime/piadapter` in 33.537 seconds. Three additional local-model
repetitions passed in 5.220 seconds. The health-then-exit and post-free-check
port-theft fixtures pass only by rejecting startup. The ordinary live test
remains skipped because its opt-in environment binding was absent.

Implementation Review 4 subsequently reproduced one focused flake: under load,
the one-shot health-then-exit fixture's one-second startup context won before
the child wait channel, returning `ErrPiLocalModelHealth` rather than the
fixture's required `ErrPiLocalModelProcess`. Startup still failed closed, but
the focused mandatory command was not stable and the exact classification
evidence is insufficient.

The source-lock status is
`implementation_review_4_failed_repair_budget_exhausted_human_required_not_materialized`.
No llama.cpp asset, GGUF, local Provider request, or live canary ran.

VERDICT: FAIL — HUMAN_REQUIRED

## Authorized Product Repair 4 reverification

After explicit user authorization, the exact narrow Repair 4 passed:

```text
go test ./internal/runtime/piadapter \
  -run 'TestPiLocalModelHealthContextClassification|TestPiLocalModelServerFailsClosed/health_then_early_exit' \
  -count=20
```

Result: PASS in 11.646 seconds.

The full mandatory matrix was then rerun serially and every command exited
zero. Final race evidence:

```text
ok loom-pi-rebuild/internal/runtime 1.626s
ok loom-pi-rebuild/internal/runtime/piadapter 279.950s
```

`go test ./...` passed every package; the full race run passed every package,
including `internal/runtime/piadapter` in 29.178 seconds. Vet, formatting,
whitespace, Windows compilation, module verification, three additional
local-model repetitions, and the ordinary live-skip proof all passed.

The source-lock status after fresh independent Review 5 is
`implementation_review_5_pass_compatibility_commit_pending_not_materialized`.
No llama.cpp asset, GGUF, local Provider request, or live canary ran.

VERDICT: PASS
