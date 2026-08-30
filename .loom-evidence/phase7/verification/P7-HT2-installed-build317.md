# P7-HT2 installed Build 317 verification

Status: `PARTIAL ACCEPTANCE / CLAUDE CANCELLATION HARDENED / USER AUTH REQUIRED`

Date: 2026-08-30

## Installed identity

Loom `0.5.6` Build 317 is the exact installed and running bundle. Candidate and
installed executable bytes are identical, and strict deep code-sign verification
passed.

| File | SHA-256 |
| --- | --- |
| `Contents/MacOS/LoomLocalApp` | `9f01acb4d38f10d47e75df9cf1edaca1525e9a8430ccd38b35d46c4e2846684c` |
| `Contents/Library/Helpers/loomd` | `52bb108c351725f22b3ad836dce0c09b439bbd9a51c9e3f11bee6bfc4346a374` |
| `Contents/Info.plist` | `7604cf7f21fcd940eac6dad4b0d902d282613270eb060cf4aa4ad9896e9f2db1` |

The installed App runs as PID `56256`. Its managed bundled daemon runs as child
PID `56281` with the canonical expanded argv, exact private state, isolation,
Socket, Runtime, local-model and managed-parent bindings.

The read-only installed identity preflight passed with the explicit exact
acceptance anchor:

- 24 Providers;
- seven Runtimes;
- six executable Conversation Profiles;
- Claude Code Runtime online with capacity three;
- Mission `mission/team-instance-643ace1b858f20a1d7b60f2ccd90d6a1`;
- Team `team-instance-643ace1b858f20a1d7b60f2ccd90d6a1`;
- seven RoundTable navigation records.

A separate invocation with an intentionally incorrect expected daemon digest
failed at bundle identity admission before product state was read.

## Closed review findings

Build 317 closes the cancellation-ordering finding left after Build 316:

1. Every Swift UDS request now runs in a cancellation-aware worker. Cancelling
   the caller shuts down the exact live descriptor under a lock, interrupting a
   blocked connect, send or receive; the worker retains ownership of final
   `close`, so cancellation cannot close a later reused descriptor.
2. Claude sign-in cancellation increments a generation before cancelling the
   old polling task, sends `claude_code_cancel` before awaiting that task and
   owns the in-flight state until daemon cancellation returns. A stale task can
   no longer clear or overwrite the cancellation result.
3. Runtime & Providers distinguishes `Waiting for sign in` from
   `Cancelling sign in`. Only the Claude row renders this activity; cancellation
   removes the duplicate action while retaining progress and exact Incident
   identity.
4. A blocked Setup-poll contract proves the daemon cancellation request is
   admitted before the old poll is released. A concrete private-UDS contract
   separately proves a cancelled blocked exchange returns within one second.
5. The real Go-to-Swift protocol probes now use Swift debug compilation for
   source-test latency only. Strict decoding and the real probe executable are
   unchanged, while the daemon probe fell from `631.46s` to `53.75s` and the
   local IPC probe from `192.25s` to `4.06s`. Release App packaging still uses
   two independent release builds.

## Verification

```text
swift test --package-path apps/macos
496 XCTest cases, 2 conditional skips, 0 failures
21 strict Swift Testing contract cases, 0 failures

go test ./... -count=1
PASS
cmd/loomd          87.442s
internal/localipc  61.643s

go vet ./...
PASS

go test -race ./internal/runtime/harnessadapter ./internal/app \
  ./internal/api ./internal/localipc -count=1
PASS

go test -race ./cmd/loomd -run \
  'Test(COMP2BRouteManifestFreezesEveryLegacyMethod|COMP2CSetupConstructsInLocalIPCBundleAndRevokesRoute|ProductDaemonServesStrictSetupSnapshotWithoutCLIOrSQLiteClient)$' \
  -count=1
PASS

git diff --check
PASS

scripts/test-build-loom-local-app.sh
PASS (two independent release builds, deterministic bundle and strict signing)

scripts/test-install-loom-local-app.sh
PASS (replacement, rollback, injected failure and symlink-attack fixtures)

scripts/install-loom-local-app.sh --dry-run ...
PASS

TestPhase7InstalledIdentityPreflight
PASS (build=317, providers=24, runtimes=7, profiles=6)
```

The first repository-wide Go attempt was not rewritten as a pass: two packages
reached their ten-minute limit while separate release-mode Swift probes were
compiled under load, and unrelated subprocess fixtures also observed load
pressure. The affected fixtures passed in isolation. After changing only the
test probe build configuration, the complete default command above passed.

Two delegated review attempts produced no findings because the local CC Switch
proxy returned HTTP 502 before either reviewer started. They are not counted as
successful reviews. A main-thread review of the descriptor lifecycle, task
generation, daemon-cancel ordering, UI state ownership and probe configuration
found no remaining issue in this Build 317 slice.

## Installed visual acceptance

Three real `720 x 793` Runtime & Providers captures cover the Runtime list,
CC Switch imports and the complete Provider list. Claude is visibly
`Sign In Required`, its `Sign In` action is available, all seven Runtimes remain
distinct, long OpenCode model labels are bounded, and Provider status/action
rows remain readable without overlap. The captures contain only non-secret
setup metadata.

| Artifact | SHA-256 |
| --- | --- |
| `P7-HT2-installed-build317-runtime-provider.png` | `7d666339b63a9bebb5e1e7d57137cde2c7eac76a72b7f7797927315819b04487` |
| `P7-HT2-installed-build317-providers.png` | `9f45b9b3b8cece167649f0a5688523692ace77fe667a0cd66562bf091ad39b6c` |
| `P7-HT2-installed-build317-providers-bottom.png` | `4b0ebaafb3b0db8830e1b909084aed5007baadb5c96981a58213a428415a8b40` |

## Remaining gate

The user did not click `Sign In` during this gate. No credential was read,
imported, replaced, revoked or written, and Build 317 made no paid Provider
request. Setup correctly publishes no Claude Code Conversation Profile.

Build 313 remains the last explicitly authorized paid model matrix. Phase 7
remains `PARTIAL` until the user completes official Claude browser login, the
installed daemon publishes a Claude Code Profile without restart, and the exact
five-Runtime installed matrix passes tool selection, Proposal governance,
restart restoration and privacy checks.
