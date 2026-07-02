# Request 001: External-Project Devflow Capability Alignment

status: pending_scan  
owner: Buddys  
target: Loom upstream  
type: generalized capability request, not direct patch

## 1. Problem

Buddys can already use Loom as an external runner, but the full development flow still needs Buddys-side wrappers for several recurring gaps:

1. external task files and source packs need extra delivery logic
2. `verify` may terminate with an empty body while still being treated as a completed stage
3. partial `build/` outputs are not enough to decide whether a real project change is actually shippable
4. long-running queue execution currently depends on Buddys-side loop glue instead of a more native external-project intake model
5. final closeout still needs Buddys-side result normalization because stage evidence, gate signals, and failure classes are not consistently materialized

These are no longer just Buddys product issues. They are gaps in how an external project can hand Loom a bounded dev task and trust the execution lifecycle.

## 2. Why this should be upstream, not Buddys-only

This request should only be accepted if Loom agrees the capability is general:

- many users will want Loom to execute tasks defined outside the Loom repo
- many users need better semantics for verify/review completion than “stage file exists”
- many users need deterministic closeout packets for automation, daemons, or queue runners
- many users need a first-class way to distinguish:
  - model answered usefully
  - model answered empty / reasoning-only / malformed
  - executor produced artifacts but failed to materialize required targets

This request does **not** ask Loom to adopt:

- Buddys doc hierarchy
- Buddys task ids
- Buddys product concepts
- Buddys model policy as Loom global default

## 3. Requested capability surface

### A. External task source intake

Loom should support an explicit external-project task intake contract:

- task file path
- optional source pack attachment or file-body injection
- declared target files / required output files
- per-run model carrier and cascade overrides
- optional post-run verifier hooks or acceptance adapters

The key point is that external projects should not need ad hoc wrappers just to deliver enough bounded source context.

### B. Strong verify semantics

Loom should distinguish these cases in a machine-readable way:

1. verify completed with a substantive verdict
2. verify returned empty content
3. verify only returned reasoning / thinking content
4. verify carrier failed upstream
5. verify artifact file exists but is not acceptance-grade

An empty verify stage must not look identical to a valid verify closeout.

### C. Materialization-aware execution

Loom should support a first-class notion of “required materialized outputs”:

- if a task claims to modify `a.py` and `b_test.py`, the run should be able to declare those required outputs
- absence of those outputs should be a first-class closeout signal, not an afterthought in external wrappers

This does not require Loom to understand every downstream repo, but it should understand when its own execution did not materialize the contracted targets.

### D. Deterministic result packet

Loom should provide a normalized closeout artifact for every run with at least:

- run id
- stage completion summary
- carrier actually used by each stage
- gate recommendation
- failure classification
- missing required outputs
- empty-stage diagnostics
- retry / iterate summary

This is needed for external queue runners and long-running autonomy.

### E. Long-running queue / daemon friendliness

Loom does not need to own Buddys backlog, but it should be friendlier to external daemon loops:

- clear claim / running / terminal state transitions
- stable polling surface
- deterministic “no ready task” / “blocked” / “startup stall” signaling
- less need for caller-side shadow capture and result reclassification

## 4. Non-goals

This request does not ask Loom to:

- become Buddys' canonical state system
- import Buddys docs as a default behavior
- store Buddys product memory
- ship Buddys private model cascade as global config
- replace downstream repo-specific acceptance tests

## 5. Acceptance criteria for Loom intake

Loom should only take this request if it can aim for something like:

1. an external project can submit a task plus bounded source context without custom patching of Loom core
2. empty verify / reasoning-only verify are explicitly classified
3. missing required outputs are surfaced as first-class result fields
4. a normalized run result packet exists without downstream log scraping
5. queue/daemon callers can tell the difference between:
   - no task
   - blocked task
   - startup stall
   - implement complete but verify failed
   - review no-go

## 6. Buddys evidence behind this request

The generalized ask is grounded in repeated Buddys-side observations:

- source pack delivery had to be invented outside Loom
- result sync had to normalize shadow/live/archive evidence outside Loom
- overlay validation had to compensate for partial-build acceptance mismatch
- verify-on-MiniMax produced empty-body artifacts that looked superficially complete
- bounded daemon execution needed Buddys-side watchdog and backlog glue

These are evidence inputs, not requirements that Loom copy Buddys architecture.

## 7. Suggested upstream decomposition

If Loom accepts this request, the work should likely split into independent upstream tasks:

1. external task source contract
2. verify/review artifact quality semantics
3. required output materialization contract
4. normalized result packet
5. daemon/queue runner state model

That sequencing keeps Loom generic and reduces the chance of absorbing Buddys-specific assumptions.
