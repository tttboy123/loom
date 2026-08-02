# P2A-W3 Candidate Verification

**Date**: 2026-08-01  
**Baseline**: `848f068cbc0307f14473f6a71961db949a8734ca`  
**Candidate status**: `DETERMINISTIC GATES PASS / IMPLEMENTATION REVIEW 4 PASS / NOT ACCEPTED`  
**Live status**: `THREE ONE-SHOT GATES CONSUMED / PRODUCT FAIL / HUMAN_REQUIRED`

## Commands and results

```text
go test ./internal/teams \
  -run '^(TestBuildSavedTeamRuntimeBinding|TestValidateSavedTeamRuntimeBinding)' \
  -count=1
PASS

go test ./internal/teams -count=1
PASS

go test -race ./internal/teams \
  -run '^(TestBuildSavedTeamRuntimeBinding|TestValidateSavedTeamRuntimeBinding)' \
  -count=50
PASS

go test ./cmd/loomd \
  -run '^TestProductDaemonExecutionCompositionMaterializesConfirmedTeamForPreflight$' \
  -count=1
PASS

go test ./internal/tui \
  -run '^TestModelConfirmedExecutableTeamRefreshesSetupAndAuthoritativeSnapshot$' \
  -count=1
PASS

swift test --package-path apps/macos \
  --filter 'LocalProductStoreTests/testConfirmedExecutableTeamRefreshesAuthoritativeProductView'
PASS

go test ./cmd/loomd \
  -run '^TestProductMissionExecutionVerticalLoopbackClosesAuthorizedLineage$' \
  -count=10
PASS

go test ./internal/app \
  -run '^(TestPreparedMissionExecutionDecisionRouterMapsExactRecoveryCommand|TestAuthoritativeMissionPreparedControlRoutesExactCurrentLineage|TestAuthoritativeMissionPreflightExpiresWithoutViewChange|TestAuthoritativeMissionExecutionFlightRegistryIsBounded)$' \
  -count=1
PASS

go test ./internal/localipc \
  -run '^TestStrictSwift(ContractProbeExecutesMissionThroughRealGoServer|ExecutionProbeRejectsMalformedPreflightWire)$' \
  -count=1
PASS

go test ./internal/app \
  -run '^(TestLocalProductExecutionRejectsInvalidClosedCommandsBeforeBackend|TestAuthoritativeMissionPreparedControlRoutesExactCurrentLineage)$' \
  -count=1
PASS

go test ./internal/api ./internal/app ./internal/localipc ./internal/teams ./internal/tui ./cmd/loomd -count=1
PASS

go test -race ./internal/api ./internal/app ./internal/localipc ./internal/teams ./internal/tui ./cmd/loomd -count=1
PASS

go test -timeout=8m -p=1 -count=1 ./...
PASS

go test -race -timeout=12m -p=1 -count=1 ./...
PASS

go vet ./...
PASS

go mod verify
all modules verified

TMPDIR=/private/tmp \
CLANG_MODULE_CACHE_PATH=/private/tmp/loom-p2aw3-clang-final \
SWIFT_MODULE_CACHE_PATH=/private/tmp/loom-p2aw3-swift-final \
swift test --package-path apps/macos
57 XCTest executed; 1 explicit visual-export test skipped; 0 failures
4 Swift Testing checks passed

swift test --sanitize=thread --package-path apps/macos
PASS

swift build --package-path apps/macos -c release --product LoomLocalApp
PASS

git diff --check
PASS
```

## Format, dependency and scope notes

- Every W3-owned Go file is `gofmt` clean.
- Repository-wide `gofmt -l` reports only four unrelated, excluded
  `internal/mcp/sdk/**` files already outside the frozen contract.
- Swift strict lint reports legacy deviations only in accepted pre-existing
  sections of `LocalIPCClient.swift` and older tests; no bulk unrelated rewrite
  was performed.
- Candidate changes are confined to the exact contract-owned product/test,
  W3 evidence and `docs/CURRENT.md` paths.
- Repair 1 reopens only already-owned W3 paths. It modifies no accepted
  Journal, Projection, Rules, Work, Grant, Evidence, Supervisor, Runtime adapter
  or bridge authority.
- Repair 2 reuses only the same owned App/daemon/TUI/Swift product and test
  paths. Canonical Mission identity is derived from the already-authoritative
  Team identity; no durable field or second authority is added.
- Repair 3 reuses only parent-W3-owned daemon/TUI/Swift product paths plus the
  two Saved-Team binding files explicitly reopened by the reviewed Dormant
  Capacity Amendment. It composes accepted Saved-Team builders and the sole
  `CommitSavedTeamInstanceRecordSet` authority; it does not append raw Events.
- Materialization validates and retains every Main/SubAgent binding but counts
  only the active-on-materialization Main against capacity. Dormant SubAgents
  create no AgentInstance, Run, lease, process or reservation.
- Product reconstruction groups projected Runtime records by their exact
  authoritative `SourceProbeID`; the deterministic fixture uses the canonical
  discovery digest
  `71c3783320e0a7f37ae5c17fd7f4182879f111e2116cc87c130f6dfc25925a77`.
- Confirming a saved Team now creates exactly one TeamInstance and one Main
  AgentInstance, creates zero WorkItem/Run/Grant/execution facts, and refreshes
  both TUI and native authoritative views before preflight.
- User-owned unrelated dirty/untracked files remain unstaged and unchanged by
  this Candidate.

## Live results and final gate

- source lock: `03d213d9862ce1f7be95d4547bfaae67893e58a8c98018d6a6cd651bd251c3ce`;
- fresh Implementation Review 4: `PASS`, no P0/P1;
- Codex: `FAIL — CODEX AVAILABLE / PRODUCT PREFLIGHT NOT REACHED`;
- MiniMax: `FAIL — VERIFICATION FACT NOT COMMITTED`;
- Pi: `FAIL — PRODUCT PREFLIGHT CONFLICT / NO START`;
- Codex Result Review: `PASS` for evidence trustworthiness, no P0/P1;
- combined MiniMax/Pi Result-Evidence Review: `PASS` for evidence
  trustworthiness, no P0/P1;
- no-terminal walkthrough: not run because all prerequisite live gates must
  pass first;
- final disposition: `HUMAN_REQUIRED / NO COMMIT / NO P2A-W4`.

The Pi lineage is additionally positive evidence for the exact bounded Dormant
Capacity Amendment: one capacity-1 live Runtime materialized one Main and kept
the SubAgent dormant. Its authority inventory has one TeamDefinition, one
TeamInstance and one AgentInstance, but zero WorkItem, Run, Grant, Evidence,
dispatch or TeamExecution fact. The next product boundary failed before Start
on exact model-ID namespace incompatibility.

## Claim limits

- Deterministic loopback proves the full product execution chain but is not a
  live Provider claim.
- Codex remains a native-auth selection/preflight claim only.
- MiniMax remains a brokered non-generative verification/preflight claim only.
- Pi is the sole live controlled-execution claim permitted after review.
- Every frozen live allowance is consumed and may not be retried.
- Failed live evidence is not acceptance evidence.
- A repair requires a reviewed complete W3 reopen; no point Amendment or W4 is
  implied or authorized by this report.
