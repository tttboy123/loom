# P2D-W2C/W2D Role Dependency Context Capsule V25

Status: `SOURCE VERIFIED / INSTALLED LIVE OPEN`  
Date: 2026-08-15  
Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

## Acceptance boundary

An ordinary Team DAG dependency must transfer only its exact authorized output
to the dependent Agent. Scheduling readiness alone is insufficient, and the
implementation must not broadcast a source Agent's complete Capsule, Provider
Account, credential identity, private Evidence frames or internal runtime
state.

Each dependent Attempt freezes a newly derived Role Context Capsule and Route
Segment before authoritative dispatch. Source Attempt lineage is authoritative;
model-produced content remains untrusted reference material.

## Implementation

- `contextcapsule.ExtendRoleContextCapsule` extends an admitted Capsule without
  permitting item replacement. It deterministically repacks all disclosed and
  budget-retrievable items, preserves policy/access omissions as content-free
  metadata, and recomputes Capsule and disclosure-receipt digests.
- A new `dependency_source_authority` item kind binds the exact source node,
  Attempt, WorkItem, Run, claim generation, Runtime, Agent, Evidence receipt and
  output-summary digest.
- `TeamCoordinator` resolves every dependency from the authoritative Team
  Projection, reopens the exact finalized Evidence receipt and uses the
  existing strict authorized-output extraction boundary.
- The extracted event content is a separate role-restricted
  `untrusted_model_output`. It cannot become Goal, policy, grant, accepted
  decision, WorkItem or execution authority.
- The dependent dispatch, Capsule authority and Route Segment are rebuilt
  together before `DispatchTeamReadySet` freezes the Attempt.
- Parallel Route aggregation uses the same extension primitive, preserving its
  existing sibling/route-group restrictions.

## Negative guarantees

- Peer node, source Agent, receipt, Evidence digest, output-summary digest and
  plan-node substitutions fail closed.
- Required provenance that cannot fit the target token budget fails closed.
- Provider Account, credential reference/revision and peer private Capsule
  content are not copied into the dependent prompt.
- Evidence/result/private runtime frames are not treated as dependency model
  output; only authorized output events cross the boundary.
- No Prompt, dependency output, Provider body, secret or Authorization header
  is added to Journal or operational diagnostics.

## Verification

Passed:

```text
go test ./internal/contextcapsule ./internal/app -run '<V25 focused matrix>' -count=1
go test -race ./internal/contextcapsule ./internal/app -run '<V25 focused matrix>' -count=10
go test ./internal/contextcapsule ./internal/evidence ./internal/app ./internal/api ./internal/work ./internal/projection ./cmd/loomd -count=1
go test -p 1 ./... -count=1
go vet ./internal/contextcapsule ./internal/evidence ./internal/app ./internal/api ./internal/work ./internal/projection ./cmd/loomd
gofmt and git diff --check on the affected source set
```

The four-Provider DAG canary verifies that a Codex/OpenAI Main Agent receives
the authorized outputs of Claude/Anthropic, Loom/Kimi and Loom/MiniMax
dependencies while their Provider Account and credential identities remain
undisclosed. The dynamic fallback canary verifies a second Attempt with a
different Runtime retains exact dependency authority and freezes a new
Capsule/Segment.

## Remaining

This slice does not complete Role Context Capsule or Phase 2D. Still open:

- authoritative observed test-state assembly;
- model-specific tokenizer accounting and Provider wire adapters;
- user-visible disclosure receipt and omission inspection;
- installed Credential Vault CV6 and real multi-turn Provider acceptance;
- installed four-Agent mixed-Team ATL9 and account-local failure isolation;
- COMP2-E removal gates and complete accounting/governance UI.

No App, network, real Provider, real credential, user workspace or external
Runtime was accessed for this source verification.
