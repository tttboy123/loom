# S2-W18 Frozen WorkItem Contract

- ID: `S2-W18`
- Title: Isolated Local Pi Metadata Process Runner
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W17 local commit `b73cf8b`
- Corresponds to: `TECH-PLAN.md §4, §13.1, §14 Slice 2.2, §15.8-§15.9`,
  ADR-0003, and ADR-0004
- Frozen branch/head: `codex/loom-platform-slice2` at `b73cf8b`

## Owned files

- `internal/runtime/piadapter/process_runner.go`
- `internal/runtime/piadapter/process_runner_test.go`
- `internal/runtime/piadapter/process_unix.go`
- `internal/runtime/piadapter/process_other.go`
- `.loom-evidence/phase1-slice2/S2-W18/deliverable.md`

The Developer owns only these files. Controller-owned status, review, and
contract evidence remain outside Developer ownership. Accepted Slice 1 and
S2-W1 through S2-W17 product/test files remain unchanged. Any product-file
ownership amendment requires a recorded Controller amendment and a fresh
contract Reviewer PASS before the additional path is written.

## Objective

Provide the smallest concrete local process implementation of the accepted
S2-W17 `PiMetadataRunner` in the child adapter package
`internal/runtime/piadapter`:

1. bind one explicitly configured absolute Pi executable to one validated
   daemon-owned isolation root and a bounded execution timeout;
2. execute only the two exact S2-W17 metadata requests without a shell, prompt,
   stdin, inherited environment, user home, or user Pi agent directory;
3. contain all writable HOME/Pi/session/tmp state in a fresh private
   per-invocation directory and clean it after every outcome;
4. bound captured stdout/stderr and kill the metadata process group on Unix
   cancellation or timeout; and
5. return only S2-W17 metadata results or typed, non-disclosing errors.

This WorkItem does not locate Pi on PATH, choose an executable, inspect user Pi
config/auth, run the real Pi binary during verification, schedule discovery,
append Events, persist Runtime state, start an Agent session, send a prompt,
invoke a model, or activate a Runtime Adapter. A later locator/daemon WorkItem
must supply configured executable/search roots and scheduling.

## Frozen public boundary

### Configuration

`PiMetadataProcessRunnerConfig` contains:

- `ExecutablePath`: one absolute path selected by a higher trusted layer;
- `IsolationRoot`: one absolute, existing daemon-owned directory;
- `RuntimeSearchPaths`: one or more absolute existing directories used only to
  build the child `PATH` for an executable whose shebang uses
  `/usr/bin/env`;
- `Timeout`: a positive duration no greater than 30 seconds.

`NewPiMetadataProcessRunner(PiMetadataProcessRunnerConfig)` returns one
S2-W17 `runtime.PiMetadataRunner` or
`ErrInvalidPiMetadataProcessRunner`.

Construction:

1. rejects empty, relative, unclean, NUL-containing, missing, or invalid paths;
2. resolves the executable symlink once, requires the resolved target to be a
   non-directory regular executable no larger than 512 MiB, computes a SHA-256
   digest, and records its stable file identity;
3. resolves each search directory once, records its stable directory identity,
   and stores a copied, deduplicated, ordered canonical list;
4. resolves the isolation root once, requires a real non-symlink directory with
   exact `0700` permissions, and records its stable file identity; and
5. copies all mutable input collections.

The configured executable, isolation root, and canonical runtime-search
directories are revalidated immediately before each invocation. Identity,
executable digest, type, executable permission, search-directory identity/type,
root type, or root permission drift fails closed. The runner always invokes the
stored resolved executable path, never the original symlink, so repointing the
original symlink cannot redirect execution.

This is a local integrity check, not a general hostile-filesystem sandbox.
The higher trusted layer must select a genuine Pi installation and a
daemon-owned root. OS-level replacement between final validation and `execve`
remains a platform limitation and must not be described as eliminated.
For a script using `/usr/bin/env`, the stored search-directory identities bind
which directories participate in interpreter lookup, not the identity or bytes
of an interpreter later replaced within the same directory. The executable
digest covers the selected Pi file only. Interpreter, dynamic-loader, and
library drift remain explicit trusted-runtime-path risks owned by the higher
installation/locator boundary.

### Accepted requests

`RunPiMetadata` accepts only the exact immutable S2-W17 requests:

- `PiMetadataVersion` with `["--version"]`;
- `PiMetadataListModels` with the exact eight frozen isolation/list flags in
  their frozen order.

Unknown command values, missing/extra/reordered/mutated arguments, nil context,
or any request drift returns `ErrInvalidPiMetadataRequest` before directory
creation or process start.

No generic executable, shell, cwd, environment, stdin, arbitrary argument, or
command API is added.

### Per-invocation isolation

For every valid request, the runner:

1. creates one fresh directory beneath the validated isolation root using an
   unpredictable `loom-pi-metadata-*` name;
2. enforces `0700` on the invocation directory and its `home`, `agent`,
   `sessions`, and `tmp` children;
3. uses the invocation `home` as the child working directory;
4. provides no stdin, extra file descriptor, inherited environment, or caller
   environment value;
5. supplies exactly these child environment keys:
   - `HOME=<invocation home>`;
   - `TMPDIR=<invocation tmp>`;
   - `PI_CODING_AGENT_DIR=<invocation agent>`;
   - `PI_CODING_AGENT_SESSION_DIR=<invocation sessions>`;
   - `PATH=<canonical configured runtime search paths>`;
   - `LANG=C`;
   - `LC_ALL=C`;
   - `NO_COLOR=1`;
   - `TERM=dumb`;
   - `PI_OFFLINE=1`;
   - `PI_SKIP_VERSION_CHECK=1`;
   - `PI_TELEMETRY=0`;
6. captures stdout/stderr separately up to 256 KiB each without pipe deadlock;
7. waits for the process and its cleanup boundary; and
8. attempts to remove the exact generated invocation directory after success,
   nonzero exit, cancellation, timeout, or output overflow.

No API key, OAuth token, provider variable, proxy variable, certificate path,
user HOME, user Pi config path, project path, source path, or caller environment
value is inherited. The fixed Pi offline flags express Pi-level network intent;
this WorkItem does not claim OS-enforced network sandboxing.

### Process lifetime and output

- The runner derives a child context bounded by configured timeout.
- Caller cancellation propagates unchanged.
- Internal timeout returns `ErrPiMetadataProcessTimeout`.
- On Unix, the child starts in its own process group and cancellation/timeout
  sends termination to that original process group.
- Non-Unix platforms use the strongest standard-library child cancellation
  available and must not claim descendant-group guarantees.
- Unix process-group cleanup covers only processes that remain in the original
  group. A child that deliberately creates a new process group/session can
  escape this boundary; preventing that requires a later OS supervisor/sandbox.
- A one-second `WaitDelay` bounds stuck pipe/process teardown.
- A nonzero exit returns `ErrPiMetadataProcessFailed`.
- Stdout or stderr beyond 256 KiB returns
  `ErrPiMetadataProcessOutputTooLarge`.
- Invocation-directory cleanup failure returns
  `ErrPiMetadataIsolationCleanup`.
- Executable or isolation-root drift returns `ErrPiMetadataBindingChanged`.

Errors never include executable/root/search paths, arguments, stdout, stderr,
exit diagnostics, environment values, file digests, child errors, or
credential-like material. Failure returns a zero `PiMetadataResult`.
Successful output is copied into a fresh `PiMetadataResult` for S2-W17 to
validate; this runner does not parse or trust it.

## Frozen typed errors

Sentinels inspectable through `errors.Is`:

- `ErrInvalidPiMetadataProcessRunner`;
- `ErrInvalidPiMetadataRequest`;
- `ErrPiMetadataBindingChanged`;
- `ErrPiMetadataProcessFailed`;
- `ErrPiMetadataProcessTimeout`;
- `ErrPiMetadataProcessOutputTooLarge`;
- `ErrPiMetadataIsolationCleanup`.

Caller cancellation/deadline errors remain directly inspectable with
`errors.Is` when the caller context caused termination. If cleanup also fails,
the runner returns a zero result and `errors.Join(operationError,
ErrPiMetadataIsolationCleanup)` so both failures remain inspectable. The same
join rule applies to internal timeout, nonzero exit, overflow, or binding
failure combined with cleanup failure. Cleanup failure never hides an operation
failure, and an operation failure never hides cleanup residue. Raw
`*exec.ExitError`, `*os.PathError`, child stderr, and cleanup paths are not
returned or wrapped.

## Acceptance boundary

1. The child adapter package implements S2-W17 `runtime.PiMetadataRunner`
   without changing S2-W17 or earlier accepted files; the parent
   `internal/runtime` package remains pure and imports no concrete process
   dependency.
2. Constructor validation freezes executable target/digest, search-directory
   identities, isolation root, permissions, identities, timeout, and copied
   inputs.
3. Original executable symlink retargeting cannot redirect execution; target
   or root drift fails closed before process start.
4. Only the two exact S2-W17 requests can create state or start a process.
5. Child cwd, writable state, environment, stdin, args, output, timeout, and
   cleanup match the frozen boundary exactly.
6. Parent secrets and caller environment values are absent from the child.
7. Cancellation, timeout, nonzero exit, oversized output, binding drift, and
   cleanup failure return typed non-disclosing errors and zero output.
8. Unix timeout/cancellation terminates the original metadata process group; a
   same-group child cannot perform the delayed test side effect. New-group or
   new-session escape is explicitly not claimed.
9. Ordinary tested terminal paths leave no invocation directory. An
   intentionally induced cleanup denial returns
   `ErrPiMetadataIsolationCleanup` (joined with any operation failure);
   verification then restores fixture permissions and removes only the exact
   fixture residue.
10. Integration through S2-W17 and S2-W2 yields one canonical discovery
    observation using only a deterministic fake executable in tests.
11. Verification never invokes installed Pi, reads user Pi state, uses PATH
    lookup, accesses credentials/network, or installs a binary/dependency.
12. No daemon, scheduler, Journal/projection, Team/Draft/WorkItem/Run/Grant,
    Runtime Adapter, Agent session, prompt, model call, or activation is added.
13. Existing Slice 1 and S2-W1 through S2-W17 behavior remains green.

## Mandatory RED tests

The Developer adds
`internal/runtime/piadapter/process_runner_test.go` before product. The initial
focused command must fail only because frozen S2-W18 symbols do not exist.

Required test groups:

1. constructor matrix for invalid/relative/unclean/missing/NUL paths,
   executable type/permission/size, symlink resolution, search-path
   normalization/copying/identity, root symlink/type/mode, and timeout bounds;
2. exact request prevalidation proving invalid commands/args create no
   directory and start no process;
3. deterministic fake-executable success proving exact args, cwd, private
   directories, exact environment allowlist, absent parent secrets, bounded
   copied outputs, and complete cleanup;
4. executable symlink retarget immunity plus executable/root/search-directory
   drift rejection, with explicit proof that interpreter bytes inside an
   unchanged search directory are outside this binding;
5. S2-W17 probe and S2-W2 discovery integration through the fake executable;
6. caller cancellation, internal timeout, nonzero exit, stdout/stderr overflow,
   cleanup-failure zero-result behavior, and combined operation-plus-cleanup
   `errors.Is` inspectability;
7. Unix original-process-group cleanup using a same-group fake child whose
   delayed external marker must never appear after timeout/cancellation;
8. error non-disclosure for path/output/stderr/argument/environment/secret-like
   markers; and
9. static review proving direct `os/exec` use without shell in the child
   adapter, no inherited environment, no PATH lookup, no
   network/persistence/UI/credential imports, platform-specific cancellation
   isolated to build-tag files, and zero concrete-process imports in the parent
   `internal/runtime` package.

Tests may create private temporary directories and deterministic executable
shell fixtures under `t.TempDir()`. They must not invoke installed Pi, use a
package manager, inspect user home/config/auth, inherit provider secrets, or
access network.

## Deterministic checks

- RED:
  `go test ./internal/runtime/piadapter -run 'TestPiMetadataProcessRunner' -count=1`
- Focused GREEN:
  `go test ./internal/runtime/piadapter -run 'TestPiMetadataProcessRunner' -count=1`
- Package full:
  `go test ./internal/runtime ./internal/runtime/piadapter -count=1`
- Repeated focused race:
  `go test -race ./internal/runtime/piadapter -run 'TestPiMetadataProcessRunner' -count=20`
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
  verify the Candidate changes only frozen Developer-owned files, preserves
  branch `codex/loom-platform-slice2`, leaves accepted files unchanged, invokes
  no installed Pi, and does not move HEAD before the authorized atomic commit.

## Required evidence

- frozen contract digest;
- exact RED output and exit code;
- focused GREEN, package, focused-race-20, repository, repository-race, vet,
  format, and scope outputs;
- executable/root binding, exact request, environment allowlist, process-group
  cleanup, output bound, and invocation cleanup reports;
- proof that no actual Pi command, credential, network, package manager, or user
  Pi state was touched;
- trust-boundary and residual platform-limitation analysis;
- fresh independent implementation Reviewer verdict and findings; and
- deliverable ending with `VERDICT: PASS` only after every gate passes.

## Trust-boundary analysis

- The higher trusted layer selects the executable and private isolation root;
  this runner validates and binds them but does not discover or authorize them.
- The configured executable is trusted to be genuine Pi metadata code. Offline
  flags and isolated HOME prevent ordinary Pi startup from using user state,
  but they are not a hostile-code filesystem or network sandbox.
- Executable digest/identity and stored resolved path prevent ordinary symlink
  retarget and replacement drift; the final validation-to-exec interval remains
  an acknowledged local OS race.
- Runtime search paths exist only for `/usr/bin/env` shebang resolution and do
  not permit Loom to search for the Pi executable. Directory identity is
  revalidated, but interpreter replacement within an unchanged directory is an
  explicit higher-layer trusted-runtime-path residual risk.
- The child receives no user/provider credentials. Native authentication is
  therefore not exercised by this isolated discovery runner, and an empty
  model inventory is valid and must be reported honestly.
- Output remains untrusted Candidate data. S2-W17 parses it and S2-W2 rebinds
  source identity and revalidates the observation.
- Starting a short-lived metadata process is not Agent Runtime activation,
  session creation, prompt execution, model invocation, or Run authority.

## Governance and next gate

- This freeze authorizes no product/test write until a fresh independent
  read-only contract Reviewer returns `PASS`.
- The user's active continuous Phase 1 authorization permits mandatory RED
  automatically only after that contract PASS.
- One Developer is the only product-file writer for this Candidate lineage.
- The Controller owns deterministic checks and state/evidence publication.
- The Developer may submit only `ready_for_review`; a fresh implementation
  Reviewer decides PASS or bounded repair.
- Same-lineage repairs follow `docs/DEVELOPMENT.md`; three failed product
  repairs require `HUMAN_REQUIRED`.
- After all checks and fresh implementation Reviewer PASS, exactly one strictly
  scoped local atomic S2-W18 commit is authorized.
- Verification may execute only deterministic fake fixtures. Running installed
  Pi or activating a real Runtime still requires a later live-check gate and
  explicit authority.
- Push, merge, rebase, reset, force, release, publish, credentials, `.env`,
  dependency installation, external mutation, paid remote work, daemon or real
  Runtime activation, and autonomous execution remain prohibited.
- Executable location/discovery, stronger OS sandbox/supervisor containment,
  daemon scheduling, Runtime discovery Journal facts/projection, and Slice 2
  completion remain separately frozen future WorkItems. Bridge/JSONL, real
  Runtime Adapter execution, AgentGrant, claim generation, prepare lease,
  WorkItem dispatch, and Run lifecycle remain Slice 3 boundaries.
