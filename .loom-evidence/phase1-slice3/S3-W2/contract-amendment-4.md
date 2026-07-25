# S3-W2 Contract Amendment 4 — Persisted Runtime Status-Head Reference

- Active parent: S3-W2 parent contract plus Amendments 1, 2, and 3
- Trigger: Amendment 3 Contract Review 1 `FAIL`
- Date: `2026-07-26`
- Product repair attempts: unchanged

## Required payload reference

Every S3-W2 Run or capacity fact whose operation CAS includes the Runtime
status stream head must persist:

```json
{
  "runtime_status_stream_id": "runtime_instance:<runtime_instance_id>",
  "runtime_status_sequence": 7,
  "runtime_status_event_id": "the exact discovery-or-status Event ID at seq 7"
}
```

The fields are required on:

- `RunClaimed`;
- `RuntimeCapacityReserved`;
- reclaim's `RuntimeCapacityReleased`;
- `RunStarted`;
- `RunTerminalCommitted`;
- terminal's `RuntimeCapacityReleased`.

`RunPrepareLeaseExtended` and WorkItem facts do not add them because those
facts neither validate nor CAS Runtime availability.

## Command behavior

Before each claim/reclaim/start/terminal mutation, Authority derives the
current status-bearing Runtime fact from the accepted status stream. The
persisted stream, sequence, and Event ID must equal the exact head expectation
passed to Journal CAS.

Claim/reclaim/start require that referenced fact projects Runtime `online`.
Terminal release may reference a later offline/disabled/incompatible fact
because terminal cleanup must remain possible after Runtime degradation.

The capacity fact and its paired Run fact persist identical references.

## Projection behavior

Dependency-aware replay must verify:

1. the referenced stream is exactly
   `runtime_instance:<runtime_instance_id>`;
2. the referenced Event exists at the exact sequence with the exact Event ID
   and is a valid `RuntimeInstanceDiscovered` or
   `RuntimeInstanceStatusChanged` fact for that identity;
3. the reference is the status stream head observed by that operation, not a
   fabricated or future fact;
4. claim/reclaim/start references project `online`;
5. paired Run and capacity facts carry identical references;
6. later status facts have larger sequence and do not retroactively invalidate
   an already-linearized claim/start;
7. missing, stale, future, mismatched, or non-status references reject while
   preserving the previous Snapshot.

Because the reference is persisted, replay does not rely on database row
adjacency, wall-clock ordering, or lexicographic cross-stream order.

## Test repair

The Amendment 3 test repair additionally asserts:

- offline-before-reference rejects claim/start;
- offline-after-referenced-online-head remains projectable;
- paired facts with different status-head references reject;
- a fabricated sequence/Event ID rejects;
- terminal cleanup referencing the current offline head succeeds.

Capture focused RED on the missing reference behavior before product repair.

## Unchanged boundary

Replace Amendment 3's “no Event payload changes” sentence with:

> No public API or Event envelope changes. S3-W2 internal Event payloads add
> only the Runtime status-head reference fields frozen here.

All public APIs, owned files, stream names, lifecycle, acceptance, Journal CAS,
read surface, verification commands, trust boundaries, and exclusions remain
unchanged. Accepted Runtime discovery/status Event payloads and StateWriter are
not modified.

Fresh independent Contract Review must return `PASS` before test/product
repair.

VERDICT: PASS
