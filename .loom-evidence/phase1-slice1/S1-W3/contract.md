# S1-W3 Frozen WorkItem Contract

- ID: `S1-W3`
- Title: Immutable Evidence Artifact Store
- Risk: Strict
- Depends on: S1-W2 PASS
- Corresponds to: `TECH-PLAN.md §14 Slice 1.4, §15.21; ADR-0002`

## Owned files

- `internal/evidence/*.go`
- `internal/evidence/*_test.go`
- `go.mod` and `go.sum` only if `golang.org/x/sys/unix` must be promoted from
  the already pinned indirect dependency for descriptor-relative no-follow and
  no-replace filesystem operations

No Evidence failure/status/digest definitions existed before this WorkItem.
Introduce evidence-local typed errors; do not modify mode or journal enums and
do not modify the SQLite migration.

## Frozen artifact contract

- Digest algorithm: SHA-256.
- Canonical digest representation: 64 lowercase hexadecimal characters.
- Final path: `<root>/artifacts/sha256/<first-two-hex>/<digest>`.
- Daemon-owned state directories: `0700`.
- Staging and committed artifact files: `0600`.
- Published artifact identity is immutable content, not a caller-supplied name.

## Acceptance boundary

1. A Store validates/creates its daemon-owned root without following a
   caller-provided root symlink and enforces private directory permissions.
2. Publication writes a private staging file, streams bytes while calculating
   SHA-256, flushes the file, validates the expected digest, and only then
   atomically renames within the same state filesystem.
3. Digest mismatch returns a typed error and leaves no committed artifact.
4. Repeated or concurrent publication of identical content is idempotent and
   resolves to the same immutable artifact.
5. If a target digest path already exists, the Store verifies its bytes before
   treating publication as idempotent; corrupt existing content fails closed.
6. Read/source/flush/rename/context failures leave no file that can be mistaken
   for a committed artifact and clean the owned staging file.
7. Final artifact and state permissions are exactly `0600` and `0700`,
   independent of a permissive process umask.
8. The Store does not append Evidence metadata to SQLite, emit Events, perform
   GC, encrypt content, access credentials, or implement Slice 2.

## Required tests

- Known bytes publish to the canonical digest path and preserve exact bytes.
- Digest mismatch returns the frozen typed error with no final artifact or
  staging residue.
- Same content repeated and concurrently published returns one canonical
  artifact.
- Existing target with corrupt bytes fails closed.
- Reader failure and injected atomic-publish failure clean staging state.
- Root symlink is rejected.
- Root/shard/staging directories are `0700`; staged/final files are `0600`.
- Context cancellation before/during publication fails without commit.

## Deterministic checks

- RED: `go test ./internal/evidence -run 'Test(Publish|NewStore)' -count=1`
- Focused GREEN:
  `go test ./internal/evidence -run 'Test(Publish|NewStore)' -count=1`
- Strict race: `go test -race ./internal/evidence -count=1`
- Impact: `go test ./...`
- Repository race: `go test -race ./...`
- Static analysis: `go vet ./...`
- Diff hygiene: `git diff --check`

## Trust-boundary analysis

- Only a successfully renamed digest path is a committed Artifact.
- A caller-provided expected digest is an assertion to validate, never
  authority to choose arbitrary paths.
- Content-address paths are derived only from validated lowercase digest bytes.
- Existing content is rehashed before idempotent success.
- Staging files are never returned as committed Evidence.

## Governance

One Developer writer owns only `internal/evidence`. The Controller runs
deterministic checks. A fresh read-only Reviewer must return PASS before S1-W4
opens. No commit, push, merge, release, activation, credential change, paid
remote work, FastContext installation, migration change, or Slice 2 behavior.

## Repair lineage

- Repair 1: the first Reviewer rejected post-rename error returns, replace-style
  rename races, and `Lstat`-then-use symlink windows. Preserve rename as the
  commit point: all fallible validation occurs first; an atomic no-replace
  publication must never overwrite an existing digest path; descriptor-relative
  no-follow traversal must keep Store I/O inside the opened daemon root.
- Problem-analysis gate: Controller inspection of Repair 1 found a second
  filesystem-boundary failure. A swapped root can cause a safe write to the
  held inode but an unsafe/stale returned `Artifact.Path`, and `Close` can race
  active `Publish` calls by closing/reusing directory descriptors. A fresh
  read-only Problem Analyst must freeze the minimum Repair 2 before editing.
- Repair 2 (Problem Analyst design): make `Artifact.Digest` the only public
  identity and keep canonical layout Store-private; reject root namespace
  rebinding before staging and immediately before commit; hold a lifecycle read
  lease across digest waiting, publication, and cleanup while `Close` takes the
  write lease; capture leased FDs for cleanup; surface pre-commit cleanup
  failures; retain no fallible post-commit work; implement Linux
  `RENAME_NOREPLACE` alongside Darwin `RENAME_EXCL`.
