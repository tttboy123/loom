# S2-W23 Candidate Deliverable

- WorkItem: `S2-W23`
- Title: Runtime Status Event Writer
- Risk: Strict
- Candidate state: `ACCEPTED`
- Branch/head: `codex/loom-platform-slice2` at `b0cf75f`
- Repaired contract SHA-256:
  `2a5cb17ad5b4a4e6353c2fffabb5107c6e8b274b34c27d64ab2bdbdd9df1613a`
- Contract Repair 1 SHA-256:
  `8fe530cad6d866b3be50d8f80d2a905960aa0720552aafb174ae242ef4fda1c5`

## Candidate

The Candidate adds one authoritative StateWriter for non-empty accepted S2-W22
status transitions. It revalidates the immutable reconciliation facts and
version-1 digest, requires exact caller metadata coverage, builds canonical
`RuntimeInstanceStatusChanged` Events in transition order, and appends one
deep-copied atomic batch through the accepted EventBatchAppender port.

Each sequence must equal exactly the transition's previous sequence plus one.
Lower, equal, or higher values fail before append. Event payloads bind the
complete reconciliation, baseline, discovery, source-probe, stable-identity,
previous-fact, and status-change facts without inventory or local execution
details.

Exact immutable appender results return a copied, digest-bound commit Candidate.
Empty/no-change reconciliations, invalid sources/inputs, appender errors, and
altered results return zero commit Candidates.

## Contract review and repair

Fresh Contract Review 1 returned `REPAIR` because the original
greater-than-previous sequence rule could append a stream gap that accepted
projection replay cannot rebuild and could admit a repeated stale fact.

Contract Repair 1 requires exact `PreviousSequence + 1`, limits Journal
authority to idempotency and occupied exact-next conflicts, and adds real-SQLite
proof for stale/repeated rejection. Fresh independent repaired-contract review
returned `PASS` with no blocking findings.

## Mandatory RED

The focused command exited `1` only because the frozen writer, input, commit
Candidate, and sentinel symbols did not exist. No product file existed; no
syntax, dependency, environment, or unrelated failure occurred.

## Final file digests

```text
ed7f217f32e5e6e76bcff0a3b3bbe4314c117970f0d34745588280346b005659  internal/state/runtime_status_writer.go
b29f878ad3e939cc17755aa29ea40777719dea07ef894eab7ae6858a565a7a4a  internal/state/runtime_status_writer_test.go
```

## Controller verification

| Check | Result |
|---|---|
| focused S2-W23 test | `PASS` |
| state/runtime/journal/projection impact | `PASS` |
| focused race `-count=30` | `PASS` |
| `go test ./... -count=1` | `PASS`, all packages green |
| `go test -race ./... -count=1` | `PASS`, all packages green |
| `go vet ./...` | `PASS` |
| changed-file `gofmt` | `PASS`, no output |
| `git diff --check` | `PASS`, no output |
| import/non-disclosure/scope checks | `PASS` |
| branch/head | `PASS`, `codex/loom-platform-slice2` at `b0cf75f` |

The first full-repository race invocation encountered only the existing Pi
process-group cleanup timing test with a child marker still present. Its focused
race test then passed `-count=10`, and the full repository race passed on the
sequential retry. No Pi adapter or other out-of-scope file was changed.

Focused evidence covers:

- exact one/multi-transition Event envelope, byte-exact payload, ordering,
  normalized UTC time, and source/Causation binding;
- exact-next sequence acceptance plus lower, equal, higher, zero, duplicate,
  missing, extra, and metadata-bijection rejection before append;
- nil/typed-nil appender, nil/canceled/deadline context, zero/empty/oversized
  source or input, and appender failure;
- nil/short/long/reordered and every immutable envelope/payload appender-result
  mismatch, including equal-instant non-UTC time;
- source/input/appender/Candidate/accessor/payload mutation isolation;
- source Candidate digest revalidation, commit digest stability, complete
  source/Event sensitivity, count mismatch, and forged-digest rejection;
- real Journal atomic first append, exact retry, idempotency conflict, occupied
  exact-next stale/repeated sequence conflict, partial-batch conflict, and
  unchanged row counts on failure; and
- static import, no-discovery/no-reconciliation/no-projection/no-execution,
  non-disclosure, and scope checks.

## Trust and residual state

S2-W22 remains the only reconciliation authority. Caller metadata remains
authoritative for IDs and exact-next sequences; the writer does not allocate
metadata or read the stream head. Accepted Journal remains append/idempotency/
occupied-sequence authority. Projection handling remains a separate WorkItem.

No discovery or reconciliation invocation, absence inference, discovery Event,
projection/schema/config/file write, Pi/S2-W18/S2-W19 process, installed Pi,
user Pi state, credential, network, package manager, scheduler, daemon, Agent
session, prompt, model call, Runtime selection/reservation/activation, push,
merge, release, or external mutation was used.

## Review result

Fresh independent implementation review independently reran the full strict
matrix, verified repaired-contract compliance, exact-next and projection-rebuild
safety, source and commit digests, Event/payload/Causation completeness, exact
appender results, Journal conflict behavior, immutability, non-disclosure, and
scope. It returned `PASS` with no blocking findings:
`.loom-evidence/phase1-slice2/S2-W23/implementation-review.md`.

VERDICT: PASS
