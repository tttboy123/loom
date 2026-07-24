# S2-W5 Deliverable

- WorkItem: `S2-W5`
- Title: Structured Team Draft Content Core
- Risk: Strict
- Base branch/head: `codex/loom-platform-slice2` at `0a98851`
- Repaired contract SHA-256:
  `ba32fae6a1155916d77a4a1bcd9838693572968b046438318c41532351e4631b`
- Product SHA-256:
  `b2e284c18fea1d03eff3a54f449eef136b1b4454676aa4db7d7b9ea45af95041`
- Repair 1 test SHA-256:
  `4864d4b48543aec73309856d39fec42ee4240e6aff9c8facbf3cd0259ec2d9c0`

## Delivered boundary

The Candidate adds a pure immutable structured Team Draft content snapshot:

- one Main plus one or two SubAgent role selections;
- exact Agent, RuntimeInstance, and Runtime/model coverage of the accepted
  S2-W3 references;
- a validated S2-W1 RuntimeProfile binding for every role;
- Agent-scoped Skill/member/permission subsets;
- a bounded, deterministic, acyclic first-task graph;
- no Main-owned delivery, at least one task, and at least one task per SubAgent;
- explicit acceptance criteria and customer-rule summary;
- bounded approval markers and explicit capability gaps;
- deterministic semantic digest and deep-copy accessors; and
- a pure validation Candidate whose readiness is false whenever a gap exists.

It does not attach content to an S2-W4 revision, accept/reject/expire a Draft,
create Team/Agent/WorkItem/Event records, persist state, allocate capacity, call
a model, execute a Runtime, grant permission, or activate anything.

## Contract review

- Review 1: `FAIL`; main-only/no-task content could become ready and the current
  checkpoint was stale.
- Contract Repair 1: require one or two SubAgents, at least one SubAgent-owned
  task, typed main-only failure, and explicit RED coverage.
- Fresh Repair Review 2: `PASS`; findings: none blocking.

Evidence:

- `.loom-evidence/phase1-slice2/S2-W5/contract-review-1.md`
- `.loom-evidence/phase1-slice2/S2-W5/contract-repair-1.md`
- `.loom-evidence/phase1-slice2/S2-W5/contract-review-2.md`

## Mandatory RED

Command:

```text
go test ./internal/teams -run 'TestTeamDraftContent|TestValidateTeamDraftContent' -count=1
```

Exit code: `1`.

The build failed only on missing frozen S2-W5 types/functions including
`TeamDraftContentInput`, `TeamDraftContentSnapshot`,
`TeamDraftRoleSelection`, and `TeamDraftTaskCandidate`. There was no syntax,
dependency, or environment failure.

## Controller verification

After Repair 1, all commands passed with exit code `0`:

```text
go test ./internal/teams -run 'TestTeamDraftContent|TestValidateTeamDraftContent' -count=1
go test ./internal/teams -count=1
go test -race ./internal/teams -run 'TestTeamDraftContent|TestValidateTeamDraftContent' -count=50
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
```

Additional results:

- `gofmt -d internal/teams/draft_content.go
  internal/teams/draft_content_test.go`: no output
- `git diff --check`: no output
- import boundary: standard library, accepted `internal/runtime`, and accepted
  package-local teams contracts only
- owned product scope: `internal/teams/draft_content.go`,
  `internal/teams/draft_content_test.go`
- branch/head remained `codex/loom-platform-slice2` at `0a98851`

## Coverage and trust-boundary evidence

- Tests prove valid one- and two-SubAgent content, main-only and three-SubAgent
  rejection, exact role coverage, Runtime binding errors, model/set equality,
  and input/accessor mutation isolation including pointer budgets.
- Tests prove non-empty bounded tasks, Main no-delivery, every SubAgent
  assignment, dependency identity, cycle rejection, acceptance criteria, all
  exact/max-plus-one limits, and dependency-edge digest sensitivity.
- Tests prove normalized order-independent digests and semantic changes across
  references, RuntimeProfile, task edge/criterion, rule summary, marker, gap,
  and limits.
- Tests prove gap-driven readiness, zero/tampered/catalog-mismatched snapshot
  rejection, and zero Candidate outputs.
- Runtime compatibility is a snapshot-time Candidate check only. It neither
  reserves capacity nor authenticates or starts a process.
- Task candidates are not WorkItems. Criteria are requirements, not passed
  Evidence. Approval markers are annotations, not approvals or Grants.
- No credential, environment value, filesystem path, Provider output, process,
  Event write, external mutation, or execution authority enters the snapshot.

## Implementation review

Review 1 found no product correctness/security defect but required a positive
one-SubAgent readiness case, task dependency-edge digest proof, and this
deliverable before commit.

Evidence:
`.loom-evidence/phase1-slice2/S2-W5/implementation-review-1.md`

Fresh Repair 1 implementation review returned `PASS` with no findings.

Evidence:
`.loom-evidence/phase1-slice2/S2-W5/implementation-review-2.md`

VERDICT: PASS
