# S3-W3 Repair 1 Contract — Review 1 Closure

- Baseline: `5517a06`
- Parent contract SHA256:
  `73ee0a5892a4f4a0fcbb3a595d0c7948e82d3212612b34fde95f1e7232207434`
- Parent amendment SHA256:
  `48e7d166e4e32e4265bc33d7e79bd5d030f4642abe653f4964ed93a888a9ac0c`
- Trigger: S3-W3 Implementation Review 1 `FAIL`
- Date: `2026-07-26`

## Scope

Repair exactly the four accepted Review 1 findings. Public API, Event schemas,
operation allowlist, owned product files, Slice 3 capability, and exclusion
boundaries do not change.

## Required behavior

### 1. Opaque-ID compatibility

Every S3-W3 opaque ID consumed from accepted S3-W2 Run state must use the exact
S3-W2 language:

- non-empty and at most 128 bytes;
- valid UTF-8;
- already trimmed by `strings.TrimSpace`;
- no Unicode control characters.

Spaces inside the value, `@`, and non-ASCII valid UTF-8 are legal. UUID fields
remain exact canonical UUID-v4 values.

### 2. Operational historical Run-reference validation

Before exposing any rebuilt Grant state, `Authority.readState` must:

1. require accepted `work.Authority.Snapshot(ctx)` replay to succeed;
2. index every persisted `run/` Event by exact Event ID, stream, sequence, and
   emitted time;
3. reject duplicate/conflicting historical Event identities;
4. require each AgentGrant issue/authorize/revoke payload to reference an
   existing exact Run Event with matching stream and sequence;
5. require the Grant Event time not to precede its referenced Run Event.

Malformed references fail closed without replacing or exposing a successful
snapshot. This repair adds no second state authority; the index is
stack-local rebuild validation over the same Journal source.

### 3. Conflict normalization

All Grant mutations must map these Journal CAS/write collisions to
`ErrGrantAuthorityConflict`:

- `journal.ErrStreamHeadConflict`;
- `journal.ErrSequenceConflict`;
- `journal.ErrIdempotencyConflict`;
- `journal.ErrPartialEventBatchConflict`.

The public error may wrap the Journal cause but must remain detectable with
`errors.Is(err, ErrGrantAuthorityConflict)`. In particular, concurrent reuse
of one per-Run RequestID with different authorization semantics must never
expose a raw Journal conflict.

### 4. Exact JSON object decoding

Both `decodeExactGrantPayload` and shared `decodeExactProjectionPayload` must
reject:

- invalid JSON;
- any duplicate object member name at any nesting depth;
- unknown or missing frozen fields through existing typed validation;
- multiple top-level JSON values or trailing non-whitespace content.

Duplicate members are invalid even when their values are identical.

## Mandatory Repair RED

Tests must fail on the current Candidate for exactly these gaps:

1. a real accepted S3-W2 Run with legal spaces, `@`, and non-ASCII opaque IDs
   can be issued and authorized;
2. `Authority.Snapshot` rejects a Grant fact whose Run Event ID, stream, or
   sequence does not identify the exact historical Run Event;
3. every listed Journal collision maps to `ErrGrantAuthorityConflict`, with a
   concrete conflicting-RequestID authorization race;
4. Grant authority and projection rebuild both reject identical-value
   duplicate JSON keys, including a nested duplicate.

Each Repair marker must occur exactly once:

```text
s3_w3_repair_opaque_id_compatibility
s3_w3_repair_historical_run_reference
s3_w3_repair_conflict_normalization
s3_w3_repair_duplicate_json_keys
```

## Verification

After GREEN:

```text
go test ./internal/authorization ./internal/projection -count=1
go test -race ./internal/authorization ./internal/projection -count=30
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go test ./internal/authorization -run '^$' \
  -fuzz '^FuzzGrantTokenAndReplayNeverPanic$' -fuzztime=5s
gofmt -d <all owned Go files>
git diff --check
```

Fresh independent Implementation Review 2 is required before acceptance or
commit.

VERDICT: FROZEN
