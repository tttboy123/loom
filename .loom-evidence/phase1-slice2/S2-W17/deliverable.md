# S2-W17 Deliverable

- WorkItem: `S2-W17`
- Candidate state: `ready_for_review`
- Branch/head before authorized commit:
  `codex/loom-platform-slice2` at `42fc661`
- Frozen contract SHA256:
  `5216acc1807c6a65ae3a72d365f8026d79a61a062ccb43a63ab061d0b027e162`
- Product SHA256:
  `42f39f37c2df3b824edf2c145bfcf5ef0c3dab428a2d8f3b954e28036e0a59cf`
- Test SHA256:
  `2d7f9fdfde5a0a326b55f193c64360efb403a8d0046d415b5f59ac7e749bd2a2`

## Boundary delivered

`internal/runtime/pi_probe.go` adds a Pi-specific implementation of the
accepted S2-W2 `RuntimeProbe` over a narrow injected `PiMetadataRunner`:

- deterministic `--version` and isolated-intent `--list-models` metadata
  requests;
- strict 256-KiB UTF-8/NUL/stderr boundaries;
- one-line bounded version parsing;
- exact current six-column model table parsing, canonical `provider/model`
  identity, duplicate rejection, stable sorting, and a 1,024-row ceiling;
- safe handling of current `No models available.` output without retaining its
  path-bearing guidance;
- one validated online `pi-cli` RuntimeInstance with capacity one and only
  `pi.metadata.models` / `pi.metadata.version` observed capabilities; and
- typed fail-closed errors that never include raw stdout, stderr, runner error
  text, local paths, or credential-like material.

No concrete executable runner, `os/exec`, PATH/home/environment/config/auth
inspection, filesystem or network I/O, daemon scheduling, Runtime process,
Agent session, prompt, model call, persistence, Event, or activation exists in
this Candidate.

## Upstream metadata verification

Read-only primary-source verification on 2026-07-25 used:

- `https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/src/cli/list-models.ts`
- `https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/src/cli/args.ts`
- `https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/src/main.ts`
- `https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/src/migrations.ts`

Current `--list-models` constructs startup services and can run migrations
before printing metadata. S2-W17 therefore injects but does not implement the
runner. A later frozen runner contract must isolate all Pi state and process
effects rather than invoking this command against user state.

## Fresh contract review

Evidence:
`.loom-evidence/phase1-slice2/S2-W17/contract-review.md`

Result: `PASS`; no blocking findings.

## Mandatory RED

Command:

```text
go test ./internal/runtime -run 'TestPiRuntimeProbe|TestParsePi' -count=1
```

Exit: `1`.

The compiler reported only missing frozen S2-W17 symbols, beginning with:

```text
internal/runtime/pi_probe_test.go:286:34: undefined: PiRuntimeProbeConfig
internal/runtime/pi_probe_test.go:305:29: undefined: PiMetadataResult
internal/runtime/pi_probe_test.go:313:56: undefined: PiMetadataRunner
internal/runtime/pi_probe_test.go:373:17: undefined: PiMetadataCommand
```

There was no syntax, unrelated package, dependency, network, filesystem,
machine-state, or environment failure.

## Controller verification

All commands below completed with exit `0` after final formatting:

```text
go test ./internal/runtime -run 'TestPiRuntimeProbe|TestParsePi' -count=1
ok   loom-pi-rebuild/internal/runtime

go test ./internal/runtime -count=1
ok   loom-pi-rebuild/internal/runtime

go test -race ./internal/runtime -run 'TestPiRuntimeProbe|TestParsePi' -count=50
ok   loom-pi-rebuild/internal/runtime

go test ./... -count=1
ok   all repository packages

go test -race ./... -count=1
ok   all repository packages

go vet ./...
exit 0

gofmt -d internal/runtime/pi_probe.go internal/runtime/pi_probe_test.go
no output

git diff --check
no output
```

The focused suite covers invalid and typed-nil runners, exact command order and
arguments, S2-W2 coordinator integration, no-model handling, mutation
isolation, cancellation before/between/after calls, no-partial runner failures,
size/UTF-8/NUL/stderr rejection, version grammar, exact headers and columns,
provider/model token bounds, positive count grammar, yes/no columns, duplicate
models, prose rejection, and row ceiling.

## Scope and import boundary

- New product: `internal/runtime/pi_probe.go`
- New tests: `internal/runtime/pi_probe_test.go`
- New S2-W17 contract/review/deliverable evidence only
- Controller status edits: bounded first hunks in `docs/CURRENT.md` and
  `PROGRESS.md`
- Product imports only `context`, `errors`, `fmt`, `reflect`, `sort`, `strings`,
  and `unicode/utf8`
- No accepted Slice 1 or S2-W1 through S2-W16 product/test file changed
- User-owned `AGENTS.md`, the `PROGRESS.md` Historical/Rejected Candidate tail,
  `.codex/`, and `.loom-drafts/` remain unstaged and unmodified by this lineage
- Branch/head remains `codex/loom-platform-slice2` at `42fc661`

## Trust-boundary report

- The runner is untrusted observation I/O and cannot select an arbitrary
  executable, environment, cwd, stdin, or argument vector through this API.
- Raw process output and runner error text are validated, discarded, and never
  included in Candidate state or returned errors.
- The probe leaves `SourceProbeID` empty; accepted S2-W2 overwrites it with the
  trusted configured probe ID.
- Online status means only that both metadata requests returned valid output.
- Metadata capabilities do not claim Pi Agent tool availability.
- Model IDs are catalog strings, never entitlement, credential, capacity, or
  model-call authority.
- No Pi process was invoked and no user Pi state was inspected or changed.

## Current gate

Fresh independent implementation review:
`.loom-evidence/phase1-slice2/S2-W17/implementation-review.md`

Result: `PASS`; no blocking findings. S2-W17 is accepted and eligible for its
strictly scoped local atomic commit. No activation is authorized.

VERDICT: PASS
