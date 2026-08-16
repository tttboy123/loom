# Post-S3-W5 Queued Contract Inputs

Status: QUEUED — non-authoritative; does not expand S3-W5.

Date: `2026-07-26`

This file preserves explicit user authorization for later governance. It
cannot authorize implementation until the current S3-W5 Candidate is reviewed
and committed and the applicable Slice contract or reviewed amendment is
frozen.

## Observable Node Output reconciliation

This queued input was evaluated before the current S3-W5 Candidate. Slice 3
Exit Contract Amendment 3 and S3-W5 Contract Amendment 2 already reopened only
these accepted S3-W4 production/test pairs:

- `internal/supervisor/managed_execution.go`
- `internal/supervisor/managed_execution_test.go`
- `internal/runtime/piadapter/execution_adapter.go`
- `internal/runtime/piadapter/execution_adapter_test.go`

They froze and implemented the required trust order: Adapter decode,
Supervisor Bridge/binding/sequence validation, `AgentGrant.Authorize`,
`BoundRunStream` commit, then observer. Malformed, unauthorized, or
stale-generation Frames never publish. Bridge v1 did not change and no S3-W6
was created. Therefore this item is satisfied inside the current reviewed
vertical boundary and is not a request for another amendment.

S3-W5 itself remains execution mechanics only: logical node plus attempt
identity, explicit states and `retry_at`, bounded scheduling, independent
Run/generation/Grant/Evidence lineage, Journal milestones, restart recovery,
single CAS winner, and no hidden retry. That boundary is now implemented as
one vertical Candidate and must not be split into scheduler/writer/coordinator
WorkItems.

## Slice 4 semantic recovery policy

Freeze a later policy boundary that separates:

- `internal/verification/output_contract.go`, classifying
  `valid_nonempty`, `valid_empty`, `transient_empty`, or `invalid`; and
- `internal/rules/recovery_policy.go`, deciding
  `retry`, `fallback`, `degraded`, `blocked`, or `human_required`.

The scheduler only executes accepted decisions and never invents acceptance
semantics. This is workflow data-source degradation, not automatic Provider
fallback excluded by `TECH-PLAN.md`.

## Slice 5 client delivery

Consider `internal/api/team_execution_stream.go` plus bounded daemon/CLI wiring.
Tentative text deltas use bounded in-memory delivery; authoritative
started/retry/degraded/terminal facts come from Journal/Projection. Reconnect
uses cursor or `Last-Event-ID` plus `GlobalReadView`.

Slow consumers may coalesce text deltas but may not drop warning, retry,
degraded, or terminal facts. Overflow emits `stream_gap` plus artifact digest.
Per-token Journal Events are forbidden. Phase 1 may claim only a local event
interface or CLI timeline; Web/TUI delivery remains excluded.

Raw Grants, credentials, direct identifiers, and hidden reasoning never enter
the stream. Incremental output is tentative until terminal and Evidence
acceptance.

## Later checkpoint hardening

Phase 1 does not add model-context or process-image checkpointing, make a
checkpoint authoritative, or introduce incremental Projection checkpoints.
Those remain post-Slice-3 hardening candidates only if measurements justify a
separate governed contract.

VERDICT: QUEUED
