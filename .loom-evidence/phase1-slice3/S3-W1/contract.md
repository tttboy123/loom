# S3-W1 Contract — Bridge v1 Wire and Bound Run Stream

- WorkItem: `S3-W1`
- Risk: `STRICT`
- Frozen branch: `codex/loom-platform-slice2`
- Frozen baseline: `7b1726e`
- Date: `2026-07-25`
- Depends on: accepted Slice 2 at Review 2 and reviewed Slice 3 Exit Contract
- Corresponds to: `TECH-PLAN.md` section 7 and Slice 3 Bridge trust boundary

## Purpose

Create the complete pure trust boundary for untrusted
`loom.bridge.v1` JSON Lines frames and immutable per-Run stream prefixes. This
is one protocol-security Candidate, not a struct-only wrapper.

## Owned files

Developer:

- `protocol/bridge/v1/frame.go`
- `protocol/bridge/v1/frame_test.go`

Controller:

- `.loom-evidence/phase1-slice3/S3-W1/**`
- `docs/CURRENT.md`
- Controller-owned `PROGRESS.md` hunks

All existing product/test files, `TECH-PLAN.md`, ADRs, migrations, modules,
policies, credentials, user-owned dirty files, `.codex/**`, and
`.loom-drafts/**` are locked.

## Frozen public surface

Package: `bridgev1`

```go
const ProtocolVersion = "loom.bridge.v1"

const (
    MaxLineBytes      = 1 << 20
    MaxPayloadBytes   = 768 << 10
    MaxOpaqueIDBytes  = 128
    MaxBufferedFrames = 1024
    MaxBufferedBytes  = 8 << 20
)

type MessageType string

const (
    MessageDispatch  MessageType = "dispatch"
    MessageEvent     MessageType = "event"
    MessageEvidence  MessageType = "evidence"
    MessageResult    MessageType = "result"
    MessageCancel    MessageType = "cancel"
    MessageAck       MessageType = "ack"
    MessageHeartbeat MessageType = "heartbeat"
)

type FrameInput struct {
    MessageID              string
    CorrelationID          string
    WorkItemID             string
    RunID                  string
    ClaimGeneration        int64
    RuntimeInstanceID      string
    SenderAgentInstanceID  string
    Sequence               int64
    Type                   MessageType
    EmittedAt              time.Time
    Payload                []byte
}

type Frame

func NewFrame(FrameInput) (Frame, error)
func DecodeLine([]byte) (Frame, error)
func EncodeLine(Frame) ([]byte, error)

func (Frame) ProtocolVersion() string
func (Frame) MessageID() string
func (Frame) CorrelationID() string
func (Frame) WorkItemID() string
func (Frame) RunID() string
func (Frame) ClaimGeneration() int64
func (Frame) RuntimeInstanceID() string
func (Frame) SenderAgentInstanceID() string
func (Frame) Sequence() int64
func (Frame) Type() MessageType
func (Frame) EmittedAt() time.Time
func (Frame) Payload() []byte

type RunStreamBinding struct {
    WorkItemID             string
    RunID                  string
    ClaimGeneration        int64
    RuntimeInstanceID      string
    SenderAgentInstanceID  string
}

type BoundRunStream

func NewBoundRunStream(RunStreamBinding) (BoundRunStream, error)
func AdvanceBoundRunStream(BoundRunStream, Frame) (BoundRunStream, error)

func (BoundRunStream) Binding() RunStreamBinding
func (BoundRunStream) Frames() []Frame
func (BoundRunStream) LastSequence() int64
func (BoundRunStream) TerminalResultSeen() bool
func (BoundRunStream) BufferedBytes() int
```

Frozen error sentinels:

```go
ErrInvalidBridgeV1Frame
ErrInvalidBridgeV1Line
ErrBridgeV1LineTooLarge
ErrUnsupportedBridgeProtocol
ErrUnknownBridgeMessageType
ErrDuplicateBridgeJSONKey
ErrInvalidBridgeV1RunStream
ErrBridgeV1RunStreamBufferLimit
```

No other exported symbol is permitted.

## Frame contract

`NewFrame` and `DecodeLine` produce the same immutable validated value.

1. Wire input is exactly one UTF-8 JSON object followed by exactly one `LF`.
   Empty input, missing `LF`, `CRLF`, leading/trailing bytes, embedded literal
   newlines, multiple values, and invalid UTF-8 fail closed.
2. Top-level fields are exactly the eleven fields in `TECH-PLAN.md`:
   `protocol_version`, `message_id`, `correlation_id`, `work_item_id`,
   `run_id`, `claim_generation`, `runtime_instance_id`,
   `sender_agent_instance_id`, `seq`, `type`, `emitted_at`, plus `payload`.
   Missing, unknown, or duplicate fields reject.
3. Duplicate JSON keys reject recursively, including inside `payload`.
4. `protocol_version` is exactly `loom.bridge.v1`.
5. Message and correlation IDs are canonical lowercase 36-character UUID
   text. Other IDs are non-empty bounded UTF-8 opaque identifiers with no
   control characters or leading/trailing whitespace.
6. Claim generation and sequence are positive.
7. Type is exactly one of the seven accepted values.
8. `emitted_at` is non-zero UTC with a `Z` RFC3339Nano representation.
9. Payload is a non-null JSON object, is recursively duplicate-free, and stays
   within its exact byte bound. Payload semantics remain opaque; this WorkItem
   does not invent ACK or domain payload schemas absent from `TECH-PLAN.md`.
10. Input slices, returned payloads, and encoded lines are mutation-isolated.
11. `EncodeLine` emits one deterministic field order and one terminal `LF`,
    revalidates zero/forged values, and never exceeds `MaxLineBytes`.

Line size includes the terminal `LF`. Payload size is measured on the compact
validated JSON object stored by the Frame.

## Bound Run stream contract

`BoundRunStream` is an immutable validated prefix, not execution authority.

1. Binding values obey the same ID/generation validation as Frame.
2. Every appended Frame exactly matches all five binding fields.
3. Message IDs are unique within the prefix.
4. Sequences are strictly increasing; gaps are permitted because the accepted
   plan requires increasing, not contiguous, sequence values.
5. At most one `result` frame is accepted in a prefix.
6. Frame count and sum of encoded line bytes are bounded by the frozen limits.
7. Failed advance returns a zero new Candidate and leaves the prior Candidate
   observably unchanged.
8. Accessors deeply copy frames and payloads and preserve append order.

ACK obligation tracking, process direction, heartbeat supervision, terminal
closure, Run authority, and retry behavior require the later real Bridge
session/Runtime Adapter boundary. S3-W1 must not invent payload semantics to
implement them early.

## Mandatory RED

Before `frame.go` exists:

1. add a test-only guard that constructs and searches for these exact markers
   without spelling them contiguously in the guard;
2. run it and capture failure solely because all seven markers are absent;
3. add the complete behavioral tests before product code and capture a focused
   compile failure only on missing frozen S3-W1 symbols.

Markers:

```text
s3_w1_frame_exact_round_trip
s3_w1_frame_rejection_matrix
s3_w1_json_duplicate_key_rejection
s3_w1_frame_bounds
s3_w1_stream_binding_sequence_uniqueness
s3_w1_stream_terminal_and_buffer_bounds
s3_w1_mutation_concurrency_fuzz_static
```

## Required proof

1. Exact round-trip for all seven types and both constructor/wire paths.
2. Complete missing/unknown/type/ID/generation/sequence/time/payload/line
   rejection matrix with exact sentinel classification.
3. Recursive duplicate-key rejection, including escaped-equivalent keys.
4. Boundary-minus-one, exact-boundary, and boundary-plus-one line/payload/
   identifier/frame-count/buffer-byte tests.
5. Bound-stream mismatch, duplicate message, increasing/non-contiguous
   sequence, terminal-once, failed-advance isolation, and append-order tests.
6. Input/accessor/stream mutation isolation and concurrent read proof.
7. Fuzz seeds for malformed, truncated, nested, oversized, and valid lines;
   decoder never panics.
8. Static AST/import proof: standard library only; no I/O process, Journal,
   state, projection, Run, Grant, credential, Provider/model, filesystem,
   network, goroutine, or Slice 4 authority.

## Verification

```text
go test ./protocol/bridge/v1 -count=1
go test -race ./protocol/bridge/v1 -count=100
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go test ./protocol/bridge/v1 \
  -run '^$' -fuzz '^FuzzDecodeLineNeverPanics$' -fuzztime=5s
gofmt -d protocol/bridge/v1/frame.go protocol/bridge/v1/frame_test.go
git diff --check
```

No dependency audit is required unless a dependency is added, which this
contract forbids.

## Trust boundary and exclusions

This WorkItem parses and validates untrusted bytes into immutable protocol
Candidates only. It creates no process, Run, WorkItem, Event, Grant, workspace,
file, socket, goroutine, or side effect.

It does not authorize payload schemas, ACK session semantics, process
supervision, Runtime activation, Agent/model execution, credentials, Provider
calls, persistent state, Slice 4 rules/approval/verification, or Phase 2.

Fresh independent Contract Review must return `PASS` before mandatory RED.
Fresh independent Implementation Review must return `PASS` before acceptance
or local commit.

VERDICT: PASS
