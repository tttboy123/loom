# Phase 1 Slice 4 Exit Contract Amendment 2A

Status: FROZEN — independent Review 1 PASS.

Date: 2026-07-26

Baseline: `87ea092`

Parent: `EXIT-CONTRACT-AMENDMENT-2.md`

## Name

S4-W2 GlobalReadView Team Type Ownership Repair

## Trigger

Mandatory RED preparation located the accepted Team projection record types and
their deep-copy implementation in:

- `internal/projection/global_read_view.go`

Amendment 2 already requires S4-W2 to expose frozen semantic bindings,
classification metadata, recovery decisions, budgets, and workflow paths
through the existing immutable Team execution Projection/GlobalReadView.
Editing only `internal/projection/team_execution.go` can parse those fields but
cannot add them to `TeamExecution`, `TeamExecutionNode`, or
`TeamExecutionAttempt`, nor update their accepted copy behavior.

Moving the accepted types into another file would still edit
`global_read_view.go` and would create unnecessary structural churn. A second
projection type/cache would violate one-projection authority.

## Exact additional reopened scope

S4-W2 may additionally reopen only:

- `internal/projection/global_read_view.go`
- `internal/projection/global_read_view_test.go`

No other product, test, plan, or accepted authority file is added.

## Permitted changes

- Add only the S4-W2 frozen Team semantic/classification/recovery value fields
  to the existing Team execution record types.
- Extend existing record-local deep-copy helpers and typed accessor tests.
- Preserve all non-Team GlobalReadView fields, versioning, digesting,
  publication, and accessors unchanged.
- Keep Projection/GlobalReadView rebuildable and non-authoritative.

## Required proof

- exact S4-W2 Team fields survive full replay and typed access;
- caller mutation of returned nested Team attempt/recovery data cannot mutate
  the stored view;
- malformed Team recovery replay preserves the previous complete view;
- accepted historical S3 legacy-unbound Team streams still rebuild; and
- scope, full repository, race, vet, format, diff, and trust checks pass.

## Preserved exclusions

This repair adds no classifier/policy behavior, writer, scheduler, acceptance,
Verifier, API/CLI/daemon, Runtime/Provider, dependency, S4-W3, S4-W4, or Phase
2 capability. It does not authorize product implementation before independent
review.

VERDICT: FROZEN
