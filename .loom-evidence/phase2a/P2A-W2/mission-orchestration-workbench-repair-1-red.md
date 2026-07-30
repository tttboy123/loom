# P2A-W2 Mission Orchestration Workbench — Repair 1 RED

Status: **RED captured, implementation now GREEN pending full matrix**

The failed review was reproduced by source inspection and compile-time removal
of the callback API:

- `PreparedMissionDecisionExecutor` and its arbitrary authoritative result were
  removed;
- `MissionDecisionConfig` now accepts only
  `*PreparedMissionDecisionBackend`;
- typed prepared inputs are separated into Authorization, Review and Recovery;
- the production runner constructs the Decision API even when its immutable
  prepared registry is empty.

The repair tests require:

- a real pending Rules approval in SQLite, exact approved/rejected requests,
  one-winner concurrency, stale command binding rejection, a real post-sheet
  Journal view advance rejected before authority, and Projection refresh;
- a real Team attempt, Run, Evidence and ready-for-review lineage followed by
  exact Work acceptance;
- a real failed Team attempt followed by one exact deterministic Work recovery;
- per-action preparation so visible but unbound mutation actions stay disabled;
- a real daemon socket proving an empty production registry fails closed with
  `conflict`, not `state_unavailable`.
