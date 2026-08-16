# P2D-W2C/W2D Governed Test Report Board V28

Status: `SOURCE VERIFIED / INSTALLED BOARD LIVE OPEN`  
Date: 2026-08-15  
Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

## Acceptance boundary

Users can see whether the current Attempt for each Agent has governed test
evidence without trusting Provider prose or opening raw tool output. The Board
is a read model. Its report fields cannot authorize execution, fallback,
acceptance, policy, grants or terminal state.

This slice summarizes V27 command-level reports. It does not parse individual
test cases or display command/output content.

## Implementation

- Added an observational `GovernedTestReportSource` to Team timeline reads and
  wired the existing Attempt Loop authority through the product read service
  and compatibility Bundle facade.
- Reconstructs each report query from the exact projected current Attempt:
  Team, conversation, WorkItem, Run, claim ID/generation, Runtime, Agent,
  Incident, frozen Execution Binding digest and Context Capsule digest.
- Validates the complete report set through the V27 stable set-digest boundary.
  Empty, invalid, duplicate or unavailable state is omitted.
- Projects only report count, passed/failed counts, stable set digest and the
  latest runner, scope, outcome and report digest.
- Preserves older wire payloads with absent report fields. Swift rejects
  contradictory availability/counts, unknown closed values, malformed digests
  and report metadata without a frozen Execution Binding.
- Mission Inspector renders `Tests N passed, M failed` and the latest governed
  runner/scope/outcome on the exact Agent row.

## Privacy and failure isolation

- Raw command, arguments, stdout/stderr, Prompt, transcript, Provider response,
  API key, Authorization header and credential reference/revision are not
  added by this Board projection.
- Report-source failure does not create success, failure or Team-wide offline
  state. The affected report summary is simply absent.
- Reports from an old Attempt or another Agent cannot be selected by request
  input; the query is derived from authoritative projection identity.
- A duplicate call sequence invalidates the whole report set instead of
  choosing one value.

## Verification

Passed:

```text
go test ./internal/api -count=1
go test ./cmd/loomd -run 'TestPhase2DBoardProjectsOnlyExactGovernedTestReportSummary|TestProductAttemptLoopRuntimeGovernsLocalToolDispatchAndDelivery|TestProductComposition' -count=1
go test -race ./internal/api -run '^TestPhase2DBoardProjectsOnlyExactGovernedTestReportSummary$' -count=10
go vet ./internal/api ./cmd/loomd
gofmt diff check on affected Go files
swift test --package-path apps/macos --filter 'LocalProductModelsTests/testPhase2DTimelineNodeDecodesIndependentAgentBinding|LocalProductModelsTests/testPhase2DBoardRejectsContradictoryBindingAndAccounting|LocalProductExperienceViewTests/testMissionInspectorTabsExposeDistinctSafeReadOnlyContent'
swift test --package-path apps/macos
```

The full Swift package result was `223` XCTest cases with `1` skipped and `0`
failures, plus `10` Swift Testing cases with `0` failures. This slice did not
rerun the complete Go repository because V27 already recorded the existing
Darwin Keychain timing residual; the affected Go packages and race boundary
above are fresh.

## Remaining

- per-test-case and general verifier artifact parsing;
- raw report disclosure/diagnostic inspection under an explicit privacy gate;
- installed App Board verification and real Provider conversation;
- installed Credential Vault CV6 and four-Agent mixed-Team ATL9;
- account-local live failure isolation, complete accounting UI and COMP2-E.

No App, network, real Provider, real credential, user workspace or external
Runtime was accessed for this source verification.
