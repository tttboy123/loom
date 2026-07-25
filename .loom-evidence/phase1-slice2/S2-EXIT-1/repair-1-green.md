# S2-EXIT-1 Repair 1 GREEN

- Baseline: `39a9e0a`
- Date: `2026-07-25`
- Lineage: `S2-EXIT-1`

## Focused proof

The mandatory marker guard and repaired daemon tests passed:

```text
go test ./internal/app -run '^TestS2EXIT1Repair1MandatoryMarkers$' -count=1
ok loom-pi-rebuild/internal/app

go test ./internal/app \
  -run 'Test(LocalRuntimeObservationDaemon|NextRuntimeObservationSequence|S2EXIT1Repair1MandatoryMarkers)' \
  -count=1
ok loom-pi-rebuild/internal/app
```

The direct tests prove:

- duplicate and invalid identities reject before append with real SQLite
  `events=0`;
- a non-UTC clock rejects before append with `events=0`;
- identity-source cancellation returns exact `context.Canceled` with
  `events=0`;
- new, discovery-latest, and status-latest sequence calculation remains exact,
  while both `math.MaxInt64` overflow forms reject;
- the complete frozen configuration matrix rejects without creating a lock or
  new state file and without mutating an existing invalid state path;
- a typed-nil identity source rejects before any state side effect.

## Verification matrix

All passed:

```text
go test ./cmd/loomd -count=1
go test ./internal/app ./internal/runtime ./internal/runtime/discoveryscan \
  ./internal/runtime/piadapter ./internal/state ./internal/projection \
  ./internal/journal ./cmd/loomd -count=1
go test -race ./internal/app \
  -run 'Test(LocalRuntimeObservationDaemon|NextRuntimeObservationSequence|S2EXIT1Repair1MandatoryMarkers)' \
  -count=30
go test -race ./cmd/loomd -count=10
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
gofmt -d <owned Go files>
git diff --check
```

## Controlled compiled canary

Temporary root: `/tmp/loom-s2-exit-repair.7Npync`

The real compiled `cmd/loomd`, real timer path, isolated fixture process, real
SQLite Journal, accepted StateWriters, and projection replay produced:

```json
{"completed_cycles":2,"discovery_events":1,"status_events":0,"no_write_cycles":1,"runtime_facts":[{"runtime_instance_id":"pi-local-1","executable_version":"2.0.0","status":"online","model_ids":["provider/model-b"],"discovery_sequence":1,"status_sequence":0}]}
{"completed_cycles":1,"discovery_events":0,"status_events":0,"no_write_cycles":1,"runtime_facts":[{"runtime_instance_id":"pi-local-1","executable_version":"2.0.0","status":"online","model_ids":["provider/model-b"],"discovery_sequence":1,"status_sequence":0}]}
```

The restart preserved exactly one Event and maximum sequence 1. State and
isolation directories were `0700`; state and lock files were `0600`; the
isolation directory was empty; and no process using the canary `loomd` path
remained.

This deterministic fixture proves the repaired integration remains live. It
does not claim ambient user Pi readiness or activate a resident daemon.

VERDICT: PASS
