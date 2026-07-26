# S3-W4 Implementation Repair 1 RED

- Date: `2026-07-26`
- Trigger:
  `.loom-evidence/phase1-slice3/S3-W4/implementation-review-1.md`

## Raw-token source path RED

```text
go test ./internal/supervisor \
  -run '^TestSupervisorTerminalFailureAlwaysRevokes/raw_grant_in_source_path_is_rejected_before_adapter$' \
  -count=1
```

Exited `1`: the old implementation copied a source file whose relative path
contained the complete raw Grant, called the adapter, and failed later as
`runtime_process_failed` instead of rejecting the managed workspace.

## Raw-token workspace path RED

```text
go test ./internal/supervisor \
  -run '^TestSupervisorTerminalFailureAlwaysRevokes/raw_grant_in_workspace_or_stderr_is_never_returned/workspace-path$' \
  -count=1
```

Exited `1`: the old implementation returned `succeeded`,
`ready_for_review`, and one `WorkspaceChange` whose `Path()` contained the
complete raw Grant.

## Fuzz harness RED

Fresh independent Review reproduced the exact required fuzz command failing on
Darwin with `illegal byte sequence` at fixture creation before
`prepareManagedWorkspace` ran. This is the frozen P2 RED: a platform-rejected
fuzz filename was incorrectly treated as a product failure.

VERDICT: RED
