# S2-W27 Candidate Deliverable

- WorkItem: `S2-W27`
- Title: One-Shot Configured Runtime Discovery Commit Coordination
- Risk: Strict
- Candidate state: `ACCEPTED`
- Branch/head: `codex/loom-platform-slice2` at `494579d`
- Contract SHA-256:
  `7235cb0abf89842fcd3498aebeeb494231e3b01cc02a5bb97f67de2ae9b9b764`
- Contract review SHA-256:
  `f8e4da9f7695395e9b4030db7eddc34a4a27bb509473af83777c3d3d6ffc1641`

## Contract and review

The frozen contract adds one explicit application command between accepted
S2-W26 configured discovery and an injected accepted S2-W20 commit boundary.
Fresh independent contract review returned `PASS` with no blocking findings
before any product/test change.

It does not allocate Event metadata, call Journal/SQLite directly, query or
update projection, reconcile status, choose status-versus-rediscovery policy,
schedule/retry a scan, start a daemon, or activate a Runtime.

## Mandatory RED

Before `runtime_discovery.go` existed, the focused command exited `1` only
because the frozen app function, committer interface, and error symbols did not
exist. No syntax, existing package, dependency, SQLite, or environment failure
occurred.

## Candidate

The Candidate:

- rejects nil context and nil/typed-nil committer before every factory call;
- delegates factory/probe validation and one-shot observation once to accepted
  S2-W26;
- returns an accepted empty/all-absent snapshot with a zero commit Candidate
  and no committer call;
- calls the injected committer exactly once for a non-empty snapshot;
- returns no usable snapshot or commit Candidate after any discovery,
  committer, context, or result-validation failure;
- accepts only a committed private S2-W20 Candidate whose source digest, Event
  count, Event accessor length, and commit digest match the observed snapshot;
  and
- returns the exact accepted immutable snapshot and commit Candidate without
  retrying.

A deterministic test committer binds the port to accepted S2-W20 and a real
temporary SQLite Journal. The first invocation appends one canonical Event; an
explicit second invocation with identical observation/metadata returns the
same digests and leaves the Journal at one row.

## Candidate file digests

```text
be7521476312f2cbb64c270838d0133bc3c5e5459a8799e2afb520012c40ec4a  internal/app/runtime_discovery.go
473db85b5135eddba6af4226bcc726c40c5f496bf9b548083446d179226e04d3  internal/app/runtime_discovery_test.go
```

## Controller verification

| Check | Result |
|---|---|
| focused S2-W27 | `PASS` |
| app package | `PASS` |
| app/runtime/discoveryscan/state/journal impact | `PASS` |
| focused race `-count=50` | `PASS` |
| `go test ./... -count=1` | `PASS`, all packages green |
| `go test -race ./... -count=1` | `PASS`, all packages green |
| `go vet ./...` | `PASS` |
| frozen-file `gofmt -d` | `PASS`, no output |
| `git diff --check` | `PASS`, no output |
| import/static/scope boundary | `PASS` |
| branch/head | `PASS`, `codex/loom-platform-slice2` at `494579d` |

Focused evidence covers:

- invalid context/committer prevalidation before factories;
- empty/all-absent success with no commit;
- exact factory-build, probe-observe, and commit order;
- exact accepted snapshot/Candidate result and accessor isolation;
- factory/probe/context failures with zero commit calls;
- committer/source/cancellation/zero/wrong-source/wrong-count failures with one
  call, no retry, and zero outputs;
- complete public Candidate predicate checks; and
- real SQLite first append plus accepted exact retry idempotency.

## Trust and residual state

S2-W26 remains configured observation authority. S2-W20 remains canonical Event
construction/append and commit-Candidate authority. The app coordinator mints
neither snapshot nor commit state and imports no concrete Journal, SQLite, Pi
adapter, projection, scheduler, daemon, config, or credential package.

No installed Pi executable, user Pi state, credential, network, package
manager, production process, projection mutation, scheduler, daemon, Agent
session, prompt, model call, Runtime selection/reservation/activation, push,
merge, rebase, reset, release, or external mutation was used.

## Review gate

Fresh independent implementation review independently passed the complete
strict matrix, confirmed every result/error/authority boundary, and returned
`PASS` with no findings:
`.loom-evidence/phase1-slice2/S2-W27/implementation-review.md`.

VERDICT: PASS
