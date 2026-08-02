# P2A-W3 Native Launcher and Build Transaction Closure Contract

**Date**: 2026-08-02  
**Status**: `FROZEN / PENDING CONTRACT REVIEW`  
**Risk**: `STRICT`  
**Baseline commit**: `848f068cbc0307f14473f6a71961db949a8734ca`  
**Parent**: unique `P2A-W3 Controlled Execution Experience`  
**Result Review SHA-256**:
`4b7fd47018fa65a1db0b6ad8c4c0d147158913356ef4c73472a4623a882809aa`  
**New WorkItem**: none; `P2A-W4` does not exist

## 1. Authorization and stop-state basis

The Product Owner instructed the Controller to reprocess the failed live
results. Independent Result Review accepted their evidence, classified the
common failure as
`product_defect / codex_native_auth_symlink_identity_during_setup_construction`,
and permitted one new governed P2A-W3 repair boundary.

This contract does not reinterpret or reuse the consumed `minimax-003` and
`pi-003` lineages. It closes launcher identity, complete build attribution and
live-shaped product construction together; it is not a retry-only or
single-wrapper amendment.

## 2. Binding invariants

- Event Journal remains the only state authority; Projection remains a
  rebuildable cache.
- No Event schema, StateWriter, Rules, Supervisor, Grant, Evidence, Pi RPC,
  scheduler, queue, Team, Mission or Provider authority changes.
- Raw credentials, Grant data, paths, command output and private errors never
  enter public stderr, IPC, Journal or evidence.
- Codex OAuth remains Codex-owned native authentication. Loom may observe and
  open the login controller; it must not persist or proxy OAuth tokens.
- A symlink is never executed or trusted as an identity. Any supported launcher
  is resolved to one canonical regular executable, then the existing file and
  parent identity fencing applies before each process start.
- The unrelated `demo-resident` daemon remains untouched.

## 3. Boundary A — supported Codex launcher resolution

The product must accept both:

1. an absolute, clean, executable regular Codex native binary; and
2. the ordinary absolute user-level npm `codex` symlink for the installed
   official `@openai/codex` package.

For the npm form, resolution is data-only and fail-closed:

- resolve the supplied symlink chain without executing JavaScript, Node, npm,
  shell, PATH search, network or package hooks;
- require the canonical wrapper to be the executable regular
  `@openai/codex/bin/codex.js` shape;
- derive only the current OS/architecture's fixed official optional-package
  native binary beneath the same canonical package tree, including the
  package-local fallback layout used by the official launcher;
- canonicalize the derived binary and require an absolute clean executable
  regular file;
- return only the native binary path to the existing observer and login
  controller so their current identity revalidation remains binding;
- reject dangling/looping symlinks, arbitrary scripts, wrong package scope,
  missing or non-regular native target, directory replacement, unsupported
  OS/architecture, and any identity drift.

No ambient `node` or PATH fallback is permitted. Resolving merely to
`codex.js` is insufficient because the existing auth subprocess environment is
intentionally empty and cannot safely discover a user-level Node interpreter.

Mandatory REDs cover direct native success, official npm symlink success,
package-local fallback success, wrong scope/layout, arbitrary script, dangling
and looping link, final-target symlink/non-regular target, replacement between
resolution and execution, and immutable caller input.

## 4. Boundary B — complete safe build-stage attribution

Every product-daemon construction failure must map to exactly one allowlisted
public reason without exposing the private error:

```text
build_observer
build_state
build_setup_runtime
build_setup_credential
build_setup_provider
build_setup_native_auth
build_decision
build_execution
build_ipc
build_unknown
```

Requirements:

- stage wrappers use typed identity, never string matching;
- one unique known stage produces its exact code;
- unknown, known-plus-unknown, or multiple incompatible stages produce
  `build_unknown`;
- `run()` keeps exit code `3` and emits exactly one line
  `daemon unavailable: <reason>`;
- raw paths, underlying text, helper output, command output and joined private
  leaves remain unavailable;
- Runtime observer failures after successful construction retain the existing
  `observer_*` taxonomy and exit code `4` unchanged;
- no retry, fallback, compaction, alternate builder or second socket is added.

The prior generic `daemon unavailable` behavior is preserved only as
`build_unknown`; it may not erase a unique known construction stage.

## 5. Boundary C — live-shaped construction transaction

One deterministic component matrix must exercise the actual product path, not
call an inner handler directly:

```text
run/config
  -> productionDaemonBuilder
  -> Runtime observer construction
  -> LocalProductReadService
  -> setup + canonical Codex native auth
  -> optional mission execution composition
  -> real Go local IPC server/client
  -> bounded shutdown and cleanup
```

Required proofs:

1. Decode the self-contained exact retained Pi six-fact SQLite fixture and use
   a deterministic official-npm-layout Codex launcher fixture, deterministic Pi
   0.82.1 metadata fixture, and deterministic local-model catalog fixture.
2. Enable the local-model execution config so the test crosses the previously
   omitted `buildProductMissionExecutionAPI` branch.
3. Publish the private product socket, serve one strict snapshot and one
   zero-write saved-Team preflight over the real Go IPC client, and then close
   cleanly.
4. Prove all six authority facts and final SQLite bytes are unchanged, no
   Mission/Run/Grant/Frame/Evidence fact exists, and no hidden process/retry or
   socket/lock residue remains.
5. A setup-only MiniMax construction fixture omits all three local-model flags,
   reaches the real IPC server, and proves that a credential-only product Test
   surface does not initialize execution authority or create execution dirs.
6. Every frozen build reason is injected at the real builder/run boundary and
   yields exact one-line attribution with zero public private-data leakage.

## 6. Exact owned files

Only these production and test files may change after Contract Review PASS:

```text
internal/provider/codex_native_auth.go
internal/provider/codex_native_auth_test.go
cmd/loomd/product_daemon.go
cmd/loomd/product_daemon_test.go
cmd/loomd/run.go
cmd/loomd/run_test.go
```

Governance/evidence may change only under:

```text
.loom-evidence/phase2a/P2A-W3/
docs/CURRENT.md
```

Swift, IPC protocol, Journal, Projection, Credential Broker/Keychain helper,
Provider verifier, Execution API/Application, Supervisor, Work, Grant,
Evidence, Pi adapter and all P2A-W1/W2 files remain locked. If causal RED proves
one is required, stop `HUMAN_REQUIRED`; do not silently expand scope.

## 7. Verification gates

In order:

1. immutable contract hash and fresh independent Contract Review PASS;
2. causal RED before behavior change;
3. focused provider/daemon/run tests, repeated tests and race tests;
4. retained-state live-shaped setup-only and execution-enabled real IPC tests;
5. complete serialized `go test -count=1 -p 1 ./...` and
   `go test -count=1 -race -p 1 ./...`;
6. `go vet ./...`, module tidy diff, format, diff, scope, locked-authority,
   protocol, secret and raw-path/non-disclosure checks;
7. existing Swift normal, ThreadSanitizer and arm64 Release gates unchanged;
8. immutable implementation source lock and fresh independent Implementation
   Review PASS.

Review must fail for executing a launcher during resolution, ambient PATH/Node
fallback, accepting arbitrary symlinks/scripts, weakened identity revalidation,
generic masking of a known build stage, raw error/path disclosure, incomplete
execution-enabled construction coverage, authority mutation before Start,
hidden retry, or locked-file drift.

## 8. Live allowance after Implementation Review PASS only

No live action is authorized by contract freeze or Contract Review.

After Implementation Review PASS, at most two wholly new manifests may be
frozen:

1. one setup-only MiniMax explicit Test lineage with no local-model execution
   flags and at most one Provider request; and
2. one Pi saved-Team controlled-execution lineage with the exact installed Pi
   0.82.1, local model, official user-level npm Codex launcher, and zero network
   Provider requests.

Each needs a fresh 0700 root, 0600 pre-created SQLite and manifest, exact
source/artifact/external hashes, one daemon start, no retry/compaction, complete
raw start-result capture and postflight, and an independent Result Review. A
failed lineage is consumed. No third attempt may be improvised.

## 9. Exit condition

P2A-W3 can be accepted only after Contract Review, causal RED, implementation,
all deterministic gates, independent Implementation Review, both new live
results, independent Result Review, final no-terminal walkthrough, final lock,
and one atomic W3 commit all PASS.

Until then:

```text
P2A-W3 = HUMAN_REQUIRED / NATIVE LAUNCHER AND BUILD TRANSACTION CONTRACT PENDING REVIEW
P2A-W4 = DOES NOT EXIST
NO LIVE / NO WALKTHROUGH / NO COMMIT
```
