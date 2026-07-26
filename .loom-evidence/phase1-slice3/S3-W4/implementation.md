# S3-W4 Implementation Evidence

- Date: `2026-07-26`
- Baseline: `47b4b50`
- Contract:
  `.loom-evidence/phase1-slice3/S3-W4/contract.md`
- Amendments:
  `.loom-evidence/phase1-slice3/S3-W4/contract-amendment-1.md`,
  `.loom-evidence/phase1-slice3/S3-W4/contract-amendment-2.md`,
  `.loom-evidence/phase1-slice3/S3-W4/contract-amendment-3.md`

## Delivered boundary

The Candidate closes one vertical managed-execution boundary:

- private bounded source copy and managed workspace with deterministic
  manifests and immutable change capture;
- Unix descriptor-rooted source/final-workspace traversal using
  `openat`/`fstatat`, no-follow opens, single-link regular files, and exact
  pre/open/post device, inode, type, link, mode, and size proof;
- configured Pi stdio adapter with exact executable/search-path/environment
  binding, Bridge session validation, output bounds, caller-owned output
  pipes, and Unix process-group cleanup;
- Supervisor validation, Run start/terminal authority, per-frame Grant
  authorization, source-change handling, revocation, and cleanup;
- real deterministic temporary executable and SQLite reopen evidence without
  installed Pi, user state, credentials, network, model calls, daemon, or
  Runtime activation.

No public symbol beyond the frozen API, dependency, Event schema, StateWriter,
projection authority, daemon, scheduler, shell, network, Provider/model,
credential, or Phase 2 capability was added.

## RED and repairs

- Mandatory product RED:
  `.loom-evidence/phase1-slice3/S3-W4/red.md`
- Amendment 2 intermediate-directory replacement RED:
  `.loom-evidence/phase1-slice3/S3-W4/red-amendment-2.md`
- Verification Repair 1 readiness RED:
  `.loom-evidence/phase1-slice3/S3-W4/verification-repair-1-red.md`
- Verification Repair 2 pipe/Wait RED and independent analysis:
  `.loom-evidence/phase1-slice3/S3-W4/verification-repair-2-red.md`,
  `.loom-evidence/phase1-slice3/S3-W4/verification-repair-2-analysis.md`
- Implementation Review 1 and Repair 1 RED:
  `.loom-evidence/phase1-slice3/S3-W4/implementation-review-1.md`,
  `.loom-evidence/phase1-slice3/S3-W4/implementation-repair-1-red.md`
- Implementation Repair 2 and independent analysis:
  `.loom-evidence/phase1-slice3/S3-W4/implementation-repair-2.md`,
  `.loom-evidence/phase1-slice3/S3-W4/implementation-repair-2-analysis.md`

Repair 1 made the cancellation proof wait for the grandchild PID readiness
signal before explicit cancellation. Repair 2 replaced prohibited concurrent
`StdoutPipe`/`StderrPipe` drain versus `Cmd.Wait` with caller-owned `os.Pipe`
readers. Focused fast-exit/config plus cancellation/grandchild tests then
passed `-race -count=30` in 124.202s.

Implementation Repair 1 rejects raw Grant substrings in source manifest and
returned change paths as well as contents. The fuzz harness now returns when a
platform rejects fixture filename creation before product invocation.
Amendment 3 independently reviewed and permitted one accepted S2-W18 test-only
change: the shared non-timeout metadata fixture bound is 10s, while its
dedicated timeout proof remains 100ms. The impacted subtest passed focused
`-race -count=100` in 50.131s; timeout/cancellation/process-group tests passed
`-race -count=30` in 49.754s.

Repair 2 closes the repeated test-readiness race by publishing the grandchild
PID through same-directory temporary write, close, and atomic rename; the
reader accepts only a parsed positive PID and bounded-cleans every failure.
The exact cancellation/grandchild case passed `-race -count=100` in 130.480s.

## Exact verification

```text
go test ./internal/supervisor ./internal/runtime/piadapter -count=1
PASS

go test -race ./internal/supervisor ./internal/runtime/piadapter -count=30
Final pre-commit:
ok loom-pi-rebuild/internal/supervisor 103.432s
ok loom-pi-rebuild/internal/runtime/piadapter 541.156s

go test ./internal/supervisor ./internal/runtime/piadapter -cover -count=1
supervisor: 81.3%
piadapter: 82.7%

go test ./... -count=1
PASS

go test -race ./... -count=1
PASS

go vet ./...
PASS

go test ./internal/supervisor -run '^$' \
  -fuzz '^FuzzManagedWorkspaceAndSessionNeverPanic$' -fuzztime=5s
PASS; 8,207 executions; 33 new interesting inputs
Latest Repair 1 rerun: PASS; 20,185 executions; 20 new interesting inputs
Latest Repair 2 rerun: PASS; 20,862 executions; 22 new interesting inputs
Final pre-commit rerun: PASS; 12,013 executions; 4 new interesting inputs

gofmt -d <all ten S3-W4 owned Go files>
PASS; no output

git diff --check
PASS
```

Additional checks:

- all eight mandatory RED markers occur exactly once;
- static export/import/capability boundary test passes;
- `StdoutPipe` and `StderrPipe` are prohibited by static regression;
- Linux amd64 Supervisor and Pi adapter test binaries compile;
- Windows amd64 Supervisor and Pi adapter test binaries compile and use the
  non-Unix fail-closed path;
- no `go.mod` or `go.sum` change;
- installed Pi, user Pi state, credentials, network, package manager, daemon,
  Agent session, prompt, model call, and Runtime activation were not used.

## Security self-audit

- Top-level `.git` is skipped from the bound root listing before any child
  stat or open.
- Manifest paths come only from validated descriptor-enumerated names.
- Symlink, hardlink, device, socket, FIFO, identity mutation, oversized entry,
  and unsupported platform cases fail closed with typed errors.
- Output readers are owned and closed by the adapter, so `Wait` cannot truncate
  stdout/stderr; a fast-exit near-limit legal stderr regression verifies full
  drain.
- Raw Grant tests cover args, protocol bytes, stderr, returned result,
  workspace/source, errors, Events, and reopened SQLite.
- User-dirty `AGENTS.md`, historical `PROGRESS.md` tail, `.codex/**`, and
  `.loom-drafts/**` remain outside the Candidate.

Fresh independent Implementation Repair 2 Review returned `PASS` with no
findings. The fresh final pre-commit matrix also passed. Current gate: exact
Candidate staging and authorized local atomic commit.

VERDICT: PASS
