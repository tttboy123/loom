# S3-W5 Contract Review 1

- Date: `2026-07-26`
- Reviewer: `/root/s2_w2_contract_review`
- Mode: fresh independent read-only

## Findings

### P1 — touched create/assign lost global Run identity protection

The frozen contract allowed create/assign to replay only its WorkItem stream.
Accepted S3-W2 behavior instead discovers every prior assignment through full
Journal replay and rejects a Run ID already bound to another WorkItem. Without
a replacement global Run identity mechanism, two WorkItems could create the
same Run lineage.

Required repair: freeze a Journal-authoritative global Run identity reservation
and compatibility path, or retain a sufficient global read for create/assign.

### P1 — authority-bearing APIs were not exact

Only `journal.ReadStreamSet` had an exact signature. The Grant identity
initializer, Team dispatch/aggregation writer, planner, GlobalReadView, and
application coordinator crossed package or write-authority boundaries without
exact exported symbols, inputs, results, and typed errors.

Required repair: freeze the exact public surface and prohibit unlisted
authority-bearing exports, matching the strictness of accepted S3-W2.

## Preserved constraints

The Reviewer confirmed that the contract otherwise kept:

- one final S3-W5 and no W6;
- the reviewed Grant identity and 16-head repairs;
- the Event batch limit of 32;
- compatibility-only accepted test ownership;
- no checkpoint/cache/activation or second authority; and
- the TECH-PLAN Main/SubAgent, generation, Evidence, recovery, and canary
  targets.

`git diff --check` passed. No files were edited and no long verification was
run by the Reviewer.

VERDICT: FAIL
