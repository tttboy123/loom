# S2-W34 Candidate Deliverable

- WorkItem: `S2-W34`
- Title: One-Shot Configured Runtime Observation Cycle
- Risk: Strict
- Candidate state: `ACCEPTED_PENDING_LOCAL_COMMIT`
- Branch/head: `codex/loom-platform-slice2` at `affd2a6`
- Contract SHA-256:
  `d999672c8c265bbcfd923412681ae632eebc7eb2d2795a9ea1e3933ff3d51d32`
- Contract review SHA-256:
  `108426bd3437954feb30e257ef74ffbf0303852009d1fb32debf1f8d28b166cd`

## Contract, RED, and Candidate

Fresh contract review returned `PASS` with no findings. Only
`runtime_observation_cycle_test.go` existed for mandatory RED; the focused
command exited nonzero only because
`ErrInvalidConfiguredRuntimeObservationRun` and
`RunConfiguredRuntimeObservationOnce` were undefined.

The minimal Candidate validates context, invokes accepted S2-W26 configured
discovery exactly once, then invokes accepted S2-W33 exactly once with the
exact immutable snapshot and caller-supplied copied projection. Every error
returns the snapshot and all four downstream outputs zero. Successful
`none`/`discovery`/`status` paths return only the exact accepted facts.

```text
26a1a706c8c058d48d5304aa5eddbb7fa2008cc16ff9e02c0eb9edff8974d7ea  internal/app/runtime_observation_cycle.go
4a4b3fb31c9d7492f86a5965b24549e7ccced9a51c504f0ac56a2af177579f9e  internal/app/runtime_observation_cycle_test.go
```

## Controller verification

All frozen checks passed:

- focused S2-W34;
- app package;
- app/runtime/discoveryscan/state/projection/journal impact;
- focused race at `-count=50`;
- repository and repository-race;
- vet, formatting, and diff;
- nil/canceled/deadline/factory discovery errors with five zero outputs;
- empty/all-absent/unchanged/absence-only no-write cycles;
- current-only and mixed discovery priority;
- status-only exact S2-W33 delegation;
- missing selected dependency, mismatch, delayed cancellation, no retry, and
  opposite-path non-use;
- factory/probe ordering, direct caller-slice mutation during the accepted
  defensive-copy path, caller projection preservation, snapshot/Candidate/Event
  accessor isolation, and explicit retry; and
- static exact S2-W26→S2-W33 composition with no S2-W27, projection query/
  rebuild, metadata, direct Journal, retry loop, scheduler, daemon, config
  loader, activation, or Slice 3 authority.

The temporary SQLite test appends prior discovery sequence 1, runs a configured
mixed observation that selects only discovery sequence 2 twice idempotently,
rebuilds projection, then runs a configured status-only observation that
selects only status sequence 3 twice idempotently. Exactly three canonical
Event rows remain in discovery/discovery/status order.

## Review gate

Fresh Implementation Review 1 returned `FAIL` on one test-proof gap and found
no product defect: the S2-W34 boundary did not directly exercise the complete
S2-W26 discovery/factory error matrix.

Fresh test-only Repair 1 contract review returned `PASS`. Mandatory Repair RED
failed on all six missing case markers. The repaired tests now directly prove
oversized and typed-nil factories, invalid present/absent probe result pairs,
probe source error, and invalid discovered observation. Every case returns all
five outputs zero and makes no committer call. The complete strict matrix
passes again and the product hash remains byte-for-byte unchanged.

Fresh Repair 1 implementation review returned `PASS` with no findings after
independently rerunning the complete strict matrix. The Candidate is accepted
and may receive its one exact-scope local atomic commit after a fresh
pre-commit matrix and staged-scope audit.

VERDICT: PASS
