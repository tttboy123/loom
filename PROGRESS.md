# Loom Phase 1 Slice 1 Progress

Updated: 2026-07-25

## Execution truth

- Workspace: `/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild`
- Slice 1 completion commit: `5861f82`
- Current branch: `codex/loom-platform-slice2`
- Existing `codex/loom-platform` branch was not moved or overwritten
- Preserved pre-existing untracked paths: `.codex/installation_id`,
  `.codex/skills/`, `.loom-drafts/`
- Slice 2: S2-W1 `954416a`, S2-W2 `1170062`, S2-W3 `e196107`, S2-W4
  `0a98851`, S2-W5 `567967c`; S2-W6 accepted and locally committed in this
  checkpoint
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
- Status: `S2-W15_ACCEPTED_NOT_ACTIVATED`
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
- Next gate: local atomic S2-W15 commit, then freeze and independently review
  Team/Agent read-model projection
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
