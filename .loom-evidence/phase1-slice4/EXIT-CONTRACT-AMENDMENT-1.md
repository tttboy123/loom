# Phase 1 Slice 4 Exit Contract Amendment 1

Status: FROZEN — independent Contract Review Round 4 PASS.

Date: 2026-07-26

Baseline: `7bb9881`

Parent: `EXIT-CONTRACT.md`

## Name

S4-W1 WorkItem and Projection Approval Integration Boundary

## Necessity

The accepted Slice 3 Run/WorkItem authority currently owns the only replay
rules for `work-item/<id>` streams, and the accepted Projection /
`GlobalReadView` owns the rebuildable query surface. Durable approval must:

- append a pause/resolution fact atomically with the ApprovalRequest;
- make existing WorkItem replay understand those facts without creating a
  second WorkItem authority; and
- expose immutable RuleSet/Approval records through the accepted Projection
  rather than a second read store.

S4-W1 cannot satisfy its frozen exit capability without bounded edits inside
those accepted files. Writing unknown WorkItem Events without updating replay
would poison future Run commands; publishing an independent approval cache
would violate the one-projection boundary.

## Exact reopened scope

S4-W1 may reopen only:

- `internal/work/run_authority.go`
- `internal/work/run_authority_test.go`
- `internal/projection/projection.go`
- `internal/projection/projection_test.go`
- `internal/projection/global_read_view.go`
- `internal/projection/global_read_view_test.go`

The S4-W1 contract must enumerate the new Rule/Approval files and every
governance/evidence file before RED.

## Permitted accepted-boundary changes

### WorkItem replay

- Recognize exact `WorkItemApprovalPaused` and
  `WorkItemApprovalResolved` schema-v1 Events.
- Require a valid created/assigned WorkItem, exact causation/order, exact
  approval reference, frozen previous status, and legal transition.
- S4-W1 approval pauses only an assigned WorkItem whose bound Run remains
  `unclaimed` with empty claim ID and generation zero. It does not pause a
  claimed/running process or hold Runtime capacity/Grant authority.
- Set `waiting_approval` only from exact `assigned`.
- Restore the exact previous status only for an authorized `approved`
  resolution. `rejected`/`expired` become `blocked`; `cancelled` becomes
  `cancelled`.
- Make `Claim` require WorkItem status `assigned`. It therefore rejects
  `waiting_approval`, `blocked`, and `cancelled` before allocating a claim ID or
  capacity. The request-vs-claim CAS has exactly one winner.
- Preserve existing claimed/running start/terminal/failure/cancellation
  behavior because S4-W1 cannot create an ApprovalRequest after claim.
- Fail closed on malformed, duplicate, unknown, stale, or cross-WorkItem
  approval facts.
- Normal Run writes remain touched-stream replay and
  `AppendBatchIfStreamHeads` remains the storage write authority.

S4-W1 does not add a second general WorkItem writer. The new Rule/Approval
authority may append only the two named approval lifecycle facts under the
exact multi-stream CAS frozen in its contract.

### Projection and GlobalReadView

- Add RuleSet and ApprovalRequest records to the existing atomic full rebuild.
- Project WorkItem approval status from the same approval lifecycle Events.
- Publish a new immutable GlobalReadView only after the complete rebuild
  succeeds; failure preserves the exact old Snapshot/view.
- Typed accessors copy only the requested RuleSet, ApprovalRequest, or
  WorkItem record and never expose mutable internal slices/maps.
- Projection/View remain rebuildable and never authorize activation,
  decision, approval, continuation, or resume.

## Preserved authority

This amendment does not allow:

- edits to Journal storage, AgentGrant, Team dispatch, Supervisor, Adapter,
  Evidence Artifact Store, daemon/CLI/API, credentials, or root policy;
- Rule evaluation, approval, or resume based only on Projection/View;
- Client/Daemon identity inheritance by an Agent;
- raw authorization material in Event, Projection, log, error, or Evidence;
- output classification, retry policy, verification, Done, Provider fallback,
  checkpoint, S4-W4, or Phase 2 work.

## Verification

S4-W1 must prove:

- accepted historical Slice 3 journals rebuild identically;
- approval pause/resolution replay is exact, deterministic, mutation-isolated,
  restart-safe, and conflict-visible;
- malformed/unknown approval Events fail closed without replacing Projection
  or GlobalReadView;
- normal Run/Work commands remain compatible; `Claim` cannot bypass a paused,
  blocked, or cancelled WorkItem, and a concurrent request-vs-claim race has
  one CAS winner;
- no second writer/cache/approval authority or dependency is introduced; and
- full repository/race/vet/format/diff/scope/trust gates pass.

VERDICT: FROZEN
