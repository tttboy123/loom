# Loom Platform Development Guide

## Purpose

Loom is a local-first platform for composing and governing Agent teams. Coding is
the first reference workflow, not the product boundary.

Natural language may propose an Agent or Team. Only versioned contracts, policy,
grants, evidence, and authoritative state transitions may permit execution.

## Load only what the task needs

Codex already receives the applicable `AGENTS.md` chain. Do not reopen it merely
to start a task.

For development, architecture, or status work, use
[`docs/CURRENT.md`](docs/CURRENT.md) as the small current-state snapshot. Before
modifying a subtree, open its closest scoped `AGENTS.md` only if that file was not
already supplied. Read only the contract, ADR, architecture section, or tests
directly relevant to the requested change.

Do not preload all plans, progress logs, architecture documents, or archives.
Current code and live verification outrank prose. Accepted ADRs define durable
decisions; [`TECH-PLAN.md`](TECH-PLAN.md) defines the Phase 1 contract.

## Invariants

1. Ordinary conversation never creates a Team. Agent execution requires an
   explicit Agent trigger.
2. Model-generated or expanded Team drafts require user confirmation before
   instances or Runs are created.
3. Agent, Harness, Sidecar, and model output is a Proposal or Candidate, never
   execution authority.
4. Side effects require the applicable policy, contract, grant, and evidence
   lifecycle.
5. Event Journal facts are append-only. Read models are rebuildable projections,
   not a second state authority.
6. An executor may submit `ready_for_review`; it cannot mark its own WorkItem
   `done`.
7. Secrets never enter source control, prompts, logs, evidence, or Agent
   definitions.
8. Root policy, validators, credential boundaries, and authoritative state
   writers cannot be modified by online self-evolution.

## Development behavior

- Preserve unrelated and pre-existing worktree changes.
- Use one writer for each branch or Candidate lineage. Read-only exploration,
  documentation research, and review may run in parallel.
- Add or update tests before behavior changes. Keep changes scoped to one
  acceptance boundary.
- Use the commands supported by the current repository. Do not claim a build,
  CLI, daemon, runtime, or demo exists until code and live verification prove it.
- Use `CURRENT`, `PARTIAL`, `TARGET`, and `EXPERIMENTAL` precisely.
- Do not push, merge, publish, change credentials, run paid remote work, or
  activate autonomous execution without explicit user approval.

The detailed workflow, risk levels, team roles, and Phase 1 verification commands
are in [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md). Documentation-specific rules
are in [`docs/AGENTS.md`](docs/AGENTS.md).

## Keep this file stable

Do not place milestone percentages, active WorkItem IDs, test counts, provider
availability, or temporary incident procedures here. Update `docs/CURRENT.md`,
the relevant contract, or an on-demand runbook instead.
