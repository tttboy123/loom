# Implementation GREEN

Date: 2026-07-27

Status: `PASS — HERMETIC ONLY`

Implemented only the frozen compatibility boundary:

- exact Pi 0.82.1 no-model metadata diagnostic validation and discard;
- Pi 0.82.1 RPC `partial`/`message` lifecycle translation to newly constructed
  `loom.bridge.v1` Frames;
- synchronous passage of every constructed Frame through the existing
  Supervisor `FrameSink`;
- short-lived loopback llama.cpp lifecycle with file binding, model digest,
  exact arguments, health, timeout, and process-group cleanup; and
- an opt-in one-node final live harness that is skipped unless every explicit
  gate environment binding is present.

The installed Pi 0.82.1 RPC types and OpenAI-compatible implementation were
bound into `source-lock.json`. The compatibility decoder accepts the actual
`start.partial`, text-event `partial`, and `done.message` shape. Only text
content blocks are accepted; thinking, tool, retry, non-stop, extension,
unknown, duplicate-key, and out-of-order paths fail closed.

Bounded product Repair 1 additionally:

- accepts the exact Pi 0.82.1 `turn_start`, user-message pair, assistant-message
  lifecycle, and two-message `agent_end` sequence while applying strict nested
  allowlists;
- closes Pi stdin at settled, drains stdout through EOF, validates successful
  process exit, and only then publishes digest Evidence and Result;
- rejects metadata CR/path controls before trimming;
- writes an atomic sanitized resolved manifest before any live operation; and
- directly verifies model binding drift, occupied port, invalid health,
  cancellation, unsafe file type/mode, and inherited environment removal.

Targeted GREEN:

```text
go test ./internal/runtime ./internal/runtime/piadapter \
  -run 'Pi0821|PiRPCBridge|PiLocalModel' -count=1
```

Result: `PASS`.

Stability:

```text
go test ./internal/runtime/piadapter -run 'PiRPCBridge' -count=10
go test ./internal/runtime/piadapter -run 'PiLocalModel' -count=3
```

Result: `PASS`.

No llama.cpp archive, model asset, Provider request, or live canary was
materialized or executed.

VERDICT: PASS

## Bounded Product Repair 2

Status: `PASS — HERMETIC ONLY, IMPLEMENTATION REVIEW 3 PENDING`

Technical Correction 2 now supersedes the earlier synthetic transcript and
inferred-root implementation:

- the Pi 0.82.1 diagnostic accepts exactly three physical LF-delimited lines,
  optionally one final LF, and rejects blank/boundary whitespace;
- Provider start/done are represented only by top-level assistant
  `message_start`/`message_end`; nested `start`, `done`, and `error` updates
  fail closed;
- every text update requires semantic equality between its top-level `message`
  and nested `partial`, stable assistant identity/usage shape, and a locked
  response model;
- the final assistant message is exact `stopReason:"stop"` and is
  semantically identical in `turn_end` and `agent_end`;
- `PiLocalModelServerConfig` carries an explicit `PrivateRoot`;
- one shared read-only inspector binds the full path chains, current owner,
  exact 0700 directory/executable modes, exact 0600 model mode, no-follow leaf
  descriptors, pathname identity, size, and digest;
- server start performs the same inspection and a second full-chain
  revalidation immediately before process start; and
- the resolved live manifest obtains llama.cpp/model digests from the shared
  inspector before validating Pi or writing the atomic 0600 manifest.

The Product Repair 2 focused gate and additional stability checks passed:

```text
go test ./internal/runtime ./internal/runtime/piadapter \
  -run 'Pi0821|PiRPCBridge|PiLocalModel' -count=1
go test ./internal/runtime/piadapter -run 'PiRPCBridge' -count=10
go test ./internal/runtime/piadapter -run 'PiLocalModel' -count=3
go test ./internal/runtime/piadapter -run '^$' \
  -fuzz '^FuzzPiRPCObjectNoPanic$' -fuzztime=5s
```

The fresh fuzz run completed 125,244 executions with no failure.

No llama.cpp archive, GGUF, Provider/model request, or live canary was
materialized or executed.

VERDICT: PASS

## Bounded Product Repair 3

Status: `FAIL — IMPLEMENTATION REVIEW 4 REPRODUCED A FOCUSED FLAKE`

The final bounded repair closes the local-model readiness race found by
Implementation Review 3:

- startup requires three consecutive exact health observations while
  continuously monitoring the child wait channel;
- one final nonblocking child-exit check occurs before a ready server can be
  returned;
- the loopback port is checked again immediately before process start; and
- a deterministic test seam proves a listener that steals the port after the
  first free check is rejected before the child starts.

The behavioral health-then-exit fixture normally fails closed with
`ErrPiLocalModelProcess`, and the port-race fixture fails closed with
`ErrInvalidPiLocalModel`. Implementation Review 4 reproduced a high-load case
where the one-second fixture deadline won and returned
`ErrPiLocalModelHealth`; cleanup remained process-group bounded, but the exact
classification evidence is not stable.

No llama.cpp archive, GGUF, Provider/model request, or live canary was
materialized or executed.

VERDICT: FAIL

## Authorized Bounded Product Repair 4

Status: `PASS — HERMETIC ONLY, IMPLEMENTATION REVIEW 5 PASS`

Repair 4 changes only timeout/child-exit classification and its fixture:

- an expired health context gives an imminent child wait result a bounded
  25-millisecond classification window;
- an observed child exit remains `ErrPiLocalModelProcess`;
- a still-live process remains the original typed health cancellation/timeout;
  and
- the health-then-exit fixture uses the same ten-second test-only startup
  budget as the successful local-server fixture.

The two classification paths plus health-then-exit passed 20 consecutive
targeted repetitions before the complete matrix.

No llama.cpp archive, GGUF, Provider/model request, or live canary was
materialized or executed.

VERDICT: PASS
