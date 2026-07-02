# Agent Team Intake - 2026-07-01

## Source

- Inbox: `external-requests/requests.yaml`
- Request root: `external-requests/requests/buddys`
- Agent Team run: `devkit/runs/external-requests-intake-20260701`

## Decisions

| Request | Decision | Next action |
| --- | --- | --- |
| `loom-upstream-001-devflow-capability-alignment` | Accept and split | Keep as parent umbrella; do not implement as one batch. |
| `loom-upstream-002-materialization-and-apply-contract` | Accept, implement minimal slice first | Add generic required-output materialization and apply-outcome classification. |
| `loom-upstream-003-external-verification-surface-contract` | Accept, defer | Implement after apply/materialization closeout is stable. |
| `loom-upstream-004-external-queue-lease-and-recovery` | Accept, defer | Implement as separate lease/recovery slice. |
| `loom-upstream-005-external-candidate-drift-and-failure-classification` | Accept and split | Slice 1: generic failure classification mapping; defer drift gate and packet emitter wiring. |

## Review Fixes Applied Before Implementation

- Canonical block reason is `partial_write`; legacy inputs `partial-write` and `partial write` are normalized.
- Slice 1 excludes verification-surface fields, queue lease fields, and `empty_stage_diagnostics`.
- Output contract remains downstream-agnostic and does not encode Buddys paths, models, roles, or document layout.

## Slice 1 Acceptance

- Contract accepts optional `required_output_paths` and `apply_policy`.
- Unsafe required paths are rejected: absolute paths, parent escapes, and empty paths.
- Apply outcome is one of `applied`, `not_applied`, `apply_blocked`, `apply_partial`, `apply_not_attempted`.
- Per-path status is one of `materialized`, `missing`, `partial`, `blocked`.
- Block reason vocabulary is limited to `lock`, `policy`, `partial_write`.
- Closeout packet is machine-readable and does not contain downstream-specific fields.

## Implemented

- `devkit/materialization_contract.py`
- `devkit/test_features.py::MaterializationContractTest`

## Request 005 Slice 1

Source run: `devkit/runs/external-requests-005-intake-20260701`

- Added closed `failure_kind` enum:
  - `candidate_topic_drift`
  - `missing_contracted_outputs`
  - `verification_failed_authoritative_surface`
  - `review_rejected`
  - `empty_or_non_actionable_model_output`
- Added closed `next_action_hint` enum:
  - `retry_immediate`
  - `retry_different_carrier`
  - `skip_candidate_reopen_task`
  - `cool_down`
- Added deterministic outcome-tag mapping with explicit fallback.
- Kept the mapping advisory and downstream-agnostic.
- Deferred result packet emitter wiring until schema-version and field-append policy are confirmed.

Implemented:

- `devkit/failure_classification.py`
- `devkit/test_features.py::FailureClassificationTest`
