# S2-EXIT-1 Contract: Local Runtime Observation Daemon Integration

- WorkItem: `S2-EXIT-1`
- Risk: `STRICT`
- Depends on: accepted S2-W1 through S2-W38, especially `39a9e0a`
- Exit authority:
  `.loom-evidence/phase1-slice2/EXIT-CONTRACT.md`
- Exit contract SHA-256:
  `c7639aef81dbcda62b783c2750ed94459333f97318798c5e4989954837ac7614`
- Exit contract review:
  `.loom-evidence/phase1-slice2/EXIT-CONTRACT-REVIEW-1.md` (`PASS`)

This is the sole remaining Slice 2 product WorkItem. Clock, configuration,
state opening, process locking, commit-input metadata, lifecycle, recurrence,
entrypoint, cancellation, restart/recovery, and live verification are one
Candidate. None may be deferred to another WorkItem.

## Outcome

One explicitly configured, foreground, non-auto-starting local daemon:

1. opens one private SQLite Event Journal;
2. binds the accepted isolated Pi metadata probe factory;
3. builds the accepted discovery/status committers and projected observer;
4. performs an immediate metadata observation;
5. repeats observations serially on one concrete clock trigger;
6. rebuilds the same projection before decisions and after successful writes
   through accepted S2-W38;
7. stops cleanly on cancellation;
8. can be restarted against the same Journal and recover from committed facts;
   and
9. never starts a Runtime/Agent session, calls a model, or creates Slice 3
   resources.

## Owned files

Product and tests:

- `internal/app/runtime_daemon.go`
- `internal/app/runtime_daemon_test.go`
- `internal/app/runtime_daemon_lock_unix.go`
- `internal/app/runtime_daemon_lock_unsupported.go`
- `cmd/loomd/main.go`
- `cmd/loomd/run.go`
- `cmd/loomd/run_test.go`

Governance and evidence:

- `.loom-evidence/phase1-slice2/EXIT-CONTRACT.md`
- `.loom-evidence/phase1-slice2/EXIT-CONTRACT-REVIEW-1.md`
- `.loom-evidence/phase1-slice2/S2-EXIT-1/**`
- `.loom-evidence/phase1-slice2/daemon-integration/**`
- `docs/CURRENT.md`
- the Controller-owned S2-EXIT-1 section of `PROGRESS.md`

Existing S2-W1 through S2-W38 product and test files are read-only. Any need to
modify one requires a recorded amendment and fresh contract Review before the
edit. No dependency, schema, migration, ADR, root policy, or accepted
StateWriter change is owned.

## Frozen public application API

`internal/app/runtime_daemon.go` owns:

```go
type LocalRuntimeObservationDaemonConfig struct {
    StatePath         string
    IsolationRoot     string
    RuntimeSearchPaths []string
    ProbeID           string
    RuntimeInstanceID string
    DeviceID          string
    DisplayName       string
    ObservationInterval time.Duration
    ProcessTimeout    time.Duration
    MaxCycles         int
}

type RuntimeObservationDaemonClock interface {
    Now() time.Time
    Wait(context.Context, time.Duration) error
}

type RuntimeObservationIdentitySource interface {
    NextRuntimeObservationIdentity(context.Context) (string, error)
}

type LocalRuntimeObservationFact struct {
    RuntimeInstanceID string
    ExecutableVersion string
    Status            string
    ModelIDs          []string
    DiscoverySequence int64
    StatusSequence    int64
}

type LocalRuntimeObservationDaemonResult struct {
    CompletedCycles int
    DiscoveryEvents int
    StatusEvents    int
    NoWriteCycles   int
    RuntimeFacts    []LocalRuntimeObservationFact
}

func NewSystemRuntimeObservationDaemonClock() RuntimeObservationDaemonClock
func NewCryptographicRuntimeObservationIdentitySource() RuntimeObservationIdentitySource

func NewLocalRuntimeObservationDaemon(
    LocalRuntimeObservationDaemonConfig,
    RuntimeObservationDaemonClock,
    RuntimeObservationIdentitySource,
) (*LocalRuntimeObservationDaemon, error)

func (d *LocalRuntimeObservationDaemon) Run(
    context.Context,
) (LocalRuntimeObservationDaemonResult, error)

func (d *LocalRuntimeObservationDaemon) Close() error
```

Names may change only through a reviewed contract amendment. Implementation
helpers remain unexported.

## Configuration and state boundary

Construction must reject before opening/migrating SQLite or running a process:

- nil/typed-nil clock or identity source;
- non-absolute, unclean, NUL-containing, directory, or symlink state path;
- a state parent that is absent, non-directory, symlinked, or not `0700`;
- an existing state file that is non-regular, symlinked, or not `0600`;
- an isolation root that does not satisfy the accepted Pi adapter `0700`
  binding;
- an empty, relative, invalid, duplicated-after-resolution, or absent Runtime
  search directory list;
- empty probe, Runtime instance, device, or display identity;
- observation interval outside `10ms..24h`;
- process timeout outside the accepted Pi runner bound;
- `MaxCycles < 0` or `MaxCycles > 100000`.

`MaxCycles == 0` means run until cancellation or error. A positive value is a
controlled bounded foreground run, not a different observation path.

After full configuration validation and Pi factory binding:

1. acquire a non-blocking exclusive sibling state lock;
2. open/create the state file without following a final symlink, retain and
   revalidate its file identity, enforce `0600`;
3. open SQLite with a bounded busy timeout and foreign keys;
4. run the accepted migration;
5. build one Journal store and one projection;
6. build concrete discovery/status commit-input providers, accepted prepared
   committers, and one accepted prepared projected observer.

Any construction failure closes every acquired file/DB/lock. The constructor
does not run a probe, start a timer/goroutine, or observe a Runtime.

Only one daemon object/process may own a state path at once. A second
constructor fails closed. Same-object concurrent `Run` also fails closed.

## Concrete clock trigger

The unexported trigger implements accepted `RuntimeObservationTrigger`:

- the first `AwaitRuntimeObservation` returns immediately after context
  validation;
- later calls invoke the exact injected clock
  `Wait(ctx, ObservationInterval)` once;
- the production clock uses a stoppable `time.Timer`, stops/drains it on
  cancellation, and returns the exact context error;
- no goroutine is created by the trigger or daemon;
- tests use a deterministic injected clock and never sleep.

The same clock supplies UTC Event timestamps. Zero/non-UTC values fail before
Journal append.

## Concrete Event metadata

One unexported provider implements both accepted commit-input provider ports.
For each selected write:

- it obtains fresh opaque identities only from the injected identity source;
- identities are bounded non-empty lowercase hexadecimal values in production;
- discovery/status correlation ID, Event ID, and idempotency key are unique;
- event metadata covers exactly the selected source instances/transitions;
- discovery sequence is one greater than the latest projected discovery or
  status sequence for that Runtime, or one for a new Runtime;
- status sequence is exactly the accepted transition previous sequence plus
  one;
- duplicate IDs, overflow, source mismatch, stale projection provenance, zero
  time, identity-source error, and cancellation fail before append;
- no random value, timestamp, path, hostname, credential, or environment value
  enters payload facts.

There is no blind same-input retry. If a downstream error may follow a commit,
the daemon terminates. A later explicit restart rebuilds the Journal before
deciding whether another Event is needed.

## Lifecycle and recurrence

`Run(ctx)`:

1. rejects nil, closed, or concurrently running use;
2. invokes accepted
   `RunProjectionSynchronizedRuntimeObservationOnce(ctx, trigger, observer)`
   sequentially;
3. records a cycle only after S2-W38 success;
4. updates the returned result from accepted candidates and the post-refresh
   projection snapshot;
5. stops with nil after exactly `MaxCycles` successes when bounded;
6. stops on the first trigger, observation, writer, projection, clock, or
   identity error, returning the completed prefix plus an inspectable error;
7. returns the completed prefix plus exact `context.Canceled` or
   `context.DeadlineExceeded` on cancellation;
8. never overlaps observations or retries within a cycle.

Result facts are sorted by Runtime instance ID and deeply copied. They expose
only Runtime ID, version, status, model IDs, and projection sequences.

`Close` is idempotent after `Run` has stopped, releases SQLite/state/lock
handles, and rejects closing an actively running daemon. No background work
survives `Run`.

Restart/recovery proof must construct a new daemon against the same state:

- accepted Journal facts rebuild before the first new decision;
- an unchanged observation writes nothing;
- a changed inventory writes exactly one next-sequence discovery Event;
- a prior process/observation failure leaves accepted facts intact;
- after external fixture repair, a new daemon continues from those facts;
- exact Event order and projection facts survive DB close/reopen.

## `loomd` entry

`cmd/loomd` is a real compiled foreground entry. It:

- uses `signal.NotifyContext` for interrupt and termination;
- accepts explicit flags for every config field;
- supports repeated `--runtime-dir`;
- has no ambient defaults for state path, isolation root, Runtime directories,
  or identities;
- uses only the production clock and cryptographic identity source;
- constructs, runs, and closes one daemon;
- writes one deterministic JSON summary on bounded success or graceful signal
  cancellation;
- maps invalid input, unavailable state/config, active-lock conflict, and
  runtime failure to stable exit classes without printing local paths,
  executable output, credentials, or raw provider errors.

Importing `cmd/loomd` or constructing the daemon never starts it.

## Mandatory RED

Before product implementation, add tests in the owned test files. The focused
RED must fail only because the frozen daemon/config/clock/identity/result/API
and `loomd` run symbols are absent.

Mandatory discriminating test groups:

1. complete config/path/mode/symlink/typed-nil/side-effect-order rejection;
2. immediate-first and exact interval clock trigger behavior;
3. metadata uniqueness, exact sequence derivation, UTC, overflow, cancellation,
   and zero-append failures;
4. constructor cleanup and cross-object state lock exclusion;
5. same-object concurrent Run rejection, serial no-overlap, max-cycle stop,
   cancellation while waiting, and cancellation in-flight;
6. real SQLite discovery → no-write → rediscovery chain using the actual
   accepted factory/runner/writers/projection;
7. close/reopen restart and failure-then-restart recovery;
8. exact safe result facts and mutation isolation;
9. `loomd` flag parsing, repeated Runtime directories, exit mapping, signal
   cancellation, deterministic JSON, and non-disclosure;
10. static imports/calls proving no Agent execution, model call, Provider,
    credentials, Slice 3 resource, autonomous activation, hidden goroutine, or
    accepted-file mutation.

## Verification matrix

After focused GREEN:

```text
go test ./internal/app -run 'Test(LocalRuntimeObservationDaemon|RuntimeObservationDaemon)' -count=1
go test ./cmd/loomd -count=1
go test ./internal/app ./internal/runtime ./internal/runtime/discoveryscan \
  ./internal/runtime/piadapter ./internal/state ./internal/projection \
  ./internal/journal ./cmd/loomd -count=1
go test -race ./internal/app -run 'Test(LocalRuntimeObservationDaemon|RuntimeObservationDaemon)' -count=30
go test -race ./cmd/loomd -count=10
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
gofmt -d <owned Go files>
git diff --check
```

Run `npm audit` / `pip audit` only if this Candidate adds those ecosystems. It
must add no dependency.

## Controlled live canary

After tests pass, build the real `loomd` and existing `loom` binaries into an
isolated temporary output directory. Use:

- a private isolated state parent and Pi isolation root;
- a deterministic executable named exactly `pi` in an explicit Runtime search
  directory;
- no ambient `pi` (the current machine has none discoverable on `PATH`);
- short bounded foreground runs and one signal-canceled recurrence run;
- the same SQLite state across restarts.

Prove in the transcript:

1. first bounded daemon run records discovery sequence 1;
2. second restart rebuilds and writes no Event for unchanged metadata;
3. changed fixture metadata produces exactly the next discovery sequence;
4. a canceled recurrence exits without orphan process or staging residue;
5. a forced metadata-process failure does not corrupt accepted facts and a new
   daemon recovers after fixture repair;
6. final JSON result, Journal rows, and projection facts agree;
7. state parent/isolation directories are `0700`, state/lock files are `0600`;
8. no Agent session, model call, user Pi state, credential, service install, or
   resident process is created.

The canary fixture proves the compiled daemon lifecycle. It must explicitly say
that it does not prove readiness of a real user Pi installation.

## Reviewer gates

1. Fresh independent Contract Reviewer must return `PASS` before RED.
2. Fresh independent Implementation Reviewer audits the entire merged
   Candidate and canary after the complete matrix.
3. After implementation PASS, a different fresh Reviewer audits the whole
   Slice 2 against the Exit Contract and `TECH-PLAN.md`.

Any blocking implementation finding stays within this one lineage. A second
same-class failure requires a fresh read-only problem analyst. Three bounded
product repairs without PASS result in `HUMAN_REQUIRED`.

## Trust boundary and exclusions

This WorkItem owns local metadata observation and Journal/projection lifecycle
only. It does not authorize:

- Agent or Runtime execution;
- Provider/model calls or credentials;
- Bridge, WorkItem, Run, AgentGrant, claim/lease, workspace, or Evidence
  creation;
- persistent service installation, launch-at-login, autonomous activation, or
  production enablement;
- network calls, paid work, dependencies, schema/ADR/root-policy changes;
- push, merge, rebase, reset, release, or publication;
- Slice 3 implementation.

The only permitted live process is the bounded foreground metadata daemon and
its isolated metadata-only child executable used by the controlled canary.
