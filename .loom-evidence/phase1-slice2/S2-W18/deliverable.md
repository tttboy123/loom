# S2-W18 Candidate Deliverable

- WorkItem: `S2-W18`
- Title: Isolated Local Pi Metadata Process Runner
- Risk: Strict
- Candidate state: `ACCEPTED`
- Branch/head: `codex/loom-platform-slice2` at `b73cf8b`
- Active contract SHA256:
  `cb268df2a6488ae2c194aa0fe9e104f974624e60183637141e5fac6526d359ed`
- Contract Amendment 2 SHA256:
  `93a3ec1a4d54ee72958317080cd8be3b12c1b3f4ab86475cf3cc91da3a6bfeb3`

## Candidate

The Candidate adds the concrete S2-W17 `PiMetadataRunner` implementation only
in the amended child package `internal/runtime/piadapter`. It:

- binds one explicitly configured absolute executable by resolved path, file
  identity, executable permission, size ceiling, and SHA-256 digest;
- binds one private `0700` isolation root and ordered deduplicated runtime
  search-directory identities;
- accepts only the exact frozen version and model-list requests;
- starts the configured resolved executable directly through `os/exec`, never
  through a shell or PATH lookup;
- supplies only the frozen environment allowlist and fresh private
  HOME/agent/session/tmp directories;
- bounds stdout and stderr independently to 256 KiB;
- applies the configured timeout, a one-second wait delay, and Unix
  original-process-group cancellation;
- cleans the exact per-call directory and preserves both operation and cleanup
  errors through `errors.Join`; and
- exposes only typed, non-disclosing errors and zero output on failure.

The accepted parent `internal/runtime` package remains the pure domain/port
boundary. No accepted S2-W1 through S2-W17 product or test file changed.

## RED and architecture amendment

The mandatory RED command was:

```text
go test ./internal/runtime/piadapter -run 'TestPiMetadataProcessRunner' -count=1
```

It exited `1` only because the frozen S2-W18 constructor, configuration,
sentinels, and runner symbols did not yet exist. There was no syntax,
dependency, unrelated-test, or environment failure.

The first implementation location then exposed an accepted architecture
boundary during the strict matrix:

```text
TestRuntimeImportBoundaryStaysPureDomain
pi_process_other.go imports forbidden boundary package "os/exec"
```

No product repair was counted. Contract Amendment 2 moved the concrete adapter
to `internal/runtime/piadapter`; fresh independent Contract Review 3 returned
`PASS` before the amended paths were used.

## Final file digests

```text
45bd8520351955205bb2b9b4651fa8d7c472677c1056422858d2c6e4dd97a65f  internal/runtime/piadapter/process_runner.go
554e828a5667dcfae9a12f6eeecc4d1ecc366dc5f0ed0a1ef39a112b4aacd140  internal/runtime/piadapter/process_unix.go
ea9a2da14e6e6b8de82cfbacdf6e9377e95de130997ae7ef3ee6769d9edc3e7a  internal/runtime/piadapter/process_other.go
897c4f490eaf96b47f2560e4ca9e6221421f701c6f9edda2bca57cfb04de25e5  internal/runtime/piadapter/process_runner_test.go
```

## Controller verification

All checks ran from the repository root on 2026-07-25:

| Check | Result |
|---|---|
| `go test ./internal/runtime/piadapter -run 'TestPiMetadataProcessRunner' -count=1` | `PASS`, `ok ... 3.646s` |
| `go test ./internal/runtime ./internal/runtime/piadapter -count=1` | `PASS`, both packages green |
| `go test -race ./internal/runtime/piadapter -run 'TestPiMetadataProcessRunner' -count=20` | `PASS`, `ok ... 78.418s` |
| `go test ./... -count=1` | `PASS`, all repository packages green |
| `go test -race ./... -count=1` | `PASS`, all repository packages green |
| `go vet ./...` | `PASS` |
| changed-file `gofmt` check | `PASS`, no output |
| `git diff --check` | `PASS`, no output |
| branch/head check | `PASS`, `codex/loom-platform-slice2` at `b73cf8b` |

The final focused suite covers invalid configuration, input copying and search
path normalization, exact request rejection before state creation, environment
and directory isolation, executable symlink immunity, executable/root/search
identity drift, the documented interpreter-in-directory residual, S2-W17/S2-W2
integration, cancellation, timeout, same-group child termination, nonzero exit,
stdout/stderr overflow, cleanup-only failure, joined cleanup failures, zero
failure results, and error non-disclosure.

Static scope review confirms:

- concrete `os/exec` imports exist only in `internal/runtime/piadapter`;
- parent `internal/runtime` production files do not import a concrete process
  package;
- product code uses `exec.CommandContext` with the stored resolved executable,
  not `exec.Command`, `LookPath`, `os.Environ`, or a shell;
- the adapter imports no Journal, projection, state, Team, Agent, Evidence,
  database, network, UI, credential, or provider package; and
- only deterministic temporary shell fixtures were executed in tests.

## Trust and residual limitations

The higher trusted layer still owns genuine-Pi selection, daemon-owned root
selection, and trusted interpreter/runtime search paths. The digest and identity
checks do not eliminate the final validation-to-`execve` race. Search-directory
identity does not bind interpreter bytes replaced within the same directory.
Unix cleanup covers only descendants that remain in the original process group.
The fixed offline flags express Pi-level intent but are not an OS network
sandbox. Output remains untrusted Candidate data for S2-W17 parsing and S2-W2
rebinding.

No installed Pi command, user Pi directory, credential, provider environment,
network, package manager, dependency installation, daemon, Agent session,
prompt, model call, Runtime activation, push, merge, release, or external
mutation was used.

## Independent implementation review

The fresh independent read-only implementation Reviewer found no blocking
issues. It independently matched the contract and file hashes, checked every
frozen process/isolation boundary, and reran focused, package, focused race,
repository, repository-race, vet, format, and diff checks successfully.
Review evidence is in `implementation-review.md`.

VERDICT: PASS
