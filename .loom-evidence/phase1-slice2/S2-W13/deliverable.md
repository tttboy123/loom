# S2-W13 Deliverable

- WorkItem: `S2-W13`
- Title: Saved-Team Instance Record Set Candidate
- Risk: Strict
- Base branch/head: `codex/loom-platform-slice2` at `e8ddc82`
- Contract SHA-256:
  `0d3b38f6427e808ae92afcdb90c459fe3b1395560c943b49bae13469239d96a9`
- Product SHA-256:
  `491a268806168025a0fbfde95b6a2d453ac5d68d0eb573eefd066cd01fa89ac0`
- Test SHA-256:
  `8cc31b7ae6ca1d87d7f44151ad910a95b7aed6618fd08049c55acc7b0feadc86`

## Delivered boundary

The Candidate revalidates one exact S2-W12 saved-Team plan and prepares the
complete domain record set a later atomic writer may commit:

- caller-supplied WorkRequest, TeamInstance, Main AgentInstance, and creation
  time identities are validated but never allocated;
- the exact current Main AgentDefinition ID/version/scope/scope identity is
  resolved from the saved Team scope;
- the exact accepted Main RuntimeProfile/RuntimeInstance binding is copied;
- one TeamInstance and one Main AgentInstance record are produced;
- zero, one, or two saved SubAgents remain normalized dormant records; and
- a deterministic record-set digest binds all record and count fields.

The Candidate writes no SQLite row or Event, creates no WorkItem, Run, Evidence,
grant, workspace, or process, reserves no capacity, and executes nothing.

## Contract review

Fresh independent contract review returned `PASS` with no blocking findings.
It confirmed this is a necessary post-plan, pre-writer boundary and that the
contract-local `created` state does not claim persistence or execution.

Evidence:
`.loom-evidence/phase1-slice2/S2-W13/contract-review.md`

## Mandatory RED

The focused command exited `1` only because frozen S2-W13 types, functions, and
errors were missing. There was no test syntax, dependency, existing-code, or
environment failure.

## Controller verification

All commands passed with exit code `0`:

```text
go test ./internal/teams -run
'TestBuildSavedTeamInstanceRecordSet|TestValidateSavedTeamInstanceRecordSet'
-count=1
go test ./internal/teams -count=1
go test -race ./internal/teams -run
'TestBuildSavedTeamInstanceRecordSet|TestValidateSavedTeamInstanceRecordSet'
-count=50
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
```

Additional results:

- `gofmt -d internal/teams/saved_team_instances.go
  internal/teams/saved_team_instances_test.go`: no output
- `git diff --check`: no output
- `find . -type d -name '__pycache__' -print`: no output
- imports: standard library plus accepted `internal/agents`, `internal/mode`,
  and `internal/runtime` contracts only
- accepted Slice 1 and S2-W1 through S2-W12 product/test files remain unchanged
- branch/head remained `codex/loom-platform-slice2` at `e8ddc82`

## Coverage and trust-boundary evidence

- Tests prove project and same-ID-shadowed reusable Team resolution, project
  Team fallback to reusable AgentDefinition, exact resolved Main
  AgentDefinition scope/version, and latest-version selection.
- Tests prove Main-only/one/two dormant-member cardinality always creates one
  Team record, one Main Agent record, zero active SubAgents, and zero WorkItems.
- Tests cover invalid identities/timestamps, changed route/plan/catalog/
  discovery/selection/binding sources, source reorder determinism, and zero
  output on failure.
- Tests cover accessor/source isolation, validation shape/state/count/dormant/
  source/digest failures, and every record-set digest field.
- Static imports exclude allocation, persistence, Journal, process, network,
  filesystem, environment, goroutine, grant, and execution surfaces.

## Implementation review

Fresh independent implementation review returned `PASS` with no blocking
findings. It confirmed complete S2-W12 revalidation, exact actual
AgentDefinition version/scope resolution including both shadow and fallback
cases, caller-supplied identity handling, dormant SubAgent semantics, complete
digest/source/immutability proof, and absence of persistence or execution
surfaces.

Evidence:
`.loom-evidence/phase1-slice2/S2-W13/implementation-review.md`

VERDICT: PASS
