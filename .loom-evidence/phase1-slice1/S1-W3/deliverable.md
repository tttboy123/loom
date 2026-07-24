# S1-W3 Deliverable

## 1. What changed

- Added a Candidate Evidence Artifact Store under `internal/evidence`.
- The Candidate stages private files, computes and validates SHA-256, uses
  descriptor-relative/no-follow filesystem operations, publishes with Darwin
  and Linux atomic no-replace primitives, rehashes existing content, and
  exposes digest-only Artifact identity.
- Added private-permission, digest mismatch, corrupt target, concurrent
  idempotency, context, cleanup, symlink, root rebind, and Close/Publish
  lifecycle tests.
- Promoted the already pinned `golang.org/x/sys v0.28.0` dependency to direct
  use for filesystem syscalls.

## 2. Result and exact evidence

- Initial RED:
  `go test ./internal/evidence -run 'Test(Publish|NewStore)' -count=1`
  failed because the Evidence package did not exist.
- Initial GREEN passed focused, race, full, full race, vet, and diff checks.
- Reviewer failure 1 found post-commit error returns, replace-rename overwrite
  races, and symlink TOCTOU windows.
- Repair 1 added descriptor-relative I/O and no-replace Darwin commit.
- Controller found the second same-class failure: stale path identity after
  root rebind and Close/Publish FD lifetime races. A fresh Problem Analyst
  froze Repair 2.
- Repair 2 changed public identity to digest-only, added root inode
  revalidation, lifecycle leases, explicit cleanup errors, Linux
  `RENAME_NOREPLACE`, cross-Store concurrency, and platform compile surfaces.
- Controller evidence after Repair 2 exited 0:
  focused tests; 50-count lifecycle/cross-Store race tests; Evidence full/race;
  repository full/race; vet; diff, formatting, and whitespace checks. Linux and
  Windows were compile-only probes, not runtime evidence.
- Fresh Reviewer failure 3 found two unresolved blockers:
  - a shard directory can be rebound after it is opened but before commit,
    allowing a success digest to land in an old, non-canonical directory;
  - an existing FIFO/device at the digest name is not rejected as non-regular
    before open/hash and may block indefinitely; the EEXIST verification path
    also discards caller cancellation.

## 3. Affected files and behavior

- `internal/evidence/store.go`
- `internal/evidence/store_test.go`
- `internal/evidence/rename_darwin.go`
- `internal/evidence/rename_linux.go`
- `internal/evidence/rename_linux_test.go`
- `internal/evidence/rename_unsupported.go`
- `internal/evidence/store_windows.go`
- `go.mod`
- `go.sum`
- This is a rejected Candidate. It must not be described as the accepted
  Evidence Artifact Store or used to open projection/CLI dependency gates.

## 4. Remaining risk and unverified boundary

- Canonical shard namespace identity is not proven across post-open rebind.
- Non-regular existing targets can block instead of fail closed.
- Linux no-replace behavior is compile-verified only; no real Linux runtime ran.
- Verification used local Go 1.26.4; the Go 1.22 floor was not executed with an
  installed Go 1.22 toolchain.
- No SQLite Evidence metadata transaction, Event link, GC, daemon, encryption,
  or live Runtime boundary exists.

## 5. Next executable step

`HUMAN_REQUIRED`: decide whether to authorize a new S1-W3 lineage/repair budget.
Do not start S1-W4 or S1-W5. Any authorized repair must first add RED tests for
post-open shard rebind and FIFO/device targets, then prove the minimum fix with
a fresh strict Reviewer.

VERDICT: FAIL
