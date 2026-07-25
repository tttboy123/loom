# S2-W17 Frozen WorkItem Contract

- ID: `S2-W17`
- Title: Pi CLI Metadata Probe Core
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W1 Runtime contracts, accepted S2-W2 discovery
  coordination, and accepted S2-W16 local commit `42fc661`
- Corresponds to: `TECH-PLAN.md §4, §13.1, §14 Slice 2.1-Slice 2.2, and
  §15.7-§15.9`
- Frozen branch/head: `codex/loom-platform-slice2` at `42fc661`

## Owned files

- `internal/runtime/pi_probe.go`
- `internal/runtime/pi_probe_test.go`
- `.loom-evidence/phase1-slice2/S2-W17/deliverable.md`

The Developer owns only these files. Controller-owned status, review, and
contract evidence remain outside Developer ownership. Accepted Slice 1 and
S2-W1 through S2-W16 product/test files remain unchanged. Any product-file
ownership amendment requires a recorded Controller amendment and a fresh
contract Reviewer PASS before the additional path is written.

## Objective

Create the smallest Pi-specific metadata adapter core that can satisfy the
accepted S2-W2 `RuntimeProbe` port after a later process runner is supplied:

1. define a narrow injected runner port for two read-only Pi metadata commands;
2. freeze deterministic Pi version and model-list command requests without
   starting an Agent session or sending a prompt;
3. parse bounded current Pi metadata output into one validated S2-W1
   `RuntimeInstance` plus canonical model IDs; and
4. return that observation to the accepted S2-W2 coordinator without treating
   metadata as execution authority.

This WorkItem introduces Pi-specific command semantics and parsing, but no
`os/exec`, PATH search, filesystem or environment inspection, credential read,
daemon scheduling, Runtime process, model call, persistence, or activation. A
later separately frozen WorkItem must provide the local process runner and its
isolation boundary before automatic local discovery is complete.

## Verified Pi metadata surface

The contract is based on the current upstream Pi CLI metadata surface verified
on 2026-07-25:

- `pi --version` prints the package version and exits before session creation;
- `pi --list-models` prints either an exact six-column table headed
  `provider model context max-out thinking images`, or begins with
  `No models available.`;
- `--offline`, `--no-approve`, `--no-extensions`, `--no-skills`,
  `--no-prompt-templates`, `--no-themes`, and `--no-context-files` are current
  CLI flags; and
- the current CLI still performs startup service construction and migrations
  before `--list-models`, so this WorkItem must not invoke it directly against
  user state.

Upstream output or startup behavior is untrusted and may drift. The parser
fails closed on unrecognized format. The later concrete runner contract must
provide an isolated, non-user-state execution boundary rather than assuming
these flags alone make `--list-models` mutation-free.

## Frozen public boundary

### Pi metadata commands

`PiMetadataCommand` has exactly two values:

- `PiMetadataVersion`;
- `PiMetadataListModels`.

`PiMetadataRequest` contains only the command and a copied argument vector.
Requests are deterministic:

- version: `--version`;
- model list: `--offline`, `--no-approve`, `--no-extensions`, `--no-skills`,
  `--no-prompt-templates`, `--no-themes`, `--no-context-files`,
  `--list-models`.

No request contains a prompt, provider credential, API key, model invocation,
session identifier, file argument, project path, home path, or mutation
command.

`PiMetadataResult` contains captured stdout and stderr only. The injected
`PiMetadataRunner` exposes one context-aware operation:

```go
RunPiMetadata(context.Context, PiMetadataRequest) (PiMetadataResult, error)
```

The port does not expose a general executable, shell, environment, working
directory, stdin, or arbitrary argument surface. This prevents the domain
adapter from becoming a generic process-execution authority.

### Pi probe construction

`PiRuntimeProbeConfig` contains:

- one stable non-empty probe ID;
- one stable non-empty RuntimeInstance ID;
- one stable non-empty device ID;
- one stable non-empty display name; and
- one non-nil `PiMetadataRunner`.

`NewPiRuntimeProbe(PiRuntimeProbeConfig)` validates and copies the configuration
before returning a probe implementing the accepted S2-W2 `RuntimeProbe`.
Constructor failure returns `ErrInvalidPiRuntimeProbe`.

The resulting probe:

- returns the configured stable ID from `ID()`;
- invokes version first and model list second;
- does not invoke model list when version fails;
- checks cancellation before each call and after each return;
- returns no observation on cancellation, runner error, non-empty stderr,
  oversized/invalid output, parse failure, or S2-W1 validation failure; and
- emits exactly one observation on success.

### Bounded output

Each stdout/stderr value must be valid UTF-8, contain no NUL, and be no larger
than 256 KiB. Model output may contain at most 1,024 model rows. Oversize,
invalid encoding, NUL, non-empty stderr, and row overflow fail with typed errors
inspectable through `errors.Is`.

Errors and Candidate data must not copy raw stdout, stderr, runner error text,
paths, environment values, or credential-adjacent guidance. Error messages may
identify only the metadata operation and the Loom-owned error class.

### Version parsing

Version stdout must trim to exactly one non-empty line of at most 128 bytes.
The line may contain only ASCII letters, digits, `.`, `+`, `_`, and `-`, and
must begin with an ASCII letter or digit. The exact validated line becomes
`RuntimeInstance.ExecutableVersion`. Whitespace-separated prose, multiple
lines, control characters, and unsupported punctuation fail closed.

### Model-list parsing

The parser accepts exactly one of:

1. output whose first trimmed line is exactly `No models available.`; remaining
   guidance is ignored and no model ID is emitted; or
2. a table whose first non-empty line has exactly the six frozen headers and
   whose remaining non-empty lines each contain exactly six whitespace-delimited
   fields.

For table rows:

- provider and model are non-empty printable ASCII tokens, individually at
  most 256 bytes;
- provider cannot contain `/`; model may contain `/`;
- canonical model identity is `provider/model`;
- context and max-out match a positive decimal token with optional single
  decimal fraction and optional `K` or `M` suffix;
- thinking and images are each exactly `yes` or `no`;
- duplicate canonical model IDs fail with `ErrDuplicatePiRuntimeModel`; and
- returned model IDs are sorted lexicographically.

Unknown headers, column count drift, invalid fields, duplicate rows, prose
mixed into a table, or more than 1,024 rows fail closed with
`ErrInvalidPiMetadataOutput`.

### Runtime observation

Successful observation uses:

- configured RuntimeInstance ID, device ID, and display name;
- adapter type `pi-cli`;
- parsed executable version;
- status `online`;
- exact observed metadata capabilities
  `pi.metadata.models` and `pi.metadata.version`;
- capacity `1`;
- parsed canonical model IDs; and
- an empty source-probe ID, because the accepted S2-W2 coordinator overwrites
  that field with the trusted probe ID.

The observation must pass accepted S2-W1 `NewRuntimeInstance` validation before
return. Metadata success proves only that the isolated command surface answered;
it does not prove Agent execution, tool availability, Provider entitlement,
credential availability, model reachability, or permission to start a Run.

## Frozen typed errors

The product defines sentinels inspectable through `errors.Is`:

- `ErrInvalidPiRuntimeProbe`;
- `ErrPiMetadataCommandFailed`;
- `ErrInvalidPiMetadataOutput`;
- `ErrPiMetadataOutputTooLarge`;
- `ErrPiMetadataStderr`;
- `ErrDuplicatePiRuntimeModel`.

Context cancellation and deadline errors propagate unchanged. Runner failures
map to `ErrPiMetadataCommandFailed` without embedding the runner error text.
All other failures return a zero-length observation slice.

## Acceptance boundary

1. The probe implements the accepted S2-W2 `RuntimeProbe` without changing that
   interface or coordinator.
2. Construction fails closed for nil runner or any empty identity field.
3. Requests, invocation order, and observation content are deterministic.
4. Cancellation, version failure, model-list failure, stderr, malformed output,
   and validation failure return no partial observation.
5. Version and model parsing enforce every frozen format, size, UTF-8, NUL, row,
   duplicate, and canonicalization rule.
6. The result contains exact stable identity, `pi-cli`, online status, capacity
   one, metadata capabilities, version, and canonical provider/model IDs.
7. Inputs, requests, results, observations, capabilities, and model slices
   expose no mutable aliases.
8. Product and errors never retain or emit raw command output, runner error
   text, environment values, paths, auth guidance, or credentials.
9. Tests use deterministic in-memory runners only. They do not execute Pi,
   inspect PATH/home/environment/config/auth, access network, read or write the
   filesystem, or use wall clock.
10. Production imports are standard library plus the accepted package-local
    runtime contracts only. There is no `os`, `os/exec`, `syscall`, filesystem,
    network, SQLite, Journal, projection, UI, or Provider SDK dependency.
11. No daemon, goroutine, background refresh, concrete executable runner,
    Runtime Adapter, Agent process, session, prompt, model call, credential
    access, persistence, Event, Team/Draft/WorkItem/Run/Grant, or activation is
    introduced.
12. Existing Slice 1 and S2-W1 through S2-W16 behavior remains green.

## Mandatory RED tests

The Developer adds `internal/runtime/pi_probe_test.go` before production. The
initial focused command must fail only because the frozen S2-W17 symbols do not
exist.

Required test groups:

1. constructor rejection for every empty identity and nil/typed-nil runner;
2. exact immutable version/model request arguments and deterministic call order;
3. successful one-observation mapping, no-model mapping, canonical model sort,
   and accepted S2-W2 coordinator integration;
4. fail-closed cancellation before/between/after calls, runner failure, and
   proof that version failure prevents model invocation;
5. full output-boundary matrix: size, UTF-8, NUL, stderr, version grammar,
   header/column drift, token grammar, duplicate model, and row ceiling;
6. proof that raw stdout, stderr, runner error text, paths, and credential-like
   strings never appear in returned errors or observations;
7. mutation isolation for config, request args, runner results, observations,
   capabilities, and models; and
8. static import-boundary proof excluding general process, environment,
   filesystem, network, persistence, and execution dependencies.

## Deterministic checks

- RED:
  `go test ./internal/runtime -run 'TestPiRuntimeProbe|TestParsePi' -count=1`
- Focused GREEN:
  `go test ./internal/runtime -run 'TestPiRuntimeProbe|TestParsePi' -count=1`
- Package full:
  `go test ./internal/runtime -count=1`
- Repeated focused race:
  `go test -race ./internal/runtime -run 'TestPiRuntimeProbe|TestParsePi' -count=50`
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
  branch `codex/loom-platform-slice2`, leaves accepted product/test files
  unchanged, and does not move HEAD before the authorized atomic commit.

## Required evidence

- frozen contract digest;
- upstream Pi metadata surface URLs and verification date;
- exact RED terminal output and exit code;
- focused GREEN, package, focused-race-50, repository, repository-race, vet,
  format, and scope outputs;
- command/parse boundary and mutation-isolation report;
- secret/output non-disclosure and import-boundary report;
- trust-boundary analysis;
- fresh independent implementation Reviewer verdict and findings; and
- deliverable ending with `VERDICT: PASS` only after every gate passes.

## Trust-boundary analysis

- The injected runner is untrusted observation I/O, not general command or
  execution authority.
- Pi stdout, stderr, exit status, and runner errors are untrusted and may contain
  local paths, auth guidance, or secret-adjacent text; none enter Candidate
  state or surfaced errors.
- The Pi metadata probe is a Candidate adapter. S2-W2 remains the coordinator
  that validates and binds source probe identity.
- Online metadata status means only that both isolated metadata requests
  succeeded. It does not authorize Pi execution or establish Provider/model
  availability.
- Fixed metadata capabilities describe only the observed command surface, not
  Pi's Agent tools.
- Model IDs are catalog strings, not entitlements, credentials, capacity, or
  permission to invoke a model.
- The later concrete runner must isolate Pi startup from user config because
  current `--list-models` startup may run migrations. That later contract,
  not this parser, owns executable resolution, environment allowlisting,
  temporary state, process timeout/cleanup, and filesystem effects.

## Governance and next gate

- This freeze authorizes no product or test write until a fresh independent
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
  scoped local atomic S2-W17 commit is authorized.
- Push, merge, rebase, reset, force, release, publish, credentials, `.env`,
  dependency installation, external mutation, paid remote work, daemon or real
  Runtime activation, and autonomous execution remain prohibited.
- A concrete isolated Pi process runner, daemon scheduling, discovery Journal
  facts/projection, and Slice 2 completion remain separately frozen future
  WorkItems. Bridge/JSONL, real Runtime Adapter execution, AgentGrant, claim
  generation, prepare lease, WorkItem dispatch, and Run lifecycle remain Slice
  3 boundaries.
