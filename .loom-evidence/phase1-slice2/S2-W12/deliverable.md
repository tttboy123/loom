# S2-W12 Deliverable

- WorkItem: `S2-W12`
- Title: Direct Saved-Team Instantiation Plan
- Risk: Strict
- Base branch/head: `codex/loom-platform-slice2` at `d5850f2`
- Contract SHA-256:
  `7660be0d3fe4ddbdc8119a1b8c279de52f981a53c3d701f4918002002042a664`
- Product SHA-256:
  `e404fbee52deec03a561c336b794a400c8af5083133489e007c8400deef0207a`
- Test SHA-256:
  `8b2f1570e3a5719615a527fa2ce272e7f1627c5269024f5260b1129e43880c82`

## Delivered boundary

The Candidate adds a pure immutable direct-instantiation plan for one exact
saved Team selected through accepted S2-W9 `load_team` routing:

- the current S2-W9 resolution is re-run and must resolve an exact saved Team;
- the exact S2-W11 Runtime binding is revalidated against current Team, Agent,
  RuntimeProfile, discovery, and selection sources;
- resolution and binding Team identity, version, scope, scope identity, and
  definition digest must match;
- exactly one TeamInstance and one Main AgentInstance are planned as seeds;
- zero, one, or two saved SubAgent bindings remain normalized dormant
  candidates; and
- no Team Draft, resource ID, WorkItem, Run, grant, workspace, process, state
  write, reservation, or execution is created.

## Contract review

Fresh independent contract review returned `PASS` with no blocking findings.

Evidence:
`.loom-evidence/phase1-slice2/S2-W12/contract-review.md`

## Mandatory RED

The focused command exited `1` only because frozen S2-W12 types, functions,
errors, and helpers were missing. There was no test syntax, dependency, existing
code, or environment failure.

## Controller verification

All commands passed with exit code `0`:

```text
go test ./internal/teams -run
'TestBuildSavedTeamInstantiationPlan|TestValidateSavedTeamInstantiationPlan|TestSavedTeamInstantiationPlan'
-count=1
go test ./internal/teams -count=1
go test -race ./internal/teams -run
'TestBuildSavedTeamInstantiationPlan|TestValidateSavedTeamInstantiationPlan|TestSavedTeamInstantiationPlan'
-count=50
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
```

Additional results:

- `gofmt -d internal/teams/saved_team_instantiation.go
  internal/teams/saved_team_instantiation_test.go`: no output
- `git diff --check`: no output
- `find . -type d -name '__pycache__' -print`: no output
- imports: standard library plus accepted `internal/agents`, `internal/mode`,
  and `internal/runtime` contracts only
- accepted Slice 1 and S2-W1 through S2-W11 product/test files remain unchanged
- branch/head remained `codex/loom-platform-slice2` at `d5850f2`

## Coverage and trust-boundary evidence

- Tests prove explicit Team selection, project and reusable defaults, Team-only
  assign, reusable-default selection under a same-ID project shadow, and
  Main-only/one/two dormant-SubAgent cardinality.
- Tests cover every non-`load_team`, malformed, missing, and ambiguous routing
  path with zero output.
- Tests prove exact Main and dormant bindings, resolution/binding mismatch,
  intent-text exclusion, catalog/Team/discovery/selection reorder determinism,
  source/accessor isolation, and every semantic digest field.
- Validation tests prove zero, digest-tampered, route-changed,
  binding/source-changed, dormant-active, and nonzero-WorkItem plans fail with
  zero validation output.
- Static imports exclude allocation, persistence, Journal, process, network,
  filesystem, environment, goroutine, grant, and execution surfaces.

## Implementation review

Fresh independent implementation review returned `PASS` with no blocking
findings. It confirmed the exact `load_team`/binding boundary, same-ID reusable
Team shadow handling, independent typed source errors, dormant SubAgent
semantics, digest and immutability coverage, and absence of resource/task/state/
execution surfaces.

Evidence:
`.loom-evidence/phase1-slice2/S2-W12/implementation-review.md`

VERDICT: PASS
