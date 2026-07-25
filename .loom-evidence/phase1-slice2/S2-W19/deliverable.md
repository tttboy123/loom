# S2-W19 Candidate Deliverable

- WorkItem: `S2-W19`
- Title: Configured Local Pi Probe Factory
- Risk: Strict
- Candidate state: `ACCEPTED`
- Branch/head: `codex/loom-platform-slice2` at `8c8fb9e`
- Contract SHA256:
  `fdf5cfad9c8f1c28fe0817d2f3bc98604a7dc78cca3d73688150c9763ac22b12`

## Candidate

The Candidate adds only the concrete child-package factory frozen by S2-W19.
It:

- copies stable probe/instance/device/display identities;
- binds the S2-W18 private isolation root, timeout, and ordered canonical search
  directories;
- revalidates directory identities, types, root mode, and search-directory
  permission modes before scanning;
- inspects only the fixed current-upstream name `pi` directly beneath each
  configured directory;
- treats complete absence as `(nil, false, nil)`;
- fails closed on a present invalid first candidate;
- constructs the accepted S2-W18 runner and S2-W17 probe for the first valid
  candidate without starting a process; and
- returns only typed non-disclosing errors.

The parent `internal/runtime` package and all accepted S2-W1 through S2-W18
files remain unchanged.

## Mandatory RED

The command:

```text
go test ./internal/runtime/piadapter -run 'TestPiLocalRuntimeProbeFactory' -count=1
```

exited `1` only on undefined frozen S2-W19 factory, configuration, request, and
typed-error symbols. There was no syntax, dependency, environment, or unrelated
failure.

## Final file digests

```text
d6eec967acdea10e4e7ce1b9ff24e2971f2b32d6a4b658c138477ff2db66cbb1  internal/runtime/piadapter/local_probe.go
5c823ed85d796ad8a07a92010e2f641819a24ce0ed195759049b4a2be94b27c1  internal/runtime/piadapter/local_probe_test.go
```

## Controller verification

All checks ran from the repository root on 2026-07-25:

| Check | Result |
|---|---|
| `go test ./internal/runtime/piadapter -run 'TestPiLocalRuntimeProbeFactory' -count=1` | `PASS`, `ok ... 1.289s` |
| `go test ./internal/runtime ./internal/runtime/piadapter -count=1` | `PASS`, both packages green |
| `go test -race ./internal/runtime/piadapter -run 'TestPiLocalRuntimeProbeFactory' -count=20` | `PASS`, `ok ... 7.178s` |
| `go test ./... -count=1` | `PASS`, all repository packages green |
| `go test -race ./... -count=1` | `PASS`, all repository packages green |
| `go vet ./...` | `PASS` |
| changed-file `gofmt` check | `PASS`, no output |
| `git diff --check` | `PASS`, no output |
| branch/head check | `PASS`, `codex/loom-platform-slice2` at `8c8fb9e` |

The final focused suite proves:

- complete invalid-constructor coverage for identities, timeout, root, and
  search paths;
- copied and ordered-deduplicated canonical search bindings;
- nil/canceled/deadline request behavior;
- successful absence without process or invocation state;
- fixed-name-only, non-recursive, configured-order selection that ignores
  ambient PATH, aliases, and nested candidates;
- invalid directory, permission, size, broken-link, and portable I/O candidates
  fail closed without selecting a later executable;
- root/search identity and permission drift rejection;
- package-manager-style external-target symlink support and post-construction
  retarget immunity;
- no process at factory construction, followed by explicit fake-only
  S2-W17/S2-W18/S2-W2 integration and complete invocation cleanup; and
- typed-error non-disclosure with no raw `*os.PathError`.

Static review confirms `local_probe.go` contains no `exec.LookPath`, shell,
command start, environment lookup/inheritance, recursive walk/glob, network,
database, Journal, projection, UI, credential, daemon, or parent-package
concrete dependency.

## Trust and residual limitations

The higher trusted daemon/config layer still owns search-directory order,
private-root selection, public identities, symlink provenance, and interpreter/
runtime path integrity. First-match semantics cannot make a valid malicious
binary in a trusted earlier directory safe. S2-W18 binds the resolved candidate
but retains its acknowledged validation-to-exec and interpreter-in-directory
risks. Factory construction proves only that a probe can be built; explicit
metadata observation still yields untrusted Candidate data for S2-W17 parsing
and S2-W2 revalidation.

No installed Pi command, user Pi state, credential, provider environment,
network, package manager, dependency installation, daemon, Agent session,
prompt, model call, Runtime activation, push, merge, release, or external
mutation was used. The only executed candidate was a deterministic temporary
fake in the explicit integration test.

## Independent implementation review

The fresh independent read-only implementation Reviewer found no blocking
issues. It matched the contract and file hashes, inspected every frozen
selection/isolation boundary, and independently passed focused, package,
focused race, repository-race, vet, format, and diff checks. One repository
non-race run performed concurrently with repository-race failed transiently;
its isolated rerun and two ten-run reproductions passed. Full detail is
preserved in `implementation-review.md`.

VERDICT: PASS
