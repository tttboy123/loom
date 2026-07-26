# Phase 1 Slice 5 Exit Contract Amendment 1

Status: FROZEN — independent Amendment Review 1 PASS.

- Date: `2026-07-26`
- Baseline: `006db8c`
- Affected WorkItem: `S5-W1`
- Boundary: Team observation Projection freshness before authorized execution

## Trigger

The S5-W1 external coordinator/observer integration trace found a lifecycle
gap in the accepted `TeamCoordinator.Run`:

1. first-attempt `DispatchTeamReadySet` commits Team/Work/Run/capacity facts;
2. `prepareTeamTasks` then creates Grant and attempt capture;
3. task execution emits an already-authorized Frame to the S5-W1 observer;
4. but the coordinator's shared Projection has not rebuilt since step 1.

The recovery path has the same shape after a generation rebound:
`recoverTeamAttempts` can commit rebound facts and return an executable task
before the next Projection rebuild.

The frozen S5-W1 observer must validate Team/node/attempt/WorkItem/Run/
generation/Runtime/Agent binding against the copied `GlobalReadView`. Using
the stale pre-dispatch/pre-rebound view would reject a legitimate authorized
Frame. Relaxing that validation would violate the frozen authority boundary,
and rebuilding inside the observer would violate its bounded non-blocking
delivery contract.

## Exact ownership amendment

S5-W1 may additionally reopen:

- `internal/app/team_execution.go`
- `internal/app/team_execution_test.go`

The production hunk is limited to a named internal read-only refresh helper
and exactly two call sites:

1. after `DispatchTeamReadySet` succeeds and before `prepareTeamTasks`/task
   execution; and
2. after recovery/rebound returns executable tasks and before those tasks
   execute.

The existing app test file may add only focused tests proving each executable
path publishes a refreshed exact attempt/generation before invoking its
observer and that refresh failure stops execution.

The already-frozen external
`internal/app/team_execution_stream_test.go` remains the end-to-end
`app -> api` consumer integration and must not be moved back into
`package app`.

## Frozen behavior

The helper performs exactly one call to the accepted
`Projection.Rebuild(ctx)` and wraps failure as Team projection rebuild
failure. It:

- performs no Journal write or CAS;
- does not retry;
- does not change the dispatch/rebound transaction;
- does not create or authorize a Grant;
- does not execute a Runtime;
- does not change scheduling, capacity, generation, Evidence, recovery,
  verification, acceptance, or terminal semantics; and
- does not publish a partial view because accepted Projection rebuild keeps
  the old view on failure.

The first execution after a dispatch/rebound may begin only after the refresh
returns success. A failed/cancelled refresh returns the error and executes no
task/Frame. This is observation freshness, not a second authority or execution
permission.

## Verification

Before S5-W1 implementation Review:

- focused first-dispatch and rebound refresh tests pass;
- a forced Projection refresh failure proves zero executor calls;
- the external S5-W1 observer receives valid exact generation output;
- malformed/wrong-Team/old-generation output still fails closed;
- the existing Team DAG controlled canary and restart/recovery tests remain
  unchanged and pass;
- focused race repetition, full repository, repository-race, vet, format,
  Windows compilation, dependency, scope, and authority audits pass.

No other app product/test file, API, schema, owned file, WorkItem, dependency,
migration, daemon, live Runtime, Provider, S5-W2, or S5-W3 scope is added.

VERDICT: PASS
