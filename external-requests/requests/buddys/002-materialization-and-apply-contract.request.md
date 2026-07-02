# Request 002: Materialization And Apply Contract For External Projects

status: pending_scan  
owner: Buddys  
target: Loom upstream  
type: generalized capability request, not direct patch

## 1. Problem

When Loom executes a bounded external-project task, a run may produce a plausible implementation artifact while still failing to materialize the contracted repo outputs into the caller's actual target tree.

Observed generic failure classes:

1. stage artifacts imply a useful patch exists, but the contracted target files are absent
2. a run is functionally good in a shadow/build area, but no durable apply result exists for the caller repo
3. the caller cannot tell whether the executor:
   - never wrote the target files
   - wrote them only in a temporary area
   - declined to apply because of an internal lock or apply policy
   - partially applied and stopped

For external-project automation, "implementation text exists" is not enough. The caller needs a first-class materialization and apply contract.

## 2. Why this should be upstream

This is not Buddys-specific:

- any external repo can require exact output paths
- any queue/daemon caller needs to know whether a run changed the intended tree
- many users will separate execution sandbox from acceptance repo and still need deterministic apply semantics

This request does **not** ask Loom to understand downstream business logic. It asks Loom to report what happened to the contracted output surface.

## 3. Requested capability surface

### A. Required output materialization

A run should be able to declare:

- required output paths
- optional write scope
- whether full-file rewrites are acceptable or minimal patching is expected

Loom should then report whether each required output was:

- materialized
- missing
- partially materialized
- blocked before apply

### B. Apply outcome classification

Every run should expose a machine-readable apply outcome such as:

- `applied`
- `not_applied`
- `apply_blocked`
- `apply_partial`
- `apply_not_attempted`

If apply is blocked by an internal lock or policy, that should be explicit.

### C. Durable output handoff

If Loom intentionally separates generated build artifacts from the caller repo, it should expose a durable handoff surface that external callers can trust without scraping prose:

- exact generated files
- exact target mapping
- whether those files are ready for apply
- any blockers that prevented apply

## 4. Non-goals

This request does not ask Loom to:

- auto-merge arbitrary downstream repos
- bypass safety policy silently
- understand repo-specific acceptance tests
- copy Buddys overlay validation design

## 5. Acceptance criteria for Loom intake

Loom should only take this request if it can aim for:

1. explicit required-output declarations for external tasks
2. explicit machine-readable apply outcome classification
3. explicit missing-output reporting without downstream log scraping
4. durable handoff artifacts that external callers can consume safely

## 6. Buddys evidence behind this request

This generalized request is grounded in repeated external-project observations:

- required build outputs were sometimes missing even when stage prose looked usable
- callers had to reconstruct "effective build" state outside Loom
- final closeout sometimes required a coordinator to cross the last apply boundary manually

Those are evidence inputs only. Loom should solve the generic external-project problem, not import Buddys-specific machinery.
