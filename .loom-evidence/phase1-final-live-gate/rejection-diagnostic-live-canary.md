# Final Live Gate Rejection-Diagnostic Live Canary

- Date: `2026-07-27`
- Amendment: `PHASE1-FINAL-LIVE-PI-RPC-REJECTION-REASON-CODE-1`
- Implementation commit:
  `c7cabee40682f6e670d67b2553d85d6f8544b5c8`
- Implementation Review: `PASS`
- Invocation allowance before execution: `1`
- Invocation count: `1`
- Remaining invocation allowance: `0`
- Retry performed: `no`

## Result

The single authorized isolated diagnostic canary reached the previously proven
post-assistant-start boundary and then failed closed with exactly one allowed
diagnostic triple:

```text
phase=assistant_update event=text_start reason=text_content_progression
```

The retained bounded adapter state was:

```text
response=true agent=true turns=1 message=true done=false settled=false
```

No second diagnostic triple, free-form Runtime value, response or model value,
usage value, timestamp, prompt, model output, raw transcript, raw Grant,
credential, hidden reasoning, Event ID, or Run ID was emitted.

This result eliminates the instrumented predicates that precede or follow this
exact short-circuit point for the rejected record. In particular, the record
passed event shape and kind, content index, top-level/nested partial equality,
assistant message schema, response-model absence, response-ID transition,
timestamp identity, and usage schema/progression before failing the text
content progression predicate.

## Locked-source explanation

The installed Pi `0.82.1` source creates an empty text block, pushes
`text_start` with the mutable assistant output as `partial`, and then appends
the first text delta to that same block in the same synchronous call stack.
The installed generic event stream queues or resolves the event object by
reference and performs no clone.

A no-network, no-model minimal reproduction using the installed event-stream
implementation observed a `text_start` event whose referenced text length was
already `1` after the producer appended one byte. This proves that an async
consumer can observe non-empty text in `text_start.partial` even though the
producer constructed the block as empty before `push`.

Locked source hashes used for this conclusion:

```text
event-stream.js
44a2498660ca61efa952ad6a3f10cc0491883411bd2b4572c9a392ec4e9553ec

openai-completions.js
0d50250fe2931e66e2078279a397814202e1ecddee58faf4b8bc04c278da177a
```

This is a source-level explanation only. It does not authorize an acceptance
change.

## Isolation and authorization

- Exactly one fresh direct-child diagnostic attempt root was created.
- The attempt root and every descendant directory were mode `0700`, owner
  `uid 501`.
- Every retained attempt file was mode `0600`, owner `uid 501`.
- The SQLite leaf was pre-created at mode `0600`.
- The independent manifest was mode `0600`, owner `uid 501`, size `1590`.
- The manifest SHA-256 was
  `f0d7e25d1e908876552ed037a92a93e5ecca93490b6d73b60fdcc39e82a9bed9`.
- The manifest contained no private path, mirror URL, prompt text, raw Grant,
  credential, hidden reasoning, model output, or Runtime-derived diagnostic
  value.
- No prior attempt or manifest was selected, reopened, migrated, or appended
  by the canary.

During post-run evidence inspection, a read-only SQLite command was initially
given a nonexistent sibling filename. SQLite created one zero-byte `0644`
file. The Controller immediately identified and removed that exact empty file
before authoritative inspection, disclosed the deviation, and then reopened
the correct database with immutable read-only URI settings. No retained
artifact, evidence, or database byte changed; the final attempt-tree mode and
content checks passed.

## Journal and Evidence lineage

The retained SQLite journal contained:

```text
events=21
distinct_event_ids=21
distinct_stream_sequence_pairs=21
```

It contained exactly one each of:

- `RunTerminalCommitted`, status `failed`, reason
  `runtime_process_failed`, generation `1`;
- `WorkItemTerminal`, status `failed`, generation `1`;
- `TeamNodeAttemptTerminal`, status `failed`, output classification
  `invalid`, generation `1`;
- `AgentGrantRevoked`, reason `terminal`;
- `RuntimeCapacityReleased`; and
- `EvidenceSubmitted`.

The Evidence lineage retained one authorized Ack frame, zero output frames,
zero output payload bytes, no observed child result, and failed terminal
status. Artifact, receipt, capture, bounded-source, and SQLite SHA-256 values
were respectively:

```text
2b09580a3cdd496e9e4d9867db5b32818bf4634d5bc826e84dd2c6749be80386
4d54b2e75fbdbdca92dedbf3aee8e8e8c1ad04a3cd556eb93c4e93d429aebc44
99e88926ef8dcbc3b4683d87be5f3672789dc1aef458d4fff94124809b878e5e
bfc5aca5df67c1244e4d28031f973404e5a118d5587288609a625906d3d1a54f
5d63c3f02571d7a80766ca5896430df5ae5a30aa60b1792cd0d8f5e0f24a0ce4
```

The journal contained no payload key named `token`, `grant_token`, or
`raw_grant`; only the existing token digest field was present.

## Cleanup and preservation

- No listener remained on the controlled port.
- No final-live Pi, llama-server, or Loom daemon process remained.
- Pi CLI SHA-256 remained
  `af302f231437eaf6f37691bce4b34234fcb626bcb5eb3910d4fc3f6519bf78ca`.
- llama-server SHA-256 remained
  `a4998768a70ba2be02617ec9d8773accc2952516f4f5a8f38f621ece54cbf04b`.
- Model size remained `1117320768`, mode `0600`, owner `uid 501`, with GGUF
  v3 header and SHA-256
  `cc324af070c2ecbfd324a30884d2f951a7ff756aba85cb811a6ec436933bb046`.
- Every frozen historical evidence, manifest, source-lock, and prior SQLite
  hash matched its pre-live value.
- User-owned dirty files remained unstaged and uncommitted.

## Gate decision

The diagnostic invocation failed with one valid closed reason code. It
consumed the only invocation authorization. No retry, fallback, compaction,
model/provider switch, parser widening, production activation, push, merge, or
release is authorized.

Result-evidence Review: `PASS`

The fresh independent read-only Reviewer reported no Critical or Important
findings. It classified the disclosed post-run zero-byte SQLite sibling
creation/removal as a Minor process-hygiene issue, not result ambiguity,
because the file never contained data, was removed before immutable
authoritative inspection, and every retained artifact, database, manifest,
historical hash, mode, ownership, and cleanup invariant independently
revalidated.

Required exit after a fresh result-evidence Reviewer `PASS`:

```text
HUMAN_REQUIRED — DIAGNOSED — NO RETRY
```

VERDICT: FAIL
