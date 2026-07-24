# S1-W2 Frozen WorkItem Contract

- ID: `S1-W2`
- Title: Append-only SQLite Event Journal
- Risk: Strict
- Depends on: S1-W1 PASS
- Corresponds to: `TECH-PLAN.md §14 Slice 1.3, §5 Schema, §6 Journal;
  TECH-PLAN.md §15.16; ADR-0002`

## Owned files

- `internal/journal/*.go`
- `internal/journal/*_test.go`
- `migrations/0001_init.sql`
- `migrations/embed.go` only as the minimal `go:embed` bridge that makes
  `0001_init.sql` the runtime migration authority
- `go.mod` and `go.sum` only for the required SQLite driver

The dependency metadata amendment is necessary because Go's standard library
does not provide a SQLite driver. Pin `modernc.org/sqlite v1.35.0`, whose module
still targets Go 1.21 and therefore preserves the Phase 1 Go 1.22 floor. Do not
add a framework or CGO dependency.

## Frozen Event contract

An Event has a stable ID, stream ID, positive stream sequence, non-empty
idempotency key, non-empty type, positive schema version, UTC timestamp,
optional correlation/causation IDs, and immutable JSON payload.

No product Event/failure/status definitions existed before this WorkItem.
Introduce journal-local definitions; do not modify the mode enums.

## Acceptance boundary

1. Apply the versioned `0001_init.sql` migration transactionally with foreign
   keys enabled.
2. Append each Event inside a SQLite transaction.
3. A repeated idempotency key for the same logical Event returns the original
   fact and creates exactly one physical row.
4. Reusing an idempotency key for different immutable content fails closed with
   a typed conflict error.
5. Existing Event rows reject SQL `UPDATE` and `DELETE`, not merely through API
   omission.
6. Concurrent duplicate submissions produce one fact and deterministic
   duplicate results; concurrent stream-sequence conflicts do not create
   partial facts.
7. Invalid Events, unsupported versions, migration failure, closed/unwritable
   databases, and SQLite write errors return errors and never fall back to an
   in-memory authority.
8. Committed Events can be read in deterministic stream sequence order for
   later projection replay.
9. Use file-backed temp databases in persistence/concurrency tests. Configure
   connection PRAGMAs per connection through the modernc DSN:
   `foreign_keys(1)`, `busy_timeout(5000)`, and `journal_mode(WAL)`.
10. Do not add projection updates, Evidence metadata, daemon behavior, CLI
    commands, or Slice 2 contracts.

## Required tests

- Versioned migration creates the expected append-only schema and can be
  applied idempotently.
- Event validation covers required IDs/type/version/time/key/payload.
- First append persists all immutable fields.
- Same idempotency key + same content is idempotent.
- Same idempotency key + different content returns the frozen conflict error.
- Direct SQL update and delete are rejected by database enforcement.
- Concurrent duplicate append yields one row.
- Conflicting `(stream_id, seq)` fails without an extra row.
- Closed database/write failure returns an error and does not invent success.
- Replay read order is deterministic.

## Deterministic checks

- RED: `go test ./internal/journal -run 'Test(Append|Migrate|Read)' -count=1`
- Focused GREEN:
  `go test ./internal/journal -run 'Test(Append|Migrate|Read)' -count=1`
- Strict concurrency: `go test -race ./internal/journal -count=1`
- Impact: `go test ./...`
- Repository race: `go test -race ./...`
- Static analysis: `go vet ./...`
- Diff hygiene: `git diff --check`

## Trust-boundary analysis

- SQLite Event Journal is the only fact authority in this WorkItem.
- There is no in-memory success fallback.
- Idempotency equivalence compares persisted immutable content, not only the
  key.
- Database constraints/triggers defend append-only behavior against accidental
  direct SQL mutation.
- Failed transactions must leave no accepted partial fact.

## Governance

One Developer writer owns the Candidate. The Controller runs deterministic
checks. A fresh read-only Reviewer must return PASS before S1-W3 opens. No
commit, push, merge, release, activation, credential change, paid remote work,
FastContext installation, or Slice 2 implementation.

## Repair lineage

- Repair 1: a fresh Reviewer rejected the inline duplicate migration SQL and
  the unpinned `PRAGMA`/`BeginTx` connection sequence. The acceptance boundary
  is unchanged. The minimal ownership amendment is `migrations/embed.go` so
  `migrations/0001_init.sql` becomes the single executable source.
- Repair 2: remove incomplete future `team_instances`, `work_items`, and
  `evidence` tables introduced only to exercise foreign keys, and expose the
  embedded migration through a read-only accessor rather than a mutable
  exported variable. The accepted S1-W2 scope remains Event Journal only.
