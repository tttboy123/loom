# P7-HT2 installed Build 264 verification

Status: `PARTIAL / LOCAL INSTALL ACCEPTED / PI ASSET AND REAL MODEL MATRIX PENDING`

Date: 2026-08-29

## Bundle identity

- App: `/Users/lune/Applications/Loom.app`
- Version: `0.5.6`
- Build: `264`
- Architecture: `arm64`
- Signing: ad-hoc, `codesign --verify --deep --strict` passed
- Rollback: `/Users/lune/Applications/Loom.app.previous`, Loom `0.5.6`
  Build `263`

Candidate and installed executable bytes are identical:

| File | SHA-256 |
| --- | --- |
| `Contents/MacOS/LoomLocalApp` | `9db2b30f8091987ce5e9c5c7bdf7c0d9cae983263691aa8ebbbbc3308bedec75` |
| `Contents/Library/Helpers/loomd` | `b841084fb63e04b635918004ac4e12beec46afc7da01288bfa330b11d7aa99a2` |
| `Contents/Info.plist` | `7e1387784b47981f1446badd62fe4ec7f6610c0e6b4e78feb0a57235dffba916` |

The permission-aware candidate manifest SHA-256 is
`f8ef3749d2565f81cc6361d080f23cdcf3dcc2b873ca1cf1fc2ea2fc6ba864c0`.

## Accepted behavior

Build 264 preserves every schema-v2 terminal Proposal decision as an exact,
immutable receipt. The receipt binds Proposal identity and digest, decision,
decision time and Incident ID. A restored historical confirmed Proposal without
that receipt remains visible but cannot execute. Replay cannot create a second
receipt or a second execution authority.

The macOS review card requires the exact receipt before dispatch, shows the
decision Incident ID and provides a copy action. The daemon records a bounded,
content-free `chat_control_decision` diagnostic. It does not record the
Conversation body, Prompt, Provider response or credential.

The same build fixes later turns inside an immutable Segment. The Segment keeps
its opening Execution Binding frozen, while route validation now compares the
current Attempt's Context Capsule digest. A legitimate second or later Turn no
longer fails merely because its Capsule differs from the Segment-opening
Capsule.

## Source, build and install gates

```text
swift test --package-path apps/macos
461 XCTest cases, 2 conditional skips, 0 failures
20 strict Swift Testing contract cases, 0 failures

go test ./... -count=1
PASS

go vet ./...
PASS

go test -race ./internal/controltool ./internal/api ./cmd/loomd \
  -run 'Test(ProposalDecisionReceipt|Conversation(Control|Action|Governance|Mission)|ConfirmedGovernance|ProductChatControl|ProductOperationalDiagnosticsRecordsContentFreeControlDecision|LivePhase7ControlToolsE2E|SelectPhase7LiveProfiles)' \
  -count=1 -timeout=20m
PASS

scripts/build-loom-local-app.sh --output .../.dist-build264/Loom.app
PASS (59.53s)

scripts/test-build-loom-local-app.sh
native app build fixture PASS (303.83s)

scripts/test-install-loom-local-app.sh
native app installer fixture PASS (13.33s)
```

The deterministic double-build gate, strict signing, dry-run installation and
transactional installation all passed. The installer retained Build 263 as the
rollback bundle. No real Provider request, paid model call or credential
operation was used.

## Post-install live-gate hardening

The source-only acceptance runner now requires the exact Build 264 App and
daemon SHA-256 values above in addition to the build number, and repeats strict
signature plus file-identity verification after a managed restart. Its restart
poll no longer fails merely because the private Socket directory has not yet
appeared, while duplicate daemon processes and pseudo-Socket arguments fail
closed.

The complete `cmd/loomd` package passed in 247.200 seconds, its focused race
gate and `go vet ./cmd/loomd` passed, and the read-only installed identity
preflight passed with daemon PID 25164 and the same 24 Provider, seven Runtime,
seven Profile inventory. This hardening is test-only and does not alter the
already installed bundle bytes.

## Managed startup and restart

Build 264 was installed, launched, stopped completely and launched again. On
both starts the App ran from the installed bundle and its bundled `loomd` ran
as the managed child with canonical state, isolation, socket, Runtime and
managed-parent arguments. No secret appeared in process arguments.

The first and second read-only private UDS `setup_snapshot` probes completed in
412 and 443 milliseconds. Both returned the same inventory:

- 24 Provider descriptors;
- seven online Runtime instances;
- seven executable Conversation Profiles;
- five bounded CC Switch import candidates;
- verified DeepSeek revision 2 and MiniMax revision 23 accounts;
- distinct Codex, Claude Code, OpenCode, Pi and Loom Native Runtime rows.

The seven executable Profiles are Loom Native with DeepSeek, Loom Native with
MiniMax, OpenCode with DeepSeek, OpenCode with MiniMax, OpenCode native auth,
Claude Code native auth and Codex native auth. Pi is detected online but is not
published as an executable Conversation Profile because its locked local model
assets are absent. This is the intended fail-closed result.

## Visual acceptance

The confirmed Proposal review remains legible at compact and accessibility
sizes. It shows the exact route change, approved state, disclosure note,
decision Incident ID, copy action and governed apply action without overlap or
horizontal clipping.

| Artifact | SHA-256 |
| --- | --- |
| `action-proposal-receipt-360.png` | `e32269d016995396f0b4f4d0e04a4cc5061d856fa495808b57dd4a8b5e6e7b82` |
| `action-proposal-receipt-560.png` | `5ff7a2aa84614a1f27e01f26c7a66e2ebd5a7be50c2013421725c0c4678f9601` |

## Remaining gate

The machine still has no integrity-checked Pi local-model assets. The dedicated
installed five-Runtime runner remains source-ready and disabled. Phase 7 stays
`PARTIAL` until explicit authorization permits the asset and real-model work,
and the installed matrix proves model tool selection, Proposal review, user
confirmation and cancellation, governed execution, expiry, replay rejection
and restart restoration across all five Runtimes.
