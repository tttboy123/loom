# S1-W5 Repair 1 Contract

- Lineage: `S1-W5`
- Repair: `1 of 3`
- Trigger: fresh Standard Reviewer `VERDICT: FAIL`
- Owned files remain:
  - `cmd/loom/query.go`
  - `cmd/loom/main_test.go`

## Single product blocker

`openReadOnlyState` interpolates the raw filesystem path into a SQLite file
URI. A real Journal filename containing `?` is statted successfully but parsed
by SQLite as URI query syntax, so the CLI opens a different/nonexistent target
and incorrectly returns state unavailable.

## Required repair

- Convert the state path to an absolute local path and construct the SQLite URI
  with `net/url.URL` path encoding; do not concatenate raw path bytes into a
  URI.
- Preserve `mode=ro` and `query_only(1)`.
- Add a production-path RED test using existing migrated Journal filenames that
  contain URI delimiters and ordinary escaped characters, including `?`, `#`,
  `%`, and a space.
- Each existing readable special-path Journal must return the same deterministic
  empty status JSON and exit `0`.
- Missing and invalid paths must retain frozen exit `3` behavior without path
  leakage.
- Do not change command shape, output, exit codes, projection behavior, or any
  file outside the listed ownership.

## Checks

- Special-path target at `-count=100` and `-race -count=50`
- All frozen S1-W5 focused, package, repository, race, vet, formatting, diff,
  and binary smoke checks remain mandatory.
