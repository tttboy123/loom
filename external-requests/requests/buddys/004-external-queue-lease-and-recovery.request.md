# Request 004: External Queue Lease And Recovery Semantics

status: pending_scan  
owner: Buddys  
target: Loom upstream  
type: generalized capability request, not direct patch

## 1. Problem

When Loom is used as an external execution engine inside a long-running queue loop, the caller needs stronger semantics for `claimed` / `running` / terminal state than a plain backlog status bit.

Observed generic failure classes:

1. a task is marked `running`, but no live executor process exists
2. a run starts and stalls before a durable execution record is written
3. a caller cannot tell whether a task is truly in flight, abandoned, or safe to reclaim
4. queue progress appears blocked even though no useful work is happening

This is not a Buddys-only problem. Any external project that layers a resident queue/daemon loop over Loom will hit the same ambiguity.

## 2. Why this should be upstream

This request is generic if Loom wants to serve as an external-project execution engine:

- external callers need a durable notion of task ownership
- long-running loops need reclaim-safe state transitions
- queue health should not depend on caller-side process scraping alone
- terminal vs abandoned runs should be machine-readable

This request does **not** ask Loom to adopt:

- Buddys backlog lanes
- Buddys task ids
- Buddys docs or state files as Loom defaults

## 3. Requested capability surface

### A. Lease-backed running state

Loom should expose a generic lease model for claimed/running tasks, including:

- run id
- claim timestamp
- heartbeat timestamp
- lease timeout or expiry time
- terminal release semantics

### B. Stale-run recovery classification

Loom should classify recoverable failure states such as:

- `running_without_live_executor`
- `startup_stalled_before_execution_record`
- `lease_expired_reclaimable`
- `terminal_run_not_reconciled`

### C. Queue-safe polling surface

External callers should be able to ask, in a machine-readable way:

- is there a live run?
- is the current running state authoritative?
- is the task reclaimable?
- is the queue blocked, empty, or just cooling down?

### D. Deterministic recovery rules

If Loom intends to support daemon-like use, it should define when a caller may safely:

- reopen a stale `running` item
- retry after startup stall
- treat a missing run record as recoverable

## 4. Non-goals

This request does not ask Loom to:

- own downstream backlog schemas
- replace downstream acceptance gates
- absorb Buddys discovery catalogs or docs
- hardcode one project's retry policy

## 5. Acceptance criteria for Loom intake

Loom should only take this request if it can aim for:

1. durable `running` state tied to a run identity rather than a loose backlog bit
2. heartbeat or lease semantics suitable for external queue loops
3. machine-readable stale-run classifications
4. deterministic reclaim/recovery semantics for abandoned or startup-stalled runs

## 6. Buddys evidence behind this request

The generalized request is grounded in repeated queue symptoms:

- tasks could remain `running` even when no active executor process existed
- external callers had to scrape process state to detect zombie runs
- queue progress could appear blocked until a downstream coordinator manually reset backlog status

These are evidence inputs only. Loom should solve the generic external-queue problem, not copy Buddys wrapper logic.
