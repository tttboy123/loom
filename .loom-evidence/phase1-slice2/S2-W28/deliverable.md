# S2-W28 Candidate Deliverable

- WorkItem: `S2-W28`
- Title: Prepared Runtime Discovery Committer Adapter
- Risk: Strict
- Candidate state: `ACCEPTED`
- Branch/head: `codex/loom-platform-slice2` at `7ec635b`
- Contract SHA-256:
  `424554aca51c6d00ff1624e3d5cba386843b4c6aaa2edcf58bcce3e068b1ade8`
- Contract review SHA-256:
  `0e2a3c2baef902386b2c51390d38b932d3ba0a7e411e235ec4ca13f5d5f108c9`

## Contract and mandatory RED

Fresh independent contract review returned `PASS` with no blocking findings
before product/test changes. The focused RED then exited `1` only because the
frozen adapter, provider, constructor, and error symbols did not exist.

## Candidate

The Candidate:

- constructs only from nonnil/non-typed-nil S2-W20 appender and input provider
  bindings, without invoking them;
- statically satisfies accepted S2-W27 `RuntimeDiscoveryCommitter`;
- rejects invalid receiver/context before provider/appender calls;
- calls the provider exactly once with the immutable source snapshot;
- returns canonical exact context errors or provider sentinel plus source error;
- checks context after provider return;
- delegates the original snapshot, bound appender, and prepared input exactly
  once to accepted `state.CommitRuntimeDiscoverySnapshot`; and
- returns its exact immutable Candidate/error without retry or added state.

The adapter allocates no Event metadata and imports no Journal, SQLite,
projection, discovery, config, credential, scheduler, or daemon package.

## Candidate file digests

```text
aba00945672c9b971bcf92e0a62ac5ca4984fd9ae04d1b3ef823ba59efdf1434  internal/app/runtime_discovery_committer.go
ac9ab0e3222f7e2b3bcf939743f099c1beb1fd2cbc3edb965ce32259cc4df6f6  internal/app/runtime_discovery_committer_test.go
```

## Implementation Repair 1

Fresh Implementation Review 1 returned `FAIL` because the exported adapter's
zero value panicked on its nil provider binding. Repair 1 contract review
returned `PASS`. Mandatory Repair RED reproduced the nil-pointer panic. The
minimal repair revalidates stored appender/provider bindings at the method
boundary before context/provider/appender use; direct zero-value proof is now
GREEN.

The complete strict matrix was rerun. Its first repository-race run preserved
one unrelated Pi same-process-group child-marker timing failure. The failing
Pi test passed ten isolated race repetitions, and a fresh complete repository
race rerun passed. This transient is retained as evidence rather than erased.

## Controller verification

| Check | Result |
|---|---|
| focused S2-W28 | `PASS` |
| app package | `PASS` |
| app/runtime/discoveryscan/state/journal impact | `PASS` |
| focused race `-count=50` | `PASS` |
| `go test ./... -count=1` | `PASS`, all packages green |
| `go test -race ./... -count=1` | `PASS`, all packages green |
| `go vet ./...` | `PASS` |
| frozen-file `gofmt -d` | `PASS`, no output |
| `git diff --check` | `PASS`, no output |
| import/static/scope boundary | `PASS` |
| branch/head | `PASS`, `codex/loom-platform-slice2` at `7ec635b` |

Focused evidence covers constructor typed-nil validation, request context
prevalidation, exact provider-before-appender order, once counts, snapshot/input
and Candidate/accessor isolation, provider source/wrapped-context/cancellation,
S2-W20 source/input/appender/result failures, and the accepted S2-W27 path
through a real temporary SQLite Journal with explicit exact retry.

## Trust and residual state

The provider remains caller authority for Event metadata. S2-W20 remains Event
validation/construction/append/Candidate authority. No installed Pi, user Pi
state, credential, network, package manager, production process, projection
mutation, status policy, scheduler, daemon, Runtime selection/reservation/
activation, Agent/model call, push, merge, rebase, reset, release, or external
mutation was used.

## Review gate

Fresh Repair 1 implementation review returned `PASS` with no findings. The
Reviewer independently passed the complete strict matrix, confirmed the
zero-value panic closure, and assessed the recorded Pi child-marker failure as
consistent with a transient after ten isolated race passes and a fresh complete
repository-race pass. The Candidate is accepted for an exact-scope local atomic
commit.

VERDICT: PASS
