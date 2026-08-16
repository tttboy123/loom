# P2D-W2C Managed Source Workspace Snapshot

Status: `CURRENT / SOURCE VERIFIED / BUILD 47 PACKAGED / NOT INSTALLED`

Date: 2026-08-12

Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

## User-visible outcome

Each built-in Mission Role Context Capsule now carries the same path-free,
content-addressed observation of the managed source baseline. A retry keeps the
same Capsule digest. If the source tree changes after compilation but before an
Attempt reaches its adapter, dispatch fails closed before any Harness or
Provider call.

This is an observed workspace fact, not a Goal, confirmed constraint, accepted
decision, or model-generated authority.

## Bound facts

The `workspace-snapshot` item contains only:

- schema version and `managed_source_baseline` kind;
- SHA-256 tree digest;
- entry, file, and directory counts;
- total regular-file bytes.

It is classified as `observed`, scoped `team_shared`, and attributed to a
managed-source observation. It contains no absolute or relative path, file
name, file content, Prompt body, credential, Provider response, or hidden
reasoning. Top-level `.git` metadata is excluded by the same descriptor-rooted
managed-tree scanner used to prepare execution workspaces.

Exact Artifact revisions remain separate authority-bound Capsule items. Loom
does not infer Goal, confirmed-constraint, accepted-decision, test-state, or
prior-model-output items when no corresponding authoritative source exists.

## Runtime gate

The compiler observes the source once and binds the digest to every primary and
verifier execution. The supervisor compares that expected digest with the tree
it actually copied. A mismatch returns the existing `source_changed` terminal
reason before adapter invocation. Asset-materialized executions retain their
existing immutable materialization digest/revision chain instead of pretending
the original source digest describes the materialized root.

## Verification

- focused source snapshot, Capsule, source-drift, and static-boundary tests pass;
- affected `internal/app` and `internal/supervisor` packages pass;
- affected race tests pass;
- the serialized repository Go suite, repository vet, and complete Swift suite
  passed before the final `.git` exclusion test-only reinforcement;
- the `.git` exclusion reinforcement passes the focused matrix and diff check.

The increment is packaged in the unlaunched, uninstalled v0.5.2 build 47
Candidate. Complete repository gates, release construction, arm64,
deep-signature, permissions, no-symlink, contract-string, and ZIP
byte-equivalence checks pass. The frozen manifest is
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build47-candidate-2026-08-12/BUILD-MANIFEST.md`.

This increment does not close P2D-W2C, CV6, installed mixed-Team execution, or
real Provider acceptance.
