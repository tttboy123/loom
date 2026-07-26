# S3-W4 Mandatory RED

- Baseline: `47b4b50`
- Date: `2026-07-26`
- Contract Review: `PASS`
- Product files changed before RED: none
- Test files added:
  - `internal/supervisor/managed_workspace_test.go`
  - `internal/supervisor/managed_execution_test.go`
  - `internal/runtime/piadapter/execution_adapter_test.go`

All eight frozen markers occur exactly once:

```text
s3_w4_workspace_copy_digest_changes_cleanup
s3_w4_adapter_config_env_binding
s3_w4_bridge_dispatch_ack_result
s3_w4_grant_frame_authorization
s3_w4_cancel_timeout_process_group
s3_w4_terminal_failure_revoke
s3_w4_stale_generation_source_changed
s3_w4_bounds_failure_fuzz_static
```

Command:

```text
go test ./internal/supervisor ./internal/runtime/piadapter -count=1
```

Result: exit `1`, as required.

The new `internal/supervisor` package still had no non-test Go file.
Compilation failed only on the frozen missing S3-W4 product surface, beginning
with:

```text
undefined: AdapterResult
undefined: AdapterRequest
undefined: NewAdapterResult
undefined: AdapterResultInput
undefined: ExecuteInput
undefined: WorkspaceChange
```

`internal/runtime/piadapter` failed only because it imports that still-missing
Supervisor boundary. No product file existed, no accepted package behavior
failed, and no syntax, dependency, or environment failure was used as RED
evidence.

VERDICT: PASS
