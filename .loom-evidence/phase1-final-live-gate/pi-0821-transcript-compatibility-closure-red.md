# Final Live Gate Pi 0.82.1 Transcript Compatibility Closure — RED

Date: 2026-07-27
Baseline: `c7cabee40682f6e670d67b2553d85d6f8544b5c8`
Contract: `PHASE1-FINAL-LIVE-PI-0821-TRANSCRIPT-CLOSURE-1`
Production edits before RED: none
Live model canary invocations: zero

## Final RED test hashes

- `internal/runtime/piadapter/rpc_bridge_adapter_test.go`
  SHA-256:
  `22ca776ce1aea7d09971e65edddaec276eada43265cb79be33f1334b37c1fe68`
- `internal/app/final_live_gate_pi0821_component_test.go`
  SHA-256:
  `c2cf065c4493c85c8e3bc192e54077a51646ac7ae8c12315a1920d0d04c6b1a5`
- `internal/app/final_live_gate_live_test.go`
  SHA-256:
  `f770033cf6d3a993fdcbbdffaea2091ddf7715eaaf14cde33d61cbc325c82d2c`

`git diff -- internal/runtime/piadapter/rpc_bridge_adapter.go` was empty.
`git diff --check` over all three test files passed.

## Hermetic behavioral RED

Command:

```text
go test ./internal/runtime/piadapter \
  -run '^TestPiRPC(TranscriptClosure|TranscriptClosureRejections|TranscriptClosureFrameAuthority|TranscriptLifecycleClosure)$' \
  -count=1
```

Before the audit symbols were referenced, the command failed only for the
single- and multi-chunk forward-partial success cases. Both failures retained
the exact closed diagnostic:

```text
phase=assistant_update event=text_start reason=text_content_progression
```

The immutable-snapshot case and the new rejection/lifecycle cases behaved as
expected. No raw transcript, prompt, output, ID, Grant, credential, or private
path was printed.

## Real locked-Pi component behavioral RED

The opt-in component used:

- the installed Pi CLI with exact SHA-256
  `af302f231437eaf6f37691bce4b34234fcb626bcb5eb3910d4fc3f6519bf78ca`;
- locked `event-stream.js` SHA-256
  `44a2498660ca61efa952ad6a3f10cc0491883411bd2b4572c9a392ec4e9553ec`;
- locked `openai-completions.js` SHA-256
  `0d50250fe2931e66e2078279a397814202e1ecddee58faf4b8bc04c278da177a`;
- exact Runtime instance `runtime.pi.earendil-works.0.82.1`;
- a private temporary Team/SQLite/Evidence root;
- one loopback-only OpenAI-compatible SSE request;
- a role-only identity-free prelude followed by two distinct content chunks;
- one stable response ID on every response-bearing content/finish/usage
  chunk, a separate `stop` chunk, usage chunk, and `[DONE]`; and
- the accepted Team/Supervisor/Grant/Frame/Evidence/Journal/Projection path.

The first component construction attempt exposed only test-fixture defects:
the macOS temporary path traversed `/var` symlink resolution, request content
was decoded too narrowly, and a role prelude carrying response identity could
forward-bind identity before assistant `message_start`. Those defects were
repaired only in the test. A temporary sanitized diagnostic reported only
record type, role, content-block count, error presence/class, response-ID
presence, nested Event kind, and request validity; it recorded no raw RPC,
prompt, output, ID value, usage, or path and was removed before final RED.

After fixture correction, the locked real Pi produced the exact complete
pre-rejection sequence through assistant `message_start`, made exactly one
valid loopback request, and the unchanged product failed only at:

```text
phase=assistant_update event=text_start reason=text_content_progression
```

The bounded failure state was:

```text
response=true agent=true turns=1 message=true done=false settled=false
```

No output Frame was accepted and no model, llama.cpp, Provider, DNS, Internet,
retry, compaction, or live canary was used.

## Final compile RED for the frozen audit seam

After the behavioral REDs, the final tests added the exact frozen audit channel
and assertions. The hermetic command then failed to compile only on:

```text
undefined: PiRPCTranscriptAudit
config.TranscriptAudit undefined
```

The opt-in component command likewise failed only on:

```text
undefined: piadapter.PiRPCTranscriptAudit
unknown field TranscriptAudit in PiRPCBridgeAdapterConfig
```

The final RED also encodes nil, unbuffered, full, closed, and concurrent
distinct-channel isolation, exact audit counts/booleans, and post-result
publication.

## Live-harness RED

Command:

```text
go test ./internal/app \
  -run '^TestFinalLivePi0821TranscriptClosureCanaryIsolation$' \
  -count=1
```

It failed only because the unchanged harness still named:

```text
resolved-live-manifest-rejection-diagnostic-canary.json
controlled-canary-rejection-diagnostic-
```

instead of the independent closure manifest and attempt prefix. It did not
execute any Runtime or live canary.

## In-place Repair 3 deterministic RED

Repair-3 Contract Review passed before these test changes. Production remained
unchanged at SHA-256:

```text
internal/runtime/piadapter/rpc_bridge_adapter.go
85cf6ef8e835ff05a02b2e15aed8b0927ece34c9bd4f819318e01877e6662379
```

The repaired test-first checkpoint hashes are:

| Path | SHA-256 |
|---|---|
| `internal/runtime/piadapter/rpc_bridge_adapter_test.go` | `27c0b672b32366b25c9ca1f1125d4d82792d83634a0fd86468399dd8e6d98a19` |
| `internal/app/final_live_gate_pi0821_component_test.go` | `ca58e4a7766ce88dc1ec38f4ae99d3e87ddedbf55b0e837414715edd06563082` |
| `internal/app/final_live_gate_live_test.go` | `c549445e8220dd6e2eee3324ac8512150502c8b5324a1f5c21abac20242ba624` |

Mandatory command:

```text
go test ./internal/runtime/piadapter \
  -run '^TestPiRPCTranscriptClosurePrefixSkew$' \
  -count=1
```

Result: `FAIL`, exit `1`, only on the four new compatible-skew cases:

```text
prefix-skew-nested-start:
phase=assistant_update event=text_start reason=message_partial_mismatch

prefix-skew-top-start:
phase=assistant_update event=text_start reason=message_partial_mismatch

prefix-skew-nested-delta:
phase=assistant_update event=text_delta reason=message_partial_mismatch

prefix-skew-top-delta:
phase=assistant_update event=text_delta reason=message_partial_mismatch
```

Every failure retained the bounded state summary:

```text
response=true agent=true turns=1 message=true done=false settled=false
```

The divergent/non-text controls compile and remain encoded as rejection
cases. The success and observer-failure real-Pi components compile under the
ordinary opt-out test command; they are not the sole RED oracle. No raw
transcript, output, prompt, Grant, response ID, model/provider value, usage,
private path, or credential was printed.

Live-canary invocations consumed by Repair 3: `0`.

Production Repair 1 is now authorized inside the same Closure Contract.

## Locked-Pi usage-skew repair deterministic RED

Fresh contract-only review passed before this test-first checkpoint. Temporary
component diagnostics were removed. The pre-repair Production Repair 1
Candidate remained at:

```text
internal/runtime/piadapter/rpc_bridge_adapter.go
8fd20d3cf11de81ea9e742b2032ed6bdb004a18abd26a6d75fd149522458e82d
```

The usage-skew test-first checkpoint hashes are:

| Path | SHA-256 |
|---|---|
| `internal/runtime/piadapter/rpc_bridge_adapter_test.go` | `3393202923c7f6b484d3a39901c66c594d93c69ecfa5af94045298fbf8e884ee` |
| `internal/app/final_live_gate_pi0821_component_test.go` | `ca58e4a7766ce88dc1ec38f4ae99d3e87ddedbf55b0e837414715edd06563082` |
| `internal/app/final_live_gate_live_test.go` | `c549445e8220dd6e2eee3324ac8512150502c8b5324a1f5c21abac20242ba624` |

Mandatory command:

```text
go test ./internal/runtime/piadapter \
  -run '^TestPiRPCTranscriptClosureUsageProjectionSkew$' \
  -count=1
```

Result: `FAIL`, exit `1`, only on the three new compatible top-level-lagging
cases:

```text
usage-skew-start:
phase=assistant_update event=text_start reason=message_partial_mismatch

usage-skew-delta:
phase=assistant_update event=text_delta reason=message_partial_mismatch

usage-skew-reasoning-appears:
phase=assistant_update event=text_delta reason=message_partial_mismatch
```

Every failure retained the bounded state summary:

```text
response=true agent=true turns=1 message=true done=false settled=false
```

Reverse, crossed, disappearing-optional, `cacheWrite1h`, malformed, extra-key,
and terminal-skew controls compiled and remained closed rejections. No
temporary diagnostic hook, raw transcript, output, prompt, Grant, response ID,
model/provider value, usage value, private path, or credential was printed or
persisted.

Live-canary invocations consumed by this repair: `0`.

Production Repair 1 may continue inside the same unique Closure Contract.

## Implementation Review 2 repair RED

Fresh contract-only review passed before this test-first checkpoint. Product
behavior remained unchanged. The repaired component test now decodes the
private immutable artifact and requires exact canonical authorized-Frame
equality on success plus Ack-and-rejected-Event-only closure on observer
failure. The owned live helper test requires an explicit current-user UID
predicate.

The test-first checkpoint hashes are:

| Path | SHA-256 |
|---|---|
| `internal/runtime/piadapter/rpc_bridge_adapter.go` | `c5f5fd70787105c8ca80b355d55f980085fdb5a350c74cc692d0a3502e7a7343` |
| `internal/runtime/piadapter/rpc_bridge_adapter_test.go` | `3393202923c7f6b484d3a39901c66c594d93c69ecfa5af94045298fbf8e884ee` |
| `internal/app/final_live_gate_pi0821_component_test.go` | `aa7ea102b84c87f427f55697f85cbc919665a328184750d6e956c9793907f211` |
| `internal/app/final_live_gate_live_test.go` | `6e54def8a2febc8a6a7f5aefeeb605743a5bd93b6b18b1c51fe2afbbd9ca9d4c` |

Mandatory command:

```text
go test ./internal/app \
  -run '^TestFinalLiveBoundFilesRequireCurrentUserOwnership$' \
  -count=1
```

Result: `FAIL`, exit `1`, only because the bounded portable ownership
predicate did not yet exist:

```text
undefined: finalLiveUIDMatchesCurrent
```

No Pi process, model, llama server, network request, live manifest, or live
canary ran. Live-canary invocations consumed by this repair: `0`.

The bounded owned-file repair is authorized.

VERDICT: PASS

## Reopen 1 Repair 2 — authoritative clock binding RED

Baseline:

```text
67b251cae0e3a2086163998b309b4ebb5beadca5
```

Fresh Contract Review 2 returned `PASS` before this checkpoint. The only
implementation/test file changed before RED was the owned controlled harness:

```text
internal/app/final_live_gate_live_test.go
e88ba7e782b9c4658640741f46733465a7be8f74b6b5229fe0f871cea369c5e1
```

The test-first change requires:

- one exact UTC snapshot and a non-drifting source;
- one clock closure returning that byte-identical snapshot on every call;
- rejection of zero, non-UTC, and drifting construction; and
- a new Repair-2 manifest and attempt prefix that cannot select the consumed
  Reopen-1 evidence lineage.

Production authority files remained unchanged:

```text
internal/work/verification_authority.go
efce682c5ef4f6c1068c625441386b0be5abad6120bf9290eaab757949b561be

internal/app/team_execution.go
47f4810201d46f013f093562f1c792190dea419489721377ca14900952b13354
```

Mandatory focused command:

```text
go test ./internal/app \
  -run '^(TestFinalLiveAuthoritativeClockBinding|TestFinalLivePi0821TranscriptClosureCanaryIsolation)$' \
  -count=1
```

Result: `FAIL`, exit `1`, because the required harness-local binding does not
exist:

```text
undefined: newFinalLiveAuthoritativeClock
```

The failure occurs at all four new construction assertions. It is not a skip,
fixture failure, network failure, Pi/model invocation, or production
authority failure. No live manifest or attempt was created. Repair-2
live-canary invocations consumed: `0`.

The minimal controlled-harness GREEN is authorized. All product, parser,
authority, Journal, Evidence, Projection, Supervisor, and Bridge files remain
read-only.

VERDICT: PASS

## Reopen 1 context-alignment RED

Date: `2026-07-28`

Fresh Contract Review 3 passed before this test-first checkpoint. Product
behavior remained unchanged:

| Product path | SHA-256 |
|---|---|
| `internal/runtime/piadapter/rpc_bridge_adapter.go` | `c5f5fd70787105c8ca80b355d55f980085fdb5a350c74cc692d0a3502e7a7343` |
| `internal/runtime/piadapter/local_model_server.go` | `ffe0886a5e5100280b3c1093541bdb2e37a1018996b33bcf1cd2357097344d45` |

The frozen Reopen-1 test checkpoint is:

| Test path | SHA-256 |
|---|---|
| `internal/runtime/piadapter/rpc_bridge_adapter_test.go` | `cdf9154c1573788288cdf36168ab0d357e682d926771fb78e546b502764605f5` |
| `internal/runtime/piadapter/local_model_server_test.go` | `b82a2286adb119a37d6471e486481bdb0348a3cff799f1241dad81f52392824a` |
| `internal/app/final_live_gate_pi0821_component_test.go` | `271d0e9d441481b69ea2f7de786dac218636200b65c383e44495dd452fc9ec5b` |
| `internal/app/final_live_gate_live_test.go` | `f4bb348da3068fe61e2dcbbead94f90c6428c74ae28cd83709098ffd661178f5` |

Focused RED:

```text
go test ./internal/runtime/piadapter \
  -run '^(TestPiRPCModelOutputBudget|TestPiLocalModelContextAlignment)$' \
  -count=1
```

Result: `FAIL`, exit `1`, only because both current context surfaces remain
the pre-repair `4096` while both output surfaces are already the required
`256`:

```text
Pi RPC model budget = context:4096 output:256, want 32768/256
local model budget = context:[4096] output:[256], want 32768/256
```

Locked-Pi deterministic component RED:

```text
LOOM_PI_0821_COMPONENT=1 \
LOOM_PI_0821_EXECUTABLE=<reviewed-installed-pi> \
LOOM_PI_0821_RUNTIME_SEARCH_PATH=<reviewed-node-search-path> \
LOOM_PI_0821_RUNTIME_INSTANCE_ID=runtime.pi.earendil-works.0.82.1 \
go test -v ./internal/app \
  -run '^TestPi0821DeterministicSSETeamExecutionClosure$' \
  -count=1
```

Before process start, the component validated current-user ownership, safe
mode, and exact hashes for the installed Pi executable and all three causal
source files. The source hashes matched the frozen contract.

Result: `FAIL`, exit `1`, after exactly one valid bounded loopback request:

```text
status="blocked"
node="blocked"
attempt="failed"
classification="invalid"
terminal_reason="runtime_process_failed"
phase=assistant_update event=text_start reason=terminal_stop_reason
requests=1 valid=true output_budget=1
```

This proves the unchanged `4096` declaration is clamped by locked Pi to one
output token and fails closed on `finish_reason: length`. No llama.cpp/model
process, Internet/DNS request, live manifest, live canary, raw transcript,
prompt, output, Grant, credential, hidden reasoning, private path, or
runtime-derived stop value was printed or persisted.

Reopen-1 live-canary invocations consumed: `0`.

The bounded shared-context Production Repair 1 is authorized.

VERDICT: PASS
