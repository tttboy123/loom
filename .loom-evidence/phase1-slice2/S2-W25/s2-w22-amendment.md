# S2-W22 Contract Amendment for S2-W25

- Affected accepted WorkItem: `S2-W22`
- Amendment owner: Controller
- Status: `FROZEN_FOR_REVIEW`
- Trigger: accepted S2-W24 local commit `9838779`

## Why the accepted wording must change

The accepted S2-W22 contract was frozen before status Events existed in the
Runtime projection. Its baseline provenance fields therefore assumed that the
previous status-bearing fact was always a discovery Event:

```go
LastDiscoveryEventID
LastDiscoverySequence
```

Accepted S2-W24 now permits consecutive canonical
`RuntimeInstanceStatusChanged` Events. After the first status Event, the latest
status-bearing fact is the latest status Event, not the older discovery Event.
Keeping discovery-specific field names would either make a later status Event
reference stale provenance or silently give those names a second meaning.

## Frozen amendment

S2-W22 `RuntimeStatusBaseline` is renamed to:

```go
type RuntimeStatusBaseline struct {
    Instance         RuntimeInstance
    PreviousEventID  string
    PreviousSequence int64
}
```

The renamed fields mean exactly the previous canonical status-bearing fact for
the next reconciliation:

- the latest status Event ID/sequence when complete S2-W24 status provenance is
  present; otherwise
- the latest discovery Event ID/sequence.

S2-W22 baseline validation continues to require a nonempty Event ID and a
positive sequence. Reconciliation copies these fields without reinterpretation
to `RuntimeStatusTransition.PreviousEventID` and `PreviousSequence`.

The canonical baseline digest record renames its JSON fields to
`previous_event_id` and `previous_sequence`. Because both schema and meaning
change, `runtimeStatusBaselineDigestVersion` increments from `1` to `2`.
`runtimeStatusCandidateDigestVersion` remains `1`: the Candidate already binds
the complete versioned baseline digest, and its own schema does not change.

No compatibility aliases are retained. Parallel discovery-specific and generic
fields would permit contradictory provenance and create two candidate inputs.

## Unchanged S2-W22 authority and behavior

All other accepted S2-W22 semantics remain unchanged:

- zero through 32 canonical, uniquely identified baseline entries;
- deterministic ordering and copied immutable data;
- explicit status comparison only;
- stable device/adapter identity;
- no inference from absence;
- no Event construction or append;
- no projection, discovery, process, scheduler, daemon, activation, or external
  side effect.

S2-W25 owns the pure projection-to-baseline adapter and the contained source
rename. It does not reopen the accepted reconciliation algorithm, transition
shape, status rules, or trust boundary.

VERDICT: CONTRACT_AMENDMENT_FROZEN
