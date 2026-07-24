# S1-W4 Repair 1 Contract

- Lineage: `S1-W4`
- Repair: `1 of 3`
- Trigger: Controller pre-review strict audit
- Owned files remain:
  - `internal/projection/projection.go`
  - `internal/projection/projection_test.go`

## Blocker 1: public source injection bypasses committed-Journal authority

The Candidate publicly accepts an arbitrary `Source` on every `Rebuild`.
Production callers can therefore project fabricated, uncommitted
`journal.Event` values.

Required repair:

- Bind the production `Projection` to `*sql.DB`/Journal source at construction.
- Public `Rebuild(ctx)` must have no caller-supplied Event source.
- Keep any injectable source constructor unexported and test-only/internal.
- Add a RED API/behavior test proving public construction reads appended,
  committed Journal rows and that a Journal query failure preserves the prior
  snapshot.
- Serialize concurrent rebuilds so an older, slower candidate cannot overwrite
  a newer completed rebuild; context cancellation while waiting must fail
  without swapping.

## Blocker 2: projected digest does not match accepted Artifact identity

The accepted Evidence Store uses a digest-only identity of exactly 64 lowercase
hexadecimal SHA-256 characters. The Candidate tests use values such as
`sha256:abc123` and implementation accepts any nonempty string, so projected
Evidence cannot be guaranteed to resolve to the accepted Artifact namespace.

Required repair:

- Validate `EvidenceSubmitted.digest` as exactly 64 lowercase hexadecimal
  characters.
- Invalid length, uppercase, non-hex, or prefixed values return the existing
  projection error `ErrInvalidProjectionEvent`; do not modify Evidence errors.
- Replace placeholder test digests with valid deterministic SHA-256 hex values.
- Add focused RED cases for invalid digest forms.

## Preservation

- Preserve the frozen canonical ordering, exact-duplicate, conflict, gap,
  unknown-version, supported-unknown-type, malformed-payload, atomic snapshot,
  and deep-copy policies.
- Do not add projection persistence, migrations, mutation methods, CLI, or any
  file outside the original ownership boundary.

## Checks

- `go test ./internal/projection -run 'Test(Rebuild|Snapshot)' -count=1`
- `go test -race ./internal/projection -run 'Test(Rebuild|Snapshot)' -count=50`
- `go test ./internal/projection -count=1`
- `go test -race ./internal/projection -count=1`
- `go test ./... -count=1`
- `go test -race ./... -count=1`
- `go vet ./...`
- `gofmt` and `git diff --check`
