# S2-W3 Deliverable

- WorkItem: `S2-W3`
- Title: Bounded Team Draft Catalog Snapshot
- Branch: `codex/loom-platform-slice2`
- Repaired contract SHA256:
  `a2f70c717fb88e67163b81661cd78486995f8f0cf2976af405146aac5b21926e`

## Contract Evidence

- Contract Review 1: preserved `FAIL`; blocker was ambiguity in Agent and
  model count units.
- Repair 1: `PASS`; clarified post-resolution/post-filter count units.
- Contract Review 2: `PASS`; no blocking findings against repaired contract.

## Mandatory RED

```text
go test ./internal/teams -run 'TestTeamDraftCatalog|TestValidateTeamDraftCatalog' -count=1
exit_code=1
failure=solely undefined frozen S2-W3 catalog symbols
```

The RED failure was limited to missing `TeamDraftCatalogInput`,
`TeamDraftCatalogSnapshot`, catalog entries, references, typed errors, and
builder/validator symbols. No syntax, environment, dependency, or unrelated
repository failure was reported by Controller.

## Candidate Files

```text
f001d06dfeb1d0e52ba43322b23038d69ddce4f4691c610421c2c7da844746f7  internal/teams/catalog.go
62ec708045ecab257af90483efe50f33c4a8c5be0e3e9415786d01311dfdc431  internal/teams/catalog_test.go
```

## Verification

Controller full verification is `GREEN`.

```text
focused=exit 0
package=exit 0
focused_race_count_50=exit 0
impact=exit 0
repository_race=exit 0
vet=exit 0
gofmt=PASS
diff=PASS
import_boundary=PASS
scope=PASS
```

## Behavior Summary

S2-W3 creates a pure bounded catalog snapshot from accepted AgentDefinition
Candidates, a successful Runtime discovery snapshot, and caller-supplied
workspace Skill, member, permission, budget, concurrency, and maximum-count
Candidates.

Selected-count semantics are repaired and frozen as:

- Agent maximum counts selected catalog entries after all definitions are
  validated and each stable ID is resolved by accepted scope/version rules.
- Runtime maximum counts selectable Runtime entries after filtering observations
  to status exactly `online`.
- Model maximum counts Runtime-scoped `(runtime_instance_id, model_id)` pairs
  under online Runtime entries; the same model string under two Runtime IDs
  counts twice.
- Skill, member, and permission maxima count normalized unique sets after
  empty/duplicate validation.
- Exactly `max` entries are accepted; `max + 1` fails closed without truncation.

The snapshot keeps private state, returns deep-copy accessors, and computes a
deterministic lowercase SHA-256 digest over selected Agent entries, online
Runtime/model entries, the upstream discovery digest, normalized workspace
sets, all maximum counts, budget ceiling, and concurrency ceiling.

Reference validation returns a Candidate-only result on success and rejects
empty, duplicate, invented Agent/Runtime/model/Skill/member/permission
references, Runtime/model cross-pair mismatches, budget overflow, and
concurrency overflow with typed errors inspectable through `errors.Is`.

S2-W3 does not create a TeamDraft revision, choose a default Main Agent, load or
instantiate a Team, create a TeamInstance, bind or execute a Runtime, allocate
capacity, persist state, write Events, grant permissions, launch processes, or
activate autonomous execution.

VERDICT: PASS
