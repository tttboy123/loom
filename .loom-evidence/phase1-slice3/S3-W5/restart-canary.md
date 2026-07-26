# S3-W5 Restart and Concurrency Canary

- Date: `2026-07-26`
- Scope: controlled local SQLite and fixture execution only
- Activation: none

## Proven paths

`TestTeamDAGExecutionControlledCanary` proves:

- one Main and two independent SubAgents use the real Supervisor boundary;
- the two SubAgents overlap while separate Runtime capacity remains within its
  accepted bound;
- the dependency-bound Main starts only after both SubAgents succeed;
- Main attempt 1 records the exact controlled failure, then one explicit
  fallback directive schedules independent attempt 2 on the selected Runtime;
- every attempt receives three authorized tentative Frames;
- no tentative output bytes enter Journal payloads;
- attempt 2 succeeds and Team aggregation becomes terminal succeeded.

`TestTeamCoordinatorRecoversDurableAttemptWindows` proves:

- process loss after a terminal Run but before artifact/receipt/Evidence
  metadata completion finalizes the existing capture without another adapter
  execution;
- an expired claimed, never-started attempt revokes the old Grant, reclaims the
  same Run at generation N+1, rebinds the empty capture, fences the Team
  attempt, issues a new Grant, and executes exactly once;
- the old Grant remains projected as revoked with reason `operator`;
- the capture and Team attempt expose the same accepted N+1 binding; and
- a Run already in `running` without terminal returns
  `ErrTeamAttemptRecoveryRequired` with zero adapter executions.

The attempt-capture and Team-authority package tests additionally cover exact
retry idempotency, stale generation rejection, duplicate/divergent Frame
sequence rejection, terminal reconciliation, deep-copy accessors, and
same-stream CAS conflict behavior.

## Explicit non-claims

- No daemon, timer, resident scheduler, installed Pi process, external network,
  Provider fallback, Web/TUI, or client event stream was activated.
- The canary does not make tentative Frames authoritative.
- Slice 4 output semantics and recovery policy remain queued.
- Slice 5 client delivery remains queued.

VERDICT: PASS
