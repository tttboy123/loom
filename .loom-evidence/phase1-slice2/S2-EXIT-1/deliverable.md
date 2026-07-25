# S2-EXIT-1 Candidate Deliverable

- WorkItem: `S2-EXIT-1`
- Title: Local Runtime Observation Daemon Integration
- Risk: `STRICT`
- Candidate state: `ACCEPTED_PENDING_LOCAL_COMMIT`
- Baseline: `39a9e0a`
- Contract SHA-256:
  `e50fa0a0899b07146bd793f43dd590248a453a527cd8f3a3887933053d4710d0`
- Contract Review 1: `PASS`

## Frozen exit evidence

```text
c7639aef81dbcda62b783c2750ed94459333f97318798c5e4989954837ac7614  EXIT-CONTRACT.md
cb22ef35e6c46cb051e73ff22de9a83424573288b0f571f08d4cbc0e9f79f742  EXIT-CONTRACT-REVIEW-1.md
```

These files remained unchanged during product implementation.

## RED and Candidate

The focused test-only RED failed solely on the missing frozen daemon,
configuration, clock, identity, result, and `loomd` symbols.

The Candidate adds validated explicit configuration; a non-blocking per-state
process lock; private `0600` SQLite state; production timer and cryptographic
identity sources; exact discovery/status commit metadata; one serial S2-W38
recurrence owner; bounded run, cancellation, close, restart, and recovery; a
signal-aware foreground `loomd`; and deterministic safe JSON summaries.

It adds no dependency, migration, Event type, ADR, accepted-file modification,
goroutine, Provider/model call, credential access, Agent/Runtime execution,
Slice 3 resource, or service activation.

Product/test SHA-256:

```text
cbbd1d7bcd953db2a24e1b200464ac9c7a54922ae3198ca38f39873c059b4ee9  internal/app/runtime_daemon.go
f5b76c1c697743cae3f7ce359d2332548eeec1c0a1f2b25d1b71a92e00fc595b  internal/app/runtime_daemon_test.go
6f5922208111411a267c95e2bb715b475b7d89fbfb77e5bf4fd52fa697970e97  internal/app/runtime_daemon_lock_unix.go
a3534d7ff6210af910fdbef3e91c0bde4c826388c517eec26616dadf76e3bca2  internal/app/runtime_daemon_lock_unsupported.go
6e5149e8893f79b1736252cbaf4f52abd16ef64b497a646d580405b29c8327d5  cmd/loomd/main.go
70debfa3a068ee9a25dda86370d09cd0e62919aec455d7158bf5eeb01eaa2cf1  cmd/loomd/run.go
5dc014a7b1105074dc9fa80571dfb0ef2e4d3843075deda8abfde563ecd85c68  cmd/loomd/run_test.go
```

## Direct proof

Tests cover the complete frozen configuration rejection matrix, typed-nil
clock and identity sources, identity uniqueness/format/cancellation, non-UTC
time, next-sequence overflow, and zero-append/zero-side-effect ordering. They
also cover first-immediate and later clock waiting, state-lock exclusion,
discovery/no-write/rediscovery/status sequence paths, concurrent Run and active
Close rejection, process failure followed by restart recovery, real SQLite
reopen/replay, mutation isolation, safe command behavior, and the static
no-goroutine/no-Slice-3 boundary.

## Verification

All passed:

```text
go test ./internal/app \
  -run 'Test(LocalRuntimeObservationDaemon|NextRuntimeObservationSequence|S2EXIT1Repair1MandatoryMarkers)' \
  -count=1
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

No npm/pip ecosystem or dependency was added, so npm/pip audit is not
applicable.

## Controlled live canary

`daemon-integration/live-canary.md` records `PASS` for discovery sequence 1,
unchanged restart, rediscovery sequence 2, non-corrupting process failure,
repaired restart, real-timer signal cancellation, private permissions, empty
residue, and no orphan.

The machine had no ambient `pi`. The deterministic Pi-compatible fixture proves
daemon lifecycle only and does not claim user Pi readiness.

## Current gate

Implementation Review 1 returned `FAIL` only for missing direct metadata and
complete configuration rejection proof. Repair 1 remained in the same lineage;
its contract and fresh Contract Review passed, mandatory RED was captured, the
bounded repair is GREEN, and a fresh compiled canary passed.

Fresh independent Repair 1 Implementation Review returned `PASS` with no
blocking findings. S2-EXIT-1 is accepted pending its fresh pre-commit matrix,
exact staged-scope audit, and one authorized local atomic commit. Slice 2 still
requires a fresh whole-Slice Reviewer after that commit.

VERDICT: PASS
