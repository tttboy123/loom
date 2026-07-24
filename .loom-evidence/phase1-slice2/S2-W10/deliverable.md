# S2-W10 Deliverable

- WorkItem: `S2-W10`
- Title: Accepted Draft Instantiation Plan Candidate
- Risk: Strict
- Base branch/head: `codex/loom-platform-slice2` at `c5e9eed`
- Repaired contract SHA-256:
  `342e9ed09cdb0a638d4a9f5dfcbdab8e79e1d9f42ac7315c910d443e7e58874f`
- Product SHA-256:
  `da7797469cf7a43d92f99e04be6ffbaa99f8645022f2995da67c0b7f6c52f0be`
- Test SHA-256:
  `66c48820fba5dc6cbb58d8f8256bf57f568c06beb89af4d1fc5718c1fff2a72f`
- Implementation Repair 1 contract SHA-256:
  `6ee2a878b607a82dc04a901604a47666511a23af6eeb4fad17a3d91a65ef3915`

## Delivered boundary

The Candidate adds a pure immutable plan for one exact accepted S2-W7 Draft:

- accepted `confirm_and_start` decision validation remains the authority gate;
- exact Draft/catalog/content/binding/decision digests remain bound;
- one Main and one or two SubAgent role seeds preserve accepted Profile,
  online RuntimeInstance, Skill, member, and permission selections;
- the accepted task DAG becomes copied WorkItem seeds without allocating
  runtime resource IDs;
- Main owns no delivery task and every SubAgent owns at least one task;
- requested budget/concurrency remain distinct from catalog ceilings, with
  zero budget allowed and concurrency positive; and
- customer-rule and approval metadata remain bound into the plan digest.

The Candidate does not allocate or create TeamInstance, AgentInstance,
WorkItem, Run, AgentGrant, process, workspace, Event, transaction, or persistent
state and introduces no Slice 3 behavior.

## Contract reviews

Contract Review 1 returned `FAIL`: requested budget/concurrency were omitted and
confused with catalog ceilings.

Contract Repair 1 preserved requested values separately, but Repair 1 review
returned `FAIL`: it incorrectly required requested budget to be positive.

Contract Repair 2 restored the accepted invariant—non-negative budget including
zero, positive concurrency, and both upper ceilings. Fresh independent Contract
Review 3 returned `PASS` with no blocking findings.

Evidence:

- `.loom-evidence/phase1-slice2/S2-W10/contract-review-1.md`
- `.loom-evidence/phase1-slice2/S2-W10/contract-repair-1.md`
- `.loom-evidence/phase1-slice2/S2-W10/contract-review-2.md`
- `.loom-evidence/phase1-slice2/S2-W10/contract-repair-2.md`
- `.loom-evidence/phase1-slice2/S2-W10/contract-review-3.md`

## Mandatory RED

Command:

```text
go test ./internal/teams -run
'TestBuildAcceptedDraftInstantiationPlan|TestValidateAcceptedDraftInstantiationPlan'
-count=1
```

Exit code: `1`.

The build failed only on missing frozen S2-W10 symbols. There was no syntax,
dependency, or environment failure.

## Controller verification

All commands passed with exit code `0`:

```text
go test ./internal/teams -run
'TestBuildAcceptedDraftInstantiationPlan|TestValidateAcceptedDraftInstantiationPlan'
-count=1
go test ./internal/teams -count=1
go test -race ./internal/teams -run
'TestBuildAcceptedDraftInstantiationPlan|TestValidateAcceptedDraftInstantiationPlan'
-count=50
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
```

Additional results:

- `gofmt -d internal/teams/instantiation_plan.go
  internal/teams/instantiation_plan_test.go`: no output
- `git diff --check`: no output
- `find . -type d -name '__pycache__' -print`: no output
- imports: standard library plus accepted `internal/runtime` contract only
- accepted Slice 1 and S2-W1 through S2-W9 product/test files remain unchanged
- branch/head remained `codex/loom-platform-slice2` at `c5e9eed`

## Coverage and trust-boundary evidence

- Tests prove exact accepted success for one and two SubAgents and zero requested
  budget success.
- Tests prove rejected/expired/zero/tampered/catalog/content/binding and
  requested-value validation failures return zero output.
- Tests prove negative budget, budget overflow, and concurrency overflow
  propagate the accepted typed catalog failures.
- Tests prove exact role and task mapping, Main coordination-only semantics,
  every-SubAgent assignment, source binding, annotations, and ceilings.
- Tests prove deterministic digest sensitivity across source, roles, tasks,
  requested values, ceilings, rules, and approval markers.
- Tests prove deep-copy isolation for RuntimeProfile internals and every
  returned mutable slice.
- Static imports exclude persistence, Journal, allocation, process, network,
  filesystem, environment, goroutine, grant, and execution surfaces.

## Implementation review

Fresh independent implementation Review 1 returned `FAIL` with no product
correctness or security defect. Direct proof was incomplete for complete role
selection/no-widening, all required digest fields, and reference-mismatch
propagation.

Implementation Repair 1 changes tests and evidence only. It directly compares
every Main/SubAgent Profile/Runtime/Skill/member/permission selection, adds all
missing digest-field mutations, and adds exact
`ErrStructuredTeamDraftReferenceMismatch` zero-output proof. The product digest
is unchanged and the complete strict matrix passes again.

Evidence:

- `.loom-evidence/phase1-slice2/S2-W10/implementation-review-1.md`
- `.loom-evidence/phase1-slice2/S2-W10/implementation-repair-1-contract.md`

Fresh independent Implementation Repair 1 review returned `PASS` with no
blocking findings. It confirmed all prior proof gaps are closed, no product
defect remains, and scope/import boundaries hold.

Evidence:
`.loom-evidence/phase1-slice2/S2-W10/implementation-review-2.md`

VERDICT: PASS
