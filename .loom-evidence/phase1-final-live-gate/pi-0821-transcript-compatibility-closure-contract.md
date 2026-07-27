# Final Live Gate Pi 0.82.1 Transcript Compatibility Closure Contract

Status: FROZEN — REOPEN 1 IMPLEMENTATION REVIEW PASS — COMMIT AUTHORIZED

- Contract: `PHASE1-FINAL-LIVE-PI-0821-TRANSCRIPT-CLOSURE-1`
- Risk: `HIGH`
- Baseline: `c7cabee40682f6e670d67b2553d85d6f8544b5c8`
- Date: `2026-07-27`
- Kind: unique Final Live Gate compatibility closure
- Depends on:
  - Final Live Gate Pi RPC Rejection Reason-Code Instrumentation Contract and
    Implementation Reviews `PASS`;
  - local atomic diagnostic commit
    `c7cabee40682f6e670d67b2553d85d6f8544b5c8`;
  - the rejection-diagnostic controlled canary's reviewed `FAIL` evidence,
    with exactly one invocation consumed and no retry performed;
  - the closed diagnostic
    `phase=assistant_update event=text_start
    reason=text_content_progression`;
  - locked installed Pi `0.82.1` source and a no-network, no-model minimal
    reproduction proving mutable forward-partial reference observation; and
  - explicit user authorization on `2026-07-27` to replace the single-point
    Forward-Partial Snapshot Amendment with this unique closure contract,
    cover the complete RPC lifecycle and controlled vertical execution
    boundary, and, only after a fresh Implementation Reviewer `PASS`, run
    exactly one new isolated controlled live canary.

## Unique closure rule

This contract is the only remaining Final Live Gate Pi `0.82.1` transcript
compatibility Candidate.

It must not be split into another response-ID, partial, delta, terminal,
component, Supervisor, Grant, Frame, Evidence, cleanup, or live-canary
Amendment/WorkItem. Thin follow-on wrappers and single-predicate Amendments are
forbidden.

The Candidate must close the complete vertical boundary in one reviewed
lineage:

```text
locked Pi 0.82.1
→ deterministic OpenAI-compatible loopback SSE
→ Pi RPC full transcript
→ Pi RPC Bridge validation
→ BoundRunStream
→ AgentGrant authorization
→ Supervisor
→ Team execution
→ Frame observation
→ Evidence artifact/receipt/capture
→ Journal terminal facts
→ Projection terminal state
→ cleanup/recovery invariants
```

If a Critical/Important defect is found, repair remains inside this one
Candidate and its frozen ownership. The normal maximum of three bounded
product repairs applies. A boundary that cannot be repaired without changing
an excluded authority, permission model, dependency, protocol, or product
scope stops at `HUMAN_REQUIRED`; it does not create a new single-point
Amendment.

### In-place Repair 3 after Implementation Review 1

Fresh Implementation Review 1 returned `FAIL` with three Important findings:

1. the exact final-RED test hashes changed during bounded test-fixture
   correction, while the original contract permitted only `gofmt`;
2. the real-Pi component asserted only a subset of the required
   Frame/Evidence/Journal closure; and
3. fixed cross-record SSE sleeps made the component prove only a paced path
   and left an untested live-equivalent mutable-reference timing mode.

This section repairs the same unique Closure Contract. It is not a new
Amendment, WorkItem, Candidate, or live authorization.

The following disclosed test-only changes occurred after final RED and before
Implementation Review 1:

- hermetic requests are constructed before concurrent goroutines;
- multi-chunk and unreconstructed forward fixtures were corrected so their
  intended predicate, rather than an earlier shrink, is reached;
- invalid UTF-8 uses an actual invalid source byte rather than a JSON surrogate
  that Go replaces;
- the component Authority clock is fixed to its authoritative decision time;
- bounded failure diagnostics record only closed adapter/status codes;
- the loopback temporarily added fixed pacing while diagnosing queued mutable
  references;
- unused diagnostic code was removed; and
- the live harness retained the prior rejection manifest as historical
  evidence while switching the current constant/prefix.

Their disclosed pre-Repair-3 hashes are:

| Path | SHA-256 |
|---|---|
| `internal/runtime/piadapter/rpc_bridge_adapter_test.go` | `8905a6b46564646e57c33f786a385dce0ad2a38e4321a8d778869499602c4ccc` |
| `internal/app/final_live_gate_pi0821_component_test.go` | `98af610cab78cbf60deead4ec63117351f338f759802d4db968bfc9a954136ef` |
| `internal/app/final_live_gate_live_test.go` | `c549445e8220dd6e2eee3324ac8512150502c8b5324a1f5c21abac20242ba624` |

The original final-RED hashes remain immutable historical evidence, but the
governance rule that forbade every semantic test repair is replaced by this
bounded repair ledger. After fresh Repair-3 Contract Reviewer `PASS`, tests
must first add the exact repaired acceptance/failure assertions below and
record a new Repair-3 RED against the current production Candidate. Production
Repair 1 may begin only after that RED. The repaired tests are then frozen
again: later semantic test drift requires another in-place review and still
cannot create a point Amendment.

Repair 3 keeps all original ownership and adds no production file. It requires:

- safe compatibility for Pi's second mutable-reference timing mode, where the
  top-level message text and nested partial text may be prefix-comparable
  snapshots from different instants;
- an unpaced, flush-per-record real-Pi success component, repeated under race;
- exact success closure assertions for Frame order, output digest/bytes,
  artifact/receipt/capture binding, Journal terminal/capacity/evidence facts,
  duplicate rejection, and Projection terminal;
- one separate real-Pi observer-failure component proving no successful
  AdapterResult, one failed Evidence lineage, Grant revocation, capacity
  release, and no retry; and
- preservation of the exact one-live-canary gate.

No top-level/nested divergence, normalization, repair, synthesized delta, or
snapshot-derived output is authorized.

### In-place locked-Pi usage-skew repair during Production Repair 1

The mandatory unpaced locked-Pi component passed once, then its required
race repetition exposed a real Pi `0.82.1` serialization mode not represented
by the Repair-3 hermetic matrix:

- the top-level and nested text values were byte-for-byte equal;
- every non-text field except `usage` was semantically equal;
- the top-level message retained an earlier valid usage snapshot;
- the nested partial carried the later valid usage snapshot; and
- the pre-repair product rejected the update only with the closed
  `phase=assistant_update event=text_delta
  reason=message_partial_mismatch` diagnostic.

The bounded diagnostic ran only the deterministic loopback component. It did
not enable a model, use a live manifest, consume the one live authorization,
or persist raw RPC/SSE records or usage values. Temporary local diagnostics
must be absent from the Candidate before RED, verification, and review.

This is a repair inside the same unique Closure Contract, ownership, Candidate
lineage, and Production Repair 1. It is not a new Amendment, WorkItem,
Candidate, product surface, or live authorization.

For `text_start` and `text_delta` only, top-level `usage` may precede nested
partial `usage` when all of the following are true:

1. both complete assistant messages independently pass the frozen schema,
   identity, UTF-8, bound, and duplicate-key checks;
2. after replacing only text and then excluding only `usage`, every remaining
   field is semantically equal;
3. every required top-level usage and nested cost number is component-wise
   non-decreasing from the top-level message to the nested partial;
4. optional `reasoning` may remain absent, remain present without decreasing,
   or appear only in the nested partial; it may not disappear;
5. `cacheWrite1h` remains forbidden in either snapshot;
6. no reverse, crossed, incomparable, malformed, negative, non-finite, or
   extra-key usage relation is accepted; and
7. the nested partial remains the identity/usage progression input already
   governed by `acceptUpdate`.

At `text_end`, usage skew is forbidden: top-level message and nested partial
must again be semantically equal after replacing only text. All later terminal
records retain their existing exact identity/content requirements. Usage
never contributes output bytes, Frames, digests, Evidence, state authority,
or terminal synthesis.

After fresh contract-only Reviewer `PASS`, a new hermetic test-first checkpoint
must prove:

- accepted top-level-lagging nested-usage progression at `text_start` and
  `text_delta`, including prefix-skew text;
- unchanged acceptance when usage is equal;
- rejection of reverse, crossed, decreasing, disappearing-optional,
  `cacheWrite1h`, malformed, and terminal usage skew; and
- the unmodified pre-repair product fails the positive cases only with the
  closed `message_partial_mismatch` reason.

Only then may Production Repair 1 continue. The full real-Pi component/race and
repository verification matrices must be rerun from a clean non-diagnostic
Candidate.

### In-place Implementation Review 2 repair

Fresh Implementation Review 2 returned `FAIL` with two Important findings:

1. the contract's blanket private-Evidence non-disclosure rule contradicted
   the existing read-only Evidence authority, which intentionally persists
   every Bridge-validated, Grant-authorized canonical Frame for attempt
   lineage and replay; and
2. the locked-Pi component verified file hashes and writability but did not
   explicitly prove resolved files were owned by the current user.

This section repairs the same unique Closure Contract. It is not a new
Amendment, WorkItem, Candidate, authority, or live authorization.

The existing Evidence design remains read-only and authoritative:

- a private `0600` attempt artifact may retain only the exact canonical
  Bridge Frames that passed binding, sequence, and AgentGrant authorization;
- authorized Event payloads may therefore contain tentative assistant output
  bytes inside that private artifact;
- the capture remains attempt-scoped, digest-bound, size-bounded, and
  inaccessible to Journal/Projection as output state;
- observer rejection may retain the already authorized Event that triggered
  the rejection, but no later Evidence/Result Frame;
- terminal acceptance remains bound to the separate terminal status,
  Evidence digest, receipt, and capture closure; and
- no raw RPC/SSE record, forward snapshot, prompt, system prompt, Grant,
  credential, hidden reasoning, environment value, or unauthorized Frame is
  permitted in the artifact.

Repository governance evidence and test output remain sanitized and may record
only hashes, counts, types, statuses, closed reasons, modes, ownership, and
authorization accounting. Journal remains forbidden from storing tentative
assistant output. This repair does not authorize changing
`internal/evidence`, `internal/app/team_execution.go`, Journal, Projection,
Bridge, Supervisor, Grant, or StateWriter behavior.

The component must decode the immutable artifact's canonical frame lines and
prove exact equality with the already observed authorized Frames on success.
On observer failure it must prove the artifact contains exactly Ack plus the
single rejected-but-authorized Event and no later Evidence/Result. Both paths
must scan the artifact for prompt, raw Grant, credentials, private paths, and
other forbidden material.

The current-user ownership repair is limited to the already owned test/helper
files. Before Pi process start, the component must compare the resolved Pi CLI
and both locked source files' UID with the current user's UID, in addition to
the existing regular-file, digest, and non-group/world-writable checks. The
portable Windows compile-only gate must remain green.

After fresh contract-only Reviewer `PASS`, tests must first fail against the
current Candidate for the missing ownership predicate and insufficient
artifact-content assertion, then the bounded test/helper repair may proceed.
The full component/race/repository verification matrix must be rerun before a
new fresh Implementation Review. Live authorization remains unconsumed and
locked.

## Proven incompatibility

The single rejection-diagnostic canary accepted:

- the correlated Pi RPC command response;
- `agent_start`;
- one `turn_start`;
- the exact user `message_start` and `message_end`;
- the initial assistant `message_start`; and
- every assistant-update predicate through usage progression.

It rejected the first `text_start` only because its text content was already
non-empty:

```text
phase=assistant_update event=text_start reason=text_content_progression
```

The locked Pi `0.82.1` provider code:

1. creates an empty text block;
2. pushes `text_start` with the mutable assistant output as `partial`;
3. appends the first Provider delta to that same block in the same synchronous
   call stack; and
4. pushes `text_delta`.

The locked generic event stream queues or resolves event objects by reference
without cloning. Its async consumer can therefore observe `text_start.partial`
after one or more Provider deltas have already mutated the shared object.
Later queued `text_delta.partial` values may likewise be forward snapshots
rather than event-time immutable snapshots.

This is a Pi event-object snapshot-timing compatibility issue. It is not a
Provider/model substitution, retry, compaction, terminal, Bridge, Grant,
Evidence, or Runtime-discovery issue. This contract nevertheless verifies all
of those downstream boundaries so the Final Live Gate does not stop again on
an untested transcript or integration seam.

## Frozen ownership

### Writable product and test files

Production behavior ownership is limited to:

- `internal/runtime/piadapter/rpc_bridge_adapter.go`

The only new exported surface permitted in that file is the closed,
non-authoritative `PiRPCTranscriptAudit` value and the best-effort send-only
audit channel frozen below. It exists solely so the cross-package component
test can prove the real Pi transcript shape without disclosing transcript
bytes. It is not a Frame sink, state writer, authorization input, or production
telemetry API.

Hermetic and component test ownership is limited to:

- `internal/runtime/piadapter/rpc_bridge_adapter_test.go`
- `internal/app/final_live_gate_pi0821_component_test.go` (new)
- `internal/app/final_live_gate_live_test.go`

### Read-only exercised authorities

The following are required component-test inputs but remain read-only:

- `internal/supervisor/managed_execution.go`
- `internal/authorization/authority.go`
- `internal/evidence/store.go`
- `internal/journal/`
- `internal/projection/`
- `internal/app/team_execution.go`
- `internal/work/`
- `internal/teams/`
- `protocol/bridge/v1/`

The Candidate must not change permission rules, StateWriter/Journal authority,
Evidence semantics, Supervisor binding rules, Bridge v1, terminal aggregation,
or Projection authority. A discovered need to modify one of these read-only
surfaces is `HUMAN_REQUIRED` inside this same closure contract.

### Governance evidence

Governance evidence ownership is limited to:

- this contract;
- `pi-0821-transcript-compatibility-closure-contract-review.md`;
- `pi-0821-transcript-compatibility-closure-red.md`;
- `pi-0821-transcript-compatibility-closure-verification.md`;
- `pi-0821-transcript-compatibility-closure-implementation-review.md`;
- after the implementation commit and immediately before the authorized
  invocation,
  `resolved-live-manifest-pi-0821-transcript-closure-canary.json`;
- after that invocation,
  `pi-0821-transcript-closure-live-canary.md`; and
- the private materialization status for bounded result synchronization only.

### Freeze-time dirty-worktree quarantine

The repository was not clean when this contract was frozen. The following
modified files are pre-existing user/governance state, are not Candidate
changes, and must remain byte-for-byte unchanged:

| Path | Freeze-time SHA-256 |
|---|---|
| `.loom-evidence/phase1-final-live-gate/contract.md` | `9c5e71767f9983bc53089a8c22a03fcf430bd784d5e2fc602b618a9fca9e1dff` |
| `.loom-evidence/phase1-final-live-gate/contract-review.md` | `7cbbeb13b4699fee65e0454329306f967bcc16a32e87aca5fe790c43454b22a1` |
| `.loom-evidence/phase1-final-live-gate/source-lock.json` | `e542aa319219f486cb45538df53ff49fb03fb180b748b9661bfb2497d58658f5` |
| `AGENTS.md` | `9196bf1a9cda6448688b073a2217ff5e806b1bcb288506d3be7ed6e204c4691f` |
| `PROGRESS.md` | `49b78621fa83b26967ac3ceda305bf14ffecba779bc5c3446595557bbdbf355c` |

The following pre-existing untracked Final Live Gate inputs are likewise
quarantined and must retain these hashes:

| Path | Freeze-time SHA-256 |
|---|---|
| `source-provenance-amendment.md` | `a8f9a26573b2ce9db9ccaeb2f315063bbf07d5bd0a0b67754d38ca23d78c7d26` |
| `live-canary.md` | `2dbd149403f290800503514293a204288bb85c62d7f4c7acb4c409db1a5a1665` |
| `replacement-live-canary.md` | `05434337f83418c9e68e310e1e0281802935711e9cc4eaa5f408641396248273` |
| `additional-live-canary.md` | `2559ef9177900910e867cf196266fa2a836dd78f75c1ed37403c4ef183f2a014` |
| `progressive-identity-live-canary.md` | `5a154572c00de938cf81a31134c6a5d1c225ce8cb917be735cff787955cc59a7` |
| `rejection-diagnostic-live-canary.md` | `82adaf3345e481eabbf2db5c3b310f09e04f50dac1affa8fa08f129eedde4499` |
| `resolved-live-manifest.json` | `b1fb029113655aad94e713ca9d5184b0083a847d28f72487e36ef974aa74fe83` |
| `resolved-live-manifest-additional-canary.json` | `222d6004fc06a8afc3e1c9a8369216d9b6f3f97fa116a386c039df27362c3c32` |
| `resolved-live-manifest-progressive-identity-canary.json` | `da04a07f47e84e278b29a841f9495849ffd923f27fe00acd4f8fc2d84e83f16c` |
| `resolved-live-manifest-rejection-diagnostic-canary.json` | `f0d7e25d1e908876552ed037a92a93e5ecca93490b6d73b60fdcc39e82a9bed9` |

The pre-existing untracked `.codex/installation_id`, `.codex/skills/`,
`.loom-drafts/`, and
`.loom-evidence/phase1-slice3/POST-S3-W5-QUEUED-CONTRACT-INPUTS.md` are
user-owned and excluded in their entirety. They must not be read for product
input, modified, staged, or committed by this Candidate.

Candidate scope is measured from this freeze-time quarantine, not by treating
every `git diff HEAD` entry as Candidate-owned. Review and commit must use an
exact allowlist of the writable product/test/evidence paths above. Immediately
before Implementation Review and commit, the Controller must:

1. re-hash every quarantined file in the two tables;
2. prove `git diff --cached --name-only` contains only the exact Candidate
   allowlist;
3. prove every non-owned status entry is identical to the freeze-time status;
   and
4. preserve a sanitized path/hash/status audit in verification evidence.

All pre-existing final-live evidence is read-only, including `contract.md`,
`contract-review.md`, `source-lock.json`, `source-provenance-amendment.md`,
every prior amendment/review/verification, `live-canary.md`,
`replacement-live-canary.md`, `additional-live-canary.md`,
`progressive-identity-live-canary.md`,
`rejection-diagnostic-live-canary.md`, and every earlier resolved manifest.
Every existing private attempt root and its SQLite/Evidence descendants is
read-only. This contract must not rename, overwrite, append to, migrate, or
delete any of them.

No dependency, module, ADR, root policy, core validator permission model,
authoritative StateWriter, credential boundary, Runtime installation, model,
source lock, `.env`, daemon, scheduler, CLI product surface, roadmap queue, or
user-owned dirty file is owned.

## Complete Pi RPC lifecycle contract

An accepted transcript contains exactly one ordered lifecycle:

```text
correlated successful prompt response
agent_start
turn_start
user message_start
user message_end
assistant message_start
text_start
one or more text_delta
text_end
assistant message_end
turn_end with no tool results
agent_end with willRetry=false
agent_settled
clean child exit
```

The existing exact-key, correlation, identity, single-turn, single-assistant,
text-only, no-tool, no-retry, no-compaction, `stop` terminal, exit-code,
stderr-bound, record-count, line-size, total-output, cancellation, and process
cleanup rules remain mandatory.

The closure test matrix must prove rejection of missing, duplicate,
out-of-order, late, or post-terminal lifecycle records at every phase. It must
retain the complete existing adversarial and reason-code matrices, including:

- uncorrelated/failed prompt response;
- duplicate/unknown JSON keys and invalid record shape;
- extra turn, message, update, terminal, or record after settled;
- invalid user text-block-array binding;
- invalid assistant schema, partial mismatch, index, or event kind;
- thinking, tools, extension records, retries, compaction recovery, and
  non-`stop` completion;
- response-ID, response-model, timestamp, usage, and reasoning progression
  violations;
- invalid/empty/oversized/Grant-bearing deltas;
- invalid text end and every terminal identity mismatch;
- cancellation acknowledgement and cleanup; and
- child failure, timeout, bounded stderr, and output/record overflow.

No accepted transcript may omit `agent_settled` or clean process exit. No
terminal or exit condition may synthesize a missing transcript phase.

## Forward-partial multi-chunk contract

### Delta-only output authority

The ordered Pi `text_delta.delta` sequence remains the sole assistant-text
output authority.

A Pi assistant-message `content` snapshot is only an untrusted forward
consistency witness. It must never:

- directly create a Bridge output Frame;
- contribute bytes to the authoritative assistant accumulator;
- contribute bytes to the terminal assistant digest or Evidence;
- synthesize a missing delta;
- fill a gap, repair a divergence, or deduplicate a delta;
- authorize a result or terminal; or
- become a second output/state authority.

Only a validated and authorized `text_delta.delta` may append assistant bytes
and produce output Frames. Existing Frame binding, sequence, Grant
authorization, chunking, digest, Evidence, and result semantics remain
unchanged.

### Formal forward-partial invariant

The adapter may accept either:

1. the already accepted immutable-snapshot transcript, where `text_start`
   contains one empty text block and every later partial exactly equals the
   deltas accepted so far; or
2. the locked Pi `0.82.1` forward-partial transcript, where `text_start` and
   later updates may expose text ahead of the deltas accepted so far.

For every accepted assistant text update, define:

```text
D = exact concatenation of all accepted text_delta.delta values so far
M = text in the current top-level message
P = text in the nested partial
W = the longer of M and P, only when one is a byte prefix of the other
H = last accepted witness W
```

The following invariants are mandatory:

1. the top-level message and nested partial are semantically equal after only
   their text-block `text` values are replaced by the empty string, except for
   the bounded non-terminal top-level-lagging `usage` relation frozen above;
2. each contains exactly one text block from `text_start` through `text_end`;
3. `M` and `P` are byte-prefix comparable; neither may diverge;
4. `M`, `P`, and `W` are valid UTF-8 and no larger than the configured
   assistant-output bound;
5. neither `M` nor `P` contains the raw Grant value;
6. `H` is a byte prefix of `W`; the accepted witness never shrinks, replaces,
   or diverges even when one current serialization snapshot lags;
7. `D` is a byte prefix of both `M` and `P`; deltas never claim bytes
   inconsistent with either observed snapshot;
8. only newly accepted delta bytes extend `D`;
9. `text_end.content`, `M`, `P`, `W`, `H`, and `D` are byte-for-byte equal;
10. assistant `message_end`, `turn_end`, `agent_end`, and `agent_settled`
   retain existing exact terminal identity/content requirements; and
11. no result is accepted if any forward snapshot byte was not eventually
    reconstructed by the ordered delta sequence.

The prefix relation is byte-exact after UTF-8 validation. The adapter may
select only the longer prefix-comparable witness; it cannot concatenate,
deduplicate, normalize, trim, fold, repair, or otherwise synthesize snapshot
text.

### State mutation discipline

Minimal new state may retain a private clone of the last validated partial
text only. It is bounded by the existing assistant-output limit and discarded
with the attempt.

Validation occurs before accepting the current event:

1. exact event shape and supported kind;
2. lifecycle state;
3. content index;
4. top-level/nested equality excluding only the text value, or the exact
   bounded non-terminal top-level-lagging `usage` relation frozen above,
   followed by byte-prefix comparability of `M` and `P`;
5. assistant schema and identity/usage restrictions;
6. UTF-8, configured bound, and raw-Grant exclusion;
7. monotonic `H` prefix of `W`;
8. candidate `D` prefix of both `M` and `P`;
9. delta policy and output bound; and
10. Frame construction and FrameSink authorization.

No partially validated snapshot may mutate `H`. No rejected delta may mutate
the authoritative accumulator. Existing failed execution, cleanup, revocation,
capacity-release, and failed Evidence semantics remain in force.

### Sanitized transcript audit seam

The component must not infer forward-partial behavior merely from a successful
terminal result. The adapter may add exactly this closed observation surface:

```text
type PiRPCTranscriptAudit struct {
    ForwardPartialObserved    bool
    TextStartSnapshotBytes    int
    AcceptedBytesAtTextStart  int
    AcceptedDeltaCount        int
    FinalSnapshotBytes        int
    FinalAcceptedDeltaBytes   int
    DeltaClosure              bool
}

```

`PiRPCBridgeAdapterConfig` may add exactly:

```text
TranscriptAudit chan<- PiRPCTranscriptAudit
```

The adapter performs one best-effort send only after:

1. the child has exited cleanly;
2. the full transcript has passed terminal validation;
3. ordered deltas exactly close the final partial snapshot;
4. Evidence and Result Frames have been accepted; and
5. `supervisor.NewAdapterResult` has succeeded.

Publication is a private helper with a deferred `recover` around exactly one
non-blocking `select`:

```text
select {
case sink <- audit:
default:
}
```

The audit struct is a by-value collection of bools and ints and contains no
pointer, slice, map, interface, function, channel, string, or reference to
adapter/request state. A nil channel, an unbuffered channel with no ready
receiver, or a full channel drops the audit immediately. A closed channel may
panic at send selection; that panic is recovered inside the helper and is also
dropped. The adapter never waits, retries, closes the channel, reads from it,
or reports publication success/failure. The caller owns the channel and must
not concurrently close it while `Execute` is active.

Therefore audit availability cannot authorize, reject, mutate adapter state,
create or suppress Frames/Evidence, change cleanup, change the returned
`AdapterResult`/error, or create a goroutine leak. A nil channel preserves all
existing behavior. The component supplies a capacity-one channel, leaves it
open until `Execute` returns, and must receive exactly one audit afterward.

The value contains no strings, bytes, IDs, paths, digests, Grant material,
provider/model data, timestamps, usage, or arbitrary runtime content. The
component must assert:

```text
ForwardPartialObserved == true
TextStartSnapshotBytes > AcceptedBytesAtTextStart
AcceptedBytesAtTextStart == 0
AcceptedDeltaCount >= 2
FinalSnapshotBytes == FinalAcceptedDeltaBytes
DeltaClosure == true
```

This sanitized observation plus the exact Frame/Evidence byte-count closure is
the required inspectable proof that the locked Pi itself produced
forward-partial snapshots while only deltas became output authority.

Output-size violations remain `ErrPiRPCOutputTooLarge`. Protocol violations
retain one closed reason triple and
`errors.Is(err, ErrPiRPCProtocol) == true`.

## Locked Pi deterministic loopback component

### Runtime binding

The component test must execute the actually installed, locked Pi `0.82.1`
CLI, not a shell transcript fixture or reimplementation.

Before process start it must verify:

- installed Pi CLI SHA-256
  `af302f231437eaf6f37691bce4b34234fcb626bcb5eb3910d4fc3f6519bf78ca`;
- locked `event-stream.js` SHA-256
  `44a2498660ca61efa952ad6a3f10cc0491883411bd2b4572c9a392ec4e9553ec`;
- locked `openai-completions.js` SHA-256
  `0d50250fe2931e66e2078279a397814202e1ecddee58faf4b8bc04c278da177a`;
- exact Runtime instance ID and version binding;
- current-user ownership, non-group/world-writable resolved executable/source
  files, and private `0700` home/temp/workspace roots; and
- exact retry-disabled, compaction-disabled, offline, no-telemetry Pi settings.

The component authorization environment is exactly:

- `LOOM_PI_0821_COMPONENT=1`;
- `LOOM_PI_0821_EXECUTABLE=<reviewed absolute installed Pi executable>`;
- `LOOM_PI_0821_RUNTIME_SEARCH_PATH=<reviewed absolute locked Pi source root>`;
- `LOOM_PI_0821_RUNTIME_INSTANCE_ID=<reviewed installed Runtime instance ID>`.

When `LOOM_PI_0821_COMPONENT` is absent or not exactly `1`, ordinary repository
test runs may skip this opt-in test. When it is exactly `1`, any missing,
relative, mismatched, unreadable, wrong-owner, writable, wrong-version, or
wrong-hash binding is an immediate test failure; it must never skip, search
ambient `PATH`, substitute a fixture, or weaken validation.

The mandatory opt-in command must use `-v`, and its output must contain exactly
one bounded non-disclosing line:

```text
component_proof locked_pi=true forward_partial=true sse_requests=1 team_terminal=succeeded evidence=1
```

A zero exit without that exact sentinel is not verification evidence.

### Deterministic SSE server

The component owns an in-process HTTP server bound only to an ephemeral
`127.0.0.1` port. It must:

- reject non-loopback peers, wrong method/path/content-type/auth binding, an
  unexpected model, non-streaming requests, tools, extra turns, or an
  unexpected bounded user prompt;
- never contact DNS, the Internet, llama.cpp, a Provider, or the model file;
- return an exact OpenAI-compatible `text/event-stream`;
- emit at least two distinct non-empty content chunks, a separate
  `finish_reason=stop` chunk, exact bounded usage, and `[DONE]`;
- use one stable response ID and the exact requested model ID;
- flush deterministically after every SSE record without a sleep, scheduler
  hook, retry, or consumer acknowledgement between records;
- record only bounded request-shape booleans/counts for assertions, not raw
  prompt, credentials, hidden reasoning, or model output; and
- close cleanly with exactly one chat-completions request.

The real Pi process must transform this unpaced SSE stream into the complete
RPC lifecycle. The test must demonstrate actual forward-partial behavior,
including any prefix-comparable top-level/nested serialization skew, at the
Pi/Bridge boundary; a hand-authored RPC transcript alone is insufficient.

## Supervisor, Grant, Frame, Evidence, and projection closure

The real-Pi deterministic component must run through the accepted local Team
execution path, not call the adapter as the only assertion boundary.

It must prove:

- one Runtime discovery/online binding and capacity `1`;
- one Team plan, logical `main` node, WorkItem, Run, generation `1`, claim, and
  issued AgentGrant;
- every inbound Frame passes Bridge binding/sequence validation and
  `AgentGrant.Authorize`;
- exact Frame order:
  `Ack → one-or-more Event → Evidence → Result`;
- no output Frame is produced by `text_start`; Event payload bytes concatenate
  to the exact ordered SSE delta bytes;
- the Evidence Frame digest and byte count match only that concatenated output;
- the Result Frame is succeeded only after the complete Pi terminal lifecycle;
- tentative output reaches the authorized observer before terminal;
- one immutable artifact, receipt, and capture bind the same Evidence digest;
- Journal contains the exact succeeded Run/WorkItem/Team attempt terminal,
  Grant issue/revocation, capacity reserve/release, and Evidence submission
  lineage without duplicate IDs or stream sequences;
- Projection rebuild reaches succeeded Team terminal state;
- raw Grant, prompt, tentative text, private paths, and credentials do not
  enter Journal or repository evidence;
- no duplicate Run, Evidence, terminal, revocation, or capacity-release fact
  exists; and
- Pi child, HTTP server, listener, workspace, home/temp handles, and database
  handles close without residual process or listener.

The component must also execute a separate real-Pi deterministic observer
failure path with its own private database, Evidence root, loopback listener,
Team/Run/generation/Grant, and exactly one SSE request. The observer rejects
the first authorized output Event with a closed local test error. The path
must produce no successful AdapterResult, revoke the Grant, release capacity,
record exactly one failed Evidence and Team-attempt lineage, capture only the
authorized Frames up to and including the rejected Event, publish no accepted
observer output or later Evidence/Result Frame, and never retry.

## Non-disclosure

Errors, repository governance evidence, Journal facts, and test output must
never contain:

- raw JSON/SSE/RPC records or fragments;
- transcript text, delta text, model output, or hidden reasoning;
- prompt or system prompt;
- raw Grant material, credentials, request authorization values, or
  environment values;
- response IDs, provider/model values, usage values, timestamps, Event IDs,
  Run IDs, or filesystem paths; or
- arbitrary text derived from Pi, the loopback response, llama.cpp, a
  Provider, or a fixture.

Those surfaces may retain only bounded state booleans/counts, closed
diagnostic triples, hashes/digests, modes, ownership, Event types/statuses,
and exact authorization accounting.

Private `0600` attempt Evidence is the sole exception and only for the exact
Bridge-validated, Grant-authorized canonical Frames frozen in the
Implementation Review 2 repair. It must not retain raw RPC/SSE records,
forward snapshots, prompts, Grants, credentials, hidden reasoning,
environment values, private paths, or unauthorized/later Frames. Authorized
Event output in private Evidence remains tentative until terminal
Evidence/receipt/capture acceptance and never becomes Journal or Projection
state authority.

## Historical original RED and mandatory Repair-3 RED

After Contract Reviewer `PASS` and before production/harness changes, tests
must encode all of the following.

### Hermetic adapter RED

1. single-chunk locked-Pi forward snapshot:
   `text_start.partial` already contains the first delta, followed by that
   exact `text_delta`;
2. multi-chunk read-ahead where early partials contain multiple future deltas;
3. prefix-comparable top-level/nested snapshot skew in both directions while
   all non-text fields remain exact;
4. preservation of the existing empty-start transcript;
5. partial shrink, top/nested divergence or non-text mismatch, delta-prefix
   mismatch, unreconstructed bytes,
   reordered/missing delta, oversized partial, invalid UTF-8, and raw Grant
   rejection;
6. zero Frame publication from `text_start`;
7. exact Frame payload/order/count and final digest equivalence between
   immutable and forward-partial forms;
8. nil, full, unbuffered-without-receiver, and closed audit channels cannot
   change success/error, exact Frame/Evidence bytes, publication count, child
   cleanup, or return latency; a capacity-one channel receives exactly one
   by-value audit only after result construction;
9. concurrent executions with distinct open audit channels are race-free and
   cannot cross-deliver observations;
10. complete lifecycle missing/duplicate/out-of-order terminal rejection; and
11. complete existing fails-closed/reason-code/non-disclosure matrices.

### Historical original real-Pi/audit RED

This subsection records the already completed pre-Repair-2 RED. It is
immutable lineage evidence and is not a Repair-3 command or predicate.

The real component RED is performed in two explicit substeps inside this same
contract:

1. before referencing the new audit types, the test executes locked Pi against
   the deterministic SSE fixture through the full vertical path and fails only
   with the existing closed
   `phase=assistant_update event=text_start
   reason=text_content_progression` diagnostic before any output Frame; and
2. the same test is then extended with the frozen sanitized audit assertion
   and must fail to compile only because the audit type/configuration field do
   not yet exist.

The original final RED hashes remain frozen historical evidence. Repair 3
supersedes the gofmt-only rule only for the disclosed fixture corrections and
the exact new assertions in this contract. After fresh Repair-3 Contract
Reviewer `PASS`, the tests must add the new prefix-skew, full vertical closure,
unpaced success, and observer-failure requirements and record a new Repair-3
RED hash set before Production Repair 1. Those repaired tests then remain
frozen except for `gofmt`.

Across those two substeps, the new opt-in component test must:

- verify the locked Pi/source hashes;
- start deterministic loopback SSE;
- enter the full Team/Supervisor/Grant/Frame/Evidence path; and
- prove the baseline diagnostic and the missing audit surface before any
  production edit.

The failure must be due to the missing forward-partial compatibility, not a
bad SSE fixture, missing Pi binding, skipped test, networking error, or
component setup defect.

### Repair-3 RED against the current Candidate

After fresh Repair-3 Contract Reviewer `PASS` and before Production Repair 1,
the repaired tests must be added while the current production Candidate still
requires full top-level/nested semantic equality.

The deterministic Repair-3 production RED is hermetic:

1. a text-start update where `M` is the first-delta prefix and `P` is the
   multi-delta witness;
2. a text-delta update where `M == candidate D` and `P` contains one or more
   future deltas;
3. the symmetric forms with top-level `M` ahead and nested `P` at candidate
   `D`; and
4. non-text-equal but text-divergent controls that remain rejected.

The first three must fail only with the existing closed
`phase=assistant_update ... reason=message_partial_mismatch` diagnostic under
the pre-Repair-3 product. The divergent controls must already pass as
rejections. This proves that the new acceptance predicate, rather than the
existing forward-partial `H/P/D` support, is missing.

The repaired real-Pi success and observer-failure components, full vertical
assertions, and unpaced server are added in the same test-first checkpoint.
They may expose the same closed mismatch in the real process, but the
hermetic prefix-skew failures are the mandatory deterministic product RED;
component scheduling is not used as the sole RED oracle.

The audit type and configuration field already exist in the Repair-3 baseline
and must compile. Repair 3 neither deletes nor re-creates the historical audit
compile RED.

Mandatory Repair-3 RED command:

```text
go test ./internal/runtime/piadapter \
  -run '^TestPiRPCTranscriptClosurePrefixSkew$' \
  -count=1
```

Before Production Repair 1, record:

- the exact Repair-3 test hashes;
- the focused failing command and exact closed reason;
- the fact that `rpc_bridge_adapter.go` is unchanged from the
  Implementation-Review-1 product hash; and
- zero live-canary invocations.

Mandatory locked-Pi usage-skew repair RED command:

```text
go test ./internal/runtime/piadapter \
  -run '^TestPiRPCTranscriptClosureUsageProjectionSkew$' \
  -count=1
```

Before continuing Production Repair 1, record the focused failure, exact
closed reason, current test/product hashes, removal of all temporary
diagnostics, and zero live-canary invocations.

### Historical original live-harness RED

The live harness test must require the independent manifest:

```text
resolved-live-manifest-pi-0821-transcript-closure-canary.json
```

and attempt prefix:

```text
controlled-canary-pi-0821-transcript-closure-
```

It must fail at baseline because the prior rejection-diagnostic name/prefix
remain.

This live-harness RED was completed before the original GREEN and is immutable
historical evidence. The current pre-Repair-3 harness already uses the closure
manifest and prefix. Repair 3 must retain that harness as GREEN regression
coverage; it must not require or synthesize the historical failure again.

Historical original focused RED commands:

```text
go test ./internal/runtime/piadapter \
  -run '^TestPiRPC(TranscriptClosure|TranscriptClosureUsageProjectionSkew|TranscriptClosureRejections|TranscriptClosureFrameAuthority|TranscriptLifecycleClosure)$' \
  -count=1

test -n "$LOOM_PI_0821_EXECUTABLE" &&
test -n "$LOOM_PI_0821_RUNTIME_SEARCH_PATH" &&
test -n "$LOOM_PI_0821_RUNTIME_INSTANCE_ID" &&
LOOM_PI_0821_COMPONENT=1 \
go test -v ./internal/app \
  -run '^TestPi0821DeterministicSSETeamExecution(Closure|FailureClosure)$' \
  -count=1

go test ./internal/app \
  -run '^TestFinalLivePi0821TranscriptClosureCanaryIsolation$' \
  -count=1
```

RED output itself must contain no forbidden material.

## Minimal GREEN

Implementation is limited to:

- one bounded last-partial snapshot field in private per-execution RPC state;
- narrow internal text-snapshot extraction and byte-prefix validation;
- validation-before-mutation for the snapshot and authoritative delta
  accumulator;
- the exact sanitized, non-blocking, panic-safe, non-authoritative transcript
  audit channel frozen above;
- complete lifecycle hermetic guards;
- prefix-comparable top-level/nested witness validation without snapshot
  authority;
- the real locked-Pi deterministic unpaced loopback SSE success and observer
  failure vertical component tests;
- changing only the opt-in live manifest filename and fresh attempt prefix;
  and
- no unrelated refactor.

The component test may use only the Go standard library, existing repository
packages, and the already installed locked Pi binding. It must not add a
dependency, vendored fixture, downloader, model, Provider, credential, daemon,
or network service.

## Verification matrix

Before Implementation Review:

```text
go test ./internal/runtime/piadapter \
  -run '^TestPiRPC(TranscriptClosure|TranscriptClosureRejections|TranscriptClosureFrameAuthority|TranscriptLifecycleClosure)$' \
  -count=1

test -n "$LOOM_PI_0821_EXECUTABLE" &&
test -n "$LOOM_PI_0821_RUNTIME_SEARCH_PATH" &&
test -n "$LOOM_PI_0821_RUNTIME_INSTANCE_ID" &&
LOOM_PI_0821_COMPONENT=1 \
go test -v ./internal/app \
  -run '^TestPi0821DeterministicSSETeamExecution(Closure|FailureClosure)$' \
  -count=1

go test ./internal/app \
  -run '^TestFinalLivePi0821TranscriptClosureCanaryIsolation$' \
  -count=1

go test ./internal/runtime/piadapter ./internal/supervisor \
  ./internal/authorization ./internal/evidence ./internal/app -count=1

go test -race ./internal/runtime/piadapter \
  -run '^TestPiRPC(TranscriptClosure|TranscriptClosureUsageProjectionSkew|TranscriptClosureRejections|TranscriptClosureFrameAuthority|TranscriptLifecycleClosure)$' \
  -count=30

test -n "$LOOM_PI_0821_EXECUTABLE" &&
test -n "$LOOM_PI_0821_RUNTIME_SEARCH_PATH" &&
test -n "$LOOM_PI_0821_RUNTIME_INSTANCE_ID" &&
LOOM_PI_0821_COMPONENT=1 \
go test -v -race ./internal/app \
  -run '^TestPi0821DeterministicSSETeamExecution(Closure|FailureClosure)$' \
  -count=10

go test -race ./internal/app \
  -run '^TestFinalLivePi0821TranscriptClosureCanaryIsolation$' \
  -count=30

go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go mod verify
gofmt -d \
  internal/runtime/piadapter/rpc_bridge_adapter.go \
  internal/runtime/piadapter/rpc_bridge_adapter_test.go \
  internal/app/final_live_gate_pi0821_component_test.go \
  internal/app/final_live_gate_live_test.go
git diff --check
GOOS=windows GOARCH=amd64 go test -exec=true \
  ./internal/runtime/piadapter ./internal/app
```

The component authorization environment may bind the already reviewed private
Pi executable/search path. It must not enable the live model canary, expose
paths in repository evidence, or persist a component attempt outside test
temporary directories.

Scope and preservation checks must prove:

- only the exact writable files and this contract's governance evidence
  changed in the Candidate;
- all read-only authority surfaces remain byte-for-byte unchanged;
- all previously accepted transcripts remain accepted;
- all previously rejected transcripts retain the same primary error identity
  and closed diagnostic;
- forward partials never directly produce Frames or Evidence bytes;
- the real-Pi component executes rather than skips and uses exactly one
  loopback SSE request;
- each mandatory component command prints the exact proof sentinel once and
  the sanitized transcript audit proves text-start read-ahead plus final delta
  closure;
- no dependency, Bridge/Journal/Projection/StateWriter, policy,
  Grant/Evidence authority, Runtime/model/source-lock, credential, `.env`,
  daemon, scheduler, CLI product, or queued roadmap scope changed;
- no llama/model live canary runs during RED, GREEN, or review;
- all historical evidence and private-attempt hashes remain unchanged; and
- no user-owned dirty file is staged or committed.

## Review, commit, and single live gate

1. Fresh Contract Reviewer must return `PASS` before RED.
2. Fresh Implementation Reviewer must inspect:
   - the entire writable diff;
   - genuine hermetic, component, and harness RED;
   - forward-partial multi-chunk/state-mutation correctness;
   - complete Pi RPC lifecycle coverage;
   - proof that the component used the locked real Pi and deterministic
     loopback SSE rather than a transcript fixture;
   - Supervisor/Grant/Frame/Evidence/Journal/Projection closure;
   - cleanup, non-disclosure, scope, and historical preservation; and
   - the complete verification matrix.
3. Any Critical or Important finding blocks commit and live execution.
4. Reviewer findings are repaired only in this same Candidate, within frozen
   ownership and the three-repair ceiling. No new point Amendment is allowed.
5. After Implementation Reviewer `PASS`, stage only owned product/tests and
   this contract's pre-live governance evidence, review the staged diff, and
   create one local atomic commit. No push, merge, rebase, reset, force,
   release, or publication is authorized.
6. Immediately before live execution, revalidate installed Pi/llama/model and
   locked source hashes, private modes/ownership, port/process cleanup,
   historical evidence/attempt hashes, authorization environment, and absence
   of the new manifest/attempt.
7. Create the independent sanitized manifest, then execute the exact opt-in
   live test once.
8. Stop immediately after that invocation. A fresh independent read-only
   result-evidence Reviewer must audit invocation count, complete transcript
   result, Frame/Evidence lineage, modes, digests, cleanup, non-disclosure,
   historical preservation, and verdict.

Authorization accounting is exact: this contract authorizes one live canary
invocation only after Implementation Reviewer `PASS`. The real-Pi deterministic
loopback component is mandatory verification and does not consume the live
model canary authorization.

After the live invocation, remaining live authorization is zero. Any test
failure, timeout, protocol rejection, Runtime/model failure, cleanup defect,
ambiguous result, or success consumes it. There is no retry, fallback,
compaction, Provider/model switch, parser widening, or second invocation.

## Exit state

If the single live canary passes with:

- exactly one fresh isolated attempt;
- complete accepted Pi RPC lifecycle;
- one correlated Ack and at least one authorized output Event;
- exact Evidence/Result Frames;
- succeeded Run/WorkItem/Team attempt lineage;
- Grant revocation and capacity release;
- Projection succeeded terminal;
- cleanup and historical preservation; and
- fresh result-evidence Reviewer `PASS`;

then the Final Live Gate reaches:

```text
READY_FOR_FINAL_USER_SIGNOFF
```

It does not become `COMPLETE` until the user provides final review/sign-off.

If the canary fails, times out, is ambiguous, leaks forbidden data, violates
cleanup/evidence invariants, or lacks Reviewer `PASS`, it consumes the
authorization and stops at:

```text
HUMAN_REQUIRED — NO RETRY
```

No outcome authorizes production activation, resident daemon activation,
autonomous execution, Phase 2, push, merge, release, or publication.

VERDICT: PASS

## Reopen 1 — bounded terminal-budget closure

### Authorization and lineage

On `2026-07-28`, after the first transcript-closure live canary failed closed,
the user authorized all subsequent actions needed to continue without another
per-step confirmation. The Controller interprets that authorization only
inside the existing Phase 1 goal and this unique contract lineage.

This section reopens:

```text
PHASE1-FINAL-LIVE-PI-0821-TRANSCRIPT-CLOSURE-1
```

It does not create a new Amendment, WorkItem, parser exception, Runtime,
Provider, model, authority, queue, scheduler, or product surface. The
historical implementation and live-evidence commits remain immutable:

```text
625a79a1bb04513d1a82565461d146fa34996fe5
143163c4f61481cd9359e44ee52c5347923ff9f4
```

The first canary's authorization remains consumed. Reopen 1 grants one new
isolated canary only after its own Implementation Reviewer `PASS`. A failed
Reopen-1 canary cannot be rerun unchanged. It may start another bounded product
repair in this same contract only after deterministic analysis, new RED,
complete verification, and a fresh Implementation Reviewer `PASS`. The goal's
three-repair ceiling applies to this reopened lineage.

### Observed failure and safety decision

The reviewed first canary failed closed at:

```text
phase=assistant_update event=text_end reason=terminal_stop_reason
```

The bounded diagnostic intentionally did not persist the Runtime's raw
stop-reason value. Reopen 1 therefore does not claim that value as direct
evidence.

Locked Pi `0.82.1` source proves:

- an OpenAI-compatible stream initializes `stopReason` to `stop`;
- the provider's final `finish_reason` updates that field before `text_end`;
- `stop` and `end` map to successful `stop`;
- `length` maps to `length`;
- tool finishes map to `toolUse`; and
- content-filter, network, unknown, aborted, and other error paths do not
  become successful `stop`.

The first genuine locked-Pi RED made the previously bounded hypothesis exact.
Against the unchanged product, Pi sent an output-token field with value `1`;
the deterministic endpoint returned `finish_reason: length`, and Loom rejected
the transcript with the same `terminal_stop_reason`.

Locked Pi `simple-options.js`, SHA-256
`74dfde37adbd00a6af1fd707c1c5c876577793b078da9fbbd6d40bb75bfb4749`,
defines:

```text
CONTEXT_SAFETY_TOKENS = 4096
MIN_MAX_TOKENS = 1
available = model.contextWindow - estimatedContextTokens - 4096
effective = min(model.maxTokens, max(1, available))
```

The current declaration sets `contextWindow: 4096`, so the safety reserve
alone guarantees an effective output budget of `1`, independent of the
prompt's exact token estimate.

An independent read-only GGUF v3 metadata parse of the exact frozen model
proved:

```text
general.architecture = qwen2
qwen2.context_length = 32768
```

Fresh Contract Review 2 found that changing only the Pi declaration would
leave the controlled local llama-server at `--ctx-size 4096`. That mismatch is
not accepted. The correction therefore binds both Pi's declared context window
and the controlled local llama-server context to the exact installed model
metadata. It does not speculate about or enlarge the model's real capability.

The safety decision is fixed:

- `stop` remains the only accepted successful terminal reason;
- `length`, `toolUse`, `error`, `aborted`, missing, empty, or unknown reasons
  remain rejected;
- no stop reason becomes an output Frame, Evidence byte, authority fact, or
  diagnostic value;
- the parser, complete lifecycle, forward-partial rules, usage progression,
  response identity, Grant, Frame, Evidence, Journal, Projection, and
  terminal aggregation remain unchanged; and
- Reopen 1 changes one shared, named local-model context constant from the
  incorrect `4096` to the exact GGUF value `32768`;
- Pi's model declaration and llama-server's `--ctx-size` must both use that
  same constant, so advertised and executed context cannot drift;
- the declared maximum output remains `256`;
- llama-server's `--n-predict` remains `256`;
- Pi's existing `4096` safety reserve remains intact; and
- the existing `16384` assistant-byte cap remains intact.

### Risk, dependencies, and trust boundary

- Risk: `HIGH`
- Baseline:
  `143163c4f61481cd9359e44ee52c5347923ff9f4`
- Depends on:
  - the unique Closure Contract and its Contract/Implementation Reviews
    `PASS`;
  - the first transcript-closure live evidence and result-evidence Review
    `PASS`;
  - exact installed Pi `0.82.1`, llama.cpp, model, and source-lock bindings;
    and
  - user authorization on `2026-07-28` to continue necessary actions without
    another per-step confirmation.

Trust boundary:

- Loom remains the only controller and state authority.
- Pi, llama.cpp, model output, forward partials, stop reasons, and loopback SSE
  remain untrusted observations.
- `AppendBatchIfStreamHeads` and the existing authorities remain the only
  authoritative writers.
- Only Bridge-validated and Grant-authorized Frames may enter private
  attempt Evidence.
- A token budget changes execution bounds only; it grants no permission,
  acceptance, terminal authority, retry, fallback, or completion claim.

### Exact owned files

Reopen 1 may modify only:

1. `internal/runtime/piadapter/rpc_bridge_adapter.go`
2. `internal/runtime/piadapter/rpc_bridge_adapter_test.go`
3. `internal/runtime/piadapter/local_model_server.go`
4. `internal/runtime/piadapter/local_model_server_test.go`
5. `internal/app/final_live_gate_pi0821_component_test.go`
6. `internal/app/final_live_gate_live_test.go`
7. this unique contract
8. `pi-0821-transcript-compatibility-closure-contract-review.md`
9. `pi-0821-transcript-compatibility-closure-red.md`
10. `pi-0821-transcript-compatibility-closure-verification.md`
11. `pi-0821-transcript-compatibility-closure-implementation-review.md`

Only after an authorized invocation may it add:

12. `resolved-live-manifest-pi-0821-transcript-closure-reopen1-canary.json`
13. `pi-0821-transcript-closure-reopen1-live-canary.md`

The private materialization status may receive bounded result synchronization
only. All other repository and private files remain read-only.

### Reopen-time quarantine

The following now-historical closure evidence is read-only:

| Path | SHA-256 |
|---|---|
| `pi-0821-transcript-closure-live-canary.md` | `b09c365dcd761715425a98614da76aef3d707310c7bcbe6e9d2ca3dd4b91a61a` |
| `resolved-live-manifest-pi-0821-transcript-closure-canary.json` | `4dfb081363b910b2fdf041c2636ba2be7b9876ab92d103eb30a76362087e6e35` |

The retained private attempt
`controlled-canary-pi-0821-transcript-closure-2044445093` is read-only, with:

```text
canary.sqlite
204f4f6c23e093b8aa8bc5bfc18be4ac4fcfeeff3199a83899b9b4575df92e68

bounded source
bfc5aca5df67c1244e4d28031f973404e5a118d5587288609a625906d3d1a54f

failure artifact
5fd50fba14bf17ff8bafc55c10e5883b778e09b4f7f907e51fb92303f1bbefe8

Evidence capture
a4c26a54fdbe29374cc94e952da8412a7bf2fa872dda4fb64df06c12908d51d2

Evidence receipt
aab9365dab94fcd413500ae9a197d3eac3c92cb310dd84f9341baf44dbfc0e25
```

The original dirty-worktree quarantine, all earlier manifests/evidence, four
earlier private attempts, installed Runtime/model/source bindings, and
user-owned untracked files retain their frozen status and hashes.

Before commit and before any live invocation, the Controller and Reviewer must
revalidate both the original quarantine and this Reopen-time quarantine.

### Mandatory TDD acceptance

RED must precede product modification and prove all of:

1. The Pi model declaration exposes exactly `contextWindow: 32768` and
   `maxTokens: 256`; current `4096/256` fails.
2. The controlled local llama-server argument vector exposes exactly
   `--ctx-size 32768` and `--n-predict 256`; current `4096/256` fails.
3. Both values derive from the same named package constants. A future drift
   between Pi's declaration and llama-server execution must fail a focused
   test.
4. The locked real Pi component binds all three causal source files before
   process start:
   - `event-stream.js` SHA-256
     `44a2498660ca61efa952ad6a3f10cc0491883411bd2b4572c9a392ec4e9553ec`;
   - `openai-completions.js` SHA-256
     `0d50250fe2931e66e2078279a397814202e1ecddee58faf4b8bc04c278da177a`;
   - `simple-options.js` SHA-256
     `74dfde37adbd00a6af1fd707c1c5c876577793b078da9fbbd6d40bb75bfb4749`.
5. The opt-in pre-live path repeats those same three file, digest, mode, and
   current-user ownership bindings before creating its manifest or starting
   Pi/llama/model execution. The quarantined `source-lock.json` remains
   untouched; this unique contract and the executable tests are the binding
   authority for Reopen 1.
6. The locked real Pi component sends the declared output budget to the
   deterministic loopback OpenAI-compatible endpoint.
7. The loopback returns a successful text lifecycle with `finish_reason:
   length` when the request budget is below `256`, and `finish_reason: stop`
   only when it is exactly the bounded `256`.
8. Against the current product, the locked-Pi component fails closed with the
   same bounded `terminal_stop_reason`; after the minimal budget change it
   completes the full Pi RPC lifecycle.
9. Existing rejection coverage still proves `length`, `toolUse`, errors,
   missing/unknown stop reasons, terminal skew, and duplicate/out-of-order
   lifecycle events cannot produce a successful `AdapterResult`.
10. Success still closes Supervisor → Grant → Ack/Event/Evidence/Result →
   Journal → Projection; observer failure still records only the authorized
   prefix and failed terminal lineage.
11. The opt-in live harness uses a new independent manifest and fresh attempt
   prefix and cannot alias any historical canary.

The deterministic loopback may inspect only the bounded request schema needed
to prove model ID, one user turn, streaming mode, no tools, and the numeric
output-token field. It must not persist prompt/model output, contact DNS or
the Internet, or run llama.cpp/the model.

### Minimal GREEN

The only intended product change is one named GGUF-bound context constant and
one named output constant, shared by `modelsJSON` and
`piLocalModelArguments`:

```text
Pi declared context window: 4096 → 32768 tokens
llama-server --ctx-size: 4096 → 32768 tokens
Pi declared maximum output: unchanged at 256 tokens
llama-server --n-predict: unchanged at 256 tokens
```

No parser acceptance predicate may change. Any need to modify
`piRPCAssistantRejectionReason`, `piRPCAssistantText`,
`acceptAssistantEvent`, lifecycle state, Frame publication, Evidence,
Supervisor, Grant, Journal, Projection, output verification, or authority is
outside Reopen 1 and must stop for contract review inside this same unique
lineage.

### Required verification

At minimum:

```text
go test ./internal/runtime/piadapter \
  -run '^(TestPiRPC(ModelOutputBudget|TranscriptClosure|TranscriptClosureRejections|TranscriptLifecycleClosure)|TestPiLocalModelContextAlignment)$' \
  -count=1

test -n "$LOOM_PI_0821_EXECUTABLE" &&
test -n "$LOOM_PI_0821_RUNTIME_SEARCH_PATH" &&
test -n "$LOOM_PI_0821_RUNTIME_INSTANCE_ID" &&
LOOM_PI_0821_COMPONENT=1 \
go test -v ./internal/app \
  -run '^TestPi0821DeterministicSSETeamExecution(Closure|FailureClosure)$' \
  -count=1

go test ./internal/runtime/piadapter ./internal/supervisor \
  ./internal/authorization ./internal/evidence ./internal/app -count=1

go test -race ./internal/runtime/piadapter \
  -run '^(TestPiRPC(ModelOutputBudget|TranscriptClosure|TranscriptClosureRejections|TranscriptLifecycleClosure)|TestPiLocalModelContextAlignment)$' \
  -count=30

LOOM_PI_0821_COMPONENT=1 \
go test -v -race ./internal/app \
  -run '^TestPi0821DeterministicSSETeamExecution(Closure|FailureClosure)$' \
  -count=10

go test -race ./internal/app \
  -run '^TestFinalLivePi0821TranscriptClosureCanaryIsolation$' \
  -count=30

go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go mod verify
gofmt -d \
  internal/runtime/piadapter/rpc_bridge_adapter.go \
  internal/runtime/piadapter/rpc_bridge_adapter_test.go \
  internal/runtime/piadapter/local_model_server.go \
  internal/runtime/piadapter/local_model_server_test.go \
  internal/app/final_live_gate_pi0821_component_test.go \
  internal/app/final_live_gate_live_test.go
git diff --check
GOOS=windows GOARCH=amd64 go test -exec=true \
  ./internal/runtime/piadapter ./internal/app
```

The component commands must bind the already reviewed installed Pi executable,
search path, and Runtime instance ID; must execute rather than skip; and must
print the exact success sentinel once per successful run. No llama/model live
execution occurs before Implementation Reviewer `PASS`.

### Review, commit, and live accounting

1. A fresh Contract Reviewer must return `PASS` before RED.
2. A fresh Implementation Reviewer must verify genuine RED against both
   Pi-declared and llama-executed `contextWindow/maxTokens: 4096/256`, minimal
   GREEN with both context surfaces aligned at `32768` and both output
   surfaces unchanged at `256`, exact GGUF and three-file locked-source proof,
   unchanged parser/rejection set, locked-Pi deterministic component closure,
   complete verification, scope, quarantine, and non-disclosure.
3. Only after Reviewer `PASS` may the exact owned Candidate be committed
   locally.
4. Immediately before live, all installed/historical bindings, modes, owners,
   hashes, port/process cleanup, authorization environment, and absence of the
   Reopen-1 manifest/attempt must revalidate.
5. One Reopen-1 canary is then authorized. It consumes its allowance on any
   result. There is no unchanged rerun.
6. A fresh read-only result-evidence Reviewer must audit the result before any
   next action.

If the canary passes, the Final Live Gate reaches
`READY_FOR_FINAL_USER_SIGNOFF`. If it fails, the Controller may continue only
through a new bounded repair inside this same unique contract, subject to the
three-repair ceiling and all RED/Review gates. No point Amendment is allowed.

No outcome authorizes push, merge, release, publication, production or
resident daemon activation, credentials, Phase 2, or user sign-off.

VERDICT: PASS
