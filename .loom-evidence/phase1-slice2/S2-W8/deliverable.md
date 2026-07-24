# S2-W8 Deliverable

- WorkItem: `S2-W8`
- Title: Immutable Saved TeamDefinition Core
- Risk: Strict
- Base branch/head: `codex/loom-platform-slice2` at `ab88c5c`
- Contract SHA-256:
  `1c02f1e05d1c43ec057bd62955fcc1084f6f0098e79ac02fcd7675ddad81072e`
- Repair 1 product SHA-256:
  `e93944c34fd36f65dfb983e8a6d4e0891a612942d2b7087bda6cd658cc7e8b3d`
- Repair 1 test SHA-256:
  `f45213d5ce3c1f51c3578f406ceb754a0cc18880f9ba2e4bbc92b1a494cfc846`

## Delivered boundary

The Candidate adds a pure immutable saved TeamDefinition contract:

- exactly one Main and zero, one, or two Phase 1 SubAgent roles;
- stable active AgentDefinition and valid RuntimeProfile references;
- no RuntimeInstance, online/capacity/device/heartbeat state;
- deterministic normalized role order and semantic digest;
- project-over-reusable, exact-scope, latest-version resolution; and
- a pure complete-team load Candidate.

Main-only is valid only at this saved-definition layer. It does not weaken the
accepted S2-W5 rule that an execution-ready generated Draft must contain
SubAgent-owned delivery work.

The Candidate does not route Mode input, choose a default Main, create or mutate
a Team Draft, create TeamInstance/AgentInstance/WorkItem resources, persist
state, bind a live RuntimeInstance, or execute anything.

## Contract review

Fresh independent contract review returned `PASS` with no blocking findings.

Evidence:
`.loom-evidence/phase1-slice2/S2-W8/contract-review.md`

## Mandatory RED

Command:

```text
go test ./internal/teams -run 'TestTeamDefinition|TestResolveTeamDefinition|TestValidateTeamDefinition' -count=1
```

Exit code: `1`.

The build failed only on missing frozen S2-W8 symbols. There was no syntax,
dependency, or environment failure.

## Controller verification

All commands passed with exit code `0`:

```text
go test ./internal/teams -run 'TestTeamDefinition|TestResolveTeamDefinition|TestValidateTeamDefinition' -count=1
go test ./internal/teams -count=1
go test -race ./internal/teams -run 'TestTeamDefinition|TestResolveTeamDefinition|TestValidateTeamDefinition' -count=50
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
```

Additional results:

- `gofmt -d internal/teams/team_definition.go
  internal/teams/team_definition_test.go`: no output
- `git diff --check`: no output
- `find . -type d -name '__pycache__' -print`: no output
- imports: standard library plus accepted `internal/agents` and
  `internal/runtime` contracts only
- accepted S2-W1 through S2-W7 product/test files remain unchanged
- branch/head remained `codex/loom-platform-slice2` at `ab88c5c`

## Coverage and trust-boundary evidence

- Tests prove valid Main-only, one-SubAgent, and two-SubAgent definitions,
  normalized roles, exact Agent/Profile binding, profile switching, and deep
  copy isolation.
- Tests reject all identity/scope/status/cardinality/role/catalog/reference/
  duplicate failures with zero output.
- Tests prove project/reusable precedence, exact scope, latest version, archive
  exclusion, reorder invariance, duplicate winner, invalid/noncanonical
  Candidate, and not-found resolution.
- Tests prove validation rejects zero, role/digest tamper, and current catalog
  reference removal with zero Candidate output.
- Tests prove digest determinism and sensitivity to every frozen semantic field.
- Static imports exclude persistence, RuntimeInstance binding, mode routing,
  process, network, filesystem, environment, and goroutine surfaces.

## Implementation review

Review 1 found two bounded defects: duplicate non-winners blocked resolution,
and the zero SubAgent accessor could panic.

Evidence:
`.loom-evidence/phase1-slice2/S2-W8/implementation-review-1.md`

Repair 1 makes duplicate detection winner-scoped, makes the zero accessor safe,
adds exact regression coverage, and passes the complete strict matrix again.
Fresh independent Repair 1 implementation review returned `PASS` with no
findings.

Evidence:
`.loom-evidence/phase1-slice2/S2-W8/implementation-review-2.md`

VERDICT: PASS
