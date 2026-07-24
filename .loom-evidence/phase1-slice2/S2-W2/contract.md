# S2-W2 Frozen WorkItem Contract

- ID: `S2-W2`
- Title: Deterministic Runtime Discovery Coordination
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W1 local commit `954416a`
- Corresponds to: `TECH-PLAN.md §3.3, §4, §13.1, §14 Slice 2.2, and
  §15.8-§15.9`
- Frozen branch/head: `codex/loom-platform-slice2` at `954416a`

## Owned files

- `internal/runtime/discovery.go`
- `internal/runtime/discovery_test.go`
- `.loom-evidence/phase1-slice2/S2-W2/deliverable.md`

The Developer owns only these files. Controller-owned status, review, and
contract evidence remain outside Developer ownership. Any product-file
ownership amendment requires a recorded Controller amendment and a fresh
contract Reviewer PASS before the additional path is written.

## Objective

Create the smallest deterministic coordination boundary that a later single
Loom daemon can use to discover local Runtime candidates:

1. define a read-only `RuntimeProbe` port whose implementations can later
   observe a particular local Runtime family;
2. coordinate a bounded set of injected probes without importing or choosing a
   concrete CLI, Provider SDK, process runner, network client, or persistence
   adapter;
3. validate and canonicalize the reported RuntimeInstance, executable version,
   model IDs, and capabilities; and
4. return an immutable, digest-addressed discovery snapshot that is Candidate
   input for a later Team Draft catalog.

This WorkItem establishes discovery coordination and snapshot semantics only.
It does not claim that automatic local discovery is fully delivered. Concrete
OS/CLI probes, daemon scheduling, Journal events, liveness refresh, and Team
Draft catalog composition require separately frozen later WorkItems.

## Frozen discovery boundary

### RuntimeProbe port

A `RuntimeProbe` has a stable, non-empty probe ID and a context-aware observation
operation. The operation returns zero or more `RuntimeObservation` Candidates.

- The coordinator accepts probes only through the interface.
- The coordinator does not know executable names, filesystem locations,
  Provider endpoints, or model-list commands.
- Duplicate probe IDs fail closed before any probe is invoked.
- Nil probes and empty probe IDs fail closed before any probe is invoked.
- Probe invocation order is deterministic by probe ID and independent of input
  ordering.
- Context cancellation stops further invocation and returns no usable snapshot.
- Any probe error returns no usable snapshot; partial results cannot be
  mistaken for a complete catalog.

Probe implementations remain untrusted observation adapters. A successful probe
does not authorize process start, network access, credentials, capacity
allocation, Agent binding, or Run creation.

### RuntimeObservation

Each observation contains:

- one S2-W1 `RuntimeInstance`;
- a deterministic set of non-empty model IDs reported for that Runtime;
- the source probe ID assigned by the coordinator, not trusted from probe
  output.

The embedded RuntimeInstance must pass the accepted S2-W1 constructor
validation. Its executable version and observed capabilities remain observation
data. Model IDs are Candidate identifiers only and carry no Provider
authorization or guarantee of current availability.

Empty model inventory is permitted because a local Runtime may be discoverable
without exposing a safe model-list operation. Consumers must not infer that an
empty inventory is complete.

### Discovery snapshot

A successful discovery returns:

- observations sorted by RuntimeInstance ID;
- model IDs and capabilities sorted lexicographically;
- a lowercase SHA-256 digest over a documented canonical representation;
- no mutable aliases to probe-owned inputs or coordinator-owned internal data.

Snapshot construction fails closed when:

- a RuntimeInstance is invalid;
- a model ID is empty or duplicated within one observation;
- two observations report the same RuntimeInstance ID, even if their remaining
  fields are identical;
- canonical encoding cannot be produced.

The digest includes every catalog-relevant field exposed by this WorkItem:
source probe ID, RuntimeInstance ID, device ID, adapter type, display name,
executable version, status, capabilities, capacity, and model IDs. Equal
canonical observations produce the same digest regardless of probe or
observation input order; any included field change produces a different digest.

An empty, valid probe set may produce one deterministic empty snapshot. A
configured probe set that fails does not produce an empty-success snapshot.

## Acceptance boundary

1. `internal/runtime` remains independent of concrete Agent CLIs, Provider SDKs,
   SQLite, Journal, projection, Evidence, UI, network, filesystem discovery,
   and process execution packages.
2. Probe-set validation occurs completely before the first invocation.
3. Probe invocation and final snapshot ordering are deterministic under input
   reordering.
4. Context cancellation and probe errors return typed, inspectable errors and
   no usable partial snapshot.
5. Every observation is revalidated through the accepted S2-W1 RuntimeInstance
   boundary; no probe can bypass instance invariants.
6. Empty or duplicate model IDs and duplicate RuntimeInstance IDs fail with
   typed errors inspectable through `errors.Is`.
7. The snapshot digest is stable under input reordering and changes when any
   included field changes.
8. Returned observations, RuntimeInstances, models, and capability collections
   do not expose mutable aliases.
9. Tests use deterministic in-memory fakes only. They do not inspect the
   developer machine, environment variables, PATH, home directory, installed
   CLIs, credentials, network, or wall clock.
10. No migration, persistence, Event append, Team/Draft/AgentInstance, default
    Main Agent, Runtime binding, capacity allocation, Bridge, CLI command,
    daemon entrypoint, goroutine, background refresh, real Runtime Adapter,
    external process, Run, Grant, credential access, or product activation is
    introduced.
11. Existing S2-W1 and Slice 1 behavior remains unchanged and green.

## Mandatory RED tests

The Developer adds `internal/runtime/discovery_test.go` before implementation.
The initial focused command must fail because the frozen discovery symbols do
not exist, not because of syntax, environment, dependency, or unrelated
repository failure.

Required test groups:

1. probe-set prevalidation for nil, empty, and duplicate probe IDs, proving no
   probe was invoked;
2. deterministic probe invocation and snapshot ordering under reordered input;
3. RuntimeInstance revalidation and typed rejection of duplicate instance IDs;
4. model inventory normalization plus typed empty/duplicate rejection;
5. context cancellation and probe-error fail-closed behavior with no partial
   snapshot;
6. digest stability under input reordering and digest sensitivity for every
   included field class;
7. mutation isolation for probe inputs and every returned slice;
8. static import-boundary assertion proving the production file contains no
   concrete I/O, process, network, persistence, UI, or adapter dependency.

## Deterministic checks

- RED:
  `go test ./internal/runtime -run 'TestDiscoverRuntime|TestRuntimeDiscovery' -count=1`
- Focused GREEN:
  `go test ./internal/runtime -run 'TestDiscoverRuntime|TestRuntimeDiscovery' -count=1`
- Package full:
  `go test ./internal/runtime -count=1`
- Repeated focused race:
  `go test -race ./internal/runtime -run 'TestDiscoverRuntime|TestRuntimeDiscovery' -count=50`
- Impact:
  `go test ./... -count=1`
- Repository race:
  `go test -race ./... -count=1`
- Static analysis:
  `go vet ./...`
- Formatting and diff:
  `gofmt` on changed Go files, trailing-whitespace check, and
  `git diff --check`
- Scope:
  verify the Candidate changes only the frozen Developer-owned files, preserves
  branch `codex/loom-platform-slice2`, and does not move HEAD before the
  authorized atomic commit.

## Required evidence

- frozen contract digest;
- exact RED terminal output and exit code;
- focused GREEN, package, repeated race, impact, repository race, and vet
  terminal output;
- deterministic digest and mutation-isolation report;
- changed-file and import-boundary report;
- trust-boundary analysis;
- fresh independent implementation Reviewer verdict and findings;
- deliverable ending with `VERDICT: PASS` only after all gates pass.

## Trust-boundary analysis

- A RuntimeProbe is an untrusted observation port, not execution authority.
- RuntimeObservation and the discovery snapshot are rebuildable Candidate data,
  not Event Journal facts or a second authoritative state writer.
- Discovery success does not imply Runtime compatibility, binding approval,
  available capacity, credential access, or permission to start a process.
- Model IDs are observed strings, not Provider entitlements or permission to
  invoke a model.
- Failures return no complete snapshot; callers cannot silently treat partial
  discovery as authoritative completeness.
- No raw credential, token, environment value, executable output, home path, or
  Provider response enters the snapshot.
- Later Team Draft catalog membership, invented-ID rejection, and acceptance
  remain governed by their own frozen contracts.

## Governance and next gate

- This freeze authorizes no product write until a fresh independent read-only
  contract Reviewer returns `PASS`.
- The user's active continuous Phase 1 authorization permits the Developer to
  begin mandatory RED automatically after that contract PASS.
- One Developer is the only product-file writer for this Candidate lineage.
- The Controller owns deterministic checks and state/evidence publication.
- The Developer may submit only `ready_for_review`; a fresh implementation
  Reviewer decides PASS or required repair.
- Same-lineage bounded repairs follow `docs/DEVELOPMENT.md`; three failed
  product repairs require `HUMAN_REQUIRED`.
- After all checks and fresh implementation Reviewer PASS, exactly one strictly
  scoped local atomic S2-W2 commit is authorized.
- Push, merge, rebase, reset, force, release, publish, credentials, `.env`,
  dependency installation, external mutation, paid remote work, daemon or real
  Runtime activation, and autonomous execution remain prohibited.
- Bridge/JSONL, real Runtime Adapter execution, AgentGrant, claim generation,
  prepare lease, WorkItem dispatch, and Run lifecycle remain Slice 3 boundaries.
