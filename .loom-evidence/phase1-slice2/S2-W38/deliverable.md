# S2-W38 Candidate Deliverable

- WorkItem: `S2-W38`
- Title: Projection-Synchronized Triggered Runtime Observation
- Risk: Strict
- Candidate state: `ACCEPTED_PENDING_LOCAL_COMMIT`
- Branch/head: `codex/loom-platform-slice2` at `9175f94`
- Active Contract Repair 2 SHA-256:
  `f19a9b82253c7cd972b9106eac534ac43b30f0a381f37aa9c3b16e33215e4947`
- Repair 2 Contract Review 1 SHA-256:
  `b47f41c5dd9ba511cbcca29aa50a7775181cb27f1b35ce33ee837804114fc333`

## Contract lineage

The original recurrence contract passed fresh Contract Review 1, but
Controller pre-RED code-truth review returned `FAIL`: repeated S2-W37 calls
would read the prepared observer's stale in-memory projection after committed
Events. No implementation began.

Contract Repair 1 postponed recurrence and inserted pre/post projection
refresh, but fresh Repair 1 Review 1 returned `FAIL`: a separate projection
parameter could differ from the observer's private binding, and the
partial-success language exceeded what S2-W37 can expose. This was the second
same-class stale-view failure, so fresh read-only Problem Analysis 1 was
mandatory.

Problem Analysis 1 confirmed an application-composition/read-model lifecycle
defect, not a Journal, replay, planner, trigger, or writer defect. Contract
Repair 2 removes the separate parameter, captures only `observer.readModel`,
rejects a zero-value observer, freezes the exact error boundary, and passed
fresh Contract Review 1 with no findings.

## RED and minimal Candidate

The first RED attempt exposed that Go's compile-error limit had hidden one
test-fixture field mistake behind the missing frozen symbols. That RED was
discarded: the fixture was corrected, the newly added product file was removed,
and mandatory RED was rerun from test-only state.

The valid mandatory RED failed solely because
`ErrInvalidProjectionSynchronizedRuntimeObservationRun` and
`RunProjectionSynchronizedRuntimeObservationOnce` were missing.

The minimal Candidate validates context, trigger, observer, and the observer's
private bound projection. One unexported trigger decorator performs underlying
await → context check → pre-observation rebuild. The exported function calls
accepted S2-W37 exactly once and performs one post-success rebuild of the same
captured projection.

```text
33d7e46283a257d1b7cb555b77a94f5491405db0d42654c934183ba58790c9c0  internal/app/runtime_observation_loop.go
60c65473c339a95768051e8359e121f83eb2e04280f92cbabfa886070f1dff06  internal/app/runtime_observation_loop_test.go
```

## Controller verification

The complete strict matrix passes: focused, app, impacted
app/runtime/discoveryscan/state/projection/journal packages,
focused-race-50, repository, repository-race, vet, formatting, and diff.

Direct proof covers nil and typed-nil trigger, nil and zero-value observer,
pre-context, trigger error/cancellation, deterministic pre-refresh failure,
the complete S2-W37/downstream failure matrix with exact call counts, and no
retry or post-refresh on error.

A post-success refresh failure proves the exact successful discovery tuple is
retained with the refresh error, the authoritative Event remains persisted,
the previous projection snapshot is preserved, and no retry occurs.

The real SQLite chain reuses the same read model, observer, trigger, and
shallow-copied scripted factory binding across two calls. Without any
test-side inter-call rebuild, seeded discovery sequence 1 is followed by mixed
discovery sequence 2 and status-only sequence 3. Product post-refresh exposes
the changed display/offline discovery facts after call one and the online
status facts after call two.

Static proof requires one underlying await, one exact S2-W37 call, two
same-binding `Rebuild` calls, one private-binding capture, allowed imports, and
no loop or prohibited authority.

## Trust boundary

S2-W38 owns only trigger-scoped pre-refresh, one S2-W37 observation, and
post-success refresh of the same captured rebuildable read model. Journal and
accepted committers remain authoritative.

It adds no recurrence, serialization, time/timer/ticker/channel/signal,
goroutine, retry, configuration, daemon/CLI entry, direct lower-layer
composition, Event metadata, activation, or Slice 3 authority. An S2-W37 error
does not claim that no Event committed and is never blindly retried. Concurrent
invocations remain outside this sequential one-shot contract.

## Implementation Repair 1

Fresh Implementation Review 1 returned `FAIL` on one mandatory proof gap and
found no product defect: none/discovery/status success paths were not collected
in one direct runtime order/count group.

Implementation Repair 1 is test/evidence-only and locks the product hash.
Fresh repair-contract review returned `PASS` with no findings. Mandatory
Repair RED failed only on four missing unique case-local success markers.

The repaired tests now prove none by exposing Runtime A before factory work,
appending Runtime B during that work, selecting no writer, and exposing B only
after post-refresh. Discovery and status use real prepared committers behind
recording wrappers to prove exact trigger→factory→probe→selected-writer traces,
exact selected/opposite counts, and post-refresh sequence 2 facts. The complete
strict matrix passes again and product remains byte-for-byte unchanged.

## Review gate

Fresh independent Repair 1 Implementation Review 2 returned `PASS` with no
findings after independently verifying the new direct three-path proof, exact
projection binding, error asymmetry, SQLite chain, static boundary, product
lock, and complete strict matrix. The Candidate is accepted and may receive its
one exact-scope local atomic commit. The fresh pre-commit focused, app, impact,
focused-race-50, repository, repository-race, vet, formatting, diff, and
product-lock matrix passes; only the staged-scope audit remains.

VERDICT: PASS
