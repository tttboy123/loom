# P2D-W2B/W2D Installed Runtime and Provider Account Separation - Build 91

Status: `CURRENT / VERIFIED SLICE`

Date: 2026-08-22

Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

## Installed boundary

- App: `/Users/lune/Applications/Loom.app`, version `0.5.3`, build `91`.
- App executable SHA-256:
  `416174159e513b450943c6609062df7f886873812f4e6928b492028a93c4194d`.
- Bundled daemon SHA-256:
  `8f8f99222f4eeccff2d99e70d802cde80c9dda671414f9e9c2828f53abb19927`.
- The old App and daemon both exited before installation. The new App became
  the managed parent of the new bundled daemon; no old Socket service was
  reused.
- Bundle, daemon, state database and product Socket passed owner-only checks.
  The bundle passed strict deep signature verification.

## Root cause and correction

Runtime discovery had been incorrectly gated by the presence of a verified
Provider Account. Codex and Claude Code executables therefore disappeared from
the Runtime directory when their brokered OpenAI or Anthropic Vault accounts
were absent, even though the Harnesses were installed and executable-attested.
The bundled service PATH also omitted Claude, so the installed daemon could not
discover it even though source tests could.

Build 91 separates the three states:

1. executable-attested Harness discovery publishes the Runtime row;
2. a verified compatible Provider Account publishes a freezable Execution
   Profile and Agent Team role;
3. the UI explains a detected Harness without an eligible account and names
   the Provider setup action instead of hiding the Runtime or claiming it is
   executable by a Team.

Codex, Claude Code and OpenCode remain distinct Harnesses. No synthetic
OpenCode Provider or global Team Provider was introduced. Missing credentials
do not produce an Agent definition or role, and no runtime identity validation
was weakened.

## Installed result

- Catalog: 25 Providers, 6 online Runtimes, 4 Conversation Profiles.
- Online Runtimes: Claude Code, Codex, Loom Native/DeepSeek, Loom
  Native/MiniMax, OpenCode and Pi.
- Verified Vault accounts remained scoped to DeepSeek and MiniMax. OpenAI
  native conversation auth remained available; Anthropic and Kimi remained
  honestly unconfigured.
- A bounded installed DeepSeek network probe returned a real reply. The plain
  conversation tool surface remained intentionally absent.
- The build 91 installed OpenCode Conversation gate returned the exact bounded
  `E2E-OK` acceptance reply with the same 25/6/4 catalog.
- Build 90 had already passed the installed OpenCode Conversation, OpenCode
  Agent Team, and four-Agent DeepSeek/MiniMax mixed-Team live gates. Build 91
  changes only Runtime/account presentation on top of that verified behavior.

## Verification

- Full `go test ./... -p 1 -count=1`: PASS.
- `go vet ./...`: PASS.
- macOS suite: 292 tests, 1 intentional skip, 0 failures.
- Strict Swift wire-contract suite: 15/15 PASS.
- Native App builder fixture: PASS across two release builds.
- Builder contract resolves `node`, `codex`, `opencode` and `claude` into the
  bundled service PATH.
- `git diff --check`: PASS.
- Installed visual capture was attempted, but the macOS graphical session was
  locked and produced a black frame. Functional and accessibility-source tests
  passed; the unlocked installed visual walk-through remains open.

No credential body, credential reference, Prompt, conversation content,
Provider body, Authorization header or environment secret was read or recorded
in this evidence.

## Remaining Phase 2D gates

This slice closes Runtime disappearance and Runtime/Provider Account status
ambiguity. It does not close Phase 2D. The four-Harness/four-Provider installed
Team, remaining account-local failure cells, approved fallback, complete
accounting/governance UI, full Capsule disclosure/encryption matrix, compatible
custom endpoint import, and unlocked installed visual walk-through remain open.
