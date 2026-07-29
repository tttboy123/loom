# P2A-W2 Live Gate and Deterministic Checkpoint Commit Amendment

**Date**: 2026-07-29
**Status**: CANDIDATE — awaiting fresh independent Contract Review
**Parent contract**: `P2A-W2 Team Builder and Provider Onboarding`
**Product Owner authorization**: `全部授权，并进行提交`

## 1. Why this one bounded amendment exists

The frozen W2 contract requires a separately frozen live gate and puts the
local atomic commit after a successful controlled live result. It also
explicitly prohibits using the MiniMax secret previously pasted into chat.

No fresh user-entered MiniMax secret is available to the Controller. Loom cannot
manufacture a Provider credential, recover one from chat, shell history,
environment, configuration, or caches, or silently reuse the prohibited value.
That is a required live input rather than an additional authorization.

The Product Owner has now explicitly authorized all remaining in-scope actions
and requested a commit. This amendment therefore does exactly two things:

1. freezes the one W2 controlled live gate without executing it while its fresh
   credential precondition is absent;
2. permits one atomic deterministic-checkpoint commit after this amendment
   receives fresh independent Contract Review `PASS`.

It does not accept W2, claim live delivery, unlock P2A-W3, create P2A-W4, or
weaken any credential, authority, or execution boundary.

## 2. Exact frozen live manifest

### Candidate and source identity

- baseline commit:
  `7c1c469c46c97eac0cab39a756b2d59867a49563`;
- frozen W2 contract SHA-256:
  `f2e7a4d27866da4ca5f08623db6d1082587f8fd4e986f3c39f1d839dd0609a55`;
- exact W2 product/test Merkle SHA-256:
  `bd11f85b1b46cfd4927131484f77e8dff36afd9b5793d18ef061aec6b72a4dac`;
- exact source lock:
  `.loom-evidence/phase2a/P2A-W2/live-source-lock.json`.

Any source, toolchain, Codex target, or Pi target mismatch stops before build,
Keychain, Codex execution, network, daemon, app, or Journal mutation.

### Attempt identity and paths

Exactly one later controlled attempt may use:

```text
attempt_id        = p2a-w2-live-20260729-001
attempt_root      = /Users/lune/Library/Application Support/Loom/phase2a-w2-live-20260729-001
state_path        = <attempt_root>/state/loom.db
isolation_root    = <attempt_root>/isolation
manifest_copy     = <attempt_root>/manifest/live-gate-and-checkpoint-amendment.md
source_lock_copy  = <attempt_root>/manifest/live-source-lock.json
daemon_binary     = <attempt_root>/bin/loomd
native_app_binary = <attempt_root>/bin/LoomLocalApp
product_socket    = /Users/lune/Library/Application Support/Loom/run/loomd.sock
```

- the attempt root and every directory below it must be newly created,
  user-owned, non-symlink, and mode `0700`;
- ordinary files, SQLite files, logs, manifest copies, and source-lock copies
  must be user-owned, regular, non-symlink files with mode `0600`;
- executables must be user-owned, regular, non-symlink files with mode `0700`;
- the default socket parent may be created only if its exact pre-state is
  absent; it must be user-owned, non-symlink, and mode `0700`;
- the socket must be the one private AF_UNIX socket at the exact default native
  product path, owned by the effective user and mode `0600`;
- an existing socket, socket parent with unexpected identity, resident product
  daemon, or unexpected path stops before mutation.

### Exact build and Runtime inputs

The later attempt may build only:

```text
go build -trimpath -buildvcs=false -o <attempt_root>/bin/loomd ./cmd/loomd
swift build --package-path apps/macos --scratch-path <attempt_root>/swift-build -c release
```

The exact release `LoomLocalApp` executable may then be copied byte-for-byte to
`<attempt_root>/bin/LoomLocalApp`. Build outputs must be hashed and recorded
before execution.

The daemon invocation is frozen to:

```text
<attempt_root>/bin/loomd
  --state <attempt_root>/state/loom.db
  --isolation-root <attempt_root>/isolation
  --runtime-dir /Users/lune/Library/Application Support/Loom/runtimes/pi/0.82.1/node_modules/.bin
  --probe-id p2a-w2-live-probe-001
  --instance-id runtime.p2a-w2-live.pi.0.82.1
  --device-id device.p2a-w2-live.local
  --display-name "Pi 0.82.1 W2 Canary"
  --interval 30s
  --process-timeout 10s
  --max-cycles 0
  --socket /Users/lune/Library/Application Support/Loom/run/loomd.sock
  --codex-executable /Users/lune/Documents/Codex/devtools/npm/bin/codex
```

The daemon receives a credential-clean, minimal environment. No Provider key,
base URL, proxy override, chat value, or ambient Loom state is copied into it.

### Product journey

After exact preflight and only after a fresh MiniMax secret is physically
entered in the native app's `MiniMax API key` SecureField, the one attempt may:

1. launch the exact native app binary against the exact default private socket;
2. observe Codex only through the exact `codex login status` path;
3. display the discovered locked Pi Runtime/model and `native_auth`;
4. store the fresh MiniMax secret in Loom's fixed Keychain service;
5. perform exactly one `Test` action, producing at most one non-generative
   `GET https://api.minimaxi.com/v1/models` request with redirects, proxy, retry,
   and compression disabled;
6. start one blank Candidate Team Builder from the visible `Create team`
   control;
7. answer exactly one visible question at a time, inspect the exact preview, and
   explicitly confirm once;
8. prove the result is one active isolated TeamDefinition and no TeamInstance,
   AgentInstance, WorkItem, Run, Grant, Evidence, dispatch, or generation
   Provider request;
9. revoke the canary MiniMax credential through the product, proving Keychain
   deletion and a safe revoked Journal fact;
10. stop the native app and daemon, preserve only secret-negative result
    evidence, and remove the exact socket, socket parent if it was initially
    absent, and attempt root after Result-Evidence Review.

The native app generates the TeamDefinition ID. The user never types or sees an
internal ID, socket path, SQLite path, cursor, service command, or credential
reference.

### Consumption and stop rules

- Live allowance is consumed only when a fresh secret has been entered and the
  user activates `Store securely`.
- Absence of a fresh credential leaves the live result `HUMAN_REQUIRED`,
  consumes no live allowance, and permits no partial Codex, daemon, app,
  Keychain, network, or Journal canary.
- Provider rejection, timeout, network failure, Keychain denial, source drift,
  Runtime incompatibility, UI failure, unexpected Event, rollback failure, or
  secret exposure stops the sole attempt with no retry or alternate path.
- A consumed attempt requires fresh independent Result-Evidence Review whether
  it passes or fails.

At amendment freeze time the fresh credential precondition is absent.
Therefore no live action is authorized by fact until the user later supplies a
new credential through the product UI.

## 3. Deterministic checkpoint commit exception

After fresh independent Contract Review `PASS`, the Controller may create
exactly one local atomic commit containing:

- the reviewed W2 product source and tests;
- the frozen W2 contract, Contract Review, RED, deterministic verification,
  independent Implementation Review, this amendment, its source lock and
  Contract Review;
- the W2 status update in `docs/CURRENT.md`.

The commit message must identify the result as a deterministic W2 checkpoint,
not accepted live delivery.

Before staging, the Controller must reproduce:

- exact W2 owned-file scope;
- the source-lock Merkle digest;
- `git diff --check`;
- complete Go tests and repository race tests;
- Go vet/module/format gates;
- Swift debug tests, release build, and thread-sanitizer tests;
- empty staging and secret-negative scans.

Staging must name only the exact W2 owned files and W2 evidence. All pre-existing
Phase 1 evidence changes, `AGENTS.md`, `PROGRESS.md`, `.codex/`, `.loom-drafts/`,
Swift build products, and every other excluded path remain untouched and
unstaged.

## 4. Status after checkpoint commit

The checkpoint commit does not satisfy sections 16 or 17's live exit:

```text
P2A-W2 deterministic implementation = REVIEWED CHECKPOINT
P2A-W2 controlled live result       = HUMAN_REQUIRED
P2A-W2 acceptance                    = NOT ACCEPTED
P2A-W3                               = LOCKED
P2A-W4                               = DOES NOT EXIST
```

When a genuinely fresh credential is later entered through the native product,
the exact frozen live gate above may resume without another source change. Any
source or manifest change requires review of this same W2 boundary, not a new
WorkItem.
