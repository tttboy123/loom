# P2D-W2C/W2D Governed Test Report V27

Status: `SOURCE VERIFIED / FULL-REPOSITORY CREDENTIAL TEST RESIDUAL`  
Date: 2026-08-15  
Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

## Acceptance boundary

A model sentence such as "tests pass" is untrusted output. Loom may disclose a
test outcome as observed state only when a recognized test command ran through
the governed Tool Gateway and the exact ToolCall, Attempt authority, encrypted
result payload and delivery transition are all authoritative and replayable.

This slice records command-level exit observations. It does not parse
individual test cases or accept arbitrary model/verifier documents as facts.

## Implementation

- Added an immutable `GovernedTestReport` contract with strict canonical JSON,
  digest validation and an order-independent report-set digest.
- Recognizes a single-command subset of Go, Swift, Cargo, Pytest, npm, pnpm,
  Yarn and Bun test runners. Shell chaining, substitution, redirection, quoted
  shell fragments, environment prefixes and unsupported commands are rejected.
- The common Tool Gateway constructs a report only after the exact Bash result
  payload has been encrypted and accepted by Attempt payload authority.
- Reports bind runner, coarse scope, exit-derived outcome, ToolCall sequence,
  argument digest, execution identity, output digest and duration.
- `GovernedTestReportCommitted` is replayed inside the Attempt Loop stream.
  Exact idempotent replay succeeds; argument, payload or Attempt substitution
  fails closed.
- Team queries return only reports whose exact Tool result is `delivered`.
- `TeamCoordinator` derives report queries from the projected dependency
  Attempt, frozen Execution Binding and Context Capsule. Report state is not a
  user/model-supplied Team request field.
- Dependency and aggregation Capsules add compact report summaries and a set
  digest to the existing role-restricted `observed_execution_state` item.
  Provider output remains a separate `untrusted_model_output`.

## Negative guarantees

- Ordinary Bash commands do not create governed test reports.
- Raw command, stdout/stderr, Prompt, Provider response and credential content
  are not added to Journal, diagnostics or Role Capsule by the report path.
- An accepted but undelivered result is not disclosed as observed test state.
- A model cannot manufacture a report, change its outcome or promote its own
  prose into authoritative/observed state.
- A report does not authorize execution, fallback, policy, grants, WorkItems or
  terminal state.

## Verification

Passed:

```text
go test ./internal/verification ./internal/work ./internal/app -count=1
go test ./cmd/loomd -count=1
go test -race ./internal/verification ./internal/work ./internal/app -count=1
go test -race ./cmd/loomd -run '^TestProductAttemptLoopRuntime(GovernsLocalToolDispatchAndDelivery|DoesNotReportOrdinaryBashAsTests)$' -count=1
go vet ./internal/verification ./internal/work ./internal/app ./cmd/loomd
go test ./internal/credentials -run '^TestKeychainHelperProcessOutputFailsClosed$' -count=1 -v
go test ./internal/credentials -count=3
gofmt and diff checks on the affected source set
```

The fresh full-repository command:

```text
go test ./... -count=1
```

passed `cmd/loomd`, all V27 packages and every other reported package except
`internal/credentials`. `TestKeychainHelperProcessOutputFailsClosed` expected
`helper_response` but observed `helper_request` during that run. The isolated
test passed immediately, and the complete credentials package then passed three
consecutive runs. This pre-existing Darwin helper timing/stage residual is
preserved; the full-repository gate is not rewritten as a pass.

## Remaining

- per-test-case/JUnit/`go test -json` parsing and general verifier artifacts;
- user-visible report, disclosure receipt and diagnostic inspection;
- model-specific tokenizer accounting and Provider ContextAdapters;
- installed Credential Vault CV6 and real multi-turn Provider acceptance;
- installed four-Agent mixed-Team ATL9 and account-local failure isolation;
- COMP2-E removal gates and complete accounting/governance UI.

No App, network, real Provider, real credential, user workspace or external
Runtime was accessed for this source verification.
