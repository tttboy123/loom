# S1-W2 Deliverable

## 1. What changed

- Added a pure-Go SQLite dependency pinned to `modernc.org/sqlite v1.35.0`.
- Added the embedded, versioned `migrations/0001_init.sql` authority.
- Added an Event Journal with validation, transactional append, immutable
  content comparison, typed idempotency/sequence conflicts, and ordered replay.
- Added SQLite triggers that reject direct Event `UPDATE` and `DELETE`.
- Added file-backed migration, concurrency, failure, and replay tests.

## 2. Result and exact evidence

- RED:
  `go test ./internal/journal -run 'Test(Append|Migrate|Read)' -count=1`
  failed to compile on missing `Migrate`, `Event`, and `Store`.
- Initial GREEN passed focused, journal race, full, full race, vet, and diff
  checks, but the first strict Reviewer returned FAIL:
  runtime executed an inline migration duplicate and did not pin the connection
  between `PRAGMA foreign_keys` and `BeginTx`.
- Repair 1 embedded the versioned file and pinned `*sql.Conn`, but introduced
  incomplete future tables and a mutable exported migration variable.
- Repair 2 removed those future tables, added a no-future-table regression
  check, and exposed embedded SQL through an immutable accessor.
- After Repair 2, the Controller command union exited 0:
  focused journal tests, `go test -race ./internal/journal -count=1`,
  `go test ./...`, `go test -race ./...`, `go vet ./...`,
  `git diff --check`, and Go formatting cleanliness.
- A fresh strict Reviewer reran journal race and full tests, reported no
  findings, and returned PASS.

## 3. Affected files and behavior

- `go.mod`
- `go.sum`
- `internal/journal/store.go`
- `internal/journal/migration.go`
- `internal/journal/journal_test.go`
- `migrations/0001_init.sql`
- `migrations/embed.go`
- SQLite is the only accepted Event fact authority. Repeated identical
  idempotency submissions return the original fact; different content with the
  same key fails closed; conflicting stream sequence does not append.

## 4. Remaining risk and unverified boundary

- The future daemon opener must centralize the same per-connection modernc DSN
  pragmas. S1-W2 proves file-backed behavior and the pinned migration
  connection, but no daemon or production DB lifecycle exists yet.
- There is no projection update or Evidence metadata transaction in S1-W2;
  those remain later Slice 1 WorkItems.
- Candidate files remain untracked because staging/commit was not authorized.

## 5. Next executable step

Freeze S1-W3 and implement the daemon-owned immutable Evidence Artifact Store
with digest validation, atomic publication, permissions, idempotency, and
failure cleanup.

VERDICT: PASS
