# S2-W20 Frozen WorkItem Contract

- ID: `S2-W20`
- Title: Runtime Discovery Event Writer
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W19 local commit `1b2c486`
- Corresponds to: `TECH-PLAN.md §4, §6, §13.1, §14 Slice 2.2`,
  ADR-0002, ADR-0003, and accepted S2-W2/S2-W14
- Frozen branch/head: `codex/loom-platform-slice2` at `1b2c486`

## Owned files

- `internal/state/runtime_discovery_writer.go`
- `internal/state/runtime_discovery_writer_test.go`
- `.loom-evidence/phase1-slice2/S2-W20/deliverable.md`

The Developer owns only these files. Controller-owned contract, review, status,
and documentation evidence remain outside Developer ownership. S2-W19 and all
earlier accepted files remain unchanged. Any ownership amendment requires a
recorded Controller amendment and fresh contract Reviewer `PASS`.

## Objective

Add the smallest authoritative StateWriter boundary that atomically records one
non-empty accepted S2-W2 Runtime discovery snapshot:

1. revalidate that commit metadata exactly covers every discovered
   RuntimeInstance once;
2. emit one canonical `RuntimeInstanceDiscovered` Event per observation in
   stable snapshot order;
3. bind every payload to the complete source discovery digest and its source
   probe/model/capability facts;
4. append all Events through accepted S2-W14 `AppendBatch`; and
5. accept only an exact immutable appender result before returning a copied,
   digest-bound commit Candidate.

This WorkItem does not run discovery, locate or execute Pi, schedule a daemon,
record empty/absent scans, infer offline/status changes, update projection,
choose a RuntimeProfile, create a Team/Agent/WorkItem/Run/Grant, start a
Runtime Adapter, invoke a model, or activate anything.

## Frozen public boundary

### Appender

The writer reuses the accepted package-level `state.EventBatchAppender` port:

```go
AppendBatch(context.Context, []journal.Event) ([]journal.Event, error)
```

No concrete SQLite dependency is imported by product code.

### Commit input

`RuntimeDiscoveryCommitInput` contains:

- `DiscoveryID`: non-empty correlation identity for this bounded discovery
  observation;
- `EmittedAt`: non-zero timestamp, normalized to UTC; and
- `Events`: one `RuntimeDiscoveryEventInput` for each observation.

Each `RuntimeDiscoveryEventInput` contains:

- `RuntimeInstanceID`: exact observation instance ID;
- `EventID`: non-empty unique Event identity;
- `IdempotencyKey`: non-empty unique message/idempotency identity; and
- `Seq`: positive sequence for the derived RuntimeInstance stream.

The event-input slice is copied. Its length must be `1..32`, exactly equal the
snapshot observation count, and form a bijection over observation instance IDs.
Duplicate/extra/missing instance IDs, duplicate Event IDs, duplicate
idempotency keys, nonpositive sequences, or duplicate derived stream/sequence
pairs fail before the appender is called.

`DiscoveryID`, Event IDs, idempotency keys, Runtime IDs, and sequences are
caller-supplied authoritative metadata; this writer does not allocate IDs or
read current stream state.

### Source validation

`CommitRuntimeDiscoverySnapshot(ctx, appender, snapshot, input)` rejects:

- nil context or nil/typed-nil appender;
- canceled/deadline context before append;
- zero/empty discovery digest;
- empty observation snapshots;
- more than 32 observations;
- an observation with empty SourceProbeID;
- a RuntimeInstance that fails accepted `runtime.NewRuntimeInstance`;
- empty, duplicate, unsorted, or otherwise non-canonical model IDs;
- duplicate RuntimeInstance IDs; and
- any input/snapshot coverage mismatch.

Because `RuntimeDiscoverySnapshot` has private state and public copy accessors,
non-zero snapshots originate from accepted `runtime.DiscoverRuntime`. The writer
still revalidates all accessible observation fields and requires a lowercase
64-character SHA-256 digest. It does not invent or rewrite source facts.

### Canonical Events

Events are built in accepted snapshot observation order, which S2-W2 freezes by
RuntimeInstance ID. For each observation:

- `ID`: matched `EventID`;
- `StreamID`: `"runtime_instance:" + RuntimeInstance.ID`;
- `Seq`: matched positive sequence;
- `IdempotencyKey`: matched key;
- `Type`: exactly `RuntimeInstanceDiscovered`;
- `SchemaVersion`: `1`;
- `EmittedAt`: common normalized UTC time;
- `CorrelationID`: common `DiscoveryID`;
- `CausationID`: empty; and
- `PayloadJSON`: canonical JSON described below.

Each payload contains:

```text
discovery_digest
source_probe_id
instance:
  id
  device_id
  adapter_type
  display_name
  executable_version
  status
  observed_capabilities
  capacity
model_ids
```

Capabilities and model IDs are copied in their accepted canonical order.
Payloads contain no executable path, search path, isolation path, file identity,
file digest, environment, stdout/stderr, credential, Provider secret, prompt,
session, or user Pi state.

### Append and result validation

The writer calls `AppendBatch` exactly once with a deep copy of the complete
Event batch. Appender errors propagate and return a zero Candidate. The writer
requires the returned slice to have the same length and exact immutable Event
content in the same order. Nil, short, long, reordered, envelope-mutated, or
payload-mutated results return `ErrRuntimeDiscoveryCommitResultMismatch` and a
zero Candidate.

An exact retry is delegated to accepted S2-W14 idempotency. Partial-existing,
idempotency, and sequence conflicts remain S2-W14 failures and cannot produce a
successful Candidate.

### Commit Candidate

On exact success, `RuntimeDiscoveryCommitCandidate` is immutable and exposes
copying accessors:

- `Committed() bool`;
- `SourceDiscoveryDigest() string`;
- `Events() []journal.Event`;
- `EventCount() int`; and
- `CommitDigest() string`.

`CommitDigest` is lowercase SHA-256 over a versioned canonical encoding of the
source discovery digest plus every complete immutable committed Event,
including payload bytes. It is stable under identical input/retry and sensitive
to every included source/envelope/payload field.

All failure paths return the zero Candidate. Caller/event input, appender input,
appender result, returned accessors, and source snapshot accessors cannot mutate
each other or the Candidate.

## Frozen typed errors

Sentinels inspectable through `errors.Is`:

- `ErrInvalidRuntimeDiscoveryCommitInput`;
- `ErrInvalidRuntimeDiscoveryCommitSource`;
- `ErrRuntimeDiscoveryCommitResultMismatch`;
- `ErrRuntimeDiscoveryCommitDigestMismatch`; and
- `ErrEmptyRuntimeDiscoveryCommit`.

Appender/Journal and caller context errors remain inspectable. Product errors
must not include payload JSON, executable/model output, paths, environment,
credential-like material, or raw appender result contents.

## Acceptance boundary

1. Only the new `internal/state` writer/test files implement behavior; accepted
   Journal, Runtime, projection, and earlier product files remain unchanged.
2. Invalid context/appender/input/source fails before append and returns the
   zero Candidate.
3. Empty snapshots fail distinctly with `ErrEmptyRuntimeDiscoveryCommit`; they
   do not fabricate an offline observation or call `AppendBatch`.
4. Event metadata exactly and uniquely covers `1..32` snapshot observations.
5. One canonical `RuntimeInstanceDiscovered` Event per observation is built in
   stable instance order with exact stream/type/version/time/correlation.
6. Payloads bind complete normalized RuntimeInstance, SourceProbeID, models,
   capabilities, and the full discovery digest without local paths or secrets.
7. The entire batch is appended atomically once through S2-W14.
8. Only an exact immutable same-order appender result yields `Committed=true`.
9. Exact retry is stable and conflicts/failures produce no successful
   Candidate or partial new Journal state.
10. Candidate/accessor/appender/input mutation isolation and complete digest
    sensitivity are proven.
11. Real SQLite integration proves all-or-none persistence and exact retry.
12. No discovery process, installed Pi, user state, credential, network,
    package manager, daemon, scheduler, projection update, RuntimeProfile
    selection, Agent session, prompt, model call, or activation is added.
13. Existing Slice 1 and S2-W1 through S2-W19 behavior remains green.

## Mandatory RED tests

The Developer adds `internal/state/runtime_discovery_writer_test.go` before
product. The initial focused command must fail only because frozen S2-W20
symbols do not exist.

Required groups:

1. nil/typed-nil appender, nil/canceled/deadline context, invalid commit
   metadata, duplicates, sequence errors, coverage mismatch, empty and
   oversized snapshot behavior, all proving zero appender calls;
2. source revalidation for SourceProbeID, RuntimeInstance, models, duplicate
   instances, and digest form where constructible without changing accepted
   files;
3. exact canonical multi-observation Event envelope/payload/order proof;
4. recording-appender proof of one deep-copied batch and exact returned
   Candidate;
5. appender error plus nil/short/long/reordered/mutated-result rejection;
6. Candidate, accessor, input, appender-batch/result, payload, and source
   mutation isolation;
7. digest stability for identical commits/retries and sensitivity to every
   source/envelope/payload field;
8. accepted Journal real-SQLite exact retry, idempotency/sequence/partial
   conflict, and all-or-none row-count proof;
9. cancellation before append and appender failure leave no successful
   Candidate; and
10. static import/scope review proving no concrete SQL, process, network,
    daemon, scheduler, projection, credential, UI, Team/Work/Run/Grant, or
    activation dependency.

Tests construct snapshots only through deterministic fake S2-W2 RuntimeProbe
implementations. They do not invoke Pi or S2-W18 processes.

## Deterministic checks

- RED:
  `go test ./internal/state -run 'TestCommitRuntimeDiscoverySnapshot|TestRuntimeDiscoveryCommit' -count=1`
- Focused GREEN:
  same command
- Package full:
  `go test ./internal/state ./internal/runtime ./internal/journal -count=1`
- Repeated focused race:
  `go test -race ./internal/state -run 'TestCommitRuntimeDiscoverySnapshot|TestRuntimeDiscoveryCommit' -count=30`
- Impact:
  `go test ./... -count=1`
- Repository race:
  `go test -race ./... -count=1`
- Static analysis:
  `go vet ./...`
- Formatting and diff:
  changed-file `gofmt`, trailing-whitespace check, and `git diff --check`
- Scope:
  verify only frozen Developer-owned files change, accepted files remain
  unchanged, branch is `codex/loom-platform-slice2`, HEAD remains `1b2c486`
  until the authorized atomic commit, and no Pi process is invoked.

## Required evidence

- frozen contract digest and fresh contract review;
- exact RED output/exit;
- focused/package/race-30/repository/repository-race/vet/format/scope outputs;
- exact Event/payload/order/atomicity/retry/conflict/mutation/digest reports;
- proof that paths/secrets/process/network/daemon/projection/activation were
  absent;
- trust-boundary and residual-state analysis;
- fresh independent implementation Reviewer verdict/findings; and
- deliverable ending `VERDICT: PASS` only after all gates pass.

## Trust-boundary analysis

- S2-W2 owns discovery normalization and digest construction; S2-W20 treats the
  snapshot as untrusted Candidate input, revalidates public facts, and binds
  them to Events without becoming a second Runtime catalog authority.
- The caller owns unique Event/idempotency identities and correct next stream
  sequence. S2-W14 remains the concurrency/idempotency/atomicity authority and
  fails closed on conflicts.
- `RuntimeInstanceDiscovered` records an observed candidate fact; it does not
  select a RuntimeProfile, authorize a Run, or activate an executable.
- Empty/absent scans cannot determine whether a previously known instance went
  offline without historical state. They are rejected here rather than
  fabricating `RuntimeInstanceStatusChanged`; later status reconciliation owns
  that boundary.
- The Event payload deliberately excludes local executable/search/isolation
  paths and credentials. Version/model/capability metadata are inventory facts,
  not secrets or execution authority.

## Governance and next gate

- No product/test write until fresh independent contract Reviewer `PASS`.
- Mandatory RED is automatically authorized only after that `PASS`.
- One Developer owns the Candidate lineage; Controller owns checks/evidence;
  fresh implementation Reviewer decides `PASS` or bounded repair.
- Three failed bounded product repairs require `HUMAN_REQUIRED`.
- After all gates and Reviewer `PASS`, one strictly scoped local atomic S2-W20
  commit is authorized.
- Runtime projection, status reconciliation, discovery scheduler, daemon
  entrypoint/config, and live Runtime checks remain later Slice 2 WorkItems.
- Bridge, Runtime Adapter execution, Grants/claims/leases, WorkItem dispatch,
  and Run lifecycle remain Slice 3.
- Push, merge, rebase, reset, force, release, publish, credentials, `.env`,
  dependency installation, external mutation, paid remote work, daemon or real
  Runtime activation, and autonomous execution remain prohibited.
