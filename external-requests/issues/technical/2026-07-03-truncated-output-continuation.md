# Technical Issue: Truncated Output Continuation Recovery

## Status

Deferred for product discussion.

## Source

Observed while validating Loom local runtime on 2026-07-03.

## Problem

When a provider returns a partial but non-empty answer with `finish_reason=length`,
Loom needs a reliable continuation strategy instead of treating the partial text
as final truth or dropping the run into a generic materialization failure.

Current risks:

- partially emitted file content reaches materialization
- long implementation tasks become flaky even when the model is otherwise
  producing correct content
- repeated retries waste tokens because the system restarts from scratch instead
  of continuing from the truncation point

## Why this is an external dependency style issue

This is a general runtime robustness concern caused by provider/output limits
and response behavior, not by any single downstream project.

## Requested follow-up discussion

Evaluate whether Loom should standardize:

- continuation-at-length retry policy
- merged-output deduplication rules
- partial-file closure checks
- per-stage continuation attempt tracing
- escalation rules when continuation still returns incomplete content

## Evidence

- run: `devkit/runs/smoke-dev-20260703`
- response diag showed `finish_reason=length`
- resulting stage text ended mid-section
