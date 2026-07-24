# S1-W3-L2 Frozen Repair Contract

- Parent WorkItem: `S1-W3`
- Lineage: `S1-W3-L2`
- Risk: Strict
- Authorization: explicit human approval on 2026-07-24
- Repair budget: one bounded Candidate
- Corresponds to: `TECH-PLAN.md §5.1; S1-W3 acceptance; fresh Reviewer
  failure 3`

## Owned files

- `internal/evidence/store.go`
- `internal/evidence/store_test.go`
- Evidence platform files only if a compile-safe error definition must stay
  aligned

Do not modify mode, journal, migrations, CLI, docs, Go dependencies, S1-W4,
S1-W5, or Slice 2 behavior.

## Frozen blockers

### Blocker 1: canonical shard rebind

After opening `<root>/artifacts/sha256/<shard>`, the named shard may be renamed
or replaced before the no-replace commit. Committing through the old descriptor
would orphan the digest outside the frozen canonical path.

Required behavior:

- Capture the opened shard device/inode identity.
- Immediately before commit, reopen the named shard relative to the held
  `sha256FD` with `O_DIRECTORY|O_NOFOLLOW` and compare identity.
- A missing, symlinked, or different shard returns a typed
  `ErrShardChanged`, cleans staging, and commits nowhere.
- Keep root revalidation and atomic no-replace rename unchanged.

### Blocker 2: non-regular existing target

A FIFO/device/socket/directory at the digest name must not block the Store or be
hashed as Artifact bytes.

Required behavior:

- Open existing targets descriptor-relatively with `O_NOFOLLOW|O_NONBLOCK`.
- `fstat` before hashing and accept only regular files.
- Non-regular targets return `ErrCorruptArtifact` promptly and remain
  untouched.
- The no-replace `EEXIST` verification path must use the caller context, not
  `context.Background()`.

## Mandatory RED tests

1. `TestPublishRejectsShardRebindImmediatelyPreRename`
   deterministically replaces the opened shard in `beforeRename`; it must
   initially demonstrate commit into the old shard, then GREEN must return
   `ErrShardChanged` with no artifact in old/new/escape paths and no staging.
2. `TestPublishRejectsFIFOExistingTargetWithoutBlocking`
   creates a FIFO at the digest path, runs Publish with a bounded timeout, and
   must initially demonstrate blocking; the RED test must unblock its goroutine
   before returning so cleanup cannot deadlock.
3. `TestPublishEEXISTVerificationHonorsCallerCancellation`
   creates a target in the pre-rename race, cancels the caller context, and
   proves the EEXIST recheck returns `context.Canceled`, not idempotent success.

## Preservation checks

- All existing Evidence tests remain enabled.
- Digest-only identity, private permissions, root rebind protection, lifecycle
  leases, staging cleanup, Darwin/Linux no-replace behavior, and cross-Store
  idempotency must not regress.
- No SQLite metadata, Event, GC, encryption, network, credential, or Slice 2
  behavior.

## Deterministic checks

- RED/focused:
  `go test ./internal/evidence -run
  'TestPublish(RejectsShardRebind|RejectsFIFO|EEXISTVerification)' -count=1`
- Repeated focused race:
  `go test -race ./internal/evidence -run
  'TestPublish(RejectsShardRebind|RejectsFIFO|EEXISTVerification)' -count=50`
- Evidence full: `go test ./internal/evidence -count=1`
- Evidence race: `go test -race ./internal/evidence -count=1`
- Impact: `go test ./... -count=1`
- Repository race: `go test -race ./... -count=1`
- Static analysis: `go vet ./...`
- Formatting/diff: `gofmt`, trailing-whitespace check, `git diff --check`

## Gate

A fresh read-only strict Reviewer must return `VERDICT: PASS` against this
contract and the complete S1-W3 Candidate before S1-W4 may open. Any failure
returns to human direction; no automatic scope expansion is authorized.
