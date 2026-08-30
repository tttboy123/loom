# P7-HT2 installed Build 315 verification

Status: `PARTIAL ACCEPTANCE / CLAUDE SIGN-IN RECOVERY INSTALLED / USER AUTH REQUIRED`

Date: 2026-08-30

## Installed identity

Loom `0.5.6` Build 315 is the exact installed and running bundle. Candidate and
installed executable bytes are identical, and strict deep code-sign verification
passed.

| File | SHA-256 |
| --- | --- |
| `Contents/MacOS/LoomLocalApp` | `67b7c1d3764194304e9161f87a037eaeb83a17061f9a176d5b6435f515eeaaca` |
| `Contents/Library/Helpers/loomd` | `12108c8558b0da0ca911fcd7baec1d51fd18e94295b36cb4c35ac3fd4b76277b` |
| `Contents/Info.plist` | `3b728db99c452bef77e526676110e06633dafdaef16cd8b2251fbd83b6815eab` |

The installed App runs as PID `5870`. Its managed bundled daemon runs as child
PID `5959` with the canonical expanded argv, exact private state, isolation,
Socket, Runtime and managed-parent bindings.

The read-only installed identity preflight passed with:

- 24 Providers;
- seven Runtimes;
- six executable Conversation Profiles;
- Claude Code Runtime online with capacity three;
- the exact Mission/Team acceptance anchor;
- seven RoundTable navigation records.

A negative preflight invocation used a malformed expected daemon digest and
failed at identity admission before reading product state. Repeating with the
exact installed digest passed. No fuzzy governance selection was used.

## Claude native-auth recovery

Build 315 includes the complete bounded native-auth recovery path introduced in
the Build 314 candidate and the final installed status clarification:

1. The Runtime page offers `Sign In` only for an online Claude Code Runtime
   without an executable Conversation Route.
2. The daemon starts the official `claude auth login` command through an exact,
   revalidated executable and a minimal non-secret environment. The fixed CLI
   acknowledgement is supplied without user data; process groups are bounded
   and cleaned up.
3. `claude_code_connect` is an exact admitted UDS and product-composition Route.
4. Setup re-observes native auth on every bounded Snapshot. A successful login
   can publish the Claude Conversation Profile without restarting Loom or
   `loomd`.
5. Login failures remain Claude-local and retain stage, retryability and
   Incident identity instead of collapsing Runtime & Providers.
6. The installed UI distinguishes process discovery from execution readiness:
   Claude now shows orange `Sign In Required`, not the contradictory `Online`,
   beside the intact `Sign In` button.

The user did not click `Sign In` during this gate. The official CLI still
reports `loggedIn=false`, `authMethod=none`, `apiProvider=firstParty`, and Setup
correctly publishes no Claude Code Conversation Profile. No credential was
read, imported, replaced, revoked or written.

## Verification

```text
go test ./... -count=1
PASS

go vet ./...
PASS

go test -race ./internal/runtime/harnessadapter ./internal/app ./internal/api \
  ./internal/localipc ./cmd/loomd -run '<Claude and affected contracts>' -count=1
PASS

swift test --package-path apps/macos
484 XCTest cases, 2 conditional skips, 0 failures
20 strict Swift Testing contract cases, 0 failures

scripts/test-build-loom-local-app.sh
PASS (two independent release builds)

scripts/test-install-loom-local-app.sh
PASS

scripts/install-loom-local-app.sh --dry-run ...
PASS

TestPhase7InstalledIdentityPreflight
PASS (build=315, providers=24, runtimes=7, profiles=6)
```

Installed visual inspection used the real 720px Runtime & Providers sheet. The
Claude status and button are visible without overlap or horizontal clipping;
the OpenCode model list remains bounded and truncated. The image contains only
non-secret setup metadata.

| Artifact | SHA-256 |
| --- | --- |
| `P7-HT2-installed-build315-runtime-provider.png` | `e2eab1b528bef9fdce7bd288971672dd84ae00f34429566c1c6c363c11776d98` |

## Remaining gate

Build 313 remains the last explicitly authorized real-model matrix: Codex,
OpenCode, Pi and Loom Native passed all 28 Tools, Proposal decisions,
RoundTable governance and restart restoration. Build 315 does not claim that
paid matrix was rerun.

Phase 7 remains `PARTIAL`. The user must click `Sign In`, complete the official
Claude browser authentication and let Setup publish an executable Claude Code
Conversation Profile. Only then can the exact installed five-Runtime matrix be
run and the Goal considered for completion.
