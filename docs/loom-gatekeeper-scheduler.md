# Loom Gatekeeper + Scheduler (Phase D)

> **Status**: implemented in `devkit/gatekeeper.py` and `devkit/scheduler.py`
> on `feat/phase-d-gatekeeper-scheduler` @ `d145ea3`.
> **Date**: 2026-07-05.
> **Scope**: typed final-state verdict (`GateVerdict`) + typed next-task
> driver (`ScheduleDecision`). **Does not** replace `iterate.py`'s
> `run_loop` or `devkit/__main__.py`'s autopilot gate; integration is
> opt-in (see "Integration status" below).

## What these two modules are

`gatekeeper.py` and `scheduler.py` take two responsibilities out of the
Runner:

- **Gatekeeper** owns the "is this evidence good enough?" decision. It
  reads `devkit/runs/<run-id>/evidence.json`, classifies its source,
  checks the four failure codes, and emits a `GateVerdict` that the
  Runner can act on without parsing JSON itself.
- **Scheduler** owns the "what's next?" decision. It reads
  `devkit/backlog.json`, filters by status + dep-satisfied + lease-free +
  budget-ok, sorts by priority, and returns a `ScheduleDecision` that the
  Runner can dispatch without re-implementing the filter.

Both follow the `devkit/repairer.py` style: pure functions, dataclasses,
JSON-Schema validation, stdlib only.

## Gatekeeper

### `GateVerdict` — the boundary object

A typed verdict with two envelopes (matches `gate_verdict.schema.json`,
Draft 2020-12):

```python
@dataclass
class GateVerdict:
    api_version: str       # "loom.dev/v1"
    kind: str              # "GateVerdict"
    metadata: dict         # {id, run_id, work_item_id, timestamp, schema_version?}
    spec: dict             # {evidence_source, passed, reason, failure_codes}
```

Convenience accessors (not on disk, just for ergonomics):
`verdict_id`, `run_id`, `work_item_id`, `timestamp`, `evidence_source`,
`passed`, `reason`, `failure_codes`.

### `classify_evidence_source(packet)`

```python
from devkit import gatekeeper

gatekeeper.classify_evidence_source({"spec": {"source": "inner_sandbox"}})
# → "inner_sandbox"

gatekeeper.classify_evidence_source({"spec": {"source": "garbage"}})
# → "unknown"
```

Returns one of: `inner_sandbox`, `materialized_repo`, `external_signal`,
`unknown`. Anything not in the enum is normalised to `unknown`; the
classifier never raises.

### `evaluate_final_gate(run_id, work_item_id, evidence_dir)`

```python
from devkit import gatekeeper
import pathlib

verdict = gatekeeper.evaluate_final_gate("r-001", "wi-1", pathlib.Path("devkit/runs/r-001"))
# verdict.passed          # bool
# verdict.evidence_source # one of the 4 enum values
# verdict.reason          # "all_gates_passed" or descriptive
# verdict.failure_codes   # [] when passed, ≥1 entry when failed
```

`evidence_dir` is the directory that contains `evidence.json`. If the
file is missing, the verdict is `passed=False, failure_codes=["EVIDENCE_MISSING"]`.

When the evidence file exists, the gate runs these checks in order (any
failure short-circuits with that code; multiple failures are not
aggregated into one verdict — the first one wins):

| Trigger | Failure code |
|---|---|
| `evidence.json` missing or unreadable | `EVIDENCE_MISSING` |
| `test_results.failed > 0` | `TEST_REGRESSION` |
| `cost_usd > budget_cap_usd` | `BUDGET_EXCEEDED` |
| `artifact_manifest` missing required `source` field | `EVIDENCE_INVALID` |

If all four pass, the verdict is `passed=True, reason="all_gates_passed"`,
`failure_codes=[]`.

### Verdict persistence

```python
from devkit import gatekeeper
import pathlib

verdict = gatekeeper.evaluate_final_gate("r-001", "wi-1", pathlib.Path("devkit/runs/r-001"))

# Atomic write (writes to <path>.tmp then renames).
written = gatekeeper.write_verdict(verdict, pathlib.Path("devkit/runs/r-001/verdict.json"))

# Round-trip: returns None if the file is missing or invalid JSON.
loaded = gatekeeper.load_verdict(pathlib.Path("devkit/runs/r-001/verdict.json"))
```

`write_verdict` accepts either a `GateVerdict` instance or a raw dict; a
raw dict is validated against `gate_verdict.schema.json` first.

### Verdict validation

```python
gatekeeper.validate_verdict(payload)  # raises jsonschema.ValidationError on bad input
```

Useful for boundary checks when receiving verdicts from outside the
process (e.g. a future MCP gate endpoint).

### Schema

`devkit/protocol_schemas/gate_verdict.schema.json` — Draft 2020-12. The
`spec.failure_codes` enum mirrors the four failure codes
(`EVIDENCE_MISSING`, `TEST_REGRESSION`, `BUDGET_EXCEEDED`,
`EVIDENCE_INVALID`).

## Scheduler

### `ScheduleDecision` — the boundary object

A typed next-task hint:

```python
@dataclass
class ScheduleDecision:
    work_item_id: str           # "" when no work
    reason: str                 # see enum below
    blocked_by: list[str]       # dep ids not yet done/skipped
    estimated_cost_usd: float
    priority: str               # "high" | "medium" | "low"
```

`reason` is one of:

| Value | Meaning |
|---|---|
| `ready` | Selected. Deps satisfied, budget ok, no lease conflict. |
| `no_pending_tasks` | Backlog has no `status=pending` rows. |
| `blocked` | A pending task exists but its deps aren't satisfied. `blocked_by` lists the dep ids. |
| `leased` | A pending task exists but is currently leased by another run. |
| `over_budget` | A pending task's estimated cost exceeds the cap. |
| `backlog_missing` | `backlog.json` missing or unreadable. |

`is_actionable()` returns `True` only when `work_item_id` is set **and**
`reason == "ready"`. This is the canonical check before dispatching.

### `select_next_pending(backlog_path, lease_path=None)`

```python
from devkit import scheduler
import pathlib

decision = scheduler.select_next_pending(pathlib.Path("devkit/backlog.json"))
if decision.is_actionable():
    run(decision.work_item_id)
else:
    log(decision.reason)
```

Filter pipeline (in order):

1. **Pending only** — drop everything not `status=pending`.
2. **Dep check** — keep only tasks whose `deps` are all `done`/`skipped`
   in the same backlog.
3. **Lease check** — drop the task whose id matches an unexpired lease
   (if `lease_path` is provided).
4. **Budget check** — keep only tasks whose estimated cost fits under the
   per-task cap (uses `devkit/budget.py`'s existing cap configuration).
5. **Priority sort** — `high > medium > low`, ties broken by id for
   determinism.

Returns `None` when no task survives the pipeline. The caller should
treat `None` as "no work right now" — same semantics as a `ScheduleDecision`
with `reason="no_pending_tasks"`.

### `list_blocked(backlog_path)`

```python
blocked = scheduler.list_blocked(pathlib.Path("devkit/backlog.json"))
# → list[ScheduleDecision] with reason="blocked" and blocked_by populated
```

Useful for surfacing "why isn't anything happening?" answers to a
dashboard.

### Lease lifecycle

```python
import pathlib
from devkit import scheduler

lease_path = pathlib.Path("devkit/runs/r-001/lease.json")

# Claim — atomic write. Returns False if a non-stale lease exists.
ok = scheduler.claim_lease("wi-1", "r-001", lease_path)

# Check staleness (default 5min). True if lease exists and is older than
# max_age_s.
stale = scheduler.is_lease_stale(lease_path)              # default 300s
stale = scheduler.is_lease_stale(lease_path, max_age_s=600)  # override

# Release — best-effort; missing file is not an error.
scheduler.release_lease(lease_path)
```

Lease format on disk:

```json
{
  "work_item_id": "wi-1",
  "run_id": "r-001",
  "claimed_at": "2026-07-05T03:50:00+00:00",
  "epoch_ms": 1783219800000
}
```

`claim_lease` uses an atomic `.tmp + rename` write so a crashed write
never leaves a half-written file.

## Verified end-to-end (smoke test, 2026-07-05)

```text
[1] classify: 4 enum values + unknown fallback, no raises
[2] gate missing evidence.json → passed=False, failure_codes=['EVIDENCE_MISSING']
[3] gate with synthetic evidence → passed=True, reason='all_gates_passed', source='inner_sandbox'
[4] select_next_pending with mixed priorities + 1 blocked + 1 leased → returns eligible high-priority
[5] list_blocked → returns the dep-blocked task with blocked_by=['dep-x']
[6a] claim_lease → True
[6b] re-claim within 5min → False
[6c] is_lease_stale on fresh lease → False
[6d] release_lease → file removed, no error
```

All 159 tests pass (`test_gatekeeper_scheduler 52/52`, `test_repairer
51/51`, `test_observer_triager 27/27`, `test_rdloop_spec_integration
29/29`).

## Known limitations

1. **Does not replace `iterate.py`'s `run_loop`**. The scheduler is a
   `select_next_pending`-style seam — the autopilot's outer loop still
   lives in `iterate.py` and reads `backlog.json` ad-hoc.
2. **Does not replace the rdloop runner gate**. `evaluate_final_gate`
   returns a `GateVerdict` but the rdloop runner's gate logic
   (`Runner._gate_decision` or equivalent) does not consult it yet. Today
   the gatekeeper is reachable as a library function but not invoked by
   the running autopilot.
3. **No automatic verdict emission**. The autopilot must explicitly call
   `evaluate_final_gate` after a run; nothing wires it in. A follow-up
   patch in `devkit/__main__.py auto` (or a hook in `iterate.py`) is the
   natural integration point.
4. **No scheduler event stream**. Selecting next-task is a one-shot call;
   there's no observer pattern that fires when the backlog changes.
   Phase E/F's admission + safety-filter modules can sit on top of this
   for backpressure-aware scheduling.
5. **Single-process lease**. Lease files don't coordinate across
   processes (e.g. multiple autopilot daemons on the same machine).
   `claim_lease` is local-atomic only.
6. **No claim-lease path integration with the runner**. The runner must
   `claim_lease` itself before running a work item; there's no
   decorator-style `with scheduler.lease(...)` context manager yet.
7. **Failure-code coverage is narrow**. Four codes today; the docs v2
   blueprint lists 15 failure modes the gatekeeper should eventually
   know about (calibration drift, TOCTOU, verifier drift, …). This
   phase is the foundation; the next phase adds the broader taxonomy.

## Integration status (as-of 2026-07-05)

Both modules ship but **the autopilot does not call them**. This is the
honest status of the Phase D delivery:

| Caller | Uses Gatekeeper? | Uses Scheduler? |
|---|---|---|
| `devkit/__main__.py auto` | ❌ direct gate logic | ❌ direct backlog loop |
| `devkit/iterate.py` | ❌ direct gate logic | ❌ direct backlog loop |
| `devkit/repairer.py` | n/a | n/a |
| `devkit/state_writer.py` | n/a | n/a |
| `devkit/protocol.py` (Phase C) | n/a | n/a |

### Proposed migration (not yet executed)

Step 1 — wire `evaluate_final_gate` into the runner:

```python
# iterate.py (proposed patch)
from devkit import gatekeeper

verdict = gatekeeper.evaluate_final_gate(
    run_id=current_run_id,
    work_item_id=current_wi_id,
    evidence_dir=current_evidence_dir,
)
if not verdict.passed:
    # Branch on verdict.failure_codes[0] for the specific recovery path
    # instead of pattern-matching the run-log.
    handle_gate_failure(verdict)
```

Step 2 — wire `select_next_pending` into the autopilot:

```python
# devkit/__main__.py auto (proposed patch)
from devkit import scheduler

decision = scheduler.select_next_pending(
    backlog_path=backlog_path,
    lease_path=current_lease_path,
)
if not decision.is_actionable():
    log(f"no work: {decision.reason}")
    return
scheduler.claim_lease(decision.work_item_id, current_run_id, current_lease_path)
run(decision.work_item_id)
```

Step 3 — add an ADR under `docs/architecture-decisions/` describing the
separation of concerns: Runner drives execution, Gatekeeper owns
verdict, Scheduler owns selection.

Items (1)–(3) are mechanical but should land together to avoid a
half-migrated state where some paths use the new typed APIs and others
don't.