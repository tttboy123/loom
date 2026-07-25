# S2-W29 Candidate Deliverable

- WorkItem: `S2-W29`
- Title: One-Shot Observed Runtime Status Commit Coordination
- Risk: Strict
- Candidate state: `ACCEPTED`
- Branch/head: `codex/loom-platform-slice2` at `9296832`
- Contract SHA-256:
  `9a56217e50d0e1a93e37b33bd043c1dbd8c6fa11b762b66000264c71fab97dbe`
- Contract review SHA-256:
  `d9a8ffa3edf227b8549229a31f5f66229d57744a0149109051e66107b079540d`

## Contract and mandatory RED

Fresh independent contract review returned `PASS` with no findings before
product/test changes. The mandatory focused test-only RED exited `1` only
because the frozen errors, committer port, and coordinator symbols did not
exist. There was no syntax, dependency, or environment failure.

## Candidate

The Candidate:

- validates nil/typed-nil committer and context before reconciliation;
- calls accepted S2-W22 reconciliation exactly once;
- returns an exact valid zero-transition reconciliation without committing;
- delegates one non-empty immutable reconciliation exactly once;
- accepts only an exact S2-W23 commit result bound to reconciliation, baseline,
  discovery, Event count, accessor count, and a lowercase SHA-256 digest;
- returns zero Candidates for every error or result mismatch; and
- neither retries nor retains or mutates any source or Candidate.

The product imports only `context`, `errors`, `strings`, `internal/runtime`,
and `internal/state`. It does not build projection baselines, prepare Event
metadata, access Journal/SQLite/projection directly, run discovery, invent
status policy, schedule, start a daemon, or activate a Runtime.

## Candidate file digests

```text
23caf1af3147e4d87cfebdf1bab9cf5e7791c8c8e6f2427462baf4509d36e92c  internal/app/runtime_status.go
fd51e606f8ad147587f0a8592fa864aca1f093f241027574ea1b9f61c1ac93c1  internal/app/runtime_status_test.go
```

## Implementation Repair 1

Fresh Implementation Review 1 returned `FAIL` on three proof/ordering gaps:
the zero-transition return preceded the post-reconciliation context check,
unrelated valid commit Candidates did not isolate later result checks, and the
static AST callback made no assertion.

Repair 1 Contract Review 1 rejected a proposed result interface because its
Event accessor would require a forbidden Journal import. Amendment 1 replaced
that mechanism with a private primitive fact record populated from the concrete
S2-W23 Candidate; fresh Amendment 1 review returned `PASS`.

Mandatory Repair RED first reproduced the delayed no-change cancellation as an
incorrect nil-error success, then failed on the missing primitive fact record
and validator. The minimal repair:

- checks context immediately after reconciliation and before the no-change
  return;
- extracts all seven concrete S2-W23 public result facts into primitive values;
- validates a known-valid primitive record and rejects seven isolated
  single-field mutations; and
- replaces the no-op AST walk with explicit import, goroutine, allocation,
  Journal/projection/discovery/scheduler/process, and Event-construction
  rejections.

The public API, imports, delegation, status semantics, and authority boundary
remain unchanged. The complete strict matrix passed again after Repair 1.

## Controller verification

| Check | Result |
|---|---|
| mandatory focused RED | `PASS`, exit `1` only on missing frozen symbols |
| focused GREEN | `PASS` |
| app package | `PASS` |
| app/runtime/state/projection/journal impact | `PASS` |
| focused race `-count=50` | `PASS` |
| `go test ./... -count=1` | `PASS`, all packages green |
| `go test -race ./... -count=1` | `PASS`, all packages green |
| `go vet ./...` | `PASS` |
| frozen-file `gofmt -d` | `PASS`, no output |
| `git diff --check` | `PASS`, no output |
| import/static/scope boundary | `PASS` |
| branch/head | `PASS`, `codex/loom-platform-slice2` at `9296832` |

Focused evidence covers nil/typed-nil dependency and context prevalidation,
valid zero-transition no-commit behavior, exact one/multiple transition
delegation, invalid baseline and identity drift, committer and post-commit
context errors, public commit-result mismatches, source/Candidate/accessor
mutation isolation, and accepted S2-W23 exact append/retry through a real
temporary SQLite Journal.

## Trust and residual state

S2-W22 remains reconciliation, ordering, stable-identity, and
no-absence-inference authority. S2-W23 remains Event metadata, canonical Event,
atomic append, exact retry, and commit digest authority. No installed Pi, user
Pi state, credential, network, package manager, production process,
projection mutation, scheduler, daemon, Runtime selection/reservation/
activation, Agent/model call, push, merge, rebase, reset, release, or external
mutation was used.

## Review gate

Fresh independent Repair 1 implementation review returned `PASS` with no
findings after independently rerunning the complete strict matrix. The
Candidate is accepted for an exact-scope local atomic commit.

VERDICT: PASS
