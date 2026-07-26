# S5-W1 Mandatory Behavioral RED

- Date: `2026-07-26`
- Baseline: `006db8c`
- Contract: independent Contract Repair Review 3 `PASS`
- Product edits before RED: none

## Test-only changes

RED adds only frozen tests:

- Journal page behavior in `internal/journal/journal_test.go`;
- selective immutable view behavior in
  `internal/projection/global_read_view_test.go`;
- local cursor/gap/config behavior in
  `internal/api/team_execution_stream_test.go`;
- external-package observer conformance in
  `internal/app/team_execution_stream_test.go`; and
- finite injected CLI behavior in `cmd/loom/main_test.go`.

## Exact command

```text
go test ./internal/journal ./internal/projection ./internal/api ./internal/app ./cmd/loom
```

Exit: `1`, as required.

## Observed absence

The failure is the frozen capability gap:

- `Store.ReadPageAfterHeads`, `MaxCursorStreams`,
  `MaxReadPageEvents`, and Journal cursor/page errors do not exist;
- `GlobalReadView.WorkItemsForTeam`,
  `ApprovalRequestsForTeam`, and `AgentGrantsForRun` do not exist;
- `internal/api` has no product file, stream config/constructor, cursor codec,
  gap/page values, or typed errors;
- the external `app.NodeOutputObserver` conformance cannot compile because
  `api.TeamExecutionStream` does not exist; and
- `cmd/loom` cannot compile its frozen timeline RED until the new API exists.

Representative compiler evidence:

```text
internal/journal/journal_test.go:835:21:
  store.ReadPageAfterHeads undefined
internal/projection/global_read_view_test.go:200:20:
  view.WorkItemsForTeam undefined
internal/api/team_execution_stream_test.go:35:13:
  undefined: TeamExecutionStreamConfig
loom-pi-rebuild/internal/api: no non-test Go files
FAIL loom-pi-rebuild/internal/app [build failed]
FAIL loom-pi-rebuild/cmd/loom [build failed]
```

No production file, migration, dependency, writer, daemon, Runtime, Provider,
network, WorkPackage, S5-W2, or S5-W3 behavior was added to obtain RED.

## Amendment 1 lifecycle RED

After independent Exit Contract Amendment 1 Review 1 `PASS`, the exact focused
command was:

```text
go test ./internal/app \
  -run '^TestTeamCoordinatorRefreshesProjectionBeforeAuthorizedObservation$' \
  -v
```

Exit: `1`, as required.

Both subtests reached the frozen failure:

- first dispatch: the observer could not find generation 1 in the stale
  pre-dispatch view, so the authorized output was rejected and the attempt
  classified invalid/blocked;
- generation rebound: the observer could not find generation 2 in the stale
  pre-rebound view and produced the same invalid/blocked outcome.

No app production edit preceded this RED.

VERDICT: PASS
