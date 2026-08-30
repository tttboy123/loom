# P7-HT2 installed Build 316 verification

Status: `PARTIAL ACCEPTANCE / SETUP INVENTORY AND CLAUDE RECOVERY HARDENED / USER AUTH REQUIRED`

Date: 2026-08-30

## Installed identity

Loom `0.5.6` Build 316 is the exact installed and running bundle. Candidate and
installed executable bytes are identical, and strict deep code-sign verification
passed.

| File | SHA-256 |
| --- | --- |
| `Contents/MacOS/LoomLocalApp` | `78235b818f36aa64756e3977cfc61914f9af22b526d1b459a3125fbd29bfc455` |
| `Contents/Library/Helpers/loomd` | `12108c8558b0da0ca911fcd7baec1d51fd18e94295b36cb4c35ac3fd4b76277b` |
| `Contents/Info.plist` | `5c7552c87daada68c730c3cf4a627f46175c795e5f66f108c9126197c4acaf19` |

The installed App runs as PID `7545`. Its managed bundled daemon runs as child
PID `7566` with the canonical expanded argv, exact private state, isolation,
Socket, Runtime and managed-parent bindings.

The read-only installed identity preflight passed with the explicit exact
acceptance anchor:

- 24 Providers;
- seven Runtimes;
- six executable Conversation Profiles;
- Claude Code Runtime online with capacity three;
- Mission `mission/team-instance-643ace1b858f20a1d7b60f2ccd90d6a1`;
- Team `team-instance-643ace1b858f20a1d7b60f2ccd90d6a1`;
- seven RoundTable navigation records.

Automatic governance-anchor selection failed closed because the current history
does not contain one unique P7-marked blocked Mission. The explicit exact IDs
then passed; no fuzzy first-match selection was used. A separate invocation with
an intentionally incorrect expected daemon digest failed at bundle identity
admission before product state was read.

## Closed review findings

Build 316 closes both findings from the post-Build-315 Swift review:

1. Every production write to `setupSnapshot` now passes through one
   `admitSetupSnapshot` gate. A structurally valid empty projection preserves the
   last known-good Provider and Runtime inventory, publishes `empty_setup`, and
   never becomes `.ready`. A source contract rejects any new direct write path.
2. Claude native login now binds one App-generated Incident ID to the concrete
   UDS request. Local transport failures are converted to safe staged errors and
   the installed operational diagnostic contains only operation, Provider,
   stage, result, retryability, elapsed time and Incident identity.
3. Login timeout is classified as `profile_publish`, remains retryable and keeps
   the same Incident ID. Remote errors retain the daemon Incident when present.
4. Runtime & Providers displays stage and recovery semantics, says that login
   completes in the browser, and exposes an icon-only accessible Cancel control
   while the bounded poll is active. Cancellation clears in-flight presentation
   without inventing a failure.
5. Claude login process tests now also cover Home and temporary-directory
   identity replacement plus explicit descendant process-group termination.

The empty-inventory gate is shared by background Provider Account policy, rate
card, remote tool enrollment, Vault, Team, import, configure, verify, replace and
revoke refreshes. This prevents the original class of “all Providers and
Runtimes disappeared” regressions from reappearing in a different operation.

## Verification

```text
go test ./...
PASS

go vet ./...
PASS

go test -race ./internal/runtime/harnessadapter ./internal/app \
  ./internal/api ./internal/localipc
PASS

go test -race ./cmd/loomd -run \
  'ConnectClaude|ClaudeCode|HarnessRuntimeRefresh|RouteManifest|TypedRegistry|TypedHandler|ServesStrictSetupSnapshot|RefreshesClaude' \
  -timeout 15m
PASS

swift test --package-path apps/macos
491 XCTest cases, 2 conditional skips, 0 failures
20 strict Swift Testing contract cases, 0 failures

scripts/test-build-loom-local-app.sh
PASS (two independent release builds)

scripts/test-install-loom-local-app.sh
PASS

scripts/install-loom-local-app.sh --dry-run ...
PASS

TestPhase7InstalledIdentityPreflight
PASS (build=316, providers=24, runtimes=7, profiles=6)
```

The combined full-package daemon race command reached the repository's default
ten-minute test timeout while race mode was recompiling the strict Swift probe.
It reported no race. The four affected internal packages pass in a clean command,
and the affected daemon subset passes with its explicit 15-minute bound. The
ordinary complete repository suite passes, including the full strict Go/Swift
IPC contract.

Installed visual inspection used the real 720px Runtime & Providers sheet. The
Claude `Sign In Required` status and `Sign In` action remain visible without
overlap; Provider and Runtime inventories are complete, and the long OpenCode
model list remains bounded. The screenshot contains only non-secret setup
metadata.

| Artifact | SHA-256 |
| --- | --- |
| `P7-HT2-installed-build316-runtime-provider.png` | `42a14e1f85bbeb1166d99e3db4dff11ab84906d316eedd411e10cab4270d654f` |

## Remaining gate

The user did not click `Sign In` during this gate. The official Claude CLI still
reports `loggedIn=false`, `authMethod=none`, `apiProvider=firstParty`; Setup
correctly publishes no Claude Code Conversation Profile. No credential was
read, imported, replaced, revoked or written.

Build 313 remains the last explicitly authorized paid model matrix. Phase 7
remains `PARTIAL` until the user completes official Claude browser login, the
same installed daemon publishes a Claude Code Profile without restart, and the
exact five-Runtime installed matrix passes tool selection, Proposal governance,
restart restoration and privacy checks.
