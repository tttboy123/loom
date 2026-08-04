# Phase 3A Entry Audit

Date: 2026-08-03

Status: `HUMAN_REQUIRED — BOUNDED ENTRY AMENDMENT REVIEW REQUIRED`

Repository identity:

- physical cwd and Git top-level:
  `/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild`;
- branch: `codex/loom-platform-slice2`;
- HEAD: `6d380233b5b89309a1a7ce3919aa611654e0f4ee`;
- `docs/CURRENT.md`: Phase 2A and P2B-W1 accepted at that commit.

The code-graph MCP was attempted first as required but returned
`Transport closed`; the audit therefore used read-only local source search.
No product code, process, Provider, network, staging or commit action occurred.

## Dirty boundary

The pre-existing worktree has 52 unrelated modified/untracked entries. The
following groups remain user-owned and excluded from Phase 3A:

- `.loom-evidence/phase1-final-live-gate/**`;
- `AGENTS.md`, `PROGRESS.md`, `README.md`;
- `internal/projection/team_execution_test.go`;
- `.codex/**`;
- `.loom-drafts/**`;
- `.loom-evidence/phase1-slice3/**`;
- `.loom-evidence/plan-amendments/2026-08-01-*`;
- `apps/macos/.build/**`.

The new Gate 0 evidence is isolated under `.loom-evidence/phase3a/**`.

## Prerequisite matrix

| Prerequisite | Status | Current evidence | Entry consequence |
|---|---|---|---|
| Phase 2A/2B acceptance | DONE | `docs/CURRENT.md`; commit `6d380233` | No reconstruction permitted |
| Runtime capability/compatibility reads | PARTIAL | `internal/runtime/catalog.go` validates generic required/observed capability sets; discovery and GlobalReadView expose them | No accepted `skill_materialization` capability or conformance level |
| Credential/Provider boundary | DONE | `internal/credentials/credential_broker.go` and Keychain boundary keep secrets outside Journal and assets | Phase 3A must consume references only |
| Agent/Team/WorkPackage binding | PARTIAL | immutable Agent/Team/WorkPackage digests exist; saved Team configuration records exact Skill ID/revision/digest | exact Skill binding is not carried through executable plan or Run |
| exact revision/digest in Run lineage | MISSING | `ExecutionNodeInput`, `TeamDispatchInput`, `TeamNodeSemanticBinding`, `RunRecord` and Run Events contain no Skill revision binding | Contract cannot honestly claim immutable Run assets yet |
| Artifact/Evidence/Journal/CAS | DONE | content-addressed Evidence Store, `ReadStreamSet`, `AppendBatchIfStreamHeads`, accepted Evidence/terminal authority | Reuse; no second writer/store |
| Projection/GlobalReadView | DONE foundation | atomic rebuild/versioned immutable typed views and old-view preservation exist | Asset projections/accessors are new P3A-owned additions |
| accepted terminal Run/Evidence | DONE | Team acceptance authority binds source/verifier receipts and terminal facts | Suitable promotion gate foundation |
| Runtime-native Skill materialization | MISSING | Pi discovery advertises only metadata model/version; controlled execution uses `--no-skills` in metadata/process/RPC paths | Requires reviewed capability and private per-Run materialization contract |
| archive/restore/rollback CAS | PARTIAL | Team archive/restore and multi-stream CAS patterns exist | Asset-specific activation/rollback facts do not exist yet |
| journey correlation/evidence substrate | MISSING | no `journey_id` schema or structured cross-client evidence harness found | New mandatory Exit Gate cannot currently be executed |
| real GUI/TUI shared-root E2E | MISSING | production native/TUI/IPC surfaces exist, but no final PTY+native shared-root journey harness exists | Unit/direct-service/visual-only evidence is insufficient |

## Critical findings

### 1. Exact Skill data stops at saved Team configuration

`TeamDefinitionSaved` configuration already validates exact Skill ID, revision
and SHA-256 digest. That proves an accepted Phase 2 binding entry. However:

- `internal/teams/execution_plan.go` carries only node, agent, Runtime, role,
  dependency and attempt data;
- `internal/work/team_execution_authority.go` carries semantic verification and
  recovery bindings, but no asset revision set;
- `internal/work/run_authority.go` Run records and Events carry no Skill
  revision/digest.

Therefore a Run cannot yet prove which asset bytes it executed.

### 2. Runtime materialization is not an accepted capability

The Runtime catalog can enforce arbitrary capabilities, but the installed Pi
probe advertises only `pi.metadata.models` and `pi.metadata.version`. Pi
metadata, process and RPC paths explicitly use `--no-skills`. There is no
reviewed per-Run private Skill root, exact materialization manifest, conflict
policy, cleanup fact or Runtime conformance signal.

### 3. The new Cross-client Exit Gate has no execution substrate

Production GUI, TUI and local IPC exist, but no `journey_id` correlation field,
Daemon structured journey log, PTY automation evidence bundle or shared-root
native/TUI scenario runner exists. The previously accepted visual-only evidence
cannot satisfy the new rule and will not be reused as Phase 3A completion proof.

## Gate decision

The core authority foundation is sufficient to design Phase 3A, but the exact
Run asset lineage, Runtime materialization capability and mandatory cross-client
journey substrate are critical entry gaps under the active Goal. Freezing the
original P3A-W1 contract without explicitly reopening these shared boundaries
would be misleading.

Per Gate 0, product implementation stops. The bounded proposal in
`BOUNDED-ENTRY-AMENDMENT-PROPOSAL.md` must receive explicit authorization and
independent review before ADR/Exit Contract freeze.

