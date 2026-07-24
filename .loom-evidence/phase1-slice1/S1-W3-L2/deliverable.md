# S1-W3-L2 Deliverable

## 1. What changed

- Added typed `ErrShardChanged` handling and captured the opened shard
  device/inode identity.
- Reopened and compared the canonical shard relative to the held `sha256FD`
  immediately before atomic no-replace publication.
- Changed existing-target verification to use
  `O_NOFOLLOW|O_NONBLOCK`, reject non-regular targets before hashing, and
  preserve caller cancellation in the `EEXIST` verification path.
- Added deterministic regressions for shard rebind, FIFO and Unix socket
  targets, and canceled `EEXIST` verification.

## 2. Result and exact evidence

- Mandatory RED first demonstrated all three defects:
  - shard rebind returned success instead of `ErrShardChanged`;
  - FIFO verification blocked until the test force-unblocked it;
  - canceled `EEXIST` verification returned success.
- A repair review then exposed an existing Unix socket returning raw
  `EOPNOTSUPP`; the added socket regression failed before the minimum
  descriptor-relative classification repair.
- Controller verification exited 0:
  - focused blocker tests: `ok loom-pi-rebuild/internal/evidence 0.315s`;
  - focused race at `-count=50`:
    `ok loom-pi-rebuild/internal/evidence 2.444s`;
  - Evidence full/race: `0.467s` / `1.503s`;
  - repository full/race: all packages passed;
  - `go vet ./...`, `git diff --check`, and `gofmt` cleanliness passed;
  - Linux arm64 and Windows amd64 compile-only probes passed.
- A new Controller-owned strict Reviewer independently reran focused, race,
  full-repository, vet, diff, and cross-platform compile probes and returned
  `VERDICT: PASS` with no findings.

## 3. Affected files and behavior

- `internal/evidence/store.go`
- `internal/evidence/store_test.go`
- `internal/evidence/store_windows.go`
- The Evidence Artifact Store now fails closed when the canonical shard binding
  changes immediately before publication.
- Existing FIFO, device, socket, directory, and symlink targets are not treated
  as Artifact bytes; supported non-regular cases return typed
  `ErrCorruptArtifact` without blocking or replacing the target.
- The original `S1-W3` failed deliverable remains historical evidence and was
  not overwritten.

## 4. Remaining risk and unverified boundary

- Shard validation is an immediate pre-commit identity check, not a
  kernel-atomic binding assertion across the final rename syscall.
- Linux and Windows were compile-checked only; no non-Darwin runtime was
  executed.
- Verification used local Go 1.26.4; the Go 1.22 floor was not executed with a
  separate installed toolchain.
- No Evidence metadata transaction, GC, encryption, daemon, or live Runtime
  boundary was added.

## 5. Next executable step

Open S1-W4 only: freeze the strict projection WorkItem contract, add the minimum
RED replay/projection tests, and require focused GREEN, impact GREEN, and a
fresh Reviewer verdict before S1-W5.

VERDICT: PASS
