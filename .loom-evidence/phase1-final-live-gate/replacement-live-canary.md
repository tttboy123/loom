# Final Live Gate Replacement Controlled Canary

Date: 2026-07-27
Status: `FAIL — HUMAN_REQUIRED — NO SECOND REPLACEMENT`

## Authorization and exact repair

- Original controlled canary attempts: `1`, consumed and failed.
- User-authorized replacement attempts: `1`, consumed and failed.
- Remaining live attempts: `0`.
- Metadata repair commit:
  `b34d8da635c6037c6f3658d580c1dc75bf541f63`.
- Metadata repair Contract Review 2: `PASS`.
- Metadata repair Implementation Review: `PASS`.
- Focused, package, focused-race-50, full, full-race, vet, module, format,
  Windows compile, and scope checks: `PASS`.

The replacement used the same installed Pi, llama.cpp, model, private root,
loopback port, approval, local-execution authorization, and existing live test.
It added no retry, fallback, alternate model/source, credential, network
Provider, daemon, or autonomous execution.

## Fresh pre-live evidence

Before the replacement:

- the repair was the exact committed `HEAD`;
- Pi, llama-server, and model full SHA-256 values matched the frozen source
  lock;
- the model remained `1117320768` bytes, mode `0600`, and GGUF v3;
- the private root was current-user-owned mode `0700`;
- the sanitized resolved manifest was current-user-owned mode `0600`, parsed
  successfully, contained the exact three hashes, and contained no private
  absolute path or mirror detail;
- the loopback port had no listener and no final-live process existed;
- a production metadata-only preflight used the real installed Pi runner and
  the committed Loom parser and returned `PASS`; and
- its temporary overlay and private temporary directory were removed before
  live execution.

## Exact replacement invocation

The existing opt-in test ran exactly once after those gates:

```text
go test ./internal/app \
  -run '^TestFinalLiveGatePiRPCOfflineModel$' \
  -count=1 -v
```

Result:

```text
=== RUN   TestFinalLiveGatePiRPCOfflineModel
    final_live_gate_live_test.go:331: final live Team execution failed: status="" err=Team attempt evidence commit main/1: invalid Team node recovery
        runtime adapter failure
        Pi RPC protocol failed
        response=true agent=true turns=1 message=false done=false settled=false
--- FAIL: TestFinalLiveGatePiRPCOfflineModel (5.07s)
FAIL
FAIL    loom-pi-rebuild/internal/app
```

## Failure boundary

The replacement passed:

- exact installed Runtime discovery;
- local llama.cpp startup;
- Pi RPC adapter construction;
- WorkItem/Run/claim/generation/Grant creation; and
- Team attempt dispatch.

It failed closed after the Pi RPC prompt response, agent start, and first turn
start, before Loom accepted an assistant message, terminal result, or output
payload.

The failure receipt records:

```text
terminal_status         = failed
authorized_frame_count  = 1
output_frame_count      = 0
output_payload_bytes    = 0
evidence_digest         = 29e45d86f0d2de4b9ad06d8c344f071005045e21098b9b9c71c8ec7a7c2b5ccb
summary_digest          = f521318e939d6439fada924ab8e28ef5b8a8036bd96a3797023531740ab6a509
```

The one authorized Frame is the pre-output acknowledgement. No assistant
output Frame or payload was authorized.

## Source-level incompatibility

The installed locked Pi `0.82.1` source constructs a user message whose
`content` is an array containing a text block:

```text
role = user
content = [{ type: text, text: expandedText }]
timestamp = current time
```

RPC mode forwards the session event unchanged. Loom's
`piRPCUserMessage` instead requires the exact keys
`role`, `content`, and `timestamp` but parses `content` only as a JSON string.
Its deterministic fixture also models `content` as a string.

This schema mismatch explains the observed state: response, agent start, and
turn start were accepted, but the user-message record was not accepted and no
assistant message or settled state followed.

The raw RPC transcript was deliberately not persisted or copied into
repository evidence. Therefore this report claims the source-level schema
incompatibility that matches the observed state, not a byte-for-byte captured
dynamic record.

## Authoritative failure recovery

Read-only SQLite inspection found exactly one each of:

- `RunTerminalCommitted`;
- `WorkItemTerminal`;
- `TeamNodeAttemptTerminal`;
- `AgentGrantRevoked`;
- `RuntimeCapacityReleased`; and
- `EvidenceSubmitted`.

The Run, WorkItem, and Team attempt terminal statuses are all `failed`.
The Journal remains append-only, the Grant is revoked, capacity is released,
and digest-bound failure Evidence exists. No success/ready-for-review terminal
fact or assistant output was accepted.

## Cleanup and hardening observation

Immediately after failure:

- port `127.0.0.1:18427` had no listener;
- no llama-server, Pi RPC, or bound Runtime process remained;
- Pi, llama-server, and model full SHA-256 values remained exact;
- the resolved manifest remained sanitized and mode `0600`; and
- excluded user-owned worktree dirt remained unstaged.

The canary SQLite leaf was initially created as mode `0644` inside a mode
`0700` private parent. The private parent prevented traversal by other users,
but the leaf did not meet the desired defense-in-depth mode. The Controller
immediately tightened only that exact leaf to mode `0600` without changing its
contents. This is recorded as an additional bounded product-hardening gap; it
is not silently treated as passing behavior.

No raw Grant, credential, hidden reasoning, private absolute diagnostic path,
raw RPC record, or model output was copied into this replacement failure
evidence. The reviewed static prompt literal remains only in existing test
source and was not duplicated here.

## Stop condition

The one user-authorized replacement is consumed. This failure activates an
absolute stop:

- no second replacement;
- no retry or hidden rerun;
- no alternate model, source, Provider, or adapter;
- no Pi RPC parser repair; and
- no SQLite creation-mode repair

without a new explicit user authorization, frozen bounded amendment, fresh
Contract Review, RED/GREEN, full verification, and fresh Implementation
Review.

## Fresh independent failure-evidence review

The independent Reviewer returned `PASS` with no blocking findings. The review
confirmed:

- the original attempt count is one, the replacement attempt count is one, and
  the remaining authorized count is zero;
- the recorded failure state and source-level diagnosis are supported without
  claiming a byte-for-byte dynamic RPC transcript;
- no assistant output payload was accepted or copied into this evidence;
- cleanup, digest preservation, terminal failure facts, Grant revocation,
  capacity release, and digest-bound Evidence are recorded without converting
  the live result to success;
- the observed SQLite `0644` creation mode and immediate leaf-only `0600`
  hardening remain disclosed as a product-hardening gap; and
- commit `b34d8da635c6037c6f3658d580c1dc75bf541f63` contains only the reviewed
  metadata repair, while excluded user-owned worktree changes remain unstaged.

The Reviewer PASS validates this failure-evidence record only. It does not
authorize another execution and does not change the live result.

VERDICT: FAIL
