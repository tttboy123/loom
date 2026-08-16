# P2D-W2C/W2D Observed Acceptance State V26

Status: `SOURCE VERIFIED / FULL-REPOSITORY DAEMON STABILITY RESIDUAL`  
Date: 2026-08-15  
Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

## Acceptance boundary

A dependent or aggregation Agent must not infer that a source was accepted from
Provider-generated prose. Before source output is disclosed, Loom must validate
the already-authoritative projected Attempt status, Output Contract, output
classification and acceptance decision. The resulting Role Context Capsule
must preserve the distinction between authoritative lineage, observed
execution state and untrusted model output.

This slice deliberately does not claim structured test-result ingestion. A
model sentence such as "tests pass" remains untrusted content unless a future
tool/verifier path commits a typed authoritative result.

## Implementation

- Added `observed_execution_state` as a valid Role Context Capsule item kind.
- `TeamAggregationSource` now carries projected terminal status, Output Contract
  version/digest, output classification/digest and acceptance
  decision/digest/time.
- Source resolution reads these fields from the exact projected dependency
  Attempt and Team node after Evidence lineage validation.
- Capsule assembly requires `succeeded`, a valid versioned Output Contract,
  `valid_nonempty` or `valid_empty`, a valid classification digest, and an exact
  accepted decision digest with a UTC decision time.
- Every source contributes three role-restricted items: authoritative lineage,
  observed acceptance state and untrusted model output.
- The observed payload is intentionally compact to preserve Pi's existing 6 KiB
  ContextAdapter dispatch limit. Output Contract and Evidence digests are still
  validated and remain bound by adjacent authoritative lineage.

## Negative guarantees

- Classification and acceptance-decision substitutions fail closed.
- Provider-generated content cannot become observed, authoritative, policy,
  Goal, grant, WorkItem or acceptance state.
- Provider Account, credential reference/revision and private peer Capsule
  content are not copied into the dependent prompt.
- No Prompt, model output, Provider body, secret or Authorization header is
  added to Journal or operational diagnostics.
- No structured `test_report`, command result or verifier receipt is claimed by
  this slice.

## Verification

Passed:

```text
go test ./internal/contextcapsule ./internal/app -run '<V26 focused matrix>' -count=1
go test ./internal/contextcapsule ./internal/evidence ./internal/app -count=1
go test -race ./internal/contextcapsule ./internal/app -run '<V26 focused matrix>' -count=10
go test ./internal/contextcapsule ./internal/evidence ./internal/app ./internal/api ./internal/work ./internal/projection ./cmd/loomd -count=1
go vet ./internal/contextcapsule ./internal/evidence ./internal/app ./internal/api ./internal/work ./internal/projection ./cmd/loomd
gofmt and git diff --check on the affected source set
go test ./cmd/loomd -run '^TestProductDaemonContainsExactPiMetadataTimeoutWithoutStoppingIPC$' -count=1 -v
go test ./cmd/loomd -run '^TestProductLoomNativeAttemptRecoveryRejectsUnsupportedAndDriftedState$' -count=1 -v
```

The fresh serial full-repository command:

```text
go test -p 1 ./... -count=1
```

passed every package except `cmd/loomd`. That package first reported
`TestProductDaemonContainsExactPiMetadataTimeoutWithoutStoppingIPC` with
`invalid local IPC protocol`, then hit the 10-minute package timeout while a
Loom Native recovery test was active. The implicated tests passed independently
in 13.38 seconds and 0.43 seconds. This is retained as a daemon suite
timing/stability residual; the full-repository gate is not reported as passing.

## Remaining

- typed authoritative `test_report` and tool/verifier result ingestion;
- model-specific tokenizer accounting and Provider ContextAdapters;
- user-visible disclosure receipt and omission inspection;
- installed Credential Vault CV6 and real multi-turn Provider acceptance;
- installed four-Agent mixed-Team ATL9 and account-local failure isolation;
- COMP2-E removal gates and complete accounting/governance UI.

No App, network, real Provider, real credential, user workspace or external
Runtime was accessed for this source verification.
