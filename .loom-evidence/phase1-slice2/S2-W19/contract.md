# S2-W19 Frozen WorkItem Contract

- ID: `S2-W19`
- Title: Configured Local Pi Probe Factory
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W18 local commit `8c8fb9e`
- Corresponds to: `TECH-PLAN.md §4, §13.1, §14 Slice 2.2, §15.8-§15.9`,
  ADR-0003, and ADR-0004
- Frozen branch/head: `codex/loom-platform-slice2` at `8c8fb9e`

## Upstream fact

Read-only verification against the current primary Pi source on 2026-07-25
confirms that the coding-agent package publishes the fixed executable name
`pi`. This contract freezes only that name. It does not infer aliases, invoke a
package manager, install or update Pi, or trust ambient PATH.

## Owned files

- `internal/runtime/piadapter/local_probe.go`
- `internal/runtime/piadapter/local_probe_test.go`
- `.loom-evidence/phase1-slice2/S2-W19/deliverable.md`

The Developer owns only these files. Controller-owned contract, review, status,
and documentation evidence remain outside Developer ownership. S2-W18 and all
earlier accepted files remain unchanged. Any ownership amendment requires a
recorded Controller amendment and fresh contract Reviewer `PASS` before the
additional path is written.

## Objective

Add the smallest concrete factory that closes the configuration gap between a
daemon-supplied trusted local search-path set and the accepted S2-W17/S2-W18 Pi
probe chain:

1. freeze caller-supplied probe identity, device/display identity, private
   isolation root, timeout, and ordered trusted runtime search directories;
2. inspect only the fixed `pi` entry directly beneath those directories, in
   configured order, without ambient PATH lookup or recursive scanning;
3. distinguish a normal absent installation from an invalid/shadowing
   installation or binding drift;
4. construct the accepted S2-W18 process runner and S2-W17 Runtime probe for the
   first valid configured candidate; and
5. expose no executable or search path in public results or errors.

This WorkItem does not start Pi while locating or constructing the probe,
schedule discovery, create a daemon entrypoint, read user Pi config/auth,
inherit environment, append Events, persist or project Runtime state, choose a
RuntimeProfile, start an Agent session, send a prompt, invoke a model, or
activate a Runtime Adapter.

## Frozen public boundary

### Configuration

`PiLocalRuntimeProbeFactoryConfig` contains:

- `ProbeID`: explicit stable S2-W2 probe identity;
- `InstanceID`: explicit stable RuntimeInstance identity;
- `DeviceID`: explicit local device identity;
- `DisplayName`: explicit user-facing Runtime name;
- `IsolationRoot`: the same daemon-owned private root boundary required by
  S2-W18;
- `RuntimeSearchPaths`: one or more ordered trusted local executable/interpreter
  directories; and
- `Timeout`: the S2-W18 per-metadata-call timeout.

`NewPiLocalRuntimeProbeFactory(PiLocalRuntimeProbeFactoryConfig)` returns a
non-nil immutable `*PiLocalRuntimeProbeFactory` or
`ErrInvalidPiLocalRuntimeProbeFactory`.

Construction:

1. rejects empty identity/display fields;
2. applies the S2-W18 timeout bound: positive and no greater than 30 seconds;
3. binds `IsolationRoot` using the S2-W18 real, non-symlink, exact-`0700`,
   stable-identity rule;
4. resolves, validates, deduplicates, and binds `RuntimeSearchPaths` using the
   S2-W18 ordered stable-directory-identity rule; and
5. copies all caller-owned mutable inputs.

No environment variable, process current directory, user home, shell profile,
package-manager prefix, or ambient PATH is consulted.

### Probe construction

`BuildProbe(context.Context)` returns:

- `(probe, true, nil)` when one configured Pi installation is selected and the
  accepted S2-W18 runner plus S2-W17 probe are constructed;
- `(nil, false, nil)` when the fixed `pi` entry is absent from every configured
  directory; or
- `(nil, false, typedError)` for invalid input, binding drift, an invalid
  shadowing candidate, or construction failure.

Before filesystem inspection, the method rejects a nil context with
`ErrInvalidPiLocalRuntimeProbeRequest` and propagates caller cancellation or
deadline unchanged. It then revalidates the isolation-root and search-directory
identities/types/permissions.

The scan:

1. checks only `filepath.Join(canonicalSearchDirectory, "pi")`;
2. visits directories once in the frozen order;
3. does not recurse, glob, call `exec.LookPath`, or inspect any other name;
4. treats `os.ErrNotExist` as normal absence and continues;
5. treats permission/I/O errors or a present non-regular, non-executable,
   oversized, broken-link, or otherwise invalid first candidate as
   `ErrPiLocalRuntimeCandidateInvalid`;
6. selects the first present valid candidate, matching the trusted configured
   search order, and does not inspect later directories after selection; and
7. passes the original selected candidate path, frozen canonical search paths,
   isolation root, and timeout to `NewPiMetadataProcessRunner`, then passes the
   configured identities and returned runner to `runtime.NewPiRuntimeProbe`.

A valid symlink named `pi` may resolve outside its search directory, as is
normal for package-manager shims. S2-W18 resolves, hashes, identity-binds, and
later invokes the target. The higher trusted layer therefore owns the search
directory and its symlink policy. A symlink retarget after probe construction
cannot redirect the already-bound runner.

The factory performs only bounded metadata inspection/hashing required by
S2-W18 construction. It creates no invocation directory and starts no process.
Only a later explicit call to the returned probe's `ObserveRuntime` may execute
the accepted isolated metadata requests.

### Immutability and disclosure

The factory stores copied scalar fields plus bound directory identities. It
does not expose configured paths, resolved executable paths, file identities,
digests, or the concrete runner.

Errors never include paths, candidate names beyond the fixed safe label `pi`,
filesystem diagnostics, identities, digests, environment values, or
credential-like material. Raw `*os.PathError` is not returned or wrapped.

## Frozen typed errors

Sentinels inspectable through `errors.Is`:

- `ErrInvalidPiLocalRuntimeProbeFactory`;
- `ErrInvalidPiLocalRuntimeProbeRequest`;
- `ErrPiLocalRuntimeProbeBindingChanged`;
- `ErrPiLocalRuntimeCandidateInvalid`; and
- `ErrPiLocalRuntimeProbeConstructionFailed`.

Caller cancellation and deadline errors remain directly inspectable. Every
failure returns `nil, false`; no partial probe escapes.

## Acceptance boundary

1. The factory lives only in the concrete `internal/runtime/piadapter` child
   package; parent `internal/runtime` remains pure and unchanged.
2. Construction freezes copied identity/display values, timeout, private root,
   and ordered canonical search-directory bindings.
3. Probe construction revalidates root/search bindings before scanning.
4. Only direct fixed-name `pi` candidates beneath explicitly configured
   directories are inspected; ambient PATH, HOME, cwd, profiles, package
   managers, recursion, aliases, and network are absent.
5. Empty search results are a successful absence, not an online/offline
   Runtime observation and not an error.
6. A present invalid first candidate fails closed and cannot be skipped in
   favor of a later candidate.
7. The first valid candidate deterministically constructs exactly one accepted
   S2-W18 runner and S2-W17 probe with the configured identities.
8. Factory construction and `BuildProbe` do not create temporary state or
   start a process.
9. A deterministic fake executable proves end-to-end S2-W17/S2-W18/S2-W2
   integration only when the returned probe is explicitly observed in tests.
10. Symlink retarget after probe construction cannot redirect execution;
    search/root drift before construction fails closed.
11. Errors and absent results disclose no local path, raw filesystem error,
    digest, environment, output, argument, or secret-like marker.
12. Verification never invokes installed Pi, reads user Pi state, accesses
    credentials/network, installs a binary/dependency, or uses a package
    manager.
13. No daemon, scheduler, Event/Journal/projection, Team/Draft/WorkItem/Run/
    Grant, RuntimeProfile selection, Agent session, prompt, model call, or
    Runtime activation is added.
14. Existing Slice 1 and S2-W1 through S2-W18 behavior remains green.

## Mandatory RED tests

The Developer adds `internal/runtime/piadapter/local_probe_test.go` before
product. The initial focused command must fail only because frozen S2-W19
symbols do not exist.

Required test groups:

1. constructor matrix for empty identities/display, timeout bounds, missing/
   relative/unclean/NUL/root/search paths, root type/symlink/mode, search type,
   ordered deduplication, and caller-slice copying;
2. nil/pre-canceled/deadline context behavior before candidate inspection;
3. empty configured directories yielding `(nil, false, nil)` with no
   invocation directory or process;
4. direct fixed-name and ordered-first selection using only deterministic fake
   executables, including proof that aliases, nested files, and ambient PATH are
   ignored;
5. invalid first-candidate fail-closed behavior for type, permission, size,
   broken symlink, and deterministic permission/I/O failure where portable;
6. root and search-directory identity/type/permission drift rejection before
   candidate selection;
7. valid external-target symlink construction and post-construction retarget
   immunity;
8. explicit observation of the returned fake-backed probe through S2-W2,
   proving configured identities, metadata parsing, model inventory, and
   complete S2-W18 invocation cleanup;
9. error non-disclosure for path, filesystem, environment, output, argument,
   and secret-like markers; and
10. static review proving no `exec.LookPath`, shell, inherited environment,
    recursive walk/glob, network, persistence, UI, credential, daemon, or
    parent-package concrete import.

Tests may create private temporary directories and deterministic executable
shell fixtures under `t.TempDir()`. Only the explicit end-to-end integration
test may execute its temporary fake through the accepted S2-W18 boundary.
Tests must not invoke installed Pi, inherit provider secrets, inspect user
home/config/auth, use a package manager, or access network.

## Deterministic checks

- RED:
  `go test ./internal/runtime/piadapter -run 'TestPiLocalRuntimeProbeFactory' -count=1`
- Focused GREEN:
  `go test ./internal/runtime/piadapter -run 'TestPiLocalRuntimeProbeFactory' -count=1`
- Package full:
  `go test ./internal/runtime ./internal/runtime/piadapter -count=1`
- Repeated focused race:
  `go test -race ./internal/runtime/piadapter -run 'TestPiLocalRuntimeProbeFactory' -count=20`
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
  unchanged, branch is `codex/loom-platform-slice2`, HEAD remains `8c8fb9e`
  until the authorized atomic commit, and no installed Pi is invoked.

## Required evidence

- frozen contract digest and current upstream fixed-name source;
- exact RED output and exit code;
- focused GREEN, package, focused-race-20, repository, repository-race, vet,
  format, import-boundary, and scope outputs;
- copied/bound config, absence, ordered selection, invalid shadow,
  drift, symlink, no-process construction, and fake integration reports;
- proof that ambient PATH/HOME/cwd, actual Pi, user state, credential, network,
  package manager, daemon, persistence, and activation were not used;
- trust-boundary and residual platform-limitation analysis;
- fresh independent implementation Reviewer verdict and findings; and
- deliverable ending with `VERDICT: PASS` only after every gate passes.

## Trust-boundary analysis

- The higher trusted daemon/config layer selects and orders trusted search
  directories, supplies stable public identities, and owns the private root.
- First-match behavior deliberately mirrors the supplied order, not ambient
  process PATH. A malicious entry in a trusted earlier directory is not made
  safe by this factory; present-invalid entries fail closed, while a valid but
  malicious executable remains a higher-layer installation-integrity risk.
- Package-manager symlinks may leave the search directory. S2-W18 binds the
  resolved target, but the higher layer still owns symlink provenance,
  interpreter/runtime paths, and the acknowledged validation-to-exec race.
- Successful construction proves only that a bounded candidate can back a
  Runtime probe. Only later isolated metadata observation can produce an
  untrusted Runtime Candidate, which S2-W17 parses and S2-W2 revalidates.
- Normal absence produces no fabricated offline RuntimeInstance. Persisted
  transitions from previously present to absent belong to later daemon/Event
  WorkItems.

## Governance and next gate

- This freeze authorizes no product/test write until a fresh independent
  read-only contract Reviewer returns `PASS`.
- The user's active continuous Phase 1 authorization permits mandatory RED
  automatically only after that contract `PASS`.
- One Developer is the only product-file writer for this Candidate lineage.
- The Controller owns deterministic checks and evidence publication.
- The Developer may submit only `ready_for_review`; a fresh implementation
  Reviewer decides `PASS` or bounded repair.
- Same-lineage repairs follow `docs/DEVELOPMENT.md`; three failed product
  repairs require `HUMAN_REQUIRED`.
- After all checks and fresh implementation Reviewer `PASS`, exactly one
  strictly scoped local atomic S2-W19 commit is authorized.
- Daemon scheduling, Event persistence/status transitions, projection, and
  executable-installation management remain later Slice 2 WorkItems.
- Bridge/JSONL, real Runtime Adapter execution, AgentGrant, claim generation,
  prepare lease, WorkItem dispatch, and Run lifecycle remain Slice 3.
- Push, merge, rebase, reset, force, release, publish, credentials, `.env`,
  dependency installation, external mutation, paid remote work, daemon or real
  Runtime activation, and autonomous execution remain prohibited.
