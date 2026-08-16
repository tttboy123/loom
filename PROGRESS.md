# Loom Phase 1 Slice 1 Progress

Updated: 2026-07-25

## Execution truth

- Workspace: `/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild`
- Slice 1 completion commit: `5861f82`
- Current branch: `codex/loom-platform-slice2`
- Existing `codex/loom-platform` branch was not moved or overwritten
- Preserved pre-existing untracked paths: `.codex/installation_id`,
  `.codex/skills/`, `.loom-drafts/`
- Slice 2: S2-W1 through S2-W38 plus S2-EXIT-1 accepted and locally committed;
  latest accepted commit `46eefaf`
- Push, merge, release, activation, credential changes, paid remote work, and
  FastContext installation: prohibited

## WorkItems

| WorkItem | Risk | Status | Gate |
|---|---|---|---|
| S1-W1 | Standard | PASS | RED, GREEN, impact, race, vet, reviewer PASS |
| S1-W2 | Strict | PASS | RED, two repairs, strict GREEN, fresh reviewer PASS |
| S1-W3 | Strict | PASS | L2 bounded repair GREEN + fresh reviewer PASS |
| S1-W4 | Strict | PASS | Two repairs, strict GREEN, fresh scoped reviewer PASS |
| S1-W5 | Standard | PASS | One repair, GREEN, fresh reviewer PASS |

## Completed evidence

- S1-W1: `.loom-evidence/phase1-slice1/S1-W1/deliverable.md`
- S1-W2: `.loom-evidence/phase1-slice1/S1-W2/deliverable.md`
- S1-W3 historical failed lineage:
  `.loom-evidence/phase1-slice1/S1-W3/deliverable.md`
- S1-W3 accepted repair lineage:
  `.loom-evidence/phase1-slice1/S1-W3-L2/deliverable.md`
- S1-W4: `.loom-evidence/phase1-slice1/S1-W4/deliverable.md`
- S1-W5: `.loom-evidence/phase1-slice1/S1-W5/deliverable.md`

## Slice 1 terminal gate

- Final repository `go test ./... -count=1`: PASS
- Final repository `go test -race ./... -count=1`: PASS
- Final `go vet ./...`, build, formatting, and `git diff --check`: PASS
- Five accepted WorkItem deliverables end with `VERDICT: PASS`
- Final evidence:
  `.loom-evidence/phase1-slice1/final-verification.md`
- State: Slice 1 committed at `5861f82`, not activated

## Slice 2 transition

- Branch: `codex/loom-platform-slice2`
- Status: `S3_W3_ACCEPTED`
- Accepted S2-W1 local commit: `954416a`
- Frozen contract:
  `.loom-evidence/phase1-slice2/S2-W1/contract.md`
- Contract SHA256:
  `3b3a54f71419d59ac67c81edface6bcc1dd2ff9c6603bc4c3cce83be4bc599d0`
- Fresh review evidence:
  `.loom-evidence/phase1-slice2/S2-W1/contract-review.md`
- Reviewer verdict: `PASS`; findings: none blocking
- Mandatory RED: exit `1`; missing frozen domain symbols; no
  environment/syntax/dependency failure
- Controller GREEN: focused `0`; package `0`; race `-count=50` `0`; impact
  `0`; repository race `0`; vet `0`; gofmt/diff/scope `PASS`
- Candidate evidence:
  `.loom-evidence/phase1-slice2/S2-W1/deliverable.md`
- Implementation Review 1: `FAIL`; required findings: empty Definition ID
  crossed stable IDs, profile-switching proof was hollow, and commit scope
  classification was incomplete
- Review 1 evidence:
  `.loom-evidence/phase1-slice2/S2-W1/review-1.md`
- Repair 1 contract:
  `.loom-evidence/phase1-slice2/S2-W1/repair-1-contract.md`
- Product repair attempt: `1/3`
- Repair 1 RED: exit `1`; empty Definition ID incorrectly returned success
- Repair 1 GREEN: focused/package/race-50/impact/repository-race/vet all exit
  `0`; gofmt/diff/branch-head/owned-scope/import boundary `PASS`
- Repair 1 product digests:
  `definition.go=97a1918e...699788`,
  `definition_test.go=1881b873...2d82`,
  `catalog.go=a892e6bd...d3a9`,
  `catalog_test.go=b7422099...5ec8`
- Fresh implementation Review 2: `PASS`; findings: none blocking
- Review 2 evidence:
  `.loom-evidence/phase1-slice2/S2-W1/review-2.md`
- Scope classification: four frozen Go files are Developer-owned;
  `docs/CURRENT.md`, `PROGRESS.md`, and S2-W1 evidence are Controller-owned;
  pre-existing `AGENTS.md`, `.codex/`, and `.loom-drafts/` are excluded from
  the S2-W1 commit
- S2-W1 boundary: pure AgentDefinition, RuntimeProfile, RuntimeInstance, scope
  resolution, and side-effect-free compatibility validation only
- Authority correction: Bridge/JSON-RPC/JSONL, AgentGrant, claim generation,
  prepare lease, WorkItem dispatch, and real Runtime Adapter remain Slice 3
  under `TECH-PLAN.md §14`; later research drafts are Candidate material
- S2-W2 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W2/contract.md`
- S2-W2 contract SHA256:
  active normalized `7f8dab95bf7a98d3e7615252fc3d490fa5108fad956b29eea82c3b049e96d3c9`;
  original reviewed `8ab4fdfadda9808a4ed2c0f750efc850d60db48e8f147758c8356e5702d53ed1`
- S2-W2 contract review:
  `.loom-evidence/phase1-slice2/S2-W2/contract-review.md`
- S2-W2 contract Reviewer: `PASS`; findings: none blocking
- S2-W2 EOF-only contract amendment and fresh review:
  `.loom-evidence/phase1-slice2/S2-W2/contract-amendment-1.md`,
  `.loom-evidence/phase1-slice2/S2-W2/contract-amendment-1-review.md`;
  semantic changes: none; Reviewer: `PASS`
- S2-W2 boundary: injected RuntimeProbe coordination, validated model and
  capability observations, deterministic immutable digest snapshot; no concrete
  CLI probe, daemon scheduling, persistence, Runtime execution, or activation
- S2-W2 mandatory RED: exit `1`; missing frozen discovery symbols only
- S2-W2 pre-review regression RED: exit `1`; immutable snapshot accessors
  missing; probe-ID one-time capture coverage added
- S2-W2 product digests:
  `discovery.go=2543d956...0dbc24`,
  `discovery_test.go=9482ce30...0895`
- S2-W2 Controller checks: focused `0`; package `0`; focused race
  `-count=50` `0`; impact `0`; repository race `0`; vet `0`;
  gofmt/diff/import/scope `PASS`
- S2-W2 Candidate:
  `.loom-evidence/phase1-slice2/S2-W2/deliverable.md`
- S2-W2 fresh implementation review:
  `.loom-evidence/phase1-slice2/S2-W2/implementation-review.md`
- S2-W2 implementation Reviewer: `PASS`; findings: none blocking
- Candidate last line: `VERDICT: PASS`
- Accepted S2-W2 local commit: `1170062`
- S2-W3 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W3/contract.md`
- S2-W3 Contract Review 1:
  `.loom-evidence/phase1-slice2/S2-W3/contract-review-1.md`;
  verdict `FAIL`; blocker: Agent and Runtime-scoped model count units ambiguous
- S2-W3 Contract Repair 1:
  `.loom-evidence/phase1-slice2/S2-W3/contract-repair-1.md`;
  selected Agents, online Runtimes, Runtime/model pairs, and normalized sets now
  have explicit `max`/`max + 1` semantics
- S2-W3 Contract Review 2:
  `.loom-evidence/phase1-slice2/S2-W3/contract-review-2.md`;
  verdict `PASS`; findings: none blocking
- S2-W3 boundary: bounded immutable Team Draft catalog plus pure membership and
  ceiling validation; no Draft revision, default Main Agent, Team load,
  TeamInstance, persistence, process, or execution authority
- S2-W3 mandatory RED: exit `1`; missing frozen catalog symbols only
- S2-W3 product digests:
  `catalog.go=f001d06d...746f7`,
  `catalog_test.go=62ec7080...dc431`
- S2-W3 Controller checks: focused `0`; package `0`; focused race
  `-count=50` `0`; impact `0`; repository race `0`; vet `0`;
  gofmt/diff/import/scope `PASS`
- S2-W3 Candidate:
  `.loom-evidence/phase1-slice2/S2-W3/deliverable.md`
- S2-W3 fresh implementation review:
  `.loom-evidence/phase1-slice2/S2-W3/implementation-review.md`
- S2-W3 implementation Reviewer: `PASS`; findings: none
- Candidate last line: `VERDICT: PASS`
- Accepted S2-W3 local commit: `e196107`
- S2-W4 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W4/contract.md`
- S2-W4 boundary: immutable Draft revisions, one unresolved question, catalog
  revalidation, stale command rejection, acceptance eligibility Candidate only;
  no acceptance, TeamInstance, persistence, model call, or execution
- S2-W4 contract SHA256:
  `153f6efc24d724649257a1c68ec80a8e482e368b4d8adc38cb691f1c3645fcfc`
- S2-W4 fresh contract review:
  `.loom-evidence/phase1-slice2/S2-W4/contract-review.md`
- S2-W4 contract Reviewer: `PASS`; findings: none blocking
- S2-W4 mandatory RED: exit `1`; missing frozen Draft symbols only
- S2-W4 product digests:
  `draft.go=dbaf71ac...e0a95`,
  `draft_test.go=15233838...38c75`
- S2-W4 Controller checks: focused `0`; package `0`; focused race
  `-count=50` `0`; impact `0`; repository race `0`; vet `0`;
  gofmt/diff/import/scope `PASS`
- S2-W4 Candidate:
  `.loom-evidence/phase1-slice2/S2-W4/deliverable.md`
- S2-W4 fresh implementation review:
  `.loom-evidence/phase1-slice2/S2-W4/implementation-review.md`
- S2-W4 implementation Reviewer: `PASS`; findings: none blocking
- Candidate last line: `VERDICT: PASS`
- Accepted S2-W4 local commit: `0a98851`
- S2-W5 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W5/contract.md`
- S2-W5 boundary: pure immutable structured Draft content with exact role and
  Runtime binding coverage, bounded first-task DAG, acceptance criteria,
  customer-rule summary, approval markers, capability gaps, and readiness
  Candidate only
- S2-W5 Contract Review 1:
  `.loom-evidence/phase1-slice2/S2-W5/contract-review-1.md`;
  verdict `FAIL`; blockers: main-only/no-task content could become ready and
  the current checkpoint still named S2-W4
- S2-W5 Contract Repair 1: require one or two SubAgents, at least one
  SubAgent-owned task, typed main-only rejection, and explicit RED coverage
- S2-W5 repaired contract SHA256:
  `ba32fae6a1155916d77a4a1bcd9838693572968b046438318c41532351e4631b`
- S2-W5 Contract Review 2:
  `.loom-evidence/phase1-slice2/S2-W5/contract-review-2.md`;
  verdict `PASS`; findings: none blocking
- S2-W5 mandatory RED: exit `1`; missing frozen content symbols only
- S2-W5 product digest:
  `draft_content.go=b2e284c1...f95041`
- S2-W5 Implementation Review 1:
  `.loom-evidence/phase1-slice2/S2-W5/implementation-review-1.md`;
  verdict `FAIL`; no product defect; missing one-SubAgent positive proof,
  dependency-edge digest proof, and pre-commit deliverable
- S2-W5 Repair 1:
  `.loom-evidence/phase1-slice2/S2-W5/repair-1-contract.md`;
  product changes: none; repair attempt `1/3`
- S2-W5 Repair 1 test digest:
  `draft_content_test.go=4864d4b4...2d9c0`
- S2-W5 Repair 1 checks: focused `0`; package `0`; focused race
  `-count=50` `0`; impact `0`; repository race `0`; vet `0`;
  gofmt/diff/import/scope `PASS`
- S2-W5 Candidate:
  `.loom-evidence/phase1-slice2/S2-W5/deliverable.md`;
  last line `VERDICT: PASS`
- S2-W5 fresh Repair 1 implementation review:
  `.loom-evidence/phase1-slice2/S2-W5/implementation-review-2.md`
- S2-W5 Repair 1 Reviewer: `PASS`; findings: none
- Accepted S2-W5 local commit: `567967c`
- S2-W6 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W6/contract.md`
- S2-W6 boundary: pure immutable composition of S2-W4 revisions with validated
  S2-W5 content and binding digest; gaps require a question; eligibility
  requires exact latest gap-free proposed content
- S2-W6 contract SHA256:
  `fca127c6115ab031ea7aef98aebc8eb05ad5f38f89ccf0ee39ad4fdad966ee5c`
- S2-W6 fresh contract review:
  `.loom-evidence/phase1-slice2/S2-W6/contract-review.md`
- S2-W6 contract Reviewer: `PASS`; findings: none blocking
- S2-W6 mandatory RED: exit `1`; missing frozen structured Draft symbols only
- S2-W6 product digest:
  `structured_draft.go=cfa37e43...e1d5`
- S2-W6 Implementation Review 1:
  `.loom-evidence/phase1-slice2/S2-W6/implementation-review-1.md`;
  verdict `FAIL`; no product defect; missing complete answer/edit typed failure
  and zero-output proof
- S2-W6 Repair 1:
  `.loom-evidence/phase1-slice2/S2-W6/repair-1-contract.md`;
  product changes: none; repair attempt `1/3`
- S2-W6 Repair 1 test digest:
  `structured_draft_test.go=019b222c...09768`
- S2-W6 Repair 1 checks: focused `0`; package `0`; focused race
  `-count=50` `0`; impact `0`; repository race `0`; vet `0`;
  gofmt/diff/import/scope `PASS`
- S2-W6 Candidate:
  `.loom-evidence/phase1-slice2/S2-W6/deliverable.md`;
  last line `VERDICT: PASS`
- S2-W6 fresh Repair 1 implementation review:
  `.loom-evidence/phase1-slice2/S2-W6/implementation-review-2.md`
- S2-W6 Repair 1 Reviewer: `PASS`; findings: none
- Accepted S2-W6 local commit: `b810800`
- S2-W7 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W7/contract.md`
- S2-W7 boundary: explicit typed accepted/rejected/expired terminal Draft
  decision over one exact S2-W6 revision; no resource creation or execution
- S2-W7 contract SHA256:
  `44646dd98c830072105469f4f9e52d40711de470f2af948bc091291dbb8308f4`
- S2-W7 fresh contract review:
  `.loom-evidence/phase1-slice2/S2-W7/contract-review.md`
- S2-W7 contract Reviewer: `PASS`; findings: none blocking
- S2-W7 mandatory RED: exit `1`; missing frozen terminal decision symbols only
- S2-W7 product/test digests:
  `draft_decision.go=9966e171...87b77c`,
  `draft_decision_test.go=b5fab077...35c8d0`
- S2-W7 checks: focused `0`; package `0`; focused race `-count=50` `0`;
  impact `0`; repository race `0`; vet `0`; gofmt/diff/import/scope `PASS`
- S2-W7 Candidate:
  `.loom-evidence/phase1-slice2/S2-W7/deliverable.md`;
  last line `VERDICT: PASS`
- S2-W7 fresh implementation review:
  `.loom-evidence/phase1-slice2/S2-W7/implementation-review.md`
- S2-W7 implementation Reviewer: `PASS`; findings: none blocking
- Accepted S2-W7 local commit: `ab88c5c`
- S2-W8 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W8/contract.md`
- S2-W8 boundary: immutable saved TeamDefinition core and pure complete-team
  load Candidate; no default-Main routing, live binding, resource creation, or
  execution
- S2-W8 contract SHA256:
  `1c02f1e05d1c43ec057bd62955fcc1084f6f0098e79ac02fcd7675ddad81072e`
- S2-W8 fresh contract review:
  `.loom-evidence/phase1-slice2/S2-W8/contract-review.md`
- S2-W8 contract Reviewer: `PASS`; findings: none blocking
- S2-W8 mandatory RED: exit `1`; missing frozen TeamDefinition symbols only
- S2-W8 product/test digests:
  `team_definition.go=eacdf87e...379d76`,
  `team_definition_test.go=8d5db7bd...744f1c`
- S2-W8 checks: focused `0`; package `0`; focused race `-count=50` `0`;
  impact `0`; repository race `0`; vet `0`; gofmt/diff/import/scope `PASS`
- S2-W8 Candidate:
  `.loom-evidence/phase1-slice2/S2-W8/deliverable.md`;
  last line `VERDICT: PENDING_REVIEW`
- S2-W8 Implementation Review 1:
  `.loom-evidence/phase1-slice2/S2-W8/implementation-review-1.md`;
  verdict `FAIL`; duplicate non-winners blocked resolution and zero-value
  SubAgent accessor panicked
- S2-W8 Repair 1:
  `.loom-evidence/phase1-slice2/S2-W8/repair-1-contract.md`;
  product repair attempt `1/3`
- S2-W8 Repair 1 product/test digests:
  `team_definition.go=e93944c3...7e8b3d`,
  `team_definition_test.go=f45213d5...cfc846`
- S2-W8 Repair 1 strict checks: focused `0`; package `0`; focused race
  `-count=50` `0`; impact `0`; repository race `0`; vet `0`;
  gofmt/diff/import/scope `PASS`
- S2-W8 fresh Repair 1 implementation review:
  `.loom-evidence/phase1-slice2/S2-W8/implementation-review-2.md`
- S2-W8 Repair 1 Reviewer: `PASS`; findings: none blocking
- S2-W8 Candidate last line: `VERDICT: PASS`
- Accepted S2-W8 local commit: `af5f158`
- S2-W9 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W9/contract.md`
- S2-W9 boundary: explicit Agent-mode Team resolution Candidate only; no Draft,
  instance, persistence, or execution
- S2-W9 contract SHA256:
  `8cd136b1cab71b4298d637637d3d4fd9d2f3708ae340269b6bbdc80db1da6333`
- S2-W9 fresh contract review:
  `.loom-evidence/phase1-slice2/S2-W9/contract-review.md`
- S2-W9 contract Reviewer: `PASS`; findings: none blocking
- S2-W9 mandatory RED: exit `1`; missing frozen Team Resolver symbols only
- S2-W9 product/test digests:
  `resolver.go=dc47d766...d5033e`,
  `resolver_test.go=4ece2a60...6fec9`
- S2-W9 checks: focused `0`; package `0`; focused race `-count=50` `0`;
  impact `0`; repository race `0`; vet `0`; gofmt/diff/import/scope `PASS`
- S2-W9 Candidate:
  `.loom-evidence/phase1-slice2/S2-W9/deliverable.md`;
  last line `VERDICT: PENDING_REVIEW`
- S2-W9 implementation Review 1:
  `.loom-evidence/phase1-slice2/S2-W9/implementation-review-1.md`
- S2-W9 implementation Reviewer: `FAIL`; no product defect; direct
  fail-closed test proof incomplete
- S2-W9 Repair 1/3:
  `.loom-evidence/phase1-slice2/S2-W9/repair-1-contract.md`
- S2-W9 Repair 1 boundary: test/evidence only; no product change
- S2-W9 Repair 1 checks: focused `0`; package `0`; focused race
  `-count=50` `0`; impact `0`; repository race `0`; vet `0`;
  gofmt/diff/import/scope `PASS`
- S2-W9 Repair 1 review:
  `.loom-evidence/phase1-slice2/S2-W9/implementation-review-2.md`
- S2-W9 Repair 1 Reviewer: `PASS`; findings: none blocking
- S2-W9 Candidate last line: `VERDICT: PASS`
- Accepted S2-W9 local commit: `c5e9eed`
- S2-W10 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W10/contract.md`
- S2-W10 boundary: accepted-Draft instantiation plan Candidate only; no
  resource allocation, persistence, or execution
- S2-W10 contract SHA256:
  `72a98b3d02bb9fd2221e5eb56b695bbb3ed1a9c436b9fcd4a6e796f174872327`
- S2-W10 contract Review 1:
  `.loom-evidence/phase1-slice2/S2-W10/contract-review-1.md`
- S2-W10 contract Reviewer: `FAIL`; requested budget/concurrency omitted and
  confused with catalog ceilings
- S2-W10 Contract Repair 1/3:
  `.loom-evidence/phase1-slice2/S2-W10/contract-repair-1.md`
- S2-W10 repaired contract SHA256:
  `5b6e9c660c0a871549da0e45be55d2a2bbb05e54f1e0ad370665a949597917b0`
- S2-W10 Contract Repair 1 SHA256:
  `acd5a921ebdc95597172d8bc82522c31086c9a248de267c9272d357b8f288994`
- S2-W10 Contract Repair 1 review:
  `.loom-evidence/phase1-slice2/S2-W10/contract-review-2.md`
- S2-W10 Contract Repair 1 Reviewer: `FAIL`; zero requested budget was
  incorrectly rejected
- S2-W10 Contract Repair 2/3:
  `.loom-evidence/phase1-slice2/S2-W10/contract-repair-2.md`
- S2-W10 Repair 2 contract SHA256:
  `342e9ed09cdb0a638d4a9f5dfcbdab8e79e1d9f42ac7315c910d443e7e58874f`
- S2-W10 Contract Repair 2 SHA256:
  `362aaeaeec9e42b6cc7515cf78457820ed48fde8da89a3e0ee0ddce164d3d23b`
- S2-W10 Contract Repair 2 review:
  `.loom-evidence/phase1-slice2/S2-W10/contract-review-3.md`
- S2-W10 Contract Repair 2 Reviewer: `PASS`; findings: none blocking
- S2-W10 mandatory RED: exit `1`; missing frozen instantiation-plan symbols only
- S2-W10 product/test digests:
  `instantiation_plan.go=da779746...52f0be`,
  `instantiation_plan_test.go=32852d90...8e222`
- S2-W10 checks: focused `0`; package `0`; focused race `-count=50` `0`;
  impact `0`; repository race `0`; vet `0`; gofmt/diff/import/scope `PASS`
- S2-W10 Candidate:
  `.loom-evidence/phase1-slice2/S2-W10/deliverable.md`;
  last line `VERDICT: PENDING_REVIEW`
- S2-W10 implementation Review 1:
  `.loom-evidence/phase1-slice2/S2-W10/implementation-review-1.md`
- S2-W10 implementation Reviewer: `FAIL`; no product defect; direct role,
  digest, and reference-failure proof incomplete
- S2-W10 Implementation Repair 1/3:
  `.loom-evidence/phase1-slice2/S2-W10/implementation-repair-1-contract.md`
- S2-W10 Repair 1 boundary: test/evidence only; no product change
- S2-W10 repaired product/test digests:
  `instantiation_plan.go=da779746...52f0be`,
  `instantiation_plan_test.go=66c48820...2a72f`
- S2-W10 Repair 1 checks: focused `0`; package `0`; focused race
  `-count=50` `0`; impact `0`; repository race `0`; vet `0`;
  gofmt/diff/import/scope `PASS`
- S2-W10 Repair 1 review:
  `.loom-evidence/phase1-slice2/S2-W10/implementation-review-2.md`
- S2-W10 Repair 1 Reviewer: `PASS`; findings: none blocking
- S2-W10 Candidate last line: `VERDICT: PASS`
- Accepted S2-W10 local commit: `88eea03`
- S2-W11 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W11/contract.md`
- S2-W11 boundary: saved-Team Runtime binding Candidate only; no capacity
  reservation, resource creation, persistence, or execution
- S2-W11 contract SHA256:
  `502af7baee6b9cccbbf59d692738b4e4ee0a4050e8ba1ea32d8b52c34217dacc`
- S2-W11 fresh contract review:
  `.loom-evidence/phase1-slice2/S2-W11/contract-review.md`
- S2-W11 contract Reviewer: `PASS`; findings: none blocking
- S2-W11 mandatory RED: exit `1`; missing frozen saved-Team binding symbols only
- S2-W11 product/test digests:
  `saved_team_binding.go=bee39549...962054`,
  `saved_team_binding_test.go=3dd2c49f...92a458`
- S2-W11 checks: focused `0`; package `0`; focused race `-count=50` `0`;
  impact `0`; repository race `0`; vet `0`; gofmt/diff/import/scope `PASS`
- S2-W11 Candidate:
  `.loom-evidence/phase1-slice2/S2-W11/deliverable.md`;
  last line `VERDICT: PENDING_REVIEW`
- S2-W11 implementation Review 1:
  `.loom-evidence/phase1-slice2/S2-W11/implementation-review-1.md`
- S2-W11 implementation Reviewer: `FAIL`; observation revalidation product gap
  plus validation/catalog/archive test gaps
- S2-W11 Repair 1/3:
  `.loom-evidence/phase1-slice2/S2-W11/repair-1-contract.md`
- S2-W11 repaired product/test digests:
  `saved_team_binding.go=406e3113...398560`,
  `saved_team_binding_test.go=4e6c61b0...e3312d`
- S2-W11 Repair 1 checks: focused `0`; package `0`; focused race
  `-count=50` `0`; impact `0`; repository race `0`; vet `0`;
  gofmt/diff/import/scope `PASS`
- S2-W11 Repair 1 review:
  `.loom-evidence/phase1-slice2/S2-W11/implementation-review-2.md`
- S2-W11 Repair 1 Reviewer: `PASS`; findings: none blocking
- S2-W11 Candidate last line: `VERDICT: PASS`
- Accepted S2-W11 local commit: `d5850f2`
- S2-W12 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W12/contract.md`
- S2-W12 boundary: direct saved-Team instantiation plan Candidate only; Main
  planned, unassigned SubAgents dormant, no resource creation
- S2-W12 contract SHA256:
  `7660be0d3fe4ddbdc8119a1b8c279de52f981a53c3d701f4918002002042a664`
- S2-W12 fresh contract review:
  `.loom-evidence/phase1-slice2/S2-W12/contract-review.md`
- S2-W12 contract Reviewer: `PASS`; findings: none blocking
- S2-W12 mandatory RED: exit `1`; missing frozen direct saved-Team
  instantiation-plan symbols only
- S2-W12 product/test digests:
  `saved_team_instantiation.go=e404fbee...ef0207a`,
  `saved_team_instantiation_test.go=8b2f1570...880c82`
- S2-W12 checks: focused `0`; package `0`; focused race `-count=50` `0`;
  impact `0`; repository race `0`; vet `0`; gofmt/diff/import/scope `PASS`
- S2-W12 deliverable:
  `.loom-evidence/phase1-slice2/S2-W12/deliverable.md`
- S2-W12 fresh implementation review:
  `.loom-evidence/phase1-slice2/S2-W12/implementation-review.md`
- S2-W12 implementation Reviewer: `PASS`; findings: none blocking
- S2-W12 Candidate last line: `VERDICT: PASS`
- Accepted S2-W12 local commit: `e8ddc82`
- S2-W13 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W13/contract.md`
- S2-W13 boundary: pure saved-Team TeamInstance/Main AgentInstance record set;
  dormant SubAgents remain non-active; no state write or execution
- S2-W13 contract SHA256:
  `0d3b38f6427e808ae92afcdb90c459fe3b1395560c943b49bae13469239d96a9`
- S2-W13 fresh contract review:
  `.loom-evidence/phase1-slice2/S2-W13/contract-review.md`
- S2-W13 contract Reviewer: `PASS`; findings: none blocking
- S2-W13 mandatory RED: exit `1`; missing frozen record-set symbols only
- S2-W13 product/test digests:
  `saved_team_instances.go=491a2688...89ac0`,
  `saved_team_instances_test.go=8cc31b7a...eadc86`
- S2-W13 checks: focused `0`; package `0`; focused race `-count=50` `0`;
  impact `0`; repository race `0`; vet `0`; gofmt/diff/import/scope `PASS`
- S2-W13 deliverable:
  `.loom-evidence/phase1-slice2/S2-W13/deliverable.md`
- S2-W13 fresh implementation review:
  `.loom-evidence/phase1-slice2/S2-W13/implementation-review.md`
- S2-W13 implementation Reviewer: `PASS`; findings: none blocking
- S2-W13 Candidate last line: `VERDICT: PASS`
- Accepted S2-W13 local commit: `89dbff3`
- S2-W14 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W14/contract.md`
- S2-W14 boundary: bounded atomic Journal batch append only; no schema,
  Team/Agent payload, projection, resource creation, or execution
- S2-W14 contract SHA256:
  `6618f29e7bc8817624e82e115325cc2c9f754db5e6f2d5d06d708dca9a6383f4`
- S2-W14 fresh contract review:
  `.loom-evidence/phase1-slice2/S2-W14/contract-review.md`
- S2-W14 contract Reviewer: `PASS`; findings: none blocking
- S2-W14 mandatory RED: exit `1`; missing frozen batch symbols only
- S2-W14 product/test digests:
  `store.go=59e6df0f...cbdda1`,
  `batch_test.go=2099c961...1bda5f`
- S2-W14 checks: focused `0`; package `0`; focused race `-count=50` `0`;
  impact `0`; repository race `0`; vet `0`; gofmt/diff/migration/scope `PASS`
- S2-W14 deliverable:
  `.loom-evidence/phase1-slice2/S2-W14/deliverable.md`
- S2-W14 fresh implementation review:
  `.loom-evidence/phase1-slice2/S2-W14/implementation-review.md`
- S2-W14 implementation Reviewer: `PASS`; findings: none blocking
- S2-W14 Candidate last line: `VERDICT: PASS`
- Accepted S2-W14 local commit: `f293a9f`
- S2-W15 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W15/contract.md`
- S2-W15 boundary: exact saved-Team Team/Main StateWriter facts only; no
  projection, WorkItem, process, or execution
- S2-W15 contract SHA256:
  `01f476ffa03f392bd88bb31615005ceb547a54ced4485aa3779ee28b01a0bd74`
- S2-W15 fresh contract review:
  `.loom-evidence/phase1-slice2/S2-W15/contract-review.md`
- S2-W15 contract Reviewer: `PASS`; findings: none blocking
- S2-W15 mandatory RED: failed only on missing frozen S2-W15 symbols
- S2-W15 product:
  `internal/state/saved_team_writer.go`
- S2-W15 tests:
  `internal/state/saved_team_writer_test.go`
- S2-W15 strict matrix: focused, package, focused-race-50, repository,
  repository-race, vet, format, diff, and import/scope checks all `PASS`
- S2-W15 implementation Review 1:
  `.loom-evidence/phase1-slice2/S2-W15/implementation-review-1.md`
- S2-W15 Review 1: `REPAIR`; bounded same-ID shadow and digest-sensitivity
  evidence gaps, with no blocking production boundary finding
- S2-W15 Repair 1 contract:
  `.loom-evidence/phase1-slice2/S2-W15/repair-1-contract.md`
- S2-W15 Repair 1: real simultaneous same-ID project/reusable Team shadow,
  full Event/source sensitivity matrix, and exact payload-byte digest binding
- S2-W15 fresh Repair 1 review:
  `.loom-evidence/phase1-slice2/S2-W15/implementation-review-2.md`
- S2-W15 Repair 1 Reviewer: `PASS`; findings: none blocking
- S2-W15 Candidate last line: `VERDICT: PASS`
- Accepted S2-W15 local commit: `560834a`
- S2-W16 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W16/contract.md`
- S2-W16 boundary: rebuildable Team/Main Agent read-model projection only; no
  schema, CLI, resource, process, WorkItem, or execution
- S2-W16 contract SHA256:
  `2cc5a6a6354dba52228625467f849b4c5ac5dbd58fec2c0aeedd5e4a1a559fb1`
- S2-W16 contract Review 1:
  `.loom-evidence/phase1-slice2/S2-W16/contract-review-1.md`
- S2-W16 contract Review 1: `REPAIR`; bounded S1-W4 projection ownership
  wording conflict only, with no technical contract blocker
- S2-W16 repaired contract review:
  `.loom-evidence/phase1-slice2/S2-W16/contract-review-2.md`
- S2-W16 repaired contract Reviewer: `PASS`; findings: none blocking
- S2-W16 mandatory RED: failed only on missing frozen Snapshot fields/types
- S2-W16 product:
  `internal/projection/projection.go`
- S2-W16 tests:
  `internal/projection/projection_test.go`
- S2-W16 strict matrix: focused, package, focused-race-50, repository,
  repository-race, vet, format, diff, and import/scope checks all `PASS`
- S2-W16 implementation Review 1:
  `.loom-evidence/phase1-slice2/S2-W16/implementation-review-1.md`
- S2-W16 Review 1: `REPAIR`; required zero/empty payload field presence was not
  distinguishable from omission
- S2-W16 Repair 1 contract:
  `.loom-evidence/phase1-slice2/S2-W16/repair-1-contract.md`
- S2-W16 Repair 1: private pointer-backed fact decoding plus missing-field RED
  matrix; public read-model shape unchanged
- S2-W16 fresh Repair 1 review:
  `.loom-evidence/phase1-slice2/S2-W16/implementation-review-2.md`
- S2-W16 Repair 1 Reviewer: `PASS`; findings: none blocking
- S2-W16 Candidate last line: `VERDICT: PASS`
- Accepted S2-W16 local commit: `42fc661`
- S2-W17 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W17/contract.md`
- S2-W17 boundary: Pi-specific version/model metadata command semantics,
  bounded parsing, and S2-W2 `RuntimeProbe` adaptation over an injected narrow
  runner; no executable runner, PATH/home/config/auth/environment inspection,
  process, daemon scheduling, Runtime execution, or activation
- S2-W17 contract SHA256:
  `5216acc1807c6a65ae3a72d365f8026d79a61a062ccb43a63ab061d0b027e162`
- Upstream Pi metadata surface verified read-only on 2026-07-25; current
  `--list-models` startup may run migrations, so the contract explicitly
  forbids direct invocation against user state
- S2-W17 fresh contract review:
  `.loom-evidence/phase1-slice2/S2-W17/contract-review.md`
- S2-W17 contract Reviewer: `PASS`; findings: none blocking
- S2-W17 mandatory RED: exit `1`; failed only on missing frozen S2-W17
  symbols, with no syntax, dependency, environment, or unrelated failure
- S2-W17 product:
  `internal/runtime/pi_probe.go`
- S2-W17 tests:
  `internal/runtime/pi_probe_test.go`
- S2-W17 product SHA256:
  `42f39f37c2df3b824edf2c145bfcf5ef0c3dab428a2d8f3b954e28036e0a59cf`
- S2-W17 test SHA256:
  `2d7f9fdfde5a0a326b55f193c64360efb403a8d0046d415b5f59ac7e749bd2a2`
- S2-W17 strict matrix: focused, package, focused-race-50, repository,
  repository-race, vet, gofmt, diff, import boundary, non-disclosure, and scope
  checks all `PASS`
- S2-W17 Candidate:
  `.loom-evidence/phase1-slice2/S2-W17/deliverable.md`
- S2-W17 fresh implementation review:
  `.loom-evidence/phase1-slice2/S2-W17/implementation-review.md`
- S2-W17 implementation Reviewer: `PASS`; findings: none blocking
- S2-W17 Candidate last line: `VERDICT: PASS`
- Accepted S2-W17 local commit: `b73cf8b`
- S2-W18 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W18/contract.md`
- S2-W18 boundary: exact-request local Pi metadata process runner with pinned
  executable identity/digest, private per-call state, environment allowlist,
  bounded output, timeout/process-group cleanup, and no installed-Pi execution
  during verification
- S2-W18 contract SHA256:
  `cb268df2a6488ae2c194aa0fe9e104f974624e60183637141e5fac6526d359ed`
- S2-W18 contract Review 1:
  `.loom-evidence/phase1-slice2/S2-W18/contract-review-1.md`
- S2-W18 contract Review 1: `REPAIR`; required combined cleanup-error
  precedence, honest original-process-group scope, and explicit
  `/usr/bin/env` interpreter-path residual risk
- S2-W18 Contract Repair 1:
  `.loom-evidence/phase1-slice2/S2-W18/contract-repair-1.md`
- S2-W18 fresh Contract Repair 1 review:
  `.loom-evidence/phase1-slice2/S2-W18/contract-review-2.md`
- S2-W18 repaired-contract Reviewer: `PASS`; findings: none blocking
- S2-W18 initial mandatory RED: failed only on missing frozen runner symbols
- S2-W18 pre-review focused GREEN: `PASS`
- S2-W18 strict matrix: `FAIL` at accepted S2-W1 pure-domain import boundary
  because the initial owned path placed concrete `os/exec` in
  `internal/runtime`
- S2-W18 Contract Amendment 2:
  `.loom-evidence/phase1-slice2/S2-W18/contract-amendment-2.md`
- Amendment 2 moves only the concrete adapter to
  `internal/runtime/piadapter`; no behavior or trust boundary expands
- S2-W18 fresh Amendment 2 review:
  `.loom-evidence/phase1-slice2/S2-W18/contract-review-3.md`
- S2-W18 Amendment 2 Reviewer: `PASS`; findings: none blocking
- S2-W18 amended product/test digests:
  `process_runner.go=45bd8520...a65f`,
  `process_unix.go=554e828a...140`,
  `process_other.go=ea9a2da1...e7a`,
  `process_runner_test.go=897c4f49...25e5`
- S2-W18 amended Candidate checks: focused `0`; package `0`; focused race
  `-count=20` `0`; repository `0`; repository race `0`; vet `0`; gofmt, diff,
  pure-parent import boundary, direct-exec/environment, branch/head, and
  owned-scope checks `PASS`
- S2-W18 Candidate:
  `.loom-evidence/phase1-slice2/S2-W18/deliverable.md`
- S2-W18 fresh implementation review:
  `.loom-evidence/phase1-slice2/S2-W18/implementation-review.md`
- S2-W18 implementation Reviewer: `PASS`; findings: none blocking
- S2-W18 Candidate last line: `VERDICT: PASS`
- Accepted S2-W18 local commit: `8c8fb9e`
- S2-W19 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W19/contract.md`
- S2-W19 contract SHA256:
  `fdf5cfad9c8f1c28fe0817d2f3bc98604a7dc78cca3d73688150c9763ac22b12`
- S2-W19 boundary: configured fixed-name `pi` search and accepted S2-W18/
  S2-W17 probe construction only; no ambient PATH, process during
  construction, daemon scheduling, Event persistence, or activation
- S2-W19 current upstream source confirms executable name `pi`
- S2-W19 fresh contract review:
  `.loom-evidence/phase1-slice2/S2-W19/contract-review.md`
- S2-W19 contract Reviewer: `PASS`; findings: none blocking
- S2-W19 mandatory RED: exit `1`; failed only on missing frozen factory,
  config, request, and typed-error symbols
- S2-W19 product/test digests:
  `local_probe.go=d6eec967...6cbb1`,
  `local_probe_test.go=5c823ed8...b27c1`
- S2-W19 strict matrix: focused `0`; package `0`; focused race `-count=20`
  `0`; repository `0`; repository race `0`; vet `0`; gofmt, diff,
  fixed-name/no-PATH/no-process boundary, parent purity, branch/head, and scope
  checks `PASS`
- S2-W19 Candidate:
  `.loom-evidence/phase1-slice2/S2-W19/deliverable.md`
- S2-W19 fresh implementation review:
  `.loom-evidence/phase1-slice2/S2-W19/implementation-review.md`
- S2-W19 implementation Reviewer: `PASS`; findings: none blocking
- S2-W19 Reviewer transient: one repository non-race run failed while run
  concurrently with repository-race; isolated rerun and two ten-run
  reproductions passed, so it was not confirmed as a product blocker
- S2-W19 Candidate last line: `VERDICT: PASS`
- Accepted S2-W19 local commit: `1b2c486`
- S2-W20 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W20/contract.md`
- S2-W20 contract SHA256:
  `d403da481f25754d144f3721e696d21ba67479626b2a91c1551f6f87a199e0b6`
- S2-W20 boundary: atomic canonical `RuntimeInstanceDiscovered` Event batch
  for one non-empty accepted S2-W2 snapshot; no discovery execution, absence/
  status inference, projection, scheduler, daemon, or activation
- S2-W20 fresh contract review:
  `.loom-evidence/phase1-slice2/S2-W20/contract-review.md`
- S2-W20 contract Reviewer: `PASS`; findings: none blocking
- S2-W20 mandatory RED: exit `1`; failed only on missing frozen writer, input,
  Candidate, and typed-error symbols
- S2-W20 product/test digests:
  `runtime_discovery_writer.go=35740067...cea6c`,
  `runtime_discovery_writer_test.go=f4358ab9...5e701`
- S2-W20 strict matrix: focused `0`; package `0`; focused race `-count=30`
  `0`; repository `0`; repository race `0`; vet `0`; gofmt, diff, Event
  payload/order, SQLite atomicity/conflict, import, branch/head, and scope
  checks `PASS`
- S2-W20 Candidate:
  `.loom-evidence/phase1-slice2/S2-W20/deliverable.md`
- S2-W20 Implementation Review 1:
  `.loom-evidence/phase1-slice2/S2-W20/implementation-review-1.md`
- S2-W20 Implementation Review 1: `FAIL`; same-instant non-UTC `EmittedAt`
  values were not rejected as an exact appender-result mismatch
- S2-W20 Repair 1 contract:
  `.loom-evidence/phase1-slice2/S2-W20/repair-1-contract.md`
- S2-W20 Repair 1 contract SHA256:
  `6cead182bd9c872cbedc386c3596580610a24b8b2b608d2dc9fad67dc17a596a`
- S2-W20 Repair 1 RED: exit `1`; the previous writer returned no error for the
  non-UTC timestamp-location mutation
- S2-W20 Repair 1: requires UTC locations on both compared Events; no Event,
  payload, digest, journal, source-validation, or scope semantics changed
- S2-W20 Repair 1 strict matrix: focused `0`; package `0`; focused race
  `-count=30` `0`; repository `0`; repository race `0`; vet `0`; gofmt, diff,
  import, branch/head, and scope checks `PASS`
- S2-W20 Repair Review 2:
  `.loom-evidence/phase1-slice2/S2-W20/implementation-review-2.md`
- S2-W20 Repair Review 2: `FAIL`; Repair 1 product gap closed, but frozen
  deadline-context proof was missing
- S2-W20 Repair 2 contract:
  `.loom-evidence/phase1-slice2/S2-W20/repair-2-contract.md`
- S2-W20 Repair 2 contract SHA256:
  `b717d5ee61c9c012f849908f7432c8b2c75008409110de36143023ba0d9c699a`
- S2-W20 Repair 2: test-only proof for `context.DeadlineExceeded`, zero
  Candidate, and zero append calls; product hash unchanged
- S2-W20 Repair 2 strict matrix: focused `0`; package `0`; focused race
  `-count=30` `0`; repository `0`; repository race `0`; vet `0`; gofmt, diff,
  import, branch/head, and scope checks `PASS`
- S2-W20 Repair 2 fresh implementation review:
  `.loom-evidence/phase1-slice2/S2-W20/implementation-review-3.md`
- S2-W20 Repair 2 Reviewer: `PASS`; findings: none blocking
- S2-W20 Candidate last line: `VERDICT: PASS`
- Accepted S2-W20 local commit: `501ac33`
- S2-W21 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W21/contract.md`
- S2-W21 contract SHA256:
  `9061a1c1c40aa3be77a85933a77627af8b93a57cc75ea787996faff37a13cfbb`
- S2-W21 boundary: rebuildable latest Runtime inventory from committed
  `RuntimeInstanceDiscovered` Events; no absence/status inference,
  `RuntimeInstanceStatusChanged`, discovery execution, write, scheduling, or
  activation
- S2-W21 Contract Review 1:
  `.loom-evidence/phase1-slice2/S2-W21/contract-review-1.md`
- S2-W21 Contract Review 1: `REPAIR`; caller-owned Event ID, idempotency key,
  and correlation metadata lacked an authority for arbitrary changed values
- S2-W21 repaired metadata rule: reject emptiness and accepted replay
  conflicts; accept fresh alternate nonempty unique caller-owned values
- S2-W21 fresh repaired-contract review:
  `.loom-evidence/phase1-slice2/S2-W21/contract-review-2.md`
- S2-W21 repaired-contract Reviewer: `PASS`; findings: none blocking
- S2-W21 Contract Amendment 1:
  `.loom-evidence/phase1-slice2/S2-W21/contract-amendment-1.md`
- S2-W21 Contract Amendment 1 SHA256:
  `275ac81c5b1511f26f4a2f50c18fdf198af813753376d6fe89d7bc288936309a`
- S2-W21 Amendment 1: executable version is projected exactly and may be empty,
  matching accepted `runtime.NewRuntimeInstance`; no product change started
- S2-W21 fresh Amendment 1 contract review:
  `.loom-evidence/phase1-slice2/S2-W21/contract-review-3.md`
- S2-W21 Amendment 1 Reviewer: `PASS`; findings: none blocking
- S2-W21 mandatory RED: exit `1`; failed only on missing frozen
  `RuntimeInstances`, projection-local `RuntimeInstance`, and apply behavior
- S2-W21 product/test digests:
  `projection.go=1c9e0bd2...02206`,
  `projection_test.go=9e507713...a29c`,
  `runtime_discovery.go=e74677aa...5a751`,
  `runtime_discovery_test.go=07e35017...c8c64`
- S2-W21 strict matrix: focused `0`; package `0`; impact `0`; focused race
  `-count=30` `0`; repository `0`; repository race `0`; vet `0`; gofmt, diff,
  import, non-disclosure, identity, branch/head, and scope checks `PASS`
- S2-W21 Candidate:
  `.loom-evidence/phase1-slice2/S2-W21/deliverable.md`
- S2-W21 fresh implementation review:
  `.loom-evidence/phase1-slice2/S2-W21/implementation-review.md`
- S2-W21 implementation Reviewer: `PASS`; findings: none blocking
- S2-W21 Candidate last line: `VERDICT: PASS`
- Accepted S2-W21 local commit: `366bc48`
- S2-W22 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W22/contract.md`
- S2-W22 contract SHA256:
  `5f8f2a1e06950d927925fb49b7c70f7c177ebf1229193d47b30cff603e989de9`
- S2-W22 boundary: pure observed status transitions for matching stable
  baseline/current identities; no new/absent-ID inference, Event persistence,
  projection, discovery execution, scheduling, or activation
- S2-W22 fresh contract review:
  `.loom-evidence/phase1-slice2/S2-W22/contract-review.md`
- S2-W22 contract Reviewer: `PASS`; findings: none blocking
- S2-W22 mandatory RED: exit `1`; failed only on missing frozen baseline,
  transition, Candidate, reconciliation, and digest symbols
- S2-W22 product/test digests:
  `status_reconciliation.go=c81658d7...2c3ee`,
  `status_reconciliation_test.go=5b9fc0b0...e6e84`
- S2-W22 strict matrix: focused `0`; package `0`; impact `0`; focused race
  `-count=50` `0`; repository `0`; repository race `0`; vet `0`; gofmt, diff,
  pure-domain import, no-write/no-probe/no-execution, branch/head, and scope
  checks `PASS`
- S2-W22 implementation Review 1:
  `.loom-evidence/phase1-slice2/S2-W22/implementation-review-1.md`
- S2-W22 implementation Review 1: `FAIL`; no product defect; missing direct
  baseline-set addition digest proof and independent same-status non-status
  inventory-change proof
- S2-W22 Repair 1 contract:
  `.loom-evidence/phase1-slice2/S2-W22/repair-1-contract.md`
- S2-W22 Repair 1 contract SHA256:
  `8461af09f56b0de5bb5220b80c8fcdeb5157497fbec930d45f2d9dae08fc3f9d`
- S2-W22 Repair 1: tests only; product digest remains exactly
  `c81658d781889d4b0e240539db45bc93cd9579263797184d59eb28a40e12c3ee`
- S2-W22 Repair 1 proof: baseline-set addition changes both baseline and
  Candidate digests while reorder remains stable; independent display name,
  executable version, capabilities, capacity, model IDs, and source probe
  changes remain valid zero-transition same-status Candidates
- S2-W22 Repair 1 strict matrix: focused `0`; package `0`; impact `0`; focused
  race `-count=50` `0`; repository `0`; repository race `0`; vet `0`; gofmt,
  diff, product-hash, and scope checks `PASS`
- S2-W22 transient matrix note: one concurrent full-repository run hit only the
  pre-existing three-second Pi metadata timeout; the target passed sequentially
  `-count=20`, full repository passed sequentially, and repository race passed
- S2-W22 Candidate:
  `.loom-evidence/phase1-slice2/S2-W22/deliverable.md`
- S2-W22 fresh Repair 1 implementation review:
  `.loom-evidence/phase1-slice2/S2-W22/implementation-review-2.md`
- S2-W22 Repair 1 implementation Reviewer: `PASS`; findings: none blocking;
  both test-proof gaps closed; product unchanged; transient concurrent Pi
  timeout independently not reproduced and does not affect verdict
- S2-W22 Candidate last line: `VERDICT: PASS`
- Accepted S2-W22 local commit: `b0cf75f`
- S2-W23 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W23/contract.md`
- S2-W23 contract SHA256:
  `9c8669e9db0c42a77a67dc31f5674d0fbb1fc4ea0c6f532c18eb6d68f0963767`
- S2-W23 boundary: one non-empty accepted S2-W22 Candidate to an exact atomic
  batch of canonical `RuntimeInstanceStatusChanged` Events; no discovery,
  reconciliation, absence inference, projection, scheduling, daemon, or
  activation
- S2-W23 Contract Review 1:
  `.loom-evidence/phase1-slice2/S2-W23/contract-review-1.md`
- S2-W23 Contract Review 1: `REPAIR`; sequence greater than previous could
  append a gap that projection replay rejects and admit a repeated stale fact
- S2-W23 Contract Repair 1:
  `.loom-evidence/phase1-slice2/S2-W23/contract-repair-1.md`
- S2-W23 repaired contract SHA256:
  `2a5cb17ad5b4a4e6353c2fffabb5107c6e8b274b34c27d64ab2bdbdd9df1613a`
- S2-W23 Contract Repair 1 SHA256:
  `8fe530cad6d866b3be50d8f80d2a905960aa0720552aafb174ae242ef4fda1c5`
- S2-W23 Repair 1: require exact `PreviousSequence + 1`; lower/equal/higher
  values fail before append; Journal proves occupied exact-next rejection
- S2-W23 fresh repaired-contract review:
  `.loom-evidence/phase1-slice2/S2-W23/contract-review-2.md`
- S2-W23 repaired-contract Reviewer: `PASS`; findings: none blocking
- S2-W23 mandatory RED: exit `1`; failed only on missing frozen writer, input,
  commit Candidate, and sentinel symbols before the product file existed
- S2-W23 product/test digests:
  `runtime_status_writer.go=ed7f217f...05659`,
  `runtime_status_writer_test.go=b29f878a...a7a4a`
- S2-W23 strict matrix: focused `0`; state/runtime/journal/projection impact
  `0`; focused race `-count=30` `0`; repository `0`; repository race `0`; vet
  `0`; gofmt, diff, import, non-disclosure, exact-next, real-Journal conflict,
  and scope checks `PASS`
- S2-W23 transient matrix note: the first repository-race run hit only the
  existing Pi process-group cleanup marker timing test; that focused race test
  passed `-count=10`, repository race then passed sequentially, and no Pi file
  changed
- S2-W23 Candidate:
  `.loom-evidence/phase1-slice2/S2-W23/deliverable.md`
- S2-W23 fresh implementation review:
  `.loom-evidence/phase1-slice2/S2-W23/implementation-review.md`
- S2-W23 implementation Reviewer: `PASS`; findings: none blocking; complete
  strict matrix independently passed; Pi cleanup transient not reproduced
- S2-W23 Candidate last line: `VERDICT: PASS`
- Accepted S2-W23 local commit: `1ba3238`
- S2-W24 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W24/contract.md`
- S2-W24 contract SHA256:
  `4e5e0ce7b75b432c7c8c11ba87a23d35237569ee1a11fb99504af020712c8ec7`
- S2-W24 boundary: project canonical `RuntimeInstanceStatusChanged` facts into
  status plus separate status provenance while preserving discovery/inventory;
  no Event append, discovery/reconciliation, next-baseline construction,
  scheduling, daemon, or activation
- S2-W24 fresh contract review:
  `.loom-evidence/phase1-slice2/S2-W24/contract-review.md`
- S2-W24 contract Reviewer: `PASS`; findings: none blocking
- S2-W24 mandatory RED: exit `1`; failed only on the ten missing frozen status
  read-model fields before status product/dispatch behavior existed
- S2-W24 product/test digests:
  `projection.go=783d258f...d2671`,
  `runtime_discovery.go=6ec9bac4...40599`,
  `runtime_status.go=8bf7d0f3...bb2b1`,
  `runtime_discovery_test.go=33e5963d...a37f1`,
  `runtime_status_test.go=ff8f08cc...7967c`
- S2-W24 strict matrix: focused `0`; projection package `0`; impact `0`;
  focused race `-count=30` `0`; repository `0`; repository race `0`; vet `0`;
  gofmt, diff, import, duplicate-payload, non-disclosure, atomic-rebuild, and
  scope checks `PASS`
- S2-W24 Candidate:
  `.loom-evidence/phase1-slice2/S2-W24/deliverable.md`
- S2-W24 fresh implementation review:
  `.loom-evidence/phase1-slice2/S2-W24/implementation-review.md`
- S2-W24 implementation Reviewer: `PASS`; findings: none blocking; complete
  strict matrix independently passed
- S2-W24 Candidate last line: `VERDICT: PASS`
- Accepted S2-W24 local commit: `9838779`
- S2-W25 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W25/contract.md`
- S2-W25 original contract SHA256:
  `a771b8438fdf7293609d687b06d3426d8d5dc153e6b47d80618dc034e6112dbd`
- S2-W25 S2-W22 amendment:
  `.loom-evidence/phase1-slice2/S2-W25/s2-w22-amendment.md`
- S2-W25 amendment SHA256:
  `737585e1e8084e21e3287b145211b145e2bff4eac692b221bb5c88e5162cbcc7`
- S2-W25 Contract Review 1:
  `.loom-evidence/phase1-slice2/S2-W25/contract-review-1.md`
- S2-W25 Contract Review 1: `FAIL`; original validation admitted a forged
  discovery/previous Event pair
- S2-W25 Contract Repair 1:
  `.loom-evidence/phase1-slice2/S2-W25/contract-repair-1.md`
- S2-W25 repaired contract SHA256:
  `52e04294aaf86313b09bb769be134095921fdeb6a3818e412b4565e2650985e3`
- S2-W25 repaired boundary: pure zero-through-32 copied projection-to-baseline
  adapter; discovery and previous Event ID equality must match sequence equality
  in both directions; no Journal replay/write, discovery/reconciliation,
  scheduling, daemon, Pi process, or activation
- S2-W25 fresh Contract Repair 1 review:
  `.loom-evidence/phase1-slice2/S2-W25/contract-review-2.md`
- S2-W25 repaired-contract Reviewer: `PASS`; findings: none blocking
- S2-W25 mandatory RED: exit `1`; failed only on missing
  `BuildRuntimeStatusBaselines` and
  `ErrInvalidRuntimeStatusBaselineProjection`
- S2-W25 product/test digests:
  `status_reconciliation.go=89e10f9e...9123`,
  `status_reconciliation_test.go=effa3c4e...bf6`,
  `runtime_status_writer_test.go=4bda177b...80e8`,
  `runtime_status_test.go=e11f1a59...3351`,
  `runtime_status_baseline.go=d1ba17f8...927`,
  `runtime_status_baseline_test.go=33d30de9...a42f`
- S2-W25 strict matrix: focused adapter `0`; renamed S2-W22 `0`;
  writer/projection regression `0`; package/impact `0`; focused race
  `-count=30` `0`; repository `0`; repository race `0`; vet `0`; gofmt, diff,
  import, non-disclosure, mutation-isolation, forged-provenance, and scope
  checks `PASS`
- S2-W25 Candidate:
  `.loom-evidence/phase1-slice2/S2-W25/deliverable.md`
- S2-W25 Implementation Review 1:
  `.loom-evidence/phase1-slice2/S2-W25/implementation-review-1.md`
- S2-W25 Implementation Review 1: `FAIL`; cross-record Event ID reuse could
  mint baseline provenance impossible under accepted Journal replay
- S2-W25 Implementation Repair 1 contract:
  `.loom-evidence/phase1-slice2/S2-W25/implementation-repair-1-contract.md`
- S2-W25 active amended contract SHA256:
  `d47939a110667b640cb3f264a7edb74557ee9e0988e99928549cd358cd297a49`
- S2-W25 Repair 1 boundary: reject every same-role/cross-role reuse across
  Runtime records among discovery, current status, and previous status Event
  IDs; allow only the valid same-record first-status discovery/previous alias
- S2-W25 Repair 1 product/test state before contract review: unchanged from the
  reviewed Candidate
- S2-W25 fresh Implementation Repair 1 contract review:
  `.loom-evidence/phase1-slice2/S2-W25/implementation-repair-1-contract-review.md`
- S2-W25 Repair 1 contract Reviewer: `PASS`; findings: none blocking
- S2-W25 mandatory Repair RED: exit `1`; all nine cross-record same-role/
  cross-role Event ID combinations were incorrectly accepted by the reviewed
  Candidate; no unrelated failure
- S2-W25 Repair 1 behavior: one combined Event-ID-to-Runtime owner set rejects
  reuse by a different Runtime while preserving the valid same-record first
  status discovery/previous alias
- S2-W25 Repair 1 product/test digests:
  `runtime_status_baseline.go=b39b8183...68b86`,
  `runtime_status_baseline_test.go=51a58e60...6c317`;
  all other Candidate product/test hashes unchanged
- S2-W25 Repair 1 strict matrix: focused adapter `0`; renamed S2-W22 `0`;
  writer/projection regression `0`; package/impact `0`; focused race
  `-count=30` `0`; repository `0`; repository race `0`; vet `0`; gofmt, diff,
  import, non-disclosure, nine-combination ownership, and scope checks `PASS`
- S2-W25 fresh Implementation Repair 1 review:
  `.loom-evidence/phase1-slice2/S2-W25/implementation-review-2.md`
- S2-W25 Repair 1 implementation Reviewer: `PASS`; findings: none; complete
  strict matrix independently passed
- S2-W25 Candidate last line: `VERDICT: PASS`
- Accepted S2-W25 local commit: `38d914b`
- S2-W26 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W26/contract.md`
- S2-W26 contract SHA256:
  `5593c5de01b91c0937907e863f864e7edb686390274b8492e4fff0e32f2dcdd0`
- S2-W26 fresh contract review:
  `.loom-evidence/phase1-slice2/S2-W26/contract-review.md`
- S2-W26 contract Reviewer: `PASS`; findings: none blocking
- S2-W26 boundary: prevalidate and copy zero through 32 configured factories;
  build each once in caller order; filter only canonical explicit absence;
  collect every present probe before observation; delegate once to accepted
  S2-W2; no persistence-order decision, status inference, scheduling, daemon,
  process, or activation
- S2-W26 mandatory RED: exit `1`; failed only on missing frozen package symbols;
  no syntax, existing-package, dependency, or environment failure
- S2-W26 product/test digests:
  `scan.go=2e8cbdc...11d4`,
  `scan_test.go=e8e2f297...2e12`
- S2-W26 strict matrix: focused `0`; discoveryscan/runtime packages `0`;
  runtime/Pi/discoveryscan impact `0`; focused race `-count=50` `0`;
  repository `0`; repository race `0`; vet `0`; gofmt, diff, import,
  non-disclosure, mutation-isolation, concrete Pi absence, and scope checks
  `PASS`
- S2-W26 Candidate:
  `.loom-evidence/phase1-slice2/S2-W26/deliverable.md`
- S2-W26 Implementation Review 1:
  `.loom-evidence/phase1-slice2/S2-W26/implementation-review-1.md`
- S2-W26 Implementation Review 1: `FAIL`; exact-32 inclusive upper-bound
  acceptance lacked direct test proof; no product defect or other finding
- S2-W26 Implementation Repair 1 contract:
  `.loom-evidence/phase1-slice2/S2-W26/implementation-repair-1-contract.md`
- S2-W26 Repair 1 contract SHA256:
  `b44e4747f1487470f1d98cad0da62ab8eb70b38e7a9b3c80d78bb4b501a22ee6`
- S2-W26 fresh Repair 1 contract review:
  `.loom-evidence/phase1-slice2/S2-W26/implementation-repair-1-contract-review.md`
- S2-W26 Repair 1 contract Reviewer: `PASS`; findings: none blocking; product
  is read-only
- S2-W26 mandatory Repair RED: exit `1`; no exact-maximum source assertion
  existed in the reviewed test
- S2-W26 Repair 1: added exact-32 canonical-absent success, caller order, valid
  empty snapshot, and exactly-once call proof; 33 rejection remains unchanged
- S2-W26 Repair 1 hashes:
  `scan.go=2e8cbdc...11d4` unchanged,
  `scan_test.go=5bbfb4f6...9a74`
- S2-W26 Repair 1 strict matrix: focused `0`; package/impact `0`; focused race
  `-count=50` `0`; repository `0`; repository race `0`; vet `0`; gofmt, diff,
  import, non-disclosure, exact-bound, and scope checks `PASS`
- S2-W26 fresh Repair 1 implementation review:
  `.loom-evidence/phase1-slice2/S2-W26/implementation-review-2.md`
- S2-W26 Repair 1 implementation Reviewer: `PASS`; findings: none; complete
  strict matrix independently passed
- S2-W26 Candidate last line: `VERDICT: PASS`
- Accepted S2-W26 local commit: `494579d`
- S2-W27 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W27/contract.md`
- S2-W27 contract SHA256:
  `7235cb0abf89842fcd3498aebeeb494231e3b01cc02a5bb97f67de2ae9b9b764`
- S2-W27 fresh contract review:
  `.loom-evidence/phase1-slice2/S2-W27/contract-review.md`
- S2-W27 contract Reviewer: `PASS`; findings: none blocking
- S2-W27 boundary: one explicit app command runs S2-W26 once, skips commit for
  empty/all-absent, calls one injected committer for non-empty, and accepts
  only an exact S2-W20 Candidate; no Event metadata allocation, direct
  Journal/projection, status policy, scheduling, daemon, activation, or Slice 3
- S2-W27 mandatory RED: exit `1`; failed only on missing frozen app symbols;
  no syntax, existing-package, dependency, SQLite, or environment failure
- S2-W27 product/test digests:
  `runtime_discovery.go=be752147...ec4a`,
  `runtime_discovery_test.go=473db85b...04d3`
- S2-W27 strict matrix: focused `0`; app package `0`; app/runtime/
  discoveryscan/state/journal impact `0`; focused race `-count=50` `0`;
  repository `0`; repository race `0`; vet `0`; gofmt, diff, import, mutation,
  non-disclosure, real SQLite exact retry, and scope checks `PASS`
- S2-W27 Candidate:
  `.loom-evidence/phase1-slice2/S2-W27/deliverable.md`
- S2-W27 fresh implementation review:
  `.loom-evidence/phase1-slice2/S2-W27/implementation-review.md`
- S2-W27 implementation Reviewer: `PASS`; findings: none; complete strict
  matrix independently passed
- S2-W27 Candidate last line: `VERDICT: PASS`
- Accepted S2-W27 local commit: `7ec635b`
- S2-W28 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W28/contract.md`
- S2-W28 contract SHA256:
  `424554aca51c6d00ff1624e3d5cba386843b4c6aaa2edcf58bcce3e068b1ade8`
- S2-W28 fresh contract review:
  `.loom-evidence/phase1-slice2/S2-W28/contract-review.md`
- S2-W28 contract Reviewer: `PASS`; findings: none blocking
- S2-W28 boundary: concrete S2-W27 committer adapter binds an accepted S2-W20
  appender and injected caller-authoritative input provider; provider once,
  S2-W20 once, no metadata allocation, concrete Journal/projection, status
  policy, scheduling, daemon, activation, or Slice 3
- S2-W28 mandatory RED: exit `1`; failed only on missing frozen symbols
- S2-W28 product/test digests:
  `runtime_discovery_committer.go=9daf0916...6907`,
  `runtime_discovery_committer_test.go=14adfaf8...73aa`
- S2-W28 strict matrix: focused `0`; app package `0`; app/runtime/
  discoveryscan/state/journal impact `0`; focused race `-count=50` `0`;
  repository `0`; repository race `0`; vet `0`; gofmt, diff, import, mutation,
  non-disclosure, real SQLite S2-W27 exact retry, and scope checks `PASS`
- S2-W28 Candidate:
  `.loom-evidence/phase1-slice2/S2-W28/deliverable.md`
- S2-W28 Implementation Review 1: `FAIL`; exported zero-value adapter could
  panic on nil stored provider
- S2-W28 Repair 1 contract/review: frozen test plus method-boundary stored
  binding validation; contract Reviewer `PASS`
- S2-W28 Repair RED: reproduced nil-pointer panic; minimal repair GREEN
- S2-W28 repaired hashes:
  `runtime_discovery_committer.go=aba00945...1434`,
  `runtime_discovery_committer_test.go=ac9ab0e3...f6f6`
- S2-W28 repaired matrix: all checks pass on fresh rerun; first repository-race
  run preserved one unrelated Pi child-marker timing failure, isolated race
  `-count=10` and fresh repository-race rerun both passed
- S2-W28 Repair 1 implementation review:
  `.loom-evidence/phase1-slice2/S2-W28/implementation-review-2.md`
- S2-W28 Repair 1 implementation Reviewer: `PASS`; findings: none; zero-value
  panic closure and complete strict matrix independently verified
- S2-W28 Candidate last line: `VERDICT: PASS`
- Accepted S2-W28 local commit: `9296832`
- S2-W29 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W29/contract.md`
- S2-W29 contract SHA256:
  `9a56217e50d0e1a93e37b33bd043c1dbd8c6fa11b762b66000264c71fab97dbe`
- S2-W29 boundary: one-shot coordination of accepted S2-W22 reconciliation
  with an injected accepted S2-W23 committer; zero-transition no-commit,
  no discovery execution, metadata allocation, projection/Journal access,
  scheduling, daemon, Runtime activation, or Slice 3
- S2-W29 fresh contract review:
  `.loom-evidence/phase1-slice2/S2-W29/contract-review.md`
- S2-W29 contract Reviewer: `PASS`; findings: none
- S2-W29 mandatory RED: exit `1`; failed only on missing frozen symbols
- S2-W29 product/test digests:
  `runtime_status.go=127ccf36...6e11`,
  `runtime_status_test.go=9742ee41...533c`
- S2-W29 strict matrix: focused `0`; app package `0`; app/runtime/state/
  projection/journal impact `0`; focused race `-count=50` `0`; repository `0`;
  repository race `0`; vet `0`; gofmt, diff, import, mutation, real SQLite exact
  append/retry, non-disclosure, and scope checks `PASS`
- S2-W29 Candidate:
  `.loom-evidence/phase1-slice2/S2-W29/deliverable.md`
- S2-W29 Implementation Review 1: `FAIL`; zero-transition path skipped the
  post-reconciliation context gate, result mismatch tests did not isolate later
  checks, and the static AST assertion was a no-op
- S2-W29 Repair 1 contract: move the context gate before no-change return,
  validate a private public-accessor result interface with isolated one-field
  mutations, and replace the no-op static check with real AST/source assertions
- S2-W29 Repair 1 Contract Review 1: `FAIL`; the proposed interface's
  `Events() []journal.Event` method would require a forbidden product Journal
  import
- S2-W29 Repair 1 Amendment 1: replace the interface with a primitive private
  fact record populated from the concrete S2-W23 Candidate; isolate all seven
  checks without changing imports or public API
- S2-W29 Repair 1 Amendment 1 Reviewer: `PASS`; findings: none
- S2-W29 Repair RED 1: delayed no-change cancellation reproduced incorrect
  nil-error success
- S2-W29 Repair RED 2: failed on missing primitive result facts and validator
- S2-W29 repaired product/test digests:
  `runtime_status.go=23caf1af...6e92c`,
  `runtime_status_test.go=fd51e606...c93c1`
- S2-W29 Repair 1: context gate now precedes no-change return; concrete S2-W23
  accessors reduce to isolated primitive facts; seven single-field mutations
  and real AST/source negative assertions are GREEN
- S2-W29 repaired strict matrix: focused `0`; app package `0`; app/runtime/
  state/projection/journal impact `0`; focused race `-count=50` `0`;
  repository `0`; repository race `0`; vet `0`; gofmt/diff `PASS`
- S2-W29 Repair 1 implementation review:
  `.loom-evidence/phase1-slice2/S2-W29/implementation-review-2.md`
- S2-W29 Repair 1 implementation Reviewer: `PASS`; findings: none; all three
  Review 1 gaps and the complete strict matrix independently verified
- S2-W29 Candidate last line: `VERDICT: PASS`
- Accepted S2-W29 local commit: `51d0489`
- S2-W30 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W30/contract.md`
- S2-W30 contract SHA256:
  `12883c8a64bfbbf929685c10ad7c83082c748db8eb0508a4459dd3033fb0e8fd`
- S2-W30 boundary: concrete S2-W29 status committer adapter binds an accepted
  S2-W23 appender and injected caller-authoritative input provider; provider
  once, S2-W23 once, zero-value fail-closed, no metadata allocation, concrete
  Journal/projection, discovery/status policy, scheduling, daemon, activation,
  or Slice 3
- S2-W30 fresh contract review:
  `.loom-evidence/phase1-slice2/S2-W30/contract-review.md`
- S2-W30 contract Reviewer: `PASS`; findings: none
- S2-W30 mandatory RED: exit `1`; failed only on missing frozen symbols
- S2-W30 product/test digests:
  `runtime_status_committer.go=9ab30769...59298`,
  `runtime_status_committer_test.go=639f9cd9...49bd9`
- S2-W30 strict matrix: focused `0`; app package `0`; app/runtime/state/
  projection/journal impact `0`; focused race `-count=50` `0`; repository `0`;
  repository race `0`; vet `0`; gofmt, diff, zero-value, import, mutation, real
  SQLite S2-W29 exact retry, non-disclosure, and scope checks `PASS`
- S2-W30 Candidate:
  `.loom-evidence/phase1-slice2/S2-W30/deliverable.md`
- S2-W30 fresh implementation review:
  `.loom-evidence/phase1-slice2/S2-W30/implementation-review.md`
- S2-W30 implementation Reviewer: `PASS`; findings: none; complete strict
  matrix independently passed
- S2-W30 Candidate last line: `VERDICT: PASS`
- Accepted S2-W30 local commit: `47f225b`
- S2-W31 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W31/contract.md`
- S2-W31 contract SHA256:
  `887d8a75237c7e578e966435f2e9701d2a5c3fec6d1b6729af7dcaf055c4749c`
- S2-W31 boundary: caller-supplied copied projection Snapshot through accepted
  S2-W25 baseline construction into accepted S2-W29 status coordination; no
  product Journal query/rebuild, discovery write, metadata allocation,
  discovery/status policy composition, scheduling, daemon, activation, or
  Slice 3
- S2-W31 fresh contract review:
  `.loom-evidence/phase1-slice2/S2-W31/contract-review.md`
- S2-W31 contract Reviewer: `PASS`; findings: none
- S2-W31 mandatory RED: exit `1`; failed only on missing frozen symbols
- S2-W31 product/test digests:
  `runtime_status_projection.go=bb9b96a6...d6a40f`,
  `runtime_status_projection_test.go=86d1db54...5b8e3`
- S2-W31 strict matrix: focused `0`; app package `0`; app/runtime/state/
  projection/journal impact `0`; focused race `-count=50` `0`; repository `0`;
  repository race `0`; vet `0`; gofmt, diff, import, mutation, real SQLite
  projection-to-status exact retry, no-policy, and scope checks `PASS`
- S2-W31 Candidate:
  `.loom-evidence/phase1-slice2/S2-W31/deliverable.md`
- S2-W31 Implementation Review 1: `FAIL`; missing status-bearing provenance,
  complete invalid-projection matrix, and S2-W29 error/zero-output/Candidate
  mutation evidence; product boundary itself had no finding
- S2-W31 Repair 1 contract: test-only closure of all three evidence gaps;
  product must remain byte-for-byte unchanged
- S2-W31 Repair 1 contract review:
  `.loom-evidence/phase1-slice2/S2-W31/implementation-repair-1-contract-review.md`
- S2-W31 Repair 1 contract Reviewer: `PASS`; findings: none; repair remains
  test-only and product hash is frozen
- S2-W31 Repair 1 mandatory RED: exit `1`; the old discovery-only fixture
  returned discovery Event sequence `1` while the new status-bearing assertion
  required status Event sequence `2`
- S2-W31 Repair 1 product/test digests:
  `runtime_status_projection.go=bb9b96a6...d6a40f` unchanged,
  `runtime_status_projection_test.go=083a3a8c...e3169`
- S2-W31 Repair 1 proof: first/consecutive status and rediscovery provenance;
  exact sentinel across oversized/key/core/model/discovery/status/sequence/
  cross-record Event defects; invalid discovery, identity drift, committer,
  result mismatch, and deterministic post-baseline context propagation; nested
  projection and reconciliation/commit accessor mutation isolation
- S2-W31 Repair 1 strict matrix: focused `0`; app package `0`;
  app/runtime/state/projection/journal impact `0`; focused race `-count=50` `0`;
  repository `0`; repository race `0`; vet `0`; gofmt and diff `PASS`
- S2-W31 Repair 1 Implementation Review 1: `FAIL`; missing projected
  stable-identity defect while status provenance and S2-W29 propagation/
  mutation closures passed
- S2-W31 Repair 1 review evidence:
  `.loom-evidence/phase1-slice2/S2-W31/implementation-repair-1-review-1.md`
- S2-W31 Repair 1 bounded test-only correction: added valid-key/empty-DeviceID
  projection rejection; focused and complete strict matrix pass; product hash
  remains unchanged
- S2-W31 fresh Implementation Review 2:
  `.loom-evidence/phase1-slice2/S2-W31/implementation-review-2.md`
- S2-W31 Repair 1 implementation Reviewer: `PASS`; findings: none; full strict
  matrix independently passed
- S2-W31 Candidate last line: `VERDICT: PASS`
- Accepted S2-W31 local commit: `b00f8d8`
- User architecture authorization: S2-W32 uses discovery-priority Runtime
  observation writes
- Accepted ADR-0007:
  `docs/adr/0007-runtime-observation-discovery-priority.md`
- S2-W32 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W32/contract.md`
- S2-W32 boundary: pure `none`/`discovery`/`status` Candidate; discovery wins
  mixed inventory/status changes; stable identity drift errors; absence does
  not create status/deletion; no writer, Event metadata, Journal, scheduler,
  daemon, activation, or Slice 3 authority
- S2-W32 fresh contract review:
  `.loom-evidence/phase1-slice2/S2-W32/contract-review.md`
- S2-W32 contract Reviewer: `PASS`; findings: none
- S2-W32 mandatory RED: exit `1`; failed only on missing frozen error, kind,
  Candidate, and planner symbols
- S2-W32 product/test digests:
  `runtime_write_plan.go=f50a6d5a...e9c97`,
  `runtime_write_plan_test.go=c60cbd01...018c`
- S2-W32 proof: none/current-only/every inventory/status-only/mixed precedence,
  absence, invalid/oversized projection, invalid discovery, identity drift,
  delayed context, map-order/digest sensitivity, mutation, zero Candidate, and
  static no-authority boundary
- S2-W32 strict matrix: focused `0`; app `0`; app/runtime/projection impact `0`;
  focused race `-count=50` `0`; repository `0`; repository race `0`; vet `0`;
  gofmt, diff, and ADR index/link `PASS`
- S2-W32 Candidate:
  `.loom-evidence/phase1-slice2/S2-W32/deliverable.md`
- S2-W32 Implementation Review 1: `FAIL`; canonical Candidate digest and
  sensitivity table omit exposed `Planned()`; all other behavior/boundaries
  passed
- S2-W32 Review 1 evidence:
  `.loom-evidence/phase1-slice2/S2-W32/implementation-review-1.md`
- S2-W32 Repair 1 contract: add only existing `planned` fact to versioned
  digest payload and direct single-field RED
- S2-W32 Repair 1 contract review:
  `.loom-evidence/phase1-slice2/S2-W32/implementation-repair-1-contract-review.md`
- S2-W32 Repair 1 contract Reviewer: `PASS`; findings: none
- S2-W32 Repair 1 mandatory RED: exit `1`; mutating `planned` did not change
  Candidate digest
- S2-W32 Repair 1 product/test digests:
  `runtime_write_plan.go=0a7ac256...c721b`,
  `runtime_write_plan_test.go=57eace71...22f8d`
- S2-W32 Repair 1: versioned digest payload now contains exact `planned`; its
  direct sensitivity test and complete strict matrix pass
- S2-W32 fresh Implementation Review 2:
  `.loom-evidence/phase1-slice2/S2-W32/implementation-review-2.md`
- S2-W32 Repair 1 implementation Reviewer: `PASS`; findings: none; complete
  strict matrix independently passed
- S2-W32 Candidate last line: `VERDICT: PASS`
- Accepted S2-W32 local commit: `26bf981`
- S2-W33 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W33/contract.md`
- S2-W33 fresh contract review:
  `.loom-evidence/phase1-slice2/S2-W33/contract-review.md`
- S2-W33 contract Reviewer: `PASS`; findings: none
- S2-W33 mandatory RED: failed only on missing frozen coordinator/error symbols
- S2-W33 Candidate product/test SHA256:
  `runtime_observation_write.go=951f454...6773`,
  `runtime_observation_write_test.go=a3703902...947`
- S2-W33 Controller strict matrix: focused, app, impact, focused-race-50,
  repository, repository-race, vet, format, diff, real SQLite discovery-priority
  then status-only chain, idempotent retries, and static boundary all `PASS`
- S2-W33 Candidate:
  `.loom-evidence/phase1-slice2/S2-W33/deliverable.md`
- S2-W33 fresh implementation review:
  `.loom-evidence/phase1-slice2/S2-W33/implementation-review.md`
- S2-W33 implementation Reviewer: `PASS`; findings: none; complete strict
  matrix independently passed
- S2-W33 Candidate last line: `VERDICT: PASS`
- S2-W33 boundary: plan once; none writes nothing; discovery invokes only
  discovery committer; status invokes only S2-W31; zero non-selected outputs;
  no discovery execution, projection query, metadata, retry, scheduler, daemon,
  activation, or Slice 3 authority
- Accepted S2-W33 local commit: `affd2a6`
- S2-W34 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W34/contract.md`
- S2-W34 fresh contract review:
  `.loom-evidence/phase1-slice2/S2-W34/contract-review.md`
- S2-W34 contract Reviewer: `PASS`; findings: none
- S2-W34 mandatory RED: failed only on missing frozen coordinator/error symbols
- S2-W34 Candidate product/test SHA256:
  `runtime_observation_cycle.go=26a1a706...d7ea`,
  `runtime_observation_cycle_test.go=8759409d...2776`
- S2-W34 Controller strict matrix: focused, app, impact, focused-race-50,
  repository, repository-race, vet, format, diff, configured SQLite
  discovery-priority then status-only chain, idempotent retries, and static
  boundary all `PASS`
- S2-W34 Candidate:
  `.loom-evidence/phase1-slice2/S2-W34/deliverable.md`
- S2-W34 Implementation Review 1:
  `.loom-evidence/phase1-slice2/S2-W34/implementation-review-1.md`
- S2-W34 Review 1: `FAIL`; one test-proof gap; no product defect
- S2-W34 Repair 1 contract:
  `.loom-evidence/phase1-slice2/S2-W34/implementation-repair-1-contract.md`
- S2-W34 Repair 1 contract review:
  `.loom-evidence/phase1-slice2/S2-W34/implementation-repair-1-contract-review.md`
- S2-W34 Repair 1 contract Reviewer: `PASS`; findings: none
- S2-W34 Repair 1 scope: test-only complete S2-W26 error matrix; product hash
  frozen unchanged
- S2-W34 Repair 1 mandatory RED: failed on all six missing canonical case
  markers
- S2-W34 Repair 1 test SHA256:
  `runtime_observation_cycle_test.go=4a4b3fb3...79f9e`
- S2-W34 Repair 1: oversized/typed-nil factories, invalid present/absent probe
  results, probe source error, and invalid discovered observation now directly
  prove five-zero/no-committer propagation; complete strict matrix `PASS`;
  product SHA unchanged
- S2-W34 fresh Implementation Review 2:
  `.loom-evidence/phase1-slice2/S2-W34/implementation-review-2.md`
- S2-W34 Repair 1 implementation Reviewer: `PASS`; findings: none; complete
  strict matrix independently passed
- S2-W34 Candidate last line: `VERDICT: PASS`
- S2-W34 boundary: execute accepted S2-W26 once, delegate its exact snapshot
  plus caller-supplied projection to S2-W33 once; no S2-W27 unconditional
  commit, projection query/rebuild, metadata, retry, scheduler, daemon/config,
  activation, or Slice 3 authority
- Accepted S2-W34 local commit: `a6eb816`
- S2-W35 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W35/contract.md`
- S2-W35 fresh contract review:
  `.loom-evidence/phase1-slice2/S2-W35/contract-review.md`
- S2-W35 contract Reviewer: `PASS`; findings: none
- S2-W35 mandatory RED: failed only on missing frozen coordinator/error symbols
- S2-W35 Candidate product/test SHA256:
  `runtime_observation_projected.go=08409802...fe04`,
  `runtime_observation_projected_test.go=7ec0bfd0...7b1`
- S2-W35 Controller strict matrix: focused, app, impact, focused-race-50,
  repository, repository-race, vet, format, diff, projected SQLite
  discovery-priority/status-only idempotency, mutation, and static boundary all
  `PASS`
- S2-W35 Candidate:
  `.loom-evidence/phase1-slice2/S2-W35/deliverable.md`
- S2-W35 Implementation Review 1:
  `.loom-evidence/phase1-slice2/S2-W35/implementation-review-1.md`
- S2-W35 Review 1: `FAIL`; SQLite exact-Events test-proof gap; no product defect
- S2-W35 Repair 1 contract:
  `.loom-evidence/phase1-slice2/S2-W35/implementation-repair-1-contract.md`
- S2-W35 Repair 1 contract review:
  `.loom-evidence/phase1-slice2/S2-W35/implementation-repair-1-contract-review.md`
- S2-W35 Repair 1 contract Reviewer: `PASS`; findings: none
- S2-W35 Repair 1 scope: test-only Event types/sequences, exact retry, and final
  rebuilt projection facts; product SHA frozen unchanged
- S2-W35 Repair 1 mandatory RED: failed only on the three missing canonical
  coverage markers
- S2-W35 Repair 1: exact persisted Event types/sequences, exact retry Candidate
  identity, and final rebuilt Runtime facts are now directly asserted; complete
  strict matrix `PASS`; product SHA unchanged
- S2-W35 fresh Implementation Review 2:
  `.loom-evidence/phase1-slice2/S2-W35/implementation-review-2.md`
- S2-W35 Repair 1 implementation Reviewer: `PASS`; findings: none; complete
  strict matrix independently passed
- S2-W35 Candidate last line: `VERDICT: PASS`
- S2-W35 boundary: read accepted projection Snapshot once, delegate exact
  copied baseline to S2-W34 once; no rebuild, direct Journal/SQLite, metadata,
  retry, scheduler, daemon/config, activation, or Slice 3 authority
- Accepted S2-W35 local commit: `c8ecc2a`
- S2-W36 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W36/contract.md`
- S2-W36 fresh contract review:
  `.loom-evidence/phase1-slice2/S2-W36/contract-review.md`
- S2-W36 contract Reviewer: `PASS`; findings: none
- S2-W36 mandatory RED: focused compile failed only on missing frozen
  observer/constructor/error symbols
- S2-W36 Candidate product/test SHA256:
  `runtime_observer.go=a9421c3b...9213`,
  `runtime_observer_test.go=5c645321...515c`
- S2-W36 Controller strict matrix: focused, app, impact, focused-race-50,
  repository, repository-race, vet, format, diff, prepared SQLite
  discovery-priority/status-only exact Events, explicit retry idempotency,
  final projection, mutation, and static boundary all `PASS`
- S2-W36 Candidate:
  `.loom-evidence/phase1-slice2/S2-W36/deliverable.md`
- S2-W36 Implementation Review 1:
  `.loom-evidence/phase1-slice2/S2-W36/implementation-review-1.md`
- S2-W36 Review 1: `FAIL`; direct context/downstream error-matrix test-proof
  gap; no product defect
- S2-W36 Repair 1 contract:
  `.loom-evidence/phase1-slice2/S2-W36/implementation-repair-1-contract.md`
- S2-W36 Repair 1 contract review:
  `.loom-evidence/phase1-slice2/S2-W36/implementation-repair-1-contract-review.md`
- S2-W36 Repair 1 contract Reviewer: `PASS`; findings: none
- S2-W36 Repair 1 scope: test-only direct error matrix; product hash frozen
  unchanged
- S2-W36 Repair 1 mandatory RED: failed only on all twelve missing canonical
  error-case markers
- S2-W36 Repair 1 test SHA256:
  `runtime_observer_test.go=8ce88b02...94e2`
- S2-W36 Repair 1: canceled/deadline, identity drift, missing/typed-nil
  discovery/status writer, writer error/result mismatch, and delayed
  cancellation now directly prove five-zero/exact-call/no-retry behavior;
  complete strict matrix `PASS`; product SHA unchanged
- S2-W36 fresh Implementation Review 2:
  `.loom-evidence/phase1-slice2/S2-W36/implementation-review-2.md`
- S2-W36 Review 2: `FAIL`; configured-discovery failure still used anonymous
  committers and lacked direct zero-call proof; no product defect
- S2-W36 repeated-failure rule: second same-class proof failure requires fresh
  read-only problem analyst before Repair 2
- S2-W36 fresh problem analysis:
  `.loom-evidence/phase1-slice2/S2-W36/problem-analysis-1.md`
- S2-W36 problem analysis: false-green contract-to-test traceability gap; no
  product defect; configured-discovery tuple `1/0/0` required
- S2-W36 Repair 2 contract:
  `.loom-evidence/phase1-slice2/S2-W36/implementation-repair-2-contract.md`
- S2-W36 Repair 2 scope: test-only named configured-discovery sentinel and
  exact factory/discovery/status call tuple `1/0/0`; product hash frozen
- S2-W36 Repair 2 contract Review 1:
  `.loom-evidence/phase1-slice2/S2-W36/implementation-repair-2-contract-review-1.md`
- S2-W36 Repair 2 contract Review 1: `FAIL`; global call-count marker collision;
  no implementation began
- S2-W36 Repair 2 Amendment 1:
  `.loom-evidence/phase1-slice2/S2-W36/implementation-repair-2-contract-amendment-1.md`
- S2-W36 Repair 2 Amendment 1: unique case-local factory/committer markers;
  behavior/scope/product lock unchanged
- S2-W36 Repair 2 fresh Amendment Review 2:
  `.loom-evidence/phase1-slice2/S2-W36/implementation-repair-2-contract-review-2.md`
- S2-W36 Repair 2 Amendment Reviewer: `PASS`; findings: none
- S2-W36 Repair 2 mandatory RED: failed only on all four missing unique
  case-local behavior markers
- S2-W36 Repair 2 test SHA256:
  `runtime_observer_test.go=ee80f702...b23a`
- S2-W36 Repair 2: configured-discovery sentinel, five-zero outputs, and exact
  runtime factory/discovery/status call tuple `1/0/0`; complete strict matrix
  `PASS`; product SHA unchanged
- S2-W36 fresh Implementation Review 3:
  `.loom-evidence/phase1-slice2/S2-W36/implementation-review-3.md`
- S2-W36 Repair 2 implementation Reviewer: `PASS`; findings: none; complete
  strict matrix independently passed
- S2-W36 Candidate last line: `VERDICT: PASS`
- S2-W36 boundary: immutable shallow-copied factory binding plus accepted
  projection/committers; each explicit RunOnce delegates exactly once to
  S2-W35; no direct lower-layer composition, retry, scheduler, daemon/config,
  activation, or Slice 3 authority
- Accepted S2-W36 local commit: `38891c3`
- S2-W37 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W37/contract.md`
- S2-W37 contract Review 1:
  `.loom-evidence/phase1-slice2/S2-W37/contract-review-1.md`
- S2-W37 contract Review 1: `FAIL`; typed-nil validation/import whitelist
  ambiguity; no implementation began
- S2-W37 Amendment 1:
  `.loom-evidence/phase1-slice2/S2-W37/contract-amendment-1.md`
- S2-W37 Amendment 1: require accepted same-package
  `nilAppInterface(trigger)`; no new import/API/authority
- S2-W37 fresh Amendment Review 2:
  `.loom-evidence/phase1-slice2/S2-W37/contract-review-2.md`
- S2-W37 Amendment Reviewer: `PASS`; findings: none
- S2-W37 mandatory RED: focused compile failed only on missing frozen trigger
  interface, error, and function symbols
- S2-W37 product/test SHA256:
  `runtime_observation_trigger.go=47baf06b...1be6`,
  `runtime_observation_trigger_test.go=a1389a84...91e8`
- S2-W37 Controller strict matrix: focused/app/impact/focused-race-50/
  repository/repository-race/vet/format/diff all `PASS`
- S2-W37 direct proof: nil and typed-nil inputs, pre/post-trigger context,
  trigger error/cancellation, complete downstream error matrix, exact
  selected/opposite call counts, five-zero failures, and no retry/fallback
- S2-W37 real SQLite proof: two triggered discovery calls plus two triggered
  status calls yield exact discovery/discovery/status Events at sequences
  1/2/3 and final changed-display/online projection facts
- S2-W37 boundary: await one injected trigger then invoke accepted S2-W36 once;
  no time/timer/ticker/channel/signal/loop/goroutine/config/daemon/activation or
  direct lower-layer authority
- S2-W37 Candidate evidence:
  `.loom-evidence/phase1-slice2/S2-W37/deliverable.md`
- S2-W37 fresh Implementation Review 1:
  `.loom-evidence/phase1-slice2/S2-W37/implementation-review-1.md`
- S2-W37 implementation Reviewer: `PASS`; findings: none; complete strict
  matrix independently passed
- S2-W37 Candidate last line: `VERDICT: PASS`
- S2-W37 fresh pre-commit matrix: focused/app/impact/focused-race-50/
  repository/repository-race/vet/format/diff all `PASS`
- Accepted S2-W37 local commit: `9175f94`
- S2-W37 post-commit focused/repository checks: `PASS`
- S2-W38 frozen contract:
  `.loom-evidence/phase1-slice2/S2-W38/contract.md`
- S2-W38 contract SHA256:
  `81e7cb6c...d3a3b`
- S2-W38 fresh Contract Review 1:
  `.loom-evidence/phase1-slice2/S2-W38/contract-review-1.md`
- S2-W38 contract Reviewer: `PASS`; findings: none
- S2-W38 Controller contract check:
  `.loom-evidence/phase1-slice2/S2-W38/controller-contract-check-1.md`
- S2-W38 Controller contract check: `FAIL`; recurrence-only composition would
  read stale bound projection after a committed observation; no implementation
  began
- S2-W38 Contract Repair 1:
  `.loom-evidence/phase1-slice2/S2-W38/contract-repair-1.md`
- S2-W38 Repair 1 fresh Contract Review 1:
  `.loom-evidence/phase1-slice2/S2-W38/contract-repair-1-review-1.md`
- S2-W38 Repair 1 contract Reviewer: `FAIL`; separate projection may not be the
  observer's bound read model, and partial-success language exceeded
  post-S2-W37 observability; no implementation began
- S2-W38 same-class stale-projection failure count: `2`; fresh read-only
  problem analysis required before Contract Repair 2
- S2-W38 fresh Problem Analysis 1:
  `.loom-evidence/phase1-slice2/S2-W38/problem-analysis-1.md`
- S2-W38 Problem Analysis 1: lifecycle binding defect; use only the observer's
  captured private read model; no post-refresh context check; S2-W37 errors
  remain five-zero without a no-write claim
- S2-W38 Contract Repair 2:
  `.loom-evidence/phase1-slice2/S2-W38/contract-repair-2.md`
- S2-W38 Repair 2 fresh Contract Review 1:
  `.loom-evidence/phase1-slice2/S2-W38/contract-repair-2-review-1.md`
- S2-W38 Repair 2 contract Reviewer: `PASS`; findings: none
- S2-W38 first RED attempt: discarded because Go's compile-error limit hid one
  test-fixture field error; product file removed before retry
- S2-W38 valid mandatory RED: focused compile failed only on missing repaired
  frozen error/function symbols
- S2-W38 product/test SHA256:
  `runtime_observation_loop.go=33d7e462...0c9c0`,
  `runtime_observation_loop_test.go=258c84a0...9029`
- S2-W38 Controller strict matrix: focused/app/impact/focused-race-50/
  repository/repository-race/vet/format/diff all `PASS`
- S2-W38 post-success refresh failure proof: exact successful tuple plus
  refresh error, authoritative Event retained, previous projection preserved,
  no retry
- S2-W38 real SQLite proof: same read model/observer/trigger/scripted factory,
  no test-side inter-call rebuild, exact discovery/discovery/status Events at
  sequences 1/2/3 and final changed-display/online projection
- S2-W38 Candidate evidence:
  `.loom-evidence/phase1-slice2/S2-W38/deliverable.md`
- S2-W38 fresh Implementation Review 1:
  `.loom-evidence/phase1-slice2/S2-W38/implementation-review-1.md`
- S2-W38 Implementation Reviewer 1: `FAIL`; missing direct none/discovery/
  status success-path runtime order/count group; no product defect
- S2-W38 Implementation Repair 1 contract:
  `.loom-evidence/phase1-slice2/S2-W38/implementation-repair-1-contract.md`
- S2-W38 Repair 1 fresh Contract Review 1:
  `.loom-evidence/phase1-slice2/S2-W38/implementation-repair-1-contract-review-1.md`
- S2-W38 Repair 1 contract Reviewer: `PASS`; findings: none
- S2-W38 Repair 1: test/evidence only; product SHA locked
  `33d7e462...0c9c0`; product repair attempt remains `0/3`
- S2-W38 Repair 1 mandatory RED: failed only on four missing unique case-local
  none/discovery/status/order coverage markers
- S2-W38 Repair 1 test SHA256:
  `runtime_observation_loop_test.go=60c65473...ff06`
- S2-W38 Repair 1 direct success proof: none exposes A before factory and B
  after factory-side append/post-refresh with writers `0/0`; discovery and
  status use real prepared committers behind recording wrappers with exact
  trigger→factory→probe→selected-writer traces, counts, and post-refresh
  sequence 2 facts
- S2-W38 Repair 1 complete strict matrix and product-lock check: `PASS`;
  product SHA remains `33d7e462...0c9c0`
- S2-W38 fresh Implementation Review 2:
  `.loom-evidence/phase1-slice2/S2-W38/implementation-review-2.md`
- S2-W38 Repair 1 implementation Reviewer: `PASS`; findings: none; complete
  strict matrix and product lock independently passed
- S2-W38 Candidate last line: `VERDICT: PASS`
- S2-W38 fresh pre-commit matrix: focused/app/impact/focused-race-50/
  repository/repository-race/vet/format/diff/product-lock all `PASS`
- S2-W38 Repair 2 boundary: exact bound projection only; await→context→
  pre-refresh decorator, exact S2-W37 once, post-success refresh; successful
  tuple plus error only for post-S2-W37 rebuild failure; no recurrence/
  serialization/scheduler/config/daemon/retry/activation authority
- S2-W38 repaired boundary: trigger-scoped pre-refresh, one exact S2-W37 call,
  and post-success projection refresh; post-commit refresh errors retain exact
  successful outputs plus error; no loop/time/scheduler/config/daemon/retry/
  activation or direct lower-layer authority
- S2-W38 local atomic commit: `39a9e0a`; post-commit focused and repository
  checks: `PASS`; nothing activated
- Slice 2 Exit Contract:
  `.loom-evidence/phase1-slice2/EXIT-CONTRACT.md`;
  SHA256 `c7639aef...ac7614`; fresh Contract Reviewer: `PASS`
- Exit Contract policy: no thin W39/W40; only the merged
  `S2-EXIT-1 Local Runtime Observation Daemon Integration` may close the
  remaining Slice 2 lifecycle boundary
- S2-EXIT-1 frozen contract:
  `.loom-evidence/phase1-slice2/S2-EXIT-1/contract.md`;
  SHA256 `e50fa0a...d4710d0`; fresh Contract Reviewer: `PASS`
- S2-EXIT-1 mandatory RED: failed only on missing frozen daemon/command symbols
- S2-EXIT-1 Candidate: explicit configuration, private SQLite state and
  sibling process lock, real clock and metadata identities, serial
  projection-synchronized recurrence, foreground `loomd`, bounded/cancelable
  lifecycle, close/restart/recovery, and safe deterministic result output
- S2-EXIT-1 Implementation Review 1: `FAIL` only for incomplete direct
  metadata-failure/zero-append and complete configuration-rejection proof; no
  product/security/Slice 3 defect
- S2-EXIT-1 Repair 1 contract and fresh Contract Review: `PASS`; same lineage,
  no new WorkItem, one behavior-preserving unexported sequence helper
  extraction plus bounded tests/evidence
- S2-EXIT-1 Repair 1 mandatory RED: failed only on seven exact missing markers
- S2-EXIT-1 Repair 1 direct GREEN: duplicate/invalid identity, non-UTC time,
  exact identity-source cancellation, and sequence overflow proven; real
  SQLite remains `events=0` for pre-append failures
- S2-EXIT-1 complete config GREEN: parent/state/isolation/Runtime-directory/
  interval/timeout/max-cycle/typed-nil identity matrix rejects with unchanged
  state and no lock side effect
- S2-EXIT-1 complete matrix: focused, impact, command, focused-race-30,
  command-race-10, repository, repository-race, vet, format, and diff `PASS`
- S2-EXIT-1 controlled live proof:
  `.loom-evidence/phase1-slice2/daemon-integration/live-canary.md` and
  `S2-EXIT-1/repair-1-green.md`; compiled foreground lifecycle, restart,
  recovery, real timer/cancel, permissions, residue, and no-orphan checks
  `PASS`; ambient user Pi readiness is not claimed
- S2-EXIT-1 fresh Repair 1 Implementation Reviewer: `PASS`; findings: none
- S2-EXIT-1 Candidate last line: `VERDICT: PASS`
- S2-EXIT-1 local atomic commit: `46eefaf`; exact staged scope excluded
  `AGENTS.md`, the user-owned Historical/Rejected Candidate `PROGRESS.md` hunk,
  `.codex/**`, and `.loom-drafts/**`
- S2-EXIT-1 post-commit focused, command, and repository checks: `PASS`;
  nothing activated
- Whole-Slice Review 1: `FAIL` only because `46eefaf` committed the pre-commit
  `CURRENT`/`PROGRESS` wording; product/security/concurrency findings: none;
  independent repository/race/vet/build/fresh-canary checks: `PASS`
- Whole-Slice Review 1 Repair 1: frozen status-only governance reconciliation;
  no new WorkItem and no product/test change
- Whole-Slice Review 1 Repair 1 fresh Contract Reviewer: `PASS`; findings: none
- User authorization: one separate status-only Slice 2 governance commit and
  fresh whole-Slice re-review explicitly authorized
- This checkpoint records the governance reconciliation; next gate is the
  fresh whole-Slice re-review, and S3 remains closed until Reviewer `PASS`
- Whole-Slice Review 2: `PASS`; findings: none; independent repository/race/
  vet/format/build and bounded discovery→restart→rediscovery canary passed
- Slice 2 accepted reviewed HEAD: `7b1726e`; nothing activated
- Slice 3 entered governance setup only; current gate is fresh review of the
  maximum-five-WorkItem Slice 3 Exit Contract
- Slice 3 Exit Contract SHA256: `a681b1cd...b74e0af8`; fresh independent
  Contract Reviewer: `PASS`; findings/amendments: none
- S3-W1 frozen boundary: complete pure Bridge v1 JSONL frame and immutable
  bound-Run stream validation; no process, persistence, Grant, execution, ACK
  payload semantics, or activation
- S3-W1 Contract Review 1: `FAIL` only for contradictory eleven-plus-payload
  versus twelve-total field wording; no other finding
- S3-W1 Contract Amendment 1: exact twelve top-level fields and exact-12
  missing/extra proof; no API or authority change
- S3-W1 Amendment 1 Contract Review: `PASS`; findings: none
- S3-W1 mandatory RED: marker-only absence failure, followed by complete
  behavioral compile failure on missing frozen symbols before product code
- S3-W1 Candidate verification: focused `PASS`; focused race `-count=100`
  `PASS`; repository/race/vet/fuzz/format/diff/export checks `PASS`
- S3-W1 Implementation Review 1: `PASS`; findings: none; independent
  focused/race/full/race-full/vet/fuzz/format/scope/export checks `PASS`
- S3-W1 final pre-commit matrix and exact staged-scope audit: `PASS`
- S3-W1 accepted; its atomic local commit records the product, tests,
  governance, evidence, and reviewed Slice 2-to-3 transition
- S3-W1 accepted local commit: `c21a8f1`
- S3-W2 frozen boundary: one Journal multi-stream-head CAS plus atomic
  create/assign, claim/capacity, lease/reclaim, start/terminal, and projection
  authority; no thin writer/projection/coordinator split
- S3-W2 Contract Review 1: `FAIL` on accepted Runtime stream spelling,
  executor-to-`done` leakage, reclaim of running Runs, and lexical
  cross-stream replay
- S3-W2 Amendment 1: accepted Runtime stream, successful Run to
  `ready_for_review`, claimed-only reclaim, and dependency-aware two-pass
  projection; no API/owned-scope expansion
- S3-W2 Amendment 1 Contract Review 1: `PASS`; findings: none
- S3-W2 mandatory RED: `PASS` as RED; all seven markers exactly once and
  focused compile failure only on missing frozen S3-W2 symbols/behavior
- S3-W2 minimal GREEN: Journal multi-stream-head CAS focused test `PASS`
- S3-W2 implementation feasibility finding: restart-safe Authority Snapshot
  cannot enumerate all facts through existing known-stream-only Store reads
- S3-W2 Amendment 2: add deterministic mutation-isolated read-only
  `Store.ReadAll`; no registry Event, DB handle, writer, owned-scope, or
  authority expansion
- S3-W2 Amendment 2 Contract Review 1: `PASS`; focused ReadAll RED then Journal
  package GREEN
- S3-W2 compatibility finding: capacity facts in `runtime_instance:` would
  break accepted adjacent Runtime status sequences and require an unauthorized
  StateWriter change
- S3-W2 Amendment 3: separate `runtime_capacity:<id>` facts plus exact CAS of
  both accepted Runtime status head and capacity head; owned scope unchanged
- S3-W2 Amendment 3 Contract Review 1: `FAIL`; live status-head CAS ordering
  was not persisted and therefore not auditable by replay
- S3-W2 Amendment 4: required status stream/sequence/Event reference on
  S3-W2 Run/capacity facts; no public API/Event envelope/owned-scope expansion
- S3-W2 Amendment 4 Contract Review 1: `PASS`; offline-before/after,
  exact observed status head, paired-fact identity, and degraded terminal
  cleanup are replay-auditable with no public/owned-scope expansion
- S3-W2 Amendment 4 focused RED: `PASS` as RED; existing behavior polluted the
  accepted status stream, left the capacity stream empty, and omitted the
  frozen persisted status-head reference
- S3-W2 complete Candidate verification: focused `PASS`; focused race
  `-count=30` `PASS`; repository/race/vet/fuzz/format/diff/marker/export/scope
  checks `PASS`; real Journal reopen and historical-capacity replay proof pass
- S3-W2 Implementation Review 1: `FAIL` on evidence only; no owned product,
  safety, concurrency, projection, trust-boundary, or Slice 4 defect found;
  unchanged Pi fixtures failed while Reviewer and Controller full suites
  overlapped
- S3-W2 Review 1 evidence repair: exact `go test ./... -count=1` and exact
  repository race command each passed three consecutive isolated attempts;
  no waiver, product change, package exclusion, or `-p 1`
- S3-W2 Implementation Review 2: `PASS`; findings: none; independent
  focused/full/race/vet/format/diff checks `PASS`
- S3-W2 fresh pre-commit matrix: focused, 30-run focused race, repository,
  repository-race, vet, fuzz, format, diff, marker, export, and scope checks
  `PASS`
- S3-W2 accepted local commit: `5517a06`; post-commit focused and repository
  tests `PASS`
- S3-W3 frozen boundary: one AgentGrant local security authority for random
  one-time token issuance, hash-only persistence, exact Run/generation/
  operation binding, linearized authorization, rotation/revocation, and
  rebuildable projection; no Broker, adapter, supervisor, or Team DAG
- S3-W3 Contract Review 1: `PASS`; findings: none
- S3-W3 Amendment 1: correct impossible Run-versus-Grant one-loser proof to
  the two safe serial orders and freeze per-Run cross-generation RequestID
  uniqueness; no API/owned-scope/capability expansion
- S3-W3 Amendment 1 Contract Review 1: `PASS`; findings: none
- S3-W3 mandatory RED: `PASS` as RED; all six markers exactly once and focused
  compile failure only on missing frozen S3-W3 symbols/behavior
- S3-W3 complete Candidate: one-time random token issuance, hash-only Event
  persistence, exact Run/generation/operation binding, linearized
  authorization, rotation/revocation, and strict rebuildable projection
- S3-W3 security repair: targeted RED proved nested `%#v` could disclose the
  token through the containing struct; private redacting storage now keeps
  `%v`, `%+v`, `%#v`, JSON, Events, SQLite, projection, and errors clean
- S3-W3 Candidate verification: focused `PASS`; focused race `-count=30`
  `PASS`; repository/race/vet/fuzz/format/diff/marker/export/coverage/scope/
  token-leak checks `PASS`; authorization coverage 85.3%, projection coverage
  82.8%
- S3-W3 Implementation Review 1: `FAIL`; four in-scope product findings:
  S3-W2 opaque-ID compatibility, operational historical Run-reference
  validation, four-way Journal conflict normalization, and duplicate JSON-key
  rejection
- S3-W3 Repair 1 freezes exactly those four closures; no API, Event schema,
  owned-scope, or capability expansion
- S3-W3 Repair 1 Contract Review 1: `PASS`; findings: none
- S3-W3 Repair 1 mandatory RED: `PASS` as RED; all four markers exactly once
  and focused failures only on the four frozen product gaps
- S3-W3 Repair 1 Candidate: all four findings closed with no public API/Event
  schema/owned-scope/capability expansion
- S3-W3 Repair 1 verification: focused, 30-run focused race, repository,
  repository-race, vet, fuzz, format, diff, all ten markers, export, coverage,
  scope, and token-surface checks `PASS`; authorization coverage 85.4%,
  projection coverage 82.9%
- S3-W3 Implementation Review 2: `PASS`; findings: none; independent
  focused/full/race/vet/fuzz/format/diff/marker checks `PASS`
- S3-W3 fresh final pre-commit matrix: focused, 30-run focused race,
  repository, repository-race, vet, fuzz, format, diff, marker, export, and
  exact-scope checks `PASS`
- S3-W3 accepted for exact atomic local commit
- Current gate: S3-W4 contract governance; no S3-W4 product work before fresh
  Contract Review `PASS`
- S3-W3 accepted local commit: `47b4b50`; post-commit focused and repository
  tests `PASS`
- S3-W4 frozen boundary: private managed workspace/source invariance,
  configured Pi stdio adapter, bounded Bridge session, per-frame Grant
  authorization, process-group cancel/timeout cleanup, change capture, and
  supervisor terminal/revocation in one vertical Candidate
- S3-W4 Contract Review 1: `FAIL`; hardlink/path-race proof was incomplete and
  child `failed` result conflicted with later prose
- S3-W4 Amendment 1: Unix single-link/no-follow/identity proof, non-Unix
  fail-closed behavior, and exact child succeeded/failed terminal semantics;
  no API/Event schema/owned-scope/capability expansion
- S3-W4 Amendment 1 Contract Review 1: `PASS`; findings: none
- S3-W4 mandatory RED: `PASS`; focused command exited `1` only on the frozen
  missing Workspace, Supervisor, and Pi execution-adapter symbols; all eight
  markers occur exactly once and no product file existed
- S3-W4 preliminary Candidate: focused/full/race/vet/fuzz/cross-compile
  development checks reached GREEN, including the exact combined 30-run race
  command; pre-review security self-audit found one remaining
  intermediate-directory replacement proof gap
- S3-W4 Amendment 2: add only Unix descriptor-rooted `openat/fstatat` traversal
  plus a non-Unix fail-closed stub; no API/dependency/Event/capability change
- S3-W4 Amendment 2 Contract Review 1: `PASS`; findings: none
- S3-W4 Amendment 2 behavioral RED: `PASS` as RED; the focused command exited
  `1` because the old lexical traversal accepted the controlled
  intermediate-directory symlink replacement
- Current gate: minimal descriptor-rooted Amendment 2 implementation in the
  two reviewed platform files; no Runtime capability is activated
- S3-W4 exact 30-run race verification Repair 1 RED: Supervisor passed, while
  Pi adapter exposed a test readiness race in which the fixed 750ms
  cancellation deadline could fire before the helper wrote its grandchild PID
  proof
- S3-W4 Verification Repair 1 scope: test-only explicit PID readiness before
  cancellation; product behavior/API/authority/capability unchanged
- S3-W4 exact 30-run race Verification Repair 2 RED: Supervisor passed, while
  Pi adapter exposed the prohibited concurrent `Cmd.Wait` versus
  `StdoutPipe`/`StderrPipe` drain race as `read |0: file already closed`
- S3-W4 Verification Repair 2 scope: adapter-owned output pipe readers so
  `Wait` cannot close them before drain; public API/protocol/authority/
  capability unchanged
- S3-W4 independent Problem Analysis: confirmed product-level `Wait` versus
  managed-pipe defect and caller-owned `os.Pipe` as the minimal safe repair;
  treating closed pipes as EOF or delaying the fixture would be unsafe
- S3-W4 Verification Repair 2 focused GREEN: fast-exit/config and
  cancel/grandchild cases passed `-race -count=30` in 124.202s; static
  no-managed-pipe and near-limit fast-stderr regressions added
- S3-W4 exact post-repair 30-run race: `PASS`; Supervisor 123.221s, Pi adapter
  555.437s
- S3-W4 post-repair matrix: focused, coverage (Supervisor 81.3%, Pi 83.9%),
  repository, repository-race, vet, fuzz (8,207 executions), format, diff,
  eight-marker, static boundary, Linux compile, Windows compile, dependency,
  and scope checks `PASS`
- Current gate: fresh independent S3-W4 Implementation Review; no Runtime
  capability is activated
- S3-W4 Implementation Review 1: `FAIL`; P1 raw Grant path-text gap and P2
  Darwin fuzz fixture filename-rejection misclassification
- S3-W4 Implementation Repair 1: existing owned files only; no API/Event/
  authority/dependency/scope/capability expansion
- S3-W4 Repair 1 RED: source-token filename reached the adapter and
  workspace-token filename returned a successful change; independent fuzz RED
  failed during platform fixture creation before product invocation
- Current gate: Repair 1 focused GREEN and full verification matrix; no
  Runtime capability is activated
- S3-W4 Repair 1 focused GREEN: exact token-path cases `PASS`; 10s fuzz
  `PASS` with 19,245 executions
- S3-W4 Repair 1 impact race: Supervisor `PASS` for 30 runs in 103.366s; Pi
  package failed after 528.388s only in accepted S2-W18 metadata fixture whose
  shared 3s timeout expired before its first diagnostic
- Impacted S2-W18 subtest focused `-race -count=100`: `PASS` in 47.699s
- S3-W4 Amendment 3: test-only ownership of
  `internal/runtime/piadapter/process_runner_test.go` to raise only the shared
  non-timeout fixture bound to at most 10s; explicit 100ms timeout proof and
  all product boundaries unchanged
- Current gate: fresh independent Amendment 3 Contract Review before changing
  the accepted-prerequisite test file; no Runtime capability is activated
- S3-W4 Amendment 3 Contract Review 1: `PASS`; findings: none
- S3-W4 Amendment 3 implementation: shared non-timeout accepted fixture bound
  3s -> 10s only; explicit 100ms timeout proof and product code unchanged
- Current gate: focused Amendment 3 proof then exact combined race matrix; no
  Runtime capability is activated
- S3-W4 Amendment 3 focused proof: impacted subtest `-race -count=100` `PASS`
  in 50.131s; metadata timeout/cancellation/process-group
  `-race -count=30` `PASS` in 49.754s
- S3-W4 Repair 1 exact combined race: `PASS`; Supervisor 101.281s, Pi adapter
  552.160s
- S3-W4 Repair 1 final matrix: focused, coverage (Supervisor 81.3%, Pi 82.7%),
  repository, repository-race, vet, 10s fuzz (20,185 executions), format,
  diff, marker, static boundary, Linux/Windows compile, dependency, and scope
  checks `PASS`
- Current gate: fresh independent S3-W4 Implementation Repair 1 Review; no
  Runtime capability is activated
- S3-W4 Implementation Repair 1 Review 1: `FAIL`; repeated race exposed empty
  `grandchild.pid` readiness window from helper `os.WriteFile`
- Same-type readiness failure threshold reached; fresh read-only Problem
  Analyst invoked before Repair 2
- S3-W4 Implementation Repair 2 frozen test-only: same-directory temporary PID
  file + atomic rename, reader accepts only a valid positive PID; no product/
  API/protocol/authority/dependency/scope/capability change
- Current gate: fresh Problem Analysis then Repair 2 RED/GREEN; no Runtime
  capability is activated
- S3-W4 Repair 2 Problem Analysis: confirmed test-only PID publication/
  consumption race; existing frozen Repair 2 scope sufficient
- S3-W4 Repair 2 implementation: helper temporary-file write+close+atomic
  rename; reader accepts only parsed positive PID and bounded-cleans on every
  readiness failure
- Current gate: Repair 2 focused repeated race proof; no product or Runtime
  capability is activated
- S3-W4 Repair 2 focused cancellation/grandchild `-race -count=100`: `PASS`
  in 130.480s
- S3-W4 Repair 2 exact combined race: `PASS`; Supervisor 99.335s, Pi
  546.046s
- S3-W4 Repair 2 latest matrix: repository, repository-race, vet, 10s fuzz
  (20,862 executions), format, diff, marker, static boundary, Linux/Windows
  compile, dependency, and scope checks `PASS`
- Current gate: fresh independent S3-W4 Implementation Repair 2 Review; no
  product or Runtime capability is activated
- S3-W4 Implementation Repair 2 Review 1: `PASS`; findings: none; independent
  cancellation 30-run race, token paths, fuzz, related ordinary/race,
  repository, vet, format, diff, dependency, marker, and static checks `PASS`
- S3-W4 implementation deliverable: `VERDICT: PASS`
- Current gate: fresh final pre-commit matrix and exact Candidate staging; no
  product or Runtime capability is activated
- S3-W4 fresh final pre-commit matrix: exact 30-run race `PASS` (Supervisor
  103.432s, Pi 541.156s), repository, repository-race, vet, fuzz (12,013
  executions), format, diff, marker, static boundary, Linux/Windows compile,
  dependency, and scope checks `PASS`
- S3-W4 accepted for exact authorized local atomic commit; no Runtime
  capability is activated
- Verification used deterministic temporary fixtures only; installed Pi, user
  Pi state, credentials, network, package manager, daemon, session, prompt,
  model call, and Runtime activation were not used
- No push, merge, release, runtime activation, credential change, paid remote
  work, or FastContext installation is authorized

## Drafts (in `.loom-drafts/`, untracked, awaiting review)

- `phase1-slice1.optimized.goalspec.yaml` — 220 lines, v2 optimized GoalSpec
- `new-session-prompt.md` — 68 lines, new Codex session launch prompt
- `phase-roadmap-supplements.md` — v0.4 (~770 lines), 4-version evolution
  - v0.1: Phase 1-4 structure with "可吸 / 不吸 / 主动排除"
  - v0.2: + Omnigent + Codex App + 5-layer design model + 7 invariants
  - v0.3: + Claude Code Hooks 22 events + SKILL.md 开放标准 + Codex Record&Replay
  - v0.4: + MCP 2025-11-25 wire format + envelope diff vs Loom Bridge + 3 alignment options + recommended option C (JSON-RPC 2.0 v1.1) + 5 Phase 1 必决决策
- `community-survey-v1.md` — 7 个用户指定 repo 实地验证 (web_fetch GitHub 主页)
- `community-survey-v2.md` — 14 个项目按 4 category 横向调研 (Goose/OpenCode/Aider/Cline/Continue/Pi + Letta/LangGraph/PraisonAI/Pydantic-AI + Temporal/Inngest/Restate/Prefect + Langfuse/Helicone/Phoenix/LangSmith)
  - **AOS Community Edition (aos-ce) ⭐6.9k**：强相关 — Rust, 21 capsules, `aos mcp serve` 是 Codex/Claude/Grok 共享 product edge
  - **open-connector**：4+ 同名低星项目，最相关 `openconnector-dev/openconnector` (1★) — 还在 source code 阶段
  - **colibri (trending 17.5k)**：实际 GitHub `Colibri` 账号 0 star — trending 数字疑似错配
  - **wloc (trending 5.9k)**：`wloc-org/wloc` 404 — 不存在
  - **torlink**：多个 Tor P2P / Tor link 列表项目 — 不相关
  - **Codex-Dream-Skin ⭐12.1k**：Codex 桌面端换肤 (CDP 远程调试) — 不相关
  - **exploitarium**：公开 exploit PoC 仓库 — 不相关

## 2026-07-24 社区调研 v1 关键发现 (community-survey-v1.md)

- **AOS CE = Loom 最值得深挖的"下游消费者"候选** — Rust agent 操作系统, 21 first-party capsules, `aos mcp serve` 是 Codex/Claude/Grok 共享的 MCP edge
- **AOS CE 关键设计**：
  - Product CLI `aos` 拥有 `init/status/migrate/update/distro/mcp/serve-health` roots
  - Unicity Audit: Sigstore bundles + GitHub build-provenance attestations + `runtime-compatibility.toml` pins
  - 闭源 AOS 之上，用户用 Forge 工具自建 capsule（meta-harness 模式）
- **AOS CE vs Loom 对比**：
  - 同：capsule ↔ subagent 同构（用户空间能力块）
  - 同：`aos mcp serve` ↔ Loom Bridge v1.1（MCP 集成方向一致）
  - 异：AOS CE 是 product surface（CLI + HTTP API），Loom 是 view layer（3 views）
  - 异：AOS CE 鼓励"用户自造 capsule"，Loom 鼓励"Planner 派发已有 subagent"
  - 同：两者都不造 LLM runtime（Loom 拒绝代理 LLM，AOS CE 依赖 astrid runtime）
- **AOS CE 待深挖项**：`docs/meta-harness.md` / `docs/release-channels.md` / 21 capsules 分类 / `aos mcp serve` 协议实现
- **trending 数字不可信**：colibri 给 17.5k 实际 0, wloc 给 5.9k 实际 404 — 必须 web_fetch GitHub 验证
- **下次 v0.5 调研方向**：AOS CE 深挖（Rust 21 capsules + meta-harness 模型）作为 Phase 2/3 Loom 借鉴参考

## 2026-07-24 v0.4 调研摘要

- **MCP wire format = JSONL over stdio + JSON-RPC 2.0 envelope** — Loom Bridge Phase 1 已选 JSONL ✓ 对齐
- **Envelope 不兼容**：Loom 用 `type/payload`，MCP 用 `method/params`/`result`/`error`
- **推荐选项 C**：Loom Bridge v1.1 升级到 JSON-RPC 2.0 envelope（加 `jsonrpc: "2.0"` + 改 `type` → `method` + `payload` → `params`/`result`/`error` + 保留 `loom.*` 业务字段）
- **成本**：v1.0 → v1.1 是字段 rename，shim 兼容
- **收益**：Phase 2 接入 MCP 生态（1000+ 现成 Server）零迁移
- **MCP version pin**：2025-11-25（v1 现行稳定 + Anthropic / OpenAI / Google / MS 全员支持）
- **不跟进 v2 SDK**：2026-07-28 beta 出来后等 GA + 6 个月生产验证
- **Loom 拒绝代理 LLM**：`sampling/createMessage` 收到直接 fail closed，保持 Loom = 编排 + 观测
- **Tool annotations ↔ Loom Risk 字段**：直接映射（readOnlyHint / destructiveHint / idempotentHint / openWorldHint）
- **Subagent ≠ Sampling**：正交概念，Bridge 不实现 Sampling 原语
- **未来 Loom 文档待补**：5 个新文件（bridge-envelope-v1.1.md / mcp-host.md / mcp-server.md / mcp-version-pin.md / `TECH-PLAN.md §7` 修订）

## 2026-07-24 社区调研 v2 关键发现 (community-survey-v2.md)

**调研方法升级**：吸取 v1 教训，12 个 web_fetch 大部分截断。这次改用 `raw.githubusercontent.com/owner/repo/branch/README.md` 直接抓原始 markdown — 14 个项目全部成功。

**14 个项目按 4 类别横向对比**：

| Category | 项目 | License | 关键定位 | Loom 借鉴度 |
|---|---|---|---|---|
| **Cat 1: Coding agent** | Goose | Apache 2.0 | Linux Foundation AAIF, 15+ providers, 70+ MCP extensions, Custom Distributions | 中 (Distro 概念) |
| | OpenCode | MIT | TypeScript, build/plan/general 三 agent (Tab 切换) | 中 (双 agent 模式) |
| | Aider | Apache 2.0 | 6.8M PyPI installs, 88% singularity (Aider 自写) | 低 (Singularity evidence) |
| | **Cline** | Apache 2.0 | Coordinator + specialists + SDK @cline/sdk + 4 product surfaces | **强** (multi-agent model) |
| | Continue | Apache 2.0 | ⚠️ **ARCHIVED** 2.0.0 final release — 不再维护 | 教训 (商业模式) |
| | **Pi** | MIT | 4 工具极简 + YOLO + Session tree + Steering 队列 | **强** (极简哲学 + 树状 history) |
| **Cat 2: Stateful agent** | Letta | MIT | Memory-first, 旧 repo 留 V1 server, active dev = letta-code | 中 (memory + skills) |
| | **LangGraph** | MIT | "Durable execution" + "Interrupts" + 灵感来自 Pregel/Beam | **强** (durable execution 命名) |
| | **PraisonAI** | MIT | 25+ features 全参考, 14μs instantiation, MCP+A2A+Policy+Memory | **强** (25+ features 列表 reference) |
| | **Pydantic AI** | MIT | "FastAPI feeling", Capabilities composable bundles, Pydantic Stack | **强** (FastAPI feeling + capabilities) |
| **Cat 3: Durable execution** | **Temporal** | MIT | 8+ years mature, Workflows/Activities/Workers + Replay | **强** (Replay 模式) |
| | Inngest | SSPL+DOSP | Event API → Stream → Runner → Queue → Executor | 中 (Flow control primitives) |
| | **Restate** | Apache 2.0 | **"Durable AI Agents" 显式 use case**, Exactly-once + Suspending + Durable Promises | **强** (exactly-once + suspend) |
| | Prefect | Source-available | Python workflow orchestration (decorator pattern) | 低 (Python 深度) |
| **Cat 4: LLM observability** | **Langfuse** | MIT | 16k★, ClickHouse-based, YC W23 (acquired by ClickHouse 2026-01), massive 集成生态 | **强** (OpenAPI spec + integration) |
| | Helicone | Apache 2.0 | AI Gateway 100+ models, Cloudflare Workers proxy | 中 (商业模式不同) |
| | **Arize Phoenix** | Elastic 2.0 ⚠️ | OTel-based, **Remote MCP Server** 内置, OpenInference standards | **强** (OTel + Remote MCP) |
| | LangSmith | Closed | LangChain 商业, TraceID/SpanID/ParentID | 中 (APM 模式) |

**3 大跨 category 模式（Loom 必须借鉴）**：

1. **"Coordinator + specialists" 是 multi-agent 事实标准**
   - OpenCode (build + plan + general) / Cline (coordinator + specialists) / Pi (Steering + Follow-up + 4 tools) / LangGraph (Deep Agents = subagents) / PraisonAI (Orchestrator Workers) 同构
   - **Loom 直接对位**: Planner = coordinator, subagent = specialists

2. **"Durable execution" 是 agent 必备**
   - Temporal (Workflows + Activities + Replay) / Inngest (Event-driven + Flow control) / Restate ("Durable AI Agents" 显式 use case + Exactly-once + Suspending + Durable Promises) / LangGraph / Pydantic AI
   - **Loom 直接对位**: Run claim + heartbeat = durable execution 的 view layer 实现（验证 Loom 方向正确）

3. **"MCP + Remote MCP Server" 是 2026 行业新趋势**
   - Goose 70+ extensions via MCP / Cline `cline mcp` / PraisonAI 4 transport / Pydantic AI MCP
   - **Arize Phoenix 直接把 observability 暴露为 Remote MCP Server**（让 Claude Code / Cursor 直接 query traces）
   - **Loom 直接对位**: Bridge v1.1 (JSON-RPC 2.0) + Phase 2 MCP + cost/governance view 暴露为 Remote MCP Server

**Loom 应该做 vs 不做（关键边界）**：

| ✅ 应该做 | ❌ 不应该做 |
|---|---|
| view layer (不抢 agent 产品) | agent runtime (那是 Goose / Cline / Aider) |
| Event Journal (append-only, rebuildable) | memory model (那是 Letta / Pydantic AI) |
| durable execution 的 view | durable execution 本身 (那是 Temporal / Restate) |
| "Coordinator + specialists" 编排协议 | multi-agent framework 本身 (那是 LangGraph) |
| cost / governance view | observability 平台 (那是 Langfuse / Phoenix) |
| Bridge v1.1 (JSON-RPC 2.0) | subagent 完整 SDK (那是 Cline @cline/sdk) |
| CLI + run claim | IDE 集成 / Desktop (那是 Cline / Continue) |
| local-first + MIT | SaaS-first + Elastic License (那是 Phoenix) |

**Permanent "不吸" 清单 (v2 新增)**：
- **Continue**: 已停维护 = 行业教训 ("vibe-coded weekend project" 死法)
- **Prefect**: 商业化强 + Python 深度集成 + Cloud-first (跟 Loom local-first 冲突)
- **Helicone**: 商业化 AI Gateway (跟 Loom "fail closed on sampling/createMessage" 冲突)
- **LangSmith**: 闭源商业 (跟 Loom MIT 倾向冲突)
- **Arize Phoenix (license only)**: Elastic License 2.0 ≠ OSI 开源 (Loom 应该避免)

**借鉴优先级**：
- **P0 (Phase 1 必借鉴)**: Cline multi-agent model / LangGraph durable execution 命名 / Pydantic AI capabilities bundle
- **P1 (Phase 2 应该借鉴)**: Phoenix Remote MCP Server + OTel / Langfuse 集成生态 / Restate exactly-once
- **P2 (Phase 3+ 长期)**: Temporal replay-based resume / Langfuse OpenAPI spec / LangSmith env var auto-integration

**决策（Phase 1 必决）**：
1. **不学 Helicone 做 AI Gateway** — Loom 严格 fail closed on sampling/createMessage
2. **不学 Phoenix 用 Elastic License 2.0** — Loom 倾向 MIT
3. **Phase 2 cost/governance view 暴露为 Remote MCP Server** — 学 Phoenix 模式
4. **Phase 1 Bridge v1.1 用 "Coordinator + specialists" 命名** — 学 Cline

**调研方法改进（memory 候选）**：
- ❌ 之前 `web_fetch github.com/owner/repo` 大部分被 chrome 截断
- ✅ 改用 `web_fetch raw.githubusercontent.com/owner/repo/branch/README.md` 直接拿原始 markdown
- 14 个项目全部成功，是 v1 方法的有效升级

## 2026-07-24 社区调研 v3 前沿深挖 (community-survey-v3-deepdive.md, 45K 字节)

**调研范围**: v2 14 项目 / 4 category 基础上, 选最活跃项目深挖 2026-Q3 演进, 抓 8 README + 8 web_search 2026 月度新鲜。

**3 大 2026-Q3 行业共识 (TL;DR)**：

1. **"Coordinator + specialists" 是 multi-agent 事实标准** — OpenAI + Anthropic 2026 双向背书 (能用单 Agent 解决的不堆 Agent, 业界 80% "成功案例" 是 Subagent 主从模式)
2. **"Durable execution" 是 agent 必备** — Restate "Durable AI Agents" 第一 use case, Temporal Replay 2026 主推, DBOS Conductor + 新 MCP server, Microsoft pg_durable 内嵌 SQL
3. **"MCP + Remote MCP Server" 是 2026 行业新趋势** — MCP 2026-07-28 RC 史上最大修订, Arize Phoenix / Langfuse / 企查查 9 server / 197 tool 全暴露

**Cat 1 (Coding Agent) 2026-Q3 深挖**：
- **Cline** 6,614 commits + 4 product surfaces (CLI/Kanban/VS Code/JetBrains) + @cline/sdk + Multi-Agent Teams (`cline --team-name auth-sprint`) + Scheduled Agents (`cline schedule create`)
- **Pi 大 rebrand** `badlogic/pi-mono` → `earendil-works/pi-mono`, 4 packages (pi-ai / pi-agent-core / pi-coding-agent / pi-tui), supply-chain hardening (`min-release-age=2` / `save-exact=true`), 3 containerization patterns (Gondolin / Docker / OpenShell)
- **OpenCode 简化** 3 agents → 2 agents (build/plan), 22 语言本地化, Desktop BETA
- **共识**: 拒绝内置复杂 permission / Coordinator+specialists / 多 provider / 简化极简 / Session 持久化

**Cat 2 (Stateful Agent) 2026-Q3 深挖**：
- **Pydantic AI Harness v0.5.0** (官方 capability 库, 2026-07) — 22 capability area 含 CodeMode (Monty 沙箱) / ToolSearch / Sub-agents / Memory / StuckLoopDetection / CostTracking / SecretRedaction。Pydantic Stack = Pydantic AI + Logfire + **Logfire AI Gateway**
- **subagents-pydantic-ai v0.5.0** (94 commits) — sync/async/auto 3 mode, nested subagents, dynamic agent creation, SubAgentSpec YAML/JSON, question mode
- **LangChain 1.0** (2025-10) — Middleware 洋葱圈 (5 hook: before_agent/after_agent/before_model/wrap_model_call/after_model/wrap_tool_call), `create_agent` 10 行起步, PIIMiddleware
- **LangChain 1.1.0** (2025-11-24) — Model Profiles `.profile` 属性
- **LangGraph 1.0** — 5 pillars, Deep Agents (高级包: planning + filesystem + subagents + memory), 灵感 Pregel/Beam/NetworkX
- **create_deep_agent** 18 参数签名 + **三级上下文压缩** (tool input offloading / tool result offloading > 20k token / summarization 85% 窗口阈值) + 10 中间件栈固定顺序
- **OpenAI + Anthropic 2026 双向背书** Subagent 主从模式

**Cat 3 (Durable Execution) 2026-Q3 深挖**：
- **Restate "Durable AI Agents"** 6 primitives + 5 SDKs + SemVer 友好 (x.y → x.y+1 无需 manual migration)
- **Temporal Replay 2026** 4 大新功能: Serverless Workers / Standalone Activities / Workflow Streams / Google ADK + OpenAI Agents SDK 集成。NVIDIA / Salesforce / Twilio / Descript / OpenAI Venkat (ex-Rockset) 全推荐
- **DBOS Conductor** (2026-07) — 新 MCP Server + OpenMetrics + RBAC + Bulk Workflow forking + Google ADK plugin + DBOS Transact for Java 1.0
- **Microsoft pg_durable** (NEW 2026) — **PostgreSQL extension, SQL 关键字** `df.start() |=> 'name' ~> 'sql'` + pgrx + duroxide runtime + 0.2.2 latest
- **Hatchet** — 3,208 commits, Postgres-based, **~416 jobs/s**, 11ms p50 insert→result, multi-tenant OTEL
- **Belay (Elixir)** NEW 2026-07 — Journal-based, 1.0.0-rc.5, **$USD budget per job** (3 层 cost 控制), 99,004 jobs / 7h soak test 0 violations, MCP server built-in
- **共识**: AI workflow 是 2026 主 use case / Postgres 作为底层 / 不自己造 durable engine (Loom 应该 view layer 选 Restate/DBOS 接入)

**Cat 4 (LLM Observability) 2026-Q3 深挖**：
- **Arize Phoenix 2026-Q3** — **PXI (Phoenix Intelligence) AI debugging agent** + **Remote MCP Server 内置** + `.agents/skills/` 多 editor (Claude Code/Codex/Cursor 同步) + 25+ Python + 7 TS + 2 Java + 2 Go integrations
- **OpenInference 1,948 commits** — **2026-06 semantic conventions 正式进入 OpenTelemetry GenAI 工作组** (`spec/reasoning` PR #3112) — 这是 2026 observability 行业最大事件
- **OpenTelemetry** — **CNCF graduated**, 12+ languages, 200+ collector components, 1020+ integrations
- **traceloop/openllmetry v0.49+** — "Our semantic conventions are now part of OpenTelemetry!"
- **Langfuse (ClickHouse 2026-01 acquired)** — 16,054 stars, **50M+ SDK installs/month**, **10B+ observations/month**, 2,300+ customers, 99.9% uptime, **Coding agents SKILL.md 新发布 + Platform MCP Server**
- **🆕 MCP 2026-07-28 RC 史上最大修订** — 4 大生产化信号: **协议从"会话绑定"走向"请求自包含"** + **能力从"列出来"走向"管起来"** + **任务从"一次调用"走向"持续完成"** + **结果从"模型说了什么"走向"依据能否还原"**。5 大变化: 无状态核心 + 能力发现 + 缓存 + Extensions 一等公民 + Tasks + MCP Apps
- **🆕 企查查 MCP 2026-07** — 9 Server / 197 tool / 27 SKILL, **5 层能力矩阵** (Tool → Server → Resources → SKILL → 全局约束), 适配 WorkBuddy/QoderWork/QClaw/IMA/MiniMax/LobsterAI
- **共识**: MCP 是 2026 必备 surface / Coding agents 是 2026 必备用户 / OTel 是 trace 标准 / 不重建 observability 平台

**v3 新增借鉴优先级**：

| P0 (Phase 1 必做) | 来源 | Loom 落地 |
|---|---|---|
| **Coordinator + specialists** 命名 | Cline / OpenCode / Pi | Bridge v1.1 `loom.dispatch` |
| **JSON-RPC 2.0 envelope** | MCP 2026-07-28 | Bridge v1.1 (已定) |
| **W3C Trace Context 传播** | MCP 2026-07-28 | Bridge v1.1 trace 字段 |
| **完整 JSON Schema 2020-12** | MCP 2026-07-28 | Bridge v1.1 method schema |
| **Capabilities = composable bundles** | Pydantic AI Harness v0.5.0 | Bridge v1.1 method dispatch |
| **Middleware 洋葱圈** 模式 | LangChain 1.0 | daemon 中间件栈 |
| **SubAgent 双类型** (SubAgent dict + CompiledSubAgent) | LangGraph Deep Agents | subagent spec |
| **$USD budget per run** | Belay | Run claim 必填 |
| **OTel spans 作为 trace 出口** | OpenTelemetry CNCF | daemon observability |
| **多 editor skill 同步** (`.agents/skills/`) | Phoenix | `.loom-drafts/skills/` |
| **Remote MCP Server 暴露** (Phase 2 但 P0) | Phoenix / Langfuse | cost view 准备 |
| **5 层能力矩阵** (Phase 2 但 P0) | 企查查 MCP | cost view 准备 |

| P1 (Phase 2) | P2 (Phase 3+) |
|---|---|
| "Code mode" 概念 (Pydantic) | SQL 关键字路线 (Microsoft pg_durable) |
| 三级上下文压缩 (LangGraph Deep Agents) | Loom 出 "loom-harness" 包 |
| context_schema 模式 (LangGraph) | USD budget true-up (Belay) |
| DBOS 新 MCP server (DBOS Conductor 2026-07) | Logfire AI Gateway 统一 LLM proxy |
| Coding agents SKILL.md (Langfuse) | Temporal Workflow Streams |
| Postgres-based + OTEL (Hatchet) | |
| Resources 暴露稳定知识 (企查查 MCP) | |
| SKILL 组织业务流程 (企查查 MCP) | |
| PXI 概念 (Phoenix) — "loom-insight" AI helper | |
| Chaos test harness (Belay) — 7h kill -9 | |
| Exactly-once 语义 (Restate) | |
| OAuth 2.1 (MCP 2026-07-28) | |

**v3 新增 Permanent "不吸" 清单**：
- ❌ **Phoenix license (ELv2)** — 跟 Loom MIT 冲突
- ❌ **Helicone AI Gateway** — 跟 Loom "fail closed on sampling/createMessage" 冲突
- ❌ **LangSmith** — 闭源商业
- ❌ **Prefect** — Python 深度 + Cloud-first 跟 Loom local-first 冲突
- ❌ **Continue** — 已 ARCHIVED 2.0.0 行业教训
- ❌ **OpenInference 整套** — Loom 只需 OTel 输出, 不需要 25 integrations
- ❌ **Belay / Hatchet / DBOS 整套** — Loom 是 view layer, 选一个接, 不重做 durable engine
- ❌ **Temporal 整套** — 9 年成熟, Loom 不重做

**5 个新未来 Loom 文档待补（v3 落地）**：
1. `docs/bridge/loom-bridge-v1.1-architecture.md` — JSON-RPC 2.0 + W3C Trace + JSON Schema 2020-12 + Coordinator dispatch
2. `docs/integrations/mcp-2026-07-28-rc.md` — 借鉴 4 大生产化信号, 写 Loom Bridge v1.1 的 MCP 兼容性
3. `docs/architecture/subagent-spec.md` — SubAgent dict + CompiledSubAgent 双类型 + YAML 配置
4. `docs/integrations/dbos-vs-restate-vs-temporal.md` — Loom 推荐选哪个 durable engine 做底层
5. `docs/skills/loom-coding-agents.md` — 学 Langfuse / Phoenix, 写 Loom 自己的 SKILL.md 供 Claude Code / Codex / Cursor 调

**v3 调研方法学**：
- README 抓取: 全部用 `raw.githubusercontent.com/owner/repo/branch/README.md` (8/8 成功)
- web_search 时间过滤: `freshness=month` 拿 2026-Q3 最新
- 多角度交叉验证: GitHub README (官方) + web_search 月度新鲜 (2026-Q3 趋势) + PyPI/npm 包 (代码级实锤)
- Loom 借鉴落地: 每个项目先问 "该不该学" (P0/P1/P2/不吸), 再问 "学到 Bridge v1.1 / Phase 2 / Phase 3 哪里"

## Historical/Rejected Candidate — 2026-07-24 工作队列命名重排

> **Authority status (2026-07-25): historical Candidate, not executable.**
> This preserved research queue does not define Slice 2. Its assignments of
> Bridge v1.1, JSON-RPC/JSONL, AgentGrant, claim generation, prepare lease,
> WorkItem dispatch, or a real Runtime Adapter to S2-W1 are rejected because
> they conflict with `TECH-PLAN.md §14`, which keeps those capabilities in
> Slice 3. The current S2-W1 authority is the frozen contract linked in the
> top-level Slice 2 transition section.

**⚠️ 纠正记录 (2026-07-24 16:18 SGT)**:
- **错误**: 把"新调研产物"放进 S1-W1~W5 命名
- **正确**: S1 (Phase 1 Slice 1) 已完成, commit `5861f82`, 5 件 WorkItem 全部 PASS
  - S1-W1: `internal/mode` domain router (4 Agent triggers) — **不是** Bridge envelope
  - S1-W2: SQLite Event Journal + migrations + append-only triggers
  - S1-W3: Evidence Artifact Store (SHA-256 + atomic publish)
  - S1-W4: rebuildable projection + WorkItem + Evidence snapshot
  - S1-W5: `loom route` + `loom status` CLI + exit codes
- **新调研产物应该进 Slice 2 (S2-W1~W5)**, branch = `codex/loom-platform-slice2`, 状态 `READY_FOR_CONTRACT`
- **phase1-slice1.optimized.goalspec.yaml 是给 Slice 1 用的**, Slice 2 需要自己的 goalspec (项 #10)
- **没有再走一遍 S1 这回事**, Slice 1 已经 frozen commit `5861f82`

**重排时间**: 2026-07-24 16:18 SGT
**重排方式**: TodoWrite 15 项重排完成, 1 认错纠正 + 5 S2 work item + 3 S2 doc 收尾 + 1 Slice 2 spec + 5 Phase 2/3 分桶
**入队原则**: S2 work item 按顺序 → S2 收尾 doc 紧跟 → Slice 2 spec 准备 → Phase 2 准备 → Phase 2 docs → Phase 3+ 暂缓 → 长期监控

### 工作队列 (15 项, 1 纠正 + 5 S2 work item + 3 S2 doc + 1 spec + 5 P2/3 分桶)

| # | 优先级 | 状态 | 项 | 时机 |
|---|---|---|---|---|
| 1 | meta | **in_progress** | **[认错纠正] 新调研产物应进 S2-W\* 而非 S1 — S1 5 件已 PASS (commit 5861f82), 命名错位需重排** | now (本回合) |
| 2 | P0-high | pending | **S2-W1**: Bridge v1.1 envelope = JSON-RPC 2.0 + W3C Trace + JSON Schema 2020-12 + Coordinator dispatch (在 S1 mode router 之上) (4-5 天) | after #1 |
| 3 | P0-high | pending | **S2-W2**: Daemon 长驻进程 (S1 只做了 CLI 单次调用) + age 加密 (S1 没做) + LICENSE (MIT) + docs/governance/non-negotiables.md | after S2-W1 |
| 4 | doc | pending | **[S2-W1 收尾] 出 `docs/bridge/loom-bridge-v1.1-architecture.md`** | S2-W1 done |
| 5 | P0-high | pending | **S2-W3**: Coordinator + specialists dispatch 落地 + SubAgent 双类型 (SubAgent dict + CompiledSubAgent) — 调 Cline Multi-Agent Teams 模式 | after S2-W2 |
| 6 | doc | pending | **[S2-W3 收尾] 出 `docs/architecture/subagent-spec.md`** | S2-W3 done |
| 7 | P0-high | pending | **S2-W4**: $USD budget per run middleware 必填 + OTel SDK 接入 (默认 endpoint `localhost:4317` + W3C trace propagation) | after S2-W3 |
| 8 | P0-high | pending | **S2-W5**: Capabilities = composable bundles 架构 (Pydantic AI Harness 借鉴) + SKILL.md v1 (`.loom/skills/loom-bridge.md` 3 editor 同步) + 1h chaos test | after S2-W4 |
| 9 | doc | pending | **[S2-W5 收尾] 出 `docs/skills/loom-coding-agents.md` v1** | S2-W5 done |
| 10 | spec | pending | **Slice 2 spec**: 出 `phase1-slice2.goalspec.yaml` (S1-slice1 spec 是 Slice 1 用的, Slice 2 需要自己的 spec) | before S2-W1 contract freeze |
| 11 | Phase 2 P0 | pending | P0-10 `.agents/skills/` 多 editor 同步 (Slice 2 末尾先出 1 个最小版) + P0-11 Remote MCP Server 框架 (Slice 2 写好不开放) + P0-12 5 层能力矩阵规划 | after S2-W5 |
| 12 | Phase 2 P1 | pending | P1-2 三级上下文压缩 + P1-3 context_schema + P1-11 exactly-once (SQLite `INSERT OR IGNORE`, Slice 2 顺便加) + P1-12 OAuth 2.1 (Phase 2 末/Phase 3) | after #11 |
| 13 | Phase 2 docs | pending | `docs/integrations/mcp-2026-07-28-rc.md` (Phase 2 准备时) + `docs/integrations/dbos-vs-restate-vs-temporal.md` (Phase 3 选型前) + `docs/skills/loom-coding-agents.md` v2 | when ready |
| 14 | Phase 3+ 暂缓 | pending/cancelled | P1-1 Code mode 跳过 (Loom 不是 user 写代码场景) / P1-4 DBOS Conductor MCP Phase 3 选型后学 / P1-6 Postgres Phase 2 multi-user 才需要 / P1-9 loom-insight 3-6 月数据后 / P1-10 7h chaos Phase 3 才上 | 不定 |
| 15 | 长期监控 | pending | 每月 DBOS / pg_durable / MCP minor / Temporal Replay 2027 — 跟踪演化, 不主动 follow | 月度 |

### 5 件现在做 (Phase 1 Slice 2 主干, ~ 4 周, P0-high)

1. **S2-W1 (项 #2)**: Bridge v1.1 envelope = JSON-RPC 2.0 + W3C Trace + JSON Schema 2020-12 + Coordinator dispatch 4 件套 (在 S1 mode router 之上)
2. **S2-W2 (项 #3)**: Daemon 长驻进程 (S1 只做了 CLI 单次调用, 没守护进程) + age 加密 + LICENSE (MIT) + docs/governance/non-negotiables.md
3. **S2-W3 (项 #5)**: Coordinator + specialists dispatch + SubAgent 双类型 (SubAgent dict + CompiledSubAgent)
4. **S2-W4 (项 #7)**: $USD budget per run middleware 必填 + OTel SDK 接入
5. **S2-W5 (项 #8)**: Capabilities = composable bundles 架构 + SKILL.md v1 (3 editor 触发入口) + 1h chaos test

### Loom 客户端 3 形态 (用户要求 2026-07-24 16:36 SGT 落地)

**关键判断**: Loom 客户端 ≠ "前台 agent CLI" 跟 Codex 抢; Loom 客户端 = **view layer console** 取代 sqlite3+jq 查 journal。

| 形态 | 何时 | 队列项 | 作用 |
|---|---|---|---|
| **SKILL.md v1** (3 editor 触发) | **S2-W5** | 项 #8 内含 | Claude Code / Codex / Cursor 3 个 editor 同时调 Loom (走 daemon JSON-RPC 2.0) |
| **Loom Console TUI 最小版** (bubbletea + lipgloss) | **Slice 2 末尾先出 1 个** | **项 #11 (新加)** | journal/evidence/cost 实时观察器 (纯 view, 不思考不执行), 取代 sqlite3+jq |
| **Loom Console TUI 完整版** (含 cost dashboard + trace 串联) | **Phase 2 准备** | **项 #12 (新加合并)** | TUI 加 cost + trace view, 多人多机用 |
| **Loom Web** (本地静态, `localhost:7432`) | **Phase 3+ 长期** (等 TUI 跑稳) | **项 #15 (新加合并)** | 时间线 + 树状 + 验证 + dashboard, 等 TUI 跑稳再说 |

**Loom 客户端严格不抢**:
- ❌ 不做 "前台 agent CLI" 跟 Cline / OpenCode / Pi 抢 (思考+执行)
- ❌ 不做 IDE 跟 Cursor 抢
- ❌ 不做完整 observability SaaS 跟 Langfuse / Phoenix 抢
- ❌ 不开外部端口 (Loom Web 只在 localhost)

**类比**:
- Docker 时代: `docker ps` / `docker stats` CLI (看容器, 不跑容器)
- K8s 时代: `kubectl get pods -w` / Dashboard (看资源, 不写容器)
- AI agent 时代: **Loom Console TUI** (看 journal, 不写 agent)

### 调研文档归属 (什么时候读 + 读什么) — 修订

| 文档 | 何时读 | 频次 | 角色 |
|---|---|---|---|
| `phase1-slice1.optimized.goalspec.yaml` | **仅 Slice 1 启动 session 时**(已过, 历史) | 一次 (历史) | 历史 (S1 spec) |
| **`phase1-slice2.goalspec.yaml`** (待出) | **Slice 2 启动 session 时 (项 #10) → S2-W1 contract freeze** | 一次, 启动 S2 | 启动 S2 |
| `new-session-prompt.md` | **Slice 2 启动 session 时** | 一次, 配 S2 session | 启动 |
| `phase-roadmap-supplements.md` v0.4 | **每个 S2-W 启动前** | 每个 work item 一次 | 路线图 |
| `community-survey-v3-deepdive.md` | **P0 / P1 实现时按章节查** | 频繁 (主参考) | 主参考 |
| `community-survey-v2.md` | **总览查"X 项目借鉴了哪些"** | 中等 | 总览 |
| `community-survey-v1.md` | **历史背景, 不再 active 读** | 仅 v1 引用时 | 历史 |

### 借鉴优先级时间表 (什么时间做哪个 P0/P1) — 修订

| 项 | 时间 | 落地 |
|---|---|---|
| P0-1 Coordinator + specialists 命名 | **S2-W1** (项 #2) | Bridge v1.1 `loom.dispatch` |
| P0-2/3/4 JSON-RPC 2.0 + W3C Trace + JSON Schema 2020-12 | **S2-W1** (项 #2) | Bridge v1.1 envelope |
| P0-5 Capabilities = composable bundles | **S2-W5** (项 #8) | Bridge v1.1 method dispatch |
| P0-6 Middleware 洋葱圈 | **S2-W2/W4** | daemon 中间件栈 |
| P0-7 SubAgent 双类型 | **S2-W3** (项 #5) | subagent spec |
| P0-8 $USD budget per run | **S2-W4** (项 #7) | Run claim 必填 |
| P0-9 OTel spans trace 出口 | **S2-W4** (项 #7) | daemon observability |
| P0-10 多 editor skill 同步 | Phase 2 准备 (项 #11) | `.loom/skills/` |
| P0-11 Remote MCP Server | Phase 2 (项 #11) | cost view |
| P0-12 5 层能力矩阵 | Phase 2 (项 #11) | cost view |
| P0-13 MIT + view layer 严格 fail closed | **S2-W2** (项 #3) | LICENSE + governance doc |
| P1-2 三级上下文压缩 | Phase 2 (项 #12) | run 长后 |
| P1-3 context_schema | Phase 2 (项 #12) | multi-tenant |
| P1-11 exactly-once | Slice 2 顺便 (项 #7/8) | SQLite |
| P1-12 OAuth 2.1 | Phase 2 末 (项 #12) | auth |

### 关键判断 (从 5 件 锐利主干推导)

- 借鉴的越多, 失去的越快。Loom 差异化 = view layer + MIT + local-first + Coordinator, 不是 feature 数量
- 跳过 P1-1 (Code mode) 因为 Loom 的 run 不是 user 写代码场景
- 跳过 P1-6 (Postgres) Slice 2 因为 single-user SQLite 够用
- 跳过 P1-9 (PXI) Slice 2 因为 Loom 数据还不够多

### 长期监控 (项 #15)

- 每月看一眼 DBOS Conductor / pg_durable / MCP 后续 minor / Temporal Replay 2027
- 跟踪演化, 不主动 follow

## Candidate Draft (Non-Authoritative) — 2026-07-25 Graph Engineering Plan v1

> This plan is preserved for review and decomposition evidence only. It is not
> frozen, cannot authorize implementation, and cannot override
> `TECH-PLAN.md`, `docs/CURRENT.md`, or the active WorkItem contract.

**触发**: 用户 2026-07-25 02:00 SGT 选 "选项 B" — 先出 Graph Engineering 计划文档再 freeze 进 spec。

**输出文件**: `.loom-drafts/loom-graph-engineering-plan.md` (21,040 字节, 12 章节)

**内容覆盖**:
- §1 概述 (关键不变量: S1 frozen / 新工作用 S2-W* / Slice 2 spec 自己写)
- §2 完整 DAG (16 tasks × 12 parallel groups, 关键路径 ~5 周)
- §3 plan.json schema (per task: task_id/parallel_group/dependencies/acceptance_criteria/atomic_commit_targets/...)
- §4 plan-execute-verify 闭环 (状态机: pending → in_progress → ready_for_review → done/repair, 最多 3 次 fix)
- §5 State Management (5 文件: meta.json/plan.json/progress.json/verify.json/deliverable.md + Reducer 模式)
- §6 Checkpoint 策略 (4 级: atomic commit / task commit / group commit / slice milestone, 跨项目 overflow 4-hint)
- §7 风险 Mitigation (5 大: overflow / 失败重试 / Verifier 自治 / 命名错位 / Git 污染)
- §8 借鉴优先级时间表 (P0-1~14 全部落具体 task)
- §9 Loom 客户端 3 形态 (SKILL.md v1 / TUI 最小版 / TUI 完整版 / Loom Web)
- §10 实施下一步 (用户 review → freeze → 出 spec → 启动 S2-W1)
- §11 验收标准 (本 plan freeze 后 checklist)
- §12 一句话总结 + 附录 A (文件结构) + 附录 B (跨项目经验索引)

**关键设计**:
- **借鉴 graph engineering 范式**: DAG + parallelGroup + 状态机 + Reducer + checkpoint
- **5 文件 state 模式**: 跟 Slice 1 evidence 风格对齐, 复用 `deliverable.md` + `VERDICT` 末行
- **跨项目经验全部用上**: 硬规则 §1-6 + overflow 4-hint + Verifier 自治 + 命名错位 mitigation
- **关键路径估时**: G1 (1-2d) → G2 (4-5d) → G3 (3-4d) → G4 (4-5d) → G5 (2-3d) → G6 (3-4d) → G7 (2-3d) = **~25 天 ≈ 5 周**

**下一步 (等用户 review)**:
1. 用户 review 整篇, 重点 §2 DAG + §2.2 acceptance + §4.1 状态机 + §6 checkpoint + §7 风险
2. 反馈 OK / 部分改 / 大改
3. mavis 改 v2 (如需)
4. lune freeze 标 "FROZEN" + 日期
5. mavis 出 `phase1-slice2.goalspec.yaml` (本 plan 是骨架, spec 是 contract)
6. 启动 S2-W1 独立 session 跑 Bridge v1.1 envelope

## Candidate Draft (Non-Authoritative) — 2026-07-25 Loom Handoff Prompt v1

> This handoff is preserved as Candidate context only. Its internal readiness
> labels are not Reviewer acceptance and cannot bypass current repository,
> contract, or human gates.

**触发**: 用户 2026-07-25 02:34 SGT 准备跨项目 / 跨 session 传承上下文, 让新项目能立刻接上 Loom 工作。

**输出文件**: `.loom-drafts/loom-handoff-prompt.md` (17,827 字节, 14 章节 + 3 附录)

**内容覆盖** (跨项目传承完整图景):
- §0 项目身份 (Loom / workspace / owner / license / branch)
- §1 一句话定位 Loom
- §2 关键不变量 (S1 frozen / 4 不抢红线 / 5 件现在做)
- §3 借鉴优先级 (P0-1~14 + P1 + P2 全部落具体 task)
- §4 调研文档归属 (7 个文档, 何时读, 角色)
- §5 跨项目硬规则 6 条 (违反会被 plan 引擎 auto-reject)
- §6 跨项目 SKILL 5 大风险 mitigation
- §7 Loom 客户端 3 形态 (用户 2026-07-24 决策)
- §8 当前状态 (2026-07-25 02:34 SGT)
- §9 下一步 3 选 1
- §10 不要做 (Loom 严格不抢, 11 条)
- §11 项目依赖 (协议 / 运行时 / 客户端 / 可选集成)
- §12 Loom 内部模块依赖 (3 层无环)
- §13 联系方式 / review 路径
- §14 一句话给新 session 的话
- 附录 A 关键文件路径速查
- 附录 B 跨项目经验索引
- 附录 C 启动新 session 的 prompt 模板 (复制粘贴用)

**关键设计**:
- **跨项目传承**: 新 session 读完 14 章节就能立刻接上工作, 不需要重新调研
- **硬规则 + 跨项目 SKILL + 风险 mitigation 全部浓缩**: 避免 "应该用 S1-W1 还是 S2-W1" 这种翻车
- **附录 C 启动 prompt 模板**: 完整可复制, 一行命令启动新 session
- **Historical self-label `VERDICT: READY`**: this was the draft's own status;
  it still requires governed review and must not be used directly as execution
  authority

**下一步**:
1. 用户复制附录 C prompt 模板到新 session
2. 新 session 读完 handoff prompt 整篇
3. 立刻接上 Loom Slice 2 工作, 不需要重新调研

## Candidate Draft (Non-Authoritative) — 2026-07-27 Loom Scheduler Supplement v1.1

> This draft is preserved as Candidate context only. Its internal readiness
> label is not Reviewer acceptance and cannot bypass current repository,
> contract, or human gates.

**触发**: 用户 2026-07-27 指令 "可以保留下文档, 然后看下社区怎么做 Agent 调度的" — 在 v1 Graph Engineering Plan 基础上, 用 Linux 进程模型补出 runtime 层。

**输出文件**: `.loom-drafts/loom-scheduler-supplement-2026-07-27.md` (21,678 字节, 578 行)

**核心方法** (跨项目可复用):
- **Linux 进程模型做 agent runtime**: Agent=process / cgroup=资源限制 / CFS+SCHED_FIFO/RR=scheduler / swap→compressor (LLM 没真 swap) / OOM killer=token budget
- **PID 1=main agent, 不可 OOM-killed** (Linux init 特殊)
- **Bounce-back RT cap = 30%** of system budget (仿 `sched_rt_runtime_us`)
- **OOM graceful**: atomic commit + partial deliverable + ZOMBIE + exit 137 (128+SIGKILL)
- **Contract 5-element**: goal / scope_in / scope_out / context_slice / done_when / on_fail
- **3 invariants on handoff**: explicit whitelist slice / verifier_feedback raw forwarded / failure_context 携带 commit+assertion+log
- **Context compressor**: local `qwen2.5-coder-1.5b-instruct-q4_k_m` 跑压缩 (cheap) ; pressure thresholds 50%/80%/95%
- **Quotas via `.loom-config/quotas.yaml`** (policy-driven, 不硬编码)

**5 章 + 5 开放问题**:
- §1 process model (task_struct + fork/exec/wait/kill)
- §2 cgroup (token_limit / token_high / token_weight / wall_budget / io / pids)
- §3 scheduler (CFS vruntime + 5 sched_policy + nice)
- §4 contract (5-element) + state machine (reducer)
- §5 compressor (3 modes: summarize / regenerate / offload)
- §10 5 open questions (PID 1 不可 OOM / policy DSL / bounce-back cap / worktree per parallel / blocked first-class state)

**下一步**:
1. 用户 review v1.1 5 开放问题
2. 通过后 freeze 进 Loom Slice 2 spec
3. v1.2 = v1.1 接受部分 + community validation 整合 (multica/omnigent 调研)

## Candidate Draft (Non-Authoritative) — 2026-07-27 Loom Community Survey: Multica + Omnigent

> This survey is preserved as Candidate context only. Its internal
> readiness label is not Reviewer acceptance and cannot bypass current
> repository, contract, or human gates.

**触发**: 用户 2026-07-27 21:28 SGT 追问 "看下 multica 和 omnigent 的实现" — 在 v1.1 scheduler 基础上, 对最贴近 Loom 定位的两个社区项目做 implementation 深读。

**输出文件**: `.loom-drafts/loom-community-survey-multica-omnigent-2026-07-27.md` (24,192 字节, 507 行, 9 章 + VERDICT)

**3 个非显见发现** (开头就点出来):
1. **Multica 记忆系统 0 embedding**: 6 张纯关系表 + JSONB snapshot + 显式 `agent_skill` join。15,400+ stars 证明 "vector search 不是必要的"
2. **Omnigent 是 Loom 直接架构竞品**: 自我定位 "Kubernetes for AI agents" — 跟 Loom 任务书几乎重合。2026-06 开源, ⭐ 4,200+, Apache 2.0
3. **两家都没用 process model**: Multica 用 communication graph, Omnigent 用 wrapped CLI session。Loom 的 `task_struct + cgroup + CFS + OOM killer` 是真正原创

**Multica 6 张表 (community-validated 模式, 无 embedding)**:
- `workspace` + `workspace.context` (全局 prompt)
- `issue` (task unit, JSONB context_refs + acceptance_criteria)
- `agent_task_queue.context` (JSONB — 一次性 frozen snapshot)
- `skill + skill_file + agent_skill` (显式 join, 无 cosine)
- `comment + activity_log` (work memory + audit)

**Omnigent 3-type policy DSL** (推荐直接采用):
- `tool-whitelist` (allow/deny)
- `cost-limit` (max_per_session USD)
- `approval-gate` (pattern matching: `rm -rf`, `DROP TABLE` 等)

**Omnigent 旗舰 example**:
- **Polly (🐙)**: 多 agent 编排器, 每个子 agent **独立 Git Worktree**, 跨供应商 cross-review, 跟 v1.1 §4 parallel group 直接对应
- **Debby (🟠🔵)**: 双 agent 辩论师, 适合做 evaluation 不适合 production

**5 个直接 fold 进 v1.2 的具体建议** (§4):
1. Skill lookup 用显式 join, vector 退到 opt-in
2. Parallel group 成员**自动 worktree** (Polly 模式, 2 个 `loom worktree` 子命令)
3. Policy DSL 采用 Omnigent 3-type trichotomy
4. Snapshot frozen at dispatch (Multica JSONB 模式), child **禁止** 自由 DB 查询
5. `blocked` 提升为 first-class reducer state (区别于 `failed`)

**6 个 v1.1 已领先的领域** (§5, 不能丢):
1. Process tree semantics (fork/wait4/SIGCHLD/OOM)
2. 确定性 exit code 137 (Linux convention)
3. Bounce-back RT cap 30%
4. 3-retry-then-escalate + verifier feedback 保留
5. PID 1 不可 OOM-killed
6. `token_high` (throttle) vs `token_limit` (hard cap) 二分

**Cross-check 状态**: 4 独立 CSDN 源对 Multica schema 交叉验证一致; Omnigent 定位 cross-checked by CSDN + GitHub topic page。**无 primary source code** — 两个项目有公开 GitHub 仓库 (multica-ai/multica, omnigent-ai/omnigent) 但本次 search 没拿到 repo 级别内容。所有实现细节视为 **secondary**, v1.2 阶段需 code-confirmation。

**v1.2 路径** (给用户):
- v1.1 freeze 等用户 review 5 开放问题
- 接受后 v1.2 = v1.1 接受部分 + §4 5 个 amendments + §6 5 个 open question 用户决策
- 推荐采用率: PID 1 OOM (adopt Linux) / Policy DSL (adopt Omnigent 3-type) / Bounce-back cap (keep 30%) / Worktree (yes, 2 subcommands) / Blocked state (yes, 替代 silent 30 min failure mode)

**下一步**:
1. 用户 review v1.1 + 这份 survey
2. 决定 v1.1 5 开放问题
3. 决定 v1.2 5 amendments 是否全部采纳
4. 决定 v1.2 是冻结进 spec 还是继续 draft

## User Decision (Authoritative, 2026-07-27) — v1.1 5 开放问题全部接受

> This decision is **authoritative** (explicit user phrase), not a draft.
> It closes every open question raised in the v1.1 supplement §10 and
> the v1.2 community survey §6.

**触发**: 用户 2026-07-27 21:53 SGT 回复 "1可行, 2可行, 3可行, 4可行, 5可行" — 对 v1.2 推荐 5 项全部确认接受。

**决策表** (5/5 accept):

| # | 开放问题 | 用户决策 | 影响文档 / 代码位置 |
|---|---|---|---|
| 1 | PID 1 不可 OOM-killed | **可行** — adopt Linux convention verbatim | supplement v1.1 §1.2 已隐含, v1.2 显式化 |
| 2 | Policy DSL 格式 | **可行** — adopt Omnigent 3-type trichotomy (tool-whitelist / cost-limit / approval-gate) | supplement v1.1 §10 关闭, 新增 §6 Policy DSL section |
| 3 | Bounce-back RT cap 30% | **可行** — 保留 30% 作 default, 通过 `quotas.yaml` 暴露 | supplement v1.1 §2 + §3, no change to default value |
| 4 | Worktree per parallel agent | **可行** — 加 2 个 `loom worktree` 子命令 (create / merge) | supplement v1.1 §4 parallel group 显式 worktree 语义 |
| 5 | `blocked` first-class reducer state | **可行** — 替代 silent 30 min failure mode | supplement v1.1 §4 状态机加 `blocked` 状态 (与 `repair` 并列) |

**结果**:
- v1.1 5 开放问题 → 全部关闭
- v1.2 启动条件 = 满足 (用户已授权 5 个 amendments)
- v1.2 = v1.1 frozen 部分 + §4 5 amendments 实施 + 用户决策的 5 项显式化

**没变的事项** (v1.1 frozen 部分, 5/5 接受后仍然保持):
- Linux process model 整体设计 (Agent=process / cgroup / CFS+SCHED_FIFO/RR / OOM killer)
- Contract 5-element (goal / scope_in / scope_out / context_slice / done_when / on_fail)
- 3 handoff invariants (whitelist slice / verifier_feedback raw / failure_context)
- 3 compressor modes (offload_to_file / summarize_in_place / drop_with_pointer)
- Exit code 137 (Linux convention)
- 3-retry-then-escalate with verifier feedback preservation

**下一步** (待用户确认是否启动 v1.2 合并文档):
1. **Option A**: 立即写 v1.2 = v1.1 frozen 接受部分 + 5 amendments 完整 spec (估计 30-40K 字节, 12-15 章节)
2. **Option B**: 分步走 — 先 freeze v1.1 进 spec (单独动作), 然后开 v1.2 任务 (新 todo list, 多 worker 协作)
3. **Option C**: 用户先看 v1.1 frozen + survey 整体, 再决定

**v1.2 内容预期** (无论何时启动):
- §0-1 v1.1 frozen 摘录 (process model / cgroup / scheduler / contract / compressor)
- §2 Policy DSL (3-type, Omnigent trichotomy) — 新增
- §3 Worktree per parallel agent (2 个 `loom worktree` 子命令 spec) — 新增
- §4 `blocked` reducer state (parent re-parenting 路径) — 新增
- §5 Compressor integration with snapshot (3 modes 跟 JSONB snapshot 联动) — 增强
- §6 接受 v1.1 5 开放问题的设计理由 (每个有 1 段 "为什么" 跟社区引用)
- §7 Open questions for v1.3 (next tier)
- VERDICT: READY_FOR_FREEZE  (跟 v1.1 的 READY_FOR_REVIEW 区别)

---

## Community Audit 3 (2026-07-27 22:14 SGT) — Anthropic Long-Running Coding Harness

**触发**: 用户 2026-07-27 21:53 SGT "可行，再做几次审计吧，调研社区内容"，3 个新社区审计之一 (Anthropic)。

**调研内容** (.loom-drafts/loom-community-survey-anthropic-harness-2026-07-27.md, 17,583 bytes / 336 lines):
- 9 章节 + 14,5XX 字节, 4 primary source + 3 secondary source
- 核心创新: GAN-inspired Generator + Evaluator (双 agent, 不同 context)
- 3 件套环境: Feature List / init.sh / claude-progress.txt
- 4 维评分: orig(0.4) + qual(0.3) + craft(0.2) + func(0.1)
- 6 条非显而易见工程模式: context anxiety / self-eval distortion / premature done / context reset / 2-phase init / 4h wall clock

**v1.2 amendment 候选 (5 条)**:
- #6: Feature List per task (源自 §3.1 + §3.3)
- #7: `loom task init-sh` + Init phase (源自 §3.5)
- #8: Verifier 不读 reasoning (源自 §3.2)
- #9: `blocked` + `task.progress.md` resume (源自 §3.4)
- #10: `task.wall_clock_budget` (default 4h) (源自 §3.6)

**决策**: 推荐 B (只接受 #6 + #10, Feature List + wall clock), 工作量 +1 天, ROI 最高, 风险低。#7-#9 nice-to-have 放 v1.3。

---

## Community Audit 4 (2026-07-27 22:14 SGT) — Swarms (Enterprise-Grade Multi-Agent Framework)

**触发**: 用户 2026-07-27 21:53 SGT "再做几次审计"，3 个新社区审计之二 (Swarms)。

**调研内容** (.loom-drafts/loom-community-survey-swarms-2026-07-27.md, 17,469 bytes / 441 lines):
- 9 章节 + 17,XXX 字节, 4 primary source + 3 secondary source
- 核心: **8 种 orchestration topology** (Hierarchical / Parallel / Sequential / Graph / Dynamic Rearrangement / MixtureOfAgents / Forest / Router)
- 6 工程组件: 异构 model / tool library / memory 选择 / load balancing / pool / horizontal scaling
- 3,261 commits = mature enterprise-grade

**v1.2 amendment 候选 (5 条)**:
- #15: 异构 model per agent (源自 §3.1)
- #16: `loom tools` 预制 registry (源自 §3.3)
- #17: Agent pool + round-robin (源自 §3.5)
- #18: HierarchicalSwarm 显式支持 (源自 §2.2.1)
- #19: Parallel group 加 Reducer (源自 §2.2.2)

**决策**: 推荐 B (只接受 #15 + #19, 异构 model + parallel reducer), 工作量 +1.5 天, parallel group 真正缺。#18 (topology 字段) 可放 v1.3 等需求驱动。#16-#17 风险高, 放 v1.3。

---

## Community Audit 5 (2026-07-27 22:16 SGT) — aevatar.ai (Orleans + Event Sourcing 分布式 Agent 平台)

**触发**: 用户 2026-07-27 21:53 SGT "再做几次审计"，3 个新社区审计之三 (aevatar.ai)。

**调研内容** (.loom-drafts/loom-community-survey-aevatar-2026-07-27.md, 18,040 bytes / 393 lines):
- 9 章节 + 18,XXX 字节, 4 primary source + 3 secondary source
- 核心: **Orleans virtual actor + Event Sourcing** 分布式范式
- 3 repo 分工: framework (C#) / station (TS) / gagents (C#)
- 6 工程组件: concurrency / auto-scaling / grain key 复用 / sandbox / event log / 插件化部署

**v1.2 amendment 候选 (3 条 v1.2 + 2 条 v1.3)**:
- #20: `agent.grain_key` 复用 (源自 §3.3) — v1.2
- #21: `loom task sandbox` Docker 沙箱 (源自 §3.4) — v1.2, 但 Mac 门槛↑
- #22: `task.event_log` JSONL 审计 (源自 §3.5) — v1.2
- #23: 分布式 actor 模型 — v1.3, 1+ 周工作量
- #24: Agent marketplace — v1.3, 1 周工作量

**决策**: 推荐 B (只接受 #20 + #22, grain_key + event_log), 工作量 +1.5 天, 都是纯加项, 不改 v1.1 架构。#21 (sandbox) 涉及 Docker 改 v1.3。#23-#24 远期分布式。

---

## 3 Audits 综合决策表 (2026-07-27 22:16 SGT)

| 来源 | 候选 amendment | 推荐采纳 | 工作量 | 风险 |
|---|---|---|---|---|
| Anthropic #6 | Feature List | ✅ v1.2 | 0.5 天 | 低 |
| Anthropic #10 | wall clock budget | ✅ v1.2 | 0.5 天 | 低 |
| Swarms #15 | 异构 model per agent | ✅ v1.2 | 0.5 天 | 中 |
| Swarms #19 | Parallel Reducer | ✅ v1.2 | 1 天 | 中 |
| Aevatar #20 | grain_key 复用 | ✅ v1.2 | 0.5 天 | 低 |
| Aevatar #22 | event_log 审计 | ✅ v1.2 | 1 天 | 低 |
| Anthropic #7-9 | init-sh / verifier no-reasoning / blocked resume | ⏸ v1.3 | - | - |
| Swarms #16-18 | tools registry / pool / topology 字段 | ⏸ v1.3 | - | - |
| Aevatar #21 | Docker sandbox | ⏸ v1.3 (Mac 门槛) | - | - |
| Aevatar #23-24 | 分布式 / marketplace | ⏸ v1.3+ | - | - |

**v1.2 推荐总计 6 项 amendment**, 工作量合计 **4 天**, 风险均低/中。

**v1.2 启动条件** (待用户决策):
- 之前 5/5 v1.1 开放问题已接受
- 现在 6 项 amendment 推荐已就位
- Option A: 立即合并 v1.2 文档 (4 天, 含实施)
- Option B: 分步 — 先 freeze v1.1, 再启动 v1.2
- Option C: 用户先看 5 份 survey + supplement 整体再决定
- Option D (NEW): 先批准 6 项 amendment, 写 v1.2 合并文档 (但 v1.2 = spec 阶段, 不立即实施代码)

**等用户决策**, 不抢跑。

---

## Community Audit 6 (2026-08-01 12:35 SGT) — Multica (人+Agent 协作平台, 4 人小团队 22k+ stars)

**触发**: 用户 2026-08-01 12:30 SGT "看下 multica 的 mermaid 图"。在 5 audit 之后,用户主动要求看一个具体项目的架构图。Multica 之前只在 memory 中有初步事实,本轮做正式调研。

**位置**: `/Users/lune/multica-analysis/` (新建)
- `src/0{1-4}-*.mmd` (4 张 Mermaid 源)
- `svg/0{1-4}-*.svg` (mmdc 渲染, 35-51KB)
- `png/0{1-4}-*.svg.png` (qlmanage 转换, 165-210KB 健康范围)
- `README.md` (12K 字节技术解析)

**4 张图**:
1. `01-architecture.mmd` — 部署视图 4 层 (User/Server/Storage/Daemon/14 CLI)
2. `02-control-vs-data-plane.mmd` — **关键洞察**:server 协议层不实现"代码执行",代码不出本机
3. `03-five-abstractions.mmd` — Workspace/Squad/Agent/Skill/Runtime + Issue 1:N Task
4. `04-loom-vs-multica.mmd` — 6 个共享问题 + 两套方案对比

**Multica 关键事实** (从 README + .env.example + 源码分析):
- 仓库: `multica-ai/multica`, v0.3.1 (2026-05-15), 最新 v0.2.7 release (2026-07)
- 创始人: 张佳园 (Jiayuan), ex-TikTok, 2023 创业做过 Devv.AI / DevCode 均 pivot
- 14 款 CLI 适配: Claude Code, Codex, CodeBuddy, GitHub Copilot CLI, OpenCode, OpenClaw, Hermes, Pi, Cursor Agent, Kimi, Kiro CLI, Antigravity, Qoder CLI, Trae CLI
- 3 token 分层: Session Cookie (multica_auth) / PAT (mul_*) / Daemon Token (mdt_*)
- 2 层并发: Agent `max_concurrent_tasks=6` + Daemon `MULTICA_DAEMON_MAX_CONCURRENT_TASKS=20`
- 协议: Apache 2.0 + 自定义商业条款 (商业托管需授权)
- 技术栈: Next.js 16 + Go (Chi + sqlc + WS) + PostgreSQL 17 + pgvector
- Daemon 3s 轮询 / 15s 心跳 / 5min dispatched 超时 / 2.5h running 超时
- 1 次重试上限 + session 继承 (manual rerun 不继承)
- **ClawHavoc 事件** (2026-02) 推动 Skill 安全设计: 显式挂载 / 明文标注 / 作用域隔离

**v1.2 amendment 候选 (借鉴, 3 条)**:
- #25: **Issue 1:N Task 拆分** (源自 Multica §3.2) — 替代 Loom 当前 WorkItem 单一实体, 让重试/session 继承/Rerun 更干净
- #26: **3-token 分层** (源自 Multica §3.3) — Loom 拆 UserToken / DaemonToken / RunToken
- #27: **Squad 反自循环规则** (源自 Multica §4.1) — 如果引入 leader routing, 必须先加这条

**v1.2 候选更新 (合并后总计 9 条)**:
| # | 来源 | 内容 | 工作量 | 风险 |
|---|---|---|---|---|
| #1-6 | 之前 5 audit | Feature List / wall clock / 异构 model / Parallel Reducer / grain_key / event_log | 4 天 | 低-中 |
| #25 | Multica | Issue 1:N Task 拆分 | 1.5 天 | 中(需重设计 WorkItem) |
| #26 | Multica | 3-token 分层 | 0.5 天 | 低 |
| #27 | Multica | Squad 反自循环规则 | 0.5 天 | 低(仅 if 引入 routing) |

**v1.2 启动总计 9 条 amendment**, 工作量合计 **6.5 天**, 风险 #25 较高(架构重设计)。

**已知 Multica 限制** (Loom 应避免):
- Cloud 模式 Waitlist, 零运维门槛未达成
- 自托管要求 docker compose + 常驻主机, 个人开发者门槛高
- Autopilot webhook/API 预留未启用
- MCP 11 家里只 1 家真用 (Claude Code) — 多 CLI 适配是表面文章

**决策**: Multica 调研已完成, 3 条借鉴加入 v1.2 候选池。**等用户决定** v1.2 启动时机(同 5/5 决策的"不抢跑"原则)。

---

## v2.0 Spec 撰写 (2026-08-01 17:35 SGT) — Multica 全对齐 + AgentENV 沙箱

**触发**: 用户 2026-08-01 两道决策:
- Q1: **C. 全对齐 (>30 天)** — Loom v2.0 全面对齐 Multica 5 维度 (Skill / Squad / 3rd-party CLI / 3-token / MCP)
- Q2: **A. 写 v2.0 spec 优先, 再实施** — 先出完整架构 spec, 用户 signoff 后再动代码
- AgentEnv 选型: **kvcache-ai/AgentENV** (Kimi K3 同款, MIT, E2B 兼容 API)

**产出**: `.loom-drafts/v2.0-spec.md` (26.5K 字节, 7 章节, READY_FOR_REVIEW)

**v2.0 核心命题** (一句话):
```
v2.0 = Multica 组织层 (Skill + Squad + 3rd-CLI + 3-token + MCP)
     + Loom 运行时层 (Go daemon + Linux 进程 + cgroup + OOM)
     + AgentENV 沙箱层 (Firecracker + overlaybd + <50ms 启停)
     + Multica 没有的本地化强化 (凭证代理 + Sidecar + 显式批准)
```

**5 维度设计要点**:
- **Skill**: 4 源限制 (registry/git/local/inline), ClawHavoc 3 防御 (env_whitelist/scope/mounts)
- **Squad**: leader 协议 + anti-self-loop + 3 工作模式 (one_to_one/one_to_many/fanout)
- **3rd-CLI**: 14 个 CLI 3 档优先级 (P0/P1/P2), 能力矩阵 yaml
- **3-Token**: UserToken / DaemonToken / RunToken (5min 寿命, Task 终态吊销)
- **MCP**: Broker 模式 (不自实现协议), 仅 Claude Code 真正消费

**Sandbox 层 — AgentENV**:
- 集成模型: E2B 兼容 HTTP API + 官方 E2B SDK 调用, **不 fork 它的代码**
- 生命周期映射: Task dispatched → aenv start / Blocked → aenv pause / 终态 → aenv delete
- 回退: Phase 2.A 默认可选, Phase 2.D 强制默认 + `--no-sandbox` opt-out

**数据模型迁移**:
- WorkItem 拆分为 Issue + Task (Multica #25 经验)
- 新增 Skill / Squad / RoutingRule 实体
- v1 WorkItem 表保留 90 天只读
- 迁移脚本 `cmd/loom/migrate/v1_to_v2.go`

**v0.3 不变量保留率**: 14 条中 9 条直接保留, 2 条升级, 2 条拆分重写, 1 条保持 (#8 凭证 → 升级为 3-token)

**实施分 6 阶段 / 25 天** (单人节奏):
| 阶段 | 内容 | 天数 | 风险 |
|---|---|---|---|
| 2.A | Skill 系统 + Squad 抽象 | 5 | 低-中 |
| 2.B | Issue/Task 拆分 | 3 | 中-高 |
| 2.C | 3-Token 认证 | 4 | 中 |
| 2.D | AgentENV 沙箱层 | 5 | 中-高 |
| 2.E | 3rd-Party CLI 适配扩展 | 5 | 低-中 |
| 2.F | MCP Broker + 集成 | 3 | 中 |
| **合计** | | **25** | |

**v1.2 池状态**: 9 条 amendment 候选**全部暂停**,v2.0 走通后再决定是否回归 v1.2 路径 (避免双线并进)

**硬回滚条件** (任一满足 → 立即停 v2.0, 回退 v1.x):
- AgentENV Phase 2.D Day 4 仍无法与 Task 状态机对接
- 3-Token Phase 2.C fuzzing 发现安全 bug
- Issue/Task 拆分导致 v1 数据迁移丢失 > 0.1% WorkItem

**下一步**: 等用户对 `.loom-drafts/v2.0-spec.md` 决策 (signoff / 改 / 砍)。不抢跑。

---

## v2.0 Spec VM 部署补充 (2026-08-01 17:40 SGT) — AgentENV 不支持 macOS 主机

**触发**: 用户问 "AgentENV 能部署到 VM 上么?" + "给一个整体评估提示词"。

**关键发现 — v2.0 spec 隐藏缺口**:
- **AgentENV 部署硬要求**: Ubuntu 24.04 + Linux kernel 6.8+ + **`/dev/kvm` 可访问** (Firecracker 必需)
- **macOS 主机直接跑 = 不支持** (Apple Silicon/Intel Mac 都没有 `/dev/kvm`)
- **macOS 内的 Linux VM (UTM/Parallels/QEMU) 也不支持** (虚拟化层不暴露 KVM nested)
- **Linux x86_64/arm64 VM 完全支持** (EC2/GCE/Azure/Hetzner/Graviton)

**Loom 影响** (Customer #0 = lune, Apple Silicon Mac):
- v2.0 spec 需补 **§3.7 部署拓扑与平台限制** 章节
- 推荐模式: **A. macOS daemon + Linux VM sidecar** (OrbStack/Colima/Lima/Tailscale SSH 调远程 Linux VM)
- Phase 2.D Day 1 需先验证 macOS-to-Linux-VM 联通性, 不能假定直接 local Firecracker

**评估提示词产出**: `.loom-drafts/v2.0-evaluation-prompt.md` (7.2K 字节, READY_FOR_REVIEW)
- 15 个核心问题 (P0 战略 3 / P1 架构 6 / P2 实施 4 / P3 上下文 2)
- 严格输出格式 (8 章节 + VERDICT 行)
- 6 条硬规则 (必须给证据 / 独立思考 / 不客气 / 承认不确定 / 不写代码 / 长度 8-15K)
- 5 条必须查的事实 (multica-ai/multica 真实状态 / kvcache-ai/AgentENV issue 列表 / Loom 节奏 / 2026 Q2-Q3 同类项目)

**v2.0 spec 待修订项** (评估时同步关注):
1. 补 §3.7 部署拓扑 (3 候选 + 推荐 A)
2. 补测试章节 (各 Phase 覆盖率 + fuzzing 范围)
3. 补可观测性章节 (AgentENV ublk metrics 聚合 + 三视图衔接)
4. 补性能预算章节 (5min dispatched / 2.5h running / 6 并发 沿用 Multica 还是自定?)

---

## CubeSandbox 对比评估 (2026-08-01 17:50 SGT) — 替代沙箱候选

**触发**: 用户问 "那使用 cubesandbox 呢?你看看 能不能 Mac 部署"。

**确认**: CubeSandbox = **TencentCloud/CubeSandbox** (腾讯云开源), `https://github.com/TencentCloud/CubeSandbox` + `https://cubesandbox.com/`

**CubeSandbox 关键事实 (新增)**:
- **GitHub**: TencentCloud/CubeSandbox, **10.8k stars** / 994 forks (AgentENV 2.7k 的 4x)
- **License**: **Apache 2.0** (vs AgentENV MIT)
- **CNCF Landscape**: 是 (云原生计算基金会收录)
- **底层**: RustVMM + KVM (跟 AgentENV 一样), 独特是 **PVM 半虚拟化** (`kvm_pvm` 内核模块)
- **E2B 兼容**: ✅ 完整 (Roadmap 还有 gap 要补)
- **腾讯云生产验证**: 是 (OpenCloudOS 9 内核直接进 yum 仓库)
- **冷启动**: ~60ms (裸金属) / **76ms (云 CVM PVM 模式)**
- **架构组件 8 个**: CubeAPI / CubeMaster / CubeProxy / Cubelet / CubeVS (eBPF) / CubeEgress (L7 网关) / CubeHypervisor / CubeShim
- **v0.5 起支持 ARM64**

**macOS 部署答案 — 跟 AgentENV 一样, 都跑不了**:
| 平台 | AgentENV | CubeSandbox |
|---|---|---|
| macOS 主机直接 | ❌ | ❌ |
| macOS 内的 Linux VM (UTM/Parallels/QEMU) | ❌ | ❌ |
| Linux x86_64 VM (EC2/GCE/Azure) | ✅ | ✅ |
| Linux arm64 VM (Graviton) | ✅ | ✅ |
| **普通云 CVM (无 /dev/kvm, 50-100元/月)** | ❌ | ✅ **PVM 模式可跑** |
| K8s 集群 | ✅ | ✅ |

**关键差异 — CubeSandbox 多了 PVM 模式**:
- 腾讯云 PVM 是基于页表的半虚拟化, 装 `kvm_pvm` 内核模块后, 可以在 **没有 /dev/kvm 的云 CVM 上跑** (普通云 CVM 屏蔽了硬件虚拟化)
- 性能损耗小 (60-80ms 冷启动, 几乎和真 KVM 一样)
- macOS 用户受益: 不用买裸金属 / 嵌套 virt 云, 50-100元/月的普通 CVM 就能跑

**5 维对比 (对 v2.0 spec §3 选型的影响)**:
| 维度 | AgentENV (kvcache-ai) | CubeSandbox (Tencent) | Loom v2.0 spec 影响 |
|---|---|---|---|
| **macOS 部署** | ❌ 需裸金属 / nested virt 云 | ❌ 但云 CVM PVM 50元/月 | 选 CubeSandbox 部署成本低 |
| **网络隔离** | 文档提及, 无 eBPF | ✅ **CubeVS eBPF 内核级 + CubeEgress L7 网关** | spec §3 可借 CubeEgress 设计 |
| **凭证安全** | 无专门方案 | ✅ **CubeEgress 凭证保险库** (凭证不进入沙箱) | spec §2.4.3 "3-Token 凭证流" 可直接借 |
| **生态** | 2.7k stars, Kimi K3 内部用 | ✅ **10.8k stars + CNCF Landscape + 腾讯云** | CubeSandbox 集成风险更低 |
| **License** | MIT (宽松) | Apache 2.0 (宽松, 但要求 NOTICE) | 都 OK |
| **性能** | <50ms 启动 | 60-80ms 启动 | 差异 < 30ms, 用户感知不到 |

**结论 (我的顾问意见)**:
- **短期推荐 CubeSandbox**: 部署成本低 (PVM 云 CVM 50元/月 vs 裸金属), 网络 + 凭证安全原生解决 v2.0 spec 的两个关键问题, CNCF Landscape 长期保障
- **长期保留 AgentENV 作为 escape hatch**: 如果用户有 Kimi K3 训练场景, 或自己买裸金属, AgentENV 的 <50ms 也 OK
- **v2.0 spec 修订建议**: §3 改为 **"沙箱层 — 抽象 + 双实现"**, 走 E2B 兼容层, 让 AgentENV / CubeSandbox 都是可换的 backend (类似 Loom v0.3 RuntimeAdapter 模式), 用 `--sandbox-backend` flag 切换

**v2.0 spec 修订草稿 (§3 改为)**:
```
§3 Sandbox 层 — kvcache-ai/AgentENV + TencentCloud/CubeSandbox 双后端

3.1 抽象层 (E2B Compatible Client Interface)
    - 定义 `SandboxClient` Go interface (start/pause/resume/exec/delete/timeout)
    - 所有 v2.0 代码只跟 interface 打交道
3.2 Backend A: kvcache-ai/AgentENV (Kimi K3 同款, MIT, Firecracker)
    优势: <50ms 启动, OCI + overlaybd, snapshot 极致
    限制: 需真实 KVM (裸金属 / nested virt 云)
3.3 Backend B: TencentCloud/CubeSandbox (CNCF Landscape, Apache 2.0, PVM)
    优势: 云 CVM 50元/月可跑, eBPF 网络隔离, 凭证保险库
    限制: 启动 60-80ms, RoadMap 依赖
3.4 选择规则
    - 默认: CubeSandbox (云 CVM PVM 模式, 部署成本低)
    - 高级: AgentENV (用户指定 --sandbox-backend=agentenv)
    - 回退: --no-sandbox (走 v1.x 进程隔离)
```

**评估提示词 (`.loom-drafts/v2.0-evaluation-prompt.md`) 待追加 2 个问题**:
- P0-2a: 沙箱选 CubeSandbox (10.8k stars + CNCF + PVM 云 CVM) 而非 AgentENV 是否合理?
- P0-2b: CubeEgress 凭证保险库 vs Loom 3-Token 设计, 是否冲突 / 重叠 / 互补?

---

## 完整 AI Agent 沙箱 Backend 调研 (2026-08-01 17:55 SGT) — 14 个候选

**触发**: 用户问 "backend 调研下还有哪些" (在 CubeSandbox 之后)

**14 个候选分类**:

### 8 个自托管/开源 Sandbox
1. **TencentCloud/CubeSandbox** (10.8k stars, Apache 2.0, CNCF, Firecracker+KVM+PVM, <60ms, E2B 兼容, **CubeEgress 凭证保险库**)
2. **daytonaio/daytona** (12k+ stars, AGPL-3.0, eBPF hook 不用 namespace, 87ms, MCP+5 语言 SDK)
3. **e2b-dev/e2b** (~5k+, Apache 2.0 自托管, Firecracker, ~150ms, 闭源为主, BYOC)
4. **kvcache-ai/AgentENV** (2.7k, MIT, Firecracker+OCI+overlaybd+ublk, **<50ms**, E2B 兼容, Kimi K3 验证)
5. **alibaba/OpenSandbox** (2.4k+, Apache 2.0, Docker/K8s + execd Go, **3.5s/1000 批量**, E2B 兼容)
6. **agentscope-ai/agentscope-runtime** (8.2k, Apache 2.0, K8s+Docker+FC, 通义实验室, 长期记忆, Alias-Agent)
7. **kubernetes-sigs/agent-sandbox** (710 commits, Apache 2.0, **K8s 官方 SIG Apps**, v0.1.0 alpha, Sandbox CRD + WarmPool, gVisor/Kata)
8. **agent-sandbox/agent-sandbox** (国产, ~1k, K8s+Docker, MCP+REST API, 轻量化, 不用 CRD)

### 4 个云服务 (managed)
9. **阿里云 ACS Agent Sandbox** (MicroVM, 1.5w instances/min, 70% 成本降, Kimi 客户, E2B+K8s 兼容)
10. **AWS Bedrock AgentCore** (Firecracker microVM, 8h session, 框架无关, 算力费无 session 费)
11. **Google Vertex AI Agent Builder** (Apigee + Agent Registry, 自有协议)
12. **Azure AI Foundry** (AutoGen + Semantic Kernel, 自有协议)

### 2 个早期/特殊
13. **withastro/flue** (TypeScript, Cloudflare Workers, 不是 AI Agent 沙箱)
14. **echoVic/blade-agent-runtime** (BAR, Go, **Git worktree 隔离非 microVM**, 49 commits, MIT, 轻量级 Coding Agent 包装层)

**关键发现 — macOS 部署**:
- **所有 microVM backend (Firecracker/KVM) 都不行** — macOS 无 /dev/kvm, VM 嵌套虚拟化不通
- **PVM (CubeSandbox) 是云 CVM 的唯一解** — macOS 内 Linux VM 仍不行
- **Loom 唯一现实路径**: macOS daemon + 远程 Linux VM (必须)

**关键发现 — 凭证 + 网络隔离**:
- **CubeSandbox 顶级**: CubeEgress 凭证保险库 + eBPF + L7 网关
- **K8s SIG 中等**: K8s Secrets + K8s NetPol (需要自己配)
- **AgentENV 弱**: 无专门凭证方案, 文档提及网络隔离
- **E2B 基础版**: 无原生隔离

**关键发现 — License 风险**:
- **Daytona AGPL-3.0** ⚠️ — 商业部署传染风险, 不推荐主选
- 其他主流: MIT / Apache 2.0 (License 友好)

**选型建议 (3 选 1, 按 Loom 适配度排序)**:

🥇 **主选: CubeSandbox** (短期 + 长期)
- 10.8k stars + CNCF + 腾讯云生产 = 集成风险最低
- PVM 模式 = macOS 用户 50元/月跑
- CubeEgress 凭证保险库 = 直接对应 Loom §2.4 凭证流
- 8 组件架构 = 网络隔离企业级
- Apache 2.0 + E2B 兼容 = License 清 + 切换零成本

🥈 **备选 1: AgentENV** (escape hatch)
- <50ms 启动 = 最快
- Kimi K3 大规模 RL 训练验证
- MIT License 最自由
- **何时用**: 用户买裸金属 / 跑训练场景

🥉 **备选 2: E2B 自托管** (商业备用)
- 原生 E2B = Loom 抽象层最干净
- Perplexity/Manus 验证 = 商业稳定
- **何时用**: 用户已有 E2B 部署

**不推荐**:
- **Daytona**: AGPL-3.0 商业不友好
- **K8s SIG agent-sandbox**: v0.1.0 alpha, API 还在破
- **OpenSandbox / AgentScope**: 阿里生态, 国际社区偏弱
- **云服务 (ACS / AgentCore)**: vendor lock-in, 不符合 Loom 本地优先

**v2.0 spec §3 修订 (升级版)**:
```go
type SandboxClient interface {
    Start/Pause/Resume/Exec/Delete/Timeout
}
// 三 backend (都 E2B 协议, 切换零成本)
type CubeSandboxClient struct { ... }   // 默认
type AgentENVClient struct { ... }      // --sandbox-backend=agentenv
type E2BClient struct { ... }           // --sandbox-backend=e2b
```

**跨项目方法论 lesson (完整 backend 调研)**:
1. **14 个候选都不破 macOS 限制** — 这是平台架构的硬约束, 任何"AI agent 沙箱"都跑不了 macOS 主机
2. **E2B 协议是事实标准** — 6/14 候选直接兼容, 应该用 E2B 抽象层而不是 vendor lock-in
3. **CNCF Landscape > star 数** — 长期维护保障信号
4. **凭证 + eBPF 网络隔离是"标配"** — AgentENV 这种"快但裸"的方案, 不适合企业部署
5. **License 传染性 (AGPL)** 应该在第一轮就排除 — Daytona 性能再好, AGPL 也让商业用户头疼
6. **PVM 半虚拟化是云 CVM 跑沙箱的关键** — 5/14 候选支持, 但 macOS 不破
7. **国产 K8s SIG 项目 (agent-sandbox) vs 国际 (kubernetes-sigs/agent-sandbox) 是两个** — 名字撞, 内容完全不一样

---

## v2.0 沙箱后端决策 (2026-08-01 17:58 SGT) — 单一 CubeSandbox, 不要兼容全套

**触发**: 用户问 "这么多后端都要兼容么?" + "用 cubeSandbox 先跑起来"

**决策**:
- v2.0 spec §3 **改为单一 CubeSandbox 后端** (YAGNI — 双/三 backend 是过度设计)
- 三个候选 (AgentENV / E2B) 留作 v2.1+ escape hatch, 不写抽象层
- v2.0 spec §3.5 选型理由写清楚 (10.8k stars + CNCF + PVM + 凭证 + Apache 2.0)
- 部署走 5 步 SOP, 三路径可选 (macOS 本地 OrbStack / 腾讯云 CVM PVM / 已有 Linux 服务器)

**CubeSandbox 仓库验证** (2026-08-01 完成):
- 本地 clone: `~/Documents/Codex/2026-06-18/cubesandbox/`
- LICENSE: **Apache 2.0** ✅ (商业友好)
- 最近 commit: 2026-08-01 (今天, 活跃) ✅
- 架构 8 组件: CubeAPI / CubeEgress / CubeMaster / CubeProxy / Cubelet / CubeShim / cube-lifecycle-manager / cubelog
- E2B 兼容 SDK: `sdk/python/cubesandbox/` (纯 E2B 协议, 零代码改动)
- 例子: `examples/code-sandbox-quickstart/` 14 个 Python 例子 (`from e2b_code_interpreter import Sandbox` 一行接入)
- 一键部署: `deploy/one-click/install.sh` (5 条命令, 3 分钟)

**SOP 产出**: `.loom-drafts/cubesandbox-deploy-sop.md` (8.3K 字节, 7 章节, READY_FOR_REVIEW)
- 第 0 章: 部署前置 (macOS 主机直接 ❌, 必须 Linux VM)
- 第 1 章: 3 路径对比表
- 第 2 章: 路径 A 完整步骤 (macOS 本地调试)
- 第 3 章: 路径 B 完整步骤 (生产, PVM 模式 50元/月)
- 第 4 章: 路径 C 完整步骤 (一键普通 KVM)
- 第 5 章: Loom 集成最小设计 (`internal/sandbox/cubesandbox.go`)
- 第 6 章: 验证清单 (8 项必跑)
- 第 7 章: 下一步
- 第 8 章: 风险与回滚

**v2.0 spec §3 改写要点** (用户 signoff 后执行):
```markdown
§3 Sandbox 层 — 单一 CubeSandbox 后端
3.1 抽象层 (E2B Compatible Go interface)
3.2 Backend: TencentCloud/CubeSandbox (唯一)
3.3 部署路径 (3 选 1, 见 SOP)
3.4 凭证流 (借鉴 CubeEgress 凭证保险库)
3.5 选型理由
```

**等用户决策**: 跑 A 路径 (macOS 本地调试) 还是 B 路径 (腾讯云 CVM 生产) 还是只写代码 wrapper 不真部署?

---

## CubeSandbox 一键脚本交付 (2026-08-01 18:00 SGT) — 路径 C 已有 Linux 服务器

**触发**: 用户选 "C. 已有 Linux 服务器" (5 分钟, 一键普通 KVM)

**交付物**: `.loom-drafts/cubesandbox-server-install.sh` (4.5K 字节, bash 语法检查通过)
- 8 步流程: 环境检查 → Docker 检查 → 工具检查 → 一键 install → 进程验证 → 健康检查 → 打印 E2B endpoint → 创建代码执行模板 → WebUI 提示
- 容错: 缺 /dev/kvm 报错退出; 缺 Docker 自动装 (dnf/apt 都支持); 模板创建失败不影响主流程
- 输出: 自动打印 SERVER_IP, 给 Loom daemon 用的 4 行环境变量

**用户执行 3 条命令**:
```bash
# 1. macOS 端 scp 脚本
scp /Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/agent-platform/.loom-drafts/cubesandbox-server-install.sh <user>@<server>:/tmp/install.sh

# 2. SSH 上服务器, 跑脚本 (5-10 分钟)
ssh <user>@<server> 'sudo bash /tmp/install.sh'

# 3. macOS 端验证 E2B SDK (替换 <SERVER_IP> 和 <TPL_ID>)
export E2B_API_URL=http://<SERVER_IP>:3000
export E2B_API_KEY=e2b_0000000000000000000000000000000000000000
pip install e2b-code-interpreter
python -c "from e2b_code_interpreter import Sandbox; sb = Sandbox.create(template='<TPL_ID>'); print(sb.run_code('1+1').text)"
```

**等用户反馈**: 跑通后告诉我 SERVER_IP 和 TPL_ID, 我写 `internal/sandbox/cubesandbox.go` (Loom daemon 的 CubeSandbox wrapper) + 改 v2.0 spec §3。

---

## GitHub Agent Skills 调研 (2026-08-01 18:25 SGT) — 腾讯云管理视角

**触发**: 用户 2026-08-01 18:10 SGT — "腾讯云我有服务器，你看看github上有什么skills么。我想把云服务托管到Agent上"

**交付物**: `.loom-drafts/github-agent-skills-research-2026-08-01.md` (13.5K 字节, 6 章节, READY_FOR_REVIEW)
- §1 Agent Skills 生态全景 (3 官方 / 5 Awesome / 9 高质量 / 5 中文 / SKILL.md 标准)
- §2 腾讯云官方布局 (CloudBase 39 MCP + 22 Skills / CodeBuddy IDE / 大模型知识引擎 / @cloudbase/mcp SDK / 2026 大会战略 / AI-Infra-Guard)
- §3 4 条路径对比 (A CloudBase / B SKILL.md / C MCP server / D Terraform)
- §4 我的建议 (短期 A+C 组合, 中期 v2.0 §3.6, 长期 v2.1+)
- §5 5 条跨项目方法论 lesson
- §6 Next Steps (4 个待用户决策问题 + 4 个待写产物)

**关键发现**:
- **Agent Skills 已成 2026 工业标准** — anthropics/skills (17k+) / superpowers (225k+) / awesome-claude-skills (56.8k+) / VoltAgent (19.2k+)
- **腾讯云官方已经做完 Serverless 场景** — CloudBase 平台 39 MCP + 22 Skills + SkillHub 7.8万技能, `clawhub.ai/binggg/cloudbase` 一键安装
- **CVM/IaaS 还是空, 留给 MCP server 填** — 推荐 `internal/mcp/tencent-cloud/main.go` (mcp-go + tccli wrapper, 5-10 个核心命令)
- **SKILL.md vs MCP 边界**:
  - **SKILL.md** = 流程化 SOP (怎么思考) → 用于"安全检查/成本优化/合规审计"
  - **MCP server** = 工具集 (能做什么) → 用于"云资源 CRUD"
  - 云管理主要是"操作工具" — **走 MCP**
- **腾讯 AI-Infra-Guard (3.6k stars) 提供 Skills Scan** — 任何第三方 skill 必须先过这个扫描, 解决"恶意 skill" 顾虑

**建议方案** (待用户 signoff):
- **短期 (本周)**: 写 `tencent-cloud-mcp` server (C 路径), 5-10 个 tccli 命令 wrapper; 同时走 CloudBase (A 路径) 覆盖 Serverless
- **中期 (v2.0 spec)**: 加 §3.6 Cloud Provider Adapter, 借鉴 CubeEgress 凭证保险库 + Loom 3-Token 设计
- **长期 (v2.1+)**: 加 v2.0 评估提示词 P0-2c 问题, 评估"MCP Broker vs 独立 Cloud Adapter"

**等用户决策 4 个问题**:
1. **场景**: CVM (云服务器) / CloudBase (Serverless) / 都管?
2. **走现成还是自建**: CloudBase 一键装 (A) / 自写 tencent-cloud-mcp (C) / 都走?
3. **凭证**: tccli 凭证 → 环境变量 / Keychain / CubeEgress 凭证保险库?
4. **集成**: Loom daemon 加 cloud adapter / 独立 tencent-cloud-mcp daemon / 都不集成只产 skill?

**新增待写产物 (确认后)**:
- [ ] `internal/mcp/tencent-cloud/main.go` (mcp-go + tccli wrapper)
- [ ] `internal/mcp/tencent-cloud/tools_test.go`
- [ ] `.loom-drafts/tencent-cloud-mcp-design.md`
- [ ] `.loom-drafts/v2.0-evaluation-prompt.md` 追加 P0-2c 问题

---

## tencent-cloud SKILL.md 落地 (2026-08-01 18:15 SGT) — Mavis 现在就能管云

**触发**: 用户 2026-08-01 18:13 SGT — "我想先让你来作为Agent来管理腾讯云资源，通过skills"

**决策 (用户拍板)**:
- Q1 场景: 都管 (CVM + CloudBase + CDB + COS)
- Q2 路径: B. 自写 tencent-cloud-mcp (过渡用 SKILL.md, 目标 MCP daemon)
- Q3 凭证: macOS Keychain (跟 Loom 习惯一致)
- Q4 集成: 独立 tencent-cloud-mcp daemon (轻量解耦, 当前 SKILL.md 形态)

**交付物**: `~/.claude/skills/tencent-cloud/` (Anthropic SKILL.md 标准, 立即可用)
```
tencent-cloud/
├── SKILL.md                          # 4.0K, 触发条件 + 快速开始 + 安全约束
├── scripts/
│   ├── _creds.sh                     # 1.1K, 从 macOS Keychain 读凭证
│   ├── setup-keychain.sh             # 2.2K, 一键存 SecretId/SecretKey/Region
│   ├── cvm.sh                        # 4.5K, CVM list/describe/start/stop/reboot/reset-pass/destroy
│   ├── cdb.sh                        # 2.9K, CDB MySQL list/describe/start/stop/restart/query-log
│   ├── cos.sh                        # 2.9K, COS list/ls/cat/put/rm
│   └── cloudbase.sh                  # 2.5K, CloudBase envs/functions/databases/storage
└── references/
    └── tccli-cheatsheet.md           # 3.9K, 5 大服务命令速查
```

**核心设计**:
- 凭证走 macOS Keychain (service=tencent-cloud, account=tccli-{secretid,secretkey,region})
- 4 个服务统一通过 `_creds.sh` 读凭证, 绝不直接读环境变量
- 危险操作默认拒绝 (destroy/delete/reset-pass 必须 `--force`)
- bash 语法全过, 凭证未存时优雅失败并提示跑 setup-keychain.sh

**用户下一步 (我等他执行)**:
```bash
bash ~/.claude/skills/tencent-cloud/scripts/setup-keychain.sh
# 提示输入 SecretId/SecretKey/Region, 自动存 Keychain + 验证 DescribeRegions
```

**后续 (v2 MCP daemon)**:
- 等用户完成 setup 后, 我实测能 list CVM/CloudBase/CDB/COS
- 写 `.loom-drafts/tencent-cloud-mcp-design.md` 把这个 skill 包装成独立 MCP daemon
- 写 `internal/mcp/tencent-cloud/main.go` (mcp-go + tccli)
- 集成到 Loom v2.0 spec §3.6 Cloud Provider Adapter

**新增待写产物 (实测后)**:
- [ ] `.loom-drafts/tencent-cloud-mcp-design.md` (MCP daemon 设计)
- [ ] `internal/mcp/tencent-cloud/main.go` (mcp-go wrapper)
- [ ] `internal/mcp/tencent-cloud/tools_test.go`
- [ ] `references/troubleshooting.md` (错误处理速查)

---

## 腾讯云 AKSK 诊断 (2026-08-01 18:43 SGT) — 整个 APPID 被服务端拒认

**触发**: 用户给我 AKSK 让我直接调, 期望能绕过之前的 tccli 路径问题

**实测**:
- 给的 AKSK: `AKIDWJwR...Qjp` / `CE4jJv...iiYK` (32 字符 secretKey, 格式对)
- `tccli cvm DescribeInstances --region ap-shanghai --Limit 1` 立即返回 `secretId is invalid` exit 255
- 3 个不同 secretId (老的 AKIDQghS / 新的 AKIDZJvP / 这次 AKIDWJwR) **全部**被拒

**根因诊断**:
- ✅ API Explorer 用主账号 session 调能跑通 (返回 TotalCount: 0) — 说明账号本身"还活着"
- ❌ 但所有 secretId + 任何 SDK (tccli / tencentcloud-sdk-python 即将验证) 都 `secretId is invalid`
- ❌ 用户在控制台看到密钥状态"启用"
- → **腾讯云服务端 (cvm.tencentcloudapi.com) 拒认这个 APPID 1322505111 下的所有 secretId**

**可能根因 (按概率)**:
1. 欠费冻结 (50%): https://console.cloud.tencent.com/expense 查余额
2. 违规被风控 (30%): 频繁创建/删除实例触发反作弊
3. 账号冻结 / 子系统异常 (20%): 需工单

**用户下一步**:
1. 🔥 立即 revoke 刚贴的 AKSK (已经在 mavis 会话历史里)
2. 查账户状态 (https://console.cloud.tencent.com/expense)
3. 工单申诉 (https://console.cloud.tencent.com/workorder) 选"账号类 > API 密钥异常"

**已写入 memory** (跨项目 lesson, 写完后追加)

---

## WorkBuddy 腾讯云 API 专家 skill 发现 (2026-08-01 18:48 SGT) — 绕过 AKSK 风控的第二条路

**触发**: 用户提到 "WorkBuddy - 腾讯云API专家"

**关键发现**:
- 腾讯云官方 API 中心页面 (cloud.tencent.com/document/api) 有官方 skill 入口: "腾讯云 API 专家"
- 描述: "支持在 Workbuddy 通过自然语言管理腾讯云 200+ 产品资源, 智能检索 API 并构造 CLI 命令"
- **意义**: WorkBuddy 客户端 + 腾讯云 API 专家 skill 走的是腾讯云官方认证通道, **可能绕开被风控的 APPID 1322505111 下的 AKSK**

**WorkBuddy 是什么** (2026-03-09 上线):
- 腾讯云 CodeBuddy 团队的官方桌面 AI Agent 客户端
- 兼容 Anthropic SKILL.md 协议 (1.3万+ skills)
- 内置腾讯混元 / DeepSeek / GLM / Kimi / MiniMax 模型
- 微信/QQ/飞书/钉钉 一键直连
- 注册送 5000 Credits, 每月 500 免费, 每日签到 100

**与用户 tencent-cloud SKILL.md 的关系**:
- 用户写的 ~/.claude/skills/tencent-cloud/ 走 tccli (用户 AKSK, 被风控)
- WorkBuddy 腾讯云 API 专家走腾讯云官方 skill (腾讯云自己认证, 可能不同认证通道)
- **两个独立通道, 一个被风控不影响另一个**

**下一步建议 (并行两条路)**:
- **路 A** (你之前选的): 等子账号建好 / 工单解封, 重测 tencent-cloud SKILL.md
- **路 B** (新发现, 立即可用): 下载 WorkBuddy 客户端 → 装"腾讯云 API 专家" skill → 立即通过自然语言管云

**等用户决定**: 走 B 路试试? 还是先按原计划 A 路?

---

## WorkBuddy 腾讯云 API 专家 验证 — 走 AKSK 鉴权, 不会绕开风控 (2026-08-01 18:55 SGT)

**触发**: 用户让我搜 WorkBuddy 腾讯云 API 专家实际怎么用

**关键发现 (来自 CloudQ Skill 教程原文)**:
> 必填: 配置 AK/SK. CloudQ 需要访问你的腾讯云账号才能执行巡检、查询等操作.
> 登录腾讯云控制台, 进入访问密钥管理获取你的 SecretId 和 SecretKey
> 在 WorkBuddy 的 CloudQ Skill 配置中, 将 AK/SK 填入对应的环境变量

**修正之前的错误判断**:
- ❌ 错: "WorkBuddy 走腾讯云官方认证通道, 绕开用户 AKSK 风控"
- ✅ 对: WorkBuddy 跟 tccli / tencentcloud-sdk-python / API Explorer **用同一套 secretId/secretKey 鉴权**, 调同一组 endpoint (cvm.tencentcloudapi.com 等)
- HMAC-SHA256 签名是腾讯云标准, 任何客户端最终都生成同样格式的 Signature
- **WorkBuddy 客户端登录走 OAuth/SSO** 跟 **AKSK API 调调用** 是两套独立机制:
  - 登录: 腾讯云账号体系 (跟 APPID 1322505111 关联)
  - API 调: 用户 AKSK (跟 APPID 1322505111 关联, 已被服务端拒认)
- 如果 AKSK 被拒, WorkBuddy 登录 OK, 但 "腾讯云 API 专家" skill 调任何 API 也会 `secretId is invalid`

**WorkBuddy 仍然有用 (但要等 AKSK 修好)**:
- AKSK 修好后, WorkBuddy 是个"自然语言管云" 利器 (200+ 产品)
- 跟用户的 ~/.claude/skills/tencent-cloud/ 是互补关系, 不替代

**真正的修法 (按优先级)**:
1. 🔥 **立即**: revoke 刚贴的 AKSK `AKIDWJwR...Qjp`
2. 💰 **查账户状态**: https://console.cloud.tencent.com/expense (欠费/冻结/违规)
3. 🛠 **提工单**: https://console.cloud.tencent.com/workorder 选 "账号类 → API 密钥异常"
4. 🆕 **子账号**: 在 https://console.cloud.tencent.com/cam 建子用户, 给 QcloudCVMReadOnlyAccess, 用子用户 AKSK 试 (虽然同 APPID 仍可能被风控, 但绕过主账号的某些限制)

**预期时间线**:
- revoke: 1 分钟
- 查账户: 2 分钟
- 提工单: 5 分钟, 客服 1-3 工作日回复
- 子账号: 10 分钟

**同步 memory**: 已记录 "WorkBuddy 跟 tccli 走同一套 AKSK 鉴权" 的 lesson
