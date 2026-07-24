# S1-W4 Repair 2 Contract

- Lineage: `S1-W4`
- Repair: `2 of 3`
- Trigger: fresh strict Reviewer `VERDICT: FAIL`
- Owned files remain:
  - `internal/projection/projection.go`
  - `internal/projection/projection_test.go`

## Single blocker

`Projection.Rebuild` selects between acquiring `rebuildGate` and
`ctx.Done()`. If cancellation and gate availability are both ready, Go may
select the gate branch and call the source after the caller was canceled.

Controller reproduced the failure:

`go test ./internal/projection -run
'^TestRebuildSerializesConcurrentCandidatesAndCanceledWaiterDoesNotSwap$'
-count=200`

failed repeatedly with:

`canceled waiter Rebuild() error = missing blocking source call, want context canceled`

## Required repair

- Immediately after acquiring `rebuildGate`, re-check `ctx.Err()` before
  calling the source.
- A canceled waiter must return `context.Canceled`, must not consume a source
  call, and must not swap the snapshot.
- Strengthen the focused regression so the post-acquire cancellation branch is
  deterministic or prove it with a repeated count.
- Do not change APIs, replay semantics, Event/status values, digest policy, or
  any file outside the original ownership boundary.

## Checks

- Targeted:
  `go test ./internal/projection -run
  '^TestRebuildSerializesConcurrentCandidatesAndCanceledWaiterDoesNotSwap$'
  -count=200`
- Targeted race:
  `go test -race ./internal/projection -run
  '^TestRebuildSerializesConcurrentCandidatesAndCanceledWaiterDoesNotSwap$'
  -count=100`
- All frozen S1-W4 focused, package, repository, race, vet, formatting, and
  diff checks remain mandatory.
