# P2A-W1 Local Product Launch Failure Diagnosis

**Date**: 2026-07-29  
**Status**: `LOCALIZED - PRODUCTION SWITCH BOUNDARY`  
**Production retry**: none  
**Resident service mutation**: none  
**Resident Journal mutation**: none

## Preserved failure

The consumed Local Product Vertical Live Closure failed before
`LOCAL_PRODUCT_CLOSURE_READY`. The native App never opened. The only retained
daemon stderr surface is the closed text `daemon failed`; it does not identify
whether the failure originated in observer execution, private IPC startup,
private IPC termination, or shutdown.

That historical result remains:

```text
FAIL - ROLLED_BACK - HUMAN_REQUIRED
allowance = 0
```

## Bounded localization

All diagnostic runs used the exact retained Candidate:

```text
loom
b12e5261632efc10585163ea323a22951f736b5565b5a717b0cf77a2765d0f29

loomd
fdca8152e924d32e2a0163a74e0e357afbcd135fce58be25f1c531d3ebb1596b
```

Each run used a private `0700` root, a copy of the one-Event resident SQLite
database, a private socket, an empty process environment with a fixed system
`PATH`, and no Provider credential. Every private root and service label was
removed after observation.

### Direct one-cycle reproduction

The exact Candidate, fresh isolation root, private SQLite copy, and private
UDS completed one observation cycle:

```text
exit = 0
completed_cycles = 1
discovery_events = 1
stderr = empty
socket after exit = absent
private Events = 2
```

### Existing-isolation reproduction

The exact Candidate with a symlink-free copy of the resident isolation root,
the same probe identity, the same Runtime search paths, and a private SQLite
copy completed normally:

```text
exit = 0
completed_cycles = 1
no_write_cycles = 1
stderr = empty
private Events = 1
```

### Private launchd one-cycle fixture

An isolated user launchd label with `KeepAlive=false`, `max-cycles=1`, private
working directory, private logs, private SQLite, and private UDS completed:

```text
runs = 1
last exit code = 0
stderr = empty
private Events = 2
```

An earlier harness invocation was deliberately discarded because its wait
condition treated launchd's `(never exited)` text as completion and booted the
still-running private job out. It changed no governed state and its label/root
were removed.

### Resident-like private launchd fixture

An isolated label using `KeepAlive=true`, no maximum cycle count, a copy of the
resident isolation root, the same probe/runtime/device/display configuration,
a private SQLite copy, and a private UDS reached readiness. The exact Candidate
CLI read a typed snapshot from that socket.

```text
socket ready = yes
initial PID = 89994
PID after 12 seconds = 89994
runs = 1
status bytes = 753
stderr bytes = 0
private Events = 1
```

The private label was booted out and its root was deleted.

### Path-shape control

A direct run under a private path containing spaces also completed with
`exit=0` and empty stderr.

## Current conclusion

The evidence rules out a stable defect in:

- the retained Candidate executable bytes;
- one-cycle observer execution;
- existing isolation metadata;
- concurrent read-only product database access;
- private UDS startup and cleanup;
- launchd execution with an empty environment;
- `KeepAlive=true`;
- the resident probe/runtime/device/display configuration;
- source-path provenance metadata, which is present on both Candidate
  executables.

The failure is not currently reproducible outside the production switch
transaction. The remaining boundary is the exact install/bootout/bootstrap
transition and exact production UDS path. The generic stderr surface is too
weak to assign a narrower root cause without guessing.

The next safe action is one reviewed P2A-W1 repair contract that adds closed
reason-code instrumentation, proves the exact production socket path against
copied state without restarting the resident observer, and hardens the
single-bootstrap switch ordering. It must not reinterpret the consumed canary
or authorize an unreviewed retry.

## Replacement transaction diagnosis

The reviewed replacement transaction was invoked once and returned:

```text
LOCAL_PRODUCT_CLOSURE_RESULT result=rollback_incomplete initial_bootstrap_calls=1 consumed=1 rollback_count=1 restart_count=0
```

It emitted no `LOCAL_PRODUCT_CLOSURE_READY` marker and created no reason record.
The native App was never launched. The allowance is `0` and no retry is
authorized.

The transaction sets `initial_calls=1` and `consumed=1` immediately before its
only Candidate bootstrap. After readiness, it evaluates socket ownership,
Candidate process identity, installed product bytes/modes, installed App
bytes/bundle/signature, typed status, Journal, crash inventory, and staging as
bare `set -e` predicates. Those predicates have no closed phase attribution
and do not call `write_failure_reason`. A bootstrap/readiness failure normally
calls that writer, but the writer itself is also a bare command. Therefore the
absence of a reason record cannot identify the exact failed predicate.

The 6.5-second transaction duration is consistent with reaching an early
post-bootstrap predicate rather than exhausting the 20-second readiness loop,
but this is only an inference. The exact predicate remains unknown.

Rollback restored the exact original binaries, wrapper, plist, metadata,
Journal, reports, and absence predicates. Its synchronous 20-second
`wait_original_running` deadline expired while launchd was still
`spawn scheduled`, so the transaction honestly reported
`rollback_incomplete`. The unchanged original observer subsequently recovered
without another transaction or manual lifecycle command and was stable at PID
`43503`, runs `32`, across three samples spanning sixteen seconds.

The deterministic defects are:

1. incomplete closed phase attribution from bootstrap through the READY
   boundary;
2. a reason-writer failure can itself erase the only classification;
3. the original-service recovery deadline does not cover the observed
   launchd throttle plus observer startup envelope;
4. late successful recovery cannot update an already emitted
   `rollback_incomplete` result.

The next safe action is a reviewed, non-live extension of the same P2A-W1
contract that closes every pre-READY predicate, makes reason recording
best-effort without hiding the primary phase, tests delayed recovery, and
retains allowance `0`. It must not infer the missing predicate or authorize
another canary.
