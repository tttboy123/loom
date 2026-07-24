# S2-W9 Deliverable

- WorkItem: `S2-W9`
- Title: Explicit Agent-Mode Team Resolver
- Risk: Strict
- Base branch/head: `codex/loom-platform-slice2` at `af5f158`
- Contract SHA-256:
  `8cd136b1cab71b4298d637637d3d4fd9d2f3708ae340269b6bbdc80db1da6333`
- Product SHA-256:
  `dc47d76602f1ad78fad1bed3dd81da01f62771f2e5734c5f84295bc06dd5033e`
- Test SHA-256:
  `4ece2a60d6c2398a9c355b5b9f819586f385c2954e60df195623c2e8bba6fec9`
- Repair 1 contract SHA-256:
  `c3e098083919d43e488bb2724526f575cd600263ea5725655c053eaf0b328650`

## Delivered boundary

The Candidate adds a pure explicit Agent-mode Team Resolver:

- accepted S1 `mode.Route` remains the sole Agent-mode gate;
- selected saved Team returns `load_team`;
- selected catalogued Main returns `direct_main`;
- selected non-Main returns default-Main-plus-selected-SubAgent `draft_seed`;
- targetless use-Agent follows project default, reusable default, then
  default-Main seed; and
- assign resolves Team-only/Agent-only and rejects zero or ambiguous matches.

Catalogs revalidate current AgentDefinition, RuntimeProfile, and TeamDefinition
sources before resolution. Free-form intent text is neither copied nor hashed.

The Candidate is routing data only. It does not create or mutate a TeamDefinition
or Team Draft, bind a RuntimeInstance, create TeamInstance/AgentInstance/
WorkItem resources, persist state, or execute anything.

## Contract review

Fresh independent contract review returned `PASS` with no blocking findings.

Evidence:
`.loom-evidence/phase1-slice2/S2-W9/contract-review.md`

## Mandatory RED

Command:

```text
go test ./internal/teams -run 'TestResolveAgentModeTeam' -count=1
```

Exit code: `1`.

The build failed only on missing frozen S2-W9 symbols. There was no syntax,
dependency, or environment failure.

## Controller verification

All commands passed with exit code `0`:

```text
go test ./internal/teams -run 'TestResolveAgentModeTeam' -count=1
go test ./internal/teams -count=1
go test -race ./internal/teams -run 'TestResolveAgentModeTeam' -count=50
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
```

Additional results:

- `gofmt -d internal/teams/resolver.go internal/teams/resolver_test.go`: no
  output
- `git diff --check`: no output
- `find . -type d -name '__pycache__' -print`: no output
- imports: standard library plus accepted `internal/agents`, `internal/mode`,
  and `internal/runtime` contracts only
- accepted S1 and S2-W1 through S2-W8 product/test files remain unchanged
- branch/head remained `codex/loom-platform-slice2` at `af5f158`

## Coverage and trust-boundary evidence

- Tests prove Router gating, caller-forced mode rejection, and free-text
  exclusion from Candidate/digest.
- Tests prove selected Team project precedence, selected Main, selected
  non-Main, project/reusable/default-Main use-Agent paths.
- Tests prove assign Team-only, Agent-only, not-found, and ambiguity behavior.
- Tests reject malformed context/target, invalid Main/default catalogs, and
  invalid/duplicate Agent/Profile sources, invalid Team sources, selected
  source not-found, and wrong-scope defaults with zero Candidate output.
- Tests prove catalog reorder determinism, deep copy isolation, and digest
  sensitivity across every frozen semantic field.
- Static imports exclude persistence, live Runtime binding, process, network,
  filesystem, environment, and goroutine surfaces.

## Implementation review

Initial fresh independent implementation review returned `FAIL` because direct
test proof was missing for seven frozen failure paths; no product correctness or
security defect was identified.

Evidence:
`.loom-evidence/phase1-slice2/S2-W9/implementation-review-1.md`

Repair 1 changed tests and evidence only. It added direct zero-Candidate proof
for selected Team/Agent not-found, invalid/duplicate AgentDefinition catalogs,
duplicate RuntimeProfile catalogs, and project/reusable defaults whose saved
Team scope is wrong. The product digest is unchanged. The full strict matrix
passes with the repaired test digest.

Repair contract:
`.loom-evidence/phase1-slice2/S2-W9/repair-1-contract.md`

Fresh independent Repair 1 implementation review returned `PASS` with no
blocking findings. It confirmed every Repair 1 path reaches the exact
zero-Candidate assertion, no product defect remains, and scope/import
boundaries hold.

Evidence:
`.loom-evidence/phase1-slice2/S2-W9/implementation-review-2.md`

VERDICT: PASS
