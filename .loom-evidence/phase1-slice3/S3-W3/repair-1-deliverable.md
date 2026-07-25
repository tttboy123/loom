# S3-W3 Repair 1 Candidate Deliverable

- Baseline: `5517a06`
- Repair Contract SHA256:
  `b15d742d8b2f8ec312d03acaf79d2051e4840fefcd21fdf29bbafc50399734e5`
- Candidate state: `ready_for_review`
- Date: `2026-07-26`

## Review 1 closure

All four accepted findings are closed in the existing owned boundary:

1. S3-W3 opaque IDs now use the exact accepted S3-W2 language: trimmed,
   control-free valid UTF-8 up to 128 bytes.
2. Operational Grant replay reads one Journal source view, requires accepted
   S3-W2 Run replay, indexes exact historical Run Event identity, stream,
   sequence, and time, and rejects every missing/mismatched reference before
   exposing a snapshot.
3. Stream-head, sequence, idempotency, and partial-batch Journal collisions all
   map to `ErrGrantAuthorityConflict`.
4. Grant and shared projection JSON decoders recursively reject duplicate
   object member names, including identical-value and nested duplicates.

The Run-source validation occurs after the Candidate's Journal read, avoiding a
pre-read validation gap while retaining stack-local, rebuild-only reference
state. No second state authority was introduced.

## Verification

All frozen commands passed:

```text
go test ./internal/authorization ./internal/projection -count=1
go test -race ./internal/authorization ./internal/projection -count=30
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go test ./internal/authorization -run '^$' \
  -fuzz '^FuzzGrantTokenAndReplayNeverPanic$' -fuzztime=5s
gofmt -d <all owned Go files>
git diff --check
```

The fuzz run completed 288,639 executions with 211 total interesting inputs.
Coverage is 85.4% for `internal/authorization` and 82.9% for
`internal/projection`. All ten original and Repair markers occur exactly once.
Import, export, dependency, forbidden-capability, token-surface, owned-scope,
and user-dirty-state checks pass.

The public API and Event schemas are unchanged. No dependency, Provider,
CredentialGrant, Broker, adapter, supervisor, Team DAG, process, network,
daemon, model, or Runtime activation was added.

VERDICT: PASS
