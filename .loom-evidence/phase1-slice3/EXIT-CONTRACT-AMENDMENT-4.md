# Slice 3 Exit Contract Amendment 4 — S3-W4 Export Guard Compatibility

- Date: `2026-07-26`
- Baseline: `cd1e594`
- Parent:
  `.loom-evidence/phase1-slice3/EXIT-CONTRACT-AMENDMENT-3.md`
- Trigger: mandatory S3-W5 focused verification

## Finding

S3-W4's accepted static export guard is implemented in
`internal/supervisor/managed_workspace_test.go`. Exit Contract Amendment 3
requires new exported FrameSink and AuthorizedFrame symbols in
`internal/supervisor/managed_execution.go`, but did not reopen the guard's
allowlist. The accepted new exports therefore cannot pass the unchanged S3-W4
compatibility test.

## Bounded amendment

S3-W5 additionally owns exactly:

- `internal/supervisor/managed_workspace_test.go`

The only permitted change is to add the exact exports frozen by S3-W5 Contract
Amendment 2 to the existing static allowlist:

- `ErrAuthorizedFrameObserver`
- `FrameSink`
- `AcceptFrame`
- `AuthorizedFrame`
- `Frame`
- `Tentative`
- `AuthorizedFrameObserver`
- `ObserveAuthorizedFrame`
- `FrameObserver`

No assertion, forbidden import, forbidden capability, file list, timeout, or
other S3-W4 behavior may be weakened or changed.

No product file, WorkItem, scheduler, writer, coordinator, protocol, or client
delivery scope is added.

VERDICT: FROZEN
