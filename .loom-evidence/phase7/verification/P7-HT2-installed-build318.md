# P7-HT2 installed Build 318 verification

Status: `PARTIAL ACCEPTANCE / AVAILABLE-RUNTIME LIVE FAILED SAFELY ON CODEX AUTH / CLAUDE LIVE N/A BY USER`

Date: 2026-08-30

## Installed identity

Loom `0.5.6` Build 318 is the exact installed and running bundle at
`/Users/lune/Applications/Loom.app`. Candidate and installed bytes are
identical, strict deep code-sign verification passed, and Build 317 is retained
as the transactional rollback bundle.

| File | SHA-256 |
| --- | --- |
| `Contents/MacOS/LoomLocalApp` | `6fa0dd327b8d4dceb2ed1a60aa321d04243248dfb8c8ad7af0c8f155b7a3a779` |
| `Contents/Library/Helpers/loomd` | `c7583c8a3884728c9f7bc758a002fe666e3b77d51aed18e66612f9f63cc52ce4` |
| `Contents/Info.plist` | `c4974be62f47c925c14ec2a9c6199f5127d6d83bbeb7b09d5750530b0a06ba13` |

The cold-started App runs as PID `414`; its bundled managed daemon runs as PID
`416` with the canonical state, isolation, Socket, Runtime, local-model and
managed-parent arguments. No standalone daemon was started.

## Installed behavior

Build 318 closes the final Claude Profile overclaim found after Build 317.
Native auth alone is insufficient: Setup requires the canonical Claude Runtime
instance, exact adapter and model, positive capacity and `workspace_edit` before
publishing the static native Conversation Profile. Foreign, stale,
zero-capacity or capability-incomplete Runtime rows fail closed.

The acceptance runner independently validates the exact Profile ID, protocol,
auth mode, Provider Account and credential revision shape for Codex, OpenCode,
Claude Code, Pi and Loom Native. Its content-only privacy marker and bounded
post-restart state/diagnostic scan are verifier authority, not App execution
authority. A real Swift client/private-UDS/product-daemon contract proves that
a blocked Setup exchange cannot delay the same-Incident Claude cancellation.

## Verification

```text
go test ./... -count=1
PASS (all packages; cmd/loomd 239.215s, internal/localipc 231.805s)

go vet ./...
PASS

affected Setup, Harness and daemon race gates
PASS

swift test --package-path apps/macos
496 XCTest cases, 2 conditional skips, 0 failures
21 strict Swift Testing contract cases, 0 failures

scripts/test-build-loom-local-app.sh
PASS (two independent deterministic release builds, strict signing and smoke)

scripts/test-install-loom-local-app.sh
PASS (replacement, rollback, injected failure and link-attack fixtures)

scripts/install-loom-local-app.sh --dry-run --app .dist-build318/Loom.app \
  --destination /Users/lune/Applications/Loom.app
PASS

codesign --verify --deep --strict /Users/lune/Applications/Loom.app
PASS

TestPhase7InstalledIdentityPreflight
PASS (build=318, daemon PID=416, providers=24, runtimes=7, profiles=6,
      exact Mission/Team anchor, registry Sessions=7)
```

The first default-concurrency Go run is preserved as failed evidence: one Pi
metadata-isolation Setup read observed a transient UDS failure and one Claude
fixture missed its five-second PID startup window under concurrent Swift
compilation. Both passed independently. Only fixture admission/recovery windows
were bounded at 30 seconds; production deadlines and five-second process-group
cleanup remained unchanged. The second default-concurrency repository run
passed completely.

## Current installed state

The exact read-only preflight reports:

- 24 Providers;
- seven online Runtimes;
- six executable Conversation Profiles;
- Claude Code Runtime `runtime.claude-code.local` online with capacity three;
- no Claude Code Conversation Profile;
- the exact Phase 7 Mission/Team anchor and seven RoundTable navigation records.

No UI production source changed between Builds 317 and 318, so the accepted
Build 317 Runtime & Providers captures remain representative of the installed
screen: Claude is `Sign In Required` and the Sign In action is present.

## Acceptance amendment

On 2026-08-30 the user stated that Claude Code is unavailable and explicitly
waived its installed login and paid live call. Loom keeps the complete Claude
adapter, strict Profile admission, cancellation, privacy and failure-isolation
source coverage. The installed Snapshot truthfully keeps Claude unavailable;
it is N/A for this user's live matrix rather than passed or failed.

The available-Runtime gate requires both its own opt-in and
`LOOM_PHASE7_CLAUDE_LIVE_WAIVER=1`. It rejects the waiver if an executable
Claude Profile is present, so a newly available Runtime must join the full gate.

## Build 318 live rerun result

The explicitly authorized available-Runtime gate made a real Codex request
against exact Build 318 bytes. It stopped in 16.69 seconds on the first
confirmation turn with `conversation_unavailable` and safe Provider subcode
`codex_control_first_turn`, Incident
`loom-client-46a24eb3e9a0e564-13`. A narrower read-only Codex probe reproduced
the failure as Incident `loom-client-cecaae1afd54604f-3`. No later model call,
confirmation or governance mutation ran.

A direct bounded native Codex invocation returned `invalid_refresh_token` while
the legacy `codex login status` command still exited successfully. The installed
Setup observer therefore published a false executable Codex Profile. This is a
Build 318 product defect, not a Claude failure and not a Loom Tool schema defect.
No credential was read, printed or changed during diagnosis.

## Remaining gate

The absence of a Claude Profile remains truthful user-auth state, not a runtime
failure. Build 319 must install the active Codex refresh probe and actionable
mid-Session auth mapping. After the user completes the official Codex login,
the available-Runtime matrix must pass against exact Build 319 bytes.

Build 313 remains the last completed paid matrix. Phase 7 remains `PARTIAL`
until the same installed tool-selection, governance, privacy and restart matrix
passes against exact Build 319 bytes for Codex, OpenCode, Pi and Loom Native
under the explicit Claude live waiver.
