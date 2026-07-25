# S3-W1 Deliverable — Bridge v1 Wire and Bound Run Stream

- Baseline: `7b1726e`
- Date: `2026-07-25`
- Product scope:
  `protocol/bridge/v1/frame.go`,
  `protocol/bridge/v1/frame_test.go`

## Delivered boundary

- Exact twelve-field `loom.bridge.v1` JSON Lines parsing and deterministic
  encoding.
- Recursive duplicate-key rejection, including escaped-equivalent keys.
- Canonical UUID, opaque identifier, generation, sequence, type, UTC timestamp,
  payload, line, and buffer validation.
- Mutation-isolated Frame values and immutable bound per-Run stream prefixes.
- Exact binding, unique-message, increasing-sequence, one-result, frame-count,
  and byte-count enforcement.
- Standard-library-only pure validation; no I/O, persistence, process,
  goroutine, Run, Grant, model, credential, or activation authority.

## TDD evidence

- Marker-only RED:
  `.loom-evidence/phase1-slice3/S3-W1/red-marker-guard.md`
- Complete behavioral compile RED:
  `.loom-evidence/phase1-slice3/S3-W1/red-behavior.md`

## Verification

- `go test ./protocol/bridge/v1 -count=1`: PASS
- `go test -race ./protocol/bridge/v1 -count=100`: PASS
  (`455.451s`)
- `go test ./... -count=1`: PASS
- `go test -race ./... -count=1`: PASS
- `go vet ./...`: PASS
- `go test ./protocol/bridge/v1 -run '^$' -fuzz
  '^FuzzDecodeLineNeverPanics$' -fuzztime=5s`: PASS
  (`119418` executions, no panic)
- `gofmt -d protocol/bridge/v1/frame.go
  protocol/bridge/v1/frame_test.go`: clean
- `git diff --check`: PASS
- `go doc ./protocol/bridge/v1`: frozen exports only

The first overlong race attempt was interrupted without a failure after
identifying repeated deep copies of historical payload bytes. The
implementation was reduced without changing semantics to copy immutable Frame
values internally while retaining cloning at construction, append, and every
public accessor boundary. The complete required race command was then rerun
from the beginning and passed.

## Review

Fresh independent Implementation Review 1 returned `PASS` with no findings
after independent focused, race, full-repository, repository-race, vet, fuzz,
format, scope, and export checks.

VERDICT: PASS
