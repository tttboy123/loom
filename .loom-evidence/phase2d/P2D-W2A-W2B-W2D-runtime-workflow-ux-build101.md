# Phase 2D Build 101 Runtime and Workflow UX Evidence

Status: `CURRENT / VERIFIED SLICE`

Date: 2026-08-22

## Installed product

- App: `/Users/lune/Applications/Loom.app`
- Version: `0.5.3`
- Build: `101`
- Candidate: `/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.3-build101-2026-08-22/Loom.app`
- Bundle signature: strict deep verification passed.
- Lifecycle: cold launch started the bundled daemon as the App child; no manual
  `loomd` step was used.
- Daemon identity: canonical non-secret state, isolation, Socket, Harness
  executable and managed-parent arguments were present.

## Installed setup snapshot

The privacy-safe local contract probe returned:

- Providers: `25`
- Runtimes: `7`
- Conversation Profiles: `4`
- Runtime IDs: Claude Code, Codex, three Loom Native routes, OpenCode and Pi.

No credential import, verification, Provider request, chat content or secret
read was used for this inventory gate.

## Product behavior

- Detected Harness Runtimes remain visible independently of Provider Account
  readiness; adding OpenCode does not hide unrelated Runtimes.
- OpenAI is configurable as a brokered Provider Account without removing the
  Codex native-auth Conversation Profile.
- Conversation route selection enumerates every Profile and labels Harness plus
  Provider Account; Model and reasoning remain separate controls.
- Unsupported reasoning effort selection fails closed and does not create a
  false route transition.
- Running and blocked Missions remain visible if their Team temporarily becomes
  non-executable.
- Mission preflight and setup failures show a Runtime & Providers recovery
  action instead of an inert button or false empty directory.
- RoundTable writer and target seats follow confirmed add/drop order; a retired
  seat cannot be restored by local ordering state.

An app-window-only installed visual check showed the conversation-first shell,
route controls and `Local service ready` with no overlap. No other application
window was captured and the image was not added to evidence because existing
conversation labels are private user state.

## Verification

- `swift test --package-path apps/macos`: 305 tests passed, one intentional
  visual-export skip, zero failures.
- Strict Swift Testing wire contracts: 16 passed.
- Affected Go packages: passed.
- `go vet ./...`: passed.
- `git diff --check`: passed.
- `scripts/test-build-loom-local-app.sh`: passed for build 101.
- Installer transaction fixture: passed before final install; the build 101
  atomic install and strict post-install signature validation also passed.
- Installed setup probe: `25 / 7 / 4`.

## Remaining Phase 2D gates

Phase 2D remains `ACTIVE / PARTIAL`. This slice does not close the installed
four-Harness/four-Provider Team matrix, complete account-local failure matrix,
explicit approved fallback, complete accounting/governance UI, or remaining
Context Capsule disclosure/encryption gates.
