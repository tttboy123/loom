# S2-W24 Candidate Deliverable

- WorkItem: `S2-W24`
- Title: Runtime Status Event Projection
- Risk: Strict
- Candidate state: `ACCEPTED`
- Branch/head: `codex/loom-platform-slice2` at `1ba3238`
- Contract SHA-256:
  `4e5e0ce7b75b432c7c8c11ba87a23d35237569ee1a11fb99504af020712c8ec7`

## Candidate

The Candidate makes committed canonical `RuntimeInstanceStatusChanged` Events
an accepted rebuildable projection fact. It adds separate status-transition
provenance to the Runtime read model, validates the complete flat S2-W23
payload/envelope and duplicate fields, requires an existing Runtime, stable
identity, exact current `from_status`, exact latest status-bearing provenance,
causation, and exact-next sequence, then changes only status and status
metadata.

Discovery and inventory fields remain copied and unchanged across status Events.
Consecutive status Events chain from the latest status Event. A later accepted
rediscovery retains S2-W21 complete-replacement semantics and naturally resets
the separate status-transition metadata to zero.

## Mandatory RED

The focused command exited `1` only because the ten frozen status read-model
fields did not exist. The status product file and dispatch behavior did not yet
exist. No syntax, dependency, environment, or unrelated failure occurred.

## Final file digests

```text
783d258f230ff9b776569740dda42e671d3ea5bc6133b0d24f6a3a3cea3d2671  internal/projection/projection.go
6ec9bac4601659ea0c37f11b5516257abf8ce74ab6976650a3f430a840840599  internal/projection/runtime_discovery.go
8bf7d0f372e19ec0593b4f5e36508c113334d7d26315e36fb0e2162b417bb2b1  internal/projection/runtime_status.go
33e5963d4b1d1d87ceb09ceca4ff1dd8e5be5496a7d278c8114df5f591fa37f1  internal/projection/runtime_discovery_test.go
ff8f08ccd94725299ac46074afadcdaa2bb04ce428a4578ddd3cee37f8a7967c  internal/projection/runtime_status_test.go
```

`internal/projection/projection_test.go` was inside the frozen ownership
boundary but required no change.

## Controller verification

| Check | Result |
|---|---|
| focused S2-W24 test | `PASS` |
| projection package | `PASS` |
| projection/state/runtime/journal impact | `PASS` |
| focused race `-count=30` | `PASS` |
| `go test ./... -count=1` | `PASS`, all packages green |
| `go test -race ./... -count=1` | `PASS`, all packages green |
| `go vet ./...` | `PASS` |
| frozen-file `gofmt` | `PASS`, no output |
| `git diff --check` | `PASS`, no output |
| import/non-disclosure/scope checks | `PASS` |
| branch/head | `PASS`, `codex/loom-platform-slice2` at `1ba3238` |

Focused evidence covers:

- real S2-W20 discovery plus S2-W22 reconciliation plus S2-W23 status Event
  Journal integration;
- exact status/provenance output with discovery/inventory preservation,
  deterministic fresh rebuild, copied Snapshot access, and source isolation;
- consecutive status chaining and later rediscovery replacement/reset;
- status-before-discovery, unknown Runtime, device/adapter drift, current
  from-status mismatch, equal/unknown statuses, stale/future previous
  provenance, causation mismatch, lower/equal/higher sequence, and exact
  `MaxInt64` overflow rejection;
- empty envelope fields, zero/non-UTC time, schema/stream mismatch, malformed
  JSON, all missing/null fields, unknown/duplicate fields, invalid digests, and
  empty source probe;
- failed-rebuild Snapshot preservation, unordered input, exact duplicate replay,
  empty rebuild, and source/payload/accessor mutation isolation; and
- static import, no-write/no-discovery/no-reconciliation/no-execution,
  non-disclosure, and scope checks.

## Trust and residual state

Accepted replay remains Event ordering, duplicate/conflict, sequence-gap,
schema-version, cancellation, and atomic Snapshot-swap authority. S2-W23 remains
Event construction/write authority. S2-W21 rediscovery and S2-W22 baseline
contracts remain unchanged.

This projection exposes separate latest status provenance but does not construct
or reinterpret a future S2-W22 baseline. Scheduler/baseline integration remains
a separate frozen WorkItem.

No Event append, StateWriter, discovery/reconciliation invocation, next-baseline
construction, projection-table/direct SQL/schema/config/file write, Pi process,
installed Pi, user Pi state, credential, network, package manager, scheduler,
daemon, Agent session, prompt, model call, Runtime selection/reservation/
activation, push, merge, release, or external mutation was used.

## Review result

Fresh independent implementation review independently passed the complete
strict matrix and verified contract compliance, dispatch, read-model shape,
latest status-bearing provenance, rediscovery reset, exact payload/duplicate/
envelope validation, replay authority, Snapshot atomicity/immutability,
non-disclosure, and scope. It returned `PASS` with no blocking findings:
`.loom-evidence/phase1-slice2/S2-W24/implementation-review.md`.

VERDICT: PASS
