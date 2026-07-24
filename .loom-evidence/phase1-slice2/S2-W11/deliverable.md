# S2-W11 Deliverable

- WorkItem: `S2-W11`
- Title: Saved Team Runtime Binding Candidate
- Risk: Strict
- Base branch/head: `codex/loom-platform-slice2` at `88eea03`
- Contract SHA-256:
  `502af7baee6b9cccbbf59d692738b4e4ee0a4050e8ba1ea32d8b52c34217dacc`
- Product SHA-256:
  `406e31133db71fd89bce6511ec66fe25c30f740687f26e2497cf615a48398560`
- Test SHA-256:
  `4e6c61b03ead0db1e7045c0fa5fb42fa76e162ba81756dcf87a3dde29be3312d`
- Repair 1 contract SHA-256:
  `ebca693774c794fa1d718086fdbf146b52bc832c27ac16329c4a6b182fb3ea62`

## Delivered boundary

The Candidate re-resolves one exact saved TeamDefinition and validates an
explicit RuntimeInstance selection for every role:

- project/reusable/latest saved-Team resolution remains S2-W8 authoritative;
- every exact Team Agent/Profile binding is preserved;
- every RuntimeInstance comes from the exact current S2-W2 discovery snapshot;
- accepted S2-W1 `runtime.ValidateBinding` checks online status, adapter, and
  required capabilities;
- discovered model inventory must contain the exact Profile model;
- shared Runtime capacity must cover the number of selected bindings; and
- no capacity is reserved or mutated.

The Candidate creates no TeamInstance, AgentInstance, WorkItem, Run, grant,
workspace, process, Event, transaction, or persistent state.

## Contract review

Fresh independent contract review returned `PASS` with no blocking findings.

Evidence:
`.loom-evidence/phase1-slice2/S2-W11/contract-review.md`

## Mandatory RED

The focused command exited `1` only because frozen S2-W11 symbols were missing.
There was no syntax, dependency, or environment failure.

## Controller verification

All commands passed with exit code `0`:

```text
go test ./internal/teams -run
'TestBuildSavedTeamRuntimeBinding|TestValidateSavedTeamRuntimeBinding|TestSavedTeamRuntimeBinding'
-count=1
go test ./internal/teams -count=1
go test -race ./internal/teams -run
'TestBuildSavedTeamRuntimeBinding|TestValidateSavedTeamRuntimeBinding|TestSavedTeamRuntimeBinding'
-count=50
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
```

Additional results:

- `gofmt -d internal/teams/saved_team_binding.go
  internal/teams/saved_team_binding_test.go`: no output
- `git diff --check`: no output
- `find . -type d -name '__pycache__' -print`: no output
- imports: standard library plus accepted `internal/agents` and
  `internal/runtime` contracts only
- accepted Slice 1 and S2-W1 through S2-W10 product/test files remain unchanged
- branch/head remained `codex/loom-platform-slice2` at `88eea03`

## Coverage and trust-boundary evidence

- Tests prove Main-only, one-SubAgent, and two-SubAgent exact binding.
- Tests prove project precedence, latest version, source digest binding, and
  catalog/selection reorder determinism.
- Tests cover incomplete/duplicate/extra selections, missing Runtime, offline/
  incompatible/disabled status, adapter/capability/model mismatch, invalid
  Profile catalog, and shared-capacity failure with zero Candidate.
- Tests prove capacity is not mutated.
- Tests cover every frozen digest semantic field, accessor isolation, zero/
  digest-invalid/source-mismatched validation, and zero validation output.
- Static imports exclude allocation, persistence, Journal, process, network,
  filesystem, environment, goroutine, grant, and execution surfaces.

## Implementation review

Fresh independent implementation Review 1 returned `FAIL`: unselected
discovery observations were not fully revalidated, and direct validation/
catalog/archive failure proof was incomplete.

Repair 1 now revalidates every observation through accepted
`runtime.NewRuntimeInstance` plus model-inventory rules before selection.
Tests add changed-selection/discovery/capacity validation, duplicate Agent/
Profile catalogs, malformed selection, duplicate discovery rejection, and
archived exclusion. The complete strict matrix passes again.

Evidence:

- `.loom-evidence/phase1-slice2/S2-W11/implementation-review-1.md`
- `.loom-evidence/phase1-slice2/S2-W11/repair-1-contract.md`

Fresh independent Repair 1 implementation review returned `PASS` with no
blocking findings. It confirmed the product gap and all requested proof gaps
are closed, with scope/import boundaries intact.

Evidence:
`.loom-evidence/phase1-slice2/S2-W11/implementation-review-2.md`

VERDICT: PASS
