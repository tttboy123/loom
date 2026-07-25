# S2-W35 Candidate Deliverable

- WorkItem: `S2-W35`
- Title: One-Shot Projected Configured Runtime Observation Cycle
- Risk: Strict
- Candidate state: `ACCEPTED_PENDING_LOCAL_COMMIT`
- Branch/head: `codex/loom-platform-slice2` at `a6eb816`
- Contract SHA-256:
  `a9071190a1dd8a6e391ec0144006b6ab4627317fd6ca08ff7b6c2836a1f722f8`
- Contract review SHA-256:
  `40d61246c5352e2deeac90cc668d51d8b53bbec1f3b7bdf3dba30cf28d23774a`

## Contract, RED, and Candidate

Fresh contract review returned `PASS` with no findings. Mandatory RED added only
the test file and failed solely on the missing frozen S2-W35 error/coordinator
symbols.

The minimal Candidate validates context/read model, reads accepted
`Projection.Snapshot()` exactly once, checks context, and delegates the exact
copied baseline to S2-W34 exactly once. Every error returns all five outputs
zero; success returns the exact downstream facts.

```text
084098029b6256cd2a8e741709fc044cb83e37adc204ba1d51cdaee8ceabfe04  internal/app/runtime_observation_projected.go
7ec0bfd0396ff3770598042635ef77067d35c38cf4954de62a21aa079f4657b1  internal/app/runtime_observation_projected_test.go
```

## Controller verification

The complete strict matrix passes: focused, app, app/runtime/discoveryscan/
state/projection/journal impact, focused-race-50, repository,
repository-race, vet, formatting, and diff.

Focused proof covers context/read-model validation; exact Snapshot-before-
factory composition; empty/absence/unchanged no-write; mixed discovery
priority; status-only; configured discovery/write/cancellation five-zero
errors; opposite-path non-use; explicit caller retry; read-model/discovery/
commit accessor mutation isolation; and static no-rebuild/direct Journal/
metadata/retry/scheduler/daemon/config/activation/Slice 3 boundaries.

The real SQLite test rebuilds a real projection from discovery sequence 1,
runs projected mixed discovery sequence 2 twice idempotently, rebuilds, then
runs projected status sequence 3 twice idempotently. Exactly three Events
remain, with types `RuntimeInstanceDiscovered`, `RuntimeInstanceDiscovered`,
and `RuntimeInstanceStatusChanged` at sequences 1, 2, and 3. A final rebuild
proves the exact display name, online status, discovery sequence 2, and status
sequence 3; no opposite writer is called.

## Review gate

Fresh Implementation Review 1 returned `FAIL` on one SQLite exact-Events proof
gap and found no product defect. Repair 1 was frozen as test-only; fresh
repair-contract review returned `PASS` with no findings. Mandatory Repair RED
failed only on the three missing coverage markers. The test-only repair now
proves exact persisted Event types/sequences, exact retry Candidate identity,
and final rebuilt Runtime facts. The product hash remains byte-for-byte
unchanged, and the complete strict matrix passes again. Fresh Repair 1
implementation review returned `PASS` with no findings after independently
rerunning the complete strict matrix. The Candidate is accepted and may
receive its one exact-scope local atomic commit after a fresh pre-commit matrix
and staged-scope audit.

VERDICT: PASS
