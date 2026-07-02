# Technical Issue: Materialization Protocol Gap

## Status

Deferred for product discussion.

## Source

Observed while validating Loom local runtime on 2026-07-03.

## Problem

Loom can receive a plausible implement-stage answer that contains code and file
intent, but the structured artifact contract is still weaker than it should be
for long or partially truncated outputs.

Current symptoms:

- stage body may contain usable file content without a strong machine-readable
  file manifest
- artifact metadata may be too weak for external callers to trust without
  rescanning prose
- large outputs mix explanation, code, and file intent in one body

## Why this is an external dependency style issue

This is not tied to one downstream project. Any external caller that wants
durable apply semantics needs a stronger file/output contract from Loom.

## Requested follow-up discussion

Evaluate whether Loom should standardize a stronger implement artifact protocol,
for example:

- explicit `files[]`
- per-file path + content boundaries
- structured verify commands
- explicit truncated / partial markers
- stronger handoff packet for external callers

## Evidence

- run: `devkit/runs/smoke-dev-20260703`
- artifact: `02-implement.artifact.json`
- stage text: `02-implement.md`
